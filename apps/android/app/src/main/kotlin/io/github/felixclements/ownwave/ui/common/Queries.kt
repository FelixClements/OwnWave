package io.github.felixclements.ownwave.ui.common

import androidx.compose.runtime.Composable
import io.github.felixclements.ownwave.api.models.GenreDistribution
import io.github.felixclements.ownwave.api.models.Playlist
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.data.apiBody

/** Queries several pages share (apps/web/src/lib/queries.ts). */

@Composable
fun rememberStations(): Query<List<Station>> =
    rememberQuery("stations") { apiBody { apis.stations.listStations() }.map { it.stations } }

@Composable
fun rememberPlaylists(): Query<List<Playlist>> =
    rememberQuery("playlists") { apiBody { apis.playlists.listPlaylists() }.map { it.playlists } }

@Composable
fun rememberGenres(): Query<GenreDistribution> =
    rememberQuery("genres") { apiBody { apis.genres.listGenres() } }

/** Station genre id to display name (falls back to the id). */
@Composable
fun rememberGenreNames(): (String) -> String {
    val names = rememberGenres().data?.genres?.associate { it.genre to it.name }.orEmpty()
    return { id -> names[id] ?: id.replace('_', ' ') }
}

/** "Techno, House" or "All genres". */
@Composable
fun rememberStationGenreLabel(): (Station) -> String {
    val name = rememberGenreNames()
    return { s -> if (s.genres.isEmpty()) "All genres" else s.genres.joinToString(", ") { name(it) } }
}
