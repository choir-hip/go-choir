import Foundation

// What native code and shared code hand each other about the radio. The run sheet, its writer and the decisions are shared code (AutoradioLab/radio-runtime.ts, radio-bridge.ts); the deck (Deck.swift) is the
// audio. Exact has no call from native into shared code (docs/reference/exact/native-modules.md, "Out (Swift to the UI)"), so this is a mailbox, in the retrieval lane's pattern:
//   native -> shared   `send` puts a verb (JSON text) in the inbox and `changed` tells the module to touch the source `radioNote`; shared code's pump reads the inbox through the app module's `radioFacts` op.
//   shared -> native   `radioSet` hands over the turn route's queue snapshot (the voice turn carries it) and the entry on air (what a steering tap needs), as shared code last made them.
// Native because the voice turn engine and the handlers that have not moved are native; this file decides nothing about the radio.

public final class RadioBridge: @unchecked Sendable {
    public static let shared = RadioBridge()
    private let lock = NSLock()
    private var inbox: [String] = []
    private var queueJSON: Data?
    private var onAirJSON: Data?
    private var sheetJSON: Data?
    private var signedOutAck = false
    /// Set by the app module: asks shared code to look again (touches `radioNote`).
    public var changed: @Sendable () -> Void = {}

    /// A verb for shared code: `{verb: ..., ...}`. Verbs: turnOps {json}, turnEnded, level {value: pause|play|duck}, stopAll, openWeb {url}, closeWeb.
    public func send(_ verb: String, _ fields: [String: Any] = [:]) {
        var m = fields; m["verb"] = verb
        guard let d = try? JSONSerialization.data(withJSONObject: m), let s = String(data: d, encoding: .utf8) else { return }
        lock.lock(); if verb == "stopAll" { signedOutAck = false }; inbox.append(s); lock.unlock()
        changed()
    }
    /// Put an entry on the run sheet: `mode` is now, next or add. `words`: what a page's own text said, held for the entry (the entry itself is a pointer).
    public func place(_ entry: RunSheetEntry, mode: String, words: (title: String, paragraphs: [String])? = nil) {
        guard let d = try? JSONEncoder().encode(entry), let e = try? JSONSerialization.jsonObject(with: d) else { return }
        var f: [String: Any] = ["entry": e, "mode": mode]
        if let words { f["words"] = ["title": words.title, "paragraphs": words.paragraphs] }
        send("place", f)
    }
    /// Shared code says its sign-out is done: the `left` report went (or the deadline passed), the sheet and its file are gone (`radioSet` with `signedOut`).
    public func markSignedOut() { lock.lock(); signedOutAck = true; lock.unlock() }
    /// Sign-out waits here, with the old key still held, until shared code says it is done or `seconds` pass; whichever comes first. Returns whether shared code answered.
    @discardableResult public func waitSignedOut(seconds: Double) async -> Bool {
        let end = Date().addingTimeInterval(seconds)
        while Date() < end {
            lock.lock(); let done = signedOutAck; lock.unlock()
            if done { return true }
            try? await Task.sleep(nanoseconds: 50_000_000)
        }
        lock.lock(); defer { lock.unlock() }; return signedOutAck
    }
    /// What the pump takes: every message not yet read.
    public func drain() -> [String] { lock.lock(); defer { lock.unlock() }; let m = inbox; inbox = []; return m }

    /// The turn route's `queue` field, as shared code made it (JSON bytes; nil before the first pump).
    public var snapshot: Data? { lock.lock(); defer { lock.unlock() }; return queueJSON }
    public func set(snapshot: Data?, onAir: Data?, sheet: Data? = nil) { lock.lock(); queueJSON = snapshot; onAirJSON = onAir; sheetJSON = sheet; lock.unlock() }
    /// The run sheet as shared code last said (what the voice's retrieval and the content sync read of it); nil before the first pump.
    public func sheet() -> RunSheet? { lock.lock(); let d = sheetJSON; lock.unlock(); return d.flatMap { try? JSONDecoder().decode(RunSheet.self, from: $0) } }
    /// The entry on air as shared code last said (nil when none): what a "more like this" tap and a take's "responds to" read.
    public func onAirEntry() -> RunSheetEntry? { lock.lock(); let d = onAirJSON; lock.unlock(); return d.flatMap { try? JSONDecoder().decode(RunSheetEntry.self, from: $0) } }
}
