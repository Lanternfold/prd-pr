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
prdpr on PATH (released CLI)
 ↓
Go engine
 ↓
deterministic packet on disk
 ↓
this same Cursor session reads the packet and implements it
 ↓
prdpr verify  (engine)
 ↓
engine-selected next READY phase (prdpr prepare, no --phase)
```

Facts:

- Interactive Cursor is the implementation actor.
- The plugin is intentionally thin. It does not own the graph, repair counts, or Git policy.
- The plugin does **not** spawn another Cursor session and must not call `cursor-agent` or `agent`.
- The plugin must not call `prdpr run` or `prdpr phase`.
- The Go engine remains authoritative.
- Runtime requirement is `prdpr` on `PATH`. There is no fallback to `<workspace>/dist/prdpr` and no search of a PRD→PR source checkout.
- After a VERIFIED phase, the plugin calls `prdpr prepare` without `--phase` so the engine selects the next READY phase. That is not plugin-owned sequencing; it is repeating an engine command.

`validate-prd` is optional. `inspect` and a separate `preflight` invocation are optional diagnostics. `prdpr <PRD.md>` and `prdpr prepare` already run the contract gate; `prepare` already runs preflight and refuses when it is blocking.

## CLI-first vs plugin-first

| Workflow | Entry | Then |
|---|---|---|
| CLI-first | Shell: `prdpr path/to/PRD.md` | Open printed `product_root` in Cursor. `/prdpr` (or `prdpr prepare`) implements the packet. |
| Plugin-first | Cursor already open; `/prdpr` | Plugin runs `prdpr <PRD>` if this folder is not yet the product. If `product_root` differs, stop and reopen that folder. |

Do not mix this interactive path with headless `prdpr phase` in the same session.

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

## Local plugin install (V0)

V0 is local load, not Cursor Marketplace (Marketplace remains **PLANNED**).

1. Install released `prdpr` **v0.1.1** on `PATH` ([USER_GUIDE.md](USER_GUIDE.md#installation-and-setup)). `prdpr version` should print `0.1.1`.
2. Copy `prdpr-cursor/` to `~/.cursor/plugins/local/prdpr` (copy, not a symlink to a tree outside that directory — Cursor may ignore those symlinks).
3. Reload the Cursor window if required. Enable the plugin in **Customize** if needed.
4. Run `/prdpr` in a product workspace (or a folder with `PRD.md`).

The plugin does not need a built engine checkout. If `prdpr` is missing, the skill stops and points at the user-guide install section.

Contributor note: you may copy `prdpr-cursor/` from a clone of this repository. That is how you obtain the plugin files until Marketplace exists. It is not how the plugin locates the `prdpr` binary.
