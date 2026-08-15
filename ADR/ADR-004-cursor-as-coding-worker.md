# Cursor as coding worker

## Status

Accepted

## Context

The PRD names Cursor CLI as the initial coding worker and forbids replacing Cursor. The orchestrator must not treat “Done.” as evidence.

## Decision

Cursor is a specialized **coding worker** invoked with a task packet on disk, a workspace root, and a timeout. It is not the orchestrator, not the model router, and not the reviewer. CLI churn is isolated in an adapter. The running `prdpr` binary is never a write target.

The **Cursor plugin** (ADR-012) is a different layer: user-facing entry in the IDE. It must not replace or merge with this worker adapter. P4 is unchanged.

## Consequences

Coding quality depends on Cursor CLI. Non-interactive flags must be pinned when implementation starts (OAQ-8). Reasoning tasks use LLM adapters (ADR-008), so Cursor outage blocks coding but not necessarily planning/review.

## Revisit When

Cursor CLI cannot apply work non-interactively, or a second coding worker is required. Keep the `Worker` interface; swap the adapter.
