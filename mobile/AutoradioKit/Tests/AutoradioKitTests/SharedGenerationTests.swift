import XCTest
@testable import AutoradioKit

/// D-2 (cross-account write): what shared code pushes to a native mirror carries the account generation it was computed under, and a push from before a sign-out that lands after it changes nothing.
final class SharedGenerationTests: XCTestCase {
    override func setUp() { SharedGeneration.resetForTest(); RadioBridge.shared.wipe(); HeldPages.shared.wipe() }
    override func tearDown() { SharedGeneration.resetForTest(); RadioBridge.shared.wipe(); HeldPages.shared.wipe() }

    func testAPushOfTheOldAccountThatLandsAfterTheWipeIsDropped() {
        XCTAssertTrue(RadioBridge.shared.set(gen: 4, snapshot: Data("A".utf8), facts: nil, responds: nil))
        SharedGeneration.announce(5)                                  // the sign-out began: shared code's generation moved to 5, the signOut call said so
        RadioBridge.shared.wipe()
        XCTAssertFalse(RadioBridge.shared.set(gen: 4, snapshot: Data("old account".utf8), facts: nil, responds: nil), "the pulse begun under generation 4 lands after the wipe")
        XCTAssertNil(RadioBridge.shared.snapshot, "nothing of the old account is stored")
        XCTAssertTrue(RadioBridge.shared.set(gen: 5, snapshot: Data("B".utf8), facts: nil, responds: nil))
        XCTAssertEqual(RadioBridge.shared.snapshot, Data("B".utf8))
    }
    func testTheFirstPushOfTheLaunchIsDroppedToo() {
        SharedGeneration.announce(1)                                  // nothing was pushed before the sign-out: the announcement alone closes the gap
        XCTAssertFalse(RadioBridge.shared.set(gen: 0, snapshot: Data("old".utf8), facts: nil, responds: nil))
        XCTAssertNil(RadioBridge.shared.snapshot)
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

    // D2: a JS context that boots below the newest generation native saw (a reload with the process alive) is adopted at its boot announcement, not refused for the life of the process.
    func testABootBelowTheNewestIsAdopted() {
        SharedGeneration.announce(9)
        XCTAssertFalse(RadioBridge.shared.set(gen: 0, snapshot: Data("x".utf8), facts: nil, responds: nil), "an ordinary push below the newest is refused")
        SharedGeneration.boot(0)
        XCTAssertTrue(RadioBridge.shared.set(gen: 0, snapshot: Data("y".utf8), facts: nil, responds: nil), "after the boot announcement the new context's pushes are taken")
        XCTAssertEqual(RadioBridge.shared.snapshot, Data("y".utf8))
        SharedGeneration.announce(1); XCTAssertFalse(SharedGeneration.accepts(0), "and a sign-out floor still refuses what is older")
    }
    // D1: the wipe's last act announces a generation above everything computed before it; what is computed after (the public paper, signed out) is taken.
    func testNothingComputedBeforeTheWipeLandsAfterItAndAPublicPushIsTaken() {
        SharedGeneration.announce(3)                                  // the sign-out began (the signOut call)
        XCTAssertTrue(SharedGeneration.accepts(3), "equality is taken: a pulse that began after the sign-out read this generation")
        SharedGeneration.announce(5)                                  // the wipe's last act
        XCTAssertFalse(RadioBridge.shared.set(gen: 3, snapshot: Data("old account's queue".utf8), facts: nil, responds: nil))
        XCTAssertNil(RadioBridge.shared.snapshot)
        XCTAssertTrue(RadioBridge.shared.set(gen: 5, snapshot: Data("public paper".utf8), facts: nil, responds: nil), "signed out, the paper still plays")
    }
    // D4: the check and the write are one step, so a push that passed the check cannot land after an announcement and the wipe that follows it.
    func testTheCheckAndTheWriteAreOneStep() async {
        for round in 0..<300 {
            SharedGeneration.resetForTest(); RadioBridge.shared.wipe(); HeldPages.shared.wipe()
            await withTaskGroup(of: Void.self) { g in
                for _ in 0..<3 { g.addTask { for _ in 0..<40 { RadioBridge.shared.set(gen: 1, snapshot: Data("old".utf8), facts: nil, responds: nil); HeldPages.shared.hold(gen: 1, HeldPage(address: "a")) } } }
                g.addTask { SharedGeneration.announce(2); RadioBridge.shared.wipe(); HeldPages.shared.wipe() }
            }
            // after the announcement and the wipe, nothing of generation 1 may be there, in whatever order the tasks ran (a push before the announcement is wiped by it; one after is refused)
            XCTAssertNil(RadioBridge.shared.snapshot, "round \(round)"); XCTAssertEqual(HeldPages.shared.count, 0, "round \(round)")
        }
    }
}
