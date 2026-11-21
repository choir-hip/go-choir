import XCTest
@testable import AutoradioKit

// Lane autoradio-dj-clips. Failure list (the numbers are in the test names):
// C1 a story with 0, 1 or 5 clips sends 0, 1 or at most 3; invalid clips and repeated ids are left out
// C2 one bulk clips request per stretch, cached after; a 404 or an empty list leaves the request as it was
// C3 a podcast_clip segment decodes with its clip; one with no clip is dropped by the decoder
// Playing a clip, its lead-in, Keep listening, the preflight and a skipped clip are entries on the run sheet: RunSheetStoreTests.

private func clipRow(_ id: String, route: String = "/a", start: Double = 10, end: Double = 70, show: String = "Show", ep: String = "Episode", quote: String = "A quote.") -> [String: Any] {
    ["clip_id": id, "story": ["route": route, "version": 2, "title": "Story"], "show": show, "episode_title": ep, "episode_guid": "g-\(id)",
     "feed_url": "https://example.test/feed.xml", "published": "2026-11-16T09:00:00Z", "audio_url": "https://example.test/\(id).mp3",
     "duration_seconds": 900, "start": start, "end": end, "quote": quote, "support": 0.9, "fingerprint": testFingerprintJSON()]
}
private func hex(_ n: Int) -> String { "clip-" + String(format: "%012x", n) }

final class ClipCandidatesTests: XCTestCase {
    func store(_ rows: [[String: Any]]) -> ClipStore { let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse)); return s }
    func cand(_ route: String) -> StretchCandidate { StretchCandidate(route: route, version: 2, title: "T", dek: "D", text: nil) }

    func testC1_ZeroOneFiveClipsSendAtMostThree() {
        for (n, want) in [(0, 0), (1, 1), (5, 3)] {
            let s = store((0..<n).map { clipRow(hex($0 + 1)) })
            let body = StretchCandidates.body([cand("a")], texts: [:], clips: s.candidateClips(forRoutes: ["a"]))
            XCTAssertEqual((body[0]["clips"] as? [[String: Any]])?.count ?? 0, want, "\(n) clips")
        }
    }
    func testC1_OnlyWhatTheServerAcceptsIsSent() {
        let long = String(repeating: "é", count: 300)
        let s = store([clipRow(hex(1), start: 0, end: 3), clipRow(hex(2), start: 0, end: 400), clipRow("bad-id"), clipRow(hex(3), show: " ", ep: "x"),
                       clipRow(hex(4), show: long, ep: long, quote: String(repeating: "q", count: 900)), clipRow(hex(4)), clipRow(hex(5), route: "/b")])
        let c = s.candidateClips(forRoutes: ["a", "b"])
        XCTAssertEqual(c["a"]?.map(\.clipID), [hex(4)], "too short, too long, a bad id, no show, a repeated id are out")
        XCTAssertLessThanOrEqual(c["a"]![0].show.utf8.count, 200); XCTAssertLessThanOrEqual(c["a"]![0].quote.utf8.count, 400)
        XCTAssertEqual(c["b"]?.map(\.clipID), [hex(5)])
        XCTAssertEqual(Set(c.values.flatMap { $0 }.map(\.clipID)).count, c.values.flatMap { $0 }.count, "ids are unique in the request")
    }
    func testC1_OnlyTheFieldsTheServerNamesAreCopied() {
        let s = store([clipRow(hex(1))])
        let j = s.candidateClips(forRoutes: ["a"])["a"]![0].json
        XCTAssertEqual(Set(j.keys), ["clip_id", "show", "episode_title", "published", "seconds", "quote"])
        XCTAssertEqual(j["seconds"] as? Double, 60)
    }
    // C9 (the story page's rows) is shared code now: clips.test.ts H1.
    func testC3_ABlockForTheClipFollowsTheLeadIn() throws {
        let seg = IncomingSegment(stretchID: "s", segmentID: "s/1", kind: "podcast_clip", title: "Show on it", text: "Here is how Show put it.", cites: [], blockCount: 1,
                                  clip: ProgrammeClipRef(clipID: hex(1), storyRef: "pub:a@v2", seconds: 60))
        let p = try XCTUnwrap(SegmentPrep.prepare(seg))
        XCTAssertEqual(p.blocks, [.speech(text: "Here is how Show put it."), .podcastClip(clipID: hex(1))])
        XCTAssertEqual(p.segment.clip?.clipID, hex(1)); XCTAssertEqual(p.segment.kind, "podcast_clip")
        var noClip = seg; noClip.clip = nil
        XCTAssertNil(SegmentPrep.prepare(noClip))
    }
    func testC3_TheDecoderReadsTheClipAndDropsALineWithNone() {
        var d = StretchDecoder()
        let ok = segmentLine(["kind": "podcast_clip", "title": "T", "text": "Lead.", "clip": ["clip_id": hex(1), "story_ref": "pub:a@v2", "seconds": 56.5]])
        let bad = segmentLine(["kind": "podcast_clip", "title": "T", "text": "Lead."])
        let evs = d.feed(ok + "\n" + bad + "\n")
        XCTAssertEqual(evs.count, 1)
        if case .segment(let s) = evs[0] { XCTAssertEqual(s.clip, ProgrammeClipRef(clipID: hex(1), storyRef: "pub:a@v2", seconds: 56.5)) } else { XCTFail() }
        XCTAssertEqual(d.droppedCount, 1)
    }
}

// MARK: The requests

final class ClipRequestTests: XCTestCase {
    var env: ChoirEnvironment!
    var clipsStatus = 200
    var clipRows: [[String: Any]] = []
    override func setUp() {
        env = ChoirEnvironment.shared
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock()); env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { [self] req in
            switch req.path {
            case "/api/platform/publications/front":
                return .init(json: ["stories": ["a", "b", "c"].enumerated().map { i, r in
                    ["title": "Story \(r)", "dek": "Dek \(r).", "route": ["bare": r], "section": "world", "words": 40, "importance": 9 - i] as [String: Any] }])
            case "/api/platform/publications/resolve": return .init(json: ["payload": ["text": "Text."]])
            case "/api/platform/podcast-clips": return .init(status: clipsStatus, json: ["clips": clipRows])
            default: return .init(status: 404, json: ["error": "nope"])
            }
        }
    }
    override func tearDown() { env.session = .shared }
    var clipCalls: Int { StubProtocol.seen.filter { $0.path == "/api/platform/podcast-clips" }.count }

    func testC2_OneBulkRequestForTheWholeListAndNoneAfterwards() async throws {
        clipRows = [clipRow(hex(1), route: "/a"), clipRow(hex(2), route: "/b"), clipRow(hex(3), route: "/b")]
        let store = ClipStore()
        let src = RouteStretchSource(baseURL: ChoirEnvironment.defaultBaseURL)
        let client = env.client(baseURL: ChoirEnvironment.defaultBaseURL)
        let c1 = try await src.candidates(client: client, skipping: [], clips: store)
        XCTAssertEqual(clipCalls, 1, "one request, not one per story")
        XCTAssertEqual(c1.map { ($0["clips"] as? [[String: Any]])?.count ?? 0 }, [1, 2, 0])
        _ = try await src.candidates(client: client, skipping: ["a"], clips: store)
        XCTAssertEqual(clipCalls, 1, "cached for the session")
        store.markStale()
        _ = try await src.candidates(client: client, skipping: [], clips: store)
        XCTAssertEqual(clipCalls, 2, "refreshed with the front")
    }
    func testC2_ARouteThatAnswers404LeavesTheRequestAsItWas() async throws {
        clipsStatus = 404
        let src = RouteStretchSource(baseURL: ChoirEnvironment.defaultBaseURL)
        let c = try await src.candidates(client: env.client(baseURL: ChoirEnvironment.defaultBaseURL), skipping: [], clips: ClipStore())
        XCTAssertEqual(c.count, 3); XCTAssertTrue(c.allSatisfy { $0["clips"] == nil })
        clipsStatus = 200; clipRows = []
        let c2 = try await src.candidates(client: env.client(baseURL: ChoirEnvironment.defaultBaseURL), skipping: [], clips: ClipStore())
        XCTAssertTrue(c2.allSatisfy { $0["clips"] == nil }, "an empty list too")
    }
    func testC7_TheListIsHandedOverFromTheStoryOnward() {
        let rows = ["a", "b", "c", "d"].map { ["title": "T\($0)", "dek": "D.", "route": ["bare": $0], "importance": $0 == "a" ? 9 : 1] as [String: Any] }
        let c = StretchCandidates.fromFront(["stories": rows], onlyRoutes: ["c", "d"])
        XCTAssertEqual(c.map(\.route), ["c", "d"], "its stories, its order")
    }
}
