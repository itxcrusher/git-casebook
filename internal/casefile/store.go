package casefile

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/itxcrusher/git-casebook/internal/model"
	productschema "github.com/itxcrusher/git-casebook/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

const (
	PolicyFile   = "policy.yaml"
	CaseFile     = "case.json"
	EventsFile   = "events.jsonl"
	FindingsFile = "findings.jsonl"
	ReportFile   = "report.md"
)

type Store struct {
	Root string
}

var (
	compiledSchema     *jsonschema.Schema
	compiledSchemaErr  error
	compiledSchemaOnce sync.Once
)

func Open(root string) (*Store, error) {
	if root == "" {
		root = ".case"
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve case directory: %w", err)
	}
	abs = filepath.Clean(abs)
	if _, statErr := os.Stat(abs); statErr == nil {
		resolved, resolveErr := filepath.EvalSymlinks(abs)
		if resolveErr != nil {
			return nil, fmt.Errorf("resolve case directory symlinks: %w", resolveErr)
		}
		abs = resolved
	}
	return &Store{Root: abs}, nil
}

func (s *Store) Init(caseID, operator string) (model.Policy, error) {
	if operator == "" {
		operator = "local-operator"
	}
	if caseID == "" {
		var err error
		caseID, err = model.NewCaseID()
		if err != nil {
			return model.Policy{}, err
		}
	}
	if !model.IsStableID(caseID) {
		return model.Policy{}, fmt.Errorf("case id %q is not a stable id", caseID)
	}
	if _, err := os.Stat(s.path(PolicyFile)); err == nil {
		return model.Policy{}, fmt.Errorf("case already exists at %s", s.Root)
	} else if !errors.Is(err, os.ErrNotExist) {
		return model.Policy{}, err
	}
	for _, dir := range []string{s.Root, s.path("sources"), s.path("artifacts", "sha256"), s.path("control", "home"), s.path("control", "hooks"), s.path("control", "template"), s.path("control", "xdg")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return model.Policy{}, fmt.Errorf("create case directory %s: %w", dir, err)
		}
	}
	policy := model.Policy{
		SchemaVersion: model.SchemaVersion,
		Profile:       "offline-provenance-v1",
		CaseID:        caseID,
		Operator:      model.Operator{Kind: "HUMAN", Identifier: operator},
		Sources:       []model.PolicySource{},
		Limits: model.Limits{
			CommandTimeoutSeconds:     120,
			AcquisitionTimeoutSeconds: 600,
			MaxCommandOutputBytes:     16 << 20,
		},
	}
	if err := s.SavePolicy(policy); err != nil {
		return model.Policy{}, err
	}
	for _, name := range []string{EventsFile, FindingsFile} {
		if err := atomicWrite(s.path(name), []byte{}, 0o600); err != nil {
			return model.Policy{}, err
		}
	}
	if err := s.AppendEvent(model.Event{
		EventID: "event-case-created", Kind: "CASE_CREATED", Timestamp: now(), Actor: operator,
		Details: map[string]any{"capability_level": 0},
	}); err != nil {
		return model.Policy{}, err
	}
	return policy, nil
}

func (s *Store) SavePolicy(policy model.Policy) error {
	if err := validatePolicy(policy); err != nil {
		return err
	}
	b, err := yaml.Marshal(policy)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	return atomicWrite(s.path(PolicyFile), b, 0o600)
}

func (s *Store) LoadPolicy() (model.Policy, error) {
	f, err := os.Open(s.path(PolicyFile))
	if err != nil {
		return model.Policy{}, fmt.Errorf("open policy: %w", err)
	}
	defer f.Close()
	decoder := yaml.NewDecoder(io.LimitReader(f, 4<<20))
	decoder.KnownFields(true)
	var policy model.Policy
	if err := decoder.Decode(&policy); err != nil {
		return model.Policy{}, fmt.Errorf("decode policy: %w", err)
	}
	if err := validatePolicy(policy); err != nil {
		return model.Policy{}, err
	}
	return policy, nil
}

func (s *Store) AddSource(source model.PolicySource) (model.Policy, error) {
	policy, err := s.LoadPolicy()
	if err != nil {
		return model.Policy{}, err
	}
	if source.SourceID == "" {
		source.SourceID = fmt.Sprintf("source-%02d", len(policy.Sources)+1)
	}
	if source.Role == "" {
		source.Role = "UNKNOWN"
	}
	if source.Kind == "" {
		source.Kind = "auto"
	}
	for _, existing := range policy.Sources {
		if existing.SourceID == source.SourceID {
			return model.Policy{}, fmt.Errorf("source id %q already exists", source.SourceID)
		}
		if existing.Locator == source.Locator {
			return policy, nil
		}
	}
	policy.Sources = append(policy.Sources, source)
	if err := s.SavePolicy(policy); err != nil {
		return model.Policy{}, err
	}
	if err := s.AppendEvent(model.Event{
		EventID: fmt.Sprintf("event-source-declared-%s-%d", source.SourceID, time.Now().UTC().UnixNano()),
		Kind:    "SOURCE_DECLARED", Timestamp: now(), Actor: policy.Operator.Identifier,
		Details: map[string]any{"source_id": source.SourceID, "kind": source.Kind},
	}); err != nil {
		return model.Policy{}, err
	}
	return policy, nil
}

func (s *Store) SaveCase(c *model.Case) error {
	model.Normalize(c)
	c.SourceCount = len(c.Sources)
	fingerprint, err := model.FingerprintCase(*c)
	if err != nil {
		return err
	}
	c.CaseFingerprint = fingerprint
	if err := model.Validate(*c); err != nil {
		return fmt.Errorf("semantic validation: %w", err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal case: %w", err)
	}
	b = append(b, '\n')
	if err := ValidateSchema(b); err != nil {
		return fmt.Errorf("schema validation: %w", err)
	}
	return atomicWrite(s.path(CaseFile), b, 0o600)
}

func (s *Store) LoadCase() (model.Case, error) {
	b, err := os.ReadFile(s.path(CaseFile))
	if err != nil {
		return model.Case{}, fmt.Errorf("read case: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	var c model.Case
	if err := decoder.Decode(&c); err != nil {
		return model.Case{}, fmt.Errorf("decode case: %w", err)
	}
	return c, nil
}

func ValidateSchema(document []byte) error {
	compiledSchemaOnce.Do(func() {
		var schemaDocument any
		if err := json.Unmarshal(productschema.CaseV1, &schemaDocument); err != nil {
			compiledSchemaErr = fmt.Errorf("decode embedded schema: %w", err)
			return
		}
		compiler := jsonschema.NewCompiler()
		compiler.AssertFormat()
		if err := compiler.AddResource("urn:git-casebook:case:1.0.0", schemaDocument); err != nil {
			compiledSchemaErr = fmt.Errorf("load embedded schema: %w", err)
			return
		}
		compiledSchema, compiledSchemaErr = compiler.Compile("urn:git-casebook:case:1.0.0")
		if compiledSchemaErr != nil {
			compiledSchemaErr = fmt.Errorf("compile embedded schema: %w", compiledSchemaErr)
		}
	})
	if compiledSchemaErr != nil {
		return compiledSchemaErr
	}
	var value any
	if err := json.Unmarshal(document, &value); err != nil {
		return fmt.Errorf("decode JSON for schema validation: %w", err)
	}
	if err := compiledSchema.Validate(value); err != nil {
		return err
	}
	return nil
}

func (s *Store) AppendEvent(event model.Event) error {
	if event.Timestamp == "" {
		event.Timestamp = now()
	}
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	path := s.path(EventsFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open event stream: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return f.Sync()
}

func (s *Store) ValidateJSONL(name string) error {
	f, err := os.Open(s.path(name))
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 4<<20)
	line := 0
	for scanner.Scan() {
		line++
		if !json.Valid(scanner.Bytes()) {
			return fmt.Errorf("%s line %d is not valid JSON", name, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	return nil
}

func (s *Store) Path(parts ...string) (string, error) {
	candidate := s.path(parts...)
	rel, err := filepath.Rel(s.Root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path escapes case directory")
	}
	current := s.Root
	segments := strings.Split(rel, string(filepath.Separator))
	for _, segment := range segments {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			break
		}
		if statErr != nil {
			return "", statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("path traverses symlink %s", current)
		}
	}
	return candidate, nil
}

func (s *Store) path(parts ...string) string {
	all := append([]string{s.Root}, parts...)
	return filepath.Join(all...)
}

func validatePolicy(policy model.Policy) error {
	if policy.SchemaVersion != model.SchemaVersion {
		return fmt.Errorf("unsupported policy schema_version %q", policy.SchemaVersion)
	}
	if !model.IsStableID(policy.CaseID) {
		return fmt.Errorf("invalid case_id %q", policy.CaseID)
	}
	if policy.Profile == "" || policy.Operator.Identifier == "" {
		return fmt.Errorf("policy profile and operator are required")
	}
	if policy.Operator.Kind != "HUMAN" && policy.Operator.Kind != "AUTOMATION" {
		return fmt.Errorf("unsupported operator kind %q", policy.Operator.Kind)
	}
	seen := map[string]bool{}
	for _, source := range policy.Sources {
		if !model.IsStableID(source.SourceID) || seen[source.SourceID] {
			return fmt.Errorf("invalid or duplicate source id %q", source.SourceID)
		}
		seen[source.SourceID] = true
		if strings.TrimSpace(source.Locator) == "" {
			return fmt.Errorf("source %s has an empty locator", source.SourceID)
		}
		switch source.Role {
		case "ORIGINAL", "FORK", "MIRROR", "ARCHIVE", "UNKNOWN":
		default:
			return fmt.Errorf("source %s has unsupported role %q", source.SourceID, source.Role)
		}
		switch source.Kind {
		case "auto", "local", "remote":
		default:
			return fmt.Errorf("source %s has unsupported kind %q", source.SourceID, source.Kind)
		}
	}
	if policy.Limits.CommandTimeoutSeconds < 1 || policy.Limits.AcquisitionTimeoutSeconds < 1 || policy.Limits.MaxCommandOutputBytes < 1024 {
		return fmt.Errorf("policy limits must be positive")
	}
	return nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		_ = tmp.Close()
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
