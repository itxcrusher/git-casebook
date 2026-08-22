package model

const SchemaVersion = "1.0.0"

type Case struct {
	SchemaVersion   string         `json:"schema_version"`
	CaseID          string         `json:"case_id"`
	CreatedAt       string         `json:"created_at"`
	Tool            Tool           `json:"tool"`
	Status          string         `json:"status"`
	Policy          PolicyEvidence `json:"policy"`
	Operator        Operator       `json:"operator"`
	SourceCount     int            `json:"source_count"`
	CaseFingerprint string         `json:"case_fingerprint"`
	Sources         []Source       `json:"sources"`
	Relationships   []Relationship `json:"relationships"`
	EvidenceItems   []EvidenceItem `json:"evidence_items"`
	Findings        []Finding      `json:"findings"`
	Gates           []Gate         `json:"gates"`
	Decisions       []Decision     `json:"decisions"`
	Extensions      map[string]any `json:"extensions"`
}

type Tool struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	GitVersion string `json:"git_version"`
	Platform   string `json:"platform"`
}

type PolicyEvidence struct {
	Profile                 string `json:"profile"`
	Fingerprint             string `json:"fingerprint"`
	CapabilityLevel         int    `json:"capability_level"`
	NetworkAfterAcquisition bool   `json:"network_after_acquisition"`
}

type Operator struct {
	Kind       string `json:"kind" yaml:"kind"`
	Identifier string `json:"identifier" yaml:"identifier"`
}

type Source struct {
	SourceID           string          `json:"source_id"`
	OriginalLocator    string          `json:"original_locator"`
	LocalMirrorLocator string          `json:"local_mirror_locator"`
	Role               string          `json:"role"`
	DiscoveryMethod    string          `json:"discovery_method"`
	RetrievedAt        string          `json:"retrieved_at"`
	DefaultBranch      *string         `json:"default_branch"`
	RemoteMetadata     RemoteMetadata  `json:"remote_metadata"`
	ObjectFormat       string          `json:"object_format"`
	Shallow            bool            `json:"shallow"`
	PartialClone       bool            `json:"partial_clone"`
	LFS                LFSState        `json:"lfs"`
	Submodules         SubmoduleState  `json:"submodules"`
	Integrity          string          `json:"integrity"`
	Completeness       string          `json:"completeness"`
	Refs               []Ref           `json:"refs"`
	Objects            ObjectInventory `json:"objects"`
	SourceFingerprint  string          `json:"source_fingerprint"`
	Extensions         map[string]any  `json:"extensions"`
}

type RemoteMetadata struct {
	FetchLocators []string `json:"fetch_locators"`
	PushDisabled  bool     `json:"push_disabled"`
}

type LFSState struct {
	Declared         bool   `json:"declared"`
	PointerCount     int    `json:"pointer_count"`
	ObjectsAvailable string `json:"objects_available"`
}

type SubmoduleState struct {
	Declared bool     `json:"declared"`
	Entries  []string `json:"entries"`
	Fetched  bool     `json:"fetched"`
}

type Ref struct {
	OriginalName                string  `json:"original_name"`
	Type                        string  `json:"type"`
	ObjectID                    string  `json:"object_id"`
	PeeledObjectID              *string `json:"peeled_object_id"`
	ProposedMapping             *string `json:"proposed_mapping"`
	MappingState                string  `json:"mapping_state"`
	VerifiedDestinationObjectID *string `json:"verified_destination_object_id"`
	CollisionState              string  `json:"collision_state"`
	ArchivalDisposition         string  `json:"archival_disposition"`
}

type ObjectInventory struct {
	CommitCount              int    `json:"commit_count"`
	ReachableObjectCount     int    `json:"reachable_object_count"`
	AllObjectCount           int    `json:"all_object_count"`
	CommitSetDigest          string `json:"commit_set_digest"`
	ReachableObjectSetDigest string `json:"reachable_object_set_digest"`
	ManifestArtifact         string `json:"manifest_artifact"`
	UnreachableObjectCount   int    `json:"unreachable_object_count"`
}

type Relationship struct {
	RelationshipID         string   `json:"relationship_id"`
	SourceA                string   `json:"source_a"`
	SourceB                string   `json:"source_b"`
	Classification         string   `json:"classification"`
	SharedCommitCount      int      `json:"shared_commit_count"`
	SourceAOnlyCommitCount int      `json:"source_a_only_commit_count"`
	SourceBOnlyCommitCount int      `json:"source_b_only_commit_count"`
	SourceAOnlyObjectCount int      `json:"source_a_only_object_count"`
	SourceBOnlyObjectCount int      `json:"source_b_only_object_count"`
	ReasonCodes            []string `json:"reason_codes"`
	EvidenceIDs            []string `json:"evidence_ids"`
}

type EvidenceItem struct {
	EvidenceID        string   `json:"evidence_id"`
	Producer          string   `json:"producer"`
	Method            string   `json:"method"`
	Inputs            []string `json:"inputs"`
	ObservedAt        string   `json:"observed_at"`
	OutputFingerprint string   `json:"output_fingerprint"`
	RawArtifact       *string  `json:"raw_artifact"`
	VerificationState string   `json:"verification_state"`
}

type Finding struct {
	FindingID         string   `json:"finding_id"`
	Kind              string   `json:"kind"`
	Summary           string   `json:"summary"`
	EvidenceIDs       []string `json:"evidence_ids"`
	VerificationState string   `json:"verification_state"`
}

type Gate struct {
	GateID    string  `json:"gate_id"`
	Action    string  `json:"action"`
	State     string  `json:"state"`
	Scope     string  `json:"scope"`
	Authority *string `json:"authority"`
	Reason    string  `json:"reason"`
	ChangedAt string  `json:"changed_at"`
}

type Decision struct {
	DecisionID     string   `json:"decision_id"`
	Recommendation string   `json:"recommendation"`
	ActorKind      string   `json:"actor_kind"`
	Status         string   `json:"status"`
	EvidenceIDs    []string `json:"evidence_ids"`
	Rationale      string   `json:"rationale"`
}

type Policy struct {
	SchemaVersion string         `yaml:"schema_version"`
	Profile       string         `yaml:"profile"`
	CaseID        string         `yaml:"case_id"`
	Operator      Operator       `yaml:"operator"`
	Sources       []PolicySource `yaml:"sources"`
	Limits        Limits         `yaml:"limits"`
}

type PolicySource struct {
	SourceID string `yaml:"source_id"`
	Locator  string `yaml:"locator"`
	Role     string `yaml:"role"`
	Kind     string `yaml:"kind"`
}

type Limits struct {
	CommandTimeoutSeconds     int   `yaml:"command_timeout_seconds"`
	AcquisitionTimeoutSeconds int   `yaml:"acquisition_timeout_seconds"`
	MaxCommandOutputBytes     int64 `yaml:"max_command_output_bytes"`
}

type ManifestIndex struct {
	SchemaVersion           string `json:"schema_version"`
	CommitManifest          string `json:"commit_manifest"`
	ReachableObjectManifest string `json:"reachable_object_manifest"`
	AllObjectManifest       string `json:"all_object_manifest"`
}

type RelationshipArtifact struct {
	SchemaVersion      string   `json:"schema_version"`
	SourceA            string   `json:"source_a"`
	SourceB            string   `json:"source_b"`
	SharedCommits      []string `json:"shared_commits"`
	SourceAOnlyCommits []string `json:"source_a_only_commits"`
	SourceBOnlyCommits []string `json:"source_b_only_commits"`
	SourceAOnlyObjects []string `json:"source_a_only_objects"`
	SourceBOnlyObjects []string `json:"source_b_only_objects"`
}

type Event struct {
	EventID   string         `json:"event_id"`
	Kind      string         `json:"kind"`
	Timestamp string         `json:"timestamp"`
	Actor     string         `json:"actor"`
	Details   map[string]any `json:"details"`
}
