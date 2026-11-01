# Releasing a computer's key to the owner's Mac (stage 2, S1)

Date: 2026-11-01. Status: design v2 for review (red; the build is tier 2).
v1 (c153b24f8) was sent back by Devin and DeepSeek; every finding is taken
here, and "Review v1" at the end maps them.
Goal file: `definitions/choir-mac-sync-stage2-2026-11-01.md`, slice S1.
End state: `design/resource-fabric-2026-10-31.md` (keys and passkeys row).

## The problem

The Mac holds a verified copy of its computer's tape and snapshot
(`internal/tapefollow`). The private parts are sealed under the computer's
private-artifact key (the DEK, `computerevent/privacy.go`), which exists in
the guest and, wrapped, in two-approval host escrow. To read offline, the
Mac needs the DEK. The release must be:

- **for the owner's own computer**: the computer is joined to the signed-in
  user by vmctl, and the guest checks the owner itself;
- **owner-approved for this device**: a user-verified passkey assertion
  whose challenge commits to the device key, the computer and a nonce;
- **started and finished by native code only**: the webview in the app
  cannot start, finish or redeem a release;
- **wrapped end to end**: the guest wraps the DEK to the device key; the
  host relays ciphertext;
- **recorded before the key leaves**, on the computer's tape, and visible;
- **never by an email session**.

What the DEK opens: sealed payload envelopes, sealed snapshot chunks
(`filecas.SealChunk(u.dek, ...)`, `snapshot/snapshot.go:435`), and the file
CAS digest key (`filecas.FileDigestKey`, `autoputer/run.go:488`). The reader
needs all three.

## Decisions

### D1. The device key

A P-256 key-agreement key in the Mac's Secure Enclave
(`SecureEnclave.P256.KeyAgreement.PrivateKey`), not exportable, not
synchronizable, access control `.privateKeyUsage` plus `.userPresence`
(Touch ID or the login password). The app asks once per launch and reuses
the authentication context (`LAContext.touchIDAuthenticationAllowableReuseDuration`)
for that session, so a stolen unlocked Mac cannot open the local copy after
a relaunch without the owner's presence. A Mac without a Secure Enclave
refuses release; no software fallback. Tests use a software P-256 key with
the same wrap.

Encodings, pinned: the device public key is SEC1 uncompressed (65 bytes),
carried as lowercase hex; the device id is lowercase hex SHA-256 of those
65 bytes. The server and the guest both reject anything else. The device
name is the Mac's computer name, trimmed, at most 64 characters, display
only, and every surface escapes it.

### D2. The wrap

`devicekey.SealDEK(devicePub, computerID, deviceID, keyVersionDigest, dek)`:
an ephemeral P-256 key; ECDH with the device key; HKDF-SHA256 with an
**empty salt** and info `choir/devicekey/v1` `0x00` computerID `0x00`
deviceID `0x00` keyVersionDigest; XChaCha20-Poly1305 over the DEK with the
same info as associated data. The record (ephemeral public key SEC1
uncompressed, nonce, ciphertext, key version digest, device id, computer id)
is not secret. A fixed test vector is checked in Go and in the app's Swift
CryptoKit code, so a mismatch fails in CI, not on the owner's Mac. Failure
modes tested first: wrong device, computer or version, any tampered field.

### D3. The ceremony, in auth

A new table `device_release_attempts` (migration; the existing
`challenge_state.type` check admits only registration and login): id
(32 random bytes, hex), user id, computer id, device pub, device id, name,
nonce, WebAuthn session data, state (`begun`, `approved`, `redeemed`),
created and expiry (five minutes), grant id.

1. **Begin** `POST /auth/device-release/begin`, native app only. Refused
   unless `AUTH_DEVICE_RELEASE` is on and the session's method (`am`) is
   passkey. Validates the encodings (D1) and the point (on curve, not
   identity). The computer is not checked here (auth has no ownership data);
   it is checked at redeem by vmctl and by the guest. The challenge is
   `SHA-256("choir/device-release/v1" 0x00 computerID 0x00 devicePub 0x00
   nonce)`, passed with `webauthn.WithChallenge` and
   `webauthn.WithUserVerification(protocol.VerificationRequired)`.
   `WithChallenge` is marked experimental in go-webauthn v0.16.4: the
   module is pinned and a ceremony test fails if a caller challenge is not
   what the authenticator signs.
2. **Approve**: the app opens `ASWebAuthenticationSession` at
   `https://choir.news/devices/approve?attempt=<id>`, served by auth on the
   RP origin. The page renders only stored attempt fields (computer, device
   name escaped, first 16 hex of the device id), with
   `frame-ancestors 'none'`, a strict CSP, and no query-derived content. At
   the same moment the app shows the same triple in a **native** dialog
   (AppKit, not the webview), so the owner compares two displays the server
   script cannot both paint.
3. **Finish** `POST /auth/device-release/finish` from that page: same-origin
   (`Origin` and `Sec-Fetch-Site` checked), the attempt must be `begun` and
   unexpired. Auth recomputes the challenge from the stored attempt, verifies
   the assertion with the stored session data (UV required), and moves the
   attempt to `approved` with one conditional update (`WHERE state='begun'`),
   so a concurrent double finish approves once. The browser gets only "done,
   return to the app"; no key, grant or record goes to the browser.
4. **Collect** `GET /auth/device-release/grant?attempt=<id>`, native app
   only, its own session, same user: when the attempt is `approved`, auth
   returns a grant: a hostauth-signed statement {grant id, user id, computer
   id, device pub, device id, name, attempt id, assertion digest, expiry
   two minutes}. Collecting does not consume; redeem does.

### D4. Redeem, through the proxy and vmctl, then the guest

`POST /api/computer/device-release` with the grant, native app only, its own
session. The proxy:

1. checks the grant's signature, expiry, and that its user is the session's
   user;
2. joins user to computer through vmctl (`LookupComputerContext`), refusing
   a computer that is not the user's, and asks vmctl whether it is running
   (a stopped computer: "start the computer and retry"; nothing is queued);
3. forwards to the guest's release route over the existing owner path, with
   `X-Authenticated-User`.

The guest:

1. refuses unless `AuthenticatedOwner(r)` equals its `CHOIR_OWNER_ID` and
   the computer id is its own;
2. refuses with "the computer's key is not loaded yet; retry in a moment"
   while booting;
3. validates the device pub and recomputes the device id;
4. if a release with this grant id is already recorded, returns the same
   wrap (redeem is idempotent, so a lost response retries without a new
   ceremony);
5. else appends a projection batch with one op `device_key_released` on
   table `device_key_releases` (grant id primary key, device id, device pub,
   name, key version digest, released_at from the op body, attempt id,
   assertion digest); the enclosing `projection_batch_recorded` event's
   `authority_ref` is `device-release:<grant id>`. The apply is an
   idempotent insert keyed by grant id, so replay and rebuild agree;
6. only after the append is durable, wraps the DEK (D2) and returns it.

The proxy marks the grant redeemed in auth only after the guest answers
with a wrap; until then it can be redeemed again within its two minutes.

### D5. Native-only routes in the app

The desktop proxy forwards renderer requests with the native cookie jar
(`cmd/desktop/desktop_auth.go:219-237`). `/auth/device-release/*` and
`/api/computer/device-release` join `blockedDesktopAuthRoutes`, so the
webview cannot reach them; the native code calls them with its own client.
The approve page opens in the system browser, never in the webview.

### D6. Compatibility: two releases, refuse class

There is no skip rule today: an unknown op fails replay
(`ProjectionBatch.Validate`, `store/project.go:116-117`), and SR7 (design
only) says key and security records are never skippable. So:

- **Release N** ships the reader: the op kind, its apply, the table, and a
  store epoch bump with the guest release floor raised, so no binary older
  than N runs a store that may hold the op. `AUTH_DEVICE_RELEASE` stays off
  everywhere in N (a correctness sequence, not an off-by-default switch).
- **Release N+1**, once N is the floor across the fleet, turns
  `AUTH_DEVICE_RELEASE` on for d@e, then the owner's computer.

The record is a projection op, not a canonical event kind: it is guest
state, and a new kind would need a reducer case on the host
(`computerevent/reducer.go:142-145`) for no gain. `key_rotated` and
`key_revoked` are the guest credential lifecycle and are not reused. This
deviates from the goal file's "new key-domain tape event"; the goal file is
updated with this design.

Rollback fence: after the first release, a revert must keep the reader
(op, apply, table), because the tape now holds the op; only the routes and
the app flow may be reverted.

### D7. The Mac side and the devices surface

The app stores the release record beside the local copy
(`~/Library/Application Support/Choir/<computer-id>/device-key.json`). To
open, it asks the Secure Enclave for the ECDH secret (user presence, once a
launch), derives the wrap key, and holds the DEK in memory only. Sign-out
keeps the device key. "Remove this Mac" deletes the Secure Enclave key and
the record.

The guest serves `device_key_releases` to its owner (`GET /api/devices`).
The app's menu lists every released device with its release time, from the
tape; the surface has a devices page. Until S3 both say a released Mac can
read everything sealed under the current key and that revocation needs S3.
While the computer is stopped the list is unavailable (S1 limit; a host-side
read of the tape is later).

### D8. Kill switch

`AUTH_DEVICE_RELEASE` off refuses begin, finish, collect, and the proxy's
redeem; registered in `switches/registry.yaml`. The guest route has no
switch of its own: it acts only on a redeemed grant, and the reader must stay.

## Threats

| Actor | Attempt | Stopped by |
|---|---|---|
| Another user with a passkey | Name your computer | vmctl's join at redeem and the guest's owner check |
| Script in the app's webview | Begin, finish, collect or redeem with its key | The routes are blocked in the desktop proxy (D5) |
| Script on the server's pages | Show a false device on the approve page | The native dialog shows the stored triple independently |
| Network attacker | Replay an assertion or a grant | Single-use attempt states; grants signed, short, idempotent by id |
| Email-session holder | Release | Begin refuses `am` other than passkey |
| Presence-only authenticator | Release without UV | UV required at begin and verified at finish |
| Compromised host | Release to its own key | Already reaches any DEK through key delivery (`platform/key_delivery.go`); named, not prevented |
| Compromised guest | Exfiltrate without a record | Holds the DEK anyway; the record detects honest guests only; named |
| Stolen unlocked Mac | Open the local copy | Secure Enclave key use needs user presence once a launch |

## Proof

Staging, no person: a test account with a pre-minted software passkey and a
software device key.

- Positive: release succeeds; the devices list shows it; the wrap opens with
  the device key and decrypts a sealed payload from the local copy.
- Refused: an email session's begin; another user's computer at redeem; an
  assertion over another device's challenge; a presence-only assertion; a
  concurrent double finish (one approval); a second redeem returns the same
  wrap; a stopped computer with its stated cause; a booting guest with its
  stated cause; the switch off at each of begin, finish, collect, redeem.
- In the app's own test: the webview's requests to the blocked routes are
  refused.
- The Go/Swift wrap vector in CI. The Secure Enclave and Touch ID path is
  proved on the owner's Mac when the owner is home.

## Build order

1. `internal/devicekey` and the test vector.
2. Release N: the op, its apply, the table, the store epoch and floor.
3. Auth: the attempts table, begin, approve page, finish, collect; the switch.
4. The proxy redeem and the guest route.
5. The app: Secure Enclave key, native dialog, blocked routes, open.
6. Release N+1: the switch on for d@e.

## Review v1, mapped

Devin 1 and DeepSeek 2 (ownership): D4. Devin 2 and DeepSeek 3 (compat): D6.
Devin 3 and DeepSeek 1 (renderer): D5 and the native dialog. DeepSeek 4
(UV): D3. DeepSeek 5 and Devin 6 (return path): collect and redeem by the
native app. DeepSeek 6 (attempt storage, atomic consume): D3 table and
conditional update. Devin 5 and DeepSeek 7 (authority_ref): D4 step 5.
DeepSeek 8 (encodings, salt, vector): D1, D2. DeepSeek 9 (stolen Mac): D1
user presence. DeepSeek 10 (citations): fixed. Devin 4 and DeepSeek 11
(deviation): D6 and the goal file. DeepSeek 12 (experimental API): D3.
DeepSeek 13 (proof negatives): Proof. DeepSeek 14 (kill switch): D8.
DeepSeek 15 (name): D1, D3 page. DeepSeek 16 (booting): D4 step 2. Devin 7
(idempotent apply, released_at): D4 step 5. Devin 8 (stopped computer list):
D7. Devin 9 (consume order): redeem marks after the guest answers. Devin 10
(begin validation, registry): D3, D8. Open questions: through the proxy and
vmctl (D4); the page is on the RP origin with framing and CSRF checks (D3).
