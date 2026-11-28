import XCTest
@testable import AutoradioKit

// The place the deck reports (docs/design/sentence-follow-in-shared-code-2026-11-28.md): the digest, each unit's range in the received text, and the one read of the player under a document generation.
// F9 and F20: a document replaced under the playhead, at another index or at the SAME index, gives no place. F18: a position for another block gives the same words and unit with 0 and 0.
final class SentencePlaceTests: XCTestCase {
    static let story = "First sentence here. Second one follows it.\n\nA second paragraph\nwraps over two lines. It ends here.\n\n" + String(repeating: "A long paragraph sentence that goes on and on. ", count: 12)

    func prepared(_ source: String, id: String = "e1") -> PreparedSegment {
        let paras = source.components(separatedBy: "\n\n").map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }.filter { !$0.isEmpty }
        let inc = IncomingSegment(stretchID: "rs", segmentID: id, kind: "story", title: "T", text: paras.joined(separator: "\n\n"), cites: [], blockCount: nil, source: source)
        return SegmentPrep.prepare(inc)!
    }

    func testDigestMatchesTheSharedCodeVectors() {
        XCTAssertEqual(DeckDigest.of(""), "0:811c9dc5")
        XCTAssertEqual(DeckDigest.of("a"), "1:e40c292c")
        XCTAssertEqual(DeckDigest.of("foobar"), "6:bf9cf968")
        XCTAssertEqual(DeckDigest.of("caf\u{E9} \u{1F600}"), "7:f1b38d91", "an emoji is two UTF-16 code units; radio-follow.test.ts pins the same string")
    }

    func testEveryUnitsRangeIsItsTextInTheSource() throws {
        let src = Self.story
        let p = prepared(src)
        XCTAssertGreaterThan(p.blocks.count, 3, "the long paragraph is cut into several units")
        let ranges = try XCTUnwrap(p.segment.unitRanges)
        XCTAssertEqual(ranges.count, p.blocks.count)
        XCTAssertEqual(p.segment.sourceDigest, DeckDigest.of(src))
        var last = 0
        let u = Array(src.utf16)
        for (b, r) in zip(p.blocks, ranges) {
            let slice = String(decoding: u[r.start..<(r.start + r.length)], as: UTF16.self)
            func squash(_ s: String) -> String { s.split(whereSeparator: \.isWhitespace).joined(separator: " ") }
            XCTAssertEqual(squash(slice), squash(b.spokenText ?? ""), "the range holds the unit's words, a line break standing for a space")
            XCTAssertGreaterThanOrEqual(r.start, last, "in order, not overlapping"); last = r.start + r.length
            XCTAssertFalse(slice.hasPrefix(" ") || slice.hasSuffix(" ") || slice.hasSuffix("\n"))
        }
    }

    func testARepeatedSentenceIsFoundAtItsOwnRangeNotTheFirst() throws {
        let src = "Same words here.\n\nOther words entirely.\n\nSame words here."
        let r = try XCTUnwrap(prepared(src).segment.unitRanges)
        XCTAssertEqual(r.count, 3)
        XCTAssertEqual(r[2].start, 16 + 2 + 21 + 2)
        XCTAssertEqual(r[0].start, 0)
    }

    func testWideCharactersCountInUTF16() throws {
        let src = "Caf\u{E9} \u{1F600} first.\n\nSecond one."
        let r = try XCTUnwrap(prepared(src).segment.unitRanges)
        XCTAssertEqual(r[0].length, "Caf\u{E9} \u{1F600} first.".utf16.count)
        XCTAssertEqual(r[1].start, src.utf16.count - "Second one.".utf16.count)
    }

    func testNoSourceOrAUnitThatIsNotInTheSourceGivesNoRanges() {
        let inc = IncomingSegment(stretchID: "rs", segmentID: "e", kind: "story", title: "T", text: "One.", cites: [], blockCount: nil)
        XCTAssertNil(SegmentPrep.prepare(inc)?.segment.unitRanges)
        XCTAssertNil(DeckDigest.ranges(of: ["Absent words."], in: "Other text."))
        XCTAssertNil(DeckDigest.ranges(of: [""], in: "Other text."))
    }

    // MARK: the one read

    @MainActor func stage(_ doc: ProgrammeDocument, gen: Int, tag: Int = 0) async -> (ProgrammePlayer, Task<Bool, Never>) {
        let a = virtualAudio(); a.programmePlay()
        let r = StageRegistry.builtIn()
        let player = ProgrammePlayer(audio: a, registry: r, log: EventLog(clock: LabClock()), configuration: StageRegistry.defaultConfiguration(), document: doc)
        player.setDocument(doc)                                   // generation 1
        let t = Task { await a.playProgramme([Float](repeating: 0, count: 32_000), rate: 16_000, tag: tag, gen: gen) }    // 2 s of audio
        _ = await poll(2) { a.programmePosition(tag: tag) != nil }
        return (player, t)
    }

    @MainActor func testThePlaceCarriesTheEntryTheRangeTheDigestAndTheTiming() async throws {
        let src = Self.story, p = prepared(src)
        var doc = ProgrammeDocument(id: "d", title: "T", blocks: []); doc.append(p)
        let (player, t) = await stage(doc, gen: 1)
        let place = player.spokenPlace()
        XCTAssertEqual(place.entry, "e1")
        XCTAssertEqual(place.digest, DeckDigest.of(src))
        XCTAssertEqual(place.start, p.segment.unitRanges![0].start); XCTAssertEqual(place.length, p.segment.unitRanges![0].length)
        XCTAssertEqual(place.total, 2, accuracy: 0.01)
        XCTAssertGreaterThanOrEqual(place.seconds, 0)
        t.cancel()
    }

    @MainActor func testF9ADocumentReplacedUnderThePlayheadGivesNoPlace() async {
        var doc = ProgrammeDocument(id: "d", title: "T", blocks: []); doc.append(prepared(Self.story))
        let (player, t) = await stage(doc, gen: 1)
        var other = ProgrammeDocument(id: "d2", title: "T", blocks: []); other.append(prepared("Another entry entirely. With words.", id: "e2"))
        player.setDocument(other)                                  // generation 2: the playhead still says 1
        XCTAssertEqual(player.spokenPlace(), DeckPlace(), "old timing is never given with new words")
        t.cancel()
    }

    @MainActor func testF20ADocumentReplacedAtTheSameIndexGivesNoPlaceEither() async {
        var doc = ProgrammeDocument(id: "d", title: "T", blocks: []); doc.append(prepared("Same index old words. More old words.", id: "e1"))
        let (player, t) = await stage(doc, gen: 1, tag: 0)
        var rev = ProgrammeDocument(id: "d", title: "T", blocks: []); rev.append(prepared("Same index new words. More new words.", id: "e1"))
        player.setDocument(rev)                                    // the same document id, the same entry id and block 0
        XCTAssertEqual(player.spokenPlace().entry, "", "a block-index tag alone would have passed this; the generation does not")
        t.cancel()
    }

    @MainActor func testF18APositionForAnotherBlockGivesTheSameWordsWithNoTiming() async {
        var doc = ProgrammeDocument(id: "d", title: "T", blocks: []); let p = prepared(Self.story); doc.append(p)
        let (player, t) = await stage(doc, gen: 1, tag: 3)         // the audio is for block 3; the player is on block 0
        let place = player.spokenPlace()
        XCTAssertEqual(place.entry, "e1"); XCTAssertEqual(place.start, p.segment.unitRanges![0].start)
        XCTAssertEqual(place.total, 0); XCTAssertEqual(place.seconds, 0)
        t.cancel()
    }

    @MainActor func testADocumentWithNoRangesGivesNoPlace() {
        let a = virtualAudio()
        let player = ProgrammePlayer(audio: a, registry: StageRegistry.builtIn(), log: EventLog(clock: LabClock()), configuration: StageRegistry.defaultConfiguration(), document: segmentedDocument(["One. Two."]))
        XCTAssertEqual(player.spokenPlace(), DeckPlace())
    }
}
