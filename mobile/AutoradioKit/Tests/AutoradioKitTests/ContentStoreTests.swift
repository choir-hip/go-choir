import XCTest
@testable import AutoradioKit

// Failure list: ContentStore.swift and ContentSync.swift headers. Fixtures use example.com only; titles and bodies carry a marker the logs must never show.
final class ContentStoreTests: XCTestCase {
    final class Box<T>: @unchecked Sendable { private let l = NSLock(); private var v: T; init(_ v: T) { self.v = v }
        var value: T { get { l.lock(); defer { l.unlock() }; return v } set { l.lock(); v = newValue; l.unlock() } } }
    final class Clock: @unchecked Sendable { private let b = Box(Date(timeIntervalSince1970: 1_794_000_000)); var now: Date { b.value }; func advance(_ s: Double) { b.value = b.value.addingTimeInterval(s) } }
    var dirs: [URL] = []
    override func tearDown() { for d in dirs { try? FileManager.default.removeItem(at: d) }; dirs = [] }
    func root() -> URL { let u = FileManager.default.temporaryDirectory.appendingPathComponent("cs-\(UUID().uuidString)"); dirs.append(u); return u }
    func item(_ id: String, _ kind: ContentKind, bytes: Int = 1000, days: Double = 0, clock: Clock, pinned: Bool = false) -> StoredItem {
        StoredItem(id: id, kind: kind, title: "T \(id)", text: String(repeating: "x", count: bytes), date: clock.now.addingTimeInterval(-days * 86400), address: id, version: "1", pinned: pinned, syncedAt: clock.now)
    }

    func testRoundTripSurvivesARestartWithCheckpoints() {
        let r = root(), c = Clock()
        let s = ContentStore(root: r, clock: { c.now })
        s.put(item("pub:/a", .story, clock: c)); s.setCheckpoint("k", "v"); s.markSynced(.story); s.flush()
        let again = ContentStore(root: r, clock: { c.now })
        XCTAssertEqual(again.get("pub:/a")?.title, "T pub:/a"); XCTAssertEqual(again.checkpoint("k"), "v"); XCTAssertNotNil(again.lastSynced(.story))
        XCTAssertEqual(again.summary().perKind[.story]?.count, 1)
    }
    func testACorruptManifestStartsEmptyWithoutACrash() throws {
        let r = root(), c = Clock()
        let s = ContentStore(root: r, clock: { c.now }); s.put(item("a", .mail, clock: c)); s.flush()
        try Data("{not json".utf8).write(to: r.appendingPathComponent("manifest.json"))
        XCTAssertEqual(ContentStore(root: r, clock: { c.now }).summary().totalBytes, 0)
    }
    func testEveryWriteIsProtectedAndTheRootIsKeptOutOfBackups() throws {
        let r = root(), c = Clock(); let seen = Box<[String]>([])
        let s = ContentStore(root: r, clock: { c.now }, protect: { seen.value.append($0.lastPathComponent) })
        s.put(item("m1", .mail, clock: c)); s.flush()
        XCTAssertTrue(seen.value.contains("mail"), "the kind's folder"); XCTAssertTrue(seen.value.contains("manifest.json")); XCTAssertTrue(seen.value.contains { $0.hasSuffix(".json") && $0 != "manifest.json" })
        XCTAssertTrue(ContentStore.writeOptions.contains(.completeFileProtectionUntilFirstUserAuthentication)); XCTAssertTrue(ContentStore.writeOptions.contains(.atomic))
        XCTAssertEqual(try r.resourceValues(forKeys: [.isExcludedFromBackupKey]).isExcludedFromBackup, true)
    }
    func testEvictionIsOldestFirstByKindAndNeverTouchesPinnedOrDocuments() {
        let r = root(), c = Clock()
        var b = ContentStore.Budget.standard; b.totalBytes = 5000; b.retentionDays = [:]; b.perKindBytes = [:]
        let s = ContentStore(root: r, budget: b, clock: { c.now })
        s.put(item("story-old", .story, bytes: 1000, days: 5, clock: c)); s.put(item("story-new", .story, bytes: 1000, days: 1, clock: c))
        s.put(item("file-1", .file, bytes: 1000, days: 9, clock: c)); s.put(item("mail-old", .mail, bytes: 1000, days: 100, clock: c))
        s.put(item("doc", .document, bytes: 1000, days: 400, clock: c)); s.put(item("draft", .mail, bytes: 1000, days: 400, clock: c, pinned: true))
        let dropped = s.enforceBudget()
        XCTAssertEqual(dropped, ["story-old", "story-new"].prefix(dropped.count).map { $0 }, "stories go first, the oldest of them first")
        XCTAssertTrue(s.has("doc")); XCTAssertTrue(s.has("draft"), "what the owner wrote is never evicted")
        XCTAssertLessThanOrEqual(s.summary().totalBytes, 5000 + 1)
        b.totalBytes = 100; s.budget = b; _ = s.enforceBudget()
        XCTAssertTrue(s.has("doc") && s.has("draft")); XCTAssertFalse(s.has("mail-old")); XCTAssertFalse(s.has("file-1"))
    }
    func testRetentionDropsOldStoriesEvenUnderBudget() {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        s.put(item("old", .story, days: 31, clock: c)); s.put(item("fresh", .story, days: 2, clock: c)); s.put(item("oldmail", .mail, days: 900, clock: c))
        _ = s.enforceBudget()
        XCTAssertFalse(s.has("old")); XCTAssertTrue(s.has("fresh")); XCTAssertTrue(s.has("oldmail"), "mail is kept until the budget needs the room")
    }
    func testTheTotalIsVisible() {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        s.put(item("a", .story, bytes: 2_000_000, clock: c)); s.put(item("b", .mail, bytes: 500_000, clock: c))
        XCTAssertTrue(s.summary().line.hasPrefix("3 MB of 400 MB"), s.summary().line)
    }
    func testSignOutWipesTheFilesTheMemoryAndTheIndex() throws {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        let corpus = SavedCorpus(readerStore: nil); corpus.setDeltaSource("store") { s.drain() }
        s.put(StoredItem(id: "mail:1", kind: .mail, title: "Tunnel referendum", text: "The referendum notes.", date: c.now, address: "mail:1", syncedAt: c.now)); s.flush()
        XCTAssertEqual(corpus.retrieve("tunnel referendum").items.count, 1)
        ContentSync.wipeAccount(store: s, corpus: corpus)
        XCTAssertEqual(s.summary().totalBytes, 0); XCTAssertNil(s.get("mail:1"))
        XCTAssertTrue(corpus.retrieve("tunnel referendum").isEmpty, "the index forgot it")
        let left = (FileManager.default.enumerator(at: r, includingPropertiesForKeys: nil)?.allObjects as? [URL] ?? []).filter { !$0.hasDirectoryPath }
        XCTAssertTrue(left.isEmpty, "no file of the account remains: \(left.map(\.lastPathComponent))")
        XCTAssertTrue(ContentStore(root: r, clock: { c.now }).summary().totalBytes == 0, "and a restart finds nothing")
    }
    func testTheIndexReadsFromTheStoreAndFollowsChanges() {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        let corpus = SavedCorpus(readerStore: nil); corpus.setDeltaSource("store") { s.drain() }
        s.put(StoredItem(id: "pub:/s", kind: .story, title: "Senate passes the water bill", text: "The Senate voted on Tuesday.", date: c.now, address: "pub:/s", source: "Autopaper", syncedAt: c.now))
        XCTAssertEqual(corpus.retrieve("tell me about the Senate").items.map(\.address), ["pub:/s"])
        s.remove("pub:/s"); XCTAssertTrue(corpus.retrieve("tell me about the Senate").isEmpty)
    }
    func testAPageHeldOnThePhoneRendersOfflineAndSaysWhenItWasSynced() {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        s.put(StoredItem(id: "pub:/s", kind: .story, title: "Water bill", text: "First paragraph.\nSecond paragraph.", date: c.now, address: "pub:/s", syncedAt: c.now)); s.markSynced(.story)
        c.advance(7200)
        let d = s.readerDoc(address: "pub:/s")
        XCTAssertEqual(d?.status, "ready"); XCTAssertEqual(d?.lines, ["First paragraph.", "Second paragraph."]); XCTAssertEqual(d?.meta, "Saved on this phone, last synced 2 hours ago")
        XCTAssertNil(s.readerDoc(address: "pub:/other"))
    }
    func testTheReaderFallsBackToTheStoreWhenTheNetworkFails() async {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        s.put(StoredItem(id: "texture:d1", kind: .document, title: "My notes", text: "Tomatoes.", date: c.now, address: "texture:d1", syncedAt: c.now))
        let store = ReaderStore(limit: 5, fetcher: { _ in throw URLError(.notConnectedToInternet) })
        store.offline = { s.readerDoc(address: $0) }
        let doc = await store.doc("texture:d1")
        XCTAssertEqual(doc.status, "ready"); XCTAssertEqual(doc.title, "My notes")
    }
    /// The cutover file's D3: the old mail copy is deleted at the first open of the new code, nothing else is.
    func testTheOldMailCopyIsDeletedAtOpenAndNothingElseIs() {
        let r = root(), c = Clock()
        do {
            let s = ContentStore(root: r, clock: { c.now })
            s.put(StoredItem(id: "mail:m1", kind: .mail, title: "SECRET-SUBJECT", text: "SECRET-BODY", date: c.now, address: "mail:m1", syncedAt: c.now, meta: ["folder": "inbox"]))
            s.put(StoredItem(id: "mail:draft/d1", kind: .mail, title: "SECRET-DRAFT", text: "SECRET-BODY", date: c.now, address: "mail:draft/d1", pinned: true, syncedAt: c.now))
            s.put(StoredItem(id: "pub:story", kind: .story, title: "A story", text: "Story text.", date: c.now, address: "pub:story", syncedAt: c.now))
            s.setCheckpoint("mail.inbox.done", "1"); s.setCheckpoint("story.cursor", "x"); s.markSynced(.mail); s.markSynced(.story); s.flush()
            XCTAssertEqual(s.ids(.mail).count, 2)
        }
        let s2 = ContentStore(root: r, clock: { c.now })
        XCTAssertEqual(s2.ids(.mail).count, 0); XCTAssertNil(s2.get("mail:m1")); XCTAssertNil(s2.get("mail:draft/d1"))
        XCTAssertNil(s2.checkpoint("mail.inbox.done")); XCTAssertNil(s2.lastSynced(.mail))
        XCTAssertEqual(s2.get("pub:story")?.text, "Story text."); XCTAssertEqual(s2.checkpoint("story.cursor"), "x"); XCTAssertNotNil(s2.lastSynced(.story))
        let mailDir = r.appendingPathComponent("items/mail")
        XCTAssertFalse(FileManager.default.fileExists(atPath: mailDir.path), "the mail files are gone")
        let drained = s2.drain(); XCTAssertTrue(drained.removed.contains("mail:m1"), "the retrieval index is told the ids left")
        // Another open finds nothing to delete and writes nothing.
        XCTAssertEqual(ContentStore(root: r, clock: { c.now }).purgeMail(), 0)
    }
}

final class ContentSyncTests: XCTestCase {
    typealias Box = ContentStoreTests.Box
    var dirs: [URL] = []
    override func tearDown() { for d in dirs { try? FileManager.default.removeItem(at: d) }; dirs = [] }
    let clock = ContentStoreTests.Clock()
    func store(_ root: URL? = nil, log: (@Sendable (String) -> Void)? = nil) -> ContentStore {
        let r = root ?? FileManager.default.temporaryDirectory.appendingPathComponent("sy-\(UUID().uuidString)"); dirs.append(r)
        return ContentStore(root: r, clock: { [clock] in clock.now }, log: log)
    }
    func sync(_ s: ContentStore, _ f: ContentSync.Fetchers, log: @escaping @Sendable (String) -> Void = { _ in }) -> ContentSync { ContentSync(store: s, fetch: f, clock: { [clock] in clock.now }, log: log) }

    func testDocumentsAreFetchedWhenTheirVersionChangesAndKeptPinned() async {
        let s = store(); let bodies = Box(0); let ver = Box("v1")
        var f = ContentSync.Fetchers()
        f.documentPage = { c in c == nil ? ([.init(id: "d1", title: "Notes", updated: ver.value)], "n") : ([.init(id: "d2", title: "Plans", updated: "v1")], nil) }
        f.documentBody = { id in bodies.value += 1; return "Markdown of \(id)" }
        await sync(s, f).syncAll(); XCTAssertEqual(bodies.value, 2)
        await sync(s, f).syncAll(); XCTAssertEqual(bodies.value, 2, "unchanged: nothing fetched")
        ver.value = "v2"; await sync(s, f).syncAll(); XCTAssertEqual(bodies.value, 3)
        XCTAssertEqual(s.meta("texture:d1")?.pinned, true)
    }
    func testStoriesNewestFirstHeldSkippedOldOnesNotFetchedAFailureSkipped() async {
        let s = store(); let calls = Box<[String]>([]); var f = ContentSync.Fetchers()
        let now = clock.now
        f.stories = { [.init(route: "/old", title: "Old", date: now.addingTimeInterval(-40 * 86400)), .init(route: "/a", title: "A", date: now.addingTimeInterval(-3600)),
                       .init(route: "/b", title: "B", date: now.addingTimeInterval(-60)), .init(route: "/bad", title: "Bad", date: now.addingTimeInterval(-120))] }
        f.storyBody = { r in calls.value.append(r); if r == "/bad" { throw URLError(.badServerResponse) }; return "Body \(r)" }
        let rep = await sync(s, f).syncAll()
        XCTAssertEqual(calls.value, ["/b", "/bad", "/a"]); XCTAssertEqual(rep.first { $0.kind == .story }?.failed, 1)
        calls.value = []; await sync(s, f).syncAll(); XCTAssertEqual(calls.value, ["/bad"], "held stories are not asked again")
    }
    func testTheRunSheetHistoryKeepsOnlyWhatWasHeard() async {
        let s = store(); var f = ContentSync.Fetchers()
        var sheet = RunSheet()
        sheet.entries = [RunSheetEntry(id: "h1", kind: .story, placedBy: .listener, state: .heard, title: "Heard story", text: "Words."), RunSheetEntry(id: "a1", kind: .story, placedBy: .listener, state: .ahead, title: "Later", text: "x")]
        f.runSheet = { sheet }
        await sync(s, f).syncAll()
        XCTAssertEqual(s.ids(.runsheet), ["run:h1"], "only what was heard, with its outcome"); XCTAssertEqual(s.get("run:h1")?.meta["outcome"], "heard")
    }
    func testTwoRunsAtOnceAreOneRun() async {
        let s = store(); var f = ContentSync.Fetchers()
        f.stories = { try await Task.sleep(nanoseconds: 150_000_000); return [] }
        let sy = sync(s, f)
        async let a = sy.syncAll()
        try? await Task.sleep(nanoseconds: 30_000_000)
        let b = await sy.syncAll(); let aa = await a
        XCTAssertTrue(b.isEmpty); XCTAssertFalse(aa.isEmpty)
    }
}

final class ContentStoreBootTests: XCTestCase {
    func brokenRoot() -> URL {            // a folder that cannot be made: its parent is a regular file
        let f = FileManager.default.temporaryDirectory.appendingPathComponent("blocker-\(UUID().uuidString)")
        try? Data("x".utf8).write(to: f); return f.appendingPathComponent("content")
    }
    func testAStoreThatCannotOpenDoesNothingAndLogsOnlyACode() {
        let logs = ContentStoreTests.Box<[String]>([]); let root = brokenRoot()
        let s = ContentStore(root: root, log: { logs.value.append($0) })
        XCTAssertFalse(s.usable)
        s.put(StoredItem(id: "mail:1", kind: .mail, title: "SECRET-SUBJECT", text: "SECRET-BODY", address: "mail:1")); s.flush()
        XCTAssertNil(s.get("mail:1")); XCTAssertEqual(s.summary().totalBytes, 0); XCTAssertTrue(s.drain().up.isEmpty)
        XCTAssertEqual(logs.value.count, 1); XCTAssertTrue(logs.value[0].hasPrefix("content: store open failed ("), logs.value[0])
        XCTAssertFalse(logs.value[0].contains("SECRET")); XCTAssertFalse(logs.value[0].contains("blocker"), "no path in the log")
        let said = ContentStoreTests.Box<[String]>([])
        XCTAssertFalse(ContentBoot.start(store: s, corpus: SavedCorpus(readerStore: nil), say: { said.value.append($0) }))
        XCTAssertTrue(said.value[0].hasPrefix("AUTORADIO store open FAILED "), said.value[0])
        // The retrieval path still answers, from what else the phone holds.
        let corpus = SavedCorpus(readerStore: nil); corpus.setSource("t") { [SavedCorpus.Document(id: "a", title: "Senate vote", address: "pub:a", text: "The Senate voted.")] }
        XCTAssertEqual(corpus.retrieve("senate vote").items.count, 1)
    }
    func testAnEmptyStoreOpensAndSaysSo() {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("boot-\(UUID().uuidString)"); defer { try? FileManager.default.removeItem(at: root) }
        let s = ContentStore(root: root); let said = ContentStoreTests.Box<[String]>([])
        XCTAssertTrue(ContentBoot.start(store: s, corpus: SavedCorpus(readerStore: nil), say: { said.value.append($0) }))
        XCTAssertEqual(said.value, ["AUTORADIO store open ok items=0"])
    }
    func testASlowOrBrokenStoreNeverHoldsUpTheReader() async {
        let store = ReaderStore(limit: 5, fetcher: { a in ReaderDoc(address: a, status: "ready", title: "From the network") })
        store.offline = { _ in sleep(0); return nil }
        let d = await store.doc("pub:/p/x")
        XCTAssertEqual(d.title, "From the network")
    }
}
