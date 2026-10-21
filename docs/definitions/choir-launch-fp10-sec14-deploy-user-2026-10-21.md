---
definition_version: 4

# FP10, SDC SEC-14 second half: CI deploys to Node B through a dedicated
# user whose key runs only a host-side gate. Station of
# docs/definitions/choir-launch-first-pass-metamission-2026-10-21.md.
readiness: executable

review:
  reviewer: none (red candidate; FP0's bunch panel reviews it frozen)
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T22:00:00Z
  source:
    canonical_ref: lane/fp10-sec14-deploy-user at 961588290 (main)
    deploy_identity: c893a956a serving (metamission now card)
  worktrees:
    - path: lanes/fp10-sec14-deploy-user
      status: clean
      class: goal_candidate
      owner: FP10 lane worker
      touch: goal_owned
      recovery: lane branch on origin

finish:
  deliver: >-
    A stolen CI deploy key can no longer run arbitrary commands as root on
    Node B. At most it can deploy, prebuild or read the receipt for a commit
    that is already on main.
  artifact: >-
    The choir-deploy user on Node B (nix/node-b.nix). Its only authorized key
    is the CI key with restrict and a forced command. That command passes the
    ssh request to scripts/node-b-ci-gate through one sudo rule. The gate
    accepts three verbs (receipt, prebuild, deploy) with typed arguments. It
    checks that the commit is an ancestor of origin/main, then runs
    .github/scripts/node-b-ci-{prebuild,deploy} as read from that commit.
    ci.yml connects as choir-deploy; host-key pinning (6bfe06fcf) is kept.
  acceptance:
    - action: .github/scripts/node-b-ci-gate-test and deploy-workflow-contract-test in CI
      proves: the gate's parser refuses every request outside the grammar, and no workflow ssh uses root or sends a script on stdin
      evidence_class: CI test
    - action: deploy 2's own CI run (receipt read, prebuild, deploy all through choir-deploy), then FP0's refusal probes on Node B
      proves: the deploy works through the gate, and the gate refuses a non-main commit, a shell request and an unknown verb
      evidence_class: deployed proof
  rollback: >-
    Deploy 2 (the CI switch): git revert. Root still accepts the CI key until
    step 3, so the revert's run deploys as root. Deploy 1 (the user and the
    gate): git revert; nothing uses the user yet.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: minimize what holding the CI deploy key lets you run on Node B, while every deploy class (frontend, service pointer, host OS, active refresh) still lands
  goodharting_would_be: a non-root login whose sudo rule allows bash, or a gate that runs the script it receives on stdin

homotopy:
  realism_axis: parser tests -> gate on Node B exercised by FP0 as choir-deploy -> deploy 2's real CI run through the gate -> root key removed

boundaries:
  mutation_class: red
  authority_sources:
    - the metamission's FP10 entry (SEC-14 second half) and the lane brief of 2026-10-21
    - CLAUDE.md, docs/choir-doctrine.md
  must_preserve:
    - host-key pinning (NODE_B_KNOWN_HOSTS, HostKeyAlias, StrictHostKeyChecking yes)
    - the deploy program's behavior; the move out of the heredocs is byte-for-byte
    - root accepts the CI key until deploy 2 is proven, so a revert can always deploy
  excluded:
    - Node B access, Nix builds, live probes (FP0's)
    - a non-root deploy program (a NixOS switch needs root; the gate narrows who picks the program)
    - downgrade refusal on the host (residual)
  protected_surfaces: [deployment routing, Node B ssh access, sudo]

now:
  status: checkpoint_incomplete
  slice: built on the lane; landing, the deploys and the proofs are FP0's (steps below)
  source_ref: 3fe19b0e9
  deploy_identity: c893a956a
  candidate:
    id: FP10-sec14-deploy-user
    state: rehearsing
    ref: lane/fp10-sec14-deploy-user
    base: 961588290
    digest: none
    scope: [nix/node-b.nix, scripts/node-b-ci-gate, scripts/node-b-ci-gate-probe, .github/workflows/ci.yml, .github/scripts]
  conjecture:
    id: C-SEC14-gate
    claim: the three CI remote commands fit a three-verb grammar, and the deploy needs nothing from the root ssh session that the gate's clean environment lacks
    test: deploy 2's dispatch run succeeds through the gate
    edge: missing_oracle
    delta_o: scripts/node-b-ci-gate-probe on Node B between deploy 1 and deploy 2, including a real prebuild through the gate
    scope_if_supported: every deploy class
    status: proposed
    evidence_refs: []
  decision:
    what: dedicated user + forced command + one sudo rule (runAs root) to a gate that runs the deploy program from a main commit; deploy 1 adds it, deploy 2 switches CI, step 3 removes root's CI key
    kind: safety
    status: proposal
    evidence_ref: docs/problems/ci-deploys-to-node-b-as-root-2026-10-21.md
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      Every CI test passes at each landing point: the gate's grammar, the host
      wiring, the workflow contracts and the classifier. The moved programs
      match the heredocs byte for byte. Nothing has been built with Nix or run
      on Node B; this VM cannot do either.
    main_uncertainty: >-
      Whether the NixOS module wiring evaluates and behaves as written (sudo
      rule rendering, useDefaultShell for a system user, sshd with the
      restrict key). Also whether the deploy relied on anything in the root
      ssh session that /etc/set-environment does not restore.
    next_observation: deploy 1's host OS build and switch, then the probe
  blocker_or_risk: >-
    The lane branch is not on GitHub: the agents VM's token lacks the
    `workflow` scope, so GitHub refuses any push that changes
    .github/workflows/ci.yml. FP0 takes the commits from this worktree
    (lanes/fp10-sec14-deploy-user on the agents VM) or pushes them with a
    workflow-scoped credential. Deploy 2 touches only .github/, so no deploy is selected on push; it needs
    a workflow_dispatch deploy to prove it. The receipt read is best effort:
    a gate failure there is silent ("receipt unavailable"), so read that line.
  next_action: FP0 freezes and reviews deploy 1 (commits e892961cc, 67ef79140), lands it, runs the probe

receipts: []
---

# FP10: CI deploys as choir-deploy

Problem record: `docs/problems/ci-deploys-to-node-b-as-root-2026-10-21.md`.

## Rollout for FP0 (exact steps)

Commits, oldest first. Land them in three groups, each rebased onto main:

| group | commits | what |
|---|---|---|
| deploy 1 | f803872b9 (green docs), e892961cc (tests), 67ef79140 | choir-deploy user, gate, sudo rule; the heredocs move to `.github/scripts/node-b-ci-{prebuild,deploy}`; CI still root |
| deploy 2 | 11922ea8a (tests), 237d8a1c9 | CI uses choir-deploy and gate requests |
| step 3 | 3fe19b0e9 | root drops the CI key |

**Deploy 1.**
1. Push. The classifier selects a host OS deploy (`nix/node-b.nix`,
   `scripts/node-b-ci-gate`).
2. In the deploy log, the deploy runs as root from
   `.github/scripts/node-b-ci-deploy` and ends with "Staging deploy
   complete". Then check health and the receipt commit.
3. On Node B as root, from `/opt/go-choir`, run
   `scripts/node-b-ci-gate-probe <off-main-sha> <deployed-main-sha>`.
   - `<off-main-sha>`: any full SHA GitHub serves that is not on main, for
     example this lane's tip before landing, or any `lane/*` tip.
   - `<deployed-main-sha>`: the deploy-1 commit.
   - Every line must say PASS and the probe must exit 0. That covers the
     single restricted key, a single sudo rule `(root) NOPASSWD` to the gate,
     the receipt read, five refusals (one of them the off-main commit),
     `sudo bash` refused, and a real prebuild through the gate.
4. Rollback if the switch or the probe fails: `git revert` the group. Root
   still deploys.

**Deploy 2.**
1. Push. No deploy is selected (only `.github/` changed). Push itself proves
   only the receipt read: the deploy-impact log must say "Read Node B deploy
   receipt", not "unavailable".
2. Run CI by `workflow_dispatch` on main (`force_staging_deploy` true,
   `refresh_active_computers` false). It is a forced full deploy, so it holds
   the activation lease like any red deploy.
3. In the log, the deploy step shows
   `node-b-ci-gate: deploy <sha> as root from .github/scripts/node-b-ci-deploy`
   and "Staging deploy complete". The prebuild job shows the matching
   prebuild line. `/var/lib/go-choir/deploy-receipt.json` names the dispatch
   run id.
4. On Node B: `journalctl -t sudo --since -1h` shows
   `choir-deploy : ... COMMAND=/nix/store/...-go-choir-ci-deploy-gate -- deploy ...`.
   `journalctl -u sshd` shows `Accepted publickey for choir-deploy`, and no
   root login from the CI key after the switch.
5. Rollback: `git revert` the group. Root still has the CI key, so the next
   deploy goes as root.

**Step 3**, after deploy 2's proof. It is red, and it rides the next deploy or
gets its own.
1. Push. The classifier selects a host OS deploy, which runs through the gate.
2. On Node B, `/etc/ssh/authorized_keys.d/root` holds only the operator key.
3. Check `/root/.ssh/authorized_keys` for a hand-placed copy of the
   `github-actions-deploy@go-choir` key. If it is there, remove that line (an
   on-host step for FP0 or the owner).
4. Rollback: `git revert`. Its deploy also runs through the gate.

## Residuals

- Downgrade: a key holder can deploy any older main commit that carries
  `.github/scripts/node-b-ci-deploy`. CI's stale-guard covers honest reruns.
  A host-side check would compare against the receipt's target_commit.
- A key holder can choose the booleans, including
  `REFRESH_ACTIVE_COMPUTERS=true`, which interrupts busy computers.
- Root still runs the deploy program. A NixOS switch needs root; the gate
  narrows who picks the program, not what the program may do.
- The gate refuses commits from before the move (they lack the program), so
  rolling back to such a commit means the owner deploys by hand.


## Review bunch 4b (frozen 646f6488a): findings and dispositions

Source: `~/runs/sec14-findings.md` (GPT-6.1-sol send back; Gemini 4 Argon
accept). FP0's rebase maps this lane's shas in order: deploy 1 = 0fca75329,
deploy 2 = 7637ad240, root-key removal = d948c3ecd.

**B1 (blocking, GPT-6.1-sol): an allowed ancestor deploy restores root's CI
key. Accepted.**
- Evidence: the gate's only commit check is `merge-base --is-ancestor
  <commit> origin/main` (`scripts/node-b-ci-gate`, `run()`). The deploy-1
  commit (lane 67ef79140, rebased 0fca75329) carries both the deploy program
  and `ciDeployKey` in root's key list (`git show 67ef79140:nix/node-b.nix`,
  line 532). Deploy 2 (7637ad240) carries both as well.
- Attack: after step 3, a stolen key sends `deploy DEPLOY_COMMIT=0fca75329...
  DEPLOY_HOST=true DEPLOY_HOST_OS=true HOST_SERVICES=`. The request is
  accepted, the NixOS switch restores root's CI key, and the key holder has a
  root shell. CI's stale-guard is never consulted on a direct gate request.
- The residual "downgrade is an availability risk" in this file was wrong:
  here a downgrade restores arbitrary root.
- Fix, to land with deploy 1 (it changes only the gate): the gate keeps a
  host floor in `/var/lib/go-choir/ci-gate-floor`.
  - Before running a deploy, the gate raises the floor to the requested
    commit.
  - The gate refuses any prebuild or deploy commit that is not a descendant
    of the floor. A missing file falls back to a seed main commit written in
    the gate. A malformed floor, or a floor git can't resolve, is refused.
  - The floor can't be lowered through the gate: the gate writes only a
    commit that already descends from the current floor. Once step 3 is
    deployed through the gate, the floor is at or above step 3, so no
    accepted commit restores root's key.

**N1 (non-blocking, both): the rollout table names pre-rebase shas.
Accepted.** The rollout below is remapped to the rebased identities.

**N2 (non-blocking, Argon): run the program as `bash "$program" </dev/null`
so a future stdin read can't swallow script lines. Residual.** Today no
command in either program reads stdin (Argon checked), and `bash -s` from a
file keeps the transport the heredoc had. The change is untestable off Node
B, so it waits.

**N3 (Argon's landing constraint): the three groups land in separate
sequential deploys. Already the rollout.**
