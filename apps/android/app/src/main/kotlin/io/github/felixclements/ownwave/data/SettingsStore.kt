package io.github.felixclements.ownwave.data

import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import io.github.felixclements.ownwave.api.models.Quality
import io.github.felixclements.ownwave.playback.QualitySettings
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn

/**
 * App settings in DataStore. [settings] is a StateFlow, so the player's stream resolver can read the
 * current quality on its loader thread without suspending.
 */
class SettingsStore(private val dataStore: DataStore<Preferences>, scope: CoroutineScope) {

    val settings: StateFlow<QualitySettings> = dataStore.data
        .map { prefs ->
            val defaults = QualitySettings()
            QualitySettings(
                wifi = prefs[WIFI].toQuality() ?: defaults.wifi,
                mobile = prefs[MOBILE].toQuality() ?: defaults.mobile,
            )
        }
        .stateIn(scope, SharingStarted.Eagerly, QualitySettings())

    val current: QualitySettings get() = settings.value

    suspend fun setWifiQuality(quality: Quality) {
        dataStore.edit { it[WIFI] = quality.value }
    }

    suspend fun setMobileQuality(quality: Quality) {
        dataStore.edit { it[MOBILE] = quality.value }
    }

    private fun String?.toQuality(): Quality? = Quality.entries.firstOrNull { it.value == this }

    private companion object {
        val WIFI = stringPreferencesKey("quality_wifi")
        val MOBILE = stringPreferencesKey("quality_mobile")
    }
}
