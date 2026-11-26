package choir.autoradio

/** The stable JNI surface implemented by native/native_models.cpp. No fallback loader exists. */
internal object NativeBridge {
    init { System.loadLibrary("autoradio_models") }

    fun interface TokenCallback { fun onToken(text: String) }

    external fun info(): String
    external fun load(path: String, kind: String, contextTokens: Int, threads: Int, batchSize: Int): Long
    external fun generate(handle: Long, prompt: String, maxTokens: Int, seed: Long, temperature: Float,
                          topK: Int, topP: Float, callback: TokenCallback?): String
    external fun embed(handle: Long, prompt: String): FloatArray
    external fun cancel(handle: Long)
    external fun unload(handle: Long)
    external fun ttsCapabilities(): String
}
