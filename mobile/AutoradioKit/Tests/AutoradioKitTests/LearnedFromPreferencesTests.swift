import XCTest
@testable import AutoradioKit

// Failure list: see LearnedFromPreferences.swift (1 empty, 2 no entities, 3 reset, 4 order).
final class LearnedFromPreferencesTests: XCTestCase {
    func testLinesAreNewestFirstAndOmitEmptyEntities() {
        let acts = [ToldAct(at: 10, kind: "more", route: "a", title: "Harbor vote", entities: ["Harbor Authority"], topic: "world"),
                    ToldAct(at: 20, kind: "less", route: "b", title: "Chef opens", entities: [], topic: "culture")]
        let l = PreferencesLearned.lines(from: acts)
        XCTAssertEqual(l.map(\.text), ["Less of this: Chef opens", "More like this: Harbor vote (Harbor Authority)"])
        XCTAssertEqual(l.map(\.kind), ["less", "more"])
        XCTAssertTrue(PreferencesLearned.lines(from: []).isEmpty)
    }

    func testResetEmptiesPreferences() async {
        let prefs = Preferences(file: nil)
        XCTAssertNotNil(prefs.record(kind: "more", StoryFacts(route: "r1", title: "Harbor vote", section: "world", entities: ["Harbor Authority"])))
        let p = PreferencesLearned(prefs: { prefs })
        let before = await p.lines(); XCTAssertEqual(before.count, 1)
        await p.reset()
        let after = await p.lines(); XCTAssertEqual(after.count, 0)
        XCTAssertEqual(prefs.count, 0, "the shading's source is emptied, not only the screen")
    }

    func testNoPreferencesYetIsAnEmptyList() async {
        let p = PreferencesLearned(prefs: { nil })
        let l = await p.lines(); XCTAssertTrue(l.isEmpty)
        await p.reset()
    }
}
