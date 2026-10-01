package com.onesui.monitor.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Edit
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.Messages
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.ProbeResult
import com.onesui.monitor.data.UiState

private val ranges = listOf(3_600L to "1 小时", 21_600L to "6 小时", 86_400L to "24 小时", 259_200L to "3 天")

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun ProxyDetailScreen(state: UiState, vm: MonitorController) {
    val detail = state.proxyDetail
    val monitor = detail?.monitor ?: state.proxies.firstOrNull { it.panel.id == state.proxyPanelId && it.monitor.id == state.proxyId }?.monitor
    val canManage = state.features(state.proxyPanelId).manageProxies
    var deleting by remember { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        if (monitor != null) {
                            StatusBadge(monitor.status)
                            Spacer(Modifier.width(8.dp))
                        }
                        Text(monitor?.name ?: "代理监测", maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                },
                navigationIcon = {
                    IconButton(onClick = { vm.back() }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回") }
                },
                actions = {
                    if (canManage && monitor != null) {
                        IconButton(onClick = { state.proxyPanelId?.let { vm.editProxy(it, monitor) } }) {
                            Icon(Icons.Default.Edit, contentDescription = "编辑")
                        }
                    }
                    IconButton(onClick = vm::refresh) { Icon(Icons.Default.Refresh, contentDescription = "刷新") }
                },
            )
        },
    ) { padding ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 12.dp, vertical = 4.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            state.proxyError?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall) }
            if (monitor == null) {
                Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
                return@Column
            }
            Section("状态") {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        proxyStatusText(monitor.status),
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.Bold,
                        color = proxyStatusColor(monitor.status),
                    )
                    Spacer(Modifier.weight(1f))
                    Tag(Format.protocol(monitor.type))
                }
                SelectionContainer {
                    Text(
                        "${monitor.host}:${monitor.port}" + if (monitor.username.isNotBlank()) " · 用户 ${monitor.username}" else "",
                        style = MaterialTheme.typography.bodyMedium,
                    )
                }
                Text(
                    "${checkedBy(monitor)} · 每 ${Format.interval(monitor.interval)}" + if (monitor.target.isNotBlank()) " · ${monitor.target}" else "",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                monitor.last?.let { last ->
                    Spacer(Modifier.height(8.dp))
                    ProbeSummary(last)
                }
            }
            state.proxyAction?.let { action ->
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(action, style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                    TextButton(onClick = vm::clearProxyAction) { Text("知道了") }
                }
            }
            if (canManage) {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = vm::checkProxyNow) { Text("立即检测") }
                    OutlinedButton(onClick = { vm.setProxyEnabled(!monitor.enabled) }) { Text(if (monitor.enabled) "暂停检测" else "恢复检测") }
                    OutlinedButton(onClick = { deleting = true }) { Text("删除", color = MaterialTheme.colorScheme.error) }
                }
            }

            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                ranges.forEach { (seconds, label) ->
                    FilterChip(selected = state.proxyRange == seconds, onClick = { vm.setProxyRange(seconds) }, label = { Text(label) })
                }
            }
            if (detail != null) {
                val checks = detail.points.sumOf { it.checks }
                val failures = detail.points.sumOf { it.failures }
                Section("延迟与可用率") {
                    Row {
                        LabelValue("可用率", if (checks > 0) String.format(java.util.Locale.US, "%.2f%%", (checks - failures) * 100.0 / checks) else "-", Modifier.weight(1f))
                        LabelValue("检测次数", checks.toString(), Modifier.weight(1f))
                        LabelValue("失败", failures.toString(), Modifier.weight(1f))
                    }
                    val points = detail.points.filter { it.latencyMs > 0 }
                    if (points.size >= 2) {
                        Spacer(Modifier.height(8.dp))
                        LineChart(
                            series = listOf(ChartSeries("延迟", MaterialTheme.colorScheme.primary, points.map { it.latencyMs })),
                            maxValue = null,
                            formatValue = { "${it.toLong()} ms" },
                            startLabel = Format.time(points.first().time),
                            endLabel = Format.time(points.last().time),
                        )
                    } else {
                        Text("检测几次后会显示延迟曲线。", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
                if (detail.exitIps.isNotEmpty()) {
                    Section("出口 IP") {
                        detail.exitIps.forEach { exit ->
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text("${Format.flag(exit.country)} ${exit.ip}", style = MaterialTheme.typography.bodyMedium, modifier = Modifier.weight(1f))
                                Text(
                                    "${exit.checks} 次 · 最近 ${Format.dateTime(exit.lastSeen)}",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                    }
                }
                if (detail.failures.isNotEmpty()) {
                    Section("最近失败") {
                        detail.failures.forEach { failure ->
                            Column(Modifier.padding(vertical = 3.dp)) {
                                Text(Format.dateTime(failure.time), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                Text(Messages.probe(failure), style = MaterialTheme.typography.bodySmall)
                            }
                        }
                    }
                }
            }
            Spacer(Modifier.height(16.dp))
        }
    }

    if (deleting) {
        AlertDialog(
            onDismissRequest = { deleting = false },
            title = { Text("删除这个代理监测？") },
            text = { Text("面板会删除它和全部检测记录。") },
            confirmButton = { TextButton(onClick = { deleting = false; vm.deleteProxy() }) { Text("删除") } },
            dismissButton = { TextButton(onClick = { deleting = false }) { Text("取消") } },
        )
    }
}

/** One check: latency broken into steps, exit IP, or why it failed. */
@Composable
fun ProbeSummary(result: ProbeResult) {
    if (!result.ok) {
        Text(Messages.probe(result), style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.error)
        if (result.time > 0) {
            Text(Format.dateTime(result.time), style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        return
    }
    Row {
        LabelValue("总延迟", "${result.latencyMs} ms", Modifier.weight(1f))
        LabelValue("连接代理", "${result.connectMs} ms", Modifier.weight(1f))
        LabelValue("代理握手", if (result.handshakeMs > 0) "${result.handshakeMs} ms" else "-", Modifier.weight(1f))
    }
    Row {
        LabelValue("TLS", if (result.tlsMs > 0) "${result.tlsMs} ms" else "-", Modifier.weight(1f))
        LabelValue("首包", "${result.ttfbMs} ms", Modifier.weight(1f))
        LabelValue("出口", if (result.exitIp.isNotBlank()) "${Format.flag(result.country)} ${result.exitIp}" else "-", Modifier.weight(1f))
    }
    if (result.time > 0) {
        Text("检测于 ${Format.dateTime(result.time)}", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

private val intervals = listOf(30, 60, 300, 600, 1800)

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun ProxyEditScreen(state: UiState, vm: MonitorController) {
    val editing = state.editing ?: return
    val input = editing.input
    val servers = state.data[editing.panelId]?.overview?.servers.orEmpty()
    var portText by remember(editing.panelId, input.id) { mutableStateOf(if (input.port > 0) input.port.toString() else "") }
    var serverMenu by remember { mutableStateOf(false) }
    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(if (input.id == 0L) "添加代理监测" else "编辑代理监测") },
                navigationIcon = {
                    IconButton(onClick = { vm.back() }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回") }
                },
            )
        },
    ) { padding ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            if (state.panels.size > 1) {
                Text("面板：${state.panels.firstOrNull { it.id == editing.panelId }?.name.orEmpty()}", style = MaterialTheme.typography.labelLarge)
            }
            OutlinedTextField(
                value = input.link,
                onValueChange = { value -> vm.updateEditing { it.copy(link = value) } },
                label = { Text("粘贴代理链接（可选）") },
                placeholder = { Text("socks5://用户:密码@IP:端口") },
                supportingText = { Text("支持 socks5://、http://、v2rayN 的 socks:// 链接和 IP:端口:用户名:密码；填了链接会代替下面的地址和账号") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = input.name,
                onValueChange = { value -> vm.updateEditing { it.copy(name = value) } },
                label = { Text("名称（可选）") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            if (input.link.isBlank()) {
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(selected = input.type == "socks5", onClick = { vm.updateEditing { it.copy(type = "socks5") } }, label = { Text("SOCKS5") })
                    FilterChip(selected = input.type == "http", onClick = { vm.updateEditing { it.copy(type = "http") } }, label = { Text("HTTP") })
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    OutlinedTextField(
                        value = input.host,
                        onValueChange = { value -> vm.updateEditing { it.copy(host = value.trim()) } },
                        label = { Text("地址") },
                        singleLine = true,
                        modifier = Modifier.weight(2f),
                    )
                    OutlinedTextField(
                        value = portText,
                        onValueChange = { value ->
                            portText = value.filter { it.isDigit() }.take(5)
                            vm.updateEditing { it.copy(port = portText.toIntOrNull() ?: 0) }
                        },
                        label = { Text("端口") },
                        singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                        modifier = Modifier.weight(1f),
                    )
                }
                OutlinedTextField(
                    value = input.username,
                    onValueChange = { value -> vm.updateEditing { it.copy(username = value) } },
                    label = { Text("用户名（可选）") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                OutlinedTextField(
                    value = input.password.orEmpty(),
                    onValueChange = { value -> vm.updateEditing { it.copy(password = value) } },
                    label = { Text("密码（可选）") },
                    placeholder = { if (editing.hadPassword) Text("留空表示不修改") },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    modifier = Modifier.fillMaxWidth(),
                )
            }

            Text("由哪台服务器检测", style = MaterialTheme.typography.labelLarge)
            Box {
                val current = servers.firstOrNull { !it.local && it.id == input.serverId }
                OutlinedButton(onClick = { serverMenu = true }, modifier = Modifier.fillMaxWidth()) {
                    Text(
                        if (input.serverId == 0L) "主控本机" else current?.name ?: "服务器 #${input.serverId}",
                        modifier = Modifier.weight(1f),
                    )
                    Text("更换", color = MaterialTheme.colorScheme.primary)
                }
                DropdownMenu(expanded = serverMenu, onDismissRequest = { serverMenu = false }) {
                    DropdownMenuItem(text = { Text("主控本机") }, onClick = { serverMenu = false; vm.updateEditing { it.copy(serverId = 0) } })
                    servers.filter { !it.local }.forEach { server ->
                        DropdownMenuItem(
                            text = { Text(server.name + if (server.online) "" else "（离线）") },
                            onClick = { serverMenu = false; vm.updateEditing { it.copy(serverId = server.id) } },
                        )
                    }
                }
            }
            Text(
                "检测从这台服务器发起。代理只允许中转服务器 IP 连接时，选那台中转服务器；子服务器需要已更新到支持代理检测的版本。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )

            Text("检测间隔", style = MaterialTheme.typography.labelLarge)
            ChoiceRow(intervals, input.interval, { Format.interval(it) }) { value -> vm.updateEditing { it.copy(interval = value) } }

            OutlinedTextField(
                value = input.target,
                onValueChange = { value -> vm.updateEditing { it.copy(target = value.trim()) } },
                label = { Text("测试网址（可选）") },
                placeholder = { Text("默认 Cloudflare，同时显示出口 IP") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            SwitchRow("启用检测", input.enabled) { value -> vm.updateEditing { it.copy(enabled = value) } }

            editing.error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            editing.testResult?.let { result ->
                Section(if (result.ok) "测试通过" else "测试失败") { ProbeSummary(result) }
            }
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = vm::testEditing, enabled = !editing.busy, modifier = Modifier.weight(1f)) { Text("测试") }
                Button(onClick = vm::saveEditing, enabled = !editing.busy, modifier = Modifier.weight(1f)) {
                    if (editing.busy) CircularProgressIndicator(Modifier.height(18.dp), strokeWidth = 2.dp) else Text("保存")
                }
            }
            Spacer(Modifier.height(24.dp))
        }
    }
}
