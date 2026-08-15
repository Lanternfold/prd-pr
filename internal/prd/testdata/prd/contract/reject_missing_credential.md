# PRD: Summarizer

**Status:** Draft
**Product:** Summarizer
**Owner:** Test

# 1. Product Overview

A Go CLI that summarizes text using OpenAI.

# 2. Goals

- Return a summary string

# 3. Non-Goals

- Chat UI

# 4. Dependencies

- openai

# 5. Requirements

- REQ-001: When the user submits text, the product must send the prompt to OpenAI chat completions and return the assistant text
- REQ-002: On failure the CLI must print an error and exit non-zero

# 6. Acceptance Criteria

- AC-001: a successful call prints the assistant text
- AC-002: a failed call exits non-zero

# 7. Testing

- TEST-001: fake OpenAI transport

# 8. Phases

## P1: Core

Objective: Summarize
Requirements: REQ-001, REQ-002
Acceptance Criteria: AC-001, AC-002
Tests: TEST-001
Definition of Done:
- tests pass
