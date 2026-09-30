package com.onesui.monitor.data

import org.json.JSONArray
import org.json.JSONObject

data class Usage(val used: Long, val total: Long) {
    val percent: Double get() = if (total > 0) used.toDouble() * 100.0 / total else 0.0
}

data class NetPair(val sent: Long, val recv: Long)

data class Latency(
    val lastMs: Long?,
    val averageMs: Double,
    val p95Ms: Long,
    val lossPct: Double,
    val samples: Int,
)

data class MetricSample(
    val time: Long,
    val cpu: Double,
    val mem: Double,
    val swap: Double,
    val disk: Double,
    val processes: Int,
    val netSentRate: Long,
    val netRecvRate: Long,
)

data class Server(
    val id: Long,
    val name: String,
    val local: Boolean,
    val online: Boolean,
    val lastSeen: Long,
    val remoteIp: String,
    val publicHost: String,
    val version: String,
    val connMode: String,
    val hostname: String,
    val os: String,
    val arch: String,
    val platform: String,
    val kernel: String,
    val virtualization: String,
    val cpuModel: String,
    val uptime: Long,
    val cpuPercent: Double,
    val cpuCores: Int,
    val memory: Usage,
    val swap: Usage,
    val disk: Usage,
    val network: NetPair,
    val netRate: NetPair,
    val load1: Double,
    val load5: Double,
    val load15: Double,
    val processCount: Int,
    val tcpConns: Int,
    val udpConns: Int,
    val ipv4: List<String>,
    val ipv6: List<String>,
    val singBoxRunning: Boolean,
    val xrayRunning: Boolean,
    val xrayVersion: String,
    val latency: Latency?,
    val history: List<MetricSample>,
    // Optional fields added by newer panels (group, region, billing).
    val group: String,
    val region: String,
    val remark: String,
    val expireAt: Long,
    val price: Double,
    val currency: String,
)

data class Overview(
    val panelVersion: String,
    val serverTime: Long,
    val servers: List<Server>,
)

object MonitorJson {
    fun parseOverview(obj: JSONObject): Overview {
        val servers = obj.optJSONArray("servers")
        return Overview(
            panelVersion = obj.optString("panel_version"),
            serverTime = obj.optLong("server_time"),
            servers = servers.objects().map(::parseServer),
        )
    }

    fun parseServer(obj: JSONObject): Server {
        val report = obj.optJSONObject("report") ?: JSONObject()
        val load = report.optJSONObject("load") ?: JSONObject()
        val cores = report.optJSONObject("cores") ?: JSONObject()
        return Server(
            id = obj.optLong("id"),
            name = obj.optString("name"),
            local = obj.optBoolean("local"),
            online = obj.optBoolean("online"),
            lastSeen = obj.optLong("last_seen"),
            remoteIp = obj.optString("remote_ip"),
            publicHost = obj.optString("public_host"),
            version = obj.optString("version"),
            connMode = obj.optString("conn_mode"),
            hostname = report.optString("hostname"),
            os = report.optString("os"),
            arch = report.optString("arch"),
            platform = report.optString("platform"),
            kernel = report.optString("kernel"),
            virtualization = report.optString("virtualization"),
            cpuModel = report.optString("cpu_model"),
            uptime = report.optLong("uptime"),
            cpuPercent = report.optDouble("cpu_percent", 0.0).finite(),
            cpuCores = report.optInt("cpu_cores"),
            memory = usage(report.optJSONObject("memory")),
            swap = usage(report.optJSONObject("swap")),
            disk = usage(report.optJSONObject("disk")),
            network = net(report.optJSONObject("network")),
            netRate = net(report.optJSONObject("net_rate")),
            load1 = load.optDouble("load1", 0.0).finite(),
            load5 = load.optDouble("load5", 0.0).finite(),
            load15 = load.optDouble("load15", 0.0).finite(),
            processCount = report.optInt("process_count"),
            tcpConns = report.optInt("tcp_conns"),
            udpConns = report.optInt("udp_conns"),
            ipv4 = report.optJSONArray("ipv4").strings(),
            ipv6 = report.optJSONArray("ipv6").strings(),
            singBoxRunning = cores.optBoolean("singbox_running"),
            xrayRunning = cores.optBoolean("xray_running"),
            xrayVersion = cores.optString("xray_version"),
            latency = obj.optJSONObject("latency")?.let(::latency),
            history = obj.optJSONArray("history").objects().map(::sample),
            group = obj.optString("group"),
            region = obj.optString("region"),
            remark = obj.optString("remark"),
            expireAt = obj.optLong("expire_at"),
            price = obj.optDouble("price", 0.0).finite(),
            currency = obj.optString("currency"),
        )
    }

    private fun usage(obj: JSONObject?) = Usage(obj?.optLong("used") ?: 0, obj?.optLong("total") ?: 0)

    private fun net(obj: JSONObject?) = NetPair(obj?.optLong("sent") ?: 0, obj?.optLong("recv") ?: 0)

    private fun latency(obj: JSONObject): Latency? {
        val samples = obj.optInt("samples")
        if (samples <= 0) return null
        return Latency(
            lastMs = if (obj.isNull("last_ms")) null else obj.optLong("last_ms"),
            averageMs = obj.optDouble("average_ms", 0.0).finite(),
            p95Ms = obj.optLong("p95_ms"),
            lossPct = obj.optDouble("loss_pct", 0.0).finite(),
            samples = samples,
        )
    }

    private fun sample(obj: JSONObject) = MetricSample(
        time = obj.optLong("time"),
        cpu = obj.optDouble("cpu_percent", 0.0).finite(),
        mem = obj.optDouble("mem_percent", 0.0).finite(),
        swap = obj.optDouble("swap_percent", 0.0).finite(),
        disk = obj.optDouble("disk_percent", 0.0).finite(),
        processes = obj.optInt("process_count"),
        netSentRate = obj.optLong("net_sent_rate"),
        netRecvRate = obj.optLong("net_recv_rate"),
    )

    private fun Double.finite() = if (isNaN() || isInfinite()) 0.0 else this

    private fun JSONArray?.objects(): List<JSONObject> {
        if (this == null) return emptyList()
        return (0 until length()).mapNotNull { optJSONObject(it) }
    }

    private fun JSONArray?.strings(): List<String> {
        if (this == null) return emptyList()
        return (0 until length()).map { optString(it) }.filter { it.isNotBlank() }
    }
}
