---
name: prdpr
description: Start the PRD→PR workflow in this Cursor workspace by invoking the existing prdpr engine.
---

# /prdpr

Start PRD→PR from this Cursor session. Follow the **prdpr** skill. Do not invent orchestration.

1. Say you are invoking the PRD→PR engine. After a successful `prepare`, this current Cursor session is the implementation actor for the packet only.
2. Identify the current Cursor workspace/project root. Pass that path explicitly to the engine. Do not assume the shell cwd, the repo name, or a fixed home path.
3. Look only at `PRD.md` in that workspace root. If it exists, use it. If it does not, ask the human for the PRD path. Do not recurse or guess among multiple files.
4. Resolve the engine as `prdpr` on `PATH` only. If it is missing, STOP with an actionable error: install the released PRD→PR CLI (current release **v0.1.1**), put `prdpr` on `PATH`, then retry. Point to [docs/USER_GUIDE.md](https://github.com/lanternfold/prd-pr/blob/main/docs/USER_GUIDE.md#installation-and-setup). Do not use `<workspace>/dist/prdpr`. Do not search for, clone, or build a PRD→PR source checkout.
5. `prdpr validate-prd` is **optional**. The engine already contract-validates inside `prdpr <PRD.md>` and `prdpr prepare`. If you run `validate-prd` and the status is REJECTED, STOP, present the complete engine report, and do not create a project or implement. Do not reimplement validation in chat.
6. **Plugin-first (no product yet):** run `prdpr <PRD>` (PRD-only bootstrap + prepare). Use the printed `product_root` for later commands. If Cursor is not opened on that directory, STOP and tell the human to open it, then run `/prdpr` again.
7. **Already the product (including CLI-first):** run `prdpr prepare` on the workspace (omit `--phase`; the engine selects the next READY phase). `prepare` internally runs preflight and refuses when it is blocking. Do not treat a separate `inspect` or `preflight` CLI call as required. Those commands are optional diagnostics.
8. After `prepare` succeeds, read the packet file the engine printed. Implement **only** that packet in this session. Do not spawn Cursor or `cursor-agent`.
9. After implementation, run `prdpr verify` on the product root. Report the engine result. Do not verify in chat.
10. If verify is not VERIFIED, run `prdpr review`. If the engine recommends repair, run `prdpr repair` and implement that repair packet in this session, then verify again. If the engine asks for a human, ask one engine-provided question.
11. After a phase is VERIFIED but the project is not complete, call `prdpr prepare` without `--phase`. The engine chooses the next READY phase. Implement that packet in this same session, then verify. Stop on waiting_for_human or prepare refusal.
12. After the engine reports the project completed, run `prdpr runtime`. If startup fails, follow the engine repair packet in this session (bounded). Do not loop forever.
13. Never call `prdpr run` or `prdpr phase` from this plugin (those launch the headless worker). The plugin is not a scheduler.
