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

// MARK: F01-F07 reading the stretch

final class StretchDecoderTests: XCTestCase {
    func segs(_ e: [StretchEvent]) -> [IncomingSegment] { e.compactMap { if case .segment(let s) = $0 { return s } else { return nil } } }

    func testF01_ChunkEndingInsideALineWaits() {
        var d = StretchDecoder()
        let line = segmentLine(["kind": "story", "title": "T", "text": "Hello there."])
        XCTAssertTrue(d.feed(String(line.prefix(10))).isEmpty, "half a line is not a line")
        XCTAssertTrue(d.feed(String(line.dropFirst(10).prefix(15))).isEmpty)
        let got = segs(d.feed(String(line.dropFirst(25)) + "\n"))
        XCTAssertEqual(got.map(\.text), ["Hello there."])
        XCTAssertTrue(d.finish().isEmpty)
    }

    func testF01_LineEndingsLFCRLFAndLoneCR() {
        var d = StretchDecoder()
        let a = segmentLine(["kind": "story", "title": "A", "text": "One."]), b = segmentLine(["kind": "story", "title": "B", "text": "Two."])
        let c = segmentLine(["kind": "story", "title": "C", "text": "Three."])
        XCTAssertEqual(segs(d.feed(a + "\r\n" + b + "\r" + c + "\n")).map(\.title), ["A", "B", "C"])
        // a CR at the end of a chunk may be the first half of a CRLF: the next chunk's LF must not make an empty line count
        var e = StretchDecoder()
        XCTAssertEqual(segs(e.feed(a + "\r")).map(\.title), ["A"])
        XCTAssertEqual(segs(e.feed("\n" + b + "\n")).map(\.title), ["B"])
        XCTAssertEqual(e.ignoredCount, 0)
    }

    func testF01_TheLastLineWithoutALineEndingIsReadAtFinish() {
        var d = StretchDecoder()
        XCTAssertTrue(d.feed(segmentLine(["kind": "story", "title": "T", "text": "No newline."])).isEmpty)
        XCTAssertEqual(segs(d.finish()).map(\.text), ["No newline."])
    }

    func testF02_NoiseIsIgnoredAndCounted() {
        var d = StretchDecoder()
        let noise = ": autoradio\n\n\nevent: handoff\ndata: {\"x\":1}\n\nnot json at all\n[1,2,3]\n\"a string\"\n{\"kind\":\"story\"\n"
        XCTAssertTrue(d.feed(noise).isEmpty)
        XCTAssertGreaterThan(d.ignoredCount, 0)
        let ok = segs(d.feed(segmentLine(["kind": "story", "title": "T", "text": "Still works."]) + "\n"))
        XCTAssertEqual(ok.map(\.text), ["Still works."], "the stream goes on after noise")
    }

    func testF03_TheFixtureStretch_PhoneDropsOnlyWhatItCannotSpeak() throws {
        let f = try LearningFixtures.json("stretch_lines.json")
        let lines = try XCTUnwrap(f["segment_lines"] as? [[String: Any]])
        XCTAssertEqual(lines.count, 32, "the fixture changed: recount")
        var d = StretchDecoder()
        var got: [IncomingSegment] = []
        for l in lines { got += segs(d.feed(segmentLine(try XCTUnwrap(l["line"] as? [String: Any])) + "\n")) }
        got += segs(d.finish())
        XCTAssertEqual(d.droppedCount, 2, "an unknown kind (weather) and an empty text; the server validates the rest")
        XCTAssertEqual(got.count, 30)
        XCTAssertFalse(got.contains { $0.kind == "weather" })
        // a padded title is trimmed; an empty one (the server drops it) is spoken here under the kind's plain label
        let titles = got.compactMap { SegmentPrep.prepare($0)?.segment.title }
        XCTAssertFalse(titles.contains { $0 != $0.trimmingCharacters(in: .whitespaces) || $0.isEmpty })
        XCTAssertTrue(titles.contains("A quiet harbour")); XCTAssertTrue(titles.contains("Story"))
        XCTAssertTrue(got.allSatisfy { !$0.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty })
        // every kept fixture line becomes a segment with one block ("The harbour reopened today." and the like)
        XCTAssertTrue(got.allSatisfy { SegmentPrep.prepare($0)?.blocks.count == 1 })
    }

    func testF04_Titles() {
        var d = StretchDecoder()
        func title(_ t: Any?, kind: String = "story") -> String {
            var o: [String: Any] = ["kind": kind, "text": "Words."]; if let t { o["title"] = t }
            return segs(d.feed(segmentLine(o) + "\n")).first.flatMap { SegmentPrep.prepare($0)?.segment.title } ?? "<none>"
        }
        XCTAssertEqual(title("a\nb"), "a b")
        XCTAssertEqual(title("a\u{07}b"), "a b")
        XCTAssertEqual(title("   Padded   title  "), "Padded title")
        XCTAssertLessThanOrEqual(title(String(repeating: "é", count: 150)).utf8.count, 200, "capped in bytes, never half a character")
        XCTAssertFalse(title(String(repeating: "é", count: 150)).contains("\u{FFFD}"))
        XCTAssertEqual(title(nil, kind: "ident"), "Station break")
        XCTAssertEqual(title("", kind: "story"), "Story")
        XCTAssertEqual(title("pub:harbour-1"), "Story", "a raw ref is never a title")
    }

    func testF05_Cites() {
        var d = StretchDecoder()
        let line = segmentLine(["kind": "story", "title": "T", "text": "Words.",
                                "cites": ["pub:a", ["ref": "pub:b", "role": "subject", "label": "The harbour story"], ["ref": "pub:b", "role": "quoted", "label": "The harbour story"], ["ref": "mail:1", "role": "mentioned"]]])
        let s = segs(d.feed(line + "\n")).first
        let prepared = s.flatMap { SegmentPrep.prepare($0) }
        XCTAssertEqual(prepared?.segment.cites.map(\.ref), ["pub:a", "pub:b", "mail:1"], "one ref once")
        XCTAssertEqual(prepared?.segment.sources, ["The harbour story"], "a chip only for a cite with a label; never a raw ref")
    }

    func testF06_IdsLocalAndRepeated() {
        var d = StretchDecoder()
        let a = segs(d.feed(segmentLine(["kind": "story", "title": "A", "text": "One."]) + "\n" + segmentLine(["kind": "story", "title": "B", "text": "Two."]) + "\n"))
        XCTAssertEqual(a.map(\.segmentID), ["local/0", "local/1"])
        XCTAssertEqual(Set(a.map(\.stretchID)), ["local"])
        // a header names the stretch; the server's ids are kept
        var e = StretchDecoder()
        _ = e.feed("event: stretch\ndata: {\"stretch_id\":\"st-9\"}\n\n")
        let b = segs(e.feed("event: segment\ndata: " + segmentLine(["kind": "story", "title": "A", "text": "One."]) + "\n\n"))
        XCTAssertEqual(b.first?.stretchID, "st-9"); XCTAssertEqual(b.first?.segmentID, "st-9/0")
        let c = segs(e.feed("event: segment\ndata: " + segmentLine(["stretch_id": "st-9", "segment_id": "st-9/5", "kind": "headlines", "text": "Words.", "block_count": 3]) + "\n\n"))
        XCTAssertEqual(c.first?.segmentID, "st-9/5"); XCTAssertEqual(c.first?.blockCount, 3)
        // appended once
        var doc = ProgrammeDocument(id: "radio:st-9", title: "Autoradio", blocks: [])
        XCTAssertTrue(doc.append(SegmentPrep.prepare(c[0])!))
        XCTAssertFalse(doc.append(SegmentPrep.prepare(c[0])!), "the same segment id twice")
        XCTAssertEqual(doc.segments.count, 1)
    }

    func testF07_DoneErrorAndAStreamWithNoDone() {
        var d = StretchDecoder()
        let ev = d.feed("event: segment\ndata: " + segmentLine(["kind": "story", "title": "A", "text": "One."]) + "\n\nevent: error\ndata: {\"spoken\":\"Trouble.\",\"reason\":\"upstream\"}\n\nevent: done\ndata: {\"stretch_id\":\"x\"}\n\n")
        XCTAssertEqual(ev.count, 3)
        if case .error(let m) = ev[1] { XCTAssertTrue(m.contains("upstream")) } else { XCTFail("error first") }
        XCTAssertEqual(ev[2], .done)
        var e = StretchDecoder()
        let got = e.feed(segmentLine(["kind": "story", "title": "A", "text": "One."]) + "\n") + e.finish()
        XCTAssertEqual(segs(got).count, 1, "no done: what arrived is kept")
        XCTAssertFalse(got.contains(.done))
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
        XCTAssertEqual(SegmentPrep.prepare(s)?.segment.blockCountMismatch, false)
        s.blockCount = 5
        let p = SegmentPrep.prepare(s)
        XCTAssertEqual(p?.segment.serverBlockCount, 5, "the phone never overrules the server")
        XCTAssertEqual(p?.segment.blockCountMismatch, true)
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
        var pv = ChoirPlayerView(); pv.signedIn = true; pv.playing = true
        let tr = BarTracker()
        let r = tr.radioInfo(player: pv, snapshot: snap, document: d, route: "", nextTitle: "The next entry", now: Date())
        XCTAssertEqual(r.title, "Title 0"); XCTAssertEqual(r.kind, "Story")
        XCTAssertEqual(r.sources, ["The harbour story"])
        XCTAssertEqual(r.position, "", "no \"N of M\": the bar shows what is on air and its progress"); XCTAssertEqual(r.count, 1); XCTAssertEqual(r.index, 0)
        XCTAssertEqual(r.next, "The next entry", "the run sheet's next entry, never the next segment of a stretch")
        XCTAssertEqual(r.sentences, ["Two."], "the passage being read is still the block")
        XCTAssertFalse(String(data: try! JSONEncoder().encode(r), encoding: .utf8)!.contains("pub:"), "no raw ref reaches the screen")
        // one article: unchanged
        let plain = ProgrammeDocument(id: "choir:x", title: "A story", blocks: [.speech(text: "One."), .speech(text: "Two.")])
        let p = tr.radioInfo(player: pv, snapshot: snap, document: plain, route: "/p/x", now: Date())
        XCTAssertEqual(p.title, "A story"); XCTAssertEqual(p.kind, ""); XCTAssertEqual(p.sources, []); XCTAssertEqual(p.count, 2); XCTAssertEqual(p.position, "")
    }
}

// MARK: F18 the tracker, pure

final class SegmentTrackerTests: XCTestCase {
    final class Sink: @unchecked Sendable {
        var events: [PlaybackEvent] = []
        var names: [String] { events.map { "\($0.kind.rawValue):\($0.segmentID.split(separator: "/").last ?? ""):\($0.lastBlock)/\($0.blockCount)" } }
    }
    func make() -> (SegmentTracker, Sink) {
        let t = SegmentTracker(); let s = Sink(); t.observe { s.events.append($0) }; return (t, s)
    }

    // 4 units: segment 0 has server blocks [0,0,1] (one long paragraph then one short), segment 1 has one unit
    func twoSegments() -> ProgrammeDocument {
        let long = String(repeating: "This sentence is exactly the kind of thing a radio says. ", count: 30)
        return segmentedDocument([long + "\n\nShort close.", "Next."])
    }

    func testF18_PlayedThroughReportsBlocksOncePerServerBlockAndFinishes() {
        let d = twoSegments()
        let n = d.segments[0].blockCount
        XCTAssertGreaterThan(n, 2)
        let (t, s) = make()
        for u in 0..<n { t.unitBegan(u, in: d); t.unitEnded(u, played: true, in: d) }
        XCTAssertEqual(s.names, ["segmentBegan:0:0/2", "blockBegan:0:0/2", "blockBegan:0:1/2", "finished:0:1/2"])
        t.unitBegan(n, in: d); t.unitEnded(n, played: true, in: d)
        XCTAssertEqual(Array(s.names.suffix(3)), ["segmentBegan:1:0/1", "blockBegan:1:0/1", "finished:1:0/1"])
    }

    func testF18_ForwardJumpIsSkippedBackwardIsRewound() {
        let d = twoSegments(); let n = d.segments[0].blockCount
        let (t, s) = make()
        t.unitBegan(1, in: d)                       // inside segment 0, still on server block 0
        t.unitBegan(n, in: d)                       // forward to segment 1
        XCTAssertEqual(s.names, ["segmentBegan:0:0/2", "blockBegan:0:0/2", "skipped:0:0/2", "segmentBegan:1:0/1", "blockBegan:1:0/1"])
        let before = s.events.count
        t.unitBegan(0, in: d)                       // backward to segment 0
        XCTAssertEqual(s.names[before], "rewound:1:0/1")
        XCTAssertEqual(s.names[before + 1], "segmentBegan:0:0/2")
    }

    func testF18_RestartingTheSameSegmentIsNotASkipAndLastBlockNeverGoesBack() {
        let d = twoSegments(); let n = d.segments[0].blockCount
        let (t, s) = make()
        t.unitBegan(n - 1, in: d)                   // the short closing block: server block 1 began
        t.unitBegan(0, in: d)                       // previous: back to the start of the same segment
        XCTAssertFalse(s.names.contains { $0.hasPrefix("skipped") || $0.hasPrefix("rewound") })
        XCTAssertEqual(s.events.filter { $0.kind == .blockBegan }.map(\.lastBlock), [1], "block 0 began after block 1: not reported again")
        t.unitEnded(n - 1, played: true, in: d)
        XCTAssertEqual(s.events.last?.kind, .finished); XCTAssertEqual(s.events.last?.lastBlock, 1)
    }

    func testF17_ABlockThatDidNotPlayIsFailedNotFinished() {
        let d = segmentedDocument(["Only."])
        let (t, s) = make()
        t.unitBegan(0, in: d); t.unitEnded(0, played: false, in: d)
        XCTAssertEqual(s.events.map(\.kind), [.segmentBegan, .blockBegan, .failed])
    }

    func testF18_PauseResumeCutAndSupersede() {
        let d = segmentedDocument(["One."])
        let (t, s) = make()
        t.paused(true); XCTAssertTrue(s.events.isEmpty, "nothing open: nothing to report")
        t.unitBegan(0, in: d)
        XCTAssertTrue(s.events.isEmpty, "begun while the listener is paused: not heard, so not announced")
        t.paused(false)
        t.paused(true); t.paused(true); t.paused(false)
        t.cut(true); t.cut(true); t.cut(false)
        XCTAssertEqual(s.events.map(\.kind), [.segmentBegan, .blockBegan, .paused, .resumed, .cut, .cutEnded], "each transition once; the beginning came when the listener played")
        t.documentReplaced()
        XCTAssertEqual(s.events.last?.kind, .superseded)
        t.documentReplaced(); XCTAssertEqual(s.events.filter { $0.kind == .superseded }.count, 1)
    }

    func testAnArticleWithNoSegmentsReportsNothing() {
        let d = ProgrammeDocument(id: "choir:x", title: "T", blocks: [.speech(text: "One."), .speech(text: "Two.")])
        let (t, s) = make()
        t.unitBegan(0, in: d); t.unitEnded(0, played: true, in: d); t.paused(true); t.cut(true); t.documentReplaced()
        XCTAssertTrue(s.events.isEmpty)
    }
}

// MARK: F14-F18 the player

final class SegmentPlayerTests: XCTestCase {
    final class Sink: @unchecked Sendable {
        private let lock = NSLock(); private var e: [PlaybackEvent] = []
        func add(_ x: PlaybackEvent) { lock.lock(); e.append(x); lock.unlock() }
        var events: [PlaybackEvent] { lock.lock(); defer { lock.unlock() }; return e }
        var kinds: [String] { events.map { "\($0.kind.rawValue):\($0.segmentID.split(separator: "/").last ?? "")" } }
    }
    func makePlayer(voice: String = "stub-voice", doc: ProgrammeDocument) -> (ProgrammePlayer, Sink) {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: voice))
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
        let s = Sink(); p.playback.observe { s.add($0) }
        return (p, s)
    }

    func testF18_PlaysThroughAndTellsTheObserver() async {
        let (p, s) = makePlayer(doc: segmentedDocument(["One.\n\nTwo.", "Three."]))
        p.play()
        let done = await poll(10) { s.kinds.contains("finished:1") }
        XCTAssertTrue(done)
        XCTAssertEqual(s.kinds, ["segmentBegan:0", "blockBegan:0", "blockBegan:0", "finished:0", "segmentBegan:1", "blockBegan:1", "finished:1"])
        XCTAssertEqual(s.events.filter { $0.kind == .blockBegan }.map(\.lastBlock), [0, 1, 0])
        XCTAssertEqual(s.events.first { $0.kind == .finished && $0.segmentID == "st1/0" }?.blockCount, 2)
    }

    func testF14_ASegmentArrivingDuringPlayDoesNotDisturbTheBlockNow() async {
        let (p, s) = makePlayer(voice: "long-voice", doc: segmentedDocument(["One.\n\nTwo."]))
        p.play()
        _ = await poll { p.renderPosition() != nil || p.blockIndex == 0 }
        try? await Task.sleep(nanoseconds: 300_000_000)
        let before = (p.blockIndex, s.events.count)
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/9", kind: "story", title: "Late", text: "Late words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(extra, toDocument: "radio:t"))
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(p.blockIndex, before.0, "no restart, no skip")
        XCTAssertEqual(s.events.count, before.1, "no event for the arrival")
        XCTAssertEqual(p.document.segments.count, 2); XCTAssertEqual(p.document.blocks.count, 3)
    }

    func testF13_ASegmentForAnotherDocumentIsRefused() {
        let (p, _) = makePlayer(doc: segmentedDocument(["One."]))
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "old", segmentID: "old/1", kind: "story", title: "Late", text: "Late.", cites: [], blockCount: nil))!
        XCTAssertFalse(p.appendSegment(extra, toDocument: "radio:old"))
        XCTAssertEqual(p.document.segments.count, 1)
    }

    func testF15_ASegmentAfterTheProgrammeRanDryResumesPlaying() async {
        let (p, s) = makePlayer(doc: segmentedDocument(["One."]))
        p.play()
        let ok1 = await poll(10) { s.kinds.contains("finished:0") }; XCTAssertTrue(ok1)
        try? await Task.sleep(nanoseconds: 200_000_000)
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/1", kind: "story", title: "Next", text: "Next words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(extra, toDocument: "radio:t"))
        let ok2 = await poll(10) { s.kinds.contains("finished:1") }; XCTAssertTrue(ok2, "it carried on by itself")
        XCTAssertEqual(p.document.segments.map(\.firstBlock), [0, 1])
    }

    func testF15_WhilePausedTheSegmentQueuesInSilence() async {
        let (p, s) = makePlayer(doc: segmentedDocument(["One."]))      // never played: userPaused
        let extra = SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/1", kind: "story", title: "Next", text: "Next words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(extra, toDocument: "radio:t"))
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertTrue(s.events.isEmpty); XCTAssertEqual(p.blockIndex, 0); XCTAssertTrue(p.userPaused)
        XCTAssertEqual(p.document.blocks.count, 2)
    }

    func testF16_SkipReportsOnlyTheSkipThatHappened() async {
        let (p, s) = makePlayer(voice: "long-voice", doc: segmentedDocument(["A1.", "B1."]))
        p.play()
        let ok3 = await poll { s.kinds.contains("blockBegan:0") }; XCTAssertTrue(ok3)
        p.next()
        let ok4 = await poll { s.kinds.contains("blockBegan:1") }; XCTAssertTrue(ok4)
        p.next()                                            // the last segment: nothing happens
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(s.kinds, ["segmentBegan:0", "blockBegan:0", "skipped:0", "segmentBegan:1", "blockBegan:1"])
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
        let (p, s) = makePlayer(voice: "no-such-voice", doc: segmentedDocument(["One."]))
        p.play()
        let ok5 = await poll(4) { s.kinds.contains("failed:0") }; XCTAssertTrue(ok5)
        XCTAssertFalse(s.kinds.contains("finished:0"))
    }

    func testF18_PauseAndYieldAreObserved() async {
        let (p, s) = makePlayer(voice: "long-voice", doc: segmentedDocument(["One."]))
        p.play()
        let ok6 = await poll(4) { s.kinds.contains("blockBegan:0") }; XCTAssertTrue(ok6)
        p.pause(); p.play()
        await p.setLevel(.pause); await p.setLevel(.play)
        XCTAssertEqual(s.events.map(\.kind).suffix(4), [.paused, .resumed, .cut, .cutEnded])
        p.pause()
    }

    func testReplacingTheDocumentMidSegmentSupersedes() async {
        let (p, s) = makePlayer(voice: "long-voice", doc: segmentedDocument(["One."]))
        p.play()
        let ok7 = await poll(4) { s.kinds.contains("blockBegan:0") }; XCTAssertTrue(ok7)
        p.setDocument(ProgrammeDocument(id: "choir:x", title: "T", blocks: [.speech(text: "Other.")]))
        XCTAssertEqual(s.events.last?.kind, .superseded)
        p.pause()
    }
}
