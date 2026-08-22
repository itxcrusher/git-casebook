package model

import (
	"fmt"
	"sort"
)

var classifications = map[string]bool{
	"EXACT": true, "SUPERSET": true, "SUBSET": true,
	"DIVERGED": true, "DISJOINT": true, "UNKNOWN": true,
}

func Validate(c Case) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %q", c.SchemaVersion)
	}
	if !IsStableID(c.CaseID) {
		return fmt.Errorf("invalid case_id %q", c.CaseID)
	}
	if c.SourceCount != len(c.Sources) || c.SourceCount < 1 {
		return fmt.Errorf("source_count %d does not match %d sources", c.SourceCount, len(c.Sources))
	}
	if !IsSHA256(c.CaseFingerprint) {
		return fmt.Errorf("invalid case_fingerprint")
	}
	if c.Policy.CapabilityLevel < 0 || c.Policy.CapabilityLevel > 2 {
		return fmt.Errorf("v0.1 capability_level must be between 0 and 2")
	}
	if c.Policy.NetworkAfterAcquisition {
		return fmt.Errorf("network_after_acquisition must be false")
	}
	if !IsSHA256(c.Policy.Fingerprint) {
		return fmt.Errorf("invalid policy fingerprint")
	}
	sourceIDs := make(map[string]bool, len(c.Sources))
	for _, source := range c.Sources {
		if !IsStableID(source.SourceID) || sourceIDs[source.SourceID] {
			return fmt.Errorf("invalid or duplicate source_id %q", source.SourceID)
		}
		sourceIDs[source.SourceID] = true
		if !IsSHA256(source.SourceFingerprint) {
			return fmt.Errorf("source %s has invalid fingerprint", source.SourceID)
		}
		seenRefs := map[string]bool{}
		for _, ref := range source.Refs {
			if seenRefs[ref.OriginalName] {
				return fmt.Errorf("source %s has duplicate ref %q", source.SourceID, ref.OriginalName)
			}
			seenRefs[ref.OriginalName] = true
			if !IsOID(ref.ObjectID) {
				return fmt.Errorf("source %s ref %q has invalid object id", source.SourceID, ref.OriginalName)
			}
			if ref.PeeledObjectID != nil && !IsOID(*ref.PeeledObjectID) {
				return fmt.Errorf("source %s ref %q has invalid peeled object id", source.SourceID, ref.OriginalName)
			}
			if ref.CollisionState == "COLLISION" && ref.ProposedMapping != nil {
				return fmt.Errorf("colliding ref %q retains an active destination", ref.OriginalName)
			}
		}
		if source.Objects.CommitCount < 0 || source.Objects.ReachableObjectCount < 0 || source.Objects.AllObjectCount < 0 {
			return fmt.Errorf("source %s has negative object counts", source.SourceID)
		}
		if !IsSHA256(source.Objects.CommitSetDigest) || !IsSHA256(source.Objects.ReachableObjectSetDigest) {
			return fmt.Errorf("source %s has invalid object digests", source.SourceID)
		}
	}
	relationshipIDs := map[string]bool{}
	byPair := map[string]Relationship{}
	for _, rel := range c.Relationships {
		if !IsStableID(rel.RelationshipID) || relationshipIDs[rel.RelationshipID] {
			return fmt.Errorf("invalid or duplicate relationship_id %q", rel.RelationshipID)
		}
		relationshipIDs[rel.RelationshipID] = true
		if !sourceIDs[rel.SourceA] || !sourceIDs[rel.SourceB] || rel.SourceA == rel.SourceB {
			return fmt.Errorf("relationship %s references invalid sources", rel.RelationshipID)
		}
		if !classifications[rel.Classification] {
			return fmt.Errorf("relationship %s has invalid classification", rel.RelationshipID)
		}
		if rel.SharedCommitCount < 0 || rel.SourceAOnlyCommitCount < 0 || rel.SourceBOnlyCommitCount < 0 || rel.SourceAOnlyObjectCount < 0 || rel.SourceBOnlyObjectCount < 0 {
			return fmt.Errorf("relationship %s has negative counts", rel.RelationshipID)
		}
		byPair[rel.SourceA+"\x00"+rel.SourceB] = rel
	}
	for _, rel := range c.Relationships {
		inverse, ok := byPair[rel.SourceB+"\x00"+rel.SourceA]
		if !ok {
			return fmt.Errorf("relationship %s is missing its inverse", rel.RelationshipID)
		}
		want := inverseClassification(rel.Classification)
		if inverse.Classification != want {
			return fmt.Errorf("relationship %s inverse is %s, want %s", rel.RelationshipID, inverse.Classification, want)
		}
		if rel.SharedCommitCount != inverse.SharedCommitCount || rel.SourceAOnlyCommitCount != inverse.SourceBOnlyCommitCount || rel.SourceBOnlyCommitCount != inverse.SourceAOnlyCommitCount {
			return fmt.Errorf("relationship %s inverse counts do not match", rel.RelationshipID)
		}
	}
	evidenceIDs := map[string]bool{}
	for _, item := range c.EvidenceItems {
		if !IsStableID(item.EvidenceID) || evidenceIDs[item.EvidenceID] {
			return fmt.Errorf("invalid or duplicate evidence_id %q", item.EvidenceID)
		}
		evidenceIDs[item.EvidenceID] = true
		if !IsSHA256(item.OutputFingerprint) {
			return fmt.Errorf("evidence %s has invalid output fingerprint", item.EvidenceID)
		}
	}
	for _, rel := range c.Relationships {
		for _, id := range rel.EvidenceIDs {
			if !evidenceIDs[id] {
				return fmt.Errorf("relationship %s references unknown evidence %s", rel.RelationshipID, id)
			}
		}
	}
	findingIDs := map[string]bool{}
	for _, finding := range c.Findings {
		if !IsStableID(finding.FindingID) || findingIDs[finding.FindingID] {
			return fmt.Errorf("invalid or duplicate finding_id %q", finding.FindingID)
		}
		findingIDs[finding.FindingID] = true
		for _, id := range finding.EvidenceIDs {
			if !evidenceIDs[id] {
				return fmt.Errorf("finding %s references unknown evidence %s", finding.FindingID, id)
			}
		}
	}
	gateIDs := map[string]bool{}
	for _, gate := range c.Gates {
		if !IsStableID(gate.GateID) || gateIDs[gate.GateID] {
			return fmt.Errorf("invalid or duplicate gate_id %q", gate.GateID)
		}
		gateIDs[gate.GateID] = true
	}
	decisionIDs := map[string]bool{}
	for _, decision := range c.Decisions {
		if !IsStableID(decision.DecisionID) || decisionIDs[decision.DecisionID] {
			return fmt.Errorf("invalid or duplicate decision_id %q", decision.DecisionID)
		}
		decisionIDs[decision.DecisionID] = true
		for _, id := range decision.EvidenceIDs {
			if !evidenceIDs[id] {
				return fmt.Errorf("decision %s references unknown evidence %s", decision.DecisionID, id)
			}
		}
	}
	computed, err := FingerprintCase(c)
	if err != nil {
		return err
	}
	if computed != c.CaseFingerprint {
		return fmt.Errorf("case fingerprint mismatch: got %s, want %s", c.CaseFingerprint, computed)
	}
	return nil
}

func inverseClassification(classification string) string {
	switch classification {
	case "SUPERSET":
		return "SUBSET"
	case "SUBSET":
		return "SUPERSET"
	default:
		return classification
	}
}

func Normalize(c *Case) {
	if c.Sources == nil {
		c.Sources = []Source{}
	}
	if c.Relationships == nil {
		c.Relationships = []Relationship{}
	}
	if c.EvidenceItems == nil {
		c.EvidenceItems = []EvidenceItem{}
	}
	if c.Findings == nil {
		c.Findings = []Finding{}
	}
	if c.Gates == nil {
		c.Gates = []Gate{}
	}
	if c.Decisions == nil {
		c.Decisions = []Decision{}
	}
	if c.Extensions == nil {
		c.Extensions = map[string]any{}
	}
	sort.Slice(c.Sources, func(i, j int) bool { return c.Sources[i].SourceID < c.Sources[j].SourceID })
	for i := range c.Sources {
		if c.Sources[i].Refs == nil {
			c.Sources[i].Refs = []Ref{}
		}
		if c.Sources[i].RemoteMetadata.FetchLocators == nil {
			c.Sources[i].RemoteMetadata.FetchLocators = []string{}
		}
		if c.Sources[i].Submodules.Entries == nil {
			c.Sources[i].Submodules.Entries = []string{}
		}
		if c.Sources[i].Extensions == nil {
			c.Sources[i].Extensions = map[string]any{}
		}
		sort.Slice(c.Sources[i].Refs, func(a, b int) bool { return c.Sources[i].Refs[a].OriginalName < c.Sources[i].Refs[b].OriginalName })
		sort.Strings(c.Sources[i].RemoteMetadata.FetchLocators)
		sort.Strings(c.Sources[i].Submodules.Entries)
	}
	sort.Slice(c.Relationships, func(i, j int) bool { return c.Relationships[i].RelationshipID < c.Relationships[j].RelationshipID })
	for i := range c.Relationships {
		if c.Relationships[i].ReasonCodes == nil {
			c.Relationships[i].ReasonCodes = []string{}
		}
		if c.Relationships[i].EvidenceIDs == nil {
			c.Relationships[i].EvidenceIDs = []string{}
		}
		sort.Strings(c.Relationships[i].ReasonCodes)
		sort.Strings(c.Relationships[i].EvidenceIDs)
	}
	sort.Slice(c.EvidenceItems, func(i, j int) bool { return c.EvidenceItems[i].EvidenceID < c.EvidenceItems[j].EvidenceID })
	sort.Slice(c.Findings, func(i, j int) bool { return c.Findings[i].FindingID < c.Findings[j].FindingID })
	for i := range c.Findings {
		if c.Findings[i].EvidenceIDs == nil {
			c.Findings[i].EvidenceIDs = []string{}
		}
	}
	sort.Slice(c.Gates, func(i, j int) bool { return c.Gates[i].GateID < c.Gates[j].GateID })
	sort.Slice(c.Decisions, func(i, j int) bool { return c.Decisions[i].DecisionID < c.Decisions[j].DecisionID })
	for i := range c.Decisions {
		if c.Decisions[i].EvidenceIDs == nil {
			c.Decisions[i].EvidenceIDs = []string{}
		}
	}
}
