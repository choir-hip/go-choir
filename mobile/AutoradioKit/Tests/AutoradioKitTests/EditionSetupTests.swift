import XCTest
@testable import AutoradioKit

// The failure list is at the top of EditionSetup.swift (E1..E9).
final class EditionSetupTests: XCTestCase {
    let sug = EditionTopics.suggestions()

    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("edition-\(UUID().uuidString).json") }

    func testSuggestionsAreGenerousAndStartWithTheHouseSections() {
        XCTAssertGreaterThanOrEqual(sug.count, 18)
        XCTAssertEqual(Array(sug.prefix(10)), ["World", "U.S.", "Politics", "Business", "Technology", "Science", "Health", "Climate", "Culture", "Sports"])
        XCTAssertEqual(Set(sug.map(EditionTopics.key)).count, sug.count, "no duplicates")
    }

    func testLiveSectionsAreMergedWithoutDuplicates() {
        let s = EditionTopics.suggestions(liveSections: ["tech_ai", "world", "housing_market"])
        XCTAssertEqual(s.filter { $0 == "Technology" }.count, 1)
        XCTAssertTrue(s.contains("Housing market"))
        XCTAssertEqual(EditionTopics.label(forSectionID: "us"), "U.S.")
    }

    func testNoSuggestionOrLineAsksWhatTheReaderDoesNotWant() {
        let words = (sug + [EditionSetupText.explainer, EditionSetupText.voiceHint, EditionSetupText.emptyLine, EditionSetupText.spokenOpen]).joined(separator: " ").lowercased()
        for bad in ["tired of", "do not want", "don't want", "avoid", "less of", "anything you"] { XCTAssertFalse(words.contains(bad), bad) }
        XCTAssertTrue(EditionSetupText.explainer.contains("any time"))
    }

    func testE1TheSameTopicTwiceIsOne() {
        var s = EditionSelection()
        s.toggle("Space", among: sug); XCTAssertEqual(s.topics, ["Space"])
        XCTAssertNil(s.add("space", among: sug)); XCTAssertNil(s.add(" SPACE ", among: sug))
        XCTAssertEqual(s.add("tech", among: sug), "Technology")
        XCTAssertNil(s.add("technology", among: sug)); XCTAssertNil(s.add("Spaces", among: sug))
        XCTAssertEqual(s.topics, ["Space", "Technology"])
        s.toggle("space", among: sug); XCTAssertFalse(s.contains("Space"), "a second tap deselects")
        var u = EditionSelection(); _ = u.add("us", among: sug); XCTAssertEqual(u.topics, ["U.S."]); XCTAssertNil(u.add("U.S.", among: sug))
        var c = EditionSelection(); _ = c.add("Chess", among: sug); XCTAssertNil(c.add("chess", among: sug))
    }

    func testE2SpeechNamesTopicsNotInTheSuggestions() {
        var s = EditionSelection()
        let o = s.apply(spoken: "add space and chess", among: sug)
        XCTAssertEqual(o.added, ["Space", "Chess"])
        XCTAssertEqual(o.reply, "Added Space and Chess.")
        XCTAssertEqual(s.topics, ["Space", "Chess"])
        let o2 = s.apply(spoken: "also sourdough baking, and the tech", among: sug)
        XCTAssertEqual(o2.added, ["Sourdough baking", "Technology"], "\(o2)")
    }

    func testSpokenForms() {
        XCTAssertEqual(SpokenTopics.parse("I'd like to hear about space").adds, ["space"])
        XCTAssertEqual(SpokenTopics.parse("Space.").adds, ["space"])
        XCTAssertEqual(SpokenTopics.parse("please add the U.S. and politics").adds, ["us", "politics"])
        XCTAssertEqual(SpokenTopics.parse("drop chess").removes, ["chess"])
        XCTAssertEqual(SpokenTopics.parse("take off sports and music").removes, ["sports", "music"])
        XCTAssertTrue(SpokenTopics.parse("continue").finish)
        XCTAssertTrue(SpokenTopics.parse("that's all").finish)
        XCTAssertTrue(SpokenTopics.parse("tell me about science, health and film").adds == ["science", "health", "film"])
    }

    func testE5QuestionsAndLongSentencesAreNotTopics() {
        XCTAssertTrue(SpokenTopics.parse("what is this for?").isEmpty)
        XCTAssertTrue(SpokenTopics.parse("how does this work").isEmpty)
        XCTAssertTrue(SpokenTopics.parse("I was thinking about whether the weather today will affect anything at all").isEmpty)
        XCTAssertTrue(SpokenTopics.parse("").isEmpty)
        XCTAssertTrue(SpokenTopics.parse("um").isEmpty)
        var s = EditionSelection()
        XCTAssertTrue(s.apply(spoken: "what is this", among: sug).reply.contains("Tell me a topic"))
        XCTAssertTrue(s.topics.isEmpty)
    }

    func testE8NegativeWordsOnlyTakeOffWhatIsThere() {
        var s = EditionSelection(topics: ["Sports", "Music"])
        _ = s.apply(spoken: "not sports", among: sug)
        XCTAssertEqual(s.topics, ["Music"])
        let o = s.apply(spoken: "no gossip", among: sug)
        XCTAssertEqual(s.topics, ["Music"]); XCTAssertTrue(o.added.isEmpty)
        XCTAssertTrue(s.apply(spoken: "nothing", among: sug).added.isEmpty)
        XCTAssertTrue(s.apply(spoken: "none", among: sug).removed.isEmpty)
    }

    func testE3LongOrEmptyTypedTopics() {
        var s = EditionSelection()
        XCTAssertTrue(s.addTyped("   ", among: sug).isEmpty)
        XCTAssertTrue(s.addTyped("...,;!", among: sug).isEmpty)
        let long = Array(repeating: "harbour", count: 40).joined(separator: " ")
        let added = s.addTyped(long, among: sug)
        XCTAssertEqual(added.count, 1)
        XCTAssertLessThanOrEqual(added[0].count, EditionTopics.maxTopicLength)
        XCTAssertFalse(added[0].hasSuffix(" "))
        XCTAssertEqual(s.addTyped("space, chess; Arts and crafts", among: sug), ["Space", "Chess", "Arts and crafts"])
        XCTAssertEqual(s.addTyped("a\u{0007}b", among: sug), ["Ab"], "control characters are dropped")
    }

    func testE4NoMinimumAndNoMaximum() {
        let w = LocalEditionWriter.edition(topics: [])
        XCTAssertEqual(w.sections.first?.lines, ["The whole paper, until I say otherwise."])
        XCTAssertTrue(EditionSetupText.selectionLine([]).contains("general edition"))
        var s = EditionSelection()
        for t in sug { s.toggle(t, among: sug) }
        XCTAssertEqual(s.topics.count, sug.count, "every suggestion can be selected")
        XCTAssertEqual(LocalEditionWriter.edition(topics: s.topics).sections[0].lines.count, sug.count)
        XCTAssertTrue(EditionSetupText.selectionLine(["Space"]).contains("Space"))
        XCTAssertTrue(EditionSetupText.selectionLine(["Space", "Chess", "Film"]).contains("3 topics"))
    }

    func testE6E7SavedLocallyAndShownAgain() {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f) }
        let a = EditionSetupSession(suggestions: sug, file: f)
        XCTAssertFalse(a.hasSaved)
        a.toggle("Space"); _ = a.addTyped("Chess"); _ = a.hear("add film")
        XCTAssertTrue(a.hasSaved)
        let b = EditionSetupSession(suggestions: sug, file: f)        // coming back to the step
        XCTAssertEqual(b.topics, ["Space", "Chess", "Film"])
        let chips = b.chips()
        XCTAssertEqual(chips.first?.label, "Chess", "the reader's own topic is a chip, first")
        XCTAssertTrue(chips.first { $0.label == "Space" }!.selected)
        XCTAssertFalse(chips.first { $0.label == "World" }!.selected)
    }

    func testE7CorruptFileStartsEmpty() throws {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f) }
        try Data("{not json".utf8).write(to: f)
        XCTAssertEqual(EditionSetupSession(suggestions: sug, file: f).topics, [])
        try Data().write(to: f)
        XCTAssertEqual(EditionSetupSession(suggestions: sug, file: f).topics, [])
    }

    func testE9ADeselectedOwnTopicStaysAsAChip() {
        let s = EditionSetupSession(suggestions: sug, file: nil)
        _ = s.addTyped("Chess"); s.toggle("Chess")
        XCTAssertEqual(s.topics, [])
        let c = s.chips().first { $0.label == "Chess" }
        XCTAssertNotNil(c); XCTAssertFalse(c!.selected); XCTAssertTrue(c!.own)
    }

    func testFinishByVoiceCallsBack() {
        let s = EditionSetupSession(suggestions: sug, file: nil)
        nonisolated(unsafe) var done = false
        s.onFinish = { done = true }
        XCTAssertEqual(s.hear("continue"), "Okay.")
        XCTAssertTrue(done)
    }

    func testTheProgrammeAndTheDeskUseTheSession() async throws {
        let s = EditionSetupSession(suggestions: sug, file: nil)
        EditionSetupSession.shared = s; defer { EditionSetupSession.shared = nil }
        let doc = try await EditionSetupProgramme(StageParams()).load()
        XCTAssertEqual(doc.blocks.compactMap(\.spokenText), [EditionSetupText.spokenOpen])
        var got: [Revision] = []
        for try await r in EditionSetupDesk(StageParams()).submit(DeskTurn(turnID: "t1", programmeID: "p", blockIndex: 0, listenerWords: "add space and chess")) { got.append(r) }
        XCTAssertEqual(s.topics, ["Space", "Chess"])
        XCTAssertEqual(got.first?.blocks.compactMap(\.spokenText), ["Added Space and Chess."])
    }

    func testTheEditionIsWrittenWithoutANetworkFromTheChoices() async throws {
        let e = try await LocalEditionWriter().write(topics: ["Space", "Chess"]) { _ in }
        XCTAssertEqual(e.title, "My edition")
        XCTAssertEqual(e.lines, ["What I follow", "Space", "Chess"])
    }
}

// C1..C4: finishing always ends.
final class EditionCompletionTests: XCTestCase {
    struct Throws: EditionWriter { func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition { throw EditionError.failed("no") } }
    struct Never: EditionWriter {
        func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
            try await Task.sleep(nanoseconds: 60_000_000_000); return LocalEditionWriter.edition(topics: [])
        }
    }

    func testSuccess() async {
        let o = await EditionCompletion.run(topics: ["Space"], writer: LocalEditionWriter()) { _ in }
        guard case .written(let e) = o else { return XCTFail("\(o)") }
        XCTAssertEqual(e.lines, ["What I follow", "Space"])
    }

    func testFailureSaysSoPlainly() async {
        let o = await EditionCompletion.run(topics: ["Space"], writer: Throws()) { _ in }
        XCTAssertEqual(o, .failed(EditionCompletion.failedLine))
        XCTAssertTrue(EditionCompletion.failedLine.contains("topics are saved"))
    }

    func testAWriterThatNeverReturnsTimesOutAtTheDeadline() async {
        let t0 = Date()
        let o = await EditionCompletion.run(topics: [], writer: Never(), deadline: 0.3) { _ in }
        XCTAssertEqual(o, .timedOut)
        XCTAssertLessThan(Date().timeIntervalSince(t0), 5, "it did not wait for the writer")
    }

    func testTheDemoWriterFailsOnceThenSucceeds() async {
        var e = LocalEditionWriter.edition(topics: []); e.failFirst = true; e.writingLines = []
        let w = DemoEditionWriter(edition: e, tickMs: 1)
        let a = await EditionCompletion.run(topics: ["Film"], writer: w) { _ in }
        XCTAssertEqual(a, .failed(EditionCompletion.failedLine))
        let b = await EditionCompletion.run(topics: ["Film"], writer: w) { _ in }
        guard case .written(let ed) = b else { return XCTFail("\(b)") }
        XCTAssertEqual(ed.lines, ["What I follow", "Film"], "Retry writes from the same choices")
    }

    func testContinueAnywayIsTheGeneralEdition() {
        XCTAssertEqual(EditionCompletion.general().lines, ["What I follow", "The whole paper, until I say otherwise."])
    }
}
