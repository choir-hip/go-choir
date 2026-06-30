# Handoff: Integrate `@aiden0z/pptx-renderer` in a worktree

## Context

The current `SlidesApp.svelte` PPTX parser is too naive. It extracts only text runs (`<a:t>`) and images (`<a:blip>`) and renders them as a flat list of paragraphs. This loses slide layout, font sizes, text boxes, bullets, shapes, and master-slide styling.

The fix is to replace the custom parser with a dedicated browser-native PPTX renderer.

## Worktree

Create and work in a separate worktree so this does not block `main` while the Texture and maild work lands.

```bash
git worktree add ../go-choir-pptx-renderer -b feat/pptx-renderer
```

## Goal

Integrate `@aiden0z/pptx-renderer` into `frontend/src/lib/SlidesApp.svelte` so that `.pptx` files render with layout fidelity comparable to PowerPoint/Keynote.

## Acceptance criteria

1. `SlidesApp.svelte` uses `@aiden0z/pptx-renderer` for `.pptx` files instead of the custom `parsePptx` function.
2. The slide stage shows rendered HTML/SVG per slide with proper layout, fonts, and positioning.
3. Navigation (next/prev, keyboard, thumbnails, fullscreen) still works.
4. A simple PPTX file (e.g., the Choir deck) renders cleanly on both desktop and mobile.
5. No new dependency issues in the Vite build (`pnpm build` passes).
6. Existing PDF and HTML slide handling remains unchanged.

## Files to touch

- `frontend/src/lib/SlidesApp.svelte` — replace `parsePptx` and the `pptx` slide rendering branch
- `frontend/package.json` — add `@aiden0z/pptx-renderer`
- `frontend/src/lib/SlidesApp.svelte` styles — remove or adjust `.slide-pptx-*` if the renderer emits its own markup
- Optional: `frontend/src/lib/apps/registry.ts` — no change expected, but verify the `slides` app entry still matches

## Suggested implementation sketch

1. Install the dependency:
   ```bash
   cd frontend
   pnpm add @aiden0z/pptx-renderer
   ```

2. In `SlidesApp.svelte`, import the viewer:
   ```ts
   import { PptxViewer } from '@aiden0z/pptx-renderer';
   ```

3. Replace `parsePptx(arrayBuffer)` with something like:
   ```ts
   async function parsePptx(arrayBuffer) {
     const viewer = new PptxViewer();
     const presentation = await viewer.parse(arrayBuffer);
     return presentation.slides.map((slide, index) => ({
       type: 'pptx',
       title: slide.title || `Slide ${index + 1}`,
       render: (container) => viewer.render(slide, container),
       destroy: () => viewer.destroy(),
     }));
   }
   ```
   (Check the actual `pptx-renderer` API; the above is illustrative.)

4. In the slide stage, instead of rendering texts/images manually, use an action or `onMount` to call `slide.render(slideContainer)` for the current slide.

5. Clean up blob URLs and renderer instances on slide change/unmount.

## Verification steps

1. `pnpm build` passes.
2. Open `Choir_Deck_0.pptx` in the Slides app and compare visually with Keynote.
3. Test next/prev, keyboard navigation, fullscreen, and thumbnails.
4. Test a PDF file to confirm non-PPTX paths still work.

## Out of scope

- Do not change PDF handling.
- Do not change HTML slide handling.
- Do not change the file picker or registry.

## Background reading

- Current broken parser: `frontend/src/lib/SlidesApp.svelte:102-181`
- Current broken render: `frontend/src/lib/SlidesApp.svelte:447-455`
- Renderer options considered: `docs/handoff-texture-structured-doc-legacy-surfaces-2026-06-22.md` (adjacent discussion)

## Notes

- This should be done in a worktree because the current `main` session is focused on the Texture structured-document patch and the maild migration fix.
- The Chrome debug browser session is already configured for staging acceptance; do not repurpose it for PPTX development unless you can keep it stable.
