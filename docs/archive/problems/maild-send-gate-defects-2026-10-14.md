# maild's Send Gate Can Be Opened by the Party It Gates

Date: 2026-10-14. Source: the agentic email desk design
([report](../../reports/agentic-email-desk-2026-10-14.md), §1.4, defects G1–G6),
re-checked read-only against the code at `origin/main` `6b07fd651`. Status:
**documented** (phase E0, docs only); G1–G4 and G6 **repaired on branch
`fix/maild-send-gate-2026-10-14`, not deployed** (phase E1; see "E1 status"). Mutation class of the repair:
**red** (approval is an auth surface; it decides provider sends).

## The problem in one sentence

maild decides "the owner approved this send" from who reached it, and today
the owner's own computer (a guest VM bound to the owner) reaches it exactly
like the owner's browser session, so the party that drafts can also approve,
and three side doors (reply approval, the trusted-sender verdict, and the
approval-email recipient) let the same party or an outsider stand in for the
owner.

## How callers reach maild today (code)

| caller | path | what maild sees |
|---|---|---|
| owner's browser | Caddy `/api/*` → proxy `:8082` (`HandleEmailAPI`, `internal/proxy/email.go:14-59`): cookie JWT or API key, then forwards to `127.0.0.1:8087` with `X-Authenticated-User`, `X-Authenticated-Email`, `X-Internal-Caller` and the hostauth credential (`email.go:44-48`) | loopback source, `X-Internal-Caller: true`, credential (checked only in hostauth enforce) |
| owner's computer (guest) | directly to the host tap address `:8087` (maild binds `0.0.0.0`, `nix/node-b.nix:629-632`; the guest gets `RUNTIME_MAILD_URL`, `nix/autoputer-vm.nix:406,437`) | tap source address; S1a asks vmctl which live ownership owns that address and requires its user to equal the asserted `X-Authenticated-User` (`internal/maild/api.go:129-154`) |
| Resend | Caddy `/api/email/resend/webhook` → maild (`nix/node-b.nix:384-386`) | Svix HMAC (`webhook.go:87-90`) |

Both of the first two end in the same function,
`authenticatedInternalOwnerWithEmail` (`api.go:208-234`), which returns
`(ownerID, email, ok)` and **no caller class**. Every route handler after it
treats the two callers identically.

## Defects, verified

| # | Defect | Evidence (code at `6b07fd651`) | Verdict | Severity |
|---|---|---|---|---|
| G1 | A guest bound to the owner can approve and send | `HandleDrafts` authenticates with `authenticatedInternalOwnerWithEmail` (`drafts.go:119-123`); the tap branch returns the owner after S1a (`api.go:229-233`); `POST /api/email/drafts/{id}/send` (`drafts.go:151-157`) calls `sendApprovedDraft(..., "owner_click_approved", "")` (`drafts.go:222-234`), which records an owner approval event and calls Resend (`drafts.go:359-389`). The guest reads the current `version_hash` from `GET /api/email/drafts/{id}` (`drafts.go:213-220`), so it has everything the send needs | **confirmed** | **High**: the approval gate is inside the blast radius of what it gates; any process in the guest (desk workers run as guest root, isolation review F4) sends as the user from `choir.news` |
| G2 | Approval replies are not sender-authenticated | `processApprovalReply` checks only that the parsed `From` equals the token's `approval_email` (`approval_reply.go:41-44`); no SPF, DKIM or DMARC result is consulted before `approve`, `reject` or `edit` acts (`approval_reply.go:64-101`). The bearer is the token in the reply address (`approval_reply.go:15-28`) | **confirmed** | **Medium**: anyone who sees an approval email (a forward, a shared screen, a mailbox rule) can approve by forging `From` |
| G3 | The authentication verdict is unaligned | `hasPassingAuthenticationResultEvidence` (`webhook.go:275-287`) returns true for a `dmarc=pass`, `dkim=pass` **or `spf=pass`** substring in **any** `Authentication-Results` **or `ARC-Authentication-Results`** header, without checking the authserv-id (who wrote it) or that the passing domain is the `From` domain. It gates "trusted sender" status on whitelist aliases (`webhook.go:243-255`) | **confirmed** | **Medium** by design, **Low-Medium** today: the verdict sets `trust_status=trusted`, `prompt_bar_equivalent` and `pending_email_appagent_intent` (`webhook.go:262-271`), which nothing consumes yet; it becomes an instruction channel the moment the email desk reads it |
| G4 | The `edit:` reply turns the owner's instruction into the email body | `UpdateDraftTextFromApprovalEdit` replaces `text_body` with `"Owner approval reply requested edits:\n\n" + editText`, bumps the version and leaves the draft pending (`drafts.go:818-859`); `processApprovalReply` then mails a fresh approval request for that body (`approval_reply.go:84-98`). The agent that would have rewritten the draft (`internal/agentcore/email_lifecycle.go`) was deleted in `9679837f3` | **confirmed** | **Low-Medium**: the owner sees the instruction-as-body in the next approval email, but one more `approve` sends the instruction itself to the recipient; the HTML body is left as it was, so the two bodies now disagree |
| G5 | Delivery feedback is recorded but never acted on | The webhook stores every event (`webhook.go:111-119`) but only `email.received` is processed (`shouldIngestEmail`, `webhook.go:156-158`); `email.bounced`, `email.complained`, `email.suppressed` stay as rows in `email_webhook_events`. Outbound messages are not in the global `email_provider_message_index` (`send.go:293-341` writes only the owner mailbox), so a bounce cannot even be mapped back to its owner without a scan | **confirmed** | **Low** today (every send is owner-approved, one real user), **Medium** once policy sends exist (E4) |
| G6 | The risk-alert and approval-email recipient comes from a header the computer sets | For a tap caller, `authenticatedInternalOwnerWithEmail` returns `normalizedTrustedEmail(r.Header.Get("X-Authenticated-Email"))` (`api.go:233`). `HandleRiskAlert` mails that address (`notifications.go:29-42,63-73`). **Also** `handleDraftApprovalEmail` mails it the draft (up to 4,000 characters of body) and binds the approval token to it (`drafts.go:236-283`, `CreateDraftApprovalToken`, `drafts.go:690-737`) | **confirmed, wider than reported** | **High** (raised from the isolation review's Medium, F10): see the chain below |

### The G6 → G2 chain: a guest send that survives a G1-only fix

The approval-email route is reachable from the guest, and the guest picks
its recipient. So, without touching the send route:

1. the guest creates a draft to any recipient (`POST /api/email/drafts`);
2. it asks for an approval email with `X-Authenticated-Email:
   attacker@example.net` (`POST /api/email/drafts/{id}/approval-email`);
   maild mints a token bound to that address and mails it there;
3. the attacker replies `approve` from `attacker@example.net`; the only
   check, `From == token.approval_email`, passes (`approval_reply.go:41-44`);
4. maild sends the draft as `email_reply_approved`.

The same route is also an open relay of up to 4,000 characters of
guest-chosen text from `updates@choir.news` to any address. A fix for G1
that does not also fix G6 and G2 leaves the gate open.

### A seventh defect found while verifying: the owner's UI is guest-served

| # | Defect | Evidence | Verdict | Severity |
|---|---|---|---|---|
| G7 | The Mail app that makes the owner's "approve" click is served by the guest, on the same origin as `/api/email` | `HandleComputerSurface` reverse-proxies Desktop, apps and assets to the vmctl-resolved guest after authentication (`internal/proxy/computer_surface.go:27-89`; `PlatformShellRoot` is only for unsigned callers, `internal/proxy/config.go:65-68`). The send call is a same-origin `fetch` (`frontend/src/lib/MailApp.svelte:752`), which carries the owner's `choir_access` cookie | **confirmed (new)** | **High**: a compromised guest can serve JavaScript that calls the send route from the owner's own browser session; to maild and the proxy it is an owner act. No change inside maild can tell the two apart |

G7 is a substrate problem (where the approval surface is served), not a maild
bug. It needs either an approval surface on a host-served origin the guest
cannot script (its own origin and cookie), or a per-approval user-verified
WebAuthn assertion bound to the version hash. E1 does not repair it; it is
recorded as a residual with that direction.

### Smaller gaps that bear on the fix

- **API keys reach the send route.** The proxy authenticates `/api/email/*`
  with a cookie **or** a `choir_sk_` API key (`internal/proxy/email.go:19-26`,
  `handlers.go:478-488`); a key with `write:runtime` or `admin` passes the
  scope check (`handlers.go:490-520`). A headless key is not an owner act,
  and a key handed to an agent inside the computer is G1 by another route.
- **Repeated headers collapse.** `resendReceivedEmail.Headers` is
  `map[string]string` (`resend.go:33`), so when a message carries several
  `Authentication-Results` headers only one survives, and which one is
  unspecified (UNVERIFIED: Resend's JSON shape for repeated headers was not
  observed). An aligned-verdict check is only as strong as the header it
  is handed.
- **The receiving provider's authserv-id is unknown.** No inbound fixture or
  doc in the repo records what Resend's MTA writes as the authserv-id
  (UNVERIFIED; the operator can read it from any real message's raw headers
  in the Mail app). A verdict that trusts "the provider's header" needs that
  value configured.

## Failure modes the repair must handle

Each is written before the code and becomes a test.

1. A guest tap caller bound to the owner posts `/drafts/{id}/send` with the
   current version hash: **403**, no provider call, no approval event, the
   draft stays pending.
2. The same guest adds `X-Internal-Caller: true` and a guessed hostauth
   header: still 403. The class comes from the transport, never from headers.
3. The owner's session through the proxy (loopback + credential) sends: still
   202 (no regression).
4. The guest can still create, list and read drafts, stage attachments and
   read mail (draft authority is kept; only approval moves).
5. The guest asks for an approval email with a forged
   `X-Authenticated-Email`: the email goes to the owner's address of record,
   never the forged one; if there is no address of record, nothing is sent
   and the call is refused.
6. The guest raises a risk alert with a forged email: same rule.
7. An approval reply with `From` = owner but no `Authentication-Results`:
   rejected, a risk alert goes to the owner, nothing is sent, the token stays
   usable by the real owner.
8. A reply whose `dmarc=pass` comes from an authserv-id that is not the
   configured provider's: rejected.
9. A reply with a trusted authserv-id whose `dmarc=pass` is for another
   domain (`header.from` ≠ the `From` domain): rejected.
10. A reply with `dkim=pass` for an unrelated domain, or `spf=pass` alone:
    rejected.
11. An `Authentication-Results` value that contains a line break or a second
    header name (headers joined into one value): no verdict (rejected).
12. A trusted, aligned `dmarc=pass` (or aligned `dkim=pass`) plus a live
    token: approved and sent exactly once.
13. `reject` and `edit:` replies need the same authentication as `approve`.
14. An authenticated `edit:` reply changes nothing that could be sent: body,
    version and hash unchanged, no provider call, no new approval email; the
    request is recorded for the future desk and the token is consumed.
15. A whitelisted sender on a trusted alias with only `spf=pass` or an
    untrusted authserv-id: refused as before for missing evidence;
    with a trusted, aligned result: accepted.
16. With no trusted authserv-id configured, every verdict is "unverified"
    (fail closed), and maild says so once at startup.
17. The rollback switch set to `legacy` restores the pre-repair behavior for
    G1, G2 and G3; an unknown value is treated as `enforce`.
18. The proxy refuses an API-key caller on the send route before maild is
    reached; path variants (`/send/`, extra slashes) are classified the same
    way maild parses them.

## Repair plan (E1), mutation class red

- **Caller classes in maild.** `authenticatedInternalOwnerWithEmail` returns
  a class: *host session* (loopback or unix transport, `X-Internal-Caller`,
  hostauth `Authorize`) or *guest* (tap source, S1a-bound). The send route
  requires the host session class. Guests keep draft, read, attachment and
  propose authority.
- **Owner address of record (G6).** Recipients of approval emails and risk
  alerts come from the owner's address of record, never from a guest header.
  maild cannot read the auth registry today (the auth DB is SQLite in WAL
  mode under a directory maild may not write, `nix/node-b.nix:527-538,627`,
  and the auth service has no internal lookup route), so the record is
  written only by the host session path: the proxy's `X-Authenticated-Email`,
  which comes from the auth-signed JWT `email` claim or the auth DB
  (`internal/proxy/handlers.go:469,580-590`). The auth service stays the
  authority; maild keeps a derived copy refreshed on every owner request.
- **Reply authentication (G2, G3).** One verdict function used by both the
  approval reply and the trusted-sender policy: only `Authentication-Results`
  whose authserv-id is in `MAILD_TRUSTED_AUTHSERV_IDS`; `dmarc=pass` with
  `header.from` equal to the `From` domain, or `dkim=pass` with `header.d`
  aligned to it (relaxed: equal or parent/child domain); `spf` alone and ARC
  headers never count; an explicit `dmarc=fail` vetoes. Unconfigured means
  unverified.
- **Edit replies (G4).** Recorded (`email_draft_edit_requests`) and never
  applied to the body until the email desk exists to consume them.
- **API keys (proxy).** The proxy refuses API-key auth on the send route.
- **Rollback switch.** `MAILD_SEND_GATE_MODE=legacy` (default `enforce`).
- **G5** is not wired in E1: there are no autonomous sends to pause, and
  mapping a bounce to its owner first needs outbound messages in the global
  provider index. It moves to E4 (policies) with per-user suppression.

## Rollback path

- Behavior: set `MAILD_SEND_GATE_MODE=legacy` in maild's environment and
  restart maild (restores G1, G2, G3 behavior; the G4 and G6 repairs stay,
  because neither can block a legitimate owner send).
- Code: `git revert` of the E1 commit; no schema change needs undoing (the
  two new tables are additive and unread by older code).

## Ordering relative to hostauth

The G1 repair does not depend on hostauth: a guest is classified by its tap
source address, which it cannot make loopback. Until hostauth moves from
`observe` to `enforce`, **any process in the host network namespace**
(Firecracker VMMs, SearXNG, goproxy) can still claim the host session class
on loopback; that is F2 in
[loopback-is-a-credential](../../problems/loopback-is-a-credential-2026-10-14.md), and
enforce closes it for maild's send route with no further maild change.

## Heresy delta

- **Discovered:** G1–G6 (re-verified), G6 is wider than reported (the
  approval-email route, the G6 → G2 chain, an open relay of draft text);
  G7, the guest-served approval surface; API keys on the send route;
  header collapse under the verdict.
- **Introduced:** none (docs only).
- **Repaired:** none (docs only).

## E1 status (branch `fix/maild-send-gate-2026-10-14`, not deployed)

| # | State | Where |
|---|---|---|
| G1 | **repaired** for direct callers: the send route needs the owner-session class; the proxy refuses API keys on it | `internal/maild/api.go` (`authenticateCaller`, `requireOwnerSession`), `drafts.go` (`HandleDrafts`), `internal/proxy/email.go` (`isEmailOwnerActRoute`) |
| G2 | **repaired**: every reply command needs a trusted, aligned DMARC or DKIM pass as well as the token; a refusal alerts the owner and leaves the token usable | `approval_reply.go`, `sender_auth.go` |
| G3 | **repaired**: one verdict (`senderAuthenticated`) for replies and trusted aliases | `sender_auth.go`, `webhook.go` |
| G4 | **repaired**: edit replies are recorded in `email_draft_edit_requests`, never applied | `approval_reply.go`, `drafts.go` (`RecordDraftEditRequest`) |
| G5 | **residual**, moved to E4 (see the plan above) | none |
| G6 | **repaired**, including the approval-email route: owner-facing mail goes to the address of record (`email_owner_addresses`, written only from an owner session) | `api.go` (`rememberOwnerAddress`, `ownerAddressOfRecord`), `notifications.go`, `store.go` |
| G7 | **residual** (substrate): needs a host-served approval origin or per-approval WebAuthn | none |

Residuals beyond G5 and G7:

- **The authserv-id must be configured.** Until `MAILD_TRUSTED_AUTHSERV_IDS`
  is set, approval-by-reply and trusted-sender aliases fail closed (the Mail
  app still approves). Whether the provider strips a sender-supplied
  `Authentication-Results` that claims its authserv-id is UNVERIFIED; the
  token remains a second factor if it does not.
- **Header collapse** (one value per header name) still decides which
  `Authentication-Results` the verdict sees; a value that looks like two
  joined headers is refused.
- **Relaxed alignment has no public-suffix list**: a DKIM pass for a public
  suffix with a dot (`co.uk`) would align; no such signer exists in practice.
- **The address of record is learned**: a guest-originated approval request
  or alert is refused until the owner has made one signed-in mail request
  since deploy. Reading the auth registry directly needs an auth-service
  lookup route (maild cannot read the WAL-mode auth DB without write access
  to its directory).
- **Host-namespace callers** can claim the owner-session class until
  hostauth `enforce` (F2).
- **No rate limit** on guest-requested approval emails to the owner.
- Edit requests are not yet shown in the Mail app.

## Conjecture

An approval is an owner act only if the bytes that produced it could not
have been produced by the computer being approved. E1 makes that true for
every path that reaches maild directly; G7 shows it is still false for the
path through the owner's own browser, which the computer renders.
