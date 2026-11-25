import Foundation

// The voice cards: tap a voice, hear a short line in it at once, and that voice is the one the radio speaks with from then on.
//
// WAYS THIS CAN FAIL (each has a test in VoiceChoiceTests):
//  1. A tap while another sample is playing: the first stops (the host's `halt`), only the newest plays; a late finish of the old one
//     changes nothing.
//  2. A voice whose model is not on the phone: its card shows progress while the model downloads, the voice is chosen at once (the
//     radio speaks with bundled Tiny Voca until the model is ready, `makeSpeaker`), and the sample plays when the download ends, unless
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
    /// The model it is a voice of: its id (for a stable name in the page), its name, and the credit its licence asks for. `model` is empty for
    /// the bundled voice, which needs no download.
    public var model: String = ""
    public var group: String = ""
    public var credit: String = ""
}

public final class VoiceSampler: @unchecked Sendable {
    /// Make the sample line in this voice and play it; returns when it has been heard, throws when the voice could not speak.
    public typealias Say = @Sendable (_ option: VoiceOption) async throws -> Void

    public let options: [VoiceOption]
    private let readiness: VoiceReadiness
    /// model id -> progress (0...1) of the download its own row is running, or nil.
    private let running: @Sendable (String) -> Double?
    private let say: Say
    private let halt: @Sendable () -> Void
    private let choose: @Sendable (VoiceOption) -> Void

    private struct State {
        var selected: String
        var generation = 0
        var playing: String?
        var sample: Task<Void, Never>?
        var sampleFailed: [String: String] = [:] // option id -> words
    }
    private let st: Locked<State>
    public var onChange: (@Sendable () -> Void)?

    public init(options: [VoiceOption] = VoiceCatalog.options, selected: String, readiness: VoiceReadiness, say: @escaping Say,
                halt: @escaping @Sendable () -> Void, choose: @escaping @Sendable (VoiceOption) -> Void,
                running: @escaping @Sendable (String) -> Double? = { _ in nil }) {
        self.options = options; self.readiness = readiness; self.running = running; self.say = say; self.halt = halt; self.choose = choose
        st = Locked(State(selected: selected))
    }

    public var selectedID: String { st.value.selected }
    /// The choice was changed elsewhere (the Components page): the cards show it. Plays nothing.
    public func syncSelected(_ id: String) {
        guard options.contains(where: { $0.id == id }) else { return }
        let moved = st.write { s -> Bool in let was = s.selected != id; s.selected = id; return was }
        if moved { onChange?() }
    }

    public func cards() -> [VoiceCardState] {
        let s = st.value
        return options.map { o in
            var state = "ready", progress = 1.0, note = ""
            if !o.modelID.isEmpty, !readiness.isReady(model: o.modelID) {
                if let p = running(o.modelID) { state = "downloading"; progress = min(0.99, max(0, p)); note = "Downloading \(o.sizeLabel)" }
                else { state = "needs"; progress = 0; note = "Download \(o.group.isEmpty ? "its model" : o.group) (\(o.sizeLabel)) above to hear this voice." }
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
        onChange?()
    }

    /// The listener pressed Test on a voice: its sample plays and nothing is chosen (Use is `pick`). A voice whose model is not on the phone plays nothing: its row says to download the model.
    public func hear(_ id: String) {
        guard let o = options.first(where: { $0.id == id }), o.modelID.isEmpty || readiness.isReady(model: o.modelID) else { return }
        let (oldSample, gen): (Task<Void, Never>?, Int) = st.write { s in
            let t = s.sample; s.sample = nil; s.generation += 1; s.playing = nil; s.sampleFailed[o.id] = nil
            return (t, s.generation)
        }
        oldSample?.cancel(); halt()
        startSample(o, gen)
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
                self.st.write { s in if s.generation == gen { s.sampleFailed[o.id] = "That voice could not play just now. The radio will use its built-in voice until it can." } }
            }
            self.st.write { s in if s.generation == gen, s.playing == o.id { s.playing = nil } }
            self.onChange?()
        }
        let keep = st.write { s -> Bool in if s.generation == gen { s.sample = task; return true }; return false }
        if !keep { task.cancel() }
    }
}
