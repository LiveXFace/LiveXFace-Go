# LiveXFace Go SDK Constitution

## Core Principles

### I. Keep the SDK a Faithful Public API Consumer

The SDK MUST expose the backend's public recognition contract rather than implement
server-side authorization, inference, billing, or persistence rules. HTTP methods,
paths, JSON/multipart naming, required fields, and response types MUST follow the
pinned contract and explicit backend changes. Existing public methods MUST remain
compatible unless a reviewed breaking-change release and migration explicitly replace them.

### II. Preserve Safe Transport and Retry Semantics

Changes MUST preserve configured timeouts, request cancellation/client injection
where supported, authentication headers, idempotency keys, and bounded retry behavior.
Retries MUST NOT turn non-idempotent or single-use operations into duplicate actions.
Retry-After handling and distinctions between transport and API failures MUST remain
covered by regression tests. Do not copy another SDK's retry policy without checking
this SDK's existing contract and behavior.

### III. Preserve Typed Errors and Partial Results

Expose existing status, API error code, request correlation, and retry details through
the SDK's established types. Handle empty and non-JSON error responses safely.
Search partial results and incompatible-profile errors MUST NOT be silently converted
into complete success. Additive optional server fields MUST NOT cause clients to fail
when an older supported server omits them. SDKs MUST NOT compare stored vectors locally.

### IV. Protect Credentials and Biometric Data

API keys, tokens, face images, and embedding vectors MUST NOT be logged, embedded in
examples as real values, or committed in fixtures. Validate client-side inputs without
claiming that validation replaces backend authorization. Tests MUST use mocked transport,
synthetic fixtures, or explicitly authorized service/data environments.

### V. Verify Contract and Runtime Compatibility

Specifications and tasks for behavior changes MUST include regression tests and the
existing contract checks. Contract fixture updates MUST be intentional, traceable to a
backend release or reviewed backend contract, and accompanied by version constant/file
updates and tests. Backend breaking changes MUST coordinate a new major release of
affected SDKs with migration documentation. No package tag or registry publication
is implied by a code merge or a completed Spec Kit task list.

## Engineering Constraints

Use Go 1.22 or newer as declared in `go.mod`. Preserve the module import path,
exported Go API, caller contexts, and existing injected HTTP client behavior.

The repository's `CONTRACT_VERSION`, exposed version constant, and committed
`contract/` fixture define its validated baseline; do not silently replace them with
the newest backend contract. Reuse existing test conventions and keep the supported
runtime/dependency matrix explicit. This SDK is an independent Git repository.

## Development Workflow and Quality Gates

Use repository-local Spec Kit artifacts for new features: specify, clarify uncertain
behavior, plan, tasks, analyze, implement, and converge. Read this constitution during
planning and analysis. Changed behavior requires tests even if generic scaffolding says
tests are optional. Historical OpenSpec artifacts are reference only; do not invent
retroactive specifications for completed releases.

Run `go vet ./...`, verify `gofmt -l .` is empty, and run `go test -race ./...`.
Current CI covers Go 1.22 and stable; local execution on one version is not proof
that the full version matrix passed.

Preserve existing CI gates, record exact verification results and blockers, and complete
applicable checks before merge. Cross-repository tasks MUST link the backend contract and
dependent PRs. Publication, deployment, issue changes, and external/destructive operations
require authorization in the current task.

## Governance

This initial constitution is adopted on 2026-10-05 for the Spec Kit migration.
Amendments MUST be reviewed with rationale and impact on public SDK compatibility,
active feature specifications, tests, and supported runtime versions. Conflicts MUST
be surfaced to the maintainer instead of silently overriding existing constraints.

Use MAJOR for incompatible principle changes, MINOR for new or materially expanded
rules, and PATCH for clarifications. Reviews MUST check compliance and record approved
exceptions. Completed feature directories remain historical; current contracts,
examples, and release documentation MUST be maintained separately.

**Version**: 1.0.0 | **Ratified**: 2026-10-05 | **Last Amended**: 2026-10-05
