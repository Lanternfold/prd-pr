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

The current published release is **v0.1.1**. Install that binary, put `prdpr` on `PATH`, then pass a PRD:

```bash
prdpr version   # 0.1.1
prdpr doctor
prdpr path/to/PRD.md
```

`prdpr <PRD.md>` and `prdpr bootstrap <PRD.md>` are the same PRD-only entry. The engine validates the PRD as part of that command. Do not start by creating a product directory by hand.

Optional diagnostic (no project mutation; not required before the product command):

```bash
prdpr validate-prd path/to/PRD.md
```

## Installation and setup

### Released binary (normal users)

Current release: **[v0.1.1](https://github.com/Lanternfold/prd-pr/releases/tag/v0.1.1)**.

1. Download the `prdpr` artifact that matches your OS and architecture (`prdpr_0.1.1_darwin_arm64`, `prdpr_0.1.1_darwin_amd64`, `prdpr_0.1.1_linux_arm64`, or `prdpr_0.1.1_linux_amd64`).
2. Download `prdpr_0.1.1_checksums.txt` from the same release and verify the artifact.
3. Install the file as `prdpr` on your `PATH` (for example `$HOME/.local/bin/prdpr`) and mark it executable.
4. Run `prdpr version` (expect `0.1.1`) and `prdpr doctor`.

Example (macOS Apple Silicon):

```bash
mkdir -p "$HOME/.local/bin"
# download prdpr_0.1.1_darwin_arm64 and prdpr_0.1.1_checksums.txt from the release
shasum -a 256 -c prdpr_0.1.1_checksums.txt --ignore-missing
install -m 0755 prdpr_0.1.1_darwin_arm64 "$HOME/.local/bin/prdpr"
export PATH="$HOME/.local/bin:$PATH"
prdpr version
```

Add `$HOME/.local/bin` to your shell `PATH` if it is not already there.

### Install from source

Requires Go (version in `go.mod`) and Git. From a clone of this repository:

```bash
go install ./cmd/prdpr
```

`go install` writes `prdpr` to `$GOBIN` if set, otherwise `$GOPATH/bin` (default `$HOME/go/bin`). Add that directory to `PATH` if the shell cannot find `prdpr`. A checkout install (`go install ./cmd/prdpr`) reports `dev`. A tagged module install (`go install github.com/lanternfold/prd-pr/cmd/prdpr@v0.1.1`) and GitHub Release binaries report `0.1.1`.

### Contributor / developer build

Clone, test, and keep a local binary under `dist/` (does not install to `PATH`):

```bash
git clone https://github.com/lanternfold/prd-pr
cd prd-pr
go test ./...
go build -o dist/prdpr ./cmd/prdpr
./dist/prdpr doctor
```

See [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md).

### Check the environment

```bash
prdpr doctor
```

Reports OS, architecture, Go, Git, Cursor editor, Cursor Agent, and GitHub CLI. Missing Git is an error. Missing Cursor or `gh` is informational.

### Prerequisites

| Requirement | Status |
|---|---|
| Go (version in `go.mod`) | Required to install or build from source; not required to run a released binary |
| Git | Required |
| A PRD (contract-validated during `prdpr <PRD.md>`) | Required |
| Studio layout (`Tools/`, `Products/`, …) or `PRDPR_STUDIO` | Required for PRD-only bootstrap |
| Cursor IDE | Required for interactive `/prdpr` |
| `prdpr` on `PATH` | Required for the normal workflow |
| `cursor-agent` | Required only for headless `prdpr run` / `prdpr phase` |
| `gh` + GitHub auth | Optional; needed when GitHub delivery is enabled |

### Git

Use a normal local Git identity (`user.name`, `user.email`). PRD→PR commits as you. It does not rewrite published history and does not force-push.

### GitHub

Default config has `GitHubEnabled=false`. Local commits still happen after verified phases (`AutoCommit` default true). Push is skipped when there is no remote.

When you enable GitHub (today: inject `config.Config` in code/tests; **YAML/env config loading is PARTIAL / not a user CLI**), the engine can create a remote, push, and open a milestone PR. Auto-merge stays off unless you set `AutoMergeEnabled`.

### Cursor plugin (V0)

The plugin is a thin adapter. It requires **released `prdpr` on `PATH`**. It does not require a PRD→PR source checkout and must not use `<workspace>/dist/prdpr`.

See [CURSOR.md](CURSOR.md) and [prdpr-cursor/README.md](../prdpr-cursor/README.md).

1. Install `prdpr` v0.1.1 on `PATH` (above).
2. Copy the `prdpr-cursor/` directory to `~/.cursor/plugins/local/prdpr` (prefer copy over a symlink whose target is outside that folder):

```bash
mkdir -p ~/.cursor/plugins/local
rm -rf ~/.cursor/plugins/local/prdpr
cp -R /path/to/prdpr-cursor ~/.cursor/plugins/local/prdpr
```

3. Reload Cursor if `/prdpr` does not appear. Enable the plugin in **Customize** if needed.
4. Run `/prdpr`.

Marketplace packaging is **PLANNED** (not V0).

### Cursor Auto-review

PRD→PR does **not** control Cursor’s global Run Mode or permission prompts. `.cursor/rules` are behavioral instructions for the model; they do not grant terminal access.

If Auto-review blocks engine commands (`prdpr verify`, tests, Git), approve the PRD→PR / workspace commands in Cursor settings. That is a Cursor product setting, not an orchestrator flag.

### Required configuration

For a first local run:

- `prdpr` on `PATH` (see install above)
- Git
- A discoverable Studio root (`PRDPR_STUDIO` or a parent that looks like Studio)
- A PRD that the engine can accept (it validates during `prdpr <PRD.md>`)

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

There are two supported workflows. Both use the same engine. Do not mix either with headless `prdpr phase` in the same session.

### CLI-first

1. Author `~/Inbox/my-product.md` to the [PRD contract](PRD_AUTHORING_CONTRACT.md).
2. Optionally `prdpr validate-prd ~/Inbox/my-product.md` to inspect the contract without creating a project. If REJECTED, edit the PRD. This step is diagnostic; it is not required.
3. `prdpr ~/Inbox/my-product.md` — engine validates the PRD, selects project type, creates `…/Products/<slug>/` when needed, copies `PRD.md`, writes `.project/`, Cursor rules, Git baseline, optional GitHub, then **prepare** (packet on disk). `prepare` internally runs preflight and may refuse if it is blocking. A separate `inspect` / `preflight` CLI call is optional.
4. Open that product directory in Cursor. `/prdpr` (or `prdpr prepare` if a packet is not already printed) implements **only the packet** in this session.
5. Engine `prdpr verify` runs tests. Worker “done” is ignored.
6. On failure: `review` → maybe `repair` (max 3 product attempts) → verify again.
7. On verified success: engine commits (default). If GitHub is enabled, it pushes and may open a PR. Then the next READY phase is prepared (plugin `prdpr prepare` without `--phase`) or walked headless (`prdpr phase`).
8. When the graph is fully COMPLETED, `prdpr runtime` starts a structured local runtime if the project type has one.

### Cursor-plugin-first

1. Install `prdpr` v0.1.1 on `PATH` and load the plugin ([Cursor plugin](#cursor-plugin-v0)).
2. Open a folder that contains `PRD.md`, or be ready to supply a PRD path.
3. Run `/prdpr`. The plugin resolves `prdpr` on `PATH` only. Optional `validate-prd`, then `prdpr <PRD>` if this workspace is not yet the product.
4. If the engine prints a `product_root` that is not the current workspace, open that directory in Cursor and run `/prdpr` again. Implementation must happen in the product tree.
5. The same session reads the `packet:` JSON on disk, implements only that packet, then `prdpr verify`, with engine-driven review/repair and later `prepare` calls as above.

What you should see: printed `product_root`, `packet:` path, `verified_success: true|false`, and occasionally `waiting_for_human`. You should not see the plugin spawn another Cursor.

## Human intervention

PRD→PR stops and writes `.project/human/request.json` when it cannot proceed safely. Answer with `prdpr feedback` then `prdpr resume`.

Typical stops:

| Situation | What you do |
|---|---|
| Contract REJECTED | Edit the PRD; optionally `validate-prd`, then `prdpr <PRD.md>` again |
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
| Interactive | CLI-first `prdpr <PRD.md>` then `/prdpr`, or plugin-first `/prdpr` | Current Cursor session | Plugin calls `prepare` again after each verified phase |
| Headless | `prdpr phase` | `cursor-agent` subprocess | `Engine.RunGraph` walks READY phases in one process |

Do not call `prdpr run` or `prdpr phase` from the plugin (nested Cursor is forbidden).

## Self-development (this repository)

Ordinary `prdpr prepare` / `run` / `phase` against the PRD→PR orchestrator repository is refused. That guard is unchanged.

To modify `prd-pr` through the engine, all of the following are required:

1. Pass `--self-development` on the execution request (`prepare`, `run`, or `phase`).
2. Include an explicit `Execution mode: SELF_DEVELOPMENT` line in the PRD (a title mention is not enough).
3. Target the current orchestrator checkout (`module github.com/lanternfold/prd-pr` in `go.mod`).

The dedicated path records mode, repository identity, authorization, implementation, and verification in `.project/`. Worker completion is not success; `prdpr verify` must pass before self-development is marked completed.
