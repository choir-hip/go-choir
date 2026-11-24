import XCTest
@testable import AutoradioKit

// Kept turns on a listener's phone (KeptTurnStore.prune): the newest 20, none older than 30 days, and never any sound. The failure list, written first:
//  K1 more than `keep` turns: the oldest folders go, the newest stay.
//  K2 a turn older than the age limit goes whatever the count.
//  K3 a turn folder from an earlier build that holds mic.caf and reference.caf loses the sound and keeps its words (record.json, context.json).
//  K4 a store with nothing in it prunes nothing and does not throw.
final class KeptTurnsPruneTests: XCTestCase {
    private func store() -> KeptTurnStore { KeptTurnStore(root: FileManager.default.temporaryDirectory.appendingPathComponent("kept-\(UUID().uuidString)")) }
    private func turn(_ s: KeptTurnStore, _ id: String, ageDays: Double, sound: Bool) throws {
        let f = s.root.appendingPathComponent(id); try FileManager.default.createDirectory(at: f, withIntermediateDirectories: true)
        try Data("{}".utf8).write(to: f.appendingPathComponent("record.json")); try Data("{}".utf8).write(to: f.appendingPathComponent("context.json"))
        if sound { try Data([0]).write(to: f.appendingPathComponent("mic.caf")); try Data([0]).write(to: f.appendingPathComponent("reference.caf")) }
        try FileManager.default.setAttributes([.modificationDate: Date().addingTimeInterval(-ageDays * 86_400)], ofItemAtPath: f.path)
    }
    private func names(_ s: KeptTurnStore) -> [String] { ((try? FileManager.default.contentsOfDirectory(atPath: s.root.path)) ?? []).sorted() }
    func testK1OldestBeyondTheCountGo() throws {
        let s = store(); for i in 0..<5 { try turn(s, "t\(i)", ageDays: Double(i), sound: false) }
        XCTAssertEqual(s.prune(keep: 3), 2); XCTAssertEqual(names(s), ["t0", "t1", "t2"])
    }
    func testK2OldTurnsGoWhateverTheCount() throws {
        let s = store(); try turn(s, "new", ageDays: 1, sound: false); try turn(s, "old", ageDays: 40, sound: false)
        XCTAssertEqual(s.prune(keep: 20, maxAgeDays: 30), 1); XCTAssertEqual(names(s), ["new"])
    }
    func testK3SoundIsRemovedWordsStay() throws {
        let s = store(); try turn(s, "t", ageDays: 1, sound: true)
        _ = s.prune(); let f = s.root.appendingPathComponent("t")
        XCTAssertFalse(FileManager.default.fileExists(atPath: f.appendingPathComponent("mic.caf").path)); XCTAssertFalse(FileManager.default.fileExists(atPath: f.appendingPathComponent("reference.caf").path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: f.appendingPathComponent("record.json").path)); XCTAssertEqual(s.list().count, 1)
    }
    func testK4EmptyStore() { XCTAssertEqual(store().prune(), 0) }
}
