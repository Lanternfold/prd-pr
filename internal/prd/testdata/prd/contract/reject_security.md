# PRD: Login CLI

**Status:** Draft
**Product:** Login CLI
**Owner:** Test

# 1. Product Overview

A Go CLI where users can log in with passwords.

# 2. Goals

- Authenticate a local user

# 3. Non-Goals

- Social login

# 4. Requirements

- REQ-001: Users can log in with passwords
- REQ-002: After login the CLI must print authenticated

# 5. Acceptance Criteria

- AC-001: a correct password prints authenticated
- AC-002: a wrong password prints rejected

# 6. Testing

- TEST-001: login success and failure

# 7. Phases

## P1: Core

Objective: Password login
Requirements: REQ-001, REQ-002
Acceptance Criteria: AC-001, AC-002
Tests: TEST-001
Definition of Done:
- tests pass
