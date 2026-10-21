# A story cannot be asked for more research, and an owner revise tells Texture to throw the story away (2026-10-20)

Found while designing owner request "for any story to be able to request that
it be expanded. Eg more research" (design:
[story-expansion-request-design-2026-10-20.md](../reports/story-expansion-request-design-2026-10-20.md)).
Mutation class of the fix: red (Texture canonical writes by owner revision,
proxy authority, a new corpusd read). Problem first; no code in this commit.

Evidence labels follow the design: **code** is read at `0a541483c`;
**hypothesis** is a behavior claim with no trace yet (CLAUDE.md "Review the
traces before attributing behavior").

## P1. No door from a reader to a story's Texture desk (code)

- The downstream half exists: `POST /api/texture/documents/{id}/revise`
  (`internal/textureowner/texture_agent_revision.go`, `handleLifecycleOwnerRevision`)
  commits an owner revision carrying the head forward, wakes the desk and
  reopens settled work.
- Every Autopaper story is a document on the paper computer, owned by a
  dedicated account (`CORPUSD_AUTOPAPER_OWNERS`, `nix/node-b.nix`). The
  reader is another account, so its `/revise` is refused as "document not
  found". The public bundle hides the source document id
  (`versionMetadata.SourceDocID` is stored but not returned).
- The proxy has no operator or platform-owner concept. Today the only way to
  ask is the paper account's credential, a file on the operator's laptop.

## P2. The wake prompt misdescribes a carried-forward story (code; effect is a hypothesis)

`buildAgentRevisionRequest` (`texture_agent_revision.go:671-672`) appends, when
the head is `AuthorUser` with `input_origin=user_prompt`: "The current
revision is the owner's request itself, not prose to keep: write the first
useful document that answers it." That branch was written for a document whose
first revision is the owner's own request. An owner `/revise` also stamps
`input_origin=user_prompt`, but the revision carries the whole story forward
and has a parent (`ParentRevisionID: expectedHead`). For a finished story the
sentence points the wrong way. No trace of the live effect yet; phase 0 of the
design reads one.

## P3. Related, discovered, not fixed here (code; hypothesis)

`input_origin` is a durable metadata key, so it flows from the owner revision
into Texture's next revision. `promptOnlyInitialModelPriorTextureRevision`
(`texture_revision_metadata.go:90`) then stamps that revision
`model_prior_interim` / `grounding_status=model_prior_interim` /
`texture_version_stage=interim`. No reader of those keys was found in
`internal` or `frontend/src` (a grep), so publishing is not gated on them, but
an expanded story's revision may carry a wrong "model prior" label in its
metadata. Recorded as a heresy for a later cut, not repaired in phase 1.

## Phase 1 status (branch worktree-agent-a0236e0c335ef4037, not pushed)

Landed in order: this note; the wake-prompt condition (P2, test first);
corpusd `GET /internal/platform/publications/expansion-source`; proxy
`POST /api/platform/publications/expand` and `GET .../expansion`; the story
header control and a local spec; the deployed spec (written, not run).

Choices the design left open, taken conservatively:

- Dark has two shapes: an empty `PROXY_EXPAND_REQUESTERS` or
  `PROXY_EXPAND=off` makes the POST a 404 and the read `200 allowed:false`
  (design 11 said the route returns 404; the read stays 200 so the page logs
  no failed request).
- "Already asked" is checked three ways: proxy memory, the paper's chain
  (the owner revision id is recomputed from owner, computer, trajectory and
  client_request_id; `texture_owner_revision_identity_test.go` pins that
  formula), and the paper's replay or digest-conflict answer.
- The fate line is held in proxy memory, so a proxy restart drops it; the
  request itself stays on the paper's chain and a repeat ask answers
  "already asked". Phase 2 (a read-only guest endpoint) makes it durable.
- The fate "unknown" (the paper cannot be read) is added to the design's
  list so a stopped paper never reads as "no new version".
- The trace half of the acceptance hazard check needs the paper account's
  credential and is an operator step (`choir trace export`); the deployed
  spec records the window and ids it needs.

Residual P3 stays open: the interim stamp on Texture's revision after an
owner revise.

## Phase 1 decisions (owner questions answered with the design's recommendations)

The owner left the night to us and the CLAUDE.md "No Blocking Asks" rule
applies, so each open owner question takes the design's recommended answer:

1. Allowlist only the owner, button hidden from everyone else: **yes**
   (`PROXY_EXPAND_REQUESTERS`, empty means off).
2. Use the existing `/revise` owner door with a fixed request text, no new
   record kind: **yes**.
3. Skip management on this path; Texture asks research itself: **yes**.
4. One request per published version, three in flight, twelve a day, the day
   cap soft until phase 2: **yes**.
5. Phase 0 and the acceptance run on one real, old, low-importance story:
   **deferred to the owner**; the spec is written but is not run tonight.
6. Fix the "owner's request itself, not prose to keep" sentence in the same
   change, test first: **yes**.
7. "Revised" means "revised since your request" until phase 2: **yes**.
8. The owner's user id is the allowlist's deploy value: **yes**, supplied by
   the owner at turn-on; this change ships with the value empty (dark).
9. Outside readers stay out until the pre-production review: **yes**.

## Public door (owner 2026-10-20 section 32)

Owner decision, [owner notes](../reports/owner-notes-2026-10-14.md) section 32:
"Research more" on a published Autopaper story must be usable by readers who
are not logged in ("could be abused but dont worry about this for now"), and
the research should dig deeper with web search as well as more outlets. This
supersedes phase 1 decision 9 ("outside readers stay out until the
pre-production review"). Mutation class: red (proxy authority on an
unauthenticated route that ends in a Texture owner revision). Problem first;
no code in this commit.

The shape: a new flag `PROXY_EXPAND_PUBLIC` (default off). When on, a caller
with no owner session is a **public reader**: the proxy still asks the paper
through the story owner's door with the fixed request text, so the paper sees
the owner's act with the provenance "public reader". Nothing about the reader
reaches the guest. Signed-in allowlisted owners keep the existing path and are
not charged to the per-address throttle. `PROXY_EXPAND=off` still wins over
everything. A public reader is limited per address (3 an hour, 10 a UTC day,
in proxy memory) on top of the global limits (3 in flight, 12 a day, one per
published version).

Failure modes guarded, written before the code (each gets a failing test
first, in `internal/proxy/platform_expand_test.go`, ids P1 to P9):

- **P1 Anonymous accepted while the flag is off.** With
  `PROXY_EXPAND_PUBLIC` off, an anonymous POST must still be 401 (or 404 when
  dark) and the read `allowed:false`; the flag must not be implied by a
  non-empty allowlist.
- **P2 Owner allowlist path broken.** With the flag on, an allowlisted
  signed-in owner still asks and reads as before, is not charged to the
  per-address throttle, and `PROXY_EXPAND=off` still makes both routes dark
  even with the flag on.
- **P3 One address exhausts the shared daily cap.** One address asking
  different stories must be stopped after 3 in an hour and 10 in a UTC day
  with a typed `address_limit` refusal (429, Retry-After, the words "too many
  requests from your address; try later"); another address is still served;
  the window slides (an hour later the address may ask again; the next UTC day
  resets the day count).
- **P4 Spoofed X-Forwarded-For bypasses the per-address limit.** The address
  is derived exactly as the auth limiter does (`auth.clientIP`: the last
  `X-Forwarded-For` entry, which Caddy appends, else the peer address). A
  client that sends its own leading `X-Forwarded-For` entries, or changes
  them per request, is still one address.
- **P5 A second request on the same published version.** Public readers do
  not get a second revise for one version, across addresses; a refused repeat
  does not spend the address's allowance.
- **P6 A non-Autopaper or unknown route.** An anonymous read answers
  `allowed:false` and an anonymous POST answers the same 404 as an unknown
  route, without reaching the paper and without spending the address's
  allowance.
- **P7 The in-flight cap.** Public readers share the 3-in-flight and 12-a-day
  global limits; the fourth in flight is refused with the global `limit`
  status, not the address reason.
- **P8 Restart resets the in-memory counters.** Accepted residual: the proxy
  restarts on every deploy and keeps these counters in memory, so a restart
  gives each address a fresh allowance. The one-per-version rule survives
  because it is checked against the paper's chain. A test pins the residual so
  it is not mistaken for a guarantee. A durable counter is a later cut if
  abuse appears.
- **P9 The anonymous requester's identity reaches the guest.** The call to the
  paper carries the story owner as the user, the request text carries the
  marker `provenance: public reader`, and neither the address, the forwarded
  header, a session user id nor a cookie appears in the paper request or the
  proxy log line. No reader identity is stored: the ledger keeps only a hash
  of the address, in memory, for the throttle.

Also changed with the door: the fixed request text asks research to search the
web for new reporting, primary documents and additional distinct outlets and
to add what is new with citations, still as an expansion and not a rewrite.

Residuals recorded, not fixed: an anonymous read still costs a corpusd lookup
and, when a request is open, paper reads (bounded here by returning only the
newest request to a public reader); the global 12 a day can still be spent by
twelve different addresses; there is no CAPTCHA. The owner said not to worry
about abuse for now.

## P4. The door can never reach Texture: the paper's document read has no computer id (staging, 2026-10-21 02:31 UTC)

Evidence. Deploy 7b55b19a7 (CI 39532380202, staging deployed at 02:30:29
UTC) turned on `PROXY_EXPAND_PUBLIC`. The deployed spec
(`frontend/tests/autopaper-expand-deployed.spec.js`, `EXPAND_PUBLIC=on`) on
the FCA near-miss story (`pub-e61662f3299f634504fbcef7eef6773f`, 101 words,
2 sources) passed the unknown-route refusal, the read (`allowed: true`)
and the phone-width button, then got 502 `unavailable` on the POST. The
proxy log: `expand: read document: paper document has no lifecycle head`,
then `request=rx-4b600d44b21e ... requester=public
outcome=paper_unavailable`.

Cause (code, confirmed by reading both sides). `paperDocument`
(`internal/proxy/platform_expand.go:445`) requires `current_revision_id`,
`computer_id` and `trajectory_id` in the paper's `GET
/api/texture/documents/{id}`. The guest's `textureDocumentResponse`
(`internal/textureowner/texture.go:151`) has `trajectory_id` and
`current_revision_id` but no `computer_id` field at all, so the check
fails on every story. The proxy needs the document's own computer id
because the paper names the owner revision
`uuid.NewSHA1(owner, doc.ComputerID, doc.TrajectoryID, client_request_id)`
(`texture_agent_revision.go:156`), and the "already asked" check
recomputes that id. The proxy's unit tests stub a paper that returns
`computer_id`, so they pass against a contract the real paper does not
serve. This affects the owner-only door as well; it has never worked on
staging, and nobody noticed because the deployed spec had never run.

Consequence. No request reached the paper: nothing was written on its
chain, and the proxy ledger released the entry (no `accepted`). The
reader saw "the paper is not reachable right now", which is wrong: the
paper was reachable and the contract was broken.

Fix (smallest). Add `computer_id` (the document's own scope) to the
guest's document read, pinned by a textureowner test, so the paper serves
what the proxy already expects. Additive; old proxies ignore it. The
paper must take the new guest build (a `public_platform` computer takes a
planned refresh while busy). Rollback: revert; the door returns to this
502. Related lesson: a proxy test that fakes the guest should be built
from the guest's response type, not a hand-written map; recorded, not
done here.

## P5. After P4, the paper refuses the revise with a 409 the door calls "busy" (staging, 2026-10-21 02:46 UTC)

Evidence. With e0d1b0b6f deployed (CI 39533914520) and the paper refreshed
to it at 02:45:06 (planned restart, busy allowed), the logged-out ask on
the FCA story (`rx-4b600d44b21e`) and on a story two hours old, the UK
pension one (`pub-f7d2be0512ce1158219908232fc71729`, `rx-348ef09415e9`,
importance 4), both passed the document read and got 409 `busy`. So P4 is
repaired, and the next step fails on fresh and old stories alike.

What is known. The door's `busy` is its default for any paper 409 that
is not "digest conflict" or "head changed"
(`internal/proxy/platform_expand.go`). The paper's revise
(`internal/textureowner/texture_agent_revision.go`
`handleLifecycleOwnerRevision`) answers 409 for four other causes: the
trajectory is not live; several open desk target work items; none open
after the settled-work reopen; a lifecycle commit conflict. Neither the
proxy nor the paper logs which one; the paper's console shows nothing for
the call. Which cause it is: unknown (no trace evidence yet).

Next. The proxy logs the paper's 409 message (truncated, no content), the
ask is repeated on a third low-importance story, and the cause is written
here before any fix.
