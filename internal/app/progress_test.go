package app

import (
	"strings"
	"testing"
)

// A silent acquisition is indistinguishable from a hang. Acquiring a repository
// inside a large GitHub fork network can transfer hundreds of megabytes, and
// with no output the operator has no signal that anything is happening.
func TestAcquisitionProgressNamesTheSourceAndLocator(t *testing.T) {
	line := acquisitionStartLine("source-01", "https://github.com/octocat/Spoon-Knife.git")

	if !strings.Contains(line, "source-01") {
		t.Fatalf("progress line omits the source id: %q", line)
	}
	if !strings.Contains(line, "Spoon-Knife") {
		t.Fatalf("progress line omits the locator: %q", line)
	}
}

func TestAcquisitionCompletionReportsSize(t *testing.T) {
	line := acquisitionDoneLine("source-01", 473000000)

	if !strings.Contains(line, "source-01") {
		t.Fatalf("completion line omits the source id: %q", line)
	}
	if !strings.Contains(line, "MiB") {
		t.Fatalf("completion line omits a human-readable size: %q", line)
	}
}

// The size alone does not tell an operator why a small project produced a huge
// mirror. Naming the shared-object-storage cause is the actionable part.
func TestUnusuallyLargeMirrorIsExplained(t *testing.T) {
	line := acquisitionDoneLine("source-01", 600*1024*1024)

	if !strings.Contains(line, "fork network") {
		t.Fatalf("large mirror gives no explanation: %q", line)
	}
}

func TestOrdinaryMirrorIsNotFlagged(t *testing.T) {
	line := acquisitionDoneLine("source-01", 2*1024*1024)

	if strings.Contains(line, "fork network") {
		t.Fatalf("ordinary mirror was flagged as unusual: %q", line)
	}
}

func TestHumanBytesUsesBinaryUnits(t *testing.T) {
	cases := map[int64]string{
		512:                    "512 B",
		2 * 1024:               "2.0 KiB",
		3 * 1024 * 1024:        "3.0 MiB",
		5 * 1024 * 1024 * 1024: "5.0 GiB",
	}
	for size, want := range cases {
		if got := humanBytes(size); got != want {
			t.Fatalf("humanBytes(%d) = %q, want %q", size, got, want)
		}
	}
}
