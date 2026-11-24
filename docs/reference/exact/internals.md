# Exact: internals

Part of [EXACT.md](../EXACT.md). Written against `76cd50f9e`, corrected against `de212e671` on
2026-11-18 (`file:line` marks are at `76cd50f9e` unless marked). Marks: **[run]**,
**[src]**, **[doc]**, **[unrun]**. Figures from Exact's own docs stay marked
**[doc, relayed]** and are not promoted to measured.

## Pipeline

```text
app.contract --Contract compiler (syntax, types, analyze, lower)--> plan (tables + bytecode)
app.ts --TypeScript check + bake--> Hermes bytecode (HBC), loaded after first pixel
plan --> runner (VM, keyed instances, events, timers on a seekable clock, data seam)
        --> kernel (columnar node arena, Taffy CSS layout, transactions)
        --> host (apple: C ABI static lib + UIKit/AppKit presenters; CoreText measures text)
```

Crates [doc: docs/reference.md:123-160]: `contract/` (compiler and CLI),
`plan/` (format and validating decoder), `runner/`, `kernel/` (typed columnar
arena, EXWF wire frames, validate-then-apply transactions, changed-geometry
receipts), `motion/` (CSS transitions and one spring on a seekable clock),
`host/apple/`, `host/web/`, `host/linux/`, `js/` (the TypeScript executor),
`vendor/taffy` (Taffy 0.9.2 with two patches). `ls llp` fails at the pinned
commit although README and source comments cite LLP numbers heavily: the
decision records are not in the tree [run]. Docs and source comments are the
spec we can read.

## How app logic executes [src, doc]

- Contract compiles to bytecode that the runner interprets. The first frame is
  **baked**: constant data is evaluated into the plan, so the boot path
  "executes and compiles nothing" and no app JavaScript runs before first pixel
  [doc: README.md].
- `app.ts` runs in a separate Hermes engine ("hermes-lean-sys", pinned bundle
  `hermes-vanilla-d412d3bd8512-v5` at `76cd50f9e`; the set is v6 at `de212e671`,
  with Android arm64 among them, `docs/reference.md:30-34`) behind **one door**: `__exact_host` in the
  prelude calls `host_door(ctx, op, a, b, out)` in `js/src/door.rs:6-22`. Ops
  are fetch, storage, native calls, timers, crypto; each is grant-checked.
- Work is split into **turns**: each turn of an answer (its call, or its
  resumption after a `fetch`, storage step or yield it awaited) has 100 ms of its
  own CPU time, and at most ten budgets (1 s) of wall clock; `scheduler.yield()`
  ends a turn and continues in a new one with a budget of its own. An overrun
  fails the resource, which is asked again at 250 ms, then 1 s
  (`docs/reference.md:2208-2226` at `de212e671`). A *synchronous native
  `call`* has its own 100 ms on the main thread
  (`docs/recipes/native-module.md:72`). Earlier text here ran the two together
  (`js/src/turn.rs`, `turns.rs`) [src, doc]. `turn.rs:1-6` holds a turn to its budget in the calling
  thread's **CPU time** with a wall-clock backstop, because a wall-clock budget
  once refused a 60 ms turn that read 247 ms under load.
- Answers are **checked against the shape Contract declared** before the view
  sees them. A failing source shows the resource's `else` placeholder.
- Native hosts: a resource's plain HTTP requests overlap, up to six at a time,
  replies delivered in the order asked; a mutation's answer hands over one
  request at a time; storage runs in one queue in issue order; resource `GET`s
  keep 128 queued, running or unconsumed requests and 64 MiB of buffers;
  `exactIndependentHttp` reads leave the sequence; `exactStream` keeps a
  response open and counts twice its ceiling on that budget
  (`docs/reference.md:2138-2190, 2482-2510` at `de212e671`). A Hermes module has
  a 64 MiB heap (`:2198`). Relevant to streaming model replies into the lab.
  Sockets exist (`js/src/socket.rs`; `ws:` needs a `net.websocket` grant)
  [src, doc].
- **Nothing is issued after an answer returns** (`docs/reference.md:3005-3037` in the Exact checkout [doc]; used by `pages.ts` since 2026-11-24). Storage an answer starts and does not await finishes behind it
  (run in the one storage queue, in the order issued); a storage call, a `fetch` or a `native.call` made when no answer is in flight is refused and logged, and a write chained with `.then` behind an
  earlier one is issued only when that one lands, which can be after the answer. So "behind the view" means started inside the answer and not awaited, never chained. [unrun on a phone]
- Rust data sources implement `exact_runner::DataSource`; on iOS a replaced
  Rust module runs interpreted (`wasmi`) in development only [doc].
- Swift packages talk to the runner through the same seam: `later` answers
  re-enter as source answers; `changed(topic)` asks again every answer that
  called `native.watch(topic)` [doc: recipes/native-module.md]. (The source
  path was not traced; `js/src/watch.rs` is about interrupts, not topics.)

## Rendering [src, doc]

- The kernel computes boxes; the Apple host owns text measurement through a
  callback into CoreText; UIKit views are created per node. "A hatched node is
  a view of its own and leaves Exact's fast path" (`NativeModule.swift`,
  `ExactNativeModule.swift:~255`), so native views cost more than flat leaves.
- Scrolling lists reuse rows; native control kinds are real UIKit controls
  (system buttons, switches, tab bars, navigation controllers).
- Animations: `transition` on `translate/scale/rotate/opacity` (and more; see
  `rendering-and-motion.md`) and one spring, executed by `exact-motion`; keyframes
  on boxes and SVG are lowered to Core Animation; 3D `rotate` is `CATransform3D`; wall-clock animations do not follow the
  driver's clock unless `clock +N real`.
- Text: CoreText, native selection and editing; Markdown has its own reader
  and editor modules loaded on demand (`ExactMarkdown`).
- GPU: an optional wgpu/Metal canvas module (Vello) loaded after first pixel;
  absent (no Metal toolchain at build time), canvases use Core Graphics [run
  notice]. Contract itself never compiles to Metal; details and sources in
  `rendering-and-motion.md` section 2.

## Numbers

Measured by us [run, simulator, debug "development" build]:

| Quantity | Value |
|---|---|
| Rust host build, cold | 5 min 37 s (`host-dev` profile, optimized) |
| Swift host build | 47.63 s (142 steps) |
| First end-to-end build wall time | about 15 min under `nice -n 10` with another agent compiling |
| `.app` size | 46 MB (binary 42 MB) |
| Dylibs | modules 648 KB, web 784 KB, svg 460 KB, video 268 KB, midi 124 KB, sound 96 KB |
| `target/` | 3.8 GB (Exact checkout) + 3.8 GB (app) |
| Hermes bundles | 525 MB for 5 targets after the aborted `setup` |
| Spike claim call, simulator | 363 ms from request to `PlayAndRecord` |
| Driver tree for the spike | 16 nodes |

Relayed from Exact's README at `de212e671` (`README.md:135-141, 155-156`) [doc,
relayed; **not** verified by us]: cold start to interactive first frame 100 ms
p50; dev edit-to-present 100 ms p50 (a saved Contract or TypeScript edit in the
page in 99 ms on a fresh app, web); touch one line, rebuild crate 30 s; the
blocking gate, warm after a one-crate edit, 30 s (`76cd50f9e` read 65 s for the
whole suite); web runtime about 20 KB. Exact says each budget "is tracked on every commit"
(`rules/RULES.md`).

Not measured: cold start on the phone, resident memory, frame timing under a
live-updating timeline, battery, Hermes turn cost per answer. `perf` and
`FrameHealth*.swift` are the instruments [src]; `bun scripts/metrics.mjs
--long` measures binary size of the hello app (`apps/hello/app.contract:1-3`).

## Concurrency model [src]

Main thread: runner, kernel, UIKit, all module entry points. Hermes: an engine
thread (`js/src/engine.rs`) at `76cd50f9e`; at `de212e671` the manifest's
`typescript.placement` is `main` (the default) or `worker`, and `worker` makes
answers arrive `Later`, outside the action's transaction (`docs/reference.md:4665`;
`docs/agent-pitfalls.md:956-958`). Package and module work: our queues;
replies from any thread are copied and hopped to main
(`NativeModule.swift:55-60`). A stale (destroyed-instance) nonce is dropped
and logged.

## Open questions

- Per-answer Hermes turn cost and the cost of `changed` at 20 Hz.
- Cold start and memory on an iPhone 14 Pro Max with the lab loaded.
- What exactly is in the 42 MB binary, and how much does the development build
  differ from `--release`?
- How large can a `list` or an `svg` get before frame health degrades?
- Where are the LLP decision records (`llp/` is missing from the tree)?
