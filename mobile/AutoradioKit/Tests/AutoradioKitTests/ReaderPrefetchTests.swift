import XCTest
@testable import AutoradioKit

// The Swift reader store's cache (what the voice's page text and the retrieval corpus still read). The stack's prefetch is shared code now (reader.test.ts R8).
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
