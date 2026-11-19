import XCTest
@testable import EmbedPass

final class EmbedPassTests: XCTestCase {
    func row(_ id: String, _ text: String = "body", ordinal: Int = 0, title: String = "Headline", sha: String? = nil, labels: [String]? = nil, priv: Bool? = nil) -> PassRow {
        PassRow(kind: "capture", id: id, ordinal: ordinal, title: title, text: text, textSha: sha ?? "sha-" + text, isPrivate: priv, labels: labels)
    }
    /// A fake model: a 768 vector with a known shape, counting calls and prompts.
    final class Fake { var prompts: [String] = []
        func embed(_ p: String) throws -> (vec: [Float], tokens: Int) {
            prompts.append(p)
            return ((0..<768).map { Float($0 % 7) + 1 + Float(p.count % 5) }, 10)
        }
    }

    func testPromptStringsAreTheCardsTitledForm() {
        XCTAssertEqual(PassPrompt.document(title: "Senate votes", text: "The bill passed."), "title: Senate votes | text: The bill passed.")
        XCTAssertEqual(PassPrompt.document(title: "  ", text: "x"), "title: none | text: x")   // the card's untitled form
    }

    // Numbers worked by hand: the first 256 of a vector of 768, re-normalised, norm 1 to 6 places; the 768 is not unit length.
    func testTruncationAndNormalisation() {
        var v = [Float](repeating: 0, count: 768)
        v[0] = 3; v[1] = 4; v[300] = 100           // outside the first 256: must not count
        let t = PassVector.truncate(v)
        XCTAssertEqual(t.count, 256)
        XCTAssertEqual(t[0], 0.6, accuracy: 1e-6); XCTAssertEqual(t[1], 0.8, accuracy: 1e-6)
        XCTAssertEqual(PassVector.norm(t), 1, accuracy: 1e-6)
        let raw = (0..<768).map { Float(sin(Double($0))) * 400 }   // raw norms in the hundreds, as the GGUF's
        let n = PassVector.norm(raw)
        let u = PassVector.truncate(raw.map { $0 / n })
        XCTAssertEqual(PassVector.norm(u), 1, accuracy: 1e-6)
    }

    func testFloat16RoundTrip() {
        let v = PassVector.truncate((0..<768).map { Float(cos(Double($0))) })
        let back = PassVector.fromF16Bytes(PassVector.f16Bytes(v))
        XCTAssertEqual(back.count, 256)
        for (a, b) in zip(v, back) { XCTAssertEqual(a, b, accuracy: 1e-3) }
        XCTAssertEqual(PassVector.f16Bytes(v).count, 512)
        XCTAssertEqual(PassVector.norm(back), 1, accuracy: 2e-3)   // half precision keeps unit length to about 1e-3
    }

    func testFileFormatRoundTrip() throws {
        let r = PassVectorRow(kind: "story_passage", id: "obj:v/1", ordinal: 3, textSha: "abc", model: "m1", dim: 256,
                              v: PassVector.f16Bytes(PassVector.truncate((0..<768).map { Float($0) + 1 })))
        let line = try PassFile.encode(r)
        XCTAssertFalse(line.contains("\n"))
        XCTAssertEqual(try PassFile.decode(line), r)
        XCTAssertTrue(line.contains("\"text_sha\":\"abc\""))
        XCTAssertFalse(line.contains("\"text\""), "a vector row never carries text")
    }

    func testResumeSkipsWhatIsWrittenAndToleratesATornLine() throws {
        var out: [String] = []
        let f = Fake()
        let a = try PassRunner(model: "m", existing: [], embed: f.embed, sink: { out.append($0) })
        try a.process(row("one", "alpha")); try a.process(row("two", "beta"))
        XCTAssertEqual(f.prompts.count, 2)
        // a killed run leaves half a line at the end
        let existing = out + [String(out[0].prefix(30))]
        var out2: [String] = []
        let g = Fake()
        let b = try PassRunner(model: "m", existing: existing, embed: g.embed, sink: { out2.append($0) })
        try b.process(row("one", "alpha")); try b.process(row("two", "beta")); try b.process(row("three", "gamma"))
        XCTAssertEqual(g.prompts, ["title: Headline | text: gamma"])
        XCTAssertEqual(out2.count, 1)
        XCTAssertEqual(b.progress.skippedHeld, 2)
    }

    func testEqualTextIsEmbeddedOnce() throws {
        var out: [String] = []
        let f = Fake()
        let r = try PassRunner(model: "m", existing: [], embed: f.embed, sink: { out.append($0) })
        try r.process(row("v1", "same", ordinal: 0)); try r.process(row("v2", "same", ordinal: 0))
        XCTAssertEqual(f.prompts.count, 1); XCTAssertEqual(r.progress.reused, 1); XCTAssertEqual(out.count, 2)
        XCTAssertEqual(try PassFile.decode(out[0]).v, try PassFile.decode(out[1]).v)
    }

    /// The model reads the title with the text, so equal text under another title is another input: found 2026-11-19 when corpusd
    /// refused the first file (a sample agreed only to cosine 0.958): 2,661 of 278,048 rows had been given the vector of a row
    /// with the same passage and a different headline.
    func testEqualTextUnderAnotherTitleIsEmbeddedAgain() throws {
        var out: [String] = []
        let f = Fake()
        let r = try PassRunner(model: "m", existing: [], embed: f.embed, sink: { out.append($0) })
        try r.process(row("v1", "same", title: "First headline")); try r.process(row("v2", "same", title: "Another headline"))
        try r.process(row("v3", "same", title: " First headline "))
        XCTAssertEqual(f.prompts, ["title: First headline | text: same", "title: Another headline | text: same"])
        XCTAssertEqual(r.progress.reused, 1); XCTAssertEqual(out.count, 3)
        XCTAssertNotEqual(try PassFile.decode(out[0]).v, try PassFile.decode(out[1]).v)
        XCTAssertEqual(try PassFile.decode(out[0]).v, try PassFile.decode(out[2]).v)
    }

    /// A resumed run does not know the titles of what it wrote before, so it reuses nothing from the file: it only skips what is held.
    func testAResumedRunReusesNothingFromTheFile() throws {
        var out: [String] = []
        let f = Fake()
        let a = try PassRunner(model: "m", existing: [], embed: f.embed, sink: { out.append($0) })
        try a.process(row("v1", "same", title: "First headline"))
        var out2: [String] = []
        let g = Fake()
        let b = try PassRunner(model: "m", existing: out, embed: g.embed, sink: { out2.append($0) })
        try b.process(row("v1", "same", title: "First headline")); try b.process(row("v2", "same", title: "Another headline"))
        XCTAssertEqual(g.prompts, ["title: Another headline | text: same"]); XCTAssertEqual(b.progress.reused, 0); XCTAssertEqual(b.progress.skippedHeld, 1)
    }

    func testARowMarkedPrivateIsNeverEmbeddedOrWritten() throws {
        var out: [String] = []
        let f = Fake()
        let r = try PassRunner(model: "m", existing: [], embed: f.embed, sink: { out.append($0) })
        for x in [row("p1", priv: true), row("p2", labels: ["owner_private:mail"]), row("p3", labels: ["world_untrusted", " Owner_Private:capture"])] {
            XCTAssertFalse(try r.process(x))
        }
        XCTAssertTrue(try r.process(row("ok", labels: ["world_untrusted"])))
        XCTAssertEqual(f.prompts.count, 1); XCTAssertEqual(out.count, 1); XCTAssertEqual(r.progress.refusedPrivate, 3)
        // even when its text equals a public row's already held vector
        XCTAssertFalse(try r.process(row("p4", "body", priv: true)))
        XCTAssertEqual(out.count, 1)
    }

    func testAnotherModelsFileIsRefused() throws {
        let line = try PassFile.encode(PassVectorRow(kind: "capture", id: "x", ordinal: 0, textSha: "s", model: "old", dim: 256, v: Data(count: 512)))
        XCTAssertThrowsError(try PassRunner(model: "new", existing: [line], embed: Fake().embed, sink: { _ in }))
    }

    func testExportLineDecodes() throws {
        let j = #"{"kind":"capture","id":"obj:c","ordinal":0,"title":"T","text":"x","text_sha":"s","published_at":"","passages":3}"#
        let r = try JSONDecoder().decode(PassRow.self, from: Data(j.utf8))
        XCTAssertEqual(r.key, "capture\u{1f}obj:c\u{1f}0"); XCTAssertFalse(r.markedPrivate)
    }
}
