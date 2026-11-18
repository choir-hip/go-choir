import Foundation

// Demo mode's takes routes (the simulator tour fixtures/tours/takes-record-publish.steps): the public list and a record from fixtures, and the
// listener's own takes kept in memory for the run. Nothing leaves the phone. The wire shapes are TakesClient's.
enum TakesDemo {
    private static let lock = NSLock()
    nonisolated(unsafe) private static var stored: [[String: Any]] = []
    nonisolated(unsafe) private static var published: Set<String> = []

    static func answer(method: String, path: String, body: Data) -> (Int, Data)? {
        guard path.hasPrefix("/api/autoradio/takes") || path.hasPrefix("/api/platform/takes") else { return nil }
        func json(_ o: Any) -> Data { (try? JSONSerialization.data(withJSONObject: o)) ?? Data() }
        lock.lock(); defer { lock.unlock() }
        switch (method, path) {
        case ("POST", "/api/autoradio/takes"):
            let s = String(decoding: body, as: UTF8.self)
            var words = ""
            if let r = s.range(of: "name=\"transcript\"\r\n\r\n"), let e = s[r.upperBound...].range(of: "\r\n") { words = String(s[r.upperBound..<e.lowerBound]) }
            let id = String(format: "take-%016x", stored.count + 0xd01)
            stored.append(["take_id": id, "duration_seconds": 5, "transcript": words, "published": false, "publisher": ""])
            return (200, json(["take_id": id]))
        case ("POST", "/api/platform/takes"):
            let id = ((try? JSONSerialization.jsonObject(with: body)) as? [String: Any])?["take_id"] as? String ?? ""
            guard let i = stored.firstIndex(where: { ($0["take_id"] as? String) == id }) else { return (404, json(["error": "no such take"])) }
            stored[i]["published"] = true; stored[i]["publisher"] = "voice-de0001"; stored[i]["published_at"] = "2026-11-18T00:00:00Z"
            published.insert(id)
            return (200, json(["take_id": id, "publisher": "voice-de0001", "state": "live", "duration_seconds": 5, "transcript": stored[i]["transcript"] ?? ""]))
        case ("GET", "/api/autoradio/takes"): return (200, json(["takes": stored]))
        case ("GET", "/api/platform/takes"): return (200, Demo.fixture("takes-list") ?? json(["takes": []]))
        case ("GET", "/api/platform/takes/record"): return (200, Demo.fixture("takes-record") ?? json([:]))
        case ("DELETE", _):
            let id = path.split(separator: "/").last.map(String.init) ?? ""
            stored.removeAll { ($0["take_id"] as? String) == id }
            return (204, Data())
        default: return nil
        }
    }
}
