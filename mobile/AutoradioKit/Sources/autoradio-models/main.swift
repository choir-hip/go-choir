import Foundation
import AutoradioKit
import AutoradioModels

// Measures each on-device model stage on a kept turn's audio and prints one line per stage:
//   id | load ms (first, warm) | MB fetched | MB resident after load / after unload | speed | where | result
// "first" includes the download when the model is not yet on disk. usage:
//   autoradio-models --turn <folder> [--stages silero-vad,parakeet-eou,...] [--json out.json] [--no-warm]

var args = Array(CommandLine.arguments.dropFirst())
func take(_ f: String) -> String? {
    guard let i = args.firstIndex(of: f), i + 1 < args.count else { return nil }
    let v = args[i + 1]; args.removeSubrange(i...i + 1); return v
}
let turnPath = take("--turn"), only = take("--stages")?.split(separator: ",").map(String.init), jsonOut = take("--json")
let warm = !args.contains("--no-warm")
#if os(macOS)
let whereRan = "Mac (\(ProcessInfo.processInfo.machineModel))"
#else
let whereRan = "iPhone"
#endif
extension ProcessInfo { var machineModel: String { var s = 0; sysctlbyname("hw.model", nil, &s, nil, 0); var c = [CChar](repeating: 0, count: s); sysctlbyname("hw.model", &c, &s, nil, 0); return String(cString: c) } }

struct Row: Codable {
    var id: String, kind: String, model: String
    var loadMsFirst: Double?, loadMsWarm: Double?, fetchedMB: Double?
    var residentAfterLoadMB: Double?, residentAfterUnloadMB: Double?
    var speed: String = "", result: String = "", ranOn: String = "", error: String?
}

guard let turnPath else { FileHandle.standardError.write(Data("usage: autoradio-models --turn <folder> [--stages a,b] [--json out] [--no-warm]\n".utf8)); exit(2) }
let turn = KeptTurn(folder: URL(fileURLWithPath: turnPath))
let ctx = try turn.context()
let audio = try AudioFiles.readMono(turn.micURL)
let seconds = Double(audio.count) / 16000
let words = (ctx.expectedWords ?? []).joined(separator: " ")

let registry = StageRegistry()
ModelHub.register(into: registry)

func frames(_ s: [Float], step: Int = 512) -> [AudioFrame] {
    stride(from: 0, to: s.count, by: step).map { AudioFrame(samples: Array(s[$0..<min(s.count, $0 + step)]), sampleRate: 16000, t: Double($0) / 16000) }
}
func f1(_ x: Double) -> String { String(format: "%.1f", x) }

// Each case: stage kind, id, the model id it loads, and one exercise returning (speed, result).
struct Case { let kind: StageKind; let id: String; let model: String; let run: () async throws -> (String, String) }
let cases: [Case] = [
    Case(kind: .speechBounds, id: "silero-vad", model: "silero-vad-v6.2.1") {
        let vad = try registry.make(.speechBounds, as: SpeechBounds.self, LabConfiguration(name: "p", stages: [.speechBounds: .init(id: "silero-vad")]))
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
    Case(kind: .transcriber, id: "parakeet-eou", model: "parakeet-eou-120m") {
        let t = try registry.make(.transcriber, as: Transcriber.self, LabConfiguration(name: "p", stages: [.transcriber: .init(id: "parakeet-eou")]))
        let s = try await t.begin()
        let t0 = nowMs()
        for fr in frames(audio) { s.append(fr) }
        let text = try await s.finish()
        let ms = nowMs() - t0
        return ("RTF \(String(format: "%.3f", ms / (seconds * 1000))) (\(Int(ms)) ms for \(f1(seconds)) s, chunk 320 ms)", text)
    },
    Case(kind: .transcriber, id: "parakeet-tdt", model: "parakeet-tdt-0.6b-v2") {
        let t = try registry.make(.transcriber, as: Transcriber.self, LabConfiguration(name: "p", stages: [.transcriber: .init(id: "parakeet-tdt")]))
        let s = try await t.begin()
        let t0 = nowMs()
        for fr in frames(audio) { s.append(fr) }
        let text = try await s.finish()
        let ms = nowMs() - t0
        return ("RTF \(String(format: "%.3f", ms / (seconds * 1000))) (\(Int(ms)) ms for \(f1(seconds)) s, batch)", text)
    },
    Case(kind: .system1, id: "llama-qwen3-0.6b", model: "qwen3-0.6b-q8_0") {
        let s = try registry.make(.system1, as: System1.self, LabConfiguration(name: "p", stages: [.system1: .init(id: "llama-qwen3-0.6b")]))
        let ans = try await s.answer(System1Input(context: ctx.position.context, listenerWords: words, questions: ctx.position.questions))
        let avg = ans.map(\.ms).reduce(0, +) / Double(max(1, ans.count))
        return ("\(Int(avg)) ms per question (\(ans.count) questions)", ans.map { "\($0.questionID)=\($0.option) p=\(String(format: "%.2f", $0.score))" }.joined(separator: "; "))
    },
    Case(kind: .voice, id: "kokoro", model: "kokoro-82m-ane") { try await speak("kokoro") },
    Case(kind: .voice, id: "pocket-tts", model: "pocket-tts") { try await speak("pocket-tts") },
    Case(kind: .transcriber, id: "lfm3-audio-live", model: "lfm3-audio-2b") {
        let t = try registry.make(.transcriber, as: Transcriber.self, LabConfiguration(name: "p", stages: [.transcriber: .init(id: "lfm3-audio-live")]))
        let s = try await t.begin()
        let t0 = nowMs()
        for fr in frames(audio) { s.append(fr) }
        let text = try await s.finish()
        let ms = nowMs() - t0
        return ("RTF \(String(format: "%.3f", ms / (seconds * 1000))) live (\(Int(ms)) ms for \(f1(seconds)) s)", text)
    },
    Case(kind: .voice, id: "lfm3-audio-tts", model: "lfm3-audio-2b") { try await speak("lfm3-audio-tts") },
]

func speak(_ id: String) async throws -> (String, String) {
    let v = try registry.make(.voice, as: Voice.self, LabConfiguration(name: "p", stages: [.voice: .init(id: id)]))
    let (text, cont) = AsyncStream<String>.makeStream()
    cont.yield("Harbour Park opened after the cranes moved away. Residents said the quiet was strange at first.")
    cont.finish()
    let t0 = nowMs()
    var first: Double?, n = 0, rate = 24000.0
    for try await f in v.speak(text) {
        if first == nil { first = nowMs() - t0 }
        n += f.samples.count; rate = f.sampleRate
    }
    let total = nowMs() - t0, secs = Double(n) / rate
    return ("first audio \(Int(first ?? -1)) ms; RTF \(String(format: "%.2f", total / max(1, secs * 1000)))", "\(f1(secs)) s of audio at \(Int(rate)) Hz")
}

var rows: [Row] = []
for c in cases where only == nil || only!.contains(c.id) {
    var row = Row(id: c.id, kind: c.kind.rawValue, model: c.model, ranOn: whereRan)
    do {
        // First run: includes download and first-time Core ML compile.
        _ = try await c.run()
        let r1 = await ModelHub.store.loadReports.first { $0.id == c.model }
        row.loadMsFirst = r1?.loadMs; row.fetchedMB = r1.map { Double($0.fetchedBytes) / 1_048_576 }
        row.residentAfterLoadMB = r1?.residentAfterMB
        // Warm run: speed and result.
        let (speed, result) = try await c.run()
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
    func ms(_ x: Double?) -> String { x.map { String(Int($0)) } ?? "-" }
    func mb(_ x: Double?) -> String { x.map { String(Int($0)) } ?? "-" }
    print("\(row.id) | load \(ms(row.loadMsFirst)) ms first, \(ms(row.loadMsWarm)) ms warm | fetched \(mb(row.fetchedMB)) MB | resident \(mb(row.residentAfterLoadMB)) MB loaded, \(mb(row.residentAfterUnloadMB)) MB after unload | \(row.speed) | \(whereRan) | \(row.error.map { "ERROR: \($0)" } ?? row.result)")
    fflush(stdout)
}
if let jsonOut {
    let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
    try e.encode(rows).write(to: URL(fileURLWithPath: jsonOut))
}
exit(rows.contains { $0.error != nil } ? 3 : 0)

func nowMs() -> Double { Double(DispatchTime.now().uptimeNanoseconds) / 1e6 }
