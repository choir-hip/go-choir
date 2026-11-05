# Texture Articles Do Not Say Which Models Wrote Them (2026-11-05)

Owner, 2026-11-05: "we should be noting in the texture articles for
autopaper (and otherwise) which models were used. this was setup months
ago but its out of date/broken/unwired currently -- eg it doesnt even list
engineering (note we should include when we use claude delegate, and when
we switch models in a trajectory). this is a key bit of provenance."

Mutation class of the fix: **orange** (a new durable event kind, a new
owner route, a manifest field on new published versions, the source panel
and the published reader). Rollback: revert; versions published in between
keep their `model_provenance` manifest field, which older readers ignore.

## What was built, and why it does not answer the question

1. **The source panel's "Models / Effective roles" box**
   (`b26efec3e`, 2026-06-19, `TextureSourcePanel.svelte`). It calls
   `GET /api/model-policy/resolve?role=` for four hard-coded roles
   (conductor, texture, research, management) and shows what the policy
   *would* pick today. That is a prediction about a future run, not a
   record of past ones. Since then the platform moved under it:
   - the default pool spreads threads over six models by a thread hash
     (`modelcatalog` pools, `modelpolicy.Assign`), so the role's resolved
     spec (`pool:default`) says nothing about which member served a run;
   - quota switches (`toolloop.go` `switchOnQuota`, 2026-11-04) and pool
     failover (`model_failover`) move a run to another model mid-run;
   - engineering became a desk with its own model, and the Claude delegate
     (headless Claude Code in the capsule) does most source edits;
   - management tasks, the System 1 / Jev decision model and research
     children all call models the box never listed.
   It shows nothing on the published reader at all.
2. **`model_id` on desk records** (`8f8cc6848`, 2026-10-14,
   `internal/textureowner/record_model.go`, problem
   `desk-records-lack-model-id-2026-10-14.md`). Texture's dispatched
   records carry the answering model of the turn that wrote them. It is
   wired (`texture_cell_author.go`) and correct for what it is, but it is
   one model per record, only for records, and it is not a list of the
   models that served a document.
3. **The per-call cost line** (`loop.progress` phase `tool_loop`, written
   by `toolloop.go` after every answered call, with `llm_provider`,
   `llm_model` after any switch, and tokens). This *is* the record of
   which model answered each call, and `/api/costs` reads it. But it is
   telemetry: the 2026-10-30 retirement (`telemetry_retire.go`, phased
   `loop.progress` after 14 days) deletes it from the working store, and
   each line carries up to 2 KB of response text and prompt shape. A
   published article lives forever; a provenance read off this line would
   empty after two weeks.

So there was no durable, small, per-run record of the models that served
a run, and nothing joined a document to its runs' models.

## Record of truth

The model that answered a call is known in exactly one place: the tool
loop, at the moment the provider returns (`activeLLMConfig` after any quota
switch or failover; the provider's reported model on `resp.Model`). The
decision model's calls are known where `recordJevCostFor` records their
cost. The Claude delegate's use is already durable on the engineering
report (`EngineeringAssignmentReport.DelegateUse`: calls, successes,
model), derived from the run's cells.

## Design

- **`model.served`, one durable line per tool-loop activation**
  (`types.EventModelServed`, acknowledged, object graph, never retired),
  written by a `defer` in `RunToolLoop` whichever way the loop returns:
  the selection the activation started on (with its policy source), each
  model that answered with its call count and tokens, and every switch
  (from, to, reason: `model_failover`, `provider_quota`,
  `provider_quota_retry`). One line per activation, not per call: a few
  hundred bytes per run instead of a second per-call line beside the
  2 KB cost line. A decision-model call writes one line with role
  `system1`.
- **Lineage.** A document's runs are the runs on its channel or one of its
  trajectories (Texture, the research desk, engineering casts, which carry
  the document's trajectory), and the management turn that cast an
  engineering run of the lineage (a point lookup, so its other children do
  not leak in). Every read is a generated, indexed column: runs by
  `og_meta_channel_id` and `og_meta_trajectory_id`, served lines by
  `og_meta_run_id`, engineering reports by `og_meta_trajectory_id`
  (`TestModelProvenanceReadsUseTheMetadataIndexes` pins the plans). The
  cost is bounded by the lineage, never by the store. A lineage with more
  than 400 runs keeps its newest 400 by creation time and says so through
  `runs`; the casters come on top, one per kept engineering run.
- **Research children** are not runs (Cast/Report records). Each child's
  loop writes its `model.served` line on its parent run with role
  `research_child`, so the parent's document reads it with no new edge.
- **Provenance** (`GET /api/texture/documents/{id}/models`,
  `Runtime.DocumentModelProvenance`): models by desk role with calls,
  runs, tokens and how each was reached (`selected`, the switch reason, or
  `claude-delegate`), plus the switch list. A run with no `model.served`
  line (it ended before this deploy) shows its selection marked
  `selected_unrecorded`, with no call count.
- **Published.** The proxy reads the route from the publishing computer at
  publish time; corpusd pins only the public projection
  (`ModelProvenance.Public`, schema `choir.model_provenance.public.v1`)
  into the version manifest (`model_provenance`), so the published reader
  and story page show it from corpusd alone. The projection is built field
  by field: role, provider, model, reported model, calls, tokens, how it
  was reached, delegate use, and switches as role/from/to/reason. Run ids,
  the document id, and selection sources (policy file paths, task overlay
  ids) stay on the owner route only.

## Residuals

- Runs that ended before the deploy have no `model.served` line; they show
  their selection only, marked as not recorded. Versions published before
  the deploy carry no provenance.
- A long activation that has not returned yet shows nothing until it
  passivates or ends; a crash loses the open activation's line (a crash
  closes the run).
- The management turn that cast an engineering run is shown whole: its
  calls served every record of that wake, not only this document's.
  Management's own turns on a document's Cast with no engineering run are
  not attributed (no run-level link exists).
- The lineage costs one indexed read for the channel, one per trajectory
  (runs and reports), and one per kept run for its `model.served` lines.
  Publish calls it once per story version.
- Runs reached only through `requested_by_run_id` on another channel and
  trajectory (that field has no generated column) are not walked; review r1
  (B3) dropped that unindexed walk. Verifier and engineering runs carry
  the document's trajectory and are reached that way.
- Management tasks are not reached: they hang off management's turn, which
  enters the lineage only as an engineering run's caster, and their parent
  link is not an indexed field.
- The revisions read for their trajectories is capped at 500 per document.
- The System 1 judge's provider is the one the Decisions response names,
  else the gateway transport's (OpenRouter).

## Review r1 (Devin, 2026-11-05): adjudication

- B1 public manifest leaked internal detail (policy paths, overlay ids, run
  and doc ids): fixed with the explicit public projection, and
  `TestPublishedModelProvenanceCarriesNoInternalDetail`.
- B2 research children were not recorded: fixed, children write on the
  parent run as `research_child`;
  `TestDocumentModelProvenanceReachesResearchChildren` runs the real child
  loop with a quota switch.
- B3 unindexed `requested_by_run_id` and report `loop_id` scans: fixed by
  restructuring (no schema change): reports are read by trajectory and the
  requested-by walk is gone.
- N5 judge provider from the call, N6 reported model kept, N7 newest-400
  cap, N8 report read errors surface: fixed.
- The non-tool-loop `executeWithProvider` path (stub or bridge provider,
  no tools) writes no line.
- The downloaded exports (`export_html.go` and the other formats) do not
  render the provenance yet; the version's manifest in the resolve bundle
  carries it.
