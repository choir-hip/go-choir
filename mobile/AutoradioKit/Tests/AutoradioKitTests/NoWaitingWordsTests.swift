import XCTest

// Owner, 2026-11-17: no page says "Loading" or "Opening". The repo check is mobile/scripts/check-no-loading.py; this runs it
// with the tests so adding such a string again fails the suite (and proof-app.sh runs it too).
final class NoWaitingWordsTests: XCTestCase {
    func testNoUserVisibleLoadingOrOpeningString() throws {
        let script = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("scripts/check-no-loading.py")
        try XCTSkipUnless(FileManager.default.fileExists(atPath: script.path), "the script is not beside the tests")
        let p = Process(); p.executableURL = URL(fileURLWithPath: "/usr/bin/python3"); p.arguments = ["-I", script.path]
        let pipe = Pipe(); p.standardOutput = pipe; p.standardError = pipe
        try p.run(); p.waitUntilExit()
        let out = String(data: pipe.fileHandleForReading.readDataToEndOfFile(), encoding: .utf8) ?? ""
        XCTAssertEqual(p.terminationStatus, 0, out)
    }
}
