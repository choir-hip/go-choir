// Spike: takes the session only through Exact's claim, plays a tone, taps the mic, prints RMS.
import AVFoundation
import ExactPackage
import Foundation
import UIKit

public final class SpikeModule: ExactPackage {
    private let context: ExactPackageContext
    private var claim: ExactSubscription?
    private var engine: AVAudioEngine?
    private var player: AVAudioPlayerNode?
    private var state = "stopped"
    private var rms: Double = 0
    private var frames = 0
    private var lines: [String] = []
    private var lastPublish = Date.distantPast

    public init(context: ExactPackageContext) { self.context = context }

    private func note(_ s: String) {
        let route = AVAudioSession.sharedInstance().currentRoute
        let line = "\(s) cat=\(AVAudioSession.sharedInstance().category.rawValue) mode=\(AVAudioSession.sharedInstance().mode.rawValue) in=\(route.inputs.map { $0.portType.rawValue }) out=\(route.outputs.map { $0.portType.rawValue })"
        NSLog("SPIKE \(line)")
        context.log("SPIKE \(line)")
        lines.append(line); if lines.count > 6 { lines.removeFirst() }
        context.changed("spike.spikeStatus")
    }

    private var status: [String: Any] { ["state": state, "rms": rms, "frames": frames, "log": lines.joined(separator: "\n")] }

    public func later(_ name: String, _ args: [String: Any], preferred: Any?, reply: @escaping ([String: Any]) -> Void) {
        switch name {
        case "spikeStatus": reply(status)
        case "spikeMode":
            switch args["mode"] as? String ?? "stopped" {
            case "listening": start(talking: false)
            case "talking": start(talking: true)
            default: stop()
            }
            reply(status)
        default: reply(["error": "NotSupportedError"])
        }
    }

    private func stop() {
        player?.stop(); engine?.inputNode.removeTap(onBus: 0); engine?.stop()
        engine = nil; player = nil
        claim?.cancel(); claim = nil
        state = "stopped"; note("stopped")
    }

    private func start(talking: Bool) {
        stop()
        let t0 = Date()
        // listening: category from the claim, default mode, A2DP allowed (no HFP), speaker default.
        // talking: voiceChat mode, engine voice processing on.
        guard let c = context.audioSession.claim(
            mode: talking ? "voiceChat" : "default",
            options: talking ? ["defaultToSpeaker", "allowBluetoothHFP"] : ["defaultToSpeaker", "allowBluetoothA2DP", "mixWithOthers"],
            background: true,
            events: { [weak self] e in self?.note("event \(e)") }) else { note("claim refused"); return }
        claim = c
        note("claimed \(talking ? "talking" : "listening") in \(Int(Date().timeIntervalSince(t0) * 1000))ms")
        let engine = AVAudioEngine(), player = AVAudioPlayerNode()
        do {
            if talking { try engine.inputNode.setVoiceProcessingEnabled(true) }
            engine.attach(player)
            let fmt = AVAudioFormat(standardFormatWithSampleRate: 48000, channels: 1)!
            engine.connect(player, to: engine.mainMixerNode, format: fmt)
            let n = AVAudioFrameCount(48000)
            let buf = AVAudioPCMBuffer(pcmFormat: fmt, frameCapacity: n)!
            buf.frameLength = n
            for i in 0..<Int(n) { buf.floatChannelData![0][i] = 0.2 * sinf(2 * .pi * 440 * Float(i) / 48000) }
            let inFmt = engine.inputNode.outputFormat(forBus: 0)
            engine.inputNode.installTap(onBus: 0, bufferSize: 2048, format: inFmt) { [weak self] b, _ in
                guard let self, let d = b.floatChannelData?[0] else { return }
                var s: Float = 0; for i in 0..<Int(b.frameLength) { s += d[i] * d[i] }
                let r = Double(sqrt(s / Float(max(1, b.frameLength))))
                DispatchQueue.main.async {
                    self.rms = r; self.frames += Int(b.frameLength)
                    if Date().timeIntervalSince(self.lastPublish) > 1 {
                        self.lastPublish = Date()
                        NSLog("SPIKE rms=%.5f frames=%d state=%@ fg=%d", r, self.frames, self.state, UIApplication.shared.applicationState == .active ? 1 : 0)
                        self.context.changed("spike.spikeStatus")
                    }
                }
            }
            engine.prepare(); try engine.start()
            player.scheduleBuffer(buf, at: nil, options: .loops)
            player.play()
            self.engine = engine; self.player = player
            state = talking ? "talking" : "listening"
            note("running in=\(inFmt.sampleRate)Hz")
        } catch { note("engine failed \(error)"); claim?.cancel(); claim = nil }
    }
}
