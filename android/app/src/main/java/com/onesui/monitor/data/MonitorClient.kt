package com.onesui.monitor.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
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
    class NotSupported : MonitorError("面板版本过旧，不支持这个功能，请先更新面板")
    /** The panel turned this off for the monitor key. */
    class Forbidden(message: String) : MonitorError(message)
    /** HTTPS failed verification; [fingerprint] is the certificate the user may choose to trust. */
    class UntrustedCertificate(val fingerprint: String) : MonitorError("面板证书不受系统信任")
    class Failed(message: String) : MonitorError(message)
}

/** Client for the panel's /apiv2/monitor API, authenticated by the monitor key. */
class MonitorClient(private val panel: Panel) : MonitorApi {

    override suspend fun overview(): Overview = withContext(Dispatchers.IO) {
        MonitorJson.parseOverview(objectOf(request("GET", "apiv2/monitor/servers")))
    }

    override suspend fun server(id: Long): Server = withContext(Dispatchers.IO) {
        MonitorJson.parseServer(objectOf(request("GET", "apiv2/monitor/servers/$id")))
    }

    override suspend fun nodes(id: Long): List<NodeItem> = withContext(Dispatchers.IO) {
        MonitorJson.parseNodes(objectOf(request("GET", "apiv2/monitor/servers/$id/nodes")))
    }

    override suspend fun proxies(): List<ProxyMonitor> = withContext(Dispatchers.IO) {
        MonitorJson.parseProxies(request("GET", "apiv2/monitor/proxies") as? JSONArray)
    }

    override suspend fun proxy(id: Long, rangeSeconds: Long): ProxyDetail = withContext(Dispatchers.IO) {
        MonitorJson.parseProxyDetail(objectOf(request("GET", "apiv2/monitor/proxies/$id?range=$rangeSeconds")))
    }

    override suspend fun saveProxy(input: ProxyInput): ProxyMonitor = withContext(Dispatchers.IO) {
        MonitorJson.parseProxy(objectOf(request("POST", "apiv2/monitor/proxies", input.toJson())))
    }

    override suspend fun testProxy(input: ProxyInput): ProbeResult = withContext(Dispatchers.IO) {
        MonitorJson.parseProbe(objectOf(request("POST", "apiv2/monitor/proxies/test", input.toJson(), SLOW)))
    }

    override suspend fun checkProxy(id: Long): ProbeResult = withContext(Dispatchers.IO) {
        MonitorJson.parseProbe(objectOf(request("POST", "apiv2/monitor/proxies/$id/check", JSONObject(), SLOW)))
    }

    override suspend fun deleteProxy(id: Long) = withContext(Dispatchers.IO) {
        request("POST", "apiv2/monitor/proxies/$id/delete", JSONObject())
        Unit
    }

    override suspend fun startSpeedtest(id: Long): SpeedtestTarget = withContext(Dispatchers.IO) {
        MonitorJson.parseSpeedtest(objectOf(request("POST", "apiv2/monitor/servers/$id/speedtest", JSONObject(), SLOW)))
    }

    override suspend fun startRelaySpeedtest(id: Long, options: RelayOptions): RelayJob = withContext(Dispatchers.IO) {
        MonitorJson.parseRelayJob(objectOf(request("POST", "apiv2/monitor/servers/$id/speedtest/relay", options.toJson(), SLOW)))
    }

    override suspend fun relaySpeedtest(jobId: String): RelayJob = withContext(Dispatchers.IO) {
        MonitorJson.parseRelayJob(objectOf(request("GET", "apiv2/monitor/speedtests/" + URLEncoder.encode(jobId, "UTF-8"))))
    }

    override suspend fun clientState(id: Long): ClientState = withContext(Dispatchers.IO) {
        ClientJson.parseState(objectOf(request("GET", "apiv2/monitor/servers/$id/client", readTimeout = SLOW)))
    }

    override suspend fun clientCall(id: Long, action: String, data: JSONObject): ClientState = withContext(Dispatchers.IO) {
        // A latency test of a large subscription runs for minutes on the device.
        val body = JSONObject().put("action", action).put("data", data)
        ClientJson.parseState(objectOf(request("POST", "apiv2/monitor/servers/$id/client", body, CLIENT_CALL)))
    }

    override suspend fun clientExit(id: Long): ClientExit = withContext(Dispatchers.IO) {
        ClientJson.parseExit(objectOf(request("POST", "apiv2/monitor/servers/$id/client", JSONObject().put("action", "exit"), SLOW)))
    }

    private fun objectOf(value: Any?): JSONObject =
        value as? JSONObject ?: throw MonitorError.Failed("面板返回为空")

    /** Sends one request and returns the response's "obj": an object, an array or null. */
    private fun request(method: String, path: String, body: JSONObject? = null, readTimeout: Int = 10_000): Any? {
        val connection = open(URL(panel.url + path))
        try {
            connection.requestMethod = method
            connection.readTimeout = readTimeout
            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json")
            }
            val code = try {
                if (body != null) connection.outputStream.use { it.write(body.toString().toByteArray()) }
                connection.responseCode
            } catch (e: SSLHandshakeException) {
                val fingerprint = if (panel.certPin.isEmpty()) probeFingerprint(URL(panel.url)) else null
                throw if (fingerprint != null) MonitorError.UntrustedCertificate(fingerprint) else e
            }
            val text = (if (code in 200..299) connection.inputStream else connection.errorStream)
                ?.bufferedReader()?.use { it.readText() }.orEmpty()
            if (code == 401) throw MonitorError.Unauthorized()
            if (code == 404) throw MonitorError.NotSupported()
            val json = runCatching { JSONObject(text) }.getOrNull()
                ?: throw MonitorError.Failed("面板返回了无法识别的内容（HTTP $code），请确认地址是面板访问地址")
            val raw = json.optString("msg").removePrefix(":").trim()
            val msg = Messages.panel(raw)
            if (code == 403) throw MonitorError.Forbidden(msg.ifBlank { "面板不允许这个操作" })
            if (!json.optBoolean("success")) {
                // The JSON 404 of a panel without this route.
                if (raw.equals("not found", ignoreCase = true)) throw MonitorError.NotSupported()
                throw MonitorError.Failed(msg.ifBlank { "HTTP $code" })
            }
            return json.opt("obj").takeUnless { it == JSONObject.NULL }
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
        private const val SLOW = 40_000
        private const val CLIENT_CALL = 250_000

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
