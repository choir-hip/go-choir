# Mission: maild Email Ingress v0

Last updated: 2026-05-27

Reference: [choir-email-reference-v0.md](choir-email-reference-v0.md)

## Mission Status

```text
status: checkpoint_incomplete
current artifact state: maild/proxy/frontend behavior slice, deploy cleanup, trusted-mail evidence classification, proxy trust-label enforcement, webhook-correlated acceptance tooling, and canonical outbound From enforcement are deployed on Node B at d2460c4 through GitHub Actions; /opt/go-choir is a clean checkout matching deploy.env and public health; Resend domain/webhook setup and DNS/MX remain unconfigured
what shipped: maild service, SQLite mailbox, webhook verifier, duplicate webhook ingest retry, receive-policy gates, quarantine metadata, source packets, Email app with Compose, row attachment indicators, collapsed raw-header/stored-recipient Details, proxy auth forwarding, proxy-owned Send to Choir, source-packet provenance/text refs, bounded normalized email-body handoff, ingress-event receipts, read-only maildctl, bounded provider logging, reply threading headers
locally proven: fake signed Resend webhook -> fetch/normalize/store/quarantine/source packet; duplicate email.received after transient provider failure retries and stores missing message idempotently; trusted-upload-style alias rejects unwhitelisted sender and accepts whitelisted sender; whitelisted trusted-upload messages store trusted sender status and authentication-results evidence; owner-only send; outbound send canonicalizes display-name input back to the numeric alias; owned reply target -> In-Reply-To/References; proxy-owned Send to Choir now carries provenance, stable text refs, bounded normalized email body, and ingress receipt; proxy rejects unexpected source-packet trust labels before prompt-bar submission; acceptance checker now requires selected message provider ids and matching webhook receipt; message list attachment indicator; message-detail raw headers and stored recipient API/UI details surface; Compose posts plain owner-send payload through /api/email/send; frontend production build; NixOS maild/Caddy route eval; read-only provider readiness probe; dry-run Resend setup helper; webhook secret handoff dry-run; dry-run Gandi DNS plan/rollback tooling; mail acceptance checker fake-ssh path
deployed proven: GitHub Actions run 26533204422 passed and deployed d2460c41a74d2d13132bb7b38059d017b97a0db4 to staging; public health reports proxy/sandbox deployed_commit d2460c41a74d2d13132bb7b38059d017b97a0db4; /opt/go-choir HEAD matches d2460c4 with clean status; Node B maild health is ok with webhook_secret_configured=false and zero message/event counters
unproven claims: real Resend webhook, Resend domain verification, Gandi DNS/MX, real inbound/outbound mail, real Send to Choir trace from received email
next executable probe: obtain a Resend key/dashboard session that can read domain and webhook configuration, run scripts/mail-provider-readiness to verify exact provider truth, then deploy RESEND_WEBHOOK_SECRET and plan Gandi DNS from exact Resend records before any MX mutation
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
last checkpoint: receive-policy enforcement deployed on 2026-05-26 at 7de363e
current artifact state: cmd/maild, internal/maild, proxy forwarding/MAS handoff, Email app shell, Node B maild service route, maildctl, and mail credential deploy script are deployed through GitHub Actions at behavior commit 7de363e; Resend receiving/webhook and Gandi DNS are not configured
what shipped: maild service, minimal Email app with Compose and collapsed raw-header/stored-recipient Details, proxy auth boundary, Send to Choir handoff, operator inspection CLI, bounded provider logging, RFC reply threading headers for owner replies, ingress-event handoff receipts, duplicate webhook ingest retry, receive-policy gates, and a read-only mail acceptance checker
what was proven:
  - signed fake Resend webhook verification, idempotency, missing-secret, missing-header, and mutated-body rejection
  - fake Resend retrieval stores inbound message, quarantines attachment metadata, and creates UNTRUSTED_EXTERNAL_EMAIL source packet
  - duplicate `email.received` delivery after a transient provider fetch failure retries ingest and stores the missing message while keeping one webhook event row
  - trusted-upload-style receive policy rejects unwhitelisted sender before storing the message and accepts a whitelisted sender on an unlisted exact plus alias
  - owner-only outbound send through fake Resend stores Sent row
  - owned reply targets preserve provider message_id and emit In-Reply-To/References headers in the Resend send payload
  - proxy forwards authenticated /api/email/* to maild while stripping spoofed identity and Cookie headers
  - proxy-owned Send to Choir fetches a source packet and submits guarded prompt-bar text to the resolved user computer
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
belief-state changes:
  - maild as separate microservice remains the right boundary
  - Resend credentials belong in /var/lib/go-choir/maild.env, not gateway-provider.env or platformd
  - 000@choir.news must be seeded to the real auth user id through MAILD_ROOT_OWNER_ID; Nix must not bake in a placeholder owner
  - plus aliases should not implicitly fall back to 000 because that weakens secret-alias policy
  - outbound reaches Resend, but the current Resend account state rejects 000@choir.news because choir.news is not verified
  - the GitHub Actions 403/setup failure appears recovered after rerun; local GitHub API checks also showed the user and org membership active
remaining error field:
  - real provider/DNS proof is still untouched because exact Resend verification/receiving records and webhook secret are missing
  - current Resend key cannot inspect domains or webhooks because it is restricted to sending only
  - attachment content download/extraction remains intentionally deferred
highest-impact remaining uncertainty: exact Resend domain/receiving/webhook configuration needed to prove real inbound and outbound
next executable probe: obtain a sufficiently scoped Resend key or dashboard session, run scripts/mail-provider-readiness until Resend domain/webhook records are visible, install RESEND_WEBHOOK_SECRET through /var/lib/go-choir/maild.env, update Gandi DNS from exact provider records, and run the real inbound/quarantine/source-packet/outbound acceptance
suggested resume goal string: continue docs/mission-maild-email-ingress-v0.md from deployed receive-policy checkpoint 7de363e; obtain Resend domain/webhook provider truth, use scripts/mail-provider-readiness before DNS mutation, configure Gandi from exact records, then prove real inbound mail, quarantine, source-packet MAS handoff, and owner reply
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
