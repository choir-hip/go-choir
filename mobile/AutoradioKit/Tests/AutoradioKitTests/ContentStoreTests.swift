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
    func testMailBoxAndMessageRenderOfflineFromTheStore() async {
        let r = root(), c = Clock(); let s = ContentStore(root: r, clock: { c.now })
        var row = MailSummary(); row.id = "m1"; row.subject = "Tunnel vote"; row.fromDisplay = "Ana Example"; row.fromAddress = "ana@example.com"; row.at = "2026-11-14T09:00:00Z"; row.snippet = "Notes."
        s.put(ContentStore.mailItem(row, text: "Full body about the tunnel.", folder: "inbox", syncedAt: c.now, to: ["me@example.com"])); s.markSynced(.mail)
        let svc = MailService()
        svc.offlineRows = { s.mailSummaries(folder: $0) }
        svc.offlineDetail = { id in s.get("mail:" + id).map { var d = MailDetail(); d.summary = ContentStore.summary($0); d.text = $0.text; return d } }
        svc.offlineNote = { s.syncedWords(.mail) ?? "never synced" }
        let box = await svc.box(address: "mailbox:inbox")
        XCTAssertEqual(box.rows.count, 1, "status \(box.status) \(box.message)"); XCTAssertTrue(box.message.contains("saved on this phone"), box.message)
        let msg = await svc.message(address: "mail:m1")
        XCTAssertEqual(msg.status, "ready"); XCTAssertEqual(msg.subject, "Tunnel vote"); XCTAssertTrue(msg.message.contains("last synced"))
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
    func row(_ i: Int) -> MailSummary { var m = MailSummary(); m.id = "m\(i)"; m.subject = "SECRET-SUBJECT \(i)"; m.fromDisplay = "SECRET-SENDER"; m.snippet = "snip"; m.at = "2026-11-1\(i % 9)T09:00:00Z"; return m }
    func sync(_ s: ContentStore, _ f: ContentSync.Fetchers, log: @escaping @Sendable (String) -> Void = { _ in }) -> ContentSync { ContentSync(store: s, fetch: f, clock: { [clock] in clock.now }, log: log) }

    func mailFetchers(pages: [[Int]], calls: Box<[String]>) -> ContentSync.Fetchers {
        var f = ContentSync.Fetchers(); f.mailFolders = ["inbox"]
        f.mailPage = { _, cursor in
            let n = cursor.flatMap(Int.init) ?? 0; calls.value.append("page\(n)")
            return (pages[n].map(self.row), n + 1 < pages.count ? String(n + 1) : nil)
        }
        f.mailBody = { id in calls.value.append("body:" + id); var d = MailDetail(); d.summary.id = id; d.text = "SECRET-BODY of \(id)"; return d }
        return f
    }

    func testMailWalksAllPagesAndKeepsFullBodiesLogsHoldNoContent() async {
        let calls = Box<[String]>([]), logs = Box<[String]>([]); let s = store { logs.value.append($0) }
        let sy = sync(s, mailFetchers(pages: [[1, 2], [3, 4], [5]], calls: calls)) { logs.value.append($0) }
        await sy.syncAll()
        XCTAssertEqual(s.ids(.mail).count, 5); XCTAssertEqual(s.get("mail:m3")?.text, "SECRET-BODY of m3")
        let all = logs.value.joined(separator: "\n")
        XCTAssertTrue(all.contains("content sync mail: fetched 5")); XCTAssertFalse(all.contains("SECRET"), "counts and ids only")
    }
    func testAKilledSyncResumesWithoutFetchingHeldBodiesAgain() async {
        let calls = Box<[String]>([]); let s = store()
        let busy = Box(0)
        let sy = sync(s, mailFetchers(pages: [[1, 2, 3], [4, 5, 6]], calls: calls))
        sy.audioBusy = { busy.value += 1; return busy.value > 4 }        // the radio needs the network after a few requests
        let r1 = await sy.syncAll()
        XCTAssertEqual(r1.first { $0.kind == .mail }?.stoppedBy, "radio")
        let held = s.ids(.mail).filter { s.get($0)?.version == "body" }.count
        XCTAssertGreaterThan(held, 0); XCTAssertLessThan(held, 6)
        s.flush()
        // The app is killed and started again: a new store over the same files, a new syncer, the radio idle.
        let s2 = ContentStore(root: s.root, clock: { [clock] in clock.now }); let calls2 = Box<[String]>([])
        let sy2 = sync(s2, mailFetchers(pages: [[1, 2, 3], [4, 5, 6]], calls: calls2))
        await sy2.syncAll()
        XCTAssertEqual(s2.ids(.mail).count, 6)
        XCTAssertEqual(calls2.value.filter { $0.hasPrefix("body:") }.count, 6 - held, "only the missing bodies are fetched")
    }
    func testAWalkedFolderStopsAtTheFirstPageThatIsAllHeld() async {
        let calls = Box<[String]>([]); let s = store()
        await sync(s, mailFetchers(pages: [[1, 2], [3, 4], [5, 6]], calls: calls)).syncAll()
        calls.value = []
        await sync(s, mailFetchers(pages: [[7, 1, 2], [3, 4], [5, 6]], calls: calls)).syncAll()
        XCTAssertEqual(calls.value.filter { $0.hasPrefix("page") }, ["page0", "page1"], "page 0 had news; page 1 was all held and ended the walk")
        XCTAssertEqual(calls.value.filter { $0.hasPrefix("body") }, ["body:m7"])
        XCTAssertEqual(s.ids(.mail).count, 7)
    }
    func testAServerThatIgnoresTheCursorDoesNotLoopForever() async {
        let s = store(); var f = ContentSync.Fetchers(); f.mailFolders = ["inbox"]
        f.mailPage = { _, _ in ([self.row(1)], "same") }; f.mailBody = { _ in var d = MailDetail(); d.text = "t"; return d }
        let r = await sync(s, f).syncAll()
        XCTAssertEqual(r.first { $0.kind == .mail }?.fetched, 1)
    }
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
    func testFilesTakeOnlySmallTextAndFollowSizeAndModified() async {
        let s = store(); let reads = Box<[String]>([]); var f = ContentSync.Fetchers()
        let when = Date(timeIntervalSince1970: 100)
        f.fileList = { p in p == "/" ? [FileEntry(name: "notes.md", path: "/notes.md", isDirectory: false, size: 10, modified: when), FileEntry(name: "pic.png", path: "/pic.png", isDirectory: false, size: 10, modified: when),
                                         FileEntry(name: "huge.txt", path: "/huge.txt", isDirectory: false, size: 9_000_000, modified: when), FileEntry(name: "sub", path: "/sub", isDirectory: true, size: 0, modified: nil)]
                                      : [FileEntry(name: "deep.txt", path: "/sub/deep.txt", isDirectory: false, size: 5, modified: when)] }
        f.fileText = { reads.value.append($0); return "text of \($0)" }
        await sync(s, f).syncAll()
        XCTAssertEqual(Set(reads.value), ["/notes.md", "/sub/deep.txt"]); await sync(s, f).syncAll(); XCTAssertEqual(reads.value.count, 2)
    }
    func testTranscriptsAreLimitedToRecentEpisodesAndTheRunSheetHistoryIsKept() async {
        let s = store(); var f = ContentSync.Fetchers(); let asked = Box<[String]>([]); let now = clock.now
        f.transcriptTargets = { (0..<30).map { (feed: "https://example.com/f.xml", id: "e\($0)", title: "Show: E\($0)", date: now.addingTimeInterval(Double($0 == 29 ? -30 * 86400 : -3600 * Double($0)))) } }
        f.transcript = { _, id in asked.value.append(id); return PodcastTranscript(durationSeconds: 60, source: "x", blocks: [.init(start: 0, end: 5, text: "We talked about \(id).")]) }
        var sheet = RunSheet()
        sheet.entries = [RunSheetEntry(id: "h1", kind: .story, placedBy: .listener, state: .heard, title: "Heard story", text: "Words."), RunSheetEntry(id: "a1", kind: .story, placedBy: .listener, state: .ahead, title: "Later", text: "x")]
        f.runSheet = { sheet }
        await sync(s, f).syncAll()
        XCTAssertEqual(asked.value.count, ContentSync.transcriptsPerRun); XCTAssertFalse(asked.value.contains("e29"))
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
