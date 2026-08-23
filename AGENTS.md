# Agent Instructions

GitCasebook is a local, deterministic Git-forensics CLI. It preserves uncertain
Git sources, proves how their histories relate, and plans a collision-safe
archival baseline. It never executes, rewrites, or pushes the sources it
examines.

## Read order

1. `README.md` for the product contract and the v0.1 boundary.
2. `docs/safety-model.md` for capability levels and what is deliberately absent.
3. `docs/cli.md` for the command surface and exit semantics.
4. `docs/evidence-format.md` for canonical case state.
5. `docs/architecture.md` for package layout and the Git execution boundary.
6. `docs/fixtures.md` for the F01-F15 synthetic corpus and its provenance rules.
7. `docs/versioning.md` for pre-1.0 compatibility rules.
8. `docs/dependencies.md` before adding or updating any dependency.
9. `CONTRIBUTING.md` and `SECURITY.md` for the contributor and disclosure
   process.

## Prerequisites

- Go 1.27.0 (see `go.mod`).
- A native `git` on `PATH`. The tool fails closed without one, and the
  integration tests build real synthetic repositories.

## Build and test

```text
go build ./...
go vet ./...
test -z "$(gofmt -l .)"
go test ./... -count=1 -timeout=20m
```

`gofmt -l .` exits 0 even when it lists unformatted files, which is why the
check above wraps it; CI uses the same form. `gofmt -w .` fixes what it finds.

The full run takes several minutes because `./test` creates real Git
repositories. CI additionally runs, and a change touching the Git boundary
should reproduce locally:

```text
go test -race ./internal/... -count=1 -timeout=15m
```

plus a Linux network-namespace and symlink-confinement run and a release
artifact dry run. See `.github/workflows/ci.yml`. Passing only the four commands
above is not evidence that CI will pass.

## Contributing changes

- Open an issue first for anything that changes the evidence model,
  relationship semantics, the safety boundary, or the command surface.
- Branch from `dev` and open a pull request against `dev`. `main` holds
  reviewed stable history and is updated from `dev`.
- Complete `.github/pull_request_template.md`. It is the merge contract.
- Every commit needs a DCO `Signed-off-by` trailer: `git commit -s`. See
  `CONTRIBUTING.md` and `DCO`. The name and email must match the commit author.
- Conventional commit subjects.
- A change to Git behavior needs a synthetic regression fixture.
- The maintainer reviews and merges.

## Design rules

- **Deterministic evidence first, agent interpretation second, human authority
  last.** Deterministic tooling establishes facts. Any inference must be
  labeled as inference. Authority-bearing decisions stay with the operator.
- **Fail closed.** When evidence is insufficient, return `UNKNOWN` with a
  reason. Never manufacture a stronger classification than the objects support.
- `case.json` is canonical machine state. `report.md` is generated from it and
  is never authoritative.
- **Git runs through a command-class allowlist** in `internal/gitexec/runner.go`:
  `PROBE`, `ACQUISITION`, and `OFFLINE_ANALYSIS`. Any subcommand outside its
  class is rejected before the process is created, which is what enforces the
  offline-analysis guarantee. Widening an allowlist is a safety-boundary change:
  open an issue first.
- Git is invoked as a direct subprocess with an argument vector, never a shell
  string. Inherited Git configuration is replaced by an isolated home and empty
  global config, and credential helpers, prompts, hooks, templates,
  replace-object traversal, and LFS smudge are disabled.
- Only source acquisition may use the network. Analysis runs offline.
- This is not an operating-system sandbox. Native Git still parses untrusted
  objects, so containment comes from never checking out and never executing.
- Changing `--json` output or the evidence schema is a compatibility change.
  Read `docs/versioning.md` first. The schema is `schema/case-v1.schema.json`
  and sets `additionalProperties: false` throughout, so a new `case.json` field
  fails validation; the sanctioned extension point is the `extensions` map
  under `io.github.itxcrusher.git-casebook`.
- Reimplementing Git object semantics that native Git already provides
  correctly is a bug, not an optimization.

## Do not

- Add checkout, execution, install, rewrite, push, or publication capability.
  Their absence is the product's safety guarantee, not an oversight.
- Add a dependency without recording it in `docs/dependencies.md`, retaining
  its license text in `THIRD_PARTY_NOTICES.md`, and updating `NOTICE` where
  attribution is required.
- Add fixtures other than project-authored synthetic ones or material with
  explicit, compatible redistribution rights. The corpus must stay clean-room
  and deterministic: fixed synthetic identities, fixed timestamps,
  `example.invalid` addresses.
- Submit private repository contents, credentials, or real customer evidence in
  an issue, a fixture, or a test.
- Move or replace a published tag, or edit a published release's assets.
