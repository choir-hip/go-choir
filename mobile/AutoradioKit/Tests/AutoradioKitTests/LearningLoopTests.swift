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
//  F6  the learning route: its fixture parses; an empty card and an older shape parse; 404/501 is "not yet", a dead network is "offline", no key is "signed out"
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
    func testF1TheSegmentEventKeepsTheServersEntryAndClaim() throws {
        let segs = segments(try text("segment-settle.sse"))
        XCTAssertEqual(segs.count, 2)
        XCTAssertEqual(segs[0].entryID, "d-s7-1"); XCTAssertEqual(segs[0].claimID, "calib:autoradio-dj:s7:1")
        XCTAssertEqual(segs[1].entryID, "d-s7-2"); XCTAssertNil(segs[1].claimID, "a transition has no forecast")
        let e = try XCTUnwrap(ProgrammeRouteFeed.entry(from: segs[0], clips: ClipStore()))
        XCTAssertEqual(e.id, "d-s7-1", "the server's entry id is the phone's, so an aired op reaches the entry the server settles")
        XCTAssertEqual(e.forecast?.claimId, "calib:autoradio-dj:s7:1"); XCTAssertNil(e.forecast?.pHeard, "no probability is invented")
        let wire = try JSONSerialization.jsonObject(with: JSONEncoder().encode(ProgrammeRouteFeed.placeOp(e, cause: "open"))) as! [String: Any]
        let f = ((wire["entry"] as! [String: Any])["forecast"]) as! [String: Any]
        XCTAssertEqual(f["claim_id"] as? String, "calib:autoradio-dj:s7:1"); XCTAssertNil(f["p_heard"])
        let t = try XCTUnwrap(ProgrammeRouteFeed.entry(from: segs[1], clips: ClipStore()))
        XCTAssertNil(t.forecast); XCTAssertEqual(t.id, "d-s7-2")
    }
    func testF1AnOlderServersEventMakesTheEntryItAlwaysMade() throws {
        let s = try XCTUnwrap(segments(try text("segment-old.sse")).first)
        XCTAssertNil(s.entryID); XCTAssertNil(s.claimID)
        let e = try XCTUnwrap(ProgrammeRouteFeed.entry(from: s, clips: ClipStore()))
        XCTAssertEqual(e.id, ProgrammeRouteFeed.entryID("s8/1")); XCTAssertNil(e.forecast)
    }
    func testF1AClaimWithNoProbabilityStillRoundTripsAndShowsNoForecastSentence() throws {
        var e = RunSheetEntry(id: "d-1", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 10)
        e.forecast = RunSheetForecast(claimId: "c9"); e.state = .onAir
        let back = try JSONDecoder().decode(RunSheetEntry.self, from: JSONEncoder().encode(e))
        XCTAssertEqual(back.forecast?.claimId, "c9"); XCTAssertNil(back.forecast?.pHeard)
        // (no number, no sentence: radio.test.ts 8)
    }

    // F2
    func testF2ARelaunchClosesTheEntryOnAirAsFailedNeverLeft() async throws {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("lp-\(UUID().uuidString).json")
        let s1 = RunSheetStore(storage: f); let p1 = RunSheetPlayer(store: s1); p1.engine = FakeEngine()
        p1.pageText = { _, t in (t, ["one"]) }
        let e = RunSheetEntry(id: "d-9", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 10)
        s1.applyFromDesk(ProgrammeRouteFeed.placeOp(e, cause: "open")); s1.airedOnAir("d-9"); s1.save()
        let s2 = RunSheetStore(storage: f); s2.restore()
        let closed = try XCTUnwrap(s2.entry("d-9"))
        XCTAssertEqual(closed.ended?.outcome, .failed); XCTAssertNotEqual(closed.ended?.outcome, .left)
        let op = try XCTUnwrap(s2.pending.last { $0.op == .aired && $0.state != .onAir })
        XCTAssertEqual(op.by, .phone); XCTAssertEqual(op.ended?.outcome, .failed); XCTAssertEqual(op.reason, "interrupted by a restart")
        XCTAssertFalse(s2.pending.contains { $0.op == .skip }, "the listener chose nothing")
    }
    func testF2SignOutIsStillLeft() async {
        let s = RunSheetStore(storage: nil); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        let e = RunSheetEntry(id: "d-9", kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 10)
        s.applyFromDesk(ProgrammeRouteFeed.placeOp(e, cause: "open")); s.airedOnAir("d-9")
        p.stopAll()
        XCTAssertEqual(s.entry("d-9")?.ended?.outcome, .left)
    }

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

    // F6
    func testF6TheLearningRouteParses() throws {
        let r = try XCTUnwrap(LearningWire.parse(try fixture("learning.json")))
        XCTAssertEqual(r.card.forecasts, 14); XCTAssertEqual(r.card.settled, 11); XCTAssertEqual(r.card.heardThrough, 8)
        XCTAssertEqual(try XCTUnwrap(r.card.heardThroughRate), 0.7272727, accuracy: 0.0001)
        XCTAssertEqual(r.card.byKind.map(\.label), ["story", "podcast_clip"]); XCTAssertEqual(r.card.byCause.map(\.label), ["lesson", "importance"])
        XCTAssertEqual(r.card.noVerdict, 1); XCTAssertEqual(r.rows.count, 4)
        XCTAssertEqual(r.rows[1].status, "settled"); XCTAssertEqual(r.rows[1].heardThrough, true); XCTAssertEqual(r.rows[1].op?.outcome, "finished")
        XCTAssertEqual(r.rows[3].status, "no_verdict"); XCTAssertEqual(r.rows[3].noVerdictReason, "left"); XCTAssertNil(r.rows[3].heardThrough)
        let e = try XCTUnwrap(LearningWire.parse(try fixture("learning-empty.json")))
        XCTAssertEqual(e.card.settled, 0); XCTAssertNil(e.card.heardThroughRate); XCTAssertTrue(e.rows.isEmpty)
        XCTAssertNotNil(LearningWire.parse(Data(#"{"scorecard":{"settled":3},"rows":[{"claim_id":"c"}]}"#.utf8)), "an older shape with fields missing still reads")
        XCTAssertNil(LearningWire.parse(Data("[1,2]".utf8)))
    }
    func stub(_ h: @escaping (StubProtocol.Req) -> StubProtocol.Resp, key: Bool = true) -> ChoirClient {
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        StubProtocol.seen = []; StubProtocol.handler = h
        return ChoirClient(baseURL: "https://choir.test", keys: key ? InMemoryKeyStore([KeyAccount.apiKey: "k", KeyAccount.computerID: "c"]) : InMemoryKeyStore([:]), session: URLSession(configuration: cfg), log: nil)
    }
    func testF6AMissingRouteIsNotYetADeadNetworkIsOfflineNoKeyIsSignedOut() async throws {
        let ok = await LearningClient(client: stub { _ in .init(raw: (try? String(contentsOf: Self.mobile.appendingPathComponent("fixtures/learning/learning.json"))) ?? "{}") }).read()
        guard case .ready(let r) = ok else { return XCTFail("\(ok)") }
        XCTAssertEqual(r.card.settled, 11); XCTAssertEqual(StubProtocol.seen.last?.path, "/api/autoradio/learning")
        for status in [404, 501] { let l = await LearningClient(client: stub { _ in .init(status: status) }).read(); XCTAssertEqual(l, .notYet, "\(status)") }
        let l5 = await LearningClient(client: stub { _ in .init(status: 503) }).read(); XCTAssertEqual(l5, .offline)
        let l6 = await LearningClient(client: stub({ _ in .init() }, key: false)).read(); XCTAssertEqual(l6, .signedOut)
        let l7 = await LearningClient(client: stub { _ in .init(raw: "not json") }).read(); XCTAssertEqual(l7, .notYet, "an answer in no shape we know is treated as not there yet")
    }

    // F7
    func testF7WithoutTheRouteThePageSaysSoAndShowsNoNumbers() throws {
        let nb = NotebookText.parse(try text("notebook.md"))
        let p = NotebookPage.make(NotebookPageInput(doc: .found(address: "texture:nb1", notebook: nb), learning: .notYet))
        XCTAssertEqual(p.scoreState, "notyet"); XCTAssertFalse(p.scoreNotice.isEmpty); XCTAssertTrue(p.score.isEmpty); XCTAssertTrue(p.byKind.isEmpty)
        XCTAssertTrue(p.scoreNotice.contains("not available yet"))
        XCTAssertEqual(p.docState, "ready"); XCTAssertEqual(p.docAddress, "texture:nb1"); XCTAssertEqual(p.sections.count, 4)
        let none = NotebookPage.make(NotebookPageInput(doc: .none, learning: .offline))
        XCTAssertEqual(none.docState, "none"); XCTAssertEqual(none.scoreState, "offline"); XCTAssertTrue(none.sections.isEmpty)
        let down = NotebookPage.make(NotebookPageInput(doc: .unavailable, learning: .signedOut))
        XCTAssertEqual(down.docState, "offline"); XCTAssertFalse(down.docNotice.isEmpty)
    }
    func testF7WithTheRouteItShowsCountsAndTheRowsBehindALine() throws {
        let nb = NotebookText.parse(try text("notebook.md"))
        let read = try XCTUnwrap(LearningWire.parse(try fixture("learning.json")))
        var input = NotebookPageInput(doc: .found(address: "texture:nb1", notebook: nb), learning: .ready(read))
        var p = NotebookPage.make(input)
        XCTAssertEqual(p.scoreState, "ready")
        XCTAssertEqual(p.score.map(\.label), ["Forecasts settled", "Heard through", "Still open"]); XCTAssertEqual(p.score.map(\.value), ["11", "8 of 11 (73%)", "2"])
        XCTAssertEqual(p.byKind.map(\.label), ["Story", "Podcast clip"]); XCTAssertEqual(p.byKind.map(\.value), ["6 of 8 heard through", "2 of 3 heard through"])
        XCTAssertEqual(p.noVerdict, "1 forecast with no verdict: you left, or the app could not play it.")
        let lesson = try XCTUnwrap(p.sections[1].lines.first)
        XCTAssertEqual(lesson.tag, "heard 9 of 11"); XCTAssertTrue(lesson.canWhy)
        input.selected = lesson.id
        p = NotebookPage.make(input)
        XCTAssertEqual(p.selectedText, lesson.text); XCTAssertEqual(p.evidence.map(\.title), ["Council keeps libraries open late", "A regional grid adds its largest battery", "Harbor fees rise"])
        XCTAssertEqual(p.evidence.map(\.outcome), ["Not yet heard", "Heard through", "Skipped"])
        XCTAssertEqual(p.evidence[2].detail, "The desk forecast 80 in 100.")
        let mine = try XCTUnwrap(p.sections[0].lines.first); input.selected = mine.id
        p = NotebookPage.make(input)
        XCTAssertTrue(p.evidence.isEmpty); XCTAssertFalse(p.evidenceNote.isEmpty, "a line with no forecasts says so; nothing is made up")
    }

    // F8
    func testF8EverythingHeldAboutTheListenerIsListedWithWhereItIsRead() {
        let held = NotebookPage.held
        XCTAssertGreaterThanOrEqual(held.count, 12)
        for h in held { XCTAssertFalse(h.what.isEmpty); XCTAssertFalse(h.whereRead.isEmpty, h.what) }
        let all = held.map(\.what).joined(separator: "\n")
        for kind in ["autoradio.turn", "autoradio.forecast", "autoradio.settlement", "autoradio.sheet_op", "autoradio.lesson", "autoradio.strike", "autoradio.note", "autoradio.take", "autoradio.no_verdict"] {
            XCTAssertTrue(all.contains(kind), kind)
        }
        for local in ["More and less taps", "Topics", "Mail"] { XCTAssertTrue(all.contains(local), local) }
        XCTAssertTrue(held.contains { $0.whereRead.contains("only on this phone") })
    }

    // F9
    func testF9ATurnNamesTheEntryOnAirItsClaimAndItsStories() async throws {
        let s = RunSheetStore(storage: nil); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        var e = RunSheetEntry(id: "d-s7-1", kind: .story, placedBy: .desk, title: "Harbor fees rise", text: "Words.",
                              cites: [RunSheetCite(ref: "pub:harbor-fees@v2", role: .subject, label: "Harbor fees rise"), RunSheetCite(ref: "pub:tide@v1", role: .mentioned, label: "Tide tables")], estSeconds: 10)
        e.forecast = RunSheetForecast(claimId: "calib:autoradio-dj:s7:1")
        s.applyFromDesk(ProgrammeRouteFeed.placeOp(e, cause: "open")); s.airedOnAir("d-s7-1")
        let q = try JSONSerialization.jsonObject(with: try XCTUnwrap(p.turnQueueJSON())) as! [String: Any]
        let air = try XCTUnwrap(q["on_air"] as? [String: Any])
        XCTAssertEqual(air["id"] as? String, "d-s7-1"); XCTAssertEqual(air["claim_id"] as? String, "calib:autoradio-dj:s7:1"); XCTAssertEqual(air["title"] as? String, "Harbor fees rise")
        XCTAssertEqual(air["stories"] as? [String], ["harbor-fees", "tide"])
        XCTAssertTrue(q["current"] is NSNull, "the old `current` stays as it was: a spoken entry has no item to hand over")
        var turn = DeskTurn(turnID: "t1", programmeID: "p", blockIndex: 0, listenerWords: "less of this"); turn.queue = p.turnQueueJSON()
        let body = AutoradioDeskStage.body(turn, id: "t1", history: [])
        XCTAssertNil(body["on_air"], "the turn's top level gains no field (the route refuses unknown ones)"); XCTAssertNotNil((body["queue"] as? [String: Any])?["on_air"])
    }

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
