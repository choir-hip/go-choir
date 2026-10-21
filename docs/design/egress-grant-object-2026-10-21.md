# The egress grant object: one model for open egress and app-desk grants (NET-05)

Status: design, green; the record and its rules are built unwired in
`internal/egressgrant` (red, off). Date: 2026-10-21. Station:
[FP10 NET-05](../definitions/choir-launch-fp10-net-05-2026-10-21.md). Problem:
[no grant object for open egress](../problems/no-grant-object-for-open-egress-2026-10-21.md).
Builds on: the [grants design](engineering-network-grants-2026-10-10.md) §4.0a
(the L0 to L4 ladder), §4.2 (the grant object), §5 (phases); the
[app-agent network policy](app-agent-network-policy-2026-10-21.md) (NET-10)
§2.1 and §3; `internal/owneract` (owner-act receipts).

## 1. What this adds, and what it narrows from §4.2

The grants design's `EgressGrant` was written for engineering fetches
(L1) with tiers decided by management or research. NET-05 is about **open**
egress (L1, L3, L4) and must share one model with app-desk manifest grants.
The first slice therefore narrows §4.2:

- **One decider: the owner act.** Every decide is an owner-act receipt
  (`internal/owneract`), minted by the proxy after it authenticates the
  owner, verified by the reader. Management and research recommend; they
  never decide in this slice. Tier 0 to 2 delegation (grants design §4.3 and
  its open question 2) is a later widening that needs the owner's ruling.
- **No tiers, signals or categories yet.** A grant is an explicit list of
  destinations. Predicate grants come later.
- **Holder-typed.** The holder is an engineering assignment (the grant dies
  with it) or an installed app desk (the grant lives until its renewal
  expiry or until the owner revokes it or uninstalls the desk).

L2 is not a grant. It stays the compiled-in default for implementation
capsules (`capsule.EcosystemEgressPolicy`); a grant only ever adds L1, L3 or
L4 on top of it.

## 2. The record

`egressgrant.Grant`, schema `choir.egress_grant/v1`:

| Field | Meaning |
| --- | --- |
| `grant_id` | `egress-grant:sha256:<hex>` of the RFC 8785 canonical JSON of the declaration (everything below except lifecycle). A changed field is a different grant |
| `holder` | `{kind: engineering_assignment \| app_desk, ref}` |
| `computer_id` | the computer the grant is for; the owner act must name it |
| `level` | `L1` (fetch and mount), `L3` (open web through the proxy), `L4` (full network) |
| `destinations` | `{scheme, host, port, path_prefix}`; `host: "*"` is the wildcard |
| `methods` | HTTP methods for L1/L3; default `GET`, `HEAD` |
| `budget` | `max_requests`, `max_download_bytes`, `max_upload_bytes`; all required, all capped |
| `not_before`, `expires_at` | UTC, whole seconds; the lifetime is capped per holder |
| `conditions` | e.g. `no_private_data_mounted` (required for L4) |
| `purpose` | free text shown to the owner; never a scope |
| `verb`, `effect_class`, `operation_contract`, `data_ceiling`, `credential` | the NET-10 §2.1 manifest entry, required for app desks |
| `disposition` | `requested`, `granted`, `denied`, `revoked`, `expired`, `exhausted`, `interrupted` |
| `decision` | `{verdict, owner_user_id, owner_act_receipt_id, owner_act_digest, at}` |
| `requested_at`, `closed_at`, `close_reason` | lifecycle |

A manifest's egress entry (NET-10 §2.1) **is** a declaration with holder
`app_desk`; install turns each approved entry into a `requested` grant and
the owner's approval decides it. An engineering `request_grant` (grants
design §4.3 step 1) is the same declaration with holder
`engineering_assignment`. There is one validator and one checker.

## 3. Validation (fail closed)

Refused at request and on every re-read:

- unknown holder kind or empty ref; empty computer; unknown level;
- a destination that is an IP literal, `localhost`, or a `.internal`,
  `.local` or `.localhost` name. A grant names hosts only; the enforcement
  point re-checks the resolved address and refuses private, loopback,
  link-local, CGNAT, multicast and metadata addresses, as L2 does today;
- a wildcard below L3; any wildcard, and any L4, for an app desk (desks call
  verbs on named endpoints, NET-10 rule 1);
- a scheme other than `https` at L1 and L3; other than `tcp` or `udp` at L4;
- L4 without `no_private_data_mounted` (grants design §4.0a: L4 only for
  capsules with no private data);
- an app desk without verb or effect class, or `visible-write` without an
  operation contract (NET-10 install checks);
- a budget field that is zero, negative or above the hard cap;
- no window, or a lifetime over 24 hours (assignment) or 90 days (app desk;
  NET-10 §3.6 renewal);
- a `grant_id` that is not the digest of the declaration.

Hard caps are package constants and are deliberately generous; per-grant
budgets are the owner's choice inside them (grants design open question 3).

## 4. Lifecycle and who may move it

```text
requested --owner act grant--> granted --clock--> expired
    |  \--owner act deny-----> denied     |--budget--> exhausted
    |                                     |--revoke--> revoked
    \--crash boot / revoke--> interrupted / revoked
```

- **Decide** (`requested` to `granted` or `denied`): only with an owner-act
  receipt of kind `egress.grant.decide` that verifies under the host
  owner-act key for this computer, inside its 5-minute TTL, whose subject is
  exactly `{grant_id, decision}` for this grant. The receipt id and digest
  are kept on the record. A decide on any other disposition is refused, so a
  replayed receipt cannot re-grant a revoked or expired grant.
- **Revoke, expire, exhaust, interrupt** only narrow, so they need no owner
  act; the caller is management, the owner, the holder's terminal state or
  the clock. A closed grant never reopens; asking again is a new grant.
- **Check** allows a use only when the grant is `granted`, now is in
  `[not_before, expires_at)`, and the use's level, scheme, host, port, path
  and method are covered. Enforcement points call it on every request
  (`revocation_check_point: immediately_before_dispatch`).

**Restart (CLAUDE.md).** A crash boot closes `requested` grants as
`interrupted by a restart`: an undecided request is an open obligation. A
`granted` grant is standing authority with its own clock, not work: an
app-desk grant survives; an assignment grant expires because the crash
closes its assignment. No grant is ever re-requested at boot.

## 5. Wiring (designed, not built; each step its own red mission)

1. **Store.** A reducer-owned objectgraph kind `choir.egress_grant`,
   registered online (a new kind, no scan or rewrite of og_objects). Ledger
   acts mirror it for Texture (request = precommit, decide = owner resolve,
   revoke = directive retract); the record wins on disagreement.
2. **Owner act.** Add `egress.grant.decide` to `internal/owneract`'s act
   kinds and to the proxy's minting route, behind the same owner session or
   API-key check as `selfdev.approve`. The Texture approval card shows the
   destinations, level, budget, window and, for app desks, the NET-10
   fields. App install (ARCH-03) mints one act per approved entry at first;
   a batch subject (`grant_ids`) is a later change.
3. **Engineering verbs.** `request_grant` on the engineering handle and a
   `decide_grant` route that only forwards an owner act (no management
   handle verb decides). The capsule capability's `ExternalAccess` carries
   the granted ids, and `engineering_assignment_runtime.go:603` relaxes from
   "empty" to "exactly the assignment's bound set".
4. **Enforcement, observe first.** A `CHOIR_EGRESS_GRANTS=off|observe|enforce`
   switch, default `off`, then `observe` (log the check result, refuse
   nothing beyond L2), then `enforce`:
   - **L3** at the capsule egress proxy (`internal/capsule/egress.go`, FP6's
     file): a host outside the L2 list consults the capsule's grant set via
     `Check`; the proxy still resolves and refuses non-public addresses.
     When NET-01 moves egress to the host, the same check moves with it.
   - **L1** at the host fetch service (NET-01) and the mirrors (NET-02):
     each fetch names its grant; the service checks it before dialing.
   - **L4** needs a separate capsule class with its own NAT'd interface;
     that touches vmctl and the image (FP8's lane) and is design-only here.
5. **Host-side trust** (the guest is not trusted). An enforcement point on
   the host must not take the guest's word for a grant. Recommended: the
   host keeps a small table of `(computer_id, grant_id, expires_at,
   declaration digest)` written by the proxy when it mints the decide act,
   and a revocation set written when the owner revokes through the proxy;
   the guest presents the declaration, the host recomputes the id and looks
   it up. A guest-side revocation by management is forwarded to the host as
   a narrowing (always accepted). This makes the host table the enforcement
   authority and the tape record the supervision authority for the same
   fact, written by the same owner act; standing question 5 is answered by
   "host table wins for enforcement, tape record wins for display", and a
   mismatch is a detector finding. Named residual for the owner.

## 6. Verification plan (FP0, when wired)

Not runnable now (nothing calls the package). For the wiring missions, on a
test computer: request an L3 grant for one public API host; confirm the
proxy refuses it before the owner act and allows it after; let it expire and
confirm refusal; revoke a second one mid-assignment and confirm the next
request is refused; replay the decide receipt and confirm no change; request
`169.254.169.254`, `localhost` and `*.internal` and confirm refusal at
request.

## 7. Open questions (named residuals, defaults stated)

1. May management decide L1 exact-pin grants (grants design tier 1)?
   Default: no, owner act only, until the owner rules.
2. App-desk grant lifetime cap: default 90 days, renewed by owner act.
3. Hard caps: 100 000 requests, 16 GiB down, 256 MiB up per grant. Default
   stands until a real workload needs more.
4. Host-side grant table (§5.5) versus a long-lived host-signed grant
   token the guest presents. Default: the table (revocation is a lookup, not
   a token race).
