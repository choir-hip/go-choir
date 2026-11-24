import XCTest
import AVFoundation
import FluidAudio
import CryptoKit
import Speech
@testable import AutoradioKit
@testable import AutoradioModels

// Fluid Speech Mini, the recogniser the owner chose, run through Core ML on the Mac over the fixture speech in the repository.
// THESE ARE MAC NUMBERS, NOT PHONE NUMBERS: the Neural Engine, the memory and the heat of a phone are not measured here.
//
// The failure list (each line is a test below; written before the stage was run):
//  F1 The model list is wrong: a file without a 64-hex digest, a duplicate, a copy of a file not in the list, a URL not pinned to the commit.
//  F2 A download that does not match its digest is accepted, or a half-fetched folder is called present.
//  F3 The stage begins without the files and tries the network or hangs: it must throw `unavailable` at once.
//  F4 The stage hears the fixtures wrong: word error above the bound on the three synthesised turns.
//  F5 Silence or a few samples become words (a cough, a door), or crash the model's one-second minimum.
//  F6 The final transcript is not one pass over the speech-bounded audio: frames appended in small pieces must give the same words as one buffer.
//  F7 Two sessions at once load two copies of the model (one slot).
//
// The model files come from ~/.cache/autoradio-test-models/fluid-speech-mini (fetched once, through ModelStore.ensureBundle, so the same
// download path the app uses is exercised and every file is checked against its pinned SHA-256). Offline with nothing cached: the model tests are
// skipped with a message that says so; the other tests still run.
final class FluidSpeechMiniTests: XCTestCase {
    static let cacheRoot = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".cache/autoradio-test-models", isDirectory: true)

    /// The model folder, fetched once; nil with a reason when it cannot be had.
    static func modelDirectory() async -> (URL?, String) {
        let store = ModelStore(root: cacheRoot)
        let spec = KnownModels.fluidSpeechMini
        if let d = await store.bundleDirectoryIfPresent(spec) { return (d, "") }
        do { return (try await store.ensureBundle(spec), "") }
        catch { return (nil, "Fluid Speech Mini is not in \(cacheRoot.path) and could not be fetched (\(error)); run once online to cache it") }
    }

    struct Fixture { var name: String, url: URL, reference: String }
    /// The three synthesised turns in mobile/fixtures (macOS `say`, no third-party audio) and the sentences they were made from
    /// (mobile/scripts/make-fixtures.py).
    static var fixtures: [Fixture] {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("fixtures", isDirectory: true)
        return [
            Fixture(name: "plain turn", url: root.appendingPathComponent("turns/plain/mic.caf"), reference: "Please tell me more about the harbour and the park."),
            Fixture(name: "turn over music", url: root.appendingPathComponent("turns/mixed/mic.caf"), reference: "Please tell me more about the harbour and the park."),
            Fixture(name: "programme clip", url: root.appendingPathComponent("programme/clip.caf"), reference: "This is a short clip from another source, played inside the edition."),
        ]
    }
    static func samples(_ f: Fixture) throws -> [Float] { try AudioFiles.readMono(f.url, rate: 16_000) }

    /// Words, lower case, no punctuation, one spelling of the one word the voice and the model spell differently.
    static func words(_ s: String) -> [String] {
        s.lowercased().replacingOccurrences(of: "harbour", with: "harbor")
            .components(separatedBy: CharacterSet.alphanumerics.inverted).filter { !$0.isEmpty }
    }
    /// Word error rate: word-level edit distance over the reference length.
    static func wer(_ hyp: String, _ ref: String) -> Double {
        let h = words(hyp), r = words(ref)
        var d = Array(0...h.count)
        for i in 1...max(r.count, 1) where i <= r.count {
            var prev = d[0]; d[0] = i
            for j in 1...max(h.count, 1) where j <= h.count {
                let cur = d[j]
                d[j] = min(d[j] + 1, d[j - 1] + 1, prev + (r[i - 1] == h[j - 1] ? 0 : 1))
                prev = cur
            }
            if h.isEmpty { d[0] = i }
        }
        return Double(d[h.count]) / Double(max(r.count, 1))
    }

    func frames(_ s: [Float], step: Int) -> [AudioFrame] {
        stride(from: 0, to: s.count, by: step).map { AudioFrame(samples: Array(s[$0..<min(s.count, $0 + step)]), sampleRate: 16_000, t: Double($0) / 16_000) }
    }

    // MARK: F1

    func testTheModelListIsConsistent() {
        let b = KnownModels.fluidSpeechMini
        XCTAssertEqual(b.commit.count, 40)
        XCTAssertEqual(Set(b.files.map(\.path)).count, b.files.count, "a path is listed twice")
        for f in b.files {
            XCTAssertEqual(f.sha256.count, 64, f.path); XCTAssertTrue(f.sha256.allSatisfy { $0.isHexDigit && !$0.isUppercase }, f.path)
            XCTAssertGreaterThan(f.bytes, 0, f.path)
            XCTAssertTrue(b.url(for: f).absoluteString.contains("/resolve/\(b.commit)/"), "the URL must name the commit, not a branch")
        }
        for c in b.copies { XCTAssertTrue(b.files.contains { $0.path.hasPrefix(c[0] + "/") }, "copy source \(c[0]) is not in the list") }
        XCTAssertTrue(b.files.contains { $0.path == "LICENSE" } && b.files.contains { $0.path == "NOTICE" }, "CC BY 4.0: LICENSE and NOTICE stay with the files")
        XCTAssertTrue(b.credit.contains("Fluid Speech Mini by FluidVoice"))
        XCTAssertTrue(KnownModels.ids.contains(b.id), "the model list (catalog) names it")
        XCTAssertEqual(KnownModels.managed.first { $0.id == b.id }?.approxBytes, b.totalBytes)
    }

    // MARK: F2

    /// A tiny bundle served from memory: the store fetches it, checks every digest, makes the copy; a wrong digest is refused and the folder is not present.
    func testTheStoreChecksEveryFileAndOnlyAWholeFolderIsPresent() async throws {
        let payloads: [String: Data] = ["a.mlmodelc/w.bin": Data("alpha".utf8), "vocab.json": Data("{}".utf8)]
        func sha(_ d: Data) -> String { SHA256Probe.hex(d) }
        let good = ModelBundleSpec(id: "t", repo: "x/y", commit: String(repeating: "a", count: 40),
            files: payloads.map { .init(path: $0.key, bytes: Int64($0.value.count), sha256: sha($0.value)) }.sorted { $0.path < $1.path },
            folder: "t", license: "x", credit: "x", copies: [["a.mlmodelc", "acopy.mlmodelc"]])
        StubURLProtocol.responses = Dictionary(uniqueKeysWithValues: good.files.map { (good.url(for: $0), payloads[$0.path]!) })
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubURLProtocol.self]
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("bundle-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let store = ModelStore(root: root, session: URLSession(configuration: cfg))
        let none = await store.bundleDirectoryIfPresent(good)
        XCTAssertNil(none, "nothing fetched yet: not present")
        let dir = try await store.ensureBundle(good)
        let present = await store.bundleDirectoryIfPresent(good)
        XCTAssertEqual(present, dir)
        XCTAssertEqual(try Data(contentsOf: dir.appendingPathComponent("acopy.mlmodelc/w.bin")), Data("alpha".utf8), "the copy the loader expects exists")
        // a half-fetched folder is not present
        try FileManager.default.removeItem(at: dir.appendingPathComponent("vocab.json"))
        let partial = await store.bundleDirectoryIfPresent(good)
        XCTAssertNil(partial)
        // a file that does not match its digest is refused
        var bad = good; bad.id = "t2"; bad.folder = "t2"
        bad.files[0].sha256 = String(repeating: "0", count: 64)
        StubURLProtocol.responses = Dictionary(uniqueKeysWithValues: bad.files.map { (bad.url(for: $0), payloads[$0.path]!) })
        do { _ = try await store.ensureBundle(bad); XCTFail("a wrong digest was accepted") } catch {}
        let after = await store.bundleDirectoryIfPresent(bad)
        XCTAssertNil(after)
    }

    // S1 (review of d67c0cb77): a path from a served list must stay inside the model's own folder.
    func testAPathThatLeavesTheModelsFolderIsRefused() async throws {
        let sha = String(repeating: "a", count: 64)
        for bad in ["../escape.bin", "a/../../escape.bin", "/abs/escape.bin", "a//b", "a/./b", "a\\b", "", "a/", "a\u{0}b"] {
            let b = ModelBundleSpec(id: "t", repo: "x/y", commit: String(repeating: "a", count: 40), files: [.init(path: bad, bytes: 1, sha256: sha)], folder: "t", license: "x", credit: "x")
            XCTAssertFalse(b.pathsStayInside, "\(bad.debugDescription) was accepted")
            let entry = ModelListEntry(id: "t", job: "transcriber", label: "", repository: "x/y", revision: b.commit, files: [.init(path: bad, bytes: 1, sha256: sha)], license: "", credit: nil, recommendedFor: [], rank: 1)
            XCTAssertTrue(try ModelList.decode(ModelList(version: 1, models: [entry]).encoded()).models.isEmpty, "the list kept \(bad.debugDescription)")
            let root = FileManager.default.temporaryDirectory.appendingPathComponent("trav-\(UUID().uuidString)")
            defer { try? FileManager.default.removeItem(at: root) }
            do { _ = try await ModelStore(root: root).ensureBundle(b); XCTFail("fetched \(bad.debugDescription)") } catch {}
        }
        let folderOut = ModelBundleSpec(id: "t", repo: "x/y", commit: String(repeating: "a", count: 40), files: [], folder: "../out", license: "", credit: "")
        XCTAssertFalse(folderOut.pathsStayInside)
        let ok = ModelBundleSpec(id: "t", repo: "x/y", commit: String(repeating: "a", count: 40), files: [.init(path: "a.mlmodelc/weights/w.bin", bytes: 1, sha256: sha)], folder: "t", license: "", credit: "")
        XCTAssertTrue(ok.pathsStayInside)
    }

    // S2 (review): a pin bump must replace the copy a loader reads, not keep the old network under the new name.
    func testACopyIsReplacedWhenTheSourceChanges() async throws {
        let v1 = Data("old".utf8), v2 = Data("newer network".utf8)
        func spec(_ d: Data) -> ModelBundleSpec { ModelBundleSpec(id: "t", repo: "x/y", commit: String(repeating: "a", count: 40),
            files: [.init(path: "j.mlmodelc/w.bin", bytes: Int64(d.count), sha256: SHA256Probe.hex(d))], folder: "t", license: "x", credit: "x", copies: [["j.mlmodelc", "jcopy.mlmodelc"]]) }
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubURLProtocol.self]
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("copy-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let store = ModelStore(root: root, session: URLSession(configuration: cfg))
        StubURLProtocol.responses = [spec(v1).url(for: spec(v1).files[0]): v1]
        let dir = try await store.ensureBundle(spec(v1))
        StubURLProtocol.responses = [spec(v2).url(for: spec(v2).files[0]): v2]
        _ = try await store.ensureBundle(spec(v2))
        XCTAssertEqual(try Data(contentsOf: dir.appendingPathComponent("jcopy.mlmodelc/w.bin")), v2)
    }

    // S5 (review): the one-slot claim, counted: two callers at once make the model once.
    func testTwoCallersAtOnceMakeTheModelOnce() async throws {
        let slot = ModelSlot<Int>("slot-count-test")
        let makes = Locked<Int>(0)
        let make: @Sendable () async throws -> (Int, @Sendable () async -> Void) = {
            makes.write { $0 += 1 }; try await Task.sleep(nanoseconds: 100_000_000); return (7, {})
        }
        async let a = slot.get(make), b = slot.get(make)
        _ = try await (a, b)
        XCTAssertEqual(makes.read { $0 }, 1)
    }

    // MARK: F3

    func testBeginWithoutTheFilesThrowsAtOnce() async {
        let stage = FluidSpeechMiniTranscriber(directory: { nil })
        do { _ = try await stage.begin(); XCTFail("it began with no model") }
        catch let e as LabError { XCTAssertTrue("\(e)".contains("not on this phone"), "\(e)") }
        catch { XCTFail("\(error)") }
    }

    // MARK: F4 to F7 (the model itself)

    func testHearsTheFixtureSpeech() async throws {
        let (dir, why) = await Self.modelDirectory()
        guard let dir else { throw XCTSkip(why) }
        let stage = FluidSpeechMiniTranscriber(directory: { dir })
        let first = Date(); _ = try await stage.begin(); let loadSeconds = Date().timeIntervalSince(first)
        print("FLUID-MINI Mac model load + warm-up: \(String(format: "%.2f", loadSeconds)) s")
        var totalErr = 0.0, totalWords = 0.0, totalSeconds = 0.0, totalTime = 0.0
        for f in Self.fixtures {
            let audio = try Self.samples(f)
            let seconds = Double(audio.count) / 16_000
            let s = try await stage.begin()
            for fr in frames(audio, step: 512) { s.append(fr) }
            let t0 = Date()
            let text = try await s.finish()
            let took = Date().timeIntervalSince(t0)
            let w = Self.wer(text, f.reference)
            print("FLUID-MINI \(f.name): \"\(text)\" | WER \(String(format: "%.3f", w)) | \(String(format: "%.3f", took)) s for \(String(format: "%.2f", seconds)) s of audio (\(String(format: "%.3f", took / seconds)) per second)")
            totalErr += w * Double(Self.words(f.reference).count); totalWords += Double(Self.words(f.reference).count)
            totalSeconds += seconds; totalTime += took
            XCTAssertLessThanOrEqual(w, 0.15, "\(f.name): heard \"\(text)\"")                       // F4
            XCTAssertFalse(text.isEmpty)
        }
        print("FLUID-MINI total: WER \(String(format: "%.3f", totalErr / totalWords)) over \(Int(totalWords)) words; \(String(format: "%.3f", totalTime / totalSeconds)) s per second of audio")
    }

    func testSilenceAndTinyAudioGiveNoWordsAndDoNotCrash() async throws {
        let (dir, why) = await Self.modelDirectory()
        guard let dir else { throw XCTSkip(why) }
        let stage = FluidSpeechMiniTranscriber(directory: { dir })
        let s = try await stage.begin()
        for fr in frames([Float](repeating: 0, count: 32_000), step: 512) { s.append(fr) }
        let silent = try await s.finish()
        XCTAssertEqual(silent, "", "two seconds of silence")                                           // F5
        let t = try await stage.begin()
        t.append(AudioFrame(samples: [Float](repeating: 0.01, count: 400), sampleRate: 16_000, t: 0))
        let tiny = try await t.finish()
        XCTAssertEqual(tiny, "", "a few hundred samples are not speech")
        let u = try await stage.begin()
        u.append(AudioFrame(samples: [Float](repeating: 0.001, count: 8_000), sampleRate: 16_000, t: 0))     // half a second: padded to the model's minimum
        _ = try await u.finish()
    }

    func testOnePassGivesTheSameWordsWhateverThePieces() async throws {
        let (dir, why) = await Self.modelDirectory()
        guard let dir else { throw XCTSkip(why) }
        let stage = FluidSpeechMiniTranscriber(directory: { dir })
        let audio = try Self.samples(Self.fixtures[0])
        var texts: [String] = []
        for step in [160, 4_096, audio.count] {
            let s = try await stage.begin()
            for fr in frames(audio, step: step) { s.append(fr) }
            texts.append(try await s.finish())
        }
        XCTAssertEqual(texts[0], texts[1]); XCTAssertEqual(texts[1], texts[2])                          // F6
        // frames at the phone's 48 kHz are resampled to 16 kHz by the stage, and still give the words
        let up = (0..<(audio.count * 3)).map { i -> Float in audio[i / 3] }
        let s = try await stage.begin()
        for fr in stride(from: 0, to: up.count, by: 1536).map({ AudioFrame(samples: Array(up[$0..<min(up.count, $0 + 1536)]), sampleRate: 48_000, t: Double($0) / 48_000) }) { s.append(fr) }
        let at48 = try await s.finish()
        XCTAssertLessThanOrEqual(Self.wer(at48, Self.fixtures[0].reference), 0.25, "48 kHz input: heard \"\(at48)\"")
    }

    func testTwoSessionsShareOneLoadedModel() async throws {
        let (dir, why) = await Self.modelDirectory()
        guard let dir else { throw XCTSkip(why) }
        let stage = FluidSpeechMiniTranscriber(directory: { dir })
        async let a = stage.begin(), b = stage.begin()
        _ = try await (a, b)
        // F7: one slot (counted in testTwoCallersAtOnceMakeTheModelOnce)
        let audio = try Self.samples(Self.fixtures[0])
        async let t1: String = { let s = try await stage.begin(); for fr in self.frames(audio, step: 512) { s.append(fr) }; return try await s.finish() }()
        async let t2: String = { let s = try await stage.begin(); for fr in self.frames(audio, step: 512) { s.append(fr) }; return try await s.finish() }()
        let (x, y) = try await (t1, t2)
        XCTAssertEqual(x, y)
    }

    // MARK: The same fixtures through Parakeet TDT, for the numbers beside (a measurement, run on request: AUTORADIO_COMPARE_PARAKEET=1; it
    // fetches Parakeet TDT 0.6B v2 once into ~/.cache/autoradio-test-models/parakeet-tdt-v2, about 480 MB)

    func testParakeetTDTOnTheSameFixturesForComparison() async throws {
        guard ProcessInfo.processInfo.environment["AUTORADIO_COMPARE_PARAKEET"] == "1" else { throw XCTSkip("set AUTORADIO_COMPARE_PARAKEET=1 to run the comparison") }
        let models = try await AsrModels.downloadAndLoad(to: Self.cacheRoot.appendingPathComponent("parakeet-tdt-v2"), version: .v2)
        let mgr = AsrManager(config: .default)
        try await mgr.loadModels(models)
        var st = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
        _ = try? await mgr.transcribe([Float](repeating: 0, count: 16_000), decoderState: &st)
        var err = 0.0, n = 0.0, secs = 0.0, took = 0.0
        for f in Self.fixtures {
            let audio = try Self.samples(f)
            var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            let t0 = Date()
            let text = try await mgr.transcribe(audio, decoderState: &state).text
            let dt = Date().timeIntervalSince(t0)
            let w = Self.wer(text, f.reference)
            print("PARAKEET-TDT \(f.name): \"\(text)\" | WER \(String(format: "%.3f", w)) | \(String(format: "%.3f", dt / (Double(audio.count) / 16_000))) per second")
            err += w * Double(Self.words(f.reference).count); n += Double(Self.words(f.reference).count)
            secs += Double(audio.count) / 16_000; took += dt
        }
        print("PARAKEET-TDT total: WER \(String(format: "%.3f", err / n)) over \(Int(n)) words; \(String(format: "%.3f", took / secs)) s per second of audio")
    }
}

// MARK: Real human speech: word error against a reference, Fluid Speech Mini beside Parakeet and Apple's recogniser
//
// The fixtures above are synthesised (macOS `say`), so every model hears them perfectly. This runs the same models over real readers: set
// AUTORADIO_REAL_SPEECH=<folder> holding `references.json` ([{"id","text"}]) and `<id>.wav` (16 kHz mono). The folder used for the figures in
// ON-DEVICE-MODELS.md section 11 is the 73 clips of the LibriSpeech "clean" validation subset
// (hf-internal-testing/librispeech_asr_dummy; LibriSpeech is CC BY 4.0, read by volunteers of LibriVox). Skipped when the variable is not set.
// Parakeet TDT (the cached v2 folder) runs when AUTORADIO_COMPARE_PARAKEET=1; Apple's recogniser runs only when this process is already
// authorised for speech (a test process on the Mac cannot ask), otherwise the figure is reported as not measured.
extension FluidSpeechMiniTests {
    struct Ref: Decodable { var id: String; var text: String }

    /// Errors and reference length over the whole set (corpus word error), not the mean of per-clip rates.
    struct Tally { var errors = 0.0, words = 0.0, audio = 0.0, took = 0.0
        mutating func add(_ hyp: String, _ ref: String, audio a: Double, took t: Double) {
            let n = Double(FluidSpeechMiniTests.words(ref).count)
            errors += FluidSpeechMiniTests.wer(hyp, ref) * n; words += n; audio += a; took += t
        }
        var rate: Double { errors / max(words, 1) }
    }

    /// A turn past the model's 15 s window (the engine allows 30 s): FluidAudio's chunking must not drop spans (review note: only Mike's relayed 31-minute run backed this).
    func testALongTurnKeepsEveryWord() async throws {
        guard let folder = ProcessInfo.processInfo.environment["AUTORADIO_REAL_SPEECH"] else { throw XCTSkip("set AUTORADIO_REAL_SPEECH=<folder of wav clips and references.json>") }
        let dir = URL(fileURLWithPath: folder)
        let refs = try JSONDecoder().decode([Ref].self, from: Data(contentsOf: dir.appendingPathComponent("references.json")))
        let (model, why) = await Self.modelDirectory()
        guard let model else { throw XCTSkip(why) }
        var audio: [Float] = [], text: [String] = []
        for r in refs where audio.count < 16_000 * 28 {
            audio += try AudioFiles.readMono(dir.appendingPathComponent("\(r.id).wav"), rate: 16_000); text.append(r.text)
        }
        XCTAssertGreaterThan(audio.count, 16_000 * 20, "the clips make a turn over 20 s")
        let s = try await FluidSpeechMiniTranscriber(directory: { model }).begin()
        for fr in frames(audio, step: 512) { s.append(fr) }
        let heard = try await s.finish()
        let w = Self.wer(heard, text.joined(separator: " "))
        print("REAL-SPEECH long turn \(audio.count / 16_000) s: WER \(String(format: "%.4f", w))")
        XCTAssertLessThanOrEqual(w, 0.10, "a long turn lost words: heard \(heard.count) characters")
    }

    func testRealSpeechWordError() async throws {
        guard let folder = ProcessInfo.processInfo.environment["AUTORADIO_REAL_SPEECH"] else { throw XCTSkip("set AUTORADIO_REAL_SPEECH=<folder of wav clips and references.json>") }
        let dir = URL(fileURLWithPath: folder)
        let refs = try JSONDecoder().decode([Ref].self, from: Data(contentsOf: dir.appendingPathComponent("references.json")))
        let (model, why) = await Self.modelDirectory()
        guard let model else { throw XCTSkip(why) }
        let clips = try refs.map { ($0, try AudioFiles.readMono(dir.appendingPathComponent("\($0.id).wav"), rate: 16_000)) }
        let hours = clips.reduce(0.0) { $0 + Double($1.1.count) / 16_000 } / 3600
        print("REAL-SPEECH \(clips.count) clips, \(String(format: "%.1f", hours * 60)) minutes of audio")

        // Fluid Speech Mini, through the stage the app runs (frames of 512 samples, then `finish`).
        let stage = FluidSpeechMiniTranscriber(directory: { model })
        _ = try await stage.begin()
        var fluid = Tally()
        for (r, audio) in clips {
            let s = try await stage.begin()
            for fr in frames(audio, step: 512) { s.append(fr) }
            let t0 = Date(); let text = try await s.finish()
            fluid.add(text, r.text, audio: Double(audio.count) / 16_000, took: Date().timeIntervalSince(t0))
        }
        print("REAL-SPEECH fluid-speech-mini: WER \(String(format: "%.4f", fluid.rate)) over \(Int(fluid.words)) words; \(String(format: "%.3f", fluid.took / fluid.audio)) s per second of audio")
        XCTAssertLessThanOrEqual(fluid.rate, 0.10, "Fluid Speech Mini on real readers")

        if ProcessInfo.processInfo.environment["AUTORADIO_COMPARE_PARAKEET"] == "1" {
            let models = try await AsrModels.downloadAndLoad(to: Self.cacheRoot.appendingPathComponent("parakeet-tdt-v2"), version: .v2)
            let mgr = AsrManager(config: .default)
            try await mgr.loadModels(models)
            var st = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            _ = try? await mgr.transcribe([Float](repeating: 0, count: 16_000), decoderState: &st)
            var tdt = Tally()
            for (r, audio) in clips {
                var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
                let t0 = Date(); let text = try await mgr.transcribe(audio, decoderState: &state).text
                tdt.add(text, r.text, audio: Double(audio.count) / 16_000, took: Date().timeIntervalSince(t0))
            }
            print("REAL-SPEECH parakeet-tdt-0.6b-v2: WER \(String(format: "%.4f", tdt.rate)) over \(Int(tdt.words)) words; \(String(format: "%.3f", tdt.took / tdt.audio)) s per second of audio")

            // The streaming model the app uses for live words, through its stage.
            let eou = ParakeetEOUTranscriber(StageParams())
            var live = Tally()
            do {
                for (r, audio) in clips {
                    let s = try await eou.begin()
                    for fr in frames(audio, step: 512) { s.append(fr) }
                    let t0 = Date(); let text = try await s.finish()
                    live.add(text, r.text, audio: Double(audio.count) / 16_000, took: Date().timeIntervalSince(t0))
                }
                print("REAL-SPEECH parakeet-eou-120m: WER \(String(format: "%.4f", live.rate)) over \(Int(live.words)) words; \(String(format: "%.3f", live.took / live.audio)) s per second of audio")
            } catch { print("REAL-SPEECH parakeet-eou-120m: not measured (\(error))") }
        }

        if SFSpeechRecognizer.authorizationStatus() == .authorized {
            var apple = Tally()
            let a = SFSpeechTranscriber(StageParams())
            for (r, audio) in clips {
                let s = try await a.begin()
                for fr in frames(audio, step: 512) { s.append(fr) }
                let t0 = Date(); let text = try await s.finish()
                apple.add(text, r.text, audio: Double(audio.count) / 16_000, took: Date().timeIntervalSince(t0))
            }
            print("REAL-SPEECH apple-speech: WER \(String(format: "%.4f", apple.rate)) over \(Int(apple.words)) words")
        } else {
            print("REAL-SPEECH apple-speech: not measured (this test process is not authorised for speech recognition, status \(SFSpeechRecognizer.authorizationStatus().rawValue); a Mac test process cannot ask)")
        }
    }
}

/// SHA-256 as lower-case hex, for the stub bundle.
enum SHA256Probe {
    static func hex(_ d: Data) -> String { SHA256.hash(data: d).map { String(format: "%02x", $0) }.joined() }
}

/// Serves fixed bytes for fixed URLs (supports the store's Range requests).
final class StubURLProtocol: URLProtocol {
    nonisolated(unsafe) static var responses: [URL: Data] = [:]
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        guard let url = request.url, let data = Self.responses[url] else {
            client?.urlProtocol(self, didReceive: HTTPURLResponse(url: request.url!, statusCode: 404, httpVersion: nil, headerFields: nil)!, cacheStoragePolicy: .notAllowed)
            client?.urlProtocolDidFinishLoading(self); return
        }
        client?.urlProtocol(self, didReceive: HTTPURLResponse(url: url, statusCode: 200, httpVersion: nil, headerFields: nil)!, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }
    override func stopLoading() {}
}
