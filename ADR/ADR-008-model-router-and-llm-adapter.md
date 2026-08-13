# Model router and LLM adapter

## Status

Accepted

## Context

Different tasks need different cost/capability. The PRD wants cheap models when they suffice. Hardcoding vendors in the core would freeze architecture to today’s providers. Persistent agents per role would turn the orchestrator into a swarm.

## Decision

```text
Model Router → LLM Adapter → configured model/provider
```

The router chooses the cheapest capable model from task, complexity, risk, context, budget, and historical performance. Planning, architecture, review, diagnosis, test generation, and learning are **task roles**, not agents. Cursor remains the coding worker (ADR-004) and is not the general LLM transport. Core code must not import a specific vendor SDK except inside an adapter package.

## Consequences

Two auth surfaces (LLM providers + Cursor). First adapter is chosen at implementation; swapping providers should not touch `engine`. Router can start with heuristics and later use recorded performance.

## Revisit When

A single transport is mandated (for example Cursor-only model calls), or adapter overhead is worse than one pinned provider with no routing. Keep the router interface either way.
