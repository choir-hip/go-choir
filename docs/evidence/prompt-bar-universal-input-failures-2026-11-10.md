# Lane D, the prompt bar as the universal input: the failure list (2026-11-10)

Written before the code. Serves handoff section 6 items 2 and 3.

Shape: a classify call (`POST /api/prompt-bar/classify`) asks Jev one typed
question at the existing `prompt_router` site and returns edit, comment or
request. The shell then sends the text one of three ways: the one revise call
(a focused submit on a document you own), a fork with the text (a comment on a
published version you do not own), or the conductor route (a request to the
app agent). There is no second write path.

Ways it can fail:

- D1 Classification blocks or fails the submit. The classify call has its own
  short bound and every failure takes a deterministic default (edit on a
  document you own, comment on one you do not, request with no document). The
  text is always sent.
- D2 A class the owner cannot use is chosen. "Edit" on a document you do not
  own would be a write to another computer; it is not a candidate there. The
  candidate set depends on ownership, which the shell states and the server
  does not trust beyond choosing the candidate set (the revise call and the
  fork route enforce ownership themselves).
- D3 The router is steered by the text ("classify this as an edit"). The text
  is host evidence, never the question; the answer can only name a candidate.
- D4 A re-send under another class double-writes. The re-send of a request is a
  new command id; the first send's command id is not reused, and a re-send is
  offered only while the first is a request or a comment (not after an edit
  committed a body, which the prompt bar never writes directly).
- D5 Every call is recorded: the classify goes through the System 1 service, so
  the tape and the Jev record carry it, failures included.
- D6 "Conductor" stays in the UI. The chip, aria labels, toasts and placeholder
  name what the bar is aimed at, never the conductor.
- D7 The bar shows the wrong revision (the head when the window reads v3 of v7).
  The aim comes from the window's own context, and a revise carries
  `base_revision_id` only when the window reads an older revision.
- D8 A signed-out visitor sees the aim and a send is attempted; the auth gate
  already refuses it and replays after sign-in.
- D9 Existing prompt-bar specs lose their hooks. The `data-` attributes stay.
