import XCTest
@testable import AutoradioKit

/// A fake model: the vector of a prompt is a unit vector from its words (hashed into 16 slots), so texts sharing words are near.
private struct HashEmbedder: Embedder {
    var engine = "fake-1"
    var dimension = 64
    var fail = false
    func embed(_ prompts: [String]) async throws -> [[Float]] {
        if fail { throw EmbedderError.unavailable("no model") }
        return prompts.map { p in
            var v = [Float](repeating: 0, count: 64)
            for w in p.lowercased().split(whereSeparator: { !$0.isLetter }) {
                var h: UInt64 = 5381; for u in w.utf8 { h = h &* 33 &+ UInt64(u) }          // a hash that does not change between runs
                v[Int(h % 64)] += 1
            }
            return truncatedUnit(v, dim: 64)
        }
    }
}

final class EmbedderTests: XCTestCase {
    func dir() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("embed-\(UUID().uuidString)") }
    let items = zip(0..<10, ["alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet"]).map { EmbeddingJob.Item(id: "d\($0.0)", key: "k\($0.0)", title: "t", text: "story about bridges and senate votes \($0.1)") }

    func testPromptStringsAreTheCards() {
        XCTAssertEqual(EmbedPrompt.document(title: nil, text: "x"), "title: none | text: x")
        XCTAssertEqual(EmbedPrompt.document(title: "A", text: "x"), "title: A | text: x")
        XCTAssertEqual(EmbedPrompt.query("why"), "task: search result | query: why")
    }

    func testTruncationIsUnit() {
        let v = truncatedUnit([3, 4, 12, 0], dim: 2)
        XCTAssertEqual(v[0], 0.6, accuracy: 1e-6); XCTAssertEqual(v[1], 0.8, accuracy: 1e-6)
    }

    func testJobEmbedsOnlyWhatChangedAndSurvivesReopen() async throws {
        let d = dir()
        let e = HashEmbedder()
        let s1 = VectorStore(directory: d, engine: e.engine, dimension: 64)
        let p1 = await EmbeddingJob(store: s1, embedder: e, busy: { false }).run(items)
        XCTAssertEqual(p1.embedded, 10); XCTAssertTrue(p1.finished)
        // a new store object over the same directory: nothing to do, everything found
        let s2 = VectorStore(directory: d, engine: e.engine, dimension: 64)
        XCTAssertEqual(s2.count, 10)
        var changed = items; changed[3].key = "k3-new"; changed.removeLast()
        let p2 = await EmbeddingJob(store: s2, embedder: e, busy: { false }).run(changed)
        XCTAssertEqual(p2.embedded, 1); XCTAssertEqual(p2.skipped, 8); XCTAssertEqual(p2.removed, 1)
        XCTAssertEqual(s2.count, 9)
        // the search finds the passage whose words the query shares
        let q = try await e.embed([EmbedPrompt.query("delta")])[0]
        XCTAssertEqual(s2.search(q, k: 3).first?.id, "d3")
    }

    func testEngineChangeStartsEmpty() async throws {
        let d = dir()
        let s1 = VectorStore(directory: d, engine: "a", dimension: 64)
        await EmbeddingJob(store: s1, embedder: HashEmbedder(engine: "a"), busy: { false }).run(items)
        XCTAssertEqual(VectorStore(directory: d, engine: "b", dimension: 64).count, 0)
        XCTAssertEqual(VectorStore(directory: d, engine: "a", dimension: 8).count, 0)
    }

    func testCancelledJobResumes() async throws {
        let d = dir(), e = HashEmbedder()
        let s = VectorStore(directory: d, engine: e.engine, dimension: 64)
        var b = EmbeddingJob.Budget(); b.passagesPerSlice = 2; b.restBetweenSlices = .milliseconds(40)
        let job = EmbeddingJob(store: s, embedder: e, busy: { false }, budget: b)
        let t = Task { await job.run(items) }
        try await Task.sleep(for: .milliseconds(70)); t.cancel()
        let first = await t.value
        XCTAssertFalse(first.finished); XCTAssertGreaterThan(first.embedded, 0); XCTAssertLessThan(first.embedded, 10)
        let reopened = VectorStore(directory: d, engine: e.engine, dimension: 64)
        XCTAssertEqual(reopened.count, first.embedded)                 // saved when the job ends (here: cancelled)
        let second = await EmbeddingJob(store: reopened, embedder: e, busy: { false }).run(items)
        XCTAssertEqual(second.embedded, 10 - first.embedded); XCTAssertEqual(second.skipped, first.embedded); XCTAssertTrue(second.finished)
    }

    func testSavesEveryNSlicesAndAtTheEnd() async throws {
        let d = dir(), e = HashEmbedder()
        let s = VectorStore(directory: d, engine: e.engine, dimension: 64)
        var b = EmbeddingJob.Budget(); b.passagesPerSlice = 1; b.restBetweenSlices = .milliseconds(1); b.savesEverySlices = 4
        let onDisk = Locked<[Int]>([])
        let p = await EmbeddingJob(store: s, embedder: e, busy: { false }, budget: b).run(items) { _ in
            let n = VectorStore(directory: d, engine: e.engine, dimension: 64).count; onDisk.write { $0.append(n) }
        }
        XCTAssertEqual(onDisk.read { $0 }, [0, 0, 0, 4, 4, 4, 4, 8, 8, 8], "the file is rewritten every 4 slices, not after each one")
        XCTAssertTrue(p.finished)
        XCTAssertEqual(VectorStore(directory: d, engine: e.engine, dimension: 64).count, 10, "and once more at the end")
    }

    func testSpeechWins() async throws {
        let d = dir(), e = HashEmbedder()
        let s = VectorStore(directory: d, engine: e.engine, dimension: 64)
        let speaking = Locked(true)
        var b = EmbeddingJob.Budget(); b.busyPoll = .milliseconds(20)
        let job = EmbeddingJob(store: s, embedder: e, busy: { speaking.read { $0 } }, budget: b)
        let t = Task { await job.run(items) }
        try await Task.sleep(for: .milliseconds(150))
        XCTAssertEqual(s.count, 0)                                     // nothing embedded while a voice line is on
        speaking.write { $0 = false }
        let p = await t.value
        XCTAssertEqual(p.embedded, 10); XCTAssertGreaterThan(p.waitedForSpeech, 0)
    }

    func testMissingModelStopsTheJob() async {
        let s = VectorStore(directory: dir(), engine: "fake-1", dimension: 64)
        let p = await EmbeddingJob(store: s, embedder: HashEmbedder(fail: true), busy: { false }).run(items)
        XCTAssertEqual(p.embedded, 0); XCTAssertEqual(p.failed, 10)
    }

    func testRankFusionRewardsAgreement() {
        let f = RankFusion.rrf([["a", "b", "c"], ["c", "d", "a"]])
        XCTAssertEqual(f.first, "a")
        XCTAssertEqual(Set(f), ["a", "b", "c", "d"])
    }
}
