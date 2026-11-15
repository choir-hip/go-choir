import XCTest
@testable import AutoradioKit

// The stack's neighbours are fetched before the listener swipes. The failure list is in TextureReaderSource.swift (items 7 to 10);
// each test names the item or case it covers.
final class ReaderPrefetchTests: XCTestCase {
    final class Fetches: @unchecked Sendable {
        private let lock = NSLock()
        private var log: [String] = []
        var failing: Set<String> = []
        var delayNs: UInt64 = 0
        func note(_ a: String) { lock.lock(); log.append(a); lock.unlock() }
        var all: [String] { lock.lock(); defer { lock.unlock() }; return log }
        func count(_ a: String) -> Int { all.filter { $0 == a }.count }
    }
    struct Boom: Error {}

    func store(limit: Int = 40, _ f: Fetches) -> ReaderStore {
        ReaderStore(limit: limit, fetcher: { a in
            f.note(a)
            if f.delayNs > 0 { try await Task.sleep(nanoseconds: f.delayNs) }
            if f.failing.contains(a) { throw Boom() }
            return ReaderDoc(address: a, status: "ready", title: "Title of \(a)")
        })
    }
    func wait(_ ts: [Task<Void, Never>]) async { for t in ts { await t.value } }

    // The two neighbours are fetched and decoded; reaching one later makes no request.
    func testPrefetchedNeighboursAreServedFromTheCache() async {
        let f = Fetches(), s = store(f)
        await wait(s.prefetch(["pub:a/prev", "pub:a/next"]))
        XCTAssertEqual(f.all.sorted(), ["pub:a/next", "pub:a/prev"])
        let d = await s.doc("pub:a/next")
        XCTAssertEqual(d.title, "Title of pub:a/next")
        XCTAssertEqual(f.all.count, 2, "no second request for a held story")
    }

    // 7: a neighbour that fails to fetch is not kept; the card loads on demand.
    func testFailedPrefetchLoadsOnDemand() async {
        let f = Fetches(); f.failing = ["pub:a/next"]
        let s = store(f)
        await wait(s.prefetch(["pub:a/next"]))
        XCTAssertNil(s.cached("pub:a/next"))
        f.failing = []
        let d = await s.doc("pub:a/next")
        XCTAssertEqual(d.status, "ready")
        XCTAssertEqual(f.count("pub:a/next"), 2)
    }

    // 8: swipe before the prefetch is done: the page waits for the fetch in flight, one request.
    func testSwipeDuringPrefetchSharesTheFetch() async {
        let f = Fetches(); f.delayNs = 80_000_000
        let s = store(f)
        let ts = s.prefetch(["pub:a/next"])
        let d = await s.doc("pub:a/next")
        await wait(ts)
        XCTAssertEqual(d.status, "ready")
        XCTAssertEqual(f.count("pub:a/next"), 1)
    }

    // A stack of one article has no neighbours: nothing is asked. Empty, repeated and non-story addresses are skipped.
    func testNothingToPrefetchAsksNothing() async {
        let f = Fetches(), s = store(f)
        await wait(s.prefetch(["", ""]))
        await wait(s.prefetch(["pub:a/x", "pub:a/x", "texture:doc1", "paper:front"]))
        XCTAssertEqual(f.all, ["pub:a/x"])
    }

    // A story already held is not asked for again by a prefetch (the neighbour of the next card is often held).
    func testHeldStoryIsNotPrefetchedAgain() async {
        let f = Fetches(), s = store(f)
        _ = await s.doc("pub:a/one")
        await wait(s.prefetch(["pub:a/one"]))
        XCTAssertEqual(f.count("pub:a/one"), 1)
    }

    // 9: a long session: the cache stays at its bound, the least recently used goes, a story just read stays.
    func testCacheIsBoundedByCountLeastRecentlyUsedFirst() async {
        let f = Fetches(), s = store(limit: 3, f)
        for a in ["pub:1", "pub:2", "pub:3"] { _ = await s.doc(a) }
        _ = s.cached("pub:1")                     // 1 is read again: 2 is now the oldest
        _ = await s.doc("pub:4")
        XCTAssertEqual(s.cachedCount, 3)
        XCTAssertNil(s.cached("pub:2"))
        XCTAssertNotNil(s.cached("pub:1"))
        XCTAssertNotNil(s.cached("pub:4"))
        for i in 5...30 { _ = await s.doc("pub:\(i)") }
        XCTAssertEqual(s.cachedCount, 3)
    }

    // 10: a held story is shown as held; `force` (the page's reload) fetches again.
    func testForceReadsAgain() async {
        let f = Fetches(), s = store(f)
        _ = await s.doc("pub:a/x")
        _ = await s.doc("pub:a/x")
        XCTAssertEqual(f.count("pub:a/x"), 1)
        _ = await s.doc("pub:a/x", force: true)
        XCTAssertEqual(f.count("pub:a/x"), 2)
    }

    // The cache keeps going after an invalidation (reload): the address is fetched again once.
    func testInvalidateThenDocFetchesOnce() async {
        let f = Fetches(), s = store(f)
        _ = await s.doc("pub:a/x")
        s.invalidate("pub:a/x")
        _ = await s.doc("pub:a/x")
        XCTAssertEqual(f.count("pub:a/x"), 2)
        XCTAssertEqual(s.cachedCount, 1)
    }
}
