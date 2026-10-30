# Finance group review: accounting, invoicing, expense, cap-table, pos, inventory, records, assets

Reviewer scope: `src/{accounting,invoicing,expense,cap-table,pos,inventory,records,assets}` in the
HeyPuter/everything shallow clone (commit a7f61cdc). I read the code and did not run it: no npm, no tests
executed. Test counts are `it(`/`test(` occurrences, not pass results. Paths below are relative to
`repo/src/`.

## Cross-cutting facts (read first)

- **Stack.** All eight are vanilla-JS ES modules built with Vite. The tests are Vitest, some with jsdom,
  happy-dom or fake-indexeddb. The only runtime dependencies are `lucide` (ISC) and, in assets,
  `qrcode-generator` (MIT). There is **no GPL/AGPL or non-OSI dependency**. records even ships a
  license gate (`records/tools/check-licenses.mjs`). The real platform dependency is **Puter.js loaded
  unpinned from `https://js.puter.com/v2/`** at runtime: auth, KV, FS, hosting, workers and `puter.ai`.
  Every app's whole persistence layer is written against Puter KV/FS, so none of the storage code
  ports.
- **Provenance and style.** The code is clearly machine-generated and then iterated through review
  rounds. Signs: very long narrative comments that recount earlier bugs ("used to…"), test files named
  `review-*.test.js` and `regressions.test.js`, and giant single files (`cap-table/src/lib/store.js`
  7,264 lines, `captable.js` 5,031, `pos/src/views/register.js` 2,632). Even so, it is **not slop where
  it matters**. The pure domain modules are careful, internally consistent and heavily unit-tested. The
  "slop" is volume, not wrong math.
- **The substrate finding.** Four apps each built a private sync/journal layer to make up for Puter KV's
  last-writer-wins:
  - accounting: `src/lib/sync.js`, 558 lines (rev counter, change log, outbox).
  - cap-table: `src/lib/audit.js` (526) plus `kv.js` (205). An intent/compensation saga over KV
    compare-and-set.
  - inventory: `src/lib/persistence.js`. An immutable `inv:op:v1:<uuid>` op journal with logical
    clocks.
  - records: `src/data/journal.js`. A per-tab session journal.

  All of this is what Choir's Dolt-backed object graph already provides:
  `internal/objectgraph/store.go` has `ConditionalBatchStore.PutBatchConditional`, which is
  compare-and-write in one Dolt transaction, and Dolt gives commit history and branches. **Port the
  domain modules and drop every storage/sync module.**
- **Money handling, by app.**
  - **Integer minor units:** accounting throughout, and the pos cart math.
  - **Float major units with `round2`:** invoicing, expense, cap-table money, inventory costs, and the
    records currency field.

  Details are in each section. One rule for Choir: store integer minor units, keep an explicit currency
  and exponent on every amount, and never use floats on the object body.

---

## 1. accounting (QuickBooks-style double-entry books)

**What and size.** About 13k lines of source and CSS plus 1.6k lines of tests (137 cases). The pieces:
- Domain layer in `src/lib/`: `money.js` 165, `txn.js` 525, `ledger.js` 273, `reports.js` 922,
  `store.js` 812, `bankimport.js` 217, `banking.js` 158, `reconcile.js` 119, `recurring.js`.
- Storage/sync: `sync.js` 558 and `idb.js`.
- UI: `forms/`, `views/` and `ui/`.

**Quality: real engine, the best code in the group.**
- `money.js` keeps all money as **integer cents**. It has largest-remainder `allocate()` (exact
  proration with signed weights), basis-point tax rates (`applyRate`), and a careful locale parser
  (`parseMoney` handles parentheses, U+2212, and decimal-comma detection).
- `txn.js` is a real **posting rulebook**. There are 17 transaction types: invoice, payment, estimate,
  sales receipt, credit memo, refund receipt, expense, check, bill, bill payment, vendor credit, cc
  credit, PO, deposit, transfer, journal and cc payment.
  - `accrualPostings()` and `cashPostings()` are pure functions from a document to postings, and every
    list sums to zero.
  - Cash basis prorates each payment over the paid invoice's revenue lines with `allocate()`, so
    cash-basis P&L ties to the bank to the cent.
  - `withSnapshots()` freezes the tax rate and item income/expense accounts onto each posting document
    when it is saved, so later edits to rates or items never rewrite past periods. `freezeOldValues()`
    back-fills older records.
- `ledger.js` is a column-oriented general ledger. Postings sit in `Int32Array`/`Float64Array` sorted
  by day, with counting-sort per-account indexes. Floats hold exact integers below 2^53, so this is
  safe. It provides date-range binary search, registers with running balances, and an `imbalance()`
  invariant.
- `reports.js` covers P&L, balance sheet, trial balance, GL, cash flow, aging, sales tax and more.
  Retained earnings is computed virtually from the fiscal-year start, with no closing entries. That is
  correct.
- `store.checkTxn()` (`store.js:531`) enforces:
  - no over-application of payments or credits;
  - same customer/vendor across links;
  - a closing-date lock, with a narrow exception for applying old credits (`_onlyAppliesCredit`);
  - deposited payments cannot change amount;
  - a document cannot shrink below what has already been paid on it.
- Tests assert `imbalance() === 0` on both bases (`tests/books.test.js:42`).

**Weaknesses.**
1. **Documents are mutable in place.** `saveTxn` overwrites, `voidTxn` mutates, and `deleteTxn` removes
   the record (`store.js:623`). The audit trail is only a "recent changes" log.
2. **Single currency per company, and 2 minor digits hard-coded.** `formatMoney` forces 2 fraction
   digits, and `toCents` is ×100. JPY displays as "¥1,234.00", and 3-decimal currencies (KWD, BHD)
   cannot be represented. There is no FX.
3. **Multi-device conflicts.** Cross-record invariants (for example, total applied ≤ invoice total)
   are validated only against the local copy, then synced last-writer-wins per record. Two devices
   applying payments at the same moment can over-apply an invoice.
4. **No inventory/COGS posting** (items are not tracked stock) and **no payroll**.

**Worth taking.**
- The **posting-rules engine**: `txn.js` plus `money.js` (`allocate`, `applyRate`, `parseMoney`,
  `lineAmount`). These are the real asset. They port directly to Go as pure functions.
- The report definitions in `reports.js`.
- `bankimport.js`, an OFX 1.x/2.x and CSV statement importer with FITID/loose dedupe, plus the
  bank-rule and match heuristics in `banking.js` (`findMatches`, `suggestFromHistory`).
- The reconciliation model (`reconcile.js`), which stores cleared ids and amounts so later edits show
  up as discrepancies.

**App-agent design (object graph).**

Proposed object types:

| Kind | Identity | Notes |
|---|---|---|
| `choir.party` | external key, versioned | customer/vendor; **shared** with invoicing, expense, cap-table stakeholders, pos customers, and contacts/CRM |
| `choir.fin_account` | external key, versioned | chart of accounts; `role` (ar, ap, undeposited, salesTax, retainedEarnings…) and type |
| `choir.fin_item` | external key, versioned | product/service; **shared** with invoicing, pos, inventory |
| `choir.fin_tax_rate` | external key, versioned | basis points |
| `choir.fin_txn` | external key, **versioned** | one object per business document. Body = today's record, but amounts are `{minor:int, ccy, exp}`, plus `applied[]`, line snapshots and `voided` |
| `choir.bank_line` | content hash | an imported statement line; FITID in metadata |
| `choir.reconciliation` | external key | statement date, ending balance, cleared `{txnId, amount}` |
| `choir.fin_period_close` | external key | closing date; a Dolt tag/commit ref per close |

Edges (all derived from the bodies for traversal; the body stays the truth):
- `settles`: payment/credit → invoice/bill;
- `deposits`: deposit → payment;
- `party_of`: txn → party;
- `matched`: bank_line → txn;
- `evidences`: capture/receipt → txn;
- `converted_from`: invoice → estimate;
- `bills_customer`: purchase line → party.

**Postings are a projection, not objects.** `postings(txn, basis)` is pure. Recompute the ledger, as
the app already does, or keep it as a derived Dolt table. Then add a `WriteValidator`
(`dolt_store.go:81`) that rejects any `fin_txn` version whose postings do not sum to zero, and any batch
that would over-apply or that touches a locked period. Save, apply and void run as one
`PutBatchConditional` with `ExpectedVersionID` on the invoice and the payment. That gives a
**server-side guarantee** of the multi-record invariants this app can only check locally.

**A ledger on a versioned database is the natural fit.**
- Every document edit is a Dolt commit, so complete history and "who changed this posted invoice" come
  free. That replaces the mutable-record weakness and the audit log.
- "Reconciled transactions changed afterwards" becomes a `dolt diff` since the reconciliation's commit.
- Period close becomes a tag, and "as filed" reports are reads at that tag.
- **Branches carry agent proposals.** For example: the bookkeeping agent recategorizes 300 bank lines
  or proposes year-end adjusting entries on a branch, the owner reviews the diff, then merges. That is
  supervised self-development applied to books.

**Direct manipulation:** enter or edit a document, categorize or match a bank line, tick off items in a
reconciliation, drill down from a report.

**Desk verbs:**
- `post_document(type, draft)` (returns validation errors);
- `void` / `apply_payment`;
- `import_statement(file)`;
- `categorize_lines(rule|suggestions)`, done on a branch;
- `reconcile(account, statement)`;
- `run_report(id, period, basis)`, which returns a cited, typed result;
- `close_period(date)`, which requires the owner's act;
- `explain_balance(account, date)`, which walks postings back to documents.

**Desks it calls:**
- research: FX rates and tax-rate lookups with citations, and vendor identification from cryptic
  statement descriptions;
- Texture: monthly close memo and narrative P&L;
- engineering: a new bank CSV dialect becomes a capsule-built importer.

**Better as an agent:** categorizing and matching bank feeds is the 80% of bookkeeping drudgery, and it
is model work. The app already has rules plus history suggestions (`banking.js:143`) for the agent to
extend.

**Publish vs private.** Books are **private**; nothing goes to the Autopaper graph by default.
Publishable reference objects other computers can adopt:
- chart-of-accounts templates;
- jurisdiction tax-rate tables, as research-cited objects;
- bank CSV dialect mappings and statement-importer specs.

**Priority: HIGH**, as the engine every other finance app writes into. Effort the Choir way:
- Go port of `money`/`txn`/`ledger`/`reports` plus the validator: about 3–4k LOC, 2–3 weeks.
- Svelte window over it: 1–2 weeks.
- Bank import and agent categorization: 1 week.

## 2. invoicing (quotes, invoices, payment links, PDFs)

**What and size.** About 22k source lines plus 7k test lines (745 cases).
- Domain: `src/lib/money.js` 320, `doc.js` 295, `aging.js` 240, `reports.js` 408, `store.js` 1,918.
- `pdf.js` 786 + `pdf-spec.js` 347: a dependency-free PDF writer.
- `xlsx.js` 387: a dependency-free first-sheet .xlsx reader using `DecompressionStream`.
- `i18n.js` 581.
- `workers/email-worker.js` 753: a Puter worker that relays email.

**Quality.** A real, careful document model in **float major units**:
- `round2()` is `Math.round((n+EPSILON)*100)/100`.
- Multiple named taxes per document and per line, tax-inclusive mode, a discount prorated before tax,
  deposits, installment schedules filled in date order (`scheduleRows`), refunds as negative payments,
  write-offs, and manual FX per document and per payment ("absent means not converted, never a guessed
  1").

Weaknesses:
- Rounding is consistent (per-row tax rounded, total = taxable + Σrows), but it is floats with
  2-decimal rounding for every currency, including JPY.
- Storage writes **one JSON file per issue year** (`invoicing-data/docs-YYYY.json`) and serializes
  writes only within a tab. Two devices editing invoices from the same year overwrite each other's
  whole-year file.
- Clients and items are whole-list KV entries, with the same problem.
- HTML rendering escapes consistently (60 `esc()` calls in `ui/paper.js`).

The email worker is an **app-owner relay** of user-written text to arbitrary recipients. It is
rate-limited per user and per day, links are restricted, and sends are billed to the user's Puter token.
It is thoughtfully done, but Choir should not copy the pattern: email is a policy-gated channel.

**Worth taking:**
- `pdf.js` and `pdf-spec.js`, an MIT, zero-dependency text PDF writer (WinAnsi and base-14 Helvetica
  only, no CJK).
- `xlsx.js`, a minimal reader for importing line items.
- The installment-schedule and late-fee rules (`latefee.js`), the per-line multi-tax model, and the
  tax-note concept (reverse charge or exemption text printed in the document's language).
- Aging buckets.

**App-agent design.** **Do not give it its own store.** An invoice is a `choir.fin_txn` of type invoice
in the accounting graph. Invoicing is a *window and a desk role* over the same objects. Its extra kinds
and edges:
- `choir.fin_doc_render`: content hash, the PDF bytes or a manifest. Edge `rendered_from` points to the
  exact `fin_txn` version, so "the PDF the client received" is pinned to a revision.
- `choir.delivery`: who, when, channel, version digest. Edge `delivered` → `fin_txn`.
- `choir.recurring_schedule`: a standing intent. Under the restart rule it is a standing owner intent;
  a crash closes only the occurrence in flight.
- `references`: invoice line → any object that justifies the charge. That can be a time entry, a CAD
  part, a Texture deliverable or a `choir.capture`. This cross-app link is the real win: "bill for the
  report Texture wrote this month" becomes a graph query.

Versioning gives "what did version 3 that we sent say" and undo for free. A draft is just an
unpublished version.

**Desk verbs:**
- `draft_invoice(party, from: [objects])`;
- `send(invoice, channel)`, which needs the owner's act or a standing policy;
- `remind(overdue_policy)`;
- `record_payment`;
- `statement(party)`.

**Desks it calls:** Texture for wording of reminders and cover notes, research for the client's legal
or billing details and tax ids, and accounting for posting.

**Publish vs private.** Private. A shared invoice page should be a **capability link to one rendered
version**, not an Autopaper publication. The app's `*.puter.site` share-page design is the model to
avoid copying literally. Only templates and branding are publishable.

**Priority: MEDIUM.** It falls out of accounting. On top of that, add PDF, delivery, recurring and the
window, about 1–1.5 weeks.

## 3. expense (receipt capture, AI OCR, expense reports, statement match)

**What and size.** About 6.5k lines, 79 tests. `src/lib/store.js` 537, `ocr.js` 176, `reconcile.js` 83,
`csv.js` 196, `money.js` 80.

**Quality: thin but sound.**
- Money is float with `round2`, totals are kept per currency, and nothing is converted.
- OCR is a strict-JSON prompt to `puter.ai.chat` with vision (default model `gpt-5.6-luna`, through
  Puter), falling back to `img2txt` plus extraction.
- `normalizeOcrResult` is defensive: it rejects future dates and fuzzy-matches the category.
- Statement matching (`reconcile.js`) scores amount, date lag and merchant tokens, then pairs greedily.
- Report workflow: draft → submitted → approved → reimbursed. With a single user, "approved" carries no
  separation of duties.
- Storage is one FS file per month plus whole-file reports and statements, with per-path queues within
  a tab only.

**Worth taking:** only ideas. The OCR JSON contract and normalizer (`ocr.js:13–160`), and the
match-scoring function. accounting's `banking.js` supersedes the statement import.

**App-agent design.** This is the **most agent-shaped app in the group**, and it should not be a
separate ledger. Objects:
- `choir.receipt_capture`: content hash, an image/PDF blob plus metadata. It is a sibling of
  `choir.web_capture` and `choir.media_item`, so a receipt photo is a capture like any other.
- `choir.extraction`: content hash. The OCR result, with model id and confidence. Edge `derived_from`
  → capture. It is the agent's evidence.
- The expense itself is a `choir.fin_txn` (expense or bill) with edge `evidences` from the capture.
- `choir.expense_report`: versioned, with edge `contains` → fin_txns. A reimbursement is a
  `fin_txn` of type billPayment that `settles` it.

**Flow:** a photo is dropped, emailed in, or pulled from email attachments. The agent extracts, matches
the bank line, and drafts the expense **on a branch**. The owner confirms. Matching against bank lines
uses accounting's matcher.

**Desks it calls:** research (merchant resolution, FX on the receipt date, cited) and accounting
(posting).

**Better as an agent:** capture-to-books with no forms, plus receipts found automatically for unmatched
card charges.

**Publish:** nothing; receipts are private.

**Priority: MEDIUM.** It is high-value once accounting exists and adds about 1 week (a capture kind,
the extraction prompt, a confirm UI).

## 4. cap-table (Carta/Pulley-style equity management)

**What and size.** The largest app: about 30k source lines and 1,145 tests.
- `src/lib/captable.js` 5,031: pure math.
- `store.js` 7,264.
- `financing.js` 548 and `financing-waterfall.js` 252.
- `reports.js` 778: ASC 718 Black-Scholes, Rule 701, Form 3921.
- `audit.js` 526 and `csvimport.js` 899.
- A marketing `site/` with SEO/landing docs.

**Quality.** A **real domain engine**, deeper than the rest of the group:
- securities: shares, options (ISO/NSO, early exercise), RSUs, warrants, pre/post-money SAFEs and notes;
- vesting with cliffs, tranches and single/double-trigger acceleration;
- plan reserves with recycling rules, stock splits restated by effective date, and as-of-date
  ownership;
- round modeling as a damped **fixed-point iteration** (`modelRoundBase`, `captable.js:2937`) covering
  post-money SAFE circularity, discount floors, anti-dilution and the pool top-up, with explicit refusal
  when it cannot converge;
- an exit liquidation waterfall with breakevens.

Share counts are integers via `Math.round`. Money and price-per-share are floats.

**Fidelity flags:**
- Investor and conversion shares use `Math.round(value/price)` with an unrounded PPS. Common practice
  (NVCA) rounds PPS to a fixed number of decimals and **floors** shares. Treat the numbers as modeling
  outputs, not closing documents.
- The ASC 718 and 409A-adjacent reports carry legal and audit weight. Expert review is needed before
  anyone relies on them.

**Over-engineering:** `audit.js` and the 7k-line `store.js` implement a durable-intent/compensation
journal over KV CAS. Choir's Dolt history plus conditional batches make it unnecessary.

**Worth taking:**
- `captable.js` as a **reference implementation and test oracle**: vesting, fully diluted, conversion
  and the waterfall. Port selectively.
- `docs/financing-conventions.md` and `docs/INCENTIVE_PLANS.md` as specs.
- The security data model in the header comment of `captable.js:1–60` is clean and maps 1:1 to objects.

**App-agent design.** Objects:
- `choir.equity_company`;
- `choir.share_class`: versioned; authorized shares, preference, participation, anti-dilution;
- `choir.equity_plan`: versioned, with dated reserve history;
- `choir.security`: external key, versioned, with a kind discriminator exactly as in the
  `captable.js` header;
- `choir.valuation_409a`;
- `choir.round_scenario` and `choir.exit_scenario`.

Edges:
- `holds`: party → security, reusing `choir.party` from accounting;
- `of_class`: security → class;
- `draws_from`: security → plan;
- `converted_into`: SAFE → shares;
- `succeeds`: transfer certificate → closed certificate;
- `evidences`: signed agreement or 409A PDF → security.

**Dolt mapping is excellent.** As-of-date ownership becomes a read at a commit, and the audit history
comes free.
- **A round model is a branch.** "Close the round" is a merge of a reviewed branch: certificates issued,
  convertibles marked converted, pool resized, all in one commit.
- The app's restore points become Dolt refs.

**Cross-app links:**
- the round's cash is a `fin_txn` deposit in accounting, linked by `funded_by`;
- option exercise proceeds post to equity;
- ASC 718 expense is a journal entry the agent proposes into accounting.

**Desk verbs:**
- `issue(security)`;
- `model_round(terms)` → a scenario branch;
- `close_round(branch)`, which needs the owner's act;
- `exercise` / `cancel` / `depart(party)`;
- `waterfall(exit_value)`;
- `investor_update()`, delegated to Texture with a cited ownership table;
- `needs_attention()` (expired grants, unfiled 83(b)s; the app already computes this).

**Desks it calls:** research (SAFE/NVCA term conventions, comparable valuations, cited) and Texture
(board consent drafts, investor updates).

**Publish vs private.** Strictly private, and investor-confidential. At most, owner-approved aggregate
facts ("capital raised", "post-money") can be published as a Texture.

**Priority: LOW short term.** It is niche, legally sensitive and large; MEDIUM if the owner runs Choir
as a company. Effort: a Go port of the core math (vesting, FD, conversion, waterfall) at about 3–4k LOC,
3–5 weeks, using the upstream tests as an oracle.

## 5. pos (point of sale register)

**What and size.** About 25k source lines and 533 tests.
- `src/lib/store.js` 3,737 and `views/register.js` 2,632.
- `totals.js` 180: cart math in **integer cents**.
- `money.js` 173, `refunds.js` 182, `drawer.js` 115.
- `escpos.js` 363: a thermal printer byte encoder for CP437 at 32/48 columns.
- `outbox.js`, `loyalty.js`, `pin.js`, `scan.js`.

**Quality: solid, retail-specific.**
- The cart computes in cents with largest-remainder order-discount proration capped per line, and
  inclusive or exclusive tax.
- Stored amounts are decimal hundredths as floats, converted at the edges; `parseAmount` uses
  `toPrecision(12)` to recover the typed decimal.
- Refunds are separate numbered receipts, so the original is never rewritten.
- The offline outbox uses localStorage so a sale is durable before the success screen.
- The manager PIN is hashed with PBKDF2 at 600k iterations and honestly labelled "a counter-height
  barrier".

Weaknesses:
- **Stock is a mutable `stock` field inside a whole-array KV entry** (`pos:products:v1`). Two registers
  lose stock updates; a "stock debt" localStorage patch works around it.
- Loyalty is derived from receipts with no parallel ledger, which is good.

**Worth taking:**
- `escpos.js`, if Choir ever prints receipts or labels; it is pure and MIT.
- The `totals.js` cart algorithm.
- The rule that refunds are new immutable documents.

**App-agent design.** Objects:
- `choir.pos_sale`: content hash, immutable; lines with a `fin_item` ref, tenders, tax breakdown;
- `choir.pos_refund`: immutable, edge `refunds` → sale;
- `choir.drawer_session`: versioned, open/close counts.

Projections:
- each sale emits a `choir.stock_movement`, the inventory kind below;
- a daily Z-report becomes one accounting `fin_txn` (a sales-receipt summary), edge `summarizes` → sales.

Immutable sales on Dolt map cleanly. The mutable stock counter is the part that does **not** map, and it
should become movements.

**Desk:** mostly direct manipulation; checkout must not wait on an agent. Agent verbs:
`end_of_day()`, `reprice(items)`, `sales_report()`.

**Publish:** the product catalog and price list can be published as a storefront object other computers
adopt. Sales are private.

**Priority: LOW.** Retail hardware is out of scope for a personal persistent computer. Effort is about
2–3 weeks if ever needed.

## 6. inventory (stock, locations, suppliers, reorder)

**What and size.** About 24k source lines and 669 tests.
- `src/lib/store.js` 3,059 and `persistence.js` 235: an op journal with logical clocks.
- `stock.js` 244, `lots.js` 202 (FEFO), `kits.js` 187, `variants.js` 200, `reorder.js` 266,
  `demand.js` 135, `replenish.js` 159, `stocktake.js` 254.
- `csv.js` 652 and `barcode.js` 135.

**Quality: competent.**
- Per-location stock is a map `{locationId: qty}`, and the cached total is rewritten on every change.
- Measured units allow 3 decimals.
- Cost is a **moving average** (`store.js:1291`), in float `round2`.
- Kits come in two modes, build (components out, kit in at component cost) and sell (explode on sale).
- Variants hang off a family.
- Demand-based reorder points are suggested but never auto-applied.

The **stock map is the truth** and the activity log is history. The author added an immutable op
journal (`persistence.js`) so concurrent sessions merge, with stock deltas accumulated rather than
overwritten. That is the correct instinct, built on the wrong substrate.

**Worth taking:**
- `kits.js` (buildable quantity, kit unit cost) and `lots.js` (FEFO consumption).
- The algorithms in `demand.js` and `reorder.js`.
- The data model in the `store.js:1–80` header.

**App-agent design.**
- **Invert it.** Make `choir.stock_movement` (content hash, immutable: item, location, ±qty, unit cost,
  reason, source ref) the truth, and stock-on-hand plus moving-average cost a projection. That is the
  same ledger pattern as accounting, and a Dolt-versioned movement log is the natural fit.
- Other objects:
  - `choir.fin_item` (shared), with inventory metadata such as bins and minimums;
  - `choir.location`;
  - `choir.supplier` (a `choir.party` role);
  - `choir.purchase_order` (a `fin_txn` of type purchaseOrder);
  - `choir.stocktake`: versioned, its commit creates adjustment movements.
- Edges: `moved_by` (movement → pos_sale, PO receipt or stocktake) and `component_of` (kit).
- Valuation posts to accounting as an inventory asset with COGS. accounting currently lacks this, so
  it would be new work.

**Desk verbs:** `receive(po)`, `adjust`, `count(scope)`, `reorder_suggestions()`,
`what_runs_out(days)`.

**Desks it calls:** research for supplier prices and lead times, cited.

**Publish:** a public catalog or availability object at most. Everything else is private.

**Priority: LOW.** It matters only together with pos or a small-business owner. Effort is about 2 weeks.

## 7. records (Airtable-style relational base)

**What and size.** About 23k source lines and 437 tests, with axe accessibility checks.
- `src/data/formula.js` 1,036: lexer, parser, 84 functions, compiled to closures. There is **no
  `eval`/`new Function`**.
- `compute.js` 406: lazily cached formula, lookup, rollup and count values with cycle/stack guards.
- `query.js` 577: filter groups and date conditions.
- `fields.js` 687: 27 field types with type conversion.
- `base.js` 1,199 and `store.js` 1,108.
- `views/grid/grid.js` 2,121: a canvas-painted grid for 100k rows, plus kanban, calendar, timeline,
  gallery and form views.
- It is marked **alpha**. Its README warns "data could be lost".

**Quality.** It has the best general-purpose engine in the group.
- Field references are stored by id, so renames never break them.
- Links are two-way.
- Undo/redo covers schema changes.

The currency field is a float, which is fine for a database UI but not for a ledger.

Storage: a schema file `bases/<id>/base.json` plus record shards of 2,000 per JSON file, with a per-tab
session journal replayed on open.

**Worth taking:**
- `formula.js` + `compute.js` + `query.js` + `fields.js`, about 2.7k LOC. This is a self-contained
  typed-table engine with a formula language, which is useful if Choir wants agent-editable structured
  tables.
- Overlap check: this may duplicate a spreadsheet formula engine another group reviews. Pick one formula
  language for Choir.

**App-agent design.**
- **This app *is* an object-graph browser.** Objects:
  - `choir.base`: versioned;
  - `choir.table_schema`: versioned, holding field definitions and formulas by field id;
  - `choir.view`: versioned, holding filter, sort and layout;
  - `choir.record`: external key, versioned, with metadata `table_id` and cell values by field id.
- Link fields become typed edges (`links_to`, field id in metadata).
- **The key move:** a Records table can be a *typed view over other apps' objects*. Examples: a
  "Clients" table whose rows are `choir.party` objects; an "Invoices" view over `fin_txn`; a
  "Sources" table over `choir.web_capture`. Lookups and rollups then run across apps with no export.
- Where it does not map cleanly:
  - 100k rows as individual JSON objects in the object graph is heavy for list/filter.
  - For large user-created tables, a generated Dolt table per Records table, with the object graph
    holding the schema and links, is likely better.
  - Computed fields should stay a projection and should not be stored.
- Versioning gives cell-level history (replacing the record "recent changes"), and **branches serve as
  "try a bulk edit"** for agent-driven mass updates.

**Desk verbs:**
- `create_table(schema)`, `add_field(formula)`, `upsert_records`, `query(view|filter)`;
- `link(record, object)`;
- `fill_column_from_research(field, prompt)`: research fills a column with cited values, and each cell
  carries an `evidences` edge.

**Desks it calls:** research for enrichment with citations, Texture for writing up a view as a
document, and engineering for importers.

**Publish:** owner-chosen bases or views can publish to Autopaper as datasets. Lineage and citation
edges are what make a published dataset trustworthy. Everything else is private.

**Priority: MEDIUM**, possibly high if no spreadsheet engine is chosen elsewhere. A typed table over the
object graph is a general agent substrate. Effort:
- engine port (or TS reuse in the frontend with a Go store): 2–3 weeks;
- a simple grid window: 1–2 weeks;
- skip the canvas grid at first.

## 8. assets (equipment/asset tracker: "AssetDesk")

**What and size.** **The source is missing.** `git ls-files src/assets` returns only `README.md`,
icons, `index.html`, `package.json`, `package-lock.json` and `vite.config.js`. There is no `src/` and
no `tests/`. `index.html` loads `/src/main.js`, which does not exist. The README describes about 30
features and an architecture (`lib/model.js`, `depreciation.js`, `zip.js`, …) that are **not in the
repository**, and the top-level README links it to `../equipment/`. As shipped, it cannot be built.

**Quality:** none to judge. The README's claims are unverified.

**Worth taking:** only the feature list as a product checklist. Its ideas: assignments,
reservations, sets/accessories, condition at check-out, straight-line depreciation, QR labels,
stocktake.

**App-agent design, if built.** `choir.asset` (versioned) and `choir.assignment` (holder party, dates,
condition, photo captures). Depreciation is a computed projection that posts monthly journal entries
into accounting. This is a thin feature on top of inventory's item and party objects, not a separate
app.

**Priority: LOW.** There is nothing to take, and Choir does not need it short term.

---

## Group summary

**Top picks**
1. **accounting's posting engine**: `accounting/src/lib/{money,txn,ledger,reports}.js`. Integer cents,
   pure document→postings rules for accrual and cash basis, snapshotting of rates and accounts, and
   exact proration. Port to Go as the finance core, store `fin_txn` versions in the Dolt object graph,
   and enforce the zero-sum and no-over-application invariants with a `WriteValidator` and
   `PutBatchConditional`. **A double-entry ledger on Dolt is the clearest fit for Choir's substrate in
   the whole group.**
2. **records' formula, compute and query engine**, as a typed-table layer that views other apps'
   objects. Choose it or another group's spreadsheet engine, not both.
3. **expense as an agent flow, not an app**: a receipt capture becomes an extraction (evidence), then a
   draft `fin_txn` on a branch, then owner confirmation.

**Reusable pieces across apps**
- Money utilities: `accounting/src/lib/money.js` (`allocate`, `applyRate`, `parseMoney`) and the
  `pos/src/lib/totals.js` cart algorithm.
- Importers and exporters:
  - `accounting/src/lib/bankimport.js`: OFX 1.x/2.x and CSV, with dedupe;
  - `invoicing/src/lib/pdf.js`: a zero-dependency PDF writer;
  - `invoicing/src/lib/xlsx.js`: a zero-dependency xlsx reader;
  - `pos/src/lib/escpos.js`: thermal printing;
  - `inventory/src/lib/csv.js`.
- Domain algorithms:
  - cap-table vesting, round fixed point and exit waterfall (as an oracle);
  - inventory kits, FEFO lots and demand-based reorder points;
  - invoicing installment schedules and multi-tax.
- A shared object model so these stop being silos:
  - `choir.party`, `choir.fin_item`, `choir.fin_account` and `choir.fin_txn` (versioned);
  - immutable `choir.stock_movement` and `choir.pos_sale`;
  - captures as evidence.

  Invoicing, expense, pos, inventory and assets become windows and desk roles over one ledger graph.

**Alarming or notable**
- **assets has no source code** despite a detailed README.
- **Float money:** invoicing, expense, cap-table, inventory and records. Accounting and the pos cart are
  integer-cent. Accounting hard-codes 2 minor digits and has a single currency.
- **Multi-device last-writer-wins** breaks cross-record invariants in every app:
  - over-applied payments in accounting;
  - whole-year file overwrites in invoicing;
  - lost stock in pos.

  Four apps built bespoke journal/saga layers to compensate. Do not port any of them; Dolt conditional
  batches and history replace them.
- **Puter lock-in:** an unpinned CDN script provides auth, storage and AI. The invoicing email worker
  is an app-owner mail relay. Cap-table share and PPS rounding differs from NVCA practice, and its ASC
  718/Rule 701 reports need expert review before anyone relies on them.
- **Licenses are clean:** MIT repo, ISC/MIT runtime dependencies.
