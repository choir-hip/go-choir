# A daily email of the top Autopaper stories

Draft 2026-10-20. Design only: no code changed. This document is green. The
class of the work it describes is in §9.

- Owner request, 2026-10-20: "Would be cool to get an email digest of
  autopaper every day."
- The digest is **outbound only** and is **not the email desk**. The
  [agentic email design](agentic-email-desk-2026-10-14.md) describes an agent
  desk that reads, drafts and proposes. Nothing here reads mail, drafts, or
  involves an agent. A fixed program picks stories and fills a fixed template.
- Sibling: [story expansion](story-expansion-request-design-2026-10-20.md).

Evidence labels: **code** (read at `e7ea4afda`, with a path), **doc** (a dated
Choir document), **dns** (a public lookup on 2026-10-20), **inference**
(reasoning, untested), **UNVERIFIED** (could not be confirmed from here).

## 1. Summary

1. **Choir already sends mail from the host.** `maild` sends through Resend
   from `Choir <updates@choir.news>` and has two fixed-template, host-initiated
   send paths: account recovery and risk alerts. The sender domain was
   reported verified on 2026-10-15 (doc). A digest is a third. No new
   provider, library or account is needed.
2. **maild is the home, a host timer the trigger.** maild holds the Resend key,
   the owner's address of record and can hold the subscriber table; corpusd
   hides secrets and should stay so. A systemd timer (as the daily compaction
   timer does) calls one host-internal maild route. No agent, computer or
   model call is involved; a digest costs about three-hundredths of a cent.
3. **The owner subscribes with one click while signed in.** That is the
   standing policy the email design already names ("a daily digest to their
   personal address"). The address is the account's address of record; none
   is typed, stored in the repo, or put in a config file.
4. **The one real gap is ranking.** The front page's order is computed in the
   browser (`frontend/src/lib/autopaper.ts`), not on the server. Phase 1
   mirrors it in Go behind a parity test and names the duplication as a
   residual; the substrate fix is a server-side rank (§4.3).
5. **Phase 1 is one night** and needs no credential or account action. The
   first real send needs one owner click. Whether the host has a Resend key is
   UNVERIFIED from here; a missing key fails closed and visibly (§8, M2).

## 2. Problem and intent

A reader has to open the site to see the day's paper. The owner wants it to
arrive. The first reader is the owner; there are no outside users yet.

**Intent.** One short, honest email a day: the stories that matter, each with
its headline, dek, section and source count, linked to the live page. Nothing
the site does not already show publicly. An empty day sends nothing.

**Non-goals.** No agent composes or sends. No inbound handling or reply
parsing. No open or click tracking, no remote images. No personalised ranking.
No outside subscribers in phase 1 (§11 sketches the path). No change to what
Autopaper publishes.

## 3. What exists today

### 3.1 The sender (code, doc, dns)

- **Provider client.** `resendClient.sendEmail` posts to Resend's `/emails`
  with a bearer key and an `Idempotency-Key` that hashes the payload
  (`internal/maild/resend.go:162`). The key is `RESEND_API_KEY` from
  `/var/lib/go-choir/maild.env` (`internal/maild/config.go`,
  `nix/node-b.nix:689`).
- **The precedent to copy.** `HandleAuthRecoveryLink`
  (`internal/maild/auth_mail.go:45`) is a host-internal route: loopback only,
  `X-Internal-Caller: true`, the host token (`hostauth.Authorize`), and a
  refusal of any request carrying `X-Authenticated-User`. It renders a fixed
  template and sends one message. The risk alert
  (`internal/maild/notifications.go:57`) does the same to the owner's address
  of record. Neither goes through the draft-approval gate, because neither is
  an agent send. The digest has the same shape.
- **Address of record.** maild keeps a per-owner address in
  `email_owner_addresses`, refreshed on every owner-session request
  (`rememberOwnerAddress`, `internal/maild/api.go:289`). The proxy forwards
  authenticated `/api/email/*` with `X-Authenticated-Email`
  (`internal/proxy/email.go:53`). A guest can never set it (G6 fix).
- **Sender identity (dns, 2026-10-20).** `resend._domainkey.choir.news` has a
  DKIM key; `send.choir.news` has SPF `include:amazonses.com` and an MX at
  `feedback-smtp.us-east-1.amazonses.com`; `_dmarc.choir.news` is
  `v=DMARC1; p=none;` with no `rua` report address; the root SPF is
  `include:_mailcust.gandi.net ?all`; the root MX is `inbound-smtp.us-east-1`.
  Inference: Resend signs as `d=choir.news` (aligned) and uses a `send.`
  return path (relaxed-aligned to the organizational domain), so DMARC should
  pass on both. I have not seen a delivered header; UNVERIFIED until §10.
- **Rate limit.** Resend allows 10 requests a second per team across all keys
  (doc: email design §3.4). One mail a day is far below it.

### 3.2 What maild does not do, and the scheduler precedent (code)

maild ignores delivery feedback: only `email.received` is acted on, while
`email.bounced`, `email.complained` and `email.suppressed` are stored unread
(`webhook.go`; defect G5 in the email design). Harmless for one owner,
blocking for outside subscribers. It has no scheduler. Host timers do that
work: `go-choir-front-importance` reads the front list at
`http://127.0.0.1:8086/internal/platform/publications/front?limit=100`
(`nix/node-b.nix:966-1010`); `go-choir-store-compaction` runs daily at 04:00
UTC with `Persistent = false` (`:1401`), the schedule the owner approved on
2026-10-20 (notes §26); `go-choir-disk-gc` curls a loopback internal route
with the host token header from `$CREDENTIALS_DIRECTORY` (`:195-215`).

### 3.4 The front list and its order (code)

The internal front route returns up to 100 stories newest first. Each row has
title, dek, routes, `updated_at`, `version_count`, `source_count`, `words`, and
`importance`, `promo` and `section` when the host scorer's file is fresh
(under 24 hours) (`publication_front.go`, `internal/frontimportance`).

The ranking is in TypeScript: `importanceProminence` multiplies importance
(unscored counts as 4) by a depth factor (one source halves it, two sources weigh
0.75, a body under 150 words times 0.8) by a recency decay with a 24 hour half
life; promotional and "Open a story..." titles rank last; a story with fewer
than two sources may not lead (`autopaper.ts:183-300`). Nothing on the server
computes it. The mail is built on the server, so the rule must exist there.

## 4. The design

### 4.1 Shape

```text
systemd timer (12:00 UTC, Persistent=false)
  -> oneshot: curl POST http://127.0.0.1:8087/internal/maild/digest/run + host token
       maild (host-internal only):
         1. read subscribers (digest_subscriptions where enabled)
         2. fetch the front list from corpusd (internal, host token)
         3. select and rank stories (deterministic Go function)
         4. per subscriber: skip if today's run row exists; insert it; send
            via Resend with Idempotency-Key per (owner, UTC date); record fate
```

Owner surface, through the existing authenticated email proxy path:

- `GET /api/email/digest` returns `{subscribed, address_masked, last_run}`;
  `last_run` holds date, kind, status, provider message id, the story ids and
  `ranked_by`.
- `PUT` and `DELETE` on the same path subscribe and unsubscribe.
- `POST /api/email/digest/send-now?preview=1` returns the selected stories
  and the rendered text without sending. Without `preview` it sends now to the
  caller, limited to one an hour. It is the acceptance and preview path.

All three are owner acts: they need the browser session, and an API key is
refused, as for mail sends (`isEmailOwnerActRoute`, `internal/proxy/email.go`).

### 4.2 Why maild and a host timer

A maild route plus a host timer matches two precedents, keeps the Resend key
in maild, and makes a missed tick a missed digest rather than a backlog. A
corpusd job is wrong: corpusd hides secrets and has no mail identity. An
in-process ticker in maild is wrong: a deploy restart near 12:00 skews or
double-fires it, while a timer unit is visible in nix and `systemctl`. A
computer's schedule (management or Autoradio) fits later, if the digest
becomes an agent-composed edition. The SW design says email distribution is a
separate channel owned by management (`autopaper-sw-design-2026-10-09.md`
§3); a deterministic digest is not an agent act, so it needs no desk (Q3).

`Persistent = false`: after a host outage the timer does not fire a stale
digest. This is CLAUDE.md "Restarts End Work": a missed run is closed, not
resumed.

### 4.3 Choosing the stories

Rule, in one place, as a pure function over the front rows and a clock:

1. Candidates: rows updated in the last 24 hours, not `promo`, title not
   "Open a story...", at least 2 sources (unknown source count is allowed, as
   on the front page).
2. Score: importance (4 if unscored) times depth factor times
   `0.5^(age/24h)`, age from the clock at run time.
3. Take the top 8, at most 3 from one section. The lead is the first story
   that may lead (two or more sources).
4. **No repeats.** A story sent before is skipped unless it has a newer
   version than the one last sent (`digest_sent(owner, publication_id,
   version)`); a revised story is marked "Updated".
5. If fewer than 3 stories qualify, send nothing and record `skipped:
   too_few`. Never send a digest to say there is no news.
6. If scores are stale the rank falls back to recency; the run row records
   `ranked_by: recency` so a poor digest is explainable.

**The duplication, named.** This mirrors `autopaper.ts` in Go. Phase 1 adds a
parity test: one JSON fixture of front rows is ranked by the TypeScript
function (node) and the Go function, and the orders must match. That makes
drift a CI failure. It is a heresy (two copies of a rule) that I introduce
knowingly. The substrate fix is to compute rank once on the server and have
both the page and the digest read it. That is a front-list change with its own
red ceremony, so it is a named residual, not a phase 1 task.

### 4.4 The message

- **From** `Choir <updates@choir.news>` (phase 1; see §6). **To** the owner's
  address of record only. **Reply-To** unset; the footer says replies are not
  read yet (a no-reply sender costs a little deliverability; one owner
  accepts it).
- **Subject** `Autopaper, Tue 21 Oct: <lead headline>`, cut to 120
  characters, CR and LF removed.
- **Body.** Plain text and a small HTML part. Per story: headline as a link to
  `https://choir.news/autopaper/<slug>`, the dek, "World, 5 sources, updated
  3 hours ago". Grouped under the lead and then by section labels from
  `autopaper.ts` (World; U.S. Politics; Business & Finance; Tech & AI).
- **Footer.** "Written by AI agents from cited sources. Open any story to see
  them" (the disclosure line the owner ruled on 2026-10-20, notes §27 item
  14); "Manage this email: https://choir.news/?app=email"; the standard
  automated signature.
- **Never** remote images, web fonts, tracking pixels or redirecting links.
  All text from stories is HTML-escaped; links are built from slugs, not from
  model text.
- **Headers.** `X-Choir-Maild: v0-autopaper-digest`;
  `List-Unsubscribe: <https://choir.news/?app=email&digest=off>` (RFC 2369;
  phase 1 has one subscriber, so one-click POST waits for phase 2).

### 4.5 State

Three additive tables in maild's routing SQLite, beside
`email_owner_addresses`: `digest_subscriptions`, `digest_runs` (owner, UTC
date, kind, status, provider message id, story ids, `ranked_by`;
`UNIQUE(owner, run_date, kind)`) and `digest_sent` (the version last sent per
story). The address is read from `email_owner_addresses` at send time, never
copied. The front list stays the authority for stories; losing these tables
loses history and the subscription, not content.

## 5. Cost

No model calls in v1. One subscriber is 30 mails a month; Resend Pro is $20
for 50,000 with overage at $0.90 per 1,000 (doc: email design §3.6; the
owner's plan is UNVERIFIED), so well under $0.03 at overage rates. At 1,000
subscribers (phase 2), 30,000 a month fits inside Pro. An optional model
written intro would cost about $0.00005 a call with Jev (handoff, event
layer); it is excluded from v1 because it adds an agent decision to a
deterministic mail, for no request from the owner.

## 6. Deliverability

- **Authentication.** DKIM aligned, SPF on the return-path domain, DMARC
  present (dns, inference). Gmail and Yahoo require this of bulk senders and
  one-click unsubscribe above 5,000 a day; phase 1 is far below that. The
  owner confirms the real result with "Show original" on the first message.
- **DMARC reports.** `p=none` with no `rua` means nobody sees failures. A
  `rua` address is a DNS edit only the owner can make: advised, not blocking.
- **Shared reputation.** The digest shares `updates@choir.news` with account
  recovery and risk alerts. At one mail a day that is no risk. Before outside
  subscribers, move it to its own sender subdomain (the email design proposes
  `notify.` for transactional mail) so a newsletter complaint cannot hurt
  sign-in recovery. That needs a Resend domain and DNS records: owner action,
  phase 2.
- **Spam placement.** A new daily sender may land in spam or promotions at
  first. A plain text part, no images, few links and a stable From help; the
  owner marking the first one "not spam" helps more.
- **Feedback loops.** Bounces and complaints are unread (G5): irrelevant for
  the owner, the first thing to build for anyone else.

## 7. Unsubscribe and consent

Phase 1: the owner opts in and out with a toggle in the Mail app; the footer
and the `List-Unsubscribe` header point at it. Unsubscribing is immediate: the
next run reads the table. The subscription is an owner act through the
session, the email design's "policy grant" for a recurring send to the owner's
own address; a guest or an API key cannot subscribe the owner.

Outside subscribers (phase 2) need double opt-in, a signed unsubscribe token
that works without sign-in, RFC 8058 one-click POST (a public maild route,
sent by Caddy as the webhook route is), a suppression list fed by bounces and
complaints, a postal address in the footer for CAN-SPAM, and unsubscribe
honoured within a day. Signing tokens needs a new secret file, an owner
credential action, so it waits.

## 8. Ways it could fail

Written before code; tests follow this list.

- **M1 Duplicate sends.** A rerun, a timer double-fire or a retry after a
  maild restart. `UNIQUE(owner, run_date, kind)` plus an explicit Resend
  `Idempotency-Key` of `choir_digest_<owner>_<date>` (a small change: today the
  key hashes the payload, which changes if the front list changes between
  attempts). Test: call the run route twice, count messages.
- **M2 The Resend key is missing or wrong.** `sendEmail` returns "RESEND_API_KEY
  is not configured"; the run row is `failed: provider` and the Mail app shows
  it. Fails closed, loudly in the log.
- **M3 Resend is down or throttles (429/5xx).** One retry after 30 seconds,
  then `failed`. No catch-up loop; `send-now` is the manual retry.
- **M4 Corpusd or the front list is unavailable.** `skipped: front_unavailable`.
  No computer is involved, so a stopped paper is not a failure.
- **M5 Stale scores.** Fallback to recency, flagged in the row. **M6 Repeats.**
  `digest_sent` suppresses them; a test runs two consecutive days.
- **M7 Hostile or broken text.** Titles and deks are model-written. All text is
  escaped, CR and LF stripped from the subject, links built from slugs, no
  remote content. A fixture with `<script>`, a quote and a line break in a
  title asserts the output.
- **M8 No address of record.** Recorded as `skipped: no_address`; the subscribe
  call itself records one (it is an owner-session request).
- **M9 A missed day or a restart mid-run.** No catch-up and no boot resumer.
  The row stays `started`; the Mail app shows the last run date; `send-now`
  fills a gap.
- **M10 Mail lands in spam.** See §6; the owner is the detector.
- **M11 Wrong base URL.** One environment, `choir.news`; the base comes from
  `MAILD_PRIMARY_DOMAIN`.
- **M12 The send gate is weakened.** The new routes are session-only, the run
  route is host-internal (loopback, header, host token, no
  `X-Authenticated-User`); a guest on the tap cannot reach it. A test calls
  the run route from a non-loopback address and from a request carrying an
  authenticated-user header and expects 403.
- **M13 The ranking copy drifts from the page.** The parity test (§4.3).

## 9. Mutation class and ceremony

**Phase 1: red.** It adds an outbound send path on the shared sending domain,
a route in the mail send gate's neighbourhood (owner acts that cause mail),
and a new host timer in `nix/node-b.nix` (a deploy surface). Texture,
vmctl, the gateway, auth and the object graph are untouched. The ranking
function and the Mail app toggle are orange within it.

**Conjecture delta.** A deterministic top-stories email, ranked as the front
page is, is worth opening daily. Falsified if the owner stops opening it in a
fortnight or finds its picks unlike the front page.

**Heresy delta.** Discovered: the front rank lives only in the browser; the
mail path has no scheduler; delivery feedback is unread (G5, existing).
Introduced: a Go copy of the rank, held by a parity test. Repaired: none.

**Problem first.** One commit adds `docs/problems/` notes for "front rank is
client-only" and "Resend idempotency key hashes the payload", before code.

**Standing questions.** Provenance: the owner's quote above. The front list is
the authority for stories and the auth registry for the address; the digest
tables are derived. The digest depends on maild, corpusd and Resend, not on
any computer. No resumer; one run a day by a unique key. Operable without SSH
through `GET /api/email/digest`, `send-now?preview=1` and the log. No object
kinds; the timer is declared in nix.

## 10. Acceptance proof on staging

Staging is `https://choir.news`. A deployed spec,
`frontend/tests/autopaper-digest-deployed.spec.js`, with a signed-in test
account (a QA account that has used an email route, so it has an address of
record). It never signs in as the owner.

**API assertions.**
1. Signed out: `GET /api/email/digest` is 401. An API key on `PUT` is 403.
2. `PUT` subscribes; `GET` shows `subscribed:true` and a masked address.
3. `send-now?preview=1` returns 3 to 8 stories, each in the live front list,
   none `promo`, none known to have under 2 sources, at most 3 per section, in
   the order the parity-tested rank gives for the list the spec just read.
4. The rendered text has no `<script`, no `http:` link, no image tag, and
   every link starts with `https://choir.news/autopaper/`.
5. `send-now` returns a provider message id; a second call within the hour is
   refused; `DELETE` unsubscribes and a later `send-now` is refused.

**Scheduled run.** Next day, `GET /api/email/digest` for the owner's QA
account shows `kind: scheduled`, `status: sent` for today's UTC date. This
proves the timer and the route, which the spec cannot trigger (the run route
is internal). The first scheduled run is observed by a person reading the
inbox and the account's last-run row.

**Deliverability proof (human step).** The owner opens the first real message
in the inbox, uses "Show original", and confirms SPF, DKIM and DMARC all show
`pass`, and which folder it landed in. The result goes in the final report.
An automated variant that sends to a mailbox maild itself receives
(`000@choir.news`) could read the `Authentication-Results` header on the
inbound copy; whether that mailbox is readable by a test account is
UNVERIFIED, so it is not relied on.

**The artifact.** JSON from the spec: the front list read, the preview story
ids with their ranks, the send result, the run rows for two days, and the
rendered text. Repeatable by re-running the spec.

**Final evidence** names the pushed SHA, CI run, staging identity, spec
result, artifact, the owner's header check, class and surfaces, heresy delta
and the next realism axis (a second recipient).

## 11. Phased plan

### Phase 1: one night, red

1. Problem notes commit (green). Then tests first, from §8: parity fixture,
   render fixture, uniqueness and idempotency, gate refusals, selection rules,
   two-day no-repeat.
2. maild: the tables, selector, renderer, the three owner routes, the
   host-internal run route, the explicit idempotency key, the front-list
   client (`MAILD_CORPUSD_URL`), the kill switch `MAILD_DIGEST=off`; extend
   `isEmailOwnerActRoute` to the write routes.
3. Nix: `go-choir-digest` oneshot and timer (`OnCalendar = "*-*-* 12:00:00
   UTC"`, `Persistent = false`, host token by `LoadCredential`, a curl to the
   run route). Frontend: a toggle and last-run line in the Mail app.
4. The §10 spec; landing loop: commit, push, CI, deploy, identity, spec. The
   owner then opts in and checks the headers.

No credential, secret or account action is needed to build or land it. The
first real send needs the owner's one click. If the host has no Resend key,
M2 reports it and that one fact becomes an owner action.

### Phase 2: before any other subscriber

Bounce and complaint handling (G5) and a suppression list; a separate sender
subdomain with Resend and DNS setup; double opt-in, signed unsubscribe token
(a new secret) and RFC 8058 one-click; server-side rank shared by page and
digest; DMARC `rua`; per-section or weekly editions on request.

## 12. Rollback

Set `MAILD_DIGEST=off`: the run route refuses and the timer's call does
nothing. Or stop and disable `go-choir-digest.timer`, or `DELETE` the
subscription. Revert the commit to remove the code; the tables are additive and
can stay. Mail already sent cannot be recalled. No product restore path is
involved: nothing here is computer state.

## 13. Questions for the owner

Each is answerable yes or no; the recommendation is mine.

1. Send at 12:00 UTC every day, and skip a day with fewer than three
   qualifying stories? Recommend yes.
2. Send from the existing `updates@choir.news` for the owner only, and move to
   a separate subdomain before anyone else subscribes? Recommend yes.
3. Keep the digest a deterministic host job with your opt-in as its policy,
   rather than a management or email-desk act? Recommend yes.
4. Opt in with a toggle in the Mail app, using your account's address of
   record? Recommend yes.
5. No model-written text in v1; the footer carries the AI disclosure line?
   Recommend yes.
6. Accept the Go copy of the front ranking with a parity test now, and fix it
   properly (server-side rank) later? Recommend yes.
7. Add a DMARC `rua` address you read, as a DNS edit by you, when convenient?
   Recommend yes; not blocking.
8. Keep outside subscribers out until bounce and complaint handling exist?
   Recommend yes.
