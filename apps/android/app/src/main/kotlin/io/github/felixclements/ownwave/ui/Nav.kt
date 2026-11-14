package io.github.felixclements.ownwave.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.listSaver
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.snapshots.SnapshotStateList
import androidx.compose.runtime.staticCompositionLocalOf

/** The pages of the web app (apps/web/src/App.tsx routes). */
sealed interface Route {
    data object Home : Route
    data object Search : Route
    data object Library : Route
    data object Playlists : Route
    data object Admin : Route
    data object Stations : Route
    data object Settings : Route
    data class Station(val id: String) : Route
    data class Playlist(val id: String) : Route
    data class Genre(val id: String) : Route

    /** A string form, so the back stack survives configuration changes and process death. */
    fun encode(): String = when (this) {
        Home -> "home"
        Search -> "search"
        Library -> "library"
        Playlists -> "playlists"
        Admin -> "admin"
        Stations -> "stations"
        Settings -> "settings"
        is Station -> "station/$id"
        is Playlist -> "playlist/$id"
        is Genre -> "genre/$id"
    }

    companion object {
        fun decode(s: String): Route = when {
            s.startsWith("station/") -> Station(s.removePrefix("station/"))
            s.startsWith("playlist/") -> Playlist(s.removePrefix("playlist/"))
            s.startsWith("genre/") -> Genre(s.removePrefix("genre/"))
            s == "search" -> Search
            s == "library" -> Library
            s == "playlists" -> Playlists
            s == "admin" -> Admin
            s == "stations" -> Stations
            s == "settings" -> Settings
            else -> Home
        }
    }
}

/** One back stack. A bottom-nav tab starts a new one, like the web's links. */
class Navigator(private val stack: SnapshotStateList<String>) {
    val current: Route get() = Route.decode(stack.last())

    /** The tab the stack started from: the bottom nav highlights it. */
    val root: Route get() = Route.decode(stack.first())

    val canGoBack: Boolean get() = stack.size > 1

    fun push(route: Route) {
        val r = route.encode()
        if (stack.last() != r) stack.add(r)
    }

    fun back() {
        if (stack.size > 1) stack.removeAt(stack.lastIndex)
    }

    /** Opens a tab from scratch; tapping the open tab again goes back to its top. */
    fun tab(route: Route) {
        stack.clear()
        stack.add(route.encode())
    }

    /** Replaces the current page, e.g. after deleting what it showed. */
    fun replace(route: Route) {
        stack[stack.lastIndex] = route.encode()
    }
}

@Composable
fun rememberNavigator(): Navigator {
    val stack = rememberSaveable(
        saver = listSaver<SnapshotStateList<String>, String>(save = { it.toList() }, restore = { it.toMutableStateList() }),
    ) { mutableStateListOf(Route.Home.encode()) }
    return remember(stack) { Navigator(stack) }
}

private fun List<String>.toMutableStateList() = mutableStateListOf<String>().also { it.addAll(this) }

val LocalNavigator = staticCompositionLocalOf<Navigator> { error("No Navigator") }
