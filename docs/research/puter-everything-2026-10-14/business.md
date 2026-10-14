# Business group review: crm, hiring, help-desk, legal, contracts, signatures, survey, time-tracking, shifts

Reviewer group: business. Clone: `puter-everything/repo/src/<app>` (paths below are relative to that).
Method: I read READMEs, every `package.json` and lockfile license field, the domain/engine modules
(`core/`, `lib/`, `shared/`, `server/`, `workers/`) and sampled tests. Nothing was executed.

## Group-wide findings

**Stack, identical in all nine apps.** Vanilla JS (no framework), Vite, Vitest, Lucide icons (ISC),
and Puter.js as the only backend (`puter.kv`, `puter.fs`, `puter.ai`, `puter.workers`,
`puter.hosting`, `puter.email`). Runtime third-party dependencies are few: `pdf-lib` (MIT, signatures),
`pdfjs-dist` (Apache-2.0, contracts and signatures), `qrcode-generator` (MIT, survey).
Non-MIT licenses: Lucide is ISC, pdf.js is Apache-2.0, tslib is 0BSD, pako is MIT+Zlib. The only
MPL-2.0 packages (about 12 per app) are build-time (`lightningcss` under Vite) and `axe-core`
(help-desk, test only). **No GPL/AGPL and no license problem.**

**Quality is much better than "vibe slop".** Size per app is 13k to 36k lines (src plus tests). Every
app has a pure, DOM-free domain layer with real behavioral tests: 200 to 555 `it()` cases per app.
Help-desk tests drive the HTTP backend through its routes. Signatures tests run both workers the way
the Puter runtime runs them and measure the text drawn into flattened PDFs. The code is clearly
machine-generated over many iterations: there is one authorial voice across apps, and comments narrate
past bugs ("used to become...", "said out loud rather than quietly emptying"). But it is disciplined:
integer cents, `YYYY-MM-DD` strings to avoid time-zone drift, input normalizers on every load,
undo and trash, escaping, and rate limits.

**The cost of how it was built is duplication.** No code is shared between apps. I found 5
independent CSV parsers, 4 iCalendar writers, and 9 separate sets of dom/toast/modal/theme helpers.
Only `hiring/src/lib/dom.js` and `shifts/src/lib/dom.js` are byte-identical. Each app also re-solves
the same storage problems on its own: an offline outbox, multi-device reconciliation, snapshots and
backups, and a per-owner deployed worker.

**Two structural limits that Choir removes:**
1. **No always-on process.** Puter workers only answer requests, so anything scheduled runs only
   while the owner has a browser tab open:
   - Signature reminders: `signatures/src/ui/actions.js:351` `runTimers`.
   - The help-desk email inbox is polled every 2 minutes only while the app is open
     (`help-desk/src/client/mailbox.js`).
   - CRM "timed nudges while a tab is open" (README).
   - Contracts admits that its only alerting is a badge plus an exported `.ics` file
     (`contracts/src/lib/ical.js` header).

   A persistent Choir computer with an app desk fixes this whole class of problem. **This is the
   strongest argument for "app as agent" in this group.**
2. **No shared state between apps.** Each app has its own private Puter store and its own idea of a
   person or company. The same human is held separately as a CRM contact, an ATS candidate, a
   help-desk requester, a legal contact, a contract counterparty, a signer, a time-tracking client
   and a shift employee. This is exactly what the object graph is for (see the group summary).

**Repeated pattern worth naming: "owner-hosted public edge".** Signatures, survey, help-desk and
legal each deploy a small worker into the owner's own account. It serves an unauthenticated or
capability-link API for outsiders: signers, respondents, help-center customers, invited firm members.
It stores only SHA-256 hashes of tokens, keeps a SHA-256 admin secret baked into the source, and
enforces caps. In Choir terms this is a `choir.public_route` plus a desk-owned handler, and these
workers are a good spec for it.

### Object-graph conventions used below

The existing graph (`internal/objectgraph/object.go`) gives each object:
- `obj:<kind>:<owner>:<suffix>` ids, with an identity mode of either external key or content hash;
- a JSON `Metadata` field, a `Body`, `VersionID`, `SupersededBy` and `Tombstone`;
- typed edges.

Dolt adds commits, history and branches. Kinds and edges I reuse, from
`internal/objectgraph/registry.go`:
- Kinds: `choir.texture_document`/`texture_revision`, `choir.artifact_blob`,
  `choir.artifact_manifest`, `choir.consent_record`, `choir.review_record`,
  `choir.verifier_attestation`, `choir.web_capture`, `choir.source_entity`, `choir.work_item`,
  `choir.channel_message`, `choir.inbox_delivery`, `choir.source_schedule`, `choir.publication`,
  `choir.public_route`, `choir.provenance_*`.
- Edges: `references`, `cites`, `derived_from`, `contains`, `contains_blob`, `has_version`,
  `supersedes`, `granted_consent`, `consent_for`, `authored_review`, `reviews`, `attests_to`,
  `routes_to`.

Following the Autopaper design rule "existing types only unless we need to", I propose **a small
set of new cross-app kinds**. Each is needed because nothing existing fits, and each is shared by
three or more apps:

| Proposed kind | Used by | Identity |
|---|---|---|
| `choir.party` (person or organization; `party_kind`, names, emails, phones, domains, `org` link) | all nine | external key |
| `choir.time_entry` (minutes, date, rate snapshot, billable, `billed_by`) | legal, time-tracking, shifts (attendance), help-desk (time on ticket) | external key, versioned |
| `choir.invoice` (lines snapshotting entries, integer cents, payments as children) | legal, time-tracking, crm (recurring deal value) | external key, versioned |
| `choir.ledger_txn` (append-only money movement; never edited, only reversed) | legal trust accounting, invoice payments | content hash |
| `choir.deadline` (date, kind, owner party, source object, alarms) | contracts, legal (statute of limitations, court dates), hiring (offer expiry), signatures (expiry), crm follow-ups | external key |
| `choir.record_event` (one typed change on any business object: field diffs, actor, via) | help-desk ticket trail, crm stage history, hiring timeline, signatures audit | content hash |

Everything else stays app-specific: ticket, deal, candidate, matter, contract, envelope, survey,
shift. Edges point at the shared kinds.

**Privacy rule for the whole group:** almost all of this is personal data about third parties.
Default is **private to the computer**. Publish to the Autopaper object graph only reusable,
non-personal artifacts: templates, rubrics, KB articles, clause libraries, survey designs, aggregate
results the owner chooses to release.

**Dolt caveat (important):** Dolt history makes erasure hard. Hiring has GDPR-style retention purges
(`hiring/src/core/retention.js`) and help-desk has hard delete. A versioned graph needs one of two
things for party bodies and attachments: crypto-shredding (a per-party key, encrypted bodies,
destroy the key to erase), or a history-rewrite path. Otherwise "delete for good" is false. This
should be designed before any of these apps lands.

---

## 1. crm (Sales CRM)

**1. What and stack.** A founder-scale sales CRM: leads, companies, deals on a stage board,
follow-ups, activity log, playbooks, forecast, saved views, command palette, CSV import, `.ics`
export, and a light AI assist. 17.8k src lines and 6.0k test lines (555 cases). Runtime dependency
is `lucide` only.
- Storage is one KV key per collection holding whole arrays (`crm/src/lib/store.js:1-48`,
  2,323 lines).
- Offline outbox with base-value conflict detection: `crm/src/lib/outbox.js`.

**2. Quality.** Solid CRUD with careful details:
- Stage history per deal and recurring-deal contract value (`lib/forecast.js`, `lib/recurring.js`).
- Exact-after-normalization duplicate detection (`lib/dupes.js`).
- Forecast weighting with a confidence override per deal.

The AI (`lib/ai.js`) is three button-press prompts: paste-to-fields, draft an email, summarize a
timeline. Parsing is defensive and nothing is saved automatically. There is no engine here. The
`store.js` monolith of whole-collection writes would not scale past hundreds of records, which the
author admits.

**3. Worth taking.** **Mostly a schema and ideas; this is generic CRUD.** Worth keeping:
- the deal schema (`store.js:9-35`, with stageHistory, close reason, billing/term, per-deal
  confidence);
- the forecast math in `lib/forecast.js` (small, pure, tested);
- the "going cold" and "no next step" attention rules (`lib/attention.js`);
- playbooks as cadence templates (`lib/playbooks.js`).

No UI code is worth taking.

**4. App-agent design.**
- *Objects:*
  - `choir.party` holds contacts and companies; CRM-only fields go in metadata under `crm.*`.
  - `choir.crm_deal` (versioned, external key) has edges `references` to parties (with a role in
    the edge metadata) and to the company party.
  - Stage changes become `choir.record_event` objects. Stage history is not stored as an array: it
    is the deal's Dolt history plus these events.
  - Activities (call, meeting, email, note) are `choir.record_event` objects that `references` the
    party and the deal. An email activity links to the Mail app's `choir.channel_message`/
    `choir.inbox_delivery` rather than copying the text.
  - Follow-ups are `choir.work_item` objects, or `choir.deadline` objects when they are just dates.
  - Playbooks are template objects.
- *Versioning for free:* undo, the 30-day trash (tombstones), the pre-import backup (a Dolt commit
  or branch), and "what did the pipeline look like on Sept 1" (time-travel query) all replace
  hand-written code: `lib/backups.js`, the outbox, and the trash collection. **Branches** give
  forecast scenarios ("what if these three deals slip").
- *Direct manipulation:* drag a deal across stages, edit fields, log an activity, merge duplicates.
- *Verbs:* `upsert_party`, `merge_parties`, `create_deal`, `move_deal(stage, reason)`,
  `log_activity`, `schedule_followup`, `apply_playbook`, `forecast(range, scenario_branch)`,
  `list_attention()`.
- *Calls:*
  - research (enrich a party from its web presence; each fact is a `choir.web_capture` cited from
    the party);
  - Mail (draft and send; replies come back as activities);
  - Calendar (follow-ups);
  - Texture (proposals and recap documents that `references` the deal).
- *Why an agent:* a CRM fails because nobody types into it. An always-on desk that watches the
  Mail app and Calendar can log activities, notice cold leads, and draft the nudge without being
  asked. The owner then only approves.
- *Publish vs private:* the parties, deals and activities are private. Playbook templates and
  stage definitions could be published.

**5. Priority: low.** It is generic CRUD. Its value is the shared `choir.party` and the
"log activities from Mail automatically" behavior, which belong to a Mail/party desk before a CRM
window. Effort if rebuilt: about 2 to 3 weeks on top of party objects.

---

## 2. hiring (Applicant tracking)

**1. What and stack.** An ATS: jobs, a public job page, a candidate kanban, interviews with
time-zone-correct `.ics` and a subscribable feed, scorecards with rubric snapshots, an offer status
machine, onboarding checklists, retention purges, and snapshots. 17.4k src lines and 6.1k test lines
(519 cases). Runtime dependency is `lucide` only. The domain layer is `hiring/src/core/*.js`.
**It has no AI at all.**

**2. Quality.** Clean domain modules:
- An explicit transition table for offers (`core/offers.js:14-21`).
- Scorecards snapshot the rubric they were rated against (`core/scorecards.js:35-40`).
- Screening answers are stored with their question label (`core/screening.js:1-9`).

Applications do not flow in: the public job page is a static HTML file published to Puter hosting
(`core/jobpage.js`), and applying is a `mailto:` (`applySubject`). `docs/shared-hiring-design.md`
honestly says multi-user hiring is unsolved (KV is single-user). Interview conflict detection works
across time zones.

**3. Worth taking.** **Schema and ideas only; this is generic CRUD.** Worth keeping:
- snapshot-on-write for rubrics and questions (a good general rule, and Dolt plus content-hash
  references give it to you);
- the offer state machine;
- the interview slot-proposal flow.

**4. App-agent design.**
- *Objects:*
  - `choir.party` holds the candidate.
  - `choir.hiring_job` (versioned) is the posting.
  - `choir.hiring_application` links candidate to job; its stage lives in metadata and its moves
    are `choir.record_event` objects.
  - An interview is a Calendar app event that `references` the application.
  - A **scorecard is a `choir.review_record`**, using the existing `authored_review` and `reviews`
    edges. The rubric is a content-hashed object the review `references`, so rubric snapshots come
    for free.
  - An offer is a `choir.texture_document` (the letter) plus an offer object with status. Signing
    goes through the signatures desk (the envelope `references` the offer).
  - Onboarding tasks are `choir.work_item` objects.
- *Versioning:* candidate history, "who moved this candidate", restore points (replaces
  `core/snapshots.js`). Retention needs the crypto-shredding caveat above.
- *Verbs:* `post_job`, `ingest_application(source)`, `advance(application, stage)`,
  `propose_slots`, `book_interview`, `request_scorecard(interviewer)`, `debrief(application)`,
  `draft_offer`, `send_offer_for_signature`.
- *Calls:* Mail (applications by email become applications), Calendar, research (verify claims on
  a CV or public work, with each check as a cited capture), Texture (job descriptions, offer
  letters), signatures.
- *Why an agent:*
  - parsing inbound applications from mail;
  - chasing interviewers for scorecards;
  - drafting a debrief that cites every scorecard.

  Screening and ranking candidates automatically is a fairness and legal hazard. Keep the agent to
  logistics and summarizing, and record every model-assisted judgement as a `choir.review_record`
  marked as agent-authored.
- *Publish vs private:* job postings are publishable (a `choir.publication` with a
  `choir.public_route` for the apply form). Interview guides and rubrics can be published as
  templates. Everything about candidates is private.

**5. Priority: low.** Generic CRUD; Choir has no hiring need now. Effort: about 3 weeks once
party, review_record and the signatures and calendar desks exist.

---

## 3. help-desk (Zendesk-style ticketing)

**1. What and stack.** A shared ticket inbox with:
- statuses, priorities, groups, assignees, followers, CCs, tags and custom fields;
- macros, triggers, views, SLA targets and auto-close;
- a public help center with knowledge-base articles, "My requests" and CSAT;
- an email channel, reports, presence, and an AI writing assistant.

18.9k src lines and 5.3k test lines (323 cases, including `security.test.js` and
`concurrency.test.js`). Runtime dependency is `lucide` only; tests use jsdom, axe-core and
fake-indexeddb. It also has a license check script (`tools/check-licenses.mjs`).
- One backend runs in three hosts: a Puter worker (`worker/index.js`), the owner's browser, or
  tests. Routes are in `src/server/backend.js` (1,152 lines) and `src/server/tickets.js`.
- KV key layout with a change feed: `src/shared/keys.js`, README "Key-value layout".
- IndexedDB replica fed by `/sync?since=<rev>`: `src/client/replica.js`.

**2. Quality.** **The best-engineered app in the group, with a real engine:**
- `src/shared/rules.js` (348 lines): a condition language (`all`/`any` over 17 fields with
  operators such as `changed_to`), action application, `{{placeholder}}` rendering, and an SLA
  calculator that pauses while pending or on hold and handles first-reply semantics.
- `src/server/tickets.js` `updateTicket` (lines 389-500):
  - triggers run in order, each seeing the previous result (`runTriggers`, line 196);
  - field-level path updates so concurrent agents don't clobber each other;
  - a `ver` counter for ordering;
  - per-ticket event records;
  - daily caps on outbound mail.
- `src/shared/html.js`: an allowlist HTML sanitizer that rebuilds markup from tokens and needs no
  DOM.
- Uploads a browser could run (such as SVG) are served as `octet-stream` with a sandboxed CSP
  (tested).

It is labelled "early alpha". The AI is thin (`src/views/ai.js`): summary, draft and rewrite
prompts, inserted into the editor for human review. The email channel is limited by the polling
problem above.

**3. Worth taking.**
- **The rules engine (`shared/rules.js`)** is the most reusable piece in this group. It is a
  small, tested, declarative condition and action language that could become the generic "views
  and triggers" language for any record-type app desk, not just tickets.
- The SLA function.
- The sanitizer: compare it with whatever Choir uses before adopting.
- The event-trail shape (`kind: 'change'`, `changes[]`, `triggers[]`, `macro`).
- The ticket model (`shared/model.js`).

**4. App-agent design.**
- *Objects:*
  - `choir.helpdesk_ticket` (versioned, external key). Requester and org are `choir.party`;
    assignee is a Choir user or agent.
  - Each public reply or internal note is a `choir.channel_message`. Inbound mail is the existing
    `choir.inbox_delivery`, linked with `derived_from` (this replaces the `e:<id>:<time>` events).
  - Field changes are `choir.record_event` objects.
  - Macros, triggers, views and SLA policies are config objects (versioned, so every rule edit has
    history and can be reverted).
  - KB articles are `choir.texture_document` objects (Texture is the sole writer, so articles get
    revisions and transclusion).
  - The help center is a `choir.publication` with `choir.public_route` for the portal and request
    form.
  - Merged and problem/incident tickets link with `supersedes` and `references`.
- *Versioning:* the hand-built change feed, `ver` counter, replica sync and undo
  (`client/replica.js`, `client/cache.js`) collapse into Dolt history plus the graph's watermark
  reads. **Branches** are useful for testing a new trigger set against last month's tickets before
  turning it on: replay the events on a branch and diff.
- *Direct manipulation:* reply, note, set fields, apply a macro, bulk edit, merge.
- *Verbs:* `open_ticket(from message)`, `triage(ticket)` (sets type, priority, group, tags with
  reasons), `draft_reply(ticket, cite_kb=true)`, `apply_macro`, `escalate_to(desk)`, `solve`,
  `link_problem`, `kb_gap_report()`, `run_view(conditions)`, `sla_risk()`.
- *Calls:*
  - research, for answers grounded in the KB and docs, with each reply citing article revisions
    (`cites`);
  - engineering, where a bug report becomes a `choir.work_item` cast via management and the ticket
    `references` it;
  - Texture, to draft or patch KB articles from solved tickets;
  - Mail.
- *Why an agent:* triage, drafting grounded replies, and detecting SLA risk are what this app most
  needs, and the always-on desk fixes the browser-open mail polling. For Choir specifically, **a
  help desk whose bug tickets become engineering work items is the self-development loop with a
  customer face.**
- *Publish vs private:* tickets and parties are private. KB articles can be published (a help
  center is publication by design), and so can macro and trigger templates. CSAT aggregates are
  optional.

**5. Priority: medium.** It is the strongest app-agent candidate in the group, and its rules engine
is reusable. A Choir-native version (tickets as messages, KB in Texture, triage desk) is about 3 to
4 weeks. The rules engine port alone is about 3 days.

---

## 4. legal (Practice management, a Clio alternative)

**1. What and stack.** Matters, contacts, conflict checks, time and expenses with rounding and a
rate cascade, bills (draft, pending, issued) with discounts and tax, payments, **trust
accounting**, a calendar, tasks, documents with merge-field templates, reports, and team sign-in
through a firm worker. The largest app: 21.0k src lines and 7.6k test lines (453 cases). Runtime
dependency is `lucide` only.
- Domain modules are in `legal/src/core/`: `finance.js` 339 lines, `billing-ops.js`,
  `matters.js`, `merge.js`, `duration.js`, `money.js`.
- The team worker is `public/worker/legal-team-worker.js`.

**2. Quality.** The domain math is careful:
- Bills snapshot activity lines (`core/finance.js:56-71`).
- Discounts apply to fees only and tax may exclude disbursements (`billTotals`, line 84).
- `checkTrustWithdrawal` (line 274) refuses a backdated withdrawal that would make the matter's
  running ledger negative on any later day.
- Three-way reconciliation is in `billing-ops.js:211`.
- `conflictSearch` (`core/matters.js:35`) normalizes accents and punctuation and matches phone
  digits.

**Alarming for real use:**
1. Trust and ledger rules are enforced **only in the browser**. The firm worker is a generic
   record proxy that checks membership and nothing else (`legal-team-worker.js:362-389`), so any
   member can rewrite any record, including trust transactions.
2. Transactions are mutable records, not an append-only ledger (`ignoreTxIds` exists because
   transactions can be edited).
3. The README says "do not use for real client matters, trust funds".

For a regulated domain this is a demonstrator.

**3. Worth taking.** Algorithms and a schema:
- the bill math and aging buckets;
- the backdated-withdrawal check (a good invariant to state);
- conflict search;
- the billing increment rounding (`duration.js`: "1:30", "1h 30m", "90m");
- merge fields (`merge.js`) as a field list only, because Texture should own the documents.

**4. App-agent design.**
- *Objects:*
  - `choir.legal_matter` (versioned) uses `references` edges to parties carrying a
    `relationship` (client, opposing, counsel, judge, witness). This maps cleanly; their
    `matterParties` is already an edge list.
  - Time, expense and flat-fee entries are `choir.time_entry` objects.
  - Bills are `choir.invoice` objects.
  - **Every money movement is an append-only `choir.ledger_txn` (content hash, never edited; a
    correction is a reversing transaction that `supersedes`).** Trust balances are computed views.
    The withdrawal invariant is enforced by the matter desk in the same conditional batch
    (`ConditionalBatchStore.PutBatchConditional` already exists), not in the UI.
  - Statute-of-limitations dates and court dates are `choir.deadline` objects.
  - Documents are `choir.texture_document` (generated) or `choir.artifact_blob` (uploaded).
  - A conflict-check result is a `choir.review_record` that `reviews` the new matter, so the check
    becomes evidence on file.
- *What maps poorly:* their per-collection version counters and debounced batch sync have no
  counterpart and are not needed; Dolt replaces them. Multi-user firm membership needs Choir's
  sharing model, which this report does not design.
- *Verbs:* `open_matter` (runs `conflict_check` first), `log_time(nl)`, `draft_bills(period)`,
  `issue_bill`, `record_payment`, `trust_deposit` / `trust_withdraw` (invariant-checked),
  `reconcile(statement)`, `generate_document(template, matter)`, `deadline_watch()`.
- *Calls:* research (court rules, deadline computation, case law, each cited), Texture (pleadings
  and letters), Calendar, signatures (engagement letters), time-tracking (shares `time_entry`).
- *Why an agent:* drafting time entries from the day's Mail, Calendar and Texture activity (the
  number one pain in practice management), deadline watch, conflict checks on intake, and a
  pre-bill review. Deadline computation must cite the rules and stay suggest-only.
- *Publish vs private:* everything is private and privileged. Only document templates and task
  checklists could be published.

**5. Priority: low.** It is regulated and liability-heavy, and it has no Choir use case short term.
Take the `ledger_txn` design lesson. Effort: 6 weeks or more if ever.

---

## 5. contracts (Contract manager)

**1. What and stack.** A contract book with renewal tracking:
- auto-renew roll-forward, notice windows, and renewal *decisions* recorded with term history;
- recurring obligations, as one record with a live occurrence;
- AI obligation extraction from pasted text or PDF text, with OCR via `puter.ai.img2txt` behind a
  consent gate;
- documents read in the app via pdf.js;
- an `.ics` export and feed, CSV import with column mapping, and per-currency totals.

The smallest app: 7.8k src lines and 2.6k test lines (249 cases). Runtime dependencies are
`pdfjs-dist` (Apache-2.0) and `lucide`. Storage is one `contracts.json` plus `contacts.json` in
Puter FS (README "Storage design").

**2. Quality.**
- `lib/dates.js` renewal logic is the real substance: `nextTermEnd`, `agreementEnd`, `isTermEnd`
  and `completedTerms`, with month-end anchoring that does not drift (lines 120-230).
- `lib/ical.js` is a correct RFC 5545 writer: 75-octet folding, escaping, exclusive all-day
  DTEND, stable UIDs, alarms.

The AI extraction (`lib/extract.js`) is **thin**: one prompt, a 60k-character cap, defensive JSON
parsing, and output of `{text, owner, due, repeat}` **with no clause citation or source span**, so
a reviewer can't see where an obligation came from. "us" and "them" are guessed by the model.

**3. Worth taking.**
- The **renewal and term date math** (`lib/dates.js`): small and pure; port it.
- The **ICS writer** (`lib/ical.js`): the best of the four in the group.
- The contract schema (`lib/store.js:329-363`: term, autoRenew, termMonths, noticeDays, decision,
  termHistory, currency).
- The extraction prompt only as a starting point.

**4. App-agent design.**
- *Objects:*
  - `choir.contract` (versioned): counterparty is a `choir.party`; its terms live in metadata.
  - The signed PDF is a `choir.artifact_blob`, which `choir.contract` reaches via `contains_blob`.
    Amendments link to the contract with `supersedes` or `references`.
  - Extracted text is a `choir.retrieval_source` with `choir.retrieval_span` objects (both
    already exist).
  - **Each obligation is an object that `cites` the exact `retrieval_span` of the clause** it came
    from, with an owner party and a `choir.deadline`.
  - A renewal decision is a `choir.texture_decision`-style record or a `review_record` with a note.
  - Term history is Dolt history plus completed-term objects.
- *Maps cleanly:* nearly everything. The app already treats obligations as children with
  occurrence history.
- *Versioning:* redlines between versions of a contract body; "what were the terms when we
  renewed in 2025". **Branches** let you try a negotiated variant: mark up a branch and diff
  obligations.
- *Verbs:* `ingest_contract(file)` (OCR, extract terms and obligations with spans, mark as
  suggestions), `review_contract(playbook)` (flag clauses against a clause library),
  `renewal_watch()`, `record_decision`, `compare(contract_a, contract_b)`,
  `obligations_due(range, owner)`.
- *Calls:* research (counterparty facts, governing-law questions; cited), Texture (memos and
  redlines; Texture writes them), signatures (send for signature, and the resulting envelope
  `references` the contract), Calendar (deadlines), Mail (notice letters).
- *Why an agent:* **contract review is the clearest "better as an agent" case in the group.**
  Extraction with clause-level citations, review against the owner's own standards, and an
  always-on renewal watch that drafts the notice letter 60 days out instead of relying on a badge.
- *Publish vs private:* contracts are private. A **clause library and review playbooks** are
  natural Autopaper publications that other computers adopt (and cite back).

**5. Priority: medium.** Small, document-centric, a clean fit with Texture, research and
retrieval spans, and high agent value. Effort: about 2 weeks for ingest, cited obligations and
renewal watch, reusing the PDF app and Texture.

---

## 6. signatures (DocHub-style e-signature)

**1. What and stack.**
- *Build and prepare:* merge PDFs and images into one document; edit pages (reorder, rotate,
  delete) with fields remapped; place fields by role, with snapping, align and copy-to-pages.
- *Send:* capability links per signer, optional access codes, signing order.
- *Sign:* a consent disclosure step, then draw, type or upload a signature.
- *After sending:* expiry, reminders, correction or reissue of signers, void.
- *Complete:* a flattened PDF plus a certificate page; templates by role.

12.2k src lines and 5.6k test lines (343 cases). Runtime dependencies: `pdf-lib` (MIT),
`pdfjs-dist` (Apache-2.0), `lucide`.
- Sign worker: `src/workers/sign-worker.js`, 1,114 lines, one per user.
- Email worker: `src/workers/email-worker.js`, one for the app.
- Signing page: `src/sign-main.js`.

**2. Quality.** Strong:
- The worker header (`sign-worker.js:1-70`) is a good security design:
  - 160-bit tokens stored only as SHA-256 hashes;
  - the token travels in the URL fragment, so it never reaches server logs;
  - sequential order is enforced server-side;
  - access codes are salted hashes with lockout (5 failures, then 15 minutes);
  - each signer's response has its own key, so a signer can't race an admin sync;
  - evidence (IP, user agent, time-zone offset) is recorded at view, consent and sign.
- `lib/flatten.js` handles WinAnsi fallback, bisection text fitting and wrapping, with tests that
  measure the drawn strings.
- `lib/pdfedit.js` page plans remap fields under rotation and deletion.

**Caveats:**
1. **The final signed PDF is built in the owner's browser** (`buildSignedPdf`, `flatten.js:319`),
   and there is **no cryptographic seal** (no PAdES or PDF signature dictionary, no hash of the
   final document recorded server-side). The certificate page shows only the *original's*
   fingerprint (lines 265-270), so the owner could alter values before flattening without
   detection.
2. The admin secret lives in the owner's KV.
3. Reminders only go out while the app is open (`ui/actions.js:351`).

This is acceptable for informal use, but weak as evidence.

**3. Worth taking.**
- **`lib/flatten.js` and `lib/pdfedit.js` with pdf-lib.** This is a real, tested PDF stamping,
  merging and page-plan engine, and Choir's PDF app could use it directly.
- The field model (`lib/model.js`: normalized rects, snapping, role templates).
- The capability-link worker protocol, as the spec for a Choir public route.
- The consent disclosure and evidence record shape (`lib/consent.js`).

**4. App-agent design.**
- *Objects:*
  - The original PDF is a `choir.artifact_blob`.
  - `choir.signature_envelope` (versioned) holds signers (parties with role and order), field
    layout, expiry and status.
  - **Each signer's consent is the existing `choir.consent_record`** (`granted_consent` /
    `consent_for` edges).
  - Each signature act is a `choir.record_event` carrying the evidence.
  - The completed PDF is a new `artifact_blob` that is `derived_from` the original. Its hash is
    sealed by a **`choir.verifier_attestation`** (`attests_to` the completed blob and the event
    set) **written by the desk on the computer, not by the owner's browser.** That fixes caveat 1
    and gives an audit trail that other computers can verify.
  - The signing link is a `choir.public_route` that `routes_to` the envelope, and stores only the
    token hash.
  - Templates are envelopes without signers.
- *Links:* the envelope `references` the contract (contracts app), the offer (hiring), the matter
  or engagement (legal), or the deal (crm).
- *Versioning:* history of who changed which field when. "Correct signer" becomes a `supersedes`
  edge between signer records rather than a mutation.
- *Verbs:* `prepare(doc, signers)` (with auto field placement from detected signature lines),
  `send`, `remind`, `correct_signer`, `void`, `complete` (flatten and attest), `verify(pdf)` (hash
  lookup against attestations).
- *Calls:* Mail (send links, notifications), contracts and hiring (the originating object), PDF
  app (rendering).
- *Why an agent:* modest gains: auto-placing fields, chasing signers on a real schedule (always
  on), and producing an attested completion. The bigger value is that the substrate (consent
  record plus verifier attestation) makes the result trustworthy.
- *Publish vs private:* envelopes are private. Templates (field layouts by role) can be published.
  Attestation hashes could be published as a public verification record, without content.

**5. Priority: medium-low.** The PDF engine is worth lifting into the PDF app soon (about 3 days).
A full signing desk is about 2 to 3 weeks and needs public routes for outsiders, plus legal
(ESIGN/eIDAS) review before anyone relies on it.

---

## 7. survey (SurveyMonkey/Typeform-style surveys)

**1. What and stack.**
- *Build:* 14 question types; skip, page and display logic; themes with WCAG contrast checks;
  undo/redo; 27 templates; an AI survey draft; paste-a-list import.
- *Collect:* multiple collectors per survey (password, close date, limits, one per browser), QR
  codes and embeds.
- *Analyze:* summaries, NPS, grids, ranking, frequent words, filters, export.

15.2k src lines and 4.3k test lines (317 cases). Runtime dependencies: `qrcode-generator` (MIT)
and `lucide`.
- Collector worker: `src/worker/collector.js`.
- Survey model: `src/model/survey.js` (immutable values with structural sharing).

**2. Quality.** A real engine:
- `src/shared/runtime.js` (521 lines) is a dependency-free validator and logic walker
  (`sanitizeAnswer`, `validateAnswer`, `conditionHolds`, `nextStep`, `walk`, `validateResponse`,
  `seededShuffle`). The *same source* is prepended into the worker at deploy
  (`src/data/worker-source.js`), so server validation is exactly client validation.
- Analysis is pure (`src/analysis/aggregate.js`) and runs in a Web Worker.

The AI draft (`src/model/ai.js`) maps model output onto its own factories, with caps and fresh ids.
Alpha-labelled, but complete for its scope.

**3. Worth taking.**
- **`shared/runtime.js` plus the survey document schema (`model/survey.js`) plus
  `analysis/aggregate.js`.** A portable form, logic and validation engine with a defined answer
  shape per type (documented at `runtime.js:15-25`).
- The "one validator, two hosts" pattern.
- Its answer-scale presets are useful data.

**4. App-agent design.**
- *Objects:*
  - `choir.survey` (versioned): the document, with its body as JSON. Drafts versus published
    match **Dolt branches** exactly: edit on a draft branch and publish by merge; respondents only
    ever see the published commit. This replaces the app's own "edits stay private until you
    publish".
  - A collector is a `choir.public_route` that `routes_to` the survey, with the collector settings.
  - `choir.survey_response` (content hash, one per submission) is `derived_from` the survey
    version it answered, so analysis always knows which wording was answered.
  - An analysis report is a `choir.texture_document` that `cites` response aggregates.
  - Respondents are anonymous by default; they are linked to a `choir.party` only when a contact
    question is answered and the owner opts in.
- *Cross-app links:* a CSAT survey `references` a help-desk ticket; candidate surveys
  `references` a hiring application; customer research `references` CRM parties. Free-text answers
  can be handed to research as evidence objects.
- *Verbs:* `draft_survey(goal, audience)` (design review for leading or double-barreled
  questions), `publish`, `open_collector`, `close`, `analyze(filters)`,
  `code_open_text(themes)`, `compare_waves(a, b)`, `report` (via Texture).
- *Calls:* Texture (report), research (benchmark against public data, cited), Mail (distribution),
  CRM and help-desk (audiences).
- *Why an agent:* analyzing open text (thematic coding with quotes as evidence), questionnaire
  design critique, and a report that cites responses. The deterministic runtime stays
  deterministic; the agent works above it.
- *Publish vs private:* survey designs and templates are publishable to Autopaper. Aggregate
  results are publishable when the owner chooses (a natural Autopaper story source: "poll
  results"). Raw responses are private.

**5. Priority: medium.** It is a portable engine, fits branches well, and is useful for Choir
itself (owner and user feedback, Autopaper reader polls). Effort: about 2 weeks; lift
`runtime.js` and `aggregate.js` nearly as-is, and rebuild the builder UI in Svelte.

---

## 8. time-tracking ("Hourly")

**1. What and stack.**
- A timer and manual entries with duration parsing, timesheets and a week grid.
- Invoicing: per-client unbilled totals, rate locking, invoice branding, payments and aging.
- Effective-dated rate cascade, rounding and minimums.
- Budgets and retainers, Toggl/Clockify/Harvest CSV import, an offline PWA, multi-device
  reconciliation, and URL actions.

10.8k src lines and 3.4k test lines (201 cases). Runtime dependency is `lucide` only. Storage is KV
for settings, clients and projects, plus one FS JSON file per month for entries
(`src/lib/store.js:1-45`, 2,347 lines).

**2. Quality.** Good small-business accounting details:
- effective-dated rates (`rateAt`, line 64);
- rate, rounding, minimum and currency frozen at invoicing;
- an id-based three-way merge (`mergeById`, line 197).

The importer (`lib/importer.js`) knows real Toggl, Clockify and Harvest headers. It is still
generic CRUD; `store.js` is a monolith.

**3. Worth taking.** **Schema and ideas only.** Worth keeping:
- effective-dated rate periods;
- "lock values at invoicing" (this is what snapshots and content-hash references give for free);
- the importer header map.

`mergeById` and the offline queue are not needed on Dolt.

**4. App-agent design.**
- *Objects:* entries are **`choir.time_entry`**, shared with legal and shifts. Clients are
  `choir.party`. Projects are `choir.project` objects, or reuse a Choir project or work-area
  object if one exists. Invoices are `choir.invoice`, lines `references` entries, and payments are
  `choir.ledger_txn`.
- *Distinctive Choir angle:* **time entries can be derived from the computer's own trace.** Runs,
  work items and Texture revisions already carry timestamps (`choir.run`, `choir.work_item`). An
  entry can be `derived_from` the runs and documents it covers. That makes timesheets auditable,
  and it gives a way to account for agent compute time per project or client next to human time.
- *Verbs:* `start` / `stop`, `log(nl)` ("1.5h on Acme review yesterday"),
  `suggest_entries(day)` (from Calendar, Mail, Texture and run history), `unbilled(client)`,
  `invoice(client, period)`, `record_payment`, `import(csv)`.
- *Calls:* Calendar, Mail, Texture and run history (the evidence for suggestions); legal (shares
  entries); Mail (sending invoices).
- *Why an agent:* "forgot to track" is the core failure, and a desk that reconstructs the day from
  evidence fixes it. Forgot-to-stop detection works without an open tab.
- *Publish vs private:* private. Nothing to publish except perhaps invoice templates.

**5. Priority: low**, but adopt `choir.time_entry` early if Choir wants per-project cost
accounting for agent work. Effort: about 1.5 weeks.

---

## 9. shifts (Shift scheduling)

**1. What and stack.**
- A week grid of employees by days, with drag-to-move, conflict checks across overnight shifts,
  and copy-last-week.
- Availability and time off; coverage requirements; hours guardrails; working-time limits.
- A labor budget with an overtime multiplier.
- Swaps (request, review, approve); attendance with clock-in and clock-out; payroll CSV; reports.
- A public read-only snapshot, CSV import, restore points and offline support.

13.9k src lines and 5.0k test lines (369 cases). Runtime dependency is `lucide` only. Domain code
is in `shifts/src/core/`.

**2. Quality.** Clean and well tested.
- The "engine" is `suggestAssignees` (`core/scheduling.js:282-345`): hard filters (time off,
  unavailable, overlap, max hours), then ranking by rule warnings, availability, hours deficit and
  role match.
- `planFill` (`core/fill.js`) is a **greedy** pass that writes each pick into a working copy, so
  later picks see earlier ones. It is not an optimizer (no ILP or constraint solver) and makes no
  fairness guarantee across weeks.

Generic CRUD plus a reasonable heuristic.

**3. Worth taking.** **Schema and ideas only.** Worth keeping:
- the shift, availability, coverage and rules model;
- overnight conflict math (`core/shift-time.js`);
- the "proposals with reasons, reviewed before one undoable write" UX for any agent-proposed bulk
  change. That last idea is the best thing here, and it applies to every app desk.

**4. App-agent design.**
- *Objects:*
  - Employees are `choir.party` with the employee role in metadata.
  - `choir.shift` (versioned), with an `assigned_to` relationship as a `references` edge.
  - Availability and time off are objects per party.
  - Attendance is a `choir.time_entry` `derived_from` the shift.
  - A swap is a `choir.work_item`-like request with a decision.
- *Branches are the killer feature:* the agent builds next week's schedule on a **branch**, the
  manager reviews the diff (who moved, cost delta, rule warnings), and merges. That replaces their
  in-memory proposal plan.
- *Verbs:* `propose_schedule(week, constraints)` (on a branch), `fill_open`, `evaluate_move`,
  `request_swap` / `approve_swap`, `coverage_gaps`, `labor_cost(scenario)`, `publish_snapshot`.
- *Calls:* Calendar (per-employee feeds), Mail and SMS (notify changes), time-tracking (shared
  attendance entries).
- *Why an agent:* "make next week work given these call-outs" is a natural-language, multi-
  constraint task that suits an agent driving a deterministic checker (`ruleViolations`,
  `findConflicts`).
- *Publish vs private:* private. The public weekly snapshot is a `choir.public_route` to a
  rendered view, not an Autopaper publication.

**5. Priority: low.** Generic CRUD with no Choir need. The branch-review pattern is the
transferable idea. Effort: about 2 weeks.

---

## Group summary

**Top picks (in order):**
1. **help-desk:** the `shared/rules.js` condition, action and SLA engine, plus the
   ticket-as-messages model. It is the best "app as agent" fit (triage, grounded replies, bug to
   engineering work item).
2. **contracts:** `lib/dates.js` renewal math and `lib/ical.js`. Clause-cited obligation
   extraction plus an always-on renewal watch is the clearest case where an agent beats the app.
3. **survey:** `shared/runtime.js`, a validation and logic engine shared by client and server,
   plus `analysis/aggregate.js`. Draft and published map onto Dolt branches.
4. **signatures:** the `lib/flatten.js` + `lib/pdfedit.js` pdf-lib engine for Choir's PDF app.
   The completion should be attested by the desk (`choir.verifier_attestation`), not built in the
   owner's browser.

**Generic CRUD whose value is only a schema or ideas:** crm, hiring, time-tracking, shifts, and
mostly legal (except its ledger lessons).

**Cross-app reusable pieces:**
- *Code:* the help-desk rules DSL and HTML sanitizer, the survey runtime, the contracts ICS writer
  and renewal math, the signatures PDF flatten and page-plan code, the legal bill math and
  conflict search, and the shifts suggest and fill heuristic.
- *Patterns:*
  - snapshot-on-write for anything a judgement was made against (rubrics, rates, questions);
    content-hash references give this for free;
  - "proposals with reasons, reviewed, then one undoable write" for agent bulk actions; Dolt
    branches give this for free;
  - the owner-hosted capability-link edge (hashed tokens, fragment-borne tokens, server-side
    order and expiry) as the spec for `choir.public_route` handlers.
- *Shared object kinds that make these apps one system instead of nine:* `choir.party`,
  `choir.time_entry`, `choir.invoice`, `choir.ledger_txn` (append-only), `choir.deadline` and
  `choir.record_event`, alongside the existing `consent_record`, `review_record`,
  `verifier_attestation`, `retrieval_span`, `channel_message` and `public_route`.
  **`choir.party` is the single highest-value addition.**

**Alarms:**
- *No license issues.* Runtime licenses are MIT, ISC, Apache-2.0 and 0BSD. MPL-2.0 appears only
  in build and test tooling.
- *Legal:* trust-accounting invariants are client-side only, and transactions are mutable.
- *Signatures:* the final PDF is flattened in the owner's browser with no cryptographic seal.
- *Scheduled work:* everything scheduled across the group runs only while a tab is open.
- *Dolt history versus erasure:* Dolt history conflicts with GDPR-style erasure, which these apps
  support (hiring retention, help-desk hard delete). It needs crypto-shredding or a
  history-rewrite path before any of this personal data goes into the versioned graph.
