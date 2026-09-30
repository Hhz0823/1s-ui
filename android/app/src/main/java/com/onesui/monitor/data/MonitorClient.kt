package com.onesui.monitor.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import java.security.cert.X509Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLHandshakeException
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.TrustManager
import javax.net.ssl.X509TrustManager

sealed class MonitorError(message: String) : IOException(message) {
    class Unauthorized : MonitorError("监控密钥无效或已停用，请在面板设置里重新生成并绑定")
    class NotSupported : MonitorError("面板版本过旧，不支持手机监控，请先更新面板")
    /** HTTPS failed verification; [fingerprint] is the certificate the user may choose to trust. */
    class UntrustedCertificate(val fingerprint: String) : MonitorError("面板证书不受系统信任")
    class Failed(message: String) : MonitorError(message)
}

/** Read-only client for the panel's /apiv2/monitor API. */
class MonitorClient(private val panel: Panel) {

    suspend fun overview(): Overview = withContext(Dispatchers.IO) {
        MonitorJson.parseOverview(get("apiv2/monitor/servers"))
    }

    suspend fun server(id: Long): Server = withContext(Dispatchers.IO) {
        MonitorJson.parseServer(get("apiv2/monitor/servers/$id"))
    }

    private fun get(path: String): JSONObject {
        val connection = open(URL(panel.url + path))
        try {
            val code = try {
                connection.responseCode
            } catch (e: SSLHandshakeException) {
                val fingerprint = if (panel.certPin.isEmpty()) probeFingerprint(URL(panel.url)) else null
                throw if (fingerprint != null) MonitorError.UntrustedCertificate(fingerprint) else e
            }
            val body = (if (code in 200..299) connection.inputStream else connection.errorStream)
                ?.bufferedReader()?.use { it.readText() }.orEmpty()
            if (code == 401) throw MonitorError.Unauthorized()
            if (code == 404) throw MonitorError.NotSupported()
            val json = runCatching { JSONObject(body) }.getOrNull()
                ?: throw MonitorError.Failed("面板返回了无法识别的内容（HTTP $code），请确认地址是面板访问地址")
            if (!json.optBoolean("success")) {
                val msg = json.optString("msg")
                if (msg.contains("not found")) throw MonitorError.NotSupported()
                throw MonitorError.Failed(msg.ifBlank { "HTTP $code" })
            }
            return json.optJSONObject("obj") ?: throw MonitorError.Failed("面板返回为空")
        } finally {
            connection.disconnect()
        }
    }

    private fun open(url: URL): HttpURLConnection {
        val connection = url.openConnection() as HttpURLConnection
        connection.connectTimeout = 8_000
        connection.readTimeout = 10_000
        connection.useCaches = false
        connection.setRequestProperty("X-Monitor-Key", panel.key)
        connection.setRequestProperty("Accept", "application/json")
        if (connection is HttpsURLConnection && panel.certPin.isNotEmpty()) {
            connection.sslSocketFactory = pinnedFactory(panel.certPin)
            // The pin identifies the exact certificate, so the name on it
            // (often an IP-less self-signed cert) does not need to match.
            connection.hostnameVerifier = HostnameVerifier { _, _ -> true }
        }
        return connection
    }

    companion object {
        fun fingerprint(cert: X509Certificate): String =
            MessageDigest.getInstance("SHA-256").digest(cert.encoded).joinToString(":") { "%02X".format(it) }

        private fun pinnedFactory(pin: String): SSLSocketFactory {
            val trustManager = object : X509TrustManager {
                override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) = Unit
                override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {
                    val leaf = chain?.firstOrNull() ?: throw java.security.cert.CertificateException("empty chain")
                    if (!fingerprint(leaf).equals(pin, ignoreCase = true)) {
                        throw java.security.cert.CertificateException("certificate changed")
                    }
                }
                override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
            }
            return SSLContext.getInstance("TLS").apply { init(null, arrayOf<TrustManager>(trustManager), null) }.socketFactory
        }

        /** Reads the server certificate without trusting it, only to show its fingerprint. */
        private fun probeFingerprint(url: URL): String? = runCatching {
            var captured: X509Certificate? = null
            val capture = object : X509TrustManager {
                override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) = Unit
                override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {
                    captured = chain?.firstOrNull()
                    throw java.security.cert.CertificateException("probe only")
                }
                override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
            }
            val factory = SSLContext.getInstance("TLS").apply { init(null, arrayOf<TrustManager>(capture), null) }.socketFactory
            val connection = url.openConnection() as HttpsURLConnection
            connection.sslSocketFactory = factory
            connection.connectTimeout = 8_000
            connection.readTimeout = 8_000
            runCatching { connection.connect() }
            connection.disconnect()
            captured?.let(::fingerprint)
        }.getOrNull()
    }
}
