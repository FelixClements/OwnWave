package io.github.felixclements.ownwave.playback

import io.github.felixclements.ownwave.api.models.Quality
import okhttp3.HttpUrl

/**
 * Turns `ownwave://track/<id>` into `<server>/api/stream/<id>?quality=<q>` when a track loads.
 * The server and quality are read at that moment, so a network change applies from the next track.
 */
class StreamResolver(
    private val server: () -> HttpUrl?,
    private val quality: () -> Quality,
) {
    /** The stream URL, or null when [uri] is not an OwnWave track URI or nobody is signed in. */
    fun resolve(uri: String): HttpUrl? {
        val trackId = MediaIds.trackIdFromUri(uri) ?: return null
        val base = server() ?: return null
        return base.newBuilder()
            .encodedPath("/")
            .addPathSegments("api/stream")
            .addPathSegment(trackId)
            .addQueryParameter("quality", quality().value)
            .build()
    }
}
