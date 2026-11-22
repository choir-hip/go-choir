import XCTest
@testable import AutoradioKit

@MainActor
final class HeldRestTests: KeychainFakeCase {
    
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("keep-\(UUID().uuidString).json") }

    func testTheCapDropsTheOldest() {
        let k = HeldRest(file: nil)
        for i in 0..<(HeldRest.cap + 5) { k.put(id: "e\(i)", rest: "r") }
        XCTAssertEqual(k.count, HeldRest.cap); XCTAssertNil(k.row("e0")); XCTAssertNotNil(k.row("e54"))
    }
    func testTheRestSurvivesARelaunch() {
        let f = tmp(); HeldRest(file: f).put(id: "x", rest: "The rest.")
        XCTAssertEqual(HeldRest(file: f).row("x")?.rest, "The rest.")
    }


}
