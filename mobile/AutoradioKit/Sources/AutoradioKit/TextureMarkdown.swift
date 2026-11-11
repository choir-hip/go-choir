import Foundation

// A small Markdown reader for documents with no body_doc (a legacy revision, a text file, a mail body). It reads the
// blocks the reader draws: headings, paragraphs, lists, quotes, code fences, rules, tables; and the inline marks strong,
// emphasis, code and links. It is conservative: anything it does not recognise is literal text.
// Its failure list is in TextureDoc.swift (item 10 and 11).

public enum Markdown {
    public struct Block {
        public var kind: String          // p h li quote code rule table
        public var text = ""
        public var level = 0
        public var depth = 0
        public var marker = ""
        public var rows: [[String]] = []
        public var header = false
    }

    public static func blocks(_ src: String) -> [Block] {
        let lines = src.replacingOccurrences(of: "\r\n", with: "\n").components(separatedBy: "\n")
        var out: [Block] = []
        var para: [String] = []
        func flush() {
            if !para.isEmpty { out.append(Block(kind: "p", text: para.joined(separator: " "))); para = [] }
        }
        var i = 0
        while i < lines.count {
            let raw = lines[i]
            let line = raw.trimmingCharacters(in: CharacterSet(charactersIn: " \t"))
            // fenced code (an unterminated fence runs to the end: its text is kept)
            if line.hasPrefix("```") {
                flush()
                var code: [String] = []
                i += 1
                while i < lines.count, !lines[i].trimmingCharacters(in: .whitespaces).hasPrefix("```") { code.append(lines[i]); i += 1 }
                out.append(Block(kind: "code", text: code.joined(separator: "\n")))
                i += 1; continue
            }
            if line.isEmpty { flush(); i += 1; continue }
            if isRule(line) { flush(); out.append(Block(kind: "rule")); i += 1; continue }
            if let h = heading(line) { flush(); out.append(Block(kind: "h", text: h.text, level: h.level)); i += 1; continue }
            // table: a header row followed by a separator row
            if line.hasPrefix("|") || line.contains("|"), i + 1 < lines.count, isSeparator(lines[i + 1]), cells(line).count > 1 {
                flush()
                var rows = [cells(line)]
                i += 2
                while i < lines.count, lines[i].contains("|"), !lines[i].rdTrim.isEmpty { rows.append(cells(lines[i])); i += 1 }
                out.append(Block(kind: "table", rows: rows, header: true))
                continue
            }
            if line.hasPrefix(">") {
                flush()
                var depth = 0
                var rest = Substring(line)
                while rest.hasPrefix(">") { depth += 1; rest = rest.dropFirst().drop(while: { $0 == " " }) }
                if !rest.isEmpty { out.append(Block(kind: "quote", text: String(rest), depth: depth)) }
                i += 1; continue
            }
            if let li = listItem(raw) {
                flush()
                out.append(Block(kind: "li", text: li.text, depth: li.depth, marker: li.marker))
                i += 1; continue
            }
            para.append(line)
            i += 1
        }
        flush()
        // Consecutive quote lines of one depth join into one paragraph.
        var merged: [Block] = []
        for b in out {
            if b.kind == "quote", let l = merged.last, l.kind == "quote", l.depth == b.depth { merged[merged.count - 1].text += " " + b.text }
            else { merged.append(b) }
        }
        return merged
    }

    static func isRule(_ l: String) -> Bool {
        let t = l.replacingOccurrences(of: " ", with: "")
        guard t.count >= 3, let c = t.first, c == "-" || c == "*" || c == "_" else { return false }
        return t.allSatisfy { $0 == c }
    }

    static func heading(_ l: String) -> (level: Int, text: String)? {
        var n = 0
        for ch in l { if ch == "#" { n += 1 } else { break } }
        guard n >= 1, n <= 6, l.count > n, l[l.index(l.startIndex, offsetBy: n)] == " " else { return nil }
        return (n, String(l.dropFirst(n)).rdTrim.trimmingCharacters(in: CharacterSet(charactersIn: "#")).rdTrim)
    }

    static func isSeparator(_ l: String) -> Bool {
        let t = l.rdTrim
        guard t.contains("-"), t.contains("|") || t.hasPrefix(":") else { return false }
        return t.allSatisfy { "|-: ".contains($0) }
    }

    static func cells(_ l: String) -> [String] {
        var t = l.rdTrim
        if t.hasPrefix("|") { t.removeFirst() }
        if t.hasSuffix("|") { t.removeLast() }
        return t.components(separatedBy: "|").map { $0.rdTrim }
    }

    static func listItem(_ raw: String) -> (text: String, depth: Int, marker: String)? {
        let indent = raw.prefix(while: { $0 == " " || $0 == "\t" }).reduce(0) { $0 + ($1 == "\t" ? 4 : 1) }
        let t = raw.trimmingCharacters(in: CharacterSet(charactersIn: " \t"))
        let depth = 1 + indent / 2
        if let f = t.first, "-*+".contains(f), t.dropFirst().first == " ", !isRule(t) {
            return (String(t.dropFirst(2)).rdTrim, depth, depth % 2 == 1 ? "\u{2022}" : "\u{2013}")
        }
        let digits = t.prefix(while: { $0.isNumber })
        if !digits.isEmpty, digits.count <= 4, t.dropFirst(digits.count).hasPrefix(". ") {
            return (String(t.dropFirst(digits.count + 2)).rdTrim, depth, "\(digits).")
        }
        return nil
    }

    // MARK: Inline

    /// Strong, emphasis, code and links; every unmatched marker is literal.
    public static func inline(_ s: String, idPrefix: String) -> [ReaderRun] {
        var runs: [ReaderRun] = []
        func add(_ text: String, kind: String = "t", weight: Int, slant: String, href: String = "") {
            if text.isEmpty { return }
            runs.append(ReaderRun(id: "\(idPrefix)-\(runs.count)", text: text, kind: kind, weight: weight, slant: slant, href: href))
        }
        func parse(_ s: String, weight: Int, slant: String) {
            let chars = Array(s)
            var buf = ""
            var i = 0
            func flush() { add(buf, weight: weight, slant: slant); buf = "" }
            while i < chars.count {
                let c = chars[i]
                if c == "`", let j = find(chars, "`", from: i + 1), j > i + 1 {
                    flush(); add(String(chars[(i + 1)..<j]), kind: "code", weight: weight, slant: slant); i = j + 1; continue
                }
                if c == "*", i + 1 < chars.count, chars[i + 1] == "*", let j = findPair(chars, "**", from: i + 2), j > i + 2 {
                    flush(); parse(String(chars[(i + 2)..<j]), weight: 700, slant: slant); i = j + 2; continue
                }
                if (c == "*" || c == "_"), i + 1 < chars.count, !chars[i + 1].isWhitespace, boundaryBefore(chars, i, c),
                   let j = find(chars, c, from: i + 1), j > i + 1, !chars[j - 1].isWhitespace, boundaryAfter(chars, j, c) {
                    flush(); parse(String(chars[(i + 1)..<j]), weight: weight, slant: "italic"); i = j + 1; continue
                }
                if c == "[", let close = find(chars, "]", from: i + 1), close + 1 < chars.count, chars[close + 1] == "(",
                   let end = find(chars, ")", from: close + 2) {
                    let label = String(chars[(i + 1)..<close]), href = String(chars[(close + 2)..<end])
                    if !label.isEmpty, !href.isEmpty, !href.contains(" ") {
                        flush(); add(label, kind: "link", weight: weight, slant: slant, href: href); i = end + 1; continue
                    }
                }
                buf.append(c); i += 1
            }
            flush()
        }
        parse(s, weight: 400, slant: "normal")
        return runs
    }

    static func find(_ a: [Character], _ c: Character, from: Int) -> Int? {
        var i = from
        while i < a.count { if a[i] == c { return i }; i += 1 }
        return nil
    }
    static func findPair(_ a: [Character], _ p: String, from: Int) -> Int? {
        let pc = Array(p)
        var i = from
        while i + pc.count <= a.count { if Array(a[i..<(i + pc.count)]) == pc { return i }; i += 1 }
        return nil
    }
    /// `_` inside a word (snake_case) and `*` between spaces (5 * 3) are not emphasis.
    static func boundaryBefore(_ a: [Character], _ i: Int, _ c: Character) -> Bool {
        guard i > 0 else { return true }
        return c == "*" ? true : !(a[i - 1].isLetter || a[i - 1].isNumber)
    }
    static func boundaryAfter(_ a: [Character], _ j: Int, _ c: Character) -> Bool {
        guard j + 1 < a.count else { return true }
        return c == "*" ? true : !(a[j + 1].isLetter || a[j + 1].isNumber)
    }
}
