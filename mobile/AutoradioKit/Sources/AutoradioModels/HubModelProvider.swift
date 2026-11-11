import Foundation
import AutoradioKit

// The one path by which a model gets onto a phone and proves itself, used by voice setup on the phone and by
// `autoradio-models --setup` on the Mac: fetch (the library downloads and checksums its Hugging Face files; the pinned GGUF is
// downloaded and SHA-256 checked by the kit's ModelStore), load (measured), then run the model and MEASURE it: load time, memory in
// use and left (`os_proc_available_memory()` on the phone), the phone's temperature, and for a language model the tokens per second
// reading and writing, for a voice the time to the first sound. Setup chooses from those numbers.
//
// WAYS THIS CAN FAIL (each is a distinct message in the row, none of them "not registered"):
//  1. The id is not in the catalog: setup's own test makes that impossible; here it throws a bad-input error naming the id.
//  2. The download stalls or the network drops: the library throws, or no bytes arrive (setup's watchdog cancels the transfer).
//  3. The model loads but the phone has too little memory: the process is killed (the check cannot report it); setup's ladder
//     starts at the first model that fits the device's own reading (SetupCatalog.plan), the floor of the defence.
//  4. The model loads and produces wrong output (silence, no speech found, the wrong words, no sentence): outputOK false with a note.
//  5. The build has no llama.cpp, or llama.cpp cannot run the file's architecture: the check says so and setup moves on.
//  6. The model is too slow or leaves too little memory: the numbers say so and setup moves on (LiveSetupBackend.check).
public final class HubModelProvider: ModelProvider, @unchecked Sendable {
    public static let shared = HubModelProvider()

    public func fetch(_ id: String, role: String, onBytes: @escaping @Sendable (Int64) -> Void) async throws {
        guard KnownModels.contains(id) else { throw LabError.badInput("setup asked for \(id), which is not in the model catalog") }
        // The libraries do not report bytes in one shape, so the bytes on disk are watched while the load runs.
        let spec = KnownModels.managed.first { $0.id == id }
        let root = spec?.cachePath
        let baseline = root.map(ModelStore.directoryBytes) ?? 0
        let watcher = Task {
            while !Task.isCancelled {
                if let root { onBytes(max(0, ModelStore.directoryBytes(root) - baseline)) }
                try? await Task.sleep(nanoseconds: 500_000_000)
            }
        }
        defer { watcher.cancel() }
        switch id {
        case "silero-vad-v6.2.1": _ = try await SileroVAD.probabilities([Float](repeating: 0, count: 4096))
        case "parakeet-eou-120m": let s = try await ParakeetEOUTranscriber(StageParams()).begin(); s.cancel()
        case "parakeet-tdt-0.6b-v2": let s = try await ParakeetTDTTranscriber(StageParams()).begin(); s.cancel()
        case "kokoro-82m-ane": _ = try await KokoroVoice.synthesizeOnce("Ready.")
        case "pocket-tts": _ = try await PocketTTSVoice.firstFrameMs("Ready.")
        default:
            guard KnownModels.isPinned(id) else { throw LabError.unavailable("no loader for \(id) in this build") }
            #if canImport(llama)
            _ = try await ModelStore.shared.ensure(id) { done, _ in onBytes(done) }
            if role == "responder" { _ = try await LlamaLocalModel.engine(id, download: false) }
            else { _ = try await LlamaSystem1.loadEngine(download: false) }
            #else
            throw LabError.unavailable("this build has no llama.cpp framework, so the small language model cannot load")
            #endif
        }
        if let root { onBytes(max(0, ModelStore.directoryBytes(root) - baseline)) }
    }

    public func check(_ id: String, role: String) async throws -> SetupCheck {
        guard KnownModels.contains(id) else { throw LabError.badInput("setup asked for \(id), which is not in the model catalog") }
        guard let audio = FixtureTurn.micSamples16k(), !audio.isEmpty else {
            return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "The bundled utterance could not be read.")
        }
        let seconds = Double(audio.count) / 16000
        let words = FixtureTurn.expectedWords
        func frames() -> [AudioFrame] { ModelProof.frames(audio) }
        func finish(_ ok: Bool, speed: Double, unit: String, note: String, firstAudioMs: Double? = nil,
                    prompt: Double? = nil, decode: Double? = nil) async -> SetupCheck {
            let r = await ModelStore.shared.loadReports.first { $0.id == id }
            return SetupCheck(loadMs: r?.loadMs ?? 0, speed: speed, unit: unit, memoryMB: r?.residentAfterMB ?? Double(residentBytes()) / 1_048_576,
                              outputOK: ok, note: ok ? "" : note, promptTokensPerSecond: prompt, decodeTokensPerSecond: decode, firstAudioMs: firstAudioMs,
                              availableMemoryMB: DeviceReading.availableMemoryMB(), thermal: DeviceReading.thermal())
        }

        switch id {
        case "silero-vad-v6.2.1":
            let p = try await SileroVAD.probabilities(audio)
            let top = p.probs.max() ?? 0
            return await finish(top > 0.5, speed: p.inferenceMs / (seconds * 1000), unit: "of real time",
                                note: "The speech detector found no speech in the bundled utterance (highest \(Int(top * 100))%).")
        case "parakeet-eou-120m", "parakeet-tdt-0.6b-v2":
            let stage: Transcriber = id == "parakeet-eou-120m" ? ParakeetEOUTranscriber(StageParams()) : ParakeetTDTTranscriber(StageParams())
            let s = try await stage.begin()
            let t0 = nowMs()
            for f in frames() { s.append(f) }
            let text = try await s.finish()
            let ms = nowMs() - t0
            let heard = words.filter { text.lowercased().contains($0) }.count
            return await finish(heard >= 2, speed: ms / (seconds * 1000), unit: "of real time",
                                note: "It heard \"\(text)\" where the bundled utterance says \"tell me more about the park\".")
        case "kokoro-82m-ane":
            let r = try await KokoroVoice.synthesizeOnce("Good morning. This is your radio.")
            return await finish(Self.spoke(r.samples, rate: r.rate), speed: r.ms / max(1, Double(r.samples.count) / r.rate * 1000), unit: "of real time",
                                note: "The voice produced silence or almost nothing.", firstAudioMs: r.ms)       // Kokoro does not stream: first sound is the whole sentence
        case "pocket-tts":
            let r = try await PocketTTSVoice.firstFrameMs("Good morning. This is your radio.")
            return await finish(r.seconds > 0.5, speed: r.totalMs / max(1, r.seconds * 1000), unit: "of real time", note: "The voice produced almost no audio.",
                                firstAudioMs: r.firstMs)
        default:
            guard KnownModels.isPinned(id) else {
                return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "No check is defined for \(id) in this build.")
            }
            #if canImport(llama)
            if role == "responder" { return try await Self.checkResponder(id, finish: finish) }
            return try await Self.checkDecision(finish: finish)
            #else
            return SetupCheck(loadMs: 0, speed: 0, unit: "", memoryMB: 0, outputOK: false, note: "This build has no llama.cpp framework, so the small language model cannot run.")
            #endif
        }
    }

    /// Audible: more than a third of a second and not silence.
    static func spoke(_ samples: [Float], rate: Double) -> Bool {
        guard rate > 0, Double(samples.count) / rate > 0.35 else { return false }
        let rms = (samples.reduce(0) { $0 + $1 * $1 } / Float(samples.count)).squareRoot()
        return rms > 0.003
    }

    typealias Finish = (Bool, Double, String, String, Double?, Double?, Double?) async -> SetupCheck

    // MARK: Sample asks against invented demo pages (the Mac proof prints what each responder says, cold and warm)

    public static let samplePages: [(ask: String, title: String, kind: String, text: String)] = [
        ("What is the council actually going to build?", "The harbor council approves a tide barrier", "texture",
         "The harbor council voted on Tuesday to approve a tide barrier across the east channel. Engineers say the barrier will close when water rises above a set level. Construction is planned to start next spring, and shipping will be rerouted during the work. Some residents asked about the cost and the effect on fishing."),
        ("Tell me more about the battery", "A regional grid adds its largest battery yet", "texture",
         "A regional grid operator switched on its largest battery this week. The battery stores energy from solar farms and releases it in the evening when demand peaks. Operators said it can supply thousands of homes for several hours. A second, larger site is under review."),
        ("What's on the front page?", "Today's paper", "paper",
         "Top stories: the harbor council approves a tide barrier; a regional grid adds its largest battery yet; a charging pilot reports its first results; the library extends its weekend hours."),
        ("Why did the park close?", "Mail", "mail", ""),
    ]

    #if canImport(llama)
    /// The decision model must score a clear request as "yes".
    static func checkDecision(finish: Finish) async throws -> SetupCheck {
        let sys = LlamaSystem1(StageParams())
        let q = Question(id: "asks_for_something", text: "Does the listener ask for something to be done or looked up?", options: ["yes", "no"])
        let input = System1Input(context: "A story about a new park.", listenerWords: "Tell me more about the park", questions: [q])
        _ = try await sys.answer(input)                 // the first decode after a load is slower than the rest
        let t0 = nowMs()
        let ans = try await sys.answer(input)
        let decisionMs = nowMs() - t0
        let decided = ans.first?.option == "yes"
        let note = decided ? "" : "On a clear request it chose \"\(ans.first?.option ?? "nothing")\" instead of yes."
        return await finish(decided, decisionMs, "ms per decision", note, nil, nil, nil)
    }

    /// The responder is measured on the device: the page read cold (nothing cached) and warm (read ahead), tokens per second reading
    /// and writing, and that it writes at least one sentence the voice can say. Generation here stops after a fixed number of tokens
    /// because this is a measurement, not speech.
    static func checkResponder(_ id: String, finish: Finish) async throws -> SetupCheck {
        let e = try await LlamaLocalModel.engine(id, download: false)
        let layout = ChatLayout.forModel(id)
        let p = samplePages[0]
        let system = AckPrompt.system()
        let prefix = AckPrompt.userPrefix(pageTitle: p.title, pageKind: p.kind, pageText: p.text)
        let suffix = AckPrompt.userSuffix(listener: p.ask, previous: [])
        // Cold.
        e.resetCache()
        var cold = LlamaStats(), text = ""
        try e.generate(system: system, userPrefix: prefix, userSuffix: suffix, layout: layout, temperature: 0.7, stopAtSentence: false,
                       measureTokens: 40, onPiece: { text += $0 }, onStats: { cold = $0 })
        // Warm: the page read ahead, then the question.
        e.resetCache()
        _ = try e.prefill(system: system, userPrefix: prefix, layout: layout, shouldStop: { false })
        var warm = LlamaStats()
        try e.generate(system: system, userPrefix: prefix, userSuffix: suffix, layout: layout, temperature: 0.7, stopAtSentence: false,
                       measureTokens: 40, onStats: { warm = $0 })
        let sentence = AckPrompt.firstSentence(text)
        let note = sentence == nil ? "It wrote nothing the voice could say." : String(format: "page of %d tokens read cold in %.0f ms, warm in %.0f ms (%d of %d tokens already cached); first sentence \"%@\"",
                                                                                         cold.promptTokens, cold.prefillMs, warm.prefillMs, warm.reusedTokens, warm.promptTokens, sentence ?? "")
        var r = await finish(sentence != nil, cold.firstTokenMs, "ms to the first word, cold", note,
                             nil, cold.promptTokensPerSecond, cold.decodeTokensPerSecond)
        if sentence != nil { r.note = note }
        r.speed = cold.firstTokenMs; r.unit = "ms to the first word, page cold; \(Int(warm.firstTokenMs)) ms warm"
        return r
    }
    #endif

    /// What the responder says for each sample ask, cold and warm: each sentence with its time, the cache report and why the stream
    /// ended. For the Mac proof's log; the owner reads these to judge whether the model is good enough.
    public static func sampleStreams(modelID: String) async -> [String] {
        #if canImport(llama)
        var out: [String] = ["== responder \(modelID)"]
        let m = LlamaLocalModel(modelID: modelID)
        guard let e = try? await LlamaLocalModel.engine(modelID, download: false) else { return out + ["  not loaded"] }
        var said: [String: [String]] = [:]          // lines said per page: only the same page's lines are context, as in the app
        for p in samplePages {
            for warm in [false, true] {
                e.resetCache()
                let system = AckPrompt.system()
                let prefix = AckPrompt.userPrefix(pageTitle: p.title, pageKind: p.kind, pageText: p.text)
                if warm { await m.prefill(system: system, userPrefix: prefix) }
                out.append("ASK \"\(p.ask)\" on \"\(p.title)\" (\(p.kind)), \(warm ? "WARM page cache" : "COLD cache")")
                let t0 = nowMs()
                var splitter = SentenceSplitter()
                var n = 0
                func show(_ raw: String) {
                    guard let l = AckPrompt.speakable(raw) else { return }
                    n += 1
                    if !warm { said[p.title, default: []].append(l) }
                    out.append(String(format: "  %6.0f ms  ", nowMs() - t0) + l)
                }
                do {
                    for try await piece in m.stream(system: system, userPrefix: prefix, userSuffix: AckPrompt.userSuffix(listener: p.ask, previous: Array((said[p.title] ?? []).suffix(6))), temperature: 0.7) {
                        for s in splitter.push(piece) { show(s) }
                    }
                    for s in splitter.finish() { show(s) }
                    let r = m.lastReport()
                    out.append(String(format: "  %6.0f ms  the model ended the stream by itself after %d sentences; prompt %d tokens, %d already cached, read in %.0f ms", nowMs() - t0, n,
                                      r?.promptTokens ?? 0, r?.reusedTokens ?? 0, r?.prefillMs ?? 0))
                } catch { out.append("  stream failed: \(error)") }
            }
        }
        return out
        #else
        return ["no llama.cpp in this build"]
        #endif
    }

    /// An acknowledgement and a decision asked at the same moment on separate contexts: how long each takes together.
    public static func contention(modelID: String) async -> String {
        #if canImport(llama)
        let m = LlamaLocalModel(modelID: modelID)
        let sys = LlamaSystem1(StageParams())
        let p = samplePages[0]
        let q = Question(id: "asks_for_something", text: "Does the listener ask for something to be done or looked up?", options: ["yes", "no"])
        let input = System1Input(context: p.text, listenerWords: p.ask, questions: [q])
        guard (try? await sys.answer(input)) != nil else { return "contention: decision model not loaded" }
        let system = AckPrompt.system()
        let prefix = AckPrompt.userPrefix(pageTitle: p.title, pageKind: p.kind, pageText: p.text)
        let suffix = AckPrompt.userSuffix(listener: p.ask, previous: [])
        /// Milliseconds until the first words of the acknowledgement, from `start`.
        func firstWords(from start: Double) async -> Double {
            do { for try await _ in m.stream(system: system, userPrefix: prefix, userSuffix: suffix, temperature: 0.7).prefix(1) {} } catch {}
            return nowMs() - start
        }
        // Alone.
        let aloneAck = await firstWords(from: nowMs())
        let d0 = nowMs(); _ = try? await sys.answer(input); let aloneDecision = nowMs() - d0
        // Together, the acknowledgement announced first as the engine does it.
        let c0 = nowMs()
        m.willGenerate()
        async let ack: Double = { let t = await firstWords(from: c0); m.generationEnded(); return t }()
        async let dec: Double = { _ = try? await sys.answer(input); return nowMs() - c0 }()
        let (ackT, decT) = await (ack, dec)
        return String(format: "contention (separate contexts): first words alone %.0f ms, with a decision asked at the same moment %.0f ms; decision alone %.0f ms, together %.0f ms",
                      aloneAck, ackT, aloneDecision, decT)
        #else
        return "contention: no llama.cpp in this build"
        #endif
    }
}
