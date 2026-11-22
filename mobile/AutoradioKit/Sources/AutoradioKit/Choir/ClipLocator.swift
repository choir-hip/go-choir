import Foundation
import AVFoundation

// Finding a podcast clip again in the file the host serves now (docs/reference/PODCAST-CLIP-FINGERPRINT.md section 2).
// The owner's decision (2026-11-17): fetch one window by byte range, find the clip in it by fingerprint, and play THE BYTES JUST FETCHED AND
// MEASURED. The host is not asked again (one host serves a different file on every request). One path for every host.

/// What a located clip is: a playable file holding exactly the clip.
public struct LocatedClip: Sendable, Equatable {
    public var file: URL
    public var seconds: Double
    /// Where the clip starts in the served file minus where it started in the file the fingerprint was cut from (an estimate for the log).
    public var offsetSeconds: Double
    public var bytes: Int
}

public enum ClipLocation: Sendable, Equatable {
    case located(LocatedClip)
    case refused(String)
}

public protocol ClipLocating: Sendable {
    func locate(_ clip: PodcastClip) async -> ClipLocation
    /// Forget the file made for this clip (the entry ended, or was dropped).
    func discard(_ clip: PodcastClip)
}

/// The container of the fetched bytes: where a frame boundary is, and how to cut a playable file out of aligned bytes.
public protocol ClipContainer: Sendable {
    var fileExtension: String { get }
    /// The bytes from the first frame boundary on (leading bytes of a cut frame dropped); nil when there is no audio in them.
    func align(_ window: Data) -> Data?
    /// A playable file of the seconds [from, to) of aligned bytes (counted from the start of `aligned`).
    func cut(_ aligned: Data, from: Double, to: Double) -> Data?
}

public protocol ClipRangeFetching: Sendable {
    /// One ranged GET (redirects followed by the transport), streamed. Throws `ClipFetchError`.
    func fetch(url: URL, range: ClosedRange<Int>, deadline: TimeInterval) async throws -> Data
}
public enum ClipFetchError: Error, Equatable { case status(Int), deadline, transport(String) }

public protocol ClipDecoding: Sendable {
    /// 16 kHz mono s16 samples of the audio file at `file`.
    func pcm16k(file: URL) throws -> [Int16]
}

// MARK: The real parts

public struct URLSessionClipFetcher: ClipRangeFetching {
    public var session: URLSession
    public init(session: URLSession = .shared) { self.session = session }
    public func fetch(url: URL, range: ClosedRange<Int>, deadline: TimeInterval) async throws -> Data {
        var req = URLRequest(url: url)
        req.setValue("bytes=\(range.lowerBound)-\(range.upperBound)", forHTTPHeaderField: "Range")
        req.timeoutInterval = max(5, min(deadline, 30))
        let session = self.session
        return try await withThrowingTaskGroup(of: Data.self) { group in
            group.addTask {
                do {
                    let (bytes, resp) = try await session.bytes(for: req)
                    let code = (resp as? HTTPURLResponse)?.statusCode ?? 0
                    guard code == 206 else { throw ClipFetchError.status(code) }       // 200: the host ignored the range; the whole file is not fetched
                    var out = Data(); out.reserveCapacity(range.count)
                    for try await b in bytes { out.append(b); if out.count >= range.count { break } }
                    return out
                } catch let e as ClipFetchError { throw e } catch is CancellationError { throw ClipFetchError.deadline } catch { throw ClipFetchError.transport((error as NSError).localizedDescription) }
            }
            group.addTask { try await Task.sleep(nanoseconds: UInt64(max(0, deadline) * 1_000_000_000)); throw ClipFetchError.deadline }
            defer { group.cancelAll() }
            return try await group.next() ?? Data()
        }
    }
}

public struct AVClipDecoder: ClipDecoding {
    public init() {}
    public func pcm16k(file: URL) throws -> [Int16] {
        let f = try AVAudioFile(forReading: file)
        let inFmt = f.processingFormat
        guard let outFmt = AVAudioFormat(commonFormat: .pcmFormatFloat32, sampleRate: 16_000, channels: 1, interleaved: false),
              let conv = AVAudioConverter(from: inFmt, to: outFmt),
              let inBuf = AVAudioPCMBuffer(pcmFormat: inFmt, frameCapacity: 32_768),
              let outBuf = AVAudioPCMBuffer(pcmFormat: outFmt, frameCapacity: 32_000) else { throw NSError(domain: "clip.decode", code: 1, userInfo: [NSLocalizedDescriptionKey: "no converter"]) }
        var out = [Int16](); out.reserveCapacity(Int(Double(f.length) / inFmt.sampleRate * 16_000) + 32_000)
        var eof = false
        while true {
            var err: NSError?
            let status = conv.convert(to: outBuf, error: &err) { _, st in
                if eof { st.pointee = .endOfStream; return nil }
                inBuf.frameLength = 0
                do { try f.read(into: inBuf) } catch { eof = true; st.pointee = .endOfStream; return nil }
                if inBuf.frameLength == 0 { eof = true; st.pointee = .endOfStream; return nil }
                st.pointee = .haveData; return inBuf
            }
            if status == .error { throw err ?? NSError(domain: "clip.decode", code: 2) }
            if let p = outBuf.floatChannelData?[0] {
                for i in 0..<Int(outBuf.frameLength) { out.append(Int16(max(-32768, min(32767, (p[i] * 32768).rounded())))) }
            }
            if status == .endOfStream || outBuf.frameLength == 0 { break }
        }
        return out
    }
}

/// MPEG audio layer III (mp3): frame boundaries by header sync, and a cut on whole frames.
public struct Mp3Container: ClipContainer {
    public init() {}
    public var fileExtension: String { "mp3" }

    struct Header { var length: Int, samples: Int, sampleRate: Int }
    private static let bitrates1: [Int] = [0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0]
    private static let bitrates2: [Int] = [0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0]
    private static let rates: [[Int]] = [[11025, 12000, 8000], [], [22050, 24000, 16000], [44100, 48000, 32000]]    // index by version bits

    /// The frame header at `i`, or nil when the four bytes are not a layer III header.
    static func header(_ b: UnsafeBufferPointer<UInt8>, _ i: Int) -> Header? {
        guard i >= 0, i + 4 <= b.count, b[i] == 0xFF, b[i + 1] & 0xE0 == 0xE0 else { return nil }
        let ver = Int(b[i + 1] >> 3) & 3, layer = Int(b[i + 1] >> 1) & 3
        guard ver != 1, layer == 1 else { return nil }                       // reserved version; layer III only
        let bi = Int(b[i + 2] >> 4), si = Int(b[i + 2] >> 2) & 3, pad = Int(b[i + 2] >> 1) & 1
        guard bi != 0, bi != 15, si != 3 else { return nil }                 // free format and bad index are not frames
        let kbps = (ver == 3 ? bitrates1 : bitrates2)[bi], sr = rates[ver][si]
        let mpeg1 = ver == 3
        let len = (mpeg1 ? 144 : 72) * kbps * 1000 / sr + pad
        return Header(length: len, samples: mpeg1 ? 1152 : 576, sampleRate: sr)
    }

    /// The offset of the first position that starts a run of frames (three in a row, or to the end of the data); nil when there is none.
    public static func firstFrame(in data: Data) -> Int? {
        data.withUnsafeBytes { raw -> Int? in
            let b = raw.bindMemory(to: UInt8.self)
            var i = 0
            while i + 4 <= b.count {
                if b[i] == 0xFF, b[i + 1] & 0xE0 == 0xE0, header(b, i) != nil {
                    var j = i, run = 0
                    while let hh = header(b, j) { run += 1; j += hh.length; if run >= 3 || j >= b.count { break } }
                    if run >= 3 || (run >= 1 && j + 4 > b.count && j <= b.count) { return i }
                }
                i += 1
            }
            return nil
        }
    }

    public func align(_ window: Data) -> Data? {
        guard let i = Self.firstFrame(in: window) else { return nil }
        return window.subdata(in: window.startIndex.advanced(by: i)..<window.endIndex)
    }

    /// Whole frames for [from, to), with two frames before `from` (the bit reservoir: they decode as silence when their own data is missing,
    /// so the clip's first sound is intact).
    public func cut(_ aligned: Data, from: Double, to: Double) -> Data? {
        aligned.withUnsafeBytes { raw -> Data? in
            let b = raw.bindMemory(to: UInt8.self)
            var offsets: [Int] = [], times: [Double] = []
            var i = 0, t = 0.0
            while let h = Self.header(b, i), i + h.length <= b.count { offsets.append(i); times.append(t); i += h.length; t += Double(h.samples) / Double(h.sampleRate) }
            offsets.append(i)
            guard offsets.count > 1 else { return nil }
            let n = offsets.count - 1
            let first = (times.lastIndex(where: { $0 <= from }) ?? 0)
            let lastEx = times.firstIndex(where: { $0 >= to }) ?? n
            let a = max(0, first - 2)
            guard lastEx > first else { return nil }
            return Data(bytes: b.baseAddress! + offsets[a], count: offsets[lastEx] - offsets[a])
        }
    }
}

// MARK: The locator

public final class ClipLocator: ClipLocating, @unchecked Sendable {
    public static let shared = ClipLocator()
    public let fetcher: ClipRangeFetching
    public let decoder: ClipDecoding
    public let container: ClipContainer
    public let directory: URL
    /// Seconds either side of the stored clip that the window covers (the measured offsets were all within 240 s).
    public var margin = 240.0
    public var fetchDeadline: TimeInterval = 90
    public var log: @Sendable (String) -> Void = { print($0) }

    public init(fetcher: ClipRangeFetching = URLSessionClipFetcher(), decoder: ClipDecoding = AVClipDecoder(), container: ClipContainer = Mp3Container(), directory: URL? = nil) {
        self.fetcher = fetcher; self.decoder = decoder; self.container = container
        self.directory = directory ?? AccountWipe.url("clips")
        try? FileManager.default.removeItem(at: self.directory)            // what a crash left behind
        try? FileManager.default.createDirectory(at: self.directory, withIntermediateDirectories: true)
    }

    private func path(_ id: String, _ ext: String) -> URL { directory.appendingPathComponent(id.filter { $0.isLetter || $0.isNumber || $0 == "-" } + "." + ext) }
    public func discard(_ clip: PodcastClip) { try? FileManager.default.removeItem(at: path(clip.clipID, container.fileExtension)) }

    public func locate(_ clip: PodcastClip) async -> ClipLocation {
        guard let rec = clip.fingerprint, clip.isRelocatable,
              let head = ClipFingerprint.unpack(rec.head), let tail = ClipFingerprint.unpack(rec.tail) else { return refuse(clip, "the clip has no usable fingerprint") }
        let t0 = Date()
        func ms(_ since: Date) -> Int { Int(Date().timeIntervalSince(since) * 1000) }
        let from = max(0, rec.start - margin), to = rec.start + margin          // centred on the RECORD's start, never the clip's top-level times
        var hi = rec.byte(at: to)
        if rec.fileBytes > 0 { hi = min(hi, Int(rec.fileBytes) - 1) }
        let lo = rec.byte(at: from)
        guard hi > lo else { return refuse(clip, "the window is empty") }

        var raw: Data
        do { raw = try await fetcher.fetch(url: clip.audioURL, range: lo...hi, deadline: fetchDeadline) }
        catch { return refuse(clip, "fetch failed (\(error))", fetchMs: ms(t0)) }
        let fetchMs = ms(t0)
        guard let aligned = container.align(raw) else { return refuse(clip, "no audio frame in the fetched bytes", bytes: raw.count, fetchMs: fetchMs) }
        raw = Data()

        let t1 = Date()
        let tmp = path("window-" + UUID().uuidString.prefix(8), container.fileExtension)
        let pcm: [Int16]
        do {
            try aligned.write(to: tmp)
            defer { try? FileManager.default.removeItem(at: tmp) }
            pcm = try decoder.pcm16k(file: tmp)
        } catch { return refuse(clip, "decode failed (\((error as NSError).localizedDescription))", bytes: aligned.count, fetchMs: fetchMs, decodeMs: ms(t1)) }
        let decodeMs = ms(t1)

        let t2 = Date()
        let track = ClipFingerprint.track(pcm)
        guard let h = ClipFingerprint.locate(template: head, in: track), let t = ClipFingerprint.locate(template: tail, in: track) else {
            return refuse(clip, "the window is shorter than the fingerprint", bytes: aligned.count, fetchMs: fetchMs, decodeMs: decodeMs, fpMs: ms(t2))
        }
        let fpMs = ms(t2)
        // Spec section 2: off = window_t0 + pos/10 - head_at (tail_at); played from head_at + off_head to tail_at + 12 + off_tail.
        let offHead = h.seconds - rec.headAt, offTail = t.seconds - rec.tailAt
        let est = from + offHead
        func fail(_ why: String) -> ClipLocation { refuse(clip, why, bytes: aligned.count, fetchMs: fetchMs, decodeMs: decodeMs, fpMs: fpMs, h: h, t: t, offset: est) }
        guard h.found else { return fail("the head of the clip is not in the window") }
        guard t.found else { return fail("the tail of the clip is not in the window") }
        guard abs(offHead - offTail) <= ClipFingerprint.agreeSeconds else { return fail("head and tail disagree by \(String(format: "%.1f", abs(offHead - offTail))) s (something is inside the clip)") }
        let s0 = h.seconds, s1 = t.seconds + Double(tail.count / ClipFingerprint.bands) / ClipFingerprint.fps, length = s1 - s0
        guard length > 0, s1 <= Double(pcm.count) / 16_000 + 0.2 else { return fail("the clip runs past the window") }
        guard let bytes = container.cut(aligned, from: s0, to: s1) else { return fail("no whole frames for the clip") }
        let file = path(clip.clipID, container.fileExtension)
        do { try bytes.write(to: file) } catch { return fail("could not keep the clip (\((error as NSError).localizedDescription))") }
        log("AUTORADIO clip locate id=\(clip.clipID) bytes=\(aligned.count) fetch_ms=\(fetchMs) decode_ms=\(decodeMs) fp_ms=\(fpMs) head_peak=\(f(h.peak)) head_margin=\(f(h.margin)) tail_peak=\(f(t.peak)) offset_s=\(f(est)) ok=true why=")
        return .located(LocatedClip(file: file, seconds: length, offsetSeconds: est, bytes: bytes.count))
    }

    private func f(_ v: Double) -> String { String(format: "%.2f", v) }

    private func refuse(_ clip: PodcastClip, _ why: String, bytes: Int = 0, fetchMs: Int = 0, decodeMs: Int = 0, fpMs: Int = 0,
                        h: ClipFingerprint.Match? = nil, t: ClipFingerprint.Match? = nil, offset: Double = 0) -> ClipLocation {
        log("AUTORADIO clip locate id=\(clip.clipID) bytes=\(bytes) fetch_ms=\(fetchMs) decode_ms=\(decodeMs) fp_ms=\(fpMs) head_peak=\(f(h?.peak ?? 0)) head_margin=\(f(h?.margin ?? 0)) tail_peak=\(f(t?.peak ?? 0)) offset_s=\(f(offset)) ok=false why=\(why)")
        return .refused(why)
    }
}
