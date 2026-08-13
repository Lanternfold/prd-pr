# IMPLEMENTATION PLAN: PRD→PR

**Status:** Draft v3 (canonical PRD §52 phase IDs)  
**Inputs:** `PRD.md`, `ARCHITECTURE.md`, `ADR/`  
**Canonical phase IDs:** PRD §52 **P0–P13**. This plan does not use a second numbering system.

This plan is the bridge from architecture to coding. It preserves PRD phase **identity and scope** while recording **delivery slices** and **runtime invariants** that do not get their own phase IDs.

**MVP goal:** prove **orchestration**, not model quality. Do not spend money on LLM calls before the deterministic loop works.

---

## Canonical phase identity

Product and engineering share one catalog. Names and IDs come from PRD §52:

| ID | Name | Status |
|---|---|---|
| P0 | Skeleton (CLI + state + init + doctor) | Done (two delivery slices) |
| P1 | PRD Parser | Done |
| P2 | Graph Engine | Done |
| P3 | Preflight | Not started (P4 uses fail-closed runtime gates instead of a preflight report) |
| P4 | Cursor Worker | Done |
| P5 | Git/GitHub | Not started |
| P6 | CI Integration | Not started |
| P7 | Test Engine | Not started |
| P8 | Review + Model Router | Not started |
| P9 | Self-Fix | Not started |
| P10 | Human Interaction | Not started |
| P11 | Learning | Not started |
| P12 | Subagent / Optimization | Not started (V1: `NO_SUBAGENT`, ADR-007) |
| P13 | Self-Dogfooding | Not started |

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
| Plan P6 Minimal Git | **P5** local subset | Baseline SHA before Cursor **writes** |
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

This is a **runtime** rule (ADR-002, ADR-010), not a second phase ID. PRD P4 (Cursor) does not list P5 as a *build* dependency of the adapter. The running engine still must not let Cursor write until a local Git baseline exists.

MVP does **not** need GitHub PR creation. Local Git (P5 local subset) is sufficient.

The orchestrator must be able to determine:

- current branch
- current commit
- clean / dirty working tree
- baseline commit (recorded before the worker runs)
- changes introduced by the worker (vs that baseline)

**Delivery:** implement P5 local inspect/baseline before enabling real Cursor writes in P4.

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

That slice is **First Dogfood Milestone**, after P0, P1, P2, P3, P4 (packet + worker), P5 (local Git), and P7 (local tests), plus a **minimal engine** stub. It does not require P5 GitHub, P6 CI, P8–P13.

**Default planning path:** Deterministic PRD phase → plan → task packet (P4). A Noop/Deterministic planner is sufficient. A live LLM adapter can be enabled later (P8).

**First dogfood target:** a **tiny separate Git repository**, not this repo. Example: `~/Studio/Experiments/prdpr-fixture/`. Do **not** self-modify `~/Studio/Tools/prd-pr/` for the first dogfood.

**Not in MVP:** GitHub PRs, GitHub Actions, live LLM calls, LLM review, diagnosis/repair/rewind, human notification workflow, knowledge promotion, cost budgets, subagent *execution*, design/Figma, self-hosting, self-modification of this tool.

---

## Shared conventions (all phases)

- Go module `github.com/lanternfold/prd-pr`, binary `prdpr`.
- Conceptual packages: `internal/engine` coordinates; other `internal/*` are capabilities. Exact folder names may change if they stay faithful to this split.
- No MCP, no database, no extra services (ADR-009).
- Tests: unit + fakes. No live Cursor/GitHub/LLM in default `go test`.
- Human involvement until P10 is **us building the tool**, not the product’s human-workflow feature.
- Do **not** introduce: database, MCP, agent framework, message queue, cloud worker, daemon, vector DB, multi-agent execution, or parallel workers.

**Structured IDs (known limitation):** Architecture expects `REQ-*` / `AC-*` / `TEST-*` on product PRDs when present. This repo’s `PRD.md` currently illustrates the format and does not yet carry a full ID inventory. Do not invent IDs. Parser and planner must tolerate missing IDs with diagnostics, not silent fabrication (OAQ-6).

---

## P0 — Skeleton (done)

**Objective.** Go CLI + durable project state: binary, version/help, `doctor`, `prdpr init`, `.project/state.json`, events, lock, product-root jail.

**Why.** Everything hangs off one binary (ADR-001). Resume/lock must be real before long runs (ADR-002).

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

**Why.** Fail closed before invoking Cursor on a broken machine.

**Dependencies.** P0, P1 (PRD §52).

**Files/packages.** `internal/preflight`; enrich `prdpr doctor`.

**Interfaces.** `Report` with tool versions (or missing), dependency class, blocking list. No secret values. Git missing is BLOCKING for `run`. GitHub/`gh` is OPTIONAL until P5 GitHub subset / P6.

**Tests.** Fake PATH; missing git is BLOCKING for run; optional tool missing does not block; missing `gh` does not block MVP run.

**Definition of Done.** `prdpr run` refuses to start when Git is missing or the tree policy is violated (define: refuse dirty tree unless configured).

**Defer.** Xcode/Docker deep checks, Keychain credential matrix, auto-install, requiring GitHub.

---

## P4 — Cursor worker (done)

**Objective.** Task packet + invoke Cursor CLI in a **checkpointed** workspace, timeout, capture transcript (redacted), list changed files vs baseline. Include a **deterministic planner** (Noop/Deterministic) that maps PRD phases → packet **without a live LLM**.

**Why.** Coding lane (ADR-004). Packet needs a plan; MVP must not require a paid model (ADR-008). Worker claim of success is not verification (ADR-005).

**Dependencies.** P0, P1 (PRD §52). Runtime still requires P3 preflight not blocking and P5 local baseline before writes.

**Files/packages.** `internal/plan`, `internal/packet`, `internal/cursor`, `internal/engine` stub, `internal/redact`, `internal/proc`, and P4-minimum `internal/vcs` (inspect/baseline/diff only).

**Interfaces.** `DeterministicPlanner` → `packet.json`; `Worker.Run` → `WorkerResult`; `claimed_done` ignored. Fail closed if baseline missing. Jail diffs inside product root (ADR-010). First dogfood workspace is the **fixture repo**.

**Tests.** Fake worker; timeout kills process group; paths outside jail fail; redaction; run refused without baseline; planner golden with no network.

**Definition of Done.** Fixture `prdpr run` (or `--mvp`) invokes worker or fake, writes `execution.json`, prints claim plus observed changes. Pin Cursor CLI version when the real adapter lands (OAQ-8).

**Defer.** Live LLM planner, multi-step sessions, repair loop, self-dogfood, pointing Cursor at this orchestrator repo.

---

## P5 — Git / GitHub

**Objective.** Local Git inspection and recoverable baseline; later, optional push and **one PR per run/milestone** (ADR-011). Never merge default unless configured.

**Why.** Git SHAs are the code checkpoint. GitHub is CI’s door and is **not** required to prove the orchestration loop.

**Dependencies.** P0 (PRD §52).

**Delivery slices (not extra IDs).**

1. **Local Git (before real Cursor writes):** branch, HEAD SHA, dirty/clean, baseline record, diff vs baseline. No `gh`, no PR.
2. **GitHub (after first dogfood):** `Push`, `OpenPR(milestone)`. Engine must still complete a local run with GitHub disabled.

**Files/packages.** `internal/vcs` (local subset first; `gh` later).

**Definition of Done (local).** Fixture repo: record baseline; after a file change, report paths vs baseline; `prdpr run` must not invoke the coding worker until this baseline exists.

**Definition of Done (GitHub).** Manual smoke can open one PR when `gh` is available. MVP dogfood does not require this.

**Defer.** Stacked PRs, auto-merge, per-phase PR mode.

---

## P6 — CI (GitHub Actions)

**Objective.** Watch GitHub Actions after a PR exists. Local tests remain the per-phase gate; CI is the milestone gate.

**Why.** Independent remote verification (ADR-005). CI must not block MVP.

**Dependencies.** P5.

**Files/packages.** `internal/ci`.

**Definition of Done.** After a milestone push, orchestrator records Actions status. Local test failure still fails the phase without extra PRs.

**Defer.** Deep artifact mining, non-Actions CI. Not part of first dogfood.

---

## P7 — Test engine

**Objective.** Orchestrator runs tests **itself** in the product/fixture workspace, parses pass/fail, records the result. MVP verification gate.

**Why.** Independent verification (ADR-005). The worker is never the grader.

**Dependencies.** P0, P1 (PRD §52). Runtime sequence still: packet → git baseline → worker → independent diff → independent tests.

**Files/packages.** `internal/testeng` (local runner only).

**Definition of Done.** After a fixture worker change, `prdpr` runs tests in the fixture and records pass/fail in `.project/` without GitHub Actions.

**Defer.** Actions watch (P6), iOS/`xcodebuild`, adversarial LLM test generation.

---

## P8 — Review + Model Router

**Objective.** Independent review + applicable quality gates. Model Router + LLM **interfaces** (Noop default). Deterministic checks first; live LLM review later.

**Why.** Worker must not grade itself (ADR-005). Router must not hardcode vendors (ADR-008). First dogfood already has independent tests; this adds extra gates.

**Dependencies.** P0, P1 (PRD §52).

**Files/packages.** `internal/review`, `internal/modelrouter`, `internal/llm` (interfaces + Noop).

**Definition of Done.** Enabled gates can fail a phase without LLM. Default `go test` passes with Noop. Live provider is documented, not required.

**Defer.** Paid review models, visual screenshot review, until the deterministic loop is proven.

---

## P9 — Self-Fix

**Objective.** Classify product vs infrastructure; per-incident 3-attempt repair; rewind via Git SHA; replay affected-set (ADR-006, ADR-003).

**Why.** Self-fixing loop. After first dogfood on purpose.

**Dependencies.** P2, P4, P7 (PRD §52). P8 and P6 are useful evidence, not build blockers.

**Files/packages.** `internal/diagnose`, `internal/repair`; engine `recovery`.

**Definition of Done.** Deliberately broken fixture: diagnose, repair or rewind, replay descendants, or block with a report. No silent fourth attempt.

**Defer.** Live LLM diagnosis until after first dogfood.

---

## P10 — Human interaction

**Objective.** One question at a time, credential request (Keychain), validation task, 30s single attention notify, `feedback` / `resume`.

**Why.** Blocking input must survive process exit. Not required for first dogfood.

**Dependencies.** P0 (PRD §52).

**Files/packages.** `internal/human`, `internal/notify`.

**Definition of Done.** Forced clarification stops the run; `prdpr feedback` + `resume` continues. Secrets not in JSON.

---

## P11 — Learning

**Objective.** Phase learning review → observations; promotion pipeline; cost ledger + budget pause.

**Why.** Learning and cost are PRD goals but not required to move code.

**Dependencies.** P0 (PRD §52). Richer after execution history exists.

**Files/packages.** `internal/knowledge`, `internal/cost`.

**Definition of Done.** After a phase, files exist under `.project/knowledge/` and a cost line can appear on `status`. No vector DB (ADR-009).

---

## P12 — Subagent / Optimization

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
  → P3 Preflight
  → P5 local Git baseline
  → P4 packet + Cursor worker
  → P7 independent tests
        ★ First Dogfood / MVP Orchestrator
          (separate fixture; no GitHub, no live LLM, no self-modification)

Then:

  → P5 GitHub subset
  → P6 CI
  → P8 Review + router interfaces
  → P9 Self-Fix
  → P10 Human
  → P11 Learning
  → P12 Subagent flag only (NO_SUBAGENT)
  → engine wiring (no new ID)
  → P13 Self-dogfood
```

P3 can overlap with P5 local Git. P7 can overlap with P4 adapter work. **Do not start GitHub/CI/review/repair/human/learning/self-dogfood until the MVP slice has been used on the fixture repository.**

---

## First Dogfood Milestone

**Name:** MVP Orchestrator  

**When:** P0, P1, P2, P3, P4, P5 local Git, and P7 complete, with a minimal engine stub.

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
4. A Git baseline is recorded (branch, commit, clean/dirty, baseline SHA).
5. Cursor is invoked (manual smoke) against the **fixture**.
6. Cursor modifies the fixture repository.
7. PRD→PR independently inspects the diff vs baseline.
8. PRD→PR independently runs tests in the fixture.
9. PRD→PR determines pass/fail.
10. The resulting state is persisted under the fixture’s `.project/`.
11. The process can resume after interruption.

The worker’s own claim of success must **not** count as verification.

**First dogfood must not require:** GitHub, GitHub Actions, a live LLM, self-modification of PRD→PR, a repair loop, the human notification system.

**Success metric:** one trivial change in the fixture repo is orchestrated end-to-end as above.

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
