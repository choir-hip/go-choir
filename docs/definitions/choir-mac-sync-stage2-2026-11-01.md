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
readiness: drafted
readiness_authority: >-
  Owner, 2026-10-31, roadmap stage 2: "The Mac app with sync"; "Anything
  which is about self-dev, autopaper, or psa, waits until after we get
  storage growth, memory spikes, and mac app/sync working well." Owner,
  2026-10-31: keep doing other work while the data system is observed.
  Readiness is drafted: placing a computer key on a second device is a
  security boundary, so this file is reviewed (Devin) before it executes.
review:
  reviewer: pending (one Devin seat on this file; the key transfer build is tier 2)
  frozen_ref: none
  verdict: none
  evidence_ref: none
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
    - a key release route: the guest exports its private-artifact key wrapped
      to a Mac device key, only on an owner passkey assertion, recorded on the
      tape as a key-release event naming the device;
    - the Mac stores the wrapped key in the Keychain (device-only, no iCloud
      sync), and revocation (owner act) rotates the computer key so later
      payloads are unreadable on a revoked Mac;
    - a local reader: replay of the local tape into a Mac-side SQLite
      projection (the guest's projector code, no VM), served to the app's
      surface when the backend is unreachable, read-only;
    - lane/mac-arm-guest landed once release churn is quiet.
  acceptance: >-
    Staging E2E, no person (email sign-in test account): sign in, sync,
    release the key, cut the network (backend unreachable), open a document
    the test account wrote and read its text from the local copy; revoke,
    write a new document online, sync, and the new payload does not decrypt
    on the Mac. A repeatable artifact records each step.
  rollback: >-
    The key release route behind a host switch (off refuses releases); the
    Mac deletes its Keychain item on revoke or sign-out; revert removes the
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
      owner passkey assertion, and every release is a tape event;
    - the Mac copy stays read-only: no write path from the Mac to the computer
      in this station;
    - the local reader never runs model calls or effects;
    - adversarial drills on d@e only; never the paper or the owner's computer.
  exclusions:
    - the Mac as a writer, bursting, workstation sites (resource fabric, later);
    - File Provider, voice, computer use (later stations).
now:
  status: drafted
  slice: S0, review this file
  believed_state: >-
    Sync and status work; the only gap to offline reading is the key and a
    reader. The guest already exports its key for escrow, so the release is a
    new authorization, not a new export mechanism.
  main_uncertainty: >-
    Whether a passkey assertion the host verifies can authorize a guest-held
    key release without the host ever seeing the key (the guest wraps it to
    the Mac's public key; the host relays).
  next_observation: >-
    Read ExportKeyForEscrow and the escrow path, and the desktop session's
    passkey flow, to confirm the guest can verify the owner's assertion (or
    trust the host's verification) before wrapping.
  next_action: one Devin seat reviews this file; then S1, the key release design.
receipts: []
```
