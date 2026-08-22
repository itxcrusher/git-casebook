package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/itxcrusher/git-casebook/internal/app"
	"github.com/itxcrusher/git-casebook/internal/casefile"
	"github.com/itxcrusher/git-casebook/internal/evidence"
	"github.com/itxcrusher/git-casebook/internal/model"
	"github.com/itxcrusher/git-casebook/internal/refplan"
	"github.com/itxcrusher/git-casebook/test/fixtures"
)

func TestSyntheticFixtureCorpus(t *testing.T) {
	slots := make(chan struct{}, 4)
	runFixture := func(name string, fn func(*testing.T)) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			slots <- struct{}{}
			defer func() { <-slots }()
			fn(t)
		})
	}
	runFixture("F01_exact_duplicate", testF01)
	runFixture("F02_simple_superset", testF02)
	runFixture("F03_diverged_siblings", testF03)
	runFixture("F04_unrelated_histories", testF04)
	runFixture("F05_branch_only_recovered_work", testF05)
	runFixture("F06_synthetic_pull_merge_refs", testF06)
	runFixture("F07_ref_name_collision", testF07)
	runFixture("F08_tags", testF08)
	runFixture("F09_misleading_default", testF09)
	runFixture("F10_shallow_clone", testF10)
	runFixture("F11_corrupt_repository", testF11)
	runFixture("F12_submodule", testF12)
	runFixture("F13_git_lfs_pointer", testF13)
	runFixture("F14_unusual_nested_refs", testF14)
	runFixture("F15_three_source_ambiguity", testF15)
}

func testF01(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "exact-a")
	a.Commit("README.md", "synthetic exact\n", "base")
	b := fixtures.Clone(t, root, "exact-b", a)
	c, verification, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "EXACT")
	wantRelationship(t, c, "source-02", "source-01", "EXACT")
	if !verification.Ready {
		t.Fatalf("exact case not ready: %+v", verification)
	}
}

func testF02(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "subset-a")
	a.Commit("base.txt", "base\n", "base")
	b := fixtures.Clone(t, root, "superset-b", a)
	b.Commit("new.txt", "new\n", "additional work")
	c, _, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "SUBSET")
	wantRelationship(t, c, "source-02", "source-01", "SUPERSET")
}

func testF03(t *testing.T) {
	root := t.TempDir()
	base := fixtures.New(t, root, "diverged-base")
	base.Commit("base.txt", "base\n", "base")
	a := fixtures.Clone(t, root, "diverged-a", base)
	b := fixtures.Clone(t, root, "diverged-b", base)
	a.Commit("a.txt", "a\n", "a work")
	b.Commit("b.txt", "b\n", "b work")
	c, _, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "DIVERGED")
	wantRelationship(t, c, "source-02", "source-01", "DIVERGED")
}

func testF04(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "disjoint-a")
	b := fixtures.New(t, root, "disjoint-b")
	a.Commit("a.txt", "a\n", "independent a")
	b.Commit("b.txt", "b\n", "independent b")
	c, _, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "DISJOINT")
	wantRelationship(t, c, "source-02", "source-01", "DISJOINT")
}

func testF05(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "branch-a")
	a.Commit("base.txt", "base\n", "base")
	b := fixtures.Clone(t, root, "branch-b", a)
	b.Checkout("feature/recovered", true)
	b.Commit("feature.txt", "branch only\n", "branch-only work")
	b.Checkout("main", false)
	c, _, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "SUBSET")
	if !hasRef(c, "source-02", "refs/heads/feature/recovered") {
		t.Fatal("branch-only work was not inventoried")
	}
}

func testF06(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "pull-a")
	base := a.Commit("base.txt", "base\n", "base")
	b := fixtures.Clone(t, root, "pull-b", a)
	b.Checkout("feature", true)
	head := b.Commit("feature.txt", "feature\n", "pull head")
	b.Checkout("main", false)
	merge := b.MergeNoFF("feature", "synthetic pull merge")
	b.ResetHard(base)
	b.UpdateRef("refs/pull/17/head", head)
	b.UpdateRef("refs/pull/17/merge", merge)
	b.Run("-C", b.Path, "branch", "-D", "feature")
	c, _, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "SUBSET")
	for _, name := range []string{"refs/pull/17/head", "refs/pull/17/merge"} {
		ref := getRef(t, c, "source-02", name)
		if ref.ProposedMapping == nil || strings.HasPrefix(*ref.ProposedMapping, "refs/pull/") {
			t.Fatalf("provider-managed ref was not safely remapped: %+v", ref)
		}
	}
}

func testF07(t *testing.T) {
	destination := "refs/heads/archive/source-01/heads/collision"
	sources := []model.Source{{SourceID: "source-01", Refs: []model.Ref{
		{OriginalName: "refs/heads/collision", ObjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ProposedMapping: &destination, MappingState: "PROPOSED", CollisionState: "NONE"},
		{OriginalName: "refs/remotes/origin/collision", ObjectID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ProposedMapping: &destination, MappingState: "PROPOSED", CollisionState: "NONE"},
	}}}
	if refplan.DetectCollisions(sources) != 1 {
		t.Fatal("synthetic mapping collision was not detected")
	}
	for _, ref := range sources[0].Refs {
		if ref.ProposedMapping != nil || ref.CollisionState != "COLLISION" {
			t.Fatal("colliding destination was not withheld")
		}
	}
}

func testF08(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "tags-a")
	base := a.Commit("base.txt", "base\n", "base")
	a.TagLightweight("shared-light", base)
	b := fixtures.Clone(t, root, "tags-b", a)
	newCommit := b.Commit("new.txt", "new\n", "tagged work")
	b.TagAnnotated("unique-annotated", newCommit)
	b.TagLightweight("unique-light", newCommit)
	c, _, _ := investigate(t, root, a.Path, b.Path)
	wantRelationship(t, c, "source-01", "source-02", "SUBSET")
	annotated := getRef(t, c, "source-02", "refs/tags/unique-annotated")
	if annotated.PeeledObjectID == nil {
		t.Fatal("annotated tag was not peeled")
	}
	if getRef(t, c, "source-02", "refs/tags/shared-light").PeeledObjectID != nil {
		t.Fatal("lightweight tag unexpectedly has a peeled object")
	}
}

func testF09(t *testing.T) {
	root := t.TempDir()
	repo := fixtures.New(t, root, "misleading-default")
	repo.Commit("README.md", "placeholder\n", "placeholder")
	repo.Checkout("development", true)
	repo.Commit("src/product.txt", "actual project\n", "project work")
	repo.Checkout("main", false)
	c, _, _ := investigate(t, root, repo.Path)
	if c.Sources[0].DefaultBranch == nil || *c.Sources[0].DefaultBranch != "refs/heads/main" {
		t.Fatalf("unexpected default branch: %+v", c.Sources[0].DefaultBranch)
	}
	if !hasRef(c, "source-01", "refs/heads/development") || c.Sources[0].Objects.CommitCount != 2 {
		t.Fatal("richer non-default history was not surfaced")
	}
}

func testF10(t *testing.T) {
	root := t.TempDir()
	complete := fixtures.New(t, root, "complete")
	complete.Commit("one.txt", "one\n", "one")
	complete.Commit("two.txt", "two\n", "two")
	complete.Commit("three.txt", "three\n", "three")
	shallow := fixtures.ShallowBareClone(t, root, "shallow.git", complete)
	c, verification, _ := investigate(t, root, complete.Path, shallow.Path)
	if !c.Sources[1].Shallow || c.Sources[1].Completeness != "INCOMPLETE" {
		t.Fatalf("shallow source not marked incomplete: %+v", c.Sources[1])
	}
	wantRelationship(t, c, "source-01", "source-02", "UNKNOWN")
	if verification.Ready {
		t.Fatal("shallow case must not be ready")
	}
}

func testF11(t *testing.T) {
	root := t.TempDir()
	complete := fixtures.New(t, root, "corrupt-base")
	complete.Commit("base.txt", "base\n", "base")
	broken := fixtures.Clone(t, root, "corrupt", complete)
	brokenOID := broken.Commit("broken.txt", "broken\n", "commit to corrupt")
	fixtures.CorruptLooseObject(t, broken, brokenOID)
	c, verification, _ := investigate(t, root, complete.Path, broken.Path)
	wantRelationship(t, c, "source-01", "source-02", "UNKNOWN")
	if verification.Ready {
		t.Fatal("corrupt case must not be ready")
	}
	if c.Sources[1].Integrity == "VERIFIED" {
		t.Fatal("corrupt source was incorrectly trusted")
	}
}

func testF12(t *testing.T) {
	root := t.TempDir()
	child := fixtures.New(t, root, "child")
	childOID := child.Commit("child.txt", "child\n", "child")
	parent := fixtures.New(t, root, "parent")
	fixtures.WriteFile(t, parent, ".gitmodules", "[submodule \"vendor/child\"]\n\tpath = vendor/child\n\turl = https://example.invalid/child.git\n")
	parent.Run("-C", parent.Path, "add", ".gitmodules")
	fixtures.Gitlink(t, parent, "vendor/child", childOID)
	parent.Run("-C", parent.Path, "commit", "-m", "declare synthetic submodule")
	c, _, _ := investigate(t, root, parent.Path)
	if !c.Sources[0].Submodules.Declared || len(c.Sources[0].Submodules.Entries) != 1 || c.Sources[0].Submodules.Fetched {
		t.Fatalf("submodule state incorrect: %+v", c.Sources[0].Submodules)
	}
}

func testF13(t *testing.T) {
	root := t.TempDir()
	repo := fixtures.New(t, root, "lfs-pointer")
	fixtures.WriteFile(t, repo, ".gitattributes", "*.bin filter=lfs diff=lfs merge=lfs -text\n")
	fixtures.WriteFile(t, repo, "sample.bin", "version https://git-lfs.github.com/spec/v1\noid sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\nsize 123\n")
	repo.CommitAll("synthetic lfs pointer")
	c, verification, _ := investigate(t, root, repo.Path)
	lfs := c.Sources[0].LFS
	if !lfs.Declared || lfs.PointerCount != 1 || lfs.ObjectsAvailable != "NO" {
		t.Fatalf("LFS pointer state incorrect: %+v", lfs)
	}
	if verification.Ready {
		t.Fatal("case with unavailable LFS content must remain incomplete")
	}
}

func testF14(t *testing.T) {
	root := t.TempDir()
	repo := fixtures.New(t, root, "unusual-refs")
	oid := repo.Commit("base.txt", "base\n", "base")
	repo.UpdateRef("refs/heads/feature/nested", oid)
	repo.UpdateRef("refs/custom/review/v1.2+safe", oid)
	repo.UpdateRef("refs/notes/review", oid)
	c, _, _ := investigate(t, root, repo.Path)
	destinations := map[string]bool{}
	for _, ref := range c.Sources[0].Refs {
		if ref.ProposedMapping == nil {
			t.Fatalf("unusual ref mapping was withheld without collision: %+v", ref)
		}
		if destinations[*ref.ProposedMapping] {
			t.Fatalf("duplicate mapping %s", *ref.ProposedMapping)
		}
		destinations[*ref.ProposedMapping] = true
	}
}

func testF15(t *testing.T) {
	root := t.TempDir()
	a := fixtures.New(t, root, "old-a")
	a.Commit("base.txt", "base\n", "base")
	b := fixtures.Clone(t, root, "superset-b", a)
	b.Commit("b.txt", "b\n", "b work")
	cRepo := fixtures.Clone(t, root, "diverged-c", a)
	cRepo.Commit("c.txt", "c\n", "c work")
	c, _, _ := investigate(t, root, a.Path, b.Path, cRepo.Path)
	wantRelationship(t, c, "source-01", "source-02", "SUBSET")
	wantRelationship(t, c, "source-01", "source-03", "SUBSET")
	wantRelationship(t, c, "source-02", "source-03", "DIVERGED")
}

func TestTrapFixturesDoNotExecuteInheritedBehavior(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repo := fixtures.New(t, root, "traps")
	sentinel := filepath.Join(root, "inherited-behavior-ran")
	hookPath := filepath.Join(repo.Path, ".git", "hooks", "post-checkout")
	hook := "#!/bin/sh\nprintf trap > '" + filepath.ToSlash(sentinel) + "'\n"
	if err := os.WriteFile(hookPath, []byte(hook), 0o700); err != nil {
		t.Fatal(err)
	}
	fixtures.WriteFile(t, repo, "package.json", `{"scripts":{"postinstall":"printf trap"}}`+"\n")
	fixtures.WriteFile(t, repo, "run.sh", hook)
	fixtures.WriteFile(t, repo, ".gitmodules", "[submodule \"trap\"]\n\tpath = trap\n\turl = ext::sh -c printf-trap\n")
	fixtures.WriteFile(t, repo, ".gitattributes", "*.trap filter=sentinel diff=sentinel\n")
	repo.CommitAll("harmless trap declarations")
	repo.Run("-C", repo.Path, "config", "filter.sentinel.smudge", "sh -c printf-trap")
	repo.Run("-C", repo.Path, "config", "diff.sentinel.textconv", "sh -c printf-trap")
	before, err := evidence.TreeFingerprint(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	c, _, caseRoot := investigate(t, root, repo.Path)
	after, err := evidence.TreeFingerprint(repo.Path)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("input repository changed during investigation")
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("an inherited hook, script, filter, or textconv created the sentinel")
	}
	mirror := filepath.Join(caseRoot, filepath.FromSlash(c.Sources[0].LocalMirrorLocator))
	wantMirror := extensionString(t, c.Sources[0], "mirror_tree_fingerprint")
	gotMirror, err := evidence.TreeFingerprint(mirror)
	if err != nil {
		t.Fatal(err)
	}
	if gotMirror != wantMirror {
		t.Fatal("preserved mirror changed during analysis")
	}
}

func TestDeterministicSemanticFingerprintAcrossCasePaths(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repo := fixtures.New(t, root, "deterministic")
	repo.Commit("base.txt", "base\n", "base")
	c1, _, _ := investigateAt(t, filepath.Join(root, "case-one"), repo.Path)
	c2, _, _ := investigateAt(t, filepath.Join(root, "case-two"), repo.Path)
	if c1.CaseID == c2.CaseID {
		t.Fatal("case IDs should be independent metadata")
	}
	if c1.CreatedAt == c2.CreatedAt {
		t.Log("created timestamps happened to match; semantic comparison remains valid")
	}
	if c1.CaseFingerprint != c2.CaseFingerprint {
		t.Fatalf("semantic fingerprints differ: %s != %s", c1.CaseFingerprint, c2.CaseFingerprint)
	}
}

func TestFileURLAcquisitionRevokesNetworkBeforeAnalysis(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repo := fixtures.New(t, root, "file-remote")
	repo.Commit("base.txt", "base\n", "base")
	c, verification, _ := investigate(t, root, fixtures.FileURL(repo.Path))
	if !verification.Ready {
		t.Fatalf("file URL case not ready: %+v", verification)
	}
	wantGate := false
	for _, gate := range c.Gates {
		if gate.Action == "NETWORK" && gate.State == "REVOKED" {
			wantGate = true
		}
	}
	if !wantGate {
		t.Fatal("network acquisition gate was not revoked")
	}
	fields := c.Extensions["io.github.itxcrusher.git-casebook"].(map[string]any)
	count, ok := fields["analysis_network_capable_command_count"].(float64)
	if !ok || count != 0 {
		t.Fatalf("analysis command audit is not zero: %#v", fields)
	}
}

func TestCaseSchemaAndJSONLValidate(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	repo := fixtures.New(t, root, "schema")
	repo.Commit("base.txt", "base\n", "base")
	_, _, caseRoot := investigate(t, root, repo.Path)
	raw, err := os.ReadFile(filepath.Join(caseRoot, casefile.CaseFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := casefile.ValidateSchema(raw); err != nil {
		t.Fatal(err)
	}
	store, _ := casefile.Open(caseRoot)
	for _, name := range []string{casefile.EventsFile, casefile.FindingsFile} {
		if err := store.ValidateJSONL(name); err != nil {
			t.Fatal(err)
		}
	}
}

func investigate(t *testing.T, root string, sources ...string) (model.Case, app.Verification, string) {
	t.Helper()
	return investigateAt(t, filepath.Join(root, "case"), sources...)
}

func investigateAt(t *testing.T, caseRoot string, sources ...string) (model.Case, app.Verification, string) {
	t.Helper()
	application := app.App{CaseRoot: caseRoot}
	verification, err := application.Investigate(context.Background(), sources, "synthetic-test")
	if err != nil {
		t.Fatal(err)
	}
	store, err := casefile.Open(caseRoot)
	if err != nil {
		t.Fatal(err)
	}
	c, err := store.LoadCase()
	if err != nil {
		t.Fatal(err)
	}
	return c, verification, store.Root
}

func wantRelationship(t *testing.T, c model.Case, a, b, want string) {
	t.Helper()
	for _, relationship := range c.Relationships {
		if relationship.SourceA == a && relationship.SourceB == b {
			if relationship.Classification != want {
				t.Fatalf("%s -> %s got %s, want %s; reasons=%v; source-a=%+v; source-b=%+v", a, b, relationship.Classification, want, relationship.ReasonCodes, sourceByID(c, a), sourceByID(c, b))
			}
			return
		}
	}
	t.Fatalf("relationship %s -> %s not found", a, b)
}

func sourceByID(c model.Case, id string) model.Source {
	for _, source := range c.Sources {
		if source.SourceID == id {
			return source
		}
	}
	return model.Source{}
}

func hasRef(c model.Case, sourceID, name string) bool {
	for _, source := range c.Sources {
		if source.SourceID == sourceID {
			for _, ref := range source.Refs {
				if ref.OriginalName == name {
					return true
				}
			}
		}
	}
	return false
}

func getRef(t *testing.T, c model.Case, sourceID, name string) model.Ref {
	t.Helper()
	for _, source := range c.Sources {
		if source.SourceID == sourceID {
			for _, ref := range source.Refs {
				if ref.OriginalName == name {
					return ref
				}
			}
		}
	}
	t.Fatalf("ref %s not found in %s", name, sourceID)
	return model.Ref{}
}

func extensionString(t *testing.T, source model.Source, key string) string {
	t.Helper()
	fields, ok := source.Extensions["io.github.itxcrusher.git-casebook"].(map[string]any)
	if !ok {
		b, _ := json.Marshal(source.Extensions)
		t.Fatalf("extension namespace unavailable: %s", b)
	}
	value, _ := fields[key].(string)
	return value
}

func TestLinuxSymlinkDoesNotEscapeCase(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("Linux-specific symlink boundary check")
	}
	root := t.TempDir()
	caseRoot := filepath.Join(root, "case")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(caseRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(caseRoot, "artifacts")); err != nil {
		t.Fatal(err)
	}
	store := evidence.Store{CaseRoot: caseRoot}
	if _, _, err := store.PutBytes("txt", []byte("must not escape")); err == nil {
		t.Fatal("artifact store followed an escaping symlink")
	}
}
