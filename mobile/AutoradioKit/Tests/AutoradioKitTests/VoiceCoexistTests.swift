import XCTest
@testable import AutoradioKit

/// Voice on must not stop the radio, and the radio must not fight other apps for the microphone.
@MainActor
final class VoiceCoexistTests: XCTestCase {
    func coordinator(voice: Bool = true) -> (SessionCoordinator, CountingOwner) {
        let o = CountingOwner()
        return (SessionCoordinator(inner: o, log: EventLog(clock: LabClock()), audio: LiveAudio(), policy: { .switchOnAddress }, voiceOn: { voice }), o)
    }
    final class CountingOwner: AudioSessionOwner, @unchecked Sendable {
        var claims: [AudioState] = []; var reasserts = 0
        func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { claims.append(state) }
        func holds(_ state: AudioState) -> Bool { !claims.isEmpty && claims.last == state }
        func reassert() async throws { reasserts += 1 }
    }

    /// A: the route change that voice on itself causes (the Bluetooth headset leaves A2DP) is not the headset going away: the radio is not paused.



    /// B(b): in the background voice on gives the microphone up and plays on; in the foreground it takes it back.
    func testTheBackgroundGivesUpTheMicrophoneAndTheForegroundTakesItBack() async {
        let (c, _) = coordinator()
        XCTAssertEqual(c.target(for: .listening), .talking)
        await c.setBackgrounded(true)
        XCTAssertEqual(c.target(for: .listening), .playing, "a playback session: no input for another app to be blocked by")
        XCTAssertTrue(c.audio.micSuspended)
        await c.setBackgrounded(false)
        XCTAssertEqual(c.target(for: .listening), .talking)
        XCTAssertFalse(c.audio.micSuspended)
    }

    /// F6: the lifecycle notices set the fact in order; the tasks that carry it out may run in any order and still end with the app in front when it is.
    func testALockAndUnlockInAQuickSuccessionLeavesTheAppInFront() async {
        let (c, _) = coordinator()
        c.noteBackgrounded(true); c.noteBackgrounded(false)       // didEnterBackground, then didBecomeActive
        await c.applyBackgrounded(); await c.applyBackgrounded()    // the two tasks, in either order
        XCTAssertFalse(c.isBackgrounded); XCTAssertFalse(c.audio.micSuspended)
        c.noteBackgrounded(true); await c.applyBackgrounded()
        XCTAssertTrue(c.isBackgrounded); XCTAssertTrue(c.audio.micSuspended)
        c.noteBackgrounded(false); await c.applyBackgrounded(); await c.applyBackgrounded()
        XCTAssertFalse(c.isBackgrounded); XCTAssertFalse(c.audio.micSuspended)
    }

    /// B(a),(c): while another app has the session nothing here asks for it back, except the foreground, the interruption's end and Play.
    func testWhileAnotherAppHasTheSessionTheRetryDoesNotAskForItBack() async {
        let (c, o) = coordinator()
        try? await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        c.heard("interrupted", "")
        XCTAssertTrue(c.isInterrupted)
        XCTAssertTrue(c.audio.micSuspended, "the input tap is stopped")
        let before = o.reasserts + o.claims.count
        let ok = await c.ensureReady(reason: "retry")
        XCTAssertFalse(ok)
        XCTAssertEqual(o.reasserts + o.claims.count, before, "no claim, no reassert while another app is recording")
        _ = await c.ensureReady(reason: "play")
        XCTAssertFalse(c.isInterrupted, "Play takes the session back")
    }
    func testAnInterruptionsEndGivesTheMicrophoneBack() async {
        let (c, _) = coordinator()
        try? await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        c.heard("interrupted", "")
        await c.interruptionEnded(shouldResume: true)
        XCTAssertFalse(c.isInterrupted)
        XCTAssertFalse(c.audio.micSuspended)
    }

    /// A: the pause for speech that comes to nothing is given back.
    func testAPauseForSpeechWithNothingHeardIsGivenBack() async {
        TurnEngine.nothingHeardSeconds = 0.05; defer { TurnEngine.nothingHeardSeconds = 8 }
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("vc-\(UUID().uuidString)"), session: NullSessionOwner())
        rt.player.play()
        await rt.engine.hfApply(.pausePlayback)
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertTrue(rt.audio.programmeAudible || !rt.player.userPaused)
        let lvl = rt.player.level
        XCTAssertEqual(lvl, .play, "the radio plays on after nothing was heard")
    }
}
