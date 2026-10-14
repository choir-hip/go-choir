# Principal Identity and Signed Acts

Date: 2026-10-15. Mutation class of this report: **green** (design only;
no code, no deploy, no secrets). Everything it proposes to build is orange
or red, named per station. Code was read at `main` 2bd769d1b and on the
local branches `fix/selfdev-mode-owner-only` (6bee3d318),
`fix/desk-cell-confinement` (790d5e2c1), `fix/maild-send-gate-2026-10-14`
and `security/internal-route-auth`. None of those branches is deployed.
Claims not confirmed in code or evidence are marked **UNVERIFIED**.

Sources read first: [CLAUDE.md](../../CLAUDE.md),
[choir-doctrine.md](../choir-doctrine.md),
[computer-ontology.md](../computer-ontology.md), the problem records
[guest-can-arm-own-selfdev-mode](../problems/guest-can-arm-own-selfdev-mode-2026-10-15.md),
[maild-send-gate-defects](../problems/maild-send-gate-defects-2026-10-14.md)
and [loopback-is-a-credential](../problems/loopback-is-a-credential-2026-10-14.md),
the reports [surface audit](surface-audit-cli-mcp-api-2026-10-14.md),
[self-development update architecture](selfdev-update-architecture-2026-10-15.md)
and [computer isolation security review](computer-isolation-security-review-2026-10-14.md),
and two earlier key studies this report builds on rather than repeats:
[precommitment-records.md §6](../precommitment-records.md) and its archived
code audit
[precommitment-signing-and-computer-keys-2026-10-11.md](../archive/precommitment/precommitment-signing-and-computer-keys-2026-10-11.md).

## The governing principle

**Owner (2026-10-15, paraphrased):** an agent's identity could be the
model, the model family, a session trajectory, a pattern, or a document,
filesystem or database. Maybe the answer is that **identity is the security
boundary that can maintain a cryptographic identity.**

**Lead's refinement:** the identity in question is the *accountable*
identity, the one a track record and precommitments attach to. Model,
trajectory and pattern are descriptive attributes, signed over inside
records, not identities. Track records survive model swaps: a desk's
calibration survives a model change, and `model_id` is a slice dimension.
Boundaries nest, so certification nests: a passkey in the secure enclave,
the computer in its VM, the desk certified by the computer. Desk-cell
confinement is what earns desks keys.

**How this report applies it.** I take the principle literally and use it
as the test for every candidate:

> A principal holds a key only if some boundary keeps that key away from
> every party the principal's signature is meant to be distinguished from.

The number of keys therefore equals the number of boundaries that can hold
one, not the number of names that act. A name that lives inside someone
else's boundary is an **attribute**, carried inside that boundary's signed
record. Separately, a name can be a **principal** (an accountable identity
with a track record, refusable and scorable) without being a **key holder**.
The two must not be conflated: the desk is a principal; whether it is a key
holder depends on where its signing happens, not on whether it matters.

Tested against the lead's refinement, the principle agrees on users,
computers, models and trajectories, and disagrees on one point: **desk-cell
confinement earns desks honest *attribution*, not keys.** Confinement puts
the desk's model-written code in a cage with no sockets
(790d5e2c1: seccomp denies `socket`, `connect`, `bind`; empty network
namespace; Landlock). The cage's only channel is the inherited pipe to the
guest runtime. So every desk act reaches a signer through the runtime, and
the runtime is one boundary that speaks for all four desks. A desk key held
by the runtime is a second label on the runtime's signature. §2.4 gives the
conditions under which a desk would become its own boundary, and reserves
the certificate slot for it so that later step needs no format change.

## The answer in one page

1. **Yes, an identification system is the substrate fix, but it is small.**
   The gaps share one cause: an authority check reads *where a request came
   from* (loopback, the tap address, a shared guest capability, a same-origin
   cookie) and treats it as *who signed it*. The fix is that each
   consequential act carries a signature from the boundary that actually
   decided it, and that the party that accepts the act (corpusd, maild, an
   adopting computer) verifies that signature instead of the transport.
2. **Key holders:** the owner (passkey, already registered for login); the
   host's own principals (corpusd's platform-control key, which exists; a
   new proxy owner-act key; a new host-builder key); and each computer (one
   computer identity key, plus the realization signer keys that already
   exist, certified by it).
3. **Principals without keys:** desks (attribute signed by the computer;
   keys deferred until a desk is its own boundary), sub-RLM cells, models,
   trajectories, capsules and API keys. API keys are bearer delegations from
   the owner, never signers; an owner-act receipt records that one was used.
4. **The format already exists.** `computerevent.Receipt` is RFC 8785
   canonical JSON, a `choir-receipt-v1\0` preimage prefix, a UUIDv7 receipt
   id, a canonical UTC issue time, and a rotation-aware key resolver
   (`internal/computerevent/receipt.go:19-92,173-263`,
   `canonical.go:12-15`). New act kinds reuse it and add an audience, a
   nonce, an expiry and a subject binding.
5. **S1 (red, small, on Station A's path):** owner-act receipts signed by a
   proxy key, carried into the owner's mode writes, and a corpusd admission
   gate that refuses an `effect_accepted` append unless its authority is a
   receipt corpusd can verify: an owner-armed ModeReceipt, a platform update
   offer, or (once S2 lands) a consensus decision receipt.
6. **S2 (red, M, on Station A's path, after desk-cell confinement):**
   signed ballots and a corpusd consensus decision receipt. One of the two
   seats is signed **outside the guest**, by the host builder, so the guest
   alone cannot reach quorum.
7. **S3 (red, M):** a host-served approval origin, then passkey-signed owner
   acts for the consequential acts. This is the only fix for maild G7: the
   browser stamps the page's origin into every WebAuthn assertion, so an
   assertion made from guest-served JavaScript is refusable.
8. **Later:** computer identity keys with escrow and published key history
   (S4, before cross-owner adoption or public track records); desk keys
   (S5, conditional).

## 0. What exists today

Two read-only sweeps of the code at 2bd769d1b, plus the branches named
above. Much of this matches the 2026-10-11 key study; the differences are
new findings and are marked as such.

**The platform-control key.** One host-held Ed25519 key in corpusd signs
every receipt in the `platform-control` domain for every computer.
`LoadOrCreateSigningKey` reads 64 raw bytes, or generates and writes a new
key at mode 0600 on first start (`internal/platform/signing.go:42-85`). The
path is `PLATFORM_SIGNING_KEY_PATH`, by default
`/var/lib/go-choir/platform-artifacts/signing-key`
(`internal/platform/config.go:12,52`), and only corpusd and checkpointd
mount it (`nix/node-b.nix:319,750,821`). It signs event-head and pin
receipts, ModeReceipts (`internal/platform/self_development_modes.go:189`),
lifecycle receipts, `VerifierKeyPinned` and `ExecutionIdentityJoin`
(`internal/platform/checkpoints.go:172,467`), the self-development
`AuthorityReceipt`s for checkpoints, routes and platform update offers
(`internal/selfdevprotocol/control.go:172-181`), credential envelopes and
capability tokens (below), and published version manifests
(`signing.go:97-135`). Its public key is pinned in the repo
(`internal/computerevent/appender.go:1098-1115`). Rotation scaffolding
exists and has no callers: `control_key_history` with activation and
invalidation columns (`internal/platform/computer_events.go:64-81`), a
resolver that enforces them (`control_keys.go:33-61`), and
`activateControlKey` and `revokeControlKey` (`:88-117`). `key_rotated` is
declared and never appended (`internal/computerevent/event.go:56`).

The ModeReceipt covers the computer, the old and new mode, generations, the
bound operation, bundle, heads and state commitments, expiry, the policy
and consensus digests, the `consumed_*` copy of the previous grant, the
idempotency key and the request commitment
(`self_development_modes.go:165-188`). Replay is controlled by the
generation CAS and the idempotency key. The guest verifies ModeReceipts in
the decision handler (`internal/agentcore/api_self_development.go:636-689`).
The owner-only branch adds `mode_writer` (`host-internal`,
`guest-consumption`, `corpusd-expiry`) and routes every non-internal caller
to `ConsumeAsGuest`, which admits only the two consumption writes.

**New finding: the guest has no trust anchor.** Only the proxy
(`internal/proxy/handlers.go:347`) and the CLI (`cmd/choir/main.go:47`)
compare the platform key with the repo pin. The guest verifies its boot
envelope against the public key embedded *in the envelope*
(`internal/platform/credential_envelope.go:72-91`,
`internal/selfdev/credentials.go:63-72`), then trusts that key for every
later receipt (`credentials.go:672-683`). vmctl takes the key from
corpusd's `GET /internal/platform/control-key` without a pin
(`internal/vmctl/ownership.go:1514-1544`).

**Guest signers and the updater.** `choir-receipt-signer` runs twice in
every guest, each instance as its own user, with its key on the data disk:
`guest-core` at `/mnt/persistent/choir-signers/guest-core/key.ed25519` and
`verifier-control` at `/mnt/persistent/choir-signers/verifier/key.ed25519`
(`nix/autoputer-vm.nix:591-650`). A key is generated with `O_EXCL` at
mode 0600 on the first boot of a disk
(`internal/receiptsigner/signing.go:17-81`). guest-core signs only
`HealthReceipt`, `MaterializationReceipt`, `UpdaterRecoveryReceipt`,
`KernelCapabilityReceipt` and `ExecutionIdentity`, and refuses a foreign
`computer_id` or an `issued_at` outside −10/+1 minutes
(`internal/receiptsigner/receiptsigner.go:80-109`). verifier-control signs
`VerifierCertificate`s (`internal/verifierprotocol/verifier.go:44-53`). The
updater holds no key and asks guest-core to sign
(`internal/updater/updater.go:136-139,379-385,1005`); release integrity is
digest-only (`closure.go:439-449`). The verifier-control key is pinned on
first use in `control_key_history`, and a different key is then refused
(`checkpoints.go:108-198`). **guest-core is registered nowhere:** every
verifier of a guest-core receipt uses the public key the guest presents
(`internal/updater/client.go:231`,
`internal/proxy/execution_identity.go:148-201`). The guest runtime runs as
root with no `User=` (`nix/autoputer-vm.nix:845-857`). It hides the key
files but not `/run/choir-signers` or `/run/choir-verifier`, so it can
call both signer sockets for their allowlisted kinds.

**Credential envelopes** are not encrypted. An envelope is a bearer
bootstrap token signed by platform-control over
`"choir-computer-credential-envelope-v1\0" || JCS(envelope)`. It lives at
most 15 minutes, is delivered on a per-realization `CHOIR_CRED` disk, and
is erased after use (`internal/platform/credential_envelope.go:21-39,346,380-389`;
`nix/autoputer-vm.nix:295-299`; `internal/autoputer/run.go:274-310`).
Its bearer and nonce are HMAC-SHA256 keyed with the platform Ed25519
*seed* (`credential_envelope.go:391-398`). That is key reuse across
primitives (**new finding**: a hygiene problem, not an exploit). corpusd
exchanges the envelope once for a `ComputerCapability`
(`"choir-computer-capability-v1\0"`, 30-minute TTL,
`internal/platform/event_capability.go:15-84`) whose scopes are
`event:read`, `event:pin` and `event:append` on every path
(`credential_envelope.go:211,227,312`).

**Key escrow** wraps one key per computer, the privacy DEK, and never a
signing key. The wrap is X25519 ECDH, then HKDF-SHA256 with the ComputerID
as salt, then XChaCha20-Poly1305 with the ComputerID as AAD
(`internal/keyescrow/keyescrow.go:40,115-165,280-287`). It is write-once
after genesis; a reveal needs two operator approvals and is logged
(`internal/platform/key_escrow.go:44-240`). Re-delivery to a new
realization re-seals the DEK to an ephemeral key, on proof of the
realization's issuance (`internal/platform/key_delivery.go:45-140`). A
`passkey_prf` protector already exists for an owner root key
(`keyescrow.go:34-38,228-260`; auth's `unwrap_root`,
`internal/auth/handlers.go:987-1084`), but the web app does not use it.
The escrow key is one X25519 file under `platform-artifacts`
(`config.go:53`). The operator tokens (`X-Choir-Operator`) are compared by
hash and gated by headers alone (`key_escrow_http.go:297-332`).

**The auth service.** Sessions are EdDSA JWTs signed with
`/var/lib/go-choir/auth-signing/ed25519-key`, which `ssh-keygen` generates
at first start (`nix/node-b.nix:520`; signing at
`internal/auth/handlers.go:285-298`). The claims are only `sub`, `email`,
`iat`, `exp` and `scope`: no session id, no `jti`, no audience. Access
tokens live 5 minutes. Refresh tokens live 720 hours, are opaque, are
stored hashed, and rotate on `GET /auth/session`
(`handlers.go:302-438,1181-1250`). Both cookies are `HttpOnly`,
`SameSite=Lax`, `Secure` in production, and host-only, with no `Domain`
(`handlers.go:27-31,325-345`).

The proxy checks only the JWT's signature and `scope`, with no revocation
lookup (`internal/proxy/handlers.go:432-472`), and falls back to a
`choir_sk_` API key (`:478-488,539-604`). It strips client identity
headers and sets `X-Authenticated-User`, `-Email` and `-Scopes` for the
guest (`:43-54,276-309,419-427`). The guest trusts those headers from any
caller that passes `HostSourcedCaller`: a unix socket, the default
gateway, **any loopback address**, or 192.0.2.0/24
(`internal/server/host_caller.go:31-61`).

**Passkeys.** Choir uses go-webauthn v0.16.4 (`go.mod:13`) with RP
`choir.news` (`cmd/auth/main.go:33-37`, `nix/node-b.nix:534-535`).
Registration requires user verification and requests PRF
(`handlers.go:476-582`). Login does **not** require user verification:
`BeginLogin` is called without `WithUserVerification`, so the library
checks presence only (`handlers.go:586-704`). Challenges are random bytes
from the library, stored in `challenge_state` (whose type admits only
`registration` and `login`) with a 5-minute TTL, and deleted on use
(`internal/auth/store.go:52-61,665-695`). Each credential keeps its COSE
public key, sign count, AAGUID and flags (`store.go:35-50`). The sign
count update is non-fatal, and the clone warning is never read
(`handlers.go:956`).

**No step-up or transaction-signing flow exists.** The library can
already do it (`WithChallenge`, `WithUserVerification`), so a
server-chosen challenge equal to an act digest needs a new challenge type,
a handler and a verifier, not a new library. The web app calls
`navigator.credentials.get` only for login
(`frontend/src/lib/auth.js:201-209`). No owner act uses a passkey today:

- self-development approve and mode set accept a cookie, or an API key
  with a self-development or `admin` scope
  (`internal/proxy/self_development.go:140-153,349-354`);
- mail send accepts a cookie only (`internal/proxy/email.go:30-32`);
- API-key creation needs only a cookie
  (`internal/auth/handlers.go:1706-1785`).

**Consensus ballots are hashes.** `BallotAttestation`
(`internal/decisionpolicy/types.go:150-164`) carries a free-string
`SignerProvenance`. Its `Attestation` is
`sha256("choir-ballot-attestation-v1:" || JCS(ballot))`, with no key
(`digest.go:64-129`), and the eligibility proof digest is checked only for
its shape (`reduce.go:302`). `Reduce` checks that the policy, manifest,
subject, selection and window agree, and applies the quorum and recusal
rules (`reduce.go:34-255`). Its only production caller is the guest's
decision handler (`api_self_development.go:401`), and no production code
builds a ballot. Anyone who knows the subject can make a quorum.

**`effect_accepted` is not authorized at append.** Two producers exist,
both in the guest:

- the decision handler, with `AuthorityRef`
  `external-owner:<X-Authenticated-User>` or
  `qualified-consensus:<digest>`, and the ModeReceipt as the first input
  artifact (`api_self_development.go:257-601`);
- the platform update path, with `platform-control:update` and the offer
  digest as `DecisionRef` (`internal/agentcore/platform_update.go:180-195`).

corpusd's append route requires only `event:append`
(`internal/platform/event_handlers.go:236-340`). `CompareAndSwap` checks
the commitments, the pins, the head CAS, the shared reducer's projection
and the digest (`computer_events.go:184-260`). Of an `effect_accepted`,
the reducer asks only that nothing is pending and that the target,
`ProposedEffectRef`, `DecisionRef` and `VerifierRefs` are present
(`internal/computerevent/reducer.go:82-91`). Nothing reads `AuthorityRef`,
the ModeReceipt or a consensus receipt. A decision binding check exists,
but it runs in the guest
(`internal/agentcore/self_development_decision_binding.go:27-120`).

**Receipts signed today, by signer:**

- platform-control: everything listed above;
- guest-core: health, materialization, updater recovery, kernel capability
  and execution identity receipts;
- verifier-control: verifier certificates;
- auth: session JWTs;
- a per-process ephemeral key: capsule capabilities
  (`internal/capsule/executor.go:130`, `capability.go:27-76`).

The host builder signs nothing: its receipt is plain JSON, written 0444
(`internal/builder/receipt.go:13-23`). maild verifies Resend's Svix HMAC
(`internal/maild/webhook.go:80-94,336`). No ECDSA, RSA or X.509 signing
exists in `internal/` or `cmd/`.

**Per-computer identity material.** The ComputerID is
`"computer-" + sha256(userID || 0 || desktopID)[:32]`, and the realization
id is `<vm>-epoch-<n>` (`internal/vmctl/ownership.go:3474-3485`). Both are
passed to the guest as kernel parameters. **No key is minted when a
computer is created.** A computer has the shared platform key, two
disk-scoped guest signer keys, an escrowed privacy DEK, per-realization
envelopes and capabilities, and a gateway token. Nothing in the code
re-keys or forks. The S9 definition requires fresh child identity material
before any runtime or network starts. Every guest key lives on
`/mnt/persistent`, so any copy of `data.img` carries all of them
(inferred from where the keys are stored, not tested).

## 1. Threat model

Each layer is named by the actor it defends against. "Defends" means the
actor cannot produce an act the verifier accepts as someone else's.

**A compromised guest** (any code with root in a computer: a capsule escape,
a Yaegi interpreter bug in a desk worker before confinement, a malicious
dependency in the runtime, a malicious tenant who owns the computer). This
is the primary adversary of self-development, because the guest is the
subject of the acts being authorized. It holds the guest's event
capability, can reach corpusd, maild and the gateway on the tap, can call
both guest signer sockets, and can serve any bytes as the owner's Desktop.
The design must make every owner act and every consensus decision
unforgeable by it. Defended by: owner-act receipts signed outside the guest
(S1), corpusd's admission gate (S1), a ballot signed outside the guest (S2),
and an approval origin the guest cannot serve (S3). Not defended: the guest
can sign anything with its own computer key, which is correct (that is the
computer speaking) and is why the computer key never authorizes the
computer's own effects.

**Compromised desk code** (model-written Go in one desk's Yaegi cell,
prompt-injected or confused). Before confinement this is the compromised
guest, because desk workers run as guest root
(isolation review F4). After confinement (790d5e2c1) a desk cell has no
sockets, no exec, no filesystem writes outside its tmp dir, and one pipe to
the runtime. Its only power is to say things *as that desk* through the
runtime. The design must keep it from (a) speaking as another desk, which
the runtime's pipe-to-desk binding gives once the cell cannot reach the
runtime's memory, and (b) producing facts the runtime vouches for (rule
settlements, test results), which domain separation gives: a desk act and a
runtime fact are different receipt kinds, and only the runtime may request
the latter. Defended by: confinement plus attribution (S2). Desk keys add
nothing here (§2.4).

**A stolen session cookie** (theft by malware on the owner's machine, a
leaked HAR, a shared browser profile). The access JWT lives 5 minutes and
the proxy never checks revocation; the real prize is `choir_refresh`,
which lives 720 hours and mints new access tokens. Either way the holder
is the owner for every route that reads the cookie. S1 does not defend
against it: the proxy will sign an owner-act receipt for a valid session.
S3 does, for the acts that require a passkey assertion, because the
assertion needs the authenticator and user verification at the moment of
the act. Routine reads and writes stay cookie-gated; that is the right
trade.

**Malicious guest-served JavaScript** (maild G7). The owner's Desktop,
apps and Mail are served by the guest on the same origin as `/api/*`
(`internal/proxy/computer_surface.go:27-89`), so guest JS can call any
owner route with the owner's cookie. To the proxy this is indistinguishable
from the owner's click, and no proxy-side receipt changes that. It can even
call `navigator.credentials.get` with `rpId` choir.news and show the owner
an authenticator prompt. Defended by S3 only: a host-served approval
origin, an `Origin` check on owner-act endpoints, `frame-ancestors 'none'`,
and the verifier of every passkey act refusing any assertion whose
`clientDataJSON.origin` is not the approval origin. Guest JS can still
*request* an approval; it cannot complete one, and the approval page shows
the owner what the host, not the guest, says is being approved.

**A non-root host process** (SearXNG, goproxy, Qdrant, Caddy; a VMM before
the jailer). Today loopback is the credential, so any of them can call
corpusd as "host-internal" and, after the owner-only fix, arm a mode as the
owner. hostauth enforce (`security/internal-route-auth`) closes the
generic hole. S1 additionally makes the owner's authority depend on a
signature from the proxy's key, so a loopback caller without that key
cannot arm a mode or post an owner decision even while hostauth is in
observe. This is a deliberate overlap: two independent conditions for the
most consequential write.

**Host compromise is out of scope.** Root on Node B (or the hosting
provider, or anyone holding a copy of the disks, which are not encrypted)
holds the platform-control key, the proxy and builder keys, the JWT key,
the escrow key, every guest disk and every guest's RAM. No software-only
design removes that; the 2026-10-11 key study says the same. The design
does three things about it anyway. It names each host key separately, so a
compromise of one *service* (the proxy, say) does not yield another's
authority. It keeps the passkey, the one key the host never holds, as the
signer of the acts that matter most, so a host compromise cannot produce a
*past* owner assertion for an act the owner never saw (it can still act
going forward, by serving a lying approval page). And it leaves external
anchoring of platform heads as the only defense against operator
backdating (precommitment-records §6, unchanged).

## 2. Principals and key custody

### 2.1 The owner (a human user): yes, a passkey

The owner is the one principal whose key lives in a boundary the platform
does not control: the authenticator's secure element. Choir already
registers passkeys for login (go-webauthn v0.16.4, `go.mod:13`). The key
signs owner acts in S3; until then, the owner's authority is a verified
session, attested by the proxy's key (S1).

Which acts need a passkey (S3): approving a self-development operation
(approve(operation, bundle digest)), arming or changing a self-development
mode (in particular arming the standing autonomous mode), approving an
email send, approving an adoption into a persistent computer, revoking a
computer key, and minting an API key that carries an owner-act scope. The
fleet push stays the owner's merge on GitHub, under GitHub's own
authentication; Choir does not need to re-sign it.

Every registered credential is an owner key. Revoking a passkey in auth
must propagate to corpusd's owner key history (S3), so a lost device stops
signing owner acts.

### 2.2 The host: several principals, each its own key

The host is not one principal. Each host service that *decides* something
signs as itself, so that the verifier can tell which decision it is
reading:

- **platform-control** (corpusd; exists). Signs event-head receipts,
  pins, checkpoints, mode receipts, platform update offers, credential
  envelopes. Gains in S1: the admission decision for `effect_accepted`; in
  S2: the consensus decision receipt.
- **host-owner-act** (the proxy; new in S1). Signs "I verified the owner of
  computer C, by method M, for act A over subject S." The proxy is where
  the owner is authenticated, so the proxy is where this statement can be
  made truthfully. A separate key, held only by the proxy through systemd
  `LoadCredential=`, means a non-root host process on loopback cannot
  produce it, and a corpusd bug cannot produce it either.
- **host-builder** (the build service; new, P5). Signs "I built closure X
  from base B plus patch P with recipe R" and, in S2, "I ran the declared
  test command against that tree and it passed." The owner's distribution
  ruling makes the host the sole *signed* builder; today the builder
  receipt is unsigned JSON (`internal/builder/receipt.go:13-23`), so the
  key is a prerequisite of that ruling.
- **auth-session** (the auth service's JWT key; exists). Signs sessions.
  It never signs acts; a session is an authentication, not an act.
- **escrow** (corpusd's escrow key; exists). Wraps computers' data keys.
  Not an act signer.

### 2.3 The computer: yes, one identity key plus certified realization keys

A computer is the boundary the ontology already treats as identity: a
stable ComputerID and its event chain, inside a VM. It gets one **computer
identity key**, generated in the guest at genesis, pinned by a
platform-signed receipt in `control_key_history`, escrowed like the
privacy key, delivered to each new realization, and never re-minted for an
existing computer. That is the plan in precommitment-records §6 item 3; this
report adopts it and adds one thing: the existing **realization keys**
(`guest-core` and `verifier-control`, generated per disk) become
*certified* by the computer key at each boot, instead of being trusted on
first use or not at all. A lost disk then re-mints realization keys and
re-certifies them, which resolves the "verifier trust substitution refused
after a lose-the-disk re-realization" hypothesis in the 2026-10-11 study.

What the computer key signs: records the computer makes about itself
(precommitment commit hashes, desk records with desk and model attributes,
ballots of seats that run inside it, release records it publishes,
adoption requests). What it never signs: authorization of its own effects.
corpusd must never accept a computer-key signature as authority for that
same computer's `effect_accepted`; that would be the self-approval loop
again, with a better-looking signature.

Custody caveat, stated plainly: "a key inside a computer is usable by any
code in it" is true of guest root. Today the runtime runs as root
(`nix/autoputer-vm.nix:845-857`, no `User=`) and can call the guest-core
socket and write to `/run/choir-verifier`, where the verifier signer
listens. Confinement removes desk cells from that set; the non-root
runtime (S1 security floor) is what would remove the runtime's ambient
root. Until then the computer key is exactly as strong as guest root, which
is why it is useful for attribution and never for self-authorization.

### 2.4 Desks: principals yes, keys no (for now)

A desk is an accountable identity: it has a track record, precommitments,
calibration, a seat in a consensus policy, and its records must survive a
model swap. All of that is satisfied by a **desk attribute inside a record
the computer signs** (`desk`, `seat`, `model_id`, `assignment`,
`trajectory_id`). The question is whether a separate desk key adds
anything.

It would add something only if a verifier needs to distinguish desk A's
signature from desk B's against an adversary that controls one of them but
not the other. Walk the cases:

- *The adversary is desk A's cell.* After confinement it cannot reach any
  signer; it can only send acts through its pipe, which the runtime labels
  as A. It cannot speak as B. A desk key adds nothing; confinement did the
  work.
- *The adversary is the runtime (or guest root).* It holds every
  runtime-held desk key and can sign as any desk. A desk key adds nothing.
- *The adversary is outside the computer.* The computer key already
  distinguishes this computer from it.

So a runtime-held desk sub-key is a label. It would also cost something
real: revocation and rotation for four keys per computer, and a reader who
sees "signed by management" may believe the management desk is a separate
trust domain when it is not. That is the mislabeled-authority hazard the
2026-10-11 study raised about host-held computer keys.

**Answer to the lead's ordering question.** Yes, order S2 after desk-cell
confinement lands, but for a different reason than desk keys: before
confinement, any desk cell is guest root and can call the guest signer
directly, so even the computer's attribution of a ballot to a seat is not
honest. After confinement, attribution is honest and needs no desk key.

**When a desk would earn a key (S5, conditional).** All three must hold:
the guest runtime is non-root and cannot read the desk's key file; the
desk's signing happens in a unit the runtime cannot drive as another desk
(for example a per-desk signer reachable only from that desk's cell over a
file descriptor the spawner hands it, with `SO_PEERCRED` checking the
desk's uid); and some verifier outside the computer needs desk-versus-desk
separation, such as a seat whose independence a cross-owner policy relies
on, or a desk that runs in its own VM. Then the desk's key is certified by
the computer key with `subject_kind: desk`, which the certificate format
reserves now (§3.4).

### 2.5 Sub-RLM cells: no

A sub-RLM cell is a per-assignment worker inside a desk's boundary
(engineering's cells run inside its capsule). It shares the desk's
boundary and is ephemeral. Its identity is `assignment_id` and a cell
index, as attributes. A key would be usable by the very model code it is
meant to identify.

### 2.6 Models and model families: no

A model is not a boundary at all: it is a service the gateway calls. A
model cannot hold a key, and any key "for" a model would be held by the
runtime that called it. `model_id` (`<provider>/<model>` of the answering
call, added to desk records in 8f8cc6848 and excluded from replay digests)
is a slice dimension of a track record, signed inside the record. This
keeps the lead's requirement: a desk's calibration survives a model swap,
and anyone can still read calibration per model.

### 2.7 Trajectories: no

A trajectory is a durable work identity on the tape (`trajectory_id` in
every event, `internal/computerevent/event.go:87`). It is a *subject* of
records, not a party that signs them. Its integrity comes from the event
chain and corpusd's head receipts, which already bind every event to its
sequence and predecessor.

### 2.8 Capsules: no

A capsule is a real boundary (namespaces, seccomp, Landlock, broker), but
what runs inside it is engineering's model-written code, which is the
thing being verified. A capsule key would let that code vouch for its own
results. Capsule outputs are verified by evidence (frozen bundle digests,
rule settlements, the host builder's rebuild), not signed by the capsule.

### 2.9 API keys: bearer delegations, never signers

An API key (`choir_sk_`) is the owner's delegation to a process. It is a
bearer secret and proves possession, not presence. It may authenticate an
owner act in S1, and the owner-act receipt records `auth_method: api_key`
and the key id, so a verifier or policy can refuse it. After S3, the
consequential acts on owner and persistent computers refuse API keys
(open decision 1).

## 3. Signed-act formats

### 3.1 One envelope

Every new signed act is a `computerevent.Receipt`
(`internal/computerevent/receipt.go`). The envelope is already right:

- **Canonical encoding:** RFC 8785 JSON with integers only; floats are
  refused so number rendering cannot enter a preimage
  (`internal/computerevent/canonical.go:12-15`).
- **Preimage:** `"choir-receipt-v1\0" || sha256(JCS(payload))`
  (`receipt.go:19,258-263`). The payload includes `receipt_kind`,
  `issuer`, `issued_at` and `required_signers` (each with its
  `signer_domain` and `key_id`), so the kind and the signer's domain are
  inside the signed bytes.
- **Identity and time:** a UUIDv7 `receipt_id`, a canonical UTC
  `issued_at` (`receipt.go:53-92,173-188`).
- **Key resolution:** `ResolveReceiptKey(domain, computerID, keyID,
  sequence, issuedAt)` (`receipt.go:49-51`), which is already the right
  shape for a rotation-aware key history.

### 3.2 Domain separation

Domain separation is by the pair (`receipt_kind`, `signer_domain`), both
signed, plus one rule every verifier follows: **a verifier accepts a kind
only from the signer domains its allowlist names for that kind.** corpusd
accepts `OwnerActReceipt` only from `host-owner-act`; a
`ConsensusBallot` for the `capsule-verifier` seat only from
`host-builder`; a `ConsensusBallot` for the `independent-reviewer` seat
only from the computer's `guest-core` (certified realization key) with
`desk: management` inside. The guest receipt signer already enforces the
mirror image on its side: it signs only an allowlist of kinds
(`HealthReceipt`, `MaterializationReceipt`, `UpdaterRecoveryReceipt`,
`KernelCapabilityReceipt`, `ExecutionIdentity`), pins the issuer per kind,
refuses a foreign `computer_id` and an `issued_at` outside −10/+1 minutes
(`internal/receiptsigner/receiptsigner.go:80-109`). S2 adds
`ConsensusBallot` to that allowlist and nothing else. A desk act and a
runtime fact are different kinds, so a desk cannot obtain a runtime-fact
signature by asking for a desk-act one.

Two existing signatures do not follow this envelope and should not be
copied: `selfdevprotocol.AuthorityReceipt` signs `JCS(receipt)` with no
domain prefix (`internal/selfdevprotocol/control.go:172-181,326-364`), and
the capsule `Capability` signs plain `json.Marshal` output with no prefix
(`internal/capsule/capability.go:27-76`). Neither collides with a
prefixed preimage today (prefixed preimages begin `choir-`, these begin
`{`), but both should migrate to the common envelope when next touched,
and every new kind in this report uses it. Likewise the credential
envelope derives its bearer and nonce by HMAC keyed with the platform
Ed25519 *seed* (`internal/platform/credential_envelope.go:391-398`); new
derived secrets should use HKDF from a separate key, not a signing seed.

The passkey format (S3) is WebAuthn's, not ours, and gets its own domain
inside the challenge: `challenge = sha256("choir-owner-act-v1\0" ||
JCS(act_body))`. A login challenge is random bytes and never parses as
that prefix, so a login assertion can never be replayed as an act, and the
reverse.

### 3.3 Common act fields

Every act receipt (owner act, ballot, consensus decision) carries, in
addition to the envelope:

- `act_kind` (for example `selfdev.mode.set`, `selfdev.decide`,
  `mail.send`, `release.adopt`, `key.revoke`);
- `principal`: `{kind, id}` (`owner`/user id, `computer`/ComputerID,
  `seat`/policy seat id) and, for owner acts, `auth_method`
  (`session`, `api_key`, `passkey`) and what identifies the credential:
  for a session, the JWT's `iat` and the sha256 of the token (the JWT
  carries no session id or `jti` today, `internal/auth/handlers.go:285-298`);
  for an API key, its key id; for a passkey, the credential id;
- `audience`: the verifier and the computer it is for (`corpusd`,
  `computer_id`), so a receipt minted for computer A is refused for B;
- `subject`: the exact digests the act is about (operation id, bundle
  digest, desired and effective heads and state commitments for a decision;
  draft id and version hash for a send; release id and adopter head for an
  adoption; policy digest for a mode);
- `nonce`: 128 random bits;
- `not_after`: an expiry (minutes for an owner act; the policy's validity
  for a standing mode);
- for passkey acts: the raw WebAuthn assertion (`authenticatorData`,
  `clientDataJSON`, `signature`, credential id), so any holder of the
  owner's public key can re-verify it later.

### 3.4 The certificate

A key certificate is a receipt of kind `KeyCertificate` signed by the
parent key: `{subject_kind (computer | realization | desk | service),
subject_id, computer_id, signer_domain, public_key, activation_sequence,
not_after, parent_key_id}`. The platform signs the computer's certificate
(the existing `VerifierKeyPinned` pattern,
`internal/platform/checkpoints.go:108-198`, generalized); the computer key
signs realization certificates; `desk` is reserved for S5. Rows land in
`control_key_history`, whose columns already carry activation and
invalidation (`internal/platform/control_keys.go:16-117`).

### 3.5 Replay protection

Four mechanisms, each for a different replay:

- **Within an act's lifetime:** `not_after` plus a corpusd
  `consumed_act_receipts` table keyed by `receipt_id`, written in the same
  transaction as the effect it authorizes. A second use returns the
  original result if the request is identical (idempotent retry) and is
  refused otherwise.
- **Across computers:** `audience.computer_id` must equal the event's
  `computer_id`.
- **Across subjects:** the subject digests must equal the event's
  `ParentEventID`, `ProposedEffectRef` and expected heads, which the
  existing decision binding already checks guest-side
  (`internal/agentcore/self_development_decision_binding.go:48-53`); S1
  repeats the check where it counts, in corpusd.
- **Across kinds:** the (`receipt_kind`, `signer_domain`) allowlist and
  the WebAuthn challenge prefix.

The event chain gives a fifth for free: an operation can be accepted once,
because the reducer will not accept a second decision for an operation
already decided.

### 3.6 What verifies what, and where

- **corpusd, at event append.** Today `ComputerEventCAS.CompareAndSwap`
  (`internal/platform/computer_events.go:184-280`) checks the commitments,
  the pins, idempotency, the shared reducer's projection and the event
  digest, then signs the head receipt. It sees the whole event, including
  `EventKind`, `AuthorityRef`, `DecisionRef` and `InputArtifactRefs`
  (`internal/computerevent/event.go:77-109`), but never asks who authorized
  an `effect_accepted`. S1 adds that check here, before the head receipt is
  signed. This is the only verification that binds: it runs outside the
  guest, before the event exists.
- **corpusd, at mode write.** S1: an owner mode write must carry an
  `OwnerActReceipt` from the proxy.
- **corpusd's actuator, at decision time.** S2: verifies ballots against
  the policy and mints the `ConsensusDecisionReceipt`.
- **The guest reducer and decision code.** Keep verifying what they verify
  today (`verifyDecisionModeReceipt`, `verifyConsumedModeReceipt`), as
  defense in depth and for a legible refusal before append. They are not
  authority: the guest is the subject. **No replay-time rule is added**
  (problem record compatibility item 13): the admission check happens once
  at append, and the head receipt's signature is the durable proof that it
  passed.
- **Other computers (adoption) and outside readers.** S2 adds
  `authority_receipt_digest` to the head receipt fields of an
  `effect_accepted` event, so an adopter reading a release record can
  follow: release record → publisher's `effect_accepted` head receipt
  (platform-signed) → authority receipt (owner act, consensus decision or
  offer) → ballots → builder receipt. The adopter verifies each signature
  against the published key history; it re-verifies the passkey assertion
  itself when the authority is an owner act signed by passkey.
- **maild.** S3: the send route verifies the owner-act receipt (or the
  passkey assertion) for `mail.send` over `(draft_id, version_hash)`.

## 4. Key lifecycle

**Generation.** The owner's passkey is generated in the authenticator at
registration. Host keys are generated by a provisioning step, stored
root-only 0600 and delivered by `LoadCredential=` to their one service;
the platform-control key is still auto-generated on first start
(`internal/platform/signing.go:42-75`; the comment says production should
provision a backed-up key). The computer identity key is generated in the
guest at genesis by the guest signer (S4); realization keys are generated
per disk, as today.

**Certification chain.**

```text
owner passkey ─────────────────────────────── (registered in auth; S3 publishes to corpusd)
platform-control (repo-pinned trust root)
  ├── host-owner-act    (KeyCertificate, subject_kind=service)
  ├── host-builder      (KeyCertificate, subject_kind=service)
  └── computer identity key   (ComputerKeyPinned at genesis; S4)
        ├── guest-core realization key       (KeyCertificate, per boot/disk)
        ├── verifier-control realization key (KeyCertificate)
        └── desk key (reserved, subject_kind=desk; S5, conditional)
```

Until S4, the guest-core and verifier-control keys are pinned on first use
by platform-control. The verifier key already is
(`checkpoints.go:108-198`); guest-core is registered nowhere, and every
verifier of a guest-core receipt uses the public key the guest presents
(`internal/updater/client.go:231`, `internal/proxy/execution_identity.go:148-201`).
S2 pins guest-core on first use the same way, which is enough for test
computers in Station A.

**The trust root must actually be checked.** platform-control's public key
is pinned in the repo (`internal/computerevent/appender.go:1098-1115`),
but only the proxy (`internal/proxy/handlers.go:347`) and the CLI
(`cmd/choir/main.go:47`) compare against it. The guest verifies its boot
envelope against the key *embedded in the envelope*
(`internal/platform/credential_envelope.go:72-91`,
`internal/selfdev/credentials.go:63-72`) and then trusts that key for
every receipt; vmctl takes the key from corpusd's `GET
/internal/platform/control-key` (`internal/vmctl/ownership.go:1514-1544`).
Against a host attacker this changes nothing (out of scope), but it means
no guest has a trust anchor of its own, and an adopter in S2 needs one to
verify another computer's chain. The guest image should carry the pinned
key and its successors (from `control_key_history` with the overlap
policy) and refuse an envelope signed by anything else.

**Rotation.** A computer key rotates by a `key_rotated` event signed by the
old key and countersigned by platform-control; the event kind is declared
and never appended today (2026-10-11 study, §1.4). Host service keys rotate
by a new certificate with an overlap window, under the "reviewed overlap
policy" `computerevent/appender.go` asks for. The owner rotates by
registering a new passkey and revoking the old.

**Revocation.** A revoked key gets `first_invalid_sequence` and
`first_invalid_time` in `control_key_history`; the resolver already refuses
signatures past those points. Who may revoke a computer key is open
decision 3 (default: owner passkey plus platform, platform alone in an
emergency with a visible event). Revoking a passkey in auth writes the same
row for the owner key.

**Fork re-keying.** A fork is a new computer with a freshly minted identity
(ontology, owner-ratified 2026-10-01; S9). Before any child runtime or
network starts, the child generates its own computer key; the platform pins
it with a `ComputerKeyPinned` that names the parent checkpoint in its
lineage field; the parent's key and realization keys are never copied, and
on the whole-disk path they are scrubbed before boot. The parent may sign a
lineage statement ("C2 forked from my checkpoint K"), which the owner act
that requested the fork authorizes. Results return only through
publication and adoption, so no parent key ever needs to verify inside the
child.

**Restore.** Keys are identity, not state, so they are outside the restore
set. A product restore is a forward event on the same chain, so
`control_key_history` keeps its rows and the resolver keeps refusing
revoked keys after a restore to a head that predates the revocation. A
restore never re-activates a key. A machine snapshot copies RAM, including
loaded keys, so the one-shot snapshot rule (the app-development roadmap's
S3 station, not this report's S3) must land before any memory snapshot of
a keyed computer is resumed twice. The privacy key already needs the same
rule; snapshots today hold it in plaintext RAM for up to 36 hours
(isolation review §3.8).

**Lost disk.** The computer key comes back from escrow through the same
path as the privacy key; realization keys are re-minted and re-certified.
No existing computer ever mints a new identity key silently.

## 5. How it closes each known gap

**`effect_accepted` authorization (the residual of the owner-only fix).**
corpusd refuses an append whose `EventKind` is `effect_accepted` unless its
authority verifies, by `AuthorityRef` prefix:

- `platform-control:update` (`internal/agentcore/platform_update.go:180-195`;
  today a string the guest writes): `DecisionRef` must be the digest of a
  platform update offer corpusd itself minted for this computer and head
  (corpusd already signs offers, `internal/platform/platform_update.go:136`).
- `external-owner:<user>`: the first input artifact must be a ModeReceipt
  signed by platform-control with `old_mode: accept_once`,
  `mode_writer: guest-consumption` (the field the owner-only fix adds), and
  consumed operation and bundle equal to the event's `ParentEventID` and
  `ProposedEffectRef`; and the arming ModeReceipt it consumed must carry an
  `OwnerActReceipt` minted for this computer. corpusd does not re-derive
  ownership: the proxy's signature attests that its ownership check (vmctl
  `LookupComputerContext`, or an API key bound to the computer) passed.
  Today the
  `<user>` after `external-owner:` is copied by the guest from the
  `X-Authenticated-User` header it trusts on any loopback caller
  (`internal/agentcore/api_self_development.go:596-601`,
  `internal/server/host_caller.go:31-61`), and corpusd never reads it
  (`self_development_decision_binding.go:40-46` checks it guest-side
  only); after S1 it must equal the owner-act receipt's principal.
- `qualified-consensus:<digest>` (`internal/decisionpolicy/types.go:7`):
  refused until S2; after S2, the digest must name a
  `ConsensusDecisionReceipt` corpusd's actuator minted.

Anything else is refused with a typed reason and no head receipt. This is
append-time only, keyed on an activation sequence per computer, so history
is not re-judged.

**Mode writes.** The owner-only fix moves authority from "holds
`event:append`" to "is host-internal". S1 moves it from "is host-internal"
to "carries an owner-act receipt signed by the proxy's key", and records
the owner, the method and the receipt digest in the ModeReceipt (a new
`owner_act_receipt_digest` field next to `mode_writer`). A loopback process
without the proxy's key can no longer arm a mode, even while hostauth is in
observe. Expiry writes and guest consumption keep their current rules.

**Admin keys.** Three different things are called admin keys, and the
design treats each differently. The API-key `admin` scope (it satisfies
`computer:self_development:mode` and `:approve` at the proxy,
`internal/proxy/self_development.go:140-153,349-353`) remains a bearer
delegation: the owner-act receipt records it as `auth_method: api_key`,
and after S3 the consequential acts refuse it on owner and persistent
computers. The escrow operator tokens (`X-Choir-Operator`, gated by headers
alone, `internal/platform/key_escrow_http.go:297-309`) are a host operator
credential, not an owner act; they need the hostauth transport check the
surface audit asks for, and later an operator passkey, but they are outside
this report's critical path. The hostauth shared secret is a host-service
credential, not a principal's key; it stays, with the residual its
problem record names (one secret for every caller).

**maild G7.** A host-served approval origin (default
`https://approve.choir.news`, served by Caddy from a host-only bundle and
routed to the proxy, auth and maild, never to a guest), with
`frame-ancestors 'none'` and an `Origin` check on every act endpoint. The
page renders the draft from maild, not from the guest, so the owner sees
what will be sent. The approval itself is a passkey assertion over
`mail.send(draft_id, version_hash)` with user verification required; maild
(or corpusd on maild's behalf) refuses any assertion whose
`clientDataJSON.origin` is not the approval origin, which is exactly what
an assertion requested by guest-served JavaScript carries. The session
cookies are host-only on `choir.news` (`internal/auth/handlers.go:325-345`,
no `Domain`), so the approval origin does not inherit the owner's session
and needs none: each act is authenticated by its own assertion. The E1
caller-class repair stays: guests draft and request; only an owner act
sends.

**Release record signing.** The record (selfdev architecture §1.1) carries
three signatures by three boundaries, each attesting only what it knows:
the **host builder** over the build (closure from base plus patch, and the
test run); the **publisher computer** over the record's content and
evidence (who made it; S4, until then platform-witnessed); and
**platform-control** over the publication (the object was published by
computer C at head H, no later than T). Under the owner's ruling only
source is published and the host is the sole signed builder, so the
builder signature is the one an adopter's bytes depend on.

**Cross-computer adoption.** The adopter's gate verifies, before its state
changes: the builder signature over the closure it will run (rebuilt for
its own base); the publisher's `effect_accepted` head receipt and its
authority receipt (S2's `authority_receipt_digest`), so it can see whether
the release was owner-approved or consensus-approved and under which
policy; and the adoption request's own owner act (Station B: an owner act
per adoption). A record signature replayed as a platform offer is refused
by kind and domain.

**Not closed by this design, named.** The guest's own loopback still
admits any in-guest process as the owner on guest routes
(`HostSourcedCaller`, surface audit §4); the fix is the proxy signing the
identity headers it forwards (isolation review §4.6), which can reuse the
`host-owner-act` key with a per-request HMAC or signature, as a separate
orange/red slice. Cross-tenant object reads, the corpusd event-read bypass
and pprof are authorization bugs, not identity gaps.

## 6. Stations

### S1: owner-act receipts and the effect admission gate

**On Station A's critical path:** Station A's one owner act is arming the
standing mode, and its claim "no guest self-acceptance" is only true if
corpusd, not the guest, decides what an acceptance needs. Depends on the
owner-only mode fix (6bee3d318) landing first.

Spec:

1. **Proxy key.** Generate an Ed25519 key, root 0600, delivered to the
   proxy only via `LoadCredential=`; signer domain `host-owner-act`.
   corpusd learns its public key through a platform-signed
   `KeyCertificate` row, or a pinned config value in the first slice.
2. **`OwnerActReceipt`.** The proxy mints one on every owner mode write
   (`internal/proxy/self_development.go:125-213`) and every owner decision
   it forwards, after its existing ownership check: `{act_kind, principal:
   {owner, user_id, auth_method, credential (§3.3)}, audience: {corpusd,
   computer_id}, subject, nonce, not_after (+5 min)}`. It sends the receipt
   to corpusd with the request.
3. **corpusd mode write.** A host-internal mode write without a valid
   `OwnerActReceipt` whose subject matches the requested mode is refused;
   the ModeReceipt records `owner_act_receipt_digest`. Rollout: log-only
   for one deploy, then enforce, like hostauth.
4. **corpusd admission gate.** In `CompareAndSwap`, before signing the head
   receipt, an `effect_accepted` event must pass the §5 rules for its
   `AuthorityRef`; `qualified-consensus:` is refused until S2. A refusal returns a
   typed 403 and no receipt. Receipts used are recorded in
   `consumed_act_receipts`.
5. **An issuance log, not the payload.** corpusd keeps only the current
   ModeReceipt per computer today (`computer_self_development_modes`,
   `mode_receipt_json`). The event's input artifacts are pinned
   (`internal/platform/event_artifacts.go:92-140`), but an owner-class
   payload may be ciphertext under the guest's privacy key, so the gate
   must not depend on reading it. S1 adds an append-only
   `self_development_mode_receipts` table (digest → receipt), written in the
   same transaction as each mode write; the gate looks up the event's first
   input artifact digest there. A receipt corpusd never issued is refused
   by construction. The log also closes the chain the decision needs: the
   event cites the *consumed* receipt (`mode_writer: guest-consumption`,
   `old_mode: accept_once`), and the gate follows it to the arming receipt
   of the previous generation, which must be `host-internal` with an
   `owner_act_receipt_digest` for this computer.

Mutation class **red** (approval authority, canonical event acceptance,
a new host key). Size: S to M (one key, two corpusd checks, one proxy
change, one table).

Acceptance probes (staging, on a freshly minted test computer):

- the owner (tracer, by API key on a test computer) arms `accept_once` and
  approves; the operation reaches `applied`; the ModeReceipt names the
  owner-act receipt digest;
- a guest-side append of an `effect_accepted` with `external-owner:x` and a
  fabricated or stale ModeReceipt is refused by corpusd, with the reason in
  corpusd's log. Admissible evidence: an in-process corpusd handler test on
  the embedded Dolt store, as the owner-only fix used, plus a staging probe
  from the test computer's terminal with the guest's own capability
  (UNVERIFIED that the terminal user can read the capability; if not, the
  in-process test stands, as in the problem record);
- an append citing a consumed ModeReceipt for a different operation is
  refused;
- a loopback `curl` to corpusd's mode route with the hostauth header but no
  owner-act receipt is refused (enforce stage);
- a replayed owner-act receipt (same `receipt_id`) for a second mode write
  is refused;
- the platform update path still applies (one LU or operator offer on a
  test computer).

Rollback: set the gate to log-only (a corpusd environment flag), or
`git revert`. The new ModeReceipt field and table are additive.

### S2: signed ballots and the consensus decision receipt (P4's signatures)

**On Station A's critical path**, after desk-cell confinement
(`fix/desk-cell-confinement`) lands and after S1. Station A's acceptance
requires "two signed verification ballots" and an authority reference
`consensus:<receipt>` (in code, `qualified-consensus:<digest>`).

Spec:

1. **`ConsensusBallot` kind**: `{policy_digest, subject_binding_digest,
   seat, verdict, evidence_refs, computer_id, desk, model_id, nonce}`,
   replacing the hash-only attestation in `internal/decisionpolicy`.
2. **The `capsule-verifier` seat is signed by `host-builder`.** The host
   build service (P5) rebuilds the frozen bundle's tree and runs its
   declared test command in its sandbox, then signs the ballot. This makes
   one of the two seats a party outside the guest: a compromised guest
   cannot reach quorum alone. (UNVERIFIED: whether the builder's sandbox
   can run the repo's Go tests within its timeout; if not, the first slice
   signs the build and the rule settlements, and the test run follows.)
3. **The `independent-reviewer` seat** is management's Verify verdict,
   signed by the guest-core realization key with `desk: management` and
   `model_id` inside. Its independence from engineering is attribution
   inside one guest: honest after confinement, weak against guest root.
   That is the named residual the selfdev report already accepts for test
   computers.
4. **The actuator in corpusd** collects ballots, checks each against the
   allowlist and the policy (`decisionpolicy.Reduce`), and mints a
   `ConsensusDecisionReceipt` signed by platform-control. The guest appends
   `effect_accepted` with `qualified-consensus:<digest>`, and S1's gate
   verifies it. The head receipt gains `authority_receipt_digest`.
5. **Keys it needs:** the host-builder key (with P5's build service, also
   on Station A's path) and a first-use pin of the guest-core key in
   `control_key_history`, reusing the verifier-control pin.

Class **red**; size M. Acceptance: Station A's own probes (its positive leg
and its two negative legs) plus: a ballot signed by guest-core for the
`capsule-verifier` seat is refused; a ballot whose `subject_binding_digest`
names another operation is refused; a decision receipt for computer A is
refused on computer B. Rollback: disarm the standing mode (owner act);
revert. No history depends on it.

### S3: passkey-signed owner acts on a host-served approval origin

Not on Station A's path; required before autonomy or adoption on a
persistent computer, and the only repair of G7. Red (auth and session,
approval authority); size M.

What exists to build on: go-webauthn v0.16.4 with registration requiring
user verification (`internal/auth/handlers.go:476-582`); a challenge table
keyed by challenge with a 5-minute TTL and delete-on-use
(`internal/auth/store.go:52-61,665-695`); the library's
`webauthn.WithChallenge` and `WithUserVerification` login options. What
does not: any step-up or transaction flow (`BeginLogin` and
`ValidateLogin` have one call site each, `handlers.go:663,949`); the
challenge table's `type` admits only `registration` and `login`; login
does not require user verification (no `WithUserVerification`, so the
library checks presence only); the sign count update is non-fatal and
`CloneWarning` is never read (`handlers.go:956`).

Spec:

1. **Approval origin.** `approve.choir.news`, a host-only static bundle
   served by Caddy; act endpoints check `Origin`; `frame-ancestors 'none'`;
   no session cookie. An approval link carries an act id and a one-time
   view token minted by the host when an approval is requested (by the
   owner's Desktop or by the guest); the page fetches the act summary from
   the host service that owns the act (maild for a draft, corpusd for an
   operation), never from the guest.
2. **Act challenges.** A new `challenge_state.type = 'owner_act'`; auth's
   `act/begin` computes `challenge = sha256("choir-owner-act-v1\0" ||
   JCS(act_body))` and calls `BeginLogin` with `WithChallenge` and
   `WithUserVerification(required)`, restricted to the owner's
   credentials.
3. **Verification.** auth's `act/finish` validates the assertion (rpId,
   `origin == https://approve.choir.news`, UV flag, challenge, sign count
   with clone detection fatal for acts) and the proxy mints the
   `OwnerActReceipt` with `auth_method: passkey` and the raw assertion
   embedded. corpusd and maild re-verify the assertion against the owner's
   credential public keys, which auth publishes to corpusd's owner key
   history on registration and revocation, so a compromised proxy alone
   cannot mint a passkey-method receipt.
4. **Acts moved first:** mail send (G7), then self-development decide and
   mode set on owner and persistent computers, adoption into a persistent
   computer, minting an API key with an owner-act scope, and computer key
   revocation.

Acceptance: the owner approves a send and a self-development decision on
the approval origin with a passkey (a Playwright virtual authenticator for
the repeatable artifact); an assertion requested from the guest-served
origin is refused for origin mismatch; a cookie-only request to a
passkey-required act is refused; an assertion without the UV flag is
refused; a replayed assertion (same challenge) is refused; a revoked
passkey's assertion is refused. Rollback: a per-act policy flag returns
the act to session-attested S1 receipts.

### Later

- **S4: computer identity keys** (red, M to L): genesis key, platform pin,
  escrow and delivery, realization certificates, published key history in
  Store B, `key_rotated`, revocation. Required before cross-owner adoption,
  public track records (`computer-signed+platform-witnessed/v1`) and any
  fork (S9). Not on Stations A to C, which run on test computers of one
  owner and rely on platform witnessing for attribution.
- **Proxy-signed identity headers to the guest** (orange/red, S to M):
  closes the in-guest loopback-as-owner gap. Independent of S1 to S3.
- **Platform key custody** (red operations, S): provision and back up the
  platform-control key and write its overlap policy. Its importance grows
  with every receipt kind it signs; do it before S2.
- **A guest trust anchor** (orange, S): the guest image carries the pinned
  platform-control key and refuses an envelope signed by anything else;
  vmctl does the same instead of trusting corpusd's control-key route.
  Before Station B, whose adopter must verify another computer's chain.
- **Hygiene, when next touched** (orange, S): derive the envelope's bearer
  and nonce from an HKDF key rather than the platform signing seed; move
  `AuthorityReceipt` and the capsule `Capability` to the prefixed receipt
  envelope.
- **S5: desk keys** (conditional, §2.4).

Order: hostauth enforce and the owner-only fix (both in flight) → S1 →
desk-cell confinement → S2 → Station A → S3 → S4 before Station B
reaches a persistent or cross-owner computer.

## 7. Open decisions

1. **API keys as owner acts.** Default: allowed on test-tier computers,
   recorded as `auth_method: api_key`; after S3, refused for the
   consequential acts on owner and persistent computers. The tracer keeps
   working on test computers.
2. **Who signs the S1 owner-act receipt.** Default: the proxy, with its own
   key, because it is the party that authenticated the owner and because it
   makes the owner's authority independent of loopback. The alternative,
   corpusd signing with platform-control on the proxy's host-internal call,
   saves one key and keeps the dependence on hostauth.
3. **Who may revoke a computer key.** Default: the owner's passkey plus the
   platform; the platform alone in an emergency, with an owner-visible
   `key_revoked` event.
4. **The approval origin.** Default: `approve.choir.news`, host-served,
   passkeys kept under `rpId` choir.news so existing registrations work;
   the assertion's origin, not the rpId, is what separates it from the
   guest.
5. **Desk keys.** Default: none until the three S5 conditions hold; the
   certificate slot is reserved now.

## Conformance with settled decisions

- **C14 (a human is a policy-selected seat).** Kept: S2 makes consensus a
  verifiable authority, and owner acts are one authority kind among three.
- **Ontology: forks get freshly minted identity material; results return
  only through publication and adoption.** Kept (§4).
- **Restore-set boundary.** Keys are outside restore; restore is forward
  (§4).
- **Owner distribution ruling: source only, host the sole signed builder.**
  Made concrete: the builder gets a key (§2.2).
- **Re-engineering 2026-10-12: per-computer, per-desk, per-sub-RLM signing
  deferred as a non-functional requirement.** Kept for desks and sub-RLMs;
  computer keys stay deferred to S4. S1 and S2 add only host keys and one
  new kind on an existing guest signer, because those close functional
  authority gaps, not non-functional ones.
- **precommitment-records §6.** Adopted: one computer identity key, no
  host-held per-computer key, platform witness on v0 records, never
  re-sign.
- **CLAUDE.md restart rules.** Untouched.

## Unverified claims, collected

- That the host builder's sandbox can run the declared Go test command
  within a usable timeout (S2 item 2).
- Whether an `effect_accepted` event's pinned ModeReceipt payload is
  plaintext to corpusd. S1 avoids the question with its own issuance log
  (S1 item 5).
- Whether the guest terminal user can read the guest's event capability,
  which decides whether S1's negative probe can run on staging or only
  in-process.
- That every guest key on `/mnt/persistent` travels with a copy of
  `data.img` (inferred from storage paths; no fork or copy was made).
- That the platform-control key has no backup (the repo shows none; Node B
  was not inspected).

## Class, heresy delta, residuals

Class of this report: green. Protected surfaces the proposed work touches:
canonical event acceptance (S1, S2), approval authority (S1, S3), auth and
session (S3), a new host key and a new host signing surface (S1, P5),
Trace and evidence receipts (S2).

Heresy delta:

- **Discovered.** corpusd signs a head receipt for any `effect_accepted`
  the guest appends without checking its authority; the
  `external-owner:<user>` actor is copied from a header the guest trusts
  from any loopback caller. The guest has no trust anchor: it trusts the
  platform key embedded in its own boot envelope, and vmctl trusts
  corpusd's answer. Passkey login does not require user verification. The
  credential envelope's bearer is an HMAC keyed by the platform signing
  seed. `AuthorityReceipt` signatures carry no domain prefix. The host
  builder's receipt is unsigned, so "the host is the sole signed builder"
  has no signer yet. The guest runtime, as root, can reach both guest
  signers, so guest-signed ballots are as strong as guest root until the
  runtime is non-root. A runtime-held desk key would be a mislabeled
  authority.
- **Introduced.** None.
- **Repaired.** None (design only).

Residual risks:

- Host compromise forges everything except past passkey assertions.
- Seat independence inside one guest is attribution, not separation.
- A stolen session cookie remains the owner for every act not yet behind
  S3.
- The platform-control key is auto-generated and, as far as the repo shows,
  not backed up; every new receipt kind raises its value.
