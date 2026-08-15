# Cursor Plugin as primary UX; Go remains the core engine

## Status

Accepted

## Context

PRD→PR is used from Cursor. Earlier architecture treated the Go CLI as the primary workflow (`prdpr run`) and rejected an IDE plugin as the *coding worker* (packet-on-disk + subprocess). Those two ideas were collapsed: “not a plugin” for execution became “no Cursor-native UI.”

Users work in Cursor. A second, engine-shaped CLI as the main product surface fights that. Duplicating orchestration inside a plugin, MCP server, SDK agent, or subagent graph would create a second orchestrator.

The Go engine already owns parse, graph, preflight, state, planning, workers, and (later) verification and repair. It must stay independently testable without Cursor installed.

## Decision

**Primary user interface:** a Cursor-native Plugin (thin adapter).

**Core engine:** the existing local Go binary. It does not depend on Cursor IDE, the plugin, MCP, or cloud services.

Conceptually:

```text
User
 ↓
Cursor
 ↓
PRD→PR Plugin
 ├── command
 └── skill
 ↓
PRD→PR Go Engine
 ├── state
 ├── parser
 ├── graph
 ├── preflight
 ├── planning
 ├── workers
 ├── verification
 └── future repair/learning
```

Four layers stay distinct:

1. **Core Engine** — orchestration, DAG, state, planning, verification, repair, knowledge, model routing.
2. **CLI Interface** — `prdpr` commands over the engine. Remains independently executable and is the test/automation surface.
3. **Cursor Plugin Interface** — Cursor-facing entry point. Invokes the engine, passes workspace/project context, presents state/results, guides Cursor through the workflow. Does **not** own orchestration logic.
4. **Worker Adapters** — implementation execution (P4 Cursor worker: task packet + subprocess). The plugin is **not** the worker. Do not collapse plugin and worker into one abstraction (ADR-004).

The plugin must **not** duplicate: orchestration state, DAG, repair, retry counts, Git semantics, verification, knowledge storage, model routing, or PRD contract validation. Step 0 of `/prdpr` is `prdpr validate-prd`; REJECTED stops before project creation. After VALID, `prdpr <PRD.md>` bootstraps Studio placement and prepare. The plugin must not call `prdpr run`.

**Plugin V0 (interface milestone, not a new PRD phase):** plugin manifest, one `/prdpr` command, one PRD→PR skill, minimal documentation. No MCP, subagents, hooks, custom agents, SDK, cloud services, databases, or extra commands unless strictly necessary.

**Deferred (document only):** hooks, additional commands, Cursor Agent SDK adapter, optional subagents, MCP if a concrete need appears (ADR-009).

**CLI remains available** so the engine can be run, tested, and scripted without the plugin.

This does not redesign P4.

## Consequences

Day-to-day UX lives in Cursor; correctness still lives in Go. Two surfaces (plugin + CLI) must stay thin over the same engine or they will drift. Plugin V0 is small; richer Cursor features stay optional. The engine can still fake-worker test without Cursor. Coding still depends on the Cursor worker adapter when real implementation runs (ADR-004).

ADR-001 still holds: one local Go binary is the engine, not a second orchestrator. A Cursor plugin is an interface, not a daemon or SaaS.

## Revisit When

The plugin cannot invoke the engine with enough workspace context, or Cursor requires MCP/SDK/hooks for a *demonstrated* need that the thin command+skill cannot meet. Add that one capability; do not move orchestration into the plugin.
