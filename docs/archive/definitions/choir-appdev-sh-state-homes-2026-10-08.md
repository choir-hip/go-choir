---
definition_version: 4

# SH — State homes (operational invariant O21: a realization holds no unique
# state). Station of the supervised app-development metamission. Owner
# direction 2026-10-08: "Yes, good policy and good ideas. Let's do it" — in
# reply to the proposal to ratify O21, inventory state homes, land the two
# safe boot fixes, and deliver the escrowed privacy key to a new realization.
# Owner added: the desktop app (Wails, out of current scope) must be able to
# move a computer hosted <-> local and back; O21 is its prerequisite.
readiness: drafted

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-08T23:59:00Z
  source:
    canonical_ref: main 2a5d6166
    deploy_identity: >-
      staging choir.news deployed 85befb1c (corpusd 85befb1c; checkpointd
      67729e07+; vmctl 0f7c58ba)
  worktrees:
    - path: /Users/wiz/go-choir skills/agentic-consensus/*
      status: dirty
      class: other_agent_wip
      owner: unknown (pre-existing)
      touch: forbidden
      recovery: leave in place
    - path: /Users/wiz/go-choir docs/deck/*
      status: dirty (untracked)
      class: user_wip
      owner: owner or other agent (appeared 2026-10-08)
      touch: forbidden
      recovery: leave in place
  observed:
    - >-
      Recovery admission staging acceptance (2026-10-08): a fresh realization
      of disposable computer-f177676f crash-looped on a missing privacy key
      and vmctl waited 30m blind
      (docs/problems/fresh-realization-missing-privacy-key-blind-boot-2026-10-08.md).
    - >-
      State homes inventory (docs/state-homes-inventory-2026-10-08.md):
      privacy key and capsule artifacts exist only on the realization; files
      have a 15-minute loss window; updater `current` and signer trust
      unverified.

finish:
  deliver: >-
    Losing a realization loses nothing. Any computer can be realized again
    from tape, content store and escrow with no operator step, and a start
    that cannot succeed is refused before boot with a typed reason instead
    of a blind wait.
  artifact: >-
    (1) Typed pre-boot refusal `privacy_key_unavailable` in vmctl recovery
    admission, and prompt typed reporting of guest fatal-startup errors so
    the host stops waiting. (2) Guaranteed custodian escrow before a key is
    a computer's only copy. (3) Escrow-to-realization key delivery on the
    root-only credential disk, bound to ComputerID + realization epoch,
    consume-once, transparency-logged before unwrap; deletion of the debugfs
    copier. (4) Capsule subject/receipt bytes content-addressed into the
    platform store before refs commit. (5) Files sync barrier before every
    planned realization change.
  acceptance:
    - action: >-
        Disposable on staging with no escrow-deliverable key (pre-slice-3):
        remove the realization and request resolve.
      proves: >-
        typed 503 `privacy_key_unavailable` within seconds, durable across a
        vmctl restart, no boot; any guest fatal startup error ends the start
        attempt promptly with its reason recorded.
      evidence_class: deployed proof (disposable)
    - action: >-
        Lose-the-disk disposable: create, use (files, capsule run,
        self-development apply), destroy the data disk, realize again.
      proves: >-
        identical effective head, content witness, files, capsule artifacts
        and release; key delivered from escrow with a transparency entry; no
        debugfs or operator step.
      evidence_class: deployed proof (disposable)
  rollback: >-
    Revert station commits. Delivery is additive; with it reverted, the
    typed refusal holds the computer safely instead of crash-looping. Never
    rewrite tape, delete escrow records, or reveal a key as rollback.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the set of computer state whose only copy is a realization, and
    the time from an unsatisfiable start to a typed refusal, while keeping
    key access no wider than today's authorized holders.
  goodharting_would_be: >-
    Recreating a fresh key for an existing computer (silently orphaning its
    encrypted history); copying keys VM-to-VM or via debugfs; shortening the
    health timeout instead of reporting the cause; marking state
    "realization-scoped" to drop it from the inventory.

homotopy:
  realism_axis: >-
    Typed refusal on a disposable -> lose-the-disk disposable with escrow
    delivery -> owner computer re-realization -> desktop (local) realization
    of a hosted computer.

boundaries:
  mutation_class: red
  authority_sources:
    - owner statement 2026-10-08 ("good policy and good ideas. Let's do it")
    - docs/computer-ontology.md (realization is replaceable machine state)
    - docs/operational-invariants-register-2026-10-08.md (O21, O8, O9, O20)
    - key custody policy (custodian unwrap for verified jobs; human reveal two-approval)
  must_preserve:
    - no new party gains plaintext key access; human reveal stays two-approval
    - an existing computer never mints a replacement privacy key
    - tape is never rewritten; escrow records are never deleted
    - credentials never reach readable channels (O16)
  excluded:
    - desktop app placement implementation (recorded as the realism axis end)
    - owner-derived (passkey) protector and end-to-end custody
    - offline append for local realizations
  protected_surfaces:
    - privacy key custody (internal/keyescrow, internal/platform/key_escrow*, guest key load)
    - vmctl recovery admission and realization lifecycle
    - credential disk construction (internal/vmmanager)
    - capsule artifact authority

now:
  status: working
  slice: >-
    lose-the-disk proof PASSED on staging 14:45Z (Gate 1 exit test). Slices
    1-3 landed (typed key refusal; write-once escrow; escrow-to-realization
    key delivery). Files half: hydration on a seeded tree (c72c38c4),
    durable fresh-volume marker (799097e3), reboot-path-only deploys so new
    realizations run current code (a47122de). Receipt
    docs/evidence/sh-lose-the-disk-2026-10-09T14-44-09-081Z.json.
  source_ref: 32a1c2e8
  deploy_identity: 799097e3 (base image)
  candidate:
    id: none
    state: none
    ref: none
    base: none
    digest: none
    scope: []
  conjecture:
    id: C1-bridge
    claim: >-
      If every piece of computer state has a home outside the realization,
      recovery, host move and hosted<->desktop placement reduce to one
      construct-from-homes operation, and the realization-loss problem class
      stops recurring.
    test: lose-the-disk disposable proof; count new realization-loss problem docs.
    edge: missing_oracle
    delta_o: the state homes inventory plus the lose-the-disk proof
    scope_if_supported: staging computers
    status: proposed
    evidence_refs: [docs/state-homes-inventory-2026-10-08.md]
  decision:
    what: >-
      Deliver the escrowed privacy key to the same computer's new
      realization automatically (bound, consume-once, transparency-logged).
    kind: authority
    status: owner_ratified
    evidence_ref: docs/problems/fresh-realization-missing-privacy-key-blind-boot-2026-10-08.md
    owner_ratification_ref: owner statement 2026-10-08 ("Let's do it")
  belief:
    believed_state: >-
      vmctl can decide key satisfiability before boot from (previous data
      image present) OR (escrow record present and delivery available).
    main_uncertainty: >-
      Whether vmctl can see escrow presence without new authority, and how
      the guest reports fatal startup errors to the host today.
    next_observation: vmctl boot path and guest startup error reporting.
  blocker_or_risk: >-
    Red surface (custody, vmctl). Mitigated by disposable-first proof and
    refusing rather than guessing.
  next_action: >-
    Copier deleted (a72e2d32, deployed 15:22Z). Cold-recover proof on a
    disposable is not possible yet: the owner cold-recover API requires an
    immutable ComputerVersion route slot, which only a first
    self-development route transition creates (162 staging slots; a new
    account has none; 503 "immutable ComputerVersion route unavailable" at
    15:28Z). So cold recovery of a routed computer taking its key from
    escrow is unproven until Gate 2's self-development acceptance, which
    already includes lose-the-disk after a self-development apply; rollback
    is git revert a72e2d32. Residual: an unrouted computer has no
    owner-facing recovery for a broken disk (remove-and-resolve is
    internal). Remaining SH items (effective-head and transparency checks,
    slices 4-5) move to Gate 2's S stations, where capsules and releases
    first exist; Gate 1's SH exit (lose-the-disk) is met.
receipts:
  - docs/evidence/sh-lose-the-disk-2026-10-09T14-44-09-081Z.json
---

# SH — State homes

This station enforces operational invariant **O21** from the
[operational invariants register](../operational-invariants-register-2026-10-08.md):
a realization holds no unique state. The
[state homes inventory](../state-homes-inventory-2026-10-08.md) lists every
piece of guest persistent state with its durable home and verdict.

Slices, in order:

1. Typed refusal and prompt fatal-startup reporting (safe fixes).
2. Guaranteed escrow before a key is the only copy.
3. Escrow-to-realization key delivery; delete the debugfs copier.
4. Capsule artifacts into the content store.
5. Files sync barrier before planned realization changes; lose-the-disk proof.

The desktop app's hosted↔local move is the end of the realism axis. It needs
two more decisions recorded in the inventory but excluded here: a key
protector for an unattested local machine, and offline appends.

## Slice 3 design — escrow-to-realization key delivery (proposal, for panel)

Owner-ratified intent: a new realization of an existing computer receives
the computer's privacy key from custodian escrow, automatically, bound,
consume-once, transparency-logged before unwrap. No new party gains
plaintext access.

**Binding.** A realization is minted when vmctl asks corpusd to issue a
credential envelope `(ComputerID, RealizationID, epoch, nonce,
request_commitment)`; the guest exchanges it once for its capability
(`credential_envelope_issued` / `_consumed` lifecycle receipts). The guest
capability itself carries no realization identity. Delivery is therefore
bound to the envelope's `request_commitment`, which names exactly one
realization issuance.

**Flow.**

1. Guest boot, chain exists, key file absent: generate an ephemeral X25519
   keypair (in memory only).
2. `POST /internal/computers/keys/realization-delivery` with
   `{computer_id, request_commitment, recipient_public_key}`, authorized by
   the guest capability for that computer (current revocation epoch).
3. corpusd: require exactly one `credential_envelope_issued` record for
   `(computer_id, request_commitment)`; insert a consume-once delivery row
   keyed by `(computer_id, request_commitment)` (a replay with the same
   recipient key returns the same sealed result; a different recipient key
   is refused); append a key-escrow transparency entry
   `{type: realization_delivery, computer_id, request_commitment, key_digest}`
   **before** unwrap; open the custodian wrap; seal the DEK to the
   recipient key; return the sealed wrap. Plaintext exists only inside
   corpusd's process and the guest.
4. Guest opens with the ephemeral private key, checks the digest against
   the escrow digest, writes the canonical key file (mode 0400, `O_EXCL`),
   and continues boot. Any failure is the typed `privacy_key_unavailable`
   refusal.
5. vmctl admission: a fresh realization of an existing chain is admitted
   when a custodian escrow record exists (read from corpusd's escrow
   status, internal caller); otherwise the slice-1 refusal stands, and a
   durable `privacy_key_unavailable` condition reopens when escrow appears.

**Deletion.** Once proved on a lose-the-disk disposable,
`internal/vmctl/trusted_guest_copier.go` (debugfs copy from quarantined
images) and its cold-recover phase are deleted.

**Alternatives considered.** (a) Deliver inside the credential exchange
response — tighter binding but changes the exchange/consumption protocol
(red auth surface). (b) vmctl writes the key on the credential disk —
puts plaintext in a host service that does not otherwise need it and on a
disk image. (c) Operator-only reveal — rejected by owner (O21).

**Open questions for review.** Whether `request_commitment` binding is
sufficient against a guest replaying another realization's commitment
(it would need that realization's capability for the same computer —
same trust domain); whether delivery should also be recorded on the
canonical tape; desktop (unattested local) realizations are excluded.

### Slice 3 design — revision after panel (2026-10-09)

Panel ([evidence](../evidence/sh-panel-review-2026-10-08.md)): approve with
changes. Revised binding and recovery:

- **Envelope-possession proof, not capability alone.** The delivery request
  carries the envelope `nonce`. corpusd already derives it as
  `credentialPRF(seed, "nonce", request_commitment)`, so only the holder of
  that realization's envelope (and the issuer) knows it. The request is
  valid only for the **current** issuance of the computer (no later
  `credential_envelope_issued`) and with the guest capability for the same
  computer. The nonce never enters a log; the transparency entry records
  `request_commitment` and the recipient key digest.
- **Crash-safe re-delivery.** Not hard consume-once: a request with a valid
  nonce proof may be re-delivered to a new recipient key (a guest that
  crashed after delivery), each delivery transparency-logged before
  unwrap. Only the envelope holder can ask, so re-delivery widens nothing.
  Delivery happens right after the credential exchange, before the
  envelope is erased.
- **Verify against the tape.** Before writing the key, the guest decrypts
  one private artifact named by the tape (or the projection base) with the
  delivered key; a mismatch is a typed `privacy_key_mismatch` refusal and
  an alarm — escrow holds the wrong key. The file is written temp → fsync →
  rename.
- **Transparency in the same transaction** as the delivery record, before
  unwrap.
- **Tape.** Delivery is custody, not computer state: the transparency log
  is the authority. After recovery the guest appends a non-secret
  `privacy_key_delivered` lifecycle receipt citing the transparency entry,
  so the owner sees it.
- **Admission.** vmctl reopens a durable `privacy_key_unavailable`
  condition (and admits a fresh realization) when the custodian escrow
  exists; a delivery failure in the guest is reported with its reason.

Alternative (a), delivery inside the exchange response, remains the
stricter long-term binding; the nonce proof gives the same caller binding
without changing the exchange protocol.
