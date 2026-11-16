import XCTest
@testable import AutoradioKit

// Slice A of the Autoradio desk. The Go reducer (internal/runsheet) and this one read the same fixtures, mobile/fixtures/runsheet/*.json,
// each `{start, ops, want, results}` (plus `decode_error` where the ops must fail to decode), and agree on every one.
// The failure list, each item a fixture or a test below:
//   move after on air dropped (01) · rewrite of a heard entry dropped (02) · aired from the desk refused (03) · aired backwards refused (04)
//   place with a vanished after lands next (05) · queued after the listener's last placement (06) · now cuts the entry on air (07)
//   duplicate op_id is a no-op (08) · clear at a stale version dropped (09) · one version per applied op (10)
//   unknown entry fields ignored (11) · unknown op is an error (12, 14) · the Go and Swift tests read the same files.
final class RunSheetTests: XCTestCase {
    static let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        .deletingLastPathComponent().appendingPathComponent("fixtures/runsheet")

    static func fixtureFiles() throws -> [URL] {
        try FileManager.default.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil).filter { $0.pathExtension == "json" }
            .sorted { $0.lastPathComponent < $1.lastPathComponent }
    }

    struct Loaded {
        var start: RunSheet
        var want: RunSheet
        var results: [RunSheetResult]
        var ops: [RunSheetOp]
        var decodeError: Bool
        var opsDecodeFailed: Bool
    }

    func load(_ url: URL) throws -> Loaded {
        let data = try Data(contentsOf: url)
        let root = try XCTUnwrap(JSONSerialization.jsonObject(with: data) as? [String: Any])
        func part<T: Decodable>(_ key: String, _ type: T.Type) throws -> T {
            let v = try XCTUnwrap(root[key], "\(url.lastPathComponent): no \(key)")
            return try JSONDecoder().decode(T.self, from: JSONSerialization.data(withJSONObject: v))
        }
        var ops: [RunSheetOp] = []
        var failed = false
        for raw in try XCTUnwrap(root["ops"] as? [Any]) {
            do { ops.append(try JSONDecoder().decode(RunSheetOp.self, from: JSONSerialization.data(withJSONObject: raw))) } catch { failed = true; break }
        }
        return Loaded(start: try part("start", RunSheet.self), want: try part("want", RunSheet.self), results: try part("results", [RunSheetResult].self),
                      ops: ops, decodeError: (root["decode_error"] as? Bool) ?? false, opsDecodeFailed: failed)
    }

    func testEverySharedFixtureAgrees() throws {
        let files = try Self.fixtureFiles()
        XCTAssertGreaterThanOrEqual(files.count, 20)
        for f in files {
            let fx = try load(f)
            let name = f.lastPathComponent
            if fx.decodeError { XCTAssertTrue(fx.opsDecodeFailed, "\(name): expected the ops to fail to decode"); continue }
            XCTAssertFalse(fx.opsDecodeFailed, "\(name): ops failed to decode")
            let (sheet, results) = RunSheetReducer.applyAll(fx.start, fx.ops)
            XCTAssertEqual(sheet, fx.want, name)
            XCTAssertEqual(results, fx.results, name)
        }
    }

    func fixtureOutcome(_ prefix: String) throws -> (Loaded, RunSheet, [RunSheetResult]) {
        let f = try XCTUnwrap(try Self.fixtureFiles().first { $0.lastPathComponent.hasPrefix(prefix) }, prefix)
        let fx = try load(f)
        let (s, r) = RunSheetReducer.applyAll(fx.start, fx.ops)
        return (fx, s, r)
    }

    // The failure list, one test each, over the fixtures (the whole-sheet comparison is in the loop above).
    func testMoveAfterItWentOnAirIsDropped() throws { let (_, s, r) = try fixtureOutcome("01_"); XCTAssertEqual(r.map(\.dropped), [nil, "not_ahead", "not_ahead"]); XCTAssertEqual(s.onAir, "b") }
    func testRewriteOfHeardIsDropped() throws { let (_, _, r) = try fixtureOutcome("02_"); XCTAssertEqual(r.first?.dropped, "not_ahead") }
    func testAiredFromTheDeskIsRefused() throws { let (_, s, r) = try fixtureOutcome("03_"); XCTAssertEqual(r.map(\.dropped), ["not_phone", "not_phone"]); XCTAssertEqual(s.version, 2) }
    func testAiredBackwardsIsRefused() throws { let (_, _, r) = try fixtureOutcome("04_"); XCTAssertEqual(r.prefix(3).map(\.dropped), ["bad_transition", "bad_transition", "bad_transition"]) }
    func testPlaceWithAVanishedAfterLandsNext() throws { let (_, s, _) = try fixtureOutcome("05_"); XCTAssertEqual(s.entries.map(\.id), ["h", "a", "z", "x", "b", "y", "c"]) }
    func testQueuedLandsAfterTheListenersLastPlacement() throws { let (_, s, _) = try fixtureOutcome("06_"); XCTAssertEqual(s.entries.map(\.id), ["a", "d1", "l1", "q", "d2"]) }
    func testNowCutsTheEntryOnAir() throws {
        let (_, s, _) = try fixtureOutcome("07_")
        XCTAssertEqual(s.entry("a")?.state, .skipped); XCTAssertEqual(s.entry("a")?.ended?.outcome, .cut); XCTAssertEqual(s.onAir, "x")
    }
    func testADuplicateOpIdIsANoOp() throws { let (_, s, r) = try fixtureOutcome("08_"); XCTAssertEqual(r.map(\.dropped), [nil, "duplicate", "duplicate"]); XCTAssertEqual(s.entries.map(\.id), ["a", "x"]) }
    func testClearAtAStaleVersionIsDropped() throws { let (_, _, r) = try fixtureOutcome("09_"); XCTAssertEqual(r.first?.dropped, "stale_version") }
    func testVersionIncrementsOncePerAppliedOp() throws {
        let (fx, s, r) = try fixtureOutcome("10_")
        XCTAssertEqual(s.version, fx.start.version + Int64(r.filter(\.applied).count))
    }

    func testUnknownOpAndKindAreDecodeErrors() {
        let json = #"{"op_id":"o1","by":"phone","op":"teleport","base_version":0}"#
        XCTAssertThrowsError(try JSONDecoder().decode(RunSheetOp.self, from: Data(json.utf8)))
        let e = #"{"id":"x","kind":"jingle","placed_by":"desk","state":"ahead","title":"","est_seconds":1}"#
        XCTAssertThrowsError(try JSONDecoder().decode(RunSheetEntry.self, from: Data(e.utf8)))
    }

    func testUnknownEntryFieldsAreIgnored() throws {
        let e = #"{"id":"x","kind":"page","placed_by":"listener","state":"ahead","title":"t","est_seconds":1,"wibble":{"a":[1]}}"#
        let entry = try JSONDecoder().decode(RunSheetEntry.self, from: Data(e.utf8))
        XCTAssertEqual(entry.id, "x")
    }

    func testRoundTripThroughJSON() throws {
        let f = try XCTUnwrap(try Self.fixtureFiles().first { $0.lastPathComponent.hasPrefix("19_") })
        let fx = try load(f)
        let data = try JSONEncoder().encode(fx.want)
        XCTAssertEqual(try JSONDecoder().decode(RunSheet.self, from: data), fx.want)
        let o = try JSONEncoder().encode(fx.ops[0])
        XCTAssertEqual(try JSONDecoder().decode(RunSheetOp.self, from: o), fx.ops[0])
    }

    func testAppliedOpsAreBoundedAndTheInputIsNotChanged() {
        var s = RunSheet()
        let before = s
        _ = s.applying(RunSheetOp(opId: "z", by: .listener, op: .clear, baseVersion: 0))
        XCTAssertEqual(s, before)
        for i in 0..<(RunSheetBounds.maxAppliedOps + 10) {
            let (n, r) = s.applying(RunSheetOp(opId: "o\(i)", by: .listener, op: .clear, baseVersion: s.version))
            XCTAssertTrue(r.applied); s = n
        }
        XCTAssertEqual(s.appliedOps.count, RunSheetBounds.maxAppliedOps)
        XCTAssertEqual(s.version, Int64(RunSheetBounds.maxAppliedOps + 10))
        XCTAssertEqual(s.applying(RunSheetOp(opId: "o\(RunSheetBounds.maxAppliedOps + 9)", by: .listener, op: .clear, baseVersion: s.version)).result.dropped, "duplicate")
    }

    func testValidId() {
        for ok in ["a", "p-1a2b3c4d", "d-turn_7.2-3", String(repeating: "a", count: 64)] { XCTAssertTrue(RunSheet.validId(ok), ok) }
        for bad in ["", "-a", ".a", "a b", "a/b", String(repeating: "a", count: 65), "é"] { XCTAssertFalse(RunSheet.validId(bad), bad) }
    }
}
