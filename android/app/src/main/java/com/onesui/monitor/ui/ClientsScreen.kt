package com.onesui.monitor.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
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
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.ClientDevice
import com.onesui.monitor.data.ClientModes
import com.onesui.monitor.data.ClientNode
import com.onesui.monitor.data.ClientState
import com.onesui.monitor.data.ClientSubscription
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.Messages
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.Screen
import com.onesui.monitor.data.UiState

/** A latency the way the client lists show it, with its color. */
@Composable
fun nodeLatency(node: ClientNode): Pair<String, Color> = when {
    node.delayMs > 0 -> "${node.delayMs} ms" to when {
        node.delayMs < 300 -> StatusColors.online
        node.delayMs < 800 -> StatusColors.warn
        else -> StatusColors.offline
    }
    node.failed -> "失败" to StatusColors.offline
    node.tcpMs > 0 -> "TCP ${node.tcpMs} ms" to MaterialTheme.colorScheme.onSurfaceVariant
    else -> "未测" to MaterialTheme.colorScheme.outline
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ClientsScreen(state: UiState, vm: MonitorController) {
    val devices = state.clientDevices
    Scaffold(
        topBar = {
            TopAppBar(
                title = { PanelSwitcher(state, vm, "客户端代理") },
                actions = {
                    IconButton(onClick = vm::refresh) { Icon(Icons.Default.Refresh, contentDescription = "刷新") }
                    IconButton(onClick = { vm.navigate(Screen.SETTINGS) }) { Icon(Icons.Default.Settings, contentDescription = "设置") }
                },
            )
        },
        bottomBar = { HomeBar(state, vm) },
    ) { padding ->
        LazyColumn(
            contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = padding.calculateTopPadding() + 4.dp, bottom = padding.calculateBottomPadding() + 16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.fillMaxSize(),
        ) {
            item {
                Text(
                    "飞牛 NAS、OpenWrt 软路由等安装了 1S-UI 的设备可以当代理客户端（类似 v2rayN / PassWall）。在这里查看它们走哪个节点、是否可用，并切换节点和分流模式。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(horizontal = 4.dp),
                )
            }
            state.scopedPanels.forEach { panel ->
                val data = state.data[panel.id]
                val note = when {
                    data?.error != null -> "${panel.name}：${data.error}"
                    data?.overview != null && !state.features(panel.id).clients -> "${panel.name}：面板版本过旧，不支持客户端代理，请先更新面板"
                    else -> null
                }
                if (note != null) {
                    item(key = "note-${panel.id}") {
                        Text(note, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall, modifier = Modifier.padding(horizontal = 4.dp))
                    }
                }
            }
            val loaded = state.scopedPanels.any { state.data[it.id]?.overview != null }
            if (!loaded && state.panelErrors.isEmpty()) {
                item { Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() } }
            } else if (devices.isEmpty() && loaded) {
                item {
                    Text(
                        "还没有可以管理的客户端设备。在飞牛 NAS 或 OpenWrt 上安装 1S-UI 并绑定主控（面板「服务器监控 → 添加子服务器 → 密钥接入」），设备上线后会出现在这里。",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        modifier = Modifier.padding(8.dp),
                    )
                }
            }
            items(devices, key = { it.key }) { device ->
                ClientCard(device, state, showPanel = state.allPanels) { vm.openClient(device) }
            }
        }
    }
}

@Composable
private fun ClientCard(device: ClientDevice, state: UiState, showPanel: Boolean, onClick: () -> Unit) {
    val view = state.clientStates[device.key]
    val client = view?.state
    Card(
        onClick = onClick,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                StatusDot(device.server.online)
                Spacer(Modifier.width(8.dp))
                Text(
                    device.server.name.ifBlank { device.server.hostname },
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                if (showPanel) Tag(device.panel.name)
                if (device.server.local) Tag("主控")
                if (client?.platform?.openWrt == true) Tag("OpenWrt")
                Text(
                    when {
                        client == null -> ""
                        client.settings.enabled && client.applied -> "已开启"
                        client.settings.enabled -> "未生效"
                        else -> "已关闭"
                    },
                    style = MaterialTheme.typography.labelLarge,
                    color = when {
                        client == null -> MaterialTheme.colorScheme.outline
                        client.settings.enabled && client.applied && client.problem.isBlank() -> StatusColors.online
                        client.settings.enabled -> StatusColors.warn
                        else -> MaterialTheme.colorScheme.outline
                    },
                )
            }
            Spacer(Modifier.height(4.dp))
            when {
                !device.server.online -> Text("设备离线 · ${Format.ago(device.server.lastSeen)}", style = MaterialTheme.typography.bodySmall, color = StatusColors.offline)
                client != null -> {
                    Text(client.summary, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    client.current?.let { node ->
                        val (latency, color) = nodeLatency(node)
                        Text(
                            "${Format.protocol(node.protocol)} · $latency · ${client.nodes.size} 个节点",
                            style = MaterialTheme.typography.labelSmall,
                            color = color,
                        )
                    }
                    if (client.settings.enabled && client.problem.isNotBlank()) {
                        Text(Messages.panel(client.problem), style = MaterialTheme.typography.labelSmall, color = StatusColors.warn)
                    }
                }
                view?.error != null -> Text(view.error, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
                else -> Text("正在读取…", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun ClientScreen(state: UiState, vm: MonitorController) {
    val device = state.openClient
    val view = state.clientStates[state.clientKey]
    val client = view?.state
    val manage = device?.manage == true && device.server.online
    var byLatency by rememberSaveable { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Column {
                        Text(device?.server?.name ?: "客户端代理", maxLines = 1, overflow = TextOverflow.Ellipsis)
                        Text(
                            "客户端代理" + if (state.panels.size > 1) " · ${device?.panel?.name.orEmpty()}" else "",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
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
        LazyColumn(
            contentPadding = PaddingValues(start = 12.dp, end = 12.dp, top = padding.calculateTopPadding() + 4.dp, bottom = padding.calculateBottomPadding() + 24.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.fillMaxSize(),
        ) {
            if (device == null || client == null) {
                item {
                    Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) {
                        val error = view?.error
                        if (error != null) Text(error, color = MaterialTheme.colorScheme.error) else CircularProgressIndicator()
                    }
                }
                return@LazyColumn
            }
            view?.error?.let { error ->
                item { Text("刷新失败：$error", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
            }
            if (!device.manage) {
                item {
                    Text(
                        "面板已关闭 App 管理客户端代理，只能查看。可在面板「设置 → 前端与后端 → 手机监控 App」里开启。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            item { ClientStatus(state, vm, client, manage) }
            state.clientAction?.let { action ->
                item {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        if (state.clientBusy) {
                            CircularProgressIndicator(Modifier.height(18.dp).width(18.dp), strokeWidth = 2.dp)
                            Spacer(Modifier.width(8.dp))
                        }
                        Text(action, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                        if (!state.clientBusy) TextButton(onClick = vm::clearClientAction) { Text("知道了") }
                    }
                }
            }
            item {
                Section("分流模式") {
                    ChoiceRow(ClientModes.all, client.settings.mode, ClientModes::label) { mode ->
                        if (manage && !state.clientBusy && mode != client.settings.mode) vm.clientSetMode(mode)
                    }
                    Text(ClientModes.hint(client.settings.mode), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
            item {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("节点 ${client.nodes.size}", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                    FilterChip(selected = byLatency, onClick = { byLatency = !byLatency }, label = { Text("按延迟") })
                }
            }
            if (manage) {
                item {
                    FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(onClick = { vm.clientTest() }, enabled = !state.clientBusy && !client.testing && client.nodes.isNotEmpty()) { Text("全部测延迟") }
                        OutlinedButton(onClick = { vm.clientSelect(0) }, enabled = !state.clientBusy && !client.settings.auto) { Text("自动选择最快") }
                    }
                }
            }
            if (client.testing) {
                item { LinearProgressIndicator(Modifier.fillMaxWidth()) }
            }
            if (client.nodes.isEmpty()) {
                item {
                    Text(
                        "还没有节点。请在设备的 1S-UI 面板「客户端代理」里添加订阅或分享链接。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            val nodes = if (byLatency) {
                client.nodes.sortedWith(compareBy<ClientNode> { if (it.delayMs > 0) 0 else 1 }.thenBy { it.delayMs })
            } else {
                client.nodes
            }
            items(nodes, key = { "node-${it.id}" }) { node ->
                NodeRow(node, current = node.id == client.currentId, enabled = manage && !state.clientBusy) { vm.clientSelect(node.id) }
            }
            if (client.subscriptions.isNotEmpty()) {
                item {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text("订阅 ${client.subscriptions.size}", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold, modifier = Modifier.weight(1f))
                        if (manage) TextButton(onClick = { vm.clientUpdateSubscription(0) }, enabled = !state.clientBusy) { Text("全部更新") }
                    }
                }
                items(client.subscriptions, key = { "sub-${it.id}" }) { subscription ->
                    SubscriptionCard(subscription, manage && !state.clientBusy) { vm.clientUpdateSubscription(subscription.id) }
                }
            }
        }
    }
}

@Composable
private fun ClientStatus(state: UiState, vm: MonitorController, client: ClientState, manage: Boolean) {
    Section("代理") {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                val on = client.settings.enabled
                Text(
                    if (on) (if (client.applied) "已开启" else "已开启，未生效") else "已关闭",
                    style = MaterialTheme.typography.headlineSmall,
                    fontWeight = FontWeight.Bold,
                    color = when {
                        on && client.applied && client.problem.isBlank() -> StatusColors.online
                        on -> StatusColors.warn
                        else -> MaterialTheme.colorScheme.onSurfaceVariant
                    },
                )
                Text(client.summary, style = MaterialTheme.typography.bodyMedium)
            }
            Switch(
                checked = client.settings.enabled,
                onCheckedChange = { vm.clientSetEnabled(it) },
                enabled = manage && !state.clientBusy,
            )
        }
        if (client.settings.enabled && client.problem.isNotBlank()) {
            Text(Messages.panel(client.problem), style = MaterialTheme.typography.bodySmall, color = StatusColors.warn)
        }
        client.current?.let { node ->
            Spacer(Modifier.height(6.dp))
            val (latency, color) = nodeLatency(node)
            Row {
                LabelValue("当前节点", node.name, Modifier.weight(2f))
                Column(Modifier.weight(1f).padding(vertical = 4.dp)) {
                    Text("延迟", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    Text(latency, style = MaterialTheme.typography.bodyMedium, color = color)
                }
            }
        }
        val exit = state.clientExit
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                if (exit != null && exit.ok) {
                    "出口 ${Format.flag(exit.country)} ${exit.ip}" + if (exit.delayMs > 0) " · ${exit.delayMs} ms" else ""
                } else {
                    "检测经过代理访问外网时的出口 IP"
                },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.weight(1f),
            )
            TextButton(onClick = vm::clientCheckExit, enabled = !state.clientBusy && client.applied) { Text("检测出口") }
        }
        HorizontalDivider(Modifier.padding(vertical = 6.dp))
        Text(
            listOfNotNull(
                if (client.platform.openWrt) "OpenWrt" else client.platform.os.ifBlank { null },
                if (client.settings.tun) "透明代理已开启" else "透明代理未开启",
                if (client.settings.mixed && client.settings.mixedPort > 0) "代理端口 ${client.settings.mixedPort}" else null,
                if (client.settings.dnsHijack && client.platform.openWrt) "接管局域网 DNS" else null,
            ).joinToString(" · "),
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun NodeRow(node: ClientNode, current: Boolean, enabled: Boolean, onSelect: () -> Unit) {
    val (latency, color) = nodeLatency(node)
    Card(
        colors = CardDefaults.cardColors(
            containerColor = if (current) MaterialTheme.colorScheme.secondaryContainer else MaterialTheme.colorScheme.surface,
        ),
        modifier = Modifier.fillMaxWidth().clickable(enabled = enabled && !current, onClick = onSelect),
    ) {
        Row(Modifier.padding(horizontal = 12.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.width(24.dp)) {
                if (current) Icon(Icons.Default.Check, contentDescription = "当前节点", tint = MaterialTheme.colorScheme.primary)
            }
            Column(Modifier.weight(1f)) {
                Text(node.name, style = MaterialTheme.typography.bodyMedium, fontWeight = if (current) FontWeight.SemiBold else FontWeight.Normal, maxLines = 1, overflow = TextOverflow.Ellipsis)
                Text(
                    listOf(Format.protocol(node.protocol), node.subscription.ifBlank { "手动添加" }).joinToString(" · "),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (node.failed) {
                    Text(Messages.panel(node.testError), style = MaterialTheme.typography.labelSmall, color = StatusColors.offline, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            Text(latency, style = MaterialTheme.typography.labelLarge, color = color)
        }
    }
}

@Composable
private fun SubscriptionCard(subscription: ClientSubscription, canUpdate: Boolean, onUpdate: () -> Unit) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
    ) {
        Column(Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) {
                    Text(subscription.name, style = MaterialTheme.typography.bodyLarge, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(
                        "${subscription.nodeCount} 个节点 · " + if (subscription.updatedAt > 0) "${Format.ago(subscription.updatedAt)}更新" else "未更新",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                if (canUpdate) TextButton(onClick = onUpdate) { Text("更新") }
            }
            if (subscription.total > 0) {
                Spacer(Modifier.height(4.dp))
                UsageBar(
                    "流量",
                    subscription.used * 100.0 / subscription.total,
                    "${Format.bytes(subscription.used)} / ${Format.bytes(subscription.total)}",
                )
            }
            if (subscription.expire > 0) {
                Text(
                    "到期 ${Format.date(subscription.expire)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (subscription.expire * 1000 < System.currentTimeMillis()) StatusColors.offline else MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
            if (subscription.lastError.isNotBlank()) {
                Text(
                    "上次更新失败：" + Messages.panel(subscription.lastError),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.padding(top = 2.dp),
                )
            }
        }
    }
}
