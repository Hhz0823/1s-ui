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
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.NodeItem
import com.onesui.monitor.data.NodePing
import com.onesui.monitor.data.UiState

/** The server's inbounds: protocol, port, security, users and traffic, never their secrets. */
@Composable
fun NodesTab(state: UiState, vm: MonitorController) {
    val nodes = state.nodes
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        state.nodesError?.let {
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
        if (nodes == null) {
            if (state.nodesError == null) {
                Box(Modifier.fillMaxWidth().padding(48.dp), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            }
            return@Column
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text("${nodes.size} 个节点", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                Text(
                    "↑ ${Format.rate(nodes.sumOf { it.uploadBps })}  ↓ ${Format.rate(nodes.sumOf { it.downloadBps })}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            OutlinedButton(onClick = vm::pingNodes, enabled = !state.pingingNodes && nodes.isNotEmpty()) {
                Text(if (state.pingingNodes) "测试中…" else "测延迟")
            }
        }
        val address = vm.serverAddress(state)
        if (address != null && state.nodePings.isNotEmpty()) {
            Text(
                "延迟是手机到 $address 各端口的 TCP 连接时间；Hysteria2、TUIC 等 UDP 协议不适用。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        if (nodes.isEmpty()) {
            Text("这台服务器还没有节点。", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        nodes.forEach { node -> NodeCard(node, state.nodePings[node.id]) }
        Spacer(Modifier.height(16.dp))
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun NodeCard(node: NodeItem, ping: NodePing?) {
    Card(
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        elevation = CardDefaults.cardElevation(defaultElevation = 1.dp),
        modifier = Modifier.fillMaxWidth(),
    ) {
        Column(Modifier.padding(14.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                StatusDot(node.online, size = 8.dp)
                Spacer(Modifier.width(8.dp))
                Text(
                    node.tag.ifBlank { Format.protocol(node.type) },
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                Text("端口 ${node.port}", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
            Spacer(Modifier.height(6.dp))
            FlowRow(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                Tag(Format.protocol(node.type))
                when (node.security) {
                    "reality" -> Tag("REALITY")
                    "tls" -> Tag("TLS")
                }
                if (node.transport.isNotBlank()) Tag(node.transport.uppercase())
                if (node.encryption) Tag("ENC")
                if (node.coreType == "xray") Tag("Xray")
                if (node.cdn.isNotBlank()) Tag("CDN ${node.cdn}")
                if (node.exhausted) Tag("流量用完")
            }
            Spacer(Modifier.height(6.dp))
            Row {
                LabelValue("↑ 实时", Format.rate(node.uploadBps), Modifier.weight(1f))
                LabelValue("↓ 实时", Format.rate(node.downloadBps), Modifier.weight(1f))
                LabelValue("用户", node.users.toString(), Modifier.weight(1f))
            }
            Row {
                LabelValue("↑ 总计", Format.bytes(node.uploadBytes), Modifier.weight(1f))
                LabelValue("↓ 总计", Format.bytes(node.downloadBytes), Modifier.weight(1f))
                LabelValue(
                    "延迟",
                    when {
                        node.udpOnly -> "UDP"
                        ping == null -> "-"
                        else -> ping.ms?.let { Format.ms(it) } ?: ping.error ?: "失败"
                    },
                    Modifier.weight(1f),
                )
            }
            if (node.trafficLimit > 0) {
                Spacer(Modifier.height(4.dp))
                UsageBar(
                    "本月流量",
                    node.trafficUsed * 100.0 / node.trafficLimit,
                    "${Format.bytes(node.trafficUsed)} / ${Format.bytes(node.trafficLimit)}",
                )
            }
            if (node.ipLimit > 0 || node.activeIps > 0) {
                Text(
                    "在线 IP ${node.activeIps}" + if (node.ipLimit > 0) " / 限 ${node.ipLimit}" else "",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 4.dp),
                )
            }
        }
    }
}
