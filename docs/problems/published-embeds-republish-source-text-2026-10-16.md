# Published stories republish source text as blocks instead of transcluding it (2026-10-16)

Status: open; reported by the owner.

## Observation

Owner, 2026-10-16, with three phone screenshots of paper stories:
"one thing I don't like is these source sections. They are all bad. We
have a transclusion ui. But these aren't that. These are just us
republishing other content. It's bad. Ugly. Senseless."

The screenshots:

| Story | What the box shows |
|---|---|
| Citizen Lab links targeted online ads to zero-click spyware attacks (v4) | one paragraph of the Guardian piece, captioned "[3] 'Nightmare Scenario': Zero-Click Spyware Is Infecting Phones via Targeted Ads" |
| EU leaders rally behind Zelensky … (v12) | page furniture from the BBC capture: "Image source, Reuters", "Image caption,", the photo caption, "ByTiffany Wertheimer" |
| Commercial goods stay out of Gaza … (v26) | three boxes in a row: a deck sentence; a one-line excerpt from a second article; "Save", "Share", a photo caption |

## Evidence (code)

- The boxes are `{{embed <handle>}}` citations. The Texture Write grammar
  turns `{{embed}}` into a `source_ref` with `display_mode: expanded_ref`
  (`internal/texturewrite/apply.go`). With no quote it pins the capture's
  blocks; the platform records the anchor with relation `embeds`
  (`internal/platform/publication_versions.go`).
- The published reader draws each one as a block:
  `renderPublishedEmbedBlock` (`frontend/src/lib/texture-published-citations.ts`)
  prints up to eight pinned capture blocks verbatim as blockquotes, with
  a "[n] title" caption. `renderParagraph`
  (`frontend/src/lib/texture-structured-editor-doc.ts`) splits the
  paragraph around it. Captures keep page furniture as blocks, so the
  box shows "Save", "Share" and photo credits.
- Unpublished documents do the same through `renderLocalEmbedBlock` and
  the `expanded_ref` branch of `renderInlineSourceRef`.
- The desk is told to do this. `internal/yaegikernel/surface.go` (Write)
  says "transclude with {{embed r2.c1}}", and
  `internal/textureprompts/texture.yaml` says "{{embed r2.c1}} transcludes
  a source as a block". Tests pin both texts.
- It is common. In the 78 opening Texture runs surveyed this morning,
  24 transcluded the capture whole with `{{embed r1.c1}}`
  (docs/problems/texture-opens-stories-blind-to-their-capture-2026-10-16.md).
- It is a rights exposure too.
  docs/research/autopaper-media-and-rights-2026-10-14/README.md rates
  public transclusion of capture blocks as moderate risk for press text,
  because it "reproduces expression on a public page".

## The transclusion UI that already exists

A `{{cite}}` renders as an inline numbered marker. Its popover shows the
source title, the quoted words marked inside their pinned block, the
quote check badge, and "Open source". The Sources button in the toolbar
lists the numbered sources above the text. That is the transclusion the
owner means: the source stays the source's, and the story links into it.

## Settled-decision note

Embeds as blocks were built on purpose (1764afe83, 2026-10-13; the Cut 2
definition's `{{embed}}` block node). The owner's word above supersedes
that rendering. It does not ask for any change to stored citations.

## Fix direction

1. **Reader (orange, frontend):** draw every embed, published or not, as
   the same inline numbered marker a cite gets. A quoted embed's popover
   shows its quote in its block, as a cite's does. A quoteless embed's
   popover shows the title and "Open source", never the capture's text.
   Delete the block renderers. This fixes every existing story at once,
   since the reader renders the stored body. Stored `expanded_ref` nodes
   keep their mode, so an owner edit serializes them back unchanged.
2. **Desk surface (yellow):** stop teaching `{{embed}}` in the Write
   surface and the Texture prompt, so new stories cite passages. The
   grammar still parses it, so old frames and stray uses do not fail.

Not chosen: refusing `{{embed}}` in Texture writes (a red change to
canonical writes, and the reader fix already removes the republishing);
capping embed length (still republishes, and still ugly).
