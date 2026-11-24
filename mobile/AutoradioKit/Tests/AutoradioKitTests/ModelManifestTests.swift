import XCTest
import CryptoKit
@testable import AutoradioKit

// Round 5: readiness by MANIFEST, not by "90% of a round number". A library-managed model is ready iff a manifest (every file's relative path, exact length, SHA-256 of the files as they were when the check passed) exists for the current pin and every
// file is present with that length. Trust on first consented fetch: the digests are of what the first consented fetch left on disk.
final class ModelManifestTests: XCTestCase {
    func tmp() -> URL { let d = FileManager.default.temporaryDirectory.appendingPathComponent("man-\(UUID().uuidString)", isDirectory: true); try? FileManager.default.createDirectory(at: d, withIntermediateDirectories: true); return d }
    func put(_ base: URL, _ path: String, _ bytes: Int) { let u = base.appendingPathComponent(path); try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true); try? Data((0..<bytes).map { UInt8($0 % 251) }).write(to: u) }
    /// A fake library cache: two folders under a base; the "model card" says 350 MB and the library keeps a few KB: the round number must not matter.
    func make() -> (base: URL, store: URL, spec: ManagedModelSpec) {
        let base = tmp(), store = tmp()
        put(base, "kokoro/a.mlmodelc/weights/w.bin", 3000); put(base, "kokoro/a.mlmodelc/model.mil", 700); put(base, "kokoro-82m-coreml/voices.json", 120)
        return (base, store, ManagedModelSpec(id: "kk", label: "k", source: "hf:x", approxBytes: 350_000_000, license: "x", cachePath: base, cacheFolders: ["kokoro", "kokoro-82m-coreml"]))
    }
    func pins(_ m: (base: URL, store: URL, spec: ManagedModelSpec)) -> FileVoiceReadiness {
        FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store) })
    }

    func testM1ReadyWhenTheManifestIsWrittenAtThePassWhateverTheCardSays() {
        let m = make(), r = pins(m)
        XCTAssertFalse(r.isReady(model: "kk"), "nothing passed yet")
        r.markReady(model: "kk")                                                       // the verifier's pass: seals the manifest
        XCTAssertTrue(r.isReady(model: "kk"), "3.8 KB on disk of a 350 MB card figure: exact, not 90%")
        let man = ModelManifests.read("kk", root: m.store)!
        XCTAssertEqual(Set(man.files.map(\.path)), ["kokoro/a.mlmodelc/weights/w.bin", "kokoro/a.mlmodelc/model.mil", "kokoro-82m-coreml/voices.json"])
        XCTAssertEqual(man.files.first { $0.path.hasSuffix("w.bin") }?.bytes, 3000)
    }
    func testM2AMissingFileIsNotReadyEvenWhenAlmostEverythingIsThere() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        try? FileManager.default.removeItem(at: m.base.appendingPathComponent("kokoro-82m-coreml/voices.json"))      // 3% of the bytes: the old 90% rule called it ready
        XCTAssertFalse(r.isReady(model: "kk"))
    }
    func testM3ATruncatedFileIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        put(m.base, "kokoro/a.mlmodelc/weights/w.bin", 2999); XCTAssertFalse(r.isReady(model: "kk"))
    }
    func testM4AFetchDemotesFirst_SoAFailedReFetchIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk"); XCTAssertTrue(r.isReady(model: "kk"))
        r.markNotReady(model: "kk")                                                    // what the provider does before every fetch
        XCTAssertNil(ModelManifests.read("kk", root: m.store), "the manifest is gone")
        XCTAssertFalse(r.isReady(model: "kk"), "the files are all there and it is still not ready: a re-fetch that fails must not leave it ready")
    }
    func testM5APinChangeIsNotReady() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let other = FileVoiceReadiness(file: m.store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: m.spec, root: m.store, libraryPin: "fluidaudio-9.9.9") })
        XCTAssertFalse(other.isReady(model: "kk"))
    }
    func testM6HashesAreVerifiedWhenAskedAndCatchAChangeThatKeepsTheLength() {
        let m = make(), r = pins(m); r.markReady(model: "kk")
        let man = ModelManifests.read("kk", root: m.store)!
        XCTAssertTrue(ModelManifests.verifyHashes(man, base: m.base))
        var d = try! Data(contentsOf: m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil")); d[0] ^= 0xFF; try! d.write(to: m.base.appendingPathComponent("kokoro/a.mlmodelc/model.mil"))
        XCTAssertTrue(r.isReady(model: "kk"), "lengths only on every launch")
        XCTAssertFalse(ModelManifests.verifyHashes(man, base: m.base), "the hash check (at the check that grants readiness, and after a failed load) sees it")
    }
    func testM7AnEmptyFolderCannotBeSealed() {
        let base = tmp(), store = tmp()
        let spec = ManagedModelSpec(id: "kk", label: "k", source: "hf:x", approxBytes: 5, license: "x", cachePath: base, cacheFolders: ["nothing"])
        let r = FileVoiceReadiness(file: store.appendingPathComponent("ready.json"), pins: { ModelPins.managed($0, spec: spec, root: store) })
        r.markReady(model: "kk"); XCTAssertFalse(r.isReady(model: "kk"), "no files, no manifest, no readiness")
    }

    // Round 5: a library is initialised only for a model that is ready by manifest, or while it is being fetched or checked (LibraryScope). Anything else is not a load: it would be a fetch of missing files through someone else's window.
    func testS1ALibraryInitialiseForAModelThatIsNotReadyIsRefused() async {
        let rec = MemoryVoiceReadiness()
        XCTAssertThrowsError(try LibraryScope.require("kokoro-82m-ane", readiness: rec))
        XCTAssertNoThrow(try LibraryScope.$checking.withValue("kokoro-82m-ane") { try LibraryScope.require("kokoro-82m-ane", readiness: rec) }, "inside its own fetch or check")
        XCTAssertThrowsError(try LibraryScope.$checking.withValue("pocket-tts") { try LibraryScope.require("kokoro-82m-ane", readiness: rec) }, "another model's scope does not cover it")
        rec.markReady(model: "kokoro-82m-ane"); XCTAssertNoThrow(try LibraryScope.require("kokoro-82m-ane", readiness: rec))
    }
    func testS2WithheldIsDerivedFromTheManifestAndTheLoadEntryPoint() {
        let noLocal = ManagedModelSpec(id: "k", label: "k", source: "x", approxBytes: 1, license: "x", cachePath: nil, cacheFolders: ["k"], loadsWithoutFetching: false)
        let local = ManagedModelSpec(id: "t", label: "t", source: "x", approxBytes: 1, license: "x", cachePath: nil, cacheFolders: ["t"], loadsWithoutFetching: true)
        XCTAssertTrue(ModelOffer.isWithheld(noLocal, ready: false), "a library with no local-only load, with no manifest: withheld")
        XCTAssertFalse(ModelOffer.isWithheld(noLocal, ready: true), "offered once its manifest exists")
        XCTAssertFalse(ModelOffer.isWithheld(local, ready: false), "a library that loads from the folder without fetching is offered")
        for id in ["kokoro-82m-ane", "pocket-tts", "phonon-31m", "silero-vad-v6.2.1"] { XCTAssertFalse(KnownModels.managed.first { $0.id == id }!.loadsWithoutFetching, id) }
        for id in ["parakeet-eou-120m", "parakeet-tdt-0.6b-v2"] { XCTAssertTrue(KnownModels.managed.first { $0.id == id }!.loadsWithoutFetching, id) }
    }
}
