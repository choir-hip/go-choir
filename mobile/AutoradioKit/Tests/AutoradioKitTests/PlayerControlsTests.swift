import XCTest
@testable import AutoradioKit

// Player controls (owner QA round 1, 2026-11-15): speed to 3x, next and previous story, autoplay, the DJ's candidates.
// The failure list is in mobile/README.md ("Player controls"); the numbers in the test names (P1 and so on) are its numbers.

// MARK: Speed

final class PlaybackSpeedTests: XCTestCase {
    func testP1_RangeIsHalfToThreeAndGarbageIsOne() {
        XCTAssertEqual(PlaybackSpeed.clamp(9), 3); XCTAssertEqual(PlaybackSpeed.clamp(0.01), 0.5); XCTAssertEqual(PlaybackSpeed.clamp(2.5), 2.5)
        XCTAssertEqual(PlaybackSpeed.clamp(.nan), 1); XCTAssertEqual(PlaybackSpeed.clamp(.infinity), 1)
    }
    func testP2_ASpeedOffTheStepSnapsToTheStep() {
        XCTAssertEqual(PlaybackSpeed.snap(1.2345), 1.25); XCTAssertEqual(PlaybackSpeed.snap(1.02), 1); XCTAssertEqual(PlaybackSpeed.snap(2.99), 3)
        XCTAssertEqual(PlaybackSpeed.snap(0.7), 0.7); XCTAssertEqual(PlaybackSpeed.snap(1.15), 1.15)
        XCTAssertEqual(PlaybackSpeed.name(1), "1"); XCTAssertEqual(PlaybackSpeed.name(1.25), "1.25"); XCTAssertEqual(PlaybackSpeed.name(2.5), "2.5")
        XCTAssertEqual(PlaybackSpeed.name(3), "3"); XCTAssertEqual(PlaybackSpeed.name(1.05), "1.05"); XCTAssertEqual(PlaybackSpeed.name(7), "3")
    }
    func suite() -> UserDefaults { let n = "speed-\(UUID().uuidString)"; let d = UserDefaults(suiteName: n)!; d.removePersistentDomain(forName: n); return d }
    func testP5_TheChosenSpeedIsKeptAndAGarbageOneIsIgnored() {
        let d = suite(); let s = SpeedStore(defaults: d)
        XCTAssertNil(s.load(), "nothing saved")
        s.save(2.75); XCTAssertEqual(SpeedStore(defaults: d).load(), 2.75, "a new store on the same defaults reads it")
        s.save(9); XCTAssertEqual(s.load(), 3, "saved clamped")
        for bad in [0.0, -1, 99, Double.nan] as [Double] { d.set(bad, forKey: "autoradio.playbackSpeed"); XCTAssertNil(s.load(), "\(bad)") }
        d.set("fast", forKey: "autoradio.playbackSpeed"); XCTAssertNil(s.load())
    }
    func testP1_TheTrackerAndThePlayerAndTheAudioAgreeOnTheCeiling() {
        let tr = BarTracker(); let got = Locked<[Double]>([])
        tr.speedSink = { v in got.write { $0.append(v) } }
        tr.setSpeed(3); tr.setSpeed(9); tr.setSpeed(1.2345)
        XCTAssertEqual(got.read { $0 }, [3, 3, 1.25])
        let a = virtualAudio(); a.setProgrammeRate(3)
        XCTAssertEqual(a.programmeRate, 3); XCTAssertEqual(a.timePitch.rate, 3, "the time-pitch unit takes it (its range is 1/32 to 32)")
        a.setProgrammeRate(.nan); XCTAssertEqual(a.programmeRate, 1)
        var info = RadioInfo(); info.speedValue = 3
        XCTAssertEqual(info.speedValue, 3)
        let r = tr.radioInfo(level: "play")
        XCTAssertEqual(r.speedValue, 1.25)
    }
    func testP4_ThreeTimesIsThreeTimesFasterForAnyVoice() async {
        let a = virtualAudio(); a.programmePlay()
        a.setProgrammeRate(3)
        let t0 = Date(); await a.playProgramme([Float](repeating: 0, count: 48_000), rate: 16_000, tag: 1)       // 3 s of source audio
        let took = Date().timeIntervalSince(t0)
        XCTAssertLessThan(took, 1.5); XCTAssertGreaterThan(took, 0.8, "3 s at 3x is about 1 s")
    }
    func testP3_ChangingTheSpeedMidSentenceNeverMovesThePositionBack() async {
        let a = virtualAudio(); a.programmePlay(); a.setProgrammeRate(1)
        let task = Task { await a.playProgramme([Float](repeating: 0, count: 48_000), rate: 16_000, tag: 2) }
        try? await Task.sleep(nanoseconds: 400_000_000)
        let before = a.programmePosition(tag: 2)?.seconds ?? -1
        a.setProgrammeRate(3)
        try? await Task.sleep(nanoseconds: 100_000_000)
        let after = a.programmePosition(tag: 2)?.seconds ?? -1
        XCTAssertGreaterThanOrEqual(after, before, "the position holds across the change")
        XCTAssertGreaterThan(before, 0.2)
        let t0 = Date(); await task.value
        XCTAssertLessThan(Date().timeIntervalSince(t0), 1.0, "the rest of the 3 s block goes at 3x, not at 1x")
    }
}

// MARK: Previous, and the paper stack

final class StoryCursorTests: XCTestCase {
    func testP8_PreviousRestartsAfterThreeSecondsOrPastTheFirstBlock() {
        XCTAssertFalse(StoryCursor.previousRestartsCurrent(blockIndex: 0, secondsIntoBlock: 0))
        XCTAssertFalse(StoryCursor.previousRestartsCurrent(blockIndex: 0, secondsIntoBlock: 2.9))
        XCTAssertTrue(StoryCursor.previousRestartsCurrent(blockIndex: 0, secondsIntoBlock: 3))
        XCTAssertTrue(StoryCursor.previousRestartsCurrent(blockIndex: 1, secondsIntoBlock: 0))
    }
}

// MARK: The request and the app

final class PlayerAppTests: XCTestCase {
    var env: ChoirEnvironment!
    let failing = Locked<Set<String>>([])
    var programmeCalls: [StubProtocol.Req] { StubProtocol.seen.filter { $0.path == "/api/autoradio/programme" } }

    override func setUp() {
        env = ChoirEnvironment.shared
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock()); env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        failing.write { $0 = [] }
        StubProtocol.handler = { [self] req in front(req) }
    }
    override func tearDown() { env.session = .shared; LabEnvironment.virtualAudio = false; LabEnvironment.promptsAllowed = true }

    func front(_ req: StubProtocol.Req) -> StubProtocol.Resp {
        switch req.path {
        case "/api/platform/publications/front":
            return .init(json: ["stories": ["a", "b", "c"].enumerated().map { i, r in
                ["title": "Story \(r)", "dek": "Dek \(r).", "route": ["bare": r], "section": "world", "words": 40, "importance": 9 - i] as [String: Any] }])
        case "/api/platform/publications/resolve":
            let r = req.query["route"] ?? ""
            if failing.read({ $0.contains(r) }) { return .init(status: 404, json: ["error": "gone"]) }
            return .init(json: ["publication": ["title": "Story \(r)"], "version": ["id": "pv-\(r)", "manifest_hash": "mh"], "payload": ["text": "Text of \(r)."]])
        default: return .init(status: 404, json: ["error": "nope"])
        }
    }

    final class EmptyVoice: Voice, @unchecked Sendable {
        static let stageID = "empty-voice"
        static var descriptor: StageDescriptor { StageDescriptor(displayName: "empty voice") }
        init(_ p: StageParams) {}
        func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> { AsyncThrowingStream { $0.finish() } }
    }


    func resolved() -> [String] { StubProtocol.seen.filter { $0.path == "/api/platform/publications/resolve" }.compactMap { $0.query["route"] } }
}

