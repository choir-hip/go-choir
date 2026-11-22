import Foundation

// Takes, what stays native: the story that was on air when a take began recording (`TakeResponds`, the take recorder's provenance), and
// the read-only copies shared code pushes of the listener's own ids and muted voices (`TakeOwn`, `TakeMutes`, `TakeMirror`: AutoradioModule+Takes.swift
// `takeMirror`; nothing in Swift reads them since the programme request moved to AutoradioLab/radio-writer.ts, so they are written and wiped only).
// The page and the wire for publishing are shared code: AutoradioLab/takes.ts, take-session.ts; the state table is in docs/reference/AUTORADIO-TAKES.md.

/// The story that was on air when recording began (`responds_to`): provenance for the take.
public struct TakeResponds: Codable, Equatable, Sendable {
    public var route: String
    public var title: String
    public init(route: String, title: String) { self.route = route; self.title = title }

    var json: [String: Any] { ["route": route, "title": title] }
    static func parse(_ v: Any?) -> TakeResponds? {
        guard let o = v as? [String: Any], let r = o["route"] as? String, !r.isEmpty else { return nil }
        return TakeResponds(route: PodcastClip.bare(r), title: (o["title"] as? String) ?? "")
    }

    /// The story an entry on air is about: its page, else the first story it cites. Nil for an entry with none (an opening, an answer, a take).
    @MainActor public static func from(entry e: RunSheetEntry?) -> TakeResponds? {
        guard let e, e.kind != .take, e.kind != .transition else { return nil }
        var route = ""
        let addr = RadioPage.openAddress(e)
        if addr.hasPrefix("pub:") { route = String(addr.dropFirst(4)) }
        if route.isEmpty, let c = RadioPage.builtOn(e).first(where: { $0.address.hasPrefix("pub:") }) { route = String(c.address.dropFirst(4)); return TakeResponds(route: route, title: c.title) }
        guard !route.isEmpty else { return nil }
        return TakeResponds(route: route, title: e.displayTitle)
    }
}

/// Where the phone keeps these (its own support folder).
enum TakeFiles {
    /// The pre-release build's two files (own ids, muted voices): never read by this build, removed with the account (AccountWipe).
    static let legacy: [URL?] = [url("takes-own.json"), url("takes-muted.json")]
    static func url(_ name: String) -> URL? {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.appendingPathComponent("Autoradio").appendingPathComponent(name)
    }
}

/// The listener's own takes (ids and handle), so the public list never offers their own voice back. A READ-ONLY copy of shared code's data, pushed with `replace` (never the old files: `TakeLegacy`).
public final class TakeOwn: @unchecked Sendable {
    public static let shared = TakeOwn(file: TakeFiles.url("takes-copy-own.json"))
    private let lock = NSLock()
    private var ids: Set<String> = []
    private var _handle = ""
    private let file: URL?
    public init(file: URL? = nil) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
            ids = Set((o["ids"] as? [String]) ?? []); _handle = (o["handle"] as? String) ?? ""
        }
    }
    public var handle: String { lock.lock(); defer { lock.unlock() }; return _handle }
    static func validHandle(_ s: String) -> Bool { !s.isEmpty && s.utf8.count <= 64 && !s.unicodeScalars.contains { CharacterSet.whitespacesAndNewlines.contains($0) || $0.properties.generalCategory == .control } }
    /// Shared code's copy: the whole state, in `takes-copy-own.json`.
    public func replace(ids new: [String], handle h: String) {
        lock.lock(); ids = Set(new); _handle = TakeOwn.validHandle(h) ? h : ""; lock.unlock(); save()
    }
    /// Sign-out: the ids and the handle are the account's.
    public func wipe() {
        lock.lock(); ids = []; _handle = ""; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
    }
    private func save() {
        guard let file else { return }
        lock.lock(); let o: [String: Any] = ["ids": Array(ids).sorted(), "handle": _handle]; lock.unlock()
        if let d = try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys]) { try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true); try? d.write(to: file, options: .atomic) }
    }
}

/// The programme's copy accepts a push only from the account generation that is current (take-session.ts sends its account generation with every push). A push older than one already seen is refused, and
/// after the account wipe so is every push of a generation already seen (an old account's push still in flight); the new account's first push, of a newer generation, is accepted. Exact's TS hot reload does not
/// restart this module and restarts the generation at 0, so after one every push is refused for the rest of that development session: relaunch.
public enum TakeMirror {
    nonisolated(unsafe) static var floor = 0, seen = -1
    private static let lock = NSLock()
    public static func accept(_ gen: Int) -> Bool { lock.lock(); defer { lock.unlock() }; let ok = gen >= max(floor, seen); seen = max(seen, gen); return ok }
    public static func invalidate() { lock.lock(); floor = seen + 1; lock.unlock() }
    static func reset() { lock.lock(); floor = 0; seen = -1; lock.unlock() }
}

/// Voices the listener will not hear: a READ-ONLY copy of shared code's list (app:/data/takes.json), pushed with `replace`, kept in `takes-copy-muted.json`
/// (not the old muted file, which nothing reads).
public final class TakeMutes: @unchecked Sendable {
    public static let shared = TakeMutes(file: TakeFiles.url("takes-copy-muted.json"))
    private let lock = NSLock()
    private var handles: [String] = []
    private let file: URL?
    public init(file: URL? = nil) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file), let a = (try? JSONSerialization.jsonObject(with: d)) as? [String] { handles = a.filter(TakeOwn.validHandle) }
    }
    public var all: [String] { lock.lock(); defer { lock.unlock() }; return handles }
    /// Sign-out: the muted voices are the account's.
    public func wipe() {
        lock.lock(); handles = []; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
    }
    public func isMuted(_ h: String) -> Bool { lock.lock(); defer { lock.unlock() }; return handles.contains(h) }
    public func replace(_ new: [String]) {
        lock.lock(); handles = new.filter(TakeOwn.validHandle); lock.unlock()
        save()
    }
    private func save() {
        guard let file else { return }
        lock.lock(); let a = handles; lock.unlock()
        if let d = try? JSONSerialization.data(withJSONObject: a), (try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)) != nil { try? d.write(to: file, options: .atomic) }
    }
}
