# Prompt bar targeting audit (origin/main, 2026-11-04)

All paths are on origin/main. V = verified by reading the code or commit. H = hypothesis.

## 1. End-to-end path

1. Frontend input. `PromptSurface.svelte` Enter handler dispatches `promptsubmit` with `{text, continueDocId, continueWindowId}` (`frontend/src/lib/PromptSurface.svelte:89-102`). V
2. `Desktop.svelte` `handlePromptSubmit` (:1246-1262) stashes the prompt for auth if signed out. Otherwise it calls `submitPromptText` (:1297). V
3. `submitConductorPrompt` POSTs `/api/prompt-bar` with `{text, command_id, continue_doc_id?}` (`frontend/src/lib/conductor.js:7-31`). V
4. Route: `internal/apihandler/routes.go:19-20` maps to `HandlePromptBar` and `HandlePromptBarSubmission`. V
5. Backend `HandlePromptBar` (`internal/textureowner/prompt_bar.go:82-137`) takes exactly three fields and refuses unknown ones (:94). V

Routing branches (every one):

| # | Condition | Result | Where |
|---|---|---|---|
| A | Signed out | Auth prompt; the prompt is replayed after login | `Desktop.svelte:1249-1256` |
| B | `continue_doc_id` present | Owner revise of that Texture document. 202 with `choir.prompt_bar_continue.v1`, no conductor run. | `prompt_bar.go:111-113, 145-160` |
| B2 | Same, but the caller is in-process (empty RemoteAddr, a desk) | 403: "only the Texture desk writes a Texture" | `prompt_bar.go:146-149` |
| C | Text is exactly one whitespace-free token that parses as an absolute http(s) URL | Conductor decision `open_app` with `app = AppHintForMedia(media type)`. "files" becomes "browser". Texture is not involved. | `internal/content/content.go:1639-1660`, `prompt_bar.go:210-213, 235-238` |
| D | Everything else (the default) | `requestedApp = agentprofile.Texture` (:207), decision `open_app/texture`, then `EnsureTextureHandoff` creates a new document and trajectory with `HandoffKindUserPrompt` | `prompt_bar.go:206-246` |

On the client, the decision is polled, then handled in `Desktop.svelte:1320-1360`:
- `toast` shows a toast.
- `open_app` with texture opens a Texture window.
- `open_app` with any other app opens that app with `sourceUrl`.
- Anything else shows "unsupported route".

The decision is not model-routed. `CompletePromptBarDecision` is documented as "deterministic" (`internal/agentcore/texture_model_api.go:40-42`). The conductor desk row has no verbs, targets or tools (`internal/deskmanifest/manifest.go:300-304`). V
Whether the `conductor` model role or prompt overlay is used by any run is H (the plan makes the same call).

Does nearly everything go to Texture? Yes. Only a bare URL goes elsewhere, and a focused Texture document also takes the text, as a revise. V

## 2. Focus

- The prompt bar knows the active window, but it reads only one thing from it. `promptContinueTarget(windows, activeWindowId, {authenticated})` (`frontend/src/lib/prompt-target.js:9-19`) returns a target only if the active window is:
  - app `texture`;
  - not closed, hidden or minimized;
  - holding a `docId`;
  - not a published or guest preview.
  Every other focus (Mail, Autopaper, Browser, none) returns null. V
- What the user sees: a chip, shown only when a target exists, reading "Continue <title>" or "New document" (`PromptSurface.svelte:352-374`). Tapping it toggles between the two. V
  - There is no indicator for any other app.
  - There is no "Conductor" or "no focus" label.
  - The placeholder and status text are generic ("Routing through conductor...", `Desktop.svelte:1306`).
- What is sent: `text`, `command_id` and `continue_doc_id` only (`conductor.js:14-18`). No focused app id, no selection, no cursor or range, and the window id stays client-side (it is used to refocus the window afterwards, `Desktop.svelte:1280`). V
- Focus changes routing only for the focused-Texture case.
  - An exception: a single URL defaults to "new document", which then reaches the display app (`prompt-target.js:21-35`, `PromptSurface.svelte:75`). V

## 3. Prompt bar to Texture connection

- It is this "continue" chip. It is not specific to self-development Textures. Any focused owner Texture document qualifies. The self-development link is indirect: self-development requests are Texture documents, and the Texture root casts to management (`docs/archive/reports/selfdev-readiness-2026-10-15.md:66-85`: "The request then enters through the prompt bar... returns a trajectory_id and a Texture doc_id"). So a focused self-dev request document can be continued from the prompt bar like any document. V
  I found no self-dev-specific code in `prompt-target.js` or `prompt_bar.go`. H that the owner remembers it as self-dev because that was the motivating workflow.
- Mechanism: `continuePromptBarDocument` calls `reviseOwnerDocument`, the same core as `POST /api/texture/documents/{id}/revise`. It is an owner-authored revision (the owner's text is the `owner_prompt`), not a cast to the Texture desk (`prompt_bar.go:139-160`). V
  - The Texture desk then handles the revision as it does any owner revise.
  - A settled document continues on a new trajectory (F1).
- When: `d338007b1` (2026-10-26), "texture: the prompt bar continues the document in view through the owner revise". Design doc commit `c33a18f00` (2026-10-26). Tests `983208f15`. Review-284 fixes `5d07e0c75` (URL defaults to new document). V
- Why, from `docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md`:
  - The owner typed the next step into the prompt bar and got a second new document.
  - Owner direction 2026-10-26 00:5x: "when it makes sense, build on top of existing documents."
  - F1 (a settled document accepts revision) had just made it possible.
  - Residual register FE-02 (`docs/evidence/residual-register-2026-10-26.md:76`): the conductor is rules only; "a Jev router with an overridable chip is recommended"; the owner "chose direction A", and this commit is A's step 1.
  So the chip is the overridable-chip half of FE-02. The router half (MA4.8) never shipped. V

## 4. App agents and app packages

- No. The package manifest has no `accepts` field. Fields are `opens` (media type plus file extensions, for Open with), `kinds`, `uses_kinds`, `desk` (must be null before P4), `backend` and `grants` (`internal/apppackage/manifest.go:55-90`, :196-203). V
- `opens` routes files, not prompts. The plan (section 9.2, "Candidates") cites `manifest.go:88-94` as the `accepts` source; that is `opens`. `accepts` is a new field the router build must add. V
- The prompt bar sends nothing to a non-Texture desk.
  - Texture is hard-coded (`prompt_bar.go:207`), plus the URL display apps.
  - The Email desk row exists (`manifest.go:306-310`) but has no prompt-bar route.
  - Only Texture has an app desk today. Package desks are null until P4. V

## 5. Autopaper articles

- They are Texture documents, published. `AutopaperStory.svelte` links to `/pub/texture/<slug>` and `/pub/texture/<slug>/v<n>`; `autopaper.ts:9-17` says there is no separate Autopaper page. Op-eds are "connected Texture publishing with an Opinion section" (`docs/ACTIVE.md:229`). V
- Could a user comment on one through the prompt bar today? No. V
  - `promptContinueTarget` rejects any window with `publishedRoutePath` or `publishedGuest` (`prompt-target.js:16`).
  - An Autopaper reader window is not app `texture`, so it has no target either.
  - Even if it were targeted, `continue_doc_id` means owner revise of the owner's own document, and the owner does not own a published article. The article belongs to the paper computer. The server returns 404 "document not found" for a non-owned document (per the design doc; `reviseOwnerDocument` is owner-scoped).
  - A prompt typed with Autopaper focused creates an unrelated new Texture from the text. The article is not linked.
- Missing:
  1. The prompt bar must carry a typed reference to the focused article (object id, slug and version) and any selection.
  2. A reply or response kind and writer. Per owner rulings, an app desk is the sole agent writer of its kinds, so this is likely an Autopaper or Texture response kind. A letter or op-ed is a new Texture with a `responds_to` link, not a revise.
  3. A route from a letter or op-ed to the editor, for example through the Autopaper object graph and a precommitment or settlement step.
  4. Router choices: "comment", "letter", "op-ed".
  5. Visible focus showing "Responding to <article>".
  The existing op-ed machinery (paper-side, connected Texture) is the closest piece. I did not trace whether it accepts external input. H.

## 6. Gaps against the owner's model

| Owner's model | Reality | Verdict |
|---|---|---|
| Universal input for all app agents | Only Texture (revise or new) and bare-URL display apps. No `accepts`, no package desks, no Mail route. | Gap |
| Shows which app agent is focused | Chip only when a Texture document window is active. Otherwise nothing. | Gap |
| No focus goes to the conductor | No focus goes to the deterministic default (new Texture), not to a router. There is no "Conductor" label. | Partial: the default is the router, but it is a rule, not a router |
| Prompt bar to self-dev Textures connection | Exists as the generic Texture continue chip (2026-10-26), motivated by FE-02. Not self-dev specific. | Matches, but narrower and broader than described |
| (implicit) focus is the target | Focus reduces to "active window is an owner Texture doc". Other apps' focus is ignored. | Gap |

Other inconsistencies:
- The chip's default is "continue" for any non-URL text. Typing an unrelated idea with a document focused revises the document. The only recourse is the toggle. V
- Plan comment and code agree that the URL classifier requires exactly one token (`content.go:1639-1660`). "Check out https://x" goes to Texture. V
- The client also polls a conductor decision for what is a synchronous rule. This is a leftover of the planned model router. H

## Where this sits in the plan

- Section 9.2 (MA4.8) matches this audit.
  - Today's conductor description matches the code at the cited lines.
  - The router replaces the Texture default with a System 1 typed choice over candidates from `accepts`, which does not exist yet. MA4.8 must add `accepts` (it cannot reuse `opens`).
  - The existing continue is the "continue the focused document" standing choice, so it is already built.
  - Not yet in 9.2, but the owner's model needs it: a focus-aware request (app id plus object ref plus selection) as router context, so the Autopaper comment, letter and op-ed case becomes a candidate.
- Phase 5 H3 (`prompt-bar` package): the front end pairs with the router. Visible-focus chip generalisation (any focused app agent, "Conductor" when none) belongs there, or earlier if the owner wants the label before the router.
- Section 11 gate: H3 is red, tier 2. Changing the request body from three fields to a focus context touches the strict-decoder API (`DisallowUnknownFields`, `prompt_bar.go:94`), so it needs a coordinated frontend and backend change.
