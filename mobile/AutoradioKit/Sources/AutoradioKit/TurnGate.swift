import Foundation

/// Whether a turn may be sent at all: only with an account's key (`keyPresent`, wired by the module, as on main), and never while the account is being signed out.
/// "A sign-out is running" IS "the sign-out task slot is occupied", read under the lock that guards the slot: there is no flag that anything sets or clears separately. `signOut(_:)` is the ONE way the gate shuts (it
/// creates the task, stores it and moves the epoch inside the lock), and the task's own last act empties the slot under the same lock, so no caller can shut the gate without the sequence that reopens it. The EPOCH moves
/// only there. Every turn entry captures it and asks `admits` again at the last point before anything leaves the phone; a turn's epoch travels with its requests as a task-local (`turnEpoch`) to `ChoirClient.request`, the one
/// native key read, which refuses a request of an older epoch even after a new sign-in. Natively there is no event that says a key was stored (a sign-in is shared code writing the Exact secret, `keepKey`), so the gate keeps
/// no copy of "who is signed in": after the sequence ends, whether a turn may go is what `keyPresent` says, as on main.
public final class TurnGate: @unchecked Sendable {
    public static let shared = TurnGate()
    private let lock = NSLock()
    private var value = true, holds = 0, epochN = 0
    private var slot: Task<Void, Never>?
    /// Whether an account's key is there (set once by the module); nil in the engine alone (tests, the lab). Read outside the lock.
    public var keyPresent: (@Sendable () -> Bool)?
    /// The sign-out epoch of the turn this task belongs to (set where the turn's pipeline starts; child tasks inherit it); nil for a request no turn made.
    @TaskLocal public static var turnEpoch: Int?
    public var epoch: Int { lock.lock(); defer { lock.unlock() }; return epochN }
    /// A sign-out is running: its task is in the slot.
    public var signedOutShut: Bool { lock.lock(); defer { lock.unlock() }; return slot != nil }
    /// The account side only (a screen's `hold` is not part of it): no sign-out running, not switched off, and a key if the module wired a reader.
    public var accountOpen: Bool { lock.lock(); let ok = value && slot == nil; let k = keyPresent; lock.unlock(); return ok && (k?() ?? true) }
    public var allowed: Bool {
        get { lock.lock(); let ok = value && holds == 0 && slot == nil; let k = keyPresent; lock.unlock(); return ok && (k?() ?? true) }
        set { lock.lock(); value = newValue; lock.unlock() }
    }
    /// One locked read: this epoch is the current one and no sign-out is running.
    public func admits(_ e: Int) -> Bool { lock.lock(); defer { lock.unlock() }; return slot == nil && epochN == e }
    /// Read under the gate's lock, only if no sign-out runs and `epoch` (nil: the current one) is still current: what `signOut` also takes, so a read is wholly before or wholly after it.
    func guarded<T>(_ epoch: Int?, _ read: () -> T?) -> T? { lock.lock(); defer { lock.unlock() }; if slot != nil || (epoch != nil && epoch != epochN) { return nil }; return read() }
    /// The ONLY way the gate shuts: runs `body` as the sign-out task, or returns the one already running. The slot is emptied by the task's last act, under the same lock.
    @discardableResult
    public func signOut(_ body: @escaping @Sendable () async -> Void) -> Task<Void, Never> {
        lock.lock(); defer { lock.unlock() }
        if let t = slot { return t }
        epochN += 1
        let t = Task { await body(); self.lock.lock(); self.slot = nil; self.lock.unlock() }
        slot = t; return t
    }
    /// Tests only: forget a previous test's sign-out and settings.
    func resetForTests() { lock.lock(); value = true; holds = 0; slot = nil; lock.unlock(); keyPresent = nil }
    /// A screen that listens for its own check (the first run's microphone check) holds turns back for as long as it listens; every `hold` has its `release`.
    public func hold() { lock.lock(); holds += 1; lock.unlock() }
    public func release() { lock.lock(); holds = max(0, holds - 1); lock.unlock() }
}
