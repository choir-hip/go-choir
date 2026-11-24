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

    // Review round 3: Phonon's pack is a subfolder of Pocket's repo folder (`pocket-tts/phonon`). Each model owns ITS files only: neither's seal covers the other's, and neither's Remove, purge or retire deletes the other's.
    //  R5 with both installed and sealed, removing Pocket leaves Phonon sealed and present (and its files on disk); removing Phonon leaves Pocket sealed and present.
    //  R6 Pocket's manifest holds none of Phonon's files; a killed fetch's clean-up (`purge`) and a new pin's (`retire`) of Pocket also leave Phonon alone.
    func bothInstalled() -> (root: URL, manifests: URL, pocket: ModelPin, phonon: ModelPin) {
        let base = tmp(), manifests = tmp()
        KnownModels.ttsRootProvider = { base.appendingPathComponent("fluidaudio/Models", isDirectory: true) }
        write(KnownModels.ttsRoot, "pocket-tts/v2/en/model.bin"); write(KnownModels.ttsRoot, "pocket-tts/constants.bin")
        write(KnownModels.ttsRoot, "pocket-tts/phonon/phonon-31m-en/constants_bin/en_us_Easton.safetensors"); write(KnownModels.ttsRoot, "pocket-tts/phonon/phonon-31m-en/model.bin")
        let pocket = try! XCTUnwrap(ModelPins.current("pocket-tts", root: manifests)), phonon = try! XCTUnwrap(ModelPins.current("phonon-31m", root: manifests))
        XCTAssertTrue(pocket.seal?() ?? false); XCTAssertTrue(phonon.seal?() ?? false)
        return (KnownModels.ttsRoot, manifests, pocket, phonon)
    }
    func testR5RemovingOneOfThePairLeavesTheOtherSealedAndOnDisk() {
        let a = bothInstalled()
        a.pocket.erase?()
        XCTAssertFalse(FileManager.default.fileExists(atPath: a.root.appendingPathComponent("pocket-tts/v2").path), "Pocket's own files went")
        XCTAssertTrue(FileManager.default.fileExists(atPath: a.root.appendingPathComponent("pocket-tts/phonon/phonon-31m-en/model.bin").path), "Phonon's did not")
        XCTAssertTrue(a.phonon.present(), "Phonon is still sealed and present")
        XCTAssertFalse(a.pocket.onDisk?() ?? true)
        let b = bothInstalled()
        b.phonon.erase?()
        XCTAssertTrue(b.pocket.present(), "removing Phonon does not break Pocket's seal")
        XCTAssertTrue(FileManager.default.fileExists(atPath: b.root.appendingPathComponent("pocket-tts/v2/en/model.bin").path))
    }
    func testR6PocketsManifestHoldsNoneOfPhononsFilesAndItsCleanUpsSpareThem() {
        let a = bothInstalled()
        let man = try! XCTUnwrap(ModelManifests.build(pin: "p", base: a.root, folders: ["pocket-tts"], excluding: ["pocket-tts/phonon"]))
        XCTAssertFalse(man.files.contains { $0.path.contains("phonon") }); XCTAssertTrue(man.files.contains { $0.path == "pocket-tts/constants.bin" })
        a.pocket.purge?()
        XCTAssertTrue(a.phonon.present(), "a killed Pocket fetch's clean-up leaves Phonon alone")
        let c = bothInstalled()
        let old = ModelManifest(pin: "fluidaudio-0.0.1|pocket-tts", files: [.init(path: "pocket-tts/v2/en/model.bin", bytes: 1, sha256: "00")])
        XCTAssertTrue(ModelManifests.write(old, id: "pocket-tts", root: c.manifests), "an older version's record exists, so retire has something to retire")
        XCTAssertTrue(c.pocket.retire?() ?? false)
        XCTAssertTrue(c.phonon.present(), "another version's clean-up of Pocket spares Phonon")
        XCTAssertFalse(FileManager.default.fileExists(atPath: c.root.appendingPathComponent("pocket-tts/v2").path))
    }

    // The configurator's delete path (a rejected configuration's models) for the voice models under the TTS root.
    func testR7TheConfiguratorsDeleteOfPocketSparesPhononAndTheOtherWayRound() async {
        let b = bothInstalled()
        let backend = LiveConfigBackend(store: ModelStore(root: tmp(), registerKnown: true, session: nil, door: .open), managedRoot: tmp())
        let pocket = try! XCTUnwrap(CandidateTable.candidate("pocket-tts", .voice)), phonon = try! XCTUnwrap(CandidateTable.candidate("phonon-31m", .voice))
        await backend.delete(pocket)
        XCTAssertFalse(FileManager.default.fileExists(atPath: b.root.appendingPathComponent("pocket-tts/v2").path), "Pocket's own files went")
        XCTAssertTrue(FileManager.default.fileExists(atPath: b.root.appendingPathComponent("pocket-tts/phonon/phonon-31m-en/model.bin").path), "Phonon's did not")
        let c = bothInstalled()
        await backend.delete(phonon)
        XCTAssertTrue(FileManager.default.fileExists(atPath: c.root.appendingPathComponent("pocket-tts/v2/en/model.bin").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: c.root.appendingPathComponent("pocket-tts/phonon").path))
    }
    func testR8WithOnlyPhononInstalledPocketIsNotOnDisk() async {
        let base = tmp()
        KnownModels.ttsRootProvider = { base.appendingPathComponent("fluidaudio/Models", isDirectory: true) }
        write(KnownModels.ttsRoot, "pocket-tts/phonon/phonon-31m-en/model.bin")
        let backend = LiveConfigBackend(store: ModelStore(root: tmp(), registerKnown: true, session: nil, door: .open), managedRoot: tmp())
        let onDisk = await backend.isOnDisk(try! XCTUnwrap(CandidateTable.candidate("pocket-tts", .voice)))
        XCTAssertFalse(onDisk, "Phonon's folder is not Pocket's")
    }
}
