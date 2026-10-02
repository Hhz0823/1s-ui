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

private fun server(id: Long, name: String, local: Boolean = false, online: Boolean = true, vararg capabilities: String) = MonitorJson.parseServer(
    JSONObject().put("id", id).put("name", name).put("local", local).put("online", online)
        .put("public_host", if (local) "" else "203.0.113.$id").put("version", "1.7.0")
        .put("report", JSONObject().put("panel", JSONObject().put("capabilities", JSONArray(capabilities.toList()))))
)

private val clientJson = """{"settings":{"enabled":true,"mode":"rule","node_id":1,"auto":false,"tun":true,"mixed":true,"mixed_port":7890,"dns_hijack":true},
    "platform":{"os":"linux","openwrt":true,"root":true,"tun_device":true},"running":true,"applied":true,"current_id":1,
    "subscriptions":[{"id":3,"name":"airport","url":"https://sub.example.com/…","node_count":2,"upload":1024,"download":2048,"total":1048576,"expire":1924905600}],
    "nodes":[{"id":1,"subscription_id":3,"subscription":"airport","name":"hk-01","protocol":"vless","host":"203.0.113.1","port":443,"tcp_ms":20,"delay_ms":180,"tested_at":10},
             {"id":2,"subscription_id":3,"subscription":"airport","name":"jp-01","protocol":"hysteria2","host":"203.0.113.2","port":8443,"tested_at":10,"test_error":"the node did not relay the connection"}]}"""

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
    val tested = mutableListOf<ProxyInput>()
    val clientCalls = mutableListOf<Pair<String, String>>()
    var relayOptions: RelayOptions? = null
    var relayPolls = 0
    override suspend fun overview(): Overview {
        if (failing) throw MonitorError.Unauthorized()
        return Overview(
            "1.7.0", 0,
            listOf(
                server(0, "$name-main", local = true),
                server(7, "$name-hk"),
                server(8, "$name-nas", false, true, Capability.PROXY_CLIENT, Capability.SPEEDTEST_CLIENT, Capability.NODE_CHECKS, Capability.NODE_LINK),
            ),
            Features(
                nodes = true, proxyMonitors = true, manageProxies = true, speedtest = true, speedtestPort = 5201,
                nodeMonitors = true, relaySpeedtest = true, clients = true, manageClients = true,
            ),
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
    override suspend fun testProxy(input: ProxyInput): ProbeResult {
        tested += input
        return MonitorJson.parseProbe(JSONObject().put("ok", true).put("latency_ms", 88))
    }
    override suspend fun checkProxy(id: Long) = MonitorJson.parseProbe(JSONObject().put("ok", false).put("stage", "connect").put("error", "connection refused"))
    override suspend fun deleteProxy(id: Long) {
        proxyList.removeAll { it.id == id }
    }
    override suspend fun startSpeedtest(id: Long): SpeedtestTarget = throw IOException("not in unit tests")
    override suspend fun startRelaySpeedtest(id: Long, options: RelayOptions): RelayJob {
        relayOptions = options
        return MonitorJson.parseRelayJob(JSONObject("""{"id":"job1","relay_id":${options.relayId},"relay_name":"$name-nas","status":"running","current":"tcp_ping","results":[]}"""))
    }
    override suspend fun relaySpeedtest(jobId: String): RelayJob {
        relayPolls++
        return MonitorJson.parseRelayJob(
            JSONObject(
                """{"id":"$jobId","relay_name":"$name-nas","status":"done","host":"203.0.113.7","results":[
                {"test":"tcp_ping","host":"203.0.113.7","ping":{"sent":10,"received":10,"min_ms":30,"avg_ms":35.5,"max_ms":40,"jitter_ms":2}},
                {"test":"udp_ping","error":"no reply from 203.0.113.7"}]}"""
            )
        )
    }
    var client = ClientJson.parseState(JSONObject(clientJson))
    override suspend fun clientState(id: Long) = client
    override suspend fun clientCall(id: Long, action: String, data: JSONObject): ClientState {
        clientCalls += action to data.toString()
        if (action == "select") {
            val chosen = data.getLong("node_id")
            client = client.copy(currentId = chosen, settings = client.settings.copy(nodeId = chosen, auto = chosen == 0L))
        }
        if (action == "mode" && data.has("enabled")) client = client.copy(settings = client.settings.copy(enabled = data.getBoolean("enabled")))
        return client
    }
    override suspend fun clientExit(id: Long) = ClientJson.parseExit(JSONObject("""{"ok":true,"delay_ms":210,"ip":"198.51.100.20","country":"HK"}"""))
}

class ControllerTest {
    private val tokyo = Panel("a", "Tokyo", "http://1.1.1.1:2095/", "k1")
    private val frankfurt = Panel("b", "Frankfurt", "http://2.2.2.2:2095/", "k2")

    private fun controller(store: MemoryStore, apis: Map<String, FakeApi>, latest: String = "v1.7.1") =
        MonitorController(store, CoroutineScope(Dispatchers.Unconfined), { apis.getValue(it.id) }, appVersion = "1.7.0", latestRelease = { ReleaseInfo(latest) })

    /** Waits for work the controller finishes on another thread (relay polling). */
    private fun waitFor(what: String, check: () -> Boolean) {
        val deadline = System.currentTimeMillis() + 10_000
        while (!check()) {
            if (System.currentTimeMillis() > deadline) throw AssertionError("timed out waiting for $what")
            Thread.sleep(20)
        }
    }

    @Test
    fun homeShowsEveryPanelAndKeepsGoingWhenOneFails() = runBlocking {
        val apis = mapOf("a" to FakeApi("tokyo"), "b" to FakeApi("fra"))
        val vm = controller(MemoryStore(listOf(tokyo, frankfurt)), apis)
        assertEquals(ALL_PANELS, vm.state.value.panelId)
        vm.poll()
        assertEquals(listOf("tokyo-main", "tokyo-hk", "tokyo-nas", "fra-main", "fra-hk", "fra-nas"), vm.state.value.servers.map { it.server.name })
        assertTrue(vm.state.value.features("a").speedtest)

        apis.getValue("b").failing = true
        vm.poll()
        val state = vm.state.value
        // The last good list stays on screen with the panel's error next to it.
        assertEquals(6, state.servers.size)
        assertEquals(listOf("Frankfurt"), state.panelErrors.map { it.first.name })
        assertTrue(state.panelErrors.single().second.contains("密钥"))

        vm.selectPanel("a")
        assertEquals(listOf("tokyo-main", "tokyo-hk", "tokyo-nas"), vm.state.value.servers.map { it.server.name })
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
        val vm = MonitorController(store, CoroutineScope(Dispatchers.Unconfined), { apis[it.id] ?: FakeApi("new") }, latestRelease = { ReleaseInfo("v1.7.0") })
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
        assertEquals(0, input.getInt("node_inbound_id"))

        val features = MonitorJson.parseOverview(
            JSONObject("""{"features":{"node_monitors":true,"relay_speedtest":true,"clients":true,"manage_clients":false},"servers":[]}""")
        ).features
        assertTrue(features.nodeMonitors && features.relaySpeedtest && features.clients && !features.manageClients)

        val node = MonitorJson.parseProxy(
            JSONObject("""{"id":9,"name":"nas · vless","type":"node","protocol":"vless","host":"203.0.113.9","port":443,
                "node_server_id":4,"node_inbound_id":12,"node_server_name":"nas","status":"down",
                "last":{"ok":false,"stage":"handshake","error":"reality verification failed"}}""")
        )
        assertTrue(node.isNode)
        assertEquals(12L, node.nodeInboundId)
        assertEquals("nas", node.nodeServerName)
        assertTrue(Messages.probe(node.last!!, node = true).startsWith("节点握手失败：REALITY 验证失败"))
        val relayed = MonitorJson.parseProbe(JSONObject().put("stage", "tunnel").put("error", "the node did not relay the connection (wrong UUID/password)"))
        assertTrue(Messages.probe(relayed, node = true).contains("UUID / 密码不对"))
        assertTrue(Messages.probe(MonitorJson.parseProbe(JSONObject().put("stage", "connect").put("error", "connection refused")), node = true).startsWith("连不上节点"))

        val job = MonitorJson.parseRelayJob(
            JSONObject("""{"id":"a1","relay_id":0,"relay_name":"panel","status":"done","tests":["udp_download"],
                "results":[{"test":"udp_download","host":"203.0.113.9","udp":{"target_mbps":50,"sent_packets":1000,"received_packets":900,"bits_per_second":4.5e7,"jitter_ms":3.5}}]}""")
        )
        assertFalse(job.running)
        assertEquals(10.0, job.results.single().udp!!.lossPct, 0.001)
        val options = RelayOptions(4, listOf("tcp_ping"), 10, 4, 50).toJson()
        assertEquals(4L, options.getLong("relay_id"))
        assertEquals("tcp_ping", options.getJSONArray("tests").getString(0))

        val client = ClientJson.parseState(JSONObject(clientJson))
        assertEquals("hk-01", client.current?.name)
        assertTrue(client.platform.openWrt && client.settings.tun)
        assertTrue(client.nodes[1].failed && !client.nodes[0].failed)
        assertEquals(3072L, client.subscriptions.single().used)
        assertEquals("绕过大陆 · hk-01", client.summary)
        assertEquals("没有符合自动选择条件的节点", Messages.panel("no node matches the automatic selection"))
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
        assertEquals("测速任务已结束或过期，请重新测速", Messages.panel("speed test not found"))
        assertEquals("这台设备的面板版本过旧，没有客户端代理，请先更新", Messages.panel("the panel on this server has no proxy client yet; update it"))
    }

    @Test
    fun comparesVersionsAndFindsReleases() = runBlocking {
        assertTrue(Versions.isNewer("v1.7.1", "1.7.0"))
        assertTrue(Versions.isNewer("1.10.0", "1.9.9"))
        assertFalse(Versions.isNewer("v1.7.0", "1.7.0"))
        assertFalse(Versions.isNewer("v1.6.0-boost", "1.6.0"))
        assertFalse("an unknown version is never offered an update", Versions.isNewer("v9.0.0", ""))
        assertTrue(Versions.compare("1.7", "1.7.0") == 0)

        val api = ReleaseChecker { url ->
            when {
                url.startsWith("https://api.github.com/") -> HttpAnswer(200, """{"tag_name":"v1.7.2","body":"notes","assets":[{"name":"1s-ui-monitor-android.apk"}]}""")
                else -> HttpAnswer(404, "")
            }
        }.latest()
        assertEquals("1.7.2", api.version)
        assertTrue(api.hasApk)
        assertEquals("https://github.com/Hhz0823/1s-ui/releases/download/v1.7.2/1s-ui-monitor-android.apk", api.apkUrl)
        assertTrue(api.mirrorApkUrl.startsWith("https://ghfast.top/https://github.com/"))

        // api.github.com blocked: the release page redirect names the tag.
        val visited = mutableListOf<String>()
        val redirect = ReleaseChecker { url ->
            visited += url
            when (url) {
                "https://github.com/Hhz0823/1s-ui/releases/latest" -> HttpAnswer(302, "", "https://github.com/Hhz0823/1s-ui/releases/tag/v1.7.3")
                else -> HttpAnswer(403, "")
            }
        }.latest()
        assertEquals("v1.7.3", redirect.tag)
        assertEquals(2, visited.size)

        // GitHub blocked too: a mirror serves the page itself.
        val mirror = ReleaseChecker { url ->
            if (url.startsWith("https://ghfast.top/")) HttpAnswer(200, """<a href="/Hhz0823/1s-ui/releases/tag/v1.7.4">v1.7.4</a>""") else throw java.io.IOException("blocked")
        }.latest()
        assertEquals("v1.7.4", mirror.tag)

        val failure = runCatching { ReleaseChecker { throw java.io.IOException("offline") }.latest() }.exceptionOrNull()
        assertTrue(failure!!.message!!.contains("GitHub"))
    }

    @Test
    fun checksForUpdatesOfTheAppAndPanels() = runBlocking {
        val vm = controller(MemoryStore(listOf(tokyo)), mapOf("a" to FakeApi("tokyo")))
        vm.poll()
        vm.checkUpdates()
        val update = vm.state.value.update
        assertEquals("1.7.1", update.latest?.version)
        assertTrue(update.appUpdate)
        assertTrue("servers on 1.7.0 can update", update.outdated(vm.state.value.servers.first().server.version))
        vm.dismissUpdate()
        assertTrue(vm.state.value.update.dismissed)

        val current = controller(MemoryStore(listOf(tokyo)), mapOf("a" to FakeApi("tokyo")), latest = "v1.7.0")
        current.checkUpdates()
        assertFalse(current.state.value.update.appUpdate)
    }

    @Test
    fun clientDevicesShowAndSwitchTheirProxy() = runBlocking {
        val api = FakeApi("tokyo")
        val vm = controller(MemoryStore(listOf(tokyo)), mapOf("a" to api))
        vm.showHome(Screen.CLIENTS)
        vm.poll()
        val devices = vm.state.value.clientDevices
        // The panel host always has the client; managed servers need a panel that reports it.
        assertEquals(listOf("tokyo-main", "tokyo-nas"), devices.map { it.server.name })
        assertEquals("hk-01", vm.state.value.clientStates[devices[1].key]?.state?.current?.name)

        vm.openClient(devices[1])
        assertEquals(Screen.CLIENT, vm.state.value.screen)
        vm.clientSelect(2)
        assertEquals("select" to """{"node_id":2}""", api.clientCalls.last())
        assertEquals(2L, vm.state.value.clientStates[devices[1].key]?.state?.currentId)
        assertEquals("已切换到 jp-01", vm.state.value.clientAction)
        vm.clientSetEnabled(false)
        assertFalse(vm.state.value.clientStates[devices[1].key]!!.state!!.settings.enabled)
        vm.clientCheckExit()
        assertEquals("198.51.100.20", vm.state.value.clientExit?.ip)
        vm.clientTest()
        assertEquals("test" to """{"ids":[]}""", api.clientCalls.last())

        assertTrue(vm.back())
        assertEquals(Screen.CLIENTS, vm.state.value.screen)
        assertNull(vm.state.value.clientExit)
        assertFalse(vm.back())

        // From a server's page the client opens and Back returns there.
        vm.openServer(vm.state.value.servers.first { it.server.name == "tokyo-nas" })
        assertEquals("tokyo-nas", vm.state.value.detailClient?.server?.name)
        vm.openClient(vm.state.value.detailClient!!)
        assertTrue(vm.back())
        assertEquals(Screen.DETAIL, vm.state.value.screen)
    }

    @Test
    fun relaySpeedTestRunsFromAnotherServer() = runBlocking {
        val api = FakeApi("tokyo")
        val vm = controller(MemoryStore(listOf(tokyo)), mapOf("a" to api))
        vm.poll()
        vm.openServer(vm.state.value.servers.first { it.server.name == "tokyo-hk" })
        vm.selectDetailTab(DetailTab.SPEED)
        // The panel host and the updated NAS can test this server; the old server cannot test itself.
        assertEquals(listOf("tokyo-main", "tokyo-nas"), vm.state.value.relaySources.map { it.name })
        vm.setSpeedSource(vm.state.value.relaySources.first { it.name == "tokyo-nas" })
        vm.startSpeedTest(SpeedSuite.LATENCY)
        waitFor("the relay test") { !vm.state.value.speed.running }
        val speed = vm.state.value.speed
        assertEquals(RelayOptions(8, listOf("tcp_ping", "udp_ping"), 10, 4, 50), api.relayOptions)
        assertEquals(1, api.relayPolls)
        assertEquals(35.5, speed.tcpPing!!.avgMs, 0.001)
        assertEquals("no reply from 203.0.113.7", speed.errors[SpeedPhase.UDP_PING])
        assertEquals("tokyo-nas", speed.relayName)
        assertEquals("203.0.113.7", speed.host)
        assertNull(speed.error)
    }

    @Test
    fun nodeMonitorsComeFromLinksOrServerInbounds() = runBlocking {
        val api = FakeApi("tokyo")
        val vm = controller(MemoryStore(listOf(tokyo)), mapOf("a" to api))
        vm.showHome(Screen.PROXIES)
        vm.poll()
        vm.editProxy("a")
        vm.setEditKind(MonitorKind.LINK)
        vm.saveEditing()
        assertEquals("请粘贴节点的分享链接", vm.state.value.editing?.error)
        vm.updateEditing { it.copy(link = "vless://id@203.0.113.9:443?security=reality#hk") }
        vm.testEditing()
        assertEquals("node", api.tested.last().type)
        assertEquals("vless://id@203.0.113.9:443?security=reality#hk", api.tested.last().link)

        vm.setEditKind(MonitorKind.INBOUND)
        vm.setNodeServer(8)
        assertEquals(listOf(1L), vm.state.value.editing?.inbounds?.map { it.id })
        vm.saveEditing()
        assertEquals("请选择要监测的节点", vm.state.value.editing?.error)
        vm.updateEditing { it.copy(nodeInboundId = 1, serverId = 8) }
        vm.saveEditing()
        val saved = api.saved.last()
        assertEquals("node", saved.type)
        assertEquals("", saved.link)
        assertEquals(8L, saved.nodeServerId)
        assertEquals(1L, saved.nodeInboundId)
        assertEquals(Screen.PROXIES, vm.state.value.screen)

        // Back to a proxy: the node fields are not sent.
        vm.editProxy("a")
        vm.setEditKind(MonitorKind.INBOUND)
        vm.setEditKind(MonitorKind.PROXY)
        vm.updateEditing { it.copy(link = "socks5://198.51.100.1:1080", nodeInboundId = 1) }
        vm.saveEditing()
        assertEquals(0L, api.saved.last().nodeInboundId)
        assertEquals("socks5", api.saved.last().type)
    }
}
