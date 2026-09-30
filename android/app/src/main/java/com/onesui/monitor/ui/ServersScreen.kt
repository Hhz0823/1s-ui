package com.onesui.monitor.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.Server

private enum class StatusFilter(val label: String) { ALL("全部"), ONLINE("在线"), OFFLINE("离线") }

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServersScreen(state: UiState, vm: MonitorViewModel) {
    var filter by rememberSaveable { mutableStateOf(StatusFilter.ALL) }
    var group by rememberSaveable { mutableStateOf("") }
    var query by rememberSaveable { mutableStateOf("") }
    var searching by rememberSaveable { mutableStateOf(false) }
    val servers = state.overview?.servers.orEmpty()
    val groups = servers.map { it.group }.filter { it.isNotBlank() }.distinct().sorted()
    val shown = servers.filter { server ->
        when (filter) {
            StatusFilter.ALL -> true
            StatusFilter.ONLINE -> server.online
            StatusFilter.OFFLINE -> !server.online
        } && (group.isEmpty() || server.group == group) &&
            (query.isBlank() || listOf(server.name, server.hostname, server.remoteIp, server.publicHost, server.remark)
                .any { it.contains(query.trim(), ignoreCase = true) })
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { PanelSwitcher(state, vm) },
                actions = {
                    IconButton(onClick = { searching = !searching; if (!searching) query = "" }) {
                        Icon(Icons.Default.Search, contentDescription = "搜索")
                    }
                    IconButton(onClick = vm::refresh) { Icon(Icons.Default.Refresh, contentDescription = "刷新") }
                    IconButton(onClick = { vm.navigate(Screen.SETTINGS) }) { Icon(Icons.Default.Settings, contentDescription = "设置") }
                },
            )
        },
    ) { padding ->
        LazyVerticalGrid(
            columns = GridCells.Adaptive(minSize = 320.dp),
            contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = padding.calculateTopPadding() + 4.dp, bottom = padding.calculateBottomPadding() + 16.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.fillMaxSize(),
        ) {
            if (searching) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    OutlinedTextField(
                        value = query,
                        onValueChange = { query = it },
                        placeholder = { Text("按名称、主机名或 IP 搜索") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            item(span = { GridItemSpan(maxLineSpan) }) { SummaryCard(servers, state) }
            item(span = { GridItemSpan(maxLineSpan) }) {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(StatusFilter.entries.toList()) { f ->
                        val count = when (f) {
                            StatusFilter.ALL -> servers.size
                            StatusFilter.ONLINE -> servers.count { it.online }
                            StatusFilter.OFFLINE -> servers.count { !it.online }
                        }
                        FilterChip(selected = filter == f, onClick = { filter = f }, label = { Text("${f.label} $count") })
                    }
                    items(groups) { g ->
                        FilterChip(selected = group == g, onClick = { group = if (group == g) "" else g }, label = { Text(g) })
                    }
                }
            }
            if (state.overview == null) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) {
                        if (state.error != null) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Text(state.error, color = MaterialTheme.colorScheme.error)
                                TextButton(onClick = vm::refresh) { Text("重试") }
                            }
                        } else {
                            CircularProgressIndicator()
                        }
                    }
                }
            }
            items(shown, key = { (if (it.local) "l" else "n") + it.id }) { server ->
                ServerCard(server) { vm.openServer(server) }
            }
        }
    }
}

@Composable
private fun PanelSwitcher(state: UiState, vm: MonitorViewModel) {
    var open by remember { mutableStateOf(false) }
    Box {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.clickable { open = true },
        ) {
            Text(state.panel?.name ?: "服务器监控", maxLines = 1, overflow = TextOverflow.Ellipsis)
            Icon(Icons.Default.ArrowDropDown, contentDescription = "切换面板")
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            state.panels.forEach { panel ->
                DropdownMenuItem(
                    text = { Text(panel.name) },
                    leadingIcon = { if (panel.id == state.panelId) Icon(Icons.Default.Check, contentDescription = null) },
                    onClick = { open = false; vm.selectPanel(panel.id) },
                )
            }
            DropdownMenuItem(text = { Text("绑定新面板…") }, onClick = { open = false; vm.navigate(Screen.BIND) })
        }
    }
}

@Composable
private fun SummaryCard(servers: List<Server>, state: UiState) {
    val online = servers.filter { it.online }
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
        Column(Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text("在线 ${online.size} / ${servers.size}", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
                Spacer(Modifier.weight(1f))
                val stale = state.error != null && state.overview != null
                Text(
                    if (stale) "连接中断：${state.error}" else if (state.updatedAt > 0) "实时" else "",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (stale) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onPrimaryContainer,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f, fill = false),
                )
            }
            Spacer(Modifier.height(10.dp))
            Row {
                LabelValue("↑ 实时上传", Format.rate(online.sumOf { it.netRate.sent }), Modifier.weight(1f))
                LabelValue("↓ 实时下载", Format.rate(online.sumOf { it.netRate.recv }), Modifier.weight(1f))
            }
            Row {
                LabelValue("↑ 总上传", Format.bytes(servers.sumOf { it.network.sent }), Modifier.weight(1f))
                LabelValue("↓ 总下载", Format.bytes(servers.sumOf { it.network.recv }), Modifier.weight(1f))
            }
        }
    }
}

@Composable
fun ServerCard(server: Server, onClick: () -> Unit) {
    Card(
        onClick = onClick,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                StatusDot(server.online)
                Spacer(Modifier.width(8.dp))
                val flag = Format.flag(server.region)
                Text(
                    (if (flag.isNotEmpty()) "$flag " else "") + server.name.ifBlank { server.hostname },
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                if (server.local) Tag("主控")
                Text(
                    if (server.online) Format.uptime(server.uptime) else Format.ago(server.lastSeen),
                    style = MaterialTheme.typography.labelSmall,
                    color = if (server.online) MaterialTheme.colorScheme.onSurfaceVariant else StatusColors.offline,
                )
            }
            val os = Format.osLabel(server)
            if (os.isNotBlank()) {
                Text(os, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Spacer(Modifier.height(10.dp))
            UsageBar("CPU", server.cpuPercent, if (server.cpuCores > 0) "${server.cpuCores} 核" else "")
            Spacer(Modifier.height(6.dp))
            UsageBar("内存", server.memory.percent, "${Format.bytes(server.memory.used)} / ${Format.bytes(server.memory.total)}")
            Spacer(Modifier.height(6.dp))
            UsageBar("磁盘", server.disk.percent, "${Format.bytes(server.disk.used)} / ${Format.bytes(server.disk.total)}")
            Spacer(Modifier.height(10.dp))
            Row {
                RateText("↑", server.netRate.sent, StatusColors.upload, Modifier.weight(1f))
                RateText("↓", server.netRate.recv, StatusColors.download, Modifier.weight(1f))
                Text(
                    "负载 %.2f".format(server.load1),
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            Text(
                "总流量 ↑ ${Format.bytes(server.network.sent)}  ↓ ${Format.bytes(server.network.recv)}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 2.dp),
            )
        }
    }
}

@Composable
private fun RateText(arrow: String, value: Long, color: Color, modifier: Modifier) {
    Text("$arrow ${Format.rate(value)}", style = MaterialTheme.typography.labelMedium, color = color, modifier = modifier)
}

@Composable
fun Tag(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSecondaryContainer,
        modifier = Modifier
            .padding(end = 6.dp)
            .background(MaterialTheme.colorScheme.secondaryContainer, RoundedCornerShape(4.dp))
            .padding(horizontal = 6.dp, vertical = 1.dp),
    )
}
