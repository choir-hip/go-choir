# The three step 0 contracts for the Autopaper mission (2026-11-10)

Mutation class: green (a design; no runtime change). Rollback: revert.
Status: DRAFT by a Sonnet seat, grounded in the code at `origin/main`
9bbdfe68c. The orchestrator reviews and finalizes. Nothing here is settled
until it does. Section 8 lists every place a choice was made.

Binding inputs: handoff
[autopaper-next-mission-handoff-2026-11-09.md](autopaper-next-mission-handoff-2026-11-09.md)
sections 3, 4 and 6; goal file
[choir-autopaper-overnight-2026-11-09.md](../definitions/choir-autopaper-overnight-2026-11-09.md).
Citations are `file:line` at that commit. "Today" means that commit.

## 0. Three facts that shape all three contracts

1. **Two stores, no direct path.** A computer's Textures live on its own
   store (`choir.texture_revision`, writer Texture, owner edits admitted:
   `internal/objectgraph/registry.go:97-98`). Published things live on
   corpusd's graph (`choir.publication`, `choir.publication_version`, owner
   corpusd: `registry.go:194-195`). A computer reaches corpusd as a bound
   guest on a few narrow routes (`internal/platform/handlers.go:348-358`,
   `bindTapCaller`). No computer-to-computer route for Textures or records was found (`internal/tapefollow` and `internal/bundle` follow a tape for sync, not for records between users; not read in depth).
2. **A published revision is a graph object, not a local id.** Publication id
   = `pub-` + hash of (owner id, source doc id)
   (`internal/platform/publication_versions.go:279-286`). Version n is a
   `choir.publication_version` object (`publication_graph.go:129-137`) with
   `versionMetadata` (`publication_versions.go:385-403`): source revision id
   and hash, manifest hash, `supersedes` edge, platform signatures. It is
   immutable. A local `revision_id` means nothing on another computer, so
   every cross-computer reference below names a version object, never a
   local revision.
3. **The platform is the only courier.** Everything that must cross
   (parent links, op-eds, verdicts, notices) crosses as a corpusd object that
   the receiving computer polls. No push to a guest.

## 1. Contract 1: the revise call

### 1.1 What exists today

| Fact | Where |
|---|---|
| The owner revise: `POST /api/texture/documents/{id}/revise`; also the prompt bar's focused submit on an owned document | `internal/textureowner/texture_agent_revision.go:56`, `prompt_bar.go:189-194,233-248` |
| It does not edit text. It commits an `AuthorUser` revision that carries the head content forward, with the directive in metadata (`input_origin=user_prompt`, `owner_prompt`), and wakes the desk, which then writes its own `AuthorAppAgent` revision | `texture_agent_revision.go:201-222,249-253`; `texture_cell_author.go:350-361` |
| A direct body edit is a different route: `POST /documents/{id}/revisions` (idempotency key, expected lifecycle version, parent revision id required) | `internal/textureowner/texture.go:1075-1121` |
| Restore exists only for unbound documents | `texture.go:1857-1885` |
| Base check: the revise must name the current head, else 409 "document head changed" | `texture_agent_revision.go:166-172,198-200`; store re-checks `store/texture.go:461-469` |
| Parent: exactly one. `Revision.ParentRevisionID` is a single string; the lifecycle commit overwrites it with the expected head | `internal/types/texture.go:166-168`; `internal/store/lifecycle.go:4442` |
| Multi-parent: none. No merge, no fork, no comment, no `forked_from` anywhere (grep of `internal/` finds only unrelated hits) | n/a |
| Revision hash covers parent hash, body, source entities and the `Provenance` blob | `internal/types/texture_revision_hash.go:67-100`; `store/texture.go:480` |
| `Provenance` is system-attributed, canonical JSON, and is inside that hash; its fields are `omitempty`, so a new optional field leaves old hashes unchanged | `internal/types/texture_provenance.go:25-42` |
| Who writes: Texture is the document custodian (`deskmanifest.DocumentCustodianName`, `document_custodian.go:14`); the owner's own edit path is a second writer class (`OwnerEdits`); a desk calling in-process is refused (`prompt_bar.go:227-237`) | `registry.go:97-98`, `objectgraph/writer.go:332` |
| Idempotency: revision id = SHA1(owner, computer, trajectory, client request id); command id `owner-revise:<id>`; replay returns the same receipt | `texture_agent_revision.go:207,226-231,255-262` |
| Request into the desk: a Cast directive record to `texture:<doc>` (`DeliverOwnerRequest`), emitter owner, label `owner_private:prompt_bar`, deterministic id `owner-request:<run>` | `internal/agentcore/prompt_router.go:372-417`; record model `internal/types/commitment.go:252-409` |
| Restart: a crash boot closes every open round with a system Report, fate `interrupted_by_restart`, which wakes no one | `internal/agentcore/inbox_records.go:1324-1360`; `types/commitment.go:535-560` |
| "Published revision": a `choir.publication_version` object, state `published`; link form `choir:pub/<slug>/v<n>` is accepted as an external citation target | `service.go:980-995`, `publication_versions.go:293-300` |
| Untrusted text label `world_untrusted` exists | `internal/types/label*.go:25` |

Consequence: the "edit" and "comment" cases already exist as two different
routes, the "take a version" and "merge" cases do not exist, and the revision
model has one parent. The contract adds a parent-link list and one entry
point. It does not add a second writer.

### 1.2 Specification

**Entry point.** One Go function in `internal/textureowner`, the only caller of
the canonical write for these cases:

`reviseDocument(ctx, ReviseRequest) (ReviseReceipt, error)`

`ReviseRequest` fields:

| Field | Meaning | Source |
|---|---|---|
| `doc_id` | the document being revised | path |
| `base_revision_id` | the revision responded to; empty only when creating a document from sources | body |
| `author` | owner id and label | never the body: derived from the authenticated session (`authenticateUser`) |
| `text` | the instruction or comment, plain text, at most 16 KiB | body |
| `body` | owner-only direct body edit (`body_doc` JSON); mutually exclusive with `sources` | body (decision D1) |
| `sources[]` | each `{publication_version_id, manifest_hash}` (canonical id of a `choir.publication_version`) | body, verified, see below |
| `client_request_id` | idempotency key; required | body or `Idempotency-Key` header |

`/revise`, the prompt bar's focused submit and the editor's body save all call
it. The old `ExpectedHeadRevisionID` field maps to `base_revision_id`.

**The cases.**

| text | body | sources | Case | Result |
|---|---|---|---|---|
| yes | no | none | comment or instruction | today's behavior (carry head, wake desk), plus parent links; owner only on an owned document |
| no | yes | none | edit | the `/revisions` path, now routed here; owner only; base must be the head |
| no | no | exactly 1 | take | no model call. One transaction: a revision whose body is the source's published body, author = requester, parent links `[base, source]` |
| yes | no | 1 or more | merge | request revision committed (carries head, links recorded), desk woken; the desk's revision inherits the links |
| any | any | any, on a document the caller does not own | refused at the owner's Texture (404 as today) | the client converts it to a fork |

Refused combinations: no text, no body and no source (400); `body` with
`sources` (400); more than 8 sources (400); text over the cap (413).

**Fork and comment on a document you do not own.** A comment on someone
else's document is a fork on the commenter's own computer, never a write to
the other computer. A fork is `reviseDocument` on a new document with empty
base and `sources=[the published version]`:
- with no text: a take (the fork is a verbatim copy);
- with text: a merge whose only source is the version. The desk writes the
  comment's answer into the fork, and the fork "updates and explains itself".
The fork's first revision carries `parent_links=[{role:source, ...}]` and the
document records `forked_from` (the same link) in its first revision's
provenance. A fork is private until published.

**Parent-link storage (decision D3).** Add `ParentLinks []ParentLink` to
`types.Provenance` (`texture_provenance.go:25`), `omitempty`, sorted by
(role, ref) in `CanonicalJSON`. It is inside the rev2 hash, so a link cannot
be altered after the fact, and nothing in the revision schema changes.

`ParentLink` fields:
- `role`: `base` or `source`;
- `revision_id`: local, for `base` only;
- `publication_version_id`: canonical id, for `source` only;
- `manifest_hash`: pins the exact content of a source;
- `route_path`: `/pub/texture/<slug>/v<n>`, for display;
- `author_label`: the public handle of the source's author at the time.
The chain parent stays `ParentRevisionID` = the head (the store forces this,
`lifecycle.go:4442`). `base` is recorded as a link too, so "responds to v3
of v7" is data.

**Validation (all before any write; any failure writes nothing).**
1. Each source resolves, through the new bound-guest read route
   `GET /internal/platform/publications/versions/{canonical_id}` (patterned on
   `handlers.go:348-358`), to a `choir.publication_version` that exists, is
   not tombstoned, and whose publication state is `published`. Its
   `manifest_hash` equals the request's. A source from a deleted document is
   fine: the version is immutable (fact 2). An unpublished or unknown source
   is 422. The platform being unreachable is 503 and nothing commits.
2. The source's author is read from the graph object's `owner_id`, never from
   the request.
3. Base: `base_revision_id` must be a revision of this document. For the
   edit and take cases it must be the current head, else 409 with the current
   head id in the body (no silent rebase; same as `texture_agent_revision.go:171`).
   For comment and merge an older base is accepted (decision D2): the chain
   parent is the head, and the older base is recorded as the `base` link and
   shown to the desk as the revision responded to.
4. A source may not be the caller's own pending fork or any revision that
   is not a published version (so no unpublished text can be pulled across
   computers by naming a local id).
5. Author and owner come from the session. A `computer_id` or `owner_id` in
   the body is an unknown field and is refused (`DisallowUnknownFields`, as
   `prompt_bar.go:108`).

**The no-model fast path.** Take = validation, fetch the source body from the
platform by version id (hash-checked against `manifest_hash`), strip embeds
that are private to the source computer (the publish path already refuses
them, `platform_publish.go:~296`), build the revision, and commit through
`CommitLifecycleArtifactHeadWithSourceGraph` once. No run, no cast, no wake.
It then emits the document revision event (`emitTextureDocumentRevisionEvent`)
so the UI updates. Author kind is `user` (decision D4).

**The merge path.** Two commits, as today:
1. The request revision (AuthorUser, carries head, `owner_prompt` = text,
   `ParentLinks` = base + sources). The wake is the existing
   `dispatchTextureRevisionWake`.
2. The desk's revision. The host stamps the same `ParentLinks` into its
   `Provenance` (the desk cannot set them: `buildStructuredAppagentRevisionProvenance`
   reads them from the trigger revision). The desk's frame shows each source as
   a data block labelled `world_untrusted` with the source's author label and
   version, fetched by the host at request time and pinned by hash. The
   desk's prose must say what it took and from whom. Links are metadata, the
   prose is the desk's.

**Idempotency.** The revision id stays the SHA1 of (owner, computer,
trajectory, client request id). The command digest now covers the provenance,
so the same request id with different sources or text is
`ErrLifecycleCommandConflict` (409), and the same request replays to the
same receipt with no second wake (`texture_agent_revision.go:249`).

**Restart.** Take is one transaction: it either committed or it did not. A
merge's request revision is canonical history and stays; the open round on
`texture:<doc>` is closed by the crash boot with `interrupted_by_restart`
(`inbox_records.go:1324`), the document shows that fate, and the owner's next
revise reopens the work (`textureWorkSettledByDesk`, `:364-401`). Nothing
resumes. A planned-restart resume is out of scope (`CLAUDE.md`: no boot-time
resumers without the marker).

**Credit and citation.** Computed, never stored separately: for a revision,
credit goes to the author of every `source` link (resolved on the platform)
and to the base's author. The published version's `versionMetadata` gains
`parents: [{role, publication_version_id}]` (section 3.2), so a reader can see
"built from" without opening the author's computer.

**Who may send it.** The owner of the document, through the authenticated
owner route only. An in-process desk call is refused as today
(`prompt_bar.go:234-236`). Management, research and engineering never call it:
a lane that needs a Texture change sends a record and Texture writes.

### 1.3 How each lane uses the one call

| Lane | Call |
|---|---|
| B sections | the house list is a published Texture; a reader's copy is a take of it, and their edits are `body` edits. Section questions are not written through revise. |
| C1 | edit (`body`), comment (`text`), fork (new doc, `sources=[v]`), progressive update (the merge path with a research Report as the trigger) |
| C2 | publishing an op-ed is a publish of a fork (section 3); the paper's response is a revise on the story with `sources=[op-ed version]` and a text from the desk's own judgment, emitted by the story's desk, not by a user route |
| D prompt bar | classify input to text/body/none; shows `base` and the case chosen; re-send under another case is a new request id |
| G style | a personal style guide is a take of the house guide plus `body` edits; read-time only |
| H ledger | H1 defines the record types; an accepted algorithm² proposal is a revise with the proposal as a source (the proposal must be a published version or the revise is refused: decision D9) |

The desk's own response to an op-ed (C2) is the one writer path that is not an
owner route. It is a desk turn on the story's document whose trigger
revision/record carries the op-ed link; the host stamps the link into the
revision's provenance. It does not pass through `reviseDocument`'s owner
checks, and H1 must say so (D6).

### 1.4 Failures and tests

| # | Failure | Covered by |
|---|---|---|
| R1 | Forged parent link: a body names a `publication_version_id` that does not exist, or names a real one with a wrong `manifest_hash` | `TestReviseRefusesUnknownOrMismatchedSource` (unit on validation); d@e drill with a hand-built request |
| R2 | Source is unpublished: a local revision id or a draft's version | `TestReviseRefusesLocalRevisionAsSource` |
| R3 | Source's document was deleted by its author: the published version must still work | d@e drill: publish, archive the source doc, take the version |
| R4 | Stale base on an edit or take: refused with the head id; no write | `TestReviseStaleBaseOnEditIs409` |
| R5 | Older base on a comment: accepted, chain parent is the head, `base` link names the older revision | `TestReviseOlderBaseCommentRecordsBaseLink` |
| R6 | Replay of the same request id: same receipt, one revision, one wake | `TestReviseReplayIsOneRevisionOneWake` |
| R7 | Same request id with different sources: 409, no second revision | `TestReviseRequestIdWithOtherSourcesConflicts` |
| R8 | Restart mid-merge: round closed `interrupted_by_restart`, no resume, no second desk run, request revision stays | restart test beside `source_schedule_restart_test.go` pattern, and a d@e kill drill |
| R9 | Platform unreachable during source fetch: 503, nothing committed | `TestReviseSourceFetchDownWritesNothing` |
| R10 | A non-owner revises someone else's document: 404 (same as missing), no leak that it exists | `TestReviseOtherOwnersDocumentIs404` |
| R11 | A desk calls revise in-process: 403 | existing guard, extend `prompt_bar_route_test.go` |
| R12 | A source whose text tries to instruct the desk ("ignore the user") | merge-path prompt test: source arrives as `world_untrusted`, never in the instruction slot; d@e drill with a hostile op-ed |
| R13 | Cross-computer identity confusion: a body carries another `owner_id` or `computer_id` | unknown-field refusal test |
| R14 | Take of a source with private embeds or capsule evidence | `TestTakeRefusesPrivateEmbeds` (reuse `PrivateEmbedsInBody`) |
| R15 | `body` plus `sources` together | `TestReviseRefusesBodyWithSources` |
| R16 | Hash chain: a stored revision's `ParentLinks` altered after commit | `TestRevisionHashCoversParentLinks` |
| R17 | The old route still works unchanged for existing clients (no `sources`) | the existing `texture_editor_save_contract_test.go` and `prompt_bar_*` suites stay green |

## 2. Mutation class and review for contract 1

Red: Texture canonical writes (protected surface). Additive (optional field,
new validation), one-act rollback (revert; old revisions have no links). Review:
one Devin round (handoff section 2). Lane C1 owns it. The `Provenance` field
and store change touch `internal/types` and `internal/store`: run
`check-v1-writer-purity` and `check-decode-roots`.

## 3. Contract 2: publish and adopt

### 3.1 What exists today

| Fact | Where |
|---|---|
| Three publish modes, set by the route, never the body: `auto` (computer's own story), `owner`, `opinion` | `internal/platform/types.go:50-57`; `internal/proxy/platform_publish.go:233,262` |
| An op-ed route exists and is half-built: `POST /api/platform/texture/opinions`, browser session only, byline `writer-<hash>`, no parent or `responds_to` field, 5 new pieces a day | `platform_publish.go:153-230`; `platform/publication_opinion.go:24-26,103-130` |
| A runtime kill switch `CORPUSD_OPINION_LISTING` gates it (on in `nix/node-b.nix:1126`); this violates the no-switches rule | `publication_opinion.go:52,72` |
| Publish is proxy-driven: the proxy reads the document and revision from the owner's computer and posts to corpusd; corpusd verifies nothing about parents (there are none) | `platform_publish.go:262-360`; `service.go:226-376` |
| Publication is idempotent per (owner, doc, version n): replay returns the existing version | `service.go:313-323` |
| Versions are signed with the platform key; a version has a `supersedes` edge to its predecessor | `service.go:469-475`; `publication_graph.go:178-181` |
| Mode and owner are fixed by the first version; a story stays a story | `service.go:345-372`, `ErrOpinionModeConflict` |
| The only link from a publication to another publication is a `choir:pub/` citation, written as a `references` edge; nothing produces or renders it | `service.go:980-995`; design `docs/reports/op-ed-section-design-2026-10-20.md` §3 |
| Published search admits a bound guest (244325257); a reader's computer can find stories, not op-eds by parent | `handlers.go:348-358` |
| A proposal path records "an edit proposed to a published story" with `delivery_state=recorded_for_author`; nothing delivers it (no consumer outside `internal/platform`) | `service.go:708-913`; grep `proposal-deliveries` |
| Durable no-model polling exists: a source schedule is a chain of `source_poll_due` wakes with a persisted generation; a crash closes only the poll in flight and the next tick is new work | `internal/agentcore/source_schedule.go:25-45` |
| A system arrival record to management from a poll exists (`capture_arrival.go:56-100`); system records do not wake, except an enumerated list | `store/management_tasks.go:76-106`; `store/inbox.go:277` |
| Per-computer Jev rate bucket keyed (computer id, scope); scopes today: default and `intake_labels` | `internal/gateway/jev.go:43-52,337-346` |
| The section scorer is a Node B host job (`corpusd front-importance`), host credential, computer id `host:front-importance`, scores in one JSON file; no per-bucket limit in `host_judgment.go` (grep) | `internal/frontimportance/scorer.go:1-60,95-130` |

### 3.2 Specification: what a published op-ed is

**It is a publication version with a verified `responds_to`.** No new
publication kind. Existing mode `opinion` stays the mode; the route sets it.
Add to `versionMetadata` (and mirror on the publication object's metadata for
the first version, fixed like mode and owner):

- `responds_to`: `{publication_version_id, route_path, publication_id}` of the
  story version the op-ed answers. Set by the proxy from the revision's
  provenance (the fork's first-revision `source` link to an Autopaper story),
  never from the body. Absent for a plain opinion.
- `parents`: the full `ParentLinks` of the published revision, as
  `[{role, publication_version_id}]` (sources only; the local `base` is
  dropped).
- `oped_state`: `pending`, `accepted`, `rejected` (the gate's verdict, section
  3.4). Not part of the immutable version; it lives on the verdict object.

**corpusd verifies on publish.** For every `source` parent and for
`responds_to`, corpusd reads the target object and refuses (409) if it does
not exist, is not a `choir.publication_version` in a `published` publication,
or its manifest hash differs. For an op-ed, `responds_to` must name a version
of an `auto`-mode publication. A reader publishing an op-ed on a story
revision that does not exist is therefore refused before any object is
written.

**New corpusd kinds** (register in `internal/objectgraph/registry.go`, owner
corpusd, writer corpusd; registry hygiene; `TestEveryRegisteredKindHasAWriter`):
- `choir.oped_verdict`: one per (op-ed version, gate run). Written by the
  courier route in 3.4.
- `choir.changelog_notice`: contract 3.

**Remove the kill switch** `CORPUSD_OPINION_LISTING` and
`SetOpinionListing` as part of lane C2 (D7). Keep the daily limit; it is a
correctness bound, not a feature switch (D7).

**Relation to the paper's response.** The paper's response is a new version of
the story. Its published `parents` include `{role:source,
publication_version_id: <op-ed version>}`. That is the citation. corpusd
also writes a `references` edge from the new version to the op-ed version
(existing mechanism, `service.go:529-551`).

### 3.3 How the paper's computer learns of an op-ed

Options, cheapest first:

| Option | Cost | Cons |
|---|---|---|
| A. Poll a corpusd route on a durable schedule | one indexed range read per poll; reuses `source_schedule` (durable, no model call, crash-safe) | latency up to the interval (default 15 min, floor 1 min, `source_schedule.go:34-36`) |
| B. corpusd pushes to the paper's computer | immediate | the computer may be down or restarting; needs a new inbound route on a guest; fate-shares the publish with the paper's liveness |
| C. Subscribe via the proxy live socket | immediate | the proxy has no durable subscription; lost on restart |

Recommended: A (decision D5). Spec:
- New corpusd route `GET /internal/platform/opeds?after=<cursor>&limit=`.
  Admitted to a bound guest only; it returns op-eds whose `responds_to`
  publication is owned by the caller's bound owner (owner read from the
  binding, never a parameter). Returns `{items:[{op-ed version id, route,
  author_label, responds_to, published_at, text_ref}], next_cursor}`. The
  cursor is the graph's creation order (`created_at`, canonical id), so replay
  of a page yields the same items.
- A new standing schedule kind `oped_intake` on the paper's computer, armed at
  boot like other schedules (`recoverSourceSchedulesAtBoot`). The cursor is on
  the schedule record. A crash closes the poll in flight and the next tick
  starts from the saved cursor, so replayed items are deduplicated by the
  deterministic record id below.
- The op-ed text is fetched with the version read route (1.2) and is
  `world_untrusted`. It never enters an instruction slot.

**Adoption on the paper's tape.** For each new op-ed the poller commits one
silent system record, `oped_arrival`, to `management:<owner>`: id
`oped-arrival:<op-ed version id>` (so a replay mints nothing), body = the
route, author label, `responds_to`, text hash. System, so it wakes no one
(`management_tasks.go:76`).

### 3.4 The gate and where its verdict is recorded

The gate runs on the paper's computer (decision D8), as host runtime code
that calls Jev through the computer's own gateway bucket, like
`JudgeRevisionSupport` (`support_judge.go:96`), off the commit path. It is not
a model run of management and not a desk. Jev is reached with a new rate
scope `judgments:oped_gate` so it does not starve intake labels.

Order per op-ed: poll, `oped_arrival`, gate Jev call, verdict record, then (if
accepted) wake.

**Verdict record on the paper's tape (authoritative).** A system Report
`oped_verdict:<op-ed version id>:<gate run n>` to `management:<owner>` with:
`verdict` (`accepted`, `rejected_abusive`, `rejected_redundant`,
`rejected_unclear`, `rejected_off_topic`), the full probability distribution
per question, the criteria document's revision id and hash, model, cost,
latency. Rejected verdicts are records too (handoff 4.C). When the desk is
woken it may read the rejected ones for that story as untrusted text.

**Courier copy.** The paper's computer then posts the verdict to
`POST /internal/platform/oped-verdicts` (bound guest; corpusd checks the
op-ed's `responds_to` publication is owned by the caller's owner, else 403).
corpusd writes `choir.oped_verdict` with the tape record id and a hash of the
record body so the two can be compared. This copy makes the verdict visible to
the author and to the listing. The paper's tape remains the single authority;
the corpusd copy is derived.

**Reader's tape.** corpusd derives the author's changelog notice from the
verdict (contract 3). The reader's computer writes its own record when it
polls its notice feed (adoption). So the verdict lives on both tapes, as two
records with the same `verdict_hash`.

**Listing rules.** `accepted`: listed on the story and on the author's page.
`rejected_abusive`: not listed on the story; visible to its author. Other
rejects: listed on the author's own page only. All three states are readable by
the desk when it is next woken.

**Waking the desk.** On `accepted`, or on the reject-backoff rule, a record
that wakes `texture:<story doc>` is needed. Today a system record does not wake
(`inbox.go:277`), so there are two ways (D6):
- emit a Cast from `management:<owner>` (an agent record wakes);
- add `oped_wake` to the enumerated waking system records in
  `RecordWakesReader` (`management_tasks.go:76-106`), a store change in a
  red surface.
Recommended: the first, since it needs no store change; it makes management
an intermediary, which the goal file calls a hypothesis. The wake state
(unread rejects by distinct author, the doubling threshold and gap, coalescing)
is a fold over `oped_arrival`, `oped_verdict` and `oped_wake_decision` records
on the tape per story. H1 defines those types, C2 emits them.

**Gate unavailable.** If the Jev call fails or the gateway is held, the op-ed
stays `pending` (no verdict record is invented), the poll cursor still
advances, and a retry runs on the next poll up to 3 times over 24 hours, after
which a `gate_unavailable` record is written and the op-ed is neither listed on
the story nor used to wake the desk. Never accept on failure.

### 3.5 How the reader's computer learns the paper responded

The paper's response version is published with the op-ed in its `parents`.
corpusd's publish path (contract 3) writes a notice for the op-ed's author.
The reader's computer polls `GET /internal/platform/notices?after=` (section 5.2) and writes its record. The reader's document also gains a derived
view "responded to by the paper at v<n>" by reading the notice; there is no
back-write into the reader's Texture.

### 3.6 Which computer runs a reader's section questions, and where the cache lives

This is the handoff's open item. One Jev call carries one state and many
questions (`system1.go`), so one story costs one call whatever the number of
sections. Cost is stories times lists, not sections.

| Option | Runs on | Cache | Rate bucket consequence | Cons |
|---|---|---|---|---|
| A. All on the host | Node B job, host credential | corpusd, keyed (story version canonical id, hash of normalized question text, judge version), shared by every reader | the host route has no per-bucket limiter (`host_judgment.go`), so the only limit is the organization's provider limit; lane A must size one pool and a per-computer admission quota | the host sees every reader's question text (their interests); readers' lists cost the host; a reader's burst can starve the house list; needs a new bound-guest write route |
| B. All on the reader's computer | the reader's computer, own gateway bucket, new scope `judgments:sections` | the reader's own store (derived, rebuildable) | per-computer bucket (`jev.go:337-346`); lane A must raise it for this scope to about 15 calls a second for the re-sort time in handoff 4.B; 100 visible stories = 100 calls | no sharing: cost scales with readers times lists |
| C. House list on the host; every other list on the reader's computer | both | house: corpusd; others: reader's store | house list uses the existing host job; others use B's bucket | no sharing of popular custom lists (deferred) |

Recommended: C (D10). Reasons: private lists stay private; a guest cannot write
into a shared cache (a forged "yes" would poison every reader); the house list
already runs this way (`scorer.go:1-60`). To get the sharing the handoff
wants, a published section list (a consenting reader's copy) can later be
scored on the host like the house list; that is pass 2.

Cache key everywhere: (story version canonical id, SHA-256 of the normalized
question text, judge version). A new section runs only its question. The
visible page is computed first. The section document hash is not in the key
(so editing a list reuses unchanged questions); thresholds live in the
document and apply at read time to stored distributions.

### 3.7 Failures and tests

| # | Failure | Covered by |
|---|---|---|
| P1 | A reader publishes an op-ed naming a story version that does not exist | `TestPublishOpedRefusesMissingRespondsTo` (corpusd); d@e drill |
| P2 | `responds_to` names a version of a non-story publication (owner mode) | `TestPublishOpedRespondsToMustBeAutoMode` |
| P3 | A forged `parents` list: real version ids that the revision's own provenance does not carry | proxy reads parents from the computer's revision, not the body; `TestPublishIgnoresBodyParents` |
| P4 | Replayed adoption: the same page polled twice, or after a crash | `TestOpedIntakeReplayMintsOneArrival` (deterministic id) and crash test on the schedule |
| P5 | Gate unavailable: Jev down, held or rate limited | `TestOpedGateUnavailableStaysPending`; never accepted by default |
| P6 | Gate gives a verdict twice (retry): one verdict per run, the later run supersedes | `TestOpedVerdictRunIdempotent` |
| P7 | Another user posts a verdict for someone else's story | `TestOpedVerdictRouteChecksRespondsToOwner` (403) |
| P8 | Cross-computer identity confusion: the poll route returns another owner's op-eds because a parameter named an owner | route takes no owner parameter; `TestOpedsRouteScopedToBoundOwner` |
| P9 | Op-ed text attacks the desk or the gate | op-ed enters Jev and the desk as `world_untrusted` data; d@e drill with a hostile op-ed; gate test with injection text |
| P10 | A rejected op-ed wakes the desk | `TestRejectedOpedDoesNotWakeDesk` |
| P11 | Reject-wake backoff: threshold and gap double after a no-change wake and reset after a change | fold unit test over fixture records |
| P12 | Restart mid-gate: the poll in flight is closed `interrupted by a restart`; the op-ed stays `pending`; the next tick re-gates once | `TestOpedGateCrashClosesAndRegatesOnce` |
| P13 | Two op-eds arrive while the desk is working: they join that wake | coalescing test |
| P14 | The same user publishes the same fork twice (version replay) | existing replay, `service.go:313` |
| P15 | A cached section answer is served for a changed story | key includes the version canonical id; `TestSectionCacheKeyedByVersion` |
| P16 | A guest writes a section answer into the shared cache | there is no such route (option C); route-registry test |
| P17 | Opinion pieces leak into the news front or related walk | existing F1 tests (`oped-not-connected` F1) stay green |
| P18 | Daily limit blocks a user whose first op-ed was rejected by the gate | limit counts publications, not verdicts; `TestOpinionDailyLimitCountsPublications` |

## 4. Mutation class and review for contract 2

Red: publication and adoption on the object graph (protected), a new corpusd
kind, a new auth admission (bound guest read/write routes), and an auth/store
surface. Silent-failure potential (a forged link or verdict is invisible until
shown). Review: two seats, Devin and a Sonnet seat (CLAUDE.md tier 2; the
goal file says one Devin round for C1, C2 and H1 and two seats where a defect
would be silent or irreversible: this is the second kind). New routes also run
`scripts/check-guest-security-epoch` (see
`docs/problems/published-search-refuses-every-guest-2026-11-09.md` for the
precedent). Rollback: revert; published objects and verdicts are tape records
and are not unwritten. Prove on d@e before the paper.

## 5. Contract 3: the changelog events

### 5.1 What exists today

| Fact | Where |
|---|---|
| No changelog or activity feed exists. Grep for `changelog` finds docs only | repo grep |
| Notifications are email through `maild` (`HandleNotificationAPI` forwards to it); risk alerts go to the owner's address of record | `internal/proxy/notifications.go:8-10`; `internal/maild/notifications.go` |
| Live UI events: the in-process bus and product events | `internal/events/bus.go`; `internal/agentcore/product_events.go`; `live_ws.go` |
| Document revision events | `emitTextureDocumentRevisionEvent` (`texture_agent_revision.go:251`) |
| The computer tape's event kinds are a closed set; adding one is a reducer change | `internal/computerevent/event.go:33-90,177` |
| Silent system records on a computer's ledger are the established way to carry facts that wake nobody (round closure, arrival) | `types/commitment.go:535-560`; `capture_arrival.go:56` |
| Cross-computer delivery to an author: none. The proposal path records `recorded_for_author` and stops | `service.go:713-913` |
| Custodian-lease reads for a package desk exist (E reads the tape through one) | handoff 4.E; `internal/deskmanifest` |

### 5.2 Specification

**One rule.** When a published version has a `parents` entry whose target
publication is owned by an owner other than the publisher, corpusd writes one
notice to that owner. It runs inside the publish transaction in
`Service.PublishTexture` (`service.go:226`), after parent verification. It is
not model work.

`relation` is derived from the child's publication mode: `opinion` with
`responds_to` = the parent: `fork_published` if the parent is another
op-ed, `oped_on_your_story` if the parent is the target's story; `auto`:
`paper_responded`; `owner`: `taken_into`.

**Event record `choir.changelog_notice`** (corpusd kind, immutable, one per
event):

| Field | Meaning |
|---|---|
| `event_id` | SHA-256 of (kind, recipient owner, child version id, parent version id); the idempotency key |
| `recipient_owner_id` | who is told |
| `kind` | closed enum, below |
| `level` | `major` or `minor`, set by the kind table, never by a model |
| `occurred_at` | RFC 3339 |
| `actor_label` | public handle of the other party; never an email |
| `subject` | `{publication_version_id, route_path}` of the child, and of the parent when there is one |
| `summary` | at most 200 characters, platform template, not model prose |
| `reason` | verdict reason text for `oped_verdict`, marked untrusted (Jev output) |
| `verdict_hash` | for verdicts: hash of the tape record on the paper's tape |
| `origin` | `corpusd` for platform-derived events |

Notices contain no content of the child, only references. A recipient reads
the child through the public route.

**Event kinds** (closed enum; adding one is a contract change):

| Kind | Level | Derived where | Told |
|---|---|---|---|
| `parent_used` (relation `fork_published`, `paper_responded`, `taken_into`, `oped_on_your_story`) | major | corpusd, at publish | the parent's author |
| `oped_verdict` | major | corpusd, from the courier verdict | the op-ed's author, with reason |
| `update_finished` | major | the computer's own tape | the document's owner |
| `update_interrupted` (fate `interrupted by a restart`) | major | the computer's own tape | the document's owner |
| `update_started`, `update_progress`, `revision_committed` | minor | the computer's own tape | the document's owner |
| `wake_decision` (the desk was or was not woken, and why) | minor | the paper's tape | the paper's owner |
| `oped_arrival` | minor | the paper's tape | the paper's owner |

`update_*` events are not cross-computer. They are derived on the owning
computer from the records and lifecycle events it already has: the open round
and its final Report, or the closure.

**How an event refers to the tape.** Every event, local or adopted, is
written to the recipient computer's ledger as a silent system record with a
`Changelog` body (an additive optional field on `CommitmentRecord`, like
`Captures` and `Job`, `commitment.go:252-409`), emitter `system`, inbox
`changelog:<owner>`, id `changelog:<event_id>`. The record is the local
anchor. It carries:
- `tape_ref`: `{computer_id, record_id, inbox_seq}` of the local record, and
  for local events the `{doc_id, revision_id, conversation_id, round}` it
  summarizes;
- `span`: `{from_seq, to_seq}` in the computer's event chain
  (`computerevent.Event.Sequence`) covering the work the event summarizes, so
  zooming from one major event to the whole tape is a range read;
- `remote_ref`: the corpusd notice object id, for adopted events.

**Level and zoom.** The UI shows `major` by default. Zoom in means: include
`minor` events, then expand any event's `span` to the raw records in range.
The level is a property of the kind (table above); a lane that adds a kind
adds its row.

**Delivery to the author's computer.** The recipient's computers poll
`GET /internal/platform/notices?after=<cursor>&limit=`, bound guest, scoped to
the bound owner (no owner parameter). A durable `notice_intake` schedule (same
substrate as `oped_intake`) commits one `changelog:<event_id>` record per
notice. A replay mints nothing (deterministic id). The changelog app reads
the ledger through its custodian lease; it does not read corpusd.

**Silent by construction.** A changelog record is a system record that is not
on the waking list (`RecordWakesReader`), so it never starts a run. A notice
is never produced for a notice. No notice is produced when the parent's
author equals the publisher.

### 5.3 Why derive at publication on the publisher's side

Only corpusd sees both authors and has verified the parent link (contract 2).
The reader's computer cannot know a stranger forked its op-ed. A poll of the
feed is the whole delivery. The alternative (each computer scanning the graph
for children of its versions) costs a query per version per poll and exposes
who is watching what.

### 5.4 Failures and tests

| # | Failure | Covered by |
|---|---|---|
| C1 | Notification loop between two users: A forks B's op-ed, B forks A's fork, and so on | notices never wake a desk or create a revision; only a human publish creates a child; `TestNoticeDoesNotWake`; two-identity d@e drill |
| C2 | Self notice: publisher is the parent's author | `TestNoNoticeForOwnParent` |
| C3 | Duplicate notice from replayed publish | `event_id` deterministic; `TestNoticeIdempotentOnReplay` |
| C4 | Forged parent link to spam a stranger | link verified (P1); one notice per (child, parent) only; `TestSpamLinkOneNoticePerChild`; residual: a true-but-irrelevant link still notifies (D11) |
| C5 | Cross-computer identity confusion: owner with two computers receives the same notice twice | per-computer cursor, deterministic record id per tape; each tape gets one; documented, not a bug (D12) |
| C6 | Notice carries content or an email | schema has no body field; `TestNoticeHasNoEmail` |
| C7 | Verdict reason carries injection text | `reason` marked untrusted, rendered as text, never given to a desk; render test |
| C8 | `update_interrupted` missing after a crash | boot closure writes the event; `TestCrashBootWritesUpdateInterrupted` and a d@e kill drill |
| C9 | `update_finished` written for an interrupted update | fold takes the final Report's stop reason; `TestFinishedNotWrittenOnClosure` |
| C10 | Notice feed unavailable | poll retries on the next tick; changelog shows last sync time; `TestNoticeIntakeSurvivesPlatformDown` |
| C11 | Replayed adoption after crash mid-page | cursor advances after commit; `TestNoticeIntakeCrashReplay` |
| C12 | A recipient reads another owner's notices | route scoped to binding; `TestNoticesRouteScopedToBoundOwner` |
| C13 | A new event kind added without a level | closed enum compile check and `TestEveryKindHasLevel` |
| C14 | Zoom span points past the tape | `TestSpanWithinChain` |
| C15 | The changelog app shows an event no desk produced | acceptance rule from the goal file: every event traces to a `tape_ref` or `remote_ref`; d@e check |

## 6. Mutation class and review for contract 3

Red: it adds a corpusd kind, a bound-guest read route, a new record body on
`CommitmentRecord` (store schema decode) and a boot-path closure write. Silent
failure possible (a missing event looks like no event). Review: two seats for
the corpusd derivation and the record body (tier 2); the app (lane E) follows
standing tiers. H1 owns the record types, C1 and C2 emit them, E reads them.

## 7. Lane to files, with the contract touchpoints

Files not named for a lane belong to nobody; touch them only by asking the
orchestrator.

| Lane | Owns (handoff) | Contract touchpoints it also needs (and who owns the touched file) |
|---|---|---|
| A. Jev | `internal/gateway`, `internal/yaegikernel/system1.go` | new rate scopes `judgments:sections`, `judgments:oped_gate` in `gateway/jev.go:43-52` (A owns) |
| B. Sections | `internal/frontimportance/scorer.go`, the four lists | section cache key (3.6); the house list read path. B must not touch `publication_front_importance.go` writes beyond its list (it is B's file per handoff) |
| C1. Texture revisions | `internal/textureowner/prompt_bar.go` and the Texture write path | `reviseDocument`; `types.Provenance.ParentLinks` (`internal/types/texture_provenance.go`); store hash and decode (`internal/store/texture.go`, `lifecycle.go`); new version-read route client. C1 owns all of these |
| G1. House style | the read path only | none; its take of the house guide uses C1's call later (G2) |
| H1. Ledger records | defines record types | `types.CommitmentRecord` bodies `Changelog`, `oped_arrival`, `oped_verdict`, `oped_wake_decision`; event-kind table (5.2). H1 owns `internal/types/commitment_*.go` additions; C1/C2 only emit |
| Q. Article quality | `internal/agentcore/intake_labels.go`, `story_publish.go`, `internal/store/management_tasks.go`, writer and management prompts, the citation renderer | none. Warning: `management_tasks.go:76` is the waking list (D6); Q must not edit `RecordWakesReader` |
| C2. Op-eds | none stated in the handoff | `internal/platform` publish verification and `responds_to` (`service.go`, `publication_versions.go`, `publication_graph.go`, `publication_opinion.go`, `internal/proxy/platform_publish.go`); new routes in `platform/handlers.go`; kinds in `objectgraph/registry.go`; `oped_intake` schedule in `agentcore/source_schedule.go`; the gate. C2 owns these. C1 must not edit `service.go` |
| D. Prompt bar | frontend prompt bar until C1's first slice | classification and case display; calls C1's contract |
| E. Changelog app | its own manifest | reads `changelog:<owner>` through the lease; owns `notice_intake` consumer UI only. The poller and route belong to C2 (the same substrate as `oped_intake`) |
| F. Asking | none stated | uses revise (new doc, text only) and C1's progressive update |
| G2. Personal style | none | revise take plus `body` |
| H2. Algorithm² | none | revise with a published proposal as source |

Collision list (files two lanes would both want):
- `internal/platform/service.go`: C2 only. Contract 3's notice derivation sits
  inside `PublishTexture` and is also C2's, which owns the publish
  verification. H1 specifies the record, C2 writes the code.
- `internal/types/commitment.go`: H1 only. C1 and C2 request additions from H1.
- `internal/objectgraph/registry.go`: C2 adds both new kinds in one change.
- `internal/agentcore/source_schedule.go`: C2 adds both schedule kinds in one
  change; lane X5 (schedules, O4) must rebase on it.

## 8. Decisions for the orchestrator

| # | Decision | Options | Recommendation and cost |
|---|---|---|---|
| D1 | What is `text` in the "edit" row? The handoff table says "an edit (owner, in the body)" but the owner revise today carries an instruction, not a body | (a) text is always an instruction; add an owner-only `body` field for direct edits; (b) text is the new body when the owner sends it with no sources | (a). Adds a fourth field to the handoff's four-parameter call, but keeps text meaning one thing. (b) makes the same field an instruction in the merge case and a body in the edit case |
| D2 | Stale or older base | (a) always refuse a non-head base; (b) accept an older base for comment and merge, refuse for edit and take | (b). The handoff wants "v3 of v7"; cost is the chain parent differs from the named base (recorded as a `base` link). (a) is simpler but breaks that feature |
| D3 | Where parent links live | (a) `Provenance.ParentLinks` (inside the hash); (b) a new `ParentRevisionIDs` column on `Revision`; (c) revision metadata | (a). No schema change, tamper-evident, additive. (b) is cleaner for queries but changes the store schema (red). (c) is not hash-covered |
| D4 | Author kind of a take | (a) `user` (the requester); (b) `appagent` | (a). No desk acts, so the owner-edit writer class is the honest one (registry `OwnerEdits`). Cost: the doc spec "Texture is the sole writer" is met by the custodian package, not by a desk run; state this in the problem record |
| D5 | How the paper learns of op-eds and how readers learn of notices | poll, push, live socket | Poll on a durable schedule. Costs up to one interval of latency (default 15 min; floor 1 min). Push buys seconds but fate-shares publish with the paper's liveness |
| D6 | How the accepted/backoff wake reaches the story's desk | (a) a Cast from `management:<owner>` (agent record wakes); (b) add `oped_wake` to the waking system records in `RecordWakesReader` | (a). No store change. It makes management an intermediary, which the goal file calls a hypothesis. (b) is direct but edits a red surface and the Q lane's file. Also: the desk's response (C2) writes with provenance links without passing the owner checks in `reviseDocument`; H1 must define that variant |
| D7 | The `CORPUSD_OPINION_LISTING` kill switch and the daily limit | delete the switch; keep or delete the limit | Delete the switch (no-switches rule, `CLAUDE.md`). Keep the limit as a correctness bound (5 new pieces/day). The owner can overrule |
| D8 | Where the gate runs | (a) the paper's computer; (b) a Node B host job at publish time | (a). The criteria document is a Texture the paper's owner edits; the verdict is an act of the paper on its own tape; the text is judged where it is consumed. Cost: the author waits for the poll interval and sees `pending`; if the paper's computer is down there is no verdict. (b) gives instant verdicts but the criteria must be a published document and the paper's tape only learns of the verdict by adoption |
| D9 | Algorithm² proposals as sources | require a published version; or allow a local proposal record | Require a published version. Cost: every proposal is publicly visible unless it is published to a private audience, which does not exist. If the owner objects, H2 needs a second source kind; do not widen the revise call without a ruling |
| D10 | Section questions | A all host; B all reader; C house on host, rest on the reader | C. No sharing of custom lists until pass 2; shared cache only written by the host. Lane A must size two buckets (host pool, per-computer `judgments:sections`). A gives the sharing the handoff hoped for but exposes every reader's interests and creates a guest-writable cache or a new host quota |
| D11 | Link spam: a true-but-unrelated parent link still notifies | accept; or rate-limit notices per publisher | Accept, one notice per (child, parent), plus a per-publisher cap of 20 notices a day at corpusd. Cost: a cap needs a counter in corpusd (derived from the notice objects' `created_at`, like the opinion limit) |
| D12 | An owner with two computers | notices to the owner (each computer gets one copy), or to one named computer | To the owner; each tape gets one deterministic record. Cost: duplicates across an owner's computers; the platform does not record which computer published |
| D13 | New cross-computer read route for a version body | new narrow route `GET /internal/platform/publications/versions/{id}`; or reuse the public export route | New narrow bound-guest route. Public export works too (research reads it) but returns a rendering; the merge needs `body_doc` and a hash check. Cost: one more auth admission to review |
| D14 | Gate failure policy | 3 retries over 24 hours then `gate_unavailable`; or retry forever | 3 retries. The handoff asks the author to always see a verdict; `gate_unavailable` is shown to the author as such. Cost: a stuck gate leaves the op-ed unlisted on the story |
| D15 | Level assignment for the changelog | by kind (table 5.2), or per event by a model | By kind. Cost: no per-event nuance. A model-set level is an unreviewable channel into the notification surface |

### 8.1 Rulings (orchestrator, 2026-11-10)

Every recommendation in the table is adopted. Builders code against the
recommended option. Notes where the ruling adds something:

- **D5.** Poll. The paper's `oped_intake` runs at the one-minute floor; a
  reader's `notice_intake` runs every five minutes. Each poll is one indexed
  range read and no model call. Physics: 1,000 computers polling every five
  minutes is about 3 reads a second at corpusd.
- **D6.** The wake is a Cast emitted by host code under `management:<owner>`.
  No management model run is involved, so this does not add the costly
  intermediary the owner called a hypothesis (2026-11-09). The decision to
  wake is its own record (`oped_wake_decision`), so moving the wake to a
  direct system record later changes one emitter and no history.
- **D7.** The switch is deleted in lane C2's first commit. The daily limit
  stays.
- **D8.** The gate runs on the paper's computer. With D5 the author sees
  `pending` for about a minute. Each gate judgment goes through lane A's one
  recording function, so the gate's thresholds can be calibrated.
- **D9.** A published version is required. Named residual for the owner:
  proposals are public until a private audience exists.
- **D10.** C. Lane A sizes the per-computer `judgments:sections` scope
  inside its one shared limiter; lane B does not add a limiter.
- **D11.** The cap is derived from the notice objects, with no new counter
  table.
- **Scope tonight.** Contract 1 (C1, H1) is wave 1. Contract 2 and the
  cross-computer half of contract 3 are wave 2 (C2). The local `update_*`
  events of contract 3 are wave 1 (H1 defines, C1 emits). A lane that finds
  the contract wrong stops and reports; it does not fork the contract.

## 9. Standing questions that apply

1. **Who settled this?** The shape (one call, sources as parents, publish and
   adopt, the notice rule) is the owner's, quoted in the handoff. The choices
   in section 8 are this seat's and are proposals until the orchestrator
   rules and, for D7 and D9, the owner.
2. **Conformance to settled decisions.** Texture is the sole canonical
   writer: kept (D4 notes the owner-edit writer class that already exists).
   Desks talk only through records: kept; the desk's op-ed response is a desk
   turn, not a cross-desk write. Shared content in Store B: kept (parents,
   verdicts and notices are corpusd objects). The no-switches rule: D7 removes
   one. Results cross by publication and adoption only: kept; no
   computer-to-computer route is added.
3. **Deletion citers.** The only deletion is `CORPUSD_OPINION_LISTING` and
   `SetOpinionListing`. Citers to repair in the same commit: `cmd/corpusd/main.go:76`,
   `nix/node-b.nix:1120-1126`, the opinion tests, and
   `docs/problems/oped-not-connected-2026-10-20.md` (F4 guard). No other
   deletion.
4. **Production consumers.** `/revise`, `/revisions`, the prompt bar and the
   opinion route all have callers today; the contract wraps them. The proposal
   delivery path has no consumer and is not built on here.
5. **Single state authority.** Parents: the signed revision's provenance on the
   author's tape (the published copy is derived). Verdict: the paper's tape
   (the corpusd copy is a courier copy with the same hash). Notice: the
   corpusd object; the local record is the adopted copy. Cache: host for the
   house list, reader's store for the rest. No fact has two winners.
6. **Artifact-verified success.** Each contract is done when a two-identity
   run on staging shows the artifact: a revision whose provenance names the
   source; an op-ed version with a verified `responds_to` and a verdict
   record on both tapes; a notice record on the author's tape that the
   changelog app renders. A merged lane or green unit test is not success.
7. **Fate-sharing.** Reading a story, an op-ed or a notice needs corpusd only.
   Writing a take needs the platform (to fetch the source) and fails closed
   with a 503. The gate needs the paper's computer and the gateway; its
   absence leaves the op-ed `pending`, not lost.
8. **Restart.** Take: atomic. Merge: crash closes the round, never resumes.
   `oped_intake` and `notice_intake`: cursor on the schedule record; a crash
   closes only the poll in flight (`source_schedule.go:25-45`). The cursor
   lives on the computer's store; replays are deduplicated by deterministic
   record ids. A planned restart needs nothing extra.
9. **No SSH.** Every probe is a product API or CLI call: `GET .../notices`,
   `GET .../opeds`, the ledger read. The acceptance steps use the choir
   CLI and the deployed spec, not Node B shell access.
10. **Registry hygiene.** Two new corpusd kinds go in `registry.go` with a
    writer; the new record body needs a decode root; no definition is
    created or superseded by this doc.

## 10. What this draft could not establish from the code

- Whether `RecordWakesReader`'s list (`management_tasks.go:76-125`) has a
  generic hook for a new waking system kind beyond the ones read; only the
  first 30 lines were read (D6 assumes there is not).
- Whether corpusd records the publishing computer id on a publication
  object. The graph batch sets no `ComputerID`
  (`publication_graph.go:100-137`), so D12 assumes it does not.
- The deployed Jev judgment body cap and per-organization limits (step 0.4);
  section 3.6's numbers are lane A's to confirm.
- Whether the Texture agent's frame can carry N source blocks without a
  prompt-size change (R12, the merge path). The frame code is
  `internal/agentcore/texture_frame.go`; not read in full.
- How the frontend shows `forked_from`; nothing in `frontend/` was read.
- Which computer's gateway bucket a bound-guest `GET .../opeds` poll uses: it
  is not a model call, so none, but the schedule's wake uses the
  `scheduleActor` mailbox (`source_schedule.go:48-60`).
