import Foundation

// MARK: Text to spoken blocks

public enum ChoirText {
    /// Plain spoken paragraphs from a story or revision body (markdown-ish text).
    /// The longest a spoken block is: about two sentences, so the first sound of a story comes after one short block is made and the rest
    /// is made while it plays. Reading pages keep the longer paragraphs.
    public static let speechChars = 280

    public static func paragraphs(_ text: String, maxChars: Int = 600) -> [String] {
        var out: [String] = []
        for raw in text.replacingOccurrences(of: "\r\n", with: "\n").components(separatedBy: "\n\n") {
            let p = strip(raw)
            if p.isEmpty { continue }
            out.append(contentsOf: split(p, maxChars: maxChars))
        }
        return out
    }

    /// Title first (unless the body already starts with it), then the paragraphs.
    public static func blocks(title: String, text: String, maxBlocks: Int = 200) -> [String] {
        var ps = paragraphs(text, maxChars: speechChars)
        let t = strip(title)
        if !t.isEmpty, ps.first?.lowercased() != t.lowercased() { ps.insert(t.hasSuffix(".") ? t : t + ".", at: 0) }
        return Array(ps.prefix(maxBlocks))
    }

    static func strip(_ s: String) -> String {
        var t = s
        // links [text](url) -> text; citation marks [12]; heading, list, quote, emphasis marks
        t = t.replacingOccurrences(of: "\\[([^\\]]*)\\]\\([^)]*\\)", with: "$1", options: .regularExpression)
        t = t.replacingOccurrences(of: "\\[\\d+\\]", with: "", options: .regularExpression)
        t = t.components(separatedBy: "\n").map { line in
            line.replacingOccurrences(of: "^\\s*(#{1,6}\\s+|>\\s*|[-*+]\\s+|\\d+\\.\\s+)", with: "", options: .regularExpression)
        }.joined(separator: " ")
        t = t.replacingOccurrences(of: "[*_`]", with: "", options: .regularExpression)
        t = t.replacingOccurrences(of: "\\s+", with: " ", options: .regularExpression)
        t = t.replacingOccurrences(of: " ([.,;:!?])", with: "$1", options: .regularExpression)
        return t.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static func split(_ p: String, maxChars: Int) -> [String] {
        if p.count <= maxChars { return [p] }
        var out: [String] = [], cur = ""
        var sentences: [String] = []
        p.enumerateSubstrings(in: p.startIndex..<p.endIndex, options: .bySentences) { s, _, _, _ in if let s { sentences.append(s) } }
        for s in sentences {
            if !cur.isEmpty, cur.count + s.count > maxChars { out.append(cur.trimmingCharacters(in: .whitespaces)); cur = "" }
            cur += s
        }
        if !cur.trimmingCharacters(in: .whitespaces).isEmpty { out.append(cur.trimmingCharacters(in: .whitespaces)) }
        return out
    }
}
