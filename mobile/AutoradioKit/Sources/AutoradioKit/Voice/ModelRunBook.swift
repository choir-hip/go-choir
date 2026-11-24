import Foundation

/// What Settings > Voice and models remembers between presses, per account: each row's Test line, and the last failure of each model's download. Native because the runs are; kept here so the rules are tested.
/// A Test line belongs to the account that ran it (its words are what that person said): sign-out clears the book, and a report from a run that began before the wipe is dropped. A failure stays on its row until
/// that model is pressed again or is on the phone, so another row's run does not erase it.
public final class ModelRunBook: @unchecked Sendable {
    private let lock = NSLock()
    private var lines: [String: (state: String, line: String, ms: Double)] = [:]
    private var failures: [String: String] = [:]
    public init() {}

    /// A Test report. `generation` is the account generation the test began in; a later one means the account left.
    public func test(_ key: String, state: String, line: String, ms: Double, generation: Int) {
        lock.lock(); defer { lock.unlock() }
        guard generation == AccountWipe.generation else { return }
        lines[key] = (state, line, ms)
    }
    public func tests() -> [(key: String, state: String, line: String, ms: Double)] {
        lock.lock(); defer { lock.unlock() }
        return lines.map { (key: $0.key, state: $0.value.state, line: $0.value.line, ms: $0.value.ms) }
    }
    /// True when a test of this row is running.
    public func isRunning(_ key: String) -> Bool { lock.lock(); defer { lock.unlock() }; return ["recording", "working"].contains(lines[key]?.state ?? "") }
    public func failed(_ id: String, _ line: String) { lock.lock(); failures[id] = line; lock.unlock() }
    public func speakerFailed(_ line: String) { failed(ModelOffer.speakerID, line) }
    /// Sign-out.
    public func reset() { lock.lock(); lines = [:]; failures = [:]; lock.unlock() }

    /// The runs shared code sees: the live ones, and the failures of models that are not running now and not on the phone. A model that is running again drops its old failure.
    public func runs(current: [[String: Any]], isReady: (String) -> Bool) -> [[String: Any]] {
        lock.lock(); defer { lock.unlock() }
        let live = Set(current.compactMap { $0["id"] as? String })
        for id in live { failures[id] = nil }
        for id in Array(failures.keys) where isReady(id) { failures[id] = nil }
        return current + failures.map { ["id": $0.key, "state": "failed", "progress": 0.0, "line": $0.value] }
    }
}
