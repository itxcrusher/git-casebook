# Security Policy

## Supported versions

GitCasebook is pre-1.0. Supported versions are identified in
[GitHub Releases](https://github.com/itxcrusher/git-casebook/releases); security
fixes also land on the current `main` branch before the next release.

## Reporting a vulnerability

Use a private GitHub security advisory when that feature is available to you:

<https://github.com/itxcrusher/git-casebook/security/advisories/new>

Do not place credentials, private repository contents, crafted private objects,
or sensitive case evidence in a public issue. If the private advisory route is
unavailable, contact the repository owner through an already established private
channel and include only the minimum reproduction material.

Please include the affected command, operating system, Git version, expected
boundary, observed behavior, and a synthetic reproduction where possible.

## Input trust model

All repository inputs are untrusted. v0.1 does not checkout or execute repository
application code. Acquisition may use the network for an explicitly declared
remote. After acquisition, provenance analysis is designed to be offline.

The CLI isolates system/global Git configuration where practical, disables
credential prompting/helpers and replacement traversal, uses empty hook/template
directories, invokes Git without a shell, bounds command output/time, and has no
push or history-rewrite function.

These controls are not an operating-system sandbox. Native Git still parses
untrusted objects, and vulnerabilities in Git or the host remain possible. Use a
separate OS account, container, or VM for sources that may be deliberately
malicious. Never expose the host home directory, SSH agent, cloud credentials, or
Docker socket to such an environment.

Generated cases may reveal repository locators, ref names, object IDs, authorship
metadata, or other sensitive provenance. Review evidence before sharing it.
