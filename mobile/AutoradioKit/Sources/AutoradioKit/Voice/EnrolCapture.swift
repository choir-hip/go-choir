import Foundation

// Voice enrolment, the native part (owner, 2026-11-24: "we should learn user voice in voice onboarding"; "convert everything to exact that can be converted"). What stays native is only what needs the microphone's frames and
// the embedding model: ending one spoken sentence (`TakeCollector`), embedding the takes into a profile held IN MEMORY, comparing the confirming sentence with it, and storing it. WHICH sentence comes next, how many takes there are,
// what the page shows and when it says "stored" is the page's logic and lives in Exact (AutoradioLab/voicelearn.ts); this class answers its calls and reports facts.
//
// WAYS THIS CAN GO WRONG (each has a test in EnrolCaptureTests):
//  V1 A take that ends before it has heard speech, or with under 1.5 s of it (the floor the profile learns from): not usable, nothing kept.
//  V2 A take never ends (a loud room) or the microphone gives no frames: it ends at the longest length, or when the page's timer calls `expire`, as not usable.
//  V3 The profile is built in memory and stored ONLY when the confirming sentence matched: a mismatched, unsure or abandoned enrolment leaves no profile on disk and the stored one (if any) untouched.
//  V4 Three takes of different voices do not make a profile.
//  V5 Frames after a take ended, or with no take begun, are not kept (no audio of the person outlives its use: `discard` drops every take).
//  V6 No embedding model on the phone: nothing is captured.
public final class EnrolCapture: @unchecked Sendable {
    public enum State: String, Sendable { case idle, listening, done, error }
    private let gate: SpeakerGate
    private let lock = NSLock()
    private var collector = TakeCollector()
    private var stateValue: State = .idle
    private var reason = ""
    private var takes: [[Float]] = []
    private var candidate: SpeakerProfile?
    public var onChange: (@Sendable () -> Void)?
    public init(gate: SpeakerGate = .shared) { self.gate = gate }

    public var state: State { lock.lock(); defer { lock.unlock() }; return stateValue }

    /// One more sentence is about to be read: frames count from now. False (and `error`) when there is no model (V6).
    @discardableResult
    public func begin() -> Bool {
        guard gate.canListen else { set(.error, "The voice model is not on this phone yet."); return false }
        lock.lock(); collector = TakeCollector(); reason = ""; lock.unlock()
        set(.listening)
        return true
    }
    /// One microphone frame. The take ends by itself (a pause after speech, or the longest length).
    public func feed(_ f: AudioFrame) {
        lock.lock()
        guard stateValue == .listening else { lock.unlock(); return }                    // V5
        collector.feed(f)
        let ended = collector.ended != nil
        if ended { stateValue = .done }
        lock.unlock()
        if ended { onChange?() }
    }
    /// The page's timer: nothing ended the take (no frames came). It ends as a take that was not heard (V2).
    public func expire() {
        lock.lock(); let was = stateValue == .listening; if was { stateValue = .done }; lock.unlock()
        if was { onChange?() }
    }
    /// The microphone could not be had (refused, restricted, or the engine did not start): the page says so.
    public func fail(_ why: String) { set(.error, why) }

    /// The take that just ended was heard well enough to learn from (V1).
    public var usable: Bool { lock.lock(); defer { lock.unlock() }; return stateValue == .done && collector.usable }

    /// Keep the finished take for the profile; returns how many are kept. Not usable: nothing is kept.
    @discardableResult
    public func keep() -> Int {
        lock.lock(); defer { lock.unlock() }
        if stateValue == .done, collector.usable { takes.append(collector.samples) }
        return takes.count
    }
    /// Build the candidate profile from the kept takes, in memory (V3, V4). True when they make one voice.
    public func build() async -> Bool {
        lock.lock(); let t = takes; lock.unlock()
        let p = await gate.buildCandidate(t)
        lock.lock(); candidate = p; lock.unlock()
        onChange?()
        return p != nil
    }
    /// The confirming sentence (the take that just ended) against the candidate: "match", "mismatch" or "unknown".
    public func check() async -> String {
        lock.lock(); let c = candidate, s = collector.samples, ok = stateValue == .done && collector.usable; lock.unlock()
        guard let c, ok else { return "unknown" }
        return await gate.judge(s, against: c).rawValue
    }
    /// Store the candidate as the person's voice. Only after `check` said "match" (the page decides); false when there is none.
    @discardableResult
    public func store() -> Bool {
        lock.lock(); let c = candidate; lock.unlock()
        guard let c else { return false }
        gate.adopt(c)
        discard()
        return true
    }
    /// The take that just ended was not usable (or was used): ready for the next, the kept takes and the candidate stay.
    public func dropTake() {
        lock.lock(); collector = TakeCollector(); reason = ""; stateValue = .idle; lock.unlock()
        onChange?()
    }
    /// Drop everything in progress: the takes, the candidate, the take being heard. The stored voice is not touched.
    public func discard() {
        lock.lock(); takes = []; candidate = nil; collector = TakeCollector(); reason = ""; stateValue = .idle; lock.unlock()
        onChange?()
    }

    private func set(_ s: State, _ why: String = "") { lock.lock(); stateValue = s; reason = why; lock.unlock(); onChange?() }

    /// What shared code sees: the capture's state and level, whether the finished take is usable, how many takes are kept, whether a candidate exists, whether there is a model.
    public func facts() -> [String: Any] {
        lock.lock(); defer { lock.unlock() }
        return ["state": stateValue.rawValue, "level": collector.level, "usable": stateValue == .done && collector.usable, "kept": takes.count, "built": candidate != nil,
                "ready": gate.canListen, "reason": reason]
    }
}
