import XCTest
@testable import AutoradioKit

/// The failure list is at the head of PodcastFeed.swift and PodcastService.swift. Every feed here is synthetic.
final class PodcastTests: XCTestCase {
    static let feed = #"""
    <?xml version="1.0" encoding="UTF-8"?>
    <rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:media="http://search.yahoo.com/mrss/">
    <channel>
      <title>Synthetic Hour</title>
      <itunes:author>Test Author</itunes:author>
      <description>A show for tests.</description>
      <itunes:image href="http://img.example/show.jpg"/>
      <item>
        <title>Episode &amp; One</title>
        <guid isPermaLink="false">guid-1</guid>
        <pubDate>Mon, 10 Nov 2026 09:30:00 +0000</pubDate>
        <itunes:duration>01:02:03</itunes:duration>
        <enclosure url="https://cdn.example/one.mp3" length="123" type="audio/mpeg"/>
        <description><![CDATA[<p>First <b>para</b> &amp; more.</p><p>Second para.</p><ul><li>Item A</li></ul>]]></description>
        <itunes:image href="https://img.example/ep1.jpg"/>
      </item>
      <item>
        <title>Two</title>
        <guid>guid-2</guid>
        <pubDate>10 Nov 2026 08:00 GMT</pubDate>
        <itunes:duration>45:10</itunes:duration>
        <enclosure url="https://cdn.example/two.m4a"/>
        <description>Plain notes with &lt;i&gt;escaped&lt;/i&gt; html &#38; entity.</description>
      </item>
      <item>
        <title>Three</title>
        <pubDate>2026-11-01T12:00:00Z</pubDate>
        <itunes:duration>2710.5</itunes:duration>
        <enclosure url="https://cdn.example/a.jpg" type="image/jpeg"/>
        <enclosure url="https://cdn.example/three.mp3" type="audio/mpeg"/>
      </item>
      <item><title>No audio</title><guid>guid-4</guid><itunes:duration>junk</itunes:duration><description>Just text.</description></item>
      <item><title>Bad scheme</title><guid>guid-5</guid><enclosure url="file:///etc/passwd" type="audio/mpeg"/><itunes:duration>0</itunes:duration></item>
      <item><title>Dup</title><guid>guid-1</guid><enclosure url="https://cdn.example/dup.mp3" type="audio/mpeg"/></item>
    </channel></rss>
    """#

    func parsed() throws -> PodcastFeed { try RSSFeedParser.parse(Data(Self.feed.utf8), feedURL: "https://feeds.example/show.xml", fetchedAt: 1) }

    func testShowAndEpisodes_1_2_3_4_5() throws {
        let f = try parsed()
        XCTAssertEqual(f.show.title, "Synthetic Hour"); XCTAssertEqual(f.show.author, "Test Author")
        XCTAssertEqual(f.show.artworkURL, "https://img.example/show.jpg", "http artwork is upgraded")
        XCTAssertEqual(f.show.summary, "A show for tests.")
        XCTAssertEqual(f.episodes.map(\.id), ["guid-1", "guid-2", "https://cdn.example/three.mp3", "guid-4", "guid-5"], "no guid: the enclosure; a repeated guid once")
        let e1 = f.episodes[0]
        XCTAssertEqual(e1.title, "Episode & One")
        XCTAssertEqual(e1.duration, 3723); XCTAssertEqual(e1.enclosureURL, "https://cdn.example/one.mp3"); XCTAssertEqual(e1.enclosureType, "audio/mpeg")
        XCTAssertEqual(e1.published, 1_794_303_000)
        XCTAssertEqual(e1.notes, "First para & more.\n\nSecond para.\n\n• Item A")
        XCTAssertEqual(e1.artworkURL, "https://img.example/ep1.jpg")
        XCTAssertEqual(f.episodes[1].duration, 2710); XCTAssertEqual(f.episodes[1].enclosureType, "")
        XCTAssertEqual(f.episodes[1].notes, "Plain notes with escaped html & entity.")
        XCTAssertNotNil(f.episodes[1].published, "a date with no weekday and a zone name")
        XCTAssertEqual(f.episodes[2].duration, 2710.5); XCTAssertEqual(f.episodes[2].enclosureURL, "https://cdn.example/three.mp3", "the image enclosure is skipped")
        XCTAssertNotNil(f.episodes[2].published, "ISO 8601")
        XCTAssertFalse(f.episodes[3].playable); XCTAssertNil(f.episodes[3].duration)
        XCTAssertFalse(f.episodes[4].playable, "file: is not streamable"); XCTAssertNil(f.episodes[4].duration, "0 is unknown")
    }

    func testBadFeeds_6() {
        XCTAssertThrowsError(try RSSFeedParser.parse(Data("<!DOCTYPE html><html><body>hi</body></html>".utf8), feedURL: "x"))
        XCTAssertThrowsError(try RSSFeedParser.parse(Data(), feedURL: "x"))
        XCTAssertThrowsError(try RSSFeedParser.parse(Data("<rss><channel></channel></rss>".utf8), feedURL: "x"))
    }

    func testManyItemsKeepNewest300_7() throws {
        var s = "<rss><channel><title>Big</title>"
        for i in 0..<400 { s += "<item><title>E\(i)</title><guid>g\(i)</guid><enclosure url=\"https://c.example/\(i).mp3\" type=\"audio/mpeg\"/></item>" }
        s += "</channel></rss>"
        let f = try RSSFeedParser.parse(Data(s.utf8), feedURL: "x")
        XCTAssertEqual(f.episodes.count, 300); XCTAssertEqual(f.episodes[0].id, "g0")
    }

    func testDurationsAndDates_2_4() {
        XCTAssertEqual(PodcastText.duration("1:02:03"), 3723); XCTAssertEqual(PodcastText.duration("45:10"), 2710); XCTAssertEqual(PodcastText.duration("90"), 90)
        XCTAssertNil(PodcastText.duration("")); XCTAssertNil(PodcastText.duration("1:2:3:4")); XCTAssertNil(PodcastText.duration("x:10")); XCTAssertNil(PodcastText.duration("0"))
        XCTAssertEqual(PodcastText.durationLabel(3723), "1 h 2 min"); XCTAssertEqual(PodcastText.durationLabel(2700), "45 min"); XCTAssertEqual(PodcastText.durationLabel(7200), "2 h")
        XCTAssertEqual(PodcastText.clock(760), "12:40"); XCTAssertEqual(PodcastText.clock(3723), "1:02:03")
        XCTAssertNotNil(PodcastText.date("Mon, 10 Nov 2026 09:30 +0100")); XCTAssertNil(PodcastText.date("sometime")); XCTAssertNil(PodcastText.date(""))
    }

    func testRelativeDates() {
        let now = PodcastText.date("Mon, 16 Nov 2026 12:00:00 +0000")!
        func day(_ s: String) -> Double { PodcastText.date(s)! }
        XCTAssertEqual(PodcastText.relative(now - 60, now: now), "Today")
        XCTAssertEqual(PodcastText.relative(day("Sun, 15 Nov 2026 12:00:00 +0000"), now: now), "Yesterday")
        XCTAssertEqual(PodcastText.relative(day("Thu, 12 Nov 2026 12:00:00 +0000"), now: now), "4 days ago")
        XCTAssertEqual(PodcastText.relative(day("Mon, 02 Nov 2026 12:00:00 +0000"), now: now), "Nov 2")
        XCTAssertEqual(PodcastText.relative(day("Mon, 03 Nov 2025 12:00:00 +0000"), now: now), "Nov 3, 2025")
        XCTAssertEqual(PodcastText.relative(nil, now: now), "")
    }

    // MARK: Directory

    func testSearchDecode_8() throws {
        let body = #"{"resultCount":4,"results":[{"collectionName":"Show A","artistName":"Ann","feedUrl":"https://f.example/a.xml","artworkUrl600":"http://i.example/a.jpg"},{"collectionName":"No feed"},{"collectionName":"","feedUrl":"https://f.example/b.xml"},{"collectionName":"Show A again","feedUrl":"https://f.example/a.xml/"}]}"#
        let s = try PodcastDirectory.decodeSearch(Data(body.utf8))
        XCTAssertEqual(s.map(\.title), ["Show A"]); XCTAssertEqual(s[0].artworkURL, "https://i.example/a.jpg"); XCTAssertEqual(s[0].author, "Ann")
        XCTAssertEqual(try PodcastDirectory.decodeSearch(Data(#"{"results":[]}"#.utf8)), [])
        XCTAssertThrowsError(try PodcastDirectory.decodeSearch(Data("nope".utf8)))
        XCTAssertTrue(PodcastDirectory.searchURL("AT&T news")!.absoluteString.contains("term=AT%26T%20news"))
    }

    func testChartNewsOnly() throws {
        let body = #"{"feed":{"results":[{"id":"1","name":"Sports","genres":[{"name":"Sports"}]},{"id":"2","name":"Daily","genres":[{"name":"News"}]},{"id":"3","name":"Pol","genres":[{"name":"News Commentary"}]}]}}"#
        XCTAssertEqual(try PodcastDirectory.decodeChartNews(Data(body.utf8)).map(\.id), ["2", "3"])
        XCTAssertEqual(try PodcastDirectory.decodeChartNews(Data(body.utf8), limit: 1).count, 1)
    }

    func testSyncDecode_7() throws {
        let body = #"{"subscriptions":[{"feed_url":"https://f.example/a.xml","title":"https://f.example/a.xml","content_item":{"title":"Real A"}},{"feed_url":"","title":"x"},{"feed_url":"https://f.example/b.xml","title":"B","author":"Bo","artwork_url":"http://i.example/b.png"}]}"#
        let l = try PodcastSyncCodec.decodeList(Data(body.utf8))
        XCTAssertEqual(l.map(\.title), ["Real A", "B"]); XCTAssertEqual(l[1].artworkURL, "https://i.example/b.png")
    }

    // MARK: Service

    func tmp() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("pod-" + UUID().uuidString) }
    func service(dir: URL? = nil, now: @escaping @Sendable () -> Double = { 2_000_000_000 }) -> PodcastService {
        PodcastService(directory: dir, fetch: { u in
            if u.host == "feeds.example" { return Data(Self.feed.utf8) }
            throw URLError(.cannotConnectToHost)
        }, clock: now)
    }
    func settle(_ s: PodcastService, _ feed: String) async {
        for _ in 0..<200 { if s.cachedFeed(feed) != nil, !s.loading(feed) { return }; try? await Task.sleep(nanoseconds: 10_000_000) }
    }
    let feedURL = "https://feeds.example/show.xml"

    func testSubscribePersistsAndNoDoubles_1_8() async throws {
        let d = tmp(); defer { try? FileManager.default.removeItem(at: d) }
        let s = service(dir: d)
        XCTAssertTrue(s.subscribe(PodcastShow(feedURL: feedURL, title: "S")))
        XCTAssertFalse(s.subscribe(PodcastShow(feedURL: feedURL + "/", title: "S")), "a trailing slash is the same feed")
        XCTAssertFalse(s.unsubscribe("https://other.example/x"))
        await settle(s, feedURL)
        let again = service(dir: d)
        XCTAssertEqual(again.subscriptions().map(\.title), ["Synthetic Hour"], "persisted, with the feed's own title")
        XCTAssertNotNil(again.cachedFeed(feedURL), "the feed cache is on disk")
        XCTAssertTrue(again.unsubscribe(feedURL))
        XCTAssertEqual(service(dir: d).subscriptions(), [])
        try Data("{broken".utf8).write(to: d.appendingPathComponent("podcasts-library.json"))
        XCTAssertEqual(service(dir: d).subscriptions(), [], "a damaged file is an empty library")
    }

    func testUnplayedAndResume_5() async throws {
        let s = service()
        s.subscribe(PodcastShow(feedURL: feedURL, title: "S")); await settle(s, feedURL)
        var v = s.show(address: PodcastService.showAddress(feedURL))
        XCTAssertEqual(v.status, "ready"); XCTAssertEqual(v.episodes.count, 5)
        XCTAssertEqual(v.episodes.filter(\.unplayed).count, 3, "the episodes with audio and no position"); XCTAssertEqual(v.unplayedLine, "3 unplayed")
        let k = PodcastKey.episode(feed: feedURL, id: "guid-1")
        s.report(episodeId: k, seconds: 760, duration: 3723)
        XCTAssertEqual(s.resumeSeconds(k), 760)
        v = s.show(address: PodcastService.showAddress(feedURL))
        XCTAssertEqual(v.episodes[0].progress, "49 min left"); XCTAssertFalse(v.episodes[0].unplayed)
        let ev = s.episode(address: PodcastService.episodeAddress(feed: feedURL, id: "guid-1"))
        XCTAssertEqual(ev.resumeLabel, "Resume from 12:40"); XCTAssertTrue(ev.hasResume)
        s.report(episodeId: k, seconds: 3710, duration: 3723)
        XCTAssertNil(s.resumeSeconds(k), "within 20 s of the end is finished"); XCTAssertEqual(s.show(address: PodcastService.showAddress(feedURL)).episodes[0].progress, "Played")
        s.report(episodeId: "not-a-pod-id", seconds: 5, duration: nil); XCTAssertNil(s.position("not-a-pod-id"))
        let home = s.home(query: "")
        XCTAssertEqual(home.shows.count, 1); XCTAssertEqual(home.shows[0].unplayedCount, 2)
    }

    func testPlayAndQueueGoThroughTheHook() async throws {
        let s = service()
        s.subscribe(PodcastShow(feedURL: feedURL, title: "S")); await settle(s, feedURL)
        let box = Box()
        s.playHook = { r, enq in box.add((r, enq)) }
        let a = PodcastService.episodeAddress(feed: feedURL, id: "guid-1")
        s.report(episodeId: PodcastKey.episode(feed: feedURL, id: "guid-1"), seconds: 100, duration: 3723)
        XCTAssertEqual(s.play(address: a, mode: "resume").status, "ok")
        XCTAssertEqual(s.play(address: a, mode: "start").status, "ok")
        XCTAssertEqual(s.enqueue(address: a).status, "ok")
        let got = box.all
        XCTAssertEqual(got[0].0.start, 100); XCTAssertNil(got[1].0.start); XCTAssertEqual(got[2].1, true); XCTAssertEqual(got[0].0.show, "Synthetic Hour")
        XCTAssertEqual(got[0].0.url.absoluteString, "https://cdn.example/one.mp3")
        XCTAssertEqual(s.play(address: PodcastService.episodeAddress(feed: feedURL, id: "guid-4"), mode: "resume").status, "error", "no enclosure")
    }
    final class Box: @unchecked Sendable { var l: [(PodcastPlayRequest, Bool)] = []; let k = NSLock()
        func add(_ x: (PodcastPlayRequest, Bool)) { k.lock(); l.append(x); k.unlock() }
        var all: [(PodcastPlayRequest, Bool)] { k.lock(); defer { k.unlock() }; return l } }

    func testClipEntryPoint_3() async throws {
        let s = service()
        let opened = OpenedBox()
        s.opener = { a, t in opened.set(a, t) }
        // The four values a clip carries, for a show the reader does not follow.
        let a = PodcastEpisodes.open(episodeId: "guid-2", feedURL: feedURL, enclosureURL: "https://cdn.example/two.m4a", at: 760, service: s)
        XCTAssertEqual(opened.address, a)
        XCTAssertEqual(PodcastService.parseEpisode(a)?.id, "guid-2")
        await settle(s, feedURL)
        var v = s.episode(address: a)
        XCTAssertEqual(v.status, "ready"); XCTAssertEqual(v.title, "Two"); XCTAssertEqual(v.showTitle, "Synthetic Hour"); XCTAssertFalse(v.subscribed)
        XCTAssertTrue(v.playable); XCTAssertEqual(v.playingFrom, "From the clip at 12:40")
        s.nowHook = { PodcastNow(episodeId: PodcastKey.episode(feed: self.feedURL, id: "guid-2"), isClip: true, start: 760, elapsed: 770, duration: 2710) }
        v = s.episode(address: a)
        XCTAssertEqual(v.playingFrom, "Playing from 12:40"); XCTAssertTrue(v.fromClip); XCTAssertTrue(v.current)
        XCTAssertEqual(s.position(PodcastKey.episode(feed: feedURL, id: "guid-2"))?.seconds, 770, "the playhead is kept")
        let kept = KeepBox(); s.keepHook = { kept.hit() }; s.playHook = { _, _ in XCTFail("keep listening must not restart") }
        XCTAssertEqual(s.play(address: a, mode: "keep").status, "ok"); XCTAssertTrue(kept.did)
        // An episode that left the feed still plays from the clip's own audio.
        let gone = PodcastEpisodes.open(episodeId: "old-guid", feedURL: feedURL, enclosureURL: "https://cdn.example/old.mp3", at: 5, title: "Old one", service: s)
        let g = s.episode(address: gone)
        XCTAssertEqual(g.status, "ready"); XCTAssertEqual(g.title, "Old one"); XCTAssertTrue(g.playable); XCTAssertFalse(g.note.isEmpty)
        XCTAssertEqual(s.subscribe(PodcastShow(feedURL: feedURL, title: "")), true)
        XCTAssertTrue(s.episode(address: a).subscribed)
    }
    final class OpenedBox: @unchecked Sendable { var address = ""; func set(_ a: String, _ t: String) { address = a } }
    final class KeepBox: @unchecked Sendable { var did = false; func hit() { did = true } }

    func testFailingFeedIsASentence_2() async throws {
        let s = service()
        let a = PodcastService.showAddress("https://down.example/x.xml")
        _ = s.show(address: a)
        for _ in 0..<100 { if s.show(address: a).status == "error" { break }; try? await Task.sleep(nanoseconds: 10_000_000) }
        let v = s.show(address: a)
        XCTAssertEqual(v.status, "error"); XCTAssertTrue(v.message.contains("could not be reached"))
        XCTAssertEqual(s.show(address: "podcast:show/").status, "error")
    }

    func testSearchProvider() async throws {
        let s = service()
        s.subscribe(PodcastShow(feedURL: feedURL, title: "S")); await settle(s, feedURL)
        let h = s.searchHits("synthetic")
        XCTAssertEqual(h.first?.address, PodcastService.showAddress(feedURL)); XCTAssertEqual(h.first?.kind, "podcast")
        let inNotes = s.searchHits("escaped")
        XCTAssertEqual(inNotes.map(\.address), [PodcastService.episodeAddress(feed: feedURL, id: "guid-2")]); XCTAssertTrue(inNotes[0].detail.contains("in the notes"))
        XCTAssertEqual(s.searchHits("  "), [])
        SearchProviders.register(name: "podcasts") { s.searchHits($0) }
        SearchProviders.register(name: "podcasts") { s.searchHits($0) }
        XCTAssertEqual(SearchProviders.hits(for: "synthetic").filter { $0.address == PodcastService.showAddress(feedURL) }.count, 1)
        SearchProviders.unregister(name: "podcasts")
    }

    func testAddresses() {
        XCTAssertEqual(PageAddress.kind(of: "podcasts:"), "podcast"); XCTAssertEqual(PageAddress.kind(of: "podcast:show/x"), "podcast")
        XCTAssertEqual(PageAddress.defaultTitle("podcasts:"), "Podcasts")
        let a = PodcastService.episodeAddress(feed: "https://f.example/a?b=1&c=2", id: "tag:x/1 2")
        XCTAssertEqual(PodcastService.parseEpisode(a)?.feed, "https://f.example/a?b=1&c=2"); XCTAssertEqual(PodcastService.parseEpisode(a)?.id, "tag:x/1 2")
        XCTAssertEqual(PodcastKey.parseEpisode(PodcastKey.episode(feed: "https://f.example/a", id: "g/1"))?.id, "g/1")
        XCTAssertTrue(PageAddress.places.contains { $0.address == "podcasts:" })
    }
}
