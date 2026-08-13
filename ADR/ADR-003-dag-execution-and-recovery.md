# DAG execution and recovery

## Status

Accepted

## Context

Work has dependencies, independent branches, and failures whose origin may be upstream. Blind retry is forbidden. Rewind and replay must affect only descendants.

## Decision

Represent the project as an explicit DAG (`graph.json`). Edges are “must complete before.” The graph records independent nodes and potential parallelism. The **engine** walks the graph, persists node state, and computes affected-sets for rewind/replay. Git SHA is the code rewind primitive. V1 still **schedules sequentially** (ADR-007).

## Consequences

Recovery is graph math plus Git, not LLM memory. Planning can refine the graph as a recorded mutation. A linear list would be simpler but cannot partial-replay.

## Revisit When

Phases become so fine-grained that graph overhead dominates, or true parallel workers are introduced (still a DAG; scheduler changes, not the model).
