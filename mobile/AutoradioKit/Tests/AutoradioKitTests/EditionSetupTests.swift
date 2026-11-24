import XCTest
@testable import AutoradioKit

// The topics, the tree, the saved file and the spoken edits are shared code now (AutoradioLab/edition.test.ts, edition-picks.test.ts). What is tested here is what stayed native:
// the voice desk's mailbox (the words wait for shared code, its answer is the line said back), the pushed copy of the picks, the old file's read and drop, and the edition writer.
final class EditionSetupTests: XCTestCase {
    /// Shared code, on its own timer: takes what was heard and answers each once.
    func sharedCode(line: String, finish: Bool) -> Task<Void, Never> {
        Task {
            for _ in 0..<200 {
                if let i = (EditionNative.op(["op": "editionHeard"])["items"] as? [[String: String]] ?? []).first {
                    _ = EditionNative.op(["op": "editionSaid", "id": i["id"] ?? "", "line": line, "finish": finish]); return
                }
                try? await Task.sleep(nanoseconds: 10_000_000)
            }
        }
    }
    func say(_ words: String) async throws -> [String] {
        var lines: [String] = []
        for try await r in EditionSetupDesk(StageParams()).submit(DeskTurn(turnID: "t", programmeID: "p", blockIndex: 0, listenerWords: words)) { lines += r.blocks.compactMap(\.spokenText) }
        return lines
    }

    func testTheDeskPutsTheWordsInTheMailboxAndSaysWhatSharedCodeAnswers() async throws {
        let talk = EditionTalk.shared; talk.clear(); defer { talk.clear() }
        let doc = try await EditionSetupProgramme(StageParams()).load()
        XCTAssertEqual(doc.blocks.compactMap(\.spokenText), ["Tap the topics you want, or tell me. Say, add space and chess."])
        let t = sharedCode(line: "Added Space and Chess.", finish: false)
        let lines = try await say("add space and chess"); await t.value
        XCTAssertEqual(lines, ["Added Space and Chess."])
    }

    func testFinishByVoiceCallsBackAndAnUnansweredTurnIsSilentButEnds() async throws {
        let talk = EditionTalk.shared; talk.clear(); defer { talk.clear(); talk.onFinish = nil }
        let done = expectation(description: "finish"); talk.onFinish = { done.fulfill() }
        let t = sharedCode(line: "Okay.", finish: true)
        let lines = try await say("done"); await t.value
        await fulfillment(of: [done], timeout: 2)
        XCTAssertEqual(lines, ["Okay."])
        let none = await talk.say("add chess", deadline: 0.05)
        XCTAssertNil(none); talk.clear()
        XCTAssertEqual((EditionNative.op(["op": "editionHeard"])["items"] as? [[String: String]])?.count, 0, "clear() drops what was never taken")
    }

    func testTheMirrorIsACopyInMemoryThatTheWipeEmptiesAndForgets() {
        SharedGeneration.resetForTest(); EditionPicksMirror.shared.reset(); defer { EditionPicksMirror.shared.reset(); SharedGeneration.resetForTest() }
        XCTAssertFalse(EditionPicksMirror.shared.known, "nothing pushed yet: an empty copy is not the listener's choice")
        _ = EditionNative.op(["op": "editionMirror", "topics": ["Space", "Chess"], "gen": 1])
        XCTAssertEqual(EditionPicksMirror.shared.topics, ["Space", "Chess"]); XCTAssertTrue(EditionPicksMirror.shared.known)
        _ = EditionNative.op(["op": "editionMirror", "topics": [String](), "gen": 1])
        XCTAssertEqual(EditionPicksMirror.shared.topics, []); XCTAssertTrue(EditionPicksMirror.shared.known, "pushed empty IS the listener's choice")
        _ = EditionNative.op(["op": "editionMirrorReset"])
        XCTAssertFalse(EditionPicksMirror.shared.known); XCTAssertEqual(EditionPicksMirror.shared.topics, [])
    }

    func testTheOldFileIsNeitherReadNorDeletedByTheEditionCalls() throws {
        guard let f = TakeFiles.url("edition-topics.json") else { return XCTFail("no Application Support") }
        let had = try? Data(contentsOf: f); defer { if let had { try? had.write(to: f) } else { try? FileManager.default.removeItem(at: f) } }
        try FileManager.default.createDirectory(at: f.deletingLastPathComponent(), withIntermediateDirectories: true)
        try Data("{\"topics\":[\"Space\"]}".utf8).write(to: f)
        XCTAssertEqual(EditionNative.op(["op": "editionLegacy"]).count, 0, "no read op")
        XCTAssertEqual(EditionNative.op(["op": "editionLegacyDrop"]).count, 0, "no drop op")
        XCTAssertEqual(try String(contentsOf: f, encoding: .utf8), "{\"topics\":[\"Space\"]}", "left where it is, unread")
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

    func testAWriterThatNeverReturnsTimesOutAtTheDeadlineAndIsCancelled() async {
        final class Watching: EditionWriter, @unchecked Sendable {
            let lock = NSLock(); private var cancelledFlag = false
            var cancelled: Bool { lock.lock(); defer { lock.unlock() }; return cancelledFlag }
            func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
                do { try await Task.sleep(nanoseconds: 60_000_000_000) } catch { lock.lock(); cancelledFlag = true; lock.unlock(); throw error }
                return LocalEditionWriter.edition(topics: [])
            }
        }
        let w = Watching(), t0 = Date()
        let o = await EditionCompletion.run(topics: [], writer: w, deadline: 0.3) { _ in }
        XCTAssertEqual(o, .timedOut)
        XCTAssertLessThan(Date().timeIntervalSince(t0), 5, "it did not wait for the writer")
        XCTAssertTrue(w.cancelled, "the writer's task was cancelled at the deadline")
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
}
