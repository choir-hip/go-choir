import XCTest
@testable import AutoradioKit

@MainActor final class FakeRecorder: TakeRecorderEngine {
    var seconds = 10.0, bytes = 4000, header: Double?, started: [URL] = [], failStart = false
    func start(to url: URL) throws { if failStart { throw LabError.unavailable("x") }; started.append(url); try? (header.map { Self.mp4(seconds: $0) } ?? Data(repeating: 1, count: bytes)).write(to: url) }
    static func mp4(seconds: Double) -> Data {
        func box(_ t: String, _ b: [UInt8]) -> [UInt8] { let n = UInt32(8 + b.count); return [UInt8(n >> 24), UInt8(n >> 16 & 255), UInt8(n >> 8 & 255), UInt8(n & 255)] + Array(t.utf8) + b }
        let d = UInt32(seconds * 1000)
        let mvhd = [UInt8](repeating: 0, count: 12) + [0, 0, 0x03, 0xE8] + [UInt8(d >> 24), UInt8(d >> 16 & 255), UInt8(d >> 8 & 255), UInt8(d & 255)] + [UInt8](repeating: 0, count: 80)
        return Data(box("ftyp", Array("M4A ".utf8) + [0, 0, 0, 0]) + box("moov", box("mvhd", mvhd)))
    }
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

/// Recording a take (failure list: TakeSession.swift, R1 to R7).
@MainActor final class TakeSessionTests: XCTestCase {
    var rec = FakeRecorder(), gate = FakeGate(), prev = FakePreview(), clock = Date(timeIntervalSince1970: 1000)
    var dir: URL!
    var own = TakeOwn()
    var mic = true, responds: TakeResponds?
    var words = "A neighbour speaking."
    var client: TakesClient?

    override func setUp() {
        dir = FileManager.default.temporaryDirectory.appendingPathComponent("takes-\(UUID().uuidString)")
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        client = TakesClient(client: ChoirClient(baseURL: "https://choir.news", keys: InMemoryKeyStore([KeyAccount.apiKey: "k", KeyAccount.computerID: "c"]), session: URLSession(configuration: cfg)))
        StubProtocol.seen = []
        StubProtocol.handler = { r in
            switch (r.method, r.path) {
            case ("POST", "/api/platform/takes"): return .init(json: ["take_id": "take-00000000000000c1", "publisher": "voice-me0001", "state": "live"])
            case ("GET", _): return .init(json: ["takes": []])
            default: return .init(status: 204)
            }
        }
    }
    override func tearDown() { try? FileManager.default.removeItem(at: dir) }
    func session() -> TakeSession {
        TakeSession(recorder: rec, transcriber: FakeTranscriber(words: words), gate: gate, preview: prev, client: { [unowned self] in self.client }, micAllowed: { [unowned self] in self.mic },
                    responds: { [unowned self] in self.responds }, own: own, dir: dir, now: { [unowned self] in self.clock })
    }
    func files() -> [String] { (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? [] }

    func testR1RecordingNeedsTheMicAndYieldsTheRadioOnceUntilTheTakeEnds() async {
        mic = false
        var s = session(); await s.start()
        XCTAssertEqual(s.state, .idle); XCTAssertTrue(s.notice.contains("microphone")); XCTAssertEqual(gate.yielded, 0)
        mic = true; s = session()
        await s.start(); XCTAssertEqual(s.state, .recording); XCTAssertEqual(gate.yielded, 1)
        clock += 10; await s.stop(); XCTAssertEqual(s.state, .review); XCTAssertEqual(gate.given, 0, "the radio stays quiet through the review")
        await s.start(); XCTAssertEqual(gate.yielded, 1, "re-record does not yield again"); XCTAssertEqual(s.state, .recording)
        s.discard(); XCTAssertEqual(gate.given, 1); XCTAssertEqual(s.state, .idle)
    }

    func testR1ARefusedGateStartsNothing() async {
        gate.refuse = "another app is using the audio"
        let s = session(); await s.start()
        XCTAssertEqual(s.state, .idle); XCTAssertTrue(s.notice.contains("another app")); XCTAssertTrue(rec.started.isEmpty)
    }

    func testR1ARecorderThatCannotOpenGivesTheRadioBack() async {
        rec.failStart = true
        let s = session(); await s.start()
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(gate.given, 1)
    }

    func testR2TooShortIsNotKept() async {
        rec.seconds = 1.5
        let s = session(); await s.start(); clock += 1.5; await s.stop()
        XCTAssertEqual(s.state, .idle); XCTAssertTrue(s.notice.contains("at least 3")); XCTAssertEqual(files(), []); XCTAssertEqual(gate.given, 1)
    }

    func testR2TheCeilingStopsItAndSaysSo() async {
        rec.seconds = 60
        let s = session(); await s.start()
        clock += 30; await s.tick(); XCTAssertEqual(s.state, .recording)
        clock += 30; await s.tick()
        XCTAssertEqual(s.state, .review); XCTAssertTrue(s.notice.contains("60 seconds"))
    }

    func testTheCeilingStopsHalfASecondEarlyAndAHeaderOf60Point3ClaimsExactly60() async throws {
        rec.seconds = 59.5; rec.header = 60.3
        var claimed = ""
        StubProtocol.handler = { r in
            if r.method == "POST" { claimed = String(decoding: r.raw, as: UTF8.self); return .init(status: 201, json: ["take_id": "take-00000000000000c1", "publisher": "voice-me0001", "state": "live"]) }
            return .init(json: ["takes": []])
        }
        let s = session(); await s.start()
        clock += 59.4; await s.tick(); XCTAssertEqual(s.state, .recording)
        clock += 0.1; await s.tick()
        XCTAssertEqual(s.state, .review, "auto-stop fires at 59.5 s elapsed"); XCTAssertTrue(s.notice.contains("Stopped at 60 seconds")); XCTAssertEqual(s.duration, 60.0)
        await s.publish()
        XCTAssertTrue(claimed.contains("name=\"duration_seconds\"\r\n\r\n60.00\r\n"), "the claim is exactly 60.00")
        XCTAssertEqual(s.state, .idle)
    }

    func testAFileUnder3SecondsByItsHeaderIsNotKept() async {
        rec.seconds = 3.2; rec.header = 2.9
        let s = session(); await s.start(); clock += 3.2; await s.stop()
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(files(), [])
    }

    func testR2TooLargeIsNotKept() async {
        rec.bytes = TakeBounds.maxBytes + 1
        let s = session(); await s.start(); clock += 10; await s.stop()
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(files(), [])
    }

    func testR3ReRecordDeletesThePreviousFileAndDiscardDeletesTheFile() async {
        let s = session(); await s.start(); clock += 10; await s.stop()
        let first = files(); XCTAssertEqual(first.count, 1)
        await s.start(); XCTAssertEqual(files().count, 1); XCTAssertNotEqual(files(), first, "the old file is gone, a new one is recording")
        clock += 10; await s.stop(); s.discard(); XCTAssertEqual(files(), [])
        XCTAssertTrue(StubProtocol.seen.isEmpty, "nothing left the phone")
    }

    func testR4AnEmptyTranscriptCannotBePublished() async {
        words = "  "
        let s = session(); await s.start(); clock += 10; await s.stop()
        XCTAssertEqual(s.state, .review); XCTAssertFalse(s.canPublish); XCTAssertTrue(s.notice.contains("Record it again"))
        await s.publish(); XCTAssertTrue(StubProtocol.seen.isEmpty)
    }

    func testR5PublishIsOneMultipartPostAndRemembersIt() async {
        responds = TakeResponds(route: "harbor-fees", title: "Harbor fees rise")
        let s = session(); await s.start(); clock += 10; await s.stop()
        XCTAssertEqual(s.respondsTo?.route, "harbor-fees")
        await s.publish()
        XCTAssertEqual(StubProtocol.seen.prefix(2).map { $0.method + " " + $0.path }, ["POST /api/platform/takes", "GET /api/platform/takes"])
        XCTAssertEqual(s.published?.takeID, "take-00000000000000c1")
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.published?.publisher, "voice-me0001"); XCTAssertEqual(gate.given, 1); XCTAssertEqual(files(), [])
        XCTAssertEqual(own.handle, "voice-me0001")
        let raw = String(decoding: StubProtocol.seen[0].raw, as: UTF8.self)
        XCTAssertTrue(raw.contains("harbor-fees"))
    }

    func testR5AFailedPublishKeepsTheRecordingAndARetryPublishesOnce() async {
        var fail = true
        StubProtocol.handler = { r in
            if r.path == "/api/platform/takes", r.method == "POST" { return fail ? .init(status: 500) : .init(status: 201, json: ["take_id": "take-00000000000000c1", "publisher": "voice-me0001", "state": "live"]) }
            return .init(json: ["takes": []])
        }
        let s = session(); await s.start(); clock += 10; await s.stop()
        await s.publish(); XCTAssertEqual(s.state, .review); XCTAssertTrue(s.notice.contains("Could not publish")); XCTAssertTrue(s.notice.contains("kept")); XCTAssertEqual(gate.given, 0)
        XCTAssertEqual(files().count, 1, "the recording is still on the phone")
        fail = false; await s.publish()
        XCTAssertEqual(StubProtocol.seen.filter { $0.method == "POST" && $0.path == "/api/platform/takes" }.count, 2)
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(own.handle, "voice-me0001")
    }

    func testAServerNotDeployedYetKeepsTheTakeAndSaysSo() async {
        StubProtocol.handler = { _ in .init(status: 404) }
        let s = session(); await s.start(); clock += 10; await s.stop()
        await s.publish()
        XCTAssertEqual(s.state, .review); XCTAssertEqual(s.notice, "Takes are not available yet. Your recording is kept; publish it again later.")
        XCTAssertEqual(files().count, 1); XCTAssertFalse(s.transcript.isEmpty)
        await s.refresh(); XCTAssertEqual(s.notice, "Takes are not available yet.")
    }

    func testTheDailyLimitIsShownInTheServersWords() async {
        StubProtocol.handler = { _ in .init(status: 429, json: ["error": "invalid take: 10 takes in the last 24 hours, the limit is 10"]) }
        let s = session(); await s.start(); clock += 10; await s.stop(); await s.publish()
        XCTAssertTrue(s.notice.hasPrefix("invalid take: 10 takes in the last 24 hours, the limit is 10")); XCTAssertEqual(s.state, .review)
    }

    func testR6NoEntryOnAirMeansNoRespondsTo() async {
        responds = nil
        let s = session(); await s.start(); XCTAssertNil(s.respondsTo)
    }

    func testReviewPreviewTogglesAndPublishStopsIt() async {
        let s = session(); await s.start(); clock += 10; await s.stop()
        s.togglePreview(); XCTAssertTrue(s.previewing); s.togglePreview(); XCTAssertFalse(s.previewing)
        s.togglePreview(); await s.publish(); XCTAssertFalse(s.previewing)
    }

    func testDeleteCallsTheClientAndDropsTheRow() async {
        StubProtocol.handler = { r in r.method == "GET" ? .init(json: ["takes": [["take_id": "take-00000000000000a5", "duration_seconds": 9, "transcript": "x", "published": true, "publisher": "voice-me0001"]]]) : .init(status: 204) }
        let s = session(); await s.refresh(); XCTAssertEqual(s.mine.count, 1)
        await s.delete("take-00000000000000a5"); XCTAssertEqual(s.mine.count, 0)
    }

    /// The on-air Take control (radio-take-start) sends the same `record` action as the Takes section: one flow. From idle it cites the on-air entry,
    /// the model shows the panel's states in turn (recording, review with a transcript and Publish), and a take with nothing on air cites nothing.
    func testTheOnAirTakeControlStartsTheSameFlowAndCitesTheEntryOnAir() async {
        let s = session()
        var p = TakesPage.make(session: s, mutes: [], own: own)
        XCTAssertEqual(p.state, "idle"); XCTAssertTrue(p.canRecord, "the Take control is offered when nothing is under way")
        responds = TakeResponds(route: "harbor-fees", title: "Harbor fees rise")
        await s.start(); clock += 5
        p = TakesPage.make(session: s, mutes: [], own: own)
        XCTAssertEqual(p.state, "recording"); XCTAssertFalse(p.canRecord, "the control gives way to Stop"); XCTAssertTrue(p.canStop)
        XCTAssertEqual(p.respondsTo, "Answering: Harbor fees rise"); XCTAssertEqual(s.respondsTo?.route, "harbor-fees")
        await s.stop()
        p = TakesPage.make(session: s, mutes: [], own: own)
        XCTAssertEqual(p.state, "review"); XCTAssertEqual(p.transcript, "A neighbour speaking."); XCTAssertTrue(p.canPublish); XCTAssertTrue(p.canDiscard)
        s.discard()
        responds = nil
        let t = session(); await t.start()
        XCTAssertNil(t.respondsTo); XCTAssertEqual(TakesPage.make(session: t, mutes: [], own: own).respondsTo, "")
    }

    func testPageModelFollowsTheState() async {
        let s = session(); var p = TakesPage.make(session: s, mutes: ["voice-1"], own: own)
        XCTAssertTrue(p.canRecord); XCTAssertFalse(p.canStop); XCTAssertEqual(p.muted.first?.line, "Never play takes by voice-1.")
        responds = TakeResponds(route: "r", title: "Harbor fees rise")
        await s.start(); clock += 12
        p = TakesPage.make(session: s, mutes: [], own: own)
        XCTAssertTrue(p.canStop); XCTAssertEqual(p.elapsed, "0:12"); XCTAssertEqual(p.limit, "1:00"); XCTAssertEqual(p.respondsTo, "Answering: Harbor fees rise")
        await s.stop()
        p = TakesPage.make(session: s, mutes: [], own: own)
        XCTAssertTrue(p.canPublish); XCTAssertEqual(p.publishLine, TakeBounds.publishLine); XCTAssertEqual(p.transcript, "A neighbour speaking."); XCTAssertTrue(p.canDiscard)
    }
}

/// A take on the run sheet: played like a clip entry, reported through the same ops (station L7, part 4).
@MainActor final class TakePlaybackTests: XCTestCase {
    func entry() -> RunSheetEntry {
        RunSheetEntry.take(Take(takeID: "take-00000000000000a1", publisher: "voice-3fa2c1", publishedAt: nil, duration: 14, transcript: "Words.", respondsTo: nil, record: nil, audioPath: "", state: "live"),
                           leadIn: "A neighbour.", id: "d-t1", forecast: RunSheetForecast(pHeard: 0.6, causedBy: "c", claimId: "k"))
    }
    func testALeadInThenTheFetchedAudioAndAFinishedOutcome() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("take-play-\(UUID().uuidString).m4a"); try? Data([1]).write(to: f)
        p.takeAudio = { id in id == "take-00000000000000a1" ? f : nil }
        p.playNow(entry())
        _ = await poll { e.log.contains("speak:d-t1") }
        XCTAssertTrue(e.log.contains("speak:d-t1"), "the lead-in is spoken first")
        p.documentEnded(id: "rs:d-t1", anyPlayed: true)
        _ = await poll { e.log.contains { $0.hasPrefix("remote:") } }
        XCTAssertTrue(e.log.contains { $0.hasPrefix("remote:take-play-") }, "then the publisher's audio plays")
        p.remoteEnded(generation: p.generation)
        XCTAssertEqual(s.sheet.entries.first { $0.id == "d-t1" }?.ended?.outcome, .finished)
        XCTAssertFalse(FileManager.default.fileExists(atPath: f.path), "the fetched file is deleted when the entry ends")
    }
    func testATakeThatCannotBeFetchedFailsOnceAndTheRadioGoesOn() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        p.takeAudio = { _ in nil }
        p.playNow(entry())
        _ = await poll { s.sheet.entries.first { $0.id == "d-t1" }?.ended != nil }
        XCTAssertEqual(s.sheet.entries.first { $0.id == "d-t1" }?.ended?.outcome, .failed)
    }
    func testThePageShowsThePublisherTheTranscriptAndAMuteHandle() {
        var sheet = RunSheet(); var e = entry(); e.state = .onAir
        sheet.entries = [e]; sheet.onAir = e.id
        var i = RadioPageInput(sheet: sheet)
        let t = Take(takeID: "take-00000000000000a1", publisher: "voice-3fa2c1", publishedAt: nil, duration: 14, transcript: "Words.", respondsTo: nil, record: TakeRecordSummary(), audioPath: "", state: "live")
        i.takeLookup = { $0 == t.takeID ? t : nil }
        let o = RadioPage.make(i)
        XCTAssertEqual(o.kindLabel, "Take"); XCTAssertEqual(o.takePublisher, "voice-3fa2c1"); XCTAssertEqual(o.takeTranscript, "Words."); XCTAssertEqual(o.takeRecord, "new: no outcomes yet")
        XCTAssertEqual(o.source, "voice-3fa2c1"); XCTAssertTrue(o.canWhy); XCTAssertTrue(o.forecast.contains("60 in 100"))
    }
}
