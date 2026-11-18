import XCTest
@testable import AutoradioKit

/// Takes, the pure and wire parts (failure list: Takes.swift, T1 to T7).
final class TakesTests: XCTestCase {
    static let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    func fixture(_ n: String) throws -> Data { try Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/takes/\(n)")) }
    func list() throws -> [Take] { TakesClient.takes(try JSONSerialization.jsonObject(with: fixture("list.json"))) }

    func testT1BoundsAreTheSpecs() {
        XCTAssertEqual(TakeBounds.minSeconds, 3); XCTAssertEqual(TakeBounds.maxSeconds, 60)
        XCTAssertEqual(TakeBounds.maxBytes, 512 * 1024); XCTAssertEqual(TakeBounds.maxTranscriptBytes, 2000)
        XCTAssertEqual(TakeBounds.sampleRate, 24_000); XCTAssertEqual(TakeBounds.bitRate, 32_000)
    }

    func testT2ListKeepsLiveTakesWithAnIdAndAHandle() throws {
        let t = try list()
        XCTAssertEqual(t.map(\.takeID), ["take-00000000000000a1", "take-00000000000000a2", "take-00000000000000a3", "take-00000000000000a5"], "deleted and id-less rows are not takes")
        XCTAssertEqual(t[0].title, "The harbor vote is tonight and nobody has said what the fee is for.", "the title is the first line")
        XCTAssertEqual(t[0].respondsTo, TakeResponds(route: "harbor-fees", title: "Harbor fees rise"))
        XCTAssertEqual(t[0].record?.words, "heard through by 7 of 11 listeners over 5 takes")
        XCTAssertEqual(t[1].record?.words, "new: no outcomes yet")
        XCTAssertEqual((t[2].record ?? TakeRecordSummary()).words, "new: no outcomes yet", "no record at all reads as new")
        XCTAssertEqual(t[0].audioPath, "/api/platform/takes/take-00000000000000a1/audio")
    }

    func testT2AnAudioAddressOffTheHostIsNotFollowedWithTheKey() {
        XCTAssertEqual(Take.pathOf("//evil.example/x"), nil)
        XCTAssertEqual(Take.pathOf("https://choir.news/api/platform/takes/x/audio"), "/api/platform/takes/x/audio")
    }

    func testT3CandidatesAreNotMineNotMutedNewestFirst() throws {
        let own = TakeOwn(); own.add(id: "x", handle: "voice-me0001")
        let mutes = TakeMutes(); mutes.mute("voice-9b0e44")
        let c = TakeCandidates.select(try list(), own: own, mutes: mutes)
        XCTAssertEqual(c.map(\.takeID), ["take-00000000000000a1", "take-00000000000000a3"], "not the listener's own (by handle), not a muted publisher's, newest first")
        let byID = TakeOwn(); byID.add(id: "take-00000000000000a1", handle: "")
        XCTAssertFalse(TakeCandidates.select(try list(), own: byID, mutes: TakeMutes()).contains { $0.takeID == "take-00000000000000a1" }, "the listener's own take by id")
    }

    func testT3AtMostThreeAndTheyCountTowardForty() throws {
        let many = (0..<6).map { i in Take(takeID: "take-\(i)", publisher: "voice-\(i)", publishedAt: Date(timeIntervalSince1970: Double(i)), duration: 5, transcript: "w\(i)", respondsTo: nil, record: nil, audioPath: "", state: "live") }
        let picked = TakeCandidates.select(many, own: TakeOwn(), mutes: TakeMutes())
        XCTAssertEqual(picked.count, 3); XCTAssertEqual(picked.first?.takeID, "take-5")
        let stories = (0..<40).map { ["route": "r\($0)"] as [String: Any] }
        let merged = TakeCandidates.merged(stories: stories, takes: picked)
        XCTAssertEqual(merged.count, 40); XCTAssertEqual(merged.filter { ($0["kind"] as? String) == "take" }.count, 3)
    }

    func testT3CandidateJSONHasTheSpecsFieldsAndNoAudioAddress() throws {
        let j = TakeCandidates.json(try list()[0])
        XCTAssertEqual(j["kind"] as? String, "take")
        for k in ["take_id", "publisher", "title", "transcript", "duration_seconds", "responds_to", "record"] { XCTAssertNotNil(j[k], k) }
        XCTAssertNil(j["audio_url"]); XCTAssertNil(j["audio_path"])
        XCTAssertLessThanOrEqual((j["title"] as! String).utf8.count, 120)
    }

    func testT4ATakeLineBecomesAnEntryWithItsCites() throws {
        let store = TakeStore(); store.replace(try list())
        var d = StretchDecoder()
        let evs = d.feed(String(decoding: try fixture("segment-take.sse"), as: UTF8.self)) + d.finish()
        guard case .segment(let seg)? = evs.first else { return XCTFail("no segment: \(evs)") }
        XCTAssertEqual(seg.kind, "take"); XCTAssertEqual(seg.takeID, "take-00000000000000a1"); XCTAssertEqual(seg.forecast?.pHeard, 0.6)
        let e = try XCTUnwrap(ProgrammeRouteFeed.entry(from: seg, clips: ClipStore(), takes: store))
        XCTAssertEqual(e.kind, .take); XCTAssertEqual(e.takeID, "take-00000000000000a1")
        XCTAssertEqual(e.source?.label, "voice-3fa2c1"); XCTAssertEqual(e.forecast?.pHeard, 0.6)
        XCTAssertEqual(e.text, "Here is a neighbour on the harbor fee.")
        XCTAssertEqual(e.cites?.map(\.ref), ["pub:harbor-fees"], "the story it answered is a Built on row")
        let rows = RadioPage.builtOn(e)
        XCTAssertEqual(rows.map(\.address), ["pub:harbor-fees"])
        XCTAssertEqual(RunSheetEntry.estimateSeconds(words: e.text!) + 14.2, e.estSeconds, accuracy: 0.01)
        XCTAssertNil(ProgrammeRouteFeed.entry(from: seg, clips: ClipStore(), takes: TakeStore()), "a take the phone does not hold is not played")
    }

    func testT4ATakeLineWithoutAForecastOrATakeIsDropped() {
        var d = StretchDecoder()
        _ = d.feed("{\"kind\":\"take\",\"text\":\"x\",\"take\":\"take-00000000000000a1\"}\n")
        _ = d.feed("{\"kind\":\"take\",\"text\":\"x\",\"p_heard\":0.5}\n")
        XCTAssertEqual(d.droppedCount, 2)
    }

    func testT4ATakeEntryRoundTripsTheSheetsJSON() throws {
        let e = RunSheetEntry.take(try list()[0], leadIn: "Here.", id: "d-t1", forecast: RunSheetForecast(pHeard: 0.5, causedBy: "c", claimId: "k"))
        let back = try JSONDecoder().decode(RunSheetEntry.self, from: try JSONEncoder().encode(e))
        XCTAssertEqual(back, e); XCTAssertEqual(back.kind, .take)
    }

    func testT5MultipartCarriesAudioTranscriptAndHash() throws {
        let audio = Data((0..<2000).map { UInt8($0 % 251) })
        let u = TakesClient.Upload(audio: audio, transcript: "hello there", duration: 12.5, respondsTo: TakeResponds(route: "harbor-fees", title: "Harbor"))
        let m = TakesClient.multipart(u, boundary: "B")
        XCTAssertEqual(m.contentType, "multipart/form-data; boundary=B")
        let s = String(decoding: m.body, as: UTF8.self)
        XCTAssertTrue(s.contains("name=\"transcript\"\r\n\r\nhello there"))
        XCTAssertTrue(s.contains("name=\"duration_seconds\"\r\n\r\n12.50"))
        XCTAssertTrue(s.contains("name=\"sha256\"\r\n\r\n" + u.sha256))
        XCTAssertTrue(s.contains("name=\"responds_to\"\r\n\r\n{\"route\":\"harbor-fees\",\"title\":\"Harbor\"}"))
        XCTAssertTrue(s.contains("filename=\"take.m4a\"\r\nContent-Type: audio/mp4"))
        XCTAssertNotNil(m.body.range(of: audio)); XCTAssertEqual(u.sha256.count, 64)
    }

    func testT6MutesAreKeptListedUndoneAndSurviveARelaunch() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("mutes-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let m = TakeMutes(file: f)
        XCTAssertTrue(m.mute("voice-3fa2c1")); XCTAssertFalse(m.mute("voice-3fa2c1"), "twice is one"); XCTAssertFalse(m.mute("bad handle"))
        XCTAssertEqual(TakeMutes(file: f).all, ["voice-3fa2c1"])
        XCTAssertTrue(m.unmute("voice-3fa2c1")); XCTAssertEqual(TakeMutes(file: f).all, [])
        XCTAssertEqual(TakeMutes.notebookLine("voice-3fa2c1"), "Never play takes by voice-3fa2c1.")
    }

    func testT6OwnIdsAndHandleSurviveARelaunch() {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("own-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        TakeOwn(file: f).add(id: "take-1", handle: "voice-me0001")
        XCTAssertEqual(TakeOwn(file: f).handle, "voice-me0001")
    }

    func testRespondsToFromTheEntryOnAir() async {
        await MainActor.run {
            var story = RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "Harbor fees rise", text: "x", estSeconds: 3)
            story.cites = [RunSheetCite(ref: "pub:harbor-fees@v2", role: .subject, label: "Harbor fees rise")]
            XCTAssertEqual(TakeResponds.from(entry: story), TakeResponds(route: "harbor-fees", title: "Harbor fees rise"))
            XCTAssertNil(TakeResponds.from(entry: RunSheetEntry(id: "o", kind: .opening, placedBy: .phone, title: "Opening", text: "x", estSeconds: 3)))
            XCTAssertNil(TakeResponds.from(entry: nil))
            XCTAssertNil(TakeResponds.from(entry: RunSheetEntry.take(Take(takeID: "take-1", publisher: "voice-1", publishedAt: nil, duration: 5, transcript: "t", respondsTo: TakeResponds(route: "r", title: "t"), record: nil, audioPath: "", state: "live"), leadIn: "", id: "d-1")))
        }
    }
}

/// Takes through the client: the stub server answers; nothing leaves the process.
final class TakesClientTests: XCTestCase {
    var client: TakesClient!
    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        let keys = InMemoryKeyStore([KeyAccount.apiKey: "k", KeyAccount.computerID: "comp-1"])
        client = TakesClient(client: ChoirClient(baseURL: "https://choir.news", keys: keys, session: URLSession(configuration: cfg)))
        StubProtocol.seen = []
    }
    func fx(_ n: String) throws -> Any { try JSONSerialization.jsonObject(with: Data(contentsOf: TakesTests.mobile.appendingPathComponent("fixtures/takes/\(n)"))) }

    func testUploadPostsMultipartToTheComputerAndReturnsTheId() async throws {
        StubProtocol.handler = { _ in .init(json: ["take_id": "take-00000000000000b6"]) }
        let id = try await client.upload(.init(audio: Data(repeating: 7, count: 100), transcript: "hi", duration: 4, respondsTo: nil))
        XCTAssertEqual(id, "take-00000000000000b6")
        let r = try XCTUnwrap(StubProtocol.seen.last)
        XCTAssertEqual(r.method, "POST"); XCTAssertEqual(r.path, "/api/autoradio/takes")
        XCTAssertTrue(r.headers["content-type"]?.hasPrefix("multipart/form-data; boundary=") ?? false)
        XCTAssertEqual(r.headers["x-choir-computer"], "comp-1")
    }

    func testUploadOverTheBoundIsRefusedBeforeTheNetwork() async {
        do { _ = try await client.upload(.init(audio: Data(count: TakeBounds.maxBytes + 1), transcript: "hi", duration: 4, respondsTo: nil)); XCTFail() } catch {}
        XCTAssertTrue(StubProtocol.seen.isEmpty)
    }

    func testPublishListMineRecordDelete() async throws {
        StubProtocol.handler = { r in
            switch (r.method, r.path) {
            case ("POST", "/api/platform/takes"): return .init(json: ["take_id": r.body["take_id"] as Any, "publisher": "voice-me0001", "state": "live"])
            case ("GET", "/api/platform/takes"): return .init(json: (try? self.fx("list.json")) ?? [:])
            case ("GET", "/api/autoradio/takes"): return .init(json: (try? self.fx("mine.json")) ?? [:])
            case ("GET", "/api/platform/takes/record"): return .init(json: (try? self.fx("record.json")) ?? [:])
            default: return .init(status: 204)
            }
        }
        let t = try await client.publish(takeID: "take-00000000000000b6")
        XCTAssertEqual(t.publisher, "voice-me0001")
        let all = try await client.list(); XCTAssertEqual(all.count, 4)
        XCTAssertEqual(StubProtocol.seen.last?.query["limit"], "50")
        let mine = try await client.mine(); XCTAssertEqual(mine.map(\.published), [true, false])
        let rec = try await client.record(publisher: "voice-me0001"); XCTAssertEqual(rec.words, "heard through by 7 of 11 listeners over 5 takes")
        XCTAssertEqual(StubProtocol.seen.last?.query["publisher"], "voice-me0001")
        try await client.delete(takeID: "take-00000000000000b6")
        XCTAssertEqual(StubProtocol.seen.suffix(2).map { $0.method + " " + $0.path }, ["DELETE /api/platform/takes/take-00000000000000b6", "DELETE /api/autoradio/takes/take-00000000000000b6"])
    }

    func testT7AnUnknownPublisherReadsAsNew() async throws {
        StubProtocol.handler = { _ in .init(status: 404) }
        let rec = try await client.record(publisher: "voice-none"); XCTAssertEqual(rec.words, "new: no outcomes yet")
    }

    func testAudioIsFetchedFromTheListsPathWithTheKey() async throws {
        StubProtocol.handler = { _ in .init(status: 200, raw: "AUDIOBYTES") }
        let d = try await client.audio(path: Take.audioPath("take-00000000000000a1"))
        XCTAssertEqual(String(decoding: d, as: UTF8.self), "AUDIOBYTES")
        XCTAssertEqual(StubProtocol.seen.last?.headers["accept"], "audio/mp4"); XCTAssertEqual(StubProtocol.seen.last?.headers["authorization"], "Bearer k")
    }
}
