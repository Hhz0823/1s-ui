package com.onesui.monitor.ui

import androidx.compose.ui.graphics.Color

/** Status colors that stay readable in both themes. */
object StatusColors {
    val online = Color(0xFF22A861)
    val offline = Color(0xFFE5484D)
    val warn = Color(0xFFF1A10D)
    val upload = Color(0xFF7C5CFF)
    val download = Color(0xFF14A3B8)
}

fun usageColor(percent: Double): Color = when {
    percent >= 90 -> StatusColors.offline
    percent >= 75 -> StatusColors.warn
    else -> StatusColors.online
}
