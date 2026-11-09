# Handoff: the next Autopaper mission, and Autoradio decisions (2026-11-09)

Mutation class: green (a design handoff; no runtime change). Rollback: revert.

Source: a design conversation with the owner on 2026-11-09, held in a side
session while the main thread ran the Autopaper improvement mission. This is
input for the next throughline `/goal` file, not a goal file. The main thread
authors that goal file with `skills/throughline/SKILL.md` once the current
mission's live station closes.

Owner's words are quoted where they settle a point. Research figures are
labelled where they are relayed rather than verified.

## 1. Where this sits

- The current mission
  ([choir-autopaper-improvement-2026-11-09.md](../definitions/choir-autopaper-improvement-2026-11-09.md),
  S1 source audit through S5 renderers) finishes its live station first.
- This mission comes next. Its lead lane is Jev parallelism.
- Autoradio (section 7) is re-evaluated after this mission, not built in
  it.

## 2. How to run it: make it work, then make it nice

Owner: "a better agentic engineer in practice is to try to do everything all
at once and then review that. And it's probably going to be 85% good. And
then after doing QA on the first pass, then do a second pass." And: "you
want to be aggressive, but not excessively so."

So the mission has one wide build pass, a QA pass, and a second pass. It
has no small-v1-then-grow ladder. Everything in section 4 is in scope
for pass 1. It runs under the Operating Tempo rules in `CLAUDE.md`:
- Sonnet builders, one per lane in their own worktrees.
- Opus orchestrates.
- Runtime checks verify.

## 3. Dependencies and waves

Almost every lane depends on two contracts and one measurement. Doing those
first, and only those, is what stops the fan-out turning into merge
friction.

**Step 0 (orchestrator, before any builder starts; hours, not days):**

1. **The diff prompt contract.** This is the one input to the Texture agent:
   - the base revision it was written against;
   - its kind: `edit` (the owner's change in the body), `comment` (applies
     to the whole document), or `merge` (whole branch, picked parts, or by
     meaning);
   - the author;
   - the payload.

   Every lane that writes to a Texture goes through it.
2. **The changelog event contract.** It names the events the changelog app
   shows, at least:
   - a branch was made of a document you own;
   - your branch was merged;
   - a branch finished updating.

   It also defines how an event refers to the tape and how a
   major/minor level lets the view zoom.
3. **Jev measurement.** Measure:
   - the real per-organization limits, from the OpenRouter and TypeSafe
     consoles;
   - latency as questions per call grow (1, 8, 32, 64);
   - whether accuracy holds at high question counts.

   Lanes B and F set their concurrency from these numbers.

**Wave 1 (five lanes in parallel, disjoint packages):**

| Lane | Scope | Depends on |
|---|---|---|
| A. Jev | gateway limits, shared state, worker pool, distributions, version pin, cost | step 0.3 |
| B. Sections | section Texture, yes/no question per section, cache, visible-first recompute, calibration | 0.3 (numbers only) |
| C1. Texture revisions | diff prompts against a revision, Revise, branching, progressive branch updates | 0.1 |
| G1. House style guide | the house style guide as a Texture, applied at write and read time | none |
| H1. Ledger recording | responses, branches and merges on the tape with categories, entities and authors | 0.1, 0.2 |

**Wave 2 (starts as its wave 1 dependency lands, not when all of wave 1
lands):**

| Lane | Scope | Depends on |
|---|---|---|
| C2. Merges | whole-branch, pick-parts and by-meaning merges, credit kept | C1 |
| D. Prompt bar | universal input, revision indicator, undoable Jev routing, icon and verb in place of "Conductor" | C1, A |
| E. Changelog app | unseen-events pop-up, zoom from major events to the whole tape | C1, H1 (events emitted) |
| F. Asking | ask about an uncovered story, start an investigation, add a source by asking | C1 (a branch updates progressively), A |
| G2. Personal style | personal style guides and themes as Textures | G1 |
| H2. Algorithm² | the second prompt that proposes edits to the algorithm; a text or outline view of the behavior ledger | H1, C2 (an accepted proposal is a merge) |

The real chain is 0.1 → C1 → C2 → H2. That is the critical path. Staff
C1 with the strongest builder, and land it in slices so D, E and F can
start on the first slice.

Concurrency: about five builders at once across the Mac and the agents VM,
with compiles staggered (CLAUDE.md). Wave 2 has six lanes. Start them in
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
| 8 KiB judgment body | gateway body caps (`internal/gateway/body_caps_test.go`) | about 128 KiB; this is the binding constraint |
| 60 requests a minute per bucket | `internal/gateway/ratelimit.go`, `internal/gateway/jev.go:313` | token bucket at about 15–20 a second from the measured limit; interactive work in its own scope; shared backoff that honors `Retry-After` |
| 2-minute timeout | gateway Jev client | about 10 s plus one retry |
| sequential front scorer, `MaxCalls` 80 | `internal/frontimportance/scorer.go:110` | worker pool of 8–16 |
| per-question context | the System 1 question shape | one shared `state` per call, short per-question criteria; keep state under 32k |

Also:
- Store full probability distributions, so changing a threshold replays
  without new calls.
- Pin `jev-1.14` (the current `jev-latest`) wherever thresholds must stay
  stable.
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
- Compute scales with distinct section documents, not readers, so popular
  lists share their results.
- Calibrate per-section thresholds on a few hundred hand-labelled stories.
- An empty section suggests adding a source.

### C. Texture writing: edits, comments, branches, merges

Owner: in-place editing of documents you own "is still the essential
feature".

- **Edits and comments are one input.** Both are a diff prompt against a
  named revision, sent by Revise:
  - an edit is a change in the body, and only the owner can make one;
  - a comment is about the whole document, and anyone can make one.
- **No comments anchored to a span** (owner: "I find that to be quite an
  annoying feature… they're kind of holding it wrong"). If you disagree
  with part of a document, make your own version and edit it. Revisit only
  if users ask.
- **You can respond to an older revision.** The prompt bar shows which
  revision you are responding to (for example "v3 of v7"), and Texture
  already shows "newer version available".
- **A comment on a document you do not own creates a version you own.**
  It shows that it is updating straight away (progressive research), and
  research, engineering or management add revisions as their findings
  arrive. The document text explains what was asked, what was found and
  what changed.
- **Versions branch:** each person's comments and revisions are their own,
  and a response to an older revision branches from it.
- **The owner merges** in one of three ways:
  - the whole branch;
  - picked parts;
  - an agentic merge by meaning.

  Each is a diff prompt through Revise. The branch's author keeps the
  credit and is notified.
- **A merge is the strongest adoption signal we have.** Record it on the
  tape as a settled outcome: this person's work was adopted into that
  document.

Check before building: whether today's Texture revise path (see
`internal/textureowner/prompt_bar.go`, `continuePromptBarDocument`) can carry
`comment` and `merge` kinds, or needs a new path.

### D. The prompt bar

- It is the universal input:
  - on a document you own, Revise sends your edits;
  - on any document, it writes a whole-document comment;
  - otherwise it talks to the app agent: ask for a story, add a source, and
    so on.
- Show what it is aimed at with an icon and a verb in the placeholder
  ("Edit this story…", "Reply to…"). Drop the word "Conductor".
- Jev's routing is visible and one tap undoes it.
- Show the revision being responded to.

### E. The changelog app (new)

I found no existing design doc for it; the owner has meant to build it.
- A pop-up when there is something you have not seen: a branch of your
  document, a merge of your branch, a finished update.
- Major events by default; zoom in to reach the whole tape.
- This is the notification surface until the Autoradio app gives us push
  notifications.

### F. Asking and investigating

- Ask about a story no one is covering. Drive an investigation; it lands as
  a branch that updates progressively.
- Add sources by asking: RSS and beyond.

### G. Style

- A house style guide as a Texture, then personal style guides.
- Themes are Textures too: design values the renderer applies, shared,
  copied and adopted like section lists and algorithm documents.
- See the "Style-guide Textures (planned)" section in
  `docs/choir-prompting-invariants.md` and the precursor `Style.texture`.

### H. The behavior ledger and algorithm²

- The algorithm is a prompt that ranks and selects. Algorithm² is a prompt
  that governs how the algorithm changes.
- Behavior (responses, branches, merges, with categories, entities and
  authors) is a private ledger, the owner's property. It is evidence for
  algorithm² only.
- Algorithm² proposes edits to the algorithm, or applies them on rules the
  owner wrote ("auto-accept interest additions, ask before removals"). An
  accepted proposal is a merge.
- Algorithm² itself is edited by hand. There is no algorithm³.
- Show the ledger as text or an outline the owner can read.

Owner: "This is just going to expand and simplify as we build and use it
more." Pass 1 gets it working; it does not need to be final.

## 5. Out of scope

- Comments anchored to a span of text (rejected).
- Autoradio (section 7).

## 6. Done for pass 1

A reader can, on staging:
1. Edit their section list and watch the paper re-sort within seconds.
2. Edit their own document in place and Revise.
3. Comment on someone else's story and get their own branch, which updates
   and explains itself.
4. Have the owner merge that branch (whole, picked parts, or by meaning),
   and get notified in the changelog app.
5. Ask about an uncovered story and add a source by asking.
6. See their behavior ledger, and an algorithm² proposal that came from it.

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
  - Over-the-air updates deliver only our own reviewed releases.
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
