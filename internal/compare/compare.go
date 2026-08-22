package compare

import (
	"fmt"
	"sort"
	"time"

	"github.com/itxcrusher/git-casebook/internal/evidence"
	"github.com/itxcrusher/git-casebook/internal/inventory"
	"github.com/itxcrusher/git-casebook/internal/model"
)

func All(sources []model.Source, data map[string]inventory.Data, artifacts evidence.Store) ([]model.Relationship, []model.EvidenceItem, error) {
	ordered := append([]model.Source(nil), sources...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].SourceID < ordered[j].SourceID })
	var relationships []model.Relationship
	var evidenceItems []model.EvidenceItem
	for _, sourceA := range ordered {
		for _, sourceB := range ordered {
			if sourceA.SourceID == sourceB.SourceID {
				continue
			}
			relationship, artifact := Classify(sourceA, data[sourceA.SourceID], sourceB, data[sourceB.SourceID])
			path, digest, err := artifacts.PutJSON("relationship.json", artifact)
			if err != nil {
				return nil, nil, err
			}
			evidenceID := "evidence-" + relationship.RelationshipID
			relationship.EvidenceIDs = []string{evidenceID}
			verification := "VERIFIED"
			if relationship.Classification == "UNKNOWN" {
				verification = "INCOMPLETE"
			}
			evidenceItems = append(evidenceItems, model.EvidenceItem{
				EvidenceID: evidenceID, Producer: "git-casebook", Method: "set-containment-over-all-ref-reachable-manifests",
				Inputs:     []string{"source:" + sourceA.SourceID, "source:" + sourceB.SourceID},
				ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), OutputFingerprint: digest,
				RawArtifact: &path, VerificationState: verification,
			})
			relationships = append(relationships, relationship)
		}
	}
	sort.Slice(relationships, func(i, j int) bool { return relationships[i].RelationshipID < relationships[j].RelationshipID })
	sort.Slice(evidenceItems, func(i, j int) bool { return evidenceItems[i].EvidenceID < evidenceItems[j].EvidenceID })
	return relationships, evidenceItems, nil
}

func Classify(sourceA model.Source, dataA inventory.Data, sourceB model.Source, dataB inventory.Data) (model.Relationship, model.RelationshipArtifact) {
	sharedCommits, aOnlyCommits, bOnlyCommits := partition(dataA.Commits, dataB.Commits)
	_, aOnlyObjects, bOnlyObjects := partition(dataA.ReachableObjects, dataB.ReachableObjects)
	relationship := model.Relationship{
		RelationshipID: "relationship-" + sourceA.SourceID + "-to-" + sourceB.SourceID,
		SourceA:        sourceA.SourceID, SourceB: sourceB.SourceID,
		SharedCommitCount:      len(sharedCommits),
		SourceAOnlyCommitCount: len(aOnlyCommits), SourceBOnlyCommitCount: len(bOnlyCommits),
		SourceAOnlyObjectCount: len(aOnlyObjects), SourceBOnlyObjectCount: len(bOnlyObjects),
		EvidenceIDs: []string{},
	}
	reasons := preconditionReasons(sourceA, dataA, sourceB, dataB)
	if len(reasons) > 0 {
		relationship.Classification = "UNKNOWN"
		relationship.ReasonCodes = reasons
	} else {
		commitsAContainsB := len(bOnlyCommits) == 0
		commitsBContainsA := len(aOnlyCommits) == 0
		objectsAContainsB := len(bOnlyObjects) == 0
		objectsBContainsA := len(aOnlyObjects) == 0
		switch {
		case commitsAContainsB && commitsBContainsA && objectsAContainsB && objectsBContainsA:
			relationship.Classification = "EXACT"
			relationship.ReasonCodes = []string{"COMMIT_AND_OBJECT_SETS_EQUAL"}
		case commitsAContainsB && objectsAContainsB:
			relationship.Classification = "SUPERSET"
			relationship.ReasonCodes = []string{"SOURCE_B_COMMIT_AND_OBJECT_SETS_PROPERLY_CONTAINED"}
		case commitsBContainsA && objectsBContainsA:
			relationship.Classification = "SUBSET"
			relationship.ReasonCodes = []string{"SOURCE_A_COMMIT_AND_OBJECT_SETS_PROPERLY_CONTAINED"}
		case len(sharedCommits) > 0:
			relationship.Classification = "DIVERGED"
			relationship.ReasonCodes = []string{"SHARED_COMMITS_WITHOUT_FULL_SET_CONTAINMENT"}
		default:
			relationship.Classification = "DISJOINT"
			relationship.ReasonCodes = []string{"NO_SHARED_REACHABLE_COMMITS"}
		}
	}
	artifact := model.RelationshipArtifact{
		SchemaVersion: model.SchemaVersion, SourceA: sourceA.SourceID, SourceB: sourceB.SourceID,
		SharedCommits: sharedCommits, SourceAOnlyCommits: aOnlyCommits, SourceBOnlyCommits: bOnlyCommits,
		SourceAOnlyObjects: aOnlyObjects, SourceBOnlyObjects: bOnlyObjects,
	}
	return relationship, artifact
}

func preconditionReasons(a model.Source, dataA inventory.Data, b model.Source, dataB inventory.Data) []string {
	var reasons []string
	if a.Integrity != "VERIFIED" {
		reasons = append(reasons, "SOURCE_A_INTEGRITY_UNVERIFIED")
	}
	if b.Integrity != "VERIFIED" {
		reasons = append(reasons, "SOURCE_B_INTEGRITY_UNVERIFIED")
	}
	if a.Completeness != "COMPLETE" {
		reasons = append(reasons, "SOURCE_A_INCOMPLETE")
	}
	if b.Completeness != "COMPLETE" {
		reasons = append(reasons, "SOURCE_B_INCOMPLETE")
	}
	if a.ObjectFormat == "unknown" || b.ObjectFormat == "unknown" || a.ObjectFormat != b.ObjectFormat {
		reasons = append(reasons, "OBJECT_FORMAT_INCOMPATIBLE_OR_UNKNOWN")
	}
	if len(dataA.Commits) == 0 || len(dataB.Commits) == 0 {
		reasons = append(reasons, "EMPTY_COMMIT_SET")
	}
	if len(dataA.Commits) != a.Objects.CommitCount || model.DigestLines(dataA.Commits) != a.Objects.CommitSetDigest || len(dataA.ReachableObjects) != a.Objects.ReachableObjectCount || model.DigestLines(dataA.ReachableObjects) != a.Objects.ReachableObjectSetDigest {
		reasons = append(reasons, "SOURCE_A_MANIFEST_MISMATCH")
	}
	if len(dataB.Commits) != b.Objects.CommitCount || model.DigestLines(dataB.Commits) != b.Objects.CommitSetDigest || len(dataB.ReachableObjects) != b.Objects.ReachableObjectCount || model.DigestLines(dataB.ReachableObjects) != b.Objects.ReachableObjectSetDigest {
		reasons = append(reasons, "SOURCE_B_MANIFEST_MISMATCH")
	}
	sort.Strings(reasons)
	return unique(reasons)
}

func partition(a, b []string) (shared, aOnly, bOnly []string) {
	setA := make(map[string]bool, len(a))
	setB := make(map[string]bool, len(b))
	for _, value := range a {
		setA[value] = true
	}
	for _, value := range b {
		setB[value] = true
	}
	for value := range setA {
		if setB[value] {
			shared = append(shared, value)
		} else {
			aOnly = append(aOnly, value)
		}
	}
	for value := range setB {
		if !setA[value] {
			bOnly = append(bOnly, value)
		}
	}
	sort.Strings(shared)
	sort.Strings(aOnly)
	sort.Strings(bOnly)
	return shared, aOnly, bOnly
}

func unique(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := []string{values[0]}
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func AssertInverse(relationships []model.Relationship) error {
	byPair := map[string]model.Relationship{}
	for _, rel := range relationships {
		byPair[rel.SourceA+"\x00"+rel.SourceB] = rel
	}
	for _, rel := range relationships {
		inverse, ok := byPair[rel.SourceB+"\x00"+rel.SourceA]
		if !ok {
			return fmt.Errorf("missing inverse for %s", rel.RelationshipID)
		}
		want := rel.Classification
		if rel.Classification == "SUPERSET" {
			want = "SUBSET"
		} else if rel.Classification == "SUBSET" {
			want = "SUPERSET"
		}
		if inverse.Classification != want {
			return fmt.Errorf("inverse for %s is %s, want %s", rel.RelationshipID, inverse.Classification, want)
		}
	}
	return nil
}
