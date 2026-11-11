package io.github.felixclements.ownwave.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Same palette as the web UI (apps/web/tailwind.config.ts).
private val Base = Color(0xFF0D0D0F)
private val Panel = Color(0xFF141417)
private val Card = Color(0xFF1B1B1F)
private val Raised = Color(0xFF26262C)
private val Line = Color(0xFF2E2E35)
private val Ink = Color(0xFFF5F5F7)
private val Muted = Color(0xFFA1A1AA)
private val Faint = Color(0xFF71717A)
private val Accent = Color(0xFF2DD4BF)
private val AccentStrong = Color(0xFF14B8A6)
private val Danger = Color(0xFFF87171)

private val OwnWaveColors = darkColorScheme(
    primary = Accent,
    onPrimary = Base,
    primaryContainer = AccentStrong,
    onPrimaryContainer = Base,
    secondary = AccentStrong,
    onSecondary = Base,
    background = Base,
    onBackground = Ink,
    surface = Base,
    onSurface = Ink,
    surfaceVariant = Card,
    onSurfaceVariant = Muted,
    surfaceContainerLowest = Base,
    surfaceContainerLow = Panel,
    surfaceContainer = Panel,
    surfaceContainerHigh = Card,
    surfaceContainerHighest = Raised,
    outline = Line,
    outlineVariant = Faint,
    error = Danger,
    onError = Base,
)

/** Dark only, like the web UI. */
@Composable
fun OwnWaveTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = OwnWaveColors, content = content)
}
