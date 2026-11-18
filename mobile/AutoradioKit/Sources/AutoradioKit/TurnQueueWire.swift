import Foundation

// The turn route's queue words, over the run sheet (docs/reference/AUTORADIO-TURNS.md, "The queue"). `POST /api/autoradio/turns` still takes
// a `queue` object and answers with a `queue_ops` event in the old vocabulary until the turn route moves onto the run sheet (slices D3 and
// E2). This file is the only place the phone still speaks it, and it is a translation: the run sheet is the one list.
//
//   out: {version, current:{id,item,position_seconds}|null, upcoming:[{id,item,added_by}]}   (first 50 entries ahead)
//   in:  {"turn_id","base_version","ops":[{"op":"play_now|play_next|enqueue|remove|move|skip|previous|clear", ...}]}
//
// Ways this can fail (each has a test in RunSheetStoreTests):
//  1. An item the turn route cannot take (an address over its bound): left out of what the desk is shown.
//  2. A replayed turn changes the sheet twice: a turn's ops apply once (`turn_id`), and every op carries an op_id made from it.
//  3. An op the sheet refuses (an entry that is no longer ahead): dropped silently, as the old rebase rule did.
//  4. A garbled op stops the others: each op is read on its own.

extension RunSheetPlayer {
    private static func clamp(_ s: String, _ n: Int) -> String { StretchCandidates.cut(s, bytes: n) }

    /// The wire item for an entry, or nil when it has none the desk can take.
    static func wireItem(_ e: RunSheetEntry) -> [String: Any]? {
        let title = clamp(e.displayTitle, 300)
        switch e.kind {
        case .page:
            if let r = e.paperRoute { return r.utf8.count <= 256 ? ["kind": "story", "route": r, "title": title] : nil }
            if let u = e.webURL { return u.absoluteString.utf8.count <= 1024 ? ["kind": "page", "url": u.absoluteString, "title": title] : nil }
            return nil
        case .document:
            guard let b = e.book, b.id.utf8.count <= 128 else { return nil }
            return ["kind": "document", "id": b.id, "title": title, "at": b.at]
        case .clip, .episode:
            guard let a = e.audioSpec, a.url.absoluteString.utf8.count <= 1024 else { return nil }
            var o: [String: Any] = ["kind": "audio", "url": a.url.absoluteString, "title": title, "show": clamp(a.show, 200)]
            if let s = a.start { o["start"] = s }
            if let en = a.end { o["end"] = en }
            if let ep = a.episodeId, ep.utf8.count <= 128 { o["episode_id"] = ep }
            return o
        default:
            return nil          // spoken entries (opening, transition, story, answer, result, mail) have no address to hand over
        }
    }

    /// The run sheet as the turn route's `queue` field.
    public func turnQueueJSON(maxUpcoming: Int = 50) -> Data? {
        let by: (RunSheetBy) -> String = { $0 == .listener ? "listener" : ($0 == .desk ? "dj" : "programme") }
        var cur: Any = NSNull()
        if let e = store.onAir(), let item = Self.wireItem(e) {
            cur = ["id": e.id, "item": item, "position_seconds": min(max(Double(documentBlock), 0), 10_000_000)] as [String: Any]
        }
        let up: [[String: Any]] = store.ahead(limit: Int.max).compactMap { e in
            Self.wireItem(e).map { ["id": e.id, "item": $0, "added_by": by(e.placedBy)] }
        }
        var o: [String: Any] = ["version": Int(store.sheet.version), "current": cur, "upcoming": Array(up.prefix(maxUpcoming))]
        // The entry on air, whatever its kind, so a steering turn ("less of this") has its referent: id, title, forecast claim, stories it cites.
        // Inside `queue`, where the route reads unknown fields leniently (the turn's top level refuses them).
        if let e = store.onAir() {
            var a: [String: Any] = ["id": e.id, "title": Self.clamp(e.displayTitle, 300)]
            if let c = e.forecast?.claimId, !c.isEmpty { a["claim_id"] = c }
            var stories: [String] = []
            for c in e.cites ?? [] where c.ref.hasPrefix("pub:") { let r = SteerTap.story(c.ref); if !r.isEmpty, !stories.contains(r) { stories.append(r) } }
            if !stories.isEmpty { a["stories"] = Array(stories.prefix(12)) }
            o["on_air"] = a
        }
        return try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys, .withoutEscapingSlashes])
    }

    private static func entry(fromWire item: [String: Any]) -> RunSheetEntry? {
        let title = item["title"] as? String ?? ""
        switch item["kind"] as? String ?? "" {
        case "story": return (item["route"] as? String).flatMap { $0.isEmpty ? nil : RunSheetEntry.paperStory(route: $0, title: title, by: .desk) }
        case "page": return (item["url"] as? String).flatMap(URL.init(string:)).map { RunSheetEntry.webPage(url: $0, title: title, by: .desk) }
        case "document": return (item["id"] as? String).map { RunSheetEntry.book(id: $0, title: title, at: item["at"] as? Int ?? 0, by: .desk) }
        case "audio":
            guard let u = (item["url"] as? String).flatMap(URL.init(string:)) else { return nil }
            let a = RunSheetAudio(url: u, title: title, show: item["show"] as? String ?? "", start: (item["start"] as? NSNumber)?.doubleValue,
                                  end: (item["end"] as? NSNumber)?.doubleValue, episodeId: item["episode_id"] as? String)
            return .audio(a, by: .desk)
        default: return nil
        }
    }

    /// What the turn route answered (the `queue_ops` event). The old rebase rule holds by the reducer's own rules: an op that names an entry
    /// applies only while that entry is ahead; additions always apply; `clear` and `previous` only when the sheet is still at `base_version`.
    @discardableResult
    public func applyTurnOps(_ data: Data) -> (applied: Int, dropped: Int) {
        let json = try? JSONSerialization.jsonObject(with: data)
        guard let o = (json as? [String: Any]) ?? (json as? [Any]).map({ ["ops": $0] }) else { return (0, 0) }
        let turn = o["turn_id"] as? String ?? ""
        let base = (o["base_version"] as? NSNumber)?.int64Value
        let raw = (o["ops"] as? [Any]) ?? (o["queue_ops"] as? [Any]) ?? []
        if !turn.isEmpty, !raw.isEmpty {
            if turnLog.contains(turn) { return (0, raw.count) }
            turnLog.append(turn); if turnLog.count > 100 { turnLog.removeFirst(turnLog.count - 100) }
        }
        var applied = 0, dropped = 0
        for (n, any) in raw.enumerated() {
            guard let d = any as? [String: Any], let name = d["op"] as? String else { dropped += 1; continue }
            let reason = (d["reason"] as? String).flatMap { $0.isEmpty ? nil : String($0.prefix(160)) } ?? "the desk answered"
            let id = d["id"] as? String
            let opId = turn.isEmpty ? store.mintOpId() : String("t-\(turn)-\(n)".prefix(64))
            var r: RunSheetResult?
            switch name {
            case "play_now", "play_next", "enqueue":
                guard let item = d["item"] as? [String: Any], let e = Self.entry(fromWire: item) else { dropped += 1; continue }
                let at: RunSheetAt = name == "play_now" ? .now : (name == "play_next" ? .next : .end)
                if at == .now { wants = true }
                r = store.applyFromDesk(RunSheetOp(opId: opId, by: .desk, op: .place, at: at, entry: e, reason: reason))
            case "remove":
                guard let id else { dropped += 1; continue }
                r = store.applyFromDesk(RunSheetOp(opId: opId, by: .desk, op: .remove, id: id, reason: reason))
            case "move":
                guard let id, let to = (d["to_index"] as? NSNumber)?.intValue else { dropped += 1; continue }
                let ahead = store.ahead(limit: Int.max).filter { $0.id != id }
                let op: RunSheetOp = to <= 0 || ahead.isEmpty
                    ? RunSheetOp(opId: opId, by: .desk, op: .move, at: .next, id: id, reason: reason)
                    : RunSheetOp(opId: opId, by: .desk, op: .move, at: .after, after: ahead[min(to, ahead.count) - 1].id, id: id, reason: reason)
                r = store.applyFromDesk(op)
            case "skip":
                guard let air = store.onAir(), id == nil || id == air.id else { dropped += 1; continue }
                r = store.applyFromDesk(RunSheetOp(opId: opId, by: .desk, op: .skip, id: air.id, ended: RunSheetEnded(outcome: .skipped), reason: reason))
            case "previous", "clear":
                let want = (d["base_version"] as? NSNumber)?.int64Value ?? base
                if want != nil, want != store.sheet.version { dropped += 1; continue }
                r = store.applyFromDesk(RunSheetOp(opId: opId, by: .desk, op: name == "previous" ? .previous : .clear, baseVersion: store.sheet.version, reason: reason))
            default: dropped += 1; continue
            }
            if r?.applied == true { applied += 1 } else { dropped += 1 }
        }
        return (applied, dropped)
    }
}
