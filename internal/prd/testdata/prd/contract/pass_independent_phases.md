# PRD: Two Libraries

**Status:** Draft
**Product:** Two Libraries
**Owner:** Test

# 1. Product Overview

Two independent Go helpers in one module: Add and Sub.

# 2. Goals

- Ship Add and Sub

# 3. Non-Goals

- Multiply/Divide

# 4. Requirements

- REQ-001: Add(a, b int) int returns a+b
- REQ-002: Sub(a, b int) int returns a-b

# 5. Acceptance Criteria

- AC-001: Add(2, 2) returns 4
- AC-002: Sub(5, 3) returns 2

# 6. Testing

- TEST-001: unit test Add
- TEST-002: unit test Sub

# 7. Phases

## P1: Add

Objective: Implement Add
Dependencies:
Requirements: REQ-001
Acceptance Criteria: AC-001
Tests: TEST-001
Definition of Done:
- tests for Add pass

## P2: Sub

Objective: Implement Sub
Dependencies:
Requirements: REQ-002
Acceptance Criteria: AC-002
Tests: TEST-002
Definition of Done:
- tests for Sub pass
