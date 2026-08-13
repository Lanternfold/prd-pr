# Sequential V1 execution

## Status

Accepted

## Context

The DAG can express independent work. Parallel Cursor workers on one working tree would race. A swarm framework is a non-goal.

## Decision

V1 **executes one phase/task at a time**. The DAG still stores dependencies, independent nodes, potential parallelism, and affected descendants. Default subagent decision is **NO_SUBAGENT**. Future parallel execution must use isolated Git worktrees or branches plus a merge node.

## Consequences

Wall-clock is slower on independent nodes. Implementation and testing stay simple. Enabling parallelism later is a scheduler change, not a graph redesign.

## Revisit When

Benchmarks show independent nodes dominate wall-clock *and* worktree isolation is implemented. Do not revisit by sharing one dirty tree.
