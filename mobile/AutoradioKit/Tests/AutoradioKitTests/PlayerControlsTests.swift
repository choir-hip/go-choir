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
        var snap = LabSnapshot(); snap.running = true; snap.blockCount = 1
        let r = tr.radioInfo(player: ChoirPlayerView(), snapshot: snap, document: ProgrammeDocument(id: "d", title: "T", blocks: [.speech(text: "One.")]), route: "r")
        XCTAssertEqual(r.speedValue, 1.25); XCTAssertEqual(r.speed, "1.25")
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

// MARK: The DJ's candidates

final class StretchCandidatesTests: XCTestCase {
    func row(_ route: String, title: String = "Title", dek: String = "A dek.", importance: Int = 5, extra: [String: Any] = [:]) -> [String: Any] {
        var o: [String: Any] = ["title": title, "dek": dek, "route": ["bare": route], "importance": importance, "latest_revision": 3, "version_count": 9]
        for (k, v) in extra { o[k] = v }
        return o
    }
    func testP19_OrderedByTheWeightAndStableAndTheVersionIsTheLatestRevision() {
        let c = StretchCandidates.fromFront(["stories": [row("low", importance: 2), row("high", importance: 9), row("mid-a", importance: 5), row("mid-b", importance: 5)]])
        XCTAssertEqual(c.map(\.route), ["high", "mid-a", "mid-b", "low"])
        XCTAssertEqual(c.first?.version, 3); XCTAssertEqual(c.first?.ref, "pub:high@v3")
    }
    func testP20_AnythingTheServerWouldRefuseIsShapedOrDropped() {
        let long = String(repeating: "é", count: 400)                                   // 800 bytes
        let rows = [row("ok", title: long, dek: String(repeating: "d", count: 2000)), row("nodek", dek: "  "), row("a b"), row("a@b"), row("a#b"), row(""),
                    row("dup"), row("dup"), row("two\nlines", title: "One\nline here"), row("notitle", title: " ")]
        let c = StretchCandidates.fromFront(["stories": rows])
        XCTAssertEqual(c.map(\.route), ["ok", "dup"])
        XCTAssertLessThanOrEqual(c[0].title.utf8.count, 300); XCTAssertLessThanOrEqual(c[0].dek.utf8.count, 1000)
        XCTAssertTrue(c[0].title.allSatisfy { $0 == "é" }, "cut on a character, not a byte")
        let many = StretchCandidates.fromFront(["stories": (0..<55).map { row("r\($0)") }])
        XCTAssertEqual(many.count, 40, "at most 40")
    }
    func testP20_TextOnlyOnTheFirstTenAndNeverOver8KB() {
        let items = StretchCandidates.fromFront(["stories": (0..<14).map { row("r\($0)", importance: 10 - ($0 / 2)) }])
        var texts: [String: String] = [:]
        for c in items { texts[c.route] = String(repeating: "word ", count: 5000) + "\n\nSecond paragraph." }
        let body = StretchCandidates.body(items, texts: texts)
        XCTAssertEqual(body.count, 14)
        XCTAssertEqual(body.prefix(10).filter { $0["text"] != nil }.count, 10)
        XCTAssertEqual(body.suffix(4).filter { $0["text"] != nil }.count, 0)
        for b in body.prefix(10) { XCTAssertLessThanOrEqual((b["text"] as? String ?? "").utf8.count, 8 * 1024) }
        // a short story keeps its paragraphs and loses its markup
        let one = StretchCandidates.body([items[0]], texts: [items[0].route: "# Heading\n\nFirst **bold** [link](http://x).\n\nSecond."])
        XCTAssertEqual(one[0]["text"] as? String, "Heading\n\nFirst bold link.\n\nSecond.")
        // a candidate with no text still goes
        XCTAssertNil(StretchCandidates.body([items[0]], texts: [:])[0]["text"])
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

/// A programme route with no stretch (404): Play falls to the story plain, and asks for nothing else.
struct NoStretch: StretchSource {
    func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { throw ChoirError.empty("no stretch") }
}
