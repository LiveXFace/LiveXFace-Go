# Spec Kit Development Workflow

## Scope and Baseline

This repository uses Spec Kit for new feature work. Read `AGENTS.md` and
[`constitution.md`](../.specify/memory/constitution.md) before planning.
OpenSpec documents from the aggregate workspace are historical references only.
Do not run `/opsx:*`, reactivate legacy skills, or regenerate completed changes
such as `versioned-embeddings` as if they were new implementation tasks.

This is an independent Git/Spec Kit project. Start the coding agent from this
repository so its skills are discovered; restart an existing workspace-root
session if its skill catalog still shows the old workflow. Never initialize Git
or `.specify/` at the aggregate workspace root.

## Installed Integrations and Invocation

The adoption baseline uses Specify CLI **1.0.13**, Bash scripts, and both Codex
and Claude skills. The existing default integration remains Claude.
Invoke process steps in agent chat, not the terminal:

- Codex: `$speckit-constitution`, `$speckit-specify`, `$speckit-clarify`,
  `$speckit-plan`, `$speckit-tasks`, `$speckit-analyze`,
  `$speckit-implement`, `$speckit-converge`.
- Claude: corresponding `/speckit-<step>` skills.
- Other agents require their own integration; do not assume Codex or Claude
  skills register commands for OpenCode or another client.

Use the constitution skill for governance only. For a bounded feature, specify
the outcome and compatibility boundaries, clarify uncertainty, plan against
the existing repository, generate tasks with regression/verification work,
analyze consistency, implement, and converge. Repeat implementation/convergence
when remaining tasks are found. Convergence never substitutes for actual tests.
A documentation-only adoption does not need an artificial feature specification.

## Artifacts and Cross-Repository Work

Keep feature artifacts under `specs/<feature-directory>/` with their specification,
plan, tasks, and generated design/contract documents. Commit accepted artifacts
with the associated implementation. Completed feature directories are historical
records; make later amendments explicit and keep current contracts/docs separate.
There is no OpenSpec archive/sync command in the adopted workflow.

For a multi-repo feature, use a common descriptive key and cross-link owning
specifications/PRs. Record dependencies and release order in the plans.
The backend owns published OpenAPI; the engine owns inference/profile behavior;
the console owns UX; each SDK owns its typed consumer and transport API.
Create artifacts only in repositories actually affected by the feature.

## Sharing, Updating, and Verification

Track `.specify/` shared scripts/templates/workflow/constitution and the selected
agent skill directories. Do not track secrets, agent local settings, generated
test/build output, `.specify/feature.json`, or extension local overrides.
The provided `.specify/.gitignore` excludes machine-local Spec Kit state.
Check `specify version` and `specify integration list` before refreshing.
Do not rerun `specify init --here --force` over customized paths without reviewing
their baseline and the expected changes. CLI upgrades are separate reviewed work.

Use ordinary Git branches and PR review; automatic branching is not assumed.
Do not install extensions, publish packages, mutate issues, deploy, or merge
without authorization for the current task. Existing CI remains authoritative.
Behavior changes require tests even when the generic task scaffold calls them
optional. Report exact commands/results and every blocked or skipped check.
The repository constitution and `AGENTS.md` describe its actual quality gates.

Official references:
[existing projects](https://github.github.io/spec-kit/guides/existing-projects.html),
[agent integrations](https://github.github.io/spec-kit/reference/integrations.html),
[SDD workflow](https://github.github.io/spec-kit/reference/agentic-sdd.html).
