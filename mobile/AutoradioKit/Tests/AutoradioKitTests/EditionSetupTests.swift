import XCTest
@testable import AutoradioKit

// The topics, the tree, the saved file and the spoken edits are shared code now (AutoradioLab/edition.test.ts, edition-picks.test.ts). What is tested here is what stayed native:
// the voice desk's mailbox (the words wait for shared code, its answer is the line said back), the pushed copy of the picks, the old file's read and drop, and the edition writer.
final class EditionSetupTests: XCTestCase {
    func testTheDeskPutsTheWordsInTheMailboxAndSaysWhatSharedCodeAnswers() async throws {
        let talk = EditionTalk.shared; talk.clear(); defer { talk.clear() }
        let doc = try await EditionSetupProgramme(StageParams()).load()
        XCTAssertEqual(doc.blocks.compactMap(\.spokenText), ["Tap the topics you want, or tell me. Say, add space and chess."])
        // shared code, on its own timer: takes what was heard, answers each once
        let sharedCode = Task {
            for _ in 0..<200 {
                let items = EditionNative.op(["op": "editionHeard"])["items"] as? [[String: String]] ?? []
                if let i = items.first { XCTAssertEqual(i["words"], "add space and chess"); _ = EditionNative.op(["op": "editionSaid", "id": i["id"] ?? "", "line": "Added Space and Chess.", "finish": false]); return }
                try? await Task.sleep(nanoseconds: 10_000_000)
            }
        }
        var got: [Revision] = []
        for try await r in EditionSetupDesk(StageParams()).submit(DeskTurn(turnID: "t1", programmeID: "p", blockIndex: 0, listenerWords: "add space and chess")) { got.append(r) }
        await sharedCode.value
        XCTAssertEqual(got.first?.blocks.compactMap(\.spokenText), ["Added Space and Chess."])
    }

    func testAnswerThatNeverComesIsNoSpeechAndTheDeskStillEnds() async {
        let talk = EditionTalk.shared; talk.clear(); defer { talk.clear() }
        let a = await talk.say("add chess", deadline: 0.05)
        XCTAssertNil(a)
        XCTAssertEqual((EditionNative.op(["op": "editionHeard"])["items"] as? [[String: String]])?.count, 1, "the words stay for shared code to take or for clear() to drop")
        talk.clear()
        XCTAssertEqual((EditionNative.op(["op": "editionHeard"])["items"] as? [[String: String]])?.count, 0)
    }

    func testFinishByVoiceCallsBack() async throws {
        let talk = EditionTalk.shared; talk.clear(); defer { talk.clear(); talk.onFinish = nil }
        let done = expectation(description: "finish")
        talk.onFinish = { done.fulfill() }
        let sharedCode = Task {
            for _ in 0..<200 {
                let items = EditionNative.op(["op": "editionHeard"])["items"] as? [[String: String]] ?? []
                if let i = items.first { _ = EditionNative.op(["op": "editionSaid", "id": i["id"] ?? "", "line": "Okay.", "finish": true]); return }
                try? await Task.sleep(nanoseconds: 10_000_000)
            }
        }
        var lines: [String] = []
        for try await r in EditionSetupDesk(StageParams()).submit(DeskTurn(turnID: "t2", programmeID: "p", blockIndex: 0, listenerWords: "done")) { lines += r.blocks.compactMap(\.spokenText) }
        await sharedCode.value
        await fulfillment(of: [done], timeout: 2)
        XCTAssertEqual(lines, ["Okay."])
    }

    func testTheMirrorIsACopyInMemoryThatTheWipeEmpties() {
        _ = EditionNative.op(["op": "editionMirror", "topics": ["Space", "Chess"]])
        XCTAssertEqual(EditionPicksMirror.shared.topics, ["Space", "Chess"])
        _ = EditionNative.op(["op": "editionMirror", "topics": [String]()])
        XCTAssertEqual(EditionPicksMirror.shared.topics, [])
    }

    func testTheOldFileIsReadAndDroppedOnlyWhenAsked() throws {
        guard let f = EditionPicksLegacy.file else { return XCTFail("no Application Support") }
        let had = try? Data(contentsOf: f); defer { if let had { try? had.write(to: f) } else { try? FileManager.default.removeItem(at: f) } }
        try? FileManager.default.removeItem(at: f)
        XCTAssertEqual(EditionNative.op(["op": "editionLegacy"])["found"] as? Bool, false)
        try FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{\"topics\":[\"Space\"]}".utf8).write(to: f)
        let r = EditionNative.op(["op": "editionLegacy"])
        XCTAssertEqual(r["found"] as? Bool, true); XCTAssertEqual(r["text"] as? String, "{\"topics\":[\"Space\"]}")
        XCTAssertTrue(FileManager.default.fileExists(atPath: f.path), "reading removes nothing")
        XCTAssertEqual(EditionNative.op(["op": "editionLegacyDrop"])["gone"] as? Bool, true)
        XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
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
