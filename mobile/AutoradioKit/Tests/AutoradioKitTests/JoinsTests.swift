import XCTest
@testable import AutoradioKit

// Lane autoradio-joins. The seams the six lanes left open. Failure list (numbers are in the test names):
// J1 a clip in the queue can open its episode: the page opens at the clip's start; an episode with no guid is found by its audio address;
//    audio the phone cannot place (no feed known) offers nothing (no dead button); the queue says so (`canOpenEpisode`)
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

@MainActor
final class JoinOpenEpisodeTests: XCTestCase {
    func store(_ rows: [[String: Any]]) -> ClipStore { let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse)); return s }
    func audio(_ n: Int, ep: String?, start: Double? = 10, end: Double? = 70) -> RunSheetAudio {
        RunSheetAudio(url: URL(string: "https://example.test/\(hex(n)).mp3")!, title: "Episode", show: "Show", start: start, end: end, episodeId: ep)
    }

    func testJ1_AClipOpensItsEpisodeAtItsStart() {
        let s = store([clipRow(hex(1), guid: "g1")])
        let t = PodcastBridge.target(of: audio(1, ep: "g1"), store: s)
        XCTAssertEqual(t?.episodeId, "g1"); XCTAssertEqual(t?.feedURL.absoluteString, "https://example.test/feed.xml"); XCTAssertEqual(t?.at, 10)
        XCTAssertEqual(t?.enclosureURL.lastPathComponent, hex(1) + ".mp3")
    }
    func testJ1_AClipWithNoGuidIsFoundByItsAudioAndKeysOnTheAudioAddress() {
        let s = store([clipRow(hex(2), guid: "")])
        let t = PodcastBridge.target(of: audio(2, ep: nil), store: s)
        XCTAssertEqual(t?.episodeId, nil); XCTAssertEqual(t?.feedURL.absoluteString, "https://example.test/feed.xml")
    }
    func testJ1_APodcastLaneKeyNeedsNoStore() {
        let key = PodcastKey.episode(feed: "https://f.test/rss", id: "guid-9")
        let t = PodcastBridge.target(of: audio(3, ep: key, start: nil, end: nil), store: ClipStore())
        XCTAssertEqual(t?.episodeId, "guid-9"); XCTAssertEqual(t?.feedURL.absoluteString, "https://f.test/rss"); XCTAssertEqual(t?.at, 0)
    }
    func testJ1_AudioThePhoneCannotPlaceOffersNothing() {
        XCTAssertNil(PodcastBridge.target(of: audio(4, ep: "zz"), store: ClipStore()))
        XCTAssertNil(PodcastBridge.target(of: nil, store: ClipStore()))
        let noFeed = store([clipRow(hex(5), feed: nil)])
        XCTAssertNil(PodcastBridge.target(of: audio(5, ep: "g"), store: noFeed))
    }
    func testJ1_OpenGoesToTheHookAndSaysWhetherItDid() {
        let s = store([clipRow(hex(1), guid: "g1")])
        var got: (String?, URL?, URL, Double)?
        let saved = PodcastBridge.open
        PodcastBridge.open = { got = ($0, $1, $2, $3) }
        defer { PodcastBridge.open = saved }
        XCTAssertTrue(PodcastBridge.openEpisode(of: audio(1, ep: "g1"), store: s))
        XCTAssertEqual(got?.0, "g1"); XCTAssertEqual(got?.3, 10)
        got = nil
        XCTAssertFalse(PodcastBridge.openEpisode(of: audio(9, ep: "nope"), store: ClipStore()))
        XCTAssertNil(got)
        PodcastBridge.open = nil
        XCTAssertFalse(PodcastBridge.openEpisode(of: audio(1, ep: "g1"), store: s), "with no hook there is no episode page to open")
    }
    func testJ1_TheQueueSaysWhenItsCurrentClipCanOpenItsEpisode() async {
        let s = store([clipRow(hex(1), guid: "g1")])
        let q = RunSheetPlayer(store: RunSheetStore()); q.engine = FakeEngine()
        q.playNow(.audio(audio(1, ep: "g1"))); await q.settle()
        XCTAssertTrue(q.info(clips: s).canOpenEpisode)
        XCTAssertFalse(q.info(clips: ClipStore()).canOpenEpisode)
    }
}

// MARK: J3 Hear it on the article page, J6 Clips on a show page

@MainActor
final class JoinHearItTests: XCTestCase {
    func store(_ rows: [[String: Any]]) -> ClipStore { let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse)); return s }

    // J3's rows (a story's own clips, a version of it, no other page) are shared code now: clips.test.ts H1.
    func testJ3_QueueAddsTheClip() async {
        let s = store([clipRow(hex(1))]); let q = RunSheetPlayer(store: RunSheetStore()); q.engine = FakeEngine()
        HearIt.enqueue(hex(1), store: s, player: q)
        XCTAssertEqual(q.store.ahead(limit: 10).count, 1)
    }
    // J3's "a clip the phone does not have says so" and "Open episode names the page at the clip's start" are shared code now: clips.test.ts C5.
    func testJ3_PlayOfAMismatchedFilePlaysTheEpisodeFromItsStartNotTheClip() async {
        let s = store([clipRow(hex(1))]); s.markMismatch(hex(1)); let q = RunSheetPlayer(store: RunSheetStore()); q.engine = FakeEngine()
        await HearIt.play(hex(1), store: s, player: q); await q.settle()
        guard let a = q.store.onAir()?.audioSpec else { return XCTFail("nothing playing") }
        XCTAssertNil(a.start); XCTAssertNil(a.end); XCTAssertEqual(q.store.onAir()?.kind, .episode)
    }
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
    func testJ6_ABadRecordIsNoClip() {
        for r in ["not json", "{}", "[]", "", record(["clip_id": ""])] { XCTAssertNil(PodcastClip.parse(record: r)) }
    }
}
