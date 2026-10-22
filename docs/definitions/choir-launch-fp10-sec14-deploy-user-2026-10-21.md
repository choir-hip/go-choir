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
  source_ref: lane tip after the bunch 4b repair (frozen predecessor 646f6488a)
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
    what: dedicated user + forced command + one sudo rule (runAs root) to a gate that runs the deploy program from a main commit at or above a host floor it only raises; deploy 1 adds it, deploy 2 switches CI, step 3 removes root's CI key
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
  next_action: FP0 rebases the two floor commits into the deploy-1 group, re-reviews the delta (scripts/node-b-ci-gate, its test, the probe), lands deploy 1 and runs the probe

receipts: []
---

# FP10: CI deploys as choir-deploy

Problem record: `docs/problems/ci-deploys-to-node-b-as-root-2026-10-21.md`.

## Rollout for FP0 (exact steps)

Use the rebased identities on FP0's frozen 646f6488a. The repair commits
(this lane, after 2e5c45442) are the test commit "the CI gate refuses
commits below its host floor" and the fix commit "the CI gate keeps a host
floor". **Both join the deploy-1 group**: rebase them onto 0fca75329, before
7637ad240. They apply cleanly to the deploy-1 tree, and every CI test passes
there.

| group | commits | what |
|---|---|---|
| deploy 1 | ae4791f36 (docs), 707226549 (tests), 0fca75329, + floor test, + floor fix | choir-deploy user, gate with host floor, sudo rule; programs moved; CI still root |
| deploy 2 | cafbfbe2f (tests), 7637ad240 | CI uses choir-deploy and gate requests; the first gate deploy writes the floor |
| step 3 | d948c3ecd | root drops the CI key; its deploy raises the floor to step 3 or above |

**Deploy 1.**
1. Push. The classifier selects a host OS deploy (`nix/node-b.nix`,
   `scripts/node-b-ci-gate`).
2. In the deploy log, the deploy runs as root from
   `.github/scripts/node-b-ci-deploy` and ends with "Staging deploy
   complete". Then check health and the receipt commit.
3. On Node B as root, from `/opt/go-choir`, run
   `scripts/node-b-ci-gate-probe <off-main-sha> <deploy-1-main-sha>`.
   - `<off-main-sha>`: any full SHA GitHub serves that is not on main (a
     `lane/*` tip).
   - Every line must say PASS (exit 0). That covers:
     - the single restricted key;
     - one `(root) NOPASSWD` sudo rule, to the gate;
     - the receipt read;
     - five refusals, one of them the off-main commit;
     - `sudo bash` refused;
     - a real prebuild through the gate.
   - No floor file exists yet; the seed applies.
4. Rollback: `git revert` the group. Root still deploys.

**Deploy 2.**
1. Push. No deploy is selected (only `.github/`). The deploy-impact log must
   say "Read Node B deploy receipt", not "unavailable".
2. Run a `workflow_dispatch` on main (`force_staging_deploy` true,
   `refresh_active_computers` false). It holds the activation lease.
3. In the log, look for `node-b-ci-gate: deploy <sha> as root from
   .github/scripts/node-b-ci-deploy` and "Staging deploy complete".
   `/var/lib/go-choir/deploy-receipt.json` names the dispatch run. Also,
   `/var/lib/go-choir/ci-gate-floor` holds the dispatched commit.
4. On Node B, check the logs:
   - `journalctl -t sudo --since -1h` shows `choir-deploy : ...
     COMMAND=/nix/store/...-go-choir-ci-deploy-gate -- deploy ...`;
   - `journalctl -u sshd` shows `Accepted publickey for choir-deploy`.
5. Rollback: `git revert` the group. The revert is forward (above the floor),
   and root still has the CI key.

**Step 3**, after deploy 2's proof.
1. Push. The host OS deploy runs through the gate and raises the floor to the
   step-3 commit.
2. On Node B, `/etc/ssh/authorized_keys.d/root` holds only the operator key.
3. Check `/root/.ssh/authorized_keys` for a hand-placed
   `github-actions-deploy@go-choir` line, and remove it if present.
4. Run `scripts/node-b-ci-gate-probe <off-main-sha> "" <deploy-1-main-sha>`.
   The new lines must PASS:
   - the host floor exists;
   - a commit below the floor is refused (sent as a prebuild, never a
     deploy);
   - the floor is unchanged.
   Deploy 2's sha (7637ad240's landed identity) must be refused the same way.
5. Rollback: `git revert`, which is forward, so above the floor.

**Step 4 (recommended follow-up, green-sized red):** set `seed_floor` in
`scripts/node-b-ci-gate` to the landed step-3 sha. The floor then holds even
if `/var/lib/go-choir/ci-gate-floor` is lost.

## Residuals

- Floor file loss: if `/var/lib/go-choir/ci-gate-floor` is deleted, the seed
  (961588290) applies, and it is below deploy 1, so the escape reopens. Step 4
  closes this.
- Booleans: a key holder can choose them, including
  `REFRESH_ACTIVE_COMPUTERS=true`, which interrupts busy computers.
- Root still runs the deploy program: a NixOS switch needs root. The gate
  narrows who picks the program.
- Pre-move commits: the gate refuses commits from before the move (and below
  the floor), so a rollback to them is the owner's by hand. The rollback path
  is git revert (forward).
- Stdin (N2): the program is run as `bash -s` from a file, which keeps the
  heredoc's transport. `bash "$program" </dev/null` would stop a future
  stdin read swallowing script lines. Deferred, because it can't be tested
  off Node B.

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

## Delta re-review (Gemini 3.8 flash, ffde1b3d8): findings and dispositions

**F1 (blocking): concurrent gate runs can lower the floor. Accepted.**
- Evidence: `scripts/node-b-ci-gate` at ffde1b3d8 has no lock. `check_floor`
  and `advance_floor` are separate steps, and `advance_floor` writes the
  requested commit without re-reading the file.
- CI's `staging-deploy-node-b` concurrency group does not help: a stolen key
  calls the gate directly.
- Two deploys that both descend from the floor can interleave. Step 3 is
  written, then deploy 2 overwrites it, and deploy 2's activation puts the CI
  key back on root.
- Fix:
  - one exclusive `flock` on `/var/lib/go-choir/ci-gate-floor.lock`, held
    from before `check_floor` through the advance and the deploy run;
  - the advance is a compare-and-swap: under the lock, the file must still
    hold the floor that was checked, and the new commit must descend from it.

**F2 (blocking): losing the floor file falls back to the seed and reopens
the escape. Accepted.** It was this file's own residual.
- Evidence: `check_floor` at ffde1b3d8 falls back to `seed_floor=961588290`,
  which is an ancestor of 0fca75329 and 7637ad240, and both of those put the
  CI key back on root.
- Fix: the gate drops the seed. With no floor file it refuses every
  prebuild and deploy. The one exception is a bootstrap marker,
  `/var/lib/go-choir/ci-gate-floor-bootstrap`:
  - Only a NixOS activation script in nix/node-b.nix writes it, and only
    while the floor file is missing.
  - The gate never writes it. The gate deletes it on its first deploy.
  - Step 3 removes that activation script. Every commit that installs the
    marker therefore sits below the floor once step 3 is deployed. After
    that, losing the floor file means every request is refused until root
    restores the file by hand.

**Non-blocking, stdin: fixed** (requested). The program runs as
`bash <file> </dev/null`. Neither program uses `$0` or `BASH_SOURCE`
(grep: no hits).

**Non-blocking, the floor rises before the run: residual (by design).** A
failed deploy leaves the floor raised, so rollback is forward, with git
revert.

Prebuild checks the floor but takes no lock: it neither activates nor
advances the floor. Locking it would also serialize it behind a running
deploy, which defeats its purpose.
