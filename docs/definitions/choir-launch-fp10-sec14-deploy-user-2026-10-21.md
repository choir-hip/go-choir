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
  status: working
  slice: problem record, then tests, then deploy-1 and deploy-2 commits
  source_ref: 961588290
  deploy_identity: c893a956a
  candidate:
    id: FP10-sec14-deploy-user
    state: rehearsing
    ref: lane/fp10-sec14-deploy-user
    base: 961588290
    digest: none
    scope: [nix/node-b.nix, scripts/node-b-ci-gate, .github/workflows/ci.yml, .github/scripts]
  conjecture:
    id: C-SEC14-gate
    claim: the three CI remote commands fit a three-verb grammar, and the deploy needs nothing from the root ssh session that the gate's clean environment lacks
    test: deploy 2's CI run succeeds for its deploy class through the gate
    edge: missing_oracle
    delta_o: FP0 runs the gate as choir-deploy on Node B before deploy 2 (receipt, then prebuild of the deployed commit)
    scope_if_supported: every deploy class
    status: proposed
    evidence_refs: []
  decision:
    what: dedicated user + forced command + one sudo rule to a gate that runs the deploy program from a main commit; two deploys, root key removed third
    kind: safety
    status: proposal
    evidence_ref: docs/problems/ci-deploys-to-node-b-as-root-2026-10-21.md
    owner_ratification_ref: not_applicable
  belief:
    believed_state: CI's ssh is root with the whole deploy on stdin
    main_uncertainty: whether the root ssh session carried environment the deploy silently relies on
    next_observation: the gate run by FP0 on Node B between deploy 1 and deploy 2
  blocker_or_risk: none
  next_action: write the gate test and contract-test changes, then the gate

receipts: []
---

# FP10: CI deploys as choir-deploy

Problem record: `docs/problems/ci-deploys-to-node-b-as-root-2026-10-21.md`.
