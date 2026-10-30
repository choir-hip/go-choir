# Published Textures end with a source list instead of a Sources button (2026-10-13)

Status: open; reported by the owner in session.

## Observation

Owner, 2026-10-13: "textures from autopaper (and maybe all textures in
general, uncertain) have a list of sources at the bottom of their content.
that's wrong. we used to have a sources button in the texture menu bar. we
should have that again, which should expand the sources section above the
content of the texture. but while it's collapsed (by default) users just see
the transclusion/citation points."

## Evidence

- `frontend/src/lib/TextureEditor.svelte` `renderDocumentHTML` appends
  `renderPublishedSourcesHTML(citations)` after the structured body whenever a
  publication bundle carries citations, so every published Texture with
  citation anchors (all Autopaper stories) ends with the numbered list.
- `frontend/src/lib/TextureToolbar.svelte`: in published mode the toolbar shows
  Copy text, Download and Edit (reader) or Propose (derivative); the Sources
  button exists only in the editing branch, where it opens
  `TextureSourcePanel`.
- Unpublished Textures do not render a list at the bottom; only the published
  path does.
- The deployed reader spec (`frontend/tests/autopaper-reader-deployed.spec.js`)
  asserted the list inside the reader, so it pinned the wrong shape.

## Contract

The text carries only its numbered citation markers and transclusions. A
Sources button in the toolbar (published reader and derivative) opens the
numbered source list above the text; it is closed by default.
