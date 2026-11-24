import XCTest
import FluidAudio
@testable import AutoradioKit
@testable import AutoradioModels

// Our idea of where the voice models live is the library's own answer, string-exact (owner bug, 2026-11-24): no hand-written root can drift from FluidAudio's `TtsCacheDirectory` and `Repo.folderName`.
final class TtsRootLibraryTests: XCTestCase {
    override func tearDown() { KnownModels.ttsRootProvider = KnownModels.defaultTtsRoot }
    override func setUp() { ModelHub.register(into: StageRegistry()) }                      // what the app does at launch: it hands the library's directory to the catalog
    func testTheCatalogsTtsRootIsTheLibrarysDirectoryString_ExactIncludingCase() throws {
        let lib = try TtsCacheDirectory.ensure().appendingPathComponent("Models", isDirectory: true)
        XCTAssertEqual(KnownModels.ttsRoot.path, lib.path)
        XCTAssertEqual(KnownModels.managed.first { $0.id == "kokoro-82m-ane" }?.cachePath?.path, lib.path)
    }
    func testTheVoiceModelsFoldersAreTheLibrarysRepoFolders() {
        let pocket = Repo.pocketTts.folderName
        XCTAssertEqual(KnownModels.managed.first { $0.id == "pocket-tts" }?.cacheFolders, [pocket])
        XCTAssertEqual(KnownModels.managed.first { $0.id == "phonon-31m" }?.cacheFolders, [pocket + "/" + PocketTtsSmallModel.phonon31m.repoSubdirectory.split(separator: "/").first.map(String.init)!])
        let phononSub = PocketTtsSmallModel.phonon31m.repoSubdirectory.split(separator: "/").first.map(String.init)!
        XCTAssertEqual(KnownModels.managed.first { $0.id == "pocket-tts" }?.excludedFolders, [pocket + "/" + phononSub], "Pocket spares the folder the library gives Phonon: a rename in the library fails here, not on a phone")
        let kokoro = KnownModels.managed.first { $0.id == "kokoro-82m-ane" }?.cacheFolders ?? []
        XCTAssertTrue(kokoro.contains(Repo.kokoro.folderName)); XCTAssertTrue(kokoro.contains { Repo.kokoroAne.folderName.hasPrefix($0) }, "\(kokoro) vs \(Repo.kokoroAne.folderName)")
    }
}
