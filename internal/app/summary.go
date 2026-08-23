package app

import (
	"fmt"
	"sort"

	"github.com/itxcrusher/git-casebook/internal/model"
)

// SummaryLines renders the case result for a human reading the terminal.
//
// It is a view over case.json and carries no authority: case.json remains the
// canonical state and report.md the durable human artifact. Its only job is to
// put the answer the operator asked for on stdout instead of leaving it in a
// file they have to go and find.
func SummaryLines(c model.Case) []string {
	lines := []string{fmt.Sprintf("Case %s: %d source(s), status %s", c.CaseID, len(c.Sources), c.Status)}

	for _, source := range c.Sources {
		if source.Integrity != "VERIFIED" || source.Completeness != "COMPLETE" {
			lines = append(lines, fmt.Sprintf("  %s: integrity %s, completeness %s",
				source.SourceID, source.Integrity, source.Completeness))
		}
	}

	for _, rel := range pairwise(c.Relationships) {
		lines = append(lines, "  "+relationshipLine(rel))
	}

	refs, collisions, review := refTotals(c)
	lines = append(lines, fmt.Sprintf("  %d ref(s) inventoried, %d mapping collision(s), %d ref(s) needing review",
		refs, collisions, review))
	return lines
}

// pairwise returns one directional record per unordered source pair, preferring
// the direction that states the containment positively so the reader sees
// "A SUPERSET B" rather than the equivalent inverse.
func pairwise(relationships []model.Relationship) []model.Relationship {
	seen := make(map[string]bool, len(relationships))
	chosen := make([]model.Relationship, 0, len(relationships))
	byPair := make(map[string][]model.Relationship, len(relationships))
	order := make([]string, 0, len(relationships))

	for _, rel := range relationships {
		key := pairKey(rel.SourceA, rel.SourceB)
		if !seen[key] {
			seen[key] = true
			order = append(order, key)
		}
		byPair[key] = append(byPair[key], rel)
	}

	for _, key := range order {
		candidates := byPair[key]
		best := candidates[0]
		for _, candidate := range candidates[1:] {
			if preferDirection(candidate, best) {
				best = candidate
			}
		}
		chosen = append(chosen, best)
	}
	return chosen
}

func pairKey(a, b string) string {
	pair := []string{a, b}
	sort.Strings(pair)
	return pair[0] + "\x00" + pair[1]
}

// preferDirection picks SUPERSET over its SUBSET inverse, then falls back to a
// stable source-ID ordering so output does not depend on map iteration.
func preferDirection(candidate, current model.Relationship) bool {
	if candidate.Classification == "SUPERSET" && current.Classification != "SUPERSET" {
		return true
	}
	if current.Classification == "SUPERSET" && candidate.Classification != "SUPERSET" {
		return false
	}
	return candidate.SourceA < current.SourceA
}

func relationshipLine(rel model.Relationship) string {
	line := fmt.Sprintf("%s %s %s", rel.SourceA, rel.Classification, rel.SourceB)
	if detail := relationshipDetail(rel); detail != "" {
		line += " (" + detail + ")"
	}
	return line
}

func relationshipDetail(rel model.Relationship) string {
	switch rel.Classification {
	case "EXACT":
		return fmt.Sprintf("%d shared commits", rel.SharedCommitCount)
	case "UNKNOWN":
		if len(rel.ReasonCodes) > 0 {
			return "evidence insufficient: " + rel.ReasonCodes[0]
		}
		return "evidence insufficient to classify safely"
	case "DISJOINT":
		return "no shared commit"
	}

	if rel.SourceAOnlyCommitCount == 0 && rel.SourceBOnlyCommitCount == 0 {
		// Object-only containment. Reporting the classification without this
		// detail is what makes a correct result read as a contradiction.
		if unique := rel.SourceAOnlyObjectCount + rel.SourceBOnlyObjectCount; unique > 0 {
			return fmt.Sprintf("no unique commits; %d object(s) held by one source only", unique)
		}
		return fmt.Sprintf("%d shared commits", rel.SharedCommitCount)
	}

	if rel.SourceAOnlyCommitCount > 0 && rel.SourceBOnlyCommitCount > 0 {
		return fmt.Sprintf("%d commit(s) only in %s, %d only in %s",
			rel.SourceAOnlyCommitCount, rel.SourceA, rel.SourceBOnlyCommitCount, rel.SourceB)
	}
	if rel.SourceAOnlyCommitCount > 0 {
		return fmt.Sprintf("%d commit(s) only in %s", rel.SourceAOnlyCommitCount, rel.SourceA)
	}
	return fmt.Sprintf("%d commit(s) only in %s", rel.SourceBOnlyCommitCount, rel.SourceB)
}

func refTotals(c model.Case) (refs, collisions, review int) {
	for _, source := range c.Sources {
		for _, ref := range source.Refs {
			refs++
			if ref.CollisionState == "COLLISION" {
				collisions++
			}
			if ref.ArchivalDisposition == "REVIEW" {
				review++
			}
		}
	}
	return refs, collisions, review
}
