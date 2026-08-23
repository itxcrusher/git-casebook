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

	"github.com/itxcrusher/git-casebook/internal/version"
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

func TestReleaseNoteLinksAreSafeInGitHubReleaseBodies(t *testing.T) {
	root := repositoryRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, "docs", "release-notes-v*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no versioned release notes found")
	}

	link := regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		inFence := false
		for lineNumber, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}

			for _, match := range link.FindAllStringSubmatch(line, -1) {
				target := strings.TrimSpace(match[1])
				lower := strings.ToLower(target)
				if strings.HasPrefix(lower, "https://") ||
					strings.HasPrefix(lower, "http://") ||
					strings.HasPrefix(lower, "mailto:") ||
					strings.HasPrefix(target, "#") {
					continue
				}
				t.Errorf("%s:%d uses release-body-unsafe relative link %q", filepath.ToSlash(path), lineNumber+1, target)
			}
		}
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

// The release workflow resolves its notes as docs/release-notes-<tag>.md only
// after the immutable tag has been pushed. A mismatch there already cost one
// v0.1.0 recovery cycle, so couple the filename to the version constant and
// fail on dev instead of after tagging.
func TestReleaseNotesExistForTheCurrentVersionLine(t *testing.T) {
	root := repositoryRoot(t)
	current := strings.TrimSuffix(version.Development, "-dev")
	if current == version.Development {
		t.Fatalf("development version %q does not carry the expected -dev suffix", version.Development)
	}

	relative := filepath.Join("docs", "release-notes-v"+current+".md")
	if _, err := os.Stat(filepath.Join(root, relative)); err != nil {
		t.Fatalf("release workflow will look for docs/release-notes-v%s.md and it is missing: %v", current, err)
	}

	if !bytes.Contains(readRepositoryFile(t, root, relative), []byte("@v"+current)) {
		t.Errorf("docs/release-notes-v%s.md does not document the @v%s install command", current, current)
	}

	if !bytes.Contains(readRepositoryFile(t, root, "README.md"), []byte("@v"+current)) {
		t.Errorf("README.md does not advertise @v%s; it ships inside every release archive", current)
	}
}
