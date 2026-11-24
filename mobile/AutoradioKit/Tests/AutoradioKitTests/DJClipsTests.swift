import XCTest
@testable import AutoradioKit

// Lane autoradio-dj-clips. Failure list (the numbers are in the test names):
// C1 a story with 0, 1 or 5 clips sends 0, 1 or at most 3; invalid clips and repeated ids are left out
// C2 one bulk clips request per stretch, cached after; a 404 or an empty list leaves the request as it was
// C3 a podcast_clip segment decodes with its clip; one with no clip is dropped by the decoder
// Playing a clip, its lead-in, Keep listening, the preflight and a skipped clip are entries on the run sheet: RunSheetStoreTests.

private func clipRow(_ id: String, route: String = "/a", start: Double = 10, end: Double = 70, show: String = "Show", ep: String = "Episode", quote: String = "A quote.") -> [String: Any] {
    ["clip_id": id, "story": ["route": route, "version": 2, "title": "Story"], "show": show, "episode_title": ep, "episode_guid": "g-\(id)",
     "feed_url": "https://example.test/feed.xml", "published": "2026-11-16T09:00:00Z", "audio_url": "https://example.test/\(id).mp3",
     "duration_seconds": 900, "start": start, "end": end, "quote": quote, "support": 0.9, "fingerprint": testFingerprintJSON()]
}
private func hex(_ n: Int) -> String { "clip-" + String(format: "%012x", n) }

final class ClipCandidatesTests: XCTestCase {
    // C9 (the story page's rows) is shared code now: clips.test.ts H1.
    func testC3_ABlockForTheClipFollowsTheLeadIn() throws {
        let seg = IncomingSegment(stretchID: "s", segmentID: "s/1", kind: "podcast_clip", title: "Show on it", text: "Here is how Show put it.", cites: [], blockCount: 1,
                                  clip: ProgrammeClipRef(clipID: hex(1), storyRef: "pub:a@v2", seconds: 60))
        let p = try XCTUnwrap(SegmentPrep.prepare(seg))
        XCTAssertEqual(p.blocks, [.speech(text: "Here is how Show put it."), .podcastClip(clipID: hex(1))])
        XCTAssertEqual(p.segment.clip?.clipID, hex(1)); XCTAssertEqual(p.segment.kind, "podcast_clip")
        var noClip = seg; noClip.clip = nil
        XCTAssertNil(SegmentPrep.prepare(noClip))
    }
    func testC3_TheDecoderReadsTheClipAndDropsALineWithNone() {
        var d = StretchDecoder()
        let ok = segmentLine(["kind": "podcast_clip", "title": "T", "text": "Lead.", "clip": ["clip_id": hex(1), "story_ref": "pub:a@v2", "seconds": 56.5]])
        let bad = segmentLine(["kind": "podcast_clip", "title": "T", "text": "Lead."])
        let evs = d.feed(ok + "\n" + bad + "\n")
        XCTAssertEqual(evs.count, 1)
        if case .segment(let s) = evs[0] { XCTAssertEqual(s.clip, ProgrammeClipRef(clipID: hex(1), storyRef: "pub:a@v2", seconds: 56.5)) } else { XCTFail() }
        XCTAssertEqual(d.droppedCount, 1)
    }
}
