import Foundation

// The phone's Texture document stays Texture-format compatible always (owner, 2026-11-16: "It still needs to be texture format compatible
// always"). A revision is the Markdown text (`content`) and the structured `body_doc` (`choir.texture_doc.v1`). The editor works on the
// Markdown; `TextureBodyMerge` writes the `body_doc` that goes with it, as the web editor does (frontend/src/packages/texture/
// texture-structured-editor-doc.ts): paragraph, heading, bullet_list, ordered_list, list_item, blockquote, code_block, horizontal_rule,
// hard_break, text with the marks strong, emphasis and code. The web has no link mark: a link is its text in `body_doc` and `[t](u)` in `content`.
//
// The rule that keeps it compatible: a block the listener did not change is the ORIGINAL node, byte for byte (its ids, its source_ref
// citations, an embed, a table, a node type this phone has never heard of). Only a changed block is written fresh, and it keeps its old id and
// the citations that were on it. A node with no Markdown block of its own (an embed, a source flow) stays where it was.
//
// Ways this can fail (each has a test in TextureBodyTests):
//  1. Load, edit nothing, save: the Markdown is the original string and the body_doc is the original, equal in every key.
//  2. A small edit changes other nodes: only the edited node differs; every other node is equal to its original.
//  3. A citation (source_ref), an embed, a table or an unknown node is dropped or reordered by an edit nearby.
//  4. An edited block whose original node this phone cannot write (a table, an embed) is flattened into a paragraph: it keeps its node and the
//     edit lives in `content` (counted in `opaqueEdits`).
//  5. A new document (no body_doc) gets none, or one the web editor cannot open: it gets schema and doc with fresh ids.

public enum TextureMd {
    // MARK: Blocks

    /// Top-level blocks of Markdown source: a fenced block is one block (blank lines inside it stay), otherwise blocks end at a blank line.
    public static func blocks(_ src: String) -> [String] {
        let lines = src.replacingOccurrences(of: "\r\n", with: "\n").components(separatedBy: "\n")
        var out: [String] = [], cur: [String] = []
        var i = 0
        func flush() { if !cur.isEmpty { out.append(cur.joined(separator: "\n")); cur = [] } }
        while i < lines.count {
            let l = lines[i]
            let t = l.trimmingCharacters(in: .whitespaces)
            if t.hasPrefix("```") || t.hasPrefix("~~~") {
                flush()
                let fence = String(t.prefix(3))
                var block = [l]; i += 1
                while i < lines.count { block.append(lines[i]); if lines[i].trimmingCharacters(in: .whitespaces).hasPrefix(fence) { i += 1; break }; i += 1 }
                out.append(block.joined(separator: "\n")); continue
            }
            if t.isEmpty { flush() } else { cur.append(l) }
            i += 1
        }
        flush()
        return out
    }

    enum Kind: Equatable { case code, heading, rule, quote, list, table, paragraph }

    static func kind(of block: String) -> Kind {
        let first = block.components(separatedBy: "\n").first ?? ""
        let t = first.trimmingCharacters(in: .whitespaces)
        if t.hasPrefix("```") || t.hasPrefix("~~~") { return .code }
        if headingLevel(t) != nil { return .heading }
        if isRule(t), !block.contains("\n") { return .rule }
        if t.hasPrefix(">") { return .quote }
        if listMarker(first) != nil { return .list }
        if t.hasPrefix("|") && block.contains("|") && block.components(separatedBy: "\n").count >= 2 { return .table }
        return .paragraph
    }

    static func headingLevel(_ t: String) -> (Int, String)? {
        var n = 0
        for c in t { if c == "#" { n += 1 } else { break } }
        guard (1...6).contains(n), t.dropFirst(n).first == " " else { return nil }
        return (n, String(t.dropFirst(n + 1)).trimmingCharacters(in: .whitespaces))
    }
    static func isRule(_ t: String) -> Bool {
        let s = t.replacingOccurrences(of: " ", with: "")
        guard s.count >= 3, let c = s.first, "-*_".contains(c) else { return false }
        return s.allSatisfy { $0 == c }
    }
    /// (indent, ordered?, number, rest) of a list line.
    static func listMarker(_ line: String) -> (indent: Int, ordered: Bool, number: Int, rest: String)? {
        var indent = 0
        var chars = Substring(line)
        while let c = chars.first, c == " " || c == "\t" { indent += c == "\t" ? 4 : 1; chars = chars.dropFirst() }
        if let c = chars.first, "-*+".contains(c), chars.dropFirst().first == " " { return (indent, false, 0, String(chars.dropFirst(2))) }
        var digits = ""
        var rest = chars
        while let c = rest.first, c.isNumber { digits.append(c); rest = rest.dropFirst() }
        if !digits.isEmpty, let p = rest.first, p == "." || p == ")", rest.dropFirst().first == " " { return (indent, true, Int(digits) ?? 1, String(rest.dropFirst(2))) }
        return nil
    }

    // MARK: Matching keys

    /// What a block says, without its syntax: letters and digits, lower case, link destinations and markers gone.
    public static func key(_ block: String) -> String {
        var s = block
        if s.trimmingCharacters(in: .whitespaces).hasPrefix("```") || s.trimmingCharacters(in: .whitespaces).hasPrefix("~~~") { s = s.components(separatedBy: "\n").dropFirst().joined(separator: "\n") }   // the fence's language word
        s = s.components(separatedBy: "\n").map { $0.replacingOccurrences(of: "^\\s*(>\\s*)*([-*+]|\\d+[.)])\\s+", with: "", options: .regularExpression) }.joined(separator: "\n")   // list markers
        s = s.replacingOccurrences(of: "\\]\\([^)]*\\)", with: "]", options: .regularExpression)
        return String(s.unicodeScalars.filter { CharacterSet.letters.union(.decimalDigits).contains($0) }.map(Character.init)).lowercased()
    }

    static func plainText(_ node: [String: Any]) -> String {
        if (node["type"] as? String) == "text" { return node["text"] as? String ?? "" }
        return ((node["content"] as? [[String: Any]]) ?? []).map(plainText).joined()
    }
    static func nodeKey(_ node: [String: Any]) -> String {
        String(plainText(node).unicodeScalars.filter { CharacterSet.letters.union(.decimalDigits).contains($0) }.map(Character.init)).lowercased()
    }

    static func compatible(_ k: Kind, _ type: String) -> Bool {
        switch k {
        case .code: return type == "code_block"
        case .heading: return type == "heading"
        case .rule: return type == "horizontal_rule"
        case .quote: return type == "blockquote"
        case .list: return type == "bullet_list" || type == "ordered_list"
        case .table: return type == "table"
        case .paragraph: return type == "paragraph"
        }
    }
    /// The node types this phone can write from Markdown.
    static let writable: Set<String> = ["paragraph", "heading", "bullet_list", "ordered_list", "blockquote", "code_block", "horizontal_rule"]

    // MARK: Markdown to nodes (what the web editor writes)

    static func newID(_ kind: String) -> String { "\(kind)-" + UUID().uuidString.replacingOccurrences(of: "-", with: "").prefix(8).lowercased() }

    public static func node(for block: String) -> [String: Any] {
        switch kind(of: block) {
        case .code:
            var lines = block.components(separatedBy: "\n")
            lines.removeFirst()
            if let last = lines.last, last.trimmingCharacters(in: .whitespaces).hasPrefix("```") || last.trimmingCharacters(in: .whitespaces).hasPrefix("~~~") { lines.removeLast() }
            return ["type": "code_block", "attrs": ["id": newID("code-block")], "content": [["type": "text", "text": lines.joined(separator: "\n")]]]
        case .heading:
            let (n, text) = headingLevel(block.trimmingCharacters(in: .whitespaces)) ?? (1, block)
            return ["type": "heading", "attrs": ["id": newID("heading"), "level": n], "content": inline(text)]
        case .rule:
            return ["type": "horizontal_rule", "attrs": ["id": newID("hr")]]
        case .quote:
            let inner = block.components(separatedBy: "\n").map { l -> String in
                var t = l.trimmingCharacters(in: .whitespaces)
                if t.hasPrefix(">") { t.removeFirst(); if t.hasPrefix(" ") { t.removeFirst() } }
                return t
            }.joined(separator: "\n")
            let content = blocks(inner).map(node(for:))
            return ["type": "blockquote", "attrs": ["id": newID("blockquote")], "content": content.isEmpty ? [paragraph("")] : content]
        case .list:
            return list(block.components(separatedBy: "\n"))
        case .table, .paragraph:
            return paragraph(block)
        }
    }

    static func paragraph(_ text: String) -> [String: Any] {
        ["type": "paragraph", "attrs": ["id": newID("paragraph")], "content": inline(text)]
    }

    static func list(_ lines: [String]) -> [String: Any] {
        let base = lines.compactMap { listMarker($0)?.indent }.min() ?? 0
        let first = listMarker(lines[0])
        var items: [[String]] = []
        for l in lines {
            if let m = listMarker(l), m.indent <= base { items.append([l]) } else if !items.isEmpty { items[items.count - 1].append(l) } else { items.append([l]) }
        }
        let ordered = first?.ordered ?? false
        let nodes: [[String: Any]] = items.map { item in
            let head = listMarker(item[0])?.rest ?? item[0]
            var content: [[String: Any]] = []
            var para = [head]
            var rest = Array(item.dropFirst())
            if let k = rest.firstIndex(where: { listMarker($0) != nil }) {
                para += rest[..<k].map { $0.trimmingCharacters(in: .whitespaces) }
                content.append(paragraphLines(para))
                content.append(list(Array(rest[k...])))
                rest = []
            } else {
                para += rest.map { $0.trimmingCharacters(in: .whitespaces) }
                content.append(paragraphLines(para))
            }
            return ["type": "list_item", "attrs": ["id": newID("list-item")], "content": content]
        }
        var attrs: [String: Any] = ["id": newID("list")]
        if ordered { attrs["start"] = max(first?.number ?? 1, 1) }
        return ["type": ordered ? "ordered_list" : "bullet_list", "attrs": attrs, "content": nodes]
    }

    static func paragraphLines(_ lines: [String]) -> [String: Any] {
        ["type": "paragraph", "attrs": ["id": newID("paragraph")], "content": inline(lines.joined(separator: "\n"))]
    }

    // MARK: Inline

    /// Inline Markdown as text nodes with the marks strong, emphasis and code, and hard breaks. A link keeps its text (the web has no link mark).
    static func inline(_ src: String) -> [[String: Any]] {
        var out: [[String: Any]] = []
        var lastMarks: [String]?
        func add(_ text: String, _ marks: [String]) {
            guard !text.isEmpty else { return }
            if let last = out.last, (last["type"] as? String) == "text", lastMarks == marks {
                var l = last; l["text"] = (last["text"] as? String ?? "") + text; out[out.count - 1] = l
                return
            }
            var n: [String: Any] = ["type": "text", "text": text]
            if !marks.isEmpty { n["marks"] = marks.map { ["type": $0] } }
            out.append(n); lastMarks = marks
        }
        func scan(_ s: [Character], _ marks: [String]) {
            var i = 0, buf = ""
            func flush() { add(buf, marks); buf = "" }
            while i < s.count {
                let c = s[i]
                if c == "\\", i + 1 < s.count {
                    if s[i + 1] == "\n" { flush(); out.append(["type": "hard_break"]); i += 2; continue }
                    buf.append(s[i + 1]); i += 2; continue
                }
                if c == " ", i + 2 < s.count, s[i + 1] == " ", s[i + 2] == "\n" { flush(); out.append(["type": "hard_break"]); i += 3; continue }
                if c == "\n" { buf.append(" "); i += 1; continue }
                if c == "`", let j = s[(i + 1)...].firstIndex(of: "`") {
                    flush(); add(String(s[(i + 1)..<j]), marks.contains("code") ? marks : marks + ["code"]); i = j + 1; continue
                }
                if (c == "*" || c == "_"), i + 1 < s.count, s[i + 1] == c, let j = closer(s, from: i + 2, [c, c]) {
                    flush(); scan(Array(s[(i + 2)..<j]), marks.contains("strong") ? marks : marks + ["strong"]); i = j + 2; continue
                }
                if (c == "*" || c == "_"), i + 1 < s.count, s[i + 1] != " ", let j = closer(s, from: i + 1, [c]) {
                    flush(); scan(Array(s[(i + 1)..<j]), marks.contains("emphasis") ? marks : marks + ["emphasis"]); i = j + 1; continue
                }
                if c == "[", let close = s[(i + 1)...].firstIndex(of: "]"), close + 1 < s.count, s[close + 1] == "(", let end = s[(close + 2)...].firstIndex(of: ")") {
                    flush(); scan(Array(s[(i + 1)..<close]), marks); i = end + 1; continue
                }
                buf.append(c); i += 1
            }
            flush()
        }
        func closer(_ s: [Character], from: Int, _ pat: [Character]) -> Int? {
            var j = from
            while j + pat.count <= s.count {
                if Array(s[j..<(j + pat.count)]) == pat, j > from { return j }
                j += 1
            }
            return nil
        }
        scan(Array(src), [])
        return out
    }
}

public enum TextureBodyMerge {
    public struct Result: Sendable { public var body: [String: Any]; public var opaqueEdits: Int }
    typealias Node = [String: Any]

    static func root(content: [Node], like original: [String: Any]?) -> [String: Any] {
        var out = original ?? [:]
        out["schema"] = (original?["schema"] as? String) ?? "choir.texture_doc.v1"
        var doc = (original?["doc"] as? [String: Any]) ?? ["type": "doc", "attrs": ["id": TextureMd.newID("doc")]]
        doc["content"] = content
        out["doc"] = doc
        return out
    }

    static func topNodes(_ body: [String: Any]?) -> [Node] { ((body?["doc"] as? [String: Any])?["content"] as? [Node]) ?? [] }

    /// The `body_doc` for `newMarkdown`, given the revision it was loaded from (`original`, `originalMarkdown`).
    public static func merge(original: [String: Any]?, originalMarkdown: String, newMarkdown: String) -> Result {
        if let original, newMarkdown == originalMarkdown { return Result(body: original, opaqueEdits: 0) }
        let s = topNodes(original)
        let b = TextureMd.blocks(originalMarkdown), n = TextureMd.blocks(newMarkdown)
        if original == nil || s.isEmpty && b.isEmpty {
            let nodes = n.map(TextureMd.node(for:))
            return Result(body: root(content: nodes.isEmpty ? [TextureMd.paragraph("")] : nodes, like: original), opaqueEdits: 0)
        }
        // Which original node each original block is (forward, by what it says and what it is).
        var map = [Int?](repeating: nil, count: b.count), claimed = Set<Int>()
        var cursor = 0
        for i in 0..<b.count {
            let k = TextureMd.kind(of: b[i]), key = TextureMd.key(b[i])
            var j = cursor
            while j < s.count {
                if TextureMd.compatible(k, (s[j]["type"] as? String) ?? ""), TextureMd.nodeKey(s[j]) == key { map[i] = j; claimed.insert(j); cursor = j + 1; break }
                j += 1
            }
        }
        // Nodes no block claims (an embed, a source flow, an unknown node) stay after the block that came before them.
        var lead: [Node] = [], attach: [Int: [Node]] = [:]
        var lastClaimedBlock: Int?
        let blockOf = Dictionary(uniqueKeysWithValues: map.enumerated().compactMap { i, j in j.map { ($0, i) } })
        for j in 0..<s.count {
            if let i = blockOf[j] { lastClaimedBlock = i }
            else if let i = lastClaimedBlock { attach[i, default: []].append(s[j]) }
            else { lead.append(s[j]) }
        }
        // Block diff (longest common subsequence on the exact source).
        let m = b.count, p = n.count
        var lcs = [[Int]](repeating: [Int](repeating: 0, count: p + 1), count: m + 1)
        if m > 0 && p > 0 { for i in stride(from: m - 1, through: 0, by: -1) { for k in stride(from: p - 1, through: 0, by: -1) {
            lcs[i][k] = b[i] == n[k] ? lcs[i + 1][k + 1] + 1 : max(lcs[i + 1][k], lcs[i][k + 1])
        } } }
        var pairs: [(Int, Int)] = []
        var i = 0, k = 0
        while i < m && k < p { if b[i] == n[k] { pairs.append((i, k)); i += 1; k += 1 } else if lcs[i + 1][k] >= lcs[i][k + 1] { i += 1 } else { k += 1 } }
        pairs.append((m, p))
        var out = lead
        var opaque = 0
        var pi = 0, pk = 0
        for (mi, mk) in pairs {
            let deleted = Array(pi..<mi), inserted = Array(pk..<mk)
            for (t, kk) in inserted.enumerated() {
                let old: Node? = t < deleted.count ? map[deleted[t]].map { s[$0] } : nil
                if let old, !TextureMd.writable.contains((old["type"] as? String) ?? "") {
                    out.append(old); opaque += 1; continue          // a table, an embed, an unknown node: kept; the edit lives in `content`
                }
                var fresh = TextureMd.node(for: n[kk])
                if let old { carry(from: old, to: &fresh) }
                out.append(fresh)
            }
            for d in deleted { if let a = attach[d] { out += a } }           // what sat after a removed block stays
            if mi < m {
                if let j = map[mi] { out.append(s[j]) } else { out.append(TextureMd.node(for: n[mk])) }
                if let a = attach[mi] { out += a }
            }
            pi = mi + 1; pk = mk + 1
        }
        return Result(body: root(content: out.isEmpty ? [TextureMd.paragraph("")] : out, like: original), opaqueEdits: opaque)
    }

    /// An edited node keeps the id it had, and a paragraph keeps its citations (they stay at the end, where the web editor puts them).
    static func carry(from old: Node, to fresh: inout Node) {
        if let id = (old["attrs"] as? [String: Any])?["id"] {
            var a = (fresh["attrs"] as? [String: Any]) ?? [:]; a["id"] = id; fresh["attrs"] = a
        }
        guard (old["type"] as? String) == "paragraph", (fresh["type"] as? String) == "paragraph" else { return }
        let refs = ((old["content"] as? [Node]) ?? []).filter { ($0["type"] as? String) == "source_ref" }
        if !refs.isEmpty { fresh["content"] = ((fresh["content"] as? [Node]) ?? []) + refs }
    }
}

extension TextureMd {
    /// The Markdown for a `body_doc` (the server's `content` is this, or near it). Nodes with no Markdown form (an embed, a table, a node this
    /// phone does not know) have no block: they are kept by the merge, not written here.
    public static func markdown(from body: [String: Any]?) -> String {
        TextureBodyMerge.topNodes(body).compactMap { block($0, indent: "") }.joined(separator: "\n\n")
    }

    static func inlineMarkdown(_ nodes: [[String: Any]]) -> String {
        var out = ""
        for n in nodes {
            switch n["type"] as? String {
            case "text":
                var t = n["text"] as? String ?? ""
                let marks = ((n["marks"] as? [[String: Any]]) ?? []).compactMap { $0["type"] as? String }
                if marks.contains("code") { t = "`\(t)`" }
                if marks.contains("emphasis") { t = "*\(t)*" }
                if marks.contains("strong") { t = "**\(t)**" }
                out += t
            case "hard_break": out += "  \n"
            default: break                    // source_ref and the rest have no text of their own
            }
        }
        return out
    }

    static func block(_ n: [String: Any], indent: String) -> String? {
        let content = (n["content"] as? [[String: Any]]) ?? []
        switch n["type"] as? String {
        case "paragraph": return indent + inlineMarkdown(content)
        case "heading": return String(repeating: "#", count: (n["attrs"] as? [String: Any])?["level"] as? Int ?? 1) + " " + inlineMarkdown(content)
        case "horizontal_rule": return "---"
        case "code_block": return "```\n" + (content.first?["text"] as? String ?? "") + "\n```"
        case "blockquote": return content.compactMap { block($0, indent: "") }.joined(separator: "\n\n").components(separatedBy: "\n").map { "> " + $0 }.joined(separator: "\n")
        case "bullet_list", "ordered_list":
            let ordered = (n["type"] as? String) == "ordered_list"
            var num = (n["attrs"] as? [String: Any])?["start"] as? Int ?? 1
            var lines: [String] = []
            for item in content {
                let kids = (item["content"] as? [[String: Any]]) ?? []
                let marker = ordered ? "\(num)." : "-"
                num += 1
                for (idx, kid) in kids.enumerated() {
                    if idx == 0, (kid["type"] as? String) == "paragraph" { lines.append(indent + marker + " " + inlineMarkdown((kid["content"] as? [[String: Any]]) ?? [])) }
                    else if let b = block(kid, indent: indent + "  ") { lines.append(b) }
                }
            }
            return lines.joined(separator: "\n")
        default: return nil
        }
    }
}
