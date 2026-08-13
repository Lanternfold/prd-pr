# PRD: Fixture App

**Status:** Draft  
**Product:** Fixture  
**Owner:** Test  
**Repository:** example/fixture

# 1. Product Overview

A tiny app that adds two numbers.

# 2. Goals

- Ship an Add function
- Cover it with tests

# 3. Non-Goals

- Networking

# 4. Requirements

- REQ-001: The app must expose Add(a, b int) int
- REQ-002: Add must document overflow behavior

# 5. Acceptance Criteria

- AC-001: Add(2, 2) returns 4
- AC-002: Overflow behavior is documented

# 6. Testing

- TEST-001: unit test Add

# 7. Phases

## P1: Core

Objective: Implement Add
Dependencies:
Requirements: REQ-001, REQ-002
Acceptance Criteria: AC-001
Implementation Tasks:
- write Add
Tests: TEST-001
Definition of Done:
- tests pass
