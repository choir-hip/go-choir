import XCTest
@testable import AutoradioKit

// Provenance: the stories an entry was built on survive every hop on the phone (owner, 2026-11-17: "We need to keep the provenance").
// The hops: the programme stream decoder, the run sheet entry, its op in the log, the store's file, a relaunch, and the heard record.
@MainActor
final class ProvenanceTests: XCTestCase {
    func sse(_ segs: [[String: Any]]) -> String {
        segs.map { "event: segment\ndata: " + String(data: try! JSONSerialization.data(withJSONObject: $0), encoding: .utf8)! + "\n\n" }.joined()
    }
    func cite(_ route: String, _ role: String, _ label: String) -> [String: Any] { ["ref": "pub:\(route)@v2", "role": role, "label": label] }

    func testStoryRefsSurviveEveryHopAndARelaunchAndTheHeardRecord() throws {
        let clips = ClipStore()
        clips.replace([PodcastClip.parse(["clip_id": "clip-000000000001", "story": ["route": "tide", "version": 3, "title": "Tide tables"], "show": "The Hour", "episode_title": "Ep 1",
                                          "episode_guid": "g1", "feed_url": "https://example.test/f.xml", "published": "2026-11-16T09:00:00Z", "audio_url": "https://example.test/1.mp3",
                                          "duration_seconds": 900, "start": 10, "end": 70, "quote": "q", "support": 0.9, "fingerprint": testFingerprintJSON()])!])
        let segs: [[String: Any]] = [
            ["segment_id": "s.s1", "kind": "story", "title": "Barrier", "text": "The barrier held.", "cites": [cite("harbour-barrier", "subject", "The harbour barrier")]],
            ["segment_id": "s.s2", "kind": "update", "title": "Update", "text": "More on the barrier.", "cites": [cite("harbour-barrier", "subject", "The harbour barrier")]],
            ["segment_id": "s.s3", "kind": "headlines", "title": "Headlines", "text": "Three things.", "cites": [cite("a", "subject", "Story A"), cite("b", "quoted", "Story B"), cite("c", "mentioned", "Story C")]],
            ["segment_id": "s.s4", "kind": "transition", "title": "", "text": "And now.", "between": ["after_segment": "s.s1", "before_segment": "s.s3"]],
            ["segment_id": "s.s5", "kind": "podcast_clip", "title": "Clip", "text": "A voice from the show.", "clip": ["clip_id": "clip-000000000001", "story_ref": "pub:tide@v3", "seconds": 60]],
            ["segment_id": "s.s6", "kind": "ident", "title": "", "text": "This is Autoradio."],
        ]
        var dec = StretchDecoder()
        var incoming: [IncomingSegment] = []
        for e in dec.feed(sse(segs)) { if case .segment(let s) = e { incoming.append(s) } }
        XCTAssertEqual(incoming.count, 6, "the decoder keeps every segment")
        XCTAssertEqual(incoming[2].cites.map(\.ref), ["pub:a@v2", "pub:b@v2", "pub:c@v2"], "hop 1: the decoder keeps every cite ref, role and label")
        XCTAssertEqual(incoming[4].clip?.storyRef, "pub:tide@v3", "the decoder keeps a clip's story_ref")

        let f = FileManager.default.temporaryDirectory.appendingPathComponent("prov-\(UUID().uuidString).json"); defer { try? FileManager.default.removeItem(at: f) }
        let s1 = RunSheetStore(storage: f)
        var routes: [String: [String]] = [:]
        for seg in incoming {
            let entry = try XCTUnwrap(ProgrammeRouteFeed.entry(from: seg, clips: clips), seg.segmentID)          // hop 2: the run sheet entry
            let op = ProgrammeRouteFeed.placeOp(entry, cause: "extend")
            let enc = try JSONEncoder().encode(op); let back = try JSONDecoder().decode(RunSheetOp.self, from: enc)    // hop 3: the op in the log (its wire form)
            XCTAssertEqual(back.entry?.cites, entry.cites, "hop 3: \(seg.kind) cites survive the op's wire form")
            XCTAssertTrue(s1.applyFromDesk(op).applied)
            routes[entry.id] = RadioPage.builtOn(entry).map(\.address)
        }
        XCTAssertEqual(routes["d-s.s1"], ["pub:harbour-barrier"]); XCTAssertEqual(routes["d-s.s3"], ["pub:a", "pub:b", "pub:c"])
        XCTAssertEqual(routes["d-s.s5"], ["pub:tide"], "a podcast clip carries the story it was cut for")
        XCTAssertEqual(routes["d-s.s4"], [], "a transition names neighbours (between), not stories"); XCTAssertEqual(routes["d-s.s6"], [], "an opening has no story")
        XCTAssertEqual(s1.entry("d-s.s4")?.between, RunSheetBetween(after: "d-s.s1", before: "d-s.s3"))

        // hops 4 and 5: the store's file and a relaunch
        s1.save()
        let s2 = RunSheetStore(storage: f); s2.restore()
        XCTAssertEqual(s2.entry("d-s.s3")?.cites?.map(\.ref), ["pub:a@v2", "pub:b@v2", "pub:c@v2"]); XCTAssertEqual(s2.entry("d-s.s3")?.cites?.map(\.label), ["Story A", "Story B", "Story C"])
        XCTAssertEqual(s2.entry("d-s.s5")?.cites?.first?.ref, "pub:tide@v3"); XCTAssertEqual(s2.entry("d-s.s5")?.cites?.first?.label, "Tide tables")

        // hop 6: the heard record (the entry stays on the sheet with its cites) and another relaunch
        s2.airedOnAir("d-s.s1"); s2.airedEnded("d-s.s1", as: .heard, outcome: .finished)
        s2.save()
        let s3 = RunSheetStore(storage: f); s3.restore()
        XCTAssertEqual(s3.entry("d-s.s1")?.state, .heard)
        XCTAssertEqual(s3.entry("d-s.s1")?.cites?.first?.ref, "pub:harbour-barrier@v2")
        XCTAssertEqual(RadioPage.builtOn(s3.sheet.heard().first!).map(\.title), ["The harbour barrier"], "the page reads the provenance back from the heard record")
        // the page: tappable rows for every cite, whatever its role, and the title opens the story
        var onAir = s3.entry("d-s.s3")!; onAir.state = .onAir
        let rows = RadioPage.builtOn(onAir)
        XCTAssertEqual(rows.map(\.address), ["pub:a", "pub:b", "pub:c"]); XCTAssertEqual(rows.map(\.role), ["", "quoted", "mentioned"]); XCTAssertEqual(rows.map(\.title), ["Story A", "Story B", "Story C"])
        var one = s3.entry("d-s.s1")!; one.state = .onAir
        XCTAssertEqual(RadioPage.openAddress(one), "pub:harbour-barrier", "a desk story's title opens its story")
        // and a replay from Heard keeps it
        let p = RunSheetPlayer(store: s3); p.engine = FakeEngine()
        p.perform(.replay(id: "d-s.s1"))
        XCTAssertEqual(s3.onAir()?.cites?.first?.ref, "pub:harbour-barrier@v2")
    }
}
