# Contributing

GitCasebook is **feature-frozen** (see
[Maintenance status](README.md#maintenance-status)). New capability needs a real
case demonstrating the need before an issue, let alone a pull request.

Within that boundary it accepts focused changes that strengthen the
deterministic preserve/compare/prove/plan contract, and it actively wants
correctness fixes, safety fixes, unhandled Git and ref shapes, installation
friction, and output that hides the evidence behind a result.

## Where things are documented

Read in this order before a first change:

1. [README](README.md) for the product contract and the v0.1 boundary.
2. [docs/safety-model.md](docs/safety-model.md) for capability levels and what
   is deliberately absent. Absent capabilities are the safety guarantee, not
   oversights.
3. [docs/cli.md](docs/cli.md) for the command surface and exit semantics.
4. [docs/evidence-format.md](docs/evidence-format.md) for canonical case state.
   The schema sets `additionalProperties: false` on every object except the
   `extensions` map, so a new `case.json` field fails validation. That map is
   the sanctioned extension point, and GitCasebook's own reverse-domain key
   inside it is `io.github.itxcrusher.git-casebook`.
5. [docs/architecture.md](docs/architecture.md) for package layout and the Git
   execution boundary, including the `PROBE` / `ACQUISITION` /
   `OFFLINE_ANALYSIS` command classes. Widening an allowlist is a
   safety-boundary change: open an issue first.
6. [docs/fixtures.md](docs/fixtures.md) for the F01-F15 synthetic corpus and its
   provenance rules.
7. [docs/versioning.md](docs/versioning.md) before changing `--json` output or
   the evidence schema.
8. [docs/dependencies.md](docs/dependencies.md) before adding or updating any
   dependency.
9. [SECURITY.md](SECURITY.md) for the private disclosure process.

## Before opening a change

1. Open an issue for behavior that changes the evidence model, relationship
   semantics, safety boundary, or command surface.
2. Use only project-authored synthetic fixtures or material with explicit,
   compatible redistribution rights.
3. Do not submit private repositories, credentials, customer evidence, copied
   competitor code, or fixtures derived from uncleared sources.
4. Keep inherited checkout, execution, package installation, history rewriting,
   remote writes, and publication outside v0.1.
5. Deterministic evidence comes first, agent interpretation second, human
   authority last. Any inference must be labeled as inference, and
   authority-bearing decisions stay with the operator.
6. Reimplementing Git object semantics that native Git already provides
   correctly is a bug, not an optimization.

## Development checks

Use Go 1.27.0 (see `go.mod`) and a native Git executable on `PATH`. The tool
fails closed without one, and the integration tests build real synthetic
repositories, so the full run takes several minutes.

```text
go build ./cmd/git-casebook
go build ./...
go vet ./...
test -z "$(gofmt -l .)"
go test ./... -count=1 -timeout=20m
```

`gofmt -l .` exits 0 even while listing unformatted files, which is why the
check above wraps it; CI uses the same form. `gofmt -w .` fixes what it finds.

**Passing those commands is not evidence that CI will pass.** CI also runs
a race-detector pass, a Linux network-namespace and symlink-confinement run, and
a release-artifact dry run. A change touching the Git boundary should reproduce
the race pass locally:

```text
go test -race ./internal/... -count=1 -timeout=15m
```

See `.github/workflows/ci.yml` for the full matrix.

Changes to Git behavior need a synthetic regression fixture and a deterministic
expected result. Safety changes need a harmless negative/trap test.

## Developer Certificate of Origin

This project uses Developer Certificate of Origin 1.1 and does not require a CLA.
Every commit must include a `Signed-off-by` trailer confirming the certification
in [DCO](DCO):

```text
git commit -s -m "type: concise description"
```

The sign-off name and email must match the contributor identity for that commit.
Do not add sign-offs for another person without their authorization.

## Pull requests

- Target `dev`; `main` contains reviewed stable history.
- Complete `.github/pull_request_template.md`. It is the merge contract.
- Use one concern per commit and conventional commit messages.
- Explain evidence-format or compatibility impact.
- State which checks ran and on which platform.
- Identify new dependencies and their licenses. A new dependency must also be
  recorded in `docs/dependencies.md`, have its license text retained in
  `THIRD_PARTY_NOTICES.md`, and update `NOTICE` where attribution is required.
- Confirm that no repository-controlled content executes in Levels 0-2.

The project currently has one maintainer, who reviews and merges. A separate
governance document is intentionally deferred until real contributor roles
require one.

Published tags and release assets are immutable. A release is never moved,
replaced, or edited after publication.
