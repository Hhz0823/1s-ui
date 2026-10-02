package com.onesui.monitor.ui

import android.Manifest
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
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
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.Panel
import com.onesui.monitor.data.Screen
import com.onesui.monitor.data.ThemeMode
import com.onesui.monitor.data.UiState

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun SettingsScreen(state: UiState, vm: MonitorController) {
    val settings = state.settings
    var deleting by remember { mutableStateOf<Panel?>(null) }
    val notificationPermission = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { granted ->
        vm.updateSettings { it.copy(alertsEnabled = granted) }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("设置") },
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
                .padding(horizontal = 12.dp, vertical = 4.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Section("已绑定的面板") {
                state.panels.forEach { panel ->
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Column(Modifier.weight(1f)) {
                            Text(panel.name, style = MaterialTheme.typography.bodyLarge)
                            Text(
                                panel.url + if (panel.certPin.isNotEmpty()) "（已固定证书）" else "",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            val version = state.data[panel.id]?.overview?.panelVersion.orEmpty()
                            if (version.isNotBlank()) {
                                Text(
                                    "面板版本 " + versionText(version, state.update),
                                    style = MaterialTheme.typography.labelSmall,
                                    color = if (state.update.outdated(version)) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                        IconButton(onClick = { deleting = panel }) { Icon(Icons.Default.Delete, contentDescription = "解绑") }
                    }
                }
                Spacer(Modifier.height(6.dp))
                OutlinedButton(onClick = { vm.navigate(Screen.BIND) }, modifier = Modifier.fillMaxWidth()) {
                    Icon(Icons.Default.Add, contentDescription = null)
                    Text("绑定新面板")
                }
            }

            Section("刷新") {
                Text("打开 App 时每隔几秒刷新一次", style = MaterialTheme.typography.bodySmall)
                Spacer(Modifier.height(6.dp))
                ChoiceRow(listOf(1, 3, 5, 10), settings.refreshSeconds, { "$it 秒" }) { value ->
                    vm.updateSettings { it.copy(refreshSeconds = value) }
                }
            }

            Section("外观") {
                ChoiceRow(ThemeMode.entries.toList(), settings.themeMode, {
                    when (it) {
                        ThemeMode.SYSTEM -> "跟随系统"
                        ThemeMode.LIGHT -> "浅色"
                        ThemeMode.DARK -> "深色"
                    }
                }) { value -> vm.updateSettings { it.copy(themeMode = value) } }
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                    SwitchRow("使用壁纸取色", settings.dynamicColor) { value -> vm.updateSettings { it.copy(dynamicColor = value) } }
                }
            }

            Section("后台告警") {
                SwitchRow("后台监控并通知", settings.alertsEnabled) { enabled ->
                    if (enabled && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                        notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS)
                    } else {
                        vm.updateSettings { it.copy(alertsEnabled = enabled) }
                    }
                }
                Text(
                    "开启后 App 关闭时也会定时检查所有面板，服务器离线、恢复、资源过高或代理监测不可用时发通知。部分手机需要在系统设置里允许本 App 后台运行、关闭电池优化。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Spacer(Modifier.height(8.dp))
                Text("检查间隔", style = MaterialTheme.typography.labelLarge)
                ChoiceRow(listOf(30, 60, 120, 300), settings.alertIntervalSeconds, { if (it < 60) "$it 秒" else "${it / 60} 分钟" }) { value ->
                    vm.updateSettings { it.copy(alertIntervalSeconds = value) }
                }
                SwitchRow("离线 / 恢复提醒", settings.alertOffline) { value -> vm.updateSettings { it.copy(alertOffline = value) } }
                SwitchRow("代理不可用 / 恢复提醒", settings.alertProxies) { value -> vm.updateSettings { it.copy(alertProxies = value) } }
                ThresholdRow("CPU 过高", settings.cpuThreshold) { value -> vm.updateSettings { it.copy(cpuThreshold = value) } }
                ThresholdRow("内存过高", settings.memThreshold) { value -> vm.updateSettings { it.copy(memThreshold = value) } }
                ThresholdRow("磁盘快满", settings.diskThreshold) { value -> vm.updateSettings { it.copy(diskThreshold = value) } }
            }

            UpdateSection(state, vm)
            Spacer(Modifier.height(16.dp))
        }
    }

    deleting?.let { panel ->
        AlertDialog(
            onDismissRequest = { deleting = null },
            title = { Text("解绑 ${panel.name}？") },
            text = { Text("App 会删除这个面板的地址和密钥。面板上的密钥不受影响，如需作废请在面板设置里停用。") },
            confirmButton = { TextButton(onClick = { vm.deletePanel(panel.id); deleting = null }) { Text("解绑") } },
            dismissButton = { TextButton(onClick = { deleting = null }) { Text("取消") } },
        )
    }
}

@Composable
private fun ThresholdRow(label: String, value: Int, onChange: (Int) -> Unit) {
    Column(Modifier.padding(top = 6.dp)) {
        Text(label, style = MaterialTheme.typography.labelLarge)
        ChoiceRow(listOf(0, 80, 90, 95), value, { if (it == 0) "关闭" else "≥ $it%" }, onChange)
    }
}
