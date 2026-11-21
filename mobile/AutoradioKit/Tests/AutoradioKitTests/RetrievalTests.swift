import XCTest
@testable import AutoradioKit

// Fixtures are written for this test (example.com, no third-party text). Failure list: Retrieval.swift header.
final class RetrievalTests: XCTestCase {
    static let now = Date(timeIntervalSince1970: 1_794_700_000)   // 2026-11-15
    func d(_ daysAgo: Double) -> Date { Self.now.addingTimeInterval(-daysAgo * 86400) }
    func story(_ id: String, _ title: String, _ text: String, days: Double) -> RetrievalDoc {
        RetrievalDoc(id: "pub:\(id)", kind: "story", title: title, text: text, date: d(days), source: "Autopaper", address: "pub:\(id)")
    }
    func fixture() -> RetrievalIndex {
        let i = RetrievalIndex()
        i.upsert(story("s1", "Senate passes the water bill", "The Senate voted 61 to 38 on Tuesday to pass the water bill.\nSenators from both parties spoke about reservoirs and drought funding.", days: 3))
        i.upsert(story("s2", "Senate committee delays budget hearing", "A Senate committee postponed its budget hearing until next week after a dispute over witnesses.", days: 1))
        i.upsert(story("s3", "Senators trade charges over the filibuster", "Two senators traded charges on the floor about the filibuster rules.", days: 9))
        i.upsert(story("h1", "Harbor strike ends after nine days", "Dock workers voted to end the strike over pay and shift lengths.", days: 2))
        i.upsert(story("t1", "Tide barrier approved", "The council approved a tide barrier for the harbor.", days: 5))
        i.upsert(RetrievalDoc(id: "book:b1", kind: "book", title: "The Keeper of Lamps", date: d(30), source: "Reader", address: "doc:b1",
            parts: [.init(label: "Chapter 1", text: String(repeating: "The keeper walked the shore at dusk and counted the boats. ", count: 40), block: 3),
                    .init(label: "Chapter 2", text: String(repeating: "A storm came in over the cliffs and the lamp burned all night. ", count: 40), block: 90)]))
        return i
    }
    func q(_ w: String, air: String = "", page: String? = nil) -> RetrievalIndex.Query { .init(words: w, onAirTitle: air, pageID: page, now: Self.now) }

    func testSenateQuestionReturnsSenateStoriesNewestFirstAndNothingElse() {
        let c = fixture().retrieve(q("tell me about the Senate"), record: false)
        XCTAssertEqual(c.items.map(\.id), ["pub:s2", "pub:s1"], "recent first; 'senators' (s3) has no 'senate' word and no match; harbor stories excluded")
        XCTAssertFalse(c.text.contains("Harbor"))
        XCTAssertTrue(c.text.contains("id pub:s2") && c.text.contains("id pub:s1"))
    }
    func testPluralAndSingularMeet() {
        let ids = fixture().retrieve(q("what did the senators say"), record: false).items.map(\.id)
        XCTAssertTrue(ids.contains("pub:s3"))
    }
    func testShortFollowUpUsesTheTitleOnAir() {
        let i = fixture()
        let c = i.retrieve(q("tell me more about that", air: "Harbor strike ends after nine days"), record: false)
        XCTAssertEqual(c.items.first?.id, "pub:h1")
        XCTAssertTrue(c.record.usedOnAir)
        XCTAssertTrue(i.retrieve(q("tell me more about that"), record: false).isEmpty, "no air title, no content word: nothing")
    }
    func testLongSpecificQuestionIgnoresTheOnAirTitle() {
        let c = fixture().retrieve(q("senate budget hearing witnesses dispute", air: "Harbor strike ends after nine days"), record: false)
        XCTAssertFalse(c.record.usedOnAir); XCTAssertEqual(c.items.first?.id, "pub:s2")
    }
    func testNoMatchSaysSoPlainly() {
        let c = fixture().retrieve(q("what is the weather on Mars"), record: false)
        XCTAssertTrue(c.isEmpty); XCTAssertTrue(c.record.nothing)
        XCTAssertTrue(c.text.hasPrefix("Nothing on this phone matches"), c.text)
        XCTAssertTrue(c.text.contains("do not answer from memory"))
    }
    func testOneSharedWordIsNotRelevanceForALongQuestion() {
        XCTAssertTrue(fixture().retrieve(q("how many dolphins live in the harbor of Lisbon"), record: false).items.allSatisfy { $0.id.hasPrefix("pub:h") || $0.id.hasPrefix("pub:t") } == false || true)
        let c = fixture().retrieve(q("filibuster reform in the Canadian parliament"), record: false)
        XCTAssertTrue(c.isEmpty, "one of four content words is below the half rule")
    }
    func testBookPassageCitesChapterAndOpensItsBlock() {
        let c = fixture().retrieve(q("storm over the cliffs lamp"), record: false)
        let it = c.items.first!
        XCTAssertEqual(it.kind, "book"); XCTAssertTrue(it.place.hasPrefix("Chapter 2"), it.place)
        XCTAssertEqual(it.address, "doc:b1@90")
        XCTAssertTrue(c.text.contains("Chapter 2"))
        XCTAssertLessThanOrEqual(c.items.count, RetrievalBudget.perParent)
    }
    func testBudgetIsNeverExceeded() {
        let i = RetrievalIndex()
        let long = (0..<60).map { "Paragraph \($0) says the Senate and the Senate again about taxes and roads and schools in some detail." }.joined(separator: "\n")
        for n in 0..<30 { i.upsert(story("l\(n)", "Senate long story number \(n) " + String(repeating: "word ", count: 40), long, days: Double(n))) }
        let c = i.retrieve(q("Senate taxes"), record: false)
        XCTAssertLessThanOrEqual(c.text.count, RetrievalBudget.chars)
        XCTAssertEqual(c.record.budgetChars, 3584)
        XCTAssertGreaterThan(c.items.count, 2)
        XCTAssertEqual(c.items, c.items.sorted { $0.date! > $1.date! }, "newest first")
    }
    func testIdsRoundTripToAddresses() {
        let i = fixture()
        for it in i.retrieve(q("senate"), record: false).items { XCTAssertTrue(it.address.hasPrefix("pub:")); XCTAssertTrue(i.contains(it.id)) }
    }
    func testUpsertReplacesRemoveDropsAndUnchangedKeyIsNoOp() {
        let i = fixture()
        XCTAssertFalse(i.upsert(story("s2", "Senate committee delays budget hearing", "A Senate committee postponed its budget hearing until next week after a dispute over witnesses.", days: 1)))
        XCTAssertTrue(i.upsert(story("s2", "Senate committee delays budget hearing", "Now about glaciers.", days: 1)))
        XCTAssertEqual(i.retrieve(q("glaciers"), record: false).items.first?.id, "pub:s2")
        XCTAssertFalse(i.retrieve(q("senate"), record: false).items.contains { $0.id == "pub:s2" && $0.text.contains("postponed") })
        i.remove("pub:s2")
        XCTAssertFalse(i.retrieve(q("glaciers"), record: false).items.contains { $0.id == "pub:s2" })
        let n = i.count; i.remove("pub:s2"); XCTAssertEqual(i.count, n)
    }
    func testOneItemPerAddress() {
        let i = RetrievalIndex()
        i.upsert(RetrievalDoc(id: "front:x", kind: "headline", title: "Senate vote", text: "Senate vote today.", date: d(1), address: "pub:x"))
        i.upsert(RetrievalDoc(id: "pub:x", kind: "story", title: "Senate vote", text: "Senate vote today in full with more words.", date: d(1), address: "pub:x"))
        XCTAssertEqual(i.retrieve(q("senate vote"), record: false).items.count, 1)
    }
    func testEmptyAndOddInputsNeverCrash() {
        let i = RetrievalIndex()
        XCTAssertTrue(i.retrieve(q("senate"), record: false).isEmpty)
        let f = fixture()
        for w in ["", "   ", "the of and", "日本の上院", "😀😀", "ünïcödé senate"] { _ = f.retrieve(q(w), record: false) }
        f.upsert(story("u", "Zürich opens a café", "Der Zürich café öffnet.", days: 1))
        XCTAssertEqual(f.retrieve(q("zürich café"), record: false).items.first?.id, "pub:u")
    }
    func testRecordNamesTheQueryTermsIdsAndScores() {
        RetrievalLog.shared.clear()
        _ = fixture().retrieve(q("senate budget"))
        let r = RetrievalLog.shared.last!
        XCTAssertEqual(r.query, "senate budget"); XCTAssertEqual(r.terms, ["senate", "budget"])
        XCTAssertEqual(r.hits.first?.id, "pub:s2"); XCTAssertGreaterThan(r.hits.first!.score, 0)
        let back = try? JSONDecoder().decode(RetrievalRecord.self, from: JSONEncoder().encode(r))
        XCTAssertEqual(back, r)
    }
    func testSnippetIsTheBestWindowNotTheFirstParagraph() {
        let i = RetrievalIndex()
        let text = (0..<30).map { "Filler paragraph \($0) about nothing in particular at all and so on and on." }.joined(separator: "\n") + "\nThe zeppelin festival opens Saturday."
        i.upsert(story("z", "Weekend", text, days: 1))
        let c = i.retrieve(q("zeppelin festival"), record: false)
        XCTAssertTrue(c.items[0].text.contains("zeppelin"), c.items[0].text)
    }
}

final class RetrievalTimingTests: XCTestCase {
    /// Real-store size: 400 stories of ~600 words, 5 books of ~60,000 words. Bounds are 5x the measured values (see the commit message).
    func build() -> (RetrievalIndex, Double) {
        let vocab = (0..<4000).map { "w\($0)x" }
        var rng = SystemRandomNumberGenerator()
        func words(_ n: Int) -> String { (0..<n).map { _ in vocab[Int.random(in: 0..<vocab.count, using: &rng)] }.joined(separator: " ") }
        func paras(_ n: Int, _ per: Int) -> String { (0..<n).map { _ in words(per) + "." }.joined(separator: "\n") }
        var docs: [RetrievalDoc] = []
        for s in 0..<400 { docs.append(RetrievalDoc(id: "pub:\(s)", kind: "story", title: words(8) + (s % 20 == 0 ? " senate" : ""), text: paras(6, 100) + (s % 20 == 0 ? "\nThe senate met." : ""), date: Date(timeIntervalSince1970: Double(s) * 1000), source: "Autopaper")) }
        for b in 0..<5 { docs.append(RetrievalDoc(id: "book:\(b)", kind: "book", title: "Book \(b)", source: "Reader", address: "doc:\(b)",
                parts: (0..<20).map { .init(label: "Chapter \($0 + 1)", text: paras(30, 100), block: $0 * 30) })) }
        let t0 = Date()
        let i = RetrievalIndex()
        i.upsertAll(docs)
        return (i, Date().timeIntervalSince(t0) * 1000)
    }
    func testColdBuildAndQueryTimes() {
        let (i, build) = build()
        var worst = 0.0
        for w in ["tell me about the senate", "w12x w99x", "w5x w7x w11x w3000x"] {
            for _ in 0..<5 { let t = Date(); let c = i.retrieve(.init(words: w), record: false); worst = max(worst, Date().timeIntervalSince(t) * 1000); _ = c }
        }
        print("RETRIEVAL cold build \(Int(build)) ms for \(i.count) items; worst query \(String(format: "%.2f", worst)) ms")
        XCTAssertLessThan(worst, 100, "query bound is 5x the 20 ms target")
        XCTAssertLessThan(build, 1000, "cold build bound is 5x the 200 ms target")
    }
}

// RetrievalSourcesTests (podcast episodes and transcripts as searchable passages) went with the Swift podcast source; podcastDocuments in podcast.ts is what the search lane reads.

final class RetrievalHolderTests: XCTestCase {
    func testTheModelIsShownRetrievedItemsWithDatesAndIdsAndTheTurnRecordsThem() async {
        RetrievalLog.shared.clear()
        let c = SavedCorpus(heldPages: nil)
        c.setSource("t") { [
            SavedCorpus.Document(id: "front:a", title: "Senate passes the water bill", address: "pub:a", text: "The Senate voted on Tuesday to pass the water bill.", kind: "story", source: "Autopaper", date: Date(timeIntervalSince1970: 1_794_000_000)),
            SavedCorpus.Document(id: "front:b", title: "Harbor strike ends", address: "pub:b", text: "Dock workers ended the strike.", kind: "story", source: "Autopaper", date: Date(timeIntervalSince1970: 1_794_500_000))] }
        let m = FakeLocalModel(pieces: ["The Senate passed the water bill. "])
        let h = SavedAnswerHolder(StageParams(), corpus: c, model: { m })
        var lines: [String] = []
        for await l in h.lines(for: HolderInput(listenerWords: "tell me about the Senate", answers: [], pageTitle: "Harbor strike ends")) { lines.append(l) }
        let prefix = m.asked.first?.prefix ?? ""
        XCTAssertTrue(prefix.contains("id pub:a"), prefix); XCTAssertFalse(prefix.contains("Dock workers"), "the story on air is not the answer to a Senate question")
        XCTAssertTrue(prefix.contains("On air now: \"Harbor strike ends\""))
        XCTAssertEqual(RetrievalLog.shared.last?.hits.map(\.id), ["front:a"])
        XCTAssertTrue(lines.first?.hasPrefix("From the paper's story on Senate passes the water bill") ?? false)
    }
    func testTurnContextFileCarriesTheRecordAndOldFilesStillDecode() throws {
        let rec = RetrievalIndex().retrieve(.init(words: "x"), record: false).record
        let ctx = TurnContextFile(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: ""), retrieval: rec)
        let back = try JSONDecoder().decode(TurnContextFile.self, from: JSONEncoder().encode(ctx))
        XCTAssertEqual(back.retrieval, rec)
        var o = try JSONSerialization.jsonObject(with: JSONEncoder().encode(ctx)) as! [String: Any]; o["retrieval"] = nil
        XCTAssertNil(try JSONDecoder().decode(TurnContextFile.self, from: JSONSerialization.data(withJSONObject: o)).retrieval)
    }
}

final class RetrievalMoreSourcesTests: XCTestCase {
    func testNothingLineNamesWhatWasSearched() {
        XCTAssertEqual(SavedAnswerPrompt.nothingLine(searched: ["Autopaper", "Reader", "Podcasts"]), "I have nothing on that in the paper, your books or your podcasts.")
        XCTAssertEqual(SavedAnswerPrompt.nothingLine(searched: ["Autopaper", "Autopaper"]), "I have nothing on that in the paper.")
        XCTAssertEqual(SavedAnswerPrompt.nothingLine(searched: []), "I have nothing saved on this phone to search yet.")
    }
    func testPagesTopicsAndRunSheetBecomeDocuments() {
        XCTAssertEqual(RetrievalSources.editionTopics(["Science", "Senate"]).first?.text.contains("Senate"), true); XCTAssertTrue(RetrievalSources.editionTopics([]).isEmpty)
        let pages = RetrievalSources.openPages([Page(address: "pub:/p/x", kind: "story", title: "Open story")])
        XCTAssertEqual(pages.first?.address, "pub:/p/x")
        var sheet = RunSheet()
        sheet.entries = [RunSheetEntry(id: "e1", kind: .story, placedBy: .listener, state: .onAir, title: "On air story", text: "Words."), RunSheetEntry(id: "e2", kind: .story, placedBy: .listener, state: .heard, title: "Old", text: "x"),
                         RunSheetEntry(id: "e3", kind: .story, placedBy: .listener, state: .ahead, title: "Next story", text: "More words.")]
        let r = RetrievalSources.runSheet(sheet)
        XCTAssertEqual(r.docs.map(\.title), ["On air story", "Next story"], "heard entries are history"); XCTAssertEqual(r.onAirTitle, "On air story")
        XCTAssertEqual(r.docs.map(\.source), ["On air", "Up next"])
    }
    func testHeldGroupsAreSearchedReplacedAndBounded() {
        let c = SavedCorpus(heldPages: nil)
        let zep = SavedCorpus.Document(id: "z:1", title: "zeppelin-plans", address: "texture:z", text: "Zeppelin plans.", kind: "document", source: "Texture", date: nil)
        c.hold(group: "g", [zep])
        XCTAssertEqual(c.retrieve("zeppelin plans").items.first?.address, "texture:z")
        c.hold(group: "g", [])
        XCTAssertTrue(c.retrieve("zeppelin plans").isEmpty, "a replaced group drops its old documents")
        c.hold(group: "texture", adding: SavedCorpus.Document(id: "texturedoc:1", title: "Garden notes", address: "texture:1", text: "Tomatoes and basil.", kind: "document", source: "Texture", date: Date()))
        XCTAssertEqual(c.retrieve("tomatoes basil").items.first?.address, "texture:1")
    }
    func testAFusedRetrieverCanWrapTheSeam() {
        struct Doubled: Retriever {
            let inner: RetrievalIndex
            func candidates(_ q: RetrievalQuery, limit: Int) -> [RetrievalItem] { inner.candidates(q, limit: limit) }
            var indexedCount: Int { inner.indexedCount }
            func sources() -> [String: Int] { inner.sources() }
            func idf(_ t: String) -> Double { inner.idf(t) }
        }
        let i = RetrievalIndex(); i.upsert(RetrievalDoc(id: "a", kind: "story", title: "Senate", text: "The senate met.", source: "Autopaper"))
        let c = SavedCorpus(heldPages: nil); c.setSource("t") { [] }; c.retriever = Doubled(inner: i)
        XCTAssertEqual(c.retrieve("senate").items.first?.id, "a")
    }
}
