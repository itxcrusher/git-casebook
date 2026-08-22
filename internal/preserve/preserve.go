package preserve

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/itxcrusher/repo-rehab/internal/evidence"
	"github.com/itxcrusher/repo-rehab/internal/gitexec"
	"github.com/itxcrusher/repo-rehab/internal/model"
)

var scpLocator = regexp.MustCompile(`^[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:[^\s]+$`)

type Result struct {
	MirrorPath          string
	MirrorRelativePath  string
	StoredLocator       string
	Method              string
	RetrievedAt         string
	NetworkUsed         bool
	OriginalFingerprint string
	MirrorFingerprint   string
}

func Acquire(ctx context.Context, runner *gitexec.Runner, caseRoot string, source model.PolicySource) (Result, error) {
	kind, stored, err := ClassifyLocator(source.Locator, source.Kind)
	if err != nil {
		return Result{}, err
	}
	mirrorRel := filepath.ToSlash(filepath.Join("sources", source.SourceID+".git"))
	mirrorPath := filepath.Join(caseRoot, filepath.FromSlash(mirrorRel))
	if _, err := os.Stat(mirrorPath); err == nil {
		if _, probeErr := runner.Run(ctx, gitexec.ClassAnalysis, mirrorPath, "rev-parse", "--is-bare-repository"); probeErr != nil {
			return Result{}, fmt.Errorf("existing preserved source %s is not a valid bare repository: %w", source.SourceID, probeErr)
		}
		digest, err := evidence.TreeFingerprint(mirrorPath)
		if err != nil {
			return Result{}, err
		}
		return Result{MirrorPath: mirrorPath, MirrorRelativePath: mirrorRel, StoredLocator: stored, Method: "REUSED_PRESERVED_MIRROR", RetrievedAt: time.Now().UTC().Format(time.RFC3339Nano), NetworkUsed: false, MirrorFingerprint: digest}, nil
	} else if !os.IsNotExist(err) {
		return Result{}, err
	}

	result := Result{
		MirrorPath: mirrorPath, MirrorRelativePath: mirrorRel, StoredLocator: stored,
		Method: "MIRROR_CLONE", RetrievedAt: time.Now().UTC().Format(time.RFC3339Nano),
		NetworkUsed: kind == "remote",
	}
	if kind == "local" {
		result.OriginalFingerprint, err = evidence.TreeFingerprint(stored)
		if err != nil {
			return Result{}, fmt.Errorf("fingerprint local source before acquisition: %w", err)
		}
	}
	cloneLocator := source.Locator
	if kind == "local" {
		cloneLocator = stored
	}
	if _, err := runner.Run(ctx, gitexec.ClassAcquisition, "", "clone", "--mirror", "--no-hardlinks", "--no-recurse-submodules", "--", cloneLocator, mirrorPath); err != nil {
		return result, fmt.Errorf("preserve source %s: %w", source.SourceID, err)
	}
	if _, err := runner.Run(ctx, gitexec.ClassAcquisition, mirrorPath, "config", "--local", "--replace-all", "remote.origin.pushurl", "disabled://no-push"); err != nil {
		return result, fmt.Errorf("disable push destination for source %s: %w", source.SourceID, err)
	}
	if kind == "local" {
		after, err := evidence.TreeFingerprint(stored)
		if err != nil {
			return result, fmt.Errorf("fingerprint local source after acquisition: %w", err)
		}
		if after != result.OriginalFingerprint {
			return result, fmt.Errorf("local source %s changed during acquisition", source.SourceID)
		}
	}
	result.MirrorFingerprint, err = evidence.TreeFingerprint(mirrorPath)
	if err != nil {
		return result, fmt.Errorf("fingerprint preserved mirror: %w", err)
	}
	return result, nil
}

func ClassifyLocator(locator, requestedKind string) (string, string, error) {
	if strings.TrimSpace(locator) == "" || strings.HasPrefix(locator, "-") {
		return "", "", fmt.Errorf("invalid source locator")
	}
	if info, err := os.Stat(locator); err == nil {
		if !info.IsDir() {
			return "", "", fmt.Errorf("local source must be a Git directory")
		}
		if requestedKind == "remote" {
			return "", "", fmt.Errorf("source is local but policy requires remote")
		}
		abs, err := filepath.Abs(locator)
		if err != nil {
			return "", "", err
		}
		return "local", filepath.ToSlash(abs), nil
	}
	if requestedKind == "local" {
		return "", "", fmt.Errorf("declared local source does not exist")
	}
	if scpLocator.MatchString(locator) {
		return "remote", locator, nil
	}
	parsed, err := url.Parse(locator)
	if err != nil {
		return "", "", fmt.Errorf("parse remote locator: %w", err)
	}
	switch parsed.Scheme {
	case "https", "ssh", "file":
	default:
		return "", "", fmt.Errorf("unsupported source protocol %q", parsed.Scheme)
	}
	if parsed.Scheme != "file" && parsed.Host == "" {
		return "", "", fmt.Errorf("remote locator has no host")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("remote locators with query strings or fragments are not accepted")
	}
	if parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return "", "", fmt.Errorf("credentials in source locators are not accepted")
		}
	}
	return "remote", parsed.String(), nil
}
