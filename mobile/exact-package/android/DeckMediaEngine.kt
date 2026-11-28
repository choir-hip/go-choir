package choir.autoradio

import android.content.Context
import android.media.AudioAttributes
import android.net.Uri
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import android.speech.tts.Voice
import androidx.annotation.OptIn
import androidx.media3.common.AudioAttributes as MediaAudioAttributes
import androidx.media3.common.ForwardingPlayer
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.analytics.AnalyticsListener
import dev.exact.canvas.AudioClaim
import dev.exact.canvas.ExtensionContext
import dev.exact.video.PlaybackService
import java.io.File
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeout

/** One ExoPlayer for remote/file audio and file-backed local speech. Media3 owns rendering; the deck owns decisions and evidence. */
@OptIn(UnstableApi::class)
class DeckMediaEngine(
    private val context: ExtensionContext,
    private val remote: (String, Double?) -> Unit,
    private val focusRegained: () -> Unit,
) : DeckEngine {
    override var host: DeckEngineHost? = null
    private val main = Handler(Looper.getMainLooper())
    private val app: Context = context.application
    private val exo = ExoPlayer.Builder(app).build()
    private val sourceFactory = androidx.media3.exoplayer.source.DefaultMediaSourceFactory(app)
    private val speechDir = File(context.cache, "deck-speech")
    private val utteranceWaiters = ConcurrentHashMap<String, CompletableDeferred<Boolean>>()
    private var tts: TextToSpeech? = null
    private var ttsReady: CompletableDeferred<Boolean>? = null
    private var playbackClaim: AutoCloseable? = null
    private var focusClaim: AudioClaim? = null
    private var waitingForFocus = false
    private var expectedPause = true
    private var generation = 0L
    private var itemID = ""
    private var item: DeckItem? = null
    private var audioItem: MediaItem? = null
    private var speechChunks: List<SpeechChunk> = emptyList()
    private var speechFiles: List<File> = emptyList()
    private var itemDigest = ""                              // DeckDigest of the speech item's paragraphs as received; made once when the item starts
    private var initialPositionMs = 0L
    private var released = false
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private var start: Job? = null
    private var shouldPause = true                 // the latest transport intent: play()/resume() clear it, pause() and an outside pause set it
    private var expectedPauseAt = 0L
    private var incompleteItem: String? = null            // the speech item whose chunks are still being made

    private val sessionPlayer = object : ForwardingPlayer(exo) {
        override fun getAvailableCommands(): Player.Commands = super.getAvailableCommands().buildUpon()
            .add(Player.COMMAND_SEEK_TO_NEXT).add(Player.COMMAND_SEEK_TO_PREVIOUS).build()
        override fun isCommandAvailable(command: Int): Boolean =
            command == Player.COMMAND_SEEK_TO_NEXT || command == Player.COMMAND_SEEK_TO_PREVIOUS || super.isCommandAvailable(command)
        override fun seekToNext() = remote("next", null)
        override fun seekToPrevious() = remote("previous", null)
        override fun seekTo(positionMs: Long) = remote("seek", positionMs / 1000.0)
        override fun seekTo(mediaItemIndex: Int, positionMs: Long) = remote("seek", positionMs / 1000.0)
        // A lock screen, headset or notification command acts on the player at once, as the Apple
        // deck does, and is told to shared code for the journal. Without super.play()/super.pause()
        // the session would report a pause the player never made, and the deck would read it as a
        // system pause and drop the session.
        override fun play() { shouldPause = false; expectedPause = false; expectedPauseAt = 0; super.play(); remote("play", null) }
        override fun pause() { shouldPause = true; expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); super.pause(); remote("pause", null) }
        // Media3's session may pause by setting this rather than by `pause()`; both are the deck's
        // own transport and must be expected (an unexpected pause is the service letting the player
        // go, which is retried, not a listener pause).
        override fun setPlayWhenReady(playWhenReady: Boolean) {
            if (playWhenReady) { shouldPause = false; expectedPause = false; expectedPauseAt = 0 }
            else { shouldPause = true; expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime() }
            super.setPlayWhenReady(playWhenReady)
            remote(if (playWhenReady) "play" else "pause", null)
        }
    }
    val player: Player get() = sessionPlayer

    private val listener = object : Player.Listener {
        override fun onPlaybackStateChanged(state: Int) {
            // More of a spoken story is still being made (start is the synthesis job): the player may
            // hold only the chunks appended so far, so an ENDED here is not the story's end. If the
            // job stopped without queueing the rest, the story failed rather than finished.
            if (state != Player.STATE_ENDED || !isCurrent()) return
            if (start?.isActive == true) return                       // the synthesis job will report a failure itself if it stopped short
            if (incompleteItem == itemID) host?.onFailed(generation, itemID, "speech-incomplete") else host?.onEnded(generation, itemID)
        }
        override fun onPlayerError(error: androidx.media3.common.PlaybackException) {
            if (isCurrent()) host?.onFailed(generation, itemID, "engine-error")
        }
        override fun onPlayWhenReadyChanged(playWhenReady: Boolean, reason: Int) {
            if (playWhenReady || !isCurrent()) return
            // A pause the deck did not ask for (the service's stopPlayers after a refused foreground
            // promotion, or the host dropping the player) is an outside pause and is retried. A
            // pause through this deck's own pause() arrives within a moment of that call.
            val asked = expectedPause && SystemClock.elapsedRealtime() - expectedPauseAt < 1_500
            if (asked) return
            when (reason) {
                Player.PLAY_WHEN_READY_CHANGE_REASON_AUDIO_BECOMING_NOISY -> {
                    shouldPause = true; expectedPause = true
                    host?.onSystemPause(generation, itemID, routeLost = true, transient = false)
                }
                Player.PLAY_WHEN_READY_CHANGE_REASON_AUDIO_FOCUS_LOSS -> Unit // the process focus claim reports transient versus lasting
                else -> {
                    // PlaybackService stops its players when foreground promotion fails. Its claim is gone; hold the item and claim again on the deck's retry.
                    shouldPause = true; expectedPause = true
                    releaseSession()
                    host?.onPausedOutsideDeck(generation, itemID)
                }
            }
        }
        override fun onPlaybackSuppressionReasonChanged(reason: Int) {
            if (reason == Player.PLAYBACK_SUPPRESSION_REASON_UNSUITABLE_AUDIO_ROUTE ||
                reason == Player.PLAYBACK_SUPPRESSION_REASON_UNSUITABLE_AUDIO_OUTPUT) {
                // The route went: hold the player as well as the deck, so a synthesis that finishes
                // now cannot start sound on a route the deck has reported lost.
                shouldPause = true; expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); exo.pause()
                host?.onSystemPause(generation, itemID, routeLost = true, transient = false)
            }
        }
    }

    private val analytics = object : AnalyticsListener {
        override fun onAudioPositionAdvancing(eventTime: AnalyticsListener.EventTime, playbackPositionMs: Long) {
            if (isCurrent() && !expectedPause) {
                host?.onSounding(generation, itemID, position())
            }
        }
    }

    init {
        exo.repeatMode = Player.REPEAT_MODE_OFF
        // The process's focus claim owns audio focus (context.audioFocus), not ExoPlayer: a second
        // request from this process would take focus from its own claim and the deck would pause
        // itself with AUDIOFOCUS_LOSS (heard as `focus`, 2026-11-26).
        exo.setAudioAttributes(MediaAudioAttributes.Builder()
            .setUsage(androidx.media3.common.C.USAGE_MEDIA)
            .setContentType(androidx.media3.common.C.AUDIO_CONTENT_TYPE_SPEECH).build(), false)
        exo.addListener(listener)
        exo.addAnalyticsListener(analytics)
    }

    private fun isCurrent() = !released && itemID.isNotEmpty() && item != null

    override suspend fun prepareSession(): Boolean {
        if (playbackClaim != null && focusClaim != null) return true
        releaseSession()
        // The session wraps the deck's forwarding player, so a notification or headset command
        // reaches the deck (our play/pause/next overrides) rather than ExoPlayer directly.
        val playback = PlaybackService.claim(app, sessionPlayer) ?: return false
        playbackClaim = playback
        val focus = context.audioFocus.claim("playback", AudioAttributes.Builder()
            .setUsage(AudioAttributes.USAGE_MEDIA).setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build(),
            pauseWhenDucked = true) { event -> main.post { focusChanged(event) } }
        focusClaim = focus
        if (focus == null) { releaseSession(); return false }
        return true
    }

    private fun focusChanged(event: String) {
        when (event) {
            "focus-transient", "focus" -> {
                waitingForFocus = true
                if (isCurrent()) {
                    // A focus loss while the first chunks are still being made must also hold the
                    // start: bumping the epoch makes startSpeech pause instead of play when it
                    // finishes. `expectedPause` with no timestamp is that pending start (a listener
                    // pause stamps one), so a real listener pause is never reported as a system one.
                    val pendingStart = expectedPause && expectedPauseAt == 0L
                    if (!expectedPause || pendingStart) {
                        shouldPause = true
                        expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); exo.pause()
                        host?.onSystemPause(generation, itemID, routeLost = false, transient = event == "focus-transient")
                    }
                }
            }
            "focus-gained" -> if (waitingForFocus) {
                waitingForFocus = false
                if (isCurrent()) focusRegained()
            }
        }
    }

    private fun releaseSession() {
        focusClaim?.release(); focusClaim = null; waitingForFocus = false
        playbackClaim?.close(); playbackClaim = null
    }

    override suspend fun play(item: DeckItem, generation: Long): Boolean {
        if (released) return false
        shouldPause = false
        expectedPause = true; expectedPauseAt = 0
        incompleteItem = null
        start?.cancel(); start = null
        exo.stop(); exo.clearMediaItems(); tts?.stop()
        speechFiles.forEach { it.delete() }; speechFiles = emptyList(); speechChunks = emptyList()
        itemDigest = if (item.kind == "speech") DeckDigest.of(item.paragraphs) else ""
        this.generation = generation; this.itemID = item.id; this.item = item; audioItem = null
        initialPositionMs = (if (item.seconds > 0) item.seconds else item.start).coerceAtLeast(0.0).times(1000).toLong()

        val media: List<MediaItem>
        val chunks: List<SpeechChunk>
        if (item.kind == "audio") {
            val uri = audioUri(item.url) ?: return false
            val base = MediaItem.Builder().setUri(uri).setMediaMetadata(metadata(item)).apply {
                if (item.end > 0) setClippingConfiguration(MediaItem.ClippingConfiguration.Builder().setEndPositionMs(item.end.times(1000).toLong()).build())
            }.build()
            audioItem = base
            media = listOf(base); chunks = emptyList()
            exo.setWakeMode(if (uri.scheme == "https" || uri.scheme == "http") androidx.media3.common.C.WAKE_MODE_NETWORK else androidx.media3.common.C.WAKE_MODE_LOCAL)
        } else {
            val texts = speechChunksOf(item) ?: return false
            if (texts.isEmpty()) { println("deck: speech has no paragraphs"); return false }
            chunks = texts
            // On-device speech takes seconds to make; shared code cannot wait for it inside a data
            // answer (Exact's wall backstop is a second, and the answer would be let go). The start
            // is made in the background and the deck reports `sounding` when the first words play.
            start?.cancel()
            start = scope.launch { startSpeech(item, texts, generation) }
            return true
        }
        if (this.generation != generation || released) { speechFiles.forEach { it.delete() }; return false }
        exo.setMediaSources(media.map { sourceFactory.createMediaSource(it) })
        exo.prepare()
        if (initialPositionMs > 0) exo.seekTo(initialPositionMs)
        // A pause pressed while the first chunks were being made wins over the start.
        if (shouldPause) { expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); exo.pause() } else { expectedPause = false; expectedPauseAt = 0; exo.play() }
        return true
    }

    /** The first words, then the rest while they play: two chunks before the player starts (a one-line
     *  first paragraph can be shorter than the next chunk's synthesis), then one at a time. */
    private suspend fun startSpeech(item: DeckItem, texts: List<SpeechChunk>, generation: Long) {
        incompleteItem = item.id
        val first = synthesizeRange(texts, 0, minOf(2, texts.size), generation)
        if (first == null) { host?.onFailed(generation, item.id, "speech-unavailable"); return }
        if (released || this.generation != generation || itemID != item.id) { first.forEach { it.file.delete() }; return }
        speechFiles = first.map { it.file }
        speechChunks = first
        exo.setWakeMode(androidx.media3.common.C.WAKE_MODE_LOCAL)
        exo.setMediaSources(first.map { sourceFactory.createMediaSource(MediaItem.Builder().setUri(Uri.fromFile(it.file)).setMediaMetadata(metadata(item)).build()) })
        exo.prepare()
        if (initialPositionMs > 0) exo.seekTo(initialPositionMs)
        if (shouldPause) { expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); exo.pause() } else { expectedPause = false; expectedPauseAt = 0; exo.play() }
        for (index in first.size until texts.size) {
            if (released || this.generation != generation || itemID != item.id) break
            val made = synthesizeRange(texts, index, index + 1, generation) ?: break
            speechFiles = speechFiles + made.map { it.file }
            speechChunks = speechChunks + made
            exo.addMediaItem(MediaItem.Builder().setUri(Uri.fromFile(made[0].file)).setMediaMetadata(metadata(item)).build())
            // It reached the end while the next chunk was being made: play on, unless the listener
            // paused meanwhile, in which case the pause wins and the added chunk waits.
            if (exo.playbackState == Player.STATE_ENDED && !shouldPause) exo.play()
        }
        if (speechChunks.size == texts.size && incompleteItem == item.id) incompleteItem = null
        // The player ended on the chunks made so far while the last synthesis was still running; the
        // ENDED listener returned because this job was active, and no further state change is coming.
        // The story stopped short: report the failure now rather than wedge on a dead item.
        if (incompleteItem == item.id && !released && this.generation == generation && itemID == item.id && exo.playbackState == Player.STATE_ENDED) {
            host?.onFailed(generation, itemID, "speech-incomplete")
        }
    }

    private fun metadata(item: DeckItem): MediaMetadata = MediaMetadata.Builder()
        .setTitle(item.title)
        .setArtist(item.show.ifEmpty { "Autoradio" })
        .setAlbumTitle("Autoradio")
        .setMediaType(MediaMetadata.MEDIA_TYPE_AUDIO_BOOK_CHAPTER)
        .build()

    private fun audioUri(raw: String): Uri? {
        if (raw.isEmpty()) return null
        if (raw.startsWith("app:/cache/files/")) {
            val parts = raw.removePrefix("app:/cache/files/").split('/')
            if (parts.size != 2 || parts.any { it.isEmpty() || it == "." || it == ".." }) return null
            val file = File(File(File(context.cache, "files"), parts[0]), parts[1])
            if (!file.isFile) return null
            return Uri.fromFile(file)
        }
        val uri = Uri.parse(raw)
        return when (uri.scheme?.lowercase(Locale.ROOT)) {
            "https", "http" -> if (uri.host.isNullOrEmpty()) null else uri
            "file" -> uri
            else -> null
        }
    }

    /** The story's paragraphs as speech chunks, each part within the engine's one-utterance bound. */
    private fun speechChunksOf(item: DeckItem): List<SpeechChunk>? {
        if (speechDir.exists()) speechDir.deleteRecursively()
        if (!speechDir.mkdirs()) return null
        return SpeechRanges.parts(item.paragraphs, item.block, minOf(TextToSpeech.getMaxSpeechInputLength(), 3500))
            .map { SpeechChunk(it.block, it.text, File(speechDir, "pending"), it.start, it.length) }
    }

    /** Make the chunks in [from, until), each to its own file, and return them. The engine plays one
     *  utterance at a time, so a range of one is one synthesis (the rolling start uses that). */
    private suspend fun synthesizeRange(chunks: List<SpeechChunk>, from: Int, until: Int, generation: Long): List<SpeechChunk>? {
        val engine = speechEngine() ?: return null
        val language = engine.setLanguage(Locale.getDefault())
        if (language == TextToSpeech.LANG_MISSING_DATA || language == TextToSpeech.LANG_NOT_SUPPORTED) {
            val fallback = engine.setLanguage(Locale.US)
            if (fallback == TextToSpeech.LANG_MISSING_DATA || fallback == TextToSpeech.LANG_NOT_SUPPORTED) return null
        }
        val made = ArrayList<SpeechChunk>()
        for (index in from until until) {
            if (this.generation != generation || released) { made.forEach { it.file.delete() }; return null }
            val file = File(speechDir, "deck-$generation-$index.wav")
            val id = "deck-$generation-$index"
            val done = CompletableDeferred<Boolean>(); utteranceWaiters[id] = done
            val params = Bundle().apply { putFloat(TextToSpeech.Engine.KEY_PARAM_VOLUME, 1.0f) }
            val given = engine.synthesizeToFile(chunks[index].text, params, file, id)
            if (given != TextToSpeech.SUCCESS) { utteranceWaiters.remove(id); file.delete(); made.forEach { it.file.delete() }; return null }
            val ok = try { withTimeout(120_000) { done.await() } } catch (e: Exception) { if (e is kotlinx.coroutines.CancellationException) throw e; false }
            utteranceWaiters.remove(id)
            if (!ok || !file.isFile || file.length() == 0L) { println("deck: chunk $index synthesis failed"); file.delete(); made.forEach { it.file.delete() }; return null }
            made += chunks[index].copy(file = file)
        }
        return made
    }

    private suspend fun speechEngine(): TextToSpeech? {
        tts?.let { return it }
        val ready = CompletableDeferred<Boolean>(); ttsReady = ready
        // The listener is attached to the engine being made, not to the field: the field is set only
        // once the engine is ready (a failed engine must not be kept and reused).
        var madeRef: TextToSpeech? = null
        val made = TextToSpeech(app, { status -> main.post {
            if (status == TextToSpeech.SUCCESS) {
                madeRef?.setOnUtteranceProgressListener(progress)
                ready.complete(true)
            } else ready.complete(false)
        } })
        madeRef = made
        val ok = try { withTimeout(15_000) { ready.await() } } catch (_: Exception) { false }
        return if (ok) { tts = made; made } else { runCatching { made.shutdown() }; tts = null; if (ttsReady === ready) ttsReady = null; null }
    }

    private val progress = object : UtteranceProgressListener() {
        override fun onStart(utteranceId: String?) {}
        override fun onDone(utteranceId: String?) { utteranceWaiters.remove(utteranceId)?.complete(true) }
        override fun onError(utteranceId: String?) { utteranceWaiters.remove(utteranceId)?.complete(false) }
        override fun onError(utteranceId: String?, errorCode: Int) { utteranceWaiters.remove(utteranceId)?.complete(false) }
        override fun onStop(utteranceId: String?, interrupted: Boolean) { utteranceWaiters.remove(utteranceId)?.complete(false) }
    }

    override fun pause() { shouldPause = true; expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); exo.pause() }
    override fun resume() {
        if (waitingForFocus) {
            focusClaim?.release(); focusClaim = null; waitingForFocus = false
            focusClaim = context.audioFocus.claim("playback", AudioAttributes.Builder()
                .setUsage(AudioAttributes.USAGE_MEDIA).setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build(), true) { event -> main.post { focusChanged(event) } }
            if (focusClaim == null) { shouldPause = true; expectedPause = true; expectedPauseAt = SystemClock.elapsedRealtime(); exo.pause(); host?.onPausedOutsideDeck(generation, itemID); return }
        }
        shouldPause = false; expectedPause = false; expectedPauseAt = 0; exo.play()
    }
    override fun stop() {
        start?.cancel(); start = null
        shouldPause = true; expectedPause = true; generation += 1; itemID = ""; item = null; audioItem = null; speechChunks = emptyList(); itemDigest = ""; incompleteItem = null
        utteranceWaiters.values.forEach { it.complete(false) }; utteranceWaiters.clear()
        tts?.stop(); exo.stop(); exo.clearMediaItems(); releaseSession()
        speechFiles.forEach { it.delete() }; speechFiles = emptyList(); speechDir.deleteRecursively()
        exo.setWakeMode(androidx.media3.common.C.WAKE_MODE_NONE)
    }
    override fun seekTo(seconds: Double) { if (isCurrent()) exo.seekTo(seconds.coerceAtLeast(0.0).times(1000).toLong()) }
    override fun seekBy(seconds: Double) { if (isCurrent()) exo.seekTo(exo.currentPosition + seconds.times(1000).toLong()) }
    override fun clearEnd() {
        val current = item ?: return
        val base = audioItem ?: return
        val positionMs = exo.currentPosition
        val playWhenReady = exo.playWhenReady
        expectedPause = true
        val unclipped = base.buildUpon().setClippingConfiguration(MediaItem.ClippingConfiguration.Builder().build()).build()
        audioItem = unclipped
        exo.setMediaSource(sourceFactory.createMediaSource(unclipped), positionMs)
        exo.prepare(); shouldPause = !playWhenReady; expectedPause = !playWhenReady
        if (playWhenReady) exo.play() else exo.pause()
    }
    override fun restart() { if (isCurrent()) exo.seekTo(0, 0L) }
    override fun position(): DeckPosition {
        val current = item ?: return DeckPosition()
        val seconds = exo.currentPosition / 1000.0
        return if (current.kind == "audio") DeckPosition(0, seconds, 0, duration())
        else {
            val index = exo.currentMediaItemIndex.coerceIn(0, maxOf(0, speechChunks.lastIndex))
            DeckPosition(speechChunks.getOrNull(index)?.block ?: current.block, seconds, speechChunks.lastOrNull()?.block ?: current.block, 0.0)
        }
    }
    /** Voice: only the player knows which chunk sounds and how far. Run where ExoPlayer and `speechChunks` live (the main looper); the chunks and the player share that thread and no document can be replaced under them, so no
     *  generation is needed. The duration of the chunk is Media3's, C.TIME_UNSET (a large negative number) until it is known: that is 0 (placeOf). */
    override fun place(): DeckPlace {
        val current = item ?: return DeckPlace()
        if (current.kind != "speech") return DeckPlace()
        return placeOf(itemID, itemDigest, speechChunks.map { SpeechPart(it.block, it.text, it.start, it.length) }, exo.currentMediaItemIndex, exo.currentPosition, exo.duration)
    }
    private fun duration(): Double = if (exo.duration == androidx.media3.common.C.TIME_UNSET) 0.0 else exo.duration / 1000.0
    override fun release() { if (!released) { start?.cancel(); scope.cancel(); utteranceWaiters.values.forEach { it.complete(false) }; utteranceWaiters.clear(); stop(); exo.removeListener(listener); exo.removeAnalyticsListener(analytics); exo.release(); tts?.shutdown(); tts = null; released = true } }
    override fun wipeCache() { speechDir.deleteRecursively() }

    private data class SpeechChunk(val block: Int, val text: String, val file: File, val start: Int, val length: Int)
}
