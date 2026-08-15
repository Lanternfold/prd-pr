# What PRD→PR has built

Canonical description of the **implemented** system. Design intent that is not wired yet is marked PARTIAL or PLANNED.

Companion: [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md), [PHASES.md](PHASES.md), [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md).

Root [ARCHITECTURE.md](../ARCHITECTURE.md) is the design document (RADs, OAQs). Prefer this file for “what runs today.”

## End-to-end flow

```text
PRD
 ↓
deterministic validation          IMPLEMENTED
 ↓
semantic completeness             PARTIAL (only if LLM adapter ≠ None)
 ↓
human clarification when required IMPLEMENTED
 ↓
Studio/project bootstrap          IMPLEMENTED
 ↓
Git/GitHub bootstrap              IMPLEMENTED local Git; GitHub PARTIAL (off by default)
 ↓
phase graph                       IMPLEMENTED
 ↓
READY phase                       IMPLEMENTED (explicit --phase cannot bypass READY)
 ↓
prepare packet                    IMPLEMENTED
 ↓
Cursor implementation             IMPLEMENTED (plugin session XOR cursor-agent)
 ↓
independent verification          IMPLEMENTED
 ↓
review/diagnosis                  IMPLEMENTED deterministic; LLM PARTIAL
 ↓
bounded repair loop               IMPLEMENTED (≤3 product attempts)
 ↓
verified phase                    IMPLEMENTED
 ↓
commit/delivery                   IMPLEMENTED (AutoCommit/AutoPush defaults true)
 ↓
next READY phase                  IMPLEMENTED headless via RunGraph; interactive via repeated prepare
 ↓
AllCompleted
 ↓
runtime                           IMPLEMENTED start; repair loop PARTIAL
 ↓
done
```

## Actors

| Actor | Responsibility | Not authoritative for |
|---|---|---|
| ENGINE | Orchestration, state, policy, packets, gates | Product taste; Cursor UI |
| GRAPH | Phase dependency and READY/BLOCKED | When to invoke a worker; verification |
| CURSOR | Implementation of the current packet | Verification, Git lifecycle, scheduling |
| SHELL | Tests, local runtime, git/gh subprocesses | Pass/fail policy (engine interprets) |
| LLM | Optional semantic completeness, review, diagnosis | Verification; scheduling; PRD mutation |
| HUMAN | Unresolved product decisions and exceptions | Day-to-day phase sequencing |
| GITHUB | Remotes, PRs, rulesets when enabled | Local verification |
| CI | Observe Actions checks | Repair strategy; local test gate |

**Cursor saying “done” does not establish verification.** `prdpr verify` / `internal/testeng` does.

## Verification boundary

- Worker `claimed_done` / `worker_claimed_success` is recorded and ignored for gating.
- Only verification evidence may set `verified_success=true`.
- Unverified implementation must not be committed or pushed (`requireVerifiedExecution`, `assertCommitGate`).
- Manual ACs that tests cannot cover stop for a human.

## Git/GitHub boundary

Git/GitHub lifecycle is **engine-owned**. Cursor must not independently commit, push, open PRs, merge, or rewrite history unless the engine packet explicitly requires a product-side Git change (unusual). Engine packets list `forbidden_paths` and the plugin skill forbids launching `cursor-agent`.

See [GIT_GITHUB.md](GIT_GITHUB.md).

## Studio bootstrap

```text
user provides PRD path
        ↓
contract validation
        ↓
optional LLM completeness (skipped on llm.None)
        ↓
bootstrap.SelectType
        ↓
studio.Discover (PRDPR_STUDIO, cwd walk, $HOME/Studio)
        ↓
create/reuse destination
        ↓
place PRD.md, type scaffolding, .project, Cursor rules
        ↓
git init / baseline
        ↓
optional GitHub remote + rulesets
        ↓
prepare next READY phase
```

The user should not create the product directory manually.

## Package map (actual tree)

```text
cmd/prdpr                 binary entry
internal/
  cli/                    CLI dispatch (not orchestration)
  engine/                 run/prepare/verify/repair/graph walk/delivery
  prd/                    parse + contract validation
  graph/                  DAG, Ready, Refresh, serialize
  state/                  .project state, lock, events
  packet/                 task packet schema
  plan/                   deterministic phase → packet
  preflight/              advisory readiness
  cursor/                 P4 worker adapter (not the plugin)
  testeng/                independent tests
  review/                 quality/review result
  modelrouter/            NONE/cheap/strong decision
  llm/                    Adapter interface + None/Static/Fail
  diagnose/               Classify (deterministic)
  repair/                 incidents, 3-attempt rule, rewind plan
  human/                  request/response files
  knowledge/              project observation files
  cost/                   .project/cost.jsonl
  subagent/               Decide; V1 always sequential (NO_SUBAGENT default)
  ci/                     gh checks observer
  vcs/                    git + gh
  proc/                   subprocess runner
  fsguard/                workspace jail
  config/                 Defaults() struct
  bootstrap/              type, place, Cursor rules
  studio/                 Studio layout discovery
  apprun/                 structured local runtime
  notify/                 optional attention bell
  redact/                 secret scrubbing
prdpr-cursor/             thin plugin (command + skill)
```

There is no `internal/report` package. `PROJECT_REPORT.md` generation is **PLANNED**.

| Package | Callers | Must not own |
|---|---|---|
| `engine` | `cli` | Domain parse/test implementations |
| `graph` | `engine`, `cli/inspect` | Worker invocation |
| `prd` | `engine`, `cli` | Execution |
| `cursor` | `engine` | Plugin UX; verification |
| `testeng` | `engine` | Writing product code |
| `vcs` | `engine` | CI interpretation |
| `human` | `engine`, `cli` | Notification transport |
| `prdpr-cursor` | user in Cursor | DAG, repair counts, Git policy |

## Persistence

Product repo `.project/` holds `state.json`, `graph.json`, packets, execution, incidents, human request/response. Git holds code. No database (ADR-009).
