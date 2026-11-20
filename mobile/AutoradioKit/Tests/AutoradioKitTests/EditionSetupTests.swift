import XCTest
@testable import AutoradioKit

// The failure list is at the top of EditionSetup.swift (E1..E9).
final class EditionSetupTests: XCTestCase {
    let sug = EditionTopics.suggestions()

    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("edition-\(UUID().uuidString).json") }

    func testSuggestionsAreGenerousAndStartWithTheHouseSections() {
        XCTAssertGreaterThanOrEqual(sug.count, 18)
        XCTAssertEqual(EditionTopics.tree.count, 10, "ten top-level rows when collapsed")
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

    func rows(_ exp: [String], _ sel: [String], live: [String] = []) -> [TopicRow] {
        TopicTree.rows(liveSections: live, expanded: Set(exp.map(EditionTopics.key)), selection: EditionSelection(topics: sel), own: [])
    }

    func testT2TheGraphIsSound() {
        let tops = EditionTopics.tree.map(\.label)
        XCTAssertEqual(Set(tops.map(EditionTopics.key)).count, tops.count)
        for c in EditionTopics.tree {
            XCTAssertEqual(Set(c.children.map(EditionTopics.key)).count, c.children.count, "no child twice under \(c.label)")
            for k in c.children { XCTAssertFalse(tops.contains(k), "\(k) is both a category and a subtopic: a cycle risk"); XCTAssertFalse(EditionTopics.parents(of: k).isEmpty) }
        }
        XCTAssertEqual(Set(EditionTopics.parents(of: "AI")), ["Technology", "Business"])
        XCTAssertEqual(Set(EditionTopics.parents(of: "Public health")), ["Health", "Politics"])
        XCTAssertEqual(Set(EditionTopics.parents(of: "Energy")), ["Climate", "Business"])
        XCTAssertEqual(Set(EditionTopics.parents(of: "Security")), ["Technology", "World"])
        XCTAssertEqual(EditionTopics.parents(of: "ai"), EditionTopics.parents(of: "AI"))
        XCTAssertEqual(sug.filter { EditionTopics.key($0) == "ai" }.count, 1, "one AI in the universe")
        var s = EditionSelection(); XCTAssertEqual(s.add("add ai".replacingOccurrences(of: "add ", with: ""), among: sug), "AI")
        var v = EditionSelection(); XCTAssertEqual(v.apply(spoken: "add AI", among: sug).added, ["AI"])
    }

    func testT1ASharedSubtopicIsOneTopicLitEverywhere() {
        let r = rows(["Technology", "Business"], ["AI"])
        let ai = r.filter { $0.label == "AI" }
        XCTAssertEqual(ai.count, 2)
        XCTAssertTrue(ai.allSatisfy { $0.selected })
        XCTAssertEqual(ai.map(\.note), ["also in Technology", "also in Business"])
        XCTAssertEqual(EditionSelection(topics: ["AI", "ai", "AI"]).topics, ["AI"])
        XCTAssertEqual(r.first { $0.label == "Technology" }?.note, "1 picked")
        XCTAssertEqual(r.first { $0.label == "Business" }?.note, "1 picked")
    }

    func testT3PartialSelectionAndNothingMovesOnPicking() {
        XCTAssertEqual(rows([], []).count, 10)
        let before = rows(["Culture"], []).map(\.key)
        let after = rows(["Culture"], ["Film", "Music", "Culture", "Sports"]).map(\.key)
        XCTAssertEqual(before, after, "picking changes no row")
        let r = rows([], ["Film"])
        XCTAssertEqual(r.first { $0.label == "Culture" }?.partial, true)
        XCTAssertEqual(r.first { $0.label == "Culture" }?.selected, false, "a child does not pick its parent")
        let p = rows([], ["Culture"])
        XCTAssertEqual(p.first { $0.label == "Culture" }?.selected, true)
        XCTAssertEqual(p.first { $0.label == "Culture" }?.partial, false)
        XCTAssertTrue(rows(["Culture"], ["Culture"]).filter { $0.kind == "sub" }.allSatisfy { !$0.selected }, "a parent does not pick its children")
        XCTAssertEqual(rows(["Sports"], []).count, 10, "Sports has nothing to open")
        XCTAssertEqual(rows(["Culture"], []).count, 10 + 7)
        XCTAssertFalse(rows([], []).first { $0.label == "Sports" }!.expandable)
    }

    func testSessionExpandAndLiveSections() {
        let s = EditionSetupSession(suggestions: EditionTopics.suggestions(), file: nil)
        s.toggleExpanded("Business"); XCTAssertEqual(s.treeRows().count, 10 + 7)
        s.toggleExpanded("business"); XCTAssertEqual(s.treeRows().count, 10)
        s.setLiveSections(["housing_market", "world"])
        XCTAssertEqual(s.treeRows().last?.label, "Housing market")
        XCTAssertEqual(s.treeRows().count, 11)
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
        let rows = b.treeRows()
        XCTAssertEqual(rows.first { $0.kind == "yours" }?.label, "Chess", "the reader's own topic is under Yours")
        XCTAssertEqual(rows.first { $0.label == "Science" }?.partial, true, "Space is picked under Science")
        XCTAssertEqual(rows.first { $0.label == "World" }?.selected, false)
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
        let c = s.treeRows().first { $0.label == "Chess" }
        XCTAssertNotNil(c); XCTAssertFalse(c!.selected); XCTAssertEqual(c!.kind, "yours")
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

    func testAWriterThatFailsOnceThenSucceedsRecoversOnRetry() async {
        final class FailOnce: EditionWriter, @unchecked Sendable {
            let lock = NSLock(); var failed = false
            func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
                lock.lock(); let first = !failed; failed = true; lock.unlock()
                if first { throw EditionError.failed("The desk did not answer.") }
                return LocalEditionWriter.edition(topics: topics)
            }
        }
        let w = FailOnce()
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

// The edition's address (owner finding: "Open edition" showed "Could not open this page. HTTP 404: document not found").
// Cause: obOpenEdition (members-onboarding.contract) and OnboardingController.editionReady opened "texture:my-edition", which the
// reader fetched as a document id; the writer is local, so the server never had that document.
final class EditionAddressTests: XCTestCase {
    func testTheEditionAddressResolvesToTheLocalPage() {
        XCTAssertEqual(PageAddress.canonical("texture:my-edition"), "onboarding:edit")
        XCTAssertEqual(PageAddress.canonical(" Texture:My-Edition "), "onboarding:edit")
        XCTAssertEqual(PageAddress.canonical("texture:abc123"), "texture:abc123", "a real document is left alone")
        XCTAssertEqual(PageAddress.kind(of: PageAddress.edition), "onboarding", "a page the onboarding source serves, not the reader")
        XCTAssertEqual(PageAddress.defaultTitle("onboarding:edit"), "My edition")
    }

    func testOpeningTheOldAddressOpensTheLocalPageOnce() {
        let p = OpenPages(file: nil)
        let a = p.open("texture:my-edition", title: "My edition")
        XCTAssertEqual(a.address, "onboarding:edit"); XCTAssertEqual(a.kind, "onboarding")
        _ = p.open("onboarding:edit")
        XCTAssertEqual(p.list().filter { $0.address.contains("edition") || $0.address.contains("edit") }.count, 1)
        XCTAssertFalse(p.list().contains { $0.address == "texture:my-edition" }, "no path leaves the 404 address in the deck")
    }

    func testASavedDeckWithTheOldAddressIsRepaired() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("pages-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let old = #"{"pages":[{"address":"paper:front","kind":"paper","title":"Paper","opener":"","pinned":false},{"address":"texture:my-edition","kind":"reader","title":"My edition","opener":"","pinned":false}],"recency":["texture:my-edition","paper:front"]}"#
        try Data(old.utf8).write(to: f)
        let p = OpenPages(file: f)
        XCTAssertEqual(p.list().map(\.address).sorted(), ["onboarding:edit", "paper:front", "radio:"], "the Autoradio page is always open")
        XCTAssertEqual(p.list().first?.address, "radio:", "every launch lands on the Autoradio page")
        XCTAssertEqual(p.list().first { $0.address == "onboarding:edit" }?.kind, "onboarding")
    }
}
