# Exact: rendering, motion, 3D and drawing

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[src]**
read in source at the pinned commit, **[doc]** stated in Exact's docs only,
**[test]** read in a fixture or test that exists in the tree (not run by us),
**[run]** run by us, **[inferred]** reasoned from reads, **[unrun]** a sketch
nobody has built. Paths are relative to the Exact repo root (`mobile/.exact-src`
here) unless they start with `docs/` of this repo. Source of this page: the
2026-11-10 read-only research "Exact Contract: pipeline, motion, 3D, drawing,
themes" (every sketch in it is unrun, and so is every sketch here).

## 1. Direct answers

1. **Contract does not compile to Metal.** It compiles to a plan (tables plus
   bytecode) that a Rust runner interprets. A Rust kernel lays out boxes, and
   UIKit views and Core Animation layers present them. Metal enters in three
   optional places only: the Canvas 2D replay module (Vello on wgpu/Metal), a
   per-app Rust GPU module (wgpu/Metal surfaces with WGSL shaders), and Metal
   shaders for some SVG filters. Contract has no shader or Metal syntax [src].
2. **3D transforms on boxes exist.** `rotate="y 40deg"`, a third `translate`
   length for z, `perspective` on the parent, `backface-visibility`. Missing:
   `preserve-3d`, `matrix`, `skew`, and animation of the axis, z or perspective
   (section 4). An earlier version of our pages said "no 3D"; that was wrong.
3. **A rotating flat card is native. A rotating solid is not** buildable as a
   nested 3D tree, because children of a 3D box flatten into its plane.

## 2. The pipeline and who draws what on iOS

```text
app.contract --contract/{syntax,types,analyze,lower}--> plan (tables + bytecode)  [doc: docs/reference.md:123-160]
plan --> exact-runner (VM, keyed instances, events, timers)
     --> exact-kernel (columnar node arena, Taffy CSS layout, style rows)
     --> exact-motion (transitions, keyframes, springs, drag timelines)  [src: motion/src/lib.rs:1-48]
     --> host/apple Rust static lib (C ABI, batches of ops) --> Swift ExactKit presenters (UIKit on iOS)
```

| Thing | Drawn by | Source |
|---|---|---|
| Boxes, text, lists, buttons, tabs, sheets, navigation | UIKit views, one per node; text measured by CoreText through a callback; native control kinds are real UIKit controls | `host/apple/Sources/ExactKit/IOS/*` (`PresenterIOS.swift`); `docs/contract-for-agents.md:1649-1741` on glass buttons and bar items [src, doc] |
| `svg` | Core Animation layers (`CAShapeLayer` etc.); CSS animations lowered to `CAKeyframeAnimation` | `host/apple/Sources/ExactKit/SvgScene.swift:1-6, 88-176` [src] |
| SVG masks by luminance, filters | rasterised by `exact-svg-raster` with Metal shaders compiled from `host/apple/metal/SvgFilter.metal`; needs the Metal toolchain at build time | `SvgIsland.swift:1-10`; `host/apple/build.mjs:147-163` [src] |
| `canvas surface=name(args)`, Canvas 2D | `app.ts` exports `draw(surface, args, ctx, frame)`; calls are recorded as an op list and replayed on the main thread into a Core Graphics bitmap used as layer contents | `js/bake/src/canvas-types.d.ts:1-36`; `canvas/src/list.rs`; `host/apple/Sources/ExactSurfaces/Canvas2D.swift:1-12` [src] |
| The same canvas on the GPU | the op list replayed by Vello (wgpu on Metal) into an IOSurface, used only when the dylib is bundled and the canvas asks for the next frame | `Canvas2DGpu.swift:1-12, 36-48`; `canvas/vello/Cargo.toml:1-6`; `EXACT_CANVAS_GPU` [src] |
| App GPU module | a Rust crate `<app>-gpu` implementing `exact_gpu::Surface`, loaded after first paint as a dylib or framework | `gpu/src/lib.rs:1-25`; `scripts/app.schema.json:94-126`; `host/apple/build.mjs:686-691, 961`; `gpu/src/acquire.rs:138-159` [src] |
| A hyphenated native tag (`tetra-mark`) | any `UIView` from a package or the app module | `host/apple/modules/ExactNativeModule.swift:58-61` (`typealias ExactNativeView = UIView`); `docs/recipes/native-views.md:1-20` [src, doc] |

Our build log prints "no Canvas 2D GPU module: canvases draw with Core
Graphics" when the Metal toolchain is absent (`host/apple/build.mjs:872` [src];
the notice appeared in our build [run]). That absence affects Vello canvases and
SVG filters. **A package view or a Core Animation animation needs no Metal
toolchain** [inferred].

Where code runs: Contract expressions, derives, actions and tasks run in the
runner VM, not in Hermes. "App JavaScript executed before first pixel: none"
(`README.md` budgets table) [doc]. `app.ts` runs on a separate Hermes thread
(`internals.md`), so a canvas `draw` is JavaScript on every frame it runs.

The GPU canvas can host Contract children, captured by the host and
composited by the shader (`apps/caltrain/gpu/src/glass.rs:1-10`;
`apps/weatherlight/app.contract:369` puts text children inside a `canvas
surface=weather(...)`) [src]. Caltrain ships three surfaces (`aurora`, `glass`,
`stack`; `apps/caltrain/gpu/src/lib.rs:44-51`).

## 3. Motion catalogue

Source of truth: `docs/recipes/motion.md:1-64`,
`docs/contract-for-humans.md:1448-1525`, `motion/src/lib.rs:1-48`.

| Feature | Syntax | Where it runs on iOS |
|---|---|---|
| Transition | `transition="opacity 200ms ease, translate -exact-spring(240, 26, 1)"` | `exact-motion` evaluates in closed form each display-link frame; the host applies `present` ops to the UIView (`PresenterIOS.swift:~996-1003`). Main thread, no TypeScript [src] |
| Spring | `-exact-spring(stiffness, damping, mass)` inside `transition` only; an `animation` refuses it (`motion.md:50-51`) | same engine; `motion/src/spring.rs`, `uikit.rs` [src] |
| Keyframes | `keyframes rise` with `from opacity=0 translate="0px 8px"`, `50% ...`, `to ...`; `animation="rise 300ms 60ms both"`, `infinite`, per-step `animation-timing-function` | boxes (translate, scale, rotate z, `background-color`) and SVG are lowered to `CAKeyframeAnimation`, so the render server plays them (`SvgScene.swift:88-176`; `host/apple/src/box_motion_tests.rs:95-120` asserts the key paths `transform.translation.x/y`, `transform.rotation.z`, `transform.scale`, `backgroundColor`). The test sets `host.svg.box_motion = true` (~line 97); that it is on in a shipping iOS build is [inferred] |
| Stagger | `` animation=`rise 300ms ${i * 60}ms both` `` (`motion.md:30`) | same |
| Exit animation | `-exact-exit-animation="leave 200ms ease-in both"`; must end; the node leaves layout at once and is inert; not inside a screen being popped (`motion.md:46-49`) | host |
| Layout animation | `-exact-layout-transition="320ms ease"` on rows that reorder or resize; children jump to place, unscaled | engine `layout` property (`motion/src/property.rs:48-53`) |
| Shared element | `sharedElement="name"` on two nodes; flies when one commit destroys the holder and creates another (`motion.md:55`; `apps/shared-elements/app.contract:1-20`) | iOS live flight (`presentFlight`). A pushed route never flies by `sharedElement` because the card stays mounted (`docs/agent-pitfalls.md:427-433`). The platform push is UIKit's, with an iOS 18 zoom via `navigationSource=` on the pushed column (same entry) |
| Numeric `height` | `height=(open ? 120 : 0)` with `transition="height 200ms"` | engine. `width` and `top` are refused and the message names `-exact-layout-transition` (`motion.md:40-42`; `docs/contract-for-agents.md:4313-4318`) |
| SVG geometry | `cx cy r x y rx ry stroke-dashoffset` animate; `d` (path morph) in `transition` only, not keyframes (`motion/src/property.rs:38-44, 118-125`) | engine + Core Animation; apps `sparkline`, `svg-gallery` |
| Drag-driven timeline | source `-exact-drag-timeline="--fling x"` on the dragged node; consumers `animation="deckTilt 1s linear both" animation-timeline="--fling" animation-range="-300px 300px"`; wrapper `timeline-scope="--fling"` (`apps/interaction-gallery/app.contract:35-41, 380-384`; `contract/cli/tests/it/timelines.rs:1-60`). Paint rows only; `scroll()` and `%` ranges refused (`timelines.rs:139-150`) | `motion/src/engine/timeline.rs:1-20`: the consumer is held at the local time the presented translate of the source gives, re-sought every frame, so the follower moves in the same frame as the finger with no state commit. iOS wiring: `host/apple/src/style.rs:405`. An iOS XCTest of it was not read [unrun] |
| Transport timeline | `animation=lit(...) animation-timeline=Transport` (`docs/contract-for-agents.md:2040-2067`) | tied to Exact's own `sequence`/`exactTransport()`, not to our `AVAudioEngine`; not usable for syllable sync as it is [inferred] |
| Per-frame task | `task name when cond` with `every(frame, step)` (`docs/contract-for-humans.md:1440`; `docs/recipes/timers.md:72`, `wait=frame`) | runner VM, one action per presented frame, no catch-up. The sanctioned per-frame callback, no JavaScript |
| Gesture physics | `pan=drag`, `panrelease=settle(...)` appends velocity (vx, vy); `transformDragFor`, `transformgeometry`, `transformrelease(x,y,scale,vx,vy,vscale)` (`interaction-gallery:350-372, 383-384`); `heightrelease`; `reorderdrop` | recognisers are native; actions run in the runner |
| Physics | none in Contract beyond the spring timing function. Rapier exists only in the optional `game/` add-on (`game/README.md:1-12`); GPU surfaces hold their own springs (`apps/caltrain/gpu/src/stack.rs:20-30`) | |

Rules:

- **`pan` supplies the move since its last event, not the total.** The first
  event is measured from the press, so the sum of the events is the offset from
  the press. `panrelease` appends release velocity (`docs/contract-grammar.md:1862-1863`
  [src, read by us]; also an agent finding in a lane). A test like `if dy < -60`
  inside one `pan` event is almost always wrong: accumulate into state (Spark
  does, `apps/spark/app.contract:100-105`) or decide in `panrelease`.
- A `pan` hears a drag that starts inside nested buttons, and a tap still
  presses; a nested text input keeps its own drags (`contract-grammar.md:1864-1868`).
- Reduced motion is manual: read `exactViewport().prefersReducedMotion` and
  gate (`motion.md:6-24, 53-54`).
- `backdrop-filter` does not animate (`contract-for-humans.md:1036`).
- Under the test driver the clock is virtual; transitions end on `clock settle`
  and real-time animation needs `clock +N real` (`testing-and-debugging.md`).

## 4. Transforms

**2D.** `translate="x y"` (px or % of the box), `scale=n` (one factor), and
`rotate=deg`, each animatable by itself. A `transform=` on a box is accepted
only as those three functions, once each, in that order, and is lowered to the
three rows (`contract/lower/src/box_transform.rs:1-12, 190-231`;
`docs/contract-for-agents.md:4163`). These are refused as `transform` functions
with "is a 3D function" or "has no individual property": `matrix`, `matrix3d`,
`skew*`, `scaleX/Y/Z`, `scale3d`, `translate3d`, `translateZ`, `rotateX/Y`,
`rotate3d`, `perspective()` (`box_transform.rs:218-231`). SVG elements keep the
whole `transform` property (`contract/lower/src/vocab.rs:104`) [src]. That
refusal text, and the single doc row at `contract-for-agents.md:4163`, is why we
once wrote "no 3D".

**3D, as individual properties** [src]:

- `rotate` takes `<angle>`, `x|y|z <angle>`, or `<n n n> <angle>`
  (`kernel/src/style/space.rs:34-60`). The angle row is separate from a
  `rotate_axis` row.
- `translate` takes a third length for z (`space.rs:147-155, 205-220`).
- `perspective=<px|none>` and `perspective-origin` go on the **parent**
  (`contract/lower/src/style_names.rs:276-278`; `space.rs:166-173`). A
  perspective parent is a containing block (`contract/lower/src/contain.rs:37`).
- `backface-visibility` (`style_names.rs:279`).
- iOS draws it as `CATransform3D` on the box's layer, with the parent's
  `sublayerTransform` as the perspective, and keeps hit testing in line with
  the drawn quad (`host/apple/Sources/ExactKit/SpaceTransform.swift:1-45`;
  `host/apple/tests/ExactKitTests/SpaceHitIOSTests.swift:16-60` [test]).

Working syntax, from the parity page `scripts/fixtures/space.contract:15-33`
[src, read by us]:

```contract
box position="absolute" left=20 top=20 width=100 height=70 perspective=200
  box position="absolute" left=10 top=10 width=80 height=50 rotate="y 40deg"
      background-image="linear-gradient(to right, #e53935, #1e88e5)"
```

Other cells on that page: `rotate="x 50deg"`, `rotate="1 1 0 35deg"` (axis of
three numbers), `translate="0 0 60px"`, `perspective-origin="left top"`,
`rotate="y 150deg" backface-visibility="hidden"`. A turn without a
`perspective` parent is drawn flattened, as CSS does (the page's `flat` cell).

**Limits.** `SpaceTransform.swift:1-5` says sublayers "flatten into its plane,
which is CSS's initial `transform-style: flat`" [src]. There is no
`transform-style` property in `style_names.rs`, so **no `preserve-3d`**: a child
of a rotated box rides the box's plane, and a solid cannot be built as a nested
tree.

**What animates.**

- The `rotate` angle animates through `transition`: the engine's
  `Property::Rotate` is the angle in degrees (`motion/src/property.rs:48-60`;
  `kernel/src/style/space.rs:1-4`: "`rotate` stays the angle row the engine
  animates; its axis is a row of its own"), and the iOS `present` op for
  `rotate` calls `applyTransform`, which reads the axis row
  (`PresenterIOS.swift:~1001`; `SpaceTransform.swift:17-25`). So
  `` rotate=`y ${deg}deg` `` with `transition="rotate 300ms ease"` should spin
  about y [inferred from those two reads; **unrun**].
- The axis, z translation and perspective are not in `Property::ALL`
  (`property.rs`), so they change instantly [src].
- Keyframe `animation` of `rotate` on iOS lowers to `transform.rotation.z`
  (`box_motion_tests.rs:117`; `host/apple/src/svg_lower.rs:306, 867`). No x or
  y rotation key path was found, so a keyframed y-spin is probably drawn as a
  z-spin or ignored [inferred, unrun]. Use `transition` or a frame task for a
  3D angle, not `animation`.

## 5. Custom drawing: what exists

All [src] unless marked.

- **Gradients**: `background-image="linear-gradient(...)"`, radial and conic via
  `kernel/src/gradient.rs` and `host/apple/Sources/ExactKit/Gradient.swift`;
  `mask-image` takes a gradient (`contract-for-agents.md:1439` [doc]).
- **Shadows**: multi-layer `box-shadow`, animatable (`property.rs:101-110`);
  `text-shadow`. Inset and spread: `host/apple/Sources/ExactKit/BoxShadow.swift:1-10`.
- **`filter`** on boxes and images: `blur brightness contrast drop-shadow
  grayscale hue-rotate invert opacity saturate sepia`; web, macOS and iOS draw
  it (`contract-for-agents.md:4238-4247` [doc]).
- **`clip-path`** (`ClipPath.swift:1-14`), `border-radius`, `corner-shape`.
- **Glass**: `backgroundMaterial="glass"` or `"glass-clear"` becomes
  `UIGlassEffect` on iOS 26, else `UIBlurEffect(.systemUltraThinMaterial)`
  (`Backdrop.swift:238-247`). Material names include `ultra-thin, chrome-dark,
  prominent, sidebar, hud-window, glass-clear, glass, thin`
  (`contract/cli/tests/it/styles_rows.rs:426-446`). Text over a blur uses a
  vibrancy view (`IOS/VibrancyIOS.swift:1-17`). `backdrop-filter="blur()
  saturate()"` on iOS is a fixed `.light` system-blur approximation ("iOS has no
  public arbitrary-radius backdrop blur", `Backdrop.swift:19-25`) and does not
  animate. Whether an interactive glass variant can be chosen by attribute: not
  found; `materialEffect(kind, interactive:)` (`Backdrop.swift:238`) is chosen
  by the host.
- **Blend modes**: `mix-blend-mode` is a row, but the iOS SVG path logs "not
  supported on iOS" (`SvgIsland.swift:283`). Treat blend modes as unavailable on
  iOS boxes [inferred beyond the SVG case]. Canvas 2D has
  `globalCompositeOperation`.
- **Canvas 2D**: paths, gradients, shadows, `globalCompositeOperation`, text,
  images, `roundRect`, conic gradients (`js/bake/src/canvas-types.d.ts:4-22`).
  No 3D, no WebGL, no shader hook.
- **Shaders**: only in the Rust GPU module (WGSL, reflected at build:
  `apps/caltrain/gpu/shaders/{aurora,glass,stack}.wgsl`, `gpu/reflect`). Not
  expressible in Contract. No SKSL.
- **Particles, Lottie, Rive**: none found. Particles need a canvas, a GPU
  surface or a native `CAEmitterLayer` view. Path morphs (`d`) have landed in
  `motion/src/path.rs`.
- **Image**: `filter` works on `image`; a `video` element exists
  (`apps/video-player`). SVG files do not draw through `image` on Apple
  (`docs/design/autoradio-app-ui-2026-11-10.md`, Appendix B, citing CFA:1837-1907).
- Contract expressions have `sin`, `cos`, `floor`, `ceil`, `round`, `min`, `max`
  but **no `sqrt`, `acos`, `atan2`** (`docs/contract-grammar.md:1017-1032`).

## 6. Native Swift views from a package

A hyphenated tag listed in `app.json` `modules` (app module) or declared in a
package's `exact.json` `views` becomes a view of any `UIView` subclass
(`docs/recipes/native-views.md:1-50`; `docs/contract-for-agents.md:3175-3183`).
Size comes from `intrinsicSize` (`ExactNativeViews.swift:40`). Props are
strings, resent whole on change. Unknown attributes pass through as props; known
names such as `color` and `value` are refused. A per-frame ticket
`context.frames { f in ... }` follows the session clock
(`ExactNativeModule.swift:131`); the recipe says never use `CADisplayLink`, so
that `clock +N` moves the view. Cost: "a hatched node is a view of its own and
leaves Exact's fast path" (`gaps.md` #7). Details of packaging are in
`native-modules.md`.

## 7. Data-bound styling and themes

- Any style attribute accepts an expression. Upstream examples [src]:
  `background-color=bg` (`apps/canvas-gallery/app.contract`),
  `box-shadow=cell.shadow` (`apps/design-tool/app.contract:1232`),
  `` translate=`${tx}px ${ty}px` rotate=tilt scale=(...) `` and
  `transition=cardTransition(pos, dragging)` where a `fn` returns a transition
  string (`apps/spark/app.contract:228, 50`), string-named tokens
  (`apps/expose/app.contract:646`).
- Colours: "a string from data naming [a token] reads the same colour"; "a value
  computed from state or data names a token only whole"; a colour is any CSS
  colour or `light-dark(a, b)`, parsed once for every host
  (`contract-for-agents.md:4711-4719, 4850-4851` [doc]). A theme resource
  carrying `light-dark(...)` strings is therefore plausible [inferred].
- Our `AutoradioLab` already binds `background-color=th.paper`,
  `font-family=th.fontRead`, `border-radius=th.radiusCard`, `box-shadow=sh`,
  `scale=th.behindScale3` from a package resource (`mobile/AutoradioLab`,
  read by the research on disk, not on this branch).
- **Status: the binding mechanism is proven upstream; our bindings are unrun
  on a device.** What stays unrun: that they paint on the phone, and that the
  runtime parse of composed strings (`transition`, `box-shadow`) is accepted.
  The compiler validates literals and literal choice arms only
  (`contract/.../values.rs:520-570` for `Transitions::parse`;
  `styles_rows.rs:448-459` for materials). Runtime refusal behaviour was not
  read.
- `backgroundMaterial` bound to data is allowed only as a conditional of
  literal names plus `none`, each arm checked at compile time
  (`styles_rows.rs:448-471`). An opaque runtime string such as
  `th.material2` is not shown anywhere and is likely refused [inferred]. Write
  `backgroundMaterial=(th.material2 == "glass" ? "glass" : "none")`. One node
  with that conditional replaces an if/else that duplicates a box, and avoids a
  remount when the theme changes.
- Motion durations: bind them inside the `transition` string,
  `` transition=`opacity ${th.fast}` `` with `th.fast = "120ms ease"`. Spark's
  fn-built string of literals is proven [src]; a theme-built one is unrun.
- Compile-time tokens (`app.tamber`, `setMode`, `setScheme`, `light-dark`,
  `color-mix`) restyle by mode, not by data (`docs/recipes/theme.md`,
  `docs/recipes/tamber.md`, `apps/theme-fixture`). Shipped themes can use them;
  a user theme from JSON uses bound data. CSS custom properties cannot be set
  from state and `var()` is refused in lengths (CFA:3166, 4271, 4718;
  CG:1530-1534 [doc]).

## 8. Recipes

All sketches here are **[unrun]**. Syntax is lifted from the fixtures cited and
adjusted. Check each with `contract build` before relying on it.

### 8a. A 3D mark (the TetraMark)

Ranking, fastest and truest first:

| Rank | Option | Per-frame cost | Notes |
|---|---|---|---|
| 1 | Swift package view: `CATransformLayer` with four `CAShapeLayer` faces and a repeating `CABasicAnimation` | none on the CPU after setup; the render server plays it | True 3D with occlusion and perspective. Costs one hatched view. Change speed by swapping the animation on a state prop. [unrun] |
| 2 | Pure Contract: `sin`/`cos` projection in `fn`s, `svg polygon points=...`, `every(frame, step)` | runner VM; one string diff and one path update per face per frame | No Swift, no TypeScript, themed by `th`. Rotation and the perspective divide need no `sqrt`. Back-face culling by signed 2D area is enough for a convex solid, so no depth sort. [unrun] |
| 3 | Rust GPU surface (wgpu/Metal, WGSL) | GPU | Overkill for a 44 pt mark and needs a Rust crate in the app; the only route to lit or shaded faces. Pattern: `apps/caltrain/gpu/src/stack.rs` |
| 4 | Canvas 2D from `app.ts` | a Hermes call per frame, ops recorded, Core Graphics on the main thread | Works, but spends JavaScript time on a 60 Hz mark and depends on Hermes turn latency, which is unmeasured. Last fallback |
| 5 | Three flat SVG polygons with per-face `rotate`/`scale` transitions | engine | Fake 3D; matches the drawn three-polygon mark |

Pure Contract sketch (option 2):

```contract
fn rotYx(x: number, z: number, a: number): number = x * cos(a) + z * sin(a)
fn rotYz(x: number, z: number, a: number): number = z * cos(a) - x * sin(a)
fn rotX(y: number, z: number, a: number): number = y * cos(a) - z * sin(a)
fn persp(v: number, z: number): number = v * 150 / (150 - z)
fn area(ax: number, ay: number, bx: number, by: number, cx: number, cy: number): number =
  (bx - ax) * (cy - ay) - (by - ay) * (cx - ax)

component TetraMark
  props
    spin: number                      // radians per second, from the state
  state t = 0
  task tick when spin > 0
    every(frame, step)
  action step
    t = performanceNow() / 1000 * spin
  // project four vertices (1,1,1) (1,-1,-1) (-1,1,-1) (-1,-1,1) times 14 into x0..y3, then:
  view
    svg viewBox="-22 -22 44 44" width=44 height=44 testId="mark"
      if area(x0, y0, x1, y1, x2, y2) > 0
        polygon points=`${x0},${y0} ${x1},${y1} ${x2},${y2}` fill=th.accent stroke=th.fg
      // the other three faces likewise
```

Reduced motion: set `spin=0`, which disarms the task. Grow and shrink: `scale` on
the `svg` with `transition="scale -exact-spring(220, 18, 1)"`.

Package view sketch (option 1), pattern of `docs/recipes/native-views.md:5-50`:

```swift
final class TetraView: ExactNativeInstance {
  private let host = UIView()
  private let spinner = CATransformLayer()           // true 3D sublayers, no flattening
  // init: four CAShapeLayer faces added to spinner; host.layer.sublayerTransform.m34 = -1/150
  override func setProps(_ p: [String: String]) throws {
    let spin = CABasicAnimation(keyPath: "transform.rotation.y")
    spin.byValue = 2 * Double.pi; spin.repeatCount = .infinity
    spin.duration = p["state"] == "thinking" ? 1.2 : 6
    spinner.add(spin, forKey: "spin")                // plays on the render server
  }
}
```

Contract side: `tetra-mark state=bi.state testId="mark" width=44 height=44`.

### 8b. Glass bar with depth

Native glass material, a layered shadow, a highlight gradient on the inner edge,
and a small 3D tilt on pull:

```contract
box position="absolute" left=12 right=12 bottom="calc(env(safe-area-inset-bottom) + 8px)" perspective=900
  row testId="bar" min-height=56 border-radius=th.radiusPill
    backgroundMaterial=(th.material2 == "glass" ? "glass" : "none")
    background-color=(th.material2 == "glass" ? "transparent" : th.paper)
    box-shadow=th.shadow2
    rotate=`x ${pullDeg}deg` transition=(still ? "none" : "rotate -exact-spring(220, 24, 1)")
    overflow="hidden"
    box position="absolute" inset=0 border-radius=th.radiusPill pointer-events="none"
      background-image="linear-gradient(180deg, rgba(255,255,255,0.35), rgba(255,255,255,0) 40%)"
```

Limits: `backdrop-filter` is a fixed approximation and does not animate; blend modes
are unsupported; the glass look is the system's, themeable only as `glass` vs
`glass-clear` vs a blur style.

### 8c. Card swipe with throw

Two routes. The safe one is Spark's plain `pan`/`panrelease`: accumulate the
drag into state, tilt with `rotate=tx*0.06`, release with
`transition="translate -exact-spring(260,22,1)"`, decide by distance or
velocity (`apps/spark/app.contract:46-52, 73-160, 222-245`). A 3D lean is cheap
on top: `` rotate=`y ${tx * 0.05}deg` `` with `transition="none"` while dragging.

The declarative route uses the drag timeline so the stamps and the next card
follow the finger with no state commits
(`apps/interaction-gallery/app.contract:345-390`):

```contract
keyframes tilt   from rotate=-15  to rotate=15
keyframes likeIn from opacity=0  to opacity=1
fn verdict(x: number, vx: number, at: number): number =
  (x > at or vx > 800) ? 2 : ((x < 0 - at or vx < -800) ? 1 : 0)   // 120 pt or 800 pt/s

box timeline-scope="--fling" position="relative" overflow="hidden"
  box id="top" translate=`${panX}px ${panY}px`
      transition="translate -exact-spring(300, 30, 1)" -exact-drag-timeline="--fling x"
    box transform-origin="50% 100%" animation="tilt 1s linear both"
        animation-timeline="--fling" animation-range="-300px 300px"
    box transformDragFor="top" transformgeometry=geometry transformrelease=released
        touch-action="none" position="absolute" inset=0
```

There is no free-flight inertia engine beyond the spring plus release velocity.
On a single card (closing a page) keep `swipeleft=closePage(address)`, whose
node follows the finger with a 64 pt knee (`issues/20261022-no-swipeleft.md`
lists iOS as not yet run [doc]), and add a `translate` spring. An iOS run of
the drag timeline was not found [unrun]; Spark is the safer fallback.

### 8d. Page transitions

Between pages we host ourselves (not UIKit routes): a keyed one-child `for`, so
a change is an exit plus an entrance (the idiom appears at
`docs/contract-for-agents.md:2066`):

```contract
keyframes pageIn  from opacity=0 translate="0px 16px"
keyframes pageOut to opacity=0 translate="0px -8px"
for g in [pg.top] key=g
  column position="absolute" inset=0
    animation=(still ? "none" : "pageIn 320ms cubic-bezier(0.16, 1, 0.3, 1) both")
    -exact-exit-animation=(still ? "none" : "pageOut 200ms ease-in both")
```

Card to page flight: `sharedElement` flies only when one commit destroys one
holder and creates the other (`docs/agent-pitfalls.md:427-433`). If the host
page stays mounted, create the header holder in the same commit that hides the
pile, or use real `routes` and the iOS 18 zoom (`navigationSource=`).
Real routes get UIKit's push and swipe-back (`docs/recipes/navigation-stack.md`);
the price is that pages are then routes, not addresses.

**Route and tab rule (agent finding, recorded as stated):** the iOS host builds
its tab controller only from `tablist` tabs. A route that is not in the tablist
must be pushed, not selected.

### 8e. Waiting animation

All compositor-side, no state commit per frame:

```contract
keyframes bob 0% translate="0px 0px" scale=1  50% translate="0px -6px" scale=1.15  100% translate="0px 0px" scale=1
row gap=6 testId="waiting" aria-label="Working"
  for i in range(5) key=i
    box width=8 height=8 border-radius=4 background-color=th.accent
      animation=(still ? "none" : `bob 900ms ease-in-out ${i * 120}ms infinite`)
```

Variations: `stroke-dashoffset` flow on an `svg path` (`apps/sparkline`); a `d`
morph via `transition="d 700ms ease-in-out"` toggled by a task between two path
strings. Real lifecycle lines arriving: `animation="rise 300ms ... both"` per
new row key. Syllable sync with our own audio: drive from state the package
writes (`context.changed`) and use `transition`; Exact's `Transport` timeline
belongs to its own `sequence` [inferred].

## 9. Cannot do, and the fallback

| Cannot | Fallback |
|---|---|
| `preserve-3d` solids as a nested tree | Pure-Contract projection, or a package `CATransformLayer` view |
| `matrix`, `skew` | SVG elements keep full `transform`; or a package view |
| Animate rotation axis, z, perspective | Animate the angle with `transition`, or a frame task |
| Keyframed 3D rotation (probably z only) | `transition` or a frame task |
| Particle systems | `CAEmitterLayer` in a package view; small counts on Canvas 2D |
| Lottie, Rive | SVG keyframes, or a native view |
| Animated backdrop blur | Fade a material layer's opacity instead |
| Animate `width` or `top` | `-exact-layout-transition` |
| `mix-blend-mode` on iOS boxes | Canvas 2D compositing, or a pre-blended asset |
| Lit or shaded 3D faces | Rust GPU surface (a Rust crate; we write no Rust, so avoid) |
| Layer above a UIKit sheet | Authored overlays (design doc, Appendix B) |

## 10. Upstream examples worth grepping

| Path | Shows |
|---|---|
| `apps/spark/app.contract` | Tinder-style deck with `pan`/`panrelease`, spring release, tilt, stamps, undo |
| `apps/interaction-gallery/app.contract:345-390` | drag timeline deck, `transformDragFor`, throw threshold, swipe-to-keep, height sheets, reorder |
| `apps/motion-gallery` | `sweep`, `pulse`, `spin`, `glow`, `tour` keyframes on Core Animation |
| `apps/shared-elements` | every flight form, iOS 18 zoom via `navigationSource` |
| `apps/sparkline` | `polyline points=` bound from data, `stroke-dashoffset` draw-in |
| `apps/svg-gallery`, `apps/canvas-gallery` | every SVG and Canvas 2D feature |
| `apps/caltrain/gpu`, `apps/weatherlight` | GPU surfaces, glass over Contract children, 3D card stack |
| `scripts/fixtures/space.contract`, `space-hits.contract` | the 3D parity pages |
| `apps/messages` | glass buttons, ultra-thin composer, reduced-motion gating (`:278-280, 337, 384`) |
| `apps/metronome` | `animation-timeline=Transport` beat lights |
| `apps/photo-editor`, `apps/map-demo` | native module views |
| `apps/theme-fixture`, `docs/recipes/{theme,tamber}.md` | tokens, modes |
| `game/` | optional 3D engine in a GPU module; far beyond need |

## Open items

- Whether a keyframed `rotate` with a 3D axis lowers to z only on iOS boxes.
- Whether a runtime-composed `transition` or `box-shadow` string from a theme
  resource is accepted on a device.
- iOS execution of `-exact-drag-timeline`.
- Hermes turn latency for a 60 Hz `draw`.
- Whether a Rust app GPU module compiles here without the Metal toolchain
  (the Vello module and SVG filter library need it; the WGSL path reads as
  runtime-compiled; the full iOS build graph was not traced).
