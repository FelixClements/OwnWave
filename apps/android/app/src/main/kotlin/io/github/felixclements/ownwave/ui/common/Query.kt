package io.github.felixclements.ownwave.ui.common

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.ApiFactory
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import java.util.concurrent.ConcurrentHashMap

/**
 * What every signed-in screen needs: the session, its APIs, and a small cache. Mutations call
 * [invalidate], and every query on screen refetches (like the web's react-query invalidations, only coarser).
 */
class AppData(val session: Session, val apis: ApiFactory.Apis) {
    internal val version = MutableStateFlow(0)
    internal val cache = ConcurrentHashMap<String, Any>()

    fun invalidate() = version.update { it + 1 }

    private fun url(path: String) = session.server.newBuilder().addPathSegments(path).build().toString()

    fun stationCover(id: String) = url("api/stations/$id/cover")
    fun trackCover(id: String) = url("api/tracks/$id/cover")
    fun playlistCover(id: String) = url("api/playlists/$id/cover")
}

val LocalAppData = staticCompositionLocalOf<AppData> { error("No AppData") }

class Query<T>(
    val data: T?,
    val error: String?,
    /** A fetch is running (also while showing cached data). */
    val fetching: Boolean,
    val refresh: () -> Unit,
) {
    /** Nothing to show yet. */
    val loading get() = data == null && error == null && fetching
}

/**
 * Fetches [key] when first shown, after [refresh], and after any mutation. Shows the cached value
 * meanwhile. With [pollMs], fetches again after that delay for as long as it returns non-null.
 */
@Suppress("UNCHECKED_CAST")
@Composable
fun <T : Any> rememberQuery(
    key: String,
    enabled: Boolean = true,
    pollMs: (T) -> Long? = { null },
    fetch: suspend AppData.() -> Result<T>,
): Query<T> {
    val app = LocalAppData.current
    val version by app.version.collectAsState()
    var data by remember(key) { mutableStateOf(app.cache[key] as T?) }
    var error by remember(key) { mutableStateOf<String?>(null) }
    var fetching by remember(key) { mutableStateOf(enabled) }
    var nonce by remember(key) { mutableIntStateOf(0) }
    val currentFetch by rememberUpdatedState(fetch)
    val currentPoll by rememberUpdatedState(pollMs)

    LaunchedEffect(key, version, nonce, enabled) {
        if (!enabled) {
            fetching = false
            return@LaunchedEffect
        }
        while (true) {
            fetching = true
            val result = app.currentFetch()
            fetching = false
            result.onSuccess {
                app.cache[key] = it
                data = it
                error = null
            }.onFailure { error = it.message ?: "Something went wrong." }
            val wait = result.getOrNull()?.let(currentPoll) ?: break
            delay(wait)
        }
    }
    return Query(data, error, fetching) { nonce++ }
}

class Mutation(
    val pending: Boolean,
    val error: String?,
    /** What the running or last call was about, e.g. a row id, to show its spinner. */
    val tag: Any?,
    private val start: (Any?, suspend AppData.() -> Result<*>, (Any?) -> Unit) -> Unit,
    val reset: () -> Unit,
) {
    /** Runs [block]; on success every query refetches, then [onSuccess] gets the result. */
    @Suppress("UNCHECKED_CAST")
    fun <R> run(tag: Any? = null, onSuccess: (R) -> Unit = {}, block: suspend AppData.() -> Result<R>) =
        start(tag, block) { onSuccess(it as R) }
}

@Composable
fun rememberMutation(invalidate: Boolean = true): Mutation {
    val app = LocalAppData.current
    val scope = rememberCoroutineScope()
    var pending by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var tag by remember { mutableStateOf<Any?>(null) }
    return Mutation(
        pending = pending,
        error = error,
        tag = tag,
        start = { t, block, onSuccess ->
            if (!pending) {
                pending = true
                error = null
                tag = t
                scope.launch {
                    val result = app.block()
                    pending = false
                    result.onSuccess {
                        if (invalidate) app.invalidate()
                        onSuccess(it)
                    }.onFailure { error = it.message ?: "Something went wrong." }
                }
            }
        },
        reset = { error = null },
    )
}
