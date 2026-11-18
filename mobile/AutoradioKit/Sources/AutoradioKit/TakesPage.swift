import Foundation

// The Takes section of the Autoradio page, as a model: what the record button, the review, the listener's takes, the muted voices and the
// listener's own record say. Pure: the session and the lists in, words out (design 11.2, 11.6, 11.7).
//
// Ways this can fail (written before the code; each is a test in TakesTests):
//   P1  the control offered matches the state: Record when idle, Stop with the time and the 1:00 bound while recording, a wait line while
//       the transcript is made, the review actions after, nothing to press while publishing
//   P2  Publish is offered only with a transcript; the sentence "Your voice will be offered..." is on the review, with the rights line
//   P3  a take recorded over an entry on air says which story it answers
//   P4  my takes: newest first, a length and a word for published or kept on the phone; a muted voice has an Unmute

public struct TakeMineRow: Codable, Sendable, Equatable {
    public var id = "", title = "", length = "", state = ""
    public init() {}
}
public struct TakeMutedRow: Codable, Sendable, Equatable {
    public var handle = "", line = ""
    public init() {}
}
public struct TakesPageInfo: Codable, Sendable, Equatable {
    /// idle | recording | transcribing | review | publishing
    public var state = "idle"
    public var elapsed = "0:00", limit = "1:00"
    public var fraction = 0.0
    public var canRecord = true, canStop = false, canPublish = false, canDiscard = false
    public var previewing = false
    public var transcript = ""
    public var respondsTo = ""
    public var publishLine = "", rightsLine = ""
    public var notice = ""
    public var justPublished = ""
    public var mine: [TakeMineRow] = []
    public var muted: [TakeMutedRow] = []
    /// The listener's own record: "heard through by 7 of 11 listeners over 5 takes", or "new: no outcomes yet"; "" before they publish.
    public var record = ""
    public var handle = ""
    public init() {}
}

public enum TakesPage {
    static func clock(_ s: Double) -> String { let n = max(0, Int(s)); return "\(n / 60):" + String(format: "%02d", n % 60) }

    @MainActor public static func make(session s: TakeSession, mutes: [String], own: TakeOwn) -> TakesPageInfo {
        var o = TakesPageInfo()
        o.state = s.state.rawValue
        o.elapsed = clock(s.elapsed); o.limit = clock(TakeBounds.maxSeconds)
        o.fraction = min(1, max(0, s.elapsed / TakeBounds.maxSeconds)) * (s.state == .recording ? 1 : 0)
        o.canRecord = s.state == .idle
        o.canStop = s.state == .recording
        o.canDiscard = s.state == .review
        o.canPublish = s.canPublish
        o.previewing = s.previewing
        o.notice = s.notice
        if s.state == .review || s.state == .publishing { o.transcript = s.transcript; o.publishLine = TakeBounds.publishLine; o.rightsLine = TakeBounds.rightsLine }
        if s.state == .recording || s.state == .review || s.state == .publishing, let r = s.respondsTo, !r.title.isEmpty { o.respondsTo = "Answering: \(r.title)" }
        if let p = s.published { o.justPublished = "Published as \(p.publisher)." }
        let df = RelativeDateTimeFormatter(); df.unitsStyle = .short
        o.mine = s.mine.sorted { ($0.at ?? .distantPast) > ($1.at ?? .distantPast) }.map { t in
            var r = TakeMineRow()
            r.id = t.takeID; r.title = t.title.isEmpty ? "A take" : t.title; r.length = RadioPage.length(t.duration)
            r.state = t.published ? "Published" : "On this phone"
            return r
        }
        o.muted = mutes.map { h in var r = TakeMutedRow(); r.handle = h; r.line = TakeMutes.notebookLine(h); return r }
        o.handle = own.handle
        if let rec = s.myRecord, !own.handle.isEmpty { o.record = rec.words }
        return o
    }
}
