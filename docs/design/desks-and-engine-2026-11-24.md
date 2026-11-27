# Desks and engine: the design as ruled on 2026-11-24

Standing: governing | kind: decided | settled-by: owner 2026-11-24 (line 11): 'the new design superseded'; docs/system-map.md section 10 names it current

This is the current design. It is the owner's rulings of 2026-11-22 to
2026-11-24 written as one text, with what the code has today beside each part.
Where it conflicts with an older design
([ideal-form draft](ideal-form-draft-2026-11-23.md),
[first design](first-design-2026-11-22.md),
[semantic layer](semantic-layer-2026-11-20.md)), this one wins (owner,
2026-11-24: "everything where designs conflict, the new design superseded").
The owner's words are in the `owner_rulings` of
[the first design mission](../definitions/choir-first-design-2026-11-22.md);
this file does not repeat them.

Status words: **built** (on main and deployed), **partly**, **not built**.
A name in a doc is not evidence; each status below is from reading code on
2026-11-24.

## 1. The desks

| Desk | What it does | Status |
| --- | --- | --- |
| Texture | One desk. The root and the stories are addressable inside it. The root opens stories; a story may open its own successor. | partly: stories built; the root as designed is not built |
| Capture | New. Turns arrivals into a symbolic compression. Custodian of subjects, mentions, groups and counts. | not built |
| Research | A service to the root and to stories: graph first, web second, asked for a named gap. Does not open or shape stories. | built, but still shapes stories through management |
| Autoradio | Takes the voice back from the phone's local model with a transition. | partly |
| Engineering | Unchanged: mutates only inside capsules. | built |
| Management | Computer-wide coherence, admits engineering, and manages all scoring. Decides no capture. | built as the old overloaded form |

The graph custodian is the capture desk's host service, as the mail service is
the Mail desk's: the part that runs with no model. The root does not write the
index; it asks the capture desk through a record. The root's own weight on a
subject is the root's forecast, layered on the counted rate.

## 2. What happens when something arrives

1. **Host code, no model:** arrival, cleaning, copy check, embedding,
   neighbours, grouping by neighbourhood and shared subjects, counting.
2. **The capture desk** holds the arrivals as data in its REPL, not as text in
   its prompt. It sends each group to a sub-call with a context chosen for
   that group: the stories nearest to it, the subject names its neighbours
   carry, and the slice of the paper about those subjects.
3. **Each sub-call returns a symbolic compression** of each capture: many
   subjects (one bucket, no kinds) and one clause for what is asserted.
   A capture is not put in a bucket.
4. **The custodian service writes** subjects, mentions and counts from that.
5. **No desk session is spent on a single capture.** Desks see totals and are
   woken by events.

The context slice is the digest the owner asked for: current events, and
compressed history for the months since the model's training cutoff. It goes
to the capture path only. Desks with a REPL get their context by querying the
corpus (section 5). Which context to attach to which kind of group is learned
(section 4).

**Today:** a host-started management task, a full agent run, decides at most
12 captures with a 10-minute deadline. On 3.4 hours of logs, 78 of 145 tasks
carried one capture. Intake kept up (479 taken in against 469 arrivals);
whether older captures were dropped is being measured.

## 3. Importance and novelty

- **Importance is a property of subjects**, counted from independent origins,
  with equal weights to start and weights learned later. A capture or story
  with many important subjects is important.
- **Novelty is the other dimension**, and the two multiply.

What the offline experiments on 2026-11-23's arrivals showed
([report](../evidence/offline-experiments-2026-11-24/README.md)):

- Importance from subjects holds up: it ranked the best Huang reinstatement
  arrival 2 of 170.
- **Every distance-based novelty failed on a turn** in a running story,
  because a turn's reports are close to the story's earlier reports.
  Corpus-wide sparsity ranked them 132.
- Burst (a subject's fast origin rate over its slow one) found the turn in its
  first hour (8 of the top 10) and cannot tell an echo from a first report.
- Counting new subject pairs rewards arrivals that list many things.
- Burst with near-copies scoring zero separated turn, follow-up and echo, but
  was found after the fact on one day. It is a lead, not a result.
- Vector groups almost never span more than one subject (90% had exactly one
  main subject), so "happening" is not a kind. A group that matters gets
  named and is then a subject.

So novelty is the open problem, and cheap counting does not solve it. The
model arms (flat batches against grouped calls with context) are the next
evidence. The measure cannot see a misranking until the paper records what
each page render showed; that record is part of the decision record.

## 4. The learning engine, at every decision

- Every decision point makes a forecast with a probability, settled by
  something the decider cannot write.
- **Creating an RLM is a protocol act that carries a prediction** of what the
  new one will do. It is scored, and the parent is shown its score before it
  delegates again. This is also what keeps fan-out and successor stories from
  running away, with no quota.
- **Management manages scoring.** Scores that plain data settles are settled
  in bulk by host code and management sees totals. For the rest it uses data
  in its own REPL, System 1 as a tool, its own thread, a sub-call, or another
  desk, and it audits a sample of what it did not look at.
- **Bounds are learned, not chosen.** A concurrency bound that follows a
  provider's real refusals or measured resource pressure is right. A constant
  we picked is not.
- A root edit to the index needs no prior review: it is a visible, revertible
  revision scored as a forecast.

**Today:** research children may carry a forecast (optional; without one the
weight is zero); a blind judge settles it under a scorer no desk can write
(built). Management's delegation carries no forecast and is never scored. No
parent is shown its sub-call scores. A fixed spacer of 40 sub-call model calls
a minute outlived the gateway limit it was written for; its removal is in
`lane/spacer-out`.

## 5. The RLM substrate

What the design needs, against the code:

| Needed | Today |
| --- | --- |
| Any desk's code creates another RLM | Only research spawns children; management delegates a task; Texture can do neither |
| A child may create a child | Forbidden by rule |
| Parent and child message each other while running | Not built; the parent blocks until all children finish |
| The corpus and new captures as values in the REPL | Per-item reads and small searches only; no iterator, no vector query from code, no traversal as values (`lane/corpus-repl`) |
| One plain model call returning text as a value | Not built; System 1 returns probabilities only, and only research may call it |

Reusable as it stands: desk-to-desk Cast, Ask, Reply and Report (they wake
their target), the sub-call ledger and blind judge, the custodian and lease
pattern, vectors and the neighbours query.

Deprecated: the entity candidate service (with entities and events) and the
generic fan-out helper.

## 6. The root, the page and the radio

- The root wakes on events, never a clock: arrivals that matter with no story,
  a burst on a subject that has one, a forecast settled, a part of the measure
  moving, a listener's question that could not be answered.
- The front page is a host render the root steers; it is also a person's
  edition, rendered on their own computer over the paper plus their private
  things. Nothing leaves a person's computer without explicit publishing.
- A story's point has turned when arrivals on its subjects are novel relative
  to the story itself (tentative, and section 3 says how hard that is to
  measure).
- When the listener speaks, the phone's local model answers at once and keeps
  talking until the Autoradio desk interrupts it, leading with a transition.
- The personal root learns from everything on the person's computer, added
  gradually; Autoradio first.

## 7. Order of work

1. Remove the spacer and the serial switch; shared back-off driven by real
   refusals (`lane/spacer-out`).
2. Corpus verbs in the REPL, for research first (`lane/corpus-repl`).
3. One creation verb on the protocol: required forecast, messages both ways,
   children may create children; a plain model call; sub-call scores fed back.
4. The capture desk, first in shadow beside today's path: host grouping,
   grouped calls, a decision-record row per group, nothing a reader sees.
5. Learned context, management's scoring, cutover, and deletion of the
   management task role, the intake labels and the event layer.
6. The root as designed, on the same substrate.

Shipping the app keeps the landing slot throughout.

## 8. Open

- What measures novelty (section 3).
- Who names a new subject and when two names are merged: the capture desk by
  default, with plain code where vectors and co-occurrence make it obvious.
- Whether a story carries the union of its captures' symbols or its own
  smaller set.
- How far back the history slice goes, given the paper holds data only from
  its first day.
- A separate voice for the phone's local model (owner's note, for later).
