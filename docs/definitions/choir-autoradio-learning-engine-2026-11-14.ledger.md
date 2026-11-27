# Ledger: Autoradio learns its listener

Standing: built | kind: happened | scope: in part | settled-by: the ledger of choir-autoradio-learning-engine-2026-11-14.md; heard-memory row in docs/current-architecture.md line 121 (proven on d@e 2026-11-18, 4fc286f69, e2225bf67 on origin/main) | note: goal file now.status is working at slice L0 (line 277); the ledger is review rounds only

Append only. One entry per pass.

- 2026-11-15T00:28Z. PR 110 merged to main as 0157fd919 (docs only; the claim renumbered C87 to C90, since C87 to C89 were taken on main). Owner: seats are DeepSeek and Sonnet while Gemini is out of quota; build the phone app from main tonight; work overnight. L0 sent to both seats at 00:27Z.
- 2026-11-15T02:10Z. **L0 round 1: both seats REPAIR** (Sonnet seat, DeepSeek V4.1 Flash seat). The design and this file are revised; both go back to the same two seats. Section numbers are the design's. Owner rulings added: none. Nothing deferred without a station.

  Blocking findings (S = Sonnet seat, D = DeepSeek seat):

  | Finding | Disposition |
  |---|---|
  | S1, D1: per-segment settlement cannot ride one record; fallback false | Adopted. One claim per content segment, built by a server-side constructor, deterministic id from stretch and segment, tags carry cause and lesson ref; the stretch record is an index (3.3, 12). The first-hour deferral is deleted. Acceptance rewritten (L1, L2). |
  | D2: forecast after the evidence | Adopted. The claim is appended before its segment is streamed; a failed write ends the call; claims of a stretch that never completes stay and settle; a dropped line has none (3.2, 3.4). |
  | S2, D3: the phone can settle a forecast through the generic route | Adopted in part. The Autoradio namespace is reserved on create, settle, get, list and summary; only the heard handler settles, through a store method with a closed rule registry (4.3). Not adopted: refusing rule and owner methods from any API key. `cmd/choir/precommit.go:72` uses them for ordinary claims and the generic handler cannot tell a key from a session; the reservation closes the only case the DJ reads. Red change, station L1a. Acceptance reads provenance, not a scorer name. |
  | D4: programme request `heard` field is a second writer | Adopted. The field is removed; the heard route is the one writer; unknown fields answer 400; a call-site test (4.1, 9). |
  | S3, D5: settlement not a deterministic function | Adopted. Receipt order, first terminal event wins, one settlement id, later events stored and ignored, refusals for unknown segment and hash mismatch, 409 on a changed event_id, `resume` and `seq`, heard through defined (final block began) and named apart from covered (past half), lapse at 12 hours by the existing deadline, open and lapsed excluded and counted, late reports allowed, ack and retention (4.2). |
  | D6: rule 2 holes | Adopted. Notebook sections return the full air log, every forecast with outcome, turn records with words, lessons with cites and tallies, instructions; the phone's local file is compacted after ack; operational traces named; clear is a visibility act (6.3). |
  | D7: band record is a finish rate | Adopted. Marked cases only; no band table, count, rate, total or streak; golden-prompt test (5, 6.1, L3 acceptance). A listener view is read from the extended `calibration.Summary`. |
  | S5: tally not computable | Adopted. Cause and lesson ref are claim tags; the tally is read from them and says what the DJ said caused a segment (6.2). |
  | S4, D9: measurement gameable | Adopted. Causal base rate, per segment kind, coverage and strict Brier beside it, paired difference and standard error, tallies in the same reading, settles ARL-C1 only (5). |
  | S6, D10: no output validation | Adopted. Validator table, handler-minted ids, same-response cite refused, drop-line vs end-call, p in [0,1] refused outside (3.2). |
  | S7: injection | Adopted. The turn route's content-derived fence and one-line treatment for every world or listener string; instruction refs must resolve; corrected lessons stored as the listener's words (6.1). |
  | D8: classes and acceptance conflict | Adopted. L3 red with two seats; L4 orange because clear deletes nothing; phone acceptance aligned with D3; code proof split from observation; latency claim dropped until River; P2 depends on L2. |

  Should-fix and nits:

  | Item | Disposition |
  |---|---|
  | S: Rule 2 gaps (claims, run sheet and text, traces, strike memory, tag filtering) | Adopted (6.3, 4.3, 12). |
  | S: finish rate per band | Adopted (6.1). |
  | S: goal station order, contract step, L3/L4 classes | Adopted: fixtures in L1a, L3 red, P1 depends on L1a (goal stations). |
  | S: add a test that only the heard handler writes a heard event | Adopted (L2 acceptance). |
  | S: acceptance items that cannot fail (8, 7, 5, 4) | Adopted: ARL-C1, lesson truth and the subject test are observations, named so; code proof is separate. |
  | S: Ring-Expedite and red ceremony not named | Adopted (Order, L1a, L1). |
  | S: `explore` reads like a gate | Adopted: a cause label, no room-size setting (3.1, 7). |
  | S nit: stale refs 5af04baad, 21e7860bd | Adopted: design and goal cite 955d4527f. |
  | S nit: slice 2 seats | Adopted (design 9, goal now block). |
  | S nit: modelpolicy "future" comment | Adopted: L1 changes it and registers the role (design 9). |
  | D: candidate set field missing | Adopted: request field `candidates`, plan A; the guest reading the front list replaces it later (6.1, 13). |
  | D: `caused_by` grammar for instruction and edition | Adopted (3.1, 3.2). |
  | D, S: lesson record kind and storage | Adopted: `autoradio.lesson` records held by the Autoradio desk, `autoradio-lesson:L<n>`; D2's default stands (6.2, 12). |
  | D: lesson ids and `replaces` model-chosen | Adopted: handler mints ids; `replaces` must name an existing active lesson (3.2). |
  | D: same-response lesson cites | Adopted: refused, stated (3.2). |
  | D: tally lineage across replace, strike, clear | Adopted: replacement starts at zero and shows the chain, strike freezes, clear changes nothing (6.2). |
  | D: prompt growth | Adopted: 40 marked cases, 30 active lessons, compaction rule and what the DJ sees of older history (6.1). The numbers are bounds on the prompt, not on calls; whether they fit is open (13). |
  | D: L2 omits slice 4 resume, redirect and dedupe | Adopted in part: dedupe is in L2 (covered); the resume question and redirect are excluded from this mission and named in the goal file. |
  | D: P2 depends only on P1 | Adopted. |
  | D: L3 keyword test | Adopted: a golden prompt of the exact shape. |
  | D: `explore` vs "no room for explore" | Adopted (3.1, 7). |
  | D: D2 "run sheet kind" ambiguous | Adopted: the desk-held record kinds are named; no run sheet kind is used for the notebook. |
  | D: two `heard` notions | Adopted: heard through scores, covered dedupes (4.2). |
  | D: band record source is a second scoring path | Adopted: `calibration.Summary` is extended (5, 6.3). |
  | D nit: low-p "costs nothing" | Adopted (5). |
  | D nit: `left` cannot be sent when killed | Adopted: synthesised at the next launch (4.2). |
  | D nit: air log vs settlement as "the outcome" | Adopted: the settlement is the outcome, the air log its evidence (terms). |
  | D nit: no server in-flight cap | Adopted: nothing in the design adds one; one request in flight stays caller discipline. |
  | D nit: turn `heard` vs air `heard` | Adopted in the terms (the turn field is a playing position); no field is renamed in code. |
  | D nit: acceptance 4 vs section 12 | Adopted: lesson quality is an observation, read by the owner. |
  | Found while verifying | The generic list and summary join the newest 10000 records (`DefaultMaxRecords`); a listener's claims and settlements would evict other agents' calibration. L1a excludes reserved claims before the limit. |
  | Found while verifying | `calibration.BaseRate` is the in-sample mean (`math.go:236`); the measurement needs a causal base rate, added to `GroupStats` in L1a. C90's wording says "the listener's base rate"; the test is the one in the goal file. |

  Left open (design 13): whether the guest can read the front list; whether the real model writes lessons worth reading; whether 40 marked cases and 30 lessons fit the useful context; whether the generic handler can ever tell a key from a session (not needed by this design).
- 2026-11-15T03:20Z. **L0 round 2.** Sonnet seat (`claude.out`): ACCEPT, no blocking findings. DeepSeek V4.1 Flash seat (`omp-deepseek-v41-flash.out`): REPAIR, two blocking findings. Both hold round 1 closed and the rule 1 departure sound. The round 1 entry above names the seats by model; the files are `claude.out` and `omp-deepseek-v41-flash.out`. Revised on the same branch. Owner rulings added: none.

  | Finding | Disposition |
  |---|---|
  | D blocking 1: heard rule arrival-order dependent across batches | Adopted by the lead's decision. A settlement is evidence and never rewritten: the first terminal event the server receives settles; section retitled "deterministic given receipt order"; one batch normalised by `seq`; the race outcome stated; a later disagreeing terminal event is a recorded conflict shown in the notebook's air log and not shown to the DJ as an outcome (4.2). L2 acceptance states exactly those properties. The claim's `blocks` governs the heard-through test and the refusal; a differing `block_count` is refused; 0, negative and out-of-range values defined (4.2). |
  | D blocking 2: lesson cites not in the input | Adopted. Each marked case carries its settling `air:<id>`; gestures and corrections are listed with ids; open and lapsed cases cite nothing (3.2, 6.1). L3 acceptance adds a positive case and the open and lapsed negatives. |
  | S should-fix 1: lapsed segments dropped | Adopted. The abandoned class (a lapsed segment with a `cut`, `paused`, `resume` or `left`) is reported; a second, labelled reading counts it as not heard; the primary reading is unchanged; the DJ sees "stopped at block i of n" (4.2, 6.1, 5). |
  | S 2: weak comparator; ARL-C1 wording | Adopted. Pooled running rate shrunk per kind; ARL-C1 reworded to "separates better than the causal base rate", not testing that the notebook caused it (5, goal conjecture). |
  | S 3: receipt order across batches | Covered by D blocking 1. |
  | S 4, D store queries: 10000-record limit, no prefix or negation in `JSONFieldMatch` | Adopted. L1a names two store queries (agent-prefix exclusion, claims by tag); the notebook and measurement page by cursor and report truncation (4.3, L1a scope). |
  | S, D: L1a reservation conditions | Adopted. Test the stored cleaned agent and tags, not the request or id prefix; the heard handler requires exact `autoradio-dj`; acceptance adds `" autoradio-dj"`, `Autoradio-DJ` and a case-varied tag (4.3, L1a acceptance). |
  | S 6, D: deadline and commit time; partial stretch | Adopted. A start record is written once and the deadline derives from it; segment text is written before its claim; a retry is a new call and a new stretch id, never re-streaming a claimed segment; crash-and-retry is in the L1 acceptance (3.3). |
  | S 5, D: tag encoding over 64 bytes | Adopted. Only `cause:` and `lesson:L<n>` are tags; the full ref goes in `criterion`; ids at most 40 bytes; the check uses `cleanName` (3.3). |
  | D: "a run of three" | Adopted in 3.2, 9 and the L1 acceptance. |
  | D: settlement provenance | Chose the air event: the settlement's evidence names an air event the heard route wrote, plus the one-caller test; the acceptance matches (4.3). No route or actor is added to the settlement record. |
  | D: `covered` threshold | Same as the programme design ("past half"): more than half, `2 * (last_block + 1) > blocks` (4.2; 9). |
  | D: rule 2 completeness, owner-private labels | Adopted. The server holds each streamed segment's text in `autoradio.segment` and the notebook returns it; the response trace is named; every autoradio kind, claims included, carries the owner-private label, with a test that no other read path reaches them (3.3, 6.3, 12, L1a). |
  | D: instruction ref mapping | Adopted: `instruction:turn/<id>` names `autoradio-turn-req:<id>` (a failed turn counts); `instruction:note/<id>` names `autoradio-note:<id>` (3.2, 6.3). |
  | D: struck cites and late tallies | Adopted. The DJ is shown struck lessons' cites; a struck tally is as of the strike, later reports shown separately (6.1, 6.2). |
  | D, S 7: candidates | Adopted. Called listener-phone world data; at most 40 items, 8 KB each, 200 KB total, else 400; the cite validator trusting the phone is stated (6.1). |
  | S 8: 40 cases are a finish rate by hand | Adopted as a plain statement in 5: the guard is on what is shown, not on arithmetic. No tighter bound or balanced sampling. |
  | S 9: replay after the first terminal | Adopted: stored, `ignored`, nothing changes (4.2). |
  | S nit: codes are body fields | Adopted (4.2, fixtures). |
  | S nit: tag check calls `cleanName` | Adopted (3.3). |
  | D nit: stale source ref | Adopted: the Go tree is as at 955d4527f, main having moved only in `mobile/` (design header, goal start). |
  | D nit: seat citation | Adopted: design 9 cites the 2026-11-15 ruling. |
  | D nit: 4.4 settlement-rule change missing from section 9 | Adopted: row added. |
  | D nit: `better_means` and reliability | Adopted: corrections per hour are read as reliability evidence in the notebook, outside the ARL-C1 reading. |
  | D nit: "owner-provided" candidates | Adopted. |
  | D nit: ledger seat names | Noted above. |
- 2026-11-15T04:30Z. **L0 round 3, DeepSeek delta: REPAIR; round 3 structural assessment (rule 7).** Both round 2 blockers were closed. One new blocker came from the fix for them: the claim writer tagged `lesson:L<n>` while the tally read `ref:L`, so the notebook would have shown every tally as zero and no acceptance read a tally. **What recurred.** Each of rounds 1, 2 and 3 closed its findings and its edits opened a new inconsistency, because the design restated the same identifiers (tags, result codes, outcome classes, id grammars, the hash length) in prose in several sections and in the acceptance items, and each edit changed some copies and not others. **The change.** Patching is stopped. Design section 12 is now the one normative vocabulary, in tables: claim tags (12.1), record kinds with their label values, including the claim and settlement and the new conflict record (12.2), event fields, types, checks and result codes (12.3), outcome classes and the notebook enum (12.4), id and ref grammars (12.5), the hash comparison (12.6) and the bounds (12.7). Sections 3 to 6, 9 and the goal file refer to those tables by number and do not restate the literals; where prose and section 12 differ, section 12 wins. Fine detail that the tables do not fix is settled by each station's failure list, written before its tests, under that station's own review. Owner rulings added: none.

  | Round 3 finding | Disposition |
  |---|---|
  | Blocker: tally reads a tag the writer does not write | Adopted. The tally tags are named once (12.1) and 6.2 refers to them; L4 acceptance gains the tally clause (one heard and one not heard forecast caused by one lesson, tally read from the tape; a zero tally fails); `delta_o` names the tally tags. |
  | 1: last block zero both required and refused | Adopted (12.3): refuse a block count of zero and a negative or out-of-range last block; a first-block skip and a one-block finish are fixtures and in the L2 acceptance. |
  | 2: abandoned vs heard through | Adopted (12.4): abandoned means the final block never began; a lapsed segment whose final block began is the class stopped late, counted as heard in the second reading; the notebook enum is the table; L2 acceptance reads the classes. |
  | 3: lapsed case shown a forbidden id | Adopted: only a settled case carries an id (6.1, 3.2, L3 acceptance). |
  | 4: two codes for a later disagreeing terminal event | Adopted (12.3, 4.2): duplicate, conflict-recorded, ignored, one each; L2 acceptance. |
  | 5: padded name example wrong | Adopted: the id uses the cleaned name, so only the case variant differs; the padded name stays in the acceptance as a trim case (4.3, L1a). |
  | 6: claim record kind and label unnamed | Adopted (12.2): the forecast and settlement kinds with labels; the L1a test iterates the table. |
  | 7: 32 hex tag vs 64 hex event hash | Adopted (12.6, fixtures). |
  | 8: event_id grammar, conflict id | Adopted (12.5 the turn route's regex; 12.2 a conflict record keyed by the later event's id). |
  | 9: programme body cap | Adopted (12.7): 256 KB. |
  | 10: candidates shape | Adopted (12.7): 40 items by importance, text on the first ten. |
  | 11: false "only mobile moved"; wrong ParseGroupBy cite | Adopted: "mobile/ and three docs files; no Go file"; `handler.go:379`. Fixed in the design header and goal start. |
- 2026-11-15T05:40Z. **L0 round 4 and the change of gate.** The Sonnet seat returned REPAIR with two new blockers and a should-fix list; the DeepSeek seat timed out and returned nothing. The round 3 blocker (the tally tag) is closed and nothing from rounds 1 to 3 was found dropped. Four rounds each found real defects, and each in stations that are not yet being built, so the whole-design gate is the wrong unit (the lead's conservative decision, recorded). L0 is closed for the scope the first builds need: stations L1a and L1, and P1 on fixtures. Each of L2, L3, L4, P2 and P3 now has a first acceptance line: its seats re-review that station's sections of the design, with the round 4 findings as inputs, before its tests are written. There is no fifth whole-design review. Owner rulings added: none.

  | Round 4 finding (Sonnet seat) | Disposition |
  |---|---|
  | B1: block count rule, and the phone deleting refused events | Adopted. `SplitBlocks` is one server function with edge-text fixtures, and the segment event carries `block_count` (12.3, 3.4). The phone deletes only on accepted, duplicate, ignored or conflict-recorded and keeps and surfaces refused and changed (4.2, 12.3). P2 acceptance: a simulator plays a deployed stretch to the end and the tape holds a settlement for every played segment. |
  | B2: replay after a crash between the air append and the settlement | Adopted (4.2): on duplicate the handler settles if no settlement exists; the handler re-reads and compares after every conditional append, since the store answers success on a conflict (`commitment.go:47-50`). Fixture and L2 acceptance added. |
  | 12.3 status column contradicts "HTTP status is 200" | Adopted: the column is dropped; the table shows the phone's action. |
  | 12.2 read-by columns | Adopted: air, settlement and forecast rows list the notebook, input builder, dedupe and the point read. |
  | no code for a repeat non-terminal event | Adopted: `accepted` now means stored while the segment was unsettled (12.3). |
  | 4.4 vs 12.3 on segments | Adopted: a signal names a stretch and segment or none; a correction carries `turn_id` (4.4, 12.3). |
  | title can kill the stretch | Adopted: a title validator and a fixture (3.2, 12.7). |
  | heard-through gaming by one-block segments | Design choice, conservative option: the reading is stratified by block-count class (5, 12.7). |
  | lesson ids race | Adopted: compare-and-append (6.2). |
  | replay semantics | Adopted: a request record maps request_id to the stretch; claim writes compare with `SameClaim` after the append (3.3, 12.2, 12.5). |
  | L3 evidence is local only | Design choice, conservative option: L3 gains a deployed proof on d@e that the real model's output parses into lessons (goal L3). |
  | batch bound handling | Adopted: refused whole with 400; the phone chunks (4.2, 12.7). |
  | which lookup the heard handler uses | Adopted: a point read by claim id; the tag query is for the notebook and listings (4.2). |
  | nits | Adopted: stale "round" wording fixed in the goal file; candidates aggregate bound removed (80 KB follows from the item bound); strike and clear id grammar (12.2, 12.5); invalid lesson lines never count toward the run of three nor end the call (3.2); unknown type and `ended_at` before `started_at` are refusals (12.3). |

  C90's wording in `docs/calibration/claude-ledger.md` said "that listener's base rate"; it now names the causal base rate of design section 5, as one claim. The goal file already did.
