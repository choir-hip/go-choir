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

## Investigation 2026-11-09 evening

Read only, for clustering. Its defects (duplicates, length, language) belong
to the mission lane. Relation to this cluster: tonight the paper's supply is
thin on both sides. Search is answered by one provider (Keenable) and about
half its request lines are paid Bedrock Haiku, so thin or duplicated articles
can come from supply as well as from prompts. Under "understand before
tuning", do not tune prompts for quality until search supply and model
availability are measured stable. See
[the cluster record](paper-model-and-search-plane-cluster-2026-11-09.md).

## Lane Q build: causes and failure list (2026-11-10)

Read from the code at `origin/main` 9bbdfe68c and the 2026-11-09 audit data
(`all_articles.csv`). No run traces were read (the builder has no staging
access), so each cause below is a code-confirmed mechanism, and the
reader-visible effect is checked by rerunning the audit after deploy. The
The wave 0 source audit had not landed when this was written; if it
ranks a defect above these, it is the next commit. Every fix below revises a
mechanism that already runs; none adds a second system. A refusal at publish
reuses the existing hold: `AutoPublishRevision` returns a reason, the Texture
turn's result carries it to the writer as `publish_held`, and the revision
stays committed with its version a gap until a later revision passes. A hold
costs no model call of its own; the only loop is the writer revising, which
its turn bounds already limit.

Data checked in the CSV: stories from 10-27 on (2,950) have a section only
when under 14 days old at the time of the scorer's pass; below 100 words:
710 (24%), below 80: 416, below 50: 53.

| Defect | Confirmed cause | Fix | How it could fail | Test |
|---|---|---|---|---|
| Item language is the source's first language | `internal/sources/rss.go:131` stamps `firstString(source.Languages)` on every item; a mixed or mislabelled feed is wrong per item. `Script` is already read from the text (`script.go`), `Language` never is | `sources.GuessLanguage(text)` (function-word vote, Latin-script languages) overrides the label when it is confident and differs; the source's label stays when unsure | A short title with few function words stays unsure and keeps the source label (no worse than today); a code-switched item takes the majority language | `internal/sources` language test |
| Non-English stories publish | No writer prompt line names a language (`internal/textureprompts/texture.yaml:40-46`), and `AutoPublishRevision` (`internal/agentcore/story_publish.go:187-286`) has no language condition; two French outlets pass the two-outlet gate | One writer prompt line (write in English whatever the sources' language) and a held condition when the headline plus body vote for another language, or are mostly another script | Quotes in another language inside an English story must not trip it: the vote is over the whole text and needs a clear majority of function words in one other language; a story wholly in another script is held | `story_publish` language tests, including an English story with a French quote |
| Thin stories publish | No word floor anywhere; the only publish conditions are a citation, two outlets and a headline (`story_publish.go:210-249`). The writer prompt has no depth target and says "One report ... is enough" and "Stop asking for research when its answers stop changing the document" (`texture.yaml:40-46,70`). From 10-27, 24% of stories are under 100 words, 53 under 50 | `MinPublishWords = 100` held condition (words as the server counts them, CJK at two characters a word); the held reason tells the writer to expand from the captures it cites; the prompt gets a depth line and loses the two brevity pushes | A genuinely short breaking item is held until a second revision adds context: that is the intended trade; the hold is not a drop and the next revision that clears the floor publishes. A writer that never expands leaves the story a gap, visible in the turn result | floor and CJK tests |
| Placeholder text and process text publish | Pre-gate stories explain most: 91 of the 203 under-50-word stories are from 10-13 to 10-15, before the gates; the rest are thin stories. "Status." and "Revision N" lines are forbidden by the prompt only (`texture.yaml:43`) | A held condition for a body paragraph that opens with "Status", "Revision N", "Update:" or "In this revision", and for placeholder bodies (under the floor already) | A real sentence starting "Update:" in a live-blog style is held once; the writer rewrites it as prose, as the prompt already requires | process-text test |
| Duplicate stories | Jev's candidate list is the 30 newest stories plus 10 title-overlap picks from the rest of a 72-hour window (`internal/agentcore/intake_labels.go:50-53`, `:92-107`, `:120-137`); title overlap treats a CJK title as one token (`titleTokens`), and management's story-library guard (`internal/store/management_tasks.go:638`) only notices stories opened since its read. Nothing compares a story that is about to publish with the stories already published | A held condition at publish: another story in the index, updated earlier, whose headline shares at least 80% of its content words with this headline (both at least four content words) holds this story, naming the other; the Jev overlap tokenizer splits CJK into bigrams | A recurring headline form ("Markets close lower") shares words with yesterday's: the 80% bound and the four-word minimum limit it, and the hold is only on the later story. A distinct story that shares a headline's words is held until it differs: the reason names the other story so the writer can differentiate it or feed it | headline overlap tests |
| Adjacent duplicate cite markers (169 of 663 bodies) | The renderer emits one marker per anchor (`frontend/src/packages/texture/texture-structured-editor-doc.ts:255-257`, `texture-published-citations.ts:300-308`) with no check of the previous marker | `renderInlineContent` drops a marker that resolves to the same source number as the marker just before it with only space between. The source list still shows every quote | A dropped marker's popover quote is not reachable from the text; it stays in the Sources list | texture-structured-editor-doc spec |
| Roundup captured as one story | No rule in `management.yaml:26-31` for a capture that reports several events | One rule in the management prompt: split a roundup into its events or decline it | Over-splitting a single event with sub-parts; the rule names "several unrelated events" only | prompt text check |
| Failing feeds | `rss:venturebeat` answers 429 and `official:cisa:advisories` 403 to the paper's user agent and to curl, a browser agent and a feed-reader agent alike for CISA (read from this builder's network 2026-11-10); VentureBeat serves only fetchers it knows by name. Passing as another fetcher would be impersonation | Retire both rows with the reason here; both outlets stay reachable by search | A later change of the sites' policy would not be noticed; the audit's per-source error share shows it | registry guard updated |
| Stories with no section | Sections vanish by date (every story 10-13 to 10-26, none after 10-27) and in partial runs (11-07, 11-08): the 14-day score retention and the newest-100 read. Lane B owns sections | Left to lane B | | |
| Single-source stories | None since the two-outlet gate: 0 of 2,950 stories from 10-27 cite one source | Nothing to fix; the audit rerun shows it stays zero | | |

### Outcome of the build (2026-11-10, after the fresh source audit)

The wave 0 audit (`docs/evidence/autopaper-source-audit-2026-11-10/`) re-ranked
the list. Built: the language stamp, the publish holds (100-word floor,
non-English, process text) with the prompt's English and depth lines, one
marker for adjacent citations of a source (the stored text projection,
`internal/texturedoc/projection.go:158-168`, as well as the reader: 25% of
stored texts had "[1] [1]", so the renderer alone was not the cause), the
outlet fold (`internal/outlet/outlet.go`: bbc.co.uk into bbc.com, europesays.com
none), and the two failing feeds retired. Not built, with the reason:

- **Failed support checks (597 of 21,784).** The consequence already exists:
  a failed verdict mints a record to `texture:<doc>` that wakes the writer
  (`internal/store/support.go:178-205`, counted as work at
  `internal/store/story_research_request.go:421`, shown to the writer at
  `internal/texturewrite/frame.go:324`). A hold at publish cannot use the
  verdict because the verdict is written after the commit, off its path
  (`support_judge.go:60-90`), so a hold would mean making every publish wait
  on a model call. Hypotheses for the 14% still carrying one, not settled
  without the writer's turns after a `support_failure` record: (a) the count
  includes earlier published versions that a later revision corrected; (b) the
  judge lags (16% of 7-day checks pending), so the record arrives late; (c) the
  writer does not correct. Needs the run ledger of a story with a failure.
- **Duplicates.** A headline-overlap hold at publish was designed and not
  built: nothing local records which of two similar stories published first,
  so any hold rule either misses half the pairs or lets each of a pair hold
  the other (both stranded). The place to decide is the platform's publish,
  which knows the published set (not this lane's files). The Jev shortlist
  (`intake_labels.go:50-52`) is unchanged; lane A is rewriting its call site.
  Note the audit's duplicate pairs are within two days, inside the 72-hour
  window, so the window is not shown to be the cause.
- **Roundups, sections, revision churn.** Roundups: new stories show none
  (audit). Sections: lane B's file. Revision churn (median 4 revisions) is
  recorded, not investigated: it needs the turn-commit reasons of a few
  stories.
- **The floor value.** 100 words held 24% of stories from 10-27 on; the audit's
  new-story median is 187 with 36% under 150. The floor is the guard, the
  prompt's depth line is the lever; raise the floor only after the next audit
  shows the prompt moved the median.
