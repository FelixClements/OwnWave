package io.github.felixclements.ownwave.ui.common

import android.graphics.Bitmap
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import coil3.compose.AsyncImage
import coil3.imageLoader
import coil3.request.ImageRequest
import coil3.request.SuccessResult
import coil3.request.allowHardware
import coil3.toBitmap
import io.github.felixclements.ownwave.ui.theme.Ow
import kotlin.math.roundToInt

/** Side padding of every page. */
val PagePad = 16.dp

/** Height of the top bar the shell lays over each page, plus the status bar. Pages start below it. */
val LocalTopInset = staticCompositionLocalOf { 64.dp }

fun Rgb.toColor(alpha: Float = 1f) = Color(r, g, b).copy(alpha = alpha)

// --- Artwork ------------------------------------------------------------------------------------

/**
 * Artwork: the image when it loads, else a gradient from the name with its initials, so missing
 * cover art still looks deliberate.
 */
@Composable
fun Art(url: String?, name: String, modifier: Modifier = Modifier, shape: Shape = RoundedCornerShape(6.dp)) {
    val c = remember(name) { Colors.nameColor(name) }
    BoxWithConstraints(
        modifier
            .clip(shape)
            .background(Brush.linearGradient(listOf(c.toColor(), c.scaled(1 / 3.0).toColor()))),
        contentAlignment = Alignment.Center,
    ) {
        val size = with(LocalDensity.current) { (maxWidth * 0.32f).toSp().value.coerceIn(10f, 64f).sp }
        Text(Format.initials(name), color = Color.White.copy(alpha = 0.8f), fontWeight = FontWeight.Bold, fontSize = size)
        if (url != null) {
            AsyncImage(model = url, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
    }
}

private val artColors = HashMap<String, Rgb>()

/** Colour sampled from artwork, for hero gradients; the name's colour until (or unless) it loads. */
@Composable
fun rememberArtColor(url: String?, name: String): Color {
    val context = LocalContext.current
    var rgb by remember(url, name) { mutableStateOf(url?.let { artColors[it] } ?: Colors.nameColor(name)) }
    LaunchedEffect(url, name) {
        if (url == null || artColors.containsKey(url)) return@LaunchedEffect
        val request = ImageRequest.Builder(context).data(url).size(32).allowHardware(false).build()
        val result = context.imageLoader.execute(request) as? SuccessResult ?: return@LaunchedEffect
        val bitmap = result.image.toBitmap().copy(Bitmap.Config.ARGB_8888, false)
        val pixels = IntArray(bitmap.width * bitmap.height)
        bitmap.getPixels(pixels, 0, bitmap.width, 0, 0, bitmap.width, bitmap.height)
        val sampled = Colors.dominantColor(pixels, bitmap.width, bitmap.height)
        val chosen = if (sampled == Colors.FALLBACK) Colors.nameColor(name) else sampled
        artColors[url] = chosen
        rgb = chosen
    }
    return rgb.toColor()
}

// --- Buttons ------------------------------------------------------------------------------------

enum class Variant { Primary, Secondary, Ghost, Danger }

/** The web's pill button. */
@Composable
fun OwButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: Variant = Variant.Secondary,
    small: Boolean = false,
    enabled: Boolean = true,
    loading: Boolean = false,
    icon: ImageVector? = null,
) {
    val active = enabled && !loading
    val (bg, fg, border) = when (variant) {
        Variant.Primary -> Triple(Ow.Accent, Color.Black, null)
        Variant.Secondary -> Triple(Color.Transparent, Ow.Ink, Ow.Line)
        Variant.Ghost -> Triple(Color.Transparent, Ow.Muted, null)
        Variant.Danger -> Triple(Color.Transparent, Ow.Danger, Ow.Danger.copy(alpha = 0.6f))
    }
    Row(
        modifier
            .alpha(if (active) 1f else 0.5f)
            .height(if (small) 32.dp else 40.dp)
            .clip(CircleShape)
            .background(bg)
            .then(if (border != null) Modifier.border(1.dp, border, CircleShape) else Modifier)
            .clickable(enabled = active, role = Role.Button, onClick = onClick)
            .padding(horizontal = if (small) 14.dp else 20.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.CenterHorizontally),
    ) {
        if (loading) {
            CircularProgressIndicator(Modifier.size(16.dp), color = fg, strokeWidth = 2.dp)
        } else if (icon != null) {
            Icon(icon, contentDescription = null, tint = fg, modifier = Modifier.size(16.dp))
        }
        Text(text, color = fg, fontSize = if (small) 13.sp else 14.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
    }
}

/** Round icon button: muted, the accent when [active]. */
@Composable
fun OwIconButton(
    icon: ImageVector,
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    active: Boolean = false,
    enabled: Boolean = true,
    size: Dp = 40.dp,
    iconSize: Dp = 20.dp,
    tint: Color? = null,
    background: Color = Color.Transparent,
) {
    Box(
        modifier
            .size(size)
            .clip(CircleShape)
            .background(background)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .semantics { contentDescription = label }
            .alpha(if (enabled) 1f else 0.4f),
        contentAlignment = Alignment.Center,
    ) {
        Icon(icon, contentDescription = null, tint = tint ?: if (active) Ow.Accent else Ow.Muted, modifier = Modifier.size(iconSize))
    }
}

/** The round accent play button. */
@Composable
fun PlayButton(playing: Boolean, label: String, onClick: () -> Unit, modifier: Modifier = Modifier, size: Dp = 56.dp) {
    Box(
        modifier
            .size(size)
            .shadow(8.dp, CircleShape)
            .clip(CircleShape)
            .background(Ow.Accent)
            .clickable(role = Role.Button, onClick = onClick)
            .semantics { contentDescription = label },
        contentAlignment = Alignment.Center,
    ) {
        Icon(
            if (playing) Icons.Default.Pause else Icons.Default.PlayArrow,
            contentDescription = null,
            tint = Color.Black,
            modifier = Modifier.size(size * 0.5f),
        )
    }
}

// --- Tabs, chips, badges ----------------------------------------------------------------------

data class TabItem<T>(val id: T, val label: String, val count: Int? = null)

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun <T> Tabs(tabs: List<TabItem<T>>, selected: T, onSelect: (T) -> Unit, modifier: Modifier = Modifier) {
    FlowRow(modifier, horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        tabs.forEach { t ->
            val on = t.id == selected
            Row(
                Modifier
                    .height(32.dp)
                    .clip(CircleShape)
                    .background(if (on) Ow.Ink else Ow.Raised)
                    .clickable(role = Role.Tab) { onSelect(t.id) }
                    .padding(horizontal = 14.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                val fg = if (on) Color.Black else Ow.Ink
                Text(t.label, color = fg, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                t.count?.let { Text("  $it", color = fg.copy(alpha = 0.6f), fontSize = 13.sp, fontWeight = FontWeight.SemiBold) }
            }
        }
    }
}

@Composable
fun OwChip(selected: Boolean, onClick: () -> Unit, content: @Composable RowScope.() -> Unit) {
    Row(
        Modifier
            .height(32.dp)
            .clip(CircleShape)
            .background(if (selected) Ow.Accent else Ow.Card)
            .border(1.dp, if (selected) Ow.Accent else Ow.Line, CircleShape)
            .clickable(role = Role.Checkbox, onClick = onClick)
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        CompositionLocalProvider(LocalContentColor provides if (selected) Color.Black else Ow.Ink) { content() }
    }
}

/** Small rounded label, e.g. "New" or "Fuzzy". */
@Composable
fun Badge(text: String, accent: Boolean = false) {
    Text(
        text,
        fontSize = 11.sp,
        fontWeight = if (accent) FontWeight.Bold else FontWeight.Normal,
        color = if (accent) Ow.Accent else Ow.Muted,
        modifier = Modifier
            .clip(CircleShape)
            .background(if (accent) Ow.AccentSoft else Ow.Raised)
            .padding(horizontal = 8.dp, vertical = 2.dp),
    )
}

// --- States ---------------------------------------------------------------------------------------

@Composable
fun Spinner(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxWidth().padding(vertical = 64.dp), contentAlignment = Alignment.Center) {
        CircularProgressIndicator(Modifier.size(24.dp), color = Ow.Muted, strokeWidth = 2.dp)
    }
}

@Composable
fun Empty(title: String, modifier: Modifier = Modifier, icon: ImageVector? = null, body: String? = null, action: (@Composable () -> Unit)? = null) {
    Column(
        modifier.fillMaxWidth().padding(horizontal = 24.dp, vertical = 64.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        icon?.let { Icon(it, contentDescription = null, tint = Ow.Faint, modifier = Modifier.size(48.dp).padding(bottom = 4.dp)) }
        if (icon != null) Spacer(Modifier.height(12.dp))
        Text(title, fontSize = 18.sp, fontWeight = FontWeight.Bold, textAlign = TextAlign.Center)
        body?.let {
            Text(it, fontSize = 14.sp, color = Ow.Muted, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 4.dp).widthIn(max = 420.dp))
        }
        action?.let {
            Spacer(Modifier.height(20.dp))
            it()
        }
    }
}

@Composable
fun ErrorText(error: String?, modifier: Modifier = Modifier) {
    if (error.isNullOrBlank()) return
    Text(error, color = Ow.Danger, fontSize = 14.sp, modifier = modifier)
}

@Composable
fun SectionHeader(title: String, modifier: Modifier = Modifier, action: (@Composable () -> Unit)? = null) {
    Row(
        modifier.fillMaxWidth().padding(bottom = 12.dp),
        verticalAlignment = Alignment.Bottom,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(title, style = androidx.compose.material3.MaterialTheme.typography.headlineMedium, modifier = Modifier.weight(1f, fill = false))
        action?.invoke()
    }
}

@Composable
fun PageTitle(title: String, modifier: Modifier = Modifier, subtitle: String? = null, action: (@Composable () -> Unit)? = null) {
    Column(modifier.fillMaxWidth().padding(horizontal = PagePad).padding(top = 8.dp, bottom = 24.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(title, style = androidx.compose.material3.MaterialTheme.typography.headlineLarge, modifier = Modifier.weight(1f))
            action?.invoke()
        }
        subtitle?.let { Text(it, fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.padding(top = 4.dp)) }
    }
}

/** Space for the shell's top bar, as a page's first item. */
@Composable
fun TopSpacer() = Spacer(Modifier.height(LocalTopInset.current))

/** Rounded card like the web's `rounded-xl bg-card p-6`. */
@Composable
fun CardBox(title: String, modifier: Modifier = Modifier, hint: String? = null, action: (@Composable () -> Unit)? = null, content: @Composable () -> Unit) {
    Column(
        modifier
            .fillMaxWidth()
            .padding(horizontal = PagePad)
            .padding(bottom = 24.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(Ow.Card)
            .padding(20.dp),
    ) {
        Text(title, fontSize = 18.sp, fontWeight = FontWeight.Bold)
        hint?.let { Text(it, fontSize = 14.sp, color = Ow.Muted, modifier = Modifier.padding(top = 4.dp)) }
        action?.let {
            Spacer(Modifier.height(12.dp))
            it()
        }
        Spacer(Modifier.height(16.dp))
        content()
    }
}

// --- Inputs ---------------------------------------------------------------------------------------

@Composable
fun OwTextField(
    value: String,
    onValueChange: (String) -> Unit,
    modifier: Modifier = Modifier,
    label: String? = null,
    placeholder: String? = null,
    hint: String? = null,
    password: Boolean = false,
    keyboardType: KeyboardType = KeyboardType.Text,
    imeAction: ImeAction = ImeAction.Done,
    onDone: (() -> Unit)? = null,
) {
    Column(modifier) {
        label?.let { Text(it, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, modifier = Modifier.padding(bottom = 6.dp)) }
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            singleLine = true,
            textStyle = TextStyle(color = Ow.Ink, fontSize = 15.sp),
            cursorBrush = SolidColor(Ow.Accent),
            visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
            keyboardOptions = KeyboardOptions(keyboardType = if (password) KeyboardType.Password else keyboardType, imeAction = imeAction),
            keyboardActions = KeyboardActions(onDone = { onDone?.invoke() }, onSearch = { onDone?.invoke() }, onGo = { onDone?.invoke() }),
            modifier = Modifier.fillMaxWidth(),
            decorationBox = { inner ->
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(44.dp)
                        .clip(RoundedCornerShape(6.dp))
                        .background(Ow.Card)
                        .border(1.dp, Ow.Line, RoundedCornerShape(6.dp))
                        .padding(horizontal = 14.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    if (value.isEmpty() && placeholder != null) Text(placeholder, color = Ow.Faint, fontSize = 15.sp, maxLines = 1)
                    inner()
                }
            },
        )
        hint?.let { Text(it, fontSize = 12.sp, color = Ow.Muted, modifier = Modifier.padding(top = 4.dp)) }
    }
}

/** Pill search field with a leading icon and a clear button. */
@Composable
fun SearchField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    icon: ImageVector,
    modifier: Modifier = Modifier,
    height: Dp = 48.dp,
    shape: Shape = CircleShape,
) {
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        singleLine = true,
        textStyle = TextStyle(color = Ow.Ink, fontSize = 15.sp),
        cursorBrush = SolidColor(Ow.Accent),
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
        modifier = modifier.fillMaxWidth(),
        decorationBox = { inner ->
            Row(
                Modifier.fillMaxWidth().height(height).clip(shape).background(Ow.Raised).padding(start = 16.dp, end = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(icon, contentDescription = null, tint = Ow.Muted, modifier = Modifier.size(20.dp))
                Spacer(Modifier.width(12.dp))
                Box(Modifier.weight(1f)) {
                    if (value.isEmpty()) Text(placeholder, color = Ow.Faint, fontSize = 15.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    inner()
                }
                if (value.isNotEmpty()) {
                    OwIconButton(Icons.Default.Close, "Clear search", { onValueChange("") }, size = 32.dp, iconSize = 16.dp)
                }
            }
        },
    )
}

// --- Dialogs --------------------------------------------------------------------------------------

/** The web's modal: raised card with a title, a close button, a scrolling body and a footer. */
@Composable
fun OwDialog(title: String, onDismiss: () -> Unit, footer: (@Composable RowScope.() -> Unit)? = null, content: @Composable () -> Unit) {
    Dialog(onDismissRequest = onDismiss, properties = DialogProperties(usePlatformDefaultWidth = false)) {
        Column(
            Modifier
                .padding(16.dp)
                .fillMaxWidth()
                .widthIn(max = 640.dp)
                .heightIn(max = 720.dp)
                .clip(RoundedCornerShape(12.dp))
                .background(Ow.Raised),
        ) {
            Row(Modifier.fillMaxWidth().padding(start = 24.dp, end = 12.dp, top = 16.dp, bottom = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(title, fontSize = 20.sp, fontWeight = FontWeight.Bold, modifier = Modifier.weight(1f))
                OwIconButton(Icons.Default.Close, "Close", onDismiss)
            }
            Column(Modifier.weight(1f, fill = false).verticalScroll(rememberScrollState()).padding(horizontal = 24.dp, vertical = 12.dp)) {
                content()
            }
            footer?.let {
                Row(
                    Modifier.fillMaxWidth().padding(start = 24.dp, end = 24.dp, top = 12.dp, bottom = 20.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp, Alignment.End),
                    verticalAlignment = Alignment.CenterVertically,
                    content = it,
                )
            }
        }
    }
}

/** Yes/no question, like the web's `confirm()`. */
@Composable
fun ConfirmDialog(text: String, confirm: String, onConfirm: () -> Unit, onDismiss: () -> Unit) {
    OwDialog(
        title = "Are you sure?",
        onDismiss = onDismiss,
        footer = {
            OwButton("Cancel", onDismiss, variant = Variant.Ghost)
            OwButton(confirm, { onDismiss(); onConfirm() }, variant = Variant.Danger)
        },
    ) {
        Text(text, fontSize = 15.sp)
    }
}

// --- Media ----------------------------------------------------------------------------------------

/** Animated bars for the track that is playing. */
@Composable
fun Equalizer(paused: Boolean, modifier: Modifier = Modifier) {
    val transition = rememberInfiniteTransition(label = "eq")
    Row(modifier.height(14.dp), horizontalArrangement = Arrangement.spacedBy(2.dp), verticalAlignment = Alignment.Bottom) {
        listOf(0, 180, 360).forEach { delayMs ->
            val scale by transition.animateFloat(
                initialValue = 0.3f,
                targetValue = 1f,
                animationSpec = infiniteRepeatable(tween(450, delayMillis = delayMs), RepeatMode.Reverse),
                label = "bar",
            )
            Box(
                Modifier
                    .width(3.dp)
                    .fillMaxHeight()
                    .graphicsLayer {
                        scaleY = if (paused) 0.4f else scale
                        transformOrigin = TransformOrigin(0.5f, 1f)
                    }
                    .clip(RoundedCornerShape(1.dp))
                    .background(Ow.Accent),
            )
        }
    }
}

/**
 * A grid of [columns] cards inside a LazyColumn, like the web's CardGrid (cards carry their own
 * 12dp padding, so the rows sit 4dp from the screen edge).
 */
fun <T> LazyListScope.cardGrid(items: List<T>, key: (T) -> Any, columns: Int = 2, content: @Composable (T) -> Unit) {
    items(items.chunked(columns), key = { row -> row.joinToString("|") { key(it).toString() } }) { row ->
        Row(Modifier.fillMaxWidth().padding(horizontal = 4.dp)) {
            row.forEach { Box(Modifier.weight(1f)) { content(it) } }
            repeat(columns - row.size) { Spacer(Modifier.weight(1f)) }
        }
    }
}

/** Square card: artwork, a title and a subtitle. */
@Composable
fun MediaCard(
    art: String?,
    name: String,
    subtitle: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    highlight: Boolean = false,
    overlay: @Composable (androidx.compose.foundation.layout.BoxScope.() -> Unit)? = null,
    footer: (@Composable () -> Unit)? = null,
) {
    Column(
        modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .clickable(onClick = onClick)
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Box {
            Art(art, name, Modifier.fillMaxWidth().aspectSquare().shadow(8.dp, RoundedCornerShape(6.dp)))
            overlay?.invoke(this)
        }
        Column {
            Text(
                name,
                fontWeight = FontWeight.SemiBold,
                fontSize = 15.sp,
                color = if (highlight) Ow.Accent else Ow.Ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            subtitle?.let { Text(it, fontSize = 13.sp, color = Ow.Muted, maxLines = 2, overflow = TextOverflow.Ellipsis) }
        }
        footer?.invoke()
    }
}

fun Modifier.aspectSquare() = aspectRatio(1f)

/** One track row: number or equalizer, artwork, title and artist, then [trailing]. */
@Composable
fun TrackRow(
    title: String,
    subtitle: String?,
    art: String?,
    artName: String,
    modifier: Modifier = Modifier,
    index: Int? = null,
    current: Boolean = false,
    playing: Boolean = false,
    onClick: (() -> Unit)? = null,
    trailing: (@Composable RowScope.() -> Unit)? = null,
) {
    Row(
        modifier
            .fillMaxWidth()
            .padding(horizontal = 4.dp)
            .clip(RoundedCornerShape(6.dp))
            .background(if (current) Ow.Raised.copy(alpha = 0.5f) else Color.Transparent)
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (index != null || current) {
            Box(Modifier.width(32.dp)) {
                if (current) Equalizer(paused = !playing) else Text("$index", color = Ow.Muted, fontSize = 14.sp)
            }
        }
        Art(art, artName, Modifier.size(40.dp))
        Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
            Text(
                title,
                fontWeight = FontWeight.Medium,
                fontSize = 14.sp,
                color = if (current) Ow.Accent else Ow.Ink,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            subtitle?.let { Text(it, fontSize = 13.sp, color = Ow.Muted, maxLines = 1, overflow = TextOverflow.Ellipsis) }
        }
        trailing?.invoke(this)
    }
}

/** Big page header with artwork and a gradient taken from it; [actions] sit on the fading colour below. */
@Composable
fun HeroHeader(
    color: Color,
    art: @Composable () -> Unit,
    kind: String,
    title: String,
    meta: (@Composable () -> Unit)? = null,
    actions: (@Composable RowScope.() -> Unit)? = null,
) {
    Column(Modifier.fillMaxWidth()) {
        Column(
            Modifier
                .fillMaxWidth()
                .background(Brush.verticalGradient(0f to color, 0.7f to color.copy(alpha = 0.35f), 1f to color.copy(alpha = 0.18f)))
                .padding(top = LocalTopInset.current + 32.dp, bottom = 24.dp, start = PagePad, end = PagePad),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(Modifier.width(176.dp).shadow(24.dp, RoundedCornerShape(6.dp))) { art() }
            Spacer(Modifier.height(24.dp))
            Text(kind, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
            Text(
                title,
                fontSize = 36.sp,
                lineHeight = 40.sp,
                fontWeight = FontWeight.Black,
                letterSpacing = (-0.9).sp,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(top = 4.dp),
            )
            meta?.let {
                Box(Modifier.padding(top = 12.dp)) {
                    CompositionLocalProvider(LocalContentColor provides Ow.Ink.copy(alpha = 0.8f)) { it() }
                }
            }
        }
        if (actions != null) {
            Row(
                Modifier
                    .fillMaxWidth()
                    .background(Brush.verticalGradient(listOf(color.copy(alpha = 0.18f), Color.Transparent)))
                    .padding(horizontal = PagePad, vertical = 24.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(16.dp),
                content = actions,
            )
        }
    }
}

/** Card-surface outline used by dashed placeholders. */
val DashedBorder = BorderStroke(1.dp, Ow.Line)

/** Percent of [part] in [whole], rounded. */
fun percent(part: Int, whole: Int): Int = if (whole > 0) (part * 100.0 / whole).roundToInt() else 0
