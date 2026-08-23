# Agent Instructions

GitCasebook is a local, deterministic Git-forensics CLI. It preserves uncertain
Git sources, proves how their histories relate, and plans a collision-safe
archival baseline. It never executes, rewrites, or pushes the repositories it
examines.

## Read order

1. `README.md` for the product contract and the v0.1 boundary.
2. `docs/safety-model.md` for capability levels and what is deliberately absent.
3. `docs/evidence-format.md` for canonical case state.
4. `docs/architecture.md` for package layout and the Git execution boundary.
5. `docs/versioning.md` for pre-1.0 compatibility rules.
6. `CONTRIBUTING.md` for the contributor workflow and DCO.

## Build and test

```bash
go build ./...
go test ./...
go vet ./...
gofmt -l .
```

The full suite includes integration tests that create real synthetic Git
repositories and takes several minutes. Run it before any merge; a green
subset is not sufficient evidence.

## Branching and review

- `main` is stable, reviewed history. `dev` is where work lands.
- Work directly on `dev` when a change does not warrant review.
- Open a pull request when it does.
- **Before merging any pull request, dispatch an adversarial reviewer to
  challenge the change and return a recommendation.** Act on it, then merge.
  This review is required. A reviewer that finds nothing must say so
  explicitly rather than be skipped.
- Conventional commits, explicit staging, no AI attribution in commit messages
  or pull request descriptions.

## Design rules

- **Deterministic evidence first, agent interpretation second, human authority
  last.** Deterministic tooling establishes facts. Any inference must be
  labeled as inference. Authority-bearing decisions stay with the operator.
- **Fail closed.** When evidence is insufficient, return `UNKNOWN` with a
  reason. Never manufacture a stronger classification than the objects support.
- `case.json` is canonical machine state. `report.md` is a generated view and
  is never authoritative.
- Git is invoked as a subprocess without a shell, with inherited
  configuration, credential helpers, hooks, and filters disabled.
- Only source acquisition may use the network. Analysis runs offline.
- Changing `--json` output or the evidence schema is a compatibility change:
  read `docs/versioning.md` first.

## Do not

- Add checkout, execution, install, rewrite, push, or publication capability.
  Their absence is the product's safety guarantee, not an oversight.
- Reimplement Git object semantics that native Git already provides correctly.
- Introduce a dependency without recording it in `docs/dependencies.md`.
- Add fixtures from real-world or third-party repositories. The test corpus is
  clean-room synthetic and must stay that way for licensing reasons.
- Move or replace a published tag, or edit a published release's assets.
