package io.github.felixclements.ownwave.data

import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.auth.Session
import java.io.IOException

class StationRepository(private val apis: ApiFactory) {

    /** Shared stations first, as the server orders them. */
    suspend fun stations(session: Session): Result<List<Station>> = try {
        val response = apis.forServer(session.server).stations.listStations()
        val body = response.body()
        if (response.isSuccessful && body != null) {
            Result.success(body.stations)
        } else {
            Result.failure(IOException("HTTP ${response.code()}"))
        }
    } catch (e: IOException) {
        Result.failure(e)
    } catch (e: kotlinx.serialization.SerializationException) {
        Result.failure(e)
    }

    fun coverUrl(session: Session, stationId: String): String =
        session.server.newBuilder().addPathSegments("api/stations/$stationId/cover").build().toString()
}
