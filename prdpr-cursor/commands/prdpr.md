---
name: prdpr
description: Start the PRD→PR workflow in this Cursor workspace by invoking the existing prdpr engine.
---

# /prdpr

Start PRD→PR from this Cursor session. Follow the **prdpr** skill. Do not invent orchestration.

1. Say you are invoking the PRD→PR engine. After a successful `prepare`, this current Cursor session is the implementation actor for the packet only.
2. Identify the current Cursor workspace/project root. Pass that path explicitly to the engine. Do not assume the shell cwd, the repo name, or a fixed home path.
3. Look only at `PRD.md` in that workspace root. If it exists, use it. If it does not, ask the human for the PRD path. Do not recurse or guess among multiple files.
4. Resolve the `prdpr` binary (PATH, else workspace `dist/prdpr` when this repo is the engine). Invoke existing commands: `inspect`, `preflight`, `prepare`, `verify`, `review`, `repair`, `feedback`, `resume`, `status`. Pass the workspace directory and PRD path as the CLI requires.
5. After `prepare` succeeds, read the packet file the engine printed. Implement **only** that packet in this session. Do not spawn Cursor or `cursor-agent`.
6. After implementation, run `prdpr verify` on the workspace. Report the engine result. Do not verify in chat.
7. If verify is not VERIFIED, run `prdpr review`. If the engine recommends repair, run `prdpr repair` and implement that repair packet in this session, then verify again. If the engine asks for a human, ask one engine-provided question.
8. Never call `prdpr run` or `prdpr phase` from this plugin (those launch the headless worker).
