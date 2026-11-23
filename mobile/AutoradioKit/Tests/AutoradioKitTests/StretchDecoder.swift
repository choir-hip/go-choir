import Foundation
@testable import AutoradioKit

// The stretch decoder, as the tests use it to build segments. Production reads the programme route in shared code
// (AutoradioLab/programme-model.ts, radio-writer.ts); the Swift decoder was dead in the app and is kept here only so the
// player and run sheet tests can feed segments through the same wire shapes.

public enum StretchEvent: Sendable, Equatable {
    case segment(IncomingSegment)
    case done
    case error(String)
}

/// Reads a stretch as it arrives: server-sent events (`event:` and `data:` lines, the programme route) or one JSON object per line (the
/// fixtures). Chunks may end anywhere. Line endings are LF, CRLF or a lone CR. Anything it cannot use is counted and skipped.
public struct StretchDecoder {
    public private(set) var ignoredCount = 0
    public private(set) var droppedCount = 0
    private var line = ""
    private var afterCR = false
    private var sse = SSEParser()
    private var stretchID = "local"
    private var seen = 0
    public init() {}

    public mutating func feed(_ chunk: String) -> [StretchEvent] {
        var out: [StretchEvent] = []
        for u in chunk.unicodeScalars {
            if u == "\n" {
                if afterCR { afterCR = false; continue }                  // the LF of a CRLF whose CR already ended the line
                out += endLine()
            } else if u == "\r" {
                out += endLine(); afterCR = true
            } else {
                afterCR = false; line.unicodeScalars.append(u)
            }
        }
        return out
    }
    /// The stream ended: a last line with no line ending still counts.
    public mutating func finish() -> [StretchEvent] {
        afterCR = false
        var out = line.isEmpty ? [] : endLine()
        out += flushPending()
        return out
    }

    /// An `event:` line that no `data:` line followed (a bare `event: done`): it is dispatched when the next line that is not a comment or
    /// its data arrives, or when the stream ends. (AsyncLineSequence drops the blank line, so the blank line cannot be waited for.)
    private var pendingEvent: String?
    private mutating func flushPending() -> [StretchEvent] {
        guard let p = pendingEvent else { return [] }
        pendingEvent = nil
        return dispatch(p, [:])
    }

    private mutating func endLine() -> [StretchEvent] {
        let l = line; line = ""
        if l.hasPrefix(":") { return [] }                                    // a comment says nothing about the event in progress
        var out: [StretchEvent] = []
        if !l.hasPrefix("data:") { out = flushPending() }
        let rest = process(l)
        if l.hasPrefix("event:") { pendingEvent = l.dropFirst(6).trimmingCharacters(in: .whitespaces) }
        else if l.hasPrefix("data:") { pendingEvent = nil }
        return out + rest
    }

    private mutating func process(_ l: String) -> [StretchEvent] {
        if l.trimmingCharacters(in: .whitespaces).isEmpty { _ = sse.push(""); return [] }
        if l.hasPrefix("{") {
            guard let o = Self.object(l) else { ignoredCount += 1; return [] }
            return dispatch((o["type"] as? String) ?? (o["event"] as? String) ?? "segment", o)
        }
        if let e = sse.push(l) {
            guard let o = Self.object(e.data) else { ignoredCount += 1; return [] }
            return dispatch(e.name == "message" ? "segment" : e.name, o)
        }
        if l.hasPrefix(":") || l.hasPrefix("event:") || l.hasPrefix("id:") || l.hasPrefix("retry:") { return [] }
        ignoredCount += 1                                                    // not JSON, not SSE
        return []
    }

    /// A JSON number that is a whole number: not a bool, not a fraction, not a string.
    static func wholeNumber(_ v: Any?) -> Int? {
        guard let n = v as? NSNumber, CFGetTypeID(n) != CFBooleanGetTypeID() else { return nil }
        let d = n.doubleValue
        return d.isFinite && d == d.rounded() && abs(d) < 1e9 ? Int(d) : nil
    }
    private static func object(_ s: String) -> [String: Any]? {
        (try? JSONSerialization.jsonObject(with: Data(s.utf8))) as? [String: Any]
    }

    private mutating func dispatch(_ name: String, _ o: [String: Any]) -> [StretchEvent] {
        switch name {
        case "segment":
            let n = seen; seen += 1
            let kind = (o["kind"] as? String) ?? ""
            let text = (o["text"] as? String) ?? ""
            guard SegmentKinds.isKnown(kind), !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { droppedCount += 1; return [] }
            var clip: ProgrammeClipRef?
            if let c = o["clip"] as? [String: Any], let id = c["clip_id"] as? String, !id.isEmpty {
                clip = ProgrammeClipRef(clipID: id, storyRef: c["story_ref"] as? String ?? "", seconds: (c["seconds"] as? NSNumber)?.doubleValue ?? 0)
            }
            if kind == "podcast_clip", clip == nil { droppedCount += 1; return [] }
            // ASSUMED wire: `take` is the id, or an object with `take_id`; the forecast is `forecast{p_heard,caused_by,claim_id}` or the line's own flat fields.
            var takeID: String?
            if let t = o["take"] as? String { takeID = t } else if let t = o["take"] as? [String: Any] { takeID = t["take_id"] as? String }
            if kind == "take", takeID.map({ !$0.isEmpty && $0.utf8.count <= 64 && $0.unicodeScalars.allSatisfy { $0.value < 128 && (CharacterSet.alphanumerics.contains($0) || $0 == "-" || $0 == "_") } }) != true { droppedCount += 1; return [] }
            var forecast: RunSheetForecast?
            let fo = (o["forecast"] as? [String: Any]) ?? o
            if let p = (fo["p_heard"] as? NSNumber)?.doubleValue, p.isFinite, p >= 0, p <= 1 {
                forecast = RunSheetForecast(pHeard: p, causedBy: (fo["caused_by"] as? String) ?? "", claimId: (fo["claim_id"] as? String) ?? (o["claim_id"] as? String) ?? "")
            }
            // ASSUMED wire (learn-settle 64e1b34c2): the event carries `entry_id` and, for a content segment, `claim_id`, flat on the event.
            let entryID = (o["entry_id"] as? String).flatMap { $0.isEmpty ? nil : $0 }
            let claimID = ((o["claim_id"] as? String) ?? ((o["forecast"] as? [String: Any])?["claim_id"] as? String)).flatMap { $0.isEmpty ? nil : $0 }
            if kind == "take", forecast == nil { droppedCount += 1; return [] }
            let st = (o["stretch_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? stretchID
            let id = (o["segment_id"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? "\(st)/\(n)"
            var cites: [ProgrammeCite] = []
            for c in (o["cites"] as? [Any]) ?? [] {
                if let r = c as? String, !r.isEmpty { cites.append(ProgrammeCite(ref: r)) }
                else if let d = c as? [String: Any], let r = d["ref"] as? String, !r.isEmpty {
                    cites.append(ProgrammeCite(ref: r, role: d["role"] as? String ?? "", label: d["label"] as? String ?? ""))
                }
            }
            var between: IncomingBetween?
            if let b = o["between"] as? [String: Any], let a = (b["after_segment"] ?? b["after"]) as? String, let z = (b["before_segment"] ?? b["before"]) as? String, !a.isEmpty, !z.isEmpty {
                between = IncomingBetween(after: a, before: z)
            }
            return [.segment(IncomingSegment(stretchID: st, segmentID: id, kind: kind, title: (o["title"] as? String) ?? "", text: text, cites: cites,
                                             blockCount: Self.wholeNumber(o["block_count"]), clip: kind == "podcast_clip" ? clip : nil, between: between,
                                             takeID: kind == "take" ? takeID : nil, forecast: forecast, entryID: entryID, claimID: claimID))]
        case "stretch":
            if let id = o["stretch_id"] as? String, !id.isEmpty { stretchID = id }
            return []
        case "done": return [.done]
        case "error":
            let reason = o["reason"] as? String ?? "", spoken = o["spoken"] as? String ?? ""
            return [.error(reason.isEmpty ? (spoken.isEmpty ? "the stretch failed" : spoken) : (spoken.isEmpty ? reason : "\(reason): \(spoken)"))]
        default:
            ignoredCount += 1                                                // handoff (reserved) and anything unknown
            return []
        }
    }
}

