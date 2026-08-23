# GitCasebook

**Deterministic Git forensics for repositories you didn't build.**

GitCasebook preserves uncertain Git sources, proves how their histories relate,
and produces a collision-safe archival plan before you modify or execute
anything.

**Preserve. Compare. Prove. Plan.**

Status: **pre-1.0**. Published versions are listed in
[GitHub Releases](https://github.com/itxcrusher/git-casebook/releases).

## The problem

An inherited project may exist in a developer account, an organization mirror,
an abandoned fork, and several branches that disagree about the latest state.
Default-branch comparisons cannot prove which commits, objects, tags, or unusual
refs would be lost. Running the project before establishing trust can also
trigger repository-controlled behavior.

GitCasebook handles the earlier forensic step:

```text
uncertain repositories
    -> preserve sources without checkout
    -> prove all-ref Git relationships
    -> plan canonical archival refs
    -> produce an inspectable forensic case
    -> then archaeology or modernization may begin
```

## One-command investigation

```text
git-casebook investigate old-company-repo developer-copy organization-copy --case review.case
```

When the executable is on `PATH`, Git's ordinary dashed-command discovery also
supports:

```text
git casebook investigate old-company-repo developer-copy organization-copy --case review.case
```

A synthetic case might report:

```text
developer-copy  SUPERSET  organization-copy
2 unique commits
3 source-only refs
0 archival mapping collisions
```

The command writes canonical evidence below `review.case/`. It does not push or
apply the proposed archival plan.

## Testing on real repository sets

GitCasebook v0.1 is looking for independent real cases involving inherited,
duplicated, forked, mirrored, or otherwise historically uncertain repositories.
If you run it on two or more real sources, share the outcome through the
[real repository case form](https://github.com/itxcrusher/git-casebook/issues/new?template=real-case.yml).

Useful reports include successful classifications as well as actionable
fail-closed `UNKNOWN` results. Do not upload private repository contents,
credentials, or sensitive case artifacts; summaries of the preservation question
and the evidence that mattered are enough.

## What GitCasebook guarantees

For supported complete Git sources, v0.1:

- preserves each source as a bare mirror without checkout;
- inventories all relevant refs and all objects reachable from them;
- records integrity, shallow, partial, submodule, and LFS evidence;
- classifies every source pair as `EXACT`, `SUBSET`, `SUPERSET`, `DIVERGED`,
  `DISJOINT`, or fail-closed `UNKNOWN`;
- emits evidence supporting shared and source-only commit/object claims;
- proposes deterministic, reversible, source-namespaced archival refs;
- withholds colliding mappings instead of overwriting them; and
- produces versioned JSON truth plus a derived Markdown report.

## What it deliberately does not do

v0.1 does not execute application code, install inherited dependencies, check
out worktrees, initialize submodules, fetch Git LFS bodies, modernize code,
rewrite history, push refs, publish repositories, scan licenses/secrets/CVEs, or
make ownership and legal conclusions. Those capabilities are absent, not
silently enabled.

GitCasebook is not a code chatbot, specification extractor, dependency graph,
or refactoring engine. Agents and later archaeology tools can consume its
evidence after repository identity and provenance have been established.

## Safety model

Repository inputs are untrusted. GitCasebook invokes native Git directly with
explicit argument vectors, isolated system/global configuration, credentials
and prompts disabled, empty hooks/templates, replacement traversal disabled,
bounded output, and timeouts. Acquisition is the only network-capable stage.
Offline analysis uses an explicit command allowlist and has no checkout,
mutation, or remote-write operation.

This is not an operating-system sandbox. Native Git still parses untrusted
objects. Use separate OS isolation for deliberately hostile inputs. Review the
[security policy](SECURITY.md) and [safety model](docs/safety-model.md).

## Installation

GitCasebook requires a supported native Git executable. The versioned source
installation command for v0.1.1 is:

```text
go install github.com/itxcrusher/git-casebook/cmd/git-casebook@v0.1.1
```

For an untagged development checkout, use:

```text
go install ./cmd/git-casebook
```

Supported and tested targets for publication are Windows, Linux, and macOS. See
[versioning and compatibility](docs/versioning.md) for the 0.x policy.

## Commands

The guided `investigate` command creates or resumes a case, preserves sources,
switches to offline analysis, inventories and compares them, plans archival
refs, verifies evidence, and generates the report.

Resumable lifecycle commands are also available:

```text
git-casebook case init --case review.case
git-casebook source add --case review.case old-company-repo
git-casebook source add --case review.case developer-copy
git-casebook preserve --case review.case
git-casebook inspect --case review.case
git-casebook compare --case review.case
git-casebook refs plan --case review.case
git-casebook verify --case review.case
git-casebook report --case review.case
```

Use `--json` for machine-readable command results and `--json-errors` for
structured errors. See the [command reference](docs/cli.md) for exit semantics.

## Case layout

```text
review.case/
|-- policy.yaml
|-- case.json
|-- events.jsonl
|-- findings.jsonl
|-- sources/
|   `-- source-01.git/
|-- artifacts/
|   `-- sha256/
|-- control/
`-- report.md
```

`case.json` is canonical machine truth. Policy is human-authored YAML, events
and findings are append-only JSONL, large evidence is content-addressed, and
`report.md` is always derived.

## Relationship meanings

GitCasebook compares commit and object sets reachable from all included refs,
not just default branches:

| Relationship | Meaning for source A relative to source B |
| --- | --- |
| `EXACT` | Reachable commit and object sets are equal |
| `SUPERSET` | A properly contains B's reachable commit and object sets |
| `SUBSET` | A's reachable commit and object sets are properly contained by B |
| `DIVERGED` | Sources share reachable commits but neither contains the other |
| `DISJOINT` | Sources share no reachable commit |
| `UNKNOWN` | Evidence is incomplete, corrupt, incompatible, or otherwise untrusted |

`UNKNOWN` is a valid fail-closed outcome, not an error concealed as certainty.

## Current limitations

- Authenticated remote acquisition has no dedicated credential workflow yet.
- Git SHA-1 and SHA-256 repositories are inventoried, but incompatible object
  formats cannot receive a confident cross-format relationship.
- LFS pointer/config state is reported; LFS bodies are not fetched.
- Submodules are reported; their URLs are not followed.
- Ref plans are proposals only and are never pushed.
- v0.1 evidence schemas may evolve under the documented 0.x policy.

## Evidence and design

- [Evidence format](docs/evidence-format.md)
- [Command reference](docs/cli.md)
- [Architecture](docs/architecture.md)
- [Safety and trust boundaries](docs/safety-model.md)
- [Synthetic fixture contract](docs/fixtures.md)
- [Dependencies and licenses](docs/dependencies.md)
- [Versioning and compatibility](docs/versioning.md)

## Contributing

Contributions use Developer Certificate of Origin 1.1 sign-off and do not
require a CLA. See [CONTRIBUTING.md](CONTRIBUTING.md), [DCO](DCO), and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE),
[NOTICE](NOTICE), and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
