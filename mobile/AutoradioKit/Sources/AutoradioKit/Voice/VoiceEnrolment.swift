import Foundation

// Learning the person's voice in voice onboarding (owner, 2026-11-24: "we should learn user voice in voice onboarding"). After the downloads the page asks for a few short sentences read aloud (three, about ten seconds in all), builds the
// profile with the existing gate (SpeakerGate.learn: three mutually alike takes), then asks for one more sentence and checks it against the profile before saying it is stored. The page and its words are Exact (onboarding.ts, voicelearn.ts);
// this file is native only because it takes the microphone's frames and the embedding model (voice). The profile stays on the phone (SpeakerStore). In this build the verdict on later turns is only logged (HandsFree.hfCommit).
//
// WAYS THIS CAN GO WRONG (each has a test in VoiceEnrolmentTests):
//  V1 A take that ends before it has heard speech (a cough, silence): it is not a take; the same sentence is asked again and nothing is kept.
//  V2 A take never ends (a loud room, no pause): it ends at the longest length, with what it has.
//  V3 Three takes of different voices or too little speech: the gate does not call the voice learned; the page says so and starts over, never "stored".
//  V4 The confirming sentence is not heard as the learned voice: "not sure", offered again or learn again; the profile is not changed by the confirmation.
//  V5 "Learn again" or "Forget my voice" deletes the old profile before anything new is built, so a failed second try never leaves a half old, half new profile.
//  V6 No embedding model on the phone: the flow says so at once and takes no audio.

/// Decides when one spoken sentence is over. Pure: no audio, no clock; the frames carry their own length.
public struct TakeCollector: Sendable {
    public static let minSpeechSeconds = 1.2, endSilenceSeconds = 0.7, maxSeconds = 7.0
    /// Frame level (dBFS RMS) above which a frame counts as speech (the first-run microphone check uses the same floor).
    public static let floorDb = MicCheck.floorDb

    public private(set) var samples: [Float] = []
    public private(set) var speech = 0.0, total = 0.0, silence = 0.0, level = 0.0
    public enum End: Equatable, Sendable { case heard, timedOut }
    public private(set) var ended: End?
    public init() {}

    public mutating func feed(_ f: AudioFrame) {
        guard ended == nil, f.sampleRate == busSampleRate else { return }
        let db = f.rmsDb, s = f.duration
        let target = min(1, max(0, (db - MicCheck.meterLowDb) / (MicCheck.meterHighDb - MicCheck.meterLowDb)))
        level = target > level ? level + (target - level) * 0.6 : level + (target - level) * 0.15
        total += s
        if db >= Self.floorDb { speech += s; silence = 0 } else if speech > 0 { silence += s }
        if speech > 0 { samples.append(contentsOf: f.samples) }                      // the lead-in silence is not kept
        if speech >= Self.minSpeechSeconds && silence >= Self.endSilenceSeconds { ended = .heard }
        else if total >= Self.maxSeconds { ended = .timedOut }
    }
    /// V1: a take that ended with too little speech is not a take.
    public var usable: Bool { ended != nil && speech >= 1.0 && samples.count >= 16_000 }
}

public final class VoiceEnrolment: @unchecked Sendable {
    /// Sentences to read before the profile is built; the confirming one is one more.
    public static let learnTakes = 3
    public enum Phase: String, Sendable { case idle, listening, building, confirm, confirming, stored, unsure, failed, noModel }

    private let gate: SpeakerGate
    private let lock = NSLock()
    private var collector = TakeCollector()
    private var takes: [[Float]] = []
    private var phaseValue: Phase = .idle
    private var again = false                      // the last take was unusable: ask the same sentence again
    private var finishing = false                  // a take ended and its step is running: later frames are not another take
    private var call = ""
    public var onChange: (@Sendable () -> Void)?
    public init(gate: SpeakerGate = .shared) { self.gate = gate }

    public var phase: Phase { lock.lock(); defer { lock.unlock() }; return phaseValue }

    /// V5, V6: start over. The old profile is deleted first.
    public func start() {
        guard gate.canListen else { set(.noModel); return }
        gate.forget()
        lock.lock(); takes = []; collector = TakeCollector(); again = false; finishing = false; call = ""; lock.unlock()
        set(.idle)
    }
    /// "Learn again" was opened: the page starts from its first sentence, and the old profile stays until the person starts (`start`).
    public func rewind() {
        lock.lock(); takes = []; again = false; finishing = false; call = ""; collector = TakeCollector(); lock.unlock()
        set(gate.canListen ? .idle : .noModel)
    }
    /// "Forget my voice".
    public func forget() { gate.forget(); lock.lock(); takes = []; again = false; finishing = false; call = ""; lock.unlock(); set(.idle) }

    /// The person is about to read the next sentence (the learning ones, then the confirming one): frames now count.
    public func begin() {
        guard gate.canListen else { set(.noModel); return }
        lock.lock()
        let p = phaseValue
        guard p == .idle || p == .confirm || p == .unsure || p == .failed else { lock.unlock(); return }
        if p == .unsure || p == .failed { takes = []; gate.forget() }
        collector = TakeCollector(); again = false; finishing = false
        lock.unlock()
        set(takesCount >= Self.learnTakes ? .confirming : .listening)
    }
    private var takesCount: Int { lock.lock(); defer { lock.unlock() }; return takes.count }

    /// One microphone frame. When the take ends this schedules the next step.
    public func feed(_ f: AudioFrame) {
        lock.lock()
        guard !finishing, phaseValue == .listening || phaseValue == .confirming else { lock.unlock(); return }
        collector.feed(f)
        let done = collector.ended != nil, c = collector, confirming = phaseValue == .confirming
        if done { finishing = true }
        lock.unlock()
        if !done { return }                                                      // (no change is announced per frame: the page hears phases, not levels)
        Task { await self.finish(c, confirming: confirming) }
    }

    func finish(_ c: TakeCollector, confirming: Bool) async {
        if !c.usable { lock.lock(); again = true; phaseValue = confirming ? .confirm : .idle; lock.unlock(); onChange?(); return }            // V1
        if confirming {
            let r = await gate.judge(c.samples)                                                                                                     // V4: judging changes nothing
            lock.lock(); call = r.rawValue; phaseValue = r == .match ? .stored : .unsure; lock.unlock(); onChange?(); return
        }
        lock.lock(); takes.append(c.samples); let n = takes.count; phaseValue = n >= Self.learnTakes ? .building : .idle; let all = takes; lock.unlock()
        onChange?()
        guard n >= Self.learnTakes else { return }
        for t in all { await gate.learn(t) }
        lock.lock(); phaseValue = gate.learned ? .confirm : .failed; if !gate.learned { takes = [] }; lock.unlock()                                   // V3
        onChange?()
    }

    private func set(_ p: Phase) { lock.lock(); phaseValue = p; lock.unlock(); onChange?() }

    /// What shared code sees. `taken`: sentences read so far toward the profile; `again`: the last sentence was not heard; `level`: 0 to 1, for the meter.
    public func facts() -> [String: Any] {
        lock.lock(); defer { lock.unlock() }
        return ["phase": phaseValue.rawValue, "taken": min(takes.count, Self.learnTakes), "needed": Self.learnTakes, "again": again, "level": collector.level,
                "learned": gate.learned, "ready": gate.canListen, "call": call]
    }
}
