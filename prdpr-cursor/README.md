# PRD→PR Cursor plugin (V0)

Thin Cursor UX for the PRD→PR **Go engine**. The plugin does not orchestrate.

User guide: [docs/USER_GUIDE.md](../docs/USER_GUIDE.md) · Cursor architecture: [docs/CURSOR.md](../docs/CURSOR.md) · CLI: [docs/CLI.md](../docs/CLI.md)

```text
User → Cursor → this plugin (/prdpr + skill) → prdpr CLI → Go engine
```

The P4 Cursor **worker** is a separate adapter. This plugin must not launch `cursor-agent` or another Cursor.

## Contents

- `.cursor-plugin/plugin.json` — Cursor Plugin manifest
- `commands/prdpr.md` — `/prdpr`
- `skills/prdpr/SKILL.md` — how to talk to the engine

No MCP, hooks, subagents, custom agents, or extra commands.

## Requirements

- Cursor (local plugin load)
- `prdpr` on `PATH`, or `dist/prdpr` in the engine workspace

## Local install (not Marketplace)

```bash
mkdir -p ~/.cursor/plugins/local
ln -sfn /absolute/path/to/prd-pr/prdpr-cursor ~/.cursor/plugins/local/prdpr
```

Reload the Cursor window. Enable the plugin in **Customize** if needed.

## `/prdpr`

1. Takes a PRD path (user-supplied or workspace `PRD.md`).
2. Step 0: `prdpr validate-prd <PRD>`. If REJECTED, stop and show the engine report. Do not create a project or implement.
3. If VALID, `prdpr <PRD>` bootstraps a Studio project when needed, then prepare. Later commands use the printed `product_root`.
4. Reads the engine task packet.
5. Implements **only that packet** in the current Cursor session.
7. Does not run `prdpr run` or `prdpr phase` (nested Cursor worker).
8. Runs `prdpr verify` after implementation. The plugin does not grade the work.
9. On failed verify: `prdpr review`, then `prdpr repair` if the engine says so, implement the repair packet here, verify again.
10. After a VERIFIED phase, if the project is not complete, `prdpr prepare` without `--phase` (engine selects the next READY phase) and implement that packet in this session.
11. Surfaces engine human requests one at a time. Does not ask permission for ordinary edits.

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
