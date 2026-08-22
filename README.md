# repo-rehab

`repo-rehab` is a local-first forensic Git CLI for uncertain repository history.
It creates an inspectable case before anyone modifies, executes, rewrites, or
publishes inherited code.

Status: **pre-release; v0.1 under development**. The repository name is a working
implementation namespace, not a final brand.

## The problem

An inherited project may exist in a developer account, an organization mirror,
an abandoned fork, and several branches that disagree about the latest state.
Default-branch comparisons do not prove which objects or refs would be lost.
Running the project before establishing trust can also trigger hooks, filters,
package scripts, credentials, or network access.

## The v0.1 promise

Given one or more uncertain Git repositories, create a local forensic case that:

- preserves every source as a bare mirror without checkout;
- verifies refs, reachable objects, integrity, and completeness;
- classifies pairwise history as `EXACT`, `SUBSET`, `SUPERSET`, `DIVERGED`,
  `DISJOINT`, or fail-closed `UNKNOWN`;
- proposes reversible, source-namespaced archival refs and withholds collisions;
- records closed safety gates and uncertainty; and
- emits canonical JSON evidence plus a generated Markdown report.

Short form: **preserve + compare + prove + plan**.

## What v0.1 does not do

It does not execute application code, install inherited dependencies, initialize
submodules, fetch Git LFS bodies, modernize code, rewrite history, push refs,
publish repositories, scan licenses/secrets/CVEs, or make ownership and legal
conclusions. Those capabilities are absent, not silently enabled.

## Requirements

- Windows or Linux;
- a native Git executable with the capabilities used by `git clone --mirror`,
  `git fsck`, `git for-each-ref`, `git rev-list`, and `git cat-file`;
- Go 1.27.0 to build from source.

The CLI records the exact Git and tool versions in every case. Go 1.27.0 was the
current stable Go release when this baseline was created; see the official
[Go release history](https://go.dev/doc/devel/release).

## Build and test

```text
go build ./cmd/repo-rehab
go test ./...
go vet ./...
```

No package manager, code generator, or release tool is required.

## Five-minute example

```text
repo-rehab investigate old-company-repo developer-fork org-mirror --case review.case
```

A generated synthetic report can summarize:

```text
developer-fork SUPERSET old-company-repo
developer-fork SUPERSET org-mirror
2 unique commits
3 source-only refs
0 archival mapping collisions
```

The actual machine output is written below `review.case/`; the command never
pushes its proposed plan.

## Resumable commands

```text
repo-rehab case init --case review.case
repo-rehab source add --case review.case old-company-repo
repo-rehab source add --case review.case developer-fork
repo-rehab preserve --case review.case
repo-rehab inspect --case review.case
repo-rehab compare --case review.case
repo-rehab refs plan --case review.case
repo-rehab verify --case review.case
repo-rehab report --case review.case
```

Use `--json` for machine-readable command results and `--json-errors` for
structured errors. Successful fail-closed evidence can contain `UNKNOWN`; it is
not converted into a guessed relationship.

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

`case.json` is canonical machine truth. Policy is human-authored YAML, events and
findings are append-only JSONL, large evidence is content-addressed, and
`report.md` is always a derivative.

## Safety model

Repository inputs are untrusted. Git runs through direct process argument vectors
with isolated system/global configuration, credentials and prompts disabled,
empty hooks/templates, replacement refs disabled during traversal, bounded
output, and timeouts. Acquisition is the only network-capable stage. Offline
analysis uses an explicit command allowlist and has no checkout, mutation, or
remote-write operation.

This is not a sandbox. A vulnerability in the installed Git executable or host
operating system is outside the v0.1 proof. Review [SECURITY.md](SECURITY.md) and
the [safety model](docs/safety-model.md) before using hostile inputs.

## Evidence and design

- [Evidence format](docs/evidence-format.md)
- [Command reference](docs/cli.md)
- [Architecture](docs/architecture.md)
- [Safety and trust boundaries](docs/safety-model.md)
- [Synthetic fixture contract](docs/fixtures.md)
- [Dependency and license record](docs/dependencies.md)

## Contributing

Contributions use Developer Certificate of Origin 1.1 sign-off and do not require
a CLA. See [CONTRIBUTING.md](CONTRIBUTING.md), [DCO](DCO), and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and
[NOTICE](NOTICE).
