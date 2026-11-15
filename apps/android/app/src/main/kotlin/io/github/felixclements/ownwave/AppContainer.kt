package io.github.felixclements.ownwave

import android.content.Context
import androidx.datastore.preferences.preferencesDataStore
import io.github.felixclements.ownwave.auth.AccountRepository
import io.github.felixclements.ownwave.auth.AuthInterceptor
import io.github.felixclements.ownwave.auth.DeviceRepository
import io.github.felixclements.ownwave.auth.SessionStore
import io.github.felixclements.ownwave.auth.TokenCipher
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.ApiFactory
import io.github.felixclements.ownwave.data.PlaybackRepository
import io.github.felixclements.ownwave.data.ResumeStore
import io.github.felixclements.ownwave.data.SettingsStore
import io.github.felixclements.ownwave.data.StationRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import okhttp3.Cache
import okhttp3.OkHttpClient
import java.io.File

private val Context.sessionDataStore by preferencesDataStore(name = "session")
private val Context.settingsDataStore by preferencesDataStore(name = "settings")
private val Context.resumeDataStore by preferencesDataStore(name = "resume")

/** The app's long-lived objects, built once in [OwnWaveApp]. */
class AppContainer(context: Context) {
    val appScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    val sessionStore = SessionStore(context.sessionDataStore, TokenCipher(), appScope)
    val settingsStore = SettingsStore(context.settingsDataStore, appScope)
    val resumeStore = ResumeStore(context.resumeDataStore)

    /**
     * Shared by Retrofit, Coil, ExoPlayer and the artwork provider.
     * Redirects are not followed: a redirect means a login gate, not OwnWave.
     */
    val httpClient: OkHttpClient = OkHttpClient.Builder()
        .followRedirects(false)
        .followSslRedirects(false)
        .cache(Cache(File(context.cacheDir, "http"), 50L * 1024 * 1024))
        .addInterceptor(
            AuthInterceptor(session = { sessionStore.current }) { dead ->
                appScope.launch { if (sessionStore.current == dead) sessionStore.clear() }
            },
        )
        .build()

    val apiFactory = ApiFactory(httpClient)
    private val apis = apiFactory

    val accountRepository = AccountRepository(apis, allowHttp = BuildConfig.DEBUG)
    val deviceRepository = DeviceRepository(apis)
    val stationRepository = StationRepository(apis)
    val playbackRepository = PlaybackRepository(apis)

    /** Revokes the device token when the server is reachable, then forgets the session. The player stops itself. */
    suspend fun signOut(session: Session) {
        accountRepository.logout(session)
        sessionStore.clear()
    }
}
