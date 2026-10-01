package com.onesui.monitor.data

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.json.JSONArray
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.IOException

private class MemoryStore(private var list: List<Panel>) : AppStore {
    var selected: String? = null
    private var settings = AppSettings()
    override fun panels() = list
    override fun savePanel(panel: Panel) {
        list = list.filter { it.id != panel.id } + panel
    }
    override fun deletePanel(id: String) {
        list = list.filter { it.id != id }
    }
    override fun selectedPanelId() = selected
    override fun selectPanel(id: String) {
        selected = id
    }
    override fun settings() = settings
    override fun saveSettings(value: AppSettings) {
        settings = value
    }
}

private fun server(id: Long, name: String, local: Boolean = false, online: Boolean = true) = MonitorJson.parseServer(
    JSONObject().put("id", id).put("name", name).put("local", local).put("online", online)
        .put("public_host", if (local) "" else "203.0.113.$id")
)

private fun proxy(id: Long, name: String, status: String) = MonitorJson.parseProxy(
    JSONObject().put("id", id).put("name", name).put("type", "socks5").put("host", "198.51.100.$id").put("port", 1080)
        .put("status", status).put("enabled", true).put("interval", 60)
        .put("last", JSONObject().put("ok", status == "up").put("latency_ms", 120).put("stage", if (status == "down") "auth" else "")
            .put("error", if (status == "down") "proxy rejected the username or password" else ""))
)

private class FakeApi(val name: String) : MonitorApi {
    var failing = false
    var proxyList = mutableListOf<ProxyMonitor>()
    val saved = mutableListOf<ProxyInput>()
    override suspend fun overview(): Overview {
        if (failing) throw MonitorError.Unauthorized()
        return Overview(
            "1.7.1", 0,
            listOf(server(0, "$name-main", local = true), server(7, "$name-hk")),
            Features(nodes = true, proxyMonitors = true, manageProxies = true, speedtest = true, speedtestPort = 5201),
        )
    }
    override suspend fun server(id: Long) = server(id, "$name-detail-$id")
    override suspend fun nodes(id: Long) = MonitorJson.parseNodes(
        JSONObject().put("items", JSONArray().put(JSONObject().put("id", 1).put("tag", "vless").put("type", "vless").put("port", 443)))
    )
    override suspend fun proxies() = proxyList.toList()
    override suspend fun proxy(id: Long, rangeSeconds: Long) =
        ProxyDetail(proxyList.first { it.id == id }, rangeSeconds, emptyList(), emptyList(), emptyList())
    override suspend fun saveProxy(input: ProxyInput): ProxyMonitor {
        saved += input
        val monitor = proxy(if (input.id == 0L) 99 else input.id, input.name, "pending")
        proxyList.removeAll { it.id == monitor.id }
        proxyList += monitor
        return monitor
    }
    override suspend fun testProxy(input: ProxyInput) = MonitorJson.parseProbe(JSONObject().put("ok", true).put("latency_ms", 88))
    override suspend fun checkProxy(id: Long) = MonitorJson.parseProbe(JSONObject().put("ok", false).put("stage", "connect").put("error", "connection refused"))
    override suspend fun deleteProxy(id: Long) {
        proxyList.removeAll { it.id == id }
    }
    override suspend fun startSpeedtest(id: Long): SpeedtestTarget = throw IOException("not in unit tests")
}

class ControllerTest {
    private val tokyo = Panel("a", "Tokyo", "http://1.1.1.1:2095/", "k1")
    private val frankfurt = Panel("b", "Frankfurt", "http://2.2.2.2:2095/", "k2")

    private fun controller(store: MemoryStore, apis: Map<String, FakeApi>) =
        MonitorController(store, CoroutineScope(Dispatchers.Unconfined), { apis.getValue(it.id) })

    @Test
    fun homeShowsEveryPanelAndKeepsGoingWhenOneFails() = runBlocking {
        val apis = mapOf("a" to FakeApi("tokyo"), "b" to FakeApi("fra"))
        val vm = controller(MemoryStore(listOf(tokyo, frankfurt)), apis)
        assertEquals(ALL_PANELS, vm.state.value.panelId)
        vm.poll()
        assertEquals(listOf("tokyo-main", "tokyo-hk", "fra-main", "fra-hk"), vm.state.value.servers.map { it.server.name })
        assertTrue(vm.state.value.features("a").speedtest)

        apis.getValue("b").failing = true
        vm.poll()
        val state = vm.state.value
        // The last good list stays on screen with the panel's error next to it.
        assertEquals(4, state.servers.size)
        assertEquals(listOf("Frankfurt"), state.panelErrors.map { it.first.name })
        assertTrue(state.panelErrors.single().second.contains("密钥"))

        vm.selectPanel("a")
        assertEquals(listOf("tokyo-main", "tokyo-hk"), vm.state.value.servers.map { it.server.name })
    }

    @Test
    fun serverDetailLoadsNodesAndGoesBack() = runBlocking {
        val apis = mapOf("a" to FakeApi("tokyo"))
        val vm = controller(MemoryStore(listOf(tokyo)), apis)
        assertEquals("a", vm.state.value.panelId)
        vm.poll()
        val hk = vm.state.value.servers.first { it.server.name == "tokyo-hk" }
        vm.openServer(hk)
        assertEquals(Screen.DETAIL, vm.state.value.screen)
        assertEquals("203.0.113.7", vm.serverAddress())
        vm.selectDetailTab(DetailTab.NODES)
        vm.poll()
        assertEquals("tokyo-detail-7", vm.state.value.detail?.name)
        assertEquals(443, vm.state.value.nodes?.single()?.port)

        vm.openServer(vm.state.value.servers.first { it.server.local })
        assertEquals("1.1.1.1", vm.serverAddress())
        assertNull("nodes of another server are not shown", vm.state.value.nodes)
        assertTrue(vm.back())
        assertEquals(Screen.SERVERS, vm.state.value.screen)
        assertFalse(vm.back())
    }

    @Test
    fun proxyEditorTestsSavesAndDeletes() = runBlocking {
        val api = FakeApi("tokyo")
        api.proxyList += proxy(1, "hk-socks", "up")
        val vm = controller(MemoryStore(listOf(tokyo)), mapOf("a" to api))
        vm.showHome(Screen.PROXIES)
        vm.poll()
        assertEquals(listOf("hk-socks"), vm.state.value.proxies.map { it.monitor.name })

        vm.editProxy("a")
        vm.updateEditing { it.copy(link = "socks5://u:p@198.51.100.9:1080", name = "new") }
        vm.testEditing()
        assertEquals(88L, vm.state.value.editing?.testResult?.latencyMs)
        vm.saveEditing()
        assertEquals(Screen.PROXIES, vm.state.value.screen)
        assertNull(vm.state.value.editing)
        assertEquals("socks5://u:p@198.51.100.9:1080", api.saved.single().link)
        assertEquals(listOf("hk-socks", "new"), vm.state.value.proxies.map { it.monitor.name })

        vm.openProxy(vm.state.value.proxies.first())
        vm.poll()
        assertNotNull(vm.state.value.proxyDetail)
        vm.checkProxyNow()
        assertTrue(vm.state.value.proxyAction!!.contains("连接被拒绝"))
        // Editing keeps the stored password unless a new one is typed.
        vm.editProxy("a", vm.state.value.proxyDetail!!.monitor)
        assertNull(vm.state.value.editing!!.input.password)
        assertTrue(vm.back())
        assertEquals(Screen.PROXY_DETAIL, vm.state.value.screen)
        vm.deleteProxy()
        assertEquals(Screen.PROXIES, vm.state.value.screen)
        assertEquals(listOf("new"), vm.state.value.proxies.map { it.monitor.name })
    }

    @Test
    fun bindingASecondPanelShowsAllPanels() = runBlocking {
        val store = MemoryStore(listOf(tokyo))
        val apis = mutableMapOf("a" to FakeApi("tokyo"))
        val vm = MonitorController(store, CoroutineScope(Dispatchers.Unconfined), { apis[it.id] ?: FakeApi("new") })
        val result = vm.bind("Frankfurt", "2.2.2.2:2095", "k2")
        assertEquals(BindResult.Success, result)
        assertEquals(ALL_PANELS, vm.state.value.panelId)
        assertEquals(ALL_PANELS, store.selected)
        assertEquals(2, vm.state.value.panels.size)
        vm.deletePanel(vm.state.value.panels.first { it.name == "Frankfurt" }.id)
        assertEquals("a", vm.state.value.panelId)
    }

    @Test
    fun proxyAlertsFireOnStateChangesOnly() {
        val panel = tokyo
        val (first, s1) = ProxyAlertEvaluator.evaluate(panel, listOf(proxy(1, "hk", "up")), emptyMap())
        assertTrue(first.isEmpty())
        val (down, s2) = ProxyAlertEvaluator.evaluate(panel, listOf(proxy(1, "hk", "down")), s1)
        assertEquals(listOf("代理 hk 不可用"), down.map { it.title })
        assertEquals("认证失败：用户名或密码错误", down.single().text)
        // An unreachable relay or a paused monitor keeps the last state.
        val (unknown, s3) = ProxyAlertEvaluator.evaluate(panel, listOf(proxy(1, "hk", "unknown")), s2)
        assertTrue(unknown.isEmpty())
        val (back, _) = ProxyAlertEvaluator.evaluate(panel, listOf(proxy(1, "hk", "up")), s3)
        assertEquals(listOf("代理 hk 已恢复"), back.map { it.title })
        assertFalse(back.single().text.isBlank())
    }

    @Test
    fun parsesPanelResponses() {
        val overview = MonitorJson.parseOverview(
            JSONObject("""{"panel_version":"1.7.1","features":{"nodes":true,"proxy_monitors":true,"manage_proxies":false,"speedtest":true,"speedtest_port":5201},"servers":[]}""")
        )
        assertEquals(Features(true, true, false, true, 5201), overview.features)
        assertEquals(Features(), MonitorJson.parseOverview(JSONObject("""{"servers":[]}""")).features)

        val nodes = MonitorJson.parseNodes(
            JSONObject(
                """{"items":[{"id":3,"tag":"vless-xhttp","type":"vless","core_type":"xray","port":443,"online":true,
                "upload_bps":10,"download_bps":20,"security":"reality","transport":"xhttp","encryption":true,"users":2,"cdn":"cdn.example.com:2053"},
                {"id":4,"tag":"hy2","type":"hysteria2","port":8443}]}"""
            )
        )
        assertEquals("reality", nodes[0].security)
        assertTrue(nodes[0].encryption && !nodes[0].udpOnly && nodes[1].udpOnly)
        assertEquals("cdn.example.com:2053", nodes[0].cdn)

        val detail = MonitorJson.parseProxyDetail(
            JSONObject(
                """{"id":5,"name":"jp","type":"http","host":"h","port":3128,"has_password":true,"server_id":2,"server_name":"relay",
                "status":"down","uptime":97.5,"recent":[{"time":1,"ok":true,"latency_ms":80}],"range":3600,
                "points":[{"time":0,"checks":4,"failures":1,"latency_ms":91.5,"max_ms":120}],
                "failures":[{"ok":false,"stage":"connect","error":"timed out"}],
                "exit_ips":[{"ip":"203.0.113.5","country":"JP","checks":3}]}"""
            )
        )
        assertEquals("relay", detail.monitor.serverName)
        assertTrue(detail.monitor.hasPassword)
        assertEquals(97.5, detail.monitor.uptime, 0.001)
        assertEquals(91.5, detail.points.single().latencyMs, 0.001)
        assertEquals("连不上代理：连接超时，地址不对或被防火墙拦截", Messages.probe(detail.failures.single()))
        assertEquals("JP", detail.exitIps.single().country)

        val target = MonitorJson.parseSpeedtest(JSONObject("""{"token":"00112233445566778899aabbccddeeff","port":5201,"hosts":["203.0.113.9","2001:db8::9"]}"""))
        assertEquals(listOf("203.0.113.9", "2001:db8::9"), target.hosts)
        assertEquals(0x11.toByte(), SpeedTester.decodeToken(target.token)[1])
        assertEquals(0xff.toByte(), SpeedTester.decodeToken(target.token)[15])

        val input = ProxyInput(name = "x", host = "h", port = 1, password = null).toJson()
        assertTrue(input.isNull("password"))
    }

    @Test
    fun translatesPanelMessages() {
        assertEquals("子服务器不在线", Messages.panel("managed server is offline or not connected via WebSocket"))
        assertEquals("这台服务器的面板版本过旧，不支持测速，请先更新", Messages.panel("the panel on this server is too old for speed tests; update it"))
        assertEquals("something new", Messages.panel("something new"))
        assertEquals("94.3 Mbps", Format.bits(94.3e6))
        assertEquals("1.20 Gbps", Format.bits(1.2e9))
        assertEquals("SOCKS5", Format.protocol("socks"))
        assertEquals("HY2", Format.protocol("hysteria2"))
    }
}
