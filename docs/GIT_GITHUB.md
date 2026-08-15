# Git and GitHub

Engine-owned lifecycle. Cursor is not the VCS owner.

Related: [FLOW.md](FLOW.md), ADR-002, ADR-011, `internal/engine/delivery.go`, `internal/engine/repo.go`.

## Delivery invariant

After a **verified** implementation pass, the engine must leave a durable Git outcome:

```text
verified phase
 ↓
commit          (AutoCommit default true; refused if not verified)
 ↓
push            (AutoPush default true; skipped if no remote and GitHub disabled)
 ↓
if GitHub enabled and feature-branch policy applies:
    push the owned branch
    open/update milestone PR when pr_boundary says so (run complete or every phase)
if GitHub enabled and direct push to the configured base is the policy
    (GitHubEnabled with pr_boundary=never and no UseFeatureBranch):
    push that branch
```

There is no supported happy path where a verified phase is left only in the working tree for the user to commit. `AutoCommit=false` exists for tests and explicit overrides; it is not the product default.

Rules:

- Never commit or push unverified implementation.
- Never force-push.
- Never destructively reset unrelated history.
- Respect branch protection: when feature branches are required, the engine refuses product push to the base branch.
- Do not merge the default branch unless `AutoMergeEnabled` (default false).
- If GitHub authentication or push fails **while GitHub is enabled**, the engine writes a human request and keeps resumable state. Local commits already made are kept.
- If GitHub is **disabled**, a local commit is the durable outcome; missing remotes skip push.

## What Cursor must not do

Unless the packet explicitly requires a product Git operation (rare):

- commit
- push
- create PRs
- merge
- rewrite history

`prdpr commit` / `prdpr pr` are refused until verification passed.

## PR policy (ADR-011)

Default `pr_boundary=run`: one milestone PR when the **project** completes, not one PR per phase. `phase` and `never` exist as config values.

Feature branches are used when `UseFeatureBranch` is true **or** GitHub is enabled and `pr_boundary != never`.

## Bootstrap

`prdpr <PRD.md>` initializes Git in the product directory. Users should not have to `git init` first. GitHub remote creation requires `GitHubEnabled` plus `gh` auth and configured owner/name; otherwise local-only or a human request (PRD-only path).
