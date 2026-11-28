import XCTest
@testable import AutoradioKit

// Written from the failure list at the top of BarState.swift.
final class BarStateTests: XCTestCase {
    private let t0 = Date(timeIntervalSince1970: 1000)

    // (The bar's states, words and windows are bar.ts: bar.test.ts. What is native is what the engine told it, kept raw.)
    func testEventsAreKeptRawWithTheirTime() {
        let tr = BarTracker()
        tr.observe(.holdingSaid("Looking at the third paragraph…"), now: t0)
        XCTAssertEqual(tr.raw().holding, "Looking at the third paragraph…")
        tr.observe(.phase(from: "playing", to: "candidate"), now: t0)
        XCTAssertEqual(tr.raw().holding, "", "a new turn clears the old line")
        tr.observe(.revisionArrived(id: "r", fromBlock: 1, blocks: 2, summary: "s"), now: t0)
        XCTAssertEqual(tr.raw().revisedAt, 1000)
        tr.observe(.error(stage: "desk", message: "boom"), now: t0.addingTimeInterval(10))
        XCTAssertEqual(tr.raw().errorStage, "desk"); XCTAssertEqual(tr.raw().errorMessage, "boom"); XCTAssertEqual(tr.raw().errorAt, 1010)
        tr.observe(.error(stage: "desk", message: "boom"), now: t0.addingTimeInterval(13))
        XCTAssertEqual(tr.raw().errorAt, 1013, "a repeat moves the time, so the window restarts")
        tr.observe(.error(stage: "other", message: "x"), now: t0.addingTimeInterval(20))
        XCTAssertEqual(tr.raw().errorAt, 1013, "only the engine's own stages are errors for the bar")
        XCTAssertEqual(tr.expiry(now: t0.addingTimeInterval(5))!, 1.0, accuracy: 0.01)
        XCTAssertNil(tr.expiry(now: t0.addingTimeInterval(30)))
        tr.wipe()
        XCTAssertEqual(tr.raw().errorAt, 0); XCTAssertEqual(tr.raw().holding, ""); XCTAssertEqual(tr.raw().revisedAt, 0)
    }

    func testRadioInfoCarriesOnlyWhatNativeReports() {
        let tr = BarTracker()
        tr.setSpeed(1.5); _ = tr.feedMic(db: -20)
        let r = tr.radioInfo(level: "dip")
        XCTAssertEqual(r.level, "dip"); XCTAssertEqual(r.speedValue, 1.5); XCTAssertEqual(r.micLevel, 1, accuracy: 0.001)
        let keys = Set((try! JSONSerialization.jsonObject(with: JSONEncoder().encode(r)) as! [String: Any]).keys)
        XCTAssertEqual(keys, ["level", "speedValue", "micLevel"], "the title and whether it plays are shared code's: they are not across the boundary")
    }
}
