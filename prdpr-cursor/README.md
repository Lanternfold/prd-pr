# PRD→PR Cursor plugin (V0)

Thin Cursor UX for the PRD→PR **Go engine**. The plugin does not orchestrate.

User guide: [docs/USER_GUIDE.md](../docs/USER_GUIDE.md) · Cursor architecture: [docs/CURSOR.md](../docs/CURSOR.md) · CLI: [docs/CLI.md](../docs/CLI.md)

```text
User → Cursor → this plugin (/prdpr + skill) → prdpr CLI (PATH) → Go engine
         → packet on disk → this same Cursor session implements the packet
         → prdpr verify
```

The P4 Cursor **worker** is a separate adapter. This plugin must not launch `cursor-agent`, `agent`, `prdpr run`, or `prdpr phase`.

The plugin does **not** require a PRD→PR source checkout at runtime. The engine is the released `prdpr` binary on `PATH`.

## Contents

- `.cursor-plugin/plugin.json` — Cursor Plugin manifest
- `commands/prdpr.md` — `/prdpr`
- `skills/prdpr/SKILL.md` — how to talk to the engine

No MCP, hooks, subagents, custom agents, or extra commands.

## V0 installation

Current published CLI release: **v0.1.1**. Marketplace publication is **not** part of V0 (future work).

1. Install released `prdpr` and put it on `PATH`. See [docs/USER_GUIDE.md](../docs/USER_GUIDE.md#installation-and-setup). Confirm:

   ```bash
   command -v prdpr
   prdpr version
   ```

   Expect `0.1.1` for that release.

2. Load this plugin into Cursor. Copy the plugin directory (prefer copy over symlink; Cursor may ignore a symlink whose target is outside `~/.cursor/plugins/local`):

   ```bash
   mkdir -p ~/.cursor/plugins/local
   rm -rf ~/.cursor/plugins/local/prdpr
   cp -R /path/to/prdpr-cursor ~/.cursor/plugins/local/prdpr
   ```

   You only need this `prdpr-cursor/` folder. You do not need to build the Go engine. A full source checkout is not a runtime requirement.

3. Reload the Cursor window if `/prdpr` does not appear. Enable the plugin in **Customize** if needed (including third-party / local plugins).

4. Open a product workspace (or a folder that contains `PRD.md`) and run `/prdpr`.

If `prdpr` is not on `PATH`, `/prdpr` must stop with that install link. It must not look for `dist/prdpr` or compile a checkout.

## `/prdpr`

Two entry workflows, same implementation actor (this Cursor session):

| Workflow | How you start | Engine command |
|---|---|---|
| **CLI-first** | `prdpr path/to/PRD.md` in a shell, then open printed `product_root` in Cursor | `prdpr prepare` in the product workspace |
| **Plugin-first** | Open a folder with `PRD.md` (or supply a PRD path) and run `/prdpr` | `prdpr <PRD>` (bootstrap + prepare). If `product_root` is not this workspace, stop and reopen there. |

Then:

1. Resolve `prdpr` on `PATH` only. If missing, stop and point at the user-guide install section.
2. `prdpr validate-prd` is optional. The engine already validates inside `prdpr <PRD.md>` and `prepare`. If you run it and it is REJECTED, stop.
3. `inspect` and a separate `preflight` CLI call are optional diagnostics. `prepare` already runs preflight internally and refuses when blocking. Do not describe `prepare` as skipping that gate.
4. Read the engine task packet (`packet:` path printed by the CLI, JSON under `.project/`).
5. Implement **only that packet** in the current Cursor session.
6. Run `prdpr verify`. The plugin does not grade the work.
7. On failed verify: `prdpr review`, then `prdpr repair` if the engine says so, implement the repair packet here, verify again.
8. After a VERIFIED phase, if the project is not complete, `prdpr prepare` without `--phase` (engine selects the next READY phase) and implement that packet in this session.
9. After project completion, `prdpr runtime` if the engine expects it.
10. Never run `prdpr run` or `prdpr phase`.

## Engine CLI

```text
prdpr validate-prd [--json] <PRD.md>
prdpr <PRD.md>
prdpr bootstrap <PRD.md>
prdpr inspect [--json] [--graph] <PRD.md>
prdpr preflight [--json] [--prd FILE] [directory]
prdpr prepare [--prd FILE] [--phase ID] [directory]
prdpr verify [--json] [directory]
prdpr review [--json] [directory]
prdpr repair [--json] [directory]
prdpr runtime [directory]
prdpr feedback [--request ID] [--credential NAME] [directory]
prdpr resume [directory]
prdpr status [directory]
```

Pass the workspace directory explicitly. Do not rely on cwd.

`prdpr run --worker cursor` and `prdpr phase` remain the **headless** P4 path. The plugin must not call them.
