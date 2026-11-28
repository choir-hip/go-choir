import XCTest
@testable import AutoradioKit

// Station P1, the programme player. The failure list is in mobile/README.md ("Programme player (station P1): ways it can
// fail"); the numbers in the test names (F03 and so on) are its numbers. The fixtures are mobile/fixtures/learning/ (the
// server builder's: blocks.json, stretch_lines.json), read through #filePath so a moved directory fails the test.

enum LearningFixtures {
    static var dir: URL {
        URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures/learning")
    }
    static func json(_ name: String) throws -> [String: Any] {
        let d = try Data(contentsOf: dir.appendingPathComponent(name))
        return try XCTUnwrap(try JSONSerialization.jsonObject(with: d) as? [String: Any])
    }
}

func segmentLine(_ o: [String: Any]) -> String {
    String(decoding: (try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys])) ?? Data(), as: UTF8.self)
}

/// Build a segmented document by hand: each text is one segment.
func segmentedDocument(_ texts: [String], id: String = "radio:t", kinds: [String]? = nil) -> ProgrammeDocument {
    var d = ProgrammeDocument(id: id, title: "Autoradio", blocks: [])
    for (i, t) in texts.enumerated() {
        let s = IncomingSegment(stretchID: "st1", segmentID: "st1/\(i)", kind: kinds?[i] ?? "story", title: "Title \(i)", text: t, cites: [], blockCount: nil)
        _ = d.append(SegmentPrep.prepare(s)!)
    }
    return d
}

// MARK: F04-F06 what the phone makes of a segment it speaks
// (Reading the stretch's lines, F01 to F03 and F07, is shared code's: AutoradioLab/programme.test.ts. These pin what stays native, SegmentPrep.)

final class SegmentPrepTests: XCTestCase {
    func prepared(_ title: String?, kind: String = "story") -> String {
        SegmentPrep.prepare(IncomingSegment(stretchID: "s", segmentID: "s/0", kind: kind, title: title ?? "", text: "Words.", cites: [], blockCount: nil))?.segment.title ?? "<none>"
    }

    func testF04_Titles() {
        XCTAssertEqual(prepared("a\nb"), "a b")
        XCTAssertEqual(prepared("a\u{07}b"), "a b")
        XCTAssertEqual(prepared("   Padded   title  "), "Padded title")
        XCTAssertLessThanOrEqual(prepared(String(repeating: "é", count: 150)).utf8.count, 200, "capped in bytes, never half a character")
        XCTAssertFalse(prepared(String(repeating: "é", count: 150)).contains("\u{FFFD}"))
        XCTAssertEqual(prepared(nil, kind: "ident"), "Station break")
        XCTAssertEqual(prepared("", kind: "story"), "Story")
        XCTAssertEqual(prepared("pub:harbour-1"), "Story", "a raw ref is never a title")
    }

    func testF05_Cites() {
        let s = IncomingSegment(stretchID: "s", segmentID: "s/0", kind: "story", title: "T", text: "Words.", cites: [
            ProgrammeCite(ref: "pub:a"), ProgrammeCite(ref: "pub:b", role: "subject", label: "The harbour story"),
            ProgrammeCite(ref: "pub:b", role: "quoted", label: "The harbour story"), ProgrammeCite(ref: "mail:1", role: "mentioned")], blockCount: nil)
        let prepared = SegmentPrep.prepare(s)
        XCTAssertEqual(prepared?.segment.cites.map(\.ref), ["pub:a", "pub:b", "mail:1"], "one ref once")
        XCTAssertEqual(prepared?.segment.sources, ["The harbour story"], "a chip only for a cite with a label; never a raw ref")
    }

    /// F03, over the server's fixture corpus, read by a minimal reader of the lines (shared code decodes the stretch; this pins what native still does with a segment).
    func testF03_TheFixtureCorpusBecomesOneBlockSegments() throws {
        let f = try LearningFixtures.json("stretch_lines.json")
        let lines = try XCTUnwrap(f["segment_lines"] as? [[String: Any]]).compactMap { $0["line"] as? [String: Any] }
        XCTAssertEqual(lines.count, 32, "the fixture changed: recount")
        let kept = lines.filter { SegmentKinds.label($0["kind"] as? String ?? "") != "" && !($0["text"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines).isEmpty }
        XCTAssertEqual(kept.count, 30, "an unknown kind (weather) and an empty text are not spoken")
        let prepared = kept.enumerated().map { i, l in
            SegmentPrep.prepare(IncomingSegment(stretchID: "fx", segmentID: "fx/\(i)", kind: l["kind"] as? String ?? "", title: l["title"] as? String ?? "", text: l["text"] as? String ?? "", cites: [], blockCount: nil))
        }
        XCTAssertTrue(prepared.allSatisfy { $0 != nil })
        XCTAssertTrue(prepared.allSatisfy { $0?.blocks.count == 1 }, "every kept fixture line is one block")
        let titles = prepared.compactMap { $0?.segment.title }
        XCTAssertFalse(titles.contains { $0 != $0.trimmingCharacters(in: .whitespaces) || $0.isEmpty })
        XCTAssertTrue(titles.contains("A quiet harbour")); XCTAssertTrue(titles.contains("Story"))
        XCTAssertEqual(SplitBlocks.split("  \n\n "), [])
    }

    func testF06_AppendedOnce() {
        let c = IncomingSegment(stretchID: "st-9", segmentID: "st-9/5", kind: "headlines", title: "", text: "Words.", cites: [], blockCount: 3)
        var doc = ProgrammeDocument(id: "radio:st-9", title: "Autoradio", blocks: [])
        XCTAssertTrue(doc.append(SegmentPrep.prepare(c)!))
        XCTAssertFalse(doc.append(SegmentPrep.prepare(c)!), "the same segment id twice")
        XCTAssertEqual(doc.segments.count, 1)
    }
}

// MARK: F08-F10 blocks

final class SplitBlocksTests: XCTestCase {
    func testF08_TheServersCases() throws {
        let f = try LearningFixtures.json("blocks.json")
        let cases = try XCTUnwrap(f["cases"] as? [[String: Any]])
        XCTAssertEqual(cases.count, 9)
        for c in cases {
            let name = c["name"] as? String ?? "?"
            XCTAssertEqual(SplitBlocks.split(c["text"] as? String ?? ""), c["blocks"] as? [String], name)
            XCTAssertEqual(SplitBlocks.split(c["text"] as? String ?? "").count, c["block_count"] as? Int, name)
        }
    }
    func testF08_LoneCRIsALineEnding() {
        XCTAssertEqual(SplitBlocks.split("A first.\r\rA second."), ["A first.", "A second."])
        XCTAssertEqual(SplitBlocks.split("A first.\rstill first.\r\r\rA second."), ["A first.\nstill first.", "A second."])
    }

    func testF09_TheServersCountIsTheCountReported() {
        let two = "A first.\n\nA second."
        var s = IncomingSegment(stretchID: "s", segmentID: "s/0", kind: "story", title: "T", text: two, cites: [], blockCount: nil)
        XCTAssertEqual(SegmentPrep.prepare(s)?.segment.serverBlockCount, 2, "no count sent: computed, for fixtures")
        s.blockCount = 2
        XCTAssertEqual(SegmentPrep.prepare(s)?.segment.serverBlockCount, 2, "the server's count equal to the phone's split")
        XCTAssertEqual(SegmentPrep.prepare(s)?.segment.unitServerBlocks, [0, 1])
        s.blockCount = 5
        let p = SegmentPrep.prepare(s)
        XCTAssertEqual(p?.segment.serverBlockCount, 5, "the phone never overrules the server")
        XCTAssertEqual(p?.segment.unitServerBlocks, [0, 1])
        s.blockCount = 1
        let q = SegmentPrep.prepare(s)
        XCTAssertEqual(q?.segment.unitServerBlocks, [0, 0], "last_block is clamped inside the server's count")
        s.blockCount = 0
        XCTAssertEqual(SegmentPrep.prepare(s)?.segment.serverBlockCount, 2, "zero is not a count")
        s.blockCount = -3
        XCTAssertEqual(SegmentPrep.prepare(s)?.segment.serverBlockCount, 2)
    }

    func testF10_ALongParagraphIsSeveralUnitsOfOneServerBlock() {
        let sentence = "This sentence is exactly the kind of thing a radio says. "
        let long = String(repeating: sentence, count: 30)           // about 1700 characters in one paragraph
        let s = IncomingSegment(stretchID: "s", segmentID: "s/0", kind: "story", title: "T", text: "Short opening.\n\n" + long + "\n\nShort close.", cites: [], blockCount: 3)
        let p = SegmentPrep.prepare(s)
        XCTAssertNotNil(p)
        let units = p?.segment.unitServerBlocks ?? []
        XCTAssertGreaterThan(units.count, 3, "the long paragraph is subdivided")
        XCTAssertEqual(units.first, 0); XCTAssertEqual(units.last, 2)
        XCTAssertEqual(units, units.sorted(), "unit to server block never goes backward")
        XCTAssertEqual(Set(units), [0, 1, 2])
        XCTAssertEqual(p?.segment.blockCount, units.count)
        XCTAssertEqual(p?.blocks.count, units.count)
        XCTAssertEqual(p?.segment.serverBlockCount, 3)
    }
    func testNoSpeakableBlocksIsNoSegment() {
        let s = IncomingSegment(stretchID: "s", segmentID: "s/0", kind: "story", title: "T", text: " \n\n ", cites: [], blockCount: 1)
        XCTAssertNil(SegmentPrep.prepare(s))
    }
}

// MARK: F11-F13 the document

final class SegmentedDocumentTests: XCTestCase {
    func testF11_AnOldDocumentDecodesWithNoSegments() throws {
        let old = #"{"id":"choir:x","title":"T","blocks":[{"speech":{"text":"One."}}]}"#
        let d = try JSONDecoder().decode(ProgrammeDocument.self, from: Data(old.utf8))
        XCTAssertEqual(d.segments, [])
        XCTAssertEqual(d.blocks.count, 1)
        let round = try JSONDecoder().decode(ProgrammeDocument.self, from: JSONEncoder().encode(segmentedDocument(["One.\n\nTwo.", "Three."])))
        XCTAssertEqual(round, segmentedDocument(["One.\n\nTwo.", "Three."]))
    }

    /// A document a build wrote with the keys this build no longer has (`blockCountMismatch`, `clip` on a segment) still decodes: extra keys are ignored.
    func testADocumentWithTheOldSegmentKeysStillDecodes() throws {
        let old = #"{"id":"radio:x","title":"T","blocks":[{"speech":{"text":"One."}}],"segments":[{"id":"s/0","stretchID":"s","kind":"podcast_clip","title":"T","cites":[],"firstBlock":0,"blockCount":1,"serverBlockCount":1,"blockCountMismatch":true,"unitServerBlocks":[0],"clip":{"clipID":"clip-1","storyRef":"","seconds":4}}]}"#
        let d = try JSONDecoder().decode(ProgrammeDocument.self, from: Data(old.utf8))
        XCTAssertEqual(d.segments.count, 1); XCTAssertEqual(d.segments[0].serverBlockCount, 1); XCTAssertEqual(d.segments[0].unitServerBlocks, [0])
    }

    func testAppendNumbersTheBlocks() {
        let d = segmentedDocument(["One.\n\nTwo.", "Three.", "Four.\n\nFive.\n\nSix."])
        XCTAssertEqual(d.segments.map(\.firstBlock), [0, 2, 3])
        XCTAssertEqual(d.segments.map(\.blockCount), [2, 1, 3])
        XCTAssertEqual(d.blocks.count, 6)
        XCTAssertEqual((0..<6).map { d.segmentIndex(containingBlock: $0) }, [0, 0, 1, 2, 2, 2])
        XCTAssertNil(d.segmentIndex(containingBlock: 6)); XCTAssertNil(d.segmentIndex(containingBlock: -1))
    }



    func testNowPlayingNamesTheSegment() {
        var d = segmentedDocument(["One.\n\nTwo.", "Three."], kinds: ["story", "headlines"])
        d.segments[1].cites = [ProgrammeCite(ref: "pub:x", role: "subject", label: "The port story")]
        let a = ProgrammeNowPlaying.describe(document: d, blockIndex: 1)
        XCTAssertEqual(a.title, "Title 0"); XCTAssertEqual(a.artist, "Autoradio · Story")
        let b = ProgrammeNowPlaying.describe(document: d, blockIndex: 2)
        XCTAssertEqual(b.title, "Title 1"); XCTAssertEqual(b.artist, "Autoradio · Headlines"); XCTAssertEqual(b.sources, ["The port story"])
        let plain = ProgrammeDocument(id: "choir:x", title: "A story", blocks: [.speech(text: "Hello there.")])
        let c = ProgrammeNowPlaying.describe(document: plain, blockIndex: 0)
        XCTAssertEqual(c.title, "speech: Hello there.", "the single-article path is exactly what it was")
        XCTAssertEqual(c.artist, "A story")
    }

    func testF19_RadioInfoNamesTheEntryOnAirHasNoNOfMAndNeverRawRefs() {
        var d = segmentedDocument(["One.\n\nTwo.", "Three."], kinds: ["story", "headlines"])
        d.segments[0].cites = [ProgrammeCite(ref: "pub:harbour-1", role: "subject", label: "The harbour story"), ProgrammeCite(ref: "pub:no-label", role: "quoted", label: "")]
        var snap = LabSnapshot(); snap.blockIndex = 1; snap.blockCount = 3; snap.running = true
        var pv = ChoirPlayerView(); pv.playing = true
        let tr = BarTracker()
        let r = tr.radioInfo(player: pv, snapshot: snap, document: d, now: Date())
        XCTAssertEqual(r.title, "Title 0")
        XCTAssertFalse(String(data: try! JSONEncoder().encode(r), encoding: .utf8)!.contains("pub:"), "no raw ref reaches the screen")
        // one article: unchanged
        let plain = ProgrammeDocument(id: "choir:x", title: "A story", blocks: [.speech(text: "One."), .speech(text: "Two.")])
        let p = tr.radioInfo(player: pv, snapshot: snap, document: plain, now: Date())
        XCTAssertEqual(p.title, "A story")
    }
}

// MARK: F14-F18 the player

/// What the player itself publishes, as its readers see it: the blocks that began on air (the event log's `programmeAudioStart`, which
/// the turn timeline reads), the dry ends (`onRanDry`, which the radio stage reads), the pause and level callbacks (`onPaused`,
/// `onLevel`, which the remote audio and the bar read), and the player's own state.
final class PlayerWatch: @unchecked Sendable {
    private let lock = NSLock()
    private var dry: [(String, Bool)] = [], paused: [Bool] = [], levels: [PlayLevel] = []
    let log: EventLog
    init(_ log: EventLog, _ p: ProgrammePlayer) {
        self.log = log
        p.onRanDry = { [weak self] id, played in guard let self else { return }; self.lock.lock(); defer { self.lock.unlock() }; self.dry.append((id, played)) }
        p.onPaused = { [weak self] on in guard let self else { return }; self.lock.lock(); defer { self.lock.unlock() }; self.paused.append(on) }
        p.onLevel = { [weak self] l in guard let self else { return }; self.lock.lock(); defer { self.lock.unlock() }; self.levels.append(l) }
    }
    /// The blocks that began to play, in order.
    var air: [Int] { log.events.compactMap { if case .programmeAudioStart(let b) = $0.event { return b } else { return nil } } }
    /// Voice failures the log recorded.
    var voiceErrors: [String] { log.events.compactMap { if case .error(let stage, let m) = $0.event, stage == "voice" { return m } else { return nil } } }
    /// Each time the loop ran dry: the document and whether any block of the run made sound.
    var ranDry: [(id: String, played: Bool)] { lock.lock(); defer { lock.unlock() }; return dry.map { (id: $0.0, played: $0.1) } }
    var pauses: [Bool] { lock.lock(); defer { lock.unlock() }; return paused }
    var levelChanges: [PlayLevel] { lock.lock(); defer { lock.unlock() }; return levels }
}

final class SegmentPlayerTests: XCTestCase {
    func makePlayer(voice: String = "stub-voice", doc: ProgrammeDocument) -> (ProgrammePlayer, PlayerWatch) {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: voice))
        let log = EventLog(clock: LabClock())
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: log, configuration: cfg, document: doc)
        return (p, PlayerWatch(log, p))
    }

    func testF18_PlaysThroughAndTellsTheObserver() async {
        let (p, w) = makePlayer(doc: segmentedDocument(["One.\n\nTwo.", "Three."]))
        p.play()
        let done = await poll(10) { !w.ranDry.isEmpty }
        XCTAssertTrue(done)
        XCTAssertEqual(w.air, [0, 1, 2], "each playback block began once, in order, across both segments")
        XCTAssertEqual(w.ranDry.map(\.id), ["radio:t"]); XCTAssertEqual(w.ranDry.map(\.played), [true], "it played to the end, once")
        let d = p.document
        XCTAssertEqual(w.air.map { b in d.segments[d.segmentIndex(containingBlock: b)!].unitServerBlocks[b - d.segments[d.segmentIndex(containingBlock: b)!].firstBlock] }, [0, 1, 0], "the server block each began")
        XCTAssertEqual(d.segments[0].serverBlockCount, 2)
    }

    func testF14_ASegmentArrivingDuringPlayDoesNotDisturbTheBlockNow() async {
        let (p, w) = makePlayer(voice: "long-voice", doc: segmentedDocument(["One.\n\nTwo."]))
        p.play()
        let onAir = await poll(4) { w.air == [0] }; XCTAssertTrue(onAir, "block 0 is on air before the arrival")
        try? await Task.sleep(nanoseconds: 300_000_000)
        let before = (p.blockIndex, w.log.count, w.air)
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/9", kind: "story", title: "Late", text: "Late words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(extra, toDocument: "radio:t"))
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(p.blockIndex, before.0, "no restart, no skip")
        XCTAssertEqual(w.log.count, before.1, "nothing new on air or in the log for the arrival")
        XCTAssertEqual(w.air, before.2)
        XCTAssertEqual(p.document.segments.count, 2); XCTAssertEqual(p.document.blocks.count, 3)
    }

    func testF13_ASegmentForAnotherDocumentIsRefused() {
        let (p, _) = makePlayer(doc: segmentedDocument(["One."]))
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "old", segmentID: "old/1", kind: "story", title: "Late", text: "Late.", cites: [], blockCount: nil))!
        XCTAssertFalse(p.appendSegment(extra, toDocument: "radio:old"))
        XCTAssertEqual(p.document.segments.count, 1)
    }

    func testF15_ASegmentAfterTheProgrammeRanDryResumesPlaying() async {
        let (p, w) = makePlayer(doc: segmentedDocument(["One."]))
        p.play()
        let ok1 = await poll(10) { w.ranDry.count == 1 }; XCTAssertTrue(ok1)
        try? await Task.sleep(nanoseconds: 200_000_000)
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/1", kind: "story", title: "Next", text: "Next words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(extra, toDocument: "radio:t"))
        let ok2 = await poll(10) { w.ranDry.count == 2 }; XCTAssertTrue(ok2, "it carried on by itself")
        XCTAssertEqual(w.air, [0, 1])
        XCTAssertEqual(p.document.segments.map(\.firstBlock), [0, 1])
    }

    func testF15_WhilePausedTheSegmentQueuesInSilence() async {
        let (p, w) = makePlayer(doc: segmentedDocument(["One."]))      // never played: userPaused
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/1", kind: "story", title: "Next", text: "Next words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(extra, toDocument: "radio:t"))
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertTrue(w.air.isEmpty); XCTAssertTrue(w.ranDry.isEmpty); XCTAssertEqual(p.blockIndex, 0); XCTAssertTrue(p.userPaused)
        XCTAssertEqual(p.document.blocks.count, 2)
    }

    func testF16_SkipReportsOnlyTheSkipThatHappened() async {
        let (p, w) = makePlayer(voice: "long-voice", doc: segmentedDocument(["A1.", "B1."]))
        p.play()
        let ok3 = await poll { w.air == [0] }; XCTAssertTrue(ok3)
        p.next()
        let ok4 = await poll(1.5) { w.air == [0, 1] }; XCTAssertTrue(ok4, "the skip began segment 1 at once, well inside the 3 s block it left")
        p.next()                                            // the last segment: nothing happens
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(w.air, [0, 1], "the skip began the next segment once; the skip at the end began nothing")
        XCTAssertEqual(p.blockIndex, 1)
        p.pause()
    }

    func testF16_SkipAndPreviousBySegment() async {
        let (p, _) = makePlayer(voice: "long-voice", doc: segmentedDocument(["A1.\n\nA2.\n\nA3.", "B1.", "C1."]))   // blocks 0,1,2 | 3 | 4
        p.play()
        XCTAssertEqual(p.currentSegmentIndex, 0)
        p.next(); XCTAssertEqual(p.blockIndex, 3); XCTAssertEqual(p.currentSegmentIndex, 1)
        p.next(); XCTAssertEqual(p.blockIndex, 4)
        p.next(); XCTAssertEqual(p.blockIndex, 4, "on the last segment skip does nothing: no wrap")
        p.previous(); XCTAssertEqual(p.blockIndex, 3, "at the start of a segment: the one before")
        p.start(at: 2)                                                  // inside segment 0
        p.previous(); XCTAssertEqual(p.blockIndex, 0, "inside a segment: its start")
        p.previous(); XCTAssertEqual(p.blockIndex, 0, "nothing before the first segment")
        p.jumpToSegment(2); XCTAssertEqual(p.blockIndex, 4)
        p.jumpToSegment(7); XCTAssertEqual(p.blockIndex, 4, "outside the list: nothing")
        p.jumpToSegment(-1); XCTAssertEqual(p.blockIndex, 4)
        p.pause()
    }

    func testF17_NoVoiceIsNotAFinish() async {
        let (p, w) = makePlayer(voice: "no-such-voice", doc: segmentedDocument(["One."]))
        p.play()
        let ok5 = await poll(4) { !w.ranDry.isEmpty }; XCTAssertTrue(ok5)
        XCTAssertEqual(w.ranDry.map(\.played), [false], "the run ended dry with no block made sound: not a finish")
        XCTAssertTrue(w.air.isEmpty); XCTAssertFalse(w.voiceErrors.isEmpty, "the failure is in the log")
    }

    func testF18_PauseAndYieldAreObserved() async {
        let (p, w) = makePlayer(voice: "long-voice", doc: segmentedDocument(["One."]))
        p.play()
        let ok6 = await poll(4) { w.air == [0] }; XCTAssertTrue(ok6)
        p.pause(); XCTAssertTrue(p.userPaused); p.play(); XCTAssertFalse(p.userPaused)
        await p.setLevel(.pause); XCTAssertEqual(p.level, .pause); await p.setLevel(.play); XCTAssertEqual(p.level, .play)
        XCTAssertEqual(w.pauses, [false, true, false], "play, pause, play: each told once, nothing doubled")
        XCTAssertEqual(w.levelChanges, [.pause, .play], "the yield to a turn and its return, each once")
        p.pause()
    }

    /// What the original asserted (the tracker's last event was `superseded` at once): the open segment is let go of when the document is
    /// replaced. On the player: the index returns to 0 and the document is the new one; the loop already running is not stopped, it ends
    /// dry on the replacement (the block it was on finishing first), and the replacement's own block is never begun.
    func testReplacingTheDocumentMidSegmentSupersedes() async {
        let (p, w) = makePlayer(voice: "long-voice", doc: segmentedDocument(["A1.", "B1.", "C1."]))
        p.play()
        let ok7 = await poll(4) { w.air == [0] }; XCTAssertTrue(ok7)
        p.next()
        let ok8 = await poll(4) { w.air == [0, 1] }; XCTAssertTrue(ok8)
        XCTAssertEqual(p.blockIndex, 1, "playing the second segment")
        p.setDocument(ProgrammeDocument(id: "choir:x", title: "T", blocks: [.speech(text: "Other.")]))
        XCTAssertEqual(p.document.id, "choir:x"); XCTAssertEqual(p.blockIndex, 0, "the index of the old document does not carry over")
        let dry = await poll(8) { !w.ranDry.isEmpty }; XCTAssertTrue(dry, "the running loop ends on the replacement")
        XCTAssertEqual(w.ranDry.map(\.id), ["choir:x"])
        XCTAssertEqual(w.air, [0, 1], "nothing of the replacement began by itself")
        XCTAssertEqual(p.document.id, "choir:x"); XCTAssertEqual(p.blockIndex, 0)
        p.pause()
    }
}
