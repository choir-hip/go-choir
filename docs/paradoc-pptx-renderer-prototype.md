# Parallax: Prototype the PPTX Renderer

## Status

Open. Not yet started.

## Mission conjecture

If we prototype a PPTX renderer using the recommended `@aiden0z/pptx-renderer` library against a mock slide-deck object schema, then we will have a proof that the Slides app can render PPTX and a functor design for slide objects.

## Deeper goal

The PPTX renderer is a missing functor from slide objects to a presentation surface. The deeper goal is to prove that the Slides app can be rebuilt as a view over `choir.slide_deck` objects and that the rendering library is viable. This work also validates the app-as-functor pattern for future app rewrites.

## Witness / spec

Deliver a prototype in a Svelte worktree with:

- A mock `choir.slide_deck` object schema (slides, layouts, text boxes, images, shapes).
- Integration of `@aiden0z/pptx-renderer` into a prototype `SlidesApp.svelte`.
- A sample PPTX rendered from the mock object.
- Notes on library limitations, browser compatibility, and bundle size.
- A comparison with any alternative libraries considered.
- A plan for how the real `choir.slide_deck` object will be defined and loaded.

## Invariants / qualities / domain ramp

- Do not integrate into the production Slides app in this prototype.
- Do not change the production package.json unless the prototype proves the library.
- The mock schema must align with the object-graph schema style.
- Keep the prototype runnable in a standalone worktree.

## Authority / bounds

- Green/yellow mutation class: prototype and design document.
- No production code change unless the prototype is approved.
- Branch: `prototype/pptx-renderer`.
- Worktree: `pptx-prototype`.

## Bridge conjecture + sub-conjectures

- Main conjecture: `@aiden0z/pptx-renderer` is the right library for rendering PPTX in the browser.
- Sub-conjecture 1: the library can render a slide deck from a JSON object.
- Sub-conjecture 2: the bundle size and performance are acceptable for the Slides app.
- Sub-conjecture 3: the mock schema can be promoted to a real `choir.slide_deck` object kind.

## Ledger / move log

- Move 0: Read the existing PPTX renderer handoff doc.
- Move 1: Set up a minimal Svelte prototype.
- Move 2: Install and integrate `@aiden0z/pptx-renderer`.
- Move 3: Define the mock slide-deck schema.
- Move 4: Render a sample PPTX.
- Move 5: Document limitations and alternatives.
- Move 6: Commit the prototype.

## Version / lineage

- Predecessor: `@/Users/wiz/go-choir/docs/handoff-pptx-renderer-worktree.md`.
- Successor link: this prototype will inform the `choir.slide_deck` object design and the Slides app rewrite.

## Learning state

- Retained: the library API, rendering pipeline, and mock schema.
- Promoted outward: the renderer decision and slide-deck schema proposal.

## Settlement

Done when the prototype can render a sample PPTX from a mock object and the design notes are committed.
