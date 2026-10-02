package com.onesui.monitor.ui

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Home
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import com.onesui.monitor.data.ALL_PANELS
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.data.Screen
import com.onesui.monitor.data.UiState

/** The home tabs: servers of every panel, client devices, and proxy and node monitors. */
@Composable
fun HomeBar(state: UiState, vm: MonitorController) {
    NavigationBar {
        NavigationBarItem(
            selected = state.screen == Screen.SERVERS,
            onClick = { vm.showHome(Screen.SERVERS) },
            icon = { Icon(Icons.Default.Home, contentDescription = null) },
            label = { Text("服务器") },
        )
        NavigationBarItem(
            selected = state.screen == Screen.CLIENTS,
            onClick = { vm.showHome(Screen.CLIENTS) },
            icon = { Icon(Icons.AutoMirrored.Filled.Send, contentDescription = null) },
            label = { Text("客户端") },
        )
        NavigationBarItem(
            selected = state.screen == Screen.PROXIES,
            onClick = { vm.showHome(Screen.PROXIES) },
            icon = { Icon(Icons.Default.CheckCircle, contentDescription = null) },
            label = { Text("监测") },
        )
    }
}

/** Title that switches between one panel and all panels. */
@Composable
fun PanelSwitcher(state: UiState, vm: MonitorController, fallback: String) {
    var open by remember { mutableStateOf(false) }
    Box {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.clickable { open = true },
        ) {
            val title = when {
                state.allPanels -> "全部面板（${state.panels.size}）"
                else -> state.panel?.name ?: fallback
            }
            Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Icon(Icons.Default.ArrowDropDown, contentDescription = "切换面板")
        }
        DropdownMenu(expanded = open, onDismissRequest = { open = false }) {
            if (state.panels.size > 1) {
                DropdownMenuItem(
                    text = { Text("全部面板") },
                    leadingIcon = { if (state.allPanels) Icon(Icons.Default.Check, contentDescription = null) },
                    onClick = { open = false; vm.selectPanel(ALL_PANELS) },
                )
                HorizontalDivider()
            }
            state.panels.forEach { panel ->
                DropdownMenuItem(
                    text = { Text(panel.name) },
                    leadingIcon = { if (panel.id == state.panelId) Icon(Icons.Default.Check, contentDescription = null) },
                    onClick = { open = false; vm.selectPanel(panel.id) },
                )
            }
            HorizontalDivider()
            DropdownMenuItem(
                text = { Text("绑定新面板…", color = MaterialTheme.colorScheme.primary) },
                onClick = { open = false; vm.navigate(Screen.BIND) },
            )
        }
    }
}
