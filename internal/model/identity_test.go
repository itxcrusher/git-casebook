package model

import "testing"

func TestPolicyFingerprintIgnoresCaseAndLocatorPaths(t *testing.T) {
	base := Policy{SchemaVersion: SchemaVersion, Profile: "offline-provenance-v1", CaseID: "case-one", Operator: Operator{Kind: "HUMAN", Identifier: "operator"}, Sources: []PolicySource{{SourceID: "source-01", Locator: "/tmp/one", Role: "UNKNOWN", Kind: "local"}}, Limits: Limits{CommandTimeoutSeconds: 1, AcquisitionTimeoutSeconds: 2, MaxCommandOutputBytes: 1024}}
	other := base
	other.CaseID = "case-two"
	other.Sources = append([]PolicySource(nil), base.Sources...)
	other.Sources[0].Locator = "C:/different/path"
	a, err := FingerprintPolicy(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := FingerprintPolicy(other)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("path-independent policy fingerprints differ: %s != %s", a, b)
	}
}
