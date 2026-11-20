# The disclosure gate refuses publishes of stories whose Texture actor carries labels from reads of 2026-11-06 to 2026-11-08 (2026-11-20)

The labels are old and carried, not new. No landing of 2026-11-20 touched them.
The gate is right about what it is given and wrong about what it is given: the
labels stay on a story's actor and on its saved evidence after the read that
earned them was fixed, so a story that was ever labelled is labelled for good.

## Problem

On the paper (public computer `vm-1669020adcece51c2f5ff9b6df399ef5`, computer
`computer-1355b83bced1613eb048ee28d66688a8`) the host's story publisher is
refused by the disclosure gate for a set of old stories, and one research desk
cannot deliver its Report. The refusal is the publish-auto branch of
`disclosureAsk` (`internal/agentcore/disclosure_gate.go:507-518`, log line
`disclosure: publish-auto work refused without an owner ask`): the rules table
says `ask` for an owner-private label that is not admitted, the paper never
asks the owner (owner, 2026-11-08), so the attempt is refused and recorded
(`disclosure.decision`, phase `refused`).

- Publication refused on 2026-11-20, 9 times since the paper booted at 16:39Z
  (the gate counters: `publication|owner_private|ask|enforced` 9 against
  `publication|-|allow|allowed` 572 in `GET /api/runtime/observability`, read
  22:30Z), on four stories, every one with the labels `owner_private:autopaper,
  computers, model-policy, prompt-bar, texture`.
- A research desk (`research:94226d5a`, run ae50d61b) refused twice at the
  `desk_message` sink on `owner_private:autopaper` alone (19:47:21.837Z and
  19:47:42.487Z); its notes record the same refusal since 2026-11-19
  09:3xZ.
- The same four stories have had no published version since 2026-11-08
  (graph DB, below) while research and Texture keep revising them: 12 days.

## Evidence

Read only: the paper's run API with the paper key, the paper's console on node
B, the graph DB, the gate counters, the repo at origin/main (the paper runs
`bfc1492c6`, so every landing of today is in it).

### The gate, and what each label does at each sink

- `gateDisclosure` (`disclosure_gate.go:387`) takes the run's label set (cache
  union the record on the run) and calls `disclosure.Decide` (`internal/disclosure/rules.go:281`).
  Enforce mode for every kind landed 2026-11-08 06:22 (-0500, `1c9fcf758`,
  "modes deleted"); counters read `mode: enforce`.
- Table (`rules.go:138`): an owner-private label at `publication` is `ask`,
  and `allow` only when the host admits it. The admission
  (`publish_auto_admission.go:41-48`, `publishAutoLabels`) names four atoms:
  `capture`, `document`, `evidence`, `story`, and only for the document custodian on a
  publish-auto story whose provenance walks to the admitter. Every other
  owner-private atom asks, and on publish-auto work asks means refused.
- At `desk_message` the owner flow (`rules.go:196`, `ownerFlowAtoms`) lifts an
  ask for `capture, evidence, document, texture, story, model-policy,
  computers, prompt-bar` between the owner's own desks. `autopaper` is not in
  the list, which is why a research Report to Texture is refused on
  `autopaper` alone while every other label of the run passes (ledger, run
  ae50d61b, `verdict.hits`: one hit).
- The refusal is reached from `AutoPublishRevision` (`story_publish.go:235`),
  after the citation, outlet, headline and quality holds: a story that passes
  those and still carries the labels stops here.

### Where the labels come from

A run carries the union of the labels of everything it read
(`run_labels.go:1-30`). Reads that set the atoms in question:

| Atom | Read point | File:line |
| --- | --- | --- |
| `autopaper`, `prompt-bar`, `computers`, `model-policy`, and the like | a desk's product API read, labelled by the path's segment after `/api/` | `run_labels.go:282-305` (`productAPIReadLabels`, `productAPIObjectKind`) |
| `texture` | the Texture search verb and any child that read Texture text | `run_labels.go:187-199`, `researchTextureLabels` |

The `autopaper` atom was earned by GET `/api/autopaper/sources/schedules`
(the polling schedules, source configuration) until `e59ef0187` (2026-11-08
15:34 -0500; the ledger line of that date: "schedule reads stop tainting intake
Casts; the schedules read carries world_untrusted, not owner_private:autopaper").
That fix changed the read (`run_labels.go:295-303`). It did not change what the
earlier reads had already written. The only `/api/autopaper/` GET routes that
exist are `events` and `sources/schedules` (`internal/apihandler/routes.go:49-52`),
so no read of today earns `autopaper`; I found none.

### How a label that was earned once stays (four carriers, each read in a ledger)

1. **A Texture actor's carried conversation.** A Texture run with
   `actor_rewarm_carry_conversation` (`texture_agent_revision.go:775`)
   continues the previous run's conversation and takes that run's recorded
   labels (`run_memory.go:186-192`, `carryActorConversation`; the carried
   messages are kept verbatim, so the labels are accurate about the context).
   Texture run e8d38504 (story 1fa86f8b, 19:49:07Z): its metadata names source
   run 6dc8a68b, which held all ten labels; the first progress event of
   e8d38504 (19:54:24Z) already shows them, with no read of its own. The
   story's Texture actor carried `autopaper`, `prompt-bar`, `texture` in its
   first run in the list (2026-11-06T15:31Z), `computers` by 2026-11-06 21:38Z,
   `model-policy` by 2026-11-19 00:40Z, and has never lost one (every sampled
   run, 18 of 210, a set that only grows).
2. **Evidence a desk saved, and read back.** `save_evidence` stamps the writer
   run's whole label set on the object (`tools_evidence.go:133`, `StampMetadataLabels`);
   `read_evidence` puts the object's labels on the reader (`run_labels.go:249-255`).
   Research run ae50d61b started at 19:43:15Z with `capture, document, story`;
   at 19:43:49.49Z one cell read four evidence objects and the run jumped to
   nine labels, among them `autopaper`. Evidence 444ac6aa (written
   2026-11-18T00:2xZ, the desk's confirmed packet) carries all nine in its metadata.
3. **The desk session's union.** A run on a live desk session starts with
   every label any earlier run read into it (`tools_desk.go:791`,
   `seedRunLabels`); the session lives 30 minutes idle
   (`tools_desk.go:93`). Short; not the cause here (the paper rebooted at
   16:39Z and the runs still carry the labels).
4. **Records.** A record carries the labels of the run that committed it, and
   a run that is shown the record takes them (`run_labels.go:359`,
   `labelsOfRecord`). Research Reports to story 1fa86f8b carried
   `computers` from 2026-11-06T21:00Z and `model-policy` from 2026-11-19T00:29Z
   (record seq 561, run 9401f1d8). No Report to this story carries `autopaper`
   after 2026-11-08 (102 did before: 2 on 11-05, 37 on 11-06, 4 on 11-07, 59 on 11-08).

### Three refused runs in detail

All times 2026-11-20 UTC. Each run's labels were read from its own ledger and
from `GET /api/runs/{id}`.

1. **Texture e8d38504, story 1fa86f8b ("Beef import expansion draws Farm
   Bureau pushback ..."; 111 revisions).** Created 19:49:07Z by an inbox
   record (`integrate_execution_findings`), in the 19:35Z wave of research and
   Texture wakes. Source run 6dc8a68b (10:59Z), labels carried at once.
   System 1 asked at 19:55:07.9Z (yes 0.07, no 0.87, unsure 0.06). 19:55:10.202Z
   `disclosure.decision` phase `refused`, sink `publication`, target the story,
   hits `autopaper, computers, model-policy, prompt-bar, texture`. The run
   ended at 19:55:12Z, `completed`.
2. **Texture f090dbad, story 3004429b ("Felix Diemer: anticipation, not frame
   rate ...", 38 revisions).** Third refusal of that story in 36 minutes:
   12ce9cf8 at 19:55:33.891Z, 946aecb7 at 20:04:01.012Z, f090dbad at
   20:11:00.966Z, each a run that wrote a revision, published, was refused and
   ended `completed`. Same five hits. The last is the end of the window the
   other record names (20:11:01Z).
3. **Research ae50d61b, desk 94226d5a ("Trump faces backlash after invoking
   9/11 ...", 50 revisions).** 19:36:29Z (the same wave), 19:42:54Z started.
   Labels `capture, document, story` until the cell at 19:43:49Z read four
   saved evidence objects. At 19:47:21.837Z and 19:47:42.487Z `ReportPacket` and
   `Report` to `texture` refused: sink `desk_message`, one hit, `owner_private:autopaper`,
   System 1 no 0.81 and 0.68. The material is the desk's own packet of BBC,
   PolitiFact and Al Jazeera captures on a public political story.

The other two refused stories are 0ee907cc ("Ubicloud reports 100% schema
accuracy ...", 47 revisions; 19:59:49.185Z, run 2785312e) and 80035f27
("Republicans' midterm losses put the cost of living ...", 111 revisions; 17:16:42Z,
17:19:29Z, 17:22:59Z and 17:58:10Z, runs 00f08d9f, f867d11d, d5cbc4a1, 638311dc).
Total in ledgers since the 16:39Z boot: 4 + 1 + 3 + 1 = 9, the counter.

### Was it private material

What the labels stand for, in each run's own reads:

- `autopaper`: the polling schedules, source configuration, ruled "not owner
  content" on 2026-11-08 (ledger, the owner's "never ask" entry). Not private.
- `prompt-bar`: the owner's own prompt text that opened the story. The owner
  asked for the story; publishing it is the intent, not a leak.
- `computers`, `model-policy`: the desk's product API reads of the computer's
  status and model policy, made by research desks diagnosing the platform (the
  `platform_status` notes of the other record's loop). Operational, owner's
  computer: not public, and not in any of the stories' texts that I read, but
  they are in the carried conversation, so the context does hold them.
- `texture`: Texture search over the owner's documents. Owner text.

So the labels are true about the context and untrue about the stories: the
stories are public news built from public captures, and the carried
conversation still holds reads that were never part of them.

### How long, and how often

- Research desk 94226d5a: 44 refused `desk_message` Reports in the ledgers of
  its runs from 2026-11-16 on (scan stopped there): first 2026-11-17 18:04:10Z
  (run f8ff1494), 3 on 11-17, 13 on 11-18, 21 on 11-19, 7 on 11-20, last
  19:47:42Z. The desk's own notes put the first at 2026-11-19 09:3xZ; the
  ledgers are earlier.
- Publication of story 80035f27: refused 11 times, 04:24:35Z to 17:58:10Z on
  2026-11-20 (ledger scan of its runs since 2026-11-16: none earlier). Its
  earlier revisions were held by other publish holds, so the gate was not
  reached. The other three stories were scanned only in the 19:30Z to 20:20Z
  window. Gate counters have no earlier data (reset by the 16:39Z boot).
- The four stories have no publication after 2026-11-08 (graph DB,
  `og_objects` kind `choir.publication_version`): 1fa86f8b last 2026-11-07
  22:43Z; 0ee907cc 11-07 04:23Z; 3004429b 11-07 20:39Z; 80035f27 11-08
  09:49Z. Eight more stories created on or before 2026-11-08 and revised today
  show the same: 19c5ba93, 449165b4, 51320559, 57da147e, 6ffae721, 9a4c6fd9,
  ce0118ec and 94226d5a (last versions 2026-11-07 01:11Z to 2026-11-08
  11:10Z; none after 2026-11-09). 12 stories in the 1,000 docs updated today.
  (I read the labels of four of them and of 94226d5a; the other seven I did
  not read.) The stop coincides with the enforce flip of 2026-11-08 06:22 -0500
  and the intake fix that followed it at 15:34.
- The journal's gateway lines `site=disclosure_gate` for this computer
  (Jev calls for every ask, refused or pending) run at about 5 an hour from the
  start of the journal (2026-11-18 16:50Z) to now, with no step at any landing:
  30 on 11-18 (from 18:00Z), about 140 on 11-19, about 135 on 11-20.
- Landings: none of today's commits touch `run_labels.go`,
  `disclosure_gate.go`, `publish_auto_admission.go`, `internal/disclosure`,
  `tools_evidence.go` or `internal/types/labels.go`; the last commits to those
  paths are `a989b8b59` and `a4ffef8d3` of 2026-11-18, which change the
  capture floor on corpus search, not these atoms. Deploys that booted the
  paper today: 02:07, 03:19, 03:49, 05:00 ... and 16:39:27Z (receipt
  `bfc1492c6`, 16:27:29Z). The refusals precede and follow every one of them.

### Effect

- Publishing: 9 of 581 gate decisions on the publication sink since 16:39Z
  (1.5%). By stories the loss is 12 stories that are still being worked and
  have not been published in 12 days. Nothing is lost for the 572 other
  decisions. Every refusal is final for its revision: the revision stays
  `completed` and the next revision tries again, with the same labels; none of
  the 9 was later published.
- The desks do not retry in a loop: each refused run ends. They are woken
  again by the waves of the other record (14:48Z, 19:35Z today; about every 4
  to 5 hours): the 19:35Z wave woke these stories and produced these 5
  refusals in 25 minutes. The refusals are not the source of the waves. They are
  what the waves run into.
- The research desk 94226d5a writes `platform_status` notes ("refused by disclosure
  gate (owner_private:autopaper)") that management reads as a platform defect;
  management's consolidated escalation is a pending owner ask (disclosure
  3adf5e4c, `owner_private:capture,prompt-bar,story`, System 1 no 0.91). The
  paper's disclosure list holds 920 pending asks; 364 management, 66 research,
  3 Texture messages and 5 publications carry `autopaper`, from runs dated
  2026-11-05 (11), 11-08 (80), and a research tail of 1 to 5 a day from
  11-10 to 11-20 (the research desks'; 15 on 11-16 to 11-20).
  Not a source of the Relate story, but a second thing management turns into
  an escalation.

## Belief

The gate is right and the labels are wrong, in a specific way: the label model
says "this run read X" and then keeps that for the life of the actor and of
every object the actor saved. Three of the five atoms (`autopaper`,
`prompt-bar`, and what `computers` and `model-policy` stand for in these
stories' words) are not what the stories publish; the story text is public news.
Weakening the gate (adding `autopaper`, `prompt-bar`, `texture`, `computers`,
`model-policy` to `publishAutoLabels`, or `autopaper` to `ownerFlowAtoms`)
would admit what is in the conversation, including the owner's Texture search
text and platform status, and is the wrong place.

The smallest correct fix is at the carry, not at the gate: a Texture run that
continues its actor's conversation (`carryActorConversation`,
`run_memory.go:186-192`) takes the source run's labels with the messages. For
a publish-auto story whose source run's labels are not all in
`publishAutoLabels`, the run should start with no carried conversation (the
story's document is its state; the carry is a warm-cache and continuity
choice, owner 2026-11-09) and so with a clean label set that is true of its
context. The same rule for the research desk's read of its own old evidence
needs a second, separate step (the evidence stays stamped), named below.
Mutation class red (the disclosure labels and the path to Texture's canonical
write, publication); two seats; rollback revert; protected surfaces: run
labels, the disclosure gate, publish-auto admission.

Confidence about the cause: about 85%. About the mechanism of the carry on the
Texture actors, 90% (a ledger names the source run and the first progress
event shows the labels with no read). About the claim that no read today earns
`autopaper`: 70%; I found no route that does and no run that did, but I did not
read every run.

## Remaining error

- The earliest refusal of these stories: I have only the ledgers of today
  (counters reset at 16:39Z; console keeps 20:49Z onward and shows none, which
  agrees with the two-hour gap after 20:11Z). My ledger scan of earlier days
  covered 94226d5a and 80035f27 from 2026-11-16 only; 1fa86f8b, 0ee907cc and
  3004429b before today were not scanned.
- The first run that earned `autopaper` on each of the four stories is not
  read: the run list returns the newest 500 runs of a story, and 1fa86f8b,
  0ee907cc, 3004429b started before the oldest 500. All three already carry it
  in their oldest listed Texture run (2026-11-06). I did not read
  that run's events, which list no label before the model's first iteration
  (2026-11-06 ledgers carry no `labels`).
- Whether the 12 stories are all label-saturated: I read four and the research
  desk of a fifth. The rest are inferred from no publication after
  2026-11-09.
- Why the paper's other old stories do not appear: either nothing was revised,
  or they were fresh enough to be clean. Not counted.
- The 502 `failed to load private texture revision` on
  `/internal/platform/texture/publications` in the console (85 "failed and stays
  a gap" lines against 91 "published" between 20:49Z and 22:18Z) is another,
  larger stopper of publishes and is not this record. It needs its own
  record.
- The "10 refusals in 16 minutes" of the other record: the ledgers and the
  counter give 5 in that window (19:55:10Z to 20:11:00.966Z) and 9 since
  16:39Z; the one-per-console-line count may have included more lines than gate
  decisions.

## Next

Not done here (this record writes no code):

1. The fix at the carry, red, two seats: a Texture run on a publish-auto story
   does not carry the source run's conversation or labels when the labels exceed
   `publishAutoLabels` (`internal/agentcore/run_memory.go:186-192`, with the
   caller at `internal/textureowner/texture_agent_revision.go:775`). Test
   first: a publish-auto story whose source run holds `autopaper` starts a
   Texture run whose label set lacks it, and its publish passes the
   admission; a story whose source run holds only the four admitted atoms
   still carries. Rollback: revert.
2. The desk-message and evidence side (orange to red): a research desk's saved
   evidence keeps the labels of the run that saved it. Decide whether the
   evidence object should carry the labels of what it was derived from (the
   packet's reads) instead of the whole run's set; then 444ac6aa stops
   carrying `autopaper` for new reads. The record 94226d5a needs a fresh
   packet or a repaired object.
3. Owner or lead: the 12 stories. After 1 lands the next Texture run on each
   publishes its current revision; if the owner wants it sooner, an owner act
   on the tape.
4. Record the 502 stopper separately.
5. Re-read: the gate counters on the paper after the next wave (about
   00:30Z) should show no new `publication|owner_private|ask` if nothing wakes
   the four stories; the next wave that does will show it.
