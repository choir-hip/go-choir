import Foundation

// Hands-free is ONE state, on or off (owner, 2026-11-15): on, the microphone is open and the turn policy opens and closes turns by
// itself; off, the microphone is closed, the engine hears nothing, typing still works. No tap-to-start-one-turn, no push-to-talk.
//
// WHEN IT IS ON (owner, 2026-11-15, "voice should only be turned on manually, or manually + on app start and + when user unlocks their
// phone and opens the app"; replaces "kept across launches"): it turns ON when the app is opened or comes back to the foreground, if the
// microphone is granted and the reader has not turned it off in this foreground session, and by the mic button at any time. It turns
// itself OFF after `idleLimit` (3 minutes) with no turn from the reader, and by the mic button. It behaves the same whatever is playing
// (a story, a podcast, a book): playing or pausing neither resets nor stops the timer. When it turns itself off nothing is announced:
// the mic control just shows off. A turn is the reader's when it passed the visibility rule and was sent (a committed hands-free turn or a
// typed question); background sound that never became a turn does not count.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in HandsFreeToggleTests):
//  T1 A fresh launch before the microphone question is answered: OFF; granted: ON, declined: OFF.
//  T2 The microphone is declined and later granted: nothing turns it on until the app is opened again or the reader taps it.
//  T3 Nothing is stored between launches: a launch decides again (the old stored value is ignored).
//  T4 The switch goes OFF in the middle of a turn: the open turn is dropped at once and nothing more is heard (engine `handsFreeSet`).
//  T5 Cancel while the speech goes on (a TV): the turn is dropped, the switch stays ON, and no new turn opens until the speech has paused.
//  T6 Speech that never goes quiet: the turn is closed by its deadline (30 s) and sent as it stands.
//  T7 The listening state shows for voice activity alone (a TV, a cough): it must not (`ListeningVisibility`).
//  T8 Three minutes with no turn: OFF by itself; a turn resets the time; pause or play does not; sound that was never a turn does not.
//  T9 The reader turns it off: it stays off until the next time the app is opened or returns to the foreground.

public final class HandsFreeSwitch: @unchecked Sendable {
    public static let shared = HandsFreeSwitch()
    /// Minutes of no turn from the reader after which voice turns itself off.
    public static let idleLimit: TimeInterval = 180
    private let lock = NSLock()
    private let now: @Sendable () -> Date
    private var on = false, decidedFlag = false, manualOff = false
    private var lastTurn: Date
    /// Told after every change (the app refreshes the bar).
    public var onChange: (@Sendable () -> Void)?
    /// `defaults` is accepted for old callers and not used: nothing is kept across launches.
    public init(defaults: UserDefaults? = nil, now: @escaping @Sendable () -> Date = { Date() }) { self.now = now; lastTurn = now() }

    /// Whether the microphone question has been answered in this launch.
    public var decided: Bool { lock.lock(); defer { lock.unlock() }; return decidedFlag }
    /// On or off. Undecided counts as off: nothing listens before the microphone has been asked about.
    public var isOn: Bool { lock.lock(); defer { lock.unlock() }; return on }
    /// The mic button (the reader's choice, at any time). Off stays off until the app is next opened.
    public func set(_ value: Bool) {
        lock.lock(); on = value; decidedFlag = true; manualOff = !value; if value { lastTurn = now() }; lock.unlock()
        onChange?()
    }
    /// The app was opened or came back to the foreground: a new foreground session. ON when the microphone is granted (a reader who
    /// turned it off in the last session has not decided anything for this one).
    public func appOpened(micGranted: Bool) {
        lock.lock(); decidedFlag = true; manualOff = false; if micGranted { on = true; lastTurn = now() }; lock.unlock()
        onChange?()
    }
    /// The first answer to the microphone question (launch): ON when granted, OFF when declined; a later answer changes nothing (T2).
    public func decideFirstRun(micGranted: Bool) { if !decided { appOpened(micGranted: micGranted) } }
    /// A turn from the reader was sent: the time starts again.
    public func readerSpoke() { lock.lock(); lastTurn = now(); lock.unlock() }
    /// Turns itself off when three minutes have passed with no turn. Returns whether it just did. Silent: no announcement.
    @discardableResult
    public func expireIfIdle() -> Bool {
        lock.lock()
        let expired = on && now().timeIntervalSince(lastTurn) >= Self.idleLimit
        if expired { on = false }
        lock.unlock()
        if expired { onChange?() }
        return expired
    }
}

/// When the listening state is shown (the bar's "talking" state with the live words). Pure, so it is tested without audio.
/// Voice activity alone is never enough (a TV, a cough, a door): the reader has begun saying something meant for the radio when the
/// segment has gone on for a while and has words in it, or when the engine has already taken it as a turn.
public enum ListeningVisibility {
    /// Sustained speech before the words count (the policy's own `holdSpeechMs` for a barge-in is 700).
    public static let minSpeechMs = 700
    /// Recognised words before it counts (a cough or a noise yields none or one).
    public static let minWords = 2

    public static func visible(handsFree: Bool, speechMs: Int, words: Int, addressed: Bool, notATurn: Bool) -> Bool {
        guard handsFree, !notATurn else { return false }
        if addressed { return true }
        return speechMs >= minSpeechMs && words >= minWords
    }
}
