# Lane C2 (op-eds, the gate, the desk's response): failure list, 2026-11-10

Mutation class: green (a list; no runtime change). Rollback: revert.

Binding: `docs/design/autopaper-step0-contracts-2026-11-10.md` sections 3 and 5,
rulings 8.1 and amendments A2 to A7. Written before the code; each row names the
test that is written first.

| # | Way it fails | Rule | Test |
|---|---|---|---|
| X1 | The kill switch or the daily-limit env value survives somewhere (nix, config, registry, spec) | one constant, no setter, no env, no registry entry | repo gates `check-switch-registry`; `TestOpinionDailyLimitIsAConstant` |
| X2 | A reader names a `responds_to` the body made up | the proxy derives `responds_to` and `parents` from the published revision's own provenance (walking the document's own chain), never from a body field | `TestOpedLinksComeFromProvenance` (proxy) |
| X3 | An op-ed on a missing, non-story or mismatched version is written | corpusd refuses 409 before any object is written | `TestPublishOpedRefusesMissingRespondsTo`, `TestPublishOpedRespondsToMustBeAutoMode` |
| X4 | The oped poll returns another owner's op-eds | the route has no owner parameter for a guest; the owner is the bound owner; the proxy overwrites it | `TestOpedsRouteScopedToBoundOwner` |
| X5 | Poll replay mints a second arrival, or a crash between the record and the cursor wedges the poll | commit-if-absent; "already committed" is done; the cursor advances after | `TestOpedIntakeReplayMintsOneArrival`, `TestOpedIntakeCrashBetweenRecordAndCursor` |
| X6 | A poll in flight at a crash is resumed, or a schedule whose tick crashes the computer twice loops | same two-crash rule as a source schedule; a poll is a closed obligation at boot, the next tick is new work | `TestPlatformPollCrashTwiceCloses` |
| X7 | The gate accepts on failure | Jev failure leaves the op-ed pending; no verdict invented; third failure posts `unavailable`, never `accepted` | `TestOpedGateUnavailableStaysPending`, `TestOpedGateThirdFailurePostsUnavailable` |
| X8 | Injection text in the op-ed reaches an instruction slot | op-ed text enters Jev and the desk as `world_untrusted` data only | `TestOpedGatePromptFramesTextAsData` |
| X9 | A verdict is posted for someone else's story | corpusd checks `responds_to` publication owner equals the bound owner (403) | `TestOpedVerdictRouteChecksRespondsToOwner` |
| X10 | A verdict run is written twice | one verdict per (version, run); the later run supersedes | `TestOpedVerdictRunIdempotent` |
| X11 | A rejected op-ed wakes the desk | only `oped_wake` wakes, and only accepted or the reject backoff emits it | `TestRejectedOpedDoesNotWakeDesk` |
| X12 | The reject backoff wakes at once or never | threshold and gap double after a no-change wake, reset after a change | `TestRejectBackoffFold` |
| X13 | Two op-eds arriving while the desk works each start a wake | they join the open wake | `TestOpedCoalescing` |
| X14 | `oped_wake` is not on the waking list, or other silent records wake | the list gains exactly this kind | `TestOpedWakeWakesAndNoOtherOpedRecordDoes` |
| X15 | The desk cannot read the op-ed it was woken for, or cannot read the rejects | the desk's frame carries the accepted op-ed(s) and the story's rejected ones as `world_untrusted` blocks, pinned by manifest hash | `TestDeskFrameCarriesOpedAndRejects` |
| X16 | The desk's revision does not cite the op-ed | the host stamps the op-ed source link into the desk's revision provenance; the proxy turns that into `parents` on publish | `TestDeskRevisionInheritsOpedLink` |
| X17 | A fork of an op-ed notifies nobody, or the author, or a self-fork notifies the author | corpusd writes one notice per (child, parent) to the parent's owner, none for the same owner | `TestNoticeOnForkOfOped`, `TestNoNoticeForOwnParent`, `TestNoticeIdempotentOnReplay` |
| X18 | A notice carries content or an email | the schema has no body field | `TestNoticeHasNoEmail` |
| X19 | A notice poll replay mints twice, or a reader reads another owner's notices | deterministic record id; route scoped to the binding | `TestNoticeIntakeReplay`, `TestNoticesRouteScopedToBoundOwner` |
| X20 | Calibration counts a rejected op-ed the desk used as nothing | a rejected op-ed whose version appears in a later desk revision's links is a false reject; an accepted op-ed whose desk turn changed nothing is a false accept; both are records | `TestCalibrationFromDeskReading` |
