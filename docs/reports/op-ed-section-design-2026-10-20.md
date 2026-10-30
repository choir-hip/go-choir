# The Opinion section of Autopaper: house op-eds and human op-eds

Draft 2026-10-20. Design only: no code changed. Mutation class of this
document is green. The class of the work it describes is in §9.

- Owner request, 2026-10-20: "before autoradio & iOS app we should build
  the op-ed section. That really makes the autopaper a paper. Hmm, I guess
  this requires some design thinking."
- Owner addition, 2026-10-20: "Op-Eds could be both done by agentic desks,
  and also by humans (or external agents for humans) to make it
  interactive."
- Standing rulings: [owner notes](owner-notes-2026-10-14.md) §6 (Opinions
  are built in the short term, launch gated on the foundations, line 70),
  §27 item 14 (a one-line AI-written disclosure on published stories, line
  609), §27 item 19 (the forecast act, line 621).
- Earlier intent: station LC in
  [the ranking and contributions intent](../archive/reports/autopaper-ranking-and-contributions-intent-2026-10-14.md)
  (line 56): op-eds that cite the paper's stories at pinned versions, in a
  clearly labelled Opinion section.
- Authority: [agent product doctrine](../agent-product-doctrine.md).
  Texture stays the sole writer of canonical document revisions.

## 1. Summary

Autopaper has news. It has no opinion. A newspaper has both, and the
second is what makes the first a paper with a point of view that readers
can disagree with.

This design adds an **Opinion section** with two kinds of author, because
the owner asked for both.

- **The house.** The paper's own desks write an op-ed in one editorial
  voice. It argues from news stories the paper already published, linked
  at the exact versions the desk read. It carries one to three dated
  forecasts, recorded as precommitment records before the piece goes out,
  to be scored in public later. That is the differentiator.
- **Humans, and agents acting for humans.** A person writes an op-ed in
  their own Texture on their own computer and submits it. Their own agent,
  holding a key the person issued, may draft or submit for them. The paper
  does not rewrite these. It screens, labels and lists them. Authors can
  reply to the house, and the house can reply to them.

The key decision is who owns the words. House pieces are Texture revisions
on the paper's computer, as doctrine requires. Human pieces stay the
human's own documents, and the paper only decides whether to **list**
them. One small record, the opinion listing, does that for both kinds. So
phase 1 can be house-only without a later migration.

Phase 1 is a house-only slice, unlisted to the public, so the owner can
read real pieces and judge quality. Human and agent authors come in phases
3 and 4. Public launch waits for the AI disclosure line and the forecast
record and settlement loop, as the owner ruled.

## 2. Problem and intent

**The problem.** Every page of Autopaper is a synthesis. The front list
holds living stories that the desks write and revise as evidence arrives
(`frontend/src/lib/autopaper.ts:18`). Nothing argues for a view, and
nothing invites a reader to answer back. The paper is to read, not to join.

**Why before Autoradio and the iOS app.** Those are new surfaces over the
same objects. Opinion is new content they will carry. Autoradio is already
framed as able to transcribe a take into an Autopaper Opinion (owner notes
§6). The content should exist before the surfaces that read it aloud.

**Intent.**

1. The paper can say "here is what we think the news means, and what we
   think happens next", plainly labelled, plainly AI-written, and
   checkable later.
2. People can answer the paper, in the paper, with its stories as the
   shared ground.
3. Opinion never leaks into the news. A reader who wants only news gets
   only news, ranked as before.

**Non-goals.** No change to news stories. No reader comment boxes. No
personalisation of the section. No public launch in phase 1. No model call
on the read path.

## 3. What exists today, with evidence

Facts read from the code at `1d192d9d7`.

**How a story reaches the front page.** Management gets a record of new
captures and opens a story with a Cast addressed to `texture:new`
(`internal/types/story.go:25`). The store creates a document and a
trajectory of kind `story` (`story.go:30`) and wakes Texture. A Cast may
set publish `auto` (`internal/types/commitment.go:405`), and then every
committed revision publishes as the next version. The computer publishes
through the guest route (`internal/proxy/platform_publish.go:150`), which
binds the caller to its owner and computer and sets mode `auto`
(`platform_publish.go:171`). The platform keeps the first version's mode
(`internal/platform/types.go:32-43`).

**What counts as a story.** A publication is an Autopaper story only if
its mode is `auto` and its owner is allowlisted
(`internal/platform/publication_front.go:149`). A publication a person
publishes by hand has mode `owner` and is not a story. The platform
already separates the paper's output from anyone else's.

**Ranking and sections.** The front page ranks by a host scorer's
importance, a depth factor and recency (`autopaper.ts:237`). The scorer
files each story under one of four sections
(`internal/frontimportance/scorer.go:340`;
`internal/platform/publication_front_importance.go:41`; the frontend list
at `autopaper.ts:330`). The scorer reads the front list, so an op-ed on
that list would be scored as news. §7 closes this off.

**What a story can link to.** Publish refuses an anchor whose target is
not a capture (`internal/platform/publication_versions.go:448`), so the
anchor path cannot point at another published story. There is a second
path. `publicCitationTarget` accepts a `choir:pub/` prefix
(`internal/platform/service.go:904-915`) and publish writes each such
citation as a `references` edge (`service.go:529-551`). Nothing produces
or renders a `choir:pub/` target yet. It is the existing place for "this
argues from story X at version n".

**What a forecast is.** A Cast can carry hypotheses of kind `forecast`,
which need a criterion and a `resolves_by` date after the commit and at
most seven days later (`internal/types/commitment.go:325-362`; refusals at
`internal/store/inbox.go:216-263`). Settlement of hypotheses on the live
paper is not built (`docs/precommitment-records.md` §8, line 761). A public
track record needs commitment hashes published when made with a signed
receipt (same document, §6). The last report found almost every recent
hypothesis was an assertion and one was a dated forecast
(`docs/reports/choir-last-three-days-2026-10-17.md:141`). So the record can
hold a forecast. Nothing yet scores one.

**What an outside author can use.** Accounts use passkeys. A person can
mint API keys with named scopes, optionally bound to one computer, with an
expiry and revocation (`internal/auth/store.go:868-934`), and proxy
handlers already check for an exact scope (for example `computer:lifecycle`,
`internal/proxy/computer_lifecycle.go:86`). A person can publish their own
Texture by hand (`platform_publish.go:142`). A reader can propose an edit
to a published story (`internal/platform/service.go:633`), but that is not
an op-ed.

**What does not exist.** No Opinion section, no author field beyond the
owner id, no AI disclosure line in the frontend (a search of the Autopaper
Svelte files and `TextureEditor.svelte` for "disclos" finds nothing), no
forecast settlement, no reply link between publications.

**Not verified.** The cost of one story turn (so no cap below rests on a
measurement; the paper tracks model cost in `internal/llmcost`); whether
the paper's computer can read the public front list's importance scores;
how owner-published slugs and paper slugs share route space; which desk
sends the forecast Cast. These are phase 1 investigation items.

## 4. The design choices

### 4.1 Whose voice

- **A. One house editorial voice.** Cheap, consistent, accountable as an
  institution, one simple label, one record to score. Risk: bland, and the
  paper agrees with itself.
- **B. Named AI columnists with stable positions.** Readers get someone to
  follow. But a model told to hold a position performs it, so its
  forecasts would measure a costume, not a view. It needs per-persona
  memory and more calls.
- **C. Owner-authored.** The owner's stated first use. It is one author,
  not a section.
- **D. Human and agent-for-human submissions.** Makes the section
  interactive. Needs identity, screening and abuse handling.

**Lean.** House voice (A) first, because forecasts attach cleanly to one
accountable voice. C is not separate: the owner is a human author under D
and the first one admitted. B is deferred. If the house voice is bland, the
cheaper fix is varied reasoning desks behind one byline, not personas.

### 4.2 How an op-ed picks its topic

- **A. Owner prompts.** Total control, but it does not run by itself.
- **B1. From the news the paper holds.** Management picks a published
  story whose event is corroborated by two or more outlets, is deep, and
  is still moving. The event data lives on the computer (reasons and
  origins at `internal/types/story.go:62-86`; counting rules in
  [the related-stories design](related-stories-graph-design-2026-10-19.md)
  §3.3). No new plumbing. Risk: the paper's opinion follows its own news,
  which is circular.
- **B2. From the host's importance scores.** Better signal of what
  matters, but those scores live on the host and corpusd attaches them. The
  computer does not read them today.
- **C. Demand from a human piece.** Available from phase 4.

**Lean.** B1, with owner nudges as a note to management, not a command
(management still checks the topic has stories to argue from). B2 later if
the owner finds B1 picks unimportant topics. To break the circle, every
house piece must cite **at least one primary capture** as a real anchor
beside the paper's own stories.

### 4.3 Forecasts

- **A. None.** Simple, but then the paper's opinion is as unaccountable as
  everyone else's, and there is no differentiator.
- **B. Optional.** Writers pick easy ones. The claims design names this
  cherry-picking (`claims-events-entities-ontology-2026-10-14.md` §7.0,
  gaming table, lines 955-962).
- **C. Required, one to three per piece.** Every opinion is paired with
  something that can be wrong, and the record is the learning signal item
  19 asks for. Risks: padding, a seven-day horizon, and a settler that
  does not exist yet.

**Lean: C.** The hypothesis record already has kind, criterion, date and
probability (`commitment.go:337-350`). Rules that keep it honest:

1. **Specific and dated.** The store already refuses a forecast with no
   date or one past seven days (`inbox.go:238-263`). A forecast that cannot
   settle counts against the writer (precommitment document §5).
2. **Committed before publication.** The record is minted when the Cast
   commits and the piece publishes after. In phase 1 a check compares text
   and record. In phase 2 the forecast box on the page is **drawn from the
   records**, so the page cannot drift from the commitment.
3. **Settled by someone else.** Rules first, for scheduled checkable events
   (item 19). The writing desk never settles, and sees aggregates of its
   own record, not item verdicts.
4. **No public score before the foundations.** Commitment hashes with
   signed receipts are not built, and the second witness waits on a signing
   key (precommitment document §6). Until then forecasts show as "open,
   unaudited". We do not print a score a reader cannot verify.
5. **Seven days for now.** An op-ed cannot forecast next year, which is a
   real loss. Raising the horizon is a red change to the hypothesis rules.
   Defer it until the first loop settles (Question 3).

Volume and coverage show beside accuracy, so a short record reads "n too
small", never "accurate".

### 4.4 Labelling: clearly opinion, clearly AI

Three labels, set by the platform from fields, never from the piece's text.

| Author class | Label |
|---|---|
| house | "Opinion. Written by Autopaper's AI desks." |
| agent for human | "Opinion. Written by an AI agent on behalf of [name]." |
| human | "Opinion. Submitted by [name]." |

- `author_class` is set from how the piece arrived (§4.9). A piece that
  claims in its text to be the house cannot get the house label.
- The human label says "submitted by", not "written by a human". The
  platform knows the channel (browser session or API key). It cannot prove a
  person typed the words. The author affirms authorship. We do not print a
  claim we cannot back.
- The house and agent labels **are** the item 14 disclosure and must exist
  before any public listing. The disclosure line is not built yet.
- The label shows in the list row, above the article and in the title. On a
  story page it belongs in the route's header bar
  (`frontend/src/lib/AutopaperRoute.svelte`, `ap-header`), because the
  article body is written imperatively by the editor and has no slot for a
  platform label ([related-stories design](related-stories-graph-design-2026-10-19.md) §3.6).
- Opinion does not use the news tier names (lead, top, brief). The list is
  a plain, dated column.

### 4.5 Linking to the news it argues from

- **At writing.** The piece cites each story at a pinned version (the
  pinning rule in `docs/autopaper-sw-design-2026-10-09.md` §2). The target
  is `choir:pub/<slug>/v<n>`, using the existing external-citation path.
  Validation against Store B at publish time is new, so a piece cannot cite
  a version that does not exist.
- **At reading.** A "From the news" block lists the pinned stories with
  title and version, and a mark if a newer version exists. Clicking goes to
  `/autopaper/<slug>/v<n>`, a route that exists (`autopaper.ts:48`).
- **In the graph (phase 2).** An `argues_from` edge from the op-ed version
  to each pinned story version, written in the same atomic batch as the
  version (`PublishTextureToGraph`, `publication_graph.go:31`). "Which
  op-eds argue from this story" is then one indexed lookup. Same catch as
  the related-stories design: the read path turns unknown edge kinds into
  citations, so the kind goes in the skip list in
  `service_publication_read.go`.
- **On the news page.** A small "Opinion on this" link appears under a
  story only when an op-ed argues from it. It is also how readers find the
  section.

### 4.6 Disagreement and rebuttal

- **A. A scheduled house counterpoint.** Staged disagreement inside one
  voice, double the cost, looks dishonest.
- **B. A reply is an op-ed with a link.** One mechanism for house and
  human, no comment moderation, cost only when someone has something to
  say.
- **C. Comment boxes.** The moderation load is the whole product, and the
  owner's contributions intent chose edits and responses over comments.

**Lean: B.** A piece may declare `responds_to`, pinning another Opinion
version. The target page shows a Responses list below it with the same
labels.

- **Human to house.** From phase 4. The piece pins the house piece and may
  contest one of its forecasts.
- **House to human.** Management's choice, never on request, at most one a
  day, only to a screened piece citing at least two published stories. The
  human text is untrusted data to the house desk (§10).
- **Forecast follow-up.** No note per settled forecast. One short "Looking
  back" brief a week covers every forecast settled that week, wrong ones
  first. One call for many outcomes.
- **Right of reply.** If a house piece names a public figure, their reply is
  a claim of theirs shown beside it (claims design §7.3, harm row, line
  1037). House pieces must not characterize private individuals.

### 4.7 Cadence and cost

The owner prefers more output per call, not more calls.

- **Cadence.** Start at three house pieces a week, hard cap one a day. The
  cap is a setting.
- **One call writes.** One Texture activation reads the pinned stories and
  writes the piece. No fresh research cast by default, because the stories
  already hold the research. It may ask for one primary source.
- **Forecasts ride the same call**, written and committed in the same turn.
- **Checks reuse what exists.** The citation support check already runs on
  stories. The op-ed uses it, batched.
- **Settlement is batched.** Rules settle most forecasts with no model. A
  model settler runs once a day over everything due.
- **Measure.** Phase 1 records tokens and cost per piece so the owner sets
  the cap on facts.
- **Schedule.** The daily "is an op-ed due" tick should reuse the standing
  intent substrate source schedules use, not a new timer. A crash closes a
  half-written piece with the fate "interrupted by a restart" and never
  resumes it (`CLAUDE.md`).

### 4.8 Authority boundaries

Texture is the only agent that writes canonical revisions; management never
writes documents; research is read-only (agent product doctrine,
Authority Boundaries).

- **House op-eds.** Management opens the piece with a Cast and picks the
  topic and pinned stories. Texture writes it. Research may supply a
  primary source. No new desk, no new writer. The only difference from news
  is the opening Cast's kind (§6.2).
- **Human op-eds.** The human's documents on the human's computer, edited
  by the human or their own Texture. The paper's Texture never edits them.
- **The platform lists, it does not write.** Listing, delisting, screening
  and labelling are platform acts on a separate record.

### 4.9 Human and external-agent authors

Phase 1 stays house-only, but the data model does not block this.

**How a submission arrives.** Two doors, one destination.

1. **The signed-in person** writes in their own Texture. A new "Submit to
   Opinion" action publishes the revision through the existing owner route
   (`platform_publish.go:142`) and files a submission naming the
   publication and revision.
2. **An external agent with the person's delegation.** The person mints an
   API key with scope `opinion:draft` or `opinion:submit`, bound to their
   computer, with an expiry (`internal/auth/store.go:910`). The agent writes
   the draft into the person's Texture through the document API. With
   `opinion:submit` it also files the submission. With only `opinion:draft`
   the draft waits for the person to press submit. **The default is
   draft-only.** Direct submit is for allowlisted authors in good standing.
   The person answers for everything their agent files.

Both doors end the same way. The platform reads the person's document and
revision from their computer, as the publish route already does, so the
caller names only the document and revision and cannot hand the platform
text of its choosing.

**Identity.** The platform sets `author_class` from the door: a browser
session is `human`, an API key is `agent_for_human`. The byline is the
account's display name; the key's label is kept as `via` and shown as "by
agent [label]". A display name is required, a real name is not. One
account has one name, and no name may imitate the house or the paper.
`author_class` is never read from a request body.

**The editorial gate and the doctrine.** Two ways to build it:

- *X. Texture publishes the submission.* The paper's Texture writes the
  text as a revision. This satisfies "sole writer" literally and is wrong in
  substance: the paper becomes author of someone else's words, invites
  quiet editing, blurs who wrote what, makes the paper responsible for all
  of it, and spends a Texture turn per submission.
- *Y. A separately owned object.* The human's document stays the human's.
  The paper holds a **listing**: whether it lists the piece, the screening
  result, the author class.

**Recommendation: Y.** Doctrine makes Texture the sole writer of the
paper's canonical revisions and treats a user's own edits as a separate
writer class (`AuthorUser`). A human op-ed is not a canonical Autopaper
revision, and no desk writes any of it. The paper's act is publication-side:
list or not. That keeps our voice and theirs apart, which the labels depend
on, and matches the existing split by publish mode
(`publication_front.go:149`). The paper's Texture may *respond* to a human
piece, as its own house op-ed, but never edits one.

**Screening and moderation before launch.**

1. **Invite only.** Only allowlisted accounts, the owner first.
2. **Cheap automatic checks.** Account standing, length bounds, a rate limit
   (for example one a day and five a week per author), citations that
   resolve, valid forecast format.
3. **One model screen per submission.** A typed judgement call, like the
   existing Jev scorer, for spam, harassment, doxxing, defamation of a named
   private individual, illegal content. Batched where possible.
4. **A human decision.** Before launch the owner approves every listing.
   The piece waits as `pending` with the screen result beside it. Later,
   management may approve low-risk pieces under an owner policy.
5. **Takedown is a listing change.** A report or owner action sets the
   listing to `delisted` with a reason. The paper does not delete the
   author's publication. A legal order is another path and needs counsel,
   who reads later (item 14's pattern).
6. **Untrusted text.** A submission is data. It never enters a desk's
   instruction channel. When the house reads one, it is quoted and marked
   as outside content.

**Do outside op-eds carry forecasts?** Yes, optionally, under the same
rules. The claims design says Choir records a speaker's claims as
attributed precommitments and speakers do not emit records (decision D1,
line 1499 of that document). So:

- An author may add one to three forecasts in the form, using the same
  fields.
- The **platform** mints the commitment, with the **admission time** as the
  commit time, attributed to the author's account, with `via` for an agent.
- Settlement is the same, and the author never settles.
- A per-author scoreboard shows forecasts made, settled and scored, beside
  the pieces that carried none. Optional forecasts invite cherry-picking, so
  the visible count of pieces **without** forecasts is the countermeasure.

**Replies between authors** are covered in §4.6. One `responds_to` edge
serves house-to-human, human-to-house and human-to-human.

## 5. Recommended design

**An Opinion section of labelled, dated, linked pieces, written first by
the house in one voice with recorded forecasts, and later by admitted
humans and their agents as their own documents, all listed through one
record the paper controls.**

1. One piece, one listing. Every piece is a publication plus an *opinion
   listing* carrying author class, state and screening.
2. House pieces are Texture stories opened by a management Cast of kind
   `opinion`. They publish once.
3. Human pieces are the author's own publications, filed as submissions,
   screened and listed.
4. Labels come from the listing, set by the platform.
5. Links to news are pinned-version references now, graph edges later.
   Replies are pieces with `responds_to`.
6. Forecasts are recorded before publication, shown as open commitments,
   and scored in public only after the foundations exist.
7. Opinion never enters the news front, its ranking, or the host scorer.

## 6. Data model

### 6.1 Records

Existing records are reused. The additions are small.

| Item | Kind | Notes |
|---|---|---|
| `piece_kind` (`news`, `opinion`) | field on publication metadata, Store B | Stamped by the platform from the route used. Kept from the first version, like publish mode |
| `choir.opinion_listing` | new object kind, Store B, platform-owned | One per piece. Fields below |
| `argues_from` | new edge kind | Op-ed version to pinned story version |
| `responds_to` | new edge kind | Op-ed version to pinned Opinion version |
| forecast records | existing `choir.commitment_record`, hypothesis kind `forecast` | No new type |
| forecast export | the existing exported-verdicts pattern (`StoryVerdictsPath`, `story.go`) | Phase 2 |

The listing holds: publication id and version, `author_class` (`house`,
`human`, `agent_for_human`), author account id, `via`, state (`pending`,
`listed`, `delisted`, `rejected`), reason, screening result, who decided,
and times. The platform writes it. A state change is a new write that
supersedes the last, so history stays. A new kind must be registered or
the write gate refuses it (`internal/objectgraph/registry.go`, `CheckWrite`).
The Autopaper design says avoid new types unless needed; these are needed,
because the listing is the only way the paper can list another person's
document without owning it.

### 6.2 How a house piece is marked opinion

The model must not choose the label. The opening Cast carries
`kind: opinion`, which the runtime copies into the trajectory's subject
refs beside the publish mode (`story.go:34-40`). The guest publish request
(`types.StoryPublishRequest`, decoded with `DisallowUnknownFields` at
`platform_publish.go:163`) gains one field, `piece_kind`, set by the runtime
from the trajectory. The platform accepts `opinion` only on the guest
route, only for an allowlisted owner, only with mode `auto`.

### 6.3 The list

The Opinion list is the listings in state `listed`, newest first, a new
corpusd read. Like the front list it reads Store B alone and calls no
computer, so it works while the paper computer is stopped.

## 7. How it fits the current front page

1. `frontStory` skips a publication whose `piece_kind` is `opinion`
   (`publication_front.go:403`).
2. The host scorer reads the front list, so it never sees opinion and files
   none under a section.
3. `AUTOPAPER_SECTIONS` (`autopaper.ts:330`) gains an `opinion` tab, but
   `usesSections` must not count opinion toward its half-the-stories
   threshold, and the tab shows only when listed pieces exist.
4. `parseAutopaperPath` (`autopaper.ts:48`) reads the first path segment as
   a story slug, so `/autopaper/opinion` would collide with a story titled
   "Opinion". The tab is a section view of the front page with its own list
   call, not a new top-level path. A piece keeps the normal
   `/autopaper/<slug>` form.

## 8. Phased plan

**Phase 1: the smallest useful slice. House pieces, unlisted.**

- Define `piece_kind`, the listing and the reference format.
- Management opens an `opinion` Cast from a corroborated, deep story, at
  most three a week. Texture writes the piece, pinned to the stories it
  read, with at least one primary-source anchor and one to three forecasts
  committed in the same Cast.
- The piece publishes once. Its listing is `pending`; the public Opinion
  list is empty. The owner reads pieces by direct link and an internal list.
- Opinion is excluded from the news front and the scorer.
- The page shows the house label in the header. Forecasts appear as text
  marked "open, unaudited".
- Cost is recorded per piece.
- This phase answers only one question: are the pieces good.

**Phase 2: checkable and linkable.** `argues_from` edges, the "From the
news" block, "Opinion on this" under news. Forecast box drawn from records;
records exported; a rule settler; the weekly "Looking back" brief. The AI
disclosure line built. Platform refusal of a second version unless it is a
labelled correction. Public listing of house pieces after the owner's go.

**Phase 3: the owner as first human author.** The "Submit to Opinion"
action, the submission record, screening, the approval queue, the `human`
label. Owner and invited accounts only. This proves the human path with one
trusted author.

**Phase 4: agents for humans, and replies.** Key scopes `opinion:draft` and
`opinion:submit`, draft-only default. `responds_to`, the Responses list,
house replies under the one-a-day rule, author scoreboards.

**Phase 5, not now.** Open submissions, Autoradio takes into Opinion,
personalisation, longer forecast horizons.

## 9. Mutation classes

- **Green:** this document.
- **Orange:** management and Texture prompt framing for the `opinion` Cast
  (new desk behaviour); exclusion from front list and scorer (a mistake
  leaks opinion into news, so it needs a regression test); the Opinion
  tab, header label and Responses list; the screening model call.
- **Red:** `piece_kind` on the guest publish route and metadata
  (publication write path); the listing and new edge kinds (Store B schema
  and writes); forecast records, export, settler and scoreboard
  (commitment ledger, evidence, scoring); API key scopes and the submission
  route (auth and publish).

Red work follows the full ceremony in `CLAUDE.md`: conjecture delta,
protected surfaces, admissible evidence, rollback, heresy delta. Each
phase's goal file carries it.

## 10. Failure modes

- **Opinion appears in news ranking or sections.** Guard: `piece_kind`
  exclusion, a regression test, scorer never sees it.
- **Label spoofing.** Guard: the label comes from the listing, never text or
  request body.
- **The paper argues only from itself.** Guard: one primary anchor per
  house piece.
- **Confabulated facts.** Guard: every factual claim cites a pinned story or
  capture; the support check runs; unsupported claims block publish.
- **Vague or padded forecasts.** Guard: specific, dated, probability,
  criterion; unresolvable counts as wrong; coverage shown.
- **Page drifts from the commitment.** Guard: publish once; box drawn from
  records.
- **A public score nobody can verify.** Guard: none until signed receipts
  exist; "unaudited" until then.
- **A dull, self-agreeing section.** Guard: the owner reads phase 1; varied
  reasoning desks behind one byline.
- **Cost runs away.** Guard: weekly cap, cost per piece, batched checks.
- **Prompt injection from a submission.** Guard: submissions are quoted
  untrusted data, never instructions.
- **Spam, sybil accounts, agent floods.** Guard: invite only, rate limits,
  one name per account, draft-only default, revocable keys, owner approval.
- **Defamation or harassment.** Guard: no pieces about private individuals,
  the screen, takedown by delisting, counsel reads later.
- **Candidate endorsement.** The paper becomes a political actor. Question
  13 asks the owner to forbid it.
- **A crash mid-piece.** The piece closes "interrupted by a restart" and is
  not resumed; the next tick is new work.
- **Direct-link exposure before launch.** Unlisted is not private. Question
  5 asks if that is acceptable.
- **Slug collision between a human piece and a paper story.** Unverified;
  check before phase 3.

## 11. Acceptance proof on staging

Staging is `https://choir.news`. Phase 1 proof:

1. The deployed commit matches the pushed SHA in the health report.
2. Management opens one `opinion` Cast from a story that meets the rule.
   The ledger shows the Cast kind, the pinned stories, and the forecast
   records committed **before** the publish event.
3. The piece publishes as v1 with `piece_kind` `opinion` and a `pending`
   listing. A second revision is not published.
4. A Playwright test and an API call assert that the front list API does not
   contain the piece; the public Opinion list is empty; the direct link
   loads with the house label in the header; each "From the news" reference
   resolves to a real published version; the forecasts in the text equal the
   committed records.
5. The scorer's next run files the piece under no section.
6. A cost record exists for the piece.
7. A malformed Cast (a forecast with no date, an opinion with no primary
   anchor) is refused, and the refusal is in the ledger.

The artifact is a bundle of Cast id, trajectory id, publication id,
forecast record ids and the Playwright report. Phases 3 and 4 add: a
spoofed label fails; a revoked key stops working; a `pending` listing is
invisible to the public; a delisted piece leaves the list and its permalink
still resolves.

## 12. Rollback

- **Stop production.** One setting stops management opening `opinion`
  Casts. Existing pieces stay.
- **Hide the section.** A second setting makes the public list empty and
  hides the tab.
- **Delist.** Any listing can be set to `delisted`. No data is deleted.
- **Revert.** The code is additive: a field, three kinds, a route. A revert
  leaves unused kinds nothing reads. The proxy refuses unknown fields on the
  guest publish request, so guest and platform must update together; the
  guest update path (station LU) covers that and the order belongs in the
  phase 1 goal file.
- **Records.** Minted forecast records are never edited. A rollback marks
  them withdrawn by a new record.

## Questions for the owner

Each can be answered yes or no. My recommendation follows each.

1. **House voice first.** Phase 1 is one unnamed house voice, "Autopaper's
   AI desks", not named AI columnists. Recommendation: **yes**.
2. **Forecasts required.** Every house op-ed carries one to three dated
   forecasts recorded before publication. Recommendation: **yes**.
3. **Seven-day limit stays for now.** A longer horizon is a later change.
   Recommendation: **yes**.
4. **Cadence.** Three house pieces a week, never more than one a day, until
   we have cost data. Recommendation: **yes**.
5. **Unlisted but reachable in phase 1.** Pre-launch pieces are public by
   direct link but not listed. Recommendation: **yes**; labels are on from
   day one.
6. **Disclosure first.** The AI disclosure line (item 14) is built before
   the Opinion tab is public. Recommendation: **yes**.
7. **Publish once.** An op-ed publishes as v1 only; later changes are
   labelled corrections. Recommendation: **yes**.
8. **No public score until receipts exist.** Forecasts show as "open,
   unaudited" until signed commitment receipts exist. Recommendation:
   **yes**.
9. **Human op-eds stay the human's.** The paper only lists a human piece;
   Texture never republishes or edits it. Recommendation: **yes**.
10. **Agents for humans are draft-only by default.** Direct submit needs a
    scoped key and an allowlisted author. Recommendation: **yes**.
11. **Invite only, you approve.** Until you say otherwise only invited
    accounts submit, you are first, and you approve every listing.
    Recommendation: **yes**.
12. **Optional forecasts for outside authors.** Allowed, recorded by the
    paper at admission time, on a per-author scoreboard that also shows
    pieces with none. Recommendation: **yes**.
13. **Two limits on the house.** It never endorses candidates and never
    writes about private individuals. Recommendation: **yes**.
14. **House replies are management's choice.** At most one a day, never on
    request, only to a screened piece citing two or more published stories.
    Recommendation: **yes**.
15. **No comments.** Replies are op-eds with a link; there are no comment
    boxes. Recommendation: **yes**.
