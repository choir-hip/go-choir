import Foundation

// The launch's timing line, and audio made before it was wanted (the block's samples). The written-ahead opening that fed `PreparedAudio` is the notebook lane's.

public struct PreparedAudio: Sendable, Equatable {
    public var text: String
    public var samples: [Float]
    public var rate: Double
    public init(text: String, samples: [Float], rate: Double) { self.text = text; self.samples = samples; self.rate = rate }
}

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
