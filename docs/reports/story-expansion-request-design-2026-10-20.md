# Ask a story to be expanded: a reader request that becomes more research

Draft 2026-10-20. Design only: no code changed. This document is green. The
class of the work it describes is in §8.

- Owner request, 2026-10-20: "Another related feature, maybe even simpler,
  is just for any story to be able to request that it be expanded. Eg more
  research."
- Authority: [agent product doctrine](../agent-product-doctrine.md). Texture
  is the sole writer of revisions and of published text. Research is
  read-only. There are four desks and no new ones (owner notes §23).
- Sibling designs: [related stories](related-stories-graph-design-2026-10-19.md)
  (same deploy path) and the separate
  [daily digest](autopaper-daily-digest-design-2026-10-20.md).

Evidence labels: **code** (read at `e7ea4afda`, with a path), **doc** (a dated
Choir document), **inference** (reasoning, untested), **hypothesis** (a
behavior claim with no trace yet; CLAUDE.md "Review the traces before
attributing behavior").

## 1. Summary

1. **The downstream half exists.** A story's Texture desk has a door for owner
   direction: `POST /api/texture/documents/{id}/revise`. It commits an
   owner-authored revision that carries the story forward, wakes the desk, and
   reopens the desk's work if it had settled. Texture then decides, as on any
   wake, whether to ask research, revise, or do nothing. The CLI already
   wraps it (`choir texture revise`).
2. **The upstream half is an identity gap.** The public stories belong to the
   paper's dedicated account, not to the person reading them. A reader's
   session cannot call `/revise` on those documents, and the public bundle
   hides the source document id. Today the only way to ask is the paper
   account's credential, a file on the operator's laptop.
3. **Smallest path:** one allowlisted proxy route. The owner clicks "More
   research"; the proxy checks the allowlist, asks corpusd which document the
   route publishes, rate-limits, and calls the paper's `/revise` with a fixed
   request text. The reader sees a fate line. Management is not a hop (§4.3).
4. **One defect comes first.** After an owner revise, Texture's wake prompt
   says "The current revision is the owner's request itself, not prose to
   keep: write the first useful document that answers it"
   (`internal/textureowner/texture_agent_revision.go:672`). For a finished
   story that points the wrong way. The sentence is in the code; the live
   effect is a hypothesis until a trace shows it (§3.4).
5. **Phase 0 needs no code.** Run the existing CLI on one low-importance story
   and read the trace: it measures cost and shows whether the defect bites.
6. **Phase 1 is one night** and ships dark behind an allowlist holding only
   the owner. It needs no credential or account action; turning it on needs
   one non-secret value, the owner's user id (§10).

## 2. Problem and intent

A published story is as deep as the paper's schedule made it. A reader who
wants more can only wait for the next poll to touch it. The owner wants to
say "this one, go deeper" and to see what became of the request.

The lever asks; it does not command. Doctrine says Texture delegation is
agentic and may not become a gate that forces a research call. So a request
can leave a story unchanged. That is a valid fate and must be shown.

**Non-goals.** No outside users: anyone off the allowlist gets no button and a
refusal. No free-text steering in phase 1 (an injection and cost surface; the
owner asked for "more research", not an editing console). No new desk, record
kind or graph kind. No change to what Texture may write or when a story
publishes.

## 3. What exists today

### 3.1 The door into a story's Texture desk (code)

`HandleTextureAgentRevision` serves the `/revise` route
(`texture_agent_revision.go:51`); `handleLifecycleOwnerRevision` (`:96`) needs
a live trajectory and commits an `AuthorUser` revision copying the head's
content, body, sources and citations, with the direction in metadata
(`input_origin=user_prompt`, `owner_prompt`, `:152`). It then wakes the
Texture desk (`dispatchTextureRevisionWake`, `texture_controller.go:26`). If
the desk had completed its work item the handler reopens one
(`reopenTextureWorkForOwner`, `:277`; the fix for
`docs/problems/settled-document-refuses-owner-revision-2026-10-26.md`); a
refused or cancelled item still refuses (409). It is idempotent on
`client_request_id`: the revision id hashes owner, computer, trajectory and
that id, so a replay returns the same revision.

### 3.2 Who owns the stories (code, doc)

Every Autopaper story is a document on the designated paper computer, owned by
a dedicated account (doc: `reeng-autopaper-paper-computer-2026-10-13.md`); the
front list shows only that owner (`CORPUSD_AUTOPAPER_OWNERS`,
`nix/node-b.nix:869`; `isAutopaperStory`, `publication_front.go:149`). The
reader is a different account, and the proxy has no operator concept or
platform-owner allowlist, so a reader's `/revise` is refused as "document not
found", correctly. A published version records its source document
(`versionMetadata.SourceDocID`, `publication_versions.go:385`) but the public
bundle does not return it, so a route-to-document lookup is a new
internal-only corpusd read.

### 3.3 How a host already acts for a computer's owner (code)

`publishTextureRevision` (`internal/proxy/platform_publish.go:179`) already
reads a computer's documents for its owner: it resolves the computer
(`resolveComputerURL`, `handlers.go:1268`) and calls the guest with
`X-Authenticated-User` (`fetchAutoputerJSON`, `:607`), trusted only from the
host peer (`HostSourcedCaller`). That helper only does GET; a POST helper is a
small addition on the same trust.

This is an authority bridge, so §8 classes it red. It moves a capability the
operator already holds (the paper account's credential) from a laptop file to
an allowlisted, logged session. It grants nobody a power they lack today.

### 3.4 What Texture sees after an owner revise (code)

`buildAgentRevisionRequest` (`texture_agent_revision.go:642`) writes "The
owner's direction for this revision:" and the text. Because the head is
`AuthorUser` with `input_origin=user_prompt`, it then appends the sentence in
§1.4. That branch was written for a document whose first revision is the
owner's own request. After a carry-forward revise the head is the owner's
revision but its content is the whole story.

I have no trace of this. Phase 0 reads one (§10). The fix is one condition:
apply the sentence only when the owner's revision has no parent. It is orange
and gets a test first.

Two smaller facts. The owner revision takes a revision number and is never
published (`AutoPublishRevision` runs on Texture's own commits only,
`texture_cell_author.go:393`), so each request leaves one cosmetic gap in the
published numbering. And `textureAgentRevisionRequest.Provenance` is
`json:"-"`, so the request id can ride only in the prompt text and
`client_request_id`.

### 3.5 Research, the publish gate and cost (code, doc)

Texture addresses `research:<doc>` and `management:<owner>` directly with
records (`recordsRoute`, `internal/agentcore/inbox_records.go:48`); neither
Texture nor management needs the other to reach research. A new revision
publishes only if it cites a captured source, spans at least two outlets
(`MinPublishOutlets = 2`, `story_publish.go:239`, owner §24) and leads with a
headline; otherwise it is held and no version appears (the "no new version"
fate).

The paper ran about $3.3 an hour in desk calls (`handoff-2026-10-15.md`: $2.52
for 178 runs, about $0.014 a run). An expansion is about one Texture run, one
to three research runs and a second Texture run: roughly $0.05 to $0.25
(inference, unmeasured). Search is separate: Keenable's free tier is paced at
3,772 calls a day (owner notes §23). Phase 0 measures both.

## 4. The design

### 4.1 The path

```text
owner (signed in) clicks "More research" on a story
  -> proxy  POST /api/platform/publications/expand     (allowlist, limits)   new
  -> corpusd internal read: route -> {source doc, owner, source revision}    new
  -> paper  POST /api/texture/documents/{doc}/revise   (fixed request text)  reused
  -> Texture desk wakes; may Cast research:<doc>; research reports back      reused
  -> Texture revises (or not); the gate publishes the next version           reused
  -> GET .../expansion?route= shows the fate                                 new
```

### 4.2 Reuse and new work

Reused: `/revise` (owner direction, reopening settled work), the
`client_request_id` de-duplication, the Texture-to-research records, the
publish gate, `resolveComputerURL`, and the host-calls-guest trust (GET today).
New: the route-to-source lookup in corpusd, the allowlist, limiter and kill
switch, the two proxy routes with a POST helper, and the button and fate line
in the story header of `AutopaperRoute.svelte`. Changed: one prompt condition.
No new object kind, edge kind, record kind or desk.

### 4.3 Why not management, and why not a new record

The owner's sketch was button, directive, management, research, Texture. The
code and doctrine support a shorter path.

- Texture already asks research itself. A management hop adds a wake and a
  place to stall for no authority it must supply. Management keeps spend
  arbitration and sees Texture's reports.
- An owner-emitted note record addressed to `texture:<doc>` is the cleaner
  long-term channel: no revision number, no framing defect. The store accepts
  owner-emitted records (`CommitmentEmitterOwner`, `types/commitment.go:453`)
  and they wake their reader (`RecordWakesReader`,
  `store/management_tasks.go:64`). But no HTTP route accepts one, and how
  Texture's frame renders it is untested. That is phase 2, chosen only if
  phase 0 shows `/revise` fits badly.

`/revise` is the sanctioned owner door (doctrine: humans interface only with
Texture), it exists, and it has tests.

### 4.4 The request text

Fixed in phase 1:

```text
[expansion rx-<id>] The owner asked for this story to be expanded with more
research. Look for additional outlets, primary sources and missing context.
Add what you can support with citations from sources you have read. Keep the
story's existing text, structure and citations unless a new source corrects
them. If there is nothing new to add, change nothing and say why in a note to
management.
```

The marker lets a person or a trace search find the revision. The last
sentence gives Texture an honest way to decline.

### 4.5 Request identity

`client_request_id = "expand:" + publication_id + ":" + source_revision_id`,
where the source revision is the one the latest published version was cut
from. A test proves these before code:
- A second click on the same published version sends the same id. The store
  replays it or refuses it as a conflicting command (the command digest
  includes the expected head). Either way there is no second owner revision
  and no second wake. The proxy shows "already asked".
- When Texture publishes a newer version the id changes and a new request is
  allowed.
- The limit lives in the paper's chain, so a proxy restart cannot lose it.

### 4.6 The proxy routes

`POST /api/platform/publications/expand` with `{"route":"/pub/texture/<slug>"}`:

1. Anonymous: 401. A user id not in `PROXY_EXPAND_REQUESTERS` (empty means
   off): 403. An API key: 403, as for mail sends (`isEmailOwnerActRoute`
   precedent), since an owner act needs the browser session.
2. Ask corpusd for the route's source document. Not an Autopaper story, not
   public, not published: 404, same body as an unknown route.
3. Apply the limits (§5). Resolve the paper by the story's owner id, read the
   document head, `POST /revise` with §4.4 text and the §4.5 id. A stale head
   gets one retry.
4. Return `202 {request_id, revision_id, status:"asked"}` and log one line:
   request id, publication id, requester hash, outcome.

`GET /api/platform/publications/expansion?route=` returns `{allowed, requests}`.
It answers 200 with `allowed:false` to anonymous and non-allowlisted callers,
so the page needs no 401 handling and the button never shows to anyone else.

## 5. Limits and budget

- Who may ask: the owner only; hard; `PROXY_EXPAND_REQUESTERS`, empty means off.
- Same published version: one request; hard and durable, in the paper's chain
  (§4.5).
- In flight: 3. Per UTC day: 12. Both soft, in proxy memory.
- Request text: a fixed template, in code. Kill switch: `PROXY_EXPAND=off`.

Twelve a day costs at most about $3 at the high estimate, near 4% of the
paper's baseline. The caps are soft on purpose: the proxy has no state
directory and the site deploys about twenty times a day, so a restart refills
the counter. For one trusted requester that is acceptable, because the
per-version limit is the hard one. The durable version belongs in phase 2:
count marker revisions through a read-only guest endpoint, or move arbitration
to management, which doctrine names as the resource arbiter.

## 6. Visible fate

One line under the button, each state derived from data that exists.

- **asked**: the proxy got a 202.
- **accepted**: `GET /api/texture/revisions/{revision_id}` finds the owner
  revision on the chain.
- **revised**: `/history` (author kind, time, parent; no metadata) shows a
  Texture revision after it, and the public nav shows a higher version.
- **no new version**: 30 minutes passed with none (phase 2 reads the work item
  to say "unchanged").
- **declined**: the proxy refused (403, 409, 429).
- **interrupted**: the paper restarted first; shown only after the restart
  drill (§9).

Limits, stated up front. "Revised" means a revision followed the request, not
that it was caused by it; a poll-woken revision could land first, and proving
cause needs the request id in Texture's metadata (phase 2). "No new version"
is silence, not a decision. And I do not know whether a restart resumes or
closes this request: `dispatchTextureRevisionWake` says the boot scan
re-derives the wake from the document head, while CLAUDE.md says a crash
closes open work. The drill tests it before the page claims "interrupted".

## 7. Ways it could fail

Written before code; tests follow this list.

- **F1 Texture rewrites or shortens the story** (§3.4). Phase 0 reads the
  trace first; the framing fix lands with a test; the text says to keep the
  story; the acceptance run compares old and new versions.
- **F2 Money spent, nothing changed.** Valid; the caps bound it and the fate
  shows it. Fewer than two outlets after research is the same case (§3.5).
- **F3 A non-owner asks.** Allowlist, 403, `allowed:false`; a test signs in as
  an ordinary account.
- **F4 The impersonation bridge widens.** The guest call is built only from a
  corpusd answer, never the request body; the document must belong to the
  story's owner; the allowlist is explicit; every call logs.
- **F5 Prompt injection.** The request text is fixed; no new untrusted text
  enters a prompt.
- **F6 A race with the paper's own revision.** 409 on a stale head; one retry,
  then "the story just changed, try again". Two open Texture items also give
  409 (`ownerRevisionTarget`), shown as "busy".
- **F7 The paper is stopped or booting.** A clear 502; nothing is queued, so
  nothing resumes after a crash.
- **F8 A busy paper.** The in-flight and daily caps; a request is one more
  wake, not a new load class.
- **F9 Numbering gap and route existence.** One cosmetic gap per request; an
  unknown, held or non-story route shares one 404 body.

## 8. Mutation class and ceremony

**Phase 1: red.** Protected surfaces: Texture canonical writes (the
carry-forward owner revision), proxy authority (a host acting as another
account), and a new corpusd read of source document ids. Provider routing,
vmctl, auth renewal and the gateway are untouched. The framing fix and the
button are orange within it.

**Conjecture delta.** A reader-triggered, fixed-text owner revision lets
Texture ask research and publish a better version at a cost the owner accepts.
Falsified if phase 0 shows rewriting instead of expansion, or more than half
of requests change nothing. Admissible evidence is a deployed spec and a trace
export on `choir.news`; local runs shape the prompt condition and limiter only.

**Heresy delta.** Discovered: the framing sentence misdescribes a carried
story; `Provenance` is dropped from the revise request; the proxy has no
durable limit state or operator concept. Introduced: a host-to-guest POST as
another account (§3.3). Repaired: the framing sentence.

**Problem first.** One commit adds `docs/problems/` notes for the framing
defect and the missing operator concept before any code.

**Standing questions.** Provenance: the owner's quote above. The paper's chain
holds the request; proxy counters are guards, not authority. The button
fate-shares with the proxy, corpusd and the paper; reading does not. No
queue, no boot sweep. Operable without SSH through the log line, the fate
read and `choir trace`. No new kinds; nothing deleted.

## 9. Acceptance proof on staging

Staging is `https://choir.news`. A deployed spec,
`frontend/tests/autopaper-expand-deployed.spec.js`, modelled on
`autopaper-reader-deployed.spec.js`. It needs an allowlisted expansion-QA
account (beside the owner) and an ordinary QA account. The owner picks the
target story (Q5).

**API assertions.**
1. Anonymous `GET expansion`: 200, `allowed:false`. Anonymous `POST`: 401.
2. Ordinary account: `allowed:false`, `POST` 403. API key on the allowlisted
   account: 403.
3. Allowlisted `POST` on the chosen story: 202 with a request id. A second
   `POST` for the same published version: "already asked", and the paper's
   revision list shows one marker revision.
4. A non-story route: 404.
5. Within 20 minutes the fate is `revised` or "no new version". Record which.

**Hazard and trace checks.** Read the story before and after: the new latest
version keeps at least 80% of the earlier paragraphs, its distinct source
count is not lower, and when revised either sources or words rose. Export the
window with `choir trace` (turn-commit reasons, `controls[]` to research,
work-item states, the first prompt Texture saw) and assert the prompt does not
contain "not prose to keep". A version that loses the old story fails the run.

**Restart drill.** At a quiet moment file a request, restart the paper by the
planned path, and record whether the request resumed or closed.

**Browser.** Signed in as the allowlisted account at 375 by 812: the button is
tappable, the fate line appears, no horizontal overflow. Signed out: no
button.

**Artifact.** JSON with request and revision ids, fate with times, before and
after counts, cost for the window from the costs API, and the trace summary.
Repeatable on another story. **Final evidence** names the pushed SHA, CI run,
staging identity, spec result, artifact, class and surfaces, heresy delta and
next realism axis.

## 10. Phased plan

### Phase 0: no code, a few hours

1. Choose one old, low-importance story with two or more sources.
2. With the existing CLI and the paper account's credential (never printed),
   run `choir texture revise` with the §4.4 text and a stable `--request-id`.
3. Export the trace. Record the first prompt, whether Texture cast research,
   cost and search calls, time to a new version, and what happened to the old
   text.
4. Write the result in `docs/problems/` as F1 evidence and decide from it
   whether `/revise` stays the channel.

### Phase 1: one night, red, dark by default

1. Problem note commit (green), then code.
2. Test first: a carry-forward owner revise with a parent does not say "not
   prose to keep"; a seed owner request still does. Then the one-line fix.
3. Corpusd: internal read `route -> {publication_id, owner_id, source_doc_id,
   latest_revision, source_revision_id}`, internal callers only.
4. Proxy: `PROXY_EXPAND_REQUESTERS`, `PROXY_EXPAND`, the POST helper, the two
   routes, the limiter, the log line, metric `platform_expand.request`.
5. Frontend: the button in the story header, visible when `allowed`; a fate
   line that polls every 30 seconds while a request is open.
6. The §9 spec. Landing loop: commit, push, CI, staging deploy, identity, spec.
7. Turn on by setting the allowlist to the owner's user id.

Phase 1 needs no credential, secret or account action. It needs one
non-secret deploy value (the owner's user id) to turn on, and the owner's
choice of target story for acceptance.

### Phase 2: after phase 1 is stable

A read-only guest endpoint for request fates and daily counts (durable
limiter, real "unchanged" fate); the request id inside revision metadata; if
the trace argues for it, an owner-emitted note record to `texture:<doc>`
instead of `/revise`; an optional focus from a closed list, never free text;
signed-in readers and per-user budgets only after the launch gates.

## 11. Rollback

Set `PROXY_EXPAND=off` or empty the allowlist: the route returns 404, the
button disappears, no data moves. Or revert the commit: owner revisions
already on the paper's chain are immutable history that carries the story
forward unchanged, and a request in flight settles like any wake. The framing
fix reverts alone, returning a known, documented defect. The product restore
path is not involved.

## 12. Questions for the owner

Each is answerable yes or no; the recommendation is mine.

1. Allowlist only you in phase 1, with the button hidden from everyone else?
   Recommend yes.
2. Use the existing `/revise` owner door with a fixed request text, and not
   build a new record kind yet? Recommend yes.
3. Skip management on this path and let Texture ask research itself?
   Recommend yes.
4. Cap at one request per published version, three in flight and twelve a
   day, with the day cap soft until phase 2? Recommend yes.
5. Run phase 0 and the acceptance run on one real, old, low-importance live
   story, knowing a good result publishes a real new version? Recommend yes.
6. Fix the "owner's request itself, not prose to keep" sentence in the same
   change, test first? Recommend yes.
7. Accept that "revised" means "revised since your request" until phase 2?
   Recommend yes.
8. Give me your user id as the allowlist's deploy value? It is not a secret.
   Recommend yes.
9. Keep outside readers out until the pre-production review? Recommend yes.
