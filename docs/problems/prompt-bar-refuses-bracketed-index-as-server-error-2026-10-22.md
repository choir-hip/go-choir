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

- R1. The owner's literal text now exists in a revision whose body would
  not pass the agent rules. A Texture `Write` that keeps such a block
  unchanged is re-validated as a whole document
  (`texture_cell_author.go` `commitTextureWrite`, and the store's
  `prepareTextureRevisionV2` at commit), and is refused for the carried
  `[N]` text. The agent can rewrite the block. A durable fix is a
  marker that travels with owner-literal nodes, or validating only the
  text a Write changed (`texturewrite.ChangedBlocks`).
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

Rollback: `git revert` of the fix commit. No schema, no stored-data
migration. A revision stored under the exemption keeps its text; after a
revert only a later whole-document re-validation of that revision's
blocks could refuse it (R1).
