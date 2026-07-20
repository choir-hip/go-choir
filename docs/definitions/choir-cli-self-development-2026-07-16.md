---
title: "Make Choir Self-Developing — Capsule-Scoped Audited Work"
definition_version: 2

start:
  captured_at: 2026-07-18T06:04:32Z
  source:
    canonical_ref: refs/heads/main@a36ebb08b024d74c06c3124c49c46e5acc4d2b63
    origin_ref: refs/remotes/origin/main@a36ebb08b024d74c06c3124c49c46e5acc4d2b63
    relation: canonical_ref_equals_origin_ref
    deploy_identity: "choir.news /health reported 9d9945e65f5b54069e1a86a530cb0960d96b3474 at authoring; source/deploy mismatch must be reconciled before deployed acceptance"
  worktree_inventory:
    path: /Users/wiz/go-choir
    status: clean
    branch: main
    preservation_rule: "Preserve unrelated worktrees, owner computers, accepted ComputerVersions, rollback realizations, and production recovery images. Build from immutable inputs; never import an unreviewed worker branch or host-local VM state."
  observed_artifact:
    - claim: "Audited construction is complete: served staging computers use vmctl-owned ComputerVersion route CAS and can be constructed, verified, rolled back, reconstructed, and inspected without SSH."
      evidence_ref: docs/evidence/audited-construction-terminal-receipt-2026-07-17.md
    - claim: "The supported choir CLI exposes generic run, trajectory, current-computer lifecycle/status, and API-key operations. It has no self-development transaction, approval, materialization, rollback, receipt, or explicit target-computer product contract; run list/cancel still call forbidden /api/agent routes."
      evidence_ref: cmd/choir/main.go:494-662
    - claim: "Super, CoSuper, and VSuper currently receive direct writable/coding tools; Super receives VM-control tools. Self-development still delegates effects through worker/package/lineage paths."
      evidence_ref: "internal/agentprofile/agentprofile.go:88-123; internal/agentcore/tool_profiles.go:382-426; internal/agentcore/tools_vmctl.go"
    - claim: "Capsule role/capability and transaction components exist, but production isolation is scaffolded: Executor.Spawn omits namespaces/cgroups/overlayfs/broker, Destroy cleanup is TODO, capsule exec/file tools return not_implemented, and registration has no production caller."
      evidence_ref: "internal/capsule/executor.go:46-90; internal/capsule/capsule.go:140-162; internal/agentcore/tools_capsule.go:385-516"
    - claim: "Trace is an observation projection, EventRecord is per-run, TransactionTape is in-memory, and no canonical per-computer event ledger or typed guest updater exists."
      evidence_ref: "internal/trace/store.go; internal/types/task.go:375-421; internal/capsule/transaction/tape.go; repository symbol inspection"
  problem:
    classification: substrate
    statement: "Choir cannot yet develop itself honestly. Direct core-VM tools and worker paths remain live; capsule isolation/broker/cleanup are scaffolds; no canonical cross-restart computer-event authority exists; complete trajectory retention lacks privacy semantics; no safe guest updater or public self-development CLI/API exists; current capability minting depends on an excluded host daemon; and the previous phase order required deployed proof before deployment."
    existing_fix_connection: "Reuse the existing ComputerVersion constructor/verifier, embedded Dolt, immutable artifact inputs, route CAS, trajectory/Trace observations, capsule types, transaction classifier, CLI client, and guest lifecycle. Replace—not wrap—the direct mutation, host-authority, worker/candidate-VM, package/lineage activation, and internal API paths."
  corrections:
    - corrected_at: 2026-07-18T06:04:32Z
      preserves_original_observation: true
      clarification: "The immutable start receipt remains the original authoring observation. Execution reconciliation uses the newer canonical base named in now; Definition v2 does not rewrite start history."
      evidence_ref: skills/definition/SKILL.md
    - corrected_at: 2026-07-18T18:43:30Z
      preserves_original_observation: true
      clarification: "Owner-settled cutover: the Choir VM is normally long-lived and reconstructible; every state mutation and agent trajectory is audited; super has no bash; cosuper shell/filesystem effects are capsule-scoped; researchers remain VM-local with typed safe writes; candidate VMs and host repair are excluded; promotion is an accepted event."
      evidence_ref: "Owner statements in this 2026-07-18 conversation"
    - corrected_at: 2026-07-18T19:26:00Z
      preserves_original_observation: true
      clarification: "Whole-mission consensus exposed additional source facts: capsule HostAuthority is host-side and unwired; VSuper remains effectful; the guest updater and public self-development API do not exist; complete trajectory privacy/migration/append protocols were underspecified; and C required deployed evidence before D deployed. The execution contract below repairs those blockers before code."
      evidence_ref: "/tmp/choir-selfdev-full-mission-round1 panel; source citations in observed_artifact"
    - corrected_at: 2026-07-18T22:17:41Z
      preserves_original_observation: true
      clarification: "Owner clarified that candidate VMs and worker VMs are obsolete product concepts, not merely forbidden self-development fallbacks. Their lifecycle/controller/tool/profile/prompt/API code must be deleted in this mission; no unrelated-worker-VM retention exception remains. Generic delegated agents may continue only through the durable agent/capsule model and are not worker VMs."
      evidence_ref: "Owner clarification in this 2026-07-18 conversation"

owner_ratification:
  status: settled
  settled_by: owner
  recorded_at: 2026-07-18T22:17:41Z
  delegation_ref: "The owner instructed that the entire mission, not only A0, be executable and iterated with a diverse panel including Claude until confirmed. The owner-settled product boundaries are stable ComputerID/event identity, promotion as accepted event, no-bash Super, capsule-scoped CoSuper effects, typed Researcher writes, complete deletion of candidate-VM and worker-VM concepts/code, reconstructibility, and vmctl as route actuator. Generic delegated agents are not worker VMs. The exact topology below remains under the owner-settled two-Dolt/D-ROUTE contract; it does not ratify a third store or alter vmctl's sole route-slot CAS."
  outcome: "One stable Choir computer develops itself through complete causal event audit, real capsule-scoped effects, external approval, typed guest materialization, supported public CLI/API control, deployed restart/reconstruction, rejection, and rollback."
  execution_contract:
    identity_and_promotion:
      - "Stable ComputerID plus its canonical event chain is the evolving computer. RealizationID is replaceable machine state."
      - "Promotion means appending an authorized acceptance event. A capsule transaction is inert until acceptance. Materialization, checkpoint creation, service restart, and route CAS are effects/projections, not alternate promotion authorities."
      - "ComputerVersion=(CodeRef, ArtifactProgramRef) is an immutable reconstruction checkpoint at a canonical event head. For self-development, a candidate is a frozen CapsuleEffectBundle, never a VM, desktop, or candidate route."
      - "The current route slot may continue to point to the latest ComputerVersion projection while product identity remains ComputerID. vmctl alone actuates route CAS; it never writes or settles computer events."
    canonical_event_authority:
      owner: "Exactly one ComputerEventAppender in the trusted guest core validates and sequences semantic events for one ComputerID. Agents, capsules, Trace, vmctl, route tables, status rows, and reducers cannot append independently."
      durable_form: "Exactly two Dolt stores remain. `computer_event_heads` and append/idempotency receipts are narrow control tables on the existing corpusd world-wire sql-server beside—but semantically separate from—world-wire objects and route-slot tables. Immutable event bodies and payloads use the existing artifact/blob service. VM-local embedded Dolt is the event index and accepted-state materialization, never the sole durable copy. No third database, route-equivalent row, or new host service exists."
      credential_delivery: "At each realization build, ordinary vmctl/constructor materialization creates a dedicated realization-local ext4 credential partition containing one root-owned mode-0400 ComputerCredentialEnvelope; Firecracker attaches it as a separate writable boot drive. The credential never enters CodeRef, ArtifactProgramRef, data artifacts, persistent VM config, logs, kernel argv, shared service environment, or model context. The updater service masks the mount; before agent runtime, the trusted guest core alone reads the envelope, deterministically prepares a shorter-lived append/pin capability, atomically persists a root-only recovery handoff, completes the authenticated consumption receipt, and only then unlinks the envelope. A crash before durable handoff leaves the envelope eligible only for byte-identical preparation of the same capability; a crash after handoff resumes consumption from the handoff. Once consumed, exchange replay always refuses without returning a bearer. Live renewal atomically replaces the durable recovery handoff before exposing the renewed in-memory pair. Current and pre-signed next-epoch capabilities plus pending key-revocation receipt survive process restart in `/run/choir-runtime-handoff`, which is inaccessible to updater, agents, capsules, signers, logs, immutable artifacts, and argv; after the revocation event commits, the old capability and transition state are erased."
      platform_transport: "The guest appender never receives SQL, route, signing, or host credentials. Through the existing authenticated proxy/corpusd service, it pins blobs and calls a typed event CAS endpoint using the short-lived capability. corpusd is the mechanical transaction writer and returns an EventHeadReceipt; the guest ComputerEventAppender is the sole authorized semantic caller. The capability is unavailable to models, capsules, updater, and ordinary tools; expiry/revocation or service unavailability fails closed."
      minimum_hashed_envelope: [schema_version, event_id, computer_id, sequence, previous_head, event_kind, occurred_at, idempotency_key, request_commitment, trajectory_id, parent_event_id, capsule_id, actor_profile, authority_ref, model_policy_refs, input_artifact_refs, output_artifact_refs, payload_commitment, privacy_class, proposed_effect_ref, decision_ref, verifier_refs, reducer_version, expected_desired_event_head, expected_effective_event_head, expected_pending_transition_ref, expected_desired_state_commitment, expected_effective_state_commitment, resulting_effective_state_commitment]
      derived_heads: "canonical_event_head is digest(E) for every appended event. desired/effective event heads, desired/effective state commitments, and nullable pending_transition_ref are deterministic reducer projections; no event contains its own digest. `resulting_effective_state_commitment` is SHA-256 over canonical reducer_version plus ordered effective CodeRef/ArtifactProgramRef/embedded-Dolt state refs and advances only on an atomically effective ResearcherUpdate, MaterializationApplied, or RollbackApplied."
      receipt_signing_contract:
        encoding: "Generate opaque UUIDv7 receipt_id and the ordered `required_signers` entries {signer_domain,key_id} first. `canonical_payload_sha256` is lowercase hex SHA-256 of RFC 8785 JSON for the complete receipt with only `canonical_payload_sha256` and `signature_set` omitted; receipt_id, receipt_kind, issuer, issued_at, required_signers, and all kind fields are therefore bound without self-reference. Each required signer Ed25519-signs the exact byte string UTF8(`choir-receipt-v1`) || 0x00 || raw_32_byte_canonical_payload_sha256. `signature_set` is an ordered one-for-one copy of required_signers with signature added; missing, duplicate, extra, relabeled, or invalid entries refuse. Required common fields are receipt_version, receipt_kind, receipt_id, issuer, issued_at, required_signers, canonical_payload_sha256, and signature_set."
        trust_roots: "corpusd owns the platform-control Ed25519 key in its deployment credential store and signs PinReceipt, EventHeadReceipt, ModeReceipt, LifecycleReceipt, CheckpointReceipt, and RouteProjectionCertificate. The already configured owner-promotion public key is pinned as the offline owner-recovery trust anchor for control-key rotation/revocation; its corresponding private key remains outside Choir services and ordinary deploy credentials and is not post-genesis route approval. An independent verifier key signs VerifierCertificate. choir-updater invokes the root guest credential service to sign HealthReceipt, MaterializationReceipt, UpdaterRecoveryReceipt, and updater-key changes without receiving private bytes. Platform, owner-recovery, verifier, and updater public-key histories are pinned in corpusd and constructed guest releases; updater key/ComputerID/RealizationID starts in GenesisImported. No private signing key enters agents, capsules, updater payloads, immutable source artifacts, or argv."
        rotation_and_time: "Planned platform rotation receipt has old-platform, new-platform, and owner-recovery signatures. The guest verifies it, appends key_rotated while old remains valid, receives the old-key EventHeadReceipt, then corpusd activates new. Emergency replacement has new-platform plus owner-recovery signatures and an explicit compromised-key cutoff; the pinned owner-recovery anchor authorizes the special recovery CAS, its EventHeadReceipt carries new-platform plus owner-recovery signatures, and new activates only after that receipt. Updater rotation follows the same old/new/owner-authorized-event order; emergency updater replacement requires owner-recovery signature. Revocation is owner-recovery-signed and, for an active signing key, names a simultaneously authorized replacement. Every receipt is pinned and inserted into corpusd `control_key_history` before its key event; activation follows the event. Old public keys remain indefinitely. Revocation names key_id and first invalid sequence/time; pre-cutoff receipts remain historical evidence, at/after-cutoff receipts refuse. Authorization expiry uses corpusd time and frozen maximum skew; event/pin/history receipts do not expire."
        verifier_matrix: "Guest appender verifies corpusd pin/head/mode/lifecycle/key receipts; corpusd verifies updater/verifier signatures and append capability; choir-updater verifies EventHeadReceipt plus accepted decision/materialization command; vmctl verifies corpusd EventHeadReceipt, both AuthorizationEvidence objects, RouteProjectionCertificate, CheckpointReceipt, MaterializationReceipt, VerifierCertificate, and exact route command; clean external clients verify all public receipts against pinned key history. Unknown key, bad signature, wrong issuer/kind/ComputerID, expired authorization, discontinuous rotation, or missing per-computer key event fails closed."
        kind_fields:
          PinReceipt: [computer_id, artifact_digest, media_type, length, privacy_class, pin_namespace, commitment_binding]
          PinReceipt_commitment_binding: "A payload PinReceipt carries `pin_intent_commitment`; the event-body PinReceipt carries final `request_commitment`. The pin-intent commitment is SHA-256 of RFC 8785 canonical JSON for the complete immutable `event_intent` object plus transition input before receipts exist. The final request commitment is SHA-256 of RFC 8785 canonical JSON `{event_intent, pin_intent_commitment, payload_pin_receipt_digests}` with ordered receipt digests. The append CAS and EventHeadReceipt bind both through the final request commitment and ordered receipt digests; no receipt hashes a value that depends on its own digest."
          EventHeadReceipt: [computer_id, previous_head, event_digest, sequence, event_kind, request_commitment, pin_receipt_digests, desired_event_head, effective_event_head, pending_transition_ref, desired_state_commitment, effective_state_commitment]
          ModeReceipt: [computer_id, old_mode, new_mode, old_generation, committed_generation, operation_id, base_event_head, expected_desired_event_head, expected_effective_event_head, expected_pending_transition_ref, expected_desired_state_commitment, expected_effective_state_commitment, bundle_digest, expires_at, idempotency_key, request_commitment]
          LifecycleReceipt: [computer_id, action, prior_lifecycle_state, resulting_lifecycle_state, generation, idempotency_key, request_commitment, completed_at]
          HealthReceipt: [computer_id, realization_id, release_digest, probe_contract_digest, started_at, completed_at, outcome, observation_artifact_digests]
          MaterializationReceipt: [computer_id, realization_id, accepted_or_rollback_event_head, prior_release_digest, resulting_release_digest, health_receipt_digest, outcome, request_commitment]
          UpdaterRecoveryReceipt: [computer_id, realization_id, accepted_event_head, attempted_release_digest, prior_release_digest, resulting_release_digest, outcome, health_receipt_digest, request_commitment]
          VerifierCertificate: [computer_id, bundle_digest, source_tree_digest, runtime_artifact_digest, base_effective_event_head, test_receipt_digests, policy_digest, decision, verified_at]
          CheckpointReceipt: [computer_id, canonical_event_head, desired_event_head, effective_event_head, effective_state_commitment, reducer_version, computer_version, release_digest, reconstruction_input_digests, materialization_receipt_digest, verifier_certificate_digest]
          PlatformKeyRotationReceipt: [old_key_id, new_key_id, new_public_key, activation_sequence, activation_time, compromise_cutoff]
          UpdaterKeyRotationReceipt: [computer_id, realization_id, old_key_id, new_key_id, new_public_key, activation_sequence, activation_time, authorizing_event_head_receipt_id]
          KeyRevocationReceipt: [key_domain, computer_id, key_id, first_invalid_sequence, first_invalid_time, replacement_key_id, reason_commitment]
          RouteProjectionCertificate: [computer_id, canonical_event_head, effective_event_head, event_head_receipt_id, accepted_event_authorization_evidence_ref, promotion_join_evidence_ref, checkpoint_receipt_digest, materialization_receipt_digest, verifier_certificate_digest, route_transition_command, route_transition_command_sha256, expires_at]
      external_attestations: "Signed receipts attest final artifact/event/state digests and are not fields inside the hashed object they attest. This removes self-reference."
      event_kinds: [genesis_imported, trajectory_started, model_resolved, message_recorded, tool_invoked, tool_returned, artifact_produced, effect_proposed, verification_recorded, effect_accepted, effect_rejected, materialization_started, materialization_applied, materialization_failed, rollback_requested, rollback_applied, researcher_update, checkpoint_published, route_projection_updated, lifecycle_observed, key_rotated, key_revoked, recovery_recorded]
    append_and_recovery_protocol:
      - "For an initialized computer, resolve corpusd canonical head H plus derived desired/effective heads and compare the embedded index. Mismatch refuses mutation and enters projection repair."
      - "Use the public two-phase guest client: redact/classify/encrypt each private payload into its final random-nonce envelope without pinning; compute every content-addressed ref and place it in immutable `event_intent`; canonicalize that intent without sequence, previous head, final request commitment, or PinReceipt digests; compute `pin_intent_commitment = SHA256(RFC8785(event_intent))`; immediately before pin, the guest appender TCB AEAD-decrypts/authenticates the exact frozen envelope against expected ComputerID/EventID using the root guest keyring, then pins those unchanged bytes; compute `request_commitment = SHA256(RFC8785({event_intent, pin_intent_commitment, payload_pin_receipt_digests}))` with ordered receipt digests; canonicalize event body E(previous=H), compute digest(E), pin E with a PinReceipt bound to the final request commitment, and continue to CAS. corpusd never receives the privacy key; it re-opens private envelopes structurally during append validation and requires metadata ComputerID/EventID to equal E. Orphan pins are non-authoritative and garbage-collectable after the retention floor."
      - "Write E and proposed semantic changes as prepared rows in one embedded-Dolt transaction keyed by idempotency key and expected heads."
      - "Call the authenticated corpusd typed endpoint to CAS `computer_event_heads` H→digest(E). corpusd validates capability scope, sequence, previous head, kind-specific desired/effective-head preconditions, request commitment, and PinReceipts, then returns signed EventHeadReceipt. Only the scoped ComputerEventAppender may request this event CAS; vmctl remains sole writer only for the separate route-slot CAS."
      - "Finalize the embedded index/materialization transaction. A crash before head CAS discards the prepared event; a crash after CAS reconstructs/finalizes from E. Acknowledgement occurs only after a verified EventHeadReceipt and embedded finalization."
      - "Causal-only events—trajectory/model/message/tool/artifact/proposal/verification observations—may regenerate E against a newer `previous_head` while preserving immutable payload digests. Decision/effective-state requests bind immutable proposal/decision refs and expected desired/effective heads. After a causal-only CAS conflict, the appender may regenerate E against the new canonical head only when those state heads still match; any state-head change refuses. No pinned payload digest or request commitment is rewritten."
      - "Idempotency is request-commitment based: the same key plus byte-identical canonical request returns the original durable operation/receipt in every state, including terminal; the same key with a different commitment returns conflict before effects."
      - "No state effect is materialized before its acceptance/head CAS. Route/checkpoint CAS cannot acknowledge an event. For accepted durable state changes, checkpoint publication and route projection have separate receipts and the operation is not terminal success until required projection is complete."
    state_transition_matrix:
      invariant: "At most one pending transition exists. No EffectAccepted or RollbackRequested may append while pending_transition_ref is non-null. No second materialization may start. Safety rollback during a pending transition first requires a verified MaterializationFailed/UpdaterRecoveryReceipt that restores the prior release and clears pending; overlapping desired/effective mutations are forbidden."
      causal_and_refusal_events: "Trajectory/model/message/tool/artifact/proposal/verification, EffectRejected, lifecycle/key/checkpoint/route observations require exact referenced objects and expected desired/effective heads/commitments but move none of desired_event_head, effective_event_head, pending_transition_ref, or state commitments."
      EffectAccepted: "Requires expected desired/effective heads and commitments to equal current, pending=null, exact verified proposal/bundle, and bound accept_once. Sets desired_event_head=digest(E), desired_state_commitment=bundle target, pending_transition_ref=digest(E); effective head/commitment stay unchanged."
      MaterializationStarted: "Requires pending_transition_ref=accepted event, exact desired/effective projections, and updater command bound to that event. Moves no head/commitment."
      MaterializationApplied: "Requires pending accepted event, unchanged desired/effective projections, matching MaterializationReceipt and resulting release. Sets effective_event_head=digest(E), effective_state_commitment=desired_state_commitment, pending=null; desired projection retains the accepted event/target."
      MaterializationFailed: "Requires pending accepted/rollback event and verified MaterializationReceipt or UpdaterRecoveryReceipt proving prior effective release restored. Sets desired_event_head=effective_event_head, desired_state_commitment=effective_state_commitment, pending=null; effective projection stays unchanged."
      RollbackRequested: "Requires pending=null, exact current desired/effective heads/commitments, target prior applied event/checkpoint/materialization/route generation, and owner rollback authority. Sets desired_event_head=digest(E), desired_state_commitment=target prior commitment, pending_transition_ref=digest(E); effective projection stays unchanged."
      RollbackApplied: "Requires pending rollback event, unchanged state projections, and matching MaterializationReceipt. Sets effective_event_head=digest(E), effective_state_commitment=desired_state_commitment, pending=null; desired projection retains rollback request/target."
      ResearcherUpdate: "Requires pending=null and exact desired/effective heads/commitments. The typed Dolt mutation and event CAS finalize fate-sharing; both desired_event_head and effective_event_head become digest(E), both commitments become the deterministic new commitment, pending remains null."
      accept_once_binding: "ModeReceipt binds operation, bundle, expected desired/effective heads and commitments, pending=null, expiry, and generation. Any mismatch refuses before acceptance."
    genesis_migration_and_versions:
      - "GenesisImported is the only absent-row transition: corpusd insert-if-absent expects no `computer_event_heads` row, uses sequence=1 and previous_head=`0000000000000000000000000000000000000000000000000000000000000000`, and binds the frozen baseline ComputerVersion, embedded-Dolt digest, source tree, effective marker, reducer version, ComputerID, updater public key, and disposable-computer authorization. On success canonical_event_head=desired_event_head=effective_event_head=digest(GenesisImported), desired_state_commitment=effective_state_commitment=frozen baseline commitment, and pending_transition_ref=null. Same idempotency key/request returns the original genesis receipt; any competing genesis request conflicts. The head row is never deleted, reset, or re-genesised."
      - "This mission performs that genesis exactly once for one explicitly disposable staging computer. Pre-genesis trajectories are labeled legacy_unproven and are never represented as complete. Other owner computers and historical workspaces remain untouched."
      - "Envelope and reducer versions are immutable integers. V1 readers preserve unknown fields; a reducer upgrade appends a migration event and retains the previous reducer for deterministic replay. No event or accepted artifact is rewritten or deleted."
      - "Schema changes are additive during this mission. Destructive migration, Dolt reset, event compaction, history synthesis, and cross-computer backfill are forbidden. G0 freezes table names/indexes and a migration rehearsal that can be discarded before B."
      - "The corpusd recovery head is discoverable from ComputerID after realization loss. Reconstruction fetches the immutable chain, verifies hashes/receipts, rebuilds embedded Dolt, derives desired/effective event heads and the effective-state commitment by deterministic fold, and never reruns a model, tool, network request, or nondeterministic observation."
    trajectory_privacy_and_retention:
      - "Complete trajectory means a complete causal envelope and content commitment for every model resolution, message, tool call/result, artifact, decision, refusal, and external receipt after GenesisImported—not permanent plaintext retention."
      - "Secrets, bearer values, environment credentials, and provider tokens are replaced before hashing with typed non-reversible secret handles. Detection failure refuses/quarantines the append and revokes the implicated capability; secrets never enter model-visible events, argv, diffs, logs, or immutable artifacts."
      - "Private message/tool payloads use XChaCha20-Poly1305 owner-scoped immutable artifacts: a fresh random nonce per artifact; canonical metadata plus computer_id, event_id, media type, length, privacy class, and key-version digest as AAD; ciphertext digest in the event. A root-only per-computer keyring is generated with crypto/rand and owned by the guest core credential service, outside agents/capsules/updater. Rotation is guest-core-generated and event-receipted; old keys remain until their payload retention disposition. The keyring is realization-local private-observation availability, not canonical effective state: reconstruction may intentionally lose old plaintext but never accepted typed effects. No private key is delivered by host, constructor, model, or artifact."
      - "Large outputs use content-addressed artifact indirection; the event keeps media type, length, digest, privacy class, key version when encrypted, and selectors. Truncation never substitutes for a digest commitment. External/nondeterministic results are recorded, not replayed."
      - "For this mission, canonical envelopes, accepted-effect artifacts, verifier/authority receipts, rollback refs, and encrypted trajectory payloads are retained without compaction through terminal acceptance. Later privacy deletion requires a separate owner-ratified policy and leaves an immutable tombstone/commitment; key loss may remove plaintext availability but not effective-state reconstruction."
    capsule_trust_boundary:
      - "Capsule capability authority is guest-local, inside the trusted core runtime and outside all agent processes/capsules. The host-side HostAuthority/HostClient/vsock design is retired from self-development and receives no deployment or fallback path. No Node B host daemon/configuration change is required."
      - "The authority creates opaque 256-bit handles bound in process to computer_id, agent_run_id, role, capsule_id, verb set, path/network policy, resource limits, expiry, and revocation epoch. Handles are injected into tool execution context, never model text. Restart revokes every handle; durable operations recover by policy and mint new handles."
      - "The core resolves capability before connecting over a permissioned per-capsule AF_UNIX broker socket. Broker peer credentials, socket ownership, capsule identity, and verb/path policy are all checked; the workload never receives authority keys."
      - "Isolation is fail-closed: user/PID/mount/network/UTS/IPC namespaces, cgroup v2 limits, read-only immutable lowerdir, private overlay upperdir, no host bind mounts, capability drop, no_new_privs, broker default-deny seccomp, capability-specific workload seccomp, and enforcing Landlock are mandatory. Missing kernel support or cleanup failure refuses admission; best-effort downgrade is forbidden."
      - "Build capsules have no external network, secrets, host/vsock devices, or mutable dependency cache. Lowerdir pins the source tree, toolchain, module/dependency cache, build recipe, and base event head."
    role_cutover:
      - "Super retains orchestration, capsule lifecycle, durable agent delegation, inspection, verification request, and decision-proposal tools only—no bash, writable file, coding, shipper, worker-VM, candidate, route, or host tools."
      - "CoSuper retains its agent loop and research/read tools but every shell/filesystem/build action is a capsule_* broker verb under one capability; direct core-VM coding/write tools are removed."
      - "VSuper/candidate-super and all aliases, worker-VM/candidate-VM lifecycle/controllers/tools/APIs/prompts, and their retention/configuration code are deleted from production. Requests using retired names fail closed. No sibling profile may retain AllowWritableFiles, AllowCodingTools, shipper, or VM-delegation authority. Generic delegated agents use durable runs/trajectories and capsules; they are not worker VMs."
      - "Researcher remains VM-local with read/research/evidence tools and the typed update_coagent source-packet write. It has no bash, raw Dolt, writable file, capsule commit, acceptance, route, or host authority. Its typed mutation is appended through ComputerEventAppender in the same logical operation."
    capsule_effect_bundle:
      required: [bundle_version, computer_id, base_event_head, trajectory_ref, capsule_identity, capability_policy_digest, source_tree_ref, ordered_file_effects, generated_artifact_refs, build_recipe_ref, runtime_artifact_ref, test_receipts, verifier_receipts, dependency_toolchain_refs, resource_receipts, content_digest]
      rules: "The bundle is immutable, content-addressed, stale-base checked, secret scanned, independently verified from a read-only capsule, and cannot contain symlink escapes, devices, sockets, ownership escalation, host paths, unknown artifact kinds, network dependencies, or unbounded resource requests."
    guest_updater_and_materialization:
      trust_boundary: "A minimal root-owned choir-updater service runs inside the guest, outside the agent runtime and capsules. It accepts only ComputerEventAppender-authorized operations over a permissioned Unix socket and remains available when the main Choir service fails. It has no model/provider or host authority."
      release_form: "Accepted CodeClosure contains immutable CodeRef source tree, offline build recipe/toolchain/dependencies, signed verifier receipts, runtime/service artifacts, and compatibility manifest. Updater stages it under a digest-named read-only guest release directory; no in-place executable overwrite occurs."
      state_machine: [prepared, accepted, materializing, applied, failed, rollback_pending, rolled_back]
      protocol: "Acceptance records desired code but does not advance effective code. Updater stages and verifies the release, atomically swaps a root-owned current-release pointer, restarts only the guest Choir service through the guest service manager, and waits for build/event-schema/health/marker checks. MaterializationApplied then advances effective head. CLI terminal success requires Applied plus checkpoint/route receipts."
      failure: "On stage/restart/health failure, updater atomically restores the prior release, restarts it, records MaterializationFailed through the recovery append path, leaves effective head unchanged, and returns a durable failed operation. Repeated commands are idempotent. If the main runtime cannot append, updater writes a signed recovery receipt that ComputerEventAppender imports before any later acceptance."
      rollback: "RollbackRequested selects a prior applied effective head, never arbitrary bytes. Updater stages/verifies that pinned release, swaps/restarts/health-checks it, and appends RollbackApplied. History and rejected/failed events remain."
      compatibility: "New event/reducer versions are not emitted until updater and runtime can read the previous applied guest release and the previous updater can safely refuse the new version. Platform rollback has a one-way security ratchet: the pre-cutover R0 deploy may be used only before GenesisImported; C's accepted certificate-enforcing SHA and immutable deployment inputs become R1 before genesis. After genesis, guest change failure first event-rolls back to a compatible applied head, and platform rollback may target only R1 or a later release that preserves certificate-only route refusal, event/key schemas, and updater recovery. R0 is permanently inadmissible for the initialized ComputerID."
    checkpoint_and_route_projection:
      - "After MaterializationApplied or RollbackApplied, constructor emits ComputerVersion plus corpusd-signed CheckpointReceipt containing CodeRef, ArtifactProgramRef, ComputerID, canonical/desired/effective event heads, effective-state commitment, embedded-Dolt reconstruction inputs, reducer version, MaterializationReceipt, and VerifierCertificate. `checkpoint_published` records its digest through ComputerEventAppender."
      - "Route authorization construction is non-circular and ordered. First corpusd RFC8785-encodes the inner `AcceptedEventAuthorizationEvidence` payload {version, computer_id, accepted_or_rollback_event_digest, event_head_receipt_id, effective_event_head, old_computer_version, new_computer_version, decision_actor, decision_scope}, then uses existing `routeledger.NewAuthorizationEvidence(approval, route_slot_id, new_computer_version, payload, created_at)`; the resulting `approval:sha256:` ref is the existing routeledger hash over {evidence_kind, route_slot_id, computer_version, payload_sha256, created_at}, not merely the inner payload hash. Second it constructs `PromotionJoinEvidence` the same way with inner payload {version, computer_id, event_head_receipt_id, checkpoint_receipt_digest, materialization_receipt_digest, verifier_certificate_digest, old_computer_version, new_computer_version} and kind promotion_certificate. Third it builds the exact routeledger.TransitionCommand {route_slot_id, transition_kind, old_computer_version, new_computer_version, expected_generation, approval_ref, promotion_certificate_ref, rollback_target_receipt_id when rollback, idempotency_key} and signs the outer RouteProjectionCertificate over that normalized command and both evidence refs. Neither evidence payload nor command contains the outer certificate digest."
      - "For the post-genesis mission ComputerID, the trusted core RouteProjectionClient is the sole presenter through the typed endpoint. vmctl resolves both full AuthorizationEvidence objects, verifies their existing wrapper hashes/kinds/route/new-version bindings, deeply verifies their inner payload joins and the outer certificate/receipts, reconstructs and RFC8785-byte-compares the normalized TransitionCommand plus its SHA-256, then calls routeledger.Transition; vmctl alone CASes the route slot. Every legacy bootstrap/promote/rollback path lacking this certificate refuses this slot. OwnerPromotionApproval/G3PromotionAcceptance remain non-authorizing historical predecessor receipts only."
      - "Reconstruction from the previous checkpoint resolves ComputerID through corpusd, fetches the externally pinned event chain to the latest recovery head, verifies hashes/receipts, then verifies effective state before serving."
    public_cli_api_and_auth:
      target: "Every command requires --computer <ComputerID>. During this mission the API key is owner-issued and bound to exactly the disposable staging ComputerID; ambient current-computer selection is forbidden for acceptance. Cross-computer delegated grants remain successor scope."
      secret_input: "Self-development commands remove the plaintext `--api-key`, `--prompt`, and `--reason` forms. Keys come only from `CHOIR_API_KEY` or `--api-key-file <0600-path|->`; prompts/reasons come from `--prompt-file <path|->` and `--reason-file <path|->`, with `-` meaning stdin. The CLI never echoes these values, scrubs diagnostic bodies, refuses group/world-readable key files, and sends them only in authenticated request bodies/headers. Existing plaintext `--api-key` is retired for all choir commands before product acceptance."
      cli:
        - "choir self-dev start --computer <id> --idempotency-key <key> --prompt-file <path|->"
        - "choir self-dev status --computer <id> <operation-id>"
        - "choir self-dev inspect --computer <id> <operation-id>"
        - "choir self-dev approve --computer <id> <operation-id> --expected-desired-head <hash> --expected-effective-head <hash> --bundle <digest> --verifier <digest> --idempotency-key <key>"
        - "choir self-dev reject --computer <id> <operation-id> --expected-desired-head <hash> --expected-effective-head <hash> --bundle <digest> --verifier <digest> --reason-file <path|-> --idempotency-key <key>"
        - "choir self-dev rollback --computer <id> --expected-desired-head <hash> --current-applied-head <hash> --to-applied-head <hash> --prior-materialization <digest> --prior-checkpoint <digest> --expected-route-generation <n> --idempotency-key <key>"
        - "choir self-dev wait --computer <id> <operation-id>"
        - "choir self-dev genesis --computer <id> --baseline-version <digest> --baseline-state <digest> --expected-absent --idempotency-key <key>"
        - "choir self-dev mode get --computer <id>"
        - "choir self-dev mode set --computer <id> --mode <off|audit_only|propose_only> --expected-generation <n> --idempotency-key <key>; for `--mode accept_once` only, additionally require --expires-at <time> --operation <id> --expected-desired-head <hash> --expected-effective-head <hash> --bundle <digest>"
        - "choir computer status --computer <id>"
        - "choir computer start|stop|restart --computer <id> --idempotency-key <key>"
      api:
        - "POST /api/computers/{computer_id}/self-development/operations"
        - "POST /api/computers/{computer_id}/self-development/genesis"
        - "GET /api/computers/{computer_id}/self-development/operations/{operation_id}"
        - "GET /api/computers/{computer_id}/self-development/operations/{operation_id}/receipts"
        - "POST /api/computers/{computer_id}/self-development/operations/{operation_id}/decision"
        - "POST /api/computers/{computer_id}/self-development/rollbacks"
        - "GET /api/computers/{computer_id}/events/{event_head}"
        - "GET /api/computers/{computer_id}/self-development/mode"
        - "PUT /api/computers/{computer_id}/self-development/mode"
        - "POST /api/computers/{computer_id}/lifecycle/{start|stop|restart}"
      scopes: [computer:self_development:read, computer:self_development:genesis, computer:self_development:propose, computer:self_development:approve, computer:self_development:rollback, computer:self_development:mode, computer:lifecycle]
      decision_requests: "Approve and reject bind operation_id, immutable proposal event, bundle digest, verifier digest, expected_desired_event_head, expected_effective_event_head, expected commitments/pending ref from the frozen operation, actor/scope, reason or decision, and request commitment. Rollback binds expected desired head, current applied effective head, target prior applied head, prior MaterializationReceipt, CheckpointReceipt, expected route generation, actor/scope, and request commitment. Supplied bindings must equal immutable records; mismatch refuses. No expected causal head is required: appender uses latest canonical head only while every state projection remains unchanged."
      lifecycle_authority: "Lifecycle is a platform actuator/projection, not guest semantic promotion. Existing authenticated lifecycle control stores an idempotent signed LifecycleReceipt outside the stopped guest; on next start the appender verifies and joins it as `lifecycle_observed`. Lifecycle cannot claim a guest event while stopped and lifecycle receipts are never state-acceptance authority."
      operation_states: [requested, executing, frozen, verified, awaiting_approval, accepted, materializing, applied, rejected, rollback_pending, rolled_back, failed, degraded]
      receipts: [operation_id, request_commitment, computer_id, trajectory_id, capsule_id, base_head, bundle_digest, verifier_refs, decision_actor, decision_event, desired_head, effective_head, materialization_receipt, checkpoint_ref, route_certificate, route_generation, mode_receipt, lifecycle_receipt, terminal_state, error]
      durability: "Self-development operations, decisions, and receipts are event-derived; mode and lifecycle are signed platform-control receipts joined into the event chain when the guest is live. All survive runtime/computer restart. Public endpoints return the original receipt for same-key/same-request retries and refuse same-key/different-request, wrong target/scope/head/bundle, missing verifier, non-disposable target, and internal/test-only bypass. Existing choir run list/cancel migrate from /api/agent/* to public /api/runs resources before they can be cited."
    activation_and_landing_safety:
      modes: [off, audit_only, propose_only, accept_once]
      authority: "Mode is a generation-CAS platform control row on the existing corpusd sql-server, changed only through the public owner-scoped mode endpoint. It is not semantic state or a third authority. Guest admission requires a current signed ModeReceipt; unreachable, missing, expired, stale-generation, or mismatched target defaults to off. Mode bindings operation/bundle/expiry and expected desired/effective heads/commitments are forbidden for off, audit_only, and propose_only and mandatory for accept_once. Before genesis, ModeReceipt uses the zero head and may remain off only."
      operation_matrix: "Under off: mode get/set, status/inspect/event/receipt reads, lifecycle, deterministic refusals, owner-scoped safety rollback, and exactly one `computer:self_development:genesis` call are available. Genesis requires absent row, disposable target, frozen baseline/G0/G1 receipts, zero head, and mode off; it leaves mode off. Audit, proposal, and approval refuse under off. audit_only additionally permits non-effectful event/reconstruction probes. propose_only additionally permits one capsule/frozen proposal and rejection, but not approval. accept_once permits only the exact bound approval and returns to propose_only after its decision. Owner rollback is available under every mode but, if pending is non-null, must first complete the recovery/MaterializationFailed transition; it never overlaps another transition."
      genesis_identity_binding: "G1 freezes and reviews `candidate_ref`, the exact source candidate commit. C later lands that candidate and produces a distinct `deployed_release_ref`, the immutable `buildinfo.Commit` served by the disposable target. Genesis requires both identities plus exact G0/G1 receipt refs, binds all four into the GenesisImported event's immutable input-artifact commitment, and never requires candidate_ref to equal deployed_release_ref."
      old_path: "Worker-VM and candidate-VM concepts and code are deleted in the release candidate, together with direct-role self-development paths; none are retained as unrelated features or fallbacks. With mode off, self-development refuses rather than falling back."
    gate_rejection_and_platform_rollback:
      G0: "Set blocked_incomplete; mutate no behavior; repair and re-freeze owner contract conformance."
      G1: "Discard or repair the unlanded code candidate; no deployment or role cutover."
      G2: "Immediately set mode off and revoke capsule capabilities. Before genesis, any platform failure may use R0. Once GenesisImported commits, R0 is permanently forbidden; isolation, secret, corruption, or operability failure stops the disposable computer, preserves receipts, guest-rolls back if possible, and deploys only the frozen R1 certificate-enforcing security floor or a later compatible release. Harmless evidence gaps may be repaired in a new commit only while effects remain off."
      G3: "Set mode off, revoke capabilities, and preserve receipts. If guest effective state changed, use choir-updater to event-roll back to the last compatible applied head. If platform behavior or operability failed after genesis, stop serving the disposable computer until R1 or a later certificate-enforcing compatible release is healthy; never deploy R0. Never leave a rejected build serving while repairing."
      data: "No schema/event rollback deletes history. Forward repair is additive. R0 pre-cutover source/deploy identity, R1 post-cutover security-floor SHA and immutable deployment inputs, baseline ComputerVersion, route generation, release pointer, and effective event head are frozen before genesis."

finish:
  deliver: "One explicitly disposable staging Choir computer develops Choir itself end to end through the supported public choir CLI: complete causal audit, real capsule-scoped cosuper effects, independent verification, external scoped approval, typed guest materialization, restart/reconstruction, rejection, rollback, and exact deployed receipts—with no direct super/cosuper/VSuper mutation, worker/candidate VM, host daemon/repair, mutable branch, package/lineage activation, SSH, raw vmctl, internal API, or model rerun."
  artifact: "The settled event, privacy, capsule, role, updater, checkpoint, public CLI/API/auth, activation, and rollback contracts above implemented as one clean cutover; worker-VM/candidate-VM code and all old self-development paths deleted, retired names refused, and Choir-in-Choir blocked."
  acceptance:
    - action: "From a clean external client, use a key bound to the disposable ComputerID and `choir self-dev start --computer ...`; observe durable operation/trajectory identity through status/inspect before and after runtime restart."
      proves: "Explicit product-path intake and durable targeting exist without ambient current-computer or internal routes."
      evidence_class: deployed_cli_targeted_intake
    - action: "Attempt direct bash/write/build/worker/candidate/shipper/route tools as Super, CoSuper, VSuper/candidate-super, Researcher, Conductor, and unscoped profiles. Require refusal; prove CoSuper shell/file work succeeds only through one capability-bound capsule and Researcher only through typed update_coagent."
      proves: "Every effectful role and alias has a complete disposition; no residual core mutation path remains."
      evidence_class: deployed_role_effect_isolation
    - action: "Exercise kernel prerequisite absence, namespace/network escape, host/vsock access, symlink/device/socket escape, resource exhaustion, capability theft/expiry/restart/revocation, broker impersonation, Landlock/seccomp failure, and cleanup failure. Require fail-closed refusal and no core mutation."
      proves: "Capsules are real isolated effect chambers with a guest-local TCB, not process labels or best-effort sandboxes."
      evidence_class: deployed_capsule_security
    - action: "Record a complete post-genesis trajectory with model/message/tool/artifact/refusal events, secret canaries, large outputs, concurrent trajectories, and a typed researcher update. Independently verify chain hashes, privacy commitments, canonical order, idempotency, and reconstruction without rerunning models/tools."
      proves: "Audit completeness, privacy, concurrency, and replay semantics hold together."
      evidence_class: independent_event_privacy_reconstruction
    - action: "In propose_only mode, build a harmless marker change from pinned offline source/toolchain/dependencies, freeze the exact CapsuleEffectBundle, independently verify it, and prove core/effective state and served marker remain unchanged."
      proves: "Speculative work is isolated and immutable before acceptance."
      evidence_class: deployed_frozen_effect_bundle
    - action: "Use scoped `choir self-dev approve` with expected head/bundle. Observe accepted→materializing→applied, atomic release swap, guest-service health, checkpoint and route receipts, then restart the runtime and entire computer and observe the new marker/head without SSH."
      proves: "An accepted event safely changes effective guest code and survives restart/reconstruction through supported paths."
      evidence_class: deployed_self_development_apply
    - action: "Inject crashes before/after artifact pin, embedded prepare, head CAS, release swap, health check, checkpoint, and route CAS. Require the declared state-machine recovery, no duplicate effect, no acknowledged lost head, and old effective service on failed materialization."
      proves: "Pin-before-ack, updater, checkpoint, and projection failures cannot strand or corrupt the computer."
      evidence_class: deployed_failure_atomicity
    - action: "Reject a second verified bundle and preserve its full audit without applying it. Then use scoped `choir self-dev rollback` to the prior applied head; verify atomic rematerialization, restart, old marker, retained history, and rollback/checkpoint/route receipts."
      proves: "Rejection preserves learning without contamination and rollback is event-derived, nondestructive, and operable."
      evidence_class: deployed_rejection_and_rollback
    - action: "Retry identical canonical requests with the same idempotency key and require the original receipt; attempt same key with changed request, wrong target/key scope/head/bundle/verifier, expired accept_once, internal `/api/agent/*`, raw vmctl, SSH, mutable branch, AppAdoption/lineage, worker/candidate VM, host daemon, and non-disposable target. Require deterministic replay or refusal before effects."
      proves: "Authority and negative-path boundaries are product-enforced."
      evidence_class: deployed_authority_refusals
    - action: "For the pushed origin/main SHA, verify CI, exact Node B/choir.news identity, deployed modes, independently retrieve/hash/reconstruct E's immutable receipts and replay only non-mutating/refusal checks, confirm prior deploy/effective-head rollback refs and registry closure, and freeze the G3 consensus packet. Do not perform a second accepted mutation."
      proves: "The full loop works on deployed staging and can be safely unwound."
      evidence_class: complete_deployed_capsule_self_development
  rollback: "Use the contract's gate-specific dispositions. Before genesis, platform rollback may use frozen R0. Before acceptance, revoke/destroy the capsule and retain its pinned audit. After guest acceptance, event-roll back through choir-updater to the prior applied head. After GenesisImported, platform rollback may deploy only frozen R1 or a later compatible release preserving certificate-only route refusal, event/key schemas, and updater recovery; R0 is forbidden even if that means stopping the disposable computer and forward-repairing from immutable R1 inputs. Never delete events, reset Dolt, restore opaque data.img, switch to a candidate VM, mutate Node B manually, use SSH, or leave a rejected build serving."
  landing:
    required: true
    environment: staging_node_b_and_choir_news_one_disposable_computer
    required_receipts: [pushed_origin_main_commit, ci, pre_cutover_R0_ref, post_cutover_R1_security_floor, deployed_build_identity, disposable_computer_id, baseline_event_head, baseline_computer_version, baseline_route_generation, activation_modes, cli_targeted_intake, role_refusals, capsule_security, event_privacy_reconstruction, frozen_effect_bundle, approval, materialization, checkpoint, route_projection, runtime_restart, computer_restart, failure_atomicity, rejection, rollback, authority_refusals, no_ssh]
    registry_hygiene:
      must_update: [docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      rule: "Terminal closure makes this Definition historical. It does not automatically promote Choir-in-Choir or performance drafts."
  not_done_when:
    - "Only A0, a code scaffold, local/macOS tests, mocked capsules, generic CLI JSON, CI, or dashboard prose passes."
    - "Any gate is skipped, softened by panel majority, or passed without its deterministic evidence."
    - "Any direct role, VSuper alias, worker/candidate VM, host daemon, mutable branch, package/lineage, raw vmctl, SSH, internal API, best-effort isolation, or destructive rollback path remains usable for self-development."
    - "An accepted event can be acknowledged but not discovered, reconstructed, materialized, health-checked, checkpointed when required, or rolled back after realization loss."
    - "Complete trajectory capture stores secrets/plaintext indiscriminately or reconstruction reruns a model/tool/network effect."

boundaries:
  mutation_class: red
  protected_surfaces: [computer_event_authority, agent_trajectories, privacy_and_secrets, embedded_Dolt, platform_control_head_CAS, immutable_artifacts, ComputerVersion_checkpoints, capsule_isolation, guest_capability_authority, tool_registry, agent_roles, guest_updater, public_API, choir_CLI, API_key_scopes, vmctl_route_projection, run_acceptance, deployment_routing]
  heresy_delta:
    discovered: ["audited construction omitted complete trajectory causality", "direct Super/CoSuper/VSuper mutation and obsolete worker-VM/candidate-VM paths remain live", "capsule host dependency and isolation/broker/cleanup are unwired scaffolds", "no canonical event ledger, privacy protocol, guest updater, or public self-development API exists", "promotion-as-route and candidate-VM prose conflicts with the owner cutover", "the previous C/D order required deployed proof before deployment", "the initial G0 packet incorrectly preserved an unrelated-worker-VM exception"]
    introduced: []
    repaired: []
  must_preserve:
    - "One logical event authority and one materialized effective head per ComputerID; projections never overrule it."
    - "No host repair/configuration or capsule host daemon. Ordinary tracked service deployment and existing vmctl actuation remain landing/control-plane effects, not guest agent authority."
    - "Super no bash; CoSuper effects only in capsules; VSuper retired; Researcher typed update only."
    - "No acknowledged state depends solely on a realization, mutable disk, process memory, model rerun, secret plaintext, or unpinned artifact."
    - "All isolation and authorization fail closed."
  excluded:
    - "Node B kernel/NixOS/Firecracker repair, new host service, provider-secret administration, or one guest repairing its host."
    - "Choir-in-Choir grants/control, cross-computer execution, fleet migration, package marketplace, Features UI, or performance optimization."
    - "Migration of any non-disposable owner computer or synthesis of pre-genesis trajectory history."
  completion_evidence_floor: [deployed_cli_targeted_intake, deployed_role_effect_isolation, deployed_capsule_security, independent_event_privacy_reconstruction, deployed_frozen_effect_bundle, deployed_self_development_apply, deployed_failure_atomicity, deployed_rejection_and_rollback, deployed_authority_refusals, complete_deployed_capsule_self_development]

measures:
  - name: acknowledged_mutation_without_complete_event
    kind: gate
    baseline: unknown
    desired: 0
    decision_use: "Block G1/G2/G3 on any missing causal, authority, pin, privacy, or effective-state edge."
    cannot_prove: "Usefulness or deployed behavior without product acceptance."
  - name: effectful_tools_outside_capsules
    kind: gate
    baseline: "Super/CoSuper/VSuper direct tools live"
    desired: 0
    decision_use: "Block deployment or activation on any residual self-development caller."
    cannot_prove: "Kernel isolation or event completeness."
  - name: trajectory_storage_and_replay_cost
    kind: telemetry
    baseline: unknown
    desired: "Measure only; no retention/compaction change in this mission."
    decision_use: "Inform a later owner-ratified retention mission."
    cannot_prove: "Permission to delete or compact canonical evidence."

execution:
  - id: A-contract-conformance
    purpose: "Land the code-free doctrine/ontology/API/security/migration contract already settled above and map every existing writer/caller to it."
    work:
      - "Before implementation investment, obtain a reproducible immutable-image kernel receipt from the pinned Nix guest configuration. It must prove user/PID/mount/network/UTS/IPC namespaces, cgroup v2, loadable overlayfs, seccomp/filter, and boot-enabled enforcing Landlock; any missing capability sets blocked_incomplete before G0 and no kernel/NixOS/Firecracker repair or downgrade is authorized. Because the target realization does not exist before C, G0 also freezes the public no-SSH signed KernelCapabilityReceipt contract that B must implement. C must retrieve that receipt for the exact disposable ComputerID and bind it to the immutable guest image/config digest before D or GenesisImported; failure then invokes C/G2 rejection rather than SSH/raw vmctl."
      - "Replace current route-flip/candidate-VM/promotion claims in docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/current-architecture.md, and registry witnesses with the exact identity/promotion/event/checkpoint clauses above."
      - "Freeze a complete writer/caller/deletion inventory covering Trace/EventRecord/Trajectory/run memory, embedded Dolt, platform head and route CAS, ArtifactProgram/ComputerVersion, AppAdoption/lineage, all role profiles/tool installers, VSuper aliases, worker/shipper paths, capsule HostClient/HostAuthority, Executor/broker/tape, API/CLI routes, and materializers."
      - "Freeze concrete V1 tables/indexes/canonical encoding, append/recovery state machine, genesis fixture, reducer compatibility, privacy classification/redaction/encryption, guest authority, updater, public CLI/API/auth, activation modes, and all rollback refs exactly conforming to this contract. Ordinary naming/library choices may vary; semantic behavior may not."
      - "Document every obsolete path's delete/retire disposition; worker-VM/candidate-VM code has no retention classification. Prove no third semantic store or host dependency is proposed."
    exit: "G0 receives a code-free conformance packet with no unresolved owner/authority/security/migration/product decision."
  - id: B-implement-cutover-effects-off
    purpose: "Implement the entire event, capsule, role, updater, checkpoint, CLI/API, auth, and activation substrate while self-development activation remains off by default. Normal non-self-development product behavior is not disabled."
    entry: "G0 accepted exact conformance; any semantic deviation returns to blocked_incomplete and owner review."
    work:
      - "Implement event chain/head CAS, constructor credential-envelope seam, signed receipt/trust-root rotation, exact route-command certificate and post-genesis legacy refusal, embedded projection, genesis, replay, privacy/artifact handling, reducer/version compatibility, failure injection, and the R0→R1 one-way platform rollback ratchet."
      - "Replace host capability dependency with guest-local authority; implement mandatory kernel isolation, broker routes, resource/network/path policy, cleanup/restart/revocation, frozen bundles, and a signed public no-SSH KernelCapabilityReceipt bound to ComputerID, realization, immutable guest image/config digest, observed boot parameters, capability values, and observation time."
      - "Cut over all roles/aliases; delete VSuper, worker-VM, candidate-VM, and self-development package/lineage paths. Preserve generic durable agent delegation only through runs/trajectories and capsules; preserve no VM-worker exception."
      - "Implement choir-updater release slots/recovery, checkpoint/projection adapter, public self-dev/lifecycle API, CLI grammar, scoped keys, durable operations, and off/audit_only/propose_only/accept_once modes."
      - "Build from pinned offline inputs and add focused deterministic tests for every declared state transition/refusal."
    exit: "A frozen source candidate builds and deterministic tests pass; all self-development effects remain off. G1 reviews the complete candidate, migrations, negative caller inventory, and prior rollback refs before landing."
  - id: C-land-and-deploy-off
    purpose: "Land the complete cutover with self-development activation mode off before any deployed effect proof."
    entry: "G1 accepted the frozen code candidate."
    work:
      - "Freeze R0 source/deploy identity, baseline ComputerVersion/event head/route generation/release pointer. Commit and push origin/main; monitor CI and Node B deployment; verify choir.news build identity equals the pushed SHA."
      - "Confirm mode off refuses audit/proposal/approval while preserving mode control, reads, lifecycle, exact one-time genesis, and safety rollback; confirm ordinary non-self-development health. Retrieve and independently verify the signed public KernelCapabilityReceipt for the exact disposable ComputerID against the G0 immutable-image receipt; any mismatch, missing mandatory capability, or unsupported no-SSH observation blocks D. Before genesis, roll back to R0 on security, schema, startup, operability, or capability-attestation failure."
      - "After C acceptance, pin the exact certificate-enforcing deployed SHA, immutable deployment inputs, event/key schema readers, updater recovery artifact, route-refusal proof, and verified deployed kernel receipt as R1. Rehearse redeploy from those immutable inputs without creating genesis. D may not begin until R1 independently passes health, kernel-capability attestation, and legacy route-authority refusal."
    exit: "Exact R1 SHA serves staging safely with self-development off; R0 pre-genesis rollback and R1 post-genesis recovery are both proven available, and the security ratchet boundary is frozen."
  - id: D-deployed-proposal-gate
    purpose: "Prove real deployed event and capsule behavior without allowing core state acceptance."
    entry: "C deployed and independently reconstructed exact R1 with mode off, and the signed public KernelCapabilityReceipt for the exact disposable ComputerID matches the G0 immutable-image receipt; R0 becomes permanently inadmissible when D commits GenesisImported."
    work:
      - "On one disposable ComputerID, create GenesisImported, activate audit_only, and prove append/privacy/restart/reconstruction/failure cases."
      - "Activate propose_only, run CLI-started cosuper capsule work, freeze and independently verify one harmless bundle, exercise role/capability/isolation/cleanup/refusal cases, and prove effective state unchanged."
    exit: "G2 accepts deployed audit/proposal evidence and authorizes one exact expiring accept_once operation; otherwise gate rejection rollback applies."
  - id: E-deployed-self-development-loop
    purpose: "Exercise the accepted, rejected, failed, restarted, reconstructed, and rolled-back product path."
    entry: "G2 bound accept_once to the exact ComputerID/base/bundle/operation."
    work:
      - "Approve and observe accepted→materializing→applied, marker, checkpoint/route receipts, runtime and computer restart, and reconstruction."
      - "Exercise every declared crash boundary; reject a second bundle; rollback to the prior applied head; prove old marker, retained history, and authority refusals."
      - "Set mode off after the bounded loop and freeze the deployed G3 packet."
    exit: "All finish acceptance classes have deployed immutable receipts; G3 independently retrieves, hashes, reconstructs, and verifies them from a clean external client, replays only non-mutating/refusal paths, and adjudicates the frozen packet."
  - id: F-terminal-closure
    purpose: "Close only after deployed consensus and safe dispositions."
    entry: "G3 accepted with no reproducible blocker."
    work:
      - "Record accepted source/deploy/computer/trajectory/event/capsule/checkpoint/route/rollback identities, classify dirty paths and capsules, confirm mode policy, and update terminal Definition/registries."
      - "Keep Choir-in-Choir and performance drafts blocked; choose no successor automatically."
    exit: "This Definition is complete historical evidence authority, registries are coherent, and no temporary candidate/capsule/scratch output remains."

orchestration:
  implementation_order: "A → G0 → B → G1 → C → D → G2 → E → G3 → F. Only read-only mapping and disjoint checks parallelize; doctrine authority, event head, acceptance, deployment, activation, updater, route projection, now, and registries serialize."
  state_authority:
    events: "Immutable event chain plus stable ComputerID head CAS."
    live_state: "Embedded Dolt and guest releases materialize the canonical effective head."
    observations: "Trajectory/EventRecord/Trace are joined projections/artifacts, never append authority."
    infrastructure: "ComputerVersion, route slot, lifecycle, status, and vmctl are reconstruction/serving projections and actuators."
  decision_gates:
    - id: G0-owner-contract-conformance
      after: A-contract-conformance
      before: B-implement-cutover-effects-off
      frozen_input_required: [owner_execution_contract, doctrine_diff, writer_caller_deletion_inventory, V1_schema_encoding, append_recovery_protocol, genesis_migration, constructor_credential_seam, receipt_fields_and_trust_roots, exact_route_transition_binding, privacy_retention, capability_TCB, updater_contract, public_CLI_API_auth, mode_operation_matrix, immutable_kernel_capability_receipt, deployed_kernel_receipt_contract, deletion_dispositions, rollback_refs]
      deterministic_first: "Standing-decision conformance; exact two-Dolt/corpusd table and typed transport map; no-third-store/host-dependency check; constructor credential delivery with no agent/host-daemon exposure; receipt canonicalization/signing/rotation/revocation; post-genesis sole route certificate and exact RouteTransitionCommand; complete writer/caller set; genesis/concurrency/state-machine totality; migration/recovery; public API/auth/decision/mode/lifecycle/refusal coherence; positive reproducible immutable-image kernel receipt; exact signed public no-SSH deployed receipt contract."
      review: "Diverse agentic-consensus panel verifies conformance only; it cannot change or ratify semantics."
      minority_rule: "Any unresolved owner decision, omitted writer, dual authority, host dependency, destructive migration, privacy hole, undefined rollback, missing mandatory immutable guest-kernel capability, or undefined public no-SSH deployed capability receipt sets blocked_incomplete and blocks B."
    - id: G1-frozen-effects-off-code-candidate
      after: B-implement-cutover-effects-off
      before: C-land-and-deploy-off
      frozen_input_required: [base_ref, candidate_ref, path_scope, content_digest, tests, migration_rehearsal, role_tool_inventory, API_contract_tests, updater_failure_tests, kernel_capability_receipt_tests, platform_rollback_ref]
      deterministic_first: "Build; event ordering/idempotency/tamper/pin/recovery; reducer/genesis; privacy canaries; role/tool absence; capability/isolation source contracts; signed kernel receipt identity/digest/tamper/staleness tests; updater state machine; API/auth/idempotency; mode-off refusal."
      review: "Diverse builder/falsifier/security/recovery panel over immutable source candidate."
      minority_rule: "Any reproducible data loss, authority leak, secret exposure, direct mutation, unsafe migration, or rollback failure blocks landing."
    - id: G2-deployed-pre-acceptance
      after: D-deployed-proposal-gate
      before: E-deployed-self-development-loop
      frozen_input_required: [source_SHA, deployed_identity, ComputerID, deployed_kernel_capability_receipt, genesis_head, audit_receipts, capsule_identity, capability_policy, bundle_digest, verifier_receipts, effective_state_proof, rollback_refs]
      deterministic_first: "Exact deployed kernel receipt matches G0 immutable-image evidence; kernel fail-closed isolation; broker/capability/revocation; role refusals; privacy/event reconstruction; crash recovery; stale target/base; proposal inertness; cleanup; no host/worker/candidate/internal path."
      review: "Diverse panel reviews deployed proposal-only evidence; it may authorize only the exact accept_once named in the packet."
      minority_rule: "Any core mutation before acceptance, isolation downgrade, missing event, secret leak, target ambiguity, or unrecoverable state triggers G2 rejection disposition."
    - id: G3-complete-deployed-loop
      after: E-deployed-self-development-loop
      before: F-terminal-closure
      frozen_input_required: [pushed_SHA, deployed_identity, external_CLI_transcript, accepted_event_head, effective_head, trajectory_refs, capsule_receipts, materialization_receipt, checkpoint_ref, route_generation, restart_reconstruction, failure_injection, rejection, rollback, authority_refusals, registry_diff]
      deterministic_first: "From a clean external client, independently retrieve and hash E's immutable receipts, reconstruct effective state, verify source/deploy identity and rollback availability, and replay only non-mutating reads and authority refusals. A second accepted mutation is forbidden without a new G2 authorization."
      review: "Full diverse agentic-consensus panel including Claude; findings are verified locally and one reproducible blocker overrides agreement."
      minority_rule: "Any reproducible deployed blocker invokes G3 rollback before repair; no model majority can complete the mission."
  prohibitions:
    - "No behavior code before G0; no deploy before G1; no accepted effect before G2; no closure before G3."
    - "No gate panel changes owner-settled semantics or substitutes for deterministic evidence."
    - "No moving candidate is reviewed; material semantic changes require a new frozen panel."

now:
  status: working
  slice: "B-disabled-cutover"
  question: "Can the complete source cutover implement the accepted G0 contracts with activation off, delete every obsolete worker/candidate path, and freeze one deterministic G1 candidate without weakening the two-store/event/route authority?"
  b_substrate_assessment:
    observed_at: 2026-07-19T04:25:00Z
    class: substrate
    status: "documented before the first B repair commit; G1 remains blocked"
    evidence: "Source-level product-path audit found that the renamed CoSuper commit tool still resolved a Super-owned lifecycle handle instead of a granted run-bound capability; `TransactionRecord` froze only classified paths/modes and no immutable runtime bytes; `choir-updater` had no runtime client/restart reconciliation; the public operation/decision/genesis/rollback CLI grammar was absent; and no checkpoint/route projection follows materialization. A proposal could therefore look frozen without being reconstructible or applyable."
    source_refs: [internal/agentcore/tools_capsule.go, internal/capsule/executor.go, internal/capsule/transaction/builder.go, internal/updater/updater.go, internal/agentcore/api_self_development.go, cmd/choir/main.go, internal/vmctl/route_authority.go]
    substrate_vs_symptom: "The shared substrate gap is incomplete guest effect custody from granted capsule capability through immutable release, accepted event, updater journal, checkpoint, and vmctl projection. Individual missing commands or state transitions are symptoms; B must complete the custody chain rather than patching status output."
    existing_replacement_check: "The root-isolated `choir-updater`, canonical ComputerEventAppender, routeledger/vmctl CAS, and capsule Executor already provide intended replacement substrates but were not wired into one path. B connects those replacements and deletes the metadata-only/process-local paths rather than extending them."
    partial_repair_state: "Run-bound CoSuper capability transfer, immutable release staging, private prompt/decision events, public CLI routing, operation/event recovery lookups, and updater apply reconciliation are implemented in the uncommitted candidate. They are not accepted evidence and may be revised."
    remaining_error: "Checkpoint issuance, certificate-only vmctl projection, rollback selection/rematerialization, exact verifier certificate semantics, deterministic failure injection, and end-to-end product-path tests remain incomplete. No G1 review, deployment, genesis, or effect activation is admissible."
    rollback: "Before deployment, discard the unlanded B candidate and retain R0. No platform schema, staging computer, event head, route, or effective guest state has changed."
    heresy_delta:
      discovered: 5
      introduced: 0
      repaired: 0
  rollback_substrate_repair:
    observed_at: 2026-07-19T06:15:00Z
    class: substrate
    mutation_class: red
    status: "documented before the first B behavior commit; implemented only in the uncommitted candidate; G1 remains blocked"
    problem: "The public CLI exposed `self-dev rollback`, but no public handler created RollbackRequested, no reconciler rematerialized a selected applied head, and applied operations retained neither their updater release digest nor route transition receipt. GenesisImported accepted only two hashes and did not pin the running immutable release or publish a baseline checkpoint, so the mission's first post-genesis rollback had no materializable prior release or vmctl rollback target. Appending a rollback request before validating those inputs would leave the canonical head permanently pending."
    source_refs: [cmd/choir/main.go, internal/agentcore/api_self_development.go, internal/agentcore/self_development_materializer.go, internal/selfdev/operations.go, internal/updater/updater.go, internal/vmctl/route_client.go]
    existing_replacement_check: "The updater release store, routeledger receipt history, vmctl immutable-input catalog, ComputerVersion route resolver, corpusd checkpoint authority, and event reducer already implement the intended authorities. The repair connects them; it creates no third semantic store and no host-local rollback path."
    repair: "The candidate now imports the immutable Nix baseline into the root updater once, publishes and records a genesis checkpoint/baseline operation, persists release/ComputerVersion/route receipt joins on every applied operation, resolves prior immutable inputs through a typed internal vmctl endpoint, validates all prior receipts and CAS heads before RollbackRequested, and idempotently applies the pinned prior release through RollbackApplied, checkpoint publication, certificate-only vmctl rollback CAS, and terminal `rolled_back` state."
    evidence: "`go test ./internal/updater -run TestUpdaterImportsImmutableBaselineOnce -count=1 -v`; `go test ./internal/vmctl -run 'TestClientPinsInputsAndTransitionsRoute|TestSelfDevelopmentRouteProjectionRequiresExactPlatformCertificate' -count=1 -v`; `go test ./internal/selfdev -run TestRollbackStartBindsPriorAppliedReceiptsAndReplays -count=1 -v`; `go test ./cmd/choir -run TestSelfDevelopmentCLIUsesExplicitTargetAndImmutableBindings -count=1 -v`; and full affected Go package suites passed. `nix eval --raw .#nixosConfigurations.go-choir-sandbox-vm.config.systemd.services.go-choir-sandbox.environment.CHOIR_BASELINE_RELEASE_ROOT` resolved the immutable sandbox package path."
    admissible_evidence_class: "focused deterministic source tests and Nix evaluation only; no deployed rollback, restart, route, or genesis claim"
    rollback: "Before deployment, discard the unlanded candidate and retain R0. No platform row, updater pointer, ComputerID event head, checkpoint, or route changed."
    protected_surfaces: [GenesisImported, computer_event_head_CAS, guest_updater, ComputerVersion_checkpoint, vmctl_route_projection, rollback_receipts, additive_Dolt_schema]
    heresy_delta:
      discovered: 1
      introduced: 0
      repaired: 1
    conjecture_delta: "The two-store/single-appender/vmctl-actuator topology is unchanged. The new evidence rejects the conjecture that CLI grammar plus reducer cases constituted rollback; durable pinned release and route-receipt custody are required."
    remaining_error: "The genesis verifier evidence is still represented only by a digest rather than an independently verifiable certificate, deterministic rollback failure injection and a complete API-to-updater-to-vmctl product-path test remain absent, and no frozen whole-candidate G1 review has occurred."
  g1_candidate_rejection:
    observed_at: 2026-07-19T07:10:00Z
    class: substrate
    mutation_class: red
    status: "documented before any B behavior commit; frozen tree b2daf1c64dc56d43405b7c72bf026d7ffc3f52e4 is rejected and must not land"
    problem: "The first G1 source freeze retained dormant but callable raw writable/coding registries and live vmctl fork-desktop, publish-desktop, request-worker, and hibernate-worker handlers/clients/ownership paths. The live Texture prompt and run-acceptance interpreter still taught and credited worker-VM/AppChangePackage execution. This violates the owner-settled complete deletion of worker/candidate VM concepts and allows internal callers to bypass the new durable agent/capsule model even though Super's default registry no longer exposed the tools."
    evidence: "Local verification of the panel's low-severity dead-authority finding found a broader substrate blocker: `internal/agentcore/tools_coding.go` still constructed write/edit/bash tools behind latent policy booleans; `internal/agentcore/tools_vmctl.go` retained fork/publish tools; `internal/vmctl/handlers.go` still registered fork/publish/request-worker/hibernate-worker endpoints; `internal/vmctl/client.go`, `internal/vmctl/ownership.go`, `internal/vmmanager`, `internal/agentcore/run_acceptance.go`, and `internal/textureprompts/overlays/run_system.yaml` still carried the obsolete topology. The 1/7 healthy G1 panel member accepted the frozen tree only after classifying this as non-blocking; owner-settled deletion semantics override that classification."
    existing_replacement_check: "Durable CoSuper/Researcher runs, guest-local capsules, the run-bound capsule capability, ComputerEventAppender, and the public self-development operation replace the worker/candidate VM topology. The repair deletes latent registries, handlers, clients, prompts, acceptance credit, configuration, and comprehensive legacy tests rather than disabling or aliasing them."
    admissible_evidence_class: "a new frozen source tree, negative symbol/route inventory, focused deterministic tests, complete Go compilation, Nix evaluation, and a healthy diverse G1 review; no deploy is admissible before acceptance"
    rollback: "Discard the unlanded rejected tree and retain R0. No staging route, ComputerID event head, updater release, checkpoint, or effective state changed."
    heresy_delta:
      discovered: 1
      introduced: 0
      repaired: 0
    remaining_error: "Complete the cross-repository deletion, migrate live prompts and run acceptance to durable agent/capsule evidence, rerun deterministic suites, and freeze a new tree. The prior b2daf1c6 review is stale."
  g1_pre_freeze_recovery_audit:
    observed_at: 2026-07-19T09:09:34Z
    class: substrate
    mutation_class: red
    status: "documented before the B repair commit; repairs exist only in the uncommitted candidate; G1 remains blocked"
    problem: "Focused pre-freeze execution review found seven coupled custody failures: the capsule masked the persistent source path named by CoSuper and had no immutable tracked source lower; accept_once remained effective after expiry and approval omitted desired/effective state commitments; vmctl issued the guest credential to the realization VMID while public operations target the stable ComputerID and the materializer scanned operations under SandboxID; explicit rollback passed a retained release directory to an updater that accepted only incoming directories; verifier certificates changed identity on every retry and checkpoint/route requests rebound to later causal heads, making restart reconciliation conflict with their own idempotency keys; the capsule admission cleanup used a non-recursive unmount; and the guest Nix evaluation referenced a nonexistent standalone `npm` package."
    source_refs: [internal/capsule/executor.go, internal/capsule/source_snapshot.go, internal/platform/self_development_modes.go, internal/proxy/self_development_operations.go, internal/agentcore/api_self_development.go, internal/vmctl/ownership.go, nix/sandbox-vm.nix, internal/agentcore/self_development_materializer.go, internal/updater/updater.go, internal/updater/verifier_certificate.go]
    existing_replacement_check: "The existing stable desktop selector, route/event state commitments, updater retained release store, updater operation journal, corpusd idempotent checkpoint authority, and capsule overlay substrate are the intended replacements. The repair binds and reuses them; it adds no semantic store, worker VM, host mutation route, SSH path, or mutable branch."
    repair: "The uncommitted candidate snapshots only clean Git-tracked source into an immutable capsule lower at `/workspace/platform` and exposes its digest; durably expires accept_once and binds both state commitments at CLI, proxy, and guest; propagates stable DesktopID as ComputerID while retaining VMID solely as realization; admits exact read-only retained releases for rollback; journals verifier certificates and derives checkpoint/route joins from their exact event receipts with renewable short-lived route authorizations; recursively detaches failed capsule mount trees; and restores evaluable Nix package composition."
    evidence: "`go test ./internal/capsule ./internal/platform ./internal/proxy ./internal/agentcore ./internal/updater ./internal/vmctl ./cmd/choir` passed after the repairs. `go test ./...` passed every package except live provider/search integration tests whose upstream accounts returned exhausted quota (Tavily 432, Serper 400, SerpAPI 429, ZAI 429). Linux capsule tests compile with `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go test -c`. Nix parse and guest service evaluation now resolve kernel config and `/mnt/persistent/files/Source/platform` without an evaluation error."
    admissible_evidence_class: "focused deterministic source tests, complete offline package compilation excluding externally quota-bound integration calls, Linux cross-compilation, and Nix evaluation; no deployed kernel, rollback, restart, route, capsule, or product-path claim"
    protected_surfaces: [stable_ComputerID, realization_credential, accept_once_mode, guest_capsule_source, verifier_certificate, checkpoint_event_join, vmctl_route_projection, updater_rollback, restart_reconciliation]
    rollback: "Before deployment, discard the unlanded candidate and retain R0. No staging mode, credential, capsule, event head, updater pointer, checkpoint, route, or effective release changed."
    heresy_delta:
      discovered: 7
      introduced: 0
      repaired: 0
    conjecture_delta: "The topology remains two Dolt stores, one canonical event appender, guest-local capsule/updater effects, and vmctl-only route CAS. The audit rejects the conjecture that locally idempotent components imply restart durability; every cross-component request must bind a stable event receipt or retain its first signed response."
    remaining_error: "Commit this problem checkpoint separately, then commit the coherent repair, rerun frozen-tree verification and negative inventories, and obtain a healthy diverse G1 review. No deployment or mode activation is admissible."
  discovered_blocker:
    observed_at: 2026-07-18T23:58:00Z
    class: substrate
    status: "G0 contract repair required before the B candidate may freeze"
    evidence: "`ComputeRequestCommitment` binds payload PinReceipt digests, while each PinReceipt is required to bind that same final request commitment. PinReceipt IDs/timestamps/signatures make the receipt digest unknowable before pinning, so private prompt/result payloads cannot be pinned and appended without a circular hash dependency. Existing tests inject preselected receipt digests and therefore did not execute the public pin-to-append transition."
    source_refs: [internal/computerevent/request.go:8-48, internal/computerevent/appender.go:77-136, internal/platform/event_artifacts.go:62-91, internal/platform/event_artifacts.go:94-166]
    belief_delta: "The two-store/single-appender topology remains sound, but the frozen V1 request/pin commitment relation is not executable as written. B must separate a pre-pin immutable intent commitment from the final append request commitment, bind both in the PinReceipt and append CAS, add an end-to-end private-payload append test, and refreeze this narrow semantic delta through G0 before G1."
    remaining_error: "No behavior candidate may be frozen or deployed until the repaired commitment graph has deterministic evidence and independent G0 review."
    repair:
      implemented_at: 2026-07-19T00:42:00Z
      mutation_class: red
      protected_surfaces: [canonical_event_commitments, PinReceipt, EventHeadReceipt, corpusd_head_CAS, embedded_projection_recovery]
      result: "Implemented a directed commitment graph: immutable pin intent -> payload PinReceipt digests -> final request commitment -> event PinReceipt/head CAS. `TestPrivatePayloadAppendCompletesDirectedCommitmentGraph` exercises real XChaCha20 private-envelope pinning through corpusd CAS and embedded finalization."
      evidence: "`go test ./internal/platform -run TestPrivatePayloadAppendCompletesDirectedCommitmentGraph -count=1 -v` passed; full affected packages remain required after G0 refreeze."
      rollback: "Before deployment, revert the unlanded candidate. No schema or deployed state changed."
      heresy_delta:
        discovered: 1
        introduced: 0
        repaired: 1
      conjecture_delta: "The single-appender/two-store architecture is retained; only the non-executable cyclic commitment edge is replaced."
      gate: "Independent G0 review of this exact repaired contract and implementation evidence remains mandatory before B resumes."
      first_panel:
        frozen_diff_sha256: 1142f7ff1f87bcbfe1045de05459938d9d924da2420ca05cb7ab13bdfdc781b0
        health: "2/8 completed; Cursor and OpenCode succeeded, four timed out, two provider/model runs failed."
        blocker: "Cursor reproduced a Definition/code verifier divergence: prose could be read as hashing only pin_intent_commitment plus receipt digests while code hashes canonical `{event_intent,pin_intent_commitment,payload_pin_receipt_digests}`. OpenCode accepted the runtime graph but did not override the reproducible authority ambiguity."
        disposition: "Problem recorded and Definition corrected to the exact canonical object already implemented. No runtime behavior changed. Material Definition correction invalidates the first review identity; a fresh diverse panel is required."
      second_panel:
        frozen_candidate_sha256: aa5fd11cafc3028f29c931e60c34cc9dea123ccafc3fb9215d1a476762025175
        health: "4/4 completed: Codex, Claude, Cursor, and OpenCode."
        consensus: "Claude, Cursor, and OpenCode accepted the corrected formula."
        minority_blocker: "Codex reproduced two private-payload product-path failures. `PinPrivatePayload` encrypts with a random nonce and pins in one call but requires pin_intent_commitment before the resulting envelope digest can be placed in event_intent; the in-process test bypasses the unusable public HTTP sequence. Append validation also accepts a private envelope whose metadata EventID belongs to a different event, producing an accepted payload that later refuses decryption."
        source_refs: [internal/computerevent/http_client.go:77-110, internal/platform/event_artifacts.go:92-171, internal/platform/event_artifacts_test.go:102-184]
        disposition: "Reproducible minority blocker overrides agreement. G0 remains blocked. B must expose a two-phase private-envelope freeze then exact-envelope pin API, exercise it through HTTP client/handler, validate private metadata ComputerID and EventID against the appended event, and refreeze."
        repair:
          implemented_at: 2026-07-19T01:39:00Z
          result: "Replaced encrypt-and-pin with public `PreparePrivatePayload` then `PinPrivatePayload` over the exact frozen envelope; append validation now rejects private envelope metadata whose ComputerID or EventID differs from the appended event."
          evidence: "`TestPrivatePayloadAppendCompletesDirectedCommitmentGraph` now uses the authenticated HTTP client/handler for payload pin, event pin, head CAS, receipt verification, and head read, and separately proves a cross-event envelope refusal. Focused test passed."
          rollback: "Before deployment, revert the unlanded candidate. No deployed schema or state changed."
          heresy_delta:
            discovered: 2
            introduced: 0
            repaired: 2
          gate: "A fresh frozen G0 panel must verify both repairs; the second panel remains rejected."
      third_panel:
        frozen_candidate_sha256: 4316ced83fd43d23288009674a362232936a29ba88c255017a5f11f02df0cf39
        health: "4/4 completed: Codex, Claude, Cursor, and OpenCode."
        consensus: "Claude, Cursor, and OpenCode accepted both prior repairs."
        minority_blocker: "Codex demonstrated that both the public pin client and corpusd only parsed plaintext envelope metadata. A caller could rewrite EventID or ciphertext, re-canonicalize, and obtain structurally valid pins; append would accept content that the guest keyring later rejects as unauthentic."
        disposition: "G0 remains blocked. Because corpusd must not receive the guest decryption key, the guest appender TCB must AEAD-decrypt/authenticate the exact frozen envelope against expected ComputerID/EventID immediately before pin; corpusd retains structural identity/content/pin validation. The authenticated public-path test must include metadata/ciphertext tamper refusal before another refreeze."
        repair:
          implemented_at: 2026-07-19T02:01:00Z
          result: "`PinPrivatePayload` now requires the guest `PrivateArtifactCipher` and successfully AEAD-decrypts the exact envelope against expected ComputerID/EventID before sending any pin request. corpusd remains structurally validating and keyless."
          evidence: "The authenticated HTTP product-path test now mutates canonical metadata and ciphertext independently; both refuse before pin. The valid private envelope still completes payload pin, event pin, corpusd CAS, EventHeadReceipt verification, embedded finalization, and head read."
          rollback: "Before deployment, revert the unlanded candidate. No deployed schema or state changed."
          heresy_delta:
            discovered: 1
            introduced: 0
            repaired: 1
          gate: "A fresh frozen G0 panel must verify the AEAD-authenticated product path; the third panel remains rejected."
      fourth_panel:
        frozen_candidate_sha256: 5ea756814a3ca1ef9551c3f5d3864e985370848e69f0355e7c3378c5caf4c330
        health: "3/4 completed; Codex, Claude, and OpenCode succeeded; Cursor timed out."
        consensus: "Claude and OpenCode accepted the AEAD-authenticated client path."
        minority_blocker: "Codex traced the bootstrap credential envelope through kernel argv into `/run/go-choir-sandbox.env`, which is shared with the separate root updater, and found exchange replay returns the full event:append bearer. Non-appender updater code can therefore bypass the AEAD-validating guest client, submit structurally valid invalid ciphertext over raw HTTP, and obtain an authoritative head CAS."
        source_refs: [nix/sandbox-vm.nix:286, nix/sandbox-vm.nix:343, internal/platform/credential_envelope.go:168, internal/platform/event_handlers.go:139, internal/platform/computer_events.go:143]
        disposition: "G0 remains blocked. Remove append credentials from kernel argv and updater/shared environment, deliver them through an appender-only root credential boundary, make consumed exchange refuse replay rather than returning the bearer, and prove a non-appender principal cannot reach append CAS."
        repair:
          implemented_at: 2026-07-19T02:44:00Z
          result: "Removed the envelope from Firecracker kernel arguments and the shared runtime environment. vmmanager now formats a dedicated realization-local ext4 credential drive; the guest mounts it outside the updater namespace, trusted core enforces root/mode/regular-file bounds and unlinks before exchange, and consumed exchange replay refuses without a bearer."
          evidence: "`TestComputerCredentialUsesDedicatedDiskNotKernelArguments`, `TestConsumeComputerCredentialEnvelopeErasesSingleUseFile`, `TestConsumeComputerCredentialEnvelopeRejectsLooseMode`, and `TestCredentialEnvelopeExchangeRefusesReplay` pass. Six affected Go package suites pass; NixOS guest toplevel evaluation succeeds."
          rollback: "Before deployment, revert the unlanded candidate and retain pre-genesis R0. No deployed credential or event state changed."
          heresy_delta:
            discovered: 2
            introduced: 0
            repaired: 2
          gate: "A fresh frozen G0 panel must verify the appender-only credential boundary; the fourth panel remains rejected."
      fifth_panel:
        frozen_candidate_sha256: 1229c61e177f155c65be0d8475635474fe9ebe4f167a2eb320dc2c55af4eb5ee
        health: "3/4 completed: Codex, Cursor, and OpenCode; Claude timed out."
        consensus: "Cursor and OpenCode accepted the file/disk/replay boundary."
        minority_blocker: "Codex showed the root updater still shared guest process visibility and retained CAP_SYS_PTRACE/debug syscalls. It could read the trusted sandbox's in-memory renewable append bearer and bypass AEAD-validating client code despite the credential mount mask."
        source_refs: [nix/sandbox-vm.nix:326-356, internal/selfdev/credentials.go:20-31]
        disposition: "G0 remains blocked. Keep the root-owned updater authority but isolate its PID namespace, empty its capability bounding set, deny debug/process-memory syscalls, and prove the generated service unit carries those restrictions."
        repair:
          implemented_at: 2026-07-19T03:11:00Z
          result: "The root updater now runs in a private PID namespace with empty capability/ambient sets, invisible PID-scoped procfs, and `~@debug` syscall filtering. It cannot inspect trusted-core memory. Its only environment file contains nonsecret ComputerID/realization identity; credential mount, gateway token, and append bearer remain absent."
          evidence: "Nix evaluation of the generated updater serviceConfig proves `PrivatePIDs=true`, `ProtectProc=invisible`, `ProcSubset=pid`, empty `CapabilityBoundingSet`/`AmbientCapabilities`, `SystemCallFilter=[~@debug]`, credential `InaccessiblePaths`, and the identity-only environment file."
          rollback: "Before deployment, revert the unlanded candidate. No deployed service namespace or credential changed."
          heresy_delta:
            discovered: 1
            introduced: 0
            repaired: 1
          gate: "A fresh frozen G0 panel must verify updater cannot recover the appender bearer; the fifth panel remains rejected."
      sixth_panel:
        frozen_candidate_sha256: b4f3a3b28943fe58ba04ca0df39cd01ef93d0af9e5e2ca93e9f0eb180924880a
        health: "Initial run completed Cursor only; retry completed Codex and OpenCode while Claude was rate-limited."
        consensus: "Cursor accepted the process-isolated updater."
        minority_blocker: "Codex showed root updater still had a general PID 1 control path through `systemctl`; it could create a transient unrestricted root service outside its namespace, read bootstrap credentials, or inspect trusted-core memory."
        source_refs: [internal/updater/runtime.go:15-29, cmd/choir-updater/main_linux.go:25-43, nix/sandbox-vm.nix:337-395]
        disposition: "G0 remains blocked. Replace general systemctl access with a fixed restart trigger and root-owned path/oneshot bridge, mask PID 1 control sockets from updater, and prove updater can name no arbitrary unit."
        repair:
          implemented_at: 2026-07-19T03:49:00Z
          result: "Updater now atomically writes only `/run/choir-updater-control/restart`. A root-owned Nix-store path unit removes the trigger and restarts exactly `go-choir-sandbox.service`. Updater masks systemd/dbus control sockets and retains its empty capabilities/private PID/debug restrictions."
          evidence: "`TestRestartRequestManagerPublishesOnlyFixedTrigger` proves the fixed trigger and arbitrary-target refusal; updater package/binary compile, NixOS toplevel evaluation, and generated path-unit evaluation pass."
          rollback: "Before deployment, revert the unlanded candidate. No deployed service control path changed."
          heresy_delta:
            discovered: 1
            introduced: 0
            repaired: 1
          gate: "A fresh frozen G0 panel must verify no general PID 1 escape; the sixth panel remains rejected."
      seventh_panel:
        frozen_candidate_sha256: ec65831c1df8abf7b068e49bc6e7a1c2640a1aa327d5d1c494f065f654d2c203
        initial_health: "OpenCode completed and accepted; Codex was interrupted by provider policy and Cursor timed out."
        retry_health: "Codex, Cursor, and OMP GPT-5.5 completed successfully."
        verdict: accepted_G0_repair
        findings: "All three retry reviewers reported ACCEPT_G0_REPAIR with no reproducible blocker. They verified production has no general updater service-manager authority: fixed atomic trigger, fixed path unit, immutable exact-unit oneshot, masked systemd/dbus sockets, process/capability restrictions, and arbitrary-target refusal."
        evidence_ref: "docs/evidence/self-development-g0-conformance-2026-07-18.md terminal repair panel receipt; prompt/manifest/output SHA-256 identities are recorded there and raw `/tmp` transcripts are diagnostic only."
        acceptance: "G0 is re-accepted and B may resume. The only post-review mutation is this assurance receipt and matching evidence prose; it does not alter the reviewed runtime, Nix, protocol, or test content, so no rerun is required."
        residual_risk: "The fixed path and exact unit name remain deployment invariants; G1 must verify the frozen complete B candidate and deployed R1 must prove namespace behavior in the real guest."
  reconciliation:
    observed_at: 2026-07-18T22:32:45Z
    source_ref: refs/heads/main@5483a082d0012890343deb3693eea15c53a98415_equals_refs/remotes/origin/main@5483a082d0012890343deb3693eea15c53a98415_before_G0_docs
    deploy_identity: "choir.news /health reported proxy commit 2bc1799f72ce437b35d4606a23d14e62b7239ac5; A changes documentation only and claims no new behavior"
    authority_identities: [owner_whole_mission_mandate:2026-07-18, predecessor:docs/definitions/choir-audited-autoputer-construction-2026-07-15.md@complete, successor:docs/definitions/choir-in-choir-computer-control-draft-2026-07-18.md@blocked]
    worktree_inventory_ref: "Intentional A/G0 candidate modifies this Definition and six doctrine/ontology/current-state support docs and adds one conformance evidence packet; no runtime file changed."
    status: reconciled
  prerequisite_preflight:
    observed_at: 2026-07-18T21:41:00Z
    constructor_credential_seam:
      status: proven_existing_seam
      source_receipt: "ProductionMaterializer passes one populate callback into diskinstantiation.Backend.Instantiate; Ext4Backend executes that callback in a fresh realization root before `mkfs.ext4 -d`; VMConstructionLauncher attaches the resulting constructed data device directly to the realization without recreating it. vmctl runs as root on Node B, so a constructor-created mode-0400 credential inode is root-owned without chown or a new host service. Semantic construction observations enumerate the dedicated `files/` subtree, so a sibling `credentials/` inode is excluded from CodeRef/ArtifactProgramRef and observation claims. The guest data device becomes mutable after launch; this proves delivery, not cryptographic immutability after boot."
      source_refs: [internal/computerversion/production_materializer.go:69-191, internal/diskinstantiation/ext4.go:26-125, internal/vmctl/construction_launcher.go:32-91, nix/node-b.nix:84-108, nix/node-b.nix:456-492]
      experiment_receipt: "A disposable 32 MiB ext4 image populated through `mkfs.ext4 -d` contained `/credentials/computer-event-envelope` byte-for-byte at mode 0400; image SHA-256 47ff7d6fb69974a1fb4d68c7376b052e8ac72e0d99dcdf7febb2d5d3b83cf940. The local source inode retained uid 501, confirming B must rely on the existing root vmctl service (or explicitly assert uid/gid 0) rather than assuming mkfs rewrites ownership."
      implementation_constraint: "B must create the envelope only inside the root-run per-realization staging root, assert uid/gid 0 and mode 0400 before image construction, exclude it from observations and logs, consume/revoke it in the guest core, and add leakage/reconstruction tests."
    immutable_guest_kernel:
      status: proven_for_pinned_guest_configuration
      pinned_input: "flake.lock nixpkgs 4c1018dae018162ec878d42fec712642d214fdfa; evaluated go-choir-sandbox-vm kernel 6.18.21"
      config_receipt: "Realized config /nix/store/252bxb6q8p4fpza6bj0v4ndr98vxrnhk-linux-config-6.18.21, SHA-256 5abba8875e79ba9c8bcd7d9604d137af310641dc44caf536424dc2cdd4c032eb: CONFIG_USER_NS/PID_NS/NET_NS/UTS_NS/IPC_NS/CGROUPS/MEMCG/CGROUP_PIDS/CGROUP_BPF/SECCOMP/SECCOMP_FILTER/SECURITY_LANDLOCK=y; CONFIG_MEMCG_V1 is unset; CONFIG_OVERLAY_FS=m."
      boot_receipt: "Evaluated microvm.kernelParams contains `lsm=landlock,yama,bpf` and no cgroup-v1 override. NixOS/systemd 256 removes supported legacy/hybrid mode and defaults to cgroup v2. Realized modules tree contains overlay.ko.xz, SHA-256 a2004b3492257fc1d471fd607aed53537c1dc181b5d8d41024c6b697c2c3fcab."
      disposition: "All mandatory immutable-image capabilities are positive; no kernel/NixOS/Firecracker repair is indicated. The current public computer status proves a served immutable ComputerVersion but does not bind its running guest to a kernel/config digest. That known observability gap is B work and a hard C-before-D check, not an impossible pre-target G0 requirement."
  candidate:
    id: self-development-B-disabled-cutover-round-10
    state: rejected_G1_repairing
    ref: d5f3b4778439bb71745e951712a229993300d51d
    owner: integration-authority
    base: 5483a082d0012890343deb3693eea15c53a98415
    scope: "Effects-OFF B cutover after structural owner decisions and the accepted exact Linux boot harness. Round-10 review confirms five previously documented source blockers were never connected to those settled decisions: proposal start still passes a raw encrypted-envelope digest where Store.Start requires `artifact:sha256:`; proxy exact start retry remains mode-gated; exact approval replay excludes legal post-decision descendants; startup decision recovery bypasses the shared complete join; and the mapped-UID broker child still creates a pathname socket inside the parent-root mode-0700 directory instead of receiving a parent-owned listener FD."
    prior_candidates: [7d635330bf14bd8be505291c6a9d807264650afe, 8bad0a25aa4dc4d4e5fc4ce1a60314a0721f1135, f9cc324633fc64a40c407aa8abd328f9b257127a, 5ae5b6106bf60610b2404e4b1b1f5f26865c337e, 32b315971dc4939ccf8499d7740336300d5da81a, fb0e56e33de17fbf7cf7326b345fa701d6a241a3, 153c68668a8b16f47ff5fba17a983d2d37339cbb, 18e4f9dbfb37eb7d518103a8315542bc11f02f92, ae881720132809d6d6092b4a739e43a311489000]
    verification: "Six of seven reviewers completed; Cursor, Codex, Devin, and omp-gpt55 rejected while OpenCode and Gemini accepted and GLM failed before review. Local source inspection confirms all five blocker classes at the cited paths. These are the same round-9 symptoms for which the owner subsequently settled typed artifact refs, any-mode exact replay, one shared decision-binding verifier, parent-owned inherited broker listener, and Node A Linux proof; the harness substrate was repaired, but the product callsites were not."
    disposition: "Rejected at G1. Effects remain OFF and no deployment is authorized. The owner's structural recovery decision authorizes one coherent repair connecting the five exact settled semantics, followed by an Executor.Spawn Linux proof and a newly frozen G1 panel."
  g1_round_11_probe:
    observed_at: 2026-07-19T23:31:00Z
    status: rejected_capsule_admission_substrate
    source_identity: 8b258d3bf7f75ffae1657c5cdef9272c5d21bc7c
    mutation_class: red
    protected_surfaces: [guest_capsule_isolation, cgroup_v2_admission, inherited_broker_listener]
    evidence_class: "Exact Node A x86_64-linux opt-in Executor.Spawn integration using the immutable Nix capsule-broker. The test creates a clean frozen source repo, an isolated overlay lower root, and calls the production Executor path as root."
    success_before_blocker: "The source candidate passed the focused Go packages, exact Nix guest-image build, and exact Firecracker effects-OFF boot harness at build identity 8b258d3b. Canonical pinned input/output artifact references, shared decision recovery verification, guest-owned mode consumption, and parent-owned listener FD transfer are present in source."
    problem: "`TestExecutorInheritedBrokerListenerEndToEnd` failed before broker spawn with `failed to create cgroup capsule/g1-listener-3455982: cgroups: invalid group path`. `CreateCgroup` passes relative `capsule/<id>` to containerd/cgroups v2 `NewManager`; that API requires an absolute cgroup path. The effects-OFF boot harness did not exercise Executor.Spawn and therefore could not reveal this admission blocker."
    followup_problem: "After correcting the disposable probe to use absolute `/capsule/<id>`, the broker reached the inherited listener process but logged `failed to mount capsule procfs: operation not permitted`; the parent then reported readiness timeout. The current one-stage `SysProcAttr` creates USER and PID/mount/network/UTS/IPC/cgroup namespaces simultaneously. The leading hypothesis is that the new PID namespace is owned by the creator's user namespace, so UID 0 in the simultaneously created child user namespace lacks mount authority for procfs. The child also remains an unreaped zombie during readiness, so signal-0 cannot report the early exit."
    followup_disposition: "Documented before namespace-launcher repair. Do not remove the user namespace, proc isolation, Landlock, seccomp, or any mandatory isolation probe. Implement a two-stage launcher that enters the mapped user namespace first, then creates the remaining namespaces and forks the broker PID; preserve fd 3 and cgroup inheritance, fail fast on launcher/broker exit, and prove the exact production path."
    disposition: "Documented before repair. G1 remains rejected and effects remain OFF. This is a capsule substrate blocker, not a deployment or product-acceptance gap."
    next_probe: "Keep absolute `/capsule/<id>`, implement and test ordered user-then-PID/mount/network/UTS/IPC/cgroup namespace creation without weakening isolation, retain parent listener ownership and authenticated readiness, and rerun the exact Node A Executor.Spawn integration through reconnect and complete cgroup/mount/socket cleanup."
    rollback: "Revert the focused cgroup path commit; no deployed state or production cgroup was mutated by the failed disposable probe."
  g1_round_12_probe:
    observed_at: 2026-07-20T00:23:00Z
    status: rejected_checkpoint_reference_and_stale_authority
    source_identity: 00d25827e249ec9d59052b5b3e5a28eaf546b662
    mutation_class: red
    protected_surfaces: [checkpoint_reconstruction, canonical_artifact_authority, current_product_guidance]
    evidence_class: "Frozen-source independent review plus local source confirmation. The candidate passed focused authority packages, exact Nix guest-image construction, and the Node A Executor.Spawn capsule lifecycle proof before review."
    success_before_blocker: "Finalized start events now repair missing operation projections before current-mode gating and bind the original public request commitment; immutable event artifact refs and downstream authority joins require canonical typed refs; the production capsule path passes namespace, cgroup, inherited-listener reconnect, and cleanup proof."
    problem: "`CheckpointAuthority.Publish` still stores `checkpoint_artifact_ref` as the invented, unresolvable `artifact://sha256/<digest>` form rather than canonical `artifact:sha256:<digest>`. Separately, current authority text in `docs/computer-ontology.md` and `docs/current-architecture.md` still says the now-wired event/updater/public API/capsule substrate is absent or inert and retains deleted direct-role/worker/package/host-authority paths as current. These violate the owner-settled typed-reference and deletion-citer contracts."
    disposition: "Documented before repair. G1 remains rejected and effects remain OFF. Replace the checkpoint reference through the canonical constructor, update only the stale current-state claims to the exact effects-OFF cutover state, add focused reconstruction/reference coverage, and freeze a new candidate for a clean round-13 panel."
    rollback: "Revert the future focused checkpoint/current-guidance repair; no deployed effect or route was enabled by this rejected candidate."
  g1_round_13_probe:
    observed_at: 2026-07-20T00:45:00Z
    status: rejected_start_run_recovery_and_deletion_citers
    source_identity: f5d5a76dd9aebc9672da08a40e93c4e359788f36
    mutation_class: red
    protected_surfaces: [proposal_crash_recovery, durable_run_binding, deletion_citers]
    evidence_class: "Frozen-source independent review plus local source confirmation; focused packages, exact guest-image build, exact effects-OFF Firecracker boot/restart identity proof, and prior unchanged Executor.Spawn proof passed."
    success_before_blocker: "Checkpoint persistence now uses canonical typed artifact refs and current ontology/architecture describe the effects-OFF cutover accurately."
    problem: "Event-first start recovery reconstructs a requested operation and returns before `ensureSelfDevelopmentRun`; every later retry takes the existing-operation fast path, so a crash after `TrajectoryStarted` can strand the proposal without a durable run. The deletion-citer sweep also missed current `specs/README.md` and `specs/promotion_protocol.tla` candidate-branch/route-flip authority plus README/capsule comments that still name deleted `capsule-host`/HostAuthority surfaces as current."
    disposition: "Documented before repair. G1 remains rejected and effects remain OFF. Reuse the normal run-binding path after deterministic operation repair, strengthen the event-first crash regression to require one executing bound run, and remove or rewrite every obsolete current citer without preserving a compatibility path."
    rollback: "Revert the future focused recovery/citer repair; no deployed effect, run, or route was enabled by this rejected candidate."
  g1_round_14_probe:
    observed_at: 2026-07-20T01:08:00Z
    status: rejected_remaining_atomicity_and_obsolete_promotion
    source_identity: 2fdd63f9078a8c6400d1852c693603e382c52bb6
    mutation_class: red
    protected_surfaces: [durable_run_binding, credential_scope, updater_recovery, bundle_identity, obsolete_promotion]
    evidence_class: "Frozen-source independent review with focused reproducible tests and local source confirmation; exact guest-image/effects-OFF boot and unchanged capsule lifecycle proofs passed."
    success_before_blocker: "Event-only proposal recovery creates one executing operation/run in the covered path; obsolete promotion TLA gate and host capability citers are removed."
    problem: "Five source joins remain incomplete: an existing requested operation still returns before missing-run repair and concurrent list-then-create can duplicate runs; event-read alone authorizes credential renewal that mints append/pin scope; updater restart failure after pointer swap neither restores the prior release nor returns signed recovery; materialization does not compare bundle ComputerID/TrajectoryRef/CapsuleIdentity to the durable operation; and the exported DoltPromotionAdapter plus guest env/current citers still preserve callable tag/commit/reset candidate promotion."
    disposition: "Documented before repair. G1 remains rejected and effects remain OFF. Repair all five at their shared authority boundaries: serialize and repair exactly-one operation/run binding, require append scope for renewal, make post-swap restart failure rollback-and-receipt complete, bind every bundle identity, and delete the obsolete promotion adapter/config/citers without a shim."
    rollback: "Revert the future coherent authority repair; no deployed self-development mode, run, route, or release was enabled by this rejected candidate."
  g1_round_15_probe:
    observed_at: 2026-07-20T01:44:00Z
    status: rejected_stale_detector_exclusion
    source_identity: 5a922b2bdf7ff676ed14c0cf0c6581c7933542c8
    mutation_class: yellow
    protected_surfaces: [deletion_detector, obsolete_promotion_regression]
    evidence_class: "Six substantive frozen-source reviewers completed: five accepted and one supplied a reproducible severe deletion-citer blocker; exact Node A guest-image, effects-OFF boot identity, and Executor.Spawn lifecycle proofs passed."
    success_before_blocker: "All five round-14 authority gaps are repaired and their focused tests pass; the obsolete adapter/source/config/current product citers are deleted."
    problem: "The live I4 heresy-detector manifest still excludes `DoltPromotionAdapter` and `dolt_promotion_adapter.go` by design. Deletion succeeded, but the guard would ignore a future reintroduction of the same destructive embedded-reset authority."
    disposition: "Documented before repair. Severe-minority rule rejects G1 despite five accepts. Remove the obsolete exclusion, detect both destructive reset and adapter-symbol reintroduction in production, run the detector, then freeze a final candidate for a clean gate."
    rollback: "Revert the focused detector-manifest repair; no deployed behavior is involved."
  decision:
    selected: "Execute the entire A→F mission under the fixed execution contract above. Candidate VMs and worker VMs are obsolete and their code is deleted; generic delegated agents use durable runs/trajectories and capsules. A/G0 reconciles rather than invents semantics; implementation lands with only self-development activation off; deployed G2 precedes the one bounded acceptance; G3 precedes closure."
    kind: architecture_and_execution_authority
    status: settled
    source: owner
    settled_by: owner
    evidence_ref: "Owner whole-mission instruction and explicit worker-VM/candidate-VM deletion clarification in this 2026-07-18 conversation"
    recorded_at: 2026-07-18T22:17:41Z
    consequence: "G0 must delete its unrelated-worker retention exception and rerun the frozen panel. B deletes worker-VM/candidate-VM lifecycle, controller, tool, API, profile, prompt, and configuration code; no fallback or unrelated VM-worker classification survives."
  evidence_refs: [docs/evidence/self-development-g0-conformance-2026-07-18.md, d5f3b4778439bb71745e951712a229993300d51d, /tmp/choir-selfdev-g1-final-panel/manifest.tsv]
  blocker_or_risk: "The exact Linux boot harness now exists and passes; the remaining blocker is incomplete connection of the four settled shared-authority decisions into product callsites. Until typed prompt refs, any-mode start/decision replay, shared recovery verification, and parent-owned broker FD transfer are exercised end to end, unit and boot evidence can still pass while proposal/capsule execution is impossible."
  next_action: "Implement the five settled connections as one coherent substrate repair, add focused proposal/replay/recovery tests and a real Node A Executor.Spawn broker-FD proof, then freeze a new candidate and rerun the diverse G1 panel."
  dead_end_assessment:
    trigger: "Nine G1 source candidates over two days; every accepted local repair exposed another cross-layer mirror or unexercised Linux transition."
    dependency_graph: "Public CLI → proxy ownership/mode/idempotency → guest API/start-intent/event appender → operation store/run → capsule broker namespaces/socket/capability → verifier/decision event → recovery reconciler/materializer/updater → checkpoint/route. Current docs/skills independently describe portions of that graph."
    substrate_vs_symptoms: "Substrate: no exact Linux lifecycle harness and no single decision/start binding authority. Symptoms: raw artifact ref, mode-ordered retry, mutable-state terminal replay, weak recovery mirror, broker directory ownership, and stale current guidance."
    existing_replacements: "Canonical ComputerEventAppender, selfdev operation/start-intent stores, guest-local capsule Executor, isolated verifier/updater, checkpoint and vmctl route CAS are the intended replacements and are wired partially; none replaces the missing end-to-end verifier of their joins."
    authority_needed: "Resolved in part by owner on 2026-07-19: use Node A as the Linux harness and update it to current code/config first; resolve approval retry, decision binding, broker socket, and artifact representation through agentic consensus. Product semantics requiring owner judgment remain unsettled until the panel synthesis is reviewed."

  structural_recovery_decision:
    status: settled
    source: owner
    settled_by: owner
    recorded_at: 2026-07-19T19:45:00Z
    selected: "Use Node A (`ssh node-a`) as the x86_64-linux harness. It is a lagging Choir mirror; first fast-forward its clean checkout and deploy the current Node A code/config with rollback preserved. Run agentic consensus on approval retry semantics, canonical decision-binding authority, capsule broker socket ownership, and standardized artifact references before further product repair."
    boundary: "Node A is harness infrastructure, not staging/product acceptance. SSH may administer the harness but is not admissible proof for the public no-SSH self-development product path. Effects remain OFF and G1 remains rejected."
  structural_semantics_consensus:
    status: settled
    source: owner_directed_agentic_consensus
    settled_by: owner
    recorded_at: 2026-07-19T20:34:00Z
    panel_health: "Five substantive independent reviews: Codex, Cursor, OpenCode, OMP GPT-5.5, and OMP Gemini 3.5. Devin timed out; OMP GLM 5.2 failed before review. This was a semantics panel, not G1 acceptance."
    receipt: {manifest: /tmp/choir-selfdev-structural-decisions-panel/manifest.tsv, manifest_sha256: 5d2ee5d89435eb1485af943d347de8434d2379d8c8aae08d02ca21fe0418eb74, codex_sha256: b9b04b745d69f1f21516683398421dbcf3db1a9e3890f4355af0d9287d5b4642, cursor_sha256: 50279d47d3e4db9584957e242d7ffa51a684c958e0f888cab08341aead60955a, opencode_sha256: 9774dd14790f4bfb40b24c541900b33e6e28cb8ff88415e49e8e1e408d5f41a6, omp_gpt55_sha256: afb0bc20bf68da3601f2c92c73515fc68f7743a1ace77d9e56384a62273f9e3b, omp_gemini35_sha256: 772f109a4983fc878e07d1644dae54cd5b5885d1dbdce83eb73f306add73fb6d}
    decisions:
      exact_proposal_retry: "This retry is an HTTP response-recovery read, not a new model attempt. After a durable exact `(owner scope, ComputerID, idempotency key, canonical request commitment)` match, return the current operation under any later mode and perform no run, capsule, capability, model, event, or resume effect. A finalized start event may repair only its missing deterministic projection. An intent-only or otherwise incomplete effect still needs current propose_only authority. Same key with changed commitment is 409 before effects."
      exact_decision_retry: "Approval/rejection retry is exact only when the immutable public request, actor/scope, operation, event kind/digest/receipt, bundle, verifier, expected heads/commitments/pending ref, and original mode-consumption binding all join. Return HTTP 200 with the current operation projection, whose existing DecisionEvent and DecisionReceipt name the immutable original decision; V1 adds no second envelope. Legal approval descendants include accepted, materializing, applied, and decision-bound failure/degraded/rollback states. Rejection remains rejected. Never re-consume mode, append a decision, or trigger materialization from the retry."
      decision_binding_authority: "One guest-owned typed command resolver classifies new_authorized, exact_replay, request_conflict, durable_inconsistency, or current_authority_refused. A pure shared verifier supplies canonical value comparisons to the guest command path, startup recovery, projection reconciliation, and materializer; it has no mutation authority. The proxy owns public authentication, owner/computer scope, and routing only. Corpusd remains mechanical signed mode CAS; the guest resolver drives/verifies mode use for new commands. The event chain is truth and operation rows are repairable projections."
      capsule_broker_socket: "Guest-core parent creates the parent-root mode-0700 directory and Unix listener, passes the listening FD into the user-namespace child, and owns cleanup. The broker accepts only that inherited listener and performs authenticated capability-bound RPC readiness. SO_PEERCRED overflow UID is defense in depth, not sole authentication. Three reviewers preferred FD passing; one preferred chown-to-65534 and one child tmpfs. Parent listener won because it keeps the control pathname outside child ownership. Node A must verify actual UID translation, reconnect, cleanup, and FD leakage before G1."
      artifact_references: "Keep three typed meanings: SHA256Digest is exactly 32 bytes rendered as 64 lowercase hex; ArtifactRef renders `artifact:sha256:<digest>` only after bytes are pinned; ArtifactURI renders `artifact+sha256://<digest>/<canonical locator>` only when a real resolver location exists. Heads, event/receipt/bundle digests, and state commitments remain digests. Domain refs such as code, artifact-program, checkpoint, approval, and certificate remain distinct types. Add canonical parsers/constructors; normalize legacy V1 raw event refs only at projection boundaries and never rewrite immutable event bytes or manufacture a ref/URI from an unpinned digest."
    dissent: "The broker implementation had genuine engineering dissent, retained above. Artifact internal Go shape differed between typed strings and byte-array value types; canonical wire/storage forms and no-history-rewrite were unanimous. The chosen byte-array digest/value-wrapper design avoids repeated hex comparison and allocates text only at serialization boundaries."
    owner_readable_summary: "Retries here mean the client repeats an HTTP request because its response was lost. Choir must answer what already happened without treating that retry as fresh permission to code or deploy. The guest computer—not the proxy—decides whether durable records prove the same request. The broker socket stays parent-owned, and artifact names become explicit types instead of interchangeable strings."


  node_a_update_receipt:
    observed_at: 2026-07-19T20:16:00Z
    status: updated_host_guest_image_deploy_blocked
    source_identity: dfb87d1f7d7e4be0a83c6cf32586e4c1af2d5818
    prior_source_identity: fb2b54aa1142bdb1eb84eeaf277063e4e90c4b8c
    active_system: /nix/store/na5g9yjsja4gqnl7q8iqc5w8h1h6vid9-nixos-system-go-choir-a-26.05.20260409.4c1018d
    prior_system_rollback: /nix/store/r82nwfx6yxg0si317call636713pcpix-nixos-system-go-choir-a-26.05.20260409.4c1018d
    evidence_class: "Exact x86_64-linux Nix host build, dry activation, activation, systemd health, loopback API health, and public build-identity readback on Node A."
    initial_problem: "The first build found commonGoArgs.vendorHash stale: Nix expected sha256-JxOGfaZ3J71NVicFEhn1Vsgy5nOa1Sk74gQ0oroAhLA= and computed sha256-NQ3VEnZ8q5Lo1uat8z9lV7YCM4auEkQu6uiI1TcIEvs=."
    resolution: "Commit dfb87d1f refreshed the deterministic Go module hash. The exact Node A realization then built and switched. `systemctl is-system-running` returned running, no failed units were listed, and both loopback and https://choir-ip.com/health reported healthy proxy/vmctl with build commit dfb87d1f7d7e4be0a83c6cf32586e4c1af2d5818. That service-level health is not full product health: post-switch vmctl refused to reattach the retained `vm-universal-wire-platform` because canonical route slot `computer:universal-wire-platform:platform` is absent."
    mutation_class: red
    protected_surfaces: [deployment_configuration, Node_A_harness]
    rollback_path: "Switch to `/nix/store/r82nwfx6yxg0si317call636713pcpix-nixos-system-go-choir-a-26.05.20260409.4c1018d` and reset the clean Node A checkout to fb2b54aa if the harness update must be abandoned."
    heresy_delta: {discovered: "Pinned Go dependency hash did not match the current module graph; service-level health did not surface a missing canonical route for the retained platform computer.", introduced: none, repaired: "The pinned hash now matches the exact current module graph and the host realization builds; route migration remains open."}
    conjecture_delta: "Node A is a current x86_64-linux host, but the first exact harness boot disproved the belief that its installed guest image was current. The host package pointer updated while `/var/lib/go-choir/guest` remained an older unmanaged image. The pre-existing Firecracker process remains running but current vmctl correctly refuses to adopt it without a canonical ComputerVersion route. Do not invent that route or delete the retained computer; use the distinct disposable harness identity."

  node_a_linux_harness_receipt:
    observed_at: 2026-07-19T20:35:26Z
    status: rejected_stale_guest_image
    source_identity: 890bf117
    command: "CHOIR_G1_LINUX_HARNESS=1 CHOIR_G1_EXPECTED_COMMIT=dfb87d1f7d7e4be0a83c6cf32586e4c1af2d5818 go test ./internal/vmmanager -run '^TestSelfDevelopmentEffectsOffGuestHarness$' -count=1 -v"
    evidence: "Node A launched a disposable Firecracker VM from `/var/lib/go-choir/guest`, isolated it on 10.200.1.0/30 beside the retained VM, booted NixOS, served `/health` from the current dynamically injected sandbox binary, then returned 503 `self-development mode authority unavailable` instead of the required effects-OFF 409. Cleanup killed only `vm-selfdev-g1-harness` and removed its temporary state."
    root_cause: "The flake already builds the exact `guest-image` package and passes an unused `guestRunner` specialArg, but `nix/node-b.nix` only creates `/var/lib/go-choir/guest`; no activation path installs or atomically advances the built image. Host code can therefore report the current commit while booting stale guest configuration. This is a substrate/deploy-identity defect, not a mode-handler defect."
    existing_fix_connection: "Connect the existing immutable `guest-image` output to Node A/B activation with an atomic versioned pointer and preserved pre-managed rollback directory. Do not patch the guest handler around missing authority."
    rollback_path: "No harness VM or state remains. The active host system and retained pre-existing Firecracker process were not replaced. Any guest-image pointer cutover must preserve the current unmanaged directory as an explicit rollback ref."
    heresy_delta: {discovered: "Host build identity and guest image/config identity can diverge because the canonical guest package is not deployed.", introduced: none, repaired: none}

  node_a_exact_guest_receipt:
    observed_at: 2026-07-19T22:08:52Z
    status: accepted_g1_linux_harness
    source_identity: 967e3b01600be32a5db3352a3b1546a921619c2a
    guest_image_ref: /tmp/g1-platform-route@fcdef13e088aa7d388a1fd2e08df9e8fb15e58ed
    managed_guest_rollback: /nix/store/mmkgcsg58nfca1hzscd2jw4ss861b4yl-go-choir-guest-image
    pre_managed_guest_rollback: /var/lib/go-choir/guest-pre-managed-rollback
    command: "CHOIR_G1_LINUX_HARNESS=1 CHOIR_G1_RUN_ID=bound967 CHOIR_G1_EXPECTED_COMMIT=fcdef13e088aa7d388a1fd2e08df9e8fb15e58ed CHOIR_G1_{KERNEL,INITRD,ROOTFS,STORE_DISK,KERNEL_PARAMS}=/tmp/g1-platform-route/... go test ./internal/vmmanager -run '^TestSelfDevelopmentEffectsOffGuestHarness$' -count=1 -v"
    evidence: "PASS in 8.54s on Node A. The exact x86_64 Firecracker guest completed user/PID/mount/network/UTS/IPC, cgroup v2, loaded+mounted overlayfs, enforced seccomp, and enforced Landlock probes; exchanged and consumed the one-time canonical credential directly with corpusd; reconstructed computer event authority; configured guest-local capsule authority; served health with exact guest build fcdef13e; and refused a new effects-OFF proposal with 409. The unrouted disposable harness correctly refused a public KernelCapabilityReceipt with 503 `computer route identity unavailable`; C must obtain 200 only after an authorized serving route binds the exact deployed ComputerVersion. The disposable VM/tap/state were removed."
    problem: none
    next_probe: "Freeze the exact source candidate and rerun full deterministic/Nix verification plus a diverse G1 panel. If accepted, C deploys effects OFF and requires the route-bound public signed KernelCapabilityReceipt before D."
    heresy_delta: {discovered: none, introduced: none, repaired: "Managed exact guest deployment, all mandatory kernel/isolation probes, canonical credential delivery, direct guest event-authority routing, and explicit unrouted receipt refusal are repaired and runtime-proven."}

successor:
  status: selected_draft_non_executable
  candidate_goal: docs/definitions/choir-in-choir-computer-control-draft-2026-07-18.md
  note: "Choir-in-Choir remains blocked until terminal closure and separate grant/addressing owner ratification and registry promotion."

review_receipts:
  - id: architecture-consensus-a0-only-2026-07-18
    scope: superseded_narrow_review
    outcome: "Confirmed only code-free A0; superseded by the owner's whole-mission confirmation requirement."
  - id: whole-mission-consensus-round-1-2026-07-18
    panel: [codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Claude, Codex, Cursor, OpenCode, and omp-gpt55 completed; Devin timed out; Gemini and GLM provider calls failed before review."
    outcome: repair
    adjudication: "Claude found no blockers, but four successful reviewers found reproducible whole-mission blockers. Minority rule applied. The execution contract, host-free capability TCB, complete event/privacy/migration protocol, updater, public CLI/API/auth, all-role cutover, phase/deploy order, activation modes, and gate rollback dispositions above repair them."
  - id: whole-mission-consensus-round-2-2026-07-18
    panel: [codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Claude, Codex, Cursor, OpenCode, and omp-gpt55 commands completed; omp-gpt55 produced no substantive report; Devin timed out; Gemini and GLM provider calls failed before review."
    outcome: repair
    adjudication: "Claude and OpenCode found no blocker. Cursor and Codex produced reproducible minority blockers; Codex incorrectly reported current-packet Claude absence even though the runner captured a current frozen-packet Claude review. Minority rule still applied. Repairs make event pin receipts external, place the event head on explicit corpusd control tables under the two-Dolt contract, define typed authenticated transport and route certificates, partition stale rebase, add positive G0 kernel preflight, settle key custody/AEAD, remove secret argv, expose mode/lifecycle contracts, unify idempotency, and make G3 non-mutating."
  - id: whole-mission-consensus-round-3-2026-07-18
    panel: [codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Claude, Codex, Cursor, OpenCode, and omp-gpt55 produced substantive current-packet reviews; Devin timed out; Gemini and GLM provider calls failed before review."
    outcome: repair
    adjudication: "Claude, OpenCode, and omp-gpt55 found no blocker. Cursor and Codex demonstrated minority blockers; Codex's claim that current-packet Claude was absent was false because the runner captured `/tmp/choir-selfdev-full-mission-round3/claude.out`, but its semantic findings were independently adjudicated. Repairs define non-self-referential desired/effective heads, total genesis CAS, receipt schemas and Ed25519 trust/rotation, constructor credential delivery and blocked disposition, sole post-genesis route certificate with exact RouteTransitionCommand, mode operation matrix and safety rollback, and canonical decision bindings."
  - id: whole-mission-consensus-round-4-2026-07-18
    panel: [codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Claude, Codex, Cursor, OpenCode, and omp-gpt55 produced substantive current-packet reviews; Devin timed out; Gemini and GLM provider calls failed before review."
    outcome: repair
    adjudication: "Claude, OpenCode, and omp-gpt55 found no blocker. Cursor and Codex demonstrated minority blockers. Repairs define non-circular existing AuthorizationEvidence construction plus exact route command, a total off-mode genesis operation, named receipt/key rotation and revocation contracts, and desired/effective-head decision binding."
  - id: whole-mission-consensus-round-5-2026-07-18
    panel: [codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Claude, Codex, Cursor, OpenCode, and omp-gpt55 produced substantive current-packet reviews; Devin timed out; Gemini and GLM provider calls failed before review."
    outcome: repair
    adjudication: "Claude, Cursor, OpenCode, and omp-gpt55 found no blocker. Codex demonstrated two minority blockers. Repairs define the exact non-self-referential receipt digest/signature preimage, initialize all genesis projections, and add the one-way R0-to-R1 security ratchet so post-genesis rollback cannot restore superseded route authority."
  - id: whole-mission-consensus-round-6-2026-07-18
    reviewed_at: 2026-07-18T20:52:22Z
    reviewed_definition_sha256: 4b5fd0b6531abd4d15578171372d96452f350157dfd698cb1767a780c27ba512
    manifest: /tmp/choir-selfdev-full-mission-round6/manifest.tsv
    panel: [codex, devin, claude, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Claude, Codex, Cursor, OpenCode, and omp-gpt55 produced substantive current-packet reviews; Devin timed out; Gemini and GLM provider calls failed before review."
    verdicts: [claude:CONFIRMED, opencode:CONFIRMED, cursor:CONFIRMED_WITH_NONBLOCKING_NOTES, omp-gpt55:CONFIRMED_WITH_NONBLOCKING_NOTES, codex:CONFIRMED_WITH_NONBLOCKING_NOTES]
    outcome: accept
    adjudication: "All five successful independent reviewers found no whole-mission blocker. Claude's current-packet opinion is CONFIRMED. The panel independently verified the receipt preimage, R0-to-R1 ratchet, genesis projections, route evidence, state transitions, authority topology, phase gates, and blocked implementation prerequisites. No minority repair remains."
    receipt_binding: "This post-review mutation only updates compact `now` and review-receipt bookkeeping; it changes no architecture, authority, phase, gate, acceptance criterion, rollback, or next-action semantics. Any later semantic change requires a new frozen panel."
  - id: preflight-consensus-round-7-2026-07-18
    reviewed_at: 2026-07-18T21:46:50Z
    reviewed_definition_sha256: 9d0f8e591064e6275374900fd4b5f0d4d177d33ebdad864ad5d9512e737cbf46
    manifest: /tmp/choir-selfdev-preflight-panel/manifest.tsv
    panel: [codex, claude, omp-gpt55]
    health: "All three configured reviewers completed substantive current-packet reviews using OpenAI GPT-5.6, Claude, and OMP GPT-5.5."
    verdicts: [codex:CONFIRMED_WITH_NONBLOCKING_NOTES, claude:CONFIRMED_WITH_NONBLOCKING_NOTES, omp-gpt55:CONFIRMED_WITH_NONBLOCKING_NOTES]
    outcome: accept
    adjudication: "No reviewer found a whole-mission blocker. All independently confirmed the existing constructor populate/mkfs/attachment seam, positive pinned kernel capabilities, and the non-circular fail-closed G0→B/G1→C-before-D attestation order. A/G0 must freeze KernelCapabilityReceipt kind fields, signer/trust root, measurement derivation, freshness, and public verification; B/G1 must test loaded overlay state, uid/gid/mode, image-at-rest leakage, receipt tamper/staleness, and exact realization binding. These are already required conformance/implementation details, not new semantics or deferred blockers."
    receipt_binding: "The post-review mutation corrects the preflight timestamp and immutable-device wording, updates the candidate freeze identity, and appends this receipt. It changes no reviewed architecture, authority, phase, gate, acceptance criterion, rollback, or next-action semantics. Any later semantic change requires a new frozen panel."

  - id: G0-terminal-contract-conformance-2026-07-18
    reviewed_at: 2026-07-18T22:44:04Z
    freeze_identity: sha256:31eee3f95322f7c6698ca69b581e8e2bc8f4415fccee34dd00372083e780d4cd
    manifest: /tmp/choir-selfdev-g0-terminal-panel/manifest.tsv
    panel: [cursor, opencode, omp-gpt55]
    health: "All three configured reviewers completed. OpenCode and omp-gpt55 confirmed; Cursor's sole finding was a claimed freeze mismatch."
    verdicts: [opencode:CONFIRMED, omp-gpt55:CONFIRMED, cursor:REPAIR]
    outcome: accept
    adjudication: "All semantic blockers from prior G0 rounds are repaired. The Cursor digest finding is rejected by exact deterministic recomputation: after replacing only the freeze_identity scalar with pending_content_digest, the ordered per-file rows are 67a7de1f... definition, 31e43907... doctrine, f54bab30... ontology, 9c058788... agent doctrine, 7e9deb7e... architecture, 407f1777... runtime invariants, f8264c4d... platform state, and baebf4a2... packet; their framed-row SHA-256 is exactly 31eee3f95322f7c6698ca69b581e8e2bc8f4415fccee34dd00372083e780d4cd. OpenCode's own report omitted substitution when displaying 6cfb2bd... but found no semantic blocker; omp-gpt55 independently reproduced the accepted digest. The claim that start.status must become dirty is also rejected: start is the immutable authoring receipt and now.reconciliation is the sole current inventory."
    receipt_binding: "This post-review mutation advances now from A/G0 to B and appends the review receipt only. It changes no reviewed architecture, authority, schema, gate, acceptance, rollback, or deletion semantics. Any later semantic contract change requires a new frozen review."

  - id: G1-disabled-cutover-round-2-2026-07-19
    reviewed_at: 2026-07-19T10:38:00Z
    requested_candidate_ref: 8bad0a25bf05c6ed513ecf4ddfef8f8da0b548de
    actual_reviewed_candidate_ref: 8bad0a25aa4dc4d4e5fc4ce1a60314a0721f1135
    manifest: /tmp/choir-selfdev-g1-final-panel/manifest.tsv
    panel: [codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Codex, Cursor, OpenCode, omp-gpt55, and omp-gemini35 completed; Devin timed out; omp-glm52 failed before review. The prompt mistyped the full SHA, but Codex, Cursor, and omp-gpt55 explicitly resolved and inspected the actual local same-prefix candidate."
    verdicts: [codex:REJECT_G1, cursor:REJECT_G1, omp-gpt55:REJECT_G1, opencode:ACCEPT_G1, omp-gemini35:ACCEPT_G1]
    outcome: repair
    adjudication: "Minority rule is not needed: three independent reviewers supplied reproducible source blockers. Exact candidate identity is corrected in now; all substantive confirmed blockers are documented there before repair. Claims that conflict with the settled topology are adjudicated during implementation: old app-adoption/promotion and public agent-loop paths are not separately retained product features under the owner clean-cutover decision; accept_once returns to propose_only; capsule admission and production vmctl must fail closed."

  - id: G1-disabled-cutover-round-3-2026-07-19
    reviewed_at: 2026-07-19T14:25:05Z
    candidate_ref: f9cc324633fc64a40c407aa8abd328f9b257127a
    authority_ref: 236dec0317e8ca2a2b071cbcd2ca2fcac580f8a2
    manifest: /tmp/choir-selfdev-g1-f9cc-panel/manifest.tsv
    panel: [codex, cursor, opencode, omp-gpt55, omp-gemini35]
    health: "All five configured reviewers exited successfully. Codex, Cursor, omp-gpt55, and omp-gemini35 returned substantive candidate reviews; OpenCode did not reach its required verdict after its read-only worktree request was denied and is excluded from adjudication."
    verdicts: [codex:REJECT_G1, cursor:REJECT_G1, omp-gpt55:REJECT_G1, omp-gemini35:ACCEPT_G1, opencode:INCOMPLETE_NO_VERDICT]
    outcome: repair
    adjudication: "Three independent reviewers reproduced source blockers, so G1 rejects without minority-rule ambiguity. Codex and omp-gpt55 independently found broken accept_once consumption, missing cross-owner cookie authorization, and surviving AppChangePackage/AppAdoption product surfaces. Codex additionally found embedded security projection loss; omp-gpt55 found missing exact public routes and corpusd private-key custody. Cursor independently showed the updater-readable restart handoff re-exposes the appender bearer and privacy key. Gemini accepted but did not rebut these concrete paths. This receipt documents the problems before any repair-code commit."

  - id: G1-disabled-cutover-round-4-2026-07-19
    reviewed_at: 2026-07-19T15:07:03Z
    requested_candidate_ref: 5ae5b610c4901f6958a5ae5747cba61f283fa548
    actual_candidate_ref: 5ae5b6106bf60610b2404e4b1b1f5f26865c337e
    authority_ref: a7e744008eef74e3427ae4a2d38a9e1326c1d7fd
    manifest: /tmp/choir-selfdev-g1-round4-panel/manifest.tsv
    panel: [codex, cursor, opencode, omp-gpt55, omp-gemini35]
    health: "All five configured reviewers exited successfully. Codex, Cursor, omp-gpt55, and omp-gemini35 returned required verdicts; OpenCode did not reach a verdict and is excluded."
    verdicts: [codex:REJECT_G1, cursor:ACCEPT_G1, omp-gpt55:REJECT_G1, omp-gemini35:ACCEPT_G1, opencode:INCOMPLETE_NO_VERDICT]
    outcome: repair
    adjudication: "Exact identity alone rejects G1: Codex and omp-gpt55 confirmed the named SHA does not exist; Cursor and Gemini also observed the mismatch but incorrectly treated it as non-blocking despite the immutable freeze contract. Codex additionally supplied source-reproducible signer-custody, altered accept_once replay, and obsolete-current-citer blockers. Local inspection confirms each against the frozen G0 contract and candidate. This receipt documents them before any repair-code commit."

  - id: G1-disabled-cutover-round-5-2026-07-19
    reviewed_at: 2026-07-19T16:11:05Z
    candidate_ref: 32b315971dc4939ccf8499d7740336300d5da81a
    authority_ref: f665099166d6ddafcb5722eca1b4448131de0f93
    manifest: /tmp/choir-selfdev-g1-round5-panel/manifest.tsv
    panel: [codex, cursor, opencode, omp-gpt55, omp-gemini35]
    health: "Codex, Cursor, OpenCode, and omp-gpt55 completed substantive reviews; omp-gemini35 failed before review."
    verdicts: [codex:REJECT_G1, cursor:REJECT_G1, opencode:REJECT_G1, omp-gpt55:REJECT_G1, omp-gemini35:FAILED]
    outcome: repair
    adjudication: "All four successful reviewers rejected. Confirmed blockers cluster into one incomplete authority cutover: mutually callable signer sockets; credential revocation attempted before genesis and an append-before-durable-capability crash window; proxy-only accept_once rejection refusal; and current Compute Monitor/platform-state/doctrine guidance for deleted candidate/package/adoption paths. This receipt documents every confirmed class before repair-code mutation. Deployed-only gates remain deferred correctly and are not counted as blockers."

  - id: G1-disabled-cutover-round-6-2026-07-19
    reviewed_at: 2026-07-19T17:00:13Z
    candidate_ref: fb0e56e33de17fbf7cf7326b345fa701d6a241a3
    authority_ref: 5602600891f52c7f5fe13c520c75788f41d6147d
    manifest: /tmp/choir-selfdev-g1-round6-panel/manifest.tsv
    panel: [codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Codex, Cursor, OpenCode, omp-gpt55, and omp-gemini35 completed substantive reviews; Devin timed out; omp-glm52 failed before review."
    verdicts: [codex:REJECT_G1, cursor:ACCEPT_G1, opencode:ACCEPT_G1, omp-gpt55:REJECT_G1, omp-gemini35:ACCEPT_G1, devin:TIMED_OUT, omp-glm52:FAILED]
    outcome: repair
    adjudication: "Minority rule rejects G1. Local source inspection confirms Codex's two blockers—non-atomic credential handoff acquisition/restore and canonical decision replay after current-mode validation—and omp-gpt55's independent-authority blocker: updater owns both signer clients and exposes verifier signing to guest runtime callers through updater.sock. The three accepting reviews did not rebut these reproducible paths. This receipt documents every confirmed problem before repair-code mutation; deployed-only gates remain excluded."

  - id: G1-disabled-cutover-round-7-2026-07-19
    reviewed_at: 2026-07-19T18:04:52Z
    candidate_ref: 153c68668a8b16f47ff5fba17a983d2d37339cbb
    authority_ref: 7990a2022e28bf59211e54f72d4b3e0d684d3254
    manifest: /tmp/choir-selfdev-g1-round7-panel/manifest.tsv
    panel: [codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Codex, Devin, Cursor, OpenCode, omp-gpt55, and omp-gemini35 completed; omp-glm52 failed before review."
    verdicts: [codex:REJECT_G1, cursor:REJECT_G1, omp-gpt55:REJECT_G1, opencode:ACCEPT_G1, omp-gemini35:ACCEPT_G1, devin:ACCEPT_G1, omp-glm52:FAILED]
    outcome: repair
    adjudication: "The split panel rejects under the severe-minority rule. Local inspection confirms Codex's public-proxy terminal-replay blocker, Cursor's proxy-only start-mode gate, and omp-gpt55's stale top-level candidate-computer/promotion/lineage guidance. The three accepting reviews did not rebut these reproducible paths. This receipt records all confirmed problems before repair-code mutation; prior round-6 repairs remain sound and deployed-only C–F gates remain excluded."

  - id: G1-disabled-cutover-round-8-2026-07-19
    reviewed_at: 2026-07-19T18:42:42Z
    candidate_ref: 18e4f9dbfb37eb7d518103a8315542bc11f02f92
    authority_ref: 12e09034a6b961dc263cfa9c02f82d9d95c66024
    manifest: /tmp/choir-selfdev-g1-round8-panel/manifest.tsv
    panel: [codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Codex, Devin, Cursor, OpenCode, omp-gpt55, and omp-gemini35 completed; omp-glm52 failed before review."
    verdicts: [codex:REJECT_G1, omp-gpt55:REJECT_G1, devin:REJECT_G1, cursor:ACCEPT_G1, opencode:ACCEPT_G1, omp-gemini35:ACCEPT_G1, omp-glm52:FAILED]
    outcome: repair
    adjudication: "The split panel rejects under the severe-minority rule. Codex identified incomplete terminal semantic joins and stale current runtime guidance; omp-gpt55 reproduced changed proposal retry after an event-only crash; Devin identified the user-namespace-inverted broker peer check and dial-only readiness. Local inspection confirms each class. The accepting reviews did not rebut them. This receipt records all confirmed problems before repair-code mutation; prior repairs remain sound and deployed-only C–F gates remain excluded."

  - id: G1-disabled-cutover-round-9-2026-07-19
    reviewed_at: 2026-07-19T19:33:16Z
    candidate_ref: ae881720132809d6d6092b4a739e43a311489000
    authority_ref: d12e1d9d337fffabee5dd9c0385159213b463b24
    manifest: /tmp/choir-selfdev-g1-round9-panel/manifest.tsv
    panel: [codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Codex, Devin, Cursor, OpenCode, omp-gpt55, and omp-gemini35 completed; omp-glm52 failed before review."
    verdicts: [codex:REJECT_G1, devin:REJECT_G1, cursor:REJECT_G1, omp-gpt55:REJECT_G1, opencode:ACCEPT_G1, omp-gemini35:ACCEPT_G1, omp-glm52:FAILED]
    outcome: blocked_incomplete_structural_escalation
    adjudication: "Four independent reviewers rejected. Local inspection confirms Cursor's non-executable raw prompt artifact join, omp-gpt55's public start replay ordering, Devin's weak startup decision recovery, and Codex's post-materialization terminal replay, broker-directory ownership, and stale-current-authority findings. These cluster at missing executable Linux lifecycle and duplicated binding-authority substrates. AGENTS.md Dead-End Escalation now prohibits another incremental patch without explicit owner direction."

  - id: G1-disabled-cutover-round-10-2026-07-19
    reviewed_at: 2026-07-19T22:26:16Z
    candidate_ref: d5f3b4778439bb71745e951712a229993300d51d
    authority_ref: d5f3b4778439bb71745e951712a229993300d51d
    manifest: /tmp/choir-selfdev-g1-final-panel/manifest.tsv
    panel: [codex, devin, cursor, opencode, omp-gpt55, omp-gemini35, omp-glm52]
    health: "Codex, Devin, Cursor, OpenCode, omp-gpt55, and omp-gemini35 completed; omp-glm52 failed before review."
    verdicts: [codex:REJECT_G1, devin:REJECT_G1, cursor:REJECT_G1, omp-gpt55:REJECT_G1, opencode:ACCEPT_G1, omp-gemini35:ACCEPT_G1, omp-glm52:FAILED]
    outcome: repair
    adjudication: "Four independent reviewers rejected. Local inspection confirms raw prompt artifact refs, proxy-first mode gating on exact start retry, decision replay limited to accepted/rejected states, weak duplicated startup recovery validation, and child-created broker socket custody. The two accepting reviews did not rebut these source paths. The owner has already settled the corresponding structural semantics, so this receipt documents the still-unwired connections before one coherent repair; effects remain OFF and C is blocked."

view:
  path: none
  endpoint: "http://127.0.0.1:8788"
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-cli-self-development-2026-07-16.md --serve 127.0.0.1:8788 --watch"
  generator_version: "definition-dashboard-js/v1"
  authority: "The dashboard is a read-only projection. This Markdown/YAML Definition and coherent registries are mission authority."
---

# Make Choir Self-Developing — Capsule-Scoped Audited Work

Completion is the entire deployed loop, not permission to start A: one stable staging Choir computer records complete causal history, develops a verified change in a real guest-local capsule, receives external scoped approval through the public CLI, safely materializes and checkpoints it, survives restart/reconstruction, rejects another change, rolls back through retained events, and closes only after G3. Direct role mutation, worker/candidate VMs, host authority, internal APIs, SSH, raw vmctl, mutable branches, and local-only proof do not count.
