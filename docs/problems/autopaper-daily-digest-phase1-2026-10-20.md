# Autopaper daily digest, phase 1: two problems found before building (2026-10-20)

Status: phase 1 built on `worktree-agent-acf270b75bf8b2602` (not pushed, not
deployed, not run against staging; see "As built" below). Mutation class of the work: **red** (a new
outbound send path on the shared sending domain, a host timer in
`nix/node-b.nix`, a route beside the mail send gate). The Mail app toggle and
the ranking function are orange within it.
Design: [autopaper-daily-digest-design-2026-10-20.md](../reports/autopaper-daily-digest-design-2026-10-20.md).
Owner request, 2026-10-20: "Would be cool to get an email digest of autopaper
every day."

## Problem 1: the front-page rank exists only in the browser

`rankFrontStories` (`frontend/src/lib/autopaper.ts`) orders the front page.
Nothing on the server computes it; corpusd returns rows newest first. A mail
built on the server must pick "the stories that matter" and today has no rule
to call.

Phase 1 mirrors the rule in Go (`internal/maild/digest_rank.go`) and holds the
two copies together with a parity test that ranks one JSON fixture with the
TypeScript function (node) and the Go function and requires the same order.
This is a heresy I introduce knowingly: two copies of one rule. The substrate
fix is a server-side rank both the page and the digest read; it is a
front-list change with its own red ceremony and stays a named residual.

Note on the clock: the design text says age is measured "from the clock at run
time". The page's rank reads no clock; it measures age from the **edition
time** (the newest update in the list). The mirror follows the page, not the
design sentence, so the digest's order is the front page's order. The
24-hour candidate window is the only place the digest reads a clock.

## Problem 2: the Resend idempotency key hashes the payload

`resendSendIdempotencyKey` (`internal/maild/resend.go`) is a hash of the
request body. A digest retry after a restart, or a second run after the front
list changed, builds a different body and therefore a different key, so the
provider would not deduplicate it. The daily send needs a key that names the
send (`choir_digest_<owner>_<UTC date>`), not its content.

Phase 1 adds an optional explicit key on the send request (unexported field,
not serialised); every existing caller keeps the payload hash.

## Defaults taken for the design's eight owner questions

The owner left the night to us; `CLAUDE.md` "No Blocking Asks" says take the
conservative option and state it. Each recommendation in design section 13 is
taken as the default:

1. Send at 12:00 UTC daily; skip a day with fewer than three qualifying
   stories. Yes.
2. Send from the existing `updates@choir.news` to the owner only; a separate
   subdomain before anyone else subscribes. Yes.
3. A deterministic host job with the owner's opt-in as its policy, not a
   management or email-desk act. Yes.
4. Opt in with a toggle in the Mail app, using the account's address of
   record. Yes. **Nobody is opted in by default.**
5. No model-written text; the footer carries the AI disclosure line. Yes.
6. Accept the Go copy of the rank with a parity test now; server-side rank
   later. Yes.
7. DMARC `rua` address: a DNS edit only the owner can make. Advised, not
   blocking, not done here.
8. No outside subscribers until bounce and complaint handling exist (G5).
   Yes: the subscribe route accepts only the signed-in owner's own address.

## Failure modes

The design's M1 to M13 are the list; the test files carry them as header
comments, written before the tests and the code. Two are added here:

- M14 The timer calls the run route without the host-internal credential, or
  a guest on the tap does. The route checks loopback, the marker header, the
  credential (`hostauth.Authorize`) and refuses any `X-Authenticated-User`.
  The timer sends the credential header from `$CREDENTIALS_DIRECTORY`; it does
  not rely on loopback alone. If the credential file is unreadable the timer
  unit fails visibly rather than calling without it.
- M15 The Resend key is missing at run time. The run row is
  `failed: provider`, the log says so, the unit exits non-zero (a failed
  systemd unit is the visible signal), and nothing is marked sent.

## Heresy delta

Discovered: front rank is client-only; the mail path has no scheduler; the
idempotency key names the content, not the send; delivery feedback is unread
(G5, existing). Introduced: a Go copy of the rank under a parity test.
Repaired: none.

## Rollback

`MAILD_DIGEST=off` (run route refuses, subscribe stays readable), or stop and
disable `go-choir-digest.timer`, or `DELETE` the subscription, or revert the
commit. Tables are additive and can stay. Mail already sent cannot be
recalled.

## As built (2026-10-20)

Where: `internal/maild/digest*.go` (rank mirror, selection, render, store,
routes), `cmd/maild/main.go` (boot closes started runs), `internal/proxy/email.go`
(digest writes are owner acts an API key cannot do), `nix/node-b.nix`
(`go-choir-digest` oneshot and timer, `MAILD_CORPUSD_URL`, `MAILD_DIGEST`),
`frontend/src/lib/MailApp.svelte` (the toggle), specs under `frontend/tests/`.

How the owner opts in: Mail app, signed in, the calendar button in the list
header, switch On. It is the account's address of record; no address is typed.

Timer: 12:00 UTC daily, `Persistent = false`, a oneshot that curls
`http://127.0.0.1:8087/internal/maild/digest/run` with `X-Internal-Caller: true`
and the host-internal token header from `$CREDENTIALS_DIRECTORY`. It exits 1 if
the credential is unreadable. curl `--fail-with-body`, so a 503 (no Resend key)
or 502 (provider failure) fails the unit.

Deviations from the design, each chosen for a stated reason:

1. The rank measures age from the edition time, not the run clock (the page
   reads no clock); the 24 hour candidate window is the only clock read.
2. The digest reads 60 stories, the page's first load, not 100.
3. `UNIQUE(owner, run_date, kind)` is a partial unique index on scheduled rows
   only, so send-now can run once an hour. Scheduled reruns the same day are
   allowed after a skipped, failed or interrupted row (Resend's key dedups a
   send that did land); a started or sent row is never rerun.
4. Preview and send-now do not read or write the no-repeat ledger: the ledger
   belongs to the scheduled series, and a manual send must not hide stories
   from tomorrow's mail. Stories with no section are not capped as a group.
5. A missing Resend key answers 503 even with nobody subscribed, so the unit
   fails visibly from the first deploy rather than at the first opt-in.
6. The footer link `https://choir.news/?app=email` lands on the desktop, not
   the Mail window: a bare `?app=email` is consumed by the shell and opens
   nothing (an existing spec asserts that). Residual.

Residual risks: the Go rank is a copy (parity test guards drift only where
node is on PATH, `CHOIR_REQUIRE_NODE=1` makes its absence a failure; the
committed golden guards the Go side everywhere); the Resend key's presence on
the host is unverified; DMARC `p=none` has no `rua`; deliverability is
unproven until the owner reads "Show original" on the first message; bounce
and complaint handling (G5) still blocks any second recipient.

Acceptance (not run): `frontend/tests/autopaper-digest-deployed.spec.js` after
the landing loop. It registers a QA account at `delivered+digest<run>@resend.dev`
(Resend's sink), checks 401 signed out, opt-in, a preview of 3 to 8 stories
that are on the live front list, in page order, with safe links, one real
send-now, the one-an-hour limit and opt-out. A second run armed with
`DIGEST_ARM_SCHEDULED=1`, then `DIGEST_CHECK_SCHEDULED=1` after 12:00 UTC the
next day, proves the timer.
