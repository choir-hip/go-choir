import Foundation

/// The ledger of native calls shared code makes (`native.later`), kept so a call whose reply never comes is NAMED
/// (docs/problems/phone-pages-stall-placard-2026-11-24.md). Permanent diagnostic logging: op name and a sequence number only,
/// never an argument, a payload or user data. The two entry points wrap their reply here: AutoradioModule.later and the app
/// module's ModelsModule.later (ModelsEntry.swift).
///
///   AUTORADIO native begin op=<op> n=<seq>
///   AUTORADIO native end op=<op> n=<seq> ms=<elapsed>
///   AUTORADIO native open op=<op> n=<seq> for_ms=<age>      (every 5 s, for each call open longer than 5 s)
///   AUTORADIO native twice op=<op> n=<seq>                  (a reply called a second time)
public final class NativeOpLog: @unchecked Sendable {
    public static let shared = NativeOpLog()

    private struct Open { let op: String; let started: Int }
    private let lock = NSLock()
    private var seq = 0
    private var open: [Int: Open] = [:]
    private var finished: Set<Int> = []        // calls already answered, kept to name a second reply (only their number)
    private let now: () -> Int                 // milliseconds
    private let sink: (String) -> Void
    private let thresholdMs: Int
    private var timer: DispatchSourceTimer?

    public init(now: @escaping () -> Int = { Int(DispatchTime.now().uptimeNanoseconds / 1_000_000) },
                sink: @escaping (String) -> Void = { print($0) },
                thresholdMs: Int = 5000) {
        self.now = now; self.sink = sink; self.thresholdMs = thresholdMs
    }

    /// Log the begin and return the reply to hand on: it logs the end, or the second call.
    public func wrap(op: String, reply: @escaping ([String: Any]) -> Void) -> ([String: Any]) -> Void {
        lock.lock()
        seq += 1
        let n = seq
        open[n] = Open(op: op, started: now())
        lock.unlock()
        sink("AUTORADIO native begin op=\(op) n=\(n)")
        startWatcher()
        return { [self] value in
            lock.lock()
            let call = open.removeValue(forKey: n)
            let again = call == nil && finished.contains(n)
            if call != nil { finished.insert(n); if finished.count > 4096 { finished = finished.filter { $0 > n - 1024 } } }
            let ms = call.map { now() - $0.started } ?? 0
            lock.unlock()
            if let call { sink("AUTORADIO native end op=\(call.op) n=\(n) ms=\(ms)") }
            else if again { sink("AUTORADIO native twice op=\(op) n=\(n)") }
            reply(value)
        }
    }

    /// Log every call open longer than the threshold. The watcher's timer calls it every 5 s.
    public func check() {
        lock.lock()
        let t = now()
        let late = open.filter { t - $0.value.started > thresholdMs }.sorted { $0.key < $1.key }
        lock.unlock()
        for (n, c) in late { sink("AUTORADIO native open op=\(c.op) n=\(n) for_ms=\(t - c.started)") }
    }

    /// Started with the first call. On a background queue, not the main actor: a main thread that is itself stuck must not silence the watcher.
    private func startWatcher() {
        lock.lock(); defer { lock.unlock() }
        guard timer == nil else { return }
        let t = DispatchSource.makeTimerSource(queue: DispatchQueue(label: "autoradio.native-op-log", qos: .utility))
        t.schedule(deadline: .now() + 5, repeating: 5)
        t.setEventHandler { [weak self] in self?.check() }
        t.resume()
        timer = t
    }
}

/// The reply an entry point hands its body in place of ExactReply (a final class that cannot be wrapped): the same `send`, through the ledger.
public final class LoggedReply: @unchecked Sendable {
    private let go: ([String: Any]) -> Void
    public init(_ go: @escaping ([String: Any]) -> Void) { self.go = go }
    public func send(_ value: [String: Any]) { go(value) }
}
