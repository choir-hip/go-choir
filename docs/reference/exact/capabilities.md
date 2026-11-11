# Exact: on-demand capabilities (GPU module, Canvas 2D, textflow, Markdown, game add-on, native modules)

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**,
**[src]**, **[doc]**, **[test]**, **[inferred]**, **[unrun]** as in the front
page. Paths are relative to the Exact repo root (`mobile/.exact-src` here).
Source of this page: the 2026-11-11 read-only research "Does Exact render
Contract UI through Metal?" (nothing here was built by us; every sketch is
**[unrun]**). Upstream `main` was 20 commits ahead of the pin at the time of
the research, with no renderer change [doc].

## 1. What "loaded on demand" means

README: "GPU rendering, the Markdown editor, text flow, and browser SQLite are
separate artifacts, loaded on demand. Nothing is a feature flag on a core crate"
(`README.md:124-125`) [src]. The repository table lists the four directories:
"`gpu/`, `canvas/`, `textflow/`, `markdown/` ... Capabilities loaded on demand:
the wgpu canvas, Canvas 2D, text around shapes, and the Markdown editor"
(`README.md:728`) [src]. All four directories exist at the pin and at upstream
`main` [doc, GitHub contents API via the research].

"On demand" is runtime lazy loading, not a build flag we set. The GPU module is
a separate dylib (iOS: `dlopen` after the first painted frame, when a canvas is
on screen), and the Markdown parser loads when a `markup="markdown"` node's
content is new or changed [src]. Textflow is a leaf crate used when the flow
attributes appear [inferred from its crate shape]. The contract for humans says
the same of native modules and GPU: "Native modules and GPU capabilities are
separate optional artifacts; they do not add features to every core build"
(`docs/contract-for-humans.md:1559-1560`) [src].

## 2. The GPU module (`gpu/`, crate `exact-gpu`)

**What it is.** A `Surface` trait over wgpu. "The same Rust renders on Metal and
on the browser's WebGPU" (`docs/reference.md:142`) [src]. WGSL shaders are the one
declaration authority, reflected at build time by `gpu/reflect`
(`exact-gpu-reflect`, naga only) [src]. wgpu is built with features
`["wgsl","metal","vulkan","dx12"]` (`gpu/Cargo.toml:24`) [src], so one crate
targets Metal (Apple), Vulkan (Android, Linux) and WebGPU.

**What Contract writes.** A `canvas` node with a `surface=` name and arguments.
Contract only declares the canvas and its arguments: "it is not a drawing-command
language" (`docs/contract-for-humans.md:1539-1541`) [src]. The canvas is laid out
as an ordinary box. Example (`apps/weatherlight/app.contract:369`, shortened) [src]:

```contract
canvas surface=weather(outlook.cloud, outlook.rain, outlook.day, outlook.hour, outlook.speed, animated) testId="weather-sky" width="100%" height=440 border-radius=24 overflow="hidden" background-color="#183e56"
  text city font-family="serif" font-size=34
```

The children (`text`, `row`) are Contract nodes drawn by the host and, for glass,
captured and composited by the shader (`apps/caltrain/gpu/src/glass.rs:1-10`)
[src]. The `background-color` is the fallback. Pointer and touch reach the
surface: `canvas surface=ink(points) pointerdown=begin ...`
(`docs/contract-grammar.md:1940`) [doc].

### 2.1 Writing one in Rust: the full how-to [unrun]

A drawing surface needs Rust. There is no route to supply WGSL or a drawing API
from the app's TypeScript at runtime: the `gpu` object in `app.json` allows only
`shaderRoots`, `shaderPreludes` and `modules` (`scripts/app.schema.json:94-125`)
[src]. (That no other route exists is [inferred] from the schema.)

1. **Crate layout**, copied from `apps/weatherlight/gpu` [src]:

   ```text
   <app>/app.json
   <app>/app.contract
   <app>/gpu/Cargo.toml
   <app>/gpu/build.rs
   <app>/gpu/shaders/foo.wgsl
   <app>/gpu/src/lib.rs
   ```

   The primary crate is named `<app>-gpu` and found by convention. Extra modules
   are `<app>-gpu-<m>`.
2. **`Cargo.toml`** (`apps/weatherlight/gpu/Cargo.toml`) [src]:

   ```toml
   [lib]
   crate-type = ["cdylib", "rlib"]
   [dependencies]
   exact-gpu = { path = "../../../gpu" }
   [target.'cfg(target_arch = "wasm32")'.dependencies]
   wasm-bindgen = "0.2"   # also wasm-bindgen-futures, web-sys (HtmlCanvasElement)
   [build-dependencies]
   exact-gpu-reflect = { path = "../../../gpu/reflect" }
   ```

   The paths are workspace-relative. The SDK is not published to crates.io
   (`publish = false`), and an app GPU crate is listed in the Exact root
   `Cargo.toml` `members` (`apps/weatherlight/gpu` is) [src]. Our crate must sit
   in an Exact checkout at a pinned sha, or be pathed into one.
3. **`build.rs`** reflects every `.wgsl` in `shaders/`
   (`apps/weatherlight/gpu/build.rs`) [src]:

   ```rust
   let generated = exact_gpu_reflect::generate(&dir).unwrap_or_else(|e| panic!("{e}"));
   // writes generated.rust to $OUT_DIR/shaders.rs
   ```
4. **`src/lib.rs`**: implement `exact_gpu::Surface` (`gpu/src/lib.rs:202`) and
   register it (`apps/weatherlight/gpu/src/lib.rs:208-213`) [src]:

   ```rust
   pub static REGISTRY: Registry = Registry {
       surfaces: &[("weather", 6, weather)],
       shaders: shaders::SHADERS,
   };
   exact_gpu::module!(REGISTRY);
   ```

   The `Surface` trait has, among others, `bind(&mut self, inputs: &[Value],
   at_ms: Option<f64>)` (the canvas arguments as typed values; read numbers with
   `exact_gpu::json::number`), `render(...)`, `frame_ready`, `input` (pointer
   events), `clock(seekable)` (so the test driver can own time), `lifecycle`
   (stop work when hidden) and `render_scale` (`gpu/src/lib.rs:202-331`) [src].
   Read the trait's doc comments before writing one.
5. **Register in `app.json`.** The primary crate needs no entry: the Weatherlight
   `app.json` has no `gpu` key. Extra modules take
   `"gpu": {"modules": {"<m>": ["surfaceName"]}}`; "Module m is the crate
   `<app>-gpu-m`, loaded the first time a canvas of one of its surfaces is on
   screen" (`scripts/app.schema.json:119`) [src].
6. **Use it**: `canvas surface=foo(args...) width=... height=...`.
7. **Build.** iOS: `bun host/apple/build.mjs <app> --ios --run` builds
   `lib<app>_gpu.dylib` per module, signs it and carries it as a framework
   (`host/apple/build.mjs:669-691`, `:924-`) [src]. Android: the `.so` goes in
   `jniLibs` beside the app crate, shaders as assets; `bun scripts/agent-android.mjs
   build <app> --apk` (`host/android/README.md:50-60`) [src]. Android needs the
   NDK, Gradle and the `aarch64-linux-android` Rust target
   (`host/android/README.md:9-15`) [doc]. Whether the module runs on our devices
   is **[unrun]**.

**The example to copy:** `apps/weatherlight/gpu` (one fragment-shader sky,
`shaders/weather.wgsl`) for a mark or a wait field; `apps/caltrain/gpu` for glass
over children and a 3D card stack [src].

**Risks, stated plainly.**

- **The ABI is unversioned.** "The host and module ship together. Before 1.0,
  missing required symbols refuse loading; there is no compatibility shim"
  (`gpu/README.md:3-4`) [src]. A past change was breaking (`gpu_child_view` adds
  the child's `testId`; "All embedders must rebuild against this revision",
  `gpu/README.md:6-9`) [src]. Rebuild host and module from one commit.
- **The crate must live in the Exact workspace** (or be pathed to it), because
  the SDK is unpublished and moves with the pin.
- **Build-time cost is unmeasured.** The only documented figure is for the whole
  Apple host module step: a fresh checkout's first build is "106 -> 79 s on the
  M4" (`host/apple/build.mjs:850-851`) [src]. Nothing is specific to one surface
  crate. Expect extra minutes on a cold build and a cached build after
  [inferred, not measured]. Per Exact rule 5 in `EXACT.md`, state the measured
  added time when we add a crate.
- **Android: "A GPU canvas over children draws nothing"**: Caltrain's sky "is
  blank with its module packaged, where a canvas without children (a game's
  world) draws" (`host/android/README.md:1254-1255`, pointing at `QUEUE.md`)
  [src]. A surface with no Contract children draws; children over it are the gap.
  Design a mark or wait surface with no children, and place text beside it.
- Whether the WGSL path needs the Metal toolchain: the module's `build.rs` mentions
  Metal only in a patched-vendor note and we found no `xcrun metal` requirement
  (naga turns WGSL into MSL at run time) [inferred; no build was run]. The first
  build is the test.

## 3. Canvas 2D (`canvas/`)

`canvas/Cargo.toml:1`: "the recorder, the list format and f64 path geometry"
[src]. An app data module (TypeScript or Rust) records HTML-Canvas-like calls:
`app.ts` exports `draw(surface, args, ctx, frame)` (`js/bake/src/canvas-types.d.ts`,
`apps/canvas-gallery/app.ts:1-17`) [src]. Drawn "by the browser, Core Graphics, or
tiny-skia" (`README.md:613`) [src].

- **Contract:** the same `canvas surface=name(args)` node as in section 2.
- **iOS default is Core Graphics.** Vello (wgpu on Metal) draws a canvas only if
  `exact-canvas-vello` was built and the canvas animates: Canvas2DGpu "dlopens the
  first time a canvas that animates" (`host/apple/build.mjs:1300-1301`); on a GPU
  failure "the host falls back to Core Graphics" (`Canvas2DGpu.swift:50, :281`)
  [src]. That static canvases stay on Core Graphics is [inferred] from the dlopen
  condition.
- **Android and Linux:** the ops replay onto Android's Canvas, or onto vello or
  tiny-skia [src].
- Cost: a Hermes call per frame for an animating `draw`
  (`rendering-and-motion.md` section 2). It is the last ranked choice for the mark.

## 4. Text flow (`textflow/`): text around shapes

**This corrects our earlier claim that Exact has no text-flow-around-an-obstacle
primitive.** It does: `wrap-flow`, `shape-outside` and `shape-margin`, CSS
Exclusions style, set on the obstacle box. README: "Text around shapes with CSS's
`shape-outside` and `wrap-flow`: draggable orbs, a dancer, an editorial layout.
One Rust walker shared by every host" (`README.md:597`, the Textflow row)
[src]; "text around shapes (LLP 1043.000)" is in the paint feature list
(`README.md:645`) [src].

Minimal form (`apps/textflow/app.contract`, the Drag scene, shortened) [src]:

```contract
box width=proseWidth height=bodyHeight position="relative" overflow="hidden"
  text body testId="drag-prose" width=proseWidth height=bodyHeight font-family="serif" white-space="pre-wrap" line-height=1.5
  box testId="orb-1" position="absolute" left=x1 top=y1 width=104 height=104 border-radius=100 wrap-flow="both" shape-outside="circle()" shape-margin=8
```

- The obstacle is a sibling box in the same container as a `text` body. Other
  shapes come from `shape-outside=part.outline` with `clip-path=part.clip`
  (the Orbs and Dancer scenes) [src].
- The Article scene sets a drop cap and a pull quote in a block column, where
  "paragraphs have none [height]: auto-height flow sizes them" (the header of
  `apps/textflow/app.contract`) [src]. This is the closest match to a reader with
  side notes or figures: the text re-flows around a placed box.
- The demo's obstacles are absolutely positioned. For a side note or a figure in a
  reader, the plan is a box with `wrap-flow` and a rectangular shape. Which
  shapes are supported beyond `circle()` and the `clip-path` outlines was not
  verified (LLP 1043.000 is not in the checkout: there is no `llp/` dir)
  [unrun].
- Hosts: the README says one walker is shared by every host; the app has web,
  apple and linux crates and no android crate [src]. Android use is **[unrun]**.
- iOS: `bun host/apple/build.mjs --app textflow --ios --run` (`apps/textflow/README.md`)
  [doc]. We have not run it.
- Loading: `textflow` is a leaf rlib crate (`textflow/Cargo.toml`) with no extra
  toolchain [src].

## 5. Markdown (`markdown/`): reader and editor

Two elements, one attribute (`docs/contract-grammar.md:1394-1399`) [src]:

- `text note.body markup="markdown"` is the **reader**: "headings, lists, quotes,
  code and links painted as one node, on every host". The value stays the
  Markdown source.
- `textarea id="editor" value=draft input=edit markup="markdown"` is the
  **editor**: the platform text view with the syntax dimmed on macOS and iOS,
  live on the web, a plain textarea on Linux.

Taps and links: inline **atoms**, pills that show a label that is not their
source. `atom citation link="cite:"`, `atoms="citation file mention"`,
`atomFaces=faces`, and "a press selects it and then is the node's `atompress`"
(`contract-grammar.md:1462-1483`, `:1518-1530`) [src]. Atoms work on a `text`
reader as well as on the editor:

```contract
text draft markup="markdown" atoms="citation" atompress=pressed
```

(from the `Composer` example, `docs/contract-grammar.md:1479-1483`.)

- **No documented footnote tap.** `format(id, "footnote")` is an editor command
  (`contract-grammar.md:1412`); we found no footnote rendering or tap behaviour
  in the reader [src, by search only]. Citations as atoms is the **candidate
  mechanism** for a numbered citation mark [unrun].
- The heading scale is fixed (1.6/1.4/1.2/1.1/1.0) and there are no per-element
  styling hooks (`markdown/src/pieces.rs:133-142`) [src]. Whether the reader
  carries our typography (a serif, line height) is [unrun]; ordinary text
  attributes apply to the node [inferred].
- Loading: the parser loads on demand (`contract-grammar.md:1390`) [src]. No Rust
  is needed; Contract only. Reference apps: `apps/markdown` (reader) and
  `apps/markdown-stress` (toolbar, selection).

## 6. The game engine add-on (`game/`)

"A game is an Exact2 app with a world inside a `canvas`. Write gameplay in Rust
and menus, HUDs and accessible controls in Contract. The simulation and renderer
share one module, loaded after the app's first pixel" (`game/README.md:3-5`) [src].

- **Optional by construction.** It is a separate Cargo workspace, "absent from
  exact2's root members"; "An app opts in by linking `exact-game` and
  `exact-game-render` into its **GPU module**" (`game/README.md:9-12`) [src].
  Removing it removes the engine.
- **What it has:** a 3D wgpu renderer, Rapier physics, SSAO (`AmbientOcclusion`,
  off by default), `ParticleLook` for fire, smoke and sparks, and `render_scale`
  with ordinary Contract HUD children kept at native resolution
  (`game/README.md:305-312, 463-466`) [src]. Hosts: iOS, macOS, Linux, Windows,
  Android and web (`:422-423, :594`) [doc].
- **Contract side** (`game/README.md:495-504`) [src]:

  ```contract
  component Screen
    resource hud = exactSurface("world") as shape Hud
    view
      canvas surface=world() width="100%" height="100%"
        text `Lit ${hud.lit}`
  ```

- **Cost.** We would write scene and gameplay Rust (`logic/src/lib.rs`), carry the
  engine workspace and a heavy dependency set. Size and build time are not
  measured. For a mark or a wait field a bare `exact-gpu` WGSL surface is far
  lighter [inferred]. Use the add-on only for a scene graph, particles or
  physics. It is a GPU canvas, so it inherits the Android children gap.

## 7. Native modules (hyphenated tags)

"Where an app needs a platform widget, it uses a *native module*: a hyphenated tag
like `exact-map`, backed by Swift, Kotlin or Rust, laid out by the kernel like any
other box" (`README.md:506-508`) [src]. The bake checks the tag against the
`modules` list in `app.json`; "Merely inventing a tag does not create a widget"
(`docs/contract-for-humans.md:1553-1558`) [src].

```contract
exact-map testId="map" flex=1 min-height=0 width="100%" latitude=37.7946 longitude=-122.404 zoomLevel=zoom markers=stores change=pick
```

(`apps/map-demo/app.contract:40`, shortened. The listing in `app.json` is
`"modules": ["photo-editor"]`, `apps/photo-editor/app.json:14-16`.) [src]

This is the mechanism our Swift package views use (`native-modules.md`). A native
module view is a UIView, not Metal, unless the module draws with Metal itself.
It is separate from `hatch="word"`, which hands native code the UIView Exact
built (`README.md:509-512`) [src]. iOS today: yes (MapKit, photo-editor). Our
package views are Swift, so they are iOS-only [inferred].

## 8. What our build needs

- **A real Xcode developer directory.** Exact's iOS build must run with real
  Xcode's developer dir (`DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`).
  In the nix dev shell `xcode-select -p` points at `/nix/store/...apple-sdk-14.4`,
  where `xcrun` finds neither the iphoneos SDK nor `xcodebuild` [run, by the
  research agent].
- **Metal Toolchain: not installed on this Mac** (Xcode 27.1, component
  27A266a, "Status: uninstalled"; `xcrun -sdk macosx metal --version` fails with
  "tool 'metal' not found") [run].
  - **What it affects:** (1) `ExactSvgFilter.metallib`: SVG filter pictures draw on
    Metal instead of Core Image; (2) `libexact_canvas_gpu.dylib`:
    `exact-canvas-vello` becomes real instead of a stub
    (`canvas/vello/build.rs:64-78`), so animating Canvas 2D goes through vello.
    Boxes, text and lists do not change.
  - **How to install:** `xcodebuild -downloadComponent MetalToolchain` (the
    command Exact's own warning prints, `host/apple/build.mjs:155, 872`) [src],
    with real Xcode.app selected. Download size not verified. Then rebuild
    (`bun host/apple/build.mjs <app> --ios --run`); the two warnings go away.
  - **What it does not affect:** the per-app GPU module (section 2) [inferred; see
    the risk note there].
- **Cold-build cost** of any added Rust crate: not measured (section 2.1).

## 9. iOS and Android today

| Capability | iOS | Android |
|---|---|---|
| GPU module (`canvas surface=`) | Loaded by `GpuIOS.swift` onto a CAMetalLayer [src]; ours unrun | Ships as a `.so` (`host/android/README.md:50-60`), Vulkan backend; **a GPU canvas with children draws nothing** (`:1254`) [src]; unrun |
| Canvas 2D | Core Graphics by default; vello once the Metal Toolchain is installed [src] | Replayed onto Android Canvas [src] |
| textflow | Yes per the README (`README.md:645`) [doc]; unrun by us | No android crate in the app [src]; unrun |
| Markdown reader | Yes, all hosts per the grammar [doc]; unrun by us | Same [doc]; unrun |
| Game add-on | Listed host (`game/README.md:594`) [doc] | Listed host [doc]; same children gap |
| Native module (Swift package view) | Yes | No: Swift only [inferred] |
