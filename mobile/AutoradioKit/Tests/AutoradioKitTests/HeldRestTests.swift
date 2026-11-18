import XCTest
@testable import AutoradioKit

@MainActor
final class HeldRestTests: XCTestCase {
    final class Capture: RunSheetPoster, @unchecked Sendable {
        let lock = NSLock(); private var _ops: [RunSheetOp] = []
        var ops: [RunSheetOp] { lock.lock(); defer { lock.unlock() }; return _ops }
        func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome { lock.lock(); _ops += ops; lock.unlock(); return .answered([]) }
        func fetchSheet() async -> RunSheet? { nil }
    }
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("keep-\(UUID().uuidString).json") }

    func testTheCapDropsTheOldest() {
        let k = HeldRest(file: nil)
        for i in 0..<(HeldRest.cap + 5) { k.put(id: "e\(i)", rest: "r") }
        XCTAssertEqual(k.count, HeldRest.cap); XCTAssertNil(k.row("e0")); XCTAssertNotNil(k.row("e54"))
    }
    func testAStaleGenerationWriteIsDropped() {
        let k = HeldRest(file: nil)
        let g = AccountWipe.generation
        AccountWipe.bumpGeneration()
        k.put(id: "x", rest: "r", generation: g)
        XCTAssertNil(k.row("x"))
    }
    func testSignOutEmptiesTheStoreAndItsFile() async {
        let f = tmp(); let k = HeldRest(file: f)
        k.put(id: "x", rest: "r")
        XCTAssertTrue(FileManager.default.fileExists(atPath: f.path))
        let w = AccountWipe(); w.register("held-rest") { k.wipe() }
        await w.run()
        XCTAssertEqual(k.count, 0); XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
        XCTAssertNotNil(AccountWipe().names.count as Int?)
    }
    func testTheRestSurvivesARelaunch() {
        let f = tmp(); HeldRest(file: f).put(id: "x", rest: "The rest.")
        XCTAssertEqual(HeldRest(file: f).row("x")?.rest, "The rest.")
    }
    func testAHeldContinuationPlaysTheRestAfterARelaunch() async {
        // the process forgot `provided`; the store still has the words
        let k = HeldRest.shared
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        var c = RunSheetEntry(id: "c-keep1", kind: .mail, placedBy: .phone, title: "Real subject", estSeconds: 3)
        c.source = RunSheetSource(ref: "mail:m1", label: "Real subject")
        k.put(id: c.id, rest: "The rest of it.")
        p.playNow(c); await p.settle()
        XCTAssertEqual(e.speaks, ["speak:c-keep1"], "the rest is spoken, not 'Could not play'")
        XCTAssertEqual(p.message, "")
    }
    func testPostedOpsCarryTheRealTitlesOfABookAMessageAndAFile() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        p.registerTextSource(kind: "document") { _, _ in ["Block."] }
        let poster = Capture(); s.poster = poster
        let book = RunSheetEntry.book(id: "b1", title: "Confidential Merger Memoir", at: 0)
        let mail = RunSheetEntry.readAloud(address: "mail:m9", title: "Zebrafish quarterly invoice mentions Dolores")
        let file = RunSheetEntry.filesAudio(address: "file:/Podcasts/x.mp3", title: "Tuesday Marmalade Lecture")
        s.place(book, at: .end); s.place(mail, at: .end); s.place(file, at: .end)
        await s.flushTask?.value; s.flush(); await s.flushTask?.value
        let all = poster.ops.map { String(decoding: (try? JSONEncoder().encode($0)) ?? Data(), as: UTF8.self) }.joined()
        XCTAssertFalse(poster.ops.isEmpty)
        for w in ["Confidential Merger Memoir", "Zebrafish quarterly invoice mentions Dolores", "Tuesday Marmalade Lecture"] { XCTAssertTrue(all.contains(w), w) }
        for w in ["Read aloud", "A message", "A file"] { XCTAssertFalse(all.contains(w), w) }
        XCTAssertEqual(book.displayTitle, "Confidential Merger Memoir"); XCTAssertEqual(mail.displayTitle, "Zebrafish quarterly invoice mentions Dolores")
    }
}
