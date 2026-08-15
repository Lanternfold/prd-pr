# IMPLEMENTATION PLAN: PRD→PR

**Status:** Draft v3 (canonical PRD §52 phase IDs)  
**Inputs:** `PRD.md`, `ARCHITECTURE.md`, `ADR/`  
**Canonical phase IDs:** PRD §52 **P0–P13**. This plan does not use a second numbering system.

This plan is the bridge from architecture to coding. It preserves PRD phase **identity and scope** while recording **delivery slices** and **runtime invariants** that do not get their own phase IDs.

Layers (ADR-012): **core engine** (Go) · **CLI interface** · **Cursor plugin interface** (primary UX; Plugin V0 in `prdpr-cursor/`) · **worker adapters** (P4). Do not collapse plugin and worker.

**MVP goal:** prove **orchestration**, not model quality. Do not spend money on LLM calls before the deterministic loop works.

---

## Canonical phase identity

Product and engineering share one catalog. Names and IDs come from PRD §52:

| ID | Name | Status |
|---|---|---|
| P0 | Skeleton (CLI + state + init + doctor) | PASS (two delivery slices) |
| P1 | PRD Parser | PASS |
| P2 | Graph Engine | PASS |
| P3 | Preflight | PASS |
| P4 | Cursor Worker | PASS |
| P5 | Git/GitHub | PASS (local lifecycle + optional gh; GitHub not required for local dogfood) |
| P6 | CI Integration | PASS (orchestrator Actions workflow + optional watch) |
| P7 | Test Engine | PASS |
| P8 | Review + Model Router | PASS |
| P9 | Self-Fix | PASS |
| P10 | Human Interaction | PASS |
| P11 | Learning | PASS |
| P12 | Subagent / Optimization | PASS (V1: `NO_SUBAGENT`, ADR-007) |
| P13 | Self-Dogfooding | PASS (disposable fixture loop; not self-modification of this repo) |

Build-order edges are the **explicit Dependencies** in PRD §52. The DAG engine must not infer edges from numbering or document order.

### Historical mapping (retired IDs)

Draft v2 of this plan used a different numbering to split delivery. Those IDs are **retired**. Use only the table below when reading older notes:

| Retired plan ID | Canonical PRD ID | What was preserved |
|---|---|---|
| Plan P0 Foundation | **P0** slice A (CLI) | CLI, version, help, doctor |
| Plan P1 State | **P0** slice B (persistence) | `.project/`, lock, events, jail |
| Plan P2 Parser | **P1** | Deterministic PRD parse |
| Plan P3 DAG | **P2** | Graph engine |
| Plan P4 Preflight | **P3** | Environment inspection |
| Plan P5 Planner + router interfaces | **P4** packet/plan + **P8** router | No live LLM; Noop/Deterministic default (ADR-008) |
| Plan P6 Minimal Git | **P4** pre-write Git safety + **P5** broader Git | P4 inspect/baseline/diff before writes; P5 commits/branches/push/GitHub |
| Plan P7 Cursor | **P4** | Worker after a recoverable checkpoint |
| Plan P8 Independent test | **P7** | Orchestrator grades; worker does not (ADR-005) |
| Plan P9 GitHub | **P5** GitHub subset | After first dogfood; PRs not required for MVP |
| Plan P10 CI | **P6** | After GitHub; not an MVP gate |
| Plan P11 Review | **P8** review work | Deterministic gates first |
| Plan P12 Diagnosis | **P9** | After MVP; 3-attempt rule (ADR-006) |
| Plan P13 Human | **P10** | Not required for first dogfood |
| Plan P14 Learning/cost | **P11** | After there is execution to learn from |
| Plan P15 Full engine | **no new ID** | Engine wiring before P13 |
| Plan P16 Self-dogfood | **P13** | Forbidden as first dogfood |

Retired numbering is not used in new work.

---

## Invariant: Git checkpoint before real Cursor execution

**NO REAL AUTONOMOUS CODE MODIFICATION WITHOUT A RECOVERABLE GIT CHECKPOINT.**

This is a **runtime** rule (ADR-002, ADR-010), not a second phase ID.

**P4 requires and enforces a local Git baseline before real Cursor writes.** That minimum local Git safety is part of the Cursor worker, not a P5 delivery gate:

- verify Git repository
- verify product root
- inspect working tree
- establish baseline SHA
- capture baseline
- inspect changes after worker execution

**P5** owns the broader Git lifecycle (commits, branches, pushes, GitHub, PRs, remote state, milestone PR behavior). Do not treat P4 writes as blocked on P5. Do not move the P4 baseline capability back into P5.

MVP does **not** need GitHub PR creation. P5 GitHub integration is not required for the first local dogfood.

The orchestrator (P4 safety, then later P5 lifecycle) must be able to determine:

- current branch
- current commit
- clean / dirty working tree
- baseline commit (recorded before the worker runs)
- changes introduced by the worker (vs that baseline)

---

## MVP Orchestrator milestone

**Do not build the full autonomous system before using it.**

The first useful product is a **thin vertical slice**:

```text
PRD.md
  → parse
  → deterministic plan (PRD phases → sequential tasks)
  → task packet
  → record Git baseline (branch, SHA, dirty/clean)
  → invoke Cursor
  → independently inspect diff vs baseline
  → independently run tests
  → determine pass/fail (do not trust the worker)
  → persist state
  → be resumable after interruption
```

That slice is **First Dogfood Milestone**, after P0, P1, P2, P3 (readiness reporting), P4 (worker **and** local Git baseline), plus a **minimal engine** stub. Independent verification (P7) is later in the loop. It does not require P5 (local Git lifecycle or GitHub), P6 CI, or P8–P13. P5 GitHub integration is **not** required for the first local dogfood.

**Default planning path:** Deterministic PRD phase → plan → task packet (P4). A Noop/Deterministic planner is sufficient. A live LLM adapter can be enabled later (P8).

**First dogfood target:** a **tiny separate Git repository**, not this repo. Example: `~/Studio/Experiments/prdpr-fixture/`. Do **not** self-modify `~/Studio/Tools/prd-pr/` for the first dogfood.

**Not in MVP:** GitHub PRs, GitHub Actions, live LLM calls, LLM review, diagnosis/repair/rewind, human notification workflow, knowledge promotion, cost budgets, subagent *execution*, design/Figma, self-hosting, self-modification of this tool.

---

## Shared conventions (all phases)

- Go module `github.com/lanternfold/prd-pr`, binary `prdpr`.
- Conceptual packages: `internal/engine` coordinates; other `internal/*` are capabilities. Exact folder names may change if they stay faithful to this split.
- No MCP, no database, no extra services (ADR-009). Cursor plugin is a UX adapter, not MCP (ADR-012).
- Tests: unit + fakes. No live Cursor/GitHub/LLM in default `go test`. The core engine must not require Cursor IDE.
- Human involvement until P10 is **us building the tool**, not the product’s human-workflow feature.
- Do **not** introduce: database, MCP, agent framework, message queue, cloud worker, daemon, vector DB, multi-agent execution, or parallel workers.
- Do **not** collapse the Cursor plugin (UX) with the Cursor worker (P4). Plugin V0 lives in `prdpr-cursor/` and must not launch `cursor-agent`.

**Structured IDs (known limitation):** Architecture expects `REQ-*` / `AC-*` / `TEST-*` on product PRDs when present. This repo’s `PRD.md` currently illustrates the format and does not yet carry a full ID inventory. Do not invent IDs. Parser and planner must tolerate missing IDs with diagnostics, not silent fabrication (OAQ-6).

---

## P0 — Skeleton (done)

**Objective.** Go CLI + durable project state: binary, version/help, `doctor`, `prdpr init`, `.project/state.json`, events, lock, product-root jail.

**Why.** The core engine hangs off one binary (ADR-001). Resume/lock must be real before long runs (ADR-002). The Cursor plugin is not P0 (ADR-012).

**Dependencies.** None.

**Delivery slices (already completed, not extra IDs).** (A) CLI foundation. (B) persistence. Slice B followed A so resume/lock existed before parse/run.

**Files/packages.** `cmd/prdpr`, `internal/cli`, `internal/config`, `internal/state`, `internal/fsguard`.

**Definition of Done.** `go build` produces `prdpr`; `doctor` reports OS/arch/git; `init` persists and reloads; second `init` is safe; concurrent locks fail; crash-before-rename keeps prior state.

**Defer.** Live status UI, full phase enum, cost ledger, knowledge files.

---

## P1 — PRD parser (done)

**Objective.** `prdpr inspect PRD.md` → structured `PRDDocument` + ID report.

**Why.** Nothing to orchestrate without requirements and phases.

**Dependencies.** P0.

**Files/packages.** `internal/prd`, `testdata/prd/`.

**Interfaces.** `Parse(markdown) → PRDDocument`; preserve existing `REQ-*`/`AC-*`/`TEST-*`; duplicate IDs are errors; do not rewrite the user’s PRD.

**Definition of Done.** Inspect prints section/REQ/AC/phase counts; `--json` dump; missing optional sections are warnings; this repo’s `PRD.md` parses.

**Defer.** Ambiguity LLM pass, filling this PRD with a complete REQ inventory.

---

## P2 — Graph engine (done)

**Objective.** Parsed phases → validated DAG: topo order, cycles, ready/blocked, affected-set, rewind/replay *calculation*.

**Why.** Rewind/replay and sequential execution need the graph (ADR-003, ADR-007).

**Dependencies.** P1.

**Files/packages.** `internal/graph`. Persistence of `graph.json` into `.project/` may wait until engine wiring; serialization API exists.

**Interfaces.** Edges from **explicit** phase Dependencies only. `Ready()`, `Independent()`, `Affected(id)`, `SequentialOrder()`, cycle error with path.

**Definition of Done.** This repo’s PRD graph has declared edges; cycles detected; independent branches skipped on rewind; `inspect --graph` prints the DAG.

**Defer.** Replay executor, live graph mutation during planning.

---

## P3 — Preflight

**Objective.** Inspect environment: OS, git, Cursor CLI presence, dirty tree, required CLIs from PRD deps (AVAILABLE/MISSING/OPTIONAL/BLOCKING). `gh` may be reported but must **not** block MVP.

**Why.** Explicit readiness report before a run: OS, git, Cursor CLI, dirty tree, required CLIs. Fail-closed enforcement immediately before Cursor execution remains P4.

**Dependencies.** P0, P1 (PRD §52).

**Files/packages.** `internal/preflight`; enrich `prdpr doctor`.

**Interfaces.** `Report` with tool versions (or missing), dependency class, blocking list. No secret values. Git missing is BLOCKING on the report. GitHub/`gh` is OPTIONAL until P5 GitHub subset / P6.

**Relationship to P4.** P3 is **advisory/readiness orchestration**: it produces an explicit readiness report. P4 is **authoritative runtime safety enforcement**: it still performs fail-closed checks immediately before execution. P3 being unavailable must **never** cause P4 to bypass its own safety checks (including the local Git baseline).

**Tests.** Fake PATH; missing git is BLOCKING on the report; optional tool missing does not block; missing `gh` does not block MVP run.

**Definition of Done.** `prdpr` emits an explicit preflight/readiness report (OS, git, Cursor CLI, dirty tree, required CLIs) with AVAILABLE/MISSING/OPTIONAL/BLOCKING. Git missing or tree-policy violation is BLOCKING on that report (define: dirty tree unless configured). Engine start may consult the report; Cursor writes remain gated by P4 regardless.

**Defer.** Xcode/Docker deep checks, Keychain credential matrix, auto-install, requiring GitHub.

---

## P4 — Cursor worker (done)

**Objective.** Task packet + invoke Cursor CLI in a **checkpointed** workspace, timeout, capture transcript (redacted), list changed files vs baseline. Include a **deterministic planner** (Noop/Deterministic) that maps PRD phases → packet **without a live LLM**.

**Why.** Coding lane (ADR-004). Packet needs a plan; MVP must not require a paid model (ADR-008). Worker claim of success is not verification (ADR-005).

**Dependencies.** P0, P1 (PRD §52). **P4 requires and enforces a local Git baseline before real Cursor writes.** P3 readiness is advisory and must not be treated as a substitute for those checks; P3 unavailable must not bypass P4. P5 is **not** a write-gate for the worker.

**Files/packages.** `internal/plan`, `internal/packet`, `internal/cursor`, `internal/engine` stub, `internal/redact`, `internal/proc`, and P4-minimum `internal/vcs` (inspect/baseline/diff only).

**Interfaces.** `DeterministicPlanner` → `packet.json`; `Worker.Run` → `WorkerResult`; `claimed_done` ignored. Fail closed if baseline missing. Jail diffs inside product root (ADR-010). First dogfood workspace is the **fixture repo**.

**Tests.** Fake worker; timeout kills process group; paths outside jail fail; redaction; run refused without baseline; planner golden with no network.

**Definition of Done.** Fixture `prdpr run` (or `--mvp`) invokes worker or fake, writes `execution.json`, prints claim plus observed changes. Pin Cursor CLI version when the real adapter lands (OAQ-8).

**Defer.** Live LLM planner, multi-step sessions, repair loop, self-dogfood, pointing Cursor at this orchestrator repo. **Do not redesign P4 into a Cursor plugin.** Plugin UX is a separate interface milestone (ADR-012).

---

## P5 — Git / GitHub

**Status:** PASS.

**Objective.** Broader Git lifecycle and GitHub integration: commits, branches, pushes, remote state, and **one PR per run/milestone** (ADR-011). Never merge default unless configured.

The **minimum** local Git safety required before Cursor writes (verify repo/product root, inspect working tree, establish/capture baseline SHA, inspect changes after the worker) is already **P4**. Do not re-own or re-gate that capability here.

**Why.** Commits, branches, remotes, and GitHub are the product Git lifecycle. GitHub is CI’s door and is **not** required to prove the orchestration loop or the first local dogfood. P4 already owns the pre-write checkpoint.

**Dependencies.** P0 (PRD §52).

**Delivery slices (not extra IDs).**

1. **Local Git lifecycle:** commits, branches, push-to-local-remote as needed. No `gh`, no PR. Does **not** include re-implementing P4’s pre-write inspect/baseline invariant.
2. **GitHub (after first dogfood):** `Push`, `OpenPR(milestone)`, remote state, milestone PR behavior. Engine must still complete a local run with GitHub disabled.

**Files/packages.** `internal/vcs` extended beyond P4’s inspect/baseline/diff; `gh` later.

**Definition of Done (local).** Fixture repo: create branch, commit worker results, report local history. Pre-write baseline remains P4’s invariant.

**Definition of Done (GitHub).** Manual smoke can open one PR when `gh` is available. First local dogfood does not require this.

**Defer.** Stacked PRs, auto-merge, per-phase PR mode.

---

## P6 — CI (GitHub Actions)

**Status:** PASS.

**Objective.** Watch GitHub Actions after a PR exists. Local tests remain the per-phase gate; CI is the milestone gate.

**Why.** Independent remote verification (ADR-005). CI must not block MVP.

**Dependencies.** P5.

**Files/packages.** `internal/ci`.

**Definition of Done.** After a milestone push, orchestrator records Actions status. Local test failure still fails the phase without extra PRs.

**Defer.** Deep artifact mining, non-Actions CI. Not part of first dogfood.

---

## P7 — Test engine

**Status:** PASS.

**Objective.** Orchestrator runs tests **itself** in the product/fixture workspace, parses pass/fail, records the result. MVP verification gate.

**Why.** Independent verification (ADR-005). The worker is never the grader. Cursor's claim of completion is never verification. Only P7 may set `verified_success = true`.

**Dependencies.** P0, P1 (PRD §52). Runtime sequence still: packet → **P4** git baseline → worker (or interactive session) → independent diff → independent tests. The baseline step is P4, not P5.

**Files/packages.** `internal/testeng` (local runner only); `prdpr verify`; plugin calls verify after implementation.

**Definition of Done.** After a fixture change, `prdpr verify` runs tests in the fixture and records pass/fail in `.project/` without GitHub Actions. `worker_claimed_success` alone never verifies. Interactive `/prdpr` stops on failed verify (no P8/P9).

**Defer.** Actions watch (P6), iOS/`xcodebuild`, adversarial LLM test generation, diagnosis/repair (P8/P9).

---

## P8 — Review + Model Router

**Status:** PASS.

**Objective.** Independent review + applicable quality gates. Model Router + LLM **interfaces** (Noop default). Deterministic checks first; live LLM review later.

**Why.** Worker must not grade itself (ADR-005). Router must not hardcode vendors (ADR-008). Independent tests (P7) grade the worker; this phase adds extra gates.

**Dependencies.** P0, P1 (PRD §52).

**Files/packages.** `internal/review`, `internal/modelrouter`, `internal/llm` (interfaces + Noop).

**Definition of Done.** Enabled gates can fail a phase without LLM. Default `go test` passes with Noop. Live provider is documented, not required.

**Defer.** Paid review models, visual screenshot review, until the deterministic loop is proven.

---

## P9 — Self-Fix

**Status:** PASS.

**Objective.** Classify product vs infrastructure; per-incident 3-attempt repair; rewind via Git SHA; replay affected-set (ADR-006, ADR-003).

**Why.** Self-fixing loop. After first dogfood on purpose.

**Dependencies.** P2, P4, P7 (PRD §52). P8 and P6 are useful evidence, not build blockers.

**Files/packages.** `internal/diagnose`, `internal/repair`; engine `recovery`.

**Definition of Done.** Deliberately broken fixture: diagnose, repair or rewind, replay descendants, or block with a report. No silent fourth attempt.

**Defer.** Live LLM diagnosis until after first dogfood.

---

## P10 — Human interaction

**Status:** PASS.

**Objective.** One question at a time, credential request (Keychain), validation task, 30s single attention notify, `feedback` / `resume`.

**Why.** Blocking input must survive process exit. Not required for first dogfood.

**Dependencies.** P0 (PRD §52).

**Files/packages.** `internal/human`, `internal/notify`.

**Definition of Done.** Forced clarification stops the run; `prdpr feedback` + `resume` continues. Secrets not in JSON.

---

## P11 — Learning

**Status:** PASS.

**Objective.** Phase learning review → observations; promotion pipeline; cost ledger + budget pause.

**Why.** Learning and cost are PRD goals but not required to move code.

**Dependencies.** P0 (PRD §52). Richer after execution history exists.

**Files/packages.** `internal/knowledge`, `internal/cost`.

**Definition of Done.** After a phase, files exist under `.project/knowledge/` and a cost line can appear on `status`. No vector DB (ADR-009).

---

## P12 — Subagent / Optimization

**Status:** PASS (decision interface only; V1 ignores USE_SUBAGENT).

**Objective.** Record `USE_SUBAGENT` / `NO_SUBAGENT`. V1 scheduler **ignores** `USE_SUBAGENT` and runs sequentially (ADR-007).

**Why.** PRD includes this phase. Architecture forbids parallel Cursor on one tree. Do not build a swarm.

**Dependencies.** P2.

**Definition of Done.** Graph still exposes parallel candidates; scheduler executes one node; default decision is `NO_SUBAGENT`.

**Defer.** Isolated worktrees and real parallel workers until measured need.

---

## Engine wiring (no new phase ID)

**Objective.** Replace the MVP stub with the full sequential engine: schedule, all steps, milestone PR, recovery, human pause, learning hook.

**Why.** Capabilities can exist before they are one reliable loop. Do not give this a competing ID. It is required **before P13**.

**Files/packages.** `internal/engine` (`run`, `scheduler`, `executor`, `recovery`); live status (PRD §47).

**Definition of Done.** `prdpr run PRD.md` on a small **product fixture** completes or blocks with structured state; `status`/`logs`/`pause`/`resume` work. Deterministic path still works with Noop LLM.

---

## P13 — Self-dogfooding

**Status:** PASS (disposable product fixture; orchestrator self-repo remains jail-refused).

**Objective.** Point PRD→PR at `~/Studio/Tools/prd-pr/` with a **small** improvement PRD. Build to `dist/`; never overwrite the running binary (ADR-010).

**Why.** PRD ultimate loop. Unsafe until engine wiring is boring. **Forbidden as the first dogfood.**

**Dependencies.** P1, P2, P3, P4, P5, P7, P8, P9 (PRD §52), plus engine wiring and a completed first dogfood on a **separate** fixture.

**Definition of Done.** One real improvement goes parse → plan → Cursor → test → commit → milestone PR → human merge.

---

## Recommended delivery order

PRD §52 allows parallelism after P0. Humans building this repo still work sequentially. Recommended order (does not add IDs):

```text
P0 Skeleton (done: CLI then state)
  → P1 Parser (done)
  → P2 Graph (done)
  → P3 Preflight (readiness report)
  → P4 packet + Cursor worker
       (enforces local Git baseline before real writes)
  → P7 independent tests (later in the loop)
        ★ First Dogfood / MVP Orchestrator
          (separate fixture; P3 + P4 worker + P4 Git baseline;
           independent verification later; no P5 GitHub, no live LLM,
           no self-modification)

Then:

  → P5 Git/GitHub (broader lifecycle, then GitHub)
  → P6 CI
  → P8 Review + router interfaces
  → P9 Self-Fix
  → P10 Human
  → P11 Learning
  → P12 Subagent flag only (NO_SUBAGENT)
  → engine wiring (no new ID)
  → P13 Self-dogfood
```

P3 can overlap with P4 worker work. P4 must still enforce its own fail-closed checks if P3 is missing. P7 can overlap with remaining P4 polish. **Do not start GitHub/CI/review/repair/human/learning/self-dogfood until the MVP slice has been used on the fixture repository.** Do **not** implement P5 local inspect/baseline as a prerequisite for P4 writes; that capability is already P4.

**Cursor Plugin V0** lives in `prdpr-cursor/`. It is an interface milestone over the engine, not a new phase and not a P4 redesign. Do not start P5 from the plugin.

---

## First Dogfood Milestone

**Name:** MVP Orchestrator  

**When:** P0, P1, P2, P3 (readiness reporting), and P4 (worker **and** local Git baseline) complete, with a minimal engine stub. Independent verification (P7) is later in the loop. P5 GitHub integration is **not** required. P5 local Git lifecycle is **not** a prerequisite for P4 writes.

**Where:** a **real, tiny, separate Git repository**. Example: `~/Studio/Experiments/prdpr-fixture/`.

Do **not** use `~/Studio/Tools/prd-pr/` as the first dogfood workspace.

**Fixture contents (example):**

- an existing function
- an existing test
- a small PRD that asks to add another small function (and a test)

**Workflow that must actually run:**

1. PRD is parsed.
2. A **deterministic** plan is generated (no live LLM).
3. A task packet is generated.
4. P4 records a Git baseline (branch, commit, clean/dirty, baseline SHA) before real Cursor writes.
5. Cursor is invoked (manual smoke) against the **fixture**.
6. Cursor modifies the fixture repository.
7. PRD→PR independently inspects the diff vs baseline.
8. PRD→PR independently runs tests in the fixture.
9. PRD→PR determines pass/fail.
10. The resulting state is persisted under the fixture’s `.project/`.
11. The process can resume after interruption.

The worker’s own claim of success must **not** count as verification.

**First dogfood requires:** P3 readiness reporting; P4 worker; P4 local Git baseline; independent verification later (P7).

**First dogfood must not require:** P5 GitHub integration, GitHub Actions, a live LLM, self-modification of PRD→PR, a repair loop, the human notification system, or the Cursor plugin. Do not wait for P5 local Git lifecycle before P4. First dogfood uses the engine CLI (and worker), not Plugin V0.

**Success metric:** one trivial change in the fixture repo is orchestrated end-to-end as above.

---

## Interface milestone: Cursor Plugin V0

**Not a canonical phase.** Do not assign P14 or rename P0–P13. Plugin V0 is a **delivery slice / interface milestone** over the existing engine (ADR-012).

**When:** after the Go engine is independently executable for the workflow the plugin will invoke. Current P0–P4 already provide CLI, parse, graph, preflight, state, planning, and the Cursor **worker**. This milestone does **not** start P5 and does **not** implement new engine functionality.

**Location:** `prdpr-cursor/` (Cursor Plugin format: `.cursor-plugin/plugin.json`, `commands/prdpr.md`, `skills/prdpr/SKILL.md`).

**V0 invocation:** existing `prdpr inspect`, `preflight`, `prepare`, and `status` with an explicit workspace directory. Plugin V0 does **not** call `prdpr run` (that would nest the P4 Cursor worker inside this Cursor session). After `prepare`, the current Cursor session implements the packet. Independent verification is not in this milestone.

**Plugin V0 contains only:**

1. plugin manifest
2. one `/prdpr` command
3. one PRD→PR skill
4. minimal documentation

**Plugin V0 must:**

- invoke the PRD→PR Go engine
- pass workspace/project context
- present orchestration state/results
- guide Cursor through the workflow
- remain a thin adapter (no duplicated orchestration)

**Plugin V0 must not:**

- maintain orchestration state
- implement the DAG, repair, retry policy, Git semantics, verification, knowledge, or model routing
- collapse into the P4 worker adapter
- add MCP, subagents, hooks, custom agents, SDK, cloud services, databases, or extra commands unless strictly necessary

**Deferred Cursor capabilities (document only):** hooks, additional commands, Cursor Agent SDK adapter, optional subagents, MCP if a concrete need emerges.

---

## Deferred Complexity

Explicitly **not** in the initial build (MVP and first several phases):

- Live LLM planning, review, diagnosis, repair, or cost optimization  
- GitHub PRs and GitHub Actions (until after first dogfood)  
- Repair / rewind / replay loop as a dogfood requirement  
- Human notification / clarification workflow as a dogfood requirement  
- Self-modification of this repository  
- Multi-agent / parallel Cursor (ADR-007)  
- Worktree isolation for parallelism  
- MCP, databases, queues, daemons, cloud orchestrator, cloud workers  
- Agent frameworks  
- Cursor plugin features beyond V0 (hooks, extra commands, SDK adapter, optional subagents)  
- Collapsing the Cursor plugin with the P4 worker  
- Vector DBs  
- Per-phase GitHub PRs; auto-merge  
- Vendor SDKs in `engine`  
- Persistent planner/reviewer/diagnoser agents  
- Design/Figma pipeline  
- Full quality-gate matrix (a11y, perf, adversarial)  
- Knowledge promotion to global policy without a human  
- Cost-perfect CI billing  
- Human-minutes forecasting quality  
- `prdpr run --phase` (OAQ-15)  
- Self-modification of the running binary  
- Benchmark suite from PRD §53 as a gate (track later as fixtures)

Build those only when the MVP loop is real on the fixture and a listed ADR’s “Revisit When” is met.
