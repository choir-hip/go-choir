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
- **The bake runs data answers with no native module and no account, and an answer that READS `exactHost()` is left unbaked BECAUSE THE CALL THROWS AT THE BAKE** (`js/src/prelude.js`: `exactHost() is unavailable during bake`; `docs/reference.md`: "the bake leaves its answers for the host"; `js/tests/it/inputs.rs` asserts `initial.len == 0`). The throw is the signal: it must propagate. Our gate (`app.ts` `answer`) calls `markNotBaked()` (`api.ts`), a bare `exactHost()` with no try, first in every account-gated answer. The first attempt wrapped the call in a try (`hostKind`, which tolerates a missing function) and so swallowed the signal: the gated answer was evaluated at bake, hit the closed boundary and failed the bake twice (`Data { resource: "preqd" }: Unavailable("the account boundary is closed (locked)")`, 2026-11-21; an `else empty()` on the resource does not help, a bake-time `Unavailable` is fatal). Never wrap `exactHost()` in a try in an answer. `hostKind` (tolerant) tells a phone (`native`) from a host with no native module for logic that must not throw (`wipeFact`).
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

## Kept answers, what a derive sees at the first frame, and `transient` (2026-11-21, account partition; read from the checkout at the pin, not run on a device)

- **What is kept.** A resource whose source read device state shows the last answer the runner kept for it, asked with the same identifying arguments, before the data module is ready (`docs/contract-for-agents.md:767-791`; the runner side is `runner/src/runner/kept_wait.rs:1-9`: a kept entry is admitted when its identifying arguments are known and equal the kept ones, consumed on a known non-empty mismatch, and waits while every differing argument is empty, `none` or reads a resource still on its fallback). `Admission::kept_seed` (`kept_wait.rs:46-52`) is "it shows a kept seed": the resource's value at the first frame IS the kept answer.
- **What a derive sees.** A `derive` reads resources as they stand, so before the first live answer it reads the KEPT answer of a resource, not its placeholder. Our card `acs` is such a resource (its source reads the Keychain through native), its identifying argument (the tick string) can be equal at two launches, and `derive acctHandle = acs.handle` would therefore hand the PREVIOUS launch's handle to every gated resource at the first frame: another account's kept answers, shown. The fix is `transient` on `acs` and on `au`, which its tick reads (`contract/lower/src/lib.rs:665-666`, `set_resource_transient`; `docs/contract-for-agents.md:788-791`: "never kept, a launch starts from its placeholder"). The handle is then `""` at the first frame, and a gated resource whose identifying argument is `""` is waiting, not mismatched (`kept_wait.rs:3-5`): its kept answer shows only after the handle is known and equals the one it was kept for.
- **What the app does** (account-world.test.ts, "I7"): every resource of every contract has `acctHandle` as a parsed argument, or ends its line with `transient`, or is on a short written list (the display's size, the page's visibility, the voice models' state, the host fact, the build rows). The native resources that hold account content (`choirAuth`, `choirPlayer`, `radioView`, `conversationEntries`, `voiceProfile`, `voiceCards`, `takesRecorder`, `podcastNow`, `textureEditInfo`, `pageModel`, `settingsFacts`, `radioNote`, `deckState`, `barFacts`, `readerFollow`, `readerAim`, `importedFollow`, `retrievalAsk`, `pageRequest`) are `transient`: a contract change only, nothing native reads an argument.

## Storage error codes (2026-11-21; read from the checkout at the pin)

`js/src/storage.js:36` says the storage layer maps the host's errors to a few names (`ENOENT`, `EEXIST`, `ENOTDIR`, `EISDIR`, `ENOTEMPTY`, `EBUSY`, `EACCES`); `:51-52` and `:62` map the numeric codes 2 and 3 (and 2 on the other table) to `ENOENT`, 13 to `EACCES`, 16 to `EBUSY`. A missing file is `ENOENT` by that table, but a host can answer another code (`failed`) for the same absence, so code that must tell "absent" from "unreadable" (the sweep's `stat` of a leftover) treats `ENOENT` as absent and any other code as unreadable. The ending mark is no longer read by shared code at all: native reads it (the file, `AccountWipe.Readiness.readMark`), see the design note's "THE LAUNCH VERDICT".
