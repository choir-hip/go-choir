import XCTest
@testable import AutoradioKit

final class SpineTests: XCTestCase {
    func testOpenPagesOpenCloseFocusAndNeverEmpty() {
        let p = OpenPages(file: nil)
        XCTAssertEqual(p.current?.address, "radio:", "the app opens on the Autoradio page")
        p.open("pub:a", title: "A")
        p.open("pub:b", title: "B")
        XCTAssertEqual(p.count, 3)
        XCTAssertEqual(p.list().map(\.address), ["pub:b", "pub:a", "radio:"])
        p.focus("pub:a")
        XCTAssertEqual(p.current?.address, "pub:a")
        p.close("pub:a")
        XCTAssertEqual(p.count, 2)
        p.close("pub:b"); p.close("radio:")
        XCTAssertEqual(p.current?.address, "radio:", "the Autoradio page cannot be closed")
        XCTAssertEqual(p.count, 1)
    }

    func testClosingTheFrontPageReturnsToItsOpener() {
        let p = OpenPages(file: nil)
        p.open("pub:a", title: "A")             // opener: radio:
        p.open("pub:b", title: "B", opener: "pub:a")
        p.close("pub:b")
        XCTAssertEqual(p.current?.address, "pub:a")
    }

    func testPersistenceRestoresAddresses() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("pages-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let p = OpenPages(file: f); p.open("settings:root", title: "Settings")
        let q = OpenPages(file: f)
        XCTAssertEqual(q.current?.address, "radio:", "every launch lands on the Autoradio page")
        XCTAssertEqual(q.count, 2)
        XCTAssertEqual(PageAddress.kind(of: "radio:"), "radio"); XCTAssertEqual(PageAddress.defaultTitle("radio:"), "Autoradio")
    }

    func testThemesLoadAndResolveBothSchemes() {
        let s = ThemeStore(userDir: nil, defaults: UserDefaults(suiteName: "spine-test-\(UUID().uuidString)")!)
        XCTAssertEqual(s.problems, [])
        XCTAssertEqual(s.list().map(\.id).sorted(), ["chrome-pastel", "depth", "lantern", "liquid-metal", "newsprint", "signal"])
        s.setMode("dark"); s.setTheme("newsprint")
        let t = s.current()
        XCTAssertEqual(t.scheme, "dark"); XCTAssertEqual(t.paper, "#0E0E0E"); XCTAssertEqual(t.material2, "solid")
        s.setMode("light"); s.setTheme("depth")
        XCTAssertEqual(s.current().material2, "glass")
        XCTAssertEqual(s.current().paper, "#FFF7FE")
    }

    func testContrastGateRejectsAThemeWithUnreadableText() {
        let bad = ##"{"schema_version":3,"id":"bad","name":"Bad","colors":{"paper":"#FFFFFF","fg":"#EEEEEE","muted":"#EEEEEE","accent":"#0000FF","onAccent":"#FFFFFF"}}"##
        let s = ThemeStore(bundled: ["bad": bad], userDir: nil, defaults: UserDefaults(suiteName: "spine-test-\(UUID().uuidString)")!)
        XCTAssertTrue(s.list().isEmpty)
        XCTAssertFalse(s.problems.isEmpty)
    }

    func testPageSourceFallsBackToNotYet() async {
        let r = PageSources(); r.register(NotYetPageSource())
        let m = await r.model(for: "mailbox:inbox")
        XCTAssertEqual(m.status, "notyet"); XCTAssertEqual(m.kind, "mailbox")
    }
}
