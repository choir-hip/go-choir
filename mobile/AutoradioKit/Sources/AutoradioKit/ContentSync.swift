import Foundation

// Fills the ContentStore (ContentStore.swift) in the background: every kind, newest first, incremental, resumable, paused while the radio needs
// the network, on any network (the app has no Wi-Fi-only rule for text; a model download asks first because it is gigabytes, this is megabytes).
//
// PER KIND (list route / body route / bulk or since-cursor route? / rule):
//  story     GET /api/platform/publications/front?limit=100 (title, dek, updated_at, words) / GET .../publications/resolve?route= / no bulk body route,
//            no since: the list is the newest 100; one resolve per story (WOULD NEED: GET .../publications/front?include=text&since=<iso>) / newest
//            first, a story held with the same updated_at is not fetched again; retention 30 days.
//  mail      moved to shared code on 2026-11-19 (AutoradioLab/mail.ts keeps the phone's copy of the listener's mail); nothing here fetches it.
//  document  GET /api/texture/documents?limit=500&cursor= (id, title, updated_at) / GET /api/texture/documents/{id} then /revisions/{rid} (the
//            Markdown) / cursor paging, no since, no bodies (WOULD NEED: GET /api/texture/documents?include=content&updated_since=) / all pages; a
//            document is fetched when its updated_at differs from the held version; never evicted.
//  file      moved to shared code: the Files page (AutoradioLab/files.ts) reads the server's folders itself and nothing here fetches them.
//  podcast   moved to shared code (AutoradioLab/podcast.ts); nothing here fetches shows, notes or transcripts.
//  runsheet  local: heard and skipped entries with their outcome are copied from the sheet when it changes (no request); retention 90 days.
//
// WAYS THIS CAN FAIL (tests in ContentSyncTests, fake fetchers and a fake clock):
//  1. Killed mid-sync: the next run skips held items and continues.  2. A failing item (404, bad body): skipped, counted, the run goes on; it is
//  not retried in the same run.  3. Radio busy: the run stops before the next request.  4. Not signed in / offline: nothing is changed.  5. Two runs
//  at once: the second returns at once.  6. A cancelled task stops at the next item.  7. Logs carry counts and ids, never titles or text.
//  8. A list page that repeats (a server that ignores the cursor): the walk stops when a cursor repeats.

public struct SyncReport: Equatable, Sendable {
    public var kind: ContentKind
    public var fetched = 0, skipped = 0, failed = 0
    public var stoppedBy = ""          // "", "radio", "cancelled", "error"
}

public final class ContentSync: @unchecked Sendable {
    public struct Fetchers: Sendable {
        public var documentPage: @Sendable (_ cursor: String?) async throws -> (docs: [DocRow], next: String?) = { _ in ([], nil) }
        public var documentBody: @Sendable (_ id: String) async throws -> String? = { _ in nil }
        public var stories: @Sendable () async throws -> [StoryBodies.Story] = { [] }
        public var storyBody: @Sendable (_ route: String) async throws -> String? = { _ in nil }
        public var runSheet: @Sendable () async -> RunSheet? = { nil }
        public init() {}
    }
    public struct DocRow: Sendable, Equatable {
        public var id: String, title: String, updated: String
        public init(id: String, title: String, updated: String) { self.id = id; self.title = title; self.updated = updated }
    }

    let store: ContentStore
    public var fetch: Fetchers
    public var audioBusy: @Sendable () async -> Bool = { false }
    private let clock: @Sendable () -> Date
    private let log: @Sendable (String) -> Void
    private let lock = NSLock()
    private var running = false

    public init(store: ContentStore, fetch: Fetchers, clock: @escaping @Sendable () -> Date = { Date() }, log: @escaping @Sendable (String) -> Void = { _ in }) {
        self.store = store; self.fetch = fetch; self.clock = clock; self.log = log
    }

    /// Every kind, in the order the listener feels them: stories, mail, documents, then the slow ones. Returns one report per kind.
    @discardableResult
    public func syncAll() async -> [SyncReport] {
        lock.lock(); if running { lock.unlock(); return [] }; running = true; startedUnder = AccountWipe.generation; lock.unlock()
        defer { lock.lock(); running = false; startedUnder = nil; lock.unlock() }
        var out: [SyncReport] = []
        if let sheet = await fetch.runSheet() { recordRunSheet(sheet); writeMarkSynced(.runsheet) }
        for step in [syncStories, syncDocuments] as [() async -> SyncReport] {
            let r = await step()
            out.append(r)
            writeMarkSynced(r.kind); writeFlush()
            log("content sync \(r.kind.rawValue): fetched \(r.fetched), held \(r.skipped), failed \(r.failed)\(r.stoppedBy.isEmpty ? "" : ", stopped: " + r.stoppedBy)")
            if r.stoppedBy == "radio" || r.stoppedBy == "cancelled" { break }
        }
        if live { _ = store.enforceBudget() }
        return out
    }

    /// The account generation this run started under (nil outside `syncAll`). A write after the account was wiped is dropped.
    private var startedUnder: Int?
    private var live: Bool { lock.lock(); defer { lock.unlock() }; return startedUnder == nil || startedUnder == AccountWipe.generation }
    private func writePut(_ i: StoredItem) { if live { store.put(i) } }
    private func writeFlush() { if live { store.flush() } }
    private func writeMarkSynced(_ k: ContentKind) { if live { store.markSynced(k) } }
    private func writeSetCheckpoint(_ k: String, _ v: String) { if live { store.setCheckpoint(k, v) } }

    private func gate(_ r: inout SyncReport) async -> Bool {
        if !live { r.stoppedBy = "cancelled"; return false }
        if Task.isCancelled { r.stoppedBy = "cancelled"; return false }
        if await audioBusy() { r.stoppedBy = "radio"; return false }
        return true
    }

    // MARK: stories

    func syncStories() async -> SyncReport {
        var r = SyncReport(kind: .story)
        guard let list = try? await fetch.stories() else { r.stoppedBy = "error"; return r }
        let now = clock()
        for s in list.filter({ !$0.route.isEmpty }).sorted(by: { ($0.date ?? .distantPast) > ($1.date ?? .distantPast) }) {
            if let d = s.date, now.timeIntervalSince(d) > 30 * 86400 { continue }              // past retention: not fetched only to be dropped
            let version = s.date.map { String($0.timeIntervalSince1970) } ?? "0"
            let id = "pub:" + s.route
            if store.has(id, version: version) { r.skipped += 1; continue }
            guard await gate(&r) else { return r }
            do {
                guard let text = try await fetch.storyBody(s.route), !text.isEmpty else { r.failed += 1; continue }
                writePut(StoredItem(id: id, kind: .story, title: s.title, text: text, date: s.date, address: id, source: "Autopaper", version: version, syncedAt: now))
                r.fetched += 1
            } catch is CancellationError { r.stoppedBy = "cancelled"; return r } catch { r.failed += 1 }
        }
        return r
    }

    // MARK: documents

    func syncDocuments() async -> SyncReport {
        var r = SyncReport(kind: .document)
        var cursor: String? = nil, seen = Set<String>()
        while true {
            guard await gate(&r) else { return r }
            let page: (docs: [DocRow], next: String?)
            do { page = try await fetch.documentPage(cursor) } catch is CancellationError { r.stoppedBy = "cancelled"; return r } catch { r.failed += 1; r.stoppedBy = "error"; return r }
            for d in page.docs where !d.id.isEmpty {
                let id = "texture:" + d.id
                if store.has(id, version: d.updated) { r.skipped += 1; continue }
                guard await gate(&r) else { return r }
                do {
                    guard let text = try await fetch.documentBody(d.id) else { r.failed += 1; continue }
                    writePut(StoredItem(id: id, kind: .document, title: d.title.isEmpty ? "Untitled" : d.title, text: text, date: PaperStory.date(d.updated), address: id,
                                         source: "Texture", version: d.updated, pinned: true, syncedAt: clock()))
                    r.fetched += 1
                } catch is CancellationError { r.stoppedBy = "cancelled"; return r } catch { r.failed += 1 }
            }
            writeFlush()
            guard let next = page.next, !next.isEmpty, seen.insert(next).inserted else { break }
            cursor = next
        }
        return r
    }

    // MARK: the run sheet's history (local)

    /// Copy the heard and skipped entries, with how they ended, from a sheet snapshot. No request.
    public func recordRunSheet(_ sheet: RunSheet) {
        let now = clock()
        for e in sheet.entries where e.state == .heard || e.state == .skipped {
            let id = "run:" + e.id
            if store.has(id) { continue }
            let outcome = e.ended.map { "\($0)" } ?? (e.state == .skipped ? "skipped" : "heard")
            writePut(StoredItem(id: id, kind: .runsheet, title: e.title, text: [e.title, e.text ?? ""].filter { !$0.isEmpty }.joined(separator: "\n"), date: now,
                                 address: e.source?.ref ?? id, source: e.state == .skipped ? "Skipped" : "Heard", version: "1", syncedAt: now, meta: ["outcome": outcome, "kind": e.kind.rawValue]))
        }
    }
}

public extension ContentSync.Fetchers {
    /// The real routes (see the table at the top of ContentSync.swift).
    static func live() -> ContentSync.Fetchers {
        var f = ContentSync.Fetchers()
        func client() -> ChoirClient { ChoirEnvironment.shared.client() }
        f.documentPage = { cursor in
            var q = ["limit": "500"]; if let cursor { q["cursor"] = cursor }
            let o = try await client().send("GET", "/api/texture/documents", query: q, route: "/api/texture/documents").object()
            let docs = (o["documents"] as? [[String: Any]] ?? []).compactMap { d -> ContentSync.DocRow? in
                guard let id = d["doc_id"] as? String, !id.isEmpty, d["archived_at"] == nil || d["archived_at"] is NSNull else { return nil }
                return .init(id: id, title: (d["title"] as? String) ?? "", updated: (d["updated_at"] as? String) ?? "")
            }
            return (docs, o["next_cursor"] as? String)
        }
        f.documentBody = { id in
            let d = try await ReaderStore.shared.fetch("texture:" + id)
            return d.source.isEmpty ? (d.lines.isEmpty ? nil : d.lines.joined(separator: "\n")) : d.source
        }
        f.stories = {
            try await ChoirFront.stories(client: client(), limit: 100).map { StoryBodies.Story(route: $0.route, title: $0.title, date: PaperStory.date($0.updated)) }
        }
        f.storyBody = StoryBodies.resolveFetcher
        // The podcasts' shows, episodes and transcripts were read here from the Swift PodcastService. The podcast app is shared code now
        // (AutoradioLab/podcast.ts, `podcastDocuments`); until the content store moves and reads it, the phone's own search holds no podcasts.
        f.runSheet = { await MainActor.run { RunSheetStore.shared.sheet } }
        return f
    }
}

public extension ContentSync {
    /// The app's one syncer: the shared store, the real routes. `kick()` starts a run in the background (a run in progress is not doubled).
    static let shared: ContentSync = ContentSync(store: .shared, fetch: .live(), log: { fputs($0 + "\n", stderr) })
    /// Off until the app turns it on (a test must not sync the machine's real mail).
    nonisolated(unsafe) static var enabled = false
    nonisolated(unsafe) static var kicked: Task<[SyncReport], Never>?
    static func kick() {
        guard enabled, ContentStore.shared.usable else { return }
        kicked = Task.detached(priority: .utility) { await ContentSync.shared.syncAll() }
    }
    /// What sign-out does: the store, the index and the kept answers go with the account.
    static func wipeAccount(store: ContentStore = .shared, corpus: SavedCorpus = .shared) {
        kicked?.cancel(); kicked = nil
        store.wipe(); corpus.reset()
    }
}

public enum ContentBoot {
    /// Opens the store and wires it to the index, off the main thread and after launch, so nothing the store does can hold up the front page.
    /// Prints one line to the console: `AUTORADIO store open ok items=<n>` or `AUTORADIO store open FAILED <reason>`; returns whether it opened.
    @discardableResult
    public static func start(store: ContentStore = .shared, corpus: SavedCorpus = .shared, say: @Sendable (String) -> Void = { fputs($0 + "\n", stderr) }) -> Bool {
        guard store.usable else { say("AUTORADIO store open FAILED \(store.openReason)"); return false }
        say("AUTORADIO store open ok items=\(store.summary().perKind.values.reduce(0) { $0 + $1.count })")
        corpus.setDeltaSource("store") { store.drain() }      // the index reads it at the first question (or the warm-up the app schedules), never here
        return true
    }
}
