import XCTest
@testable import AutoradioKit

// The Autoradio page's gestures (the ops they send) and the shape of the facts native hands over. The page's model is shared code: radio.test.ts and radio-page.test.ts (the tests that were here for
// the status lines, rows, greeting and "built on" were ported there first).

@MainActor
final class RadioPageTests: XCTestCase {
    func entry(_ id: String, _ kind: RunSheetKind = .story, by: RunSheetBy = .desk, state: RunSheetState = .ahead, title: String? = nil,
               est: Double = 60, outcome: RunSheetOutcome? = nil) -> RunSheetEntry {
        var e = RunSheetEntry(id: id, kind: kind, placedBy: by, state: state, title: title ?? "Title \(id)", text: kind.textRequired ? "Words." : nil, estSeconds: est)
        if let outcome { e.ended = RunSheetEnded(outcome: outcome) }
        return e
    }
    // 13 the gestures
    func makePlayer(_ es: [RunSheetEntry]) -> (RunSheetPlayer, RunSheetStore) {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        for e in es { s.place(e, at: .end, by: e.placedBy, reason: "test") }
        return (p, s)
    }
    func ids(_ s: RunSheetStore) -> [String] { s.sheet.ahead().map(\.id) }

    func test13a_ReorderIsOneMoveOp() {
        let (p, s) = makePlayer(["a", "b", "c", "d"].map { entry($0) })
        let before = s.pending.count
        XCTAssertNotNil(p.perform(.reorder(id: "d", before: "b")))
        XCTAssertEqual(ids(s), ["a", "d", "b", "c"])
        XCTAssertEqual(s.pending.count, before + 1)
        let op = s.pending.last!
        XCTAssertEqual(op.op, .move); XCTAssertEqual(op.at, .after); XCTAssertEqual(op.after, "a"); XCTAssertEqual(op.id, "d"); XCTAssertEqual(op.by, .listener)
        p.perform(.reorder(id: "c", before: "a")); XCTAssertEqual(ids(s), ["c", "a", "d", "b"]); XCTAssertEqual(s.pending.last?.at, .next)
        p.perform(.reorder(id: "c", before: nil)); XCTAssertEqual(ids(s), ["a", "d", "b", "c"]); XCTAssertEqual(s.pending.last?.at, .end)
        let n = s.pending.count
        XCTAssertNil(p.perform(.reorder(id: "c", before: nil)), "already last: nothing is sent")
        XCTAssertEqual(s.pending.count, n)
        XCTAssertNil(p.perform(.reorder(id: "zzz", before: "a")))
    }

    func test13b_ASwipeIsOneRemoveOp() {
        let (p, s) = makePlayer(["a", "b", "c"].map { entry($0) })
        p.perform(.remove(id: "b"))
        XCTAssertEqual(ids(s), ["a", "c"]); XCTAssertEqual(s.pending.last?.op, .remove); XCTAssertEqual(s.pending.last?.id, "b"); XCTAssertEqual(s.pending.last?.by, .listener)
        let n = s.pending.count
        XCTAssertNil(p.perform(.remove(id: "b"))); XCTAssertEqual(s.pending.count, n, "a second swipe sends nothing")
    }

    func test13c_ATapIsThePlayNowOpTheSheetUsed() {
        let (p, s) = makePlayer(["a", "b", "c"].map { entry($0) })
        p.perform(.playNow(id: "c"))
        XCTAssertEqual(s.pending.last?.op, .move); XCTAssertEqual(s.pending.last?.at, .now); XCTAssertEqual(s.pending.last?.id, "c")
        XCTAssertEqual(s.onAir()?.id, "c"); XCTAssertEqual(ids(s), ["a", "b"], "what was ahead stays ahead")
    }

    func test13d_ReplayFromHeardPlacesAFreshCopyNow() {
        let (p, s) = makePlayer(["a", "b"].map { entry($0) })
        s.airedOnAir("a"); s.airedEnded("a", as: .heard, outcome: .finished)
        XCTAssertEqual(s.sheet.heard().first?.id, "a")
        p.perform(.replay(id: "a"))
        let op = s.pending.last!
        XCTAssertEqual(op.op, .place); XCTAssertEqual(op.at, .now); XCTAssertNotEqual(op.entry?.id, "a")
        XCTAssertEqual(op.entry?.title, "Title a"); XCTAssertEqual(s.onAir()?.title, "Title a"); XCTAssertEqual(s.onAir()?.state, .onAir)
        XCTAssertNil(p.perform(.replay(id: "b")), "an entry still ahead is not replayed")
    }

    // 14 a stale gesture is dropped by the reducer, not repaired here
    func test14_AGestureForAnEntryNoLongerAheadChangesNothing() {
        let (p, s) = makePlayer(["a", "b"].map { entry($0) })
        let v = s.sheet.version
        XCTAssertNil(p.perform(.playNow(id: "gone"))); XCTAssertNil(p.perform(.remove(id: "gone")))
        XCTAssertEqual(s.sheet.version, v)
    }

    // The page is empty on the device when the shape the module answers is not the shape the contract declares (an extra or missing field makes the resource fail).
    // Every Swift field of the facts is in interface/radio.json, and the other way.
    func test17_TheFactsHaveExactlyTheFieldsTheContractDeclares() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("exact-package/interface/radio.json")
        let j = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        let shapes = try XCTUnwrap(j["shapes"] as? [String: [String: String]])
        let got = Set(try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(RadioPageFacts())) as? [String: Any]).keys)
        XCTAssertEqual(got, Set(try XCTUnwrap(shapes["RadioPageFacts"]).keys), "the Swift fields and the declared shape differ")
        let def = try XCTUnwrap(((j["names"] as? [[String: Any]])?.first { ($0["source"] as? String) == "radioPageFacts" }?["substitute"] as? [String: Any])?["default"] as? [String: Any])
        XCTAssertEqual(Set(def.keys), got, "the substitute default has every field")
    }
}
