package com.onesui.monitor.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.Format
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.PingStats
import com.onesui.monitor.data.SpeedPhase
import com.onesui.monitor.data.SpeedSuite
import com.onesui.monitor.data.UdpStats
import com.onesui.monitor.data.UiState

/** Phone-to-server latency and TCP / UDP throughput. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SpeedTab(state: UiState, vm: MonitorController) {
    val speed = state.speed
    val settings = state.settings
    val port = state.features(state.detailPanelId).speedtestPort
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Section("测速") {
            Text(
                "测手机到这台服务器的延迟和 TCP、UDP 速度。服务器只在测速时临时打开端口 $port（TCP 和 UDP），测完自动关闭；连不上时请在防火墙和安全组放行这个端口。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(8.dp))
            Text("每项时长", style = MaterialTheme.typography.labelLarge)
            ChoiceRow(listOf(5, 10, 15), settings.speedSeconds, { "$it 秒" }) { value -> vm.updateSettings { it.copy(speedSeconds = value) } }
            Text("TCP 并发连接", style = MaterialTheme.typography.labelLarge)
            ChoiceRow(listOf(1, 4, 8), settings.speedStreams, { "$it 条" }) { value -> vm.updateSettings { it.copy(speedStreams = value) } }
            Text("UDP 发送速率", style = MaterialTheme.typography.labelLarge)
            ChoiceRow(listOf(10, 30, 50, 100, 200, 500), settings.udpMbps, { "$it Mbps" }) { value -> vm.updateSettings { it.copy(udpMbps = value) } }
            Text(
                "UDP 按这个速率收发，速率超过线路能力时丢包会变多；丢包多说明运营商对 UDP 限速或干扰（影响 Hysteria2、TUIC 等）。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(10.dp))
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                SpeedSuite.entries.forEach { suite ->
                    Button(onClick = { vm.startSpeedTest(suite) }, enabled = !speed.running) { Text(suite.label) }
                }
                if (speed.running) {
                    OutlinedButton(onClick = vm::stopSpeedTest) { Text("停止") }
                }
            }
        }

        speed.progress?.let { progress ->
            Section("正在测试：${progress.phase.label}") {
                LinearProgressIndicator(progress = { progress.fraction.coerceIn(0f, 1f) }, modifier = Modifier.fillMaxWidth())
                Spacer(Modifier.height(8.dp))
                Text(
                    progress.latencyMs?.let { Format.ms(it) } ?: Format.bits(progress.bitsPerSecond),
                    style = MaterialTheme.typography.headlineMedium,
                    fontWeight = FontWeight.Bold,
                )
            }
        }
        speed.error?.let { Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium) }
        if (speed.host.isNotBlank()) {
            Text(
                "测速地址 ${speed.host}:${speed.port}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        if (speed.tcpPing != null || speed.udpPing != null || speed.errors.containsKey(SpeedPhase.TCP_PING) || speed.errors.containsKey(SpeedPhase.UDP_PING)) {
            Section("延迟") {
                Row {
                    PingValue("TCP", speed.tcpPing, Modifier.weight(1f))
                    PingValue("UDP", speed.udpPing, Modifier.weight(1f))
                }
                PhaseErrors(speed.errors, listOf(SpeedPhase.TCP_PING, SpeedPhase.UDP_PING))
            }
        }
        if (speed.tcpDown != null || speed.tcpUp != null || speed.errors.containsKey(SpeedPhase.TCP_DOWNLOAD) || speed.errors.containsKey(SpeedPhase.TCP_UPLOAD)) {
            Section("TCP") {
                Row {
                    BigValue("下载", speed.tcpDown?.let { Format.bits(it.bitsPerSecond) }, Modifier.weight(1f))
                    BigValue("上传", speed.tcpUp?.let { Format.bits(it.bitsPerSecond) }, Modifier.weight(1f))
                }
                val streams = speed.tcpDown?.streams ?: speed.tcpUp?.streams
                if (streams != null) {
                    Text(
                        "$streams 条并发连接 · 下载 ${Format.bytes(speed.tcpDown?.bytes ?: 0)} · 上传 ${Format.bytes(speed.tcpUp?.bytes ?: 0)}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                PhaseErrors(speed.errors, listOf(SpeedPhase.TCP_DOWNLOAD, SpeedPhase.TCP_UPLOAD))
            }
        }
        if (speed.udpDown != null || speed.udpUp != null || speed.errors.containsKey(SpeedPhase.UDP_DOWNLOAD) || speed.errors.containsKey(SpeedPhase.UDP_UPLOAD)) {
            Section("UDP") {
                Row {
                    BigValue("下载", speed.udpDown?.let { Format.bits(it.bitsPerSecond) }, Modifier.weight(1f))
                    BigValue("上传", speed.udpUp?.let { Format.bits(it.bitsPerSecond) }, Modifier.weight(1f))
                }
                Row {
                    LabelValue("下载丢包", speed.udpDown?.let(::loss) ?: "-", Modifier.weight(1f))
                    LabelValue("上传丢包", speed.udpUp?.let(::loss) ?: "-", Modifier.weight(1f))
                }
                Row {
                    LabelValue("下载抖动", speed.udpDown?.let { Format.ms(it.jitterMs) } ?: "-", Modifier.weight(1f))
                    LabelValue("上传抖动", speed.udpUp?.let { Format.ms(it.jitterMs) } ?: "-", Modifier.weight(1f))
                }
                val target = speed.udpDown?.targetMbps ?: speed.udpUp?.targetMbps
                if (target != null) {
                    Text("发送速率 $target Mbps", style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                PhaseErrors(speed.errors, listOf(SpeedPhase.UDP_DOWNLOAD, SpeedPhase.UDP_UPLOAD))
            }
        }
        Spacer(Modifier.height(16.dp))
    }
}

private fun loss(stats: UdpStats): String =
    String.format(java.util.Locale.US, "%.1f%%", stats.lossPct) + " (${stats.receivedPackets}/${stats.sentPackets})"

@Composable
private fun PingValue(label: String, stats: PingStats?, modifier: Modifier) {
    Column(modifier) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (stats == null) {
            Text("-", style = MaterialTheme.typography.titleLarge)
            return@Column
        }
        if (stats.received == 0) {
            Text("全部超时", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.error)
            return@Column
        }
        Text(Format.ms(stats.avgMs), style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        Text(
            "最低 ${Format.ms(stats.minMs)} · 最高 ${Format.ms(stats.maxMs)}\n抖动 ${Format.ms(stats.jitterMs)} · 丢包 ${String.format(java.util.Locale.US, "%.0f%%", stats.lossPct)}",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun BigValue(label: String, value: String?, modifier: Modifier) {
    Column(modifier, horizontalAlignment = Alignment.Start) {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value ?: "-", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun PhaseErrors(errors: Map<SpeedPhase, String>, phases: List<SpeedPhase>) {
    phases.forEach { phase ->
        errors[phase]?.let {
            Text("${phase.label}：$it", color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
        }
    }
}
