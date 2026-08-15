# PRD-prd-pr-P1-self-development-bootstrap.md

# Product Overview

`prd-pr` currently contains a safety guard that refuses to invoke the normal coding worker against the PRD→PR orchestrator repository itself.

This PRD defines the smallest controlled change required to add a second execution arm: an explicit `SELF_DEVELOPMENT` mode that can safely modify the `prd-pr` repository.

This is a bootstrap PRD only. It must NOT implement the full PRD→PR V2 feature set.

Execution mode: SELF_DEVELOPMENT

P1 will be implemented manually in Cursor against the existing `prd-pr` repository. After P1 is implemented, tested, and released, the resulting `prdpr` becomes capable of executing the larger V2 PRD against itself.

# Goals

1. Preserve the existing refusal for ordinary product execution against the orchestrator repository.
2. Add an explicit `SELF_DEVELOPMENT` execution mode.
3. Make self-development opt-in and impossible to trigger merely because the current workspace happens to be `prd-pr`.
4. Create a dedicated execution boundary for self-development.
5. Persist and audit self-development state.
6. Run the full applicable `prd-pr` verification suite before self-development can be considered successful.
7. Keep the change small enough to review and verify manually in Cursor.
8. Preserve existing CLI behavior for ordinary product workflows.

# Non-Goals

1. Implement intelligent PRD preflight.
2. Implement multi-stack verification.
3. Implement human review.
4. Implement artifact discovery.
5. Implement dependency timing.
6. Implement review-pack documents.
7. Implement Execution Contract V2.
8. Implement automatic release.
9. Implement automatic local CLI update.
10. Remove, disable, or weaken the existing orchestrator-repository safety guard.
11. Rewrite the existing orchestration architecture.

# Stack

Language: Go

Repository: existing `prd-pr` repository

CLI: existing `prdpr` CLI

Testing: existing Go test suite and existing project verification commands

# Requirements

### REQ-001: Preserve normal orchestrator guard

The existing behavior MUST remain unchanged for ordinary product execution.

When a normal PRD targets the `prd-pr` orchestrator repository, the normal coding-worker path MUST refuse execution.

### REQ-002: Explicit self-development declaration

The execution model MUST support an explicit `SELF_DEVELOPMENT` mode.

The mode MUST be represented in persisted execution state or an equivalent durable execution contract.

### REQ-003: No implicit self-development

The engine MUST NOT infer `SELF_DEVELOPMENT` merely because:
- the current working directory is the `prd-pr` repository;
- the repository contains `.project`;
- the PRD title mentions `prd-pr`;
- the target project happens to be the orchestrator.

The PRD/execution request MUST explicitly declare self-development.

### REQ-004: Repository identity check

Before allowing `SELF_DEVELOPMENT`, the engine MUST confirm that the target repository is the configured/current `prd-pr` orchestrator repository.

A self-development request targeting another repository MUST be refused.

### REQ-005: Dedicated execution boundary

`SELF_DEVELOPMENT` MUST use an explicit execution path distinct from the ordinary product coding-worker path.

The implementation MUST NOT simply remove or bypass the existing guard.

### REQ-006: Precondition checks

Before self-development execution, the engine MUST confirm:
- explicit self-development mode
- target repository identity
- valid PRD
- ready phase
- no unresolved blocking condition known to the current engine
- repository is in an allowed state for self-development

If a required precondition fails, execution MUST be refused.

### REQ-007: Persistent lifecycle state

The execution state MUST distinguish at minimum:
- normal execution
- self-development execution
- self-development refused
- self-development running
- self-development completed
- self-development failed

### REQ-008: Auditability

A self-development run MUST record:
- mode
- target repository identity
- authorization/precondition result
- implementation result
- verification result

### REQ-009: Verification

A successful self-development implementation MUST run the existing applicable `prd-pr` verification/test suite before reporting success.

Coding-worker completion alone MUST NOT constitute successful self-development.

### REQ-010: Failure safety

If self-development implementation or verification fails, the engine MUST report failure and MUST NOT claim that the repository is successfully upgraded.

### REQ-011: CLI compatibility

Existing normal `prdpr` invocation behavior MUST remain compatible.

### REQ-012: Tests

Automated tests MUST cover:
- ordinary execution against orchestrator remains refused
- explicit self-development is accepted when all preconditions are satisfied
- self-development is refused without explicit declaration
- self-development is refused for a non-orchestrator repository
- self-development state is persisted
- self-development verification is required
- implementation/verification failure is reported correctly

# Acceptance Criteria

### AC-001: Normal guard preserved

Verifies REQ-001.

PASS when an ordinary product execution against the orchestrator repository still produces the existing refusal and does not invoke a coding worker.

### AC-002: Explicit mode

Verifies REQ-002, REQ-003.

PASS when an execution request containing explicit `SELF_DEVELOPMENT` mode is distinguishable from ordinary execution and an ordinary request cannot enter that mode implicitly.

### AC-003: Repository identity

Verifies REQ-004.

PASS when self-development is accepted only for the configured/current orchestrator repository and is refused for another repository.

### AC-004: Dedicated path

Verifies REQ-005.

PASS when self-development uses a dedicated execution path and the existing normal guard remains active for ordinary execution.

### AC-005: Preconditions

Verifies REQ-006.

PASS when each required precondition is checked before self-development starts and failed preconditions prevent execution.

### AC-006: Lifecycle state

Verifies REQ-007, REQ-008.

PASS when self-development state and repository identity are persisted and distinguishable from normal execution.

### AC-007: Verification

Verifies REQ-009, REQ-010.

PASS when self-development cannot report success until the existing applicable verification suite succeeds, and verification failure produces a failure result.

### AC-008: CLI regression

Verifies REQ-011.

PASS when existing normal CLI workflows continue to behave as before.

### AC-009: Automated coverage

Verifies REQ-012.

PASS when all required self-development and regression tests pass.

# Phases

## P1: Controlled Self-Development Bootstrap

Objective: Add a safe, explicit self-development execution arm without weakening ordinary orchestrator-repository protection.

Dependencies:

Requirements: REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007, REQ-008, REQ-009, REQ-010, REQ-011, REQ-012

Acceptance Criteria: AC-001, AC-002, AC-003, AC-004, AC-005, AC-006, AC-007, AC-008, AC-009

Implementation Tasks:
- Inspect the existing orchestrator-repository guard and execution state machine.
- Identify the smallest extension point for self-development.
- Add explicit `SELF_DEVELOPMENT` mode.
- Add repository identity validation.
- Add a dedicated self-development execution path.
- Preserve the existing normal coding-worker refusal.
- Persist self-development lifecycle state.
- Integrate existing verification.
- Add automated regression and self-development tests.
- Update relevant CLI/docs/help text.

Tests: TEST-001, TEST-002, TEST-003, TEST-004, TEST-005, TEST-006

Definition of Done:
- All requirements pass.
- All acceptance criteria pass.
- Existing normal orchestrator protection remains intact.
- Explicit self-development can reach implementation only after all preconditions pass.
- Self-development cannot report success without verification.
- Automated tests pass.
- No V2 preflight, review, release, or multi-stack features are implemented as part of P1.

# Testing

TEST-001: Normal coding-worker invocation against orchestrator remains refused.

TEST-002: Explicit SELF_DEVELOPMENT invocation against the orchestrator is accepted when preconditions pass.

TEST-003: SELF_DEVELOPMENT without explicit declaration is refused.

TEST-004: SELF_DEVELOPMENT targeting a non-orchestrator repository is refused.

TEST-005: Self-development lifecycle and authorization state persist correctly.

TEST-006: Self-development cannot succeed when verification fails.

# Definition of Done

P1 is complete when `prd-pr` has a safe, explicit, tested self-development arm that preserves the existing normal guard and is ready for P2 to use through `prdpr`.
