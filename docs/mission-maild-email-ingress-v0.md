# Mission: maild Email Ingress v0

Last updated: 2026-05-26

Reference: [choir-email-reference-v0.md](choir-email-reference-v0.md)

## Mission Status

```text
status: checkpoint_incomplete
current artifact state: local maild/proxy/frontend/deploy slice exists; no commit, deploy, Resend setup, or DNS/MX changes yet
what shipped: none
locally proven: fake signed Resend webhook -> fetch/normalize/store/quarantine/source packet; owner-only send; proxy-owned Send to Choir contract; frontend production build; NixOS maild/Caddy route eval
unproven claims: deployed Node B service, real Resend webhook, Gandi DNS/MX, staging auth route, real MAS handoff, real inbound/outbound mail
next executable probe: visual check the minimal Email app, then commit/push/deploy and prove maild route identity on staging before touching DNS/MX
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
last checkpoint: local implementation/deploy wiring checkpoint on 2026-05-26
current artifact state: cmd/maild, internal/maild, proxy forwarding/MAS handoff, Email app shell, Node B maild service route, and mail credential deploy script are present locally
what shipped: nothing yet
what was proven:
  - signed fake Resend webhook verification, idempotency, missing-secret, missing-header, and mutated-body rejection
  - fake Resend retrieval stores inbound message, quarantines attachment metadata, and creates UNTRUSTED_EXTERNAL_EMAIL source packet
  - owner-only outbound send through fake Resend stores Sent row
  - proxy forwards authenticated /api/email/* to maild while stripping spoofed identity and Cookie headers
  - proxy-owned Send to Choir fetches a source packet and submits guarded prompt-bar text to the resolved user computer
  - frontend production build succeeds
  - local Playwright visual harness renders Email app on desktop and mobile-sized windows with fixture mail and no `undefined` text
  - NixOS eval exposes go-choir-maild and Caddy webhook route before generic /api/*
unproven or partial claims:
  - frontend visual/browser proof
  - x86_64 Linux package build/vendor hashes on Node B/CI
  - deployed systemd service health
  - real Resend webhook and API payload compatibility
  - Gandi MX/SPF/DKIM/DMARC setup and rollback
  - real inbox appearance, real reply/send, and real Send to Choir trace
belief-state changes:
  - maild as separate microservice remains the right boundary
  - Resend credentials belong in /var/lib/go-choir/maild.env, not gateway-provider.env or platformd
  - 000@choir.news must be seeded to the real auth user id through MAILD_ROOT_OWNER_ID; Nix must not bake in a placeholder owner
  - plus aliases should not implicitly fall back to 000 because that weakens secret-alias policy
remaining error field:
  - deployed proof and real provider/DNS proof are still untouched
  - v0 does not yet expose raw headers/details in the UI
  - attachment content download/extraction remains intentionally deferred
highest-impact remaining uncertainty: deployed route/auth/provider behavior under real staging conditions
next executable probe: run frontend visual check, then commit/push and let CI/deploy prove x86_64 service packaging before configuring Resend/Gandi
suggested resume goal string: continue docs/mission-maild-email-ingress-v0.md from the local implementation checkpoint; visual-check Email app, commit/push, monitor CI/deploy, verify maild staging route, then configure Resend/Gandi only after rollback evidence
evidence artifact refs:
  - nix develop -c go test ./internal/maild ./cmd/maild ./internal/proxy
  - npm run build in frontend
  - local Playwright screenshots: /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/choir-email-visual/email-desktop.png and /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/choir-email-visual/email-mobile-narrow.png
  - nix eval .#packages.x86_64-linux.maild.pname
  - nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-maild.description
  - nix eval .#nixosConfigurations.go-choir-b.config.services.caddy.virtualHosts."choir.news".extraConfig
rollback refs:
  - do not add MX until deployed route is proven
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
