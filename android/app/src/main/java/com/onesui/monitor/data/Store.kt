package com.onesui.monitor.data

import android.content.Context
import android.content.SharedPreferences
import org.json.JSONArray
import org.json.JSONObject
import java.util.UUID

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
)

/** Panels and settings live in private app storage; keys never leave the device except to their panel. */
class Store(context: Context) {
    private val prefs: SharedPreferences = context.applicationContext.getSharedPreferences("monitor", Context.MODE_PRIVATE)

    fun panels(): List<Panel> {
        val array = runCatching { JSONArray(prefs.getString(KEY_PANELS, "[]")) }.getOrElse { JSONArray() }
        return (0 until array.length()).mapNotNull { index ->
            val obj = array.optJSONObject(index) ?: return@mapNotNull null
            Panel(
                id = obj.optString("id").ifBlank { UUID.randomUUID().toString() },
                name = obj.optString("name"),
                url = obj.optString("url"),
                key = obj.optString("key"),
                certPin = obj.optString("cert_pin"),
            )
        }
    }

    fun savePanel(panel: Panel) {
        val list = panels().toMutableList()
        val index = list.indexOfFirst { it.id == panel.id }
        if (index >= 0) list[index] = panel else list.add(panel)
        writePanels(list)
    }

    fun deletePanel(id: String) {
        writePanels(panels().filterNot { it.id == id })
        if (selectedPanelId() == id) prefs.edit().remove(KEY_SELECTED).apply()
    }

    fun selectedPanelId(): String? = prefs.getString(KEY_SELECTED, null)

    fun selectPanel(id: String) = prefs.edit().putString(KEY_SELECTED, id).apply()

    private fun writePanels(list: List<Panel>) {
        val array = JSONArray()
        list.forEach {
            array.put(
                JSONObject()
                    .put("id", it.id)
                    .put("name", it.name)
                    .put("url", it.url)
                    .put("key", it.key)
                    .put("cert_pin", it.certPin)
            )
        }
        prefs.edit().putString(KEY_PANELS, array.toString()).apply()
    }

    fun settings(): AppSettings = AppSettings(
        refreshSeconds = prefs.getInt("refresh_seconds", 3),
        themeMode = runCatching { ThemeMode.valueOf(prefs.getString("theme_mode", null) ?: "SYSTEM") }.getOrDefault(ThemeMode.SYSTEM),
        dynamicColor = prefs.getBoolean("dynamic_color", true),
        alertsEnabled = prefs.getBoolean("alerts_enabled", false),
        alertIntervalSeconds = prefs.getInt("alert_interval", 60),
        alertOffline = prefs.getBoolean("alert_offline", true),
        cpuThreshold = prefs.getInt("cpu_threshold", 90),
        memThreshold = prefs.getInt("mem_threshold", 90),
        diskThreshold = prefs.getInt("disk_threshold", 90),
    )

    fun saveSettings(value: AppSettings) {
        prefs.edit()
            .putInt("refresh_seconds", value.refreshSeconds)
            .putString("theme_mode", value.themeMode.name)
            .putBoolean("dynamic_color", value.dynamicColor)
            .putBoolean("alerts_enabled", value.alertsEnabled)
            .putInt("alert_interval", value.alertIntervalSeconds)
            .putBoolean("alert_offline", value.alertOffline)
            .putInt("cpu_threshold", value.cpuThreshold)
            .putInt("mem_threshold", value.memThreshold)
            .putInt("disk_threshold", value.diskThreshold)
            .apply()
    }

    companion object {
        private const val KEY_PANELS = "panels"
        private const val KEY_SELECTED = "selected_panel"
    }
}
