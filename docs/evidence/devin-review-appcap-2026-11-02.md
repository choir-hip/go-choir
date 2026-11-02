# Devin review 389: self-development package captures (2026-11-02)

Branch selfdev-app-captures (5c8d4b16a, 44060d2ce), landed squashed. Tier 1, one seat (orange; agentcore listing fields and Freeze warnings, visualcapture plan and fixture backend, judge and engineer prompts, Texture embed subset). Verdict: approve, confidence 0.8. Security-Effect none: concur.

Strongest support: the fixture backend can only be armed in-process during a scene, listens on loopback, mints no session and touches no store; bounds are enforced at plan, validation and Freeze; failures become manifest notes; the judge's missing-capture rule only fires when packages_touched is non-empty; Texture embed passes route-only evidence through unchanged.

Non-blocking, deferred as residuals:
1. PatchPaths skips git C-quoted paths, so a package whose only changed files have unusual names escapes packages_touched in the judge listing (Freeze still warns from real paths).
2. Deleting or renaming a package away counts as touching it, so a -next promotion that deletes the original would carry a permanent "uncaptured" signal. Fix before the originals are deleted.
Nits: --package help says at most 2 but extras become notes; degenerate scene states on hand-made manifests; bundlePackagesTouched swallows read errors; --timeout may be a no-op under new headless; CLI slug trim; notes truncation can drop the failed-scene count note.
