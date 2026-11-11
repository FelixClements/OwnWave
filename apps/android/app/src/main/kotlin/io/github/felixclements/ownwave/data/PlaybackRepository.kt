package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.api.models.FeedbackRequest
import io.github.felixclements.ownwave.api.models.QueueTrack
import io.github.felixclements.ownwave.api.models.RecordPlayRequest
import io.github.felixclements.ownwave.auth.Session
import kotlinx.serialization.SerializationException
import retrofit2.Response
import java.io.IOException
import java.util.UUID

/**
 * Station queues, plays and feedback. Nothing here retries: a failed call is reported once and left,
 * because bursts of 4xx responses get the phone's IP banned by CrowdSec.
 */
class PlaybackRepository(private val apis: ApiFactory) {

    /** Up to 200 tracks, in the order the station wants them played. */
    suspend fun queue(session: Session, stationId: String): Result<List<QueueTrack>> = call {
        apis.forServer(session.server).stations.getQueue(UUID.fromString(stationId))
    }.map { it.queue }

    suspend fun recordPlay(session: Session, trackId: String, stationId: String?): Boolean = call {
        apis.forServer(session.server).playback.recordPlay(
            UUID.fromString(trackId),
            RecordPlayRequest(stationId = stationId ?: ""),
        )
    }.isSuccess

    suspend fun setLiked(session: Session, trackId: String, liked: Boolean): Boolean = call {
        val api = apis.forServer(session.server)
        val body = FeedbackRequest(FeedbackRequest.Feedback.LIKE)
        val id = UUID.fromString(trackId)
        if (liked) api.playback.recordFeedback(id, body) else api.feedback.deleteFeedback(id, body)
    }.isSuccess

    /** Bans a track for good: the server never queues it again. */
    suspend fun ban(session: Session, trackId: String): Boolean = call {
        apis.forServer(session.server).playback.recordFeedback(
            UUID.fromString(trackId),
            FeedbackRequest(FeedbackRequest.Feedback.BAN),
        )
    }.isSuccess

    private inline fun <T : Any> call(block: () -> Response<T>): Result<T?> = try {
        val response = block()
        // A 204 has no body: Retrofit gives null for Response<Unit>.
        if (response.isSuccessful) Result.success(response.body()) else Result.failure(IOException("HTTP ${response.code()}"))
    } catch (e: IOException) {
        Result.failure(e)
    } catch (e: SerializationException) {
        Result.failure(e)
    } catch (e: IllegalArgumentException) {
        // Not a UUID: never sent.
        Result.failure(e)
    }

    private fun <T : Any> Result<T?>.map(transform: (T) -> List<QueueTrack>): Result<List<QueueTrack>> =
        fold(
            onSuccess = { body -> body?.let { Result.success(transform(it)) } ?: Result.failure(IOException("Empty body")) },
            onFailure = { Result.failure(it) },
        )
}
