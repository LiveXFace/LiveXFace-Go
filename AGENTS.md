# Agent Instructions for LiveXFace Go SDK

## Workflow and Scope

Use Spec Kit for new feature work, not OpenSpec. Read
`.specify/memory/constitution.md` and `docs/SPEC_KIT_WORKFLOW.md` first.
Respect applicable ancestor instructions. Start the agent from this repository
to load its `speckit-*` skills; Codex uses `$speckit-<step>`, Claude uses
`/speckit-<step>`. Do not invoke `/opsx:*` or modify historical OpenSpec archives.

Own the public SDK API, transport, typed responses/errors, and pinned contract
fixtures. The backend owns public HTTP behavior; do not implement server business
rules in the SDK. Preserve supported runtimes and safe retry/idempotency behavior.

Do not create feature artifacts in a sibling repo without identifying its
ownership and dependencies. For cross-repo work, link the common feature key,
owning specs, contract changes, and PR/release order. Preserve unrelated edits.
Do not infer publication, deployment, merge, or issue authority from an earlier task.

## Discovery and Evidence

Prefer codebase-memory MCP for code discovery when available. Confirm the project
and generation, inspect relevant symbols/traces, and check index coverage for
evidence paths. Read source for recorded gaps; if MCP is unavailable, use bounded
source/config inspection and disclose limitations. Graph absence is not proof
that code does not exist. Parallel task markers do not authorize spawning agents.

## Quality Gates

Run `go vet ./...`, verify `gofmt -l .` is empty, and run `go test -race ./...`.
Current CI covers Go 1.22 and stable; local execution on one version is not proof
that the full version matrix passed.

Changed behavior requires regression tests in specs/tasks. Keep existing CI gates;
document exact results and blocked/skipped checks. Documentation-only changes
require artifact/path validation, not a claim that application tests ran.
Never log or commit secrets, API keys, tokens, or biometric fixtures.
