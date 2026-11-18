import XCTest
@testable import AutoradioKit

/// The tray's glyph rows: even for the count there is (launcher.contract passes `cols` from this rule).
final class TrayGridTests: XCTestCase {
    func testNineGlyphsAreThreeRowsOfThree() {
        XCTAssertEqual(TrayGrid.columns(count: 9), 3); XCTAssertEqual(TrayGrid.rows(count: 9), 3); XCTAssertEqual(TrayGrid.emptyInLastRow(count: 9), 0)
    }
    func testOtherCountsThatCanBeEvenAreEven() {
        for n in [4, 6, 8, 12] { XCTAssertEqual(TrayGrid.emptyInLastRow(count: n), 0, "\(n) glyphs leave a gap in the last row") }
        XCTAssertEqual(TrayGrid.columns(count: 8), 4)
    }
    func testNoRowEndsWithOneGlyphAloneForTheCountsTheTrayHas() {
        for n in 6...12 where n != 7 && n != 11 { XCTAssertNotEqual(n % TrayGrid.columns(count: n), 1, "\(n)") }
    }
}
