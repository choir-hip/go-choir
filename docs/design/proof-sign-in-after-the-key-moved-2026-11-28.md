# How the real-data proof signs in now that the key lives in shared code (plan, red: auth and session)

Standing: undecided | kind: wanted | settled-by: none yet; a plan for review | note: written by reading only on 2026-11-28 (origin/main 54add1a0c); nothing was built or run except `simctl list`; the layout of Exact's scratch secret store was read from source and never exercised, and whether the QA key may mint keys is not known

Problem record: [the real-data proof cannot sign in](../problems/the-real-data-proof-cannot-sign-in-since-the-key-moved-to-shared-code-2026-11-28.md).
Exact sources read (read-only, the lead's tree `.lane/trees/qa-mac/mobile/.exact-src`): `docs/contract-for-agents.md`, `docs/reference.md`, `host/apple/src/store.rs`, `picker.rs`, `store_secrets.rs`, `scripts/agent.mjs`, `scripts/agent-launch.mjs`, `vendor/ibex/crates/ibex2/src/secrets/`.

## The short answer

The record says the proof's key goes into a Keychain item nothing reads. That is only half the story, and the other half matters more.

The native Keychain item and Exact's secret item are the same item in a normal launch: service is the bundle id, account is `choir.key` (`AutoradioKit/Sources/AutoradioKit/Choir/ChoirClient.swift:80-82`; `ibex2 secrets/darwin_keychain.mm:54-58`). So `ProofKey` would be read by shared code in a normal launch.

But the proof does not do a normal launch. It runs `exact.sh agent ios`, and **in an agent drive Exact's secret store is not the Keychain.** It is memory, or a file per secret in the drive's scratch tree:

- `host/apple/src/store.rs:11-14`: "Agent mode gets memory stores unless `EXACT_STORE=real` says otherwise: a scripted drive starts from nothing and leaves nothing in a developer's keychain. A drive that names a scratch store keeps secrets in that tree instead."
- `store.rs:119-130` (`endow_for`): `memory = (agent && store != "real") || store == "memory"`, then the file store under the scratch tree.
- `docs/contract-for-agents.md:1745-1746`, `:264`, `docs/reference.md:3598-3611`: the same, and a native drive with no `--storage` uses the store named `default`.
- `scripts/exact-env.sh` runs Exact under `env -i` and passes a fixed list of variables. `EXACT_STORE` is not on it.

So since `5721a63cd` the proof's key was unreachable twice over: shared code stopped asking native, and the Keychain is not what shared code reads in a drive. A fix that only revives a native Keychain write could not work (section 4, option C).

The recommended fix is a script change only: before each drive, the proof writes the key and its id as two files into Exact's scratch secret store, using a disposable key it mints from the QA key. No app code changes. Details below.

## 1. How shared code holds the key today

**Names.** `app-signin.ts:31-33`: `KEY_NAME = 'choir.key'`, `KEY_ID_NAME = 'choir.key-id'`, `COMPUTER_ID_NAME = 'choir.computer-id'`. The grants are `secret.keep choir.key`, `secret.keep choir.key-id`, `secret.keep choir.computer-id` (`app.ts:59-62`). A secret name must match `[a-z0-9._-]{1,64}` (`darwin.rs:94`).

**Write.** A sign-in ends in `env.keep(key, keyId)` (`app-signin.ts:159`, `:193`, `:239`). `app.ts:129-133` binds that to `adoptKey(store, key, keyId)` (`account.ts:503`). `adoptKey` calls `keepKey` (`app-signin.ts:48-52`), then `resetSettled()` so the next answer reconciles again.

`keepKey` order: `store.set(choir.key)`, then `store.set(choir.key-id)`; if either throws, both are forgotten and the error goes up; then `store.forget(choir.computer-id)` (a failure there is ignored). The computer id is never written by sign-in. Shared code finds the computer from the server and keeps it in memory only (`api.ts:282-338`, `discovered`).

**Read.** Every read is through the store handle each answer is given (`app.ts:327-329`, `bindStore`; `api.ts:88-90`). `storeSource` (`api.ts:111-124`) answers three native-shaped ops from it:

- `keystate`: `present` if `choir.key` reads non-empty, else `absent`; a throw is `unreadable`.
- `session`: `{found, value: key, computerId}` (`api.ts:118`; consumed by `keptSession`, `api.ts:195-204`).
- `get` for the three accounts (`SECRET_OF`, `api.ts:84`).

`keyState()` (`api.ts:417-429`) adds the digest: SHA-256 of the key, lower hex (`api.ts:286-289`). There is no refresh. A 401 marks the key refused.

**The account stamp.** `STAMP_FILE = app:/data/account-stamp` (`account.ts:55`) holds the digest of the key whose data the shared stores hold. It is a key digest, not an account id. A new key for the same account is "another account" to it.

**The account generation.** `generation` in `api.ts:209-211` is process memory. It starts at 0 each launch, is announced to native as a boot (`api.ts:153-159`) and is bumped by sign-out and wipe. It is never written to disk. A seed has nothing to do with it.

**What "this install holds key K for account A" means, in order:**

1. Secret `choir.key` is exactly K: the bytes of `choir_sk_...`, no newline, no space. (The file store returns the file bytes as text, `secrets/mod.rs:304-315`; a trailing newline would go into `Authorization: Bearer`.)
2. Secret `choir.key-id` is the server's id for K. Not secret, but sign-out needs it: `heldKey` (`account.ts:276`) and the pending-revoke list (`account.ts:403-406`) use it.
3. `choir.computer-id` is absent.
4. `app:/data/signout-pending` (`PENDING_FILE`, `account.ts:53`) is absent.
5. `app:/data/account-stamp` is absent, or holds SHA-256(K).

**What `reconcile` does at the next launch** (`account.ts:151-158`, `:227-`): key `present`, marker none, stamp none: `adopt`, write the stamp, nothing wiped. Stamp equals: `nothing`. Stamp differs: `account-change`, which runs native `wipe`, wipes every shared store (WIPES, `account.ts:67-94`: onboarding, paper front, takes, files, mail and the rest), then writes the new stamp. Marker pending with the key present: the marker is removed, nothing wiped. Key unreadable: nothing. Key absent with a stamp or marker: finish the interrupted sign-out.

## 2. Where the secret lives in a simulator, and what the agent driver offers

**In a normal launch** (not a drive): Keychain generic password, service = the bundle id (`app_identity`, `secrets/mod.rs:409-419`), account = the secret name, value = the plain bytes, `AfterFirstUnlockThisDeviceOnly` on iOS (`darwin_keychain.mm:217-218`). An access group is added only for a provisioned build with `ExactKeychainAccessGroup` (`:68-72`). No wrapping or encryption. The id is per variant: the bundle id is the variant's app id (`app-identity.sh`), so `lab-one` and production do not share items.

The store is read into a snapshot before the runner boots and written after each commit (`store.rs:1-6`). A native module's init can run after that read (the comment at `ModelsEntry.swift:28-31` says so). That race is why a native import cannot be trusted on a first launch.

**In an agent drive** (`EXACT_AGENT` set, `EXACT_STORE` not `real`):

- Scratch tree = `<container>/Library/Caches/exact/<app id>/agent/<store name>/` (`picker.rs:40-54`, `:73-81`). With no `--storage` the name is `default` (`agent-launch.mjs:234-236`). The proof passes none.
- Secrets are files `<scratch>/secrets/<name>`, plain text, file 0600, directory 0700 (`secrets/mod.rs:140-160`, `store.rs:119-127`). `app:/data` is `<scratch>/data`, which is where the proof's `reset_state` already writes and deletes files (`proof-real.sh:35-38`).
- It is kept between drives until a drive passes `--reset-storage` (`reference.md:3601`). The proof passes neither flag.
- Under `--reset-storage` or a comparing drive, the tree is emptied at first boot (`picker.rs:86-116`).

**Can a simulator-only native routine write an item shared code reads as its own?** In a normal launch, yes: the same service and account (`ProofKey` did exactly that). In a drive, no: shared code never reads the Keychain there. It would need `EXACT_STORE=real` (below).

**Every route the driver offers (all read from the docs; none run):**

| Route | What it does for a secret | Source |
| --- | --- | --- |
| A file placed in the scratch secret tree before launch | Read at boot as the app's own secret. Works. Not an op: a script `cp`/`printf` into the container | `contract-for-agents.md:264`; `reference.md:3608-3611`; `store.rs:119-127`; `picker.rs:73-81` |
| `--storage <name>` / the `default` store | Names the tree. Kept between drives | `contract-for-agents.md:249-258`; `agent-launch.mjs:234` |
| `--reset-storage` | Empties secrets too, at boot | `reference.md:3601`; `agent.mjs:421-427` |
| `EXACT_STORE=real` in the launch environment | Drive uses the real Keychain (and UserDefaults for persisted state). The simctl launch forwards only this variable | `contract-for-agents.md:1746`; `reference.md:3009`; `devices.mjs:340-344`; `store.rs:55-60`. Blocked today: `exact-env.sh` runs `env -i` with a fixed list that lacks it |
| `EXACT_STORE=memory` | Memory stores | `store.rs:14-16` |
| `EXACT_AGENT_STORED` (authored `exact test` fixtures) | A `#oauth.<name>` entry seeds a plain secret, only with `EXACT_STORE=memory`. For authored tests, which have no real server | `data/src/storage.rs:134-160`; `agent-test.mjs:893` |
| `prefer <name> <value>` | Host facts only (colour scheme, visibility, online and so on). No secret | `contract-for-agents.md:7323`, `:7580-7601` |
| `prefer extension <name>=<json>` | Substitutes a package's answer. Only a native package that reads it; would be native code | `contract-for-agents.md:7601` |
| `call <tool> <json>` | A tool the app declares. `AutoradioLab/app.contract` declares none | `reference.md:2934` |
| `--login`, `--login-env`, `--home` | Launch entries and a child process's login shell. Not secrets | `reference.md:4952`; `agent.mjs:438` |
| Launch arguments, an environment variable of the app | Exact reads none for secrets. The app's own native hooks (`runFromLaunchArguments`) are staged out of store builds | `sync-exact-package.sh:9-13` |
| `state`, `logs`, `tree` | Read only. `state` masks secrets | `reference.md:2925` |

So there is **no driver op that sets a secret** and no `prefer` that injects one. The only no-code route is the file in the scratch tree.

Two Exact facts that bound any seam: the hosts keep a production bake out of agent mode (`reference.md:2880-2882`: "The hosts keep a production bake out of agent mode"), so a drive can only exist on a development build; and a line of `grants` that begins with `development` is left out of a production bake (`contract-for-agents.md:1727-1735`; archives set `EXACT_UPDATE_TRUST=production`).

## 3. Is there already a sign-in a script can use?

| Route | What it needs | Touches the server? | Fit |
| --- | --- | --- | --- |
| Reviewer sign-in. `reviewerRun` (`app-signin.ts:215-248`), UI: type a name with no `@` in the email field, then a password field and Sign in appear (`account.contract:100-118`). Server `POST /auth/app/demo/login` (`internal/auth/demo_account.go`) | A configured reviewer credential on staging. `docs/ship/demo-account.md` ends "Nothing here was run against Node B", so whether staging has one is **not known**. A password typed into drive steps lands in `exact.sh`'s argument list | Yes: mints a 30-day key with six `read:`/`write:` scopes on the **reviewer's** account | A different account and an empty computer, not d@e. Fine as a small second smoke of the real sign-in UI; not a real-data proof |
| Email code (`POST /auth/email/begin`, `/auth/email/verify`, `internal/auth/email_signin.go`) | A mailbox the test machine can read | Yes | Makes a browser session cookie of method `email_code`. Key minting, device approval and credentials need a passkey session (`requirePasskeySessionAny`, `email_signin.go:710-720`). It cannot make an app key |
| Passkey create or sign-in (`/auth/app/login/*`, `/auth/app/register/finish`) | The platform passkey screen | Yes | A script cannot approve it (`ProofKey.swift:3-4`) |
| Device flow (`/auth/device/begin`, `/approve`, `/token`, `device_login.go`) | A passkey session approves the user code | Yes | Same wall: approval is a passkey act |
| `POST /auth/api-keys` with a bearer key (`handlers.go:1941-2046`) | A key with `manage:keys` or `admin`. It may delegate only a subset of its own scopes, never `owner:acts`, and may set `expires_at` and a label | Yes, but the route exists and no server change is needed | **Usable.** Mints a disposable key and returns `secret` once and the key's `id` |
| `GET /auth/whoami` with a bearer key (`whoami.go`) | Any valid key | Reads | Answers `handle` (`writer-xxxxxx`), `key_id`, `scopes`, `reviewer`. Lets the script check which account it holds and what the key may do, without the email |

Whether the d@e QA key carries `manage:keys` or `admin` is **not known**: `~/.config/choir-qa/keys/` was not opened. The script finds out at run time from `whoami` `scopes`.

**A finding that changes the plan (from code, not run).** Sign-out is now shared code. `heldKey` (`account.ts:276`) reads the key and its id. `signOutInner` (`account.ts:380-`) revokes the held key on the server with `revokeDirect` (`account.ts:294-308`). With no id it asks `whoami` for one and then `DELETE /auth/api-keys/{id}` with that key. A key may delete itself without `manage:keys` (`docs/ship/demo-account.md`, first section). So the proof's sign-out drill (`proof-real.sh:529-545`), once the app is really signed in with the QA key, **revokes the QA key on staging**. The comment at `proof-real.sh:529` ("the proof key has no key id, so nothing is revoked on the server") was true of the old native sign-out and is false now. The next signed-in run would kill the key that the run after it needs. Any fix must use a disposable key.

## 4. Options

Notation: "real path" means the code a person's sign-in runs (`signInRun`, `keepKey`, `adoptKey`).

### A. Script seeds Exact's scratch secret store (files), with a disposable key. Recommended.

- **What and where.** Script only (`mobile/scripts/proof-real.sh`, `proof-common.sh`, a new small `proof-key.sh` helper, `test-proof-helpers.sh`). Once per run: read the QA key from its file, `GET /auth/whoami`, check `handle` and that `scopes` allow minting, `POST /auth/api-keys` with a label like `proof-<time>`, `expires_at` three hours out, no `computer_id`, scopes a subset of the QA key's. In every `reset_state`, write `choir.key` (the minted secret, no newline) and `choir.key-id` (the returned id) into `$CONT/Library/Caches/exact/<app id>/agent/default/secrets/` (directory 0700, files 0600), and remove `account-stamp` and `signout-pending` from `.../data/`. A trap revokes the disposable key and deletes the files. The `NOKEY` branch deletes the files instead of writing them.
- **Compile-out.** Nothing to compile out. No app, kit or package file changes. The proof of that is a diff gate: the change touches nothing under `mobile/AutoradioLab`, `mobile/AutoradioKit`, `mobile/exact-package`. `check-store-ipa.py`, `store-dev-names.txt` and `scripts/check-native-stores` are not involved and stay as they are.
- **D6.** Neutral. It writes where shared code already rests the key and adds no native reader or writer. If D6 answers "shared code only" (the design's recommendation), A is already right and `ProofKey` can be deleted later. If D6 answers "native holds it", A would be redone, but it is a script and cheap. D6 as written: "Where is the account key held on the phone? ... options: shared code's secret store is the only place it rests, and it is pushed once into native memory for the live voice turn (the design's recommendation: one path on both platforms); it stays as today, in two hands; the server issues short-lived keys first, which is server work" (`docs/definitions/choir-finish-the-move-to-exact-2026-11-27.md`, D6, default "as today").
- **Leak or misuse.** A plaintext key sits in the simulator container's Caches for the length of a run (the old design put one in Documents, which is backed up and file-shared; Caches is neither). A key left by a killed run: bounded by the trap and the three-hour server expiry. The minted key must never be on a command line (use `curl --config` or `-H @file`) or in output. A wrong key file could sign in the wrong account: pinned by the `whoami` handle check.
- **Real path or around it?** It goes around the sign-in action and `keepKey`. It uses the real read path (`storeSource`, `keyState`, `reconcile` adopt, every request) exactly as a relaunch after a real sign-in does. It does not prove `adoptKey` or the sign-in screen. Those have tests (`app-signin.test.ts`, `reviewer-signin.test.ts`) and a person's device check.

### B. A shared-code seam in development builds only

- **What and where.** Shared: a grant line `development fs.read app:/data/proof-key` (and the matching `fs.write` or remove), and a step before `settleAccount` in `app.ts`'s `answer` that, when `grants` still names that path, reads the file, deletes it and calls `adoptKey(store, key, keyId)`. A production bake leaves the `development` lines out (`contract-for-agents.md:1727-1735`), and the code can test its own `grants` constant to stay inert.
- **Compile-out.** Only half. The grant disappears from a store build, so a read is refused. The code and the path literal stay in the shared bundle. Exact has no compile-time condition for shared TypeScript that I found (`context.development` is for native packages, `reference.md:6959-6963`). `check-store-ipa.py` looks only inside Mach-O files, and where the shared bundle lives in the ipa is **not determined**. So the proof of compile-out would need a store archive (`store-negative-control.sh`), which only the lead can run. `proof-key` is already on `store-dev-names.txt` as `compiled`; moving it to shared TypeScript changes what that line means.
- **D6.** Neutral to consistent (it uses shared code's store).
- **Leak or misuse.** A new red path in the auth code of every build, gated by one grant. A plaintext key file in `app:/data` until read. A bug in the gate makes a key-injection door in a store build.
- **Real path?** Partly: it runs `adoptKey` and the stamp rules. It still skips the sign-in screen and the ceremony.
- **Verdict.** More code, more review, a weaker compile-out than A, for one extra property (`adoptKey` in a drive). Not worth it now. Revisit if the owner wants `adoptKey` proved on a simulator.

### C. Revive the native import: keep `ProofKey`, pass `EXACT_STORE=real`

- **What and where.** `exact-env.sh` and `exact-guard.sh` pass `EXACT_STORE=real`; the proof keeps `ProofKey.swift`, `ModelsEntry.swift:38` and `AutoradioModule.swift:129`. A warm-up launch is needed first, because the snapshot is read before the import (section 2).
- **Compile-out.** Existing: `#if targetEnvironment(simulator)`, names on `store-dev-names.txt`, `check-store-ipa.py`. Already proved by the gate.
- **D6.** Bad. It writes a Keychain item from native code that shared code reads: the "two hands" the design says does not hold up. `ProofKey` is on the survey's "blocked on D6" list (`docs/evidence/non-voice-native-code-2026-11-28.md:32,73`).
- **Other costs.** `EXACT_STORE=real` also moves persisted state to UserDefaults (`store.rs:55-66`) and leaves a real key in the simulator Keychain, which outlives a reinstall. It adds native logic outside voice, against the owner's rule, with no reason only native can meet. It needs a two-launch dance to beat the snapshot race. The key still ends in a native Keychain write the proof cannot revoke by itself.
- **Real path?** Around it, like A, with more moving parts.
- **Verdict.** No.

### D. Sign in by the real UI as the reviewer

- **What and where.** Drive steps only: `type` the name into `settings`/`signedout` sign-in, type the password, tap Sign in (`account.contract:100-118`). Needs a reviewer credential on staging (not known to exist). The password is in the drive's argument list.
- **Compile-out.** None needed; it is the shipping code.
- **D6.** Neutral. It uses the real `keepKey`.
- **Leak.** A shared reviewer password in `ps` output; a 30-day key for the reviewer account if the run dies before sign-out.
- **Real path?** The most real (`reviewerRun`, `keepKey`, `adoptKey`, reconcile). But it is the reviewer's empty computer, not d@e, so it proves sign-in and not real data.
- **Verdict.** A good second, small proof later. Not the answer to this problem.

### E. Mint nothing: seed the QA key itself (A without the disposable key)

Simplest, and wrong. The sign-out drill revokes the QA key (section 3). Rejected.

### Recommendation

**A.** Reasons: no new app code, so nothing to compile out and no store risk; it follows the owner's rule that nothing outside voice grows in native; it is neutral to D6; it uses Exact's own documented scratch store, so the app reads the key by the same code as any signed-in launch; and a disposable key makes the sign-out drill a real server-side revoke test instead of an attack on the QA key. D stays on the list as a later smoke. After one signed-in proof passes, a second landing can delete `ProofKey`, `ProofKeyTests`, the two call sites and update `store-dev-names.txt` and `store-build.test.ts:1348-1351` (they require the names on the list; that test then changes with the deletion). That deletion waits on D6 for the survey's row, so it is not in the first change.

What A depends on that was **not exercised**: that Exact reads a hand-placed file `secrets/choir.key` with no lease metadata (reads go straight to the backend, `data/src/secrets.rs:407-411`; the leases guard writes); that the app id in the Caches path is the variant's id; that nothing in the agent launch empties the tree without `--reset-storage`. The first run answers all three, and the assertion in section 5 says so within a minute.

## 5. The proof's own blindness

`proof-real.sh` has no check that the app signed in. `run_drive` (`proof-common.sh:57-121`) judges a drive only by the driver's exit code. All the `chk` and `bad` lines run at the end (`proof-real.sh:582-`), and `fail` stops the whole script at the first failed drive (`proof-common.sh:117`) unless `PROOF_KEEP_GOING=1`. That is why the 2026-11-28 run stopped at the edition picks drive and never ran a single assertion.

**Where to assert.** Straight after the warm-up drive, `proof-real.sh:139` (`run_drive warm light "" --touch platform`). The drive already opens `settings-nav-account` and dumps the tree (`:137`). Add, at once after it:

- `[settings-account-signed]` reads `"Yes"` in `$SHOTS/drive-warm.txt` (the same pattern the later check uses, `:783`, `:829`). If not, `fail "sign-in" "the app launched signed out: the key was not read from <path>; see api: session lines"` and print the `api: session found=... key=... computer=...` lines (`api.ts:200-201`) from the drive.
- `[settings-account-handle]` starts `writer-` and equals the handle `whoami` gave the script for the disposable key (not a hard-coded value).
- The console line `account: another account signed in` is absent (a stale stamp wiped the seed).

Add the same two checks after the survey drive, `:163`, which also dumps the account page (`:149`). The warm-up exists because a first launch after an install once showed signed out in 4 of 16 runs (`:134-135`). With a file placed before launch there is no snapshot race, so one miss is a real failure, not a flake. Allow the warm-up to be run once more (not more) and say so in the output.

**Checks that were silently unreachable or vacuous while signed out:**

- Never reached at all in the last run: every `chk` and `bad` of section 4 (`:582-844`), because the first failed drive stopped the script.
- Pass without ever having been signed in: `bad "drill: Files shows no rows after sign-out"` and `bad "drill: Mail shows no rows after sign-out"` (`:832-833`; `$AFTER` is the whole drive when the page never said "Yes"); the marker and stamp check (`:834`, nothing to leave); the whole signed-out tour (`:798-827`), which runs with `NOKEY` and cannot tell a signed-out launch from a signed-in-then-signed-out one; and the signed-out `bad` lines about no write actions.
- Fail correctly while signed out (so they were not blind, just unreached): `chk "drill: signed in before"` (`:829`), `chk "settings: the account shows a handle"`, `bad "paper: no sign-in card"`, the `signed in:` lines at `:783-797`.

So the plan adds a positive check for each vacuous one: the drill must have seen rows before (`files-row-/` or `mail-row-` in the "before" section) and `choir.key` must be gone from the scratch tree afterwards.

## 6. Failure list for option A, each with its test

Tests are written first. Shell tests go in `mobile/scripts/test-proof-helpers.sh` (stub `curl`, a fake container), run with `sh mobile/scripts/test-proof-helpers.sh`. "Proof" means a check inside `proof-real.sh` that runs on every proof.

| # | Way it goes wrong | Test |
| --- | --- | --- |
| F1 | The key file has a trailing newline or a space, so `Bearer` is wrong and the server answers 401: signed out | Shell: the helper writes `choir.key` with `printf %s`; the file's byte count equals the key's length and its first bytes are `choir_sk_`. Proof: the warm-up assertion |
| F2 | `choir.key-id` is missing or wrong: sign-out records no id or revokes the wrong key | Shell with stub `curl`: the id written is the `id` of the mint reply; mint reply without an id aborts before any file is written |
| F3 | The scratch path is wrong (variant app id, store name other than `default`, Exact layout moved): the app ignores the files and runs signed out | Shell: the path is built from `app-identity.sh id`, for production and for `lab-one`, under a fake container. Proof: the warm-up assertion names the path it wrote |
| F4 | A stale `account-stamp` from an earlier run with another key: `account-change` wipes every shared store and the seeded `onboarding.json` goes, so the drives run on the first-run page | Shell: `reset_state`'s helper removes `account-stamp` and `signout-pending` under every `agent/*/data`. Proof: the console shows no `account: another account signed in` in the warm-up |
| F5 | The drill revokes the QA master key | Shell: the seeded key is never equal to the master key (stub mint returns a different secret); an unmintable master key ends the script before any seed with a clear message. Proof (after the drill): `whoami` with the master key still answers 200 and the disposable key answers 401 |
| F6 | A killed or failed run leaves a live key and plaintext files | Shell: the trap helper deletes the files and sends `DELETE` for the minted id (stub `curl` records it). Server side: `expires_at` three hours out. Proof: the last line of a run says which key id it revoked |
| F7 | The script signs in the wrong account (a stray `REAL_KEY_FILE`, the owner's key) | Shell: a `whoami` handle that differs from the pinned test handle aborts before minting. The pin is committed in the script after the first good run (value not known today). Adversarial drills stay on d@e only |
| F8 | The proof passes signed out | Shell: the assertion helper over a canned signed-out tree fails with the sign-in message and over a canned signed-in tree passes. A negative control: run the helper on `drive-warm.txt` from a launch with the seed deleted |
| F9 | A seam ships in a store build | A has no app code: a diff gate (in `test-proof-helpers.sh` or `scripts/gates`) fails if the change set under `mobile/scripts/proof-*` also touches `AutoradioLab/`, `AutoradioKit/` or `exact-package/`. `check-store-ipa.py` is unchanged and still must pass on the next archive |
| F10 | The key reaches output, a log, a command line or the repo | Shell: run the helper with a fake `choir_sk_` key under `sh -x` capture and grep stdout, stderr and the stub `curl`'s argv for it; the existing leak check (`proof-real.sh:585`) is widened to the scratch tree's text files except `secrets/`. The key goes to `curl` by `--config` or `-H @file`, never argv |
| F11 | The mint is called once per drive (the retry path re-runs `reset_state`) and floods the key list | Shell: stub `curl` counts `POST /auth/api-keys` calls across three `reset_state` runs: exactly one |
| F12 | A drive passes `--reset-storage` or a comparing step, so Exact empties the tree and the key vanishes mid-run | Shell: grep the proof scripts for `reset-storage` and `compare` and fail on a hit. Proof: the per-drive assertion in section 5 |
| F13 | The signed-out tour (`NOKEY`) still finds the key because a failed drill never signed out | Shell: the `NOKEY` branch deletes `choir.key`, `choir.key-id`, `account-stamp`; proof: the tour's first screen asserts `signedout-banner` (already `:801`) and that the scratch `secrets/` has no files |
| F14 | The drill signs out but `choir.key` stays on disk | Proof, after the drill: no `choir.key` under the scratch tree (`store.forget` removes the file) and the disposable key answers 401 on `whoami` |
| F15 | The disposable key lacks a scope a page needs (a narrower key than the QA key), so a page that worked with the master key now shows "not allowed" | Mint with exactly the master key's own delegable scopes (the default when `scopes` is omitted, `handlers.go:1766-1781`); the first signed-in run is read against the old screenshots |
| F16 | The simulator keychain still holds an old native key from earlier runs, hiding a defect | Not used by A. A one-line step in `reset_state` deletes nothing there; recorded as a residual, not tested |

## 7. Two side findings

### (a) The signed-out banner "covers" a row. Refuted as an app defect, confirmed as a proof defect (from the contract, not run)

The banner is an overlay by design. `SignedOutBanner` is the first child of `bar-stack`, which is `position="absolute" left=0 right=0 bottom=0 z-index=20` (`bar.contract:136-138`; the card is `signedout.contract:16`). It sits over the bottom 64 or so points of every page while `bannerOn` is true (`account.contract:46`).

It is not a layout defect for a person, because every page body reserves the room: `PageScroll` ends with `BarClear` (`ui.contract:349-353`, `:371-382`), which is 214 points tall while the banner shows and 150 without. `EditEditionPage` is a `PageScroll` (`onboarding.contract:293`). At the end of the scroll the last row is clear. A row in the middle of a long page can sit under the card until the person scrolls, which is what every overlay does and what scrolling is for. The proof's own signed-out drive says the same: "the end of the paper shows its last line", "the card is the bar column's first child on every screen" (`proof-real.sh:813-820`).

So `ob-topic-open-Culture` was refused because the driver will not tap a covered view, and the drive did not scroll the row above the card first. The right fix is in the proof: before the tap at `proof-real.sh:364`, scroll the target clear of the banner with the existing `layout` plus `edit-edition-scroll drag` pattern used for Film on the same line, or run that drive signed in (no banner). With A in place the edition drive is signed in and the card is absent, so the problem goes away on its own. No contract change. If the owner wants the first screen of a page to never have a tappable row under the card, that is a new product rule, not a defect found here. Not run: whether Culture is in the bottom band on an iPhone 18 Pro is inferred.

### (b) `sim-device.sh` picks a runtime that cannot host the device type. Confirmed, with a fix

`sim-device.sh:11` takes `tail -1` of the text list of runtimes. On this Mac that is `iOS-27-1`. `simctl list -j runtimes` reports, for each runtime, `supportedDeviceTypes`. Read here: `iOS-27-0` lists 62 types including `iPhone-18-Pro`; `iOS-27-1` lists exactly 1 and not `iPhone-18-Pro`. `simctl create` then fails and `2>/dev/null` hides why (`:12`), so the script dies under `set -eu` with an empty runtime or udid and no message.

Fix: choose from the JSON, not the text.

```sh
runtime=$($SIMCTL list -j runtimes | /usr/bin/python3 -I -c '
import json, sys
dev = sys.argv[1]
rs = [r for r in json.load(sys.stdin)["runtimes"]
      if r.get("isAvailable") and r.get("platform", "iOS") == "iOS"
      and any(d["identifier"] == dev for d in r.get("supportedDeviceTypes", []))]
rs.sort(key=lambda r: tuple(int(p) for p in r["version"].split(".")))
print(rs[-1]["identifier"] if rs else "")
' com.apple.CoreSimulator.SimDeviceType.iPhone-18-Pro)
[ -n "$runtime" ] || { echo "sim-device: no available iOS runtime supports iPhone-18-Pro" >&2; exit 1; }
```

Compare versions as numbers, not text. Let `simctl create` print its error (drop `2>/dev/null`, or capture it and echo on failure). Test: a canned JSON with a newer runtime that lacks the device type picks the older one; one with none exits non-zero with the message. (A shell test over a stub `simctl` in `test-proof-helpers.sh`.) The type id is `com.apple.CoreSimulator.SimDeviceType.iPhone-18-Pro`, already in the script.

## 8. What I could not determine

- Whether the d@e QA key may mint keys (`manage:keys` or `admin`). The script learns it from `whoami`. If it cannot, option A needs a key that can, which is an owner act on the server, and the signed-in drill must not run with the master key.
- Whether Exact reads a hand-placed `secrets/choir.key` as the app's secret. Read from source, not run. The first run settles it.
- Whether staging has a reviewer credential (option D).
- Where the shared bundle sits in a store ipa, so whether option B could be proved compiled out without an archive.
- The pinned test handle for d@e (a public value, learned at the first good run).
- Whether the claim in the problem record, that no one ran the proof between 2026-11-23 and 2026-11-28, holds; unchanged.

## 9. Change class and rollback for the work this plans

The proof change is a script (yellow) that handles a bearer key and a server mint and revoke of a key on staging: treat it as red for review (auth and session; key custody), two seats, with the conjecture delta "a hand-placed Exact scratch secret is read as the app's own", protected surfaces "auth and session; account keys", evidence class "a signed-in proof run on staging", rollback "revert the script change; the QA key is untouched because the proof only ever holds a disposable key", heresy delta none (no native code added, one native reader of nothing). Expedite: none (scripts, no deploy). This document alone is green (rollback: revert).
