import XCTest
@testable import AutoradioKit

/// A relaunch five minutes after an opening was heard: the one-opening rule refuses a second greeting, and the radio does not wait for the desk;
/// the first playable entry ahead (kept from the last listening, or the cached first entry) starts at once.
@MainActor
final class QuickRelaunchTests: XCTestCase {
    func test_RelaunchWithinTheWindowStartsWithTheEntryAheadAndNoDeskReply() async {
        let now = Date(timeIntervalSince1970: 1_800_000_000)
        let s = RunSheetStore(); s.now = { now }
        let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        // the last listening: an opening heard five minutes ago, and a cached first entry placed ahead for this launch
        var heard = RunSheetEntry(id: "p-old", kind: .opening, placedBy: .phone, title: "Opening", text: "Good morning.", estSeconds: 5)
        heard.state = .heard
        heard.ended = RunSheetEnded(outcome: .finished, at: ISO8601DateFormatter().string(from: now.addingTimeInterval(-300)), lastBlock: 0)
        s.replaceSheet(RunSheet(entries: [heard]))
        let first = RunSheetEntry(id: "d-first", kind: .story, placedBy: .desk, title: "The harbour", text: "The harbour barrier was approved.", estSeconds: 8)
        s.place(first, at: .end, by: .desk)
        // the relaunch: a new opening is offered, and refused
        p.open(PhoneOpening.entry(lines: [.init(title: "A", dek: "a.")], now: now))
        _ = await poll { e.log.contains("speak:d-first") }
        XCTAssertTrue(e.log.contains("speak:d-first"), "the first sound is the entry that was ahead, with no desk reply: \(e.log)")
        XCTAssertFalse(e.log.contains { $0.hasPrefix("speak:p-") }, "no second greeting")
    }
}
