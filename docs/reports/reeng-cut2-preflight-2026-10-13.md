# Cut 2 pre-flight: the cited facts on current main, 2026-10-13

Cut 2 (docs/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md)
was written against main 03499ef9. Its `start.observed` facts were
re-verified read-only on 76ab40fd (Cut 1 core), 104 commits later.

Unchanged files (no diff since 03499ef9): internal/search/search.go,
internal/hostlink, internal/gateway/searchplane/router.go,
internal/gateway/jev.go, selfdev_candidate_judgment.go,
internal/sourcegraph, internal/sourcefetch, internal/texturedoc,
internal/platform/{captures,publication_graph,service_publication_read,source_metadata,store}.go,
coagent_update_packet.go and UniversalWireApp.svelte. Their line numbers
still hold. Moved but unchanged: choir.WebSearch (yaegikernel/choir.go:513),
web_search (researchtools.go:620-625), FetchURL (researchtools.go:676),
HandleSearch and its 30 s deadline (gateway/handlers.go:758, :816), the
desk session key, reclaim and release (tools_desk.go:136, :232, :281), the
cell result map (tools_desk.go:467-476), BindCell Begin and End
(choir.go:170, :185-187), PollSource (poll_source.go:159), the four
ExtractReadableHTML callers (content.go:1851, article_import.go:59,
browsercontrol/handler.go:686, :782), tools_texture.go (:800, :947,
:1174, :1187), the publish path (platform/service.go:152, :199-210,
:245-254), the schedule route (apihandler/routes.go:30;
source_schedule.go:266-297) and the renderer (texture-source-renderer.ts:362-484).

## Changed or gone, and what it means for Cut 2

1. **The research egress budget is gone** (Cut 1 L2): egress_budget.go and
   the per-call charge are deleted. Comments in yaegikernel/choir.go
   (:511, :544, :597) still mention a per-activation egress budget. 2a's
   "the egress budget went in Cut 1" holds; nothing to fold streams into.
2. **Module sets**: Message and Emit are gone from Texture, research and
   management (0255e81c); research and Texture lost Inbox and Emits,
   Texture and management keep Updates for leftover packets. The
   observed fact "Texture: module set ReadDoc and ApplyTexture plus
   retired Message and Emit" is stale.
3. **Texture learns through records**, not the update packet
   (inbox_records.go, texture_dispatch_fold.go): inboxes texture:<doc>,
   research:<doc>, management:<owner>; a cell reads (cursor, head] and
   advances the cursor in its commit. coagent_update_packet.go:161 is no
   longer on Texture's wake path. 2a's frame "rebuilt from C1's inbox past
   the cursor" builds on choir.Records().
4. **Management's prompt** already has a typed reply (ReportPacket to
   r.From with a coagent_source_packet; work_disposition completed closes
   the round); the "choir.Report with evidence refs" fact is gone. The
   translation contract (plain summary plus artifact handles) is still
   missing.
5. **Management can address any Texture of its owner** (records routing,
   inbox_records.go:72-77), so the W2 finding "management cannot feed a
   Texture other than its own trajectory's" no longer holds for records.
   There is still no capture-arrival notice, and no way for management to
   open a new Texture by Cast (2b's living stories need one).
6. **A cell's host calls end with the cell** (yaegikernel/worker_mux.go,
   errCallAfterCell). 2a's async search must keep arrivals in a
   worker-side store written by the reader loop, which later cells read;
   a goroutine calling back after its cell fails.
7. **Model policy is a two-model pool** (modelpolicy/model_policy.go:467-484,
   modelcatalog.Pools()): deepseek-v4.1-flash and gpt-6-luna per thread,
   each the other's fallback; Texture at high effort. The model comparison
   starts from the pool.
8. **The autonomous Universal Wire publish path still exists**
   (proxy/handlers.go:1803, agentcore/wire_platform_publish.go:213), though
   the doc assumed Cut 1 deleted it. 2b deletes it with the reader change.
9. Smaller: the Texture overlays total 35,230 bytes (revision_policy
   16,898, run_system 15,129; revision_media_source_research_required.yaml
   deleted); the ProductAPI allow-list lost universal-wire, trace and
   continuations; PollSourceOnce lost its admit parameter; the renderer
   lost sourceEntityDisplayPolicy.
