import XCTest
@testable import AutoradioKit

/// The radio lost its audio after a take and could not get it back (owner's phone, 2026-11-18): the engine did not start after the capture
/// session went back to the playback one, and the code asserted the same session again on every try. Failure list:
///  1. a listening -> playing switch builds a new engine before the next block (it does not restart the old one);
///  2. an engine start that fails is tried once on a new engine;
///  3. reassert succeeds but the engine will not start: the coordinator releases and claims anew (not the same reassert again);
///  4. the full re-claim fails too: ensureReady says no (the entry stays held, with backoff, as before);
///  5. while an entry is held for the session the bar and the radio page are not "playing";
///  6. a take's recorder is never left alive after publish, discard, a too-short take or a failed start;
///  7. a block that finds the engine down asks the host to take it again.
final class AudioHeldTests: XCTestCase {
    final class Owner: AudioSessionOwner, @unchecked Sendable {
        var releases = 0, reasserts = 0, claims = 0, held: AudioState?, failClaim = false
        func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { if failClaim { throw Boom() }; claims += 1; held = state }
        func release() { releases += 1; held = nil }
        func holds(_ state: AudioState) -> Bool { held == state }
        func reassert() async throws { reasserts += 1 }
        func describe() -> String { "fake" }
    }
    struct Boom: Error {}

    /// A coordinator over a LiveAudio whose engine start is the test's (no hardware).
    func rig(voiceOn: Bool = false, drive: @escaping @Sendable (Int) throws -> Void) -> (SessionCoordinator, Owner, LiveAudio, Locked<Int>) {
        let o = Owner(); let a = LiveAudio(); let calls = Locked(0)
        a.driveForTest = { _, _ in let n = calls.write { $0 += 1; return $0 }; try drive(n) }
        try? a.start(bus: AudioBus(), clock: LabClock(), wantMic: true, mic: false)
        let c = SessionCoordinator(inner: o, log: EventLog(clock: LabClock()), audio: a, policy: { .switchOnAddress }, voiceOn: { voiceOn })
        return (c, o, a, calls)
    }

    func testAListeningToPlayingSwitchBuildsANewEngine() async throws {
        let (c, _, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        let b0 = a.engineBuilds
        try await c.setRecordingTake(true)
        XCTAssertEqual(c.current, .listening)
        XCTAssertGreaterThan(a.engineBuilds, b0, "to the capture session: a new engine")
        let b1 = a.engineBuilds
        try await c.setRecordingTake(false)
        XCTAssertEqual(c.current, .playing)
        XCTAssertGreaterThan(a.engineBuilds, b1, "and back to the playback session: a new engine, not the old one started again")
        XCTAssertTrue(a.running); XCTAssertFalse(a.needsRecovery)
    }

    func testAStartThatFailsIsTriedOnceOnANewEngine() throws {
        let (_, _, a, calls) = rig { n in if n == 2 { throw Boom() } }       // call 1 was the first start in rig
        let b = a.engineBuilds
        a.interrupted()
        try a.resume(voiceProcessing: false, mic: false)
        XCTAssertEqual(calls.value, 3, "failed once, then started")
        XCTAssertGreaterThan(a.engineBuilds, b)
        XCTAssertTrue(a.running)
    }

    func testReassertThatLeavesTheEngineDownDoesAFullReclaimInsteadOfRepeating() async throws {
        let (c, o, a, calls) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.interrupted()
        // the first two starts (the engine and its new-engine retry, both after the reassert) fail; the start after the full re-claim works
        let claimsBefore = o.claims
        a.driveForTest = { _, _ in let n = calls.write { $0 += 1; return $0 }; if o.releases == 0 { throw Boom() }; _ = n }
        let ok = await c.ensureReady(reason: "play")
        XCTAssertTrue(ok)
        XCTAssertEqual(o.reasserts, 1, "the claim was asserted once, not again and again")
        XCTAssertEqual(o.releases, 1, "then given up")
        XCTAssertEqual(o.claims, claimsBefore + 1, "and taken anew")
        XCTAssertTrue(a.running)
    }

    func testAFullReclaimThatFailsToo() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.driveForTest = { _, _ in throw Boom() }
        a.interrupted()
        let ok = await c.ensureReady(reason: "play")
        XCTAssertFalse(ok, "the entry stays held; the player retries with backoff")
        XCTAssertEqual(o.releases, 1)
        XCTAssertTrue(a.needsRecovery)
    }

    @MainActor func testAnEntryHeldForTheSessionIsNotPlayingOnTheBarOrThePage() async {
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        p.sessionReady = { _ in "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        XCTAssertTrue(p.isWaitingForSession)
        var b = BarInfo(); b.state = "playing"; b.playing = true
        let ob = p.overlay(b)
        XCTAssertEqual(ob.state, "paused"); XCTAssertFalse(ob.playing)
        var r = RadioInfo(); r.playing = true
        let or = p.overlay(r)
        XCTAssertFalse(or.playing); XCTAssertTrue(or.paused)
    }

    func testABlockThatFindsTheEngineDownGetsAnAnswerAtOnce() async {
        let (_, _, a, _) = rig { _ in }
        let asked = Locked(0)
        a.onNeedsEngine = { asked.write { $0 += 1 }; return false }
        a.interrupted()
        let t0 = Date()
        let played = await a.playProgramme([Float](repeating: 0, count: 1600), rate: 16_000, tag: 0)
        XCTAssertFalse(played, "nothing was heard")
        XCTAssertEqual(asked.value, 1)
        XCTAssertLessThan(Date().timeIntervalSince(t0), 2, "no 45 s wait")
    }

    func testAnEngineLostBlockStopsTheLoopAtThatBlockAndIsNeverRanDry() async {
        let a = LiveAudio(); let calls = Locked(0)
        a.driveForTest = { _, _ in calls.write { $0 += 1 } }
        try? a.start(bus: AudioBus(), clock: LabClock(), wantMic: false, mic: false)
        a.interrupted()
        a.onNeedsEngine = { false }
        let r = StageRegistry.builtIn(); r.register(.voice, LongVoice.self) { LongVoice($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "long-voice"))
        let p = ProgrammePlayer(audio: a, registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                                document: ProgrammeDocument(id: "story", title: "S", blocks: [.speech(text: "One."), .speech(text: "Two.")]))
        let lost = Locked<[String]>([]), dry = Locked<[String]>([])
        p.onEngineLost = { id, n in lost.write { $0.append("\(id)#\(n)") } }
        p.onRanDry = { id, _ in dry.write { $0.append(id) } }
        p.start(at: 1)
        _ = await poll(5) { !lost.value.isEmpty }
        XCTAssertEqual(lost.value, ["story#1"])
        XCTAssertEqual(p.blockIndex, 1, "the block that did not play")
        try? await Task.sleep(nanoseconds: 200_000_000)
        XCTAssertTrue(dry.value.isEmpty, "the run did not end")
    }

    @MainActor func testAnEngineLostMidEntryHoldsItAndNothingIsPostedUntilItFinishesHeardOnce() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        var ready = true
        p.sessionReady = { _ in ready ? nil : "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        XCTAssertEqual(e.speaks, ["speak:a"])
        ready = false
        p.engineLost(documentID: "rs:a")
        _ = await poll { p.isWaitingForSession }
        XCTAssertTrue(p.isWaitingForSession)
        XCTAssertEqual(s.sheet.onAirEntry?.id, "a", "still on air")
        XCTAssertNil(s.sheet.entries.first { $0.id == "a" }?.ended, "no ended op, no heard, no failed")
        var b = BarInfo(); b.state = "playing"; b.playing = true
        XCTAssertEqual(p.overlay(b).state, "paused")
        ready = true; p.sessionMayBeBack()
        _ = await poll { e.log.contains("resumeSpoken") }
        XCTAssertTrue(e.log.contains("resumeSpoken"))
        XCTAssertEqual(e.speaks, ["speak:a"], "not spoken again from the start")
        XCTAssertFalse(p.isWaitingForSession)
        p.documentEnded(id: "rs:a", anyPlayed: true)
        XCTAssertEqual(s.sheet.entries.first { $0.id == "a" }?.ended?.outcome, .finished)
        XCTAssertEqual(s.sheet.entries.first { $0.id == "a" }?.state, .heard)
    }

    @MainActor func testAnEntryThatMadeNoSoundIsNotAVerdict() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        p.documentEnded(id: "rs:a", anyPlayed: false)
        let a = s.sheet.entries.first { $0.id == "a" }
        XCTAssertEqual(a?.ended?.outcome, .failed, "failed: the server records no verdict for it")
        XCTAssertNotEqual(a?.state, .heard)
    }

    func testAnInterruptionWithoutAResumeIsNotAnswerForByABlock() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.programmePlay()
        c.heard("interrupted", "")
        await c.interruptionEnded(shouldResume: false)
        let claims = o.claims, releases = o.releases, reasserts = o.reasserts
        let up = await a.onNeedsEngine!()
        XCTAssertFalse(up)
        XCTAssertEqual([o.claims, o.releases, o.reasserts], [claims, releases, reasserts], "the other audio keeps the session")
        let retry = await c.ensureReady(reason: "retry")
        XCTAssertFalse(retry, "a backoff retry does not take it either")
        let play = await c.ensureReady(reason: "play")
        XCTAssertTrue(play, "Play does")
        XCTAssertTrue(a.running)
    }

    func testAStopIsNotUndoneByABlockThatFindsTheEngineDown() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.programmePlay()
        a.stop()                                       // LabRuntime.stop: audio first, the claim after
        var up = await a.onNeedsEngine!()
        XCTAssertFalse(up, "stopped: not wanted")
        c.release()
        let before = o.claims
        up = await a.onNeedsEngine!()
        XCTAssertFalse(up)
        XCTAssertEqual(o.claims, before)
    }

    func testAPausedProgrammeIsNotBroughtBackByABlock() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.programmePlay(); a.programmePause()
        a.interrupted()
        let before = (o.claims, o.releases, o.reasserts)
        let up = await a.onNeedsEngine!()
        XCTAssertFalse(up)
        XCTAssertTrue(before == (o.claims, o.releases, o.reasserts))
    }

    func testAPlayingProgrammeWithTheEngineDownIsBroughtBack() async throws {
        let (c, _, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.programmePlay()
        a.interrupted()
        let up = await a.onNeedsEngine!()
        XCTAssertTrue(up); XCTAssertTrue(a.running)
    }

    func testARemoteSoundPlayingKeepsTheClaimWhenTheEngineWillNotStart() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        c.remoteAudioActive = { true }
        a.driveForTest = { _, _ in throw Boom() }
        a.interrupted()
        let ok = await c.ensureReady(reason: "play")
        XCTAssertFalse(ok)
        XCTAssertEqual(o.releases, 0, "a podcast is not cut by a deactivation")
    }

    func testAReclaimDuringATakeReleasesNothing() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        try await c.setRecordingTake(true)
        XCTAssertEqual(c.current, .listening)
        a.driveForTest = { _, _ in throw Boom() }
        a.interrupted()
        let releases = o.releases
        let ok = await c.ensureReady(reason: "play")
        XCTAssertFalse(ok)
        XCTAssertEqual(o.releases, releases, "the session is not deactivated under the recorder")
    }

    func testAFailedSwitchToListeningRefusesTheTakeAndTheRadioIsAsItWas() async throws {
        let (c, _, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        a.driveForTest = { _, mic in if mic { throw Boom() } }       // the capture session's engine will not start
        do { try await c.setRecordingTake(true); XCTFail("the take must not start") } catch {}
        XCTAssertFalse(c.isRecordingTake)
        XCTAssertEqual(c.current, .playing)
        XCTAssertTrue(a.running, "the radio's engine is back")
    }

    @MainActor func testPlayWhileHeldRetriesAndDoesNotPause() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        var ready = false
        p.sessionReady = { _ in ready ? nil : "the audio session could not be taken" }
        XCTAssertFalse(p.playPressed(), "nothing held: an ordinary press")
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        XCTAssertTrue(p.isWaitingForSession)                          // a hold at the start
        ready = true
        XCTAssertTrue(p.playPressed(), "a retry only")
        _ = await poll { e.log.contains("speak:a") }
        XCTAssertTrue(e.log.contains("speak:a"))
        XCTAssertFalse(p.isWaitingForSession)
        ready = false
        p.engineLost(documentID: "rs:a")                              // a hold in the middle
        _ = await poll { p.isWaitingForSession }
        ready = true
        XCTAssertTrue(p.playPressed(), "a retry only")
        _ = await poll { e.log.contains("resumeSpoken") }
        XCTAssertTrue(e.log.contains("resumeSpoken"))
        XCTAssertFalse(p.playPressed(), "no longer held: an ordinary press")
    }

    // MARK: Round 3

    /// A held entry (mid-entry), for the tests of who paused.
    @MainActor func heldMid(delays: [TimeInterval] = [3600]) async -> (RunSheetPlayer, RunSheetStore, FakeEngine, Locked<Bool>) {
        RunSheetPlayer.sessionRetryDelays = delays
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let ready = Locked(true)
        p.sessionReady = { _ in ready.value ? nil : "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        ready.value = false
        p.engineLost(documentID: "rs:a", block: 1)
        _ = await poll { p.isWaitingForSession }
        return (p, s, e, ready)
    }

    @MainActor func testOnePlayAfterANoResumeInterruptionPlays() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        let (p, _, e, ready) = await heldMid()
        p.spokenPaused = { true }; p.spokenPausedByListener = { false }          // the system's pause: userPaused, but not the listener's
        ready.value = true
        XCTAssertTrue(p.playPressed())
        _ = await poll { e.log.contains("resumeSpoken") }
        XCTAssertTrue(e.log.contains("resumeSpoken"), "one press plays")
    }

    @MainActor func testAFailedThenRecoveredResumeOverTheSystemsPauseGoesOnByItself() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        let (p, _, e, ready) = await heldMid(delays: [0.02])
        p.spokenPaused = { true }; p.spokenPausedByListener = { false }
        ready.value = false; p.sessionMayBeBack()                                 // fails once, the timer is armed
        try? await Task.sleep(nanoseconds: 100_000_000)
        ready.value = true
        _ = await poll { e.log.contains("resumeSpoken") }
        XCTAssertTrue(e.log.contains("resumeSpoken"), "no press")
    }

    @MainActor func testTheListenersOwnPauseIsKeptByTheRetryAndEndedByPlay() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        let (p, _, e, ready) = await heldMid(delays: [0.02])
        p.spokenPaused = { true }; p.spokenPausedByListener = { true }
        ready.value = true
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(e.log.contains("resumeSpoken"), "the retry does not undo the listener's pause")
        XCTAssertTrue(p.isWaitingForSession, "it stays held")
        XCTAssertTrue(p.playPressed())
        _ = await poll { e.log.contains("resumeSpoken") }
        XCTAssertTrue(e.log.contains("resumeSpoken"), "their Play ends it")
    }

    func testWhoPausedIsRecorded() {
        let a = virtualAudio()
        let p = ProgrammePlayer(audio: a, registry: StageRegistry.builtIn(), log: EventLog(clock: LabClock()), configuration: StageRegistry.defaultConfiguration(),
                                document: ProgrammeDocument(id: "none", title: "", blocks: []))
        p.pause(reason: "system interruption"); XCTAssertTrue(p.userPaused); XCTAssertFalse(p.pausedByListener)
        p.pause(reason: "route lost"); XCTAssertTrue(p.pausedByListener)
        p.pause(reason: "system interruption"); p.play(); XCTAssertFalse(p.userPaused)
        p.pause(reason: "listener"); XCTAssertTrue(p.pausedByListener)
    }

    @MainActor func testTwoOverlappingBeginsSpeakOnce() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let ready = Locked(false)
        p.sessionReady = { _ in try? await Task.sleep(nanoseconds: 60_000_000); return ready.value ? nil : "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        XCTAssertTrue(p.isWaitingForSession)
        ready.value = true
        p.sessionMayBeBack(); _ = p.playPressed()
        _ = await poll { e.speaks.count >= 1 }
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(e.speaks, ["speak:a"], "the entry is spoken once, not again from its start")
    }

    func testALossNotedByACancelledLoopDoesNotHoldTheNextDocument() {
        let a = virtualAudio()
        let p = ProgrammePlayer(audio: a, registry: StageRegistry.builtIn(), log: EventLog(clock: LabClock()), configuration: StageRegistry.defaultConfiguration(),
                                document: ProgrammeDocument(id: "none", title: "", blocks: []))
        p.engineLostFlag.value = true
        p.start(at: 0)
        XCTAssertFalse(p.engineLostFlag.value)
    }

    func testABargeInStopsTheVoiceLinesWaitersAndNotTheProgrammeBlocks() {
        let (_, _, a, _) = rig { _ in }
        let programme = Once<Void>(), voice = Once<Void>(), vid = UUID()
        a.waiters.write { $0[UUID()] = programme; $0[vid] = voice }
        a.voiceWaiterIDs.write { _ = $0.insert(vid) }
        a.releaseVoiceWaiters()
        XCTAssertFalse(voice.resume(()), "the voice line's wait was released")
        XCTAssertTrue(programme.resume(()), "the programme block's wait was not: its remaining words are still to be said")
    }

    @MainActor func testSigningOutOnAnEntryThatNeverSoundedIsNotHeard() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        p.sessionReady = { _ in "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        p.stopAll()
        let a = s.sheet.entries.first { $0.id == "a" }
        XCTAssertEqual(a?.state, .skipped); XCTAssertEqual(a?.ended?.outcome, .failed, "no verdict")
        // one that sounded leaves as before
        let s2 = RunSheetStore(); let p2 = RunSheetPlayer(store: s2); p2.engine = FakeEngine()
        p2.playNow(RunSheetEntry(id: "b", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p2.settle()
        p2.stopAll()
        let b = s2.sheet.entries.first { $0.id == "b" }
        XCTAssertEqual(b?.state, .heard); XCTAssertEqual(b?.ended?.outcome, .left)
    }

    @MainActor func testATapOnAStoryWhileWaitingForPlayCountsAsAPress() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let reasons = Locked<[String]>([])
        p.sessionReady = { r in reasons.write { $0.append(r) }; return nil }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        XCTAssertEqual(reasons.value.last, "play", "an explicit placement asks like a press")
    }

    func testAStaleEngineFlagDoesNotRefuseATakeOnAWorkingEngine() async throws {
        let (c, _, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        try await c.setRecordingTake(true)
        c._engineOK = false                                           // left over from an earlier failure; the engine has recovered
        XCTAssertFalse(a.needsRecovery)
        try await c.setRecordingTake(true)                            // must not throw
    }

    func testAClaimThatFailsForATakeLeavesNoTakeFlag() async {
        let (c, o, _, _) = rig { _ in }
        o.failClaim = true
        do { try await c.setRecordingTake(true); XCTFail("must throw") } catch {}
        XCTAssertFalse(c.isRecordingTake)
    }

    // MARK: Round 4

    @MainActor func testAnEntryHeldAtItsStartStaysHeldOverTheListenersPauseUntilPlay() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let ready = Locked(false)
        p.sessionReady = { _ in ready.value ? nil : "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        XCTAssertTrue(p.isWaitingForSession)
        p.spokenPausedByListener = { true }                          // a lost route, or a lock-screen pause
        ready.value = true
        p.sessionMayBeBack(reason: "retry")
        p.sessionMayBeBack(reason: "resume")                         // the foreground
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(e.speaks.isEmpty, "nothing plays")
        XCTAssertTrue(p.isWaitingForSession, "still held")
        XCTAssertTrue(p.playPressed())
        _ = await poll { e.speaks.count == 1 }
        XCTAssertEqual(e.speaks, ["speak:a"], "one press plays")
    }

    @MainActor func testOpeningTheAppDoesNotReviveAPausedHeldEntry() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("rt-\(UUID().uuidString)"), session: NullSessionOwner(), emptyProgramme: true)
        let s = RunSheetStore(); let q = RunSheetPlayer(store: s); let e = FakeEngine(); q.engine = e
        let ready = Locked(false)
        q.sessionReady = { _ in ready.value ? nil : "the audio session could not be taken" }
        q.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await q.settle()
        let c = PlaybackContinuity(runtime: rt, queue: q)
        rt.player.pause(reason: "system interruption")              // an interruption that gave no go-ahead
        ready.value = true
        c.returnedToForeground()
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(e.speaks.isEmpty); XCTAssertTrue(q.isWaitingForSession)
    }

    func testAFailedReclaimClaimPutsTheEngineBack() async throws {
        let (c, o, a, _) = rig { _ in }
        try await c.claim(.playing, preferBuiltInMic: false, reason: "start")
        c.release()
        o.failClaim = true
        let ok = await c.ensureReady(reason: "play")
        XCTAssertFalse(ok)
        XCTAssertTrue(a.running, "the engine is started again, and observed again")
    }

    @MainActor func testTheLockScreensPlayReachesAHeldEntryAndItsPauseStaysThePausers() async {
        let old = RunSheetPlayer.sessionRetryDelays; defer { RunSheetPlayer.sessionRetryDelays = old }
        RunSheetPlayer.sessionRetryDelays = [3600]
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let ready = Locked(false)
        p.sessionReady = { _ in ready.value ? nil : "the audio session could not be taken" }
        p.playNow(RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5)); await p.settle()
        p.spokenPausedByListener = { true }
        ready.value = true
        XCTAssertTrue(p.remoteTransport("play"), "taken by the run sheet player")
        _ = await poll { e.speaks.count == 1 }
        XCTAssertEqual(e.speaks, ["speak:a"], "it plays once")
        XCTAssertFalse(p.remoteTransport("pause"), "nothing held and nothing remote: the programme player pauses it")
        let a = ProgrammePlayer(audio: virtualAudio(), registry: StageRegistry.builtIn(), log: EventLog(clock: LabClock()), configuration: StageRegistry.defaultConfiguration(),
                                document: ProgrammeDocument(id: "none", title: "", blocks: []))
        a.pause(reason: "remote command pause")
        XCTAssertTrue(a.pausedByListener, "the lock screen's pause is the listener's")
    }
}
