# L2a Merge Recall on the Paper: Why After-Window 1 Read 0.174

Date: 2026-10-14. Station:
[L2a](../definitions/choir-ledger-l2a-synthesis-intake-2026-10-14.md).
Window: 06:51-12:51 UTC on the paper (computer-1355b83b…), scored in
[l2a-after-window-1-2026-10-14.md](../../evidence/l2a-after-window-1-2026-10-14.md)
(merge recall 0.174 = 8 of 46, 55 of 425 captures attributed, 31
false-merge candidates). Status: documented; fixes follow in separate
commits. Mutation class of this note: green.

Method: read-only product routes on the paper (management's inbox, every
story and source inbox, management's 75 runs from 06:00 to 13:40 with
their cell code and output, `/api/autopaper/sources/schedules`,
`/api/runtime/observability`, `/api/captures`, documents and revisions),
joined per capture. Each of the 46 judge pairs was mapped back to its two
capture ids. Raw files are outside the repo (session scratchpad,
`l2a/`). Nothing on the paper was changed.

## The measure that matters: articles per source

Owner, 2026-10-14 about 14:15 UTC: "we should be able to scale up the
sources while only scaling up article production significantly
sublinearly. More sources means only marginally more articles.
Fundamentally, the ratio of sources to articles is a crude proxy of
quality." The paper went from 14 to 20 schedules at 14:15 UTC, with 8
more after the next deploy. So the target is: captures per new story and
sources per story rise, and new stories per hour grow far slower than
captures per hour.

Baseline from this window (12 schedules delivered captures; intake
counts only, not research's citations):

| measure | 06:51-12:51 UTC |
|---|---|
| captures per hour | 70.8 (425) |
| new stories opened per hour | 30.2 (181) |
| captures per new story | 2.35 |
| distinct pages per new story | 1.65 (298 pages; 146 captures re-capture a page) |
| stories that received a window capture | 175 |
| intake outlets per story | 1.09 (mean) |
| stories with 2+ intake outlets | 7.4% (5.7% of the stories opened in the window) |

Read plainly: almost every distinct page became its own story. One new
story per 1.65 pages is near-linear production. (Published versions
cite more outlets than this, because research widens each story from
the web; baseline A measured a median of about 6. That is research per
story, not intake merging.)

## Finding 1: the scorer read only the newest 50 documents

`scripts/l2a_window_pairs.py` finds each capture's story by reading the
inboxes of `GET /api/texture/documents`. That route is capped at 50
(`HandleTextureListDocuments`, `listTextureDocuments(…, 50)`), ordered by
update time. Scored at about 13:30, it saw the 50 stories updated most
recently, so most stories opened or fed earlier in the window were
invisible. The window's dispositions name 175 story documents.

Re-read with every story inbox (the 50, plus every `texture:<doc>` in
management's receipts, plus every story id named in management's cells
and their output):

| measure | scored | re-read |
|---|---|---|
| cross-outlet same-event pairs ending in one story | 8 of 46 (0.174) | **29 of 46 (0.630)**; 25 (0.543) counting only each capture's first story |
| captures in a story (opened or fed) | 55 (13%) | **256 (60%)** |

So 21 of the 38 "missed" pairs and most of the 13% were the scorer, not
the paper.

## Finding 2: Jev labelled nothing on the paper

`intake.labels` is on (`2026-10-14T06:50:57Z`), yet every poll since then
records `labelled: 0`. From `/api/autopaper/sources/schedules`, per hour
of `new` captures, all unlabelled: 07h 108, 08h 89, 09h 68, 10h 49, 11h
49, 12h 50, 13h 24. After the 13:43 refresh onto 6f8e36ce the 13:58
polls are still `labelled 0`.

The cause is in the guest log (`/api/runtime/observability`,
`recent_log`), one line per capture:

    2026/10/14 13:58:26 intake labels: rss:hn_frontpage: obj:choir.web_capture:…:key-5a5c3926… unlabelled: gateway client: Jev bearer is not authorized for this VM

That is the gateway's `HandleJudgment` refusing with 403 after
`IdentityRegistry.BindJevPeer` fails (`internal/gateway/jev.go:273`,
`internal/gateway/config.go:404-423`). The bearer itself is valid
(`ValidateCredential` passed, or the answer would be 401), so the refusal
is `Jev bearer is bound to another VM`: the credential's `JevPeerIP` is
an address other than the paper's current guest address.

Why the address moved: a computer's gateway token persists across boots
(`<state>/<vm>/persist/gateway-token`, re-imported by vmctl's
`ensureExistingGatewayCredential`), but its guest address does not. The
address comes from the host port allocated at each boot
(`vmmanager.guestAndHostIP`, `nextPort++`). `BindJevPeer` pins the first
address that ever called Jev with that bearer, for the credential's
whole life, and only `IssueCredential` clears it. So after any cold boot
on a new port, Jev refuses that computer until its credential is
re-issued. The paper had several boots today (deploy refreshes, the
10:25 crash-refresh, the 13:43 refresh). The fixture computer was freshly
minted, so its first address was its only one, which fits its labels
working (not checked: that computer is stopped).

Directly confirmed: the refusal string, and 0 labelled on every poll in
the window. Inferred from code, not from host data: that the pinned
address is the paper's address from an earlier boot (the host-side Jev
receipts or the gateway identity file would show it). The same pin
guards every Jev call the computer makes, so the support judge
(citation verdicts on published versions) should be refused the same
way on this computer; not checked.

Management therefore saw `labels={…:{"says":"unlabelled"}}` for every
capture it printed (97 window captures, all unlabelled), and routed by
filtering `choir.Stories(200,"")` headlines with keywords. The window
measured management's routing with the larger library view and without
labels. The L2a conjecture (labels make one story per event) was not
tested on the paper: no label, no label override, no Jev-settled feed
(owner ruling O3) could exist.

Timing fits: from a capture's `fetched_at` to its arrival record the
median is 0.56 s (28 captures readable through `/api/captures`), so the
refusals are fast. A replay of the same request shape (30 stories, the
poll's siblings, both questions) sent directly to OpenRouter's Decisions
endpoint answered in 0.23-0.31 s each (5 calls, about $0.00023 each), so
Jev and the request shape are fine.

## Finding 3: the 17 pairs that really missed

| cause | pairs | decisive trace |
|---|---|---|
| A. one capture never disposed: the 08:22 burst read by failed management runs | 10 (#8, #13, #15-#22) | records 508-514 (08:22:12-37, 28 captures); runs created 08:22:12 and 08:23:43 ended `loop.failed`: `gateway client: provider opencode-go: circuit open (upstream unhealthy)`; documented in [captures-undisposed-after-failed-management-runs](captures-undisposed-after-failed-management-runs-2026-10-14.md), fixed by e3437f1c |
| B. a re-capture of a page already in the story, declined as a duplicate | 3 (#2, #4, #14) | 07:35:16 cell: `Duplicate captures: each page is already cited in an existing story — the BBC 'Is Zelensky right...' fact-check in the Yandex-strikes story, … the SF Standard piece in the OpenAI-engineer story …`; meta.ai "Muse Wonder" re-captured at 08:22:34 and 10:55:26 after opening 5d39fbcd at 07:38. Merged at page level; the pair counts two capture ids of one page |
| C. a second story opened for an event that already had one | 2 (#9, #10) | 08:54:21 cell lists `9111b4c6 \| Russian strikes hit key power plant in Kyiv, Zelensky says`; 08:56:35 cell opens `texture:new "Ukraine reports widespread power loss after Russian attacks"` (c376261b); the capture's label line read `says=unlabelled` (08:53:25) |
| D. fed into a story on the same topic, not the same event | 2 (#23, #24) | 08:38:47 feed of the Guardian Europe live blog into bfffcc44 ("Zelenskyy tells NPR…"): `adds current developments in the existing Ukraine-war story` |

Not found: no pair missed because the first story did not exist yet when
the second capture arrived, and none because a capture was declined as
"no news".

Citations (approximate: capture key or URL found in the story's current
revision): of the 29 merged pairs, both captures are cited in 5, one in
17, neither in 7. Research's own re-captures of the same pages are not
matched, so this undercounts.

## Finding 4: what happened to the 425 captures

| fate | captures | of which re-captures of a URL already captured |
|---|---|---|
| in a story (opened 158, fed 113; 15 both) | 256 | 85 |
| declined only | 122 | 43 |
| no disposition | 47 | 18 |

The 47 without a disposition:
- 28: the 08:22 burst (cause A).
- 7: at 09:27:23 management opened 7 stories (receipts
  `rlm:cast:1 to texture:3165c0ad…`, `…08a0491f…`) whose Casts carry
  `"captures":[""]`: the cell that set the capture variables had failed
  to compile at 09:27:16, and the next cell used them empty. The runtime
  accepted a Cast whose only capture id is empty. 11 opening Casts in
  the window carry no capture.
- 10: hn_best records at 10:40:16 and 11:55:36 (HN best-of re-lists),
  each arriving seconds after a turn started; that turn reported every
  record disposed. Mechanism not confirmed.
- 2: 08:38:15-16 (openai_news, theregister), named in a 08:40 cell; no
  disposition record found.

Re-captures: 146 of the 425 window captures (34%) re-capture a URL
already captured, 120 with an unchanged title (hn_frontpage 70, hn_best
31, guardian_world 35). A capture's identity is URL + doc_hash
(`sourcegraph.WriteCaptures`), and HN feed items carry points and comment
counts in their text, so every poll that sees changed counts writes a new
capture. Live blogs re-capture by design. They inflate arrivals and
declines and create judge pairs between two captures of one page.

## Finding 5: the 31 false-merge candidates

| story | pairs | reading |
|---|---|---|
| 1bdba291 "UK hosts Zelenskyy…" | 9 | legitimate: Guardian article, three captures of the Guardian UK-politics live blog and the BBC piece, all on Zelenskyy's London visit and Burnham's announcements around it |
| 1bdba291 | 5 | **wrong (topic feed)**: the Guardian Europe live blog on Russian strikes and the 600,000 outage, fed at 08:55:24 with `Feed to existing Ukraine coverage rather than duplicate event`; 73 s later the same capture opened c376261b |
| 4fbe5d23 Ledger | 1 | legitimate: one disclosure (BleepingComputer, Zetter) |
| c20a106b Starlink | 5 | legitimate: the BBC roundup headline covers the strikes and the Starlink restriction; it was fed to both stories |
| c9e0ecae fake blood | 1 | legitimate: MIT TR's newsletter item on its own article |
| b5db8e8d GPT-6 Astra launch | 5 | legitimate: the launch post (two captures), the safety overview and "Path to Astra" are one launch |
| b5db8e8d | 4 | **wrong (case study folded into launch)**: the Playco customer case study, fed 11:56:34 |
| 00762957 Astra case study | 1 | **wrong**: two different customer case studies (Parallel, Legora) in one story |

21 legitimate, 10 wrong. The wrong ones are management feeding by topic
(the "Ukraine coverage", the "Astra" product), which is failure mode C/D
from the other side. This list came from the capped scorer; the re-read
scorer will produce a longer list that needs reading again.

## Clustering assessment

Four causes, not one:

1. **Measurement (the scorer's 50-document cap).** 21 of 38 misses and
   most of the attribution gap. Fix the scorer; no product change.
2. **Jev's peer pin versus per-boot guest addresses (gateway × VM
   lifecycle).** A substrate defect: every computer that cold-boots onto
   a new port with its persisted credential loses Jev. On the paper it
   disabled all of L2a's runtime mechanism, and likely the support judge.
   It does not explain a measured miss directly; it explains why the
   residual routing errors (C, D and the 10 wrong merges, all "same
   topic vs same event" errors made from keyword-filtered headlines) had
   no label to correct them. That labels would have prevented them is
   the L2a conjecture, still untested on the paper.
3. **The failed-turn incident.** 10 of the 17 real misses; already
   fixed (e3437f1c).
4. **Re-captures of unchanged pages** (34% of window captures). Inflates
   the denominator and management's work; 3 pairs. A design consequence
   of capture identity, separate from routing.

Smaller: a Cast with an empty capture id is accepted (7 captures).

The substrate-level action is cause 2; patching management's prompt for
C/D before labels reach management would be tuning without the
mechanism under test.

## Fix direction

1. Scorer (yellow): enumerate every story inbox (receipts, story ids
   named in management's cells, the document list); count a pair merged
   when its captures share a story; report label coverage from the
   schedules route and re-captures.
2. Jev peer pin (red; gateway credential and vmctl): the host re-pins the
   bearer to the computer's current guest address when it reconciles the
   credential after a boot. vmctl already calls
   `/provider/v1/credentials/ensure` for every ready VM and knows the
   guest address (`ComputerURL`); it sends it, and the gateway sets
   `JevPeerIP` to it. A bearer copied to another VM stays refused,
   because only host lifecycle control can move the pin. The paper
   recovers on its next ensure after deploy, with no credential rotation.
3. Label failures visible on the poll record, not only in a log ring
   (no-SSH operability).
4. Later, separately: capture identity for feeds whose text churns (HN),
   and refusing a Cast whose capture id is empty.

Then a second 6-hour window on the paper with the fixed scorer, read
against this one, in this order:
1. label coverage (labelled / new on the schedules route) near 100%, and
   no "Jev bearer is not authorized" lines;
2. the production ratios above: captures per new story and intake outlets
   per story rise, new stories per hour grow far slower than captures per
   hour (the paper now has 20 schedules, so captures per hour will rise;
   new stories per hour should barely move);
3. merge recall and the false merges read by hand (a rise in captures
   per story bought with topic-level merges is the failure the owner's
   ratio cannot see by itself);
4. the override rate (management disagreeing with Jev).

The labels alone may not move the ratio far: management still decides,
and with S2a its tasks do. If labels arrive and new stories per hour
stays near pages per hour, the next lever is the one already ruled (O3):
Jev settles high-confidence feeds without a model call.

## Heresy delta

- discovered: Jev pinned to a computer's first guest address while
  addresses change per boot; the window scorer capped at 50 documents;
  Casts accepted with an empty capture id.
- introduced: none.
- repaired: none (this note).
