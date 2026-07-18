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
      credential_delivery: "At each realization build, the existing constructor writes a single-use ComputerCredentialEnvelope into the root-owned guest credential partition as realization-local input, never into CodeRef, ArtifactProgramRef, data artifacts, logs, kernel argv, or model context. The envelope contains a random ComputerID-scoped corpusd bearer, expiry, and revocation epoch; it is readable only by the root core credential service, exchanged over TLS for a shorter-lived append/pin capability, then erased. Live rotation uses the authenticated corpusd channel and atomic 0400 credential-file replacement; reconstruction provisions a new envelope and revokes the old realization. This is ordinary vmctl/constructor materialization, not SSH, MMDS/vsock authority, a host daemon, or guest-to-host control. A must locate and freeze the concrete constructor insertion seam; if the existing constructor cannot deliver this envelope without a new host service or agent visibility, G0 sets blocked_incomplete."
      platform_transport: "The guest appender never receives SQL, route, signing, or host credentials. Through the existing authenticated proxy/corpusd service, it pins blobs and calls a typed event CAS endpoint using the short-lived capability. corpusd is the mechanical transaction writer and returns an EventHeadReceipt; the guest ComputerEventAppender is the sole authorized semantic caller. The capability is unavailable to models, capsules, updater, and ordinary tools; expiry/revocation or service unavailability fails closed."
      minimum_hashed_envelope: [schema_version, event_id, computer_id, sequence, previous_head, event_kind, occurred_at, idempotency_key, request_commitment, trajectory_id, parent_event_id, capsule_id, actor_profile, authority_ref, model_policy_refs, input_artifact_refs, output_artifact_refs, payload_commitment, privacy_class, proposed_effect_ref, decision_ref, verifier_refs, reducer_version, expected_desired_event_head, expected_effective_event_head, expected_pending_transition_ref, expected_desired_state_commitment, expected_effective_state_commitment, resulting_effective_state_commitment]
      derived_heads: "canonical_event_head is digest(E) for every appended event. desired/effective event heads, desired/effective state commitments, and nullable pending_transition_ref are deterministic reducer projections; no event contains its own digest. `resulting_effective_state_commitment` is SHA-256 over canonical reducer_version plus ordered effective CodeRef/ArtifactProgramRef/embedded-Dolt state refs and advances only on an atomically effective ResearcherUpdate, MaterializationApplied, or RollbackApplied."
      receipt_signing_contract:
        encoding: "Generate opaque UUIDv7 receipt_id and the ordered `required_signers` entries {signer_domain,key_id} first. `canonical_payload_sha256` is lowercase hex SHA-256 of RFC 8785 JSON for the complete receipt with only `canonical_payload_sha256` and `signature_set` omitted; receipt_id, receipt_kind, issuer, issued_at, required_signers, and all kind fields are therefore bound without self-reference. Each required signer Ed25519-signs the exact byte string UTF8(`choir-receipt-v1`) || 0x00 || raw_32_byte_canonical_payload_sha256. `signature_set` is an ordered one-for-one copy of required_signers with signature added; missing, duplicate, extra, relabeled, or invalid entries refuse. Required common fields are receipt_version, receipt_kind, receipt_id, issuer, issued_at, required_signers, canonical_payload_sha256, and signature_set."
        trust_roots: "corpusd owns the platform-control Ed25519 key in its deployment credential store and signs PinReceipt, EventHeadReceipt, ModeReceipt, LifecycleReceipt, CheckpointReceipt, and RouteProjectionCertificate. The already configured owner-promotion public key is pinned as the offline owner-recovery trust anchor for control-key rotation/revocation; its corresponding private key remains outside Choir services and ordinary deploy credentials and is not post-genesis route approval. An independent verifier key signs VerifierCertificate. choir-updater invokes the root guest credential service to sign HealthReceipt, MaterializationReceipt, UpdaterRecoveryReceipt, and updater-key changes without receiving private bytes. Platform, owner-recovery, verifier, and updater public-key histories are pinned in corpusd and constructed guest releases; updater key/ComputerID/RealizationID starts in GenesisImported. No private signing key enters agents, capsules, updater payloads, immutable source artifacts, or argv."
        rotation_and_time: "Planned platform rotation receipt has old-platform, new-platform, and owner-recovery signatures. The guest verifies it, appends key_rotated while old remains valid, receives the old-key EventHeadReceipt, then corpusd activates new. Emergency replacement has new-platform plus owner-recovery signatures and an explicit compromised-key cutoff; the pinned owner-recovery anchor authorizes the special recovery CAS, its EventHeadReceipt carries new-platform plus owner-recovery signatures, and new activates only after that receipt. Updater rotation follows the same old/new/owner-authorized-event order; emergency updater replacement requires owner-recovery signature. Revocation is owner-recovery-signed and, for an active signing key, names a simultaneously authorized replacement. Every receipt is pinned and inserted into corpusd `control_key_history` before its key event; activation follows the event. Old public keys remain indefinitely. Revocation names key_id and first invalid sequence/time; pre-cutoff receipts remain historical evidence, at/after-cutoff receipts refuse. Authorization expiry uses corpusd time and frozen maximum skew; event/pin/history receipts do not expire."
        verifier_matrix: "Guest appender verifies corpusd pin/head/mode/lifecycle/key receipts; corpusd verifies updater/verifier signatures and append capability; choir-updater verifies EventHeadReceipt plus accepted decision/materialization command; vmctl verifies corpusd EventHeadReceipt, both AuthorizationEvidence objects, RouteProjectionCertificate, CheckpointReceipt, MaterializationReceipt, VerifierCertificate, and exact route command; clean external clients verify all public receipts against pinned key history. Unknown key, bad signature, wrong issuer/kind/ComputerID, expired authorization, discontinuous rotation, or missing per-computer key event fails closed."
        kind_fields:
          PinReceipt: [computer_id, artifact_digest, media_type, length, privacy_class, pin_namespace, request_commitment]
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
      - "Redact/classify payloads, obtain PinReceipts for payload artifacts, canonicalize event body E(previous=H), compute digest(E), pin E, and obtain its external PinReceipt. Orphan pins are non-authoritative and garbage-collectable after the retention floor."
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
    id: self-development-G0-contract-conformance
    state: accepted_G0_contract
    ref: docs/evidence/self-development-g0-conformance-2026-07-18.md
    owner: integration-authority
    base: 5483a082d0012890343deb3693eea15c53a98415
    freeze_identity: "sha256:31eee3f95322f7c6698ca69b581e8e2bc8f4415fccee34dd00372083e780d4cd"
    freeze_algorithm: "Replace the complete freeze_identity scalar with pending_content_digest, then compute lowercase SHA-256 for each repo-relative scope path in listed order; encode each row as <hex><two ASCII spaces><path><LF>; freeze identity is lowercase SHA-256 of the concatenated UTF-8 rows. Equivalent command after the substitution: shasum -a 256 <ordered paths> | shasum -a 256."
    scope: [docs/definitions/choir-cli-self-development-2026-07-16.md, docs/choir-doctrine.md, docs/computer-ontology.md, docs/agent-product-doctrine.md, docs/current-architecture.md, docs/runtime-invariants.md, docs/platform-os-app-state.md, docs/evidence/self-development-g0-conformance-2026-07-18.md]
  decision:
    selected: "Execute the entire A→F mission under the fixed execution contract above. Candidate VMs and worker VMs are obsolete and their code is deleted; generic delegated agents use durable runs/trajectories and capsules. A/G0 reconciles rather than invents semantics; implementation lands with only self-development activation off; deployed G2 precedes the one bounded acceptance; G3 precedes closure."
    kind: architecture_and_execution_authority
    status: settled
    source: owner
    settled_by: owner
    evidence_ref: "Owner whole-mission instruction and explicit worker-VM/candidate-VM deletion clarification in this 2026-07-18 conversation"
    recorded_at: 2026-07-18T22:17:41Z
    consequence: "G0 must delete its unrelated-worker retention exception and rerun the frozen panel. B deletes worker-VM/candidate-VM lifecycle, controller, tool, API, profile, prompt, and configuration code; no fallback or unrelated VM-worker classification survives."
  evidence_refs: [docs/evidence/self-development-g0-conformance-2026-07-18.md, docs/definitions/choir-in-choir-computer-control-draft-2026-07-18.md, internal/capsule/executor.go, internal/capsule/capsule.go, internal/capsule/transaction/tape.go, internal/agentcore/tools_capsule.go, internal/agentcore/tool_profiles.go, internal/agentprofile/agentprofile.go, internal/trace/store.go, internal/types/task.go, cmd/choir/main.go]
  blocker_or_risk: "G0 is accepted. The terminal panel produced two confirmations and one claimed digest mismatch; exact recomputation from the documented substituted per-file rows yields 31eee3f95322f7c6698ca69b581e8e2bc8f4415fccee34dd00372083e780d4cd, so the mismatch is rejected as non-reproducible. The immutable start receipt remains historical; current candidate dirt is reconciled in now. B remains unbuilt, and no behavior/deploy/accepted effect is authorized until its own G1/C/D gates."
  next_action: "Land the code-free G0 boundary, then implement B as one activation-off clean cutover: canonical event/recovery substrate, guest-local capsule isolation, role and worker/candidate deletion, public updater/API/CLI/modes, deterministic tests, and a frozen G1 candidate."

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

view:
  path: none
  endpoint: "http://127.0.0.1:8788"
  generator: "node skills/definition/scripts/dashboard.mjs docs/definitions/choir-cli-self-development-2026-07-16.md --serve 127.0.0.1:8788 --watch"
  generator_version: "definition-dashboard-js/v1"
  authority: "The dashboard is a read-only projection. This Markdown/YAML Definition and coherent registries are mission authority."
---

# Make Choir Self-Developing — Capsule-Scoped Audited Work

Completion is the entire deployed loop, not permission to start A: one stable staging Choir computer records complete causal history, develops a verified change in a real guest-local capsule, receives external scoped approval through the public CLI, safely materializes and checkpoints it, survives restart/reconstruction, rejects another change, rolls back through retained events, and closes only after G3. Direct role mutation, worker/candidate VMs, host authority, internal APIs, SSH, raw vmctl, mutable branches, and local-only proof do not count.
