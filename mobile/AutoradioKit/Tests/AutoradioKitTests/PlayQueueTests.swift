import XCTest
@testable import AutoradioKit

@MainActor
final class FakeEngine: PlaybackEngine {
    var log: [String] = []
    var failRoutes: Set<String> = []
    var elapsed = 0.0
    func playStory(route: String, title: String) async -> Bool { log.append("story:\(route)"); return !failRoutes.contains(route) }
    func speak(address: String, title: String, paragraphs: [String]) async -> Bool { log.append("speak:\(address):\(paragraphs.count)"); return true }
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
    func setRemoteRate(_ r: Double) { log.append("rate:\(r)") }
    func setRemoteLevel(_ l: PlayLevel) { log.append("level:\(l.rawValue)") }
}

@MainActor
final class PlayQueueTests: XCTestCase {
    func story(_ n: String) -> PlayItem { .story(route: n, title: "S \(n)") }
    func clip(_ n: String, end: Double? = 30) -> PlayItem { .audio(url: URL(string: "https://x.test/\(n).mp3")!, title: "T \(n)", show: "Show", start: 10, end: end, episodeId: "e\(n)") }
    func make() -> (PlayQueue, FakeEngine) {
        let q = PlayQueue(); let e = FakeEngine(); q.engine = e
        q.pageText = { _, t in (t, ["one", "two", "three"]) }
        q.registerTextSource(kind: "document") { _, at in Array(["a", "b", "c", "d"].dropFirst(at)) }
        return (q, e)
    }

    func testOrder() async {
        let (q, _) = make()
        q.enqueue(story("a")); q.enqueue(story("b")); q.playNext(story("c"))
        XCTAssertEqual(q.upcoming.map(\.title), ["S c", "S a", "S b"])
        q.remove(at: 1)
        XCTAssertEqual(q.upcoming.map(\.title), ["S c", "S b"])
        q.playNow(story("z")); await q.settle()
        XCTAssertEqual(q.current, story("z"))
        XCTAssertEqual(q.upcoming.count, 2)
    }

    func testAdvanceOnSpokenEnd() async {
        let (q, e) = make()
        q.playNow(.page(url: URL(string: "https://x.test/p")!, title: "P")); await q.settle()
        XCTAssertEqual(e.log.last, "speak:q1:3")
        q.enqueue(clip("a"))
        let started = await q.spokenEnded(documentID: "text:q1")
        XCTAssertTrue(started)
        XCTAssertEqual(q.current, clip("a"))
        XCTAssertEqual(e.log.last, "remote:a.mp3:10.0:30.0")
    }

    func testClipEndAdvancesAndKeepListeningContinues() async {
        let (q, e) = make()
        q.playNow(clip("a")); await q.settle()
        q.enqueue(story("n"))
        XCTAssertEqual(q.keepListeningTitle, "T a (Show)")
        q.keepListening()
        XCTAssertNil(q.keepListeningTitle)
        XCTAssertEqual(e.log.last, "clearEnd")
        XCTAssertEqual(q.current, clip("a", end: nil))
        await q.remoteEnded(generation: q.generation)     // the episode itself ran out
        XCTAssertEqual(q.current, story("n"))
        // a clip that reaches its end advances too
        q.playNow(clip("b")); await q.settle()
        await q.remoteEnded(generation: q.generation)
        XCTAssertNil(q.current)
        // a stale end does nothing
        q.playNow(clip("c")); await q.settle()
        await q.remoteEnded(generation: q.generation - 1)
        XCTAssertEqual(q.current, clip("c"))
    }

    func testDefaultSourceRefillsAfterHandQueued() async {
        let (q, _) = make()
        var refilled = 0
        q.defaultSource = { refilled += 1; return true }
        q.playNow(.document(id: "d", title: "D", at: 2)); await q.settle()
        let r = await q.spokenEnded(documentID: "text:q1")
        XCTAssertTrue(r); XCTAssertEqual(refilled, 1)
        // a paper story ending with nothing queued leaves the paper's list in charge
        q.playNow(story("a")); await q.settle()
        let s = await q.storyEnded(route: "a")
        XCTAssertFalse(s); XCTAssertEqual(refilled, 1)
        // hand-queued plays before the default
        q.enqueue(story("b"))
        let s2 = await q.storyEnded(route: "a")
        XCTAssertTrue(s2); XCTAssertEqual(refilled, 1); XCTAssertEqual(q.current, story("b"))
    }

    func testUnplayableIsSkipped() async {
        let (q, e) = make()
        e.failRoutes = ["bad"]
        q.enqueue(story("good"))
        q.playNow(story("bad")); await q.settle()
        XCTAssertEqual(q.current, story("good"))
    }

    func testDocumentFromPositionAndReport() async {
        let (q, e) = make()
        var got: [Int] = []
        q.onDocumentPosition = { _, b in got.append(b) }
        q.playNow(.document(id: "d", title: "D", at: 2)); await q.settle()
        XCTAssertEqual(e.log.last, "speak:q1:2")
        q.spokenBlock(1)
        XCTAssertEqual(got, [3]); XCTAssertEqual(q.documentBlock, 3)
    }

    func testPauseForTurnAndSeek() async {
        let (q, e) = make()
        q.playNow(clip("a")); await q.settle()
        q.setLevel(.pause); q.setLevel(.play); q.seek(by: -15); q.pause(); q.resume()
        XCTAssertEqual(Array(e.log.suffix(5)), ["level:pause", "level:play", "seekBy:-15.0", "pauseRemote", "resumeRemote"])
        q.playNow(story("x")); await q.settle()
        e.log = []; q.setLevel(.pause)
        XCTAssertTrue(e.log.isEmpty)
    }

    func testPersistence() async {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("q-\(UUID()).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let q = PlayQueue(storage: f); let e = FakeEngine(); q.engine = e; e.elapsed = 42
        q.playNow(clip("a")); await q.settle(); q.enqueue(story("n")); q.pause()
        let q2 = PlayQueue(storage: f); let e2 = FakeEngine(); q2.engine = e2
        q2.restore()
        XCTAssertEqual(q2.current, clip("a")); XCTAssertEqual(q2.upcoming, [story("n")])
        q2.resume(); await q2.settle()
        XCTAssertEqual(e2.log.last, "remote:a.mp3:42.0:30.0")
    }

    func testURLDetection() {
        XCTAssertEqual(BarURL.detect("https://choir.news/a?b=1")?.host, "choir.news")
        XCTAssertEqual(BarURL.detect("choir.news")?.absoluteString, "https://choir.news")
        XCTAssertEqual(BarURL.detect("example.com/path/x")?.host, "example.com")
        XCTAssertNil(BarURL.detect("what is choir.news?"))
        XCTAssertNil(BarURL.detect("Tell me about it. Thanks"))
        XCTAssertNil(BarURL.detect("hello."))
        XCTAssertNil(BarURL.detect("done.Then"))
        XCTAssertNil(BarURL.detect("a@b.com"))
        XCTAssertNil(BarURL.detect("choir.news?"))
        XCTAssertNil(BarURL.detect("ftp://x.com"))
        XCTAssertNil(BarURL.detect(""))
    }

    func testPageText() {
        let body = String(repeating: "<p>Real sentence here.</p>", count: 30)
        let html = "<html><head><title>Hi &amp; Bye</title></head><body><nav>MENU</nav><article>\(body)</article><footer>FOOT</footer></body></html>"
        let r = PageText.paragraphs(html: html, fallbackTitle: "x")
        XCTAssertEqual(r.title, "Hi & Bye")
        XCTAssertFalse(r.paragraphs.joined().contains("MENU"))
        XCTAssertEqual(r.paragraphs.first, "Hi & Bye")
    }
}
