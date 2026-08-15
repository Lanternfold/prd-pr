# PRD: Cycle Product

**Status:** Draft
**Product:** Cycle Product
**Owner:** Test

# 1. Product Overview

A Go library with two sequenced helpers that are incorrectly cyclic.

# 2. Goals

- Ship Add then Sub

# 3. Non-Goals

- Multiply

# 4. Requirements

- REQ-001: Add(a, b int) int returns a+b
- REQ-002: Sub(a, b int) int returns a-b

# 5. Acceptance Criteria

- AC-001: Add(2, 2) returns 4
- AC-002: Sub(5, 3) returns 2

# 6. Testing

- TEST-001: unit tests

# 7. Phases

## P1: Add

Objective: Implement Add
Dependencies: P2
Requirements: REQ-001
Acceptance Criteria: AC-001
Tests: TEST-001
Definition of Done:
- tests for Add pass

## P2: Sub

Objective: Implement Sub
Dependencies: P1
Requirements: REQ-002
Acceptance Criteria: AC-002
Tests: TEST-001
Definition of Done:
- tests for Sub pass
