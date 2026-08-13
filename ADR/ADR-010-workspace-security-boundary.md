# Workspace security boundary

## Status

Accepted

## Context

The orchestrator is highly autonomous inside a product workspace and must not touch unrelated repos, leak secrets, overwrite its own running binary, or take destructive external actions.

## Decision

File I/O is jailed to the configured product root plus orchestrator config/knowledge paths. Git diffs after a worker must stay inside the root. Secrets live in macOS Keychain (CI: Actions Secrets); they are never logged, committed, put in PRs, knowledge, or default task packets. No production deploy, credential rotation/deletion, or financial actions unless explicitly configured. Workers must not overwrite the running `prdpr` binary.

## Consequences

Self-dogfooding uses a branch and a separate build output. Credential UX is Keychain, not `.env` in the repo. Jail bugs are treated as `FAILED` (unrecoverable), not repair incidents.

## Revisit When

Non-macOS secret stores are required (same `SecretStore` interface, new backend), or a product must legitimately write outside the root (then explicit allowlist, not a weaker default).
