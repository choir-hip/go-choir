import XCTest
@testable import AutoradioKit

// Lane autoradio-joins. The seams the six lanes left open. Failure list (numbers are in the test names):
// J1 a clip in the queue can open its episode (shared code: radio-bridge.ts `openEpisodeOf`): the entry carries the podcast lane's key (feed and guid, or the audio address when
//    there is no guid); a clip with no feed carries none, so the queue offers no dead button (`canOpenEpisode`)
// J2 a typed .pdf or .epub: the document is imported then its page opens; a failed import says why in plain words and opens nothing
// J3 Hear it on the article page: rows for the story's clips and nothing when it has none
// J4 Up next shows up to three of the DJ's coming segments after the queued items, marked as the programme's, not removable
// J5 the Reader's documents answer a question: at most 200 passages, the most recently opened documents first, each naming its book
// J6 a show page lists its clips (matched by feed), nothing when the route is missing or the show has none

private func clipRow(_ id: String, guid: String = "g", feed: String? = "https://example.test/feed.xml", route: String = "/a") -> [String: Any] {
    var r: [String: Any] = ["clip_id": id, "story": ["route": route, "version": 2, "title": "Story"], "show": "Show", "episode_title": "Episode", "episode_guid": guid,
     "published": "2026-11-16T09:00:00Z", "audio_url": "https://example.test/\(id).mp3", "duration_seconds": 900, "start": 10, "end": 70, "quote": "A quote.", "support": 0.9]
    if let feed { r["feed_url"] = feed }
    return r
}
private func hex(_ n: Int) -> String { "clip-" + String(format: "%012x", n) }

// MARK: J3 Hear it on the article page, J6 Clips on a show page

@MainActor
final class JoinHearItTests: XCTestCase {
    func testJ1_AClipEntryCarriesThePodcastLanesEpisodeKey() {
        let withGuid = PodcastClip.parse(clipRow(hex(1), guid: "g1")), noGuid = PodcastClip.parse(clipRow(hex(2), guid: "")), noFeed = PodcastClip.parse(clipRow(hex(3), feed: nil))
        XCTAssertEqual(withGuid?.clipEntry.audioSpec?.episodeId, PodcastKey.episode(feed: "https://example.test/feed.xml", id: "g1"))
        XCTAssertEqual(noGuid?.episodeEntry.audioSpec?.episodeId, PodcastKey.episode(feed: "https://example.test/feed.xml", id: PodcastKey.webURL("https://example.test/\(hex(2)).mp3")))
        XCTAssertNil(noFeed?.clipEntry.audioSpec?.episodeId)
    }
    func store(_ rows: [[String: Any]]) -> ClipStore { let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse)); return s }


    // J3's "a clip the phone does not have says so" and "Open episode names the page at the clip's start" are shared code now: clips.test.ts C5.

}

final class JoinClipRecordTests: XCTestCase {
    // J6's feed matching, asking and bad bodies are shared code now (clips.test.ts C3, C4, H2); the native store holds what the page hands it.
    func record(_ rows: [String: Any]) -> String { String(data: try! JSONSerialization.data(withJSONObject: rows), encoding: .utf8)! }

    func testJ6_AClipHandedOverByThePageCanBePlayedByIdAndIsHeldOnce() {
        let s = ClipStore(); s.replace([clipRow(hex(1))].compactMap(PodcastClip.parse))
        let two = PodcastClip.parse(record: record(clipRow(hex(2))))
        XCTAssertNotNil(two)
        s.merge([two!]); s.merge([two!])
        XCTAssertNotNil(s.clip(hex(1))); XCTAssertNotNil(s.clip(hex(2)))
        XCTAssertEqual(s.all.count, 2, "a clip handed twice is held once")
    }
    func testJ6_AClipHandedOverReplacesTheRowHeldForItsId() {
        let old = clipRow(hex(1))                                   // no fingerprint: held as the episode
        var withPrint = clipRow(hex(1)); withPrint["fingerprint"] = testFingerprintJSON()
        let s = ClipStore(); s.replace([old].compactMap(PodcastClip.parse))
        XCTAssertEqual(s.clip(hex(1))?.isRelocatable, false, "held without a usable fingerprint")
        let fresh = PodcastClip.parse(record: record(withPrint))
        XCTAssertNotNil(fresh); XCTAssertEqual(fresh?.isRelocatable, true)
        s.put(fresh!); s.put(fresh!)
        XCTAssertEqual(s.clip(hex(1))?.isRelocatable, true, "the page's record replaced the stale one")
        XCTAssertEqual(s.all.count, 1); XCTAssertEqual(s.clips(forRoute: fresh!.route).count, 1)
    }
    func testJ6_ABadRecordIsNoClip() {
        for r in ["not json", "{}", "[]", "", record(["clip_id": ""])] { XCTAssertNil(PodcastClip.parse(record: r)) }
    }
}
