# The first design: Autopaper and its readers

> **Superseded where it conflicts (owner, 2026-11-24)** by [ideal-form-draft-2026-11-23.md](ideal-form-draft-2026-11-23.md) and the rulings recorded in `docs/definitions/choir-first-design-2026-11-22.md`: vectors go before keywords and rules in the hot path; importance is counted from independent origins on a subject, not asked of a model; the front page is a host render the root steers; subjects, mentions, groups and counts are written by host code as a named graph custodian.

Draft 4, 2026-11-23 (draft 3 plus the owner's rulings of the evening of 2026-11-22 and the failed offline test; section 13 says what changed; section 12 is the panel's reading of draft 2), taken from the top after a day's conversation with the
owner. Status: **idea**; nothing here is built unless a line says so, and
nothing is ratified. Lines marked (owner) are rulings, quoted in
[first-design-path-2026-11-22.md](first-design-path-2026-11-22.md) with the
proposals that were broken on the way and the code readings. Everything else
is the lead's proposal.

## 0. What it is for

(owner) "to cover the world's stories, to personalize into individual
autopaper editions, and serve as a foundation for autoradio, while using
minimal resources enabling scaling of sources and agents". (owner) "the goal
of any run shouldnt just be to write the best story, but to be building on and
using our corpus, and then the story adds to our corpus". (owner) "we need to
make this efficient and fast."

The measure of the whole, two numbers side by side. The **surprise rate**:
how much of what arrives the paper did not expect. The **corpus share**: of
what a run relies on (the pages it cites and reads to answer), the part the
paper already held (owner, 2026-11-22: "the point isnt to cut search to zero,
but to cut it down 50, 75, 80, 90 %. and that doesnt even need to cut search
volume... now we need to scale corpus utility"). The rungs are 50, 75, 80 and
90%. Web search volume is not a target and may stay flat or grow. Beside both:
cost per capture and per published story. The reason it matters outside the
paper (owner): "the paper should become, at scale, more economical than
exposing raw web search for any agent looking for current events".

## 1. The things

| Thing | What it is |
|---|---|
| **account** | a text about the world. A **capture** is an outlet's; a **story** is ours (a Texture, with revisions) |
| **subject** | (owner) "one bucket from which any symbol can be associated with a story": anything accounts refer to. No kinds; events and entities are not modelled apart |
| **mention** | this account refers to this subject: the passage, the time, the words as written, the role it plays there. What the passage says is that source's **claim**; the graph holds claims and settles none |
| **prediction** | a claim by a story or a root about what sources will report of a subject by a time. It registers a subscription (owner) |

Stories cite and transclude stories (owner: "textures reference textures is
the essence of it"). No two stories have the same subjects and the same
claims. A subject's importance is learned and held, not asked of a model each
time (section 6a); the number of stories about it is one input (owner, draft
2), no longer the definition.

## 2. The hot path: a capture arrives (no agent session)

1. **Fetch, clean, passages, embed.** Built.
   Rung 0 of the offline slice (2026-11-22) corrected this step: the path
   runs on what sources bring, not on pages a desk read for itself (two
   thirds of captures); the rules below find actors and proposed the
   happening for about one capture in five; lists had a median of 28 entries,
   most of it page furniture. So the text is cleaned to the article first,
   and the happening is offered from the subjects of the nearest stories by
   vector, which a story's Texture named.
2. **Candidate subjects, by rule.** (owner) "maybe we can predict subjects
   from keyword extraction and/or vector embedding". Three cheap sources, no
   model: known labels and aliases found in the text; proper names and rare
   key phrases not yet known; the subjects of the captures nearest by vector.
3. **Selection by System 1, one call a poll.** (owner) "any prediction can be
   scored with system1, which can also select subjects from a list, and also
   suggest more/other/idk". For every capture in the batch: which candidates
   is it about, or other / cannot tell. One job with every capture's
   questions in it, split only where the vendor's limits refuse it (the
   existing `jevJob`). System 1 picks one option a question, so "which of
   these" is one yes/no question for each capture and candidate, and one more:
   is there an important subject not listed. Only that last one escalates. An
   answer under the counting probability is stored as tentative and wakes
   nobody.
4. **Unmatched is the common case, and it does not escalate per capture**
   (rung 0b, 2026-11-22: 46 of 75 news arrivals report something no story
   covers; when a story exists the ten nearest by vector hold it three times
   in four). An unmatched capture is held with its actors and vector.
   Unmatched captures are grouped by vector, and a group reported by several
   distinct origins is named once by a language model and offered to the
   root as a subject that matters and has no story. The language-model desk
   also resolves the hard cases System 1 flags, many a session. A random
   share of what System 1 did not escalate is checked the same way, or its
   accuracy is never known.
5. **Write mentions; match subscriptions.** A lookup by subject id finds the
   predictions waiting on it. Matches are collected for a debounced wake.

Budget to hold: one embedding and a share of one System 1 call a capture;
seconds, not minutes; no session for a capture (the escalation desk and the
audit share are sessions, many captures each, and their share of captures is
measured from the first day).

The candidate service for captures exists and is deployed (station E2 of the
entity layer, `internal/platform/capture_ready.go`: exact, near, token
subset, initials, neighbours, with ambiguity and a stoplist); writing
mentions does not. This path adopts it and its probability bands; it does not
start again. The name `choir.subject` is taken (an owner-scoped user subject);
the stored kind stays `choir.entity`, widened, unless the custodian's design
says otherwise.

This replaces three mechanisms that each ask today which thing a capture
belongs to: the intake label's "same event", the event layer's call a minute
later, and a management session for every 12 captures.

## 3. The warm path: a story is woken

A story's Texture predicts what its subjects will do next (owner: "stories
predict next events and get woken with them"). It is woken, debounced, by
mentions that answer a prediction and by surprises on its subjects. It opens
with context found one way (owner): vector to enter, the graph to move,
sampled, metadata in the context and the full object one read away. It
revises (owner: "revising not making new stories"; a story stops growing when
its main point changes), cites and transcludes other stories, states new
predictions, and asks research only for a named gap. Research starts from the
graph and reports what it added and what was missing.

Budget to hold: one session for a wake, not one for a capture.

## 4. The root

(owner) The root Texture holds the whole view, predicts the paper's future
and does much of what management does now. It is one long session, resettable
at any time; its memory is its own Texture and that Texture's revisions. It is
woken by a debounced digest: its predictions answered, and surprises (claims
no prediction expected; subjects that matter and have no story). It opens
stories, which run as its sub-calls (owner: "concurrency is to be achieved by
desk subcalls"), merges two that have become one, and spends research by how
its predictions have scored.

The root is also where the paper's index is held and learned (owner,
2026-11-22: "counting importance, that sounds like something that gets done
via the root rlm texture. thats how we hold and learn an index over time, in
an llm's repl"). Its digest carries **aggregates of the capture process**, not
captures: arrivals per source and per subject, the share matched, unmatched
groups and their origins, density of the regions arrivals fell in, what
settled, what each source cost. In its REPL it may change the index with code:
add a subject, merge or remove duplicates, change a topic's importance (owner:
"we can look into letting it write code to dynamically change that"). Those
edits are the control on source processing: importance and density decide
which sources are polled how often and how much work an arrival may trigger
(owner: "a key parameter for global and user paper/radio usage... and thus
tuning source processing"). Every edit is a revision of the root's Texture, so
it is seen and can be reverted.

Other desks speak one interface to the Texture desk and may address a story
inside it; the root is asked about the whole and sees the rest (owner: "the
middle form"). Management returns to computer-wide coherence and admitting
engineering.

Proposed: the paper's root Texture is the front page.

## 5. The readers

- **A person's computer** has its own root, which predicts its owner (what
  they will read, skip, ask) and is settled by what they do. Proposed: its
  Texture is their edition.
- **Autoradio** becomes a desk that runs Go cells (owner) and composes by
  walking the graph: a subject, the stories about it, the story behind those.
  It asks the listener's root about the whole and may address a story.
  A listener's ask is a retrieval over the paper, scored like research's
  (section 6): keyword and vector search over the corpus, ranked by new,
  important and cited (owner, 2026-11-23). Today the answering model has no
  tools and sees at most 40 front-page titles.
- **The same arrival shape carries more than news** (owner, 2026-11-22: "dont
  overfit for text research and writing... autoradio will be a conduit for
  general computer use"): an important email interrupts the programme, is read
  aloud and answered by voice; an agent's question is asked aloud and the
  answer decides its next moves, run as code. So an arrival is anything that
  reaches a computer with a subject and an importance; the hot path and the
  root's wake must not assume a news capture.

## 6. The loop that learns

Predict, subscribe, be woken, be settled, see the score. A prediction is a
durable record made with its settlement fixed: the subject, a yes/no question
System 1 can ask of one arriving passage, a probability, a deadline.
Settlement is host code that runs when a mention is written and when a
deadline passes, not a wake. **Only captures settle, and copies of one wire
report count once; the paper's own stories never settle or count as
evidence.** A subject match routes a passage to the question; it does not
settle it. A prediction whose time passes is settled false (owner); one whose
predictor was closed by a crash is closed as interrupted and left out of the
score. The score is skill against a base rate from history, not accuracy, so
predicting the near-certain earns nothing. A wake is a read: what settled or
surprised since this predictor's cursor. Nothing is held in memory for a
restart to lose or resume.

The surprise rate has a denominator the paper cannot shrink: claims from
distinct origins on subjects that many outlets are reporting, of which no
prediction settled true. It is printed beside the count of open predictions. The score
is shown to the predictor. Today predictions are stored and some are scored;
nothing matches an arrival to a prediction, nothing settles on expiry, and
Texture is never shown its score.

## 6a. Novelty and importance

Two signals, kept apart (owner, 2026-11-22). **Novelty is semantic
sparsity**: "whether you are in dense or sparse regions of semantic space...
sparse semantic space is novelty". It is computed, not asked: the density of
held captures and stories around an arrival's vector. **Importance is
learned**: "follow ups to key stories will be important but not sparse.
novelty needs to be squared with importance"; "importance is something that
you learn continuously. otherwise you're just answering from model priors".
It is accumulated state on a region or subject, updated by events (arrivals
from distinct origins, citations, what readers and listeners do, the root's
edits), and it is assigned to **captures**, not only to stories (owner: "we
do need to score captures not stories").

What this replaces: the front-importance job, a host timer every ten minutes
that asks System 1 for a 1 to 10 score and ten section questions, in practice
about one story a call (666 calls over 144 runs), used only to order the front
page. The owner's rule on such jobs: "these scheduled jobs are a real hack.
thats the stuff that works but doesnt scale". A timer that re-asks a question
becomes state updated by the event that changes the answer.

**Learning over retrieval** (owner): "predictions then retrieval then scoring
predictions. and next prediction gets an accumulated sentiment of the past
predictions scoring". Before a retrieval, the asker predicts what the corpus
holds (dense or sparse, held or not); the retrieval settles it; the next
prediction is given the accumulated score. Sparsity is what is predicted.
This is section 6's loop applied to every search, research's and a
listener's alike, and its settled predictions are the numerator of the corpus
share in section 0.

## 6b. Sources and the scaling law

The capture path is designed with the sources, not after them (owner,
2026-11-22: "that makes sense and gives us a scaling law for our resource
consumption and ontology growth"). No source is added before this path
exists. The law to hold and to measure: adding a source costs its fetch, its
embeddings and a share of one System 1 call per poll; model sessions grow with
surprises and settled predictions, not with captures; subjects grow slower
than captures and flatten.

Measured on the paper, 2026-11-23 (store reading, `0f4421aae`): the 49.7 GiB
store is mostly the machinery's own log, not content. Events 3.6 GiB,
lifecycle commands 1.7, records between desks 1.3, runs 1.2, agent run memory
7.3; stories under 2. About 20 GiB is index and page overhead of 8 million
small objects. No superseded versions, 0.24 GiB of duplicates. Earlier counts:
about 9 model calls per capture and 58 per published story; 65% of a week's
captures were research's own web reads. So today cost follows agent activity,
which follows captures. That is the law this design must break.

## 7. Queries and their cost

Two edge families carry the graph: mention (account to subject) and citation
(story to story, the `transcludes` edge that exists). For mentions, each pattern is one or two index
reads. What is new on a subject since a time, and who is waiting on a
subject, run constantly and are single lookups. Stories like this one: shared
subjects weighted by rarity. What connects two subjects: accounts mentioning
both. Entry from words: vector to subjects and passages. Every read takes a
window and a limit, because a hub subject's neighbourhood is vast. Aliases
map to one canonical id, so a merge is a row; a mention keeps its words so a
wrong merge can be redone. In the REPL: about five verbs over these.

## 8. Filling it

Stories first: the 6,690 entity rows and the alias lexicon that exist are the
first subjects, and each story's Texture names its subjects as it writes, at
no added call. Then captures, newest first, through the hot path of section 2
run over history, computed off the paper and imported once checked on a
hand-read sample. Only derived tables are written, so a bad pass is redone.

## 9. What goes

Per-capture management tasks and their dispatcher; the intake label's "same
event" question; the event layer as a separate store (its call becomes step
3); the separate entity layer design; the old root desk, its setting and its
hourly cap (so the lanes that turn it on everywhere are not landed); the
front-importance timer (section 6a); the 1.5 s spacing of research children; the search refusal gate
(in flight); `research.related_story`.

## 10. Order of work

The panel's main change: nothing is retired until what replaces it is
proved, and the design is tried offline before it is wired in.

0. Log the desk on every model call (in build, lane f0-desk-on-calls).
1. Name the custodian of subjects and mentions, and the identity rules (how a
   thing that happens is told from another of the same name: by its time and
   the subjects mentioned beside it).
2. **The offline slice that proves or kills the design.** Replay a week of
   captures through the hot path into derived tables only: candidates, one
   System 1 job an hour of captures, the escalation desk with a check of each
   new name against its nearest existing subjects. Hand-read 200. Have 50
   existing stories predict on the first three days and settle them on the
   rest under section 6. Reported: was the right subject among the candidates;
   the share escalated; new subjects per 1,000 captures over time; the share
   later merged; skill against the base rate. Kill lines, set 2026-11-22
   before any run (ledger C192): the design is rethought if, over days 4 to 7
   of the replay, (a) more than 30% of captures escalate; or (b) new subjects
   per 1,000 captures are 70% or more of what they were over days 1 and 2
   (the vocabulary is not settling); or (c) rewritten 2026-11-22 after rung 0 showed the first
   wording was loose (docs/evidence/offline-slice-rung-0-rules-find-names-not-happenings-2026-11-22.md):
   for fewer than 80 of 100 news arrivals the candidates include at least one
   actor and the happening (or the reader marks the happening as new), in a
   list of 12 or fewer; or (d) the predictions' skill against the base rate cannot be
   told from zero (its interval includes zero). A run that cannot measure a
   line counts as failing it.
   **Result (2026-11-22, ledger C192): the first run failed all four lines.**
   The record is in the ledger and
   `docs/evidence/offline-slice-rung-0-rules-find-names-not-happenings-2026-11-22.md`;
   the lane is `lane/offline-slice-r0`. By the rule set before the run, the
   hot path as drawn in section 2 is not proved and steps 3 to 7 do not start.
   What the run changed is written into section 2 (steps 1 and 4). What it
   has not yet answered is whether the changed path passes; that is the next
   run, with sparsity (section 6a) as a measured column.
2a. A read-only measurement before the next run, from vectors that exist:
   for one day of research questions and arriving captures, the density of
   the region each fell in, against what then happened (did the run find its
   answer in the corpus; did the capture open or feed a story). If density
   does not separate those outcomes, section 6a is rethought before anything
   is built on it.
3. The hot path live, written beside the three old mechanisms and compared
   with them. Nothing a reader or a desk sees changes.
4. Predictions, settlement and the score, as host code and records.
5. Stories woken by settlements and surprises; something opens a story for a
   subject that has none. Only then are per-capture management tasks, the
   intake label's question and the event layer retired, together.
6. The root, with stories as its sub-calls (a new substrate: today's research
   children have no tape and die with their parent).
7. The readers.

A standing check from step 3 on: mentions resolved again under today's
aliases and compared with what is stored, so a vocabulary that fragments or
merges wrongly is seen.

## 11. Open

1. Who is custodian of subjects and mentions (one desk writes; others ask).
2. How a person's root reads the public graph without its queries or its
   owner model flowing back.
3. What roles a mention may carry, and whether any code needs them.
4. Which model the escalation desk runs.
5. How the hot path's candidate rules are tuned without a per-capture model.
6. Whether a story's Texture naming its own subjects is enough to find the
   subjects that are not proper names.
7. (panel) Importance: the owner's rule is the number of stories about a
   subject, which is zero for a subject with no story, yet the root must be
   woken by subjects that matter and have none. **Answered by the owner,
   2026-11-22 (section 6a):** importance is learned state on captures'
   regions and subjects, held by the root. Still open: the update rule, and
   what keeps a root's edit from making a subject important by fiat.
8. (panel) A time span on a subject, optional, so two elections or two rounds
   of talks with one name stay apart. It passes the test "does code behave
   differently", but it is the first step back toward kinds.
9. (panel) Wakes on hub subjects: one capture mentioning five hubs can match
   many predictions. The read-since-cursor wake coalesces them; whether that
   is enough is measured in the offline slice.
10. (panel) Removing the 1.5 s spacing of research children needs the caller
    fixed, or the gateway's refusals return.

## 12. The panel of 2026-11-22

Five members (Claude Opus, GPT 6.1 Sol, DeepSeek v4.1 flash, Muse Spark, GLM
5.3 flash) read draft 2 and the repository, read-only. All five: proceed with
changes. What each put first: only independent captures settle and count
(Opus); the measured slice from arrival to story wake before any retirement
(Sol); every wake gated on a counted mention from a distinct outlet, tentative
ones stored silently (DeepSeek); the custodian named and predictions durable,
interrupted on a crash and not false (Muse Spark); the hot path beside the old
ones and retired only after predictions are live (GLM).

Adopted above: independence of evidence and wire copies counted once; skill
against a base rate and a fixed denominator for surprise; settlement as host
code and wakes as reads; yes/no questions per candidate in one job split only
at vendor limits; tentative mentions; the deployed capture candidates adopted;
two edge families; the offline slice with kill lines; nothing retired before
its replacement; the standing re-resolution check; the escalation desk checks
a new name against existing subjects before writing.

Corrections to draft 2's facts: the capture side of the entity layer is
deployed for candidates (it said not built); `choir.subject` is a taken name;
the debounce in the old root code also enforces a minimum gap, which is
spacing and goes with it; draft 2 retired editorial routing two steps before
anything else opened stories.

Not adopted, for the owner: a time span on subjects (open question 8);
importance read from captures (open question 7). Dropped until code needs
them: roles on mentions.

## 13. Draft 4, 2026-11-23

Added from the owner's rulings of the evening of 2026-11-22, each quoted in
`docs/definitions/choir-first-design-2026-11-22.md` under `owner_rulings`:
the corpus share as the second measure (section 0); importance learned and
held, not counted or asked (sections 1, 6a, open question 7); semantic
sparsity as novelty and as the thing predicted (6a); learning over retrieval
(6a); the root given aggregates of the capture process and its index edits as
the control on source processing (4); the capture path designed with its
sources, and the scaling law (6b); arrivals that are not news, and a
listener's ask as a retrieval (5). Added from evidence: the offline test's
failure and the measurement that comes before its next run (10); the store's
composition (6b); the front-importance timer retired (6a, 9).

Not changed: the things, the hot path's steps, settlement, the panel's
adopted points, the rule that nothing is retired before its replacement is
proved. No panel has read draft 4.

New open questions: how density is computed cheaply at arrival and over what
window; whether importance lives on subjects, on regions of vector space, or
both; what a root may change by code without review, and how such an edit is
scored; how a private computer's root learns importance for its owner without
its data crossing owners.
