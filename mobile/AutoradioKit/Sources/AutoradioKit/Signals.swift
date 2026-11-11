import Foundation

// Signals: the gesture events the listener's paper learns from (plan section 1, "What the swipes mean").
// A lane records one with `signals.record(SignalEvent(kind:subject:detail:))`. The spine records page opens and
// closes; lane 4 owns the meaning of the rest (more-of-this, less-of-this, skip, listened-through, ...).
// The default implementation keeps a short ring in memory and appends JSON lines to a file in the app's support
// directory; nothing leaves the phone in the spine.

public struct SignalEvent: Codable, Sendable, Equatable {
    public var at: Double          // seconds since 1970
    public var kind: String        // open, close, more-of-this, less-of-this, skip, ...
    public var subject: String     // a page address, or "<address>#<anchor>"
    public var detail: String
    public init(kind: String, subject: String, detail: String = "", at: Double = Date().timeIntervalSince1970) {
        self.kind = kind; self.subject = subject; self.detail = detail; self.at = at
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

public final class FileSignals: Signals, @unchecked Sendable {
    private let lock = NSLock()
    private var ring: [SignalEvent] = []
    private let file: URL?
    public init(file: URL?) { self.file = file }
    public func record(_ e: SignalEvent) {
        lock.lock(); ring.append(e); if ring.count > 200 { ring.removeFirst(ring.count - 200) }; lock.unlock()
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
