# Secrets vault design (draft, 2026-10-28)

Status: draft for review. Not built; nothing in this document is authorized
to execute. Owner requirement, 2026-10-28 (goal
docs/definitions/choir-data-architecture-selection-2026-10-28.md, section
"Owner requirement: a secrets vault"). Author: DeepSeek v4.1 flash
(delegated design research, read-only, code refs at 7e962b910). FP0
spot-checks: OpenBao latest release v2.7.2, published 2026-10-26 (GitHub
API) [V]; corpusd loads or generates the escrow key when
CHOIR_KEY_ESCROW_SEAL is unset (cmd/corpusd/main.go:176-178), and Node B
sets no seal mode (no CHOIR_KEY_ESCROW_SEAL in nix/) [V]; that hazard is
already recorded at
docs/problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md:110.
The precommitted expectations V0-01..V0-14 at the end are DeepSeek's, to be
copied into the program ledger as DeepSeek's records when this design is
accepted. Review: Devin (pending).

Read-only; no repo changes. Code refs at `7e962b910` (clean tree). Web facts checked 2026-10-28.

## Verdict

**Build one logical vault — `secretd` — as a host-side Go service with a frozen API and a Choir-owned envelope hierarchy. Do not make OpenBao, Vault, Infisical, SOPS/age, or sealed-secrets the system of record.** Use the repo's already-vetted primitives (X25519 + HKDF-SHA256 + XChaCha20-Poly1305, Ed25519) for the envelope; use SQLite for metadata; put the crypto behind a KMS seam so OpenBao Transit (or PKCS#11/HSM/KMIP) can replace the software KMS later without changing clients or the portable format. Adopt TPM 2.0/systemd-creds on Linux hosts, Secure Enclave on Apple, and a sealed root-only file as the last-resort host root. Use age/SOPS only for Nix bootstrap and offline recovery exports, never for user secrets.

The decisive constraint is not "which vault product is best"; it is the portability hard gate: a computer must restore from **tape + CAS + a small authority snapshot** onto another NixOS host, a Mac, a Linux desktop, and a self-hosted single box, and must not preclude offline desktop use. OpenBao's store is cluster-bound and its barrier keys are not a portable per-owner hierarchy; adopting it as the record would make portability an export/import project. Its useful part is the KMS/Transit engine, which fits behind the seam.

Mutation class: **red**. Review tier: **2** (Devin + DeepSeek v4.1 flash) because a defect would silently disclose secrets or silently fail crypto-erase. Ship on the proof ring first, then widen by ring — not "off for all".

## 1. Inventory: every key and secret today

The repo already violates doctrine I27's own named exception ("secrets and keys (their own vault)", `docs/choir-doctrine.md:442`). Secrets live in at least seven homes: env files, host secret files, the auth DB, the guest data image, the credential disk, Store A escrow rows, and process memory. The portability evidence already ranks the first two as blockers: the privacy DEK has one copy, and the host escrow key is the second single point (`docs/evidence/portability-inventory-and-verification-2026-10-28.md`, blockers 1 and 2). [V]

| Secret / key | Where today | Protection today | Vault owns? | Notes |
|---|---|---|---|---|
| Per-computer privacy DEK (XChaCha20, 32 B) | Guest data image `/mnt/persistent/choir-credentials/privacy-key`, mode 0400; `nix/autoputer-vm.nix:961`; load `internal/computerevent/privacy.go:100-135` | File mode + parent dir; same image as the data it protects | **Yes** | Must get an owner-ROOT protector and boot delivery; remove from data image. Current image copy = key and data in one file. [V] |
| Escrowed DEK wrap (`custodian`) | Store A, JSON `WrappedKey`; `internal/keyescrow/keyescrow.go:115-227`; upload lazy/best-effort `internal/autoputer/key_escrow.go:20-79` | Host X25519 escrow key + two-operator unwrap gate (`internal/platform/key_escrow.go:195-236`) | **Yes** | Guaranteed upload is owner-ratified (O21) but not landed. [V] |
| Host escrow private key (X25519) | `<artifacts>/key-escrow-key`, base64 0600; `keyescrow.go:71-111`; `nix/node-b.nix:465` | Optional seal under KEK (`CHOIR_KEY_ESCROW_SEAL=off|permit|require`, `internal/keyescrow/sealed.go:42-105`); default off | **Yes** | corpusd silently generates a new key if absent (`cmd/corpusd/main.go:177-186`) — a missing mount silently orphans every wrap. [V] |
| Escrow KEK (32 B) | `CHOIR_KEY_ESCROW_KEK_PATH`, absolute, 0400, outside artifacts and backup (`sealed.go:44-46,286-360`) | Owner-only regular file, no symlink | **Yes** | This is the current root of trust; R5 keeps it out of backups. [V] |
| Platform signing key (Ed25519) | `PLATFORM_SIGNING_KEY_PATH`, default artifacts/signing-key; `internal/platform/signing.go:42-73`; `internal/platform/config.go:75` | 0600 file; only corpusd/checkpointd mount it | **Yes** | One key signs every computer's `platform-control` receipts; public pinned in `internal/computerevent/appender.go:1098-1115`. Rotation scaffolding has no callers. [V] |
| Auth JWT signing key (Ed25519) | `/var/lib/go-choir/auth-signing/ed25519-key`; `nix/node-b.nix:952`; signs `internal/auth/handlers.go:285-298` | Root 0600; gateway also has it though only needs verify | **Yes** | NET-09 item 1: auth only; gateway verifies with public. [V] |
| Owner-act signing key (Ed25519) | `secrets/owner-act-signing-key`; `nix/node-b.nix:163`; `internal/owneract/owneract.go:59-60,319-322` | systemd `LoadCredential`; proxy only | **Yes** | Signs owner acts; currently no passkey step-up (separate gap). [V] |
| Host-internal service token | `secrets/internal-service-token`; `nix/node-b.nix:126`; `internal/hostauth/hostauth.go:47-67` | systemd credential, not env; header stripped at ingress; mode off/observe/enforce | **Yes** | One shared token for all host services; per-unit identity is the better end state. [V] |
| Issuer token | `secrets/issuer-token`; `nix/node-b.nix:149`; `internal/hostauth` issuer path | systemd credential | **Yes** | Needed with the internal token to mint a credential envelope. [V] |
| Dolt app/root passwords | `secrets/dolt-app-password`, `dolt-root-password`; `nix/node-b.nix:186-187` | systemd credentials; loopback DSNs | **Yes** | Dynamic DB creds are the later win. [V] |
| Escrow operator tokens | `secrets/corpusd-escrow-operators.env`; `nix/node-b.nix:1236` | Hash compare + headers (`internal/platform/key_escrow_http.go`) | **Yes** | Two-approval gate; single-file operators. [V] |
| Off-host backup credentials | `secrets/offhost-backup`; `nix/node-b.nix:386` | Restic repo password + SSH keys; same repo holds data and keys | **Yes** | One backup secret opens data + keys; split by class. [V] |
| Provider keys (LLM/search) | `gateway-provider.env`, `codex-auth.json`; `nix/node-b.nix:543,2598`; read once at startup `internal/provider/provider.go:1-10` | EnvironmentFile; hidden from other units; in backup set | **Yes** | Provider-vault design already exists (`docs/provider-vault-and-model-policy-design-2026-10-22.md`); fold it in, don't build a second vault. [V] |
| Mail provider secrets | `maild.env` (`RESEND_API_KEY`, webhook secret); `nix/node-b.nix:1067` | EnvironmentFile + hostauth | **Yes** | [V] |
| Claude delegate OAuth token | `secrets/claude-oauth-token`; vmctl writes it to the guest credential disk (`internal/vmmanager/claude_delegate.go`; `internal/capsule/claude_delegate.go:31`) | Guest-root readable; one test computer, expiring 2026-11-03 | **Yes** | Gateway relay (Option K) requires a Console API key; subscription-token intermediation is forbidden by Anthropic terms (`docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md`). [V] |
| Owner ROOT key (32 B) | Never persisted; wrapped per passkey credential in auth DB (`internal/auth/store.go:566-633`); PRF unwrap `internal/auth/handlers.go:1044-1098`; `keyescrow.go:228-270` | Passkey PRF wrap only; web app does not use it | **Yes** | It currently wraps **nothing**; it is the missing owner-level root. [V] |
| Owner per-computer content key (planned) | Not built; consensus decides per-owner envelope encryption for CAS (`docs/evidence/data-architecture-consensus-2026-10-28.md:83,135`) | — | **Yes** | Vault should own it from day one; CAS imports it. [V/H] |
| Guest signer keys (Ed25519) | `/mnt/persistent/choir-signers/{guest-core,verifier}/key.ed25519`; `nix/autoputer-vm.nix:622-626,701,750` | 0600 on data image; guest-core registered nowhere | **Yes (registry + wrap)** | Keys must stay in the guest to sign; vault owns key history, wraps, and rotation. [V] |
| Credential envelope bearer/nonce | 15-min envelope signed Ed25519; bearer/nonce HMAC keyed with the platform Ed25519 **seed** (`internal/platform/credential_envelope.go:23-39,391-398`) | Delivered on per-realization disk, erased after use | **No** | Re-mint; separate the HMAC key from the signing seed (hygiene). [V] |
| Computer capability token | 30-min Ed25519 capability, scopes `event:read/pin/append` (`internal/platform/event_capability.go:15-17`) | Signed; short-lived | **No** | Vault issues its own vault-scoped capabilities/leases. [V] |
| API keys / refresh tokens | Auth DB, SHA-256 hashes (`internal/auth/store.go:66,94,894`); refresh TTL 720 h (`internal/auth/config.go:87`) | Hashed at rest; no vault value | **No** | Auth owns issuance; vault may hold the JWT key. [V] |
| Snapshot volume key | Generated from `/dev/urandom` each boot, never written (`nix/snapshot-vault.nix:1-30`) | Ephemeral dm-crypt plain | **No** | By design; not a durable secret. [V] |

**What the vault must own:** all private key material and all user/platform credentials above, their versions, grants, leases, rotation, and audit. **What it must leave alone:** public keys, pins, control-key history, signed receipts, short-lived envelopes/capabilities/sessions, the ephemeral snapshot key, WebAuthn public credentials, non-secret config, and the tape/index itself. The tape records references and acts, never values (`internal/computerevent/redaction.go:23-31,143-180`; private payloads are already DEK-encrypted with HMAC secret handles, `internal/computerevent/privacy.go:255-277`). [V]

**Capturing the "odds and ends":** first slice is a **secret census**, not code: a repo-wide scanner using the existing `DetectPrivateSecrets` plus a host file/DB scan (`.env`, `*.env`, token files, key files, `EnvironmentFile=` lines, `LoadCredential=`, DB columns), producing an own/leave/unknown table. The repo has already run equivalent inventories for data (`docs/problems/data-has-more-than-one-home-2026-10-24.md`); secrets have not had one. [H]

## 2. Key hierarchy

```
owner passkey PRF ─┐
owner recovery kit ├──▶ owner ROOT ──▶ owner KEK_vN ──▶ per-secret DEK
custodian escrow ──┘         │                    └──▶ per-object CAS DEK
                             │                    └──▶ computer DEK (portable protector)
                             │
deployment root (Shamir/owner) ──▶ deployment KEK_vN ──▶ owner KEK server wrap
host root (TPM2 / Secure Enclave / sealed file) ──▶ deployment KEK / host identity
vault signing key (Ed25519) ──▶ leases and audit
realization key (ephemeral) ──▶ access identity only, never data
```

Decisions:

1. **Owner ROOT becomes real.** It is generated once, never persisted plaintext, and wrapped by independent protectors: passkey PRF (primary), owner recovery kit (offline 256-bit secret; required), custodian escrow (optional; default on for the recovery SLO, off for sovereign owners). This is the LUKS-keyslot/1Password/Bitwarden shape: a high-entropy secret in the wrap chain, never a password alone. [V] Bitwarden: master key → stretched key → protected user symmetric key; 1Password: account password + locally held Secret Key → 2SKD, vault keys shared by public-key encryption. [V](https://bitwarden.com/help/bitwarden-security-white-paper.md, https://1passwordstatic.com/files/security/1password-white-paper.pdf)

2. **Owner KEK** (32 B, versioned) is the data-encryption workhorse. It is wrapped three ways: under owner ROOT (portable, owner-held), under the deployment KEK (server convenience), and under the custodian escrow key (SLO recovery). The owner KEK wraps per-secret DEKs and the per-owner CAS object keys. Rotate quarterly or on suspicion; old versions remain decrypt-only until `min_decryption_version` advances.

3. **Per-secret DEK** per version. Secret value = XChaCha20-Poly1305 under that DEK; AAD binds `vault_id || owner_id || secret_id || version || key_id || purpose`, so ciphertext cannot be moved across slots. v1 stores values inline in the vault record (cap 64 KiB); larger values are a separate `secret.blob` kind in the owner CAS. One home per kind.

4. **Computer DEK** stays exactly the existing 32-byte XChaCha20 key (`internal/computerevent/privacy.go:100-135`), but gains an **owner-ROOT/owner-KEK protector**. This is the fix for the portability blocker: today only the host escrow key can open it, so losing Node B loses the data even if the owner has their ROOT. The host escrow wrap remains for SLO recovery and existing key delivery (`internal/platform/key_delivery.go:45-140`). [V]

5. **Host root** is one per host: TPM 2.0 sealed seed (Linux bare metal/desktop; systemd-creds AES256-GCM, PCR-bound, or tpm2-tss directly), Secure Enclave P-256 ECDH key (macOS; CryptoKit exposes P-256 ECDH and, in 2026 docs, MLKEM/MLDSA), or a sealed root-only file plus owner passphrase (self-hosted/VM fallback). It wraps the deployment KEK and the host identity key. [V](https://www.freedesktop.org/software/systemd/man/latest/systemd-creds.html, https://developer.apple.com/documentation/cryptokit/secureenclave)

6. **Threshold unseal** is for the **deployment root**, not every boot: Shamir 3-of-5 among owner devices/offline media for disaster recovery. Normal host boot auto-unseals from the host root. Do not require Shamir shares at every restart; it breaks unattended computers. OpenBao's own docs make the same distinction: Shamir default; auto-unseal via KMS/HSM; recovery keys authorize operations but cannot decrypt the root key. [V](https://openbao.org/docs/concepts/seal/)

7. **Rotation**:
   - secret value: new version, move `current`, old versions decrypt-only;
   - secret DEK: new version; rewrap references;
   - owner KEK: new version; lazy rewrap on read + background sweep;
   - owner ROOT: rewrap owner KEK; reissue passkey wraps and recovery kit;
   - deployment KEK: host add/remove; rewrap owner KEK server wraps;
   - host root: reinstall/hardware change; re-enroll; destroy old wrap;
   - computer DEK: on suspicion/restore; re-encrypt private payloads (expensive, scheduled);
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

**Subjects:** `owner:<id>`, `computer:<id>`, `desk:<computer>/<desk>`, `service:<host>/<unit>`, `operator:<id>`, `deployment:<id>`. **Grants** bind subject → secret → ops (`use`, `read`, `write`, `rotate`, `grant`, `destroy`) + constraints (destination host/path/method, max TTL, max uses, delivery mode, time window, owner presence). Default deny; secrets are opaque IDs; listing returns metadata only.

**Leases** are short-lived, signed tokens (Ed25519, vault key): `{lease_id, secret_id, version, subject, ops, destination, iat, exp, max_uses, lease_epoch, revocation_epoch, nonce}`. Default TTL 5 min, max 1 h; single-use for high-risk; bound to the computer/realization and the current lease epoch. Self-contained so the broker can validate offline; revocation via short TTL plus a revocation list for high-risk secrets. Attenuation only narrows.

**Delivery modes** (default first):
- **broker**: host-side egress proxy injects the credential; the guest never sees it. This is the existing gateway pattern: provider keys live on the gateway and the guest authenticates with a gateway bearer (`internal/gateway/anthropic_relay.go`, `internal/gateway/handlers.go:772-795`). The capsule egress proxy already exists (`internal/capsule/egress.go`). [V]
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

**Audit:** every grant/lease/use/rotate/revoke/destroy writes a hash-chained record `{actor, subject, secret_id, version, op, lease_id, destination, decision, reason, result, request_hash, prev_hash, sig}` with no value (optional HMAC of the value for correlation). Store in the vault audit log; publish the head to the platform control plane and an off-host witness so deletion is detectable. Per-computer tape gets only references/heads. Transparency logging, not per-unwrap theater, is the durable-substrate recommendation. [V]

**Zero trust between hosts:** host identity keys generated in TPM/SE; a signed host registry (host_id, Ed25519/X25519 keys, deployment_id, enrollment receipt); mutual TLS or signed request envelopes with pinned keys; every secret transfer encrypted to the destination host key; lease epochs fence stale hosts; no loopback/network-location trust; data at rest always encrypted. The goal's hard gate is exactly this ("signed, hashed, verified data"). [V]

**Least privilege:** per-secret destination allowlists, no wildcards by default; broker parses the destination, refuses redirects, pins DNS, validates TLS, limits method/body, and uses per-provider adapters so the model cannot put the secret into an arbitrary query string. The confused-deputy defense is the broker's destination policy, not the lease alone.

**Break-glass:** two named operators (existing escrow pattern) + owner notification + time box + full audit; cannot bypass a sovereign owner. Owner acts require a fresh passkey assertion bound to the exact write/grant; the current cookie-only owner-act gate is not sufficient (`docs/problems/no-step-up-and-no-platform-owner-2026-10-22.md`). [V]

## 4. Portability and restore

**Portable unit:** tape tail + CAS closure + authority snapshot (checkpoint) + **vault slice** + release identity. The vault slice is per-computer and small: owner KEK wraps, the computer DEK wrap under owner KEK, metadata for the secrets granted to that computer, lease/revocation epochs, audit head. It contains ciphertext only. Values are in the vault record or CAS. Target: O(#secrets granted), not all owner secrets; for an owner with 1,000 secrets, a few MiB.

| Scenario | How keys arrive | Values | Re-mint | Owner action | Offline |
|---|---|---|---|---|---|
| Same deployment, new NixOS host | Host enrolled → deployment KEK unwraps owner KEK server wrap | Snapshot + CAS | leases, bearer, capabilities, host tokens | none | after sync |
| Mac (Virtualization.framework) | Secure Enclave host root; owner unlocks ROOT via passkey PRF or recovery kit; client rewraps owner KEK to the Mac host key | Snapshot + CAS | leases, bearer, host tokens | owner unlock; one lease handover | yes after sync |
| Linux desktop | TPM2 or sealed file host root; same owner-mediated rewrap | Snapshot + CAS | same | owner unlock | yes |
| Self-hosted single box (new deployment) | New deployment root; owner ROOT/recovery kit rewraps owner KEK to the new host | Snapshot + CAS | all host keys, platform signing key per deployment | owner unlock + enrollment | yes |
| Offline desktop | Local vault + local CAS + signed leases | local | local | owner unlock | yes; cloud fenced by lease epoch |

**Re-mint vs transfer.** Transfer: owner KEK wraps, per-secret DEKs/ciphertext, computer DEK, user OAuth refresh tokens (the user's identity), audit log, public key history. Re-mint: gateway bearer, capabilities, leases, credential envelopes, session cookies, host tokens, dynamic DB creds, realization keys. Never transfer: host root private key, deployment KEK plaintext, platform-control key across deployments (a new deployment must have its own, or receipts forgery crosses deployments).

**Passkeys do not cross origins.** WebAuthn credentials are RP-ID-bound; a self-hosted box or a new domain cannot use the old passkey. The **recovery kit is the cross-deployment root**; after restoring, the owner registers a new passkey and wraps ROOT under it. This is the 1Password/Bitwarden lesson applied: the high-entropy secret is the portable protector. [V/H]

**Owner-held recovery:** recovery kit (offline, 256-bit, word/QR encoded) wraps ROOT; passkey is primary; custodian escrow is optional. Lose all protectors, lose the data — state it as product truth. Recovery acts are logged, delayed, and notified. A recovery drill must exercise the key ceremony, not just bytes (`docs/archive/choir-durable-substrate-2026-08-23.md` §3.4). [V]

**Restore ordering:** vault metadata and wraps before private data; if the vault is unavailable, restore can complete in a **sealed** mode with a manifest of unresolved secrets. Do not let the vault become a hard restore dependency before it has an HA story. In v1, keep the existing escrow DEK delivery as the boot fallback; the vault takes over when it has two hosts and a tested unseal path. This avoids turning a vault outage into a fleet-wide boot failure.

## 5. Evolution

Fixed interface (v1):
- `POST /v1/secrets`, `POST /v1/secrets/{id}/versions`, `GET /v1/secrets/{id}` (metadata only), `POST /v1/grants`, `POST /v1/leases`, `POST /v1/leases/{id}/revoke`, `POST /v1/rewrap`, `POST /v1/hosts/enroll`, `GET /v1/audit`, `GET /v1/health`.
- Opaque stable IDs; immutable versions; idempotency keys; canonical JSON; errors never contain values; allow/deny with a reason code.
- Lease token format and audit event schema are versioned, not changed casually.

Seams (the implementation can change without client changes):
- `MetaStore`: SQLite now → replicated log/Postgres later.
- `ObjectStore`: local CAS → S3-compatible/SeaweedFS.
- `KMS`: software (host root) now → OpenBao Transit → PKCS#11/HSM/KMIP.
- `AuditSink`: local hash chain → replicated log/external witness.
- `Policy`: simple grants now → org RBAC/paywall tiers later.

Scale path: single host (one SQLite, per-owner shards) → sharded multi-host (single writer per owner, lease epochs, signed replication) → dedicated vault hardware/cluster/global replication (ciphertext everywhere, KEKs region-bound by policy). The API and envelope format stay fixed; only the backends move. This is the same "implementations to the next 10x, interfaces to 100,000x" rule as the data architecture.

## 6. Build vs adopt, review, risks, precommits

| Option | Verdict | Why |
|---|---|---|
| **OpenBao** | **KMS/Transit seam later; not the record** | MPL-2.0, active v2.7.2 (2026-10-26), KV/policies/leases/audit/Transit/HA, namespaces. But cluster-bound store and barrier keys; portability becomes export/import; unseal is another boot dependency; 2.7 moved many auto-unseal mechanisms to plugins. Use Transit for key custody when HA/HSM arrives. [V](https://api.github.com/repos/openbao/openbao, https://openbao.org/docs/secrets/transit/, https://openbao.org/docs/configuration/seal/) |
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

**Mutation class and review.** Red; Tier 2 (two seats). Protected surfaces: auth/session, keys, checkpoint/evidence (audit), gateway/provider calls, possibly Store A. Ceremony: conjecture delta, protected surfaces, admissible evidence class, rollback, heresy delta. Cheap checks first: repo gates, nix eval both systems if `nix/` changes, dry-run proof scripts, CI mirror, rebase. An external crypto review is required before any user secret is stored.

**Build order (slices).**
1. Problem doc + precommit ledger + secret census (green; first commit).
2. Envelope library + host root + software KMS + SQLite metadata; unit/property/fuzz tests (red, dark).
3. `secretd` API + audit + leases; host services only; migrate **provider keys** first (highest pain, existing design); delete `gateway-provider.env` in the same mission (red, proof ring).
4. Owner vault UI + passkey step-up + grants; owner ROOT/KEK wiring; recovery kit (red; depends on passkey/origin work).
5. User secrets + broker/egress; one test computer; no key in guest; Claude delegate Option K (red).
6. Computer DEK owner protector + boot delivery; remove privacy key from data image (red).
7. Portability proof: second NixOS host with source off, Mac SE, Linux desktop, self-hosted single box, offline read (deployed proof).
8. Host enrollment + zero-trust RPCs + multi-host shards; OpenBao Transit as second KMS when a second host/HSM requires it (red).
9. HA/cluster/global replication behind the same API.

**Risks / edge cases.**
- Confused deputy/SSRF: broker destination allowlist, no redirects, DNS pinning, per-provider adapters.
- Secret exfiltration via model context/logs/tape: detection is regex + known-value hashes, not complete; broker-by-default and no-raw-read are the real controls.
- Crypto-erase vs backups: honest levels; deletion journal; retention; hard erase only with KMS key destruction.
- Host root availability: TPM PCR changes after firmware/kernel updates; recovery kit/re-enrollment; test every ring update.
- Vault boot dependency: keep the existing escrow DEK delivery fallback in v1; fail closed for user secrets, not for computer boot.
- Multi-host split-brain: single writer per owner, lease epochs, stale-epoch refusal.
- Key loss: no recovery if all protectors are lost; product truth.
- Provider ToS: subscription tokens cannot be collected/stored/intermediated by a service; use Console API keys for brokered providers.
- Passkey origin portability: recovery kit is the cross-deployment root.
- Implementation bugs: nonce reuse, AAD omission, key confusion, policy bypass; independent review + differential tests.
- Snapshot plaintext: a guest hibernated with a leased secret in RAM needs the snapshot vault; it is not imported yet (`nix/snapshot-vault.nix`).

**Precommitted expectations** (commit before evidence; 80% intervals; falsifiers; measurement difficulties):

| ID | Claim | Estimate | Falsifier | Hard to measure / mitigation |
|---|---|---|---|---|
| V0-01 | `secretd` steady RSS on Node B | 45 MiB [30, 90] | > 128 MiB | Page cache vs anon; smaps_rollup, 24 h |
| V0-02 | p99 lease issue (local socket, batched audit) | 7 ms [3, 20] | > 50 ms | audit fsync; client histograms |
| V0-03 | broker-added p99 latency | 3 ms [1, 12] | > 30 ms | gateway queueing; eBPF/client histograms |
| V0-04 | portable vault slice per 1,000 secrets | 2 MiB [1, 6] | > 10 MiB | metadata shape; measure on fixture |
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

**Dissent I am aware of.**
- *Adopt OpenBao now as the vault.* Strongest form: less crypto/access-control code, proven audit/leases, MPL-2.0. Rebuttal: cluster-bound store and unseal do not satisfy the Mac/self-hosted/offline portability gate; you still need a Choir broker and per-owner hierarchy, so adoption saves less than it appears. Use Transit.
- *Store secret values on the tape/CAS under the DEK.* Rebuttal: the tape is replicated, replayed, indexed and backed up; secret values multiply exposure, have no leases/revocation, and crypto-erase would require rewriting the tape (forbidden). References only.
- *SOPS/age + Git is enough for v1.* Rebuttal: no runtime access control, leases, audit, or revocation; plaintext values in developer workflows; wrong for user credentials.
- *Use OS keychains/password managers for user secrets.* Rebuttal: no host-side broker/audit, per-user licensing, and no per-computer capability model; acceptable as an owner-facing export/import, not the record.
- *Defer until after the data architecture.* Rebuttal: the portability evidence names the DEK single copy and escrow key as blockers now; the vault is the fix. It depends on the authority snapshot, but the envelope and census can start in parallel.
- *Per-computer vault only.* Rebuttal: owner-level secrets (one API key for all the owner's computers) are the common case; per-owner KEK with per-computer grants is the least-privilege shape.

**Confidence.**
- Inventory and current gaps: **high** (code-read at a clean ref; no live host read).
- Hierarchy shape and broker-by-default: **high**.
- Build-vs-adopt direction (OpenBao/Infisical/SOPS not the record; Choir-owned portable envelope): **high**.
- In-house `secretd` vs OpenBao Transit first: **medium** — the seam is the decision; the first implementation can change.
- Performance/operational estimates: **medium to low** — no build yet; intervals above are the precommit.
- Overall recommendation: **high**, with the explicit fallback: if independent crypto review is unavailable, put OpenBao Transit behind the KMS seam before storing user secrets.

**Key evidence anchors.** Code: `internal/keyescrow/keyescrow.go:71-111,115-227,228-270`; `internal/keyescrow/sealed.go:42-105,286-427`; `internal/computerevent/privacy.go:88-135,255-277`; `internal/computerevent/redaction.go:23-31,143-180`; `internal/filecas/filecas.go:159-240`; `internal/auth/handlers.go:1044-1098`; `internal/auth/store.go:566-633`; `internal/platform/signing.go:42-73`; `internal/platform/key_escrow.go:195-236`; `internal/platform/key_delivery.go:45-140`; `internal/platform/credential_envelope.go:23-39`; `internal/platform/event_capability.go:15-17`; `internal/hostauth/hostauth.go:47-67`; `internal/gateway/handlers.go:772-795`; `internal/gateway/anthropic_relay.go`; `internal/capsule/egress.go`; `nix/node-b.nix:126,149,163,186-187,386,465,543,623,952,1236,2598`; `nix/autoputer-vm.nix:622-626,701,750,961`; `nix/snapshot-vault.nix`. Docs: `docs/choir-doctrine.md:442`; `docs/problems/data-at-rest-snapshots-images-escrow-key-plaintext-2026-10-21.md`; `docs/problems/claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md`; `docs/problems/data-has-more-than-one-home-2026-10-24.md:111-112,164`; `docs/evidence/portability-inventory-and-verification-2026-10-28.md`; `docs/provider-vault-and-model-policy-design-2026-10-22.md`; `docs/evidence/data-architecture-consensus-2026-10-28.md:83,135`; `docs/archive/choir-durable-substrate-2026-08-23.md` §3.2. Web: OpenBao repo/release/license, Transit, seal, migration; Vault LICENSE; Infisical LICENSE/self-hosting; SOPS/age/Tink/sealed-secrets repos; systemd-creds; CryptoKit Secure Enclave; Bitwarden whitepaper; 1Password white paper; KMIP 2.1.
