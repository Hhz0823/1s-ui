package com.onesui.monitor.data

import org.json.JSONObject

/** Where bound panels and settings are kept; [Store] keeps them in app storage. */
interface AppStore {
    fun panels(): List<Panel>
    fun savePanel(panel: Panel)
    fun deletePanel(id: String)
    fun selectedPanelId(): String?
    fun selectPanel(id: String)
    fun settings(): AppSettings
    fun saveSettings(value: AppSettings)
}

/** The panel API as the app uses it; [MonitorClient] talks to a real panel. */
interface MonitorApi {
    suspend fun overview(): Overview
    suspend fun server(id: Long): Server
    suspend fun nodes(id: Long): List<NodeItem>
    suspend fun proxies(): List<ProxyMonitor>
    suspend fun proxy(id: Long, rangeSeconds: Long): ProxyDetail
    suspend fun saveProxy(input: ProxyInput): ProxyMonitor
    suspend fun testProxy(input: ProxyInput): ProbeResult
    suspend fun checkProxy(id: Long): ProbeResult
    suspend fun deleteProxy(id: Long)
    suspend fun startSpeedtest(id: Long): SpeedtestTarget
    /** Starts a speed test to server [id] run from another server, then polls it with [relaySpeedtest]. */
    suspend fun startRelaySpeedtest(id: Long, options: RelayOptions): RelayJob
    suspend fun relaySpeedtest(jobId: String): RelayJob
    /** The proxy client of server [id] (0 is the panel host). */
    suspend fun clientState(id: Long): ClientState
    /** Runs a client action that answers with the new state: mode, select, test, subscription.update. */
    suspend fun clientCall(id: Long, action: String, data: JSONObject): ClientState
    suspend fun clientExit(id: Long): ClientExit
}
