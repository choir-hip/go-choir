import XCTest
@testable import AutoradioKit

// Finding a podcast clip in the file the host serves now (lane clip-relocate). The failure list (each is a test below):
//  1  the fingerprint is the spec's, byte for byte (the conformance fixture), and its pack locates itself at position 6
//  2  an mp3 frame boundary is found by header sync in bytes that start mid-frame, frame lengths are the spec's, and a cut is whole frames
//  3  a clip in a file with a different lead-in, other gain and added noise is found within 0.5 s, and what is kept is exactly the clip
//  4  a clip that is not in the served file is refused (and says so); a host that ignores the range (200) is refused; nothing is kept
//  5  an insertion inside the clip is refused: head and tail disagree
//  6  an old record with no fingerprint is not a clip: no entry from the feed, the row offers the episode, the player never plays it as a clip
//  7  the next clip is located while the entry before it plays, once, and the clip plays from the located file; skip ends it and the file goes
//  8  the new clip fields decode (additive, optional), a bad object is dropped, and the console line has the fields it promises

// MARK: Synthetic audio (no third-party sound): deterministic noise with a speech-like, band-varying envelope.

struct LCG { var x: UInt32; mutating func next() -> UInt32 { x = x &* 1664525 &+ 1013904223; return x }
    mutating func unit() -> Double { Double(next() >> 8) / Double(1 << 24) * 2 - 1 } }

func synth(seconds: Double, seed: UInt32) -> [Int16] {
    var g = LCG(x: seed)
    let n = Int(seconds * 16_000)
    var out = [Int16](repeating: 0, count: n)
    var i = 0
    while i < n {
        let len = min(n - i, 2_400 + Int(g.next() % 4_000))               // 0.15 to 0.4 s: a "syllable"
        let amp = 2_000 + Double(g.next() % 9_000), a = 0.05 + 0.9 * Double(g.next() % 100) / 100      // loudness and a one-pole colour
        var y = 0.0
        for k in 0..<len { y = a * y + (1 - a) * g.unit(); out[i + k] = Int16(max(-32768, min(32767, y * amp * 3))) }
        i += len
    }
    return out
}

/// PCM as a mono 16 kHz WAV, and the "container" that wraps raw PCM bytes back into one (the test stand-in for mp3 frame alignment).
fileprivate func wav(_ pcm: [Int16]) -> Data {
    var d = Data()
    func u32(_ v: Int) { var x = UInt32(v).littleEndian; d.append(Data(bytes: &x, count: 4)) }
    func u16(_ v: Int) { var x = UInt16(v).littleEndian; d.append(Data(bytes: &x, count: 2)) }
    d.append("RIFF".data(using: .ascii)!); u32(36 + pcm.count * 2); d.append("WAVEfmt ".data(using: .ascii)!)
    u32(16); u16(1); u16(1); u32(16_000); u32(32_000); u16(2); u16(16)
    d.append("data".data(using: .ascii)!); u32(pcm.count * 2)
    for s in pcm { var x = s.littleEndian; d.append(Data(bytes: &x, count: 2)) }
    return d
}
fileprivate struct WavContainer: ClipContainer {
    var fileExtension: String { "wav" }
    func align(_ window: Data) -> Data? { window.count >= 3_200 ? window : nil }
    func cut(_ aligned: Data, from: Double, to: Double) -> Data? {
        let a = Int(from * 16_000) * 2, b = min(Int(to * 16_000) * 2, aligned.count)
        guard b > a else { return nil }
        var pcm = [Int16](); aligned.subdata(in: aligned.startIndex.advanced(by: a)..<aligned.startIndex.advanced(by: b)).withUnsafeBytes { r in pcm = Array(r.bindMemory(to: Int16.self)) }
        return wav(pcm)
    }
    /// Raw PCM of the window is wrapped into a file the decoder can read.
    func wrap(_ raw: Data) -> Data { var pcm = [Int16](); raw.withUnsafeBytes { r in pcm = Array(r.bindMemory(to: Int16.self)) }; return wav(pcm) }
}
/// Wav fetched by byte range: the served file is header + PCM; the fetched bytes are PCM only, and `align` of the real WavContainer wraps them.
fileprivate struct PcmContainer: ClipContainer {
    var fileExtension: String { "wav" }
    func align(_ window: Data) -> Data? { window.count >= 3_200 ? WavContainer().wrap(window) : nil }
    func cut(_ aligned: Data, from: Double, to: Double) -> Data? {
        // `aligned` is a wav: skip its 44-byte header
        WavContainer().cut(aligned.subdata(in: aligned.startIndex.advanced(by: 44)..<aligned.endIndex), from: from, to: to)
    }
}

fileprivate final class ServedFile: ClipRangeFetching, @unchecked Sendable {
    let bytes: Data, status: Int
    private(set) var calls = 0
    private(set) var lastRange: ClosedRange<Int>?
    init(_ pcm: [Int16], status: Int = 206) { bytes = wav(pcm); self.status = status }
    func fetch(url: URL, range: ClosedRange<Int>, deadline: TimeInterval) async throws -> Data {
        calls += 1; lastRange = range
        guard status == 206 else { throw ClipFetchError.status(status) }
        let lo = min(range.lowerBound, bytes.count), hi = min(range.upperBound + 1, bytes.count)
        return bytes.subdata(in: lo..<hi)
    }
}

/// The clip [100, 130] s of `source`, as the worker would record it: fingerprints cut at the 0.1 s grid, 256 kbps = 32000 bytes a second.
func fpRecord(of source: [Int16], start: Double = 100, end: Double = 130) -> ClipFingerprintRecord {
    let track = ClipFingerprint.track(source)
    let f0 = Int((start * 10).rounded()), f1 = Int((end * 10).rounded()), n = min(120, f1 - f0)
    func cut(_ a: Int, _ b: Int) -> [UInt8] { ClipFingerprint.pack(Array(track[(a * 8)..<(b * 8)]))! }
    return ClipFingerprintRecord(version: 1, head: cut(f0, f0 + n), tail: cut(f1 - n, f1), headAt: Double(f0) / 10, tailAt: Double(f1 - n) / 10, start: start, end: end,
                                 fileBytes: Int64(source.count * 2 + 44), fileSeconds: Double(source.count) / 16_000, firstFrameOffset: 44, bitrateKbps: 256, seekableByBitrate: true)
}
fileprivate func clip(_ rec: ClipFingerprintRecord?, id: String = "clip-0000000000c1", topStart: Double? = nil, topEnd: Double? = nil) -> PodcastClip {
    PodcastClip(clipID: id, route: "a", version: 1, storyTitle: "S", show: "Show", episodeTitle: "Ep", episodeGUID: "g", feedURL: nil, published: "", audioURL: URL(string: "https://example.test/ep.mp3")!,
                duration: 200, start: topStart ?? rec?.start ?? 100, end: topEnd ?? rec?.end ?? 130, quote: "q", support: 0.9, fingerprint: rec)
}
fileprivate func tmpDir() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("clip-" + UUID().uuidString) }

/// A clip row's `fingerprint` object for tests that need a relocatable clip (synthetic; 10 to 70 s).
func testFingerprintJSON() -> [String: Any] { testFingerprintCached }
private let testFingerprintCached: [String: Any] = {
    let r = fpRecord(of: synth(seconds: 80, seed: 3), start: 10, end: 70)
    return ["version": 1, "head": Data(r.head).base64EncodedString(), "tail": Data(r.tail).base64EncodedString(), "head_at": r.headAt, "tail_at": r.tailAt, "start": 10.0, "end": 70.0,
            "file_bytes": 100_000, "file_seconds": 80.0, "first_frame_offset": 44, "bitrate_kbps": 256, "seekable_by_bitrate": true]
}()

fileprivate func tmpDir2() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent("clipdec-" + UUID().uuidString + ".mp3") }

final class ClipLocatorTests: XCTestCase {
    var lines: [String] = []
    func locator(_ f: ClipRangeFetching, dir: URL) -> ClipLocator {
        let l = ClipLocator(fetcher: f, decoder: AVClipDecoder(), container: PcmContainer(), directory: dir)
        l.margin = 40
        let box = LineBox(); lineBox = box
        l.log = { box.add($0) }
        return l
    }
    var lineBox = LineBox()
    final class LineBox: @unchecked Sendable { var v: [String] = []; let k = NSLock(); func add(_ s: String) { k.lock(); v.append(s); k.unlock() } }

    // 1
    func test1_TheFingerprintIsTheSpecsByteForByte() throws {
        var url = URL(fileURLWithPath: #filePath)
        for _ in 0..<4 { url.deleteLastPathComponent() }                      // Tests/AutoradioKitTests -> AutoradioKit -> mobile
        let data = try Data(contentsOf: url.appendingPathComponent("fixtures/podcast-clip-fingerprint-v1.json"))
        let j = try XCTUnwrap(try JSONSerialization.jsonObject(with: data) as? [String: Any])
        // The fixture's signal: integer only.
        var x: UInt32 = 1
        let env = [1, 2, 4, 8, 16, 8, 4, 2]
        var pcm = [Int16](); pcm.reserveCapacity(48_000)
        for i in 0..<48_000 {
            x = x &* 1664525 &+ 1013904223
            let noise = Int(x >> 16) - 32768
            let v = (noise * env[(i / 1600) % 8]) / 64 + ((i / 20) % 2 == 0 ? 3000 : -3000)
            pcm.append(Int16(v))
        }
        XCTAssertEqual(pcm.prefix(16).map(Int.init), (j["first_samples"] as! [NSNumber]).map(\.intValue))
        let frames = ClipFingerprint.track(pcm)
        func hex(_ b: [UInt8]) -> String { b.map { String(format: "%02x", $0) }.joined() }
        XCTAssertEqual(hex(frames), j["frames_hex"] as? String)
        let loc = try XCTUnwrap(j["locate"] as? [String: Any])
        let from = loc["from"] as! Int, to = loc["to"] as! Int
        let pack = try XCTUnwrap(ClipFingerprint.pack(Array(frames[(from * 8)..<(to * 8)])))
        XCTAssertEqual(hex(pack), loc["pack_hex"] as? String)
        let m = try XCTUnwrap(ClipFingerprint.locate(template: try XCTUnwrap(ClipFingerprint.unpack(pack)), in: frames))
        XCTAssertEqual(m.pos, loc["pos"] as? Int)
        XCTAssertGreaterThanOrEqual(m.peak, loc["min_peak"] as! Double)
    }

    // 2
    func test2_Mp3FramesAreFoundByHeaderSyncAndCutWhole() throws {
        // MPEG-1 layer III, 128 kbps, 44.1 kHz: header FF FB 90 00, 417 bytes (418 with the padding bit, 92). Frames of zeros after the header are silence.
        func frame(_ pad: Int = 0) -> Data { var d = Data([0xFF, 0xFB, pad == 1 ? 0x92 : 0x90, 0x00]); d.append(Data(count: 417 + pad - 4)); return d }
        var file = Data(); for k in 0..<10 { file.append(frame(k % 2)) }          // 5 x 417 + 5 x 418
        XCTAssertEqual(file.count, 4_175)
        let junk = Data([0x12, 0xFF, 0x00, 0xFF, 0xFB, 0x00, 0x77])               // a cut frame's tail, with false syncs in it
        let window = junk + file
        let mp3 = Mp3Container()
        XCTAssertEqual(Mp3Container.firstFrame(in: window), junk.count, "false syncs (a bad bitrate, a bad run) are not frames")
        XCTAssertEqual(mp3.align(window), file)
        let ft = 1152.0 / 44_100
        let cut = try XCTUnwrap(mp3.cut(file, from: 4 * ft + 0.001, to: 8 * ft))
        // frames 2 (two before frame 4) to 7: whole frames, starting on a header, six of them
        var sum = 0; for k in 2..<8 { sum += 417 + k % 2 }
        XCTAssertEqual(cut.count, sum)
        XCTAssertEqual(Array(cut.prefix(2)), [0xFF, 0xFB])
        XCTAssertNil(mp3.align(Data(count: 5_000)), "no frames in silence bytes")
        XCTAssertNil(Mp3Container.firstFrame(in: Data([0xFF, 0xE0, 0x90, 0x00] + [UInt8](repeating: 0, count: 600))), "layer bits 00 is reserved")
    }

    func test2_ASliceOfMp3FramesDecodesToPcmOfTheRightLength() throws {
        var file = Data(); var carry = 0.0
        for _ in 0..<200 { carry += 417.96; let n = Int(carry); carry -= Double(n); var d = Data([0xFF, 0xFB, n == 418 ? 0x92 : 0x90, 0x00]); d.append(Data(count: n - 4)); file.append(d) }
        let slice = try XCTUnwrap(Mp3Container().align(Data([0x55, 0x66]) + file.dropFirst(100)))      // starts mid-frame
        let f = tmpDir2(); defer { try? FileManager.default.removeItem(at: f) }
        try slice.write(to: f)
        let pcm = try AVClipDecoder().pcm16k(file: f)
        XCTAssertEqual(Double(pcm.count) / 16_000, 199 * 1152.0 / 44_100, accuracy: 0.1, "decoded length of the frames kept")
    }

    // 3
    func test3_TheClipIsFoundInAFileWithAnotherLeadInGainAndNoise() async throws {
        let source = synth(seconds: 200, seed: 7)
        let rec = fpRecord(of: source)
        var g = LCG(x: 99)
        let lead = synth(seconds: 7.3, seed: 1234)
        let served = (lead + source).map { Int16(max(-32768, min(32767, Double($0) * 0.7 + g.unit() * 120))) }
        let dir = tmpDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let f = ServedFile(served)
        let l = locator(f, dir: dir)
        let r = await l.locate(clip(rec))
        guard case .located(let got) = r else { return XCTFail("\(r)") }
        XCTAssertEqual(got.offsetSeconds, 7.3, accuracy: 0.5)
        XCTAssertEqual(f.calls, 1, "one ranged request")
        XCTAssertEqual(got.seconds, 30)
        // what was kept is the clip: its head fingerprint is at the start of the kept file
        let kept = try AVClipDecoder().pcm16k(file: got.file)
        XCTAssertEqual(Double(kept.count) / 16_000, 30, accuracy: 0.3)
        let m = try XCTUnwrap(ClipFingerprint.locate(template: try XCTUnwrap(ClipFingerprint.unpack(rec.head)), in: ClipFingerprint.track(kept)))
        XCTAssertLessThanOrEqual(m.pos, 3); XCTAssertGreaterThan(m.peak, 0.9)
        let line = try XCTUnwrap(lineBox.v.last)
        for k in ["id=", "bytes=", "fetch_ms=", "decode_ms=", "fp_ms=", "head_peak=", "head_margin=", "tail_peak=", "offset_s=", "ok=true", "why="] { XCTAssertTrue(line.contains(k), "\(k) in \(line)") }
        XCTAssertTrue(line.hasPrefix("AUTORADIO clip locate "))
        l.discard(clip(rec)); XCTAssertFalse(FileManager.default.fileExists(atPath: got.file.path))
    }

    // 4
    func test4_AbsentClipIsRefusedAndAHostThatIgnoresTheRangeToo() async throws {
        let source = synth(seconds: 200, seed: 7), rec = fpRecord(of: source)
        let other = synth(seconds: 200, seed: 31337)                                    // another recording altogether
        let dir = tmpDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let r = await locator(ServedFile(other), dir: dir).locate(clip(rec))
        guard case .refused(let why) = r else { return XCTFail("a clip that is not there was found") }
        XCTAssertTrue(why.contains("not in the window"), why)
        XCTAssertTrue(lineBox.v.last?.contains("ok=false why=") ?? false)
        let r200 = await locator(ServedFile(source, status: 200), dir: dir).locate(clip(rec))
        guard case .refused(let why2) = r200 else { return XCTFail() }
        XCTAssertTrue(why2.contains("fetch failed"), why2)
        let kept = (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? []
        XCTAssertTrue(kept.isEmpty, "nothing is kept for a refusal: \(kept)")
    }

    // 5
    func test5_AnInsertionInsideTheClipIsRefused() async throws {
        let source = synth(seconds: 200, seed: 7), rec = fpRecord(of: source)
        let ad = synth(seconds: 8, seed: 555)
        let cut = 115 * 16_000                                                          // an ad in the middle of the 100 to 130 s clip
        let served = Array(source[..<cut]) + ad + Array(source[cut...])
        let dir = tmpDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let r = await locator(ServedFile(served), dir: dir).locate(clip(rec))
        guard case .refused(let why) = r else { return XCTFail("a clip with an ad inside it was played") }
        XCTAssertTrue(why.contains("disagree"), why)
    }

    // 3b: a repaired clip. The record was cut in a fresh download at 100 to 130 s; the top-level times are the old file's, 200 s earlier.
    func test3b_ARepairedClipIsPlayedFromTheRecordNotFromTheTopLevelTimes() async throws {
        let source = synth(seconds: 400, seed: 11)
        let rec = fpRecord(of: source, start: 300, end: 330)
        let served = synth(seconds: 1.5, seed: 4) + source
        let dir = tmpDir(); defer { try? FileManager.default.removeItem(at: dir) }
        let c = clip(rec, topStart: 100, topEnd: 130)
        let l = locator(ServedFile(served), dir: dir); l.margin = 60
        let r = await l.locate(c)
        guard case .located(let got) = r else { return XCTFail("\(r)") }
        XCTAssertEqual(got.offsetSeconds, 1.5, accuracy: 0.5)
        XCTAssertEqual(got.seconds, 30, accuracy: 0.3)
        let kept = try AVClipDecoder().pcm16k(file: got.file)
        let m = try XCTUnwrap(ClipFingerprint.locate(template: try XCTUnwrap(ClipFingerprint.unpack(rec.head)), in: ClipFingerprint.track(kept)))
        XCTAssertLessThanOrEqual(m.pos, 3); XCTAssertGreaterThan(m.peak, 0.9, "what was kept is the record's clip, not the file at 100 to 130 s")
        // a window centred on the top-level start (100) would hold nothing of it: the fetch asked near the record's start
        let f = ServedFile(served); _ = await locator(f, dir: dir).locate(c)
        XCTAssertEqual(f.lastRange?.lowerBound, 44 + (300 - 40) * 32_000)
    }

    func test3c_ARecordWithAShortHeadIsNotRelocatable() {
        var r = fpRecord(of: synth(seconds: 40, seed: 3), start: 5, end: 15)         // 10 s: 100 frames
        XCTAssertFalse(r.isUsable)
        r = fpRecord(of: synth(seconds: 40, seed: 3), start: 5, end: 35)
        XCTAssertTrue(r.isUsable)
    }

    // 8
    func test8_TheNewFieldsDecodeAdditivelyAndABadObjectIsDropped() throws {
        let rec = fpRecord(of: synth(seconds: 60, seed: 3), start: 5, end: 35)
        var row: [String: Any] = ["clip_id": "clip-0000000000a1", "story": ["route": "/a", "version": 1, "title": "S"], "show": "Show", "audio_url": "https://example.test/x.mp3",
                                  "duration_seconds": 900, "start": 5, "end": 35, "quote": "q", "support": 0.9]
        XCTAssertNil(PodcastClip.parse(row)?.fingerprint); XCTAssertEqual(PodcastClip.parse(row)?.isRelocatable, false, "an old record")
        row["fingerprint"] = ["version": 1, "head": Data(rec.head).base64EncodedString(), "tail": Data(rec.tail).base64EncodedString(), "head_at": rec.headAt, "tail_at": rec.tailAt, "start": 5.0, "end": 35.0,
                              "file_bytes": 1000, "file_seconds": 20.0, "first_frame_offset": 44, "bitrate_kbps": 256, "seekable_by_bitrate": true] as [String: Any]
        let c = try XCTUnwrap(PodcastClip.parse(row))
        XCTAssertEqual(c.fingerprint?.bitrateKbps, 256); XCTAssertEqual(c.fingerprint?.head, rec.head); XCTAssertTrue(c.isRelocatable)
        XCTAssertEqual(c.fingerprint?.byte(at: 2), 44 + 64_000)
        var notSeekable = row; var fp = row["fingerprint"] as! [String: Any]; fp["seekable_by_bitrate"] = false; notSeekable["fingerprint"] = fp
        XCTAssertFalse(PodcastClip.parse(notSeekable)!.isRelocatable, "a file that cannot be read by bitrate is not found by the phone")
        var bad = row; bad["fingerprint"] = ["version": 1, "head": "!!", "tail": "x"] as [String: Any]
        XCTAssertNotNil(PodcastClip.parse(bad)); XCTAssertNil(PodcastClip.parse(bad)?.fingerprint)
    }
}

// MARK: The player

fileprivate final class FakeLocator: ClipLocating, @unchecked Sendable {
    let k = NSLock(); private(set) var calls: [String] = []; var result: ClipLocation
    var gate: Task<Void, Never>?
    init(_ r: ClipLocation) { result = r }
    func locate(_ clip: PodcastClip) async -> ClipLocation { k.lock(); calls.append(clip.clipID); let g = gate; k.unlock(); await g?.value; return result }
    func discard(_ clip: PodcastClip) {}
}

@MainActor
final class ClipPlayerTests: XCTestCase {
    func relocatable(_ id: String) -> PodcastClip { clip(fpRecord(of: synth(seconds: 60, seed: 3), start: 5, end: 35), id: id) }

    func keptFile() -> URL { let u = FileManager.default.temporaryDirectory.appendingPathComponent("kept-\(UUID().uuidString).mp3"); try? Data([1, 2, 3]).write(to: u); return u }
    func story(_ id: String) -> RunSheetEntry { RunSheetEntry(id: id, kind: .story, placedBy: .desk, title: "T", text: "Words.", estSeconds: 10) }

    // 6


    // 7





}

// MARK: Live check (never in the normal suite)

/// A real host, the real URLSession, mp3 alignment, decode, locate and cut. Skipped unless AR_CLIP_LIVE names a JSON file holding one clip
/// record (the platform's podcast-clips row, with `audio_url` and `fingerprint`):
///   AR_CLIP_LIVE=/path/clip.json swift test --package-path AutoradioKit --filter ClipLiveTests
final class ClipLiveTests: XCTestCase {
    func testLive_ARealHostRealFetchAndDecode() async throws {
        guard let path = ProcessInfo.processInfo.environment["AR_CLIP_LIVE"], !path.isEmpty else { throw XCTSkip("AR_CLIP_LIVE is not set") }
        let data = try Data(contentsOf: URL(fileURLWithPath: path))
        var o = try XCTUnwrap(try JSONSerialization.jsonObject(with: data) as? [String: Any])
        if let rows = o["clips"] as? [[String: Any]], let first = rows.first { o = first }
        let c = try XCTUnwrap(PodcastClip.parse(o), "not a clip record")
        XCTAssertTrue(c.isRelocatable, "the record has no usable fingerprint")
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("clip-live-\(UUID().uuidString)")
        let r = await ClipLocator(directory: dir).locate(c)        // prints the AUTORADIO clip locate line
        switch r {
        case .located(let l): print("AUTORADIO clip live file=\(l.file.path) seconds=\(l.seconds) bytes=\(l.bytes)")
        case .refused(let why): XCTFail("refused: \(why)")
        }
    }
}
