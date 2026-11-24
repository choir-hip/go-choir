# The system map: what runs today

Facts read 2026-11-22 (the paper's store measured 2026-11-23). Numbers in
brackets are the map each fact comes from, [1] to [8]; [CL] is CLAUDE.md,
[CH] is docs/changes.md, [D4] is draft 4 of the first design. A fact marked
"later" supersedes the map it sits beside.

The product is the automatic computer: one persistent computer per owner
that is meant to change its own software under supervision [CL]. What runs
in production is one host (Node B, choir.news) with four owner computers
(3 active, 1 hibernated) [1], and the one that matters is "the paper": a
computer that polls about 250 sources and publishes cited English news
stories with no human in the loop [4]. The phone app (Autoradio) reads that
paper aloud; it is on one phone as a second, side-by-side install and is
not released [6][CH].

## 1. The system in one page

### Parts

- One host, Node B. Caddy takes 80/443 and sends `/auth/*` to auth (:8081),
  `/api/*` and the rest to proxy (:8082), `/health/*` to gateway (:8084)
  [1]. It refuses `/provider/*` and `/internal/*` [7].
- Host services: auth (passkeys, sessions, API keys), proxy (the one public
  API gate), vmctl (:8083, computer lifecycle), gateway (:8084, the only
  path to model providers; guests hold no keys), corpusd (:8086, the
  Autopaper object graph and platform store), maild (:8087), checkpointd (a
  timer, not a daemon), PostgreSQL 17 with databases `control` and `graph`
  [1][3][7].
- A computer is a Firecracker microVM plus its own data image holding
  `state.db` (SQLite). Its identity (`computer-<hex>`) is stable; the VM is
  replaceable [1]. The guest runs `cmd/autoputer`: the agent runtime, the
  desks, the web app's server side [3].
- Desks are model agents: management, Texture, research (with two child
  roles), engineering, plus the `autoradio` package desk (a manifest and two
  routes, no cell) [3]. A desk wakes on a record or deadline, gets one model
  turn, and writes a short Go "cell" that stages intents; the host runs the
  cell confined, then commits what it staged [3].
- The tape is the computer's hash-chained event log; `state.db` is a
  rebuildable projection of it. The host keeps tape segments on disk and
  heads and receipts in Postgres [2].

### One thing through the system: a source to a spoken sentence

1. A source (253 registry rows: 179 rss, 73 telegram, 1 gdelt, plus
   owner-added feeds) is a chain of durable `source_poll_due` wakes, one per
   source. Default 15 minutes; failures double the wait up to 24 h [4].
2. The poll fetches and parses the feed, fetches the article (2 MiB cap) and
   stores a capture: bodies on the host, one object row, a text index, a
   256-dim vector [4]. Research's own web reads are captures too, and were
   65% of 72,993 captures in the week of 11-15 [4].
3. Intake label: one Jev job per flush asks, for each capture, which story
   or capture reports the same event and which story shares its topic. A
   minute later the event layer joins captures to world events with one
   more call [4].
4. One arrival record per poll mints a management task: up to 12 captures,
   4 at once (max 8), 10 minute deadline; queued tasks start oldest first,
   never expire (a crash boot closes what is queued; a planned restart resumes it), and run in their own dispatcher lane. The task sees stories updated in
   8 h and up to 12 candidate stories per capture, and must Feed a story,
   Decline, or open a new one with a main point (20 to 240 chars) [4][3].
5. Texture, the sole writer of a story, writes a revision and ends the turn
   with Done, Wait, or a Cast to research [4][3].
6. Research starts from the paper's graph (a host-built digest of up to 40
   held pages, then the web), returns a Report with claims and gaps, and
   wakes Texture again. Each new citation is judged by Jev [4].
7. Publish gate, per revision: held if there is a private embed, no cited
   source, fewer than 2 distinct outlets, no headline, under 100 words
   (once), not English, or the disclosure gate refuses. A hold is not a
   drop; the next passing revision publishes [4].
8. A host timer runs `corpusd front-importance` every 10 minutes: Jev scores
   importance and 10 yes/no sections. `GET /api/platform/publications/front`
   lists stories newest first, with no sign-in [4][7].
9. The phone fetches the front page with the owner's key, keeps it offline,
   and POSTs `/api/autoradio/programme`; the answer's entries go at the end
   of the run sheet. Shared code hands the native deck up to 12 held
   entries (spoken text or an audio url); the deck plays them, speaks with
   an on-device voice, and keeps playing while shared code is frozen [6].
10. A hands-free voice turn: mic, voice-activity check, recogniser (the
    best listening models on the phone from the model list, Silero and
    Fluid Speech Mini with Parakeet live words; Apple's recogniser and the
    energy detector until they are downloaded), turn policy, then plain-word queue commands (no model) or a first answer
    from an on-device model and a streamed `POST /api/autoradio/turns`
    (queue plus 40 candidate stories), spoken sentence by sentence [6].
    On `lane/ask-search` (not deployed) the turn first searches the paper
    for the words (one public read, keyword plus vector) and adds the top
    10 stories to what the model sees; on main it sees 40 titles only.

Cost of steps 1 to 8 on 2026-11-22: about 9.3 model calls per capture
judged, about 23 per story woken, 31.2 per publish [3] or 58 per distinct
story published [4] (different denominators). One window (20:30 to 00:00Z):
4,069 calls for 438 captures [4].

### One change: a commit to a computer

1. Push to main. `ci.yml` (about 25 to 30 jobs) plans, runs gates and Go
   shards, then `deploy-impact` classifies the commit [1][8].
2. `deploy-staging` runs `node-b-ci-gate deploy`, then
   `.github/scripts/node-b-ci-deploy` as root on Node B: the commit must
   descend from the gate floor; disk preflight (60 GiB free by default);
   build the NixOS closure and guest images; switch; install the frontend
   and host services; restart selected services [1].
3. Deploy refresh: vmctl asks each active guest to write its own
   planned-restart marker and restart. A busy guest answers 409 and is
   retried by a sweep [1].
4. Ring walk (`internal/vmctl/ring_walk.go`): R0 the drill computer, R1 the
   class test, R2 the owner's computer, R3 everyone else, R4 public
   platform, R5 the paper last. Computers outside the open ring stay pinned
   to the last release that finished a walk. Four guards (readiness,
   appender wedge, boot success, crash restart) pin back on a breach [1].
5. A `Ring-Expedite:` trailer or a touched protected path skips the soak
   floors; the guards stay [1][CL].
6. Boot: a fresh marker (under 15 min) with the same work-state epoch
   resumes work once; anything else is a crash boot and open work is closed
   "interrupted by a restart" [1].
7. Self-development is a second route, per computer (section 6); almost
   no change goes that way today [5].

## 2. Computers, host and deploy [1]

A computer is owned by an `ownerships.json` row. vmctl owns lifecycle:
boot, hibernate (10 min idle, 2 min sweep), wake, remove. A wake resumes
the snapshot only if floor digest, security epoch (128) and ABI match and
the release is inside a 168 h window; else a cold boot. Start is refused
under a 32 GiB disk floor or when memory admission fails; "make room"
hibernates up to 3 idle computers. A computer made for a new account is
4 GiB (recorded as `mem_size_mib`; older computers keep theirs). Sign-up is
open only while the host can hold one more computer: free disk over a 100 GiB
reserve, less the gap to 2 GiB for each computer that holds less (image and
memory snapshot, as exact bytes on disk, btrfs compression seen; anything not
exactly measured counts 0), is at least 2 GiB. An account with no computer holds
nothing (owner, 2026-11-24), so sign-ups that finish before their computers
exist are bounded only by the free reading. The auth service decides it from
its own `statfs` and the file vmctl writes every 30 s, and closes when either
is missing or stale, or when no plan is configured (`internal/hostroom`). The
public Pulse says only open, closed or no reading; the figures are on the
platform operator's `/api/platform/operator/capacity`. The 2 GiB is what a real
user's computer takes (owner: 0.5 to 2 GB; the owner's own are 12 to 87 GB). No account is exempt.

Key paths: `internal/vmctl/`, `internal/vmmanager/`, `internal/proxy/`,
`internal/auth/`, `nix/node-b.nix`, `.github/scripts/node-b-ci-deploy`.

Numbers: ring-walk history has 380 walks started, 292 complete, 2 rolled
back; 294 `expedited` events (77%) [1]. 267 files in `deploy-failures/`,
unclassified [1]. Node B disk: 160 G free of 476 G (66% used) on 11-23 [1].

Not true, though a reader would assume it:
- "Automatic rollback" is true for guest releases (the ring walk), not for
  host services or the frontend; those keep `*-previous` directories only
  [1].
- The host does not write the planned-restart marker; the guest does, at
  vmctl's request [1].
- CLAUDE.md's "deploys refresh idle computers" omits the ring walk, which
  decides what a deploy may touch [1].

## 3. The store, tape and snapshots [2]

Each computer has one `state.db`. Writes go through the appender
(`internal/computerevent/appender.go`): pin the payload on the host,
stage local rows, compare-and-swap the head on corpusd, verify the receipt,
finalize. If the projection head differs from the host head the append
refuses. A snapshot producer ticks every 10 minutes; a snapshot is cut into
4 MiB chunks, sealed and uploaded, and bounds a rebuild's replay to at
most 720,000 tail events.

Key paths: `internal/store/`, `internal/computerevent/`,
`internal/snapshot/`, `internal/agentcore/snapshot_producer.go`,
`internal/recoveryplan/`, `internal/tapestore/`, `internal/platform/`.

Numbers, the paper [2]; store composition is later [CH]:
- Store 49.7 GiB on 2026-11-23, growing about 2 GiB a day [2][CH]. Of it,
  29.4 GiB is stored values: object bodies 15.7, run memory 7.3 (2.4 older
  than a week), headers 5.6; about 20 GiB is index and page overhead of 8
  million small objects. Superseded objects 0; duplicates 0.24 GiB [CH].
  This replaces map 2's "36 GiB unattributed".
- Host also holds 89 GiB tape, 34 GiB snapshot chunks, 41 GiB Postgres (28
  GiB receipt table, about 12 live), 41 GiB capture bodies [2].
- Capture of 2026-11-22: 51.3 GiB, 66 min (copy 7 min, whole-store witness
  55 min, upload 4 min); 79% of chunks new a day later [2].
- Recovery proved only on the drill account: lost-disk rebuild 2026-10-30
  (1,007 of 1,007 files); in-place rematerialize 2026-11-15, witness
  matched, 283 s; its store is about 1.1 GB [2].

Not true, though a reader would assume it:
- No capture writes the page-digest (v2) record; only the reader landed
  (`9e3e2060f`). Capture still writes v1 [2][CH].
- There is no "superseded versions" history in the guest store; a rewrite
  overwrites the row [2]. The reading found 0 superseded objects [CH].
- The weekly layout compaction trigger is deleted (`06c49c1b8`); compaction
  runs only for free pages over 10% or 2 GiB, or when forced [2][CH].

## 4. The agent runtime and desks [3]

The dispatcher (`internal/actor/dispatcher.go`) reads the tape for due
wakes and runs one serial, epoch-fenced activation per agent. 12 shared
slots; a lane for sources and one for the engineering admission mailbox.
A desk's only tool is `desk_go_eval`; verbs in the cell stage intents in a
tray and the reducer (`internal/agentcore/rlm_reduce.go`) commits the tray
atomically. A failed cell commits nothing. Texture alone advances a
revision head (`store.ApplyTextureTurn`). Each data kind has one custodian,
checked at load (`internal/objectgraph/custody.go`).

Key paths: `internal/deskmanifest/manifest.go`, `internal/actor/`,
`internal/actorruntime/handler.go`, `internal/agentcore/tool_profiles.go`,
`internal/agentcore/rlm_reduce.go`, `internal/yaegikernel/`,
`internal/modelcatalog/catalog.go`, `internal/gateway/`.

Numbers: 3 h window on 11-22: management_task 393 calls, management 93,
texture 1,187, research 1,220 [3]. 637 Jev calls in 3 h; 0 upstream 429s in
24 h [3]. Wake waits up to 2m47s seen with slots 12/12 and 33 agents due;
one research wake waited 15m39s [3]. Management task wake carries 26 to 40
queued `cell_terminal_deadline` wakes [3]. Models: `devin/swe-2-flash`
carried 1130 of 1173 research calls and 1072 of 1177 texture calls; its
catalog entry says free until 2026-12-04 and uses one person's session [3].

Not true, though a reader would assume it:
- "Every desk is a package desk registered the same way" is not built: five
  system desks are a Go table; `required.json` holds only `autoradio` [3].
- "Desks talk only through records" does not hold: Research, Feed, Decline,
  Delegate and host-minted arrival Casts are direct host actions [3].
- Engineering made no model call in 24 h [3]. Mail desk: no call seen [3].
- The act gate and trust stroke enforce only third-party app desks; none
  exists [3].
- Management has two live control planes: records and the older
  coagent-packet lifecycle; the share each carries is unknown [3].

## 5. The Autopaper pipeline [4]

The flow is in section 1. Corpus search is FTS5 text plus pgvector plus
graph rank; vectors: 38,519 stories and 250,822 captures, none pending [4].
Entities: 78 export lines in 3.6 h, a hand read found 29% not entities and
24% duplicates [4]. Publishing: about 40 publishes an hour; in 3.6 h there
were 144 publishes of 70 stories and 173 holds, about 84 of them for fewer
than 2 outlets [4]. Intake: 63 of 121 flushes held one capture [4].

Key paths: `internal/agentcore/source_schedule.go`,
`intake_labels.go`, `intake_job.go`, `event_layer.go`,
`management_tasks.go`, `research_digest.go`, `story_publish.go`,
`story_publish_quality.go`; `internal/researchtools/poll_source.go`;
`internal/sources/registry.json`; `internal/platform/publication_front.go`;
`internal/frontimportance/`.

Not true, though a reader would assume it:
- No durable record joins a capture's id to management's choice for it on main: the choice is a console line (about two hours kept). Lane `decision-record` (not deployed) writes one `choir.decision_record` row per capture and stage, read at `GET /api/decisions`.
- The redesign (subjects, mentions, predictions that name a subject) is a
  design only; its offline test failed all four kill lines (C192, false).
  Draft 4 says the hot path as drawn is not proved [4][D4].
- The old root desk is built and off (`SettingTextureRootDesk`, default
  false); later, the first design supersedes it and deletes it [4][D4][CH].
- No personal edition is computed; "My edition" is now the notebook's
  title [4].
- No story successor links exist; the "next" judgment was answered 0 times
  [4][CH]. Nothing matches an arriving capture to a prediction; nothing
  settles on expiry; Texture is never shown its score [4][D4].
- Graph-first research did not cut web searches (C191 false: 2.85 per run)
  [4][CH].
- The podcast worker runs only while a Mac session runs [4][7].

## 6. Self-development [5]

An owner request reaches Texture; management casts engineering; engineering
edits a copy of the source in a capsule (`internal/capsule`); `choir.Freeze`
builds a hash-addressed bundle (`internal/selfdev` operation, 13 states, one
open per computer); management Verifies; corpusd decides under policy
`reversible-selfdev-v1` from two ballots (management's guest-signed verdict
and a host build); the updater installs it. The owner's act is revert, or
reject before the decision. Pre-use approval was deleted 2026-11-06. The
paper is refused by rule (guest and corpusd, both) [5].

Key paths: `docs/design/selfdev-apply-and-revert-2026-11-06.md`,
`internal/selfdev/operations.go`, `internal/agentcore/freeze_candidate.go`,
`self_development_standing_consensus.go`,
`self_development_materializer.go`, `internal/platform/standing_consensus.go`,
`internal/updater/`.

Not true, though a reader would assume it:
- "Texture supervision" does not approve or pause a candidate; it shows the
  round and holds the Revert button [5]. The Revert button was never seen
  working in a deployed proof [5].
- Choir-in-Choir, fork and candidate VMs are not built [5].
- Default mode is `apply` for every non-platform computer with no row,
  including the owner's [5].
- This repo's own changes do not go through it: coding agents work in lanes,
  `scripts/land` and a push to main, with no capsule, Freeze or owner-act
  record. Bridge `scripts/selfdev-promote` last used 2026-10-26 [5].

## 7. The phone app [6]

Autoradio is an Exact app (pinned, pre-1.0): shared TypeScript and Contract
pages in `mobile/AutoradioLab`, a native module in
`mobile/exact-package/apple`, and `mobile/AutoradioKit` Swift. Rule: only
voice is native [CL]. Three code states exist:
- main: logic still mostly Swift. 94 non-voice Swift files, 17,664 lines [6].
- `lane/exact-train1` ("the train"): 109 commits ahead, moves most pages to
  shared code (non-voice Swift 46 files, 6,224 lines). Not landed; its own
  real-host proof had 7 MISSING checks [6][CH].
- `lane/account-partition-s1`: sign-in and account wipe in shared code; 12
  review rounds; real-host run had 18 MISSING checks [6][CH].

Later [CH]: "Autoradio 1", the train as a second app beside the first, has
been on the owner's phone since 2026-11-23 01:33Z, with seven defects
found there. Map 6 had the owner's phone on the 2026-11-20 build
(`bfc1492c6`); that is the older install.

Key paths: `mobile/AutoradioLab/app.ts`, `radio-runtime.ts`,
`radio-bridge.ts`, `mobile/AutoradioKit/`, `docs/reference/exact/deck.md`,
`scripts/check-autoradio-native-boundary`, `mobile/scripts/proof-real.sh`.

Not true, though a reader would assume it:
- Hands-free voice has never been spoken to on a device [6].
- On-device models: download state on the owner's phone is unknown [6].
- Tests passing on a fake host do not predict the real host (the round-11
  failure was a Keychain error no unit test could show) [6].

## 8. Web, APIs and surfaces [7]

A Svelte single-page desktop of 24 base packages, served by each owner's
computer from its own release; signed-out visitors get a small host shell
(`frontend-current`) and public pages (`/pub/texture/<slug>`, `/read`).
The proxy answers platform routes and forwards other `/api/*` to the
owner's computer with identity headers it alone sets. The `choir` CLI
(about 55 commands, also a stdio MCP server) drives the same API [7].

Key paths: `internal/proxy/handlers.go`, `internal/apihandler/routes.go`,
`cmd/choir/commands.go`, `frontend/src/App.svelte`, `internal/maild/`,
`nix/node-b.nix`.

Not true, though a reader would assume it:
- There is no web page that shows a run or trace; the routes and `choir
  trace` exist, the page does not [7]. `choir trace run <run_id>` (lane
  `trace-run`, not landed; [reference/TRACE-RUN.md](reference/TRACE-RUN.md))
  shows one run step by step on the client.
- Package publish and install have no deployed proof I could find [7].
- Public routes (front, search, resolve) have no rate limit in the proxy or
  Caddy [7].
- `routekinds.Gate` counts a foreign caller and lets it through for
  built-in kinds [7].

## 9. Process, docs and learning [8]

A lead (a Claude session) fans builders into git worktrees ("lanes"), lands
trains onto main through `scripts/land` (cherry-pick, build, vet, gates,
whole suites on the build VM, push), and CI deploys to staging. Memory is
CLAUDE.md (301 lines), 1,692 files under docs/ (43 MB), and a calibration
ledger of the lead's own claims (C20 to C198) [8].

Key paths: `scripts/land`, `scripts/gates`, `docs/calibration/claude-ledger.md`,
`docs/current-architecture.md`, `docs/ACTIVE.md`, `docs/changes.md`,
`docs/definitions/choir-standing-metamission-2026-11-22.md`.

Not true, though a reader would assume it:
- No single list of live work exists; ACTIVE.md (667 lines), the mission
  graph (36 "working" nodes, 24 pointing at archived files) and the roadmap
  disagree [8].
- Problem records are not closed when fixed (6 of 30 sampled "open" ones
  were fixed or moot) [8].
- Review output is not stored (`.agentic-consensus/` is gitignored), and a
  trailer's "review N" resolves to nothing [8].
- `docs/current-architecture.md` rows are stale in places: the F0 desk-on-
  call row, the graph-first row, the main-point row, "publishes hourly",
  and the self-development row [3][4][5][8].

## 10. Where the maps disagree with the docs or each other

- **Design documents: the newer supersedes the older where they conflict** (owner, 2026-11-24). Newest first: `docs/design/desks-and-engine-2026-11-24.md` (the current design: the desks, the capture desk, the RLM substrate, each part marked built or not); `docs/design/ideal-form-draft-2026-11-23.md`; `docs/design/first-design-2026-11-22.md`; `docs/design/semantic-layer-2026-11-20.md`. Each older file says at its top what of it is superseded.

Each is open. Both readings are given.
1. Off-host backup. [1] saw a daily encrypted `offhost-backup` timer that
   ran at 02:34Z on 11-23, and read no restore-drill result. [2] says tape
   and snapshots have no off-host copy (owner: "we will offload the tape
   soon") and did not check whether the pg_dump/restic backup runs. Open:
   what the backup covers, and whether any restore of it has run.
2. Self-development status. [5] says the ledger shows a pass proof, revert
   proof and drills on 11-08 and 11-09, so the true line is "re-proven,
   untouched since"; current-architecture says "unknown whether it
   resumed"; [3] saw no engineering call in 24 h; ACTIVE.md says "paused
   2026-11-04" [5][8]. Open: whether it works today.
3. Cost per story. [3] gives 31.2 calls per publish and 22.5 per story
   woken; [4] gives 58 per distinct story published and 23 per story woken.
   Windows (21:00 to 00:00Z against 20:30 to 00:00Z) and denominators
   differ; not reconciled.
4. Which database is "Store B". [1] calls corpusd's `control` Store A and
   `graph` Store B; [2] and current-architecture say Store B is deleted
   (`vector_store.go` still says it is the authority). Open: the names in
   use today.
5. Checkpointing of the paper. [2] could not tell whether the paper
   checkpoints automatically (`checkpoint_cadence.go` "compiles off" by its
   own comment); [1] shows a checkpointd timer running. Open: the paper's
   `computer_checkpoints` rows.
6. Podcast worker. [4] says built, whether running unknown; [7] saw 0
   workers on Node B and says it runs only while a Mac session runs.
   Open: whether the Mac session runs now.
7. Which classes get `allow_busy` on a refresh. CLAUDE.md names
   `critical_protected` and `public_platform`; [1] found `allow_busy` for
   "an always-on platform computer" and `critical_protected` only in
   reclaim order. Open.
8. Docs-only commits and deploy. CLAUDE.md says they skip deploy; [1] cites
   a docs push that deployed (`22192b13b`) and [8] counts 15 of 25 CI
   failures on docs commits, at the deploy job. Open: the cause (the filter
   or the disk preflight).
9. Snapshot ceiling. CLAUDE.md and the recovery-plan comment give 540,000
    events and "about 40 minutes at 298 events/s inside a 90-minute
    deadline"; [2] found 113 events/s as the producer's worst case, a
    30-minute boot context, and no 90-minute deadline. Open.
10. The Seat 1 reviewer. CLAUDE.md carries two statements (Gemini through
    `agy`, or Sonnet); the rationale still says Devin first [8]. Open which
    is current.
11. Phone install. [6] had the owner's phone on the 2026-11-20 build;
    later [CH] puts the train on it beside the first app since 11-23.
    ACTIVE.md still says `autoradio-integ20` [6][8].

## 11. Known weak points

One line each; source in brackets.
- No host-side deploy rollback; only guest releases roll back (2 in 380
  walks) [1]. One health-read timeout marks a good install "incomplete" [1].
- No restore has run on a store over about 1 GB, none ever on the paper
  [2][CH]. Capture takes 66 min there; a v1 restore has no fallback [2].
- Restore is transient: no forward event, appends refuse until repair [2].
- One store costs about three on the host; Node B fell to 44 GiB free on
  11-22 and refused deploys and wakes [2][1]. No off-host copy of tape or
  snapshots is confirmed (section 10) [2].
- Most deploys expedite (77% of walks), so soak floors rarely apply [1][8].
- The dispatcher's 12 slots saturate (12/12, 33 agents due; waits to
  15m39s) [3][CH]. One host runs everything, no failover [1].
- Model calls: 9.3 per capture, about 23 per story woken; dollars per call
  unmeasured, 98% of calls on an unpriced model [3][4].
- Most captures are research's own reads (65% in one week) [4]; 21 of 55
  research runs had no objective; web searches did not fall [4][CH].
- Publishing is held mostly by the 2-outlet rule (about 84 of 173 holds)
  [4]. Three mechanisms ask which story a capture belongs to [4][D4].
- The paper runs on one unofficial vendor session, free until 2026-12-04
  [3]. Entities: 29% not entities, 24% duplicates [4].
- Switches and limits against the repo's rules: Jev limiter (15/s, burst
  30, 32 in flight), 12 slots, 120 wakes an hour, 11 of 20 computer
  settings default off, `CHOIR_SNAPSHOT_DISABLE`, nix allowlists
  [3][4][1][2].
- Self-development unexercised for 13 days across 1,138 commits; default
  mode is apply with no human seat [5].
- The phone's account boundary does not work; the train fails its own
  proof; no run page exists on the web; public routes have no rate limit
  [6][7][CH].
- Hands-free voice and on-device models are unproved on a device [6].
- No single list of live work; 518 lane branches, none listed; the
  mandated read is about 115k tokens [8][CH].
- Ledger 3 days stale, a quarter unsettled; review output is not stored;
  fixed problem records stay "open" (6 of 30 sampled) [8].

## 12. Where to read next

- Changing the store or snapshots: `docs/definitions/choir-storage-
  efficiency-2026-11-22.md`; `internal/computerevent/appender.go`;
  `internal/snapshot/snapshot.go` with `internal/agentcore/
  snapshot_producer.go` [2].
- Changing a desk or the runtime: `internal/deskmanifest/manifest.go`;
  `internal/actor/dispatcher.go` with `internal/actorruntime/handler.go`;
  `internal/agentcore/tool_profiles.go` and `rlm_reduce.go` [3].
- Changing the Autopaper pipeline: `docs/design/desks-and-engine-2026-11-24.md` first, then `docs/design/first-design-2026-11-22.md`
  and the offline-slice evidence note; `internal/agentcore/
  source_schedule.go` (`HandleSourcePollDue`); `story_publish.go` [4][D4].
- Changing the phone app: CLAUDE.md "Product" bullet; `docs/reference/
  exact/deck.md`; `mobile/AutoradioLab/radio-runtime.ts`;
  `scripts/check-autoradio-native-boundary` [6].
- Changing a deploy or the host: `.github/scripts/node-b-ci-deploy` with
  `internal/vmctl/ring_walk.go`; `nix/node-b.nix`; `docs/computer-
  ontology.md` [1].
- Changing self-development: `docs/design/selfdev-apply-and-revert-
  2026-11-06.md`; `internal/agentcore/freeze_candidate.go`;
  `self_development_standing_consensus.go` [5].
- Catching up on what changed: `docs/changes.md`, then `docs/ACTIVE.md`
  lines 1 to 70 and the standing metamission [8][CH].
- Checking a claim about "what exists": `docs/current-architecture.md`, but
  treat every "built, not deployed" cell as unchecked [8].
