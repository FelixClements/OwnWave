package io.github.felixclements.ownwave.ui.player

import android.app.Application
import android.content.ComponentName
import android.net.Uri
import android.os.Bundle
import androidx.annotation.OptIn
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MediaMetadata
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.common.util.Util
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import io.github.felixclements.ownwave.playback.Commands
import io.github.felixclements.ownwave.playback.MediaIds
import io.github.felixclements.ownwave.playback.PlaybackService
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.guava.await
import kotlinx.coroutines.launch

data class TrackUi(
    val mediaId: String,
    val title: String,
    val artist: String?,
    val artworkUri: Uri?,
    val trackId: String? = null,
    val album: String? = null,
    val durationMs: Long? = null,
)

data class PlayerUiState(
    val current: TrackUi? = null,
    val isPlaying: Boolean = false,
    val buffering: Boolean = false,
    val positionMs: Long = 0,
    /** From the player, or from the queue while the server is still transcoding (no length known yet). */
    val durationMs: Long = 0,
    /** False while the stream has no known length: the first play in a new quality streams as it transcodes. */
    val seekable: Boolean = false,
    val liked: Boolean = false,
    val upNext: List<TrackUi> = emptyList(),
    /** The station playing (or loading), from the current item's media id. */
    val stationId: String? = null,
)

/**
 * The phone UI's view of [PlaybackService], through a MediaController. The UI never owns a player,
 * so the notification, Bluetooth and the app always agree.
 */
@OptIn(UnstableApi::class)
class PlayerViewModel(app: Application) : AndroidViewModel(app) {
    private val _state = MutableStateFlow(PlayerUiState())
    val state: StateFlow<PlayerUiState> = _state.asStateFlow()

    private var controller: MediaController? = null
    private val controllerFuture = MediaController.Builder(app, SessionToken(app, ComponentName(app, PlaybackService::class.java)))
        .setListener(
            object : MediaController.Listener {
                override fun onExtrasChanged(controller: MediaController, extras: Bundle) = publish()
            },
        )
        .buildAsync()

    init {
        viewModelScope.launch {
            val c = runCatching { controllerFuture.await() }.getOrNull() ?: return@launch
            controller = c
            c.addListener(
                object : Player.Listener {
                    override fun onEvents(player: Player, events: Player.Events) = publish()
                },
            )
            publish()
            // The position is not an event; poll it while something plays.
            while (true) {
                if (c.isPlaying) _state.value = _state.value.copy(positionMs = c.currentPosition)
                delay(POSITION_POLL_MS)
            }
        }
    }

    fun play(stationId: String, name: String) {
        val c = controller ?: return
        c.setMediaItem(
            MediaItem.Builder()
                .setMediaId(MediaIds.station(stationId))
                .setMediaMetadata(MediaMetadata.Builder().setTitle(name).setStation(name).build())
                .build(),
        )
        c.prepare()
        c.play()
    }

    fun togglePlay() {
        val c = controller ?: return
        Util.handlePlayPauseButtonAction(c)
    }

    fun next() {
        controller?.seekToNextMediaItem()
    }

    /** Back to the start of the track, or to the one before when near its start. */
    fun previous() {
        controller?.seekToPrevious()
    }

    fun seekTo(positionMs: Long) {
        controller?.seekTo(positionMs)
        _state.value = _state.value.copy(positionMs = positionMs)
    }

    fun toggleLike() {
        controller?.sendCustomCommand(Commands.LIKE, Bundle.EMPTY)
    }

    fun ban() {
        controller?.sendCustomCommand(Commands.BAN, Bundle.EMPTY)
    }

    /** "Not in this station": the server stops queueing the current track here, and it skips. */
    fun hide() {
        controller?.sendCustomCommand(Commands.HIDE, Bundle.EMPTY)
    }

    private fun publish() {
        val c = controller ?: return
        val item = c.currentMediaItem
        val ref = item?.mediaId?.let(MediaIds::parseTrack)
        val trackId = ref?.trackId
        val extras = c.sessionExtras
        val index = c.currentMediaItemIndex
        _state.value = PlayerUiState(
            current = item?.takeIf { trackId != null }?.toUi(),
            isPlaying = c.isPlaying,
            buffering = c.playbackState == Player.STATE_BUFFERING,
            positionMs = c.currentPosition,
            durationMs = c.duration.takeIf { it != C.TIME_UNSET } ?: item?.mediaMetadata?.durationMs ?: 0,
            seekable = c.isCurrentMediaItemSeekable && c.duration != C.TIME_UNSET,
            liked = trackId != null && extras.getString(Commands.EXTRA_TRACK_ID) == trackId &&
                extras.getBoolean(Commands.EXTRA_LIKED),
            stationId = ref?.stationId ?: item?.mediaId?.let(MediaIds::parseStation),
            upNext = (index + 1 until minOf(c.mediaItemCount, index + 1 + UP_NEXT_COUNT)).map { c.getMediaItemAt(it).toUi() },
        )
    }

    private fun MediaItem.toUi() = TrackUi(
        mediaId = mediaId,
        title = mediaMetadata.title?.toString().orEmpty(),
        artist = mediaMetadata.artist?.toString(),
        artworkUri = mediaMetadata.artworkUri,
        trackId = MediaIds.parseTrack(mediaId)?.trackId,
        album = mediaMetadata.albumTitle?.toString(),
        durationMs = mediaMetadata.durationMs,
    )

    override fun onCleared() {
        MediaController.releaseFuture(controllerFuture)
        controller = null
    }

    private companion object {
        const val POSITION_POLL_MS = 500L
        const val UP_NEXT_COUNT = 100
    }
}
