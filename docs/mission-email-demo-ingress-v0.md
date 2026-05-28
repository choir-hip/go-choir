# MissionGradient: Email Demo Ingress v0

Last updated: 2026-05-28

Reference:

- [choir-email-reference-v0.md](choir-email-reference-v0.md)
- [mission-maild-email-ingress-v0.md](mission-maild-email-ingress-v0.md)

## Goal Prompt

```text
/goal Run docs/mission-email-demo-ingress-v0.md as a Codex-operated
MissionGradient mission: prove and harden account-scoped Choir Email as a real
demo ingress surface before depending on VText. Start from the configured
Resend/Gandi/maild state: choir.news is verified for sending and receiving,
Gandi apex MX routes to Resend inbound, Resend has an enabled email.received
webhook at https://choir.news/api/email/resend/webhook, and Node B maild has
RESEND_API_KEY plus RESEND_WEBHOOK_SECRET configured. 000@choir.news is mapped
to the yusefnathanson@me.com auth account
(5bd6de97-3b58-408c-bf89-c42c81b083de). First prove real inbound mail to
000@choir.news through deployed maild and the Email app while authenticated as
that account. Then prove owner-scoped outbound reply. Then add the smallest
account-scoped policy surface needed for verified-sender email to become
prompt-bar-equivalent ingress, while public mail remains mailbox-only. Finally
prepare the email-side handoff for the known-broken VText-backed demo response
workflow: manual "Respond with Choir" and optional 000+<code>@choir.news auto
workflow should create a precise workflow request/source-packet handoff that a
later VText repair mission can consume. Do not try to repair VText inside this
email mission; current VText is known to stop after v1 without research or
coding. Preserve invariants: numeric addresses identify destination
accounts, not sender authority; external email is data unless sender authority
is separately verified; attachments/quoted/forwarded third-party content remain
untrusted; maild cannot directly run agents, mutate canonical state, promote, or
bypass conductor; no manual deploy shortcuts; and no cleanup or feature
expansion before live email proof. After live proof works, do a deletion-first
convergence pass over dead setup paths, stale docs, demo cruft, unused helpers,
and overbuilt UI. Stop at an email-complete checkpoint with exact handoff
evidence and a follow-on VText repair mission string once the email substrate
has reached the VText boundary.
```

## Mission Status

```text
status: complete
current artifact state: provider/DNS/runtime setup, public mailbox delivery,
  attachment quarantine, owner-scoped outbound reply, manual Respond with Choir,
  trusted plus-code pending workflow handoff, and a small convergence pass all
  have deployed proof. Resend reports choir.news verified for sending and
  receiving. Gandi apex MX routes choir.news inbound mail to Resend. Resend has
  an enabled email.received webhook for
  https://choir.news/api/email/resend/webhook. Node B maild health reports
  resend_api_key_configured=true and webhook_secret_configured=true. Public
  unsigned webhook probes fail closed with invalid_signature. 000@choir.news
  resolves through email_aliases to yusefnathanson@me.com's auth user id
  5bd6de97-3b58-408c-bf89-c42c81b083de.
what shipped: docs checkpoint 0a3029b and behavior commit
  63aae2950992706e4f105ca14ef5686db71decc9. The behavior commit adds the
  trusted plus-code workflow policy, maildctl alias configuration, pending
  conductor handoff rows for verified whitelisted plus-code mail, stricter
  authentication-result evidence checks, source-packet workflow provenance, and
  Email app copy that names the owner action "Respond with Choir". Cleanup
  commit fa8a84059ad1eb7b98380233d7ad9226b4f4c726 removes the extra Email app
  read-state control, updates stale Send to Choir wording, and records the
  deployed live evidence.
what was proven: readiness was re-probed; Resend loopback mail to 000@choir.news
  stored message resend-message-f20817a211067c9ef9fc1180a0aa86a9 and source
  packet resend-source-packet-b16502d434ef415f55e1fd9985ce1e0e; authenticated
  Email app showed it for the 5bd6de97-3b58-408c-bf89-c42c81b083de account;
  owner reply stored outbound resend-message-cde5994a8c26afe3826552207eb9c77e
  with provider id 53be4e8d-141d-46f0-95f8-edcdce9fb095 and looped back through
  inbound as resend-message-6d5af73adb98a6f21bbc0ae9690beb8f; direct SMTP to
  Resend MX stored public message resend-message-e6425095af768af20a75e665023f5f6c
  from external-probe@example.net with SPF/DMARC failure evidence and no ingress
  event; owner-triggered response handoff for that SMTP message created
  conductor submission 6b50c89e-3bdb-4d0b-8d2e-82d4f22dd8fb and maild ingress
  event email-ingress-event-f0f0e81661c373c1e08ef74e63a5469a.
unproven or partial claims: true external human-sender demo with the final
  chosen invite alias and downstream VText response generation. The plus-code
  email side intentionally stops at a pending_conductor handoff because VText is
  known broken after v1.
highest-impact uncertainty: VText response generation, which is deliberately a
  separate mission because the current VText flow stops after v1.
next executable probe: run the follow-on VText repair mission from the exact
  email message/source-packet/handoff evidence below.
```

## Run Checkpoint & Resumption State

```text
status: complete
last checkpoint: 2026-05-28 email-complete deployed checkpoint
current artifact state: 000@choir.news is account-scoped to
  5bd6de97-3b58-408c-bf89-c42c81b083de. Provider DNS/webhook/runtime is live.
  Public inbound mail stores as public/untrusted mailbox state. Attachment mail
  stores in Quarantine with quarantined metadata. Owner UI reply sends through
  Resend and stores Sent. Owner-triggered Respond with Choir uses the product
  prompt-bar path and records a maild ingress receipt. Verified whitelisted
  plus-code mail creates a pending_conductor email_ingress_events row and a
  source packet annotated as prompt_bar_equivalent without directly running
  agents from maild.
what shipped:
  - 0a3029b docs: checkpoint email demo ingress proof
  - 63aae2950992706e4f105ca14ef5686db71decc9 feat: add trusted email workflow
    handoff
  - fa8a84059ad1eb7b98380233d7ad9226b4f4c726 chore: converge email demo
    ingress
what was proven:
  - scripts/mail-provider-readiness passed for Resend/Gandi/Node B maild except
    local .env intentionally lacks RESEND_WEBHOOK_SECRET while Node B has it.
  - unsigned webhook negative probe returned invalid_signature without counter
    mutation.
  - Resend loopback subject CHOIR_MAIL_RESEND_LOOP_20260528T003946Z_43848
    produced webhook msg_3EKYaAzdBz5fvLkmnNyKtznWNh9 and owner-visible message
    resend-message-f20817a211067c9ef9fc1180a0aa86a9.
  - owner UI reply produced outbound provider id
    53be4e8d-141d-46f0-95f8-edcdce9fb095 and inbound webhook
    msg_3EKZDGJDMHLwwezn7pyNzCTyakR.
  - direct SMTP subject CHOIR_MAIL_SMTP_NODEB_20260528T004717Z_6736 was accepted
    by inbound-smtp.us-east-1.amazonaws.com with SES queue id
    kbqn05hld91lnvjddl8s8oti534mn22fi87pr8o1, stored as
    resend-message-e6425095af768af20a75e665023f5f6c, and showed SPF/DMARC fail
    while remaining Public inbound and UNTRUSTED_EXTERNAL_EMAIL.
  - manual Respond with Choir for the SMTP message created source packet
    resend-source-packet-2547f93e723e0e87a26db69218240152, conductor submission
    6b50c89e-3bdb-4d0b-8d2e-82d4f22dd8fb, and ingress event
    email-ingress-event-f0f0e81661c373c1e08ef74e63a5469a.
  - CI run 26548131552 and Publish/FlakeHub run 26548131531 passed for
    63aae2950992706e4f105ca14ef5686db71decc9; choir.news /health reported
    proxy and upstream deployed commit
    63aae2950992706e4f105ca14ef5686db71decc9.
  - trusted alias 000+invite-smoke@choir.news was configured for owner
    5bd6de97-3b58-408c-bf89-c42c81b083de and whitelisted sender
    000@choir.news; authenticated Resend message
    CHOIR_MAIL_TRUSTED_ALIAS_20260528T010441Z_22093 stored as
    resend-message-4ed741f34c2ba7ca0bf5be428e5204b6 with webhook
    msg_3EKbcA7xgqQEcRrThaasEbrqsVL and pending ingress event
    email-ingress-event-79ea06b1a4d3385dd3282525efc44fbc.
  - that trusted source packet
    resend-source-packet-e48931de6748b4cc38d0871c34adfc1a recorded
    prompt_bar_equivalent=true, sender_authority=verified_sender_policy,
    receive_policy_id=policy-trusted-workflow-v0, and
    workflow_handoff_status=pending_conductor.
  - direct SMTP negative probe
    CHOIR_MAIL_PLUS_NEGATIVE_20260528T010516Z_13911 to the plus-code alias was
    rejected with "passing authentication results required" and created no
    message or ingress event.
  - live Resend attachment message
    CHOIR_MAIL_ATTACHMENT_QUARANTINE_20260528T010950Z_25800 stored as
    resend-message-3d1dbb3525ec84f0bb75ff571d07d8bd in Quarantine with
    provider event msg_3EKcErsUxMa8NIo5C4S1bzlJEcR. Attachment
    resend-attachment-794476ee544e01f1d1f560957c6869f7
    (choir-quarantine-proof.txt, text/plain, 94 bytes) has status
    quarantined. Source packet
    resend-source-packet-a5372759fb72debad78b2726e4209801 records
    prompt_bar_equivalent=false and attachment_count=1. No ingress events
    exist for this message. The deployed Email app showed it in Quarantine with
    the attachment marked quarantined.
  - convergence commit fa8a84059ad1eb7b98380233d7ad9226b4f4c726 passed CI run
    26548606027, Publish/FlakeHub run 26548606065, and the Node B staging
    deploy job. Public /health reported proxy and upstream deployed commit
    fa8a84059ad1eb7b98380233d7ad9226b4f4c726. Node B maild health remained ok
    with resend_api_key_configured=true and webhook_secret_configured=true.
  - after that deploy, scripts/mail-acceptance-check passed for
    resend-message-3d1dbb3525ec84f0bb75ff571d07d8bd with folder=quarantine,
    one quarantined attachment, prompt_bar_equivalent=false, and zero ingress
    events. The deployed Email app showed Quarantine with the attachment and
    only Reply plus Respond with Choir actions.
unproven or partial claims:
  - VText response generation remains known broken after v1 and is outside this
    email mission.
  - the eventual in-person demo needs the final invite alias/sender policy
    chosen and configured; 000+invite-smoke is a proof alias, not product copy.
belief-state changes:
  - provider payload shape, webhook signature, alias resolution, mailbox
    storage, authenticated Email app visibility, owner reply, attachment
    quarantine, and plus-code policy handoff are no longer the primary
    uncertainties.
  - the live gap is now downstream VText repair, not mail transport,
    mail-side authority topology, or cleanup convergence.
remaining error field:
  - final demo alias policy choice and separate VText repair.
highest-impact remaining uncertainty:
  - why VText stops after v1 before research/coding, and how to return a VText
    response artifact to the Email app/send pipeline.
next executable probe:
  - start the VText repair mission using the exact email evidence ids below.
suggested resume goal string:
  /goal Run a VText Response Workflow Repair mission from the email-complete
  checkpoint in docs/mission-email-demo-ingress-v0.md. Use proven manual
  response handoff message resend-message-e6425095af768af20a75e665023f5f6c,
  source packet resend-source-packet-2547f93e723e0e87a26db69218240152,
  conductor submission 6b50c89e-3bdb-4d0b-8d2e-82d4f22dd8fb, and ingress event
  email-ingress-event-f0f0e81661c373c1e08ef74e63a5469a; also use proven
  trusted plus-code pending handoff message
  resend-message-4ed741f34c2ba7ca0bf5be428e5204b6, source packet
  resend-source-packet-e48931de6748b4cc38d0871c34adfc1a, and ingress event
  email-ingress-event-79ea06b1a4d3385dd3282525efc44fbc. Repair the known VText
  failure where VText reaches v1 but does not continue into research or coding.
  The repaired flow must let an owner-controlled response prompt read an email
  source packet, optionally research/code, produce a VText reply artifact, and
  return that artifact to the Email app/send pipeline with Trace evidence. Do
  not change maild provider/DNS behavior unless this mission identifies a
  specific email-side bug.
evidence artifact refs:
  - maildctl stats/messages/source-packet/ingress-events on Node B;
  - Comet authenticated Email app UI observation on choir.news;
  - Resend/Gandi readiness probes from scripts/mail-provider-readiness.
rollback refs:
  - disable Resend webhook b9cdac0e-db4b-4a63-a6c3-a046716090dd;
  - remove RESEND_WEBHOOK_SECRET from /var/lib/go-choir/maild.env to fail
    webhook closed;
  - restore Gandi apex MX away from Resend inbound if account mail routing must
    roll back.
```

## Mission Frame

This mission is not "build a full email client." It is a live demo path:

```text
person met in the real world
  -> emails a numbered account address
  -> owner sees it in Choir Email
  -> owner can reply normally
  -> owner can later ask Choir to produce a response from an owner-controlled
     prompt/VText workflow
  -> an optional special plus-code alias can auto-run that narrow workflow
```

The mission should make the email substrate real before depending on VText.
VText response generation is currently known broken: it reaches v1 and does not
continue into research or coding. Therefore, the email mission should stop at a
well-evidenced email-to-VText handoff boundary and produce a follow-on VText
repair mission, not mix VText repair into the email substrate work.

## Mobile Responsiveness Finding

Recorded: 2026-05-28 after the email-complete checkpoint, before committing the
responsive UI fix.

Evidence:

- A 390px-wide Playwright mobile harness against `EmailApp.svelte` showed the
  existing mobile CSS stacked message list and detail vertically, hid the
  mailbox rail, and left no mobile-specific mailbox navigation.
- Opening a message on mobile needed a list/detail mode switch rather than a
  cramped stacked desktop layout.
- Long provider-marker subjects in the message reader needed explicit wrapping
  to stay inside the phone viewport.

Required fix:

- Keep the desktop three-column layout unchanged.
- On phone widths, show a top address/mailbox control, keep the message list as
  the initial view, open message detail/compose as a full-width panel, and
  provide a Back action to return to the list.
- Verify no horizontal overflow at 390px and prove list/detail/compose states
  with screenshots before deploy.

## Real Artifact

An account-scoped email ingress and response substrate:

```text
<number>@choir.news
  -> email_alias
  -> mailbox/account owner UUID
  -> account's Email app
  -> owner-scoped reply or conductor ingress
```

And, after the substrate is proven:

```text
<number>+<code>@choir.news
  -> exact alias policy
  -> owner-controlled workflow prompt/VText reference
  -> account-scoped conductor run
  -> generated VText response artifact
  -> outbound reply to the original sender, if policy allows
```

`000@choir.news` is one user's mailbox, not a platform-global mailbox.
In the current deployment, that user is the `yusefnathanson@me.com` auth account
with owner id `5bd6de97-3b58-408c-bf89-c42c81b083de`.

## Hard Invariants

- Numeric local parts identify destination accounts; they never authorize the
  sender.
- Sender authority is separate from delivery. A sender is authorized only when
  message authentication evidence, configured identity mapping, and mailbox
  policy all agree.
- Public/unverified inbound mail is mailbox-only by default.
- Verified owner/delegate inbound mail may become prompt-bar-equivalent ingress
  for the destination account.
- Plus-code aliases are scoped capabilities, not general auth secrets.
- External email body, headers, attachments, quotes, forwards, links, and remote
  HTML are untrusted source material unless separately verified.
- A verified sender's own plain request may be treated like prompt-bar input;
  embedded third-party material still remains untrusted evidence.
- `maild` may verify, store, classify, send through Resend, and create source
  packets. It may not directly run agents, mutate canonical VText/files,
  promote state, or bypass conductor.
- Automatic code-alias workflows may reply only within explicit policy. They
  may not send arbitrary outbound mail, process attachments by default, or use
  inbound content as system/developer instruction.
- Provider and webhook secrets stay outside git, frontend bundles, Trace, VText,
  and user-visible source packets.
- Staging/deployed proof is required for behavior claims. Local proof is only
  shaping evidence.
- Cleanup starts after live email proof, not before.

## Value Criterion

Minimize the distance between "someone emails my numbered Choir address" and
"my account can inspect, answer, or deliberately process that email through
Choir" while preserving account authority and untrusted-source boundaries.

The artifact moves uphill when:

- real provider events replace simulated events;
- account scoping becomes explicit and testable;
- public inbound and verified-sender command ingress diverge by policy, not by
  ad hoc code paths;
- the demo becomes simpler and more deletion-shaped after proof;
- every privileged transition leaves durable evidence.

## Quality Target

`solid`, with a deliberate `minimal` slice for the first live proof.

The first live proof may be narrow: one plain text email to `000@choir.news`.
After that, the code should converge rather than expand: remove dead setup
paths, duplicate scripts, stale assumptions, and overbuilt UI only after the
real path is known.

## Belief State

Current beliefs:

- Provider setup is no longer the primary blocker.
- The next likely failures are payload-shape mismatch, webhook routing, alias
  resolution, owner ID mismatch, message storage, auth/proxy visibility, or UI
  assumptions.
- Existing Email app and maild code may contain scaffolding that was useful
  before provider setup but should be deleted once live proof identifies the
  real path.
- VText response generation is desirable for the demo, but current VText flow is
  known broken after v1. Email should prove its substrate and handoff shape
  first, then a separate VText mission should repair the downstream flow.

Highest-impact uncertainty:

```text
Can the now-proven email substrate be simplified without removing rollback
tools, acceptance probes, or evidence needed by the follow-on VText mission?
```

Next observation:

```text
Run the convergence diff through focused tests, build proof, deployed identity,
and one read-only mail acceptance check against the live quarantined attachment.
```

## Homotopy Axes

Increase realism along these axes without changing the object's topology:

1. **Provider realism:** fake webhook -> real signed Resend webhook -> real
   sender/authentication evidence -> attachment-bearing message.
2. **Account realism:** hardcoded `000` -> explicit owner UUID mapping ->
   multiple numeric aliases in tests.
3. **Authority realism:** public mailbox-only -> verified sender prompt-bar
   equivalent -> scoped plus-code automation.
4. **Response realism:** normal manual reply -> owner-triggered workflow handoff
   -> VText repair mission consumes the handoff -> VText-backed response ->
   optional automatic plus-code reply.
5. **Cleanup realism:** keep scaffolding while searching -> delete dead paths
   after proof -> shrink UI/docs/scripts to the durable shape.

Forbidden shortcut: do not replace live provider proof with local fake webhooks
once provider setup is available.

## Control Intervals

### Lambda 0: Reconfirm Readiness

Objective: ensure the mission starts from true provider/runtime state.

Actions:

- Run `scripts/mail-provider-readiness`.
- Confirm `maildctl aliases` maps `000@choir.news` to
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, the auth user for
  `yusefnathanson@me.com`.
- Confirm Gandi authoritative and public MX for `choir.news`.
- Confirm Resend domain status and receiving capability.
- Confirm Resend webhook endpoint, status, and `email.received` event.
- Confirm Node B `go-choir-maild` health reports both Resend key and webhook
  secret configured.
- Confirm public unsigned webhook returns `invalid_signature` and does not store
  counters.

Evidence:

- Command outputs with secrets redacted.
- Node B service PID/start time.
- Public `/health` deployed commit identity.

Stop/fix if:

- readiness has regressed;
- webhook secret missing;
- Resend API key cannot inspect webhook/domain;
- DNS no longer points inbound to Resend.

### Lambda 1: Real Plain Inbound

Objective: prove the minimum live path.

Actions:

- Send a plain text email from a real external mailbox to `000@choir.news`.
- Watch Resend webhook delivery status if available.
- Inspect Node B maild logs around the delivery window.
- Use `maildctl` or a read-only checker to verify:
  - webhook event row;
  - provider message id;
  - resolved recipient alias;
  - account owner id;
  - normalized message row;
  - source packet row marked `UNTRUSTED_EXTERNAL_EMAIL`;
  - zero ingress events if public sender is not verified.
- Open the deployed Email app and prove the message appears and opens.

Evidence:

- Sender address, recipient, subject marker, timestamp.
- Webhook event id and provider message id.
- Message id/source packet id.
- Screenshot or DOM/API proof of Email app list/detail.

Stop/fix if:

- Resend does not receive mail;
- webhook signature verification fails for real Resend traffic;
- message fetch from Resend fails;
- alias resolution fails;
- message stores but UI cannot see it.

### Lambda 2: Attachment Quarantine

Objective: prove public inbound remains safe when it carries a file.

Actions:

- Send one small attachment to `000@choir.news`.
- Verify message appears.
- Verify attachment metadata is stored and status is `quarantined` or `blocked`.
- Verify no attachment text is promoted into prompt-bar, VText, canonical files,
  or tool calls.

Evidence:

- Message id.
- Attachment id, filename, content type, size, status.
- Checker output proving no automatic ingress/tool/outbound action.

### Lambda 3: Outbound Reply Primitive

Objective: prove the account can reply through Resend before adding generated
responses.

Actions:

- From the Email app or authenticated API, reply from `000@choir.news` to the
  sender.
- Verify Resend accepts send.
- Verify Sent row is stored.
- Verify reply threading headers when replying to a received message.

Evidence:

- Outbound provider message id.
- Sent message row.
- Recipient mailbox observation if available.

### Lambda 4: Account-Scoped Sender Authority

Objective: split "mail delivered to the account" from "sender may command the
account computer."

Actions:

- Define the minimal data/config needed to map verified senders or delegates to
  an account/mailbox.
- Prove SPF/DKIM/DMARC/authentication results are evidence, not authority by
  themselves.
- Add tests for:
  - verified auth results but no account policy -> no MAS ingress;
  - account policy but failed/missing auth evidence -> no MAS ingress;
  - account policy plus verified sender -> prompt-bar-equivalent ingress
    allowed.

Evidence:

- Store/API tests.
- Maild policy decision records.
- No public/unverified auto-run.

### Lambda 5: Prompt-Bar-Equivalent Email Ingress

Objective: verified sender email can ask the account computer to do work, like
the prompt bar.

Actions:

- Route verified-sender email into the existing conductor/prompt-bar path under
  that account.
- Preserve provenance:
  - message id;
  - source packet id;
  - sender;
  - auth results;
  - recipient alias;
  - policy id/decision;
  - exact untrusted source sections.
- Ensure `maild` does not call agents directly. Proxy/conductor owns the run.
- Prove public inbound creates no automatic run.

Evidence:

- Trace trajectory/submission ids.
- Email ingress event row.
- Prompt/conductor evidence showing account-scoped run.
- Negative public-inbound proof.

### Lambda 6: Manual "Respond With Choir" Handoff

Objective: create the owner action and workflow request boundary for the demo
without trying to repair the known-broken VText flow in this mission.

Actions:

- Add or verify a minimal owner action in Email app: "Respond with Choir".
- The action should start an account-scoped response workflow with:
  - selected message/source packet;
  - selected owner-controlled workflow prompt or VText reference;
  - reply target constrained to the original sender.
- Stop at durable handoff evidence if VText reaches v1 and stalls before
  research/coding.

Evidence:

- UI/API proof that the selected message creates the workflow request.
- Source packet id, workflow request id, selected prompt/VText reference, and
  reply target.
- Exact VText doc/revision/run/Trace failure only as downstream boundary
  evidence, not as a repair target for this mission.

### Lambda 7: Plus-Code Auto Workflow

Objective: prove the magic demo path under a narrow scoped capability.

Actions:

- Configure a rotatable alias like `000+invite-<code>@choir.news`.
- Policy:
  - exact alias match required;
  - no attachment processing;
  - rate limited by alias and sender;
  - loop suppression for auto-submitted/list/bounce mail;
  - outbound may reply only to the original sender;
  - workflow prompt/VText reference is owner controlled;
  - inbound body is user input/source data, not system instruction.
- Send a real email to the plus-code alias.
- Verify automatic workflow starts only for the plus-code alias.
- If the automatic workflow reaches the VText boundary, stop with durable
  email-side handoff evidence and the known VText v1-stall evidence.

Evidence:

- Alias policy id.
- Inbound message id.
- Policy decision record.
- Workflow run/Trace id or blocker.
- Outbound reply id if fully successful.

### Lambda 8: Deletion-First Convergence

Objective: remove scaffolding after the live path is known.

Rules:

- No new product features.
- No speculative abstractions.
- Prefer deletions greater than additions.
- If additions exceed deletions by more than 500 lines, stop and justify.
- Remove only code/docs/scripts proven unused or stale by the live path.

Likely targets:

- stale provider-readiness assumptions now superseded by configured provider
  state;
- duplicate setup scripts or docs that contradict the actual Resend/Gandi
  workflow;
- fake/demo data paths not used by tests;
- overbuilt Email UI controls not needed for the live demo;
- old "owner click Send to Choir only" language where verified sender ingress is
  now the intended product model.

Evidence:

- Diffstat.
- Focused tests.
- Staging deploy proof for behavior-changing cleanup.
- Updated docs with current state, not historical setup noise.

## Acceptance Tests

### A. Public Mailbox Acceptance

```text
Given a public sender emails 000@choir.news
When Resend delivers email.received to maild
Then maild stores the message and source packet for the 000 account
And the Email app lists and opens the message
And no automatic conductor/MAS run starts
And attachments are quarantined by default
```

### B. Owner Reply Acceptance

```text
Given the owner opens the received message
When the owner replies from 000@choir.news
Then Resend accepts the outbound message
And maild stores a Sent row
And reply headers link the outbound message to the inbound message
```

### C. Verified Sender Command Acceptance

```text
Given a sender is mapped as an authorized owner/delegate for the 000 mailbox
And the message authentication evidence satisfies policy
When that sender emails 000@choir.news
Then the email becomes account-scoped prompt-bar-equivalent ingress
And the run records email provenance
And public/unverified senders still do not auto-run
```

### D. Plus-Code Demo Acceptance

```text
Given alias 000+invite-<code>@choir.news is configured for the owner's invite
workflow
When a person emails that exact alias
Then the narrow workflow starts automatically
And inbound content is passed as user/source input
And attachments remain unprocessed
And the workflow may reply only to the original sender
And the email side creates a durable workflow handoff to owner-controlled
prompt/VText content
And the known VText v1-stall is recorded as a downstream blocker for the
separate VText repair mission
```

### E. Cleanup Acceptance

```text
Given live email proof exists
When the convergence pass runs
Then obsolete setup/doc/UI/test scaffolding is deleted or simplified
And the diff is net-negative or near-even
And no proven behavior regresses
```

## Dense Feedback Signals

- Resend domain/webhook status.
- Gandi DNS authoritative and public records.
- Node B `go-choir-maild` health and logs.
- `email_webhook_events` count and rows.
- `email_messages` rows by provider id and owner.
- `email_source_packets` rows and trust labels.
- `email_attachments` statuses.
- `email_ingress_events` for MAS handoff.
- Trace trajectory/submission ids for prompt-bar-equivalent runs.
- Email app screenshot/DOM proof on desktop and mobile if UI changes.
- Outbound Resend message id and Sent row.

## Anti-Goodhart Rules

- Do not count local fake webhooks as live provider proof.
- Do not claim email works from Resend dashboard verification alone.
- Do not treat SPF/DKIM/DMARC pass as user/account authorization.
- Do not implement VText workaround logic inside maild.
- Do not turn plus-code aliases into unrestricted prompt-bar bypasses.
- Do not auto-process attachments for the demo.
- Do not keep adding UI panels to explain unproven machinery.
- Do not start cleanup before the live path is proven.

## Rollback Policy

Provider/DNS rollback:

- Gandi MX can be restored to the previous Gandi mail records if Resend inbound
  breaks account delivery.
- Resend webhook can be disabled or deleted if it posts malformed/unsafe events.
- `RESEND_WEBHOOK_SECRET` can be removed from `maild.env` to return webhook
  route to fail-closed.

Code rollback:

- Behavior-changing commits must follow the repo landing loop:
  commit -> push origin main -> CI -> staging deploy -> deployed identity ->
  acceptance proof.
- Cleanup commits should be small and separately revertible.

Product rollback:

- Plus-code aliases must be disableable without code deploy.
- Auto workflows must have a kill switch or policy disable state.

## Stopping Conditions

`complete` only when:

- public inbound, attachment quarantine, outbound reply, verified-sender
  prompt-bar-equivalent ingress, plus-code workflow, and deletion-first cleanup
  are all proven or explicitly marked out of scope by a superseding mission; and
- final docs reflect the live architecture and residual risks.

`checkpoint_incomplete` when:

- email substrate and email-to-VText handoff are proven, with VText repair still
  downstream;
- or a useful subset is deployed/proven and the next executable probe is clear.

`blocked_incomplete` when:

- an external provider/account issue prevents live email proof after direct
  provider/DNS/log probes;
- or the known VText v1-stall blocks the response workflow before the email-side
  handoff can be proven.

`superseded` when:

- evidence shows the desired demo should not depend on VText, or requires a
  different product architecture than account-scoped email ingress.

## Suggested Follow-On VText Mission Trigger

After Lambda 6 or Lambda 7 proves the email-side handoff, create a separate
mission with this shape:

```text
/goal Run a VText Response Workflow Repair mission from the email-complete
checkpoint in docs/mission-email-demo-ingress-v0.md. Use proven manual response
handoff message resend-message-e6425095af768af20a75e665023f5f6c, source packet
resend-source-packet-2547f93e723e0e87a26db69218240152, conductor submission
6b50c89e-3bdb-4d0b-8d2e-82d4f22dd8fb, and ingress event
email-ingress-event-f0f0e81661c373c1e08ef74e63a5469a; also use proven trusted
plus-code pending handoff message resend-message-4ed741f34c2ba7ca0bf5be428e5204b6,
source packet resend-source-packet-e48931de6748b4cc38d0871c34adfc1a, and ingress
event email-ingress-event-79ea06b1a4d3385dd3282525efc44fbc. Repair the known
VText failure where VText reaches v1 but does not continue into research or
coding. The repaired flow must let an owner-controlled response prompt read an
email source packet, optionally research/code, produce a VText reply artifact,
and return that artifact to the Email app/send pipeline with Trace evidence. Do
not change maild provider/DNS behavior unless this mission identifies a specific
email-side bug.
```
