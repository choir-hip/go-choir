import Foundation

/// The demo stretch: what `POST /api/autoradio/programme` answers in demo mode (see `DemoURLProtocol`). Hand-written in the wire shape of
/// design 2.2 (events `stretch`, `segment`, `done`) so the simulator plays segments with no computer; the server's real ids and counts
/// replace these when station L1 is deployed.
enum DemoStretch {
    static let segments: [[String: Any]] = [
        ["kind": "ident", "title": "Autoradio", "text": "This is Autoradio. Here is what is on the paper."],
        ["kind": "story", "title": "A barrier for the harbour, and the power to run it",
         "text": "The harbor council has approved a tide barrier. The paper's story on the vote says work could start by spring.\n\nThe same week, a regional grid added its largest battery yet, which matters for the barrier's pumps in a storm.",
         "cites": [["ref": "pub:demo-harbor-barrier", "role": "subject", "label": "The harbor council approves a tide barrier"],
                   ["ref": "pub:demo-grid-storage", "role": "mentioned", "label": "A regional grid adds its largest battery yet"]]],
        ["kind": "headlines", "title": "Also on the paper",
         "text": "Also on the paper. City libraries extend their evening hours. The river festival returns with a quieter schedule.",
         "cites": [["ref": "pub:demo-ev-charging", "role": "mentioned", "label": "City libraries extend their evening hours"]]],
        ["kind": "bridge", "text": "That is all for now. More soon."],
    ]

    /// What an `extend` answers, one set per request in turn: different stories each time, so the demo desk keeps writing for a while.
    static let more: [[[String: Any]]] = [
        [["kind": "story", "title": "Rain returns to the coast", "text": "A band of rain reaches the coast tonight. Forecasters expect it to clear by midday."],
         ["kind": "transition", "text": "And from the water to the wires."],
         ["kind": "story", "title": "Trams run later on weekends", "text": "The city will run trams until one in the morning on Fridays and Saturdays, starting next month."]],
        [["kind": "story", "title": "A bakery wins the street's vote", "text": "Neighbours chose a corner bakery as the street's favourite shop, the first time the prize has gone to a food seller."],
         ["kind": "story", "title": "School gardens reopen", "text": "Four school gardens reopen this week after a winter of repairs."]],
        [["kind": "story", "title": "The ferry timetable changes", "text": "From Monday the harbour ferry leaves ten minutes earlier on weekdays."]],
    ]
    nonisolated(unsafe) private static var extendCalls = 0
    private static let extendLock = NSLock()

    /// The stream for a request: the opening stretch for `open`, then a different set for each `extend`.
    static func body(cause: String) -> String {
        guard cause == "extend" else { return sse }
        extendLock.lock(); let n = extendCalls; extendCalls += 1; extendLock.unlock()
        return stream(more[n % more.count], stretch: "demo-" + UUID().uuidString.replacingOccurrences(of: "-", with: "").prefix(8).lowercased())
    }

    static var sse: String {
        // A new id for every request: the run sheet keeps what was heard across launches, and an entry id is never reused.
        let stretch = "demo-" + UUID().uuidString.replacingOccurrences(of: "-", with: "").prefix(8).lowercased()
        return stream(segments, stretch: stretch)
    }

    static func stream(_ segments: [[String: Any]], stretch: String) -> String {
        func line(_ o: [String: Any]) -> String {
            String(decoding: (try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys])) ?? Data(), as: UTF8.self)
        }
        var out = ": autoradio\n\nevent: stretch\ndata: " + line(["stretch_id": stretch]) + "\n\n"
        for (i, s) in segments.enumerated() {
            var o = s
            o["stretch_id"] = stretch; o["segment_id"] = "\(stretch)/\(i)"
            o["block_count"] = SplitBlocks.split(s["text"] as? String ?? "").count
            out += "event: segment\ndata: " + line(o) + "\n\n"
        }
        return out + "event: done\ndata: " + line(["stretch_id": stretch]) + "\n\n"
    }
}
