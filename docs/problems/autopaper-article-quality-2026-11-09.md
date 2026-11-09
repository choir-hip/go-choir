# Autopaper article quality: short, duplicated, sometimes not English (2026-11-09)

Status: open. Problem and evidence only; no fix is in this commit.

Owner, 2026-11-09: articles are sometimes "very short... it looks like a
tweet"; one was in French ("everything should be written in English"); one
summarized several unrelated topics. On the fix: "we should look to fix
existing systems, not build a new system, and then we have two systems for
the same function." On duplicates: "we should have something to prevent
that already, so it means that that system needs to be revised."

## Evidence

A read-only audit of the public list on staging
(`GET /api/platform/publications/front`, all 58 pages) and 663 bodies
(`.../export?format=md`), run 2026-11-09.

- Corpus: 5,736 stories, updated 2026-10-13 to 2026-11-09.
- Word and source counts are the server's own, for every story. Language
  was checked on title and dek for every story and on the body for the 663.
- Data: [all_articles.csv](../evidence/autopaper-quality-audit-2026-11-09/all_articles.csv)
  (every permalink with words, sources, section and flags) and
  [dup_pairs.json](../evidence/autopaper-quality-audit-2026-11-09/dup_pairs.json).

| Defect | Count | Measured on |
|---|---|---|
| Under 300 words | 4,487 (78%) | all |
| Under 150 words | 2,930 (51%) | all |
| Under 50 words | 203; 33 are placeholders ("Test", "Probe block.", a bare "[source 1: ...]") | all; the 44 under 30 were read |
| Duplicate coverage of one event | at least 74 groups, 161 stories; 6 with identical titles | all titles (similar titles within two days; a lower bound) |
| Not in English | 32 (21 French, 6 German, 3 Spanish, 1 Portuguese, 1 other); none before 10-22, 17 from 11-06 to 11-09 | all titles, bodies confirmed |
| Several unrelated events in one story | 2 clear, 2 borderline, all from a roundup capture | titles of all, about 60 bodies read |
| Casting brief as the title and slug | 18, all dated 10-14 | all titles |
| Adjacent repeated cite markers ("[1] [1]") | 169 of 663 (25%) | bodies read |
| Process text in the body ("**Status.** Revision 4.") | a handful | bodies read |
| No section | 2,974 (52%) | all |

Length:
- Median 147 words; quartiles 95 and 270; 10% reach 500.
- Median words by cited sources: 67 with one, 106 with two, 148 with three,
  493 with six or more. 2,352 stories cite exactly two.

Not checked: earlier versions of each story, the roughly 5,000 unread
bodies, and whether cited links resolve.

## The existing mechanism for each defect

From reading the code at `97556a397`. No run traces were read, so every gap
below is a hypothesis until a trace confirms it.

### Duplicates

The mechanism exists and is running. It looks back a few hours.

1. **Capture dedup** (`internal/sourcegraph/captures.go:129`) keys on URL
   and content hash. It cannot see the same event from another outlet.
2. **Jev intake label** (`internal/agentcore/intake_labels.go:50-53`) asks
   whether a capture joins an existing story. Its candidates are the 30
   newest stories plus 10 picked by shared English title words from the
   last 72 hours.
   - The paper opens about 9 stories an hour, so 30 stories is about three
     and a half hours.
   - A timeout leaves the capture unlabelled: 4.9% of captures
     (`docs/evidence/token-efficiency-2026-11-09/intake-and-jev.md`).
   - Jev's recall on "joins an existing story" is 73% in that evidence.
3. **Management** is told never to open a second story for an event
   (`internal/promptstore/defaults/management.yaml:26-31`) and to read the
   story list first. The list defaults to 50 stories (about six hours), and
   no prompt says how far back to page.
4. **The library-moved guard** (`internal/store/management_tasks.go:638`)
   refuses a new story when the list changed since the task read it. It
   stops two tasks racing. It does not help when the older story was never
   in the window read.
5. **After publication** nothing merges or withdraws a duplicate.

Hypothesis: a second report of an event arriving more than a few hours
after the first is outside both windows, so Jev and management both call it
new. The earlier record
(`docs/problems/autopaper-intake-defects-2026-10-14.md`, defect 3) named
the 50-story cap; paging was added but the windows stayed small.

### Length

- The writer prompt has no length or depth target
  (`internal/textureprompts/texture.yaml:40-46`), and several lines push
  toward brevity: "One report of an event or a statement is enough"; stop
  asking for research "when its answers stop changing the document" (:70).
- The publish gates (`internal/agentcore/story_publish.go:187-286`) need a
  headline and two distinct outlets. Nothing checks length, so a 40-word
  story citing two outlets publishes.
- Two outlets is the floor, and two is the most common count. Hypothesis:
  the writer stops at the floor.
- The 33 placeholders: either they predate the gates (which landed
  2026-10-14 to 10-17; 99 of the 203 under-50-word stories are from 10-13
  to 10-15) or they came through the owner's publish route, which the gate
  does not hold. Their publish dates settle it.

### Language

- No prompt says which language to write in.
- 70 of the 255 registered sources are labelled non-English
  (`internal/sources/registry.json`). The label is the source's first
  language, copied through (`internal/sources/rss.go:131`); nothing acts
  on it.
- Hypothesis: the writer mirrors its sources, and two French outlets pass
  the two-outlet gate. Which sources the 32 stories cite settles it.

### Several events in one story

- Management is told "A story is one event" and to decline listings
  (`management.yaml:31`). There is no rule for one capture that is itself a
  roundup of several events. Both clear cases say in their own text that
  they came from a roundup.

### The rest

- **Brief as title:** fixed on 2026-10-14
  (`docs/problems/instruction-text-published-as-story-titles-2026-10-14.md`).
  The 18 stories predate the fix and were never corrected.
- **Process text:** forbidden by the prompt only (`texture.yaml:43`); no
  check.
- **Repeated cite markers:** the reader renders one marker per cite anchor
  (`frontend/src/packages/texture/texture-published-citations.ts:300-308`)
  and does not merge adjacent anchors to the same source.
- **No section:** sections come from the importance scorer
  (`internal/frontimportance/scorer.go`), which keeps a score for 14 days
  and reads only the newest 100 stories. Hypothesis: stories older than 14
  days lose their section, about half of a 27-day corpus.

## Direction

Revise the mechanisms above; build no second system for any of them. The
work is lane Q of
[the next-mission handoff](../design/autopaper-next-mission-handoff-2026-11-09.md).
