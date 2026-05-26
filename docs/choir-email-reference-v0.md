# Choir Email Reference v0

Last updated: 2026-05-26

## Purpose

This is the durable reference for Choir Email v0. It holds product model,
architecture, security, data, provider, DNS, and UI direction. The executable
MissionGradient run lives in
[mission-maild-email-ingress-v0.md](mission-maild-email-ingress-v0.md).

## Reviewed External Assumptions

- Resend Receiving emits `email.received` webhooks for inbound mail.
- Resend webhook verification uses the raw request body, Svix headers, and the
  endpoint signing secret.
- Resend inbound attachment webhooks include metadata; attachment content is
  fetched later through Resend attachment download APIs and temporary URLs.
- Resend outbound send uses the Send Email API.
- Gandi LiveDNS v5 uses `Authorization: Bearer <PAT>` and record operations
  under `/v5/livedns/domains/{fqdn}/records`.

Re-check official Resend and Gandi docs before implementation, because provider
field names and DNS verification records can change.

## Product Model

A numeric `@choir.news` address is a mail port into a Choir account, automatic
computer, or agent endpoint. It is not primarily a fake-human identity address.

Examples:

```text
000@choir.news
000+read@choir.news
000+drop@choir.news
000+news@choir.news
000+upload-<secret>@choir.news
000+agent-014@choir.news
```

Initial allocation policy:

```text
000      root / founder / canonical early account
001-099  reserved system/founder/internal
100-999  early private beta users
1000+    public users
900000+  optional future generated app/agent endpoints
```

Do not recycle numbers. A public number is part of early-platform identity.
Public numeric addresses are enumerable handles, not secrets.

Backend identity must use opaque principals:

```text
public_address:      000@choir.news
public_number:       000
internal_user_id:    UUID/ULID
internal_mailbox_id: UUID/ULID
internal_agent_id:   UUID/ULID, if applicable
```

Never authorize by numeric email address. Resolve aliases to internal
principals, then enforce explicit ownership/capability checks.

## Service Boundary

Create a separate host-side `maild` service.

Responsibilities:

- Resend inbound webhook verification.
- Resend received-message and attachment metadata fetches.
- Resend outbound send.
- Alias and receive/send policy enforcement.
- Mailbox SQLite state.
- Raw message and quarantined attachment storage.
- Source packet creation for safe MAS consumption.

Non-responsibilities:

- Agent execution.
- Conductor decisions.
- VText canonical mutation.
- Tool execution.
- Platform publication/retrieval/citation rows.
- Model/search provider proxying.

Current service boundary:

```text
gateway   = model/search/provider egress
platformd = platform Dolt publication/retrieval/citation service
maild     = mail transport, mailbox state, policy, source packets
sandbox   = user computer, conductor, VText, researcher, super, orchestration
```

The Resend API key belongs to `maild`, not `gateway` or `platformd`.

## Host State And Config

Use a separate SQLite DB and storage root:

```text
/var/lib/go-choir/mail/mail.db
/var/lib/go-choir/mail/raw/
/var/lib/go-choir/mail/attachments/quarantine/
```

Service config:

```text
MAILD_PORT=8087
MAILD_DB_PATH=/var/lib/go-choir/mail/mail.db
MAILD_STORAGE_ROOT=/var/lib/go-choir/mail
MAILD_PRIMARY_DOMAIN=choir.news
MAILD_ROOT_OWNER_ID=<founder auth user id>
RESEND_API_KEY=...
RESEND_WEBHOOK_SECRET=...
```

Node B should add:

```text
systemd service: go-choir-maild
env file: /var/lib/go-choir/maild.env
state path: /var/lib/go-choir/mail
caddy route: /api/email/resend/webhook -> 127.0.0.1:8087
proxy config: PROXY_MAILD_URL=http://127.0.0.1:8087
```

`maild` should not need `PROXY_VMCTL_URL`, sandbox credentials, gateway tokens,
or any direct agent/runtime endpoint.

Implementation checkpoint, 2026-05-26:

- `cmd/maild` and `internal/maild` own the SQLite schema, Resend webhook
  verification, Resend received-message fetch, outbound send, attachment
  quarantine metadata, and source packet rows.
- `maild` enforces alias receive policies before storing inbound messages. The
  v0 public alias accepts public inbound, while future trusted-upload-style
  aliases can require exact unlisted plus aliases and sender whitelist rows.
- Duplicate `email.received` webhook deliveries retry ingest if the provider
  message was not stored after an earlier transient failure; stored messages
  remain idempotent by provider message id.
- The authenticated message-detail API includes stored raw headers for
  owner-visible inspection. The Email app renders those headers only inside a
  collapsed Details section; plain text remains the primary v0 body surface.
- The authenticated message-detail API also returns stored To/Cc/Bcc recipient
  rows. The Email app renders those stored recipients instead of assuming the
  active root alias, which keeps plus-alias, forwarded-mail, and Sent-message
  inspection honest.
- The Email app includes a minimal owner-initiated Compose panel that sends
  plain text through the existing authenticated `/api/email/send` route with
  From fixed to `000@choir.news`. It does not add drafts, rich HTML, aliases,
  or automation.
- `internal/proxy` owns authenticated `/api/email/*` forwarding and the
  `/api/email/messages/:id/send-to-choir` compound operation.
- Successful proxy-owned Send to Choir handoff records an owner/message-scoped
  `email_ingress_events` row in `maild` after prompt-bar submission. This is
  read-only operator evidence; `maild` still never receives sandbox credentials
  or calls MAS directly.
- The authenticated source-packet route returns provenance, a stable normalized
  text ref, and the normalized plain-text email body. The proxy submits a
  bounded copy of that body into the existing prompt-bar path under explicit
  `UNTRUSTED_EXTERNAL_EMAIL` framing, while leaving attachments quarantined and
  provider-only raw-message refs out of browser-visible state.
- Node B Nix config defines `go-choir-maild`, `/var/lib/go-choir/mail`, and a
  direct Caddy route for `/api/email/resend/webhook`.
- `nix/deploy-mail-creds.sh` deploys `RESEND_API_KEY` and
  `RESEND_WEBHOOK_SECRET` to `/var/lib/go-choir/maild.env`. It can consume the
  webhook secret file generated by `scripts/mail-resend-setup` through
  `CHOIR_MAIL_WEBHOOK_SECRET_FILE=/path/to/resend-webhook-secret.env`, and
  supports `--dry-run` for redacted validation without restarting `maild`.
- `scripts/mail-provider-readiness` is the read-only provider probe for Resend
  domain/webhook status, Gandi mail-related records, public DNS, and Node B
  `maild` health. It redacts secret-shaped fields and does not mutate provider
  state.
- `scripts/mail-acceptance-check` is the read-only post-provider acceptance
  checker. It uses `maildctl` over SSH to verify a real message, attachment
  quarantine, source-packet trust label, and expected owner/message ingress
  event count.
- `scripts/mail-resend-setup` inspects Resend domains/webhooks and can, with
  `--apply`, create/enable the `choir.news` domain and create the
  `email.received` webhook. It never prints webhook signing secrets; use
  `--write-webhook-secret-file` to save one with mode `600`.
- `scripts/mail-gandi-plan-records` plans or applies Gandi LiveDNS records from
  a Resend domain JSON response. It is dry-run by default, snapshots current
  Gandi records before apply, and requires `--allow-root-mx --apply` before
  replacing the apex MX RRset.
- `scripts/mail-gandi-rollback-records` uses the same Resend domain JSON plus a
  pre-apply Gandi snapshot to restore or delete affected RRsets. It is also
  dry-run by default and requires `--allow-root-mx --apply` for root MX rollback.
- DNS/MX, real Resend setup, attachment content download, and real staging proof
  are still future steps.

## Data Flow

Inbound:

```text
Resend receives mail for choir.news
  -> Resend posts email.received to /api/email/resend/webhook
  -> Caddy routes the webhook directly to maild
  -> maild verifies raw body and signature headers
  -> maild idempotently stores webhook metadata
  -> maild fetches full message and attachment metadata from Resend
  -> maild resolves aliases and receive policy
  -> maild stores raw and normalized message state
  -> maild quarantines attachment metadata/content by default
  -> maild creates UNTRUSTED_EXTERNAL_EMAIL source packet records
  -> Email app displays through authenticated proxy APIs
```

Manual MAS handoff:

```text
owner clicks "Send to Choir" or "Summarize with Choir"
  -> browser POSTs /api/email/messages/:id/send-to-choir through proxy
  -> proxy validates owner session
  -> proxy asks maild for an owner-visible source packet
  -> maild enforces mailbox ownership and returns provenance, normalized text ref,
     and normalized plain-text source content
  -> proxy submits a conductor-style request to the resolved user computer
  -> proxy records an owner/message-scoped email_ingress_events receipt in maild
  -> sandbox/conductor receives owner instruction plus bounded untrusted source content
```

Outbound:

```text
user composes or replies in Email app
  -> browser POSTs /api/email/send through proxy
  -> proxy validates session and forwards trusted user context to maild
  -> maild validates from_alias ownership and send policy
  -> maild calls Resend Send Email API with an idempotency key
  -> maild stores outbound message in Sent
```

No inbound message may directly trigger outbound send, tool execution, or
canonical mutation in v0.

For v0, plus aliases are exact aliases. `000+anything@choir.news` must not
silently fall back to `000@choir.news`; secret upload aliases depend on exact,
rotatable local parts.

## Data Model Sketch

Use internal UUID/ULID ids for principals and message objects.

```sql
email_aliases (
  id text primary key,
  domain text not null,
  local_part text not null,
  canonical_number integer,
  target_type text not null,
  target_id text not null,
  visibility text not null,
  receive_policy_id text not null,
  created_at text not null,
  disabled_at text,
  unique(domain, local_part)
);

email_receive_policies (
  id text primary key,
  name text not null,
  allow_public_inbound integer not null,
  allow_attachments integer not null,
  require_sender_whitelist integer not null,
  require_secret_alias integer not null,
  allow_auto_agent_read integer not null,
  allow_auto_agent_write integer not null,
  allow_auto_outbound_send integer not null,
  quarantine_by_default integer not null
);

email_sender_whitelist (
  id text primary key,
  owner_id text not null,
  alias_id text not null,
  sender_address text not null,
  created_at text not null,
  disabled_at text,
  unique(alias_id, sender_address)
);

email_messages (
  id text primary key,
  provider text not null,
  provider_message_id text,
  provider_event_id text,
  direction text not null,
  mailbox_owner_id text not null,
  alias_id text,
  from_address text not null,
  from_display text,
  subject text not null,
  text_body text,
  html_body text,
  raw_headers_json text,
  raw_message_ref text,
  authentication_results_json text,
  trust_status text not null,
  read_at text,
  received_at text,
  sent_at text,
  created_at text not null
);

email_message_recipients (
  id text primary key,
  message_id text not null,
  kind text not null,
  address text not null,
  display text
);

email_attachments (
  id text primary key,
  message_id text not null,
  provider_attachment_id text,
  filename text not null,
  content_type text not null,
  content_disposition text,
  content_id text,
  size_bytes integer,
  storage_ref text,
  status text not null,
  created_at text not null
);

email_source_packets (
  id text primary key,
  message_id text not null,
  attachment_id text,
  trust_label text not null,
  provenance_json text not null,
  text_ref text,
  created_at text not null
);

email_ingress_events (
  id text primary key,
  message_id text not null,
  source_packet_id text,
  owner_id text not null,
  conductor_submission_id text,
  status text not null,
  created_at text not null,
  completed_at text
);
```

`email_drafts` can wait unless compose draft persistence is cheap and isolated.

## API Shape

Public webhook:

```text
POST /api/email/resend/webhook
```

Authenticated mailbox APIs:

```text
GET  /api/email/messages?folder=inbox|sent|quarantine
GET  /api/email/messages/:id
POST /api/email/messages/:id/read
POST /api/email/messages/:id/send-to-choir
POST /api/email/send
```

`/api/email/messages/:id/send-to-choir` is a proxy-owned compound operation. The
browser still calls the public path, but proxy performs the maild source lookup
and the user-computer conductor submission.

Admin/dev inspection should be a CLI command or localhost-only endpoint. Do not
expose raw message/admin inspection through unauthenticated public routes.

## Default Receive Policies

Public numeric address:

```text
receive email: yes
attachments: quarantine by default
agent may summarize: yes, as untrusted source after owner action
agent may write canonical state: no
agent may send outbound email: no
```

Throwaway/signup alias:

```text
receive email: yes
attachments: no or quarantine
agent may act: no
default use: signup inbox / low-trust mail
```

Trusted upload alias:

```text
receive email: yes
requires secret alias: yes
requires manually whitelisted sender: yes
attachments: yes, but sandboxed/quarantined
agent may summarize/propose filing: yes
agent may write canonical state: only through explicit policy gate
```

Newsletter alias:

```text
receive email: yes
send email: later
label as Choir/agentic
support unsubscribe before bulk sending
```

## Security Model

Assets:

- Resend API key.
- Resend webhook signing secret.
- Gandi PAT used for DNS setup.
- Raw email bodies, headers, sender metadata, and attachments.
- Internal user/account/mailbox/agent ids.
- MAS authority to create runs, write VText, send mail, use tools, or promote
  state.
- Owner privacy and message confidentiality.
- Domain reputation for `choir.news`.

Trust boundaries:

```text
Internet sender -> Resend
Resend -> public maild webhook
Browser -> proxy authenticated APIs
proxy -> maild trusted user context
maild -> Resend API
maild -> local SQLite/storage
proxy -> sandbox/MAS ingress
sandbox/MAS -> VText/researcher/super/tools
operator shell -> Gandi API / DNS
```

Prompt injection rule:

External email body, headers, links, quoted replies, and attachments are
untrusted data. They must be wrapped as source material and never concatenated
into an agent instruction channel as if they were owner/system/developer text.

Blocked in v0:

- direct tool calls from inbound mail;
- direct outbound email from inbound mail;
- direct VText/file canonical writes;
- promotion/adoption;
- disclosure of secrets or hidden prompts;
- automatic attachment extraction into canonical state.

STRIDE summary:

- Spoofing: verify webhooks; strip trusted headers at proxy; do not trust
  visible `From:` alone.
- Tampering: keep raw and normalized state separate; record policy and alias ids;
  use idempotency.
- Repudiation: record provider event ids, owner actions, policy decisions, and
  timestamps.
- Information disclosure: keep secrets in `/var/lib/go-choir/maild.env`; do not
  log bodies, attachment URLs, webhook secrets, API keys, or Gandi PAT.
- Denial of service: body caps, attachment caps, sender/alias rate limits, fast
  webhook ack after durable enqueue, bounded workers.
- Elevation of privilege: source packets are `UNTRUSTED_EXTERNAL_EMAIL`; MAS
  handoff requires owner action or later explicit policy.

## DNS And Resend Setup

External setup after code deploy:

```text
1. Verify choir.news in Resend for sending.
2. Enable Resend Receiving for choir.news.
3. Add Resend MX record at Gandi.
4. Add SPF/DKIM/DMARC records supplied by Resend.
5. Configure Resend webhook:
   https://choir.news/api/email/resend/webhook
6. Store RESEND_API_KEY and RESEND_WEBHOOK_SECRET in /var/lib/go-choir/maild.env.
```

Before any DNS mutation, run:

```bash
scripts/mail-provider-readiness
```

Expected pre-mutation output should show Resend domain/webhook records available
from a sufficiently scoped Resend key, Gandi's current mail records, public DNS,
and Node B `maild` health. If Resend returns `restricted_api_key`, the available
key cannot retrieve exact DNS records or the webhook signing secret; use a
broader temporary Resend API key or the dashboard to obtain provider truth
first.

To retrieve provider truth or create the missing Resend objects with a
sufficiently scoped key:

```bash
scripts/mail-resend-setup \
  --write-domain-json /tmp/resend-domain.json \
  --write-webhook-secret-file /tmp/resend-webhook-secret.env

scripts/mail-resend-setup \
  --apply \
  --ensure-domain \
  --ensure-webhook \
  --write-domain-json /tmp/resend-domain.json \
  --write-webhook-secret-file /tmp/resend-webhook-secret.env
```

After reviewing `/tmp/resend-webhook-secret.env`, validate and deploy it into
`/var/lib/go-choir/maild.env`:

```bash
CHOIR_MAIL_WEBHOOK_SECRET_FILE=/tmp/resend-webhook-secret.env \
  nix/deploy-mail-creds.sh --dry-run

CHOIR_MAIL_WEBHOOK_SECRET_FILE=/tmp/resend-webhook-secret.env \
  nix/deploy-mail-creds.sh
```

Do not commit the generated JSON or secret files.

Once exact provider records are available, first dry-run the Gandi plan:

```bash
scripts/mail-gandi-plan-records --records resend-domain.json
```

If the plan includes the apex `@/MX` RRset, applying requires the intentional
root-mail cutover flags:

```bash
scripts/mail-gandi-plan-records --records resend-domain.json --allow-root-mx --apply
```

The apply command writes a pre-apply Gandi snapshot. Keep that path for rollback:

```bash
scripts/mail-gandi-rollback-records \
  --snapshot /path/to/gandi-choir.news-YYYYMMDDTHHMMSSZ.json \
  --records resend-domain.json \
  --allow-root-mx
```

Root-domain receiving is intentional for the numeric-address product model. The
owner has explicitly accepted replacing Gandi root-domain mail routing because
Gandi mailboxes are not in use for Choir. If human IMAP-style mailboxes are
later required on the root domain, that is a deliberate product architecture
change, not a split-brain MX assumption.

DNS/MX mutation should happen late, after service health, webhook verification,
secret deployment, and rollback evidence are ready.

## V0 UI Direction

The provided desktop and mobile mockups are useful for tone and layout, but they
show too many features for v0. Keep the v0 app visibly mail-shaped and
Choir-native, while avoiding account-management complexity.

Desktop v0:

- App title: `Email`.
- Three-pane layout when space permits: mailbox rail, message list, message
  detail.
- Mailbox rail: Inbox, Sent, Quarantine.
- Active address display: `000@choir.news`.
- Message list: sender, subject, snippet, received/sent time, unread indicator,
  attachment indicator, trust badge.
- Message detail: From, To, Subject, Date, plain text body, collapsed headers,
  attachments list with quarantine status, trust badge.
- Primary actions: Reply, Send to Choir, Mark read.
- Compose/reply: From alias selector initially fixed to `000@choir.news`, To,
  Subject, body, Send.

Mobile v0:

- List-first design like the mobile mockup.
- Header: `All Inboxes` or `Inbox`; compact active address selector.
- Message rows with sender, subject, snippet, time, unread dot, optional
  attachment/trust indicator.
- Floating compose button is acceptable.
- Detail opens as a separate screen or full-height panel.
- Bottom prompt bar remains outside the Email app's mail workflow.

Explicit v0 cuts from the mockups:

- No search.
- No rules UI.
- No alias management UI beyond showing `000@choir.news`.
- No storage meter.
- No multi-select bulk toolbar.
- No archive or trash unless already free from the mailbox model.
- No rich HTML compose.
- No automatic attachment scanning claim unless implemented.
- No agent/rules automation UI.
- No conversation threading.
- No newsletter/bulk sending.

Visual constraints:

- Use trust labels that are short and concrete: `Public inbound`, `Trusted
  sender`, `Attachment quarantined`, `Untrusted source`.
- Do not explain email philosophy in large text blocks inside the app.
- Avoid making cards inside cards. Use panes, lists, badges, and compact detail
  sections.
- Keep density practical; this is a utility app, not a landing page.

## Acceptance Reference

V0 should ultimately prove:

- `000@choir.news` resolves to founder/root through `email_aliases`.
- A plain text email to `000@choir.news` produces a verified webhook, stored
  inbound message, and Inbox row.
- Opening the message shows From, To, Subject, Date, body, collapsed headers, and
  trust label.
- A non-whitelisted attachment appears in Quarantine and is not processed into
  canonical state.
- "Send to Choir" creates a traceable conductor-style submission where email is
  untrusted source material.
- `scripts/mail-acceptance-check --expect-ingress-events 0` passes before owner
  handoff, then `--expect-ingress-events 1` passes after owner handoff.
- Reply/send from `000@choir.news` works only after explicit owner action and
  alias ownership validation.
- No inbound message can trigger outbound send, tool execution, canonical state
  mutation, or promotion without explicit policy.
