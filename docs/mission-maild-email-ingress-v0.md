# Mission: maild Email Ingress v0

Last updated: 2026-05-26

Reference: [choir-email-reference-v0.md](choir-email-reference-v0.md)

## Mission Status

```text
status: checkpoint_incomplete
current artifact state: deployed maild/proxy/frontend slice exists on Node B at d749fdc; Resend domain/webhook setup and DNS/MX remain unconfigured
what shipped: maild service, SQLite mailbox, webhook verifier, quarantine metadata, source packets, Email app, proxy auth forwarding, proxy-owned Send to Choir, read-only maildctl, bounded provider logging, reply threading headers
locally proven: fake signed Resend webhook -> fetch/normalize/store/quarantine/source packet; owner-only send; owned reply target -> In-Reply-To/References; proxy-owned Send to Choir contract; frontend production build; NixOS maild/Caddy route eval; read-only provider readiness probe
unproven claims: real Resend webhook, Resend domain verification, Gandi DNS/MX, real inbound/outbound mail, real Send to Choir trace from received email
next executable probe: obtain a Resend key/dashboard session that can read domain and webhook configuration, then use scripts/mail-provider-readiness to verify exact provider truth before any Gandi DNS mutation
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
  model, but must be treated as a late, reversible operations step.
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
last checkpoint: deployed reply-threading checkpoint on 2026-05-26 at d749fdc
current artifact state: cmd/maild, internal/maild, proxy forwarding/MAS handoff, Email app shell, Node B maild service route, maildctl, and mail credential deploy script are deployed; Resend receiving/webhook and Gandi DNS are not configured
what shipped: maild service, minimal Email app, proxy auth boundary, Send to Choir handoff, operator inspection CLI, bounded provider logging, and RFC reply threading headers for owner replies
what was proven:
  - signed fake Resend webhook verification, idempotency, missing-secret, missing-header, and mutated-body rejection
  - fake Resend retrieval stores inbound message, quarantines attachment metadata, and creates UNTRUSTED_EXTERNAL_EMAIL source packet
  - owner-only outbound send through fake Resend stores Sent row
  - owned reply targets preserve provider message_id and emit In-Reply-To/References headers in the Resend send payload
  - proxy forwards authenticated /api/email/* to maild while stripping spoofed identity and Cookie headers
  - proxy-owned Send to Choir fetches a source packet and submits guarded prompt-bar text to the resolved user computer
  - frontend production build succeeds
  - local Playwright visual harness renders Email app on desktop and mobile-sized windows with fixture mail and no `undefined` text
  - NixOS eval exposes go-choir-maild and Caddy webhook route before generic /api/*
  - Node B deployed commit identity and service health report d749fdcfb329226f73ce4717b86f1ac0eba5e1a0
  - read-only provider readiness probe reports Resend/Gandi/Node B state without mutating DNS or printing secrets
unproven or partial claims:
  - real Resend webhook and API payload compatibility
  - Gandi MX/SPF/DKIM/DMARC setup and rollback
  - real inbox appearance, real reply/send, and real Send to Choir trace
  - GitHub Actions no longer emits runs for recent pushes; deploy proof is manual Node B source-truth deploy proof
belief-state changes:
  - maild as separate microservice remains the right boundary
  - Resend credentials belong in /var/lib/go-choir/maild.env, not gateway-provider.env or platformd
  - 000@choir.news must be seeded to the real auth user id through MAILD_ROOT_OWNER_ID; Nix must not bake in a placeholder owner
  - plus aliases should not implicitly fall back to 000 because that weakens secret-alias policy
  - outbound reaches Resend, but the current Resend account state rejects 000@choir.news because choir.news is not verified
remaining error field:
  - real provider/DNS proof is still untouched because exact Resend verification/receiving records and webhook secret are missing
  - v0 does not yet expose raw headers/details in the UI
  - attachment content download/extraction remains intentionally deferred
highest-impact remaining uncertainty: exact Resend domain/receiving/webhook configuration needed to prove real inbound and outbound
next executable probe: obtain a sufficiently scoped Resend key or dashboard session, run scripts/mail-provider-readiness until Resend domain/webhook records are visible, install RESEND_WEBHOOK_SECRET through /var/lib/go-choir/maild.env, update Gandi DNS from exact provider records, and run the real inbound/quarantine/source-packet/outbound acceptance
suggested resume goal string: continue docs/mission-maild-email-ingress-v0.md from deployed checkpoint d749fdc; obtain Resend domain/webhook provider truth, use scripts/mail-provider-readiness before DNS mutation, configure Gandi from exact records, then prove real inbound mail, quarantine, source-packet MAS handoff, and owner reply
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
rollback refs:
  - do not add MX until exact Resend records and webhook secret are available
  - current Gandi MX/SPF remains Gandi mail defaults until provider records are verified
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
5. Snapshot current Gandi DNS, then replace root MX only after accepting that
   root-domain inbound mail will leave Gandi mailbox delivery and go to Resend.

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

Next executable probe:

- Use a broader temporary Resend API key or authenticated dashboard session to
  create/retrieve `choir.news`, enable receiving, create/retrieve the
  `email.received` webhook for `https://choir.news/api/email/resend/webhook`,
  and capture exact DNS records plus `RESEND_WEBHOOK_SECRET`.
