import XCTest
@testable import AutoradioKit

// The voice turn's side of retrieval (AutoradioKit/RetrievalFacts.swift). The ranking is search.test.ts; the runtime is retrieval-run.test.ts.
// Failure list: RetrievalFacts.swift header (1 to 4), and what the model is shown (the old RetrievalHolderTests).
final class RetrievalFactsTests: XCTestCase {
    static func wire(id: String = "x") -> [String: Any] {
        ["id": id, "text": "Today is 2026-11-14.\nPassage 1 | story | 2026-11-13 | Autopaper | id pub:a | \"Senate passes the water bill\"\nThe Senate voted.\n", "brief": "On air now: Harbor.",
         "items": [["id": "pub:a", "address": "pub:a", "kind": "story", "title": "Senate passes the water bill", "source": "Autopaper", "date": 1_794_600_000_000, "place": "", "score": 1.5, "text": "The Senate voted."]],
         "record": ["at": 1_794_700_000_000, "query": "senate", "onAir": "", "terms": ["senate"], "usedOnAir": false, "considered": 1, "indexed": 3,
                    "hits": [["id": "pub:a", "address": "pub:a", "kind": "story", "score": 1.5, "date": 1_794_600_000_000, "chars": 17]], "budgetChars": 3584, "usedChars": 120, "sources": ["Autopaper"]]]
    }
    func bridge(deadline: Double = 2) -> RetrievalBridge { let b = RetrievalBridge(); b.deadline = deadline; return b }

    func testTheAnswerOfSharedCodeIsTheContextTheTurnReads() async throws {
        let b = bridge()
        b.raise = { [unowned b] in guard let a = b.current else { return }; b.answer(id: a.id, context: Self.wire(id: a.id)) }
        let asked = await b.ask(words: "tell me about the Senate", pageTitle: "Autoradio", pageAddress: "radio:")
        let c = try XCTUnwrap(asked)
        XCTAssertEqual(c.items.map(\.id), ["pub:a"]); XCTAssertEqual(c.brief, "On air now: Harbor."); XCTAssertEqual(c.record.sources, ["Autopaper"])
        XCTAssertEqual(c.items[0].date, Date(timeIntervalSince1970: 1_794_600_000)); XCTAssertEqual(c.record.at, Date(timeIntervalSince1970: 1_794_700_000))
        XCTAssertEqual(b.lastRecord, c.record, "a kept turn names what the model was shown")
        XCTAssertNil(b.current, "an answered ask is no longer raised")
    }
    func testTheAskCarriesWhatTheTurnAsksAndWhatOnlyNativeHolds() async {
        let b = bridge(deadline: 0.05)
        b.facts = { address in "{\"pageText\":\"for \(address)\"}" }
        var seen: RetrievalAsk?
        b.raise = { [unowned b] in seen = b.current }
        _ = await b.ask(words: "senate", pageTitle: "T", pageAddress: "pub:x", now: Date(timeIntervalSince1970: 100))
        XCTAssertEqual(seen?.words, "senate"); XCTAssertEqual(seen?.pageTitle, "T"); XCTAssertEqual(seen?.pageAddress, "pub:x")
        XCTAssertEqual(seen?.now, 100_000); XCTAssertEqual(seen?.facts, "{\"pageText\":\"for pub:x\"}"); XCTAssertFalse(seen?.id.isEmpty ?? true)
    }
    func testNoAnswerInTimeIsNoContextNeverAWaitForEver_1() async {
        let b = bridge(deadline: 0.1)
        let t0 = Date()
        let c = await b.ask(words: "senate", pageTitle: "", pageAddress: "")
        XCTAssertNil(c); XCTAssertLessThan(Date().timeIntervalSince(t0), 1.5)
    }
    func testAnAnswerThatIsNotAContextIsNoContext_2() async {
        let b = bridge()
        b.raise = { [unowned b] in guard let a = b.current else { return }; b.answer(id: a.id, context: ["id": a.id, "items": "no"]) }
        let c = await b.ask(words: "senate", pageTitle: "", pageAddress: "")
        XCTAssertNil(c)
    }
    func testAnAnswerForAnAskNoLongerWaitingIsDropped_3() async {
        let b = bridge(deadline: 0.05)
        _ = await b.ask(words: "senate", pageTitle: "", pageAddress: "")
        let id = "late"; b.answer(id: id, context: Self.wire(id: id))
        XCTAssertNil(b.lastRecord, "a late answer is not what the model was shown")
    }
    func testASecondAskDropsTheFirst_4() async {
        let b = bridge(deadline: 2)
        var firstID = ""
        b.raise = { [unowned b] in if firstID.isEmpty { firstID = b.current?.id ?? "" } }
        async let first = b.ask(words: "one", pageTitle: "", pageAddress: "")
        try? await Task.sleep(nanoseconds: 100_000_000)
        b.raise = { [unowned b] in guard let a = b.current else { return }; b.answer(id: a.id, context: Self.wire(id: a.id)) }
        let second = await b.ask(words: "two", pageTitle: "", pageAddress: "")
        let one = await first
        XCTAssertNil(one, "the turn that asked it was cancelled"); XCTAssertNotNil(second)
    }
    func testTheNothingLineNamesWhatWasSearched() {
        XCTAssertEqual(SavedAnswerPrompt.nothingLine(searched: ["Autopaper", "Reader", "Podcasts"]), "I have nothing on that in the paper, your books or your podcasts.")
        XCTAssertEqual(SavedAnswerPrompt.nothingLine(searched: ["Autopaper", "Autopaper"]), "I have nothing on that in the paper.")
        XCTAssertEqual(SavedAnswerPrompt.nothingLine(searched: []), "I have nothing saved on this phone to search yet.")
    }
    func testTheModelIsShownRetrievedItemsWithDatesAndIdsAndTheContextCarriesTheRecord() async throws {
        let m = FakeLocalModel(pieces: ["The Senate passed the water bill. "])
        let docs = [FakeDoc(id: "front:a", title: "Senate passes the water bill", address: "pub:a", text: "The Senate voted on Tuesday to pass the water bill.", date: Date(timeIntervalSince1970: 1_794_000_000)),
                    FakeDoc(id: "front:b", title: "Harbor strike ends", address: "pub:b", text: "Dock workers ended the strike.", date: Date(timeIntervalSince1970: 1_794_500_000))]
        let h = SavedAnswerHolder(StageParams(), retrieval: FakeRetrieval.provider(docs), model: { m })
        var lines: [String] = []
        for await l in h.lines(for: HolderInput(listenerWords: "tell me about the Senate", answers: [], pageTitle: "Harbor strike ends")) { lines.append(l) }
        let prefix = m.asked.first?.prefix ?? ""
        XCTAssertTrue(prefix.contains("id pub:a"), prefix); XCTAssertFalse(prefix.contains("Dock workers"), "the story on air is not the answer to a Senate question")
        XCTAssertTrue(lines.first?.hasPrefix("From the paper's story on Senate passes the water bill") ?? false)
        let rec = FakeRetrieval.context(docs, "x").record
        let ctx = TurnContextFile(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: ""), retrieval: rec)
        let back = try JSONDecoder().decode(TurnContextFile.self, from: JSONEncoder().encode(ctx))
        XCTAssertEqual(back.retrieval, rec)
        var o = try JSONSerialization.jsonObject(with: JSONEncoder().encode(ctx)) as! [String: Any]; o["retrieval"] = nil
        XCTAssertNil(try JSONDecoder().decode(TurnContextFile.self, from: JSONSerialization.data(withJSONObject: o)).retrieval, "old files still decode")
    }
    func testAHolderThatGetsNoAnswerSaysThePlainHoldingLine() async {
        let m = FakeLocalModel(pieces: ["never asked"])
        let h = SavedAnswerHolder(StageParams(), retrieval: { _, _, _ in nil }, model: { m })
        var lines: [String] = []
        for await l in h.lines(for: HolderInput(listenerWords: "tell me about the Senate", answers: [])) { lines.append(l) }
        XCTAssertEqual(lines.count, 1); XCTAssertTrue(lines[0].hasPrefix("Let me look that up")); XCTAssertTrue(m.asked.isEmpty)
    }
}
