# More research is slow, hidden from the desktop, and reports once (2026-10-21)

Status: open. Problem documentation first; no fix in this commit.
Found by: the owner on a phone, 11:21 UTC ("I tried to use more research
and it said it couldn't ask now"), and the traces below.
Mutation class of the fixes: orange (guest store query, frontend, prompts);
the Texture skip touches the desk handoff and is red.

## Owner direction (2026-10-21, this session)

- "More research should be a button in the texture toolbar. And the
  toolbar should be responsive for mobile — currently it overflows with
  large text."
- "Yes, it should skip texture. But research should also be faster, and
  send iterative reports to texture, not wait until after multiple
  searches. (This is our ordinary policy)"
- "How is corpus search so slow? That's a bug to investigate and fix."
- The owner does not like the standalone `/autopaper/<story>` page.

## Evidence

Traces exported from the paper with `choir trace export` (bundles kept
outside the repo): the Apu story (trajectory `dbe27f57…`, ask
`rx-c4e206c940e3`, 04:46:58) and the France protest story (trajectory
`94dd4735…`, the owner's ask `rx-04797dbb765e`, 11:21:33). Paper console
read from the host, read-only.

### E1. Corpus search takes 30 to 90 seconds, and the story half is all of it

The paper's `corpus search` log lines for 2026-10-21 (272 searches):
`stories_ms` p50 32.0 s, p90 67.8 s, max 194.2 s; `captures_ms` is
0.2 to 1.2 s. 165 of the 272 searches (61%) returned no story at all.

In the two asks: France 53.5 s of research's 108 s first round; Apu 87.5 s
of research's 156 s, after a first attempt was killed by the 10 s cell
timeout (the host-side search kept running for 36 s and found nothing,
"captures not searched": its context was cancelled).

Cause, from code: `store.SearchDocuments` (`internal/store/texture.go`)
loads every Texture document object (2,752 on the paper today), matches
titles, and then, for every document whose title did not match, reads its
head revision one object at a time and checks that the content contains
every query term. Research queries carry five to eight terms, so the
all-terms match almost never hits, and the loop visits every story. It
is a full scan of 2,752 serial object reads per query, about 12 to 30 ms
each on a busy paper, with no index and no ranking. Several research
desks search at once (three searches ended together at 04:50:02), so
they also contend.

This was named and deferred on 2026-10-14
([corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md](../archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md):
"`store.SearchDocuments` … scans every Texture document in the guest:
guest-side, not touched here"). The paper has grown since.

### E2. The button is only on the standalone page

`AutopaperExpand` is mounted only in `AutopaperRoute.svelte`'s header
(the public `/autopaper/<story>` page, from 2026-10-13, commit 7584664d2).
On the desktop a story opens as a Texture window (`AutopaperApp.svelte`
`launchapp`), which has no control. The owner reads in the desktop and
never saw the button.

### E3. The phone showed "Could not ask just now" for an ask the server accepted

Caddy logged the iPhone's `POST /api/platform/publications/expand` as 202
after 11.56 s; the proxy logged `outcome=asked`; the follow-up read
started about a second later and showed `asked:true`. The deployed bundle
returns that notice only when the status is not 202 with `asked`, or when
`fetch` throws. Cause unknown. Hypotheses: the phone's request failed on
its side after the server wrote the answer (11.6 s wait on 5G), or the
body read failed. The 11.6 s is itself three sequential calls to a busy
paper (document read, chain check, revise).

### E4. Texture spends a turn deciding to ask research

Every ask opens with a Texture turn that reads the story and casts
research: Apu 27 s (04:47:33 to 04:48:00, of which 11 s before the first
model call), France 30 s (one compile error in the cast packet, then a
retry). The reader already asked for research.

### E5. Research reports once, at the end

The research overlay says "send interim packets" and "work iteratively",
but in both traces research sends one packet, after every search and
read: Apu at 04:50:33 (2.5 min in), France round 1 at 11:24:02 and
round 2 at 11:25:55. Texture can do nothing until then.

### E6. Waits between desks

- Ask to Texture run: Apu 35 s (04:46:58 to 04:47:33), France 10 s.
- France: research's second report at 11:25:55, its run completed at
  11:27:17 (76 s after its last model call), and Texture's next model
  call was 11:30:04. About four minutes with no desk working on the
  story. The console shows many 1 to 1.8 minute `desk_go_eval` calls in
  that window (other stories' corpus searches) and no wake line for this
  story. Cause unknown; hypothesis: the run's completion write and the
  report's wake wait behind the scans in E1.

### E6 again, on the research-first path (2026-10-21 13:28, after 73b84cb78)

A logged-out ask on the quiet sumo story (`rx-88c4b74b2040`, from v5):
research's run started at 13:28:51, 15 s after the ask, with no Texture
turn before it (E4 is gone). It sent interim packets at 13:29:45, 13:30:12
and 13:30:38 and the final Report at 13:31:34 (E5 is gone: the first packet
came 50 s after the ask). All four records reached `texture:db96d4ac…`
(inbox head 8, cursor 4). Texture's run did not start until 13:48:34, 19
minutes after the first packet; it committed v6 at 13:49:37. The ask took
21 minutes, and about 19 of them were the wait for Texture's wake.

What the ledger shows: the wake outbox had 0 unprojected rows at 13:40, so
the wake left the outbox; the obligations surface showed 162 actor updates
due (45 of kind `inbox_record`), 718 deferred, and 6 runs running. The
paper ran Texture turns for other stories throughout (the recent runs list).

Hypotheses, not findings (no trace yet shows which):

- H1: the actor dispatcher's 8 activation slots (`MaxConcurrent`, default
  8, `internal/actor/dispatcher.go`) are held by activations that run a
  whole desk turn synchronously (`ExecuteActivationSyncChecked` for a
  reactivated Texture run, `internal/actorruntime/handler.go`), so a due
  update waits for a slot behind minutes-long turns.
- H2: `PendingAgents` has no order (`SELECT DISTINCT … WHERE processed_at IS
  NULL`), and each pass fills the slots from the head of that list, so some
  actors can wait many passes. The run mix since 12:25 is not skewed by
  doc-id prefix, so this is not shown.
- H3: deferred updates whose backoff expired re-enter the due set and take
  slots, retrying and deferring again.

Instrument before fixing: the dispatcher logs an activation whose oldest
due update waited 30 s or more, with the slots in use and the due count.

### E6 measured (2026-10-21 14:07 to 14:33, c19db6c09 and 01c469c24)

The instrumentation shipped in c19db6c09. Its lines, read from the paper's
observability ring:

- Every `dispatcher: activation wait` line reads `slots=8/8`, with 42 to 59
  agents due. A slot was never free.
- The waits rise with the agent id. Between 14:09 and 14:17, the logged
  Texture desks came in ascending id order (`texture:0f4f…`, `167c…`,
  `17ac…` up to `799c…`). The waits grew from under a minute to 6m33s,
  6m58s, and 19m4s for `texture:799c…`. The sumo story's desk is
  `texture:db96…`, late in that order, and it waited about 19 minutes at
  13:29. **H2 is a finding:** `PendingAgents` returns agents in index order
  (`idx_actor_updates_backlog` on `to_agent_id`), each freed slot goes to
  the first due agent in that order, and an id that sorts late waits until
  nothing earlier is due.
- **H1 is a finding:** `in_flight` showed `initial_dispatch` activations
  (whole Texture turns) in the slots. One `management:` activation folded
  72 updates and held its slot for over 10 minutes. A reader's research
  report reaches Texture through two queued activations: `inbox_record`
  (which only submits the run) and then `initial_dispatch` (which runs the
  turn). Both wait in the same order.
- The wake outbox is not the wait. `actor wake latency` lines show 2 to 15
  s from wake to actor update, plus one 1m40s list right after a boot.
- No gateway rate-limit wait appears in the log.
- H3 is not shown: `max_defer_count` is 0 on the due set.

Why the turns that hold the slots are slow: the guest store is saturated.
The embedded Dolt engine runs behind one mutex and one connection
(`engineMu`, `configureEmbeddedDoltDB`). The observability `engine_mutex`
counters, sampled 153 s apart from 14:30:39 (9 minutes after the
01c469c24 boot), show:

| Operation | Calls | Mutex held | Per call |
|---|---|---|---|
| all | | 146.5 s of 153 s (96%) | |
| `ReadObjectSnapshotFiltered` | 116 | 48.3 s | 416 ms |
| `ListObjectsByMetadataPage` | 540 | 48.1 s | 89 ms |
| `ListObjectsByOwnerAndBody` | 13 | 29.6 s | 2.3 s |
| `GetObject` | 5441 | 5.9 s held, 382 s waited | 1.1 ms held, 70 ms waited |

A `GetObject` holds the mutex for about 1 ms and waits about 70 ms for it.
A desk turn makes hundreds of store reads, so it spends tens of seconds
waiting for the store, and holds its dispatcher slot while it waits.

### E6 structural assessment

This is the fourth record on one substrate:

- [texture-list-cold-latency-2026-10-09.md](../archive/problems/texture-list-cold-latency-2026-10-09.md);
- embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md;
- [guest-cpu-is-unindexed-store-scans-2026-10-14.md](../archive/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md);
- this one.

The substrate is the object graph used as a JSON document store, read by
scan-and-filter under one engine mutex. The dispatcher turns store
saturation into wake delay: slow turns hold the 8 slots, and the fixed
order chooses who waits.

Two existing replacements are not fully wired:

- The Dolt upgrade past the JSON unescape race landed on 2026-10-15
  (5b0772561). The engine mutex and the single connection, which were the
  workaround for that race, are still in place. The 10-11 record says to
  remove them "in measured steps" after the upgrade, and that has not been
  done.
- The generated-column indexes from the 10-14 record exist. The three hot
  reads above still scan. Whether each one is missing an index or bypassing
  one is not yet known.

Fix order, smallest blast radius first. Each fix is measured on the paper
before the next:

1. Dispatcher order: oldest due first (orange, actor kernel query). It
   removes the starvation tail: a wait becomes about the queue ahead
   divided by the slots, not unbounded for a late id.
2. The three hot reads (orange, guest queries): give each an indexed path,
   or stop calling it per turn. Target: mutex busy well under 50%.
3. Engine concurrency (red, store substrate): remove `engineMu` and raise
   the connection count in steps, per the 10-11 record's tests.
4. Only then, if waits remain: the double activation for a Texture wake,
   and the slot count.

A larger `MaxConcurrent` before steps 2 and 3 would add contention for the
same mutex, not throughput.

Landed (each deployed and measured before the next is judged):

- Step 1: af5ab17e4, `PendingAgents` oldest due first.
- Step 2, first reads: 381825279, `GetCommitmentRecord` by key (it was the
  2.3 s body scan); b1edcd9bf, the lifecycle snapshot through the
  trajectory index, with its document and revisions read by key (the 416
  ms read). The same commit names each metadata read's kind and field in the
  mutex counters, so the paged read can be attributed next.

Two more findings from the same paper log (14:07 to 15:17):

- The wake outbox listing is in front of every wake. A sweep after each
  signalled commit lists unprojected wakes by a JSON match on `$.projected`
  over every outbox row ever written (`ListUnprojectedActorWakes`). The
  `actor wake outbox list` lines show 2.0 to 3.9 s per listing, once 1m40s
  just after a boot, and `actor wake latency` shows 2 to 15 s from a wake's
  write to its dispatch. The store already knows which outbox rows each
  commit wrote.
- Source polls take dispatcher slots. After the 15:12 boot the waiting
  agents were mostly `source:` actors (28 sources, all due at once), each
  holding a slot for network I/O up to the 90 s poll timeout. Under the
  oldest-first order the backlog drained from 33 due agents to 11 in about a
  minute, in arrival order. A separate lane for I/O-bound polls is a
  candidate, not yet decided.

Measured after c038df7c6 (all of the above, plus the source-poll lane and
the lazy Report list) on the paper, 2 vCPUs, 155 s from 15:50:40, five
minutes after boot: the engine mutex was held 70% of the time (from 96 to
100%). The indexed snapshot read costs 9.4 ms (115 calls); the body scan for
`$.record_id` and the outbox listing are gone from the top. What holds it
now, named by the new counters:

| Read | Calls | Held | Per call |
|---|---|---|---|
| body `$.addressee` on commitment records (Texture's evidence sources, per turn) | 17 | 27.7 s | 1.6 s |
| `choir.run` by `$.agent_id`, JSON (`GetLatestActiveRunByAgent`, per wake and run end) | 121 | 22.6 s | 187 ms |
| `choir.capture_label` by `$.capture_id`, JSON | 276 | 14.9 s | 54 ms |
| `ListObjects` (unlabeled) | 9 | 10.8 s | 1.2 s |
| `choir.source_ref` by `$.doc_id`, JSON | 4 | 4.1 s | 1.0 s |

Each is an equality on a field with no index, the same gap the 10-14 record
named. The repair is the existing one: kind-scoped STORED generated
columns, built after the tape replay behind the health gate, each read
keeping its JSON path until its column is live. Its cost is one table
rewrite at the first boot (about 50 us CPU per row per column, measured
10-14); after that the engine computes the columns on every write.

### E9. Texture's Write cites with `{{ref N}}` and is refused

In the same run Texture's first Write was refused: "`{{ref 1}}` names no
source of this document; `{{ref}}` only keeps a citation the frame shows"
(three times). Its second Write committed, and v6 cites the same three
sources as v5: research's newly read Japan Times source did not reach the
story. A failed tool call for the dual-model evaluation (owner notes
section 35): check whether deepseek-v4.1-flash makes it too.

### E7. Texture asked for a second round

France: Texture judged the first report thin ("The first research pass
produced no readable, non-d…", 11:24:34) and cast again. Two of research's
reads failed (Guardian live page, a prefecture PDF). Reasonable judgment,
but it doubles the research time.

### E8. Malformed packets cost turns

Texture's cast and research's report each failed to compile once or
twice on map-literal syntax (France 11:22:08, 11:25:32, 11:25:43), five
to ten seconds each.

### Totals

Apu: ask 04:46:58, published 04:51:55, about 5 minutes. France: ask
11:21:33, v14 published 11:30:30, about 9 minutes. Model time is a small
share: research's model calls are 1 to 3 s each at low effort.

## Fix plan (each lands after this record)

1. Corpus story search (orange, E1): replace the scan with a ranked
   lexical index (BM25) over title and head text, kept in the guest
   runtime, built in the background at boot and updated when a revision
   commits; the search answers from titles until the index is built.
   Embeddings stay behind the 2026-10-14 ladder's criterion unless the
   owner overrides it.
2. The button in the Texture toolbar for published stories, and a
   responsive toolbar that wraps at phone width and large text (orange,
   E2). Landed on branch research-more-toolbar (owner, 2026-10-21: "Get rid
   of the standalone page"): the button is in the toolbar of a published
   story's Texture window, `AutopaperRoute` is deleted, old `/autopaper`
   links are replaced with `/pub/texture/<slug>[/v<n>]` (and `/autopaper`
   with `/`), and the digest email links to `/pub/texture/<slug>`. A story
   opened at its public Texture route has no Autopaper window context, so
   the resolve bundle now names an Autopaper story (`publication.kind`
   `"story"`) and the reader shows the AI-written disclosure from that.
   Follow-up (owner, 2026-10-21: "The toolbar should have a '3 dots' menu
   that expands if the content is too wide for the screen"): the wrapped
   toolbar took about 540 of 812 px at 200% text, so it is now one row. The
   version controls stay; the other controls have a priority and, measured
   against the row (ResizeObserver, no breakpoint), the lowest-priority ones
   move as the same DOM nodes into a "⋯" menu at the end of the row. A story's
   order is More research, Edit, Sources, Copy text, Download; a private
   document keeps Revise, then Publish, then Sources, and Cancel or Restore
   when they apply. The fate line is a status row under the controls.
3. The ask goes straight to research, which reports to Texture (red,
   E4); research sends a packet as soon as it has a read source and
   finishes with the final report (prompt, E5). Built to
   [the design](../design/research-more-goes-to-research-2026-10-21.md):
   the proxy calls the paper's `POST .../research`, which casts the
   story's research desk with the story text in one store transaction
   (`CommitStoryResearchRequest`); Texture's frame says a reader asked
   while the request is pending; a request a crash closed reads
   `interrupted` and its version may be asked once more. The proxy no
   longer sends a prompt, so an owner's ask and a public reader's ask
   are the same request at the paper (the provenance line is gone).
4. E3, E6: instrument before fixing (client error kind on the notice,
   wake latency per report).

## Heresy delta

- discovered: E1 to E9; E6 measured: the dispatcher order (H2) and slots held by whole turns (H1), over a store engine 96% busy.
- introduced: none.
- repaired: none.
