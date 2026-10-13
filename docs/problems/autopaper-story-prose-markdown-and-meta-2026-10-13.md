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
