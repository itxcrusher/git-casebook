package inventory

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/itxcrusher/repo-rehab/internal/evidence"
	"github.com/itxcrusher/repo-rehab/internal/gitexec"
	"github.com/itxcrusher/repo-rehab/internal/model"
	"github.com/itxcrusher/repo-rehab/internal/preserve"
)

const extensionNamespace = "io.github.itxcrusher.repo-rehab"

type Data struct {
	Commits             []string
	ReachableObjects    []string
	AllObjects          []string
	CompletenessReasons []string
}

type objectMeta struct {
	OID  string
	Type string
	Size int64
}

func Analyze(ctx context.Context, runner *gitexec.Runner, artifacts evidence.Store, declared model.PolicySource, acquired preserve.Result) (model.Source, Data, model.EvidenceItem, error) {
	before, err := evidence.TreeFingerprint(acquired.MirrorPath)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, fmt.Errorf("fingerprint preserved mirror before analysis: %w", err)
	}
	reasons := []string{}
	integrity := "VERIFIED"
	objectFormat := "unknown"
	shallow := false
	partial := false
	var defaultBranch *string
	refs := []model.Ref{}

	if result, runErr := runner.Run(ctx, gitexec.ClassAnalysis, acquired.MirrorPath, "rev-parse", "--is-bare-repository"); runErr != nil || strings.TrimSpace(string(result.Stdout)) != "true" {
		integrity = "FAILED"
		reasons = append(reasons, "NOT_BARE_REPOSITORY")
	}
	if result, runErr := runner.Run(ctx, gitexec.ClassAnalysis, acquired.MirrorPath, "rev-parse", "--show-object-format"); runErr == nil {
		candidate := strings.TrimSpace(string(result.Stdout))
		if candidate == "sha1" || candidate == "sha256" {
			objectFormat = candidate
		} else {
			reasons = append(reasons, "UNSUPPORTED_OBJECT_FORMAT")
		}
	} else {
		reasons = append(reasons, "OBJECT_FORMAT_UNAVAILABLE")
	}
	if result, runErr := runner.Run(ctx, gitexec.ClassAnalysis, acquired.MirrorPath, "rev-parse", "--is-shallow-repository"); runErr == nil {
		shallow = strings.TrimSpace(string(result.Stdout)) == "true"
		if shallow {
			reasons = append(reasons, "SHALLOW_SOURCE")
		}
	} else {
		reasons = append(reasons, "SHALLOW_STATE_UNKNOWN")
	}
	partial = detectPartialClone(ctx, runner, acquired.MirrorPath)
	if partial {
		reasons = append(reasons, "PARTIAL_OR_PROMISOR_SOURCE")
	}
	if hasNonEmptyFile(filepath.Join(acquired.MirrorPath, "objects", "info", "alternates")) {
		reasons = append(reasons, "ALTERNATE_OBJECT_STORE")
	}
	if hasNonEmptyFile(filepath.Join(acquired.MirrorPath, "info", "grafts")) {
		reasons = append(reasons, "GRAFTS_PRESENT")
	}
	if result, runErr := runner.Run(ctx, gitexec.ClassAnalysis, acquired.MirrorPath, "symbolic-ref", "-q", "HEAD"); runErr == nil {
		value := strings.TrimSpace(string(result.Stdout))
		if value != "" {
			defaultBranch = &value
		}
	}

	refResult, refErr := runner.Run(ctx, gitexec.ClassAnalysis, acquired.MirrorPath,
		"for-each-ref", "--format=%(refname)%00%(objectname)%00%(*objectname)%00%(objecttype)%00%(*objecttype)")
	if refErr != nil {
		integrity = "FAILED"
		reasons = append(reasons, "REF_INVENTORY_FAILED")
	} else {
		refs, err = parseRefs(refResult.Stdout)
		if err != nil {
			integrity = "FAILED"
			reasons = append(reasons, "REF_INVENTORY_INVALID")
		}
	}

	fsckResult, fsckErr := runner.Run(ctx, gitexec.ClassAnalysis, acquired.MirrorPath, "fsck", "--full", "--no-reflogs", "--no-dangling")
	fsckData := append(append([]byte(nil), fsckResult.Stdout...), fsckResult.Stderr...)
	fsckArtifact, _, artifactErr := artifacts.PutBytes("fsck.txt", fsckData)
	if artifactErr != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, artifactErr
	}
	if fsckErr != nil {
		integrity = "FAILED"
		reasons = append(reasons, "FSCK_FAILED")
	}

	commitRoots, objectRoots := traversalRoots(refs)
	commits, commitErr := revList(ctx, runner, acquired.MirrorPath, false, commitRoots)
	if commitErr != nil {
		integrity = "FAILED"
		reasons = append(reasons, "COMMIT_TRAVERSAL_FAILED")
		commits = []string{}
	}
	reachable, objectErr := revList(ctx, runner, acquired.MirrorPath, true, objectRoots)
	if objectErr != nil {
		integrity = "FAILED"
		reasons = append(reasons, "OBJECT_TRAVERSAL_FAILED")
		reachable = []string{}
	}
	allObjects, metadata, allErr := enumerateAllObjects(ctx, runner, acquired.MirrorPath)
	if allErr != nil {
		integrity = "FAILED"
		reasons = append(reasons, "ALL_OBJECT_ENUMERATION_FAILED")
		allObjects = []string{}
		metadata = nil
	}
	if len(commits) == 0 {
		reasons = append(reasons, "EMPTY_COMMIT_SET")
	}

	submodules, lfs, externalReasons := inspectExternalMaterial(ctx, runner, acquired.MirrorPath, commitRoots, reachable, metadata)
	reasons = append(reasons, externalReasons...)
	reasons = uniqueSorted(reasons)
	completeness := "COMPLETE"
	if integrity != "VERIFIED" || len(reasons) > 0 {
		completeness = "INCOMPLETE"
	}

	commitPath, _, err := artifacts.PutLines("commits", commits)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, err
	}
	reachablePath, _, err := artifacts.PutLines("objects", reachable)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, err
	}
	allPath, allDigest, err := artifacts.PutLines("all-objects", allObjects)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, err
	}
	index := model.ManifestIndex{SchemaVersion: model.SchemaVersion, CommitManifest: commitPath, ReachableObjectManifest: reachablePath, AllObjectManifest: allPath}
	indexPath, _, err := artifacts.PutJSON("manifest.json", index)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, err
	}

	source := model.Source{
		SourceID: declared.SourceID, OriginalLocator: acquired.StoredLocator,
		LocalMirrorLocator: acquired.MirrorRelativePath, Role: declared.Role,
		DiscoveryMethod: "DECLARED", RetrievedAt: acquired.RetrievedAt,
		DefaultBranch:  defaultBranch,
		RemoteMetadata: model.RemoteMetadata{FetchLocators: []string{acquired.StoredLocator}, PushDisabled: true},
		ObjectFormat:   objectFormat, Shallow: shallow, PartialClone: partial,
		LFS: lfs, Submodules: submodules, Integrity: integrity, Completeness: completeness,
		Refs: refs,
		Objects: model.ObjectInventory{
			CommitCount: len(commits), ReachableObjectCount: len(reachable), AllObjectCount: len(allObjects),
			CommitSetDigest: model.DigestLines(commits), ReachableObjectSetDigest: model.DigestLines(reachable),
			ManifestArtifact: indexPath, UnreachableObjectCount: differenceCount(allObjects, reachable),
		},
		Extensions: map[string]any{extensionNamespace: map[string]any{
			"acquisition_method":      acquired.Method,
			"mirror_tree_fingerprint": before,
			"all_object_set_digest":   allDigest,
			"fsck_artifact":           fsckArtifact,
			"completeness_reasons":    reasons,
		}},
	}
	source.SourceFingerprint, err = sourceFingerprint(source)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, err
	}
	after, err := evidence.TreeFingerprint(acquired.MirrorPath)
	if err != nil {
		return model.Source{}, Data{}, model.EvidenceItem{}, fmt.Errorf("fingerprint preserved mirror after analysis: %w", err)
	}
	if before != after {
		return model.Source{}, Data{}, model.EvidenceItem{}, fmt.Errorf("preserved source %s was mutated during analysis", declared.SourceID)
	}
	verification := "VERIFIED"
	if integrity == "FAILED" {
		verification = "FAILED"
	} else if completeness != "COMPLETE" {
		verification = "INCOMPLETE"
	}
	evidenceItem := model.EvidenceItem{
		EvidenceID: "evidence-inventory-" + declared.SourceID,
		Producer:   "repo-rehab", Method: "native-git-ref-integrity-reachability-inventory",
		Inputs:     []string{"source:" + declared.SourceID, "all-policy-included-refs"},
		ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), OutputFingerprint: source.SourceFingerprint,
		RawArtifact: &indexPath, VerificationState: verification,
	}
	return source, Data{Commits: commits, ReachableObjects: reachable, AllObjects: allObjects, CompletenessReasons: reasons}, evidenceItem, nil
}

func LoadData(artifacts evidence.Store, source model.Source) (Data, error) {
	var index model.ManifestIndex
	if err := artifacts.ReadJSON(source.Objects.ManifestArtifact, &index); err != nil {
		return Data{}, err
	}
	commits, err := artifacts.ReadLines(index.CommitManifest)
	if err != nil {
		return Data{}, err
	}
	reachable, err := artifacts.ReadLines(index.ReachableObjectManifest)
	if err != nil {
		return Data{}, err
	}
	allObjects, err := artifacts.ReadLines(index.AllObjectManifest)
	if err != nil {
		return Data{}, err
	}
	return Data{Commits: commits, ReachableObjects: reachable, AllObjects: allObjects}, nil
}

func parseRefs(output []byte) ([]model.Ref, error) {
	lines := bytes.Split(output, []byte{'\n'})
	refs := make([]model.Ref, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		fields := bytes.Split(line, []byte{0})
		if len(fields) != 5 {
			return nil, fmt.Errorf("unexpected for-each-ref record")
		}
		for _, field := range fields {
			if !utf8.Valid(field) {
				return nil, fmt.Errorf("ref inventory contains non-UTF-8 data")
			}
		}
		name, oid := string(fields[0]), string(fields[1])
		if !model.IsOID(oid) {
			return nil, fmt.Errorf("ref %q has invalid object id", name)
		}
		var peeled *string
		if len(fields[2]) > 0 {
			value := string(fields[2])
			if !model.IsOID(value) {
				return nil, fmt.Errorf("ref %q has invalid peeled object id", name)
			}
			peeled = &value
		}
		refs = append(refs, model.Ref{
			OriginalName: name, Type: refType(name), ObjectID: oid, PeeledObjectID: peeled,
			ProposedMapping: nil, MappingState: "NOT_PLANNED", VerifiedDestinationObjectID: nil,
			CollisionState: "NOT_PLANNED", ArchivalDisposition: "PRESERVE",
		})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].OriginalName < refs[j].OriginalName })
	return refs, nil
}

func refType(name string) string {
	switch {
	case strings.HasPrefix(name, "refs/heads/"):
		return "BRANCH"
	case strings.HasPrefix(name, "refs/tags/"):
		return "TAG"
	case strings.HasPrefix(name, "refs/remotes/"):
		return "REMOTE"
	case strings.HasPrefix(name, "refs/pull/") || strings.HasPrefix(name, "refs/merge-requests/"):
		return "PULL"
	case strings.HasPrefix(name, "refs/notes/"):
		return "NOTE"
	case strings.HasPrefix(name, "refs/replace/"):
		return "REPLACE"
	default:
		return "OTHER"
	}
}

func traversalRoots(refs []model.Ref) ([]string, []string) {
	commitRoots := []string{}
	objectRoots := []string{}
	for _, ref := range refs {
		if ref.Type == "REPLACE" {
			continue
		}
		root := ref.ObjectID
		objectRoots = append(objectRoots, root)
		if ref.PeeledObjectID != nil {
			commitRoots = append(commitRoots, *ref.PeeledObjectID)
		} else {
			commitRoots = append(commitRoots, root)
		}
	}
	return uniqueSorted(commitRoots), uniqueSorted(objectRoots)
}

func revList(ctx context.Context, runner *gitexec.Runner, repo string, objects bool, roots []string) ([]string, error) {
	if len(roots) == 0 {
		return []string{}, nil
	}
	args := []string{"rev-list"}
	if objects {
		args = append(args, "--objects")
	}
	args = append(args, "--no-object-names", "--missing=error", "--stdin")
	input := []byte(strings.Join(roots, "\n") + "\n")
	result, err := runner.RunInput(ctx, gitexec.ClassAnalysis, repo, input, args...)
	if err != nil {
		return nil, err
	}
	values := strings.Fields(string(result.Stdout))
	for _, value := range values {
		if !model.IsOID(value) {
			return nil, fmt.Errorf("rev-list returned invalid object id")
		}
	}
	return uniqueSorted(values), nil
}

func enumerateAllObjects(ctx context.Context, runner *gitexec.Runner, repo string) ([]string, []objectMeta, error) {
	result, err := runner.Run(ctx, gitexec.ClassAnalysis, repo, "cat-file", "--batch-all-objects", "--batch-check=%(objectname) %(objecttype) %(objectsize)")
	if err != nil {
		return nil, nil, err
	}
	var objects []string
	var metadata []objectMeta
	for _, line := range bytes.Split(result.Stdout, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) != 3 {
			return nil, nil, fmt.Errorf("unexpected cat-file object record")
		}
		oid := string(fields[0])
		size, sizeErr := strconv.ParseInt(string(fields[2]), 10, 64)
		if !model.IsOID(oid) || sizeErr != nil || size < 0 {
			return nil, nil, fmt.Errorf("invalid cat-file object record")
		}
		objects = append(objects, oid)
		metadata = append(metadata, objectMeta{OID: oid, Type: string(fields[1]), Size: size})
	}
	sort.Strings(objects)
	sort.Slice(metadata, func(i, j int) bool { return metadata[i].OID < metadata[j].OID })
	return objects, metadata, nil
}

func detectPartialClone(ctx context.Context, runner *gitexec.Runner, repo string) bool {
	result, err := runner.Run(ctx, gitexec.ClassAnalysis, repo, "config", "--local", "--get-regexp", `^(remote\..*\.promisor|extensions\.partialClone)$`)
	if err == nil {
		return len(bytes.TrimSpace(result.Stdout)) > 0
	}
	var commandErr *gitexec.CommandError
	return !errors.As(err, &commandErr) || commandErr.ExitCode != 1
}

func inspectExternalMaterial(ctx context.Context, runner *gitexec.Runner, repo string, commitRoots, reachable []string, metadata []objectMeta) (model.SubmoduleState, model.LFSState, []string) {
	submodules := model.SubmoduleState{Entries: []string{}, Fetched: false}
	lfs := model.LFSState{ObjectsAvailable: "NOT_APPLICABLE"}
	reasons := []string{}
	attributeBlobs := map[string]bool{}
	moduleBlobs := map[string]bool{}
	gitlinks := map[string]bool{}
	for _, root := range commitRoots {
		result, err := runner.Run(ctx, gitexec.ClassAnalysis, repo, "ls-tree", "-r", "-z", "--full-tree", root)
		if err != nil {
			continue
		}
		for _, record := range bytes.Split(result.Stdout, []byte{0}) {
			if len(record) == 0 {
				continue
			}
			meta, pathBytes, ok := bytes.Cut(record, []byte{'\t'})
			if !ok || !utf8.Valid(pathBytes) {
				continue
			}
			fields := strings.Fields(string(meta))
			if len(fields) != 3 {
				continue
			}
			path := string(pathBytes)
			if fields[0] == "160000" {
				gitlinks[path] = true
			}
			if path == ".gitmodules" {
				moduleBlobs[fields[2]] = true
			}
			if path == ".gitattributes" || strings.HasSuffix(path, "/.gitattributes") {
				attributeBlobs[fields[2]] = true
			}
		}
	}
	for path := range gitlinks {
		submodules.Entries = append(submodules.Entries, safeEntry(path))
	}
	for oid := range moduleBlobs {
		content, err := readBlob(ctx, runner, repo, oid)
		if err == nil && len(content) > 0 {
			submodules.Declared = true
		}
	}
	sort.Strings(submodules.Entries)
	if len(gitlinks) > 0 {
		submodules.Declared = true
	}
	for oid := range attributeBlobs {
		content, err := readBlob(ctx, runner, repo, oid)
		if err == nil && bytes.Contains(content, []byte("filter=lfs")) {
			lfs.Declared = true
		}
	}
	reachableSet := make(map[string]bool, len(reachable))
	for _, oid := range reachable {
		reachableSet[oid] = true
	}
	var candidates []objectMeta
	for _, item := range metadata {
		if item.Type == "blob" && item.Size > 0 && item.Size <= 1024 && reachableSet[item.OID] {
			candidates = append(candidates, item)
		}
	}
	if len(candidates) > 10000 {
		reasons = append(reasons, "LFS_POINTER_SCAN_LIMIT")
		lfs.ObjectsAvailable = "UNKNOWN"
		return submodules, lfs, reasons
	}
	pointers, unavailable, err := scanLFSPointers(ctx, runner, repo, candidates)
	if err != nil {
		reasons = append(reasons, "LFS_POINTER_SCAN_FAILED")
		lfs.ObjectsAvailable = "UNKNOWN"
		return submodules, lfs, reasons
	}
	lfs.PointerCount = pointers
	if pointers > 0 {
		lfs.Declared = true
		if unavailable > 0 {
			lfs.ObjectsAvailable = "NO"
			reasons = append(reasons, "LFS_OBJECTS_UNAVAILABLE")
		} else {
			lfs.ObjectsAvailable = "YES"
		}
	} else if lfs.Declared {
		lfs.ObjectsAvailable = "UNKNOWN"
	} else {
		lfs.ObjectsAvailable = "NOT_APPLICABLE"
	}
	return submodules, lfs, reasons
}

func scanLFSPointers(ctx context.Context, runner *gitexec.Runner, repo string, candidates []objectMeta) (int, int, error) {
	if len(candidates) == 0 {
		return 0, 0, nil
	}
	oids := make([]string, len(candidates))
	for i, candidate := range candidates {
		oids[i] = candidate.OID
	}
	result, err := runner.RunInput(ctx, gitexec.ClassAnalysis, repo, []byte(strings.Join(oids, "\n")+"\n"), "cat-file", "--batch")
	if err != nil {
		return 0, 0, err
	}
	reader := bufio.NewReader(bytes.NewReader(result.Stdout))
	pointers, unavailable := 0, 0
	for range candidates {
		header, err := reader.ReadString('\n')
		if err != nil {
			return 0, 0, err
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return 0, 0, fmt.Errorf("unexpected cat-file batch header")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size < 0 || size > 1024 {
			return 0, 0, fmt.Errorf("invalid cat-file batch size")
		}
		content := make([]byte, size)
		if _, err := io.ReadFull(reader, content); err != nil {
			return 0, 0, err
		}
		if trailing, err := reader.ReadByte(); err != nil || trailing != '\n' {
			return 0, 0, fmt.Errorf("invalid cat-file batch terminator")
		}
		lfsOID, ok := parseLFSPointer(content)
		if !ok {
			continue
		}
		pointers++
		path := filepath.Join(repo, "lfs", "objects", lfsOID[:2], lfsOID[2:4], lfsOID)
		if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
			unavailable++
		}
	}
	return pointers, unavailable, nil
}

func parseLFSPointer(content []byte) (string, bool) {
	text := string(content)
	if !strings.HasPrefix(text, "version https://git-lfs.github.com/spec/v1\n") {
		return "", false
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "oid sha256:") {
			oid := strings.TrimPrefix(line, "oid sha256:")
			if len(oid) == 64 {
				for _, r := range oid {
					if !strings.ContainsRune("0123456789abcdef", r) {
						return "", false
					}
				}
				return oid, true
			}
		}
	}
	return "", false
}

func readBlob(ctx context.Context, runner *gitexec.Runner, repo, oid string) ([]byte, error) {
	result, err := runner.Run(ctx, gitexec.ClassAnalysis, repo, "cat-file", "blob", oid)
	if err != nil {
		return nil, err
	}
	return result.Stdout, nil
}

func sourceFingerprint(source model.Source) (string, error) {
	refs := make([]struct {
		Name   string  `json:"name"`
		Type   string  `json:"type"`
		OID    string  `json:"oid"`
		Peeled *string `json:"peeled"`
	}, len(source.Refs))
	for i, ref := range source.Refs {
		refs[i] = struct {
			Name   string  `json:"name"`
			Type   string  `json:"type"`
			OID    string  `json:"oid"`
			Peeled *string `json:"peeled"`
		}{ref.OriginalName, ref.Type, ref.ObjectID, ref.PeeledObjectID}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name < refs[j].Name })
	projection := struct {
		DefaultBranch *string               `json:"default_branch"`
		ObjectFormat  string                `json:"object_format"`
		Shallow       bool                  `json:"shallow"`
		Partial       bool                  `json:"partial"`
		LFS           model.LFSState        `json:"lfs"`
		Submodules    model.SubmoduleState  `json:"submodules"`
		Integrity     string                `json:"integrity"`
		Completeness  string                `json:"completeness"`
		Refs          any                   `json:"refs"`
		Objects       model.ObjectInventory `json:"objects"`
		Reasons       []string              `json:"completeness_reasons"`
	}{source.DefaultBranch, source.ObjectFormat, source.Shallow, source.PartialClone, source.LFS, source.Submodules, source.Integrity, source.Completeness, refs, source.Objects, sourceReasons(source)}
	b, err := json.Marshal(projection)
	if err != nil {
		return "", err
	}
	return model.SHA256(b), nil
}

func sourceReasons(source model.Source) []string {
	value, ok := source.Extensions[extensionNamespace]
	if !ok {
		return []string{}
	}
	fields, ok := value.(map[string]any)
	if !ok {
		return []string{}
	}
	if values, ok := fields["completeness_reasons"].([]string); ok {
		return uniqueSorted(values)
	}
	values, ok := fields["completeness_reasons"].([]any)
	if !ok {
		return []string{}
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return uniqueSorted(result)
}

func differenceCount(all, subset []string) int {
	set := make(map[string]bool, len(subset))
	for _, value := range subset {
		set[value] = true
	}
	count := 0
	for _, value := range all {
		if !set[value] {
			count++
		}
	}
	return count
}

func hasNonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func safeEntry(value string) string {
	value = strings.ToValidUTF8(value, "�")
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '�'
		}
		return r
	}, value)
	if len(value) > 4096 {
		value = value[:4096]
	}
	return value
}
