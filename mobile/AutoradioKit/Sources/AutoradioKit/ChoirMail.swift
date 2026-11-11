import Foundation

// Mail, the listener's side (lane 6). The Mail agent on the listener's computer is the only custodian of email: the
// phone never talks to a mail provider, only to the computer, through the same /api/email routes the web uses
// (GET /api/email/messages?folder=, GET /api/email/messages/{id}, POST .../read and .../unread, GET /api/email/aliases,
// GET and POST /api/email/drafts, GET /api/email/drafts/{id}, POST /api/email/drafts/{id}/approval-email).
//
// Ways this can fail (written before the code, per the repo testing rule; MailTests covers each one it can):
//  1. A folder the server does not know (400 "unsupported folder"): the page shows the server's plain words, not a spinner.
//  2. A folder with nothing in it: status "empty" with a sentence, never a blank list.
//  3. No network (URLError notConnected / networkLost / timed out): status "offline"; the last good rows for that
//     folder (kept in memory) stay on screen under a banner; with none cached the page says so.
//  4. 401 / 403: "sign in again" words; 404 on a message: "this message is gone".
//  5. A message with only an HTML body: the readable text is made from it (script and style dropped, block tags become
//     paragraph breaks, entities decoded); with neither body, a single "(no text)" block, never a crash.
//  6. A huge body: the readable text is capped (200,000 characters) and the page says it was cut.
//  7. Hostile HTML for "Show original": script, iframe, object, embed, link, meta, base, on* attributes, javascript:
//     links, remote images and CSS url() are removed, and a Content-Security-Policy meta refuses every load but
//     inline style and data: images, so even a miss in the sanitiser fetches nothing and runs nothing.
//  8. Threading: "Re:", "RE:", "Fwd:", "Fw:", "[list]" prefixes, repeated ("Re: Re:"), and an empty subject (which
//     never groups, or every empty-subject message would be one thread).
//  9. An attachment name with path separators or no name; inbound attachments have no download route, so the row says
//     so (available = false) instead of linking to nothing.
// 10. Reply addressing: Reply-To header over From; for our own sent message the reply goes to its first recipient;
//     "Re: " is never doubled; the quoted body is limited to 30 lines.
// 11. Draft save with no recipient, a malformed one, or no words: refused here with a plain sentence, no request made.
//     No sending address (no alias): a plain sentence. A 403 on approval: "not allowed from this phone yet".
// 12. Dates: ISO 8601 with or without fractional seconds; an unparseable one is shown as the raw text (cut to 16).
// 13. A field missing from the JSON (no display name, no snippet, no recipients): tolerated, never a decode error.
// 14. Nothing here logs a body, a subject or an address: the client logs route templates only.

// MARK: Shapes (the Exact interface in exact-package/interface/mail.json mirrors these field for field)

public struct MailRow: Codable, Sendable, Equatable {
    public var id = "", address = "", sender = "", subject = "", snippet = "", stamp = "", trust = "", direction = ""
    public var unread = false, hasAttachments = false
    public var count = 1
    public init() {}
}
public struct MailFolderRow: Codable, Sendable, Equatable {
    public var name = "", label = "", address = "", badge = ""
    public var current = false
    public init() {}
}
public struct MailBoxView: Codable, Sendable, Equatable {
    /// "" (loading) | idle | ready | empty | offline | error
    public var status = "", message = "", folder = "", title = ""
    public var unread = 0, total = 0
    public var cached = false
    public var rows: [MailRow] = []
    public var folders: [MailFolderRow] = []
    public init() {}
}
public struct MailBlock: Codable, Sendable, Equatable {
    /// p | quote | sig
    public var kind = "p", text = ""
    public init(kind: String = "p", text: String = "") { self.kind = kind; self.text = text }
}
public struct MailAttachmentRow: Codable, Sendable, Equatable {
    public var id = "", filename = "", size = "", kind = "", address = "", note = ""
    public var available = false
    public init() {}
}
public struct MailThreadRow: Codable, Sendable, Equatable {
    public var id = "", address = "", sender = "", stamp = "", snippet = ""
    public var current = false
    public init() {}
}
public struct MailMessageView: Codable, Sendable, Equatable {
    public var status = "", message = "", id = "", address = "", subject = "", sender = "", fromAddress = "", toLine = "", ccLine = "", date = ""
    public var trust = "", trustNote = ""
    public var unread = false, htmlOnly = false, hasOriginal = false, hasQuoted = false, canReply = false, cut = false
    public var originalPath = ""
    public var blocks: [MailBlock] = []
    public var attachments: [MailAttachmentRow] = []
    public var thread: [MailThreadRow] = []
    public var replyLabel = "", spoken = ""
    public init() {}
}
public struct MailComposeView: Codable, Sendable, Equatable {
    /// "" (loading) | seed | saved | approval | error
    public var status = "", forAddress = "", sendAs = "", toLine = "", subject = "", body = "", draftId = "", note = "", reviewURL = ""
    public var replyTo = ""
    public init() {}
}

// MARK: Pure helpers

public enum MailText {
    /// Subject without "Re:", "Fwd:", "[list]" prefixes, lowercased; "" for an empty subject.
    public static func normalizedSubject(_ s: String) -> String {
        var t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        var changed = true
        while changed {
            changed = false
            let low = t.lowercased()
            for p in ["re:", "fwd:", "fw:", "aw:", "sv:"] where low.hasPrefix(p) {
                t = String(t.dropFirst(p.count)).trimmingCharacters(in: .whitespaces); changed = true; break
            }
            if !changed, t.hasPrefix("["), let close = t.firstIndex(of: "]"), t.distance(from: t.startIndex, to: close) <= 24 {
                t = String(t[t.index(after: close)...]).trimmingCharacters(in: .whitespaces); changed = true
            }
        }
        return t.lowercased()
    }

    public static func replySubject(_ s: String) -> String {
        let t = s.trimmingCharacters(in: .whitespacesAndNewlines)
        if t.isEmpty { return "Re:" }
        return t.lowercased().hasPrefix("re:") ? t : "Re: " + t
    }

    static let entities: [String: String] = ["amp": "&", "lt": "<", "gt": ">", "quot": "\"", "apos": "'", "nbsp": " ", "hellip": "…",
                                              "mdash": "—", "ndash": "–", "rsquo": "’", "lsquo": "‘", "rdquo": "”", "ldquo": "“", "copy": "©"]

    public static func decodeEntities(_ s: String) -> String {
        guard s.contains("&") else { return s }
        var out = "", i = s.startIndex
        while i < s.endIndex {
            if s[i] == "&", let semi = s[i...].prefix(12).firstIndex(of: ";") {
                let name = String(s[s.index(after: i)..<semi])
                var rep: String?
                if name.hasPrefix("#x") || name.hasPrefix("#X"), let v = UInt32(name.dropFirst(2), radix: 16), let u = Unicode.Scalar(v) { rep = String(u) }
                else if name.hasPrefix("#"), let v = UInt32(name.dropFirst()), let u = Unicode.Scalar(v) { rep = String(u) }
                else { rep = entities[name.lowercased()] }
                if let rep { out += rep; i = s.index(after: semi); continue }
            }
            out.append(s[i]); i = s.index(after: i)
        }
        return out
    }

    static func replace(_ s: String, _ pattern: String, _ with: String, options: NSRegularExpression.Options = [.caseInsensitive, .dotMatchesLineSeparators]) -> String {
        guard let re = try? NSRegularExpression(pattern: pattern, options: options) else { return s }
        return re.stringByReplacingMatches(in: s, range: NSRange(s.startIndex..., in: s), withTemplate: with)
    }

    /// HTML to plain paragraphs: no script or style, block ends are blank lines, list items are bullets.
    public static func htmlToText(_ html: String) -> String {
        var t = html
        t = replace(t, "<!--.*?-->", "")
        t = replace(t, "<(script|style|head|title|svg|noscript)\\b.*?</\\1\\s*>", "")
        t = replace(t, "<br\\s*/?>", "\n")
        t = replace(t, "<li\\b[^>]*>", "\n• ")
        t = replace(t, "</(p|div|tr|h[1-6]|ul|ol|table|blockquote)\\s*>", "\n\n")
        t = replace(t, "<[^>]*>", "")
        t = decodeEntities(t)
        t = replace(t, "[ \\t\\x{00A0}]+", " ", options: [])
        t = replace(t, " ?\\n ?", "\n", options: [])
        t = replace(t, "\\n{3,}", "\n\n", options: [])
        return t.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    static let maxChars = 200_000

    /// The readable body as blocks: paragraphs re-flowed (plain mail is hard-wrapped at 72), quoted paragraphs marked,
    /// a signature (from a "-- " line) marked. Returns whether the text came from HTML and whether it was cut.
    public static func blocks(text: String, html: String) -> (blocks: [MailBlock], htmlOnly: Bool, cut: Bool) {
        let plain = text.trimmingCharacters(in: .whitespacesAndNewlines)
        var source = plain, htmlOnly = false
        if plain.isEmpty, !html.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { source = htmlToText(html); htmlOnly = true }
        if source.isEmpty { return ([MailBlock(kind: "p", text: "(no text)")], htmlOnly, false) }
        var cut = false
        if source.count > maxChars { source = String(source.prefix(maxChars)); cut = true }
        source = source.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n")
        var out: [MailBlock] = []
        var inSig = false
        for chunk in source.components(separatedBy: "\n\n") {
            var lines = chunk.components(separatedBy: "\n")
            // "On <date>, <who> wrote:" stands apart from the quote under it.
            if !inSig, lines.count > 1, let first = lines.first?.trimmingCharacters(in: .whitespaces), first.hasSuffix("wrote:") {
                out.append(MailBlock(kind: "p", text: first)); lines.removeFirst()
            }
            if lines.first?.trimmingCharacters(in: .whitespaces) == "--" || inSig {
                inSig = true
                let body = lines.filter { $0.trimmingCharacters(in: .whitespaces) != "--" }.joined(separator: "\n")
                if !body.isEmpty { out.append(MailBlock(kind: "sig", text: body)) }
                continue
            }
            let nonEmpty = lines.filter { !$0.trimmingCharacters(in: .whitespaces).isEmpty }
            if nonEmpty.isEmpty { continue }
            let quoted = nonEmpty.filter { $0.trimmingCharacters(in: .whitespaces).hasPrefix(">") }.count * 2 > nonEmpty.count
            if quoted {
                let stripped = nonEmpty.map { l -> String in
                    var s = Substring(l.trimmingCharacters(in: .whitespaces))
                    while s.hasPrefix(">") { s = s.dropFirst(); if s.hasPrefix(" ") { s = s.dropFirst() } }
                    return String(s)
                }
                out.append(MailBlock(kind: "quote", text: reflow(stripped)))
            } else {
                out.append(MailBlock(kind: "p", text: reflow(nonEmpty)))
            }
        }
        return (out.isEmpty ? [MailBlock(kind: "p", text: "(no text)")] : out, htmlOnly, cut)
    }

    /// Join hard-wrapped lines with spaces, except where the lines look like a list or a short-lined block.
    static func reflow(_ lines: [String]) -> String {
        let t = lines.map { $0.trimmingCharacters(in: .whitespaces) }
        let listy = t.contains { $0.hasPrefix("- ") || $0.hasPrefix("* ") || $0.hasPrefix("• ") || $0.range(of: "^[0-9]+[.)] ", options: .regularExpression) != nil }
        let avg = t.isEmpty ? 0 : t.map(\.count).reduce(0, +) / t.count
        if listy || t.count == 1 || avg < 40 { return t.joined(separator: "\n") }
        return t.joined(separator: " ")
    }

    public static func hasQuoted(_ b: [MailBlock]) -> Bool { b.contains { $0.kind == "quote" } }

    /// What the radio says for a message: who, the subject, and the unquoted body, capped.
    public static func spoken(from: String, subject: String, blocks: [MailBlock]) -> String {
        let body = blocks.filter { $0.kind == "p" }.map(\.text).joined(separator: " ")
        let capped = body.count > 4000 ? String(body.prefix(4000)) + " … and more." : body
        return "Message from \(from). Subject: \(subject.isEmpty ? "none" : subject). \(capped)"
    }

    /// The quoted body of a reply: "On <date>, <who> wrote:" and up to 30 lines each starting "> ".
    public static func replyBody(date: String, who: String, blocks: [MailBlock]) -> String {
        let src = blocks.filter { $0.kind != "sig" && $0.text != "(no text)" }.map(\.text).joined(separator: "\n\n")
        let lines = src.components(separatedBy: "\n").prefix(30).map { "> " + $0 }.joined(separator: "\n")
        return "\n\nOn \(date), \(who) wrote:\n\(lines)"
    }

    public static func fileSize(_ n: Int64) -> String {
        if n < 1024 { return "\(n) B" }
        if n < 1024 * 1024 { return String(format: "%.0f KB", Double(n) / 1024) }
        return String(format: "%.1f MB", Double(n) / 1048576)
    }

    public static func safeFilename(_ s: String) -> String {
        let name = (s.replacingOccurrences(of: "\\", with: "/") as NSString).lastPathComponent.trimmingCharacters(in: .whitespacesAndNewlines)
        return name.isEmpty || name == "." || name == ".." ? "attachment" : name
    }

    public static func attachmentKind(filename: String, contentType: String) -> String {
        let ext = (filename as NSString).pathExtension.lowercased()
        if contentType.hasPrefix("image/") || ["png", "jpg", "jpeg", "gif", "heic", "webp"].contains(ext) { return "image" }
        if contentType == "application/pdf" || ext == "pdf" { return "pdf" }
        if contentType.hasPrefix("audio/") { return "audio" }
        if contentType.hasPrefix("video/") { return "video" }
        if contentType.hasPrefix("text/") || ["txt", "md", "csv"].contains(ext) { return "text" }
        if ext == "epub" { return "book" }
        return "file"
    }

    /// A light check, not a validator: one "@", something either side, a dot in the domain, no spaces.
    public static func looksLikeAddress(_ s: String) -> Bool {
        let t = s.trimmingCharacters(in: .whitespaces)
        guard !t.contains(" "), let at = t.firstIndex(of: "@"), at != t.startIndex else { return false }
        let domain = t[t.index(after: at)...]
        return domain.contains(".") && !domain.hasPrefix(".") && !domain.hasSuffix(".") && !domain.contains("@")
    }

    /// "Ana R. <ana@example.com>" -> "ana@example.com"; a bare address stays.
    public static func addressOnly(_ s: String) -> String {
        if let lt = s.firstIndex(of: "<"), let gt = s.firstIndex(of: ">"), lt < gt { return String(s[s.index(after: lt)..<gt]).trimmingCharacters(in: .whitespaces) }
        return s.trimmingCharacters(in: .whitespaces)
    }
}

public enum MailDates {
    static func parse(_ s: String) -> Date? {
        let f = ISO8601DateFormatter()
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        if let d = f.date(from: s) { return d }
        f.formatOptions = [.withInternetDateTime]
        return f.date(from: s)
    }
    /// "09:12" today, "Mon" within a week, "12 Mar" otherwise; the raw text (cut) when unparseable.
    public static func short(_ s: String, now: Date = Date(), calendar: Calendar = .current) -> String {
        guard let d = parse(s) else { return String(s.prefix(16)) }
        let f = DateFormatter(); f.calendar = calendar; f.timeZone = calendar.timeZone; f.locale = Locale(identifier: "en_GB")
        if calendar.isDate(d, inSameDayAs: now) { f.dateFormat = "HH:mm" }
        else if now.timeIntervalSince(d) < 6 * 86400 && d < now { f.dateFormat = "EEE" }
        else { f.dateFormat = "d MMM" }
        return f.string(from: d)
    }
    public static func long(_ s: String, calendar: Calendar = .current) -> String {
        guard let d = parse(s) else { return String(s.prefix(32)) }
        let f = DateFormatter(); f.calendar = calendar; f.timeZone = calendar.timeZone; f.locale = Locale(identifier: "en_GB")
        f.dateFormat = "EEE d MMM yyyy, HH:mm"
        return f.string(from: d)
    }
}

/// The sanitiser behind "Show original". Defence in depth: tags and attributes that load or run are removed, and a
/// Content-Security-Policy meta forbids everything but inline style and data: images, so a miss still loads nothing.
public enum MailHTML {
    public static let csp = "default-src 'none'; style-src 'unsafe-inline'; img-src data:"

    public static func sanitized(_ html: String) -> String {
        var t = html
        let ci: NSRegularExpression.Options = [.caseInsensitive, .dotMatchesLineSeparators]
        // Only what is inside <body> (or everything when there is no body tag).
        if let re = try? NSRegularExpression(pattern: "<body\\b[^>]*>(.*)</body\\s*>", options: ci),
           let m = re.firstMatch(in: t, range: NSRange(t.startIndex..., in: t)), let r = Range(m.range(at: 1), in: t) { t = String(t[r]) }
        t = MailText.replace(t, "<!--.*?-->", "")
        t = MailText.replace(t, "<(script|iframe|object|embed|applet|frameset|noscript|form|template)\\b.*?</\\1\\s*>", "")
        t = MailText.replace(t, "<(script|iframe|object|embed|applet|link|meta|base|input|button|frame|param|source|track)\\b[^>]*/?>", "")
        t = MailText.replace(t, "</?(html|head|body|title)\\b[^>]*>", "")
        t = MailText.replace(t, "\\s+on[a-z]+\\s*=\\s*(\"[^\"]*\"|'[^']*'|[^\\s>]+)", "")
        t = MailText.replace(t, "\\s+(href|src|action|formaction|xlink:href|background|poster)\\s*=\\s*(\"\\s*(javascript|vbscript|data:text)[^\"]*\"|'\\s*(javascript|vbscript|data:text)[^']*')", "")
        // Remote images: remove the src (keep the alt text readable); data: images stay.
        t = MailText.replace(t, "(<img\\b[^>]*?)\\s+src\\s*=\\s*(\"(?!data:image/)[^\"]*\"|'(?!data:image/)[^']*'|(?![\"'])(?!data:image/)[^\\s>]+)", "$1")
        t = MailText.replace(t, "\\s+(srcset|style)\\s*=\\s*\"[^\"]*(url\\(|expression\\(|@import)[^\"]*\"", "")
        t = MailText.replace(t, "@import[^;]*;", "")
        t = MailText.replace(t, "url\\([^)]*\\)", "none")
        return """
        <!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="\(csp)">
        <meta name="viewport" content="width=device-width, initial-scale=1"><style>:root{color-scheme:light dark}body{font:16px -apple-system,sans-serif;margin:12px;overflow-wrap:anywhere}img{max-width:100%;height:auto}table{max-width:100%}</style></head><body>\(t)</body></html>
        """
    }
}

// MARK: Parsing the maild JSON

public struct MailSummary: Sendable, Equatable {
    public var id = "", direction = "", fromAddress = "", fromDisplay = "", subject = "", snippet = "", trust = "", readAt = "", at = ""
    public var hasAttachments = false
    public var from: String { fromDisplay.isEmpty ? fromAddress : fromDisplay }
    public var unread: Bool { readAt.isEmpty && direction != "outbound" }
}
public struct MailAttachmentMeta: Sendable, Equatable { public var id = "", filename = "", contentType = "", status = ""; public var size: Int64 = 0 }
public struct MailDetail: Sendable, Equatable {
    public var summary = MailSummary()
    public var text = "", html = "", replyTo = ""
    public var to: [String] = [], cc: [String] = []
    public var attachments: [MailAttachmentMeta] = []
}
public struct MailDraftInfo: Sendable, Equatable {
    public var id = "", status = "", from = "", subject = "", text = "", replyToMessageID = ""
    public var to: [String] = []
}

public enum MailParse {
    static func s(_ o: [String: Any], _ k: String) -> String { o[k] as? String ?? "" }

    public static func summary(_ o: [String: Any]) -> MailSummary {
        var m = MailSummary()
        m.id = s(o, "id"); m.direction = s(o, "direction"); m.fromAddress = s(o, "from_address"); m.fromDisplay = s(o, "from_display")
        m.subject = s(o, "subject"); m.snippet = s(o, "snippet"); m.trust = s(o, "trust_status"); m.readAt = s(o, "read_at")
        let at = [s(o, "received_at"), s(o, "sent_at"), s(o, "created_at")].first { !$0.isEmpty } ?? ""
        m.at = at; m.hasAttachments = o["has_attachments"] as? Bool ?? false
        return m
    }

    public static func list(_ o: [String: Any]) -> (rows: [MailSummary], total: Int, unread: Int) {
        let rows = (o["messages"] as? [[String: Any]] ?? []).map(summary)
        return (rows, (o["total"] as? Int) ?? rows.count, (o["unread"] as? Int) ?? 0)
    }

    public static func detail(_ o: [String: Any]) -> MailDetail {
        var d = MailDetail()
        d.summary = summary(o["message"] as? [String: Any] ?? [:])
        d.text = s(o, "text_body"); d.html = s(o, "html_body")
        let headers = (o["raw_headers"] as? [String: Any]) ?? [:]
        for (k, v) in headers where k.lowercased() == "reply-to" { d.replyTo = MailText.addressOnly((v as? String) ?? "") }
        let rec = o["recipients"] as? [String: Any] ?? [:]
        func names(_ k: String) -> [String] { (rec[k] as? [[String: Any]] ?? []).map { r in
            let disp = s(r, "display"), addr = s(r, "address"); return disp.isEmpty ? addr : "\(disp) <\(addr)>" } }
        d.to = names("to"); d.cc = names("cc")
        d.attachments = (o["attachments"] as? [[String: Any]] ?? []).map {
            var a = MailAttachmentMeta(); a.id = s($0, "id"); a.filename = s($0, "filename"); a.contentType = s($0, "content_type")
            a.status = s($0, "status"); a.size = ($0["size_bytes"] as? NSNumber)?.int64Value ?? 0; return a }
        return d
    }

    public static func draft(_ o: [String: Any]) -> MailDraftInfo {
        var d = MailDraftInfo()
        d.id = s(o, "id"); d.status = s(o, "status"); d.from = s(o, "from_address"); d.subject = s(o, "subject"); d.text = s(o, "text_body")
        d.to = o["to_addresses"] as? [String] ?? []; d.replyToMessageID = s(o, "reply_to_message_id")
        return d
    }
}

// MARK: The service (what the Exact handlers and the PageSource call)

public enum MailThreads {
    /// Groups summaries (newest first as given) into one row per thread: the newest message stands for it.
    public static func rows(_ list: [MailSummary], now: Date = Date(), calendar: Calendar = .current) -> [MailRow] {
        var order: [String] = [], groups: [String: [MailSummary]] = [:]
        for m in list {
            let key = MailText.normalizedSubject(m.subject)
            let k = key.isEmpty ? "id:" + m.id : "s:" + key
            if groups[k] == nil { order.append(k) }
            groups[k, default: []].append(m)
        }
        return order.compactMap { k in
            guard let g = groups[k], let top = g.max(by: { ($0.at) < ($1.at) }) else { return nil }
            var r = row(top, now: now, calendar: calendar)
            r.count = g.count; r.unread = g.contains { $0.unread }
            return r
        }
    }
    public static func row(_ m: MailSummary, now: Date = Date(), calendar: Calendar = .current) -> MailRow {
        var r = MailRow()
        r.id = m.id; r.address = "mail:" + m.id; r.sender = m.from.isEmpty ? "(unknown sender)" : m.from
        r.subject = m.subject.isEmpty ? "(no subject)" : m.subject; r.snippet = m.snippet
        r.stamp = MailDates.short(m.at, now: now, calendar: calendar); r.unread = m.unread; r.hasAttachments = m.hasAttachments
        r.trust = m.trust; r.direction = m.direction
        return r
    }
    /// The messages sharing a subject, oldest first; just the one when the subject is empty.
    public static func thread(of m: MailSummary, in all: [MailSummary]) -> [MailSummary] {
        let key = MailText.normalizedSubject(m.subject)
        if key.isEmpty { return [m] }
        var seen = Set<String>()
        let same = ([m] + all).filter { MailText.normalizedSubject($0.subject) == key && seen.insert($0.id).inserted }
        return same.sorted { $0.at < $1.at }
    }
}

public final class MailService: @unchecked Sendable {
    public static let folders: [(name: String, label: String)] = [("inbox", "Inbox"), ("drafts", "Drafts"), ("sent", "Sent"), ("quarantine", "Quarantine")]
    public static let demoStates: Set<String> = ["demo-empty", "demo-loading", "demo-offline", "demo-error"]

    private let lock = NSLock()
    private var cache: [String: (rows: [MailSummary], total: Int, unread: Int)] = [:]
    private var drafts: [MailDraftInfo] = []
    private var aliasCache: String?
    private var htmlByID: [String: String] = [:]
    let environment: ChoirEnvironment
    public init(environment: ChoirEnvironment = .shared) { self.environment = environment }

    private func client() -> ChoirClient { environment.client() }
    private func cached(_ f: String) -> (rows: [MailSummary], total: Int, unread: Int)? { lock.lock(); defer { lock.unlock() }; return cache[f] }
    private func store(_ f: String, _ v: (rows: [MailSummary], total: Int, unread: Int)) { lock.lock(); cache[f] = v; lock.unlock() }

    // Plain words for a failure.
    public static func words(_ e: Error) -> (status: String, message: String) {
        if let c = e as? ChoirError {
            switch c {
            case .transport, .timeout: return ("offline", "You are offline, or your computer is not answering.")
            case .notSignedIn, .unauthorized: return ("error", "Sign in again to read your mail.")
            case .forbidden: return ("error", "This phone is not allowed to do that yet.")
            case .rateLimited: return ("error", "Too many requests. Try again in a moment.")
            case .server(let s, let m):
                if s == 404 { return ("error", "This message is gone.") }
                return ("error", m.isEmpty ? "Your computer could not do that (\(s))." : m)
            default: return ("error", "Something unexpected came back from your computer.")
            }
        }
        if let u = e as? URLError, [.notConnectedToInternet, .networkConnectionLost, .timedOut, .cannotConnectToHost].contains(u.code) {
            return ("offline", "You are offline, or your computer is not answering.")
        }
        return ("error", "Could not reach your mail.")
    }

    // MARK: Mailbox

    public func box(address: String, now: Date = Date()) async -> MailBoxView {
        var v = MailBoxView()
        let folder = PageAddress.rest(of: address)
        guard address.hasPrefix("mailbox:") else { v.status = "idle"; return v }
        v.folder = folder.isEmpty ? "inbox" : folder
        v.title = Self.folders.first { $0.name == v.folder }?.label ?? "Mail"
        v.folders = Self.folders.map { f in
            var r = MailFolderRow(); r.name = f.name; r.label = f.label; r.address = "mailbox:" + f.name; r.current = f.name == v.folder
            if f.name == "inbox", let c = cached("inbox"), c.unread > 0 { r.badge = "\(c.unread)" }
            return r
        }
        if Self.demoStates.contains(v.folder) { return await demoState(v) }
        do {
            if v.folder == "drafts" {
                let o = try await client().send("GET", "/api/email/drafts", route: "/api/email/drafts").object()
                let ds = (o["drafts"] as? [[String: Any]] ?? []).map(MailParse.draft)
                lock.lock(); drafts = ds; lock.unlock()
                v.rows = ds.map { d in
                    var r = MailRow(); r.id = d.id; r.address = "mail:draft/" + d.id; r.sender = "To " + (d.to.first ?? "(nobody yet)")
                    r.subject = d.subject.isEmpty ? "(no subject)" : d.subject; r.snippet = String(d.text.prefix(120)); r.direction = "draft"; return r }
                v.total = ds.count
            } else {
                let o = try await client().send("GET", "/api/email/messages", query: ["folder": v.folder], route: "/api/email/messages").object()
                let l = MailParse.list(o)
                store(v.folder, l)
                v.rows = MailThreads.rows(l.rows, now: now); v.total = l.total; v.unread = l.unread
                if v.folder == "inbox" { for i in v.folders.indices where v.folders[i].name == "inbox" { v.folders[i].badge = l.unread > 0 ? "\(l.unread)" : "" } }
            }
            if v.rows.isEmpty { v.status = "empty"; v.message = Self.emptyWords(v.folder) } else { v.status = "ready" }
        } catch is CancellationError {
            v.status = ""
        } catch {
            let w = Self.words(error)
            v.status = w.status; v.message = w.message
            if let c = cached(v.folder) {   // failure 3: keep the last good rows under the banner
                v.rows = MailThreads.rows(c.rows, now: now); v.total = c.total; v.unread = c.unread; v.cached = true
            }
        }
        return v
    }

    static func emptyWords(_ folder: String) -> String {
        switch folder {
        case "inbox": return "Nothing in your inbox. New mail to your address will appear here."
        case "drafts": return "No drafts. Reply to a message, or start one, and it will be kept here."
        case "sent": return "Nothing sent yet."
        case "quarantine": return "Nothing held back. Mail that cannot be trusted waits here for you."
        default: return "Nothing here."
        }
    }

    /// Demo mode only: the four states a real mailbox can be in, as real screens.
    private func demoState(_ base: MailBoxView) async -> MailBoxView {
        var v = base
        switch v.folder {
        case "demo-empty": v.status = "empty"; v.message = Self.emptyWords("inbox")
        case "demo-offline":
            v.status = "offline"; v.message = "You are offline, or your computer is not answering."
            if let c = cached("inbox") { v.rows = MailThreads.rows(c.rows); v.cached = true }
        case "demo-error": v.status = "error"; v.message = "Your computer could not list this folder (500)."
        default:
            try? await Task.sleep(nanoseconds: 4_000_000_000)
            v.status = "empty"; v.message = Self.emptyWords("inbox")
        }
        v.title = "Mail"
        return v
    }

    // MARK: A message

    private func allCached() -> [MailSummary] { lock.lock(); defer { lock.unlock() }; return cache.values.flatMap(\.rows) }

    public func message(address: String, now: Date = Date()) async -> MailMessageView {
        var v = MailMessageView()
        guard address.hasPrefix("mail:") else { v.status = "idle"; return v }
        let rest = PageAddress.rest(of: address)
        guard !rest.isEmpty, rest != "new", !rest.hasPrefix("reply/"), !rest.hasPrefix("draft/") else { v.status = "idle"; return v }
        v.id = rest; v.address = address
        do {
            let o = try await client().send("GET", "/api/email/messages/" + rest.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed)!,
                                            route: "/api/email/messages/{id}").object()
            let d = MailParse.detail(o)
            if cached("inbox") == nil { if let l = try? await client().send("GET", "/api/email/messages", query: ["folder": "inbox"], route: "/api/email/messages").object() { store("inbox", MailParse.list(l)) } }
            let s = d.summary
            v.subject = s.subject.isEmpty ? "(no subject)" : s.subject
            v.sender = s.from.isEmpty ? "(unknown sender)" : s.from; v.fromAddress = s.fromAddress
            v.toLine = d.to.joined(separator: ", "); v.ccLine = d.cc.joined(separator: ", ")
            v.date = MailDates.long(s.at); v.unread = s.unread
            v.trust = s.trust
            v.trustNote = s.trust == "quarantined" ? "Held back: this sender could not be trusted. Nothing in it was run or loaded."
                        : (s.trust == "unverified" ? "The sender could not be verified." : "")
            let b = MailText.blocks(text: d.text, html: d.html)
            lock.lock(); htmlByID[rest] = d.html; lock.unlock()
            v.blocks = b.blocks; v.htmlOnly = b.htmlOnly; v.cut = b.cut; v.hasQuoted = MailText.hasQuoted(b.blocks)
            v.hasOriginal = !d.html.isEmpty
            v.originalPath = v.hasOriginal ? "app:/cache/mail-" + Self.fileKey(rest) + ".html" : ""
            v.attachments = d.attachments.map { a in
                var r = MailAttachmentRow(); r.id = a.id; r.filename = MailText.safeFilename(a.filename); r.size = MailText.fileSize(a.size)
                r.kind = MailText.attachmentKind(filename: a.filename, contentType: a.contentType)
                // Failure 9: the web does not offer an inbound download either, so the row is honest about it. In demo mode the
                // attachments are bundled files that lane 5's viewers open by address.
                r.available = Demo.isOn
                r.address = r.available ? "file:/Mail/" + (s.id) + "/" + r.filename : ""
                r.note = r.available ? "" : "Saving attachments from mail is not offered yet."
                return r }
            v.thread = MailThreads.thread(of: s, in: allCached()).map { t in
                var r = MailThreadRow(); r.id = t.id; r.address = "mail:" + t.id; r.sender = t.from; r.snippet = t.snippet
                r.stamp = MailDates.short(t.at, now: now); r.current = t.id == s.id; return r }
            v.canReply = (s.direction != "outbound" || !d.to.isEmpty) && s.trust != "quarantined"
            let who = s.from.components(separatedBy: " <").first ?? s.from
            v.replyLabel = "Reply to " + (who.isEmpty ? "sender" : who)
            v.spoken = MailText.spoken(from: v.sender, subject: v.subject, blocks: v.blocks)
            v.status = "ready"
            if s.unread { Task { await self.setRead(id: rest, read: true) } }
        } catch is CancellationError {
            v.status = ""
        } catch {
            let w = Self.words(error); v.status = w.status; v.message = w.message
        }
        return v
    }

    /// The sanitised original document for a message read earlier in this session ("Show original").
    public func originalDocument(id: String) -> String? {
        lock.lock(); defer { lock.unlock() }
        guard let h = htmlByID[id], !h.isEmpty else { return nil }
        return MailHTML.sanitized(h)
    }

    public static func fileKey(_ id: String) -> String { String(id.unicodeScalars.map { CharacterSet.alphanumerics.contains($0) || $0 == "-" ? Character($0) : "_" }.prefix(64)) }

    @discardableResult
    public func setRead(id: String, read: Bool) async -> Bool {
        do {
            try await client().send("POST", "/api/email/messages/\(id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id)/\(read ? "read" : "unread")",
                                    route: "/api/email/messages/{id}/read")
            lock.lock()
            for k in cache.keys { for i in cache[k]!.rows.indices where cache[k]!.rows[i].id == id {
                if read && cache[k]!.rows[i].readAt.isEmpty { cache[k]!.rows[i].readAt = "read"; cache[k]!.unread = max(0, cache[k]!.unread - 1) } } }
            lock.unlock()
            return true
        } catch { return false }
    }

    // MARK: Compose (reply, new, an existing draft)

    func aliasAddress() async throws -> String {
        lock.lock(); let c = aliasCache; lock.unlock()
        if let c { return c }
        let o = try await client().send("GET", "/api/email/aliases", route: "/api/email/aliases").object()
        guard let a = (o["aliases"] as? [[String: Any]])?.first?["address"] as? String, !a.isEmpty else {
            throw ChoirError.server(status: 409, message: "You have no sending address yet.")
        }
        lock.lock(); aliasCache = a; lock.unlock()
        return a
    }

    public func compose(address: String, now: Date = Date()) async -> MailComposeView {
        var v = MailComposeView()
        guard address.hasPrefix("mail:") else { v.status = "idle"; return v }
        let rest = PageAddress.rest(of: address)
        v.forAddress = address
        if rest == "new" { v.status = "seed"; return v }
        if rest.hasPrefix("draft/") {
            let id = String(rest.dropFirst(6))
            do {
                let o = try await client().send("GET", "/api/email/drafts/\(id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id)", route: "/api/email/drafts/{id}").object()
                let d = MailParse.draft(o)
                v.status = "saved"; v.draftId = d.id; v.sendAs = d.from; v.toLine = d.to.joined(separator: ", "); v.subject = d.subject; v.body = d.text
                v.note = "Saved. Sending needs your approval."
            } catch { let w = Self.words(error); v.status = "error"; v.note = w.message }
            return v
        }
        guard rest.hasPrefix("reply/") else { v.status = "idle"; return v }
        let id = String(rest.dropFirst(6))
        let m = await message(address: "mail:" + id, now: now)
        guard m.status == "ready" else { v.status = "error"; v.note = m.message.isEmpty ? "Could not open the message to reply to." : m.message; return v }
        do {
            let o = try await client().send("GET", "/api/email/messages/\(id.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? id)", route: "/api/email/messages/{id}").object()
            let d = MailParse.detail(o)
            let target: String
            if d.summary.direction == "outbound" { target = MailText.addressOnly(d.to.first ?? "") }
            else { target = d.replyTo.isEmpty ? d.summary.fromAddress : d.replyTo }
            v.toLine = target
            v.subject = MailText.replySubject(d.summary.subject)
            v.body = MailText.replyBody(date: m.date, who: m.sender, blocks: m.blocks)
            v.replyTo = id
            v.status = "seed"
        } catch { let w = Self.words(error); v.status = "error"; v.note = w.message }
        return v
    }

    /// Words if the draft cannot be saved as it is (failure 11), else nil.
    public static func problem(to: String, body: String) -> String? {
        let addrs = to.split(whereSeparator: { $0 == "," || $0 == ";" }).map { String($0).trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
        if addrs.isEmpty { return "Add who it is for." }
        if let bad = addrs.first(where: { !MailText.looksLikeAddress(MailText.addressOnly($0)) }) { return "Check the address: \(bad)" }
        let words = body.components(separatedBy: "\n").filter { !$0.hasPrefix(">") && !$0.hasPrefix("On ") }.joined().trimmingCharacters(in: .whitespacesAndNewlines)
        if words.isEmpty { return "Write something first." }
        return nil
    }

    public func saveDraft(address: String, to: String, subject: String, body: String) async -> MailComposeView {
        var v = MailComposeView()
        v.forAddress = address; v.toLine = to; v.subject = subject; v.body = body
        let rest = PageAddress.rest(of: address)
        if rest.hasPrefix("reply/") { v.replyTo = String(rest.dropFirst(6)) }
        if let p = Self.problem(to: to, body: body) { v.status = "error"; v.note = p; return v }
        do {
            let from = try await aliasAddress()
            v.sendAs = from
            var req: [String: Any] = ["from_address": from, "subject": subject, "text_body": body,
                                      "to_addresses": to.split(whereSeparator: { $0 == "," || $0 == ";" }).map { MailText.addressOnly(String($0)) }.filter { !$0.isEmpty }]
            if !v.replyTo.isEmpty { req["reply_to_message_id"] = v.replyTo; req["source_kind"] = "mail"; req["source_ref"] = v.replyTo }
            let o = try await client().postJSON("/api/email/drafts", req, route: "/api/email/drafts").object()
            let d = MailParse.draft(o)
            v.status = "saved"; v.draftId = d.id
            v.note = "Saved as a draft. It has not been sent."
        } catch { let w = Self.words(error); v.status = "error"; v.note = w.message }
        return v
    }

    /// Sending is an owner act the server does not accept from a key (POST /api/email/drafts/{id}/send answers 403,
    /// "owner session required"). Until a device-key owner act exists the phone's way is the approval email.
    public func approvalEmail(address: String, draft: String) async -> MailComposeView {
        var v = MailComposeView()
        v.forAddress = address; v.draftId = draft
        do {
            let o = try await client().postJSON("/api/email/drafts/\(draft.addingPercentEncoding(withAllowedCharacters: .urlPathAllowed) ?? draft)/approval-email", [:],
                                                route: "/api/email/drafts/{id}/approval-email").object()
            v.status = "approval"; v.reviewURL = (o["review_url"] as? String) ?? ""
            v.note = "An approval link was emailed to you. Open it in your browser to read the draft and send it."
        } catch {
            let w = Self.words(error); v.status = "saved"; v.note = w.message
            if let c = error as? ChoirError, case .forbidden = c { v.note = "This phone cannot ask for an approval link yet." }
        }
        return v
    }
}

/// The spine's page source for mail addresses: titles and plain lines for a card's preview.
public struct MailPageSource: PageSource {
    let service: MailService
    public init(service: MailService) { self.service = service }
    public func handles(_ address: String) -> Bool { address.hasPrefix("mail:") || address.hasPrefix("mailbox:") }
    public func load(_ address: String) async throws -> PageModel {
        if address.hasPrefix("mailbox:") {
            let b = await service.box(address: address)
            return PageModel(address: address, kind: "mailbox", title: b.title.isEmpty ? "Mail" : b.title, status: b.status == "" ? "loading" : (b.status == "ready" || b.status == "empty" ? "ready" : b.status),
                             lines: b.rows.prefix(5).map { "\($0.sender): \($0.subject)" }, message: b.message)
        }
        let rest = PageAddress.rest(of: address)
        if rest == "new" { return PageModel(address: address, kind: "mail", title: "New message", status: "ready") }
        if rest.hasPrefix("reply/") || rest.hasPrefix("draft/") { return PageModel(address: address, kind: "mail", title: rest.hasPrefix("reply/") ? "Reply" : "Draft", status: "ready") }
        let m = await service.message(address: address)
        return PageModel(address: address, kind: "mail", title: m.subject.isEmpty ? "Message" : m.subject, status: m.status == "" ? "loading" : (m.status == "ready" ? "ready" : m.status),
                         lines: m.blocks.filter { $0.kind == "p" }.prefix(3).map(\.text), message: m.message)
    }
}

// MARK: Demo routes (one entry point, called from DemoURLProtocol; fixtures under mobile/fixtures/demo/mail-*.json)

public enum MailDemo {
    /// Answers a mail route from the fixtures, or nil when the request is not a mail route.
    public static func answer(method: String, path: String, query: [URLQueryItem], body: [String: Any]) -> (Int, Data)? {
        guard path.hasPrefix("/api/email/") else { return nil }
        func q(_ n: String) -> String { query.first { $0.name == n }?.value ?? "" }
        func ok(_ name: String) -> (Int, Data) { Demo.fixture(name).map { (200, $0) } ?? (404, Data(#"{"error":"not found"}"#.utf8)) }
        let parts = path.dropFirst("/api/email/".count).split(separator: "/").map(String.init)
        switch (method, parts.first ?? "", parts.count) {
        case ("GET", "messages", 1):
            let f = q("folder").isEmpty ? "inbox" : q("folder")
            return ["inbox", "sent", "quarantine"].contains(f) ? ok("mail-" + f) : (400, Data(#"{"error":"unsupported folder"}"#.utf8))
        case ("GET", "messages", 2): return ok("mail-msg-" + parts[1])
        case ("POST", "messages", 3): return (200, Data("{}".utf8))
        case ("GET", "aliases", 1): return ok("mail-aliases")
        case ("GET", "drafts", 1): return ok("mail-drafts")
        case ("GET", "drafts", 2): return ok("mail-draft-" + parts[1])
        case ("POST", "drafts", 1):
            var d: [String: Any] = ["id": "demo-draft-new", "status": "draft", "version": 1]
            for k in ["from_address", "to_addresses", "subject", "text_body", "reply_to_message_id"] { d[k] = body[k] }
            return (201, (try? JSONSerialization.data(withJSONObject: d)) ?? Data())
        case ("POST", "drafts", 3) where parts[2] == "approval-email":
            return (200, Data(#"{"status":"sent","review_url":"https://choir.news/mail/review/demo","token_id":"demo"}"#.utf8))
        default: return (404, Data(#"{"error":"demo mode serves fixtures only"}"#.utf8))
        }
    }
}

/// What the radio is asked to read aloud. The engine has no "speak this text" entry yet (see the lane report), so the
/// request is held here for whoever owns the voice to take; the mail page shows that it was asked.
public final class MailReading: @unchecked Sendable {
    public static let shared = MailReading()
    private let lock = NSLock()
    private var _last: (address: String, text: String)?
    public var last: (address: String, text: String)? { lock.lock(); defer { lock.unlock() }; return _last }
    public func request(address: String, text: String) { lock.lock(); _last = (address, text); lock.unlock() }
}
