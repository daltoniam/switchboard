---
name: pr-shepherd
description: >
  Open or shepherd a daltoniam/switchboard PR until local make ci and remote
  CI plus Crush review comments are clean. Never merge unless explicitly asked.
---

# PR Shepherd

Open or adopt a pull request in `daltoniam/switchboard`, then keep working until the exact PR head is locally and remotely clean. This is an active remediation loop: run `make ci`, diagnose, fix, test, commit, push when authorized, and re-bind every gate after every head change.

Do not use the hosted `pr-shepherd` skill. That skill is scoped to `Vluxe/Switchboard-hosted` and `task pr:check`. This repository's gate is `make ci`.

When the user explicitly authorizes merge, merge only after the Merge Authorization Contract is fully satisfied. A clean shepherding result without merge authorization means ready for merge, not merged.

## When to Use

- The user asks to open a PR in `daltoniam/switchboard`.
- The user asks to shepherd a PR, watch CI, or fix a failing PR.
- An Orca automation opens an integration PR and must loop on review comments and CI.

Do not use for another repository. A watch-only request does not authorize commits or pushes. Opening, updating, fixing, or shepherding a PR authorizes the branch commits and pushes required by that request, but never authorizes merge.

## Scope Guard

Prove repository ownership from live state:

```bash
git rev-parse --show-toplevel
git remote get-url origin
gh repo view --json nameWithOwner,owner,defaultBranchRef
```

Require `nameWithOwner == "daltoniam/switchboard"`. Read `AGENTS.md` and applicable nested instructions before changing code, choosing gates, committing, pushing, or merging.

Refuse to proceed when:

- the remote is not `daltoniam/switchboard`;
- the requested operation does not authorize its required mutations;
- unrelated worktree changes cannot be isolated safely;
- GitHub credentials are unavailable for a requested PR mutation;
- another actor moves the PR head and the resulting delta cannot be safely adopted.

Never mutate `main`, force-push, discard unrelated changes, expose secrets, or weaken tests and checks to make the gate pass.

## Exact-Head Local Gate

Run the full local gate against current `HEAD`:

```bash
make ci
```

Do not substitute individual Go commands, a partial test package, or remote CI for the full gate. Narrow commands are allowed only while diagnosing and iterating; the final local gate must be unmodified `make ci`.

`make ci` runs proto-check, build, vet, test-race (including compose lifecycle tests), lint, and security.

A zero exit is necessary. After it passes, record:

```bash
git rev-parse HEAD
```

Local `make ci` is the required remediation and pre-push gate. Do not wait on remote GitHub Actions to learn whether the change is good; reproduce and fix locally first. Remote GitHub Actions still validates the pushed commit and is required for a terminal-clean remote bind.

## Terminal-Clean Contract

Bind every claim to one exact head SHA. The PR is complete only when all conditions hold simultaneously:

1. Local `make ci` passes on the exact head.
2. The PR head still equals the local exact head.
3. The PR's required GitHub checks for that exact head are `completed/success`. Discover live check names; typical lanes are `build`, `test`, `lint`, `security`, `rust-sdk`, and `compose`.
4. The follow-on `Crush PR Review` `workflow_run` triggered after that exact-head CI run is completed/success. Because GitHub attributes `workflow_run` jobs to the default branch, do not require its run `headSha` to equal the PR head. Prefer the `Crush PR Review` check run posted onto the PR head when it exists.
5. The remote review associated with that workflow run is submitted against the exact PR head. Collect its findings from GitHub PR comments and review threads. There is no confidence score and no generated body block to check.
6. Every actionable automated-review comment encountered during shepherding has a recorded disposition and is addressed by code, tests, docs, or evidence.
7. Automated review threads containing addressed findings are resolved only after an evidence-bearing reply when repository policy and permissions allow it. No actionable automated thread remains unresolved.
8. Required branch-protection checks are successful, and `mergeStateStatus` has no blocking failure.
9. A final re-fetch still returns the pinned head SHA.

The remote review account is configured through a secret and is not fixed in repository code. Discover its login from the reviews and comments created during the exact-head `Crush PR Review` workflow run. Do not hard-code a bot identity.

### Merge Authorization Contract

If the user explicitly asks to merge, do not merge until Terminal-Clean holds and:

1. All required GitHub checks are successful on the exact head.
2. There are zero unresolved actionable automated-review threads.
3. Every remote review comment from the shepherding session is fixed on the current head or resolved with evidence.
4. `mergeable` is not conflicting and `mergeStateStatus` is `CLEAN`, or its only non-clean state is a non-blocking GitHub transient that clears on refresh.
5. Re-fetching the PR immediately before merge still returns the pinned head.

Never use admin merge, bypass branch protection, or force a merge. Use the repository's allowed merge method, preferring squash when no repository or user instruction says otherwise.

Orca integration automations must not merge.

## Procedure

### 1. Open or Adopt the PR

For a new PR:

1. Determine the default branch from `gh repo view --json defaultBranchRef`, fetch it, and branch from it.
2. If the current branch is the default branch, create and switch to a descriptive feature branch before making further changes: `git switch -c <type>/<short-description>`. Never commit requested work directly to the default branch.
3. Confirm the feature branch contains only the requested change.
4. Inspect status, staged and unstaged changes, branch commits, and the diff from the default branch.
5. Run `make ci` and remediate until it passes.
6. Commit any remaining intended changes with an imperative-mood message. AI commits MUST include:

   ```
   Co-Authored-By: <agent model name> <noreply@anthropic.com>
   ```

7. Because the commit changes HEAD, run `make ci` again on the committed exact head.
8. Push a new feature branch with `git push --set-upstream origin HEAD`, then create the PR with an accurate title, summary, and test plan. Label automation-owned PRs `orca-automation` and `integration` when the work adds an integration.

For an existing PR:

```bash
gh pr view <number-or-url> \
  --json number,url,state,baseRefName,headRefName,headRefOid,headRepositoryOwner,isCrossRepository,isDraft,mergeable,mergeStateStatus
```

Require an open same-repository PR and use its head branch in the current or an isolated worktree. Fail closed on `isCrossRepository == true`. Fetch `baseRefName` from origin and inspect any delta before adopting a head moved by another actor. Never change the default branch.

Completion: an open PR exists and local `HEAD` equals its current `headRefOid`.

### 2. Pin the Head and Discover Gates

```bash
PR=<number>
OWNER_REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
HEAD_SHA=$(gh pr view "$PR" --json headRefOid -q .headRefOid)
test "$(git rev-parse HEAD)" = "$HEAD_SHA"
gh pr view "$PR" --json statusCheckRollup,reviews,reviewDecision,mergeable,mergeStateStatus
```

Capture required and observed check names rather than assuming every matrix lane.

Completion: the exact head, local gate state, GitHub CI state, remote review comments, and merge state are recorded.

### 3. Run the Local Remediation Loop

```bash
make ci
```

For a failure:

1. Identify the first root-cause failure from the command output.
2. Reproduce with the narrowest real command documented in `AGENTS.md`.
3. Add or identify a regression test before behavior changes.
4. Implement the smallest complete fix.
5. Run the focused test immediately after each modification.
6. Repeat `make ci` after focused checks pass.

Completion: `make ci` passes on the exact HEAD.

### 4. Commit, Recheck, and Push

Before each commit, inspect the diff for scope and secrets. Commit only intended files using imperative mood. Keep the subject under 72 characters and describe the user or maintenance outcome rather than implementation details.

A commit changes the exact head, so any pre-commit local pass is stale. Run the full gate again:

```bash
make ci
git rev-parse HEAD
```

Push only when the user's request authorizes updating the PR branch. For a new branch without an upstream, use `git push --set-upstream origin HEAD`; for an adopted tracking branch, use:

```bash
git push
NEW_HEAD=$(git rev-parse HEAD)
test "$(gh pr view "$PR" --json headRefOid -q .headRefOid)" = "$NEW_HEAD"
```

Every push invalidates prior local and remote terminal claims. Return to Step 2.

### 5. Wait for GitHub Actions and Diagnose

Use `gh pr checks "$PR" --watch --interval 10` for an efficient blocking wait on the exact-head CI matrix. If it exits non-zero, fetch exact-head workflow and job evidence with `gh run list`, `gh run view`, and `gh run view --log-failed`.

After CI passes, resolve the follow-on `Crush PR Review` run, then block on `gh run watch <review-run-id> --exit-status`. Do not inspect review comments until that run is completed/success.

Diagnose failures from the failed job log, not only the check summary. Reproduce and fix locally with `make ci`, commit, push, and re-bind.

If the failure is demonstrably external or flaky, retry the workflow only after collecting evidence; never repeatedly rerun a deterministic failure.

### 6. Reconcile Remote Review Comments

Fetch exact-head review events, inline comments, issue comments, and review threads. Build a ledger from GitHub PR comments containing source, URL, path and line, finding, disposition, and fixing evidence.

For each actionable comment:

1. Verify it against the current code and repository rules.
2. Fix valid findings with tests.
3. Run focused tests and full `make ci`.
4. Commit and push when authorized.
5. Re-bind all gates to the new head.
6. Reply with the fixing commit and test evidence, then resolve only the corresponding thread when allowed.

Human comments are outside this automated ledger unless the user asks to include them.

Completion: no actionable automated-review comment remains unresolved.

### 7. Perform the Final Atomic Bind

In one final pass, re-fetch local HEAD, PR head SHA, status checks, workflow runs, reviews, comments, threads, and mergeability. Require the full Terminal-Clean Contract.

Final verdicts:

- **TERMINAL CLEAN**: every required local, CI, review-comment, thread, and head-binding condition passes.
- **NOT CLEAN**: terminal evidence exists but one or more conditions fail.
- **BLOCKED**: a required external service, credential, permission, or safely adoptable head is unavailable after reasonable retries.

Do not merge by default.

## Pitfalls

1. Using the hosted `pr-shepherd` skill or `task pr:check`.
2. Waiting on remote CI instead of reproducing and fixing with local `make ci`.
3. Running `make ci` before a commit but not rerunning it after the commit changes HEAD.
4. Looking for a confidence score or generated Findings block. Crush review posts PR comments.
5. Treating a successful remote review workflow as proof that its comments are resolved.
6. Hard-coding the remote review account identity.
7. Stopping after a fix push without rebinding local gate, remote CI, review comments, and threads.
8. Retrying deterministic CI failures instead of fixing them.
9. Force-pushing over another actor's head movement.
10. Merging without explicit authorization.

## Verification Report

Report:

- repository and PR URL;
- exact terminal head SHA;
- exact-head `make ci` result;
- exact-head CI run URL and success;
- correlated `Crush PR Review` workflow-run URL and success;
- automated PR comments addressed with fixing commits and tests;
- unresolved actionable automated thread count;
- mergeability and merge state;
- final verdict: **TERMINAL CLEAN**, **NOT CLEAN**, or **BLOCKED**.

## GitHub Gate Binding

```bash
PR=<number>
OWNER_REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner)
HEAD_SHA=$(gh pr view "$PR" --json headRefOid -q .headRefOid)

gh pr view "$PR" --json \
  number,url,state,headRefOid,statusCheckRollup,reviews,reviewDecision,mergeable,mergeStateStatus

gh api "repos/$OWNER_REPO/commits/$HEAD_SHA/check-runs" \
  --jq '{total:.total_count,runs:[.check_runs[]|{id,name,status,conclusion,details_url,head_sha}]}'
```

Require exact-head required checks to be `completed` and `success`. After the exact-head CI run completes, list review runs:

```bash
gh run list --workflow "Crush PR Review" --event workflow_run --limit 20 \
  --json databaseId,name,event,status,conclusion,url,headSha,createdAt,updatedAt
```

Select the run created immediately after the exact-head CI run completed. Require the review or comments created during that run to target this PR and, for a review event, to have `commit_id == HEAD_SHA`.

Immediately before a terminal-clean report:

1. Read local `HEAD`.
2. Fetch PR `headRefOid`.
3. Fetch exact-head check runs and workflow runs.
4. Fetch reviews, inline comments, issue comments, and all review threads.
5. Fetch `mergeable`, `mergeStateStatus`, and `reviewDecision`.
6. Fetch PR `headRefOid` again.

Abort the clean verdict if either remote head differs from local `HEAD` or if the two remote reads differ.
