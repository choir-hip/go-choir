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
