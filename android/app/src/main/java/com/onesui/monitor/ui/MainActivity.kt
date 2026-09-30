package com.onesui.monitor.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.viewModels
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import com.onesui.monitor.service.AlertService

class MainActivity : ComponentActivity() {
    private val vm: MonitorViewModel by viewModels()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        AlertService.sync(this)
        setContent {
            val state by vm.state.collectAsStateWithLifecycle()
            MonitorTheme(state.settings) {
                Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
                    val canGoBack = state.screen != Screen.SERVERS && !(state.screen == Screen.BIND && state.panels.isEmpty())
                    BackHandler(enabled = canGoBack) { vm.back() }
                    when (state.screen) {
                        Screen.SERVERS -> ServersScreen(state, vm)
                        Screen.DETAIL -> DetailScreen(state, vm)
                        Screen.SETTINGS -> SettingsScreen(state, vm)
                        Screen.BIND -> BindScreen(state, vm)
                    }
                }
            }
        }
    }

    override fun onStart() {
        super.onStart()
        vm.setForeground(true)
    }

    override fun onStop() {
        vm.setForeground(false)
        super.onStop()
    }
}
