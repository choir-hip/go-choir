now:
  status: working
  slice: W1 close — second-computer read on staging, then the completion panel
  source_ref: 19b2f964 (pushed)
  deploy_identity: staging cd7221ae (host); gateway 9c1c96c7
  candidate: {id: W1c, state: landing, ref: main, base: cd7221ae, digest: none, scope: [internal/proxy, internal/agentcore, internal/actorruntime, internal/store, internal/researchtools]}
  conjecture:
    id: W1-3
    claim: >-
      Captures written through the proxy to corpusd land in Store B with
      bodies on host disk; a source schedule of generation-fenced durable
      wakes polls without a model, records usage per poll, and never
      resumes after a crash.
    test: deployed — start a schedule on the test computer, watch Store B rows and body files grow, read a body from a second computer, restart the guest
    edge: missing_oracle
    delta_o: Store B capture rows and capture-bodies files after scheduled polls; schedule usage from the API; schedule state after a restart
    scope_if_supported: one paper computer polling a few sources, any computer reading
    status: proposed
    evidence_refs: [8574a8e1, 19b2f964]
  decision:
    what: captures in Store B behind corpusd; bodies in host content storage by sha256; computers keep overlays; the schedule is the paper computer's own state
    kind: architecture
    status: settled (owner, 2026-10-11)
    evidence_ref: problems/autopaper-captures-built-per-computer-against-store-b-2026-10-11.md
    owner_ratification_ref: owner 2026-10-11 "yes, move the write path to host store, update docs, lets not make this mistake again"
  belief:
    believed_state: >-
      Host services run cd7221ae (capture route) and the proxy fix lands with
      19b2f964. The test computer was still on c1d21d5e at 11:05Z, after the
      cd7221ae deploy; whether idle computers pick up a deploy promptly is
      unconfirmed.
    main_uncertainty: whether the test computer updates to 19b2f964 after the deploy, and whether Store B absorbs scheduled capture writes
    next_observation: test computer build identity after the deploy; then one scheduled source's first polls
  blocker_or_risk: >-
    each scheduled poll rewrites its schedule object, which in a guest is one
    event on the computer's chain per poll; fine for a handful of sources,
    about 30,000 events a day for all 211 at ten minutes. Bound it (write
    usage only on change, or batch) before scheduling the full registry.
  next_action: >-
    after 4ea25d28 and f84c29bd deploy: a second computer reads a paper
    capture with choir.ReadCapture; DW polls produce items; then the
    completion panel and W1's close
---
definition_version: 4

# W1 — content in the graph. Station of
# definitions/choir-autopaper-metamission-2026-10-11.md; design
# docs/autopaper-sw-design-2026-10-09.md §2 (storage, corrected 2026-10-11)
# and §6 W1.
readiness: executable

review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-11T09:10:00Z
  source:
    canonical_ref: main e21e1347
    deploy_identity: staging choir.news 78c9173e
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Store B (the corpus Dolt behind corpusd) is the world-wire store: the
      platform object graph and the source and ingestion tables
      (docs/computer-ontology.md, Dolt Store Taxonomy). corpusd serves
      /internal/platform/objects to bound guests: a guest may write objects
      it owns, and any bound guest may list.
    - >-
      There is no shared blob store a guest can write. The files CAS
      (/internal/computers/files/chunks) is per computer and encrypted.
    - >-
      internal/sources pollers (RSS, Telegram web preview, GDELT) and the
      211-source registry exist; internal/sourcegraph maps items to
      web_capture requests.
    - >-
      Every embedded Dolt pool runs one connection, a workaround for a race
      upstream fixed in 6bea93f2
      (problems/embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md).
      With captures in Store B this is no longer on Autopaper's ingest path;
      it still matters for Texture and every guest.

finish:
  deliver: >-
    The paper's computer polls the MVP sources on a schedule. Every new
    item becomes a choir.web_capture in Store B, with its body in host
    content storage, and any bound computer can read it. Resource use is
    recorded per poll.
  artifact: >-
    Dolt upgraded past the race fix (W1a); a corpusd capture route that
    writes captures to Store B and bodies to host storage in one request;
    research's choir.PollSource writing through it; the source schedule as
    timer obligations; a usage record per poll.
  acceptance:
    - action: >-
        The paper's test computer polls the MVP sources for an hour. Captures
        in Store B match items fetched, no body is in a Dolt row, a second
        computer reads a capture and its body, and corpusd and Store B stay
        within their memory budget.
      proves: content lands in the shared graph, readable by everyone, without the 104 G failure
      evidence_class: deployed proof (test computers)
    - action: restart the paper's computer mid-poll
      proves: a crash restart closes the poll obligation and nothing resumes on its own
      evidence_class: deployed proof
  rollback: revert each slice; Dolt downgrade only if no computer has written with the new version
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize captures landed per minute in the shared store, readable by
    every computer, with one authority per capture and no body in Dolt.
  goodharting_would_be: >-
    Captures counted in one computer's private graph; capture counts without
    readable bodies; bodies in Dolt rows; a per-computer copy of a shared
    item.

homotopy:
  realism_axis: >-
    Dolt fixed and race-tested -> one source polled by hand into Store B ->
    a second computer reads it -> the schedule -> all MVP sources for an hour.

boundaries:
  mutation_class: red (Store B writes through corpusd; host content storage; embedded store engine)
  authority_sources:
    - owner design decisions 2026-10-09 (design doc)
    - owner 2026-10-11 10:40Z — captures go to the global store; computers hold overlays
    - docs/computer-ontology.md Dolt Store Taxonomy
  must_preserve:
    - no capture body in a Dolt row
    - a guest writes only objects it owns; Store B ownership checks stay in corpusd
    - existing computers boot on their stored data
    - a crash restart never resumes work
  excluded: [new feeds, beats, W2 story handling, overlays (W2/W3)]
  protected_surfaces: [cmd/corpusd, internal/platform object graph routes, internal/store, go.mod dolt pins]

now:
  status: working
  slice: W1b2 — choir.PollSource writes captures to Store B through a corpusd capture route
  source_ref: 0dbb04c1 (local)
  deploy_identity: staging c1d21d5e
  candidate: {id: W1b2, state: designing, ref: main, base: c1d21d5e, digest: none, scope: [internal/platform, cmd/corpusd, internal/researchtools, internal/sourcegraph]}
  conjecture:
    id: W1-2
    claim: >-
      Captures written to Store B through one corpusd request per poll, with
      bodies on host disk by hash, are shared, deduplicated, and cost the
      guest one round trip per poll.
    test: unit test of the route with the real handler; staging poll by hand, then a read from a second computer
    edge: missing_oracle
    delta_o: Store B row counts and body files after a poll; a second computer's read
    scope_if_supported: one paper computer writing, any computer reading
    status: proposed
    evidence_refs: []
  decision:
    what: captures in Store B behind corpusd; bodies in host content storage by sha256; computers keep overlays
    kind: architecture
    status: settled (owner, 2026-10-11)
    evidence_ref: problems/autopaper-captures-built-per-computer-against-store-b-2026-10-11.md
    owner_ratification_ref: owner 2026-10-11 "yes, move the write path to host store, update docs, lets not make this mistake again"
  belief:
    believed_state: >-
      W1b on staging writes captures into the polling computer's embedded
      graph, which is the superseded path. The poll verb, egress and
      conditional GET work in a real guest.
    main_uncertainty: whether Store B absorbs capture writes without the October 1 memory pressure
    next_observation: a poll by hand into Store B on staging; corpus-dolt memory before and after
  blocker_or_risk: Store B is the store that grew to 104 G; bodies must never enter it, and its history needs a bound before polling all sources
  next_action: build the corpusd capture route and switch poll_source's sink to it; delete the guest capture segment

receipts:
  - id: W1a
    at: 2026-10-11T10:02Z
    deploy: staging 5b077256
    kind: deployed proof plus local race tests
    result: >-
      Dolt a6690826 -> 6bea93f2 (the JSON unescape race fix) with the
      matching go-mysql-server. go test -race passed on internal/store
      (23 min) and internal/objectgraph; textureowner, agentcore,
      yaegikernel and researchtools passed. CI green. After a restart on
      the new build, demo-arxiv-2026-10-10c reported 5b077256, served its
      ten documents 7 s after boot, and read all eight revisions of
      b2352fec with content and sources intact. No pool was widened.
    conjecture: W1-1 partially supported
  - id: W1b2-attempt
    at: 2026-10-11T11:00Z
    deploy: staging cd7221ae (host services); test computer still on c1d21d5e
    kind: deployed attempt (test computer demo-arxiv-2026-10-10, document cd1bdb26)
    result: >-
      Research polled rss:bbc_world (5 new captures, HTTP 200), but the test
      computer had not updated past c1d21d5e, so the captures went to its
      own graph by the superseded path. Node B had no capture-bodies
      directory, and corpusd and the proxy logged nothing. Reading the path
      showed W1b2 could not have worked anyway: guests reach corpusd only
      through the proxy, which forwarded objects and edges but not the
      capture routes
      (problems/capture-route-unreachable-through-proxy-2026-10-11.md,
      fixed in 8574a8e1).
    conjecture: W1-2 untested on staging; the unit tests skipped the proxy hop
  - id: W1b
    at: 2026-10-11T10:29Z
    deploy: staging c1d21d5e
    kind: deployed proof (test computer demo-arxiv-2026-10-10, trace in session scratchpad w1b/t1)
    result: >-
      A prompt asked research to poll arxiv:cs_ai. Research called
      choir.PollSource("arxiv:cs_ai", 20): HTTP 200 with zero items in 33 ms
      (arXiv publishes no listings on weekends), then a 304 in 120 ms on
      the next round. The verb, host egress and conditional GET work in a
      real guest. The write path (captures into the polling computer's
      embedded graph, bodies in a file-root segment) is superseded by the
      owner's decision the same hour
      (problems/autopaper-captures-built-per-computer-against-store-b-2026-10-11.md).
    conjecture: superseded
---

# W1 — content in the graph

Captures are the paper's raw material, and they are a resource for
everyone. They live in Store B, the global store behind corpusd. Bodies
live in host content storage by hash, never in Dolt rows. Each computer
keeps its own working state and overlays in its embedded Dolt. The paper's
computer writes captures; any computer reads them.

W1a upgraded embedded Dolt past an upstream race fix (landed). W1b gave
research a poll verb, which works on staging but wrote into the polling
computer's own graph. That was a design error, and its record is in the
problem doc above. W1b2 moves the write path to Store B.

## W1b2 — captures to Store B through corpusd

corpusd gets a capture route:

- `POST /internal/platform/captures` takes one poll's items (metadata and
  body each) from a bound guest. For each item, the host stores the body
  on disk by sha256, unless it is already there. It then creates the
  `choir.web_capture` in Store B, unless an object with that item's
  identity already exists, with the object's body empty. The metadata
  names the body as `sha256:<hex>`. Owner and computer must match the
  calling guest, as for every guest object write. The response gives
  counts and the new captures' ids, titles and URLs.
- `GET /internal/platform/captures/bodies/<sha256>` returns a body to any
  bound guest or internal caller.

Research's `choir.PollSource` keeps its surface. Its sink becomes one POST
to corpusd per poll. The guest capture segment (internal/capturesegment)
is deleted.

Ways it can fail, written before the code:

- a body reaches a Dolt row (the route must write the object with an empty
  body, whatever the guest sends);
- a guest writes captures owned by another computer (ownership check as
  for `/internal/platform/objects`);
- the metadata names a body that is not on disk (write and sync the body
  file before the object; a reader checks the hash);
- a re-poll writes duplicates (identity per item; existing objects are
  repeats);
- a huge request exhausts corpusd memory (bound items per request and
  bytes per body);
- a partial body file after a crash (write to a temp file and rename;
  content addressing makes a retry idempotent);
- a second computer cannot read a capture or its body (list and body reads
  allow any bound guest);
- Store B grows without bound (no bodies in Dolt; history bound before all
  sources poll; the October 1 memory budget for corpus-dolt is watched).

## W1c — the source schedule as timer obligations (planned)

The runtime already has durable wakes with a not-before time
(`scheduleContinuation`, internal/agentcore/continuation_schedule.go).
The schedule is one wake kind, `source_poll_due`, per scheduled source on
the paper's computer. Its handler runs the same poll (no model call),
records the poll's usage, and arms the next wake at the source's interval.
Management starts and stops a source's schedule; research's
`choir.PollSource` stays the by-hand path.

A crash restart closes every open obligation, so a crash closes each
schedule with the visible fate "interrupted by a restart". It never
resumes on its own. Management sees the closed schedules and reopens them
on its next activation. A planned update restart (with the marker)
re-arms them once.

Concrete shape (2026-10-11, from the code as it stands):

- **State:** one `choir.source_schedule` object per source in the paper
  computer's own graph (its working state, not shared content), keyed by
  source id: interval, max items, state (`active`, `stopped`,
  `interrupted_by_restart`), a generation number, the next due time,
  running totals, and the last 24 polls' usage. Usage history is bounded
  inside the object; there is no per-poll row.
- **Wake:** kind `source_poll_due`, addressed to a mailbox per source
  (`source:<id>`), so one source's polls run one at a time and a slow feed
  never holds management's mailbox. The wake carries the source id and
  generation; a wake whose generation is not the schedule's current one,
  or whose schedule is not active, does nothing.
- **Poll:** the same code as `choir.PollSource`, called directly with no
  model and no activation ledger; its usage goes to the schedule object.
- **Next wake:** armed from inside the handled wake, so it becomes durable
  with that wake's commit.
- **Control:** `/api/autopaper/sources/schedules` (list, start, stop),
  served by the runtime and on the product API allowlist, so management
  calls it with `choir.ProductAPI` and the owner can read it.
- **Crash boot:** a boot phase marks every active schedule
  `interrupted_by_restart`, the same as self-development operations; the
  pre-boot wake is refused by `restartWorkKinds`. A planned boot changes
  nothing and the pending wake fires.

Ways it can fail, written before the code:

- a crash restart resumes polling (an auto-resume loop): no boot path may
  re-arm a schedule without the planned-restart marker, and
  `source_poll_due` must join `restartWorkKinds`
  (internal/actorruntime/handler.go), the list a crash boot closes as
  interrupted_by_restart; a kind missing from it fires after the crash;
- two wakes for one source run at once: one open obligation per source,
  keyed by source id;
- a failing feed retries in a tight loop: a failure arms the next wake at
  the interval, doubling up to a day, and records the error;
- usage is not recorded: every poll writes one usage record (source, items
  fetched and new, bytes, duration, error).
