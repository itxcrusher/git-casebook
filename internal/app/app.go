package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/itxcrusher/git-casebook/internal/casefile"
	"github.com/itxcrusher/git-casebook/internal/compare"
	"github.com/itxcrusher/git-casebook/internal/evidence"
	"github.com/itxcrusher/git-casebook/internal/gitexec"
	"github.com/itxcrusher/git-casebook/internal/inventory"
	"github.com/itxcrusher/git-casebook/internal/model"
	"github.com/itxcrusher/git-casebook/internal/preserve"
	"github.com/itxcrusher/git-casebook/internal/refplan"
	"github.com/itxcrusher/git-casebook/internal/report"
	"github.com/itxcrusher/git-casebook/internal/version"
)

const extensionNamespace = "io.github.itxcrusher.git-casebook"

type App struct {
	CaseRoot string
}

type Verification struct {
	Ready   bool     `json:"ready"`
	Status  string   `json:"status"`
	Reasons []string `json:"reasons"`
}

func (a App) Init(caseID, operator string) (model.Policy, error) {
	store, err := casefile.Open(a.CaseRoot)
	if err != nil {
		return model.Policy{}, err
	}
	return store.Init(caseID, operator)
}

func (a App) AddSource(sourceID, locator, role, kind string) (model.Policy, error) {
	if _, _, err := preserve.ClassifyLocator(locator, kind); err != nil {
		return model.Policy{}, err
	}
	store, err := casefile.Open(a.CaseRoot)
	if err != nil {
		return model.Policy{}, err
	}
	return store.AddSource(model.PolicySource{SourceID: sourceID, Locator: locator, Role: role, Kind: kind})
}

func (a App) Preserve(ctx context.Context) (model.Case, error) {
	store, policy, runner, gitVersion, err := a.runtime(ctx)
	if err != nil {
		return model.Case{}, err
	}
	if len(policy.Sources) == 0 {
		return model.Case{}, fmt.Errorf("case has no declared sources")
	}
	artifacts := evidence.Store{CaseRoot: store.Root}
	createdAt := now()
	if prior, loadErr := store.LoadCase(); loadErr == nil {
		createdAt = prior.CreatedAt
	}
	c := model.Case{
		SchemaVersion: model.SchemaVersion, CaseID: policy.CaseID, CreatedAt: createdAt,
		Tool:   model.Tool{Name: "git-casebook", Version: version.Current(), GitVersion: gitVersion, Platform: runtime.GOOS + "/" + runtime.GOARCH},
		Status: "INCOMPLETE", Operator: policy.Operator,
		Sources: []model.Source{}, Relationships: []model.Relationship{}, EvidenceItems: []model.EvidenceItem{},
		Findings: []model.Finding{}, Gates: defaultGates(false, policy.Operator.Identifier), Decisions: []model.Decision{},
		Extensions: map[string]any{extensionNamespace: map[string]any{"analysis_network_capable_command_count": 0}},
	}
	c.Policy.Fingerprint, err = model.FingerprintPolicy(policy)
	if err != nil {
		return model.Case{}, err
	}
	c.Policy.Profile = policy.Profile
	c.Policy.CapabilityLevel = 1
	c.Policy.NetworkAfterAcquisition = false
	usedNetwork := false
	for _, declared := range policy.Sources {
		acquired, acquireErr := preserve.Acquire(ctx, runner, store.Root, declared)
		if acquired.NetworkUsed {
			usedNetwork = true
		}
		source, item, buildErr := acquisitionSource(artifacts, declared, acquired, acquireErr)
		if buildErr != nil {
			return model.Case{}, buildErr
		}
		c.Sources = append(c.Sources, source)
		c.EvidenceItems = append(c.EvidenceItems, item)
		if acquireErr != nil {
			c.Findings = append(c.Findings, model.Finding{
				FindingID: "finding-acquisition-" + declared.SourceID, Kind: "UNRESOLVED_UNCERTAINTY",
				Summary:     "The declared source could not be preserved; deterministic analysis remains unavailable.",
				EvidenceIDs: []string{item.EvidenceID}, VerificationState: "UNRESOLVED",
			})
		}
	}
	if usedNetwork {
		c.Gates = defaultGates(true, policy.Operator.Identifier)
	}
	if err := store.SaveCase(&c); err != nil {
		return model.Case{}, err
	}
	_ = store.AppendEvent(model.Event{EventID: eventID("preservation"), Kind: "PRESERVATION_COMPLETED", Timestamp: now(), Actor: policy.Operator.Identifier, Details: map[string]any{"source_count": len(c.Sources), "network_revoked": usedNetwork}})
	return c, nil
}

func (a App) Inspect(ctx context.Context) (model.Case, error) {
	store, policy, runner, _, err := a.runtime(ctx)
	if err != nil {
		return model.Case{}, err
	}
	c, err := store.LoadCase()
	if err != nil {
		return model.Case{}, fmt.Errorf("preserve sources before inspection: %w", err)
	}
	artifacts := evidence.Store{CaseRoot: store.Root}
	newSources := make([]model.Source, 0, len(policy.Sources))
	newEvidence := filterEvidence(c.EvidenceItems, "evidence-inventory-")
	c.Findings = filterFindings(c.Findings, "finding-completeness-")
	for _, declared := range policy.Sources {
		existing, ok := findSource(c.Sources, declared.SourceID)
		if !ok {
			return model.Case{}, fmt.Errorf("source %s is missing from canonical case state", declared.SourceID)
		}
		mirrorPath, pathErr := store.Path(existing.LocalMirrorLocator)
		if pathErr != nil {
			return model.Case{}, pathErr
		}
		if _, statErr := os.Stat(mirrorPath); statErr != nil {
			newSources = append(newSources, existing)
			continue
		}
		acquired := preserve.Result{
			MirrorPath: mirrorPath, MirrorRelativePath: existing.LocalMirrorLocator,
			StoredLocator: existing.OriginalLocator, Method: extensionString(existing.Extensions, "acquisition_method"),
			RetrievedAt: existing.RetrievedAt, MirrorFingerprint: extensionString(existing.Extensions, "mirror_tree_fingerprint"),
		}
		analyzed, data, item, analyzeErr := inventory.Analyze(ctx, runner, artifacts, declared, acquired)
		if analyzeErr != nil {
			return model.Case{}, analyzeErr
		}
		newSources = append(newSources, analyzed)
		newEvidence = append(newEvidence, item)
		if len(data.CompletenessReasons) > 0 {
			c.Findings = append(c.Findings, model.Finding{
				FindingID:   "finding-completeness-" + declared.SourceID,
				Kind:        "UNRESOLVED_UNCERTAINTY",
				Summary:     "Source completeness is limited by: " + strings.Join(data.CompletenessReasons, ", ") + ".",
				EvidenceIDs: []string{item.EvidenceID}, VerificationState: "UNRESOLVED",
			})
		}
	}
	c.Sources = newSources
	c.EvidenceItems = newEvidence
	c.Relationships = []model.Relationship{}
	c.Policy.CapabilityLevel = 2
	c.Status = "OPEN"
	setAnalysisAudit(&c, runner.Records())
	if err := store.SaveCase(&c); err != nil {
		return model.Case{}, err
	}
	_ = store.AppendEvent(model.Event{EventID: eventID("inspection"), Kind: "OFFLINE_INSPECTION_COMPLETED", Timestamp: now(), Actor: policy.Operator.Identifier, Details: map[string]any{"source_count": len(c.Sources)}})
	return c, nil
}

func (a App) Compare() (model.Case, error) {
	store, err := casefile.Open(a.CaseRoot)
	if err != nil {
		return model.Case{}, err
	}
	c, err := store.LoadCase()
	if err != nil {
		return model.Case{}, err
	}
	artifacts := evidence.Store{CaseRoot: store.Root}
	data := make(map[string]inventory.Data, len(c.Sources))
	for _, source := range c.Sources {
		loaded, loadErr := inventory.LoadData(artifacts, source)
		if loadErr != nil {
			return model.Case{}, fmt.Errorf("load source %s manifests: %w", source.SourceID, loadErr)
		}
		data[source.SourceID] = loaded
	}
	relationships, items, err := compare.All(c.Sources, data, artifacts)
	if err != nil {
		return model.Case{}, err
	}
	if err := compare.AssertInverse(relationships); err != nil {
		return model.Case{}, err
	}
	c.Relationships = relationships
	c.EvidenceItems = append(filterEvidence(c.EvidenceItems, "evidence-relationship-"), items...)
	c.Status = "OPEN"
	if err := store.SaveCase(&c); err != nil {
		return model.Case{}, err
	}
	_ = store.AppendEvent(model.Event{EventID: eventID("comparison"), Kind: "PAIRWISE_COMPARISON_COMPLETED", Timestamp: now(), Actor: c.Operator.Identifier, Details: map[string]any{"relationship_count": len(relationships)}})
	return c, nil
}

func (a App) PlanRefs(ctx context.Context) (model.Case, error) {
	store, policy, runner, _, err := a.runtime(ctx)
	if err != nil {
		return model.Case{}, err
	}
	c, err := store.LoadCase()
	if err != nil {
		return model.Case{}, err
	}
	artifacts := evidence.Store{CaseRoot: store.Root}
	sources, item, err := refplan.Plan(ctx, runner, c.Sources, artifacts)
	if err != nil {
		return model.Case{}, err
	}
	c.Sources = sources
	c.EvidenceItems = append(filterEvidence(c.EvidenceItems, "evidence-ref-plan"), item)
	setAnalysisAudit(&c, runner.Records())
	if err := store.SaveCase(&c); err != nil {
		return model.Case{}, err
	}
	_ = store.AppendEvent(model.Event{EventID: eventID("ref-plan"), Kind: "REF_PLAN_CREATED", Timestamp: now(), Actor: policy.Operator.Identifier, Details: map[string]any{"collision_count": collisionCount(c.Sources)}})
	return c, nil
}

func (a App) Verify(ctx context.Context) (Verification, error) {
	store, _, runner, _, err := a.runtime(ctx)
	if err != nil {
		return Verification{}, err
	}
	c, err := store.LoadCase()
	if err != nil {
		return Verification{}, err
	}
	artifacts := evidence.Store{CaseRoot: store.Root}
	var reasons []string
	for _, source := range c.Sources {
		if source.Integrity != "VERIFIED" || source.Completeness != "COMPLETE" {
			reasons = append(reasons, "SOURCE_INCOMPLETE:"+source.SourceID)
		}
		if err := verifySourceArtifacts(artifacts, source); err != nil {
			return Verification{}, err
		}
		mirrorPath, pathErr := store.Path(source.LocalMirrorLocator)
		if pathErr != nil {
			return Verification{}, pathErr
		}
		wantTree := extensionString(source.Extensions, "mirror_tree_fingerprint")
		if wantTree != "" {
			gotTree, treeErr := evidence.TreeFingerprint(mirrorPath)
			if treeErr != nil {
				return Verification{}, treeErr
			}
			if gotTree != wantTree {
				return Verification{}, fmt.Errorf("preserved source %s changed after inventory", source.SourceID)
			}
		}
		for _, ref := range source.Refs {
			if ref.ProposedMapping != nil {
				if _, checkErr := runner.Run(ctx, gitexec.ClassAnalysis, "", "check-ref-format", *ref.ProposedMapping); checkErr != nil {
					return Verification{}, fmt.Errorf("invalid proposed mapping for %s: %w", ref.OriginalName, checkErr)
				}
			}
		}
	}
	for _, item := range c.EvidenceItems {
		if item.RawArtifact != nil {
			if err := artifacts.Verify(*item.RawArtifact); err != nil {
				return Verification{}, err
			}
		}
	}
	if collisionCount(c.Sources) > 0 {
		reasons = append(reasons, "REF_MAPPING_COLLISION")
	}
	if reviewRefCount(c.Sources) > 0 {
		reasons = append(reasons, "REF_MAPPING_REVIEW_REQUIRED")
	}
	expectedRelationships := len(c.Sources) * max(0, len(c.Sources)-1)
	if len(c.Relationships) != expectedRelationships {
		reasons = append(reasons, "PAIRWISE_RELATIONSHIPS_MISSING")
	}
	if !hasEvidence(c.EvidenceItems, "evidence-ref-plan") {
		reasons = append(reasons, "REF_PLAN_MISSING")
	}
	for _, rel := range c.Relationships {
		if rel.Classification == "UNKNOWN" {
			reasons = append(reasons, "RELATIONSHIP_UNKNOWN:"+rel.RelationshipID)
		}
	}
	if err := store.ValidateJSONL(casefile.EventsFile); err != nil {
		return Verification{}, err
	}
	if err := store.ValidateJSONL(casefile.FindingsFile); err != nil {
		return Verification{}, err
	}
	setAnalysisAudit(&c, runner.Records())
	reasons = uniqueSorted(reasons)
	c.Status = "READY_FOR_REVIEW"
	if len(reasons) > 0 {
		c.Status = "INCOMPLETE"
	}
	if err := store.SaveCase(&c); err != nil {
		return Verification{}, err
	}
	raw, err := os.ReadFile(filepath.Join(store.Root, casefile.CaseFile))
	if err != nil {
		return Verification{}, err
	}
	if err := casefile.ValidateSchema(raw); err != nil {
		return Verification{}, err
	}
	if err := model.Validate(c); err != nil {
		return Verification{}, err
	}
	_ = store.AppendEvent(model.Event{EventID: eventID("verification"), Kind: "CASE_VERIFIED", Timestamp: now(), Actor: c.Operator.Identifier, Details: map[string]any{"status": c.Status, "reason_count": len(reasons)}})
	return Verification{Ready: len(reasons) == 0, Status: c.Status, Reasons: reasons}, nil
}

func (a App) Report() (string, error) {
	store, err := casefile.Open(a.CaseRoot)
	if err != nil {
		return "", err
	}
	c, err := store.LoadCase()
	if err != nil {
		return "", err
	}
	path, err := store.Path(casefile.ReportFile)
	if err != nil {
		return "", err
	}
	if err := report.Write(path, report.Render(c)); err != nil {
		return "", err
	}
	_ = store.AppendEvent(model.Event{EventID: eventID("report"), Kind: "REPORT_GENERATED", Timestamp: now(), Actor: c.Operator.Identifier, Details: map[string]any{"case_fingerprint": c.CaseFingerprint}})
	return path, nil
}

func (a App) Investigate(ctx context.Context, sources []string, operator string) (Verification, error) {
	store, err := casefile.Open(a.CaseRoot)
	if err != nil {
		return Verification{}, err
	}
	if _, err := os.Stat(filepath.Join(store.Root, casefile.PolicyFile)); errors.Is(err, os.ErrNotExist) {
		if _, err := store.Init("", operator); err != nil {
			return Verification{}, err
		}
	} else if err != nil {
		return Verification{}, err
	}
	for _, locator := range sources {
		if _, err := a.AddSource("", locator, "UNKNOWN", "auto"); err != nil {
			return Verification{}, err
		}
	}
	if _, err := a.Preserve(ctx); err != nil {
		return Verification{}, err
	}
	if _, err := a.Inspect(ctx); err != nil {
		return Verification{}, err
	}
	if _, err := a.Compare(); err != nil {
		return Verification{}, err
	}
	if _, err := a.PlanRefs(ctx); err != nil {
		return Verification{}, err
	}
	verification, err := a.Verify(ctx)
	if err != nil {
		return Verification{}, err
	}
	if _, err := a.Report(); err != nil {
		return Verification{}, err
	}
	return verification, nil
}

func (a App) runtime(ctx context.Context) (*casefile.Store, model.Policy, *gitexec.Runner, string, error) {
	store, err := casefile.Open(a.CaseRoot)
	if err != nil {
		return nil, model.Policy{}, nil, "", err
	}
	policy, err := store.LoadPolicy()
	if err != nil {
		return nil, model.Policy{}, nil, "", err
	}
	control, err := store.Path("control")
	if err != nil {
		return nil, model.Policy{}, nil, "", err
	}
	runner, err := gitexec.New(control, gitexec.ParseLimits(policy.Limits))
	if err != nil {
		return nil, model.Policy{}, nil, "", err
	}
	gitVersion, err := runner.Version(ctx)
	if err != nil {
		return nil, model.Policy{}, nil, "", err
	}
	return store, policy, runner, gitVersion, nil
}

func acquisitionSource(artifacts evidence.Store, declared model.PolicySource, acquired preserve.Result, acquisitionErr error) (model.Source, model.EvidenceItem, error) {
	emptyCommit, _, err := artifacts.PutLines("commits", []string{})
	if err != nil {
		return model.Source{}, model.EvidenceItem{}, err
	}
	emptyObjects, _, err := artifacts.PutLines("objects", []string{})
	if err != nil {
		return model.Source{}, model.EvidenceItem{}, err
	}
	emptyAll, _, err := artifacts.PutLines("all-objects", []string{})
	if err != nil {
		return model.Source{}, model.EvidenceItem{}, err
	}
	index := model.ManifestIndex{SchemaVersion: model.SchemaVersion, CommitManifest: emptyCommit, ReachableObjectManifest: emptyObjects, AllObjectManifest: emptyAll}
	indexPath, _, err := artifacts.PutJSON("manifest.json", index)
	if err != nil {
		return model.Source{}, model.EvidenceItem{}, err
	}
	stored := acquired.StoredLocator
	if stored == "" {
		_, normalized, normalizeErr := preserve.ClassifyLocator(declared.Locator, declared.Kind)
		if normalizeErr == nil {
			stored = normalized
		} else {
			stored = "unavailable-source-locator"
		}
	}
	retrievedAt := acquired.RetrievedAt
	if retrievedAt == "" {
		retrievedAt = now()
	}
	verification := "VERIFIED"
	status := "PRESERVED_NOT_INSPECTED"
	if acquisitionErr != nil {
		verification = "FAILED"
		status = "ACQUISITION_FAILED"
	}
	mirrorRel := acquired.MirrorRelativePath
	if mirrorRel == "" {
		mirrorRel = filepath.ToSlash(filepath.Join("sources", declared.SourceID+".git"))
	}
	digest := model.SHA256([]byte(declared.SourceID + "\x00" + status + "\x00" + acquired.MirrorFingerprint))
	source := model.Source{
		SourceID: declared.SourceID, OriginalLocator: stored, LocalMirrorLocator: mirrorRel,
		Role: declared.Role, DiscoveryMethod: "DECLARED", RetrievedAt: retrievedAt,
		RemoteMetadata: model.RemoteMetadata{FetchLocators: []string{stored}, PushDisabled: true},
		ObjectFormat:   "unknown", LFS: model.LFSState{ObjectsAvailable: "UNKNOWN"},
		Submodules: model.SubmoduleState{Entries: []string{}, Fetched: false},
		Integrity:  "NOT_CHECKED", Completeness: "UNKNOWN", Refs: []model.Ref{},
		Objects:           model.ObjectInventory{CommitSetDigest: model.DigestLines(nil), ReachableObjectSetDigest: model.DigestLines(nil), ManifestArtifact: indexPath},
		SourceFingerprint: digest,
		Extensions:        map[string]any{extensionNamespace: map[string]any{"acquisition_method": acquired.Method, "acquisition_status": status, "mirror_tree_fingerprint": acquired.MirrorFingerprint}},
	}
	item := model.EvidenceItem{
		EvidenceID: "evidence-acquisition-" + declared.SourceID, Producer: "git-casebook", Method: "controlled-git-mirror-acquisition",
		Inputs: []string{"source:" + declared.SourceID}, ObservedAt: retrievedAt, OutputFingerprint: digest,
		VerificationState: verification,
	}
	return source, item, nil
}

func defaultGates(networkRevoked bool, operator string) []model.Gate {
	changedAt := now()
	gates := []model.Gate{
		{GateID: "gate-network", Action: "NETWORK", State: "CLOSED", Scope: "after source acquisition", Reason: "Offline analysis forbids network access.", ChangedAt: changedAt},
		{GateID: "gate-source-execution", Action: "SOURCE_EXECUTION", State: "CLOSED", Scope: "all repository-controlled content", Reason: "v0.1 never executes inherited repository content.", ChangedAt: changedAt},
		{GateID: "gate-dependency-install", Action: "DEPENDENCY_INSTALL", State: "CLOSED", Scope: "all inherited dependencies", Reason: "Dependency preparation is outside v0.1.", ChangedAt: changedAt},
		{GateID: "gate-history-rewrite", Action: "HISTORY_REWRITE", State: "CLOSED", Scope: "all preserved sources", Reason: "History rewriting is outside v0.1.", ChangedAt: changedAt},
		{GateID: "gate-push", Action: "PUSH", State: "CLOSED", Scope: "all remotes", Reason: "v0.1 contains no push capability.", ChangedAt: changedAt},
		{GateID: "gate-publish", Action: "PUBLISH", State: "CLOSED", Scope: "all evidence and repositories", Reason: "Publication requires separate human review.", ChangedAt: changedAt},
		{GateID: "gate-delete-source", Action: "DELETE_SOURCE", State: "CLOSED", Scope: "all declared and preserved sources", Reason: "Source deletion is never implied by analysis.", ChangedAt: changedAt},
		{GateID: "gate-license-dependent-reuse", Action: "LICENSE_DEPENDENT_REUSE", State: "CLOSED", Scope: "all source-derived content", Reason: "The tool makes no ownership or licensing conclusion.", ChangedAt: changedAt},
	}
	if networkRevoked {
		gates[0].State = "REVOKED"
		gates[0].Authority = &operator
		gates[0].Reason = "Source acquisition ended; all provenance analysis is offline."
	}
	return gates
}

func verifySourceArtifacts(artifacts evidence.Store, source model.Source) error {
	if err := artifacts.Verify(source.Objects.ManifestArtifact); err != nil {
		return err
	}
	var index model.ManifestIndex
	if err := artifacts.ReadJSON(source.Objects.ManifestArtifact, &index); err != nil {
		return err
	}
	for _, path := range []string{index.CommitManifest, index.ReachableObjectManifest, index.AllObjectManifest} {
		if err := artifacts.Verify(path); err != nil {
			return err
		}
	}
	return verifyExtensionArtifacts(artifacts, source.Extensions)
}

func verifyExtensionArtifacts(artifacts evidence.Store, value any) error {
	switch current := value.(type) {
	case map[string]any:
		for _, child := range current {
			if err := verifyExtensionArtifacts(artifacts, child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range current {
			if err := verifyExtensionArtifacts(artifacts, child); err != nil {
				return err
			}
		}
	case string:
		if strings.HasPrefix(filepath.ToSlash(current), "artifacts/sha256/") {
			return artifacts.Verify(current)
		}
	}
	return nil
}

func findSource(sources []model.Source, id string) (model.Source, bool) {
	for _, source := range sources {
		if source.SourceID == id {
			return source, true
		}
	}
	return model.Source{}, false
}

func filterEvidence(items []model.EvidenceItem, prefix string) []model.EvidenceItem {
	result := make([]model.EvidenceItem, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item.EvidenceID, prefix) {
			result = append(result, item)
		}
	}
	return result
}

func filterFindings(items []model.Finding, prefix string) []model.Finding {
	result := make([]model.Finding, 0, len(items))
	for _, item := range items {
		if !strings.HasPrefix(item.FindingID, prefix) {
			result = append(result, item)
		}
	}
	return result
}

func extensionString(extensions map[string]any, key string) string {
	value, ok := extensions[extensionNamespace]
	if !ok {
		return ""
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	result, _ := fields[key].(string)
	return result
}

func setAnalysisAudit(c *model.Case, records []gitexec.Record) {
	count := 0
	for _, record := range records {
		if record.Class == gitexec.ClassAnalysis && record.NetworkCapable {
			count++
		}
	}
	if c.Extensions == nil {
		c.Extensions = map[string]any{}
	}
	fields, _ := c.Extensions[extensionNamespace].(map[string]any)
	if fields == nil {
		fields = map[string]any{}
	}
	fields["analysis_network_capable_command_count"] = count
	c.Extensions[extensionNamespace] = fields
}

func collisionCount(sources []model.Source) int {
	count := 0
	for _, source := range sources {
		for _, ref := range source.Refs {
			if ref.CollisionState == "COLLISION" {
				count++
			}
		}
	}
	return count
}

func reviewRefCount(sources []model.Source) int {
	count := 0
	for _, source := range sources {
		for _, ref := range source.Refs {
			if ref.ArchivalDisposition == "REVIEW" || ref.MappingState == "NOT_PLANNED" || ref.MappingState == "REJECTED" {
				count++
			}
		}
	}
	return count
}

func hasEvidence(items []model.EvidenceItem, id string) bool {
	for _, item := range items {
		if item.EvidenceID == id {
			return true
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func eventID(kind string) string {
	return "event-" + kind + "-" + fmt.Sprint(time.Now().UTC().UnixNano())
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func MarshalVerification(result Verification) []byte {
	b, _ := json.Marshal(result)
	return append(b, '\n')
}
