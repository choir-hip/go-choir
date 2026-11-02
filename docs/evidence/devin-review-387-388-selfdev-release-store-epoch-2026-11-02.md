# Devin reviews 387 and 388: self-development releases declare their store epoch (2026-11-02)

Red change on the release manifest and updater apply path (branch selfdev-release-store-epoch). Problem: every non-layered self-development apply was refused after the cutover because the plain release declared no store schema epoch (docs/problems/selfdev-release-declares-no-store-epoch-2026-11-02.md).

**Review 387 (round 1, 8f6b5cec3): send_back.** Blocking-real: the plain release declared the epoch of the materializing process while shipping the freeze-time binary; a planned restart between verify and mint could over-declare and pass both updater fences for an older binary over a newer store. Also: the "none" Security-Effect was wrong for that version; rebase detection was untested on the tape path.

**Review 388 (round 2, 24b9990ec): approve, high confidence.** The epoch is recorded in ReleaseBinaryProvenance at freeze from the same process whose bytes are staged, bound into the bundle digest; the manifest declares it only for reused_running; old or capsule-built bundles stay undeclared and the floor refuses them (fail closed); a mint-time running-binary re-check settles the operation failed on mismatch (a closed state, so no stuck computer). Journal-only settlement skipping the re-check is safe (nothing is minted).

Non-blocking, deferred: (1) a bundle with the new field is undecodable by a pre-change materializer after a downgrade between freeze and settlement (visible, heals on re-update; same as earlier bundle-field additions); (2) a transient running-binary read failure settles failed rather than retrying; (3)-(5) nits (non-Linux path skew, a discarded check in journal-only mode, an end-to-end refusal test).
