# Synthetic Fixture Contract

All test repositories are generated from new synthetic commits at test time. No
private repository, competitor fixture, or inherited application code is used.

| ID | Condition | Required result |
| --- | --- | --- |
| F01 | Differently named repositories with equal reachable history | `EXACT` both directions |
| F02 | B contains A plus a commit | A `SUBSET` B; B `SUPERSET` A |
| F03 | Shared ancestor and unique commits on both sides | `DIVERGED` both directions |
| F04 | Independent roots | `DISJOINT` both directions |
| F05 | Equal `main`, unique feature branch in B | A `SUBSET` B; feature ref retained |
| F06 | Provider-style pull head and merge refs | A `SUBSET` B; refs remapped outside provider namespace |
| F07 | Two candidates forced to one destination | Collision recorded and both destinations withheld |
| F08 | Lightweight, annotated, shared, and source-only tags | Peeling retained; set relationship correct |
| F09 | Placeholder default and richer non-default branch | Both surfaced without product-intent inference |
| F10 | Complete source and depth-one shallow source | `UNKNOWN`; shallow source incomplete |
| F11 | Referenced loose commit object removed | Integrity not verified; `UNKNOWN` |
| F12 | `.gitmodules` plus gitlink | Declaration/path inventoried; nothing fetched |
| F13 | Valid Git LFS pointer without local payload | Pointer recorded; payload `NO`; incomplete |
| F14 | Nested and unusual valid refs | Reversible, valid, unique mappings |
| F15 | A old, B superset, C diverged | A subset B/C; B and C diverged; no canonical guess |

Additional invariants cover:

- input and preserved-source immutability;
- exact/diverged/disjoint symmetry and superset/subset inverse;
- commit and object containment proof;
- deterministic semantic fingerprints across different case paths/times/IDs;
- JSON Schema plus cross-record semantic validation;
- no network-capable offline command;
- no inherited hook, package, executable, submodule, filter, or textconv behavior;
- content-addressed artifact verification; and
- Linux symlink confinement.

The fixture generator itself is trusted test tooling. It invokes Git directly
with fixed synthetic identity and timestamps, controlled config, direct argument
vectors, and reserved `example.invalid` identities. The production runner never
uses the fixture generator.
