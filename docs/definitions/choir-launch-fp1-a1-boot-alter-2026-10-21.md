---
definition_version: 4

# Station FP1 follow-up A1 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): boot stops altering
# og_objects. Authority: FP0's adjudication of the GPT-6.1-sol panel's
# finding A1 on the FP1 candidate (2026-10-21), the metamission's
# must_preserve "No boot-path step that rewrites or scans all of og_objects;
# schema changes are online and rollback-safe", and the problem record
# docs/problems/boot-alters-og-objects-when-older-columns-missing-2026-10-21.md.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commits to the batch panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:05:00Z
  source:
    canonical_ref: main 53a691be6
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /home/agent/lanes/fp1-a1-boot-alter
      status: clean
      class: goal_candidate
      owner: FP1-A1
      touch: goal_owned
      recovery: branch lane/fp1-a1-boot-alter, created from 53a691be6

finish:
  deliver: >-
    No boot of a guest runs DDL on an og_objects table that has rows: boot
    reads information_schema, records which derived columns and indexes
    exist (the live set), and every read whose path is missing keeps its
    JSON fallback. An empty table still takes the whole schema at open (no
    rows, nothing to rewrite). Adding the missing columns and indexes is an
    explicit per-computer operation, POST
    /internal/store/derived-columns?action=migrate on the guest (host peer
    only), that runs one column or index at a time with the engine released
    between steps, logs each step, reports progress on GET, and can be
    cancelled.
  artifact: >-
    Commits on lane/fp1-a1-boot-alter: the problem record (cherry-picked
    adae3e779) and this file (green); one red commit with the tests
    (written and committed with the code they drive, so every commit
    builds), the discovery-only boot, the migration and its endpoint.
  acceptance:
    - action: >-
        go test ./internal/objectgraph -run 'TestBootDiscovery|TestDerivedColumn|TestRecordColumns|TestGeneratedColumn'
        and go test -race ./internal/store -run TestDerivedColumnMigration
        and go test ./internal/autoputer -run TestDerivedColumnsEndpoint
        (and the three packages whole, which pass on the lane)
      proves: >-
        a populated store missing the columns (record and legacy), or
        missing only an index, opens and boots with og_objects' table hash
        unchanged and reads correct on JSON; the migration adds them step
        by step, is idempotent, refuses a second concurrent run, survives
        cancellation with the store usable; the endpoint refuses non-host
        callers
      evidence_class: focused tests
    - action: >-
        FP0, after the deploy: read the boot log of the paper (and of one
        test computer) for `objectgraph dolt: derived access paths`;
        GET /internal/store/derived-columns from Node B on a test computer
        whose store lacks a column, then POST ?action=migrate and poll GET
        until state=done
      proves: boot is discovery-only on the deployed path and the operation
        completes on a real computer with progress
      evidence_class: deployed proof (FP0's)

boundaries:
  mutation_class: red (object store schema, boot path)
  protected_surfaces: [the object store schema, the guest boot path, guest /internal routes]
  must_preserve:
    - No boot-path step that rewrites or scans all of og_objects.
    - Reads stay correct on every store shape (JSON fallback where a path is missing).
    - A fresh (empty) store still gets every derived column and index at open.
    - The migration never runs without an explicit host-peer request; no boot-time resumer.
  rollback: >-
    git revert of the red commit. The migration's columns are additive
    generated columns older binaries never name; a computer that migrated
    keeps them and an older binary finds them present (its ensure step is
    an information_schema read). A computer that did not migrate is in the
    shape it was in before, and an older binary would again ALTER it at
    boot: the revert brings the problem back, not a new one.

fp0_steps: >-
  Land: the red commit is the batch's store-schema change; it touches
  internal/objectgraph/dolt_store.go, internal/store (store.go,
  derived_columns.go), internal/autoputer (run.go, derived_columns.go) and
  tests. Deployed proof, from Node B (the endpoint answers the host peer
  only, so no credential is needed; GUEST is the guest's tap address and
  runtime port as for /internal/boot/timeline):
  (1) after the deploy, the paper's and one test computer's boot logs show
  `objectgraph dolt: derived access paths complete` or `... missing on a
  table with rows=true, not added at open`, and no `derived access path
  step` line; the paper's boot reaches replay_done in its usual time.
  (2) `curl -fsS http://$GUEST/internal/store/derived-columns` on the paper
  is state=idle with an empty missing list (it migrated on 85d2295b7).
  (3) On a disposable test computer whose store lacks a column (none may
  exist; a restored pre-2026-10-14 checkpoint is one), GET shows the plan,
  `curl -fsS -X POST 'http://$GUEST/internal/store/derived-columns?action=migrate'`
  answers 202, GET is polled until state=done, and the boot log has one
  `derived access path step i/n ... done in` line per step. A second POST
  while running answers 409; `?action=cancel` stops before the next step.
  (4) From inside a guest (terminal), the same GET answers 403.
  Do not run (3) on the paper while it is busy: each step holds the
  engine for one rewrite of og_objects.

now:
  status: done (lane); landing and deployed proof are FP0's
  slice: boot discovery-only and the explicit migration endpoint, on lane/fp1-a1-boot-alter
  residuals:
    - The migration is online in that the computer stays up and works
      between steps, not within one: a step on a large table holds the
      engine for one rewrite (about 50 us per row). A truly online add
      (empty side table, writer maintenance, paged backfill and validator,
      as FP1's derived-key table) is the substrate fix if a large computer
      ever needs these columns; none known does.
    - HostPeerCaller still admits TEST-NET-1 sources (pre-existing, shared
      with /internal/diag/tcp-dial).
    - A Reopen while a migration runs cancels it; the step in flight
      finishes on the old handle.
