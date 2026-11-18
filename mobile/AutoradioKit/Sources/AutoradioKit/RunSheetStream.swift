import Foundation

// What the phone knows of GET /api/autoradio/sheet/stream (docs/reference/AUTORADIO-SHEET.md), as pure rules. NOT yet a live connection: nothing
// opens the stream until slice J gives the phone the desk's entries from the sheet; this file is what that connection will use, so the rules
// are tested now.
//   - The path carries the version the phone has (`after`) and the `sheet_epoch` it holds (`epoch`): a repaired sheet sends the full sheet to a
//     reader that does not hold the epoch, even at the current version.
//   - A `sheet` event is a whole sheet: it replaces the local one and its epoch becomes the phone's.
//   - 503 is "the sheet is being rebuilt": reconnect with a growing pause. 429 is "too many streams": the phone already has the most the server
//     allows open (4), so it waits longer and does not open another.
//   - NOT done: folding a `sheet_ops` event into the local sheet (the server's versions and the phone's own are two counts until the phone rebases
//     its pending ops on the echo); a `sheet_ops` whose range does not start at the phone's version means "reconnect with after=<version>".

public enum RunSheetStreamPolicy {
    public static func path(after version: Int64?, epoch: Int64) -> String {
        var q: [String] = []
        if let v = version, v > 0 { q.append("after=\(v)") }
        if epoch > 0 { q.append("epoch=\(epoch)") }
        return "/api/autoradio/sheet/stream" + (q.isEmpty ? "" : "?" + q.joined(separator: "&"))
    }

    public enum Retry: Equatable, Sendable {
        /// Reconnect after this many seconds.
        case after(TimeInterval)
        /// Too many streams are open: wait this long and open none meanwhile.
        case tooManyStreams(TimeInterval)
        /// The route is not there (404, 501) or refuses the request: do not retry.
        case giveUp
    }

    /// What to do when the stream's request is answered with `status`; `attempt` counts the failures in a row.
    public static func retry(status: Int, attempt: Int) -> Retry {
        switch status {
        case 503: return .after(min(60, pow(2, Double(min(attempt, 6)))))        // rebuilding: 1, 2, 4 ... 60 s
        case 429: return .tooManyStreams(min(300, 30 * Double(max(attempt, 1))))
        case 404, 501: return .giveUp
        case 500...599: return .after(min(60, pow(2, Double(min(attempt, 6)))))
        default: return .giveUp
        }
    }
}

extension RunSheetStore {
    public enum StreamEvent: Equatable { case sheet, resync(from: Int64), ignored }

    /// One server-sent event of the stream. `sheet` replaces the local sheet; a `sheet_ops` that does not begin at the phone's version asks for a
    /// reconnect from the phone's version; its ops are NOT folded here (see the file's note).
    @discardableResult
    public func streamEvent(name: String, data: Data) -> StreamEvent {
        switch name {
        case "sheet":
            guard let theirs = try? JSONDecoder().decode(RunSheet.self, from: data) else { return .ignored }
            adopt(theirs, epoch: theirs.sheetEpoch > 0 ? theirs.sheetEpoch : sheetEpoch)
            return .sheet
        case "sheet_ops":
            struct Ops: Decodable { var from_version: Int64 }
            guard let o = try? JSONDecoder().decode(Ops.self, from: data) else { return .ignored }
            return o.from_version == sheet.version ? .ignored : .resync(from: sheet.version)
        default: return .ignored
        }
    }
}
