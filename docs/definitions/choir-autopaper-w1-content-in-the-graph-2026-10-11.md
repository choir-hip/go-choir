---
definition_version: 4

# W1 — content in the graph. Station of
# definitions/choir-autopaper-metamission-2026-10-11.md; design
# docs/autopaper-sw-design-2026-10-09.md §6 W1 and §0 constraint 1.
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
      status: clean apart from an untracked experiment test (build tag experiment)
      class: goal_candidate
      owner: this session
      touch: goal_owned
      recovery: git
  observed:
    - >-
      Every embedded Dolt pool runs one connection, and object graph access
      takes a shared engine mutex. Both work around a race in
      unescapeHTMLCodepoints that upstream fixed in 6bea93f2 (July 16); we
      pin July 15
      (problems/embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md).
    - >-
      Capture bodies would land in og_objects.body (LONGBLOB in Dolt) via
      objectgraph.CreateWebCapture. The design wants bodies in content
      storage, not Dolt rows.
    - >-
      internal/sourcegraph.WriteWebCaptureGraphObjects maps fetched
      sources.Item values to web_capture objects; its only caller is
      internal/cycle (sourcecycled, slated for deletion). Its content and
      extracted-text blob ids are labels ("sourcecycled:item:...") naming
      no stored blob; the text itself goes into og_objects.body.
    - >-
      The guest has no blob store outside Dolt. content_items keeps text in
      Dolt but carries a file_path column; the computer's file root is
      synced to the platform as encrypted content-addressed chunks
      (internal/filecas, internal/autoputer/file_sync.go), the path the SH
      lose-the-disk proof exercised.

finish:
  deliver: >-
    An Autopaper computer polls the MVP sources on a schedule; every new
    item is a choir.web_capture in its object graph with its body in
    content storage; Texture and health stay responsive while polling
    runs; resource use is recorded per poll.
  artifact: >-
    Dolt upgraded past the race fix; captures on their own pool (or their
    own store) so they never queue behind Texture; a research verb that
    polls a source into captures; the source schedule as timer obligations;
    a usage record per poll.
  acceptance:
    - action: >-
        Staging test computer polls the MVP sources for an hour while the
        research-only demo runs; Texture turn latency and /health latency
        are within 20% of the no-polling S1e baseline; captures counted in
        the graph match items fetched; bodies are not in Dolt rows.
      proves: C1 cannot recur; content lands in the graph
      evidence_class: deployed proof (test computer)
    - action: restart the computer mid-poll
      proves: a crash restart closes the poll obligation; the schedule resumes on its next timer
      evidence_class: deployed proof
  rollback: revert each slice; Dolt downgrade only if no computer has written with the new version
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Maximize captures landed per minute while keeping Texture and health
    latency flat, with one authority per capture.
  goodharting_would_be: >-
    A second engine or process that hides the race instead of fixing it;
    capture counts without readable bodies; polling that pauses Texture.

homotopy:
  realism_axis: >-
    Dolt fixed and race-tested -> a capture pool beside Texture's ->
    one source polled by hand -> the schedule -> all MVP sources under load.

boundaries:
  mutation_class: red (embedded store engine and concurrency)
  authority_sources: [owner design decisions 2026-10-09, owner overnight goal 2026-10-11]
  must_preserve:
    - existing computers boot on their stored data after the upgrade
    - Texture and lifecycle writes keep their transaction semantics
    - a crash restart never resumes work
  excluded: [new feeds, beats, W2 story handling]
  protected_surfaces: [internal/store, internal/objectgraph, go.mod dolt pins]

now:
  status: working
  slice: W1b — one source polled by hand (choir.PollSource) on staging
  source_ref: c1d21d5e (pushed)
  deploy_identity: staging 5b077256; c1d21d5e deploying
  candidate: {id: W1b, state: landing, ref: main, base: 5b077256, digest: c1d21d5e, scope: [internal/capturesegment, internal/sourcegraph, internal/researchtools, internal/yaegikernel, internal/sources, internal/objectgraph]}
  conjecture:
    id: W1-1
    claim: >-
      With Dolt past 6bea93f2, the embedded engine serves several
      connections safely, so captures get their own pool rather than their
      own engine.
    test: our store/objectgraph/agentcore packages under -race on the upgrade; staging boot on existing data; then a capture pool under load
    edge: missing_oracle
    delta_o: a race test reaching the lazy-JSON path (not yet reproduced locally)
    scope_if_supported: one guest process
    status: partially supported (upgrade landed and boots on existing data; widening not yet tried)
    evidence_refs: [receipt W1a]
  decision:
    what: capture bodies in daily append-only segments in the files root; capture objects carry metadata only and reach the tape through the projection intercept
    kind: architecture
    status: settled (agent; open to owner review)
    evidence_ref: f78daa03
    owner_ratification_ref: design §2 "capture bodies live in content storage, not Dolt rows"
  belief:
    believed_state: >-
      Dolt is past the race fix on staging. Captures can be written by
      research through choir.PollSource; every pool still has one
      connection, so capture writes still queue behind Texture.
    main_uncertainty: whether a poll's object writes (one tape event each) slow Texture turns measurably before the pool widens
    next_observation: W1b on staging — a research desk polls one source; counts, segment bytes, and Texture latency during the poll
  blocker_or_risk: the projection intercept makes one host round trip per capture; a large first poll may take tens of seconds
  next_action: after c1d21d5e deploys, ask a test computer's research desk to poll arxiv:cs_ai and read the trace

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

# Capture body storage (decision, agent, 2026-10-11; open to owner review):
# capture bodies are appended to one segment file per UTC day in the
# computer's file root (captures/YYYY/MM/DD.jsonl, one JSON line per body).
# The web_capture metadata names the body by segment, byte offset, length
# and sha256 (content_blob_id "capseg:YYYY-MM-DD:<offset>:<len>:<sha256>"),
# and og_objects.body stays empty. Durability comes from the existing file
# sync, which chunks files at fixed sizes, so an append changes only the
# last chunk; one file per body would have meant hundreds of thousands of
# files walked on every sync cycle. A Dolt ignored table was the other
# alternative and keeps bodies as Dolt rows, which the design rules out.
# Retention drops whole day files after 30 days, copying cited bodies to a
# pinned segment first (later slice).
---

# W1 — content in the graph

Captures are the paper's raw material. In July they starved Texture,
because everything shared one embedded connection. That single connection
works around a Dolt race that upstream fixed one day after our pin. So
W1 starts at the substrate: upgrade Dolt and prove it under the race
detector. Then captures get their own pool, research polls sources into
`choir.web_capture` objects with bodies in content storage, the schedule
runs as timer obligations, and every poll records what it cost.

## W1b — one source polled by hand (planned)

A research verb, `choir.PollSource(sourceID)`, runs the existing poller
for that source (`internal/sources`: RSS, Telegram web preview, GDELT)
through the research host egress (`internal/researchtools`, with the
egress budget charged once per poll). Each new item becomes a body file
appended to the day's capture segment plus one `choir.web_capture`
object whose metadata names the segment, offset, length and hash. The verb returns counts and capture ids, not bodies. The source
list is `internal/sources/registry.json` (moved from configs/, embedded in the guest) (211 sources: 137 RSS, 73 Telegram, 1
GDELT).

Ways it can fail, written before the code:

- a re-poll writes duplicate captures for items it already holds (the
  item id or content hash must make a second write a no-op);
- a body lands in `og_objects.body` anyway, or the metadata names bytes
  that were never written (append and sync the segment first, then write
  the object; a reader checks the hash);
- two polls append to one segment at once and interleave lines (one
  appender per computer, under a lock);
- one slow or hostile feed holds the desk cell past its timeout, or a huge
  feed floods the result (bound items per poll and the result size);
- the poll writes queue behind Texture on the one connection (measure
  Texture turn latency during a poll; this is what W1a's pool is for);
- conditional GET state (ETag, Last-Modified) is lost between polls, so
  every poll refetches everything;
- a crash mid-poll leaves appended bytes with no object; that is harmless
  (a partial last line is skipped by readers and the next append starts a
  new line), but an object must never name bytes that are not there.

## W1c — the source schedule as timer obligations (planned)

The runtime already has durable wakes with a not-before time
(`scheduleContinuation`, internal/agentcore/continuation_schedule.go).
The schedule is one wake kind, `source_poll_due`, per scheduled source.
Its handler runs the same `poll_source` host function with no model call,
records the poll's usage, and arms the next wake at the source's interval.
Management starts and stops a source's schedule; research's
`choir.PollSource` stays the by-hand path.

The owner rule decides restarts: a crash restart closes every open
obligation, so a crash closes each schedule with the visible fate
"interrupted by a restart". It never resumes on its own. Management sees
the closed schedules and reopens them on its next activation. A planned
update restart (with the marker) re-arms them once.

Ways it can fail, written before the code:

- a crash restart resumes polling (an auto-resume loop): no boot path may
  re-arm a schedule without the planned-restart marker, and
  `source_poll_due` must join `restartWorkKinds`
  (internal/actorruntime/handler.go), the list a crash boot closes as
  interrupted_by_restart; a kind missing from it fires after the crash;
- two wakes for one source run at once (a second arm while one is due):
  one open obligation per source, keyed by source id;
- a failing feed retries in a tight loop: a failure arms the next wake at
  the interval, doubling up to a day, and records the error;
- polls pile onto Texture's connection: measure Texture turn latency while
  the schedule runs (the W1 acceptance), before widening any pool;
- usage is not recorded: every poll writes one usage record (source,
  items fetched and new, bytes, duration, error).
