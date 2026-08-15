# PRD authoring contract

Longer rules: [PRD_AUTHORING_RULEBOOK.md](../PRD_AUTHORING_RULEBOOK.md). User workflow: [USER_GUIDE.md](USER_GUIDE.md).

This is the contract a **human or PRD author** must satisfy so PRD→PR can autonomously implement a product. It is not a license to invent missing product decisions.

The engine command `prdpr validate-prd <PRD.md>` is the gate. If a required decision is unknown, **ask the human**. Do not guess.

## What the validator asks

Is this PRD sufficiently precise and complete for an autonomous engineering system to safely turn it into a product?

It is a contract validator, not a Markdown quality checker. Studio/project conventions may supply safe defaults (for example, a Go library/CLI when the PRD is clearly that shape). Missing technology versions are warnings when they cannot change product behavior.

## Required product content

Authors should include, or explicitly mark as not applicable:

| Topic | What to decide | If unknown |
|---|---|---|
| Problem / objective | Who it is for and what outcome the product must produce | Ask |
| Users / use cases | Who acts and which journeys are in v1 | Ask |
| Scope / non-scope | What must not be built | Ask; do not let the agent invent adjacent features |
| Functional requirements | Observable `REQ-*` behavior | Ask |
| Acceptance criteria | Verifiable `AC-*` pass/fail conditions | Ask |
| Testing expectations | Commands, unit/integration tests, or explicit manual AC | Ask |
| Phases | Named phases, explicit dependencies, no cycles | Ask |
| Dependencies | External systems that must exist | Ask |
| Runtime / platform | web, iOS, Android, desktop, CLI, library, etc. when it changes implementation | Ask when ambiguous (especially mobile/web) |
| External integrations | Required events, payloads, and failure behavior | Ask |
| Credentials | Named credentials and purpose — never secret values | Ask the human for presence later; do not invent providers |
| Security constraints | Auth, PII, payments, secret storage, transport | Ask; do not invent a threat model |
| Expected outputs | What artifacts exist when done | Ask |
| Local run expectations | How to run/verify locally | Ask |
| Definition of done | When the agent must stop | Ask |

## Rules

- Use stable IDs (`REQ-*`, `AC-*`, `TEST-*`, `P*`) when items exist. Do not invent IDs for missing inventory.
- Requirements must be objectively understandable. Subjective taste is not implementable.
- Do not write conflicting mandates.
- Do not defer material product decisions to “the implementer.”
- Do not put secrets in the PRD.
- Safe omissions: toolchain version, file layout, and other details that Studio already defaults, when they cannot change product behavior.

## Gate

```text
PRD → prdpr validate-prd → VALID → prdpr <PRD.md> bootstrap / prepare / implementation
PRD → prdpr validate-prd → REJECTED → STOP → human updates PRD → validate again
```

`prdpr <PRD.md>` is the PRD-only entry. Studio placement uses `PRDPR_STUDIO` or a discovered Studio layout (`Tools/`, `Products/`, …), not a hardcoded personal path.

No later stage may bypass this gate. The Go engine is authoritative.
