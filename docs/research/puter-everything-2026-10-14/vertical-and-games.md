# Group report: vertical-and-games

Reviewer scope: `src/field-service`, `src/fleet`, `src/property-management`,
`src/childcare`, `src/chess`, `src/checkers`, `src/backgammon`,
`src/solitaire`, `src/sudoku`, plus the repo root (build, deploy, shared
infrastructure). All paths below are relative to the clone root
`.../puter-everything/repo/`. I read the code and did not run any of it.

---

## 0. Repo-level: build, deploy, shared infrastructure

**There is no monorepo tooling and no shared code library.** Each `src/<app>/`
is its own npm project with its own `package.json`, `package-lock.json`,
`vite.config.js` and `index.html`. There are no workspaces, no Turborepo/Nx,
no shared `packages/` folder, and no common UI kit. The only shared file is
`dev/puter-localhost.mjs` (21 lines): a Vite dev-server plugin that rewrites
the printed URL to `http://<name>.puter.localhost:<port>/`.

- **Build:** Vite (v7 in the verticals, v8 in the games), vanilla ES modules,
  no framework (no React/Svelte/Vue) in any app in my group. Node 22.12 or
  newer is required.
- **Deploy:** `amplify.yml` registers about 50 AWS Amplify Hosting
  "monorepo apps", one per `src/<app>` (`appRoot`). Each runs
  `nvm install 22 && npm ci && npm run build` and publishes `dist/`.
  `customHttp.yml` repeats the same cache headers per app: no-cache for HTML,
  and `immutable` for the hashed `/assets/*`. Each app is a static SPA on its
  own subdomain (e.g. `chess.puter.com`). `src/building-design` has no
  `package.json` and no Amplify entry (only icons), so it is a placeholder.
- **CI:** `.github/workflows/ci.yml` builds a matrix of the changed apps (by
  `git diff` on `src/<app>/`) and runs `vitest run --changed` plus `npm run
  build`. A nightly run tests the apps that changed in the last day.
  `e2e.yml` runs Playwright for a handful of heavy apps (documents, pdf,
  presentation, 3d, email, parametric). `deploy-meetings-worker.yml` deploys
  a Puter Worker with `HeyPuter/puter-worker-deploy-action` and a
  `PUTER_API_TOKEN` secret.
- **The real shared "SDK" is puter.js**, loaded from the CDN
  (`<script src="https://js.puter.com/v2/">`, 212 references across the
  repo). It is not vendored or version-pinned: every app runs whatever the
  CDN serves. In my group it is used for:
  - `puter.auth`: signIn, getUser, getProfilePicture;
  - `puter.kv`: per-user DynamoDB-backed key/value storage, the primary
    datastore for every app;
  - `puter.fs`: file attachments;
  - `puter.workers.create` / `exec` / `delete`: serverless workers deployed
    into the user's own account and called with the caller's session;
  - `puter.email.send`;
  - `puter.peer`: WebRTC with signaling and TURN, used for online games.

  `puter.env === 'app'` detects running as a window inside the Puter
  desktop, where the user is always signed in (`fleet/src/platform.js`).
  None of my nine apps call `puter.ai`.
- **Shared design system: none as a package. It is copy-paste templating.**
  The four verticals share an identical skeleton:
  - `src/core/{store,backend,cache,schema,ids,dates,money,csv,search}.js`;
  - `src/ui/{shell,viewhost,components,dom,icons,menu,dialog}.js`;
  - `src/team.js`, `public/worker/<app>-team-worker.js`.

  `ui/components.js` differs by only about 40–50 lines out of about 510
  between fleet, field-service and childcare. `store.js`, `shell.js` and
  `team.js` diverge heavily, so each app forked the template and evolved it
  separately. The games follow a second template: `engine/`, `game/`
  (clock, persistence, notation), `net/online.js`, and a 2.3–2.9k-line
  `main.js`. Checkers' spec opens with "Rebuild the single-file checkers game
  as a structural mirror of chess.puter.com"
  (`checkers/docs/superpowers/specs/...`).
- **Provenance signals:** `docs/superpowers/{plans,specs}` (the Claude Code
  "superpowers" plugin layout) in checkers and solitaire, and test files named
  by review pass (`fleet/tests/review-*.test.js`, `review2-*`,
  `review3-final.test.js`). This code is clearly agent-generated through
  repeated agentic review passes. It is not slop, though. The comments are
  dense and explain *why*. Edge cases are handled: races, offline sync,
  minified-name lookups, and timezone tests. The domain modules are pure
  functions with injected `store`/`today`/`fetch`, and the tests are real.
- **Analytics:** Plausible script in `chess/`, `checkers/`, `backgammon/`
  (and some apps outside my group).

**Implications for Choir hosting many windows:**

1. The repo is a precedent for "many small vanilla-JS apps, one auth/storage
   SDK". Choir's analogue of puter.js would be a desk/record SDK injected
   into each window.
2. The verticals' storage contract (`core/backend.js`) is a clean 8-method
   interface: `loadMeta`, `loadBucket`, `loadSettings`, `putRecords`,
   `deleteRecords`, `saveSettings`, `bumpVersions`, `nextNumber`. It already
   has a memory backend for tests. Re-pointing it at a Choir computer
   (records on the ledger or computer store) is a small adapter, which makes
   porting a vertical's domain logic cheap.
3. Puter's "team worker deployed into the owner's account, called with the
   caller's session, server-side role checks" is a pattern Choir would
   express as an app desk that owns the records and admits other principals.
4. Do not copy the copy-paste templating. Choir's `frontend/src/lib/apps`
   registry plus the `--choir-*` tokens is already the better shape.

---

## 0b. Object-graph conventions used in point 4 below

These sections follow the owner addition: shared state is the point. Every
app's state is typed objects in the computer's object graph
(`internal/objectgraph/object.go`), with these properties:
- **Identity:** `obj:<kind>:<owner>:<suffix>`.
- **Contents:** `ContentHash`, `Body`, JSON `Metadata`, `VersionID`,
  `SupersededBy` and `Tombstone`.
- **Links:** typed `Edge{FromID, ToID, Kind, Metadata}`.
- **Storage:** the embedded per-computer Dolt (`dolt_store.go`), so every
  write is a Dolt commit.

Published objects go to the **Autopaper object graph**, the global store
(Store B / corpusd), which other computers adopt by id and version
(`docs/autopaper-sw-design-2026-10-09.md` §2).

I follow the owner's rule "existing types only unless needed":
- reuse `choir.web_capture` for anything fetched from the web;
- reuse `choir.source_entity` / `choir.source_ref` for citations;
- reuse Texture documents for rendered prose (quotes, statements, reports);
- reuse `choir.commitment_record` for inter-desk asks and approvals.

New kinds are proposed only for domain records that no existing kind fits.
Kind names below (`choir.party`, `choir.ledger_txn`, …) are proposals.

**What versioning gives every app for free:**
- **History and audit:** every record change is a Dolt commit, which replaces
  `createdAt`/`updatedAt` bookkeeping and "who changed this".
- **Undo:** revert to a prior version. This replaces fleet's
  `restore()` and solitaire's JSON-snapshot stack.
- **Variants:** a Dolt branch for "try a variant", e.g. a what-if schedule, a
  draft rent increase across a portfolio, or a chess analysis line.
- **Merge:** concurrent devices and agents merge instead of using the
  verticals' hand-rolled version-counter sync.

**A cross-app finding.** The verticals' `core/store.js` + `core/schema.js`
machinery exists *only* because Puter KV has no history, transactions or
change feed. Its parts are:
- per-bucket version counters (`fl1:meta`);
- month-partitioned buckets;
- pending-write queues;
- the "applyEdits" 3-way merge.

On Dolt-backed objects most of it disappears. The parts that survive are:
- the optimistic local apply, in the window;
- the offline cache;
- atomic document numbering (a computer-local counter object).

So the store is a design reference for the *window* side, and the server
side should not be ported.

**Shared kinds across this group** (and across the Office and business
groups). These are what make the graph "shared":

| Proposed kind | Replaces (per app) | Linked from |
|---|---|---|
| `choir.party` (person or organization, with role facets: client, tenant, guardian, driver, vendor, owner, staff) | field-service `clients`, PM `tenants`/`owners`/`vendors`, childcare `guardians`/`staff`, fleet `contacts`/`vendors` | everything; the contacts/CRM app is its natural owner |
| `choir.place` (address + geo) | field-service `properties`, PM `properties`/`units` (unit = place with parent edge) | jobs, leases, vehicles' home base |
| `choir.ledger_txn` (balanced lines in integer cents; immutable; corrections by reversing txn or `SupersededBy`) | PM `transactions`, field-service `payments`/`invoices` money effect, childcare `payments`, fleet `expenses`/`fuel` cost | invoices, bills, any app that moves money |
| `choir.doc_line_items` or a Texture document with structured metadata (quote, invoice, work order, PO, statement) | field-service quotes/invoices, fleet WOs/POs, childcare invoices, PM bills | ledger txns (`settles`/`bills` edges), parties |
| `choir.calendar_event` (with recurrence rule) | field-service `visits`, fleet `reminders`/`renewals` due dates, PM lease dates, childcare events | the calendar app shows all of them in one view |
| `choir.file` / content blob (photos, receipts, scans) | each app's `puter.fs` attachments (`files.js`) | any record via an `attachment` edge |

Common edge kinds: `party_of`, `located_at`, `attachment`, `settles`,
`bills`, `derived_from` (record → `choir.web_capture`), `cites` (→
`choir.source_ref`), `scheduled_as` (→ `choir.calendar_event`), and
`requested_by` (→ `choir.commitment_record`).

**Publish vs private (default rule).** Business, financial and personal
records are **private** to the owner's computer.

What may be published to the Autopaper object graph is:
- reference data with no personal content, such as a VIN-decoded vehicle
  model spec, a recall capture, or a sudoku puzzle and its grade;
- templates, such as an inspection form, a service program, a chart of
  accounts or a lease template;
- explicit owner-approved outputs, such as an anonymized benchmark or a
  public game record.

Publishing is always an owner act, recorded as a commitment.

---

## 1. field-service (Jobber / Housecall Pro clone)

**1. What / stack.** Clients → requests → quotes → jobs (with recurring
visits) → invoices → payments, plus a schedule calendar, timesheets,
expenses, reports and team access.
- Stack: vanilla JS, Vite, `lucide` (ISC), and puter.js.
- Size: about 128 files, about 20k lines of JS. Largest files are
  `src/views/schedule.js` (1100), `src/views/common.js` (697) and
  `src/workers/email-worker.js` (641).
- Licenses: clean (ISC/MIT only).

**2. Quality.** Real domain core:
- `src/core/workflow.js` has pure pipeline operations: `requestToQuote`,
  `quoteToJob`, `jobToInvoice`, `recordPayment`, `clientBalance`,
  `deleteClientCascade`, and `applyEdits(current, before, after)`, a
  3-way field merge so an edit form does not clobber concurrent changes.
- `src/core/recurrence.js` generates visit dates (daily / weekly-by-weekday /
  monthly by day or nth weekday, count or until).
- `src/core/totals.js` handles tax and line totals.
- `src/workers/email-worker.js` is a well-reasoned anti-abuse mail relay:
  - the caller's session is required and the send is billed to the caller;
  - the body is text only and is HTML-escaped server-side;
  - recipients are capped per request and per day, sends are de-duplicated,
    and recipients are SHA-256-hashed in KV.
- 34 test files, about 263 cases, including store race tests.

Missing for a real business: payment processing, online booking, routing and
maps beyond OSM links, and a customer portal.

**3. Worth taking.**
- The quote→job→invoice→payment workflow as a data model.
- `applyEdits` (a 3-way merge on save).
- The recurrence rules.
- The email-worker abuse design, as an idea for any Choir "send on behalf of
  user" capability.

UI: nothing.

**4. App-agent design.**
- **State:** records for clients, properties, requests, quotes, jobs (each
  job owns its visits), invoices, payments, products/services, team members,
  timesheets and expenses. These map to typed objects on the computer, with
  each state transition as a ledger event (e.g. `quote.approved`,
  `visit.completed`).
- **Direct manipulation:** drag a visit on the calendar, check off a visit,
  edit line items, record a payment.
- **Verbs:**
  - `create_request(client, description)`, `draft_quote(request,
    items?)`, `convert(quote→job)`, `schedule_visits(job, recurrence,
    assignee)`, `reschedule(visit, slot)`;
  - `complete_visit(visit, notes, photos)`, `invoice_job(job)`,
    `send(doc, channel)`, `record_payment`;
  - `client_balance(client)`, `ar_aging()`.
- **Calls other desks:**
  - research: price lookup for parts and materials, client address
    geocoding;
  - Texture: render quote and invoice documents and client letters;
  - management: approval before sending money-bearing documents.
- **Why an agent:** it can turn an inbound email or voicemail transcript into
  a request and a draft quote, propose a week's schedule that balances crews,
  and chase overdue invoices with drafted reminders the owner approves.

- **Object graph:**
  - **Kinds:**
    - client → `choir.party` (client facet), and the client's service
      property → `choir.place`;
    - request → a new `choir.service_request`, with body text plus a
      `derived_from` edge to the email or voicemail capture it came from;
    - quote and invoice → line-item documents (Texture-rendered, with the
      structured line items in metadata);
    - job → `choir.job`, holding the recurrence rule;
    - each visit → `choir.calendar_event`, linked `scheduled_as` from the
      job, so the Choir calendar shows crews' visits with no integration;
    - payment → `choir.ledger_txn` with a `settles` edge to the invoice;
    - products and services → `choir.catalog_item`, which can be shared
      with the invoicing and inventory apps.
  - **Links:** the pipeline edges are `quote -derived_from-> request`,
    `job -derived_from-> quote` and `invoice -bills-> job`. These replace
    the id fields `requestId`/`quoteId`/`jobId` and make the funnel a graph
    query.
  - **Maps cleanly:** almost everything, since records are flat JSON with
    ids and references by id (`src/core/workflow.js`).
  - **Does not map cleanly:**
    - `job.visits[]` is embedded today and should become separate event
      objects so other apps can see them;
    - quote/invoice numbering needs a computer-local counter object;
    - `applyEdits` becomes a Dolt merge.
  - **Versioning:**
    - quote revisions are history (the "which version did the client
      approve" question answers itself);
    - a branch holds a draft re-schedule of the week until it is committed;
    - a mis-recorded payment is undone by a revert.
  - **Publish vs private:** nothing by default. Optionally publish a service
    or price catalogue, or derived availability for a public booking page.
    Clients, addresses, visits and money stay private.

**5. Priority: low.** It is a vertical SMB app, and Choir's near-term value is
general productivity. If wanted, it is a medium effort (about 1–2 weeks) to
port the core into an app desk, because the core is already pure and
store-injected.

---

## 2. fleet (Fleetio clone)

**1. What / stack.** Covers vehicles, meters, fuel, inspections (with a form
builder), issues, service programs, reminders and renewals, work orders,
parts and purchase orders, vendors, and costs and reports.
- Stack: vanilla JS, Vite, `lucide`.
- Size: about 184 files, about 27k lines of JS.
- Licenses: clean.

**2. Quality.** The most thoroughly engineered vertical.
- `src/core/store.js` is a local-first store:
  - in-memory Maps, applied optimistically;
  - batched flush with retry and timeout;
  - per-bucket version counters, so sync re-reads only changed buckets;
  - month-partitioned buckets for growing logs (`src/core/schema.js`
    `PARTITIONED`, `bucketOf` → `fuel_202610`);
  - an IndexedDB offline cache that includes pending writes;
  - atomic human-friendly numbering via `kv.incr` with an `atLeast`
    catch-up (`nextNumber`).
- `src/core/reminders.js`: due by time *or* meter, whichever first, with
  snooze and history.
- `src/core/vin.js` and `src/core/recalls.js` call the NHTSA vPIC and recalls
  public APIs, with fetch injected.
- `public/worker/fleet-team-worker.js` checks roles server-side. A driver may
  only write fuel, inspections, issues and meters, and the vehicle meter is
  merged field-by-field so it cannot undo others' edits.
- 62 test files, about 517 cases.

**3. Worth taking.**
- **The store/sync engine pattern** (`core/store.js` + `core/schema.js` +
  `core/backend.js`): partitioned buckets, version counters, optimistic local
  apply, offline queue. It is the best reusable piece in the group, as a
  reference design for app-window state that syncs to the computer.
- The time-or-meter reminder model.
- The NHTSA VIN and recall lookups. These are a natural research-desk tool.

**4. App-agent design.**
- **State:** a vehicle registry, plus append-only logs (meters, fuel,
  inspections, service entries). These logs are a natural fit for ledger
  events, and the reminders are derived state.
- **Direct manipulation:**
  - log fuel or a meter reading from a phone;
  - run an inspection checklist;
  - drag a work order through its statuses.
- **Verbs:**
  - `add_vehicle(vin)`, which decodes the VIN through research;
  - `log_fuel` and `log_meter`;
  - `report_issue(vehicle, text, photo)`;
  - `create_work_order(issues)`;
  - `due_service(window)`;
  - `check_recalls(vehicle|all)`;
  - `cost_per_mile(vehicle, period)`.
- **Calls other desks:**
  - research: recalls, part prices, vendor search;
  - Texture: monthly fleet report;
  - management: schedule recurring recall sweeps as a standing intent.
- **Why an agent:** it can parse a receipt photo into a fuel entry, triage
  driver-reported issues into work orders, and proactively surface recalls
  and renewals. Background monitoring is what the agent adds.

- **Object graph:**
  - **Kinds:**
    - `choir.vehicle`, with a `same_model_as` edge to a published
      `choir.vehicle_spec` (the VIN decode);
    - vendors, drivers and contacts → `choir.party`;
    - parts → `choir.catalog_item` (shared with inventory);
    - work orders and POs → line-item documents;
    - fuel, expense and service cost → `choir.ledger_txn` (or a lighter
      `choir.expense` that posts one).
  - **Logs:** meter readings, fuel fill-ups, inspections and service entries
    are small immutable objects. Each has an edge to its vehicle and an
    `attachment` edge to its receipt or photo blob.
  - **Reminders and renewals:** `choir.calendar_event` with the time-or-meter
    rule in metadata. The due-state (`reminderState`) stays derived and is
    not stored.
  - **Recalls:** a `choir.web_capture` from NHTSA (research desk), linked
    `affects` to the vehicle.
  - **Maps cleanly:** the flat records.
  - **Does not map cleanly:**
    - the whole bucket/partition/version-counter layer
      (`src/core/schema.js` `PARTITIONED`, `fl1:meta`) has no counterpart.
      It was a DynamoDB cost workaround, and Dolt history replaces it;
    - inspection *forms* (templates) vs inspection *runs* (instances) become
      two kinds with a `instance_of` edge.
  - **Versioning:** the vehicle's history *is* its Dolt history. Odometer
    corrections are visible, not silent overwrites. A branch can model a
    proposed service-program change across the fleet before applying it.
  - **Publish:** vehicle model specs, recall captures, and inspection-form
    and service-program templates. These are generic and useful to every
    computer. Vehicles, VINs tied to an owner, driver records, fuel and
    costs stay private.

**5. Priority: low** as an app. **Medium** as a *reference implementation* for
Choir's local-first window store. Reading it costs about 1 day; a port would
take about 1–2 weeks.

---

## 3. property-management (Buildium / AppFolio clone)

**1. What / stack.** Covers properties, units, leases, tenants, rent posting,
a full accounting module, maintenance, communication, a calendar and reports.
- Stack: vanilla JS, Vite, `lucide`.
- Size: about 167 files, about 31k lines of JS. The largest file is
  `src/views/transactions/dialogs.js` (1160).
- Licenses: clean.

**2. Quality.** Contains a **real double-entry ledger**.
- `src/core/ledger.js`:
  - every financial event is a transaction with balanced lines in integer
    cents (`isBalanced`);
  - 14 transaction kinds (charge, payment, credit, refund, deposit
    application, bills, transfers, owner contributions and distributions);
  - receivable lines carry `forAccountId`, so payments apply to specific
    charges;
  - pure builders, plus a memoized `ledgerIndex`.
- `src/core/financials.js`:
  - income statement, balance sheet, cash flow and trial balance;
  - general ledger, owner statement, budget-vs-actual;
  - bank reconciliation (`unreconciledLines`, `clearedBalance`).
- `src/core/leases.js`: lease status, renewals, occupancy, and recurring rent
  posting (`computePostings`, `postDue`).
- 30 test files, about 345 cases.

The UI is sprawling, and there is no tenant portal or real payment rails.

**3. Worth taking.** **The double-entry ledger + financial statements
(`core/ledger.js`, `core/financials.js`, `core/accounts.js`)** is a compact,
pure, tested small-business accounting core. It is reusable for any Choir
money-bearing app (invoicing, expense, a personal finance desk). Compare it
with `src/accounting` from another group before choosing one.

**4. App-agent design.**
- **State:**
  - a chart of accounts and balanced transactions, which map almost 1:1 onto
    append-only ledger events (a natural fit for Choir's commitment ledger
    philosophy);
  - leases and units as objects;
  - derived balances.
- **Direct manipulation:**
  - record a payment;
  - post a bill;
  - reconcile against a bank statement;
  - edit lease terms.
- **Verbs:**
  - `post_rent(through_date)`;
  - `record_payment(lease, amount)`, `apply_deposit`;
  - `rent_roll()`, `delinquency()`;
  - `owner_statement(owner, period)`;
  - `reconcile(account, statement_file)`;
  - `renewal_candidates(days)`;
  - `open_maintenance(unit, text)`.
- **Calls other desks:**
  - research: market rent comps, vendor search;
  - Texture: lease documents, owner statements, notices;
  - engineering: bank CSV importers;
  - management: approve outbound notices and money moves.
- **Why an agent:**
  - month-end close as a background job: post rent, chase late payers, draft
    owner statements;
  - reconciliation by matching bank lines;
  - maintenance triage from tenant messages.

- **Object graph:**
  - **Kinds:**
    - property and unit → `choir.place`, with a `part_of` edge from unit to
      property;
    - tenants, owners and vendors → `choir.party`;
    - lease → `choir.lease`, with edges to its unit and its tenants;
    - the lease document itself → a Texture document, `rendered_as`;
    - recurring charges → metadata on the lease;
    - each posted charge, payment, bill or transfer → `choir.ledger_txn`;
    - chart of accounts → `choir.account` objects;
    - budgets and reconciliations → their own small kinds;
    - maintenance requests → `choir.work_order` (the same kind fleet uses).
  - **Maps very cleanly:**
    - `core/ledger.js` already treats transactions as immutable balanced
      line sets in integer cents;
    - "corrections are new transactions" already holds;
    - `forAccountId` and lease/unit/property refs on each line become edge
      metadata or line fields.
  - **The one change:** `ledgerIndex()` (balances memoized on the store's
    revision counter) should be keyed on the Dolt commit instead.
  - **Versioning:**
    - reconciliation and month close become Dolt **tags**, giving a
      "books as of close" read for free;
    - a branch is the right tool for a what-if (a rent increase across a
      portfolio, a refinance) before committing;
    - nothing is ever deleted, only superseded, which is what an auditor
      wants.
  - **Publish:**
    - chart-of-accounts and lease templates;
    - a market-rent comparison built from research captures (derived and
      aggregate).
    Tenants, ledgers and owner statements are private. An owner statement can
    be *shared* with a specific owner's computer as an adopted object, which
    is a direct-share case rather than a public publish.

**5. Priority: low** for the app. **Medium** for extracting the ledger core,
which is about 3–5 days to adapt into a shared Choir accounting module.

---

## 4. childcare (brightwheel / Procare clone)

**1. What / stack.** Covers check-in/out, a door kiosk with PINs, the
activity log (meals, naps, diapers, photos, incidents, learning
observations), staff ratios, family messaging, tuition billing and reports.
- Stack: vanilla JS, Vite, `lucide`.
- Size: about 155 files, about 20k lines of JS.
- Licenses: clean.

**2. Quality.** Solid domain modules.
- `src/core/attendance.js`: one record per child per day, holding an ordered
  event list. Its deterministic ids (`s-<studentId>`) mean two devices
  converge on the same record.
- `src/core/billing.js`: integer cents, with payments applied
  oldest-invoice-first and `planBillingRun`.
- The team worker (`public/worker/childcare-team-worker.js`, 867 lines) has
  field-level privacy:
  - `STUDENT_PRIVATE` and `GUARDIAN_PRIVATE` fields;
  - which fields parents may edit;
  - kiosk PIN brute-force throttling (`KIOSK_TRIES = 40` per 15 min, with
    exit-PIN attempts counted separately).
- 71 test files, about 328 cases.

The README itself says "Early alpha… do not use it for real children's
records".

Concerns:
- Kiosk PINs are 4-digit plaintext fields on guardian and staff records, and
  in single-user mode they are compared client-side (`src/views/kiosk.js`
  lookup).
- Children's health data (allergies, medications, immunizations, photos) sits
  in Puter KV and FS with no encryption at rest beyond the platform's.

**3. Worth taking.** Mostly ideas:
- the per-day event-list attendance record;
- field-level redaction by role, done server-side.

Low code reuse value.

**4. App-agent design.**
- **State:** per-day attendance event lists and activity records. Both are
  natural ledger events. Roster, guardians, staff and billing are objects.
- **Direct manipulation:** one-tap check-in, bulk activity logging, the
  kiosk.
- **Verbs:**
  - `check_in` / `check_out(child, by)`;
  - `log_activity(children, type, details)`;
  - `daily_report(child, day)`;
  - `ratio_status()`;
  - `billing_run(period)`;
  - `message_family(child, text)`.
- **Calls other desks:** Texture for daily reports and newsletters; research
  for licensing-regulation ratio rules.
- **Why an agent:** it can draft each child's daily report from the activity
  stream and flag ratio or allergy issues in real time.

This is high-sensitivity data about minors, which raises Choir's
authority-boundary bar.

- **Object graph:**
  - **Kinds:**
    - child → `choir.party` (minor facet);
    - guardians and staff → `choir.party`, with `guardian_of` and
      `authorized_pickup` edges;
    - room → `choir.place`;
    - attendance → one `choir.attendance_day` object per child per day.
      The deterministic id `s-<studentId>` becomes the canonical-ID suffix,
      so two devices converge;
    - activity → `choir.activity` (meal, nap, photo, incident), with edges
      to the children it covers and `attachment` edges to its photos;
    - tuition invoices and payments → line-item documents and
      `choir.ledger_txn`;
    - messages → whatever the teamchat/email apps use.
  - **Maps cleanly:** the records do.
  - **Does not map cleanly:**
    - attendance is an append-only *event list inside one record*, which is
      best stored as the object body. Each check-in is a new version, and
      Dolt history makes the timeline auditable, which matters for
      incidents and licensing;
    - field-level redaction by role (`STUDENT_PRIVATE`, `GUARDIAN_PRIVATE`
      in the team worker) has no direct object-graph equivalent. It needs
      either split objects (a private `choir.party_private` facet object
      linked to the public one) or a projection layer. Splitting is
      simpler and auditable.
  - **Publish: nothing.** These are minors' data. The only publishable items
    are templates (activity types, a daily-report layout). Sharing a daily
    report with a guardian's own Choir computer is a direct share of a
    Texture document, never the Autopaper graph.

**5. Priority: low.** It is a niche vertical with heavy privacy and regulatory
burden, so do not rebuild it near-term.

---

## 5. chess

**1. What / stack.** Chess against the computer or online (Puter Peer
WebRTC), with clocks, PGN, persistence and keyboard play.
- Size: about 8.4k lines. The engine is `src/engine/chess.js` (727), the AI is
  `src/engine/ai.js` (343), and `src/main.js` is a 2363-line monolith.
- **License flag:** `public/pieces/` are the Cburnett set under **CC BY-SA
  3.0** (`public/pieces/LICENSE.txt`). That is share-alike, not MIT. It is
  fine with attribution, but do not ship the pieces as "MIT".
- Plausible analytics are included.

**2. Quality.** It has a real rules engine:
- a 0x64 mailbox board and FEN load/save;
- castling masks, en passant, promotion;
- threefold repetition (with an ep-aware repetition key), the fifty-move
  rule and insufficient material;
- SAN/PGN, and a `perft` with perft tests (10 test files, about 135 cases).

The AI is basic and textbook:
- negamax with alpha-beta and MVV-LVA ordering;
- optional quiescence;
- the "Simplified Evaluation Function" piece-square tables, plus a mop-up
  term;
- max depth 3 with no transposition table or iterative deepening;
- difficulty set by a randomness window.

It is club-beginner strength.

**3. Worth taking.** Little. The rules engine is fine but `chess.js`
(BSD-2) and Stockfish-WASM (GPL) already exist. The *idea* of
difficulty-by-randomness-window is cute.

**4. App-agent design.**
- **State:** a game as a move list (PGN), which fits a ledger event per move
  exactly.
- **Verbs:** `new_game(opponent, time)`, `move(san)`, `analyze(position)`,
  `explain(move)`, `review_game(pgn)`.
- **Why an agent:** a *coach* is the interesting part. An LLM desk narrates
  why a move is bad using engine evaluations as evidence, and research pulls
  opening theory. This is a good, low-stakes demo that "an external agent can
  drive an app window": two agents play each other on the ledger with a
  human watching.

- **Object graph:**
  - **Kinds:**
    - `choir.game`, with variant `chess`, players as `choir.party` or agent
      ids, and the time control;
    - moves as the body (a PGN). Every move is a new version, so the game's
      Dolt history *is* the move list;
    - `choir.position_analysis` (an engine evaluation plus a coach
      explanation), linked `annotates` to a game version.
  - **Versioning:**
    - "take back" is a revert;
    - **analysis variations are branches** off a game version: the exact
      semantics of a PGN variation tree, for free;
    - when two agents play, each move is a commitment on the ledger.
  - **Maps cleanly:** `chess/src/game/persistence.js` already serializes the
    game state as JSON plus PGN.
  - **Publish:** finished games, if both players agree, and annotated
    studies or puzzles. Opening-theory captures come from research as
    `choir.web_capture`. In-progress games and personal stats stay private.

**5. Priority: low** (medium as a demo of external-agent-drives-window). It
would take about 2–3 days using an existing engine.

## 6. checkers

**1–2.** American checkers, a deliberate port of the chess architecture
(`docs/superpowers/specs/2026-08-13-checkers-port-design.md`).
- Rules engine: `src/engine/checkers.js` (329). It works in whole turns, with
  a multi-jump as a single Move with a path, mandatory capture, and crowning
  that ends the turn.
- AI: `src/engine/ai.js` (166), alpha-beta to depth 8. The comment says depth
  10 was measured at about 3.6s per move.
- PDN notation and Peer online play are included.
- 14 test files, about 127 cases.
- Licenses: clean.

**3.** Nothing beyond the "whole-turn move" design idea.

**4.** Same shape as chess, with a smaller verb set.


- **Object graph:** the same `choir.game` kind with variant `checkers` (PDN
  body). This is an argument for one game kind across all board games
  rather than one per app.

**5. Priority: low.** If a board game is wanted, take one, not three.

## 7. backgammon

**1–2.** This is the most interesting game.
- Engine: `src/engine/backgammon.js` (861) covers the cube, Crawford rule
  handling and bear-off.
- AI: `src/engine/ai.js` (387) runs a 1–2 ply **expectimax over the 21
  distinct rolls**, with a hand-written evaluation in pip units:
  - race, blot exposure (using a `SHOTS` table of hits per distance), primes,
    anchors, wastage;
  - cube decisions from a hand-fit logistic `winProbability`.
- The file is honest that the weights are "judgement rather than
  measurement" and not rollout-fit.
- **`src/engine/dice.js`** implements **commit-reveal dice for online play**:
  - the roller sends `hash(nonce)`;
  - the opponent replies with a nonce in the clear;
  - the roller reveals, and both derive the dice;
  - so neither peer can steer the roll.
- 13 test files, about 212 cases.
- Licenses: clean.

**3. Worth taking.** **The commit-reveal fair-randomness scheme** is a tiny,
relevant pattern for Choir. It is a commitment protocol between two parties
who do not trust each other, which is directly analogous to precommitment
records. The expectimax-over-chance structure is a good teaching example.

**4. App-agent design.**
- **State:** moves plus dice commitments on the ledger. The commit/reveal
  records are themselves the fairness proof.
- **Verbs:** `roll` (commit/reveal), `play(moves)`, `offer_double`,
  `take/drop`, `evaluate(position)`.
- **Why an agent:** a coach that explains equity and cube decisions. It is
  also a fun demo of verifiable randomness between agents on Choir.

- **Object graph:**
  - **Kinds:**
    - `choir.game` with variant `backgammon`. The body holds the moves and
      the cube history;
    - each online roll is a pair of objects: the `dice_commit` (hash) and
      the `dice_reveal` (nonce). These map naturally onto
      `choir.commitment_record`, a commit made before the counterparty's
      contribution and resolved by a reveal;
    - the match → `choir.match` linking games, for match score and the
      Crawford game.
  - **Versioning:** moves are versions, and the history plus the
    commit/reveal records form a replayable fairness proof.
  - **Publish:** finished matches if both agree. The commitment records are
    the interesting part to publish as a worked example of verifiable
    randomness between computers.

**5. Priority: low** as a game. The dice-commitment idea is worth noting for
precommitment-record design (about 0 effort, just a reference).

## 8. solitaire

**1–2.** Klondike, draw-one only. It is the smallest app (about 3.5k lines
including CSS and tests).
- Rules: `src/game/rules.js` is 20 lines and `src/game/state.js` is 120
  lines, with snapshot-based undo through JSON deep copies, capped at 200.
- There is no solver, no winnability check, and no draw-three.
- Tests are colocated in `src/**/*.test.js`.
- Effort went into animation (`ui/animate.js`, `ui/cascade-physics.js` for the
  win cascade).
- The card art is vendored `src/vendor/cardmeister.js`: **Unlicense, with
  art lineage CC0**. That is fine.
- `docs/superpowers/` shows agent authorship.

**3.** Nothing. CC0 card art is available upstream directly.

**4.** It is not a meaningful agent surface (a hint or auto-play verb at
most).


- **Object graph:** a `choir.game` with variant `klondike`. The deal seed is
  in metadata and moves are versions, so undo is a revert. This replaces
  `src/game/state.js`'s 200-entry JSON snapshot stack. Publish nothing,
  except perhaps a "deal of the day" seed.

**5. Priority: low / skip.**

## 9. sudoku

**1–2.** This is the best-engineered game engine in the group.
- `src/engine/grid.js` uses bitmask candidates.
- `src/engine/solver.js` is a uniqueness-checking solver.
- `src/engine/techniques.js` (605) is a **human technique ladder**: full
  house, naked/hidden singles, pointing, claiming, naked/hidden pairs and
  triples, X-Wing, Y-Wing. The same ladder serves two purposes:
  - **hints**: name the technique, point at the cells, say why;
  - **grading**: difficulty means the hardest technique required, not the
    clue count.
- `src/engine/generator.js` digs a full grid and re-checks uniqueness and
  difficulty after each clue removal. It runs in a Web Worker with an inline
  fallback.
- There are seeded daily puzzles.
- `src/game/hints.js` reports a wrong digit before offering a logic hint.
- 16 test files, about 306 cases.
- Licenses: clean.

**3. Worth taking.** The *design pattern* "one explainable rule ladder powers
both hints and grading". It is a clean template for an explainable tutor or
coach agent: hints are evidence-bearing reasoning steps, not answers. The
engine itself is small and could be lifted as-is if Choir ever wants a
puzzle.

**4. App-agent design.**
- **State:** the puzzle, the move list and eliminations.
- **Verbs:** `new_puzzle(difficulty|seed)`, `place(cell, digit)`,
  `next_hint()` (which returns {technique, cells, reason}), `grade(puzzle)`.
- **Why an agent:** a tutor that adapts by tracking which techniques the user
  has not yet used. It is a good small proof of "agent returns structured,
  checkable reasoning" rather than prose.

- **Object graph:**
  - **Kinds:**
    - `choir.puzzle` (givens, seed, solution hash, and the grade with the
      hardest technique);
    - `choir.game` (variant `sudoku`), with an `instance_of` edge to the
      puzzle. The body holds the placements and eliminations;
    - hints as `choir.hint` objects (technique, cells, reason) linked
      `annotates` to the game version they explain. This is a structured,
      checkable reasoning trace.
  - **Versioning:** undo and redo are history. A branch is "try this
    guess", exactly what players do with pencil marks.
  - **Maps cleanly:** `src/game/serialize.js` and `src/game/state.js` are
    already compact.
  - **Publish:** puzzles and their grades are ideal Autopaper-graph objects.
    They are content-addressed (the canonical-ID suffix comes from the
    content hash), carry no personal data, and are reusable by every
    computer, e.g. a shared daily puzzle. Personal games and stats stay
    private.

**5. Priority: low.** If one game is ever built as an app-agent showcase,
sudoku or backgammon is the pick (about 2–3 days to port).

---

## Group summary

**Top picks (to read and adapt, not to import):**

1. **fleet `core/store.js` + `core/schema.js` + `core/backend.js`.** A
   local-first record store with:
   - an 8-method backend interface;
   - month-partitioned buckets with version counters;
   - optimistic writes, an offline queue and atomic numbering.

   This is the best reference here for how a Choir app window holds and syncs
   state against the computer (swap Puter KV for computer records or ledger
   events). Field-service, property-management and childcare are forks of the
   same pattern.
2. **property-management `core/ledger.js` + `core/financials.js`.** A pure,
   tested double-entry ledger with statements and reconciliation. It is a
   candidate shared accounting core for any money-bearing Choir app desk.
3. **sudoku `engine/techniques.js`.** The "explainable rule ladder used for
   both hints and grading" pattern, a template for coach agents.
4. **backgammon `engine/dice.js`.** Commit-reveal fair randomness between
   untrusting peers. It is a tiny precommitment-protocol exemplar.

**Cross-app reusable pieces:**
- the backend interface plus memory backend (a testable storage seam);
- `applyEdits` 3-way merge-on-save (`field-service/src/core/workflow.js`);
- the team-worker pattern: an owner-owned server that identifies the caller
  from their session (never the body), with role write-sets and field-level
  redaction. This maps onto an app desk admitting other principals;
- the email-worker anti-abuse design;
- the NHTSA VIN and recall lookups (a research-desk tool).

**Object-graph verdict** (owner addition; see section 0b):
- All nine apps' data models are flat id-keyed JSON records with id
  references, so they map cleanly onto `objectgraph.Object` + `Edge`.
- What does *not* carry over is each vertical's sync layer (buckets, version
  counters, merge-on-save), which existed only because Puter KV lacks
  history. Dolt commits replace it.
- The value of shared state comes from a handful of cross-app kinds, not
  per-app kinds:
  - `choir.party`;
  - `choir.place`;
  - `choir.ledger_txn` (seeded from property-management's ledger);
  - `choir.calendar_event`;
  - `choir.catalog_item`;
  - `choir.work_order`;
  - line-item documents;
  - `choir.game` (one kind for every board and card game).
- Existing kinds are reused for web data (`choir.web_capture`), citations
  (`choir.source_ref`), prose (Texture), and inter-desk asks and fair-dice
  commitments (`choir.commitment_record`).
- **Privacy split:** vertical business, financial and minors' data is
  strictly private. Autopaper-graph candidates are generic reference
  objects:
  - vehicle specs and recalls;
  - inspection, service-program, chart-of-accounts and lease templates;
  - graded sudoku puzzles;
  - consented game records.
- Childcare's field-level role redaction is the one model that needs a design
  decision (split private facet objects vs a projection layer).

**Shared infrastructure verdict:**
- There is no shared component library or monorepo. Every app is an
  independent Vite SPA, deployed to Amplify per folder.
- Commonality comes from copy-paste templates and from **puter.js** (auth,
  KV, FS, workers, peer, email), loaded unpinned from a CDN.
- Choir should treat puter.js as the conceptual analogue of a per-window
  desk/record SDK, not as code to adopt.

**Alarms:**
- **Licenses:**
  - chess pieces are **CC BY-SA 3.0** (share-alike) inside an "MIT" repo;
  - cardmeister is Unlicense/CC0, which is fine;
  - everything else is ISC/MIT;
  - no GPL/AGPL in this group.
- **Security / supply chain:**
  - puter.js is loaded unpinned from `js.puter.com/v2/` with no SRI;
  - Plausible analytics are in the games;
  - the shared `ui/dom.js` `h()` helper has an `html` key that sets
    `innerHTML` (low count of uses, but a footgun).
- **Childcare:** minors' health data, and 4-digit plaintext kiosk PINs
  compared client-side in single-user mode. It is self-declared alpha.
- **Quality:** the code is agent-generated (superpowers specs, `review2-*`
  test files), but it is disciplined, well-tested domain code. It is not
  slop in the cores. The UI layers (2–3k-line `main.js`, 1k-line view files)
  are the weak part and are not worth taking.
- **Short-term rebuild priority:** all low. Vertical SMB apps and games are
  not where Choir's near-term value is. Extract the ledger and the store
  pattern instead.
