# Releasing a computer's key to the owner's Mac (stage 2, S1)

Date: 2026-11-01. Status: design for review (red; the build is tier 2).
Goal file: `definitions/choir-mac-sync-stage2-2026-11-01.md`, slice S1.
End state: `design/resource-fabric-2026-10-31.md` (keys and passkeys row:
Keychain and Secure Enclave on the Mac).

## The problem

The Mac holds a verified copy of its computer's tape and snapshot
(`internal/tapefollow`), but the private parts are sealed under the
computer's private-artifact key (the DEK, `computerevent/privacy.go`). That
key exists in the guest and, wrapped, in two-approval host escrow. To read
offline, the Mac needs the DEK. The release must be:

- **owner-approved, for this device**: an owner passkey assertion whose
  challenge commits to the device's public key, the computer and a fresh
  nonce;
- **wrapped end to end**: the guest wraps the DEK to the device key; the
  host relays ciphertext only;
- **recorded**: every release is on the computer's tape and visible to the
  owner;
- **not over-granted**: an email session cannot release a key.

What the DEK opens, stated once: sealed payload envelopes, sealed snapshot
chunks (`snapshot.go:290`), and the file CAS digest key derived from it
(`autoputer/run.go:483`). The local reader needs all three.

## Decisions

### D1. The device key: Secure Enclave P-256, a software P-256 key for tests

The device key is a P-256 key-agreement key created in the Mac's Secure
Enclave (CryptoKit `SecureEnclave.P256.KeyAgreement.PrivateKey`), never
exportable, access control `.privateKeyUsage` with
`kSecAttrAccessibleWhenUnlockedThisDeviceOnly`, not synchronizable. A Mac
without a Secure Enclave (none of the supported Apple Silicon Macs) refuses
release rather than falling back to a software key. Headless tests use a
software P-256 key through the same wrap code.

Considered: X25519 in the Keychain, reusing `keyescrow.SealDEK`. Rejected:
the private key would be extractable by anything that can read the
Keychain item under the user's session; the Secure Enclave key cannot leave
the hardware. The cost is a P-256 wrap beside the X25519 escrow wrap, about
sixty lines on Go's `crypto/ecdh`.

The device id is the SHA-256 of the device public key (SEC1 uncompressed),
hex. The device name is the Mac's computer name, display only.

### D2. The wrap

`devicekey.SealDEK(devicePub, computerID, deviceID, keyVersionDigest, dek)`:
an ephemeral P-256 key, ECDH with the device key, HKDF-SHA256 with info
`choir/devicekey/v1 || computerID || deviceID || keyVersionDigest`,
XChaCha20-Poly1305 over the DEK with the same string as associated data.
The record carries the ephemeral public key, nonce, ciphertext, key version
digest and device id. It is not secret: it is stored next to the local copy
and it rides the release response. Opening needs the Secure Enclave key.
Tests name the failure modes first (wrong device, wrong computer, wrong key
version, tampered field: each fails to open).

### D3. The ceremony: auth binds the challenge

Auth already runs WebAuthn with go-webauthn v0.16.4, which takes a caller
challenge (`webauthn.WithChallenge`). A release attempt is a new auth
ceremony type, `device_release`:

1. **Begin** (`POST /auth/device-release/begin`, from the app with its
   session): refused unless the session's method is passkey (the access
   JWT's `am`; email sessions are refused here as on the credential and ROOT
   routes). Body: computer id, device public key, device name. Auth checks
   the computer is the caller's, makes a 32-byte nonce, and sets the
   challenge to `SHA-256("choir/device-release/v1" || computerID ||
   devicePub || nonce)`. The attempt (id, user, computer, device pub, name,
   nonce, session data, five-minute expiry) is stored with the existing
   challenge state.
2. **Approve**: the app opens the system browser (the existing
   `ASWebAuthenticationSession` path) at `/devices/approve?attempt=<id>`.
   The page shows the device name and the first 16 hex of its id, which the
   app also shows in its own window, so the owner sees the same device in
   both places, then runs the assertion.
3. **Finish** (`POST /auth/device-release/finish`): auth recomputes the
   challenge from the stored attempt (never from the request), verifies the
   assertion with the stored session data (user verification required), and
   consumes the attempt once. A challenge naming any other device, computer
   or nonce fails here, because the authenticator signed the stored one.

The owner's passkey is the authority. Auth is the only verifier: the
credential registry lives only in `internal/auth`, and the guest has no
trusted copy of the owner's credentials.

### D4. The relay and the guest's release

On a verified finish, auth hands the release to the host's computer route
(the same host-to-guest path the proxy uses for owner requests, admitted by
`GuestHostCaller`), carrying the release id, device id, device pub and name,
and the assertion's digest. The guest:

1. refuses unless the computer's key is loaded (the VM is running: a stopped
   computer answers "start the computer and retry", the release is not
   queued);
2. validates the device pub (on curve, not the identity), recomputes the
   device id;
3. appends a projection batch with one op, `device_key_released` (table
   `device_key_releases`: release id, device id, device pub, name, key
   version digest, released at, assertion digest), its `authority_ref`
   `device-release:<release id>`;
4. only after the append is durable, wraps the DEK (D2) and returns the
   record.

Release is recorded before the key leaves. A crash between append and
return leaves a recorded release with no key delivered; the app retries
with a new attempt, and the devices list shows both.

The record is a projection op on the tape, not a new canonical event kind:
it is guest state, replayed like every other row; corpusd validates every
event kind (`event_artifacts.go:34-41`) and a new kind is a host schema
change for no gain; `key_rotated` and `key_revoked` are the guest credential
lifecycle and are not reused. An older reader meets the op through the
existing "unknown op in a known batch" rule (recoverable changes SR7): the
batch declares it skippable with its table, which no older manifest has.

### D5. The Mac side

The app stores the release record beside the local copy
(`~/Library/Application Support/Choir/<computer-id>/device-key.json`). To
open, it asks the Secure Enclave for the ECDH secret with the record's
ephemeral key, derives the wrap key, opens the DEK into memory, and uses it
only in-process; the DEK is never written to disk or the Keychain. Sign-out
keeps the device key (no ceremony per sign-in); "Remove this Mac" in the
menu deletes the Secure Enclave key and the record.

### D6. The devices surface

The guest serves its `device_key_releases` rows to the owner's session
(`GET /api/devices`); the app's menu shows "This Mac can read offline since
<time>", and a devices page lists every release. Until S3 the page says
plainly that a released Mac can read everything sealed under the current
key, and that revoking needs S3.

## Threats

| Actor | What it can do | What stops it |
|---|---|---|
| Renderer script in the app window (server-delivered JS) | Ask for a release naming its own key | The begin call comes from native code with the device's own key; the renderer has no route to begin with another key, and the approve page shows the device id the native app also shows |
| Network attacker | Replay an assertion | The challenge is single use and bound to the stored attempt |
| Email-session thief | Release a key | Begin refuses an email session |
| Compromised host | Release to its own key; read any DEK | Already true through key delivery (`platform/key_delivery.go`); this adds no host power. Named, not prevented |
| Compromised guest | Exfiltrate the DEK without a record | Holds the DEK anyway; the tape record is detection for honest guests only. Named residual |
| Stolen unlocked Mac | Read the local copy | The same as the live app on that Mac; "Remove this Mac" plus S3 rotation |

## Build order and proof

1. `internal/devicekey`: the wrap, with its failure-mode tests first.
2. Auth ceremony `device_release` (begin, approve page, finish), email
   session refused.
3. Guest route and the `device_key_released` op, its table, the compat
   declaration, `GET /api/devices`.
4. The relay from auth to the guest.
5. The app: Secure Enclave key, release flow, record storage, open.

Proof (staging, no person): a test account with a pre-minted software
passkey (resource fabric: tests enroll with a passkey session) and a
software device key: release succeeds and the devices list shows it; the
wrap opens with the device key; an email session's begin is refused; an
assertion over another device's challenge is refused; a stopped computer
refuses with its stated cause. The Mac's Secure Enclave path is proved on
the owner's Mac when the owner is home.

Rollback: an auth switch `AUTH_DEVICE_RELEASE` off refuses begin; revert
removes the routes. A released key cannot be un-released; revocation is
S3's rotation.

## Open for review

- Whether auth should call the guest directly or through vmctl (the
  existing owner-request relay is the proxy; auth calling the guest is a new
  edge).
- Whether the approve page needs its own origin check beyond the existing
  WebAuthn RP id.
