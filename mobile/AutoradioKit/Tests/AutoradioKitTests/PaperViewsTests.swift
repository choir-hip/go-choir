import XCTest
@testable import AutoradioKit

// The failure list is at the top of Layout/PaperViews.swift (V1 to V10); each test names the numbers it covers.
final class PaperViewsTests: XCTestCase {
    static let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    let engine = LayoutEngine(doc: .builtIn, measurer: ApproxMeasurer())
    let house = EditionTopics.house.map { LiveSection(id: $0.id, label: $0.label) }

    func st(_ r: String, _ title: String, section: String = "world", sections: [String]? = nil, dek: String = "", ents: [String] = [], imp: Int = 5) -> PaperStory {
        PaperStory(route: r, title: title, dek: dek, section: section, importance: imp, entities: ents.map { PaperEntity(id: "e-\($0)", label: $0) }, sections: sections ?? [section])
    }
    /// A real front answer of choir.news (2026-11-15, 40 stories, dek dropped): the live shape with `sections` per story and the live list.
    func realFront() throws -> (stories: [PaperStory], live: [LiveSection]) {
        let d = try Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/live/front-live-2026-11-15.json"))
        let o = try JSONSerialization.jsonObject(with: d) as! [String: Any]
        return (PaperStory.parseList(o), PaperViews.parseLiveSections(o))
    }
    func front(_ s: [PaperStory], view: String = "front", picks: [String] = [], cat: [PaperStory]? = nil, live: [LiveSection]? = nil) -> PaperFront {
        PaperComposer.front(stories: s, view: view, picks: picks, categoryStories: cat, live: live ?? house, width: 393, engine: engine)
    }

    // ---- what a real story carries
    func testRealStoryCarriesAllItsSectionsAndTheLiveListComesWithIt() throws {
        let r = try realFront()
        XCTAssertEqual(r.stories.count, 40)
        XCTAssertEqual(r.live.map(\.id), ["world", "us", "politics", "business", "tech_ai", "science", "health", "climate", "culture", "sports"], "the live list is in house order")
        XCTAssertEqual(r.live.map(\.label).prefix(2), ["World", "U.S."])
        XCTAssertTrue(r.stories.contains { $0.sections.count > 1 }, "a story is in several sections")
        XCTAssertTrue(r.stories.allSatisfy { $0.sections.contains($0.section) }, "the primary section is among them")
        XCTAssertTrue(r.stories.contains { !$0.entities.isEmpty })
    }

    // V10 + switcher: the live order, counts from the front list, nothing changes when a view is selected.
    func testSwitcherIsTheLiveListAndDoesNotChangeWithTheView() throws {
        let r = try realFront()
        let a = front(r.stories, view: "front", live: r.live), b = front(r.stories, view: "sports", live: r.live), c = front(r.stories, view: "mine", picks: ["AI"], live: r.live)
        XCTAssertEqual(a.sections.map(\.id), r.live.map(\.id))
        XCTAssertEqual(a.sections, b.sections); XCTAssertEqual(a.sections, c.sections)
        XCTAssertEqual(a.sections.first { $0.id == "world" }?.count, r.stories.filter { $0.sections.contains("world") }.count)
    }
    func testWithoutALiveListTheSwitcherIsDerivedInHouseOrder() {
        let s = [st("a", "A", section: "Sports"), st("b", "B", section: "Politics"), st("c", "C", section: "Zoology"), st("d", "D", section: "World")]
        let f = PaperComposer.front(stories: s, live: [], width: 393, engine: engine)
        XCTAssertEqual(f.sections.map(\.label), ["World", "Politics", "Sports", "Zoology"])
        XCTAssertEqual(f.sections.map(\.id), ["world", "politics", "sports", "zoology"])
    }

    // ---- the Front page: as it was (the newest 60 and no more)
    func testFrontPageShowsTheNewestSixtyOnly() {
        let s = (0..<90).map { st("s\($0)", "Story number \($0) of the day") }
        XCTAssertEqual(front(s).count, 60)
        XCTAssertEqual(front(s).viewId, "front")
        XCTAssertEqual(front(s).viewLabel, "Front page")
    }

    // ---- categories
    // V5, V6: membership by every section a story is in; a section of one story is a page and a stack of one.
    func testCategoryIsEveryStoryInThatSectionAndOnlyThose() throws {
        let r = try realFront()
        let f = front(r.stories, view: "politics", live: r.live)
        let want = Set(r.stories.filter { $0.sections.contains("politics") }.map(\.route))
        XCTAssertFalse(want.isEmpty)
        XCTAssertEqual(Set(f.routes).isSubset(of: want), true)
        XCTAssertEqual(f.count, want.count)
        XCTAssertEqual(f.viewId, "politics"); XCTAssertEqual(f.viewLabel, "Politics"); XCTAssertEqual(f.sectionId, "politics")
    }
    func testCategoryUsesTheServersOwnListWhenGiven() {
        let s = [st("a", "A story in world")]
        let own = (0..<12).map { st("sp\($0)", "Sports story number \($0) today", section: "sports") }
        let f = front(s, view: "sports", cat: own)
        XCTAssertEqual(f.count, 12)
        XCTAssertEqual(Set(f.routes).isSubset(of: Set(own.map(\.route))), true)
    }
    func testASectionWithOneStoryIsAStackOfOne() {
        let f = front([st("a", "World one"), st("b", "The only sports story", section: "sports")], view: "sports")
        XCTAssertEqual(f.routes, ["b"]); XCTAssertNil(f.neighbor(of: "b", step: 1)); XCTAssertNil(f.neighbor(of: "b", step: -1))
    }
    func testAnEmptyCategoryIsAFriendlyEmptyPageNotAnError() {
        let f = front([st("a", "World one")], view: "climate")
        XCTAssertEqual(f.status, "empty"); XCTAssertEqual(f.message, "Nothing in Climate yet."); XCTAssertEqual(f.bands.count, 0)
    }
    // V7: a stale or unknown id is a category of that name, never the front, never a crash.
    func testUnknownViewIdIsACategoryOfThatName() {
        let f = front([st("a", "World one")], view: "no_such_section")
        XCTAssertEqual(f.viewId, "no_such_section"); XCTAssertEqual(f.count, 0); XCTAssertEqual(f.viewLabel, "No such section")
    }
    func testViewIdMatchesByLettersSoRiverHeadersFindTheLiveId() {
        let s = [st("a", "Tech one", section: "tech_ai")]
        XCTAssertEqual(front(s, view: "tech-ai").viewId, "tech_ai", "the river header passes a slug")
        XCTAssertEqual(front(s, view: "tech-ai").count, 1)
    }
    // the stack of a view is that view's list, in the page's reading order (the contract reads routes and titles off this answer).
    func testEachViewCarriesItsOwnStackInReadingOrder() throws {
        let r = try realFront()
        for v in ["front", "mine", "world", "sports"] {
            let f = front(r.stories, view: v, picks: ["Technology"], live: r.live)
            XCTAssertEqual(f.routes, f.bands.flatMap { $0.items.map(\.route) }, v)
            XCTAssertEqual(f.routes.count, f.titles.count)
            XCTAssertEqual(f.routes.count, Set(f.routes).count, "once each")
        }
        let a = front(r.stories, view: "front", live: r.live), b = front(r.stories, view: "sports", live: r.live)
        XCTAssertNotEqual(a.routes, b.routes)
    }

    // ---- My edition
    // V1
    func testNoPicksIsTheFriendlyLineAndTheFrontPage() {
        let s = (0..<8).map { st("s\($0)", "Story number \($0) of the day") }
        let f = front(s, view: "mine", picks: [])
        XCTAssertEqual(f.mineState, "nopicks"); XCTAssertEqual(f.mineFrom, ""); XCTAssertFalse(f.note.isEmpty)
        XCTAssertEqual(f.status, "ready"); XCTAssertEqual(f.routes.count, front(s).routes.count, "the front page is below the line")
        XCTAssertEqual(front(s, view: "mine", picks: ["  ", "..."]).mineState, "nopicks", "blank picks are no picks")
    }
    // V2
    func testPicksThatMatchNothingSaySoAndShowTheFrontPage() {
        let s = [st("a", "Floods in Spain"), st("b", "Elections in Peru")]
        let f = front(s, view: "mine", picks: ["Chess openings"])
        XCTAssertEqual(f.mineState, "nomatch"); XCTAssertEqual(f.mineFrom, "Chess openings"); XCTAssertTrue(f.note.contains("Nothing in today's paper"))
        XCTAssertEqual(Set(f.routes), Set(["a", "b"]))
    }
    func testACategoryPickTakesStoriesInThatSectionIncludingSecondarySections() {
        let s = [st("a", "A", section: "world", sections: ["world", "sports"]), st("b", "B", section: "world"), st("c", "C", section: "business")]
        let f = front(s, view: "mine", picks: ["Sports"])
        XCTAssertEqual(f.routes, ["a"]); XCTAssertEqual(f.mineState, "ready"); XCTAssertEqual(f.mineFrom, "Sports")
    }
    func testASubtopicMatchesTitleDekAndNamedEntitiesAsWholeWords() {
        let s = [st("a", "Chipmaker plans new fab", section: "business", dek: "Demand for artificial intelligence hardware"),
                 st("b", "Said nothing of note", section: "world"),                       // "said" must not be "ai"
                 st("c", "Regulators meet", section: "us", ents: ["AI"]),
                 st("d", "Retail sales rise", section: "business")]
        let f = front(s, view: "mine", picks: ["AI"])
        XCTAssertEqual(Set(f.routes), Set(["a", "c"]))
    }
    func testPluralsAndSynonymsAreTheSamePick() {
        let s = [st("a", "Election results due"), st("b", "Voters head to polls")]
        XCTAssertEqual(Set(front(s, view: "mine", picks: ["Elections"]).routes), Set(["a", "b"]))
        XCTAssertEqual(Set(front(s, view: "mine", picks: ["election"]).routes), Set(["a", "b"]))
        // "Tech" is the Technology category; "tech" and "Technology" together are one pick
        let t = [st("x", "Phone launch", section: "tech_ai")]
        XCTAssertEqual(front(t, view: "mine", picks: ["Tech", "Technology"]).mineFrom, "Technology")
        XCTAssertEqual(front(t, view: "mine", picks: ["tech"]).routes, ["x"])
    }
    // V3
    func testAnOddOwnPhraseNeverThrowsAndNeverMatchesEverything() {
        let s = [st("a", "Rangers win the derby"), st("b", "New chess engine beats champions at endgame openings")]
        XCTAssertEqual(front(s, view: "mine", picks: ["chess openings"]).routes, ["b"], "all its words are there")
        XCTAssertEqual(front(s, view: "mine", picks: ["x"]).mineState, "nomatch", "one letter matches nothing")
        XCTAssertEqual(front(s, view: "mine", picks: ["🙂 !!"]).mineState, "nopicks")
        XCTAssertEqual(front(s, view: "mine", picks: [String(repeating: "word ", count: 80)]).mineState, "nomatch")
        XCTAssertEqual(front(s, view: "mine", picks: ["the"]).mineState, "nomatch", "a stopword alone is not a topic that matches")
    }
    // V8: more picks first; equal counts keep the front page's own order.
    func testStoriesMatchingMorePicksComeFirst() {
        let a = st("a", "Space agency picks new rocket", section: "science", imp: 9)
        let b = st("b", "Chip firm backs AI in space", section: "tech_ai", imp: 3)
        let c = st("c", "Space telescope sees galaxy", section: "science", imp: 6)
        let d = st("d", "Mayor opens a bridge", section: "us", imp: 10)
        let f = front([a, b, c, d], view: "mine", picks: ["Technology", "AI", "Space"])
        XCTAssertEqual(f.routes.first, "b", "Technology section, AI and Space: three picks")
        XCTAssertEqual(f.routes, ["b", "a", "c"], "then the front page's own order (weight) among equals")
        XCTAssertFalse(f.routes.contains("d"))
    }
    // V5
    func testAStoryInTheFrontListTwiceIsListedOnce() {
        let s = [st("a", "AI one"), st("a", "AI one"), st("b", "AI two")]
        XCTAssertEqual(front(s, view: "mine", picks: ["AI"]).routes.sorted(), ["a", "b"])
    }
    func testFromLineNamesEveryPickOrTheFirstFive() {
        XCTAssertEqual(MyEdition.fromLine(["Technology", "AI", "Space"]), "Technology, AI, Space")
        XCTAssertEqual(MyEdition.fromLine(["a", "b", "c", "d", "e", "f", "g"]), "a, b, c, d, e and 2 more")
        let f = front([st("a", "AI one", section: "tech_ai")], view: "mine", picks: ["Technology", "AI", "Space"])
        XCTAssertEqual(f.mineFrom, "Technology, AI, Space")
    }
    // the real data: a plausible pick set gives a real, ordered, non-empty edition
    func testRealFrontWithRealPicks() throws {
        let r = try realFront()
        let f = front(r.stories, view: "mine", picks: ["Technology", "AI", "Space", "Iran"], live: r.live)
        XCTAssertEqual(f.mineState, "ready")
        XCTAssertGreaterThan(f.count, 5)
        XCTAssertLessThan(f.count, r.stories.count, "it is a selection, not the front page")
        let tech = r.stories.filter { $0.sections.contains("tech_ai") }.map(\.route)
        XCTAssertTrue(tech.allSatisfy { f.routes.contains($0) || f.count > f.routes.count }, "every Technology story is in it unless the page's bands left it out")
    }
    // V4: no list at all
    func testNoStoriesAtAllIsAnEmptyPageWithTheViewsStillThere() {
        for v in ["front", "mine", "world"] {
            let f = PaperComposer.front(stories: [], view: v, picks: ["AI"], live: house, width: 393, engine: engine, status: "error", message: "Could not reach choir.news.")
            XCTAssertEqual(f.status, "empty"); XCTAssertEqual(f.sections.count, 10); XCTAssertEqual(f.routes, [])
        }
    }

    // ---- the one list the stack and the player read
    func testCurrentListIsOneSnapshotWithNeighbours() {
        let l = PaperCurrentList()
        XCTAssertEqual(l.snapshot, .empty)
        l.set(view: "sports", routes: ["a", "b", "c"], titles: ["A", "B", "C"])
        XCTAssertEqual(l.snapshot.view, "sports")
        XCTAssertEqual(l.snapshot.neighbor(of: "a", step: 1), "b")
        XCTAssertNil(l.snapshot.neighbor(of: "a", step: -1)); XCTAssertNil(l.snapshot.neighbor(of: "c", step: 1)); XCTAssertNil(l.snapshot.neighbor(of: "zz", step: 1))
        let copy = l.snapshot
        l.set(view: "front", routes: ["x"], titles: ["X"])
        XCTAssertEqual(copy.routes, ["a", "b", "c"], "a copy taken when a story started does not change under it")
    }

    // ---- parsing
    func testParseSectionsAndLiveSections() {
        let s = PaperStory.parse(["title": "T", "route": ["bare": "r"], "section": "world", "sections": ["world", "politics", 3, ""]])!
        XCTAssertEqual(s.sections, ["world", "politics"])
        XCTAssertEqual(PaperStory.parse(["title": "T", "route": ["bare": "r"]])!.sections, [], "an older server sends none")
        XCTAssertEqual(PaperViews.parseLiveSections(["sections": [["id": "us", "label": "U.S."], ["id": "us", "label": "dup"], ["id": ""], ["label": "x"]]]),
                       [LiveSection(id: "us", label: "U.S.")])
        XCTAssertEqual(PaperViews.parseLiveSections([:]), [])
    }
    func testDisplayNames() {
        XCTAssertEqual(PaperViews.display("tech_ai"), "Technology"); XCTAssertEqual(PaperViews.display("us"), "U.S.")
        XCTAssertEqual(PaperViews.display("Tech and AI"), "Tech and AI"); XCTAssertEqual(PaperViews.display("arts_culture"), "Arts culture")
    }

    // ---- the contract side (the package tests do not compile contracts; these read them)
    func testContractWiring() throws {
        let lab = Self.mobile.appendingPathComponent("AutoradioLab")
        func t(_ n: String) throws -> String { try String(contentsOf: lab.appendingPathComponent(n), encoding: .utf8) }
        let members = try t("members-paper.contract"), paper = try t("paper.contract"), app = try t("app.contract")
        XCTAssertTrue(members.contains("state paperView = \"front\""), "the view is state that outlives the page on top")
        XCTAssertTrue(members.contains("paperFront(view=paperView"))
        for name in ["pickView", "paperView"] { XCTAssertTrue(app.contains("    \(name)\n"), "app.contract provides \(name)") }
        XCTAssertFalse(app.contains("    paperSection\n"))
        XCTAssertTrue(paper.contains("row flex-wrap=\"wrap\" gap=8 width=\"100%\" testId=\"paper-sections\""), "a wrapping row, never a sideways scroller")
        XCTAssertTrue(paper.contains("ViewChip(vid=\"mine\"")); XCTAssertTrue(paper.contains("ViewChip(vid=\"front\"")); XCTAssertTrue(paper.contains("testId=`paper-section-${vid}`"))
        XCTAssertTrue(paper.contains("testId=\"paper-edit-edition\""))
        XCTAssertTrue(paper.contains("openPage(\"onboarding:edit\", \"My edition\")"), "the edit control opens the edit page")
        XCTAssertFalse(paper.contains("onboarding:setup"))
    }
}
