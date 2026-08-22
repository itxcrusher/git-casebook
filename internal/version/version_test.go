package version

import "testing"

func TestResolve(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		override      string
		moduleVersion string
		want          string
	}{
		{name: "explicit release override", override: "0.1.0", moduleVersion: "(devel)", want: "0.1.0"},
		{name: "override normalization", override: "v0.1.0", moduleVersion: "v9.9.9", want: "0.1.0"},
		{name: "versioned module build info", moduleVersion: "v0.1.0", want: "0.1.0"},
		{name: "versioned module prerelease", moduleVersion: "v0.2.0-rc.1", want: "0.2.0-rc.1"},
		{name: "development checkout", moduleVersion: "(devel)", want: Development},
		{name: "clean untagged checkout pseudo-version", moduleVersion: "v0.0.0-20260822104523-0a413c2c5c62", want: Development},
		{name: "dirty untagged checkout pseudo-version", moduleVersion: "v0.0.0-20260822104523-0a413c2c5c62+dirty", want: Development},
		{name: "dirty tagged checkout", moduleVersion: "v0.1.0+dirty", want: Development},
		{name: "missing build info", want: Development},
		{name: "invalid override uses module", override: "not-a-version", moduleVersion: "v0.1.0", want: "0.1.0"},
		{name: "invalid values fail to development", override: "not-a-version", moduleVersion: "also-invalid", want: Development},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Resolve(tt.override, tt.moduleVersion); got != tt.want {
				t.Fatalf("Resolve(%q, %q) = %q, want %q", tt.override, tt.moduleVersion, got, tt.want)
			}
		})
	}
}

func TestCurrentDevelopmentCheckout(t *testing.T) {
	if got := Current(); got != Development {
		t.Fatalf("Current() = %q, want %q for local test build", got, Development)
	}
}
