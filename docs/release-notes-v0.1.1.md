# GitCasebook v0.1.1

A patch release. It changes what GitCasebook shows you, not what it concludes.

Classification logic, canonical `case.json` state, the evidence schema, and
`--json` output are byte-for-byte unchanged from v0.1.0. Case fingerprints
produced by the two versions agree.

## Why this release exists

v0.1.0 was validated against a synthetic fixture corpus. The first runs against
real external repositories exposed three problems that only appear outside a
test harness, all of them in how results are presented.

## Fixed

**The report now shows the object evidence behind each classification.**
Relationship tables listed only commit counts. Two sources holding identical
commits but differing tag objects rendered as `SUBSET` with zero unique commits
on either side, which reads as a contradiction even though the classification is
correct. Tables now carry A-only and B-only object counts and the reason code,
and object-only containment is called out explicitly.

**The command prints its result.** A completed investigation reported only that
the case was ready for review, leaving the finding in `report.md`. `investigate`
and `compare` now print one line per source pair, with the classification and
the evidence behind it, to standard output.

**Acquisition reports progress.** Preserving a source produced no output until
it finished. Because forks on GitHub share object storage, mirroring a
repository inside a large fork network can transfer far more than the project's
own history, with nothing on screen to say so. Each source now reports when
acquisition starts and how large the preserved mirror is, and an unusually large
mirror is explained.

Progress goes to standard error, so piping standard output is unaffected. Under
`--json`, standard error stays silent and standard output is unchanged.

## Upgrading

Nothing to migrate. Cases produced by v0.1.0 remain valid and readable, and
regenerating a case with v0.1.1 from the same preserved sources produces the
same canonical state.

## Distribution

The artifact set is five archives and a `SHA256SUMS` file:

- Linux amd64 and arm64;
- Windows amd64;
- macOS amd64 and arm64.

The versioned source installation command is:

```text
go install github.com/itxcrusher/git-casebook/cmd/git-casebook@v0.1.1
```

## Safety boundary

Unchanged. GitCasebook does not checkout or execute repository application code.
It does not run hooks, package managers, tests, migrations, submodule
initialization, LFS smudge filters, history rewrites, pushes, or publication
operations. Acquisition may use the network; deterministic analysis is offline
by design.

GitCasebook is not an operating-system sandbox. Review
[SECURITY.md](https://github.com/itxcrusher/git-casebook/blob/v0.1.1/SECURITY.md)
before analyzing deliberately hostile sources.

## Compatibility

The binary is pre-1.0 and the evidence schema is independently versioned. v0.1.1
writes case schema `1.0.0`, the same as v0.1.0. Review
[versioning and compatibility](https://github.com/itxcrusher/git-casebook/blob/v0.1.1/docs/versioning.md)
before integrating machine output.
