# The owner's Claude token sits on three computers, one of them an engineering arm (2026-11-03)

Date: 2026-11-03. Mutation class of any fix: red (credential custody, the
guest credential disk, gateway and provider calls, capsule egress). Problem
first; no fix in this commit. It extends SEC-08,
[claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md](claude-delegate-key-sits-on-the-guest-disk-2026-10-21.md),
with today's deployment and the capsule overlay.

Owner, 2026-11-03: "there shouldn't be credentials on computers anyway."

## The problem in one sentence

The only credential found on any computer that unlocks something outside
it is the owner's Claude OAuth token, which vmctl writes onto the
credential disk of every computer in `CHOIR_CLAUDE_DELEGATE_COMPUTERS`
(three today, one of them the Claude arm of the self-development apps
goal), and which a `claude`-mode capsule receives with egress to
`api.anthropic.com`, so the gateway neither holds the token nor meters the
calls.

## Evidence

- The inventory of d@e for the capsule overlay
  ([capsule-overlay-s0-measurement-2026-11-03.md](../evidence/capsule-overlay-s0-measurement-2026-11-03.md),
  "Secret-shaped material"): names, owners, modes and sizes only. d@e is
  not on the delegate list and has no such token; its credential disk holds
  `gateway-token`, `host-fetch.json` and `capsule-mirrors.json`.
- `nix/node-b.nix` (go-choir-vmctl Environment):
  `CHOIR_CLAUDE_DELEGATE_COMPUTERS` names three computers, the third added
  2026-11-03 for `selfdev-arm-claude`; `CHOIR_CLAUDE_DELEGATE_EXPIRES` is
  2026-11-17.
- `internal/vmmanager/claude_delegate.go` (`claudeDelegateMaterial`)
  writes `claude-delegate.json` and `claude-oauth-token` (0400, root) onto
  the credential disk at each cold boot of a listed computer;
  `internal/capsule/claude_delegate.go` hands the token to a `claude`-mode
  capsule's broker and allows CONNECT to `api.anthropic.com` with a
  512 MiB upload budget.
- The capsule overlay design
  (`docs/design/capsule-overlay-computer-state-2026-11-03.md`) gives
  engineering capsules a copy of the computer's files and store. On a
  listed computer, a capsule that holds both that copy and the token would
  have an upload path, through the token, to a host outside Choir's meter.

## Why it matters

- It is the one place the platform rule "provider keys live in the
  gateway" does not hold; every other provider call goes through the
  gateway, which holds the keys and meters the calls.
- A guest-root process on a listed computer can read the token (SEC-08).
- The overlay makes the capsule a holder of owner data; the token's egress
  is the widest upload path from such a capsule.

## Proposed removal (not built here)

Move the delegate call behind the gateway, as every other provider call
is: the gateway holds the token, the capsule's `claude` CLI talks to a
gateway route with the computer's gateway identity, and the credential
disk carries only the grant record. SEC-08's station (FP7) has the gateway
half built and dark; the guest half is the remaining work.

Until it lands, the overlay and the delegate meet on the listed computers:
`engineeringCastNetwork` (`internal/agentcore/engineering_assignment_runtime.go:497`)
makes an engineering Cast `claude`-mode whenever the delegate is available
and the Cast does not ask for the ecosystem proxy, so an engineering
capsule on the Claude arm would hold the computer-state copy, the token
and `api.anthropic.com` egress together. The overlay reaches those
computers only by the ring walk; this record is the reason to settle the
delegate before that ring.

## Design: the delegate calls Anthropic through the gateway (2026-11-03)

Mutation class red: gateway and provider calls, credential custody, the
guest credential disk, capsule egress, the guest security epoch. No
switch: the mechanism replaces the token copy outright, for every computer
the grant names.

### Where the token lives

Only on the host. The owner's file stays at
`/var/lib/go-choir/secrets/claude-oauth-token` (root 0400). The gateway's
start stage (a `+` ExecStartPre, the pattern vmctl used) copies it into the
gateway's private runtime directory, which systemd removes when the gateway
stops; `GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE` names the copy. vmctl stops
reading it, and its stage script, its `CHOIR_CLAUDE_DELEGATE_TOKEN_FILE` and
`CHOIR_CLAUDE_DELEGATE_EXPIRES`, `claudeDelegateMaterial` and the
`claude-delegate.json` and `claude-oauth-token` credential-disk files are
deleted. No computer's credential disk carries either file after its next
cold boot.

### The gateway half (FP7's relay, finished)

`internal/gateway/anthropic_relay.go` (`/provider/anthropic/v1/messages`,
`/messages/count_tokens`, `/models`) was built for a Console API key and is
dark. It gains a second credential kind and keeps everything else:

- Credential: the OAuth token file when it holds a valid token, else
  `ANTHROPIC_API_KEY`; chosen once at start and named (by kind, never by
  value) in the startup log and `/health`. Upstream, the OAuth kind sends
  `Authorization: Bearer <token>` and adds `oauth-2025-04-20` to
  `anthropic-beta`; the key kind sends `x-api-key`. Nothing the caller sent
  that authenticates crosses.
- Who: the caller's gateway bearer, validated without writing identity
  state, then the relay's own pin check (the bearer must already be pinned to
  the caller's tap address, in every binding mode: SEC-08 findings 11 and
  12). The identities and window stay `GATEWAY_ANTHROPIC_RELAY_IDENTITIES`
  and `GATEWAY_ANTHROPIC_RELAY_EXPIRES`.
- Spend cap: a per-computer UTC-day token budget
  (`GATEWAY_ANTHROPIC_RELAY_DAILY_TOKENS`; input, output and cache-creation
  tokens, cached reads excluded), checked before each call and charged
  after it; a spent budget answers 429 `rate_limit_error`. The per-computer
  in-flight cap and the gateway's fixed-window limiter stay.
- A grant read: `GET /provider/anthropic/grant` answers the calling
  computer, after the same bearer and pin checks, whether the relay serves
  it now (and until when, which model, the day's cap and use), never naming
  another computer or the credential. The guest reads its availability
  here, so the grant has one authority (the gateway) instead of a
  credential-disk copy that went stale until a cold boot.

### The guest half

- The capsule executor asks the gateway for the grant (cached 30 s, 3 s
  timeout, its own lock, never `e.mu`), instead of reading the credential
  disk.
- A claude-mode capsule's egress policy has no CONNECT host at all
  (`api.anthropic.com` leaves it) and one new route,
  `anthropic.choir.internal`, which the egress proxy (in the guest core,
  outside the capsule) serves itself: only the relay's three paths, POST or
  GET, a 32 MiB body bound, charged against the capsule's upload and
  download budgets. It checks the capsule's relay secret, removes every
  caller credential, adds the computer's gateway bearer (read from the
  credential disk by the guest core, as every other gateway call does) and
  forwards to the gateway's `/provider/anthropic/`; the answer is spliced
  back unbuffered (server-sent events). It accepts the absolute-form request
  a client sends to an HTTP proxy and, for a client that tunnels even plain
  HTTP, a CONNECT to port 80 of that one name followed by an origin-form
  request.
- The capsule's relay secret: 32 random bytes, minted by the executor at
  spawn, alive only as long as the capsule, carried to the broker the way the
  token was (environment, removed at once) and given only to the claude
  process. It is not a credential to anything outside the capsule's own
  egress proxy: it lets that proxy tell the delegate's claude process from
  a desk cell in the same capsule, so a cell cannot spend the grant on its
  own (the token gave the same separation before). The Freeze scan that
  refused a candidate carrying the token now scans for the secret.
- `claude` gets `ANTHROPIC_BASE_URL=http://anthropic.choir.internal`,
  `ANTHROPIC_AUTH_TOKEN=<relay secret>`, and the small and fast model
  variables set to the delegate's one model, and no
  `CLAUDE_CODE_OAUTH_TOKEN`.

### The grant source: the list stays, now in the gateway (follow-up named)

The owner ruled (switch audit, group 7) that the delegate becomes an owner
act per computer, with a spend cap, recorded on the tape. That is not
bounded inside this change: the computer's own tape cannot be the
authority for a host-held credential (guest root writes that tape), so the
act has to be authenticated by the proxy (the owner-act receipt signer,
`internal/owneract`), stored on the host where the gateway reads it, and
mirrored to the tape as a record. Here the list keeps its three computers
and moves to its one reader, the gateway; one nix let-binding also feeds
vmctl's tool profile (`delegate=<ids>`, which attaches the Claude binary
layer), replacing the `grant:claude-delegate` selector token. The spend cap
lands now as a bound the owner act will carry per computer. Follow-up:
"delegate grant as an owner act" (proxy route, signed receipt, host grant
store read by the gateway and vmctl, tape record, revoke), after which the
let-binding and `GATEWAY_ANTHROPIC_RELAY_IDENTITIES` are deleted.

### Anthropic's terms (residual, the owner's to rule)

SEC-08 recorded that the terms forbid a service that "collects, stores, or
intermediates" a Claude.ai subscription token, and the FP7 relay was built
for a Console key for that reason. This change holds the owner's own token
on the owner's own host and serves only the owner's own computers, which is
the same use as before with the copy moved off the guests; it is still a
relay of a subscription token. The relay takes a Console key with no code
change (remove the token file, put `ANTHROPIC_API_KEY` in
`gateway-provider.env`, restart the gateway). Recorded, not decided here.

### Security epoch

The change removes a credential from guests, so it raises
`security_epoch` (35 to 36). A hibernated listed computer would otherwise
resume onto its old release and its old credential disk, token included;
with the raise every older snapshot refuses at wake and cold-boots onto a
disk without it. Running listed computers keep the old disk until their
next cold boot; the proof cold-boots the arm.

### Ways this can fail (written before the code)

SEC-08's ten failure modes and findings 11 and 12 hold for the relay and
keep their tests. New ones:

13. The OAuth token reaches a log, an error body, `/health`, the grant read
    or a guest (redaction covers the token value as well as key shapes).
14. A missing or malformed token file falls back to the API key per call,
    or the reverse; the kind is chosen once at start, and an unusable
    choice leaves the relay off.
15. The egress route forwards a caller's `Authorization` or `x-api-key`
    to the gateway, or forwards anything but the three relay paths
    (`/provider/anthropic/grant`, other gateway routes, a path with `..`).
16. A desk cell in the capsule calls the route without the relay secret
    and is served (the secret check is constant time and happens before any
    byte goes to the gateway).
17. The computer's gateway bearer reaches the capsule: in a response
    header, an error body, the egress ledger, or by a redirect.
18. The route buffers a streamed answer, so `claude` stalls.
19. A claude-mode capsule can still CONNECT to `api.anthropic.com`, or any
    other host.
20. The grant read hangs a spawn or the observability read behind the
    network, or holds `e.mu`; a failed read must say unavailable with a
    reason, never available.
21. The spend cap is charged to the wrong computer, is never charged on a
    streamed reply, or lets a call start once spent.
22. The relay secret is predictable or outlives its capsule, or the Freeze
    scan loses its secret and lets a candidate through unchecked (the scan
    still refuses to freeze without one).
23. A hibernated listed computer resumes with the token on its disk (the
    epoch raise).
24. Claude Code does not honor `ANTHROPIC_BASE_URL` through the proxy, or
    tunnels plain HTTP: the CONNECT:80 path covers the second, and the
    staging proof is the verifier for both.
25. Anthropic answers OAuth calls from a relay differently from a direct
    client (headers, betas, user agent); the staging proof decides, and the
    relay passes `anthropic-beta`, `anthropic-version`, `user-agent` and
    `x-app` values that match a safe pattern.

### Rollback

Revert the commit and deploy, then cold-boot each listed computer (the
token returns to its disk) and lower nothing: the epoch never falls, so the
revert keeps 36. The gateway's copy goes with its stop.

### Proof on staging (deployed acceptance)

On the Claude arm (computer-a875023a6379c41f6ddb438f02e76a02), after a cold
boot on the new release:

1. The arm's credential disk lists no `claude-oauth-token` and no
   `claude-delegate.json` (names only, never contents).
2. A delegate run (`choir-delegate start`, then `wait`) ends `done` with a
   result, and the gateway journal shows `anthropic relay
   computer-a875023a… status=200` lines with token counts, credential kind
   `oauth`.
3. From a claude-mode capsule on the arm, a CONNECT to
   `api.anthropic.com:443` is refused (`host is not on the L2 ecosystem
   allowlist`), and the capsule's egress ledger shows the route
   `anthropic.choir.internal` with outcome `ok`.
4. A request to the route without the relay secret answers 401 and sends
   nothing to the gateway.

## Design review, round 1 (2026-11-03): APPROVE WITH CHANGES, twice

Panel: Devin and DeepSeek v4.1 flash (tier 2), on the design section and the
code it changes. Both approved with changes; no finding changed the
mechanism. Adjudication, with what the code does:

- Cap unset or bad (both): an unset, unreadable or non-positive
  `GATEWAY_ANTHROPIC_RELAY_DAILY_TOKENS` is the default cap (50 M tokens a
  UTC day), never no cap. A restart no longer resets the day: the charges
  are kept in `GATEWAY_ANTHROPIC_RELAY_SPEND_FILE` (atomic rename, per
  computer, ignored on another day). Check-then-charge can overshoot by the
  calls in flight (at most 4 per computer); recorded, not closed.
- Empty secret, no bearer (Devin B): a route with no secret, no bearer or
  no gateway address refuses every request; the secret is compared in
  constant time before any byte goes to the gateway.
- Keep-alive and the CONNECT:80 tunnel (Devin C): one request per
  connection. The route sends `Connection: close` to the gateway and splices
  only the answer back, so a second request on the same connection or
  tunnel is never read, let alone forwarded.
- Strip list (Devin D, DeepSeek N1): the upstream request is built fresh;
  only `Content-Type`, `Accept`, `Anthropic-Version`, `Anthropic-Beta`,
  `User-Agent` and `X-App` cross, each matched to a safe character set, then
  the computer's bearer and the capsule header (attribution only). The body
  is read whole up to 32 MiB (not streamed), which bounds it simply.
- The token copy (Devin E, DeepSeek B2): the gateway runs as root; the
  `+` stage copies with `cp -p` (root, 0400) into `RuntimeDirectory
  go-choir-gateway`, mode 0700, `RuntimeDirectoryPreserve=no`. A named
  token file that is missing, empty or malformed leaves the relay off; it
  never falls back to a key.
- Secret leaks (Devin F): the egress ledger records host, method, address
  and bytes only; refusals quote no header. The gateway never sees the
  secret (the route strips it).
- Path shape (Devin G): the client sends `/v1/...`; the route matches the
  three paths exactly (an encoded or dotted path is refused, `RawPath` must
  be empty) and maps them to `/provider/anthropic/v1/...`.
- The two computers not cold-booted (DeepSeek B3): accepted and named.
  b@c's and the 10-20 test computer's credential disks keep the token
  until their next cold boot (the epoch raise forces it at their next wake
  from hibernation). Only rotating the token makes those copies useless:
  the owner's `claude setup-token`, then the new token at the host path.
  Recorded as the owner's step after the rollout.
- `/proc/<pid>/environ` and cell separation (Devin, DeepSeek N3): the
  separation is the broker's, as before (F13, unverified on the staging
  kernel); a leaked secret now spends only through that capsule's proxy,
  bounded by its budgets and the computer's cap, and dies with the capsule.
- OAuth acceptance (DeepSeek N4, N6): the relay passes Claude Code's
  `user-agent`, `x-app` and betas and adds the OAuth beta; whether
  Anthropic accepts the relayed call is the staging proof's to decide. A
  revoked token answers 401 on every call while the grant still reads
  available; visible in the run's failure and the gateway journal.
- `/health` (DeepSeek nit): the credential kind is in the startup log and in
  each relay log line (`credential=oauth`), not in `/health`.
- count_tokens is not charged (it is not billed); an aborted stream is
  charged what it showed.

Found while building, not by the panel: vmctl's retention sweep protects
every computer a `CHOIR_*_COMPUTERS` selector names
(`internal/vmctl/retention_test_class.go`). Dropping
`CHOIR_CLAUDE_DELEGATE_COMPUTERS` from vmctl would have stopped protecting
the delegate computers. vmctl keeps the variable, fed by the same
let-binding, for that alone; nothing in vmctl reads it otherwise.

Review numbers: the design round is review 392 (Devin and DeepSeek v4.1
flash, both APPROVE WITH CHANGES); the code round below is review 393.

## Code review, round 2 (review 393, 2026-11-03): APPROVE WITH CHANGES, twice

No blocking-real finding from either seat. Fixed in the same commit:

- The grant read checks the pin whatever the answer, so a copied bearer
  learns nothing (both seats).
- The spawn reads the grant before taking the executor lock (Devin).
- The gateway's token read refuses a symlink, a file readable by group or
  others and one another user owns, as vmctl's did (Devin).
- A caller that goes away before the answer is charged its input as an
  estimate (four bytes a token), so dropping calls does not dodge the cap
  (DeepSeek).
- Forwarded success headers are redacted like error bodies (DeepSeek).
- `GATEWAY_ANTHROPIC_RELAY_MODELS` is set in nix to the delegate's model
  (DeepSeek); the grant names the served model.
- Tests: the comma-joined tool profile the nix produces (Devin's one
  blocking candidate; the selector parser splits commas, so the layer was
  attached, and the test now pins it); the capsule's reply and ledger carry
  neither the bearer nor the secret (DeepSeek).

Left as named residuals: `ClaudeDelegateStatus.ComputerID` is empty (no
consumer); the spend file is written under the relay's lock; a budget
refusal mid-route closes the connection without a 429; `Expect:
100-continue` is not answered (the staging proof decides whether Claude
Code sends it).
