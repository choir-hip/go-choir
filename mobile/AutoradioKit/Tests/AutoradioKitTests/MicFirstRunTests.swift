import XCTest
@testable import AutoradioKit

/// The microphone granted while the app already runs (a fresh install: the radio starts in setup, the microphone is asked at "Can I hear you?").
/// Found on a phone: `micPermitted` was set only when the engine first started, the grant came later through onboarding's own request, so every
/// later attempt skipped the input node (rate 0, tap fail) until the app was restarted. No audio hardware: `driveForTest` stands in for the engine.
final class MicFirstRunTests: XCTestCase {
    /// An engine "running" for playback only, as the radio is during setup (no permission yet).
    func runningForPlayback(granted: @escaping @Sendable () -> Bool) throws -> LiveAudio {
        let a = LiveAudio()
        a.driveForTest = { _, _ in }
        a.micPermissionProbe = granted
        a.micRetryBase = 0.01
        try a.start(bus: AudioBus(), clock: LabClock(), wantMic: false, mic: false)
        return a
    }
    func waitUntil(_ what: String, timeout: Double = 3, _ ok: @escaping () -> Bool) async {
        let end = Date().addingTimeInterval(timeout)
        while !ok(), Date() < end { try? await Task.sleep(nanoseconds: 10_000_000) }
    }

    func testAGrantWhileAnEngineBuiltForPlaybackRunsGivesAWorkingTapOnANewEngine() throws {
        let granted = Locked<Bool>(false)
        let a = try runningForPlayback { granted.value }
        let buildsBefore = a.engineBuilds
        granted.value = true                                   // the listener allowed it in onboarding; LiveAudio was never told
        try a.resume(voiceProcessing: true, mic: true)         // the voice switch: no category change is needed to ask for the microphone
        XCTAssertTrue(a.micActive, "the microphone is open without a restart")
        XCTAssertEqual(a.micState, .open)
        XCTAssertGreaterThan(a.engineBuilds, buildsBefore, "a new engine was built after the grant")
    }

    func testARateZeroInputIsRetriedOnANewEngineAndIsBounded() async throws {
        let a = try runningForPlayback { true }
        a.inputRateForTest = { _ in 0 }
        let before = a.engineBuilds
        try a.resume(voiceProcessing: true, mic: true)
        await waitUntil("gave up") { a.micState == .failed }
        XCTAssertEqual(a.micState, .failed)
        XCTAssertFalse(a.micActive)
        let builds = a.engineBuilds
        XCTAssertGreaterThan(builds, before + 1, "each retry ran on a new engine")
        XCTAssertLessThanOrEqual(builds - before, 1 + 6, "bounded by the retry count")
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(a.engineBuilds, builds, "no retry after it gave up")
    }

    func testARateZeroInputThatComesOnARetryOpensTheTap() async throws {
        let a = try runningForPlayback { true }
        a.inputRateForTest = { build in build >= 4 ? 48000 : 0 }
        try a.resume(voiceProcessing: true, mic: true)
        await waitUntil("open") { a.micActive }
        XCTAssertTrue(a.micActive)
        XCTAssertEqual(a.micTapRate, 48000)
        XCTAssertEqual(a.micState, .open)
    }

    func testWithoutPermissionThereIsNoRetryAndItIsReported() async throws {
        let a = try runningForPlayback { false }
        a.inputRateForTest = { _ in 0 }
        let before = a.engineBuilds
        try a.resume(voiceProcessing: true, mic: true)
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(a.micActive)
        XCTAssertEqual(a.micState, .refused)
        XCTAssertEqual(a.engineBuilds, before, "nothing was retried")
    }

    func testTheLogNeverSaysOkAtRateZero() {
        XCTAssertEqual(MicLog.tap(active: true, rate: 0), "fail")
        XCTAssertEqual(MicLog.tap(active: false, rate: 48000), "fail")
        XCTAssertEqual(MicLog.tap(active: true, rate: 48000), "ok")
        XCTAssertEqual(MicLog.tap(active: false, rate: 0), "fail")
    }

    func testOnboardingLeavesCheckingWhenTheMicrophoneCannotOpen() {
        XCTAssertEqual(MicLog.onboardingState(.refused), "refused")
        XCTAssertEqual(MicLog.onboardingState(.failed), "silent")
        XCTAssertNil(MicLog.onboardingState(.open), "an open microphone goes on listening")
        XCTAssertNil(MicLog.onboardingState(.pending), "a retry in progress goes on listening")
    }
}
