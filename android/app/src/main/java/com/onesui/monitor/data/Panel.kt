package com.onesui.monitor.data

import org.json.JSONObject
import java.net.URI
import java.util.Base64

/** A bound panel: its API base URL, read-only monitor key and optional pinned certificate. */
data class Panel(
    val id: String,
    val name: String,
    val url: String,
    val key: String,
    /** SHA-256 of a self-signed HTTPS certificate the user chose to trust. */
    val certPin: String = "",
)

object PanelUrl {
    /**
     * Normalizes what the user typed into an API base ending in "/".
     * A missing scheme means plain HTTP, the panel's default gateway. Hash
     * routes and queries copied from the browser are dropped.
     */
    fun normalize(input: String): String? {
        var value = input.trim()
        if (value.isEmpty()) return null
        if (!value.contains("://")) value = "http://$value"
        value = value.substringBefore('#').substringBefore('?')
        val uri = runCatching { URI(value) }.getOrNull() ?: return null
        val scheme = uri.scheme?.lowercase()
        if (scheme != "http" && scheme != "https") return null
        if (uri.host.isNullOrBlank()) return null
        var path = uri.rawPath.orEmpty()
        if (!path.endsWith("/")) path += "/"
        val port = if (uri.port > 0) ":${uri.port}" else ""
        val host = if (uri.host.contains(':') && !uri.host.startsWith("[")) "[${uri.host}]" else uri.host
        return "$scheme://$host$port$path"
    }

    fun displayHost(url: String): String = runCatching { URI(url).host?.trim('[', ']') }.getOrNull() ?: url
}

/** The panel encodes "1sui-monitor:" + base64url({"v":1,"url":...,"key":...}) in its QR code. */
object BindCode {
    const val PREFIX = "1sui-monitor:"

    data class Result(val url: String, val key: String)

    fun parse(text: String): Result? {
        val value = text.trim()
        if (!value.startsWith(PREFIX, ignoreCase = true)) return null
        val payload = value.substring(PREFIX.length).trim().trimStart('/')
        val json = runCatching {
            String(Base64.getUrlDecoder().decode(payload), Charsets.UTF_8)
        }.getOrNull() ?: return null
        return parseJson(json)
    }

    fun parseJson(json: String): Result? {
        val obj = runCatching { JSONObject(json) }.getOrNull() ?: return null
        val url = PanelUrl.normalize(obj.optString("url")) ?: return null
        val key = obj.optString("key").trim()
        if (key.isEmpty()) return null
        return Result(url, key)
    }
}
