# Desk Records Carry No `model_id` (2026-10-14)

Found by: E6a, the desks' first box score
([claims-e0-e6a-results](../archive/reports/claims-e0-e6a-results-2026-10-14.md)
§4 and finding **F2**). Computer: the live paper
(computer-1355b83b…), read-only through the QA owner credential.
Mutation class of the fix: **orange** (runtime behaviour: what the
reducer writes on every desk record). Status: problem recorded first;
fixed on branch `icrl-calibration-2026-10-14` (commit "fix(records):
every desk record names the model that produced it"), not pushed, not
deployed; acceptance is `model_id` on 100% of new desk records after
deploy (the experiment's phase 0 check).

## Evidence

- `docs/archive/evidence/claims-e6a/box-score.json`, key `model_ids_on_records`:
  `{"empty": 2931}`. Every Cast, Report, Ask and Note the pull read
  (2026-10-13 15:24Z to 2026-10-14 18:15Z, 373 stories) has
  `provenance.model_id == ""`.
- Code reading at `10e9f18e1` (origin/main): the field exists
  (`types.CommitmentProvenance.ModelID`, `json:"model_id"`) and five store
  paths copy it into the object-graph metadata
  (`store/commitment.go:33`, `lifecycle_commit_act.go:310,563`,
  `lifecycle_control_issue.go:387`, `inbox.go:454`), but **no code path
  ever assigns it**. `grep ModelID` outside `ScorerModelID` and LLM
  config finds only the declaration and those copies.
- The one choke point every desk act passes through,
  `agentcore.commitmentRecordForIntent` (`rlm_reduce.go`), builds the
  provenance from `ReductionScope`, which has no model field. Records
  minted by Texture's dispatch fold, management tasks, engineering casts
  and the records route all start there.
- What the runtime does know: the run's selected model is in run
  metadata (`llm_provider`, `llm_model`, mirrored to `provider`/`model`
  for Texture's revision provenance), and the tool loop knows the model
  that answered the current call (`activeLLMConfig`, which changes on
  failover, `toolloop.go`). Neither reaches the reducer.

- Re-checked read-only at ~20:30Z the same day: the eight newest front
  stories' `texture:` and `research:` inbox records (non-system) are 33 of
  33 empty, newest committed 20:21:12Z.

## Why it matters

- No per-model box score: the paper runs a pool (deepseek-v4.1-flash,
  gpt-6-luna, others) and E6a cannot say which model is under-confident.
- The in-context learning loop the owner asked for (owner notes §1,
  E6a §5 step 4 and 6) needs `model_id` on every record that carries a
  claim or prediction, so a context pack can be per (model, task type).
- Multi-scoring needs the scorer's model on score records; a self-score
  is a score by the speaking model.

## Belief state

- Substrate or symptom: **symptom of a missing field fill** at one choke
  point, not a substrate defect. No replacement implementation exists
  unwired.
- The value must be the model that produced the act. With failover, the
  run's selected model and the answering model differ; the answering
  model is the right one, with the run's selection as the fallback.

## Failure modes, written before the fix

- **M1. Replay identity breaks.** Record bodies are digested for command
  replay (`ComputeCommitInboxRecordsDigest`,
  `ComputeCommitLifecycleActDigest`, the Texture turn digest). A cell
  replayed after a failover would carry a different `model_id` and its
  replay would be refused as a conflicting command. Guard: `model_id` is
  observation metadata like `committed_at`; each digest clears it, and
  Texture's dispatch fold leaves it empty (it is stamped when the turn is
  applied, after the fold's identity is fixed). Tests replay a command
  with a different `model_id`.
- **M2. Texture revision identity changes.** The fold's records ride the
  turn body that feeds the revision identity. Guard: same as M1, the fold
  never carries the model; the Texture author stamps it on the request.
- **M3. Wrong model after failover.** Guard: the tool loop puts the
  answering provider/model on the tool-call context; the reducer prefers
  it over run metadata. Test: a context model wins over metadata.
- **M4. Empty on paths the fix misses.** System records (closures,
  capture arrivals, mechanical resolves, Jev's support records) have no
  model and stay empty on purpose: they are not desk claims. Every
  record a desk authors goes through `commitmentRecordForIntent`. Test:
  a staged Cast, Report and Ask each get the scope's model.
- **M5. Format drift.** One string, `<provider>/<model>` (for example
  `opencode-go/deepseek-v4.1-flash`), the form the claims scripts
  already use for omp models. A model name containing `/` (OpenRouter)
  stays readable: the provider is the text before the first `/`.
- **M6. Old records.** Records already written stay empty; readers treat
  empty as "unknown model". No backfill: the run that wrote them cannot
  be joined reliably to the model that answered.

## Rollback

Revert the fix commit. Records written meanwhile keep their `model_id`;
readers already accept both.
