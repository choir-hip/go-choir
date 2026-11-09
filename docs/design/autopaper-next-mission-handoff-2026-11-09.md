# Handoff: the next Autopaper mission, and Autoradio decisions (2026-11-09)

Mutation class: green (a design handoff; no runtime change). Rollback: revert.

Source: a design conversation with the owner on 2026-11-09, held in a side
session while the main thread ran the Autopaper improvement mission. This is
input for the next throughline `/goal` file, not a goal file. The main thread
authors that goal file with `skills/throughline/SKILL.md` once the current
mission's live station closes.

Owner's words are quoted where they settle a point. Research figures are
labelled where they are relayed rather than verified.

Revised the same day after a review and further owner rulings. What changed:
- readers respond with op-eds, and the paper never merges a reader's text
  (section 4.C);
- merging is one call with sources (section 4.C);
- the critical path is Texture revisions and the ledger, with Jev beside it;
- file owners per lane, review seats, custodians and staging identities are
  named.

## 1. Where this sits

- The current mission
  ([choir-autopaper-improvement-2026-11-09.md](../definitions/choir-autopaper-improvement-2026-11-09.md),
  S1 source audit through S5 renderers) finishes its live station first.
- This mission comes next. Its lead chain is Texture revisions, the ledger
  and op-eds. Jev parallelism runs beside it.
- Autoradio (section 7) is re-evaluated after this mission, not built in
  it.

## 2. How to run it: make it work, then make it nice

Owner: "a better agentic engineer in practice is to try to do everything all
at once and then review that. And it's probably going to be 85% good. And
then after doing QA on the first pass, then do a second pass." And: "you
want to be aggressive, but not excessively so."

So the mission has one wide build pass, a QA pass, and a second pass. It
has no small-v1-then-grow ladder. Everything in section 4 is in scope
for pass 1.

- Sonnet builders, one per lane in their own worktrees. Opus orchestrates.
- Runtime checks are the main verifier: drills on d@e, then deployed
  acceptance on staging.
- **Review** (owner, 2026-11-09: "One Devin round is good"): the Operating
  Tempo section of `CLAUDE.md` covers the data system cutover, not this
  mission. Lanes C1, C2 and H1 write Texture canonical records and tape
  records, so each gets one Devin round. The other lanes follow the
  standing tiers.

## 3. Dependencies and waves

Almost every lane depends on three contracts and one measurement. Doing
those first, and only those, is what stops the fan-out turning into merge
friction.

**Step 0 (orchestrator, before any builder starts; hours, not days):**

1. **The revise contract.** One call is the only input to the Texture
   agent:

   `revise(document, base revision, author, text, sources[])` → a new
   revision

   | text | sources | Meaning |
   |---|---|---|
   | yes | none | an edit (owner, in the body) or a comment (anyone, about the whole document) |
   | none | one revision | take that whole version exactly; no model call |
   | yes | one or more | the Texture agent merges under the instruction in the text |

   - The new revision records its base and every source as parents.
   - A source must be a published revision.
   - Credit and citation are read from the parent links.
   - Every lane that writes to a Texture goes through this call.
2. **The publish and adopt contract.** A reader and the paper are different
   computers. Results cross between computers only by publication to the
   Autopaper object graph and adoption from it, never by direct merge
   (`CLAUDE.md`, Safety). This step writes down:
   - what a published op-ed is on the graph and which story revision it
     names as its parent;
   - how the paper's computer learns of it and how the gate's verdict is
     recorded on each tape;
   - how the reader's computer learns that the paper responded;
   - which computer runs the section questions for a reader's own list, and
     where the cache lives (this decides whether popular lists share
     results, and which rate bucket lane A must size).
3. **The changelog event contract.** One rule produces the notifications:
   when a published revision has a parent written by someone else, that
   author is told. It covers:
   - a fork of your op-ed was published;
   - the paper responded to your op-ed;
   - another user took your version into theirs.

   Other events:
   - a document of yours finished updating;
   - an update was interrupted by a restart (a crash closes the open
     update with that fate and never resumes it; the document shows it);
   - the gate's verdict on your op-ed, with its reason.

   The contract also defines how an event refers to the tape and how a
   major/minor level lets the view zoom.
4. **Jev measurement.** Measure:
   - the real per-organization limits, from the OpenRouter and TypeSafe
     consoles;
   - latency as questions per call grow (1, 8, 32, 64);
   - whether accuracy holds at high question counts;
   - the deployed judgment body cap (the 8 KiB figure below comes from a
     test configuration string).

   Lanes B and F set their concurrency from these numbers, and the re-sort
   time in section 6 is set from them.

**Wave 1 (five lanes in parallel):**

| Lane | Scope | Depends on | Owns |
|---|---|---|---|
| A. Jev | gateway limits, shared state, a worker-pool library, distributions, cost, tests moved to 1.14 | 0.4 | `internal/gateway`, `internal/yaegikernel/system1.go`; supplies the pool that B calls |
| B. Sections | section Texture, yes/no question per section, cache, visible-first recompute | 0.2, 0.4 | `internal/frontimportance/scorer.go` and the four section lists |
| C1. Texture revisions | the revise call with sources and parents, Revise, forking, progressive updates | 0.1 | `internal/textureowner/prompt_bar.go` and the Texture write path |
| G1. House style guide | the house style guide as a Texture, applied at read time only | none | the read path only; it does not touch the write path |
| H1. Ledger records | the record types for responses, forks, publications, citations and gate verdicts | 0.1, 0.3 | defines the records; C1 and C2 emit them |

**Wave 2 (each lane starts as its dependency lands, not when all of wave 1
lands):**

| Lane | Scope | Depends on |
|---|---|---|
| C2. Op-eds | publish a fork as an op-ed, the gate, the paper's response and citation, user-to-user merges | C1, H1, 0.2 |
| D. Prompt bar | universal input, revision indicator, undoable routing, icon and verb in place of "Conductor" | C1's first slice, A |
| E. Changelog app | unseen-events pop-up, zoom from major events to the whole tape | C1, H1 (events emitted) |
| F. Asking | ask about an uncovered story, start an investigation, add a source by asking | C1, A |
| G2. Personal style | personal style guides and themes as Textures | G1 |
| H2. Algorithm² | the second prompt that proposes edits to the algorithm; a text or outline view of the behavior ledger | H1, C2 |

Lane D edits only the frontend prompt bar until C1's first slice lands;
C1 owns the server file.

**The critical path is 0.1 + 0.2 → C1 → H1 records emitted → C2 → H2.**
Staff it first, with the strongest builder on C1. C1's first slice is the
revise call with a text and no sources against a named revision, for an
owner's edit, with the parent recorded. Its proof is a revision on d@e
whose record names its base. D, E and F start on that slice. Lane A runs
beside the path; it is cheap and unblocks B and F.

Concurrency: about five builders at once across the Mac and the agents VM,
with compiles staggered (`CLAUDE.md`). Wave 2 has six lanes. Start them in
dependency order as wave 1 lanes free up rather than all at once.

**QA pass:**
- Drills on d@e, then deployed acceptance on staging (choir.news) for
  every lane.
- Then the full loop as a user, from section 6.
- Findings go to one ledger, with problem and evidence on one line each.

**Pass 2:** fix and polish from that ledger.

## 4. Scope

### A. Jev parallelism

Most of our Jev limits are ours, not the vendor's. Vendor facts:
- No question cap; only token limits (64k per request, and 32k for state
  plus any one question).
- `choice` questions take up to 255 options; `score` questions take 2–10
  levels; `noul` is a native yes/no type.
- Questions in one call share the state and run in parallel; adding
  questions "usually has little effect on response time".
- Price: $0.042 per million input tokens; output is free.

Relayed and unverified:
- about 20 requests a second and 250k tokens a second per organization;
- 70–500 ms latency;
- a shared state with fanned-out questions about 12x cheaper than separate
  calls.

| Our limit | Where | Change to |
|---|---|---|
| 12 questions per call | `internal/yaegikernel/system1.go:26` (`System1MaxQuestions`) | about 64 |
| 6 choices | same file (`System1MaxChoices`) | 255 |
| rune caps on text and choices | same file | sized to the token budget |
| 8 KiB judgment body (confirm in step 0.4) | gateway body caps (`internal/gateway/body_caps.go`, route `judgment`) | about 128 KiB |
| 60 requests a minute per bucket | `internal/gateway/ratelimit.go`, `internal/gateway/jev.go:313` | token bucket at about 15–20 a second from the measured limit; interactive work in its own scope; shared backoff that honors `Retry-After` |
| 2-minute timeout | gateway Jev client | about 10 s plus one retry |
| sequential front scorer, `MaxCalls` 80 | `internal/frontimportance/scorer.go:110` | a worker pool of 8–16 (the pool is A's library; B wires it into the scorer) |
| per-question context | the System 1 question shape | one shared `state` per call, short per-question criteria; keep state under 32k |

Also:
- Store full probability distributions, so changing a threshold replays
  without new calls.
- The runtime already calls `~typesafe/jev-latest`
  (`internal/agentcore/support_judge.go:37`), which is 1.14 today. Update
  the tests that still name `jev-1.13` to 1.14 (owner, 2026-11-09).
  `internal/agentcore/testdata/subcall_de_records.json` is recorded data
  and keeps 1.13.
- Record every call's cost.

Cost is not a constraint: rescoring 1,000 stories with about 20 questions
each is roughly $0.08. Latency and rate limits are the constraints.

### B. Sections as an editable Markdown list

Owner: sections should be "items in a list in Markdown" that a reader edits
on the fly, and Jev should "instantly recompute the whole auto paper"; this
was pulled ahead of personal style guides.

- One section Texture is the source of truth. It replaces the three
  hard-coded lists:
  - `internal/platform/publication_front_importance.go:41`;
  - `internal/frontimportance/scorer.go:340`;
  - `frontend/src/lib/autopaper.ts:341`;
  - plus `internal/maild/digest_render.go:26`.
- The house list is the default. Any reader can copy theirs and edit it.
- Each section is a `noul` question, so the number of sections is
  unbounded and a story can be in several. Importance is a `score`
  question, asked in the same call.
- Results are cached by (story revision, section document hash). Adding a
  section runs only its question.
- The visible front page recomputes first. At 15–20 calls a second the
  roughly 100 visible stories re-section in about 5–7 s, and 2,000 in about
  2 minutes. That is arithmetic, not a measurement.
- Compute scales with distinct section documents, not readers, if step 0.2
  puts the cache where lists can share it.
- Thresholds start from the house list's existing scores; no hand-labelled
  set in pass 1. A threshold lives in the section document, never as a
  per-computer setting.
- An empty section suggests adding a source.

### C. Texture writing: edits, comments, forks, op-eds

Owner: in-place editing of documents you own "is still the essential
feature".

**Writing**
- **Edits and comments are one input:** the revise call against a named
  revision.
  - An edit is a change in the body, and only the owner can make one.
  - A comment is about the whole document, and anyone with a computer can
    make one.
- **No comments anchored to a span** (owner: "I find that to be quite an
  annoying feature… they're kind of holding it wrong"). If you disagree
  with part of a document, make your own version and edit it. Revisit only
  if users ask.
- **You can respond to an older revision.** The prompt bar shows which
  revision you are responding to (for example "v3 of v7"), and Texture
  already shows "newer version available".
- **A comment on a document you do not own creates a fork you own,** on
  your own computer, private until you publish it.
  - It shows that it is updating straight away (progressive research).
  - Other desks send their findings as precommitment records, and Texture,
    the custodian, writes each revision.
  - The document text explains what was asked, what was found and what
    changed.
- **Who can respond:** anyone with a computer (subscribers and the free
  beta). Other readers can read the paper and its op-eds.

**Op-eds** (owner, 2026-11-09)
- **A published fork of an Autopaper story is an op-ed.** The owner: "it's
  kind of a mix between reporting and opinion, but we're going to stick
  with op-ed for now."
- **The paper never merges a reader's text.** When an op-ed on one of its
  stories is published, the paper's desk is woken with the op-ed in
  context. It researches the claim from its own perspective, then revises
  the story to integrate it, disagree with it, or leave the story as it is.
  The revision cites the op-ed, and the author is notified.
- **A fork of an op-ed, once published, notifies the op-ed's author.**
- Pass 1 lists published op-eds on the story they respond to and on the
  author's own page.
- Existing pieces to build on, not beside:
  `internal/platform/publication_opinion.go`, and the comment, letter and
  op-ed design in
  [autopaper-learning-materiality-2026-11-04.md](autopaper-learning-materiality-2026-11-04.md).
  Where that design differs from this section, this section is the newer
  owner ruling.

**The gate**
- A Jev judgment runs on every published op-ed before the desk is woken.
  It can reject an op-ed as abusive, redundant, incomprehensible or off
  topic.
  - Abusive: not listed against the story.
  - The others: still published on the author's own page, but the desk is
    not woken.
  - The author always sees the verdict and the reason.
- Every verdict is a record on the tape with its score, including the
  ones that did not wake the desk. When the desk is woken it can read the
  rejected op-eds for that story (as untrusted text, like any source).
- **Calibration comes from the desk's own reading:**
  - a rejected op-ed the desk goes on to use is a false reject;
  - an accepted op-ed that ends in "read it, no change" is a false accept.

  The gate's question wording and threshold live in a document and are
  edited there.
- **Rejects can wake the desk too, with backoff.** Without this, a story
  that only receives rejected op-eds is never reviewed.
  - The desk is woken when unread rejects on a story, counted by distinct
    author, pass a threshold.
  - The threshold doubles after a wake that changes nothing and resets
    after one that does.
  - A minimum gap between such wakes doubles the same way.
- **Coalesce:** op-eds that arrive while the desk is working on a story
  join that wake.
- This state is on the tape per story, so the owner can see why a wake did
  or did not happen.

**User to user**
- A user takes another user's published version into their own document
  with the same revise call: the whole version, or a merge under an
  instruction ("take their section on X, not the conclusion"). Picking
  parts and merging by meaning are the same thing with different
  instructions.
- Proof for an instructed merge: the new revision records both parents,
  and every claim it took from the source keeps its sources.

**Signals.** The adoption signals are: cited by the paper, forked and
published by others, taken into another user's document. Lane H1 records
them as precommitment records and their settlements, not as a second
ledger beside the tape.

### D. The prompt bar

- It is the universal input:
  - on a document you own, Revise sends your edits;
  - on any document, it writes a whole-document comment;
  - otherwise it talks to the app agent: ask for a story, add a source, and
    so on.
- Show what it is aimed at with an icon and a verb in the placeholder
  ("Edit this story…", "Reply to…"). Drop the word "Conductor".
- Jev classifies the input as an edit, a comment or a request to the
  agent. The chosen class is shown, and one tap re-sends the text under
  another class.
- Show the revision being responded to.

### E. The changelog app (new)

I found no existing design doc for it; the owner has meant to build it.
- It is a package desk registered like any other, with its own manifest.
  It reads the tape through a custodian lease.
- A pop-up when there is something you have not seen (the events in
  step 0.3).
- Major events by default; zoom in to reach the whole tape.
- This is the notification surface until the Autoradio app gives us push
  notifications.

### F. Asking and investigating

- Ask about a story no one is covering. It lands as a new document on the
  asker's computer that updates progressively.
- Add sources by asking. Pass 1 covers RSS and the source kinds the paper
  already has; anything further is pass 2.

### G. Style

- A house style guide as a Texture, then personal style guides.
- Pass 1 applies style at read time only.
- Themes are Textures too: design values the renderer applies, shared,
  copied and adopted like section lists and algorithm documents.
- See the "Style-guide Textures (planned)" section in
  `docs/choir-prompting-invariants.md` and the precursor `Style.texture`.

### H. The behavior ledger and algorithm²

- The algorithm is a prompt that ranks and selects. Algorithm² is a prompt
  that governs how the algorithm changes.
- Behavior (responses, forks, publications and citations, with categories,
  entities and authors) is a private ledger, the owner's property. It is
  evidence for algorithm² only.
- Algorithm² proposes edits to the algorithm, or applies them on rules the
  owner wrote ("auto-accept interest additions, ask before removals"). An
  accepted proposal is a revise call with the proposal as its source.
- Algorithm² itself is edited by hand. There is no algorithm³.
- Show the ledger as text or an outline the owner can read.

Owner: "This is just going to expand and simplify as we build and use it
more." Pass 1 gets it working; it does not need to be final.

### Custodians for the new kinds

Section lists, style guides, themes, the algorithm and algorithm² are
Texture documents, so Texture is their custodian. Gate verdicts and wake
state belong to the Autopaper desk. The goal file names the behavior
ledger's custodian and registers each kind (standing questions: single
state authority, registry hygiene).

## 5. Out of scope

- Comments anchored to a span of text (rejected).
- Autoradio (section 7).
- Subscribing to another user's edition, and newsletters. A user's edition
  is the base paper with their own section list and style guides; it is
  the next mission (owner: users "will eventually be able to basically
  fully automate their own autopaper").
- **Account-level trust and safety** (owner, 2026-11-09: "definitely a
  requirement for really launching, but it's not a core product
  requirement"). That means tracking accounts that produce many rejected
  op-eds, acting on them, and per-author gate thresholds. The gate's
  verdicts are already on the tape by author, so the evidence will exist.

## 6. Done for pass 1

Proved on staging with two identities on two computers: the paper's owner,
and a reader on d@e. A proof run as the owner responding to the owner's
own story does not count.

1. The reader edits their section list, and the visible stories re-sort
   within the time step 0.4 sets.
2. A user edits their own document in place and uses Revise.
3. The reader comments on a paper story and gets their own fork, which
   updates and explains itself.
4. The reader publishes the fork as an op-ed. The gate's verdict is on
   both tapes. The paper's desk responds in a new revision that cites the
   op-ed, and the reader sees the event in the changelog app.
5. A rejected op-ed does not wake the desk, and the desk can read it when
   it is next woken.
6. One user takes another's published version into their own document,
   whole and under an instruction, and the source's author is notified.
7. The reader asks about an uncovered story and adds a source by asking.
8. A user sees their behavior ledger, and an algorithm² proposal that came
   from it.

All of it is on the tape, every Jev call's cost is recorded, and the
measured Jev limits are written down.

## 7. Autoradio: settled decisions for the re-evaluation

Not built in this mission.
- **The App Store product is the Autoradio app:**
  - Texture and Autopaper, driven by voice. The Autoradio interface is the
    Texture interface, voice-driven, with images, audio and video embedded.
  - No desktop, no other apps, no app building, no super app, and no
    embedded Choir desktop. That rules out showing choir.news in a web
    view, since there articles are windows on the desktop.
  - Notifications open native screens. A Choir source renders natively from
    the Autopaper object graph. Outside sources open in a web view, and
    anything desktop-only opens choir.news in Safari.
  - Requested apps still run on the user's computer and are reached at
    choir.news.
- **Modules, not a super app.** Texture and Autoradio come first, Mail
  second. Each module is a reviewed first-party desk surface compiled in;
  nothing is installed at runtime. Themes are Textures (data, not code).
  Later products, such as a video app, ship as separate store apps.
- **Built on nsh** (`longbridge/nsh`, Apache-2.0):
  - Embed it in our own Xcode and Gradle projects (`NshAppView` / `NshView`
    embedding, with the host `AudioBackend`/`SpeechBackend` hooks and
    native components). This avoids `nsh package`'s Android limits.
  - Use upstream at a pinned version and fork only when we must (owner: "we
    don't need to fork it until we need to fork it").
  - Send general pieces upstream as small PRs: background session,
    Now Playing, PCM mic capture, `Speech.speak`, Android speech. Coordinate
    with issues #1059, #1060, #1064 and #1194.
  - Version 1 ships with no over-the-air code updates; every change goes
    through store review (owner, 2026-11-09: approval should be "as likely
    and quick and easy as possible").
  - nsh renders native UI from TSX, not Svelte. So the app's Texture
    renderer is a second renderer of one shared page format; the server
    decides what is on the page and each renderer only draws it.
- **Voice is local on both ends:** recognition, turn-taking, echo
  cancellation and the AI voice, ported from Mike
  (`/Users/wiz/Dev/Mike`, `mobile/plugins/speech/`).
  - Mike's voice code is relicensed so it can go upstream; all of it was
    written by the owner.
  - Owner: "Let the AI sound AI": no backchannels, no companion act. Human
    voices are the content; the AI voice is the connective tissue.
- **New native work:**
  - long-form background playback;
  - Now Playing and lock-screen controls;
  - clip start and end points anchored to transcripts;
  - playing and listening at once, with the podcast as the echo
    cancellation reference.
- **Content:** podcast audio from RSS. No YouTube in v1; its API forbids
  background and audio-only playback.
- **Pricing:**
  - $35 a month, with room to $50 and a $19–20 tier later.
  - More CPU, RAM and storage are sold as bundles. Token costs are
    amortized across users, not billed.
  - A free beta for the first ~100 users.
  - The subscription belongs to the Choir account. Launch with in-app
    purchase and web checkout at the same price, and favor the web later.
- **Open:**
  - who writes the radio program: Texture, or a new Autoradio desk (the desk
    model favors a desk);
  - the platform order after iOS;
  - CarPlay.
