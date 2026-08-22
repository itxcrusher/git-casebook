## Change

Describe the narrow behavior or evidence change.

## Verification

- [ ] `gofmt` reports no changes
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go build ./cmd/repo-rehab`
- [ ] New Git behavior has a synthetic fixture or invariant test
- [ ] No repository-controlled code executes in Levels 0-2
- [ ] No private or uncleared fixture/source material was added
- [ ] New dependencies and licenses are documented
- [ ] Every commit includes a DCO `Signed-off-by` trailer

## Evidence compatibility

State whether the JSON schema, semantic fingerprint, command UX, or safety gates
change. Link the relevant issue for contract changes.
