package io.github.felixclements.ownwave.ui.search

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.SearchHit
import io.github.felixclements.ownwave.api.models.Station
import io.github.felixclements.ownwave.api.models.Track
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.Art
import io.github.felixclements.ownwave.ui.common.Colors
import io.github.felixclements.ownwave.ui.common.Empty
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PlayButton
import io.github.felixclements.ownwave.ui.common.SearchField
import io.github.felixclements.ownwave.ui.common.SectionHeader
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.cardGrid
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.common.rememberStations
import io.github.felixclements.ownwave.ui.common.toColor
import io.github.felixclements.ownwave.ui.player.LocalPlayer
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.stations.StationCard
import io.github.felixclements.ownwave.ui.theme.Ow
import kotlinx.coroutines.delay

private fun reason(r: SearchHit.Reason) = when (r) {
    SearchHit.Reason.STATION -> "Station"
    SearchHit.Reason.GENRE -> "Genre match"
    SearchHit.Reason.PLAYLIST -> "From a matching playlist"
    SearchHit.Reason.ARTIST -> "Has this artist"
}

/** apps/web/src/pages/Search.tsx: genres to browse, or stations, tracks and artists that match. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SearchScreen() {
    var q by rememberSaveable { mutableStateOf("") }
    var query by rememberSaveable { mutableStateOf("") }
    LaunchedEffect(q) {
        delay(250)
        query = q.trim()
    }
    val searching = query.length > 1
    val stations = rememberQuery("search-stations:$query", enabled = searching) { apiBody { apis.stations.searchStations(query) } }
    val library = rememberQuery("search-library:$query", enabled = searching) { apiBody { apis.library.search(query) } }
    val all = rememberStations().data.orEmpty()
    val byId = all.associateBy { it.id }
    val hits = stations.data?.results.orEmpty().mapNotNull { h -> h.stationId?.let(byId::get)?.let { h to it } }
    val tracks = library.data?.tracks.orEmpty()
    val artists = library.data?.artists.orEmpty()

    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "field") {
            Column(Modifier.padding(horizontal = PagePad)) {
                TopSpacer()
                SearchField(q, { q = it }, "Stations, genres, playlists, artists", Icons.Default.Search, Modifier.padding(top = 4.dp, bottom = 24.dp))
            }
        }
        when {
            !searching -> browseGenres()
            stations.loading && library.loading -> item(key = "spin") { Spinner() }
            hits.isEmpty() && tracks.isEmpty() && artists.isEmpty() && !stations.fetching && !library.fetching -> item(key = "none") {
                Empty("No results for “$query”", body = "Try a genre, a playlist name or an artist.")
            }
            else -> {
                if (hits.isNotEmpty()) {
                    item(key = "top") {
                        Column(Modifier.padding(horizontal = PagePad)) {
                            Text("Top result", fontSize = 24.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(bottom = 12.dp))
                            TopResult(hits[0].second, reason(hits[0].first.reason))
                            Text("Tracks", fontSize = 24.sp, fontWeight = FontWeight.Bold, modifier = Modifier.padding(top = 24.dp, bottom = 12.dp))
                            if (tracks.isEmpty()) Text("No tracks match.", fontSize = 14.sp, color = Ow.Muted)
                        }
                    }
                    items(tracks.take(4), key = { "t-${it.id}" }) { TrackLine(it, withAlbum = true) }
                    item(key = "top-s") { Spacer(Modifier.height(40.dp)) }
                }
                if (hits.size > 1) {
                    item(key = "st-h") { SectionHeader("Stations", Modifier.padding(horizontal = PagePad)) }
                    cardGrid(hits.drop(1), key = { "s-${it.second.id}" }) { (h, s) -> StationCard(s, reason(h.reason)) }
                    item(key = "st-s") { Spacer(Modifier.height(40.dp)) }
                }
                if (hits.isEmpty() && tracks.isNotEmpty()) {
                    item(key = "lib-h") {
                        Column(Modifier.padding(horizontal = PagePad)) {
                            SectionHeader("Tracks in your library")
                            Text("No station plays these yet: add a playlist that has them.", fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.padding(bottom = 12.dp))
                        }
                    }
                    items(tracks.take(20), key = { "l-${it.id}" }) { TrackLine(it, withAlbum = false) }
                    item(key = "lib-s") { Spacer(Modifier.height(40.dp)) }
                }
                if (artists.isNotEmpty()) {
                    item(key = "artists") {
                        Column(Modifier.padding(horizontal = PagePad)) {
                            SectionHeader("Artists")
                            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                artists.take(20).forEach { a ->
                                    Text(
                                        a.name,
                                        fontSize = 14.sp,
                                        fontWeight = FontWeight.SemiBold,
                                        modifier = Modifier
                                            .clip(CircleShape)
                                            .background(Ow.Raised)
                                            .clickable { q = a.name }
                                            .padding(horizontal = 16.dp, vertical = 8.dp),
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
}

private fun androidx.compose.foundation.lazy.LazyListScope.browseGenres() {
    item(key = "genres") { BrowseGenres() }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun BrowseGenres() {
    val nav = LocalNavigator.current
    val genres = rememberQuery("browse:genres") { apiBody { apis.stations.browse("genres") } }
    val items = genres.data?.items.orEmpty()
    when {
        genres.loading -> Spinner()
        items.isEmpty() -> Empty("Nothing to browse yet", body = genres.error ?: "Genres appear here once stations use them.")
        else -> Column(Modifier.padding(horizontal = PagePad)) {
            SectionHeader("Browse genres")
            items.chunked(2).forEach { row ->
                Row(Modifier.fillMaxWidth().padding(bottom = 16.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                    row.forEach { g ->
                        Box(
                            Modifier
                                .weight(1f)
                                .aspectRatio(16f / 10f)
                                .clip(RoundedCornerShape(8.dp))
                                .background(Colors.nameColor(g.title).toColor())
                                .clickable { nav.push(Route.Genre(g.id.removePrefix("genre:"))) }
                                .padding(16.dp),
                        ) {
                            Text(g.title, fontSize = 20.sp, lineHeight = 24.sp, fontWeight = FontWeight.Black)
                            g.subtitle?.let {
                                Text(
                                    it,
                                    fontSize = 12.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = Color.White.copy(alpha = 0.7f),
                                    modifier = Modifier.align(Alignment.BottomStart),
                                )
                            }
                        }
                    }
                    if (row.size == 1) Spacer(Modifier.weight(1f))
                }
            }
        }
    }
}

@Composable
private fun TopResult(station: Station, reason: String) {
    val app = LocalAppData.current
    val nav = LocalNavigator.current
    val player = LocalPlayer.current
    val active = player.isActive(station.id)
    Box(
        Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .background(Ow.Card)
            .clickable { nav.push(Route.Station(station.id)) }
            .padding(20.dp),
    ) {
        Column(verticalArrangement = Arrangement.spacedBy(20.dp)) {
            Art(app.stationCover(station.id), station.name, Modifier.size(96.dp).shadow(8.dp, RoundedCornerShape(6.dp)))
            Column {
                Text(station.name, fontSize = 30.sp, lineHeight = 34.sp, fontWeight = FontWeight.Black, maxLines = 2, overflow = TextOverflow.Ellipsis)
                Text(
                    reason,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.padding(top = 8.dp).clip(CircleShape).background(Ow.Base).padding(horizontal = 10.dp, vertical = 4.dp),
                )
            }
        }
        PlayButton(
            playing = active && player.state.isPlaying,
            label = "Play ${station.name}",
            onClick = { player.playOrToggle(station.id, station.name) },
            size = 48.dp,
            modifier = Modifier.align(Alignment.BottomEnd),
        )
    }
}

@Composable
private fun TrackLine(t: Track, withAlbum: Boolean) {
    val app = LocalAppData.current
    Row(
        Modifier.fillMaxWidth().padding(horizontal = PagePad - 8.dp, vertical = 0.dp).padding(8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Art(app.trackCover(t.id), t.album?.takeIf { it.isNotBlank() } ?: t.title, Modifier.size(40.dp))
        Column(Modifier.weight(1f).padding(start = 12.dp)) {
            Text(t.title, fontWeight = FontWeight.Medium, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
            val sub = if (withAlbum) listOfNotNull(t.artist, t.album).filter { it.isNotBlank() }.joinToString(" · ") else t.artist.orEmpty()
            Text(sub, fontSize = 13.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}
