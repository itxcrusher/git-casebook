package version

import (
	"regexp"
	"runtime/debug"
	"strings"
)

const Development = "0.1.1-dev"

// Override is set by the release build. Ordinary versioned module installs use
// the main-module version embedded by the Go toolchain instead.
var Override string

var semanticVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
var pseudoVersion = regexp.MustCompile(`(?:^|[.-])[0-9]{14}-[0-9a-fA-F]{7,}(?:\+dirty)?$`)

// Current returns the canonical version recorded by both the CLI and case
// evidence. Release overrides take precedence, followed by the module version
// embedded by Go. Local source builds fall back to Development.
func Current() string {
	moduleVersion := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		moduleVersion = info.Main.Version
	}
	return Resolve(Override, moduleVersion)
}

// Resolve is the deterministic version-resolution seam used by tests.
func Resolve(override, moduleVersion string) string {
	if value, ok := normalize(override); ok {
		return value
	}
	if value, ok := normalize(moduleVersion); ok {
		return value
	}
	return Development
}

func normalize(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || value == "(devel)" {
		return "", false
	}
	if strings.HasSuffix(value, "+dirty") || pseudoVersion.MatchString(value) {
		return "", false
	}
	value = strings.TrimPrefix(value, "v")
	if !semanticVersion.MatchString(value) {
		return "", false
	}
	return value, true
}
