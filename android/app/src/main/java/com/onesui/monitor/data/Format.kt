package com.onesui.monitor.data

import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import kotlin.math.abs

object Format {
    private val units = arrayOf("B", "KB", "MB", "GB", "TB", "PB")

    fun bytes(value: Long): String {
        var v = value.toDouble().coerceAtLeast(0.0)
        var i = 0
        while (v >= 1024 && i < units.lastIndex) {
            v /= 1024
            i++
        }
        return if (i == 0) "${v.toLong()} B" else String.format(Locale.US, if (v >= 100) "%.0f %s" else "%.1f %s", v, units[i])
    }

    fun rate(bytesPerSecond: Long): String = bytes(bytesPerSecond) + "/s"

    /** Bits per second the way speed tests show it, e.g. "94.3 Mbps". */
    fun bits(bitsPerSecond: Double): String {
        val value = bitsPerSecond.coerceAtLeast(0.0)
        return when {
            value >= 1e9 -> String.format(Locale.US, "%.2f Gbps", value / 1e9)
            value >= 1e6 -> String.format(Locale.US, if (value >= 1e8) "%.0f Mbps" else "%.1f Mbps", value / 1e6)
            else -> String.format(Locale.US, "%.0f Kbps", value / 1e3)
        }
    }

    /** A check interval in seconds, e.g. "30 秒", "5 分钟". */
    fun interval(seconds: Int): String = when {
        seconds >= 3_600 && seconds % 3_600 == 0 -> "${seconds / 3_600} 小时"
        seconds >= 60 && seconds % 60 == 0 -> "${seconds / 60} 分钟"
        else -> "$seconds 秒"
    }

    fun ms(value: Double): String = when {
        value <= 0.0 -> "-"
        value < 10 -> String.format(Locale.US, "%.1f ms", value)
        else -> String.format(Locale.US, "%.0f ms", value)
    }

    /** "SOCKS5", "HTTP", "VLESS"…, and short names for the rest. */
    fun protocol(type: String): String = when (type.lowercase(Locale.US)) {
        "socks", "socks5" -> "SOCKS5"
        "shadowsocks" -> "SS"
        "hysteria2" -> "HY2"
        "anytls" -> "AnyTLS"
        "wireguard" -> "WG"
        else -> type.uppercase(Locale.US)
    }

    fun percent(value: Double): String = String.format(Locale.US, "%.1f%%", value.coerceIn(0.0, 100.0))

    fun uptime(seconds: Long): String {
        if (seconds <= 0) return "-"
        val days = seconds / 86_400
        val hours = seconds % 86_400 / 3_600
        val minutes = seconds % 3_600 / 60
        return when {
            days > 0 -> "${days}天 ${hours}小时"
            hours > 0 -> "${hours}小时 ${minutes}分"
            else -> "${minutes}分"
        }
    }

    fun ago(unixSeconds: Long, now: Long = System.currentTimeMillis() / 1000): String {
        if (unixSeconds <= 0) return "从未上线"
        val diff = abs(now - unixSeconds)
        return when {
            diff < 60 -> "${diff}秒前"
            diff < 3_600 -> "${diff / 60}分钟前"
            diff < 86_400 -> "${diff / 3_600}小时前"
            else -> "${diff / 86_400}天前"
        }
    }

    fun time(unixSeconds: Long): String =
        SimpleDateFormat("HH:mm", Locale.getDefault()).format(Date(unixSeconds * 1000))

    fun dateTime(unixSeconds: Long): String =
        SimpleDateFormat("MM-dd HH:mm:ss", Locale.getDefault()).format(Date(unixSeconds * 1000))

    fun date(unixSeconds: Long): String =
        SimpleDateFormat("yyyy-MM-dd", Locale.getDefault()).format(Date(unixSeconds * 1000))

    /** Two-letter region code to a flag emoji, e.g. "HK" to 🇭🇰. */
    fun flag(region: String): String {
        val code = region.trim().uppercase(Locale.US)
        if (code.length != 2 || !code.all { it in 'A'..'Z' }) return ""
        return code.map { String(Character.toChars(0x1F1E6 + (it - 'A'))) }.joinToString("")
    }

    fun osLabel(server: Server): String {
        val base = server.platform.ifBlank { server.os }
        return listOf(base, server.arch).filter { it.isNotBlank() }.joinToString(" / ")
    }
}
