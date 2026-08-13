# Single local Go binary

## Status

Accepted

## Context

PRD→PR is a personal developer tool on macOS. The PRD forbids a SaaS platform, cloud orchestrator, Kubernetes, and extra infrastructure. The system must resume after crash or terminal close.

## Decision

Ship one local Go CLI (`prdpr`). Integrations are subprocesses and HTTP APIs (`git`, `gh`, Cursor CLI, configured LLM APIs). No daemon is required for V1. Resume is from disk.

## Consequences

Simple operations and a single deployment artifact. Long-running CI watches must either keep the CLI in the foreground or exit and `resume` later. A GUI or background helper would be a later add-on, not a second orchestrator.

## Revisit When

`prdpr run` cannot stay in the foreground and polling-on-resume is proven too coarse (for example CI that outlives the laptop session). Even then prefer a local `launchd` helper over a cloud service.
