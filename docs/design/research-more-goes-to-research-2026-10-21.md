# More research goes to research first, and research reports as it goes

Date: 2026-10-21. Status: built (revised after the panel, section 8); the code follows section 8 where it differs from section 2. Mutation class:
red (a new owner-act door into a desk inbox; the public expansion door).
Problem: [research-more-slow-and-hidden-2026-10-21.md](../problems/research-more-slow-and-hidden-2026-10-21.md)
E4 to E6. Owner direction (this session): "Yes, it should skip texture.
But research should also be faster, and send iterative reports to texture,
not wait until after multiple searches. (This is our ordinary policy)".

## 1. Today

A reader's ask (`POST /api/platform/publications/expand`) makes the proxy
call the paper's `POST /api/texture/documents/{doc}/revise` as the story's
owner. The paper reopens a Texture work item, commits an owner revision
whose prompt is the expansion text, and wakes the story's Texture desk
twice (a direct `coagent_result` dispatch and the work item's outbox
wake) with intent `apply_owner_revision`. Texture reads the story and
casts research (27 to 30 s in the traces). Research works, sends one
Report at the end, and that Report wakes Texture, which writes.

Every research Report already wakes Texture, interim or final
(`store.RecordWakesReader` does not consult `Final`). Research does not
send interim packets in practice, although its overlay allows them.

## 2. Change

### 2.1 The ask becomes a Cast to the story's research desk

A new paper route, `POST /api/texture/documents/{doc}/research`, same
trust as `/revise` (host peer, `X-Authenticated-User` is the story's
owner), body `{client_request_id, prompt, expected_head_revision_id}`.
For a live story trajectory whose head matches, it commits in one
transaction:

1. A Cast directive record: inbox `research:<doc>`, sender inbox
   `texture:<doc>` (so research's Reports open and close the round in the
   story's own conversation, as Texture's Casts do), `EmitterKind owner`,
   `Provenance.AgentID "owner:<owner>"`, objective "A reader asked for more
   research on this story", body a question packet carrying the request id
   and the expansion text, record id and command id derived from
   `client_request_id` (replay returns the stored receipt).
2. The research desk's agent record when the story has none yet (the same
   mint `ApplyTextureTurn` does on Texture's first Cast to research).
3. A system Note to `texture:<doc>` from `research:<doc>`, carrying the
   request id and "expand the story with what research reports; do not
   rewrite it". A system record does not wake its reader, so Texture reads
   it with research's first Report.

No owner revision, no work item, no Texture wake. The research Cast wakes
research through the existing `inbox_record` outbox path.

Refusals: a head that moved (409 changed), a round to research already
open from another caster, a trajectory that is not live (409 busy), an
unknown or non-story document (404).

### 2.2 Research reports as it goes

Research overlay and first-turn prompt (all research, the ordinary
policy): after the first new source you have read and can cite, send an
interim packet with it at once; send another each time you have more;
the final Report closes the round. Each interim packet wakes Texture,
which revises with what it has; a later packet makes a later revision.

### 2.3 Texture expands, not rewrites

Texture's overlay: when its records hold a reader's request Note, expand
the story with the reported sources and keep its text, structure and
citations unless a source corrects them; with nothing new, change nothing.

### 2.4 The proxy

`HandlePlatformPublicationExpand` calls the new route instead of
`/revise`. The replay check reads the record by its id instead of the owner
revision. The fate line: `asked` until research's run starts, `accepted`
while it works, `revised` when a public version newer than the asked
version exists, "no new version" after the silence window, as now.

## 3. Authority

The owner of the story is the actor, as today; the proxy already speaks
as the owner to `/revise`. The new act is narrower than `/revise`: it
writes no revision and cannot change the story. Texture stays the sole
writer of canonical revisions. The record names its emitter as the owner,
not Texture, so the ledger does not claim Texture cast it.

The door admits only a live story trajectory and only a Cast to that
story's own research desk; the desk and inbox are derived from the
document, never from the request body.

## 4. Failure modes (tests written before the code)

1. The Cast is written but research is not woken (agent record missing,
   system emitter, wrong inbox).
2. Texture is woken by the ask itself (a non-system Note, a work item, a
   revision).
3. Research's final Report does not close the round, or lands in another
   conversation (wrong sender inbox).
4. A replayed request writes a second Cast or a second Note.
5. Another document's research desk, or a non-story document, can be
   targeted.
6. An interim packet does not wake Texture (regression guard).
7. The proxy reports `asked` for a refused request, or `already_asked`
   for a new one.
8. A Cast while Texture's own round with research is open breaks round
   rules (it must be refused as busy, not join silently).

## 5. Evidence

Deployed: a logged-out ask on a quiet story on staging; the trace shows
research's run starting with no Texture turn before it, at least one
interim packet before the final Report, and a published revision; the
time from ask to research start and to the first revision recorded.

## 6. Rollback

Git revert of the mission commits; the proxy goes back to `/revise`,
which is unchanged.

## 7. Heresy delta

- discovered: none new (E4 to E6 are in the problem record).
- introduced: a second owner-act door into the paper, narrower than
  `/revise`.
- repaired: on landing, E4 (the deciding Texture turn) and E5.

## 8. Panel and revision (2026-10-21)

Panel: Devin and gpt-6.1-sol, both "build with changes", high confidence;
gemini-3.8 had not answered when this was written. Findings adopted, all
verified against code (gpt-6.1-sol reproduced the first with the real
store):

- **An open round is joined, not refused.** `stepConversation` refuses a
  Cast into an open round only from another caster, and the ask and
  Texture's own Casts share the sender `texture:<doc>`. Section 2.1's
  "another caster" refusal never fires. Changed: the request is refused
  as busy whenever the research conversation has an open round, whoever
  cast it, and while the research desk has an active run.
- **Checks outside the commit race it.** `CommitInboxRecords` has no head
  or lifecycle condition. Changed: one store command,
  `CommitStoryResearchRequest`, does, in one transaction and in this
  order: replay by request id (before any current-state refusal, so a
  committed request replays after the head moves), live unarchived story
  binding, expected head, closed conversation, research agent mint, the
  Cast, its outbox wake.
- **Research is not given the story.** Changed: the paper writes the
  Cast's packet itself from the head it checked: headline, head revision
  id, the story text (bounded), and the URLs it already cites, then the
  fixed expansion instruction. The caller sends no prompt; the route takes
  only `client_request_id` and `expected_head_revision_id`. Story text is
  framed as evidence, not instruction.
- **The Note is unreliable.** A system Note shows to Texture as a
  "closure", and any other Texture turn can consume it before research
  reports. Dropped. Changed: Texture's frame is told, every turn, when the
  latest research round was opened by a reader's request and Texture has
  not committed a turn since that round closed: "a reader asked for more
  research (request id); expand the story with what research reports; keep
  its text, structure and citations unless a source corrects them". It is
  derived from the ledger at frame time, so it survives cursors, ordering
  and restarts.
- **A crash loses the ask and blocks a retry.** Under the restart rule a
  crash closes the round (boot closure, no wake), which is correct; what
  was wrong is that the fate stays "accepted" and a re-ask hits
  `already_asked`. Changed: the request's read model reports a round
  closed by a boot closure without a research Report as `interrupted`;
  the fate line says so, and the proxy admits one new attempt on the same
  version under an attempt-numbered request id.
- **The fate needs a read surface.** Changed: `GET
  /api/texture/documents/{doc}/research/{request_id}` returns the record
  id, commit time, round, whether it is open, research Reports in it, and
  the closing stop reason. `accepted` means durably admitted; `revised`
  means a public version newer than the asked one exists since the commit
  time (as today, "since", not "because").
- **Interim fanout.** Each interim Report wakes Texture; an active Texture
  run absorbs wakes, so the bound is one Texture turn per Report that lands
  between turns. Accepted, and stated.

Failure modes added to section 4: 9, a request while the research round
is open from Texture's own Cast is admitted; 10, replay after the head
moved is refused instead of replayed; 11, the expansion instruction is
missing from a Texture frame after another turn consumed the records;
12, a crash-closed request reads `accepted` or blocks a re-ask.

