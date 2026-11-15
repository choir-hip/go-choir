# Ledger: Autoradio learns its listener

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
