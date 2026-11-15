package io.github.felixclements.ownwave.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.em
import androidx.compose.ui.unit.sp

/** Same palette as the web UI (apps/web/tailwind.config.ts). */
object Ow {
    val Base = Color(0xFF0D0D0F)
    val Panel = Color(0xFF141417)
    val Card = Color(0xFF1B1B1F)
    val Raised = Color(0xFF26262C)
    val Line = Color(0xFF2E2E35)
    val Ink = Color(0xFFF5F5F7)
    val Muted = Color(0xFFA1A1AA)
    val Faint = Color(0xFF71717A)
    val Accent = Color(0xFF2DD4BF)
    val AccentStrong = Color(0xFF14B8A6)
    val AccentSoft = Color(0x262DD4BF)
    val Danger = Color(0xFFF87171)
    val Sky = Color(0xFF38BDF8)

    /** The teal wash at the top of Home and the sign-in page. */
    val HomeWash = Color(0xFF123A37)

    /** Genre chart bars (the web's validated bar colour). */
    val Bar = Color(0xFF11A897)
}

private val OwnWaveColors = darkColorScheme(
    primary = Ow.Accent,
    onPrimary = Color.Black,
    primaryContainer = Ow.AccentStrong,
    onPrimaryContainer = Color.Black,
    secondary = Ow.AccentStrong,
    onSecondary = Color.Black,
    background = Ow.Panel,
    onBackground = Ow.Ink,
    surface = Ow.Panel,
    onSurface = Ow.Ink,
    surfaceVariant = Ow.Card,
    onSurfaceVariant = Ow.Muted,
    surfaceContainerLowest = Ow.Base,
    surfaceContainerLow = Ow.Panel,
    surfaceContainer = Ow.Card,
    surfaceContainerHigh = Ow.Raised,
    surfaceContainerHighest = Ow.Raised,
    inverseSurface = Ow.Ink,
    inverseOnSurface = Color.Black,
    outline = Ow.Line,
    outlineVariant = Ow.Faint,
    error = Ow.Danger,
    onError = Color.Black,
)

/** Tight, heavy headings like the web's `font-black tracking-tight`. */
private val OwnWaveType = Typography().run {
    copy(
        headlineLarge = TextStyle(fontSize = 30.sp, fontWeight = FontWeight.Black, letterSpacing = (-0.025).em, lineHeight = 36.sp),
        headlineMedium = TextStyle(fontSize = 24.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.025).em, lineHeight = 32.sp),
        titleLarge = TextStyle(fontSize = 18.sp, fontWeight = FontWeight.Bold, lineHeight = 26.sp),
        titleMedium = TextStyle(fontSize = 16.sp, fontWeight = FontWeight.SemiBold, lineHeight = 22.sp),
        titleSmall = TextStyle(fontSize = 14.sp, fontWeight = FontWeight.SemiBold, lineHeight = 20.sp),
        bodyLarge = TextStyle(fontSize = 15.sp, lineHeight = 22.sp),
        bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 20.sp),
        bodySmall = TextStyle(fontSize = 13.sp, lineHeight = 18.sp),
        labelSmall = TextStyle(fontSize = 11.sp, lineHeight = 14.sp),
    )
}

/** Dark only, like the web UI. */
@Composable
fun OwnWaveTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = OwnWaveColors, typography = OwnWaveType, content = content)
}
