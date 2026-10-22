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
  against the Texture agent authoring rules on text. It is stored as
  typed (line endings as LF, ends trimmed; round 2 F3). Scope: the
  initial revision of `StartLifecycle` when it is user-authored with no
  `body_doc` and the caller is not in-process (round 2 F1); the request
  shape (and so the start-request digest and command replay) is
  unchanged. Not changed: the editor save path (content-only user
  revisions into an existing document keep the existing refusal, pinned by
  `TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`),
  and every agent-authored revision.
- (b) Any other `ErrInvalidTextureRevision` from the prompt bar submit is
  a 400 with the validator's reason, not a 500.

## Residual (named, not fixed here)

- R1 (confirmed at the validator, run effect unmeasured; see Round 2 F2).
  The owner's literal text now exists in a revision whose body would not
  pass the agent rules. A Texture `Write` that keeps such a block
  unchanged is re-validated as a whole document (`texture_cell_author.go`
  `commitTextureWrite` runs `texturedoc.Project` on the result, and the
  store's `prepareTextureRevisionV2` validates it again at commit), so it
  is refused for the carried `[N]` text, and the desk can only rewrite the
  block (`@replace` or `@all`). A Write that replaces the owner's text is
  unaffected. A durable fix is a marker that travels with owner-literal
  nodes, or validating only the text a Write changed
  (`texturewrite.ChangedBlocks`). No desk has been run end to end on such
  a prompt; the run ledger (turn-commit reasons and `choir.Write`
  refusals) is the missing evidence, and FP0's proof step 3 collects it.
- R2 (worse than first written; see Round 2 F2). `code_block` text and
  `code`-marked text are checked against the numbered-citation rule too:
  a desk can never write `sys.argv[1]`, even in a code span or a code
  block, and the refusal does not name the block and tells the desk to
  use a `source_ref` for code. Out of scope for this lane.
- R3. A refused prompt leaves its conductor run record behind in
  whatever state `CompletePromptBarDecision` left it (existing behavior
  for any handoff failure).
- R4. The editor save path refuses the owner's `[1]` text in an existing
  document with a 400 and reason. That is a deliberate contract today
  (it stops flattening a citation projection into text) and is not
  changed.

## Round 2 review findings (bunch 81, head 951e4e145), recorded before any fix

Reviews: Sonnet 5.5 (approve with conditions,
`_runs/bunch81/sonnet.out`) and Gemini flash (accept,
`_runs/bunch81/out/omp-gemini38.out`). The change is validation in front
of a canonical Texture revision, which CLAUDE.md lists as red ("Texture
canonical writes"): this lane is red for landing. The Fix section below
called it orange; that classification is withdrawn.

- F1, medium, authority. The management desk reaches the owner-text path,
  so the claim in the Fix section that "nothing about an agent's output is
  relaxed" is false for it. Chain: management has the `ProductAPI` verb
  (`internal/deskmanifest/manifest.go:210`); `choir.ProductAPI` allows
  `POST /api/prompt-bar` (`internal/agentcore/tools_product_api.go:216`)
  and serves it in-process as the run's owner, with an empty `RemoteAddr`
  and `X-Authenticated-User` set (`:76`); `HandlePromptBar` ->
  `ensureConductorTextureRoute` hard-codes `AuthorUser`
  (`texture_handoff.go:228`) -> `StartLifecycle` takes the owner-text
  rule. Sonnet's probe (an in-process request shaped like
  `productAPIRequest`, text `Findings so far.[1] See {{source:
  capture-7}}.\nSource: https://example.com/fake\n[source: wiki]`) got
  202 and a stored revision with `author_kind=user` and the text intact.
  On main that text was refused. The desk reads captures, Stories and
  Feeds, so the text it composes can carry untrusted input; the owner-text
  rule would store citation-lookalike strings in a revision labelled as the
  owner's words. Impact is bounded: no `source_ref` can be forged (the
  synthesized doc has no source entities), and no consumer parses these
  tokens. It is a provenance regression, not a privilege escalation, but it
  is the only content guard that stood in front of a desk-reachable
  canonical write. The signal that tells an owner's request from a desk's
  is unforgeable at the HTTP boundary: `server.GuestHostCaller` documents
  an empty `RemoteAddr` as the in-process marker, `choir.ProductAPI` is the
  only non-test producer of it, and a proxied owner request carries the
  host gateway address. Not reachable for the other desks (Texture,
  Research, Engineering have no `ProductAPI`), and not through
  `/api/texture/lifecycle-documents` (`validateProductAPIRoute` refuses a
  non-GET under `/api/texture/`; Sonnet probed path escapes and the mux
  answers 307 or 404).
- F2, medium, product outcome. R1 is confirmed at the validator, not just
  read (Sonnet ran `texturewrite.Apply` and `texturedoc.Project` on a doc
  shaped like the store's owner paragraph `print(sys.argv[1])<br>Explain
  it.`). `@insert after b1` (keeps the owner block) is refused with
  "contains legacy unresolved numbered citation syntax; use a source_ref
  node". `@replace b1` and `@all` without brackets are accepted. An `@all`
  that restates `sys.argv[1]` in plain text is refused at Apply
  (`apply.go:543`: "numeric markers such as [12] ... cite with {{cite
  ...}}"). The same text in a code span or a `code_block` passes Apply
  (code spans are exempt there) and is refused at `Project`
  (`schema.go` checks `code_block` text and code-marked text with the same
  rule): R2 is worse than first written, a desk can never write
  `sys.argv[1]`, even as code. The refusal does not name the block:
  `validateBlock` is called with the constant path `doc.content`
  (`schema.go`), so only the inline index inside the block shows, and
  "use a source_ref node" misleads for code. The desk can recover with
  `@replace` or `@all`, so it is not a deadlock, but it spends turns. Run
  effect is unmeasured: no desk was run end to end.
- F3, low. "Stored exactly as typed" is overstated. The store's plain-text
  synthesis (`userAuthoredTextStructuredTextureDoc`, unchanged by this
  lane) turns CRLF and a lone CR into LF; the handler trims leading and
  trailing whitespace. Tabs, leading spaces, blank runs, emoji, markdown
  and NUL are byte-equal (Sonnet's table). Keeping CR would need a
  different storage shape and is not done.
- F4, low. `texturedoc.ValidateOwnerText` and `ProjectOwnerText` are
  exported and accept any document. The store call site is safe (a
  `body_doc` is validated strictly first, and the owner rule is chosen only
  in the no-`body_doc`, `AuthorUser` branch), but a later caller could pass
  a model-authored doc. The owner path also ignores
  `unused_source_entity_ids` (not owner-controllable today).
- F5, low, process. The change is red for landing (above). The commit
  messages carried rollback, heresy delta, conjecture delta and
  evidence class, but named the class orange.
- F6, info. A refused submit has already persisted its conductor run keyed
  by `command_id` (R3). A client that follows the 400 and resubmits
  corrected text under the same `command_id` gets 409. Retry with a new
  `command_id`.
- F7, info. A stored owner revision with `[N]` is not publishable through
  the structured path (`platform/publication_document.go:99` fails
  `Validate` and falls back to plain content). Fail-soft.
- Also named: `SubmitOwnerPrompt` (the computer's own release-adoption
  request, `agentcore/release_adoption.go`) reaches the same rule without
  HTTP. Its text includes publisher-supplied changed paths and a test
  command. It is the owner's act and on main a `[N]` in a changed path
  would have refused the adoption with a 500; it stays literal.

Plan: (F1) the in-process marker on the request context, set by the two
owner-start handlers, read by `StartLifecycle` to choose the strict rule;
a failing test first. (F4) narrow `ValidateOwnerText` to the shape the store
synthesizes. (F2, F3, F5, F6, F7) record updates and a stricter proof step,
no new behaviour.

## Fix (as landed on lane/fp0-prompt-bar-refusal)

Mutation class: red (round 2 F5; this section first said orange). The
change is validation in front of a canonical Texture revision, a
"Texture canonical writes" surface. Protected surface touched: the
initial-revision validation of `StartLifecycle`. Conjecture delta: the
owner's input is not Texture authoring, so refusing it as a citation error
was a category error; and a caller's identity, not the text, decides which
rule applies. Admissible evidence class: HTTP handlers and `StartLifecycle`
against a real store (the tests below); the deployed proof is FP0's.

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
  the owner's words are stored as before (the same content as for text
  that has no bracket; see F3 below for the line-ending caveat), the start
  request and its digest are unchanged, so command replay is unaffected,
  and the text of an agent or a desk is not relaxed (round 1 wrongly
  claimed this without the in-process marker below). The alternatives were rejected: escaping or splitting
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
  cut), everything else 500 `failed to submit prompt`. After (a), (b) is
  live for exactly one caller: an in-process one (next bullet).
- In-process marker (round 2 F1, the management desk). `HandlePromptBar`
  and `HandleTextureLifecycleCreate` call `ownerTextContext`: when
  `strings.TrimSpace(r.RemoteAddr) == ""` (`server.GuestHostCaller`'s
  documented in-process marker; `choir.ProductAPI` is its only non-test
  producer, `tools_product_api.go:76`) the context carries
  `store.WithStrictInitialText`, and `StartLifecycle` then prepares the
  initial revision with the strict rule (`prepareTextureRevisionV2`)
  instead of the owner-text rule. A desk-shaped call with `[1]`,
  `Source:`, `{{source:` or `[source: ...]` gets a 400 `prompt refused:
  <reason>` it can correct; ordinary desk text still starts a Texture
  (202). The marker is on the context, not in the request, so the
  start-request digest and command replay are unchanged. A proxied owner
  request carries the host gateway address, never an empty one.
  `SubmitOwnerPrompt` (the computer's own release-adoption request, no
  HTTP) carries no marker and stays literal: it is the owner's act.
- Narrowed (round 2 F4). `texturedoc.ValidateOwnerText` accepts only the
  shape the store synthesizes from plain text, one paragraph of unmarked
  text and `hard_break` nodes, so a later caller that passes a model-authored
  document (a `code_block`, a mark, a heading) is refused there and has to
  pass `Validate`. That guard is unreachable through any handler (the store
  always builds the shape), and CLAUDE.md says no unit test after code, so
  it has no test of its own. The owner path still ignores
  `unused_source_entity_ids` (the synthesized document has no source
  entities; not owner-controllable).
- F3 (line endings). The store's plain-text synthesis
  (`plainTextInlineNodes`, unchanged) turns CRLF and a lone CR into LF, and
  the handler trims the text's ends. A literal-bytes guarantee for CR needs
  a different storage shape and is not done. "Stored as typed" in this
  record means: the text, with line endings as LF and the ends trimmed,
  exactly as on main for a prompt that has no bracket.
- F6. A 400 from the prompt bar leaves its conductor run keyed by
  `command_id` (R3). Resubmit corrected text under a new `command_id`; the
  same one answers 409.
- F7. A stored owner revision with `[N]` is not publishable through the
  structured path (`platform/publication_document.go:99`); it falls back to
  plain content. Fail-soft.
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
syntax". Round 2 adds
`TestHandlePromptBarInProcessCallerStaysUnderCitationRules` (five
desk-shaped texts, each 400 with `prompt refused: ... legacy ...`),
`TestHandlePromptBarInProcessCallerStillStartsATexture` (ordinary desk
text 202) and `TestTextureLifecycleCreateOwnerTextLiteralInProcessStrict`
(owner 201, in-process 400); the first and the last fail at 8c35cefdc
with 202 and 201 (the desk's text stored as an owner-labelled revision). The pre-existing `TestTextureCreateRevisionRejectsLegacySource...`
cases and `TestTextureRevisionAPIAcceptsStructuredBodyAndRejectsLegacySourceSyntax`
still pass: the editor path is unchanged.

Heresy delta. Discovered: R1 to R4 above; round 2 F1 (the management desk
reached the owner-text rule through `choir.ProductAPI`, a hole this lane
had introduced in its first head and closed in round 2), F2 (R1 confirmed,
R2 worse, refusal path names no block), F3 to F7. Introduced: owner-literal
text now sits in a stored revision that the agent text rules would refuse
(R1). Repaired: an owner's ordinary prompt answered as a server error with
no reason; the desk-reachable relaxation of round 1.

Follow-ups named, not done here (red-adjacent, widen the authoring rule or
the validator): (1) exempt `code_block` text and code-marked text from the
numbered-citation rule for agents (R2); (2) carry the block id in the
validator's path, and say something other than "use a source_ref node"
when the text is code (F2); (3) the R1 durable fix, a marker that travels
with owner-literal nodes or validating only the text a Write changed.

## Proof for FP0 on a test computer (not run by the lane)

1. After deploy, as the owner of a test computer:
   `POST /api/prompt-bar` with
   `{"text":"import sys\nprint(sys.argv[1])\nSummarize values[1] and [2]."}`
   expects 202 with `doc_id` and `revision_id`.
2. `GET /api/texture/documents/<doc_id>` (or the revision) shows the
   initial revision content equal to the text sent, with `[1]` and `[2]`
   intact; the guest log has no "prepare initial revision" line.
3. The Texture desk lands a revision, not just a run (round 2 F2). Wait
   for the document's head to become a revision authored by the Texture
   desk (an `app_agent` revision after the owner's v0) on this prompt, and
   read the run's ledger: turn-commit reasons and any `choir.Write refused`
   lines. Pass only if a desk revision lands. A run that starts and ends
   in Write refusals ("contains legacy unresolved numbered citation
   syntax") is R1 or R2 happening live: record it with the ledger as the
   evidence for the follow-ups, and treat the proof as failed for the
   launch claim "an owner can paste a script".
4. Regression: the same text through the editor's content-only save of an
   existing document still answers 400 with the reason.
5. Desk caller (round 2 F1), if the test computer's management desk can be
   cast to do it: a management cast whose objective is to
   `choir.ProductAPI` `POST /api/prompt-bar` with text containing `[1]`
   expects the desk to receive 400 `prompt refused: ...`; no document is
   created for it. The lane's tests prove the same at the handler with an
   empty `RemoteAddr`.
6. Retry note (F6): resubmitting corrected text after a 400 needs a new
   `command_id`.

Rollback: `git revert` of the fix commits (the round 2 marker commit
alone reverts to the round 1 behaviour for the desk, which is the F1
hole; revert both for main's behaviour). No schema, no stored-data
migration. A revision stored under the exemption keeps its text; after a
revert only a later whole-document re-validation of that revision's
blocks could refuse it (R1).
