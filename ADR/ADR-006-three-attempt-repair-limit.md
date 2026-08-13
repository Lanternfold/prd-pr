# Three-attempt repair limit

## Status

Accepted

## Context

The PRD caps autonomous repair at 3, then human intervention. A project-global counter would freeze the run after three unrelated failures. Infrastructure outages are not code defects.

## Decision

The limit is **3 attempts per failure incident / repair chain**. Unrelated incidents have separate counters. Attempt *n+1* must use evidence from attempt *n*. After three unsuccessful **product** attempts: STOP, write a human debugging report, enter `BLOCKED`. A human may explicitly reset or reopen the incident. No silent fourth attempt.

**Infrastructure failures do not consume attempts:** GitHub/Actions outage, LLM API unavailable, network failure, rate limiting, temporary Cursor service failure, machine sleep. Those use retry/backoff/resume.

## Consequences

Repair state must be incident-scoped, not a single project integer. Diagnosis must classify `product` vs `infrastructure`. Humans must opt in to more attempts.

## Revisit When

Measured data shows three is systematically too few or too many for a class of incidents, or classification of “infrastructure” is too noisy. Change the policy; keep per-incident accounting.
