import Foundation

// The allowlist sanitiser behind "Show original" (written for the repair of the app review, 2026-11-11, replacing a chain of
// regular expressions that removed what it knew to be bad). Nothing in the output is copied from the input except text and
// the values of a few attributes, and those are re-escaped. The iframe sandbox and the CSP meta stay as the second layer.
//
// WAYS THIS CAN FAIL (written before the code; each is a row of the hostile-input table in MailTests):
//  1. A scheme the checker does not see: unquoted `href=javascript:`, entity-encoded (`jav&#x61;script:`, `&colon;`, `&Tab;`),
//     whitespace or control characters inside the scheme (`java\tscript:`, `\u{1}javascript:`), upper case, `vbscript:`, `data:`.
//     The value is entity-decoded once (as a browser does), stripped of every space and control character, and must then START
//     with `http://`, `https://`, `mailto:` or `#`; anything else loses the attribute. Fail-closed: an entity the decoder does not
//     know stays as `&...` and so cannot match.
//  2. Event handlers (`onclick`, `onerror`, `ONLOAD`, `on\u{0}click`): attributes are allowlisted by name, so none survives.
//  3. A tag that loads or runs: `<base>`, `<meta http-equiv=refresh>`, `<link>`, `<script>`, `<iframe>`, `<object>`, `<embed>`,
//     `<svg onload>`, `<math>`, `<form action>`, `<input>`, `<button formaction>`, `<img srcset>`: only allowlisted tag names
//     are written, and script, style, svg, math, object, template, head, ... are dropped WITH their content.
//  4. CSS that fetches or runs: `url(`, `expression(`, `@import`, `behavior`, `-moz-binding`, backslash escapes, comments: a
//     `style` attribute containing any of them is dropped whole; `<style>` elements are dropped.
//  5. Markup hidden in odd syntax: comments and conditional comments, `<!DOCTYPE`, `<![CDATA[`, `<?xml`, NUL bytes, `<` inside a
//     quoted attribute value, a tag with no closing `>`, a `<script>` with no end tag, nested `<svg><svg>`.
//  6. Images: only `data:image/png|jpeg|gif|webp;base64,...` survive as `src`; remote images lose the `src` (alt stays).
//  7. Output markup breaking out of the document: text has `<` and `>` escaped, attribute values have `& " < >` escaped.
//  8. An unclosed `<head>` swallowing the message: an opening `<body>` ends the head skip.

enum HTMLSanitizer {
    static let allowedTags: Set<String> = [
        "a", "abbr", "address", "article", "aside", "b", "big", "blockquote", "br", "caption", "center", "cite", "code", "col", "colgroup",
        "dd", "del", "dfn", "div", "dl", "dt", "em", "figcaption", "figure", "font", "footer", "h1", "h2", "h3", "h4", "h5", "h6", "header",
        "hr", "i", "img", "ins", "kbd", "li", "main", "mark", "nav", "ol", "p", "pre", "q", "s", "samp", "section", "small", "span",
        "strike", "strong", "sub", "sup", "table", "tbody", "td", "tfoot", "th", "thead", "tr", "tt", "u", "ul", "var", "wbr",
    ]
    static let voidTags: Set<String> = ["br", "hr", "img", "col", "wbr"]
    /// Their content is text up to the closing tag, never markup: skipped to `</name`.
    static let rawText: Set<String> = ["script", "style", "iframe", "noscript", "noembed", "noframes", "xmp", "textarea", "title", "plaintext"]
    /// Dropped with everything inside (tracked by nesting depth).
    static let dropWithContent: Set<String> = ["svg", "math", "object", "applet", "template", "head", "frameset", "video", "audio", "canvas", "map", "select"]
    static let globalAttrs: Set<String> = ["title", "lang", "dir", "align", "valign", "width", "height", "colspan", "rowspan", "bgcolor", "color",
                                           "size", "border", "cellpadding", "cellspacing", "alt"]

    struct Tag { var name: String; var closing: Bool; var attrs: [(String, String)] }

    // MARK: Entry

    static func clean(_ input: String) -> String {
        let s = Array(input.unicodeScalars.filter { $0.value != 0 })
        let n = s.count
        var out = ""
        var i = 0
        var skipName: String?                    // inside a dropWithContent element
        var skipDepth = 0

        while i < n {
            let c = s[i]
            guard c == "<" else {
                if skipName == nil { out += c == ">" ? "&gt;" : String(c) }
                i += 1
                continue
            }
            if has(s, i, "<!--") {                                  // a comment (conditional comments are comments too)
                i = find(s, "-->", from: i + 4).map { $0 + 3 } ?? n
                continue
            }
            if i + 1 < n, s[i + 1] == "!" || s[i + 1] == "?" {      // doctype, CDATA, processing instruction
                i = (s[i...].firstIndex(of: ">")).map { $0 + 1 } ?? n
                continue
            }
            let startsTag = i + 1 < n && (isLetter(s[i + 1]) || (s[i + 1] == "/" && i + 2 < n && isLetter(s[i + 2])))
            guard startsTag else {
                if skipName == nil { out += "&lt;" }
                i += 1
                continue
            }
            guard let (tag, next) = parseTag(s, i) else { break }   // no closing '>': the rest is not a tag, drop it
            i = next

            if let sk = skipName {
                if sk == "head", !tag.closing, tag.name == "body" { skipName = nil; skipDepth = 0; continue }   // an unclosed <head>
                if tag.name == sk {
                    if tag.closing { skipDepth -= 1; if skipDepth <= 0 { skipName = nil; skipDepth = 0 } } else { skipDepth += 1 }
                }
                continue
            }
            if tag.closing {
                if allowedTags.contains(tag.name), !voidTags.contains(tag.name) { out += "</\(tag.name)>" }
                continue
            }
            if rawText.contains(tag.name) {                         // skip to the matching end tag (parsed, and ignored, next)
                i = findEndTag(s, tag.name, from: i) ?? n
                continue
            }
            if dropWithContent.contains(tag.name) { skipName = tag.name; skipDepth = 1; continue }
            if allowedTags.contains(tag.name) { out += open(tag) }
            // any other tag (html, body, form, input, base, meta, link, font-like unknowns): the tag goes, its text stays
        }
        return out
    }

    // MARK: Writing a tag

    static func open(_ tag: Tag) -> String {
        var parts = ["<" + tag.name]
        var seen = Set<String>()
        for (rawName, rawValue) in tag.attrs {
            let name = rawName
            guard !seen.contains(name) else { continue }
            var value: String?
            switch name {
            case "href" where tag.name == "a": value = safeLink(rawValue)
            case "src" where tag.name == "img": value = safeImage(rawValue)
            case "style": value = safeStyle(rawValue)
            case _ where globalAttrs.contains(name): value = stripControls(decodeEntities(rawValue))
            default: value = nil
            }
            guard let v = value else { continue }
            seen.insert(name)
            parts.append("\(name)=\"\(escapeAttr(v))\"")
        }
        if tag.name == "a" { parts.append("rel=\"noopener noreferrer nofollow\"") }
        return parts.joined(separator: " ") + ">"
    }

    // MARK: Values

    /// Characters a browser ignores or that hide a scheme: all of them are removed before the scheme is looked at.
    static func ignorable(_ v: UInt32) -> Bool {
        v <= 0x20 || (0x7F...0xA0).contains(v) || (0x200B...0x200F).contains(v) || v == 0x2028 || v == 0x2029 || v == 0xFEFF || v == 0xAD
    }
    static func compact(_ s: String) -> String {
        var out = String.UnicodeScalarView()
        for u in s.unicodeScalars where !ignorable(u.value) { out.append(u) }
        return String(out)
    }
    static func stripControls(_ s: String) -> String {
        var out = String.UnicodeScalarView()
        for u in s.unicodeScalars where u.value >= 0x20 && u.value != 0x7F { out.append(u) }
        return String(out)
    }

    /// http, https, mailto and in-page anchors only, judged after decoding entities and removing hidden characters.
    static func safeLink(_ raw: String) -> String? {
        let v = compact(decodeEntities(raw))
        let l = v.lowercased()
        if l.hasPrefix("http://") || l.hasPrefix("https://") || l.hasPrefix("mailto:") || l.hasPrefix("#") { return v }
        return nil
    }
    static func safeImage(_ raw: String) -> String? {
        let v = compact(decodeEntities(raw))
        let ok = v.range(of: "^data:image/(png|jpe?g|gif|webp);base64,[A-Za-z0-9+/=]*$", options: [.regularExpression, .caseInsensitive]) != nil
        return ok ? v : nil
    }
    static func safeStyle(_ raw: String) -> String? {
        let v = stripControls(decodeEntities(raw))
        let l = compact(v).lowercased()
        for bad in ["url(", "expression", "@import", "behavior", "binding", "javascript:", "vbscript:", "image-set", "image(", "\\", "/*", "<", ">", "src("] where l.contains(bad) {
            return nil
        }
        return v
    }

    static func escapeAttr(_ s: String) -> String {
        var out = ""
        for u in s.unicodeScalars {
            switch u {
            case "&": out += "&amp;"
            case "\"": out += "&quot;"
            case "<": out += "&lt;"
            case ">": out += "&gt;"
            default: out.unicodeScalars.append(u)
            }
        }
        return out
    }

    // MARK: Entities

    static let named: [String: UInt32] = [
        "amp": 0x26, "lt": 0x3C, "gt": 0x3E, "quot": 0x22, "apos": 0x27, "nbsp": 0xA0, "tab": 0x09, "newline": 0x0A, "colon": 0x3A,
        "lpar": 0x28, "rpar": 0x29, "sol": 0x2F, "bsol": 0x5C, "period": 0x2E, "comma": 0x2C, "semi": 0x3B, "excl": 0x21, "num": 0x23,
        "percnt": 0x25, "equals": 0x3D, "quest": 0x3F, "commat": 0x40, "lsqb": 0x5B, "rsqb": 0x5D, "lbrack": 0x5B, "rbrack": 0x5D,
        "lbrace": 0x7B, "rbrace": 0x7D, "lcub": 0x7B, "rcub": 0x7D, "lowbar": 0x5F, "plus": 0x2B, "ast": 0x2A, "dollar": 0x24,
    ]
    /// Decode character references once, as a browser does in an attribute value. A reference not understood is left as written,
    /// so it can only make a value fail the scheme check, never pass it.
    static func decodeEntities(_ s: String) -> String {
        let u = Array(s.unicodeScalars)
        var out = String.UnicodeScalarView()
        var i = 0
        while i < u.count {
            guard u[i] == "&" else { out.append(u[i]); i += 1; continue }
            var j = i + 1
            if j < u.count, u[j] == "#" {
                j += 1
                var hex = false
                if j < u.count, u[j] == "x" || u[j] == "X" { hex = true; j += 1 }
                let digitsStart = j
                var value: UInt32 = 0
                while j < u.count, let d = digit(u[j], hex: hex), j - digitsStart < 8 { value = value &* (hex ? 16 : 10) &+ d; j += 1 }
                if j == digitsStart { out.append(u[i]); i += 1; continue }
                if j < u.count, u[j] == ";" { j += 1 }
                out.append(Unicode.Scalar(value).flatMap { $0.value == 0 ? nil : $0 } ?? "\u{FFFD}")
                i = j
                continue
            }
            let nameStart = j
            while j < u.count, j - nameStart < 12, isAlnum(u[j]) { j += 1 }
            let name = String(String.UnicodeScalarView(u[nameStart..<j])).lowercased()
            if j < u.count, u[j] == ";", let v = named[name], let sc = Unicode.Scalar(v) { out.append(sc); i = j + 1; continue }
            if ["amp", "lt", "gt", "quot", "nbsp"].contains(name), let v = named[name], let sc = Unicode.Scalar(v) { out.append(sc); i = j; continue }
            out.append(u[i]); i += 1
        }
        return String(out)
    }
    private static func digit(_ c: Unicode.Scalar, hex: Bool) -> UInt32? {
        switch c.value {
        case 0x30...0x39: return c.value - 0x30
        case 0x61...0x66 where hex: return c.value - 0x61 + 10
        case 0x41...0x46 where hex: return c.value - 0x41 + 10
        default: return nil
        }
    }

    // MARK: Scanning

    static func isLetter(_ c: Unicode.Scalar) -> Bool { (0x41...0x5A).contains(c.value) || (0x61...0x7A).contains(c.value) }
    static func isAlnum(_ c: Unicode.Scalar) -> Bool { isLetter(c) || (0x30...0x39).contains(c.value) }
    static func isSpace(_ c: Unicode.Scalar) -> Bool { c == " " || c == "\t" || c == "\n" || c == "\r" || c == "\u{0C}" }

    static func has(_ s: [Unicode.Scalar], _ i: Int, _ lit: String) -> Bool {
        let l = Array(lit.unicodeScalars)
        guard i + l.count <= s.count else { return false }
        for k in 0..<l.count where lower(s[i + k]) != l[k] { return false }
        return true
    }
    static func find(_ s: [Unicode.Scalar], _ lit: String, from: Int) -> Int? {
        guard from <= s.count else { return nil }
        var i = from
        while i + lit.unicodeScalars.count <= s.count { if has(s, i, lit) { return i }; i += 1 }
        return nil
    }
    /// The index of `</name` (followed by a space, '/', '>' or the end) at or after `from`.
    static func findEndTag(_ s: [Unicode.Scalar], _ name: String, from: Int) -> Int? {
        var i = from
        let len = name.unicodeScalars.count
        while let k = find(s, "</" + name, from: i) {
            let after = k + 2 + len
            if after >= s.count || isSpace(s[after]) || s[after] == "/" || s[after] == ">" { return k }
            i = k + 1
        }
        return nil
    }
    static func lower(_ c: Unicode.Scalar) -> Unicode.Scalar { (0x41...0x5A).contains(c.value) ? Unicode.Scalar(c.value + 32)! : c }

    /// Parse the tag at `start` (s[start] == "<"). Nil when it has no closing '>'.
    static func parseTag(_ s: [Unicode.Scalar], _ start: Int) -> (Tag, Int)? {
        let n = s.count
        var i = start + 1
        var closing = false
        if s[i] == "/" { closing = true; i += 1 }
        var name = ""
        while i < n, !isSpace(s[i]), s[i] != "/", s[i] != ">" { name.unicodeScalars.append(lower(s[i])); i += 1 }
        var attrs: [(String, String)] = []
        while true {
            while i < n, isSpace(s[i]) || s[i] == "/" { i += 1 }
            guard i < n else { return nil }
            if s[i] == ">" { i += 1; break }
            var an = ""
            while i < n, !isSpace(s[i]), s[i] != "=", s[i] != ">", s[i] != "/" { an.unicodeScalars.append(lower(s[i])); i += 1 }
            if an.isEmpty { i += 1; continue }                       // a stray '=' : step over it
            while i < n, isSpace(s[i]) { i += 1 }
            var value = ""
            if i < n, s[i] == "=" {
                i += 1
                while i < n, isSpace(s[i]) { i += 1 }
                guard i < n else { return nil }
                if s[i] == "\"" || s[i] == "'" {
                    let q = s[i]; i += 1
                    while i < n, s[i] != q { value.unicodeScalars.append(s[i]); i += 1 }
                    guard i < n else { return nil }
                    i += 1                                           // the closing quote
                } else {
                    while i < n, !isSpace(s[i]), s[i] != ">" { value.unicodeScalars.append(s[i]); i += 1 }
                }
            }
            attrs.append((an, value))
        }
        return (Tag(name: name, closing: closing, attrs: attrs), i)
    }
}
