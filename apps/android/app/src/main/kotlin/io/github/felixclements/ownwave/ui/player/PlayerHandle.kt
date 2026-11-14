package io.github.felixclements.ownwave.ui.player

import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.staticCompositionLocalOf
import io.github.felixclements.ownwave.api.models.FeedbackRequest
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.common.AppData
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import java.util.UUID

/**
 * What pages need from the player (the web's `usePlayer()`): what plays, play a station, and
 * like, ban or hide any track, not only the current one.
 */
class PlayerHandle(
    private val vm: PlayerViewModel,
    private val stateRef: State<PlayerUiState>,
    private val app: AppData,
    private val scope: CoroutineScope,
    val expand: () -> Unit,
) {
    val state: PlayerUiState get() = stateRef.value

    /** Like state the user changed here, over what the server last said. */
    private val likes = mutableStateMapOf<String, Boolean>()

    fun isActive(stationId: String) = state.stationId == stationId

    fun isPlaying(stationId: String) = isActive(stationId) && state.isPlaying

    /** Plays the station, or pauses and resumes it when it already plays. */
    fun playOrToggle(stationId: String, name: String) {
        if (isActive(stationId)) vm.togglePlay() else vm.play(stationId, name)
    }

    fun toggle() = vm.togglePlay()
    fun next() = vm.next()
    fun previous() = vm.previous()
    fun seekTo(ms: Long) = vm.seekTo(ms)

    private fun currentTrackId() = state.current?.trackId

    fun liked(trackId: String, fallback: Boolean = false): Boolean =
        if (trackId == currentTrackId()) state.liked else likes[trackId] ?: fallback

    fun toggleLike(trackId: String, fallback: Boolean = false) {
        if (trackId == currentTrackId()) {
            vm.toggleLike()
            return
        }
        val now = !liked(trackId, fallback)
        likes[trackId] = now
        scope.launch {
            val body = FeedbackRequest(FeedbackRequest.Feedback.LIKE)
            val id = UUID.fromString(trackId)
            apiCall { if (now) app.apis.playback.recordFeedback(id, body) else app.apis.feedback.deleteFeedback(id, body) }
                .onFailure { likes[trackId] = !now }
        }
    }

    /** Never play the track again, on any station. Skips it when it plays. */
    fun ban(trackId: String, onDone: () -> Unit = {}) {
        if (trackId == currentTrackId()) {
            vm.ban()
            onDone()
            return
        }
        scope.launch {
            apiCall { app.apis.playback.recordFeedback(UUID.fromString(trackId), FeedbackRequest(FeedbackRequest.Feedback.BAN)) }
            app.invalidate()
            onDone()
        }
    }

    /** "Not in this station". Skips the track when it plays here. */
    fun hide(stationId: String, trackId: String) {
        if (trackId == currentTrackId() && isActive(stationId)) {
            vm.hide()
            return
        }
        scope.launch {
            apiCall {
                app.apis.stations.hideTrack(
                    UUID.fromString(stationId),
                    io.github.felixclements.ownwave.api.models.HideTrackRequest(trackId = trackId),
                )
            }
            app.invalidate()
        }
    }
}

val LocalPlayer = staticCompositionLocalOf<PlayerHandle> { error("No PlayerHandle") }
