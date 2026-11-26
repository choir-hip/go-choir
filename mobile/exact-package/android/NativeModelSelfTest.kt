package choir.autoradio

import java.io.File
import kotlin.math.sqrt
import org.json.JSONArray
import org.json.JSONObject

/** Physical-proof seam. Shared TypeScript chooses every model and supplies every input. */
class NativeModelSelfTest(
    private val models: NativeModels,
    private val downloads: ModelDownloadManager,
    private val loadVad: (File) -> SileroVad = { SileroVad.load(it) },
) {
    fun facts(): JSONObject = proof(emptyList())

    fun ensure(id: String): JSONObject {
        downloads.download(id)
        return facts()
    }

    fun remove(id: String): JSONObject {
        if (models.modelId == id) models.unload()
        downloads.remove(id)
        return facts()
    }

    fun proof(args: JSONObject): JSONObject {
        val textId = args.getString("textModelId")
        val embeddingId = args.getString("embeddingModelId")
        val vadId = args.getString("vadModelId")
        val prompt = args.getString("prompt")
        val embeddingText = args.getString("embeddingText")
        val generateTokens = args.getInt("generateTokens")
        val samples = args.getJSONArray("vadSamples")
        val text = NativeModelCatalog.require(textId)
        val embedding = NativeModelCatalog.require(embeddingId)
        val vad = NativeModelCatalog.require(vadId)
        if (text.kind != NativeModelKind.TEXT || embedding.kind != NativeModelKind.EMBEDDING || vad.kind != NativeModelKind.VAD) {
            throw NativeModelException.InvalidInput("proof model ids do not match their jobs")
        }
        if (prompt.isEmpty() || embeddingText.isEmpty() || generateTokens !in 1..1_048_576) {
            throw NativeModelException.InvalidInput("proof inputs are empty or invalid")
        }
        if (samples.length() == 0 || samples.length() % SileroSpec.WINDOW_SAMPLES != 0) {
            throw NativeModelException.InvalidInput("VAD proof samples must be whole 512-sample windows")
        }
        val audio = FloatArray(samples.length()) { index ->
            val value = samples.getDouble(index).toFloat()
            if (!value.isFinite()) throw NativeModelException.InvalidInput("VAD proof samples must be finite")
            value
        }
        if (!downloads.isReady(textId) || !downloads.isReady(embeddingId) || !downloads.isReady(vadId)) {
            throw NativeModelException.Unavailable("one or more proof models are not downloaded and verified")
        }
        val measurements = mutableListOf<JSONObject>()
        val textStarted = System.nanoTime()
        withLlama(text) {
            val generated = models.generate(prompt, generateTokens, seed = 0, temperature = 0f)
            measurements += measurement(textId, "generation", textStarted).apply {
                put("text", generated.text)
                put("promptTokens", generated.promptTokens)
                put("generatedTokens", generated.generatedTokens)
                put("prefillMs", generated.prefillMs)
                put("firstTokenMs", generated.firstTokenMs)
                put("decodeMs", generated.decodeMs)
                put("tokensPerSecond", generated.tokensPerSecond)
            }
        }
        val embeddingStarted = System.nanoTime()
        withLlama(embedding) {
            val vector = models.embed(embeddingText)
            val norm = sqrt(vector.fold(0.0) { sum, value -> sum + value.toDouble() * value.toDouble() })
            if (vector.size != embedding.embeddingDimension || !norm.isFinite() || kotlin.math.abs(norm - 1.0) > 0.0001) {
                throw NativeModelException.NativeFailure("embedding proof did not return a 768-dimensional unit vector")
            }
            measurements += measurement(embeddingId, "embedding", embeddingStarted).apply {
                put("dimensions", vector.size)
                put("norm", norm)
            }
        }
        val vadStarted = System.nanoTime()
        loadVad(downloads.file(vadId, vad.files.single().name)).use { session ->
            var minimum = Float.POSITIVE_INFINITY
            var maximum = Float.NEGATIVE_INFINITY
            var total = 0.0
            var offset = 0
            while (offset < audio.size) {
                val probability = session.probability(audio.copyOfRange(offset, offset + SileroSpec.WINDOW_SAMPLES))
                minimum = minOf(minimum, probability)
                maximum = maxOf(maximum, probability)
                total += probability
                offset += SileroSpec.WINDOW_SAMPLES
            }
            val windows = audio.size / SileroSpec.WINDOW_SAMPLES
            measurements += measurement(vadId, "vad", vadStarted).apply {
                put("windows", windows)
                put("probabilityMin", minimum.toDouble())
                put("probabilityMean", total / windows)
                put("probabilityMax", maximum.toDouble())
            }
        }
        return proof(measurements)
    }

    private fun <T> withLlama(spec: NativeModelSpec, work: () -> T): T {
        models.load(spec, downloads.file(spec.id, spec.files.single().name))
        try { return work() }
        finally { models.unload() }
    }

    private fun measurement(modelId: String, kind: String, started: Long): JSONObject = JSONObject()
        .put("modelId", modelId)
        .put("kind", kind)
        .put("state", "ok")
        .put("milliseconds", (System.nanoTime() - started) / 1_000_000.0)
        .put("text", "")
        .put("promptTokens", 0)
        .put("generatedTokens", 0)
        .put("prefillMs", 0.0)
        .put("firstTokenMs", 0.0)
        .put("decodeMs", 0.0)
        .put("tokensPerSecond", 0.0)
        .put("dimensions", 0)
        .put("norm", 0.0)
        .put("windows", 0)
        .put("probabilityMin", 0.0)
        .put("probabilityMean", 0.0)
        .put("probabilityMax", 0.0)

    private fun proof(measurements: List<JSONObject>): JSONObject {
        val runtime = try { models.info() } catch (_: NativeModelException.Unavailable) {
            NativeRuntimeInfo("llama.cpp", "", "", 0, false)
        }
        val tts = try { models.ttsCapabilities() } catch (_: NativeModelException.Unavailable) {
            NativeTtsCapabilities(false, "tiny-voca", emptyList(), "compiled native model runtime is missing")
        }
        val readiness = JSONArray()
        for (spec in NativeModelCatalog.models) readiness.put(JSONObject()
            .put("id", spec.id)
            .put("kind", spec.kind.name.lowercase())
            .put("ready", downloads.isReady(spec.id))
            .put("bytes", spec.bytes))
        return JSONObject()
            .put("available", runtime.available)
            .put("runtime", JSONObject()
                .put("name", runtime.name)
                .put("version", runtime.version)
                .put("commit", runtime.commit)
                .put("abi", runtime.abi)
                .put("available", runtime.available))
            .put("readiness", readiness)
            .put("measurements", JSONArray(measurements))
            .put("ttsAvailable", tts.available)
            .put("message", if (tts.available) "Native runtime ready." else "Native runtime ready; Tiny Voca synthesis is unavailable.")
    }
}
