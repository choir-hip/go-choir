import XCTest
@testable import AutoradioKit

// The page the shared reader hands to the voice (HeldPages.swift). Failure list: H1 the hand-over parses into blocks with their sentences, and one with no address is nothing; H2 the lines a turn reads are
// the blocks with text and no code; H3 the page source answers the held page and "loading" before the hand-over; H4 at most `limit`, least recently used first, a page just held stays; H5 sign-out forgets
// every page; H6 the follow-along finds the block and the sentence by the spoken words, never a wrong block; H7 the corpus body has no code or embeds.
final class HeldPagesTests: XCTestCase {
    let request: [String: Any] = ["op": "pageText", "address": "pub:a/x", "title": "Harbor", "dek": "A dek.", "kind": "story", "blocks": [
        ["id": "b1", "kind": "p", "text": "The first sentence is here. The second sentence follows it.", "sentences": ["The first sentence is here.", "The second sentence follows it."]],
        ["id": "b2", "kind": "code", "text": "let a = 1", "sentences": ["let a = 1"]],
        ["id": "b3", "kind": "p", "text": "A later paragraph about batteries.", "sentences": ["A later paragraph about batteries."]],
        ["id": "b4", "kind": "embed", "text": "A song", "sentences": []]] as [[String: Any]]]

    func testH1_TheHandOverParses() {
        let p = HeldPage.parse(request)
        XCTAssertEqual(p?.title, "Harbor"); XCTAssertEqual(p?.blocks.count, 4); XCTAssertEqual(p?.blocks[0].sentences.count, 2); XCTAssertEqual(p?.kind, "story")
        XCTAssertNil(HeldPage.parse(["address": ""])); XCTAssertNil(HeldPage.parse([:]))
        XCTAssertEqual(HeldPage.parse(["address": "pub:z"])?.blocks.count, 0)
    }
    func testH2_LinesAreTheTextNoCode() {
        XCTAssertEqual(HeldPage.parse(request)?.lines, ["The first sentence is here. The second sentence follows it.", "A later paragraph about batteries.", "A song"])
    }
    func testH3_ThePageSourceAnswersTheHeldPageAndLoadingBefore() async throws {
        let h = HeldPages(), s = HeldPageSource(held: h)
        XCTAssertTrue(s.handles("pub:a/x")); XCTAssertTrue(s.handles("texture:d1")); XCTAssertFalse(s.handles("paper:front"))
        let before = try await s.load("pub:a/x")
        XCTAssertEqual(before.status, "loading"); XCTAssertTrue(before.lines.isEmpty)
        h.hold(HeldPage.parse(request)!)
        let after = try await s.load("pub:a/x")
        XCTAssertEqual(after.status, "ready"); XCTAssertEqual(after.title, "Harbor"); XCTAssertEqual(after.kind, "reader"); XCTAssertEqual(after.lines.count, 3)
    }
    func testH4_BoundedLeastRecentlyUsedFirst() {
        let h = HeldPages(limit: 3)
        for a in ["pub:1", "pub:2", "pub:3"] { h.hold(HeldPage(address: a)) }
        _ = h.page("pub:1")
        h.hold(HeldPage(address: "pub:4"))
        XCTAssertEqual(h.count, 3); XCTAssertNil(h.page("pub:2")); XCTAssertNotNil(h.page("pub:1")); XCTAssertNotNil(h.page("pub:4"))
        for i in 5...30 { h.hold(HeldPage(address: "pub:\(i)")) }
        XCTAssertEqual(h.count, 3)
    }
    func testH5_SignOutForgetsEverything() {
        let h = HeldPages(); h.hold(HeldPage(address: "pub:1")); h.wipe()
        XCTAssertEqual(h.count, 0); XCTAssertNil(h.page("pub:1")); XCTAssertTrue(h.all().isEmpty)
    }
    func testH6_FollowLocatesByText() {
        let d = HeldPage.parse(request)
        let hit = ReaderFollow.locate(spoken: "The second sentence follows it.", in: d)
        XCTAssertEqual(hit?.block, "b1"); XCTAssertEqual(hit?.sentence, 1)
        XCTAssertEqual(ReaderFollow.locate(spoken: "later paragraph about", in: d)?.block, "b3")
        XCTAssertNil(ReaderFollow.locate(spoken: "nothing like this anywhere", in: d))
        XCTAssertNil(ReaderFollow.locate(spoken: "", in: d)); XCTAssertNil(ReaderFollow.locate(spoken: "The", in: nil))
        let h = HeldPages(); h.hold(d!)
        XCTAssertEqual(ReaderFollow().set(address: "pub:a/x", spoken: "later paragraph about", held: h).blockId, "b3")
        XCTAssertEqual(ReaderFollow().set(address: "pub:other", spoken: "later paragraph about", held: h).blockId, "", "a page that was not handed over highlights nothing")
    }
    func testH7_TheCorpusBodyHasNoCodeOrEmbeds() {
        XCTAssertEqual(HeldPage.parse(request)?.body, "The first sentence is here. The second sentence follows it.\nA later paragraph about batteries.")
        let c = SavedCorpus(heldPages: { let h = HeldPages(); h.hold(HeldPage.parse(request)!); return h }())
        c.refresh()
        XCTAssertFalse(c.retrieve("batteries").isEmpty)
    }
}
