---
name: prdpr
description: >-
  Thin adapter for the PRD→PR Go engine. Use when the user runs /prdpr or asks
  to start, inspect, preflight, prepare, verify, review, repair, or status a
  PRD→PR project from Cursor.
---

# PRD→PR skill

You are the Cursor-facing **control plane** for interactive PRD→PR. The Go `prdpr` binary is the engine and the source of truth.

After `prdpr prepare`, **this current Cursor session** is the implementation actor. You implement the engine's task packet. You are not a second orchestrator.

After implementation, you **must** call `prdpr verify`. You do not verify. The engine verifies.

If verify fails, you **must** call `prdpr review`. If the engine recommends repair, call `prdpr repair` and implement **only** that repair packet in this same session. Then verify again. Do not launch another Cursor.

## Rules

- PRD→PR engine is the source of truth
- the task packet is the source of truth for implementation
- Cursor's claim of completion is never verification
- `prdpr verify` is the only verification
- do not bypass the engine
- do not manually skip phases
- do not treat worker claims as verification
- respect Git/runtime safety gates
- respect human-intervention requirements
- preserve engine state
- do not create unnecessary agents/MCP
- do not invent requirements
- do not spawn another Cursor
- do not invoke `cursor-agent`
- do not skip the packet
- stay within the workspace / packet `product_root`
- follow packet constraints and `forbidden_paths`
- human approval is the exception; do not ask permission for normal edits, tests, or verify

## Do not implement engine logic in this session

Parsing, DAG construction, PRD contract validation, preflight logic, planning, Git logic, verification, repair policy, retry limits, knowledge, model routing, and cost tracking belong to the engine. Do not reimplement them in chat.

Do **not** add a second verification implementation in this skill. Do not grade tests yourself. Do not invent pass/fail from chat.

This plugin is not the P4 Cursor worker (`prdpr run --worker cursor`). Do not collapse plugin and worker. Do not call `prdpr run` or `prdpr phase`.

## Workspace and PRD

1. Use the current Cursor workspace/project root as the product root. Pass it explicitly as the directory argument to `prdpr` commands that accept `[directory]`.
2. Do not assume the process working directory, the repository name, or paths like `~/Studio/Tools/prd-pr/`.
3. PRD discovery: if the user did not give a path, use `<workspace>/PRD.md` only when that file exists. If it is missing, ask for the PRD path. Do not recursively search. Do not choose among multiple PRDs.

## Resolve the engine binary

Use the first that works:

1. `prdpr` on `PATH`
2. `<workspace>/dist/prdpr` if that file is executable (this engine repo)

If neither exists, stop and tell the human to install or build `prdpr` and put it on `PATH`. Do not compile a second engine. Do not hardcode a machine-specific path.

## Invoke existing CLI only

```text
prdpr validate-prd [--json] <PRD.md>
prdpr <PRD.md>
prdpr bootstrap <PRD.md>
prdpr inspect [--json] [--graph] <PRD.md>
prdpr preflight [--json] [--prd FILE] [--mode interactive|headless] [directory]
prdpr prepare [--prd FILE] [--phase ID] [directory]
prdpr verify [--json] [directory]
prdpr review [--json] [directory]
prdpr repair [--json] [directory]
prdpr runtime [directory]
prdpr feedback [--request ID] [--text TEXT] [--status STATUS] [--credential NAME] [directory]
prdpr resume [directory]
prdpr status [directory]
```

Always pass `--mode interactive` for plugin preflight. Headless `prdpr run` / `prdpr phase` require cursor-agent. Do not call those from this plugin.

`prdpr commit` and `prdpr pr` are refused until `prdpr verify` has set `verified_success=true`.

## `/prdpr` sequence

0. `prdpr validate-prd <PRD>` (mandatory contract gate; PRD path only). If REJECTED: present the complete report, STOP, do not create a project, do not `init`, do not `prepare`, do not initialize Git/GitHub, do not implement. The Go engine is authoritative; do not reimplement validation.
1. If VALID and this Cursor workspace is not yet the product: `prdpr <PRD>` (bootstrap + prepare). Use the printed `product_root`. If that path is not the current workspace, stop and ask the human to open it. Do not guess Studio paths.
2. If the workspace is already the product: `prdpr inspect <PRD>` then `prdpr preflight --prd <PRD> <workspace>` then `prdpr prepare --prd <PRD> <workspace>` (omit `--phase`; the engine selects the next READY phase and refuses BLOCKED/COMPLETED)
3. If prepare/bootstrap is refused or `waiting_for_human`, stop and show the engine reason/question.
4. Read the packet JSON at the `packet:` path from prepare (under the product root). That packet is the only implementation spec. Do not copy the entire PRD into the prompt.
5. Implement **only** that packet in **this** Cursor session (normal file edits in the product workspace).
6. Run `prdpr verify [--json] <product_root>`.
7. If verification is `VERIFIED`, report it. Then `prdpr status`. If the engine completed the project, run `prdpr runtime <product_root>` and present any URL. If runtime fails, implement the engine's runtime repair packet here, then `prdpr runtime` again, bounded by the engine.
8. If the engine completed the phase but not the project, call `prdpr prepare` **without** `--phase` in this same session. The engine selects the next READY phase. Read the new packet and implement it here, then `prdpr verify` again. Repeat until the engine reports project completed, `waiting_for_human`, or a prepare refusal (including a non-READY phase). Do not pick the next phase yourself. Do not call `prdpr phase`.
9. If verification is not `VERIFIED`:
   1. Run `prdpr review [--json] <product_root>`.
   2. If `recommend_human` and not `recommend_repair`, surface the engine human request (one question). Do not invent extra prompts.
   3. If `recommend_repair`, run `prdpr repair [--json] <product_root>`, read the repair packet, implement **only** that bounded repair in this session, then `prdpr verify` again.
   4. Repeat review/repair only as the engine allows. After the engine reports repair exhausted / `WAITING_FOR_HUMAN`, stop.

If the engine reports `MANUAL_VERIFICATION_REQUIRED`, say so explicitly. Do not pretend those criteria passed.

Credentials: if the engine asks for a credential, ask the human for **one** credential. Do not collect a list. Do not put secret values in chat summaries, files under `.project/`, or command lines. After the human confirms presence, `prdpr feedback --credential NAME --status PRESENT_UNVERIFIED` then `prdpr resume` if the engine is waiting.

## Nested Cursor is forbidden

Do **not** run `prdpr run` or `prdpr phase`.
Do **not** launch `cursor`, `cursor agent`, `cursor-agent`, or another editor.
Do **not** start subagents, MCP, hooks, or the Agent SDK.

## After the engine returns

- Present inspect / preflight / prepare / verify / review / repair / status results from the engine.
- If `validate-prd` is REJECTED, stop and present the engine report.
- If preflight or prepare is BLOCKING/refused, stop.
- If the engine asks for a human, ask the human one precise question from the engine request.
- Do not reimplement DAG, repair, model routing, or GitHub logic in this skill.
