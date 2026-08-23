package test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestRepositoryYAMLIsSyntacticallyValid(t *testing.T) {
	root := repositoryRoot(t)
	paths := []string{
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
		".github/dependabot.yml",
		".github/ISSUE_TEMPLATE/config.yml",
		".github/ISSUE_TEMPLATE/bug.yml",
	}
	for _, rel := range paths {
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			decoder := yaml.NewDecoder(bytes.NewReader(content))
			var value any
			if err := decoder.Decode(&value); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCanonicalProductIdentity(t *testing.T) {
	root := repositoryRoot(t)
	checks := []struct {
		path     string
		expected string
	}{
		{path: "go.mod", expected: "module github.com/itxcrusher/git-casebook"},
		{path: "README.md", expected: "# GitCasebook"},
		{path: "SECURITY.md", expected: "github.com/itxcrusher/git-casebook/security/advisories/new"},
		{path: "schema/case-v1.schema.json", expected: "urn:git-casebook:case:1.0.0"},
		{path: "cmd/git-casebook/main.go", expected: "git-casebook investigate"},
		{path: "internal/version/version.go", expected: "func Current() string"},
		{path: "cmd/git-casebook/main.go", expected: "version.Current()"},
		{path: "internal/app/app.go", expected: "Version: version.Current()"},
		{path: "scripts/build-release.ps1", expected: "internal/version.Override"},
		{path: ".github/workflows/release.yml", expected: `--notes-file "$notes"`},
	}
	for _, check := range checks {
		rel, expected := check.path, check.expected
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		if !bytes.Contains(content, []byte(expected)) {
			t.Errorf("%s does not contain canonical identity %q", rel, expected)
		}
	}
	commandDirectories, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	if len(commandDirectories) != 1 || commandDirectories[0].Name() != "git-casebook" || !commandDirectories[0].IsDir() {
		t.Errorf("cmd must contain only the canonical git-casebook command directory")
	}
}

func TestReleaseWorkflowsKeepSmokeOutsideArtifactsAndPublishExactAssets(t *testing.T) {
	root := repositoryRoot(t)
	release := readRepositoryFile(t, root, ".github/workflows/release.yml")
	ci := readRepositoryFile(t, root, ".github/workflows/ci.yml")

	for _, forbidden := range []string{`./dist/*`, `mkdir ./dist/smoke`} {
		if bytes.Contains(release, []byte(forbidden)) {
			t.Errorf("release workflow contains unsafe artifact handling %q", forbidden)
		}
	}
	for _, required := range []string{
		`smoke_dir="$(mktemp -d)"`,
		`-ListPublishAssets`,
		`test "${#assets[@]}" -eq 6`,
		`"${assets[@]}"`,
	} {
		if !bytes.Contains(release, []byte(required)) {
			t.Errorf("release workflow is missing boundary check %q", required)
		}
	}
	for _, required := range []string{
		`smoke_dir="$(mktemp -d)"`,
		`Verify publishable asset boundary`,
		`-ListPublishAssets`,
		`test "${#assets[@]}" -eq 6`,
	} {
		if !bytes.Contains(ci, []byte(required)) {
			t.Errorf("CI release dry run is missing boundary check %q", required)
		}
	}
}

func TestMarkdownRelativeLinksResolve(t *testing.T) {
	root := repositoryRoot(t)
	link := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "bin" || entry.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range link.FindAllSubmatch(content, -1) {
			target := string(match[1])
			if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
				continue
			}
			target, _, _ = strings.Cut(target, "#")
			if target == "" {
				continue
			}
			resolved := filepath.Join(filepath.Dir(path), filepath.FromSlash(target))
			if _, err := os.Stat(resolved); err != nil {
				t.Errorf("%s links to missing %s", filepath.ToSlash(path), target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate repository root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), ".."))
}

func readRepositoryFile(t *testing.T, root, relative string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return content
}
