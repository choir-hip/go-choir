import Foundation
import AutoradioKit

struct TrialDoc { var id: String; var title: String; var body: String; var date: Date? }
struct TrialQuestion: Decodable { var q: String; var kind: String; var source: String; var answer_ids: [String]?; var answer_phrases: [String]? }
private struct StoryRow: Decodable { var id: String; var h: String; var b: String; var date: String }

enum Corpus {
    static func load(dir: URL, book: URL) throws -> (docs: [TrialDoc], questions: [TrialQuestion], relevant: [Set<String>]) {
        let rows = try JSONDecoder().decode([StoryRow].self, from: Data(contentsOf: dir.appendingPathComponent("stories.json")))
        let f = DateFormatter(); f.dateFormat = "yyyy-MM-dd"; f.timeZone = TimeZone(identifier: "UTC")
        var docs = rows.map { TrialDoc(id: $0.id, title: $0.h, body: $0.b, date: f.date(from: $0.date)) }
        // The second set (2026-11-17): 26 more stories (20 answers, 6 decoys that share a name) and 26 more questions (6 cannot be answered).
        if let d = try? Data(contentsOf: dir.appendingPathComponent("stories-extra.json")), let more = try? JSONDecoder().decode([StoryRow].self, from: d) {
            docs += more.map { TrialDoc(id: $0.id, title: $0.h, body: $0.b, date: f.date(from: $0.date)) }
        }
        // Alice's Adventures in Wonderland, Project Gutenberg #11 (public domain): the text between the START and END markers,
        // curly quotes made straight, italics underscores dropped, split into passages of about 180 words.
        var t = try String(contentsOf: book, encoding: .utf8)
        if let a = t.range(of: "*** START OF"), let nl = t[a.upperBound...].firstIndex(of: "\n") { t = String(t[t.index(after: nl)...]) }
        if let e = t.range(of: "*** END OF") { t = String(t[..<e.lowerBound]) }
        t = t.replacingOccurrences(of: "\u{2019}", with: "'").replacingOccurrences(of: "\u{2018}", with: "'")
            .replacingOccurrences(of: "\u{201C}", with: "\"").replacingOccurrences(of: "\u{201D}", with: "\"").replacingOccurrences(of: "_", with: "")
            .replacingOccurrences(of: "\r", with: "")
        let paras = t.components(separatedBy: "\n\n").map { $0.split(whereSeparator: { $0.isWhitespace }).joined(separator: " ") }.filter { !$0.isEmpty }
        var cur = "", n = 0
        var pieces: [String] = []
        for p in paras {
            cur += (cur.isEmpty ? "" : " ") + p
            if cur.split(separator: " ").count >= 150 { pieces.append(cur); cur = "" }
        }
        if !cur.isEmpty { pieces.append(cur) }
        for p in pieces { n += 1; docs.append(TrialDoc(id: String(format: "alice#%04d", n), title: "Alice's Adventures in Wonderland", body: p, date: nil)) }
        var qs = try JSONDecoder().decode([TrialQuestion].self, from: Data(contentsOf: dir.appendingPathComponent("questions.json")))
        if let d = try? Data(contentsOf: dir.appendingPathComponent("questions-extra.json")), let more = try? JSONDecoder().decode([TrialQuestion].self, from: d) { qs += more }
        let rel: [Set<String>] = qs.map { q in
            if let ids = q.answer_ids { return Set(ids) }
            let ph = (q.answer_phrases ?? []).map { $0.lowercased() }
            return Set(docs.filter { d in d.id.hasPrefix("alice#") && ph.contains(where: { d.body.lowercased().contains($0) }) }.map { $0.id })
        }
        for (i, r) in rel.enumerated() where r.isEmpty && qs[i].kind != "unanswerable" { throw NSError(domain: "corpus", code: 1, userInfo: [NSLocalizedDescriptionKey: "question \(i) has no answer passage"]) }
        return (docs, qs, rel)
    }
    static func index(_ docs: [TrialDoc]) -> RetrievalIndex {
        let idx = RetrievalIndex()
        idx.upsertAll(docs.map { RetrievalDoc(id: $0.id, kind: $0.id.hasPrefix("alice") ? "book" : "story", title: $0.title, text: $0.body, date: $0.date) })
        return idx
    }
}
