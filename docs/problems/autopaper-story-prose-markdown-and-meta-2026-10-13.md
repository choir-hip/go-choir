# Published stories show raw markdown bold and Texture's own status notes

Seen 2026-10-13 on the paper's live stories (screenshot of "Ukraine war: is
Russia's offensive ability broken?", v5, from the Texture published view):

1. Inline bold renders literally: the paragraph reads "**Status.** Revision
   3. ..." and "**The answer so far.** ...". Texture's Write markdown subset
   (internal/texturewrite) parses block structure (headings, lists, quotes,
   fences) but apparently not inline emphasis, so ** reaches the published
   text as characters.
2. Texture narrates its own process in reader prose ("Status. Revision 3."),
   which its prompt forbids (later revisions rewrite ideas, never narrate).
   The trace of that story's Write turns will show whether the prompt core
   (internal/textureprompts/texture.yaml, rewritten for Write) lost the rule
   or the model ignores it.

Next observation: the story's Write turns (paper computer
computer-1355b83b…) and the texturewrite inline parser. Fix candidates:
inline emphasis marks in texturewrite and the renderer, and the
no-narration rule restored in the prompt core if the trace shows it is
missing.

## Evidence, 2026-10-13 (front page and published versions)

Public payloads: `GET /api/platform/publications/front?limit=100` (30
stories) and `/resolve?route=<bare>` for each, scanning the published
`body_doc` text nodes.

- Literal `**` in published text: 5 of 30 stories (Ukraine war v7: 18
  occurrences; Federal agents/Omar v7: 24; Epic Games v4: 14; Thunderbird
  v3: 4; Switch to Safari v5: 2). No text node in any story carries a
  mark: Write's inline parser (`internal/texturewrite/inline.go`) splits
  only `{{tokens}}`, so `**` stays characters. The schema already allows
  strong, emphasis and code marks, and both renderers (frontend
  `renderStructuredTextureDocHTML`, `internal/platform/publication_document.go`)
  already draw them.
- Process narration in reader prose, 4 of 30 stories:
  - Ukraine war, every published version v2..v7 opens with a
    `**Status.**` block that Texture carries forward and re-numbers:
    v2 "Status. Opening revision. Management delivered one captured report
    and one hypothesis: ... (H1, p=0.60)", v3/v4 "Status. Revision 2.",
    v5 "Revision 3.", v6/v7 "Revision 4.". The block is born in the first
    revision and kept by each later Write.
  - Epic Games v4: "**Status.** Epic Games has publicly confirmed ...".
  - Thunderbird v3: a section "What the management desk reports", items
    "H1, management's probability 0.85", "Management's stated stakes", and
    "A research round is opened to confirm or refute both".
  - Breaking Bad v3: "The item that first reached us, forwarded by
    management ..."; AI-infrastructure v2: "As of this revision nothing is
    confirmed."
- Prompt: the core (`internal/textureprompts/texture.yaml`) kept "Every
  later revision rewrites the ideas, not a log. Never narrate actions." but
  Cut 2 (f93fdcb9) dropped the retired run_system overlay's rules that
  covered this shape: "do not turn coordination metadata into reader-facing
  prose", "do not explain the document's evidence standards to the reader",
  "Keep those reasons outside canonical document prose". Nothing in the
  core names status lines, revision numbers, desks or hypothesis labels,
  and the frame shows management's hypotheses with their labels and
  probabilities, which Texture copies. The Write turns of the paper
  computer were not read (owner-authenticated trace); the claim that the
  model copies frame vocabulary for want of a rule is a hypothesis from the
  published series and the prompt diff.
