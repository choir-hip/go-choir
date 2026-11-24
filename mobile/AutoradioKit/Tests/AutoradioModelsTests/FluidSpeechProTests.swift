import XCTest
import Darwin
import FluidAudio
@testable import AutoradioKit
@testable import AutoradioModels

// Fluid Speech Pro (FluidVoice, CC BY 4.0), the larger recogniser the owner named beside Mini (2026-11-24: "fluid speech pro is better than
// parakeet. so fluid speech mini or pro is the way to go rn"). Same loader as Mini (FluidAudio 0.18.3 `.ultra`, from a local folder): the stage
// is `FluidSpeechProTranscriber`, the files are `KnownModels.fluidSpeechPro`.
//
// The failure list (each line a test below, written before the stage and the pin existed; they did not compile, then failed, then passed):
//  P1 The pin is wrong: a missing or duplicated file, a digest that is not 64 hex digits, a URL not pinned to the commit, no LICENSE or NOTICE,
//     no credit, a size that is not the model card's 652 MB.
//  P2 The stage begins without the files and tries the network or hangs: it must throw `unavailable` at once.
//  P3 The stage does not hear: word error above the bound on the three synthesised turns, or silence becomes words.
//  P4 Pro and Mini share a slot: loading one hands out the other's manager (two 250 MB models are not one).
//  P5 (a measurement, run on request) AUTORADIO_REAL_SPEECH=<folder> AUTORADIO_REAL_MODEL=pro|mini|tdt-v2: word error, time per second of audio,
//     load time, peak memory of ONE model in ONE process (several models in a process hide each other's peak).
final class FluidSpeechProTests: XCTestCase {
    static var cacheRoot: URL { FluidSpeechMiniTests.cacheRoot }

    static func modelDirectory() async -> (URL?, String) {
        let store = ModelStore(root: cacheRoot)
        let spec = KnownModels.fluidSpeechPro
        if let d = await store.bundleDirectoryIfPresent(spec) { return (d, "") }
        do { return (try await store.ensureBundle(spec), "") }
        catch { return (nil, "Fluid Speech Pro is not in \(cacheRoot.path) and could not be fetched (\(error)); run once online to cache it") }
    }

    // MARK: P1

    func testTheProPinIsConsistent() {
        let b = KnownModels.fluidSpeechPro
        XCTAssertEqual(b.repo, "FluidVoice-Labs/fluid-speech-pro-coreml")
        XCTAssertEqual(b.commit, "3b9d38a93d606f84011e5a0a669b7ea146375a22")
        XCTAssertEqual(Set(b.files.map(\.path)).count, b.files.count, "a path is listed twice")
        for f in b.files {
            XCTAssertEqual(f.sha256.count, 64, f.path); XCTAssertTrue(f.sha256.allSatisfy { $0.isHexDigit && !$0.isUppercase }, f.path)
            XCTAssertGreaterThan(f.bytes, 0, f.path)
            XCTAssertTrue(b.url(for: f).absoluteString.contains("/resolve/\(b.commit)/"), "the URL must name the commit, not a branch")
        }
        for c in b.copies { XCTAssertTrue(b.files.contains { $0.path.hasPrefix(c[0] + "/") }, "copy source \(c[0]) is not in the list") }
        XCTAssertEqual(b.copies, [["JointDecision.mlmodelc", "JointDecisionv3.mlmodelc"]], "FluidAudio wants the v3 joint name")
        for need in ["LICENSE", "NOTICE", "parakeet_vocab.json", "Encoder.mlmodelc/weights/weight.bin"] { XCTAssertTrue(b.files.contains { $0.path == need }, need) }
        XCTAssertTrue(b.credit.contains("Fluid Speech Pro by FluidVoice") && b.credit.contains("Moondream Parakeet Ultra"))
        XCTAssertEqual(b.totalBytes, 652_324_639, "the model card says 652 MB")
        XCTAssertTrue(KnownModels.ids.contains(b.id) && KnownModels.bundles.contains { $0.id == b.id }, "the catalog names it")
        XCTAssertEqual(KnownModels.managed.first { $0.id == b.id }?.approxBytes, b.totalBytes)
    }

    // MARK: P2

    func testBeginWithoutTheFilesThrowsAtOnce() async {
        let stage = FluidSpeechProTranscriber(directory: { nil })
        do { _ = try await stage.begin(); XCTFail("it began with no model") }
        catch let e as LabError { XCTAssertTrue("\(e)".contains("Fluid Speech Pro is not on this phone"), "\(e)") }
        catch { XCTFail("\(error)") }
    }

    // MARK: P3, P4 (the model itself)

    func testHearsTheFixtureSpeechAndNotSilence() async throws {
        let (dir, why) = await Self.modelDirectory()
        guard let dir else { throw XCTSkip(why) }
        let stage = FluidSpeechProTranscriber(directory: { dir })
        let first = Date(); _ = try await stage.begin(); print("FLUID-PRO Mac model load + warm-up: \(String(format: "%.2f", Date().timeIntervalSince(first))) s")
        for f in FluidSpeechMiniTests.fixtures {
            let audio = try FluidSpeechMiniTests.samples(f)
            let s = try await stage.begin()
            for fr in Self.frames(audio, step: 512) { s.append(fr) }
            let text = try await s.finish()
            let w = FluidSpeechMiniTests.wer(text, f.reference)
            print("FLUID-PRO \(f.name): \"\(text)\" | WER \(String(format: "%.3f", w))")
            XCTAssertLessThanOrEqual(w, 0.15, "\(f.name): heard \"\(text)\"")
        }
        let s = try await stage.begin()
        for fr in Self.frames([Float](repeating: 0, count: 32_000), step: 512) { s.append(fr) }
        let silent = try await s.finish()
        XCTAssertEqual(silent, "", "two seconds of silence")
    }

    func testProAndMiniDoNotShareASlot() async throws {
        XCTAssertNotEqual(FluidSpeechProTranscriber.stageID, FluidSpeechMiniTranscriber.stageID)
        XCTAssertFalse(FluidSpeechProTranscriber.slot === FluidSpeechMiniTranscriber.slot)
        XCTAssertEqual(FluidSpeechProTranscriber.descriptor.needsModels, [KnownModels.fluidSpeechPro.id])
    }

    // MARK: P5, the measurement

    static func frames(_ s: [Float], step: Int) -> [AudioFrame] {
        stride(from: 0, to: s.count, by: step).map { AudioFrame(samples: Array(s[$0..<min(s.count, $0 + step)]), sampleRate: 16_000, t: Double($0) / 16_000) }
    }
    /// Bytes this process holds now, and the most it has held (the kernel's physical-footprint ledger: Core ML's weights are counted).
    static func footprint() -> (now: Double, peak: Double) {
        var info = task_vm_info_data_t(); var n = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<integer_t>.size)
        let r = withUnsafeMutablePointer(to: &info) { $0.withMemoryRebound(to: integer_t.self, capacity: Int(n)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &n) } }
        guard r == KERN_SUCCESS else { return (0, 0) }
        return (Double(info.phys_footprint) / 1_048_576, Double(info.ledger_phys_footprint_peak) / 1_048_576)
    }

    func testMeasureOneModelOnRealSpeech() async throws {
        let env = ProcessInfo.processInfo.environment
        guard let folder = env["AUTORADIO_REAL_SPEECH"], let which = env["AUTORADIO_REAL_MODEL"] else { throw XCTSkip("set AUTORADIO_REAL_SPEECH=<folder> and AUTORADIO_REAL_MODEL=pro|mini|tdt-v2") }
        let dir = URL(fileURLWithPath: folder)
        let refs = try JSONDecoder().decode([FluidSpeechMiniTests.Ref].self, from: Data(contentsOf: dir.appendingPathComponent("references.json")))
        let clips = try refs.map { ($0, try AudioFiles.readMono(dir.appendingPathComponent("\($0.id).wav"), rate: 16_000)) }
        let minutes = clips.reduce(0.0) { $0 + Double($1.1.count) / 16_000 } / 60
        let base = Self.footprint()
        var tally = FluidSpeechMiniTests.Tally()
        let t0 = Date(); var loadedAt = 0.0
        switch which {
        case "pro", "mini":
            let model: URL
            if which == "pro" { let (d, why) = await Self.modelDirectory(); guard let d else { throw XCTSkip(why) }; model = d }
            else { let (d, why) = await FluidSpeechMiniTests.modelDirectory(); guard let d else { throw XCTSkip(why) }; model = d }
            let stage: Transcriber = which == "pro" ? FluidSpeechProTranscriber(directory: { model }) : FluidSpeechMiniTranscriber(directory: { model })
            _ = try await stage.begin(); loadedAt = Date().timeIntervalSince(t0)
            for (r, audio) in clips {
                let s = try await stage.begin()
                for fr in Self.frames(audio, step: 512) { s.append(fr) }
                let t1 = Date(); let text = try await s.finish()
                tally.add(text, r.text, audio: Double(audio.count) / 16_000, took: Date().timeIntervalSince(t1))
            }
        case "tdt-v2":
            let models = try await AsrModels.downloadAndLoad(to: Self.cacheRoot.appendingPathComponent("parakeet-tdt-v2"), version: .v2)
            let mgr = AsrManager(config: .default)
            try await mgr.loadModels(models)
            var st = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            _ = try? await mgr.transcribe([Float](repeating: 0, count: 16_000), decoderState: &st); loadedAt = Date().timeIntervalSince(t0)
            for (r, audio) in clips {
                var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
                let t1 = Date(); let text = try await mgr.transcribe(audio, decoderState: &state).text
                tally.add(text, r.text, audio: Double(audio.count) / 16_000, took: Date().timeIntervalSince(t1))
            }
        default: XCTFail("AUTORADIO_REAL_MODEL is pro, mini or tdt-v2"); return
        }
        let m = Self.footprint()
        print("MEASURE \(which) \(dir.lastPathComponent): \(clips.count) clips, \(String(format: "%.1f", minutes)) min | WER \(String(format: "%.4f", tally.rate)) over \(Int(tally.words)) words | \(String(format: "%.4f", tally.took / tally.audio)) s per s of audio | load+warm-up \(String(format: "%.1f", loadedAt)) s | footprint before \(Int(base.now)) MB, after \(Int(m.now)) MB, peak \(Int(m.peak)) MB")
    }

    /// Developer measurement (2026-11-24, the Omni-ears comparison): one model, several set folders, each clip's transcript written to
    /// `<AUTORADIO_REAL_OUT>/<model>/<set>.jsonl` as {"id","hyp","took"} so another script scores the same text for every recogniser.
    /// AUTORADIO_REAL_ROOT=<folder of set folders> AUTORADIO_REAL_SETS=a,b,c AUTORADIO_REAL_OUT=<folder> AUTORADIO_REAL_MODEL=pro|mini|tdt-v2
    func testTranscribeSetsToFiles() async throws {
        let env = ProcessInfo.processInfo.environment
        guard let root = env["AUTORADIO_REAL_ROOT"], let sets = env["AUTORADIO_REAL_SETS"], let out = env["AUTORADIO_REAL_OUT"], let which = env["AUTORADIO_REAL_MODEL"] else { throw XCTSkip("set AUTORADIO_REAL_ROOT, AUTORADIO_REAL_SETS, AUTORADIO_REAL_OUT, AUTORADIO_REAL_MODEL") }
        let outDir = URL(fileURLWithPath: out).appendingPathComponent(which); try FileManager.default.createDirectory(at: outDir, withIntermediateDirectories: true)
        let t0 = Date(); var loadedAt = 0.0
        var run: ([Float]) async throws -> String
        switch which {
        case "pro", "mini":
            let model: URL
            if which == "pro" { let (d, why) = await Self.modelDirectory(); guard let d else { throw XCTSkip(why) }; model = d }
            else { let (d, why) = await FluidSpeechMiniTests.modelDirectory(); guard let d else { throw XCTSkip(why) }; model = d }
            let stage: Transcriber = which == "pro" ? FluidSpeechProTranscriber(directory: { model }) : FluidSpeechMiniTranscriber(directory: { model })
            _ = try await stage.begin(); loadedAt = Date().timeIntervalSince(t0)
            run = { audio in
                let s = try await stage.begin()
                for fr in Self.frames(audio, step: 512) { s.append(fr) }
                return try await s.finish()
            }
        case "tdt-v2":
            let models = try await AsrModels.downloadAndLoad(to: Self.cacheRoot.appendingPathComponent("parakeet-tdt-v2"), version: .v2)
            let mgr = AsrManager(config: .default)
            try await mgr.loadModels(models)
            var st = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            _ = try? await mgr.transcribe([Float](repeating: 0, count: 16_000), decoderState: &st); loadedAt = Date().timeIntervalSince(t0)
            run = { audio in
                var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
                return try await mgr.transcribe(audio, decoderState: &state).text
            }
        default: XCTFail("AUTORADIO_REAL_MODEL is pro, mini or tdt-v2"); return
        }
        for name in sets.split(separator: ",").map(String.init) {
            let dir = URL(fileURLWithPath: root).appendingPathComponent(name)
            let refs = try JSONDecoder().decode([FluidSpeechMiniTests.Ref].self, from: Data(contentsOf: dir.appendingPathComponent("references.json")))
            var lines = ""; var audioSecs = 0.0, took = 0.0
            for r in refs {
                let audio = try AudioFiles.readMono(dir.appendingPathComponent("\(r.id).wav"), rate: 16_000)
                let t1 = Date(); let text = (try? await run(audio)) ?? ""; let dt = Date().timeIntervalSince(t1)
                audioSecs += Double(audio.count) / 16_000; took += dt
                let obj: [String: Any] = ["id": r.id, "hyp": text, "took": dt, "audio_s": Double(audio.count) / 16_000]
                lines += String(data: try JSONSerialization.data(withJSONObject: obj), encoding: .utf8)! + "\n"
            }
            try lines.write(to: outDir.appendingPathComponent("\(name).jsonl"), atomically: true, encoding: .utf8)
            print("TRANSCRIBED \(which) \(name): \(refs.count) clips, \(String(format: "%.4f", took / max(audioSecs, 1))) s per s of audio")
        }
        let m = Self.footprint()
        print("TRANSCRIBED \(which) done | load+warm-up \(String(format: "%.1f", loadedAt)) s | footprint after \(Int(m.now)) MB, peak \(Int(m.peak)) MB")
    }
}
