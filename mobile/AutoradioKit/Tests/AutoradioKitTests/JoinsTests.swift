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

}

// MARK: J5 first answers from the Reader's books

final class JoinLibraryTests: XCTestCase {
    func doc(_ id: String, _ title: String, added: Double) -> ShelfDoc {
        ShelfDoc(id: id, title: title, author: "", kind: "epub", added: Date(timeIntervalSince1970: added), position: 0, length: 0, sections: 1, fileName: "\(id).epub")
    }
    func text(_ paragraphs: [String]) -> DocText {
        DocText(sections: [DocSection(title: "One", start: 0, count: paragraphs.count)],
                blocks: paragraphs.map { DocBlock(kind: "paragraph", level: 0, text: $0, marker: "", section: 0) })
    }
    let sentence = "The lighthouse keeper climbed the spiral stair every evening to trim the lamp before the fog rolled in from the sea."

    func testJ5_ADocumentBecomesPassagesNamedForTheBook() {
        let src = LibraryCorpusSource(docs: { [self.doc("a", "The Keeper", added: 2)] }, text: { _ in self.text([self.sentence, self.sentence + " Again."]) })
        let d = src.documents()
        XCTAssertEqual(d.count, 1); XCTAssertEqual(d[0].title, "The Keeper"); XCTAssertEqual(d[0].address, "doc:a"); XCTAssertTrue(d[0].id.hasPrefix("book:"))
        let c = SavedCorpus(heldPages: nil); c.setSource("library") { src.documents() }; c.refresh()
        let hits = c.retrieve("who climbed the lighthouse stair to trim the lamp").items
        XCTAssertEqual(hits.first?.title, "The Keeper")
        XCTAssertEqual(SavedAnswerPrompt.source(hits[0]), "From your book The Keeper")
    }
    func testJ5_ABookIsKeptWholeByChapterForRetrievalAndTheOldIndexIsBounded() {
        let many = (0..<300).map { "Paragraph number \($0) about harbour tides and the long grey winter, told at some length so that it stands alone as a passage of its own in the book. \(String(repeating: "tide ", count: 20))" }
        let docs = [doc("old", "Old Book", added: 1), doc("new", "New Book", added: 9)]
        let src = LibraryCorpusSource(docs: { docs }, text: { _ in self.text(many) })
        let out = src.documents()
        XCTAssertEqual(out.first?.title, "New Book", "the most recent document comes first")
        XCTAssertEqual(out.count, 2, "both books are kept: the budget is thousands of passages, not 200")
        XCTAssertFalse(out[0].parts.isEmpty); XCTAssertTrue(out[0].parts[0].label.hasPrefix("chapter 1"))
        let c = SavedCorpus(heldPages: nil); c.setSource("library") { out }; c.refresh()
        XCTAssertGreaterThan(c.retrieval.count, 100, "the retrieval index sees all of it")
    }
    func testJ5_AnUnreadableOrEmptyDocumentIsSkippedAndNothingThrows() {
        let src = LibraryCorpusSource(docs: { [self.doc("a", "A", added: 1), self.doc("b", "B", added: 2)] }, text: { $0 == "a" ? nil : self.text([]) })
        XCTAssertTrue(src.documents().isEmpty)
        XCTAssertTrue(LibraryCorpusSource(docs: { [] }, text: { _ in nil }).documents().isEmpty)
    }
    func testJ5_TheSameLibraryIsNotReadAgainForEverySearch() {
        var reads = 0
        let src = LibraryCorpusSource(docs: { [self.doc("a", "A", added: 1)] }, text: { _ in reads += 1; return self.text([self.sentence]) })
        _ = src.documents(); _ = src.documents(); _ = src.documents()
        XCTAssertEqual(reads, 1)
    }
}

// MARK: J3 Hear it on the article page, J6 Clips on a show page

@MainActor
final class JoinHearItTests: XCTestCase {
    func store(_ rows: [[String: Any]]) -> ClipStore { let s = ClipStore(); s.replace(rows.compactMap(PodcastClip.parse)); return s }


    // J3's "a clip the phone does not have says so" and "Open episode names the page at the clip's start" are shared code now: clips.test.ts C5.

}

final class JoinShowClipsTests: XCTestCase {
    func body(_ rows: [[String: Any]]) -> Data { try! JSONSerialization.data(withJSONObject: ["clips": rows]) }

    func testJ6_AShowListsItsOwnClipsMatchedByFeed() {
        let s = ClipStore()
        s.merge(PodcastClips.parse(body([clipRow(hex(1), feed: "https://example.test/feed.xml"), clipRow(hex(2), feed: "https://other.test/rss"), clipRow(hex(3), feed: "https://EXAMPLE.test/feed.xml/")])))
        XCTAssertEqual(s.clips(forFeed: "https://example.test/feed.xml").map(\.clipID), [hex(1), hex(3)])
        XCTAssertTrue(s.clips(forFeed: "https://none.test/x").isEmpty)
        XCTAssertNotNil(s.clip(hex(2)), "a merged clip can be played by id")
    }
    func testJ6_MergeKeepsWhatTheBulkSetHeld() {
        let s = ClipStore(); s.replace([clipRow(hex(1))].compactMap(PodcastClip.parse))
        s.merge(PodcastClips.parse(body([clipRow(hex(2))])))
        XCTAssertNotNil(s.clip(hex(1))); XCTAssertNotNil(s.clip(hex(2)))
        s.merge(PodcastClips.parse(body([clipRow(hex(2))])))
        XCTAssertEqual(s.all.count, 2, "a clip fetched twice is held once")
    }
    func testJ6_ABadOrEmptyBodyIsSilentlyNothing() {
        XCTAssertTrue(PodcastClips.parse(Data("not json".utf8)).isEmpty)
        XCTAssertTrue(PodcastClips.parse(Data("{}".utf8)).isEmpty)
        XCTAssertTrue(PodcastClips.parse(body([])).isEmpty)
    }
    // J6's rows (quote, story, length) are shared code now: clips.test.ts C3.
    func testJ6_AFeedIsAskedForOnceAWhileNotOnEveryDraw() {
        let s = ClipStore()
        XCTAssertTrue(s.shouldAsk(feed: "https://example.test/feed.xml", now: Date(timeIntervalSince1970: 0)))
        s.didAsk(feed: "https://example.test/feed.xml", now: Date(timeIntervalSince1970: 0))
        XCTAssertFalse(s.shouldAsk(feed: "https://example.test/feed.xml/", now: Date(timeIntervalSince1970: 60)))
        XCTAssertTrue(s.shouldAsk(feed: "https://example.test/feed.xml", now: Date(timeIntervalSince1970: 1000)))
    }
}
