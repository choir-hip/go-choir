import Foundation

// Hands-free is ONE state, on or off (owner, 2026-11-15): on, the microphone is open and the turn policy opens and closes turns by
// itself; off, the microphone is closed, the engine hears nothing, typing still works. Kept across launches. There is no tap-to-start-
// one-turn and no push-to-talk.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in HandsFreeToggleTests):
//  T1 A fresh install has no stored value: it stays undecided until the microphone question is answered; granted: ON, declined: OFF.
//  T2 The microphone is declined and later granted in Settings: the stored OFF is kept (the reader turns it on; nothing turns it on).
//  T3 The value is lost between launches: it is stored in UserDefaults at once, and read back the same.
//  T4 The switch goes OFF in the middle of a turn: the open turn is dropped at once and nothing more is heard (engine `handsFreeSet`).
//  T5 Cancel while the speech goes on (a TV): the turn is dropped, the switch stays ON, and no new turn opens until the speech has paused.
//  T6 Speech that never goes quiet: the turn is closed by its deadline (30 s) and sent as it stands.
//  T7 The listening state shows for voice activity alone (a TV, a cough): it must not (`ListeningVisibility`).

public final class HandsFreeSwitch: @unchecked Sendable {
    public static let shared = HandsFreeSwitch()
    static let key = "AutoradioHandsFree"
    private let defaults: UserDefaults
    private let lock = NSLock()
    /// Told after every change (the app refreshes the bar).
    public var onChange: (@Sendable () -> Void)?
    public init(defaults: UserDefaults = .standard) { self.defaults = defaults }

    /// Whether the reader (or the microphone answer) has decided yet.
    public var decided: Bool { lock.lock(); defer { lock.unlock() }; return defaults.object(forKey: Self.key) != nil }
    /// On or off. Undecided counts as off: nothing listens before the microphone has been asked about.
    public var isOn: Bool { lock.lock(); defer { lock.unlock() }; return defaults.object(forKey: Self.key) != nil && defaults.bool(forKey: Self.key) }
    public func set(_ on: Bool) {
        lock.lock(); defaults.set(on, forKey: Self.key); lock.unlock()
        onChange?()
    }
    /// The first answer to the microphone question decides the default (T1); a later answer never overrides it (T2).
    public func decideFirstRun(micGranted: Bool) { if !decided { set(micGranted) } }
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
