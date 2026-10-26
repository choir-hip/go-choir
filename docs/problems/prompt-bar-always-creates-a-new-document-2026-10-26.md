# The prompt bar always creates a new document (FE-02 step 1)

Recorded 2026-10-26 by an Opus worker on lane/route-continue. Status: open,
documented before any fix. Mutation class of the fix: orange (prompt-bar
API, shell state) with a red edge: it reaches the Texture canonical write
path, through the existing owner revise only. No new canonical writer.

## What the owner sees

The owner has a Texture document open and types the next step into the
global prompt bar. A second, new document opens with that ask as its seed.
To build on the open document the owner has to find the document window's
own revise box. The owner's direction (2026-10-26 00:5x): "when it makes
sense, build on top of existing documents". F1 made that possible for a
settled document (docs/problems/settled-document-refuses-owner-revision-2026-10-26.md,
proved on staging 05:49); the prompt bar does not use it.

## Evidence (code, at 35f8fc26c)

- `internal/textureowner/prompt_bar.go` `HandlePromptBar` decodes
  `{text, command_id}` only (unknown fields refused) and calls
  `submitPromptBar`, which records a conductor decision `open_app` and,
  for Texture, `EnsureTextureHandoff` with `HandoffKindUserPrompt`: a new
  document and trajectory every time. The conductor's rules
  (`contentowner.ClassifyPromptBarContentIntent`): a URL opens a display
  app, everything else is a new Texture.
- `frontend/src/lib/Desktop.svelte` `submitPromptText` posts the text and
  opens a new Texture window from the decision's `doc_id`.
- Building on a document happens only from the Texture window's revise
  (`TextureEditor.svelte` `handlePrompt` -> `POST
  /api/texture/documents/{id}/revise`, `router.go:47`,
  `HandleTextureAgentRevision` -> `handleLifecycleOwnerRevision` in
  `texture_agent_revision.go`). That path holds the authority checks
  (owner-scoped document read, lifecycle-bound, live or F1-continuable,
  head check, one open Texture work item) and commits the owner revision.
- Residual FE-02 (docs/evidence/residual-register-2026-10-26.md) and ledger
  2026-10-26 01:0x: the conductor is rules only; an overridable chip is
  recommended. The owner chose direction "A"; this is its step 1, with no
  model call.

## Design

1. **API.** `POST /api/prompt-bar` takes an optional `continue_doc_id`.
   Absent: unchanged (conductor decision, new document). Present: the
   prompt-bar submit runs the existing owner revise for that document
   instead of the conductor. The revise core moves out of the HTTP handler
   into one function (`reviseOwnerDocument`) that both `/revise` and the
   prompt bar call; `/revise` keeps its exact behaviour and response. No
   second revise path.
   - Same authority: the document is read owner-scoped; another owner's or
     a missing document is 404 "document not found" (existence is not
     disclosed); an unbound document is 409; a cancelled trajectory is 409
     (cancel is a stop, F1 C8); a settled one continues on a new trajectory
     (F1).
   - Expected head: the document's current head (the prompt bar asks to
     build on the document as it now is). The revise's own head check still
     fences a head that moves between read and commit.
   - Idempotency: the prompt bar's `command_id` is the revise's
     `client_request_id`, so a retried submit cannot commit a second owner
     revision (see "Found while building" for why it is a 409, not a
     replay).
   - Text: the owner's text, trimmed exactly as the new-document path trims
     it, is the revision's `owner_prompt` unchanged (SD-07: never retyped).
   - A desk's in-process call (empty RemoteAddr, the management desk's
     ProductAPI route) is refused with 403: `product_api` already refuses a
     desk any Texture write ("only the Texture desk writes a Texture"), and
     an owner-authored revision through the prompt bar must not open that
     door.
   - Response (202): `{schema: "choir.prompt_bar_continue.v1", continued:
     true, command_id, doc_id, trajectory_id, revision_id, request_id,
     replay}`. `trajectory_id` is the one the revision committed on, a new
     one when a settled document continued. There is no conductor
     submission, so no `submission_id` or `status_url`.
2. **Frontend.** When the active window is a Texture window that holds an
   owner document (`appContext.docId`, signed in, not minimized), the
   prompt bar shows one chip inside the command field: "Continue <title>".
   One tap switches it to "New document" and back. The default is continue;
   the override resets when the in-view document changes and after each
   send. No Texture document in view: no chip, and the submit is unchanged.
   The chip lives inside the existing command field (no new band, the
   prompt bar's layout and height are unchanged), uses colour and font
   tokens only, and truncates the title at phone width.
   On a continue receipt the shell focuses the document's window (or opens
   one on the document if it was closed) and hands the window the receipt;
   the window follows the new trajectory (the same rule as its own revise,
   F1) and shows the desk as working. No new window opens.
3. **No model routing, no Jev, no new setting.** The chip is visible and
   overridable on every send, so it is its own switch. Rollback is a revert.

## Failure modes (written before the code)

- P1. A prompt-bar continue creates a document or a conductor run.
- P2. Continue on a live document does not commit an owner revision on its
  head, or wakes no desk.
- P3. Continue on a settled document is refused, or does not move it to a
  new live trajectory (F1 regression through the new caller).
- P4. Another owner's document is revised, or its existence is disclosed
  (a 409/403 instead of the 404 a missing document gets).
- P5. A cancelled document is continued.
- P6. A missing `continue_doc_id` changes the existing new-document path.
- P7. The owner's text is altered between the prompt bar and the revision
  (`owner_prompt` differs from the submitted text).
- P8. A retried submit with the same `command_id` commits two revisions.
- P9. A desk reaches an owner-authored Texture write through the prompt bar.
- P10. `/revise` changes behaviour after the refactor (its existing tests,
  F1's included, stay green).
- P11. The chip shows with no Texture document in view, does not default to
  continue, cannot be overridden in one tap, or the override leaks to the
  next document.
- P12. The chip adds a band, hides or covers the prompt, or uses colour
  literals.
- P13. A continue opens a new window instead of focusing the document's, or
  the window stays on the settled trajectory's stream.

## Residuals known at design time

- Unsaved edits in the window are not saved first: the revise commits on the
  stored head. The window's own revise saves first (`ensureCurrentRevisionSaved`);
  the prompt bar does not reach into the window. The local draft stays in
  the window's draft store.
- "In view" is the active window only; a Texture window visible behind
  another app's window gets no chip.
- The default continue is a product change on deploy for every owner; the
  chip makes it visible and one tap undoes it per send.

## Found while building

- A retried continue with the same `command_id` is not a replay: the
  revise's head check answers first (the head is already the owner's
  revision), so the retry is a 409 and nothing is committed twice. `/revise`
  behaves the same today with the same client request id. P8's test asserts
  the real guarantee (one revision, the retry a replay or a 409). A true
  replay needs the revise to look up its command before the head check: a
  change to the shared revise path, left out of this slice. The frontend
  sends a fresh `command_id` per send, so only a transport-level retry of
  one request can hit it, and it shows as an error after a successful
  revise.

## Rollback

Revert the implementation commit. The API field is optional and the old
frontend never sends it; the revise refactor carries no stored-state change.
Revisions already committed by a prompt-bar continue are ordinary owner
revisions on the document's chain.
