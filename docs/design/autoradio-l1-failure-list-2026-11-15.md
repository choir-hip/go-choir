# Station L1, the programme call: ways it can be wrong (2026-11-15)

Standing: built | kind: happened | settled-by: station L1 programme route 45bf7ce4c; its F-tests exist in internal/agentcore/api_autoradio_programme_test.go; docs/current-architecture.md:110 (programme route proven)

Written before the tests and the code, as the method requires. Station L1 of
[the goal file](../definitions/choir-autoradio-learning-engine-2026-11-14.md),
design [autoradio-learning-engine-2026-11-14.md](autoradio-learning-engine-2026-11-14.md)
sections 3, 9 and 12 (section 12 is normative) and
[the programme design](autoradio-dj-programme-2026-11-11.md) slice 2.
Class: red. Protected surfaces: gateway and provider calls (the role
`autoradio_programme`), tape and evidence (first writer of the forecast claims,
the stretch records and the index), auth and session (a device-key route).
Rollback: revert; what was written stays on the tape as history.

Each item is a test named `F<n>` in `internal/agentcore/api_autoradio_programme*_test.go`
or in `internal/modelpolicy`, `internal/appdesk`, `internal/apihandler`.

## Readings of the design taken here (where it could be read two ways)

- R1. Non-content segments (`ident`, `bridge`) are streamed
  with a segment record and no claim ("each streamed segment has exactly one
  claim" means each content segment; the design says they carry no forecast).
- R2. The index (`autoradio.stretch`) is written whenever the handler itself
  ends the call (done, a run of three invalid lines, a provider failure after a
  segment, a deadline), with the status in it; it is not written after a crash
  or after a failed claim write. Design 3.3 and 12.2 now say so (review round 1).
- R3. A replay of a `request_id` streams the leading run of stored segments
  whose claim exists, then `done` or an `error` event ("ended without an
  index" when there is none), and makes no model call; the retry is a new
  `request_id`.
- R4. Edition text arrives as the request field `edition_text` (programme
  design 2.2, slice 2); the handler numbers its paragraphs with
  `calibration.SplitBlocks`. A candidate's cite ref is `pub:<route>@v<version>`.
- R5. Lesson lines in the response are recognised and ignored (their processing
  is station L3); they never count toward the run of three. The active lessons
  the input lists are the lesson records on the tape (none can exist before L3).
- R6. A client that goes away does not stop the call (programme design 1.2 and
  slice 2's failure list: "client disconnect lets the stretch commit").

## The list

Route and request

- F1. No owner is 401, a GET is 405, a provider that cannot stream is 503;
  none of them reaches a model or writes a record.
- F2. A body over 256 KB is 413; an unknown field (`heard`, `after`, `model`),
  trailing data, a bad `request_id`, a `cause` outside `open`, `extend`, `turn`,
  `result`, a bad `local_time` or a `tz` over 64 bytes is 400 (a `tz` the guest cannot resolve is not refused: the local time is then shown by its own offset); none reaches a model
  or writes a record.
- F3. Candidates over the bounds of 12.7 answer 400 from the parser: 41 items;
  text on the eleventh item; item text over 8 KB; a duplicate ref; a missing
  route, version or title; a control character in a title. Exactly 40 items,
  text on exactly ten, text of exactly 8 KB are accepted.
- F4. `edition_text` over its bound answers 400.
- F5. The same `request_id` in flight is 409; the same `request_id` with
  another `cause` is 409.

The stream, the claims and the order

- F6. Only valid lines are streamed: each `segment` event carries the stretch
  id, the handler-minted segment id, the kind, title, text, normalised cites
  and the server's `block_count`, equal to `calibration.SplitBlocks`.
- F7. A content segment has exactly one claim carrying the tags of 12.1 (and
  `lesson:<id>` only for a lesson cause), the agent `autoradio-dj`, the kind as
  project and the probability `p_heard`; every streamed segment has one segment
  record holding its text; an `ident` or `bridge` has no claim.
- F8. The order is the guarantee: when each `segment` event reaches the wire,
  its segment record and its claim are already on the tape, the record before
  the claim (read by a writer that checks the tape at the moment of the event).
- F9. The first `segment` is flushed before the model call ends (a provider
  that waits for the test to see the first event).
- F10. The stretch start record is written once before any claim; the claim's
  deadline is the start record's stored `started_at` plus 12 hours; two claims
  of one stretch share it.
- F11. The request record maps the `request_id` to the stretch id and is
  written with the start record, before the stream begins.
- F12. At `done` the index lists every claim id as `related_ids`, with the
  status and the drop reasons; the stretch's claims listed by the tag query
  equal the index's list.
- F13. A replay of a completed `request_id` streams the same segments from the
  stored records, calls no model and writes nothing new.
- F14. Stretch ids and segment ids fit the grammar of 12.5 (segment id at most
  40 bytes), two calls never share a stretch id, and an id the model sent
  (`segment_id`, `stretch_id`, `lesson_id`) is discarded.
- F15. Dropped lines have no claim and no segment record, and take no segment
  index (the streamed ids are contiguous).

Validators (each one invalid line among valid ones is dropped, the others kept)

- F16. `p_heard` 1.7, -0.1, a string, null or absent is dropped; 0 and 1 are
  kept.
- F17. `caused_by` absent, an unknown cause (`vibes`), a bare `lesson` or a
  malformed ref is dropped; `importance` and `explore` are kept.
- F18. An unresolved instruction ref is dropped: a turn nobody spoke, a note
  nobody wrote; a listed turn and a listed note are kept.
- F19. An edition ref outside 1 to n (P0, P(n+1)), or any edition ref when no
  edition was sent, is dropped; P1 and Pn are kept.
- F20. A lesson ref naming no active lesson is dropped; one naming a lesson on
  the tape is kept.
- F21. A segment citing a lesson written in the same response is dropped
  (the lesson is not in the input), and the lesson line itself writes nothing
  and does not count toward the run of three (R5).
- F22. A cite outside the candidate set (a string, or an object with an unknown
  ref), a content segment with no cite, an unknown cite role, an empty ref is
  dropped; a candidate cite is kept and streamed with its label.
- F23. A forecast field (`p_heard`, `caused_by`) on `ident`, `bridge`,
  `answer` or `result` drops the line.
- F24. An unknown kind (`advert`, `air`, `settlement`), blank text, text over
  the kind's range, a missing title on a content kind, a title with a control
  character or a newline or over 200 bytes is dropped.
- F25. A line that is not JSON, or is JSON but not an object, is invalid; blank
  lines and bare code-fence lines are ignored; a line split across deltas is
  assembled; a last line with no newline is processed at the end; a line over
  64 KB is invalid.
- F26. A run of three consecutive invalid lines ends the call (an `error`
  event, nothing after it is processed or streamed, the index says so); three
  scattered ones do not; a valid line resets the run; blank and lesson lines
  neither count nor reset.
- F27. Every dropped line's reason is in the index; no dropped line caused a
  claim write.

Failures

- F28. A claim write that fails ends the call with an `error` event, the
  segment is not streamed, no later line is processed, no index is written (status `store_failed` on the `error` event).
- F29. A claim whose stored copy differs from the one written (`SameClaim`)
  ends the call the same way.
- F30. A provider that refuses before any segment is streamed walks to the next
  pool member and the listener sees no error; a failure after a segment is
  streamed ends the call with `error`, keeps the claims, and writes the index
  with status `failed`; every member refused is one `error` event.
- F31. A member that ends cleanly with no valid line is not an answer: the call
  walks on, and ends with an `error` event if every member did that.
- F32. A client that goes away after the first segment does not stop the call:
  the stretch commits with every segment and claim and its index (R6).
- F33. A crash mid-stretch (a provider panic after two segments) leaves the
  start, the request record, the two segments' records and claims and no index;
  the same `request_id` replays the two stored segments and an `error`
  ("the stretch ended without an index; use a new request_id") with no model call (R3); a new `request_id` makes a
  new stretch with new segment ids, never re-streams a claimed id, and leaves
  the old claims as they were.
- F34. The call creates no run, no inbox record, no directive addressed to a
  desk and no Texture document, and writes no air event and no settlement,
  whatever the provider emits (a line of kind `air` or `settlement` is an
  unknown kind); nothing resumes at boot.

Input and prompt

- F35. Every world or listener string in the input (titles, deks, candidate
  text, edition text, instruction words) sits inside a fence whose token is
  derived from the content, one line per field, fence look-alikes broken, and
  the section says it is data; a hostile title and a hostile instruction stay
  inside their fences.
- F36. The call has no tools, no token budget, reasoning `low` and not off, a
  cache key with the programme's own prefix, and the programme prompt as the
  system prompt.
- F37. The input shows no finish rate, count, total, streak or band table and
  no sentence that moves probabilities; the prompt file ships and says the same.
- F38. The input lists the listener's instructions (the turn requests on the
  tape) with their refs, the edition's numbered paragraphs, the candidates with
  their refs, the cause and the local time in the listener's zone.

Role

- F39. `autoradio_programme` normalizes as a bare role; with no policy section
  it resolves to the pool `autoradio` (River, then Bedrock Haiku 5.5) with
  reasoning `low`; a computer section overrides what it names; the `autoradio`
  role still runs reasoning off, no other role can hold off; the policy file
  that predates the role does not send the programme to the default pool.

Wiring and boundaries

- F40. The route is registered on the real mux and answers 401 unauthenticated,
  not 404; `gatewayruntime.Provider` implements `TextStreamProvider`, so the
  handler is not 503 in production; the handler's default ledger is the real
  store.
- F41. The programme handler's source has no call to the reserved settle
  method and writes no `autoradio.air` or `autoradio.settlement` record (an
  AST scan); the L1a one-caller test stands.
- F42. Every record the handler writes carries its 12.2 kind and its
  owner-private label, none is an addressed directive, none is returned by the
  generic claims routes or the addressee listing, and the stretch's claims are
  the only reserved claims of the call.
- F43. The L1a fixture `mobile/fixtures/learning/stretch_lines.json` is run
  through the handler: each segment line is kept or dropped as the fixture says,
  and the fixture's runs end the call where it says.

- F44. The stream the handler writes for the request and the model lines of
  `mobile/fixtures/learning/programme_stream.json` equals the events that
  fixture holds (the phone's contract for the `stretch`, `segment` and `done`
  events).

Review round 1 repairs (tests in `api_autoradio_programme_repair_test.go`)

- F45. A replay never streams a segment whose claim was not written (the claim
  write failed; a panic between the segment record and its claim).
- F46. A wall-clock deadline for the walk and an idle deadline between deltas
  end the call as `deadline` with an index, and the `request_id` then replays,
  not 409.
- F47. A segment cap of 30 ends the stretch with a named drop reason.
- F48. The replay reason for a stretch with no index says so, not "restart".
- F49. `mail`, `podcast_note`, `answer` and `result` are dropped by name.
- F50. Each instruction's and note's prompt text is bounded and marked.
- F51. `client_gone` is in the index.
- F52. An empty programme prompt fails before the first record.
- F53. An empty dek is 400; U+2028 and U+2029 do not reach the input.
- F54. `-0` is 0.

## Known and left

- The turn reply and the forecast claims still enter the desk pack and the
  inert scans
  ([problem record](../problems/autoradio-turn-reply-in-desk-pack-2026-11-15.md));
  every new record this station writes is a notice directive so it adds no
  reader of that kind. The fix is its own orange change.
- Stretches are not deduped against the air log (station L2), the input holds
  no marked cases, gestures, struck lessons or last titles (station L3), and
  lesson lines are not stored (station L3).
