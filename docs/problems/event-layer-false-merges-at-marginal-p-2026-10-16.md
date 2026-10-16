# Event layer V1 fails its false-merge bar: joins at p 0.85–0.92 are topic merges and fake corroboration (2026-10-16)

Status: documented. Fix to follow.

## Acceptance result (V1, design §9; the paper 01:02–07:02 UTC)

| Criterion | Bar | Result |
|---|---|---|
| Captures resolved | ≥ 95% | **211 of 211** arrived captures have a membership (100%), 0 unresolved |
| Calls per flush | ≈ 1 | 69 calls over 66 flushes |
| Cost | reported | $0.0154 for 211 captures |
| False merges, blind sample | ≤ 5% | **7.0%** with both judges agreeing (3 of 43); 9.3% by the orchestrator's reading (4 of 43). **Fails** |

The sample:
- 43 joined memberships, one per (event, member URL): 16 from the paper,
  27 from the V3a test computer `v3-events-0146`, which runs the same
  resolver on six world feeds.
- The paper has only 16 distinct-URL joins. Its other 126 joins are
  repeat captures of one URL.
- The judges were `openai/gpt-6.1-sol` and `google/gemini-3.8-flash`
  through OpenRouter. They saw only the two titles and URLs, plus the
  resolver's own definition of the same event, never Jev's p (design
  §19.2).

## The false merges

| p | Event | Joined member | Judges | Corroboration rests on it |
|---|---|---|---|---|
| 0.90 | Iran war live: Saudi blasts Houthi "war crimes" | Middle East updates: Houthis target Yemen government palace | both different | yes (2 outlets) |
| 0.89 | Are student protests being manipulated ahead of France's p… | French police accused over war-like injuries to teenagers | both different | yes (2 outlets) |
| 0.86 | Ukraine says its Starshield access is back | Wes Streeting urges US to back Ukraine on Washington visit | one different | yes, with the Starshield event (2 outlets) |
| 0.98 | Will Israeli voters punish Benjamin Netanyahu? | In Israel, Netanyahu's ultimate legacy is the opposition… | one different, one unsure | no (1 outlet) |

The first three are on the test computer. The paper's one membership
both judges called different is tesla.com/roadster, titled only "Tesla
Roadster", joined to the Roadster reveal at p 0.98. The orchestrator
reads that as a correct member: it is the reveal's official page, which
the title alone does not show.

## What the evidence shows

- Across both computers the band **0.85 ≤ p < 0.92** holds exactly 3
  joined memberships. All 3 are false merges: topic merges of live
  blogs, protest coverage, and one country's news.
- All 3 create corroboration the event would not have without them: 3
  of the test computer's 14 corroborated events.
- Every membership both judges agreed was the same event has p ≥ 0.92.
  The minimum is 0.92 on the test computer and 0.98 on the paper.
- The paper has no joined membership in the band. Its 4 corroborations
  are unaffected.
- The remaining failure above the band is opinion pieces (Netanyahu,
  0.98). It is not corroborated.

## Classification

This is substrate: the event layer's join threshold (`eventJoinP`,
`internal/agentcore/event_layer.go`). The threshold was set at 0.85 by
the design (§5) before any staging data. V3 wakes management on
corroboration, so a false corroboration becomes a false wake and a story
opened on one outlet's report. The design states the bias: "the right
bias for a gate whose failure mode is false corroboration" (§12).

## Decision

**V3 event wakes stay off on the paper.** V1 has not passed.

## Fix direction

Raise `eventJoinP` to 0.92. A match from 0.5 up to 0.92 becomes a
tentative member: visible, not counted, and never minting a rival event.
The same bound governs two new captures forming one event.

Ways it could fail:
- **Missed merges:** true same-event matches between 0.85 and 0.92 stop
  counting. There were none in this data, but n is small.
- **Circular acceptance:** the threshold was chosen on this sample. V1
  acceptance must be rerun on a fresh window after the fix.
- **Existing memberships:** tentative is stored at write time, so the
  test computer's three band members stay counted. The paper has none.
