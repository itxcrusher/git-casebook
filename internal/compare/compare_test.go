package compare

import (
	"testing"

	"github.com/itxcrusher/repo-rehab/internal/inventory"
	"github.com/itxcrusher/repo-rehab/internal/model"
)

func completeSource(id string, commits, objects []string) (model.Source, inventory.Data) {
	source := model.Source{
		SourceID: id, Integrity: "VERIFIED", Completeness: "COMPLETE", ObjectFormat: "sha1",
		Objects: model.ObjectInventory{
			CommitCount: len(commits), ReachableObjectCount: len(objects),
			CommitSetDigest: model.DigestLines(commits), ReachableObjectSetDigest: model.DigestLines(objects),
		},
	}
	return source, inventory.Data{Commits: commits, ReachableObjects: objects}
}

func TestRelationshipSetSemantics(t *testing.T) {
	a, dataA := completeSource("source-a", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	b, dataB := completeSource("source-b", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	c, dataC := completeSource("source-c", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "cccccccccccccccccccccccccccccccccccccccc"}, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "cccccccccccccccccccccccccccccccccccccccc"})
	d, dataD := completeSource("source-d", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "dddddddddddddddddddddddddddddddddddddddd"}, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "dddddddddddddddddddddddddddddddddddddddd"})
	e, dataE := completeSource("source-e", []string{"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}, []string{"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"})

	tests := []struct {
		name string
		sa   model.Source
		da   inventory.Data
		sb   model.Source
		db   inventory.Data
		want string
	}{
		{"exact", a, dataA, b, dataB, "EXACT"},
		{"subset", a, dataA, c, dataC, "SUBSET"},
		{"superset", c, dataC, a, dataA, "SUPERSET"},
		{"diverged", c, dataC, d, dataD, "DIVERGED"},
		{"disjoint", a, dataA, e, dataE, "DISJOINT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := Classify(test.sa, test.da, test.sb, test.db)
			if got.Classification != test.want {
				t.Fatalf("got %s, want %s", got.Classification, test.want)
			}
		})
	}
}

func TestIncompleteFailsClosed(t *testing.T) {
	a, dataA := completeSource("source-a", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	b, dataB := completeSource("source-b", []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, []string{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	b.Completeness = "INCOMPLETE"
	relationship, _ := Classify(a, dataA, b, dataB)
	if relationship.Classification != "UNKNOWN" {
		t.Fatalf("got %s, want UNKNOWN", relationship.Classification)
	}
}
