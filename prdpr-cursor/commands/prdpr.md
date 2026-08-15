---
name: prdpr
description: Start the PRD→PR workflow in this Cursor workspace by invoking the existing prdpr engine.
---

# /prdpr

Start PRD→PR from this Cursor session. Follow the **prdpr** skill. Do not invent orchestration.

1. Say you are invoking the PRD→PR engine. After a successful `prepare`, this current Cursor session is the implementation actor for the packet only.
2. Identify the current Cursor workspace/project root. Pass that path explicitly to the engine. Do not assume the shell cwd, the repo name, or a fixed home path.
3. Look only at `PRD.md` in that workspace root. If it exists, use it. If it does not, ask the human for the PRD path. Do not recurse or guess among multiple files.
4. **Step 0:** Resolve the `prdpr` binary (PATH, else workspace `dist/prdpr` when this repo is the engine). If the user supplied a PRD path, or `PRD.md` exists at the workspace root, run `prdpr validate-prd <PRD>`. If status is REJECTED, STOP. Present the complete engine report. Do not create a project, do not initialize Git/GitHub, do not invoke implementation. Do not reimplement validation in chat.
5. If VALID and the current workspace is **not** already the product root, run `prdpr <PRD>` (PRD-only bootstrap). Use the printed `product_root` for all later commands. If Cursor is not opened on that directory, stop and tell the human to open it. If the workspace already contains that product, `prdpr prepare` is enough.
6. After `prepare` succeeds, read the packet file the engine printed. Implement **only** that packet in this session. Do not spawn Cursor or `cursor-agent`.
7. After implementation, run `prdpr verify` on the product root. Report the engine result. Do not verify in chat.
8. If verify is not VERIFIED, run `prdpr review`. If the engine recommends repair, run `prdpr repair` and implement that repair packet in this session, then verify again. If the engine asks for a human, ask one engine-provided question.
9. After a phase is VERIFIED but the project is not complete, call `prdpr prepare` without `--phase`. The engine chooses the next READY phase. Implement that packet in this same session, then verify. Stop on waiting_for_human or prepare refusal.
10. After the engine reports the project completed, run `prdpr runtime`. If startup fails, follow the engine repair packet in this session (bounded). Do not loop forever.
11. Never call `prdpr run` or `prdpr phase` from this plugin (those launch the headless worker). The plugin is not a scheduler.
