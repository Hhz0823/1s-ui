package com.onesui.monitor.data

import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.Base64

class DataTest {
    @Test
    fun normalizesPanelAddresses() {
        assertEquals("http://1.2.3.4:2095/", PanelUrl.normalize("1.2.3.4:2095"))
        assertEquals("https://panel.example.com/app/", PanelUrl.normalize(" https://panel.example.com/app/#/agents?x=1 "))
        assertEquals("http://[2001:db8::1]:2095/", PanelUrl.normalize("http://[2001:db8::1]:2095"))
        assertNull(PanelUrl.normalize("ftp://host/"))
        assertNull(PanelUrl.normalize(""))
    }

    @Test
    fun parsesBindCodeFromPanel() {
        // Same encoding as frontend/src/components/MonitorAppKey.vue.
        val json = """{"v":1,"url":"http://1.2.3.4:2095/","key":"abc_DEF-123"}"""
        val encoded = Base64.getUrlEncoder().withoutPadding().encodeToString(json.toByteArray())
        val result = BindCode.parse("1sui-monitor:$encoded")
        assertEquals("http://1.2.3.4:2095/", result?.url)
        assertEquals("abc_DEF-123", result?.key)
        assertNull(BindCode.parse("vless://whatever"))
        assertNull(BindCode.parse("1sui-monitor:not-base64!"))
    }

    @Test
    fun parsesMonitorOverview() {
        val json = JSONObject(
            """
            {"panel_version":"1.6.4","server_time":100,"servers":[
              {"id":0,"name":"main","local":true,"online":true,"report":{"cpu_percent":12.5,
                "memory":{"used":512,"total":1024},"disk":{"used":1,"total":4},
                "net_rate":{"sent":10,"recv":20},"load":{"load1":0.5},"cores":{"singbox_running":true}}},
              {"id":3,"name":"hk","online":false,"last_seen":90,"region":"hk",
                "latency":{"last_ms":null,"average_ms":20.5,"p95_ms":40,"loss_pct":10,"samples":5},
                "history":[{"time":1,"cpu_percent":5,"net_sent_rate":7}]}
            ]}
            """.trimIndent()
        )
        val overview = MonitorJson.parseOverview(json)
        assertEquals(2, overview.servers.size)
        val main = overview.servers[0]
        assertTrue(main.local)
        assertEquals(50.0, main.memory.percent, 0.01)
        assertEquals(20L, main.netRate.recv)
        assertTrue(main.singBoxRunning)
        val hk = overview.servers[1]
        assertNull(hk.latency?.lastMs)
        assertEquals(1, hk.history.size)
        assertEquals("🇭🇰", Format.flag(hk.region))
    }

    @Test
    fun alertsFireOnlyOnChanges() {
        val panel = Panel("p", "Panel", "http://x/", "k")
        val settings = AppSettings(cpuThreshold = 90)
        fun server(online: Boolean, cpu: Double) = MonitorJson.parseServer(
            JSONObject().put("id", 1).put("name", "hk").put("online", online)
                .put("report", JSONObject().put("cpu_percent", cpu))
        )

        val (first, s1) = AlertEvaluator.evaluate(panel, listOf(server(false, 0.0)), emptyMap(), settings)
        assertTrue("first sight only records state", first.isEmpty())
        val (back, s2) = AlertEvaluator.evaluate(panel, listOf(server(true, 95.0)), s1, settings)
        assertEquals(listOf("hk 已恢复在线", "hk CPU 过高"), back.map { it.title })
        val (same, s3) = AlertEvaluator.evaluate(panel, listOf(server(true, 96.0)), s2, settings)
        assertTrue(same.isEmpty())
        val (down, _) = AlertEvaluator.evaluate(panel, listOf(server(false, 0.0)), s3, settings)
        assertEquals(listOf("hk 已离线"), down.map { it.title })
    }

    @Test
    fun formatsSizes() {
        assertEquals("0 B", Format.bytes(0))
        assertEquals("1.5 KB", Format.bytes(1536))
        assertEquals("1.0 GB/s", Format.rate(1L shl 30))
    }
}
