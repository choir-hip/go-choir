import XCTest
@testable import AutoradioKit

@MainActor
final class HeldRestTests: XCTestCase {
    
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("keep-\(UUID().uuidString).json") }

    func testTheCapDropsTheOldest() {
        let k = HeldRest(file: nil)
        for i in 0..<(HeldRest.cap + 5) { k.put(id: "e\(i)", rest: "r") }
        XCTAssertEqual(k.count, HeldRest.cap); XCTAssertNil(k.row("e0")); XCTAssertNotNil(k.row("e54"))
    }
    func testAStaleGenerationWriteIsDropped() {
        let k = HeldRest(file: nil)
        let g = AccountWipe.generation
        AccountWipe.bumpGeneration()
        k.put(id: "x", rest: "r", generation: g)
        XCTAssertNil(k.row("x"))
    }
    func testSignOutEmptiesTheStoreAndItsFile() async {
        let f = tmp(); let k = HeldRest(file: f)
        k.put(id: "x", rest: "r")
        XCTAssertTrue(FileManager.default.fileExists(atPath: f.path))
        let w = AccountWipe(); w.register("held-rest") { k.wipe() }
        await w.run()
        XCTAssertEqual(k.count, 0); XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
        XCTAssertNotNil(AccountWipe().names.count as Int?)
    }
    func testTheRestSurvivesARelaunch() {
        let f = tmp(); HeldRest(file: f).put(id: "x", rest: "The rest.")
        XCTAssertEqual(HeldRest(file: f).row("x")?.rest, "The rest.")
    }


}
