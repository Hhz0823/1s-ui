package com.onesui.monitor.ui

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.UriHandler
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.UiState
import com.onesui.monitor.data.UpdateState

/** Opens [url] in the browser; a phone without one keeps the app running. */
private fun UriHandler.openSafely(url: String) {
    runCatching { openUri(url) }
}

/** A panel or server version, with the release it can update to. */
fun versionText(version: String, update: UpdateState): String {
    if (version.isBlank()) return "-"
    val latest = update.latest
    return if (latest != null && update.outdated(version)) "$version（可更新到 ${latest.version}）" else version
}

/** The home page banner while a newer app is out. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun UpdateBanner(state: UiState, vm: MonitorController) {
    val update = state.update
    val latest = update.latest ?: return
    val uri = LocalUriHandler.current
    Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.tertiaryContainer)) {
        Column(Modifier.padding(horizontal = 14.dp, vertical = 10.dp).fillMaxWidth()) {
            Text("App 有新版本 ${latest.version}", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Text(
                "当前 ${update.appVersion}。新版和 1S-UI ${latest.tag} 一起发布，覆盖安装后绑定的面板不会丢失。",
                style = MaterialTheme.typography.bodySmall,
            )
            FlowRow(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                TextButton(onClick = { uri.openSafely(latest.apkUrl) }) { Text("下载") }
                TextButton(onClick = { uri.openSafely(latest.mirrorApkUrl) }) { Text("加速下载") }
                TextButton(onClick = vm::dismissUpdate) { Text("忽略") }
            }
        }
    }
}

/** Settings: this app's version, the latest release and where to get it. */
@OptIn(ExperimentalLayoutApi::class)
@Composable
fun UpdateSection(state: UiState, vm: MonitorController) {
    val update = state.update
    val latest = update.latest
    val uri = LocalUriHandler.current
    Section("关于与更新") {
        Row {
            LabelValue("App 版本", update.appVersion.ifBlank { "-" }, Modifier.weight(1f))
            LabelValue("最新版本", latest?.version ?: if (update.checking) "检查中…" else "-", Modifier.weight(1f))
        }
        when {
            update.checking -> LinearProgressIndicator(Modifier.fillMaxWidth().padding(vertical = 4.dp))
            update.error != null -> Text(update.error, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.error)
            latest != null && update.appUpdate -> Text("有新版本，下载后覆盖安装，绑定的面板和设置会保留。", style = MaterialTheme.typography.bodySmall)
            latest != null && !latest.hasApk -> Text("最新版本没有附带 App，当前版本可以继续使用。", style = MaterialTheme.typography.bodySmall)
            latest != null -> Text("已是最新版本。", style = MaterialTheme.typography.bodySmall)
        }
        Spacer(Modifier.height(6.dp))
        FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            OutlinedButton(onClick = vm::checkUpdates, enabled = !update.checking) { Text("检查更新") }
            if (latest != null && update.appUpdate) {
                Button(onClick = { uri.openSafely(latest.apkUrl) }) { Text("下载新版") }
                OutlinedButton(onClick = { uri.openSafely(latest.mirrorApkUrl) }) { Text("加速下载") }
            }
            if (latest != null) TextButton(onClick = { uri.openSafely(latest.pageUrl) }) { Text("更新说明") }
        }
        Text(
            "GitHub 下载慢或打不开时用「加速下载」。安装时提示签名不一致，需要先卸载旧版。面板可在面板「设置 → 服务端面板」里检测更新。",
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}
