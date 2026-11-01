# Choir Mac app with sync, stage 2: read your computer on the Mac, offline (2026-11-01)

Run with `/goal docs/definitions/choir-mac-sync-stage2-2026-11-01.md`.

Roadmap stage 2 (`docs/roadmap-2026-10-31.md`): "The owner's Mac follows its
computer's tape, and the groundwork for the whole native layer is laid." Sync
slice 1 (013c7de93) gives the Mac a verified, read-only local copy of the
tape and snapshot, with a menu-bar status; email sign-in (d439ba098,
0cff2f27a) lets the app be tested with no person. What the Mac cannot do yet
is read that copy: private payloads are envelopes sealed under the
computer's key, and the key is not on the Mac. This station puts it there
and opens the local copy when the server is unreachable. The end-state design
is `docs/design/resource-fabric-2026-10-31.md` (the Mac as a site; keys in
the Keychain, section on native capabilities).

```yaml
definition_version: throughline/1
readiness: reviewed
readiness_authority: >-
  Owner, 2026-10-31, roadmap stage 2: "The Mac app with sync"; "Anything
  which is about self-dev, autopaper, or psa, waits until after we get
  storage growth, memory spikes, and mac app/sync working well." Owner,
  2026-10-31: keep doing other work while the data system is observed.
  Readiness is drafted: placing a computer key on a second device is a
  security boundary, so this file is reviewed (Devin) before it executes.
review:
  reviewer: Devin (one seat on this file; the key release build is tier 2)
  frozen_ref: 7d463c3fc (round 2; round 1 send_back on d61726726, 13 findings taken)
  verdict: accept (three doc nits applied in this commit)
  evidence_ref: scratchpad review-stage2/devin.out and devin2.out, 2026-11-01
start:
  observed_at: 2026-11-01
  canonical_ref: origin/main at e4f4b256a
  deployed: 86768d8b4 and later docs-only commits on Node B
  artifact: >-
    cmd/desktop: Wails app with the background syncer (sync.go, internal/tapefollow),
    menu-bar status (sync_tray.go: last synced event, Sync now, Show local copy
    in Finder), email and passkey sign-in, an offline page when the backend is
    unreachable (surface.go). The local copy verifies as one tape (staging E2E
    step F2). Private payloads in it are sealed envelopes; no key on the Mac.
  candidates:
    - lane/mac-arm-guest at f157c2f7a: aarch64 guest image, builds and boots
      under vfkit; held from main because flake.nix changes the floor digest
      and would ship a guest release during the lean-system observation.
  unknowns:
    - how the computer key leaves the guest (it is guest-held: escrow export
      exists for backup, ExportKeyForEscrow) and how the owner authorizes it;
    - whether the Mac reader should reuse the guest's projector (SQLite replay
      of the local tape) or read payloads directly.
finish:
  deliver: >-
    With the network off, the owner opens the Mac app and reads their own
    computer's documents and recent activity from the local copy, decrypted
    on the Mac with a key that sits in the macOS Keychain and was released
    by the owner's passkey.
  artifact:
    - "S1 key release: the Mac makes a device key pair (S1 decides X25519,
      reusing keyescrow.SealDEK, or Secure Enclave P-256 with its own wrap);
      the owner approves with a passkey whose WebAuthn challenge commits to
      {device public key, computer id, release nonce}; the host verifies the
      assertion (the credential registry is only in internal/auth) and relays
      to the guest; the guest wraps its private-artifact key to that device
      key and appends a new key-domain tape event (device_key_released,
      naming device and key version, its authority_ref naming the verified release ceremony), never key_rotated or key_revoked,
      which are the guest credential lifecycle; the new kind lands with the
      reader-compat marker so older followers skip it."
    - "Grant scope, stated: the released key opens sealed payload envelopes,
      sealed snapshot chunks, and the file CAS digest key; all three are what
      the reader needs."
    - "Keychain: the device private key is the secret (this-device-only, not
      synchronizable); the wrapped key blob is not. Sign-out keeps the device
      key (no new ceremony per sign-in); removing the device deletes it."
    - "A devices surface: the menu bar and the surface list each released
      device with its release time, from the tape."
    - "S2 local reader: replay of the local tape into a Mac-side SQLite
      projection (ReconstructInto with the guest's projector, no VM), served
      read-only to the app's surface when the backend is unreachable."
    - "S3 revocation, its own slice after S2: a multi-version keyring
      (resolve by KeyVersionDigest), escrow holding every version,
      InstallGuestPrivacyKey accepting a next version, key delivery serving a version by digest, and a rotation event
      that re-wraps the new key to every device not revoked (the set is the
      fold of release and revoke events). Until S3, a released Mac reads
      what it was released for; that limit is stated to the owner at
      release."
    - lane/mac-arm-guest landed once release churn is quiet.
  acceptance: >-
    Staging E2E, no person, with a pre-minted test passkey credential
    (resource fabric: enrollment needs a passkey session): sign in, sync,
    release the key, cut the network, open a document the test account wrote
    and read its text from the local copy; the devices surface lists the
    Mac. Negative steps: an email session's release is refused; an assertion
    whose challenge names another device key is refused; a release while the
    computer is down fails with a stated cause. S3 adds: revoke, write online,
    sync, the new payload does not open on the revoked Mac and does on a
    second released device. A repeatable artifact records each step.
  rollback: >-
    The key release route behind a host switch (off refuses releases); the
    Mac deletes its device key on device removal or revoke; revert removes the
    reader. A released key cannot be un-released: revocation is rotation.
  landing: commit, push main, CI, staging deploy identity, the E2E above.
value:
  better_means: >-
    Minimize what the owner cannot read on their own Mac without the server,
    while no key leaves a device without an owner passkey and every release
    is on the tape.
  goodharting_would_be: >-
    Shipping the key in plaintext, or decrypting on the server and syncing
    plaintext, so the offline demo works while the key boundary is gone.
homotopy:
  realism_axis: >-
    one document read offline on d@e's test account -> the owner's computer
    -> every app surface offline (files, Texture, mail) -> the Mac as a
    writer site (later stations, resource fabric).
boundaries:
  mutation_class: red (key material, auth, a new route on the guest and host)
  invariants:
    - a computer key leaves the guest only wrapped to a device key, only on an
      owner passkey assertion whose challenge commits to that device key, the
      computer and a fresh nonce, and every release is a tape event;
    - the Mac never writes the tape or the computer's state; it submits owner
      ceremonies (release, later revoke) that the guest executes and records;
    - release needs the computer's VM running (the key exists only in the
      guest and in two-approval escrow); offline reading needs only a key
      already released;
    - the local reader never runs model calls or effects;
    - adversarial drills on d@e only; never the paper or the owner's computer.
  trust_model: >-
    The host verifies the passkey; the guest trusts the host's verified
    request; the tape records the release. A compromised host already reaches
    any key through key delivery (platform/key_delivery.go), so this adds no
    new host power. A compromised guest holds the key anyway and writes its
    own tape: for it, the tape event is detection for honest guests, not
    prevention (a named residual).
  why_not_root_prf: >-
    The owner ROOT (keyescrow.SealRoot, DerivePRFWrapKey, the unwrap_root
    login path) already delivers a PRF-protected owner secret to the client.
    A per-device key is chosen instead: least privilege per device, revocable
    per device in S3, and the resource fabric's site-key model.
  exclusions:
    - the Mac as a writer, bursting, workstation sites (resource fabric, later);
    - File Provider, voice, computer use (later stations).
now:
  status: reviewed
  slice: S1, key release design
  believed_state: >-
    Sync and status work; the only gap to offline reading is the key and a
    reader. The guest already exports its key for escrow, so the release is a
    new authorization, not a new export mechanism.
  main_uncertainty: >-
    Whether auth's WebAuthn ceremony can carry the device-key binding in its
    challenge from the Mac app (ASWebAuthenticationSession to the system
    browser) and from a headless test credential alike.
  next_observation: >-
    Read ExportKeyForEscrow and the escrow path, and the desktop session's
    passkey flow, to confirm the guest can verify the owner's assertion (or
    trust the host's verification) before wrapping.
  next_action: >-
    S1 design note (device key algorithm, challenge binding through auth's
    WebAuthn ceremony, the device_key_released event and its compat marker);
    executable once the owner points at it or the lean-system watch settles.
receipts: []
```
