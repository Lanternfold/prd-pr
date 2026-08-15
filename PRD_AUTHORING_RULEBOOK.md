# PRD Authoring Rulebook

## Purpose

A PRD produced under this rulebook must be sufficiently precise for PRD→PR
to validate it and, after validation, autonomously plan, implement, verify,
run, and deliver the product with minimal human intervention.

The PRD describes **what must be built and why**.

It must not unnecessarily prescribe implementation details when those can be
safely determined by the project architecture or established Studio
conventions.

The PRD must never invent unresolved product decisions.

When a material decision is unknown, ask the human.

---

# 1. Core Principle

A PRD must allow an autonomous engineering system to answer:

1. What are we building?
2. Why are we building it?
3. Who is it for?
4. What must it do?
5. What must it explicitly NOT do?
6. What does success look like?
7. How can success be objectively verified?
8. What are the implementation boundaries?
9. What external systems/dependencies are required?
10. What credentials or human inputs may be required?
11. What phases should the work be divided into?
12. Which phases depend on which?
13. What should exist when implementation is complete?
14. How should the application run locally?
15. What constitutes a completed product?

If any answer requires a material product decision that the author does not
know, STOP and ask the human rather than guessing.

---



# 2. Product Identity

Every PRD should identify:

- Product name
- One-line description
- Product type
- Primary platform
- Intended users
- Problem being solved

Where possible, include:

- Repository identity
- Intended Studio category
- Expected runtime
- External services

Do not invent repository names or external services.

---



# 3. Problem

Describe:

- Current problem
- Who experiences it
- Why existing solutions are insufficient
- Why this product should exist

Avoid vague statements such as:

- "make things easier"
- "make it scalable"
- "make it modern"
- "AI-powered"

unless they are further defined.

---



# 4. Goal

Define measurable product outcomes.

Each major goal should answer:

> What observable outcome proves that this goal has been achieved?

Goals are not implementation tasks.

---



# 5. Users and Use Cases

Identify the important users or actors.

For each important use case specify:

- Actor
- Trigger
- Expected behavior
- Expected result

Include important failure/edge cases when they affect product behavior.

---



# 6. Scope

Explicitly define:

## In Scope

What the product will do.

## Out of Scope

What it deliberately will not do.

Out-of-scope statements are important because they prevent the implementation
agent from expanding the product unnecessarily.

---



# 7. Functional Requirements

Every important product behavior should have a stable requirement ID.

Preferred format:

`REQ-001`

Each requirement should describe:

- Required behavior
- Relevant actor/system
- Important constraints
- Expected outcome

Requirements must be testable or explicitly marked as requiring human/manual
verification.

Avoid combining unrelated behaviors into one requirement.

Bad:

> REQ-001: Build a great, fast, secure app.

Better:

> REQ-001: When the user shares a supported Instagram Reel with the
> application, the application must identify the referenced song when a
> confident match exists.

---



# 8. Acceptance Criteria

Every material requirement should have one or more acceptance criteria.

Preferred format:

`AC-001`

Acceptance criteria should describe observable outcomes.

Good:

> AC-001: Given a Reel containing a recognizable song, the system returns the
> identified track and artist.

Bad:

> AC-001: The song recognition works correctly.

Whenever possible acceptance criteria should be deterministic.

---



# 9. Verification Expectations

Every implementation phase should define what can be automatically verified.

Preferred format:

`TEST-001`

Examples:

- unit tests
- integration tests
- API tests
- build succeeds
- required endpoint responds
- required file exists
- UI behavior can be mechanically checked

Clearly distinguish:

- Automatically verifiable
- Manually verifiable

Do not pretend that subjective UI/product criteria are automatically
verified.

---



# 10. Non-Functional Requirements

Specify only meaningful constraints.

Examples:

- Performance
- Reliability
- Security
- Accessibility
- Privacy
- Resource limits
- Compatibility
- Offline behavior

Avoid arbitrary numbers unless the product actually requires them.

Bad:

> API must respond in exactly 100ms.

Better:

> API response time should remain below 500ms for the defined MVP workload.

If no meaningful constraint exists, do not manufacture one.

---



# 11. Platform and Runtime

The PRD must identify the target platform when it materially affects
implementation.

Examples:

- Web
- iOS
- Android
- macOS
- Windows
- CLI
- Backend service
- Browser extension
- Cursor extension/plugin
- Multi-platform

For applications, describe expected local startup behavior.

Example:

> The application must be runnable locally and expose a browser-accessible
> interface.

If the platform is intentionally delegated to the architecture, say so.

---



# 12. Technology Constraints

Specify technology only when it is a genuine requirement.

Examples:

- Go
- TypeScript
- React
- PostgreSQL
- SQLite
- AWS

If technology is not material to the product requirement, do not invent
technology choices merely to make the PRD look complete.

Established Studio conventions may provide safe defaults.

---



# 13. External Dependencies

Explicitly identify external systems that materially affect implementation.

Examples:

- GitHub
- Spotify
- Instagram
- Apple APIs
- Google APIs
- OpenAI
- AWS
- Payment providers

For each important dependency specify:

- Purpose
- Required behavior
- Expected integration
- Whether it is required for MVP
- Whether credentials are required

Do not assume undocumented API behavior.

---



# 14. Credentials and Human Inputs

Declare credentials/dependencies that may be required.

Examples:

- API key
- OAuth application
- GitHub authentication
- Apple developer account
- AWS credentials

Never put actual secret values in the PRD.

Instead declare:

```text
Credential: SPOTIFY_CLIENT_ID
Required: Yes
Purpose: Spotify API integration
Acquisition: Human-provided / environment / configured provider
```

The orchestrator must request credentials through its human-interaction  
mechanism rather than asking the user to paste secrets into PRD text.

---

# 15. Security and Privacy

Identify meaningful security constraints.

Examples:

- authentication
- authorization
- secret handling
- user data
- PII
- payment data
- file access
- external API permissions

Never instruct the implementation agent to hardcode secrets.

If security behavior is material and unspecified, ask the human.

---

# 16. Data

If the product stores data, define:

- What is stored
- Why it is stored
- Ownership
- Retention expectations
- Important privacy constraints
- Required persistence behavior

Do not prescribe a database unless necessary.

---

# 17. UI / UX

For UI products specify:

- Primary user flows
- Important screens/views
- Core interactions
- States
- Error states
- Loading states
- Empty states
- Responsive/platform expectations

Define the desired visual direction where it is part of the product.

Do not require a specific design tool unless necessary.

The orchestrator may generate project-specific UI guidance separately.

---

# 18. Local Runtime

Every executable product should define how success is recognized when run  
locally.

At minimum identify:

- Expected runtime type
- Expected entry point or startup behavior
- Expected accessible interface
- Basic readiness condition

Example:

> The web application must start locally and expose the main application  
> through a browser-accessible local URL.

Do not prescribe arbitrary shell commands if the runtime can be determined  
from the project structure.

---

# 19. Project Outputs

Define what should exist after successful implementation.

Examples:

- Application
- API
- CLI
- Browser extension
- Mobile application
- Configuration
- Tests
- Documentation

Outputs should be observable.

---

# 20. Definition of Done

The PRD must define completion.

A product is not complete merely because code was written.

Completion may require:

- implementation complete
- automated tests pass
- acceptance criteria satisfied
- application builds
- application starts locally
- required integrations work
- manual acceptance completed
- Git checkpoint created
- PR created
- CI passes

Only include conditions relevant to the product.

---

# 21. Development Phases

Break implementation into logical phases.

Each phase should have:

- Stable phase ID
- Name
- Objective
- Requirements addressed
- Acceptance criteria
- Test expectations
- Expected outputs
- Dependencies

Preferred format:

`P0`, `P1`, `P2`, ...

Dependencies must be explicit.

Do NOT rely on:

- phase numbering
- document order
- lexical order
- assumed implementation sequence

If a phase has no dependency, say so.

---

# 22. Phase Dependencies

Use explicit declarations.

Example:

```
Phase: P2
```

Name: Backend API

Dependencies:

- P1

Do not write:

> P2 obviously depends on P1.

If the dependency matters, declare it.

Do not invent dependencies simply because one phase appears earlier.

---

# 23. Architecture

The PRD may define important architectural constraints.

Use architecture when the decision materially affects:

-  correctness 
-  security 
-  scalability 
-  integration 
-  maintainability 
-  product behavior 

Do not over-specify implementation structure when multiple safe approaches  
 exist.

Architecture can be refined after PRD validation when the PRD explicitly  
 allows implementation choice.

---

# 24. Decisions vs Implementation Choices

Separate:

## Product decisions

Must be resolved by the human/product owner.

Examples:

-  target platform 
-  supported providers 
-  pricing model 
-  required behavior 
-  privacy policy 
-  user-facing workflow 

## Engineering choices

May be decided by the engineering system when safe.

Examples:

-  internal package structure 
-  test organization 
-  helper functions 
-  implementation details 

Never silently convert a missing product decision into an engineering choice.

---

# 25. Ambiguity Rule

If two reasonable interpretations would produce materially different  
 products:

STOP.

Ask the human.

Do not choose one silently.

---

# 26. Contradiction Rule

A PRD must not contain conflicting requirements.

If conflicting requirements are discovered while authoring:

STOP.

Resolve the contradiction with the human before finalizing the PRD.

---

# 27. No Fake Precision

Do not add arbitrary:

-  performance numbers 
-  technology versions 
-  infrastructure choices 
-  scale assumptions 
-  security requirements 
-  test counts 

just to satisfy a template.

Precision must represent an actual product decision or justified engineering  
 constraint.

---

# 28. No Requirement Inflation

Do not create requirements for implementation details that do not matter to  
 the product.

The purpose of the PRD is not to describe every line of code.

---

# 29. Human Questions

When required information is missing, ask concise questions.

Prefer:

> Which platforms should MVP support: Web only, iOS, Android, or all three?

Not:

> Please provide additional clarification regarding platform requirements.

Questions should be actionable.

---

# 30. PRD Quality Gate

Before finalizing a PRD, perform an author-side review.

Confirm:

-  Product objective is clear 
-  Users are identified 
-  Scope is explicit 
-  Requirements are concrete 
-  Requirements have IDs 
-  Acceptance criteria are observable 
-  Test expectations exist 
-  Runtime/platform is known or safely delegated 
-  External dependencies are declared 
-  Credentials are declared without secrets 
-  Security constraints are sufficient 
-  Phases are defined 
-  Phase dependencies are explicit 
-  Expected outputs are defined 
-  Local runtime expectations exist 
-  Definition of Done exists 
-  No material contradictions remain 
-  No material product decisions are silently guessed 

---

# 31. Validator Compatibility

A PRD authored under this rulebook should be designed to pass the  
 PRD→PR Contract Validator.

However:

**Do not weaken the validator merely to make a PRD pass.**

If the validator rejects a legitimate PRD, determine whether:

1.  the PRD is actually incomplete, or 
2.  the validator is incorrectly demanding information. 

Fix the appropriate side.

---

# 32. Final Authoring Principle

The goal is not:

> "Write a detailed document."

The goal is:

> "Remove the material decisions an autonomous engineering system would  
>  otherwise have to guess."

