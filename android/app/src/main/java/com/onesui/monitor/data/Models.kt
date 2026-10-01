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

/** What a panel lets its monitor key do; older panels report nothing. */
data class Features(
    val nodes: Boolean = false,
    val proxyMonitors: Boolean = false,
    val manageProxies: Boolean = false,
    val speedtest: Boolean = false,
    val speedtestPort: Int = 0,
)

data class Overview(
    val panelVersion: String,
    val serverTime: Long,
    val servers: List<Server>,
    val features: Features = Features(),
)

/** One inbound of a server: what it is and its traffic, never its secrets. */
data class NodeItem(
    val id: Long,
    val tag: String,
    val type: String,
    val coreType: String,
    val listen: String,
    val port: Int,
    val online: Boolean,
    val uploadBps: Long,
    val downloadBps: Long,
    val uploadBytes: Long,
    val downloadBytes: Long,
    val trafficLimit: Long,
    val trafficUsed: Long,
    val exhausted: Boolean,
    val ipLimit: Int,
    val activeIps: Int,
    val security: String,
    val transport: String,
    val encryption: Boolean,
    val users: Int,
    val cdn: String,
) {
    /** Protocols that carry their traffic over UDP, where a TCP ping says nothing. */
    val udpOnly: Boolean get() = type in setOf("hysteria", "hysteria2", "tuic", "wireguard")
}

/** One proxy check, from the panel host or the server chosen to run it. */
data class ProbeResult(
    val ok: Boolean,
    val time: Long,
    val connectMs: Long,
    val handshakeMs: Long,
    val tlsMs: Long,
    val ttfbMs: Long,
    val latencyMs: Long,
    val status: Int,
    val exitIp: String,
    val country: String,
    val stage: String,
    val error: String,
)

data class ProbePoint(val time: Long, val ok: Boolean, val latencyMs: Long)

data class ProxyMonitor(
    val id: Long,
    val name: String,
    val type: String,
    val host: String,
    val port: Int,
    val username: String,
    val hasPassword: Boolean,
    val target: String,
    val serverId: Long,
    val serverName: String,
    val interval: Int,
    val enabled: Boolean,
    val status: String,
    val last: ProbeResult?,
    val uptime: Double,
    val avgLatency: Double,
    val checks: Int,
    val recent: List<ProbePoint>,
)

data class ProxyBucket(val time: Long, val checks: Int, val failures: Int, val latencyMs: Double, val maxMs: Long)

data class ExitIp(val ip: String, val country: String, val firstSeen: Long, val lastSeen: Long, val checks: Int)

data class ProxyDetail(
    val monitor: ProxyMonitor,
    val range: Long,
    val points: List<ProxyBucket>,
    val failures: List<ProbeResult>,
    val exitIps: List<ExitIp>,
)

/** What the editor sends; a null password keeps the stored one. */
data class ProxyInput(
    val id: Long = 0,
    val name: String = "",
    val link: String = "",
    val type: String = "socks5",
    val host: String = "",
    val port: Int = 0,
    val username: String = "",
    val password: String? = null,
    val target: String = "",
    val serverId: Long = 0,
    val interval: Int = 60,
    val enabled: Boolean = true,
) {
    fun toJson(): JSONObject = JSONObject()
        .put("id", id)
        .put("name", name)
        .put("link", link)
        .put("type", type)
        .put("host", host)
        .put("port", port)
        .put("username", username)
        .put("password", password ?: JSONObject.NULL)
        .put("target", target)
        .put("server_id", serverId)
        .put("interval", interval)
        .put("enabled", enabled)
}

/** A speed test session on one server. */
data class SpeedtestTarget(
    val token: String,
    val port: Int,
    val expiresAt: Long,
    val maxSeconds: Int,
    val maxUdpMbps: Int,
    val hosts: List<String>,
)

object MonitorJson {
    fun parseOverview(obj: JSONObject): Overview {
        val servers = obj.optJSONArray("servers")
        val features = obj.optJSONObject("features")
        return Overview(
            panelVersion = obj.optString("panel_version"),
            serverTime = obj.optLong("server_time"),
            servers = servers.objects().map(::parseServer),
            features = if (features == null) Features() else Features(
                nodes = features.optBoolean("nodes"),
                proxyMonitors = features.optBoolean("proxy_monitors"),
                manageProxies = features.optBoolean("manage_proxies"),
                speedtest = features.optBoolean("speedtest"),
                speedtestPort = features.optInt("speedtest_port"),
            ),
        )
    }

    fun parseNodes(obj: JSONObject): List<NodeItem> = obj.optJSONArray("items").objects().map { item ->
        NodeItem(
            id = item.optLong("id"),
            tag = item.optString("tag"),
            type = item.optString("type"),
            coreType = item.optString("core_type"),
            listen = item.optString("listen"),
            port = item.optInt("port"),
            online = item.optBoolean("online"),
            uploadBps = item.optLong("upload_bps"),
            downloadBps = item.optLong("download_bps"),
            uploadBytes = item.optLong("upload_bytes"),
            downloadBytes = item.optLong("download_bytes"),
            trafficLimit = item.optLong("traffic_limit"),
            trafficUsed = item.optLong("traffic_used"),
            exhausted = item.optBoolean("exhausted"),
            ipLimit = item.optInt("ip_limit"),
            activeIps = item.optInt("active_ips"),
            security = item.optString("security"),
            transport = item.optString("transport"),
            encryption = item.optBoolean("encryption"),
            users = item.optInt("users"),
            cdn = item.optString("cdn"),
        )
    }

    fun parseProbe(obj: JSONObject) = ProbeResult(
        ok = obj.optBoolean("ok"),
        time = obj.optLong("time"),
        connectMs = obj.optLong("connect_ms"),
        handshakeMs = obj.optLong("handshake_ms"),
        tlsMs = obj.optLong("tls_ms"),
        ttfbMs = obj.optLong("ttfb_ms"),
        latencyMs = obj.optLong("latency_ms"),
        status = obj.optInt("status"),
        exitIp = obj.optString("exit_ip"),
        country = obj.optString("country"),
        stage = obj.optString("stage"),
        error = obj.optString("error"),
    )

    fun parseProxy(obj: JSONObject) = ProxyMonitor(
        id = obj.optLong("id"),
        name = obj.optString("name"),
        type = obj.optString("type"),
        host = obj.optString("host"),
        port = obj.optInt("port"),
        username = obj.optString("username"),
        hasPassword = obj.optBoolean("has_password"),
        target = obj.optString("target"),
        serverId = obj.optLong("server_id"),
        serverName = obj.optString("server_name"),
        interval = obj.optInt("interval"),
        enabled = obj.optBoolean("enabled"),
        status = obj.optString("status"),
        last = obj.optJSONObject("last")?.let(::parseProbe),
        uptime = obj.optDouble("uptime", 0.0).finite(),
        avgLatency = obj.optDouble("avg_latency", 0.0).finite(),
        checks = obj.optInt("checks"),
        recent = obj.optJSONArray("recent").objects().map {
            ProbePoint(it.optLong("time"), it.optBoolean("ok"), it.optLong("latency_ms"))
        },
    )

    fun parseProxies(array: JSONArray?): List<ProxyMonitor> = array.objects().map(::parseProxy)

    fun parseProxyDetail(obj: JSONObject) = ProxyDetail(
        monitor = parseProxy(obj),
        range = obj.optLong("range"),
        points = obj.optJSONArray("points").objects().map {
            ProxyBucket(
                time = it.optLong("time"),
                checks = it.optInt("checks"),
                failures = it.optInt("failures"),
                latencyMs = it.optDouble("latency_ms", 0.0).finite(),
                maxMs = it.optLong("max_ms"),
            )
        },
        failures = obj.optJSONArray("failures").objects().map(::parseProbe),
        exitIps = obj.optJSONArray("exit_ips").objects().map {
            ExitIp(it.optString("ip"), it.optString("country"), it.optLong("first_seen"), it.optLong("last_seen"), it.optInt("checks"))
        },
    )

    fun parseSpeedtest(obj: JSONObject) = SpeedtestTarget(
        token = obj.optString("token"),
        port = obj.optInt("port"),
        expiresAt = obj.optLong("expires_at"),
        maxSeconds = obj.optInt("max_seconds", 15),
        maxUdpMbps = obj.optInt("max_udp_mbps", 1000),
        hosts = obj.optJSONArray("hosts").strings(),
    )

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
