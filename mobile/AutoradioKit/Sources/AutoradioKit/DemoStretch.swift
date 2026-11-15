import Foundation

/// The demo stretch: what `POST /api/autoradio/programme` answers in demo mode (see `DemoURLProtocol`). Hand-written in the wire shape of
/// design 2.2 (events `stretch`, `segment`, `done`) so the simulator plays segments with no computer; the server's real ids and counts
/// replace these when station L1 is deployed.
enum DemoStretch {
    static let segments: [[String: Any]] = [
        ["kind": "ident", "title": "Autoradio", "text": "This is Autoradio. Here is what is on the paper this morning."],
        ["kind": "story", "title": "A barrier for the harbour, and the power to run it",
         "text": "The harbor council has approved a tide barrier. The paper's story on the vote says work could start by spring.\n\nThe same week, a regional grid added its largest battery yet, which matters for the barrier's pumps in a storm.",
         "cites": [["ref": "pub:demo-harbor-barrier", "role": "subject", "label": "The harbor council approves a tide barrier"],
                   ["ref": "pub:demo-grid-storage", "role": "mentioned", "label": "A regional grid adds its largest battery yet"]]],
        ["kind": "headlines", "title": "Also on the paper",
         "text": "Also on the paper. City libraries extend their evening hours. The river festival returns with a quieter schedule.",
         "cites": [["ref": "pub:demo-ev-charging", "role": "mentioned", "label": "City libraries extend their evening hours"]]],
        ["kind": "bridge", "text": "That is all for this stretch. More soon."],
    ]

    static var sse: String {
        let stretch = "demo-stretch-1"
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
