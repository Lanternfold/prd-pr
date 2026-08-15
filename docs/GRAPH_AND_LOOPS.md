# Graph and loops

How the phase graph and the real execution loops work **today**.

Status legend: **IMPLEMENTED** · **PARTIAL** · **PLANNED** · **OUT OF SCOPE**

## What the graph is

The graph is a **phase DAG** built from explicit PRD phase dependencies (`internal/graph.FromDocument`). Nodes are PRD phases, not tasks.

| Question | Answer |
|---|---|
| Where is it built? | `graph.FromDocument` during prepare (`internal/engine/prepare.go`) |
| Where is it persisted? | `<product>/.project/graph.json` |
| READY/BLOCKED | `Refresh()`: a node is READY when all dependencies are COMPLETED; otherwise BLOCKED with a reason code |
| Dependencies | Explicit `Dependencies` on PRD phases only. No inferred edges from numbering. |
| Completion | After verified delivery succeeds, `SetStatus(COMPLETED)` then `Refresh()` unblocks dependents |
| Persisted state | Node status, edges, diagnostics. Not worker transcripts. |
| What it does **not** control | Packet contents, Cursor invocation, test commands, Git commits, LLM routing, plugin UI |

`--phase` does **not** bypass READY. `selectRunnablePhase` refuses unknown, BLOCKED, COMPLETED, and WAITING nodes. RUNNING is allowed only for the already-active phase.

**OUT OF SCOPE:** task-level DAGs, parallel Ready() workers, live graph rewrite, subagent execution.

## Outer graph vs inner phase loop

```text
OUTER GRAPH (prdpr phase / Engine.RunGraph)
  while READY remains and not waiting:
      INNER LOOP (Engine.RunPhase)
         prepare/run worker → verify → review → repair≤3 → verify
      on verified delivery: mark COMPLETED, Refresh, pick next Ready()[0]
```

Interactive plugin does **not** call `RunGraph`. It repeats `prepare` (engine picks next READY) + implement + `verify` in the **current** Cursor session.

## 1. Phase implementation loop — IMPLEMENTED

Headless: `RunPhase`. Interactive: plugin sequence with the same engine commands.

```text
entry: READY (or RUNNING current) phase, packet on disk
→ actions: Cursor implements packet (session XOR cursor-agent)
→ feedback: git diff vs baseline + testeng
→ decision: verified / manual AC / repair / human
→ retry: PrepareRepair + worker, max 3 product attempts (ADR-006)
→ exit: verified_success, WAITING_FOR_HUMAN, or stop
→ max attempts: repair.MaxAttempts = 3; infrastructure does not consume attempts
```

Diagnose currently sets origin `local_phase` in the default classifier. Upstream rewind/replay is **PARTIAL** (`applyRewind` exists; live upstream origin is not the usual path).

## 2. Outer graph progression loop — IMPLEMENTED (headless) / PARTIAL (interactive)

```text
entry: RunGraph, or plugin prepare-without-phase
→ actions: RunPhase or prepare+human implement+verify
→ feedback: graph.json statuses
→ decision: AllCompleted / firstWaitingPhase / firstReadyPhase
→ retry: next READY in the same headless process; plugin starts another prepare
→ exit: project complete, waiting, or inner loop not completed
→ max attempts: none at graph level (sequential only)
```

Stale claim **corrected:** “there is no in-process scheduler” is false for `prdpr phase`. It is still true that the **plugin** is not a scheduler.

## 3. Human interruption / resume loop — IMPLEMENTED (with PARTIAL resume routing)

```text
entry: engine writes request.json, state WAITING_FOR_HUMAN
→ actions: notify once after HumanTimeout (default 30s); process may exit
→ feedback: prdpr feedback → response.json
→ decision: prdpr resume by request kind
→ retry: GitHub → Reconcile or Prepare; runtime → StartRuntime; manual AC → Verify; else leave failed/waiting
→ exit: owning command returns
→ max attempts: not applicable (human is unbounded)
```

Resume does **not** always walk the rest of the graph by itself.

## 4. Runtime feedback loop — PARTIAL

```text
entry: graph.AllCompleted → startRuntimeLocked / prdpr runtime
→ actions: apprun start from structured type definition (not arbitrary PRD shell)
→ feedback: ready / skipped / error
→ decision: skip, success, or RepairRuntime (diagnose + optional packet)
→ retry: human or plugin implements packet then `prdpr runtime` again
→ exit: ready, skipped, human, or exhausted
→ max attempts: same incident MaxAttempts; infra does not consume
```

CLI `prdpr runtime` starts once and may emit one repair packet. It is **not** a fully autonomous start→diagnose→repair-worker→restart loop inside one call.

## 5. Recovery — IMPLEMENTED (reconcile) / PARTIAL (all crash windows)

```text
entry: next engine command acquires lock, recoverLocked
→ actions: map WORKER_RUNNING/VERIFYING/VERIFIED/COMPLETED to a resumable state; retry commit/push/PR when appropriate
→ feedback: git SHA, push status, PR view, checks
→ decision: continue, skip duplicate side effects, or human for GitHub
→ retry: idempotent push/PR
→ exit: recovered state on disk
→ max attempts: infra retries are not the product 3-attempt counter
```

No force-push. No destructive reset of unrelated history.
