package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	stableIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[-_.:][a-z0-9]+)*$`)
	sha256Pattern   = regexp.MustCompile(`^[0-9a-f]{64}$`)
	oidPattern      = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

func NewCaseID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate case id: %w", err)
	}
	return "case-" + hex.EncodeToString(b), nil
}

func SHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func DigestLines(values []string) string {
	copyValues := append([]string(nil), values...)
	sort.Strings(copyValues)
	return SHA256([]byte(strings.Join(copyValues, "\n")))
}

func FingerprintPolicy(p Policy) (string, error) {
	type semanticSource struct {
		SourceID string `json:"source_id"`
		Role     string `json:"role"`
		Kind     string `json:"kind"`
	}
	sources := make([]semanticSource, 0, len(p.Sources))
	for _, source := range p.Sources {
		sources = append(sources, semanticSource{SourceID: source.SourceID, Role: source.Role, Kind: source.Kind})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].SourceID < sources[j].SourceID })
	projection := struct {
		SchemaVersion string           `json:"schema_version"`
		Profile       string           `json:"profile"`
		Operator      Operator         `json:"operator"`
		Sources       []semanticSource `json:"sources"`
		Limits        Limits           `json:"limits"`
	}{p.SchemaVersion, p.Profile, p.Operator, sources, p.Limits}
	b, err := json.Marshal(projection)
	if err != nil {
		return "", fmt.Errorf("marshal policy projection: %w", err)
	}
	return SHA256(b), nil
}

func FingerprintCase(c Case) (string, error) {
	type semanticSource struct {
		SourceID          string          `json:"source_id"`
		Role              string          `json:"role"`
		DefaultBranch     *string         `json:"default_branch"`
		ObjectFormat      string          `json:"object_format"`
		Shallow           bool            `json:"shallow"`
		PartialClone      bool            `json:"partial_clone"`
		LFS               LFSState        `json:"lfs"`
		Submodules        SubmoduleState  `json:"submodules"`
		Integrity         string          `json:"integrity"`
		Completeness      string          `json:"completeness"`
		Refs              []Ref           `json:"refs"`
		Objects           ObjectInventory `json:"objects"`
		SourceFingerprint string          `json:"source_fingerprint"`
	}
	sources := make([]semanticSource, 0, len(c.Sources))
	for _, source := range c.Sources {
		refs := append([]Ref(nil), source.Refs...)
		sort.Slice(refs, func(i, j int) bool { return refs[i].OriginalName < refs[j].OriginalName })
		subs := source.Submodules
		subs.Entries = append([]string(nil), subs.Entries...)
		sort.Strings(subs.Entries)
		sources = append(sources, semanticSource{
			SourceID: source.SourceID, Role: source.Role, DefaultBranch: source.DefaultBranch,
			ObjectFormat: source.ObjectFormat, Shallow: source.Shallow, PartialClone: source.PartialClone,
			LFS: source.LFS, Submodules: subs, Integrity: source.Integrity,
			Completeness: source.Completeness, Refs: refs, Objects: source.Objects,
			SourceFingerprint: source.SourceFingerprint,
		})
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].SourceID < sources[j].SourceID })
	relationships := append([]Relationship(nil), c.Relationships...)
	sort.Slice(relationships, func(i, j int) bool { return relationships[i].RelationshipID < relationships[j].RelationshipID })
	findings := append([]Finding(nil), c.Findings...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].FindingID < findings[j].FindingID })
	gates := append([]Gate(nil), c.Gates...)
	for i := range gates {
		gates[i].ChangedAt = ""
	}
	sort.Slice(gates, func(i, j int) bool { return gates[i].GateID < gates[j].GateID })
	decisions := append([]Decision(nil), c.Decisions...)
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].DecisionID < decisions[j].DecisionID })
	projection := struct {
		SchemaVersion string           `json:"schema_version"`
		ToolVersion   string           `json:"tool_version"`
		GitVersion    string           `json:"git_version"`
		Status        string           `json:"status"`
		Policy        PolicyEvidence   `json:"policy"`
		Sources       []semanticSource `json:"sources"`
		Relationships []Relationship   `json:"relationships"`
		Findings      []Finding        `json:"findings"`
		Gates         []Gate           `json:"gates"`
		Decisions     []Decision       `json:"decisions"`
	}{c.SchemaVersion, c.Tool.Version, c.Tool.GitVersion, c.Status, c.Policy, sources, relationships, findings, gates, decisions}
	b, err := json.Marshal(projection)
	if err != nil {
		return "", fmt.Errorf("marshal case projection: %w", err)
	}
	return SHA256(b), nil
}

func IsStableID(value string) bool { return stableIDPattern.MatchString(value) }
func IsSHA256(value string) bool   { return sha256Pattern.MatchString(value) }
func IsOID(value string) bool      { return oidPattern.MatchString(value) }
