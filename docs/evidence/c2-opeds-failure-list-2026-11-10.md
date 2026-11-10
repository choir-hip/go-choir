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

## Review repairs (Sonnet seat, REPAIR, 2026-11-10)

| # | Way it fails | Rule | Test |
|---|---|---|---|
| R1 | A republished fork re-notifies, re-gates and re-wakes | the notice id is keyed on (child publication, parent publication); a later version updates it | `TestRepublishUpdatesTheNoticeAndNeverRenotifies` |
| R2 | One publisher spams strangers with notices | D11: 20 new notices a day per publisher, counted from notice objects | `TestSpamLinkOneNoticePerChild` |
| R3 | One publisher or story drives unbounded Jev calls | 5 gate calls per publisher and 20 per story per day; over the bound the arrival gets verdict unavailable with the reason and costs no call | `TestOpedGateIsBoundedPerPublisher` |
| R4 | Accepted op-eds wake the desk without a floor | 30 minutes between accepted wakes of a story; arrivals inside it join the next | `TestAcceptedWakeKeepsAMinimumGap` |
| R5 | A prompt carries every op-ed at full size | at most 5 shown, 40000 characters, "N more not shown" | `TestDeskFrameBoundsWhatItShows` |
| R6 | A gate-rejected op-ed becomes a public source link | only accepted op-eds are stamped; backoff rejects are read only; corpusd refuses a parent the publishing owner's gate rejected | `TestBackoffWakeRejectIsShownReadOnlyAndNeverCited`, `TestPaperResponseNeverCitesARejectedOped` |

Named residuals:
- The feeds filter on `created_at`; the index is `(object_kind, owner_id)`. The read is bounded by owner and kind, so it is cheap now; add `(object_kind, owner_id, created_at)` if notices per owner grow.
- (Repaired in the second review round, see below: the cursor is now a strict write-order sequence.)
- `false_reject` calibration has no source now that a reject is never stamped; it fires only if a desk revision cites a rejected op-ed by other means. A reject the desk uses needs a way for the desk to say so (not built).
- An unread `oped_wake` surviving a crash relies on the actor runtime's generic crash-boot gate ("pre-boot work is interrupted_by_restart", `internal/actorruntime/handler.go`); no test of mine drives that.
- `oped_intake` starts only for a computer that has published a story (the poll row is created by the first story publish) and then on every later boot; `notice_intake` only after a fork, take or merge.

## Second review round (Devin seat)

| # | Way it fails | Rule | Test |
|---|---|---|---|
| S1 | More than `limit` notices in one second wedge a victim's intake for good | the feed cursor is an opaque strict sequence (nanosecond clock, never repeated, assigned under the write lock), not whole-second `created_at`; it moves on every page; a republish keeps its place | `TestNoticeCursorDeliversEveryRowOfOneSecond` |
| S2 | One malformed feed item stalls the cursor | validated before commit; recorded in the poll's `Skipped` with its id and reason; the cursor moves | `TestMalformedFeedItemIsSkippedNotStalled` |
| S3 | The gate shares the computer's intake-label bucket | scope `oped_gate` is accepted by the gateway: bulk class plus a per-computer bucket (burst 5, 0.2 a second, 2 in flight); an unknown scope is still refused; the gate asks for it | `TestJevOpedGateScopeHasItsOwnBoundedBucket`, `TestOpedGateUsesItsOwnRateScope` |
| S4 | The per-story gate bound has no test | 20 calls a story a day | `TestOpedGateIsBoundedPerStory` |

More named residuals:
- N1: the contract names management the custodian of `choir.platform_poll`; the code makes it the kernel service. A management claim is a desk-protocol major change (the lock refused it), and the poll is host state with no model call. The contract should be amended to match.
- N3: nothing re-gates an `unavailable` op-ed, so no run 2 is ever produced. A later successful run would supersede it (corpusd and the fold already take the highest run); the trigger is not built.
- N4: a wake interrupted by a crash on a quiet story is not re-minted: crash restarts never resume work. The op-ed stays judged and the decision is on the tape; the next op-ed or reject threshold wakes the desk.
- The notice rows are read with the sequence in metadata, filtered by (kind, owner) and not indexed on the sequence; cheap while an owner has few notices.

## Third review round (Devin round 2)

| # | Way it fails | Rule | Test |
|---|---|---|---|
| T1 (B4) | After a restart with the clock behind, new notices get sequences below every consumer's cursor and are never delivered | the sequence is seeded from the largest stored one before the first is issued or read | `TestNoticeSequenceIsSeededFromStorageAtRestart` |
| T2 (N7) | A cursor that is not ours wedges the poll | treated as the start; the response says `cursor_reset` | `TestMalformedCursorMeansFromTheStartAndSaysSo` |
| T3 (N8) | A notice with no sequence is skipped for ever | the seed gives every sequence-less notice one | `TestNoticeWithoutASequenceIsGivenOneAndDelivered` |
| T4 (N9) | A fresh byline per publish evades the per-publisher gate bound, or burns another's budget | the feed item carries the publisher's owner id from the graph; the bound keys on it | `TestOpedGateBoundKeysOnThePublisherNotTheByline`, `TestOpedGateIsBoundedPerPublisher` |
| T5 (N11) | A feed-read failure or a missing judge spends the daily bound | a call counts when it is issued to Jev | `TestOpedGateBoundCountsOnlyIssuedCalls` |
| T6 (N13) | An undecodable notice row is dropped silently | the feed names it in `skipped`; the poll records it in `Skipped` | `TestUndecodableNoticeRowIsANamedSkip`, `TestFeedSkipsAreRecordedOnThePoll` |

Named residuals:
- N10: the per-story 20 a day can be exhausted by a flood so that honest op-eds get `unavailable`. The cure is the run 2 re-gate (not built; see N3).
- N12: `countRecentNoticesBy` scans every publisher's notices of the last day on each publish that notifies. Fine at today's volume; index the publisher or keep a counter if it grows.
- N14: a republished, already-consumed notice is not re-gated: the paper judges the first version of an op-ed it was told of, and later versions of the same fork are not seen.
