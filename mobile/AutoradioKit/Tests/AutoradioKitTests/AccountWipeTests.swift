import XCTest
@testable import AutoradioKit

// Sign-out wipes everything of the account that sits on the phone, and the first request of each kind after the next sign-in carries none of it.
// Failure list (written before the code):
//  1  after sign-out no file under the stores' folders holds the marker
//  2  after sign-out every in-memory store answers empty
//  3  after the next sign-in the first turn, programme, steer flush, sheet post, podcast sync and texture retry carry no marker
//  4  pending run sheet ops are dropped at sign-out, never posted under the next key
//  5  the revoke is sent in the background with the old key, after the local key is gone
@MainActor
final class AccountWipeTests: XCTestCase {
    static let marker = "MARKER-7731"
    var root: URL!

    override func setUp() {
        root = FileManager.default.temporaryDirectory.appendingPathComponent("acctwipe-\(UUID().uuidString)")
        try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
    }
    override func tearDown() { try? FileManager.default.removeItem(at: root) }

    final class OfflinePoster: RunSheetPoster, @unchecked Sendable {
        let lock = NSLock(); private(set) var posted: [[RunSheetOp]] = []
        var outcome: RunSheetPostOutcome = .notYet
        func post(ops: [RunSheetOp]) async -> RunSheetPostOutcome { lock.lock(); posted.append(ops); lock.unlock(); return outcome }
    }
    final class FakeAPI: TextureAPI {
        var posts: [String] = []
        var failure: Error? = URLError(.notConnectedToInternet)
        func head(docId: String) async throws -> TextureHead { TextureHead(content: "base", body: nil, revisionId: "r1", version: 1) }
        func post(docId: String, content: String, body: [String: Any], parent: String) async throws -> (revisionId: String, version: Int) {
            if let failure { throw failure }
            posts.append(content); return ("r2", 2)
        }
    }
    final class Recorder: @unchecked Sendable {
        let lock = NSLock(); private var _v: [String] = []
        func add(_ s: String) { lock.lock(); _v.append(s); lock.unlock() }
        var all: [String] { lock.lock(); defer { lock.unlock() }; return _v }
    }

    func markerInFiles() -> [String] {
        var hits: [String] = []
        let en = FileManager.default.enumerator(at: root, includingPropertiesForKeys: [.isRegularFileKey])
        while let u = en?.nextObject() as? URL {
            guard (try? u.resourceValues(forKeys: [.isRegularFileKey]))?.isRegularFile == true, let d = try? Data(contentsOf: u) else { continue }
            if d.range(of: Data(Self.marker.utf8)) != nil { hits.append(u.path.replacingOccurrences(of: root.path, with: "")) }
        }
        return hits
    }
    func write(_ rel: String, _ text: String) {
        let u = root.appendingPathComponent(rel)
        try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
        try? Data(text.utf8).write(to: u)
    }

    func testEverythingOfTheAccountGoesAndTheNextSignInSendsNone() async throws {
        let m = Self.marker
        // The stores, each in its own place under the temporary root.
        let turns = TurnHistory(); turns.add(listener: m, radio: [m])
        let cache = NotebookCache(file: root.appendingPathComponent("notebook-cache.md")); cache.set(m)
        let topics = root.appendingPathComponent("edition-topics.json"), topicsMarker = root.appendingPathComponent("notebook-topics-written")
        write("edition-topics.json", "{\"topics\":[\"\(m)\"]}"); write("notebook-topics-written", m)
        let notebook = NotebookStore(); notebook.doc = .found(address: "texture:x", notebook: NotebookText.parse("# My notebook\n\n## What you asked for\n- \(m)\n")); notebook.selected = m
        let steerSent = Recorder()
        let steer = SteerQueue(file: root.appendingPathComponent("steer-queue.json"), send: { _ in .offline })
        steer.enqueue(SteerTap(id: "s-1", entryID: "e1", direction: "more", stories: [m], title: m, handle: nil, at: "2026-11-18T00:00:00Z"))
        let own = TakeOwn(file: root.appendingPathComponent("takes-own.json")); own.add(id: m, handle: m)
        let mutes = TakeMutes(file: root.appendingPathComponent("takes-muted.json")); mutes.mute(m)
        let takes = TakeStore(); takes.replace([Take.parse(["take_id": m, "publisher": "pub", "state": "live", "transcript": m])!])
        let tmp = root.appendingPathComponent("tmp"); write("tmp/takes/rec.m4a", m); write("tmp/take-play-\(m).m4a", m)
        let sheet = RunSheetStore(storage: root.appendingPathComponent("runsheet.json")); sheet.retryAfter = 3600
        let poster = OfflinePoster(); sheet.poster = poster
        sheet.place(.paperStory(route: m, title: m), at: .queued); await sheet.flushTask?.value
        XCTAssertFalse(sheet.pending.isEmpty, "the route said not yet: the op is pending")
        let opening = OpeningCache(dir: root.appendingPathComponent("opening"))
        opening.put(.opening, entry: .paperStory(route: m, title: m), titles: [m], audio: [PreparedAudio(text: m, samples: [0.1, 0.2], rate: 24000)], voice: "v")
        let clips = ClipLocator(directory: root.appendingPathComponent("clips")); write("clips/window-1.mp3", m)
        let pods = PodcastService(directory: root.appendingPathComponent("pod"), fetch: { _ in
            Data("<rss><channel><title>\(m)</title><item><guid>g1</guid><title>\(m)</title><enclosure url=\"https://e.example/a.mp3\" type=\"audio/mpeg\"/></item></channel></rss>".utf8) })
        let feed = "https://example.com/\(m)"
        pods.subscribe(PodcastShow(feedURL: feed, title: m))
        pods.report(episodeId: PodcastKey.episode(feed: PodcastKey.feed(feed), id: "g1"), seconds: 42, duration: 600)
        for _ in 0..<60 where !FileManager.default.fileExists(atPath: root.appendingPathComponent("pod/podcast-feeds").path) { try await Task.sleep(nanoseconds: 50_000_000) }
        let api = FakeAPI()
        let texture = TextureEditor(api: api, storage: root.appendingPathComponent("texture-pending.json")); texture.retryAfter = 3600
        await texture.edit(address: "texture:d1", markdown: m); await texture.flush(address: "texture:d1")
        XCTAssertEqual(texture.pending["d1"], m)
        let env = ChoirEnvironment(); env.setPlaying(PlayingProgramme(programmeID: "p1", title: m, route: m, publicationVersionID: "v", manifestHash: "h", blocks: [m]))
        let content = ContentStore(root: root.appendingPathComponent("content"))
        content.put(StoredItem(id: "mail:1", kind: .mail, title: m, text: m, address: "mail:1")); content.flush()
        let mailService = MailService(environment: env)
        write("cache/mail-abc.html", "<p>\(m)</p>")

        let wipe = AccountWipe()
        wipe.registerStandard(turnHistory: turns, notebookCache: cache, topicsFile: topics, topicsMarker: topicsMarker, notebook: notebook, steer: steer,
                              takeOwn: own, takeMutes: mutes, takeStore: takes, tempDirectory: tmp, runSheet: sheet, openingCache: opening, clips: clips,
                              podcasts: pods, texture: texture, environment: env, content: content, corpus: SavedCorpus())
        let cacheDir = root.appendingPathComponent("cache")
        wipe.register("mail") { mailService.wipe(originalsIn: cacheDir) }
        XCTAssertFalse(markerInFiles().isEmpty, "the test filled the files")

        await wipe.run()

        // (a) no file holds the marker
        XCTAssertEqual(markerInFiles(), [], "files still holding the account's words")
        // (b) every in-memory store answers empty
        XCTAssertTrue(turns.snapshot().isEmpty); XCTAssertNil(cache.text)
        if case .loading = notebook.doc {} else { XCTFail("the notebook page still holds the document") }
        XCTAssertNil(notebook.learning); XCTAssertEqual(notebook.selected, "")
        XCTAssertTrue(steer.taps.isEmpty); XCTAssertEqual(own.handle, ""); XCTAssertTrue(mutes.all.isEmpty); XCTAssertNil(takes.take(m))
        XCTAssertTrue(sheet.sheet.entries.isEmpty); XCTAssertTrue(sheet.pending.isEmpty); XCTAssertNil(sheet.poster, "the poster is rebuilt at the next sign-in")
        XCTAssertNil(opening.opening(voice: "v")); XCTAssertTrue(pods.subscriptions().isEmpty)
        XCTAssertNil(pods.position(PodcastKey.episode(feed: PodcastKey.feed(feed), id: "g1"))); XCTAssertNil(pods.cachedFeed(feed))
        XCTAssertTrue(texture.pending.isEmpty); XCTAssertNil(env.playing("p1")); XCTAssertNil(content.get("mail:1"))
        XCTAssertNil(mailService.cachedRows(folder: "inbox")); XCTAssertNil(mailService.originalDocument(id: "abc"))

        // (c) the next sign-in: the first request of each kind carries nothing of the old account.
        let body = (try? JSONSerialization.data(withJSONObject: ["history": turns.snapshot().map { $0.text }])).map { String(decoding: $0, as: UTF8.self) } ?? ""
        XCTAssertFalse(body.contains(m), "turn")
        XCTAssertNil(EditionText.make(notebook: cache.text, topics: EditionSetupSession.load(topics)?.topics ?? []), "programme: edition_text")
        let sent = steerSent
        steer.setSender { t in sent.add(t.title); return .sent }
        await steer.flush(); XCTAssertEqual(sent.all, [], "steer flush")
        let newPoster = OfflinePoster(); newPoster.outcome = .answered([]); sheet.poster = newPoster
        sheet.flush(); await sheet.flushTask?.value
        XCTAssertTrue(newPoster.posted.isEmpty, "sheet ops: the old account's pending ops were dropped, not posted under the new key")
        let pushed = Recorder()
        pods.syncHooks = PodcastSyncHooks(signedIn: { true }, push: { s in pushed.add(s.title); return true }, pull: { [] })
        pods.syncIfDue()
        try await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertEqual(pushed.all, [], "podcast sync")
        await texture.retryPending(); XCTAssertTrue(api.posts.isEmpty, "texture retry")
        XCTAssertEqual(poster.posted.count, 1, "the old poster was not used again after the first post")
    }
}
