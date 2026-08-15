# PRD→PR

PRD→PR is a local engineering orchestrator. You give it a product requirements document. It validates that PRD, creates or locates a project, and drives implementation through a phase graph until the work is verified and delivered.

The Go engine is the source of truth. Cursor is an implementation actor. Cursor saying “done” is not verification.

```text
PRD path
→ validate
→ determine project
→ bootstrap
→ graph
→ implement
→ verify
→ repair
→ deliver
→ runtime
```

## Status

This repository is a working V1. Capabilities are not equally complete. See [docs/PHASES.md](docs/PHASES.md) for P0–P13 status (`IMPLEMENTED`, `PARTIAL`, `PLANNED`, `OUT OF SCOPE`).

Default tests do not call paid LLM APIs or live Cursor.

## Quick start

1. Install Go (see `go.mod`) and Git.
2. Build the engine:

```bash
go build -o dist/prdpr ./cmd/prdpr
export PATH="$PWD/dist:$PATH"
```

3. Write a PRD that can pass `prdpr validate-prd`. See [docs/PRD_AUTHORING_CONTRACT.md](docs/PRD_AUTHORING_CONTRACT.md).
4. Run:

```bash
prdpr path/to/PRD.md
```

That is the intended entry point: a **PRD path**, not a manually prepared product directory. PRD→PR places a Studio project when needed, then prepares the next READY phase.

Interactive Cursor: install the thin plugin in [prdpr-cursor/](prdpr-cursor/README.md) and run `/prdpr`. Headless: `prdpr phase` (uses `cursor-agent`). Do not mix those paths in one session.

Full walkthrough: [docs/USER_GUIDE.md](docs/USER_GUIDE.md).

## Documentation

| Audience | Start here |
|---|---|
| New user | [docs/USER_GUIDE.md](docs/USER_GUIDE.md) |
| CLI commands | [docs/CLI.md](docs/CLI.md) |
| How it actually works | [docs/FLOW.md](docs/FLOW.md) |
| Graph and loops | [docs/GRAPH_AND_LOOPS.md](docs/GRAPH_AND_LOOPS.md) |
| P0–P13 | [docs/PHASES.md](docs/PHASES.md) |
| Cursor | [docs/CURSOR.md](docs/CURSOR.md) |
| Git/GitHub | [docs/GIT_GITHUB.md](docs/GIT_GITHUB.md) |
| LLM vs human vs code | [docs/LLM_AND_HUMAN.md](docs/LLM_AND_HUMAN.md) |
| Contributor / developer | [docs/DEVELOPER_GUIDE.md](docs/DEVELOPER_GUIDE.md) |
| Docs index | [docs/README.md](docs/README.md) |

Design intent and accepted ADRs remain in [ARCHITECTURE.md](ARCHITECTURE.md) and [ADR/](ADR/). That document is not a substitute for the implemented flow in `docs/FLOW.md`.

## Security

Report undisclosed vulnerabilities privately. See [SECURITY.md](SECURITY.md). Do not file public GitHub issues for them.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE). The Go module path is `github.com/lanternfold/prd-pr`.
