import XCTest
@testable import AutoradioKit

/// D-2 (cross-account write): what shared code pushes to a native mirror carries the account generation it was computed under, and a push from before a sign-out that lands after it changes nothing.
final class SharedGenerationTests: XCTestCase {
    override func setUp() { SharedGeneration.resetForTest(); RadioBridge.shared.wipe(); EditionPicksMirror.shared.reset(); HeldPages.shared.wipe() }
    override func tearDown() { SharedGeneration.resetForTest(); RadioBridge.shared.wipe(); EditionPicksMirror.shared.reset(); HeldPages.shared.wipe() }

    func testAPushOfTheOldAccountThatLandsAfterTheWipeIsDropped() {
        XCTAssertTrue(RadioBridge.shared.set(gen: 4, snapshot: Data("A".utf8), onAir: nil, sheet: nil))
        SharedGeneration.announce(5)                                  // the sign-out began: shared code's generation moved to 5, the signOut call said so
        RadioBridge.shared.wipe()
        XCTAssertFalse(RadioBridge.shared.set(gen: 4, snapshot: Data("old account".utf8), onAir: nil, sheet: nil), "the pulse begun under generation 4 lands after the wipe")
        XCTAssertNil(RadioBridge.shared.snapshot, "nothing of the old account is stored")
        XCTAssertTrue(RadioBridge.shared.set(gen: 5, snapshot: Data("B".utf8), onAir: nil, sheet: nil))
        XCTAssertEqual(RadioBridge.shared.snapshot, Data("B".utf8))
    }
    func testTheFirstPushOfTheLaunchIsDroppedToo() {
        SharedGeneration.announce(1)                                  // nothing was pushed before the sign-out: the announcement alone closes the gap
        XCTAssertFalse(RadioBridge.shared.set(gen: 0, snapshot: Data("old".utf8), onAir: nil, sheet: nil))
        XCTAssertNil(RadioBridge.shared.snapshot)
    }
    func testALateEditionCopyIsNotKeptForTheNextAccount() {
        XCTAssertTrue(EditionPicksMirror.shared.set(gen: 2, ["space", "chess"]))
        SharedGeneration.announce(3); EditionPicksMirror.shared.reset()
        XCTAssertFalse(EditionPicksMirror.shared.set(gen: 2, ["space", "chess"]), "account A's topics computed before the sign-out")
        XCTAssertFalse(EditionPicksMirror.shared.known); XCTAssertTrue(EditionPicksMirror.shared.topics.isEmpty)
        XCTAssertTrue(EditionPicksMirror.shared.set(gen: 3, ["jazz"]))
        XCTAssertEqual(EditionPicksMirror.shared.topics, ["jazz"])
    }
    func testALatePageTextIsNotHeldForTheNextAccount() {
        XCTAssertTrue(HeldPages.shared.hold(gen: 2, HeldPage(address: "pub:a", title: "A")))
        SharedGeneration.announce(3); HeldPages.shared.wipe()
        XCTAssertFalse(HeldPages.shared.hold(gen: 2, HeldPage(address: "pub:old", title: "old account")), "the page read under generation 2 lands after the sign-out")
        XCTAssertNil(HeldPages.shared.page("pub:old")); XCTAssertEqual(HeldPages.shared.count, 0)
        XCTAssertTrue(HeldPages.shared.hold(gen: 3, HeldPage(address: "pub:new")))
        XCTAssertNotNil(HeldPages.shared.page("pub:new"))
    }
    func testThePageTextOpWithNoGenerationIsDroppedOnceOneIsKnown() {
        SharedGeneration.announce(1)
        let r: [String: Any] = ["address": "pub:a"]
        XCTAssertEqual(HeldPage.generation(of: r), 0); XCTAssertEqual(HeldPage.generation(of: ["gen": 7]), 7)
        XCTAssertFalse(HeldPages.shared.hold(gen: HeldPage.generation(of: r), HeldPage.parse(r)!), "an unstamped hand-over is refused, not trusted")
    }
}
