# PRD→PR Cursor plugin (V0)

Thin Cursor UX for the PRD→PR **Go engine**. The plugin does not orchestrate.

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

1. Uses the current Cursor workspace as the product root.
2. Uses `PRD.md` at that root, or asks for a path.
3. Runs `prdpr inspect`, `preflight`, and `prepare` with those paths.
4. Reads the engine task packet.
5. Implements **only that packet** in the current Cursor session.
6. Does not run `prdpr run` or `prdpr phase` (nested Cursor worker).
7. Runs `prdpr verify` after implementation. The plugin does not grade the work.
8. On failed verify: `prdpr review`, then `prdpr repair` if the engine says so, implement the repair packet here, verify again.
9. Surfaces engine human requests one at a time. Does not ask permission for ordinary edits.

## Engine CLI

```text
prdpr inspect [--json] [--graph] <PRD.md>
prdpr preflight [--json] [--prd FILE] [directory]
prdpr prepare [--prd FILE] [--phase ID] [directory]
prdpr verify [--json] [directory]
prdpr review [--json] [directory]
prdpr repair [--json] [directory]
prdpr feedback [--request ID] [--credential NAME] [directory]
prdpr resume [directory]
prdpr status [directory]
```

Pass the workspace directory explicitly. Do not rely on cwd.

`prdpr run --worker cursor` and `prdpr phase` remain the **headless** P4 path. The plugin must not call them.
