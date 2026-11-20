import XCTest
@testable import AutoradioKit

/// Word-hash vectors (64 slots, a hash that is the same on every run): texts that share words are near.
struct WordHashEmbedder: Embedder, QueryEmbedding {
    var engine = KnownModels.embeddingEngine
    var dimension = 64
    func vec(_ p: String) -> [Float] {
        var v = [Float](repeating: 0, count: 64)
        for w in p.lowercased().split(whereSeparator: { !$0.isLetter }) {
            var h: UInt64 = 5381; for u in w.utf8 { h = h &* 33 &+ UInt64(u) }
            v[Int(h % 64)] += 1
        }
        return truncatedUnit(v, dim: 64)
    }
    func embed(_ prompts: [String]) async throws -> [[Float]] { prompts.map(vec) }
    func embedQuery(_ text: String) -> [Float]? { vec(EmbedPrompt.query(text)) }
}

final class HybridRankTests: XCTestCase {
    let vec = (1...30).map { "v\($0)" }

    func testVectorsOnlyKeepsVectorOrder() {
        XCTAssertEqual(HybridRank.fuse(.vectorsOnly, vectors: vec, bm25: ["b1", "v3"], exact: ["b1"], limit: 5), ["v1", "v2", "v3", "v4", "v5"])
    }
    func testRRFRewardsAgreementOverEitherList() {
        let out = HybridRank.fuse(.rrf, vectors: ["a", "b", "c"], bm25: ["c", "d", "a"], exact: [], limit: 10)
        XCTAssertEqual(out.first, "a")
        XCTAssertEqual(Set(out), ["a", "b", "c", "d"])
    }
    func testExactTermHitsAreKeptEvenWhenTheVectorsDoNotRankThem() {
        let out = HybridRank.fuse(.vectorsWithExactTerms, vectors: vec, bm25: ["name1", "v2"], exact: ["name1"], limit: 20)
        XCTAssertEqual(out.count, 20)
        XCTAssertTrue(out.contains("name1"))
        XCTAssertEqual(Array(out.prefix(5)), ["v1", "v2", "v3", "v4", "v5"])          // the vector order is the order
    }
    func testExactHitAlreadyHighStaysWhereTheVectorsPutIt() {
        let out = HybridRank.fuse(.vectorsWithExactTerms, vectors: vec, bm25: ["v2"], exact: ["v2"], limit: 20)
        XCTAssertEqual(Array(out.prefix(3)), ["v1", "v2", "v3"]); XCTAssertEqual(out.filter { $0 == "v2" }.count, 1)
    }
    func testExactHitsAreCappedAtHalfTheList() {
        let many = (1...30).map { "e\($0)" }
        let out = HybridRank.fuse(.vectorsWithExactTerms, vectors: vec, bm25: many, exact: many, limit: 20)
        XCTAssertEqual(out.count, 20); XCTAssertEqual(out.filter { $0.hasPrefix("e") }.count, 10)
    }
    func testNoVectorsFallsBackToBM25ForEveryStrategy() {
        for s in FusionStrategy.allCases { XCTAssertEqual(HybridRank.fuse(s, vectors: [], bm25: ["a", "b"], exact: ["a"], limit: 5), ["a", "b"], "\(s)") }
    }
}

final class FusedRetrieverTests: XCTestCase {
    func dir() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("fused-\(UUID().uuidString)") }
    let docs: [RetrievalDoc] = [
        RetrievalDoc(id: "s1", kind: "story", title: "Harlow viaduct closes", text: "The viaduct over the river will close to cars for eleven months while crews replace the deck."),
        RetrievalDoc(id: "s2", kind: "story", title: "Senate votes", text: "The Senate passed the harbor bill sixty one to thirty eight on Tuesday night."),
        RetrievalDoc(id: "s3", kind: "story", title: "Chip maker cuts jobs", text: "Veridian Micro will lay off nine hundred workers as demand for its accelerators slows."),
        RetrievalDoc(id: "s4", kind: "story", title: "Weather", text: "Freezing drizzle glazed roads across the valley overnight and closed many schools."),
    ]
    func make(embedded: Bool = true) async throws -> (RetrievalIndex, FusedRetriever) {
        let idx = RetrievalIndex(); idx.upsertAll(docs)
        let e = WordHashEmbedder()
        let store = VectorStore(directory: dir(), engine: e.engine, dimension: 64)
        if embedded {
            for it in idx.snapshot() { store.upsert(id: it.id, key: "k", vector: try await e.embed([EmbedPrompt.document(title: it.title, text: it.text)])[0]) }
        }
        return (idx, FusedRetriever(index: idx, vectors: store, embedQuery: { e.embedQuery($0) }, coverage: { 1 }, cosineFloor: 0.3))
    }

    func testWithoutVectorsItIsBM25AloneAndItsLineSaysNoModel() async throws {
        let (idx, r) = try await make(embedded: false)
        let q = RetrievalQuery(words: "viaduct closes")
        XCTAssertEqual(r.candidates(q, limit: 10).map(\.id), idx.candidates(q, limit: 10).map(\.id))
        XCTAssertFalse(r.lastLine.contains("embed"))
    }
    func testVectorsFindAnItemThatSharesNoQueryTermWithBM25Floor() async throws {
        let (_, r) = try await make()
        // The words overlap the story's text only through the hashed vector (the titles differ), so BM25 alone finds nothing useful.
        let out = r.candidates(RetrievalQuery(words: "Veridian Micro lay off workers accelerators demand"), limit: 5)
        XCTAssertEqual(out.first?.id, "s3")
        XCTAssertTrue(r.lastLine.hasPrefix("AUTORADIO retrieve retrieved="), r.lastLine)
    }
    func testARareNameKeepsItsPassageEvenWhenTheVectorsRankItLow() async throws {
        var d = docs; d.append(RetrievalDoc(id: "s5", kind: "story", title: "Local news", text: "Councilwoman Quillfeather spoke about parking at the meeting."))
        let idx = RetrievalIndex(); idx.upsertAll(d)
        let e = WordHashEmbedder()
        let store = VectorStore(directory: dir(), engine: e.engine, dimension: 64)
        for it in idx.snapshot() { store.upsert(id: it.id, key: "k", vector: try await e.embed([EmbedPrompt.document(title: it.title, text: it.text)])[0]) }
        let r = FusedRetriever(index: idx, vectors: store, embedQuery: { e.embedQuery($0) }, coverage: { 1 }, cosineFloor: 0.3)
        let out = r.candidates(RetrievalQuery(words: "viaduct river deck cars crews Quillfeather"), limit: 20)
        XCTAssertTrue(out.map(\.id).contains("s5"))
    }
    func testWindowsOfALongItemCollapseToTheItem() async throws {
        let idx = RetrievalIndex()
        idx.upsertAll([RetrievalDoc(id: "long", kind: "story", title: "A long story", text: String(repeating: "bridge crews steel deck river ", count: 200))])
        let e = WordHashEmbedder()
        let store = VectorStore(directory: dir(), engine: e.engine, dimension: 64)
        let items = EmbeddingService.items(from: idx)
        XCTAssertGreaterThan(items.count, 1)
        for it in items { store.upsert(id: it.id, key: it.key, vector: e.vec(EmbedPrompt.document(title: it.title, text: it.text))) }
        let r = FusedRetriever(index: idx, vectors: store, embedQuery: { e.embedQuery($0) }, coverage: { 1 }, cosineFloor: 0.3)
        let out = r.candidates(RetrievalQuery(words: "bridge steel deck"), limit: 10)
        XCTAssertEqual(out.map(\.id), ["long"])
    }
    func testAnUnrelatedQueryReturnsNothingWhenNeitherSideMatches() async throws {
        let (_, r) = try await make()
        XCTAssertTrue(r.candidates(RetrievalQuery(words: "zebra quartz marmalade"), limit: 10).isEmpty)
    }
    func testPartialCoverageStaysBM25() async throws {
        let idx = RetrievalIndex(); idx.upsertAll(docs)
        let e = WordHashEmbedder()
        let store = VectorStore(directory: dir(), engine: e.engine, dimension: 64)
        let r = FusedRetriever(index: idx, vectors: store, embedQuery: { e.embedQuery($0) }, coverage: { 0.3 })
        let q = RetrievalQuery(words: "viaduct closes")
        XCTAssertEqual(r.candidates(q, limit: 10).map(\.id), idx.candidates(q, limit: 10).map(\.id))
    }
}

final class EmbeddingServiceTests: XCTestCase {
    func dir() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("svc-\(UUID().uuidString)") }
    final class Box: @unchecked Sendable { var lowPower = false, thermal = false, wifi = true, busy = false, present = false, downloads = 0, lines: [String] = []; let l = NSLock()
        func set(_ f: () -> Void) { l.lock(); f(); l.unlock() }
        func get<T>(_ f: () -> T) -> T { l.lock(); defer { l.unlock() }; return f() } }
    func service(_ b: Box, _ vdir: URL) -> EmbeddingService {
        var env = EmbeddingService.Environment()
        env.conditions = { b.get { DeviceConditions(lowPowerMode: b.lowPower, thermalSerious: b.thermal, onWiFi: b.wifi) } }
        env.radioBusy = { b.get { b.busy } }
        env.modelPresent = { b.get { b.present } }
        env.download = { b.set { b.downloads += 1; b.present = true } }
        env.load = { WordHashEmbedder() }
        env.say = { s in b.set { b.lines.append(s) } }
        env.vectorsDirectory = { vdir }
        env.dimension = 64; env.firstDelay = .milliseconds(1); env.settle = .milliseconds(1)
        var budget = EmbeddingJob.Budget(); budget.restBetweenSlices = .milliseconds(1); budget.busyPoll = .milliseconds(10)
        env.budget = budget
        return EmbeddingService(env)
    }
    func corpus() -> SavedCorpus {
        let c = SavedCorpus(heldPages: nil)
        c.setSource("t") { (0..<6).map { SavedCorpus.Document(id: "t\($0)", title: "Title \($0)", address: "pub:\($0)", text: "story text number \($0) about bridges", kind: "story", source: "Autopaper", date: Date(timeIntervalSince1970: 1_800_000_000 + Double($0))) } }
        return c
    }

    func testDownloadsOnceOnWiFiThenEmbedsEverythingAndFusesRetrieval() async throws {
        let b = Box(); let c = corpus(); let s = service(b, dir())
        s.attach(corpus: c); s.kick(); await s.waitIdle()
        XCTAssertEqual(b.get { b.downloads }, 1)
        XCTAssertEqual(s.progress.done, 6); XCTAssertEqual(s.progress.total, 6)
        XCTAssertEqual(s.modelState, .ok)
        XCTAssertTrue(c.retriever is FusedRetriever)
        s.kick(); await s.waitIdle()
        XCTAssertEqual(b.get { b.downloads }, 1)                                 // once
        XCTAssertTrue(b.get { b.lines }.contains { $0.hasPrefix("AUTORADIO embed model=ok items=6/6") }, "\(b.get { b.lines })")
    }
    func testNoDownloadOffWiFi() async throws {
        let b = Box(); b.set { b.wifi = false }
        let s = service(b, dir()); s.attach(corpus: corpus()); s.kick(); await s.waitIdle()
        XCTAssertEqual(b.get { b.downloads }, 0); XCTAssertEqual(s.modelState, .absent); XCTAssertEqual(s.progress.done, 0)
    }
    func testLowPowerAndThermalSeriousPauseTheJob() async throws {
        let b = Box(); b.set { b.present = true; b.lowPower = true }
        let s = service(b, dir()); s.attach(corpus: corpus()); s.kick()
        try await Task.sleep(for: .milliseconds(150))
        XCTAssertEqual(s.progress.done, 0)
        b.set { b.lowPower = false; b.thermal = true }
        try await Task.sleep(for: .milliseconds(100)); XCTAssertEqual(s.progress.done, 0)
        b.set { b.thermal = false }
        await s.waitIdle(); XCTAssertEqual(s.progress.done, 6)
    }
    func testSpeechOnAirPausesTheJob() async throws {
        let b = Box(); b.set { b.present = true; b.busy = true }
        let s = service(b, dir()); s.attach(corpus: corpus()); s.kick()
        try await Task.sleep(for: .milliseconds(150)); XCTAssertEqual(s.progress.done, 0)
        b.set { b.busy = false }
        await s.waitIdle(); XCTAssertEqual(s.progress.done, 6)
    }
    func testWipeRemovesVectorsAndGoesBackToBM25() async throws {
        let b = Box(); b.set { b.present = true }
        let v = dir(); let c = corpus(); let s = service(b, v)
        s.attach(corpus: c); s.kick(); await s.waitIdle()
        XCTAssertTrue(FileManager.default.fileExists(atPath: v.appendingPathComponent("vectors.f16").path))
        c.reset()                                                                // sign-out
        XCTAssertFalse(FileManager.default.fileExists(atPath: v.appendingPathComponent("vectors.f16").path))
        XCTAssertEqual(s.progress.done, 0)
        XCTAssertFalse((c.retriever as? FusedRetriever)?.usesVectors ?? false)
    }
    func testNewestItemsAreEmbeddedFirst() async throws {
        let b = Box(); b.set { b.present = true }
        let c = corpus()
        let ids = { c.refresh(); return EmbeddingService.items(from: c.retrieval) }().map(\.id)
        XCTAssertEqual(ids.first, "t5"); XCTAssertEqual(ids.last, "t0")
        _ = b
    }
    func testVectorsDirectoryIsExcludedFromBackup() async throws {
        let b = Box(); b.set { b.present = true }
        let v = dir(); let s = service(b, v); s.attach(corpus: corpus()); s.kick(); await s.waitIdle()
        let rv = try v.resourceValues(forKeys: [.isExcludedFromBackupKey])
        XCTAssertEqual(rv.isExcludedFromBackup, true)
    }
}
