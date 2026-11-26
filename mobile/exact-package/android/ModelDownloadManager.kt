package choir.autoradio

import java.io.File
import java.io.FileOutputStream
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.AtomicMoveNotSupportedException
import java.nio.file.Files
import java.nio.file.LinkOption
import java.nio.file.StandardCopyOption
import java.security.MessageDigest
import java.util.Comparator

class ModelDownloadException(val code: String, message: String) : IOException(message)

internal fun safeModelDirectory(root: File, id: String): File {
    require(id.isNotEmpty() && id.first() in 'a'..'z' && id.all { it in 'a'..'z' || it in '0'..'9' || it in "-._:" } && !id.contains("..")) {
        "invalid native model id"
    }
    val base = root.canonicalFile
    val candidate = File(base, id).canonicalFile
    require(candidate.toPath().startsWith(base.toPath()) && candidate != base) { "native model path escapes its root" }
    return File(base, id)
}

/** Downloads only manifest-pinned files, on Wi-Fi, into app-private storage. */
class ModelDownloadManager(
    root: File,
    private val onWifi: () -> Boolean,
    private val openConnection: (URL) -> HttpURLConnection = { it.openConnection() as HttpURLConnection },
) {
    private val root = root.canonicalFile.apply { require(mkdirs() || isDirectory) { "cannot create model root" } }

    fun isReady(id: String): Boolean {
        val spec = NativeModelCatalog.require(id)
        val directory = safeModelDirectory(root, id)
        return spec.files.all { file -> verifyFile(File(directory, file.name), file.sha256, file.bytes) }
    }

    fun file(id: String, name: String): File {
        val spec = NativeModelCatalog.require(id)
        val file = spec.files.singleOrNull { it.name == name } ?: throw IllegalArgumentException("unknown file for native model")
        val target = File(safeModelDirectory(root, id), file.name)
        require(target.canonicalFile.toPath().startsWith(root.toPath())) { "model file escaped its root" }
        return target
    }

    fun download(id: String, onProgress: (Long, Long) -> Unit = { _, _ -> }) {
        requireWifi()
        val spec = NativeModelCatalog.require(id)
        val directory = safeModelDirectory(root, id)
        require(directory.mkdirs() || directory.isDirectory) { "cannot create model directory" }
        require(safeModelDirectory(root, id).canonicalFile.toPath().startsWith(root.toPath())) { "model directory escaped its root" }
        for (file in spec.files) {
            requireWifi()
            val final = File(directory, file.name)
            require(final.name == file.name && file.name != "." && file.name != "..") { "invalid model file name" }
            require(final.canonicalFile.toPath().startsWith(directory.canonicalFile.toPath())) { "model file escaped its directory" }
            if (verifyFile(final, file.sha256, file.bytes)) {
                onProgress(file.bytes.toLong(), file.bytes.toLong())
                continue
            }
            var temporary: File? = null
            var connection: HttpURLConnection? = null
            try {
                temporary = File.createTempFile(".${file.name}.", ".part", directory)
                connection = httpsConnection(file.url)
                val expected = file.bytes
                val length = connection.contentLengthLong
                if (length >= 0 && length != expected) throw ModelDownloadException("SIZE_MISMATCH", "model size does not match its pin")
                val digest = MessageDigest.getInstance("SHA-256")
                var total = 0L
                connection.inputStream.use { input ->
                    FileOutputStream(temporary).use { output ->
                        val buffer = ByteArray(64 * 1024)
                        while (true) {
                            requireWifi()
                            val count = input.read(buffer)
                            if (count < 0) break
                            if (total + count > expected) throw ModelDownloadException("SIZE_MISMATCH", "model size does not match its pin")
                            output.write(buffer, 0, count)
                            digest.update(buffer, 0, count)
                            total += count
                            onProgress(total, expected)
                        }
                        output.fd.sync()
                    }
                }
                if (total != expected) throw ModelDownloadException("SIZE_MISMATCH", "model size does not match its pin")
                if (digest.digest().joinToString("") { "%02x".format(it) } != file.sha256) {
                    throw ModelDownloadException("HASH_MISMATCH", "model hash does not match its pin")
                }
                try {
                    Files.move(temporary.toPath(), final.toPath(), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING)
                } catch (_: AtomicMoveNotSupportedException) {
                    throw ModelDownloadException("ATOMIC_RENAME_UNSUPPORTED", "model storage cannot replace atomically")
                }
                temporary = null
            } catch (error: ModelDownloadException) {
                throw error
            } catch (error: IOException) {
                throw ModelDownloadException("DOWNLOAD_FAILED", error.message ?: "model download failed")
            } finally {
                connection?.disconnect()
                temporary?.delete()
            }
        }
        require(isReady(id)) { "downloaded model did not pass its pin" }
    }

    fun remove(id: String) {
        NativeModelCatalog.require(id)
        val directory = safeModelDirectory(root, id)
        if (!Files.exists(directory.toPath(), LinkOption.NOFOLLOW_LINKS)) return
        Files.walk(directory.toPath()).use { paths ->
            paths.sorted(Comparator.reverseOrder()).forEach { Files.deleteIfExists(it) }
        }
    }

    private fun requireWifi() {
        if (!onWifi()) throw ModelDownloadException("WIFI_REQUIRED", "model downloads require Wi-Fi")
    }

    private fun httpsConnection(raw: String): HttpURLConnection {
        var url = try { URL(raw) } catch (error: Exception) {
            throw ModelDownloadException("INVALID_INPUT", error.message ?: "model URL is invalid")
        }
        if (url.protocol != "https" || !url.userInfo.isNullOrEmpty()) {
            throw ModelDownloadException("INVALID_INPUT", "model URL must be HTTPS without credentials")
        }
        repeat(6) {
            val connection = openConnection(url)
            connection.requestMethod = "GET"
            connection.instanceFollowRedirects = false
            connection.useCaches = false
            connection.connectTimeout = 30_000
            connection.readTimeout = 60_000
            connection.setRequestProperty("Accept-Encoding", "identity")
            val code = connection.responseCode
            if (code in 300..399) {
                val location = connection.getHeaderField("Location")
                connection.disconnect()
                if (location.isNullOrEmpty()) throw ModelDownloadException("DOWNLOAD_FAILED", "model redirect has no location")
                url = try { URL(url, location) } catch (error: Exception) {
                    throw ModelDownloadException("DOWNLOAD_FAILED", error.message ?: "model redirect is invalid")
                }
                if (url.protocol != "https" || !url.userInfo.isNullOrEmpty()) {
                    throw ModelDownloadException("DOWNLOAD_FAILED", "model redirect must stay on HTTPS without credentials")
                }
                return@repeat
            }
            if (code != 200) {
                connection.disconnect()
                throw ModelDownloadException("DOWNLOAD_FAILED", "model server returned HTTP $code")
            }
            return connection
        }
        throw ModelDownloadException("TOO_MANY_REDIRECTS", "model download redirected too many times")
    }

    companion object {
        fun sha256(file: File): String {
            val digest = MessageDigest.getInstance("SHA-256")
            file.inputStream().use { input ->
                val buffer = ByteArray(64 * 1024)
                while (true) {
                    val count = input.read(buffer)
                    if (count < 0) break
                    digest.update(buffer, 0, count)
                }
            }
            return digest.digest().joinToString("") { "%02x".format(it) }
        }

        fun verifyFile(file: File, sha256: String, bytes: Long): Boolean =
            file.isFile && file.length() == bytes && sha256(file) == sha256
    }
}
