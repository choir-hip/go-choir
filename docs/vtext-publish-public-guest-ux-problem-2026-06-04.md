# VText Publish And Public Guest UX Problem

Date: 2026-06-04

## Problem

Publishing a VText currently makes the public URL harder to copy than it should be. The publish flow does create a public link panel, but it also opens the public route immediately, which pushes the user toward copying from the browser URL bar instead of from the VText surface.

Signed-out public VText routes also expose the generic public preview VText. A publication link should open the publication itself without showing the default preview document, and the default signed-out preview should explain Choir rather than saying "A note before sign-in."

The VText window geometry is too small for public reading, especially on mobile. Public publication windows and the signed-out default preview should take more of the available screen while preserving the desktop shell.

## Evidence

- `frontend/src/lib/VTextEditor.svelte` calls `openPublishedURL(publishResult)` immediately after `publishVText(...)`.
- The same component has a copy button, but it is secondary and appears after the link has already been opened.
- `frontend/src/lib/Desktop.svelte` seeds `public-preview-vtext` for signed-out users even when a `publicRoutePath` is present.
- `frontend/src/lib/public-preview-data.ts` titles the default VText preview "A note before sign-in."
- The default public preview window is hard-coded to `640x520`, and normal VText compact sizing is not optimized for mobile public reading.

## Desired Product Behavior

- Publishing leaves the user in the VText surface and makes "Copy link" the primary post-publish action.
- If clipboard access is available, the publish flow copies the public URL automatically and reports that clearly.
- Public publication routes do not show the signed-out default preview VText.
- The signed-out default preview explains Choir as a durable VText-centered computer for source-backed writing and publishing.
- Public VText windows use larger geometry on desktop and mobile, with mobile taking nearly all usable screen space.

## Remaining Error Field

- Clipboard writes can fail under browser permission rules, so the explicit copy button and visible link must remain.
- Public-route desktop boot order must avoid duplicate windows without breaking authenticated deep links.
- Geometry should be implemented as launch preference rather than a global VText size change, because normal multi-window authoring still needs smaller windows.
