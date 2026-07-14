# Choir Liquid Material Engine Design v0

**Status:** draft design
**Date:** 2026-05-20
**Audience:** Choir shell/UI agents, MissionGradient supervisors, design reviewers
**Related mission:** historical alternate-computer portfolio docs were pruned
during Campaign Compiler cleanup; keep this document only as the design note
for the Liquid material direction.

## Thesis

Choir should have a custom liquid material system, but it should not adopt a
generic live-DOM liquid-glass library as the product default.

The desired artifact is:

```text
one bounded GPU shell material renderer
-> owned low-resolution desktop material field
-> app/shell surfaces register geometry and state
-> shader draws liquid depth, refraction, tint, bloom, and highlights
-> real DOM text/buttons remain sharp and accessible above it
```

The avoided artifact is:

```text
many per-window liquid libraries
-> DOM screenshots or live page capture
-> private app content in canvas textures
-> fragile Safari behavior
-> high memory use during heavy desktop sessions
```

The deep design choice is:

> Refract an owned material field, not arbitrary live app DOM.

That keeps the effect GPU-native while preserving Choir's privacy, readability,
mobile Safari compatibility, and heavy-session recovery constraints.

## Why Not Directly Use `liquid-dom`

`liquid-dom` is a useful reference for the high-end end of the design space, but
it is not the right product default for Choir:

- it targets WebGPU liquid-glass rendering;
- DOM-backed content depends on experimental HTML-in-Canvas / Canvas Draw
  Element APIs;
- the relevant HTML-in-Canvas path currently depends on Chromium flags or
  origin-trial-style browser support;
- mobile Safari cannot rely on Chrome-only flags;
- Choir's app content is private and should not be captured into generic canvas
  textures.

Other WebGL libraries are better references for compatibility, but they still
often depend on DOM rasterization, `html2canvas`, `html-to-image`, SVG
`foreignObject`, multiple WebGL contexts, or broad page snapshots. Those are
research references, not a default product architecture.

References:

- [AndrewPrifer/liquid-dom](https://github.com/AndrewPrifer/liquid-dom)
- [HTML-in-Canvas explainer](https://html-in-canvas.dev/docs/overview/)
- [naughtyduk/liquidGL](https://github.com/naughtyduk/liquidGL)
- [ybouane/liquidglass](https://github.com/ybouane/liquidglass)

## Product Surfaces

The first version should apply only to shell-level surfaces where material helps
orientation without reducing content clarity:

- Shelf;
- prompt/chiron band;
- Desk button and Desk menu;
- active window titlebar;
- Desktop Overview cards and focus halos;
- app launcher/overview control surfaces;
- candidate/worker status chips if they are already product-visible.

Do not apply liquid material by default to:

- VText document body;
- Trace evidence text, payloads, code, JSON, or inspector details;
- media content stages;
- PDF/EPUB pages;
- terminal content;
- app-owned reader/player bodies where contrast and task clarity matter more.

## Engine Topology

### Renderer Host

Mount one renderer host at the desktop shell layer, conceptually:

```text
Desktop.svelte
  -> LiquidMaterialHost
  -> FloatingWindow / Shelf / Desk / Overview DOM
```

The host owns one GPU context for the shell. It may use:

- WebGL as the default GPU backend;
- WebGPU as an experimental high-end backend after mobile Safari proof exists;
- CSS/solid fallback when GPU support, battery, accessibility, or memory
  pressure says no.

Do not create one GPU context per app or window. Heavy sessions already stress
the desktop; the liquid layer must not multiply resource pressure.

### Surface Registry

Liquid surfaces register geometry and state through a small shell API:

```ts
type LiquidSurface = {
  id: string;
  rect: DOMRectReadOnly;
  zIndex: number;
  radius: number;
  intensity: number;
  tint: [number, number, number, number];
  state: "idle" | "active" | "dragging" | "opening" | "closing" | "busy";
  privacy: "public_shell" | "private_shell" | "redacted";
  motionSeed: number;
};
```

The registry should be derived from real DOM geometry through observers and
explicit shell state:

- `ResizeObserver` for rect changes;
- `IntersectionObserver` or visibility checks for hidden/minimized surfaces;
- desktop/window store updates for active, z-order, drag, restore, overview,
  and suspension state;
- live bus / Trace summaries for Chiron or agent-work material pulses.

### Material Field

The renderer should sample from a low-resolution material field owned by Choir,
not the page:

```text
wallpaper/theme field
+ window silhouette field
+ app identity tint field
+ Chiron/tool-activity waves
+ pointer/drag velocity field
+ optional procedural noise/normal field
```

This field can be cheap:

- downsampled canvas texture such as `256x144`, `384x216`, or adaptive;
- updated only when window positions, theme, wallpaper, or activity events
  change;
- not a screenshot of app contents;
- not persisted;
- not sent to Trace or logs.

### Shader Behavior

The shader draws rounded surface masks and adds the material response:

- signed-distance rounded rectangle mask;
- edge refraction from procedural normals;
- subtle chromatic edge split, bounded and readable;
- specular rim/highlight;
- soft inner/outer shadow;
- material tint and saturation;
- activity wave overlay for Chiron/agent progress;
- drag/restore/opening viscosity from state and pointer velocity.

The DOM surface content remains real DOM layered above the GPU material:

```text
GPU canvas draws material
DOM surface keeps text/icons/buttons/focus rings/accessibility
```

This avoids blurry text, inaccessible canvas controls, and private-content
capture.

## Rendering Tiers

### Tier 0: Solid

Use when:

- GPU is unavailable;
- reduced transparency is requested;
- memory pressure is high;
- battery/thermal policy asks for restraint;
- Playwright/WebKit proof shows regressions.

Output:

- solid/tinted shell materials;
- high contrast;
- no animated liquid;
- same layout and controls.

### Tier 1: CSS Frosted

Use when:

- WebGL is unavailable or disabled;
- CSS materials are enough for the current account/device;
- accessibility prefers reduced effects.

Output:

- `backdrop-filter` / `-webkit-backdrop-filter` where supported;
- translucent fill, borders, shadows, highlights;
- no live DOM rasterization;
- no acceptance dependency on browser-specific CSS quirks.

### Tier 2: WebGL Synthetic Liquid

Default target for the custom engine.

Use when:

- one WebGL context initializes cleanly;
- surfaces are bounded shell surfaces;
- frame time and memory budgets stay within limits;
- mobile Safari proof passes.

Output:

- GPU-drawn liquid material over the owned material field;
- DOM controls/text above;
- capped animation loops and idle short-circuiting;
- explicit performance telemetry.

### Tier 3: WebGPU Experimental

Research only until proven.

Use when:

- desktop Chromium or a proven Safari/WebKit WebGPU path is available;
- it outperforms WebGL on the same surface set;
- it does not rely on HTML-in-Canvas or DOM capture for product proof.

Output:

- same engine semantics as Tier 2;
- backend swap only, not a different product ontology.

## Performance Policy

Hard constraints for any promotable version:

- one GPU context for the shell liquid layer;
- no required DOM screenshots or private app-content capture;
- no full-window or full-page liquid layer by default;
- no liquid surface covering most of the viewport on mobile unless benchmarked;
- no continuous render loop when idle;
- cap mobile animation FPS when background work or battery pressure rises;
- respect `prefers-reduced-motion`;
- provide a solid/reduced-transparency fallback;
- expose product-safe performance evidence without host/global telemetry.

Suggested initial budgets:

```text
mobile Safari:
  one WebGL context
  <= 5 shell liquid surfaces visible
  <= 30 fps during active animation
  idle renders only on dirty state

desktop Chromium:
  one WebGL context
  <= 12 shell liquid surfaces visible
  <= 60 fps during active animation
  adaptive field resolution
```

Budgets are starting beliefs, not truths. The experiment must measure and adjust
them.

## Privacy And Security

The renderer must never treat private app content as texture input.

Forbidden:

- `html2canvas`, `html-to-image`, SVG `foreignObject`, or browser screenshot
  capture as required product proof;
- copying VText, Trace, Terminal, media pages, PDFs, EPUBs, uploads, or private
  Files content into GPU textures;
- persisted liquid preview pixels;
- cross-user material field sharing;
- debug routes that expose host/GPU/global system telemetry to the browser;
- accepting a Chromium-only path as mobile Safari proof.

Allowed:

- coarse shell metadata such as window rectangles, active app identity, app
  tint, public shell state, and Chiron event class;
- redacted/synthetic silhouettes;
- account-local theme/wallpaper color fields;
- aggregate product-safe frame/memory/restore-weight metrics.

## Interaction Taste

Liquid should express system state, not decorate everything.

Target behaviors:

- Desk menu opens like a pane settling into place;
- Shelf has a subtle living material current while idle;
- Chiron events create readable waves above the Shelf without blocking input;
- active window titlebar gains depth; inactive windows flatten;
- dragging a window produces a tiny viscous lag in shell chrome only;
- Overview animates surfaces spatially and uses material depth to clarify
  active/suspended/background state;
- loading/worker/candidate activity appears as calm flow, not spinner noise.

Bad behaviors:

- liquid over body text;
- constant shimmer;
- opacity that hurts contrast;
- material that makes windows feel slippery or unfocused;
- effects that hide the current z-order or app boundaries;
- aesthetic demos that ignore heavy sessions.

## Proof Bar

A valid experiment report must include:

- mobile Safari screenshots/video on `390x844` or real iPhone equivalent;
- desktop Chromium screenshots/video;
- at least one heavy desktop session with many windows;
- reduced motion and reduced transparency screenshots;
- WebGL context count;
- frame timing during open/drag/overview/chiron activity;
- JS heap and GPU/resource proxy where available;
- before/after comparison against CSS-only material;
- fallback behavior when WebGL fails;
- accessibility/readability notes;
- explicit recommendation: abandon, keep experimental, user-selectable, or
  promote after another proof loop.

## MissionGradient Notes

This is an experiment lane, not a platform-default mission by itself.

The mission should not promote liquid material simply because it looks good in
one browser. It should promote only if it proves:

```text
GPU material improves state legibility and perceived quality
without harming mobile Safari, heavy sessions, privacy, readability, or recovery.
```

If that cannot be proven, the useful output is still valuable:

- screenshots;
- perf measurements;
- failure signatures;
- fallback design tokens;
- a recommendation to retain only CSS material or Chiron/process animation.
