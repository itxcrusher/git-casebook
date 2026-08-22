# Command Reference

## Guided workflow

```text
repo-rehab investigate <source> [<source> ...] --case <directory>
```

This creates or resumes a case, declares sources, makes controlled mirrors,
switches to offline analysis, inventories and compares all sources, plans refs,
validates the case, and generates `report.md`.

Existing local Git directories and `https`, `ssh`, or `file` Git locators are
accepted. Password-bearing URLs, query strings, fragments, and `ext` transports
are rejected. Credential helpers and prompts are disabled; authenticated remote
acquisition therefore requires a future explicit credential design.

## Lifecycle commands

| Command | Level | Effect |
| --- | ---: | --- |
| `case init` | 0 | Create case policy, control directories, and append-only streams |
| `source add` | 0 | Add a declared source to policy |
| `preserve` | 1 | Clone missing sources as bare mirrors and disable their push URL |
| `inspect` | 2 | Verify/inventory preserved refs and objects offline |
| `compare` | 2 | Create directional relationship facts and set-difference sidecars |
| `refs plan` | 2 | Propose and validate mappings without applying them |
| `verify` | 2 | Recheck schema, semantics, artifacts, mirrors, refs, and fail-closed state |
| `report` | 2 | Derive Markdown from `case.json` |

`--case <directory>` may appear anywhere. `source add` command-specific flags
must appear before its locator. Use `--help` for the current surface.

## Output and exit codes

- `0`: command completed. A verified case may still be `INCOMPLETE` with
  `UNKNOWN` relationships; inspect `status` and `reasons`.
- `1`: operation or verification infrastructure failed.
- `2`: invalid command usage.

`--json` returns one compact JSON result on stdout. `--json-errors` returns an
error object with `code`, `message`, and `exit_code` on stderr. Progress and errors
do not echo accepted secret-bearing URL forms because those forms are rejected.

## Idempotence

Re-running acquisition reuses an existing valid preserved mirror and does not
fetch it. Analysis rewrites only derived canonical state and reuses
content-addressed artifacts. Each operation appends an audit event.

The CLI never refreshes a preserved source automatically. A changed upstream is a
new evidence event/source, not an implicit mutation of the forensic case.
