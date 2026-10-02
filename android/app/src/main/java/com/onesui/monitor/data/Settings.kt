package com.onesui.monitor.data

enum class ThemeMode { SYSTEM, LIGHT, DARK }

data class AppSettings(
    val refreshSeconds: Int = 3,
    val themeMode: ThemeMode = ThemeMode.SYSTEM,
    val dynamicColor: Boolean = true,
    val alertsEnabled: Boolean = false,
    val alertIntervalSeconds: Int = 60,
    val alertOffline: Boolean = true,
    /** Percent thresholds; 0 turns that alert off. */
    val cpuThreshold: Int = 90,
    val memThreshold: Int = 90,
    val diskThreshold: Int = 90,
    /** Notify when a proxy monitor goes down or recovers. */
    val alertProxies: Boolean = true,
    /** Speed test defaults. */
    val speedSeconds: Int = 10,
    val speedStreams: Int = 4,
    val udpMbps: Int = 50,
)
