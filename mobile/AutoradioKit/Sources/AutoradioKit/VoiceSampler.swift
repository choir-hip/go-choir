import Foundation

// The voice cards: tap a voice, hear a short line in it at once, and that voice is the one the radio speaks with from then on.
//
// WAYS THIS CAN FAIL (each has a test in VoiceChoiceTests):
//  1. A tap while another sample is playing: the first stops (the host's `halt`), only the newest plays; a late finish of the old one
//     changes nothing.
//  2. A voice whose model is not on the phone: its card shows progress while the model downloads, the voice is chosen at once (the
//     radio speaks with the system voice until the model is ready, `makeSpeaker`), and the sample plays when the download ends, unless
//     the listener tapped something else meanwhile.
//  3. Two voices share one model (all Kokoro voices): one download serves every card of it, and a second tap during it does not start a
//     second download.
//  4. The download fails: the card says so in plain words and can be tapped again; nothing is marked ready.
//  5. The sample fails to play (model load error): the card says so, the choice stays as tapped, the radio falls back by itself.
//  6. The listener leaves the page: `stop` halts the sample; a download in flight goes on (it resumes, it is not wasted).
//  7. An id that is not on the cards: ignored.

public struct VoiceCardState: Codable, Equatable, Sendable {
    public var id: String
    public var name: String
    public var blurb: String
    public var sizeLabel: String
    /// ready | needs | downloading | failed
    public var state: String
    public var progress: Double
    public var selected: Bool
    public var playing: Bool
    public var note: String
    /// The model it is a voice of: its id (for a stable name in the page), its name, and the credit its licence asks for. All empty for the system voice.
    public var model: String = ""
    public var group: String = ""
    public var credit: String = ""
}

public final class VoiceSampler: @unchecked Sendable {
    /// Fetch and load the model; `onBytes` receives bytes on disk so far.
    public typealias Fetch = @Sendable (_ modelID: String, _ onBytes: @escaping @Sendable (Int64) -> Void) async throws -> Void
    /// Make the sample line in this voice and play it; returns when it has been heard, throws when the voice could not speak.
    /// Run the check on the fetched model and say whether it works on this phone. The host's one verifier (`ReadinessVerifier`) writes readiness inside it; the sampler can only read it.
    public typealias Verify = @Sendable (_ modelID: String) async -> Bool
    public typealias Say = @Sendable (_ option: VoiceOption) async throws -> Void

    public let options: [VoiceOption]
    private let readiness: VoiceReadiness
    private let fetch: Fetch
    private let verify: Verify
    private let say: Say
    private let halt: @Sendable () -> Void
    private let choose: @Sendable (VoiceOption) -> Void
    private let approxBytes: @Sendable (String) -> Int64

    private struct State {
        var selected: String
        var generation = 0
        var playing: String?
        var sample: Task<Void, Never>?
        var downloads = Set<String>()                // models being fetched
        var bytes: [String: Int64] = [:]
        var failed: [String: String] = [:]      // model id -> words
        var sampleFailed: [String: String] = [:] // option id -> words
    }
    private let st: Locked<State>
    public var onChange: (@Sendable () -> Void)?

    public init(options: [VoiceOption] = VoiceCatalog.options, selected: String, readiness: VoiceReadiness, fetch: @escaping Fetch, verify: @escaping Verify, say: @escaping Say,
                halt: @escaping @Sendable () -> Void, choose: @escaping @Sendable (VoiceOption) -> Void,
                approxBytes: @escaping @Sendable (String) -> Int64 = { id in KnownModels.managed.first { $0.id == id }?.approxBytes ?? 0 }) {
        self.options = options; self.readiness = readiness; self.fetch = fetch; self.verify = verify; self.say = say; self.halt = halt; self.choose = choose
        self.approxBytes = approxBytes
        st = Locked(State(selected: selected))
    }

    public var selectedID: String { st.value.selected }
    /// The choice was changed elsewhere (the Components page): the cards show it. Plays nothing.
    public func syncSelected(_ id: String) { if options.contains(where: { $0.id == id }) { st.write { $0.selected = id } } }

    public func cards() -> [VoiceCardState] {
        let s = st.value
        return options.map { o in
            var state = "ready", progress = 1.0, note = ""
            if !o.modelID.isEmpty, !readiness.isReady(model: o.modelID) {
                if s.downloads.contains(o.modelID) {
                    state = "downloading"
                    let total = max(1, approxBytes(o.modelID))
                    progress = min(0.99, Double(s.bytes[o.modelID] ?? 0) / Double(total))
                    note = "Downloading \(o.sizeLabel)"
                } else if let f = s.failed[o.modelID] { state = "failed"; progress = 0; note = f }
                else { state = "needs"; progress = 0; note = "\(o.sizeLabel) to download" }
            }
            if let f = s.sampleFailed[o.id], state == "ready" { note = f }
            return VoiceCardState(id: o.id, name: o.name, blurb: o.blurb, sizeLabel: o.sizeLabel, state: state, progress: progress,
                                  selected: s.selected == o.id, playing: s.playing == o.id, note: note, model: o.modelID, group: o.group, credit: o.credit)
        }
    }

    /// The listener tapped a card.
    public func pick(_ id: String) {
        guard let o = options.first(where: { $0.id == id }) else { return }       // failure 7
        let (oldSample, gen): (Task<Void, Never>?, Int) = st.write { s in
            let t = s.sample; s.sample = nil; s.generation += 1; s.playing = nil; s.selected = o.id
            s.sampleFailed[o.id] = nil
            return (t, s.generation)
        }
        oldSample?.cancel(); halt()                                                 // failure 1
        choose(o)
        if o.modelID.isEmpty || readiness.isReady(model: o.modelID) { startSample(o, gen) }
        else { startDownload(o, gen) }
        onChange?()
    }

    /// The listener left the page.
    public func stop() {
        let t: Task<Void, Never>? = st.write { s in let t = s.sample; s.sample = nil; s.generation += 1; s.playing = nil; return t }
        t?.cancel(); halt()
        onChange?()
    }

    private func startSample(_ o: VoiceOption, _ gen: Int) {
        st.write { s in if s.generation == gen { s.playing = o.id } }
        let task = Task { [weak self] in
            guard let self else { return }
            do { try await self.say(o) }
            catch is CancellationError {}
            catch {
                self.st.write { s in if s.generation == gen { s.sampleFailed[o.id] = "That voice could not play just now. The radio will use the phone's voice until it can." } }
            }
            self.st.write { s in if s.generation == gen, s.playing == o.id { s.playing = nil } }
            self.onChange?()
        }
        let keep = st.write { s -> Bool in if s.generation == gen { s.sample = task; return true }; return false }
        if !keep { task.cancel() }
    }

    private func startDownload(_ o: VoiceOption, _ gen: Int) {
        let model = o.modelID
        let started = st.write { s -> Bool in
            s.failed[model] = nil
            if !s.downloads.insert(model).inserted { return false }                 // failure 3: one download per model
            s.bytes[model] = 0
            return true
        }
        if started {
            Task { [weak self] in
                guard let self else { return }
                var failure: String?
                do {
                    try await self.fetch(model) { n in self.st.write { $0.bytes[model] = n }; self.onChange?() }
                    if !(await self.verify(model)) { failure = "That voice did not work on this phone. The radio will use the phone's voice." }
                } catch let r as DownloadDoor.Refusal {
                    failure = r.person
                } catch is CancellationError { failure = "The download was stopped. Tap to try again." }
                catch { failure = "The download did not finish. Check the connection and tap to try again." }   // failure 4
                self.st.write { s in s.downloads.remove(model); if let f = failure { s.failed[model] = f } }
                // The sample plays when the download ends, unless the listener has tapped something else since.
                // The one wanted now (it may be another voice of the same model than the first tap), unless the listener moved on to a voice of
                // another model.
                if failure == nil {
                    let want = self.st.write { s -> (VoiceOption, Int)? in
                        guard let cur = self.options.first(where: { $0.id == s.selected }), cur.modelID == model else { return nil }
                        return (cur, s.generation)
                    }
                    if let (cur, again) = want { self.startSample(cur, again) }
                }
                self.onChange?()
            }
        }
    }
}
