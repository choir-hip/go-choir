import XCTest
@testable import AutoradioKit

@MainActor final class FakeRecorder: TakeRecorderEngine {
    var seconds = 10.0, bytes = 4000, started: [URL] = [], failStart = false
    func start(to url: URL) throws { if failStart { throw LabError.unavailable("x") }; started.append(url); try? Data(repeating: 1, count: bytes).write(to: url) }
    func stop() -> Double { seconds }
    func cancel() { if let u = started.last { try? FileManager.default.removeItem(at: u) } }
}
struct FakeTranscriber: TakeTranscribing { var words = "A neighbour speaking."; func transcribe(file: URL) async -> String { words } }
@MainActor final class FakeGate: TakeRadioGate {
    var yielded = 0, given = 0, refuse: String?
    func yieldForTake() async -> String? { if let r = refuse { return r }; yielded += 1; return nil }
    func giveBack() { given += 1 }
}
@MainActor final class FakePreview: TakePreviewing { var log: [String] = []; func play(_ url: URL) { log.append("play") }; func stop() { log.append("stop") } }

/// The take recorder (failure list: TakeSession.swift, R1 to R3, R6). What to do with a take (publish, words, the listener's own data) is shared code (take-session.test.ts); the recorder answers codes.
@MainActor final class TakeSessionTests: XCTestCase {
    var rec = FakeRecorder(), gate = FakeGate(), prev = FakePreview(), clock = Date(timeIntervalSince1970: 1000)
    var dir: URL!
    var mic = true, responds: TakeResponds?
    var words = "A neighbour speaking."
    let limits = TakeLimits(minSeconds: 3, maxSeconds: 59.5, maxBytes: 512 * 1024)

    override func setUp() { dir = FileManager.default.temporaryDirectory.appendingPathComponent("takes-\(UUID().uuidString)") }
    override func tearDown() { try? FileManager.default.removeItem(at: dir) }
    func session() -> TakeSession {
        TakeSession(recorder: rec, transcriber: FakeTranscriber(words: words), gate: gate, preview: prev, micAllowed: { [unowned self] in self.mic },
                    responds: { [unowned self] in self.responds }, dir: dir, now: { [unowned self] in self.clock })
    }
    func files() -> [String] { (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? [] }

    func testR1RecordingNeedsTheMicAndYieldsTheRadioOnceUntilTheTakeEnds() async {
        mic = false
        var s = session(); await s.start(limits)
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.ended, "mic"); XCTAssertEqual(gate.yielded, 0)
        mic = true; s = session()
        await s.start(limits); XCTAssertEqual(s.state, .recording); XCTAssertEqual(gate.yielded, 1); XCTAssertEqual(s.ended, "")
        clock += 10; await s.stop(); XCTAssertEqual(s.state, .review); XCTAssertEqual(gate.given, 0, "the radio stays quiet through the review")
        await s.start(limits); XCTAssertEqual(gate.yielded, 1, "re-record does not yield again"); XCTAssertEqual(s.state, .recording)
        s.discard(); XCTAssertEqual(gate.given, 1); XCTAssertEqual(s.state, .idle)
    }

    func testR1ARefusedGateStartsNothing() async {
        gate.refuse = "another app is using the audio"
        let s = session(); await s.start(limits)
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.ended, "gate"); XCTAssertEqual(s.detail, "another app is using the audio"); XCTAssertTrue(rec.started.isEmpty)
    }

    func testR1ARecorderThatCannotOpenGivesTheRadioBack() async {
        rec.failStart = true
        let s = session(); await s.start(limits)
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.ended, "open"); XCTAssertEqual(gate.given, 1)
    }

    func testR1DoneAfterAPublishDropsTheFileAndGivesTheRadioBackOnce() async {
        let s = session(); await s.start(limits); clock += 10; await s.stop()
        XCTAssertNotNil(s.audio); XCTAssertEqual(files().count, 1)
        s.done(); XCTAssertEqual(s.state, .idle); XCTAssertEqual(gate.given, 1); XCTAssertEqual(files(), []); XCTAssertNil(s.audio)
        s.done(); XCTAssertEqual(gate.given, 1, "given back once")
    }

    func testR2TooShortIsNotKept() async {
        rec.seconds = 1.5
        let s = session(); await s.start(limits); clock += 1.5; await s.stop()
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.ended, "short"); XCTAssertEqual(files(), []); XCTAssertEqual(gate.given, 1)
    }

    func testR2TheCeilingStopsItHalfASecondEarlyAndTheTakeIsKept() async {
        rec.seconds = 59.5
        let s = session(); await s.start(limits)
        clock += 59.4; await s.tick(); XCTAssertEqual(s.state, .recording)
        clock += 0.1; await s.tick()
        XCTAssertEqual(s.state, .review, "auto-stop fires at 59.5 s elapsed"); XCTAssertEqual(s.ended, "ceiling"); XCTAssertEqual(s.transcript, "A neighbour speaking."); XCTAssertEqual(gate.given, 0)
    }

    func testR2TheLimitsAreTheOnesTheSharedCodeSent() async {
        rec.seconds = 5
        let s = session(); await s.start(TakeLimits(minSeconds: 6, maxSeconds: 20, maxBytes: 100)); clock += 5; await s.stop()
        XCTAssertEqual(s.ended, "short", "a take under the asked minimum is not kept")
        rec.seconds = 30; rec.bytes = 101
        await s.start(TakeLimits(minSeconds: 3, maxSeconds: 20, maxBytes: 100)); clock += 20; await s.tick()
        XCTAssertEqual(s.ended, "large"); XCTAssertEqual(files(), [])
    }

    func testR2NothingRecordedAndTooLarge() async {
        rec.bytes = 0
        var s = session(); await s.start(limits); clock += 10; await s.stop()
        XCTAssertEqual(s.ended, "empty"); XCTAssertEqual(s.state, .idle)
        rec = FakeRecorder(); rec.bytes = limits.maxBytes + 1
        s = session(); await s.start(limits); clock += 10; await s.stop()
        XCTAssertEqual(s.ended, "large"); XCTAssertEqual(files(), [])
    }

    func testR3ReRecordDeletesThePreviousFileAndDiscardDeletesTheFile() async {
        let s = session(); await s.start(limits); clock += 10; await s.stop()
        let first = files(); XCTAssertEqual(first.count, 1)
        await s.start(limits); XCTAssertEqual(files().count, 1); XCTAssertNotEqual(files(), first, "the old file is gone, a new one is recording")
        clock += 10; await s.stop(); s.discard(); XCTAssertEqual(files(), [])
    }

    func testAnEmptyTranscriptStaysInReviewForTheSharedCodeToSayNothingWasRecognised() async {
        words = "  "
        let s = session(); await s.start(limits); clock += 10; await s.stop()
        XCTAssertEqual(s.state, .review); XCTAssertEqual(s.transcript, "")
    }

    func testR6TheEntryOnAirIsRespondsToAndNoEntryIsNone() async {
        responds = TakeResponds(route: "harbor-fees", title: "Harbor fees rise")
        var s = session(); await s.start(limits)
        XCTAssertEqual(s.snapshot["respondsRoute"] as? String, "harbor-fees"); XCTAssertEqual(s.snapshot["respondsTitle"] as? String, "Harbor fees rise")
        responds = nil
        s = session(); await s.start(limits); XCTAssertNil(s.respondsTo); XCTAssertEqual(s.snapshot["respondsRoute"] as? String, "")
    }

    func testThePreviewToggles() async {
        let s = session(); await s.start(limits)
        s.setPreview(true); XCTAssertFalse(s.previewing, "only a take in review is played")
        clock += 10; await s.stop()
        s.togglePreview(); XCTAssertTrue(s.previewing); s.togglePreview(); XCTAssertFalse(s.previewing)
        s.setPreview(true); s.setPreview(false); XCTAssertFalse(s.previewing)
    }

    func testTheSnapshotIsTheSharedCodesNative() async {
        let s = session(); await s.start(limits); clock += 12
        var o = s.snapshot
        XCTAssertEqual(o["phase"] as? String, "recording"); XCTAssertEqual(o["elapsed"] as? Double, 12)
        await s.stop(); o = s.snapshot
        XCTAssertEqual(o["phase"] as? String, "review"); XCTAssertEqual(o["transcript"] as? String, "A neighbour speaking."); XCTAssertEqual(o["publishing"] as? Bool, false)
        s.setPublishing(true); XCTAssertEqual(s.snapshot["publishing"] as? Bool, true); s.done(); XCTAssertEqual(s.snapshot["publishing"] as? Bool, false, "done and discard clear it")
        for k in ["phase", "elapsed", "seconds", "transcript", "previewing", "publishing", "ended", "detail", "respondsTitle", "respondsRoute"] { XCTAssertNotNil(o[k], k) }
    }
}
