---
definition_version: 4

# W1 — content in the graph. Station of
# definitions/choir-autopaper-metamission-2026-10-11.md; design
# docs/autopaper-sw-design-2026-10-09.md §2 (storage, corrected 2026-10-11)
# and §6 W1.
readiness: executable

review:
  reviewer: agentic-consensus completion panel (three rounds)
  frozen_ref: 66b96f10
  verdict: accept
  evidence_ref: receipts W1-panel1, W1-panel2, W1-panel3

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
  status: complete
  slice: W1 complete — the third completion panel approved the landed candidate (seven of eight, none sent back)
  source_ref: 66b96f10 (pushed; W1 code through 56ec63e0)
  deploy_identity: staging 8031258a (host); the paper computer runs 726aa392 (guest code unchanged since, except the error-text bound)
  candidate: {id: W1, state: accepted, ref: main, base: e21e1347, digest: 66b96f10, scope: [internal/platform, internal/proxy, internal/agentcore, internal/actorruntime, internal/store, internal/researchtools, internal/sources, internal/yaegikernel, internal/objectgraph]}
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
    status: supported
    evidence_refs: [receipts W1c, W1-hour, W1-read, W1-fix]
  decision:
    what: captures in Store B behind corpusd; bodies in host content storage by sha256; computers keep overlays; the schedule is the paper computer's own state
    kind: architecture
    status: settled (owner, 2026-10-11)
    evidence_ref: problems/autopaper-captures-built-per-computer-against-store-b-2026-10-11.md
    owner_ratification_ref: owner 2026-10-11 "yes, move the write path to host store, update docs, lets not make this mistake again"
  belief:
    believed_state: >-
      Every acceptance item holds on staging: scheduled polls land in Store
      B with no body bytes in rows and bodies on host disk, dedupe on
      re-poll, record usage, back off, survive a host deploy, close on a
      crash restart; a second computer reads a capture and its body. A
      running computer picks up a deploy only at its next boot.
    main_uncertainty: whether the second panel finds another path that breaks ownership, the body bound, or the restart rule
    next_observation: W2's first delivery to management on the paper computer
  blocker_or_risk: >-
    each scheduled poll rewrites its schedule object, which in a guest is one
    event on the computer's chain per poll; fine for a handful of sources,
    about 30,000 events a day for all 211 at ten minutes. Bound it (write
    usage only on change, or batch) before scheduling the full registry.
  residuals:
    - each scheduled poll adds one event to the paper computer's chain and one actor-log row; bound it before scheduling all 211 sources
    - source_poll_due wakes share the dispatcher's eight slots with desk work
    - sourcecycled (not in Node B's boot set) still writes capture bodies into rows if it is ever enabled; delete or route it through the capture route first
    - discovered heresy, not repaired — a same-owner guest can overwrite another computer's object of any kind through the generic object route (separate auth change)
    - the MVP source list is open; France 24 and UN News refuse Node B
    - third panel (suggestions) — item titles, ids and URLs are not size-capped in rows; the 1 MiB body cut can split a UTF-8 character; the stale-wake re-arm has no test of its own; a failed schedule save after a poll lets a retry fetch again (captures dedupe)
    - for W2 (second panel) — no per-guest quota on the capture route; a capture is what its writer says, so shared readers filter by a trusted writer; items beyond a poll's max_items are never fetched once validators are kept; a body file written before a failed row write is orphaned
  next_action: >-
    none for W1; W2 (definitions/choir-autopaper-w2-living-textures-2026-10-11.md)
    is the next station
    (definitions/choir-autopaper-w2-living-textures-2026-10-11.md, drafted)

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
  - id: W1c
    at: 2026-10-11T11:52Z
    deploy: staging 19b2f964 (host and guest), test computer demo-arxiv-2026-10-10 restarted onto it
    kind: deployed proof (session scratchpad w1c/)
    result: >-
      The owner API started schedules for rss:bbc_world and
      rss:aljazeera_all at five minutes, ten items. The first wakes polled
      at 11:33:40 with no model call: ten new items each, HTTP 200, 42 and 20
      ms. Store B (corpus database) gained 20 choir.web_capture rows for
      the owner with no body bytes in any row; Node B gained 20 body files
      (80 KB) under platform-artifacts/capture-bodies. Polls at 11:38:40 and
      11:43:40 wrote nothing new: BBC returned ten repeats, Al Jazeera a
      304. Each schedule's usage (fetched, new, repeated, bytes, times)
      reads back from the API. corpus-dolt stayed at 36 to 49 MB. A
      lifecycle restart at 11:44:30 (no planned-restart marker) booted the
      guest at 11:44:39; the boot closed both schedules as
      interrupted_by_restart at generation 2, and the two wakes due 11:48:40
      were refused by the actor handler's crash branch: both schedules
      still show three polls. Corpusd's capture route carried the writes;
      the guest reaches corpusd directly (corrected 13:40Z: an earlier
      wording credited the proxy hop, problems/capture-route-unreachable-
      through-proxy-2026-10-11.md).
    conjecture: W1-3 supported for one paper computer and two sources
  - id: W1-hour
    at: 2026-10-11T12:53Z
    deploy: staging 19b2f964 (host and guest)
    kind: deployed proof (session scratchpad w1c/hour-end.jsonl, storeb-hour-end.txt)
    result: >-
      Six stand-in sources (the MVP list is still open) at ten minutes,
      twenty items, from 11:52:44 to 12:52:44, plus the two earlier
      schedules on the same sources. Store B holds 69 captures for the paper
      owner, exactly the 69 new items the schedules recorded (WHO 20, BBC
      22, Al Jazeera 27), with no body bytes in any row and 69 body files
      (312 KB) on host disk. Re-polls wrote nothing (repeats and 304s). DW
      returned zero items on every poll (RSS 1.0, fixed in f84c29bd after
      the run); France 24 (403) and UN News (timeout) failed every poll and
      backed off by doubling. corpus-dolt stayed at 36 to 50 MB; corpusd's
      anonymous memory is 32 MB (its 530 MB cgroup figure is reclaimable
      file cache). Store B's Dolt history is three commits with transaction
      commits off.
    conjecture: W1-3 supported over one hour, one paper computer, six sources
  - id: W1-read
    at: 2026-10-11T13:08Z
    deploy: staging ebe02b17 (host and both test computers)
    kind: deployed proof (reader document 69076219 on demo-arxiv-2026-10-10c)
    result: >-
      A second computer, owned by a different user (demo-arxiv-2026-10-10c),
      read a capture the paper computer wrote: research called
      choir.ReadCapture on the WHO capture key-04d282d8 and returned its
      title, source id official:who:news, URL, the paper owner's id and the
      first sentence of its body, read from Store B and host disk through
      corpusd (direct, not the proxy; see the correction above). On the paper computer after the same deploy, rss:dw_en's
      first poll fetched 20 items and wrote 20 new captures (zero before
      the RSS 1.0 fix). The 13:02:44 polls succeeded across the host
      deploy; the restart onto ebe02b17 closed the schedules, and four were
      started again (France 24 and UN News stopped, reason "refuses Node B").
    conjecture: W1-3 supported for the station's acceptance scope
  - id: W1-panel1
    at: 2026-10-11T13:30Z
    deploy: staging ebe02b17 (frozen 010b22f8, same code)
    kind: agentic-consensus completion panel (session scratchpad w1c/panel)
    result: >-
      Sent back. Codex named the blockers: a guest could write a
      choir.web_capture through the generic object route (a body under
      1 KiB would sit in a row; a same-owner capture could be overwritten),
      and a failed capture write lost a feed's items because the validators
      were saved first. GPT-6.1 Sol and GPT-6 Luna found that a failed actor
      commit drops the successor wake; Claude, GLM and DeepSeek that a
      schedule can sit active with no wake; GLM that a large poll can pass
      corpusd's request bound; DeepSeek that guests reach corpusd directly,
      which corrected an earlier diagnosis. Recorded in
      problems/w1-panel-capture-boundary-and-lost-items-2026-10-11.md and
      fixed in 726aa392.
    conjecture: W1-3 held for the observed runs; the panel found paths outside them
  - id: W1-fix
    at: 2026-10-11T13:58Z
    deploy: staging 726aa392 (CI 38144278621; corpusd and proxy report it; paper computer restarted onto it)
    kind: deployed proof (session scratchpad w1c/panel2-deployed.jsonl)
    result: >-
      The restart onto 726aa392 (no planned marker) closed the four active
      schedules as interrupted_by_restart; the list now carries an overdue
      flag (false for all). Restarted at five minutes at 13:52:10, each
      schedule polled at 13:52 (20 fetched, 20 repeats, nothing new written)
      and again at 13:57, then armed 14:02. On the second poll WHO and Al
      Jazeera answered 304: the validators were kept after the successful
      write. From Node B loopback, an internal generic POST of a
      choir.web_capture carrying a body got 400 ("a capture's body belongs
      in host content storage, not the object row"); Store B holds no row
      for the probe owner. Store B still holds 92 captures with no body
      bytes in rows and 92 body files. The guest-refusal and commit-failure
      paths are proven by unit tests only.
    conjecture: W1-3 supported with the first-round fixes
  - id: W1-panel2
    at: 2026-10-11T14:30Z
    deploy: staging 726aa392 (frozen 1ddb0606)
    kind: agentic-consensus completion panel (session scratchpad w1c/panel2)
    result: >-
      Sent back on one blocker, found by Codex, GPT-6 Luna and GPT-6.1 Sol
      (Sol reproduced it against the handler and the Dolt adapter): two
      computers of one owner posting the same new item at once could both
      pass the existence check, and Store B's upsert let the later replace
      the earlier computer's capture. Claude and DeepSeek approved; Devin
      gave no verdict (it stopped at a permission prompt); Gemini and GLM
      timed out. The first-round fixes held for every reviewer. Fixed in
      56ec63e0 with an insert-only create; the new regression fails on the
      old path and passes ten runs on the new one.
    conjecture: W1-3's ownership claim held sequentially, not concurrently
  - id: W1-fix2
    at: 2026-10-11T15:03Z
    deploy: staging 8031258a (CI 38149043966; corpusd reports it, deployed 15:01:41Z)
    kind: deployed proof
    result: >-
      The first polls after the deploy went through the insert-only create:
      BBC wrote 1 new capture and counted 19 repeats, DW and Al Jazeera 20
      repeats, WHO a 304. Store B holds 102 captures for the paper's owner
      with no body bytes in any row, and 101 body files (two items share a
      body, stored once by hash). No schedule is overdue. The concurrent
      race itself is proven by the regression against the Dolt adapter,
      not on staging, where one computer polls.
    conjecture: W1-3 supported with both rounds' fixes
  - id: W1-panel3
    at: 2026-10-11T15:13Z
    deploy: staging 8031258a (frozen 66b96f10)
    kind: agentic-consensus completion panel (session scratchpad w1c/panel3)
    result: >-
      Approved. Codex, Claude, DeepSeek, Gemini, GLM, GPT-6 Luna and GPT-6.1
      Sol approved with no blocker; Devin gave no verdict (it stopped at a
      permission prompt, as in round two). Every reviewer found the
      insert-only create closes the same-owner race. Suggestions are named
      as residuals in the now card.
    conjecture: W1-3 supported; W1 complete
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
