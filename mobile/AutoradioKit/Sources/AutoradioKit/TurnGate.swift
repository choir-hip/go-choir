import Foundation

/// Whether a turn may be sent at all: only with an account's key. The app sets it from the key's presence (ChoirApp: at launch, at every sign-in change, and false at the start of a sign-out); the engine reads it
/// where a hands-free turn is committed. Default true so the engine alone (tests, the lab) behaves as before. Signed out the radio only listens (owner, 2026-11-23), and a switch left on at sign-out sends nothing (review, 2026-11-24).
public final class TurnGate: @unchecked Sendable {
    public static let shared = TurnGate()
    private let lock = NSLock()
    private var value = true, holds = 0
    /// Shut by `closeForSignOut`; opens only when the key was deleted (`keyDeleted`) and a key is present again, which is a new sign-in.
    private var sealed = false, keyGone = false
    /// False with no key, and while any screen holds turns back (`hold`).
    /// Whether an account's key is there (set once by the app); nil in the engine alone (tests, the lab).
    public var keyPresent: (@Sendable () -> Bool)?
    public var allowed: Bool {
        get {
            lock.lock(); let k = keyPresent; lock.unlock()
            let present = k?() ?? true
            lock.lock(); defer { lock.unlock() }
            if sealed, keyGone, present { sealed = false; keyGone = false }     // a key where there was none: the next sign-in has completed
            return value && holds == 0 && !sealed && present
        }
        set { lock.lock(); value = newValue; lock.unlock() }
    }
    /// The first step of sign-out: no turn starts from here until a new sign-in completes. Fails closed: a key that could not be deleted leaves it shut.
    public func closeForSignOut() { lock.lock(); sealed = true; keyGone = false; lock.unlock() }
    /// Sign-out has deleted the key: a key present after this is a new sign-in's.
    public func keyDeleted() { lock.lock(); keyGone = true; lock.unlock() }
    /// A screen that listens for its own check (the first run's microphone check) holds turns back for as long as it listens; every `hold` has its `release`.
    public func hold() { lock.lock(); holds += 1; lock.unlock() }
    public func release() { lock.lock(); holds = max(0, holds - 1); lock.unlock() }
}
