import XCTest
@testable import AutoradioKit

/// The sign-out wipe reaches the LIVE source of the reader's follow, and a follow that was cleared is set again by the next identical feed.
final class ReaderWipeTests: XCTestCase {
    func testAFollowClearedByTheWipeIsSetAgainByTheSameFeed() {
        SharedGeneration.resetForTest()
        let follow = ReaderFollow(); let feed = ReaderFeed()
        var info = RadioInfo(); info.playing = true; info.sentences = ["The first sentence is here."]; info.sentenceIndex = 0
        HeldPages.shared.hold(gen: 0, HeldPage(address: "pub:x", title: "T", blocks: [HeldBlock(id: "b1", kind: "p", text: "The first sentence is here.", sentences: ["The first sentence is here."])]))
        feed.feed(info, address: "pub:x", follow: follow)
        XCTAssertEqual(follow.info.blockId, "b1")
        follow.clear()                                                                   // what the sign-out wipe does
        feed.feed(info, address: "pub:x", follow: follow)                                 // the same address and sentence
        XCTAssertEqual(follow.info.blockId, "b1", "set again, not left dark")
    }
}
