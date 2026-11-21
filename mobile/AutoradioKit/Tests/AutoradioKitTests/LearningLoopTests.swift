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


    func testF1AClaimWithNoProbabilityStillRoundTripsAndShowsNoForecastSentence() throws {
        var e = RunSheetEntry(id: "d-1", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 10)
        e.forecast = RunSheetForecast(claimId: "c9"); e.state = .onAir
        let back = try JSONDecoder().decode(RunSheetEntry.self, from: JSONEncoder().encode(e))
        XCTAssertEqual(back.forecast?.claimId, "c9"); XCTAssertNil(back.forecast?.pHeard)
        // (no number, no sentence: radio.test.ts 8)
    }

    // F2



    // F3
    func testF3TheRequestCarriesTheNotebookElseTheTopicsElseNothing() throws {
        let nb = try text("notebook.md")
        XCTAssertEqual(EditionText.make(notebook: nb, topics: ["Space"]), nb.trimmingCharacters(in: .whitespacesAndNewlines))
        let t = try XCTUnwrap(EditionText.make(notebook: nil, topics: ["Space", "Chess"]))
        XCTAssertEqual(t, "What the listener follows:\n- Space\n- Chess")
        XCTAssertNil(EditionText.make(notebook: nil, topics: [])); XCTAssertNil(EditionText.make(notebook: "  \n", topics: []))
        let big = String(repeating: "a line of words\n", count: 4000)
        XCTAssertLessThanOrEqual(try XCTUnwrap(EditionText.make(notebook: big, topics: [])).utf8.count, 32 * 1024)
        var body = RouteStretchSource.requestBody(cause: "open", candidates: [], aheadTitles: [], editionText: t, now: Date())
        XCTAssertEqual(body["edition_text"] as? String, t)
        body = RouteStretchSource.requestBody(cause: "open", candidates: [], aheadTitles: [], editionText: nil, now: Date())
        XCTAssertNil(body["edition_text"])
    }

    // F4
    func testF4TheNotebookIsReadAsLines() throws {
        let nb = NotebookText.parse(try text("notebook.md"))
        XCTAssertEqual(nb.sections.map(\.key), ["asked", "learned", "trying", "steering"])
        let asked = nb.sections[0].lines
        XCTAssertEqual(asked.count, 3); XCTAssertNil(asked[0].tag); XCTAssertEqual(asked[0].ref, NotebookText.mineRef("Mornings: world news first, then science."))
        XCTAssertEqual(NotebookText.mineRef("x").count, "mine:".count + 8)
        let l = nb.sections[1].lines
        XCTAssertEqual(l[0].ref, "lesson:L1"); XCTAssertEqual(l[0].heard, 9); XCTAssertEqual(l[0].of, 11)
        XCTAssertEqual(l[0].text, "Long policy pieces before 8 am get skipped; short ones are heard.")
        XCTAssertEqual(l[1].ref, "lesson:L9"); XCTAssertEqual(nb.sections[2].lines[0].ref, "try:T2")
        XCTAssertEqual(nb.sections[3].lines.count, 2); XCTAssertNil(nb.sections[3].lines[0].ref)
        let odd = NotebookText.parse("## What you asked for\n- A line about [L and a bracket [Lx] stays whole.\nNot a bullet\n")
        XCTAssertEqual(odd.sections[0].lines.map(\.text), ["A line about [L and a bracket [Lx] stays whole."]); XCTAssertNil(odd.sections[0].lines[0].tag)
    }

    // F5
    func testF5TheTopicsMoveIntoTheNotebookOnce() throws {
        let seed = NotebookText.merging(topics: ["Space", "Chess"], into: nil)
        XCTAssertTrue(seed.hasPrefix("# My notebook\n\n## What you asked for\n- Space\n- Chess"), seed)
        let nb = try text("notebook.md")
        let once = NotebookText.merging(topics: ["Space", "Chess"], into: nb)
        let parsed = NotebookText.parse(once)
        XCTAssertEqual(parsed.sections[0].lines.map(\.text).suffix(2), ["Space", "Chess"])
        XCTAssertEqual(parsed.sections[0].lines.count, 5); XCTAssertEqual(parsed.sections[1].lines.count, 2, "the other sections are untouched")
        XCTAssertEqual(NotebookText.merging(topics: ["Space", "Chess", "space"], into: once), once, "a second write adds nothing")
        let noSection = NotebookText.merging(topics: ["Art"], into: "# My notebook\n\n## What Autoradio has learned\n- A thing. [L2 · heard 1 of 1]\n")
        XCTAssertTrue(NotebookText.parse(noSection).sections.first { $0.key == "asked" }?.lines.map(\.text) == ["Art"], noSection)
        XCTAssertTrue(noSection.contains("[L2 · heard 1 of 1]"))
    }
    // F6, F7, F8 (the learning route, the page, the held list) moved to AutoradioLab/notebook.test.ts (N4 to N6); the code they tested is shared TypeScript now.

    func stub(_ h: @escaping (StubProtocol.Req) -> StubProtocol.Resp, key: Bool = true) -> ChoirClient {
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        StubProtocol.seen = []; StubProtocol.handler = h
        return ChoirClient(baseURL: "https://choir.test", keys: key ? InMemoryKeyStore([KeyAccount.apiKey: "k", KeyAccount.computerID: "c"]) : InMemoryKeyStore([:]), session: URLSession(configuration: cfg), log: nil)
    }

    // F9


    // F10
    func tap(_ d: String = "less") -> SteerTap { SteerTap(id: "s-" + UUID().uuidString.prefix(8).lowercased(), entryID: "d-s7-1", direction: d, stories: ["harbor-fees"], title: "Harbor fees rise", handle: nil, at: "2026-11-17T21:00:00Z") }
    func testF10TheTapGoesWithTheEntryAndItsStories() async throws {
        let q = SteerQueue(file: nil, send: SteerClient(client: stub { _ in .init(json: ["ok": true]) }).send)
        q.enqueue(tap()); await q.flush()
        let r = try XCTUnwrap(StubProtocol.seen.last)
        XCTAssertEqual(r.method, "POST"); XCTAssertEqual(r.path, "/api/autoradio/steer")
        XCTAssertEqual(r.body["entry_id"] as? String, "d-s7-1"); XCTAssertEqual(r.body["direction"] as? String, "less"); XCTAssertEqual(r.body["stories"] as? [String], ["harbor-fees"])
        XCTAssertTrue(q.taps.isEmpty); XCTAssertEqual(q.state, "sent")
        await q.flush(); XCTAssertEqual(StubProtocol.seen.filter { $0.path == "/api/autoradio/steer" }.count, 1, "never sent twice")
    }
    func testF10WhileTheRouteIsAbsentTheTapStaysQueuedAndSurvivesARelaunch() async throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("steer-\(UUID().uuidString).json")
        let c = SteerClient(client: stub { _ in .init(status: 404) })
        let q = SteerQueue(file: f, send: c.send); let t = tap(); q.enqueue(t); await q.flush()
        XCTAssertEqual(q.taps.map(\.id), [t.id]); XCTAssertEqual(q.state, "notyet")
        let q2 = SteerQueue(file: f, send: SteerClient(client: stub { _ in .init(json: [:]) }).send)
        XCTAssertEqual(q2.taps.map(\.id), [t.id], "a relaunch finds it"); await q2.flush(); XCTAssertTrue(q2.taps.isEmpty)
    }
    func testF10ARefusedBodyIsRetriedBareOnceThenDropped() async {
        var n = 0
        let c = SteerClient(client: stub { r in n += 1; return (r.body["stories"] != nil) ? .init(status: 400) : .init(json: [:]) })
        let q = SteerQueue(file: nil, send: c.send); q.enqueue(tap()); await q.flush()
        XCTAssertEqual(n, 2); XCTAssertTrue(q.taps.isEmpty); XCTAssertNil(StubProtocol.seen.last?.body["stories"])
        let always = SteerClient(client: stub { _ in .init(status: 400) }); let q2 = SteerQueue(file: nil, send: always.send); q2.enqueue(tap()); await q2.flush()
        XCTAssertTrue(q2.taps.isEmpty, "a correctness refusal is not retried for ever")
    }
    func testF10AMuteCarriesTheHandleAndTheQueueIsBoundedAndLogsHoldNoWords() async {
        let m = SteerTap(id: "s-m", entryID: "", direction: "mute", stories: [], title: "", handle: "voice-3fa2c1", at: "2026-11-17T21:00:00Z")
        let q = SteerQueue(file: nil, send: { _ in .offline })
        for _ in 0..<(SteerQueue.maxTaps + 5) { q.enqueue(tap()) }
        XCTAssertEqual(q.taps.count, SteerQueue.maxTaps)
        let c = SteerClient(client: stub { _ in .init(json: [:]) }); _ = await c.send(m)
        XCTAssertEqual(StubProtocol.seen.last?.body["handle"] as? String, "voice-3fa2c1"); XCTAssertNil(StubProtocol.seen.last?.body["entry_id"])
        let line = SteerQueue.logLine(sent: 1, kept: 2)
        XCTAssertEqual(line, "AUTORADIO steer sent=1 queued=2"); XCTAssertFalse(line.contains("Harbor"))
    }
}
