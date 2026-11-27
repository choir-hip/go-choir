import XCTest
@testable import AutoradioKit

/// The sign-out wipe reaches the live source of the reader's aim.
final class ReaderWipeTests: XCTestCase {
    func testClearingTheAimTellsTheLiveSource() {
        let aim = ReaderAim(); let told = Locked(0)
        aim.onChange = { told.write { $0 += 1 } }
        aim.set(ReaderAimInfo(address: "pub:x", blockId: "b1", label: "L")); aim.clear()
        XCTAssertEqual(told.value, 2); XCTAssertEqual(aim.info, ReaderAimInfo())
    }
}
