import Foundation
import AutoradioKit
import EmbeddingGemma2CoreAI

// embed-trial: EmbeddingGemma 2 on the Mac, both routes. See docs/reference/EMBEDDINGGEMMA.md.
//   embed-trial bench --route a|b [--gpu 0|1] [--threads 4] [--compute ane|gpu|cpu] [--tag NAME] [--corpus DIR] [--book FILE]
//   embed-trial quality TAG [TAG ...]       recall@5 and MRR for BM25, vectors, fusion (needs bench output for each TAG)
//   embed-trial compare TAG TAG             cosine of the same texts across two routes
//   embed-trial scan [TAG]                  vector store timings at the corpus size and at 50,000 synthetic vectors
var args = Array(CommandLine.arguments.dropFirst())
let cmd = args.isEmpty ? "help" : args.removeFirst()
func opt(_ k: String, _ d: String) -> String { if let i = args.firstIndex(of: "--\(k)"), i + 1 < args.count { return args[i + 1] }; return d }
let here = FileManager.default.currentDirectoryPath
let corpusDir = URL(fileURLWithPath: opt("corpus", "\(here)/corpus"))
let bookURL = URL(fileURLWithPath: opt("book", cacheDir.appendingPathComponent("alice.txt").path))
let outDir = cacheDir.appendingPathComponent("out")
try? FileManager.default.createDirectory(at: outDir, withIntermediateDirectories: true)

func line(_ s: String) { print(s); fflush(stdout) }
func readVecs(_ tag: String, dim: Int = 768) throws -> [[Float]] {
    let d = try Data(contentsOf: outDir.appendingPathComponent("vec-\(tag).f32"))
    let a = d.withUnsafeBytes { Array($0.bindMemory(to: Float.self)) }
    return stride(from: 0, to: a.count, by: dim).map { Array(a[$0..<$0 + dim]) }
}

func bench() async throws {
    let route = opt("route", "a"), tag = opt("tag", route)
    var (docs, qs, _) = try Corpus.load(dir: corpusDir, book: bookURL)
    if let lim = Int(opt("limit", "0")), lim > 0, lim < docs.count { docs = Array(docs.prefix(lim)) }   // a bounded timing run; the vectors file then holds only these
    line("[\(tag)] corpus: \(docs.count) documents (\(docs.filter { $0.id.hasPrefix("alice") }.count) book passages), \(qs.count) questions")
    let e: TrialEmbedder
    var loadMs = 0.0, loadWarmMs: Double? = nil, prewarmS: Double? = nil
    let before = currentFootprintMB()
    if route == "a" {
        let path = cacheDir.appendingPathComponent("gguf/embeddinggemma-2-Q8_0.gguf").path
        let gpu = opt("gpu", "1") == "1", th = Int32(opt("threads", "4"))!
        let l = try LlamaEmbedder(path: path, gpu: gpu, threads: th)
        loadMs = l.loadMs; e = l
        line("[\(tag)] loaded GGUF in \(f1(loadMs)) ms (gpu \(gpu), threads \(th)); n_embd_out \(l.dim); pooling \(l.poolingName)")
    } else {
        let c: ComputePreference = ["ane": .neuralEngine, "gpu": .gpu, "cpu": .cpu][opt("compute", "ane")] ?? .neuralEngine
        let bundle = cacheDir.appendingPathComponent("coreai/model")
        let l = try await CoreAIEmbedder(bundle: bundle, compute: c)
        loadMs = l.loadMs; e = l
        line("[\(tag)] loaded Core AI (\(c.rawValue)) in \(f1(loadMs)) ms (this includes specialisation when the cache is cold)")
        let t0 = nowMs(); _ = try await l.prewarm(); prewarmS = (nowMs() - t0) / 1000
        line("[\(tag)] prewarm \(f1(prewarmS! * 1000)) ms")
    }
    let afterLoad = currentFootprintMB()
    // smoke: dimension, unit length, paraphrase pair above an unrelated pair (sentence-similarity prompt, from the card)
    let sp = "task: sentence similarity | query: "
    let s1 = try await e.embed(sp + "A man is playing a guitar on stage in front of a large crowd.").vec
    let s2 = try await e.embed(sp + "Someone performs music with a stringed instrument for an audience.").vec
    let s3 = try await e.embed(sp + "The quarterly tax filing deadline has been moved to the end of March.").vec
    line("[\(tag)] smoke: dim \(s1.count), norm \(String(format: "%.5f", norm(s1))), cos(paraphrase) \(String(format: "%.4f", dot(s1, s2))), cos(unrelated) \(String(format: "%.4f", dot(s1, s3))) -> \(dot(s1, s2) > dot(s1, s3) ? "PASS" : "FAIL")")
    // batch: every document, one at a time (the route's own pace), in corpus order
    var vecs: [[Float]] = []; var tokens = 0
    var perDoc: [Double] = []
    let t0 = nowMs()
    for d in docs {
        let t = nowMs()
        let r = try await e.embed(documentText(title: d.title, body: d.body))
        perDoc.append(nowMs() - t); tokens += r.tokens; vecs.append(r.vec)
    }
    let batchMs = nowMs() - t0
    var qv: [[Float]] = []; var qms: [Double] = []
    for q in qs { let t = nowMs(); qv.append(try await e.embed(queryPrefix + q.q).vec); qms.append(nowMs() - t) }
    // single query again, after the batch (warm), 30 repeats of the same question
    var rep: [Double] = []
    for _ in 0..<30 { let t = nowMs(); _ = try await e.embed(queryPrefix + qs[0].q); rep.append(nowMs() - t) }
    let all = vecs + qv
    var bytes = Data()
    for v in all { v.withUnsafeBytes { bytes.append(contentsOf: $0) } }
    try bytes.write(to: outDir.appendingPathComponent("vec-\(tag).f32"))
    let norms = all.map { norm($0) }
    let mem = peakMemoryMB()
    if route == "a", let l = e as? LlamaEmbedder {   // a second load in this process: the warm-cache load
        let g = opt("gpu", "1") == "1", th = Int32(opt("threads", "4"))!
        let t = nowMs(); let l2 = try LlamaEmbedder(path: cacheDir.appendingPathComponent("gguf/embeddinggemma-2-Q8_0.gguf").path, gpu: g, threads: th)
        loadWarmMs = nowMs() - t; _ = l; _ = l2
    }
    let n = Double(docs.count)
    let res: [String: Any] = [
        "tag": tag, "route": e.name, "docs": docs.count, "tokens": tokens, "batchSeconds": batchMs / 1000,
        "passagesPerSecond": n / (batchMs / 1000), "tokensPerSecond": Double(tokens) / (batchMs / 1000),
        "docMsP50": percentile(perDoc, 0.5), "docMsP95": percentile(perDoc, 0.95),
        "queryMsP50": percentile(rep, 0.5), "queryMsP95": percentile(rep, 0.95), "queryMsFirst": qms.first ?? 0,
        "loadMs": loadMs, "loadWarmMsSameProcess": loadWarmMs as Any, "prewarmSeconds": prewarmS as Any,
        "footprintBeforeLoadMB": before, "footprintAfterLoadMB": afterLoad, "peakFootprintMB": mem.footprint, "peakResidentMB": mem.resident,
        "normMin": norms.min() ?? 0, "normMax": norms.max() ?? 0, "rawNormMin": (e as? LlamaEmbedder)?.rawNormMin as Any, "rawNormMax": (e as? LlamaEmbedder)?.rawNormMax as Any, "dim": all.first?.count ?? 0,
        "os": ProcessInfo.processInfo.operatingSystemVersionString, "date": ISO8601DateFormatter().string(from: Date()),
    ]
    let j = try JSONSerialization.data(withJSONObject: res, options: [.prettyPrinted, .sortedKeys])
    try j.write(to: outDir.appendingPathComponent("bench-\(tag).json"))
    line(String(data: j, encoding: .utf8)!)
}

func quality() async throws {
    let (docs, qs, rel) = try Corpus.load(dir: corpusDir, book: bookURL)
    let idx = Corpus.index(docs)
    let ids = docs.map { $0.id }
    var table: [(String, Metrics, Metrics, Metrics)] = []   // name, all, exact, para
    func record(_ name: String, _ rankings: [[String]]) {
        var a = Metrics(), e = Metrics(), p = Metrics()
        for (i, q) in qs.enumerated() { a.add(rankings[i], rel[i]); if q.kind == "exact" { e.add(rankings[i], rel[i]) } else { p.add(rankings[i], rel[i]) } }
        table.append((name, a, e, p))
    }
    let bm = qs.map { idx.bm25Ranking($0.q, limit: 50).map { $0.id } }
    record("BM25 alone", bm)
    for tag in args {
        let v = try readVecs(tag); let dv = Array(v.prefix(docs.count)), qv = Array(v.suffix(qs.count))
        for dim in [768, 256] {
            let store = FlatStore(dim: dim); for x in dv { store.add(x) }
            let vr = qv.map { q in store.search(normalized(q, dim: dim), k: 50).map { ids[$0.row] } }
            record("vectors \(tag) \(dim)d", vr)
            record("BM25 + vectors \(tag) \(dim)d, RRF k=60", (0..<qs.count).map { Fusion.rrf([bm[$0], vr[$0]]) })
        }
    }
    line("method | all 30 | exact-name 13 | paraphrase 17")
    for (n, a, e, p) in table { line("\(n) | \(a.line) | \(e.line) | \(p.line)") }
    line("\nper question (rank of the first right answer; BM25 / each vector run at 768d):")
    for (i, q) in qs.enumerated() {
        func rk(_ l: [String]) -> String { l.firstIndex(where: { rel[i].contains($0) }).map { "\($0 + 1)" } ?? "-" }
        var s = "\(i + 1). [\(q.kind)] bm25 \(rk(bm[i]))"
        for tag in args {
            let v = try readVecs(tag); let store = FlatStore(dim: 768); for x in v.prefix(docs.count) { store.add(x) }
            s += "  \(tag) \(rk(store.search(v[docs.count + i], k: 50).map { ids[$0.row] }))"
        }
        line(s + "  | \(q.q)")
    }
}

func compare() throws {
    let a = try readVecs(args[0]), b = try readVecs(args[1])
    let c = zip(a, b).map { Double(dot($0, $1) / (norm($0) * norm($1))) }
    var c256: [Double] = zip(a, b).map { Double(dot(normalized($0, dim: 256), normalized($1, dim: 256))) }
    line("same texts, \(args[0]) vs \(args[1]): \(c.count) vectors, cosine min \(String(format: "%.5f", c.min()!)) mean \(String(format: "%.5f", c.reduce(0, +) / Double(c.count))) p5 \(String(format: "%.5f", percentile(c, 0.05)))")
    line("   at 256d: min \(String(format: "%.5f", c256.min()!)) mean \(String(format: "%.5f", c256.reduce(0, +) / Double(c256.count)))")
    c256.removeAll()
}

func scan() throws {
    let dimList = [768, 256]
    func random(_ n: Int, _ dim: Int) -> [[Float]] {
        var g = SystemRandomNumberGenerator()
        return (0..<n).map { _ in normalized((0..<768).map { _ in Float.random(in: -1...1, using: &g) }, dim: dim) }
    }
    for dim in dimList {
        for n in [330, 50_000] {
            let store = FlatStore(dim: dim)
            let vs = random(n, dim)
            for v in vs { store.add(v) }
            let file = outDir.appendingPathComponent("scan-\(dim)-\(n).f16"); try store.write(to: file)
            let mapped = try FlatStore.open(file, dim: dim)
            let q = vs[n / 2]
            var t16: [Double] = [], t32: [Double] = []
            _ = mapped.search(q, k: 50); _ = mapped.searchF32(q, k: 50)
            for _ in 0..<30 { var t = nowMs(); _ = mapped.search(q, k: 50); t16.append(nowMs() - t); t = nowMs(); _ = mapped.searchF32(q, k: 50); t32.append(nowMs() - t) }
            let top = mapped.search(q, k: 1).first
            line("dim \(dim), \(n) vectors, \(mapped.bytes / 1024) KiB f16 file: scan f16->f32 blocks p50 \(String(format: "%.3f", percentile(t16, 0.5))) ms p95 \(String(format: "%.3f", percentile(t16, 0.95))) ms | f32 in memory p50 \(String(format: "%.3f", percentile(t32, 0.5))) ms | self-match row \(top?.row ?? -1) (expect \(n / 2)) score \(String(format: "%.4f", top?.score ?? 0))")
        }
    }
}

switch cmd {
case "bench": try await bench()
case "quality": try await quality()
case "compare": try compare()
case "scan": try scan()
default: line("usage: embed-trial bench|quality|compare|scan (see the header of main.swift)")
}
