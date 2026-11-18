import Foundation
import Accelerate

/// The vector store of the reference page: unit vectors, flat float16 file (id order = row order), brute-force cosine by Accelerate.
final class FlatStore {
    let dim: Int
    private(set) var rows = 0
    private var f16: [Float16]
    private var f32: [Float] = []
    init(dim: Int) { self.dim = dim; f16 = [] }
    init(dim: Int, f16 data: [Float16]) { self.dim = dim; f16 = data; rows = data.count / dim }

    /// Add a unit vector (cut to `dim` and re-normalised: the Matryoshka option).
    func add(_ v: [Float]) { let n = normalized(v, dim: dim); f16.append(contentsOf: n.map { Float16($0) }); rows += 1 }
    func write(to url: URL) throws { try f16.withUnsafeBytes { Data($0) }.write(to: url) }
    static func open(_ url: URL, dim: Int) throws -> FlatStore {
        let d = try Data(contentsOf: url, options: .alwaysMapped)
        let n = d.count / 2
        let a = d.withUnsafeBytes { Array($0.bindMemory(to: Float16.self).prefix(n)) }
        return FlatStore(dim: dim, f16: a)
    }
    var bytes: Int { f16.count * 2 }
    func makeF32() { f32 = f16.map { Float($0) } }

    /// Scan converting float16 blocks to float32 on the fly (the file stays half size in memory), then one sgemv per block.
    func search(_ q: [Float], k: Int) -> [(row: Int, score: Float)] {
        let block = 4096
        var buf = [Float](repeating: 0, count: block * dim)
        var out = [Float](repeating: 0, count: block)
        var best: [(Int, Float)] = []; var floor: Float = -2
        f16.withUnsafeBufferPointer { src in
            var r = 0
            while r < rows {
                let m = min(block, rows - r)
                buf.withUnsafeMutableBufferPointer { b in for i in 0..<(m * dim) { b[i] = Float(src[r * dim + i]) } }
                cblas_sgemv(CblasRowMajor, CblasNoTrans, Int32(m), Int32(dim), 1, buf, Int32(dim), q, 1, 0, &out, 1)
                for i in 0..<m where out[i] > floor { best.append((r + i, out[i])) }
                if best.count > 4 * k { best.sort { $0.1 > $1.1 }; best = Array(best.prefix(k)); floor = best.last!.1 }
                r += m
            }
        }
        best.sort { $0.1 > $1.1 }
        return best.prefix(k).map { (row: $0.0, score: $0.1) }
    }
    /// The same scan over a float32 copy held in memory (the speed ceiling).
    func searchF32(_ q: [Float], k: Int) -> [(row: Int, score: Float)] {
        if f32.isEmpty { makeF32() }
        var out = [Float](repeating: 0, count: rows)
        cblas_sgemv(CblasRowMajor, CblasNoTrans, Int32(rows), Int32(dim), 1, f32, Int32(dim), q, 1, 0, &out, 1)
        var idx = Array(0..<rows)
        let kk = min(k, rows)
        idx.withUnsafeMutableBufferPointer { _ in }
        let top = idx.sorted { out[$0] > out[$1] }.prefix(kk)
        return top.map { (row: $0, score: out[$0]) }
    }
}

enum Fusion {
    /// Reciprocal rank fusion (Cormack et al. 2009), k = 60, over ranked id lists.
    static func rrf(_ lists: [[String]], k: Double = 60) -> [String] {
        var s: [String: Double] = [:]
        for l in lists { for (i, id) in l.enumerated() { s[id, default: 0] += 1 / (k + Double(i + 1)) } }
        return s.sorted { $0.value != $1.value ? $0.value > $1.value : $0.key < $1.key }.map { $0.key }
    }
}

struct Metrics { var recall5 = 0.0, mrr = 0.0, hits = 0, n = 0
    mutating func add(_ ranked: [String], _ rel: Set<String>) {
        n += 1
        if let i = ranked.firstIndex(where: { rel.contains($0) }) { mrr += 1 / Double(i + 1); if i < 5 { recall5 += 1; hits += 1 } }
    }
    var line: String { String(format: "recall@5 %.3f (%d/%d)  MRR %.3f", recall5 / Double(max(n, 1)), hits, n, mrr / Double(max(n, 1))) }
}
