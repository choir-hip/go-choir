import XCTest
@testable import AutoradioKit

/// The paper's stories handed to the voice turn by shared code (paper.ts handVoiceFront): the 40-story bound, the account generation guard and the sign-out wipe.
final class VoiceFrontTests: XCTestCase {
    override func setUp() { SharedGeneration.resetForTest(); VoiceFront.shared.wipe() }
    override func tearDown() { SharedGeneration.resetForTest(); VoiceFront.shared.wipe() }

    func rows(_ n: Int) -> [[String: Any]] { (0..<n).map { ["route": "story-\($0)", "title": "Story \($0)"] } }

    func testTheTurnTakesTheFirstFortyAsRouteAndTitle() {
        XCTAssertTrue(VoiceFront.shared.set(gen: 1, rows(90)))
        XCTAssertEqual(VoiceFront.shared.candidates.count, 40)
        XCTAssertEqual(VoiceFront.shared.candidates.first?.route, "story-0"); XCTAssertEqual(VoiceFront.shared.candidates.first?.title, "Story 0")
    }
    func testARowWithNoRouteIsLeftOut() {
        XCTAssertTrue(VoiceFront.shared.set(gen: 1, [["route": "", "title": "x"], ["title": "no route"], ["route": "a", "title": "A"]]))
        XCTAssertEqual(VoiceFront.shared.candidates.map(\.route), ["a"])
    }
    func testAListOfTheOldAccountThatLandsAfterTheSignOutIsDropped() {
        XCTAssertTrue(VoiceFront.shared.set(gen: 4, rows(3)))
        SharedGeneration.announce(5); VoiceFront.shared.wipe()
        XCTAssertFalse(VoiceFront.shared.set(gen: 4, rows(3)), "computed before the sign-out, landing after it")
        XCTAssertTrue(VoiceFront.shared.candidates.isEmpty)
        XCTAssertTrue(VoiceFront.shared.set(gen: 5, rows(2)))
        XCTAssertEqual(VoiceFront.shared.candidates.count, 2)
    }
    func testSignOutEmptiesIt() async {
        VoiceFront.shared.set(gen: 1, rows(3))
        let wipe = AccountWipe(); wipe.register("voice-front") { VoiceFront.shared.wipe() }
        await wipe.run()
        XCTAssertTrue(VoiceFront.shared.candidates.isEmpty)
    }
    /// The production registration has the step. Registering is cheap and runs nothing: the stores of the process (the enrolled voice, the deck, the held pages) are not wiped by this test.
    func testTheStandardSignOutRegistersIt() async {
        let wipe = AccountWipe()
        await MainActor.run { wipe.registerStandard(legacyTakeFiles: [], oldBuildFiles: []) }
        XCTAssertTrue(wipe.names.contains("voice-front"))
    }
}
