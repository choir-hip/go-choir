package choir.autoradio

import java.io.File
import java.util.concurrent.ExecutionException
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference
import org.json.JSONException
import org.json.JSONObject

sealed class NativeModelException(message: String, val code: String) : Exception(message) {
    class InvalidInput(message: String) : NativeModelException(message, "INVALID_INPUT")
    class Unavailable(message: String) : NativeModelException(message, "NATIVE_MODEL_UNAVAILABLE")
    class Busy(message: String) : NativeModelException(message, "NATIVE_MODEL_BUSY")
    class Cancelled(message: String = "native model operation cancelled") : NativeModelException(message, "CANCELLED")
    class Closed(message: String = "native model runtime is closed") : NativeModelException(message, "CLOSED")
    class NativeFailure(message: String) : NativeModelException(message, "NATIVE_MODEL_FAILED")
}

data class NativeRuntimeInfo(val name: String, val version: String, val commit: String, val abi: Int, val available: Boolean)
data class NativeGeneration(
    val text: String,
    val promptTokens: Int,
    val generatedTokens: Int,
    val prefillMs: Double,
    val firstTokenMs: Double,
    val decodeMs: Double,
    val tokensPerSecond: Double,
    val cancelled: Boolean,
)
data class NativeTtsCapabilities(val available: Boolean, val modelId: String, val voices: List<String>, val reason: String)

/**
 * One heavyweight llama.cpp model at a time, confined to one executor. JNI calls
 * never run on Exact's main thread; cancellation is the one lock-free operation.
 */
class NativeModels {
    private val worker = AtomicReference<Thread>()
    private val executor: ExecutorService = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "AutoradioNativeModels").apply { isDaemon = true; worker.set(this) }
    }
    private val closed = AtomicBoolean(false)
    private val activeHandle = AtomicLong(0)
    private val loadedModelId = AtomicReference<String?>(null)
    private var handle: Long? = null

    val modelId: String? get() = loadedModelId.get()

    fun info(): NativeRuntimeInfo = submit {
        val json = JSONObject(nativeCall { NativeBridge.info() })
        NativeRuntimeInfo(
            name = json.getString("name"), version = json.getString("version"), commit = json.getString("commit"),
            abi = json.getInt("abi"), available = json.getBoolean("available"),
        )
    }

    fun ttsCapabilities(): NativeTtsCapabilities = submit {
        val json = JSONObject(nativeCall { NativeBridge.ttsCapabilities() })
        NativeTtsCapabilities(
            available = json.getBoolean("available"), modelId = json.getString("modelId"),
            voices = json.getJSONArray("voices").let { array -> List(array.length()) { array.getString(it) } },
            reason = json.getString("reason"),
        )
    }

    fun load(spec: NativeModelSpec, path: File, contextTokens: Int = 2048, threads: Int = 4, batchSize: Int = 256) = submit {
        requireOpen()
        if (spec.kind != NativeModelKind.TEXT && spec.kind != NativeModelKind.EMBEDDING) {
            throw NativeModelException.InvalidInput("this native runtime loads only text and embedding models")
        }
        if (handle != null) throw NativeModelException.Busy("unload the current native model first")
        if (!path.isFile) throw NativeModelException.InvalidInput("native model file is missing")
        if (contextTokens !in 1..1_048_576 || threads !in 1..64 || batchSize !in 1..4096) {
            throw NativeModelException.InvalidInput("invalid native model parameters")
        }
        val loaded = nativeCall { NativeBridge.load(path.absolutePath, spec.kind.name.lowercase(), contextTokens, threads, batchSize) }
        if (loaded == 0L) throw NativeModelException.NativeFailure("JNI returned no model handle")
        handle = loaded
        loadedModelId.set(spec.id)
    }

    fun generate(
        prompt: String,
        maxTokens: Int,
        seed: Long = 0,
        temperature: Float = 0f,
        topK: Int = 40,
        topP: Float = 1f,
        onToken: ((String) -> Unit)? = null,
    ): NativeGeneration = submit {
        val current = requireLoaded()
        if (prompt.isEmpty() || maxTokens !in 1..1_048_576 || !temperature.isFinite() || temperature < 0 ||
            !topP.isFinite() || topP <= 0f || topP > 1f || topK !in 1..1_000_000) {
            throw NativeModelException.InvalidInput("invalid generation parameters")
        }
        val callback = onToken?.let { NativeBridge.TokenCallback(it) }
        activeHandle.set(current)
        try {
            val json = JSONObject(nativeCall { NativeBridge.generate(current, prompt, maxTokens, seed, temperature, topK, topP, callback) })
            NativeGeneration(
                text = json.getString("text"), promptTokens = json.getInt("promptTokens"),
                generatedTokens = json.getInt("generatedTokens"), prefillMs = json.getDouble("prefillMs"),
                firstTokenMs = json.getDouble("firstTokenMs"), decodeMs = json.getDouble("decodeMs"),
                tokensPerSecond = json.getDouble("tokensPerSecond"), cancelled = json.getBoolean("cancelled"),
            ).also { if (it.cancelled) throw NativeModelException.Cancelled() }
        } finally {
            activeHandle.compareAndSet(current, 0)
        }
    }

    fun embed(prompt: String): FloatArray = submit {
        val current = requireLoaded()
        activeHandle.set(current)
        try { nativeCall { NativeBridge.embed(current, prompt) } }
        finally { activeHandle.compareAndSet(current, 0) }
    }

    fun unload() = submit(allowClosed = true) { unloadNow() }

    /** Safe from any thread while a token/chunk is running on the model executor: a handle that
     * unloaded first is already gone, and native cancel treats that as the no-op it is. */
    fun cancel() {
        val current = activeHandle.get()
        if (current != 0L) runCatching { nativeCall { NativeBridge.cancel(current) } }
    }

    fun close() {
        if (Thread.currentThread() == worker.get()) throw NativeModelException.NativeFailure("close must run outside the model worker")
        if (!closed.compareAndSet(false, true)) return
        // Do not wait for an in-flight load or generation on the caller (the main thread during
        // teardown): cancel, then let the executor's own queue unload before it stops.
        cancel()
        try {
            executor.execute { runCatching { unloadNow() } }
        } catch (_: java.util.concurrent.RejectedExecutionException) {
            // the executor was already stopped: nothing is loaded
        } finally {
            executor.shutdown()
        }
    }

    private fun unloadNow() {
        val current = handle ?: return
        handle = null
        loadedModelId.set(null)
        activeHandle.set(0)
        nativeCall { NativeBridge.unload(current) }
    }

    private fun requireLoaded(): Long = handle ?: throw NativeModelException.InvalidInput("no native model is loaded")

    private fun requireOpen() {
        if (closed.get()) throw NativeModelException.Closed()
    }

    private fun <T> submit(allowClosed: Boolean = false, work: () -> T): T {
        if (!allowClosed) requireOpen()
        val future: Future<T> = try { executor.submit(work) }
        catch (_: java.util.concurrent.RejectedExecutionException) { throw NativeModelException.Closed() }
        return try { future.get() }
        catch (error: ExecutionException) {
            val cause = error.cause
            if (cause is Exception) throw cause
            if (cause is Error) throw cause
            throw NativeModelException.NativeFailure("native model worker failed")
        }
    }

    private fun <T> nativeCall(work: () -> T): T = try { work() }
    catch (error: UnsatisfiedLinkError) { throw NativeModelException.Unavailable("compiled native model runtime is missing") }
    catch (error: ExceptionInInitializerError) { throw NativeModelException.Unavailable("compiled native model runtime is missing") }
    catch (error: NoClassDefFoundError) { throw NativeModelException.Unavailable("compiled native model runtime is missing") }
    catch (error: InterruptedException) { throw NativeModelException.Cancelled() }
    catch (error: IllegalArgumentException) { throw NativeModelException.InvalidInput(error.message ?: "invalid native model input") }
    catch (error: IllegalStateException) { throw NativeModelException.NativeFailure(error.message ?: "native model operation failed") }
    catch (error: JSONException) { throw NativeModelException.NativeFailure("native model returned malformed data") }
}
