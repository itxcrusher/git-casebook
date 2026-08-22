# Dependencies and Licensing

Date reviewed: 2026-08-22

The v0.1 core uses the Go standard library, an installed native Git executable,
and two direct Go modules. Two additional modules are present in the resolved
build graph through the JSON Schema validator.

| Dependency | Version | Purpose | License evidence |
| --- | --- | --- | --- |
| `github.com/santhosh-tekuri/jsonschema/v6` | `v6.0.3` | Compile and validate the embedded Draft 2020-12 case schema | Apache-2.0 `LICENSE` in module source |
| `go.yaml.in/yaml/v3` | `v3.0.5` | Parse and emit the human-authored policy | Mixed MIT/Apache-2.0 `LICENSE`; Canonical notice retained in root `NOTICE` |
| `github.com/dlclark/regexp2` | `v1.11.0` | Transitive regular-expression support for JSON Schema validation | MIT `LICENSE` in module source |
| `golang.org/x/text` | `v0.14.0` | Transitive Unicode/text support for JSON Schema validation | BSD-style `LICENSE` in module source |
| Native Git | User-installed, exact version recorded per case | Reference implementation for mirror, refs, objects, `fsck`, reachability, and ref validation | External executable; not redistributed by this repository |

Module versions and checksums are pinned by `go.mod` and `go.sum`. No module is
used to execute or interpret inherited application code.

New dependencies require:

1. a narrow need not reasonably met by the standard library;
2. an active, identifiable upstream;
3. a license compatible with Apache-2.0 distribution;
4. committed version/checksum evidence;
5. retained attribution or notice where required; and
6. a security/maintenance review proportional to its role.

The applicable attribution and license texts are retained in
`THIRD_PARTY_NOTICES.md`. Release preparation must reproduce this review from the
resolved module graph and include `LICENSE`, `NOTICE`, and
`THIRD_PARTY_NOTICES.md` in distributed archives.
