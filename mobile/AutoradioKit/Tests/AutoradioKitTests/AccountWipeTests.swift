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

    /// The default names the files earlier builds left, in the app's own folder (the takes and deck tests pass their own stores and no legacy files, so they delete none of a developer's).
    func testTheDefaultLegacyFilesAreTheNamesEarlierBuildsUsed() {
        XCTAssertEqual(TakeFiles.legacy.compactMap { $0?.lastPathComponent }, ["takes-own.json", "takes-muted.json", "takes-copy-own.json", "takes-copy-muted.json"])
        XCTAssertTrue(TakeFiles.legacy.allSatisfy { $0?.deletingLastPathComponent().lastPathComponent == "Autoradio" })
    }

    /// The Swift run sheet, steer queue, written-ahead opening and front titles are gone from this build (shared code keeps its own under app:/data): an upgraded phone's copies hold the old account's text,
    /// so boot removes them once and sign-out removes them again. Only temp folders here, never the real Application Support.
    func testTheFourOldBuildFilesGoAtBootAndAtSignOut() async {
        let support = root.appendingPathComponent("Application Support/Autoradio"), caches = root.appendingPathComponent("Library/Caches")
        func fill() {
            write("Application Support/Autoradio/runsheet.json", Self.marker); write("Application Support/Autoradio/steer-queue.json", Self.marker)
            write("Application Support/Autoradio/front-titles.json", Self.marker); write("Library/Caches/AutoradioOpening/opening-1.pcm", Self.marker)
            write("Application Support/Autoradio/local-entries.json", Self.marker)      // the held rest of a cut passage (HeldRest, deleted 2026-11-23: nothing ever wrote it)
            write("Application Support/Autoradio/keep.json", "other")
        }
        fill(); XCTAssertFalse(markerInFiles().isEmpty)
        AccountWipe.removeOldBuildFiles(support: support, caches: caches)
        XCTAssertEqual(markerInFiles(), [], "boot removes the old build's four stores")
        XCTAssertTrue(FileManager.default.fileExists(atPath: support.appendingPathComponent("keep.json").path), "and nothing else")
        AccountWipe.removeOldBuildFiles(support: support, caches: caches)      // nothing there: no error
        fill()
        let wipe = AccountWipe()
        wipe.registerStandard(legacyTakeFiles: [],
                              tempDirectory: root.appendingPathComponent("tmp3"),
                              deck: Deck(journal: DeckJournal(storage: root.appendingPathComponent("deck-journal3.json"))), oldBuildFiles: AccountWipe.oldBuildFiles(support: support, caches: caches))
        XCTAssertTrue(wipe.names.contains("old-build-files"))
        await wipe.run()
        XCTAssertEqual(markerInFiles(), [], "sign-out removes them too")
    }

    /// Row 1 of the takes state table (docs/reference/AUTORADIO-TAKES.md): a take in review is the account's; after sign-out (or an account change) the recorder is idle, its file is gone and the radio is
    /// given back, so the next account is never shown it and can never publish it.
    func testATakeInReviewIsGoneAfterTheWipe() async {
        let rec = FakeRecorder(), gate = FakeGate(), clock = Date(timeIntervalSince1970: 1000)
        let dir = root.appendingPathComponent("takes-live")
        let s = TakeSession(recorder: rec, transcriber: FakeTranscriber(words: Self.marker), gate: gate, preview: FakePreview(), micAllowed: { true }, responds: { TakeResponds(route: "r", title: Self.marker) },
                            dir: dir, now: { clock })
        TakeSession.live = s
        let limits = TakeLimits(minSeconds: 0, maxSeconds: 59.5, maxBytes: 512 * 1024)
        await s.start(limits); await s.stop(); XCTAssertEqual(s.state, .review); XCTAssertEqual(s.transcript, Self.marker)
        let oldOwn = root.appendingPathComponent("takes-own.json"), oldMuted = root.appendingPathComponent("takes-muted.json")
        write("takes-own.json", Self.marker); write("takes-muted.json", "[\"\(Self.marker)\"]")
        let wipe = AccountWipe(); wipe.registerStandard(legacyTakeFiles: [oldOwn, oldMuted], tempDirectory: root.appendingPathComponent("tmp2"))
        await wipe.run()
        XCTAssertEqual(s.state, .idle); XCTAssertEqual(s.transcript, ""); XCTAssertNil(s.audio); XCTAssertEqual(gate.given, 1)
        XCTAssertEqual(s.snapshot["respondsTitle"] as? String, "", "the story the old account answered is not carried either")
        XCTAssertEqual((try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? [], [])
        XCTAssertFalse(FileManager.default.fileExists(atPath: oldOwn.path)); XCTAssertFalse(FileManager.default.fileExists(atPath: oldMuted.path), "the old Swift files go with the account that is leaving")
        TakeSession.live = nil
    }

    func testEverythingOfTheAccountGoesAndTheNextSignInSendsNone() async throws {
        let m = Self.marker
        // The stores, each in its own place under the temporary root.
        let turns = TurnHistory(); turns.add(listener: m, radio: [m])
        write("notebook-cache.md", m)      // old Swift files: removed with the account (AccountWipe.oldBuildFiles)
        write("edition-topics.json", "{\"topics\":[\"\(m)\"]}"); write("notebook-topics-written", m)
        let oldTakeFiles = ["takes-own.json", "takes-muted.json", "takes-copy-own.json", "takes-copy-muted.json"].map { root.appendingPathComponent($0) }
        for f in oldTakeFiles { write(f.lastPathComponent, m) }
        let tmp = root.appendingPathComponent("tmp"); write("tmp/takes/rec.m4a", m); write("tmp/take-play-\(m).m4a", m)
        let clips = ClipLocator(directory: root.appendingPathComponent("clips")); write("clips/window-1.mp3", m)
        write("texture-pending.json", "{\"d1\":{\"text\":\"\(m)\"}}")      // the unsent text the Swift editor kept: shared code keeps its own now (app:/data/texture-pending.json), and this file is only removed
        let env = ChoirEnvironment(); env.setPlaying(PlayingProgramme(programmeID: "p1", title: m, route: m, publicationVersionID: "v", manifestHash: "h", blocks: [m]))

        // What the second review found: the spoken words and the files of the app target.
        let conversation = Conversation(); conversation.begin(id: "q1", question: m)
        let acks = AckHistory(); acks.add(m, page: m)
        SharedGeneration.resetForTest(); let reader = HeldPages(limit: 4); reader.hold(gen: 0, HeldPage(address: "pub:x", title: m, blocks: [HeldBlock(id: "b1", kind: "p", text: m)]))
        VoiceFront.shared.set(gen: 0, [["route": "front-story", "title": m]])      // the paper's stories handed to the voice
        ReaderFollow.shared.set(address: "pub:x", spoken: m, held: reader); ReaderAim.shared.set(ReaderAimInfo(address: "pub:x", blockId: "b1", label: m))      // the old account's place in a page, and what its prompt was aimed at
        let kept = KeptTurnStore(root: root.appendingPathComponent("turns")); write("turns/t1/record.json", m)
        write("MikeVoice/speaker.json", m); let voice = SpeakerGate(url: root.appendingPathComponent("MikeVoice/speaker.json"))
        let openPages = OpenPageFacts(); openPages.replace(gen: 1, [FrontPage(address: "mailbox:inbox", title: m, kind: "mailbox")])
        SharedGeneration.resetForTest(); EditionPicksMirror.shared.set(gen: 1, [m])        // the native copy of the picks (the file is shared code's)
        write("onboarding.json", m); write("books/b1/text.txt", m); write("cache/files/f1.txt", m)

        let wipe = AccountWipe()
        wipe.register("open-pages") { openPages.wipe() }
        let rootRef = root!
        wipe.register("app-files") {          // as AutoradioModule registers them
            AccountWipe.removeFiles([rootRef.appendingPathComponent("onboarding.json")])
            AccountWipe.removeContents(of: rootRef.appendingPathComponent("books")); AccountWipe.removeContents(of: rootRef.appendingPathComponent("cache/files"))
        }
        wipe.registerStandard(turnHistory: turns, 
                              legacyTakeFiles: oldTakeFiles, tempDirectory: tmp, clips: clips,
                              environment: env,
                              conversation: conversation, acks: acks, reader: reader, keptTurns: kept, voice: voice,
                              deck: Deck(journal: DeckJournal(storage: root.appendingPathComponent("deck-journal.json"))),
                              oldBuildFiles: AccountWipe.oldBuildFiles(support: root, caches: nil))
        XCTAssertFalse(markerInFiles().isEmpty, "the test filled the files")

        await wipe.run()

        // (a) no file holds the marker
        XCTAssertEqual(markerInFiles(), [], "files still holding the account's words")
        // (b) every in-memory store answers empty
        XCTAssertTrue(turns.snapshot().isEmpty)
        XCTAssertNil(env.playing("p1"))
        XCTAssertTrue(conversation.entries().isEmpty); XCTAssertTrue(acks.recent.isEmpty)
        XCTAssertTrue(kept.list().isEmpty); XCTAssertFalse(voice.info().learned)
        XCTAssertTrue(openPages.list().isEmpty); XCTAssertTrue(EditionPicksMirror.shared.topics.isEmpty, "the native copy of the picks"); XCTAssertFalse(EditionPicksMirror.shared.known, "and it is forgotten"); XCTAssertEqual(reader.count, 0, "the pages the reader handed to the voice"); XCTAssertTrue(VoiceFront.shared.candidates.isEmpty, "the stories handed to the voice"); XCTAssertEqual(ReaderFollow.shared.info, ReaderFollowInfo(), "the sentence followed"); XCTAssertEqual(ReaderAim.shared.info, ReaderAimInfo(), "what the prompt was aimed at")

        // (c) the next sign-in: the first request of each kind carries nothing of the old account.
        let body = (try? JSONSerialization.data(withJSONObject: ["history": turns.snapshot().map { $0.text }])).map { String(decoding: $0, as: UTF8.self) } ?? ""
        XCTAssertFalse(body.contains(m), "turn")
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





    // testFeedLoadAfterTheWipeIsNotKept moved to AutoradioLab/podcast.test.ts ("a feed that finishes loading after the wipe is not kept").
}
