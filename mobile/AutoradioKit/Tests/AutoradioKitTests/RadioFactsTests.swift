import XCTest
@testable import AutoradioKit

/// What shared code pushes the voice and the take recorder (AutoradioLab/voice-facts.ts): native keeps it, hands it back, refuses an older account's push, and forgets it at sign-out.
final class RadioFactsTests: XCTestCase {
    override func setUp() { SharedGeneration.resetForTest(); RadioBridge.shared.wipe() }
    override func tearDown() { SharedGeneration.resetForTest(); RadioBridge.shared.wipe() }

    func testFactsAndRespondsAreKeptAndHandedBack() throws {
        let facts = try JSONSerialization.data(withJSONObject: [["id": "a", "title": "T", "text": "Words.", "state": "on_air", "ref": "pub:x"]])
        XCTAssertTrue(RadioBridge.shared.set(gen: 2, snapshot: nil, facts: facts, responds: TakeResponds(route: "harbor-fees", title: "Harbor fees rise"), onAir: true))
        XCTAssertEqual(RadioBridge.shared.facts().first?["id"] as? String, "a")
        XCTAssertEqual(RadioBridge.shared.responds(), TakeResponds(route: "harbor-fees", title: "Harbor fees rise"))
        XCTAssertTrue(RadioBridge.shared.hasOnAir())
    }

    func testAnOlderAccountsPushIsRefusedAndSignOutForgets() throws {
        XCTAssertTrue(RadioBridge.shared.set(gen: 5, snapshot: nil, facts: nil, responds: TakeResponds(route: "new", title: "N")))
        XCTAssertFalse(RadioBridge.shared.set(gen: 3, snapshot: nil, facts: nil, responds: TakeResponds(route: "old", title: "O")))
        XCTAssertEqual(RadioBridge.shared.responds()?.route, "new")
        RadioBridge.shared.wipe()
        XCTAssertNil(RadioBridge.shared.responds()); XCTAssertTrue(RadioBridge.shared.facts().isEmpty); XCTAssertFalse(RadioBridge.shared.hasOnAir())
    }
}
