# CLI reference

User-facing commands implemented in `internal/cli`. This list is the source of truth; do not invent commands.

Repository docs: https://github.com/lanternfold/prd-pr/blob/main/docs

Related: [USER_GUIDE.md](USER_GUIDE.md), [FLOW.md](FLOW.md), [CURSOR.md](CURSOR.md).

`prdpr --help` and `prdpr <command> --help` point here.

---

<a id="prd-path"></a>

## `prdpr <path/to/PRD.md>`

Same as `prdpr bootstrap`.

**What it does:** Contract-validates the PRD, selects Studio type/location, places the project, bootstraps Git (and GitHub when enabled), then prepares the next READY phase. Does not invoke a coding worker.

**When to use it:** First entry from a PRD path.

**What happens next:** Open `product_root` in Cursor and implement the printed packet, or use headless `prdpr phase`.

**Related architecture:** [FLOW.md](FLOW.md#end-to-end-flow), [USER_GUIDE.md](USER_GUIDE.md)

---

<a id="help"></a>

## `prdpr help`

**What it does:** Prints command list and documentation URLs.

**When to use it:** Discover commands.

**What happens next:** Nothing.

**Related architecture:** this file

---

<a id="version"></a>

## `prdpr version`

**What it does:** Prints the binary version. Checkout builds (`go build`, `go install ./cmd/prdpr`) print `dev`. A tagged module install (`go install github.com/lanternfold/prd-pr/cmd/prdpr@v0.1.0`) prints `0.1.0` from Go module build info. GitHub Release binaries inject the tag version via `-X github.com/lanternfold/prd-pr/internal/cli.Version=<version>`. `prdpr --version` is the same command.

**When to use it:** Support / install checks.

**What happens next:** Nothing.

**Related architecture:** `internal/cli.Version`

---

<a id="doctor"></a>

## `prdpr doctor`

**What it does:** Reports OS, arch, Go, Git, Cursor editor, Cursor Agent, GitHub CLI presence.

**When to use it:** After install.

**What happens next:** Missing Git is an error. Missing `gh` or Cursor is informational.

**Related architecture:** [USER_GUIDE.md](USER_GUIDE.md#installation-and-setup)

---

<a id="validate-prd"></a>

## `prdpr validate-prd [--json] <PRD.md>`

**What it does:** Deterministic PRD contract gate. PRD path only. No project mutation.

**When to use it:** Before bootstrap, and after any PRD edit. The plugin Step 0 must call this.

**What happens next:** VALID → bootstrap/prepare may run. REJECTED → human edits the PRD; validate again.

**Related architecture:** [PRD_AUTHORING_CONTRACT.md](PRD_AUTHORING_CONTRACT.md), [LLM_AND_HUMAN.md](LLM_AND_HUMAN.md)

---

<a id="inspect"></a>

## `prdpr inspect [--json] [--graph] <PRD.md>`

**What it does:** Parses the PRD; reports sections, IDs, diagnostics. `--graph` prints the DAG from explicit phase dependencies.

**When to use it:** Authoring and debugging a PRD without creating a project.

**What happens next:** Fix parse errors before `validate-prd` / bootstrap.

**Related architecture:** [PHASES.md](PHASES.md#p1--prd-parser)

---

<a id="bootstrap"></a>

## `prdpr bootstrap <PRD.md>`

**What it does:** PRD-only placement: validate → type/location → create/reuse destination → copy PRD → Cursor rules → Git → optional GitHub/rulesets → prepare.

**When to use it:** Same as `prdpr <PRD.md>`.

**What happens next:** Implement the prepared packet. Ambiguous type/Studio/GitHub auth stops for a human.

**Related architecture:** [FLOW.md](FLOW.md#studio-bootstrap)

---

<a id="init"></a>

## `prdpr init [directory]`

**What it does:** Creates `.project/state.json` and events if missing. Idempotent.

**When to use it:** Rare. Prefer `prdpr <PRD.md>`, which initializes as part of bootstrap. `init` does not validate a PRD or build the graph.

**What happens next:** `status` / `prepare`.

**Related architecture:** [PHASES.md](PHASES.md#p0--skeleton)

---

<a id="preflight"></a>

## `prdpr preflight [--json] [--prd FILE] [--mode interactive|headless] [--worker cursor|fake] [directory]`

**What it does:** Advisory readiness report (OS, git, Cursor, dirty tree, tools). Does not replace P4 fail-closed checks before a worker.

**When to use it:** Before a run, or from the plugin with `--mode interactive`.

**What happens next:** Blocking items must be fixed. Headless worker still enforces Git baseline separately.

**Related architecture:** [PHASES.md](PHASES.md#p3--preflight)

---

<a id="prepare"></a>

## `prdpr prepare [--prd FILE] [--phase ID] [directory]`

**What it does:** Builds a deterministic task packet and Git baseline. Selects the first READY phase when `--phase` is omitted. Refuses BLOCKED, COMPLETED, unknown, and WAITING phases. Marks the selected node RUNNING. Does not invoke Cursor.

**When to use it:** Interactive path after bootstrap; plugin after a verified phase to get the next READY packet.

**What happens next:** Implement the packet in the current Cursor session, then `verify`. Headless users usually call `run`/`phase` instead.

**Related architecture:** [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md)

---

<a id="run"></a>

## `prdpr run [--prd FILE] [--phase ID] [--worker cursor|fake] [--timeout DURATION] [directory]`

**What it does:** Headless: prepare (if needed) + invoke the P4 worker (`cursor-agent` or `fake`). Does **not** verify. `verified_success` stays false.

**When to use it:** Scripts/tests and headless implementation of **one** phase. The Cursor plugin must not call this.

**What happens next:** `prdpr verify`. Or use `prdpr phase` to run the inner loop.

**Related architecture:** [CURSOR.md](CURSOR.md#headless-cursor-agent)

---

<a id="verify"></a>

## `prdpr verify [--json] [directory]`

**What it does:** Independent verification (tests, packet/DoD evidence). Only this path may set `verified_success=true`. On success, the engine commits (default) and attempts delivery.

**When to use it:** After implementation in the current session, and after every repair.

**What happens next:** VERIFIED → delivery + next READY (plugin `prepare`, or already walked by `phase`). Fail → `review` / `repair` or human. Manual AC → human.

**Related architecture:** [FLOW.md](FLOW.md#verification-boundary)

---

<a id="review"></a>

## `prdpr review [--json] [directory]`

**What it does:** Classifies a failed verification. Deterministic first. LLM review only if the model router selects a model **and** an adapter other than `llm.None` is injected (CLI default is None).

**When to use it:** After failed `verify`, before `repair`.

**What happens next:** `recommend_repair` → `repair`. `recommend_human` → `feedback`.

**Related architecture:** [LLM_AND_HUMAN.md](LLM_AND_HUMAN.md)

---

<a id="repair"></a>

## `prdpr repair [--json] [directory]`

**What it does:** Writes a bounded repair packet (max 3 product attempts per incident). Does not apply the fix.

**When to use it:** When review recommends repair.

**What happens next:** Implement that packet in the current session (plugin) or let `prdpr phase` apply the headless repair worker. Then `verify`.

**Related architecture:** [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md#1-phase-implementation-loop)

---

<a id="phase"></a>

## `prdpr phase [--prd FILE] [--phase ID] [--worker cursor|fake] [--timeout DURATION] [directory]`

**What it does:** Headless **outer graph walk**: for each READY phase, run the inner loop (worker, verify, review, bounded repair). Stops on human wait, failure, or project complete.

**When to use it:** Unattended local runs. Plugin must not call this.

**What happens next:** Runtime start when the graph is AllCompleted. Or a human request.

**Related architecture:** [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md#2-outer-graph-loop)

---

<a id="commit"></a>

## `prdpr commit [directory]`

**What it does:** Commits product files only if `verified_success` is true and the phase is VERIFIED/COMPLETED.

**When to use it:** Manual retry if AutoCommit was off. Normal verified success already commits.

**What happens next:** `pr` if GitHub milestone policy requires it.

**Related architecture:** [GIT_GITHUB.md](GIT_GITHUB.md)

---

<a id="pr"></a>

## `prdpr pr [directory]`

**What it does:** Create or reuse a milestone PR. Refused until verified. Forces GitHub enabled for this invocation.

**When to use it:** When a PR should exist and automatic milestone open did not.

**What happens next:** `checks` / `merge` (merge still default-denied).

**Related architecture:** [GIT_GITHUB.md](GIT_GITHUB.md)

---

<a id="checks"></a>

## `prdpr checks [directory]`

**What it does:** Reads GitHub Actions / PR checks via `gh`. Observes; does not repair.

**When to use it:** After a pushed PR.

**What happens next:** Diagnosis if checks fail (not a fully autonomous CI repair loop).

**Related architecture:** [PHASES.md](PHASES.md#p6--ci-integration)

---

<a id="merge"></a>

## `prdpr merge [directory]`

**What it does:** Evaluates merge policy. Merges only if `AutoMergeEnabled` and verification/checks/approval pass. Default: refuse.

**When to use it:** Only when you explicitly want engine-driven merge.

**What happens next:** Optional branch cleanup / fast-forward of local base.

**Related architecture:** [GIT_GITHUB.md](GIT_GITHUB.md)

---

<a id="runtime"></a>

## `prdpr runtime [directory]`

**What it does:** Starts the structured local app runtime after project completion. On failure, may prepare one runtime repair packet (not a fully autonomous inner re-loop in one CLI call).

**When to use it:** After `AllCompleted`, or when the engine already started runtime and you need to retry.

**What happens next:** Ready URL, skip (no runtime for that type), or human/repair packet.

**Related architecture:** [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md#4-runtime-feedback-loop)

---

<a id="feedback"></a>

## `prdpr feedback [--request ID] [--text TEXT] [--status STATUS] [--credential NAME] [directory]`

**What it does:** Records one human response. Secrets must not be written here; credential flags record presence metadata.

**When to use it:** After a `WAITING_FOR_HUMAN` request.

**What happens next:** `prdpr resume`.

**Related architecture:** [LLM_AND_HUMAN.md](LLM_AND_HUMAN.md)

---

<a id="resume"></a>

## `prdpr resume [directory]`

**What it does:** Continues after a recorded response. GitHub auth/ruleset: reconcile or prepare. Runtime fail: start runtime again. Manual AC confirmed: verify. Otherwise leaves a resumable failed/waiting state for the next verify/repair.

**When to use it:** After `feedback`.

**What happens next:** Owning loop continues; not a full autonomous re-walk of all remaining phases unless you then call `phase` or the plugin `prepare` loop.

**Related architecture:** [GRAPH_AND_LOOPS.md](GRAPH_AND_LOOPS.md#3-human-interruption-resume-loop)

---

<a id="status"></a>

## `prdpr status [directory]`

**What it does:** Prints persisted project state (phase, status, cost line if present).

**When to use it:** Anytime, including after a crash.

**What happens next:** Choose `prepare`, `verify`, `resume`, or `runtime` from the state.

**Related architecture:** [FLOW.md](FLOW.md)

---

## Not implemented as CLI commands

These appear in architecture/PRD intent but are **not** CLI commands today: `pause`, `retry`, `logs`, `knowledge`, `learn`.
