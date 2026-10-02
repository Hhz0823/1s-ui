package com.onesui.monitor.data

import org.json.JSONArray
import org.json.JSONObject

/**
 * The proxy client of a 1S-UI device (a fnOS NAS, an OpenWrt router or the
 * panel host): the PassWall / v2rayN-style client the panel runs in its own
 * sing-box. The monitor key reads it without subscription addresses or the
 * proxy port password.
 */
data class ClientSettings(
    val enabled: Boolean,
    val mode: String,
    val nodeId: Long,
    val auto: Boolean,
    val tun: Boolean,
    val mixed: Boolean,
    val mixedListen: String,
    val mixedPort: Int,
    val dnsHijack: Boolean,
)

data class ClientPlatform(
    val os: String,
    val openWrt: Boolean,
    val root: Boolean,
    val tunDevice: Boolean,
)

data class ClientSubscription(
    val id: Long,
    val name: String,
    /** Only the subscription's host: the panel hides the rest, which carries the account token. */
    val url: String,
    val enabled: Boolean,
    val updatedAt: Long,
    val lastError: String,
    val nodeCount: Int,
    val upload: Long,
    val download: Long,
    val total: Long,
    val expire: Long,
) {
    val used: Long get() = upload + download
}

data class ClientNode(
    val id: Long,
    val subscriptionId: Long,
    val subscription: String,
    val name: String,
    val protocol: String,
    val host: String,
    val port: Int,
    /** TCP connect time to the node, and a real request through it. */
    val tcpMs: Long,
    val delayMs: Long,
    val testedAt: Long,
    val testError: String,
) {
    val tested: Boolean get() = testedAt > 0
    val failed: Boolean get() = tested && delayMs <= 0 && testError.isNotBlank()
}

data class ClientState(
    val settings: ClientSettings,
    val platform: ClientPlatform,
    val running: Boolean,
    /** sing-box runs the client's config; false while it is off or failed to start. */
    val applied: Boolean,
    val problem: String,
    val currentId: Long,
    val subscriptions: List<ClientSubscription>,
    val nodes: List<ClientNode>,
    val testing: Boolean,
) {
    val current: ClientNode? get() = nodes.firstOrNull { it.id == currentId }

    /** One line on what the client is doing. */
    val summary: String
        get() {
            if (!settings.enabled) return "代理已关闭"
            val node = current?.name ?: if (settings.auto) "自动选择" else "未选择节点"
            return "${ClientModes.label(settings.mode)} · $node"
        }
}

/** Where traffic through the client leaves the internet. */
data class ClientExit(val ok: Boolean, val delayMs: Long, val ip: String, val country: String, val error: String)

object ClientModes {
    const val RULE = "rule"
    const val GFW = "gfw"
    const val GLOBAL = "global"
    val all = listOf(RULE, GFW, GLOBAL)

    fun label(mode: String): String = when (mode) {
        RULE -> "绕过大陆"
        GFW -> "仅 GFW 列表"
        GLOBAL -> "全局代理"
        else -> mode
    }

    fun hint(mode: String): String = when (mode) {
        RULE -> "中国大陆的网站和 IP 直连，其余走代理"
        GFW -> "只有 GFW 列表里的网站走代理，其余直连"
        GLOBAL -> "除局域网外全部走代理"
        else -> ""
    }
}

object ClientJson {
    fun parseState(obj: JSONObject): ClientState {
        val settings = obj.optJSONObject("settings") ?: JSONObject()
        val platform = obj.optJSONObject("platform") ?: JSONObject()
        return ClientState(
            settings = ClientSettings(
                enabled = settings.optBoolean("enabled"),
                mode = settings.optString("mode", ClientModes.RULE),
                nodeId = settings.optLong("node_id"),
                auto = settings.optBoolean("auto"),
                tun = settings.optBoolean("tun"),
                mixed = settings.optBoolean("mixed"),
                mixedListen = settings.optString("mixed_listen"),
                mixedPort = settings.optInt("mixed_port"),
                dnsHijack = settings.optBoolean("dns_hijack"),
            ),
            platform = ClientPlatform(
                os = platform.optString("os"),
                openWrt = platform.optBoolean("openwrt"),
                root = platform.optBoolean("root"),
                tunDevice = platform.optBoolean("tun_device"),
            ),
            running = obj.optBoolean("running"),
            applied = obj.optBoolean("applied"),
            problem = obj.optString("problem"),
            currentId = obj.optLong("current_id"),
            subscriptions = objects(obj.optJSONArray("subscriptions")).map {
                ClientSubscription(
                    id = it.optLong("id"),
                    name = it.optString("name"),
                    url = it.optString("url"),
                    enabled = it.optBoolean("enabled", true),
                    updatedAt = it.optLong("updated_at"),
                    lastError = it.optString("last_error"),
                    nodeCount = it.optInt("node_count"),
                    upload = it.optLong("upload"),
                    download = it.optLong("download"),
                    total = it.optLong("total"),
                    expire = it.optLong("expire"),
                )
            },
            nodes = objects(obj.optJSONArray("nodes")).map {
                ClientNode(
                    id = it.optLong("id"),
                    subscriptionId = it.optLong("subscription_id"),
                    subscription = it.optString("subscription"),
                    name = it.optString("name"),
                    protocol = it.optString("protocol"),
                    host = it.optString("host"),
                    port = it.optInt("port"),
                    tcpMs = it.optLong("tcp_ms"),
                    delayMs = it.optLong("delay_ms"),
                    testedAt = it.optLong("tested_at"),
                    testError = it.optString("test_error"),
                )
            },
            testing = obj.optBoolean("testing"),
        )
    }

    fun parseExit(obj: JSONObject) = ClientExit(
        ok = obj.optBoolean("ok"),
        delayMs = obj.optLong("delay_ms"),
        ip = obj.optString("ip"),
        country = obj.optString("country"),
        error = obj.optString("error"),
    )

    private fun objects(array: JSONArray?): List<JSONObject> {
        if (array == null) return emptyList()
        return (0 until array.length()).mapNotNull { array.optJSONObject(it) }
    }
}
