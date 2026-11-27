import Foundation

// Hands-free is ONE state, on or off (owner, 2026-11-15): on, the microphone is open and the turn policy opens and closes turns by
// itself; off, the microphone is closed, the engine hears nothing, typing still works. No tap-to-start-one-turn, no push-to-talk.
//
// WHEN IT IS ON (owner, 2026-11-16: "it's off unless you turn it on"; replaces the 2026-11-15 rule that turned it on at launch and at
// every return to the foreground): OFF at launch, after sign-in, on return to the foreground, when playback starts, on a route change,
// after an audio interruption, always. Only the listener's tap on the Voice control (`set(true)`) turns it on. Once on it stays on until
// `idleLimit` (3 minutes) has passed since the listener last spoke, then turns itself off silently; the tap turns it off at any time.
// Nothing is stored between launches. It behaves the same whatever is playing. "The listener spoke" is a turn that passed the visibility
// rule and was sent (a committed hands-free turn or a typed question); the assistant speaking, and background sound that never became a
// turn, do not restart the timer.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in HandsFreeToggleTests):
//  V1 to V4, V9 Anything but the tap turns it on: a launch, a foreground, a granted microphone, a stored "on": it must stay off.
//  T4 The switch goes OFF in the middle of a turn: the open turn is dropped at once and nothing more is heard (engine `handsFreeSet`).
//  T5 Cancel while the speech goes on (a TV): the turn is dropped, the switch stays ON, and no new turn opens until the speech has paused.
//  T6 Speech that never goes quiet: the turn is closed by its deadline (30 s) and sent as it stands.
//  T7 The listening state shows for voice activity alone (a TV, a cough): it must not (`ListeningVisibility`).
//  V6, V7 Three minutes since the listener last spoke: OFF by itself; each turn restarts the time; pause or play does not.

public final class HandsFreeSwitch: @unchecked Sendable {
    public static let shared = HandsFreeSwitch()
    /// Minutes of no turn from the reader after which voice turns itself off.
    public static let idleLimit: TimeInterval = 180
    private let lock = NSLock()
    private let now: @Sendable () -> Date
    private var on = false
    private var lastTurn: Date
    /// Told after every change (the app refreshes the bar).
    public var onChange: (@Sendable () -> Void)?
    /// Told after every change too, for the audio session (the microphone is part of it only while voice is on). Separate from `onChange`,
    /// which the app owns.
    public var onVoiceChange: (@Sendable () -> Void)?
    /// `defaults` is accepted for old callers and not used: nothing is kept across launches.
    public init(defaults: UserDefaults? = nil, now: @escaping @Sendable () -> Date = { Date() }) { self.now = now; lastTurn = now() }

    /// On or off. Off until the listener taps the Voice control.
    public var isOn: Bool { lock.lock(); defer { lock.unlock() }; return on }
    /// The Voice control: the listener's explicit choice, the only thing that turns it on. Turning it on starts the quiet time.
    public func set(_ value: Bool) {
        if value, !TurnGate.shared.accountOpen { return }      // the one place every path to "on" passes (a take's give-back too): signed out or signing out, the microphone is never turned on
        lock.lock(); on = value; if value { lastTurn = now() }; lock.unlock()
        onChange?(); onVoiceChange?()
    }
    /// A turn from the reader was sent: the time starts again.
    public func readerSpoke() { lock.lock(); lastTurn = now(); lock.unlock() }
    /// Turns itself off when three minutes have passed since the listener last spoke. Returns whether it just did. Silent: no announcement.
    @discardableResult
    public func expireIfIdle() -> Bool {
        lock.lock()
        let expired = on && now().timeIntervalSince(lastTurn) >= Self.idleLimit
        if expired { on = false }
        lock.unlock()
        if expired { onChange?(); onVoiceChange?() }
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
