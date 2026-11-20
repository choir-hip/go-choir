import XCTest
@testable import AutoradioKit

// The design rule (owner, 2026-11-15): "no pills. no outlined bubbles." The contract-text rules (R2 to R7) moved to bun on 2026-11-20:
// mobile/AutoradioLab/contract-design-rules.test.ts. R1 tests ThemeStore, so it stays here until the themes port (lane exact-themes).
//  R1 a theme's radii come through as a capsule (999, or a card of 25 pt): the Kit clamps cards to 8...12, blocks to 4...6, whatever a theme asks.
final class DesignRulesTests: XCTestCase {
    // R1
    func testThemesAreClampedToTheRadiusRule() {
        let d = UserDefaults(suiteName: "design-\(UUID().uuidString)")!
        for id in ["chrome-pastel", "depth", "lantern", "liquid-metal", "newsprint", "signal"] {
            let s = ThemeStore(userDir: nil, defaults: d, systemScheme: { "light" }); s.setTheme(id)
            let t = s.current()
            XCTAssertTrue((8...12).contains(t.radiusCard), "\(id) card \(t.radiusCard)")
            XCTAssertTrue((4...6).contains(t.radiusControl), "\(id) control \(t.radiusControl)")
            XCTAssertLessThanOrEqual(t.radiusPill, 6, "\(id) has no pill")
        }
        XCTAssertLessThanOrEqual(Theme().radiusPill, 6); XCTAssertLessThanOrEqual(Theme().radiusCard, 12)
    }
}
