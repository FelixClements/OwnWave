package io.github.felixclements.ownwave.artwork

/**
 * Cover URIs that Android Auto (and the notification) can load without the Bearer token.
 *
 * `content://<AUTHORITY>/<kind>/<id>` maps one to one to the server's `api/<kind>/<id>/cover`, where
 * kind is `stations`, `playlists` or `tracks`. [ArtworkProvider] serves them. Anything else is refused,
 * so the provider can never be used to read other server paths.
 */
object ArtworkUris {
    const val AUTHORITY = "io.github.felixclements.ownwave.artwork"

    private val KINDS = setOf("stations", "playlists", "tracks")
    private val ID = Regex("[A-Za-z0-9_-]{1,64}")
    private val SERVER_PATH = Regex("/?api/([a-z]+)/([^/]+)/cover")

    fun station(id: String): String? = of("stations", id)

    fun track(id: String): String? = of("tracks", id)

    /** The content URI for a server-relative `artwork_url` such as `/api/stations/<id>/cover`, or null. */
    fun fromServerPath(path: String?): String? {
        val match = path?.let(SERVER_PATH::matchEntire) ?: return null
        return of(match.groupValues[1], match.groupValues[2])
    }

    /** The server path to fetch for a content URI's path segments (`[kind, id]`), or null if not allowed. */
    fun serverPath(segments: List<String>): String? {
        if (segments.size != 2) return null
        val (kind, id) = segments
        return if (allowed(kind, id)) "api/$kind/$id/cover" else null
    }

    private fun of(kind: String, id: String): String? =
        if (allowed(kind, id)) "content://$AUTHORITY/$kind/$id" else null

    private fun allowed(kind: String, id: String) = kind in KINDS && ID.matches(id)
}
