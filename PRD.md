# PRD: PRD→PR Orchestrator

**Status:** Draft v1  
**Product:** PRD→PR  
**Owner:** Lanternfold  
**Repository:** github.com/lanternfold/prd-pr

---

# 1. Product Overview

PRD→PR is a local-first autonomous software development orchestrator.

It accepts a product PRD and manages the software development lifecycle from planning through implementation, testing, review, repair, human validation and learning.

The goal is not to build an "AI coding agent".

The goal is to build a reliable engineering system that uses AI where reasoning is useful while relying on deterministic systems wherever possible.

The system should allow the human to provide:

- product intent
- constraints
- important decisions
- credentials when required
- real-world validation
- feedback when automation cannot resolve an issue

Everything else should be automated where reasonably reliable.

## Core loop

PRD
→ Preflight
→ Analyze
→ Design / Technical Planning
→ Phase Graph
→ Review Current State
→ Detailed Plan
→ Model / Tool Selection
→ Implement
→ Test
→ Review
→ Quality Gate
→ Artifact
→ Human Validation
→ Learning
→ Next Phase

When something fails:

Failure
→ Diagnose
→ Identify root cause
→ Re-plan / Repair / Rewind
→ Replay affected graph
→ Test
→ Review

Maximum autonomous repair attempts: 3.

After 3 unsuccessful attempts, stop and request human intervention.

---

# 2. Problem

Building software with modern AI coding tools is becoming increasingly fast, but the workflow remains highly manual.

A developer still needs to:

- understand the PRD
- create an implementation plan
- divide work into phases
- decide which model to use
- decide whether parallel agents are useful
- configure tools
- implement features
- run tests
- inspect failures
- determine whether failures originated in the current or previous phase
- repair code
- rerun affected work
- validate the application
- maintain project context
- remember lessons from previous projects

This creates a bottleneck.

The objective of PRD→PR is to turn this into a repeatable engineering loop.

---

# 3. Product Vision

Given a sufficiently complete PRD, the orchestrator should be capable of taking a project from:

```text
Idea / PRD

to:

Working product
+
Git history
+
Pull Requests
+
Tests
+
Artifacts
+
Human validation
+
Project knowledge
+
Reusable engineering lessons

with minimal human intervention.
```

4. Goals
Primary goals
Convert a PRD into an executable engineering plan.
Represent the project as a dependency graph.
Execute phases autonomously.
Review previous work before every phase.
Dynamically select appropriate models and tools.
Generate or adapt UI/UX design artifacts where relevant.
Implement using Cursor as the initial coding worker.
Automatically test implementations.
Independently review implementations.
Diagnose failures.
Repair failures autonomously.
Rewind to earlier phases when the root cause originated upstream.
Replay only affected downstream phases.
Stop after three unsuccessful repair attempts.
Ask humans only when necessary.
Estimate human involvement before execution.
Notify humans when their input is required.
Capture human feedback.
Maintain project knowledge.
Learn reusable lessons from execution.
Track cost and execution time.
Minimize unnecessary agents, MCP servers and infrastructure.
Eventually use the orchestrator to build and improve itself.
5. Non-Goals for V1

PRD→PR V1 will NOT attempt to be:

a public SaaS platform
a multi-tenant service
an AI model provider
an agent marketplace
a general-purpose agent swarm framework
a custom LLM runtime
a Kubernetes-based platform
a cloud-native distributed system
a production deployment platform
a replacement for GitHub
a replacement for Cursor
a replacement for Figma
a general MCP platform

Do not introduce infrastructure merely because it is technically interesting.

6. Initial Environment

PRD→PR is initially a personal developer tool.

Primary environment:

macOS
Apple Silicon
Go
Git
GitHub
GitHub Actions
Cursor CLI
local filesystem
configured LLM APIs where required

PRD→PR itself will live at:

~/Studio/Tools/prd-pr/

Product projects will live separately:

~/Studio/Products/<product>/

Example:

~/Studio/
├── Tools/
│   └── prd-pr/
└── Products/
    ├── ShareProbe/
    └── MusicExtractor/
7. Core Design Principles
7.1 Deterministic before probabilistic

Use deterministic mechanisms wherever possible.

Examples:

Git for checkpoints
tests for correctness
DAG for dependencies
schemas for state
GitHub Actions for CI
filesystem for initial knowledge storage

Use LLMs where reasoning is actually useful.

7.2 Minimum reasonable complexity

Do not introduce:

agents
MCP servers
databases
queues
cloud services
vector databases
microservices

unless there is a demonstrated need.

7.3 Minimum reasonable cost

Prefer:

local execution
cheap models for simple tasks
stronger models only for complex/high-risk tasks
deterministic tools over LLM calls
parallel agents only when useful
7.4 Human attention is expensive

Human intervention should be:

predictable
minimal
specific
time-bounded

Never ask a human to perform something that can reliably be automated.

7.5 Never blindly trust an agent

An implementation agent saying:

"Done."

is not evidence of completion.

The orchestrator must independently verify:

files
Git state
tests
builds
CI
acceptance criteria
7.6 Recovery over repetition

When something fails, don't blindly retry.

Determine:

Where did the failure originate?

Then repair at the appropriate level.

8. High-Level Architecture

Initial architecture should be a single Go application.

Conceptually:

PRD
 │
 ▼
Preflight
 │
 ▼
Project Analyzer
 │
 ▼
Phase Graph
 │
 ├──────────────┐
 ▼              ▼
Design Planner  Technical Planner
 │              │
 └──────┬───────┘
        ▼
Context / Previous Work Review
        │
        ▼
Model / Tool Router
        │
        ▼
Execution Planner
        │
        ▼
Cursor Coding Worker
        │
        ▼
Git
        │
        ▼
GitHub
        │
        ▼
GitHub Actions
        │
        ▼
Test Engine
        │
        ▼
Review Engine
        │
   ┌────┴────┐
   ▼         ▼
 PASS       FAIL
   │         │
   │      Diagnosis
   │         │
   │      Rewind/Repair
   │         │
   │       Replay
   │         │
   └────┬────┘
        ▼
Human Validation
        │
        ▼
Learning Engine
        │
        ▼
Knowledge Base
        │
        ▼
Next Phase
9. Project State Machine

The orchestrator must maintain explicit state.

Required states:

PROJECT_CREATED
PREFLIGHT
ANALYZING
PLANNING
WAITING_FOR_INPUT
READY
IMPLEMENTING
TESTING
REVIEWING
DIAGNOSING
REPAIRING
REWINDING
REPLAYING
PHASE_COMPLETE
HUMAN_VALIDATION
LEARNING
NEXT_PHASE
PROJECT_COMPLETE
BLOCKED
FAILED

State must be persisted.

The system must be able to resume after:

application crash
terminal closure
machine restart
network failure
model failure
CI failure
10. Project State

Each project must maintain structured state.

Example:

project_id:
current_phase:
current_state:
current_commit:
last_known_good_commit:
repair_attempt:
blocked_reason:
human_action_required:
updated_at:

The state must not depend on natural-language logs.

11. Project Directory

The orchestrator should create a project metadata directory.

Suggested structure:

.project/
├── state/
│   ├── state.json
│   └── graph.json
├── phases/
├── checkpoints/
├── execution/
├── reviews/
├── repairs/
├── artifacts/
├── feedback/
└── knowledge/

This directory belongs to the product repository.

12. PRD Contract

The orchestrator consumes a PRD.md.

The PRD should support:

product definition
goals
non-goals
users
user journeys
requirements
acceptance criteria
design
technical stack
architecture
dependencies
credentials
testing
security
phases
human validation
Definition of Done

Requirements must have stable identifiers.

Example:

REQ-001
REQ-002
REQ-003

Acceptance criteria:

AC-001
AC-002

Tests:

TEST-001
TEST-002

This orchestrator’s own PRD currently illustrates that ID format. A complete REQ/AC/TEST inventory for this document is not required yet; product PRDs the orchestrator consumes should use stable identifiers when they exist. The parser must not invent IDs.

13. Requirement Traceability

The system should maintain:

REQ
 ↓
AC
 ↓
Implementation
 ↓
TEST
 ↓
CI
 ↓
Artifact
 ↓
Human Validation

The orchestrator must be able to answer:

"Which tests prove that REQ-004 works?"

and:

"Which requirements are affected by this code change?"

14. Preflight

Before modifying the project, inspect:

OS
architecture
language/runtime
package manager
framework
Git
GitHub
Cursor
required CLIs
Xcode where applicable
Docker/OrbStack where applicable
environment variables
credentials
external services
repository state
current branch
uncommitted changes

Generate a preflight report.

15. Dependency Handling

Dependencies are classified as:

AVAILABLE
MISSING
OPTIONAL
BLOCKING

If a dependency is missing:

continue unrelated work where safe
block only dependent work
ask human only when necessary
16. Credential Handling

Credentials must be requested one at a time.

Rules:

never fabricate credentials
never request credentials unnecessarily early
never log credentials
never commit credentials
never put credentials in PRs
never put credentials in knowledge
never expose credentials to unrelated workers

Preferred storage:

macOS Keychain locally
GitHub Actions Secrets in CI
17. Clarification Engine

If a requirement is materially ambiguous, stop and ask the human.

Examples:

product behavior
architecture
platform
security
external API choice
important UX behavior

Ask one question at a time.

Record:

Question
Reason
Affected requirement
Human answer
Decision
18. Human Intervention Forecast

Before execution starts, estimate:

Expected human interactions
Expected human minutes
Likely timing
Potential conditional interactions
Maximum possible interactions

Example:

Expected:
4 interactions

Estimated human time:
18 minutes

Potential:
2 additional interactions

Maximum:
6

The estimate should update during execution if circumstances change.

19. Human Notification

When blocking human input is required:

immediately notify
display what is required
display why it is required
display estimated time
display what happens after the response

If no response occurs for 30 seconds:

trigger a bell/attention notification

Do not repeatedly ring every 30 seconds.

The notification system must be configurable.

20. Human Validation

When automation reaches a point requiring real-world validation, produce a precise validation task.

Example:

Build:
ShareProbe 0.2

Estimated time:
7 minutes

Tests:
1. Share Instagram Reel
2. Share YouTube Short
3. Share Safari URL
4. Share plain text

Expected behavior:
...

Report:
Pass/fail + screenshots if failure occurs
21. Phase Graph

The orchestrator converts the PRD into a DAG.

The orchestrator must build the execution DAG from explicit dependency declarations on each phase.

It must NOT infer dependencies from:

- phase numbering
- document ordering
- lexical order
- assumptions

If no dependency is declared, the node is considered independent.

A declared dependency means: the dependent phase cannot correctly begin until the listed phase has completed. Related work that is useful but not required to start is a note, not a hard dependency.

Each phase has:

ID
Name
Objective
Dependencies
Inputs
Outputs
Requirements
Acceptance Criteria
Implementation Tasks
Tests
Design Work
Risks
Human Validation
Definition of Done

The graph must support:

dependencies
parallel work
affected-node detection
checkpointing
rewind
replay
22. Phase Review

Before starting every phase:

Review:

PRD
architecture
ADRs
current code
previous phase outputs
previous tests
previous reviews
previous failures
human feedback
knowledge base

The orchestrator must determine:

What is the current state?
What has already been completed?
What is still required?
Are previous assumptions still valid?
What risks exist?
Should the plan change?
23. Design Planning

Design is a first-class artifact.

The orchestrator should determine whether the project needs:

existing UI template
component library
AI-generated design
Figma
code-first design
combination

Figma must not be mandatory.

Design outputs may include:

DESIGN.md
design tokens
screen definitions
component definitions
UX flows
assets

Design planning may execute in parallel with technical planning.

Design and technical plans must converge before implementation of dependent work.

24. Design Review

Design review should verify:

requirements represented in UX
complete flows
loading states
empty states
error states
permission states
accessibility
platform conventions
consistency

Where practical, implementation should later be visually reviewed using screenshots or equivalent artifacts.

25. Model Router

Model selection is dynamic.

Inputs:

task
complexity
risk
context size
reasoning requirement
latency
budget
historical performance

Possible task types:

planning
coding
review
architecture
diagnosis
test generation
learning

The router should select the cheapest model capable of reliably performing the task.

Track actual results.

26. Subagent Router

Subagents are optional.

For each potential parallel task determine:

independence
expected benefit
cost
coordination overhead
risk

Then decide:

USE_SUBAGENT

or:

NO_SUBAGENT

Record the outcome for future learning.

27. Implementation Worker

Cursor is the initial coding worker.

The orchestrator creates a task packet containing:

phase
objective
requirements
acceptance criteria
architecture
design
relevant previous decisions
tests
constraints
Definition of Done

The coding worker should operate within the configured workspace.

The orchestrator captures:

commands
output
changed files
tests
commit
duration
failures
model
estimated cost
28. Autonomous Permissions

Default behavior should be highly autonomous inside the configured project workspace.

Automatically allowed:

file reads
file writes
builds
tests
dependency installation
Git branches
commits
pushes
PR creation
CI execution
artifact generation
configured API usage
configured model usage
autonomous repairs
knowledge recording

Must remain restricted:

arbitrary filesystem access
unrelated repositories
secret exposure
destructive external actions
financial actions
production deployment unless explicitly configured
credential deletion/rotation
29. Testing Engine

The orchestrator must determine appropriate tests.

Potential layers:

Unit
Integration
API
Database
UI
E2E
Regression
Accessibility
Performance
Security
Static Analysis
Lint
Type Checking
Build
Packaging

Deterministic tests are preferred.

LLMs supplement testing rather than replacing it.

30. Adversarial Testing

Where useful, generate tests for:

malformed inputs
empty inputs
unexpected inputs
extreme inputs
network failures
timeouts
concurrency
duplicate actions
dependency failures
permission failures
external API changes
corrupted state

Avoid unnecessary test generation cost.

31. Regression Testing

Every discovered bug should produce:

Bug
 ↓
Root Cause
 ↓
Fix
 ↓
Regression Test
 ↓
Verification

unless a documented reason prevents it.

32. Review Engine

Every phase must be independently reviewed.

Review may include:

requirements
implementation
architecture
tests
regression
security
design

Use deterministic validation first.

Use LLM review when useful.

The implementation worker must not be the only reviewer.

33. Quality Gate

A phase cannot complete until required gates pass.

Potential gates:

Requirements       ✓
Build              ✓
Tests              ✓
Regression         ✓
Review             ✓
Security           ✓
Design             ✓
Artifact           ✓
Known Issues       ✓

Only applicable gates should be enabled for a particular phase.

34. Failure Diagnosis

When a task fails, classify the likely origin:

PRD
Requirement
Planning
Architecture
Implementation
Testing
Environment
Dependency
External Service
Previous Phase

The diagnosis should return:

root_cause
confidence
affected_phase
recommended_action
evidence
35. Self-Fixing Loop

The system must support:

Failure
 ↓
Diagnose
 ↓
Determine origin
 ↓
Re-plan OR Repair OR Rewind
 ↓
Execute
 ↓
Test
 ↓
Review

If the root cause is earlier:

P2
 ↓
Repair
 ↓
Review P2
 ↓
Replay P3
 ↓
Replay P4
 ↓
Replay P5

Only affected downstream nodes should be replayed.

36. Three Attempt Rule

Maximum autonomous repair attempts:

3

Attempt 1:

local diagnosis + repair

Attempt 2:

deeper diagnosis + broader review

Attempt 3:

independent review + stronger reasoning if justified

After three failures:

STOP
→ Human Intervention

No additional autonomous attempts.

37. Human Debugging Report

When escalation occurs, provide:

problem
failure evidence
suspected root cause
confidence
phases inspected
attempts made
why each attempt failed
relevant logs
relevant changes
required human information
recommended next action

The system should then remain paused.

38. Checkpoints

Every completed phase creates a checkpoint containing:

commit SHA
phase
plan
requirements
tests
review results
artifact
state
model information
tool information
cost
knowledge generated

Git should be the primary source of code checkpoints.

39. Rewind and Replay

The orchestrator must determine:

If this phase changes, which phases are affected?

Use the DAG.

Example:

P2
├── P3
│   └── P5
└── P4
    └── P5

If P2 changes:

Replay:
P2 → P3 → P4 → P5

Unrelated branches should not be rerun.

40. Learning Engine

At the end of every phase:

Perform a Learning Review.

Questions:

What surprised us?
What failed?
Why?
Which assumptions were wrong?
What should become a test?
What should become a reusable lesson?
Was the selected model appropriate?
Was a subagent useful?
Could human intervention have been avoided?
Did human information arrive too late?
Did implementation diverge from architecture?
Did the phase cost more than expected?
41. Knowledge Base

Start with local version-controlled files.

Suggested structure:

knowledge/
├── observations/
├── lessons/
├── patterns/
├── policies/
├── model-performance/
├── tool-performance/
└── human-preferences/

Do not introduce a vector database in V1.

Use normal files and search.

42. Knowledge Promotion

Use:

Observation
→ Candidate Lesson
→ Validated Lesson
→ Policy

Do not silently modify global orchestrator behavior from a single observation.

Important global policy changes should require human confirmation unless explicitly configured otherwise.

43. Cost Tracking

Track:

LLM cost
model
tokens where available
CI cost
external API cost
subagent cost
execution duration

Every phase should have an estimated cost.

The system should provide:

Estimated cost
Actual cost
Budget remaining

If configured budgets are likely to be exceeded, pause and inform the human.

44. Human Time Tracking

Track:

Expected human time
Actual human time
Number of interactions
Reason for interaction

The system should learn how to reduce human interaction over time.

45. Project Completion

A project is complete when:

all required phases pass
requirements are satisfied
acceptance criteria pass
tests pass
regression tests pass
required reviews pass
artifact exists
human validation completes where required
known limitations are documented
learning review completes

Generate:

PROJECT_REPORT.md

Containing:

phases
requirements
tests
failures
repairs
rewinds
models
subagents
cost
duration
human interventions
lessons
known limitations
46. CLI

Initial commands:

prdpr init
prdpr inspect PRD.md
prdpr run PRD.md
prdpr status
prdpr pause
prdpr resume
prdpr review
prdpr retry
prdpr feedback
prdpr logs
prdpr phase
prdpr knowledge
prdpr learn
prdpr doctor

The primary workflow should eventually be:

prdpr run PRD.md
47. Status Output

The CLI should provide a concise live status.

Example:

PRD→PR
────────────────────────────

Project: ShareProbe
Phase: P2 / 6

Progress: 62%

Current:
Independent Review

Tests:
143 / 143

Repair:
0 / 3

Estimated cost:
₹4.80

Actual cost:
₹3.90

Human:
Not required

Next:
Phase Gate
48. GitHub Integration

GitHub is the initial source of truth for:

repository
branches
commits
PRs
CI
checks
artifacts

The orchestrator must be able to:

create branch
implement
commit
push
create PR
monitor CI
retrieve failures
retrieve artifacts
review results
49. CI

GitHub Actions is the initial CI system.

The orchestrator should consume:

workflow status
job status
logs
test results
artifacts

CI failures should feed directly into the diagnosis engine.

50. MCP

MCP is NOT required for V1.

Prefer direct integrations.

Introduce MCP only when a real need appears.

Avoid creating multiple MCP servers unnecessarily.

51. Infrastructure

V1 should run primarily on the user's Mac.

Initial architecture:

Mac
├── PRD→PR
├── Cursor
├── Git
└── local project
       │
       ▼
    GitHub
       │
       ▼
GitHub Actions

No cloud orchestrator is required for V1.

52. Initial Development Phases

PRD→PR itself should be built in phases.

These Dependencies are hard build-order constraints for constructing the orchestrator. They are not inferred from phase numbers. Independent phases may proceed in parallel even when V1 still executes one phase at a time.

P0: Skeleton

Dependencies:
- None

Build:

Go project
CLI
configuration
logging
state persistence
project initialization
doctor command

Definition of Done:

The application can initialize a project and persist/resume state.

P1: PRD Parser

Dependencies:
- P0

Build:

PRD parser
requirement extraction
acceptance criteria extraction
phase extraction
dependency extraction
credential extraction
ambiguity detection

Definition of Done:

A PRD can be converted into structured project metadata.

P2: Graph Engine

Dependencies:
- P1

Build:

DAG
dependency resolution
parallel candidate detection
affected-node calculation
checkpoints
replay planning

Definition of Done:

A project can be represented and traversed as a dependency graph.

P3: Preflight

Dependencies:
- P0
- P1

Note: Graph algorithms (P2) are not required to inspect the environment. Declared PRD dependencies need the parser.

Build:

environment inspection
tool detection
dependency detection
Git detection
GitHub detection
Cursor detection
credential detection

Definition of Done:

The system can determine whether a project is ready to execute.

P4: Cursor Worker

Dependencies:
- P0
- P1

Note: Runtime execution must not modify product code without a Git checkpoint and should refuse to start when preflight is blocking. Those are run-time gates, not build dependencies of the Cursor adapter.

Build:

task packet generation
Cursor invocation
output capture
timeout
process handling
execution result parsing

Definition of Done:

The orchestrator can autonomously give Cursor a task and capture the result.

P5: Git/GitHub

Dependencies:
- P0

Note: Local Git and GitHub integration can be built without the parser, graph, or Cursor worker.

Build:

branches
commits
pushes
PR creation
PR state

Definition of Done:

A phase can result in a GitHub PR.

P6: CI Integration

Dependencies:
- P5

Build:

workflow triggering
status monitoring
failure retrieval
logs
artifacts

Definition of Done:

CI results can drive the orchestration state.

P7: Test Engine

Dependencies:
- P0
- P1

Note: Independent verification must not require the Cursor worker. Requirement mapping needs the parser.

Build:

test discovery
execution
result parsing
requirement mapping
regression tracking

Definition of Done:

The orchestrator can independently verify implementation.

P8: Review + Model Router

Dependencies:
- P0
- P1

Note: In a product run, review follows tests. The review and router modules can be built without the test engine.

Build:

implementation review
architecture review
requirement review
risk classification
model selection
cost tracking

Definition of Done:

The orchestrator can independently assess completed work and choose appropriate models.

P9: Self-Fix

Dependencies:
- P2
- P4
- P7

Note: Rewind/replay need the graph. Repair needs a coding worker and independent re-test. Review (P8) and CI (P6) supply extra evidence but are not required to begin this phase.

Build:

failure classification
diagnosis
root-cause detection
repair
rewind
replay
three-attempt limit

Definition of Done:

A deliberately broken project can be diagnosed, repaired and replayed autonomously.

P10: Human Interaction

Dependencies:
- P0

Note: Human pause/resume is used when self-fix blocks, but the interaction module can be built independently of diagnosis.

Build:

human questions
credential requests
blocking states
validation requests
notifications
feedback
resume

Definition of Done:

The orchestrator can stop and resume around human input.

P11: Learning

Dependencies:
- P0

Note: Learning is richer after full execution history exists; it does not require human workflow or self-fix in order to start.

Build:

observations
lessons
knowledge storage
phase retrospective
model performance
tool performance
policy proposals

Definition of Done:

Execution produces reusable knowledge.

P12: Subagent / Optimization

Dependencies:
- P2

Build:

parallel task evaluation
subagent recommendation
cost/benefit calculation
model optimization

Definition of Done:

The orchestrator can determine when additional agents are worth using.

P13: Self-Dogfooding

Dependencies:
- P1
- P2
- P3
- P4
- P5
- P7
- P8
- P9

PRD→PR uses itself to implement subsequent improvements.

Definition of Done:

PRD→PR can successfully:

Create PRD
→ plan
→ implement
→ test
→ review
→ repair
→ create PR
→ validate
→ learn
→ continue
53. Benchmark Projects

Before relying on PRD→PR for major projects, validate it against progressively harder projects.

Benchmark sequence:

Simple CLI
Small backend
Web application
iOS application
Share Extension
AI-integrated application

Track:

completion rate
human interventions
human minutes
autonomous repair success
cost
duration
regression rate
test quality
model efficiency

These benchmarks become regression tests for PRD→PR itself.

54. Documentation

Maintain:

README.md
ARCHITECTURE.md
STATUS.md
CHANGELOG.md
ADR/

Use ADRs for significant architectural decisions involving:

state management
execution model
model routing
tool integration
recovery
security
persistence
infrastructure
55. V1 Success Criteria

V1 is successful when, given a small product PRD, PRD→PR can autonomously perform:

PRD
 ↓
Preflight
 ↓
Planning
 ↓
Phase creation
 ↓
Cursor execution
 ↓
Git commit
 ↓
PR
 ↓
CI
 ↓
Testing
 ↓
Review
 ↓
Failure diagnosis
 ↓
Autonomous repair
 ↓
Artifact
 ↓
Human validation request
 ↓
Learning
 ↓
Next phase

with minimal human intervention.

56. Ultimate Success Criteria

The long-term goal is:

Give PRD→PR a sufficiently complete PRD, then spend most of the development time away from the keyboard while PRD→PR plans, builds, tests, reviews, repairs, validates and learns.

Human involvement should increasingly become:

Intent
+
Important decisions
+
Credentials
+
Real-world validation
+
Exceptional debugging

rather than:

Run command
Approve command
Run test
Fix error
Tell agent what to do
Repeat
57. Final Principle

PRD→PR should not attempt to appear autonomous.

It should be reliably autonomous.

When it knows what to do:

execute.

When it can verify:

verify.

When something fails:

diagnose.

When the root cause is upstream:

rewind.

When a fix works:

replay affected work.

When it learns something reusable:

record it.

When an important decision belongs to the human:

ask one precise question.

When three autonomous repair attempts fail:

stop.

The system should optimize for:

**Maximum reliable autonomy

Minimum human intervention
Minimum reasonable cost.**

### After pasting it

Your directory should now be:

```text
~/Studio/Tools/prd-pr/
├── go.mod
└── PRD.md
```