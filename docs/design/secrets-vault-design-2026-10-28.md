# Secrets vault design (draft, 2026-10-28; revision 2)

Status: draft for re-review. Not built; nothing in this document is authorized
to execute. Owner requirement, 2026-10-28 (goal
docs/definitions/choir-data-architecture-selection-2026-10-28.md, section
"Owner requirement 2026-10-28 ~14:47 UTC: a secrets vault"). Author: DeepSeek
v4.1 flash (delegated design research, read-only, code refs at 7e962b910,
re-verified for revision 2 at a9fa488a3). Review round 1: Devin, tier 2,
send_back (commit b32737920). This is revision 2, the author's answer to that
review; review round 2 is pending. FP0 spot-checks: OpenBao latest release
v2.7.2, published 2026-10-26 (GitHub API) [V]; corpusd loads or generates the
escrow key when CHOIR_KEY_ESCROW_SEAL is unset (cmd/corpusd/main.go:176-186),
and Node B sets no seal mode (no CHOIR_KEY_ESCROW_SEAL in nix/), so an absent
key is silently regenerated and, under SealOff, not even logged [V]; that
hazard is already recorded at
docs/problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md:108-118.
The precommitted expectations V0-01..V0-16 at the end are DeepSeek's, to be
copied into the program ledger as DeepSeek's records when this design is
accepted.

Read-only; no repo changes. Code refs at `7e962b910` (clean tree); revision-2
citation checks at `a9fa488a3` (the docs-only commit b32737920 adds no code).
Web facts checked 2026-10-28.

## Revision 2 changes

1. **B1 fixed.** §2 decision 1 now states where ROOT unwrap runs: only in an
   owner client. The login `root_key` field is retired, not kept as a
   fallback, and the platform-plaintext invariant is named.
2. **B2 fixed.** §4 names the vault slice's per-secret contents. Inline
   ciphertext (≤ 64 KiB) rides the slice; larger values ride as a CAS ref. The
   vault's host SQLite is a serving cache, not the authority. The restore table
   and V0-04 follow.
3. **B3 fixed.** V0-15 (migration fidelity) and V0-16 (a recovery-kit-only
   drill) are added; both silent-failure classes now have falsifiers.
4. **Inventory completed.** `searxng.env` (a third silent-generate secret),
   the gateway identity store, Caddy's ACME/certificate keys, SSH host keys,
   the internal-token header copy, and the `unit-env/` directory join the
   census; the JWT row is rewritten from the actual mount picture.
5. **One review item was checked and not applied**, with the evidence in
   place: `gateway-identities.json` stores SHA-256 token hashes, never raw
   bearers (`internal/gateway/config.go:155-169`), so the inventory marks it
   leave-alone. The JWT correction **was** applied after checking systemd
   semantics: `InaccessiblePaths` hides everything below a listed directory,
   so the gateway's `authSigningDir` entry hides the private key despite the
   redundant `jwtKey` exemption.
6. **Citations corrected:** pinned platform key `appender.go:1196-1215`;
   provider env read `provider.go:3393-3450` (not `:1-10`; `:3157-3210` is the
   ChatGPT token refresh, not the env read); envelope HMAC
   `credential_envelope.go:381,398-405`; escrow key default
   `internal/platform/config.go:76`; portability evidence blocker 1 carries
   both halves, blocker 2 is the files tree.
7. **Lease use-count** now has a broker seen-set per `lease_epoch`; offline
   validation bounds expiry and binding, not use count.
8. **Blast radius and metadata at rest** are named: one deployment KEK unwraps
   every non-sovereign owner's KEK, with the per-host-wrap alternative; secret
   names and destination detail are encrypted under a per-owner metadata key.
9. **Rotation and envelope hygiene:** the guest keyring gains a digest→key map
   so retired computer DEKs stay resolvable; per-purpose HKDF `info` strings
   and canonical AAD are pinned per wrap level.
10. **Sequencing and ceremony:** slice 4 is split so the computer-DEK owner
    protector lands before brokered user secrets; the build goal inherits an
    evidence class, rollback and heresy-delta section; OpenBao adoption carries
    the repo's ≥ 7-day cooldown.
11. **Dissent:** the provider-vault-only alternative is recorded and rebutted.

## Verdict

**Build one logical vault — `secretd` — as a host-side Go service with a frozen API and a Choir-owned envelope hierarchy. Do not make OpenBao, Vault, Infisical, SOPS/age, or sealed-secrets the system of record.** Use the repo's already-vetted primitives (X25519 + HKDF-SHA256 + XChaCha20-Poly1305, Ed25519) for the envelope; use SQLite for metadata; put the crypto behind a KMS seam so OpenBao Transit (or PKCS#11/HSM/KMIP) can replace the software KMS later without changing clients or the portable format. Adopt TPM 2.0/systemd-creds on Linux hosts, Secure Enclave on Apple, and a sealed root-only file as the last-resort host root. Use age/SOPS only for Nix bootstrap and offline recovery exports, never for user secrets.

The decisive constraint is not "which vault product is best"; it is the portability hard gate: a computer must restore from **tape + CAS + a small authority snapshot** onto another NixOS host, a Mac, a Linux desktop, and a self-hosted single box, and must not preclude offline desktop use. OpenBao's store is cluster-bound and its barrier keys are not a portable per-owner hierarchy; adopting it as the record would make portability an export/import project. Its useful part is the KMS/Transit engine, which fits behind the seam.

Revision 2 pins the two invariants a build must not guess: **ROOT unwrap is client-only**, and **secret values ride the portable slice**. Everything else the review accepted stands.

Mutation class: **red**. Review tier: **2** (Devin + DeepSeek v4.1 flash) because a defect would silently disclose secrets or silently fail crypto-erase. Ship on the proof ring first, then widen by ring — not "off for all".

## 1. Inventory: every key and secret today

Doctrine already carves secrets out — "secrets and keys (their own vault)", `docs/choir-doctrine.md:442` — and the carve-out has no vault behind it. Secrets live in at least seven homes: env files, host secret files, the auth DB, the guest data image, the credential disk, Store A escrow rows, and process memory. The portability evidence already ranks this as blocker 1: the privacy DEK has one copy, and the host escrow key is the second single point (`docs/evidence/portability-inventory-and-verification-2026-10-28.md`, blocker 1, both halves). Blocker 2 in the same document — the files tree as a second home — is a data-architecture item the vault does not fix. [V]

| Secret / key | Where today | Protection today | Vault owns? | Notes |
|---|---|---|---|---|
| Per-computer privacy DEK (XChaCha20, 32 B) | Guest data image `/mnt/persistent/choir-credentials/privacy-key`, mode 0400; `nix/autoputer-vm.nix:617,961`; load `internal/computerevent/privacy.go:100-135` | File mode + parent dir; same image as the data it protects | **Yes** | Must get an owner-ROOT protector and boot delivery; remove from data image. Current image copy = key and data in one file. [V] |
| Escrowed DEK wrap (`custodian`) | Store A, JSON `WrappedKey`; `internal/keyescrow/keyescrow.go:115-227`; upload lazy/best-effort `internal/autoputer/key_escrow.go:20-79` | Host X25519 escrow key + two-operator unwrap gate (`internal/platform/key_escrow.go:195-236`) | **Yes** | Guaranteed upload is owner-ratified (O21) but not landed. [V] |
| Host escrow private key (X25519) | `<artifacts>/key-escrow-key`, base64 0600; `keyescrow.go:71-111`; default `internal/platform/config.go:76`; hidden path `nix/node-b.nix:550` | Optional seal under KEK (`CHOIR_KEY_ESCROW_SEAL` = off, permit, or require; `internal/keyescrow/sealed.go:42-105`); default off | **Yes** | corpusd silently generates a new key if absent (`cmd/corpusd/main.go:176-186`); under SealOff nothing is logged even on generation — a missing mount silently orphans every wrap. [V] |
| Escrow KEK (32 B) | `CHOIR_KEY_ESCROW_KEK_PATH`, absolute, 0400, outside artifacts and backup (`sealed.go:44-46,286-360`) | Owner-only regular file, no symlink | **Yes** | This is the current root of trust; R5 keeps it out of backups. [V] |
| Platform signing key (Ed25519) | `PLATFORM_SIGNING_KEY_PATH`, default artifacts/signing-key; `internal/platform/signing.go:42-73`; `internal/platform/config.go:75` | 0600 file; only corpusd/checkpointd mount it | **Yes** | One key signs every computer's `platform-control` receipts; public pinned in `internal/computerevent/appender.go:1196-1215`. Rotation scaffolding has no callers. [V] |
| Auth JWT signing key (Ed25519) | `/var/lib/go-choir/auth-signing/ed25519-key`; created by auth's `ExecStartPre` (`nix/node-b.nix:949`); only authd sets `AUTH_JWT_PRIVATE_KEY_PATH` (`:952`) | Root 0600; authd signs; the proxy verifies with `ed25519-key.pub` (`:1003`) | **Yes** | Only authd reads the private key: the gateway unit's `InaccessiblePaths` hides the whole `authSigningDir` (`:2603`), and a hidden parent hides the `jwtKey` exemption with it. [V] |
| Owner-act signing key (Ed25519) | `secrets/owner-act-signing-key`; `nix/node-b.nix:163`; `internal/owneract/owneract.go:59-60,319-322` | systemd `LoadCredential`; proxy only | **Yes** | Signs owner acts; currently no passkey step-up (separate gap). [V] |
| Host-internal service token | `secrets/internal-service-token`; `nix/node-b.nix:126`; `internal/hostauth/hostauth.go:47-67` | systemd credential, not env; header stripped at ingress; mode off/observe/enforce | **Yes** | One shared token for all host services; per-unit identity is the better end state. [V] |
| Host-internal token header copy | `${internalTokenFile}.header`; `nix/node-b.nix:129,2684-2693` | Root 0600; a curl header line for root shells | **Yes (same record)** | A second copy of the same token on disk, kept in sync at activation; vault delivery retires the file or derives the header at use. [V] |
| Issuer token | `secrets/issuer-token`; `nix/node-b.nix:149`; `internal/hostauth` issuer path | systemd credential | **Yes** | Needed with the internal token to mint a credential envelope. [V] |
| Dolt app/root passwords | `secrets/dolt-app-password`, `dolt-root-password`; `nix/node-b.nix:186-187` | systemd credentials; loopback DSNs | **Yes** | Dynamic DB creds are the later win. [V] |
| SearXNG secret | `/var/lib/go-choir/searxng.env` (`SEARXNG_SECRET`); consumed at `nix/node-b.nix:844` | 0600 env file; one consumer | **Yes** | Third silent-generate case: created at activation when absent (`nix/node-b.nix:2665-2670`), so a lost file quietly mints a new secret. [V] |
| Gateway identity store | `/var/lib/go-choir/gateway-identities.json`; `nix/node-b.nix:547,2622` | 0600; **SHA-256 token hashes only**, never raw bearers (`internal/gateway/config.go:155-169`) | **No** | Re-minted on credential rotation; holds no recoverable credential. Keep it out of backups anyway; it is an inventory signal. [V] |
| Caddy ACME account key and certificate private keys | Caddy state directory (ACME account key, issued certificate keys); Caddy is the public edge (`nix/node-b.nix:745-885`) | Caddy user-owned files; outside the host secret set | **Host identity (registry)** | Re-issuable from ACME; losing them costs a re-issue, never data. Never transferred; a moved host re-issues. [V/H] The path is Caddy's default state dir, not configured in `nix/`. |
| SSH host keys | `/etc/ssh/ssh_host_*_key`, generated by NixOS at first boot; public halves pinned by peers (`docs/reports/node-a-backup-handover-2026-10-20.md:45-46`) | Root 0600; trust-on-first-pin | **Host identity (registry)** | Never transfer; a new host mints new keys and every peer re-pins. [V] |
| Escrow operator tokens | `secrets/corpusd-escrow-operators.env`; `nix/node-b.nix:1236` | Hash compare + headers (`internal/platform/key_escrow_http.go`) | **Yes** | Two-approval gate; single-file operators. [V] |
| Off-host backup credentials | `secrets/offhost-backup`; `nix/node-b.nix:386` | Restic repo password + SSH keys; same repo holds data and keys | **Yes** | One backup secret opens data + keys; split by class. [V] |
| Provider keys (LLM/search) | `gateway-provider.env`, `codex-auth.json`; `nix/node-b.nix:543,2598`; read from the environment when providers are constructed (`internal/provider/provider.go:6-8,3393-3450`) | EnvironmentFile; hidden from other units; in backup set | **Yes** | Provider-vault design already exists (`docs/provider-vault-and-model-policy-design-2026-10-22.md`); fold it in, don't build a second vault. [V] |
| Mail provider secrets | `maild.env` (`RESEND_API_KEY`, webhook secret); `nix/node-b.nix:1067` | EnvironmentFile + hostauth | **Yes** | [V] |
| Claude delegate OAuth token | `secrets/claude-oauth-token`; vmctl writes it to the guest credential disk (`internal/vmmanager/claude_delegate.go`; `internal/capsule/claude_delegate.go:31`) | Guest-root readable; one test computer, expiring 2026-11-03 | **Yes** | Gateway relay (Option K) requires a Console API key; subscription-token intermediation is forbidden by Anthropic terms (`docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md`). [V] |
| Owner ROOT key (32 B) | Never persisted; wrapped per passkey credential in auth DB (`internal/auth/store.go:566-633`); PRF unwrap `internal/auth/handlers.go:1044-1098`; `keyescrow.go:228-270` | Passkey PRF wrap only; web app does not use it | **Yes** | It currently wraps **nothing**; it is the missing owner-level root. Unwrap must move to the client (§2 decision 1). [V] |
| Owner per-computer content key (planned) | Not built; consensus decides per-owner envelope encryption for CAS (`docs/evidence/data-architecture-consensus-2026-10-28.md:83,135`) | — | **Yes** | Vault should own it from day one; CAS imports it. [V/H] |
| Guest signer keys (Ed25519) | `/mnt/persistent/choir-signers/{guest-core,verifier}/key.ed25519`; `nix/autoputer-vm.nix:622-626,701,750` | 0600 on data image; guest-core registered nowhere | **Yes (registry + wrap)** | Keys must stay in the guest to sign; vault owns key history, wraps, and rotation. [V] |
| Credential envelope bearer/nonce | 15-min envelope signed Ed25519; bearer/nonce HMAC keyed with the platform Ed25519 **seed** (`internal/platform/credential_envelope.go:23-39,381,398-405`) | Delivered on per-realization disk, erased after use | **No** | Re-mint; separate the HMAC key from the signing seed (hygiene). [V] |
| Computer capability token | 30-min Ed25519 capability, scopes `event:read/pin/append` (`internal/platform/event_capability.go:15-17`) | Signed; short-lived | **No** | Vault issues its own vault-scoped capabilities/leases. [V] |
| API keys / refresh tokens | Auth DB, SHA-256 hashes (`internal/auth/store.go:66,94,894`); refresh TTL 720 h (`internal/auth/config.go:87`) | Hashed at rest; no vault value | **No** | Auth owns issuance; vault may hold the JWT key. [V] |
| Snapshot volume key | Generated from `/dev/urandom` each boot, never written (`nix/snapshot-vault.nix:1-30`) | Ephemeral dm-crypt plain | **No** | By design; not a durable secret. [V] |

**What the vault must own:** all private key material and all user/platform credentials above, their versions, grants, leases, rotation, and audit. Host identity keys (SSH, Caddy ACME, host root) are owned for **registry and rotation history**, not for transfer. **What it must leave alone:** public keys, pins, control-key history, signed receipts, short-lived envelopes/capabilities/sessions, the ephemeral snapshot key, WebAuthn public credentials, hashed gateway tokens, re-issuable TLS certificates, non-secret config, and the tape/index itself. The tape records references and acts, never values (`internal/computerevent/redaction.go:23-31,143-188`; private payloads are already DEK-encrypted with HMAC secret handles, `internal/computerevent/privacy.go:255-277`). [V]

**Capturing the "odds and ends":** first slice is a **secret census**, not code: a repo-wide scanner using the existing `DetectPrivateSecrets` plus a host file/DB scan (`.env`, `*.env`, `*.header`, token files, key files, `EnvironmentFile=` lines, `LoadCredential=`, `unit-env/`, the Caddy state dir, `/etc/ssh`, DB columns), producing an own/leave/unknown table. The repo has already run equivalent inventories for data (`docs/problems/data-has-more-than-one-home-2026-10-24.md`); secrets have not had one. [H]

## 2. Key hierarchy

```
owner passkey PRF ─┐
owner recovery kit ├──▶ owner ROOT ──▶ owner KEK_vN ──▶ per-secret DEK
custodian escrow ──┘         │                    ├──▶ per-object CAS DEK
                             │                    ├──▶ per-owner metadata key
                             │                    └──▶ computer DEK (portable protector)
                             │
deployment root (Shamir/owner) ──▶ deployment KEK_vN ──▶ owner KEK server wrap
host root (TPM2 / Secure Enclave / sealed file) ──▶ deployment KEK / host identity
vault signing key (Ed25519) ──▶ leases and audit
realization key (ephemeral) ──▶ access identity only, never data
```

Decisions:

1. **Owner ROOT becomes real.** It is generated once, never persisted plaintext, and wrapped by independent protectors: passkey PRF (primary), owner recovery kit (offline 256-bit secret; required), custodian escrow (optional; default on for the recovery SLO, off for sovereign owners). This is the LUKS-keyslot/1Password/Bitwarden shape: a high-entropy secret in the wrap chain, never a password alone. [V] Bitwarden: master key → stretched key → protected user symmetric key; 1Password: account password + locally held Secret Key → 2SKD, vault keys shared by public-key encryption. [V](https://bitwarden.com/help/bitwarden-security-white-paper.md, https://1passwordstatic.com/files/security/1password-white-paper.pdf)

   **ROOT unwrap executes only in an owner client** (native app, web shell, or the recovery CLI). The auth server serves ciphertext wraps and salts; it never derives a wrap key and never returns ROOT. The `root_key` login field (`internal/auth/handlers.go:165,999-1002`) is retired, not kept as a compatibility fallback: today's `unwrapOwnerRoot` (`handlers.go:1044-1098`) derives the wrap key, unwraps ROOT, and ships it to the browser, where the app shell sets no CSP (`docs/provider-vault-and-model-policy-design-2026-10-22.md:169`). That path is dormant — no browser sends `unwrap_root` — and the recovery design already requires ROOT creation in the browser (`docs/reports/account-recovery-protector-design-2026-10-20.md:283-284`); this document makes the move mandatory. The invariant: **the platform's only paths to owner plaintext are the custodian wrap and the deployment-KEK server wrap; both exist per owner only while the owner has not elected sovereign.** Browser-held ROOT during a web unlock is a named exposure (no CSP today); the desktop/native client is the primary unwrap surface, the web shell second, the recovery CLI offline.

2. **Owner KEK** (32 B, versioned) is the data-encryption workhorse. It is wrapped three ways: under owner ROOT (portable, owner-held), under the deployment KEK (server convenience), and under the custodian escrow key (SLO recovery). The owner KEK wraps per-secret DEKs, the per-owner CAS object keys, and a **per-owner metadata key** (decision below). Rotate quarterly or on suspicion; old versions remain decrypt-only until `min_decryption_version` advances.

   **Metadata at rest.** Secret names, descriptions, and destination allowlists are encrypted under the per-owner metadata key; the vault's SQLite keeps opaque IDs, version numbers, epochs and timestamps in plaintext so the service can operate. The metadata key is wrapped under the owner KEK and the deployment KEK, so a non-sovereign host can list and serve; a sovereign owner declines the deployment-KEK wrap and their metadata opens only in an owner-unlocked session. Residual to name: counts, sizes, access times and grant shapes still leak. Destroying the metadata key erases the inventory's names and destinations; opaque IDs remain.

   **Deployment-KEK blast radius.** One deployment KEK unwraps every non-sovereign owner's KEK on every enrolled host. Compromise of the deployment root plus any enrolled host therefore opens every non-sovereign owner, not one. v1 accepts this for simplicity; the hardening path is **per-host wraps**: seal the owner KEK separately to each host identity, so revoking one host needs no fleet rewrap and a stolen host root opens only that host's wraps. Adopt per-host wraps when the second host joins, not later.

3. **Per-secret DEK** per version. Secret value = XChaCha20-Poly1305 under that DEK; AAD binds `vault_id || owner_id || secret_id || version || key_id || purpose`, so ciphertext cannot be moved across slots. Every wrap level has its own HKDF `info` string — the in-tree labels are `choir/keyescrow/v1/custodian/xchacha` and `choir/keyescrow/v1/passkey_prf` (`internal/keyescrow/keyescrow.go:40-41`); the vault adds `choir/vault/v1/<level>/<purpose>`, one string per level, never reused. AAD is canonical JSON with a domain tag (`CanonicalJSON`, as the private-artifact envelope already does at `internal/computerevent/privacy.go:255-277`); nonces are random 192-bit per seal. Cross-level and cross-purpose confusion is structurally impossible, not a review convention. v1 stores values inline in the vault record (cap 64 KiB); larger values are a separate `secret.blob` kind in the owner CAS. One home per kind.

4. **Computer DEK** stays exactly the existing 32-byte XChaCha20 key (`internal/computerevent/privacy.go:100-135`), but gains an **owner-ROOT/owner-KEK protector**. This is the fix for the portability blocker: today only the host escrow key can open it, so losing Node B loses the data even if the owner has their ROOT. The host escrow wrap remains for SLO recovery and existing key delivery (`internal/platform/key_delivery.go:45-140`). [V] Rotation requires a keyring extension: `guestPrivacyKeyring.resolve` returns only the current material (`privacy.go:225-241`), so retired DEKs must stay in a **digest→material map** loaded from the vault slice and decrypt-only. The map is the rotation mechanism, not an optimization.

5. **Host root** is one per host: TPM 2.0 sealed seed (Linux bare metal/desktop; systemd-creds AES256-GCM, PCR-bound, or tpm2-tss directly), Secure Enclave P-256 ECDH key (macOS; CryptoKit exposes P-256 ECDH and, in 2026 docs, MLKEM/MLDSA), or a sealed root-only file plus owner passphrase (self-hosted/VM fallback). It wraps the deployment KEK and the host identity key. [V](https://www.freedesktop.org/software/systemd/man/latest/systemd-creds.html, https://developer.apple.com/documentation/cryptokit/secureenclave)

6. **Threshold unseal** is for the **deployment root**, not every boot: Shamir 3-of-5 among owner devices/offline media for disaster recovery. Normal host boot auto-unseals from the host root. Do not require Shamir shares at every restart; it breaks unattended computers. OpenBao's own docs make the same distinction: Shamir default; auto-unseal via KMS/HSM; recovery keys authorize operations but cannot decrypt the root key. [V](https://openbao.org/docs/concepts/seal/)

7. **Rotation**:
   - secret value: new version, move `current`, old versions decrypt-only;
   - secret DEK: new version; rewrap references;
   - metadata key: new version; lazy re-encrypt of names/destinations on read + background sweep;
   - owner KEK: new version; lazy rewrap on read + background sweep;
   - owner ROOT: rewrap owner KEK; reissue passkey wraps and recovery kit;
   - deployment KEK: host add/remove; rewrap owner KEK server wraps (or the per-host wraps);
   - host root: reinstall/hardware change; re-enroll; destroy old wrap;
   - computer DEK: on suspicion/restore; add the new material to the keyring digest map, then re-encrypt private payloads (expensive, scheduled); old entries decrypt-only until the sweep finishes;
   - realization keys/leases: per boot/minute; re-mint, never transfer.

8. **Crypto-erase** has three honest levels:
   - *logical delete*: API hides the secret, tombstone recorded;
   - *cryptographic erase*: destroy the secret DEK or key version; all ciphertext unreadable on the host immediately;
   - *backup erase*: a replicated deletion journal is applied before any restore serves; old backup copies remain unreadable only after retention ages out, or immediately if the KEK lived in a non-exportable KMS/HSM key that is destroyed.
   Never claim level 3 before retention. For sovereign election, destroy the platform's custodian wrap and server wraps; the owner's own copies remain by their choice. The durable-substrate doc already reached this conclusion (`docs/archive/choir-durable-substrate-2026-08-23.md` §3.2). [V]

9. **Composition with CAS and tape**:
   - The vault owns the owner content key; private CAS objects use per-object DEKs wrapped under it; object IDs are keyed per owner so cross-owner equality does not leak; no convergent encryption for private content (consensus). [V]
   - Secret values are the vault's own kind, not CAS objects (the I27 exception), unless > 64 KiB.
   - The tape carries `secret://<id>@<version>` references and periodic audit-head commitments, not values. References are annotations: replay must succeed without the vault, or a restore is not durable. If a tape event's payload contained a secret, the existing redactor replaces it with a handle; extend `SecretHandle` with an optional `ref` (`internal/computerevent/redaction.go:13-16`). New tape event kinds, if needed, ride tape schema v2 (its own red goal), not v1.

## 3. Access model

**Subjects:** `owner:<id>`, `computer:<id>`, `desk:<computer>/<desk>`, `service:<host>/<unit>`, `operator:<id>`, `deployment:<id>`. **Grants** bind subject → secret → ops (`use`, `read`, `write`, `rotate`, `grant`, `destroy`) + constraints (destination host/path/method, max TTL, max uses, delivery mode, time window, owner presence). Default deny; secrets are opaque IDs; listing returns metadata only — and for a sovereign owner, only inside an unlocked session (§2 decision 2). Owner unlock happens in the client; the vault stores and serves wraps and never sees ROOT.

**Leases** are short-lived, signed tokens (Ed25519, vault key): `{lease_id, secret_id, version, subject, ops, destination, iat, exp, max_uses, lease_epoch, revocation_epoch, nonce}`. Default TTL 5 min, max 1 h; single-use for high-risk; bound to the computer/realization and the current lease epoch. Self-contained so the broker can validate offline; revocation via short TTL plus a revocation list for high-risk secrets. Attenuation only narrows. **Offline validation bounds expiry, subject, destination and version; it cannot bound use count.** The broker keeps a **seen-lease set per `lease_epoch`** keyed by `lease_id` + `nonce` and refuses a second use of a single-use lease; a consumer that validates a token without the broker gets expiry-bound use only, and high-risk secrets are broker-only for that reason.

**Delivery modes** (default first):
- **broker**: host-side egress proxy injects the credential; the guest never sees it. This is the existing gateway pattern: provider keys live on the gateway and the guest authenticates with a gateway bearer (`internal/gateway/anthropic_relay.go`, `internal/gateway/handlers.go:765-800`). The capsule egress proxy already exists (`internal/capsule/egress.go`). [V]
- **exec**: vault/host helper runs a command with the secret on a file descriptor or a private env, never argv; scrubbed after.
- **lease**: plaintext returned to the guest for a TTL; explicit owner opt-in per secret; audit; guest-root can read it, so treat as a capability, not confidentiality.
- **derive**: vault performs the HMAC/sign/decrypt and returns only the result; the key never leaves.

**Guest path:** realization credential/envelope → gateway bearer → broker route; broker checks `(computer_id, realization_id, peer address)` and the lease, then injects. The vault is not exposed directly to guests. Host services use a unix socket with `SO_PEERCRED` + hostauth token + unit identity; secrets are delivered on an fd/socket, not in the environment (VMMs inherit vmctl's environment; same class as `docs/problems/loopback-is-a-credential-2026-10-14.md`). [V]

**No secret in prompts or model context by default:**
1. The desk tool surface has no raw `read`; only `use`/`derive`.
2. Broker keeps the value out of the guest by default.
3. The existing redactor replaces known secret values before tape/context; add a known-value hash check for exact values and a taint tag for values returned by leases.
4. The context builder drops any `secret-handle:`/`secret://` value body; only the reference is visible if policy allows.
5. Egress proxy scans responses and logs for the secret value; error sanitization stays.
6. Never send secrets to model APIs; the broker injects at the edge. Prompt-cache keys are not secrets.

**Audit:** every grant/lease/use/rotate/revoke/destroy writes a hash-chained record `{actor, subject, secret_id, version, op, lease_id, destination_hash, decision, reason, result, request_hash, prev_hash, sig}` with no value (optional HMAC of the value for correlation). The destination is recorded as a hash; its plaintext lives in the encrypted metadata, so the audit log does not resurrect the exposure the metadata key removes. Store in the vault audit log; publish the head to the platform control plane and an off-host witness so deletion is detectable. The audit log records opaque `secret_id`, never names. Per-computer tape gets only references/heads. Transparency logging, not per-unwrap theater, is the durable-substrate recommendation. [V]

**Zero trust between hosts:** host identity keys generated in TPM/SE; a signed host registry (host_id, Ed25519/X25519 keys, deployment_id, enrollment receipt); mutual TLS or signed request envelopes with pinned keys; every secret transfer encrypted to the destination host key; lease epochs fence stale hosts; no loopback/network-location trust; data at rest always encrypted. The goal's hard gate is exactly this ("signed, hashed, verified data"). [V]

**Least privilege:** per-secret destination allowlists, no wildcards by default; broker parses the destination, refuses redirects, pins DNS, validates TLS, limits method/body, and uses per-provider adapters so the model cannot put the secret into an arbitrary query string. The confused-deputy defense is the broker's destination policy, not the lease alone.

**Break-glass:** two named operators (existing escrow pattern) + owner notification + time box + full audit; cannot bypass a sovereign owner. Owner acts require a fresh passkey assertion bound to the exact write/grant; the current cookie-only owner-act gate is not sufficient (`docs/problems/no-step-up-and-no-platform-owner-2026-10-22.md`). [V]

## 4. Portability and restore

**Portable unit:** tape tail + CAS closure + authority snapshot (checkpoint) + **vault slice** + release identity. The vault slice carries, per granted secret: `{secret_id, version list, wrapped DEKs under owner KEK, value ciphertext for ≤ 64 KiB secrets (or a CAS ref for larger), grant summary, lease/revocation epochs}`, plus the computer DEK and its owner-KEK wrap, the metadata-key wraps, and the audit head. Inline secret ciphertext rides the slice; nothing requires the vault's host store to be reachable at restore. The vault's own SQLite is a **serving cache, not the authority**: its replication is a later availability concern, not a portability gate. The slice carries no plaintext secret values; its metadata is limited to opaque IDs, version numbers, epochs and grant summaries. It is per-computer and small: O(#secrets granted), not all owner secrets. For an owner with 1,000 secrets at 1 KiB median values, a few MiB; at the 64 KiB cap per secret, about 64 MiB, which is the portability budget, not a defect.

| Scenario | How keys arrive | Values | Re-mint | Owner action | Offline |
|---|---|---|---|---|---|
| Same deployment, new NixOS host | Host enrolled → deployment KEK (or per-host wrap) opens the owner KEK wrap | Vault slice + CAS | leases, bearer, capabilities, host tokens | none | after sync |
| Mac (Virtualization.framework) | Secure Enclave host root; owner unlocks ROOT via passkey PRF or recovery kit; client rewraps owner KEK to the Mac host key | Vault slice + CAS | leases, bearer, host tokens | owner unlock; one lease handover | yes after sync |
| Linux desktop | TPM2 or sealed file host root; same owner-mediated rewrap | Vault slice + CAS | same | owner unlock | yes |
| Self-hosted single box (new deployment) | New deployment root; owner ROOT/recovery kit rewraps owner KEK to the new host | Vault slice + CAS | all host keys, platform signing key per deployment | owner unlock + enrollment | yes |
| Offline desktop | Local vault + local CAS + signed leases | local slice | local | owner unlock | yes; cloud fenced by lease epoch |

**Re-mint vs transfer.** Transfer: owner KEK wraps, per-secret DEKs/ciphertext, computer DEK and its keyring history, metadata-key wraps, user OAuth refresh tokens (the user's identity), audit log, public key history, host-key registry history. Re-mint: gateway bearer, capabilities, leases, credential envelopes, session cookies, host tokens, dynamic DB creds, realization keys, TLS certificates. Never transfer: host root private key, deployment KEK plaintext, SSH host keys, Caddy ACME account key, platform-control key across deployments (a new deployment must have its own, or receipt forgery crosses deployments). The platform-control trust root is a compile-time pin today (`internal/computerevent/appender.go:1196-1215`); "new deployment has its own" means it becomes provisioning configuration read at build or start, not a repo fork.

**Passkeys do not cross origins.** WebAuthn credentials are RP-ID-bound; a self-hosted box or a new domain cannot use the old passkey. The **recovery kit is the cross-deployment root**; after restoring, the owner registers a new passkey and wraps ROOT under it — in the browser, per §2 decision 1. This is the 1Password/Bitwarden lesson applied: the high-entropy secret is the portable protector. [V/H]

**Owner-held recovery:** recovery kit (offline, 256-bit, word/QR encoded) wraps ROOT; passkey is primary; custodian escrow is optional. Lose all protectors, lose the data — state it as product truth. Recovery acts are logged, delayed, and notified. A recovery drill must exercise the key ceremony, not just bytes (`docs/archive/choir-durable-substrate-2026-08-23.md` §3.4). [V]

**Restore ordering:** vault metadata and wraps before private data; if the vault is unavailable, restore can complete in a **sealed** mode with a manifest of unresolved secrets. Do not let the vault become a hard restore dependency before it has an HA story. In v1, keep the existing escrow DEK delivery as the boot fallback; the vault takes over when it has two hosts and a tested unseal path. This avoids turning a vault outage into a fleet-wide boot failure.

## 5. Evolution

Fixed interface (v1):
- `POST /v1/secrets`, `POST /v1/secrets/{id}/versions`, `GET /v1/secrets/{id}` (metadata only), `POST /v1/grants`, `POST /v1/leases`, `POST /v1/leases/{id}/revoke`, `POST /v1/rewrap`, `POST /v1/hosts/enroll`, `GET /v1/audit`, `GET /v1/health`.
- Ciphertext-only owner wrap endpoints: `GET /v1/owner/wraps`, `PUT /v1/owner/wraps/{protector}`. Until the owner-wrap migration, the auth DB serves these same records under the same contract (`internal/auth/store.go:566-633`); no endpoint ever accepts or returns ROOT plaintext.
- Opaque stable IDs; immutable versions; idempotency keys; canonical JSON; errors never contain values; allow/deny with a reason code.
- Lease token format and audit event schema are versioned, not changed casually.

Seams (the implementation can change without client changes):
- `MetaStore`: SQLite serving cache now → replicated log/Postgres later. Metadata rows are ciphertext under the per-owner metadata key; only IDs, versions, epochs and timestamps are plaintext.
- `ObjectStore`: local CAS → S3-compatible/SeaweedFS.
- `KMS`: software (host root) now → OpenBao Transit → PKCS#11/HSM/KMIP.
- `AuditSink`: local hash chain → replicated log/external witness.
- `Policy`: simple grants now → org RBAC/paywall tiers later.

Scale path: single host (one SQLite, per-owner shards) → sharded multi-host (single writer per owner, lease epochs, signed replication) → dedicated vault hardware/cluster/global replication (ciphertext everywhere, KEKs region-bound by policy). The API and envelope format stay fixed; only the backends move. This is the same "implementations to the next 10x, interfaces to 100,000x" rule as the data architecture.

## 6. Build vs adopt, review, risks, precommits

| Option | Verdict | Why |
|---|---|---|
| **OpenBao** | **KMS/Transit seam later; not the record** | MPL-2.0, active v2.7.2 (2026-10-26), KV/policies/leases/audit/Transit/HA, namespaces. But cluster-bound store and barrier keys; portability becomes export/import; unseal is another boot dependency; 2.7 moved many auto-unseal mechanisms to plugins. v2.7.2 is two days old and is a security release (Transit path traversal); the repo's engineering-network cooldown requires a version ≥ 7 days old (`docs/design/engineering-network-grants-2026-10-10.md:499-500`), so pin a newer patch when Transit is adopted. Use Transit for key custody when HA/HSM arrives. [V](https://api.github.com/repos/openbao/openbao, https://openbao.org/docs/secrets/transit/, https://openbao.org/docs/configuration/seal/) |
| **HashiCorp Vault** | **Reject** | BUSL 1.1, licensor IBM; additional-use grant excludes competing hosted/embedded offerings. A self-hosted Choir distribution bundling it needs legal review for no benefit over OpenBao. [V](https://raw.githubusercontent.com/hashicorp/vault/main/LICENSE) |
| **Infisical** | **Reject as record** | MIT core + `ee/`; self-host needs PostgreSQL and Redis; product shape is team/developer secrets, not per-computer portable authority; heavy for a desktop/self-hosted box. [V](https://raw.githubusercontent.com/Infisical/infisical/main/LICENSE, https://infisical.com/docs/self-hosting/overview) |
| **SOPS + age** | **Nix bootstrap/recovery only** | MPL-2.0/BSD-3; file encryption with GitOps, but no runtime access control, leases, audit, revocation, or rotation. Wrong for user secrets. [V](https://github.com/getsops/sops, https://github.com/FiloSottile/age) |
| **Tink** | **Not needed** | Apache-2.0, v2.9.0, misuse-resistant AEAD/key templates, but keyset management remains yours and the repo already uses Go x/crypto AEAD. Optional if a KMS speaks Tink keysets. [V](https://github.com/tink-crypto/tink-go) |
| **sealed-secrets** | **Reject** | Apache-2.0 but Kubernetes-specific, cluster-scoped key, no leases/audit/rotation. [V](https://github.com/bitnami/sealed-secrets) |
| **KMIP/HSM** | **Later KMS interface** | KMIP 2.1 is the OASIS standard; correct for enterprise hardware, overkill now. [V](https://www.oasis-open.org/committees/tc_home.php?wg_abbrev=kmip) |
| **Shamir/threshold** | **Deployment recovery, not boot** | M-of-n for disaster recovery; auto-unseal for unattended hosts. [V](https://openbao.org/docs/concepts/seal/) |
| **TPM 2.0 / Secure Enclave** | **Host root** | TPM seals to PCRs and host; SE holds a non-exportable P-256 key. Both fail on disk move/reinstall unless re-enrolled — that is desirable, but needs a recovery path. [V](https://www.freedesktop.org/software/systemd/man/latest/systemd-creds.html, https://developer.apple.com/documentation/cryptokit/secureenclave) |
| **Bitwarden/1Password/Signal hierarchies** | **Lessons, not adoption** | High-entropy secret in the wrap chain, server never holds keys, per-vault keys shared by public-key encryption, PRF passkeys wrap the user key. Signal's ratchet is not a vault model. [V](https://bitwarden.com/help/bitwarden-security-white-paper.md, https://1passwordstatic.com/files/security/1password-white-paper.pdf) |
| **Envelope encryption + KMS** | **Yes** | DEK per object/secret, KEK in KMS/host root, rewrap without plaintext, key versions, crypto-erase by key destruction. [V] |

**Recommendation detail.** Build `secretd` as a thin control plane over vetted primitives. Hard rule: no novel cryptography — reuse the in-tree constructions and Go stdlib/x-crypto; one envelope library with a minimal API; test vectors; property/fuzz tests; a differential test against OpenBao Transit before production. If the team cannot staff an independent crypto review, adopt OpenBao Transit for unwrap and keep `secretd` as policy/broker only. This satisfies "trusted open-source parts" for the crown jewels while keeping the portable hierarchy Choir-owned.

**Mutation class and review.** Red; Tier 2 (two seats). Protected surfaces: auth/session, keys, checkpoint/evidence (audit), gateway/provider calls, possibly Store A. Cheap checks first: repo gates, nix eval both systems if `nix/` changes, dry-run proof scripts, CI mirror, rebase. An external crypto review is required before any user secret is stored.

**Ceremony the build goal inherits.**
- *Conjecture delta:* the vault replaces escrow-file custody and env-file secrets with one envelope hierarchy; predicted effects are fewer single points, an auditable grant/lease surface, and a new boot dependency whose failure mode must be measured, not assumed.
- *Admissible evidence class:* code-read citations at a clean ref for current state; deployed staging acceptance for every red surface (escrow/provider migration, boot delivery, broker injection, owner unlock); precommit measurements with a named instrument; an owner-executed recovery drill; an independent crypto review report. Not admissible: local-only proof for vmctl, live computers, gateway credentials, auth/session or restore; unit tests alone; a clean scan treated as proof that no leak exists.
- *Rollback:* per-slice revert; the existing escrow DEK delivery and env files stay until their replacement proves on staging; a failed vault start must fall back to the existing boot path, not block a computer. Crypto-erase is the one irreversible act and carries no rollback.
- *Heresy delta:* discovered — server-side ROOT unwrap, `searxng.env` silent generation, the internal-token header copy, the envelope HMAC sharing the signing seed, plaintext metadata exposure, deployment-KEK blast radius; introduced — a new service and boot-path dependency, a metadata/inventory target, browser-held ROOT; repaired (when built) — silent-generate paths, keys on the guest disk, the single-copy DEK.

**Build order (slices).**
1. Problem doc + precommit ledger + secret census (green; first commit).
2. Envelope library + host root + software KMS + SQLite metadata; unit/property/fuzz tests (red, dark).
3. `secretd` API + audit + leases; host services only; migrate **provider keys** first (highest pain, existing design); delete `gateway-provider.env` in the same mission (red, proof ring).
4. Owner ROOT/KEK hierarchy + recovery kit + **client-side unwrap**; retire the login `root_key` field (red; depends on passkey/origin work).
5. Computer DEK owner protector + boot delivery; remove the privacy key from the data image; extend the keyring to digest→material (red; the blocker-1 fix; after slice 4).
6. Owner vault UI + grants + passkey step-up; metadata-key wiring (red; depends on slice 4).
7. User secrets + broker/egress; one test computer; no key in guest; Claude delegate Option K (red).
8. Portability proof: second NixOS host with source off, Mac SE, Linux desktop, self-hosted single box, offline read (deployed proof).
9. Host enrollment + zero-trust RPCs + multi-host shards; per-host owner-KEK wraps; OpenBao Transit as second KMS when a second host/HSM requires it (red).
10. HA/cluster/global replication behind the same API.

**Risks / edge cases.**
- Confused deputy/SSRF: broker destination allowlist, no redirects, DNS pinning, per-provider adapters.
- Secret exfiltration via model context/logs/tape: detection is regex + known-value hashes, not complete; broker-by-default and no-raw-read are the real controls.
- Crypto-erase vs backups: honest levels; deletion journal; retention; hard erase only with KMS key destruction.
- Host root availability: TPM PCR changes after firmware/kernel updates; recovery kit/re-enrollment; test every ring update.
- Vault boot dependency: keep the existing escrow DEK delivery fallback in v1; fail closed for user secrets, not for computer boot.
- Deployment-KEK blast radius: one key opens every non-sovereign owner on enrolled hosts; per-host wraps are the mitigation, adopted with the second host.
- Metadata exposure: names/destinations are ciphertext, but counts, sizes, timing and grant shapes remain visible; sovereign owners' metadata opens only in session.
- Lease replay: offline validation cannot enforce `max_uses`; broker seen-set per epoch, high-risk secrets broker-only.
- Multi-host split-brain: single writer per owner, lease epochs, stale-epoch refusal.
- Key loss: no recovery if all protectors are lost; product truth.
- Provider ToS: subscription tokens cannot be collected/stored/intermediated by a service; use Console API keys for brokered providers.
- Passkey origin portability: recovery kit is the cross-deployment root.
- Browser ROOT: no CSP today; desktop/native is the primary unwrap surface until a strict CSP exists.
- Implementation bugs: nonce reuse, AAD omission, key confusion, policy bypass; independent review + differential tests.
- Snapshot plaintext: a guest hibernated with a leased secret in RAM needs the snapshot vault; it is not imported yet (`nix/snapshot-vault.nix`).

**Precommitted expectations** (commit before evidence; 80% intervals; falsifiers; measurement difficulties):

| ID | Claim | Estimate | Falsifier | Hard to measure / mitigation |
|---|---|---|---|---|
| V0-01 | `secretd` steady RSS on Node B | 45 MiB [30, 90] | > 128 MiB | Page cache vs anon; smaps_rollup, 24 h |
| V0-02 | p99 lease issue (local socket, batched audit) | 7 ms [3, 20] | > 50 ms | audit fsync; client histograms |
| V0-03 | broker-added p99 latency | 3 ms [1, 12] | > 30 ms | gateway queueing; eBPF/client histograms |
| V0-04 | portable vault slice per 1,000 secrets (1 KiB median values, 64 KiB cap) | 3 MiB [1.5, 8] | > 12 MiB | cap-heavy fixture reaches ~64 MiB; measure on a fixture, report both |
| V0-05 | 1,000 secrets restored on a second host (vault portion) | 25 s [10, 90] | > 180 s | tape/CAS fetch dominates; instrument phases |
| V0-06 | No secret values in tape segments over 7 days | p = 0.85 | one non-fixture match | detector false negatives; known-value hash scan + manual review |
| V0-07 | Crypto-erase: reads fail < 1 s locally | p = 0.95 | any success | caches; restart + cross-process check |
| V0-08 | TPM-sealed host root survives 10 NixOS updates without re-enrollment | p = 0.60 | < 5/10 | PCR changes; per-update ledger |
| V0-09 | ≥90% of gateway provider calls brokered with no key in guest by end of Q1 | p = 0.60 | < 70% | per-provider API shapes; per-provider counters |
| V0-10 | Guest-root red team cannot obtain a broker-only secret without a grant | p = 0.75 | any success | deny-by-default; red-team receipt |
| V0-11 | Owner-mediated rewrap to a Mac | 12 s [5, 45] | > 120 s | WebAuthn prompts, network; timed drill |
| V0-12 | Vault adds < 5% host RSS and < 1% CPU at 1k leases/min | p = 0.70 | > 15% / 5% | noisy host; cgroup accounting |
| V0-13 | Detector catches ≥95% of injected known values in tape scan | p = 0.50 | < 80% | regex limits; inject a corpus, measure |
| V0-14 | No secret remains in env files after cutover | p = 0.80 | any `EnvironmentFile` secret | migration discipline; grep the Nix tree |
| V0-15 | After provider/escrow migration, 100% of existing custodian wraps and privacy keys unwrap to identical key digests through the vault path | p = 0.90 | any digest mismatch or missing wrap | legacy wrap formats; dry-run replay of every Store A row before cutover, not a sample |
| V0-16 | A recovery-kit-only ceremony (no host key files, no custodian wrap) restores a test computer's private tape artifacts | p = 0.70 | any byte unrecoverable | drill UX; precommit and receipt per drill |

**Dissent I am aware of.**
- *Adopt OpenBao now as the vault.* Strongest form: less crypto/access-control code, proven audit/leases, MPL-2.0. Rebuttal: cluster-bound store and unseal do not satisfy the Mac/self-hosted/offline portability gate; you still need a Choir broker and per-owner hierarchy, so adoption saves less than it appears. Use Transit.
- *Store secret values on the tape/CAS under the DEK.* Rebuttal: the tape is replicated, replayed, indexed and backed up; secret values multiply exposure, have no leases/revocation, and crypto-erase would require rewriting the tape (forbidden). References only.
- *SOPS/age + Git is enough for v1.* Rebuttal: no runtime access control, leases, audit, or revocation; plaintext values in developer workflows; wrong for user credentials.
- *Use OS keychains/password managers for user secrets.* Rebuttal: no host-side broker/audit, per-user licensing, and no per-computer capability model; acceptable as an owner-facing export/import, not the record.
- *Defer until after the data architecture.* Rebuttal: the portability evidence names the DEK single copy and escrow key as blockers now; the vault is the fix. It depends on the authority snapshot, but the envelope and census can start in parallel.
- *Per-computer vault only.* Rebuttal: owner-level secrets (one API key for all the owner's computers) are the common case; per-owner KEK with per-computer grants is the least-privilege shape.
- *Skip `secretd`; ship the provider-vault sealed files plus a computer-DEK owner-protector wrapper.* Rebuttal: the provider-vault shape (sealed files + systemd-credential KEK) is a subset of this hierarchy and cannot express per-owner KEKs, grants, leases, revocation or portability; the honest merge is to fold it in behind the vault API. The unresolved tension with "no mission-critical services from scratch" is real and is bounded only if the envelope stays thin and the named crypto review plus the Transit fallback are gates, not aspirations.

**Confidence.**
- Inventory and current gaps: **high** (code-read at a clean ref; no live host read).
- Hierarchy shape, ROOT custody (client-only unwrap) and portable slice contents: **high** (pinned by B1/B2 and verified in code).
- Build-vs-adopt direction (OpenBao/Infisical/SOPS not the record; Choir-owned portable envelope): **high**.
- In-house `secretd` vs OpenBao Transit first: **medium** — the seam is the decision; the first implementation can change.
- Performance/operational estimates: **medium to low** — no build yet; intervals above are the precommit.
- Overall recommendation: **high**, with the explicit fallback: if independent crypto review is unavailable, put OpenBao Transit behind the KMS seam before storing user secrets.

**Key evidence anchors.** Code: `internal/keyescrow/keyescrow.go:40-41,71-111,115-227,228-270`; `internal/keyescrow/sealed.go:42-105,286-427`; `internal/computerevent/privacy.go:88-135,225-241,255-277`; `internal/computerevent/redaction.go:13-16,23-31,143-188`; `internal/filecas/filecas.go:159-240`; `internal/auth/handlers.go:165,999-1002,1044-1098`; `internal/auth/store.go:566-633`; `internal/platform/config.go:75-76`; `internal/platform/signing.go:42-73`; `internal/platform/key_escrow.go:195-236`; `internal/platform/key_delivery.go:45-140`; `internal/platform/credential_envelope.go:23-39,381,398-405`; `internal/platform/event_capability.go:15-17`; `internal/hostauth/hostauth.go:47-67`; `internal/gateway/handlers.go:765-800`; `internal/gateway/config.go:155-169`; `internal/provider/provider.go:6-8,3393-3450`; `internal/computerevent/appender.go:1196-1215`; `internal/capsule/egress.go`; `nix/node-b.nix:126,129,149,163,186-187,386,504-550,543-547,623,844,952,1003,1067,1236,2598,2601-2607,2665-2670,2684-2693`; `nix/autoputer-vm.nix:617,622-626,701,750,961`; `nix/snapshot-vault.nix`. Docs: `docs/choir-doctrine.md:442`; `docs/problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md:108-118`; `docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md`; `docs/problems/data-has-more-than-one-home-2026-10-24.md:111-112,164`; `docs/evidence/portability-inventory-and-verification-2026-10-28.md` (blocker 1; blocker 2 is the files tree); `docs/provider-vault-and-model-policy-design-2026-10-22.md`; `docs/evidence/data-architecture-consensus-2026-10-28.md:83,135`; `docs/archive/choir-durable-substrate-2026-08-23.md` §3.2; `docs/reports/account-recovery-protector-design-2026-10-20.md:283-284`; `docs/design/engineering-network-grants-2026-10-10.md:499-500`. Web: OpenBao repo/release/license, Transit, seal, migration; Vault LICENSE; Infisical LICENSE/self-hosting; SOPS/age/Tink/sealed-secrets repos; systemd-creds; CryptoKit Secure Enclave; Bitwarden whitepaper; 1Password white paper; KMIP 2.1.

## Review round 1 (Devin, tier 2 seat 1), 2026-10-28

Frozen at a9fa488a3, sha256 f2906084…; verdict **send_back**. About 35
code citations verified; raw output in
`.agentic-consensus/data-arch/vault-review/out/devin.out` (gitignored).

Blocking, all accepted by FP0:
- **B1.** The design never states where owner-ROOT unwrap runs. The
  inherited code unwraps on the server and returns `root_key` at login
  (internal/auth/handlers.go:999-1002, verified). FP0 note: that path is
  dormant today (no browser code sends `unwrap_root`), and
  docs/reports/account-recovery-protector-design-2026-10-20.md already
  moves ROOT creation to the browser, so this is a design gap, not a live
  disclosure; no new problem doc. Fix: unwrap only in owner clients, retire
  the login `root_key` field, and state the invariant that the platform
  reaches owner plaintext only through the custodian and deployment-KEK
  wraps, which sovereign owners decline.
- **B2.** The vault slice is ambiguous about where secret values live at a
  source-dead restore. Fix: inline ciphertext (64 KiB or less) or a CAS ref
  rides the slice; the host SQLite is a serving cache, not the authority.
- **B3.** No falsifier for silent key loss. Add V0-15 (migration fidelity:
  every existing wrap unwraps to an identical digest) and V0-16 (a recovery-
  kit-only drill restores private artifacts with host key files deleted).

Non-blocking, to fold into the revision: inventory misses (searxng.env as
a third silent-generate secret; gateway-identities.json; Caddy/ACME and
SSH host keys; the second internal-token copy; unit-env/); the JWT row is
wrong (only authd holds the private key); citation fixes (appender.go:1196-1215;
provider.go:3157-3210,863-874); lease use-count needs a broker seen-set;
the deployment-KEK blast radius; plaintext vault metadata; the DEK keyring
needs digest-to-key lookup; the platform trust root becomes provisioning
config; split slice 4 so the computer-DEK owner protector lands earlier;
per-purpose HKDF info strings and canonical AAD; the evidence class and
heresy delta sections; the 7-day adoption rule for OpenBao 2.7.2.

Next: DeepSeek revises (author seat), then Devin re-reviews one seat.
