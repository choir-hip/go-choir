# corpusd Admits `effect_accepted` Without Checking Its Authority

Date: 2026-10-15. Source: S1 of
[principal-identity-and-signed-acts-2026-10-15.md](../archive/reports/principal-identity-and-signed-acts-2026-10-15.md)
(§0 "`effect_accepted` is not authorized at append", §5, §6 S1), and the
residual named in
[guest-can-arm-own-selfdev-mode-2026-10-15.md](../archive/problems/guest-can-arm-own-selfdev-mode-2026-10-15.md)
("Heresy delta", second line). Owner rulings: the report's defaults are
accepted ([owner-notes-2026-10-14.md §19](../reports/owner-notes-2026-10-14.md)):
the proxy signs S1 owner-act receipts; passkey acts on `approve.choir.news`
come later (S3). Code read at origin/main 133de45fa, which carries the
owner-only mode fix (`mode_writer`, `ConsumeAsGuest`) and the `owner:acts`
API-key scope. Status: **design and failure modes recorded; build follows
on branch `feat/s1-owner-act-receipts`** (not pushed, not deployed).

This commit is docs only. The code commits that follow cite it.

## The problem in two sentences

corpusd signs an event-head receipt for any `effect_accepted` the guest
appends with its own `event:append` capability, without reading the event's
`AuthorityRef`, its ModeReceipt or any consensus receipt, so the only check
that an acceptance was authorized runs inside the guest that is its subject.
And the owner's authority to arm the mode that an acceptance consumes rests
on "host-internal caller", which today means "loopback plus, once hostauth
enforces, one shared secret", not on any record of the owner having acted.

## The trace (code at 133de45fa)

1. **Append.** `HandleComputerEventAppend`
   (`internal/platform/event_handlers.go:236-256`) authorizes `event:append`
   and calls `ComputerEventCAS.CompareAndSwap`
   (`internal/platform/computer_events.go:184-282`). CAS checks the pin
   intent, request commitment, pins, idempotency, the shared reducer's
   projection and the event digest, then signs `EventHeadReceipt`. Of an
   `effect_accepted`, the reducer asks only for no pending transition, a
   target commitment, `ProposedEffectRef`, `DecisionRef` and
   `VerifierRefs` (`internal/computerevent/reducer.go:82-91`). Nothing
   reads `AuthorityRef` or the input artifacts.
2. **Two producers, both in the guest.** The owner decision
   (`internal/agentcore/api_self_development.go:257-601`) appends with
   `AuthorityRef` `external-owner:<X-Authenticated-User>` or
   `qualified-consensus:<digest>`, and the consumed ModeReceipt as input
   artifact 0 (non-private; its artifact digest is
   `sha256(ModeReceipt.CanonicalBytes())`, the same digest corpusd stores as
   `mode_receipt_digest`). The platform update
   (`internal/agentcore/platform_update.go:180-195`) appends with
   `platform-control:update`, `DecisionRef` = the offer digest and
   `ProposedEffectRef` = the manifest content digest.
3. **corpusd keeps no history it could check against.** It keeps only the
   current ModeReceipt per computer (`computer_self_development_modes`), so
   it cannot tell whether a cited ModeReceipt digest is one it issued. It
   signs platform update offers (`internal/platform/platform_update.go:135-145`)
   and records nothing about them.
4. **The owner's mode write carries no owner record.** The proxy
   authenticates the owner (session plus vmctl ownership, or an API key
   with `computer:self_development:mode` and `owner:acts`;
   `internal/proxy/self_development.go:125-213`) and calls corpusd as a
   host-internal caller (`hostauth.MarkInternal`). corpusd's `Set` writes
   `mode_writer: host-internal` and nothing about who acted. Any process
   that passes `trustedInternalCaller` arms a mode as the owner; hostauth is
   still in observe on staging, so that is any loopback process.

## Consequence

With the owner-only fix, a guest can no longer arm a mode, but it can still
skip the mode: it can append `effect_accepted` with
`external-owner:anyone` and a fabricated or stale ModeReceipt, or with
`qualified-consensus:<digest of ballots it minted>`, or with
`platform-control:update` and a digest corpusd never signed, and corpusd
will sign the head receipt. The materializer then applies the bundle. The
host-signed head receipt is the durable proof that an acceptance passed,
so today it proves nothing about authority. This is the self-approval loop
the report names, one level below the mode.

## Design (S1, as built)

Mutation class **red**: approval authority, canonical event acceptance, a
new host signing key and surface. Size S to M.

### 1. The proxy owner-act key

- One Ed25519 key, signer domain `host-owner-act`, issuer `proxy`.
  Generated at host activation (`nix/node-b.nix`, like the hostauth token)
  with `openssl genpkey -algorithm ed25519` into
  `/var/lib/go-choir/secrets/owner-act-signing-key` (PEM PKCS#8, root 0600)
  and its public key (`openssl pkey -pubout`, PEM SPKI, root 0600) beside
  it. The private key reaches **only** the proxy, by `LoadCredential=`
  (`choir-owner-act-key`); the public key reaches corpusd by
  `LoadCredential=` (`choir-owner-act-public-key`). The secrets directory
  stays hidden from every unit's mount view.
- Key id: `host-owner-act-` + the first 16 hex of sha256(public key). A
  rotation is a new key id; corpusd accepts exactly the configured one
  (a pinned config value, the report's first-slice option; the
  `KeyCertificate` row in `control_key_history` is later work).
- No key loaded: the proxy forwards without a receipt and logs it once at
  start. That is the pre-S1 behavior, and corpusd's mode decides what
  happens next.

### 2. `OwnerActReceipt`

A `computerevent.Receipt` (RFC 8785 canonical JSON, preimage
`"choir-receipt-v1\0" || sha256(JCS(payload))`, UUIDv7 `receipt_id`,
canonical UTC `issued_at`), so kind, issuer and signer domain are inside
the signed bytes. Kind fields:

| field | value |
|---|---|
| `act_kind` | `selfdev.approve` (arming `accept_once`: the owner's approval of exactly one operation and bundle) or `selfdev.mode.set` (every other mode) |
| `owner_user_id` | the authenticated user |
| `auth_method` | `session` or `api_key` |
| `credential_ref` | `session-jwt-sha256:<sha256 of the access JWT>` or `api-key:<key id>` (the JWT has no `jti`) |
| `audience` | `corpusd` |
| `computer_id` | the target computer (also the key resolver's computer) |
| `nonce` | 128 random bits, hex |
| `not_after` | `issued_at` + 5 minutes, canonical UTC |
| `subject` | `{mode, expected_generation, operation_id, bundle_digest, policy_digest, consensus_receipt_digest, request_commitment}` |

`request_commitment` is corpusd's own request commitment
(`sha256(JCS({computer_id} + SetSelfDevelopmentModeRequest))`), exported
so the proxy and corpusd compute it from the same code. It binds every
digest, head, commitment, expiry and the idempotency key of the write.
The receipt travels in the `X-Choir-Owner-Act-Receipt` header
(base64url of the canonical bytes), not in the body, so a retried PUT
with a fresh receipt keeps the same idempotency commitment.

**Approve.** The owner's approval reaches corpusd as the `accept_once` arm
(`choir self-dev approve` and the tracer both arm, then decide). The arm's
subject is the approval's subject: operation id, bundle digest, both heads,
the pending ref and both state commitments. The decision POST goes to the
guest, which is not a verifier, so S1 mints no separate receipt for it;
corpusd admits the decision by following the consumed arm (§4). A
decide-time receipt joins in S3, when the decision itself is passkey-signed.

### 3. Mode writes

- A host-internal POST must carry a valid `OwnerActReceipt`: signature by
  the configured `host-owner-act` key, kind `OwnerActReceipt`, issuer
  `proxy`, audience `corpusd`, `computer_id` equal to the route's,
  `act_kind` matching the requested mode, `subject.request_commitment`
  equal to the request's, `not_after` in the future, `issued_at` not more
  than one minute ahead.
- Single use: an `owner_act_receipts` table (primary key `receipt_id`,
  unique `nonce`) is written in the same transaction as the mode write. A
  second use of a receipt is a replay. An identical retry (same idempotency
  key and commitment) returns the stored mode before the receipt is looked
  at, as today.
- The ModeReceipt records `owner_act_receipt_digest`, `owner_user_id` and
  `owner_auth_method` (only when a receipt was verified; old receipts and
  guest/expiry writes are unchanged).
- **Issuance log.** Every mode write (owner, guest consumption, expiry)
  appends its ModeReceipt to `self_development_mode_receipts` (digest,
  computer, committed and previous generation, old/new mode, writer, owner
  act digest, receipt JSON) in the same transaction. A ModeReceipt corpusd
  never issued is absent by construction.

### 4. The admission gate on `effect_accepted`

In `CompareAndSwap`, after the idempotent-retry short-circuit (history is
not re-judged) and after the reducer and digest checks, before the head
receipt is signed. By `AuthorityRef`:

- `platform-control:update`: `DecisionRef` must be an offer digest in
  `platform_update_offers` (written by the mint handler when it signs),
  minted for this computer, with `content_digest` = `ProposedEffectRef`,
  `base_event_head` = the event's `PreviousHead`, not past its expiry plus
  two minutes, and not already admitted for a different event.
- `external-owner:<user>`: input artifact 0 must be a ModeReceipt in the
  issuance log for this computer, written by `guest-consumption`, `accept_once`
  → `propose_only`, whose `consumed_*` fields equal the event's
  `ParentEventID`, `ProposedEffectRef` and expected heads and commitments,
  not already admitted for a different event. Its previous generation's
  log row (the arm) must be `host-internal`, `accept_once`, with an
  `owner_act_receipt_digest` present in `owner_act_receipts` for this
  computer, whose `owner_user_id` equals `<user>`.
- `qualified-consensus:<digest>`: refused until S2.
- anything else: refused.

An admitted event marks its log row (`admitted_event_digest`) in the same
transaction, so one consumed grant or one offer admits one event.

### 5. Rollout and rollback

`CHOIR_OWNER_ACT_MODE` on corpusd: `observe` (default, also when unset)
or `enforce`; any other value is treated as `enforce` (fail closed, as
hostauth does). In observe, a missing, invalid or replayed receipt and an
untraceable acceptance are logged as `owner-act: observe: would refuse`
and allowed; the logs and the ModeReceipt's new fields still record what
passed. Rollback: set `observe` (one env line, corpusd restart), or
`git revert`. Every schema change is an additive table or field.

Deploy order: the deploy that ships this also generates the key at
activation and restarts both units in observe. Flip corpusd to `enforce`
after a clean window where the journal shows no `would refuse` line except
probes.

## Failure modes the build must handle

Refused in enforce (logged in observe), mode write:

1. host-internal mode write with no receipt (a loopback `curl` with the
   hostauth header);
2. receipt signed by another key, or with a broken signature, or unsigned
   (empty signature set), or with the right key under another signer
   domain or kind;
3. receipt for another computer, another audience, another act kind, or a
   subject whose `request_commitment` is not this request's;
4. receipt past `not_after`, or issued more than a minute in the future;
5. a receipt used for a second, different mode write (same `receipt_id`
   or same nonce);
6. enforce with no public key configured: every owner write refused.

Refused in enforce (logged in observe), `effect_accepted`:

7. `external-owner:x` citing a ModeReceipt corpusd never issued
   (fabricated, or signed before the log existed);
8. citing a log row that is not a guest consumption of `accept_once`
   (an arm itself, an expiry, a `propose_only` set);
9. citing a consumption whose `consumed_operation_id` or
   `consumed_bundle_digest` or heads differ from the event's (stale
   receipt, other operation);
10. a consumption whose arm has no owner-act receipt (armed before S1, or
    armed in observe without a receipt);
11. `external-owner:<user>` where `<user>` is not the arm receipt's owner;
12. a second event citing a consumption or offer already admitted for
    another event;
13. `qualified-consensus:*`;
14. `platform-control:update` with an offer digest corpusd never minted,
    minted for another computer, for another content digest or base
    head, or expired;
15. any other or empty `AuthorityRef`.

Still allowed:

16. every owner arm and set through the proxy with the key loaded
    (session that owns the computer, or an `owner:acts` API key);
17. the guest's two consumption writes and their idempotent retries (no
    receipt; writer `guest-consumption`), and corpusd's expiry write;
18. an identical retry of an owner write after it committed;
19. the owner decision that follows an S1 arm (tracer path:
    `scripts/qa_test_computer.mjs` keys with `owner:acts`, then
    `scripts/reeng_tracer.mjs` arms and decides);
20. a platform update from an offer minted after S1;
21. `effect_rejected` and every other event kind (not gated);
22. an idempotent retry of an append that was admitted before S1.

Compatibility:

23. existing ModeReceipts keep verifying; `verifyDecisionModeReceipt` and
    `verifyConsumedModeReceipt` read no new field;
24. replay is unchanged: the gate is append-time only, so no tape is
    re-judged and the guest reducer gains no rule.

## Ceremony

- **Conjecture delta.** Before: a host-signed `EventHeadReceipt` on
  `effect_accepted` meant "the event fit the chain". After (enforce): it
  also means "corpusd traced this acceptance to an authority it issued or
  verified: an offer it minted, or a ModeReceipt chain ending in an owner
  act the proxy signed". Conjecture: the proxy is the only party that can
  truthfully say an owner acted, so a signature only it can make, verified
  where the effect is admitted, removes the guest and every non-proxy
  loopback process from the set that can authorize an acceptance.
  Falsifier: an `effect_accepted` admitted in enforce whose chain does not
  end in a proxy-signed receipt or a corpusd-minted offer.
- **Protected surfaces touched.** Run acceptance (canonical
  `effect_accepted` admission), approval authority (mode writes), a new
  host key and signing surface (proxy), Trace/evidence (new append-only
  logs; ModeReceipt fields). Not touched: Texture canonical writes,
  checkpoint/route projection, vmctl, gateway/provider calls, auth/session
  issuance (the proxy reads the session; auth is unchanged), the replay
  reducer.
- **Admissible evidence.** In-process corpusd handler and CAS tests on the
  embedded Dolt store (where both decisions are made); proxy handler tests
  with a real auth store and a stub corpusd; agentcore decision tests for
  the guest side. Staging (after the Landing Loop, in observe): the
  tracer's owner approval reaches `applied` and the ModeReceipt names an
  owner-act digest; corpusd's journal shows no `would refuse` for it. The
  enforce-stage negative probes (a loopback mode write without a receipt;
  a guest append citing a fabricated ModeReceipt) are owed after the flip.
- **Rollback.** `CHOIR_OWNER_ACT_MODE=observe` (or unset), or
  `git revert` of the code commits; the tables and fields are additive and
  a revert leaves them unread.
- **Heresy delta.**
  - discovered: corpusd admits `effect_accepted` without reading its
    authority; corpusd keeps no record of the ModeReceipts or offers it
    issued; the owner's mode authority is a transport property.
  - introduced: none intended. A new key is a new custody burden (named in
    residuals).
  - repaired (on enforce): acceptance authority is verified outside the
    guest; owner mode writes carry a proxy-signed act; ModeReceipts and
    offers are logged append-only.

## Residuals

- In observe, nothing is refused; the repair is real only after the flip.
- A stolen session cookie or `owner:acts` key is still the owner; the
  proxy will sign for it. S3 (passkey acts) is the repair.
- A compromised proxy can sign any owner act. The key is per-service, so a
  corpusd or non-root host compromise cannot.
- `scripts/lib/selfdev_approval.mjs` arms modes by a direct loopback POST
  to corpusd; in enforce that path is refused by design (it is an
  operator harness for consensus probes, which are refused until S2
  anyway).
- The key is generated on the host and not backed up; losing it means
  generating a new one and restarting both units (no history depends on
  it: corpusd stores the digests and receipts it verified).

## Build (branch `feat/s1-owner-act-receipts`, not pushed)

- `internal/owneract`: the receipt format, the proxy `Signer` (PEM PKCS#8
  from `$CREDENTIALS_DIRECTORY/choir-owner-act-key` or
  `CHOIR_OWNER_ACT_KEY_FILE`) and corpusd's `Verifier` (PEM SPKI from
  `choir-owner-act-public-key` or `CHOIR_OWNER_ACT_PUBLIC_KEY_FILE`).
- `internal/proxy`: the mode PUT decodes the write, mints the receipt
  after the ownership check and sends it in `X-Choir-Owner-Act-Receipt`;
  `AuthResult.CredentialRef` names the session (JWT digest) or key id.
- `internal/platform/owner_acts.go`: `CHOIR_OWNER_ACT_MODE`, the three
  append-only tables (`owner_act_receipts`, `self_development_mode_receipts`,
  `platform_update_offers`), mode-write verification, and the
  `effect_accepted` gate called from `CompareAndSwap`. A refusal is a 403
  at the append route (`ErrEffectAdmissionRefused`) and at the mode route
  (`ErrOwnerActRefused`).
- `nix/node-b.nix`: the activation script generates the key pair; the
  proxy loads the private key and corpusd the public key by
  `LoadCredential=`; corpusd runs with `CHOIR_OWNER_ACT_MODE=observe`.

Tests: `internal/platform/owner_acts_test.go` (failure modes 1-17, 19-22
on the embedded Dolt store), `internal/proxy/owner_act_receipts_test.go`
(the session and `owner:acts` key paths mint a receipt that verifies and
binds the request commitment; proxy to real corpusd mode route in enforce:
the keyless proxy and a direct loopback write are refused, the keyed
proxy arms and the ModeReceipt names the owner act),
`internal/owneract/owneract_test.go` (keys made by the activation's
openssl commands load through the credential names and round-trip).
