// Tiny Voca, bundled with Autoradio: the radio's own voice.
//
// Native only, because it is voice: three Core ML models (Encoder and Decoder on the Neural
// Engine, Vocoder on the GPU), the unchanged Apache-2.0 Dizione C++ text frontend (static
// XCFramework), and the pinned Swift runner from razorback16/tiny-voca-ane. The page, the
// choice and the queue are shared Exact code.
//
// Nothing is downloaded and nothing falls back to Apple's speech synthesizer: the files are in
// the app (Resources/TinyVoca) and a missing file is an error with a plain reason.
//
// Sources: port github.com/razorback16/tiny-voca-ane @426d644dba5953a2971c8fa27eeac5df77c0c2bb;
// Dizione github.com/0xShug0/audio.cpp @4363a483002df5b0c3eb83ae93addf0073de388c;
// weights huggingface.co/n4ze3m/tiny-voca @5fe06641d0afecb3c04bc9faca2cf64d9b2b1600.
// Apache-2.0; the notices ship beside the files (TinyVoca/Licenses).
//
// WAYS THIS CAN FAIL (each has a test in TinyVocaVoiceTests):
//  V1 The phone has no TinyVoca folder: a plain error, never a silent line and never Apple's voice.
//  V2 One compiled model is missing: the same, naming the file, and no attempt to compile a source model.
//  V3 The phonemizer data is missing: a plain error about the voice's text data.
//  V4 A line with nothing to pronounce ("…", emoji) makes no audio and is skipped, not an error.
//  V5 A long line is split at sentence marks and within the model's 512-token bucket.
//  V6 A cancelled line stops making audio; the next line still speaks.
//  V7 Two voices are offered (F2 and M2) and are different audio for the same line.

import Accelerate
import CoreML
import AutoradioKit
import Foundation

#if canImport(dizione)
import dizione
#endif

/// The Dizione C ABI, as built by scripts/build-dizione-xcframework.sh.
private enum Phonemizer {
    struct Utterance { var ids: [Int]; var stress: [Int] }

    /// The folder named by `directory`: lexicon.tsv, align.json, g2p.json, g2p.npz, homographs.json.
    final class Engine {
        private let pointer: UnsafeMutableRawPointer

        init(directory: URL) throws {
            #if canImport(dizione)
            guard let p = ar_dizione_create(directory.path) else {
                throw TinyVocaVoice.Failure.textData(String(cString: ar_dizione_error()))
            }
            pointer = p
            #else
            throw TinyVocaVoice.Failure.textData("the phone build has no text-to-phoneme engine")
            #endif
        }
        deinit { #if canImport(dizione)
        ar_dizione_destroy(pointer)
        #endif
    }

        /// The line as utterances of at most `tokens` tokens, cut at sentence marks.
        func utterances(_ line: String, tokens: Int) throws -> [Utterance] {
            #if canImport(dizione)
            var data: UnsafeMutablePointer<Int32>?
            var count: Int32 = 0
            let status = line.withCString { ar_dizione_phonemize(pointer, $0, Int32(tokens), &data, &count) }
            guard status == 0, let buffer = data else {
                throw TinyVocaVoice.Failure.nothingToSay(String(cString: ar_dizione_error()))
            }
            defer { ar_dizione_free(buffer) }
            var out: [Utterance] = []
            var at = 1
            for _ in 0..<Int(buffer[0]) {
                let n = Int(buffer[at]); at += 1
                out.append(Utterance(ids: (0..<n).map { Int(buffer[at + $0]) },
                                     stress: (0..<n).map { Int(buffer[at + n + $0]) }))
                at += 2 * n
            }
            return out
            #else
            throw TinyVocaVoice.Failure.textData("the phone build has no text-to-phoneme engine")
            #endif
        }
    }
}

public final class TinyVocaVoice: Voice, @unchecked Sendable {
    public static let stageID = "tiny-voca"

    public enum Failure: Error, CustomStringConvertible {
        case notBundled
        case missingModel(String)
        case textData(String)
        case nothingToSay(String)
        case model(String)

        public var description: String {
            switch self {
            case .notBundled: return "The voice's files are not in this app."
            case .missingModel(let name): return "The voice's \(name) model is missing from this app."
            case .textData(let why): return "The voice's text data could not be read: \(why)"
            case .nothingToSay(let why): return "nothing to pronounce (\(why))"
            case .model(let why): return "The voice could not speak: \(why)"
            }
        }
    }

    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Tiny Voca", summary: "Bundled on-device voice; no download, no fallback.",
                        params: [.text("voice", "Voice (F2 or M2)", "F2")])
    }

    /// 0 is F2, 1 is M2, as the model's two speakers.
    public static let speakers = ["F2", "M2"]
    private static let tokensPerUtterance = 512
    private static let sampleRate = 24_000.0

    private let voiceIndex: Int

    public init(_ params: StageParams) {
        let name = params.string("voice", "F2").uppercased()
        voiceIndex = TinyVocaVoice.speakers.firstIndex(of: name) ?? 0
    }

    // MARK: Where the files are

    /// `Resources/TinyVoca`: Exact writes declared trees under ExactResources/Resources, and a
    /// flat bundle keeps it at the root. Both are looked at; nothing is downloaded.
    static var bundledDirectory: URL? {
        let root = Bundle.main.resourceURL ?? Bundle.main.bundleURL
        let candidates = [root.appendingPathComponent("ExactResources/Resources/TinyVoca"),
                          root.appendingPathComponent("TinyVoca")]
        return candidates.first { FileManager.default.fileExists(atPath: $0.appendingPathComponent("tiny-voca-coreml.json").path) }
    }

    private final class EngineBox: @unchecked Sendable {
        let engine: TinyVoca
        let phonemizer: Phonemizer.Engine
        init(engine: TinyVoca, phonemizer: Phonemizer.Engine) { self.engine = engine; self.phonemizer = phonemizer }
    }

    private static let shared = OneEngine()

    private final class OneEngine: @unchecked Sendable {
        private let lock = NSLock()
        private var box: EngineBox?
        private var loading: Task<EngineBox, Error>?

        func get() async throws -> EngineBox {
            lock.lock()
            if let box { lock.unlock(); return box }
            if let loading { lock.unlock(); return try await loading.value }
            let task = Task<EngineBox, Error> {
                guard let directory = TinyVocaVoice.bundledDirectory else { throw Failure.notBundled }
                for name in ["Encoder", "Decoder", "Vocoder"] where !FileManager.default.fileExists(atPath: directory.appendingPathComponent("\(name).mlmodelc").path) {
                    throw Failure.missingModel(name)
                }
                let phonemizer = try Phonemizer.Engine(directory: directory.appendingPathComponent("phonemizer"))
                // The vocoder on the GPU hits an Apple MPSGraph bug on macOS 27; the Mac proof runs it on
                // the CPU. iOS keeps the GPU, where the port measured it.
                #if os(macOS)
                let vocoder: MLComputeUnits = .cpuOnly
                #else
                let vocoder: MLComputeUnits = .cpuAndGPU
                #endif
                do {
                    let runner = try await TinyVoca(modelsDirectory: directory, vocoderUnits: vocoder)
                    return EngineBox(engine: runner, phonemizer: phonemizer)
                } catch { throw Failure.model("\(error)") }
            }
            loading = task
            lock.unlock()
            do {
                let box = try await task.value
                lock.lock(); self.box = box; self.loading = nil; lock.unlock()
                return box
            } catch {
                lock.lock(); self.loading = nil; lock.unlock()
                throw error
            }
        }

        func drop() { lock.lock(); box = nil; lock.unlock() }
    }

    /// Load the models now (the first line of the first block should not pay for it).
    static func warm() async -> Bool { (try? await shared.get()) != nil }

    // MARK: Speaking

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let index = voiceIndex
        return AsyncThrowingStream { stream in
            let task = Task {
                do {
                    let box = try await TinyVocaVoice.shared.get()
                    for await line in text {
                        if Task.isCancelled { break }
                        let utterances: [Phonemizer.Utterance]
                        do {
                            utterances = try box.phonemizer.utterances(line, tokens: TinyVocaVoice.tokensPerUtterance)
                        } catch TinyVocaVoice.Failure.nothingToSay {
                            continue                       // nothing to pronounce in this line; not an error (V4)
                        }
                        for u in utterances {
                            if Task.isCancelled { break }
                            guard u.ids.count >= 3, u.ids.count == u.stress.count else { continue }
                            let samples: [Float]
                            do {
                                samples = try box.engine.synthesize(tokens: u.ids, stress: u.stress, voice: index)
                            } catch { throw TinyVocaVoice.Failure.model("\(error)") }
                            if samples.isEmpty { continue }
                            stream.yield(AudioFrame(samples: samples, sampleRate: TinyVocaVoice.sampleRate,
                                                    t: ProcessInfo.processInfo.systemUptime))
                        }
                    }
                    stream.finish()
                } catch {
                    stream.finish(throwing: error)
                }
            }
            stream.onTermination = { _ in task.cancel() }
        }
    }
}
