import XCTest
@testable import AutoradioKit

// Written from the failure list at the top of BarState.swift.
final class BarStateTests: XCTestCase {
    private func snap(_ phase: String = "playing", running: Bool = true, paused: Bool = false, level: String = "play", idx: Int = 0, count: Int = 3, text: String = "") -> LabSnapshot {
        var s = LabSnapshot(); s.running = running; s.paused = paused; s.phase = phase; s.level = level
        s.blockIndex = idx; s.blockCount = count; s.transcript = text; return s
    }
    private func player(playing: Bool, title: String = "T", message: String = "") -> ChoirPlayerView {
        var p = ChoirPlayerView(); p.signedIn = true; p.playing = playing; p.title = title; p.message = message; return p
    }
    private let t0 = Date(timeIntervalSince1970: 1000)

    // (The bar's states, words and windows are bar.ts: bar.test.ts. What is native is what the engine told it, kept raw.)
    func testEventsAreKeptRawWithTheirTime() {
        let tr = BarTracker()
        tr.observe(.holdingSaid("Looking at the third paragraph…"), now: t0)
        XCTAssertEqual(tr.raw().holding, "Looking at the third paragraph…")
        tr.observe(.phase(from: "playing", to: "candidate"), now: t0)
        XCTAssertEqual(tr.raw().holding, "", "a new turn clears the old line")
        tr.observe(.revisionArrived(id: "r", fromBlock: 1, blocks: 2, summary: "s"), now: t0)
        XCTAssertEqual(tr.raw().revisedAt, 1000)
        tr.observe(.error(stage: "desk", message: "boom"), now: t0.addingTimeInterval(10))
        XCTAssertEqual(tr.raw().errorStage, "desk"); XCTAssertEqual(tr.raw().errorMessage, "boom"); XCTAssertEqual(tr.raw().errorAt, 1010)
        tr.observe(.error(stage: "desk", message: "boom"), now: t0.addingTimeInterval(13))
        XCTAssertEqual(tr.raw().errorAt, 1013, "a repeat moves the time, so the window restarts")
        tr.observe(.error(stage: "other", message: "x"), now: t0.addingTimeInterval(20))
        XCTAssertEqual(tr.raw().errorAt, 1013, "only the engine's own stages are errors for the bar")
        XCTAssertEqual(tr.expiry(now: t0.addingTimeInterval(5))!, 1.0, accuracy: 0.01)
        XCTAssertNil(tr.expiry(now: t0.addingTimeInterval(30)))
    }

    func testClockCountsOnlyRunningTime() {
        var c = BlockClock()
        c.update(index: 0, running: true, now: t0)
        c.update(index: 0, running: false, now: t0.addingTimeInterval(2))
        c.update(index: 0, running: false, now: t0.addingTimeInterval(9))
        c.update(index: 0, running: true, now: t0.addingTimeInterval(10))
        XCTAssertEqual(c.seconds(now: t0.addingTimeInterval(11)), 3, accuracy: 0.001)
        c.update(index: 1, running: true, now: t0.addingTimeInterval(11))
        XCTAssertEqual(c.seconds(now: t0.addingTimeInterval(11)), 0, accuracy: 0.001)
    }

    func testSentencesAndIndex() {
        let s = BarTracker.sentences("One short. Two is a longer sentence here. Three.")
        XCTAssertEqual(s.count, 3)
        XCTAssertEqual(BarTracker.sentenceIndex(s, progress: 0.0), 0)
        XCTAssertEqual(BarTracker.sentenceIndex(s, progress: 0.99), 2)
        XCTAssertEqual(BarTracker.sentenceIndex([], progress: 0.5), 0)
    }

    func testRadioItemsWindowKeepsCurrentAndClampsIndex() {
        let tr = BarTracker()
        let blocks = (0..<100).map { ProgrammeBlock.speech(text: "Block \($0). More words.") }
        let d = ProgrammeDocument(id: "d", title: "Doc", blocks: blocks)
        let r = tr.radioInfo(player: player(playing: true), snapshot: snap(idx: 99, count: 100), document: d, route: "pub:x", now: t0)
        XCTAssertEqual(r.items.count, 40); XCTAssertTrue(r.items.contains { $0.current })
        let over = tr.radioInfo(player: player(playing: true), snapshot: snap(idx: 500, count: 100), document: d, route: "", now: t0)
        XCTAssertEqual(over.index, 99)
        let empty = tr.radioInfo(player: player(playing: false, title: ""), snapshot: snap(count: 0), document: ProgrammeDocument(id: "n", title: "", blocks: []), route: "", now: t0)
        XCTAssertEqual(empty.count, 0); XCTAssertTrue(empty.items.isEmpty); XCTAssertFalse(empty.paused)
    }
}
