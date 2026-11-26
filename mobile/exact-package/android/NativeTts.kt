package choir.autoradio

/** Tiny Voca F2/M2 seam. It refuses until real synthesis produces PCM/WAV bytes. */
class NativeTts(private val models: NativeModels) {
    fun capabilities(): NativeTtsCapabilities = models.ttsCapabilities()

    fun synthesize(text: String, voice: String): ByteArray {
        val capabilities = capabilities()
        if (!capabilities.available || voice !in capabilities.voices) {
            throw NativeModelException.Unavailable(capabilities.reason)
        }
        // No silent substitute and no fabricated audio: this method stays a
        // refusal until a real Tiny Voca Android runner is implemented.
        throw NativeModelException.Unavailable("Tiny Voca synthesis is not implemented on Android")
    }
}
