import Foundation

// The reader's voice (lane autoradio-talk, Job 3). A learned voice lets the radio tell the reader from a TV: a hands-free candidate turn
// whose speech does not match the learned voice is not shown as listening and is not sent. Nothing leaves the phone: the profile is a
// few hundred numbers in Application Support (SpeakerStore, excluded from backup), the embeddings are made on the phone, the audio is not
// kept. With no learned voice, or no embedder, behaviour is exactly as before (every turn passes).
//
// The pure parts (the profile, thresholds, enrolment, adaptation) are the Mike speech engine's SpeakerProfile (Voice/SpeakerProfile.swift,
// unchanged). This file adds the decision, the store handle and the seam for a model that makes embeddings.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in SpeakerGateTests):
//  1. Too little speech to enrol (under 1.5 s): nothing is learned, nothing is stored.
//  2. A second speaker among the first turns (a visitor, a TV): the profile learns only from the largest group of three mutually alike
//     turns, so one or two turns of another voice do not become the reader's voice.
//  3. The reader's own voice with a cold, a different microphone: below the threshold by a little: it passes (a mismatch must be clear:
//     `rejectMargin` below the threshold). Erring toward accepting the reader.
//  4. A clear second speaker after the voice is learned: a mismatch; in this build it is only logged (HandsFree.hfCommit), nothing is dropped.
//  5. Short speech (under 1 s), no embedder, a model that failed or returned the wrong size: unknown, which passes.
//  6. "Forget my voice": the stored profile is deleted and every turn passes again at once.
//  7. A profile from another model version on disk: ignored (SpeakerStore.load), the voice is learned again.

public protocol VoiceEmbedder: Sendable {
    /// The model is on the phone and loaded.
    var isReady: Bool { get }
    /// A unit-length 256-number embedding of mono 16 kHz speech (up to 10 s are used), or nil when it cannot be made.
    func embed(_ samples: [Float]) async -> [Float]?
}

public enum SpeakerCall: String, Sendable, Equatable { case unknown, match, mismatch }

public final class SpeakerGate: @unchecked Sendable {
    public static let shared = SpeakerGate(url: SpeakerStore.url)
    /// A mismatch must be this far below the model's own threshold to reject (a conservative choice: err toward accepting the reader).
    public static let rejectMargin: Float = 0.08
    /// The audio a decision is made from: at most this much, from the start of the turn.
    public static let maxSamples = 160_000

    private let lock = NSLock()
    private var profile: SpeakerProfile
    private let url: URL?
    private let rule = SpeakerRule()
    private var embedderBox: VoiceEmbedder?
    public var onChange: (@Sendable () -> Void)?

    /// `url`: where the profile is kept (nil: only in memory, for tests). The shared gate keeps it in Application Support.
    public init(url: URL? = nil, embedder: VoiceEmbedder? = nil) {
        self.url = url
        profile = url.map { SpeakerStore.load(from: $0) } ?? SpeakerProfile()
        embedderBox = embedder
    }
    public static func standard() -> SpeakerGate { SpeakerGate(url: SpeakerStore.url) }

    public var embedder: VoiceEmbedder? { get { lock.lock(); defer { lock.unlock() }; return embedderBox } set { lock.lock(); embedderBox = newValue; lock.unlock(); onChange?() } }
    /// There is a model that can listen, so the engine keeps the turn's audio.
    public var canListen: Bool { embedder?.isReady ?? false }
    public var learned: Bool { lock.lock(); defer { lock.unlock() }; return profile.learned }
    /// Turns heard so far toward learning (0 to 3), for the Settings row.
    public var heardTurns: Int { lock.lock(); defer { lock.unlock() }; return min(profile.candidates.count, SpeakerProfile.enrolTurns) }
    public static var turnsNeeded: Int { SpeakerProfile.enrolTurns }

    /// The decision, pure: unknown passes, match passes, a mismatch rejects only when it is clearly below the threshold.
    static func call(for verdict: SpeakerProfile.Verdict) -> SpeakerCall {
        switch verdict.outcome {
        case .unknown: return .unknown
        case .match: return .match
        case .mismatch: return (verdict.margin ?? 0) <= -rejectMargin ? .mismatch : .unknown
        }
    }

    /// Whether this speech sounds like the reader. `unknown` with no learned voice, no model, or too little speech.
    public func judge(_ samples: [Float]) async -> SpeakerCall {
        guard learned, let e = embedder, e.isReady, samples.count >= 16_000 else { return .unknown }
        guard let v = await e.embed(Array(samples.prefix(Self.maxSamples))), v.count == SpeakerModelKind.wespeaker.dimensions else { return .unknown }
        lock.lock(); let p = profile; lock.unlock()
        return Self.call(for: p.check([.wespeaker: v], audioMs: samples.count / 16, rule: rule))
    }

    /// Learn from speech the reader was heard saying (a turn that was answered and not cancelled, or an enrolment utterance).
    public func learn(_ samples: [Float]) async {
        guard let e = embedder, e.isReady, samples.count >= SpeakerProfile.minimumLearnMs * 16 else { return }
        guard let v = await e.embed(Array(samples.prefix(Self.maxSamples))), v.count == SpeakerModelKind.wespeaker.dimensions else { return }
        lock.lock()
        let changed = profile.learn([.wespeaker: v], audioMs: samples.count / 16, rule: rule)
        let snapshot = profile
        lock.unlock()
        if changed { if let u = url { SpeakerStore.save(snapshot, to: u) }; onChange?() }
    }

    /// "Forget my voice" and the start of "Learn my voice": the profile is deleted.
    public func forget() {
        lock.lock(); profile = SpeakerProfile(); lock.unlock()
        if let u = url { SpeakerStore.remove(at: u) }
        onChange?()
    }
}

/// What the Settings row shows.
public struct VoiceProfileInfo: Codable, Sendable, Equatable {
    public var learned = false, ready = false
    public var heard = 0, needed = SpeakerProfile.enrolTurns
    public var line = "", credit = ""
    public init() {}
}
extension SpeakerGate {
    public func info() -> VoiceProfileInfo {
        var i = VoiceProfileInfo()
        i.learned = learned; i.heard = heardTurns; i.ready = canListen
        i.credit = WeSpeakerEmbedder.attribution
        if !i.ready { i.line = "The voice model is still being fetched (about 8 MB). Until then every voice is accepted." }
        else if i.learned { i.line = "The radio notes whether each turn sounds like you (written to the log only for now; nothing is ignored). Kept on this phone only, never uploaded." }
        else { i.line = "Say a few things to the radio and it learns your voice. Kept on this phone only, never uploaded." }
        return i
    }
}
