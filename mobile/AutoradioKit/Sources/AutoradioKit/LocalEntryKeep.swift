import Foundation

/// What a local-only run sheet entry (a message, a book, a file) may not post, kept on the phone by entry id: its real title and label (the posted
/// entry carries neutral ones) and, for the continuation of a cut passage, the words still to read. One JSON file in Application Support, protected
/// until first unlock, excluded from backup, at most 50 rows (the oldest dropped). A row goes when its entry finishes, is skipped or leaves the
/// sheet. Sign-out empties it (`AccountWipe`), and a write made for an account that was wiped meanwhile is dropped.
public final class LocalEntryKeep: @unchecked Sendable {
    public struct Row: Codable, Equatable, Sendable { public var id: String; public var title: String; public var label: String; public var rest: String? }
    public static let cap = 50
    /// The app's keep. Under the test runner it is memory only.
    public static let shared = LocalEntryKeep(file: ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] == nil ? LocalEntryKeep.defaultFile() : nil)

    private let lock = NSLock()
    private var rows: [Row] = []
    private let file: URL?

    public init(file: URL?) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file), let r = try? JSONDecoder().decode([Row].self, from: d) { rows = r }
    }

    static func defaultFile() -> URL? {
        guard let d = try? FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        let dir = d.appendingPathComponent("Autoradio", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("local-entries.json")
    }

    /// Keep (or replace) the row of an entry. `generation` is the account generation the caller's work began in.
    public func put(id: String, title: String, label: String, rest: String? = nil, generation: Int = AccountWipe.generation) {
        lock.lock(); defer { lock.unlock() }
        guard generation == AccountWipe.generation else { return }
        rows.removeAll { $0.id == id }
        rows.append(Row(id: id, title: title, label: label, rest: rest))
        if rows.count > Self.cap { rows.removeFirst(rows.count - Self.cap) }
        save()
    }
    public func row(_ id: String) -> Row? { lock.lock(); defer { lock.unlock() }; return rows.first { $0.id == id } }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return rows.count }
    public func remove(_ id: String) {
        lock.lock(); defer { lock.unlock() }
        let n = rows.count; rows.removeAll { $0.id == id }
        if rows.count != n { save() }
    }
    /// These entries finished, were skipped or left the sheet: their rows go.
    public func remove(ids: Set<String>) {
        lock.lock(); defer { lock.unlock() }
        let n = rows.count; rows.removeAll { ids.contains($0.id) }
        if rows.count != n { save() }
    }
    public func wipe() {
        lock.lock(); defer { lock.unlock() }
        rows = []
        if let file { try? FileManager.default.removeItem(at: file) }
    }

    /// Under the lock.
    private func save() {
        guard let file, let d = try? JSONEncoder().encode(rows) else { return }
        try? d.write(to: file, options: .atomic)
        try? FileManager.default.setAttributes([.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication], ofItemAtPath: file.path)
        var v = URLResourceValues(); v.isExcludedFromBackup = true
        var u = file; try? u.setResourceValues(v)
    }
}

extension RunSheetEntry {
    /// The title the phone shows for this entry: the kept real title of a local-only entry, else its own. Never used for anything posted.
    public func shownTitle(keep: LocalEntryKeep = .shared) -> String {
        if let t = keep.row(id)?.title, !t.isEmpty { return t }
        return displayTitle
    }
}
