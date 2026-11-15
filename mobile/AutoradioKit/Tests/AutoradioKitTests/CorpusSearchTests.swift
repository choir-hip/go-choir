import XCTest
@testable import AutoradioKit

/// The failure list is at the head of CorpusSearch.swift (numbers in the test names).
final class CorpusSearchTests: XCTestCase {
    // The real shape of GET /api/platform/retrieval/search (read from choir.news, trimmed).
    static let body = #"""
    {"query":"nvidia","results":[
     {"publication_id":"pub-a","publication_version_id":"pub-a-v10","title":"Trump Signals Openness to AI Guardrails","route_path":"/pub/texture/trump-a","source_id":"s1","span_id":"x","content_hash":"h","source_revision_hash":"r","snippet":"...dterms that they want AI. [5]  The groups seized on the Nvidia controversy, demanding hearings...","score":3},
     {"publication_id":"pub-b","title":"Nvidia CEO steps down","route_path":"/pub/texture/nvidia-b","snippet":"# Nvidia CEO steps down\n\nTechCrunch reports that Nvidia CEO","score":21},
     {"publication_id":"pub-c","title":"","route_path":"/pub/texture/no-title","snippet":"x","score":1},
     {"publication_id":"pub-d","title":"No route","route_path":"","snippet":"x","score":1},
     {"publication_id":"pub-b2","title":"Nvidia CEO steps down again","route_path":"/pub/texture/nvidia-b","snippet":"dup","score":1}
    ]}
    """#

    func testDecodesRealShapeAndSkipsUnopenable_8_9() throws {
        let hits = try CorpusSearch.decode(Data(Self.body.utf8))
        XCTAssertEqual(hits.map(\.route), ["/pub/texture/trump-a", "/pub/texture/nvidia-b"])
        XCTAssertEqual(hits[0].address, "pub:/pub/texture/trump-a")
        XCTAssertTrue(hits[0].snippet.contains("Nvidia controversy"))
        XCTAssertFalse(hits[0].snippet.contains("\n"))
        XCTAssertEqual(hits[1].snippet, "TechCrunch reports that Nvidia CEO", "the heading is not repeated under the title")
    }

    func testNoResultsAndBadBodies_2_3() throws {
        XCTAssertEqual(try CorpusSearch.decode(Data(#"{"query":"zzz","results":[]}"#.utf8)), [])
        XCTAssertEqual(try CorpusSearch.decode(Data(#"{"query":"zzz","results":null}"#.utf8)), [])
        XCTAssertThrowsError(try CorpusSearch.decode(Data("<html>".utf8)))
        XCTAssertThrowsError(try CorpusSearch.decode(Data(#"{"results":"no"}"#.utf8)))
        XCTAssertEqual(CorpusSearcher.note(.done([]), hasHits: false), "Nothing found in the paper.")
        XCTAssertEqual(CorpusSearcher.note(.done([]), hasHits: true), "")
        XCTAssertFalse(CorpusSearcher.note(.failed, hasHits: false).isEmpty)
        XCTAssertFalse(CorpusSearcher.note(.searching, hasHits: false).isEmpty)
    }

    func testNormalizeBlankIsEmpty_1() {
        XCTAssertEqual(CorpusSearch.normalize("  \n\t "), "")
        XCTAssertEqual(CorpusSearch.normalize("  Nvidia   chips "), "Nvidia chips")
    }

    func testRowsKeepLocalFirstAndDedupe() {
        let local = [SearchHit(address: "pub:/pub/texture/nvidia-b", title: "Nvidia CEO steps down", kind: "story", detail: "Tech")]
        let hits = try! CorpusSearch.decode(Data(Self.body.utf8))
        let rows = CorpusSearch.rows(hits, after: local)
        XCTAssertEqual(rows.map(\.address), ["pub:/pub/texture/nvidia-b", "pub:/pub/texture/trump-a"])
        XCTAssertEqual(rows[0].detail, "Tech")
    }

    func testQueryIsPercentEncoded_6() async throws {
        let keys = InMemoryKeyStore([:])
        let c = ChoirClient(baseURL: "https://example.test", keys: keys)
        for (q, want) in [("AT&T", "q=AT%26T"), ("C++ x", "q=C%2B%2B%20x"), ("a#b=c", "q=a%23b%3Dc"), ("café", "q=caf%C3%A9")] {
            let r = try await c.request("GET", CorpusSearch.path, query: ["q": q], authenticated: false)
            XCTAssertEqual(r.url?.query, want)
            XCTAssertEqual(r.url?.path, CorpusSearch.path)
            XCTAssertNil(r.value(forHTTPHeaderField: "Authorization"), "the public route carries no key")
        }
    }

    func testSearchPaperHitsThePublicRouteWithoutKey() async throws {
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init(json: ["query": "x", "results": []]) }
        let c = ChoirClient(baseURL: "https://example.test", keys: InMemoryKeyStore([:]), session: URLSession(configuration: cfg))
        let d = try await c.searchPaper("Israel")
        XCTAssertEqual(try CorpusSearch.decode(d), [])
        let seen = StubProtocol.seen.last
        XCTAssertEqual(seen?.path, "/api/platform/retrieval/search")
        XCTAssertEqual(seen?.query["q"], "Israel")
        XCTAssertNil(seen?.headers["authorization"])
    }

    /// An ask the test controls: it answers per query when released.
    final class Gate: @unchecked Sendable {
        let lock = NSLock(); var asked: [String] = []; var waiting: [String: CheckedContinuation<Data, Error>] = [:]
        func note(_ q: String) { lock.lock(); asked.append(q); lock.unlock() }
        func park(_ q: String, _ k: CheckedContinuation<Data, Error>) { lock.lock(); waiting[q] = k; lock.unlock() }
        func isWaiting(_ q: String) -> Bool { lock.lock(); defer { lock.unlock() }; return waiting[q] != nil }
        func askedList() -> [String] { lock.lock(); defer { lock.unlock() }; return asked }
        func ask(_ q: String) async throws -> Data {
            note(q)
            return try await withCheckedThrowingContinuation { k in park(q, k) }
        }
        func answer(_ q: String, _ d: Data) { lock.lock(); let k = waiting.removeValue(forKey: q); lock.unlock(); k?.resume(returning: d) }
        func fail(_ q: String) { lock.lock(); let k = waiting.removeValue(forKey: q); lock.unlock(); k?.resume(throwing: ChoirError.transport("down")) }
    }
    func eventually(_ what: String, _ cond: @escaping @Sendable () -> Bool) async {
        for _ in 0..<400 { if cond() { return }; try? await Task.sleep(nanoseconds: 5_000_000) }
        XCTFail("timed out: \(what)")
    }

    func testEmptyQueryAsksNothing_1() async throws {
        let g = Gate(); let s = CorpusSearcher(pause: 0.01, ask: g.ask)
        XCTAssertEqual(s.status(for: "   "), .idle)
        try await Task.sleep(nanoseconds: 60_000_000)
        XCTAssertEqual(g.askedList(), [])
    }

    func testSlowAnswerDoesNotBlockAndLandsOnce_4() async throws {
        let g = Gate(); let s = CorpusSearcher(pause: 0.01, ask: g.ask)
        let changed = Counter(); s.onChange = { changed.bump() }
        XCTAssertEqual(s.status(for: "nvidia"), .searching, "returns at once while the ask is out")
        await eventually("asked") { g.askedList() == ["nvidia"] && g.isWaiting("nvidia") }
        XCTAssertEqual(s.status(for: "nvidia "), .searching, "the same query is not asked twice")
        g.answer("nvidia", Data(Self.body.utf8))
        await eventually("done") { if case .done = s.status(for: "nvidia") { return true }; return false }
        XCTAssertEqual(g.askedList(), ["nvidia"])
        XCTAssertEqual(changed.n, 1)
    }

    func testLateAnswerForOlderQueryIsDropped_5() async throws {
        let g = Gate(); let s = CorpusSearcher(pause: 0.01, ask: g.ask)
        _ = s.status(for: "nvi")
        await eventually("first asked") { g.isWaiting("nvi") }
        _ = s.status(for: "nvidia")
        await eventually("second asked") { g.isWaiting("nvidia") }
        g.answer("nvidia", Data(#"{"results":[{"title":"New","route_path":"/pub/new","snippet":""}]}"#.utf8))
        await eventually("new landed") { if case .done = s.status(for: "nvidia") { return true }; return false }
        g.answer("nvi", Data(#"{"results":[{"title":"Old","route_path":"/pub/old","snippet":""}]}"#.utf8))
        try await Task.sleep(nanoseconds: 60_000_000)
        guard case .done(let h) = s.status(for: "nvidia") else { return XCTFail("state lost") }
        XCTAssertEqual(h.map(\.route), ["/pub/new"])
    }

    func testTypingFastAsksOnlyAfterThePause_7() async throws {
        let g = Gate(); let s = CorpusSearcher(pause: 0.15, ask: g.ask)
        for q in ["n", "nv", "nvi", "nvid"] { _ = s.status(for: q); try await Task.sleep(nanoseconds: 20_000_000) }
        await eventually("asked") { !g.askedList().isEmpty }
        XCTAssertEqual(g.askedList(), ["nvid"])
    }

    func testServerErrorIsAPlainFailureNotABlankScreen_3() async throws {
        let g = Gate(); let s = CorpusSearcher(pause: 0.01, ask: g.ask)
        _ = s.status(for: "israel")
        await eventually("asked") { g.isWaiting("israel") }
        g.fail("israel")
        await eventually("failed") { s.status(for: "israel") == .failed }
        XCTAssertTrue(CorpusSearcher.note(.failed, hasHits: false).contains("Could not search"))
    }

    func testClearingTheQueryDropsAnInFlightAnswer_1_5() async throws {
        let g = Gate(); let s = CorpusSearcher(pause: 0.01, ask: g.ask)
        _ = s.status(for: "israel")
        await eventually("asked") { g.isWaiting("israel") }
        XCTAssertEqual(s.status(for: ""), .idle)
        g.answer("israel", Data(Self.body.utf8))
        try await Task.sleep(nanoseconds: 60_000_000)
        XCTAssertEqual(s.status(for: ""), .idle)
    }
}

final class Counter: @unchecked Sendable { let l = NSLock(); var n = 0; func bump() { l.lock(); n += 1; l.unlock() } }
