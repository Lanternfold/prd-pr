# PRD: Notes Library

**Status:** Draft
**Product:** Notes Library
**Owner:** Test

# 1. Product Overview

A local Go library that stores notes in memory and returns them by ID. Users are developers calling the library from tests and small CLIs.

# 2. Goals

- Ship NoteStore with Get and Put
- Cover behavior with unit tests

# 3. Non-Goals

- Network sync
- GUI
- Multi-user access

# 4. Users

- Developers embedding the library

# 5. Requirements

- REQ-001: Put(id, body string) must store the body for that id
- REQ-002: Get(id string) must return the stored body or empty if missing
- REQ-003: Get after Put of the same id must return the last Put body

# 6. Acceptance Criteria

- AC-001: Put("a", "hello") then Get("a") returns hello
- AC-002: Get("missing") returns empty string
- AC-003: Put then Put then Get returns the second body

# 7. Testing

- TEST-001: unit tests for Put/Get
Local run: `go test ./...`

# 8. Phases

## P1: Core

Objective: Implement NoteStore
Dependencies:
Requirements: REQ-001, REQ-002, REQ-003
Acceptance Criteria: AC-001, AC-002, AC-003
Implementation Tasks:
- write NoteStore
Tests: TEST-001
Definition of Done:
- go test ./... passes
- Get/Put behavior matches the acceptance criteria

# 9. Security

No authentication, network, or PII. Notes are caller-provided test strings only.

# 10. Definition of Done

Library builds, unit tests pass, and Put/Get behavior matches the acceptance criteria.
