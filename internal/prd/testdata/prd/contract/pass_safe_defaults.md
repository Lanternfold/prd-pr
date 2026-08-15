# PRD: Add Library

**Status:** Draft
**Product:** Add Library
**Owner:** Test

# 1. Product Overview

A tiny Go library that adds two integers.

# 2. Goals

- Ship Add
- Cover it with tests

# 3. Non-Goals

- Networking

# 4. Requirements

- REQ-001: The library must expose Add(a, b int) int
- REQ-002: Add(2, 2) must return 4

# 5. Acceptance Criteria

- AC-001: Add(2, 2) returns 4
- AC-002: Add is covered by a unit test

# 6. Testing

- TEST-001: unit test Add
Local run: `go test ./...`

# 7. Phases

## P1: Core

Objective: Implement Add
Dependencies:
Requirements: REQ-001, REQ-002
Acceptance Criteria: AC-001, AC-002
Implementation Tasks:
- write Add
Tests: TEST-001
Definition of Done:
- tests pass
