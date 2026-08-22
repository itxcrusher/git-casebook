package refplan

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/itxcrusher/repo-rehab/internal/evidence"
	"github.com/itxcrusher/repo-rehab/internal/gitexec"
	"github.com/itxcrusher/repo-rehab/internal/model"
)

type Row struct {
	SourceID       string  `json:"source_id"`
	OriginalRef    string  `json:"original_ref"`
	ObjectID       string  `json:"object_id"`
	RefType        string  `json:"ref_type"`
	DestinationRef *string `json:"destination_ref"`
	CollisionState string  `json:"collision_state"`
	Disposition    string  `json:"disposition"`
}

type Artifact struct {
	SchemaVersion string `json:"schema_version"`
	Rows          []Row  `json:"rows"`
}

func Plan(ctx context.Context, runner *gitexec.Runner, sources []model.Source, artifacts evidence.Store) ([]model.Source, model.EvidenceItem, error) {
	planned := append([]model.Source(nil), sources...)
	var rows []Row
	for sourceIndex := range planned {
		planned[sourceIndex].Refs = append([]model.Ref(nil), planned[sourceIndex].Refs...)
		for refIndex := range planned[sourceIndex].Refs {
			ref := &planned[sourceIndex].Refs[refIndex]
			destination := destinationFor(planned[sourceIndex].SourceID, *ref)
			if _, err := runner.Run(ctx, gitexec.ClassAnalysis, "", "check-ref-format", destination); err != nil {
				ref.ProposedMapping = nil
				ref.MappingState = "REJECTED"
				ref.CollisionState = "NONE"
				ref.ArchivalDisposition = "REVIEW"
				continue
			}
			ref.ProposedMapping = &destination
			ref.MappingState = "PROPOSED"
			ref.CollisionState = "NONE"
			if ref.Type == "BRANCH" || ref.Type == "TAG" {
				ref.ArchivalDisposition = "PRESERVE"
			} else {
				ref.ArchivalDisposition = "REMAP"
			}
		}
	}
	DetectCollisions(planned)
	for _, source := range planned {
		for _, ref := range source.Refs {
			rows = append(rows, Row{
				SourceID: source.SourceID, OriginalRef: ref.OriginalName, ObjectID: ref.ObjectID,
				RefType: ref.Type, DestinationRef: ref.ProposedMapping,
				CollisionState: ref.CollisionState, Disposition: ref.ArchivalDisposition,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SourceID == rows[j].SourceID {
			return rows[i].OriginalRef < rows[j].OriginalRef
		}
		return rows[i].SourceID < rows[j].SourceID
	})
	path, digest, err := artifacts.PutJSON("ref-plan.json", Artifact{SchemaVersion: model.SchemaVersion, Rows: rows})
	if err != nil {
		return nil, model.EvidenceItem{}, err
	}
	item := model.EvidenceItem{
		EvidenceID: "evidence-ref-plan", Producer: "repo-rehab", Method: "source-namespaced-reversible-ref-mapping",
		Inputs: []string{"all-source-refs"}, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano),
		OutputFingerprint: digest, RawArtifact: &path, VerificationState: "VERIFIED",
	}
	return planned, item, nil
}

func DetectCollisions(sources []model.Source) int {
	locations := map[string][]struct{ source, ref int }{}
	for sourceIndex := range sources {
		for refIndex := range sources[sourceIndex].Refs {
			ref := &sources[sourceIndex].Refs[refIndex]
			if ref.ProposedMapping == nil {
				continue
			}
			key := strings.ToLower(*ref.ProposedMapping)
			locations[key] = append(locations[key], struct{ source, ref int }{sourceIndex, refIndex})
		}
	}
	collisions := 0
	for _, entries := range locations {
		if len(entries) < 2 {
			continue
		}
		collisions++
		for _, entry := range entries {
			ref := &sources[entry.source].Refs[entry.ref]
			ref.ProposedMapping = nil
			ref.MappingState = "REJECTED"
			ref.CollisionState = "COLLISION"
			ref.ArchivalDisposition = "REVIEW"
		}
	}
	return collisions
}

func destinationFor(sourceID string, ref model.Ref) string {
	category, prefix, remainder := "other", "refs/archive", ref.OriginalName
	switch ref.Type {
	case "BRANCH":
		category, prefix, remainder = "heads", "refs/heads/archive", strings.TrimPrefix(ref.OriginalName, "refs/heads/")
	case "TAG":
		category, prefix, remainder = "tags", "refs/tags/archive", strings.TrimPrefix(ref.OriginalName, "refs/tags/")
	case "REMOTE":
		category, prefix, remainder = "remotes", "refs/heads/archive", strings.TrimPrefix(ref.OriginalName, "refs/remotes/")
	case "PULL":
		category, prefix, remainder = "provider", "refs/heads/archive", strings.TrimPrefix(strings.TrimPrefix(ref.OriginalName, "refs/"), "pull/")
	case "NOTE":
		category, prefix, remainder = "notes", "refs/notes/archive", strings.TrimPrefix(ref.OriginalName, "refs/notes/")
	case "REPLACE":
		category, prefix, remainder = "replace", "refs/archive", strings.TrimPrefix(ref.OriginalName, "refs/replace/")
	}
	segments := strings.Split(remainder, "/")
	for i, segment := range segments {
		segments[i] = encodeSegment(segment)
	}
	return strings.Join([]string{prefix, encodeSegment(sourceID), category, strings.Join(segments, "/")}, "/")
}

func encodeSegment(value string) string {
	var builder strings.Builder
	for _, b := range []byte(value) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '_' {
			builder.WriteByte(b)
		} else {
			fmt.Fprintf(&builder, "%%%02X", b)
		}
	}
	if builder.Len() == 0 {
		return "%00"
	}
	return builder.String()
}
