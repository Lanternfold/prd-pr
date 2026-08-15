# PRD: Storage Conflict

**Status:** Draft
**Product:** Storage Conflict
**Owner:** Test

# 1. Product Overview

A Go library that stores a single note.

# 2. Goals

- Persist one note

# 3. Non-Goals

- Multi-note collections

# 4. Requirements

- REQ-001: All data must be stored only on the device
- REQ-002: All data must be stored only in the cloud

# 5. Acceptance Criteria

- AC-001: Get returns the stored note
- AC-002: storage location matches the product rule

# 6. Testing

- TEST-001: unit test store

# 7. Phases

## P1: Core

Objective: Store a note
Requirements: REQ-001, REQ-002
Acceptance Criteria: AC-001, AC-002
Tests: TEST-001
Definition of Done:
- tests pass
