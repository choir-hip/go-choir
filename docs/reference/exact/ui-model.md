# Exact: the UI model

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**
verified by running (simulator), **[src]** read from source, **[doc]** from
Exact's docs, **[seen]** observed by the builders of our app (see EXACT.md,
"Evidence marks"), **[unrun]** reasoned. Doc paths are in the Exact repo; the
canonical references are `docs/contract-for-humans.md` (tutorial order),
`docs/contract-grammar.md` (full grammar) and `docs/contract-for-agents.md`.

## The model in five lines

- A file `app.contract` holds `shape`s (record types), `routes`, and
  `component`s. The first component is the root [doc: contract-for-humans.md:82-113].
- A component has `state` (stored), `derive` (computed), `action` (state writes),
  `resource` (reactive request to a source), `mutation` (explicit request),
  `task` (scheduled action), and a `view` (the tree).
- Layout is CSS: `display:flex`, `gap`, `padding`, `flex=1`, `min-height=0`,
  `position="absolute" inset=0`. A bare node is `display:block`; names are CSS's,
  and the compiler refuses aliases [doc: README.md "Write the web, ship the platform"].
- Unsaid presentation is the platform's: text is the system label colour and
  17 pt on iPhone, a `button` is the system's button, a pushed screen has the
  system Back.
- Events bind actions: `press=increment`, `input=edit`, `submit=add`.
  `press=increment` binds, it does not call, while the view builds.

## A complete component [run: this Contract built and rendered]

From [`mobile/exact-spike/app/app.contract`](../../../mobile/exact-spike/app/app.contract):

```contract
use spikeStatus, spikeMode, SpikeStatus from "@choir/spike"

routes nav
  home "/"

component Spikeapp
  resource st = spikeStatus() as shape SpikeStatus else empty()
  mutation moded as shape SpikeStatus
  action listen
    send moded = spikeMode(mode="listening")
  view
    main testId="root" navigationKey=`${top(nav).id}` navigationBack="back" width="100%" height="100%"
      for e in stack(nav) key=e.id
        column navigationKey=`${e.id}` navigationScroll="content" position="absolute" inset=0 display="flex" flex-direction="column"
          header display="flex" align-items="center"
            text "Exact spike" role="heading" aria-level=1 flex=1
          scroll id="content" flex=1 min-height=0 padding=16
            column gap=12
              text `state: ${st.state}` testId="state"
              button press=listen testId="listen"
                text "Listening (tone + mic)"
```

Running `agent ios tree` printed this node tree (`View`, `Text`, `ScrollView`,
`Control` with `(press)` and `testId`s), 16 nodes, which is what the driver
and any agent test sees [run].

## State and actions

- `state count = 0`; `derive visible = items.filter(i => i.title.includes(query))`
  [doc: contract-for-humans.md:260-324]. Prefer `derive` to a second mutable copy.
- An action reads the state **as it was at its start**; its writes land
  together. After `count = count + 1; let before = count`, `before` is the old
  value (the compiler warns `analyze-read-after-assign`). Compute `let next =
  count + 1` once and use it for both.
- Actions have assignments, `let`, `send`, `refresh`, host commands, calls to
  other actions, `if/else`, option `match`. **No loops, no `return`, no
  `await`.** An action calling another action runs its statements right there,
  in the same commit, and still sees the starting state.
- `task ticker mount` schedules one action; `task hide when toast != ""
  key=toastUntil` waits on state [doc: contract-for-humans.md:1423-1460].

## Sources, resources and mutations

A `resource` re-requests whenever its inputs change; a `mutation` reads as an
`option<T>` (`none` until answered); `send` issues it; `refreshes items then
afterSave` re-asks and then runs an action [doc: contract-for-humans.md:540-620].

`else empty()` gives the placeholder before the first answer or on failure.
`failure(x)`, `pending(x)` read the state of a resource.

Answers are produced by the **data module** `app.ts`
(`const sources: Sources = { name: async (args, store, storage, native) => ... }`),
by a Rust crate, or by a package (our `later`). TypeScript on Hermes gets
`fetch`, `structuredClone`, Streams, `crypto.subtle` (digests, ECDSA P-256),
`TextDecoder`, `Intl.NumberFormat/DateTimeFormat/Collator`, timers inside an
answer, `scheduler.yield()` [doc: reference.md:976-1010]. Storage is
SQLite plus a file store under grants declared in `export const grants`
(`fs.read app:/data`, `sqlite.open app:/data/x.db`, `net.fetch https://host`)
[doc: reference.md:1190-1230]. Our spike's grants (microphone) live there:

```ts
export const grants = `device.microphone purpose.microphone\ndevice.microphone-background purpose.recordingInBackground`;
```

The `purpose.<key>` text comes from `strings/en.json`; the build wrote it into
`NSMicrophoneUsageDescription` [run: `plutil -p Info.plist`].

## Layout and styling

- Flex column/row, `gap`, `padding`, `width="100%"`, `max-width="26.25rem"` for
  a focal column, `env(safe-area-inset-top)`, `light-dark()` [doc: README.md].
- Roles name colours: `color="-exact-secondary-label"`, `font="-exact-footnote"`;
  a brand is an `app.tamber` file and `color="var(--ink)"`
  [doc: docs/recipes/tamber.md, theme.md]. `bun scripts/no-tells.mjs .` lists
  literal colours and sizes the platform would have chosen.
- The browser is the layout oracle; native layout is checked against Chrome
  for authored CSS [doc: README.md].

## Lists, forms, navigation (patterns)

List with rows, search, swipe-delete and long-press menu [doc:
docs/recipes/list-rows.md]:

```contract
list id="notes-list" appearance="auto" listStyle="inset-grouped" flex=1 min-height=0
  section
    for n in shown key=n.id
      row delete=remove(n.id) contextmenu=aim(n.id) contextPopover="menu"
        text n.title
```

Rules the docs state: a list's children are rows; draw no separator and no
press state; keyed `for` is required; a labelled form is a grouped list
(`grouped-form`) [doc: docs/start-here.md:16-35].

Navigation: a `routes` table (`tab home "/"`, nested `item "/item/:id"`),
`nav = push(nav, path("item","42"))`, `back(nav)`, `go(nav, url)`. The stack is
one row per `stack(nav)` entry keyed by entry id, so retained screens keep
their state; `navigationKey` on the root names the top entry,
`navigationBack` the back control's `id` [doc: contract-for-humans.md:1323-1380].
A screen chosen by `if page == ...` has no URL, Back or deep link, and the
build warns `lower-screen-by-state`. Tabs and a split list/detail need no code.
Create/edit is a sheet; a row pushes its detail [doc: start-here.md].

**Tab rule (agent finding, 2026-11-10; recorded as stated):** the iOS host
builds the tab controller only from `tablist` tabs. A route that is not in the
tablist must be pushed, not selected.

**Drag events.** A `pan` event supplies the delta since the last event, not the
total from the press (`docs/contract-grammar.md:1862-1863` [src]); accumulate
into state or decide in `panrelease`, which appends release velocity. See
`rendering-and-motion.md` section 3.

## Buttons, scroll, text and tap targets (our app build)

All items in this section are **[seen]** in our build (2026-11-11): builders
saw them on the simulator and did not read the Exact source for them, so they
are what happened to us, not Exact guarantees.

- **A native `buttonStyle="glass"|"bordered"|"filled"` button cannot carry
  our theme.** Its `text` takes no `font-family` (build error
  `lower-button-style`), so it uses the root font. In one case its label
  colour ignored the theme (black on a dark page), and `filled` tinted system
  blue. Pattern: `button appearance="none"` wrapping one themed face
  component, so font, colour and fill come from the theme.
- **The driver will not tap what is covered.** It refuses a target that the
  floating bar or the keyboard covers (the message says the bar "covers its
  middle"). Put primary buttons above long content, and end a glass layer's
  scroll frame above the bar (our `ToolScroll`), because the driver also
  scrolls a target to the bottom edge of the scroll frame, which is under the
  bar.
- **`padding-bottom` on a scroll's content column was not a reliable end
  inset.** A trailing spacer component worked. Padding was not re-tested
  after that, so do not read this as "padding never works".
- **`scroll id=` is not a tap target.** Give the scroll a `testId` to drive
  it by drag.
- **Taps on a changing page need space.** About 1.2 s between consecutive
  taps; a faster second tap can be dropped.
- **`text` collapses `\n`** unless `white-space="pre-wrap"`.
- **A separate glass node under a text field covers it**
  (`_UIVisualEffectContentView` swallows the tap). Put material, fill, radius
  and shadow on the node that holds the content. A glass bar over a full-bleed
  `scroll` rides above the keyboard with `main viewport-fit="cover"
  interactive-widget="resizes-content"` and the bar a `row position="absolute"`
  at `bottom: calc(env(safe-area-inset-bottom) + 8px)`
  (`rendering-and-motion.md` 8b).

## Embedded files and `iframe` (our app build)

**[seen]** (2026-11-11), lane 5 (Files).

- `iframe src="app:/cache/x.html" sandbox=""` renders an HTML file the app
  owns. There is no `srcdoc`: to show generated HTML, write it to a file under
  `app:/cache/` and point `src` at it.
- An `iframe` on a local `.pdf` is PDFKit's `PDFView`; `#page=N` picks the
  page.
- Media elements are paused with `paused=`, not by claiming the audio
  session (our radio and a media viewer pause each other this way).
- Our Files place brings each file to `app:/cache/files/<hash>/<name>` first
  and every viewer plays from that path.

## Components, props and names (our app build)

**[seen]** (2026-11-11) unless marked.

- **`provide` on the root, `inject` in children** carry resources, states,
  derives and actions down. The page host passes a component only what is
  specific to it (`address`, `title`). Children cannot hold resources, so
  shared resources and actions live in a members file; a members file
  (`members "./x.contract"`) needs at least one member.
- **`use` cycles are refused** (`contract-use-cycle`). Shared components live
  in a file that imports nothing of ours.
- **A prop of type `action` works for a component** (`press: action`).
  Passing a call with arguments as that prop was not tried.
- **`after(ms, action)` exists only in a `task`.** Hold-to-talk is
  `task voiceHold when voiceArmed`.
- **Avoid the names `when` and `from` for fields or props in a contract.**
  They are risky; our builders renamed them (`stamp`, `sender`). That the
  compiler refuses them was not checked.
- **`empty()` loads with status `""`.** "Loading" is `status == ""`. A stale
  resource value can show while the next one loads, so check an address field
  on the answer before drawing it.
- **`width=(wide ? "100%" : "auto")` is accepted.** Builders avoided a
  parenthesised ternary as `text` content and used a `derive`; whether it
  works there is untested.
- **Contract build takes about 2 s and finds nearly everything.** Run it
  before every simulator build.

## Inputs

`input value=query input=search placeholder="Search" type="search"` binds a
text field; `type="range"` is a slider with `aria-valuenow` readable by
accessibility; `select`, date/time and file pickers exist with native
conventions [doc: contract-for-humans.md:1116-1195, contract-grammar.md:1165-1190].
Haptics: `haptic("selection")`, `haptic("success")`. Keyboard handling is in
docs/recipes/keyboard-and-focus.md.

## Custom drawing (the timeline question for the lab)

Motion, 3D transforms (they exist; see `rendering-and-motion.md` section 4),
glass, gradients and the full list of what cannot be done are in
[rendering-and-motion.md](rendering-and-motion.md). This section keeps the
timeline question only.

Three routes, cheapest first [doc unless marked]:

1. **`svg` with `polyline`, `rect`, `path`, `text`** inside a component, with
   CSS `@keyframes`, `pathLength`/`stroke-dashoffset` draw-ins
   (apps/sparkline/app.contract). A trace timeline of a few hundred rects is
   plausible; behaviour at thousands is **[unrun]**.
2. **`canvas surface=ink(points)`** with pointer events (`pointerdown`,
   `pointermove`, `offsetX/Y`) and Canvas 2D recording drawn through Core
   Graphics on iOS (LLP 1056 in Exact's `llp/`; `docs/contract-grammar.md:1924-1945`).
   The GPU replay (Vello on wgpu/Metal) needs the Metal toolchain; without it
   the build says "no Canvas 2D GPU module: canvases draw with Core Graphics"
   [run: that notice appeared in our build log]. A canvas `draw` is `app.ts`
   JavaScript on every frame, so prefer an SVG, a transition or a package view
   for a 60 Hz mark (`rendering-and-motion.md` 8a). Install with
   `xcodebuild -downloadComponent MetalToolchain` if needed.
3. **A package-provided native view** (any `UIView`, even
   `UIHostingController.view`): declare `"views": {"my-timeline": {...}}` in the
   package interface and implement `view(_:id:props:events:)`; see
   `packages/fixtures/swatch` and `native-modules.md` [src; **[unrun]** here].

Exact itself embeds a SwiftUI `Text` for rolling numerals
(`host/apple/Sources/ExactNumerals/Numerals.swift:69-72`) [src], so a
`UIHostingController` inside a native view is an established move. We have not
run it.

Markdown: `text note.body markup="markdown"` is the read-only reader (headings,
lists, quotes, code and links painted as one node); `textarea ... input=edit
markup="markdown"` is the editor. Taps go through inline atom pills and
`atompress` (`atom citation link="cite:"`, `atoms="citation"`); no footnote tap
is documented. Citations as atoms is the candidate mechanism for a Texture
reader [src: `docs/contract-grammar.md:1394-1399, 1462-1483`; **[unrun]**]. See
`capabilities.md` section 5.

Text flow around shapes exists: `wrap-flow="both"`, `shape-outside` and
`shape-margin` on an obstacle box beside a `text` body (`apps/textflow/app.contract`,
`README.md:597`) [src; **[unrun]** by us]. An earlier version of this page and
of the app plan said there was no such primitive; that was wrong. See
`capabilities.md` section 4.

## Theming and accessibility

- Data-bound theming (a theme resource bound to `background-color`, `font-family`,
  `box-shadow`, `transition` strings): binding proven upstream, our bindings
  unrun on a device (`rendering-and-motion.md` section 7). `backgroundMaterial`
  accepts only a conditional of literal names.
- Theming: `app.tamber` design file, role colours, dark mode via the platform;
  `bun exact.mjs tamber gen swift .` writes typed names for native views
  (`ExactTamber(view).color(.surfaceCard)`) [doc: reference.md around 2606].
- Accessibility: roles and `aria-*` map to UIKit traits; `role="heading"
  aria-level=1`, `aria-label`, `aria-checked`, `aria-valuenow`,
  `aria-disabled`. `bun exact.mjs agent ios check` reports clipped text,
  covered controls, small targets and low contrast [doc: ios-driving.md].

## Open questions

- How many `svg` nodes or canvas ops keep 60 fps on an iPhone 14 Pro Max for
  the trace timeline?
- Can `markdown` render Texture pages with our link and highlight needs (atom pills for citations, no per-element styling hooks, fixed heading scale)?
- Do `wrap-flow` and `shape-outside` work with a rectangular box of a side note in a long `list virtualized` row, on iOS?
- Does a package `UIView` that hosts SwiftUI size correctly inside a Contract
  list row (`intrinsicSize`, `events.intrinsicSize(...)`)?
