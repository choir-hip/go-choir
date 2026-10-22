# The prompt bar answers HTTP 500 to an owner prompt that contains `[1]`, 2026-10-22

Mutation class of this record: green. Found on staging 2026-10-22 16:28 UTC
during the SEC-06 probe (ledger entry 16:28 of the launch first-pass
metamission).

## Evidence

- `POST /api/prompt-bar` with prompt text that embeds a shell script
  (bracketed indices such as `[1]`) answered HTTP 500
  `{"error":"failed to submit prompt"}`. The client got no reason.
- The guest log held the real cause: `texture prompt bar: submit: start
  Texture lifecycle: lifecycle start: prepare initial revision: invalid
  texture revision: doc.content.paragraph.content[135] contains legacy
  unresolved numbered citation syntax; use a source_ref node`.
- The owner's workaround was to send the script base64-encoded. The
  prompt text itself is ordinary: a pasted shell script, Python with
  `sys.argv[1]`, a markdown footnote `[1]`, a list index.

## Cause (read from code)

1. `internal/textureowner/prompt_bar.go` `HandlePromptBar` maps three error
   classes to a status (command conflict 409, pre-genesis 503) and sends
   every other error from `submitPromptBar` to
   `500 "failed to submit prompt"` (lines 99-101).
2. `submitPromptBar` -> `EnsureTextureHandoff` -> `ensureConductorTextureRoute`
   (`texture_handoff.go`) starts the Texture lifecycle with the owner's
   prompt as the initial revision: `AuthorUser`, `Content` = the prompt,
   no `body_doc`.
3. `store.StartLifecycle` (`internal/store/lifecycle.go`, "prepare initial
   revision") calls `prepareTextureRevisionV2`. With no `body_doc` it
   synthesizes a one-paragraph doc from the owner's text
   (`userAuthoredTextStructuredTextureDoc`, text and hard breaks) and then
   runs `texturedoc.Project`, which runs `texturedoc.Validate`.
4. `Validate` applies the Texture authoring rules on text
   (`legacySourceSyntaxes` in `internal/texturedoc/schema.go`):
   `\[\d+\]` is "unresolved numbered citation", and `{{source:`,
   `[source:...]`, `Source:` lines are refused too. Those rules exist to
   make a Texture agent cite through `source_ref` nodes. They are also
   applied to the owner's literal input, where there is no agent to
   correct and no citation intended.
5. The refusal wraps `store.ErrInvalidTextureRevision` (`%w` all the way
   up), but the prompt bar handler does not test for it. The Texture
   revision API does (`texture.go` returns 400 with the reason for the
   same error); the prompt bar does not.

## Impact

- An owner's ordinary prompt is refused as a server error with no
  reason. The refusal is silent from the owner's side and looks like an
  outage. The same text submitted as a revision through the editor API
  gets a 400 with the reason.
- The failure is deterministic for the text, so a client retry with the
  same text fails again.
- The conductor run record is created before the handoff fails
  (`CompletePromptBarDecision`), so a refused prompt also leaves a
  conductor run behind. This is the existing behavior of every handoff
  failure and is not changed here (see Residual).

## Decision

- (a) The owner's prompt, when it becomes the initial revision of a new
  Texture lifecycle, is the owner's literal text. It is not validated
  against the Texture agent authoring rules on text. It is stored
  exactly as typed. Scope: the initial revision of `StartLifecycle` when
  it is user-authored with no `body_doc`; the request shape (and so the
  start-request digest and command replay) is unchanged. Not changed:
  the editor save path (content-only user revisions into an existing
  document keep the existing refusal, pinned by
  `TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`),
  and every agent-authored revision.
- (b) Any other `ErrInvalidTextureRevision` from the prompt bar submit is
  a 400 with the validator's reason, not a 500.

## Residual (named, not fixed here)

- R1 (hypothesis, read from code, not exercised). The owner's literal
  text now exists in a revision whose body would not pass the agent
  rules. A Texture `Write` that keeps such a block unchanged is
  re-validated as a whole document (`texture_cell_author.go`
  `commitTextureWrite` runs `texturedoc.Project` on the result, and the
  store's `prepareTextureRevisionV2` validates it again at commit), so it
  would be refused for the carried `[N]` text, and the agent can only
  rewrite the block. A Write that replaces the owner's text is
  unaffected. A durable fix is a marker that travels with owner-literal
  nodes, or validating only the text a Write changed
  (`texturewrite.ChangedBlocks`). To be confirmed from a run's ledger
  (turn-commit reasons and `choir.Write` refusals) on a prompt that
  contains `[N]` and asks the desk to extend it.
- R2. `code_block` text and `code`-marked text are checked against the
  numbered-citation rule too. `argv[1]` in a code block is not a
  citation, for agents either. Out of scope for this lane.
- R3. A refused prompt leaves its conductor run record behind in
  whatever state `CompletePromptBarDecision` left it (existing behavior
  for any handoff failure).
- R4. The editor save path refuses the owner's `[1]` text in an existing
  document with a 400 and reason. That is a deliberate contract today
  (it stops flattening a citation projection into text) and is not
  changed.

## Fix (as landed on lane/fp0-prompt-bar-refusal)

Mutation class: orange (the owner's input into a Texture lifecycle; sits
next to the red surface "Texture canonical writes", so FP0 may land it as
red). Protected surface adjacent: the validation that stands in front of a
canonical Texture revision.

- Option (a), taken. `texturedoc.ValidateOwnerText` / `ProjectOwnerText`
  are `Validate` / `Project` without the authoring rules on text; every
  other rule (schema, node ids, node shapes, marks, no `source_ref`
  because the doc has no source entities) still applies. The store's
  `StartLifecycle` prepares the initial revision with
  `prepareOwnerInputTextureRevisionV2`: when the revision is user-authored
  with no `body_doc`, the store builds the one-paragraph doc from the
  owner's text as before and validates it with the owner-text rule. A
  revision with a `body_doc`, any agent-authored revision, and
  `CreateRevision` / the editor save path are prepared as before. Why (a):
  the owner's words are stored exactly (the same content as before for
  text that has no bracket), the start request and its digest are
  unchanged, so command replay is unaffected, and nothing about an agent's
  output is relaxed. The alternatives were rejected: escaping or splitting
  the text into several text nodes would change the owner's words or only
  hide the text from the validator and fail again when `texturewrite`
  re-derives the block; a flag in `types.StartLifecycleRequest` or a
  marker attribute in the doc would change the digest or have to survive
  the editor's rebuild of the doc (`serializeEditorStructuredDoc`
  rewrites node attrs).
- Option (b), also done. `promptBarSubmitFailure` is the one mapping from
  a failed submit to a status: command conflicts 409, pre-genesis 503
  (both unchanged), `store.ErrInvalidTextureRevision` 400 with
  `prompt refused: <the validator's reason>` (the internal wrapping is
  cut), everything else 500 `failed to submit prompt`. After (a) no
  prompt text reaches (b) today; it is the floor for the next validation
  rule.
- Owner-start surfaces now equal: `POST /api/texture/lifecycle-documents`
  (owner create) reaches the same `StartLifecycle` branch with
  user-authored plain text, so its initial text is literal too (it
  answered 400 "lifecycle Texture create rejected" before).

Tests (behaviour, against a real store through the HTTP handler):
`TestHandlePromptBarKeepsOwnersBracketedTextVerbatim` (six prompts, each
202 and the stored initial revision equal to the prompt),
`TestHandlePromptBarReplaysBracketedPrompt`,
`TestStartLifecycleKeepsOwnerLiteralInitialText` (owner text kept, replay
stable, agent-authored initial `[1]` still refused with no trajectory
persisted), `TestPromptBarSubmitFailureStatusAndReason` (the mapping). The
first three fail on 273d139c9 with HTTP 500 and "contains legacy ...
syntax". The pre-existing `TestTextureCreateRevisionRejectsLegacySource...`
cases and `TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`
still pass: the editor path is unchanged.

Heresy delta. Discovered: R1 to R4 above. Introduced: owner-literal text
now sits in a stored revision that the agent text rules would refuse (R1).
Repaired: an owner's ordinary prompt answered as a server error with no
reason.

## Proof for FP0 on a test computer (not run by the lane)

1. After deploy, as the owner of a test computer:
   `POST /api/prompt-bar` with
   `{"text":"import sys\nprint(sys.argv[1])\nSummarize values[1] and [2]."}`
   expects 202 with `doc_id` and `revision_id`.
2. `GET /api/texture/documents/<doc_id>` (or the revision) shows the
   initial revision content equal to the text sent, with `[1]` and `[2]`
   intact; the guest log has no "prepare initial revision" line.
3. A Texture desk run starts for the document (`initial_loop_id` in
   `GET /api/prompt-bar/submissions/<id>` decision). Whether the desk's
   first `choir.Write` succeeds on this text is R1's check.
4. Regression: the same text through the editor's content-only save of an
   existing document still answers 400 with the reason.

Rollback: `git revert` of the fix commit. No schema, no stored-data
migration. A revision stored under the exemption keeps its text; after a
revert only a later whole-document re-validation of that revision's
blocks could refuse it (R1).
