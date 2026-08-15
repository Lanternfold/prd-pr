# Deterministic logic, LLM, and human

**The LLM is not the verification authority and does not own scheduling.**

Related: [FLOW.md](FLOW.md), ADR-005, ADR-008.

## Deterministic (must be code)

| Concern | Where |
|---|---|
| PRD contract validation | `prd.ValidateContractFile` |
| Graph dependencies and READY | `internal/graph` |
| State transitions and locking | `internal/state` |
| Packet construction from a phase | `plan.DeterministicPlanner` |
| Verification evidence | `internal/testeng`, `engine.Verify` |
| Git safety / commit gates | `engine.assertCommitGate`, `vcs` |
| Repair attempt limits | `internal/repair` (3) |
| Orchestration policy | `internal/engine` |
| Workspace jail | `internal/fsguard` |
| Redaction | `internal/redact` |

Default CLI injects `llm.None`. Completeness review and routed LLM review **skip the network**.

## LLM (optional semantic reasoning)

Used only when an `llm.Adapter` is injected **and** `modelrouter.Route` selects a model other than `NONE`.

Appropriate roles in code: completeness, review, diagnosis, repair reasoning, runtime diagnosis (`internal/llm` role constants).

Not appropriate: marking tests passed, choosing the next READY phase, mutating the PRD, storing secrets.

There is **no shipped production provider SDK** in core. `None`, `Static`, and `Fail` adapters exist for tests. Adding a provider is an `Adapter` implementation behind the router (see [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)).

## Human

Humans decide unresolved product questions, credentials **presence**, unsafe operations, manual ACs, repair exhaustion, and GitHub/Studio placement failures.

The human is not the scheduler. After `feedback` / `resume`, the engine continues.

Ask one question at a time. Do not paste secrets into `.project/` JSON.
