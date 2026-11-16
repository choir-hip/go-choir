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

@MainActor
final class QueueOpsTests: XCTestCase {
    func story(_ n: String) -> PlayItem { .story(route: n, title: "S \(n)") }
    func make() -> (PlayQueue, FakeEngine) { let q = PlayQueue(); let e = FakeEngine(); q.engine = e; return (q, e) }

    func testOpsJSONAndSnapshot() async throws {
        let (q, _) = make()
        let json = #"{"queue_ops":[{"op":"enqueue","item":{"kind":"story","route":"a","title":"A"}},{"op":"enqueue","item":{"kind":"audio","url":"https://x.test/e.mp3","title":"E","show":"Sh","start":5,"end":20,"episode_id":"ep1"}},{"op":"bogus"},{"op":"play_next","item":{"kind":"page","url":"https://x.test/p","title":"P"}}]}"#
        let ops = QueueOp.decode(Data(json.utf8))
        XCTAssertEqual(ops.count, 3)
        q.apply(ops, by: .dj)
        XCTAssertEqual(q.entries.map(\.item.title), ["P", "A", "E"])
        XCTAssertEqual(q.version, 3)
        XCTAssertEqual(q.entries[0].note, "DJ added")
        let snap = try JSONSerialization.jsonObject(with: Data(q.snapshotJSON().utf8)) as! [String: Any]
        XCTAssertEqual(snap["version"] as? Int, 3)
        let up = snap["upcoming"] as! [[String: Any]]
        XCTAssertEqual(up.count, 3); XCTAssertEqual(up[0]["added_by"] as? String, "dj")
        XCTAssertEqual((up[2]["item"] as! [String: Any])["episode_id"] as? String, "ep1")
        // op encodes as the tag the desk reads
        let enc = String(data: try JSONEncoder().encode(QueueOp.move(id: "x", toIndex: 0)), encoding: .utf8)!
        XCTAssertTrue(enc.contains("\"op\":\"move\"") && enc.contains("\"to_index\":0"))
    }

    // Design lane: Up next says who put each item there ("by", a whole sentence the screen shows as a quiet caption), and the DJ's own reason
    // stays in `note`. A row the reader added says "Added by you"; the screen shows the note when there is one, else `by`.
    func testUpNextRowsSayWhoAddedThem() async {
        let (q, _) = make()
        q.apply([.enqueue(story("a"))], by: .reader)
        q.apply([.enqueue(story("b"))], by: .dj)
        q.apply([.enqueue(story("c"))], by: .app)
        let rows = q.info().upcoming
        XCTAssertEqual(rows.map(\.by), ["Added by you", "Added by the DJ", "From the station"])
        XCTAssertEqual(rows[0].note, "")
        XCTAssertFalse(rows[1].note.isEmpty, "the DJ's reason stays in note")
    }

    func testMoveRemoveByIdAndLog() async {
        let (q, _) = make()
        q.apply([.enqueue(story("a")), .enqueue(story("b")), .enqueue(story("c"))], by: .reader)
        let c = q.entries[2].id
        q.apply([.move(id: c, toIndex: 0)], by: .dj)
        XCTAssertEqual(q.entries.map(\.item.title), ["S c", "S a", "S b"])
        XCTAssertEqual(q.entries[0].note, "DJ moved this up")
        q.apply([.remove(id: "nope")], by: .dj)                       // unknown id: nothing changes
        XCTAssertEqual(q.entries.count, 3)
        let v = q.version
        q.apply([.remove(id: q.entries[1].id)], by: .reader)
        XCTAssertEqual(q.version, v + 1)
        XCTAssertEqual(q.log.last?.actor, .reader)
        XCTAssertEqual(q.log.filter { $0.actor == .dj }.count, 1)
        q.apply([.clear], by: .app)
        XCTAssertTrue(q.entries.isEmpty)
    }

    func testSkipAndPreviousAndIds() async {
        let (q, e) = make()
        q.apply([.enqueue(story("a")), .enqueue(story("b"))], by: .reader)
        let idA = q.entries[0].id
        q.apply([.skip()], by: .dj); await q.settle()
        XCTAssertEqual(q.current, story("a")); XCTAssertEqual(q.currentId, idA)
        q.apply([.previous], by: .reader); await q.settle()
        XCTAssertEqual(e.log.filter { $0 == "story:a" }.count, 2); XCTAssertEqual(q.currentId, idA)
        XCTAssertEqual(q.snapshot().current?.id, idA)
    }

    func testStation() async {
        let q = PlayQueue(); let e = FakeEngine(); q.engine = e
        let s = Station(); s.queue = q
        var dj = 0
        s.startDJ = { dj += 1 }
        s.otherAudioPlaying = { true }
        let blocked = await s.start(reason: .launch)
        XCTAssertFalse(blocked); XCTAssertEqual(dj, 0)
        s.otherAudioPlaying = { false }
        s.isReady = { false }
        let notReady = await s.start(reason: .launch)
        XCTAssertFalse(notReady)
        s.isReady = { true }
        let go = await s.start(reason: .launch)
        XCTAssertTrue(go); XCTAssertEqual(dj, 1)
        let again = await s.start(reason: .foreground)       // a short return does not restart it
        XCTAssertFalse(again); XCTAssertEqual(dj, 1)
    }

    func testStationResumesSavedItem() async {
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("q-\(UUID()).json")
        defer { try? FileManager.default.removeItem(at: f) }
        let q = PlayQueue(storage: f); q.engine = FakeEngine()
        q.apply([.enqueue(story("n"))], by: .reader)
        q.noteStarted(story("cur")); q.save()
        let q2 = PlayQueue(storage: f); let e2 = FakeEngine(); q2.engine = e2; q2.restore()
        let s = Station(); s.queue = q2; s.otherAudioPlaying = { false }
        var dj = 0; s.startDJ = { dj += 1 }
        let ok = await s.start(reason: .launch); await q2.settle()
        XCTAssertTrue(ok); XCTAssertEqual(dj, 0); XCTAssertEqual(e2.log, ["stopRemote", "story:cur"])
        XCTAssertEqual(q2.entries.count, 1)
    }
}

@MainActor
final class QueueRebaseTests: XCTestCase {
    func story(_ n: String) -> PlayItem { .story(route: n, title: "S \(n)") }
    func make() -> (PlayQueue, FakeEngine) { let q = PlayQueue(); let e = FakeEngine(); q.engine = e; return (q, e) }

    func testIdOpsApplyWhateverTheVersionAndDropWhenGone() {
        let (q, _) = make()
        q.apply([.enqueue(story("a")), .enqueue(story("b")), .enqueue(story("c"))], by: .reader)
        let ids = q.entries.map(\.id)
        q.apply([.remove(id: ids[0])], by: .reader)                       // the phone moved on: version is now 4
        let r = q.apply(serverOps: [.move(id: ids[2], toIndex: 0, reason: "you asked for the long read next"),
                                     .remove(id: ids[0], reason: "gone already"), .move(id: "q999", toIndex: 0)], turnId: "t1", baseVersion: 3)
        XCTAssertEqual(r.applied, 1); XCTAssertEqual(r.dropped, 2)
        XCTAssertEqual(q.entries.map(\.id), [ids[2], ids[1]])
        XCTAssertEqual(q.entries[0].note, "you asked for the long read next")      // the reason is what Up next shows
        // to_index is clamped
        q.apply(serverOps: [.move(id: ids[2], toIndex: 99)], turnId: "t2", baseVersion: 1)
        XCTAssertEqual(q.entries.map(\.id), [ids[1], ids[2]])
    }

    func testSkipOnlyIfStillCurrent() async {
        let (q, _) = make()
        q.apply([.enqueue(story("a")), .enqueue(story("b")), .enqueue(story("c"))], by: .reader)
        q.apply([.skip()], by: .reader); await q.settle()
        let cur = q.currentId!
        let stale = q.apply(serverOps: [.skip(id: "q99")], turnId: "t1", baseVersion: 0)   // that entry already finished
        XCTAssertEqual(stale.applied, 0); XCTAssertEqual(q.currentId, cur)
        let ok = q.apply(serverOps: [.skip(id: cur, reason: "boring")], turnId: "t2", baseVersion: 0); await q.settle()
        XCTAssertEqual(ok.applied, 1); XCTAssertNotEqual(q.currentId, cur)
    }

    func testAddOpsAlwaysApply() {
        let (q, _) = make()
        q.apply([.enqueue(story("a"))], by: .reader)
        let r = q.apply(serverOps: [.playNext(story("x"), reason: "from the front page"), .enqueue(story("y"))], turnId: "t1", baseVersion: -5)
        XCTAssertEqual(r.applied, 2)
        XCTAssertEqual(q.entries.map(\.item.title), ["S x", "S a", "S y"])
        XCTAssertEqual(q.entries[0].addedBy, .dj); XCTAssertEqual(q.entries[0].note, "from the front page")
    }

    func testClearAndPreviousNeedTheBaseVersion() {
        let (q, _) = make()
        q.apply([.enqueue(story("a"))], by: .reader)
        let v = q.version
        let stale = q.apply(serverOps: [.clear], turnId: "t1", baseVersion: v - 1)
        XCTAssertEqual(stale.applied, 0); XCTAssertEqual(q.entries.count, 1)
        let fresh = q.apply(serverOps: [QueueOp(.clear, reason: "start over", baseVersion: v)], turnId: "t2", baseVersion: nil)
        XCTAssertEqual(fresh.applied, 1); XCTAssertTrue(q.entries.isEmpty)
    }

    func testBaseVersionIsTheBatchStartNotEachOp() {
        let (q, _) = make()
        q.apply([.enqueue(story("a"))], by: .reader)
        let v = q.version
        // an add then a clear in one turn, both based on v: the clear still applies (the add was this turn's own change)
        let r = q.apply(serverOps: [.enqueue(story("b")), .clear], turnId: "t1", baseVersion: v)
        XCTAssertEqual(r.applied, 2); XCTAssertTrue(q.entries.isEmpty)
    }

    func testTurnIsAppliedOnce() throws {
        let (q, _) = make()
        let ev = #"{"turn_id":"turn-9","base_version":0,"replayed":false,"ops":[{"op":"enqueue","item":{"kind":"story","route":"r1","title":"One"},"base_version":0,"reason":"top story"}]}"#
        XCTAssertEqual(q.applyQueueOpsEvent(Data(ev.utf8)).applied, 1)
        let again = ev.replacingOccurrences(of: "\"replayed\":false", with: "\"replayed\":true")
        XCTAssertEqual(q.applyQueueOpsEvent(Data(again.utf8)).applied, 0)
        XCTAssertEqual(q.entries.count, 1)
        XCTAssertEqual(q.entries[0].note, "top story")
    }

    func testIdsNeverReusedAndMatchTheWireRule() {
        let (q, _) = make()
        q.apply([.enqueue(story("a")), .enqueue(story("b"))], by: .reader)
        let first = q.entries.map(\.id)
        q.apply([.clear], by: .reader)
        q.apply([.enqueue(story("c"))], by: .reader)
        XCTAssertFalse(first.contains(q.entries[0].id))
        let re = try! NSRegularExpression(pattern: "^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$")
        for id in first + q.entries.map(\.id) { XCTAssertNotNil(re.firstMatch(in: id, range: NSRange(id.startIndex..., in: id))) }
    }

    func testVersionBumpsWhenCurrentFinishesAndSnapshotIsTheWireShape() async throws {
        let (q, _) = make()
        q.playNow(.audio(url: URL(string: "https://x.test/e.mp3")!, title: "E", show: "S", start: nil, end: nil, episodeId: nil)); await q.settle()
        let v = q.version
        await q.remoteEnded(generation: q.generation)
        XCTAssertGreaterThan(q.version, v)
        let o = try JSONSerialization.jsonObject(with: Data(q.snapshotJSON().utf8)) as! [String: Any]
        XCTAssertTrue(o["current"] is NSNull)
        XCTAssertEqual(o["upcoming"] as? [Any] != nil, true)
        q.apply([.enqueue(.audio(url: URL(string: "https://x.test/f.mp3")!, title: "F", show: "S", start: 1, end: nil, episodeId: nil))], by: .dj)
        let o2 = try JSONSerialization.jsonObject(with: Data(q.snapshotJSON().utf8)) as! [String: Any]
        let item = ((o2["upcoming"] as! [[String: Any]])[0])["item"] as! [String: Any]
        XCTAssertEqual(item["kind"] as? String, "audio"); XCTAssertEqual(item["start"] as? Double, 1)
        XCTAssertEqual(((o2["upcoming"] as! [[String: Any]])[0])["added_by"] as? String, "dj")
    }
}

@MainActor
final class QueueDuplicateAndWireTests: XCTestCase {
    func story(_ n: String, _ t: String = "") -> PlayItem { .story(route: n, title: t.isEmpty ? "S \(n)" : t) }
    func make() -> (PlayQueue, FakeEngine) { let q = PlayQueue(); let e = FakeEngine(); q.engine = e; return (q, e) }

    // 1. Queueing what is playing now is a no-op, said quietly. A second copy of something not playing stays allowed.
    func testQueueingTheCurrentItemIsANoOp() async {
        let (q, _) = make()
        q.playNow(story("a")); await q.settle()
        let v = q.version
        q.enqueue(story("a", "A different title for the same story"))
        q.playNext(story("a"))
        XCTAssertTrue(q.entries.isEmpty); XCTAssertEqual(q.version, v)
        XCTAssertEqual(q.message, "Already playing.")
        q.enqueue(story("b")); q.enqueue(story("b"))                       // an intentional second copy of something not playing
        XCTAssertEqual(q.entries.count, 2)
        // the same holds for a page, a book and an audio file, from any actor
        q.playNow(.page(url: URL(string: "https://x.test/p")!, title: "P")); await q.settle()
        q.apply([.enqueue(.page(url: URL(string: "https://x.test/p")!, title: "Other"))], by: .dj)
        XCTAssertEqual(q.entries.count, 2)
        q.playNow(.audio(url: URL(string: "https://x.test/e.mp3")!, title: "E", show: "S", start: nil, end: nil, episodeId: "e1")); await q.settle()
        q.enqueue(.audio(url: URL(string: "https://x.test/e.mp3")!, title: "E", show: "S", start: 5, end: 30, episodeId: "e1"))   // a clip of the episode that is playing
        XCTAssertEqual(q.entries.count, 2)
        // playing it again from the paper is allowed (a restart), and stationed items seeded by noteStarted are not queued
        q.noteStarted(story("c")); q.enqueue(story("c")); XCTAssertEqual(q.entries.count, 2)
    }

    // 2. The snapshot keeps to the desk's bounds: at most 50 upcoming, strings clamped, an entry that cannot be clamped left out.
    func testSnapshotKeepsToTheWireBounds() throws {
        let (q, _) = make()
        for i in 0..<60 { q.enqueue(story("r\(i)", String(repeating: "é", count: 400))) }
        q.enqueue(.page(url: URL(string: "https://x.test/" + String(repeating: "a", count: 1100))!, title: "long"))
        let o = try JSONSerialization.jsonObject(with: Data(q.snapshotJSON().utf8)) as! [String: Any]
        let up = o["upcoming"] as! [[String: Any]]
        XCTAssertEqual(up.count, 50)
        XCTAssertTrue(up.allSatisfy { (($0["item"] as! [String: Any])["title"] as! String).utf8.count <= 300 })
        XCTAssertEqual(up.first?["id"] as? String, q.entries[0].id)
    }

    // 3. The exact event the server sent in the staging proof, and the apply-once rule.
    func testTheStagingEventDecodesAndAppliesOnce() {
        let (q, _) = make()
        for n in ["a", "b", "c", "d", "e"] { q.enqueue(story(n)) }          // ids q1 ... q5
        let id4 = q.entries[3].id; XCTAssertEqual(id4, "q4")
        let event = #"{"turn_id":"cmd-7f3a9c","base_version":7,"replayed":false,"ops":[{"op":"move","id":"q4","to_index":0,"base_version":7,"reason":"you asked for the long read next"}]}"#
        XCTAssertEqual(QueueOp.decode(Data(event.utf8)), [QueueOp(.move, id: "q4", toIndex: 0, reason: "you asked for the long read next", baseVersion: 7)])
        let r = q.applyQueueOpsEvent(Data(event.utf8))
        XCTAssertEqual(r.applied, 1)
        XCTAssertEqual(q.entries.first?.id, "q4")
        XCTAssertEqual(q.entries.first?.note, "you asked for the long read next")        // the row shows the DJ's reason
        XCTAssertEqual(q.info().upcoming.first?.note, "you asked for the long read next")
        let replay = event.replacingOccurrences(of: "\"replayed\":false", with: "\"replayed\":true")
        XCTAssertEqual(q.applyQueueOpsEvent(Data(replay.utf8)).applied, 0)
    }

    // A replay that arrives with no ops (the retry's version differed) is not the turn's application: a later delivery still applies.
    func testAnEmptyReplayDoesNotConsumeTheTurn() {
        let (q, _) = make()
        q.enqueue(story("a")); q.enqueue(story("b"))
        let empty = #"{"turn_id":"t9","base_version":3,"replayed":true,"ops":[]}"#
        XCTAssertEqual(q.applyQueueOpsEvent(Data(empty.utf8)).applied, 0)
        let real = #"{"turn_id":"t9","base_version":2,"replayed":false,"ops":[{"op":"move","id":"q2","to_index":0,"base_version":2,"reason":"r"}]}"#
        XCTAssertEqual(q.applyQueueOpsEvent(Data(real.utf8)).applied, 1)
    }

    // 4. A candidate the desk would refuse (a route with a space, @ or #, or over 256 bytes) is left out, not sent.
    func testCandidatesAreShapedForTheDesk() {
        let cands = [DeskCandidate(route: "good-one", title: "Good"), DeskCandidate(route: "has space", title: "x"), DeskCandidate(route: "a@b", title: "x"),
                     DeskCandidate(route: "a#b", title: "x"), DeskCandidate(route: String(repeating: "r", count: 257), title: "x"),
                     DeskCandidate(route: "ok", title: String(repeating: "t", count: 400))]
        let out = AutoradioDeskStage.wireCandidates(cands)
        XCTAssertEqual(out.map { $0["route"] }, ["good-one", "ok"])
        XCTAssertLessThanOrEqual(out[1]["title"]!.utf8.count, 300)
        XCTAssertEqual(AutoradioDeskStage.wireCandidates((0..<60).map { DeskCandidate(route: "r\($0)", title: "t") }).count, 40)
    }
}
