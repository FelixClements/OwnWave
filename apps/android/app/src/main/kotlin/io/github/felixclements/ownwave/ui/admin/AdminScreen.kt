package io.github.felixclements.ownwave.ui.admin

import android.content.ClipData
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Warning
import androidx.compose.material.icons.outlined.Cancel
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CheckboxDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.ClipEntry
import androidx.compose.ui.platform.LocalClipboard
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import io.github.felixclements.ownwave.api.models.CreateInviteRequest
import io.github.felixclements.ownwave.api.models.GenreReviewItem
import io.github.felixclements.ownwave.api.models.GenreStation
import io.github.felixclements.ownwave.api.models.GenreStationSettings
import io.github.felixclements.ownwave.api.models.PipelineRun
import io.github.felixclements.ownwave.api.models.SaveGenreCorrectionRequest
import io.github.felixclements.ownwave.api.models.SetGenreStationsRequest
import io.github.felixclements.ownwave.api.models.StartScanRequest
import io.github.felixclements.ownwave.api.models.StationChange
import io.github.felixclements.ownwave.api.models.UpdateGenreStationRequest
import io.github.felixclements.ownwave.auth.Session
import io.github.felixclements.ownwave.data.apiBody
import io.github.felixclements.ownwave.data.apiCall
import io.github.felixclements.ownwave.ui.LocalNavigator
import io.github.felixclements.ownwave.ui.Route
import io.github.felixclements.ownwave.ui.common.Badge
import io.github.felixclements.ownwave.ui.common.CardBox
import io.github.felixclements.ownwave.ui.common.ConfirmDialog
import io.github.felixclements.ownwave.ui.common.ErrorText
import io.github.felixclements.ownwave.ui.common.Format
import io.github.felixclements.ownwave.ui.common.LocalAppData
import io.github.felixclements.ownwave.ui.common.OwButton
import io.github.felixclements.ownwave.ui.common.OwIconButton
import io.github.felixclements.ownwave.ui.common.OwTextField
import io.github.felixclements.ownwave.ui.common.PagePad
import io.github.felixclements.ownwave.ui.common.PageTitle
import io.github.felixclements.ownwave.ui.common.Spinner
import io.github.felixclements.ownwave.ui.common.TopSpacer
import io.github.felixclements.ownwave.ui.common.Variant
import io.github.felixclements.ownwave.ui.common.rememberGenreNames
import io.github.felixclements.ownwave.ui.common.rememberGenres
import io.github.felixclements.ownwave.ui.common.rememberMutation
import io.github.felixclements.ownwave.ui.common.rememberQuery
import io.github.felixclements.ownwave.ui.rememberPageListState
import io.github.felixclements.ownwave.ui.theme.Ow
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.UUID
import kotlin.math.roundToInt
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/** apps/web/src/pages/Admin.tsx, one card under the other as on a phone. */
@Composable
fun AdminScreen(session: Session) {
    LazyColumn(state = rememberPageListState(), modifier = Modifier.fillMaxSize()) {
        item(key = "head") {
            Column(Modifier.padding(horizontal = PagePad)) {
                TopSpacer()
                PageTitle("Admin", Modifier.padding(top = 8.dp, bottom = 16.dp))
                Health()
            }
        }
        item(key = "scans") { Section { Scans() } }
        item(key = "genres") { Section { Genres() } }
        item(key = "genre-stations") { Section { GenreStations() } }
        item(key = "review") { Section { GenreReview() } }
        item(key = "pipeline") { Section { Pipeline() } }
        item(key = "users") { Section { Users(session) } }
        item(key = "end") { Spacer(Modifier.height(32.dp)) }
    }
}

@Composable
private fun Section(content: @Composable () -> Unit) {
    Box(Modifier.padding(horizontal = PagePad).padding(top = 24.dp)) { content() }
}

// --- Health -------------------------------------------------------------------------------------

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun Health() {
    val h = rememberQuery("health") { apiBody { apis.untyped.health() }.map(AdminJson::health) }
    when {
        h.loading -> Spinner()
        h.data == null -> ErrorText(h.error)
        else -> FlowRow(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
            h.data.forEach { (name, status) ->
                val ok = status == "ok"
                Row(
                    Modifier.clip(CircleShape).background(Ow.Raised).padding(horizontal = 12.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Icon(if (ok) Icons.Default.CheckCircle else Icons.Outlined.Cancel, null, tint = if (ok) Ow.Accent else Ow.Danger, modifier = Modifier.size(16.dp))
                    Text(name.replaceFirstChar { it.uppercase() }, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    Text(if (ok) "OK" else status, fontSize = 14.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
}

// --- Scans --------------------------------------------------------------------------------------

@Composable
private fun Scans() {
    val scans = rememberQuery("scans", pollMs = { list: List<ScanSummary> -> if (list.any { !it.finished }) 3000L else null }) {
        apiBody { apis.untyped.scans() }.map(AdminJson::scans)
    }
    val start = rememberMutation()
    var confirmFull by remember { mutableStateOf(false) }
    val latest = scans.data?.firstOrNull()
    val running = latest != null && !latest.finished
    fun scan(force: Boolean) = start.run<Any?>(tag = force) { apiCall { apis.untyped.startScan(StartScanRequest(force = force)) } }
    CardBox("Library") {
        Column(verticalArrangement = Arrangement.spacedBy(16.dp)) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OwButton("Scan for changes", { scan(false) }, variant = Variant.Primary, small = true, enabled = !running, loading = start.pending && start.tag == false)
                OwButton("Full rescan", { confirmFull = true }, small = true, enabled = !running, loading = start.pending && start.tag == true)
            }
            ErrorText(start.error)
            if (running) Progress(latest)
            when {
                scans.loading -> Spinner()
                scans.data == null -> ErrorText(scans.error)
                scans.data.isEmpty() -> Text("No scans yet.", fontSize = 14.sp, color = Ow.Muted)
                else -> Column {
                    Row(Modifier.padding(bottom = 8.dp)) {
                        TableHead("Started", Modifier.weight(1f))
                        TableHead("Status", Modifier.weight(1.4f))
                        TableHead("Imported", Modifier.width(64.dp), end = true)
                        TableHead("Failed", Modifier.width(52.dp), end = true)
                    }
                    scans.data.take(8).forEach { s ->
                        HorizontalDivider(color = Ow.Line)
                        Row(Modifier.padding(vertical = 8.dp)) {
                            Text(s.startedAt?.let(Format::timeAgo) ?: "–", fontSize = 13.sp, modifier = Modifier.weight(1f))
                            Text(s.phase().label, fontSize = 13.sp, color = if (s.status == "failed") Ow.Danger else Ow.Ink, modifier = Modifier.weight(1.4f))
                            Text("${s.imported}", fontSize = 13.sp, modifier = Modifier.width(64.dp), textAlign = androidx.compose.ui.text.style.TextAlign.End)
                            Text("${s.failedFiles}", fontSize = 13.sp, modifier = Modifier.width(52.dp), textAlign = androidx.compose.ui.text.style.TextAlign.End)
                        }
                    }
                }
            }
        }
    }
    if (confirmFull) {
        ConfirmDialog(
            "Re-import and re-analyse every file? This takes hours on a large library.",
            "Full rescan",
            {
                confirmFull = false
                scan(true)
            },
            { confirmFull = false },
        )
    }
}

@Composable
private fun TableHead(text: String, modifier: Modifier, end: Boolean = false) {
    Text(
        text.uppercase(),
        fontSize = 11.sp,
        letterSpacing = 0.8.sp,
        color = Ow.Muted,
        modifier = modifier,
        textAlign = if (end) androidx.compose.ui.text.style.TextAlign.End else androidx.compose.ui.text.style.TextAlign.Start,
    )
}

@Composable
private fun Progress(scan: ScanSummary) {
    val phase = scan.phase()
    Column(Modifier.fillMaxWidth().clip(RoundedCornerShape(8.dp)).background(Ow.Raised).padding(16.dp)) {
        Text(phase.label, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
        Text(
            "${Format.plural(scan.imported, "track")} imported · ${scan.modelSuccess} analysed",
            fontSize = 13.sp,
            color = Ow.Muted,
            modifier = Modifier.padding(top = 2.dp, bottom = 8.dp),
        )
        val p = phase.progress
        if (p != null) {
            LinearProgressIndicator(progress = { p.toFloat() }, Modifier.fillMaxWidth().height(6.dp).clip(CircleShape), color = Ow.Accent, trackColor = Ow.Base, drawStopIndicator = {})
        } else {
            LinearProgressIndicator(Modifier.fillMaxWidth().height(6.dp).clip(CircleShape), color = Ow.Accent, trackColor = Ow.Base)
        }
    }
}

// --- Genres -------------------------------------------------------------------------------------

@Composable
private fun Genres() {
    val app = LocalAppData.current
    val scope = rememberCoroutineScope()
    val genres = rememberGenres()
    var showReport by remember { mutableStateOf(false) }
    val report = rememberQuery("genre-report", enabled = showReport) { apiBody { apis.untyped.genreReport() }.map(AdminJson::report) }
    var refreshing by remember { mutableStateOf(false) }
    CardBox(
        "Genres",
        action = {
            OwButton(
                "Refresh genres",
                {
                    refreshing = true
                    scope.launch {
                        apiCall { app.apis.untyped.refreshGenres() }
                        refreshing = false
                        delay(5000)
                        app.invalidate()
                    }
                },
                small = true,
                loading = refreshing,
                icon = Icons.Default.Refresh,
            )
        },
    ) {
        Column {
            val data = genres.data
            when {
                genres.loading -> Spinner()
                data == null -> ErrorText(genres.error)
                else -> {
                    data.warnings.forEach { w ->
                        Row(
                            Modifier.fillMaxWidth().padding(bottom = 16.dp).clip(RoundedCornerShape(6.dp)).background(Ow.Danger.copy(alpha = 0.1f)).padding(12.dp),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Icon(Icons.Default.Warning, null, tint = Ow.Danger, modifier = Modifier.size(16.dp).padding(top = 2.dp))
                            Text(
                                buildAnnotatedString {
                                    withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append("Warning: ") }
                                    append(w)
                                },
                                fontSize = 14.sp,
                            )
                        }
                    }
                    GenreChart(data)
                }
            }
            HorizontalDivider(color = Ow.Line, modifier = Modifier.padding(top = 24.dp, bottom = 16.dp))
            val r = report.data
            when {
                !showReport -> OwButton("Check the audio model against file tags", { showReport = true }, variant = Variant.Ghost, small = true)
                report.loading -> Spinner()
                r == null -> ErrorText(report.error)
                else -> Column {
                    Text(
                        buildAnnotatedString {
                            append("The audio model alone agrees with the file tag on ")
                            withStyle(SpanStyle(fontWeight = FontWeight.Bold)) { append(r.audioAgreement?.let { "${(it * 100).roundToInt()}%" } ?: "–") }
                            append(" of ${Format.plural(r.tracks, "track")} with a single-genre tag. The final genres also use tags and MusicBrainz.")
                        },
                        fontSize = 14.sp,
                    )
                    if (r.confusions.isNotEmpty()) {
                        Text("Most common disagreements", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 16.dp, bottom = 4.dp))
                        r.confusions.take(8).forEach { c ->
                            Text("tagged ${c.tag}, audio says ${c.audio ?: "–"}: ${c.tracks}", fontSize = 14.sp, color = Ow.Muted)
                        }
                    }
                }
            }
        }
    }
}

/** Tracks per station genre as a ranked bar list (apps/web/src/components/GenreChart.tsx). */
@Composable
private fun GenreChart(data: io.github.felixclements.ownwave.api.models.GenreDistribution) {
    val rows = data.genres.filter { it.tracks > 0 }.sortedWith(compareByDescending<io.github.felixclements.ownwave.api.models.GenreDistributionGenresInner> { it.primaryTracks }.thenByDescending { it.tracks })
    val max = maxOf(1, rows.maxOfOrNull { it.primaryTracks } ?: 0)
    if (rows.isEmpty()) {
        Text("No genres scored yet. They appear after the first scan.", fontSize = 14.sp, color = Ow.Muted)
        return
    }
    Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text("Primary genre of ${Format.plural(data.scoredTracks, "scored track")}", fontSize = 13.sp, color = Ow.Muted, modifier = Modifier.padding(bottom = 8.dp))
        rows.forEach { g ->
            val share = if (data.scoredTracks > 0) (g.primaryTracks * 100.0 / data.scoredTracks).roundToInt() else 0
            Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 2.dp)) {
                Text(g.name, fontSize = 13.sp, color = if (g.station) Ow.Ink else Ow.Faint, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.width(112.dp))
                Box(Modifier.weight(1f).height(12.dp).padding(horizontal = 8.dp)) {
                    Box(
                        Modifier
                            .fillMaxWidth(g.primaryTracks.toFloat() / max)
                            .height(12.dp)
                            .clip(RoundedCornerShape(topEnd = 4.dp, bottomEnd = 4.dp))
                            .background(Ow.Bar),
                    )
                }
                Text(if (g.primaryTracks > 0) "$share%" else "secondary", fontSize = 13.sp, color = Ow.Muted, modifier = Modifier.width(64.dp), textAlign = androidx.compose.ui.text.style.TextAlign.End)
            }
        }
    }
}

// --- Genre stations -----------------------------------------------------------------------------

private fun genreState(g: GenreStation, settings: GenreStationSettings) = when (g.state) {
    GenreStation.State.ACTIVE -> "Live"
    GenreStation.State.EXCLUDED -> "Hidden by you"
    GenreStation.State.OFF -> "Off"
    GenreStation.State.BELOW_THRESHOLD -> "Needs ${Format.plural(settings.showAt - g.tracks, "more track")}"
}

@Composable
private fun GenreStations() {
    val q = rememberQuery("genre-stations") { apiBody { apis.admin.getGenreStations() } }
    val setEnabled = rememberMutation()
    val settings = q.data
    CardBox(
        "Genre stations",
        action = settings?.let {
            {
                Row(
                    Modifier.clip(RoundedCornerShape(6.dp)).clickable(enabled = !setEnabled.pending) {
                        setEnabled.run<Any?> { apiCall { apis.admin.setGenreStations(SetGenreStationsRequest(enabled = !it.enabled)) } }
                    },
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Checkbox(
                        checked = it.enabled,
                        onCheckedChange = null,
                        colors = CheckboxDefaults.colors(checkedColor = Ow.Accent, checkmarkColor = Color.Black, uncheckedColor = Ow.Muted),
                    )
                    Text("A station for every main genre", fontSize = 14.sp, modifier = Modifier.padding(start = 4.dp, end = 4.dp))
                }
            }
        },
    ) {
        when {
            q.loading -> Spinner()
            settings == null -> ErrorText(q.error)
            else -> Column {
                Text(
                    "Each genre station plays every song in your library with that main genre, shared with everyone. It appears once ${Format.plural(settings.showAt, "song")} have the genre and hides again below ${settings.hideBelow}. Hidden stations keep their listening history.",
                    fontSize = 14.sp,
                    color = Ow.Muted,
                )
                ErrorText(setEnabled.error)
                if (settings.genres.isEmpty()) Text("No genres yet.", fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.padding(top = 16.dp))
                settings.genres.forEach { g ->
                    HorizontalDivider(color = Ow.Line, modifier = Modifier.padding(top = 12.dp))
                    GenreStationRow(g, settings)
                }
            }
        }
    }
}

@Composable
private fun GenreStationRow(g: GenreStation, settings: GenreStationSettings) {
    val nav = LocalNavigator.current
    val update = rememberMutation()
    var editing by remember { mutableStateOf(false) }
    var name by remember(g.name) { mutableStateOf(g.name) }
    fun rename(value: String) = update.run<Any?>(tag = "name", onSuccess = { editing = false }) {
        apiCall { apis.admin.updateGenreStation(g.genre, UpdateGenreStationRequest(name = value)) }
    }
    Column(Modifier.padding(top = 12.dp)) {
        if (editing) {
            OwTextField(name, { name = it.take(100) }, label = "Name of the ${g.genreName} station", onDone = { if (name.isNotBlank()) rename(name.trim()) })
            Row(Modifier.padding(top = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OwButton("Save", { rename(name.trim()) }, variant = Variant.Primary, small = true, enabled = name.isNotBlank(), loading = update.pending && update.tag == "name")
                OwButton("Cancel", { editing = false }, variant = Variant.Ghost, small = true)
            }
        } else {
            Row(verticalAlignment = Alignment.CenterVertically) {
                val sid = g.stationId
                Text(
                    g.name,
                    fontSize = 15.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier
                        .weight(1f, fill = false)
                        .then(if (sid != null && g.state == GenreStation.State.ACTIVE) Modifier.clickable { nav.push(Route.Station(sid)) } else Modifier),
                )
                OwIconButton(Icons.Default.Edit, "Rename the ${g.genreName} station", {
                    name = g.name
                    editing = true
                }, size = 32.dp, iconSize = 14.dp)
                Spacer(Modifier.weight(1f))
                Text("${g.tracks}", fontSize = 13.sp, color = Ow.Muted, modifier = Modifier.padding(end = 12.dp))
                Text(genreState(g, settings), fontSize = 13.sp, color = if (g.state == GenreStation.State.ACTIVE) Ow.Accent else Ow.Muted)
            }
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            if (g.nameIsCustom && !editing) {
                Text(
                    "Rename back to ${g.genreName}",
                    fontSize = 12.sp,
                    color = Ow.Muted,
                    modifier = Modifier.clip(RoundedCornerShape(4.dp)).clickable { rename("") }.padding(vertical = 4.dp),
                )
            }
            Spacer(Modifier.weight(1f))
            val excluded = g.state == GenreStation.State.EXCLUDED
            OwButton(
                if (excluded) "Show ${g.genreName}" else "Hide ${g.genreName}",
                { update.run<Any?>(tag = "excluded") { apiCall { apis.admin.updateGenreStation(g.genre, UpdateGenreStationRequest(excluded = !excluded)) } } },
                variant = Variant.Ghost,
                small = true,
                loading = update.pending && update.tag == "excluded",
            )
        }
        ErrorText(update.error)
    }
}

// --- Genre review -------------------------------------------------------------------------------

@Composable
private fun GenreReview() {
    val review = rememberQuery("genre-review") { apiBody { apis.admin.getGenreReview() }.map { it.items } }
    val corrections = rememberQuery("genre-corrections") { apiBody { apis.admin.listGenreCorrections() }.map { it.corrections } }
    val genreData = rememberGenres()
    val genreName = rememberGenreNames()
    val remove = rememberMutation()
    val genres = genreData.data?.genres.orEmpty().filter { it.station }.map { it.genre to it.name }
    CardBox("Genre review") {
        Column {
            Text(
                "Songs most worth checking: those people hid on a genre station, then those whose copies disagree, then the least certain. A correction replaces the scored genre at the next scoring, which starts right away.",
                fontSize = 14.sp,
                color = Ow.Muted,
            )
            when {
                review.loading -> Spinner()
                else -> {
                    ErrorText(review.error)
                    val items = review.data.orEmpty()
                    if (review.data != null && items.isEmpty()) Text("Nothing to check.", fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.padding(vertical = 16.dp))
                    // The web caps this list's height; on a phone the page scrolls, so show the first 20.
                    items.take(20).forEachIndexed { i, item ->
                        if (i > 0) HorizontalDivider(color = Ow.Line)
                        GenreReviewRow(item, genres, genreName)
                    }
                }
            }
            HorizontalDivider(color = Ow.Line, modifier = Modifier.padding(vertical = 16.dp))
            val list = corrections.data.orEmpty()
            Text("Corrections" + if (list.isNotEmpty()) " (${list.size})" else "", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(bottom = 8.dp))
            ErrorText(corrections.error ?: remove.error)
            if (corrections.data != null && list.isEmpty()) Text("None yet.", fontSize = 14.sp, color = Ow.Muted)
            list.forEachIndexed { i, c ->
                if (i > 0) HorizontalDivider(color = Ow.Line)
                val what = c.artist?.let { "Everything by $it" } ?: "${c.trackTitle} · ${c.trackArtist ?: "Unknown artist"}"
                Row(Modifier.padding(vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(what, fontSize = 14.sp, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    Text(genreName(c.genre), fontSize = 14.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(horizontal = 8.dp))
                    OwIconButton(
                        Icons.Default.Delete,
                        "Remove the correction of $what",
                        { remove.run<Any?>(tag = c.id) { apiCall { apis.admin.deleteGenreCorrection(UUID.fromString(c.id)) } } },
                        enabled = !remove.pending,
                        size = 32.dp,
                        iconSize = 16.dp,
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun GenreReviewRow(item: GenreReviewItem, genres: List<Pair<String, String>>, genreName: (String) -> String) {
    val save = rememberMutation()
    var genre by remember(item.trackId, item.genre) { mutableStateOf(item.genre) }
    var open by remember { mutableStateOf(false) }
    Column(Modifier.padding(vertical = 12.dp)) {
        Text(item.title, fontSize = 14.sp, fontWeight = FontWeight.Medium, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(item.artist ?: "Unknown artist", fontSize = 14.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
        if (item.hiddenBy > 0 || item.copiesDisagree) {
            Row(Modifier.padding(top = 4.dp), horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                if (item.hiddenBy > 0) Badge("Hidden by ${Format.plural(item.hiddenBy, "listener")}")
                if (item.copiesDisagree) Badge("Copies disagree")
            }
        }
        FlowRow(
            Modifier.padding(top = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
            itemVerticalAlignment = Alignment.CenterVertically,
        ) {
            Box {
                Row(
                    Modifier
                        .height(36.dp)
                        .clip(RoundedCornerShape(6.dp))
                        .border(1.dp, Ow.Line, RoundedCornerShape(6.dp))
                        .background(Ow.Card)
                        .clickable { open = true }
                        .padding(start = 10.dp, end = 4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(genreName(genre), fontSize = 14.sp)
                    Icon(Icons.Default.ExpandMore, null, tint = Ow.Muted, modifier = Modifier.size(18.dp))
                }
                DropdownMenu(expanded = open, onDismissRequest = { open = false }, containerColor = Ow.Raised) {
                    genres.forEach { (id, name) ->
                        DropdownMenuItem(text = { Text(name, color = if (id == genre) Ow.Accent else Ow.Ink) }, onClick = {
                            genre = id
                            open = false
                        })
                    }
                }
            }
            OwButton(
                if (genre == item.genre) "Looks right" else "Set for this song",
                { save.run<Any?>(tag = "song") { apiCall { apis.admin.saveGenreCorrection(SaveGenreCorrectionRequest(genre = genre, trackId = item.trackId)) } } },
                small = true,
                loading = save.pending && save.tag == "song",
            )
            val artist = item.artist
            if (artist != null && genre != item.genre) {
                OwButton(
                    "Set for this artist",
                    { save.run<Any?>(tag = "artist") { apiCall { apis.admin.saveGenreCorrection(SaveGenreCorrectionRequest(genre = genre, artist = artist)) } } },
                    small = true,
                    loading = save.pending && save.tag == "artist",
                )
            }
        }
        ErrorText(save.error)
    }
}

// --- Pipeline -----------------------------------------------------------------------------------

private val triggers = mapOf(
    "startup" to "API started",
    "reconnect" to "Reconnected",
    "hourly" to "Hourly check",
    "admin" to "Refresh now",
    "settings" to "Settings changed",
    "playlist_added" to "Playlist added",
    "playlist_synced" to "Playlist synced",
    "playlist_deleted" to "Playlist removed",
    "scan_finished" to "Scan finished",
    "genres_scored" to "Genres scored",
)

private fun StationChange.Change.verb() = when (this) {
    StationChange.Change.CREATED -> "created"
    StationChange.Change.SHOWN -> "shown again"
    StationChange.Change.HIDDEN -> "hidden"
    StationChange.Change.RENAMED -> "renamed"
}

@Composable
private fun Pipeline() {
    val q = rememberQuery("pipeline", pollMs = { _: io.github.felixclements.ownwave.api.models.PipelineStatus -> 30_000L }) { apiBody { apis.admin.getPipeline() } }
    val refresh = rememberMutation()
    val status = q.data
    CardBox(
        "Keeping stations up to date",
        action = { OwButton("Refresh now", { refresh.run<Any?> { apiCall { apis.admin.refreshPipeline() } } }, small = true, loading = refresh.pending, icon = Icons.Default.Refresh) },
    ) {
        Column {
            Text(
                "After a scan and genre scoring, after playlist changes, at startup and every hour, OwnWave matches playlist songs to your files and brings the genre stations up to date.",
                fontSize = 14.sp,
                color = Ow.Muted,
            )
            ErrorText(refresh.error)
            when {
                q.loading -> Spinner()
                status == null -> ErrorText(q.error)
                else -> {
                    Column(Modifier.padding(top = 16.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                        Fact("Last scan", status.lastScan?.let { "${it.status}, ${Format.timeAgo(it.finishedAt ?: it.createdAt)}" } ?: "Never")
                        Fact(
                            "Genres scored",
                            status.genresScored?.let { "${Format.timeAgo(it.at)}, ${Format.plural(it.scored, "track")} of ${"%,d".format(it.tracks)}" } ?: "Not yet",
                        )
                        Fact("Stations refreshed", status.runs.firstOrNull()?.let { Format.timeAgo(it.finishedAt ?: it.startedAt) } ?: "Not yet")
                    }
                    if (status.runs.isNotEmpty()) {
                        Text("Recent refreshes", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(top = 24.dp, bottom = 4.dp))
                        status.runs.take(12).forEachIndexed { i, run ->
                            if (i > 0) HorizontalDivider(color = Ow.Line)
                            RunRow(run)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun Fact(label: String, value: String) {
    Row {
        Text(label, fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.width(140.dp))
        Text(value, fontSize = 14.sp, modifier = Modifier.weight(1f))
    }
}

@Composable
private fun RunRow(run: PipelineRun) {
    val parts = buildList {
        run.matched?.takeIf { it > 0 }?.let { add("${Format.plural(it, "track")} matched") }
        run.changes.forEach { add("${it.name} ${it.change.verb()}") }
    }
    Column(Modifier.padding(vertical = 8.dp)) {
        Row {
            Text(Format.timeAgo(run.finishedAt ?: run.startedAt), fontSize = 13.sp, color = Ow.Muted, modifier = Modifier.width(96.dp))
            Text(triggers[run.trigger] ?: run.trigger.replace('_', ' '), fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
        }
        Text(
            run.error ?: if (parts.isNotEmpty()) parts.joinToString(" · ") else "Nothing changed",
            fontSize = 13.sp,
            color = when {
                run.error != null -> Ow.Danger
                parts.isEmpty() -> Ow.Muted
                else -> Ow.Ink
            },
        )
    }
}

// --- People -------------------------------------------------------------------------------------

@Composable
private fun Users(session: Session) {
    val clipboard = LocalClipboard.current
    val scope = rememberCoroutineScope()
    val users = rememberQuery("users") { apiBody { apis.admin.listUsers() }.map { it.users.orEmpty() } }
    val delete = rememberMutation()
    val create = rememberMutation(invalidate = false)
    var username by remember { mutableStateOf("") }
    var invite by remember { mutableStateOf<Invite?>(null) }
    var copied by remember { mutableStateOf(false) }
    var deleting by remember { mutableStateOf<io.github.felixclements.ownwave.api.models.User?>(null) }
    fun createInvite() = create.run<Invite>(onSuccess = {
        invite = it
        copied = false
    }) {
        apiBody { apis.untyped.createInvite(CreateInviteRequest(username = username.trim().ifEmpty { null })) }.mapCatching { AdminJson.invite(it) }
    }
    CardBox("People") {
        Column {
            users.data.orEmpty().forEachIndexed { i, u ->
                if (i > 0) HorizontalDivider(color = Ow.Line)
                Row(Modifier.padding(vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(32.dp).clip(CircleShape).background(Ow.Raised), contentAlignment = Alignment.Center) {
                        Text(u.username.take(1).uppercase(), fontSize = 14.sp, fontWeight = FontWeight.Bold)
                    }
                    Text(
                        buildAnnotatedString {
                            append(u.username)
                            if (u.isAdmin) withStyle(SpanStyle(color = Ow.Accent, fontSize = 12.sp)) { append(" · admin") }
                        },
                        fontSize = 15.sp,
                        modifier = Modifier.weight(1f).padding(horizontal = 12.dp),
                    )
                    if (u.id != session.userId) {
                        OwIconButton(Icons.Default.Delete, "Delete ${u.username}", { deleting = u }, size = 32.dp, iconSize = 16.dp)
                    }
                }
            }
            ErrorText(users.error ?: delete.error)
            Spacer(Modifier.height(16.dp))
            OwTextField(username, { username = it }, label = "Invite someone", placeholder = "Username (optional)", onDone = ::createInvite)
            OwButton("Create invite link", ::createInvite, Modifier.padding(top = 12.dp), variant = Variant.Primary, loading = create.pending)
            ErrorText(create.error)
            invite?.let { inv ->
                Row(
                    Modifier.padding(top = 16.dp).fillMaxWidth().clip(RoundedCornerShape(6.dp)).background(Ow.Raised).padding(12.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column(Modifier.weight(1f)) {
                        Text(inv.url, fontSize = 13.sp, fontFamily = FontFamily.Monospace, maxLines = 2, overflow = TextOverflow.Ellipsis)
                        inv.expiresAt?.let {
                            Text("expires ${it.format(DateTimeFormatter.ofLocalizedDate(FormatStyle.MEDIUM))}", fontSize = 12.sp, color = Ow.Muted)
                        }
                    }
                    OwIconButton(
                        Icons.Default.ContentCopy,
                        if (copied) "Copied" else "Copy link",
                        {
                            scope.launch {
                                clipboard.setClipEntry(ClipEntry(ClipData.newPlainText("Invite link", inv.url)))
                                copied = true
                            }
                        },
                        active = copied,
                        size = 32.dp,
                        iconSize = 16.dp,
                    )
                }
            }
        }
    }
    deleting?.let { u ->
        ConfirmDialog(
            "Delete ${u.username} and their history?",
            "Delete",
            {
                deleting = null
                delete.run<Unit?> { apiCall { apis.admin.deleteUser(UUID.fromString(u.id)) } }
            },
            { deleting = null },
        )
    }
}

