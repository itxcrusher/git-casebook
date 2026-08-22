package refplan

import (
	"testing"

	"github.com/itxcrusher/git-casebook/internal/model"
)

func TestCollisionWithholdsDestinations(t *testing.T) {
	destination := "refs/heads/archive/source-a/heads/topic"
	sources := []model.Source{{SourceID: "source-a", Refs: []model.Ref{
		{OriginalName: "refs/heads/topic", ObjectID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ProposedMapping: &destination, MappingState: "PROPOSED", CollisionState: "NONE"},
		{OriginalName: "refs/remotes/origin/topic", ObjectID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ProposedMapping: &destination, MappingState: "PROPOSED", CollisionState: "NONE"},
	}}}
	if got := DetectCollisions(sources); got != 1 {
		t.Fatalf("got %d collision groups, want 1", got)
	}
	for _, ref := range sources[0].Refs {
		if ref.ProposedMapping != nil || ref.CollisionState != "COLLISION" || ref.ArchivalDisposition != "REVIEW" {
			t.Fatalf("collision was not fail-closed: %+v", ref)
		}
	}
}

func TestEncodingIsReversibleAndNamespaceSafe(t *testing.T) {
	ref := model.Ref{Type: "PULL", OriginalName: "refs/pull/17/merge"}
	got := destinationFor("source-a", ref)
	if got != "refs/heads/archive/source-a/provider/17/merge" {
		t.Fatalf("unexpected pull mapping %q", got)
	}
	weird := model.Ref{Type: "OTHER", OriginalName: "refs/custom/review/v1.2+safe"}
	got = destinationFor("source-a", weird)
	if got != "refs/archive/source-a/other/refs/custom/review/v1%2E2%2Bsafe" {
		t.Fatalf("unexpected unusual mapping %q", got)
	}
}
