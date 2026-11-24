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
     `...autoradio.lab`, `...autoradio.lab.one` (built: `internal/proxy/apple_app_site_association.go`, one list in `internal/appids`);
  3. **a provisioning profile that carries Associated Domains**: "the team's wildcard profile cannot carry Associated Domains" (`checklist.md`, iPhone step 1): each of the three App IDs needs the
     capability enabled in the Apple Developer account and a development (and later a distribution) profile that names it;
  4. on a development build, Settings > Developer > Associated Domains Development turned on, or the phone reads the file through Apple's CDN, which can lag a deploy by hours.
- The simulator drives no passkey sheet; `getClientCapabilities` there answers by the host's default. **The sheet is seen only on a device** [unrun].
- Android (`packages/passkeys/android/Passkeys.kt`) exists and takes `assetlinks.json` and the app's certificate fingerprints. **Not wired for Autoradio**: the server accepts no `android:apk-key-hash:`
  origin yet, and the shared sign-in code runs there with no other change needed once it does [unrun].

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
  opens, the consent line and the links are there, no code). The web bundle built with the real package's web implementation.
- [run] The Go side with a software authenticator: creation, discoverable sign-in, replay, wrong origins, fixed app list, cap (`internal/auth/app_ceremony_test.go`).
- [unrun] On a device: the passkey sheet itself, the origin AuthenticationServices writes, Associated Domains with the explicit profile, the Keychain item shared between Exact's store and the voice code,
  the same passkey in Safari at choir.news.
