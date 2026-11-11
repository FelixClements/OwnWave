package io.github.felixclements.ownwave.data

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import kotlinx.coroutines.flow.first

/** Where playback stopped: the station, the track that was playing and how far into it. */
data class ResumePoint(
    val stationId: String,
    val trackId: String,
    val title: String?,
    val artist: String?,
    val album: String?,
    val durationMs: Long?,
    val positionMs: Long,
)

/**
 * The last [ResumePoint], saved on every track change and pause, so Android Auto, Bluetooth and the
 * system media controls can resume after the app was stopped. Cleared on sign-out.
 */
class ResumeStore(private val dataStore: DataStore<Preferences>) {

    suspend fun load(): ResumePoint? {
        val prefs = dataStore.data.first()
        return ResumePoint(
            stationId = prefs[STATION] ?: return null,
            trackId = prefs[TRACK] ?: return null,
            title = prefs[TITLE],
            artist = prefs[ARTIST],
            album = prefs[ALBUM],
            durationMs = prefs[DURATION],
            positionMs = prefs[POSITION] ?: 0L,
        )
    }

    suspend fun save(point: ResumePoint) {
        dataStore.edit {
            it[STATION] = point.stationId
            it[TRACK] = point.trackId
            it.setOrRemove(TITLE, point.title)
            it.setOrRemove(ARTIST, point.artist)
            it.setOrRemove(ALBUM, point.album)
            it.setOrRemove(DURATION, point.durationMs)
            it[POSITION] = point.positionMs
        }
    }

    suspend fun clear() {
        dataStore.edit { it.clear() }
    }

    private fun <T> androidx.datastore.preferences.core.MutablePreferences.setOrRemove(key: Preferences.Key<T>, value: T?) {
        if (value == null) remove(key) else set(key, value)
    }

    private companion object {
        val STATION = stringPreferencesKey("station")
        val TRACK = stringPreferencesKey("track")
        val TITLE = stringPreferencesKey("title")
        val ARTIST = stringPreferencesKey("artist")
        val ALBUM = stringPreferencesKey("album")
        val DURATION = longPreferencesKey("duration_ms")
        val POSITION = longPreferencesKey("position_ms")
    }
}
