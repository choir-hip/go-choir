import XCTest
@testable import AutoradioKit

/// A session or engine failure is not the entry's failure; a reclaim never gives up a claim it holds; the radio's own session change is not the
/// listener's pause; the lab's fixture is never spoken on a listener's device.
@MainActor
final class AudioSessionHoldTests: XCTestCase {
    /// 2: the session cannot be taken for a while; the entry stays on air with no outcome, and speaks when it can.




    /// 1: a claim held in the state wanted is asserted again, never released.
    func testReclaimOfAHeldClaimReassertsAndDoesNotRelease() async {
        final class Owner: AudioSessionOwner, @unchecked Sendable {
            var releases = 0, reasserts = 0, claims = 0
            func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { claims += 1 }
            func release() { releases += 1 }
            func holds(_ state: AudioState) -> Bool { true }
            func reassert() async throws { reasserts += 1 }
        }
        let o = Owner()
        let c = SessionCoordinator(inner: o, log: EventLog(clock: LabClock()), audio: LiveAudio(), policy: { .switchOnAddress }, voiceOn: { false })
        try? await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        c.markSuspect("app in the background")
        // LiveAudio() is not running and not virtual here, so `ensureReady` reclaims
        _ = await c.ensureReady(reason: "play")
        XCTAssertEqual(o.releases, 0, "the held claim was not released")
        XCTAssertEqual(o.reasserts, 1)
    }

    /// 6: a paused radio gives the microphone up. Owner, 2026-11-25: "even if i pause playback, it still
    /// holds the audio session, and disrupts other apps that would eg want to use the microphone". The claim
    /// is kept (the page and shared code live on it); the STATE becomes playback, which carries no microphone.
    func testPausingDropsTheCaptureSessionAndKeepsTheClaim() async {
        final class Owner: AudioSessionOwner, @unchecked Sendable {
            var claimed: [AudioState] = []
            var releases = 0
            func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { claimed.append(state) }
            func release() { releases += 1 }
            func holds(_ state: AudioState) -> Bool { claimed.last == state }
            func reassert() async throws {}
        }
        let o = Owner()
        let c = SessionCoordinator(inner: o, log: EventLog(clock: LabClock()), audio: LiveAudio(), policy: { .alwaysListening }, voiceOn: { true })
        try? await c.claim(.listening, preferBuiltInMic: false, reason: "start")
        XCTAssertEqual(o.claimed.last, .listening, "hands-free on and playing: the capture session")
        await c.setPaused(true)
        XCTAssertEqual(o.claimed.last, .playing, "paused: playback, no microphone")
        XCTAssertEqual(o.releases, 0, "the claim is kept while paused")
        await c.setPaused(false)
        XCTAssertEqual(o.claimed.last, .listening, "resumed: back to what the policy asks for")
    }

    /// 3: a pause arriving just after the radio changed the session is the system reacting to it.
    func testAPauseRightAfterTheRadiosOwnSessionChangeIsNotTheListeners() async {
        let o = NullSessionOwner()
        let c = SessionCoordinator(inner: o, log: EventLog(clock: LabClock()), audio: LiveAudio(), policy: { .switchOnAddress }, voiceOn: { false })
        XCTAssertFalse(c.changedWithin(2.5))
        try? await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        c.markSuspect("background")
        _ = await c.ensureReady(reason: "foreground")
        XCTAssertTrue(c.changedWithin(2.5))
    }

    /// 5: nothing is loaded at start on a listener's device, and nothing is spoken.
    func testTheFixtureEditionIsNeverLoadedOnAListenersDevice() async {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("rt-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner(), emptyProgramme: true)
        XCTAssertTrue(rt.player.document.blocks.isEmpty)
        XCTAssertNotEqual(rt.player.document.id, "fixture-1")
        let lab = LabRuntime(supportDir: dir, session: NullSessionOwner())
        XCTAssertEqual(lab.player.document.id, "fixture-1", "the lab and the tests keep the fixture")
    }

    /// 4: the empty document of a stop is not a programme that ran dry.
    func testStoppingSpokenDoesNotReportAProgrammeThatRanDry() async {
        let a = virtualAudio()
        let r = StageRegistry.builtIn()
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: VoiceCatalog.bundledStageID))
        let p = ProgrammePlayer(audio: a, registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: ProgrammeDocument(id: "none", title: "", blocks: []))
        let told = Locked<[String]>([])
        p.onRanDry = { id, _ in told.write { $0.append(id) } }
        p.start(at: 0)
        try? await Task.sleep(nanoseconds: 200_000_000)
        XCTAssertTrue(told.value.isEmpty, "told: \(told.value)")
    }
}
