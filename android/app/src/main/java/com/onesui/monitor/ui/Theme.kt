package com.onesui.monitor.ui

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.dynamicDarkColorScheme
import androidx.compose.material3.dynamicLightColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import com.onesui.monitor.data.AppSettings
import com.onesui.monitor.data.ThemeMode

private val LightColors = lightColorScheme(
    primary = Color(0xFF1E6FD9),
    secondary = Color(0xFF3D8B5A),
    background = Color(0xFFF7F8FA),
    surface = Color(0xFFFFFFFF),
    surfaceVariant = Color(0xFFEDEFF3),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFF8AB6FF),
    secondary = Color(0xFF7CD39A),
    background = Color(0xFF111318),
    surface = Color(0xFF191C22),
    surfaceVariant = Color(0xFF252932),
)

/** Status colors that stay readable in both themes. */
object StatusColors {
    val online = Color(0xFF22A861)
    val offline = Color(0xFFE5484D)
    val warn = Color(0xFFF1A10D)
    val upload = Color(0xFF7C5CFF)
    val download = Color(0xFF14A3B8)
}

@Composable
fun MonitorTheme(settings: AppSettings, content: @Composable () -> Unit) {
    val dark = when (settings.themeMode) {
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }
    val context = LocalContext.current
    val colors = when {
        settings.dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S ->
            if (dark) dynamicDarkColorScheme(context) else dynamicLightColorScheme(context)
        dark -> DarkColors
        else -> LightColors
    }
    MaterialTheme(colorScheme = colors, content = content)
}

fun usageColor(percent: Double): Color = when {
    percent >= 90 -> StatusColors.offline
    percent >= 75 -> StatusColors.warn
    else -> StatusColors.online
}
