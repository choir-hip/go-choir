import XCTest
@testable import AutoradioKit

// The unsent edit the build before shared-code editing kept (Application Support/Autoradio/texture-pending.json) is read once by the app module for texture-edit.ts and removed only when it asks.
// Failure list (written before the code):
//  1  a file holding the old JSON `{"<doc>": "<markdown>"}` is read as the exact text, unchanged
//  2  no file is nil, not an empty string (shared code reads "found: false")
//  3  remove deletes the file, and removing again is not an error; reading does not remove it (a crash between the two steps loses nothing)
//  4  a file that is not UTF-8 reads as empty text, which shared code finds damaged and removes
final class LegacyTexturePendingTests: XCTestCase {
    private var dir: URL!
    override func setUp() { dir = FileManager.default.temporaryDirectory.appendingPathComponent("legacy-tex-\(UUID().uuidString)"); try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true) }
    override func tearDown() { try? FileManager.default.removeItem(at: dir) }

    func testReadsTheOldJsonUnchangedAndRemovalIsSeparate() throws {
        let f = dir.appendingPathComponent("texture-pending.json")
        let old = "{\"d1\":\"Kept by the old build\\n\\nSecond\",\"d2\":\"other\"}"
        try old.write(to: f, atomically: true, encoding: .utf8)
        XCTAssertEqual(LegacyTexturePending.read(f), old)
        XCTAssertTrue(FileManager.default.fileExists(atPath: f.path), "reading does not remove it")
        LegacyTexturePending.remove(f)
        XCTAssertFalse(FileManager.default.fileExists(atPath: f.path))
        LegacyTexturePending.remove(f)      // again: no error
        XCTAssertNil(LegacyTexturePending.read(f))
    }
    func testNoFileIsNilAndANonUTF8FileIsEmpty() throws {
        XCTAssertNil(LegacyTexturePending.read(dir.appendingPathComponent("none.json")))
        let f = dir.appendingPathComponent("bad.json")
        try Data([0xff, 0xfe, 0xfd]).write(to: f)
        XCTAssertEqual(LegacyTexturePending.read(f), "")
    }
}
