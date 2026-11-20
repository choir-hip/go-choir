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

    /// Row 1 of the takes state table (docs/reference/AUTORADIO-TAKES.md): a take in review is the account's; after sign-out (or an account change) the recorder is idle, its file is gone and the radio is
    /// given back, so the next account is never shown it and can never publish it.
    func testATakeInReviewIsGoneAfterTheWipeAndTheMarkerOfTheUpgradeIsNot() async {
        let rec = FakeRecorder(), gate = FakeGate(), clock = Date(timeIntervalSince1970: 1000)
        let dir = root.appendingPathComponent("takes-live")
        let s = TakeSession(recorder: rec, transcriber: FakeTranscriber(words: Self.marker), gate: gate, preview: FakePreview(), micAllowed: { true }, responds: { TakeResponds(route: "r", title: Self.marker) },
                            dir: dir, now: { clock })
        TakeSession.live = s
        let limits = TakeLimits(minSeconds: 0, maxSeconds: 59.5, maxBytes: 512 * 1024)
        await s.start(limits); await s.stop(); XCTAssertEqual(s.state, .review); XCTAssertEqual(s.transcript, Self.marker)
        let carried = root.appendingPathComponent("takes-carried"); XCTAssertTrue(TakeLegacy.markCarried(marker: carried))
        let wipe = AccountWipe(); wipe.registerStandard(takeOwn: TakeOwn(), takeMutes: TakeMutes(), takeStore: TakeStore(), tempDirectory: root.appendingPathComponent("tmp2"))
        await wipe.run()
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.transcript, ""); XCTAssertNil(s.audio); XCTAssertEqual(gate.given, 1)
        XCTAssertEqual(s.snapshot["respondsTitle"] as? String, "", "the story the old account answered is not carried either")
        XCTAssertEqual((try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? [], [])
        XCTAssertTrue(FileManager.default.fileExists(atPath: carried.path), "the upgrade's marker is the phone's, not the account's: the wipe leaves it")
        TakeSession.live = nil
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
        let own = TakeOwn(file: root.appendingPathComponent("takes-own.json")); own.replace(ids: [m], handle: m)
        let mutes = TakeMutes(file: root.appendingPathComponent("takes-muted.json")); mutes.replace([m])
        let takes = TakeStore(); takes.replace([Take.parse(["take_id": m, "publisher": "pub", "state": "live", "transcript": m])!])
        let tmp = root.appendingPathComponent("tmp"); write("tmp/takes/rec.m4a", m); write("tmp/take-play-\(m).m4a", m)
        let sheet = RunSheetStore(storage: root.appendingPathComponent("runsheet.json")); sheet.retryAfter = 3600
        let poster = OfflinePoster(); sheet.poster = poster
        sheet.place(.paperStory(route: m, title: m), at: .queued); await sheet.flushTask?.value
        XCTAssertFalse(sheet.pending.isEmpty, "the route said not yet: the op is pending")
        let opening = OpeningCache(dir: root.appendingPathComponent("opening"))
        opening.put(.opening, entry: .paperStory(route: m, title: m), titles: [m], audio: [PreparedAudio(text: m, samples: [0.1, 0.2], rate: 24000)], voice: "v")
        let clips = ClipLocator(directory: root.appendingPathComponent("clips")); write("clips/window-1.mp3", m)
        let api = FakeAPI()
        let texture = TextureEditor(api: api, storage: root.appendingPathComponent("texture-pending.json")); texture.retryAfter = 3600
        await texture.edit(address: "texture:d1", markdown: m); await texture.flush(address: "texture:d1")
        XCTAssertEqual(texture.pending["d1"], m)
        let env = ChoirEnvironment(); env.setPlaying(PlayingProgramme(programmeID: "p1", title: m, route: m, publicationVersionID: "v", manifestHash: "h", blocks: [m]))
        let content = ContentStore(root: root.appendingPathComponent("content"))
        content.put(StoredItem(id: "mail:1", kind: .mail, title: m, text: m, address: "mail:1")); content.flush()

        // What the second review found: the spoken words and the files of the app target.
        let mailReading = MailReading(); mailReading.request(address: m, text: m)
        let conversation = Conversation(); conversation.begin(id: "q1", question: m)
        let acks = AckHistory(); acks.add(m, page: m)
        let reader = ReaderStore(limit: 4)
        let kept = KeptTurnStore(root: root.appendingPathComponent("turns")); write("turns/t1/record.json", m)
        write("MikeVoice/speaker.json", m); let voice = SpeakerGate(url: root.appendingPathComponent("MikeVoice/speaker.json"))
        let signals = FileSignals(file: root.appendingPathComponent("signals.jsonl")); signals.record(SignalEvent(kind: "ask", subject: m, detail: m))
        let openPages = OpenPages(file: root.appendingPathComponent("open-pages.json")); openPages.open("mailbox:inbox", title: m)
        let session = EditionSetupSession(suggestions: [], file: root.appendingPathComponent("edition-selection.json"))
        EditionSetupSession.shared = session; write("edition-selection.json", "{\"topics\":[\"\(m)\"]}")
        write("onboarding.json", m); write("books/b1/text.txt", m); write("cache/files/f1.txt", m)

        let wipe = AccountWipe()
        wipe.register("signals") { signals.wipe() }
        wipe.register("open-pages") { openPages.wipe() }
        let rootRef = root!
        wipe.register("app-files") {          // as AutoradioModule registers them
            AccountWipe.removeFiles([rootRef.appendingPathComponent("onboarding.json")])
            AccountWipe.removeContents(of: rootRef.appendingPathComponent("books")); AccountWipe.removeContents(of: rootRef.appendingPathComponent("cache/files"))
        }
        wipe.registerStandard(turnHistory: turns, notebookCache: cache, topicsFile: topics, topicsMarker: topicsMarker, notebook: notebook, steer: steer,
                              takeOwn: own, takeMutes: mutes, takeStore: takes, tempDirectory: tmp, runSheet: sheet, openingCache: opening, clips: clips,
                              texture: texture, environment: env, content: content, corpus: SavedCorpus(),
                              mailReading: mailReading, conversation: conversation, acks: acks, reader: reader, keptTurns: kept, voice: voice)
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
        XCTAssertNil(opening.opening(voice: "v"))
        XCTAssertTrue(texture.pending.isEmpty); XCTAssertNil(env.playing("p1")); XCTAssertNil(content.get("mail:1"))
        XCTAssertNil(mailReading.last); XCTAssertTrue(conversation.entries().isEmpty); XCTAssertTrue(acks.recent.isEmpty)
        XCTAssertTrue(kept.list().isEmpty); XCTAssertFalse(voice.info().learned); XCTAssertTrue(signals.recent(limit: 10).isEmpty)
        XCTAssertFalse(openPages.list().contains { $0.title == m }); XCTAssertNil(EditionSetupSession.shared)

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
        await texture.retryPending(); XCTAssertTrue(api.posts.isEmpty, "texture retry")
        XCTAssertEqual(poster.posted.count, 1, "the old poster was not used again after the first post")
    }
}


// Work in flight when the account is wiped: each fake suspends, the wipe runs, the fake is released, and nothing of the old account is written or held.
actor Latch {
    private var open = false
    private var waiters: [CheckedContinuation<Void, Never>] = []
    func wait() async { if open { return }; await withCheckedContinuation { waiters.append($0) } }
    func release() { open = true; for w in waiters { w.resume() }; waiters = [] }
    var isOpen: Bool { open }
}

@MainActor
final class AccountWipeInFlightTests: XCTestCase {
    static let marker = "MARKER-7731"
    var root: URL!
    override func setUp() { root = FileManager.default.temporaryDirectory.appendingPathComponent("acctflight-\(UUID().uuidString)"); try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true) }
    override func tearDown() { try? FileManager.default.removeItem(at: root) }

    func waitFor(_ l: Latch) async { for _ in 0..<200 { if await l.isOpen { return }; try? await Task.sleep(nanoseconds: 10_000_000) } }
    func filesHoldingMarker() -> [String] {
        var hits: [String] = []
        let en = FileManager.default.enumerator(at: root, includingPropertiesForKeys: [.isRegularFileKey])
        while let u = en?.nextObject() as? URL {
            if let d = try? Data(contentsOf: u), d.range(of: Data(Self.marker.utf8)) != nil { hits.append(u.lastPathComponent) }
        }
        return hits
    }

    func testDeskStreamAppliesNothingAfterTheWipe() async {
        let m = Self.marker
        final class Feed: RunSheetFeed, @unchecked Sendable {
            let entered: Latch, release: Latch, entry: RunSheetEntry
            init(_ e: Latch, _ r: Latch, _ entry: RunSheetEntry) { entered = e; release = r; self.entry = entry }
            func ask(_ a: RunSheetAsk) async throws -> AsyncThrowingStream<RunSheetFeedEvent, Error> {
                await entered.release(); await release.wait()
                let op = ProgrammeRouteFeed.placeOp(entry, cause: "open")
                return AsyncThrowingStream { c in c.yield(.ops([op])); c.yield(.done); c.finish() }
            }
        }
        let entered = Latch(), release = Latch()
        let store = RunSheetStore(storage: root.appendingPathComponent("runsheet.json"))
        let feed = Feed(entered, release, RunSheetEntry.paperStory(route: m, title: m, by: .desk))
        let w = RunSheetWriter(store: store) { feed }
        w.open()
        await waitFor(entered)
        let wipe = AccountWipe()
        wipe.register("run-sheet") { await MainActor.run { w.stop(); store.wipe() } }
        await wipe.run()
        await release.release(); await w.settle()
        XCTAssertTrue(store.sheet.entries.isEmpty, "the desk's entries of the old account were applied after the wipe")
        XCTAssertTrue(store.pending.isEmpty); XCTAssertEqual(filesHoldingMarker(), []); XCTAssertFalse(w.inFlight)
    }

    func testStaleDeskOpIsRefusedEvenWithoutStop() async {
        let store = RunSheetStore()
        let gen = AccountWipe.generation
        await AccountWipe().run()
        let op = ProgrammeRouteFeed.placeOp(RunSheetEntry.paperStory(route: "a", title: "A", by: .desk), cause: "open")
        XCTAssertFalse(store.applyFromDesk(op, generation: gen).applied)
        XCTAssertTrue(store.applyFromDesk(op, generation: AccountWipe.generation).applied)
    }

    // testFeedLoadAfterTheWipeIsNotKept moved to AutoradioLab/podcast.test.ts ("a feed that finishes loading after the wipe is not kept").

    func testOpeningWriteAheadDoesNotRePutTheOldOpening() async {
        let m = Self.marker
        final class Feed: RunSheetFeed, @unchecked Sendable {
            let entered: Latch, release: Latch, entry: RunSheetEntry
            init(_ e: Latch, _ r: Latch, _ entry: RunSheetEntry) { entered = e; release = r; self.entry = entry }
            func ask(_ a: RunSheetAsk) async throws -> AsyncThrowingStream<RunSheetFeedEvent, Error> {
                await entered.release(); await release.wait()
                let op = ProgrammeRouteFeed.placeOp(entry, cause: "open")
                return AsyncThrowingStream { c in c.yield(.ops([op])); c.finish() }
            }
        }
        let entered = Latch(), release = Latch()
        let cache = OpeningCache(dir: root.appendingPathComponent("opening"))
        let store = RunSheetStore()
        let opening = RunSheetEntry(id: "o1", kind: .opening, placedBy: .desk, title: "Opening", text: "Good morning \(m).", estSeconds: 3)
        let feed = Feed(entered, release, opening)
        let wa = OpeningWriteAhead(cache: cache, store: store, feed: { feed }, voice: { "v" }, synth: { PreparedAudio(text: $0, samples: [0.1], rate: 24000) })
        let run = Task { @MainActor in await wa.refresh(reason: "launch") }
        await waitFor(entered)
        let wipe = AccountWipe(); wipe.register("opening") { cache.wipe() }
        await wipe.run()
        await release.release(); await run.value
        XCTAssertNil(cache.opening(voice: "v")); XCTAssertEqual(filesHoldingMarker(), [])
    }

    func testNotebookWriteAfterTheWipeKeepsNothing() async {
        let m = Self.marker
        let saved = ChoirEnvironment.shared.keys; ChoirEnvironment.shared.keys = InMemoryKeyStore([KeyAccount.apiKey: "k"]); defer { ChoirEnvironment.shared.keys = saved }
        let entered = Latch(), release = Latch()
        let s = NotebookStore(); let cache = NotebookCache(file: root.appendingPathComponent("notebook-cache.md")); s.notebookCache = cache
        s.lookupEdition = { await entered.release(); await release.wait(); return EditionInfo(found: false) }
        s.createNotebook = { "d1" }
        let marker = root.appendingPathComponent("notebook-topics-written")
        let run = Task { @MainActor in await s.writeTopics([m], marker: marker) }
        await waitFor(entered)
        let wipe = AccountWipe(); wipe.register("notebook") { await MainActor.run { cache.wipe(); s.wipe() } }
        await wipe.run()
        await release.release()
        let ok = await run.value
        XCTAssertFalse(ok); XCTAssertNil(cache.text); XCTAssertFalse(FileManager.default.fileExists(atPath: marker.path)); XCTAssertEqual(filesHoldingMarker(), [])
    }
}
