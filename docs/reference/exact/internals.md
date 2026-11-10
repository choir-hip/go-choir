# Exact: internals

Part of [EXACT.md](../EXACT.md). Pinned commit `76cd50f9e`. Marks: **[run]**,
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
  `hermes-vanilla-d412d3bd8512-v5`) behind **one door**: `__exact_host` in the
  prelude calls `host_door(ctx, op, a, b, out)` in `js/src/door.rs:6-22`. Ops
  are fetch, storage, native calls, timers, crypto; each is grant-checked.
- Work is split into **turns**: an answer runs in turns with a budget (100 ms
  per native call from TS; `scheduler.yield()` ends a turn and continues in a
  new one) (`js/src/turn.rs`, `turns.rs`; `docs/reference.md:477-478`,
  `:1004`) [src, doc]. `turn.rs:1-6` holds a turn to its budget in the calling
  thread's **CPU time** with a wall-clock backstop, because a wall-clock budget
  once refused a 60 ms turn that read 247 ms under load.
- Answers are **checked against the shape Contract declared** before the view
  sees them. A failing source shows the resource's `else` placeholder.
- Native hosts queue fetch/storage in one FIFO: up to 128 queued requests and
  64 MiB of buffers; reads marked `exactIndependentHttp` run up to six at a
  time; `exactStream` keeps a response open and counts twice its ceiling on
  the budget [doc: reference.md:1050-1075]. Relevant to streaming model replies
  into the lab. Sockets exist (`js/src/socket.rs`; `ws:` needs a grant)
  [src].
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
- Animations: `transition` on `translate/scale/rotate/opacity` and one spring,
  executed by `exact-motion`; wall-clock animations do not follow the
  driver's clock unless `clock +N real`.
- Text: CoreText, native selection and editing; Markdown has its own reader
  and editor modules loaded on demand (`ExactMarkdown`).
- GPU: an optional wgpu/Metal canvas module loaded after first pixel; absent
  (no Metal toolchain at build time), canvases use Core Graphics [run notice].

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

Relayed from Exact's README [doc, relayed; **not** verified by us]: cold
start to interactive first frame 100 ms p50; dev edit-to-present 100 ms p50;
touch one line, rebuild crate 30 s; whole blocking check suite 65 s; web
runtime about 20 KB. Exact says each budget "is tracked on every commit"
(`rules/RULES.md`).

Not measured: cold start on the phone, resident memory, frame timing under a
live-updating timeline, battery, Hermes turn cost per answer. `perf` and
`FrameHealth*.swift` are the instruments [src]; `bun scripts/metrics.mjs
--long` measures binary size of the hello app (`apps/hello/app.contract:1-3`).

## Concurrency model [src]

Main thread: runner, kernel, UIKit, all module entry points. Hermes: its own
engine thread (`js/src/engine.rs`). Package and module work: our queues;
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
