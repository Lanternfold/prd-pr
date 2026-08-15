# Single local Go binary

## Status

Accepted

## Context

PRD→PR is a personal developer tool on macOS. The PRD forbids a SaaS platform, cloud orchestrator, Kubernetes, and extra infrastructure. The system must resume after crash or terminal close.

## Decision

Ship one local Go binary (`prdpr`) as the **core engine**. The CLI is the engine’s direct interface (testable without Cursor IDE). Integrations are subprocesses and HTTP APIs (`git`, `gh`, Cursor CLI, configured LLM APIs). No daemon is required for V1. Resume is from disk.

The **primary user-facing interface** is a thin Cursor plugin that invokes this engine (ADR-012). That plugin is not a second orchestrator.

## Consequences

Simple operations and a single engine artifact. Long-running CI watches must either keep the CLI in the foreground or exit and `resume` later. Cursor-native UX is an adapter over the same binary (ADR-012), not a competing runtime.

## Revisit When

`prdpr run` cannot stay in the foreground and polling-on-resume is proven too coarse (for example CI that outlives the laptop session). Even then prefer a local `launchd` helper over a cloud service.
