# ARCHITECTURE: PRD→PR

**Status:** Draft v1  
**Product:** PRD→PR  
**Companion to:** `PRD.md` (Draft v1)  
**Constraint:** Local-first single Go binary. No SaaS, no cloud orchestrator, no MCP, no database, no message queue.

This document describes how PRD→PR will be built. It does not change product requirements. Accepted review decisions are under [Resolved Architectural Decisions](#resolved-architectural-decisions). Remaining underspecification is under [Open Architectural Questions](#open-architectural-questions).

---

## 1. Architectural principles

1. **Deterministic before probabilistic.** Git, DAGs, schemas, tests, CI, and the filesystem are the source of truth. LLMs propose; the orchestrator verifies.
2. **One process, one machine.** V1 is a single Go CLI on the user’s Mac. Integrations are subprocesses and HTTP APIs (`git`, `gh`, Cursor CLI, LLM APIs), not services.
3. **State is structured.** Resume, rewind, and learning never depend on parsing natural-language logs.
4. **Verify independently.** An implementation worker reporting “done” is not completion. Files, Git, tests, CI, and acceptance criteria are.
5. **Repair at the origin.** Diagnose, then repair, rewind, or re-plan. Do not blindly retry.
6. **Human attention is a scarce resource.** Ask one precise question, only when necessary, with a forecast of when it will happen.
7. **Minimum reasonable complexity.** Do not add agents, MCP, databases, or cloud infrastructure without a demonstrated need.
8. **The orchestrator must be able to operate on itself** without special-casing the architecture—only safety guards around the running binary and workspace.
9. **Orchestrator, not swarm.** PRD→PR is an engineering orchestrator that uses AI. It is not an agent swarm that happens to write code. Planner, reviewer, and diagnoser are **task roles**, not persistent agents.

---

## Resolved Architectural Decisions

These five decisions were accepted in architecture review. They override earlier defaults in this document and close the related open questions. Full write-ups live in `ADR/`.

### RAD-1. Three-attempt rule (ADR-006)

Autonomous repair is limited to **3 attempts per failure incident / repair chain**, not per project and not globally. Unrelated failures each get their own counter. Attempt *n+1* must use evidence from attempt *n*. After the third unsuccessful attempt: **STOP** → human debugging report → `BLOCKED` / human intervention. A human may explicitly reset or reopen an incident. The system must not silently start a fourth autonomous attempt.

### RAD-2. Infrastructure failures (ADR-006)

Outages and interruptions **do not consume repair attempts**: GitHub/Actions unavailable, LLM API unavailable, network failure, rate limiting, temporary Cursor service failure, machine sleep. Use retry/backoff/recovery. They are not automatically product/code failures.

### RAD-3. Sequential V1 parallelism (ADR-007)

The DAG still records dependencies, independent nodes, potential parallelism, and affected descendants. The **V1 scheduler executes one phase/task at a time**. No multi-agent parallel execution. Future parallelism must use isolated Git worktrees/branches, never two workers on one working tree. Subagent routing may exist as a decision; V1 defaults to **NO_SUBAGENT**.

### RAD-4. PR granularity (ADR-011)

V1 default is **phase → commit → verify → continue**, with a GitHub PR at a **run / meaningful milestone** boundary—not a PR per phase. Granularity must be configurable later without redesigning the engine. **Do not merge into the default branch** unless explicitly configured.

### RAD-5. Model architecture (ADR-008)

```text
Model Router → LLM Adapter → configured model/provider
```

The orchestrator may use different models for planning, architecture, review, diagnosis, test generation, learning, and other reasoning. The router picks the cheapest capable model from task, complexity, risk, context, budget, and historical performance. **Cursor is the coding worker**, not the general LLM runtime. Do not hardcode vendors in the core. Do not spawn a persistent agent per role.

---

## 2. System overview

PRD→PR is an **engineering loop engine**. It is not an agent framework.

```text
Human
  │  PRD, decisions, credentials, validation, feedback
  ▼
prdpr (Go CLI)
  │
  ├─ parse / plan / route / diagnose / learn     (orchestrator reasoning)
  ├─ Cursor CLI                                  (coding worker)
  ├─ local FS + Git                              (code + checkpoints)
  ├─ GitHub + Actions                            (PRs, CI, artifacts)
  ├─ tests / linters / builds                    (deterministic verification)
  └─ macOS Keychain + notifications              (secrets, human attention)
```

**What lives where**

| Location | Role |
|---|---|
| `~/Studio/Tools/prd-pr/` | Orchestrator source and binary |
| `~/Studio/Products/<product>/` | Product workspace (default) |
| `<product>/.project/` | Orchestrator metadata for that product |
| GitHub repo | Remote source of truth for branches, PRs, CI, artifacts |
| macOS Keychain | Secrets at rest on the Mac |
| GitHub Actions Secrets | Secrets in CI |

The orchestrator never owns the product’s Git history. Git is the code checkpoint. `.project/` is the orchestration checkpoint.

---

## 3. Component architecture

V1 is one binary with internal packages. There are no microservices and no in-process “agent runtime.”

**Engine vs capabilities.** The orchestration **engine** coordinates a run: schedule the next node, invoke a capability, persist results, recover. Other modules **provide capabilities** (parse, plan, code, test, review, diagnose). They do not drive the loop.

Exact Go package layout is an implementation choice. The conceptual split:

```text
cmd/prdpr                 entrypoint
internal/
  engine/                 coordinates execution (not a capability)
    run / scheduler / executor / recovery
  cli/                    commands, live status, signals
  config/                 user + project configuration
  fsguard/                workspace jail
  state/                  persistence, locking, project/phase machines
  graph/                  DAG, affected-set, replay plan
  prd/                    PRD parse, IDs, traceability index
  preflight/              environment and dependency inspection
  plan/                   technical plan, design plan, convergence
  packet/                 task packet schema
  modelrouter/            task → model choice (no vendor in core)
  llm/                    LLM adapters (configured providers)
  cursor/                 coding worker adapter
  vcs/                    git + gh
  ci/                     Actions status, logs, artifacts
  testeng/                discover, run, parse, map to REQ/AC
  review/                 independent review + quality gates
  diagnose/               failure classification
  repair/                 repair / rewind / replay + 3-attempt rule
  human/                  questions, validation tasks, pause/resume
  notify/                 attention notifications
  knowledge/              observations → lessons → policies
  cost/                   estimates, actuals, budget pause
  subagent/               USE_SUBAGENT / NO_SUBAGENT (V1: always NO)
  report/                 PROJECT_REPORT.md
  redact/                 secret scrubbing in logs
```

`engine` calls capabilities; capabilities never call `engine`. Scheduler in V1 is sequential (RAD-3).

Adapters (Cursor, Git, GitHub, CI, Keychain, notifier, LLM client) sit behind small interfaces so tests do not require the network or Cursor.

**Decision: single Go process with adapters**

- **Needed:** PRD requires a personal Mac tool with Git, GitHub Actions, and Cursor CLI.
- **Simpler alternative:** a shell script wrapping Cursor. Rejected: no durable state machine, no independent review, no rewind/replay.
- **Preferable:** one binary can persist state, jail the workspace, and treat Cursor as a worker rather than the system.
- **Revisit if:** a second runtime (e.g. a required GUI) cannot share this process. Do not split into a daemon until `prdpr run` cannot stay in the foreground plus resume from disk.

**Decision: no database in V1**

- **Needed:** durable, resumable state.
- **Simpler alternative:** SQLite. Rejected until JSON/JSONL + atomic rename is insufficient.
- **Preferable:** files in Git-friendly `.project/` match “filesystem for initial knowledge storage” and keep the product repo self-contained.
- **Revisit if:** concurrent writers, large event history, or query patterns make files painful. That is not the V1 shape.

**Decision: no MCP in V1**

- Matches PRD §50. Direct `git`/`gh`/Cursor/LLM HTTP. Revisit when an integration cannot be called directly.

---

## 4. Module responsibilities

| Module | Owns | Does not own |
|---|---|---|
| `engine` | Run loop, sequential schedule, step execution, recovery orchestration | Domain logic of parse/plan/code/test |
| `cli` | UX, flags, live status, interrupt → pause | Business rules |
| `state` | `state.json`, locks, valid transitions | How a phase is implemented |
| `graph` | Nodes, edges, parallel candidates, affected-set | Whether a node *should* exist; when to run it |
| `prd` | Structured PRD model + traceability | Execution |
| `preflight` | Readiness report, dependency classification | Installing arbitrary system software without policy |
| `plan` | Phase graph population, design vs tech plans | Editing product code |
| `modelrouter` | Task/role → model choice + recorded outcomes | Provider SDKs; Cursor invocation |
| `llm` | Provider adapters behind one interface | Routing policy |
| `cursor` | Invoke coding worker, timeout, capture | Trusting “done”; general reasoning tasks |
| `vcs` | Branch, commit, push, PR at milestone | CI interpretation |
| `ci` | Watch checks, fetch logs/artifacts | Repair strategy |
| `testeng` | Independent test execution and mapping | Writing all tests (worker may add tests; engine verifies) |
| `review` | Independent review + quality gate | Implementation |
| `diagnose` | Origin, confidence, recommended action | Applying the fix |
| `repair` | Per-incident attempt counter, rewind/replay | Counting infrastructure outages as attempts |
| `human` | Question/validation contracts, resume tokens | Notification transport |
| `knowledge` | Promotion pipeline, search | Silent global policy mutation |
| `cost` | Ledger, budget threshold | Payment |

---

## 5. Execution lifecycle

A **run** is one orchestration of a product workspace against a PRD. The **engine** owns this loop and calls capability modules as steps. Capabilities do not drive scheduling.

```text
prdpr run PRD.md
  → acquire project lock
  → load or create .project/state
  → PREFLIGHT
  → ANALYZE (parse PRD, extract IDs, detect blocking ambiguity)
  → PLAN (design ∥ technical → converge → DAG)
  → forecast human involvement and cost
  → for each ready phase in topological / parallel-safe order:
        REVIEW_CONTEXT
        ROUTE (model, tools, subagent yes/no)
        DETAILED_PLAN / task packet
        IMPLEMENT (Cursor, constrained to workspace)
        GIT checkpoint (commit on the run branch)
        TEST (local)
        CI (if this run has reached a PR/CI milestone)
        REVIEW + quality gate
        on fail: DIAGNOSE → REPAIR | REWIND | REPLAN
                 (max 3 per incident; infrastructure outages do not count)
        HUMAN_VALIDATION if the phase requires it
        LEARN
        checkpoint metadata
  → PR at run / configured milestone (not per phase)
  → PROJECT_COMPLETE + PROJECT_REPORT.md
```

Human blocking can interrupt any step: clarification, missing blocking dependency, credential, budget, or failed 3-attempt repair. The process may exit; disk state is enough to `prdpr resume`.

**Decision: crash-safe loop, not an in-memory workflow engine**

- **Needed:** resume after terminal close, reboot, network/model/CI failure (PRD §9).
- **Simpler alternative:** keep everything in RAM and tell the user to restart. Rejected by the PRD.
- **Preferable:** each step writes a checkpoint before and after side effects where possible (see persistence).
- **Revisit if:** steps become so fine-grained that file writes dominate; then batch checkpoints per step, not per file write.

### 5.1 Reconciling “plan then graph” vs “graph then plan”

The PRD core loop orders Design/Technical Planning **before** Phase Graph. The high-level diagram orders Phase Graph **before** the two planners. See [OAQ-1](#oaq-1-order-of-phase-graph-vs-designtechnical-planning).

**Chosen interpretation (until the PRD is clarified):**

1. **Analyze** produces a *coarse* DAG from explicit PRD phases plus implied work (design, foundation, implementation, validation).
2. **Design planning and technical planning** run as graph nodes and may run in parallel.
3. A **convergence** node must complete before any implementation node that depends on both.
4. Planners may **refine** the DAG (split, add, retarget edges). Refinement is a recorded graph mutation, not an implicit rewrite.

This preserves parallel design/tech work and “converge before dependent implementation” (PRD §23) without a second planning subsystem.

---

## 6. State machine

Two machines. The PRD lists a single flat list that mixes project lifecycle with per-phase work ([OAQ-2](#oaq-2-flat-vs-nested-states)).

### 6.1 Project run states

```text
PROJECT_CREATED
  → PREFLIGHT
  → ANALYZING
  → PLANNING
  → READY                 # forecast done, DAG exists, execution can start
  → RUNNING               # at least one phase in flight (composite)
  → WAITING_FOR_INPUT     # clarification or credential
  → HUMAN_VALIDATION      # real-world validation task outstanding
  → BLOCKED               # cannot proceed (blocking dep, budget, 3-attempt stop)
  → PROJECT_COMPLETE
  → FAILED                # unrecoverable; not used for “need a human”
```

`RUNNING` is a composite: the project is running if the current phase is in an execution state. The PRD does not name `RUNNING`; it is the parent of IMPLEMENTING…LEARNING. If we must stay literally on the PRD enum, persist `current_state` as the **phase** state while the project is executing, and keep `project_status` as created/ready/blocked/complete. See OAQ-2.

**Not persisted as first-class project states** (they are phase states or transitions):

- `NEXT_PHASE` — a transition after learning/checkpoint, not a stable state ([OAQ-3](#oaq-3-next_phase-as-a-state)).
- `REWINDING` / `REPLAYING` — phase-graph operations; project remains in a recovery composite.

### 6.2 Phase node states

```text
PENDING → READY → CONTEXT_REVIEW → IMPLEMENTING → TESTING → REVIEWING
       → PHASE_COMPLETE
       → DIAGNOSING → REPAIRING → REWINDING → REPLAYING
       → WAITING_FOR_INPUT | HUMAN_VALIDATION | BLOCKED | FAILED
```

Legal transitions are encoded in code (table), not prose. Invalid transitions are bugs.

**Decision: nested project + phase machines**

- **Needed:** resume “which phase, which step, which repair attempt.”
- **Simpler alternative:** one enum as listed in the PRD. Rejected because `IMPLEMENTING` and `PROJECT_CREATED` are different kinds of state, and `repair_attempt` on the project object cannot represent two in-flight phases.
- **Preferable:** project = run; node = unit of DAG execution.
- **Revisit if:** the PRD is amended to a single official enum with defined transitions; then persist that enum and keep the nest as an internal implementation detail.

---

## 7. Phase / DAG engine

Each node is a phase as specified in PRD §21 (ID, name, objective, deps, inputs, outputs, REQs, ACs, tasks, tests, design work, risks, human validation, DoD).

Edges mean **must complete before**. Independent ready nodes are **parallel candidates** recorded on the graph for rewind/replay and future scheduling. **V1 executes one ready node at a time** (RAD-3 / ADR-007).

Capabilities:

- Topological ready-set
- Parallel candidate detection (independence)
- Affected-set: given node `N` changed, all descendants that consume `N`’s outputs
- Replay plan: ordered subset, skipping unrelated branches
- Checkpoint pointer per node (`commit`, metadata path)

Rewind = move HEAD (or create a repair branch from) `last_known_good_commit` of the origin node, mark descendants stale, then replay the affected-set.

**Decision: explicit DAG in `graph.json`, not LLM-remembered phases**

- **Needed:** rewind/replay and “only affected downstream nodes.”
- **Simpler alternative:** linear phase list. Rejected: PRD requires parallel work and partial replay.
- **Preferable:** a few hundred lines of DAG code, JSON on disk.
- **Revisit if:** phases become dynamic at sub-minute granularity; still a DAG, just finer nodes. Do not introduce a workflow SaaS.

---

## 8. Persistence / state model

### 8.1 Layout (product repo)

```text
.project/
  config.yaml              # optional project overrides
  LOCK                     # exclusive run lock (pid + started_at)
  state/
    state.json             # current snapshot (atomic replace)
    graph.json             # DAG snapshot (atomic replace)
    trace.json             # REQ ↔ AC ↔ TEST ↔ file ↔ CI mapping
    events.jsonl           # append-only audit (no secrets)
  phases/<phase-id>/
    plan.json
    packet.json
    execution.json
    review.json
    gate.json
  checkpoints/<phase-id>--<commit>.json
  execution/               # captured command transcripts (redacted)
  reviews/
  repairs/
  artifacts/
  feedback/
  knowledge/               # project-scoped; see knowledge system
```

Git remains the code checkpoint. Checkpoint JSON stores SHA + plan/review/cost/knowledge references, not copies of the tree.

### 8.2 `state.json` (minimum)

Machine-readable fields from PRD §10, plus what resume actually needs:

- `project_id`, `product_root`, `prd_path`, `prd_hash`
- `current_phase_id`, `current_state`
- `current_commit`, `last_known_good_commit`
- `repair_attempt` **per failure incident / repair chain** (RAD-1), not per project
- `blocked_reason`, `human_action_required` (structured, not a paragraph)
- `budget`, `cost_actual`, `human_forecast`
- `updated_at`

Natural-language notes may exist beside this file. They are never the state.

### 8.3 Durability

- Write `*.json` via temp file + `rename`.
- Append `events.jsonl` before starting a side effect and after completing it (`intent` / `result`) so a crash can be reconciled.
- `prdpr resume` replays: if `intent` exists without `result`, run the step’s **idempotent recovery** (e.g. read Git status, CI status) rather than blindly repeating a push.

**Decision: snapshot JSON + JSONL journal, not a DB**

- **Needed:** resume and diagnosis evidence.
- **Simpler alternative:** only `state.json`. Weaker crash recovery around Git/CI. Journal is small and local.
- **Preferable:** matches “state must not depend on NL logs” while still having an audit trail.
- **Revisit if:** journal growth is a problem—rotate by run ID.

**Decision: exclusive lock file**

- **Needed:** two `prdpr run` instances would corrupt state and Git.
- **Simpler alternative:** hope the user does not double-run. Insufficient.
- **Revisit if:** a long-running daemon is introduced; then the lock is held by the daemon.

---

## 9. PRD parsing

Parser inputs: `PRD.md` (UTF-8 Markdown).

Outputs: a typed `PRDDocument`:

- product definition, goals, non-goals, users, journeys
- requirements (`REQ-*`), acceptance criteria (`AC-*`), tests (`TEST-*`)
- design, stack, architecture, dependencies, credentials, security
- phases, human validation, Definition of Done

**V1 parsing strategy (layered, cheapest first):**

1. **Conventional headings** and list structure.
2. **Stable IDs** via regex (`REQ-001`, `AC-001`, `TEST-001`).
3. **Schema completeness check** (required sections present? IDs unique? AC linked to REQ?).
4. **Ambiguity pass** for material gaps (behavior, architecture, platform, security, external API, important UX). Blocking ambiguities → one question at a time (PRD §17).

The parser does not require YAML frontmatter. If a PRD already uses IDs, those IDs win. If IDs are missing, the parser assigns stable IDs and writes them into structured metadata (not by rewriting the user’s PRD unless a later command is explicitly defined). [OAQ-6](#oaq-6-prd-strictness-and-id-assignment).

Traceability index (`trace.json`):

```text
REQ → AC → implementation paths → TEST → CI job → artifact → human validation
```

This index is how the orchestrator answers “which tests prove REQ-004?” and “which REQs does this diff touch?” (diff ∩ indexed paths, plus test map).

**Decision: schema + regex first, LLM only for ambiguity/normalization**

- **Needed:** stable IDs and traceability.
- **Simpler alternative:** send the whole PRD to an LLM and trust JSON. Rejected: IDs and DoD must be deterministic.
- **Preferable:** cheap, diffable, testable parser; LLM for prose that has no structure.
- **Revisit if:** real PRDs are too irregular and the conventional-heading contract is amended.

---

## 10. Planning

Planning is orchestrator work, not Cursor coding work (Cursor may be used later to implement the plan).

Steps:

1. Build/refine DAG from PRD phases and implied work.
2. **Technical plan:** stack, module boundaries, ADRs to write, test strategy, CI, risks.
3. **Design plan:** whether the product needs a template, component library, AI-generated design, Figma, code-first, or a mix. Figma is never mandatory. Outputs may include `DESIGN.md`, tokens, screens, components, UX flows, assets.
4. **Convergence:** conflicts (e.g. a screen the architecture cannot support) become either a graph mutation or a human question.
5. **Human involvement forecast** and **cost estimate** before `READY`.

Plans are files under `.project/phases/<id>/plan.json` plus human-readable markdown in the product repo when the plan is an artifact the implementation worker must follow (`ARCHITECTURE.md`, `ADR/*`, `DESIGN.md`).

**Decision: planners are LLM-assisted; the DAG and ADR files are the plan**

- **Needed:** convert PRD into an executable graph (PRD goals).
- **Simpler alternative:** always implement PRD sections in document order. Rejected: dependencies, design parallelism, rewind.
- **Revisit if:** planning quality is the bottleneck—then change prompts/models, not the storage shape.

---

## 11. Design integration

Design is a first-class DAG region, not a prompt preface.

- A design node’s outputs are inputs to implementation nodes.
- Design review (PRD §24) is a quality gate for those nodes: flows, empty/loading/error/permission states, a11y, platform conventions.
- Visual review uses screenshots or equivalent artifacts when practical (test engine + artifacts folder), not a Figma plugin requirement.

No Figma API in V1 unless a specific product PRD names Figma as a blocking dependency. Even then, prefer export-on-disk over live Figma MCP.

---

## 12. Model routing

```text
task/role  →  Model Router  →  LLM Adapter  →  configured provider/model
coding     →  Cursor Worker (specialized; not an LLM adapter)
```

The router selects **the cheapest model that can reliably do this task type**, using:

task, complexity, risk, context requirements, budget, historical performance.

Task types (roles, not agents): planning, architecture reasoning, review, diagnosis, test generation, learning, other reasoning.

**Do not hardcode vendors in the core.** Providers are adapter implementations selected by config. Adding a provider must not change the engine.

**Coding** is delegated to **Cursor**, a worker with its own model setting. The router may still record Cursor’s model if the CLI exposes it, for the cost ledger.

Outcomes (success, repair count, cost, duration) write to `knowledge/model-performance/` (project) and may be promoted to orchestrator-global knowledge.

**Decision: Model Router + LLM adapters for reasoning; Cursor for coding** (ADR-004, ADR-008)

- **Needed:** independent review (PRD §32) and cheaper models for simple tasks, without a swarm of role-agents.
- **Simpler alternative:** one Cursor session for everything. Rejected: the worker would review itself; cost routing would be opaque.
- **Preferable:** two lanes (reasoning vs coding) with a vendor-neutral router.
- **Revisit if:** a single adapter cannot meet latency/cost needs, or Cursor CLI becomes the only legal model transport—then keep the router interface and swap adapters.

If a configured budget would be exceeded, pause and inform the human (PRD §43). No silent overage.

---

## 13. Subagent routing

Default: **NO_SUBAGENT**.

The DAG may mark independent nodes. V1 still runs them **sequentially**. The subagent module may record `USE_SUBAGENT` | `NO_SUBAGENT` for learning/tests; the scheduler ignores `USE_SUBAGENT` in V1.

**Decision: sequential execution in V1** (ADR-007)

- **Needed:** avoid swarm complexity and dual writers on one tree.
- **Simpler alternative:** fan out one Cursor per independent node. Rejected for V1.
- **Preferable:** graph encodes parallelism; scheduler does not use it yet.
- **Revisit when:** wall-clock on independent nodes is a measured bottleneck. Then isolated **worktrees or branches** plus a merge node—never two workers on one working tree.

---

## 14. Cursor execution

Cursor is a **worker**, not the orchestrator.

The orchestrator:

1. Builds a **task packet** (PRD §27): phase, objective, REQs, ACs, architecture, design, decisions, tests, constraints, DoD. Packets are files, not chat history.
2. Invokes Cursor CLI in the **product workspace** with a timeout.
3. Captures stdout/stderr, exit code, duration, changed files (`git status`/`diff`), tests the worker ran, commit if any, model, estimated cost.
4. **Does not** treat success text as success.

Workspace policy: default highly autonomous *inside* the configured root (PRD §28). The adapter passes the packet and permission profile; the orchestrator still enforces the jail via `fsguard` on any orchestrator-side file operations.

**Decision: packet-on-disk + subprocess, not an IDE plugin**

- **Needed:** initial environment is Cursor CLI.
- **Simpler alternative:** tell the user to paste the packet into Cursor. Rejected: not autonomous.
- **Preferable:** one integration surface. CLI flags/commands will change; isolate them in `internal/cursor`.
- **Revisit if:** Cursor CLI cannot apply patches non-interactively. Then the worker interface stays; the adapter changes. See [OAQ-8](#oaq-8-cursor-cli-contract).

The running `prdpr` binary is never a write target for a worker. Self-dogfooding uses a Git branch and a rebuild step, not in-place overwrite of the executing file.

---

## 15. Git / GitHub integration

GitHub is source of truth for remotes, PRs, CI, artifacts (PRD §48). Local Git is source of truth for the working tree and SHAs.

Orchestrator can: create branch, commit, push, open PR, read PR/check state.

**V1 GitHub policy (ADR-011):**

- One **run branch** per execution (e.g. `prdpr/<project-id>/<run-id>`).
- Each completed phase **commits** on that branch after verification (`phase → commit → verify → continue`).
- Open **one PR per run / meaningful milestone**, not one PR per phase. Milestone policy is configuration (`pr_boundary`: `run` default; later `phase` or custom), so the engine does not assume PR-per-node.
- **Do not merge** to the default branch unless explicitly configured.

Commits are real Git commits authored as the user (existing `user.name` / `user.email`). The orchestrator does not rewrite published history.

Never commit: credentials, Keychain exports, `.env` with secrets, unredacted execution logs that contain secrets.

**Decision: `git` + `gh` CLIs, not a GitHub App**

- **Needed:** personal developer tool, existing GitHub auth.
- **Simpler alternative:** GitHub App + cloud webhook. Rejected by non-goals (not a service).
- **Preferable:** user’s `gh auth`, Actions already in repo.
- **Revisit if:** token scopes cannot open PRs or read checks; still prefer PAT/`gh`, not a hosted orchestrator.

---

## 16. CI integration

GitHub Actions is the only CI system in V1.

Flow: push/PR → watch workflow/job status (poll `gh`, backoff) → on failure, fetch logs + artifacts → feed **diagnosis** → do not busy-loop.

Local tests can fail closed before CI to save time and cost. CI remains the independent remote gate when the phase requires it.

**Decision: polling from the CLI, not webhooks**

- **Needed:** CI drives orchestration state (P6).
- **Simpler alternative:** GitHub webhook server on the Mac. Rejected: extra daemon, ports, auth.
- **Preferable:** `prdpr` is already running or resumed.
- **Revisit if:** poll limits or long CI require a sleep-and-resume scheduler (still local: exit, cron/`launchd`, or `prdpr resume` later)—not a cloud worker.

---

## 17. Testing

The test engine is **independent** of the coding worker.

It discovers and runs applicable layers (unit, integration, API, DB, UI, E2E, regression, a11y, performance, security, static analysis, lint, types, build, packaging)—only those that exist or were planned for the phase.

Deterministic commands first. LLMs may *propose* tests (including adversarial cases, PRD §30) but proposed tests become repo files and then run deterministically.

Every confirmed bug should yield a regression test unless a structured reason is recorded (PRD §31).

Results update `trace.json` (TEST ↔ REQ/AC).

**Decision: orchestrator invokes project test commands; it does not invent a new test runner**

- **Needed:** independent verification.
- **Simpler alternative:** trust Cursor’s test output. Rejected by §7.5.
- **Preferable:** `go test`, `xcodebuild`, npm test, etc., as detected in preflight/plan.
- **Revisit if:** a product has no runnable tests—then generating tests is in-scope, running them is still deterministic.

---

## 18. Review

Every phase is reviewed by a **different lane** than implementation:

1. Deterministic checks: diff against packet, files exist, tests ran, CI, traceability coverage, lint/build.
2. LLM review when useful: requirements, architecture, tests, security, design.

Quality gate (PRD §33): only **applicable** gates. A phase cannot enter `PHASE_COMPLETE` until enabled gates pass.

Known issues: a gate may pass with a documented limitation list; “Known Issues ✓” means issues are recorded, not that the product is flawless. [OAQ-10](#oaq-10-known-issues-gate).

`prdpr review` is an **on-demand inspection / extra review pass**, not a substitute for the automatic gate. [OAQ-11](#oaq-11-cli-review-vs-engine-review).

---

## 19. Failure diagnosis

On any failed step (test, CI, review, worker timeout, preflight):

Output a structured diagnosis:

- `incident_id` (stable for the repair chain)
- `class`: `product` | `infrastructure` (infrastructure → retry/backoff, not repair attempts)
- `root_cause` class: PRD | Requirement | Planning | Architecture | Implementation | Testing | Environment | Dependency | External Service | Previous Phase | Infrastructure
- `confidence`
- `affected_phase_id`
- `recommended_action`: `repair` | `replan` | `rewind` | `ask_human` | `block` | `retry_infra`
- `evidence` (paths, SHAs, test names, CI job IDs, **prior attempt records**—no secrets)

Diagnosis may use an LLM **after** attaching deterministic evidence (logs, diffs, graph, previous attempts).

---

## 20. Repair

Self-fix loop (PRD §35–36):

1. Diagnose origin.
2. If origin is current phase: repair in place.
3. If origin is upstream: rewind to that phase’s last known good (or repair that phase), then replay affected descendants only.
4. Re-test and re-review.

**Three-attempt ladder** (per failure incident / repair chain — RAD-1 / ADR-006):

1. Local diagnosis + repair, using current evidence  
2. Deeper diagnosis + broader review, **using attempt 1 evidence**  
3. Independent review + stronger reasoning if justified, **using attempts 1–2 evidence**  

Infrastructure failures (RAD-2) use retry/backoff and **do not increment** this counter.

Then **STOP** → human debugging report (PRD §37) → `BLOCKED`. No fourth autonomous attempt. `prdpr retry` after a stop requires an **explicit human reset/reopen** of that incident.

Prompt-only retries without new evidence still count if they are product/code repair attempts.

---

## 21. Rewind / replay

Git SHA is the code rewind primitive. DAG affected-set is the work rewind primitive.

Example: P2 changes → replay P2, then P3 and P4, then P5; skip unrelated nodes.

Implementation sketch (behavioral, not code):

1. Record rewind target SHA + node ID.
2. Create `prdpr/rewind/<id>` from that SHA (do not destroy uncommitted work: stash or refuse if dirty).
3. Mark descendant nodes `STALE`.
4. Execute replay plan.
5. Preserve repair metadata under `.project/repairs/`.

**Decision: rewind by Git branch/SHA, not by copying trees into `.project/checkpoints/`**

- **Needed:** code checkpoints (PRD §38–39).
- **Simpler alternative:** zip the workspace each phase. Redundant with Git and large.
- **Preferable:** Git is already the VCS; checkpoint JSON points at SHAs.
- **Revisit if:** non-Git artifacts (derived binaries) must rewind—store those under `.project/artifacts/` keyed by SHA.

---

## 22. Human interaction

Channels of human work:

| Kind | State | Shape |
|---|---|---|
| Clarification | `WAITING_FOR_INPUT` | one question, reason, affected REQ, decision record |
| Credential | `WAITING_FOR_INPUT` | one secret at a time, Keychain write |
| Real-world validation | `HUMAN_VALIDATION` | validation task (build, minutes, tests, expected, report format) |
| Escalation | `BLOCKED` | debugging report, paused |
| Budget / policy | `BLOCKED` or `WAITING_FOR_INPUT` | confirm overspend or global policy |

CLI: `prdpr feedback`, `prdpr resume`. Answers are stored under `.project/feedback/` as structured records (question id, answer, timestamp, duration).

Forecast (PRD §18) is computed at `READY` and updated when the graph or blockers change. Display expected / potential / maximum interactions and minutes.

**Decision: blocking input is disk-backed; the process may exit**

- **Needed:** humans are slow; terminals close.
- **Simpler alternative:** stdin-only prompts. Insufficient for 30s notification and resume after reboot.
- **Preferable:** structured pending-action file + notify + `resume`.
- **Revisit if:** a tiny local UI is truly needed; it would still read the same pending-action schema.

---

## 23. Notification system

When blocking input is required:

1. Print what/why/estimated time/what happens next.
2. If no response for **30 seconds**, one attention signal (bell / macOS notification).
3. Do not ring every 30 seconds.
4. All of this is **configurable** (off, terminal-only, osascript/notification center).

**Decision: terminal + optional `osascript`/UserNotifications, no push service**

- Matches personal Mac tool. Revisit only if the user is routinely away from the machine—then still local (`launchd`), not a vendor push SaaS.

---

## 24. Credential handling

Rules from PRD §16 are invariants:

- never fabricate, never request early, never log, never commit, never put in PRs/knowledge, never give to unrelated workers
- one at a time
- store in **macOS Keychain** locally; **GitHub Actions Secrets** in CI

Orchestrator-side: a credential **reference** (Keychain account id) may appear in state; the secret value may be injected only into the subprocess that needs it, via env, not argv.

Redaction: `internal/redact` applied to all captured transcripts before disk.

**Decision: Keychain, not `.env` in the product repo**

- **Needed:** secrets exist (API keys, etc.).
- **Simpler alternative:** `.env` gitignored. Easier to leak into PRs and knowledge.
- **Preferable:** OS secret store + explicit request UX.
- **Revisit if:** non-macOS becomes a target; then add a second store backend behind the same interface.

---

## 25. Knowledge / learning system

After every phase, a Learning Review (PRD §40) writes **observations** (structured JSON + optional prose).

Promotion (PRD §42):

```text
Observation → Candidate Lesson → Validated Lesson → Policy
```

V1 storage: files under project `.project/knowledge/` and orchestrator-global `~/Studio/Tools/prd-pr/knowledge/` (or `~/.config/prd-pr/knowledge/`) for reusable lessons. **No vector DB.** Retrieval = filename conventions + `rg`/substring search.

Global policy changes require human confirmation unless configured otherwise.

Self-improvement: lessons about models, tools, human timing, and repair feed routers and forecasts. They do not auto-edit orchestrator code. Code changes go through the same DAG/PR loop (P13 dogfooding).

**Decision: versioned files, two scopes (project vs global)**

- **Needed:** learn without poisoning all future products from one incident.
- **Simpler alternative:** one global markdown diary. Rejected: no promotion, easy secret leak, no scope.
- **Revisit if:** search quality fails on a large corpus—still prefer extra indexing files over embeddings until proven.

---

## 26. Cost tracking

Ledger entries: LLM cost, model, tokens if available, CI cost if available, external API cost, subagent cost, duration.

Per phase: estimate vs actual. Run: budget remaining.

Pause if configured budget would be exceeded.

CI cost may be incomplete ([OAQ-12](#oaq-12-ci-and-currency)). Store what APIs provide; never invent precise billing.

Display currency follows config (PRD examples use ₹). Store amounts with **currency code + minor units**, not a hardcoded rupee assumption.

Human time: expected vs actual, interaction count, reason. Actual time = timestamp from notify/prompt to recorded answer unless the human supplies a duration ([OAQ-13](#oaq-13-human-time-measurement)).

---

## 27. Security boundaries

| Boundary | Rule |
|---|---|
| Workspace jail | Orchestrator file I/O limited to product root + orchestrator config/knowledge paths |
| Foreign repos | No access |
| Secrets | Keychain/Actions only; redacted logs; stripped from packets by default |
| Destructive external | No production deploy, no credential deletion/rotation, no financial actions unless explicitly configured |
| Worker | Cursor may edit the workspace; orchestrator still verifies Git diff stays inside root |
| Self-binary | Worker cannot overwrite the running `prdpr` |
| Network | Only configured APIs (LLM, GitHub). No open-ended browsing requirement in V1 |

Preflight records OS, arch, tools, creds **presence** (not values), Git dirtiness, branch.

---

## 28. CLI architecture

Single binary `prdpr`. Subcommands from PRD §46:

| Command | Role |
|---|---|
| `init` | Create `.project/`, initial state |
| `inspect PRD.md` | Parse + report; no execution |
| `run PRD.md` | Full loop |
| `status` | Snapshot + live if lock held |
| `pause` | Cooperative pause at next safe point; persist |
| `resume` | Continue from disk |
| `review` | Extra/on-demand review |
| `retry` | Re-enter a **human-reopened** incident only; never a silent fourth autonomous attempt (RAD-1) |
| `feedback` | Submit answer/validation |
| `logs` | Redacted transcripts |
| `phase` | List/show phase graph status ([OAQ-15](#oaq-15-prdpr-phase-semantics)) |
| `knowledge` / `learn` | Inspect or run learning review |
| `doctor` | Preflight-like health of the **orchestrator** environment |

Primary UX: `prdpr run PRD.md` with a compact live status (PRD §47).

Signals: SIGINT → cooperative pause attempt, then exit; state must remain resumable.

Config search order: flags > env `PRDPR_*` > `<product>/.project/config.yaml` > `~/.config/prd-pr/config.yaml`.

**Decision: foreground CLI + disk resume, not a daemon**

- **Needed:** personal tool, `pause`/`resume`.
- **Simpler alternative:** always-on agent daemon. Rejected for V1 complexity.
- **Revisit if:** CI watches must outlive the terminal and polling-on-resume is too coarse—then a optional `launchd` helper, still local.

---

## 29. Extensibility

Extension points are **Go interfaces + config**, not plugins or MCP:

- `Worker` (Cursor first)
- `LLMClient` / adapter (configured providers; no vendor in core)
- `VCS` / `CI`
- `Notifier`
- `SecretStore`
- `TestRunner` (discovered per ecosystem)

New product languages should look like new test/preflight detectors, not a new orchestrator.

Do not build an agent marketplace or generic swarm kernel.

---

## 30. Testing architecture (for PRD→PR itself)

- Unit tests for DAG, state transitions, parser IDs, redaction, affected-set, 3-attempt rule.
- Adapter tests with fakes (fake Cursor, fake `gh`).
- Golden PRDs → golden `PRDDocument` + graph.
- Benchmark products (PRD §53) become **end-to-end regression fixtures** over time, not V0 blockers.
- A “deliberately broken project” fixture for P9.

No requirement for Kubernetes or remote device labs in V1. iOS benchmarks need local Xcode when that benchmark runs.

---

## 31. Observability

- Structured logs (JSON lines) to stderr and `.project/execution/` (redacted).
- `events.jsonl` for state transitions and side effects.
- `prdpr status` / `prdpr logs` as the user-facing view.
- No hosted APM in V1.

Metrics that matter: phase duration, cost, repair count, human minutes, gate failures. These are ledger fields, not Prometheus.

---

## 32. Failure modes

| Failure | Handling |
|---|---|
| Process crash / laptop sleep | Lock stale-detection; resume from snapshot + journal |
| Dirty Git / unexpected commit | Preflight or step abort; do not reset unless rewind |
| Cursor timeout / hang | Kill process group; diagnose as implementation/environment |
| Model API down | Retry with backoff; cheaper fallback if router allows; else wait/block |
| GitHub/CI/LLM/Cursor **infrastructure** outage, rate limit, network, sleep | Retry/backoff/resume; **do not** increment repair attempts (RAD-2) |
| Missing optional dependency | Continue unrelated nodes |
| Missing blocking dependency | Block dependent nodes; ask human if necessary |
| Budget | Pause |
| Secret in diff | Fail gate; never push |
| Three repair failures | Human report, `BLOCKED` |
| Ambiguous PRD | One question; `WAITING_FOR_INPUT` |

---

## 33. Recovery strategy

1. **Reconcile** world vs state (Git SHA, CI, lock, pending human).
2. **Never** increment the per-incident repair count for infrastructure failure (RAD-2).
3. **Idempotent steps:** commit if already committed, PR if already open, CI if already watched.
4. **Rewind** only on classified upstream defects, not on flaky network.
5. After human **explicit reset/reopen**, `retry`/`resume` may start a new incident counter. Silent fourth attempts are forbidden.

---

## 34. Data flow

```text
PRD.md
  → PRDDocument + trace.json
  → preflight report
  → graph.json + phase plans + DESIGN/ADR artifacts
  → forecast (human, cost)
  → task packet
  → Cursor (workspace files)
  → Git commit SHA (every phase)
  → GitHub PR (run / milestone, not every phase)
  → local tests + Actions
  → review + gate
  → diagnosis/repair/rewind (optional)
  → human validation record
  → knowledge files
  → next ready nodes
  → PROJECT_REPORT.md
```

Cost and model records attach to each arrow that invoked a model or CI.

---

## 35. Important interfaces / contracts

These are contracts for later code, not an implementation.

**Task packet** — versioned JSON the worker must receive. Stable fields: `schema_version`, `phase_id`, `objective`, `requirements[]`, `acceptance_criteria[]`, `constraints[]`, `dod[]`, `design_refs[]`, `adr_refs[]`, `test_commands[]`, `forbidden_paths[]`. No secret values.

**Phase node** — `id`, `deps[]`, `state`, `commit`, `current_incident_id?`, `repair_attempt` (on the incident, not the project), `gates[]`, `human_validation?`.

**Repair incident** — `incident_id`, `origin_phase_id`, `attempt` (1–3), `evidence[]` from prior attempts, `status` (`open` | `blocked` | `resolved` | `reopened`).

**Diagnosis** — see §19.

**Pending human action** — `id`, `kind` (`question`|`credential`|`validation`|`policy`|`debug`), `prompt`, `why`, `affected`, `est_minutes`, `created_at`.

**Worker result** — `exit_code`, `duration`, `changed_paths[]`, `commit?`, `transcript_ref`, `claimed_done` (ignored for gating).

**LLM client** — `Complete(task_type, messages, budget) → text + usage`. Implemented by adapters; router chooses which adapter/model. No vendor types in `engine`.

**Secret store** — `Has(ref)`, `Get(ref)`, `Put(ref, value)` ; values never logged.

Breaking these contracts requires an ADR.

---

## 36. Major architectural trade-offs

| Choice | Cost | Benefit |
|---|---|---|
| Single binary vs daemon | Resume is explicit | Simple ops, no background service |
| Files vs DB | Weaker ad-hoc query | Git-friendly, zero ops |
| Direct APIs vs MCP | One-off adapters | Matches V1 non-goals |
| Cursor as only coder vs multi-worker | Cursor outage blocks coding | Independent review + cost routing remain possible |
| Sequential DAG vs parallel Cursor | Slower wall clock | Safe worktree, cheaper V1 (ADR-007) |
| One PR per run vs per phase | Larger milestone diffs | Less GitHub noise; engine stays PR-agnostic (ADR-011) |
| Nested state vs PRD flat enum | Extra model | Actually resumable |
| Direct LLM + Cursor | Two auth configs | Worker cannot mark its own homework |

---

## 37. Self-hosting / self-improvement

PRD→PR is a product that can be pointed at `~/Studio/Tools/prd-pr/` with this PRD.

Rules that make that safe without extra architecture:

- Same DAG, workers, gates, PRs.
- Workspace root is the orchestrator repo; jail still applies.
- Build artifacts go to `dist/` or a temp dir; never overwrite the running binary.
- Global knowledge promotion still needs a human for policy.
- Benchmarks in §53 become the regression suite for the orchestrator’s own later phases.

Early phases are implemented largely by humans until the MVP loop exists; later the same architecture dogfoods itself (see `IMPLEMENTATION_PLAN.md`). That is not a second system.

---

## Open Architectural Questions

Resolved in review and removed from this list: OAQ-4 (sequential V1), OAQ-5 (per-incident attempts), OAQ-7 (router + adapters for reasoning, Cursor for coding; vendors are config), OAQ-9 (PR per run/milestone), OAQ-14 (explicit human reopen), OAQ-16 (infra does not consume attempts), OAQ-17 (three-attempt stop is `BLOCKED`, not `FAILED`).

Remaining items are implementation-time or still underspecified in the PRD.

### OAQ-1: Order of Phase Graph vs Design/Technical Planning

PRD §1 places Design/Technical Planning before Phase Graph. PRD §8 draws Phase Graph first, then the two planners. This architecture uses coarse graph → planners → convergence → refine graph. **Working default until the PRD is edited.**

### OAQ-2: Flat vs nested states

PRD §9 lists one enum mixing project and phase. Nested machines remain the implementation model. Exact persisted field names can be fixed when `state.json` is implemented.

### OAQ-3: `NEXT_PHASE` as a state

Treat as a transition, not a persisted state, unless the PRD is amended.

### OAQ-6: PRD strictness and ID assignment

Must authors supply `REQ-*` IDs? May the parser assign them without editing `PRD.md`? How free-form can sections be? Decide at parser implementation with golden PRDs.

### OAQ-8: Cursor CLI contract

Exact non-interactive invoke, permission flags, and result parsing will churn. Isolate in the Cursor adapter. Pin a CLI version when P4 (Cursor worker) starts.

### OAQ-10: Known issues gate

Does a documented known issue allow phase completion? Working default: yes if recorded and required ACs are not failed.

### OAQ-11: CLI `review` vs engine review

Working default: automatic per-phase review later; `prdpr review` is extra/inspect.

### OAQ-12: CI and currency

Actions cost is often unavailable. Store currency-tagged amounts; CI cost may be `unknown`. Display currency is config.

### OAQ-13: Human time measurement

Working default: clock from prompt to `feedback`, optional override.

### OAQ-15: `prdpr phase` semantics

Working default: list/show in V1; `run --phase ID` later.

### OAQ-18: Artifact meaning

Treat as phase-defined DAG outputs (binary, screenshots, Actions artifact, etc.).

### OAQ-19: Pause semantics

Working default: SIGINT stops scheduling, attempts graceful worker cancel, always persist.

### OAQ-20: Global knowledge location

Working default: project observations in `.project/knowledge/`; promoted global lessons in `~/.config/prd-pr/knowledge/`.

---

## Architecture summary

PRD→PR V1 is a **local Go CLI**: an **engine** coordinates a **persisted DAG**; **capability modules** parse, plan, code (Cursor), test, review, and diagnose. Git, tests, CI, and schemas are the truth. Reasoning goes through a **vendor-neutral Model Router + LLM adapters**; coding goes through **Cursor**. V1 **schedules sequentially**, commits **per phase**, opens **one PR per run/milestone**, and never auto-merges. Repair is **3 attempts per incident**, using prior evidence; **infrastructure failures do not count**. State is **`.project/` JSON + JSONL**. No MCP, database, swarm, or cloud orchestrator.

Accepted ADRs:

| File | Decision |
|---|---|
| [ADR-001](ADR/ADR-001-single-local-go-binary.md) | Single local Go binary |
| [ADR-002](ADR/ADR-002-filesystem-state-and-git-checkpoints.md) | Filesystem state + Git checkpoints |
| [ADR-003](ADR/ADR-003-dag-execution-and-recovery.md) | DAG execution and recovery |
| [ADR-004](ADR/ADR-004-cursor-as-coding-worker.md) | Cursor as coding worker |
| [ADR-005](ADR/ADR-005-independent-verification.md) | Independent verification |
| [ADR-006](ADR/ADR-006-three-attempt-repair-limit.md) | Per-incident 3-attempt repair; infra excluded |
| [ADR-007](ADR/ADR-007-sequential-v1-execution.md) | Sequential V1 scheduler |
| [ADR-008](ADR/ADR-008-model-router-and-llm-adapter.md) | Model router + LLM adapters |
| [ADR-009](ADR/ADR-009-no-mcp-or-database-v1.md) | No MCP or database in V1 |
| [ADR-010](ADR/ADR-010-workspace-security-boundary.md) | Workspace security boundary |
| [ADR-011](ADR/ADR-011-pr-per-meaningful-milestone.md) | PR per run/milestone; no auto-merge |
