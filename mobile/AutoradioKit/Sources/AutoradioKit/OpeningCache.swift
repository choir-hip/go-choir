import Foundation

// The written-ahead opening: text written by the desk and spoken by the phone's voice BEFORE the launch that plays it, kept in Caches.
//
// What is kept: the opening (text, the titles it names, the time it was written, the voice, and the audio of every block) and the first entry
// after it (the entry itself and the audio of its first block only; the rest is made while that block plays, as for any entry). At launch
// `take` decides what plays:
//   - an opening written within `bound` (2 hours) and in the same part of the same day, in the voice now chosen: it plays as the opening entry
//     with its audio, then the first entry, which is placed after it;
//   - otherwise an entry written within `entryBound` (36 hours): it plays at once and the live opening, when it arrives, is a transition.
//     The rule for stale kept audio: an OPENING names the time of day and the news of that moment, so it is stale after `bound`; a STORY is
//     not tied to the hour, so a kept first entry is still a story the morning after the evening it was written (the owner listens in the
//     morning to what was kept the evening before). 36 hours reaches the next day's start and no further;
//   - otherwise nothing: today's path (the phone's own opening, the desk's live one).
// Audio reaches the sound path through the programme player's `preparedSpeech` hook, which hands the player the samples of a block whose text
// it already holds, instead of making them. Everything after that is the ordinary path: the same entry, ops, player and output node.
// A taken record is deleted at once; its audio is held in memory for the blocks still to play.

public struct PreparedAudio: Sendable, Equatable {
    public var text: String
    public var samples: [Float]
    public var rate: Double
    public init(text: String, samples: [Float], rate: Double) { self.text = text; self.samples = samples; self.rate = rate }
}

public struct CachedEntry: Codable, Sendable, Equatable {
    public struct File: Codable, Sendable, Equatable { public var text: String; public var file: String; public var rate: Double }
    public var entry: RunSheetEntry
    /// The story titles the entry mentions (what a listener would see as its subjects).
    public var titles: [String]
    public var writtenAt: Date
    public var files: [File]
}

public enum OpeningPlan: Sendable, Equatable {
    case none
    /// The cached opening, and the entry that follows it when one is fresh enough.
    case opening(RunSheetEntry, next: RunSheetEntry?)
    /// No fresh opening, but an entry that was ready: it starts at once.
    case entry(RunSheetEntry)
}

public final class OpeningCache: @unchecked Sendable {
    struct Record: Codable { var voice: String; var opening: CachedEntry?; var first: CachedEntry? }

    public let dir: URL
    /// An opening names the time of day and the news: older than this, it is not played as new.
    public var bound: TimeInterval
    /// A story is not tied to the hour: the first entry may be this old.
    public var entryBound: TimeInterval
    /// The most audio kept on disk (Int16 samples). A slot that would pass it is kept as text only.
    public var maxBytes: Int
    public var clock: () -> Date
    public var calendar: Calendar
    /// One line per decision (ids, counts, seconds; never titles or words).
    public var log: (String) -> Void = { print($0) }
    private let lock = NSLock()
    private var held: [String: PreparedAudio] = [:]

    /// The phone's copy of the server's `calibration.AutoradioLapse` (internal/calibration/autoradio.go:35, 12 h): keep the two together.
    static let forecastLapse: TimeInterval = 12 * 3600
    public static let defaultBound: TimeInterval = 2 * 3600
    public static let defaultEntryBound: TimeInterval = 36 * 3600
    public static let defaultMaxBytes = 6_000_000

    public init(dir: URL, bound: TimeInterval = OpeningCache.defaultBound, entryBound: TimeInterval = OpeningCache.defaultEntryBound,
                maxBytes: Int = OpeningCache.defaultMaxBytes, clock: @escaping () -> Date = { Date() }, calendar: Calendar = .current) {
        self.dir = dir; self.bound = bound; self.entryBound = entryBound; self.maxBytes = maxBytes; self.clock = clock; self.calendar = calendar
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    }

    public static func defaultDirectory() -> URL? {
        guard let c = try? FileManager.default.url(for: .cachesDirectory, in: .userDomainMask, appropriateFor: nil, create: true) else { return nil }
        return c.appendingPathComponent("AutoradioOpening", isDirectory: true)
    }

    // MARK: Part of the day

    /// Morning, afternoon, evening (the greeting's own cut-offs) and the day: two times are alike when both agree.
    func dayPart(_ d: Date) -> String {
        let c = calendar.dateComponents([.year, .month, .day, .hour], from: d)
        let h = c.hour ?? 0
        return "\(c.year ?? 0)-\(c.month ?? 0)-\(c.day ?? 0)/" + (h < 5 ? "n" : h < 12 ? "m" : h < 18 ? "a" : "e")
    }

    // MARK: Reading what is kept

    private var manifest: URL { dir.appendingPathComponent("record.json") }
    func load() -> Record? {
        guard let d = try? Data(contentsOf: manifest) else { return nil }
        let dec = JSONDecoder(); dec.dateDecodingStrategy = .secondsSince1970
        return try? dec.decode(Record.self, from: d)
    }
    private func save(_ r: Record?) {
        guard let r else { try? FileManager.default.removeItem(at: manifest); prune(keeping: []); return }
        let enc = JSONEncoder(); enc.dateEncodingStrategy = .secondsSince1970
        if let d = try? enc.encode(r) { try? d.write(to: manifest, options: .atomic) }
        prune(keeping: Set((r.opening?.files ?? []).map(\.file) + (r.first?.files ?? []).map(\.file)))
    }
    /// Files that no record names are deleted.
    private func prune(keeping: Set<String>) {
        let names = (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? []
        for n in names where n.hasSuffix(".pcm") && !keeping.contains(n) { try? FileManager.default.removeItem(at: dir.appendingPathComponent(n)) }
    }
    /// Bytes of audio on disk.
    public var bytesOnDisk: Int {
        let names = (try? FileManager.default.contentsOfDirectory(atPath: dir.path)) ?? []
        return names.filter { $0.hasSuffix(".pcm") }.reduce(0) { $0 + ((try? FileManager.default.attributesOfItem(atPath: dir.appendingPathComponent($1).path)[.size] as? Int) ?? 0) }
    }

    /// What is kept, for the write-ahead's own decisions (when to ask again).
    public func opening(voice: String) -> CachedEntry? {
        guard let r = load(), r.voice == voice else { return nil }
        return r.opening
    }
    public func first(voice: String) -> CachedEntry? {
        guard let r = load(), r.voice == voice else { return nil }
        return r.first
    }
    /// True when an opening that `take` would play is ready (the launch does not wait for the network).
    public func isReady(voice: String) -> Bool { if case .none = peek(voice: voice) { return false }; return true }

    /// Sign-out: the record, every audio file and what is held in memory.
    public func wipe() {
        lock.lock(); held = [:]; lock.unlock()
        try? FileManager.default.removeItem(at: manifest)
        prune(keeping: [])
    }

    // MARK: Writing

    public enum Slot: Sendable { case opening, first }

    /// Keep `entry` and its audio in `slot`, replacing what the slot held. A changed voice drops the other slot too (its audio is another voice's).
    /// Over `maxBytes` the audio is not kept (the text is).
    @discardableResult
    public func put(_ slot: Slot, entry: RunSheetEntry, titles: [String], audio: [PreparedAudio], voice: String) -> CachedEntry {
        var r = load() ?? Record(voice: voice)
        if r.voice != voice { r = Record(voice: voice) }
        let other = (slot == .opening ? r.first : r.opening)?.files ?? []
        let otherBytes = other.reduce(0) { $0 + (((try? FileManager.default.attributesOfItem(atPath: dir.appendingPathComponent($1.file).path)[.size]) as? Int) ?? 0) }
        let size = audio.reduce(0) { $0 + $1.samples.count * 2 }
        var files: [CachedEntry.File] = []
        if otherBytes + size <= maxBytes {
            for a in audio {
                let name = Self.fileName(voice: voice, text: a.text)
                if Self.writePCM(a.samples, to: dir.appendingPathComponent(name)) { files.append(.init(text: a.text, file: name, rate: a.rate)) }
            }
        }
        let c = CachedEntry(entry: entry, titles: titles, writtenAt: clock(), files: files)
        if slot == .opening { r.opening = c } else { r.first = c }
        save(r)
        log("AUTORADIO written-ahead written: kind=\(slot == .opening ? "opening" : "first") blocks=\(audio.count) files=\(files.count) samples=\(audio.reduce(0) { $0 + $1.samples.count })")
        return c
    }

    // MARK: Taking at launch

    /// What would play now, without taking it.
    func peek(voice: String) -> OpeningPlan {
        guard let r = load(), r.voice == voice else { return .none }
        let now = clock()
        func young(_ c: CachedEntry, _ b: TimeInterval) -> Bool { let age = now.timeIntervalSince(c.writtenAt); return age >= 0 && age <= b }
        // A forecast lapses 12 hours after its stretch starts (the server: calibration AutoradioLapse): a kept story older than that is played
        // without its claim, so replaying yesterday's samples never settles a lapsed claim as heard. The server's own copy, when it still has
        // the id, answers the phone's place as a duplicate and ends under its own rule (`late`, no verdict).
        let next = r.first.flatMap { c -> RunSheetEntry? in
            guard young(c, entryBound) else { return nil }
            var e = c.entry
            if now.timeIntervalSince(c.writtenAt) >= Self.forecastLapse { e.forecast = nil }
            return e
        }
        if let o = r.opening, young(o, bound), dayPart(o.writtenAt) == dayPart(now) {
            // The clock is read now, at the moment the app opens, never at write time (Moment.swift).
            var e = o.entry
            let text = Moment.compose(body: e.text ?? "", now: now, calendar: calendar)
            e.text = text; e.estSeconds = RunSheetEntry.estimateSeconds(words: text)
            return .opening(e, next: next)
        }
        if let n = next { return .entry(n) }
        return .none
    }

    /// Why `peek` found nothing, in one line of ids and seconds.
    func whyNone(voice: String) -> String {
        guard let r = load() else { return "no kept file" }
        guard r.voice == voice else { return "voice differs" }
        let now = clock()
        func age(_ c: CachedEntry?) -> String { c.map { String(Int(now.timeIntervalSince($0.writtenAt))) } ?? "-" }
        if r.opening == nil && r.first == nil { return "nothing kept in the file" }
        return "too old or another part of the day opening_age_s=\(age(r.opening)) first_age_s=\(age(r.first)) bound_s=\(Int(bound)) entry_bound_s=\(Int(entryBound))"
    }

    /// The launch: decide, load the audio of what will play into memory, and delete the record and its files (a stale or foreign one too).
    public func take(voice: String) -> OpeningPlan {
        let plan = peek(voice: voice)
        if case .none = plan { log("AUTORADIO written-ahead refused: \(whyNone(voice: voice))") }
        else if case .opening(_, let n) = plan { log("AUTORADIO written-ahead taken: opening next=\(n != nil)") }
        else { log("AUTORADIO written-ahead taken: first entry") }
        let r = load()
        lock.lock(); held = [:]; lock.unlock()
        if let r, r.voice == voice {
            var use: [CachedEntry] = []
            switch plan {
            case .opening(let o, let n): use = [r.opening].compactMap { $0 }; if n != nil, let f = r.first { use.append(f) }; _ = o
            case .entry: use = [r.first].compactMap { $0 }
            case .none: break
            }
            lock.lock()
            for c in use { for f in c.files { if let s = Self.readPCM(dir.appendingPathComponent(f.file)) { held[f.text] = PreparedAudio(text: f.text, samples: s, rate: f.rate) } } }
            lock.unlock()
        }
        save(nil)
        return plan
    }

    /// The programme player's hook: the audio held for a block's text (one use; the memory goes with it).
    public func prepared(text: String) -> PreparedAudio? {
        lock.lock(); defer { lock.unlock() }
        return held.removeValue(forKey: text)
    }
    public var heldCount: Int { lock.lock(); defer { lock.unlock() }; return held.count }

    /// Everything kept is deleted (the voice was changed, sign-out).
    public func clear() { lock.lock(); held = [:]; lock.unlock(); save(nil) }

    // MARK: Files

    static func fileName(voice: String, text: String) -> String {
        var h: UInt64 = 0xcbf29ce484222325
        for b in (voice + "\u{0}" + text).utf8 { h = (h ^ UInt64(b)) &* 0x100000001b3 }
        return String(h, radix: 16) + ".pcm"
    }
    static func writePCM(_ samples: [Float], to url: URL) -> Bool {
        var out = Data(capacity: samples.count * 2)
        for s in samples {
            var v = Int16(max(-1, min(1, s)) * 32767).littleEndian
            withUnsafeBytes(of: &v) { out.append(contentsOf: $0) }
        }
        do { try out.write(to: url, options: .atomic); return true } catch { return false }
    }
    static func readPCM(_ url: URL) -> [Float]? {
        guard let d = try? Data(contentsOf: url), d.count >= 2 else { return nil }
        return d.withUnsafeBytes { raw -> [Float] in
            let n = d.count / 2
            return (0..<n).map { Float(Int16(littleEndian: raw.loadUnaligned(fromByteOffset: $0 * 2, as: Int16.self))) / 32767 }
        }
    }
}

// MARK: The launch line

/// The one line a cold launch prints: where the time went. `engine_ms`, `session_ms`, `sheet_ms`, `first_audio_ms` are milliseconds since the
/// process started. `request_ms` is the first desk request's wait for its stream to open (the candidates' reads included), `first_segment_ms` the
/// wait from that request's start to its first entry; both are -1 when no request was made before first audio (the cached start makes none).
/// `voice_load_ms` and `synth_ms` are the first block's voice made and its sound made (-1 when the block came from the cache).
public final class LaunchTimings: @unchecked Sendable {
    public static let shared = LaunchTimings()
    private let lock = NSLock()
    private var marks: [String: Double] = [:]
    private var printed = false
    public var start: Date
    public var clock: () -> Date = { Date() }
    public var sink: (String) -> Void = { print($0) }

    public init(start: Date = LaunchTimings.processStart()) { self.start = start }

    public static func processStart() -> Date {
        #if canImport(Darwin)
        var kp = kinfo_proc()
        var size = MemoryLayout<kinfo_proc>.stride
        var mib: [Int32] = [CTL_KERN, KERN_PROC, KERN_PROC_PID, getpid()]
        if sysctl(&mib, 4, &kp, &size, nil, 0) == 0 {
            let t = kp.kp_proc.p_un.__p_starttime
            return Date(timeIntervalSince1970: Double(t.tv_sec) + Double(t.tv_usec) / 1_000_000)
        }
        #endif
        return Date()
    }

    /// Milliseconds since the process started, now.
    public func sinceStart() -> Double { clock().timeIntervalSince(start) * 1000 }
    /// Record a mark once (the first call wins); `value` is in milliseconds, default since the process started.
    public func mark(_ name: String, ms: Double? = nil) {
        let v = ms ?? sinceStart()
        lock.lock(); if marks[name] == nil { marks[name] = v }; lock.unlock()
    }
    public func value(_ name: String) -> Int { lock.lock(); defer { lock.unlock() }; return marks[name].map { Int($0.rounded()) } ?? -1 }

    /// Print the line, once per process, at the first sound of the launch.
    public func printLine(source: String) {
        lock.lock(); let already = printed; printed = true; lock.unlock()
        guard !already else { return }
        mark("first_audio")
        // The marks above are milliseconds since the process started, so when the radio was started by a press of Play (or any start that is not
        // the launch's own) they all include the wait before it: the Mac proof taps Play 5 s in and engine, session and sheet read 5428 together.
        // `play_ms` is when the first start was asked for, `since_play_ms` what the start itself took to first sound.
        let play = value("play"), first = value("first_audio")
        let sincePlay = play >= 0 ? first - play : -1
        sink("AUTORADIO launch engine_ms=\(value("engine")) session_ms=\(value("session")) sheet_ms=\(value("sheet")) request_ms=\(value("request")) first_segment_ms=\(value("first_segment")) voice_load_ms=\(value("voice_load")) synth_ms=\(value("synth")) first_audio_ms=\(first) source=\(source) play_ms=\(play) since_play_ms=\(sincePlay)")
    }
    /// The radio opens: every mark is milliseconds from here, not from the process (a host process can be hours old).
    public func begin() { lock.lock(); defer { lock.unlock() }; guard !begun else { return }; begun = true; start = clock() }
    private var begun = false
    public func reset() { lock.lock(); marks = [:]; printed = false; begun = false; lock.unlock() }
}
