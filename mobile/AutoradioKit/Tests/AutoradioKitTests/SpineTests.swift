import XCTest
@testable import AutoradioKit

final class SpineTests: XCTestCase {
    // The open set is shared code's now (AutoradioLab/pages.test.ts P1 to P15 hold the open, close, focus, persistence and "never empty" assertions); native keeps the voice's facts.
    func testOpenPageFactsRefuseAnOlderGenerationAndWipe() {
        let f = OpenPageFacts()
        XCTAssertTrue(f.replace(gen: 2, [FrontPage(address: "pub:a", title: "A", kind: "reader"), FrontPage(address: "radio:", title: "Autoradio", kind: "radio")]))
        XCTAssertFalse(f.replace(gen: 1, [FrontPage(address: "pub:old", title: "Old", kind: "reader")]), "a push from an older account generation is refused")
        XCTAssertEqual(f.front(nil)?.address, "pub:a"); XCTAssertEqual(f.front("radio:")?.title, "Autoradio"); XCTAssertEqual(f.front("nope")?.address, "pub:a")
        f.wipe()
        XCTAssertNil(f.front(nil)); XCTAssertFalse(f.replace(gen: 2, [FrontPage(address: "pub:b", title: "B", kind: "reader")]), "after the wipe a push of a generation already seen is refused")
        XCTAssertTrue(f.replace(gen: 3, [FrontPage(address: "pub:b", title: "B", kind: "reader")]))
    }
}
