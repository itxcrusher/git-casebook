// Package fixtures generates clean-room Git histories for integration tests.
package fixtures

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type Repo struct {
	T    *testing.T
	Path string
	home string
}

func New(t *testing.T, root, name string) *Repo {
	t.Helper()
	path := filepath.Join(root, name)
	home := filepath.Join(root, "trusted-git-home")
	mustMkdir(t, home)
	mustWrite(t, filepath.Join(home, ".gitconfig"), "")
	repo := &Repo{T: t, Path: path, home: home}
	repo.run("init", "-b", "main", path)
	repo.run("-C", path, "config", "core.autocrlf", "false")
	repo.run("-C", path, "config", "user.name", "Synthetic Fixture")
	repo.run("-C", path, "config", "user.email", "fixture@example.invalid")
	return repo
}

func Clone(t *testing.T, root, name string, source *Repo) *Repo {
	t.Helper()
	path := filepath.Join(root, name)
	repo := &Repo{T: t, Path: path, home: source.home}
	repo.run("clone", "--no-local", "--", source.Path, path)
	repo.run("-C", path, "config", "core.autocrlf", "false")
	repo.run("-C", path, "config", "user.name", "Synthetic Fixture")
	repo.run("-C", path, "config", "user.email", "fixture@example.invalid")
	return repo
}

func BareClone(t *testing.T, root, name string, source *Repo) *Repo {
	t.Helper()
	path := filepath.Join(root, name)
	repo := &Repo{T: t, Path: path, home: source.home}
	repo.run("clone", "--bare", "--no-local", "--", source.Path, path)
	return repo
}

func ShallowBareClone(t *testing.T, root, name string, source *Repo) *Repo {
	t.Helper()
	path := filepath.Join(root, name)
	repo := &Repo{T: t, Path: path, home: source.home}
	locator := "file://" + filepath.ToSlash(source.Path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(locator, "file:///") {
		locator = "file:///" + filepath.ToSlash(source.Path)
	}
	repo.run("clone", "--bare", "--depth=1", "--", locator, path)
	return repo
}

func (r *Repo) Commit(name, content, message string) string {
	r.T.Helper()
	path := filepath.Join(r.Path, filepath.FromSlash(name))
	mustMkdir(r.T, filepath.Dir(path))
	mustWrite(r.T, path, content)
	r.run("-C", r.Path, "add", "--", name)
	r.run("-C", r.Path, "commit", "-m", message)
	return r.Output("rev-parse", "HEAD")
}

func (r *Repo) CommitAll(message string) string {
	r.T.Helper()
	r.run("-C", r.Path, "add", "-A")
	r.run("-C", r.Path, "commit", "-m", message)
	return r.Output("rev-parse", "HEAD")
}

func (r *Repo) Checkout(name string, create bool) {
	r.T.Helper()
	args := []string{"-C", r.Path, "checkout"}
	if create {
		args = append(args, "-b")
	}
	args = append(args, name)
	r.run(args...)
}

func (r *Repo) Branch(name, oid string) {
	r.T.Helper()
	r.run("-C", r.Path, "branch", name, oid)
}

func (r *Repo) UpdateRef(name, oid string) {
	r.T.Helper()
	r.run("-C", r.Path, "update-ref", name, oid)
}

func (r *Repo) TagLightweight(name, oid string) {
	r.T.Helper()
	r.run("-C", r.Path, "tag", name, oid)
}

func (r *Repo) TagAnnotated(name, oid string) {
	r.T.Helper()
	r.run("-C", r.Path, "tag", "-a", name, "-m", "synthetic annotated tag", oid)
}

func (r *Repo) MergeNoFF(branch, message string) string {
	r.T.Helper()
	r.run("-C", r.Path, "merge", "--no-ff", branch, "-m", message)
	return r.Output("rev-parse", "HEAD")
}

func (r *Repo) ResetHard(oid string) {
	r.T.Helper()
	r.run("-C", r.Path, "reset", "--hard", oid)
}

func (r *Repo) Output(args ...string) string {
	r.T.Helper()
	all := append([]string{"-C", r.Path}, args...)
	output, err := r.command(all...).Output()
	if err != nil {
		r.T.Fatalf("trusted fixture git %v failed: %v", all, err)
	}
	return strings.TrimSpace(string(output))
}

func (r *Repo) Run(args ...string) { r.run(args...) }

func (r *Repo) run(args ...string) {
	r.T.Helper()
	cmd := r.command(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.T.Fatalf("trusted fixture git %v failed: %v\n%s", args, err, output)
	}
}

func (r *Repo) command(args ...string) *exec.Cmd {
	r.T.Helper()
	cmd := exec.Command("git", args...)
	cmd.Env = fixtureEnv(r.home)
	cmd.Stdin = strings.NewReader("")
	return cmd
}

func fixtureEnv(home string) []string {
	allowed := map[string]bool{"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true, "TEMP": true, "TMP": true, "TMPDIR": true}
	var env []string
	for _, value := range os.Environ() {
		key, _, ok := strings.Cut(value, "=")
		if ok && allowed[strings.ToUpper(key)] {
			env = append(env, value)
		}
	}
	return append(env,
		"HOME="+home,
		"USERPROFILE="+home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(home, ".gitconfig"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=Synthetic Fixture",
		"GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=Synthetic Fixture",
		"GIT_COMMITTER_EMAIL=fixture@example.invalid",
		"GIT_AUTHOR_DATE=2020-01-02T03:04:05Z",
		"GIT_COMMITTER_DATE=2020-01-02T03:04:05Z",
	)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func FileURL(path string) string {
	value := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		return "file:///" + value
	}
	return "file://" + value
}

func CorruptLooseObject(t *testing.T, repo *Repo, oid string) {
	t.Helper()
	gitDir := repo.Output("rev-parse", "--git-dir")
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repo.Path, gitDir)
	}
	path := filepath.Join(gitDir, "objects", oid[:2], oid[2:])
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("make synthetic object writable: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove designated synthetic object: %v", err)
	}
}

func WriteFile(t *testing.T, repo *Repo, name, content string) {
	t.Helper()
	path := filepath.Join(repo.Path, filepath.FromSlash(name))
	mustMkdir(t, filepath.Dir(path))
	mustWrite(t, path, content)
}

func Gitlink(t *testing.T, repo *Repo, path, oid string) {
	t.Helper()
	repo.run("-C", repo.Path, "update-index", "--add", "--cacheinfo", fmt.Sprintf("160000,%s,%s", oid, path))
}
