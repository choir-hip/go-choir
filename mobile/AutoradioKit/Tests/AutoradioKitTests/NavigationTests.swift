import XCTest
@testable import AutoradioKit

// Navigation fixes (owner phone review 2026-11-15, items 1, 2, 3, 11). Failure lists, written before the code.
//
// Item 1, paper articles as a card stack. Ways it can fail:
//  1.1 The stack order is not the page's reading order (bands then items), so a swipe lands on a story that is not next on the page.
//  1.2 A story shown in two bands appears twice, so a swipe "moves" to the same story.
//  1.3 A swipe past the last or before the first story wraps, or leaves for the front page, instead of springing back.
//  1.4 An open page that is not in the stack (a Texture, a story the front page does not list) gets a neighbour from a wrong index
//      (indexOf -1 plus one is 0, the first story) instead of falling back to the deck's own cycle.
//  1.5 The titles drift from the routes (one list shorter), so the card names the wrong story.
//  1.6 The section page's stack is used for a story opened from the front (the contract reads the front list while an article is open).
//  1.7 An empty route (a story with no address) is a card nobody can open.

final class NavigationTests: XCTestCase {
    static let lab = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("AutoradioLab")

    private func item(_ route: String, _ title: String = "") -> PaperItem {
        PaperItem(route: route, title: title.isEmpty ? route.uppercased() : title, dek: "", section: "News", meta: "", byline: "", entityLabels: "", entityIds: "",
                  size: 17, weight: 600, lineHeight: 20, lines: 2, showDek: false, dekSize: 14, dekLines: 0, indent: false, rank: 0)
    }
    private func band(_ items: [PaperItem]) -> PaperBand { PaperBand(id: "b", module: "river", label: "", side: false, rule: false, items: items) }

    // 1.1, 1.2, 1.5, 1.7
    func testStackIsReadingOrderOnceEachWithMatchingTitles() {
        let s = PaperFront.stack(of: [band([item("a"), item("b")]), band([item("c"), item("a"), item("")])])
        XCTAssertEqual(s.routes, ["a", "b", "c"])
        XCTAssertEqual(s.titles, ["A", "B", "C"])
    }
    // 1.1 through the real composer
    func testComposedFrontCarriesTheStackInPageOrder() {
        let engine = LayoutEngine(doc: .builtIn, measurer: ApproxMeasurer())
        let stories = (1...7).map { PaperStory(route: "s\($0)", title: "Story number \($0) of the day") }
        let f = PaperComposer.front(stories: stories, width: 393, engine: engine)
        XCTAssertEqual(f.routes, f.bands.flatMap { $0.items.map(\.route) })
        XCTAssertFalse(f.routes.isEmpty)
        XCTAssertTrue(Set(f.routes).isSubset(of: Set(stories.map(\.route))), "the page may leave a story out; the stack is what the page shows")
        XCTAssertEqual(f.routes.count, f.titles.count)
    }
    // 1.3, 1.4
    func testNeighborStopsAtTheEndsAndIgnoresStrangers() {
        var f = PaperFront.empty()
        f.routes = ["a", "b", "c"]; f.titles = ["A", "B", "C"]
        XCTAssertEqual(f.neighbor(of: "a", step: 1), 1)
        XCTAssertEqual(f.neighbor(of: "b", step: -1), 0)
        XCTAssertNil(f.neighbor(of: "a", step: -1), "no wrap before the first")
        XCTAssertNil(f.neighbor(of: "c", step: 1), "no wrap after the last")
        XCTAssertNil(f.neighbor(of: "texture-x", step: 1), "a page outside the stack has no neighbour")
        XCTAssertNil(PaperFront.empty().neighbor(of: "a", step: 1))
    }
    // 1.6: the contract steps through the stack of the FRONT page while an article is open.
    func testContractStepsThroughTheFrontStack() throws {
        let stack = try String(contentsOf: Self.lab.appendingPathComponent("members-stack.contract"), encoding: .utf8)
        XCTAssertTrue(stack.contains("pfront.routes"))
        let paper = try String(contentsOf: Self.lab.appendingPathComponent("members-paper.contract"), encoding: .utf8)
        XCTAssertTrue(paper.contains("pg.top.startsWith(\"paper:section/\")"), "an open article is not a section page: the front list is the one loaded")
    }
}
