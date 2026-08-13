# PR per meaningful milestone

## Status

Accepted

## Context

The PRD’s GitHub phase DoD said a phase *can* result in a PR. Per-phase PRs add noise and couple the engine to GitHub. Local verify should not wait on PR creation.

## Decision

Default flow is **phase → commit → verify → continue**. Open **one GitHub PR per run / meaningful milestone**, not per phase. PR boundary is configuration (`pr_boundary`: default `run`; later `phase` or custom) so the engine does not assume a PR per node. **Do not merge** to the default branch unless explicitly configured.

## Consequences

CI that requires a PR runs at the milestone, not after every phase. Local tests still run per phase. Milestone diffs are larger than per-phase PRs.

## Revisit When

Reviewers need smaller PRs *and* `pr_boundary=phase` is enabled—without changing the DAG or engine. Auto-merge only with an explicit config and passing gates.
