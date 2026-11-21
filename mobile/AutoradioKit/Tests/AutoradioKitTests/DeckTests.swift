import XCTest
@testable import AutoradioKit

/// The engine the deck tests play through: it plays nothing, it records what it was asked.
@MainActor
final class FakeEngine: PlaybackEngine {
    var log: [String] = []
    var failSpeak: Set<String> = []
    var duration: Double?
    var elapsed = 0.0
    var position: (block: Int, seconds: Double) = (0, 0)
    var holdSpeak: (() async -> Void)?
    func speak(documentID: String, entryID: String, title: String, kind: String, paragraphs: [String]) async -> Bool { await holdSpeak?(); log.append("speak:\(entryID)"); return !failSpeak.contains(entryID) }
    var report: (block: Int, final: Int, seconds: Double) = (0, 0, 0)
    func spokenReport() -> (block: Int, final: Int, seconds: Double) { report }
    func restartSpoken() { log.append("restart") }
    func resumeSpoken() { log.append("resumeSpoken") }
    var remainder: String?
    var rendersSound = false
    func rendering() -> Bool { rendersSound }
    func pauseSpoken() { log.append("pauseSpoken") }
    func spokenRemainder() -> String? { remainder }
    func spokenPosition() -> (block: Int, seconds: Double) { position }
    func stopSpoken() { log.append("stopSpoken") }
    func endSpokenAfterCurrentBlock() { log.append("endAfterBlock") }
    func playRemote(url: URL, title: String, show: String, start: Double?, end: Double?) async -> Bool {
        log.append("remote:\(url.lastPathComponent):\(start ?? -1):\(end ?? -1)"); return true
    }
    func pauseRemote() { log.append("pauseRemote") }
    func resumeRemote() { log.append("resumeRemote") }
    func stopRemote() { log.append("stopRemote") }
    func seekRemote(by s: Double) { log.append("seekBy:\(s)") }
    func seekRemote(to s: Double) { log.append("seekTo:\(s)") }
    func clearRemoteEnd() { log.append("clearEnd") }
    func remotePosition() -> (elapsed: Double, duration: Double?)? { (elapsed, 600) }
    func setRemoteRate(_ r: Double) {}
    func setRemoteLevel(_ l: PlayLevel) {}
    func remoteDuration(url: URL, deadline: TimeInterval) async -> Double? { log.append("duration"); return duration }
    var speaks: [String] { log.filter { $0.hasPrefix("speak:") } }
}


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
// 17  the whole deck is wiped at sign-out; 18 no level op, no rate field
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
    func test05_HeldItemsPlayOnUpToTheSheetsBoundAndInOrder() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a"))
        d.enqueue(speech("b")); d.enqueue(speech("c")); d.enqueue(speech("d")); d.enqueue(speech("e"))
        XCTAssertEqual(d.snapshot().held, ["b", "c", "d", "e"], "every ready entry is held")
        for n in 0..<(Deck.heldMax + 5) { d.enqueue(speech("m\(n)")) }
        XCTAssertEqual(d.snapshot().held.count, Deck.heldMax, "one past the sheet's bound is refused")
        d.trim(keeping: 0); d.enqueue(speech("b")); d.enqueue(speech("c"))
        d.enqueue(speech("x"), at: 1)
        XCTAssertEqual(d.snapshot().held, ["b", "x"], "the held items from the place on are dropped first")
        d.enqueue(speech("b"))
        XCTAssertEqual(d.snapshot().held, ["x", "b"], "an id once")
        d.trim(keeping: 1); XCTAssertEqual(d.snapshot().held, ["x"])
        d.soundBegan(); d.documentEnded(id: "deck:a", anyPlayed: true); await settle()
        XCTAssertEqual(d.current?.id, "x"); XCTAssertEqual(e.speaks.last, "speak:x")
        XCTAssertEqual(ends(d), ["a:finished:true"])
    }

    // the lock screen acts on the deck at once; the journal says so first, with where the item was
    func test23_TheLockScreensNextSkipsAndPlaysOnAtOnceAndIsJournalledFirstWithThePlace() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a")); d.enqueue(speech("b")); d.enqueue(speech("c"))
        e.report = (block: 3, final: 5, seconds: 11)
        await d.remoteNext(); await settle()
        XCTAssertEqual(d.current?.id, "b"); XCTAssertEqual(e.speaks.last, "speak:b"); XCTAssertEqual(d.snapshot().held, ["c"])
        let l = d.journal.lines.filter { $0.type == "remote" || $0.type == "ended" }
        XCTAssertEqual(l.first?.type, "remote"); XCTAssertEqual(l.first?.command, "next"); XCTAssertEqual(l.first?.item, "a")
        XCTAssertEqual(l.first?.block, 3); XCTAssertEqual(l.first?.seconds, 11)
        XCTAssertEqual(l.last?.end, "skipped")
    }
    func test24_TheLockScreensPreviousRestartsAudioAndAnEntryThatHasGoneOnAndLeavesAJustBegunOneToSharedCode() async {
        let (d, e) = makeDeck()
        var a = DeckItem(id: "au", kind: "audio"); a.url = "https://x/a.mp3"; a.start = 12
        _ = await d.play(a); e.elapsed = 40
        d.remotePrevious(); XCTAssertTrue(e.log.contains("seekTo:12.0"), "audio goes back to where it began")
        _ = await d.play(speech("s")); e.report = (block: 2, final: 5, seconds: 9)
        d.remotePrevious(); XCTAssertEqual(e.log.filter { $0 == "restart" }.count, 1)
        _ = await d.play(speech("t")); e.report = (block: 0, final: 5, seconds: 1)
        d.remotePrevious(); XCTAssertEqual(e.log.filter { $0 == "restart" }.count, 1, "an entry just begun: the entry before it is shared code's")
        XCTAssertEqual(d.journal.lines.filter { $0.command == "previous" }.count, 3)
    }
    func test25_ARunDryListHasALineAndAudioAndHeardSpeechJournalSounding() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a")); e.rendersSound = true
        _ = d.snapshot(); _ = d.snapshot()
        XCTAssertEqual(d.journal.lines.filter { $0.type == "sounding" }.count, 1, "written once, when the engine renders")
        d.documentEnded(id: "deck:a", anyPlayed: true); await settle()
        XCTAssertTrue(d.journal.lines.contains { $0.command == "held-dry" }, "nothing held to play on")
        var au = DeckItem(id: "au", kind: "audio"); au.url = "https://x/a.mp3"
        _ = await d.play(au)
        XCTAssertEqual(d.journal.lines.filter { $0.type == "sounding" && $0.item == "au" }.count, 1)
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



    // 11
    func test11_TheJournalIsWipedAtSignOut() async {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f) }
        let d = Deck(journal: DeckJournal(storage: f)); d.began(id: "a")
        let w = AccountWipe()
        w.registerStandard(takeOwn: TakeOwn(), takeMutes: TakeMutes(), takeStore: TakeStore(), legacyTakeFiles: [], tempDirectory: f.deletingLastPathComponent().appendingPathComponent("wipe-tmp-\(UUID().uuidString)"), deck: d)
        await w.run()
        XCTAssertEqual(d.journal.seq, 0); XCTAssertTrue(d.journal.lines.isEmpty); XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
    }

    // Review 2026-11-19 (deck, seat 2), 7 and 8:
    //  J6 sign-out wipes the DECK, not only its journal: the item on air, the held items and the reason go, the engine stops, and a late end of the old
    //     account's item writes nothing into the new account's journal
    //  J7 the interface carries no `level` op and no `rate` (a no-op and a constant): acts that remain move something
    func test17_SignOutWipesTheDeckAndALateEndWritesNothing() async {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f); try? FileManager.default.removeItem(at: DeckJournal.seqFile(for: f)) }
        let (d, e) = makeDeck(DeckJournal(storage: f))
        _ = await d.play(speech("a")); d.soundBegan(); d.enqueue(speech("b"))
        XCTAssertNotNil(d.current); XCTAssertEqual(d.held.count, 1)
        let w = AccountWipe(); w.registerStandard(takeOwn: TakeOwn(), takeMutes: TakeMutes(), takeStore: TakeStore(), legacyTakeFiles: [], tempDirectory: f.deletingLastPathComponent().appendingPathComponent("wipe-tmp-\(UUID().uuidString)"), deck: d); await w.run()
        XCTAssertNil(d.current); XCTAssertTrue(d.held.isEmpty); XCTAssertEqual(d.why, .empty)
        XCTAssertTrue(e.log.contains("stopSpoken") || e.log.contains("stopRemote"))
        XCTAssertTrue(d.journal.lines.isEmpty); XCTAssertEqual(d.journal.seq, 0)
        XCTAssertNil(d.ended(.finished, played: true, id: "a"))      // the late end of the old account's item
        d.documentEnded(id: "deck:a", anyPlayed: true)
        XCTAssertTrue(d.journal.lines.isEmpty)
        // and it plays for the next account as a fresh deck
        _ = await d.play(speech("c")); XCTAssertEqual(d.journal.lines.first?.item, "c"); XCTAssertEqual(d.journal.lines.first?.seq, 1)
    }
    func test18_TheInterfaceTellsNoLie() async {
        let (d, e) = makeDeck()
        _ = await d.play(speech("a"))
        let before = e.log
        await d.act("level", value: 0.2)       // unknown now: nothing moves, nothing is journalled
        XCTAssertEqual(e.log, before)
        let json = String(data: (try? JSONEncoder().encode(d.snapshot())) ?? Data(), encoding: .utf8) ?? ""
        XCTAssertFalse(json.contains("\"rate\""))
    }

    // Review 2026-11-19 (deck, seat 1), A: the journal is never overwritten by a read that failed.
    //  J1 a file that is there but cannot be read is not "empty": nothing is saved over it, and once it can be read the lines made meanwhile are merged after it
    //  J2 the numbers live in a file of their own: they survive a lost lines file and never repeat
    //  J3 a failed save is counted and tried again at the next event
    //  J4 the cap never drops silently: a `system` line says how many unacknowledged lines went
    //  J5 an `ended` line of an EARLIER airing of the same entry is not taken for this one's end
    func test12_AnUnreadableJournalIsNotOverwrittenAndNothingIsLost() throws {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f); try? FileManager.default.removeItem(at: DeckJournal.seqFile(for: f)) }
        let j = DeckJournal(storage: f)
        j.append("started", item: "a"); j.append("ended", item: "a", end: "skipped")
        let good = try Data(contentsOf: f)
        // the file is there and cannot be read (a background launch before first unlock: modelled by no read permission)
        try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: f.path)
        let j2 = DeckJournal(storage: f)
        XCTAssertFalse(j2.isLoaded)
        j2.append("started", item: "b")
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: f.path)
        XCTAssertEqual(try Data(contentsOf: f), good, "nothing is saved over a file that could not be read")
        try good.write(to: f)                                   // the phone is unlocked: the file reads
        let l = j2.append("ended", item: "b", end: "skipped")
        XCTAssertTrue(j2.isLoaded)
        XCTAssertEqual(j2.lines.map { "\($0.type):\($0.item)" }, ["started:a", "ended:a", "started:b", "ended:b"])
        XCTAssertEqual(j2.lines.map(\.seq), [1, 2, 3, 4], "the unreadable time's lines are numbered after the file's")
        XCTAssertEqual(l.seq, 4); XCTAssertEqual(DeckJournal(storage: f).lines.count, 4)
    }
    func test13_TheNumbersSurviveALostLinesFile() throws {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f); try? FileManager.default.removeItem(at: DeckJournal.seqFile(for: f)) }
        let j = DeckJournal(storage: f)
        for n in 1...5 { j.append("started", item: "i\(n)") }
        try FileManager.default.removeItem(at: f)
        let j2 = DeckJournal(storage: f)
        XCTAssertEqual(j2.seq, 5); XCTAssertEqual(j2.append("started", item: "x").seq, 6, "a lost lines file does not restart the numbering")
    }
    func test14_AFailedSaveIsCountedAndTriedAgain() throws {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("deck-dir-\(UUID().uuidString)")
        let f = dir.appendingPathComponent("j.json")                // the folder does not exist: the write fails
        let j = DeckJournal(storage: f); var told: [String] = []; j.log = { told.append($0) }
        j.append("started", item: "a")
        XCTAssertEqual(j.saveFailures, 1); XCTAssertEqual(told, ["deck journal: save failed (1 failures, 1 lines held)"])
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true); defer { try? FileManager.default.removeItem(at: dir) }
        j.append("ended", item: "a", end: "skipped")
        XCTAssertEqual(DeckJournal(storage: f).lines.count, 2, "the next event writes everything held")
    }
    func test15_TheCapSaysWhatItDropped() {
        let j = DeckJournal()
        for n in 0..<(DeckJournal.maxLines + 5) { j.append("sounding", item: "i\(n)") }
        XCTAssertLessThanOrEqual(j.lines.count, DeckJournal.maxLines)
        let marks = j.lines.filter { $0.type == "system" && $0.command == "dropped" }
        XCTAssertEqual(marks.count, 1); XCTAssertTrue(marks[0].reason.hasPrefix("dropped "), marks[0].reason)
        XCTAssertGreaterThan(j.dropped, 0)
        XCTAssertEqual(Set(j.lines.map(\.seq)).count, j.lines.count, "numbers still unique")
    }
    // Review 2026-11-19 (deck, seat 2, round 2): J8 a file that reads and cannot be decoded is moved aside and journalling goes on from the numbers file, with a `system` line
    //  J9 an unreadable launch still closes the item a kill left open, once the file reads  J10 lines at or below the acknowledged number are pruned at init
    //  J11 a start suspended across the wipe makes no sound for the next account
    func test19_ACorruptJournalIsMovedAsideAndJournallingGoesOn() throws {
        let f = tmp(); let bad = f.deletingPathExtension().appendingPathExtension("bad")
        defer { for u in [f, bad, DeckJournal.seqFile(for: f)] { try? FileManager.default.removeItem(at: u) } }
        let j = DeckJournal(storage: f); for n in 1...3 { j.append("started", item: "i\(n)") }
        try Data("not json".utf8).write(to: f)
        let j2 = DeckJournal(storage: f)
        XCTAssertTrue(j2.isLoaded); XCTAssertEqual(try Data(contentsOf: bad), Data("not json".utf8))
        let l = j2.append("started", item: "x")
        XCTAssertEqual(j2.lines.map(\.command), ["journal-corrupt", ""]); XCTAssertEqual(j2.lines.first?.seq, 4); XCTAssertEqual(l.seq, 5, "the numbers file carries on")
        XCTAssertEqual(DeckJournal(storage: f).lines.count, 2, "and it is saved again")
    }
    func test20_AnUnreadableLaunchStillClosesTheItemAKillLeftOpen() throws {
        let f = tmp(); defer { try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: f.path); try? FileManager.default.removeItem(at: f); try? FileManager.default.removeItem(at: DeckJournal.seqFile(for: f)) }
        let j = DeckJournal(storage: f); j.append("started", item: "a")
        try FileManager.default.setAttributes([.posixPermissions: 0], ofItemAtPath: f.path)
        let j2 = DeckJournal(storage: f); XCTAssertNil(j2.closeOpenAtBoot()); XCTAssertFalse(j2.isLoaded)
        try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: f.path)
        j2.append("remote", item: "", command: "play")                    // the next event reads the file
        XCTAssertEqual(j2.lines.filter { $0.type == "ended" }.map { "\($0.item):\($0.end):\($0.reason)" }, ["a:failed:interrupted by a restart"])
    }
    func test21_LinesAtOrBelowTheAcknowledgedNumberArePrunedAtInit() throws {
        let f = tmp(); defer { try? FileManager.default.removeItem(at: f); try? FileManager.default.removeItem(at: DeckJournal.seqFile(for: f)) }
        let j = DeckJournal(storage: f); for n in 1...3 { j.append("started", item: "i\(n)") }
        var raw = try JSONSerialization.jsonObject(with: Data(contentsOf: f)) as! [String: Any]; raw["acked"] = 2
        try JSONSerialization.data(withJSONObject: raw).write(to: f)
        XCTAssertEqual(DeckJournal(storage: f).lines.map(\.seq), [3])
    }
    func test22_AStartSuspendedAcrossTheWipeMakesNoSound() async {
        let (d, e) = makeDeck(); var release: CheckedContinuation<Void, Never>?
        e.holdSpeak = { await withCheckedContinuation { release = $0 } }
        let t = Task { @MainActor in await d.play(self.speech("a")) }
        await settle(); XCTAssertNotNil(release)
        d.wipe(); e.log = []
        release?.resume(); _ = await t.value
        XCTAssertNil(d.current); XCTAssertTrue(e.log.contains("stopSpoken"), "the engine is stopped again when the start resolves after the wipe")
        XCTAssertTrue(d.journal.lines.isEmpty)
    }

}
