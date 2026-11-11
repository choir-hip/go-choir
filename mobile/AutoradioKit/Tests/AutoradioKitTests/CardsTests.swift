import XCTest
@testable import AutoradioKit

// Lane 4: preferences (what the swipes taught the paper), the story stack, the launcher's search and recents, signals.
// Each test answers one entry of the failure lists at the top of Preferences.swift, OpenPagesUI.swift and Signals.swift.

private final class TestClock: @unchecked Sendable {
    var t: Double
    init(_ t: Double) { self.t = t }
}

private func story(_ route: String, _ title: String, _ section: String = "World", _ entities: [String] = []) -> StoryFacts {
    StoryFacts(route: route, title: title, dek: "", section: section, entities: entities)
}

final class CardsTests: XCTestCase {
    private func prefs(_ c: TestClock = TestClock(1_000_000), file: URL? = nil) -> Preferences {
        Preferences(file: file, now: { Date(timeIntervalSince1970: c.t) })
    }

    // 1. nothing to learn from
    func testStoryWithNothingToLearnFromIsRefused() {
        let p = prefs()
        XCTAssertNil(p.record(kind: "more", story("r", "Untitled", "", [])))
        XCTAssertEqual(p.count, 0)
        XCTAssertNil(p.record(kind: "sideways", story("r", "x", "World", ["A"])))
    }

    // 2. spelling
    func testEntitiesAreNormalisedBeforeMatching() {
        let c = TestClock(1_000_000)
        let p = prefs(c)
        p.record(kind: "more", story("a", "A", "World", ["Harbor Authority"]))
        let other = story("b", "B", "Local", [" harbor  authority "])
        XCTAssertGreaterThan(p.lift(for: other), 0.1)
        XCTAssertEqual(p.lift(for: story("c", "C", "Tech", ["Grid"])), 0, accuracy: 1e-9)
    }

    // 3. double fire
    func testSameGestureWithinTwoSecondsIsOneAct() {
        let c = TestClock(1_000_000)
        let p = prefs(c)
        XCTAssertNotNil(p.record(kind: "more", story("a", "A", "World", ["X"])))
        c.t += 1
        XCTAssertNil(p.record(kind: "more", story("a", "A", "World", ["X"])))
        c.t += 5
        XCTAssertNotNil(p.record(kind: "more", story("a", "A", "World", ["X"])))
        XCTAssertEqual(p.count, 2)
    }

    // 4. corrupt file, 6. gestures survive
    func testCorruptFileStartsEmptyAndGoodFileRoundTrips() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("told-\(UUID().uuidString).json")
        defer { try? FileManager.default.removeItem(at: f) }
        try Data("{not json".utf8).write(to: f)
        XCTAssertEqual(prefs(file: f).count, 0)
        let p = prefs(file: f)
        p.record(kind: "more", story("a", "A", "World", ["X"]))
        XCTAssertTrue(p.setGesture(side: "left", meaning: "close-less"))
        XCTAssertFalse(p.setGesture(side: "left", meaning: "explode"))
        XCTAssertFalse(p.setGesture(side: "up", meaning: "close"))
        let q = prefs(file: f)
        XCTAssertEqual(q.count, 1)
        XCTAssertEqual(q.gestures.left, "close-less")
        XCTAssertEqual(GestureMeanings(left: "bogus", right: "bogus"), GestureMeanings())
    }

    // 5. saturation and cancelling
    func testLiftIsBoundedAndMoreLessCancel() {
        let c = TestClock(1_000_000)
        let p = prefs(c)
        let s = story("a", "A", "World", ["X"])
        for _ in 0..<20 { c.t += 3; p.record(kind: "more", s) }
        XCTAssertLessThanOrEqual(p.lift(for: s), Preferences.maxLift + 1e-9)
        XCTAssertGreaterThan(p.lift(for: s), 0.2)
        let q = prefs(c)
        q.record(kind: "more", s); c.t += 3; q.record(kind: "less", s)
        XCTAssertEqual(q.lift(for: s), 0, accuracy: 1e-6)
        let r = prefs(c)
        for _ in 0..<20 { c.t += 3; r.record(kind: "less", s) }
        XCTAssertGreaterThanOrEqual(r.lift(for: s), -Preferences.maxLift - 1e-9)
    }

    // 6. clock backwards, and decay
    func testFutureActsDoNotWeighMoreThanOneAndOldActsFade() {
        let c = TestClock(1_000_000)
        let p = prefs(c)
        let s = story("a", "A", "World", ["X"])
        p.record(kind: "more", s)
        let fresh = p.lift(for: s)
        c.t -= 86_400 * 3                       // clock goes backwards: age clamps to zero
        XCTAssertEqual(p.lift(for: s), fresh, accuracy: 1e-9)
        c.t = 1_000_000 + 86_400 * 14           // one half-life later
        XCTAssertLessThan(p.lift(for: s), fresh * 0.75)
        c.t = 1_000_000 + 86_400 * 200
        XCTAssertLessThan(p.lift(for: s), 0.005)
    }

    // 7. the shaded order is a stable permutation, and a "more" lifts a related story
    func testOrderIsAStablePermutationAndLiftsRelatedStory() {
        let c = TestClock(1_000_000)
        let p = prefs(c)
        let list = [story("a", "A", "World", ["Harbor"]), story("b", "B", "Tech", ["Grid"]), story("c", "C", "Culture", ["Libraries"]),
                    story("d", "D", "Local", ["Harbor", "Festival"])]
        XCTAssertEqual(p.order(list).map { $0.story.route }, ["a", "b", "c", "d"], "no acts: the paper's order")
        p.record(kind: "more", list[3])
        let o = p.order(list).map { $0.story.route }
        XCTAssertEqual(Set(o), Set(["a", "b", "c", "d"]))
        XCTAssertEqual(o.count, 4)
        XCTAssertLessThan(o.firstIndex(of: "d")!, 3, "the liked story rises")
        XCTAssertLessThan(o.firstIndex(of: "a")!, o.firstIndex(of: "b")!, "its relative with a shared entity stays ahead of the unrelated ones")
        XCTAssertEqual(p.order([]).count, 0)
    }

    // 9. forget and reset
    func testForgetAndReset() {
        let c = TestClock(1_000_000)
        let p = prefs(c)
        XCTAssertEqual(p.forget(route: "nope"), 0)
        XCTAssertEqual(p.reset(), 0)
        let s = story("a", "A", "World", ["X"])
        p.record(kind: "more", s); c.t += 5; p.record(kind: "less", s)
        XCTAssertEqual(p.forget(route: "a"), 2)
        XCTAssertEqual(p.lift(for: s), 0)
        p.record(kind: "more", s)
        XCTAssertEqual(p.reset(), 1)
    }

    func testDocumentLinesAndNotes() {
        let c = TestClock(1_700_000_000)
        let p = prefs(c)
        let s = story("a", "Talks resume", "World", ["Ukraine", "Russia"])
        p.record(kind: "more", s)
        let l = p.lines(timeZone: TimeZone(identifier: "UTC")!)
        XCTAssertEqual(l, ["More like this: Talks resume (Ukraine, Russia) · 14 Nov 22:13"])
        XCTAssertEqual(Preferences.note(kind: "more", s), "More on Ukraine")
        XCTAssertEqual(Preferences.note(kind: "less", s), "Less of Ukraine")
        XCTAssertEqual(Preferences.note(kind: "more", story("z", "z", "", [])), "More like this")
        XCTAssertTrue(p.why(for: s).contains("you asked for more on Ukraine (once)"))
        XCTAssertTrue(p.why(for: story("q", "q", "Tech", ["Grid"])).contains("Nothing you told it"))
    }

    func testGestureMeanings() {
        XCTAssertFalse(GestureMeanings().leftIsLess(surface: "stack"))
        XCTAssertTrue(GestureMeanings(left: "close-less").leftIsLess(surface: "pile"))
        XCTAssertTrue(GestureMeanings(left: "less-in-stack").leftIsLess(surface: "stack"))
        XCTAssertFalse(GestureMeanings(left: "less-in-stack").leftIsLess(surface: "pile"))
    }

    func testEntityDerivationWeak() {
        XCTAssertEqual(StoryTopics.derive(title: "The council of Port Hale approves a barrier"), ["Port Hale"])
        XCTAssertEqual(StoryTopics.derive(title: "libraries extend evening hours"), ["Libraries", "Extend"])
        XCTAssertEqual(StoryTopics.derive(title: ""), [])
        XCTAssertEqual(StoryTopics.entities(route: "r", title: "x", section: "s", given: ["Given"], map: ["r": ["Mapped"]]), ["Given"])
        XCTAssertEqual(StoryTopics.entities(route: "r", title: "x", section: "s", map: ["r": ["Mapped"]]), ["Mapped"])
    }

    // The stack
    func testStackKeepsDecidedFirstUndoesAndIgnoresRepeats() {
        let list = [story("a", "A"), story("b", "B"), story("c", "C")].map { (story: $0, lift: 0.0) }
        let s = StackSession()
        XCTAssertNil(s.undo())
        XCTAssertEqual(s.rows(shaded: list).map(\.story.route), ["a", "b", "c"])
        s.decide(route: "a", side: "left")
        s.decide(route: "a", side: "right")           // repeat: ignored
        s.decide(route: "ghost", side: "left")        // no longer in the list: kept but not counted
        s.decide(route: "b", side: "right")
        let reshaded = [list[2], list[1], list[0]]    // the shading changed the order underneath
        let rows = s.rows(shaded: reshaded)
        XCTAssertEqual(rows.map(\.story.route), ["a", "b", "c"], "decided cards keep the front")
        XCTAssertEqual(rows.map(\.dir), ["left", "right", ""])
        XCTAssertEqual(s.index(shaded: reshaded), 2)
        XCTAssertEqual(s.setAside, 2, "the ghost counts as set aside in the tally only")
        XCTAssertEqual(s.undo()?.route, "b")
        s.reset()
        XCTAssertEqual(s.index(shaded: list), 0)
        XCTAssertEqual(StackSession().rows(shaded: []).count, 0)
    }

    // Launcher: places, search, recents, undo
    func testPlacesHaveOneShortLine() {
        let rows = Places.rows(topStory: "A very long headline " + String(repeating: "x", count: 90) + "\nsecond line", extra: ["mail": "3 unread"])
        XCTAssertEqual(rows.map(\.slug), ["paper", "textures", "files", "mail", "activity", "settings"])
        XCTAssertLessThanOrEqual(rows[0].line.count, 60)
        XCTAssertFalse(rows[0].line.contains("\n"))
        XCTAssertEqual(rows[3].line, "3 unread")
        XCTAssertEqual(rows[1].line, "")
    }

    func testSearchRanksDedupesAndIgnoresPunctuation() {
        let open = [Page(address: "pub:grid", kind: "reader", title: "A regional grid adds a battery")]
        let places = Places.rows(topStory: nil, extra: [:])
        let stories = [story("grid", "A regional grid adds a battery"), story("lib", "City libraries extend hours", "Culture", ["Libraries"]),
                       story("h", "Harbor barrier", "World", ["Gridlock"])]
        XCTAssertEqual(LauncherSearch.hits(query: "  ", open: open, places: places, recent: [], stories: stories).count, 0)
        let h = LauncherSearch.hits(query: "grid", open: open, places: places, recent: [], stories: stories)
        XCTAssertEqual(h.filter { $0.address == "pub:grid" }.count, 1, "one hit per address")
        XCTAssertEqual(h.first?.address, "pub:grid", "the open page scores as a word prefix and leads")
        XCTAssertEqual(LauncherSearch.hits(query: "pap", open: [], places: places, recent: [], stories: []).first?.address, "paper:front")
        XCTAssertEqual(LauncherSearch.hits(query: ".*(", open: open, places: places, recent: [], stories: stories).count, 0)
        XCTAssertEqual(LauncherSearch.hits(query: "libr", open: [], places: places, recent: [], stories: stories).first?.address, "pub:lib")
    }

    func testRecentPagesExcludeOpenAndRepeats() {
        let ev = [SignalEvent(kind: "open", subject: "pub:a", at: 10), SignalEvent(kind: "close", subject: "pub:a", at: 20),
                  SignalEvent(kind: "open", subject: "pub:b", at: 15), SignalEvent(kind: "open", subject: "weird:thing", at: 30),
                  SignalEvent(kind: "more-of-this", subject: "pub:c", at: 40), SignalEvent(kind: "open", subject: "settings:root", at: 5)]
        XCTAssertEqual(RecentPages.addresses(events: ev, excluding: ["settings:root"]), ["pub:a", "pub:b"])
        XCTAssertEqual(RecentPages.addresses(events: ev, excluding: [], limit: 1), ["pub:a"])
        XCTAssertEqual(RecentPages.title(for: "pub:a", stories: [story("a", "Story A")]), "Story A")
        XCTAssertEqual(RecentPages.title(for: "settings:root", stories: []), "Settings")
    }

    func testUndoOfferIsAPureFunctionOfTheClock() {
        let u = UndoOffer(title: "T", address: "pub:a", since: 100)
        XCTAssertTrue(u.isOpen(at: 100)); XCTAssertTrue(u.isOpen(at: 104.9))
        XCTAssertFalse(u.isOpen(at: 105)); XCTAssertFalse(u.isOpen(at: 99))
    }

    // Signals
    func testSignalDetailIsValidJSONWithHostileText() {
        let d = SignalDetail.gesture(surface: "stack", page: "reader", meaning: "more", entities: ["A \"quoted\"\nname", "B"], topic: "World")
        let o = SignalDetail.parse(d)
        XCTAssertEqual(o["surface"] as? String, "stack")
        XCTAssertEqual((o["entities"] as? [String])?.first, "A \"quoted\"\nname")
        XCTAssertEqual(SignalDetail.parse("page").count, 0, "the spine's plain-word details parse to nothing, never throw")
    }

    func testFileSignalsReloadTheirTailAndSkipTornLines() throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("sig-\(UUID().uuidString).jsonl")
        defer { try? FileManager.default.removeItem(at: f) }
        let a = FileSignals(file: f)
        a.record(SignalEvent(kind: "open", subject: "pub:a", detail: "reader", at: 1))
        a.record(SignalEvent(kind: "close", subject: "pub:a", detail: "reader", at: 2))
        let h = try FileHandle(forWritingTo: f); try h.seekToEnd(); try h.write(contentsOf: Data("{torn\n".utf8)); try h.close()
        let b = FileSignals(file: f)
        XCTAssertEqual(b.recent(limit: 10).map(\.kind), ["close", "open"])
        b.record(SignalEvent(kind: "more-of-this", subject: "pub:a", at: 3))
        XCTAssertEqual(FileSignals(file: f).recent(limit: 10).first?.kind, "more-of-this")
        let m = MemorySignals(); m.record(SignalEvent(kind: "x", subject: "y"))
        XCTAssertEqual(m.recent(limit: 5).count, 1)
    }
}
