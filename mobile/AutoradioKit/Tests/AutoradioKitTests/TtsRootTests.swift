import XCTest
@testable import AutoradioKit

// A library's voice models live where the LIBRARY puts them, not where we wrote a string (owner bug, 2026-11-24: Kokoro, Phonon and Pocket downloaded and loaded on the phone and the row said "its files could not be sealed").
// FluidAudio 0.18.3 keeps TTS models under `Application Support/fluidaudio/Models` (lower case, TtsCacheDirectory) on iOS; the recogniser and the detector under `FluidAudio/Models`. iOS's volume is case-sensitive, so a
// folder named with the other case is a folder that does not exist: the seal found no files and failed. The Mac's volume is not, so every earlier test passed here.
//  R1 the TTS root and the recogniser root differ by name, string-exact, on iOS (`fluidaudio` and `FluidAudio`); the voice models are under the TTS root.
//  R2 a seal over a tree laid out the way the library lays it out (a temp tree, no volume case rules involved: the paths are looked up exactly as spelt) succeeds for Kokoro, Pocket TTS and Phonon, and the model is then present.
//  R3 Remove deletes the real folders.
//  R4 the Phonon folder is the library's own: the pocket repo's folder name, then `phonon`.
final class TtsRootTests: XCTestCase {
    func tmp() -> URL { let d = FileManager.default.temporaryDirectory.appendingPathComponent("tts-\(UUID().uuidString)", isDirectory: true); try? FileManager.default.createDirectory(at: d, withIntermediateDirectories: true); return d }
    override func tearDown() { KnownModels.ttsRootProvider = KnownModels.defaultTtsRoot }
    func write(_ root: URL, _ path: String, _ text: String = "x") {
        let u = root.appendingPathComponent(path); try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true); try? text.write(to: u, atomically: true, encoding: .utf8)
    }
    func testR1TheTtsRootIsTheLibrarysAndTheVoiceModelsLiveUnderIt() {
        #if os(iOS)
        XCTAssertTrue(KnownModels.ttsRoot.path.hasSuffix("/fluidaudio/Models"), KnownModels.ttsRoot.path)
        #else
        XCTAssertTrue(KnownModels.ttsRoot.path.hasSuffix("/.cache/fluidaudio/Models"), KnownModels.ttsRoot.path)
        #endif
        XCTAssertTrue(KnownModels.fluidRoot.path.hasSuffix("/FluidAudio/Models"))
        for id in ["kokoro-82m-ane", "pocket-tts", "phonon-31m"] { XCTAssertEqual(KnownModels.managed.first { $0.id == id }?.cachePath, KnownModels.ttsRoot, id) }
        for id in ["silero-vad-v6.2.1", "parakeet-eou-120m", "parakeet-tdt-0.6b-v2"] { XCTAssertEqual(KnownModels.managed.first { $0.id == id }?.cachePath, KnownModels.fluidRoot, id) }
    }
    func testR2ASealOverTheLibrarysLayoutSucceeds() {
        let base = tmp(), manifests = tmp()
        KnownModels.ttsRootProvider = { base.appendingPathComponent("fluidaudio/Models", isDirectory: true) }
        let root = KnownModels.ttsRoot
        write(root, "kokoro-82m-coreml/ANE-v3/rev1/en/model.bin"); write(root, "kokoro/g2p.json")
        write(root, "pocket-tts/v2/en/model.bin")
        write(root, "pocket-tts/phonon/phonon-31m-en/constants_bin/en_us_Easton.safetensors")
        for id in ["kokoro-82m-ane", "pocket-tts", "phonon-31m"] {
            let pin = try! XCTUnwrap(ModelPins.current(id, root: manifests))
            XCTAssertTrue(pin.seal?() ?? false, "\(id): its files were found where the library put them and sealed")
            XCTAssertTrue(pin.present(), "\(id) is present after the seal")
        }
    }
    func testR2bAFolderUnderTheOtherCaseSealsNothing() {
        let base = tmp(), manifests = tmp()
        KnownModels.ttsRootProvider = { base.appendingPathComponent("fluidaudio/Models", isDirectory: true) }
        write(base.appendingPathComponent("FluidAudio/Models"), "kokoro/g2p.json")                // the old string: another tree
        #if os(iOS)
        XCTAssertFalse(try! XCTUnwrap(ModelPins.current("kokoro-82m-ane", root: manifests)).seal?() ?? true)
        #endif
    }
    func testR3RemoveDeletesTheRealFolders() {
        let base = tmp(), manifests = tmp()
        KnownModels.ttsRootProvider = { base.appendingPathComponent("fluidaudio/Models", isDirectory: true) }
        write(KnownModels.ttsRoot, "pocket-tts/phonon/phonon-31m-en/a.bin")
        let pin = try! XCTUnwrap(ModelPins.current("phonon-31m", root: manifests))
        XCTAssertTrue(pin.onDisk?() ?? false)
        pin.purge?()
        XCTAssertFalse(FileManager.default.fileExists(atPath: KnownModels.ttsRoot.appendingPathComponent("pocket-tts/phonon").path))
    }
}
