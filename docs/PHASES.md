# Phases P0–P13

Canonical product phases from PRD §52. There is no P14+.

Status is **runtime/product completeness**, not “does a package exist.” Packages can exist while a behavior remains PARTIAL.

| ID | Name | Status |
|---|---|---|
| P0 | Skeleton | IMPLEMENTED |
| P1 | PRD Parser | IMPLEMENTED |
| P2 | Graph Engine | IMPLEMENTED (execution walk IMPLEMENTED headless; no parallel) |
| P3 | Preflight | IMPLEMENTED (advisory) |
| P4 | Cursor Worker | IMPLEMENTED |
| P5 | Git/GitHub | IMPLEMENTED local; GitHub PARTIAL (disabled by default) |
| P6 | CI Integration | PARTIAL (observe checks; no webhook daemon) |
| P7 | Test Engine | IMPLEMENTED (`go test ./...` primary) |
| P8 | Review + Model Router | IMPLEMENTED interfaces; live LLM PARTIAL |
| P9 | Self-Fix | IMPLEMENTED bounded repair; upstream rewind PARTIAL |
| P10 | Human Interaction | IMPLEMENTED |
| P11 | Learning | PARTIAL (observation files; no promotion UI) |
| P12 | Subagent / Optimization | IMPLEMENTED as `NO_SUBAGENT` decision only |
| P13 | Self-Dogfooding | PARTIAL (fixture tests; not self-modification of this repo) |

Historical plan IDs (old P14–P16 in an earlier plan draft) are **retired**. See [IMPLEMENTATION_PLAN.md](../IMPLEMENTATION_PLAN.md).

## P0 — Skeleton

**Purpose.** CLI, version/help, doctor, `init`, `.project/` state, lock, jail.

**Status.** IMPLEMENTED.

**Components.** `cmd/prdpr`, `internal/cli`, `internal/state`, `internal/fsguard`, `internal/config`.

**Runtime.** Every command.

**Limits.** No daemon, no live TUI.

## P1 — PRD Parser

**Purpose.** Markdown → `PRDDocument`, IDs, diagnostics.

**Status.** IMPLEMENTED. Contract validation is an additional gate (`prd.ValidateContractFile`), not a separate phase ID.

**Components.** `internal/prd`.

**Runtime.** `inspect`, `validate-prd`, bootstrap, prepare.

**Limits.** LLM ambiguity pass is not the parser. Missing IDs are diagnostics, not silent invention.

## P2 — Graph Engine

**Purpose.** Phase DAG, Ready/Blocked, serialize, affected-set **calculation**.

**Status.** IMPLEMENTED library + persist + READY selection + headless `RunGraph`.

**Components.** `internal/graph`, `internal/engine/graphutil.go`, `loop.go`.

**Runtime.** prepare/phase/verify completion.

**Limits.** Sequential only. No task DAG. Rewind executor is PARTIAL.

## P3 — Preflight

**Purpose.** Readiness report.

**Status.** IMPLEMENTED.

**Components.** `internal/preflight`, `prdpr doctor` / `preflight`.

**Runtime.** Plugin and CLI before prepare; P4 still fail-closes on Git baseline.

**Limits.** Does not auto-install tools. Does not grant Cursor permissions.

## P4 — Cursor Worker

**Purpose.** Packet + `cursor-agent` subprocess, timeout, transcript, diff vs baseline.

**Status.** IMPLEMENTED. Distinct from the Cursor **plugin**.

**Components.** `internal/cursor`, `internal/plan`, `internal/packet`, `internal/proc`.

**Runtime.** `prdpr run` / `prdpr phase` only. Plugin implements in-session instead.

**Limits.** Default `go test` uses fakes. Live test is `PRDPR_CURSOR_LIVE=1`.

## P5 — Git/GitHub

**Purpose.** Commits, branches, push, optional `gh` repo/PR/rulesets.

**Status.** Local lifecycle IMPLEMENTED. GitHub IMPLEMENTED when enabled; default off.

**Components.** `internal/vcs`, `internal/engine/repo.go`, `delivery.go`.

**Runtime.** After verified phases; bootstrap.

**Limits.** No GitHub App. No force-push. Auto-merge default false.

## P6 — CI Integration

**Purpose.** Observe GitHub Actions.

**Status.** PARTIAL: `internal/ci` + `prdpr checks`. Polling, not webhooks.

**Runtime.** After a PR exists, if GitHub enabled.

**Limits.** Does not replace local P7. Does not auto-repair CI flakes as a separate loop.

## P7 — Test Engine

**Purpose.** Independent verification.

**Status.** IMPLEMENTED for Go fixtures (`go test ./...`). Other ecosystems PARTIAL/PLANNED.

**Components.** `internal/testeng`, `engine.Verify`.

**Runtime.** Every `prdpr verify`.

**Limits.** Worker output is not the grade.

## P8 — Review + Model Router

**Purpose.** Extra gates + vendor-neutral routing.

**Status.** IMPLEMENTED router + review. CLI uses `llm.None`; no paid call in default tests.

**Components.** `internal/review`, `internal/modelrouter`, `internal/llm`.

**Runtime.** `prdpr review` and inner `RunPhase`.

**Limits.** Adding a live provider requires an `llm.Adapter` implementation; none ships as a required default.

## P9 — Self-Fix

**Purpose.** Diagnose, repair ≤3, rewind/replay.

**Status.** IMPLEMENTED local repair loop. Upstream origin/rewind PARTIAL.

**Components.** `internal/diagnose`, `internal/repair`, `engine` repair/runtime repair.

**Runtime.** Failed verify inside `RunPhase` or plugin review/repair.

**Limits.** No fourth autonomous attempt.

## P10 — Human Interaction

**Purpose.** One question, credentials metadata, notify, feedback/resume.

**Status.** IMPLEMENTED.

**Components.** `internal/human`, `internal/notify`.

**Runtime.** Completeness (if LLM), placement, manual AC, repair exhaustion, GitHub, runtime fail.

**Limits.** No Keychain write of secret values in the default CLI path; credential **presence** is recorded.

## P11 — Learning

**Purpose.** Observations and cost ledger.

**Status.** PARTIAL: `.project/knowledge/` puts and `cost.jsonl`. No global policy promotion CLI.

**Components.** `internal/knowledge`, `internal/cost`.

**Runtime.** After verified complete (knowledge put); review may append cost.

**Limits.** No vector DB.

## P12 — Subagent / Optimization

**Purpose.** Record USE_SUBAGENT vs NO_SUBAGENT.

**Status.** IMPLEMENTED as advisory `subagent.Decide`. Scheduler always sequential.

**Limits.** Changing this requires an ADR. Do not spawn parallel workers on one tree.

## P13 — Self-Dogfooding

**Purpose.** Orchestrator can run the loop on a **fixture** product.

**Status.** PARTIAL: `internal/engine/dogfood_test.go` and fixture PRDs. Ordinary self-modification of `prd-pr` is refused. Explicit `SELF_DEVELOPMENT` is a dedicated opt-in path (`--self-development` + PRD declaration + orchestrator identity). `AllowSelf` remains tests-only.

**Limits.** Broad dogfood of this repo is out of scope for this documentation pass.
