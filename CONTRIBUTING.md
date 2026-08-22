# Contributing

GitCasebook accepts focused changes that strengthen the deterministic
preserve/compare/prove/plan contract.

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

Use Go 1.27.0 and a native Git executable, then run:

```text
gofmt -w .
go vet ./...
go test ./...
go build ./cmd/git-casebook
```

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
- Identify new dependencies and their licenses.
- Confirm that no repository-controlled content executes in Levels 0-2.

The project currently has one maintainer. A separate governance document is
intentionally deferred until real contributor roles require one.
