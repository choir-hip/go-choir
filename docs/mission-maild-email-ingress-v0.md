# Mission: maild Email Ingress v0

Last updated: 2026-05-27

Reference: [choir-email-reference-v0.md](choir-email-reference-v0.md)

## Mission Status

```text
status: checkpoint_incomplete
current artifact state: maild/proxy/frontend behavior slice, deploy cleanup, trusted-mail evidence classification, trusted-upload auth-result evidence requirement, provider-response body cap for inbound fetches and outbound send responses, authenticated maild API request body cap, no persistent Resend raw-message download URLs, minimized source-packet provenance without internal owner/alias IDs, prompt-body data framing for Send to Choir, proxy trust-label enforcement, webhook-correlated acceptance tooling, canonical outbound From enforcement, owner-send Resend idempotency key, malformed Svix webhook-secret rejection, explicit no-auto-outbound acceptance tooling, retryable webhook-ingest acknowledgement repair, idempotent Send to Choir ingress receipts, visible/retryable Send to Choir receipt recording, public unsigned-webhook fail-closed readiness tooling, proxy identity-marker enforcement for mailbox/send routes, and deploy-impact selection are deployed on Node B at 27b15a5 through GitHub Actions; /opt/go-choir is a clean checkout and maild/proxy health report 27b15a5; Resend domain/webhook setup and DNS/MX remain unconfigured
what shipped: maild service, SQLite mailbox, webhook verifier, duplicate webhook ingest retry, retryable ingest failure 503 acknowledgement, provider-response body caps for received-email fetches and outbound send responses, authenticated API request body cap for owner-send and ingress-receipt JSON, receive-policy gates, quarantine metadata, source packets with minimized provenance, inbound raw-message refs that do not persist provider download URLs, Email app with Compose, row attachment indicators, collapsed raw-header/stored-recipient Details, proxy auth forwarding, proxy-owned Send to Choir with line-prefixed untrusted body framing, source-packet provenance/text refs, bounded normalized email-body handoff, idempotent ingress-event receipts, bounded proxy retry for ingress receipt callback, owner-visible receipt-pending Send to Choir status, read-only maildctl, bounded provider logging, reply threading headers, owner-send Resend `Idempotency-Key`, empty decoded Svix webhook-secret rejection, no-auto-outbound checker gate, readiness tooling with a public unsigned-webhook fail-closed probe, and CI deploy selection/restart support for host services
locally proven: fake signed Resend webhook -> fetch/normalize/store/quarantine/source packet; source-packet provenance retains provider/message/event/recipient/trust evidence while omitting internal `mailbox_owner_id` and `alias_id`; provider raw `download_url` is not persisted into `raw_message_ref`; oversized Resend received-email API response returns HTTP 503 ingest_retry_requested after recording the verified webhook event and before storing a message; oversized successful Resend send response returns HTTP 502 and stores no Sent row; oversized owner-send JSON request returns HTTP 413 and stores no Sent row; oversized ingress-receipt JSON request returns HTTP 413 and records no ingress event; duplicate email.received after transient provider failure retries and stores missing message idempotently; newly recorded email.received events return HTTP 503 ingest_retry_requested on retryable provider/store failure; duplicate retries that still fail return HTTP 503 duplicate_ingest_retry_requested; malformed `whsec_` webhook secret rejects without storing events; stale Svix timestamps reject; space-delimited Svix signatures accept when any v1 signature matches; duplicate owner Send to Choir ingress receipts for the same message/source/submission are idempotent and preserve a single durable event; proxy retries transient ingress receipt callback failure and reports `ingress_event_recorded=true` after recovery; proxy reports `ingress_event_recorded=false` without hiding the conductor submission id if the receipt remains unrecorded; trusted-upload-style alias rejects unwhitelisted sender, rejects a whitelisted sender when authentication-results evidence is absent, and accepts whitelisted sender when evidence is present; whitelisted trusted-upload messages store trusted sender status and authentication-results evidence; owner-only send; outbound send canonicalizes display-name input back to the numeric alias; owner-send includes a stable Resend `Idempotency-Key`; owned reply target -> In-Reply-To/References; proxy-owned Send to Choir now carries provenance, stable text refs, bounded normalized email body with every untrusted body line prefixed as `EMAIL-DATA`, and ingress receipt; proxy rejects unexpected source-packet trust labels before prompt-bar submission; acceptance checker now requires selected message provider ids and matching webhook receipt; message list attachment indicator; message-detail raw headers and stored recipient API/UI details surface; Compose posts plain owner-send payload through /api/email/send; frontend production build; NixOS maild/Caddy route eval; read-only provider readiness probe now proves the public webhook route fails closed without counter mutation; dry-run Resend setup helper; webhook secret handoff dry-run; dry-run Gandi DNS plan/rollback tooling; mail acceptance checker fake-ssh path
deployed proven: GitHub Actions push CI run 26544235598 passed for 27b15a5 and Deploy to Staging job 78192658862 succeeded; deploy impact selected `HOST_SERVICES=maild`, built the maild host-service package, updated `maild -> /var/lib/go-choir/services/maild`, restarted `go-choir-maild`, and left maild healthy; /opt/go-choir HEAD is 27b15a59fb959f72e93d6b98341df5973532a236 with clean status; public `/health` reports proxy and upstream build deployed_commit 27b15a59fb959f72e93d6b98341df5973532a236 and deployed_at 2026-05-27T23:14:08Z; Node B maild is active from /var/lib/go-choir/services/maild/bin/maild and maild health is ok with webhook_secret_configured=false and zero message/event counters; direct local deployed oversized owner-send request returns HTTP 413 `request body too large` with zero message/event counters before and after; direct local maild mailbox probes now return 401 with no owner header, 403 for spoofed X-Authenticated-User without X-Internal-Caller:true, and 200 only with the internal marker; public proxy email routes reject unauthenticated/spoofed browser requests with 401; scripts/mail-provider-readiness still proves the public webhook route returns HTTP 503 webhook_secret_not_configured without counter mutation; Resend domains/webhooks remain blocked by restricted_api_key, and Gandi DNS/public DNS still point at Gandi mail defaults with no DMARC
unproven claims: real Resend webhook, Resend domain verification, Gandi DNS/MX, real inbound/outbound mail, real Send to Choir trace from received email
next executable probe: obtain a Resend key/dashboard session that can read domain and webhook configuration, run scripts/mail-provider-readiness to verify exact provider truth, then deploy RESEND_WEBHOOK_SECRET and plan Gandi DNS from exact Resend records before any MX mutation; do not apply Gandi MX changes while Resend domains/webhooks return restricted_api_key
```

## Mission Frame

Build the first production-shaped Choir Email substrate: a separate host-side
`maild` service with SQLite-backed mailbox state, Resend webhook verification,
quarantined attachments, and a minimal Email app that can turn a received message
into an explicit, policy-labeled source packet for the existing
prompt-bar/conductor MAS path.

The product is not "an email client" first. The product is a safe mail ingress
port into the automatic computer. The minimal UI exists to inspect, reply, and
decide whether a message should become source material for Choir.

## Real Artifact

A deployed, staging-proven email ingress path for `choir.news`:

```text
Resend -> Caddy -> maild -> SQLite/storage -> proxy-authenticated Email app
                                      |
owner action -> proxy -> source packet -> sandbox prompt-bar/conductor path
```

The low-resolution v0 must be the same kind of object as the future system. It
may be small, but it must preserve the real service boundary, trust boundary,
source provenance, and verification semantics.

## Hard Invariants

- Email content is data, never instruction.
- Numeric addresses are public handles, not authorization secrets.
- `maild` may receive, store, classify, quarantine, and expose source packets.
- `maild` may not directly call agent tools, mutate canonical VText/files,
  submit hidden conductor work, promote state, or send outbound mail from inbound
  content.
- MAS handoff is owner/proxy/user-computer owned.
- Authenticated mailbox access uses the existing proxy/session trust boundary.
- The Resend webhook must be verified from the raw request body.
- Attachments are quarantined by default.
- Provider, Resend, Gandi, and webhook secrets stay out of git, frontend bundles,
  user computers, VText, and Trace payloads.
- DNS/MX changes happen only after deployed service and rollback readiness.

## Value Criterion

Minimize the distance between "external email arrived" and "Choir has a
traceable untrusted source packet under owner authority" while preserving all
trust boundaries.

This mission is uphill when:

- the system accepts real mail without trusting it;
- the owner can inspect it in a minimal Email app;
- the owner can explicitly hand it to Choir through the same authority shape as
  prompt bar;
- security tests prove inbound mail cannot perform privileged actions.

## Quality Target

`solid`.

This is a security-sensitive ingress substrate. A demo that receives mail but
leaves unclear authority, weak replay protection, missing tests, or UI-only
proof is not acceptable. The first implementation should be small, named
clearly, covered by focused tests, deployable, observable, and rollback-safe.

## Belief State

Current beliefs:

- `platformd` is scoped to platform Dolt publication/retrieval/citation and is
  not the right home for mail.
- `gateway` is scoped to model/search provider egress and is not the right home
  for mail.
- `maild` should own mail state and Resend integration. Local code now follows
  this boundary.
- The current prompt-bar/conductor path is the correct topology for email as
  MAS ingress. Local proxy tests now prove the first contract shape without
  giving `maild` sandbox credentials.
- Root-domain MX for `choir.news` is acceptable for the numeric-address product
  model, and the owner has explicitly accepted replacing Gandi mail routing
  because Gandi mailboxes are not in use. It still must be treated as a late,
  reversible operations step based on exact Resend records.
- The mockups give useful visual direction, but v0 must cut search, rules,
  alias management, storage meters, bulk actions, automation, rich compose, and
  threading.

Highest-impact uncertainty:

Whether the deployed Node B path preserves the same trust boundary under real
auth, Caddy, systemd, Resend webhook signatures, and Gandi MX routing.

Next observation that reduces uncertainty:

Staging route proof where `go-choir-maild` is active, Caddy sends only
`/api/email/resend/webhook` directly to it, authenticated Email app requests go
through proxy, and no DNS/MX mutation has happened yet.

## Homotopy Parameters

Increase realism in this order:

1. Fake Resend webhook payloads in local tests.
2. `maild` service skeleton, SQLite migrations, and source-packet model.
3. Authenticated proxy forwarding and proxy-owned MAS handoff contract.
4. Minimal desktop/mobile Email app using local/dev data.
5. Deployed service health, Caddy route, and secret injection.
6. Real Resend webhook verification without root MX mutation.
7. Gandi DNS/MX mutation with rollback records.
8. Real inbound email to `000@choir.news`.
9. Real explicit reply/send from `000@choir.news`.

Do not jump to a later realism level before the previous trust boundary is
proven.

## Receding-Horizon Execution

First control interval:

```text
implement:
  - cmd/maild skeleton
  - internal/maild config/store/migrations
  - health endpoint
  - fake webhook verification/idempotency tests
  - seeded 000 alias/policy migration

verify:
  - nix develop -c go test ./internal/maild ./cmd/maild
  - no Resend key required for fake tests
  - missing webhook secret fails closed
```

Second control interval:

```text
implement:
  - proxy config for PROXY_MAILD_URL
  - authenticated mailbox forwarding
  - proxy-owned /api/email/messages/:id/send-to-choir contract
  - source packet envelope into conductor-style submission

verify:
  - proxy tests strip spoofed user headers
  - maild cannot call sandbox directly
  - source packet is marked UNTRUSTED_EXTERNAL_EMAIL
```

Third control interval:

```text
implement:
  - minimal Email app
  - desktop three-pane layout
  - mobile list-first layout
  - Inbox, Sent, Quarantine only
  - message detail, trust badges, plain text body, attachments list
  - Reply and Send to Choir actions

verify:
  - frontend tests for list/detail/reply/send-to-choir state
  - visual check on desktop and mobile
  - no v0-cut features accidentally shipped
```

Fourth control interval:

```text
implement:
  - Node B service config
  - Caddy webhook route
  - separate /var/lib/go-choir/maild.env credential deployment
  - deployed health proof

verify:
  - CI green
  - deploy identity matches pushed SHA
  - staging health and route checks pass
  - no DNS/MX mutation yet
```

Fifth control interval:

```text
implement:
  - Resend domain/webhook setup
  - Gandi DNS/MX setup only after rollback evidence
  - real inbound and outbound acceptance

verify:
  - real email to 000@choir.news lands in Inbox
  - non-whitelisted attachment is quarantined
  - Send to Choir creates traceable conductor-style run
  - reply/send requires explicit owner action
```

After first correctness, run a quality pass for names, duplicate paths, logs,
tests, docs, and residual risks.

## Dense Feedback

Unit tests:

- alias parsing and plus-address parsing;
- seeded alias/policy migration;
- webhook raw-body verification;
- missing/invalid webhook secret;
- replay/idempotency;
- policy evaluation;
- ownership enforcement;
- attachment quarantine default;
- outbound send policy.

Integration tests:

- `maild` SQLite migrations and store round trips;
- fake Resend event to normalized message/source packet;
- proxy authenticated mailbox forwarding;
- proxy-owned send-to-Choir handoff;
- no direct `maild` sandbox/runtime dependency.

Frontend tests:

- Inbox/Sent/Quarantine navigation;
- message list and detail;
- trust badges;
- plain text rendering;
- attachment quarantine display;
- reply/send explicit action;
- Send to Choir action state.

Staging proof:

- pushed commit SHA;
- CI run and deploy status;
- staging health/build identity;
- `go-choir-maild` active on Node B;
- webhook route reachable;
- Resend webhook verified;
- Gandi DNS/MX before/after evidence;
- real inbound message id;
- real outbound sent message id, if outbound is in the landed slice;
- Trace/conductor evidence for manual MAS handoff;
- proof that inbound mail did not trigger send/tool/canonical mutation.

## Anti-Goodhart Constraints

- A row in SQLite is not success unless the event was signature-verified or
  explicitly marked fake-test.
- A visible Email app is not success unless a source packet can enter a
  traceable conductor-style run under owner authority.
- A passing webhook request is not security success unless replay, idempotency,
  raw-body verification failure, and missing-secret cases are tested.
- A successful outbound send is failure if any inbound message can cause it
  without explicit owner action.
- DNS/MX is not complete without rollback records and current Gandi zone
  evidence.
- Local tests are not proof for Resend, DNS, deployed auth, Node B secrets, or
  staging MAS behavior.

## Forbidden Shortcuts

- Do not use `/api/agent/*`, `/api/test/*`, `/internal/*`, or raw event mutation
  as acceptance proof.
- Do not authorize by numeric local part.
- Do not add direct maild-to-sandbox or maild-to-agent endpoints.
- Do not parse a JSON-mutated body for webhook verification.
- Do not log bodies, attachment URLs, webhook secrets, API keys, or Gandi PAT.
- Do not implement rich mail-client features before the ingress trust boundary is
  proven.
- Do not change DNS/MX before deployed route and rollback evidence are ready.

## V0 UI Scope

Use the provided mockups for visual direction only.

Desktop v0:

- Email app with mailbox rail, message list, message detail.
- Folders: Inbox, Sent, Quarantine.
- Active address: `000@choir.news`.
- Message rows: sender, subject, snippet, time, unread, attachment indicator,
  trust badge.
- Detail: From, To, Subject, Date, plain text body, collapsed headers,
  attachments with status, trust badge.
- Actions: Reply, Send to Choir, Mark read.

Mobile v0:

- List-first inbox.
- Header with active address selector.
- Message detail as separate screen/panel.
- Floating compose button acceptable.

Explicit cuts:

- No search.
- No rules UI.
- No alias management UI.
- No storage meter.
- No multi-select bulk toolbar.
- No archive/trash unless already trivial.
- No rich HTML compose.
- No automatic attachment scanning claim.
- No automation UI.
- No conversation threading.
- No newsletter/bulk sending.

## Rollback

- Disable or remove Resend webhook.
- Restore prior Gandi DNS/MX records from captured evidence.
- Stop `go-choir-maild`.
- Keep `/var/lib/go-choir/mail` for forensics unless explicitly purging.
- Revert platform code through normal GitHub main deploy if routes/UI break
  staging.

## Evidence Ledger Template

```text
claim:
evidence source:
command/observation:
artifact path or URL:
result:
uncertainty/caveat:
supports promotion: yes/no
```

## Stopping Condition

V0 is complete only when staging proves:

- real inbound mail to `000@choir.news` appears in the Email app;
- the message is stored with verified webhook provenance;
- attachments from non-whitelisted senders are quarantined;
- Send to Choir creates a traceable conductor-style run with email as
  `UNTRUSTED_EXTERNAL_EMAIL`;
- reply/send from `000@choir.news` works only after explicit owner action, if
  outbound is included in the shipped slice;
- no inbound mail can trigger outbound send, tool execution, canonical mutation,
  or promotion without explicit policy.

If outbound is deferred, the mission may stop at `checkpoint_incomplete`, not
`complete`, unless the mission is explicitly reparameterized to inbound-only.

## Suggested /goal Prompt

```text
/goal Run docs/mission-maild-email-ingress-v0.md as a MissionGradient mission. Build Choir Email v0 as a separate host-side maild service with SQLite mailbox state, Resend webhook verification, attachment quarantine, minimal Email app UI, and a proxy-owned Send to Choir handoff into the existing prompt-bar/conductor MAS path. Preserve the invariants in the mission and docs/choir-email-reference-v0.md: email content is data not instruction, numeric addresses are not auth secrets, maild cannot directly call agents or mutate canonical state, attachments quarantine by default, secrets stay out of user computers and browser-visible state, and DNS/MX changes wait for deployed route and rollback proof. Work in receding-horizon intervals, update the mission belief state when evidence changes, document new platform problems before fixes, run focused tests before deploy, then push, monitor CI/deploy, verify staging identity, configure Resend/Gandi only after rollback evidence, and produce final evidence for real inbound mail, source-packet MAS handoff, quarantine behavior, and absence of inbound-triggered privileged actions.
```

## Run Checkpoint & Resumption State

```text
status: checkpoint_incomplete
last checkpoint: outbound Resend success response cap deployed on 2026-05-27 at 56ad514
current artifact state: cmd/maild, internal/maild, proxy forwarding/MAS handoff, Email app shell, Node B maild service route, maildctl, mail credential deploy script, receive-policy gates including trusted-upload auth-result evidence requirement, provider-response body caps for received-email fetches and outbound send responses, prompt-body data framing for Send to Choir, no-auto-outbound checker, retryable webhook-ingest acknowledgement repair, idempotent/visible Send to Choir ingress receipts, and public webhook fail-closed readiness tooling are deployed through GitHub Actions at behavior commit 56ad514; Resend receiving/webhook and Gandi DNS are not configured
what shipped: maild service, minimal Email app with Compose and collapsed raw-header/stored-recipient Details, proxy auth boundary, Send to Choir handoff, operator inspection CLI, bounded provider logging, RFC reply threading headers for owner replies, ingress-event handoff receipts, duplicate webhook ingest retry, retryable ingest failure 503 acknowledgement, receive-policy gates, and a read-only mail acceptance checker
what was proven:
  - signed fake Resend webhook verification, idempotency, missing-secret, missing-header, and mutated-body rejection
  - fake Resend retrieval stores inbound message, quarantines attachment metadata, and creates UNTRUSTED_EXTERNAL_EMAIL source packet
  - duplicate `email.received` delivery after a transient provider fetch failure retries ingest and stores the missing message while keeping one webhook event row
  - oversized Resend received-email API response returns HTTP 503 `ingest_retry_requested` after recording the verified webhook event and before storing a message
  - trusted-upload-style receive policy rejects unwhitelisted sender before storing the message and accepts a whitelisted sender on an unlisted exact plus alias
  - sender-whitelist-gated trusted-upload policy rejects a whitelisted sender when Authentication-Results/ARC evidence is absent before storing the message
  - owner-only outbound send through fake Resend stores Sent row
  - oversized successful Resend send response returns HTTP 502 and stores no Sent row
  - owned reply targets preserve provider message_id and emit In-Reply-To/References headers in the Resend send payload
  - proxy forwards authenticated /api/email/* to maild while stripping spoofed identity and Cookie headers
  - proxy-owned Send to Choir fetches a source packet and submits guarded prompt-bar text to the resolved user computer
  - Send to Choir line-prefixes every normalized email body line as `EMAIL-DATA` so injection-like body lines remain quoted source data in the prompt-bar payload
  - frontend production build succeeds
  - local Playwright visual harness renders Email app on desktop and mobile-sized windows with fixture mail and no `undefined` text
  - NixOS eval exposes go-choir-maild and Caddy webhook route before generic /api/*
  - Node B deployed commit identity and service health reported d749fdcfb329226f73ce4717b86f1ac0eba5e1a0 at the reply-threading checkpoint
  - Node B deployed commit identity and service health report 73beb6e127199ea77c035e3729a76f23c8d03a16 at the ingress-event evidence checkpoint
  - Node B deployed commit identity and service health report 1e3d54ae3ebcdfb00646fca6a4645ee18d2ccac2 at the raw-header details checkpoint
  - Node B deployed commit identity and service health report 843ec907117e26ef741b7b1a62d58f689839dd79 at the stored-recipient details checkpoint
  - Node B deployed commit identity and service health report 5378215c341813dcec8d985c105c57c9f6181e3b at the minimal compose checkpoint
  - read-only provider readiness probe reports Resend/Gandi/Node B state without mutating DNS or printing secrets
  - Resend setup helper read-only path reports restricted_api_key with current send-only key, and fake-curl success path writes webhook secret to mode 600 without printing it
  - mail credential deploy script can consume the generated webhook-secret file in redacted dry-run mode and still fails closed when the secret is absent
  - Gandi DNS plan and rollback helpers dry-run from Resend domain JSON without mutating records
  - maildctl and maild expose owner/message-scoped ingress-event inspection;
    proxy-owned Send to Choir records a local ingress event after prompt-bar
    submission without giving maild MAS credentials
  - scripts/mail-acceptance-check verifies real message, quarantine, source
    packet trust, and expected ingress-event count through read-only maildctl
  - message detail API exposes stored raw headers to the authenticated owner,
    and the Email app renders them behind a collapsed Details section
  - message detail API exposes stored To/Cc/Bcc rows to the authenticated
    owner, and the Email app renders them instead of assuming the active alias
  - Compose posts a plain owner-send payload through `/api/email/send` with
    From fixed to `000@choir.news`
  - message list API computes owner-visible attachment existence without loading
    attachment content, and the Email app renders a compact row indicator
  - receive-policy gates are deployed; public, sender-whitelist, secret-alias,
    and attachment gates run before inbound message storage
  - retryable first-delivery webhook ingest failures now return HTTP 503
    `ingest_retry_requested`; duplicate retry failures return HTTP 503
    `duplicate_ingest_retry_requested`; duplicate retry success remains
    idempotent
unproven or partial claims:
  - real Resend webhook and API payload compatibility
  - Gandi MX/SPF/DKIM/DMARC setup and rollback
  - real inbox appearance, real reply/send, and real Send to Choir trace
  - GitHub Actions emitted runs for 73beb6e again, but CI failed before project code gates; rerun evidence includes GitHub checkout 403 "Your account is suspended" and action archive download failures
  - manual Node B source-truth deploys advanced staging through 5378215, but those deploys violate the mission landing-loop invariant and must not be counted as acceptance proof
  - GitHub Actions rerun 26448967008 recovered the landing loop: Go Vet + Build, Go Test non-runtime, runtime shards 0-3, integration smoke, frontend build, aggregate Go gate, and Deploy to Staging all passed
  - public health after the recovered Actions deploy reports proxy and sandbox deployed_commit 33de426201825ba42215e929b9366c2b351b85ab, a docs-only head on top of the behavior slice
  - scripts/mail-provider-readiness now reports Resend domains/webhooks not ready because the configured Resend key is restricted to sending only, Gandi MX/SPF still point at Gandi defaults, no DMARC exists, and Node B maild health is ok with webhook_secret_configured false
  - GitHub Actions run 26450002582 deployed ae8cb7f after passing all Go/frontend gates; public health reports proxy/sandbox deployed_commit ae8cb7f7f80a3944998549991227cd559832d150
  - GitHub Actions run 26451498114 deployed 7de363e after passing all Go gates; public health reports proxy/sandbox deployed_commit 7de363e05cfb102fcfec44303955b3c525870711; Resend remains restricted_api_key and webhook_secret_configured false
  - GitHub Actions run 26534436593 deployed a22d075 after passing all Go gates; Deploy to Staging job 78159684073 succeeded; public health reports proxy/sandbox deployed_commit a22d075c54234b4bb36f04d5015ed0616d9bb954; Node B /opt/go-choir is clean at a22d075; Resend remains restricted_api_key and webhook_secret_configured false
  - GitHub Actions run 26537198690 deployed 5a6d3cf after passing all Go/frontend gates; Deploy to Staging job 78169289186 succeeded; public health reports proxy/sandbox deployed_commit 5a6d3cfc8db7e30a714b20736115c3f616a44cef; Node B /opt/go-choir was clean at 5a6d3cf; maild health was ok with webhook_secret_configured false and zero message/event counters
  - GitHub Actions run 26537590271 deployed b62ebe3 after passing all Go/frontend gates; Deploy to Staging job 78170697458 succeeded; Node B deploy.env, /opt/go-choir HEAD, proxy health, and sandbox upstream build all report b62ebe3ad29abf480247d3700c72dbd7944fe063; Node B maild health is ok with aliases 1/messages 0/quarantined_attachments 0/webhook_events 0/ingress_events 0; scripts/mail-provider-readiness proves the public webhook route fails closed with HTTP 503 webhook_secret_not_configured and no counter mutation
  - GitHub Actions run 26541562656 deployed dc40d0a after passing all Go gates; Deploy to Staging job 78184166989 succeeded; deploy impact selected HOST_SERVICES=maild, built .#maild, updated the host service pointer, and restarted go-choir-maild; Node B deploy.env and /opt/go-choir HEAD match dc40d0a3fd43294ad40e5194e99c3f398a74a77c with clean status; maild health is ok with webhook_secret_configured false and zero message/event counters; scripts/mail-provider-readiness still reports Resend restricted_api_key, Gandi mail records/public DNS on Gandi defaults, no DMARC, and public webhook HTTP 503 webhook_secret_not_configured with no counter mutation
  - GitHub Actions run 26542167430 deployed 15ddeb4 after passing all Go gates; Deploy to Staging job 78186101002 succeeded; deploy impact selected HOST_SERVICES=maild, built .#maild, updated the host service pointer, and restarted go-choir-maild; Node B deploy.env and /opt/go-choir HEAD match 15ddeb4fe02213a58c3d1ca63b293657ddf5c4ce with clean status; maild health is ok with webhook_secret_configured false and zero message/event counters; scripts/mail-provider-readiness still reports Resend restricted_api_key, Gandi mail records/public DNS on Gandi defaults, no DMARC, and public webhook HTTP 503 webhook_secret_not_configured with no counter mutation
  - GitHub Actions run 26542524019 deployed ad1af55 after passing all Go gates; Deploy to Staging job 78187232221 succeeded; deploy impact selected HOST_SERVICES=proxy and updated the proxy host service pointer; Node B deploy.env and /opt/go-choir HEAD match ad1af5587550576ca82b5d72782d9efe2bbd8cd3 with clean status; public /health reports proxy and upstream build deployed_commit ad1af5587550576ca82b5d72782d9efe2bbd8cd3; Node B proxy and maild health are ok; scripts/mail-provider-readiness still reports Resend restricted_api_key, Gandi mail records/public DNS on Gandi defaults, no DMARC, and public webhook HTTP 503 webhook_secret_not_configured with no counter mutation
  - GitHub Actions run 26543202795 deployed 56ad514 after passing all Go gates; Deploy to Staging job 78189403049 succeeded; deploy impact selected HOST_SERVICES=maild, updated the maild host service pointer, and restarted go-choir-maild; Node B deploy.env and /opt/go-choir HEAD match 56ad514435dc10d27eef4fac7c2db08199946045 with clean status; public /health reports proxy and upstream build deployed_commit 56ad514435dc10d27eef4fac7c2db08199946045; Node B maild health is ok; scripts/mail-provider-readiness still reports Resend restricted_api_key, Gandi mail records/public DNS on Gandi defaults, no DMARC, and public webhook HTTP 503 webhook_secret_not_configured with no counter mutation
belief-state changes:
  - maild as separate microservice remains the right boundary
  - Resend credentials belong in /var/lib/go-choir/maild.env, not gateway-provider.env or platformd
  - 000@choir.news must be seeded to the real auth user id through MAILD_ROOT_OWNER_ID; Nix must not bake in a placeholder owner
  - plus aliases should not implicitly fall back to 000 because that weakens secret-alias policy
  - outbound reaches Resend, but the current Resend account state rejects 000@choir.news because choir.news is not verified
  - outbound send success responses now use the same provider response cap as received-email fetches
  - the GitHub Actions 403/setup failure appears recovered after rerun; local GitHub API checks also showed the user and org membership active
remaining error field:
  - real provider/DNS proof is still untouched because exact Resend verification/receiving records and webhook secret are missing
  - current Resend key cannot inspect domains or webhooks because it is restricted to sending only
  - attachment content download/extraction remains intentionally deferred
highest-impact remaining uncertainty: exact Resend domain/receiving/webhook configuration needed to prove real inbound and outbound
next executable probe: obtain a sufficiently scoped Resend key or dashboard session, run scripts/mail-provider-readiness until Resend domain/webhook records are visible, install RESEND_WEBHOOK_SECRET through /var/lib/go-choir/maild.env, update Gandi DNS from exact provider records, and run the real inbound/quarantine/source-packet/outbound acceptance
suggested resume goal string: continue docs/mission-maild-email-ingress-v0.md from deployed outbound provider-response cap checkpoint 56ad514; obtain Resend domain/webhook provider truth, use scripts/mail-provider-readiness before DNS mutation, configure Gandi from exact records, then prove real inbound mail, quarantine, source-packet MAS handoff, and owner reply
evidence artifact refs:
  - nix develop -c go test ./internal/maild ./cmd/maild ./internal/proxy
  - nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
  - npm run build in frontend
  - local Playwright screenshots: /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/choir-email-visual/email-desktop.png and /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/choir-email-visual/email-mobile-narrow.png
  - nix eval .#packages.x86_64-linux.maild.pname
  - nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-maild.description
  - nix eval .#nixosConfigurations.go-choir-b.config.services.caddy.virtualHosts."choir.news".extraConfig
  - public /health deployed_commit d749fdcfb329226f73ce4717b86f1ac0eba5e1a0
  - maild /health status ok with resend_api_key_configured true and webhook_secret_configured false
  - scripts/mail-provider-readiness
  - scripts/mail-resend-setup --write-domain-json /tmp/choir-resend-domain-test.json --write-webhook-secret-file /tmp/choir-resend-webhook-secret-test.env
  - fake-curl success test for scripts/mail-resend-setup --apply --ensure-domain --ensure-webhook
  - CHOIR_MAIL_WEBHOOK_SECRET_FILE=<tmp>/secret.env nix/deploy-mail-creds.sh --dry-run test-host
  - scripts/mail-gandi-plan-records --records <sample-resend-domain-json> --ttl 3600
  - scripts/mail-gandi-rollback-records --snapshot <sample-gandi-snapshot-json> --records <sample-resend-domain-json>
  - PATH=<fake-ssh> scripts/mail-acceptance-check --owner owner-000 --subject Acceptance --expect-attachment-quarantine --expect-ingress-events 0
  - GitHub Actions CI run 26447497910 for 73beb6e failed during setup/action download before project gates; rerun shows checkout 403 "Your account is suspended"
  - manual Node B deploy from /opt/go-choir at 73beb6e127199ea77c035e3729a76f23c8d03a16
  - public /health deployed_commit 73beb6e127199ea77c035e3729a76f23c8d03a16
  - maild /health status ok with stats.ingress_events 0, resend_api_key_configured true, and webhook_secret_configured false
  - maildctl ingress-events --owner 5bd6de97-3b58-408c-bf89-c42c81b083de --limit 5 -> []
  - nix develop -c go test ./internal/maild ./internal/proxy ./cmd/maildctl ./cmd/maild
  - npm run build in frontend
  - local Playwright component render: /tmp/choir-email-headers-details.png; closed Details hides raw message_id, opened Details shows message_id and authentication-results
  - GitHub Actions CI run 26448039030 for 1e3d54a failed before project gates; staging deploy job skipped
  - manual Node B deploy from /opt/go-choir at 1e3d54ae3ebcdfb00646fca6a4645ee18d2ccac2
  - public /health deployed_commit 1e3d54ae3ebcdfb00646fca6a4645ee18d2ccac2
  - maild /health status ok with stats.ingress_events 0, resend_api_key_configured true, and webhook_secret_configured false
  - local Playwright component render: /tmp/choir-email-recipient-details.png; header and Details show stored To, Cc, and Bcc recipients
  - GitHub Actions CI run 26448505213 for 843ec90 failed before project gates; staging deploy job skipped
  - manual Node B deploy from /opt/go-choir at 843ec907117e26ef741b7b1a62d58f689839dd79
  - public /health deployed_commit 843ec907117e26ef741b7b1a62d58f689839dd79
  - maild /health status ok with stats.ingress_events 0, resend_api_key_configured true, and webhook_secret_configured false
  - GitHub Actions CI run 26448967008 for 5378215 partially recovered: frontend build, non-runtime tests, integration smoke, and runtime shards 0/3 passed; Go Vet + Build and runtime shards 1/2 failed during setup; staging deploy skipped
  - manual Node B deploy from /opt/go-choir at 5378215c341813dcec8d985c105c57c9f6181e3b
  - public /health deployed_commit 5378215c341813dcec8d985c105c57c9f6181e3b
  - maild /health status ok with stats.ingress_events 0, resend_api_key_configured true, and webhook_secret_configured false
  - docs-only checkpoint commit 33de426201825ba42215e929b9366c2b351b85ab records that manual deploy is invalid acceptance evidence
  - GitHub Actions rerun 26448967008 succeeded; deploy job 77866644491 completed successfully
  - Deploy job log: checkout fetched 5378215, remote Node B pull fast-forwarded origin/main from 5378215 to 33de426, frontend bundle built, Caddy reloaded, ports 8081/8082/8083/8084/8086/8087 health passed
  - public /health on choir.news reports proxy and sandbox deployed_commit 33de426201825ba42215e929b9366c2b351b85ab, deployed_at 2026-05-26T13:05:20Z
  - maild health in deploy log reports status ok, primary_domain choir.news, resend_api_key_configured true, webhook_secret_configured false, root_owner_id_configured true, stats aliases 1/messages 0/quarantined_attachments 0/webhook_events 0/ingress_events 0
  - scripts/mail-provider-readiness reports Resend domains/webhooks 401 restricted_api_key, Gandi MX still spool/fb.mail.gandi.net, SPF still Gandi, no DMARC, and Node B maild health ok
  - nix develop -c go test ./internal/maild ./cmd/maild
  - npm run build in frontend
  - commit ae8cb7f7f80a3944998549991227cd559832d150
  - GitHub Actions CI run 26450002582 passed Go Vet + Build, Go Test non-runtime, runtime shards 0-3, integration smoke, Build Frontend, aggregate Go gate, and Deploy to Staging
  - FlakeHub publish run 26450002782 succeeded
  - public /health on choir.news reports proxy and sandbox deployed_commit ae8cb7f7f80a3944998549991227cd559832d150, deployed_at 2026-05-26T13:12:27Z
  - scripts/mail-provider-readiness after deploy still reports Resend domains/webhooks 401 restricted_api_key, Gandi MX still Gandi defaults, no DMARC, and Node B maild health ok with webhook_secret_configured false
  - GitHub Actions CI run 26537198690 for 5a6d3cf passed all Go/frontend gates; Deploy to Staging job 78169289186 succeeded; public health reported proxy/sandbox deployed_commit 5a6d3cfc8db7e30a714b20736115c3f616a44cef; Node B /opt/go-choir was clean at 5a6d3cf; maild health reported webhook_secret_configured false and zero message/event counters
  - GitHub Actions CI run 26537590271 for b62ebe3 passed all Go/frontend gates; Deploy to Staging job 78170697458 succeeded; Node B deploy.env, /opt/go-choir HEAD, proxy health, and sandbox upstream build reported b62ebe3ad29abf480247d3700c72dbd7944fe063; maild health reported aliases 1/messages 0/quarantined_attachments 0/webhook_events 0/ingress_events 0; scripts/mail-provider-readiness reported Resend domains/webhooks 401 restricted_api_key, Gandi mail records still at Gandi defaults, public DNS still Gandi MX/SPF with no DMARC, and public unsigned webhook probe HTTP 503 webhook_secret_not_configured with no counter mutation
  - GitHub Actions CI run 26541562656 for dc40d0a passed all Go gates; Deploy to Staging job 78184166989 succeeded; deploy log selected HOST_SERVICES=maild, built /nix/store/p1vvm3hfplaw6p187wmw5v3dn8vmadb5-maild-0.1.0.drv, updated maild -> /var/lib/go-choir/services/maild, restarted go-choir-maild.service, and reported maild health ok
  - Node B deploy.env and /opt/go-choir HEAD report dc40d0a3fd43294ad40e5194e99c3f398a74a77c with clean status; go-choir-maild is active/running with MainPID 2081452, ExecMainStartTimestamp Wed 2026-05-27 22:08:14 UTC, and executable /var/lib/go-choir/services/maild/bin/maild
  - scripts/mail-provider-readiness after dc40d0a reports Resend domains/webhooks 401 restricted_api_key, Gandi LiveDNS and public DNS still on Gandi MX/SPF/DKIM defaults with no DMARC, Node B maild health ok with webhook_secret_configured false and zero counters, and public unsigned webhook probe HTTP 503 webhook_secret_not_configured with no counter mutation
  - GitHub Actions CI run 26542167430 for 15ddeb4 passed all Go gates; Deploy to Staging job 78186101002 succeeded; deploy log selected HOST_SERVICES=maild, built /nix/store/hzs428g7g3dbjpz3n7b8mvsha3zblrrq-maild-0.1.0.drv, updated maild -> /var/lib/go-choir/services/maild, restarted go-choir-maild.service, and reported maild health ok
  - Node B deploy.env and /opt/go-choir HEAD report 15ddeb4fe02213a58c3d1ca63b293657ddf5c4ce with clean status; go-choir-maild is active/running with MainPID 2084460, ExecMainStartTimestamp Wed 2026-05-27 22:22:22 UTC, and executable /var/lib/go-choir/services/maild/bin/maild
  - scripts/mail-provider-readiness after 15ddeb4 reports Resend domains/webhooks 401 restricted_api_key, Gandi LiveDNS and public DNS still on Gandi MX/SPF/DKIM defaults with no DMARC, Node B maild health ok with webhook_secret_configured false and zero counters, and public unsigned webhook probe HTTP 503 webhook_secret_not_configured with no counter mutation
  - GitHub Actions CI run 26542524019 for ad1af55 passed all Go gates; Deploy to Staging job 78187232221 succeeded; deploy log selected HOST_SERVICES=proxy, updated proxy -> /var/lib/go-choir/services/proxy, and reported maild health ok
  - Node B deploy.env and /opt/go-choir HEAD report ad1af5587550576ca82b5d72782d9efe2bbd8cd3 with clean status; go-choir-proxy is active/running with MainPID 2087718, ExecMainStartTimestamp Wed 2026-05-27 22:30:43 UTC, and executable /var/lib/go-choir/services/proxy/bin/proxy; public /health reports proxy and sandbox deployed_commit ad1af5587550576ca82b5d72782d9efe2bbd8cd3
  - scripts/mail-provider-readiness after ad1af55 reports Resend domains/webhooks 401 restricted_api_key, Gandi LiveDNS and public DNS still on Gandi MX/SPF/DKIM defaults with no DMARC, Node B maild health ok with webhook_secret_configured false and zero counters, and public unsigned webhook probe HTTP 503 webhook_secret_not_configured with no counter mutation
  - nix develop -c go test ./internal/maild -run 'Test(HandleSendRejectsOversizedProviderSuccessResponse|ResendSendEmailReturnsBoundedProviderError|HandleSendRequiresOwnedFromAliasAndStoresSentMessage)'
  - nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
  - GitHub Actions CI run 26543202795 for 56ad514 passed all Go gates; Deploy to Staging job 78189403049 succeeded; deploy log selected HOST_SERVICES=maild, updated maild -> /var/lib/go-choir/services/maild, restarted go-choir-maild.service, and reported maild health ok
  - Node B deploy.env and /opt/go-choir HEAD report 56ad514435dc10d27eef4fac7c2db08199946045 with clean status; go-choir-maild is active/running with MainPID 2090770, ExecMainStartTimestamp Wed 2026-05-27 22:47:28 UTC, and executable /var/lib/go-choir/services/maild/bin/maild
  - scripts/mail-provider-readiness after 56ad514 reports Resend domains/webhooks 401 restricted_api_key, Gandi LiveDNS and public DNS still on Gandi MX/SPF/DKIM defaults with no DMARC, Node B maild health ok with webhook_secret_configured false and zero counters, and public unsigned webhook probe HTTP 503 webhook_secret_not_configured with no counter mutation
rollback refs:
  - do not add MX until exact Resend records and webhook secret are available
  - current Gandi MX/SPF remains Gandi mail defaults until provider records are verified
  - scripts/mail-gandi-plan-records snapshots Gandi records before apply
  - scripts/mail-gandi-rollback-records restores/deletes affected RRsets from that snapshot
  - stop go-choir-maild and remove Caddy webhook route if staging regresses
  - preserve /var/lib/go-choir/mail for forensics unless explicitly purging
```

## Deploy Blocker: maild Vendor Hash

Recorded: 2026-05-26.

Problem:

The first pushed implementation commit `59600385939fc0969c253e67491a055539a70c6f`
passed CI build/test gates, but the Node B staging deploy failed while building
the NixOS host closure. The new `maild` package used the generic Go service
vendor hash, but the Linux Nix build computed a different fixed-output hash.

Evidence:

```text
GitHub Actions run: 26446912518
failed job: Deploy to Staging (Node B), job id 77855699799
failed derivation: maild-0.1.0-go-modules.drv
specified hash: sha256-7sTVRCu7SWElqse4g82ERcaJAeWd9EAKmgAdmRa7Ezw=
actual hash:    sha256-dqBHF0LSI8L52jtgRZct1h8pw2C/boJsqBwsM1Z9ayE=
```

Belief-state update:

The service code passed normal Go tests, and the deploy failure is a packaging
hash mismatch isolated to the new `maild` derivation. The smallest safe fix is
to set `maild`'s package-specific `vendorHash` to the hash reported by the Node
B Linux Nix builder, then rerun the normal push/deploy loop.

## CI Finding: GitHub account checkout failure

Recorded: 2026-05-26.

Problem:

The CI rerun for behavior commit `73beb6e127199ea77c035e3729a76f23c8d03a16`
still fails before any project code is checked out or tested. Some jobs cannot
download GitHub Action archives, and the checkout job now receives HTTP 403
from GitHub with an account suspension message.

Evidence:

```text
CI run: 26447497910
rerun timestamp: 2026-05-26T12:21:13Z
Go Test / Go Vet setup: failed to download actions/setup-go@v6 archive
Detect Staging Deploy Impact checkout:
  git fetch -> HTTP 403
  remote: Your account is suspended. Please visit https://support.github.com for more information.
result: project tests, frontend build, and deploy jobs did not run in CI
```

Belief-state update:

- This checkpoint cannot use GitHub Actions as acceptance evidence until the
  GitHub account/repository access issue is resolved.
- Manual Node B deploy identity and local/dev-shell test evidence can help
  diagnose runtime behavior, but they are not acceptance proof for this mission.
  The landing-loop invariant requires GitHub Actions CI, Actions staging deploy,
  deployed identity, and product-path acceptance.

## Protocol Finding: manual Node B deploy invalidates acceptance evidence

Recorded: 2026-05-26.

Problem:

Several maild behavior checkpoints were advanced on Node B with manual
source-truth deploys after GitHub Actions failed to complete or failed to emit
runs. This violated the mission and repository landing-loop invariant:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Evidence:

```text
CI run 26448039030 for 1e3d54a: failed before project gates; deploy skipped
manual Node B deploy then reported public /health deployed_commit 1e3d54a

CI run 26448505213 for 843ec90: failed before project gates; deploy skipped
manual Node B deploy then reported public /health deployed_commit 843ec90

CI run 26448967008 for 5378215: frontend build, non-runtime tests,
integration smoke, and runtime shards 0/3 passed; Go Vet + Build and runtime
shards 1/2 failed during setup while downloading actions/setup-go@v6; deploy
skipped; manual Node B deploy then reported public /health deployed_commit
5378215c341813dcec8d985c105c57c9f6181e3b
```

Belief-state update:

- Current Node B behavior can be inspected for diagnostics, but it is tainted as
  mission acceptance evidence.
- The next safe probe is not Resend/Gandi setup. It is to rerun or repair the
  GitHub Actions path until CI gates and the staging deploy job succeed for the
  current behavior slice.
- DNS/MX and real inbound mail must wait until staging identity has been proven
  from the Actions deploy path.

## Staging Security Finding: mail State File Modes

Recorded: 2026-05-26.

Problem:

The first successful Node B deploy of `maild` created live mailbox state with
permissions that are too broad for email content. The service was active and
the Caddy route reached `maild`, but `/var/lib/go-choir/mail` was `0755` and
`/var/lib/go-choir/mail/mail.db` was `0644`.

Evidence:

```text
staging commit: 9192f378aa547ad93088b547b6213c285dc8fa67
GitHub Actions run: 26447116904
Node B services: go-choir-maild, go-choir-proxy, and caddy active
route proof: GET https://choir.news/api/email/resend/webhook -> 405 method not allowed
mode proof:
  755 root root /var/lib/go-choir/mail
  644 root root /var/lib/go-choir/mail/mail.db
  750 root root /var/lib/go-choir/mail/raw
  750 root root /var/lib/go-choir/mail/attachments
  750 root root /var/lib/go-choir/mail/attachments/quarantine
```

Belief-state update:

- `maild` as a separate service remains correct, but its host state must be
  treated as private mailbox data from the first deploy.
- Nix tmpfiles create rules alone are not enough evidence that existing paths
  and service-created SQLite files have private modes.

Required next change:

- Add explicit mode repair for the mail state paths and set a restrictive
  `go-choir-maild` umask so future mailbox files are not created world-readable.

Resolution checkpoint:

- Resolved in `nix/node-b.nix`: `go-choir-maild` now runs with `UMask =
  "0077"`, `StateDirectory = "go-choir/mail"`, and tmpfiles repair rules for
  `/var/lib/go-choir/mail`, `mail.db`, `raw`, `attachments`, and
  `attachments/quarantine`.
- Later deployed evidence in this mission records `/var/lib/go-choir/mail` as
  `0700`, `mail.db` as `0600`, and `/var/lib/go-choir/maild.env` as `0600`.

## Staging Security Finding: maild mailbox route identity boundary

Recorded: 2026-05-27.

Problem:

`maild` mailbox and send routes trust the `X-Authenticated-User` header injected
by the proxy, but the service does not require an internal-caller marker on
those routes. Node B binds `maild` to `127.0.0.1:8087` and Caddy only routes the
public Resend webhook directly to `maild`, so this is not browser-public.
However, any host-local process that can reach `maild` can spoof mailbox owner
identity by sending `X-Authenticated-User` directly. That weakens the mission
invariant that authenticated mailbox access uses the existing proxy/session
trust boundary.

Evidence:

```text
code: internal/maild/api.go HandleMessages only checks X-Authenticated-User
code: internal/maild/send.go HandleSend only checks X-Authenticated-User
code: internal/proxy/email.go forwardMaildAuthenticated strips client identity
      headers but does not mark the forwarded request as an internal caller
staging: go-choir-maild listens on 127.0.0.1:8087; Caddy routes only
      /api/email/resend/webhook directly to maild
```

Belief-state update:

- The service boundary is still right: public traffic reaches mailbox APIs only
  through proxy auth.
- The host-local boundary should still be tightened so `maild` refuses mailbox
  and send routes unless proxy has both validated the session and injected an
  internal-caller marker.

Required next change:

- Require `X-Internal-Caller: true` on authenticated `maild` mailbox and send
  routes.
- Have proxy inject that marker after stripping any client-supplied identity
  headers.
- Keep the Resend webhook route independent of this marker, because Resend
  cannot send proxy-authenticated internal headers.

Resolution checkpoint:

- Resolved locally in `internal/maild/api.go` and `internal/maild/send.go`:
  authenticated mailbox/send routes now require both `X-Authenticated-User` and
  `X-Internal-Caller: true`.
- `internal/proxy/email.go` now injects `X-Internal-Caller: true` only after
  validating the browser session and stripping client-supplied identity headers.
  The Send to Choir source-packet fetch uses the same internal marker.
- Focused coverage proves direct spoofed maild mailbox/send calls are rejected,
  proxy-injected calls still work, and client-supplied `X-Internal-Caller` is
  replaced by the proxy marker.
- Local verifier: `nix develop -c go test ./internal/maild ./cmd/maild
  ./cmd/maildctl ./internal/proxy`.

## Staging Config Finding: default alias owner reconciliation

Recorded: 2026-05-26.

Problem:

`maild` was first started before `/var/lib/go-choir/maild.env` existed. That
means the initial schema seed created `000@choir.news` with the fallback
`MAILD_ROOT_OWNER_ID` value `root`. The current seed path uses
`INSERT OR IGNORE` for the default alias, so deploying the real
`MAILD_ROOT_OWNER_ID` later would not update the already-created row.

Evidence:

```text
staging commit with first active maild: 9192f378aa547ad93088b547b6213c285dc8fa67
maild env file at first start: missing
maild default RootOwnerID without env: root
seed behavior: INSERT OR IGNORE INTO email_aliases for alias-choir-news-000
auth users include real owner candidate: 5bd6de97-3b58-408c-bf89-c42c81b083de / yusefnathanson@me.com
```

Belief-state update:

- Configuration can legitimately arrive after first service boot; bootstrap
  must reconcile platform-owned seed rows that are intended to track config.
- Numeric addresses are still not auth secrets, but an incorrect seed owner
  would make the real founder mailbox invisible and could route mail to a
  non-principal.

Required next change:

- Make the default `000@choir.news` seed upsert `target_id` from the configured
  `MAILD_ROOT_OWNER_ID` while leaving non-default aliases untouched.

Resolution checkpoint:

- Resolved in `internal/maild/store.go`: the default alias is inserted if
  missing and then explicitly reconciled by `DefaultRootAliasID` to the
  configured domain, `MAILD_ROOT_OWNER_ID`, public visibility, and public
  receive policy.
- Focused store coverage exists for reseeding the default alias with a changed
  root owner id, and later deployed evidence records `000@choir.news` mapped to
  the inferred founder auth user id.

## Deployed Checkpoint: maild ready, Resend receiving blocked on credentials

Recorded: 2026-05-26.

Status:

`maild` v0 is deployed on Node B and the host-side mailbox state is now private.
The service is configured with the send-only Resend API key and
`000@choir.news` resolves to the inferred founder auth user id. Real receiving
is not yet enabled because the available Resend key is restricted to sending and
there is no Resend webhook signing secret yet.

Evidence:

```text
latest deployed commit: 76c5d18a7fb705c5befcf35abadd2df3b9132fbb
deploy method: manual source-truth deploy from origin/main after GitHub Actions did not emit runs for post-9192f37 pushes
focused local tests: nix develop -c go test ./internal/maild ./cmd/maild ./internal/proxy
staging /health: proxy and sandbox report deployed_commit 76c5d18a7fb705c5befcf35abadd2df3b9132fbb
maild route proof: GET https://choir.news/api/email/resend/webhook -> 405 method not allowed
webhook guard proof: unsigned POST -> 503 webhook_secret_not_configured
mail state modes:
  600 root root /var/lib/go-choir/maild.env
  700 root root /var/lib/go-choir/mail
  600 root root /var/lib/go-choir/mail/mail.db
alias proof:
  choir.news|000|user|5bd6de97-3b58-408c-bf89-c42c81b083de
Gandi LiveDNS status: choir.news is on LiveDNS
current root MX: 10 spool.mail.gandi.net.; 50 fb.mail.gandi.net.
current root TXT SPF: "v=spf1 include:_mailcust.gandi.net ?all"
Resend API probe with local key: 401 restricted_api_key, "restricted to only send emails"
owner decision: replacing Gandi root MX is acceptable because Gandi mailboxes are not in use
```

Remaining blocker:

- Need a Resend API key with domain/webhook/receiving permissions, or a
  dashboard-created webhook endpoint plus its one-time signing secret, before
  DNS can safely be changed.
- Need the exact Resend receiving MX record for `choir.news` before replacing
  the current Gandi MX records. Do not guess it from generic docs.

Next safe operation:

1. Create or provide a Resend key with receiving/domain/webhook access.
2. Register webhook endpoint
   `https://choir.news/api/email/resend/webhook` for `email.received`.
3. Store the returned signing secret as `RESEND_WEBHOOK_SECRET` in
   `/var/lib/go-choir/maild.env`.
4. Enable receiving for `choir.news` in Resend and copy the exact required MX
   record.
5. Snapshot current Gandi DNS, then replace root MX with exact Resend records.
   The owner has accepted that root-domain inbound mail will leave Gandi
   mailbox delivery and go to Resend.

## Staging Evidence Finding: maild health and manual deploy path

Recorded: 2026-05-26.

Problem:

The deployed service is active and reachable, but the evidence surface is still
too weak for the final email ingress acceptance proof. `maild` only inherits the
generic `/health` response from the shared server, so health does not report
whether Resend API credentials, webhook signing, storage, and mailbox counters
are in the expected state. The staging deploy smoke loop also probes ports
`8081`, `8082`, `8083`, `8084`, and `8086`, but not `8087`, so CI/deploy logs
would not directly prove that `maild` is healthy after future pushes.

Related deploy evidence gap:

After commit `9192f37`, multiple pushed non-doc commits reached `origin/main`
but did not produce GitHub Actions runs visible through the Actions API. The
mission therefore used a manual source-truth deploy from Node B's
`/opt/go-choir` checkout. The CI workflow also has no `workflow_dispatch`
trigger, so there is no explicit manual deploy recovery path when push runs do
not materialize.

Evidence:

```text
.github/workflows/ci.yml health loop before fix: 8081 8082 8083 8084 8086
maild route proof used indirect GET /api/email/resend/webhook -> 405
GitHub Actions latest visible CI run after several pushed commits: 26447116904 at 9192f37
manual deployed commit: 76c5d18a7fb705c5befcf35abadd2df3b9132fbb
```

Belief-state update:

- Final acceptance should have a direct `maild` health artifact that can be
  checked locally, in CI deploy logs, and on Node B without exposing secrets.
- The deployment path needs an operator-invokable workflow path for recovery
  when ordinary push webhooks do not create Actions runs.

Required next change:

- Add a custom `maild` health handler that reports safe configuration booleans
  and mailbox counters.
- Include `8087` in staging health probes.
- Add a manual `workflow_dispatch` path that can force a host/frontend staging
  deploy from `main` without requiring an extra source edit.

Resolution checkpoint:

- Resolved by the direct `maild` health checkpoint and later Actions recovery.
  `internal/maild/health.go` reports safe credential/configuration booleans and
  mailbox counters; `.github/workflows/ci.yml` probes `8087`; and the workflow
  now has `workflow_dispatch` with `force_staging_deploy`.
- The historical manual deploys remain a protocol violation and are recorded as
  such above; subsequent accepted behavior proof used GitHub Actions deploy
  evidence instead of manual Node B deployment.

## Deployed Checkpoint: direct maild health proof

Recorded: 2026-05-26.

Status:

The `maild` health evidence gap is closed for the deployed service. The
workflow file now includes a manual dispatch path and the deploy smoke loop now
checks port `8087`, but GitHub Actions still did not create a push run for the
behavior commit and the first manual dispatch attempt returned an API 500.
Staging was therefore advanced again through the manual source-truth Node B
deploy path.

Evidence:

```text
behavior commit: 2beec3c1f811d565fa1b78dfcb9f734998efba41
local focused test: nix develop -c go test ./internal/maild ./cmd/maild ./internal/proxy
workflow YAML parse check: ruby -e 'require "yaml"; YAML.load_file(".github/workflows/ci.yml")'
push run evidence: no Actions run/check suite appeared for 2beec3c
manual workflow dispatch attempt: gh workflow run CI --ref main -f force_staging_deploy=true -> HTTP 500
deploy method: Node B /opt/go-choir fast-forward to origin/main, NixOS build/switch, service restart
staging /health: proxy and sandbox report deployed_commit 2beec3c1f811d565fa1b78dfcb9f734998efba41
maild local health probe:
  status: ok
  service: maild
  primary_domain: choir.news
  resend_api_key_configured: true
  webhook_secret_configured: false
  root_owner_id_configured: true
  storage_root_configured: true
  webhook_max_bytes_configured: true
  stats.aliases: 1
  stats.messages: 0
  stats.quarantined_attachments: 0
  stats.webhook_events: 0
maild deploy smoke probes: 8081, 8082, 8083, 8084, 8086, 8087 all returned health
public route proof: GET https://choir.news/api/email/resend/webhook -> 405 method not allowed
mail state modes:
  600 root root /var/lib/go-choir/maild.env
  700 root root /var/lib/go-choir/mail
  600 root root /var/lib/go-choir/mail/mail.db
```

Remaining blocker:

- The final real inbound acceptance path still requires a Resend key or
  dashboard action that can create/inspect receiving domains and webhook
  endpoints. The available key remains send-only; `RESEND_WEBHOOK_SECRET` is
  intentionally absent and health now reports that directly.

Residual platform risk:

- GitHub Actions did not emit runs for several post-`9192f37` pushes and manual
  dispatch returned an API 500. The workflow now contains a recovery trigger,
  but GitHub's event/run creation path remains unproven in this session.

## Evidence Finding: admin inspection surface absent

Recorded: 2026-05-26.

Problem:

The reference doc requires admin/dev inspection through a CLI command or a
localhost-only endpoint, and specifically forbids unauthenticated public raw
message/admin inspection. The current deployed slice has a minimal authenticated
Email app and safe health counters, but no operator inspection tool for
messages, quarantined attachments, source packets, aliases, or webhook events.
Once real Resend inbound is enabled, this would make the acceptance proof rely
too heavily on browser UI and ad hoc SQLite commands on Node B.

Evidence:

```text
reference invariant: docs/choir-email-reference-v0.md says admin/dev inspection should be a CLI command or localhost-only endpoint
current implementation: no cmd/maildctl package exists
available deployed evidence: /health counters only; no message/source-packet listing command
```

Belief-state update:

- A read-only `maildctl` CLI is the right small addition: it keeps admin
  inspection off the public edge, avoids exposing raw data through Caddy, and
  gives the final real inbound/quarantine/source-packet acceptance proof a
  repeatable command.

Required next change:

- Add `cmd/maildctl` with read-only subcommands for safe stats, aliases,
  message lists, message details, attachments, source packets, and webhook
  events.
- Package `maildctl` in Nix so it is available on Node B after deploy.

Resolution checkpoint:

- Resolved by `cmd/maildctl`, the `maildctl` package in `flake.nix`, and
  `environment.systemPackages` inclusion in `nix/node-b.nix`.
- The CLI now exposes read-only `stats`, `aliases`, `webhooks`, `messages`,
  `message`, `attachments`, `source-packet`, and `ingress-events` commands for
  final acceptance evidence without creating a public admin endpoint.

## Deployed Finding: maildctl empty list output

Recorded: 2026-05-26.

Problem:

The first deployed `maildctl` proof worked, but `maildctl webhooks --limit 5`
printed JSON `null` when no webhook rows existed. That is technically valid for
a nil Go slice, but it is a weak operator evidence format. Empty collections
should print `[]` so acceptance scripts can distinguish "query succeeded and
there are no rows" from "field absent/null".

Evidence:

```text
deployed commit: 1c4a8d9
maildctl stats -> {"aliases":1,"messages":0,"quarantined_attachments":0,"webhook_events":0}
maildctl aliases -> one 000@choir.news alias mapped to 5bd6de97-3b58-408c-bf89-c42c81b083de
maildctl webhooks --limit 5 -> null
```

Required next change:

- Initialize maild store list results as empty slices so JSON CLI output is
  stable `[]` for empty query results.

Resolution checkpoint:

- Resolved in the store list methods used by `maildctl`; empty list output is
  stable JSON `[]`.
- Focused coverage includes `TestRunWebhooksPrintsEmptyArray`, and deployed
  mission evidence records `maildctl webhooks --limit 5: []`.

## Deployed Checkpoint: maildctl inspection proof

Recorded: 2026-05-26.

Status:

The read-only operator inspection surface is deployed. `maildctl` is installed
through the Node B NixOS system path and can inspect safe stats, aliases, and
webhook receipts without exposing a public admin endpoint. Empty list output is
now stable JSON `[]`.

Evidence:

```text
latest deployed commit: 11bf6e6e958a94a8344ab061ac9fc72036f2ca92
local focused test: nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
Node B build: NixOS closure built maildctl-0.1.0 and activated it in environment.systemPackages
deploy smoke probes: 8081, 8082, 8083, 8084, 8086, 8087 health ok
public /health: proxy and sandbox report deployed_commit 11bf6e6e958a94a8344ab061ac9fc72036f2ca92
maild /health:
  resend_api_key_configured: true
  webhook_secret_configured: false
  root_owner_id_configured: true
  stats.aliases: 1
  stats.messages: 0
  stats.quarantined_attachments: 0
  stats.webhook_events: 0
maildctl stats:
  aliases: 1
  messages: 0
  quarantined_attachments: 0
  webhook_events: 0
maildctl aliases:
  alias-choir-news-000 -> choir.news/000 -> user 5bd6de97-3b58-408c-bf89-c42c81b083de
maildctl webhooks --limit 5: []
```

Remaining blocker:

- Real inbound mail, source-packet MAS handoff from a real received message,
  and real attachment quarantine still require Resend receiving/webhook
  credentials and exact Resend MX records. The deployed health and CLI now make
  that blocker explicit and verifiable instead of implicit.

## Staging Finding: outbound provider failure is opaque

Recorded: 2026-05-26.

Problem:

A low-impact outbound proof through deployed `maild` failed with HTTP 502 and
did not store a Sent row. This was expected to exercise the send-only Resend key
without touching DNS/MX. The failure response is intentionally generic for the
browser, but the service also does not currently record enough bounded provider
evidence to distinguish key scope, sender-domain verification, payload shape,
or provider outage.

Evidence:

```text
deployed commit: 11bf6e6e958a94a8344ab061ac9fc72036f2ca92
request path: POST http://127.0.0.1:8087/api/email/send
authenticated owner header: X-Authenticated-User: 5bd6de97-3b58-408c-bf89-c42c81b083de
from alias: 000@choir.news
test recipient: delivered@resend.dev
result: HTTP 502 {"error":"failed to send email"}
post-check: maildctl messages --owner 5bd6de97-3b58-408c-bf89-c42c81b083de sent -> []
```

Belief-state update:

- Outbound remains unproven with the deployed Resend key.
- `maild` should keep browser responses generic, but operator evidence needs a
  bounded provider status/reason that does not expose secrets or message body.

Required next change:

- Add bounded provider error classification/logging for outbound sends, then
  retry the outbound proof and record the actual provider reason.

Resolution checkpoint:

- Resolved by bounded outbound provider logging. The browser response remains
  generic, while operator logs captured the provider status and bounded JSON
  reason showing Resend rejected the send because `choir.news` is not verified.
- The remaining outbound blocker is provider/domain verification, not an
  unexplained local send-path failure.

## Deployed Checkpoint: outbound blocked on Resend domain verification

Recorded: 2026-05-26.

Status:

The outbound provider failure is now explained. Deployed `maild` logs a bounded
provider status/reason while keeping the browser response generic. Retrying the
same low-impact outbound proof produced a Resend `403` explaining that
`choir.news` is not verified in Resend.

Evidence:

```text
behavior commit: 30b0196
local focused test: nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
deploy method: Node B /opt/go-choir fast-forward to origin/main, NixOS build/switch, maild/proxy restart
deploy smoke probes: 8081, 8082, 8083, 8084, 8086, 8087 health ok
outbound retry:
  request path: POST http://127.0.0.1:8087/api/email/send
  owner: 5bd6de97-3b58-408c-bf89-c42c81b083de
  from: 000@choir.news
  recipient: delivered@resend.dev
  response: HTTP 502 {"error":"failed to send email"}
  sent folder after retry: []
bounded provider log:
  status=403
  detail="{\"statusCode\":403,\"message\":\"The choir.news domain is not verified. Please, add and verify your domain on https://resend.com/domains\",\"name\":\"validation_error\"}"
```

Belief-state update:

- Outbound code path reached Resend and failed for provider/domain setup, not
  alias ownership, request shape, or local storage.
- The send-only Resend key is usable enough to call `POST /emails`, but
  `choir.news` must be verified in Resend before any real outbound proof can
  pass.
- The remaining provider work is now both inbound and outbound:
  verify `choir.news` in Resend, create/configure receiving and webhook secret,
  then only after exact Resend DNS records are known update Gandi DNS.

Remaining blocker:

- Need Resend domain verification access/records for `choir.news`, plus
  receiving/webhook setup. Do not change Gandi MX/SPF/DKIM/DMARC records from
  guesses; fetch exact records from Resend first.

## Evidence Finding: reply threading metadata not wired

Recorded: 2026-05-26.

Problem:

The Email app sends `reply_to_message_id` when the owner replies, and Resend's
receiving docs recommend using the received email's RFC `message_id` as
`In-Reply-To` so mail clients can thread replies. `maild` currently accepts
`reply_to_message_id` in the request shape but ignores it when building the
Resend Send Email payload. Inbound records also store the Resend received email
id as `provider_message_id`, but do not expose the RFC `message_id` through the
typed message record used by the send path.

Evidence:

```text
frontend/src/lib/EmailApp.svelte sends reply_to_message_id: selectedMessage.id
internal/maild/send.go buildResendSendRequest does not read ReplyToMessageID
internal/maild/ingest.go receives resendReceivedEmail.MessageID but current EmailMessage struct does not expose raw_headers_json/message_id
official Resend receiving docs: use webhook data.message_id as In-Reply-To for replies
```

Belief-state update:

- Reply/send is still owner-initiated, but v0 reply behavior is incomplete
  until `maild` maps an owned reply target to `In-Reply-To`/`References`.
- This can be fixed without changing public authority boundaries or requiring
  provider credentials.

Required next change:

- Preserve received RFC `message_id` in stored message metadata.
- When `reply_to_message_id` is provided, require that the current owner can
  read the target message, extract its RFC `message_id`, and set
  `In-Reply-To` and `References` in the Resend send payload.

Resolution checkpoint:

- Resolved in `internal/maild/send.go` and covered by
  `internal/maild/send_test.go`: owner-visible reply targets now populate
  `In-Reply-To` and `References`; missing RFC ids fail before provider send;
  unowned reply targets are rejected.
- The Email app still initiates reply explicitly from the owner action, so the
  fix does not grant inbound content any send authority.

## Deployed Checkpoint: owner reply threading wired

Recorded: 2026-05-26.

Status:

The documented reply-threading gap is fixed and deployed. Inbound records now
preserve Resend's received RFC `message_id` in raw message metadata, and
owner-initiated replies resolve `reply_to_message_id` through the current
owner's message visibility before adding `In-Reply-To` and `References` to the
Resend Send Email payload. Missing RFC message ids fail before any provider
send, and unowned reply targets are rejected.

Evidence:

```text
behavior commit: d749fdcfb329226f73ce4717b86f1ac0eba5e1a0
local focused test: nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
test coverage:
  TestHandleResendWebhookFetchesAndStoresInboundMessage asserts stored message_id
  TestHandleSendAddsReplyHeadersForOwnedReplyTarget asserts In-Reply-To and References
  TestHandleSendRejectsReplyTargetMissingMessageID asserts no send without RFC id
  TestHandleSendRejectsUnownedReplyTarget asserts owner boundary before send
GitHub Actions: no new run emitted for d749fdc; newest run remains 9192f37
deploy method: Node B /opt/go-choir fast-forward to origin/main, NixOS build/switch, maild/proxy restart
deploy smoke probes: 8081, 8082, 8083, 8084, 8086, 8087 health ok
public /health:
  proxy deployed_commit: d749fdcfb329226f73ce4717b86f1ac0eba5e1a0
  sandbox deployed_commit: d749fdcfb329226f73ce4717b86f1ac0eba5e1a0
maild /health:
  status: ok
  resend_api_key_configured: true
  webhook_secret_configured: false
  root_owner_id_configured: true
  stats.aliases: 1
  stats.messages: 0
  stats.quarantined_attachments: 0
  stats.webhook_events: 0
systemd: go-choir-maild active; go-choir-proxy active
```

Belief-state update:

- The owner reply code path is production-shaped, but real provider reply proof
  still waits on Resend domain verification because outbound currently fails at
  provider setup.
- `maild` still does not need direct MAS authority for email replies; replies
  remain explicit owner sends through the authenticated Email app/proxy path.

Remaining blocker:

- Real inbound, real outbound, and real reply threading require exact Resend
  domain verification, receiving/webhook setup, and Gandi DNS changes from
  provider-sourced records. `RESEND_WEBHOOK_SECRET` is still absent on Node B.

## Provider Readiness Finding: available Resend key is send-only

Recorded: 2026-05-26.

Status:

A read-only provider readiness probe now exists at
`scripts/mail-provider-readiness`. It checks local credential presence, Resend
domains, Resend webhooks, Gandi LiveDNS mail-related records, public DNS, and
Node B `maild` health. It does not create domains, create webhooks, deploy
secrets, or mutate DNS.

Evidence:

```text
command: scripts/mail-provider-readiness
mode: read-only
local credentials:
  RESEND_API_KEY: configured
  RESEND_WEBHOOK_SECRET: missing
  GANDI_PAT: configured
Resend /domains:
  http_status: 401
  provider_error.name: restricted_api_key
  provider_error.message: This API key is restricted to only send emails
Resend /webhooks:
  http_status: 401
  provider_error.name: restricted_api_key
  provider_error.message: This API key is restricted to only send emails
Gandi LiveDNS mail state:
  @ MX: 10 spool.mail.gandi.net.; 50 fb.mail.gandi.net.
  @ TXT: "v=spf1 include:_mailcust.gandi.net ?all"
  no _dmarc TXT
public DNS:
  MX remains Gandi mail defaults
  root TXT remains Gandi SPF
Node B maild health:
  status: ok
  resend_api_key_configured: true
  webhook_secret_configured: false
  stats.messages: 0
  stats.webhook_events: 0
```

Belief-state update:

- The remaining blocker is external/provider-authority, not local service
  readiness: the currently available Resend key can send API requests but cannot
  read the domain records or webhooks needed for safe DNS configuration.
- Gandi can be inspected and later mutated with the available PAT, but the
  mission invariant still forbids DNS mutation until Resend supplies exact
  provider records and a webhook signing secret.
- Owner authorization for the Gandi mail-routing cutover is now explicit:
  replacing root-domain Gandi MX is acceptable because Gandi mailboxes are not
  in use for Choir.

Next executable probe:

- Use a broader temporary Resend API key or authenticated dashboard session to
  create/retrieve `choir.news`, enable receiving, create/retrieve the
  `email.received` webhook for `https://choir.news/api/email/resend/webhook`,
  and capture exact DNS records plus `RESEND_WEBHOOK_SECRET`.

## Tooling Checkpoint: reversible Gandi DNS application path

Recorded: 2026-05-26.

Status:

The Gandi DNS cutover path now has dry-run-first operator tooling:
`scripts/mail-gandi-plan-records` converts a Resend domain JSON response into
Gandi LiveDNS RRset operations, and `scripts/mail-gandi-rollback-records`
restores or deletes the same affected RRsets from a pre-apply Gandi snapshot.
Both scripts refuse to apply root `@/MX` changes unless `--allow-root-mx` is
provided with `--apply`.

Evidence:

```text
bash -n scripts/mail-gandi-plan-records scripts/mail-gandi-rollback-records
sample dry-run plan:
  input: sample Resend records with send MX/TXT, DKIM TXT, and receiving apex MX
  result: planned @/MX, send/MX, send/TXT, resend._domainkey/TXT
  Gandi read: current @/MX shown as 10 spool.mail.gandi.net.; 50 fb.mail.gandi.net.
  mutation: none
  root MX guard: dry-run warns; apply would require --allow-root-mx --apply
sample dry-run rollback:
  input: sample Gandi snapshot plus same sample Resend records
  result: restores @/MX from snapshot and deletes absent send/TXT
  mutation: none
```

Belief-state update:

- The Gandi side is now ready for a controlled cutover once exact Resend
  provider records are available. Remaining risk is not record application
  mechanics; it is obtaining provider truth and webhook secret from Resend.
- The owner has accepted the root MX cutover. The scripts still require an
  explicit operator flag so accidental root mail-routing changes remain
  unlikely.

## Tooling Checkpoint: Resend setup helper

Recorded: 2026-05-26.

Status:

`scripts/mail-resend-setup` now wraps the current Resend domain and webhook API
surface needed for the remaining provider-authority step. It defaults to
read-only inspection. Mutating operations require `--apply` plus explicit
`--ensure-domain`, `--ensure-webhook`, or `--verify-domain`. Webhook signing
secrets are redacted from stdout and written only when
`--write-webhook-secret-file` is provided.

Evidence:

```text
official Resend API docs checked:
  POST /domains accepts capabilities.sending and capabilities.receiving
  PATCH /domains/:domain_id can update capabilities.receiving
  GET /domains/:domain_id returns DNS records
  POST /webhooks accepts endpoint and events
  GET /webhooks/:webhook_id returns signing_secret
local current-key run:
  command: scripts/mail-resend-setup --write-domain-json /tmp/choir-resend-domain-test.json --write-webhook-secret-file /tmp/choir-resend-webhook-secret-test.env
  result: GET /domains -> 401 restricted_api_key
  mutation: none
  generated files: none
fake-curl success-path run:
  command: scripts/mail-resend-setup --apply --ensure-domain --ensure-webhook --write-domain-json <tmp>/domain.json --write-webhook-secret-file <tmp>/webhook.env
  result: domain JSON saved; webhook secret saved with mode 600
  stdout: signing_secret redacted/not printed
```

Belief-state update:

- The remaining provider step can now be executed through a repeatable script
  once a sufficiently scoped Resend key is available.
- The current key still cannot retrieve provider truth, so no Resend/Gandi
  production state was changed in this checkpoint.

Next executable probe:

- Run `scripts/mail-resend-setup --apply --ensure-domain --ensure-webhook`
  with a sufficiently scoped Resend key, save the domain JSON and webhook
  secret file, deploy `RESEND_WEBHOOK_SECRET` to Node B, then dry-run and apply
  Gandi from the saved Resend domain JSON.

## Tooling Checkpoint: webhook secret deploy handoff

Recorded: 2026-05-26.

Status:

`nix/deploy-mail-creds.sh` can now consume the webhook-secret file generated by
`scripts/mail-resend-setup` via `CHOIR_MAIL_WEBHOOK_SECRET_FILE` or
`RESEND_WEBHOOK_SECRET_FILE`. It also supports `--dry-run`, which validates
required credential presence and reports configured key names without writing
remote files or restarting `maild`.

Evidence:

```text
bash -n nix/deploy-mail-creds.sh
positive dry-run:
  CHOIR_MAIL_ENV_FILE=<tmp>/base.env
  CHOIR_MAIL_WEBHOOK_SECRET_FILE=<tmp>/secret.env
  command: nix/deploy-mail-creds.sh --dry-run test-host
  result: configured_keys includes RESEND_API_KEY, RESEND_WEBHOOK_SECRET,
          MAILD_ROOT_OWNER_ID, MAILD_PRIMARY_DOMAIN
  mutation: none
negative dry-run:
  CHOIR_MAIL_ENV_FILE=<tmp>/base.env with only RESEND_API_KEY
  command: nix/deploy-mail-creds.sh --dry-run
  result: exit=1, missing required mail credential RESEND_WEBHOOK_SECRET
  mutation: none
```

Belief-state update:

- Once a real Resend webhook secret exists, the host secret deployment step no
  longer requires copying the secret into `.env`; it can flow from the generated
  secret file directly into `/var/lib/go-choir/maild.env`.
- The deploy script still fails closed when the webhook secret is absent, which
  preserves the invariant that `maild` does not accept unsigned/unknown
  provider webhooks.

## Tooling Checkpoint: mail acceptance evidence surface

Recorded: 2026-05-26.

Status:

The real-mail acceptance path now has a read-only verifier and an ingress-event
receipt surface. `maildctl ingress-events` lists owner/message-scoped MAS
handoff records. `maild` records those rows only from an internal proxy call,
and the proxy strips client-supplied `X-Internal-Caller` before normal maild
forwarding. After successful owner-triggered Send to Choir, the proxy records
the source packet id, owner id, message id, prompt-bar submission id, and status
back in `maild`.

Evidence:

```text
focused tests:
  nix develop -c go test ./internal/maild ./internal/proxy ./cmd/maildctl ./cmd/maild
coverage:
  TestHandleMessageIngressEventsRequiresInternalCaller
  TestEmailSendToChoirFetchesSourcePacketAndSubmitsPromptBar
  TestRunIngressEventsPrintsOwnerScopedRows
acceptance checker:
  bash -n scripts/mail-acceptance-check
  fake ssh/maildctl success path verifies:
    folder: quarantine
    attachment_count: 1
    source_label: UNTRUSTED_EXTERNAL_EMAIL
    ingress_count: 0
deployed checkpoint:
  commit: 73beb6e127199ea77c035e3729a76f23c8d03a16
  CI: run 26447497910 failed during external action setup/download before project code gates
  deploy: manual Node B fast-forward, NixOS closure build, switch-to-configuration,
          restart go-choir-maild/go-choir-proxy, reload Caddy
  health: public /health reports deployed_commit 73beb6e127199ea77c035e3729a76f23c8d03a16
  maild: /health reports stats.ingress_events 0 and webhook_secret_configured false
  command: maildctl ingress-events --owner 5bd6de97-3b58-408c-bf89-c42c81b083de --limit 5
  result: []
```

Belief-state update:

- Final acceptance can now prove the negative condition "real inbound mail did
  not trigger MAS handoff" by running `scripts/mail-acceptance-check
  --expect-ingress-events 0` before owner action.
- After owner action, final acceptance can prove the positive handoff receipt by
  rerunning the same checker with `--expect-ingress-events 1` and correlating
  the ingress event's `conductor_submission_id` with prompt-bar/Trace evidence.

## UI Checkpoint: raw-header Details surface

Recorded: 2026-05-26.

Status:

The authenticated message-detail API now returns stored raw headers, and the
Email app renders them in a collapsed Details block. Plain text remains the
primary body surface; raw headers are available for inspection without becoming
instructions or a privileged action surface.

Evidence:

```text
commit: 1e3d54ae3ebcdfb00646fca6a4645ee18d2ccac2
local tests:
  nix develop -c go test ./internal/maild ./internal/proxy ./cmd/maildctl ./cmd/maild
  npm run build in frontend
local Playwright component render:
  screenshot: /tmp/choir-email-headers-details.png
  closed Details hides <msg-1@example.com>: true
  opened Details shows message_id and authentication-results: true
deployed proof:
  public /health deployed_commit 1e3d54ae3ebcdfb00646fca6a4645ee18d2ccac2
  maild /health status ok
```

Belief-state update:

- The v0 Email app now satisfies the minimal inspection requirement for raw
  headers/details while keeping HTML and attachments non-executable.
- Final real-mail acceptance still needs provider-backed messages before this
  can be proven with real Resend payload headers.

## Evidence Finding: message detail recipients are not surfaced

Recorded: 2026-05-26.

Problem:

`maild` stores normalized `to`, `cc`, and `bcc` rows in
`email_message_recipients`, but the authenticated message-detail response does
not return those rows. The Email app currently displays the active alias
`000@choir.news` as the detail recipient. That is acceptable for the seeded root
alias demo but is weaker than the v0 inspection model once plus aliases,
forwarded mail, CCs, and outbound Sent rows exist.

Evidence:

```text
schema: internal/maild/store.go creates email_message_recipients
ingest: internal/maild/ingest.go inserts to/cc/bcc rows for inbound mail
send: internal/maild/send.go inserts to/cc/bcc rows for outbound mail
API: internal/maild/api.go messageDetailResponse lacks recipients
UI: frontend/src/lib/EmailApp.svelte detail header renders activeAddress as To
```

Required next change:

- Add an owner-scoped recipient read path to `maild`, include To/Cc/Bcc in the
  message-detail API, and render those stored recipients in the Email app
  Details block without trusting them as instructions.

Resolution checkpoint:

- Resolved by `Store.ListRecipients`, the authenticated message-detail
  `recipients` response, and Email app rendering for To/Cc/Bcc in the detail
  header/Details block.
- Focused coverage exists in `TestHandleMessageDetailIncludesStoredRecipients`.

## UI Checkpoint: stored-recipient Details surface

Recorded: 2026-05-26.

Status:

The authenticated message-detail API now returns stored To/Cc/Bcc recipient
rows, and the Email app renders those recipients in the detail header and
collapsed Details block. This removes the prior assumption that every selected
message was addressed only to the active root alias.

Evidence:

```text
commit: 843ec907117e26ef741b7b1a62d58f689839dd79
local tests:
  nix develop -c go test ./internal/maild ./internal/proxy ./cmd/maildctl ./cmd/maild
  npm run build in frontend
local Playwright component render:
  screenshot: /tmp/choir-email-recipient-details.png
  header shows sender@example.com -> 000+read@choir.news
  Details shows To 000+read@choir.news, Cc Copy Person <copy@example.com>, and Bcc blind@example.com
deployed proof:
  public /health deployed_commit 843ec907117e26ef741b7b1a62d58f689839dd79
  maild /health status ok
```

Belief-state update:

- Message inspection is now more faithful for plus aliases, forwarded mail,
  copied recipients, and outbound Sent rows.
- Final real-mail acceptance still needs provider-backed messages before this
  can be proven with actual Resend recipient payloads.

## Evidence Finding: compose surface is absent

Recorded: 2026-05-26.

Problem:

The reference doc's v0 UI direction includes owner-initiated compose/reply with
fixed `000@choir.news` From, To, Subject, body, and Send. The current Email app
supports reply from a selected message, but it has no standalone Compose action.
That means outbound can be locally proven through `maild` API tests, but the
minimal Email app cannot yet initiate a new explicit owner send.

Evidence:

```text
reference: docs/choir-email-reference-v0.md says Compose/reply includes fixed From, To, Subject, body, Send
backend: internal/maild/send.go already supports POST /api/email/send with to_addresses/subject/text_body
frontend: frontend/src/lib/EmailApp.svelte sendReply posts /api/email/send, but no compose state or Compose button exists
```

Required next change:

- Add a minimal Compose panel to the Email app using the existing authenticated
  `/api/email/send` route. Keep it owner-initiated, fixed to `000@choir.news`,
  plain text only, and independent of inbound message content.

Resolution checkpoint:

- Resolved in `frontend/src/lib/EmailApp.svelte`: the Email app exposes a
  minimal Compose panel with fixed From `000@choir.news`, To, Subject, plain
  text body, and an explicit Send action through `/api/email/send`.
- The feature remains inside the owner-authenticated UI path and does not let
  inbound email trigger outbound sending.

## UI Checkpoint: minimal Compose surface

Recorded: 2026-05-26.

Status:

The Email app now has a minimal owner-initiated Compose panel. It uses fixed
From `000@choir.news`, accepts To, Subject, and plain text body, and posts to
the existing authenticated `/api/email/send` route. It does not add drafts,
rich HTML, alias management, or automation.

Evidence:

```text
commit: 5378215c341813dcec8d985c105c57c9f6181e3b
local tests:
  npm run build in frontend
local Playwright component render:
  screenshot: /tmp/choir-email-compose.png
  captured send payload:
    from_address: 000@choir.news
    to_addresses: [friend@example.com, second@example.com]
    subject: Compose proof
    text_body: Owner composed message.
CI:
  run 26448967008 partially recovered: frontend build, non-runtime tests,
  integration smoke, and runtime shards 0/3 passed; Go Vet + Build and runtime
  shards 1/2 failed during setup; staging deploy skipped.
deployed proof:
  manual Node B deploy succeeded
  public /health deployed_commit 5378215c341813dcec8d985c105c57c9f6181e3b
  maild /health status ok
```

Belief-state update:

- The v0 UI now covers both new owner-initiated compose and selected-message
  reply through the same backend send route.
- Real outbound acceptance still depends on Resend domain verification and a
  sufficiently scoped/provider-valid key.

## Evidence Finding: message list attachment indicator is absent

Recorded: 2026-05-26.

Problem:

The v0 UI scope requires a message-row attachment indicator. `maild` stores
attachment metadata and the detail API sets `has_attachments` only after loading
the selected message's attachment list, but the list API does not currently
compute `has_attachments` for each row. The Email app therefore cannot show an
attachment signal in Inbox/Sent/Quarantine before opening a message.

Evidence:

```text
reference: docs/choir-email-reference-v0.md requires message-list attachment indicator
scope: docs/mission-maild-email-ingress-v0.md V0 UI Scope requires attachment indicator
API type: internal/maild/api.go messageSummary has HasAttachments
list path: internal/maild/api.go handleMessageList uses summarizeMessage(msg)
store path: internal/maild/store.go ListMessages does not include attachment existence
UI: frontend/src/lib/EmailApp.svelte message rows render sender/time/subject/snippet/trust only
```

Required next change:

- Compute attachment existence for owner-visible message list rows and render a
  compact attachment indicator in the Email app row without loading or
  processing attachment content.

Resolution checkpoint:

```text
commit: ae8cb7f7f80a3944998549991227cd559832d150
store: internal/maild/store.go ListMessages/GetMessage compute HasAttachments with EXISTS over email_attachments
api: internal/maild/api.go summarizeMessage propagates HasAttachments
test: internal/maild/api_test.go TestHandleMessagesListsAttachmentIndicator
ui: frontend/src/lib/EmailApp.svelte renders a paperclip row indicator when message.has_attachments is true
local proof:
  nix develop -c go test ./internal/maild ./cmd/maild
  npm run build
deployed proof:
  GitHub Actions CI run 26450002582 passed all Go/frontend gates and Deploy to Staging
  public /health deployed_commit ae8cb7f7f80a3944998549991227cd559832d150
```

## Evidence Finding: MAS handoff acceptance needs an operator surface

Recorded: 2026-05-26.

Problem:

The final acceptance needs to prove two separate facts:

1. Real inbound mail creates an untrusted source packet and quarantines
   attachments without triggering privileged action.
2. A later owner action can send that source packet to Choir through the
   proxy-owned prompt-bar path.

`maild` already has an `email_ingress_events` table for the handoff record, but
`maildctl` cannot inspect those rows. This leaves an operator with message,
attachment, and source-packet evidence, but no direct read-only evidence for
"zero handoffs before owner action" or "handoff recorded after owner action."

Evidence:

```text
schema: internal/maild/store.go creates email_ingress_events
maildctl commands: stats, aliases, webhooks, messages, message, attachments, source-packet
missing command: ingress-events/list handoff records
```

Resolution checkpoint:

- Resolved by the later tooling checkpoint above. `maildctl ingress-events`
  now reads owner/message-scoped handoff records through
  `Store.ListIngressEvents`, and `maild` records those rows only through the
  internal proxy callback at `/api/email/messages/:id/ingress-events`.
- Focused coverage exists in `TestRunIngressEventsPrintsOwnerScopedRows`,
  `TestHandleMessageIngressEventsRequiresInternalCaller`, and
  `TestEmailSendToChoirFetchesSourcePacketAndSubmitsPromptBar`.
- Final real-mail acceptance can use `scripts/mail-acceptance-check
  --expect-ingress-events 0` before owner action, then rerun with
  `--expect-ingress-events 1` after explicit Send to Choir.

## Provider Readiness Recheck: Resend receiving still externally blocked

Recorded: 2026-05-26 after commit `600371a`.

Status:

The provider readiness probe still fails before the DNS/MX realism step because
the available Resend key is restricted to sending only. `maild` remains deployed
and healthy, but it has no webhook signing secret, so real Resend inbound
webhooks must remain disabled until the provider state can be inspected and the
secret can be installed through `/var/lib/go-choir/maild.env`.

Evidence:

```text
command: scripts/mail-provider-readiness
local credential presence:
  resend_api_key: configured
  resend_webhook_secret: missing
  gandi_pat: configured
Resend Domains:
  http_status: 401
  provider_error.name: restricted_api_key
  provider_error.message: This API key is restricted to only send emails
Resend Webhooks:
  http_status: 401
  provider_error.name: restricted_api_key
  provider_error.message: This API key is restricted to only send emails
Gandi LiveDNS:
  MX: 10 spool.mail.gandi.net.; 50 fb.mail.gandi.net.
  SPF TXT: "v=spf1 include:_mailcust.gandi.net ?all"
  DKIM CNAMEs: gm1/gm2/gm3._domainkey -> gandimail
  DMARC: no public _dmarc TXT observed
Node B maild health:
  status: ok
  resend_api_key_configured: true
  webhook_secret_configured: false
  root_owner_id_configured: true
  stats.messages: 0
  stats.quarantined_attachments: 0
  stats.webhook_events: 0
  stats.ingress_events: 0
```

Belief-state update:

- The external blocker is unchanged: we need either a broader Resend API key or
  dashboard access to verify domain/webhook records and retrieve/install the
  webhook signing secret.
- The no-DNS-mutation invariant remains satisfied. Gandi is still routing mail
  to Gandi, and `choir.news` must not be switched to Resend MX until the exact
  Resend records and rollback plan are known.

Next executable probe:

- Obtain a Resend key/session that can read domain and webhook configuration,
  rerun `scripts/mail-provider-readiness`, then configure webhook secret and
  DNS only from verified provider records.

## Provider Readiness Recheck: clean deploy still blocked on Resend authority

Recorded: 2026-05-27 after deploy cleanup commit `38ff631`.

Status:

The GitHub Actions deploy-cleanup recovery proved Node B is no longer running a
dirty `/opt/go-choir` checkout, and the deployed `maild` service is healthy.
The remaining provider/DNS blocker is unchanged: the available Resend key is
restricted to send-only scope and cannot read domains or webhooks, so the
mission still lacks exact Resend DNS records and a webhook signing secret.

Official docs rechecked before any provider mutation:

```text
Resend create/retrieve/update domain docs:
  - create/retrieve domain responses include capabilities and DNS records
  - create/update domain accepts capabilities.sending and capabilities.receiving
Resend webhook docs:
  - create/retrieve/list webhook responses expose signing_secret
  - webhook verification requires the raw request body and Svix headers
Resend email.received docs:
  - webhook contains metadata only; full message/attachments require Receiving
    and Attachments APIs
Resend receiving docs:
  - root-domain MX is acceptable only if replacing existing mailbox routing is
    intended; otherwise use a subdomain
Gandi LiveDNS docs:
  - v5 LiveDNS record APIs use Authorization: Bearer personal access token
```

Evidence:

```text
deployment identity:
  public https://choir.news/health:
    proxy/sandbox deployed_commit: 38ff631142bd13b02c59482007f9b68381eed811
    deployed_at: 2026-05-27T18:12:16Z
  node-b /var/lib/go-choir/deploy.env:
    CHOIR_DEPLOYED_COMMIT=38ff631142bd13b02c59482007f9b68381eed811
  node-b /opt/go-choir:
    git HEAD: 38ff631142bd13b02c59482007f9b68381eed811
    git status: ## main...origin/main

command: scripts/mail-provider-readiness
result:
  local credentials:
    RESEND_API_KEY configured
    RESEND_WEBHOOK_SECRET missing
    GANDI_PAT configured
  Resend Domains:
    http_status: 401
    provider_error.name: restricted_api_key
    provider_error.message: This API key is restricted to only send emails
  Resend Webhooks:
    http_status: 401
    provider_error.name: restricted_api_key
    provider_error.message: This API key is restricted to only send emails
  Gandi LiveDNS:
    @ MX: 10 spool.mail.gandi.net.; 50 fb.mail.gandi.net.
    @ TXT: "v=spf1 include:_mailcust.gandi.net ?all"
    gm1/gm2/gm3._domainkey CNAMEs still point at Gandi mail
    _dmarc TXT absent
  public DNS:
    MX remains Gandi mail defaults
    root TXT remains Gandi SPF
    _dmarc TXT absent
  node-b maild health:
    status ok
    resend_api_key_configured true
    webhook_secret_configured false
    root_owner_id_configured true
    stats aliases=1 messages=0 quarantined_attachments=0 webhook_events=0 ingress_events=0

command:
  scripts/mail-resend-setup --write-domain-json /tmp/choir-resend-domain-20260527.json --write-webhook-secret-file /tmp/choir-resend-webhook-secret-20260527.env
result:
  GET /domains -> 401 restricted_api_key
  no domain JSON file created
  no webhook secret file created
  mutation: none
```

Belief-state update:

- The deployed service boundary is ready for real provider setup, but the
  provider authority boundary is still closed.
- The Gandi side remains safely unchanged, and DNS/MX mutation is still
  forbidden until a scoped Resend key or dashboard session supplies exact domain
  records plus `RESEND_WEBHOOK_SECRET`.
- The root-domain MX cutover remains an intended late step because the owner
  explicitly accepted replacing Gandi mail routing.

Next executable probe:

- Use a Resend dashboard session or broader temporary API key to create/retrieve
  `choir.news`, enable receiving, create/retrieve the `email.received` webhook,
  save the webhook signing secret, and save the exact domain JSON for dry-run
  Gandi planning.

## Local Finding: trusted-upload acceptance does not yet mark trusted sender evidence

Recorded: 2026-05-27 during the receding-horizon local audit after provider
readiness remained blocked on Resend authority.

Problem:

The receive-policy gate correctly rejects a trusted-upload-style alias when the
sender is not whitelisted, and accepts the same exact plus alias after a manual
sender whitelist row exists. But the stored message does not preserve the
trusted-sender classification in the owner-visible `trust_status` field, and
the dedicated `authentication_results_json` column is not populated from
provider headers. This weakens the Email app trust badge and leaves future
trusted-upload review without a clean structured authentication-results field.

Evidence:

```text
code:
  internal/maild/webhook.go: enforceReceivePolicy returns only error/nil, so
  ingest does not know whether the sender matched a trusted whitelist.
  internal/maild/ingest.go: buildInboundRecord sets trust_status to "public"
  unless attachments exist, and never fills AuthenticationResults.
tests:
  internal/maild/webhook_test.go proves unwhitelisted trusted-upload rejection
  and whitelisted trusted-upload acceptance, but does not assert trust_status
  or authentication_results_json.
reference:
  docs/choir-email-reference-v0.md says trusted uploads require sender
  whitelist and recorded message-authentication results, while UI labels include
  "Trusted sender" and "Attachment quarantined".
```

Belief-state update:

- The authority boundary remains intact: accepted inbound content is still
  wrapped as `UNTRUSTED_EXTERNAL_EMAIL` source material and cannot trigger MAS
  actions without owner handoff.
- The local classification evidence is incomplete. A whitelisted trusted upload
  should remain untrusted as source material, but its mailbox message should
  show the owner that the sender matched the trusted-upload gate.

Next executable probe:

- Make receive-policy evaluation return a small classification result, store
  whitelisted trusted-upload messages as `trust_status="trusted"` when there
  are no attachments, keep attachments in quarantine, populate
  `authentication_results_json` from authentication-results headers, and add
  focused maild tests for both fields.

Resolution checkpoint, 2026-05-27:

- `internal/maild/webhook.go` now returns a small receive-policy result so
  ingest can tell whether an inbound message matched a trusted sender
  whitelist.
- `internal/maild/ingest.go` stores whitelisted trusted-upload messages as
  `trust_status="trusted"` when they have no attachments, while attachment
  presence still forces `trust_status="quarantined"`.
- `internal/maild/ingest.go` also extracts `authentication-results` and
  `arc-authentication-results` headers into the dedicated
  `authentication_results_json` column.
- `internal/maild/store.go` carries `authentication_results_json` through
  owner-scoped message reads for tests/operator inspection without changing the
  browser message-summary API.
- Focused verification passed:

```text
nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
```

Deployment evidence, 2026-05-27:

- Commit `d9512abd205d92ed2646ca1cf0541eb82cc9e89e` was pushed to `main`.
- GitHub Actions run `26531792060` completed successfully after Go vet/build,
  non-runtime tests, runtime shards 0-3, integration smoke, aggregate Go gate,
  and Deploy to Staging.
- Node B `/opt/go-choir` reports HEAD
  `d9512abd205d92ed2646ca1cf0541eb82cc9e89e` and clean
  `## main...origin/main` status.
- Public `https://choir.news/health` reports proxy/sandbox
  `deployed_commit=d9512abd205d92ed2646ca1cf0541eb82cc9e89e`.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `root_owner_id_configured=true`, `messages=0`, `webhook_events=0`, and
  `ingress_events=0`.
- Fresh `scripts/mail-provider-readiness` still reports Resend domains/webhooks
  `401 restricted_api_key`, missing `RESEND_WEBHOOK_SECRET`, Gandi root MX still
  `spool.mail.gandi.net` / `fb.mail.gandi.net`, Gandi SPF still active, and no
  `_dmarc` record.

Belief-state update:

- The local trusted-upload evidence gap is closed and deployed.
- The mission remains `checkpoint_incomplete` because the real provider path is
  still externally blocked on Resend authority, webhook signing secret, exact
  DNS records, and MX cutover.

## Local Finding: proxy should enforce email source-packet trust label

Recorded: 2026-05-27 during the next receding-horizon audit while provider
readiness remained blocked on Resend authority.

Problem:

The proxy-owned Send to Choir handoff gets a source packet from `maild`, builds
the prompt-bar payload, and submits it to the user's sandbox under owner
authority. `maild` currently emits `UNTRUSTED_EXTERNAL_EMAIL`, and the prompt
body explicitly repeats that external email is source material rather than
instruction. But the proxy accepts any non-empty trust label returned by
`maild`; it only defaults a blank label to `UNTRUSTED_EXTERNAL_EMAIL`.

This is not a current exploit with the existing `maild` source-packet writer,
but it weakens the boundary that matters most for the mission. The component
that turns email into MAS input should enforce the email ingress trust label,
not merely describe it in prose.

Evidence:

```text
internal/proxy/email.go:
  fetchMailSourcePacket defaults an empty TrustLabel to
  UNTRUSTED_EXTERNAL_EMAIL, but does not reject unexpected non-empty labels.

internal/proxy/email_test.go:
  TestEmailSendToChoirFetchesSourcePacketAndSubmitsPromptBar proves normal
  source-packet handoff, but no test proves the proxy rejects a source packet
  with a different trust label before prompt-bar submission.
```

Belief-state update:

- `maild` still cannot directly call agents or mutate canonical state.
- The prompt text still contains the correct untrusted-email warning.
- The proxy should add a hard gate so future maild changes or corrupt source
  packets cannot silently alter the trust frame used for MAS handoff.

Next executable probe:

- Make `fetchMailSourcePacket` reject any source packet whose trust label is not
  exactly `UNTRUSTED_EXTERNAL_EMAIL`, and add a focused proxy test that verifies
  no prompt-bar submission happens for an unexpected label.

Resolution checkpoint, 2026-05-27:

- `internal/proxy/email.go` now treats `UNTRUSTED_EXTERNAL_EMAIL` as a hard
  source-packet contract for Send to Choir. A source packet with a blank or
  unexpected trust label fails before any prompt-bar submission.
- `internal/proxy/email_test.go` now proves that an unexpected source-packet
  trust label returns a gateway error and does not call the sandbox prompt-bar
  endpoint.
- Focused verification passed:

```text
nix develop -c go test ./internal/proxy ./internal/maild ./cmd/maild ./cmd/maildctl
```

Deployment evidence, 2026-05-27:

- Commit `15af3a5c9955f113110719ac5ca9ab39a9175473` was pushed to `main`.
- GitHub Actions run `26532343049` completed successfully after Go vet/build,
  non-runtime tests, runtime shards 0-3, integration smoke, aggregate Go gate,
  and Deploy to Staging.
- Node B `/opt/go-choir` reports HEAD
  `15af3a5c9955f113110719ac5ca9ab39a9175473` and clean
  `## main...origin/main` status.
- Public `https://choir.news/health` reports proxy/sandbox
  `deployed_commit=15af3a5c9955f113110719ac5ca9ab39a9175473`.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `root_owner_id_configured=true`, `messages=0`, `webhook_events=0`, and
  `ingress_events=0`.
- Fresh `scripts/mail-provider-readiness` still reports Resend domains/webhooks
  `401 restricted_api_key`, missing `RESEND_WEBHOOK_SECRET`, Gandi root MX still
  `spool.mail.gandi.net` / `fb.mail.gandi.net`, Gandi SPF still active, and no
  `_dmarc` record.

Belief-state update:

- The local proxy trust-label handoff gap is closed and deployed.
- The mission remains `checkpoint_incomplete` because real inbound/outbound
  provider acceptance still waits on Resend authority, webhook signing secret,
  exact DNS records, and MX cutover.

## Local Finding: acceptance checker does not correlate message to webhook receipt

Recorded: 2026-05-27 while provider readiness was still blocked on Resend
authority.

Problem:

The real inbound acceptance test must prove both that the Resend webhook fired
and that the selected mailbox message came from that provider event. The current
`scripts/mail-acceptance-check` prints recent webhook receipts, but it does not
prove that the matching message is linked to one of them. `maild` stores
`provider_message_id` and `provider_event_id` on `email_messages`, but
operator-visible message reads do not expose those ids, so the checker cannot
assert correlation.

Evidence:

```text
internal/maild/store.go:
  EmailMessage does not carry provider, provider_message_id, or
  provider_event_id despite email_messages storing those columns.

cmd/maildctl/main.go:
  maildctl message returns EmailMessage, so the provider ids are unavailable to
  real acceptance runs.

scripts/mail-acceptance-check:
  fetches maildctl webhooks --limit 5 and includes them in output, but does not
  require a webhook row that matches the selected message.
```

Belief-state update:

- The service stores the needed linkage, but the proof tool is too weak.
- Strengthening this does not require provider mutation or DNS changes.
- The acceptance checker should fail closed if a selected inbound message lacks
  provider ids or matching webhook evidence.

Next executable probe:

- Carry provider ids through `EmailMessage`, make `mail-acceptance-check` query
  enough webhook receipts and require a match for the selected message, and add
  focused tests for the operator-visible fields.

Resolution checkpoint, 2026-05-27:

- `internal/maild/store.go` now carries `provider`, `provider_message_id`, and
  `provider_event_id` through owner/operator-visible message reads.
- `cmd/maildctl message` now prints those provider ids through its existing
  JSON detail output, so real acceptance can correlate a selected message to
  the provider event that delivered it.
- `scripts/mail-acceptance-check` now reads up to 100 recent webhook receipts
  and fails unless the selected inbox/quarantine message has provider ids and a
  matching webhook receipt.
- Focused verification passed:

```text
bash -n scripts/mail-acceptance-check
nix develop -c go test ./internal/maild ./cmd/maildctl ./cmd/maild ./internal/proxy
```

Deployment evidence, 2026-05-27:

- Commit `2c19e49e9e7d0000440267289c6e8f1f04f796a8` was pushed to `main`.
- GitHub Actions run `26532784445` completed successfully after Go vet/build,
  non-runtime tests, runtime shards 0-3, integration smoke, frontend build,
  aggregate Go gate, and Deploy to Staging.
- Node B `/opt/go-choir` reports HEAD
  `2c19e49e9e7d0000440267289c6e8f1f04f796a8` and clean
  `## main...origin/main` status.
- Public `https://choir.news/health` reports proxy/sandbox
  `deployed_commit=2c19e49e9e7d0000440267289c6e8f1f04f796a8`.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `root_owner_id_configured=true`, `messages=0`, `webhook_events=0`, and
  `ingress_events=0`.
- Fresh `scripts/mail-provider-readiness` still reports Resend domains/webhooks
  `401 restricted_api_key`, missing `RESEND_WEBHOOK_SECRET`, Gandi root MX still
  `spool.mail.gandi.net` / `fb.mail.gandi.net`, Gandi SPF still active, and no
  `_dmarc` record.

Belief-state update:

- The local webhook-correlation evidence gap is closed and deployed.
- The mission remains `checkpoint_incomplete` because real inbound/outbound
  provider acceptance still waits on Resend authority, webhook signing secret,
  exact DNS records, and MX cutover.

## Local Finding: outbound From is validated but not canonicalized

Recorded: 2026-05-27 while provider readiness was still blocked on Resend
authority.

Problem:

The Email app v0 intentionally presents a fixed numeric From address:
`000@choir.news`. `maild` correctly validates that the requested from address
resolves to an alias owned by the authenticated user, but after validation the
outbound send payload and Sent row keep the caller-supplied `from_address`
string. A caller could send a display-name variant such as
`Founder <000@choir.news>` through the authenticated API. That still resolves to
the owned numeric alias, but it weakens the product invariant that v0 sends from
the canonical numeric endpoint rather than caller-shaped identity text.

Evidence:

```text
docs/choir-email-reference-v0.md:
  Compose/reply: From alias selector initially fixed to 000@choir.news.

internal/maild/send.go:
  resolveOwnedFromAlias validates the address by resolving the local/domain to
  an owned alias.
  buildResendSendRequest then uses strings.TrimSpace(in.FromAddress) as the
  Resend From value.
  StoreOutboundMessage stores strings.TrimSpace(in.FromAddress) in the Sent row.
```

Belief-state update:

- This does not allow sending from an unowned local part, because alias
  ownership validation still gates the send.
- The v0 outbound identity should still be canonicalized after validation so
  provider payloads and stored Sent evidence match the numeric alias exactly.

Next executable probe:

- Canonicalize outbound From from the resolved alias for the Resend payload and
  Sent storage, and add a focused test proving display-name input still sends
  and stores `000@choir.news`.

Resolution checkpoint, 2026-05-27:

- `internal/maild/send.go` now derives the outbound Resend `From` value from
  the resolved alias after ownership validation instead of preserving the
  caller-supplied address string.
- Sent rows now store the same canonical alias address.
- `internal/maild/send_test.go` proves that `Founder <000@choir.news>` input is
  accepted only as the owned alias and is sent/stored as `000@choir.news`.
- Focused verification passed:

```text
nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
```

Deployment evidence, 2026-05-27:

- Commit `d2460c41a74d2d13132bb7b38059d017b97a0db4` was pushed to `main`.
- GitHub Actions run `26533204422` completed successfully after Go vet/build,
  non-runtime tests, runtime shards 0-3, integration smoke, aggregate Go gate,
  and Deploy to Staging.
- Node B `/opt/go-choir` reports HEAD
  `d2460c41a74d2d13132bb7b38059d017b97a0db4` and clean
  `## main...origin/main` status.
- Public `https://choir.news/health` reports proxy/sandbox
  `deployed_commit=d2460c41a74d2d13132bb7b38059d017b97a0db4`.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `root_owner_id_configured=true`, `messages=0`, `webhook_events=0`, and
  `ingress_events=0`.
- Fresh `scripts/mail-provider-readiness` still reports Resend domains/webhooks
  `401 restricted_api_key`, missing `RESEND_WEBHOOK_SECRET`, Gandi root MX still
  `spool.mail.gandi.net` / `fb.mail.gandi.net`, Gandi SPF still active, and no
  `_dmarc` record.

Belief-state update:

- The local outbound From canonicalization gap is closed and deployed.
- The mission remains `checkpoint_incomplete` because real inbound/outbound
  provider acceptance still waits on Resend authority, webhook signing secret,
  exact DNS records, and MX cutover.

## Local Finding: no-auto-outbound proof needs an explicit checker gate

Recorded: 2026-05-27 while provider readiness remained blocked on Resend
authority.

Problem:

The final acceptance requires proof that inbound email cannot trigger outbound
send, tool execution, canonical mutation, or promotion automatically. The
current `scripts/mail-acceptance-check` can prove no owner handoff happened yet
for a selected message by requiring `--expect-ingress-events 0`, and it can
prove the later explicit Send to Choir handoff with `--expect-ingress-events 1`.
It does not directly prove that no outbound Sent row appeared at or after the
selected inbound message arrived.

Evidence:

```text
scripts/mail-acceptance-check:
  supports --expect-ingress-events N
  does not inspect the Sent folder

docs/choir-email-reference-v0.md:
  acceptance says no inbound message can trigger outbound send, tool execution,
  canonical state mutation, or promotion.
```

Belief-state update:

- The code structure already separates inbound storage from outbound send.
- The real provider acceptance should still include a read-only proof that the
  selected inbound message did not create a same-owner Sent message before any
  owner action.
- This should be opt-in because a real owner mailbox may legitimately contain
  unrelated Sent rows.

Next executable probe:

- Add an opt-in `scripts/mail-acceptance-check` flag that lists Sent messages
  for the owner and fails if any Sent row is timestamped at or after the
  selected inbound message timestamp.

Resolution checkpoint, 2026-05-27:

- `scripts/mail-acceptance-check` now supports
  `--expect-no-sent-after-message`.
- The flag lists the owner's Sent folder and fails if any Sent row is
  timestamped at or after the selected inbound message's received/created
  timestamp.
- The checker remains read-only and opt-in, so ordinary mailbox inspection does
  not fail because of unrelated historical Sent rows.
- Focused verification passed:

```text
bash -n scripts/mail-acceptance-check
nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
```

Deployment evidence, 2026-05-27:

- Commit `e86b16737febeeae5339886ac70fd476cf9db5b1` was pushed to `main`.
- GitHub Actions run `26533605755` completed successfully, including the
  Deploy to Staging job.
- Node B `/opt/go-choir` reports HEAD
  `e86b16737febeeae5339886ac70fd476cf9db5b1` and clean
  `## main...origin/main` status.
- Public `https://choir.news/health` reports proxy/sandbox
  `deployed_commit=e86b16737febeeae5339886ac70fd476cf9db5b1` and
  `deployed_at=2026-05-27T19:26:31Z`.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `root_owner_id_configured=true`, `aliases=1`, `messages=0`,
  `quarantined_attachments=0`, `webhook_events=0`, and `ingress_events=0`.
- Fresh `scripts/mail-provider-readiness` still reports Resend domains/webhooks
  `401 restricted_api_key`, missing `RESEND_WEBHOOK_SECRET`, Gandi root MX still
  `spool.mail.gandi.net` / `fb.mail.gandi.net`, Gandi SPF still active, and no
  `_dmarc` record.

Belief-state update:

- The no-auto-outbound checker gap is closed and deployed.
- The mission remains `checkpoint_incomplete` because real inbound/outbound
  provider acceptance still waits on Resend authority, webhook signing secret,
  exact DNS records, and MX cutover.

## Local Finding: webhook ingest failures currently acknowledge too early

Recorded: 2026-05-27 while auditing the webhook path before real Resend
receiving.

Problem:

`HandleResendWebhook` durably records the verified Resend webhook event, then
tries to fetch and store the received email in the same request. If that fetch
or store path fails, the handler currently returns HTTP 202 with
`accepted_ingest_failed`. A 2xx response tells Resend the webhook delivery
succeeded, so the provider may not retry. That weakens the whole duplicate
retry repair path: it can repair a duplicate delivery if one arrives, but the
first transient failure no longer asks the provider for a duplicate delivery.

Evidence:

```text
internal/maild/webhook.go:
  RecordWebhookEvent succeeds first.
  ingestReceivedEmail errors are logged.
  status becomes accepted_ingest_failed.
  response remains http.StatusAccepted.

internal/maild/webhook_test.go:
  TestHandleResendWebhookDuplicateRetriesMissingInboundMessage currently expects
  the first transient provider failure to return 202 accepted_ingest_failed.

docs/choir-email-reference-v0.md:
  STRIDE denial-of-service note calls for fast webhook ack after durable enqueue
  and bounded workers; the current v0 does not have a durable background queue.
```

Belief-state update:

- Until a durable background ingest worker exists, a transient fetch/store
  failure after webhook event storage should return a retryable non-2xx status.
- Policy rejects, unknown aliases, or other permanent classification failures
  should still avoid retry loops, but provider HTTP/server failures and local
  store failures need Resend retry pressure.

Next executable probe:

- Change `HandleResendWebhook` so retryable ingest failures return a non-2xx
  status after recording the webhook event, while duplicate deliveries can still
  retry missing message storage idempotently.
- Keep response bodies and logs free of email body, webhook secret, API key, and
  attachment URL data.

Resolution checkpoint, 2026-05-27:

- `HandleResendWebhook` now returns HTTP 503 with
  `ingest_retry_requested` when a newly recorded `email.received` event hits a
  retryable fetch/store failure.
- Duplicate webhook deliveries still retry missing message storage. If the
  duplicate retry also hits a retryable failure, `maild` returns HTTP 503 with
  `duplicate_ingest_retry_requested` instead of acknowledging success.
- Permanent local rejects such as receive-policy rejection or no matching alias
  still return an accepted failure status to avoid provider retry loops.
- Focused verification passed:

```text
nix develop -c go test ./internal/maild -run 'TestHandleResendWebhook(DuplicateRetriesMissingInboundMessage|DuplicateRetryFailureRequestsAnotherRetry|RejectsUnwhitelistedTrustedUploadAlias|FetchesAndStoresInboundMessage)'
```

Deployment evidence, 2026-05-27:

- Commit `a22d075c54234b4bb36f04d5015ed0616d9bb954` was pushed to `main`.
- GitHub Actions run `26534436593` completed successfully, including Deploy to
  Staging job `78159684073`.
- Node B `/opt/go-choir` reports HEAD
  `a22d075c54234b4bb36f04d5015ed0616d9bb954` and clean
  `## main...origin/main` status.
- Public `https://choir.news/health` reports proxy/sandbox
  `deployed_commit=a22d075c54234b4bb36f04d5015ed0616d9bb954` and
  `deployed_at=2026-05-27T19:42:41Z`.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `root_owner_id_configured=true`, `aliases=1`, `messages=0`,
  `quarantined_attachments=0`, `webhook_events=0`, and `ingress_events=0`.
- Fresh `scripts/mail-provider-readiness` still reports Resend domains/webhooks
  `401 restricted_api_key`, missing `RESEND_WEBHOOK_SECRET`, Gandi root MX still
  `spool.mail.gandi.net` / `fb.mail.gandi.net`, Gandi SPF still active, and no
  `_dmarc` record.

Belief-state update:

- The retryable webhook-ingest acknowledgement gap is closed and deployed.
- The mission remains `checkpoint_incomplete` because real inbound/outbound
  provider acceptance still waits on Resend authority, webhook signing secret,
  exact DNS records, and MX cutover.

## Mission Ledger Reconciliation Checkpoint

Recorded: 2026-05-26.

Status:

Historical "Required next change" findings for private mail state modes,
default alias owner reconciliation, direct `maild` health proof,
`workflow_dispatch`, `maildctl`, empty-list CLI output, bounded provider
logging, owner reply threading, stored recipients, and minimal Compose have
explicit resolution checkpoints. This does not change deployed behavior; it
makes the mission ledger match the already-shipped service and keeps future
resumptions focused on the real remaining provider/DNS acceptance gap.

Verification:

```text
nix develop -c go test ./internal/maild ./cmd/maildctl ./cmd/maild
npm run build in frontend
nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-maild.serviceConfig.UMask
  -> "0077"
nix eval .#nixosConfigurations.go-choir-b.config.environment.systemPackages --apply 'pkgs: builtins.any (p: (p.pname or "") == "maildctl") pkgs'
  -> true
rg -n '^  workflow_dispatch:|force_staging_deploy|for port in 8081 8082 8083 8084 8086 8087' .github/workflows/ci.yml
  -> workflow_dispatch, force_staging_deploy, and 8087 smoke probe present
```

Belief-state update:

- No new behavior gap was found in these repo-local surfaces.
- The highest-impact remaining uncertainty is still external Resend
  domain/webhook truth plus webhook signing secret; DNS/MX must remain
  unchanged until that evidence is available.

## Evidence Finding: receive policy fields are not enforced on inbound ingest

Recorded: 2026-05-26.

Problem:

The reference model describes Choir Email as policy-gated ingress. The SQLite
schema already includes `email_receive_policies` fields such as
`allow_public_inbound`, `require_sender_whitelist`, `require_secret_alias`,
`allow_auto_agent_read`, `allow_auto_agent_write`,
`allow_auto_outbound_send`, and `quarantine_by_default`. However, the current
webhook ingest path resolves an alias and immediately stores the message. It
does not load the alias's receive policy, check a sender whitelist, or fail
closed for future non-public aliases.

Evidence:

```text
reference: docs/choir-email-reference-v0.md says maild owns alias and receive/send policy enforcement
schema: internal/maild/store.go creates email_receive_policies and aliases reference receive_policy_id
seed: DefaultPublicPolicyID has require_sender_whitelist=0 and quarantine_by_default=1
ingest path: internal/maild/webhook.go ingestReceivedEmail resolves alias then calls StoreInboundMessage
missing path: no GetReceivePolicy, no email_sender_whitelist table, no sender whitelist check
current tests: webhook tests prove public alias ingest/quarantine, but not deny/allow policy behavior
```

Belief-state update:

- The default public `000@choir.news` path is still acceptable for v0 public
  inbound, but the service boundary is weaker than the reference once trusted
  upload or private aliases are introduced.
- This can be fixed locally without provider access: add receive-policy loading
  and fail-closed enforcement before `StoreInboundMessage`.

Required next change:

- Add a read-side receive-policy model, create `email_sender_whitelist`, enforce
  public/whitelist/secret-alias gates before storing inbound messages, preserve
  quarantine-by-default behavior for attachments, and add focused tests proving
  unwhitelisted trusted-upload-style aliases are not stored.

Resolution checkpoint:

- Implemented in `internal/maild`: `email_sender_whitelist` is part of the
  schema, `Store.GetReceivePolicy` loads policy booleans,
  `Store.IsSenderWhitelisted` checks owner/alias/sender rows, and the Resend
  webhook ingest path enforces public, whitelist, secret-alias, and attachment
  gates before calling `StoreInboundMessage`.
- Focused tests prove a trusted-upload-style exact plus alias rejects an
  unwhitelisted sender without storing a message, while the same alias accepts
  a whitelisted sender.

Verification:

```text
nix develop -c go test ./internal/maild
nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
GitHub Actions CI run 26451498114:
  Go Vet + Build: success
  Go Test non-runtime: success
  runtime shards 0-3: success
  integration smoke: success
  Go Vet + Test + Build: success
  Deploy to Staging (Node B): success
public /health:
  proxy deployed_commit: 7de363e05cfb102fcfec44303955b3c525870711
  sandbox deployed_commit: 7de363e05cfb102fcfec44303955b3c525870711
  deployed_at: 2026-05-26T13:39:17Z
webhook route:
  GET https://choir.news/api/email/resend/webhook -> 405 method not allowed
  unsigned POST -> 503 {"status":"webhook_secret_not_configured"}
scripts/mail-provider-readiness:
  Resend Domains/Webhooks: 401 restricted_api_key
  Gandi MX/SPF/DKIM: still Gandi defaults
  Node B maild health: status ok, webhook_secret_configured false, messages 0
```

Belief-state update:

- `maild` now matches the policy-gated ingress shape more closely before real
  provider traffic arrives.
- The remaining blocker is still external provider/DNS proof: Resend domain and
  webhook truth plus `RESEND_WEBHOOK_SECRET` are required before root MX
  mutation and real inbound acceptance.

## Evidence Finding: duplicate webhook path does not retry failed ingest

Recorded: 2026-05-26.

Problem:

`maild` records a verified Resend webhook event before fetching and storing the
received email. If the first `email.received` handling records the webhook but
fails during provider fetch, alias resolution, receive-policy enforcement, or
message storage, later delivery of the same Resend event currently returns
`duplicate` immediately. That prevents a transient provider/API/store failure
from being repaired by Resend retrying the webhook.

Evidence:

```text
internal/maild/webhook.go:
  HandleResendWebhook -> RecordWebhookEvent(...)
  if !created { return duplicate }
  ingestReceivedEmail(...) may then fail with status accepted_ingest_failed
internal/maild/store.go:
  email_messages rows are keyed from provider_message_id, and StoreInboundMessage uses INSERT OR IGNORE
current tests:
  TestHandleResendWebhookStoresVerifiedEventIdempotently proves duplicate event count stays 1
missing test:
  duplicate email.received after first ingest failure retries storing the message
```

Belief-state update:

- Webhook idempotency is present, but retry semantics are too brittle for real
  provider traffic.
- A safe local fix is possible because inbound storage is already idempotent by
  provider message id. The duplicate path can retry ingest for `email.received`
  only when the message is not already stored.

Required next change:

- Add a store read to detect whether a provider message id is already stored,
  retry duplicate `email.received` ingest when it is missing, and add a focused
  test where the first provider fetch fails and the duplicate delivery stores
  the message.

Resolution checkpoint:

- Implemented in `internal/maild`: duplicate `email.received` webhooks now call
  `retryMissingReceivedEmail`, which checks `Store.HasProviderMessage` before
  retrying provider fetch and inbound storage.
- Already-stored provider message ids continue to return `duplicate` without
  refetching. Missing messages can be repaired by a duplicate provider delivery,
  and `StoreInboundMessage` remains idempotent by provider message id.
- Focused coverage exists in
  `TestHandleResendWebhookDuplicateRetriesMissingInboundMessage`.

Verification:

```text
nix develop -c go test ./internal/maild
nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy
```

Belief-state update:

- The webhook path is more realistic for provider retries: a transient first
  fetch/store failure no longer permanently strands the received message behind
  a duplicate event receipt.
- This still is not real Resend proof. Final acceptance still needs the Resend
  webhook signing secret, exact provider records, DNS cutover, and live inbound
  messages.

## Evidence Finding: Send to Choir hands MAS metadata, not the actual email source

Recorded: 2026-05-26.

Problem:

The current proxy-owned Send to Choir path is traceable, but it does not yet
hand the MAS a usable email source. `maild` stores an `email_source_packets`
row for each inbound message, but the stored `text_ref` is `NULL`. The
authenticated source-packet API returns only packet id, message id, trust
label, sender, subject, and snippet. Proxy then builds the prompt-bar payload
from that metadata. There is no runtime path that can dereference
`source_packet_id` or `text_ref` into the normalized email body.

That means the current handoff can tell Choir that an email exists and name its
trust label, but it cannot reliably let Choir read the actual inbound source
material beyond a short snippet. This falls short of "email is another ingress
point into the MAS path" even though the ownership boundary and ingress-event
receipt are correct.

Evidence:

```text
source packet schema:
  internal/maild/store.go: EmailSourcePacket has TextRef string
ingest path:
  internal/maild/ingest.go inserts email_source_packets with text_ref = NULL
authenticated source-packet API:
  internal/maild/api.go sourcePacketResponse exposes source_packet_id,
  message_id, trust_label, from_address, subject, snippet only
proxy handoff:
  internal/proxy/email.go buildEmailSourcePrompt uses source packet id,
  message id, trust label, from, subject, snippet
runtime usage search:
  no internal/runtime consumer dereferences source_packet_id or email source
  text_ref
tests:
  internal/proxy/email_test.go asserts the prompt contains
  UNTRUSTED_EXTERNAL_EMAIL, source packet id, message id, and subject, but not
  the normalized email body
reference contract:
  docs/choir-email-reference-v0.md says proxy asks maild for an owner-visible
  source packet, then sandbox/conductor receives owner instruction plus
  untrusted source refs
```

Belief-state update:

- The current Send to Choir path preserves the security boundary: maild still
  cannot call agents or mutate canonical state, and the owner-triggered ingress
  receipt is still correct.
- But the MAS handoff is only partially real. It is an owner-triggered metadata
  handoff, not yet a usable handoff of the normalized untrusted email source.

Resolution evidence, 2026-05-26:

- Commit `8c0348c` extends the authenticated source-packet response with
  provenance JSON, normalized `text_ref`, normalized plain-text body, and
  attachment presence.
- `internal/maild/ingest.go` now stores `text_ref = message:<message-id>` for
  real inbound source packets instead of `NULL`.
- `internal/proxy/email.go` now builds the prompt-bar payload from provenance,
  `text_ref`, and a bounded copy of the normalized plain-text email body under
  explicit `UNTRUSTED_EXTERNAL_EMAIL` framing.
- Focused tests cover the expanded source-packet API, real-ingest `text_ref`,
  prompt-body inclusion, and bounded truncation behavior.
- GitHub Actions run `26461534834` deployed `8c0348c` to Node B, and public
  health now reports proxy/sandbox `deployed_commit =
  8c0348cf15f2c7d555dbaf549ebc929f98a50bff`.

Residual risk:

- This closes the local metadata-only handoff gap, but the mission still lacks
  a real received-email -> Send to Choir staging proof because Resend domain,
  webhook-secret, and Gandi MX/provider access remain blocked.

## Platform Finding: Node B Deploy Identity Is Tainted By Dirty Source State

Recorded: 2026-05-27.

Problem:

Current staging cannot be used as acceptance evidence for the maild mission,
even though `go-choir-maild` is active and healthy. Node B now reports the
latest deployed commit as the later Search Provider Plane commit `a1040f7`, but
the `/opt/go-choir` checkout is not actually a clean checkout of that commit.
It is a dirty tree at `8c0348c` with tracked search-plane/runtime files modified
and untracked source files at the repository root. The dirty files are owned by
uid `501`, while `.git` is owned by root, which is the same ownership mismatch
that produced the earlier `safe.directory` deploy failure.

Evidence:

```text
public /health on 2026-05-27:
  proxy deployed_commit: a1040f7582b5e08c75524a26c285bb5ab1d08738
  sandbox deployed_commit: a1040f7582b5e08c75524a26c285bb5ab1d08738
  deployed_at: 2026-05-26T17:30:48Z

node-b /var/lib/go-choir/deploy.env:
  CHOIR_DEPLOYED_AT=2026-05-26T17:30:48Z
  CHOIR_DEPLOYED_COMMIT=a1040f7582b5e08c75524a26c285bb5ab1d08738

node-b maild health:
  status ok
  messages 0
  webhook_events 0
  ingress_events 0
  resend_api_key_configured true
  webhook_secret_configured false

node-b /opt/go-choir:
  git rev-parse HEAD -> 8c0348cf15f2c7d555dbaf549ebc929f98a50bff
  tracked modifications:
    internal/gateway/handlers.go
    internal/gateway/search.go
    internal/gateway/search_test.go
    internal/runtime/search_gateway.go
    internal/runtime/tools_research.go
  untracked source files:
    config.go errors.go health.go merge.go policy.go router.go
    search_gateway.go sqlite_health.go tools_research.go types.go
    internal/gateway/search_plane.go
    internal/gateway/searchplane/
  ownership:
    /opt/go-choir owned by uid 501 group lp
    /opt/go-choir/.git owned by root root
```

Belief-state update:

- Do not configure Resend/Gandi or mutate DNS from this staging state.
- The next executable platform repair is a GitHub Actions deploy-path fix that
  makes Node B checkout cleanup remove untracked source files and restores
  deploy identity from a clean `origin/main` checkout.
- `git clean -ffdX` is insufficient here because the dangerous extra files are
  untracked source files, not only ignored build artifacts.
- Acceptance can resume only after a GitHub Actions deploy proves:
  `deploy.env`, public `/health`, and `/opt/go-choir` all agree on the same
  clean commit, and `go-choir-maild` remains healthy.

Resolution checkpoint, 2026-05-27:

- Commit `38ff631142bd13b02c59482007f9b68381eed811` changed the GitHub Actions
  Node B checkout step from `git clean -ffdX` to `git clean -ffdx` and added a
  dirty-worktree fail-fast check after reset/clean.
- Push CI run `26529628062` passed Go vet/build, non-runtime tests, runtime
  shards 0-3, integration smoke, and aggregate Go gate. The deploy job skipped
  because only workflow/docs changed.
- Forced GitHub Actions run `26529727154` passed Go vet/build, non-runtime
  tests, runtime shards 0-3, integration smoke, frontend build, aggregate Go
  gate, and Deploy to Staging.
- The deploy log shows the stale untracked root source files removed:
  `config.go`, `errors.go`, `health.go`, `merge.go`, `policy.go`,
  `policy_test.go`, `router.go`, `router_test.go`, `search_gateway.go`,
  `sqlite_health.go`, `sqlite_health_test.go`, `tools_research.go`, and
  `types.go`.
- Public `/health`, `/var/lib/go-choir/deploy.env`, and
  `git -C /opt/go-choir rev-parse HEAD` all report
  `38ff631142bd13b02c59482007f9b68381eed811`.
- `git -C /opt/go-choir status --short --branch` reports
  `## main...origin/main` with no dirty files.
- Node B `maild` health remains ok:
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `messages=0`, `webhook_events=0`, `ingress_events=0`.

Belief-state update:

- The deploy identity blocker is resolved for the current staging state.
- The next mission uncertainty returns to provider truth: Resend domain/webhook
  visibility, webhook signing secret deployment, Gandi DNS/MX planning, and
  real inbound acceptance.

## Local Finding: Send to Choir ingress receipts are not idempotent

Recorded: 2026-05-27 while auditing the owner-triggered source-packet MAS
handoff before provider setup.

Problem:

The proxy-owned Send to Choir path submits a prompt-bar request and then asks
`maild` to record an `email_ingress_events` receipt that links the email source
packet to the conductor submission id. The receipt id is deterministic from
`message_id + conductor_submission_id`, but `Store.RecordIngressEvent` uses a
plain `INSERT`. If the same receipt is posted twice after a response loss,
proxy retry, or repeated internal callback, `maild` returns a primary-key error
instead of treating the already-recorded same receipt as success. That weakens
the final acceptance proof because the source-packet handoff ledger is less
retryable than the provider webhook ledger.

Evidence:

```text
internal/maild/ingest.go:
  ingressEventRowID(messageID, submissionID) derives a stable row id.

internal/maild/store.go:
  RecordIngressEvent executes INSERT INTO email_ingress_events without
  conflict handling.

internal/maild/api.go:
  handleRecordMessageIngressEvent returns HTTP 500 when RecordIngressEvent
  reports the duplicate primary-key error.

internal/maild/api_test.go:
  TestHandleMessageIngressEventsRequiresInternalCaller proves the happy path
  and internal-caller guard, but does not prove duplicate receipt idempotency.
```

Belief-state update:

- Ingress receipt recording should be idempotent for the same
  message/source/owner/submission tuple.
- A conflicting row with the same deterministic id but different owner,
  message, source packet, or submission should still fail closed.
- This is a local ledger reliability repair; it does not authorize inbound
  email to call agents, mutate canonical state, or send outbound mail.

Next executable probe:

- Make `RecordIngressEvent` tolerate exact duplicate receipts while rejecting
  conflicting duplicates, add a focused API/store test, then run the maild and
  proxy test slice before deploy.

Resolution evidence, 2026-05-27:

- `internal/maild/store.go` now records ingress receipts with
  `INSERT OR IGNORE`, then verifies an ignored duplicate matches the same
  message id, source packet id, owner id, and conductor submission id before
  accepting it as idempotent.
- `internal/maild/api_test.go` now posts the same internal
  `/api/email/messages/:id/ingress-events` receipt twice and proves both
  requests return `202 Accepted` while only one durable ingress event remains.
- Focused local verification passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.

Residual risk:

- This closes the local duplicate receipt gap, but it is not deployed until the
  next GitHub Actions deploy identity is verified on Node B. Real inbound
  source-packet handoff proof still depends on Resend webhook setup and DNS/MX
  cutover.

Deploy evidence, 2026-05-27:

- Commit `54b487d502883497567e8047d2f5f3ede27f72b2` was pushed to `origin/main`
  after the doc-only problem checkpoint `6f2f4cf`.
- GitHub Actions run `26536263739` completed successfully for `54b487d`; the
  Deploy to Staging job `78166049491` completed successfully.
- Node B `/var/lib/go-choir/deploy.env` reports
  `CHOIR_DEPLOYED_COMMIT=54b487d502883497567e8047d2f5f3ede27f72b2`.
- `git -C /opt/go-choir rev-parse HEAD` reports
  `54b487d502883497567e8047d2f5f3ede27f72b2`, and
  `git -C /opt/go-choir status --short --branch` reports
  `## main...origin/main` with no dirty files.
- Public `https://choir.news/health` reports proxy and upstream sandbox
  `deployed_commit=54b487d502883497567e8047d2f5f3ede27f72b2`.
- Node B `http://127.0.0.1:8087/health` reports maild `status=ok`,
  `resend_api_key_configured=true`, `webhook_secret_configured=false`,
  `messages=0`, `webhook_events=0`, and `ingress_events=0`.

Belief-state update:

- The local Send to Choir ingress receipt idempotency gap is closed and
  deployed.
- The mission remains `checkpoint_incomplete`; real inbound mail, real
  source-packet MAS handoff, and owner reply still require provider/domain
  setup.

## Local Finding: Send to Choir receipt failures are hidden from the owner

Recorded: 2026-05-27 while continuing the owner-triggered source-packet MAS
handoff audit after provider readiness still showed a restricted Resend key and
missing webhook secret.

Problem:

`internal/proxy.HandleEmailSendToChoir` submits the email source packet to the
prompt-bar path, then calls back into `maild` to record the
`email_ingress_events` receipt. If receipt recording fails, the proxy only logs
the error and still returns a normal `202 Accepted` response with the conductor
submission id. That creates a partial-success state where the owner and UI see
"Sent to Choir", but the final acceptance ledger cannot prove that the source
packet handoff was durably correlated to the message.

Evidence:

```text
internal/proxy/email.go:
  HandleEmailSendToChoir logs recordMailIngressEvent errors and continues.

frontend/src/lib/EmailApp.svelte:
  sendToChoir reports success from data.submission_id only.

scripts/mail-acceptance-check:
  final source-packet proof depends on maild ingress_events count for the
  selected message.
```

Belief-state update:

- Prompt submission and ingress receipt recording are two different effects.
  Once prompt submission succeeds, a retry by the owner can create duplicate MAS
  work, so failing the whole request after the prompt-bar call is not obviously
  safe.
- The safer v0 repair is to make receipt recording retryable and visible:
  bounded retry after prompt submission, plus an explicit response field that
  tells the UI whether the durable receipt was recorded.
- The invariant remains unchanged: `maild` does not call MAS and inbound email
  cannot trigger agent/tool/outbound action by itself.

Next executable probe:

- Add bounded proxy retries for `recordMailIngressEvent`, expose
  `ingress_event_recorded` in the Send to Choir response, surface partial
  receipt failure in the Email app status, and add focused proxy tests before
  deploy.

Resolution evidence, 2026-05-27:

- `internal/proxy/email.go` now retries the internal maild ingress receipt
  callback with short bounded delays after a successful prompt-bar submission.
- The Send to Choir response now includes `ingress_event_recorded`; if receipt
  recording remains unavailable after retry, the proxy still returns the
  conductor submission id but marks the receipt as unrecorded instead of hiding
  the partial-success state.
- `frontend/src/lib/EmailApp.svelte` renders that partial-success state as
  `receipt pending` instead of a clean success.
- `internal/proxy/email_test.go` proves the happy path reports a recorded
  receipt, transient receipt failure is retried without duplicate prompt-bar
  submission, and persistent receipt failure is visible in the response.
- Focused local verification passed:
  `nix develop -c go test ./internal/proxy ./internal/maild ./cmd/maild ./cmd/maildctl`.
- Frontend production build passed: `npm run build` in `frontend`.

Deploy evidence, 2026-05-27:

- Problem checkpoint commit: `5a786c7 docs: record mail ingress receipt visibility gap`.
- Behavior commit: `5a6d3cfc8db7e30a714b20736115c3f616a44cef fix: surface mail ingress receipt failures`.
- GitHub Actions CI run `26537198690` completed successfully for `5a6d3cf`;
  Deploy to Staging job `78169289186` completed successfully.
- Node B `/var/lib/go-choir/deploy.env`, `/opt/go-choir` HEAD, public
  `https://choir.news/health`, proxy health, and upstream sandbox health all
  report deployed commit `5a6d3cfc8db7e30a714b20736115c3f616a44cef`.
- Node B maild health reports `status=ok`, `resend_api_key_configured=true`,
  `webhook_secret_configured=false`, `messages=0`, `webhook_events=0`, and
  `ingress_events=0`.
- `scripts/mail-provider-readiness` still reports Resend domain/webhook APIs as
  `401 restricted_api_key`; Gandi LiveDNS still has Gandi MX/SPF/DKIM records
  and no DMARC record. DNS/MX remains intentionally unmodified.

Belief-state update:

- The local Send to Choir handoff is now more observable under partial failure:
  prompt submission success no longer implies a silently recorded ingress
  receipt.
- This does not complete real inbound acceptance. The mission remains
  `checkpoint_incomplete` until Resend domain/webhook setup, webhook secret
  deployment, DNS/MX cutover with rollback, real inbound mail, real quarantine,
  real Send to Choir trace, and no-auto-privileged-action proof are all
  produced.

## Local Finding: Provider readiness lacks deployed webhook fail-closed proof

Recorded: 2026-05-27 while checking provider readiness before any Resend or
Gandi mutation.

Problem:

The mission requires DNS/MX changes to wait for deployed route and rollback
proof. `scripts/mail-provider-readiness` currently reports Resend
domain/webhook API state, Gandi mail records, public DNS, and Node B maild
health, but it does not exercise the public `/api/email/resend/webhook` route.
That leaves a gap in the pre-cutover evidence: we can see that `maild` is
healthy and that the webhook secret is missing, but the standard readiness
artifact does not prove that the public Caddy route reaches maild and fails
closed without recording webhook state.

Evidence:

```text
scripts/mail-provider-readiness:
  reports credentials, Resend, Gandi, DNS, and Node B maild health only.

internal/maild/webhook.go:
  missing RESEND_WEBHOOK_SECRET returns webhook_secret_not_configured before
  storing any webhook event; invalid/missing Svix headers return
  invalid_signature when a secret is configured.
```

Belief-state update:

- A negative webhook probe is safe and useful before provider setup: POSTing an
  unsigned inert JSON body should return either `webhook_secret_not_configured`
  while the secret is absent or `invalid_signature` once the secret is present.
- The readiness probe should make that fail-closed route evidence visible
  without creating domains, webhooks, DNS records, messages, source packets, or
  ingress events.

Next executable probe:

- Extend `scripts/mail-provider-readiness` with a public webhook negative-route
  probe and a follow-up maild health snapshot so pre-DNS readiness reports show
  both route reachability and no message/webhook counter mutation.

Resolution evidence, 2026-05-27:

- `scripts/mail-provider-readiness` now posts an unsigned inert JSON body to
  `https://choir.news/api/email/resend/webhook` and reports the HTTP status,
  redacted JSON body, expected fail-closed statuses, and expected state
  mutation.
- The script then re-reads Node B maild health so the same artifact shows
  whether `messages`, `webhook_events`, or `ingress_events` changed.
- Syntax check passed: `bash -n scripts/mail-provider-readiness`.
- `shellcheck` was not available in the local environment.
- Readiness run against current staging returned:
  - Resend domains/webhooks: `401 restricted_api_key`.
  - public webhook negative probe: HTTP `503`,
    `{"status":"webhook_secret_not_configured"}`.
  - maild health before and after the negative probe:
    `messages=0`, `webhook_events=0`, `ingress_events=0`.
  - Gandi LiveDNS remains on Gandi MX/SPF/DKIM records; DNS/MX was not mutated.

Belief-state update:

- The public webhook route is reachable on `choir.news` and currently fails
  closed while the webhook secret is absent.
- This is still pre-provider evidence only. Real Resend webhook proof still
  requires a sufficiently scoped Resend key, actual webhook secret deployment,
  exact provider records, DNS/MX rollback plan, and a real signed
  `email.received` delivery.

## Staging Finding: maild was not selected for host-service pointer deploy

Recorded: 2026-05-27 while verifying the deployed mailbox identity-boundary
repair from `0408a34`.

Problem:

GitHub Actions reported a successful staging deploy for `0408a34`, and Node B
checkout/deploy identity now agree on that commit, but the running
`go-choir-maild` process did not restart and still serves the older binary.
The deploy-impact classifier selected only `proxy` as a host-service pointer
deploy for the `0408a34` diff, even though that diff changed
`internal/maild/api.go` and `internal/maild/send.go`.

Evidence:

```text
GitHub Actions CI run 26538862723:
  conclusion: success
  deploy job: 78175105408 success
  deploy log HOST_SERVICES: proxy
  deploy log: Fast building Host service proxy: ./cmd/proxy
  deploy log: Restarted go-choir-proxy.service

Node B current identity:
  /var/lib/go-choir/deploy.env:
    CHOIR_DEPLOYED_COMMIT=0408a3421c28dd484e75874a03b51401149fbd60
  /opt/go-choir HEAD:
    0408a3421c28dd484e75874a03b51401149fbd60
  /opt/go-choir status: clean

Node B maild process:
  MainPID=2042302
  ExecMainStartTimestamp=Wed 2026-05-27 20:19:29 UTC
  /proc/$pid/exe:
    /nix/store/y49k4bd87ldlmskwgmnfv7gjn9dcpr16-maild-0.1.0/bin/maild

Direct local maild spoof probe on Node B:
  no X-Authenticated-User header -> HTTP 401 {"error":"authentication required"}
  X-Authenticated-User only -> HTTP 200 {"messages":[]}
  X-Authenticated-User + X-Internal-Caller:false -> HTTP 200 {"messages":[]}
  X-Authenticated-User + X-Internal-Caller:true -> HTTP 200 {"messages":[]}
```

Impact:

- The `0408a34` mailbox identity-boundary repair is committed and CI-green, but
  not actually live in the `maild` service.
- Staging deploy identity is currently too coarse for host-service pointer
  deploys: a clean `/opt/go-choir` checkout and matching `deploy.env` do not
  prove every affected host service was rebuilt and restarted.
- Mail acceptance cannot proceed to Resend/Gandi mutation while a maild
  security repair is only partially deployed.

Belief-state update:

- The immediate blocker is not the maild code fix; it is deploy impact
  classification/service selection. Changes under `cmd/maild`,
  `internal/maild`, maild-facing proxy contracts, or Nix maild service wiring
  must select and restart `maild`.
- Health checks alone are insufficient because the old maild binary remains
  healthy while serving stale behavior.

Next executable probe:

- Fix deploy-impact classification and selected host-service build/restart
  support so maild changes select `maild`, then push through GitHub Actions and
  verify on Node B that:
  - deploy identity remains at the pushed SHA;
  - `go-choir-maild` has a post-deploy start timestamp and new binary path;
  - direct spoofed mailbox requests without `X-Internal-Caller:true` return
    HTTP 403; and
  - proxy-authenticated mailbox routes still work through the intended boundary.

Resolution evidence, 2026-05-27:

- Problem checkpoint commit:
  `b742b50 docs: record maild deploy selection gap`.
- Deploy fix commit:
  `23a74dac07550e97a6354733e01f7168f5423f0f ci: deploy maild host service changes`.
- `.github/scripts/deploy-impact-classify` now maps `cmd/maild/*` and
  `internal/maild/*` to `host_services=maild`; `internal/buildinfo/*` and
  `internal/server/*` include `maild` as an affected host-service pointer.
- `.github/workflows/ci.yml` now supports `maild` in fast host-service builds,
  host-service pointer synchronization after full host deploys, and full host
  service restarts.
- Local verification passed:
  - `bash -n .github/scripts/deploy-impact-classify`
  - deploy-impact smoke for `internal/maild/api.go` and
    `internal/maild/send.go` returned `host_services=maild`
  - metadata-only smoke for workflow/docs changes returned
    `deploy_needed=false`
  - `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`
- Push CI run `26539578580` passed for `23a74da`; it correctly skipped staging
  deploy because only docs/workflow metadata changed in that push.
- GitHub Actions `workflow_dispatch` run `26539695709` with
  `force_staging_deploy=true` passed; Deploy to Staging job `78177986212`
  succeeded.
- The deploy log shows maild in the built/activated closure, including:
  `maild-0.1.0.drv`, `unit-go-choir-maild.service.drv`,
  `stopping ... go-choir-maild.service`, `starting ...
  go-choir-maild.service`, and `Host service pointer updated from NixOS
  closure: maild -> /var/lib/go-choir/services/maild`.
- Node B evidence after deploy:
  - `/var/lib/go-choir/deploy.env`:
    `23a74dac07550e97a6354733e01f7168f5423f0f`
  - `/opt/go-choir` HEAD:
    `23a74dac07550e97a6354733e01f7168f5423f0f`
  - `/opt/go-choir` status: clean
  - `go-choir-maild` MainPID: `2072294`
  - `go-choir-maild` ExecMainStartTimestamp:
    `Wed 2026-05-27 21:28:08 UTC`
  - `/proc/$pid/exe`: `/var/lib/go-choir/services/maild/bin/maild`
- Direct local maild mailbox-route probes now prove the identity-boundary fix
  is live:
  - no owner header -> HTTP 401 `authentication required`
  - `X-Authenticated-User` only -> HTTP 403 `internal caller required`
  - `X-Authenticated-User` + `X-Internal-Caller:false` -> HTTP 403
    `internal caller required`
  - `X-Authenticated-User` + `X-Internal-Caller:true` -> HTTP 200
- Public proxy route probes return HTTP 401 for both no-auth and spoofed
  `X-Authenticated-User`/`X-Internal-Caller:true` headers, preserving the
  browser/session boundary.
- `scripts/mail-provider-readiness` after the deploy still reports:
  - local `RESEND_API_KEY` configured, `RESEND_WEBHOOK_SECRET` missing, and
    `GANDI_PAT` configured;
  - Resend domains/webhooks return `401 restricted_api_key`;
  - Gandi LiveDNS and public DNS remain on Gandi MX/SPF/DKIM and no DMARC;
  - Node B maild health is ok with `webhook_secret_configured=false` and
    `messages=0`, `webhook_events=0`, `ingress_events=0`;
  - public webhook negative probe returns HTTP 503
    `webhook_secret_not_configured` without counter mutation.

Belief-state update:

- The maild host-service deploy-selection gap is resolved for both future
  maild-only changes and the current staging state.
- The mailbox identity-boundary hardening from `0408a34` is now live on Node B.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

## Security Finding: outbound Resend success responses are not size-bounded

Recorded: 2026-05-27 during pre-MX maild hardening.

Problem:

`maild` already caps the public webhook body and the Resend received-email fetch
body before decoding or storing provider-controlled data. The outbound owner-send
path bounds Resend error response details, but a successful Resend `/emails`
response is decoded directly from `resp.Body` with `json.NewDecoder`. That leaves
the success path without the same provider response size boundary.

Evidence:

```text
code:
  internal/maild/resend.go -> retrieveReceivedEmail
    - reads successful provider responses through readProviderResponseBody.

  internal/maild/resend.go -> sendEmail
    - bounds error bodies through readProviderHTTPError;
    - decodes successful provider responses directly from resp.Body.

config:
  MAILD_PROVIDER_MAX_BYTES defaults to 4194304 and is intended to bound provider
  response bodies.
```

Impact:

- This is not an inbound-autonomy or auth bypass. Only owner-authenticated send
  requests reach `sendEmail`.
- It is still an external-provider robustness gap before real outbound proof:
  a malformed or unexpectedly large success response could consume unnecessary
  memory during decode and violate the provider body-cap invariant.

Belief-state update:

- The provider response cap should apply to both inbound fetch and outbound send
  success responses. `MAILD_PROVIDER_MAX_BYTES` should be the single configured
  ceiling for Resend response bodies that maild decodes.

Next executable probe:

- Change `sendEmail` to read successful provider responses through
  `readProviderResponseBody` before JSON decode, and add a focused oversized
  success-response test proving no sent row is accepted from an oversized
  provider response.

Resolution checkpoint:

- Problem checkpoint commit:
  `4d872011eb4c310acb1de465d531339f16c5c7d2 docs: record mail outbound provider response cap gap`.
- Fixed in
  `56ad514435dc10d27eef4fac7c2db08199946045 fix: bound mail outbound provider responses`.
- Code change:
  `sendEmail` now reads successful Resend `/emails` responses through
  `readProviderResponseBody` before JSON decode, sharing the same
  `MAILD_PROVIDER_MAX_BYTES` ceiling used by received-email fetches.
- Local focused tests passed:
  `nix develop -c go test ./internal/maild -run 'Test(HandleSendRejectsOversizedProviderSuccessResponse|ResendSendEmailReturnsBoundedProviderError|HandleSendRequiresOwnedFromAliasAndStoresSentMessage)'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26543202795` passed for `56ad514`; Deploy to Staging
  job `78189403049` succeeded.
- Deploy evidence: deploy impact selected `HOST_SERVICES=maild`, updated
  `maild -> /var/lib/go-choir/services/maild`, restarted
  `go-choir-maild.service`, and reported maild health ok.
- Node B identity: `/var/lib/go-choir/deploy.env` and `/opt/go-choir` both
  report `56ad514435dc10d27eef4fac7c2db08199946045`; `/opt/go-choir` is clean;
  `go-choir-maild` is active/running from
  `/var/lib/go-choir/services/maild/bin/maild`.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- Both Resend provider success decode paths now enforce configured response body
  caps before JSON decode.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

Next executable probe:

- Bound authenticated maild JSON API request bodies before decode, then acquire
  or create a Resend API key/dashboard path with domain/webhook management
  scope; rerun `scripts/mail-provider-readiness`; deploy a real
  `RESEND_WEBHOOK_SECRET` through the credential path; and plan DNS/MX changes
  from exact Resend records before mutating Gandi.

## Security Finding: authenticated maild API request bodies are not size-bounded

Recorded: 2026-05-27 during pre-MX maild hardening.

Problem:

`maild` bounds public Resend webhook bodies with `MAILD_WEBHOOK_MAX_BYTES` and
bounds Resend provider response bodies with `MAILD_PROVIDER_MAX_BYTES`. The
authenticated internal JSON API path still decodes request bodies directly from
`r.Body`:

- `POST /api/email/send`, used by the proxy-authenticated owner compose/reply
  path.
- `POST /api/email/messages/:id/ingress-events`, used by the proxy to record
  Send to Choir receipts after prompt-bar submission.

These routes require `X-Authenticated-User` and `X-Internal-Caller:true` and
are not public webhook routes, so this is not an external unauthenticated bypass.
It is still a resource-boundary gap in a service that is about to receive real
provider and owner traffic: a malformed or oversized authenticated/proxy-forwarded
body can be decoded without a configured API request ceiling.

Evidence:

```text
code:
  internal/maild/api.go -> decodeJSON
    - uses json.NewDecoder(r.Body) with DisallowUnknownFields.
    - no LimitReader or MaxBytesReader.

  internal/maild/send.go -> HandleSend
    - calls decodeJSON for owner-authored outbound mail.

  internal/maild/api.go -> handleRecordMessageIngressEvent
    - calls decodeJSON for Send to Choir receipt recording.

existing bounded paths:
  internal/maild/webhook.go -> HandleResendWebhook
    - reads through io.LimitReader(cfg.WebhookMaxBytes+1).

  internal/maild/resend.go -> retrieveReceivedEmail/sendEmail
    - reads successful provider responses through readProviderResponseBody.
```

Impact:

- This does not let inbound email trigger agent work, outbound send, or
  canonical state mutation by itself.
- It does weaken the service-level resource invariant before real mail routing:
  every maild ingress body that is decoded or stored should have a clear size
  boundary.

Belief-state update:

- Add a distinct `MAILD_API_MAX_BYTES` ceiling for authenticated internal JSON
  API requests rather than reusing the webhook/provider caps. The default should
  be large enough for v0 plain-text compose/reply payloads but explicit enough
  for tests and operators.

Next executable probe:

- Add `Config.APIMaxBytes`, load it from `MAILD_API_MAX_BYTES`, enforce it in
  `decodeJSON` before JSON decode, return HTTP 413 for oversized API bodies,
  and add focused tests for oversized owner-send and ingress-receipt requests
  proving no outbound message or ingress event is recorded.

Resolution checkpoint:

- Problem checkpoint commit:
  `70ca21574e7d8ff87cce1cb7bb0a887d81c5e9fd docs: record maild API body cap gap`.
- Fixed in
  `b68d21c04846e9006cabf8a47e5ebf16cc17ae3d fix: bound maild API request bodies`.
- Code change:
  `Config.APIMaxBytes` now loads from `MAILD_API_MAX_BYTES` with a 1 MiB
  default, and both owner-send plus ingress-receipt JSON decoding read through
  a bounded decoder before unmarshalling. Oversized API request bodies return
  HTTP 413 `request body too large`.
- Local focused tests passed:
  `nix develop -c go test ./internal/maild -run 'Test(HandleSendRejectsOversizedRequestBody|HandleMessageIngressEventsRejectsOversizedRequestBody|LoadConfigDefaults|ConfigEnsureDirs)'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26543654438` passed for `b68d21c`; Deploy to Staging
  job `78190812325` succeeded.
- Deploy evidence: deploy impact selected `HOST_SERVICES=maild`, built the
  maild host-service package, updated
  `maild -> /var/lib/go-choir/services/maild`, restarted
  `go-choir-maild.service`, and reported maild health ok.
- Node B identity: `/opt/go-choir` reports
  `b68d21c04846e9006cabf8a47e5ebf16cc17ae3d` with clean status;
  `go-choir-maild` is active/running from
  `/var/lib/go-choir/services/maild/bin/maild`.
- Deployed negative proof: a direct local oversized
  `POST /api/email/send` to maild with trusted internal headers returns HTTP 413
  `request body too large`; maild health counters remain
  `messages=0`, `webhook_events=0`, and `ingress_events=0` before and after.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- Every maild body currently decoded from a public/provider/authenticated API
  path now has an explicit size ceiling before JSON decode. The mission remains
  `checkpoint_incomplete`: real Resend webhook delivery, sufficient Resend
  domain/webhook management access, webhook secret deployment, Gandi DNS/MX
  mutation with rollback, real inbound mail, real quarantine, real Send to
  Choir Trace evidence, and real outbound/reply acceptance remain unproven.

Next executable probe:

- Acquire or create a Resend API key/dashboard path with domain/webhook
  management scope; rerun `scripts/mail-provider-readiness`; deploy a real
  `RESEND_WEBHOOK_SECRET` through the credential path; and plan DNS/MX changes
  from exact Resend records before mutating Gandi.

## Security Finding: inbound raw-message refs persist provider download URLs

Recorded: 2026-05-27 during pre-provider maild storage review.

Problem:

`maild` currently maps the Resend received-email `raw.download_url` field into
`email_messages.raw_message_ref`. Resend raw/attachment download URLs are
provider-generated access URLs, not stable public identifiers. Persisting such a
URL in durable SQLite creates a latent secret-retention and expiry problem even
though the current API/UI do not expose `raw_message_ref`.

Evidence:

```text
code:
  internal/maild/resend.go
    - resendReceivedEmail.Raw.DownloadURL models provider raw download URL.

  internal/maild/ingest.go -> buildInboundRecord
    - if email.Raw != nil { rawRef = email.Raw.DownloadURL }
    - StoreInboundMessage writes rawRef into email_messages.raw_message_ref.

current exposure:
  internal/maild/api.go message detail does not return raw_message_ref.
  cmd/maildctl message detail does not print raw_message_ref.
```

Impact:

- This is not a browser leak today, but it violates the direction of the
  mission invariant that provider secrets and temporary provider access material
  stay out of user-visible/product state.
- Durable state should contain stable provenance refs or internal storage refs,
  not provider bearer URLs. Raw email content storage is a future feature and
  should use owned storage with explicit retention/quarantine policy.

Belief-state update:

- For v0, do not persist Resend `raw.download_url`. Either leave
  `raw_message_ref` empty or write a stable non-secret internal ref only after
  raw content is fetched into owned storage under policy.

Next executable probe:

- Change inbound normalization so `raw_message_ref` is not populated from
  Resend `raw.download_url`; add a focused store/webhook test proving a provider
  raw download URL is not stored.

Resolution checkpoint:

- Problem checkpoint commit:
  `1aa30ad315dbf83581f2929b4df6dd357b661925 docs: record mail raw ref retention gap`.
- Fixed in
  `ef80fb2f6b367d711d5d19fe5ff498e23a0828e8 fix: avoid storing provider raw email URLs`.
- Code change:
  inbound normalization no longer copies Resend `raw.download_url` into
  `email_messages.raw_message_ref`; v0 leaves the field empty unless future raw
  email capture writes an owned, non-secret storage ref.
- Local focused test passed:
  `nix develop -c go test ./internal/maild -run 'TestHandleResendWebhookFetchesAndStoresInboundMessage'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26543976101` passed for `ef80fb2`; Deploy to Staging
  job `78191832507` succeeded.
- Deploy evidence: deploy impact selected `HOST_SERVICES=maild`, built the
  maild host-service package, updated
  `maild -> /var/lib/go-choir/services/maild`, restarted
  `go-choir-maild.service`, and reported maild health ok.
- Node B identity: `/opt/go-choir` reports
  `ef80fb2f6b367d711d5d19fe5ff498e23a0828e8` with clean status;
  `go-choir-maild` is active/running from
  `/var/lib/go-choir/services/maild/bin/maild`.
- Public `/health` reports proxy and upstream build deployed_commit
  `ef80fb2f6b367d711d5d19fe5ff498e23a0828e8` and deployed_at
  `2026-05-27T23:06:59Z`.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- Durable maild state no longer retains provider raw-message download URLs. The
  mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

Next executable probe:

- Acquire or create a Resend API key/dashboard path with domain/webhook
  management scope; rerun `scripts/mail-provider-readiness`; deploy a real
  `RESEND_WEBHOOK_SECRET` through the credential path; and plan DNS/MX changes
  from exact Resend records before mutating Gandi.

## Security Finding: source-packet provenance includes internal owner and alias IDs

Recorded: 2026-05-27 during pre-provider source-packet review.

Problem:

`maild` stores source-packet provenance JSON with `alias_id` and
`mailbox_owner_id`. The proxy then includes the full `provenance_json` string in
the prompt-bar handoff text for Send to Choir. That means internal routing IDs
can be copied into the user-computer prompt and Trace path even though they are
not needed for the agent to treat the email as untrusted source material.

Evidence:

```text
code:
  internal/maild/ingest.go -> buildInboundRecord
    provenance includes alias_id and mailbox_owner_id.

  internal/maild/api.go -> handleMessageSourcePacket
    returns packet.ProvenanceJSON.

  internal/proxy/email.go -> buildEmailSourcePrompt
    writes "Provenance: " + source.ProvenanceJSON into the prompt-bar text.
```

Impact:

- This is not an authorization bypass; source-packet access is owner-scoped and
  proxy-mediated for Send to Choir.
- It is still unnecessary internal identifier exposure across the maild ->
  proxy -> user-computer/MAS boundary. The prompt needs stable, non-secret
  provenance such as provider, provider message/event ids, resolved recipient,
  trust label, and attachment count. It does not need the internal owner or
  alias row ids.

Belief-state update:

- Keep internal owner/alias IDs in SQLite relational columns for authorization
  and lookup. Do not duplicate them into source-packet provenance that is handed
  to the user-computer prompt path.

Next executable probe:

- Remove `mailbox_owner_id` and `alias_id` from source-packet provenance JSON,
  and add focused tests proving the stored provenance remains useful but omits
  those internal identifiers.

Resolution checkpoint:

- Problem checkpoint commit:
  `98cf97ce36a4c30004134db49146ab7209b9e645 docs: record mail source provenance ID exposure`.
- Fixed in
  `27b15a59fb959f72e93d6b98341df5973532a236 fix: minimize mail source provenance`.
- Code change:
  inbound source-packet provenance no longer duplicates internal `alias_id` or
  `mailbox_owner_id`; those remain in SQLite relational columns for lookup and
  authorization. Provenance still records provider, provider event/message ids,
  resolved recipient, trust label, attachment count, and trust instructions.
- Local focused test passed:
  `nix develop -c go test ./internal/maild -run 'TestHandleResendWebhookFetchesAndStoresInboundMessage'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26544235598` passed for `27b15a5`; Deploy to Staging
  job `78192658862` succeeded.
- Deploy evidence: deploy impact selected `HOST_SERVICES=maild`, built the
  maild host-service package, updated
  `maild -> /var/lib/go-choir/services/maild`, restarted
  `go-choir-maild.service`, and reported maild health ok.
- Node B identity: `/opt/go-choir` reports
  `27b15a59fb959f72e93d6b98341df5973532a236` with clean status;
  `go-choir-maild` is active/running from
  `/var/lib/go-choir/services/maild/bin/maild`.
- Public `/health` reports proxy and upstream build deployed_commit
  `27b15a59fb959f72e93d6b98341df5973532a236` and deployed_at
  `2026-05-27T23:14:08Z`.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- The source packet remains traceable without carrying internal owner/alias row
  ids into the prompt handoff path. The mission remains `checkpoint_incomplete`:
  real Resend webhook delivery, sufficient Resend domain/webhook management
  access, webhook secret deployment, Gandi DNS/MX mutation with rollback, real
  inbound mail, real quarantine, real Send to Choir Trace evidence, and real
  outbound/reply acceptance remain unproven.

Next executable probe:

- Acquire or create a Resend API key/dashboard path with domain/webhook
  management scope; rerun `scripts/mail-provider-readiness`; deploy a real
  `RESEND_WEBHOOK_SECRET` through the credential path; and plan DNS/MX changes
  from exact Resend records before mutating Gandi.

## Staging/Design Finding: owner-send lacks provider idempotency key

Recorded: 2026-05-27 during provider-readiness continuation.

Problem:

The Choir Email reference outbound flow says `maild` sends through Resend with
an idempotency key. Current owner-authored outbound mail calls Resend
`POST /emails` without setting a provider idempotency key header. Official
Resend docs state that `POST /emails` supports the `Idempotency-Key` header for
duplicate-send prevention, with keys expiring after 24 hours.

Evidence:

```text
code path:
  internal/maild/send.go -> h.resend.sendEmail(...)
  internal/maild/resend.go -> POST {RESEND_BASE_URL}/emails

current request headers:
  Authorization
  Accept
  Content-Type

missing:
  Idempotency-Key

provider docs:
  https://resend.com/docs/dashboard/emails/idempotency-keys
  https://resend.com/docs/api-reference/emails
```

Impact:

- If the owner-send HTTP call is retried across a transient network or gateway
  failure after Resend accepted the first request, `maild` does not give Resend
  a stable key to suppress a duplicate outbound email.
- This does not enable inbound-triggered outbound mail by itself; owner-send
  still requires the authenticated proxy/maild path. It is still a reliability
  and duplicate-send gap in the explicit owner-send slice.

Belief-state update:

- The next safe in-repo improvement is to add a stable, request-derived
  idempotency key to owner-authored Resend sends and test that the header is
  present without logging or exposing secrets.

Next executable probe:

- Add an `Idempotency-Key` header to `resendClient.sendEmail`, derived from the
  canonical outbound payload, and run focused maild send tests before deploy.

Resolution evidence, 2026-05-27:

- `internal/maild/resend.go` now sets `Idempotency-Key` on Resend `POST
  /emails` calls as `choir_maild_<sha256(canonical request json)>`.
- `internal/maild/send_test.go` now verifies owner-send requests include the
  `choir_maild_` idempotency key and that identical send payloads produce a
  stable non-empty key no longer than Resend's 256-character limit.
- Focused local verification passed:
  `nix develop -c go test ./internal/maild -run 'TestHandleSend|TestResendSendEmail'`.

Remaining evidence needed:

- Broader local verification passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- Provider-level outbound proof through the real Resend API remains blocked by
  domain/webhook authority and DNS readiness; the deployed idempotency repair is
  proven at the code/service-boundary level.

Deploy resolution evidence, 2026-05-27:

- Fix commit:
  `17df13e9d9221ea24b3a2dfdb83b7dd78fa4d8d4 fix: send mail with Resend idempotency key`.
- GitHub Actions CI run `26540437227` passed for `17df13e`; Deploy to
  Staging job `78180464584` succeeded.
- Deploy impact selected `HOST_SERVICES=maild`; the deploy log shows:
  - `Fast building Host service maild: ./cmd/maild`
  - fallback Nix package build `.#maild`
  - built `/nix/store/bns82v8z38miz96p7qxi42dqk74qxlsb-maild-0.1.0.drv`
  - `Host service pointer updated: maild -> /var/lib/go-choir/services/maild`
  - `Restarted go-choir-maild.service`
- Node B after deploy:
  - `/var/lib/go-choir/deploy.env`:
    `17df13e9d9221ea24b3a2dfdb83b7dd78fa4d8d4`
  - `/opt/go-choir` HEAD:
    `17df13e9d9221ea24b3a2dfdb83b7dd78fa4d8d4`
  - `/opt/go-choir` status: clean
  - `go-choir-maild` MainPID: `2075468`
  - `go-choir-maild` ExecMainStartTimestamp:
    `Wed 2026-05-27 21:43:27 UTC`
  - `/proc/$pid/exe`: `/var/lib/go-choir/services/maild/bin/maild`
- Node B maild health remains ok with `webhook_secret_configured=false` and
  zero message/webhook/ingress counters.
- Direct local mailbox-route probes still prove the proxy identity marker
  boundary:
  - no owner header -> HTTP 401 `authentication required`
  - `X-Authenticated-User` only -> HTTP 403 `internal caller required`
  - `X-Authenticated-User` + `X-Internal-Caller:false` -> HTTP 403
    `internal caller required`
  - `X-Authenticated-User` + `X-Internal-Caller:true` -> HTTP 200
- Public proxy probes still return HTTP 401 for both unauthenticated and
  spoofed `X-Authenticated-User`/`X-Internal-Caller:true` requests.
- `scripts/mail-provider-readiness` after deploy still reports:
  - local `RESEND_API_KEY` configured, `RESEND_WEBHOOK_SECRET` missing, and
    `GANDI_PAT` configured;
  - Resend domains/webhooks return `401 restricted_api_key`;
  - Gandi LiveDNS and public DNS remain on Gandi MX/SPF/DKIM and no DMARC;
  - public webhook negative probe returns HTTP 503
    `webhook_secret_not_configured` without counter mutation.

Belief-state update:

- The owner-send idempotency repair is deployed on the live maild service.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

## Security Finding: malformed webhook secret can be treated as configured

Recorded: 2026-05-27 during webhook-verifier hardening.

Problem:

`HandleResendWebhook` only checks whether `RESEND_WEBHOOK_SECRET` is a non-empty
string before attempting Svix verification. `verifyWebhook` decodes the secret
after trimming the `whsec_` prefix, but an accidental value of `whsec_` decodes
to an empty HMAC key. That means a syntactically malformed deployed webhook
secret would move `maild` out of the safe `webhook_secret_not_configured` state
and into a verifier state whose effective signing key is empty.

Evidence:

```text
code path:
  internal/maild/webhook.go -> HandleResendWebhook
  internal/maild/webhook.go -> verifyWebhook

current behavior:
  RESEND_WEBHOOK_SECRET=""      -> 503 webhook_secret_not_configured
  RESEND_WEBHOOK_SECRET="whsec_" -> base64 decode succeeds as []byte{}

provider docs:
  Resend requires raw-body Svix signature verification:
    https://resend.com/docs/webhooks/verify-webhooks-requests
  Svix documents using the base64 portion after whsec_ as the HMAC-SHA256 key,
  timestamp replay tolerance, and space-delimited signatures:
    https://www.svix.com/guides/receiving/receive-webhooks-with-python/
```

Impact:

- This does not affect the current Node B state because
  `webhook_secret_configured=false` and the public webhook route fails closed.
- It is a deploy-time footgun before real Resend setup: a malformed non-empty
  secret should not be accepted as a configured webhook verifier.

Belief-state update:

- Before deploying a real `RESEND_WEBHOOK_SECRET`, `maild` should reject
  malformed or empty decoded Svix secrets and keep the route fail-closed.

Next executable probe:

- Add verifier tests for malformed empty `whsec_`, stale timestamps, and
  multiple space-delimited Svix signatures; then reject decoded empty signing
  keys in `verifyWebhook`.

Resolution evidence, 2026-05-27:

- Problem checkpoint commit:
  `9cb86c2 docs: record malformed mail webhook secret gap`.
- Fix commit:
  `d65641483be2d4462c0dbbf01ba25efddceca603 fix: reject empty mail webhook signing secret`.
- `internal/maild/webhook.go` now rejects decoded-empty Svix signing keys.
- `internal/maild/webhook_test.go` now verifies:
  - `RESEND_WEBHOOK_SECRET=whsec_` returns `invalid_signature` and stores no
    webhook event;
  - stale Svix timestamps reject;
  - space-delimited Svix signatures accept when any `v1` signature matches.
- Focused local verification passed:
  `nix develop -c go test ./internal/maild -run 'TestHandleResendWebhook'`.
- Broader local verification passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.

Deploy resolution evidence, 2026-05-27:

- GitHub Actions CI run `26541199014` passed for `d656414`; Deploy to Staging
  job `78182981909` succeeded.
- Deploy impact selected `HOST_SERVICES=maild`; deploy log shows:
  - `Fast building Host service maild: ./cmd/maild`
  - fallback Nix package build `.#maild`
  - built `/nix/store/9f13nfqiykkn8hzrzh0cqh1jzly8a2sa-maild-0.1.0.drv`
  - `Host service pointer updated: maild -> /var/lib/go-choir/services/maild`
  - `Restarted go-choir-maild.service`
- Node B after deploy:
  - `/var/lib/go-choir/deploy.env`:
    `d65641483be2d4462c0dbbf01ba25efddceca603`
  - `/opt/go-choir` HEAD:
    `d65641483be2d4462c0dbbf01ba25efddceca603`
  - `/opt/go-choir` status: clean
  - `go-choir-maild` MainPID: `2078513`
  - `go-choir-maild` ExecMainStartTimestamp:
    `Wed 2026-05-27 22:00:05 UTC`
  - `/proc/$pid/exe`: `/var/lib/go-choir/services/maild/bin/maild`
- Node B maild health remains ok with `webhook_secret_configured=false` and
  zero message/webhook/ingress counters.
- `scripts/mail-provider-readiness` after deploy still reports:
  - local `RESEND_API_KEY` configured, `RESEND_WEBHOOK_SECRET` missing, and
    `GANDI_PAT` configured;
  - Resend domains/webhooks return `401 restricted_api_key`;
  - Gandi LiveDNS and public DNS remain on Gandi MX/SPF/DKIM and no DMARC;
  - public webhook negative probe returns HTTP 503
    `webhook_secret_not_configured` without counter mutation.

Belief-state update:

- The malformed-webhook-secret deploy footgun is repaired on the live maild
  service before real Resend webhook-secret deployment.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

## Security Finding: trusted-upload whitelist accepts messages without auth-result evidence

Recorded: 2026-05-27 during trusted-upload receive-policy audit.

Problem:

The reference requires trusted-upload aliases to use an exact secret alias, a
manual sender whitelist, and recorded message-authentication results. Current
`enforceReceivePolicy` validates the exact secret alias and sender whitelist,
but it can accept the whitelisted sender even when the received message carries
no `Authentication-Results` or `ARC-Authentication-Results` header for the
owner/operator to inspect later.

Evidence:

```text
code:
  internal/maild/webhook.go -> enforceReceivePolicy
    - parses visible From/header From
    - checks email_sender_whitelist when RequireSenderWhitelist=true
    - does not require authentication-results evidence before accepting

  internal/maild/ingest.go -> authenticationResultsJSON
    - stores authentication-results and arc-authentication-results if present
    - returns empty string when neither header is present

tests:
  internal/maild/webhook_test.go accepts whitelisted trusted-upload aliases
  only with authentication-results present, but has no negative test for a
  whitelisted trusted-upload message with no auth-result evidence.

reference:
  docs/choir-email-reference-v0.md:
    - "Do not trust visible From: alone."
    - trusted uploads require sender whitelist and recorded
      message-authentication results.
```

Impact:

- This does not grant MAS authority: accepted email still becomes
  `UNTRUSTED_EXTERNAL_EMAIL`, and owner/proxy handoff is still required.
- It weakens the trusted-upload mailbox classification. A future operator or UI
  may see `trust_status="trusted"` for a whitelisted sender without the recorded
  authentication evidence the reference says must be available.

Belief-state update:

- Trusted-upload policy should fail closed when `RequireSenderWhitelist=true`
  and no message-authentication result header is present. The current code can
  remain tolerant of SPF/DKIM/DMARC pass/fail semantics for v0; the immediate
  invariant is that the evidence exists and is recorded.

Next executable probe:

- Add a focused negative test for a whitelisted trusted-upload alias with no
  auth-result headers, then require authentication-results or ARC
  authentication-results presence before accepting a sender-whitelist-gated
  inbound message.

Resolution checkpoint:

- Problem checkpoint commit:
  `53c27187d581dd27421308ff93d3dfd922d6975f docs: record trusted upload auth-result gap`.
- Fixed in
  `dc40d0a3fd43294ad40e5194e99c3f398a74a77c fix: require mail auth results for trusted uploads`.
- Local tests passed:
  `nix develop -c go test ./internal/maild -run 'TestHandleResendWebhook(RejectsUnwhitelistedTrustedUploadAlias|RejectsWhitelistedTrustedUploadAliasWithoutAuthenticationResults|AcceptsWhitelistedTrustedUploadAlias)'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26541562656` passed for `dc40d0a`; Deploy to
  Staging job `78184166989` succeeded.
- Deploy evidence: `HOST_SERVICES=maild`, built `.#maild`, updated
  `maild -> /var/lib/go-choir/services/maild`, restarted
  `go-choir-maild.service`, and reported maild health ok.
- Node B identity: `/var/lib/go-choir/deploy.env` and `/opt/go-choir` both
  report `dc40d0a3fd43294ad40e5194e99c3f398a74a77c`; `/opt/go-choir` is clean;
  `go-choir-maild` is active/running from
  `/var/lib/go-choir/services/maild/bin/maild`.
- Node B maild health: status ok, `webhook_secret_configured=false`, and zero
  message/webhook/ingress counters.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- Sender-whitelist-gated trusted-upload classification now fails closed unless
  the received message carries recorded `Authentication-Results` or
  `ARC-Authentication-Results` evidence.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

## Security Finding: received-email provider fetch is not size-bounded

Recorded: 2026-05-27 during pre-MX maild readiness audit.

Problem:

`HandleResendWebhook` caps the raw webhook body with `MAILD_WEBHOOK_MAX_BYTES`,
but the follow-up Resend Receiving API fetch decodes `resp.Body` directly in
`resendClient.retrieveReceivedEmail`. Real inbound mail goes through this path
before alias policy storage, attachment quarantine metadata, and source-packet
creation. A large provider response could force maild to allocate and decode an
unbounded JSON body and then store very large text/html fields in SQLite.

Evidence:

```text
code:
  internal/maild/webhook.go
    - HandleResendWebhook reads the webhook through io.LimitReader.
    - ingestReceivedEmail then calls h.resend.retrieveReceivedEmail.

  internal/maild/resend.go
    - retrieveReceivedEmail calls json.NewDecoder(resp.Body).Decode(&email)
      without an io.LimitReader or configured provider-response cap.

reference:
  Resend Receiving webhooks intentionally omit body, headers, and attachment
  content; the application must call the Receiving API to retrieve the full
  email body and headers.

  Resend raw email and attachment APIs return signed temporary download URLs.
  Attachment/raw content remains intentionally deferred, but the received-email
  JSON fetch is already on the live ingest path.
```

Impact:

- This is a denial-of-service and storage-bloat risk before root MX mutation.
- It does not break the authority invariant: oversized provider responses still
  would not gain agent/tool/canonical-state authority.
- It weakens the dense-feedback invariant that inbound processing has body caps
  before durable storage.

Belief-state update:

- The existing webhook cap is necessary but insufficient. `maild` also needs a
  configurable cap for provider API response bodies used during inbound ingest.
- The cap should fail closed and request webhook retry rather than storing a
  partial message.

Next executable probe:

- Add a `MAILD_PROVIDER_MAX_BYTES` config value and apply it to Resend
  received-email response decoding with a focused oversized-provider-response
  test.

Resolution checkpoint:

- Problem checkpoint commit:
  `02cc949850d3c32176c489d6246365125af3d415 docs: record mail provider fetch size gap`.
- Fixed in
  `15ddeb4fe02213a58c3d1ca63b293657ddf5c4ce fix: bound mail provider response bodies`.
- Code change:
  `MAILD_PROVIDER_MAX_BYTES` defaults to 4 MiB and bounds successful Resend
  received-email response bodies before JSON decode. Oversized provider
  responses fail before message storage and are treated as retryable ingest
  errors.
- Local focused tests passed:
  `nix develop -c go test ./internal/maild -run 'Test(LoadConfigDefaults|HandleResendWebhookOversizedProviderResponseRequestsRetry|HandleResendWebhookFetchesAndStoresInboundMessage)'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26542167430` passed for `15ddeb4`; Deploy to Staging
  job `78186101002` succeeded.
- Deploy evidence: `HOST_SERVICES=maild`, built `.#maild`, updated
  `maild -> /var/lib/go-choir/services/maild`, restarted
  `go-choir-maild.service`, and reported maild health ok.
- Node B identity: `/var/lib/go-choir/deploy.env` and `/opt/go-choir` both
  report `15ddeb4fe02213a58c3d1ca63b293657ddf5c4ce`; `/opt/go-choir` is clean;
  `go-choir-maild` is active/running from
  `/var/lib/go-choir/services/maild/bin/maild`.
- Node B maild health: status ok, `webhook_secret_configured=false`, and zero
  message/webhook/ingress counters.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- The Resend webhook path and Resend received-email retrieval path now both
  have explicit body caps before storage.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.

## Security Finding: Send to Choir prompt body is not strongly data-framed

Recorded: 2026-05-27 during Send to Choir prompt-injection audit.

Problem:

The proxy-owned Send to Choir path correctly requires an authenticated owner,
fetches a source packet from `maild`, rejects unexpected trust labels, and
submits through the existing prompt-bar/conductor path. The prompt text warns
that the email is `UNTRUSTED_EXTERNAL_EMAIL`, but then appends the normalized
email body as raw text after the instruction preamble. An adversarial email body
cannot bypass owner action, but it is still concatenated into the same user
prompt payload without per-line data quoting or structured delimiting.

Evidence:

```text
code:
  internal/proxy/email.go -> buildEmailSourcePrompt
    - writes owner/proxy instructions;
    - writes "Normalized email body follows as untrusted source material:";
    - appends source.TextBody directly, with only truncation.

reference:
  docs/choir-email-reference-v0.md:
    "External email body, headers, links, quoted replies, and attachments are
     untrusted data. They must be wrapped as source material and never
     concatenated into an agent instruction channel as if they were
     owner/system/developer text."
```

Impact:

- This does not give inbound mail autonomous authority: Send to Choir is still
  owner-triggered, and `maild` still cannot call agents or mutate canonical
  state.
- It weakens the prompt-injection boundary inside the first MAS handoff. A
  malicious body can visually resemble instructions after the trusted preamble.

Belief-state update:

- The first v0 prompt-bar handoff should quote untrusted email body lines as
  data, not just describe them as untrusted. Metadata derived from the email
  should also be clearly called out as untrusted or generated.

Next executable probe:

- Change `buildEmailSourcePrompt` so untrusted body content is line-prefixed or
  otherwise strongly data-framed, and add a focused test with an injection-like
  email body proving no raw line appears unframed in the prompt.

Resolution checkpoint:

- Problem checkpoint commit:
  `74ad64709c26d1731c0ec6f7778d3e64602f9ba5 docs: record mail prompt data framing gap`.
- Fixed in
  `ad1af5587550576ca82b5d72782d9efe2bbd8cd3 fix: frame mail source body as quoted data`.
- Code change:
  `buildEmailSourcePrompt` now marks sender/subject/snippet fields as
  untrusted and line-prefixes every normalized email body line with
  `EMAIL-DATA:` before submitting through `/api/prompt-bar`.
- Local focused tests passed:
  `nix develop -c go test ./internal/proxy -run 'Test(EmailSendToChoirFetchesSourcePacketAndSubmitsPromptBar|BuildEmailSourcePromptQuotesInjectionLikeBodyLines|BuildEmailSourcePromptTruncatesBody)'`.
- Local package tests passed:
  `nix develop -c go test ./internal/maild ./cmd/maild ./cmd/maildctl ./internal/proxy`.
- GitHub Actions CI run `26542524019` passed for `ad1af55`; Deploy to Staging
  job `78187232221` succeeded.
- Deploy evidence: deploy impact selected `HOST_SERVICES=proxy`, updated
  `proxy -> /var/lib/go-choir/services/proxy`, and reported maild health ok.
- Node B identity: `/var/lib/go-choir/deploy.env` and `/opt/go-choir` both
  report `ad1af5587550576ca82b5d72782d9efe2bbd8cd3`; `/opt/go-choir` is clean;
  `go-choir-proxy` is active/running from
  `/var/lib/go-choir/services/proxy/bin/proxy`.
- Public health reports proxy and upstream sandbox deployed commit
  `ad1af5587550576ca82b5d72782d9efe2bbd8cd3`.
- Provider readiness remains intentionally not ready: current Resend key still
  returns `401 restricted_api_key` for domain/webhook inspection, Gandi LiveDNS
  and public DNS still point at Gandi mail defaults, no DMARC record is present,
  and the public unsigned webhook probe returns HTTP 503
  `webhook_secret_not_configured` with no counter mutation.

Belief-state update:

- The first owner-triggered email-to-MAS handoff now frames normalized email
  body lines as quoted untrusted data rather than raw prompt continuation text.
- The mission remains `checkpoint_incomplete`: real Resend webhook delivery,
  sufficient Resend domain/webhook management access, webhook secret deployment,
  Gandi DNS/MX mutation with rollback, real inbound mail, real quarantine, real
  Send to Choir Trace evidence, and real outbound/reply acceptance remain
  unproven.
