import Foundation

// The voice the radio speaks with: one catalog, one saved choice, one readiness record, one speaker that falls back.
//
// WHY THIS FILE EXISTS (owner QA round 1, 2026-11-15: "it's using the built-in Apple fallback voice"). The trace, read from code:
//  - `StageRegistry.listenerConfiguration()` named the system voice ("avspeech") for the voice stage, so a fresh install spoke with it;
//  - a voice picked on the Components page or by voice setup went through `LabRuntime.setStage`, which changes the configuration in
//    memory only: the next launch built the listener configuration again and the choice was gone;
//  - nothing recorded that a neural voice's files were on the phone, so nothing could prefer it safely (choosing a voice whose files are
//    missing makes FluidAudio download them in the middle of a sentence).
//
// WAYS THIS CAN FAIL (each has a test in VoiceChoiceTests):
//  1. No choice saved (a fresh install): the default is the first neural voice of the catalog, not the system voice.
//  2. The saved file is garbage or names a stage nobody registered: the default is used, nothing throws.
//  3. The neural voice's model is not on the phone yet (not ready): the system voice speaks, and the log says why.
//  4. The neural voice cannot be made, throws before any sound, or makes no sound: that line is spoken by the system voice.
//  5. The neural voice is too slow to a first sound (the model is still compiling on first use): that line is spoken by the system
//     voice, the load goes on in the background, and later lines use the neural voice.
//  6. The neural voice fails partway through a line: what it made is kept, the failure is logged, the next line tries it again.
//  7. The listener cuts in (the caller cancels): nothing more is made, nothing falls back, nothing is logged as a failure.
//  8. A voice that is not one of the neural stages (a test voice, a fixture, the system voice itself) is returned as it is.
//  9. Two speakers at once: each one is its own object, and the models are shared through the stage's own slot.

/// One voice the listener can pick: a stage of the registry and its `voice` parameter, with a short friendly sample.
public struct VoiceOption: Codable, Equatable, Sendable, Identifiable {
    public var id: String
    public var stageID: String
    /// The stage's `voice` parameter; empty for the system voice.
    public var voice: String
    public var name: String
    public var blurb: String
    /// What it says when tapped.
    public var sample: String
    /// The model that must be on the phone first; empty when nothing is downloaded.
    public var modelID: String
    public init(id: String, stageID: String, voice: String, name: String, blurb: String, sample: String, modelID: String) {
        self.id = id; self.stageID = stageID; self.voice = voice; self.name = name; self.blurb = blurb; self.sample = sample; self.modelID = modelID
    }
    public var setting: StageSetting {
        voice.isEmpty ? StageSetting(id: stageID) : StageSetting(id: stageID, params: ["voice": .string(voice)])
    }
    /// "About 350 MB" for a download, "Built in" for the system voice.
    public var sizeLabel: String {
        guard !modelID.isEmpty, let m = KnownModels.managed.first(where: { $0.id == modelID }) else { return "Built in" }
        return SetupCatalog.sizeLabel(m.approxBytes)
    }
}

public enum VoiceCatalog {
    public static let kokoroModel = "kokoro-82m-ane"
    public static let pocketModel = "pocket-tts"
    /// Gradium's Phonon 31M English voice (Apache-2.0), the Core ML pack that FluidInference publishes in `pocket-tts-coreml/phonon/phonon-31m-en`.
    public static let phononModel = "phonon-31m"
    /// The stage that speaks when the neural voice is not ready or fails.
    public static let fallbackStageID = "avspeech"
    /// Stages that need a model on the phone; only these get the readiness check and the fall back.
    public static let neuralStageIDs: Set<String> = ["kokoro", "phonon", "pocket-tts"]

    /// Kokoro 82M voices that FluidInference/kokoro-82m-coreml publishes (`voices/<id>.json`, listed by the Hugging Face API on 2026-11-15)
    /// and the American and British English ones to start with. Pocket's one voice is the stage's own default. The first is the default.
    /// Phonon (an alternative to Kokoro, to be heard beside it) sits right after Kokoro's cards: three of its 103 voices, the ones the pack
    /// ships as ready-made states, so they start without a prefill (names are the pack's own: `<locale>_<Name>`, `voices/` and `voices_catalog.jsonl`,
    /// listed by the Hugging Face API on 2026-11-15). The system voice stays last.
    /// The cards the person is shown: a voice whose model no download group contains is not offered (it could never be fetched).
    public static func offered(by door: DownloadDoor = .shared) -> [VoiceOption] { options.filter { $0.modelID.isEmpty || door.group(ofModel: $0.modelID) != nil } }
    public static let options: [VoiceOption] = [
        VoiceOption(id: "kokoro:af_heart", stageID: "kokoro", voice: "af_heart", name: "Heart", blurb: "Warm and clear",
                    sample: "Hello, I'm Heart. I'll read you the paper.", modelID: kokoroModel),
        VoiceOption(id: "kokoro:af_bella", stageID: "kokoro", voice: "af_bella", name: "Bella", blurb: "Bright and quick",
                    sample: "Hi, I'm Bella. Shall we see what's new today?", modelID: kokoroModel),
        VoiceOption(id: "kokoro:am_adam", stageID: "kokoro", voice: "am_adam", name: "Adam", blurb: "Steady and calm",
                    sample: "Good morning, I'm Adam. Here is your radio.", modelID: kokoroModel),
        VoiceOption(id: "kokoro:bf_emma", stageID: "kokoro", voice: "bf_emma", name: "Emma", blurb: "British, friendly",
                    sample: "Hello, I'm Emma. Lovely to meet you.", modelID: kokoroModel),
        VoiceOption(id: "kokoro:bm_george", stageID: "kokoro", voice: "bm_george", name: "George", blurb: "British, deep",
                    sample: "Good day, I'm George. Let's begin.", modelID: kokoroModel),
        VoiceOption(id: "phonon:en_us_Easton", stageID: "phonon", voice: "en_us_Easton", name: "Easton (Phonon)", blurb: "Young, punchy, American",
                    sample: "Hi, I'm Easton. This is Phonon, a small voice that runs on your phone.", modelID: phononModel),
        VoiceOption(id: "phonon:en_us_Zoey", stageID: "phonon", voice: "en_us_Zoey", name: "Zoey (Phonon)", blurb: "Playful and upbeat, American",
                    sample: "Hi, I'm Zoey. This is Phonon. Shall we see what's new today?", modelID: phononModel),
        VoiceOption(id: "phonon:en_gb_Maeve", stageID: "phonon", voice: "en_gb_Maeve", name: "Maeve (Phonon)", blurb: "Sparky and attentive, British",
                    sample: "Hello, I'm Maeve. This is Phonon. Lovely to meet you.", modelID: phononModel),
        VoiceOption(id: "pocket-tts:alba", stageID: "pocket-tts", voice: "alba", name: "Alba", blurb: "Light and quick to start",
                    sample: "Hello, I'm Alba. I start talking quickly.", modelID: pocketModel),
        VoiceOption(id: "system", stageID: fallbackStageID, voice: "", name: "This phone's voice", blurb: "Built in, no download",
                    sample: "Hello, this is the voice that comes with your phone.", modelID: ""),
    ]
    public static var defaultOption: VoiceOption { options[0] }
    public static func option(id: String) -> VoiceOption? { options.first { $0.id == id } }
    /// The option a saved stage setting means, if it is one of ours.
    public static func option(matching s: StageSetting) -> VoiceOption? {
        var wanted = ""
        if case .string(let t)? = s.params["voice"] { wanted = t }
        let ofStage = options.filter { $0.stageID == s.id }
        // A setting with no voice parameter (the Components page picks a stage only) means the stage's first voice.
        return ofStage.first { $0.voice == wanted } ?? (wanted.isEmpty ? ofStage.first : nil)
    }
}

/// The voice stage setting that survives a relaunch. A fresh install has none, and gets the catalog's default.
public final class VoiceChoiceStore: @unchecked Sendable {
    public let file: URL
    private let lock = NSLock()
    public init(file: URL) { self.file = file }

    /// What was saved, nil when nothing usable is there (no file, garbage, an empty stage id).
    public func saved() -> StageSetting? {
        lock.lock(); defer { lock.unlock() }
        guard let d = try? Data(contentsOf: file), let s = try? JSONDecoder().decode(StageSetting.self, from: d), !s.id.isEmpty else { return nil }
        return s
    }
    /// The setting to speak with: the saved one if the registry knows its stage, else the default.
    public func current(knownStages: Set<String>? = nil) -> StageSetting {
        if let s = saved(), knownStages?.contains(s.id) ?? true { return s }
        return VoiceCatalog.defaultOption.setting
    }
    public func save(_ s: StageSetting) {
        lock.lock(); defer { lock.unlock() }
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: file, options: .atomic) }
    }
}

/// Which models are on the phone and have been loaded and heard to work. Written when a fetch ends in a working model; read before the
/// neural voice is preferred. A model the library had cached by itself (an earlier launch's pick) is not ready until it has been prepared
/// once through setup or the voice cards: a partly downloaded folder is not evidence.
public protocol VoiceReadiness: Sendable {
    func isReady(model: String) -> Bool
}
/// The same record with its writes. ONLY `ReadinessVerifier` holds one (review round 3: ONE readiness writer): the engine, the Downloads page, the voice cards and every other reader have `VoiceReadiness` and cannot write.
public protocol VoiceReadinessRecord: VoiceReadiness {
    /// Mark ready (sealing a library model's files first). false: it could not be sealed, so it is not ready.
    @discardableResult func markReady(model: String) -> Bool
    func markNotReady(model: String)
    /// A check failed: what to tell the person, and what to do with files that cannot be trusted. Called under the door's exclusive slot, before the demotion.
    func diagnose(model: String, note: String, outcome: CheckOutcome) async -> String
}
/// How a check ended: it did not run (cancelled, timed out), it ran and the model failed to load, or it ran and judged the output.
public enum CheckOutcome: Sendable { case neverRan, loadFailed, judged }
public extension VoiceReadinessRecord { func diagnose(model: String, note: String, outcome: CheckOutcome) async -> String { note } }

/// The ONE writer of readiness. A model is marked ready here, after `check` said it works on this phone, and nowhere else; a failed check demotes it. A fetch, a download card or a leftover from an older build cannot make a model ready.
///
/// WAYS THIS CAN GO WRONG (tests in ReadinessVerifierTests):
///  W1 A model whose check fails is not ready, and one that was marked before is demoted.
///  W2 A model whose check throws is not ready.
///  W3 A pass marks the model and returns the measurement; a companion is checked itself (VoiceSetup), never marked on its primary's pass.
public final class ReadinessVerifier: @unchecked Sendable {
    private let record: VoiceReadinessRecord
    public init(record: VoiceReadinessRecord) { self.record = record }
    /// What everything else reads.
    public var reading: VoiceReadiness { record }
    /// Run the check; mark on a pass only. Returns the measurement (a thrown check is a failed one).
    public func verify(model id: String, check: () async throws -> SetupCheck) async -> SetupCheck {
        let result: SetupCheck
        var outcome = CheckOutcome.judged
        do { result = try await check() }
        catch {
            outcome = (error is CancellationError || (error as? DownloadDoor.Refusal) == .timedOut) ? .neverRan : .loadFailed
            result = SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "\(error)")
        }
        if result.outputOK {
            if record.markReady(model: id) { return result }
            var failed = result; failed.outputOK = false; failed.note = "Its files could not be sealed (a file could not be read, or is not a plain file), so it is not marked ready."
            return failed
        }
        var failed = result
        let rec = record
        // The words and any clean-up happen holding the door's slot: no window can be open and no fetch can start in between.
        failed.note = (try? await DownloadDoor.shared.exclusive { await rec.diagnose(model: id, note: result.note, outcome: outcome) }) ?? result.note
        record.markNotReady(model: id)
        return failed
    }
    /// The files of a model are going: it is not ready any more.
    public func demote(model id: String) { record.markNotReady(model: id) }
}

/// In memory only (a virtual engine, tests).
public final class MemoryVoiceReadiness: VoiceReadinessRecord, @unchecked Sendable {
    private let lock = NSLock(); private var ids = Set<String>()
    public init() {}
    public func isReady(model: String) -> Bool { lock.lock(); defer { lock.unlock() }; return ids.contains(model) }
    @discardableResult public func markReady(model: String) -> Bool { lock.lock(); ids.insert(model); lock.unlock(); return true }
    public func markNotReady(model: String) { lock.lock(); ids.remove(model); lock.unlock() }
}

/// Readiness that survives a relaunch, DERIVED from the files: a model is ready only when a check passed for the pin it has NOW (`ModelPins`) and every file of it is on disk. A pin change (an app update) or a missing file makes it not
/// ready with no writer involved. The record is `id -> pin of the pass`.
public final class FileVoiceReadiness: VoiceReadinessRecord, @unchecked Sendable {
    private let file: URL
    private let lock = NSLock()
    private let pins: @Sendable (String) -> ModelPin?
    public init(file: URL, pins: @escaping @Sendable (String) -> ModelPin? = { ModelPins.current($0) }) { self.file = file; self.pins = pins }
    private func read() -> [String: String] {
        guard let d = try? Data(contentsOf: file), let a = try? JSONDecoder().decode([String: String].self, from: d) else { return [:] }     // an older file (a list of ids) is no record: nothing is ready until it passes again
        return a
    }
    private func write(_ s: [String: String]) {
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: file, options: .atomic) }
    }
    public func isReady(model: String) -> Bool {
        lock.lock(); let passed = read()[model]; lock.unlock()
        guard let passed, let p = pins(model), p.pin == passed else { return false }
        return p.present()
    }
    @discardableResult public func markReady(model: String) -> Bool {
        lock.lock(); defer { lock.unlock() }
        guard let p = pins(model) else { return false }
        if let seal = p.seal, !seal() { return false }                 // a library-managed model is sealed in its manifest (every file, exact length and digest) at the pass; nothing to seal, not ready
        var s = read(); s[model] = p.pin; s["~last:" + model] = p.pin; write(s)          // `~last` survives a demotion: it is the pin of the last pass, to tell a layout that changed
        return true
    }
    public func diagnose(model: String, note: String, outcome: CheckOutcome) async -> String {
        if outcome == .neverRan { return "The check did not run (it was cancelled or timed out), so nothing was changed. Try again." }
        guard let p = pins(model), p.purge != nil else { return note.isEmpty ? "It did not load from this phone's files." : note }
        if outcome == .judged { return note }                                                     // it loaded: its files are whole; the check judged the output and says what it found
        // It did not load. Files are kept ONLY when they are proven intact; anything the record cannot vouch for is deleted so the next fetch starts clean (a library that checks only presence would skip it and fail again).
        lock.lock(); let last = read()["~last:" + model]; lock.unlock()
        var words: String; var purge = true
        if let last, last != p.pin { words = "The files of this model were made for an older version of the app (the layout changed), so they were deleted. Download it again." }
        else {
            switch p.proof?() {
            case .intact?: words = "It did not load from this phone's files. They are intact, so downloading it again would not help."; purge = false
            case .unreadable?: words = "Its files could not be read just now, so nothing was deleted. Try again."; purge = false
            case .changed?: words = "Its files on this phone were damaged, so they were deleted. Download it again."
            case nil: words = "It did not load from this phone's files, and nothing proves they were complete, so they were deleted. Download it again."
            }
        }
        if purge {
            await ModelStore.shared.unload(model)                                                  // the manager that loaded over them goes first
            p.purge?()
            lock.lock(); var s = read(); s["~last:" + model] = nil; s[model] = nil; write(s); lock.unlock()
        }
        return words
    }
    public func markNotReady(model: String) {
        lock.lock(); defer { lock.unlock() }
        pins(model)?.unseal?()                                         // a fetch or a failed check deletes the manifest: the files that are still there are not ready
        var s = read(); if s.removeValue(forKey: model) != nil { write(s) }
    }
}

public enum Voices {
    /// The record the app uses (next to the model files).
    public static let verifier = ReadinessVerifier(record: FileVoiceReadiness(file: KnownModels.storeRoot.appendingPathComponent("voice-ready.json")))
    /// Read-only: the engine, the pages and the cards read this; only `verifier` writes it.
    public static var readiness: VoiceReadiness { verifier.reading }
    /// Slower than this to a first sound and the line is spoken by the system voice (the neural model may still be compiling on its first
    /// use; it goes on loading and the next line uses it).
    public static let firstSoundSeconds: Double = 25
}

// MARK: The speaker

enum SpeakFailure: Error, CustomStringConvertible {
    case silent, slow(Double)
    var description: String {
        switch self { case .silent: return "it made no sound"; case .slow(let s): return "no sound within \(Int(s)) seconds" }
    }
}

/// Speaks with the neural voice and, line by line, with the system voice when the neural one cannot. Not a stage anyone selects: the
/// registry's `makeSpeaker` builds it around the configured stage.
public final class SpeakingVoice: Voice, @unchecked Sendable {
    public static let stageID = "speaking-voice"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "The radio's voice", summary: "The chosen neural voice, with the system voice when it cannot speak.")
    }
    private let primary: Voice
    private let name: String
    private let fallback: () throws -> Voice
    private let log: EventLog
    private let deadline: Double

    init(primary: Voice, name: String, fallback: @escaping () throws -> Voice, log: EventLog, firstSoundSeconds: Double = Voices.firstSoundSeconds) {
        self.primary = primary; self.name = name; self.fallback = fallback; self.log = log; self.deadline = firstSoundSeconds
    }

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let (primary, name, fallback, log, deadline) = (self.primary, self.name, self.fallback, self.log, self.deadline)
        return AsyncThrowingStream { cont in
            let task = Task {
                var byPrimary = 0, byFallback = 0
                for await line in text {
                    if Task.isCancelled { break }
                    let one = AsyncStream<String> { $0.yield(line); $0.finish() }
                    let got = Locked<Int>(0)
                    var why: String?
                    do {
                        try await Self.race(primary.speak(one), got: got, deadline: deadline) { cont.yield($0) }
                    } catch is CancellationError { break }
                    catch { why = "\(error)" }
                    if Task.isCancelled { break }                                    // failure 7: a cut is not a failure
                    guard let reason = why else { byPrimary += 1; continue }
                    if got.value > 0 {                                               // failure 6: keep what it made
                        log.emit(.note("voice: \(name) stopped partway (\(reason)); what it made is kept"))
                        byPrimary += 1; continue
                    }
                    log.emit(.note("voice: \(name) could not speak this line (\(reason)); the system voice speaks it"))
                    do {
                        for try await f in try fallback().speak(AsyncStream<String> { $0.yield(line); $0.finish() }) { cont.yield(f) }
                        byFallback += 1
                    } catch { cont.finish(throwing: error); return }
                }
                if byPrimary + byFallback > 0 {
                    log.emit(.note("voice: spoke \(byPrimary) line(s) with \(name)" + (byFallback > 0 ? " and \(byFallback) with the system voice" : "")))
                }
                cont.finish()
            }
            cont.onTermination = { _ in task.cancel() }
        }
    }

    /// Forward `stream`'s frames to `sink`; throw `SpeakFailure.slow` if none has come within `deadline` seconds, `.silent` if it ended
    /// with none, and whatever the stream throws.
    static func race(_ stream: AsyncThrowingStream<AudioFrame, Error>, got: Locked<Int>, deadline: Double, sink: @escaping @Sendable (AudioFrame) -> Void) async throws {
        try await withThrowingTaskGroup(of: Bool.self) { g in
            g.addTask {
                for try await f in stream { if !f.samples.isEmpty { got.write { $0 += f.samples.count }; sink(f) } }
                if got.value == 0 { throw SpeakFailure.silent }
                return true
            }
            g.addTask {
                try await Task.sleep(nanoseconds: UInt64(deadline * 1e9))
                if got.value == 0 { throw SpeakFailure.slow(deadline) }
                return false
            }
            for try await isStream in g { if isStream { g.cancelAll(); break } }
        }
    }
}

public extension StageRegistry {
    /// The voice for speech: the configured stage; when it is a neural one, with the readiness check and the line-by-line fall back to the
    /// system voice. Every place that speaks takes its voice from here, so none of them can ignore the choice.
    func makeSpeaker(_ config: LabConfiguration, log: EventLog, readiness: VoiceReadiness = Voices.readiness,
                     firstSoundSeconds: Double = Voices.firstSoundSeconds) throws -> Voice {
        guard let s = config.setting(.voice), VoiceCatalog.neuralStageIDs.contains(s.id) else { return try make(.voice, as: Voice.self, config) }
        var system = config
        system.set(.voice, StageSetting(id: VoiceCatalog.fallbackStageID))
        let fallback: () throws -> Voice = { [self] in try make(.voice, as: Voice.self, system) }
        if LabEnvironment.virtualAudio { return try fallback() }             // no models and no synthesis under the agent
        let name = VoiceCatalog.option(matching: s)?.id ?? s.id
        guard let registered = stages(.voice).first(where: { $0.id == s.id }) else {
            log.emit(.note("voice: \(name) is not in this build; the system voice speaks"))
            return try fallback()
        }
        let missing = registered.descriptor.needsModels.filter { !readiness.isReady(model: $0) }
        if !missing.isEmpty {                                                // failure 3
            log.emit(.note("voice: \(name) is not ready (\(missing.joined(separator: ", ")) is not on this phone yet); the system voice speaks"))
            return try fallback()
        }
        let primary: Voice
        do { primary = try make(.voice, as: Voice.self, config) }
        catch {
            log.emit(.note("voice: \(name) could not be made (\(error)); the system voice speaks"))
            return try fallback()
        }
        return SpeakingVoice(primary: primary, name: name, fallback: fallback, log: log, firstSoundSeconds: firstSoundSeconds)
    }
}
