# Versioning and Compatibility

## Product versions

GitCasebook uses Semantic Versioning. The first planned release is `v0.1.0`.
During `0.x`, minor versions may change commands, machine output, or case
behavior when the change is necessary to correct forensic semantics or improve
the pre-1.0 contract. Patch versions remain backward-compatible bug and security
fixes within that minor line where practical.

Every case records the exact GitCasebook and native Git versions that produced
its evidence. Users should retain those fields when sharing or comparing cases.

## Evidence schema versions

The evidence schema has its own semantic version and is not tied to the binary
version. v0.1.0 writes case schema `1.0.0`.

- Schema patch versions may tighten validation or clarify representation without
  changing meaning.
- Schema minor versions may add optional fields that older readers can safely
  ignore only when no decision-relevant meaning is lost.
- Schema major versions may be incompatible.

Unknown fields are rejected by the v1 schema. An implementation must not silently
reinterpret a newer schema as an older one.

## Migration posture

v0.1 does not mutate older evidence in place. A future migration command must:

1. retain the original case and content-addressed artifacts;
2. declare source and destination schema versions;
3. record the migration producer and tool version;
4. fail closed when meaning cannot be preserved; and
5. emit a new case fingerprint.

Until such a command exists, open a historical case with the GitCasebook version
that produced it or regenerate evidence from unchanged preserved sources.
