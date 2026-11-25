// tiny-voca on Core ML: Encoder and Decoder on the Neural Engine (one function per length
// bucket), host steps on the CPU, Vocoder in float32 on the GPU. The model packages come from
// tiny_voca_ane (ane.convert_encoder / convert_decoder with id_dtype=np.float16, vocoder.convert).

import CoreML
import Foundation

public enum TinyVocaError: Error, CustomStringConvertible {
    case missingModel(URL)
    case tooManyTokens(Int)
    case tooManyFrames(Int)
    case noFrames
    case badInput(String)
    case missingOutput(String)

    public var description: String {
        switch self {
        case .missingModel(let url): return "no compiled model at \(url.path) (nor an .mlpackage beside it)"
        case .tooManyTokens(let n): return "\(n) tokens exceed the largest encoder bucket; split the text"
        case .tooManyFrames(let t): return "\(t) frames exceed max_frames \(Config.maxFrames); split the text"
        case .noFrames: return "utterance has no frames"
        case .badInput(let text): return text
        case .missingOutput(let name): return "model output \(name) is missing"
        }
    }
}

/// Seconds spent per stage of one utterance.
public struct StageTimes: Sendable {
    /// Encoder inputs and prediction.
    public var encoder = 0.0
    /// Reading model outputs, durations, frame expansion, decoder inputs, f0.
    public var host = 0.0
    /// Decoder prediction.
    public var decoder = 0.0
    /// Pulse train and noise.
    public var source = 0.0
    /// Vocoder inputs, prediction and output.
    public var vocoder = 0.0
    public var total: Double { encoder + host + decoder + source + vocoder }
    public init() {}
}

/// Audio of one utterance with the intermediate values (for parity checks).
public struct Synthesis: Sendable {
    public var audio: [Float]
    public var times: StageTimes
    public var encoderBucket: Int, decoderBucket: Int
    /// Encoder output [192][n] and raw duration output [2][n], real tokens only.
    public var hidden: [Float], rawDuration: [Float]
    public var durations: [Int]
    /// Decoder outputs, real frames only: pitch [T], harmonic [98][T], noise_params [36][T].
    public var pitch: [Float], harmonic: [Float], noiseParams: [Float]
    public var f0: [Float], pulses: [Float], noise: [Float]
}

final class TVLocked<T>: @unchecked Sendable {
    private var value: T
    private let lock = NSLock()
    init(_ value: T) { self.value = value }
    func get() -> T { lock.withLock { value } }
    func set(_ new: T) { lock.withLock { value = new } }
}

func tvNow() -> Double { Double(clock_gettime_nsec_np(CLOCK_UPTIME_RAW)) * 1e-9 }

public final class TinyVoca: @unchecked Sendable {
    public let modelsDirectory: URL
    public let encoderBuckets: [Int], decoderBuckets: [Int]
    public let neuralUnits: MLComputeUnits, vocoderUnits: MLComputeUnits
    private var cache: [String: MLModel] = [:]
    private let lock = NSLock()
    /// Load time in seconds of every model function loaded so far, in load order.
    public private(set) var loadTimes: [(name: String, seconds: Double)] = []

    /// Models from `modelsDirectory`: Encoder.mlmodelc, Decoder.mlmodelc, Vocoder.mlmodelc (an .mlpackage
    /// beside a missing .mlmodelc is compiled once and kept). `neuralUnits` runs Encoder and Decoder
    /// (.cpuAndNeuralEngine; .all lets the planner move long decoder buckets to the slower GPU).
    /// Functions are loaded on first use; `preload()` loads them all.
    public init(modelsDirectory: URL, neuralUnits: MLComputeUnits = .cpuAndNeuralEngine,
                vocoderUnits: MLComputeUnits = .cpuAndGPU) async throws {
        self.modelsDirectory = modelsDirectory
        self.neuralUnits = neuralUnits
        self.vocoderUnits = vocoderUnits
        for name in ["Encoder", "Decoder", "Vocoder"] { try await Self.ensureCompiled(modelsDirectory, name) }
        encoderBuckets = try await Self.buckets(modelsDirectory.appending(path: "Encoder.mlmodelc"), prefix: "n")
        decoderBuckets = try await Self.buckets(modelsDirectory.appending(path: "Decoder.mlmodelc"), prefix: "t")
    }

    public func url(_ name: String) -> URL { modelsDirectory.appending(path: "\(name).mlmodelc") }

    static func ensureCompiled(_ directory: URL, _ name: String) async throws {
        let compiled = directory.appending(path: "\(name).mlmodelc"), package = directory.appending(path: "\(name).mlpackage")
        let files = FileManager.default
        if files.fileExists(atPath: compiled.path) { return }
        guard files.fileExists(atPath: package.path) else { throw TinyVocaError.missingModel(compiled) }
        let temporary = try await MLModel.compileModel(at: package)
        try files.moveItem(at: temporary, to: compiled)
    }

    /// Bucket sizes from the function names of a multifunction model (n32, n64, ... or t128, ...).
    static func buckets(_ url: URL, prefix: String) async throws -> [Int] {
        let names = try await MLModelAsset(url: url).functionNames
        let sizes = names.compactMap { $0.hasPrefix(prefix) ? Int($0.dropFirst(prefix.count)) : nil }.sorted()
        guard !sizes.isEmpty else { throw TinyVocaError.badInput("\(url.lastPathComponent) has no \(prefix)* functions") }
        return sizes
    }

    /// One MLModel per function, loaded once.
    public func model(_ name: String, function: String? = nil) throws -> MLModel {
        let key = function.map { "\(name)/\($0)" } ?? name
        if let model = lock.withLock({ cache[key] }) { return model }
        let configuration = MLModelConfiguration()
        configuration.computeUnits = name == "Vocoder" ? vocoderUnits : neuralUnits
        configuration.functionName = function
        let start = tvNow()
        let model = try MLModel(contentsOf: url(name), configuration: configuration)
        let seconds = tvNow() - start
        return lock.withLock {
            if let first = cache[key] { return first }
            loadTimes.append((key, seconds))
            cache[key] = model
            return model
        }
    }

    /// Every function of the three models, loaded in parallel (each load parses the whole multifunction
    /// program, about 1.2 s for the decoder on an M4 Max, mostly on one core).
    public func preload() throws {
        let names = encoderBuckets.map { ("Encoder", "n\($0)") } + decoderBuckets.map { ("Decoder", "t\($0)") }
        let failure = TVLocked<Error?>(nil)
        DispatchQueue.concurrentPerform(iterations: names.count + 1) { i in
            do { _ = try i < names.count ? model(names[i].0, function: names[i].1) : model("Vocoder") } catch { failure.set(error) }
        }
        if let error = failure.get() { throw error }
    }

    static func bucket(_ length: Int, _ buckets: [Int]) -> Int? { buckets.first { length <= $0 } }

    static func output(_ result: MLFeatureProvider, _ name: String) throws -> MLMultiArray {
        guard let array = result.featureValue(for: name)?.multiArrayValue else { throw TinyVocaError.missingOutput(name) }
        return array
    }

    /// Audio [samples] at 24 kHz for one utterance of phoneme token ids and stress (0, 1, 2) from
    /// tiny_voca.text.phonemize. `voice` indexes Config.speakers. The audio is the model's output as it
    /// is (it can peak above 1; see Host.limitPeak).
    public func synthesize(tokens: [Int], stress: [Int], voice: Int, speed: Float = 1, pitchShift: Float = 0,
                           pitchRange: Float = 1, seed: UInt64 = 0) throws -> [Float] {
        try run(tokens: tokens, stress: stress, voice: voice, speed: speed, pitchShift: pitchShift,
                pitchRange: pitchRange, seed: seed).audio
    }

    /// `synthesize` with stage times and intermediate values. `noise` [T * hop] replaces the seeded noise
    /// (to compare with the Python path given the same noise).
    public func run(tokens: [Int], stress: [Int], voice: Int, speed: Float = 1, pitchShift: Float = 0,
                    pitchRange: Float = 1, seed: UInt64 = 0, noise external: [Float]? = nil) throws -> Synthesis {
        let n = tokens.count
        guard n > 0, stress.count == n else { throw TinyVocaError.badInput("ids and stress need the same nonzero length") }
        guard tokens.allSatisfy({ (0..<Config.vocabSize).contains($0) }) else { throw TinyVocaError.badInput("token id out of range") }
        guard Config.speakers.indices.contains(voice) else { throw TinyVocaError.badInput("voice must be 0..<\(Config.speakers.count)") }
        guard let size = Self.bucket(n, encoderBuckets) else { throw TinyVocaError.tooManyTokens(n) }
        var times = StageTimes()

        // Encoder: ids as fp16 (exact for small integers), padded at the end.
        var start = tvNow()
        let voiceInput = try Arrays.half([1]) { $0[0] = Float16(voice) }
        let encoderInputs: [String: Any] = [
            "tokens": try Arrays.half([1, size]) { b in for i in 0..<n { b[i] = Float16(tokens[i]) } },
            "stress": try Arrays.half([1, size]) { b in for i in 0..<n { b[i] = Float16(stress[i]) } },
            "voice": voiceInput,
            "token_mask": try Arrays.half([1, size]) { b in for i in 0..<n { b[i] = 1 } },
        ]
        let encoded = try model("Encoder", function: "n\(size)").prediction(from: MLDictionaryFeatureProvider(dictionary: encoderInputs))
        times.encoder = tvNow() - start

        // Host: durations, frame expansion, decoder inputs.
        start = tvNow()
        let hidden = Arrays.rows(try Self.output(encoded, "hidden"), length: n)       // [192][n]
        let raw = Arrays.rows(try Self.output(encoded, "duration"), length: n)        // [2][n]
        let durations = Host.durations(raw0: Array(raw[0..<n]), raw1: Array(raw[n..<(2 * n)]), tokens: tokens, speed: speed)
        let frames = durations.reduce(0, +)
        guard frames > 0 else { throw TinyVocaError.noFrames }
        guard frames <= Config.maxFrames, let length = Self.bucket(frames, decoderBuckets) else { throw TinyVocaError.tooManyFrames(frames) }
        let owner = Host.owners(durations)
        let position = Host.positions(durations)
        let channels = hidden.count / n
        let decoderInputs: [String: Any] = [
            "frames": try Arrays.half([1, channels, 1, length]) { b in
                hidden.withUnsafeBufferPointer { h in
                    for c in 0..<channels {
                        let row = h.baseAddress! + c * n, target = b.baseAddress! + c * length
                        for t in 0..<frames { target[t] = Float16(row[owner[t]]) }
                    }
                }
            },
            "position": try Arrays.half([1, 3, 1, length]) { b in
                for t in 0..<frames {
                    b[t] = Float16(position.relative[t])
                    b[length + t] = Float16(position.complement[t])
                    b[2 * length + t] = Float16(position.length[t])
                }
            },
            "voice": voiceInput,
            "frame_mask": try Arrays.half([1, length]) { b in for t in 0..<frames { b[t] = 1 } },
            "vowel_mask": try Arrays.half([1, length]) { b in for t in 0..<frames { b[t] = Config.vowelTokens[tokens[owner[t]]] ? 1 : 0 } },
            "pitch_range": try Arrays.half([1]) { $0[0] = Float16(pitchRange) },
        ]
        times.host = tvNow() - start

        start = tvNow()
        let decoded = try model("Decoder", function: "t\(length)").prediction(from: MLDictionaryFeatureProvider(dictionary: decoderInputs))
        times.decoder = tvNow() - start

        start = tvNow()
        let pitch = Arrays.rows(try Self.output(decoded, "pitch"), length: frames)
        let harmonic = Arrays.rows(try Self.output(decoded, "harmonic"), length: frames)
        let noiseParams = Arrays.rows(try Self.output(decoded, "noise_params"), length: frames)
        let f0 = Host.f0(pitch: pitch, shift: pitchShift)
        times.host += tvNow() - start

        start = tvNow()
        let samples = frames * Config.hop
        let pulses = Host.pulseTrain(f0: f0)
        let noise = external ?? Host.noise(samples: samples, seed: seed)
        guard noise.count == samples else { throw TinyVocaError.badInput("noise needs \(samples) samples, got \(noise.count)") }
        times.source = tvNow() - start

        start = tvNow()
        let vocoderInputs: [String: Any] = [
            "harmonic": try Arrays.float([1, harmonic.count / frames, frames], harmonic),
            "noise_params": try Arrays.float([1, noiseParams.count / frames, frames], noiseParams),
            "pulses": try Arrays.float([1, samples], pulses),
            "noise": try Arrays.float([1, samples], noise),
        ]
        let voiced = try model("Vocoder").prediction(from: MLDictionaryFeatureProvider(dictionary: vocoderInputs))
        let audio = Arrays.rows(try Self.output(voiced, "audio"))
        times.vocoder = tvNow() - start

        return Synthesis(audio: audio, times: times, encoderBucket: size, decoderBucket: length, hidden: hidden, rawDuration: raw,
                         durations: durations, pitch: pitch, harmonic: harmonic, noiseParams: noiseParams, f0: f0,
                         pulses: pulses, noise: noise)
    }
}
