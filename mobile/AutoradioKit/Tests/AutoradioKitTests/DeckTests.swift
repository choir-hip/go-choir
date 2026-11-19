import XCTest
@testable import AutoradioKit

// The deck (station X4). No audio is played: the engine is the run sheet tests' FakeEngine, and nothing here reaches a device.
// Failure list, written before the code (each is a test below):
//  1  the journal's numbers never fall or repeat, across a relaunch and an acknowledgement; acknowledged lines leave
//  2  an item a kill left open is closed at the next launch as failed ("interrupted by a restart"), with sound noted, and only once
//  3  heard needs sound: a finish with none is journalled failed; the Swift reading of an ended line agrees with mobile/fixtures/deck
//  4  shared code's play cuts what is playing (replaced); a late end of the cut item closes nothing
//  5  held items play on by themselves; at most three; an id once; `at` makes the list in order; trim
//  6  a transition after an item that was not heard to its end is dropped, with no line
//  7  two items in a row that made no sound stop the deck; its state says stopped
//  8  the session cannot be taken: the item stays on air with no outcome, one state, tried again, then plays
//  9  one state for why it is silent: pause, system pause, resume
// 10  the player journals what it starts and how it ends, and an end written before the app died is read before the run sheet is restored
// 11  the journal is wiped at sign-out
@MainActor
final class DeckTests: XCTestCase {
    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("deck-\(UUID().uuidString).json") }
    func makeDeck(_ journal: DeckJournal? = nil) -> (Deck, FakeEngine) {
        let d = Deck(journal: journal ?? DeckJournal()); let e = FakeEngine(); d.engine = e; d.retryDelays = [0.01]
        return (d, e)
    }
    func speech(_ id: String, requires: Bool = false) -> DeckItem {
        var i = DeckItem(id: id, kind: "speech"); i.paragraphs = "One.\n\nTwo."; i.requiresPreviousHeard = requires; return i
    }
    func ends(_ d: Deck) -> [String] { d.journal.lines.filter { $0.type == "ended" }.map { "\($0.item):\($0.end):\($0.rendered)" } }
    func settle() async { for _ in 0..<5 { await Task.yield() }; try? await Task.sleep(nanoseconds: 30_000_000) }

    // 1
    func test01_NumbersNeverFallOrRepeat() {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f) }
        let j = DeckJournal(storage: f)
        j.append("started", item: "a"); j.append("ended", item: "a", end: "skipped"); j.ack(through: 2)
        XCTAssertEqual(j.lines.count, 0); XCTAssertEqual(j.seq, 2); XCTAssertEqual(j.acked, 2)
        let j2 = DeckJournal(storage: f)
        XCTAssertEqual(j2.seq, 2); XCTAssertEqual(j2.append("started", item: "b").seq, 3)
        j2.ack(through: 1); XCTAssertEqual(j2.acked, 2, "an acknowledgement never goes back")
        XCTAssertEqual(DeckJournal(storage: f).lines.map(\.seq), [3])
    }

    // 2
    func test02_AnItemLeftOpenByAKillIsClosedAsFailedOnce() {
        let j = DeckJournal()
        j.append("started", item: "a"); j.append("sounding", item: "a", block: 2, seconds: 8, rendered: true)
        let l = j.closeOpenAtBoot()
        XCTAssertEqual(l?.end, "failed"); XCTAssertEqual(l?.reason, "interrupted by a restart"); XCTAssertEqual(l?.rendered, true); XCTAssertEqual(l?.block, 2)
        XCTAssertNil(j.closeOpenAtBoot(), "closed once")
        XCTAssertNil(DeckJournal().closeOpenAtBoot())
    }

    // 3
    func test03_HeardNeedsSound() {
        let (d, _) = makeDeck()
        d.began(id: "a"); d.soundBegan(); d.ended(.finished)
        d.began(id: "b"); d.ended(.finished)                         // finished, no sound ever
        XCTAssertEqual(ends(d), ["a:finished:true", "b:failed:false"])
        d.began(id: "c"); d.ended(.finished, played: true)           // the host says a block made sound
        XCTAssertEqual(ends(d).last, "c:finished:true")
        XCTAssertEqual(d.journal.lines.filter { $0.type == "sounding" }.count, 1)
    }
    func test03_TheSwiftReadingAgreesWithTheSharedFixtures() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .deletingLastPathComponent().appendingPathComponent("fixtures/deck")
        let files = try FileManager.default.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil).filter { $0.pathExtension == "json" }
        XCTAssertGreaterThanOrEqual(files.count, 10)
        for f in files {
            let o = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: f)) as? [String: Any])
            let lines = try JSONDecoder().decode([DeckLine].self, from: JSONSerialization.data(withJSONObject: o["journal"] as Any))
            let ops = try XCTUnwrap(o["ops"] as? [[String: Any]])
            for l in lines {
                guard let end = l.airedEnd else { continue }
                // the op the fixture carries for this line: the one whose op_id starts with its number
                guard let op = ops.first(where: { ($0["op_id"] as? String)?.hasPrefix("j\(l.seq).") == true }) else {
                    XCTAssertFalse((o["anomalies"] as? [String] ?? []).isEmpty, "\(f.lastPathComponent) line \(l.seq) has no op and no anomaly")
                    continue          // an anomaly stopped the build before this line
                }
                let ended = try XCTUnwrap(op["ended"] as? [String: Any])
                XCTAssertEqual(op["state"] as? String, end.state.rawValue, f.lastPathComponent)
                XCTAssertEqual(ended["outcome"] as? String, end.outcome.rawValue, f.lastPathComponent)
                XCTAssertEqual(ended["last_block"] as? Int, end.lastBlock, f.lastPathComponent)
                XCTAssertEqual((op["position_seconds"] as? Double) ?? 0, end.seconds, accuracy: 0.001, f.lastPathComponent)
                XCTAssertEqual(op["reason"] as? String, end.reason, f.lastPathComponent)
            }
        }
    }

    // 4
    func test04_PlayCutsWhatIsPlayingAndALateEndClosesNothing() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a")); d.soundBegan()
        _ = await d.play(speech("b"))
        XCTAssertEqual(ends(d), ["a:replaced:true"]); XCTAssertEqual(d.current?.id, "b")
        XCTAssertEqual(e.speaks, ["speak:a", "speak:b"])
        d.documentEnded(id: "deck:a", anyPlayed: true)               // the cut item's end arrives late
        XCTAssertEqual(d.current?.id, "b"); XCTAssertEqual(ends(d).count, 1)
    }

    // 5
    func test05_HeldItemsPlayOnAtMostThreeAndInOrder() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a"))
        d.enqueue(speech("b")); d.enqueue(speech("c")); d.enqueue(speech("d")); d.enqueue(speech("e"))
        XCTAssertEqual(d.snapshot().held, ["b", "c", "d"], "a fourth is refused")
        d.enqueue(speech("x"), at: 1)
        XCTAssertEqual(d.snapshot().held, ["b", "x"], "the held items from the place on are dropped first")
        d.enqueue(speech("b"))
        XCTAssertEqual(d.snapshot().held, ["x", "b"], "an id once")
        d.trim(keeping: 1); XCTAssertEqual(d.snapshot().held, ["x"])
        d.soundBegan(); d.documentEnded(id: "deck:a", anyPlayed: true); await settle()
        XCTAssertEqual(d.current?.id, "x"); XCTAssertEqual(e.speaks.last, "speak:x")
        XCTAssertEqual(ends(d), ["a:finished:true"])
    }

    // 6
    func test06_ATransitionAfterASkipIsDropped() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a")); d.soundBegan()
        d.enqueue(speech("t", requires: true)); d.enqueue(speech("n"))
        await d.act("skip"); await settle()
        XCTAssertEqual(d.current?.id, "n"); XCTAssertFalse(e.speaks.contains("speak:t"))
        XCTAssertEqual(ends(d).first, "a:skipped:true")
        XCTAssertFalse(d.journal.lines.contains { $0.item == "t" }, "dropped without a line")
        // after an item heard to its end it plays
        d.soundBegan(); d.enqueue(speech("t2", requires: true)); d.documentEnded(id: "deck:n", anyPlayed: true); await settle()
        XCTAssertEqual(d.current?.id, "t2")
    }

    // 7
    func test07_TwoSilentItemsInARowStopTheDeck() async {
        let (d, e) = makeDeck()
        e.failSpeak = ["a", "b"]
        _ = await d.play(speech("a"))
        d.enqueue(speech("b")); d.enqueue(speech("c"))
        await settle()
        XCTAssertEqual(d.why, .stopped); XCTAssertNil(d.current)
        XCTAssertEqual(ends(d), ["a:failed:false", "b:failed:false"]); XCTAssertEqual(d.snapshot().held, ["c"])
    }

    // 8
    func test08_TheSessionCannotBeTakenSoTheItemStaysOnAirWithNoOutcome() async {
        let (d, e) = makeDeck()
        var tries = 0
        d.sessionReady = { _ in tries += 1; return tries < 3 ? "another app has the session" : nil }
        _ = await d.play(speech("a"))
        XCTAssertEqual(d.why, .heldSession); XCTAssertEqual(d.current?.id, "a"); XCTAssertTrue(ends(d).isEmpty, "held is no outcome")
        await settle(); await settle()
        XCTAssertEqual(d.why, .playing); XCTAssertEqual(e.speaks, ["speak:a"]); XCTAssertEqual(tries, 3)
        XCTAssertEqual(d.journal.lines.filter { $0.type == "system" }.map(\.command), ["session-held", "session-back"])
        XCTAssertEqual(d.journal.lines.filter { $0.type == "started" }.count, 1)
    }

    // 9
    func test09_OneStateForWhyItIsSilent() async {
        let (d, e) = makeDeck()
        XCTAssertEqual(d.why, .empty)
        _ = await d.play(speech("a")); XCTAssertEqual(d.why, .playing)
        await d.act("pause"); XCTAssertEqual(d.why, .pausedListener); XCTAssertTrue(e.log.contains("pauseSpoken"))
        await d.systemMayResume(); XCTAssertEqual(d.why, .pausedListener, "the system's word never undoes the listener's pause")
        await d.act("resume"); XCTAssertEqual(d.why, .playing)
        d.systemPaused(); XCTAssertEqual(d.why, .pausedSystem)
        await d.systemMayResume(); XCTAssertEqual(d.why, .playing)
        await d.act("stop"); XCTAssertEqual(d.why, .stopped); XCTAssertEqual(ends(d).last, "a:stopped:false")
    }

    // 10
    func test10_ThePlayerJournalsWhatItStartsAndHowItEnds() async {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e; p.deck.engine = e
        let a = RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "A", text: "Words.", estSeconds: 5)
        let b = RunSheetEntry(id: "b", kind: .story, placedBy: .desk, title: "B", text: "Words.", estSeconds: 5)
        s.applyFromDesk(ProgrammeRouteFeed.placeOp(a, cause: "open")); s.applyFromDesk(ProgrammeRouteFeed.placeOp(b, cause: "open"))
        p.wants = true; p.advanceIfIdle(); await p.settle()
        XCTAssertEqual(p.deck.journal.lines.map { "\($0.type):\($0.item)" }, ["started:a"])
        e.rendersSound = true; p.checkSound(); p.checkSound()
        p.documentEnded(id: "rs:a", anyPlayed: true); await p.settle()
        XCTAssertEqual(p.deck.journal.lines.map { "\($0.type):\($0.item):\($0.end)" }, ["started:b:"], "an ended entry's lines are acknowledged once the store holds the op")
        XCTAssertEqual(s.entry("a")?.state, .heard)
        p.next(); await p.settle()
        XCTAssertEqual(s.entry("b")?.state, .skipped)
        XCTAssertEqual(p.deck.journal.seq, 5); XCTAssertTrue(p.deck.journal.lines.isEmpty)
    }
    func test10_AnEndWrittenBeforeTheAppDiedIsReadBeforeTheRunSheetIsRestored() async {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f) }
        let s = RunSheetStore(storage: f); let p = RunSheetPlayer(store: s); let e = FakeEngine(); p.engine = e
        let a = RunSheetEntry(id: "a", kind: .story, placedBy: .desk, title: "A", text: "Words.", estSeconds: 5)
        s.applyFromDesk(ProgrammeRouteFeed.placeOp(a, cause: "open")); p.wants = true; p.advanceIfIdle(); await p.settle()
        XCTAssertEqual(s.onAir()?.id, "a")
        // the deck wrote the end (heard, with sound) and the app died before the store applied it
        let j = DeckJournal()
        j.append("started", item: "a"); j.append("ended", item: "a", block: 1, seconds: 9, final: 1, rendered: true, end: "finished")
        let f3 = tmp(); defer { try? FileManager.default.removeItem(at: f3) }
        try? FileManager.default.copyItem(at: f, to: f3)
        let at = j.lines.last?.at
        let s2 = RunSheetStore(storage: f)
        s2.restore(journal: j)
        XCTAssertEqual(s2.entry("a")?.state, .heard); XCTAssertEqual(s2.entry("a")?.ended?.outcome, .finished)
        XCTAssertEqual(s2.entry("a")?.ended?.at, at)
        XCTAssertTrue(j.lines.isEmpty)
        // with no end in the journal it is as before: failed, never heard
        let s4 = RunSheetStore(storage: f3)
        s4.restore(journal: DeckJournal())
        XCTAssertEqual(s4.entry("a")?.ended?.outcome, .failed)
    }

    // 11
    func test11_TheJournalIsWipedAtSignOut() async {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f) }
        let d = Deck(journal: DeckJournal(storage: f)); d.began(id: "a")
        let w = AccountWipe()
        w.registerStandard(deck: d)
        await w.run()
        XCTAssertEqual(d.journal.seq, 0); XCTAssertTrue(d.journal.lines.isEmpty); XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
    }
}
