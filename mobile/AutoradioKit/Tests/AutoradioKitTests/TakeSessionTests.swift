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
            case ("POST", "/api/autoradio/takes"): return .init(json: ["take_id": "take-00000000000000c1"])
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

    func testR5PublishUploadsThenPublishesAndRemembersIt() async {
        responds = TakeResponds(route: "harbor-fees", title: "Harbor fees rise")
        let s = session(); await s.start(); clock += 10; await s.stop()
        XCTAssertEqual(s.respondsTo?.route, "harbor-fees")
        await s.publish()
        XCTAssertEqual(StubProtocol.seen.prefix(2).map { $0.method + " " + $0.path }, ["POST /api/autoradio/takes", "POST /api/platform/takes"])
        XCTAssertEqual(StubProtocol.seen[1].body["take_id"] as? String, "take-00000000000000c1")
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.published?.publisher, "voice-me0001"); XCTAssertEqual(gate.given, 1); XCTAssertEqual(files(), [])
        XCTAssertEqual(own.handle, "voice-me0001")
        let raw = String(decoding: StubProtocol.seen[0].raw, as: UTF8.self)
        XCTAssertTrue(raw.contains("harbor-fees"))
    }

    func testR5AFailureAfterTheUploadKeepsTheIdSoARetryDoesNotUploadTwice() async {
        var fail = true
        StubProtocol.handler = { r in
            if r.path == "/api/autoradio/takes", r.method == "POST" { return .init(json: ["take_id": "take-00000000000000c1"]) }
            if r.path == "/api/platform/takes", r.method == "POST" { return fail ? .init(status: 500) : .init(json: ["take_id": "take-00000000000000c1", "publisher": "voice-me0001"]) }
            return .init(json: ["takes": []])
        }
        let s = session(); await s.start(); clock += 10; await s.stop()
        await s.publish(); XCTAssertEqual(s.state, .review); XCTAssertTrue(s.notice.contains("Could not publish")); XCTAssertEqual(gate.given, 0)
        fail = false; await s.publish()
        XCTAssertEqual(StubProtocol.seen.filter { $0.method == "POST" && $0.path == "/api/autoradio/takes" }.count, 1)
        XCTAssertEqual(s.state, .idle)
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

/// The demo routes answer the whole take path from fixtures (the tour's server).
final class TakesDemoTests: XCTestCase {
    func testDemoPublishListDelete() throws {
        let up = TakesDemo.answer(method: "POST", path: "/api/autoradio/takes", body: Data("--B\r\nContent-Disposition: form-data; name=\"transcript\"\r\n\r\nhello demo\r\n--B--".utf8))
        let id = try XCTUnwrap(((try JSONSerialization.jsonObject(with: try XCTUnwrap(up).1)) as? [String: Any])?["take_id"] as? String)
        let pub = TakesDemo.answer(method: "POST", path: "/api/platform/takes", body: Data("{\"take_id\":\"\(id)\"}".utf8))
        XCTAssertEqual(pub?.0, 200)
        let mine = try JSONSerialization.jsonObject(with: try XCTUnwrap(TakesDemo.answer(method: "GET", path: "/api/autoradio/takes", body: Data())).1) as? [String: Any]
        XCTAssertTrue((mine?["takes"] as? [[String: Any]])?.contains { $0["transcript"] as? String == "hello demo" && $0["published"] as? Bool == true } ?? false)
        _ = TakesDemo.answer(method: "DELETE", path: "/api/autoradio/takes/\(id)", body: Data())
        XCTAssertFalse(String(decoding: try XCTUnwrap(TakesDemo.answer(method: "GET", path: "/api/autoradio/takes", body: Data())).1, as: UTF8.self).contains(id))
    }
}
