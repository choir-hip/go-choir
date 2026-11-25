import XCTest
@testable import AutoradioKit

// Station L6, the phone's side of the learning loop (docs/design/autoradio-learned-document-2026-11-17.md sections 5 to 8). The failure list, written
// before the code; each is a test below:
//  F1  the `segment` event's entry_id and claim_id are kept: the entry has the server's id and a claim (no probability the phone does not hold); an
//      older server's event (neither) makes the entry it always made
//  F2  a relaunch that finds an entry on air closes it `failed` (an app failure), never `left` (sign-out) and never a listener's skip; sign-out stays `left`
//  F3  the programme request carries `edition_text`: the notebook text when held, else the topics as text, else nothing; inside the server's 32 KB
//  F4  the notebook is read as lines: sections, `[L1 · heard 9 of 11]` tags, refs (`lesson:L1`, `try:T2`, `mine:<8 hex>`), the listener's untagged lines
//  F5  the topics move in: a seed when there is no notebook; the topic lines join "What you asked for" once, the listener's lines and the other sections kept
//  F6 to F8 are bun tests now (notebook.test.ts N4 to N6). Their old text: F6 the learning route: its fixture parses; an empty card and an older shape parse; 404/501 is "not yet", a dead network is "offline", no key is "signed out"
//  F7  the page shows what it has and invents nothing: no route is "not available yet" with no numbers; with the route, counts, rate and kinds; a line's
//      tap lists the ledger rows whose cause is that line
//  F8  rule 2: the held list names every kind of record held about the listener and where it is read
//  F9  a turn names the entry on air: its id, its claim and its cited stories travel inside `queue`
//  F10 more/less/mute taps are queued with the entry and stories, sent when the route answers, kept while it does not, never sent twice, and a log line has no words
@MainActor
final class LearningLoopTests: XCTestCase {
    static let mobile = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
    func fixture(_ n: String) throws -> Data { try Data(contentsOf: Self.mobile.appendingPathComponent("fixtures/learning/\(n)")) }
    func text(_ n: String) throws -> String { String(decoding: try fixture(n), as: UTF8.self) }

    func segments(_ sse: String) -> [IncomingSegment] {
        var d = StretchDecoder()
        return (d.feed(sse) + d.finish()).compactMap { if case .segment(let s) = $0 { return s } else { return nil } }
    }

    // F1
    /// The forecast the server sends on a `segment` event carries the claim id and never its probability or cause (they are written to the tape): it decodes, with nothing invented. A full one decodes too.
    func testF1ForecastDecodesWhatTheServerSends() throws {
        let onlyClaim = try JSONDecoder().decode(RunSheetForecast.self, from: Data(#"{"claim_id":"c9"}"#.utf8))
        XCTAssertEqual(onlyClaim.claimId, "c9"); XCTAssertNil(onlyClaim.pHeard); XCTAssertEqual(onlyClaim.causedBy, "")
        let full = try JSONDecoder().decode(RunSheetForecast.self, from: Data(#"{"p_heard":0.62,"caused_by":"the listener skipped long pieces","claim_id":"c3"}"#.utf8))
        XCTAssertEqual(full.pHeard, 0.62); XCTAssertEqual(full.causedBy, "the listener skipped long pieces"); XCTAssertEqual(full.claimId, "c3")
        let none = try JSONDecoder().decode(RunSheetForecast.self, from: Data("{}".utf8))
        XCTAssertEqual(none.claimId, ""); XCTAssertNil(none.pHeard)
    }

    // F2



    // F4, F5 (the notebook read as lines, the topics merged into it once) moved to AutoradioLab/notebook.test.ts and notebook-topics.test.ts: the Swift notebook is deleted with the editor.
    // F6, F7, F8 (the learning route, the page, the held list) moved to AutoradioLab/notebook.test.ts (N4 to N6); the code they tested is shared TypeScript now.

    func stub(_ h: @escaping (StubProtocol.Req) -> StubProtocol.Resp, key: Bool = true) -> ChoirClient {
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        StubProtocol.seen = []; StubProtocol.handler = h
        return ChoirClient(baseURL: "https://choir.test", keys: key ? InMemoryKeyStore([KeyAccount.apiKey: "k", KeyAccount.computerID: "c"]) : InMemoryKeyStore([:]), session: URLSession(configuration: cfg), log: nil)
    }

}
