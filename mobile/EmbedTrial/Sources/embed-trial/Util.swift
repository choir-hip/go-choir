import Foundation
import Darwin

func nowMs() -> Double { Double(DispatchTime.now().uptimeNanoseconds) / 1e6 }
func f1(_ x: Double) -> String { String(format: "%.1f", x) }

/// Peak physical footprint of this process in MB (the ledger's peak, what the system charges the process), and peak resident size.
func peakMemoryMB() -> (footprint: Double, resident: Double) {
    var info = task_vm_info_data_t()
    var count = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<integer_t>.size)
    let kr = withUnsafeMutablePointer(to: &info) {
        $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &count) }
    }
    var ru = rusage(); getrusage(RUSAGE_SELF, &ru)
    let fp = kr == KERN_SUCCESS ? Double(info.ledger_phys_footprint_peak) / 1_048_576 : 0
    return (fp, Double(ru.ru_maxrss) / 1_048_576)
}
func currentFootprintMB() -> Double {
    var info = task_vm_info_data_t()
    var count = mach_msg_type_number_t(MemoryLayout<task_vm_info_data_t>.size / MemoryLayout<integer_t>.size)
    let kr = withUnsafeMutablePointer(to: &info) {
        $0.withMemoryRebound(to: integer_t.self, capacity: Int(count)) { task_info(mach_task_self_, task_flavor_t(TASK_VM_INFO), $0, &count) }
    }
    return kr == KERN_SUCCESS ? Double(info.phys_footprint) / 1_048_576 : 0
}

func norm(_ v: [Float]) -> Float { sqrt(v.reduce(0) { $0 + $1 * $1 }) }
func dot(_ a: [Float], _ b: [Float]) -> Float { var s: Float = 0; for i in 0..<min(a.count, b.count) { s += a[i] * b[i] }; return s }
func normalized(_ v: [Float], dim: Int) -> [Float] {
    let h = Array(v.prefix(dim)); let n = norm(h)
    return n > 0 ? h.map { $0 / n } : h
}

let cacheDir: URL = FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent(".cache/autoradio-embed-trial")
func percentile(_ xs: [Double], _ p: Double) -> Double { let s = xs.sorted(); return s.isEmpty ? 0 : s[min(s.count - 1, Int(Double(s.count) * p))] }

/// What a route is.
protocol TrialEmbedder: AnyObject {
    var name: String { get }
    /// `text` is the whole string to embed (prompt prefix included). Returns the vector (768) and the token count.
    func embed(_ text: String) async throws -> (vec: [Float], tokens: Int)
}
// The prompt strings, verbatim from the card's model/config.json (document, titled form; search_query).
let queryPrefix = "task: search result | query: "
func documentText(title: String, body: String) -> String { "title: \(title) | text: \(body)" }
