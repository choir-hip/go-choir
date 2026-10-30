# Provider key vault, host model policy and agentic model onboarding: design

Date: 2026-10-22. Status: proposal (FP0 design worker; not owner-ratified).
Mutation class of this document: green (docs only). The slices it plans
are classed one by one in section 6.

## The owner's request

Owner, 2026-10-22, verbatim in parts:

> "I just added a new api key to .env. SYNTHETIC_API_KEY. I want you to
> update the credential sending script so you send it to node-b. Actually,
> we need a better system. What's a secure way? I should have an admin
> panel I can use to add to a key vault. And it should also give me
> control over the host level model policy. I want to switch it to be
> 50/50 synthetic and gpt-6-luna with synthetic using DeepSeek v4.1 flash
> 1017 … each new provider and model needs to be tested. … the setup of a
> provider and model needs agentic care, not human manual config of some
> config strings."

And, relayed by FP0 later the same day:

> "the admin panel should let me input keys, including swapping keys, and
> also like for Open Code or ChatGPT, I should be able to put in multiple
> keys or multiple accounts. I don't know that we need to build all of
> this right now. We can just document the plan, but this way we wouldn't
> have to pass the keys with the script. And if I revoke a key and then
> have to provide a new one, I wouldn't need to use Claude or another
> agent."

So there are two kinds of work, and they get different treatment:

- **Key care is a pure owner act.** Adding, swapping, revoking and
  replacing a key, or adding a second account, is something the owner does
  in the panel alone. No agent, no script, no SSH, no laptop `.env`.
- **Onboarding a new provider or model is agentic.** An agent reads the
  provider's docs, runs a probe suite, writes a receipt and proposes a
  catalog entry and a staged rollout. The owner approves it with one act.

This is a plan. Section 6 names the smallest slice worth building first.
The owner answered this document's open questions on 2026-10-23 (UTC).
Section 9 records the rulings, and the sections they affect say so.

## Summary

1. Provider secrets move from a plaintext env file written over SSH into
   a **provider key vault**: one directory on Node B, readable only by the
   gateway. Each record is sealed with XChaCha20-Poly1305 under a key
   encryption key (KEK) that only the gateway receives and that the backup
   leaves out. This is the same construction as the FP9 sealed escrow key.
2. A provider holds a **pool of credentials**, not one. Each is a named
   entry with a label, fingerprint, state and health. The OpenCode key pool
   that exists today becomes the general mechanism, and ChatGPT gets
   several accounts the same way.
3. The owner types a key into a **host-served admin page on its own
   origin**. The page seals the value in the browser to the vault's intake
   public key, so the proxy, auth and logs only ever see ciphertext. Every
   vault write needs a fresh passkey assertion bound to that exact write.
   A value is never shown back.
4. A swap adds the new key, lets the gateway check it, puts it into
   service, then drains and retires the old one, with no gap. A revoke
   takes effect at once without a gateway restart.
5. The **host model policy** (catalog status, pools, weights, and route
   admission per computer class) becomes a versioned, hash-chained host
   document owned by the gateway. Today's Go literal becomes version 1.
   Rollback is a new version that copies an old one. Guests read the
   current version from the gateway, and the gateway enforces it.
6. **Per-computer pins stay computer-owned.** The paper's pin to
   gpt-6-luna is an owner-edited `System/model-policy.toml` on the paper.
   The code today makes that pin soft: on failover it falls back to the
   rest of the default pool. The design fixes that (owner ruling R2,
   2026-10-23: before launch, a pinned model fails rather than falls
   back), and adds a gateway-side backstop so a new route never reaches
   the paper or the owner's computers first.
7. **Onboarding is run by management on the owner's computer.** Research
   reads the provider's docs into citable captures. A host prober (trusted
   compiled code in the gateway, never a guest) runs the probe suite
   against the vault entry and writes a signed receipt. Management files
   a precommitment record and a staged rollout. The owner approves once.
   After that the host moves the model through the stages itself
   (catalog-only, test computer, small weight, target weight) and demotes
   it automatically on stated triggers.
8. The parallel `lane/fp0-synthetic-provider` is the first manual run of
   this flow. Section 4.6 maps each of its hand steps to the part that
   automates it.
9. First slice: harden the pins, put the vault at rest with env fallback,
   add an owner-only sealed write path, and start with the OpenCode pool.
   It replaces `nix/deploy-provider-creds.sh` gradually, with no flag day.

## 1. What exists today

All citations are at `origin/main` 308513aa3.

**Where secrets live.**
- The gateway reads provider keys from process environment variables,
  loaded from `/var/lib/go-choir/gateway-provider.env` through
  `EnvironmentFile=` (`nix/node-b.nix:2278`).
- ChatGPT uses a Codex OAuth file at `/var/lib/go-choir/codex-auth.json`.
  The gateway itself rewrites that file when it refreshes the token
  (`internal/provider/provider.go:3004-3033`).
- Both files are written from the owner's laptop by
  `nix/deploy-provider-creds.sh` over SSH. The script sources the repo
  `.env` (`:50-57`), assembles the full file, and sends it to `ssh` inside
  a heredoc (`:208-215`). That puts every value in the local `ssh`
  process's arguments. The upsert mode (`CHOIR_PROVIDER_ONLY_KEYS`,
  `:59-89`) sends values on stdin and is the safer path.
- Both files are in the off-host backup set
  (`scripts/off-host-backup.sh:235-236`). Restic encrypts the backup, but
  the provider keys still leave the host in a form that one backup secret
  opens.
- Every other unit hides both files with `InaccessiblePaths`
  (`nix/node-b.nix:538-553`). The gateway's own renames of
  `codex-auth.json` leave that file hiding open, as a named residual
  (`:543-545`). The unit-env directory was the fix for the same class of
  problem: hide a directory, not files (`:508-523`).

**How the gateway uses them.**
- Credentials are read once, when providers are constructed
  (`ResolveAll`, `internal/provider/provider.go:3157-3210`;
  `NewChatGPTProviderFromEnv`, `:863-874`). A new or revoked key needs a
  gateway restart (`internal/provider/opencode_keys.go:31-34`).
- A gateway restart drains for up to 11 minutes (`nix/node-b.nix`
  go-choir-gateway `TimeoutStopSec`), and a drain has taken staging down
  before (`docs/archive/problems/deploy-takes-staging-down-during-gateway-drain-2026-10-14.md`).
- OpenCode already has a multi-key pool. It reads `OPENCODE_API_KEY` and
  `OPENCODE_API_KEY_ALT_<n>`, parks a key on 401, 402 and 429, and keeps
  one key per session for the cache (`opencode_keys.go:17-24`, `:98-133`).
  Which keys serve is a separate policy, `GATEWAY_OPENCODE_KEYS`
  (`nix/node-b.nix:2310`). A primary key that answered 401 served a third
  of OpenCode calls for hours before it was parked
  (`docs/problems/opencode-401-key-never-parked-2026-10-22.md`).
- ChatGPT has exactly one account. When it hits its usage limit, every
  desk on it fails over or stops
  (`docs/problems/chatgpt-usage-limit-stops-research-and-management-2026-10-11.md`).

**Who can reach credential routes.** Operator routes on the gateway
(breakers, credential issue) require loopback, a marker header and the
host-internal credential (`isAuthorizedCredentialCaller`,
`internal/gateway/handlers.go:803-811`). The gateway listens on 0.0.0.0
for guests (`nix/node-b.nix:2293`). Caddy refuses `/provider/*` from the
public edge (`nix/node-b.nix:771-773`).

**Owner identity.**
- There is no platform-owner or admin principal on the host. "Owner"
  always means the owner of one computer. The `admin` API-key scope passes
  proxy scope checks but stays tied to its own user
  (`internal/auth/handlers.go:1557-1580`, `internal/proxy/handlers.go:572`).
- The closest things to an owner list are per-feature user-ID allowlists:
  `CORPUSD_AUTOPAPER_OWNERS` (`internal/platform/config.go:44-47`) and
  `PROXY_EXPAND_REQUESTERS` (`internal/proxy/config.go:84-100`).
- `internal/computertiers/computer-tiers.json` names the owner's account
  as a paid account, and the paper (`computer-1355b83b…`) as the one
  platform computer.
- The owner-act gate admits any cookie session, or an API key carrying
  `owner:acts` (`internal/proxy/owner_acts.go:25-39`). There is no
  step-up anywhere: no fresh passkey assertion is required for any act.
- Signed owner-act receipts exist. The proxy signs with an Ed25519 key,
  corpusd verifies, and only three act kinds exist, all self-development
  (`internal/owneract/owneract.go:34-84`; `ownerActMode = "observe"`,
  `nix/node-b.nix:117-129`).
- Access cookies are 5-minute Ed25519 JWTs, refresh tokens last 720 hours,
  and both are HttpOnly and SameSite=Lax
  (`internal/auth/handlers.go:296-355`; `internal/auth/config.go:84-113`).

**The browser surface.**
- For a signed-in user, the proxy serves the Desktop, apps and Settings
  from that user's guest computer, on the same origin as `/auth` and
  `/api` (`internal/proxy/computer_surface.go:12-30`).
- No Content-Security-Policy is set on the app shell (Caddy block at
  `nix/node-b.nix:750-785`; `frontend/index.html`).
- Settings says of itself: "Product settings only. Runtime prompt policy
  and agent internals are not exposed here."
  (`frontend/src/lib/SettingsApp.svelte:390`).
- The only model-policy UI is a read-only list in Texture's sources panel
  (`frontend/src/lib/TextureSourcePanel.svelte:28-32`, `:92-113`).
- No UI shows provider health, breakers or quota.

**Model policy.**
- The catalog and the pools are Go literals
  (`internal/modelcatalog/catalog.go:120-241`, `Pools()` at `:294-301`).
  The `Pools()` comment records seven owner changes in ten days, each of
  which needed a deploy (`:273-293`).
- The guest resolves pools from its own compiled copy
  (`internal/modelpolicy/model_policy.go:211-241`).
- The gateway validates (provider, model) against its own copy
  (`internal/gateway/handlers.go:586-592`). It serves the catalog and
  pools at `GET /provider/v1/models` (`internal/gateway/openai_compat.go:221-235`),
  but no guest reads that endpoint.
- Per-computer policy is the guest file `System/model-policy.toml`. It is
  rewritten to the platform default unless it carries `# owner-edited`
  (`model_policy.go:45-50`, `:550-566`).
- The management task has a separate hard pin to chatgpt/gpt-6-luna with
  no fallbacks (`model_policy.go:306-324`, `:156-160`).

**The paper's pin.**
- The owner asked at 21:4x on 2026-10-22 to "make chatgpt (gpt-6-luna)
  the sole model for the paper". FP0 did it as a computer-owned config
  write: the paper's `System/model-policy.toml` names chatgpt/gpt-6-luna
  for the defaults and every role, marked `# owner-edited`
  (`docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md:875`,
  and the 21:53 line that confirms traffic moved).
- This pin is **soft**. `Assign` gives a concrete model that is a member
  of the default pool the rest of that pool as fallbacks
  (`model_policy.go:225-231`), and a test asserts exactly that
  (`internal/modelpolicy/model_policy_test.go:248`). So on a failover-class
  error, the paper's threads move to opencode-go/deepseek-v4.1-flash
  today. If Synthetic joins the default pool, they will move to Synthetic.
  See section 3.4.

**Tonight's override.** FP0 is about to route the paper 50/50 between
Synthetic DeepSeek v4.1 Flash 1017 and ChatGPT gpt-6-luna overnight, on the
owner's order. That is the owner's explicit override of "the paper is
never first for a new route", for one observation run. The design keeps
the rule as the default and makes an override like this a recorded owner
act with an end time (section 3.4).

## 2. The provider key vault

The name "vault" is already used in this repo for the FP9 snapshot vault
(`internal/vmmanager`, `VM_SNAPSHOT_VAULT_DIR`). This one is the
**provider key vault**. Code and paths say `provider-vault` so the two
never collide.

### 2.1 What an entry is

An entry is one credential for one provider account:

- `name`: a stable identifier, such as `opencode/alt-0`,
  `chatgpt/account-2` or `synthetic/primary`.
- `provider`: a provider name from the catalog.
- `kind`: `api_key`, or `oauth_account` (a token set the gateway
  refreshes itself, like ChatGPT's).
- `label`: free text from the owner, such as "OpenCode Go, personal card".
- `fingerprint`: the first 12 hex characters of SHA-256 over the value,
  plus the value's last 4 characters, the way provider dashboards show
  keys. The panel computes both in the browser at entry time, so the owner
  can check them against the provider's dashboard or a local `shasum`.
  API keys are high-entropy, so this reveals nothing usable.
- `state`: `staged`, `serving`, `draining`, `retired` or `revoked`.
- `order`: its position in the provider's pool.
- `health`: when it was last used, and its counts of 2xx, 401, 402 and
  429 over the last hour and day, and any current park. The gateway
  updates this.
- `version`: increases on every change to the entry.

A provider's credential pool is its `serving` entries in order. That is
today's OpenCode pool (`opencode_keys.go:98-133`), with the env variable
names replaced by vault names and `GATEWAY_OPENCODE_KEYS` replaced by entry
state. The park rules stay the same: 401 and 402 park for hours, 429 for a
minute. ChatGPT gets the same pool. An `oauth_account` entry is parked
when its account reports a usage limit, and the park lasts until the reset
the provider states, or a conservative default when it states none. A
session keeps one healthy entry for as long as it stays healthy, so the
prompt cache is kept, exactly as OpenCode does today.

### 2.2 At rest on Node B

- One directory, `/var/lib/go-choir/provider-vault/`, mode 0700 and root
  owned. Every unit except the gateway hides it as a directory, by adding
  it to `hostSecretPaths` (`nix/node-b.nix:545-574`). Hiding the directory
  instead of single files closes the rename hole that `codex-auth.json`
  has today.
- One file per entry version, `<name>.<version>.sealed.json`. Each file
  holds the metadata in the clear and the value sealed with
  XChaCha20-Poly1305. The associated data is the purpose string
  `choir/provider-vault/v1`, the name and the version, so a record cannot
  be swapped to another name or rolled back to an old value unnoticed.
  This is the construction in `internal/keyescrow/sealed.go:1-15`. Reuse
  that code; do not write a new seal.
- One KEK, `provider-vault-kek`. It is a systemd credential delivered only
  to the gateway (`LoadCredential=`). Its source file lives in the hidden
  secrets directory. The backup excludes it by name, the same way it
  excludes `key-escrow-kek*` (`scripts/off-host-backup.sh:188-228`; add
  `provider-vault-kek*` and add the gateway to `KEK_UNITS`).
- An intake key pair (X25519). The private half is sealed under the KEK
  like any other entry. The public half is served to the panel (2.4).
- An append-only audit log, `audit.jsonl`, hash-chained: each line carries
  the SHA-256 of the line before it (2.6).
- The backup keeps the sealed records and the audit log, and leaves out
  the KEK. So a copy of the backup, or of the vault directory, does not
  yield a key.

What this does not protect, stated plainly as FP9 did for the escrow key
(`docs/archive/definitions/choir-launch-fp9-keys-recovery-2026-10-21.md`, FP9-C2):
a root read of Node B's filesystem gets the KEK and the records together.
Owner ruling R5 (2026-10-23): for now the KEK is only kept out of
backups. A TPM (`LoadCredentialEncrypted=`) or a network unlock after boot
is deferred, together with the same open FP9 decision, rather than opened
as a second one.

### 2.3 Who can read a value

- **Only the gateway process**, and only in memory, at the moment it
  builds an upstream request.
- **No API returns a value.** That covers the panel, the owner, agents and
  the gateway's own health output. Reads return metadata only.
- **Guests never see values.** They still call the gateway with their
  bearer. The vault changes nothing on the guest side
  (`internal/gatewayruntime/provider.go:1-3`).
- **Values never reach logs.** Entries are always logged by name, which is
  already the rule for OpenCode keys (`opencode_keys.go:24`). The redactor
  for rejected-request bodies (`internal/provider/rejected_request.go:12`)
  stays as a second line of defence.
- The gateway runs with `NoNewPrivileges` and the common hardening
  (`nix/node-b.nix:470-506`), and it spawns no children. A value therefore
  lives only in the gateway's heap and in the environment that the
  env-fallback period still loads (2.8). A root process on the host can
  read that memory. That is the same boundary as the KEK above.

### 2.4 How the owner adds a key

1. The owner opens the admin page (section 5) and signs in with a
   passkey.
2. The owner picks a provider and pastes the key. The page computes the
   fingerprint, then seals the value to the vault's intake public key.
   The construction is the custodian wrap in `internal/keyescrow`
   (X25519 ECDH, HKDF-SHA256, XChaCha20-Poly1305; `keyescrow.go:1-10`,
   `:128`), widened from a fixed 32-byte DEK to a bounded byte string.
   The plaintext lives only in that page's memory, for that moment.
3. The page asks for a passkey assertion. The challenge covers the act,
   the entry name, the digest of the sealed blob and a nonce (2.5).
4. The proxy checks the session and the step-up and forwards the sealed
   blob to the gateway's operator route. The route is loopback only and
   needs the host-internal credential, like the breaker reset today
   (`handlers.go:1157-1184`). The proxy never sees plaintext.
5. The gateway opens the blob with the intake key, re-seals it under the
   KEK as a new `staged` entry version, and appends an audit line.
6. The gateway checks the key against the provider with one cheap
   authenticated call: list models, or a one-token completion. It records
   the status. A key that answers 401 or 403 stays `staged` with that
   result shown, and never serves.
7. If the check passed, the owner's single act also asked for "serve when
   checked", so the entry becomes `serving` at the order the owner chose.
   The provider's credential pool is rebuilt in place, with no gateway
   restart (2.7).

### 2.5 Step-up for vault writes

Any vault write needs a fresh WebAuthn assertion, made for that write
alone:

- The challenge is SHA-256 over the canonical JSON of
  `{act, entry, blob_digest | policy_diff_digest, nonce, not_after}`,
  with `not_after` no more than 60 seconds out.
- User verification is required. Today the go-webauthn default applies,
  because no `UserVerification` is set anywhere (`internal/auth/handlers.go:487-565`,
  `:603-681`).
- Auth verifies the assertion against the owner's registered credential.
  It then mints a single-use owner-act receipt with the existing signer:
  issuer `proxy`, a new audience `gateway`, and a new act kind such as
  `vault.entry.add` (`internal/owneract/owneract.go:128-153`). The
  receipt's subject is that digest.
- The gateway verifies the receipt (`owneract.Verifier.Verify`,
  `:235-298`), refuses a nonce it has already seen, and refuses a subject
  that does not match the blob it received.
- A cookie alone is not enough. An API key is never enough to add or
  replace a value, even one with `owner:acts`.
- The one exception goes in the safe direction. A key with `owner:acts`
  may **pause** a credential or a model, which is a fail-closed act that
  needs no secret. A demotion an agent or the host makes for cause (4.5)
  therefore never waits on the owner, and re-enabling it is a step-up act.

**Who is "the owner" here?** No platform-owner principal exists (section
1). Owner ruling R1 (2026-10-23): vault and host-policy acts are the
owner's alone. They are admitted by a host allowlist holding one user ID,
the owner's account (`5bd6de97…`): `CHOIR_PLATFORM_OWNERS`, set in
`nix/node-b.nix` by a reviewed commit, as `CORPUSD_AUTOPAPER_OWNERS` is
today, and read by the proxy, auth and the gateway. There is no second
seat.

### 2.6 Swap, revoke and replace

These are pure owner acts. No agent and no script takes part.

- **Swap (rotation with no gap).** Add the new key as in 2.4. Once it is
  `serving`, mark the old entry `draining`. A draining entry takes no new
  sessions. Sessions that hold it move to the next healthy entry on their
  next call. After a short window, or when nothing holds it, it becomes
  `retired`. The owner takes one act, "replace A with B", which is add,
  check, serve B and then drain A, with one step-up.
- **Revoke.** One step-up act. The entry becomes `revoked` at once, the
  pool is rebuilt in place, and the next call on any session holding it
  draws another entry. Revoke is for a leaked key. The owner should also
  revoke it at the provider, and the panel says so in one line.
- **Revoke, then replace.** These are two acts with nothing between them.
  The provider runs on its remaining entries in the meantime. If none
  remain, the model breaker opens and threads fail over to the next pool
  member, as today (`docs/model-pools-design-2026-10-12.md` §5).
- **Several accounts.** Add another entry for the same provider. The pool
  order decides which is drawn first. Weights inside a provider's
  credential pool are not needed now: first healthy entry wins, as for
  OpenCode today.
- **ChatGPT accounts.** An `oauth_account` entry is added in one of two
  ways:
  - *Device login, run by the gateway (preferred).* The gateway starts the
    OAuth device-authorization flow with the Codex client ID
    (`provider.go:2804-2810`). The panel shows the verification URL and
    the user code. The owner approves on any device. The gateway polls for
    the token set and seals it straight into the vault. No token ever
    passes through the browser. UNVERIFIED: that this client ID supports
    the device grant for this use. The onboarding probe must check it
    before the panel offers the option.
  - *Paste a Codex `auth.json`.* The page seals the pasted file as in 2.4.
    This works today, but it reuses a login session the laptop also
    holds. Hypothesis, not observed: refresh-token rotation means the
    laptop and the gateway can sign each other out. Device login gives the
    gateway its own session and avoids that.
  - In both cases the gateway writes back its own refreshed tokens. That
    is a vault write by the gateway alone. Each one adds an audit line
    with the reason `oauth_refresh` and needs no owner act.
  - **Usage scope** (owner ruling R4, 2026-10-23: "Fine for my usage. Not
    for production usage to customers"). Every `oauth_account` entry
    carries `usage_scope: owner_only`. The gateway draws an `owner_only`
    entry only for a call from one of the owner's own computers, which it
    knows from the bearer (`handlers.go:384`). It never draws one for
    customer computers, and never for the production traffic customers
    see. When no other entry exists, a call from any other computer finds
    the provider's pool empty for it, and the call fails. Several ChatGPT
    accounts in one pool are therefore for the owner's usage only. Whether
    the paper's pre-launch traffic counts as the owner's usage is not yet
    ruled (section 9).

### 2.7 Hot reload

Providers read credentials at construction (section 1). The change is to
pass each provider a credential source, `CredentialPool(provider)`, that
it asks on each attempt. That is how `openCodeKeyPool` already draws a key
per attempt (`provider.go:1173`). A vault change rebuilds that provider's
pool atomically, so calls in flight keep the entry they drew. This
generalizes the code that exists rather than adding a parallel path. The
OpenCode pool is the replacement already in the tree.

### 2.8 Replacing the script and the env file, with no flag day

1. **Vault first, env second, per name.** The gateway looks a credential
   up in the vault, and falls back to its process environment under a
   fixed mapping (`OPENCODE_API_KEY_ALT_0` to `opencode/alt-0`, and so
   on). Gateway `/health` gains a `credentials_source` field listing each
   name with `vault` or `env`, and no value.
2. **Import from the panel, no SSH.** One step-up act, "import
   environment credentials". The gateway seals the values it already
   holds in memory into vault entries with the same names. The ChatGPT
   auth file is imported the same way. Nothing leaves the host.
3. **Watch.** When `/health` shows every serving credential sourced from
   the vault, the env fallback is dead weight.
4. **Remove.** A reviewed commit drops `EnvironmentFile=` from the gateway
   unit (`nix/node-b.nix:2278`) and the two files from the backup set
   (`scripts/off-host-backup.sh:235-236`). The script is deleted, and in
   the same commit every citer is repaired:
   `docs/model-pools-design-2026-10-12.md` (its Node B residual), the
   `docs/learn/03-gateway-and-providers.md` pages, and the comments at
   `nix/node-b.nix:2252-2276`. An owner act shreds the old files.
5. **Until slice 1 lands**, `SYNTHETIC_API_KEY` reaches Node B through the
   script's upsert mode, which sends values on stdin and touches nothing
   else: `CHOIR_PROVIDER_ONLY_KEYS="SYNTHETIC_API_KEY" ./nix/deploy-provider-creds.sh`.
   It is an owner host step over SSH, which is break-glass, not product.
   Do not use the full-regenerate mode, which puts values in `ssh`
   arguments (`:208`).

### 2.9 Threat model

| Threat | What it gets today | What it gets with this design |
| --- | --- | --- |
| A compromised guest computer | No keys: the gateway holds them. It can spend through its bearer, bounded by rate limits and bearer binding (`GATEWAY_BEARER_BINDING=enforce`). | The same. The vault adds no guest-facing route. Vault and policy writes sit on loopback operator routes the guest cannot reach (`handlers.go:803-811`). |
| A guest-served page running script on the owner's origin (section 1: the Desktop is served from the guest, on the same origin) | Any owner route while the 5-minute cookie is valid. | Not the vault. The admin page is host-served on its own origin with its own host-only cookie (5.1), so a guest page cannot ride that cookie. Writes need a passkey assertion the script cannot produce without the owner's touch. |
| XSS inside the admin page itself | n/a | It could read a value as the owner types it. Mitigations: a strict CSP (`default-src 'self'`, no inline script, `frame-ancestors 'none'`); no `{@html}`; a page small enough to review; values sealed at once and the input cleared; nothing renders provider-controlled text unescaped. The residual is accepted: a key typed into a compromised page is exposed. |
| A stolen owner session (cookie or refresh token) | Every owner act. | Reads of metadata, and pauses. Not add, replace, rollback or approve, which all need a fresh assertion. |
| A stolen API key with `owner:acts` | Owner acts on that computer. | Pauses only. |
| A compromised proxy or auth process | Every session. | Sealed blobs only. It cannot open them, and it cannot forge a gateway-audience receipt without the owner-act key. The proxy holds that key, so a compromised proxy could mint one. Hypothesis to close: have auth sign step-up receipts with its own key, so neither process alone can authorize a write. |
| Gateway memory or root on Node B | Every key. | Every key. The boundary is stated, not hidden. TPM sealing is the open FP9 decision. |
| Backups | Plaintext provider keys inside restic (`off-host-backup.sh:235-236`). | Sealed records without the KEK. |
| Logs | Values never logged by design, but the full-regenerate script mode puts them in `ssh` arguments. | Values never logged. No script. |
| A malicious or confused agent | Could ask the owner to run the script. | Cannot add, read or replace a key. Can propose (section 4), and can pause for cause. |

### 2.10 The audit trail

`audit.jsonl` gets one line per act. Each line records:

- the time and the act;
- the entry name and version, and the fingerprint (never the value);
- the actor: the user ID and auth method, the passkey credential ID and
  the receipt nonce, or `gateway` for an OAuth refresh or a check;
- the result;
- `prev_sha256`.

The panel shows the log, and the gateway verifies the chain at boot. A
broken chain is reported on `/health` and changes nothing else. The log is
the vault's only audit authority. It is not mirrored into a second store
(standing question 5).

## 3. The host model policy as durable state

### 3.1 The document

A `HostModelPolicy` version is one JSON document:

- `version` and `parent`, plus `sha256` over canonical JSON;
- `catalog`: one entry per (provider, model), carrying today's `ModelInfo`
  fields (`catalog.go:14-47`) plus `status` and `stage` (section 4);
- `pools`: name to members (provider, model, weight, `fallback_only`), as
  today (`catalog.go:249-271`);
- `admission`: for each computer class, the stages a model must have
  reached before that class may be routed to it. The classes are
  `test`, `ordinary`, `owner` and `platform`, taken from
  `computertiers` and the owner's user ID;
- `overrides`: owner acts with an end time, for example tonight's
  "the paper may use synthetic/… until 2026-10-23T08:00Z";
- `made_by`: an owner-act receipt reference, or `host:auto-demote` with
  its trigger evidence;
- `reason`: one line of text.

It is stored in the gateway's `/var/lib/go-choir/model-policy/`. Each
version is a content-addressed file, and a `current` pointer is replaced by
rename. The directory has its own hash-chained audit log. Today's Go
literal is compiled in as **version 1**. It is also the fallback when the
directory is empty or unreadable, so a fresh host works as it does now.

**Why not the control database?** (Written against Store A; Store A is now
the PostgreSQL control database.) The gateway has no database dependency
today, and it serves every inference call. Putting the policy behind the
control database would make every model call fate-share with the platform
store (standing question 7).
A small file document read at boot and on change keeps the gateway's read
path as it is. If later work needs the policy queried with other platform
state, the control database can take a projection of this document; it must not become
a second authority.

### 3.2 Distribution and enforcement

- **Guests read it.** `EnrichMetadata` (`model_policy.go:139-183`) resolves
  pools from the gateway's current version instead of the compiled copy.
  It fetches `GET /provider/v1/models`, which gains `version` and an ETag,
  caches it for about 60 seconds, and keeps the last good copy on disk.
  If the gateway is unreachable, the last good copy wins, then the
  compiled version 1. Run metadata gains `llm_policy_version` beside
  `llm_spec` and `llm_assignment`, so a trace shows which version chose
  the model.
- **The gateway enforces it.** The catalog check at `handlers.go:586-592`
  becomes "is (provider, model) at a stage this caller's computer class
  admits, at the current version, or at the version before it within a
  short grace window". The grace window means a policy change does not
  fail runs already in flight. A refused call is the existing
  failover-class "unsupported model", so the thread moves to its next
  member. The gateway knows the caller's computer from the bearer
  (`handlers.go:384`). This check is the backstop: it holds even when a
  guest file or a guest bug names a route the class does not admit.
- **Changing weights moves threads.** Hash assignment is proportional to
  weight (`model_policy.go:252-271`), so a weight change moves some
  threads to another member, and those threads lose their cache once. The
  panel says so next to the weight control.

### 3.3 Versioning, audit and rollback

- Every change is a new version: owner edits, stage advances and
  auto-demotions alike.
- A **rollback** is a new version whose content copies an older one, with
  `reason: rollback to vN`. The pointer history is never rewound. This is
  the same rule as product restore: a forward transaction.
- An owner edit is a step-up act whose challenge covers the digest of the
  diff (2.5).
- The panel shows each version's diff from its parent, who made it, and
  why.

### 3.4 Per-computer pins, and the paper

Computer-owned policy stays computer-owned
(`docs/agent-product-doctrine.md`, Runtime Configuration). The host policy
never writes a guest file. Two changes make pins mean what they say:

1. **A concrete pin gets no pool fallbacks unless it asks for them.**
   Today a concrete model in the default pool falls back to the rest of
   the pool (`model_policy.go:225-231`; test at `model_policy_test.go:248`).
   That turns the paper's luna pin into "luna, then deepseek", and after
   Synthetic joins, "luna, then synthetic". The fix: a role that names a
   provider and model gets the fallbacks the file names
   (`fallback = "pool:default"` or a list), and none by default. That is
   already the rule for the management-task pin (`:156-160`). The cost:
   a luna outage fails the paper's turns instead of moving them. Owner
   ruling R2 (2026-10-23) accepts that cost before launch: "I'd rather
   them fail than fallback. We are still pre-launch." Problem record:
   `docs/problems/paper-luna-pin-inherits-pool-fallbacks-2026-10-22.md`.
2. **The host admits new routes to the paper and the owner's computers
   last.** `admission.platform` and `admission.owner` require the
   `target` stage, held for a soak period. Even if a guest file names
   `pool:default` while Synthetic is still at `small_weight`, the gateway
   refuses the paper's call to Synthetic, and the thread goes to its next
   member. An owner override (3.1) lifts this for one computer until its
   end time. Tonight's overnight 50/50 on the paper is exactly that kind
   of override. Today FP0 writes it as a guest file edit. Under this
   design it would be an override entry that expires on its own.

The paper therefore stays pinned to gpt-6-luna in two independent ways:
its own file, and the host's admission rule.

**Tonight's run under R2 and R3.** The owner's words for the overnight
paper run: "Just ChatGPT luna and synthetic DeepSeek 1017 tonight. I'd
rather them fail than fallback" and "Don't use Opencode as fallback
tonight." The code does not do that by default. A thread that names a
pool gets the pool's other members as fallbacks (`Assign`,
`model_policy.go:234-240`). So a paper file naming a two-member pool of
luna and Synthetic would move a failed Synthetic thread to luna, and a
failed luna thread to Synthetic. That is a fallback the owner ruled out.
A file that names the concrete models gets no pool fallbacks only after
the 1a change. Until 1a lands, a concrete pin in today's default pool
falls back to opencode-go, which R3 forbids. FP0 should check
`llm_fallbacks` in the paper's run metadata (`model_policy.go:164-166`):
empty is the ruling, and a non-empty value is a breach to record.

### 3.5 The owner's 50/50, as data

The owner's target is this version of the default pool:

```
default = [
  synthetic/<deepseek-v4.1-flash-1017 id from Synthetic's docs>  weight 1
  chatgpt/gpt-6-luna                                              weight 1
]
```

OpenCode is not in it, not even as `fallback_only` (owner ruling R3,
2026-10-23). Before launch, a pool member's failure ends the thread's
turn with the error instead of moving to the other member. This reads R2
as covering this pair, which is the pair it named. In
the policy document that is a per-pool `failover: none` setting, which
the 1a change also makes possible for pools.

This is a stage target, reached through section 4's rollout. It is not
written on day one.

## 4. Agentic provider and model onboarding

### 4.1 Who does what

The work runs on the owner's computer, through the desks (doctrine `I26`):

- **Texture** takes the owner's ask ("add Synthetic with DeepSeek 1017,
  50/50 with luna") and sends management a semantic act. Afterwards it
  renders the onboarding's commitments in the owner's document.
- **Management** owns the onboarding as a commitment. It casts research,
  files the precommitment record, asks the host for the probe run, drafts
  the policy proposal, and escalates the one owner decision. It holds no
  provider routing authority. It proposes, and the host acts.
- **Research** reads the provider's docs with `choir.Read`, which makes
  citable captures (`internal/yaegikernel/surface.go:78`). It reports a
  provider spec: endpoint and wire shape; auth header; model IDs; the
  reasoning parameter and how reasoning comes back; tool-call format;
  streaming events; caching fields; context and output limits; rate-limit
  headers and the 429 body; pricing. Every field cites a capture block.
  Fields the docs do not settle are marked `unknown`, and the probe
  settles them.
- **Engineering** has no part when the provider fits an existing adapter.
  When it needs new adapter code, that is a platform source change in
  `internal/provider`. It goes through the repo's landing loop, done by a
  coding agent as the Synthetic lane is now. It is not a capsule effect
  on a computer, because adapters are platform-owned
  (`docs/agent-product-doctrine.md`, Runtime Configuration). A generic
  OpenAI-compatible adapter whose quirks are catalog data (wire shape,
  reasoning field, replay rule, auth header) would make most new providers
  data-only. That is slice 5.
- **The host prober** is trusted compiled code in the gateway. It runs the
  probe suite with the vault entry, against a catalog entry at stage
  `probe` that no pool contains. It writes a probe receipt signed with the
  host key into the model-policy directory. Guests never hold the
  credential, and a desk cannot narrate a pass the host did not observe
  (standing question 6).
- **The owner** adds the key in the panel (2.4), which is the only human
  input of a value, and approves the rollout plan with one step-up act.

### 4.2 The flow

1. The owner asks in Texture. Texture sends a semantic act to management.
2. Management casts research, and research returns the provider spec with
   citations.
3. Management files a **precommitment record**
   (`docs/precommitment-records.md`). It predicts each probe result from
   the spec, and the latency and tool-call failure band it expects at each
   stage.
4. If no vault entry exists for the provider, management escalates "add a
   key for Synthetic" to the owner as stream input, not a blocking gate.
   The panel shows it as a pending request. The owner adds the key (2.4).
5. Management asks the host for the probe run with the proposed catalog
   entry. The gateway adds it to the policy at stage `probe`, which is
   admitted only for the prober, runs the suite (4.3), and writes the
   receipt. The receipt resolves the precommitment record's probe
   predictions.
6. If the required checks pass, management drafts a **policy proposal**:
   the catalog entry, quirks taken from the receipt, the target pool
   change, and the stage plan with its gates (4.4).
7. The owner sees the proposal in Texture, with a link to the panel, and
   approves it with one step-up act. That act authorizes the whole staged
   plan, not each step. Per the doctrine, human ratification is per effect
   class, not per effect.
8. The host advances the stages as the gates pass. Each advance is a
   policy version with its evidence. It demotes on the triggers in 4.5.
   Management reports progress, and Texture renders it.

### 4.3 The probe suite

Each check names the receipt it comes from. A check is either `required`
or `recorded`.

| Check | What it does | Why (receipt) |
| --- | --- | --- |
| Auth | A good key gets 2xx. A deliberately bad key gets the provider's 401 shape, recorded so parks work. | `opencode-401-key-never-parked-2026-10-22.md` |
| Model ID | The model is listed by the provider, and the response echoes the requested ID. | `research-child-picks-unserved-model-2026-10-11.md` |
| Basic completion | Text in and text out. Usage fields parse. | |
| Streaming | Event shape; usage in the final event; no duplicated tool-call deltas; a clean end. | `responses-stream-duplicates-tool-calls-2026-10-10.md` |
| Tool calls | One call, parallel calls, `tool_choice` required, a tool-result round trip, valid JSON arguments, and a large schema the size of the desk surface. | `tool-call-failures-dual-model-2026-10-21.md` |
| Reasoning parameter | Which effort values are accepted, ignored or rejected. Where reasoning content comes back. Whether it must be replayed, or must be stripped, on the next turn. | `model-pools-design-2026-10-12.md` §5 (replay poisoning); the OpenCode request path sends no reasoning field (`provider.go:1100-1300`) |
| Caching | The same long prefix sent twice. Whether cached tokens are reported and at what ratio. Whether a cache key or session header is honoured. | `prompt-cache-breaks-across-turns-2026-10-14.md`, `paper-chatgpt-cache-ratio-below-85-after-restarts-2026-10-15.md` |
| Context limit | The largest accepted input near the advertised window, and the error shape just over it. The result sets `ContextWindowTokens`. | `catalog.go:28-30` |
| Output budget | Whether a max-output parameter is accepted (Codex rejects one), and the real maximum. | `model-pools-design-2026-10-12.md` §1 |
| Images | The vision probe's colour-and-word image. Pass removes the model from `TextOnlyModels`. | `models-configured-text-only-2026-10-15.md`; `scripts/vision_probe.py` |
| Latency | Time to first token and total time, p50 and p90, over a fixed prompt set at desk-sized inputs. | longcat left the pool at 115 s p90 (`catalog.go:281-284`) |
| Error shapes | The 429 body and its Retry-After or quota headers; 402 or quota exhaustion; 5xx; a timeout. Each is mapped to the failover classes. | `free-model-429-opens-provider-circuit-stub-stories-2026-10-14.md`, `chatgpt-usage-limit-stops-research-and-management-2026-10-11.md` |
| Concurrency | N parallel calls on one key, and where 429s begin. That sets a per-entry concurrency hint. | `gateway-model-calls-have-no-scheduler-2026-10-21.md` |
| Cost | Usage fields against the provider's stated prices. Fills `internal/llmcost`. | `internal/llmcost/pricing.go` |

Required for `catalog`: auth, model ID, basic completion, streaming, tool
calls, the reasoning replay rule, error shapes, and a context limit at or
above the role minimum. Images are recorded, and decide only whether the
model is text-only. The rest are recorded.

`scripts/vision_probe.py` and `/api/model-policy/verify`
(`internal/agentcore/api.go:258-292`) are the seed of this suite. Today
they need the model to be in the catalog already, because the gateway
refuses others. That is why the `probe` stage exists: catalogued for the
prober only.

### 4.4 Stages and what admits a model to each

| Stage | Who is routed to it | Gate to enter |
| --- | --- | --- |
| `probe` | Only the host prober. | An owner-approved probe request and a vault entry. |
| `catalog` | Only explicit choices (a named research child, a verify call). No pool. | All required probe checks pass. |
| `test_computer` | Pool member on the computers listed as test computers in `computertiers`. | The owner's plan approval, and a probe receipt from the last 24 hours. |
| `small_weight` | Pool member for ordinary computers, at a weight of about 10% of threads. | On test computers: at least N desk runs (proposed: 50) with tool-call failures per cell no worse than the incumbent's by more than a stated margin, measured on the same task kinds as in `tool-call-failures-dual-model-2026-10-21.md`; no failover storm; p90 latency inside the bound. |
| `target` | The weight in the plan, for ordinary computers. | The same measures, held for a window (proposed: 24 hours) at small weight. |
| `owner` / `platform` admitted | The owner's computers, then the paper. | `target` held for a soak period (proposed: 72 hours), plus the plan naming those classes. Or an owner override with an end time. |

The numbers are proposals. The plan carries them, and the owner's approval
fixes them for that onboarding.

### 4.5 Automatic demotion

The host demotes on its own and writes a policy version that cites the
evidence. These are mechanical halts of one route while the computer keeps
running, so they need no owner decision. That is the doctrine's "fail
over, do not escalate".

- **To `fallback_only`** (no new threads, still a failover target):
  - the model's breaker open for more than X% of a window
    (`internal/gateway/circuit_breaker.go`, per model);
  - 429s still failing after backoff on more than Y% of calls;
  - p90 latency above the bound for a window;
  - tool-call failures per cell above the incumbent's plus the margin,
    over a minimum sample.
- **To `catalog`** (out of every pool): every credential for the provider
  parked for 401 or 402; or the model gone from the provider's model list
  in a re-probe.
- **Re-probes.** A daily re-probe runs the required checks. A failure
  demotes one stage.

Demotions stand until the owner re-promotes, which is a step-up act, or a
re-probe and the next stage's gate both pass and the owner's original plan
said "auto-recover". The owner's plan chooses which.

### 4.6 The Synthetic lane as the first manual instance

`lane/fp0-synthetic-provider` (worktree `/Users/wiz/Dev/lanes/synthetic-provider`;
no commits yet at this writing) is doing by hand what this flow automates:

| Hand step in the lane | What automates it |
| --- | --- |
| A coding agent reads Synthetic's docs. | Research's cited provider spec (4.1). |
| The adapter is written in `internal/provider`. | Still a platform change. Once a generic OpenAI-compatible adapter exists (slice 5), most of this becomes catalog data. |
| Probes are run by hand with the key from `.env`. | The host prober, with the vault entry, writing a receipt (4.3). |
| The catalog entry is a Go literal edit plus a deploy. | A `probe` or `catalog` policy version (3.1). |
| The 50/50 is a `Pools()` edit plus a deploy. | The approved stage plan, reaching `target` (4.4). |
| The key reaches Node B through `deploy-provider-creds.sh` upsert over SSH. | The panel's sealed add (2.4). |
| Tonight's paper 50/50 is a guest file edit by FP0. | An owner override with an end time (3.4). |
| Demotion, if it misbehaves, is an agent noticing and a commit. | Automatic demotion (4.5). |

The lane's probe notes should be kept as the first probe receipt, in the
4.3 table's columns, so the automated suite is checked against a real run.

## 5. The owner admin panel

### 5.1 Where it lives

- **A host-served page on its own origin**, for example
  `https://admin.choir.news`. A signed-in user's Desktop is served from
  their guest computer, on the main origin (section 1). Putting the panel
  there would let any script from a guest page call it. A separate origin
  gets its own host-only cookies. The passkey's relying-party ID
  (`AUTH_RP_ID`, `internal/auth/config.go:109`) can stay `choir.news`,
  because a registrable-domain RP ID is valid on its subdomains.
- **Strict CSP** on that origin, plus `frame-ancestors 'none'` and HSTS.
  It loads no third-party code. It is a small Svelte page built from the
  same repo but with its own entry point, so it shares no runtime with the
  Desktop.
- **Owner-only.** The page and its API refuse anyone not on the
  platform-owner allowlist (2.5).
- **It is not in Settings.** Settings is a per-computer pane served by the
  guest, and its header says runtime internals are not exposed there
  (`SettingsApp.svelte:390`). The per-computer frontend memo keeps host
  chrome in the thin platform shell, and computer panes in the computer
  (`docs/memo-per-computer-frontend-2026-08-13.md:49-51`, `:146-157`).
  Provider keys and host policy are host control-plane state, so they
  belong to the shell.
- **Texture is the supervision surface. The panel is the actuator.**
  Onboarding proposals, demotions and pending key requests reach the owner
  as commitments rendered in Texture (`docs/texture-live-supervision-architecture.md`).
  Each carries a link that opens the panel at that item, as Mail's
  approval link does (`frontend/src/App.svelte:245-262`). Acts happen in
  the panel, under step-up. The panel never reaches into a computer.

### 5.2 What it shows

- **Credentials**, by provider: name, label, fingerprint, state, order,
  source (`vault` or `env`), added and last-rotated times, last use, the
  last hour's 2xx, 401, 402 and 429 counts, and any current park. Never a
  value.
- **Provider health**: breakers per provider and model (`GET /provider/v1/breakers`,
  `handlers.go:1140-1152`), the prompt-cache hit rate
  (`gatewayHealthResponse.PromptCache`, `handlers.go:34-36`), the model
  scheduler (`:47-49`), and quota signals where the provider sends them.
- **Pools**: the members, weights and stages, and for each member its
  share of threads, failovers, p90 latency and tool-call failures per
  cell.
- **Policy versions**: the current one, its diffs, its history, and who
  made each and why.
- **Onboarding**: probe receipts, stage plans, the current stage, the next
  gate's evidence, and demotions.
- **Pins and overrides**: the host's admission rules, any live overrides
  with their end times, and each class's effective routes. The paper's
  own file is computer-owned, so the panel shows the paper's resolved
  routes as the gateway sees them (calls by model, from the gateway's own
  counters). It does not read the guest file.
- **Audit**: the vault and policy logs, and whether each chain verified.

### 5.3 What it offers

Every write takes a step-up assertion, except pauses.

- Add a key, or an account (device login or paste).
- Replace A with B.
- Revoke.
- Reorder a provider's credentials.
- Pause, or resume, a credential or a model. Pause is allowed without
  step-up and with an `owner:acts` key. Resume needs step-up.
- Set pool weights, and save the result as a new version.
- Roll back to version N.
- Add an override with an end time, for example "the paper may use X
  until T".
- Approve or reject an onboarding plan.
- Import environment credentials, once (2.8).

## 6. Phased plan for the launch first pass

Each slice names its mutation class and the protected surfaces it touches.
Red slices carry the full ceremony in their own goal file. This plan does
not replace that.

**Slice 0, now, no new code.** The Synthetic lane lands its adapter with
the model in the catalog only, outside the default pool. The key goes over
the upsert mode (2.8 step 5), as an owner host step. FP0's overnight paper
50/50 is the owner's override, recorded in the ledger with its end time.
Class: whatever the lane declares (red: provider calls).

**Slice 1, first to build (recommended).** This is the smallest slice that
removes the script from key care and protects the pins.

- 1a. **Pin hardening.** A concrete-model role gets no pool fallbacks
  unless its file names them (3.4.1). A pool can say `failover: none`, so
  under R2 its members fail instead of moving to each other (3.5). It
  needs a failing test first, then the change. Class: orange, with full ceremony, because it touches model
  routing. Rollback: revert. Protected surface: provider and model routing.
- 1b. **Vault at rest, with env fallback.** Sealed records, the KEK as a
  gateway-only credential excluded from backup, the hidden directory, a
  `CredentialPool` source with hot rebuild, starting with the OpenCode
  pool and Synthetic, `/health` `credentials_source`, and the audit log.
  Class: red (gateway and provider calls; backup contents). Protected
  surfaces: the gateway's credential path and the backup set. Rollback:
  remove the vault directory from the unit, and the env fallback serves.
- 1c. **The owner-only sealed write path.** The intake key, the
  operator route, the platform-owner allowlist, the step-up receipt (new
  audience `gateway`, act kinds `vault.*`), and a minimal panel page with
  add, replace, revoke and the list. Class: red (auth, step-up and owner
  acts). Protected surfaces: auth/session and owner-act receipts.
  Rollback: switch the route off and keep the vault read-only.
- Deployed proof: on staging, the owner adds a test key from the panel.
  `/health` shows it sourced from `vault`. A probe call uses it. A
  revoke stops it without a gateway restart. The backup's file list holds
  no `provider-vault-kek*`. A key never appears in the journal, which a
  scan of a canary value proves.

**Slice 2, pools as host data.** The versioned `HostModelPolicy`; the
guest reading it with a last-good copy; `llm_policy_version`; gateway
enforcement with the grace window and the computer-class admission rules
(3.4.2); weights and rollback in the panel. Class: red (model routing and
gateway calls). After slice 2, the owner's 50/50 is a panel act.

**Slice 3, the host prober and receipts.** The suite in 4.3, the `probe`
stage, signed receipts, and the daily re-probe. Class: red (provider
calls).

**Slice 4, agentic onboarding and automatic stages.** Management's
onboarding commitment, research's provider-spec report, the precommitment
record, the plan approval act, stage gates, and automatic demotion.
Class: red (routing). It depends on slices 2 and 3.

**Slice 5, later.** The ChatGPT device login; a generic OpenAI-compatible
adapter with quirks as data; auth-signed step-up receipts; TPM or network
unlock of the KEK (the FP9 decision); and deletion of the script and the
env file (2.8 step 4). Classes: red, except the deletion, which is
orange.

What waits past the first pass: weights inside a provider's credential
pool, per-task routing finer than a pool, and cost-aware routing.

## 7. Standing questions

1. **Who settled this?** No one. This is an orchestrator-side proposal.
   The owner settled four inputs: the vault and panel idea, multi-key and
   multi-account, key care with no agent, and agentic onboarding (all
   2026-10-22). The paper's luna pin (21:4x) and tonight's overnight
   override are also owner decisions, and so are rulings R1 to R5 of
   2026-10-23 (section 9). Everything else here waits for ratification.
2. **Does it conform to settled decisions?**
   - Host PostgreSQL (control and graph databases) plus guest SQLite: yes. It
     adds no database. Vault and policy are gateway files (3.1). (Written
     against the former two-Dolt-store rule.)
   - Platform-owned secrets and catalog, with computer-owned per-computer
     policy: yes. The host never writes a guest file (3.4).
   - Desks act by semantic act, management does not mutate, and research
     is read-only: yes (4.1).
   - Restarts end work: yes. A crash does not resume any onboarding stage
     transition in flight. The next gate evaluation is new work against
     the durable policy. Pinned stage state survives, because it is the
     policy itself.
3. **What still references what this deletes?** The script and the env
   file are cited by `docs/model-pools-design-2026-10-12.md`,
   `docs/learn/03-gateway-and-providers.md`, `nix/node-b.nix:2252-2278`
   and `scripts/off-host-backup.sh:235-236`. Each is repaired in the
   deletion commit (2.8 step 4). Nothing is deleted before then.
4. **Who consumes this in production?** The gateway is the only reader of
   credentials. Guests' `EnrichMetadata` and the gateway catalog check are
   the policy readers. `GET /provider/v1/models` has no consumer today;
   slice 2 gives it one.
5. **What is the single authority?**
   - Credentials: the vault directory. Env is a fallback with a visible
     `source`, retired in slice 5.
   - Host model policy: the `current` pointer in `model-policy/`. The
     compiled version 1 is only a fallback for an empty directory.
   - Per-computer pins: the computer's own file. The gateway's admission
     rule is a refusal, not a second source of pins.
   - Audit: each directory's own chained log.
6. **What artifact proves success?**
   - Key care: the vault record, `/health`'s source line, and an upstream
     call by entry name in the gateway's counters.
   - Onboarding: the host-signed probe receipt, and the policy version
     for each stage with its gate evidence.
   - None of these is a desk's report.
7. **What does the read path fate-share with?** Inference reads the
   credential pool and the policy from gateway memory, loaded from local
   files. That is no new dependency: not Dolt, not the proxy, not auth.
   Guests fall back to a last-good policy copy if the gateway's models
   endpoint fails. Panel writes do depend on the proxy, auth and the
   gateway together, which is acceptable for a rare owner act.
8. **What happens on restart?** The vault and policy are durable files,
   so a gateway restart loses only parks and breaker state, as today. The
   guest's policy cache is rebuilt from the gateway or its last-good file.
9. **Could an agent do this without SSH?**
   - Key care is done by the owner, in the panel, with no SSH.
   - Onboarding runs over the product path: semantic acts, the host
     prober, and policy versions.
   - The import step (2.8 step 2) replaces the only SSH step there is
     today.
   - The KEK's first generation is an activation snippet, which is host
     software, not an operation.

## 8. Heresy delta (this document)

- **Discovered:**
  - The paper's pin to gpt-6-luna is soft: it inherits default-pool
    fallbacks (`model_policy.go:225-231`, `model_policy_test.go:248`).
  - The full-regenerate mode of `deploy-provider-creds.sh` puts every
    provider value in local `ssh` arguments (`:208`).
  - Provider keys ride in the off-host backup
    (`off-host-backup.sh:235-236`).
  - The signed-in Desktop is served from the guest on the same origin as
    the owner's routes, and no CSP is set (`computer_surface.go:12-30`).
  - No owner act requires step-up (`owner_acts.go:25-39`).
  - No platform-owner principal exists.
  - Each now has its own problem record, written before any fix:
    - `docs/problems/paper-luna-pin-inherits-pool-fallbacks-2026-10-22.md`
    - `docs/problems/deploy-provider-creds-puts-keys-in-ssh-argv-2026-10-22.md`
    - `docs/problems/provider-keys-ride-in-off-host-backup-2026-10-22.md`
    - `docs/problems/guest-served-desktop-on-owner-origin-without-csp-2026-10-22.md`
    - `docs/problems/no-step-up-and-no-platform-owner-2026-10-22.md`
- **Introduced:** none. This document changes nothing.
- **Repaired:** none.

## 9. Owner rulings (2026-10-23 UTC)

The owner answered this document's five open questions, relayed by FP0.
Each answer is quoted verbatim.

- **R1. Platform owner.** "Yes, just for me." Vault and host-policy acts
  are owner-only. A committed allowlist holding the owner's user ID admits
  them (2.5). There is no second seat.
- **R2. Fail, do not fall back.** "Just ChatGPT luna and synthetic
  DeepSeek 1017 tonight. I'd rather them fail than fallback. We are still
  pre-launch." Before launch, a pinned model fails instead of falling
  back, and so do the two members of tonight's pair (3.4, 3.5, slice 1a).
  When launch comes, the owner revisits this.
- **R3. No OpenCode fallback.** "Don't use Opencode as fallback tonight."
  OpenCode is not a fallback, and it is not in the target pool even as
  `fallback_only` (3.5).
- **R4. Several ChatGPT accounts.** "Fine for my usage. Not for
  production usage to customers." Pooling several ChatGPT accounts is
  allowed only for the owner's own usage, never for customers'
  production traffic. That is enforced by `usage_scope: owner_only` (2.6).
- **R5. The KEK.** "For now just kept out of backups." The vault key is
  excluded from the backup. A TPM or network unlock is deferred (2.2).

Still open, and genuinely the owner's to settle:

1. Does the paper's pre-launch traffic count as "my usage" under R4? The
   paper is a platform computer whose stories the public reads. Until the
   owner says yes, it does not draw `owner_only` ChatGPT accounts.
