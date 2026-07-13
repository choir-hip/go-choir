# S3 Runtime Dissolution Step-2 Phase Gate

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Canonical review head: `3177a8c5`
- Mutation class: yellow
- Authority: `docs/definitions/choir-autoputer-completion-suite-2026-07-11.md:506-523`

## Question

After S3-I9/I10 extracted the tool-loop state machine, batch executor, and typed execution context into `internal/toolregistry`, and S3-I11 removed anonymous `*runtime.Runtime` embedding from `internal/actorruntime.Adapter`, do the remaining explicit named `Adapter.Runtime` and `actorHandler.rt` edges keep step 2 open, or are they lifecycle/storage integration residue assigned to later ordered steps?

## Independent Gate Result

Four independent reviewers returned `STEP2_COMPLETE` with no blocking findings and confidence `0.88-1.0`.

The canonical criterion at suite lines 512-514 requires extraction of the live execution/tool-loop core and removal of `*runtime.Runtime` embedding. It does not require deletion of every named runtime dependency. Step 3 separately owns API/config/bootstrap movement, `apihandler` removal, and the direct `cmd/sandbox` runtime import; steps 4 and 6 own app/domain and final core residue.

The extracted authority is concrete:

- `internal/toolregistry/toolloop.go` owns `RunToolLoop` and the storage-independent loop state machine;
- `internal/toolregistry/batch_executor.go` owns complete batch execution policy;
- `internal/toolregistry/execution_context.go` owns the sole typed execution context;
- runtime supplies run/storage/provider-derived integration inputs but no duplicate loop or executor path;
- `Adapter.Runtime` and `actorHandler.rt` are named fields, not anonymous embedding, and promote no runtime method set.

## Adjudication

`STEP2_COMPLETE`. The S3-I11 statement that the named adapter and handler edges were necessarily further step-2 debt was over-conservative and is superseded by this phase-gate interpretation. It was orchestrator-authored evidence, not a settled owner authority change. No doctrine or suite criterion is changed.

Authorize S3 step 3 only: move real API/config/bootstrap ownership, remove the `apihandler` wrapper, and remove direct `cmd/sandbox` runtime imports. Do not infer S3 completion or authorize app/domain step 4 before step 3 lands and passes its own gate.

## Residuals

- `Adapter.Runtime`, `actorHandler.rt`, and other lifecycle/storage integration remain explicit whole-S3 extinction debt.
- The executable inventory continues to classify those edges as `delete`; the phase gate changes ordering, not final disposition.
- The next slice must derive the smallest atomic API/config/bootstrap cutover from current production callers; it must not combine unrelated app/domain extraction.
