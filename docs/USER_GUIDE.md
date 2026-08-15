# User guide

For someone who has never used PRD→PR and wants to turn a PRD into a working product.

Related: [CLI.md](CLI.md), [CURSOR.md](CURSOR.md), [FLOW.md](FLOW.md).

## What PRD→PR is

PRD→PR is a **local orchestrator**, not a chatbot and not an agent swarm.

You supply a PRD. The engine decides whether that PRD is precise enough to implement, where the project should live, which phase is READY, and whether implementation actually passed. Cursor (interactive session or headless `cursor-agent`) writes code. The engine verifies independently.

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

The human is an **exception handler**: unresolved product decisions, missing credentials, unsafe operations, exhausted repair, GitHub auth failures. You should not operate the loop step by step unless you are debugging.

## Quick start

**v0.1.0 is not published yet.** Until a GitHub Release exists, install from a clone. Released binaries will become the normal-user installation path once v0.1.0 exists.

```bash
git clone https://github.com/lanternfold/prd-pr
cd prd-pr
go install ./cmd/prdpr
prdpr doctor
prdpr path/to/PRD.md
```

`go install` puts `prdpr` in your Go bin directory (`GOBIN` or `$(go env GOPATH)/bin`). Add that directory to `PATH` if `prdpr` is not found.

`prdpr <PRD.md>` and `prdpr bootstrap <PRD.md>` are the same PRD-only entry. Do not start by creating a product directory by hand.

`prdpr validate-prd path/to/PRD.md` is an optional diagnostic/preflight if you want to check the PRD contract before bootstrap.

## Installation and setup

### Released binary (normal users)

The first versioned release will be **v0.1.0**. It is not published yet.

After a GitHub Release exists, download the `prdpr` artifact that matches your OS and architecture, verify it against the release checksums file, and put the binary on your `PATH`. Then run `prdpr version` and `prdpr doctor`.

Until that release exists, use the clone + `go install ./cmd/prdpr` path in Quick Start.

### Git checkout (contributors)

```bash
git clone https://github.com/lanternfold/prd-pr
cd prd-pr
go test ./...
go build -o dist/prdpr ./cmd/prdpr
export PATH="$PWD/dist:$PATH"
prdpr doctor
```

A checkout build reports version `dev` unless you inject a version at link time.

### Source install with Go

From a clone of this repository:

```bash
go install ./cmd/prdpr
```

This is a **source/developer** installation path. It puts `prdpr` in your Go bin directory (`GOBIN` or `$(go env GOPATH)/bin`). The binary reports `dev` unless you pass `-ldflags`. Use a released binary for normal use once one exists.

### Prerequisites

| Requirement | Status |
|---|---|
| Git | Required |
| A PRD that can pass contract validation | Required |
| Studio layout (`Tools/`, `Products/`, …) or `PRDPR_STUDIO` | Required for PRD-only bootstrap |
| Cursor IDE | Required for interactive `/prdpr` |
| `prdpr` on `PATH` (or `dist/prdpr` in this repo) | Required |
| Go (version in `go.mod`) | Required to build from source; not required to run a released binary |
| `cursor-agent` | Required only for headless `prdpr run` / `prdpr phase` |
| `gh` + GitHub auth | Optional; needed when GitHub delivery is enabled |

### Git

Use a normal local Git identity (`user.name`, `user.email`). PRD→PR commits as you. It does not rewrite published history and does not force-push.

### GitHub

Default config has `GitHubEnabled=false`. Local commits still happen after verified phases (`AutoCommit` default true). Push is skipped when there is no remote.

When you enable GitHub (today: inject `config.Config` in code/tests; **YAML/env config loading is PARTIAL / not a user CLI**), the engine can create a remote, push, and open a milestone PR. Auto-merge stays off unless you set `AutoMergeEnabled`.

### Cursor plugin

See [CURSOR.md](CURSOR.md) and [prdpr-cursor/README.md](../prdpr-cursor/README.md).

```bash
mkdir -p ~/.cursor/plugins/local
ln -sfn /absolute/path/to/prd-pr/prdpr-cursor ~/.cursor/plugins/local/prdpr
```

Reload Cursor. Marketplace packaging is **PLANNED**.

### Cursor Auto-review

PRD→PR does **not** control Cursor’s global Run Mode or permission prompts. `.cursor/rules` are behavioral instructions for the model; they do not grant terminal access.

If Auto-review blocks engine commands (`prdpr verify`, tests, Git), approve the PRD→PR / workspace commands in Cursor settings. That is a Cursor product setting, not an orchestrator flag.

### Required configuration

For a first local run:

- `prdpr` on `PATH`
- Git
- A discoverable Studio root (`PRDPR_STUDIO` or a parent that looks like Studio)
- A VALID PRD

### Optional configuration

Defaults live in `internal/config/config.go` (`config.Defaults()`). Important fields:

| Field | Default | Meaning |
|---|---|---|
| `HumanTimeout` | 30s | One attention notify; process may exit; state remains on disk |
| `MaxRepairAttempts` | 3 | Product repair attempts per incident |
| `CostBudgetUSD` | 0 | Budget remaining; breach policy default `ask_human` |
| `GitHubEnabled` | false | Remote GitHub lifecycle |
| `GitHubVisibility` | private | New repos are private unless set public |
| `AutoCommit` | true | Commit after verified success |
| `AutoPush` | true | Push after commit when a remote exists / GitHub is enabled |
| `PRBoundary` | `run` | Milestone PR when GitHub is enabled (`run`, `phase`, `never`) |
| `AutoMergeEnabled` | false | Merge only when explicitly enabled and gates pass |
| `UseFeatureBranch` | false | Forced feature branch; also implied when GitHub + `pr_boundary != never` |
| `RulesetsEnabled` | true | GitHub rulesets when GitHub is enabled |
| `AllowForcePush` | false | Engine must not force-push |

There is **no implemented** `prdpr config` command and **no implemented** load of `~/.config/prd-pr/config.yaml` in the CLI. Architecture still describes that search order as intent.

Environment that **is** implemented:

- `PRDPR_STUDIO` — Studio root containing category directories
- `PRDPR_CURSOR_LIVE=1` — opt-in live Cursor worker test (developers)

## First run

Example, interactive:

1. Author `~/Inbox/my-product.md` to the [PRD contract](PRD_AUTHORING_CONTRACT.md).
2. Optionally `prdpr validate-prd ~/Inbox/my-product.md` — if REJECTED, edit the PRD. Nothing else runs. Bootstrap also runs the contract gate.
3. `prdpr ~/Inbox/my-product.md` — engine selects project type, creates `…/Products/<slug>/` when needed, copies `PRD.md`, writes `.project/`, Cursor rules, Git baseline, optional GitHub, then **prepare** (packet on disk).
4. Open that product directory in Cursor. `/prdpr` (or continue in CLI) implements **only the packet**.
5. Engine `prdpr verify` runs tests. Worker “done” is ignored.
6. On failure: `review` → maybe `repair` (max 3 product attempts) → verify again.
7. On verified success: engine commits (default). If GitHub is enabled, it pushes and may open a PR. Then the next READY phase is prepared (plugin) or walked (`prdpr phase`).
8. When the graph is fully COMPLETED, `prdpr runtime` starts a structured local runtime if the project type has one.

What you should see: printed `product_root`, `packet:` path, `verified_success: true|false`, and occasionally `waiting_for_human`. You should not see the plugin spawn another Cursor.

## Human intervention

PRD→PR stops and writes `.project/human/request.json` when it cannot proceed safely. Answer with `prdpr feedback` then `prdpr resume`.

Typical stops:

| Situation | What you do |
|---|---|
| Contract REJECTED | Edit the PRD; run `validate-prd` again |
| Ambiguous platform / Studio placement | Answer the one question; do not invent a stack |
| Completeness `BLOCKING_QUESTION` (only if an LLM adapter is configured) | Put the decision in the PRD; re-validate |
| Missing credential | Confirm **presence** of one named secret; do not paste the value into logs |
| Manual acceptance criteria | Confirm the AC; engine does not pretend tests covered it |
| Unsafe / blocked dependency | Decide or supply the missing system |
| Repair exhausted (3 product attempts) | Debug; the engine will not start a fourth autonomous attempt |
| GitHub auth / ruleset / push failure | Fix `gh auth` or access; `prdpr resume` |
| Runtime start failure after exhaustion | Inspect the app; follow the runtime repair packet or stop |

One question at a time. Optional 30s notification, then the process may exit. Disk state is enough to resume.

## Recovery

| Event | Behavior |
|---|---|
| Cursor stops / you close the chat | `.project/` remains. Re-run `/prdpr` or `prdpr prepare` / `verify` / `status`. Do not start a second Cursor worker. |
| Process crash | Lock + `state.json` + events. Next command reconciles. Crash during `VERIFIED` retries delivery. |
| Verification fails | Independent evidence in `.project/`. Review/repair or human. |
| Repair exhausted | `WAITING_FOR_HUMAN`; no silent fourth attempt. Infrastructure outages do not consume attempts. |
| Human input required | `feedback` then `resume`. Resume continues the owning path (verify, prepare, runtime, or GitHub reconcile). **PARTIAL:** not every human kind auto-walks the outer graph. |
| GitHub unavailable | Local Git is preserved. With GitHub enabled, push/PR failure becomes a structured human request. With GitHub disabled, push is skipped or recorded failed without blocking local completion. |

`prdpr status [directory]` shows persisted state.

## Configuration

See the table above. Runtime defaults are `config.Defaults()`. CLI commands that talk to GitHub (`pr`, `checks`, `merge`) currently force `GitHubEnabled=true` for that invocation.

## Two execution paths

| Path | Entry | Implementation actor | Graph walk |
|---|---|---|---|
| Interactive | `/prdpr` or `prdpr <PRD.md>` then implement in this session | Current Cursor session | Plugin calls `prepare` again after each verified phase |
| Headless | `prdpr phase` | `cursor-agent` subprocess | `Engine.RunGraph` walks READY phases in one process |

Do not call `prdpr run` or `prdpr phase` from the plugin (nested Cursor is forbidden).
