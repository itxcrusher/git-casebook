# Architecture

## Boundary

v0.1 is one Go command-line application backed by a supported native Git
executable. The binary owns policy, evidence, comparison, ref planning, and
reporting. Git remains the reference implementation for repository semantics.

```text
CLI
  -> case/policy store
  -> controlled Git runner
  -> bare preserved mirrors
  -> ref/object/integrity inventory
  -> set-based relationship engine
  -> collision-safe ref planner
  -> canonical case.json
  -> generated report.md
```

## Packages

| Package | Responsibility |
| --- | --- |
| `cmd/git-casebook` | Argument parsing, exit codes, human/JSON output |
| `internal/app` | Resumable Level 0-2 workflow orchestration |
| `internal/casefile` | YAML/JSON/JSONL persistence, schema and semantic validation |
| `internal/gitexec` | Direct native-Git invocation, config isolation, command classes, limits |
| `internal/preserve` | Local/remote bare mirror acquisition and push disablement |
| `internal/inventory` | Refs, `fsck`, reachability, all objects, shallow/partial, submodule/LFS state |
| `internal/compare` | Exact deterministic set classifications and proof artifacts |
| `internal/refplan` | Reversible source namespaces, Git ref validation, collision withholding |
| `internal/evidence` | SHA-256 sidecars, artifact verification, filesystem fingerprints |
| `internal/report` | Markdown derived from canonical state |
| `schema` | Embedded and distributable case schema |

The entry point uses the standard library `flag` package. A CLI framework would
add a dependency without improving this command surface.

## Native Git subprocess contract

Every Git call uses `exec.CommandContext` with an argument vector, never a shell
string. The runner:

- resolves the Git executable once;
- assigns each command to `PROBE`, `ACQUISITION`, or `OFFLINE_ANALYSIS`;
- rejects commands outside each class before process creation;
- captures bounded stdout/stderr;
- applies command and acquisition timeouts;
- closes stdin unless controlled object IDs are supplied;
- gives Git an isolated home/global config and empty hooks/templates;
- excludes inherited `GIT_*`, SSH-agent, and credential variables;
- disables prompts, helpers, optional locks, LFS smudge, and replace-object
  traversal; and
- records whether any invoked command was network-capable.

`clone` is the only network-capable command in the acquisition allowlist. Offline
analysis cannot invoke clone, fetch, pull, push, `ls-remote`, submodule, checkout,
reset, or package/application commands.

## Relationship model

For each source, sorted manifests contain commits and objects reachable from all
included refs. Replacement refs are inventoried but not traversal roots. Ref
names do not decide equality.

For complete, integrity-verified, non-empty sources with compatible object
formats:

- `EXACT`: commit and object sets equal;
- `SUPERSET`: B's commit and object sets are properly contained in A;
- `SUBSET`: A's commit and object sets are properly contained in B;
- `DIVERGED`: at least one shared commit, without complete containment;
- `DISJOINT`: no shared reachable commit;
- `UNKNOWN`: any deterministic precondition is unavailable or untrusted.

The CLI emits both directions and validates inverse/symmetry invariants.

## Portability

Case references use relative paths and forward slashes. Absolute local paths and
wall-clock fields are excluded from semantic fingerprints. Content identity,
tool/Git versions, policy semantics, relationships, gates, and ref plans remain
inside the semantic projection.

The same code and fixture corpus run on GitHub-hosted Windows, Linux, and macOS
runners. Linux CI additionally checks an OS-network-namespace run and symlink
confinement.
