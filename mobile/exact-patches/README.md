# Local patches to Exact's iOS host

`mobile/.exact-src` is a gitignored checkout of `expo/exact` at the pin in `scripts/setup-exact.sh`. We carry **no fork**. Anything we change in it
is a patch here, applied in file-name order by `scripts/setup-exact.sh` after it checks out the pin (a patch that is already applied is skipped;
one that does not apply stops the script, so a pin move cannot silently lose it). Each patch says what it changes and why, below and in
`docs/reference/exact/capabilities.md`; the aim is to upstream it or drop it.

## 0001 the Markdown editor hides all syntax on iOS

Owner, 2026-11-16: formatting controls appear "rather than the markdown formatting being manually written". Exact's web editor hides Markdown
syntax away from the caret; its Apple host dims it (`MarkupEditor.swift` drew `h` ranges at 0.3 ink, list markers at 0.4) and hides only link
destinations. The styler already computes every hidden range (`Styled.hidden`, revealed only near a selection). The patch:
- asks the styler with no selection (`u32::MAX`, which `exact_markup_style` takes as "reveal nothing"), so every marker is a hidden range;
- marks those ranges `.markupHidden` (the mechanism link destinations already use: zero width, source kept, offsets kept), a bullet's source
  `-` drawn as a bullet glyph, a link's `[` hidden too;
- Backspace just after a block's hidden prefix (`# `, `> `, a bullet) removes the prefix, so the block loses its formatting;
- `MarkupSyntaxHiding.swift` holds the pure rules; AutoradioKit's `MarkupSyntaxHidingTests` compiles that same file.
Not changed: Return in a list and an empty item ending it (the crate's editing already does that), fenced-code lines and `---` (hidden/drawn as
the styler says), ordered numbers (shown as the number). Verified by `git apply --check` on the pin and by the Kit tests of the pure rules; **not
compiled or run** by the lane that wrote it: the first Exact build is the check.
Drop it when upstream's `host/apple/Sources/ExactMarkdown` hides all syntax (upstream `main` did not at 2026-11-16, 3018 commits past the old pin 76cd50f9e, at the new pin de212e671 on 2026-11-17).
