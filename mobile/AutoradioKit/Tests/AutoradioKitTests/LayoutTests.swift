import XCTest
@testable import AutoradioKit

// The failure list is at the top of Layout/LayoutEngine.swift; each case below names the number it covers.
final class LayoutTests: XCTestCase {
    static let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let engine = LayoutEngine(doc: .builtIn, measurer: ApproxMeasurer())

    func fixture(_ name: String) throws -> [PaperStory] {
        let d = try Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/demo/\(name).json"))
        return PaperStory.parseList(try JSONSerialization.jsonObject(with: d) as! [String: Any])
    }
    func compose(_ s: [PaperStory], width: Double = 393) -> (PaperFront) {
        PaperComposer.front(stories: s, width: width, engine: engine)
    }
    func routes(_ f: PaperFront) -> [String] { f.bands.flatMap { $0.items.map(\.route) } }

    // 1
    func testEmptyOneAndTwoStories() {
        XCTAssertEqual(compose([]).bands.count, 0)
        XCTAssertEqual(compose([]).status, "empty")
        XCTAssertEqual(compose([PaperStory(route: "a", title: "Only story")]).composition, "lead")
        let two = compose([PaperStory(route: "a", title: "First story here"), PaperStory(route: "b", title: "Second story here")])
        XCTAssertEqual(routes(two).sorted(), ["a", "b"])
    }

    // the three compositions differ, from the same rules and layout document
    func testBigDay() throws {
        let f = compose(try fixture("front-big"))
        XCTAssertTrue(f.composition.hasPrefix("lead+cluster+pair(stack)+briefs+river"), f.composition)
        XCTAssertTrue(f.composition.hasSuffix("opinion"), f.composition)
        XCTAssertEqual(f.bands.first?.items.first?.route, "demo-harbor-barrier")
        XCTAssertEqual(f.bands.first?.items.first?.size, 34, "the lead takes the largest step")
        XCTAssertTrue(f.bands.first!.items.first!.entityLabels.contains("Harbor tide barrier"))
        let cluster = f.bands.first { $0.module == "cluster" }!
        XCTAssertEqual(cluster.label, "Developing"); XCTAssertEqual(cluster.items.count, 4)
        XCTAssertTrue(cluster.items.allSatisfy { $0.indent })
    }
    func testEvenDay() throws {
        let f = compose(try fixture("front-even"))
        XCTAssertEqual(f.composition, "colead+pair+briefs+river+opinion")
        XCTAssertEqual(f.bands.first?.items.count, 2)
        XCTAssertTrue(f.bands.first!.rule)
    }
    func testThinDay() throws {
        let f = compose(try fixture("front-thin"))
        XCTAssertEqual(f.composition, "pair+river", "a quiet hour has no lead")
    }

    // 4: width decides whether a pair sits side by side
    func testPairDegradesNarrowAndSitsWide() throws {
        let s = try fixture("front-big")
        let narrow = compose(s, width: 393).bands.first { $0.module == "pair" }!
        let wide = compose(s, width: 900).bands.first { $0.module == "pair" }!
        XCTAssertFalse(narrow.side); XCTAssertTrue(wide.side)
        XCTAssertGreaterThan(wide.items[0].size, 0)
    }

    // 7, 5, 2: each story once, none invented, input order does not matter
    func testEachStoryAtMostOnceAndOrderIndependent() throws {
        for name in ["front-big", "front-even", "front-thin"] {
            let s = try fixture(name)
            let a = compose(s), b = compose(Array(s.reversed()))
            XCTAssertEqual(a, b, name)
            let r = routes(a)
            XCTAssertEqual(r.count, Set(r).count, name)
            XCTAssertTrue(Set(r).isSubset(of: Set(s.map(\.route))))
        }
        let dup = compose([PaperStory(route: "a", title: "A story"), PaperStory(route: "a", title: "A story again")])
        XCTAssertEqual(routes(dup), ["a"])
    }

    // 3: a headline too long for any step is clamped at the smallest step
    func testLongHeadlineClamps() {
        let long = String(repeating: "word ", count: 80)
        let it = compose([PaperStory(route: "a", title: long)]).bands[0].items[0]
        XCTAssertEqual(it.size, 27)     // lead steps are xl then l
        XCTAssertLessThanOrEqual(it.lines, 5)
    }

    // 10, 9
    func testOpinionNeverLeadsAndDefaultsHold() {
        let f = compose([PaperStory(route: "o", title: "A column", section: "Opinion", importance: 10, sources: 30, kind: "opinion"),
                         PaperStory(route: "n", title: "A news story")])
        XCTAssertEqual(f.bands.first?.items.first?.route, "n")
        XCTAssertTrue(f.composition.hasSuffix("opinion"))
        XCTAssertEqual(f.bands.first?.items.first?.section, "News")
    }

    // 6, and malformed documents
    func testBadLayoutDocumentsFallBackOrSkip() {
        XCTAssertEqual(LayoutDocument.load(json: "{not json"), .builtIn)
        XCTAssertEqual(LayoutDocument.load(json: #"{"schema":"other"}"#), .builtIn)
        var doc = LayoutDocument.builtIn
        doc.rules.insert(.init(group: nil, cond: "nonsense >> 4", use: "pair", else: nil, then: nil), at: 0)
        doc.rules.insert(.init(group: nil, cond: nil, use: "no-such-module", else: nil, then: nil), at: 0)
        let f = PaperComposer.front(stories: (0..<5).map { PaperStory(route: "s\($0)", title: "Story number \($0)") }, width: 393,
                                    engine: LayoutEngine(doc: doc, measurer: ApproxMeasurer()))
        XCTAssertFalse(f.bands.isEmpty)
    }

    func testEmbeddedDefaultEqualsFile() throws {
        let d = try Data(contentsOf: Self.mobile.appendingPathComponent("layouts/default.json"))
        XCTAssertEqual(try JSONDecoder().decode(LayoutDocument.self, from: d), LayoutDocument.builtIn)
    }

    // the front list's story object, with entities (additive field)
    func testParseStoryWithEntities() {
        let s = PaperStory.parse(["title": "T", "route": ["bare": "r"], "section": "World", "importance": 8, "source_count": 27, "version_count": 22,
                                  "updated_at": "2026-11-10T06:00:00Z", "entities": [["id": "ua", "label": "Ukraine"], ["id": "", "label": ""]]])!
        XCTAssertEqual(s.entities, [PaperEntity(id: "ua", label: "Ukraine")])
        XCTAssertEqual(s.sources, 27); XCTAssertNotNil(s.updated)
        XCTAssertNil(PaperStory.parse(["title": "", "route": ["bare": "r"]]))
        XCTAssertNil(PaperStory.parse(["title": "T"]))
        XCTAssertEqual(PaperStory.parse(["title": "T", "route": ["bare": "r"]])!.entities, [], "an older server sends none")
    }

    func testSectionsAndSectionPage() throws {
        let s = try fixture("front-even")
        let front = compose(s)
        XCTAssertTrue(front.sections.contains { $0.id == "tech-and-ai" })
        let page = PaperComposer.front(stories: s, section: "culture", width: 393, engine: engine)
        XCTAssertEqual(page.sectionLabel, "Culture")
        XCTAssertTrue(page.bands.flatMap(\.items).allSatisfy { $0.section == "Culture" })
    }
}
