package app

import (
	"strings"
	"testing"

	"github.com/itxcrusher/git-casebook/internal/model"
)

func twoSourceCase(rel model.Relationship, inverse model.Relationship) model.Case {
	return model.Case{
		CaseID: "case-test",
		Status: "READY_FOR_REVIEW",
		Sources: []model.Source{
			{SourceID: "source-01", OriginalLocator: "https://example.invalid/a.git", Integrity: "VERIFIED", Completeness: "COMPLETE"},
			{SourceID: "source-02", OriginalLocator: "https://example.invalid/b.git", Integrity: "VERIFIED", Completeness: "COMPLETE"},
		},
		Relationships: []model.Relationship{rel, inverse},
	}
}

func joined(c model.Case) string { return strings.Join(SummaryLines(c), "\n") }

// The headline result must reach stdout. Leaving it only in report.md means the
// operator sees no answer at all after a completed investigation.
func TestSummaryStatesTheClassificationAndUniqueCommits(t *testing.T) {
	out := joined(twoSourceCase(
		model.Relationship{SourceA: "source-01", SourceB: "source-02", Classification: "SUPERSET",
			SharedCommitCount: 36, SourceAOnlyCommitCount: 79},
		model.Relationship{SourceA: "source-02", SourceB: "source-01", Classification: "SUBSET",
			SharedCommitCount: 36, SourceBOnlyCommitCount: 79},
	))

	if !strings.Contains(out, "SUPERSET") {
		t.Fatalf("summary omits the classification:\n%s", out)
	}
	if !strings.Contains(out, "79") {
		t.Fatalf("summary omits the unique commit count:\n%s", out)
	}
}

// Each unordered pair is reported once. Printing both directions doubles the
// output and buries the answer for a multi-source case.
func TestSummaryReportsEachPairOnce(t *testing.T) {
	out := joined(twoSourceCase(
		model.Relationship{SourceA: "source-01", SourceB: "source-02", Classification: "SUPERSET",
			SharedCommitCount: 36, SourceAOnlyCommitCount: 79},
		model.Relationship{SourceA: "source-02", SourceB: "source-01", Classification: "SUBSET",
			SharedCommitCount: 36, SourceBOnlyCommitCount: 79},
	))

	if strings.Count(out, "SUPERSET")+strings.Count(out, "SUBSET") != 1 {
		t.Fatalf("expected one line per unordered pair:\n%s", out)
	}
}

// Object-only containment must not print as a bare classification with no
// numbers, which is what makes the written report look self-contradictory.
func TestSummaryShowsObjectEvidenceWhenCommitsAreEqual(t *testing.T) {
	out := joined(twoSourceCase(
		model.Relationship{SourceA: "source-01", SourceB: "source-02", Classification: "SUBSET",
			SharedCommitCount: 36, SourceBOnlyObjectCount: 2},
		model.Relationship{SourceA: "source-02", SourceB: "source-01", Classification: "SUPERSET",
			SharedCommitCount: 36, SourceAOnlyObjectCount: 2},
	))

	if !strings.Contains(out, "object") {
		t.Fatalf("summary hides object-only containment evidence:\n%s", out)
	}
}

// A fail-closed result is the one an operator must not miss.
func TestSummarySurfacesUnknown(t *testing.T) {
	out := joined(twoSourceCase(
		model.Relationship{SourceA: "source-01", SourceB: "source-02", Classification: "UNKNOWN"},
		model.Relationship{SourceA: "source-02", SourceB: "source-01", Classification: "UNKNOWN"},
	))

	if !strings.Contains(out, "UNKNOWN") {
		t.Fatalf("summary omits the UNKNOWN classification:\n%s", out)
	}
}

func TestSummaryHandlesSingleSourceCase(t *testing.T) {
	c := model.Case{
		CaseID:  "case-test",
		Status:  "READY_FOR_REVIEW",
		Sources: []model.Source{{SourceID: "source-01", Integrity: "VERIFIED", Completeness: "COMPLETE"}},
	}

	out := joined(c)

	if out == "" {
		t.Fatal("single-source case produced no summary at all")
	}
	if strings.Contains(out, "->") {
		t.Fatalf("single-source case invented a relationship:\n%s", out)
	}
}
