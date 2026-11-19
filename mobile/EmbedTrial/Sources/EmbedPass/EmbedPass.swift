import Foundation

// The first embedding pass (station V3): developer tooling, not app logic. It reads a line-delimited export of public
// text, embeds each row with EmbeddingGemma 2 (the caller supplies the embedder), cuts to 256 dimensions, re-normalises,
// and writes one line per row with the vector as 256 float16 values. Format and rules: docs/reference/EMBEDDINGGEMMA.md,
// "The first pass file". No limits, no pacing: the loop embeds as fast as the embedder answers.

public let passDimension = 256

/// One line of the export (`scripts/embed-pass-export.py`). `text` and `title` are the public text; nothing else is read.
public struct PassRow: Decodable {
    public var kind: String          // "story_passage" or "capture"
    public var id: String            // publication version id, or capture id
    public var ordinal: Int
    public var title: String
    public var text: String
    public var textSha: String       // sha256 of text: equal text is embedded once
    public var isPrivate: Bool?
    public var labels: [String]?
    enum CodingKeys: String, CodingKey { case kind, id, ordinal, title, text, textSha = "text_sha", isPrivate = "private", labels }

    public init(kind: String, id: String, ordinal: Int, title: String, text: String, textSha: String, isPrivate: Bool? = nil, labels: [String]? = nil) {
        self.kind = kind; self.id = id; self.ordinal = ordinal; self.title = title; self.text = text; self.textSha = textSha
        self.isPrivate = isPrivate; self.labels = labels
    }
    /// The identity of a row in the output (what resumption skips).
    public var key: String { "\(kind)\u{1f}\(id)\u{1f}\(ordinal)" }
    /// True for a row marked private by any means the export knows: the flag, or a label that starts "owner_private".
    /// Such a row is never embedded and never written.
    public var markedPrivate: Bool {
        if isPrivate == true { return true }
        return (labels ?? []).contains { $0.trimmingCharacters(in: .whitespaces).lowercased().hasPrefix("owner_private") }
    }
}

public enum PassPrompt {
    /// The card's titled document form: `title: <headline> | text: <body>`; no title is the card's "none".
    public static func document(title: String, text: String) -> String {
        let t = title.trimmingCharacters(in: .whitespacesAndNewlines)
        return "title: \(t.isEmpty ? "none" : t) | text: \(text)"
    }
}

public enum PassVector {
    public static func norm(_ v: [Float]) -> Float { sqrt(v.reduce(0) { $0 + $1 * $1 }) }
    /// Matryoshka: the first `dim` values, re-normalised to unit length. The model's 768 values are first
    /// normalised by the caller (the GGUF's pooled vector is not unit length).
    public static func truncate(_ v: [Float], dim: Int = passDimension) -> [Float] {
        let h = Array(v.prefix(dim)); let n = norm(h)
        return n > 0 ? h.map { $0 / n } : h
    }
    public static func f16Bytes(_ v: [Float]) -> Data {
        var d = Data(capacity: v.count * 2)
        for x in v { var h = Float16(x).bitPattern.littleEndian; withUnsafeBytes(of: &h) { d.append(contentsOf: $0) } }
        return d
    }
    public static func fromF16Bytes(_ d: Data) -> [Float] {
        stride(from: 0, to: d.count - 1, by: 2).map { Float(Float16(bitPattern: UInt16(d[d.startIndex + $0]) | UInt16(d[d.startIndex + $0 + 1]) << 8)) }
    }
}

/// One line of the output file. The writer and reader are the only place the format lives.
public struct PassVectorRow: Codable, Equatable {
    public var kind: String, id: String, ordinal: Int, textSha: String, model: String, dim: Int
    public var v: Data   // 256 little-endian float16 values (512 bytes); JSON carries it as base64
    enum CodingKeys: String, CodingKey { case kind, id, ordinal, textSha = "text_sha", model, dim, v }
    public var key: String { "\(kind)\u{1f}\(id)\u{1f}\(ordinal)" }
}

public enum PassFile {
    public static func encode(_ r: PassVectorRow) throws -> String {
        let e = JSONEncoder(); e.outputFormatting = [.sortedKeys, .withoutEscapingSlashes]
        return String(decoding: try e.encode(r), as: UTF8.self)
    }
    public static func decode(_ line: String) throws -> PassVectorRow { try JSONDecoder().decode(PassVectorRow.self, from: Data(line.utf8)) }
}

public struct PassProgress {
    public var done = 0, written = 0, skippedHeld = 0, reused = 0, refusedPrivate = 0, tokens = 0, seconds = 0.0
    public var tokensPerSecond: Double { seconds > 0 ? Double(tokens) / seconds : 0 }
}

public final class PassRunner {
    let model: String
    let embed: (String) throws -> (vec: [Float], tokens: Int)   // the whole prompt string in; the unit 768-vector out
    let sink: (String) throws -> Void                            // one output line, appended and flushed
    var held: Set<String> = []                                   // keys already written
    var byInput: [String: Data] = [:]                            // (text_sha, title) -> vector bytes written in THIS run: the model reads the title too
    public private(set) var progress = PassProgress()

    /// `existing` is the output file's current lines: what is held is skipped. Nothing is reused from them: a line has no title,
    /// and equal text under another title is another input (2,661 rows of the first file carried a vector made under another headline).
    /// A line from another model hash is an error, never mixed.
    public init(model: String, existing: [String], embed: @escaping (String) throws -> (vec: [Float], tokens: Int), sink: @escaping (String) throws -> Void) throws {
        self.model = model; self.embed = embed; self.sink = sink
        for l in existing where !l.isEmpty {
            guard let r = try? PassFile.decode(l) else { continue }   // a torn last line from a killed run is not a row; it is re-embedded
            guard r.model == model else { throw PassError.otherModel(r.model) }
            held.insert(r.key)
        }
    }

    /// Embeds one input row. Returns true when a line was written.
    @discardableResult
    public func process(_ row: PassRow) throws -> Bool {
        progress.done += 1
        if row.markedPrivate { progress.refusedPrivate += 1; return false }
        if held.contains(row.key) { progress.skippedHeld += 1; return false }
        let bytes: Data
        let inputKey = row.textSha + "\u{1}" + row.title.trimmingCharacters(in: .whitespacesAndNewlines)
        if let b = byInput[inputKey] { bytes = b; progress.reused += 1 }
        else {
            let t0 = DispatchTime.now().uptimeNanoseconds
            let r = try embed(PassPrompt.document(title: row.title, text: row.text))
            progress.seconds += Double(DispatchTime.now().uptimeNanoseconds - t0) / 1e9
            progress.tokens += r.tokens
            let n = PassVector.norm(r.vec)
            bytes = PassVector.f16Bytes(PassVector.truncate(n > 0 ? r.vec.map { $0 / n } : r.vec))
            byInput[inputKey] = bytes
        }
        try sink(try PassFile.encode(PassVectorRow(kind: row.kind, id: row.id, ordinal: row.ordinal, textSha: row.textSha, model: model, dim: passDimension, v: bytes)))
        held.insert(row.key); progress.written += 1
        return true
    }
}

public enum PassError: Error, CustomStringConvertible {
    case otherModel(String)
    public var description: String { switch self { case .otherModel(let m): return "the output file holds vectors of another model (\(m)); use a new file" } }
}
