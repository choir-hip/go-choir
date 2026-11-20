# Exact's Markdown editor, and what "pretext", "reflow" and "textflow" are

Read-only research of 2026-11-20. Exact checkout: `mobile/.exact-src` in the
`qa-mac` lane, pinned at `de212e671` (2026-11-16), compared with upstream
`origin/HEAD` = `fb72ddc3a` (2026-11-20), 760 commits ahead. Paths below are
Exact paths unless they start with `mobile/` or `docs/`. Every statement is
marked [src] (read in source; "pin" or "up" says which tree), [inferred] or
[unrun]. Nothing here was built or run. Siblings: `capabilities.md` sections 4
(text flow) and 5 (Markdown) hold the earlier notes; this page corrects and
extends them.

## The answer in half a page

What exists. Exact has one Markdown editor: `textarea markup="markdown"`.
It is WYSIWYG only on the web (syntax hidden away from the caret). On iOS and
macOS it is the platform text view (TextKit 2) with the syntax dimmed, not
hidden, except each link's `](destination)`. On Android, Linux and Windows
there is no rich editor: the source is edited as plain text and `format` and
`setRangeText` are refused [src, up `docs/contract-grammar.md:1804-1810`].
Atoms (citation pills) work in the editor and the reader on web, iOS and macOS
[src]. The editor holds the text, the selection and the undo itself; the app
is told the whole value plus selection on every edit, and writes `value` to
change it [src].

What "pretext" is. Not a part of Exact. Pretext is Cheng Lou's text-layout
library (a third party; Exact's own words: "Seven studies after Cheng Lou's
Pretext demos", `apps/reflow/README.md:3`) [src]. Exact declined to take the
library ("Take: no Pretext library", `rules/DEFERRED.md:196` at pin) and built
its two ideas instead [src]. The nearest things are `textflow/` (a Rust crate:
text around shapes and line breaking by arithmetic) and `apps/reflow` (a demo
whose Rust data crate measures heights before layout). "Reflow" is an Exact
demo app, not a feature. "Textflow" is the crate and its demo app. Neither is
a text editor feature: they lay out read-only `text`, not a `textarea`
[inferred; no editor code uses them, see section 2].

What we use. The reader's `text ... markup="markdown" atoms="citation"` and
the editor `textarea id="texture-editor" markup="markdown"` (section 3), plus
our own patch that hides all syntax on iOS (`mobile/exact-patches/0001`),
which upstream has not made unnecessary and which still applies to upstream's
files [src, checked with `git apply --check`, section 1.6]. Behind the editor
the phone still runs Swift for save, merge and the offline copy.

What we should use. Lean on the editor's own state: the field is the draft.
Keep one kept record per document written as one pair (the field's text, the
text of the server revision it descends from), and make every merge write its
result into the field through `value` (section 3). That removes the class of
defects the three reviews found around "the merge never reached the field" and
"typing was dropped while saving"; it does not by itself remove the two
persisted-record findings of round three, which are ours to model.

What it takes. A pin move is not needed for the editor: the editor's Apple
and Rust code is unchanged between the pin and upstream except one Rust fix
(section 1.5). The pin move is large and unrun (section 4); do it for its own
reasons, not for the editor. The one real gap is Android: if the Texture
editor must be rich on Android, stock Exact cannot do it; it would be a plain
source editor plus a Markdown preview, which is Exact's own advice for those
hosts (section 1.4).

## 1. The Markdown editor

### 1.1 Where it lives

- The shared rules are Rust: `markdown/` (parser and styler, `markdown/src/`)
  and `markdown/editor/` (editing rules: caret around hidden syntax, Backspace,
  list Return, pending formats, undo) [src, pin `markdown/editor/src/lib.rs:1-12`].
  Its header says syntax is "hidden, never removed" and that the editor is
  "optional by construction" [src, same lines].
- The web ships the editor as its own wasm, fetched only when a Markdown
  textarea mounts [src, pin `markdown/editor/src/lib.rs:11-12`;
  `host/web-js/pieces.js:413-421`].
- iOS and macOS do not use that editor to draw; they use TextKit 2 text views
  and the styler's ranges: `host/apple/Sources/ExactMarkdown/` (`MarkupEditor.swift`,
  `MarkupEditorIOS.swift`, `MarkupEditorMac.swift`, `MarkupHidden.swift`,
  `MarkupAtoms.swift`, `MarkupCommands.swift`, `MarkupDirections.swift`,
  `MarkupRuns.swift`) [src, pin].
- Android, Linux and Windows draw the source in Exact's painted field; an
  Android `EditText` hosts every other field but never a Markdown one, "because an
  EditText shows plain text" [src, up `host/painted/src/fields.rs:17-18`, `:110-114`;
  `host/android/README.md:968-970`; the same lines exist at pin,
  `host/android/README.md:897`].
- The demo apps are `apps/markdown` (a reader app) and `apps/markdown-stress`
  (editor, toolbar, link sheet, selection state) [src].

### 1.2 WYSIWYG, per host

| Host | What the editor does | Source |
|---|---|---|
| Web | "Markdown shown as it reads, with the syntax hidden away from the caret". Syntax hidden only near the caret. | [src] up `docs/contract-grammar.md:1804-1806`; pin `:1681` equivalent |
| macOS | Platform text view; syntax dimmed; link destinations hidden. | [src] same |
| iOS | Same as macOS. With our patch 0001, all syntax is hidden. | [src] same; `mobile/exact-patches/README.md` |
| Android | Plain source, painted field; no `format`, no `setRangeText`; atoms and `paragraphDirs` are "still out". | [src] up `docs/contract-grammar.md:1807-1809`; `rules/DEFERRED.md:267-269`, `:288-290` |
| Linux, Windows | Plain source; `format` logs a refusal there. | [src] up `host/painted/src/presenter/commands.rs` (commit `e80133a2c`) |
| Terminal | Refuses `setRangeText` for every field. | [src] up `docs/contract-grammar.md:2738-2740` |

Exact's README says the same in a list of what is not there: "Markdown editor
off web and Apple: Linux, Windows, and Android show plain source"
[src, up `README.md:557`]. That sentence is new since the pin [src; absent in
pin `README.md`]; the pin's grammar named only Linux
(`docs/contract-grammar.md` pin, Markdown section) [src]. The Android
limitation is old; upstream wrote it down.

Does upstream hide all syntax on Apple? No. Upstream's `ExactMarkdown` sources
are byte-identical to the pin's for the files our patch touches
(`MarkupEditor.swift`, `MarkupHidden.swift`); the only Apple Markdown changes
in 760 commits are two new icon files and a one-line host change (`AtomIcons.swift`,
`TextAtomIcon.swift`, `MarkdownHost.swift`) [src, `git diff HEAD origin/HEAD --stat
-- host/apple/Sources/ExactMarkdown`]. Upstream's own queue says it is still
owed: "Native syntax stays visible at full advance but for link destinations
... Owed: the Apple hosts adopting the rest of `exact-markdown-editor` (the other
hidden syntax, its input rules), which waits behind LLP 1045.000's inline atoms"
[src, up `QUEUE.md:1022`; the same text at pin `QUEUE.md:947`]. The atoms it
waits for are built at the pin [src, pin `docs/contract-grammar.md:1745`], and
`rules/DEFERRED.md` still says "Take: the Apple hosts' adoption of the crate's
other hidden syntax ... waits behind atoms" [src, up `rules/DEFERRED.md:262-264`].
So the wait has no new date. We cannot drop patch 0001 on a pin move.

### 1.3 "The kept editor", "painted hosts", "source editor", "stage 2"

- "The kept editor" is not the Markdown editor. It is a test fixture for
  kept documents (a document a user chose, remembered across launches): `scripts/fixtures/kept-editor/kept.drives.mjs`,
  "D4's hard rows by the kept editor" in the "Kept documents, what stage 2 left"
  queue item, and commit `8eda2824f` "The kept editor's drive passes on macOS" [src, pin `QUEUE.md:569`;
  up `git log`]. It concerns file grants and atomic save, not Markdown.
  Nothing in it is something the owner can use for Texture.
- "Painted hosts" are Exact's own-paint hosts: Linux, Windows and Android
  (`host/painted/`), as opposed to Apple and the web, which use platform views
  [src, `host/painted/`, `host/android/README.md:960-975`].
- "The Markdown source editor on painted hosts" is commit `e80133a2c`
  (2026-11-20, "Explain the Markdown source editor on painted hosts") [src,
  up `git show e80133a2c`]. It changed docs and the refusal path only: on
  Linux, Windows and Android the editor edits the source as plain text,
  `setSelectionRange` takes source offsets, and `format` and `setRangeText`
  report a refusal in the app log. Its advice for apps that want a preview there:
  "bind a plain textarea and a `text markup="markdown"` reader to the same string"
  [src, up `docs/contract-for-agents.md` section on `setRangeText`, same commit].
- "Stage 2" in the queue is unrelated to the editor in every hit (notification
  actions, kept documents, share extension, canvas, and so on) [src, grep of
  `QUEUE.md`].
- "Native Markdown undo boundaries" and "Markdown editor" are the queue items
  that do describe the editor's open work: one-command-one-undo on native is
  "incomplete", and "phone composition/autocorrect/dictation/VoiceOver/selection
  ergonomics" and "native intrinsic editor sizing and drawn list markers" are
  owed [src, up `QUEUE.md:1020`, `:1022`; `apps/markdown-stress/README.md:492-512`].
  Their proofs are simulator proofs only [src, same README].

### 1.4 The contract surface

All of this is in `docs/contract-grammar.md`, section "Markdown: `markup`,
`format`, `select`" (up `:1786-1868`; pin `:1663-1744`) and "Inline atoms"
(up `:1869`, pin `:1745`). The two versions of both sections differ only in the
Linux/Android wording above (checked with `diff`) [src].

- Value: `value=draft`, the value stays the Markdown source, so "storage, search
  and export are the string" [src, up `:1788-1792`].
- Events: `input` (and `beforeinput`, `selectionchange`) carry an `InputEvent`
  with the whole value, `selectionStart`, `selectionEnd` (UTF-16 offsets into
  the source), `inputType`, `data`, `isComposing` [src, up `:1840-1846`, `:2676-2696`].
  The change is not delivered as an edit or a range: it is the whole value
  plus the selection after the edit plus the kind and inserted text. The
  proposed edit and the old value come with `beforeinput` [src, up `:2758-2764`].
- Selection state: `select` on the editor delivers a `MarkdownSelection`:
  `formats` (space-separated), `mixed`, `link`, `unavailable`, `atom`; "It
  carries no offsets" [src, up `:1828-1838`]. A toolbar reads a format as
  `` ` ${s.formats} `.includes(" bold ") ``.
- Commands: `format(id, command[, argument])` with `bold`, `italic`, `code`,
  `strike`, `link` (URL), `heading` ("1" to "6"), `bullet`, `ordered`, `task`,
  `toggleTask`, `quote`, `codeblock`, `footnote`, `figure` (media `src`),
  `indent`, `outdent`, `newline`; each is one undo step reported through `input`
  [src, up `:1817-1826`]. A toolbar button takes `retainFocus=true` [src, same].
- Programmatic edits: `setSelectionRange(id, start, end)` and
  `setRangeText(id, text[, start, end[, mode]])` take source offsets on web,
  macOS and iOS; each `setRangeText` is one undo step and is refused during IME
  composition [src, up `:1844-1847`, `:2724-2740`]. It fires one `input` with
  `inputType: insertReplacementText` [src, up `:2742-2750`].
- Atoms: `atom citation link="cite:"`, `atoms="citation file mention"`,
  `atomFaces=faces`, `atompress=pressed`. A caret cannot stand inside a pill;
  Backspace at its end removes its whole source as one undo step; a press selects
  it, then fires `atompress` [src, up `:1869-1890`]. This works on a `text`
  reader and the editor alike [src, same].
- `paragraphDirs`: per-line text direction; drawn on web, macOS, iOS only
  [src, up `:1850-1867`].
- IME: `input` fires at each composing change with `isComposing`; the app's
  value never ends a composition; a different value written during composition
  is held and lands when it ends [src, up `:2697-2713`;
  pin `host/apple/Sources/ExactKit/FieldComposition.swift:1-30`].
- Paste: on Apple, ordinary paste goes through the same edit path and reports
  `insertFromPaste` [src, pin `host/apple/Sources/ExactKit/FieldInput.swift:20-37`].
  macOS copies Markdown source, `Cmd+Shift+C` copies plain text; iOS has a
  "Copy Plain Text" edit-menu item [src, `apps/markdown-stress/README.md:84-88`].
- Links: `format(id, "link", url)`; link destinations are hidden on Apple and the
  web; iOS has a native "Link URL" alert in the edit menu
  (`MarkupEditorIOS.swift:59-80`, `markdownEditLink`) [src, pin].
- Lists, tasks, quotes, code blocks: the editing rules continue a list on Return
  and exit it on an empty item [src, `apps/markdown-stress/README.md:78-80`].
- Tables and images: the parser has `BlockKind::Table` and `Image`
  (`markdown/src/block.rs:20-23`) and a `figure` command (`markdown/src/edit.rs:480`)
  [src, pin]. The reader dialect "has no setext headings, indented code,
  reference links or HTML, and a list item holds one paragraph and its nested
  lists" [src, up `:1799-1803`]. How the editor shows a table or an image is [unrun].
- Footnotes: `format(id, "footnote")` exists. No tap on a footnote was found.
  `apps/markdown/README.md` says footnotes are not interpreted by the reader
  [src, pin `apps/markdown/README.md`].
- Undo: on Apple it is the platform's `UndoManager`, with the Markdown rules
  registering edits through UIKit's own insertion; on the web the crate's own
  history [src, pin `MarkupEditorIOS.swift:36-56`; `markdown/editor/src/lib.rs:6-9`].

### 1.5 What changed in the 760 commits that matters to an editor

Almost nothing in the editor itself. Commits touching `markdown/`,
`host/apple/Sources/ExactMarkdown`, `apps/markdown`, `apps/markdown-stress`
since the pin [src, `git log HEAD..origin/HEAD -- ...`]:

- `71594ac2c` "Preserve inline Markdown delimiters when deleting a line break"
  (Rust, `markdown/editor/src/editor.rs` +8 lines, test +25). A real editor bug fix.
- `e80133a2c` the painted-host explanation above.
- `f607bac14`, `567d7a64a`, `b6ece88ee`, `85c018aea`, `838193c5f`: the Apple bake
  and link composition ("Apple's composition names ... controls by use"; icons in
  text belong to the Markdown product). These change what a build links, not what
  the editor does.

What matters to long documents, by commit:

- Painted `textarea` (Linux, Windows, Android) got a performance series:
  "Reuse shaped and positioned hard lines across textarea edits" (`b6103d7ad`),
  "Pack short textarea rows into contiguous glyph storage" (`42bf25aa3`), "Reuse
  unchanged textarea rows for caret placement and hit testing" (`a8d823cd8`),
  "Paint textarea decorations without indexing the full document" (`7ed617b44`),
  scroll extents (`3379635b4`, `b88845548`), selection painting (`16745e9a0`)
  [src, `git log`]. Only the painted plain-source field benefits; it is
  what Android would use [inferred].
- The long-text rule is about the reader, not the editor: Core Text declines to
  shape a string over about 10,240 UTF-16 units, so a long document must be one
  `text` per block [src, `docs/recipes/long-text.md:54-62`, from commit `22be50514`
  of 2026-11-12, which is in the pin]. The editor is one TextKit 2 view; whether
  a 100,000-character document edits smoothly in it is [unrun]. Exact's tests of
  it are "fixed-height editor demonstrations" and the stress app's 4 MiB runs are
  of the reader [src, `apps/markdown-stress/README.md:89-91`].
- On the host side, this checkout's reader and field plumbing changed in many
  files (the `ExactKit` Apple module lost many files; section 4) but the field's
  value-write path is the same in meaning [src, section 1.6].

### 1.6 Our patch 0001 against upstream

`mobile/exact-patches/0001-markdown-editor-hides-all-syntax.patch` edits
`MarkupEditor.swift` and `MarkupHidden.swift` and adds
`MarkupSyntaxHiding.swift`. I extracted upstream's two existing files to a
scratch directory and ran `git apply --check` there; it passed [src, run,
2026-11-20]. That proves it applies; it does not prove it still compiles
against upstream's rest of `ExactKit` [unrun]. The patch's own note stands:
it has "not [been] compiled or run" by the lane that wrote it
[src, `mobile/exact-patches/README.md`].

### 1.7 What the host does with a value the app writes

This is the part that decides section 3.

- iOS (`writeValue`, `host/apple/Sources/ExactKit/IOS/TextAreaIOS.swift:646-662`
  at pin, `:649` upstream): a value equal to the field's text changes nothing;
  while text is marked (IME) the value is held and applied after; otherwise it
  replaces "the changed middle only, with the selection carried through"
  (`minimalTextEdit`, `carrySelection`), and it calls `undoManager?.removeAllActions()`
  because "native undo ranges refer to the old buffer" [src]. So a write from
  the app keeps the caret but clears the person's undo history.
- Web: `Editor::set_value` "replaces the changed middle and carries the
  selection; the history resets, as it does on Apple" (`markdown/editor/src/editor.rs:435-437`
  at pin) [src].
- macOS has the same function name at `TextAreaMac.swift:460` [src; body not read].
- There is no separate "remote update" primitive and no change event as an
  edit list. A remote change is an app write to `value` [src, inferred from
  the above].
- Android's Markdown field is Exact's painted field; its behaviour on a written
  value was not read [unrun].

## 2. "pretext", "reflow", "textflow"

Search: every case-insensitive hit in the pin and upstream, and in commit
messages [src].

### 2.1 pretext

- "Pretext" appears only as a reference to **Cheng Lou's Pretext demos**: in
  `apps/reflow/*` (README, contract, data crate comments), `apps/textflow/README.md:3`,
  `apps/textflow/app.contract:167`, `host/web/textflow-glue.js:75` ("Pretext's
  platform ledger"), `textflow/src/chrome.rs:2` ("Pretext's corpora showed where
  the walker and Chrome disagree"), `rules/DEFERRED.md` (pin `:188-205`, up
  `:202-218`) and the README app rows [src, pin and up, same hits].
- There is no crate, module, attribute or Contract keyword named pretext. Exact
  states: "Take: no Pretext library, no runtime font parsing" (`rules/DEFERRED.md:196`
  pin) [src]. Cheng Lou's Pretext is a separate JS library whose idea Exact's notes
  describe as two halves: text around shapes, and "heights before layout" by a
  `prepare`/`layout` split that measures advances once and then breaks lines by
  arithmetic [src, `apps/reflow/README.md:8-18`; `apps/reflow/app.contract:1-9`].
- The commit `f4571f536` (2026-11-13) says `Prepared::fit` lets a line run up to
  0.005 px past its offer "as Pretext's engine rounding does" [src, up `git show`].
- Nearest thing: `textflow/` (below) for the layout, `apps/reflow` for heights
  before layout. If the owner means Cheng Lou's library by name, Exact does not
  contain it.

### 2.2 textflow

- `textflow/` is a Rust crate, `exact-textflow` (`textflow/src/lib.rs:1-6`): "CSS
  shape values, exclusion bands, and arithmetic text flow". Its public pieces:
  `ShapeOutside`, `FlowShape`, `flow`, `line_breaks`, `Prepared` (advances and
  offsets, "never the source string or a font"), `balanced_breaks`, `collapse`,
  `chrome_break` [src, pin `textflow/src/lib.rs:1-40`]. About 3,800 lines [src].
- It is the one walker every host flows text with (`README.md`: "One Rust walker
  shared by every host") [src, pin `README.md:671`]. It is not a separate toolchain
  step; it is linked by the kernel and the hosts [src; capabilities.md section 4].
- Contract side: no function to call. An app writes style attributes on an
  obstacle box in the same container as a `text`: `wrap-flow="both"`,
  `shape-outside="circle()"` or a `part.outline`, `shape-margin` [src,
  `apps/textflow/app.contract`; `contract/lower/src/style_names.rs:85`
  ("css CSS Exclusions 1"); `contract/lower/src/tags.rs:1180-1200`:
  `wrap-flow: both` needs `position: absolute` and a `position: relative`
  parent]. The `text` flows around the box and re-lays out when the box moves,
  the width changes or the font size changes. A re-layout on resize is the
  normal layout pass, not an API [inferred].
- Also in Contract: `text-wrap="balance"` (up docs `contract-for-agents.md:7001`),
  `measure("id")` (an action function: "the border box as if `height` were `auto`"
  [src, up `docs/contract-grammar.md:1386`]) for heights, and per-block `text`
  nodes for long content [src].
- Hosts: web, apple, linux demo crates exist; no android crate for the demo
  [src; `apps/textflow/`]. Android use [unrun].
- Upstream since the pin: "Balanced text breaks where Chrome breaks it"
  (`a122a46ee`), balanced rounding (`65a194523`), CR/FF/NEL/U+2028/U+2029 breaks
  (`588790b02`), condensed and expanded text measured at width (`711820ecc`),
  a `white-space: pre` paragraph beside an exclusion (`bdbf92ede`), content regions
  carry resolved exclusions and paint justified flowed text (`2c03d1176`), a flowed
  slot takes the line the walker fitted (`f4571f536`) [src, `git log`]. All are
  refinements of text layout.

### 2.3 reflow

- `apps/reflow` is a demo app "Seven studies after Cheng Lou's Pretext demos" (balls,
  dragon, masonry, a magazine you drag, ASCII art, a 1,200-card wall, a spread)
  [src, `apps/reflow/README.md:3`, `:25-33`].
- Its data crate (`apps/reflow/data/`, Rust) shows "heights before layout": at
  build time `build.rs` reads glyph advances and `kern` pairs from the app's own font
  files; `typeset.rs` breaks lines by arithmetic with the same `exact-textflow`
  walker; the kernel receives text "already cut to the lines the arithmetic found"
  [src, `apps/reflow/README.md:13-20`; `apps/reflow/data/build.rs`, `typeset.rs`]. No
  font is parsed at runtime. Its measured gain: median 3.89 ms to 0.056 ms per
  one-pixel resize query on one Mac [src, same README; a data-query cost, not a frame
  time].
- "Reflow" is also used in the diary of Duo Lab (`apps/duo-lab/diary/2026-11-12-reflow-band-lines.md`) for
  the long Markdown text there, which is not the same thing [src].
- As used by a Contract app: copy the pattern (a Rust data crate depending on
  `exact-textflow`) or skip it. Cost for us: our data module is TypeScript;
  the README says "Delivering TypeScript and Rust together is not yet
  implemented" for over-the-air updates [src, up `README.md` "Not yet" list].
  A Rust source would be a new build and deploy path [inferred].

### 2.4 None of the three touches the editor

No `textarea`, `markup-editor` or `exact-markdown-editor` file mentions
textflow, reflow or pretext [src, grep]. The editor's line layout is the
platform's (TextKit 2) on Apple, the browser's on the web, the painted field's own
on painted hosts [src]. So "use the Markdown editor and their pretext/reflow/textflow
calculation" are two separate things in Exact. The textflow pieces help a *reader*
(text around a figure, balanced headings, heights before layout for a long list of
blocks). They do not make the editor flow around shapes [inferred].

## 3. How our app uses these, and the simplest correct Texture editor

### 3.1 Today

- Reader: `mobile/AutoradioLab/reader.contract:404`,
  `text rdoc.markdown markup="markdown" atoms="citation" atompress=mdPress`. Our
  own typography attributes sit on the node [src, ours]. Long documents have a
  block mode (`BlockView`) beside it; how well the one-node mode handles a long
  story with Core Text's limit is [unrun].
- Editor: `reader.contract:526`, `textarea id="texture-editor" markup="markdown"
  value=edDraft input=edInput select=edSelected`, in the `TextureEditArea`
  component, mounted when `rdoc.editable and edOn and edFor == address` [src, ours].
  `members-content.contract:25, 106, 116-121` hold `edDraft`, set it from
  `rdoc.source` at `edBegin`, write it in `edInput`, and send `textureEdit(address,
  markdown)` to the module on every input. `edSelected(s)` reads `s.formats`,
  `s.unavailable`, `s.link`; the toolbar calls `format("texture-editor", ...)`
  [src, ours]. All of that is the surface of section 1.4, so it works on iOS
  (and would on web and macOS); on Android `format` is refused and `select`'s payload
  is a plain-field `InputEvent` or absent [src for refusal; payload [unrun]].
- Patch: `mobile/exact-patches/0001` (hides syntax on iOS).
- Swift behind it: `mobile/AutoradioKit/Sources/AutoradioKit/TextureEditor.swift` (261
  lines) holds a session per document with `base` (the head as read), `draft`, a
  2-second pause then POST of the whole text, offline copy in Application Support
  (`pending`), a 30-second retry, and "the newest text wins; the parent is the head at
  that moment". It does no merge of text: it posts `parent_revision_id` and
  `TextureBodyMerge.merge(original, originalMarkdown, newMarkdown)` for the body
  document [src, `TextureEditor.swift:66-69`, `:121-150`, `:166-176`].
  `mobile/exact-package/apple/AutoradioModule+TextureEdit.swift` (40 lines) is the
  bridge: `textureEditInfo`, `textureEdit`, `textureFlush`, and a background
  notification flush [src, ours].
- The shared-code attempt (lane `exact-convo-edit`, `mobile/AutoradioLab/texture-edit.ts`,
  421 lines, not on main): states `ref`, `base`, `draft`, kept set, kept conflicts; invariants
  I1 to I4 in its header; a three-way merge of lines, a 409 retry [src, lane file head].
  The server refuses a stale parent with 409, per that file's header; I did not
  re-verify that on the server in this pass [inferred from the lane file].

### 3.2 What the editor gives us, and what it does not

Given by Exact [src, section 1.7]:

1. The field holds the text, the caret and the undo history. The app learns every
   edit as the whole value plus selection plus `inputType`, so the app's `edDraft`
   is the field's text at every `input`, composing text included.
2. A value written by the app is applied as a minimal edit with the caret carried;
   a write equal to the field's text is a no-op; a different value during IME
   composition is held, then applied.
3. Undo is the platform's; a write from the app clears it.

Not given by Exact:

1. No persistence of a draft across launches (the queue notes "durable account/prompt drafts"
   as an Interview-branch feature, not in Exact) [src, up `QUEUE.md:1022`].
2. No merge, no remote-update event, no change as an edit list, no revision notion.
3. No selection kept across a merge beyond the carried offsets (a merge that
   changes text before the caret shifts it by the edit's length through `carrySelection`)
   [src, inferred].
4. No rich editing on Android.

### 3.3 The simplest correct design [inferred, unrun]

Principle: there is exactly one live text, the field's, and exactly one stored
record, written as one pair. The "four versions" become: head (server), base (a
server text the draft descends from), draft (the field), and the merge (a
function result that is immediately written into the field and into base, never
kept).

1. Seed. On opening, read the kept record for the document, if any:
   `{base: <text of the server revision it descends from>, baseRev: <its id>, draft: <the field's text>}`.
   If one exists, the field is seeded with `draft`, not with the head. Else with
   the head's text, with `base = head.text`.
2. Every `input(value)`: set `edDraft = value` and write the pair `{base, baseRev,
   draft: value}` as one durable record (one file, one write). No other field.
3. Save (after the pause, on Done, on leaving, on background): `POST` the draft
   with `parent = baseRev`. On success, `base = draft`, `baseRev = new id`, rewrite
   the pair (the draft in the record equals the field; if the field changed during
   the post, the next save carries it, as today). On 409, read the head and go to 4.
4. Merge (also on opening when the head text is not `base`): `merged = merge3(base,
   draft, head.text)`. If the same lines changed both ways, keep the pair untouched,
   show a conflict state, post nothing. Otherwise: write `merged` to the field
   (`edDraft = merged`; Exact applies it as a minimal edit with the caret carried),
   set `base = head.text`, `baseRev = head.id`, rewrite the pair, then save.
5. The pair is the invariant: the record never holds a revision id without the text of
   that revision, so a relaunch can always merge with the right common ground.
   A record with no known base (typed when the head could not be read) is a kept
   conflict, as the lane already rules (I2).

Under the operating contract's rule of 2026-11-20 ("A page moved to Exact carries
nothing over from the Swift build"), the design reads no old Swift file: the lane's
`textureOldPending` native calls and the "old build's unsent edit" case of round one
are not needed, and `AutoradioModule+TextureEdit.swift` shrinks to the
background-notification touch at most [inferred from CLAUDE.md].

Why this is simpler: no `ref` versus `base` distinction, no "field lags the head"
case (the merge result is written to the field in the same step), no unsaved text
held anywhere but the field and the record, and the record is overwritten, never
accumulated, except a conflict which is kept apart.

### 3.4 Which of the three reviews' findings disappear

Reasoned against Exact's behavior in 1.7; none of this was run.

- Round one, "a save could post over a newer head": not removed by Exact. It is
  removed by sending `parent_revision_id` and honouring 409 (a server rule), as before.
- Round two, "a merged save never reached the field, so the next keystroke reverted
  the other writer's change": removed by design, because merging writes `value`, and
  Exact applies a write as a minimal edit with the caret carried, so the field and
  `edDraft` cannot disagree after a merge [src for the write path; inferred for the design].
- Round two, "text with no recorded base was posted over a newer head": not removed
  by Exact; removed by the pair record (no id without text) plus the conflict rule.
- Round three, finding 1, "after a merge the kept record holds the revision id from
  before the merge, not the field's text": removed by the pair record (the record is
  rewritten at the merge step, as one write of both halves) [inferred]. Exact does not
  help here; the structure does.
- Round three, finding 2, "reopening seeds the field from the head, not from the kept
  text, and typing overwrites it": removed by step 1 of the design, with Exact's field
  taking `value` and then reporting every later `input` [inferred]. Again ours.
- "Typing while a save is in flight is dropped": already handled in the Swift editor
  (failure 5) and in the lane; in this design the field is the draft, so there is
  nothing to drop [inferred].

So leaning on Exact removes the merge-to-field class (round two) and shrinks the state
the code must name; it does not remove the persistence findings, which need the pair
record and a test per transition, the same advice the problem record gave
(`docs/problems/texture-editor-in-exact-three-reviews-no-landing-2026-11-20.md`,
"Belief").

Costs to say aloud: a remote merge clears the person's undo stack on iOS and the web
(1.7); an edit offline needs the head text as base, which we must keep in the record
(it can be large for a long document) [inferred]; and Android cannot do rich editing,
so on Android the same state machine would sit behind a plain-source textarea and a
`text markup="markdown"` preview [src for the host limits].

## 4. Moving the pin from `de212e671` to upstream

I cannot build, so this is a reading of diffs, not a test.

### 4.1 What is documented

- Exact has no changelog, no migration guide, no "update notes" file. The `update/`
  crate is over-the-air delivery of apps, not release notes [src, `update/`; `docs/`
  listing]. What changed is only in `git log`, `QUEUE.md`, and the doc files themselves
  (`docs/contract-grammar.md` +265/-?, `docs/contract-for-agents.md` ~918 lines changed,
  `docs/reference.md` ~1,561 lines changed) [src, `git diff --stat HEAD origin/HEAD`].

### 4.2 What touches `mobile/exact-package` and `mobile/AutoradioLab`

- `ExactPackage` (Swift API our module imports, `host/apple/modules/ExactPackage.swift`):
  92 lines changed, all additive or doc, with two signature changes:
  `ExactVideoSource.configure(stopped:_:)` (the old form kept as an extension),
  and `ExactTrackEntry.picture` becoming a property with an observer. We use neither;
  our module uses `ExactPackageContext`, `audioSession.claim(mode:options:background:events:)`,
  `ExactSubscription`, `ExactPackageView`, `ExactPackagePlatformView`,
  `ExactPackageEvents`, `ExactPackageRefusal` [src; the claim signature is not in the diff].
- Contract grammar (read as a diff of the grammar doc): among removed lines, the
  statement "Only the root may own tasks" was rewritten as "Tasks are root-owned";
  "A resource over a list takes no `when`"; a rewritten `replace(router, ...)`
  paragraph for a stack's first visit; a rewritten line on a refused throw in an
  action. I did not read the whole 265-line diff against our contracts; each of these
  could change compile results for some of our files (we use `task after(2000)`
  per `texture-edit.ts` header) [src for the diff; effect [unrun]].
- The Apple `ExactKit` module lost many files (`Activities.swift`, `Collection.swift`,
  `BoxFilter.swift`, `AnimatedRaster*.swift`, `AtomIcons.swift` and more in
  `git diff --stat`), because upstream split the Apple build into products linked
  "by use" (`567d7a64a`, `a4c46784a`, `85c018aea`, `838193c5f`; `host/apple/Package.swift`
  +135/-?, `build.mjs` and `compose.mjs` ~440 lines) [src]. Patch 0001's files are
  not among them.
- `package.json` devDependencies: only `playwright-core` 1.64.1 to 1.65.1 [src].
  `rust-toolchain.toml` is unchanged [src, empty diff]; the setup script already
  notes the pinned toolchain lacks `aarch64-apple-ios-sim` and Hermes problems at the
  pin [src, `mobile/scripts/setup-exact.sh:60`].
- Our scripts that read Exact: `mobile/scripts/setup-exact.sh` (pin file
  `mobile/exact.pin`, 40-character sha), `sync-exact-package.sh`, `build-lab.sh`,
  `check-exact-pin.sh` [src, ours]. The sync script copies `AutoradioKit` into
  `Kit/*.swift` inside the package; whether the new link-by-use composition changes
  what `exact.json` must declare is [unrun].

### 4.3 Is it a prerequisite for the editor?

No. The editor's Apple and Rust sources are unchanged except one Rust fix, and the
atoms and `format`/`select` surface are at the pin. A pin move would give: the
`71594ac2c` delimiter fix; the painted-host textarea speed-ups (Android's plain
editor); the textflow break refinements. It would not give: hidden syntax on Apple,
a rich Android editor, any editor state API.

### 4.4 Risk, honestly

Medium-high, unproven. Reasons: 760 commits; a restructured Apple build; a changed
grammar with some tightened rules; our app is large (about 70 contract files) with
our own native package and one carried patch; and the setup note shows the pin
already needs care on this machine's toolchain. The patch applies (1.6). The
package API change is additive in what we use [src]. The compile and the first
launch are the real tests; the lead's one Mac build (CLAUDE.md "one Mac build at a
time") is the only place to learn it. If done, do it as its own mission, apart from
the editor work, with `exact.pin`, the patch check, `contract build`, `bun test` in
`mobile/AutoradioLab`, a device build, and the simulator proof as the checklist
[inferred].

## 5. Recommendation

1. Keep the pin; do not move it for the editor.
2. Keep patch 0001 and prove it once on the simulator (it has never been compiled).
3. Build the Texture editor in shared code on the field-is-the-draft design of
   section 3.3, with the single pair record and a written transition table, tests first.
   Use `format`, `select`, atoms as already wired.
4. Decide the Android story before shared-code editing is called finished: stock Exact
   offers plain source plus a Markdown preview there, not a rich editor.
5. Treat "pretext/reflow/textflow" as reader work, separate from the editor: textflow
   is a kernel feature (`wrap-flow`, `shape-outside`, `text-wrap="balance"`, `measure`)
   that needs no pin move, and the reflow demo's Rust data crate is a pattern, not a
   dependency.
