# Developer guide

For people who build, modify, test, and distribute PRD→PR. Grounded in this repository.

Users who only want to run a PRD should read [USER_GUIDE.md](USER_GUIDE.md). Architecture of the running system: [FLOW.md](FLOW.md).

## A. Developer setup

```bash
git clone https://github.com/lanternfold/prd-pr
cd prd-pr
go test ./...
go build -o dist/prdpr ./cmd/prdpr
./dist/prdpr doctor
# source/developer install (reports version "dev"):
# go install ./cmd/prdpr
```

Prerequisites: Go version from `go.mod`, Git. Cursor IDE for plugin work. `gh` only for GitHub-facing changes. `cursor-agent` only for live P4 tests.

Local config: CLI uses `config.Defaults()`. There is no implemented YAML loader. Tests pass `engine.Options{Config: cfg}`.

Git identity must be set for tests that commit.

Cursor: symlink `prdpr-cursor` as in [CURSOR.md](CURSOR.md).

## B. Repository architecture

The Go engine is the orchestration source of truth. `internal/cli` dispatches. `prdpr-cursor` is UX only.

| Package | Responsibility | Primary callers | Owns | Must not own |
|---|---|---|---|---|
| `internal/prd` | Parse + contract | engine, cli | PRD model | Execution |
| `internal/graph` | Phase DAG | engine, inspect | Ready/Blocked | Workers |
| `internal/preflight` | Advisory report | cli, engine | Tool presence | P4 baseline enforcement |
| `internal/packet` | Packet schema | plan, cursor, engine | JSON shape | Scheduling |
| `internal/plan` | Deterministic packet | engine | Phase→packet | LLM planning (not default) |
| `internal/state` | Snapshot, lock, events | engine, cli | `.project` machines | Git objects |
| `internal/fsguard` | Path jail | engine, testeng, vcs | Root confinement | Policy of when to run |
| `internal/vcs` | git/gh adapters | engine | CLI invocations | Verification |
| `internal/proc` | Process groups/timeouts | cursor, apprun | Subprocess lifecycle | Orchestration |
| `internal/cursor` | P4 `Worker` | engine | cursor-agent invoke | Plugin; `VerifiedSuccess` |
| `internal/engine` | Coordinate all loops | cli | Run/prepare/verify/delivery | Domain parsers |
| `internal/testeng` | Independent tests | engine | Pass/fail evidence | Implementation |
| `internal/review` | Review result | engine | Gate recommendation | Applying repairs |
| `internal/modelrouter` | NONE/cheap/strong | review/engine | Routing decision | HTTP to vendors |
| `internal/llm` | `Adapter` | engine, review | Provider I/O | Policy |
| `internal/diagnose` | `Classify` | engine, repair | Origin/class | Fixes |
| `internal/repair` | Incidents, attempts | engine | 3-attempt rule | Infra outages as attempts |
| `internal/human` | request/response files | engine, cli | One-question schema | Secrets |
| `internal/knowledge` | Observation JSON | engine | Files under `.project/knowledge` | Silent global policy |
| `internal/cost` | Ledger JSONL | engine, review | Cost lines | Payments |
| `internal/subagent` | `Decide` | engine prepare | Advisory choice | Spawning agents |
| `internal/ci` | `gh` check watch | engine | Observation | Merge without policy |
| `prdpr-cursor` | `/prdpr` + skill | Cursor user | UX sequence of CLI calls | Engine logic |

Important interfaces: `cursor.Worker`, `llm.Adapter`, `apprun.Starter`, `vcs.Client`/`GHClient` function hooks, `testeng` run plan.

## C. Interactive Cursor architecture

```text
User → Cursor /prdpr → thin plugin → Go engine → packet → current session → prdpr verify
```

Plugin does not spawn Cursor. Plugin is not the scheduler. Engine decides READY. Current session implements. Verification is independent.

## D. Headless architecture

```text
CLI prdpr phase → Go engine RunGraph → cursor-agent → Verify → next READY
```

Use headless for automation and tests (`--worker fake`). Use interactive when a human is in the editor.

## E. Cursor plugin build

Implemented: files under `prdpr-cursor/` (manifest, `commands/prdpr.md`, `skills/prdpr/SKILL.md`). Local symlink install. Binary resolution: `prdpr` on PATH, else workspace `dist/prdpr`.

Testing plugin changes: follow the skill against a fixture product; engine tests cover CLI. No Marketplace package pipeline (**PLANNED**).

## F. Using PRD→PR as a developer

```text
build prdpr
→ doctor
→ author/validate a fixture PRD
→ prdpr <PRD.md>   (needs Studio / PRDPR_STUDIO)
→ interactive: open product_root, /prdpr
→ headless: prdpr phase --worker fake   (in tests) or cursor-agent
→ answer only genuine human requests
→ inspect product Git history and .project/
```

Self-repo is refused as a product workspace unless `AllowSelf` (tests).

## G. How to modify PRD→PR

```text
requirement → owning package → read tests → smallest change → tests → go test ./... → diff → commit → push/PR
```

| Change | Package |
|---|---|
| PRD parsing / contract | `internal/prd` (`contract_test.go`, testdata) |
| Graph scheduling | `internal/graph`, `engine/graphutil.go`, `loop.go` |
| Orchestration | `internal/engine` |
| Cursor worker | `internal/cursor` |
| Verification | `internal/testeng` |
| Diagnosis | `internal/diagnose` |
| Repair | `internal/repair` |
| LLM / routing | `internal/llm`, `internal/modelrouter` |
| Human | `internal/human` |
| Git | `internal/vcs` |
| GitHub/CI | `vcs/github.go`, `engine/delivery.go`, `internal/ci` |
| Cursor UX | `prdpr-cursor` only if the change is how humans invoke the engine |

Do not put scheduling or verification into the plugin.

## H. Extension guide

**Add an LLM provider.** Implement `llm.Adapter`. Inject via `engine.Options{LLM: ...}`. Router stays vendor-free. Tests: `internal/llm/llm_test.go`, review tests with `Static`/`Fail`. Default tests must keep `None`.

**Change PRD validation.** `internal/prd/contract*.go` + `testdata/prd/contract/`. Do not reimplement in the plugin.

**Change graph scheduling.** `graph.Ready` / `Refresh` / `engine.selectRunnablePhase` / `RunGraph`. Do not add parallel execution without an ADR.

**Add a verification strategy.** `internal/testeng` (new runner behind `Run`). Engine must still be the only setter of `verified_success`.

**Add a repair strategy.** `internal/repair` + `engine.PrepareRepair`. Keep MaxAttempts=3 unless ADR-006 changes.

**Change human interaction.** `internal/human` kinds + `engine/human.go` resume routing.

**Change Git/GitHub.** `internal/vcs` + `engine/repo.go` + `delivery.go`. Preserve no-force-push tests (`vcs/invariants_test.go`, `engine/repo_test.go`).

**Modify Cursor behavior.** Worker flags in `internal/cursor`. Interactive wording in `prdpr-cursor`. Not both for the same concern.

**Add a configuration option.** Field on `config.Config` + `Defaults()` + tests. CLI YAML loading is still **PLANNED**; document that if you add file loading.

## I. Testing architecture

| Kind | Where | Live services |
|---|---|---|
| Unit | `*_test.go` next to packages | No |
| Engine / graph | `internal/engine/*_test.go`, `graph/*_test.go` | No |
| Fake worker | `cursor.Fake`, `cursor.Sequence` | No |
| Fake LLM | `llm.None`, `Static`, `Fail` | No |
| Git | real `git` in temp dirs | No network GitHub |
| GitHub | fake `GHClient` / stub `Git` | No |
| Plugin | manual / skill review | No default automated IDE test |
| Dogfood | `engine/dogfood_test.go` | Fixture repo, fake worker |
| Live Cursor | `internal/cursor/live_test.go` | Only if `PRDPR_CURSOR_LIVE=1` |

`go test ./...` must not require paid LLM or live Cursor.

## J. Building and releasing

**Development CI** (`.github/workflows/ci.yml`) stays responsible for `go test ./...`, `go vet ./...`, and `go build ./...` on push/PR. The required job name is `test`. Do not change that job to publish releases.

**Local contributor build:**

```bash
go test ./...
go build -o dist/prdpr ./cmd/prdpr
```

`go install ./cmd/prdpr` is a source/developer install into Go’s bin directory and reports `prdpr version` as `dev`. `go install github.com/lanternfold/prd-pr/cmd/prdpr@v0.1.0` reports `0.1.0` from Go module build info. GitHub Release binaries report `0.1.0` via `-ldflags`.

**Release builds** inject the version at link time:

```bash
go build -ldflags "-X github.com/lanternfold/prd-pr/internal/cli.Version=0.1.0" ./cmd/prdpr
```

The same injection is used by `scripts/build-release.sh`. Artifact names:

- `prdpr_<version>_darwin_arm64`
- `prdpr_<version>_darwin_amd64`
- `prdpr_<version>_linux_arm64`
- `prdpr_<version>_linux_amd64`
- `prdpr_<version>_checksums.txt`

Example for v0.1.0: `prdpr_0.1.0_darwin_arm64`.

**GitHub Releases** are published by `.github/workflows/release.yml` when a version tag matching `v*` is pushed (validated as `vMAJOR.MINOR.PATCH`). The workflow builds those four binaries, writes checksums, and creates a GitHub Release. It uses the default `GITHUB_TOKEN` (`contents: write`). No extra secrets.

The first release version is **v0.1.0**. Do not create or push that tag until the release workflow is reviewed and merged to the default branch. After that:

1. Release only a commit whose `ci` / `test` job is already green.
2. `git tag v0.1.0 <commit>`
3. `git push origin v0.1.0`

Do not document a specific release download URL until that tag exists.

Local check of the release build (does not publish):

```bash
./scripts/build-release.sh 0.1.0
./dist/release/prdpr_0.1.0_$(go env GOOS)_$(go env GOARCH) version
```

**IMPLEMENTED:** tagged GitHub Releases (`.github/workflows/release.yml` on `v*` tags).

**PLANNED:** Homebrew, plugin Marketplace, config file loader.

## K. Open-source user lifecycle

```text
Install PRD→PR → configure Cursor/GitHub if desired → write PRD
→ prdpr <PRD.md> → engine creates/locates project → orchestrates
→ human handles exceptions → verified commits (and PR if GitHub enabled)
```

Users install the binary + optional plugin + Git. PRD→PR creates the product directory, Git repo, packets, and verification. Users do not manually sequence phases unless recovering.

## L. Invariants to preserve

1. Go engine is the orchestration source of truth — otherwise plugin and CLI drift.
2. Cursor plugin remains thin — a second orchestrator cannot share state.
3. Current Cursor session is the interactive implementation actor — matches how people work.
4. Cursor does not spawn another Cursor — nested workers hide verification and double-write the tree.
5. Graph determines phase readiness — LLMs must not pick BLOCKED work.
6. Verification is independent of the implementation actor — ADR-005.
7. LLM is not verification authority.
8. LLM does not bypass deterministic engine policy (repair limits, commit gates).
9. Human is the authority for unresolved product decisions — do not invent requirements.
10. Git/GitHub lifecycle is engine-owned — history stays auditable.
11. No unverified implementation may be committed/pushed.
12. Do not introduce parallel/task-level/dynamic graph orchestration without demonstrated need (ADR-007).
13. P12 remains `NO_SUBAGENT` unless an ADR changes it.
14. Default tests must not require paid services or live Cursor — CI and contributors stay unblocked.
