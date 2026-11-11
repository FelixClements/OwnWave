package io.github.felixclements.ownwave.playback

/**
 * Media ids and URIs the player, the session and (later) Android Auto agree on.
 *
 * - A station to play: `station:<stationId>`.
 * - A queued track: `track:<trackId>|station:<stationId>`.
 * - A track's URI: `ownwave://track/<trackId>`. [StreamResolver] turns it into the stream URL when the
 *   track loads, so stored items never hold the server host, the token or a fixed quality.
 */
object MediaIds {
    const val SCHEME = "ownwave"
    const val ROOT = "root"

    data class TrackRef(val trackId: String, val stationId: String?)

    fun station(stationId: String) = "station:$stationId"

    fun track(trackId: String, stationId: String?) =
        if (stationId == null) "track:$trackId" else "track:$trackId|station:$stationId"

    fun trackUri(trackId: String) = "$SCHEME://track/$trackId"

    fun parseStation(mediaId: String): String? =
        mediaId.removePrefix("station:").takeIf { it != mediaId && it.isNotEmpty() && '|' !in it }

    fun parseTrack(mediaId: String): TrackRef? {
        if (!mediaId.startsWith("track:")) return null
        val parts = mediaId.split('|')
        val trackId = parts[0].removePrefix("track:").takeIf { it.isNotEmpty() } ?: return null
        val stationId = parts.getOrNull(1)?.removePrefix("station:")?.takeIf { it.isNotEmpty() }
        return TrackRef(trackId, stationId)
    }

    /** The track id in an `ownwave://track/<id>` URI, or null for any other URI. */
    fun trackIdFromUri(uri: String): String? =
        uri.removePrefix("$SCHEME://track/").takeIf { it != uri && it.isNotEmpty() && '/' !in it }
}
