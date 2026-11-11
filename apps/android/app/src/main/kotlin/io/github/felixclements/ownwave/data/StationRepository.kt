package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.api.models.BrowseItem
import io.github.felixclements.ownwave.api.models.SearchHit
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.auth.Session
import retrofit2.Response
import java.io.IOException

/** Station lists, the browse tree and station search. Failures are returned, never retried. */
class StationRepository(private val apis: ApiFactory) {

    /** Shared stations first, as the server orders them. */
    suspend fun stations(session: Session): Result<List<Station>> =
        call { apis.forServer(session.server).stations.listStations() }.map { it.stations }

    /** One node of `/api/browse` (`root`, `stations`, `recent`, `genres`, `playlists`, `genre:<id>`, `playlist:<id>`). */
    suspend fun browse(session: Session, node: String): Result<List<BrowseItem>> =
        call { apis.forServer(session.server).stations.browse(node) }.map { it.items }

    /** Recently played stations (`/api/browse?node=recent`), newest first. Only items with a station. */
    suspend fun recent(session: Session): Result<List<BrowseItem>> =
        browse(session, "recent").map { items -> items.filter { it.playable && it.stationId != null } }

    /** Stations matching a name, genre, playlist or artist (`/api/search/stations`), best first, at most 10. */
    suspend fun searchStations(session: Session, query: String): Result<List<SearchHit>> {
        val q = query.trim().take(MAX_QUERY)
        if (q.isEmpty()) return Result.success(emptyList())
        return call { apis.forServer(session.server).stations.searchStations(q) }.map { it.results }
    }

    fun coverUrl(session: Session, stationId: String): String =
        session.server.newBuilder().addPathSegments("api/stations/$stationId/cover").build().toString()

    private inline fun <T> call(request: () -> Response<T>): Result<T> = try {
        val response = request()
        val body = response.body()
        if (response.isSuccessful && body != null) {
            Result.success(body)
        } else {
            Result.failure(IOException("HTTP ${response.code()}"))
        }
    } catch (e: IOException) {
        Result.failure(e)
    } catch (e: kotlinx.serialization.SerializationException) {
        Result.failure(e)
    }

    private companion object {
        /** The server rejects longer queries. */
        const val MAX_QUERY = 200
    }
}
