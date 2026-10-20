# Autopaper daily digest, phase 1: two problems found before building (2026-10-20)

Status: documented; phase 1 code follows on `worktree-agent-acf270b75bf8b2602`
(not pushed, not deployed). Mutation class of the work: **red** (a new
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
