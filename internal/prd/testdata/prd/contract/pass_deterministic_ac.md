# PRD: Greeting

**Status:** Draft
**Product:** Greeting
**Owner:** Test

# 1. Product Overview

A Go function Hello() string used by a local CLI.

# 2. Goals

- Deterministic greeting

# 3. Non-Goals

- Localization

# 4. Requirements

- REQ-001: Hello must return exactly hello

# 5. Acceptance Criteria

- AC-001: Hello() returns hello
- AC-002: a unit test fails if Hello returns any other string

# 6. Testing

- TEST-001: unit test Hello

# 7. Phases

## P1: Core

Objective: Implement Hello
Requirements: REQ-001
Acceptance Criteria: AC-001
Tests: TEST-001
Definition of Done:
- go test proves Hello returns hello
