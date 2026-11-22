import XCTest
@testable import AutoradioKit

final class SpineTests: XCTestCase {
    // The open set is shared code's now (AutoradioLab/pages.test.ts P1 to P15 hold the open, close, focus, persistence and "never empty" assertions); native keeps the voice's facts.
    func testOpenPageFactsRefuseAnOlderGeneration() {
        let f = OpenPageFacts()
        XCTAssertTrue(f.replace(gen: 2, [FrontPage(address: "pub:a", title: "A", kind: "reader"), FrontPage(address: "radio:", title: "Autoradio", kind: "radio")]))
        XCTAssertFalse(f.replace(gen: 1, [FrontPage(address: "pub:old", title: "Old", kind: "reader")]), "a push from an older account generation is refused")
        XCTAssertEqual(f.front(nil)?.address, "pub:a"); XCTAssertEqual(f.front("radio:")?.title, "Autoradio"); XCTAssertEqual(f.front("nope")?.address, "pub:a")
        XCTAssertEqual(PageAddress.kind(of: "radio:"), "radio"); XCTAssertEqual(PageAddress.defaultTitle("radio:"), "Autoradio")
    }

    func testPageSourceFallsBackToNotYet() async {
        let r = PageSources(); r.register(NotYetPageSource())
        let m = await r.model(for: "mailbox:inbox")
        XCTAssertEqual(m.status, "notyet"); XCTAssertEqual(m.kind, "mailbox")
    }
}
