# The first design: Autopaper and its readers

Draft 2, 2026-11-22, taken from the top after a day's conversation with the
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

The measure of the whole: the **surprise rate** (how much of what arrives the
paper did not expect), beside cost per capture and per published story.

## 1. The things

| Thing | What it is |
|---|---|
| **account** | a text about the world. A **capture** is an outlet's; a **story** is ours (a Texture, with revisions) |
| **subject** | (owner) "one bucket from which any symbol can be associated with a story": anything accounts refer to. No kinds; events and entities are not modelled apart |
| **mention** | this account refers to this subject: the passage, the time, the words as written, the role it plays there. What the passage says is that source's **claim**; the graph holds claims and settles none |
| **prediction** | a claim by a story or a root about what sources will report of a subject by a time. It registers a subscription (owner) |

Stories cite and transclude stories (owner: "textures reference textures is
the essence of it"). No two stories have the same subjects and the same
claims; a subject's importance is the number of stories about it (owner).

## 2. The hot path: a capture arrives (no agent session)

1. **Fetch, clean, passages, embed.** Built.
2. **Candidate subjects, by rule.** (owner) "maybe we can predict subjects
   from keyword extraction and/or vector embedding". Three cheap sources, no
   model: known labels and aliases found in the text; proper names and rare
   key phrases not yet known; the subjects of the captures nearest by vector.
3. **Selection by System 1, one call a poll.** (owner) "any prediction can be
   scored with system1, which can also select subjects from a list, and also
   suggest more/other/idk". For every capture in the batch: which candidates
   is it about, or other / cannot tell. One call with every capture's
   question in it.
4. **Escalation.** "Other" goes to a language-model desk, many captures a
   session, which names a new subject or resolves the hard one. A random
   share of what System 1 did not escalate is checked the same way, or its
   accuracy is never known.
5. **Write mentions; match subscriptions.** A lookup by subject id finds the
   predictions waiting on it. Matches are collected for a debounced wake.

Budget to hold: one embedding and a share of one System 1 call a capture;
seconds, not minutes; no session of any desk.

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

## 6. The loop that learns

Predict, subscribe, be woken, be settled, see the score. Settlement is by the
claims that arrive (System 1 judges whether they answer the prediction) or by
the clock: a prediction whose time passes is settled false (owner). The score
is shown to the predictor. Today predictions are stored and some are scored;
nothing matches an arrival to a prediction, nothing settles on expiry, and
Texture is never shown its score.

## 7. Queries and their cost

One edge table, mention, carries the graph; each pattern is one or two index
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
hourly cap; the 1.5 s spacing of research children; the search refusal gate
(in flight); `research.related_story`.

## 10. Order of work

0. Log the desk on every model call: cost per capture and per story, by desk.
1. Subjects and mentions for stories, from what exists; test the query
   patterns on the paper's data (hub sizes; do rarity-weighted overlaps find
   real duplicates when read).
2. The hot path for captures; retire the three mechanisms it replaces.
3. Predictions that name a subject; match at mention write; settle on arrival
   and on expiry; show the score. The first surprise rate.
4. Stories woken by mentions; per-capture management tasks removed.
5. The root, with stories as its sub-calls.
6. The readers: the personal root and edition; Autoradio as a cell desk.

Each is a station with its own review and deployed proof; the paper keeps
publishing throughout.

## 11. Open

1. Who is custodian of subjects and mentions (one desk writes; others ask).
2. How a person's root reads the public graph without its queries or its
   owner model flowing back.
3. What roles a mention may carry, and whether any code needs them.
4. Which model the escalation desk runs.
5. How the hot path's candidate rules are tuned without a per-capture model.
6. Whether a story's Texture naming its own subjects is enough to find the
   subjects that are not proper names.
