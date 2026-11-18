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
        var releases = 0, reasserts = 0, claims = 0, held: AudioState?
        func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { claims += 1; held = state }
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
        let failing = Locked(true)
        a.driveForTest = { _, _ in calls.write { $0 += 1 }; if failing.value { throw Boom() } }
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

    @MainActor func testATakesRecorderIsNeverLeftAliveAfterThePathEnds() async {
        final class Rec: TakeRecorderEngine { var live = false, fail = false, secs = 10.0
            func start(to url: URL) throws { if fail { throw LabError.unavailable("x") }; live = true; try? Data(repeating: 1, count: 4000).write(to: url) }
            func stop() -> Double { live = false; return secs }
            func cancel() { live = false } }
        let rec = Rec(), gate = FakeGate(), dir = FileManager.default.temporaryDirectory.appendingPathComponent("held-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: dir) }
        var clock = Date(timeIntervalSince1970: 1000)
        func make() -> TakeSession {
            TakeSession(recorder: rec, transcriber: FakeTranscriber(), gate: gate, preview: FakePreview(), client: { nil }, micAllowed: { true }, responds: { nil }, own: TakeOwn(), dir: dir, now: { clock })
        }
        var s = make(); rec.fail = true; await s.start()
        XCTAssertFalse(rec.live); XCTAssertEqual(gate.given, 1, "a failed start gives the session back")
        rec.fail = false; s = make(); await s.start(); XCTAssertTrue(rec.live)
        clock += 10; await s.stop(); XCTAssertFalse(rec.live, "stop ends the recorder")
        s.discard(); XCTAssertFalse(rec.live); XCTAssertEqual(gate.given, 2)
        s = make(); await s.start(); s.discard(); XCTAssertFalse(rec.live, "discard while recording")
        rec.secs = 1; s = make(); await s.start(); clock += 1; await s.stop(); XCTAssertFalse(rec.live, "a too-short take")
    }

    func testABlockThatFindsTheEngineDownAsksTheHostForIt() async {
        let (_, _, a, _) = rig { _ in }
        let asked = Locked(0)
        a.onNeedsEngine = { asked.write { $0 += 1 } }
        a.interrupted()
        let t = Task { await a.playProgramme([Float](repeating: 0, count: 1600), rate: 16_000, tag: 0) }
        _ = await poll(2) { asked.value > 0 }
        XCTAssertEqual(asked.value, 1)
        t.cancel(); _ = await t.value
    }
}
