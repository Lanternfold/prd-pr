# Documentation index

Canonical documents for the **implemented** product. Historical design notes stay in the repository root and `ADR/`; they are labeled when they describe intent rather than current runtime.

## Users

| Document | What it answers |
|---|---|
| [USER_GUIDE.md](USER_GUIDE.md) | What PRD→PR is, install, first run, human stops, recovery, config |
| [CLI.md](CLI.md) | Every user-facing command: what it does, when to use it, what happens next |
| [CURSOR.md](CURSOR.md) | Interactive Cursor vs headless `cursor-agent`, plugin, Auto-review |
| [PRD_AUTHORING_CONTRACT.md](PRD_AUTHORING_CONTRACT.md) | What a PRD must contain to pass `validate-prd` |
| [../PRD_AUTHORING_RULEBOOK.md](../PRD_AUTHORING_RULEBOOK.md) | Longer authoring rules |

## Architecture (implemented)

| Document | What it answers |
|---|---|
| [FLOW.md](FLOW.md) | What we have built: end-to-end flow, actors, verification and Git boundaries |
| [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md) | Outer graph vs inner phase loop; every real loop |
| [PHASES.md](PHASES.md) | P0–P13 purpose, status, limitations |
| [GIT_GITHUB.md](GIT_GITHUB.md) | Engine-owned Git/GitHub lifecycle and delivery |
| [LLM_AND_HUMAN.md](LLM_AND_HUMAN.md) | Deterministic vs LLM vs human |

## Contributors

| Document | What it answers |
|---|---|
| [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) | Build, test, package, extend, invariants |
| [../ARCHITECTURE.md](../ARCHITECTURE.md) | Design intent, RADs, OAQs (not all sections are implemented) |
| [../IMPLEMENTATION_PLAN.md](../IMPLEMENTATION_PLAN.md) | Delivery history for P0–P13 |
| [../ADR/](../ADR/) | Accepted architectural decisions |

## Historical / target flow notes

| Document | How to read it |
|---|---|
| [CURRENT_ORCHESTRATOR_FLOW.md](CURRENT_ORCHESTRATOR_FLOW.md) | Snapshot notes; superseded by FLOW.md + GRAPH_AND_LOOPS.md where they disagree |
| [GRAPH_LOOP_TARGET_FLOW.md](GRAPH_LOOP_TARGET_FLOW.md) | Target wiring; status updated against the current engine |

Do not treat `ARCHITECTURE.md` §5 (`prdpr run PRD.md` as a full in-process product loop) as the interactive UX. Interactive UX is `/prdpr` + CLI `prepare`/`verify`. Headless graph walk is `prdpr phase`.
