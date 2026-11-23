import Foundation

// Two string helpers the run sheet's bounds use (cut to a byte budget on a character boundary; one line). The candidate set of the
// programme request, and the request itself, are shared code now (AutoradioLab/radio-writer.ts); the Swift writer is gone.

public enum StretchCandidates {
    /// `s` cut to at most `bytes` UTF-8 bytes, on a character boundary.
    static func cut(_ s: String, bytes: Int) -> String {
        if s.utf8.count <= bytes { return s }
        var out = ""
        var n = 0
        for ch in s {
            let c = String(ch).utf8.count
            if n + c > bytes { break }
            out.append(ch); n += c
        }
        return out
    }
    static func oneLine(_ s: String) -> String {
        s.components(separatedBy: .newlines).joined(separator: " ").replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression)
            .trimmingCharacters(in: .whitespaces)
    }
}
