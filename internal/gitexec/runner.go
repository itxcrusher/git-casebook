package gitexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/itxcrusher/repo-rehab/internal/model"
)

type Class string

const (
	ClassProbe       Class = "PROBE"
	ClassAcquisition Class = "ACQUISITION"
	ClassAnalysis    Class = "OFFLINE_ANALYSIS"
)

type Limits struct {
	CommandTimeout     time.Duration
	AcquisitionTimeout time.Duration
	MaxOutputBytes     int64
}

type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Duration time.Duration
}

type Record struct {
	Class          Class
	Command        string
	NetworkCapable bool
	Succeeded      bool
}

type Runner struct {
	gitPath     string
	controlRoot string
	limits      Limits
	mu          sync.Mutex
	records     []Record
}

type CommandError struct {
	Class    Class
	Command  string
	ExitCode int
	Stderr   string
	Cause    error
}

func (e *CommandError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git %s command failed (exit %d): %s", e.Class, e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("git %s command failed (exit %d): %v", e.Class, e.ExitCode, e.Cause)
}

func (e *CommandError) Unwrap() error { return e.Cause }

func New(controlRoot string, limits Limits) (*Runner, error) {
	path, err := exec.LookPath("git")
	if err != nil {
		return nil, fmt.Errorf("native git executable is required: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve git executable: %w", err)
	}
	controlRoot, err = filepath.Abs(controlRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve control directory: %w", err)
	}
	for _, dir := range []string{controlRoot, filepath.Join(controlRoot, "home"), filepath.Join(controlRoot, "hooks"), filepath.Join(controlRoot, "template"), filepath.Join(controlRoot, "xdg")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	config := filepath.Join(controlRoot, "gitconfig")
	if err := os.WriteFile(config, []byte{}, 0o600); err != nil {
		return nil, fmt.Errorf("create isolated git config: %w", err)
	}
	if limits.CommandTimeout <= 0 {
		limits.CommandTimeout = 2 * time.Minute
	}
	if limits.AcquisitionTimeout <= 0 {
		limits.AcquisitionTimeout = 10 * time.Minute
	}
	if limits.MaxOutputBytes < 1024 {
		limits.MaxOutputBytes = 16 << 20
	}
	return &Runner{gitPath: abs, controlRoot: controlRoot, limits: limits}, nil
}

func (r *Runner) GitPath() string { return r.gitPath }

func (r *Runner) Version(ctx context.Context) (string, error) {
	result, err := r.Run(ctx, ClassProbe, "", "version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(result.Stdout)), nil
}

func (r *Runner) Records() []Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Record(nil), r.records...)
}

func (r *Runner) Run(ctx context.Context, class Class, repo string, args ...string) (Result, error) {
	return r.RunInput(ctx, class, repo, nil, args...)
}

func (r *Runner) RunInput(ctx context.Context, class Class, repo string, input []byte, args ...string) (Result, error) {
	if len(args) == 0 {
		return Result{}, fmt.Errorf("git command is required")
	}
	if err := validateCommand(class, args); err != nil {
		return Result{}, err
	}
	timeout := r.limits.CommandTimeout
	if class == ClassAcquisition {
		timeout = r.limits.AcquisitionTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	fullArgs := r.controlArgs()
	if repo != "" {
		absRepo, err := filepath.Abs(repo)
		if err != nil {
			return Result{}, fmt.Errorf("resolve repository path: %w", err)
		}
		fullArgs = append(fullArgs, "-C", absRepo)
	}
	fullArgs = append(fullArgs, args...)
	cmd := exec.CommandContext(ctx, r.gitPath, fullArgs...)
	cmd.Dir = r.controlRoot
	cmd.Env = r.environment()
	cmd.Stdin = bytes.NewReader(input)
	stdout := &limitedBuffer{remaining: r.limits.MaxOutputBytes}
	stderr := &limitedBuffer{remaining: r.limits.MaxOutputBytes}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	started := time.Now()
	err := cmd.Run()
	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: 0, Duration: time.Since(started)}
	if err != nil {
		result.ExitCode = exitCode(err)
	}
	network := networkCapable(args[0])
	r.mu.Lock()
	r.records = append(r.records, Record{Class: class, Command: args[0], NetworkCapable: network, Succeeded: err == nil})
	r.mu.Unlock()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, &CommandError{Class: class, Command: args[0], ExitCode: result.ExitCode, Cause: context.DeadlineExceeded}
	}
	if stdout.exceeded || stderr.exceeded {
		return result, &CommandError{Class: class, Command: args[0], ExitCode: result.ExitCode, Cause: errors.New("bounded command output limit exceeded")}
	}
	if err != nil {
		message := sanitizeText(strings.TrimSpace(string(result.Stderr)))
		return result, &CommandError{Class: class, Command: args[0], ExitCode: result.ExitCode, Stderr: message, Cause: err}
	}
	return result, nil
}

func (r *Runner) controlArgs() []string {
	emptyHooks := filepath.Join(r.controlRoot, "hooks")
	return []string{
		"-c", "core.hooksPath=" + filepath.ToSlash(emptyHooks),
		"-c", "credential.helper=",
		"-c", "credential.interactive=never",
		"-c", "core.askPass=",
		"-c", "protocol.allow=never",
		"-c", "protocol.file.allow=always",
		"-c", "protocol.https.allow=always",
		"-c", "protocol.ssh.allow=always",
		"-c", "protocol.ext.allow=never",
		"-c", "filter.lfs.smudge=",
		"-c", "filter.lfs.required=false",
		"-c", "core.fsmonitor=false",
		"-c", "advice.detachedHead=false",
	}
}

func (r *Runner) environment() []string {
	allowed := map[string]bool{
		"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "WINDIR": true,
		"COMSPEC": true, "TEMP": true, "TMP": true, "TMPDIR": true,
		"LANG": true, "LC_ALL": true, "SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	}
	env := make([]string, 0, len(allowed)+10)
	for _, value := range os.Environ() {
		key, _, ok := strings.Cut(value, "=")
		if ok && allowed[strings.ToUpper(key)] {
			env = append(env, value)
		}
	}
	home := filepath.Join(r.controlRoot, "home")
	env = append(env,
		"HOME="+home,
		"USERPROFILE="+home,
		"XDG_CONFIG_HOME="+filepath.Join(r.controlRoot, "xdg"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+filepath.Join(r.controlRoot, "gitconfig"),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_NO_REPLACE_OBJECTS=1",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_LFS_SKIP_SMUDGE=1",
		"GCM_INTERACTIVE=Never",
	)
	return env
}

func validateCommand(class Class, args []string) error {
	command := args[0]
	switch class {
	case ClassProbe:
		if command != "version" {
			return fmt.Errorf("probe command %q is not allowed", command)
		}
	case ClassAcquisition:
		if command == "config" && len(args) == 5 && args[1] == "--local" && args[2] == "--replace-all" && args[3] == "remote.origin.pushurl" && args[4] == "disabled://no-push" {
			return nil
		}
		if command != "clone" {
			return fmt.Errorf("acquisition command %q is not allowed", command)
		}
	case ClassAnalysis:
		allowed := map[string]bool{
			"version": true, "rev-parse": true, "symbolic-ref": true,
			"for-each-ref": true, "fsck": true, "rev-list": true,
			"cat-file": true, "show": true, "config": true, "ls-tree": true,
			"check-ref-format": true,
		}
		if !allowed[command] {
			return fmt.Errorf("offline analysis command %q is not allowed", command)
		}
		if command == "config" && !configReadOnly(args[1:]) {
			return fmt.Errorf("offline git config invocation is not read-only")
		}
	default:
		return fmt.Errorf("unknown command class %q", class)
	}
	return nil
}

func configReadOnly(args []string) bool {
	for _, arg := range args {
		if arg == "--get" || arg == "--get-all" || arg == "--get-regexp" || arg == "--list" || arg == "--show-origin" || arg == "--show-scope" || arg == "--null" || arg == "-z" || arg == "--local" {
			continue
		}
		if strings.HasPrefix(arg, "--get-") {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return false
		}
	}
	return true
}

func networkCapable(command string) bool {
	switch command {
	case "clone", "fetch", "pull", "push", "ls-remote", "submodule":
		return true
	default:
		return false
	}
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func sanitizeText(value string) string {
	fields := strings.Fields(value)
	for i, field := range fields {
		trimmed := strings.Trim(field, "'\"()[]{}<>,")
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		if parsed.User != nil {
			parsed.User = url.User("redacted")
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		fields[i] = strings.Replace(field, trimmed, parsed.String(), 1)
	}
	return strings.Join(fields, " ")
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int64
	exceeded  bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.remaining <= 0 {
		b.exceeded = true
		return len(p), nil
	}
	writable := int64(len(p))
	if writable > b.remaining {
		writable = b.remaining
		b.exceeded = true
	}
	_, _ = b.buffer.Write(p[:writable])
	b.remaining -= writable
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func ParseLimits(l model.Limits) Limits {
	return Limits{
		CommandTimeout:     time.Duration(l.CommandTimeoutSeconds) * time.Second,
		AcquisitionTimeout: time.Duration(l.AcquisitionTimeoutSeconds) * time.Second,
		MaxOutputBytes:     l.MaxCommandOutputBytes,
	}
}

func NumericVersion(version string) string {
	for _, field := range strings.Fields(version) {
		if _, err := strconv.ParseFloat(strings.TrimSuffix(field, ".windows.1"), 64); err == nil {
			return field
		}
	}
	return version
}

var _ io.Writer = (*limitedBuffer)(nil)
