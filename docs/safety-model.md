# Safety and Trust Boundaries

## Capability levels

| Level | Capability | v0.1 |
| ---: | --- | --- |
| 0 | Define case and policy | Implemented |
| 1 | Controlled source acquisition | Implemented |
| 2 | Offline deterministic provenance | Implemented |
| 3 | General static content inspection | Deferred |
| 4 | Dependency preparation | Deferred |
| 5 | Contained inherited execution | Deferred |
| 6 | Mutation/re-baselining | Deferred |
| 7 | Push/publication | Deferred |

The executable contains no command for Levels 3-7. Source execution, dependency
installation, history rewrite, push, publication, deletion, and license-dependent
reuse gates are serialized closed.

## Trust boundaries

Repository refs, objects, filenames, commit messages, hooks, configuration,
attributes, filters, submodule declarations, LFS pointers, scripts, package
metadata, and generated reports are untrusted data. They cannot authorize a new
command, gate, path, credential, network endpoint, or scope.

| Threat | v0.1 response | Residual limitation |
| --- | --- | --- |
| Git hooks/templates | Bare mirror, no checkout, explicit empty hook/template directories | Installed Git vulnerabilities remain possible |
| Credential helpers/prompts | Inherited Git environment removed; empty helper and prompt disabled | Authenticated private remotes need a future explicit design |
| Submodule URLs | `.gitmodules` and gitlinks are inventoried; no recurse/fetch | Submodule content remains unknown |
| Filters, textconv, external diff | No checkout/diff; LFS pointer inspection reads raw Git blobs only | Git object parsing is still trusted |
| Git LFS | Smudge disabled; pointer and local payload state separated | Remote LFS availability is not proven |
| Replace refs | Inventoried; `GIT_NO_REPLACE_OBJECTS=1`; excluded as traversal roots | Human review must interpret replacement intent |
| Grafts/alternates | Detected and marked incomplete | Unusual external stores are unsupported |
| Crafted/corrupt objects | `git fsck`, missing-object errors, `UNKNOWN`; no repair | No OS sandbox in v0.1 |
| Unsafe paths/symlinks | No checkout; case confinement and symlink checks; encoded refs | Native Git still parses object and ref bytes |
| Huge inputs | Command/output time and size limits; bounded small-blob LFS scan | Filesystem fingerprinting and Git traversal can still be expensive |
| Package/application scripts | Never invoked | Later execution needs separate containment |
| Docker socket/host mounts | Not used | Not a runtime reconstruction tool |
| Network callbacks | Only clone is allowed during acquisition; offline command allowlist afterward | Command audit is not packet capture on Windows |

## Git environment isolation

The runner keeps required process variables such as `PATH`, temporary-directory
locations, system root, locale, and TLS certificate paths. It removes inherited
Git/SSH-agent/cloud-token surfaces from the Git child environment and sets:

- controlled `HOME`, `USERPROFILE`, and `XDG_CONFIG_HOME`;
- `GIT_CONFIG_NOSYSTEM=1` and an empty controlled global config;
- `GIT_TERMINAL_PROMPT=0` and `GCM_INTERACTIVE=Never`;
- `GIT_NO_REPLACE_OBJECTS=1`, `GIT_OPTIONAL_LOCKS=0`, and
  `GIT_LFS_SKIP_SMUDGE=1`;
- empty hooks/template paths and credential helper;
- protocol default deny with only file, HTTPS, and SSH acquisition transports
  enabled, and `ext` explicitly denied.

Analysis commands are allowlisted separately from acquisition. Git is invoked
with `exec.CommandContext`, bounded output, and explicit timeouts. The timeout
kills the direct Git process; complete descendant resource containment remains an
operating-system concern and is not claimed.

## Preservation invariant

Local input repositories are content-fingerprinted before and after acquisition.
Preserved bare mirrors are content-fingerprinted before and after analysis and
again during verification. Derived evidence is written only beneath
`artifacts/`, never inside a mirror.

Fixture tests prove harmless hooks, package scripts, executable files,
`.gitmodules`, filters, and textconv commands do not create a sentinel. Linux CI
also runs a local-source investigation in a network namespace with no external
interfaces.

## Fail closed

Shallow, partial/promisor, corrupt, empty, grafted, alternate-backed, unsupported,
or missing-LFS states remain visible. When relationship preconditions fail, the
successful deterministic result is `UNKNOWN` with reason codes. The tool does not
repair, deepen, fetch, infer, or convert uncertainty into a likely answer.
