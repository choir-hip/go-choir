# Self-Development G0 Contract-Conformance Packet

Date: 2026-07-18
Frozen source base: `5483a082d0012890343deb3693eea15c53a98415`
Authority: `docs/definitions/choir-cli-self-development-2026-07-16.md`
Mutation class: red, code-free contract boundary

This packet translates the owner-settled Definition into concrete V1 names and deletion dispositions. It authorizes no behavior change. The Definition remains authoritative if this projection is incomplete; a semantic mismatch blocks G0 rather than being resolved locally.

## G0 conclusion

The contract is implementable without a third database, host capsule daemon, candidate VM, SSH, raw vmctl, mutable branch, or new promotion authority. The existing constructor population callback, corpusd world-wire SQL server, artifact service, embedded guest Dolt, ComputerVersion constructor/verifier, route ledger CAS, runtime trajectory/event projections, capsule types, and CLI client are the reusable seams. Current direct role tools, VSuper aliases, worker/package self-development routes, host capsule authority, incomplete capsule executor, and internal run routes are replacement targets, not fallbacks.

## Immutable guest-kernel receipt

Pinned inputs:

- `flake.lock` nixpkgs revision `4c1018dae018162ec878d42fec712642d214fdfa`.
- Evaluated guest kernel `6.18.21`.
- Realized config `/nix/store/252bxb6q8p4fpza6bj0v4ndr98vxrnhk-linux-config-6.18.21`, SHA-256 `5abba8875e79ba9c8bcd7d9604d137af310641dc44caf536424dc2cdd4c032eb`.
- Required built-ins are `CONFIG_USER_NS`, `CONFIG_PID_NS`, `CONFIG_NET_NS`, `CONFIG_UTS_NS`, `CONFIG_IPC_NS`, `CONFIG_CGROUPS`, `CONFIG_MEMCG`, `CONFIG_CGROUP_PIDS`, `CONFIG_CGROUP_BPF`, `CONFIG_SECCOMP`, `CONFIG_SECCOMP_FILTER`, and `CONFIG_SECURITY_LANDLOCK`, all `y`; `CONFIG_MEMCG_V1` is unset.
- `CONFIG_OVERLAY_FS=m`; realized `overlay.ko.xz` SHA-256 is `a2004b3492257fc1d471fd607aed53537c1dc181b5d8d41024c6b697c2c3fcab`.
- Evaluated boot parameters contain `lsm=landlock,yama,bpf` and no cgroup-v1 override. NixOS/systemd 256 uses unified cgroup v2.

B implements `KernelCapabilityReceipt` as an RFC-8785 canonical signed receipt. Required kind fields are:

`computer_id`, `realization_id`, `computer_version`, `release_digest`, `guest_image_digest`, `kernel_config_digest`, `kernel_release`, `boot_id`, `boot_parameters`, `cgroup_filesystem_type`, `overlay_module_digest`, `observed_capabilities`, `probe_contract_digest`, `observed_at`, `expires_at`, and `lifecycle_generation`.

`observed_capabilities` is an ordered map for `user_namespace`, `pid_namespace`, `mount_namespace`, `network_namespace`, `uts_namespace`, `ipc_namespace`, `cgroup_v2`, `overlayfs_loaded_and_mountable`, `seccomp_filter_enforced`, and `landlock_enforcing`. Each entry contains `supported`, `enforced`, and a content-addressed observation ref. The root guest credential service signs the receipt with the guest-core/updater signing domain; private key bytes never enter the runtime, updater, capsule, model context, logs, or API. Before genesis, the verification key is pinned by the immutable construction manifest and R1 deployment inputs; after genesis it is also bound by `GenesisImported` and subsequent key events. corpusd verifies and stores the public receipt projection; clean clients verify it against that pinned key history.

The root boot probe derives the receipt from the installed immutable construction manifest plus direct runtime probes: namespace creation, unified cgroup-v2 controller creation, overlay module load and scratch mount, seccomp denial, Landlock ABI/ruleset denial, current boot parameters, kernel release, and boot ID. A static config claim cannot substitute for a runtime probe. The public API returns only a receipt whose `computer_id`, `realization_id`, `computer_version`, image/config/release digests, boot ID, and lifecycle generation equal the currently served realization. The receipt expires 15 minutes after observation; a fresh root probe may renew it without changing semantic state. Unknown keys, stale receipts, digest mismatch, false/missing capability, probe error, or cleanup failure refuses capsule admission and blocks D. C must retrieve this receipt through the public API without SSH and compare it with the immutable-image receipt above.

## Single writer and projection inventory

| Surface | Current writer or caller | V1 disposition |
| --- | --- | --- |
| `types.EventRecord`, runtime `emitEvent`/`EmitProductEvent` | runtime and appagent code append per-run/product events to the runtime store; live bus republishes | Observation projection only. Every post-genesis model/message/tool/artifact/refusal fact first passes through `ComputerEventAppender`; EventRecord may project the accepted canonical envelope/receipt and never allocate sequence or settle state. |
| Trace store | runtime writes a Dolt-backed observation stream | Retain as a read projection keyed to canonical event digest; no append authority. |
| `TrajectoryRecord`, work items, actor update log | runtime/store and actor runtime | Retain causality and delivery projections. Post-genesis trajectory/model/message/tool boundaries append canonical events; actor logs do not settle computer state. |
| run memory | runtime store appends plaintext messages and compaction snapshots | Replace post-genesis storage path with privacy-classified artifact commitments and encrypted owner-private payload refs. Compaction cannot erase the canonical envelope or content commitment. |
| capsule `transaction.TransactionTape` | process-local in-memory append | Delete as authority. Frozen bundle event refs and embedded prepared rows replace it; no acknowledged effect may live only in memory. |
| embedded guest Dolt | app/runtime domain writers | Remains materialized semantic state. `ComputerEventAppender` owns event-index rows and effect finalization; typed ResearcherUpdate fate-shares its exact Dolt commit with the event CAS protocol. |
| corpusd world-wire SQL | object graph and route-ledger SQL writers | Reuse for narrow event head/idempotency, mode, and lifecycle control rows. corpusd mechanically performs authenticated CAS; it does not choose semantic events. |
| artifact/blob service | existing content-addressed writers | Reuse for immutable event bodies, payloads, bundles, receipts, releases, and checkpoints. PinReceipt is required before head CAS. |
| ComputerVersion constructor/verifier | vmctl/computerversion materializer and verifier | Reuse for immutable checkpoints/reconstruction. Constructor injects one realization-local credential envelope outside observed artifact files. |
| route ledger CAS | `routeledger.SQLLedger.ApplySignedTransition`, invoked by vmctl route authority | vmctl remains sole route-slot actuator. Post-genesis self-development slots require accepted-event and promotion-join evidence plus RouteProjectionCertificate. Route CAS never appends or acknowledges a semantic event. |
| current vmctl owner/G3 promotion evidence | bootstrap/promotion candidate code | Historical/pre-genesis only for the initialized ComputerID. After genesis, legacy bootstrap/promote/rollback requests without the new certificate refuse. |
| AppChangePackage, AppAdoption, source lineage | agentcore APIs, shipper and package tools | Retain for unrelated source/app sharing only after caller classification. Remove from self-development acceptance and activation; no package/lineage record can change guest code or event heads. |
| role profile/tool registry | `agentprofile.PolicyFor`, `Runtime.InstallDefaultAgentTools` | Clean cutover: Super orchestration/capsule/inspection only; CoSuper effect verbs only through capsule broker; VSuper aliases refuse; Researcher read/research/evidence plus typed `update_coagent` only; all other profiles lack writable/coding/shipper/VM authority. |
| worker-VM/candidate-VM delegation and lifecycle | Super prompts/tools, worker controller, VM lifecycle APIs, VSuper/candidate aliases | Delete the complete obsolete substrate: controllers, handlers, clients, tool registrations, prompts, profiles/aliases, configuration, fallback synthesis, tests that assert availability, and docs. Generic delegated agents remain durable runs/trajectories using capsules; they are not worker VMs. |
| capsule HostClient/HostAuthority | guest-vsock client and `cmd/capsule-host` | Delete from the production self-development path and deployment. No host service, key, configuration, or fallback. |
| capsule Executor/Broker | scaffolded namespace/cgroup/overlay/broker code | Replace scaffolds with mandatory guest-local isolation, guest authority, AF_UNIX broker, resource/path/network policy, revocation, and complete cleanup. |
| capsule tools | defined but not installed by default; exec/file verbs return `not_implemented` | Wire only after mandatory isolation. Remove `not_implemented` success-shaped responses; unavailable isolation returns typed refusal. |
| CLI run list/cancel | `/api/agent/*` | Migrate to public `/api/runs` resources before citation. Internal routes remain unavailable to external keys. |
| existing CLI auth | `CHOIR_API_KEY` or plaintext `--api-key` | Delete plaintext flag for every command. Use environment or mode-0600 `--api-key-file`; prompts/reasons use file or stdin and are never echoed. |
| guest materializer | none for accepted guest code | Add root-owned `choir-updater`, release slots, atomic pointer, guest-service-only restart, health/recovery receipts, and event-derived rollback. |

Non-test production callers are the only basis for retaining an obsolete subsystem. Tests, prompts, comments, and docs citing deleted paths are migrated in the same implementation boundary.

## V1 durable schemas

All digest columns are lowercase hexadecimal SHA-256. All JSON columns contain RFC-8785 canonical JSON bytes. SQL timestamps are UTC with microsecond precision and serialize as RFC-3339; signed receipt timestamps retain the exact RFC-3339 precision present in their canonical preimages. Schema evolution is additive only.

### Existing corpusd world-wire SQL server

`computer_event_heads` is the one per-computer CAS row:

- primary key `computer_id`;
- `sequence`, `canonical_event_head`, `desired_event_head`, `effective_event_head`;
- `desired_state_commitment`, `effective_state_commitment`;
- nullable `pending_transition_ref`;
- `reducer_version`, `credential_revocation_epoch`, `created_at`, `updated_at`.

Constraints require sequence greater than zero after genesis, 64-character heads/commitments, and pending transition consistency. Genesis is insert-if-absent with zero previous head. No API deletes or resets this row.

`computer_event_append_receipts` stores durable idempotency and signed CAS results:

- primary key `(computer_id, idempotency_key)`;
- unique `(computer_id, sequence)` and `(computer_id, event_digest)`;
- `request_commitment`, `previous_head`, `event_kind`, `event_digest`, `event_artifact_ref`, `event_pin_receipt_digest`;
- canonical `pin_receipt_digests_json`, `event_head_receipt_json`, `event_head_receipt_digest`;
- derived desired/effective heads, commitments, pending ref, `created_at`.

Same key plus identical commitment returns this row; changed commitment conflicts before effects.

`control_key_history` pins public-key history before its authorizing key event:

- primary key `(signer_domain, computer_id, key_id)`, with the empty
  `computer_id` reserved for platform/owner-recovery domains;
- raw 32-byte `public_key`, `status`, nullable activation sequence/time,
  nullable first-invalid sequence/time, nullable replacement key;
- canonical authorizing receipt JSON/digest and insertion time;
- indexes `(signer_domain, computer_id, status, activation_sequence)` and
  `authorizing_receipt_digest`.

corpusd is the only writer. It inserts and verifies the rotation/revocation
receipt while the old key remains valid, the guest appends the authorizing key
event, and corpusd activates the new key only after the EventHeadReceipt.
Emergency replacement follows the owner-recovery-signed special CAS in the
Definition. Old public keys are retained indefinitely.

`computer_self_development_modes` is platform control, not semantic state:

- primary key `computer_id`;
- `mode`, `generation`, nullable bound `operation_id`, `bundle_digest`, expected desired/effective heads and commitments, nullable `expires_at`;
- `last_idempotency_key`, `last_request_commitment`, `mode_receipt_json`, `mode_receipt_digest`, `updated_at`.

Bindings are null for `off`, `audit_only`, and `propose_only`, mandatory for `accept_once`. Generation CAS and idempotency are enforced transactionally.

`computer_lifecycle_receipts` is the stopped-guest-safe projection:

- primary key `(computer_id, idempotency_key)`;
- unique `receipt_id`;
- `request_commitment`, `action`, prior/resulting state, `generation`, `receipt_json`, `receipt_digest`, `completed_at`, nullable `joined_event_digest`.

No lifecycle row claims guest semantic acceptance. The appender joins a verified receipt as `lifecycle_observed` after start.

Required indexes are `computer_event_append_receipts(computer_id, sequence)`, `(computer_id, event_digest)`, modes by `(mode, expires_at)`, and lifecycle by `(computer_id, generation)`. Foreign keys bind receipt rows to the head/mode computer where Dolt supports the required transactional semantics; application validation remains mandatory.

### Discardable migration rehearsal

The exact G0 corpusd DDL was rehearsed twice, unchanged, against a disposable
Dolt 1.84.1 repository. Both additive executions succeeded and `SHOW TABLES`
returned `computer_event_heads`, `computer_event_append_receipts`,
`control_key_history`, `computer_self_development_modes`, and
`computer_lifecycle_receipts`. The rehearsal SQL SHA-256 is
`0ef5cb592e671653608c2868738181a7a0113c6d22674dde08fface61e9aabac`; its
ephemeral source and database are `/tmp/choir-selfdev-g0-migration.sql` and
`/tmp/choir-selfdev-g0-migration-db`. They are diagnostic evidence, not
repository artifacts, and are discarded before B.

The rehearsal includes all primary/unique/lookup indexes named in this packet.
B converts this frozen DDL into the repository's migration mechanism and adds
transactional migration tests before any staging use. It may add only
implementation-neutral constraints required by Dolt; table ownership, columns,
keys, index semantics, and additive-only migration behavior are frozen.

### VM-local embedded Dolt

`computer_event_index` contains `event_digest` primary key, unique sequence, previous head, kind, event/payload artifact refs, privacy class, trajectory/parent/capsule refs, request/idempotency commitments, signed receipt digest, and reducer version.

`computer_event_prepares` contains `(computer_id, idempotency_key)` primary key, request commitment, expected projections, event digest/ref, proposed semantic commit ref, state `prepared|head_committed`, and timestamps. Rows finalize into the index/state or are safely discarded/recovered; they are not independent authority.

`computer_effective_state` has one row per ComputerID containing canonical/desired/effective heads, desired/effective commitments, pending transition ref, reducer version, effective CodeRef, ArtifactProgramRef, embedded-state ref, release digest, checkpoint ref, and last receipt digest. Its values are deterministic reducer output verified against corpusd.

`self_development_operations` projects operation ID, computer ID, request commitment, trajectory/capsule/base/bundle/verifier refs, immutable decision bindings, desired/effective heads, materialization/checkpoint/route receipts, mode/lifecycle receipts, state, and terminal error. It is rebuilt from events and immutable receipts; it cannot authorize an append.

Indexes cover trajectory, parent event, capsule, operation state, bundle digest, and effective head. There is no embedded canonical-head override and no third service/database.

## Canonical event and receipt protocol

Event V1 uses the exact minimum hashed envelope and event kinds in the active Definition. `event_id` is UUIDv7. `canonical_event_head = SHA256(RFC8785(event body))`; the body contains no digest of itself. The repaired commitment graph is directed: `pin_intent_commitment = SHA256(RFC8785(immutable event intent + transition input))`, excluding sequence, previous head, final request commitment, and receipt digests; payload PinReceipts bind that intent; `request_commitment = SHA256(RFC8785(event intent + pin_intent_commitment + ordered payload PinReceipt digests))`; the event-body PinReceipt binds the final request commitment; append CAS and EventHeadReceipt bind the ordered receipts. No signed receipt depends on its own digest. Signed receipts otherwise use `choir-receipt-v1`, the exact omitted-field digest rule, ordered required signers, Ed25519 signatures, trust roots, rotation/revocation, kind fields, and verifier matrix in the Definition. Unknown fields are retained by V1 readers. Unknown versions refuse mutation.

Append order is: reconcile corpusd and embedded projections; classify/redact/encrypt each private payload into its final random-nonce envelope through the public guest client; freeze content refs in immutable event intent; compute immutable pin intent; have the guest appender TCB AEAD-decrypt/authenticate each exact frozen envelope against expected ComputerID/EventID with the root guest keyring immediately before pin; pin unchanged bytes against that intent; compute the final request commitment over canonical `{event_intent,pin_intent_commitment,payload_pin_receipt_digests}`; pin the final event body against the final commitment; commit embedded prepare; authenticated corpusd head CAS; have keyless corpusd structurally re-open private envelopes and require their ComputerID/EventID to match the event; verify EventHeadReceipt; finalize embedded index/materialization; acknowledge. Crash recovery and stale causal rebase follow the Definition exactly. Decision/effective events never rebase across changed state projections. At most one pending transition exists.

`GenesisImported` is the sole absent-row transition and freezes baseline ComputerVersion, source tree, embedded state, effective marker, reducer, updater key, ComputerID, and disposable authorization. It initializes canonical, desired, and effective heads/commitments together and leaves mode off. Pre-genesis data remains `legacy_unproven`; no history is synthesized or rewritten.

## Privacy and retention V1

Complete audit means complete causal envelopes and content commitments, not plaintext forever. Before hashing, known secret forms become typed non-reversible handles. Detection failure quarantines/refuses the append and revokes the implicated capability. Owner-private message/tool payloads use XChaCha20-Poly1305 with a random nonce and the exact AAD named in the Definition; ciphertext is content-addressed. The root guest credential service owns the per-computer keyring. Large results use immutable artifact refs and selectors; truncation cannot replace a digest. This mission retains canonical envelopes, accepted effects, authority/verifier/rollback receipts, and encrypted trajectory payloads through terminal acceptance. Reconstruction never reruns a model, tool, or network call.

## Guest capability and capsule V1

The guest core owns opaque random 256-bit capability handles in process. A handle binds ComputerID, run, role, capsule, verbs, policy, resources, expiry, and revocation epoch and is injected only in execution context. It is resolved before connection to a per-capsule permissioned AF_UNIX broker. Peer credentials, socket ownership, capsule identity, verb/path/network policy, seccomp, Landlock, namespaces, cgroup v2, overlay layout, no-new-privileges, and capability drop are mandatory. Build capsules have no network, secrets, host/vsock device, bind mount, or mutable dependency cache. Restart revokes handles. Destroy must kill the process tree, unmount overlays, remove cgroups/sockets/upperdirs, verify absence, and refuse new admission after cleanup failure.

The immutable `CapsuleEffectBundle` has the exact required fields in the Definition, deterministic ordering, stale-base check, secret scan, independent read-only-capsule verification, and rejection rules for escape/special-file/host/network/resource content.

## Guest updater, projection, API, and activation V1

`choir-updater` is root-owned and outside the runtime/capsules. It stages digest-named read-only releases, verifies the accepted event and immutable closure, swaps a root-owned pointer, restarts only the guest Choir service, probes health/schema/marker, and emits guest-core-signed receipts. Failure restores the prior pointer and service; a signed recovery receipt remains importable if the runtime cannot append. Rollback selects a prior applied event/checkpoint and preserves all history.

Checkpoint publication and route projection occur only after effective application. Route authorization uses the Definition's two existing AuthorizationEvidence wrappers, exact inner payloads, wrapper hash semantics, RouteProjectionCertificate, and byte-exact normalized TransitionCommand. vmctl verifies every join and remains the only route CAS writer. Post-genesis legacy route authority refuses.

The public CLI grammar, API routes, scopes, decision bindings, operation states/receipts, file-only secret/prompt/reason inputs, explicit ComputerID targeting, mode matrix, lifecycle projection, and idempotency rules are exactly those in the Definition. `off` is default and permits only declared reads/control, lifecycle, safety rollback, deterministic refusals, and one authorized genesis. `accept_once` is exact-operation/bundle/head/commitment/generation/expiry bound and returns to `propose_only` after decision.

## Obsolete-path dispositions

- Delete `cmd/capsule-host` from deployment and remove production HostClient/vsock authority. Historical source may be deleted once every citer is migrated; no compatibility shim.
- Delete VSuper/candidate-super production profile aliases, overlays, spawn rules, tool registry, and self-development prompts. Requests fail closed.
- Remove direct Super and CoSuper bash/write/coding/shipper/VM/route tools. CoSuper receives only capsule broker effect verbs; Super receives capsule orchestration and read/decision-proposal tools.
- Delete all worker-VM/candidate-VM lifecycle, controller, handler/client, tool, prompt, profile/alias, configuration, fallback, and availability-test code. There is no unrelated-feature retention exception. Generic delegated agents use durable runs/trajectories and capsules only.
- Remove capsule success-shaped `not_implemented` responses and process-local TransactionTape authority.
- Migrate choir run list/cancel from `/api/agent/*` to `/api/runs`; keep internal routes inaccessible externally.
- Remove plaintext `--api-key` globally and plaintext prompt/reason arguments from self-development.
- Retire owner/G3 predecessor route receipts as post-genesis authority; retain them only as immutable historical/R0 evidence.
- Keep the tag-only `DoltPromotionAdapter` unwired and remove any self-development citation; destructive reset remains forbidden.

A deletion inventory check must search code, tests, prompts, `docs/`, `specs/`, deployment modules, and command registration. No deleted symbol/path may retain a live citer.

## Rollback and gate refs

R0 is source/deploy identity before the certificate-enforcing cutover and is valid only before genesis. R1 is the exact deployed certificate-only route-refusal, event/key-reader, updater-recovery, and kernel-receipt security floor and is the minimum platform rollback after genesis. Before acceptance, destroy/revoke the capsule and retain audit. After acceptance, event-roll back with `choir-updater` to a prior compatible applied head. Schema/event history is never deleted or reset. If R1 or a later compatible release cannot serve safely after genesis, stop the disposable computer and forward-repair from immutable inputs.

### G0 repair receipt: cyclic pin/request commitment

The first B implementation pass exposed a substrate blocker in the frozen G0
contract: payload PinReceipts were required both to bind the final request
commitment and to contribute their signed receipt digests to that commitment.
Receipt IDs, times, and signatures made the graph cyclic and non-executable.
The active Definition recorded the blocker before repair code.

The repaired graph above preserves every immutable binding without a cycle.
`TestPrivatePayloadAppendCompletesDirectedCommitmentGraph` encrypts a real
private payload with XChaCha20-Poly1305, pins its envelope against the immutable
intent, computes the final request from the signed payload receipt, pins the
event, executes corpusd CAS, verifies EventHeadReceipt, and finalizes embedded
Dolt. The focused command
`go test ./internal/platform -run TestPrivatePayloadAppendCompletesDirectedCommitmentGraph -count=1 -v`
passed on 2026-07-19. This packet is not re-accepted until an independent
diverse G0 panel reviews the exact repair and reports no reproducible blocker.

The first corrected-formula panel completed 4/4 reviewers. Three accepted, but
Codex reproduced two product-path blockers: the public client attempted to
encrypt and pin before the random-nonce envelope digest could enter event
intent, and corpusd did not reject an envelope created for a different EventID.
The minority blocker governed. The repair separates public envelope preparation
from exact-envelope pinning, drives the focused test through authenticated HTTP
client/handler pin and CAS routes, and rejects cross-event private envelopes.
The focused test passed after repair; a fresh frozen panel remains required.

A subsequent 4/4 panel again had a three-to-one accept/block split. Codex
demonstrated that structural envelope parsing did not authenticate AEAD
metadata or ciphertext. The guest appender now decrypts/authenticates the exact
frozen bytes with its root keyring before issuing any pin request; corpusd stays
keyless and structurally validates identity/content joins. The HTTP product-path
test independently corrupts metadata and ciphertext and proves both refuse
before pin while the unmodified envelope completes append. A fresh frozen panel
remains required.

The next panel again found one decisive minority blocker: kernel-argv delivery
copied the envelope into the shared sandbox/updater environment, and consumed
exchange replay returned the same full append bearer. The repair replaces that
path with a dedicated realization-local ext4 credential drive, removes the
secret from kernel arguments and shared environment, masks the mount from the
updater, requires trusted core to consume a root-owned mode-0400 regular file
and unlink it before exchange, and refuses all consumed-envelope replay without
a bearer. Focused drive/config, consume/erase/mode, replay-refusal, private HTTP
append tests, six affected package suites, and NixOS guest evaluation pass. A
fresh frozen panel remains required.

The following panel found that a root updater sharing guest process visibility
could still ptrace trusted core and steal its in-memory renewable bearer. The
updater now has a private PID namespace, empty capability and ambient sets,
invisible PID-scoped procfs, and `~@debug` syscall filtering; its identity-only
environment contains no credential or gateway token, and its credential mount
remains masked. Generated NixOS service configuration confirms every setting.
A fresh frozen panel remains required.

The retry review found updater's general `systemctl` access could escape every
namespace restriction through a transient root service. Updater no longer
invokes or reaches PID 1: it may atomically create only a fixed restart trigger.
A root-owned path/oneshot bridge from the immutable Nix store removes that
trigger and restarts exactly `go-choir-sandbox.service`; systemd and dbus
control sockets are masked from updater. Fixed-trigger/arbitrary-target tests,
updater compile, NixOS toplevel evaluation, and path-unit evaluation pass. A
fresh frozen panel remains required.

### Terminal G0 repair panel receipt

The final frozen runtime/Nix/test candidate used complete candidate SHA-256
`ec65831c1df8abf7b068e49bc6e7a1c2640a1aa327d5d1c494f065f654d2c203`
over base `fa3721e511179a937fef4d9328b36f9e2a96d886`, tracked binary diff,
and 37 sorted untracked files with the Definition's length framing. An initial
panel yielded one OpenCode acceptance while Codex was provider-interrupted and
Cursor timed out. A tightly scoped retry completed Codex, Cursor, and OMP
GPT-5.5; all three returned `ACCEPT_G0_REPAIR` with no reproducible blocker.

The accepted retry artifacts are content-bound as follows:

- prompt SHA-256 `1bd6f61713e120234b541b73e9832017b28d966fbc54b9f88ff73d456ac16b39`;
- manifest SHA-256 `18a9a05416a6cf50703af5e2e31efe43657909386515ca7b8e297c2a2f6a8a6c`;
- Codex output SHA-256 `a3bcd7f969968b9473955de44819f43c81a73b9b22977a1b3abecd1d128759ed`;
- Cursor output SHA-256 `2d2a1323c5d796532a0a6d173eef1e7f07eeac965ba38d0fbf7ae002c927e6dd`;
- OMP GPT-5.5 output SHA-256 `455869251c29f6245caac0e3d21bb8cc3912b7baa7020808b9bd296f29a45838`.

G0 is re-accepted. B may resume. Adding this receipt changes only assurance
prose, not the reviewed runtime, Nix, protocol, or tests, so the accepted
candidate review remains current. G1 must review the complete frozen B
candidate; deployed R1 must prove the service namespace and credential
boundary in the exact guest realization.

## G0 deterministic checklist

- Exactly two Dolt stores; corpusd control tables are not a third semantic store.
- Exactly one semantic `ComputerEventAppender` per ComputerID; corpusd is mechanical CAS and vmctl is route actuator only.
- Constructor credential envelope is root-owned mode 0400, realization-local, outside observed artifacts/logs/context, single-use, rotated/revoked, and reconstructed fresh.
- Receipt preimages, signers, trust history, route joins, reducer transitions, genesis, recovery, privacy, API auth, mode matrix, and rollback are total and non-circular.
- Every current writer/caller has an explicit retain/replace/delete/refuse disposition.
- Immutable-image kernel evidence is positive; deployed exact-realization receipt is public, signed, fresh, runtime-probed, and a hard C-before-D gate.
- No behavior implementation begins until a frozen diverse panel reports no reproducible G0 blocker.
