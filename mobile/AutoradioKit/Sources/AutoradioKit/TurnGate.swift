import Foundation

/// Whether a turn may be sent at all: only with an account's key, and never while the account is being signed out. Pure to read: nothing a read does changes it; it changes only by the events below.
/// One sign-out EPOCH (`closeForSignOut` bumps it, synchronously, as sign-out's first statement): every turn entry captures it and every later stage (commit, `runTurn`, the pipeline, applying an answer, the key read in `ChoirClient.request`) asks `admits` and drops
/// silently when it moved. Reopening is not an event of the gate's own: natively there is no "sign-in completed" call (a sign-in is shared code writing the Exact secret), so the gate compares WHO is signed in with who was signed out: open again only for a key
/// whose identity differs from the one that left (a stale write that restores the old key leaves it shut, and so does a key that could not be deleted). `value` and `holds` are what the screens and tests set; default open so the engine alone (tests, the lab) behaves as before.
public final class TurnGate: @unchecked Sendable {
    public static let shared = TurnGate()
    private let lock = NSLock()
    private var value = true, holds = 0, epochN = 0, out = false, left: String?
    public var keyPresent: (@Sendable () -> Bool)?
    /// Who is signed in (the key's id, or an in-memory stand-in for a key with none); nil when nobody is.
    public var keyIdentity: (@Sendable () -> String?)?
    public var epoch: Int { lock.lock(); defer { lock.unlock() }; return epochN }
    /// Sign-out has begun and not finished, or the key that left is still (or again) the one stored.
    public var signedOutShut: Bool { lock.lock(); let o = out, l = left, id = keyIdentity; lock.unlock(); return o || (l != nil && id?() == l) }
    /// The key that left was replaced by another (a new sign-in) while sign-out ran: it and its stores are not sign-out's to delete.
    public var keyReplaced: Bool { lock.lock(); let l = left, id = keyIdentity; lock.unlock(); if let l, let now = id?() { return now != l }; return false }
    /// The account side of the gate (a screen's `hold` is not part of it: a typed question works during the microphone check).
    public var accountOpen: Bool { lock.lock(); let v = value, k = keyPresent; lock.unlock(); return v && !signedOutShut && (k?() ?? true) }
    public var allowed: Bool { get { lock.lock(); let h = holds; lock.unlock(); return h == 0 && accountOpen } set { lock.lock(); value = newValue; lock.unlock() } }
    public func admits(_ e: Int) -> Bool { epoch == e && accountOpen }
    /// The first statement of sign-out (idempotent while one runs): the epoch moves and the identity that is leaving is remembered.
    public func closeForSignOut() { let id = keyIdentity?(); lock.lock(); if !out { out = true; epochN += 1; left = id }; lock.unlock() }
    /// Sign-out's own sequence is over, whatever came of it: what stays shut is decided by identity alone.
    public func finishSignOut() { lock.lock(); out = false; lock.unlock() }
    /// What the app wires once at launch (OnboardingController.init): the gate reads the app's key store as it is at the time of each read. Unwired (the engine alone, tests, the lab) it reads no key and stays open.
    public func wire() {
        keyPresent = { ChoirEnvironment.shared.keys.get(KeyAccount.apiKey) != nil }
        keyIdentity = { let k = ChoirEnvironment.shared.keys; return k.get(KeyAccount.keyID).flatMap { $0.isEmpty ? nil : "id:" + $0 } ?? k.get(KeyAccount.apiKey).map { "k:\($0.hashValue)" } }
    }
    /// Tests only: forget a previous test's sign-out, its readers and its settings.
    func resetForTests() { lock.lock(); value = true; holds = 0; out = false; left = nil; lock.unlock(); keyPresent = nil; keyIdentity = nil }
    /// A screen that listens for its own check (the first run's microphone check) holds turns back for as long as it listens; every `hold` has its `release`.
    public func hold() { lock.lock(); holds += 1; lock.unlock() }
    public func release() { lock.lock(); holds = max(0, holds - 1); lock.unlock() }
}
