# No MCP or database in V1

## Status

Accepted

## Context

The PRD lists MCP platforms, databases, queues, vector DBs, and extra agents as things not to introduce without demonstrated need.

## Decision

V1 uses direct integrations (`git`, `gh`, Cursor CLI, LLM HTTP) and filesystem state (ADR-002). No MCP servers, no SQL/vector database, no message queue, no cloud orchestrator.

## Consequences

Each integration is a small adapter. There is no generic plugin bus. Knowledge search is files + ripgrep/substring.

## Revisit When

An integration cannot be called directly (MCP might then wrap that one tool), or filesystem state is proven insufficient (then SQLite locally—not a hosted DB).
