# PRD: Dogfood Hello

**Status:** Draft  
**Product:** Dogfood Hello  
**Owner:** Test  
**Repository:** example/dogfood-hello

# 1. Product Overview

A tiny Go library that returns a greeting. This PRD is consumed by the orchestrator; the orchestrator must bootstrap the product Git repository itself.

# 2. Goals

- Ship Hello()
- Cover it with tests

# 3. Non-Goals

- Networking
- Using the orchestrator repository as the product

# 4. Requirements

- REQ-001: Hello must return "hello"
- REQ-002: Hello must be covered by a unit test

# 5. Acceptance Criteria

- AC-001: Hello() returns hello
- AC-002: unit tests pass

# 6. Testing

- TEST-001: unit test Hello

# 7. Phases

## P1: Foundation

Objective: Implement product foundation
Dependencies:
Requirements: REQ-001
Acceptance Criteria: AC-001
Implementation Tasks:
- add Hello
Tests: TEST-001
Definition of Done:
- tests pass

## P2: Coverage

Objective: Add documented Hello coverage
Dependencies: P1
Requirements: REQ-002
Acceptance Criteria: AC-002
Implementation Tasks:
- keep Hello tested
Tests: TEST-001
Definition of Done:
- tests pass
