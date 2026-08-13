# Filesystem state and Git checkpoints

## Status

Accepted

## Context

The orchestrator must persist structured state, resume after crash, and rewind code. The PRD prefers the filesystem for knowledge and Git for checkpoints, and does not want a database in V1.

## Decision

Product orchestration metadata lives in `<product>/.project/` as JSON snapshots plus an append-only `events.jsonl` journal (atomic replace, no secrets). Git SHAs are the code checkpoint. Checkpoint JSON points at SHAs; it does not copy the tree.

## Consequences

State is inspectable, Git-friendly, and zero-ops. Queries are file reads, not SQL. Crash recovery uses snapshot + journal reconciliation. Large journals may need rotation by run ID.

## Revisit When

Concurrent writers, journal size, or query patterns make files painful. SQLite would be the next step—not a hosted database.
