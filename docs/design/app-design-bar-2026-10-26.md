# App Design Bar

Date: 2026-10-26. Mutation class: **green** (a doc; no runtime change).

The owner called the two apps that self-development rebuilt this weekend, the
Reader (`frontend/src/packages/reader/`) and Slides
(`frontend/src/lib/SlidesApp.svelte`), "horrid". The loop that built them had no
design bar: the briefs were functional specs, verify checked that pages render,
and the approver judged function. This doc is the bar. A self-development
request that rewrites or restyles an app cites it. The verify step produces the
screenshot set in check 1. The approver goes through checks 1 to 12 against
those screenshots and the diff, and writes pass or fail for each one. One fail
blocks approval.

The critique below comes from screenshots of the real shell rendered locally
against a mocked backend, on the three presets at desktop and phone width (see
"How the screenshots were made" at the end).

## The bar

1. **Screenshot set.** Every app change ships screenshots of the app in
   London Salmon (the default, light), Futuristic Noir (dark) and Carbon Fiber
   Kintsugi, in a 1100x750 window and at 390px phone width. Cover each state
   the app has: empty, loading, loaded, error. A missing screenshot is a fail.
2. **Colors come from tokens.** No hex, `rgb()` or `hsl()` literal in app CSS
   or markup; `frontend/tests/theme-contract.spec.js` is the check (it fails
   today with 73 violations in frontend/src and is not in CI). The window body is
   `--choir-surface-app`. Toolbars and sidebars use `--choir-surface-pane`.
   Cards, controls, inputs and wells use `--choir-surface-card`,
   `--choir-surface-control`, `--choir-surface-input` and
   `--choir-surface-inset`. Reading surfaces use `--choir-surface-document`.
   Text uses `--choir-text-primary`, `--choir-text-muted`, `--choir-text-accent`
   and `--choir-text-on-accent`. States use `--choir-state-hover`,
   `--choir-state-selected` and `--choir-state-focus`. Status colors use
   `--choir-status-success|warning|danger` and their `-soft` variants. Lines
   use `--choir-border` and `--choir-border-strong`. `--choir-bg` is the
   desktop wallpaper, so it is never an app background. A color an app
   genuinely needs that no token covers (a PDF page's white, a sepia paper)
   gets added to `frontend/src/lib/theme.ts`; it never goes in as a literal.
3. **Shape and motion come from tokens.** Radii use
   `--choir-radius-control-sm`, `--choir-radius-control`,
   `--choir-radius-panel` and `--choir-radius-pill`. Shadows use
   `--choir-shadow-soft`, `--choir-shadow-floating` and
   `--choir-control-shadow`. Durations use `--choir-duration-fast` and easing
   uses `--choir-ease`. No px radius literals such as 5px, 8px or 11px.
4. **Fonts come from tokens.** Chrome uses `--choir-font-ui`, headings use
   `--choir-font-display`, code uses `--choir-font-mono`. Long-form reading
   text may use the serif reading stack. London Salmon is a serif theme, and
   `app.css` sets its buttons in italic with transparent backgrounds. The app
   must look intentional under that, not merely survive it.
5. **Type scale.** Chrome uses five sizes, the ones Files and Settings already
   use: 0.72rem (labels, the only place uppercase is allowed), 0.78rem
   (metadata and secondary text), 0.85rem (buttons, list rows, body UI), 1rem
   (section titles) and 1.4rem (empty-state heading). Weights are 400, 600 and
   700 only. Content (book text, slides) scales on its own terms.
6. **Spacing scale.** Spacing goes in steps of 4, 8, 12, 16, 24 and 32px. A
   toolbar is one row 40 to 48px tall. List rows are at least 40px. The
   content gutter is 16px.
7. **Hierarchy and chrome budget.** The window titlebar already shows the
   title, so the app does not repeat it in a header. No app logo or monogram,
   no eyebrow labels ("PRESENTATION", "DECK"), no slogan copy. On desktop the
   loaded state has at most one toolbar and one thin status line. The content
   (the page, the slide, the list) fills at least 70% of the window's content
   area at 1100x750; measure it on the screenshot. Each fact appears once: a
   count or a position is not shown in three places.
8. **Controls.** Use plain `<button>`, `<input>` and `<select>`. `app.css`
   restyles them inside `.app-content` with `!important` (background, border,
   shadow, and in Salmon italic text), so app CSS that sets button backgrounds
   or borders is dead code; judge controls only from screenshots. Icon buttons
   are 32 to 36px square and use inline stroke SVG icons drawn like the window
   controls (`.ctrl-glyph` in `FloatingWindow.svelte`: `currentColor`, stroke
   1.5, round caps). They never use Unicode glyphs (⌕ ☰ ⛶ ← →) or emoji. Every
   icon button has `aria-label` and `title`. Text buttons are verbs in
   sentence case. A view has at most one primary action.
9. **States.** The empty state gives one plain sentence on what the app opens,
   the primary action as a real button, and recent items if there are any. It
   carries no headline copy. The loading state names what is loading, inside
   the content area. The error state says what failed in plain words, then the
   detail in muted text, then a Try again button. Controls that do not apply to
   the current state are hidden: no page controls when nothing is open.
10. **Phone (390px).** The layout is one column with no horizontal scroll, and
    no control is clipped (it is visible, or it sits in an overflow menu). The
    primary content gets at least 60% of the window height, and the chrome
    above it is at most 112px tall. Text keeps 16px side gutters, tap targets
    are at least 40px, and floating buttons never sit on top of text.
11. **Contrast in every preset.** Text that people need to read has at least
    4.5:1 contrast against its background in all three presets.
    `--choir-text-subtle` fails everywhere (2.9:1 Salmon, 3.8:1 Noir, 3.6:1
    Kintsugi), so it is for decoration and disabled states only.
    `--choir-text-muted` passes (6.1, 7.6, 7.8). Text on an accent fill uses
    `--choir-text-on-accent`; in Noir that is only 3.0:1, which passes only for
    large text (at least 18.66px bold or 24px regular); smaller text on an
    accent fill needs 4.5:1, so add or adjust a token rather than use it.
12. **Robust layout and reviewable markup.** Grid templates match the children
    actually rendered: a conditional child (an upload bar, a hint line, an
    alert) must not push the content out of the `1fr` track. Use
    `minmax(0, 1fr)` tracks so that wide media cannot widen a column. Toggle
    every conditional element (contents panel, search, upload, error) once in
    the screenshot set. Markup puts one element per line, not a whole view on
    one 1,000-character line.

## Critique: the Reader

What the screenshots show, worst first.

- **The reading controls are invisible on desktop.** `.reader-shell` has
  `grid-template-rows: auto auto minmax(0, 1fr)`, but it renders five to seven
  children (the drop hint was added in first place). The toolbar
  (`[data-reader-tools]`) lands in the `1fr` track. When the content is tall it
  collapses to 0px and the body paints over it (measured: rows
  `16px 60px 0px 599px 29px`). So Previous/Next, chapter, text size, width and
  tone cannot be reached, and clicking a tone does nothing. When the content is
  short, the toolbar takes the whole flexible track instead: a 250 to 400px
  empty band with the controls floating in the middle, and the PDF canvas left
  at its 300x150 default as a blank white box (Noir PDF, every empty state, the
  phone).
- **Book text is set in the muted color.** `app.css` forces
  `.app-content p { color: var(--choir-text-muted) !important }`, so every
  paragraph of the book renders at `#755B56` in Salmon instead of the reader's
  ink `#3A1517`, and the sepia and noir tones cannot change the text color.
  Headings are dark and body text is grey: the hierarchy is inverted for
  reading.
- **A permanent instruction line sits above the title.** "Drop a file anywhere
  in this window, or use Open file…" is always visible as a stray right-aligned
  11.5px line (two lines on phone).
- **Redundant and misaligned text.** "Chapter 1 of 4 – The Long Afternoon"
  sits in its own box, offset from the text column, directly above an
  identical chapter heading, and the footer repeats "Chapter 1 / 4". The
  contents panel lists "11 blocks" per chapter, which is parser vocabulary.
  Search hits all carry the same chapter title and start mid-word ("…r nobody
  had written").
- **Icons and type.** The search icon is the Unicode ⌕ at about 8px, and
  contents is ☰. The type ramp mixes 0.72, 0.74, 0.76, 0.82, 0.84, 0.86 and
  0.98rem with 11.5, 12 and 13px, at weights 650, 700, 720 and 760.
- **The empty state contradicts itself.** With nothing open it shows a "PDF"
  chip, Previous/Next page, a page box reading "1  1" and a zoom select. It
  tells a signed-in user that "Reader keeps your place once you are signed in".
  The recents are bold italic titles over subtle-colored paths (2.9:1).
- **Hardcoded colors.** The tones `#f2e7d3`, `#2f261c`, `#070b12` and so on,
  and the PDF page `#ffffff` (`reader.css` lines 361 and 620–632).
- **The signed-out page (`/read`) is a second, hand-built copy of the UI** in
  `frontend/src/reader.js`. `frontend/reader-chrome.css` pins it to Futuristic
  Noir hex values while the product default is London Salmon. It has two links
  ("Open file" and "Open your own reader") that both go to `/`, and a contents
  button that does nothing. On phone the PDF text touches both screen edges,
  the floating ‹ › buttons sit on top of the text, "1 / 2" wraps vertically,
  and the toolbar wraps to three rows (about 250px of chrome).
- What works: the reading measure and line height are good, the search popover
  is the right shape, and the colors mostly use tokens.

## Critique: Slides

The 10-24 redesign replaced the old hardcoded `#1a1a2e` palette with tokens,
which is progress. It also buried the slide in application chrome.

- **The slide gets 42% of the window.** A 72px header (an "S" monogram, a
  "PRESENTATION" eyebrow, the file name again, "6 slides", Present), a 260px
  navigator (a "DECK / Slides" heading, a count badge, a hint), and a 56px
  player bar leave the slide 762x428 in a 1100x705 content area. The previous
  version gave it about 70%.
- **The phone layout is broken.** In `@media (max-width:760px)` the rows are
  `minmax(110px,30%) 1fr`, but `.presentation` is placed in row 1 and
  `.navigator` in row 2. The slide gets the 30% row and the filmstrip gets the
  rest, leaving half the window blank under three thumbnails. The column is
  `1fr` rather than `minmax(0,1fr)`, so the 2x-rendered page image widens it:
  the slide is cut off on the right and the Previous/Next/fullscreen buttons
  are off screen.
- **Every fact is said several times.** The slide count appears four times
  (header "6 slides", badge "6", "Slide 1 of 6", "1 / 6"), the keyboard hint
  twice, the file name twice, and fullscreen twice (Present and ⛶). Every PDF
  thumbnail is labelled "Slide".
- **Slogan empty state.** "YOUR PRESENTATION SPACE / Make room for the big
  picture." in a 3rem display face, with "CONTINUE WHERE YOU LEFT OFF" over a
  card of recents with → arrows. It has no button to open anything; the copy
  tells the user to go to Files.
- **The wrong background.** `.slides-shell` uses `--choir-bg` (the wallpaper),
  so the app body is darker than the window and than every other app in Noir
  and Kintsugi. In Salmon it gives a two-tone split against the
  `--choir-panel` header.
- **PPTX and HTML slides are not slides.** The frame has no 16:9 aspect: it
  grows to fit its text, so the frame changes size from slide to slide. PPTX
  text runs are bare `<p>`, all at one size and, because of `app.css`, all in
  the muted color, so the title is indistinguishable from the body.
- **Dead styling and lost behavior.** `.control-button.primary` (accent fill,
  `color: var(--choir-bg)`) never shows, because `app.css` overrides button
  backgrounds. Radii are 5, 8, 10, 11 and 12px literals, and the shadow is
  `rgb(0 0 0 / 18%)`. Arrows are Unicode. `handleStageClick` is still defined
  but no longer bound, so clicking the slide no longer advances it. All the
  markup is on eight lines of up to 1,055 characters.
- **The error state** reads "Source fetch failed (404)", and the retry
  control is a bare italic word.

## Shared prerequisite: let content opt out of the chrome overrides

`frontend/src/app.css` restyles `p`, `small`, `a`, `.muted`, `.chip`, `.badge`,
`.active` and `.selected` inside every `.app-content` with `!important`. That
is right for chrome and wrong for content. Add an opt-out attribute, for example
`data-choir-content`, and exclude it from those selectors (for example
`.app-content :is(p, small, …):not([data-choir-content], [data-choir-content] *)`,
which exempts the carrier and its descendants). Then put it on
the Reader's `.reader-sheet` and `.reader-pdf-text` and on Slides'
`.slide-document`. Without it, any app that shows a document renders it in grey.
This is a small shell change and should land before, or with, either app fix.

## Reader: what to change

Paste-ready for a self-development request. Files:
`frontend/src/packages/reader/App.svelte`, `reader.css`,
`frontend/src/reader.js`, `frontend/reader-chrome.css`, `frontend/read.html`.

1. Fix the shell layout. Make `.reader-shell` a flex column
   (`display:flex; flex-direction:column`) with `.reader-body { flex:1 1 auto;
   min-height:0 }`, or name every grid row explicitly. Move
   `.reader-drop-target` out of the flow: the veil is an absolute overlay on
   `.reader-shell`, shown only while a file is dragged over the window.
2. Delete the permanent `.reader-drop-hint` line. Put the drop instruction in
   the empty state's sentence and in the drag veil. Keep an
   `aria-live` region, visually hidden.
3. Merge `.reader-top` and `.reader-tools` into one 44px toolbar on
   `--choir-surface-pane`. Left: contents toggle, then Previous and Next as
   icon buttons (SVG chevrons, aria-labels "Previous page" or "Previous
   chapter"), then the position as a compact field ("3 / 14": a number input
   for PDF, a chapter `<select>` for EPUB). Right: zoom `<select>` (PDF only),
   an "Aa" button that opens a small popover with text size −/+, width
   (Narrow, Comfortable, Wide) and tone, then search and Open file. Do not
   repeat the document title in the toolbar; the titlebar has it. Author and
   word count move to the top of the contents panel.
4. Replace the ☰ and ⌕ glyphs and the A-/A+ labels with inline SVG icons drawn
   like `.ctrl-glyph` (13 to 16px, stroke 1.5, `currentColor`).
5. Reading text: add `data-choir-content` to `.reader-sheet` and
   `.reader-pdf-text` (see the prerequisite) so paragraphs render in
   `--reader-ink`. Remove the `[data-reader-section-count]` line above the
   chapter. Show each chapter's real heading once, at 1.4em, in
   `--choir-font-display`.
6. Tones. "Paper" follows the theme (`--choir-surface-document`,
   `--choir-text-primary`). Define sepia paper, ink and muted as tokens in
   `theme.ts`, then delete the hex literals in `reader.css`. Drop the "noir"
   tone: the theme already gives a dark page. Label the tones in sentence
   case. Give the PDF page a theme token, `--choir-surface-paper`, instead of
   `#ffffff`.
7. Empty state (nothing open, signed in). Hide the format chip, the page
   controls and zoom. Center a 1.4rem heading "Open a document", one muted
   sentence ("PDF or EPUB, from your computer or from Files. You can also drop
   a file here."), an Open file button, and "Read the sample" as a secondary
   text button. Below that, recents in Files-style rows: a document icon, the
   name in `--choir-text-primary` at 0.85rem, and the folder in
   `--choir-text-muted` at 0.78rem. No bold italic, no subtle-colored text.
   Remove the "once you are signed in" clause for signed-in users.
8. Contents panel: list chapter titles only, with no "N blocks". The current
   chapter gets `--choir-state-selected` and a 2px accent bar on the left.
   For a PDF with no outline, hide the contents button instead of showing "All
   pages".
9. Search results: label each hit with chapter and position ("Chapter 2 ·
   ¶14" or "Page 3"). Start snippets at a word boundary and highlight the match
   with `<mark>` in `--choir-state-focus`.
10. Type and spacing: use only 0.72, 0.78, 0.85, 1 and 1.4rem and weights 400,
    600 and 700. Replace the 11.5, 12 and 13px values in `App.svelte`. Use 8px
    gaps in the toolbar and 16px gutters.
11. Phone, 640px container and narrower: the toolbar stays one row. Keep
    contents, Previous/Next, the position and an overflow button; put Aa,
    search, zoom and Open file in the overflow menu. The sheet gets 16px side
    padding. The contents panel becomes a full-height sheet over the text.
12. Signed-out page `/read`: delete the hand-built DOM renderer in
    `frontend/src/reader.js` and mount `App.svelte` with `authenticated=false`
    (it already opens the sample). Drive `reader-chrome.css` from
    `themeCSSVariables(DEFAULT_THEME)` instead of hardcoded Noir hex, so the
    page looks like the default desktop. Keep a single "Sign in to open your
    own files" link. Delete the floating ‹ › buttons, which overlap text on
    phone.
13. Verify: the screenshot set in check 1 for empty, EPUB loaded, PDF loaded,
    contents open, search open, sepia tone and error, plus `/read` at both
    widths. Assert that the toolbar's bounding box has height ≥ 40px and does
    not intersect `.reader-body`.

## Slides: what to change

Paste-ready for a self-development request. File:
`frontend/src/lib/SlidesApp.svelte` (and the shared `app.css` prerequisite).

1. Reformat the markup to one element per line before changing anything else,
   so the diff can be reviewed.
2. Background: `.slides-shell` uses `--choir-surface-app`. The stage area
   uses `--choir-surface-inset`. Fullscreen uses `--choir-surface-media`.
3. Delete the header: the monogram, the "PRESENTATION" eyebrow, the repeated
   title, "N slides" and the Present button. The titlebar carries the file
   name.
4. Loaded layout, desktop. On the left, a 168px filmstrip on
   `--choir-surface-pane` with no heading, badge or hint. Each item is a 16:9
   thumbnail 136px wide with its number in 0.72rem muted text beside it. The
   active item gets a 2px `--choir-accent` outline at
   `--choir-radius-control-sm`. Do not show text labels; drop the "Slide"
   placeholder. The stage holds the slide in a 16:9 frame (`aspect-ratio:
   16/9`), as large as fits with 24px padding. Below it, one 44px bar: the
   position "3 / 6" on the left, Previous and Next SVG icon buttons in the
   center, and a Present button (SVG expand icon plus the label) on the right.
   No keyboard-hint text; put shortcuts in the buttons' `title`. The target is
   the slide filling at least 70% of the content area at 1100x750.
5. Restore click-to-advance: bind `handleStageClick` on the stage (left half
   back, right half forward).
6. PPTX and HTML slides: render inside the same 16:9 frame on
   `--choir-surface-document`. Size text relative to the frame (container
   query units), make the first text run a title at 2.2em in
   `--choir-font-display` and the rest body text at 1.1em, and put
   `data-choir-content` on `.slide-document` so text uses
   `--choir-text-primary`.
7. Empty state. No header, no slogan, no eyebrows. Center a 1.4rem heading
   "Open a presentation", the muted sentence "PDF, PPTX or HTML.", and an "Open
   from Files" button that dispatches `launchapp` for Files. Below that,
   recents as Files-style rows (a file-type SVG icon, the name at 0.85rem, the
   folder or time in muted 0.78rem), with no card and no arrows.
8. Error state: the heading "This presentation could not be opened", a plain
   reason ("The file was not found." for 404, "This file type is not supported
   yet." for an unknown extension), the raw detail in muted 0.78rem, and a Try
   again button.
9. Phone, at the existing `@media (max-width: 760px)` query and narrower
   (a viewport query; `.slides-shell` has no container-type): the stage comes first and takes the
   flexible row, the filmstrip becomes a 72px horizontal strip under it, and
   then the 44px bar comes last. Make every track `minmax(0, 1fr)` so the slide
   image cannot widen the layout. All three controls stay visible at 390px.
10. Tokens: replace the 5, 8, 10, 11 and 12px radii with `--choir-radius-*`,
    the `rgb(0 0 0 / 18%)` shadow with `--choir-shadow-soft`, and the Unicode
    arrows and ⛶ with SVG icons. Delete the `.control-button.primary` styles,
    which `app.css` overrides.
11. Verify: the screenshot set in check 1 for empty, PDF deck, PPTX, HTML deck
    and error, plus fullscreen at desktop width. Assert that the slide frame's
    area is ≥ 70% of `.app-content` at 1100x750 and that all player controls
    are inside the viewport at 390px.

## How the screenshots were made

They were made locally with the Vite dev server and Playwright. Browser routes
answer `/auth/session` as signed in, `/api/shell/bootstrap` with a stable
`computer_id`, `/api/desktop/state` with one restored window per scene (Slides
or Reader with an `app_context.sourceUrl` pointing at a fixture PDF, PPTX,
EPUB or HTML), `/api/preferences/theme` with the preset id, and
`/api/media/recents` with sample recents. This renders the real shell, window
chrome and `app.css` overrides without a backend, so a capsule's verify step
can produce check 1's screenshot set the same way. The previous Slides version
(`3321f633b^`) was captured the same way for comparison. The harness was kept
outside the repo for this review; promoting it to a tracked proof script is a
follow-up.
