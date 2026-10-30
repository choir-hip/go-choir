# App agent network policy: egress declarations and ingress as records (NET-10)

Status 2026-10-30: `internal/deskmanifest` now exists, so the "does not exist" statements below are stale; the document is partly superseded by NET-10.

Status: design, green (no code). Date: 2026-10-21. Superseded in part (2026-10-22): §2 field list and §7 order by [FP10 NET-10](../definitions/choir-launch-fp10-net10-2026-10-22.md). Checklist item: NET-10 in
[security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md).
Joins: S4 "Egress capability model", S7 "Appagent egress grants" and "App
packages carry app-agent desks", the [email desk design](../reports/agentic-email-desk-2026-10-14.md)
(§2.5, §3.2), ARCH-02 (desk manifest) and ARCH-03 (app package carries its desk).

This is the "plan update owed". S4 and S7 each get a short pointer to it;
this file is the one place the app-desk network policy is written down.

## State today (observed)

- `internal/deskmanifest` does not exist yet. ARCH-02 is unbuilt; the manifest
  fields below are a design input to it, not a description of code.
- The only app-agent ingress is mail: Resend webhook, Caddy, maild on the host
  (`internal/maild/webhook.go`, `ingest.go`), stored in a host per-user mailbox.
  maild's delivery to the computer as an observation record to `email:<owner>`
  is designed (email desk §2.5) but not the general model.
- No other app agent has ingress. No ingress model exists.
- A guest Maildir and `POST /api/mail/inbound` still exist in the computer
  (email desk report); they are a second ingress path and are retired by this
  design (see "Retiring paths").

## 1. Rules

1. **Egress goes through the platform egress path only** (S4 principle 1). An
   app desk has no sockets of its own. Until NET-01 lands, general guest
   egress is still open at the substrate; this policy is the target contract,
   and the manifest is enforced only where the proxy or host fetch service is
   the path.
2. **A manifest declares; it never grants.** The declaration is a request. A
   grant is a tape object (S4 grant object) issued by the owner act at
   install. A desk cannot widen its own manifest or grants.
3. **Ingress never connects into the guest.** Every inbound byte from outside
   lands first on a host service (maild for mail; a future `hooked` or
   equivalent for webhooks and push), is authenticated, deduped, labeled
   untrusted and queued on the host, then reaches the desk as a record.
   There are no listening ports, port-forwards or reverse tunnels from the
   internet into a computer.
4. **Delegation only narrows** (S4 principle 3). An app desk may hand a subset
   of its grants to a capsule or cast only through management, and the subset
   dies with the parent.

## 2. Manifest declarations

The ARCH-02 desk manifest carries two network blocks. Field names are
proposals; ARCH-02 owns the final schema.

### 2.1 `egress`

A list of declared egress verbs. Each entry is the S4 grant object in
declaration form:

| Field | Notes |
| --- | --- |
| `verb` | a named capability the desk's module surface exposes, e.g. `mail.send`, `calendar.sync`. The desk code calls the verb; it never names a URL |
| `destination` | exact origin, plus path or endpoint contract. No wildcard unless marked `wildcard: true`, which forces explicit owner approval |
| `effect_class` | `read`, `idempotent-write`, `visible-write`, `money`, `irreversible`. Declared per endpoint, not inferred from the HTTP method |
| `operation_contract` | for `visible-write`: schema digest of the accepted request shape(s), enforced at the proxy |
| `data_ceiling` | `public`, `owner-private`, `third-party-private` |
| `credential` | name of a platform-held credential the proxy injects. The desk never sees the secret |
| `budget` | requests, bytes and spend per period, counted against management's budget |
| `renewal` | renewal period for the durable grant |
| `conditions` | e.g. per-use owner confirmation for `money` and `irreversible` |

Install-time checks (fail closed): a verb with no `effect_class`, a
`visible-write` with no `operation_contract`, a private-range, loopback,
link-local, host or tap destination (never grantable), or an undeclared
credential, makes the manifest invalid and the package uninstallable.

### 2.2 `ingress`

A list of declared inbound sources. Each entry names a **host service** and a
**record shape**, never a port:

| Field | Notes |
| --- | --- |
| `source` | `mail`, `webhook`, `push`, `schedule-external` (extensible by host services only) |
| `host_service` | the host service that terminates it: `maild` for mail; a named webhook or push receiver when one exists |
| `address` | what the owner sees and approves: the alias or alias pattern for mail, the registered webhook path or provider for webhooks, the push topic. Public addresses and secret (plus-code) addresses are marked differently |
| `authenticity` | the verification the host service must perform: Svix HMAC for the Resend webhook, provider signature, etc. A source with no verifiable authenticity can only be declared with `taint: untrusted-unauthenticated`, and the desk treats every record as such |
| `taint` | `UNTRUSTED_EXTERNAL_EMAIL` and kin; flows into the desk's taint class (SEC-02, sandbox profile) |
| `record` | the record kind the desk receives, e.g. an observation record to `email:<owner>` listing message ids |
| `batching` | records are per batch, not per message, so a flood stays within the wake pacing (64 wakes a sweep, 4 a desk) |
| `wake_policy` | default, hibernated-computer delivery rule (email desk §3.2) |
| `volume` | a host-side per-owner rate and size cap; overflow is dropped or quarantined on the host with a counted receipt, never delivered |

Install-time checks: an ingress entry naming a `host_service` that does not
exist, or declaring a listener, forward or tunnel into the guest, is invalid.

## 3. Approval at install, re-approval on widening

1. **Install.** The owner sees, in Texture, one approval surface per desk: the
   egress list (destination, effect class, data ceiling, credential names,
   budget) and the ingress list (address, source, taint). Phone-permission
   style, but endpoint-specific. Approval is an owner act recorded on the tape;
   it issues the durable grants (egress) and registers the ingress routes
   (host service side). Install is a tape fact (owner ruling D1).
2. **Per-entry approval.** The owner may approve a subset. A desk whose
   verb was not approved still installs; the verb returns a typed denial.
3. **Upgrade.** The new manifest is compared with the approved one, entry by
   entry. An upgrade **widens** when it adds an entry, or changes any of:
   destination (broader origin, new path, wildcard), effect class upward,
   data ceiling upward, credential binding, budget up, an operation
   contract that accepts more, an ingress address or source, a taint class
   downward (less untrusted), or an ingress volume cap up. A change that only
   narrows applies without re-approval.
4. **Re-approval.** A widening upgrade does not activate the widened entries
   until the owner approves the diff, shown in Texture, joining the S6
   divergence guard. The previously approved subset keeps working under the
   old grant meanwhile (the upgrade is not blocked, the widening is).
5. **Revocation and uninstall.** Revoking a grant or uninstalling the desk
   revokes its egress grants and removes its ingress routes at the host
   service in the same owner act. Revocation is real because credentials stay
   with the platform (S4 principle 5).
6. **Renewal** (withdrawn for app desks by owner ruling Q3, owner notes
   §40, 2026-10-22: preset policies approved at install, never renewed by
   the owner; see FP10 NET-10 §5.1). Durable grants expire on their `renewal` period; renewal is an
   owner act or a pre-approved policy. A missed renewal expires the grant
   visibly; it does not extend silently.
7. **Events.** Issued, used, widened-pending, approved, renewed, expired and
   revoked are all tape events. Texture renders "what can my computer do out
   there, and what can reach it?" from the grant and route ledger.

## 4. Ingress lands on the host, reaches the desk as records

```text
external sender --> host edge (Caddy) --> host ingress service (maild, ...)
   verify authenticity, dedupe, label taint, apply the desk's declared policy,
   store sealed on the host queue
host service --> vmctl: is the owner's computer available?
   deliver per wake policy over the existing host-to-computer channel
computer: runtime stores payloads as data, commits ONE system-provenance
   observation record to <desk>:<owner>, listing payload ids
record --> outbox, projector, dispatcher --> desk wakes
```

Properties:

- **No inbound connection to the guest.** The host-to-computer channel is
  host-initiated and already authenticated; nothing from the outside reaches a
  guest listener.
- **Payloads are data, records are work.** On a crash restart the delivered
  payloads remain (marked untriaged); the triage run in flight closes as
  "interrupted by a restart" (CLAUDE.md). Standing intents (schedules, policy
  follow-ups) survive; occurrences do not.
- **The host service is the ingress authority.** It owns authenticity,
  dedupe, quotas, quarantine and the audit trail. maild is the first
  instance; a new ingress source is a new host service (or a new maild
  route), reviewed as one, not a desk feature.
- **Egress to answer is a separate grant.** Receiving mail does not authorize
  sending. `mail.send` is an egress verb with `visible-write`, an operation
  contract and its own approval (email desk send policy).
- **Taint follows the record.** A desk that ingests untrusted content and also
  holds a `visible-write` or credentialed egress verb is the dangerous pairing
  (email desk §6). The manifest check flags that pairing at install so the
  owner approval surface can show it, and the S4 data-flow rule bounds it:
  data ceiling is set by what the holder was given.

## 5. Where this lands in the existing plans

- **S4** (egress model): unchanged. The grant object and effect classes are
  reused as the declaration form. Add a pointer from "What S4 builds now vs
  later" to this file.
- **S7**: "Appagent egress grants" gains the `ingress` block, and the install
  and upgrade approval rules in §3. Pointer added.
- **ARCH-02**: the manifest fields `egress verbs` and `addressing and inbox
  routing` are specified here (§2).
- **ARCH-03**: install registers the desk, its grants and its ingress routes
  in one owner act (§3).
- **Email desk**: maild is the ingress authority; its desk manifest is the
  first instance of `ingress`.

## 6. Retiring paths (red, for later)

- The guest Maildir and `POST /api/mail/inbound`, if still reachable, are a
  second ingress that bypasses maild's policy. They are to be removed or
  closed once maild delivery is the sole path. Verify they are unreachable
  from outside before launch; this is FP0's check, not code in this lane.
- General guest egress closes with NET-01. Until then the egress half of the
  manifest is policy on the proxy path, and the substrate gap is named in the
  checklist (NET-01), not hidden here.

## 7. Staging of work

- **Green (this file).** The model and manifest blocks.
- **Red, off or observe-only first.** Ordered, each its own mission:
  1. manifest schema in `internal/deskmanifest` with the two blocks and the
     install-time validity checks (ARCH-02), validate-only;
  2. the widening diff function (pure) with table tests in observe mode:
     compute and log the diff on an upgrade, enforce nothing;
  3. Texture approval surface and the grant and route ledger events;
  4. enforcement at the proxy and the host services, behind a flag, default
     observe, then default on.
- **Verification plan (for FP0, deployed).** Probes are not run here.
  Steps: install a fixture desk package declaring one `read` egress and one
  mail ingress; confirm the grant and route events; upgrade it with a widened
  destination and confirm the widening is held pending; revoke and confirm
  both the egress denial and the ingress route removal; attempt an inbound
  connection to a guest port from outside and confirm it is unreachable.

## Open questions (named residuals, no blocking ask)

1. Should ingress `volume` caps be owner-adjustable or platform-fixed per
   plan tier? Default: platform-fixed, owner may lower.
2. First non-mail ingress source after maild: webhooks or push? Default:
   neither before launch.
3. Whether a desk without an approved ingress entry is told so or sees
   nothing. Default: nothing arrives; the owner sees the dropped counts.

Heresy delta: discovered, a second mail ingress path (guest Maildir and
`/api/mail/inbound`) may exist beside maild; introduced, none; repaired, none.

## Now card (FP10, NET-10 green part)

- done: this design; pointers in S4 and S7. No station file exists for FP10
  (metamission entry is "to be authored"), so the now card lives here.
- FP0: tick NET-10 "plan update owed" in the checklist (docs-only, no deploy);
  the red part (§7) stays open and is not started.
- owner decisions: none blocking; three defaults in "Open questions".
