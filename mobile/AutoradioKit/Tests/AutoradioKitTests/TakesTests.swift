import XCTest
@testable import AutoradioKit

/// Takes, what the programme still reads (failure list: Takes.swift; the page, the wire for publishing and the listener's own data are shared code, takes.test.ts and take-session.test.ts).
final class TakesTests: XCTestCase {
    static let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    func fixture(_ n: String) throws -> Data { try Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/takes/\(n)")) }
    func list() throws -> [Take] { TakesClient.takes(try JSONSerialization.jsonObject(with: fixture("list.json"))) }

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
        let own = TakeOwn(); own.replace(ids: ["x"], handle: "voice-me0001")
        let mutes = TakeMutes(); mutes.replace(["voice-9b0e44"])
        let c = TakeCandidates.select(try list(), own: own, mutes: mutes)
        XCTAssertEqual(c.map(\.takeID), ["take-00000000000000a1", "take-00000000000000a3"], "not the listener's own (by handle), not a muted publisher's, newest first")
        let byID = TakeOwn(); byID.replace(ids: ["take-00000000000000a1"], handle: "")
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

    func testT6TheProgrammesCopyOfOwnIdsAndMutesIsWhatSharedCodePushedAndSurvivesARelaunch() {
        let d = FileManager.default.temporaryDirectory.appendingPathComponent("mirror-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: d) }
        let fo = d.appendingPathComponent("takes-own.json"), fm = d.appendingPathComponent("takes-muted.json")
        let own = TakeOwn(file: fo), mutes = TakeMutes(file: fm)
        own.replace(ids: ["take-2", "take-1"], handle: "voice-me0001"); mutes.replace(["voice-3fa2c1", "bad handle"])
        XCTAssertEqual(TakeOwn(file: fo).allIDs, ["take-1", "take-2"]); XCTAssertEqual(TakeOwn(file: fo).handle, "voice-me0001")
        XCTAssertEqual(TakeMutes(file: fm).all, ["voice-3fa2c1"], "a handle that is not one is not kept")
        own.replace(ids: [], handle: ""); mutes.replace([])
        XCTAssertEqual(TakeMutes(file: fm).all, []); XCTAssertEqual(TakeOwn(file: fo).allIDs, [])
    }

    func testTheOldFilesAreReadAsTheyWereWrittenByTheOldBuild() throws {
        let d = FileManager.default.temporaryDirectory.appendingPathComponent("old-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: d, withIntermediateDirectories: true); defer { try? FileManager.default.removeItem(at: d) }
        try Data(#"{"handle":"voice-me0001","ids":["take-a","take-b"]}"#.utf8).write(to: d.appendingPathComponent("takes-own.json"))
        try Data(#"["voice-9b0e44"]"#.utf8).write(to: d.appendingPathComponent("takes-muted.json"))
        let own = TakeOwn(file: d.appendingPathComponent("takes-own.json")), mutes = TakeMutes(file: d.appendingPathComponent("takes-muted.json"))
        XCTAssertEqual(own.allIDs, ["take-a", "take-b"]); XCTAssertEqual(own.handle, "voice-me0001"); XCTAssertEqual(mutes.all, ["voice-9b0e44"])
        XCTAssertTrue(FileManager.default.fileExists(atPath: d.appendingPathComponent("takes-own.json").path), "reading removes nothing")
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

    func testTheListIsAskedWithItsLimitAndAServerWithoutTakesIsNotAvailableYet() async throws {
        StubProtocol.handler = { _ in .init(json: (try? self.fx("list.json")) ?? [:]) }
        let all = try await client.list(); XCTAssertEqual(all.count, 4)
        XCTAssertEqual(StubProtocol.seen.last?.query["limit"], "50")
        StubProtocol.handler = { _ in .init(status: 404) }
        do { _ = try await client.list(); XCTFail() } catch { XCTAssertEqual(error as? TakesUnavailable, TakesUnavailable()) }
    }

    func testTheRecordIsFlatWithItsSummaryLine() throws {
        let o = try JSONSerialization.jsonObject(with: Data(#"{"publisher":"voice-1","offered":11,"heard":7,"skipped":4,"listeners":5,"takes":3,"new":false,"summary":"heard through 7 of 11 times, by 5 distinct listeners, over 3 takes","drill":{"heard":0,"skipped":0,"drill_heard":0,"drill_skipped":0}}"#.utf8))
        XCTAssertEqual(TakeRecordSummary.parse(o)?.words, "heard through 7 of 11 times, by 5 distinct listeners, over 3 takes")
    }

    func testAudioIsFetchedFromTheListsPathWithTheKey() async throws {
        StubProtocol.handler = { _ in .init(status: 200, raw: "AUDIOBYTES") }
        let d = try await client.audio(path: Take.audioPath("take-00000000000000a1"))
        XCTAssertEqual(String(decoding: d, as: UTF8.self), "AUDIOBYTES")
        XCTAssertEqual(StubProtocol.seen.last?.headers["accept"], "audio/mp4"); XCTAssertEqual(StubProtocol.seen.last?.headers["authorization"], "Bearer k")
    }
}
