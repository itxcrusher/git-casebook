# GitCasebook v0.1.0

These notes define the contents of GitCasebook v0.1.0, the first pre-1.0
release.

## What is included

- Bare-mirror preservation for local Git repositories and declared Git remotes.
- All-ref ref, object, reachability, integrity, shallow, partial, submodule, and
  Git LFS pointer-state inventories.
- Deterministic `EXACT`, `SUBSET`, `SUPERSET`, `DIVERGED`, `DISJOINT`, and
  fail-closed `UNKNOWN` relationship classifications.
- Collision-safe, source-namespaced archival ref plans that are never applied by
  the tool.
- Versioned JSON evidence, append-only JSONL events/findings, content-addressed
  sidecars, and a derived Markdown report.
- A guided `investigate` command plus resumable Level 0-2 lifecycle commands.
- Synthetic F01-F15 coverage and Windows, Linux, and macOS CI.

## Safety boundary

GitCasebook does not checkout or execute repository application code. It does
not run hooks, package managers, tests, migrations, submodule initialization,
LFS smudge filters, history rewrites, pushes, or publication operations.
Acquisition may use the network; deterministic analysis is offline by design.

GitCasebook is not an operating-system sandbox. Review `SECURITY.md` before
analyzing deliberately hostile sources.

## Distribution

The v0.1.0 artifact set consists of five archives and a `SHA256SUMS` file:

- Linux amd64 and arm64;
- Windows amd64;
- macOS amd64 and arm64.

The versioned source installation command is:

```text
go install github.com/itxcrusher/git-casebook/cmd/git-casebook@v0.1.0
```

## Compatibility

The binary is pre-1.0 and the evidence schema is independently versioned. Review
[versioning and compatibility](https://github.com/itxcrusher/git-casebook/blob/v0.1.0/docs/versioning.md)
before integrating machine output.
