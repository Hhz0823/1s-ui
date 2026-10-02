package com.onesui.monitor.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.DetailTab
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.Server
import com.onesui.monitor.data.UiState

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DetailScreen(state: UiState, vm: MonitorController) {
    val server = state.detail
    val features = state.features(state.detailPanelId)
    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        if (server != null) {
                            StatusDot(server.online)
                            Spacer(Modifier.width(8.dp))
                        }
                        Column {
                            Text(server?.name ?: "服务器", maxLines = 1, overflow = TextOverflow.Ellipsis)
                            if (state.panels.size > 1) {
                                Text(
                                    state.detailPanel?.name.orEmpty(),
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    maxLines = 1,
                                )
                            }
                        }
                    }
                },
                navigationIcon = {
                    IconButton(onClick = { vm.back() }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回") }
                },
                actions = {
                    IconButton(onClick = vm::refresh) { Icon(Icons.Default.Refresh, contentDescription = "刷新") }
                },
            )
        },
    ) { padding ->
        if (server == null) return@Scaffold
        Column(Modifier.fillMaxSize().padding(padding)) {
            TabRow(selectedTabIndex = state.detailTab.ordinal) {
                DetailTab.entries.forEach { tab ->
                    Tab(
                        selected = state.detailTab == tab,
                        onClick = { vm.selectDetailTab(tab) },
                        text = {
                            Text(
                                when (tab) {
                                    DetailTab.OVERVIEW -> "概览"
                                    DetailTab.NODES -> "节点"
                                    DetailTab.SPEED -> "测速"
                                }
                            )
                        },
                    )
                }
            }
            when (state.detailTab) {
                DetailTab.OVERVIEW -> OverviewTab(state, vm, server)
                DetailTab.NODES -> if (features.nodes) NodesTab(state, vm) else OutdatedPanel("查看节点")
                DetailTab.SPEED -> if (features.speedtest) SpeedTab(state, vm) else OutdatedPanel("测速", features.proxyMonitors)
            }
        }
    }
}

/** A tab the bound panel cannot serve: too old, or turned off by the admin. */
@Composable
fun OutdatedPanel(feature: String, turnedOff: Boolean = false) {
    Box(Modifier.fillMaxSize().padding(32.dp), contentAlignment = Alignment.Center) {
        Text(
            if (turnedOff) "面板已关闭 App $feature，可在面板「设置 → 前端与后端 → 手机监控 App」里开启。"
            else "面板版本过旧，不支持$feature，请先更新面板。",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun OverviewTab(state: UiState, vm: MonitorController, server: Server) {
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        state.detailError?.let {
            Text("连接中断：$it", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        Section("实时状态") {
            UsageBar("CPU", server.cpuPercent, if (server.cpuCores > 0) "${server.cpuCores} 核" else "")
            Spacer(Modifier.height(8.dp))
            UsageBar("内存", server.memory.percent, "${Format.bytes(server.memory.used)} / ${Format.bytes(server.memory.total)}")
            if (server.swap.total > 0) {
                Spacer(Modifier.height(8.dp))
                UsageBar("交换", server.swap.percent, "${Format.bytes(server.swap.used)} / ${Format.bytes(server.swap.total)}")
            }
            Spacer(Modifier.height(8.dp))
            UsageBar("磁盘", server.disk.percent, "${Format.bytes(server.disk.used)} / ${Format.bytes(server.disk.total)}")
            Spacer(Modifier.height(8.dp))
            Row {
                LabelValue("↑ 上传速度", Format.rate(server.netRate.sent), Modifier.weight(1f))
                LabelValue("↓ 下载速度", Format.rate(server.netRate.recv), Modifier.weight(1f))
            }
            Row {
                LabelValue("↑ 总上传", Format.bytes(server.network.sent), Modifier.weight(1f))
                LabelValue("↓ 总下载", Format.bytes(server.network.recv), Modifier.weight(1f))
            }
            Row {
                LabelValue("负载", "%.2f / %.2f / %.2f".format(server.load1, server.load5, server.load15), Modifier.weight(1f))
                LabelValue("进程", if (server.processCount > 0) server.processCount.toString() else "-", Modifier.weight(1f))
            }
            if (server.tcpConns > 0 || server.udpConns > 0) {
                Row {
                    LabelValue("TCP 连接", server.tcpConns.toString(), Modifier.weight(1f))
                    LabelValue("UDP 连接", server.udpConns.toString(), Modifier.weight(1f))
                }
            }
        }

        state.detailClient?.let { device ->
            Section("客户端代理") {
                val client = state.clientStates[device.key]?.state
                Text(
                    client?.summary ?: "这台设备的 1S-UI 可以当代理客户端（类似 v2rayN / PassWall）：查看它走哪个节点，切换节点和分流模式。",
                    style = MaterialTheme.typography.bodyMedium,
                )
                Spacer(Modifier.height(6.dp))
                OutlinedButton(onClick = { vm.openClient(device) }) { Text("打开客户端代理") }
            }
        }

        val history = server.history
        if (history.size >= 2) {
            val start = Format.time(history.first().time)
            val end = Format.time(history.last().time)
            Section("CPU / 内存") {
                LineChart(
                    series = listOf(
                        ChartSeries("CPU", MaterialTheme.colorScheme.primary, history.map { it.cpu }),
                        ChartSeries("内存", StatusColors.online, history.map { it.mem }),
                    ),
                    maxValue = 100.0,
                    formatValue = Format::percent,
                    startLabel = start,
                    endLabel = end,
                )
            }
            Section("网络速度") {
                LineChart(
                    series = listOf(
                        ChartSeries("上传", StatusColors.upload, history.map { it.netSentRate.toDouble() }),
                        ChartSeries("下载", StatusColors.download, history.map { it.netRecvRate.toDouble() }),
                    ),
                    maxValue = null,
                    formatValue = { Format.rate(it.toLong()) },
                    startLabel = start,
                    endLabel = end,
                )
            }
            Section("磁盘") {
                LineChart(
                    series = listOf(ChartSeries("磁盘", StatusColors.warn, history.map { it.disk })),
                    maxValue = 100.0,
                    formatValue = Format::percent,
                    startLabel = start,
                    endLabel = end,
                )
            }
        } else {
            Text(
                "历史曲线会在面板积累几分钟数据后出现。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        server.latency?.let { latency ->
            Section("与面板的延迟") {
                Row {
                    LabelValue("当前", latency.lastMs?.let { "$it ms" } ?: "超时", Modifier.weight(1f))
                    LabelValue("平均", "%.0f ms".format(latency.averageMs), Modifier.weight(1f))
                }
                Row {
                    LabelValue("P95", "${latency.p95Ms} ms", Modifier.weight(1f))
                    LabelValue("丢包", "%.1f%%".format(latency.lossPct), Modifier.weight(1f))
                }
            }
        }

        Section("系统信息") { SystemInfo(server, versionText(server.version, state.update)) }
    }
}

@Composable
private fun SystemInfo(server: Server, version: String) {
    SelectionContainer {
        Column {
            Row {
                LabelValue("主机名", server.hostname, Modifier.weight(1f))
                LabelValue("系统", Format.osLabel(server), Modifier.weight(1f))
            }
            if (server.cpuModel.isNotBlank() || server.virtualization.isNotBlank()) {
                Row {
                    LabelValue("CPU 型号", server.cpuModel, Modifier.weight(1f))
                    LabelValue("虚拟化", server.virtualization, Modifier.weight(1f))
                }
            }
            if (server.kernel.isNotBlank()) LabelValue("内核", server.kernel)
            Row {
                LabelValue("运行时间", Format.uptime(server.uptime), Modifier.weight(1f))
                LabelValue("最后上报", if (server.local) "实时" else Format.ago(server.lastSeen), Modifier.weight(1f))
            }
            Row {
                LabelValue("sing-box", if (server.singBoxRunning) "运行中" else "未运行", Modifier.weight(1f))
                LabelValue(
                    "Xray",
                    if (server.xrayRunning) "运行中" + (server.xrayVersion.takeIf { it.isNotBlank() }?.let { " $it" } ?: "") else "未运行",
                    Modifier.weight(1f),
                )
            }
            Row {
                LabelValue("版本", version, Modifier.weight(1f))
                LabelValue("连接方式", if (server.local) "主控本机" else server.connMode.uppercase(), Modifier.weight(1f))
            }
            val ips = (listOf(server.publicHost, server.remoteIp) + server.ipv4 + server.ipv6).filter { it.isNotBlank() }.distinct()
            if (ips.isNotEmpty()) LabelValue("IP 地址", ips.joinToString("\n"))
            if (server.group.isNotBlank() || server.remark.isNotBlank()) {
                Row {
                    LabelValue("分组", server.group, Modifier.weight(1f))
                    LabelValue("备注", server.remark, Modifier.weight(1f))
                }
            }
            if (server.expireAt > 0 || server.price > 0) {
                Row {
                    LabelValue("到期", if (server.expireAt > 0) Format.date(server.expireAt) else "长期", Modifier.weight(1f))
                    LabelValue("价格", if (server.price > 0) "${server.currency}${"%.2f".format(server.price)}" else "免费", Modifier.weight(1f))
                }
            }
        }
    }
}

@Composable
fun Section(title: String, content: @Composable () -> Unit) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(Modifier.padding(14.dp)) {
            Text(title, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Spacer(Modifier.height(10.dp))
            content()
        }
    }
}
