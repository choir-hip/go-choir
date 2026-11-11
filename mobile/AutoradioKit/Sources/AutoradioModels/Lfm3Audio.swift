import Foundation
import FluidAudio
import AutoradioKit

// LFM3-Audio-2B through FluidAudio's LFM3AudioManager: one loaded model serves three stages.
// On a 6 GB phone it does not fit beside the other models (vendor figures from an M5 Pro Mac: 1.9 GB peak,
// a 5.3 GB first-load Core ML cache, 2.5 GB download plus 0.4 GB for speech). Loading it first unloads every
// other model the store holds, and records what happened (the load report, or the exact error).
enum LFM3Shared {
    static let id = "lfm3-audio-2b"
    static let slot = ModelSlot<LFM3AudioManager>(id)
    static func manager() async throws -> LFM3AudioManager {
        try await slot.get {
            await ModelHub.store.unloadAll(except: [id])
            let m = try await LFM3AudioManager.downloadAndLoad(backend: .coreML, includeSpeech: true)
            return (m, {})
        }
    }
}

public final class LFM3Transcriber: Transcriber, @unchecked Sendable {
    public static let stageID = "lfm3-audio-live"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "LFM3-Audio-2B live ASR",
            summary: "Streaming English transcription in 0.96 s chunks from the 2B audio model (Core ML via FluidAudio). Unloads other models first.",
            needsModels: ["lfm3-audio-2b"])
    }
    public init(_ p: StageParams) {}
    public func begin() async throws -> TranscriptionSession {
        let m = try await LFM3Shared.manager()
        try await m.startLiveTranscription()
        return Session(m)
    }
    final class Session: TranscriptionSession, @unchecked Sendable {
        let mgr: LFM3AudioManager
        let partials: AsyncStream<String>
        let cont: AsyncStream<String>.Continuation
        let input: AsyncStream<[Float]>
        let inputCont: AsyncStream<[Float]>.Continuation
        var worker: Task<Void, Never>?
        init(_ m: LFM3AudioManager) {
            mgr = m
            (partials, cont) = AsyncStream<String>.makeStream()
            (input, inputCont) = AsyncStream<[Float]>.makeStream()
            let mgr = m, cont = cont, input = input
            worker = Task {
                var buf: [Float] = []
                for await s in input {
                    buf.append(contentsOf: s)
                    guard buf.count >= 2560 else { continue }          // 160 ms pieces
                    let piece = buf; buf = []
                    if let t = try? await mgr.appendLiveAudio(piece) { cont.yield(t) }
                }
                if !buf.isEmpty { _ = try? await mgr.appendLiveAudio(buf) }
            }
        }
        func append(_ frame: AudioFrame) { inputCont.yield(samples16k(frame)) }
        func finish() async throws -> String {
            inputCont.finish(); await worker?.value
            let t = try await mgr.finishLiveTranscription()
            cont.finish()
            return t
        }
        func cancel() { inputCont.finish(); worker?.cancel(); Task { await mgr.cancelLiveTranscription() }; cont.finish() }
    }
}

public final class LFM3Voice: Voice, @unchecked Sendable {
    public static let stageID = "lfm3-audio-tts"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "LFM3-Audio-2B speech",
            summary: "Speech from the 2B audio model; audio streams out as frames are decoded (24 kHz). Unloads other models first.",
            params: [.choice("voice", "Voice", "female", ["female", "male"])],
            needsModels: ["lfm3-audio-2b"])
    }
    let voice: LFM3AudioVoice
    public init(_ p: StageParams) { voice = p.string("voice", "female") == "male" ? .male : .female }
    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let voice = voice
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    let m = try await LFM3Shared.manager()
                    for await line in text {
                        if Task.isCancelled { break }
                        _ = try await m.synthesize(line, voice: voice) { samples in
                            c.yield(AudioFrame(samples: samples, sampleRate: 24000, t: ProcessInfo.processInfo.systemUptime))
                        }
                    }
                    c.finish()
                } catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
}
