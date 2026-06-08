# VText Toolbar Responsive Layout Problem

**Date:** 2026-06-08  
**Component:** `frontend/src/lib/VTextToolbar.svelte`  
**DOM hook:** `data-vtext-toolbar`  
**CSS class:** `.doc-toolbar`

## Problem

The VText toolbar, visually the banner/control bar at the top of a VText
window, breaks at constrained window widths. User-provided screenshots showed
two failure modes:

- toolbar controls overlap when the Choir desktop window is narrow inside a
  wider browser viewport;
- controls wrap into two or three rows even though the toolbar should remain
  one row with invariant height;
- `Latest` appears twice because both the revision-line pill and center state
  label render the same state.

## Cause

The toolbar was styled as a three-column grid keyed to viewport media queries.
That is the wrong responsive axis for Choir desktop windows: a VText window can
be narrow while the browser viewport remains wide. The mobile rule also
explicitly allowed `.version-controls` and `.doc-actions` to wrap, which made
the toolbar height unstable.

The duplicated `Latest` came from separate state fields:

- `revisionLineLabel = "Latest"`;
- `stateLabel = "Latest"`.

## Required Fix

The toolbar should be container-responsive to the VText window, not
viewport-responsive to the browser. It should preserve one row and one stable
height across widths, progressively collapsing labels instead of wrapping.

Expected behavior:

- wide: `v2`, previous, next, `Latest`, `Revise`, `Compare`, `Sources`,
  `Publish v2`;
- medium: suppress duplicate center state and shorten `Publish v2` to
  `Publish` as needed;
- narrow: hide the `Latest` pill if necessary and collapse actions to compact
  affordances such as `R`, `S`, and `P`;
- all widths: no overlap, no horizontal overflow, no vertical overflow, and
  invariant toolbar height.

## Verification Contract

Regression proof should resize the actual VText window, not only the browser
viewport, and assert:

- toolbar height remains invariant;
- visible controls remain in one control band;
- `Latest` appears at most once;
- controls do not overlap;
- toolbar does not overflow horizontally or vertically.
