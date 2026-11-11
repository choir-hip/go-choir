import XCTest
@testable import AutoradioKit

// Failure list: see BarTracker.feedMic (1 silence reads 0, 2 one loud frame does not stick, 3 redraws only on real movement).
final class MicLevelTests: XCTestCase {
    func testSilenceReadsZeroAndLoudReadsNearOne() {
        XCTAssertEqual(BarTracker.levelFor(db: -120), 0)
        XCTAssertEqual(BarTracker.levelFor(db: -55), 0)
        XCTAssertEqual(BarTracker.levelFor(db: -20), 1, accuracy: 0.001)
        XCTAssertEqual(BarTracker.levelFor(db: 0), 1)
    }

    func testALoudFrameFallsAwayAndSilenceStaysQuiet() {
        let t = BarTracker()
        XCTAssertFalse(t.feedMic(db: -120), "silence on a silent tracker is no change")
        XCTAssertTrue(t.feedMic(db: -20))
        XCTAssertEqual(t.micLevel, 1, accuracy: 0.001)
        for _ in 0..<40 { _ = t.feedMic(db: -120) }
        XCTAssertEqual(t.micLevel, 0, "it does not stick")
    }

    func testTinyJitterIsNotARedraw() {
        let t = BarTracker()
        _ = t.feedMic(db: -37.5)
        XCTAssertFalse(t.feedMic(db: -37.4))
    }

    func testRadioInfoCarriesTheLevel() {
        let t = BarTracker()
        _ = t.feedMic(db: -20)
        var s = LabSnapshot(); s.blockCount = 0
        let r = t.radioInfo(player: ChoirPlayerView(), snapshot: s, document: ProgrammeDocument(id: "d", title: "t", blocks: []), route: "")
        XCTAssertEqual(r.micLevel, 1, accuracy: 0.001)
    }
}
