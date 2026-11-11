import Foundation
import AutoradioKit
#if canImport(Darwin)
import Darwin
#endif

/// Measures each model stage on a kept turn's audio: load (first and warm), bytes fetched, resident memory after load
/// and after unload, speed, result. Used by `autoradio-models` on the Mac and, through a launch argument, on the phone.
public struct ModelProofRow: Codable, Sendable {
    public var id: String, kind: String, model: String
    public var loadMsFirst: Double?, loadMsWarm: Double?, fetchedMB: Double?
    public var residentAfterLoadMB: Double?, residentAfterUnloadMB: Double?
    public var speed: String = "", result: String = "", ranOn: String = "", error: String?
    public var line: String {
        func n(_ x: Double?) -> String { x.map { String(Int($0)) } ?? "-" }
        return "\(id) | load \(n(loadMsFirst)) ms first, \(n(loadMsWarm)) ms warm | fetched \(n(fetchedMB)) MB | resident \(n(residentAfterLoadMB)) MB loaded, \(n(residentAfterUnloadMB)) MB after unload | \(speed) | \(ranOn) | \(error.map { "ERROR: \($0)" } ?? result)"
    }
}

public enum ModelProof {
    public static var machine: String {
        #if os(iOS)
        let key = "hw.machine"
        #else
        let key = "hw.model"
        #endif
        var s = 0; sysctlbyname(key, nil, &s, nil, 0)
        var c = [CChar](repeating: 0, count: max(s, 1)); sysctlbyname(key, &c, &s, nil, 0)
        #if os(iOS)
        return "iPhone \(String(cString: c))"
        #else
        return "Mac \(String(cString: c))"
        #endif
    }

    static func f1(_ x: Double) -> String { String(format: "%.1f", x) }
    static func frames(_ s: [Float], step: Int = 512) -> [AudioFrame] {
        stride(from: 0, to: s.count, by: step).map { AudioFrame(samples: Array(s[$0..<min(s.count, $0 + step)]), sampleRate: 16000, t: Double($0) / 16000) }
    }

    public static func run(turn: KeptTurn, stages only: [String]?, warm: Bool = true, report: ((ModelProofRow) -> Void)? = nil) async throws -> [ModelProofRow] {
        let ctx = try turn.context()
        let audio = try AudioFiles.readMono(turn.micURL)
        let seconds = Double(audio.count) / 16000
        let words = (ctx.expectedWords ?? []).joined(separator: " ")
        let registry = StageRegistry()
        ModelHub.register(into: registry)
        let where_ = machine

        func make<T>(_ kind: StageKind, _ id: String, _ t: T.Type) throws -> T {
            try registry.make(kind, as: t, LabConfiguration(name: "proof", stages: [kind: .init(id: id)]))
        }
        func transcribe(_ id: String, label: String) async throws -> (String, String) {
            let s = try await make(.transcriber, id, Transcriber.self).begin()
            let t0 = nowMs()
            for fr in frames(audio) { s.append(fr) }
            let text = try await s.finish()
            let ms = nowMs() - t0
            return ("RTF \(String(format: "%.3f", ms / (seconds * 1000))) (\(Int(ms)) ms for \(f1(seconds)) s, \(label))", text)
        }
        func speak(_ id: String) async throws -> (String, String) {
            let v = try make(.voice, id, Voice.self)
            let (text, cont) = AsyncStream<String>.makeStream()
            cont.yield("Harbour Park opened after the cranes moved away. Residents said the quiet was strange at first.")
            cont.finish()
            let t0 = nowMs()
            var first: Double?, n = 0, rate = 24000.0
            for try await f in v.speak(text) { if first == nil { first = nowMs() - t0 }; n += f.samples.count; rate = f.sampleRate }
            let total = nowMs() - t0, secs = Double(n) / rate
            return ("first audio \(Int(first ?? -1)) ms; RTF \(String(format: "%.2f", total / max(1, secs * 1000)))", "\(f1(secs)) s of audio at \(Int(rate)) Hz")
        }

        struct Case { let kind: StageKind; let id: String; let model: String; let run: () async throws -> (String, String) }
        let cases: [Case] = [
            Case(kind: .speechBounds, id: "silero-vad", model: "silero-vad-v6.2.1") {
                let vad = try make(.speechBounds, "silero-vad", SpeechBounds.self)
                var ev: [String] = []
                for fr in frames(audio) {
                    switch await vad.feed(fr) {
                    case .speechStarted(let at): ev.append("start \(f1(at))s")
                    case .speechEnded(let at, _): ev.append("end \(f1(at))s")
                    case .none: break
                    }
                }
                let p = try await SileroVAD.probabilities(audio)
                let hops = Double(max(1, p.probs.count))
                return ("\(f1(p.inferenceMs / hops)) ms per 256 ms hop (RTFx \(f1(seconds * 1000 / max(p.inferenceMs, 0.001))))", ev.joined(separator: ", "))
            },
            Case(kind: .transcriber, id: "parakeet-eou", model: "parakeet-eou-120m") { try await transcribe("parakeet-eou", label: "streaming, chunk 320 ms") },
            Case(kind: .transcriber, id: "parakeet-tdt", model: "parakeet-tdt-0.6b-v2") { try await transcribe("parakeet-tdt", label: "batch") },
            Case(kind: .system1, id: "llama-qwen3-0.6b", model: "qwen3-0.6b-q8_0") {
                let s = try make(.system1, "llama-qwen3-0.6b", System1.self)
                let ans = try await s.answer(System1Input(context: ctx.position.context, listenerWords: words, questions: ctx.position.questions))
                let avg = ans.map(\.ms).reduce(0, +) / Double(max(1, ans.count))
                return ("\(Int(avg)) ms per question (\(ans.count) questions)", ans.map { "\($0.questionID)=\($0.option) p=\(String(format: "%.2f", $0.score))" }.joined(separator: "; "))
            },
            Case(kind: .voice, id: "kokoro", model: "kokoro-82m-ane") { try await speak("kokoro") },
            Case(kind: .voice, id: "pocket-tts", model: "pocket-tts") { try await speak("pocket-tts") },
            Case(kind: .transcriber, id: "lfm3-audio-live", model: "lfm3-audio-2b") { try await transcribe("lfm3-audio-live", label: "live") },
            Case(kind: .voice, id: "lfm3-audio-tts", model: "lfm3-audio-2b") { try await speak("lfm3-audio-tts") },
        ]

        var rows: [ModelProofRow] = []
        for c in cases where only == nil || only!.contains(c.id) {
            var row = ModelProofRow(id: c.id, kind: c.kind.rawValue, model: c.model, ranOn: where_)
            do {
                _ = try await c.run()      // first run: download, first-time Core ML compile
                let r1 = await ModelHub.store.loadReports.first { $0.id == c.model }
                row.loadMsFirst = r1?.loadMs; row.fetchedMB = r1.map { Double($0.fetchedBytes) / 1_048_576 }
                row.residentAfterLoadMB = r1?.residentAfterMB
                let (speed, result) = try await c.run()   // warm: speed and result
                row.speed = speed; row.result = result
                await ModelHub.store.unload(c.model)
                try? await Task.sleep(nanoseconds: 500_000_000)
                row.residentAfterUnloadMB = Double(residentBytes()) / 1_048_576
                if warm {
                    _ = try await c.run()
                    row.loadMsWarm = await ModelHub.store.loadReports.first { $0.id == c.model }?.loadMs
                    await ModelHub.store.unload(c.model)
                }
            } catch { row.error = "\(error)" }
            rows.append(row)
            report?(row)
        }
        return rows
    }

    /// On the phone: a launch argument `-ModelProofStages silero-vad,parakeet-eou` runs the proof on the turn in
    /// Documents/proof/turn (mic.caf, context.json, copied in with devicectl) and writes Documents/proof/results.json.
    static func runFromLaunchArguments() {
        guard let stages = UserDefaults.standard.string(forKey: "ModelProofStages") else { return }
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("proof", isDirectory: true)
        Task.detached {
            func log(_ s: String) { fputs("autoradio proof: \(s)\n", stderr); fflush(stderr) }
            do {
                log("start on \(machine)")
                let list = stages == "all" ? nil : stages.split(separator: ",").map(String.init)
                let rows = try await run(turn: KeptTurn(folder: docs.appendingPathComponent("turn")), stages: list, warm: true) { log($0.line) }
                let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
                try e.encode(rows).write(to: docs.appendingPathComponent("results.json"))
                log("done, \(rows.count) stages")
            } catch { log("failed: \(error)") }
        }
    }
}
