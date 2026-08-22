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

func TestMarkdownRelativeLinksResolve(t *testing.T) {
	root := repositoryRoot(t)
	link := regexp.MustCompile(`\[[^\]]+\]\(([^)]+)\)`)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == "bin" {
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
