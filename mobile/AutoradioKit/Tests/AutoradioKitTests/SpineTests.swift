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

    func testPageSourceFallsBackToNotYet() async {
        let r = PageSources(); r.register(NotYetPageSource())
        let m = await r.model(for: "mailbox:inbox")
        XCTAssertEqual(m.status, "notyet"); XCTAssertEqual(m.kind, "mailbox")
    }
}
