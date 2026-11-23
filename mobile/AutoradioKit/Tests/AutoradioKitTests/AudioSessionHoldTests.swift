import XCTest
@testable import AutoradioKit

/// A session or engine failure is not the entry's failure; a reclaim never gives up a claim it holds; the radio's own session change is not the
/// listener's pause; the lab's fixture is never spoken on a listener's device.
@MainActor
final class AudioSessionHoldTests: XCTestCase {
    func entry(_ id: String) -> RunSheetEntry { RunSheetEntry(id: id, kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 5) }

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
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "avspeech"))
        let p = ProgrammePlayer(audio: a, registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: ProgrammeDocument(id: "none", title: "", blocks: []))
        let told = Locked<[String]>([])
        p.onRanDry = { id, _ in told.write { $0.append(id) } }
        p.start(at: 0)
        try? await Task.sleep(nanoseconds: 200_000_000)
        XCTAssertTrue(told.value.isEmpty, "told: \(told.value)")
    }
}
