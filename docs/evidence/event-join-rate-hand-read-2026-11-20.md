# Event layer join rate: hand read (station E1), 2026-11-20

Status: **the hand read of 50 events was not done.** Production access to the
paper's events and memberships was refused on every sanctioned path (section 2).
What follows is what could be read without them: the flush counts from the
paper's console, and the code mechanism that produces the counts. Read at
2026-11-20 05:14Z (`date -u`).

## 1. Counts from the paper's console (item 5)

Source: the paper VM's console files (`console.log.4` .. `console.log`, the read
path `scripts/status-corpus-reuse` uses: one `ssh node-b`, `sudo -n grep`, read
only). They span 03:27:34Z to 05:13:08Z on 2026-11-20 only (the console keeps
about 2.5 hours). **They do not cover "since deploy" (2026-11-19 19:53Z).**
Counts below are the sums of the `op=event_flush` lines in that span.

| Span | Flushes | Questions asked | Joined | Minted | Tentative | Unresolved |
| --- | --- | --- | --- | --- | --- | --- |
| 03:28Z to 05:12Z | 96 | 899 | 25 (2.8%) | 756 (84.1%) | 94 (10.5%) | 24 (2.7%) |

The three flushes the handoff names (04:33:36, 04:34:56, 04:34:58Z) are in the
span: queued 25 + 27 + 1 = 53; joined 3 + 0 + 0 = 3; minted 19 + 26 + 1 = 46;
tentative 3 + 1 + 0 = 4. This matches the handoff.

Split by what the vector slot had to work with (`missing_unembedded` against
`queued` on the same line; every line says `vector_state=approximate`):

| Flush kind | Flushes | Questions | Joined | Minted | Tentative |
| --- | --- | --- | --- | --- | --- |
| every capture unembedded (no vector neighbours at all) | 69 | 632 | 15 | 538 | 67 |
| some captures unembedded | 16 | 185 | 7 | 164 | 14 |
| none unembedded | 11 | 82 | 3 | 54 | 13 |

704 of 899 questions (78%) were about a capture with no stored vector at flush
time (`missing_unembedded` summed). `vector_unmapped` summed to 1,074: the
nearest captures that were found but belonged to no event. Even the flushes
with a vector for every capture minted 54 of 82 (66%), so the missing vectors
do not explain the whole rate. 656 of 899 questions were research reads.
The minted share over this span is 84%; the design's bar is 60% or less
(`current-architecture.md` row "Event layer").

## 2. What I tried to read the 50 events, and what refused

- `choir` CLI: no `choir login` credentials file, no `$CHOIR_API_KEY`. The CLI
  has no events command; the route is `GET /api/autopaper/events`
  (`internal/agentcore/event_api.go`, `internal/apihandler/routes.go:52`).
- Node B corpusd PostgreSQL (`graph`, SELECT only, `sudo -u postgres psql`):
  `og_objects` holds no `choir.world_event` or `choir.event_membership` row.
  `internal/objectgraph/registry.go:131-132` registers both as Dolt, owner
  `OwnerVMStore`: they live in the paper VM's own store on `data.img`, not in
  corpusd. `proj_capture_index` does hold the paper's captures (computer
  `computer-1355b83bced1613eb048ee28d66688a8`, 579 rows in the last hour), but
  says nothing about which event a capture belongs to.
- Console logs: flush lines carry counts, and "corroborated" lines carry event
  ids and outlets; none carries a capture id, title or membership.
- `~/.config/choir-qa/test-computers/autopaper-paper.json` (the key the
  `scripts/event-blind-sample.py` label mechanism uses, for the same computer
  id): one read-only GET of `/api/autopaper/events?limit=300` returned
  **HTTP 401**. The key is expired or revoked. I did not mint a new one (an
  authority act) and did not read `data.img` of a running VM.

So the table of 50 events, the verdict counts and the similarity scores are
missing. Nothing in this file is a verdict on any event.

## 3. Mechanism that produces a low join rate (from code, not from verdicts)

These are places where the code gives a capture no chance to meet its event.
Whether they cost joins in the 50 is not measured.

1. **The vector slot needs a stored vector, and a new capture usually has none
   at flush time.** `eventNeighbours` (`internal/agentcore/event_layer.go:879`)
   asks corpusd for the nearest captures "by the captures' stored vectors, no
   embedder"; a capture not yet embedded returns nothing and is counted in
   `missing_unembedded`. A flush runs one minute after the first queued capture
   (`eventFlushInterval = 60s`, line 40; timer at 1157-1166). In the span above
   78% of questions had no vector neighbours. Such a question is offered only
   the 30 newest events (`eventShortlistRecent`, line 48; slot built in
   `eventShortlist`, line 828, within `eventRecentHorizon` of 72 h, line 70).
2. **A neighbour that is not in an event offers nothing.** In
   `eventShortlistFor` (lines 501-530) a neighbour whose capture has no
   membership (`idx.byCapture[nid]` empty) is skipped and counted as
   `vector_unmapped`. Corpusd returns 12 nearest captures (`eventNeighbourLimit`,
   line 55) and at most 3 distinct events are kept (`eventShortlistVector`,
   line 55); most held captures predate the event layer and belong to no event
   (1,074 unmapped neighbours in the span).
3. **Captures in one flush see each other only as sibling answers, and a
   sibling link needs p >= 0.92.** Events minted in a flush are not in that
   flush's call (the index is read before the call, `resolveEvents` 346-363).
   The only link between two captures of one call is the `k<n>` choice
   (`eventQuestion`, lines 1049-1066), which unions them only at
   `eventJoinP = 0.92` (line 78, case at 687). A split chunk is asked after the
   earlier chunk committed (comment at 396-399), but a normal flush is one call.
   Flushes of 25 to 30 captures from one poll mean many same-flush pairs.
4. **The judge's threshold.** A match to a known event counts at p >= 0.92 and
   is tentative from 0.5 (lines 78-79, cases at 681-690). A tentative match
   attaches the capture to the event (counted in `tentative`, not `joined`,
   never counted toward corroboration) and does not mint; the capture that has
   no answer above 0.5 mints (lines 749-768). The threshold was raised from
   0.85 after false merges (comment at 72-77).
5. **Research reads join the same queue.** `tool_profiles.go:392` queues pages a
   research desk read; 656 of 899 questions in the span were such pages.

Which of 1 to 5 accounts for the 46 minted at 04:33Z is **not determined**. A
hand read needs the memberships (who opened each event, with what p, and what
the shortlist held), which the journal does not record.

## 4. Are captures left unresolved during drains retried? (item 4)

Code: no.

- A failed call is retried inside the flush: `resolveEventChunkAt`
  (`event_layer.go:595-643`) backs off and asks again, then halves
  (`jevNextStep`); a dial failure ("connection refused") ends the chunk's calls
  at once. What still fails is counted `Unresolved` and, in `resolveEvents`
  (lines 418-423), put in `report.Left`.
- `drainEventQueue` (1193-1216) takes the queue once per loop; on an error it
  logs and `continue`s with an emptied queue, so nothing is re-queued. The
  comment at 407 says it: "an 'unresolved' wake record, not retried again".
- `report.Left` goes to `unresolvedRequests` (`event_wakes.go:207`, 265-287),
  which commits "unresolved" records to management's inbox.
- `stopEventLayer` (1232) drops queued captures; "nothing resolves them at the
  next boot".
- The held-capture sweep (`event_wakes.go:401-519`) releases stranded held
  captures to management as unresolved records, and its comment says "a
  released capture is never put to Jev again".
- A later source poll queues only the poll's *new* captures
  (`source_schedule.go:294`, `report.Captured`), so an unresolved capture is not
  queued by the next poll.

Production: the console shows two failures in the span, each "12 captures
unresolved: judgment client ... dial tcp ... :8084: connect: connection refused"
at 03:31:10Z and 04:47:29Z (24 unresolved in total, the 24 in the table). The
log line does not name the captures and no later line mentions them, so I could
not check whether any of them got a membership later. The handoff's 48 captures
(2026-11-19 20:25Z to 20:33Z) are before the console's span and were not read.
Not determined from production; by the code they stay without a membership.

## 5. What I could not determine

- Everything in the hand read: the 50 events, their minting captures, the
  candidates, the verdicts and similarity scores.
- Counts for the full time since deploy (the console starts 03:27Z).
- Whether the 24 unresolved captures were resolved later.
- Which of the mechanisms in section 3 caused any given mint.
- The stored similarity scores: the layer records Jev's p on a membership, not a
  vector score, and the shortlist shown is not recorded.
