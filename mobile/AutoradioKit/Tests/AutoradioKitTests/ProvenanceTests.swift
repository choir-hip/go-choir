import XCTest
@testable import AutoradioKit

// Provenance: the stories an entry was built on survive every hop on the phone (owner, 2026-11-17: "We need to keep the provenance").
// The hops: the programme stream decoder, the run sheet entry, its op in the log, the store's file, a relaunch, and the heard record.
@MainActor
final class ProvenanceTests: XCTestCase {
    func sse(_ segs: [[String: Any]]) -> String {
        segs.map { "event: segment\ndata: " + String(data: try! JSONSerialization.data(withJSONObject: $0), encoding: .utf8)! + "\n\n" }.joined()
    }
    func cite(_ route: String, _ role: String, _ label: String) -> [String: Any] { ["ref": "pub:\(route)@v2", "role": role, "label": label] }


}
