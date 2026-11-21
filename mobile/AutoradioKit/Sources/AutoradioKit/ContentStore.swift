import Foundation

// THE LOCAL CONTENT STORE. Owner, 2026-11-16: "we should keep mail on the phone, and we should keep everything on the phone as much as we
// can. It's mostly just text, so we want to basically have the full content on the phone." Every text the phone is shown or can fetch is kept
// here in full and indexed from here: retrieval (SavedCorpus) and "play that" read the same rows.
//
// STORAGE: one file per item (`items/<kind>/<hash>.json`) plus `manifest.json` (metadata, sync checkpoints, last-synced times). Files, not
// SQLite: the Kit has no SQLite today and a new binding is a new dependency (the system library would need a wrapper written and, in the Exact
// package, linked); per-item files give per-file data protection, atomic writes, and a delete that is a delete. APFS handles tens of thousands of
// small files. Only metadata lives in memory; bodies are read from disk when an item is opened, and once into the retrieval index.
// PROTECTION: every file is written with `.completeFileProtectionUntilFirstUserAuthentication` (encrypted at rest, unreadable before the first
// unlock after boot, readable afterwards with the screen locked: the radio plays and syncs locked, so `.complete` would break both), the
// directories carry the same protection class, and the whole root is excluded from device backups (everything is on the server and re-synced;
// mail in a backup would add a second, uncontrolled copy). The Kit already does this for the speaker profile (Voice/SpeakerProfile.swift).
// BUDGET: total 400 MB, retention by kind, eviction oldest-first in `evictionOrder`; documents the listener wrote or saved and pinned items are
// never evicted. The total is visible (`summary().line`). SIGN-OUT: `wipe()` deletes the root.
//
// SIZES (measured 2026-11-16 on https://choir.news front list, 100 stories: 418 words on average, 1,160 at most, so about 2.7 KB of text each;
// the list spans two days so at least 50 new stories a day): 30 days of stories is about 4.5 MB, cap 40 MB. Mail is assumed at 100 messages a day
// at 4 KB of text (html and attachments are not kept): a year is about 150 MB, cap 200 MB. Texture documents about 20 KB each, a few hundred: under
// 20 MB, never evicted. Files text: text and Markdown under 1 MB each, cap 60 MB. Transcripts: about 60 KB an hour, cap 40 MB.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in ContentStoreTests):
//  1. The app is killed during a sync: items already written are kept, checkpoints are those of the last flush, the next sync resumes without
//     fetching a held item again.  2. The manifest is corrupt or missing: the store starts empty (and rebuilds as it syncs); no crash.
//  3. Over budget: the oldest evictable items go first in kind order; a pinned item or a Texture document is never evicted; the total ends at or
//     under the budget when enough is evictable.  4. Retention: a story older than 30 days goes even when under budget.
//  5. Sign-out: nothing of the previous account remains on disk, in the index or in memory.  6. A file written without protection: every write
//     goes through one function that sets it.  7. Logs: counts and ids only, never a subject, a sender or a body.
//  8. A second account signs in: it sees an empty store (wipe on sign-out, and the root is per store instance).  9. A putting race: one lock.

public enum ContentKind: String, Codable, CaseIterable, Sendable {
    case story, mail, document, file, podcast, transcript, runsheet
    /// Documents the listener wrote are never evicted (see `StoredItem.pinned`); `document` (Texture) is the listener's own.
    public var evictable: Bool { self != .document }
    public var label: String {
        switch self { case .story: return "Stories"; case .mail: return "Mail"; case .document: return "Documents"; case .file: return "Files"
                      case .podcast: return "Podcast notes"; case .transcript: return "Transcripts"; case .runsheet: return "Heard" }
    }
}

public struct StoredItem: Codable, Equatable, Sendable {
    public var id: String
    public var kind: ContentKind
    public var title: String
    public var text: String
    public var date: Date?
    /// What opens or plays it: `pub:<route>`, `mail:<id>`, `texture:<id>`, `file:<path>`, a podcast address.
    public var address: String
    public var source: String
    /// An etag or revision: an item with the same version is not fetched again.
    public var version: String
    public var pinned: Bool
    public var syncedAt: Date
    /// Kind-specific fields (mail: folder, from, fromAddress, snippet, direction, readAt, trust, to).
    public var meta: [String: String]
    public init(id: String, kind: ContentKind, title: String, text: String, date: Date? = nil, address: String, source: String = "", version: String = "",
                pinned: Bool = false, syncedAt: Date = Date(), meta: [String: String] = [:]) {
        self.id = id; self.kind = kind; self.title = title; self.text = text; self.date = date; self.address = address; self.source = source
        self.version = version; self.pinned = pinned; self.syncedAt = syncedAt; self.meta = meta
    }
    public var bytes: Int { text.utf8.count + title.utf8.count + 200 }
}

public final class ContentStore: @unchecked Sendable {
    public static let shared = ContentStore(root: defaultRoot())

    public struct Budget: Sendable {
        public var totalBytes: Int
        public var perKindBytes: [ContentKind: Int]
        /// Days an item is kept by its date (nil: until the budget needs the room).
        public var retentionDays: [ContentKind: Int]
        /// Evicted first to last.
        public var evictionOrder: [ContentKind]
        public static let standard = Budget(
            totalBytes: 400_000_000,
            perKindBytes: [.story: 40_000_000, .mail: 200_000_000, .file: 60_000_000, .transcript: 40_000_000, .podcast: 20_000_000, .runsheet: 10_000_000],
            retentionDays: [.story: 30, .podcast: 90, .transcript: 90, .runsheet: 90],
            evictionOrder: [.story, .podcast, .transcript, .file, .runsheet, .mail])
        public init(totalBytes: Int, perKindBytes: [ContentKind: Int], retentionDays: [ContentKind: Int], evictionOrder: [ContentKind]) {
            self.totalBytes = totalBytes; self.perKindBytes = perKindBytes; self.retentionDays = retentionDays; self.evictionOrder = evictionOrder
        }
    }
    public struct Meta: Codable, Equatable, Sendable {
        public var id: String, kind: ContentKind, bytes: Int, date: Date?, syncedAt: Date, pinned: Bool, version: String, title: String, address: String, file: String
    }
    private struct Manifest: Codable { var metas: [Meta]; var checkpoints: [String: String]; var synced: [String: Date] }

    /// The write options for every file: encrypted at rest, readable after the first unlock. (Asserted by a test; the platform applies it.)
    public static let writeOptions: Data.WritingOptions = [.atomic, .completeFileProtectionUntilFirstUserAuthentication]

    public let root: URL?
    public var budget: Budget
    private let clock: @Sendable () -> Date
    private let protect: (@Sendable (URL) -> Void)?
    private let log: (@Sendable (String) -> Void)?
    private let lock = NSLock()
    private var metas: [String: Meta] = [:]
    private var checkpoints: [String: String] = [:]
    private var synced: [String: Date] = [:]
    private var up: [String: Bool] = [:]          // ids whose document the index has not read yet
    private var removed = Set<String>()
    private var dirty = false
    public var onChange: (@Sendable () -> Void)?
    /// Whether the store could open its folder and write to it. When it could not, every call does nothing and the app behaves as it did without a store.
    public private(set) var usable = false
    public private(set) var openReason = ""

    /// `protect` is called with every file and directory the store creates (tests record it; the default sets the platform attributes).
    public init(root: URL?, budget: Budget = .standard, clock: @escaping @Sendable () -> Date = { Date() },
                protect: (@Sendable (URL) -> Void)? = nil, log: (@Sendable (String) -> Void)? = nil) {
        self.root = root; self.budget = budget; self.clock = clock; self.log = log
        self.protect = protect ?? { url in
            try? FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: url.path)
        }
        guard let root else { openReason = "no folder"; return }
        do {
            try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
            let probe = root.appendingPathComponent(".probe")
            try Data("ok".utf8).write(to: probe, options: Self.writeOptions)
            try? FileManager.default.removeItem(at: probe)
            usable = true
        } catch {
            let e = error as NSError
            openReason = "\(e.domain) \(e.code)"                       // a code, never a path or content
            log?("content: store open failed (\(openReason))")
            return
        }
        self.protect?(root)
        var v = URLResourceValues(); v.isExcludedFromBackup = true
        var r = root; try? r.setResourceValues(v)
        if let d = try? Data(contentsOf: root.appendingPathComponent("manifest.json")), let m = try? JSONDecoder().decode(Manifest.self, from: d) {
            for x in m.metas { metas[x.id] = x; up[x.id] = true }
            checkpoints = m.checkpoints; synced = m.synced
        }
        purgeMail()
    }

    /// 2026-11-19, the cutover file's D3: mail moved to shared code (AutoradioLab/mail.ts), whose own sealed copy is a cache of the listener's computer. This store's
    /// copy is not migrated: at every open, the mail rows, the mail files, the mail sync checkpoints and the "last synced" time are deleted (what is left to delete is
    /// nothing, so it is cheap and needs no marker), and the retrieval index is told the ids left. Nothing but mail is touched. Returns how many items it removed.
    @discardableResult
    func purgeMail() -> Int {
        guard usable else { return 0 }
        let mailIDs = ids(.mail)
        for id in mailIDs { remove(id) }
        lock.lock()
        let stale = checkpoints.keys.filter { $0.hasPrefix("mail.") }
        for k in stale { checkpoints[k] = nil }
        let hadSynced = synced.removeValue(forKey: ContentKind.mail.rawValue) != nil
        if !stale.isEmpty || hadSynced { dirty = true }
        lock.unlock()
        if let d = dir(.mail), FileManager.default.fileExists(atPath: d.path) { try? FileManager.default.removeItem(at: d) }
        if !mailIDs.isEmpty || !stale.isEmpty || hadSynced { flush(); log?("content: mail copy deleted (\(mailIDs.count) items)") }
        return mailIDs.count
    }
    static func defaultRoot() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        return d.appendingPathComponent("Autoradio", isDirectory: true).appendingPathComponent("content", isDirectory: true)
    }

    // MARK: files

    private static func fileName(_ id: String) -> String {
        var h: UInt64 = 0xcbf29ce484222325
        for b in id.utf8 { h = (h ^ UInt64(b)) &* 0x100000001b3 }
        return String(h, radix: 16)
    }
    private func dir(_ kind: ContentKind) -> URL? { root?.appendingPathComponent("items", isDirectory: true).appendingPathComponent(kind.rawValue, isDirectory: true) }
    /// The one place a file is written: protected, atomic, never in a backup.
    private func write(_ data: Data, to url: URL) {
        let fm = FileManager.default
        let d = url.deletingLastPathComponent()
        if !fm.fileExists(atPath: d.path) {
            try? fm.createDirectory(at: d, withIntermediateDirectories: true)
            protect?(d)
        }
        try? data.write(to: url, options: Self.writeOptions)
        protect?(url)
    }

    // MARK: put, get, remove

    public func put(_ item: StoredItem) {
        guard usable else { return }
        var it = item; if it.syncedAt == Date.distantPast { it.syncedAt = clock() }
        guard let dir = dir(it.kind), let data = try? JSONEncoder().encode(it) else { return }
        let name = Self.fileName(it.id) + ".json"
        write(data, to: dir.appendingPathComponent(name))
        lock.lock()
        metas[it.id] = Meta(id: it.id, kind: it.kind, bytes: it.bytes, date: it.date, syncedAt: it.syncedAt, pinned: it.pinned || !it.kind.evictable, version: it.version,
                            title: it.title, address: it.address, file: name)
        up[it.id] = true; removed.remove(it.id); dirty = true
        lock.unlock()
        onChange?()
    }
    public func get(_ id: String) -> StoredItem? {
        lock.lock(); let m = metas[id]; lock.unlock()
        guard let m, let dir = dir(m.kind), let d = try? Data(contentsOf: dir.appendingPathComponent(m.file)), let it = try? JSONDecoder().decode(StoredItem.self, from: d), it.id == id else { return nil }
        return it
    }
    public func meta(_ id: String) -> Meta? { lock.lock(); defer { lock.unlock() }; return metas[id] }
    /// Held with this version: a sync does not fetch it again.
    public func has(_ id: String, version: String? = nil) -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard let m = metas[id] else { return false }
        return version.map { $0 == m.version } ?? true
    }
    public func ids(_ kind: ContentKind) -> [String] { lock.lock(); defer { lock.unlock() }; return metas.values.filter { $0.kind == kind }.map(\.id) }
    public func items(_ kind: ContentKind) -> [StoredItem] { ids(kind).compactMap { get($0) } }
    /// The item that opens at an address (a story at `pub:<route>`, a document at `texture:<id>`, a message at `mail:<id>`).
    public func item(address: String) -> StoredItem? {
        lock.lock(); let id = metas.values.first { $0.address == address }?.id; lock.unlock()
        return id.flatMap { get($0) }
    }
    public func remove(_ id: String) {
        lock.lock(); let m = metas.removeValue(forKey: id); up[id] = nil; if m != nil { removed.insert(id); dirty = true }; lock.unlock()
        if let m, let dir = dir(m.kind) { try? FileManager.default.removeItem(at: dir.appendingPathComponent(m.file)) }
    }

    // MARK: checkpoints

    public func checkpoint(_ key: String) -> String? { lock.lock(); defer { lock.unlock() }; return checkpoints[key] }
    public func setCheckpoint(_ key: String, _ value: String?) { lock.lock(); checkpoints[key] = value; dirty = true; lock.unlock() }
    public func markSynced(_ kind: ContentKind) { lock.lock(); synced[kind.rawValue] = clock(); dirty = true; lock.unlock() }
    public func lastSynced(_ kind: ContentKind) -> Date? { lock.lock(); defer { lock.unlock() }; return synced[kind.rawValue] }
    /// Write the manifest (metadata and checkpoints). A sync flushes after every batch, so a kill loses at most one batch's checkpoint.
    public func flush() {
        guard usable else { return }
        lock.lock(); let m = Manifest(metas: Array(metas.values), checkpoints: checkpoints, synced: synced); dirty = false; lock.unlock()
        guard let root, let d = try? JSONEncoder().encode(m) else { return }
        write(d, to: root.appendingPathComponent("manifest.json"))
    }

    // MARK: budget

    public struct Summary: Sendable, Equatable {
        public var totalBytes: Int, budgetBytes: Int
        public var perKind: [ContentKind: (count: Int, bytes: Int)]
        public static func == (a: Summary, b: Summary) -> Bool { a.totalBytes == b.totalBytes && a.budgetBytes == b.budgetBytes }
        /// "412 MB of 400 MB: 3,120 stories 12 MB, 18,200 mail 280 MB ..." for a settings row.
        public var line: String {
            let parts = ContentKind.allCases.compactMap { k -> String? in
                guard let v = perKind[k], v.count > 0 else { return nil }
                return "\(v.count) \(k.label.lowercased()) \(Self.size(v.bytes))"
            }
            return "\(Self.size(totalBytes)) of \(Self.size(budgetBytes))" + (parts.isEmpty ? "" : ": " + parts.joined(separator: ", "))
        }
        static func size(_ b: Int) -> String { b >= 1_000_000 ? String(format: "%.0f MB", Double(b) / 1e6) : String(format: "%.0f KB", Double(b) / 1e3) }
    }
    public func summary() -> Summary {
        lock.lock(); defer { lock.unlock() }
        var per: [ContentKind: (count: Int, bytes: Int)] = [:]
        for m in metas.values { var v = per[m.kind] ?? (0, 0); v.count += 1; v.bytes += m.bytes; per[m.kind] = v }
        return Summary(totalBytes: per.values.reduce(0) { $0 + $1.bytes }, budgetBytes: budget.totalBytes, perKind: per)
    }

    /// Drop what is past its retention, then the oldest evictable items (kind order) while over the total or a kind's own cap. Returns the ids dropped.
    @discardableResult
    public func enforceBudget() -> [String] {
        let now = clock()
        var drop: [Meta] = []
        lock.lock()
        func evictable(_ m: Meta) -> Bool { !m.pinned && m.kind.evictable }
        for m in metas.values where evictable(m) {
            if let days = budget.retentionDays[m.kind], let d = m.date ?? Optional(m.syncedAt), now.timeIntervalSince(d) > Double(days) * 86400 { drop.append(m) }
        }
        var gone = Set(drop.map(\.id))
        func total(_ kind: ContentKind? = nil) -> Int { metas.values.filter { !gone.contains($0.id) && (kind == nil || $0.kind == kind) }.reduce(0) { $0 + $1.bytes } }
        for kind in budget.evictionOrder {
            let cap = budget.perKindBytes[kind] ?? Int.max
            let old = metas.values.filter { $0.kind == kind && evictable($0) && !gone.contains($0.id) }.sorted { ($0.date ?? $0.syncedAt) < ($1.date ?? $1.syncedAt) }
            var i = 0
            while total(kind) > cap, i < old.count { gone.insert(old[i].id); drop.append(old[i]); i += 1 }
        }
        for kind in budget.evictionOrder where total() > budget.totalBytes {
            let old = metas.values.filter { $0.kind == kind && evictable($0) && !gone.contains($0.id) }.sorted { ($0.date ?? $0.syncedAt) < ($1.date ?? $1.syncedAt) }
            var i = 0
            while total() > budget.totalBytes, i < old.count { gone.insert(old[i].id); drop.append(old[i]); i += 1 }
        }
        lock.unlock()
        for m in drop { remove(m.id) }
        if !drop.isEmpty { log?("content: evicted \(drop.count) items (\(drop.map(\.kind.rawValue).reduce(into: [String: Int]()) { $0[$1, default: 0] += 1 }))"); flush(); onChange?() }
        return drop.map(\.id)
    }

    // MARK: sign-out

    /// Delete everything: the files, the manifest, the memory, the checkpoints. Sign-out calls this.
    public func wipe() {
        lock.lock(); metas = [:]; up = [:]; removed = []; checkpoints = [:]; synced = [:]; dirty = false; lock.unlock()
        if let root { try? FileManager.default.removeItem(at: root); try? FileManager.default.createDirectory(at: root, withIntermediateDirectories: true); protect?(root) }
        log?("content: wiped")
        onChange?()
    }

    // MARK: the index reads from here

    /// What the retrieval index has not read yet: new or changed items as documents, and the ids that left. The first call after launch reads every
    /// body once (do it off the main thread: `SavedCorpus.refresh` at launch).
    public func drain() -> (up: [SavedCorpus.Document], removed: [String]) {
        lock.lock(); let ids = Array(up.keys); let gone = Array(removed); up = [:]; removed = []; lock.unlock()
        var docs: [SavedCorpus.Document] = []
        for id in ids { if let it = get(id) { docs.append(Self.document(it)) } }
        return (docs, gone)
    }
    public static func document(_ it: StoredItem) -> SavedCorpus.Document {
        let kind: String
        switch it.kind { case .story: kind = "story"; case .mail: kind = "mail"; case .document: kind = "document"; case .file: kind = "file"
                         case .podcast: kind = "podcast"; case .transcript: kind = "transcript"; case .runsheet: kind = "runsheet" }
        return SavedCorpus.Document(id: "store:" + it.id, title: it.title, address: it.address, text: it.text, kind: kind, source: it.source.isEmpty ? it.kind.label : it.source, date: it.date)
    }

    // MARK: offline pages

    /// "last synced 2 hours ago" for a banner (nil: never synced).
    public func syncedWords(_ kind: ContentKind) -> String? {
        guard let d = lastSynced(kind) else { return nil }
        let s = clock().timeIntervalSince(d)
        let when = s < 90 ? "just now" : s < 5400 ? "\(Int(s / 60)) minutes ago" : s < 129_600 ? "\(Int((s / 3600).rounded())) hours ago" : "\(Int((s / 86400).rounded())) days ago"
        return "last synced \(when)"
    }
}
