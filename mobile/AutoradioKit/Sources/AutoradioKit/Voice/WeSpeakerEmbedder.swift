import Foundation
import CoreML
import CryptoKit

// The voice-embedding model: WeSpeaker ResNet34-LM (VoxCeleb), 256 numbers per stretch of speech, as a Core ML model that runs on the
// Neural Engine. Source (read 2026-11-15): Hugging Face `FluidInference/speaker-diarization-coreml`, folder `wespeaker_v2.mlmodelc`
// (8-bit palettised weights, 7.2 MB weights, 7.95 MB in all), a Core ML conversion of `pyannote/wespeaker-voxceleb-resnet34-LM`.
// Licence: CC-BY-4.0 (WeSpeaker; the conversion's NOTICE.md asks for attribution naming WeSpeaker, pyannote and Fluid Inference, a link to
// CC-BY-4.0 and a note that the files are modified Core ML conversions: the Settings row and docs/reference/ON-DEVICE-MODELS.md carry it).
//
// Why not FluidAudio's DiarizerManager: the FluidAudio xcframework the app links (0.18.3) was built with the Diarizer trait OFF
// (mobile/scripts/build-xcframeworks.sh: `--traits TTS`); `nm` on libFluidAudio.a finds no DiarizerManager, EmbeddingExtractor or
// DiarizerModels. Core ML is Apple's own framework, so this runs the same model file with the input layout FluidAudio's
// EmbeddingExtractor uses (read in FluidAudio v0.17.7 source): waveform [3, 160000] with the speech repeat-padded into row 0, mask
// [3, 589] with row 0 all ones, output `embedding` [3, 256], row 0.
//
// WAYS THIS CAN FAIL (each answered in code; not measured on a phone):
//  1. No network or a failed download: `ensureDownloaded` returns false; the files are written to a temporary folder and moved into place
//     only when every file has the expected size (and the weights the expected SHA-256), so a half download is never loaded.
//  2. The model fails to load (a damaged file, an OS that cannot compile it): `isReady` stays false; the gate lets every turn pass.
//  3. Silence or a very short stretch: `embed` returns nil; the gate calls it unknown.
//  4. A non-finite output: nil.

public final class WeSpeakerEmbedder: VoiceEmbedder, @unchecked Sendable {
    public static let repository = "FluidInference/speaker-diarization-coreml"
    /// The repository commit these file sizes and hashes were read from (a pinned revision: the files cannot change under us).
    public static let revision = "7eab74f3463c736418d0eb10296261ae581146c9"
    static let folder = "wespeaker_v2.mlmodelc"
    static let files: [(path: String, size: Int, sha256: String?)] = [
        ("analytics/coremldata.bin", 243, "d2b1fcde6121aea3ff0e14c1dc50d09dacb0314a2e89156353c31804230a422f"),
        ("coremldata.bin", 359, "6feb2472a71fa9d8a84020c85206138a4f6261c565c9884bf518d59dd5838da7"),
        ("metadata.json", 2738, nil),
        ("model.mil", 706_900, nil),
        ("weights/weight.bin", 7_243_904, "34004f6798d35cad7071e2fdc67e63faaa782f53697e1cb49bcb452cf81ae151"),
    ]
    public static let attribution = "Voice model: WeSpeaker ResNet34-LM (VoxCeleb) as published by pyannote, converted to Core ML by Fluid Inference (the files are modified conversions). Licensed under CC BY 4.0 (https://creativecommons.org/licenses/by/4.0/)."

    private let root: URL
    private let lock = NSLock()
    private var model: MLModel?
    private var loadTried = false

    private let door: DownloadDoor
    public init(root: URL, door: DownloadDoor = .shared) { self.root = root; self.door = door }
    public static func standard() -> WeSpeakerEmbedder {
        let d = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio/models", isDirectory: true)
        return WeSpeakerEmbedder(root: d)
    }
    var modelDir: URL { root.appendingPathComponent(Self.folder, isDirectory: true) }

    public var onDisk: Bool {
        Self.files.allSatisfy { f in
            let u = modelDir.appendingPathComponent(f.path)
            return ((try? FileManager.default.attributesOfItem(atPath: u.path)[.size]) as? Int) == f.size
        }
    }
    public var isReady: Bool { lock.lock(); defer { lock.unlock() }; return model != nil }

    /// Fetch the model if it is not on the phone (about 8 MB), then load it. Quiet: false on any failure, nothing shown to the reader.
    @discardableResult
    public func ensureReady() async -> Bool {
        if isReady { return true }
        if !onDisk { guard await download() else { return false } }
        return load()
    }

    func load() -> Bool {
        lock.lock(); defer { lock.unlock() }
        if model != nil { return true }
        let cfg = MLModelConfiguration(); cfg.computeUnits = .cpuAndNeuralEngine
        guard let m = try? MLModel(contentsOf: modelDir, configuration: cfg) else { return false }
        model = m; return true
    }

    func download() async -> Bool {
        guard (try? door.authorize(model: ModelOffer.speakerID)) != nil else { return false }       // the door, before any file is touched
        let fm = FileManager.default
        let tmp = root.appendingPathComponent("download-\(UUID().uuidString.prefix(8))", isDirectory: true)
        defer { try? fm.removeItem(at: tmp) }
        do {
            for f in Self.files {
                let dst = tmp.appendingPathComponent(f.path)
                try fm.createDirectory(at: dst.deletingLastPathComponent(), withIntermediateDirectories: true)
                let url = URL(string: "https://huggingface.co/\(Self.repository)/resolve/\(Self.revision)/\(Self.folder)/\(f.path)")!
                let (data, resp) = try await door.session(for: ModelOffer.speakerID).data(from: url)       // the door: consent and a transport that cannot spend mobile data unless agreed
                guard (resp as? HTTPURLResponse)?.statusCode == 200, data.count == f.size else { return false }
                if let want = f.sha256, Self.sha256Hex(data) != want { return false }
                try data.write(to: dst)
            }
            try? fm.removeItem(at: modelDir)
            try fm.createDirectory(at: root, withIntermediateDirectories: true)
            try fm.moveItem(at: tmp, to: modelDir)
            return true
        } catch { return false }
    }

    public func embed(_ samples: [Float]) async -> [Float]? {
        lock.lock(); let m = model; lock.unlock()
        guard let m, samples.count >= 8_000 else { return nil }
        guard let wave = try? MLMultiArray(shape: [3, 160_000], dataType: .float32),
              let mask = try? MLMultiArray(shape: [3, 589], dataType: .float32) else { return nil }
        let w = wave.dataPointer.assumingMemoryBound(to: Float.self), k = mask.dataPointer.assumingMemoryBound(to: Float.self)
        for i in 0..<(3 * 160_000) { w[i] = 0 }
        for i in 0..<(3 * 589) { k[i] = 0 }
        let n = min(samples.count, 160_000)
        for i in 0..<160_000 { w[i] = samples[i % n] }          // repeat-padded, as FluidAudio does
        for i in 0..<589 { k[i] = 1 }                           // the whole stretch is the one speaker
        guard let out = try? await m.prediction(from: MLDictionaryFeatureProvider(dictionary: ["waveform": wave, "mask": mask])),
              let e = out.featureValue(for: "embedding")?.multiArrayValue, e.count >= 256 else { return nil }
        let p = e.dataPointer.assumingMemoryBound(to: Float.self)
        var v = (0..<256).map { p[$0] }
        guard v.allSatisfy(\.isFinite) else { return nil }
        let len = v.reduce(0) { $0 + $1 * $1 }.squareRoot()
        guard len > 1e-6 else { return nil }
        v = v.map { $0 / len }
        return v
    }

    static func sha256Hex(_ data: Data) -> String {
        SHA256.hash(data: data).map { String(format: "%02x", $0) }.joined()
    }
}
