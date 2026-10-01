package com.onesui.monitor.data

/** What was last known about one server, so alerts fire on changes rather than every poll. */
data class ServerAlertState(
    val offline: Boolean = false,
    val cpuHigh: Boolean = false,
    val memHigh: Boolean = false,
    val diskHigh: Boolean = false,
)

data class Alert(val key: String, val title: String, val text: String, val recovered: Boolean)

object AlertEvaluator {
    /**
     * Compares a fresh server list with the previous states. A server seen for
     * the first time only records its state, so opening the app on a server
     * that is already down does not produce a burst of old news.
     */
    fun evaluate(
        panel: Panel,
        servers: List<Server>,
        previous: Map<String, ServerAlertState>,
        settings: AppSettings,
    ): Pair<List<Alert>, Map<String, ServerAlertState>> {
        val alerts = mutableListOf<Alert>()
        val next = mutableMapOf<String, ServerAlertState>()
        for (server in servers) {
            val key = "${panel.id}:${server.id}:${if (server.local) "local" else "node"}"
            val label = server.name.ifBlank { server.hostname }
            val current = ServerAlertState(
                offline = !server.online,
                cpuHigh = server.online && over(server.cpuPercent, settings.cpuThreshold),
                memHigh = server.online && over(server.memory.percent, settings.memThreshold),
                diskHigh = server.online && over(server.disk.percent, settings.diskThreshold),
            )
            next[key] = current
            val before = previous[key] ?: continue
            if (settings.alertOffline && current.offline != before.offline) {
                alerts += if (current.offline) {
                    Alert("$key:offline", "$label 已离线", "最后在线：${Format.ago(server.lastSeen)}", recovered = false)
                } else {
                    Alert("$key:offline", "$label 已恢复在线", "面板：${panel.name}", recovered = true)
                }
            }
            if (current.cpuHigh && !before.cpuHigh) {
                alerts += Alert("$key:cpu", "$label CPU 过高", "CPU ${Format.percent(server.cpuPercent)}，阈值 ${settings.cpuThreshold}%", false)
            }
            if (current.memHigh && !before.memHigh) {
                alerts += Alert("$key:mem", "$label 内存过高", "内存 ${Format.percent(server.memory.percent)}，阈值 ${settings.memThreshold}%", false)
            }
            if (current.diskHigh && !before.diskHigh) {
                alerts += Alert("$key:disk", "$label 磁盘快满了", "磁盘 ${Format.percent(server.disk.percent)}，阈值 ${settings.diskThreshold}%", false)
            }
        }
        return alerts to next
    }

    private fun over(value: Double, threshold: Int) = threshold in 1..100 && value >= threshold
}

object ProxyAlertEvaluator {
    /**
     * Raises an alert when a proxy monitor goes down or comes back. "unknown"
     * (the checking server was unreachable) and "paused" keep the last state,
     * and a monitor seen for the first time only records it.
     */
    fun evaluate(
        panel: Panel,
        monitors: List<ProxyMonitor>,
        previous: Map<String, Boolean>,
    ): Pair<List<Alert>, Map<String, Boolean>> {
        val alerts = mutableListOf<Alert>()
        val next = mutableMapOf<String, Boolean>()
        for (monitor in monitors) {
            val key = "${panel.id}:proxy:${monitor.id}"
            val before = previous[key]
            val down = when (monitor.status) {
                "up" -> false
                "down" -> true
                else -> {
                    if (before != null) next[key] = before
                    continue
                }
            }
            next[key] = down
            if (before == null || before == down) continue
            alerts += if (down) {
                val reason = monitor.last?.let(Messages::probe).orEmpty()
                Alert("$key:down", "代理 ${monitor.name} 不可用", reason.ifBlank { "面板：${panel.name}" }, recovered = false)
            } else {
                val latency = monitor.last?.latencyMs?.let { " · ${it} ms" }.orEmpty()
                Alert("$key:down", "代理 ${monitor.name} 已恢复", "面板：${panel.name}$latency", recovered = true)
            }
        }
        return alerts to next
    }
}

