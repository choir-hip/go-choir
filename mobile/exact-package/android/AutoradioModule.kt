package choir.autoradio

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.graphics.Bitmap
import android.graphics.Color
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.net.Uri
import android.os.Handler
import android.os.Looper
import android.view.View
import android.view.ViewGroup
import android.webkit.RenderProcessGoneDetail
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import dev.exact.canvas.ExactExtension
import dev.exact.canvas.ExtensionContext
import dev.exact.canvas.ViewLifecycle
import org.json.JSONException
import org.json.JSONObject
import java.io.File
import java.net.URI
import java.util.Locale
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

private enum class LinkRoute { inApp, external, refused }

/** The same narrow URL contract as SourceLink.swift: web pages stay in the app; mail, phone and message links leave it. */
private fun route(raw: String?): LinkRoute {
    val text = raw?.trim().orEmpty()
    if (text.isEmpty()) return LinkRoute.refused
    // Android Uri.parse is deliberately permissive. Reject the malformed forms Foundation's URL(string:) rejects.
    if (text.any { it.isWhitespace() || Character.isISOControl(it) || it == '\\' }) return LinkRoute.refused
    val parsed = try {
        URI(text)
    } catch (_: Exception) {
        return LinkRoute.refused
    }
    if (!parsed.isAbsolute) return LinkRoute.refused
    val scheme = parsed.scheme?.lowercase(Locale.ROOT) ?: return LinkRoute.refused
    return when (scheme) {
        "http", "https" -> if (parsed.host.isNullOrEmpty() || parsed.rawAuthority.isNullOrEmpty()) LinkRoute.refused else LinkRoute.inApp
        "mailto", "tel", "sms", "facetime" -> if (parsed.rawSchemeSpecificPart.isNullOrEmpty()) LinkRoute.refused else LinkRoute.external
        else -> LinkRoute.refused
    }
}

/** The package's Android implementation. Unknown names are a refusal, never a silent substitute. */
class AutoradioModule(context: ExtensionContext) : ExactExtension(context) {
    private val main = Handler(Looper.getMainLooper())
    private val modelExecutor = Executors.newSingleThreadExecutor { runnable ->
        Thread(runnable, "AutoradioModelProof").apply { isDaemon = true }
    }
    private val destroyed = AtomicBoolean(false)
    private val nativeModelsLazy = lazy { NativeModels() }
    private val nativeModels get() = nativeModelsLazy.value
    private val downloadsLazy = lazy { ModelDownloadManager(File(context.data, "native-models"), ::onWifi) }
    private val downloads get() = downloadsLazy.value
    private val selfTestLazy = lazy { NativeModelSelfTest(nativeModels, downloads) }
    private val selfTest get() = selfTestLazy.value

    override fun later(name: String, args: JSONObject, reply: (JSONObject) -> Unit) {
        when (name) {
            "readerOpenExternal" -> {
                val ok = openExternal(Uri.parse(args.optString("url").trim()))
                reply(JSONObject().put("ok", ok))
            }
            "nativeModelFacts" -> modelWork(reply) { selfTest.facts() }
            "nativeModelEnsure" -> modelWork(reply) { selfTest.ensure(args.getString("id")) }
            "nativeModelRemove" -> modelWork(reply) { selfTest.remove(args.getString("id")) }
            "nativeModelProof" -> modelWork(reply) { selfTest.proof(args) }
            else -> reply(JSONObject().put("error", "NotSupportedError"))
        }
    }

    override fun view(tag: String, session: String, id: String, context: Context, props: JSONObject, emit: (String, String) -> Unit): View {
        if (tag != "web-page") throw IllegalArgumentException("Autoradio serves no view $tag")
        // Exact binds PackageView.events after this returns and immediately calls update(). Loading here would drop the first message.
        return PageWebView(context, emit, ::openExternal)
    }

    override fun update(view: View, props: JSONObject) {
        (view as? PageWebView)?.show(props.optString("page"))
    }

    override fun viewLifecycle(view: View, event: ViewLifecycle, state: android.os.Bundle?) {
        val page = view as? PageWebView ?: return
        when (event) {
            ViewLifecycle.RESUME -> page.resumePage()
            ViewLifecycle.PAUSE, ViewLifecycle.STOP -> page.pausePage()
            ViewLifecycle.DESTROY -> page.destroyPage()
            else -> Unit
        }
    }

    override fun destroyView(view: View) {
        (view as? PageWebView)?.destroyPage()
    }

    override fun destroy(done: () -> Unit) {
        if (!destroyed.compareAndSet(false, true)) { done(); return }
        try {
            if (nativeModelsLazy.isInitialized()) nativeModels.close()
        } finally {
            modelExecutor.shutdown()
            done()
        }
    }

    /** Native model execution is the only reason these package names exist; shared TypeScript owns policy. */
    private fun modelWork(reply: (JSONObject) -> Unit, work: () -> JSONObject) {
        if (destroyed.get()) return reply(closed())
        try {
            modelExecutor.execute {
                val answer = try {
                    if (destroyed.get()) closed() else work()
                } catch (error: NativeModelException) {
                    JSONObject().put("error", error.code).put("message", error.message ?: error.code)
                } catch (_: JSONException) {
                    JSONObject().put("error", "INVALID_INPUT")
                } catch (error: Throwable) {
                    // A link error or an OOM on the worker must still answer exactly once; silence
                    // would leave the shared caller waiting behind a reply that never comes.
                    JSONObject().put("error", "NATIVE_MODEL_FAILED").put("message", error.message ?: error.javaClass.name)
                }
                main.post { reply(answer) }
            }
        } catch (_: java.util.concurrent.RejectedExecutionException) {
            reply(closed())
        }
    }

    private fun closed() = JSONObject().put("error", "CLOSED")

    private fun onWifi(): Boolean {
        val manager = context.application.getSystemService(Context.CONNECTIVITY_SERVICE) as? ConnectivityManager ?: return false
        val network = manager.activeNetwork ?: return false
        val capabilities = manager.getNetworkCapabilities(network) ?: return false
        return capabilities.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) && capabilities.hasCapability(NetworkCapabilities.NET_CAPABILITY_INTERNET)
    }

    private fun openExternal(uri: Uri): Boolean {
        if (route(uri.toString()) == LinkRoute.refused) return false
        if (!context.leave(uri.toString(), "external")) return false
        val activity = context.activity
        val target = activity ?: context.application
        return try {
            val intent = Intent(Intent.ACTION_VIEW, uri)
            if (uri.scheme?.lowercase(Locale.ROOT) == "http" || uri.scheme?.lowercase(Locale.ROOT) == "https") {
                intent.addCategory(Intent.CATEGORY_BROWSABLE)
            }
            if (activity == null) intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            target.startActivity(intent)
            true
        } catch (_: ActivityNotFoundException) {
            false
        } catch (_: SecurityException) {
            false
        }
    }
}

/** A real Android WebView, laid out by Exact. It reports the same loading/done/failed messages as the Apple view. */
private class PageWebView(
    context: Context,
    private val emit: (String, String) -> Unit,
    private val openExternal: (Uri) -> Boolean,
) : WebView(context) {
    private var loaded = ""
    private var mainFrameFailed = false
    private var destroyed = false

    init {
        settings.javaScriptEnabled = true
        settings.domStorageEnabled = true
        settings.allowFileAccess = false
        settings.allowContentAccess = false
        settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
        settings.setSupportMultipleWindows(false)
        setBackgroundColor(Color.TRANSPARENT)
        webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView, request: WebResourceRequest): Boolean {
                val uri = request.url
                return when (route(uri.toString())) {
                    LinkRoute.inApp -> false
                    LinkRoute.external -> {
                        openExternal(uri)
                        true
                    }
                    LinkRoute.refused -> {
                        val about = uri.scheme?.lowercase(Locale.ROOT) == "about"
                        request.isForMainFrame && !about
                    }
                }
            }

            override fun onPageStarted(view: WebView, url: String, favicon: Bitmap?) {
                mainFrameFailed = false
                emit("message", "loading")
            }

            override fun onPageFinished(view: WebView, url: String) {
                if (mainFrameFailed) return
                emit("message", "done")
                emit("load", "")
            }

            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (request.isForMainFrame) {
                    mainFrameFailed = true
                    emit("message", "failed")
                }
            }

            override fun onRenderProcessGone(view: WebView, detail: RenderProcessGoneDetail): Boolean {
                mainFrameFailed = true
                emit("message", "failed")
                destroyPage()
                return true
            }
        }
    }

    fun show(page: String) {
        if (destroyed) return
        val next = page.trim()
        if (next == loaded) return
        loaded = next
        mainFrameFailed = false
        val uri = Uri.parse(next)
        when (route(next)) {
            LinkRoute.inApp -> loadUrl(uri.toString())
            // A page prop is not an external-open request. Only a tapped in-page hand-off may leave the app.
            LinkRoute.external, LinkRoute.refused -> emit("message", "failed")
        }
    }

    fun pausePage() {
        if (!destroyed) super.onPause()
    }

    fun resumePage() {
        if (!destroyed) super.onResume()
    }

    fun destroyPage() {
        if (destroyed) return
        destroyed = true
        stopLoading()
        (parent as? ViewGroup)?.removeView(this)
        webViewClient = WebViewClient()
        destroy()
    }
}
