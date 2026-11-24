import XCTest
@testable import AutoradioKit

// The topics, the tree, the saved file and the spoken edits are shared code now (AutoradioLab/edition.test.ts, edition-picks.test.ts). What is tested here is what stayed native:
// the voice desk's mailbox (the words wait for shared code, its answer is the line said back), the pushed copy of the picks, the old file's read and drop.
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

}
