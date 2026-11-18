import XCTest
@testable import AutoradioKit

@MainActor
final class LocalEntryKeepTests: XCTestCase {
    final class Capture: RunSheetPoster, @unchecked Sendable {
        let lock = NSLock(); private var _ops: [RunSheetOp] = []
        var ops: [RunSheetOp] { lock.lock(); defer { lock.unlock() }; return _ops }
        func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome { lock.lock(); _ops += ops; lock.unlock(); return .answered([]) }
        func fetchSheet() async -> RunSheet? { nil }
    }
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("keep-\(UUID().uuidString).json") }

    func testTheCapDropsTheOldest() {
        let k = LocalEntryKeep(file: nil)
        for i in 0..<(LocalEntryKeep.cap + 5) { k.put(id: "e\(i)", title: "t", label: "l") }
        XCTAssertEqual(k.count, LocalEntryKeep.cap); XCTAssertNil(k.row("e0")); XCTAssertNotNil(k.row("e54"))
    }
    func testAStaleGenerationWriteIsDropped() {
        let k = LocalEntryKeep(file: nil)
        let g = AccountWipe.generation
        AccountWipe.bumpGeneration()
        k.put(id: "x", title: "t", label: "l", generation: g)
        XCTAssertNil(k.row("x"))
    }
    func testSignOutEmptiesTheKeepAndItsFile() async {
        let f = tmp(); let k = LocalEntryKeep(file: f)
        k.put(id: "x", title: "t", label: "l", rest: "r")
        XCTAssertTrue(FileManager.default.fileExists(atPath: f.path))
        let w = AccountWipe(); w.register("local-entries") { k.wipe() }
        await w.run()
        XCTAssertEqual(k.count, 0); XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
        XCTAssertNotNil(AccountWipe().names.count as Int?)
    }
    func testTheKeepSurvivesARelaunch() {
        let f = tmp(); LocalEntryKeep(file: f).put(id: "x", title: "Real", label: "L", rest: "The rest.")
        XCTAssertEqual(LocalEntryKeep(file: f).row("x")?.rest, "The rest.")
    }
    func testAKeptContinuationPlaysTheRestAfterARelaunch() async {
        // the process forgot `provided`; the keep still has the words
        let k = LocalEntryKeep.shared
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        var c = RunSheetEntry(id: "c-keep1", kind: .mail, placedBy: .phone, title: "A message", estSeconds: 3)
        c.source = RunSheetSource(ref: "mail:m1", label: "A message")
        k.put(id: c.id, title: "Real subject", label: "Real", rest: "The rest of it.")
        p.playNow(c); await p.settle()
        XCTAssertEqual(e.speaks, ["speak:c-keep1"], "the rest is spoken, not 'Could not play'")
        XCTAssertEqual(p.message, "")
    }
    func testPostedOpsNameNoLocalTitleWhileTheRowShowsIt() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        p.registerTextSource(kind: "document") { _, _ in ["Block."] }
        let poster = Capture(); s.poster = poster
        let book = RunSheetEntry.book(id: "b1", title: "Confidential Merger Memoir", at: 0)
        let mail = RunSheetEntry.readAloud(address: "mail:m9", title: "Zebrafish quarterly invoice mentions Dolores")
        let file = RunSheetEntry.filesAudio(address: "file:/Podcasts/x.mp3", title: "Tuesday Marmalade Lecture")
        s.place(book, at: .end); s.place(mail, at: .end); s.place(file, at: .end)
        await s.flushTask?.value; s.flush(); await s.flushTask?.value
        let all = poster.ops.map { String(decoding: (try? JSONEncoder().encode($0)) ?? Data(), as: UTF8.self).lowercased() }.joined()
        XCTAssertFalse(poster.ops.isEmpty)
        for w in ["confidential", "merger", "memoir", "zebrafish", "dolores", "marmalade", "lecture", "tuesday"] { XCTAssertFalse(all.contains(w), w) }
        XCTAssertEqual(book.shownTitle(), "Confidential Merger Memoir")
        XCTAssertEqual(mail.shownTitle(), "Zebrafish quarterly invoice mentions Dolores")
        XCTAssertEqual(file.shownTitle(), "Tuesday Marmalade Lecture")
        XCTAssertEqual(book.displayTitle, "Read aloud", "what the server is asked about stays neutral")
    }
}
