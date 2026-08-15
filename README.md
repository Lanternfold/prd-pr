# PRD→PR

PRD→PR is a local engineering orchestrator. Its **primary input is a PRD**. The normal workflow is:

```bash
prdpr path/to/PRD.md
```

The engine validates that PRD, creates or locates a project, and drives implementation through a phase graph until the work is verified and delivered. You do not need to run a separate validation command first.

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

## Install

Requires [Go](https://go.dev/dl/) (version in `go.mod`) and Git.

**From source (current install path):**

```bash
git clone https://github.com/lanternfold/prd-pr
cd prd-pr
go install ./cmd/prdpr
```

`go install` puts the `prdpr` binary in `$GOBIN` if set, otherwise `$GOPATH/bin`, which defaults to `$HOME/go/bin`. If the shell cannot find `prdpr`, add that directory to `PATH`.

**GitHub Release binaries** are the planned V0 distribution path. They are **not published yet**. Do not expect a release download URL.

**Contributors / developers** can clone and build a local binary instead of installing:

```bash
git clone https://github.com/lanternfold/prd-pr
cd prd-pr
go test ./...
go build -o dist/prdpr ./cmd/prdpr
./dist/prdpr doctor
```

See [docs/DEVELOPER_GUIDE.md](docs/DEVELOPER_GUIDE.md).

## Quick start

1. Install so `prdpr` is on your `PATH` (see above).
2. Check the environment:

```bash
prdpr doctor
```

`doctor` reports OS, architecture, Go, Git, Cursor editor, Cursor Agent, and GitHub CLI. Missing Git is an error. Missing Cursor or `gh` is informational.

3. Write a PRD. See [docs/PRD_AUTHORING_CONTRACT.md](docs/PRD_AUTHORING_CONTRACT.md).
4. Run the product:

```bash
prdpr path/to/PRD.md
```

That is the intended entry point: a **PRD path**. The engine contract-validates the PRD as part of this command, places a Studio project when needed, then prepares the next READY phase.

Optional explicit validation (no project mutation; useful when authoring or debugging a PRD):

```bash
prdpr validate-prd path/to/PRD.md
```

Interactive Cursor: install the thin plugin in [prdpr-cursor/](prdpr-cursor/README.md) and run `/prdpr`. Headless: `prdpr phase` (uses `cursor-agent`). Do not mix those paths in one session.

Full walkthrough: [docs/USER_GUIDE.md](docs/USER_GUIDE.md).

## Documentation

| Audience | Start here |
|---|---|
| New user | [docs/USER_GUIDE.md](docs/USER_GUIDE.md) |
| Contributor / developer | [docs/DEVELOPER_GUIDE.md](docs/DEVELOPER_GUIDE.md) |
| Cursor | [docs/CURSOR.md](docs/CURSOR.md) |
| Git/GitHub | [docs/GIT_GITHUB.md](docs/GIT_GITHUB.md) |
| Security reporting | [SECURITY.md](SECURITY.md) |
| CLI commands | [docs/CLI.md](docs/CLI.md) |
| How it actually works | [docs/FLOW.md](docs/FLOW.md) |
| Docs index | [docs/README.md](docs/README.md) |

Design intent and accepted ADRs remain in [ARCHITECTURE.md](ARCHITECTURE.md) and [ADR/](ADR/). That document is not a substitute for the implemented flow in `docs/FLOW.md`.

## Security

Report undisclosed vulnerabilities privately. See [SECURITY.md](SECURITY.md). Do not file public GitHub issues for them.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE). The Go module path is `github.com/lanternfold/prd-pr`.
