# Platform operator account (2026-11-06)

Status: design, with slice O1 built beside it (branch `oper`). Class of the
design: green. Class of O1: red (a new authority on a protected surface,
package listing state). Author: Claude (Opus), from code at `dec92f21e`.

Owner ruling, 2026-11-06: "I need to be platform operator from my [personal]
account, and this also dovetails with the need for better control over the
autopaper vm (this is a noted todo; currently we are handling its keys in an
ad hoc way)."

The noted TODO is the modular-architecture ledger's residual
(`docs/definitions/choir-modular-architecture-2026-11-04.ledger.md`,
2026-11-05T18:35Z): the paper's owner acts depend on a test owner's saved
browser session and a 7-day API key renewed by hand with
`scripts/qa_renew_key.mjs`; "Successor: a platform operator identity for
platform-owned computers (server-held, owner:acts scoped to those computers,
every act on the tape) ... no secret on a developer machine."

## 1. Today

**The platform operator** (`internal/platform/packages_platform.go`,
`authorizePlatformOperator`; design
`docs/design/forced-modularity-build-2026-11-04.md` section 5). One act exists:
the package yank and forced update, `POST
/internal/platform/operator/packages/yank` on corpusd. Its gate mirrors the
key-escrow break glass (`key_escrow_http.go`): a loopback transport, the host
credential (`hostauth.Authorize`), and `X-Choir-Operator` /
`X-Choir-Operator-Token` matched against `CHOIR_PLATFORM_OPERATORS`
(`name:token,...`, read by `internal/platform/config.go`) in constant time.
Reaching it means ssh to Node B and `choir platform yank` on the host. The
operator list is empty on Node B, so the route answers 503 and its staging
proof cannot run. The record of the act is a new listing version with
`state_actor` `platform:<name>`; the name is whatever the env file says, and
nothing ties it to a person.

**Owner acts** (`internal/proxy/owner_acts.go`, `internal/owneract`). The proxy
admits an owner act for the owner's passkey session (`AuthMethod` "cookie") or
an API key carrying the `owner:acts` scope, which only a signed-in session can
mint. For acts corpusd executes, the proxy then mints an `OwnerActReceipt`: a
receipt signed by the proxy's own host-owner-act Ed25519 key (systemd
credential, private key in the proxy only), naming the act kind, the owner's
user id, the auth method, a credential reference (the session JWT's hash or
the key id, never the secret), one computer id, a subject that binds the
exact request, a 128-bit nonce and a 5-minute window. corpusd pins the public
key and verifies (`owneract.Verifier.Verify`). The verifier re-pin
(`verifier_key_pin.go`) verifies in every mode, not only under
`CHOIR_OWNER_ACT_MODE=enforce`, and records the receipt in
`owner_act_receipts`, whose unique nonce and digest make it single use.

**User ids.** The auth service's user id (`sub` of the access JWT, `id` in `GET
/auth/session`) is a UUID. It is not a secret: user ids are already in the
repo (`CORPUSD_AUTOPAPER_OWNERS` in `nix/node-b.nix`, and the tier file
below), and a package track is named by its owner id in public listings.

**The account roster in the repo.** `internal/computertiers/computer-tiers.json`
is embedded in the binaries and "set by a reviewed commit": paid accounts (the
owner's account is the one paid entry, note "the owner's account, an ordinary
paid account (owner ruling 2026-10-13)"), platform computers (the paper,
`computer-1355b83bced1613eb048ee28d66688a8`, which vmctl makes
`critical_protected`), and test computers. vmctl and corpusd
(`standing_consensus.go`) already read it. Its package doc says it: "The tiers
are not configurable on the host: a tier changes by a reviewed commit."

**The paper today.** It is owned by a dedicated test owner with no person
behind it (`e5962310-...`, registered by `qa_test_computer.mjs` with a virtual
passkey). Every act on it (model policy edits through `PUT /api/files` on
`System/model-policy.toml`, source schedule control, health reads) is done
with that owner's API key from the developer machine
(`~/.config/choir-qa/test-computers/autopaper-paper.json`), renewed every 7
days from the saved browser session by `scripts/qa_renew_key.mjs`. The key
expired on 2026-11-02 unnoticed; the current one expires 2026-11-12. A lapsed
session means nobody can act as the paper's owner.

## 2. The foundational question

What is the record of truth for "who is the platform operator"?

It is an authority grant, not a credential. A credential (the env token) says
"whoever holds this string"; a grant says "this account". The grant must be:
reviewable (who granted it, when, why), durable across host rebuilds,
settable without ssh, and readable by the one service that executes the
act. The act itself, separately, must land on a durable record that names
the account and the proof that the account's owner was present.

So two records, not one: the **grant** (account id holds the role) and the
**act** (this account, by this session, did this, at this time).

## 3. Options, cheapest to dearest

1. **A user-id list supplied at deploy.** A GitHub Actions secret holds the
   operator account ids; each staging deploy carries it to Node B through the
   deploy gate's typed request, and the deploy program writes it into a root-only
   env file corpusd reads at start. No id in the repo; no ssh; the owner
   changes it in the GitHub settings, and GitHub's audit log records who did.
2. **A hand-written env file on Node B** (as `CHOIR_KEY_ESCROW_OPERATORS` is).
   Cheaper to build, but it is set over ssh, which is what we are leaving.
3. **User ids in the repo** (`nix/node-b.nix`, or a `platform_operators` list
   in the embedded roster `internal/computertiers/computer-tiers.json`). Reviewed
   and versioned, and the roster is the natural home. Excluded by the owner
   (2026-11-06): the personal account is named at runtime, supplied at deploy,
   never in the repo.
4. **A platform record on the tape**: a `choir.platform_operator_grant`
   object in Store B, created by a bootstrap act and changed by an operator
   act. The most "on the tape", but its first entry has no better root of
   trust than option 1, and it needs a revocation story and a reader in every
   service.
5. **Roles in the auth service** (a role column on users, issued as a JWT
   claim). The general solution, and a new auth-session surface (red, tier 2,
   every token consumer). Not needed for one role held by one person.

## 4. Choice

**Option 1**: the grant is the owner's GitHub secret, carried by every deploy
into corpusd's environment; the act is an owner act with a signed receipt,
verified and recorded at corpusd. It reuses the deploy gate's typed request,
the unit env directory, the owner-act gate and the owner-act receipt, and adds
no new credential or service.

Why not a switch (CLAUDE.md, "No switches"): the list does not gate a
feature; the route is on for everyone and refuses everyone not granted. It is
an authority roster, like the paid-account list and like
`CHOIR_KEY_ESCROW_OPERATORS`. Empty means nobody holds the role, not "the
feature is off".

The env token path stays, unchanged, as **break glass**: if the proxy, the
auth service or the owner's passkey are down, an operator on the host still
yanks with `choir platform yank`. Its list stays empty on Node B until
someone writes one by hand; that is fine for break glass.

### 4.1 Bootstrap: what the owner does by hand

The repo names no operator. The steps, in order:

1. **Let O1 deploy first** (a host OS deploy: the deploy gate is installed
   by the NixOS closure, so the gate that accepts the new key must be live
   before CI sends it). Until the secret exists, CI sends nothing new and
   the old gate is never asked about the key.
2. **Read the account id.** Signed in on choir.news with the personal
   account, open `https://choir.news/auth/session`; the `user.id` field is
   the id (a UUID, lower case).
3. **Add the repository secret** `CHOIR_PLATFORM_OPERATOR_USERS` in GitHub
   (Settings, Secrets and variables, Actions) with that id as the value
   (several ids: comma-separated, no spaces, at most 8).
4. **Run a deploy** (the next push to main, or re-run the last deploy job).
   The deploy writes `/var/lib/go-choir/unit-env/corpusd-operators.env`
   (`CHOIR_PLATFORM_OPERATOR_USERS=<ids>`, mode 0600, root) and restarts
   corpusd when the content changed. corpusd logs `platform operator
   accounts: 1`.

To revoke every account: set the secret's value to `none` and deploy (a
GitHub secret cannot be empty). Deleting the secret leaves the file as it
is: a deploy without the key never touches it, so a missing secret on a fork
or a re-run never revokes the owner by accident.

How the value travels: the deploy request is the gate's closed alphabet
(`[a-z0-9,-]`); the gate accepts the key only on the `deploy` verb, only as
`none` or one to eight lower-case UUIDs, and passes it to the deploy program
as one typed value. GitHub masks the secret in the CI log. On Node B the id
appears in the deploy's journal and the root-only env file; a user id is not
a secret, and no email is involved anywhere.

### 4.2 The act through the proxy

```
owner's browser (passkey session)
  POST /api/platform/operator/packages/yank {release_id, reason, forced_release?}
proxy
  session only (an API key, even with owner:acts, is refused)
  validates the body's shape
  mints OwnerActReceipt: act platform.package.yank, owner_user_id = session user,
    auth_method session, credential_ref session-jwt-sha256:..., computer_id
    "platform" (the platform scope, not a computer), subject {release_id,
    forced_release, reason_sha256}
  POST corpusd /internal/platform/operator/packages/yank (host credential, receipt header)
corpusd (the single authority for the role check)
  loopback + host credential, as before
  exactly one credential: a receipt, or an operator token (both: refused)
  receipt verified in every owner-act mode (as the verifier re-pin is),
    subject must bind this request
  owner_user_id must be in CHOIR_PLATFORM_OPERATOR_USERS (supplied at deploy)
  receipt recorded in owner_act_receipts (single use; replay refused) BEFORE the effect
  new listing version: state yanked, state_reason, state_actor platform:user:<user id>
```

The proxy does not check the roster. corpusd is the only reader of the grant
and the only writer of the effect, so there is one place that decides. A
receipt minted for a non-operator is refused at corpusd with 403 and never
recorded.

Session only, deliberately. The ruling is "from my account"; an `owner:acts`
API key is a bearer string that lives on a disk, which is the ad hoc key
handling this design removes. The operator's role is exercised only where
the passkey is. (A CLI path, if wanted, is a short-lived key minted for one
act, section 6, step O5; not a standing key.)

The computer id slot of the receipt is the literal `platform`
(`owneract.PlatformComputerID`). Real computer ids are `computer-<32 hex>`, so
it never collides, and `Verify` checks it like any computer id, so a receipt
for a computer act can never be replayed as a platform act or the reverse.

### 4.3 Audit record

- **The effect** is the listing chain in Store B, as before: a new version
  chained by `prev_version_digest`, `state_actor`
  `platform:user:<user id>` (the token path keeps `platform:<name>`; token
  names cannot contain `:`, so the two never collide).
- **The authority** is the signed receipt, stored whole in
  `owner_act_receipts` (act kind, user id, auth method, credential reference,
  computer `platform`, receipt digest, canonical receipt JSON), with
  `mode_receipt_digest` set to the digest of the listing version it was
  minted for. It is recorded before the listing is written: an act whose
  write then fails leaves a recorded attempt and no effect, never an effect
  with no recorded authority.
- **The log line** names the actor and the receipt digest.
- **The grant** is the secret (GitHub's audit log records each change), the
  deploy that carried it (the gate logs the request on Node B), and corpusd's
  start log line with the operator count.

### 4.4 Rollback

Revert the O1 commit: the route, the roster field and the receipt kind go;
the token path is unchanged. To revoke the role without a code revert:
set the secret to `none` and deploy. Reverting O1 leaves the env file on
Node B; corpusd at the reverted commit ignores it. Acts already taken are undone by the operator's next act (a
yank is reversed by the owner's reinstate only after the platform lifts it;
lifting a yank is not built and is not part of O1).

### 4.5 Failure modes and what answers each

| Failure | Answer |
|---|---|
| A non-operator's session calls the route | corpusd 403 "not a platform operator"; nothing recorded |
| An agent's API key (admin or owner:acts) calls it | proxy 403 before any receipt is minted |
| The proxy has no owner-act key | proxy 503 "owner act receipt unavailable"; no unsigned forward |
| corpusd has no owner-act public key | 503; the token path still works |
| Receipt replayed within 5 minutes | `owner_act_receipts` nonce/digest unique: 409 |
| Receipt for another request (release, target, reason edited) | subject mismatch: 403 |
| Receipt for a computer act replayed here | computer id is not `platform`: 403 |
| A guest reaches corpusd's route | loopback and host credential required, as before: 403 |
| Both a receipt and a token sent | 400, one credential per act |
| The secret names the wrong account | change the secret and deploy; the receipt log shows every act the account took |
| The secret is set before the new gate is live | the old gate refuses the unknown key and the deploy fails visibly; remove the secret, deploy O1, add it again |
| The secret is deleted | the deploy omits the key and leaves the file: the grant stays until set to `none` |
| Observe mode (`CHOIR_OWNER_ACT_MODE` unset) | irrelevant: this route verifies in every mode |
| An email-code or email-link session | refused: the route admits a passkey session only (the access JWT's `am` claim), since an email session is as strong as the inbox and the role acts on every computer (second seat r0, 1) |
| Stolen session cookie | the same exposure as every owner act; SameSite=Lax keeps cross-site POSTs out; the 15-minute access JWT bounds it |

### 4.6 Proof

Unit: the corpusd route over the real store (receipt path, roster check,
replay, subject binding, both-credentials refusal, token path unchanged); the
proxy route (session mints a receipt with the platform scope, API keys are
refused, no signer is 503, a bad body never reaches corpusd); the gate's
parse of the new key (`.github/scripts/node-b-ci-gate-test`). Deployed: the owner, signed in, yanks a disposable release they
published (or a test release named for the purpose) and reads the listing
back with `state_actor` `platform:user:<id>`. That step needs the owner's
session; no agent holds one, by design.

## 5. Autopaper VM control from the operator role

The paper is a platform computer (`platform_computers` in the same roster),
owned by an account with no person behind it. The ad hoc part is not the
API key as such; it is that the only way to act as the paper's owner is a
session and a key on a developer's disk. The operator role replaces that:
the platform operator acts **on platform computers** with the owner-act
receipt naming the operator and the computer, and no key for the paper's
owner exists anywhere.

### 5.1 The acts

| Act | Receipt kind | What it does | Today |
|---|---|---|---|
| Read health | none (a read) | the paper's vmctl state, build, last publish, source schedules, model policy resolve | API key from the dev machine |
| Set model policy | `platform.computer.model_policy` | write `System/model-policy.toml` on a platform computer as its owner would, the old file kept as the rollback | `PUT /api/files` with the paper owner's key |
| Source schedules | `platform.computer.schedule` | start/stop/list the paper's schedules | same key |
| Restart | `platform.computer.restart` | a planned, marker-backed refresh through vmctl (the always-busy rule already allows it for `critical_protected`) | ssh / deploy only |
| Mint a short-lived key | `platform.computer.key` | for scripts that still need a bearer: a key for the paper's owner, scoped, bound to the paper, expiring in hours, minted by the auth service on a corpusd-verified operator receipt; never renewed by a saved session | `qa_renew_key.mjs` every 7 days |

### 5.2 The mechanism

One proxy gate, `authorizePlatformComputerAct(authResult, computerID, act)`:
the caller is a session (as in 4.2), the computer is in `platform_computers`,
and corpusd confirms the session's user holds the operator role (a
`/internal/platform/operator/check` read, or the receipt verification for
acts corpusd executes). For acts the guest executes (model policy, schedules),
the proxy forwards to the guest as it does for the owner today, with the
paper's owner id as the authenticated owner and an added
`X-Choir-Platform-Operator: <user id>` the guest records with the act on its
tape (the guest already records owner acts; it gains an actor field). The
receipt is recorded at corpusd in `owner_act_receipts` with the paper's
computer id, so the paper's acts are on the same append-only receipt log as
every other owner act on it.

This keeps the paper owned by its own account (its publications, its
`CORPUSD_AUTOPAPER_OWNERS` front-list entry and its tracks stay as they
are) and adds a second principal, the operator, who may act for a platform
computer. It does not make the operator the owner of the paper, and it gives
the operator nothing on any non-platform computer.

### 5.3 Build order

1. **O2 read health** (orange): `GET /api/platform/operator/computers` lists
   the platform computers with vmctl state and build; `.../<id>/health`
   relays the guest's health. No receipts (reads), session + roster only.
   Proves the gate on staging with no write.
2. **O3 model policy** (orange; provider routing): the most frequent hand
   edit, done today with the expiring key. Proxy forwards the guest's own
   file write; receipt at corpusd; the guest keeps the previous file.
3. **O4 schedules** (orange): start/stop, the second most frequent.
4. **O5 restart** (red; vmctl): a planned, marker-backed refresh request;
   vmctl's existing refusals (self-development apply, live capsule, skew)
   stay.
5. **O6 short-lived key** (red; auth): only if a script still needs a bearer
   after O2 to O4. Then retire `qa_renew_key.mjs` for the paper and delete
   `autopaper-paper.*` from developer machines.

After O4, the paper's test owner needs no saved session and no standing
key; the residual in the modular-architecture ledger closes.

### 5.4 What needs the owner's hands for section 5

Nothing new: the role from O1 is the authority. The owner confirms each
slice on staging with their session (one act each), because no agent holds
an operator session.

## 6. A future consumer: the model control app

Owner, 2026-11-06: an app plus API and CLI, accessible only to the platform
owner, to configure which models are used at every level (model pools and
their routing policies, per-desk model assignment, per-VM/computer policy),
with provider usage analytics (tokens used, limits remaining) in the same
app. Not built here; O1 is shaped so it can sit on the role without a second
mechanism.

What it needs from the role, and where O1 already provides it:

| Need | Provided by |
|---|---|
| One role check | corpusd's operator check (`platformOperatorUser`), the only reader of `CHOIR_PLATFORM_OPERATOR_USERS`. The app's reads ask it through one host-only route (`GET /internal/platform/operator/whoami?user_id=`, added with the first read slice), so the proxy never holds a second copy of the list |
| Owner-session acts | the proxy's session-only gate and an `OwnerActReceipt` per write, `computer_id` `platform` for platform-wide acts (pools, routing) and the target computer id for per-computer acts |
| Audit | the receipt in `owner_act_receipts`, recorded before the effect, with the actor's user id and credential reference; plus the effect's own record (a policy object version, a computer's tape event) |
| Act kinds | a `platform.` prefix per family: `platform.package.yank` (O1), `platform.model.pool`, `platform.model.routing`, `platform.model.desk`, `platform.computer.model_policy`, and the section 5 kinds |

Where each setting lives (so the app edits a record, not a file by hand):

- **Pools and routing policies** are host-wide gateway configuration today
  (the gateway's provider chain and breakers, `internal/gateway`). They move
  to a platform record (a versioned object in corpusd, written only by an
  operator act) that the gateway reads; the app writes that record.
- **Per-desk assignment and per-computer policy** are each computer's
  `System/model-policy.toml` today (`internal/modelpolicy`), edited by hand
  (`docs/design/app-packages-replan-2026-10-24.md` item G already plans a
  `choir.model_policy` kind and an owner-act route). For the owner's own
  computers that route is an owner act; for platform computers (the paper)
  it is section 5's operator act; the app shows both, and the policy a
  computer resolves (`/api/model-policy/resolve`) as the read-back.
- **Usage analytics** come from the gateway, which already sees every model
  call per provider and model and the upstream rate-limit answers
  (`usage_limit_reached`, 429s, reset estimates in the breakers). The app
  reads a gateway usage summary (tokens per provider, model and computer;
  limits and resets where the provider reports them) through a host-only
  route the proxy relays to an operator session. Reads need the role check,
  not a receipt.

The API is the product surface (`/api/platform/operator/...`), the CLI is a
thin client over it (`choir platform models ...`, with a session-derived,
short-lived credential per section 5 step O6, never a standing key), and
the app is a package desk published by the platform whose routes answer
only an operator session. Build order after O1: the `whoami` read and the
usage read (no writes), then per-computer policy (reusing section 5's O3),
then the platform pool/routing record (red: provider routing).

## 7. Residuals

- **O1 has no operator UI.** The act is an API call from the signed-in
  choir.news tab (for example, `fetch('/api/platform/operator/packages/yank',
  {method: 'POST', headers: {'Content-Type': 'application/json'}, body:
  JSON.stringify({release_id, reason})})` in the console). A Library
  "platform" panel is a later, separate slice.
- **No lift of a yank.** As before O1.
- **The role is one secret.** Fine for one person. A second operator, or a
  role that must change without a deploy, is the trigger for option 4 or 5.
- **The grant is not reviewable in git.** By the owner's choice (no id in the
  repo); GitHub's audit log and corpusd's start log are the trail.
- **The receipt table is in corpusd's control store**, not on a computer's
  tape. Platform acts have no computer tape; the listing chain in Store B is
  the effect's durable record.

## 8. Failure list (build) (2026-11-09, slices O2 and O3)

Written before the build of `GET /api/platform/operator/computers`, the
operator-gated policy acts, and `choir paper`. Each item is a way the gate
could fail open or strand state; the test that covers it is named. Proxy tests
are `internal/proxy/platform_operator_computers_test.go` (P), guest tests
`internal/autoputer/policy_operator_test.go` (G), corpusd
`internal/platform/operator_check_test.go` (C), CLI `cmd/choir/paper_test.go` (L).

**What the build found that the sections above assumed otherwise.** The
roster variable is `CHOIR_PLATFORM_OPERATOR_USERS` (O1, read by corpusd
only), not `CHOIR_PLATFORM_OPERATORS`, which is the break-glass
`name:token` list. O1 built only the package yank; `authorizePlatformComputerAct`,
the platform-computer routes and the receipt on a computer's tape did not
exist. 5.2 said the proxy records an owner-act receipt at corpusd for the
paper; the build records the receipt on the target computer's tape instead
(below), because corpusd's receipt table is single-use bookkeeping for acts
corpusd executes and widening its act kinds is a second red surface.

Admission

- A1 no credential is admitted (P `TestOperatorAdmissionFailsClosed`).
- A2 an unset, empty or `none` roster admits nobody (C; P `TestOperatorRosterFailsClosed`).
- A3 one malformed entry admits nobody, the well-formed neighbours included (C; P same).
- A4 an ordinary account's session or admin key is refused (P admission).
- A5 near matches of the operator id (case, spaces, prefix, suffix) are refused; the match is exact on the account id (C; P admission).
- A6 corpusd down, 5xx, malformed or untyped answers, or no corpusd URL, refuse; only a typed `{"operator":true}` admits (P roster).
- A7 an email-code or email-link session of the operator is refused; passkey session only (P admission).
- A8 an API key bound to a computer does not carry the role, whatever its scopes (P admission).
- A9 a key keeps its scopes: a read key reads and cannot PUT or verify (P admission).
- A10 the roster check answers only host callers with the host credential (C).

Targeting

- T1 an ordinary user's computer (`computer-7cc9...`) is unreachable by the operator (P `TestOperatorReachesOnlyPlatformComputers`).
- T2 a test-tier computer is unreachable: the platform tier only (P same).
- T3 case, whitespace, prefix, suffix, traversal, empty and non-computer ids are refused before vmctl is asked (P same).
- T4 vmctl absent, or not agreeing with the roster, refuses; a stopped guest is not faked (P `TestOperatorTargetNeedsVmctlAgreement`).
- T5 the computer comes from the path only: `X-Choir-Computer` and `computer_id` cannot retarget (P same).
- T6 only the allowlisted guest routes are forwarded; DELETE, PATCH, POST on the policy, other files, traversal and encoded slashes are not (P `TestOperatorForwardsOnlyTheAllowlist...`).
- T7 the operator naming a platform computer on the ordinary `/api` route still needs ownership (P `TestOperatorMarkerIsNotAnOrdinaryOwnerAct`).

Receipt

- R1 the receipt is recorded before the write lands, and a receipt that cannot be recorded refuses the write (G `TestOperatorPolicyWriteIsRecordedBeforeItLands`, `...NeedsItsReceipt`).
- R2 a client-supplied operator marker never reaches the guest (P forwarding; the header is in the ingress strip list).
- R3 the guest believes the marker only from the host peer and never in-process, so a desk cannot forge a receipt (G `TestOperatorMarkerIsNotBelieved...`).
- R4 the receipt names the account id, the act, the revision kept and both digests, never an email, and the store accepts the event kind (G `...NamesTheAccountNotAnEmail`; agentcore `TestPlatformOperatorReceiptEventIsAppended`).
- R5 a crash between the receipt and the write leaves a receipt for a write that did not happen (the CLI's read-back catches it); never a write with no receipt. Not testable without a crash; the order is covered by R1.
- R6 an ordinary owner write grows no receipt (G `...ReachOnlyThePolicySurface`).
- R7 a guest from before this build ignores the marker and would write with no receipt; the proxy writes only to a guest that says it records receipts (P `TestOperatorWriteRefusedOnAGuestThatDoesNotRecordReceipts`).
- R8 a marked request reaches no file but the policy and its revisions, and no verb but the policy read and the validated write (G same as R6).

Forwarding

- F1 no Authorization, Cookie, internal token or client header reaches the guest; the request is built from nothing (P forwarding).
- F2 the guest sees the computer's owner as the authenticated user and the operator only in the marker (P forwarding).
- F3 the host credential is sent only by the existing corpusd call to the configured corpusd URL; the guest call carries none.
- F4 no header of the guest's reaches the client (P forwarding).
- F5 status reads never wake a stopped computer (P `TestOperatorStatusListsPlatformComputers`).

CLI

- L1 the paper verbs use the operator route and never send `X-Choir-Computer`; a non-computer id never becomes a path; two platform computers need `--computer`; the receipt is printed (L).
- L2 the paper verbs are not MCP tools (L `TestPaperVerbsAreNotMCPTools`).

Edges from the policy landing

- E1 a delete then a move cannot land an unvalidated policy (G `TestPolicyCannotBeLandedUnvalidatedByMoveOrAlias`).
- E2 a write through an in-root symlink alias of the policy is validated like a PUT (G same).
- E3 left open: a DELETE of the policy file, which falls the computer back to the platform default (the owner path only; the operator path does not reach DELETE); no serialization of two concurrent PUTs.
