package com.onesui.monitor.ui

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.Messages
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.PanelProxy
import com.onesui.monitor.data.ProbePoint
import com.onesui.monitor.data.ProxyMonitor
import com.onesui.monitor.data.Screen
import com.onesui.monitor.data.UiState

/** Colors and words for a monitor's status. */
@Composable
fun proxyStatusColor(status: String): Color = when (status) {
    "up" -> StatusColors.online
    "down" -> StatusColors.offline
    "unknown" -> StatusColors.warn
    else -> MaterialTheme.colorScheme.outline
}

fun proxyStatusText(status: String): String = when (status) {
    "up" -> "可用"
    "down" -> "不可用"
    "unknown" -> "无法检测"
    "paused" -> "已暂停"
    else -> "等待检测"
}

fun checkedBy(monitor: ProxyMonitor): String =
    if (monitor.serverId == 0L) "主控检测" else "经 ${monitor.serverName.ifBlank { "服务器 #${monitor.serverId}" }} 检测"

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ProxiesScreen(state: UiState, vm: MonitorController) {
    val proxies = state.proxies
    val manageable = state.scopedPanels.filter { state.features(it.id).manageProxies }
    var choosing by remember { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = { PanelSwitcher(state, vm, "代理监测") },
                actions = {
                    IconButton(onClick = vm::refresh) { Icon(Icons.Default.Refresh, contentDescription = "刷新") }
                    IconButton(onClick = { vm.navigate(Screen.SETTINGS) }) { Icon(Icons.Default.Settings, contentDescription = "设置") }
                },
            )
        },
        bottomBar = { HomeBar(state, vm) },
        floatingActionButton = {
            if (manageable.isNotEmpty()) {
                FloatingActionButton(onClick = {
                    if (manageable.size > 1) choosing = true else vm.editProxy(manageable.first().id)
                }) { Icon(Icons.Default.Add, contentDescription = "添加代理监测") }
            }
        },
    ) { padding ->
        LazyColumn(
            contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = padding.calculateTopPadding() + 4.dp, bottom = padding.calculateBottomPadding() + 88.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.fillMaxSize(),
        ) {
            item { ProxySummary(proxies.map { it.monitor }) }
            state.scopedPanels.forEach { panel ->
                val data = state.data[panel.id]
                val features = state.features(panel.id)
                val note = when {
                    data?.proxiesError != null -> "${panel.name}：${data.proxiesError}"
                    data?.overview != null && !features.proxyMonitors -> "${panel.name}：面板版本过旧，不支持代理监测，请先更新面板"
                    data?.overview != null && !features.manageProxies -> "${panel.name}：面板已关闭 App 管理代理监测，只能查看"
                    else -> null
                }
                if (note != null) {
                    item(key = "note-${panel.id}") {
                        Text(note, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(horizontal = 4.dp))
                    }
                }
            }
            val loaded = state.scopedPanels.any { state.data[it.id]?.proxies != null }
            if (!loaded && state.scopedPanels.none { state.data[it.id]?.proxiesError != null }) {
                item {
                    Row(Modifier.fillMaxWidth().padding(48.dp), horizontalArrangement = Arrangement.Center) { CircularProgressIndicator() }
                }
            } else if (proxies.isEmpty()) {
                item {
                    Text(
                        "还没有代理监测。点右下角 + 添加 SOCKS5 或 HTTP 代理，面板会定时检测它是否可用、延迟多少、出口 IP 是什么。\n\n" +
                            "可以指定由哪台服务器检测：只允许中转服务器 IP 连接的落地代理，就选那台中转服务器。",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(8.dp),
                    )
                }
            }
            items(proxies, key = { "${it.panel.id}:${it.monitor.id}" }) { item ->
                ProxyCard(item, showPanel = state.allPanels) { vm.openProxy(item) }
            }
        }
    }

    if (choosing) {
        AlertDialog(
            onDismissRequest = { choosing = false },
            title = { Text("添加到哪个面板？") },
            text = {
                Column {
                    manageable.forEach { panel ->
                        TextButton(onClick = { choosing = false; vm.editProxy(panel.id) }, modifier = Modifier.fillMaxWidth()) {
                            Text(panel.name)
                        }
                    }
                }
            },
            confirmButton = {},
            dismissButton = { TextButton(onClick = { choosing = false }) { Text("取消") } },
        )
    }
}

@Composable
private fun ProxySummary(monitors: List<ProxyMonitor>) {
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
        Column(Modifier.padding(16.dp).fillMaxWidth()) {
            Text(
                "可用 ${monitors.count { it.status == "up" }} / ${monitors.size}",
                style = MaterialTheme.typography.titleLarge,
                fontWeight = FontWeight.Bold,
            )
            Spacer(Modifier.height(6.dp))
            Row {
                LabelValue("不可用", monitors.count { it.status == "down" }.toString(), Modifier.weight(1f))
                LabelValue("无法检测", monitors.count { it.status == "unknown" }.toString(), Modifier.weight(1f))
                val latencies = monitors.filter { it.status == "up" }.mapNotNull { it.last?.latencyMs }
                LabelValue("平均延迟", if (latencies.isEmpty()) "-" else "${latencies.average().toLong()} ms", Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun ProxyCard(item: PanelProxy, showPanel: Boolean, onClick: () -> Unit) {
    val monitor = item.monitor
    Card(
        onClick = onClick,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                StatusBadge(monitor.status)
                Spacer(Modifier.width(8.dp))
                Text(
                    monitor.name,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                if (showPanel) Tag(item.panel.name)
                Tag(Format.protocol(monitor.type))
                Text(
                    monitor.last?.takeIf { it.ok }?.let { "${it.latencyMs} ms" } ?: proxyStatusText(monitor.status),
                    style = MaterialTheme.typography.labelLarge,
                    color = proxyStatusColor(monitor.status),
                )
            }
            Text(
                "${monitor.host}:${monitor.port} · ${checkedBy(monitor)} · 每 ${Format.interval(monitor.interval)}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.padding(top = 2.dp),
            )
            if (monitor.recent.isNotEmpty()) {
                Spacer(Modifier.height(8.dp))
                RecentStrip(monitor.recent)
            }
            Spacer(Modifier.height(6.dp))
            val exit = monitor.last?.exitIp.orEmpty()
            Text(
                buildString {
                    if (monitor.checks > 0) append("24 小时可用 ${String.format(java.util.Locale.US, "%.1f%%", monitor.uptime)}")
                    if (monitor.avgLatency > 0) append(" · 平均 ${monitor.avgLatency.toLong()} ms")
                    if (exit.isNotEmpty()) append(" · 出口 ${Format.flag(monitor.last?.country.orEmpty())} $exit")
                }.trimStart(' ', '·'),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            val last = monitor.last
            if (last != null && !last.ok && monitor.status != "paused") {
                Text(
                    Messages.probe(last),
                    style = MaterialTheme.typography.labelSmall,
                    color = proxyStatusColor(monitor.status),
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
        }
    }
}

@Composable
fun StatusBadge(status: String) {
    val color = proxyStatusColor(status)
    Canvas(Modifier.width(10.dp).height(10.dp)) { drawCircle(color) }
}

/** The last checks as bars: height is latency, red is a failure, amber could not run. */
@Composable
fun RecentStrip(points: List<ProbePoint>, modifier: Modifier = Modifier) {
    val failed = StatusColors.offline
    val good = StatusColors.online
    val empty = MaterialTheme.colorScheme.surfaceVariant
    val maxLatency = points.filter { it.ok }.maxOfOrNull { it.latencyMs }?.coerceAtLeast(1) ?: 1
    Canvas(modifier.fillMaxWidth().height(28.dp)) {
        val slots = maxOf(points.size, 30)
        val gap = 2.dp.toPx()
        val width = (size.width - gap * (slots - 1)) / slots
        for (i in 0 until slots) {
            val x = i * (width + gap)
            drawRect(empty, topLeft = Offset(x, 0f), size = Size(width, size.height))
        }
        val offset = slots - points.size
        points.forEachIndexed { index, point ->
            val x = (index + offset) * (width + gap)
            val height = if (point.ok) size.height * (0.25f + 0.75f * point.latencyMs / maxLatency.toFloat()) else size.height
            drawRect(if (point.ok) good else failed, topLeft = Offset(x, size.height - height), size = Size(width, height))
        }
    }
}
