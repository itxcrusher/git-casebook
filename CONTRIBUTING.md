# Contributing

GitCasebook accepts focused changes that strengthen the deterministic
preserve/compare/prove/plan contract.

## Where things are documented

Read in this order before a first change:

1. [README](README.md) for the product contract and the v0.1 boundary.
2. [docs/safety-model.md](docs/safety-model.md) for capability levels and what
   is deliberately absent. Absent capabilities are the safety guarantee, not
   oversights.
3. [docs/cli.md](docs/cli.md) for the command surface and exit semantics.
4. [docs/evidence-format.md](docs/evidence-format.md) for canonical case state.
   The schema sets `additionalProperties: false` throughout, so a new
   `case.json` field fails validation; the sanctioned extension point is the
   `extensions` map.
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

## Before opening a change

1. Open an issue for behavior that changes the evidence model, relationship
   semantics, safety boundary, or command surface.
2. Use only project-authored synthetic fixtures or material with explicit,
   compatible redistribution rights.
3. Do not submit private repositories, credentials, customer evidence, copied
   competitor code, or fixtures derived from uncleared sources.
4. Keep inherited execution, package installation, history rewriting, remote
   writes, and publication outside v0.1.

## Development checks

Use Go 1.27.0 (see `go.mod`) and a native Git executable on `PATH`. The tool
fails closed without one, and the integration tests build real synthetic
repositories, so the full run takes several minutes.

```text
go build ./...
go vet ./...
gofmt -w .
go test ./... -count=1 -timeout=20m
```

`gofmt -l .` exits 0 even when it lists unformatted files, so a check that must
fail on unformatted code needs `test -z "$(gofmt -l .)"`. CI uses that form.

**Passing those four commands is not evidence that CI will pass.** CI also runs
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
- Use one concern per commit and conventional commit messages.
- Explain evidence-format or compatibility impact.
- State which checks ran and on which platform.
- Identify new dependencies and their licenses. A new dependency must also be
  recorded in `docs/dependencies.md`, have its license text retained in
  `THIRD_PARTY_NOTICES.md`, and update `NOTICE` where attribution is required.
- Confirm that no repository-controlled content executes in Levels 0-2.
- Do not move or replace a published tag, or edit a published release's assets.

The project currently has one maintainer. A separate governance document is
intentionally deferred until real contributor roles require one.
