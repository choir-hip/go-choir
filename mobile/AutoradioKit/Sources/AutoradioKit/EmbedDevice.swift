import Foundation
import Network

// THE EMBEDDING MODEL OF THE PHONE, as three native answers. Why native: the model is llama.cpp on the phone's processor, the radio holds that
// processor, and Wi-Fi, Low Power Mode and the thermal state are the platform's. Everything else about the vectors (which items to embed, the
// store on disk, the job's slices, saving, the fusion with BM25) is shared code (AutoradioLab/retrieval-run.ts); it asks through the app
// module's ops (modules/apple/ModelsEntry.swift):
//   embedState   {present, loaded, onWiFi, lowPower, thermal, radioBusy}   what shared code needs to decide to start, wait or stop
//   embedEnsure  {download}  -> {ok}   fetch the model once (shared code only asks to download on Wi-Fi), check it (ModelStore), load it
//   embed        {prompts}   -> {vectors} | {error}   finished prompt strings in, one unit vector each out
// Logs carry counts only.

/// What the radio holds the processor for: speech on air, speech being synthesised, remote audio.
public final class RadioDemand: @unchecked Sendable {
    public static let shared = RadioDemand()
    private let lock = NSLock()
    private var sources: [@Sendable () -> Bool] = []
    private var remote = false
    public init() {}
    /// Remote audio (a podcast, an Autopaper clip) is playing: set by the host from the main actor, read from any thread.
    public func setRemotePlaying(_ on: Bool) { lock.lock(); remote = on; lock.unlock() }
    public func add(_ source: @escaping @Sendable () -> Bool) { lock.lock(); sources.append(source); lock.unlock() }
    public var busy: Bool { lock.lock(); let s = sources, r = remote; lock.unlock(); return r || s.contains { $0() } }
}

/// Tells whether the phone is on Wi-Fi (or wired): no model is fetched over cellular without the person's mobile-data press. The ONE source of the path for every download: the device profile reads it (`DeviceProfile.current`), the search pass reads it.
public final class WiFiMonitor: @unchecked Sendable {
    public static let shared = WiFiMonitor()
    private let monitor = NWPathMonitor()
    private let lock = NSLock()
    private var wifi = false
    /// Tests only: a path to pretend.
    public var testOverride: Bool?
    /// The path changed (shared code asks the facts again, so a page rendered on Wi-Fi does not press on mobile data).
    public var onChange: (@Sendable () -> Void)?
    /// A monitor that watches nothing (tests drive `report`).
    init(testing: Bool) {}
    private init() {
        monitor.pathUpdateHandler = { [weak self] p in
            self?.report(wifi: Self.classify(satisfied: p.status == .satisfied, wifi: p.usesInterfaceType(.wifi), wired: p.usesInterfaceType(.wiredEthernet), expensive: p.isExpensive))
        }
        monitor.start(queue: DispatchQueue(label: "embed.wifi", qos: .utility))
    }
    /// Pure: a connection that works, over Wi-Fi or a cable, and not a metered one (a personal hotspot is mobile data).
    public static func classify(satisfied: Bool, wifi: Bool, wired: Bool, expensive: Bool) -> Bool { satisfied && (wifi || wired) && !expensive }
    func report(wifi w: Bool) {
        lock.lock(); let changed = wifi != w; wifi = w; let cb = onChange; lock.unlock()
        if changed { cb?(); if self === WiFiMonitor.shared { DownloadDoor.shared.pathChanged(onWiFi: w) } }
    }
    public var onWiFi: Bool { lock.lock(); defer { lock.unlock() }; return testOverride ?? wifi }
}

public final class EmbedDevice: @unchecked Sendable {
    public static let shared = EmbedDevice()
    /// Loads the model (the models module sets this; the kit has no llama.cpp).
    nonisolated(unsafe) public static var loader: @Sendable () async throws -> any Embedder & QueryEmbedding = { throw EmbedderError.unavailable("no embedding runtime in this build") }
    private let lock = NSLock()
    private var embedder: (any Embedder & QueryEmbedding)?
    private var loading: Task<Bool, Never>?
    public init() {}
    private var current: (any Embedder & QueryEmbedding)? { lock.lock(); defer { lock.unlock() }; return embedder }

    public func state() async -> [String: Any] {
        let present = await ModelStore.shared.localFileIfPresent(KnownModels.embeddingGemma2.id) != nil
        let p = ProcessInfo.processInfo
        return ["id": KnownModels.embeddingGemma2.id, "bytes": KnownModels.embeddingGemma2.bytes, "present": present, "loaded": current != nil, "onWiFi": WiFiMonitor.shared.onWiFi, "lowPower": p.isLowPowerModeEnabled,
                "thermal": p.thermalState.rawValue >= ProcessInfo.ThermalState.serious.rawValue, "radioBusy": RadioDemand.shared.busy]
    }
    /// One load at a time: a second caller that arrives while the first is loading waits for the same load (two llama loads at once would hold twice the memory).
    public func ensure(download: Bool) async -> Bool {
        if current != nil { return true }
        lock.lock()
        if let t = loading { lock.unlock(); return await t.value }
        let t = Task { await self.load(download: download) }
        loading = t; lock.unlock()
        let ok = await t.value
        lock.lock(); loading = nil; lock.unlock()
        return ok
    }
    private func load(download: Bool) async -> Bool {
        if download, await ModelStore.shared.localFileIfPresent(KnownModels.embeddingGemma2.id) == nil {
            do { _ = try await ModelStore.shared.ensure(KnownModels.embeddingGemma2.id) } catch { fputs("AUTORADIO embed model=absent (download failed)\n", stderr); return false }
        }
        do { let e = try await Self.loader(); lock.lock(); embedder = e; lock.unlock(); fputs("AUTORADIO embed model=ok\n", stderr); return true }
        catch { fputs("AUTORADIO embed model=absent (load failed)\n", stderr); return false }
    }
    public func embed(_ prompts: [String]) async throws -> [[Float]] {
        guard let e = current else { throw EmbedderError.unavailable("the model is not loaded") }
        return try await e.embed(prompts)
    }
    /// Sign-out leaves the model (it is not the account's).
    public static func op(_ r: [String: Any]) async -> [String: Any] {
        switch r["op"] as? String ?? "" {
        case "embedState": return await shared.state()
        case "embedEnsure": return ["ok": await shared.ensure(download: r["download"] as? Bool ?? false)]
        case "embed":
            do { return ["vectors": try await shared.embed(r["prompts"] as? [String] ?? []).map { $0.map { Double($0) } }] }
            catch { return ["error": "\(error)"] }
        default: return [:]
        }
    }
}
