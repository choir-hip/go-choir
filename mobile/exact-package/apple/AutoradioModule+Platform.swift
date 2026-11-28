// The platform seam of the module: every UIKit or AppKit call the lanes need goes through here, so the rest of the
// module reads the same on iOS and on the Mac (Exact builds the same package for both; the Mac host is AppKit).
// iOS bodies are the code that used to sit in the lane files; the Mac bodies are the AppKit equivalents.
import AVFoundation
import ExactPackage
import Foundation
#if canImport(UIKit)
import UIKit
#elseif canImport(AppKit)
import AppKit
#endif

enum Platform {
    /// Hand a URL to the system (the browser, Mail, the phone app).
    static func open(_ url: URL) {
        DispatchQueue.main.async {
            #if os(iOS)
            UIApplication.shared.open(url)
            #else
            NSWorkspace.shared.open(url)
            #endif
        }
    }

    /// Where the sound goes, for the setup screen ("Playing on ...").
    static func outputName() -> String {
        #if os(iOS)
        return AVAudioSession.sharedInstance().currentRoute.outputs.first?.portName ?? "this phone"
        #else
        return "this Mac"
        #endif
    }
}

#if os(macOS)
/// The Mac has no audio session: AVAudioEngine opens the default input and output itself (Exact: "the Mac has no
/// session"; its ExactAudioSession claim is nil there). So this owner holds no claim and always reports ready; the
/// states are tracked only so a trace line says what the radio asked for. Echo cancellation and the single voice live
/// in the engine (voice processing I/O), not here.
final class MacSessionOwner: AudioSessionOwner, @unchecked Sendable {
    private let note: (String) -> Void
    private let lock = NSLock()
    private var held: AudioState?
    init(context: ExactPackageContext, note: @escaping (String) -> Void) { self.note = note }
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        lock.lock(); let changed = held != state; held = state; lock.unlock()
        if changed { note("claimed \(state.rawValue) (\(reason)): \(describe())") }
    }
    func release() { lock.lock(); held = nil; lock.unlock(); note("released") }
    func describe() -> String { "ready (macOS: no audio session)" }
    func observe(_ handler: @escaping @Sendable (String, String) -> Void) {}
}
typealias PlatformSessionOwner = MacSessionOwner
#else
typealias PlatformSessionOwner = ExactSessionOwner
#endif

#if os(macOS)
/// On the Mac the unified log hides NSLog text (`<private>`) and a GUI app's stdout goes nowhere, so every `AUTORADIO ...` line (the Kit
/// prints them) would be unreadable. stdout and stderr are pointed at `~/Library/Application Support/Autoradio/console.log`, line
/// buffered, capped at 256 KB (the file is rotated to console.log.1 when it passes the cap, at launch and every minute). Lines carry
/// counts and ids, never content, as they do on the phone.
enum ConsoleFile {
    private static let cap = 256 * 1024
    nonisolated(unsafe) private static var url: URL?
    nonisolated(unsafe) private static var timer: DispatchSourceTimer?
    static func install(directory: URL) {
        try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        url = directory.appendingPathComponent("console.log")
        open()
        let t = DispatchSource.makeTimerSource(queue: .global(qos: .utility))
        t.schedule(deadline: .now() + 60, repeating: 60)
        t.setEventHandler { rotateIfNeeded() }
        t.resume(); timer = t
        print("AUTORADIO console log opened pid=\(getpid())")
    }
    private static func open() {
        guard let url else { return }
        rotateIfNeeded()
        let fd = Darwin.open(url.path, O_WRONLY | O_CREAT | O_APPEND, 0o600)
        guard fd >= 0 else { return }
        dup2(fd, STDOUT_FILENO); dup2(fd, STDERR_FILENO); Darwin.close(fd)
        setvbuf(stdout, nil, _IOLBF, 0); setvbuf(stderr, nil, _IOLBF, 0)
    }
    private static func rotateIfNeeded() {
        guard let url, let size = (try? FileManager.default.attributesOfItem(atPath: url.path)[.size]) as? Int, size > cap else { return }
        let old = url.appendingPathExtension("1")
        try? FileManager.default.removeItem(at: old)
        try? FileManager.default.moveItem(at: url, to: old)
        let fd = Darwin.open(url.path, O_WRONLY | O_CREAT | O_APPEND, 0o600)
        if fd >= 0 { dup2(fd, STDOUT_FILENO); dup2(fd, STDERR_FILENO); Darwin.close(fd) }
    }
}
#endif

#if os(iOS)
/// The phone's console, kept: every `AUTORADIO ...` line (the Kit prints them; the session owner's notes come through `write`) is also appended to
/// `Library/Application Support/Autoradio/console.log`, so a run can be read after the fact (devicectl copies the file). The standard output stays
/// as it was (a pipe tees to the original). One file, capped at 512 KB: past the cap the oldest half is dropped. Lines carry counts, ids, states
/// and error codes only, never the words spoken, titles or mail.
enum ConsoleFile {
    private static let cap = 512 * 1024
    nonisolated(unsafe) private static var url: URL?
    private static let queue = DispatchQueue(label: "autoradio.console")
    static func install(directory: URL) {
        try? FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        url = directory.appendingPathComponent("console.log")
        var fds: [Int32] = [0, 0]
        guard pipe(&fds) == 0 else { return }
        let original = dup(STDOUT_FILENO)
        dup2(fds[1], STDOUT_FILENO)
        setvbuf(stdout, nil, _IOLBF, 0)
        let readFD = fds[0]
        Thread.detachNewThread {
            var pending = Data()
            var buf = [UInt8](repeating: 0, count: 4096)
            while true {
                let n = read(readFD, &buf, buf.count)
                if n <= 0 { break }
                if original >= 0 { _ = buf.withUnsafeBytes { Darwin.write(original, $0.baseAddress, n) } }
                pending.append(buf, count: n)
                while let nl = pending.firstIndex(of: 10) {
                    let line = String(decoding: pending[pending.startIndex..<nl], as: UTF8.self)
                    pending.removeSubrange(pending.startIndex...nl)
                    if line.contains("AUTORADIO") { write(line) }
                }
            }
        }
        write("AUTORADIO console log opened pid=\(getpid())")
    }
    /// Appends one line (with the time of day), and trims the file when it is past the cap.
    static func write(_ line: String) {
        queue.async {
            guard let url else { return }
            let stamp = ISO8601DateFormatter().string(from: Date())
            guard let data = "\(stamp) \(line)\n".data(using: .utf8) else { return }
            if let h = try? FileHandle(forWritingTo: url) { h.seekToEndOfFile(); h.write(data); try? h.close() }
            else { try? data.write(to: url) }
            if let size = (try? FileManager.default.attributesOfItem(atPath: url.path)[.size]) as? Int, size > cap, let all = try? Data(contentsOf: url) {
                var tail = all.suffix(all.count / 2)
                if let nl = tail.firstIndex(of: 10) { tail = tail[tail.index(after: nl)...] }          // start at a whole line
                try? Data(tail).write(to: url)
            }
        }
    }
}

#endif
