# Independent verification

## Status

Accepted

## Context

PRD §7.5: never blindly trust an agent. Implementation, tests the worker claims to have run, and natural-language success are not completion.

## Decision

The engine treats worker output as a claim. Completion requires independent checks the orchestrator runs: workspace files, Git state, tests, builds, CI when in scope, and acceptance criteria / quality gates. The implementation worker is never the only reviewer.

## Consequences

Extra runtime and model cost for review later. Local tests can fail closed before CI. Fake workers in tests must still go through the same gate interface.

## Revisit When

A verification layer is redundant with a stronger isolated worker *and* deterministic gates still pass without it. Do not drop independent tests.
