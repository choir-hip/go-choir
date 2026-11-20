import XCTest
@testable import AutoradioKit

// The typographic system (owner, 2026-11-16). The contract-text rules (T1 to T7, T9, the doc check) moved to bun on 2026-11-20:
// mobile/AutoradioLab/contract-typography-rules.test.ts. T8 tests Theme and ThemeStore, so it stays here until the themes port (lane exact-themes).
//  T8 the Theme record lacks a role token, or a theme's own faces leak into the roles.
final class TypographyRulesTests: XCTestCase {
    // T8
    func testTheThemeCarriesTheRoleFaces() {
        let t = Theme()
        XCTAssertEqual(t.fontHead, "ui-serif"); XCTAssertEqual(t.fontText, "ui-serif")
        XCTAssertEqual(t.fontLabel, "system-ui"); XCTAssertEqual(t.fontAction, "system-ui")
        XCTAssertEqual(t.fontMono, "ui-monospace")
        // a theme that asks for a sans body or a serif interface does not move the roles: the roles are the system.
        let d = UserDefaults(suiteName: "type-\(UUID().uuidString)")!
        for id in ["chrome-pastel", "depth", "lantern", "liquid-metal", "newsprint", "signal"] {
            let s = ThemeStore(userDir: nil, defaults: d, systemScheme: { "light" }); s.setTheme(id)
            let c = s.current()
            XCTAssertEqual(c.fontHead, "ui-serif", id); XCTAssertEqual(c.fontText, "ui-serif", id)
            XCTAssertTrue(["system-ui", "ui-rounded"].contains(c.fontLabel), "\(id) label \(c.fontLabel)")
            XCTAssertEqual(c.fontLabel, c.fontAction, id)
        }
    }
}
