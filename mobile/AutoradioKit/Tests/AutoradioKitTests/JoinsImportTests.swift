import XCTest
@testable import AutoradioKit

// Lane autoradio-joins J2 (see the failure list in JoinsTests.swift).

// MARK: J2 a typed .pdf or .epub

final class JoinBarImportTests: XCTestCase {
    struct Boom: Error {}
    let url = URL(string: "https://example.test/files/My%20Book.epub")!

    func testJ2_ASuccessfulImportOpensTheDocumentPage() async {
        var opened: [String] = [], said: [String] = []
        await BarDocumentImport.run(url: url, importer: { _ in "abc123" }, open: { opened.append($0) }, say: { said.append($0) })
        XCTAssertEqual(opened, ["doc:abc123"]); XCTAssertTrue(said.isEmpty)
    }
    func testJ2_AFailureSaysWhyAndOpensNothing() async {
        for (err, want) in [(ShelfError.download, "The file could not be downloaded."), (ShelfError.tooLarge, "This file is too large to add (the limit is 100 MB).")] as [(Error, String)] {
            var opened: [String] = [], said: [String] = []
            await BarDocumentImport.run(url: url, importer: { _ in throw err }, open: { opened.append($0) }, say: { said.append($0) })
            XCTAssertTrue(opened.isEmpty)
            XCTAssertEqual(said, ["My Book.epub was not added. " + want])
        }
    }
    func testJ2_AnUnknownErrorStillSaysSomethingPlain() async {
        var said: [String] = []
        await BarDocumentImport.run(url: url, importer: { _ in throw Boom() }, open: { _ in }, say: { said.append($0) })
        XCTAssertEqual(said, ["My Book.epub was not added. This file could not be read."])
    }
    func testJ2_TheBarHandsOverTheSaidLineAsItIs() {
        let tr = BarTracker(); let t0 = Date(timeIntervalSince1970: 1000)
        tr.say("My Book.epub was not added. The file could not be downloaded.", now: t0)
        let r = tr.raw()   // bar.ts shows a line with no stage as it is, for its window (bar.test.ts B4)
        XCTAssertEqual(r.errorStage, ""); XCTAssertEqual(r.errorMessage, "My Book.epub was not added. The file could not be downloaded."); XCTAssertEqual(r.errorAt, 1000)
    }
}
