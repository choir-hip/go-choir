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
//
// Item 2, no sideways scrolling inside a page. Ways it can fail:
//  2.1 A row inside a page is wider than the screen and the page scrolls sideways (the Mail tab row had no width, inside a page
//      column that had none either, so the column grew to the row's content).
//  2.2 A `scroll overflow-x="scroll"` row is left in a page (menu chips, breadcrumbs, a table, a code block, a CSV): it competes with
//      the deck's horizontal pan for the one gesture the owner wants to be the card swipe.
//  2.3 A page's vertical `scroll` has no axis, so it also scrolls sideways when its content is wider.
//  2.4 The page's content column has no width, so one wide child widens every page built on it (PageScroll, the paper, the Autoradio page).
//  2.5 A wrapped chip row squeezes its buttons to nothing (a chip must keep its label; rows wrap, chips do not shrink).

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
    // 1.6: the contract steps through the stack of the OPEN VIEW (Front page, My edition or a category) while an article is open: the view is state, not the page on top.
    func testContractStepsThroughTheFrontStack() throws {
        let stack = try String(contentsOf: Self.lab.appendingPathComponent("members-stack.contract"), encoding: .utf8)
        XCTAssertTrue(stack.contains("pfront.routes"))
        let paper = try String(contentsOf: Self.lab.appendingPathComponent("members-paper.contract"), encoding: .utf8)
        XCTAssertTrue(paper.contains("state paperView = \"front\""), "an open article does not change the view: the list is the one the reader came from")
        XCTAssertFalse(paper.contains("paper:section/"), "a category is a view of the paper page, not a page of the deck")
    }

    // 2.2: no contract draws a sideways scroller.
    func testNoContractScrollsSideways() throws {
        for f in try contracts() {
            XCTAssertFalse(f.text.contains("overflow-x=\"scroll\""), "\(f.name) has a sideways scroller")
            XCTAssertFalse(f.text.contains("overflow-x=\"auto\""), "\(f.name) has a sideways scroller")
        }
    }
    // 2.3: every vertical page scroll closes the sideways axis (a `scroll` line that has no overflow-x).
    func testEveryScrollClosesTheSidewaysAxis() throws {
        var seen = 0
        for f in try contracts() {
            for (n, line) in f.text.split(separator: "\n", omittingEmptySubsequences: false).enumerated() {
                let t = line.trimmingCharacters(in: .whitespaces)
                guard t.hasPrefix("scroll ") else { continue }
                seen += 1
                XCTAssertTrue(t.contains("overflow-x=\"hidden\""), "\(f.name):\(n + 1) scroll has no overflow-x=\"hidden\"")
            }
        }
        XCTAssertGreaterThan(seen, 6)
    }
    // 2.1, 2.4: the shared page column is as wide as the page and no wider.
    func testPageColumnsAreAsWideAsThePage() throws {
        let ui = try text("ui.contract")
        let page = try XCTUnwrap(ui.components(separatedBy: "component PageScroll").last)
        XCTAssertTrue(page.prefix(900).contains("column width=\"100%\" box-sizing=\"border-box\" min-width=0"))
        XCTAssertTrue(try text("paper.contract").contains("column width=\"100%\" box-sizing=\"border-box\" min-width=0 padding-top"))
        XCTAssertTrue(try text("radio.contract").contains("column width=\"100%\" box-sizing=\"border-box\" min-width=0 padding-top"))
    }
    // 2.1, 2.5: the Mail tabs are a wrapping row with a width.
    func testMailTabsWrapAndHaveAWidth() throws {
        let mail = try text("mail.contract")
        XCTAssertTrue(mail.contains("row flex-wrap=\"wrap\" gap=8 width=\"100%\"\n          for f in mbx.folders"))
        for id in ["files-sorts", "files-crumbs"] { XCTAssertTrue(try text("files.contract").contains("width=\"100%\" testId=\"\(id)\"")) }
        XCTAssertTrue(try text("paper.contract").contains("row flex-wrap=\"wrap\" gap=8 width=\"100%\" testId=\"paper-sections\""))
        XCTAssertTrue(try text("launcher.contract").contains("row flex-wrap=\"wrap\" gap=8 width=\"100%\" testId=\"launcher-key-pages\""))
    }

    private func text(_ name: String) throws -> String { try String(contentsOf: Self.lab.appendingPathComponent(name), encoding: .utf8) }
    private func contracts() throws -> [(name: String, text: String)] {
        try FileManager.default.contentsOfDirectory(atPath: Self.lab.path).filter { $0.hasSuffix(".contract") }.sorted().map { ($0, try text($0)) }
    }
}
