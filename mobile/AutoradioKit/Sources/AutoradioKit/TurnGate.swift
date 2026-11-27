import Foundation

/// Whether a turn may be sent at all: only with an account's key (`keyPresent`, wired by the module, as on main), and never while the account is being signed out.
/// The sign-out part is ONE state under ONE lock and no query calls out while it decides: a flag (`signedOutShut`: sign-out has begun and its sequence has not ended) and an EPOCH (moved, synchronously,
/// by `closeForSignOut`, once per sign-out). Every turn entry captures the epoch and asks `admits` again at the last point before anything leaves the phone; a turn's epoch travels with its requests as a task-local
/// (`turnEpoch`) to `ChoirClient.request`, the one native key read, which refuses a request of an older epoch even after a new sign-in. Natively there is no event that says a key was stored (a sign-in is shared code
/// writing the Exact secret, `keepKey`), so the gate keeps no copy of "who is signed in": after the sequence ends, whether a turn may go is what `keyPresent` says, as on main.
public final class TurnGate: @unchecked Sendable {
    public static let shared = TurnGate()
    private let lock = NSLock()
    private var value = true, holds = 0, epochN = 0, out = false
    /// Whether an account's key is there (set once by the module); nil in the engine alone (tests, the lab). Read outside the lock.
    public var keyPresent: (@Sendable () -> Bool)?
    /// The sign-out epoch of the turn this task belongs to (set where the turn's pipeline starts; child tasks inherit it); nil for a request no turn made.
    @TaskLocal public static var turnEpoch: Int?
    public var epoch: Int { lock.lock(); defer { lock.unlock() }; return epochN }
    /// Sign-out has begun and its sequence has not ended.
    public var signedOutShut: Bool { lock.lock(); defer { lock.unlock() }; return out }
    /// The account side only (a screen's `hold` is not part of it): no sign-out running, not switched off, and a key if the module wired a reader.
    public var accountOpen: Bool { lock.lock(); let ok = value && !out; let k = keyPresent; lock.unlock(); return ok && (k?() ?? true) }
    public var allowed: Bool {
        get { lock.lock(); let ok = value && holds == 0 && !out; let k = keyPresent; lock.unlock(); return ok && (k?() ?? true) }
        set { lock.lock(); value = newValue; lock.unlock() }
    }
    /// One locked read: this epoch is the current one and no sign-out is running.
    public func admits(_ e: Int) -> Bool { lock.lock(); defer { lock.unlock() }; return !out && epochN == e }
    /// Read under the gate's lock, only if no sign-out runs and `epoch` (nil: the current one) is still current: what sign-out's `closeForSignOut` also takes, so a read is wholly before or wholly after it.
    func guarded<T>(_ epoch: Int?, _ read: () -> T?) -> T? { lock.lock(); defer { lock.unlock() }; if out || (epoch != nil && epoch != epochN) { return nil }; return read() }
    /// The first statement of sign-out, synchronous (idempotent while one runs): the flag is set and the epoch moves.
    public func closeForSignOut() { lock.lock(); if !out { out = true; epochN += 1 }; lock.unlock() }
    /// The sequence has ended, whatever came of it (the app calls this under its own lock, with clearing the sign-out task).
    public func finishSignOut() { lock.lock(); out = false; lock.unlock() }
    /// Tests only: forget a previous test's sign-out and settings.
    func resetForTests() { lock.lock(); value = true; holds = 0; out = false; lock.unlock(); keyPresent = nil }
    /// A screen that listens for its own check (the first run's microphone check) holds turns back for as long as it listens; every `hold` has its `release`.
    public func hold() { lock.lock(); holds += 1; lock.unlock() }
    public func release() { lock.lock(); holds = max(0, holds - 1); lock.unlock() }
}
