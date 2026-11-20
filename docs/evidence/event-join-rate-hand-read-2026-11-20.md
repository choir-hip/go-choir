# Event layer join rate: hand read (station E1), 2026-11-20

Status: the hand read of the 50 most recent events is done (section 2a). The
first draft of this file (05:14Z) had no production read; the owner confirmed
the paper's read key, the lead renewed it, and the reads below ran at
2026-11-20 05:26Z to 05:29Z (`date -u`) with read-only GETs of
`/api/autopaper/events` and SELECTs on corpusd's `graph` database.

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

## 2. How the events were read, and one limit

`GET /api/autopaper/events?limit=300` (one call) with the paper's key,
`total` 10,153 events. The route sorts by **last_seen** and cuts at 300, so it
reaches back only to events touched since about 04:40Z; the 300 returned span
first_seen 04:42Z to 05:26Z plus a few older events that were joined late. The
50 events with the newest first_seen are complete in that list. **The 04:33Z
flushes the handoff names (53 captures) are outside it** (their events were not
touched after 04:42Z), so they were not read; 2a is a different, later burst.
Vector scores were read from `vec_captures` in corpusd's `graph` database
(`1 - (a.embedding <=> b.embedding)` between two captures' vectors, SELECT
only): they are the scores **now**, not what the flush saw, and the shortlist
the judge was shown is not recorded anywhere.

### 2a. The 50 most recent minted events (first_seen 05:16:14Z to 05:26:27Z)

Judged by hand against the events that existed (title and time), strictly by
the layer's own definition (one incident; a reaction, a roundup or an analysis
piece is another event). `p` was not used for the verdict. 41 of the 50 were
opened by a research read (`web:read`), the rest by feeds.

| # | first_seen | opener source | headline | members | verdict | note |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 05:26:27 | web:read | Contaminated petrol leaves Yemeni drivers struggling to make ends meet | 1 | should-have-joined | identical headline to the 05:06:41 event, 20 min earlier; opener has no vector row |
| 2 | 05:25:35 | rss:ft_markets | Trump to host memecoin dinner as Democrats take aim at ‘conflicts of i | 1 | correctly minted |  |
| 3 | 05:25:35 | rss:ft_markets | Anthropic’s Wall Street stimulus fades | 1 | correctly minted | analysis piece |
| 4 | 05:25:35 | rss:ft_markets | So, Anthropic is finally a public company. What now? | 1 | correctly minted | analysis piece (Anthropic IPO itself is the 04:42 Al Jazeera event) |
| 5 | 05:25:35 | rss:ft_markets | Oil loses grip on bond markets as inflation ‘virus’ spreads | 1 | correctly minted |  |
| 6 | 05:24:10 | web:read | Landmark documentary 'BONDI: 7 Minutes That Changed Australia' chronic | 1 | correctly minted | a different documentary |
| 7 | 05:24:08 | web:read | ‘Terror on Bondi Beach’ Director Pays Tribute to 10-Year-Old Victim Ma | 1 | unclear | interview about the Channel 4 documentary |
| 8 | 05:24:08 | web:read | ‘I ran barefoot to help Bondi Beach victims - I saw the best and worst | 1 | unclear | witness story, may be the documentary's |
| 9 | 05:24:06 | web:read | Manifestations lycéennes : le gouvernement pointé du doigt pour sa ges | 1 | unclear | French lycee movement, one of five captures minted separately (#9,10,11,17,23) |
| 10 | 05:24:04 | web:read | Après plusieurs jours de blocage, la reprise des cours perturbée par d | 1 | unclear | Toulouse blockade, same flush as #11 |
| 11 | 05:24:03 | web:read | "Les lycées doivent être débloqués", scande le maire de Toulouse Jean- | 1 | unclear | Toulouse blockade, same flush as #10 |
| 12 | 05:23:38 | web:read | Cuatro jóvenes subieron a un auto de DiDi en el Parque de Mayo y asalt | 1 | correctly minted |  |
| 13 | 05:23:25 | web:read | Documentary on Bondi Beach Terror Attack to Air on Channel 4 / Split A | 1 | should-have-joined | same Channel 4 announcement as #14, same second; cosine 0.908 |
| 14 | 05:23:25 | web:read | "Terror on Bondi Beach" Documentary Set for Channel 4 and Streaming /  | 1 | correctly minted | first of the pair |
| 15 | 05:23:21 | web:read | Nayib Bukele formaliza su candidatura para un tercer mandato en medio  | 1 | should-have-joined | Bukele candidacy, 05:22:11 events existed; cosine 0.885 |
| 16 | 05:23:02 | web:read | Terror on Bondi Beach review – these astounding stories recreate every | 1 | correctly minted | a review |
| 17 | 05:23:02 | web:read | Title: Mouvement lycéen : l’impossible débat sur le « maintien de l’or | 1 | unclear | lycee movement analysis |
| 18 | 05:22:51 | web:read | El Salvador approves indefinite presidential re-election / Elections N | 1 | correctly minted | the reform itself |
| 19 | 05:22:51 | web:read | From Term Limits to No Limits: El Salvador’s Constitutional Reform on  | 1 | should-have-joined | analysis of #18's reform, same flush; cosine 0.868 |
| 20 | 05:22:44 | rss:hn_best | Russia reportedly planned mass-casualty attack in Poland to be blamed  | 1 | correctly minted |  |
| 21 | 05:22:44 | rss:hn_best | Gemini 4 Neon is the new #1 on the Models Engineering Index | 1 | correctly minted |  |
| 22 | 05:22:13 | web:read | 2026 French civil unrest - Wikipedia | 1 | unclear | a Wikipedia page |
| 23 | 05:22:11 | web:read | Revendications locales, vidéos virales, soutien de LFI… Comment le blo | 1 | unclear | lycee movement |
| 24 | 05:22:11 | web:read | Nayib Bukele registers candidacy for a third presidential term in El S | 1 | should-have-joined | Bukele candidacy, same flush as #26; cosine 0.871 |
| 25 | 05:22:11 | web:read | El Salvador's Bukele set to formalize third-term bid - JusticeInfo.net | 1 | should-have-joined | same; cosine 0.889 |
| 26 | 05:22:10 | web:read | Bukele to register bid for a third term on Thursday | 1 | correctly minted | first of the Bukele three |
| 27 | 05:22:10 | web:read | Client Challenge | 1 | unclear | bot-block page 'Client Challenge', identical title to an earlier capture, not an event |
| 28 | 05:21:59 | web:read | Instituto Nacional de Prevención Sísmica / Argentina.gob.ar | 1 | unclear | institute home page, not an event |
| 29 | 05:21:01 | web:read | India and Japan to expand maritime security cooperation and joint defe | 1 | should-have-joined | India-Japan 2+2, ten events 04:50-04:53; cosine to 04:53 event 0.912 |
| 30 | 05:20:59 | web:read | India and Japan Strengthen Strategic Ties at 4th 2+2 Ministerial Meet  | 1 | should-have-joined | same Asianet story as the 04:53:01 event; cosine 0.951 |
| 31 | 05:20:59 | web:read | India-Japan 2+2 dialogue: Both sides push defence, technology and mari | 1 | should-have-joined | 2+2 outcomes; cosine to ABC event 0.887 |
| 32 | 05:20:59 | web:read | India-Japan 2+2: Japan to use an Indian firing range / Wirony | 1 | should-have-joined | firing-range agreement, 04:50:10 Japan Times event; cosine 0.881 |
| 33 | 05:20:04 | web:read | M4.8 Earthquake Calingasta, Argentina — November 20, 2026 / 3 Felt Rep | 3 | correctly minted | the quake |
| 34 | 05:19:53 | web:read | Latest Earthquakes | 1 | unclear | quake list page |
| 35 | 05:19:39 | web:read | The Earthquake Event Page application supports most recent browsers, v | 1 | unclear | quake page |
| 36 | 05:18:49 | social:bsky:japantimes.c | (no title) | 1 | unclear | no title |
| 37 | 05:18:32 | social:bsky:spiegel.de | (no title) | 1 | unclear | no title |
| 38 | 05:18:17 | social:bsky:meduza.io | (no title) | 1 | unclear | no title |
| 39 | 05:17:25 | rss:guardian_world | Kellie Sloane says she’s ‘pretty sure’ NSW Liberals exodus is over aft | 1 | correctly minted |  |
| 40 | 05:17:25 | rss:guardian_world | Iraq university’s removal of male professors teaching women sparks wee | 1 | correctly minted |  |
| 41 | 05:17:25 | rss:guardian_world | Race to be next UN secretary general exposes divisions among world pow | 1 | correctly minted |  |
| 42 | 05:17:25 | rss:guardian_world | Patient ‘horrified’ to find her sensitive discussions were shared with | 1 | correctly minted |  |
| 43 | 05:17:25 | rss:guardian_world | Beau Lamarre-Condon’s explanations of body disposal ‘ring hollow’, cou | 1 | correctly minted |  |
| 44 | 05:16:56 | official:kremlin:en_news | On November 18–19, Vladimir Putin will travel to China on a working vi | 1 | correctly minted | a different trip from #47 |
| 45 | 05:16:56 | official:kremlin:en_news | Congratulations to Mirra Andreeva on winning the 2026 WTA Finals singl | 1 | correctly minted |  |
| 46 | 05:16:56 | official:kremlin:en_news | Meeting with Minister of Transport Dmitry Yadrov | 1 | correctly minted |  |
| 47 | 05:16:56 | official:kremlin:en_news | On November 19–20, Vladimir Putin will travel to Tajikistan | 1 | correctly minted |  |
| 48 | 05:16:56 | official:kremlin:en_news | Meeting with member of the Politburo, Secretary of the Central Committ | 1 | correctly minted |  |
| 49 | 05:16:22 | web:read | F-22s, F-15s Rotate in at Kadena to Bolster Pacific Airpower | 2 | correctly minted |  |
| 50 | 05:16:14 | web:read | The War Zone: The "Doomsday Plane" accompanied Trump in Asia - Pravda  | 3 | correctly minted |  |

**Counts: should have joined 10, correctly minted 25, unclear 15.** Strictly
counted, 10 of 50 (20%) are duplicates and 25 to 40 of 50 are right depending
on how the 15 unclear are read. The unclear are mostly pages that are not one
happening (a home page, a bot-block page, titleless social posts, an
earthquake list page: 8), and a French student movement minted as five
separate events (#9, #10, #11, #17, #23) plus three Bondi documentary pieces
(#7, #8, #22 is a Wikipedia page).

The 10 misses: #1 (Yemen petrol), #13 (Channel 4 documentary), #15, #24, #25
(Bukele candidacy), #19 (El Salvador reform), #29, #30, #31, #32 (the India
and Japan 2+2 meeting). Cosine between the missed capture and the event's
opener: 0.868 to 0.951 (#13 0.908, #15 0.885, #19 0.868, #24 0.871, #25 0.889,
#29 0.912, #30 0.951, #31 0.887, #32 0.881); #1 has no score because the
capture being minted has no row in `vec_captures` at all.

### Pattern of the misses, against the mechanisms in section 3

- **Same-flush siblings (mechanism 3): 6 of the 10.** #13, #19, #24, #25 were
  minted in the same second as the event they duplicate (same flush, in one
  call; neither could be in the other's shortlist; the only link is a sibling
  answer at p >= 0.92). #31 and #32 were minted at 05:20:59 together with #29
  and #30.
- **A shortlist or judgment miss with the event already in the index (not
  same-flush): 5 of the 10, overlapping.** #1 matches an event minted 20
  minutes earlier (05:06:41) with an identical headline. #15 matches events of
  05:22:11, one minute before. #29 to #32 match ten India-Japan events minted
  at 04:50 to 04:53, 28 to 30 minutes earlier; every one of those is inside the
  72 h recent slot but the slot is the 30 newest, and at this burst's rate (95
  new events per ten minutes by first_seen 04:5x and 05:0x) 30 events is
  about three minutes of events, so a 30-minute-old event is out of it
  (`eventShortlistRecent = 30`, `event_layer.go:48`). Only the vector slot
  could reach it, and the 14-day vector slot offers at most 3 events from 12
  neighbours (lines 55, 59). Whether the vector slot had the event, and
  whether the judge refused it, is **not determined**.
- **No vector at flush (mechanism 1):** #1's capture has no vector row even
  now. For the others vectors exist now; whether they existed at flush time is
  not determined (section 1 shows 78% of questions in the previous 100 minutes
  had none).
- **Threshold (mechanism 4):** the opener's `p` on these events is not the p of
  a rejected join (a mint's `p` is the judge's p of the choice it made), so a
  refused-at-0.92 candidate cannot be read from the stored data. #1 is stored
  with p = 1: the judge was confident it was a new event, which means the
  05:06 event was not in its choices (or was refused outright).
- **Unmapped neighbours (mechanism 2):** not testable from stored data.

Net: the clearest, countable cause is same-flush siblings (6 of 10) and the
30-event recency window against a burst of research reads (4 to 5 of 10). The
vector neighbours did not rescue the cases where they could have (the cosines
are 0.87 to 0.95 now), but whether that is the shortlist or the judge is not
read.

## 3. Mechanism that produces a low join rate (from code, not from verdicts)

These are places where the code gives a capture no chance to meet its event.
Whether they cost joins in the 50 is not measured.

1. **The vector slot needs a stored vector, and a new capture usually has none
   at flush time.** `eventNeighbours` (`internal/agentcore/event_layer.go:879`)
   asks corpusd for the nearest captures "by the captures' stored vectors, no
   embedder"; a capture not yet embedded returns nothing and is counted in
   `missing_unembedded`. A flush runs one minute after the first queued capture
   (`eventFlushInterval = 60s`, line 40; timer at 1159). In the span above
   78% of questions had no vector neighbours. Such a question is offered only
   the 30 newest events (`eventShortlistRecent`, line 48; slot built in
   `eventShortlist`, line 828, within `eventRecentHorizon` of 72 h, line 70).
2. **A neighbour that is not in an event offers nothing.** In
   `eventShortlistFor` (lines 501-530) a neighbour whose capture has no
   membership (`idx.byCapture[nid]` empty) is skipped and counted as
   `vector_unmapped`. Corpusd returns 12 nearest captures (`eventNeighbourLimit`,
   line 59) and at most 3 distinct events are kept (`eventShortlistVector`,
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

Production: the console shows two failures, each "12 captures unresolved:
judgment client ... connect: connection refused", at 03:31:10Z and 04:47:29Z.
The 12 captures of each are identifiable in corpusd's `proj_capture_index` as
the 12 mastodon captures with one updated_at second (03:30:46Z, source
`social:mastodon:newsie...`; 04:46:25Z, source `social:mastodon:mastod...`,
one minute before each failure).
- **04:46:25Z batch: never resolved.** At 05:28Z none of the 12 is a member of
  any event in the 300 events the route returns (which reach back to about
  04:42Z, so a later join to any event would be in it). 42 minutes unresolved.
- **03:30:46Z batch: not determined.** The route's 300 events no longer reach
  back to it. None of the 36 mastodon captures of 04:30 to 04:36 is a member
  either, but those are outside the list's window too, so that proves nothing.
- The handoff's 48 captures from 2026-11-19 20:25Z: not read.
By the code, and by the 04:46:25Z batch, a capture left by a failed flush
stays without a membership.

## 5. What I could not determine

- The 04:33Z flushes' 46 minted events (outside the route's window); the 2a
  table is the later burst and is mostly research reads.
- What the judge was shown, and any refused candidate: not recorded.
- Vector scores at flush time (scores above are from now).
- Counts for the full time since deploy (the console starts 03:27Z).
- Whether the 03:30:46Z batch was resolved later.
- Whether the 15 unclear events are right; they were not read beyond the title
  and the capture's source.
