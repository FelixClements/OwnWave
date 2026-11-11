package io.github.felixclements.ownwave.playback

import android.app.PendingIntent
import android.content.Intent
import android.net.ConnectivityManager
import androidx.core.net.toUri
import android.os.Bundle
import androidx.annotation.OptIn
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DataSourceBitmapLoader
import androidx.media3.datasource.HttpDataSource
import androidx.media3.datasource.ResolvingDataSource
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.DefaultLoadControl
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.exoplayer.upstream.DefaultLoadErrorHandlingPolicy
import androidx.media3.exoplayer.upstream.LoadErrorHandlingPolicy
import androidx.media3.session.CacheBitmapLoader
import androidx.media3.session.CommandButton
import androidx.media3.session.LibraryResult
import androidx.media3.session.MediaLibraryService
import androidx.media3.session.MediaSession
import androidx.media3.session.SessionCommand
import androidx.media3.session.SessionError
import androidx.media3.session.SessionResult
import com.google.common.collect.ImmutableList
import com.google.common.util.concurrent.Futures
import com.google.common.util.concurrent.ListenableFuture
import io.github.felixclements.ownwave.MainActivity
import io.github.felixclements.ownwave.R
import io.github.felixclements.ownwave.api.models.QueueTrack
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.auth.SessionState
import io.github.felixclements.ownwave.container
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.guava.future
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.IOException

/**
 * The one player. It serves the phone UI (through a MediaController), the notification, the lock
 * screen, Bluetooth and (from PR 3) Android Auto.
 *
 * - A controller plays a station by setting one item with media id `station:<id>`; [Callback.onSetMediaItems]
 *   swaps it for the station's queue.
 * - Track URIs are `ownwave://track/<id>`; [StreamResolver] picks the URL and quality when each one loads.
 * - The queue refills itself near the end ([StationQueue]) and plays are reported after 30 s ([PlayReporter]).
 * - Signing out (or a 401, which signs out) stops playback and clears the queue.
 */
@OptIn(UnstableApi::class)
class PlaybackService : MediaLibraryService() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private lateinit var player: ExoPlayer
    private var librarySession: MediaLibrarySession? = null

    private val stationQueue = StationQueue()
    private val playReporter = PlayReporter()

    /** Like state per track id, seeded from the queue and changed by the like button. */
    private val liked = mutableMapOf<String, Boolean>()
    private val banned = mutableSetOf<String>()
    private var stationId: String? = null
    private var refilling = false
    private var skippedErrors = 0
    private var progressJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        val app = container
        val http = OkHttpDataSource.Factory(app.httpClient)
        val resolver = StreamResolver(
            server = { app.sessionStore.current?.server },
            quality = { QualityPolicy.choose(isMetered(), app.settingsStore.current) },
        )
        val streams = ResolvingDataSource.Factory(http) { spec ->
            if (spec.uri.scheme != MediaIds.SCHEME) {
                spec
            } else {
                val url = resolver.resolve(spec.uri.toString()) ?: throw IOException("Not signed in")
                spec.withUri(url.toString().toUri())
            }
        }

        player = ExoPlayer.Builder(this)
            .setMediaSourceFactory(DefaultMediaSourceFactory(streams).setLoadErrorHandlingPolicy(NoRetryOnClientErrors()))
            .setLoadControl(
                DefaultLoadControl.Builder()
                    .setBufferDurationsMs(MIN_BUFFER_MS, MAX_BUFFER_MS, START_BUFFER_MS, REBUFFER_MS)
                    .build(),
            )
            .setAudioAttributes(
                AudioAttributes.Builder().setUsage(C.USAGE_MEDIA).setContentType(C.AUDIO_CONTENT_TYPE_MUSIC).build(),
                /* handleAudioFocus = */ true,
            )
            .setHandleAudioBecomingNoisy(true)
            .setWakeMode(C.WAKE_MODE_NETWORK)
            .build()
        player.addListener(PlayerEvents())

        val openApp = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
        librarySession = MediaLibrarySession.Builder(this, player, Callback())
            .setSessionActivity(openApp)
            // Covers need the Bearer token, so the notification loads them through the app's OkHttp client.
            .setBitmapLoader(CacheBitmapLoader(DataSourceBitmapLoader.Builder(this).setDataSourceFactory(http).build()))
            .setMediaButtonPreferences(buttons(liked = false))
            .build()

        scope.launch {
            app.sessionStore.state.collect { if (it is SessionState.SignedOut) stopAndClear() }
        }
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaLibrarySession? = librarySession

    override fun onDestroy() {
        scope.cancel()
        librarySession?.run {
            player.release()
            release()
        }
        librarySession = null
        super.onDestroy()
    }

    private val session: Session? get() = container.sessionStore.current

    private fun isMetered(): Boolean =
        getSystemService(ConnectivityManager::class.java)?.isActiveNetworkMetered ?: true

    private fun stopAndClear() {
        player.stop()
        player.clearMediaItems()
        stationId = null
        stationQueue.reset()
        liked.clear()
        banned.clear()
    }

    // --- Station queue ---------------------------------------------------------------------------

    private suspend fun loadStation(id: String): List<MediaItem> {
        val s = session ?: throw IOException("Not signed in")
        val tracks = container.playbackRepository.queue(s, id).getOrThrow()
        stationId = id
        stationQueue.reset()
        playReporter.reset()
        liked.clear()
        tracks.forEach { liked[it.id] = it.liked }
        return tracks.filter { it.id !in banned }.map { it.toMediaItem(s, id) }
    }

    private fun maybeRefill() {
        val station = stationId ?: return
        val s = session ?: return
        if (refilling || !stationQueue.needsRefill(player.currentMediaItemIndex, player.mediaItemCount)) return
        refilling = true
        scope.launch {
            try {
                val tracks = container.playbackRepository.queue(s, station).getOrNull() ?: return@launch
                if (stationId != station) return@launch
                val upcoming = (player.currentMediaItemIndex.coerceAtLeast(0) until player.mediaItemCount)
                    .mapNotNull { MediaIds.parseTrack(player.getMediaItemAt(it).mediaId)?.trackId }
                    .toSet()
                val fresh = stationQueue.fresh(tracks.filter { it.id !in banned }, { it.id }, upcoming)
                if (fresh.isEmpty()) return@launch
                fresh.forEach { liked.putIfAbsent(it.id, it.liked) }
                val firstNew = player.mediaItemCount
                player.addMediaItems(fresh.map { it.toMediaItem(s, station) })
                if (player.playbackState == Player.STATE_ENDED) {
                    player.seekTo(firstNew, 0)
                    player.play()
                }
            } finally {
                refilling = false
            }
        }
    }

    private fun QueueTrack.toMediaItem(session: Session, station: String): MediaItem =
        MediaItem.Builder()
            .setMediaId(MediaIds.track(id, station))
            .setUri(MediaIds.trackUri(id))
            .setMediaMetadata(
                MediaMetadata.Builder()
                    .setTitle(title)
                    .setArtist(artist)
                    .setAlbumTitle(album)
                    .setArtworkUri(container.stationRepository.trackCoverUrl(session, id).toUri())
                    .setDurationMs(durationSeconds?.let { (it * 1000).toLong() })
                    .setIsBrowsable(false)
                    .setIsPlayable(true)
                    .setMediaType(MediaMetadata.MEDIA_TYPE_MUSIC)
                    .build(),
            )
            .build()

    // --- Like, ban, plays ------------------------------------------------------------------------

    private fun currentTrack(): MediaIds.TrackRef? =
        player.currentMediaItem?.mediaId?.let(MediaIds::parseTrack)

    private fun toggleLike() {
        val track = currentTrack() ?: return
        val s = session ?: return
        val now = !(liked[track.trackId] ?: false)
        liked[track.trackId] = now
        publishLikeState()
        scope.launch {
            if (!container.playbackRepository.setLiked(s, track.trackId, now)) {
                liked[track.trackId] = !now
                publishLikeState()
            }
        }
    }

    /** Skips the current track at once; the ban is sent once and not retried. */
    private fun ban() {
        val track = currentTrack() ?: return
        val s = session ?: return
        banned += track.trackId
        scope.launch { container.playbackRepository.ban(s, track.trackId) }
        player.removeMediaItem(player.currentMediaItemIndex)
        maybeRefill()
    }

    private fun publishLikeState() {
        val session = librarySession ?: return
        val trackId = currentTrack()?.trackId
        val isLiked = trackId != null && liked[trackId] == true
        session.setMediaButtonPreferences(buttons(isLiked))
        session.setSessionExtras(
            Bundle().apply {
                putString(Commands.EXTRA_TRACK_ID, trackId)
                putBoolean(Commands.EXTRA_LIKED, isLiked)
            },
        )
    }

    private fun buttons(liked: Boolean): List<CommandButton> = listOf(
        CommandButton.Builder(if (liked) CommandButton.ICON_HEART_FILLED else CommandButton.ICON_HEART_UNFILLED)
            .setDisplayName(getString(if (liked) R.string.player_unlike else R.string.player_like))
            .setSessionCommand(Commands.LIKE)
            .setSlots(CommandButton.SLOT_BACK_SECONDARY)
            .build(),
        CommandButton.Builder(CommandButton.ICON_BLOCK)
            .setDisplayName(getString(R.string.player_ban))
            .setSessionCommand(Commands.BAN)
            .setSlots(CommandButton.SLOT_OVERFLOW)
            .build(),
    )

    private fun startProgressChecks() {
        progressJob?.cancel()
        progressJob = scope.launch {
            while (isActive) {
                checkPlayReported()
                delay(PROGRESS_CHECK_MS)
            }
        }
    }

    private fun checkPlayReported() {
        val item = player.currentMediaItem ?: return
        val track = MediaIds.parseTrack(item.mediaId) ?: return
        val duration = player.duration.takeIf { it != C.TIME_UNSET } ?: item.mediaMetadata.durationMs ?: 0L
        if (!playReporter.shouldReport(item.mediaId, player.currentPosition, duration)) return
        val s = session ?: return
        scope.launch { container.playbackRepository.recordPlay(s, track.trackId, track.stationId) }
    }

    // --- Player and session callbacks ------------------------------------------------------------

    private inner class PlayerEvents : Player.Listener {
        override fun onMediaItemTransition(mediaItem: MediaItem?, reason: Int) {
            mediaItem?.mediaId?.let(MediaIds::parseTrack)?.let { stationQueue.markPlayed(it.trackId) }
            publishLikeState()
            maybeRefill()
        }

        override fun onIsPlayingChanged(isPlaying: Boolean) {
            if (isPlaying) {
                skippedErrors = 0
                startProgressChecks()
            } else {
                progressJob?.cancel()
                checkPlayReported()
            }
        }

        /**
         * A track the server cannot serve (gone, or a file it cannot decode) is skipped, at most
         * [MAX_SKIPPED_ERRORS] in a row. Network errors, 401 (signed out) and 429 (rate limited) are
         * not skipped: playback pauses, and the play button tries again.
         */
        override fun onPlayerError(error: PlaybackException) {
            val code = (error.cause as? HttpDataSource.InvalidResponseCodeException)?.responseCode
            val skippable = (code != null && code in 400..499 && code != 401 && code != 429) ||
                error.errorCode in PlaybackException.ERROR_CODE_PARSING_CONTAINER_MALFORMED..PlaybackException.ERROR_CODE_DECODING_FORMAT_UNSUPPORTED
            if (session == null || !skippable || skippedErrors >= MAX_SKIPPED_ERRORS || !player.hasNextMediaItem()) return
            skippedErrors++
            player.removeMediaItem(player.currentMediaItemIndex)
            player.prepare()
            player.play()
        }
    }

    private inner class Callback : MediaLibrarySession.Callback {
        override fun onConnect(
            session: MediaSession,
            controller: MediaSession.ControllerInfo,
        ): MediaSession.ConnectionResult = MediaSession.ConnectionResult.AcceptedResultBuilder(session)
            .setAvailableSessionCommands(
                MediaSession.ConnectionResult.DEFAULT_SESSION_AND_LIBRARY_COMMANDS.buildUpon()
                    .add(Commands.LIKE)
                    .add(Commands.BAN)
                    .build(),
            )
            .build()

        override fun onCustomCommand(
            session: MediaSession,
            controller: MediaSession.ControllerInfo,
            customCommand: SessionCommand,
            args: Bundle,
        ): ListenableFuture<SessionResult> {
            when (customCommand.customAction) {
                Commands.ACTION_LIKE -> toggleLike()
                Commands.ACTION_BAN -> ban()
                else -> return Futures.immediateFuture(SessionResult(SessionError.ERROR_NOT_SUPPORTED))
            }
            return Futures.immediateFuture(SessionResult(SessionResult.RESULT_SUCCESS))
        }

        override fun onSetMediaItems(
            mediaSession: MediaSession,
            controller: MediaSession.ControllerInfo,
            mediaItems: MutableList<MediaItem>,
            startIndex: Int,
            startPositionMs: Long,
        ): ListenableFuture<MediaSession.MediaItemsWithStartPosition> = scope.future {
            val station = mediaItems.singleOrNull()?.mediaId?.let(MediaIds::parseStation)
            if (station != null) {
                MediaSession.MediaItemsWithStartPosition(loadStation(station), 0, C.TIME_UNSET)
            } else {
                MediaSession.MediaItemsWithStartPosition(resolveTracks(mediaItems), startIndex, startPositionMs)
            }
        }

        override fun onAddMediaItems(
            mediaSession: MediaSession,
            controller: MediaSession.ControllerInfo,
            mediaItems: MutableList<MediaItem>,
        ): ListenableFuture<MutableList<MediaItem>> = Futures.immediateFuture(resolveTracks(mediaItems).toMutableList())

        override fun onGetLibraryRoot(
            session: MediaLibrarySession,
            browser: MediaSession.ControllerInfo,
            params: LibraryParams?,
        ): ListenableFuture<LibraryResult<MediaItem>> = Futures.immediateFuture(
            LibraryResult.ofItem(
                MediaItem.Builder()
                    .setMediaId(MediaIds.ROOT)
                    .setMediaMetadata(MediaMetadata.Builder().setIsBrowsable(true).setIsPlayable(false).build())
                    .build(),
                params,
            ),
        )

        // The browse tree for Android Auto comes in PR 3.
        override fun onGetChildren(
            session: MediaLibrarySession,
            browser: MediaSession.ControllerInfo,
            parentId: String,
            page: Int,
            pageSize: Int,
            params: LibraryParams?,
        ): ListenableFuture<LibraryResult<ImmutableList<MediaItem>>> =
            Futures.immediateFuture(LibraryResult.ofItemList(ImmutableList.of(), params))
    }

    /** Items sent by a controller lose their URI; give OwnWave tracks theirs back and drop anything else. */
    private fun resolveTracks(items: List<MediaItem>): List<MediaItem> = items.mapNotNull { item ->
        val track = MediaIds.parseTrack(item.mediaId) ?: return@mapNotNull null
        item.buildUpon().setUri(MediaIds.trackUri(track.trackId)).build()
    }

    /** ExoPlayer retries failed loads by default. A 4xx will not get better, and bursts of them get IPs banned. */
    private class NoRetryOnClientErrors : DefaultLoadErrorHandlingPolicy() {
        override fun getRetryDelayMsFor(loadErrorInfo: LoadErrorHandlingPolicy.LoadErrorInfo): Long {
            val code = (loadErrorInfo.exception as? HttpDataSource.InvalidResponseCodeException)?.responseCode
            return if (code != null && code in 400..499) C.TIME_UNSET else super.getRetryDelayMsFor(loadErrorInfo)
        }
    }

    private companion object {
        // About 30 s to 2 min ahead: enough for tunnels in the car without fetching whole tracks on mobile data.
        const val MIN_BUFFER_MS = 30_000
        const val MAX_BUFFER_MS = 120_000
        const val START_BUFFER_MS = 2_500
        const val REBUFFER_MS = 5_000
        const val PROGRESS_CHECK_MS = 1_000L
        const val MAX_SKIPPED_ERRORS = 3
    }
}
