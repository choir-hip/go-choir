# VText Compare After Publish UI Problem

Date: 2026-06-05

## Problem

After publishing a VText and then returning to a historical version to compare, the publish panel can remain visible while compare is running or failing. If compare fails, the UI can show only the global error pill instead of a compare panel. The global error pill is also styled with danger text on a danger background, which can make the message unreadable and appear as a blank red bar.

## Evidence

- `publishResult` is set after publish and is not cleared when `handleCompareToDraft()` starts.
- The compare panel renders only for `compareResult`, `mergePreview`, `comparePending`, or `mergePending`; a failed compare with no `compareResult` removes the compare surface.
- `.error-float` uses `background: var(--choir-status-danger)` and `color: var(--choir-status-danger)`.

## Desired Behavior

- Starting compare should dismiss the publish result panel.
- Compare should keep ownership of the document-top panel while pending or failed.
- Failed compare should show an explicit compare failure panel with a retry action.
- Global error messages must remain readable.

## Remaining Error Field

This does not root-cause why the compare model call took roughly 30 seconds or failed on the user's real document. It fixes the product-state handling and visibility failure so the next occurrence is diagnosable from the UI and logs.
