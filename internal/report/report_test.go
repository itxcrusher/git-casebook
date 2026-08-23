package report

import (
	"strings"
	"testing"

	"github.com/itxcrusher/git-casebook/internal/model"
)

func objectContainedCase() model.Case {
	return model.Case{
		CaseID:          "case-test",
		Status:          "READY_FOR_REVIEW",
		CaseFingerprint: "fingerprint",
		Tool:            model.Tool{Version: "0.0.0-test", GitVersion: "git version 2.45.2"},
		Sources: []model.Source{
			{SourceID: "source-01", OriginalLocator: "https://example.invalid/a.git", Role: "UNKNOWN", Integrity: "VERIFIED", Completeness: "COMPLETE"},
			{SourceID: "source-02", OriginalLocator: "https://example.invalid/b.git", Role: "UNKNOWN", Integrity: "VERIFIED", Completeness: "COMPLETE"},
		},
		Relationships: []model.Relationship{{
			RelationshipID:         "relationship-source-01-to-source-02",
			SourceA:                "source-01",
			SourceB:                "source-02",
			Classification:         "SUBSET",
			SharedCommitCount:      36,
			SourceAOnlyCommitCount: 0,
			SourceBOnlyCommitCount: 0,
			SourceAOnlyObjectCount: 0,
			SourceBOnlyObjectCount: 2,
			ReasonCodes:            []string{"SOURCE_A_COMMIT_AND_OBJECT_SETS_PROPERLY_CONTAINED"},
			EvidenceIDs:            []string{"evidence-relationship-source-01-to-source-02"},
		}},
	}
}

// A containment proven only by objects must not render as an unexplained
// classification. Without object columns the row reads "SUBSET, 0 A-only,
// 0 B-only", which a human reasonably reads as a contradiction.
func TestRelationshipTableShowsObjectEvidenceBehindClassification(t *testing.T) {
	rendered := string(Render(objectContainedCase()))

	if !strings.Contains(rendered, "A-only objects") || !strings.Contains(rendered, "B-only objects") {
		t.Fatalf("relationship table omits the object columns the classification used:\n%s", rendered)
	}
	if !strings.Contains(rendered, "SOURCE_A_COMMIT_AND_OBJECT_SETS_PROPERLY_CONTAINED") {
		t.Fatalf("relationship table omits the reason code explaining the classification:\n%s", rendered)
	}
}

// Object-only containment deserves an explicit note so the reader is never left
// to infer why a zero-unique-commit pair is not EXACT.
func TestObjectOnlyContainmentIsCalledOut(t *testing.T) {
	rendered := string(Render(objectContainedCase()))

	if !strings.Contains(rendered, "no unique commits") {
		t.Fatalf("report does not explain object-only containment:\n%s", rendered)
	}
}

func TestReasonCodesAreEscaped(t *testing.T) {
	c := objectContainedCase()
	c.Relationships[0].ReasonCodes = []string{"BAD|CODE"}

	rendered := string(Render(c))

	if strings.Contains(rendered, "BAD|CODE") {
		t.Fatalf("reason code was not escaped for the Markdown table:\n%s", rendered)
	}
}
