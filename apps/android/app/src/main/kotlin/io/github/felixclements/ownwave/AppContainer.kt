package io.github.felixclements.ownwave

import android.content.Context
import androidx.datastore.preferences.preferencesDataStore
import io.github.felixclements.ownwave.auth.AccountRepository
import io.github.felixclements.ownwave.auth.AuthInterceptor
import io.github.felixclements.ownwave.auth.SessionStore
import io.github.felixclements.ownwave.auth.TokenCipher
import io.github.felixclements.ownwave.data.ApiFactory
import io.github.felixclements.ownwave.data.StationRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import okhttp3.Cache
import okhttp3.OkHttpClient
import java.io.File

private val Context.sessionDataStore by preferencesDataStore(name = "session")

/** The app's long-lived objects, built once in [OwnWaveApp]. */
class AppContainer(context: Context) {
    val appScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    val sessionStore = SessionStore(context.sessionDataStore, TokenCipher(), appScope)

    /**
     * Shared by Retrofit, Coil and (later) ExoPlayer and the artwork provider.
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

    private val apis = ApiFactory(httpClient)

    val accountRepository = AccountRepository(apis, allowHttp = BuildConfig.DEBUG)
    val stationRepository = StationRepository(apis)
}
