import Foundation

/// Whether a turn may be sent at all: only with an account's key. The app sets it from the key's presence (ChoirApp: at launch, at every sign-in change, and false at the start of a sign-out); the engine reads it
/// where a hands-free turn is committed. Default true so the engine alone (tests, the lab) behaves as before. Signed out the radio only listens (owner, 2026-11-23), and a switch left on at sign-out sends nothing (review, 2026-11-24).
public final class TurnGate: @unchecked Sendable {
    public static let shared = TurnGate()
    private let lock = NSLock()
    private var value = true
    public var allowed: Bool { get { lock.lock(); defer { lock.unlock() }; return value } set { lock.lock(); value = newValue; lock.unlock() } }
}
