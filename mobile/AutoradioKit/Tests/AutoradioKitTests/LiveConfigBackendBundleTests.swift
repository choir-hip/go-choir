import XCTest
@testable import AutoradioKit

// B2 (review of d67c0cb77): a folder model (Fluid Speech Mini) lives in the model store's root, not in FluidAudio's folder, so the backend
// never saw it on disk and a rejected configuration's 253 MB was never deleted.
final class LiveConfigBackendBundleTests: XCTestCase {
    func testAFolderModelIsOnDiskAndIsDeletedWithItsReadiness() async throws {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent("bundle-\(UUID().uuidString)")
        defer { try? FileManager.default.removeItem(at: root) }
        let store = ModelStore(root: root)
        let b = LiveConfigBackend(store: store, probe: { DeviceProbe.simulated(.tier8) })
        let c = try XCTUnwrap(CandidateTable.candidate("fluid-speech-mini", .transcriber))
        let off = await b.isOnDisk(c)
        XCTAssertFalse(off)
        let dir = root.appendingPathComponent(KnownModels.fluidSpeechMini.folder, isDirectory: true)
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        try Data("x".utf8).write(to: dir.appendingPathComponent("config.json"))
        let on = await b.isOnDisk(c)
        XCTAssertTrue(on, "a partly fetched folder is on disk too: it is what a rejected configuration must remove")
        Voices.readiness.markReady(model: c.id)
        await b.delete(c)
        XCTAssertFalse(FileManager.default.fileExists(atPath: dir.path))
        XCTAssertFalse(Voices.readiness.isReady(model: c.id))
    }
}
