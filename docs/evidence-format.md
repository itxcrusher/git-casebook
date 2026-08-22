# Evidence Format

## Authority

The evidence flow has one machine authority:

```text
policy.yaml input
  -> case.json + content-addressed artifacts + append-only JSONL
  -> report.md
```

`report.md` is generated and must never be edited as source truth.

## Case files

| Path | Purpose |
| --- | --- |
| `policy.yaml` | Human-authored source declarations, operator, and limits |
| `case.json` | Canonical versioned machine state |
| `events.jsonl` | Append-only action and gate audit |
| `findings.jsonl` | Append-only labeled human/agent findings; empty in the deterministic happy path |
| `sources/*.git` | Bare preserved mirrors, separate from derived evidence |
| `artifacts/sha256/*` | Immutable manifests, `fsck` output, comparison proofs, and ref-plan rows |
| `control/` | Isolated Git home/config/hooks/templates used by this case |
| `report.md` | Concise human rendering linked to the case fingerprint |

## Schema

[`schema/case-v1.schema.json`](../schema/case-v1.schema.json) uses JSON Schema
Draft 2020-12 and semantic version `1.0.0`. Unknown fields are rejected.
Extensions must live under a reverse-domain-style key inside `extensions`.

The Go validator adds cross-record checks that JSON Schema cannot express:

- unique stable IDs and source refs;
- source count equality;
- valid Git object IDs and SHA-256 fingerprints;
- known relationship sources and evidence items;
- complete directional inverse relationships;
- collision groups with no active destination; and
- exact semantic case fingerprint reproduction.

Major schema versions may be incompatible. A future minor version may add
optional meaning only when older readers cannot silently misinterpret it. A patch
version may clarify validation without changing semantics.

## Stable identity and fingerprints

- Case IDs are random stable metadata for one case and do not define semantic
  equality.
- Default source IDs are ordered `source-01`, `source-02`, and so on; users may
  supply their own valid stable IDs.
- Relationship and evidence IDs derive from source IDs and operation type.
- Sidecar filenames are the SHA-256 digest of their exact bytes.
- Commit/object-set digests hash sorted full object IDs.
- The semantic case fingerprint excludes absolute paths, case ID, observation
  timestamps, elapsed duration, acquisition directory, platform label, and raw
  mirror filesystem fingerprints.

Moving a case does not change its semantic identity. A different Git version is
retained as a decision-relevant input and can change the case fingerprint.

## Facts, findings, gates, and decisions

Relationships, refs, object manifests, and integrity results are deterministic
facts. Findings are separately typed as deterministic fact, agent inference,
human assertion, or unresolved uncertainty. Decisions remain proposed/accepted
human or agent recommendations and never become Git facts.

Unsafe gates are closed by default. A report or agent cannot open one by editing
Markdown. v0.1 has no implementation for later gate capabilities.

## Sensitive evidence

A case may contain local paths, remote locators, ref names, object IDs, and
provenance metadata. URLs containing passwords, query strings, or fragments are
rejected, but this is not a secret scanner. Review a case before sharing it.
