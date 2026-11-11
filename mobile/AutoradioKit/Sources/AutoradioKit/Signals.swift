import Foundation

// Signals: the gesture events the listener's paper learns from (plan section 1, "What the swipes mean").
// A lane records one with `signals.record(SignalEvent(kind:subject:detail:))`. The spine records page opens and
// closes; lane 4 owns the meaning of the rest.
// The default implementation keeps a short ring in memory and appends JSON lines to a file in the app's support
// directory; nothing leaves the phone.
//
// Kinds in use (SignalKind): open, close (the spine); more-of-this, less-of-this, set-aside, undo, forget, reset
// (the gestures and the listener's acts on what was learned); gesture-ignored (a swipe the meanings in force turned
// into nothing, recorded so the comparison in the plan can count it).
// A gesture is one event: subject = the page address (a story is `pub:<route>`), detail = the JSON line the plan asks
// for (SignalDetail: surface, page kind, meaning in force, entities, topic), the time is `at`.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. The file holds a torn or foreign line (a crash mid-append): loading skips it and keeps the rest.
//  2. The file grows without bound: the ring is 200 in memory; the file is trimmed to its last 2000 lines at launch.
//  3. Two threads record at once: one lock covers the ring and the append.
//  4. A detail with quotes or newlines: it is built with JSONSerialization, never by concatenating strings.
//  5. recent(limit:) after a relaunch would be empty and "recent pages" with it: the ring is loaded from the file's tail.
//  6. A detail that is not JSON (the spine's open/close details are plain words): parse returns [:], never throws.

public struct SignalEvent: Codable, Sendable, Equatable {
    public var at: Double          // seconds since 1970
    public var kind: String        // open, close, more-of-this, less-of-this, set-aside, ...
    public var subject: String     // a page address, or "<address>#<anchor>"
    public var detail: String
    public init(kind: String, subject: String, detail: String = "", at: Double = Date().timeIntervalSince1970) {
        self.kind = kind; self.subject = subject; self.detail = detail; self.at = at
    }
}

public enum SignalKind {
    public static let open = "open", close = "close"
    public static let more = "more-of-this", less = "less-of-this", setAside = "set-aside"
    public static let undo = "undo", forget = "forget", reset = "reset", ignored = "gesture-ignored"
}

public enum SignalDetail {
    /// One gesture's detail line: {surface: pile|stack|front, page, meaning, entities, topic}.
    public static func gesture(surface: String, page: String, meaning: String, entities: [String], topic: String) -> String {
        let o: [String: Any] = ["surface": surface, "page": page, "meaning": meaning, "entities": entities, "topic": topic]
        guard let d = try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys]), let s = String(data: d, encoding: .utf8) else { return "{}" }
        return s
    }
    public static func parse(_ detail: String) -> [String: Any] {
        (try? JSONSerialization.jsonObject(with: Data(detail.utf8))) as? [String: Any] ?? [:]
    }
}

public protocol Signals: AnyObject, Sendable {
    func record(_ e: SignalEvent)
    func recent(limit: Int) -> [SignalEvent]
}

public final class NullSignals: Signals {
    public init() {}
    public func record(_ e: SignalEvent) {}
    public func recent(limit: Int) -> [SignalEvent] { [] }
}

/// A ring in memory only (tests, and the preferences when no file is wanted).
public final class MemorySignals: Signals, @unchecked Sendable {
    private let lock = NSLock()
    private var ring: [SignalEvent] = []
    public init() {}
    public func record(_ e: SignalEvent) { lock.lock(); ring.append(e); lock.unlock() }
    public func recent(limit: Int) -> [SignalEvent] { lock.lock(); defer { lock.unlock() }; return Array(ring.suffix(limit).reversed()) }
}

public final class FileSignals: Signals, @unchecked Sendable {
    private let lock = NSLock()
    private var ring: [SignalEvent] = []
    private let file: URL?
    public init(file: URL?) {
        self.file = file
        guard let file, let text = try? String(contentsOf: file, encoding: .utf8) else { return }
        var lines = text.split(separator: "\n", omittingEmptySubsequences: true)
        if lines.count > 2000 {
            lines = Array(lines.suffix(2000))
            try? (lines.joined(separator: "\n") + "\n").write(to: file, atomically: true, encoding: .utf8)
        }
        ring = lines.suffix(200).compactMap { try? JSONDecoder().decode(SignalEvent.self, from: Data($0.utf8)) }
    }
    public func record(_ e: SignalEvent) {
        lock.lock(); defer { lock.unlock() }
        ring.append(e); if ring.count > 200 { ring.removeFirst(ring.count - 200) }
        guard let file, let d = try? JSONEncoder().encode(e) else { return }
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        var line = d; line.append(0x0A)
        if let h = try? FileHandle(forWritingTo: file) { defer { try? h.close() }; _ = try? h.seekToEnd(); try? h.write(contentsOf: line) }
        else { try? line.write(to: file) }
    }
    public func recent(limit: Int) -> [SignalEvent] {
        lock.lock(); defer { lock.unlock() }
        return Array(ring.suffix(limit).reversed())
    }
}
