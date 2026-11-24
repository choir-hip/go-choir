# Exact: passkeys (`@exact/passkeys`) and the secret store, as Autoradio uses them

Part of [EXACT.md](../EXACT.md). Read against the pinned checkout `f53347503` (`mobile/exact.pin`). Marks as in [gaps.md](gaps.md): **[src]** read in Exact's source, **[doc]** in its docs,
**[run]** run here and seen, **[unrun]** not run by us (only a device shows it). Paths without a prefix are under `mobile/.exact-src/`. Written 2026-11-23 with the native-passkey change
(owner, 2026-11-23: "we should have native passkey account creation and sign in. no 'code'. this is a priority"; "by native, i mean exact").

## What Exact gives

- `@exact/passkeys` is a first-party package with WebAuthn's names: `credentials.create({publicKey})`, `credentials.get({publicKey, mediation?})`, `PublicKeyCredential.getClientCapabilities()` and three
  "signal" calls (`packages/passkeys/exact.json`) [src]. The options and the answers are WebAuthn's JSON forms (`PublicKeyCredentialCreationOptionsJSON`, `RegistrationResponseJSON`,
  `AuthenticationResponseJSON`), so the server's own options go in and the answer goes back to the server unchanged. A PRF output is withheld from the answer (`withhold`); only `prf.enabled` is answered.
- One grant, `auth.passkey <rp id>` (here `auth.passkey choir.news`). A call whose `rp.id` (create) or `rpId` (get) is not granted is refused with `SecurityError` (`exact.json` `grants`) [src].
- Apple (`packages/passkeys/apple/Passkeys.swift`) [src]: `ASAuthorizationPlatformPublicKeyCredentialProvider`; create takes `challenge`, `user.id`, `user.name`, `rp.id` and the `prf` and
  `largeBlob` extensions; get takes `rpId` and `challenge`. A get with **no `allowCredentials` is a discoverable request**: the system sheet lists the passkeys it holds for the site. The answer carries
  `userHandle` when the credential has one (`Passkeys.swift:486-487`). iOS 17.4 or newer (the checklist's floor); the app's `minimumOS` is 18.0.
- Origin: the package's manifest says what it reports as the client's origin: `"clientOrigin": {"ios": "https://{rpId}", "macos": "https://{rpId}"}` (`exact.json`, both calls) [src]. The clientDataJSON
  itself is written by AuthenticationServices (`rawClientDataJSON`, `Passkeys.swift:455,486`); Apple's own text for the origin value was not found (a developer-forum answer says "https://" plus the
  relying party id, which the package's manifest repeats). The package's checklist asks a device run to record it ("the origin it names is the one AuthenticationServices writes: record it", `packages/passkeys/test/checklist.md` step 2) [unrun].
  The server therefore accepts no new origin: `AUTH_RP_ORIGINS` is `https://choir.news`, which is exactly `https://` plus the relying party id.
- Errors are WebAuthn's DOMException names (`packages/passkeys/exact.json` `substitute.ceremony`): `NotAllowedError` for a cancel, **and also for "no credential" on get** (`choices.none` maps to the same
  Apple code `canceled`, so the app cannot tell a cancel from "this phone holds no passkey for the site"); `InvalidStateError` for an excluded credential on create (`matchedExcludedCredential`);
  `NotSupportedError` when `getClientCapabilities().available` is false; `SecurityError` for a grant mismatch. Autoradio words them in `mobile/AutoradioLab/passkey-ceremony.ts` (`ceremonyMessage`).
- What it needs outside the code [src, `docs/contract-for-agents.md:1862-1882`, `host/apple/presence.mjs:294-297`, `packages/passkeys/test/checklist.md`]:
  1. the Associated Domains entitlement `webcredentials:choir.news`, which the bake derives from the `auth.passkey` grant (a development build asks `webcredentials:choir.news?mode=developer`);
  2. `https://choir.news/.well-known/apple-app-site-association`, served as JSON, 200, no redirect, no sign-in, naming each app: `webcredentials.apps` = `S47GERGJZ3.news.choir.autoradio`,
     `...autoradio.lab.one` (the release app and the second development app; built: `internal/proxy/apple_app_site_association.go` serves `internal/appids`, the one list the finish routes also accept). The first development app, `news.choir.autoradio.lab`, is on a free team that cannot carry Associated Domains: it is on neither list, and the app says so (`account-state.ts`, `passkeySupport`);
  3. **a provisioning profile that carries Associated Domains**: "the team's wildcard profile cannot carry Associated Domains" (`checklist.md`, iPhone step 1): each App ID on the list needs the
     capability enabled in the Apple Developer account and a development (and later a distribution) profile that names it;
  4. on a development build, Settings > Developer > Associated Domains Development turned on, or the phone reads the file through Apple's CDN, which can lag a deploy by hours.
- The simulator drives no passkey sheet; `getClientCapabilities` there answers by the host's default. **The sheet is seen only on a device** [unrun].
- Android (`packages/passkeys/android/Passkeys.kt`) exists and takes `assetlinks.json` and the app's certificate fingerprints. **Not wired for Autoradio**: the server accepts no `android:apk-key-hash:`
  origin yet, so the app offers no sign-in on `exactPage().platform == 'android'` (`passkeySupport`) and says so; the shared sign-in code needs no other change once the origin is accepted [unrun].

## A fresh assertion for account deletion

Deletion uses the same `credentials.get` with options from `POST /auth/account/delete/begin` (sent with the app's key; the options name this account's credentials and require user verification, unlike sign-in's discoverable request). The answer goes unchanged to `/auth/account/delete/finish`. `account-delete.ts` owns the run; it never keeps the assertion and logs no body. A cancelled sheet leaves everything as it was; only a 200 from finish signs the phone out and wipes it. [unrun on a device]

## The secret store (where the app's key lives)

- A data module grants `secret.keep <name>` and uses `store.set(name, value)`, `store.get(name)` (a string or `null`) and `store.forget(name)` inside an answer (`docs/contract-for-agents.md:1533-1545`) [doc].
  A contract resource may send a secret as `{secret:name}` in a header, so a page never sees it (`contract-for-agents.md:1371`) [doc]; Autoradio's sends go through `api.ts` instead (one seam, with the
  computer id and the 401 handling).
- **Apple, read in the source** (`vendor/ibex/crates/ibex2/src/secrets/darwin_keychain.mm`, `secrets/mod.rs:409`) [src]: a generic-password item, `kSecAttrService` = the app's bundle identifier
  (`app_identity` = `Bundle.main.bundleIdentifier`), `kSecAttrAccount` = the secret's name, value = the bytes. On iOS the data-protection Keychain; `kSecAttrAccessible =
  kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` (set on add, line 217-218); no `kSecAttrSynchronizable`, so the item is neither iCloud-synced nor restored to another device from a backup. No access
  group unless the app has widgets (`ExactKeychainAccessGroup`), so the item is in the app's default group, which the app's own Swift code reads with a plain `SecItemCopyMatching` for the same service and account.
- Autoradio's three secrets: `choir.key` (the bearer key), `choir.key-id` (its server id, for the revoke at sign-out) and `choir.computer-id` (not a secret; the listener's computer, written by the voice
  code when it first discovers it). They are granted in `app.ts` (`secret.keep choir.key` and so on) and named in `app-signin.ts`; the Swift constants are `KeyAccount` in `ChoirClient.swift`.
- **Who reads the key.** Shared code (`api.ts`, through the store of the current answer) for every page. The voice code in Swift (`ChoirDesk`, `AutoradioDesk`, `ChoirSystem1`: the turn's server calls;
  the Texture editor; the notebook) calls the server itself and reads the same Keychain items through `KeychainKeyStore` (service = bundle id, the names above). That is why the Swift stays: the voice's
  calls cannot be handed a key by shared code per call without a rewrite of the turn engine, and the item is already where Swift can read it. Swift **writes** the key at no time any more.
- A read of a locked Keychain (before the first unlock after a restart) throws; `api.ts` answers `unreadable`, never "signed out" (`account-state.ts`).
- No presence policy (Face ID) on the key tonight (`secret.keep choir.key user-presence` would need `@exact/local-auth` and a lock the voice code cannot meet).
- Not done: Exact's store on Android keeps AES-GCM files wrapped by the Keystore [doc]; the Android app has no voice code, so nothing there reads the key but shared code.

## What is proved

- [run] `bun test` in `mobile/AutoradioLab` (the whole suite, with `@exact/passkeys` stubbed by `exact-stubs-preload.ts`), `contract build app.contract`, and `test web` (including `signin.test.contract`: the screen
  opens, the consent line and the links are there; a new account asks for the mailed code before the passkey screen). The web bundle built with the real package's web implementation.
- [run] The Go side with a software authenticator: creation, discoverable sign-in, replay, wrong origins, fixed app list, cap (`internal/auth/app_ceremony_test.go`).
- [unrun] On a device: the passkey sheet itself, the origin AuthenticationServices writes, Associated Domains with the explicit profile, the Keychain item shared between Exact's store and the voice code,
  the same passkey in Safari at choir.news.

## What the server's `app` field is, and is not

- `app` in the two finish bodies is a **label**, not an authorization: a bundle id from `internal/appids`, client-claimed, not bound to the request. All apps get the same full-scope key (`AllScopes()`, the device approval's grant, which includes `owner:acts`; `HandleCreateAPIKey` refuses to grant that scope from a key, the device approval does not). Do not "tighten" the label into a check: what proves the app is the passkey itself (the platform makes it only for an app the association file names).
- Residuals: platform passkeys report a sign counter of 0, so the counter gives no clone detection on the web or the app, and the server ignores `CloneWarning` as the web does. Cancelling during a create can leave an orphan passkey on the phone for choir.news (made, never finished). A register finish for an account that already has a passkey answers 400 when its pending row is gone, 409 (`ErrAddressTaken`) when the address gained an account meanwhile, and a passkey is added to an existing account only by the recovery flow.
- The app-login challenge table is in memory, 5 minutes, used once, at most 8 live per source (an IPv4 address or an IPv6 /64) and 20000 in all; at the ceiling the oldest challenge anywhere is dropped (O(1): the table is an expiry queue). It takes at least 2500 sources (20000 / 8) begun within five minutes to reach the ceiling; a source that shares a victim's address can drop the victim's challenge with eight begins. Either makes one finish answer "challenge not found" and the person taps again; nothing is replayed. A restart makes the app begin again.
- **An account is made only at `register/finish`, and only for an address proved in the same ceremony** (docs/problems/passkey-claims-an-unproved-address-2026-11-24.md). `POST /auth/register/mail {email, binding}` mails an 8-digit code (the existing email-attempt table, purpose `register`, no user row; ten minutes, five wrong tries, per-address and per-IP limits); `POST /auth/register/begin {email, attempt_id, code, binding}` spends it and returns creation options for a user id made then, holding one `pending_registrations` row (address, id, session data, five minutes) and no user; `register/finish` (web) and `app/register/finish` consume the pending row with one `DELETE ... RETURNING`, verify the passkey, and write the user (`email_verified_at` set), the credential and nothing else in one transaction. `binding` is a secret the client makes and keeps (the web keeps it in `sessionStorage`, the app in memory): a code works only for the client that asked, and the phone has no cookie jar to rely on. An address that already has an account is never registered again: `register/mail` for it mails the recovery link instead and answers the same, so there is no 409 oracle and no second holder. The only way a passkey is added to an existing account is recovery: `recovery/verify` proves nothing by itself, and at `register/finish` one transaction (`AddRecoveryCredential`) sets `email_verified_at` and, if that changed a row (an address never proved), first deletes the account's old passkeys, keys, sessions and grants. An account whose address is not proved signs in to nothing and mints nothing (`address_unproved`): enforced at the credential validators (the access-token validator in auth and the proxy, which both require the `ev` proof claim; `GetAPIKeyByHash`; `GetRefreshSessionByTokenHash`; the device and desktop grant redemptions), not in handlers, and tested over every route in the table. At start every key, session, grant and account-set row of such accounts is revoked once (the operator's bootstrap account is exempt). Recovery of such an account also clears its PRF owner root and account sets; recovery of a proved account takes nothing, and mail access alone suffices to take over a proved account. Not proved: the app's step on a device, and that the recovery mail reaches a phone user (the app does not open links; an address that already has an account recovers on choir.news). Measured in production (2026-11-24): 199 of 1,097 accounts unproved, 27 with a passkey, none owning a computer; those 27 sign in only after a recovery mail, with no fallback if mail is down.
