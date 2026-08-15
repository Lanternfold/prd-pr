# Cursor integration

Two different Cursor surfaces exist. Do not collapse them.

Related: [FLOW.md](FLOW.md), [CLI.md](CLI.md), [prdpr-cursor/README.md](../prdpr-cursor/README.md), ADR-004, ADR-012.

## Current session (interactive) — primary UX

```text
User
 ↓
Cursor /prdpr
 ↓
thin plugin (command + skill)
 ↓
Go engine CLI
 ↓
deterministic packet
 ↓
this same Cursor session implements the packet
 ↓
prdpr verify  (engine)
```

Facts:

- Interactive Cursor is the implementation actor.
- The plugin is intentionally thin. It does not own the graph, repair counts, or Git policy.
- The plugin does **not** spawn another Cursor session and must not call `cursor-agent`.
- The plugin must not call `prdpr run` or `prdpr phase`.
- The Go engine remains authoritative.
- After a VERIFIED phase, the plugin calls `prdpr prepare` without `--phase` so the engine selects the next READY phase. That is not plugin-owned sequencing; it is repeating an engine command.

## Headless `cursor-agent` — P4 worker

```text
CLI prdpr run | prdpr phase
 ↓
Go engine
 ↓
cursor-agent --print --force --trust --workspace <product-root>
 ↓
implementation
 ↓
engine Verify
```

Use headless when no human is sitting in the editor. Use interactive when you are in Cursor and want to watch/edit.

Pinned mechanism (code): `internal/cursor.PinnedMechanism`.

## Project Cursor rules vs global permissions

| Layer | Role |
|---|---|
| Product `.cursor/rules` written at bootstrap | Behavioral instructions for implementation |
| Plugin skill `prdpr-cursor/skills/prdpr/SKILL.md` | How to talk to the engine |
| Cursor **Run Mode / Auto-review / sandbox** | Cursor product settings. PRD→PR does not set them. |

`.cursor/rules` does **not** grant terminal permissions. If Auto-review prompts on `prdpr` or tests, approve those commands in Cursor. That expectation is documented, not automated.

## Local plugin install

Development mechanism (IMPLEMENTED): symlink `prdpr-cursor` into `~/.cursor/plugins/local/prdpr`. Reload the window.

Marketplace packaging and versioned distribution: **PLANNED**.
