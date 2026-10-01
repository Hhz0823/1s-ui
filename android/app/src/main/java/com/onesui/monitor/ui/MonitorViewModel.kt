package com.onesui.monitor.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.onesui.monitor.MonitorApp
import com.onesui.monitor.data.AppSettings
import com.onesui.monitor.data.MonitorClient
import com.onesui.monitor.data.MonitorError
import com.onesui.monitor.data.Overview
import com.onesui.monitor.data.Panel
import com.onesui.monitor.data.PanelUrl
import com.onesui.monitor.data.Server
import com.onesui.monitor.service.AlertService
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.util.UUID

enum class Screen { SERVERS, DETAIL, SETTINGS, BIND }

data class UiState(
    val panels: List<Panel> = emptyList(),
    val panelId: String? = null,
    val settings: AppSettings = AppSettings(),
    val screen: Screen = Screen.SERVERS,
    val overview: Overview? = null,
    val error: String? = null,
    val loading: Boolean = false,
    val updatedAt: Long = 0,
    val detailId: Long? = null,
    val detailLocal: Boolean = false,
    val detail: Server? = null,
    val detailError: String? = null,
) {
    val panel: Panel? get() = panels.firstOrNull { it.id == panelId }
}

sealed class BindResult {
    data object Success : BindResult()
    data class NeedsTrust(val fingerprint: String) : BindResult()
    data class Failure(val message: String) : BindResult()
}

class MonitorViewModel(app: Application) : AndroidViewModel(app) {
    private val store = (app as MonitorApp).store
    private val _state = MutableStateFlow(UiState())
    val state: StateFlow<UiState> = _state.asStateFlow()
    private var poller: Job? = null
    private var foreground = false

    init {
        val panels = store.panels()
        val selected = store.selectedPanelId()?.takeIf { id -> panels.any { it.id == id } } ?: panels.firstOrNull()?.id
        _state.value = UiState(
            panels = panels,
            panelId = selected,
            settings = store.settings(),
            screen = if (panels.isEmpty()) Screen.BIND else Screen.SERVERS,
        )
    }

    fun setForeground(value: Boolean) {
        foreground = value
        if (value) restartPolling() else poller?.cancel()
    }

    fun refresh() = restartPolling()

    private fun restartPolling() {
        poller?.cancel()
        if (!foreground) return
        poller = viewModelScope.launch {
            while (isActive) {
                poll()
                delay(_state.value.settings.refreshSeconds.coerceAtLeast(1) * 1000L)
            }
        }
    }

    private suspend fun poll() {
        val current = _state.value
        val panel = current.panel ?: return
        val client = MonitorClient(panel)
        _state.update { it.copy(loading = true) }
        if (current.screen == Screen.DETAIL && current.detailId != null) {
            val id = current.detailId
            runCatching { client.server(if (current.detailLocal) 0 else id) }
                .onSuccess { server -> _state.update { if (it.detailId == id) it.copy(detail = server, detailError = null, loading = false) else it } }
                .onFailure { e -> _state.update { it.copy(detailError = message(e), loading = false) } }
            return
        }
        runCatching { client.overview() }
            .onSuccess { overview ->
                _state.update {
                    if (it.panelId != panel.id) it
                    else it.copy(overview = overview, error = null, loading = false, updatedAt = System.currentTimeMillis())
                }
            }
            .onFailure { e -> _state.update { if (it.panelId == panel.id) it.copy(error = message(e), loading = false) else it } }
    }

    fun selectPanel(id: String) {
        store.selectPanel(id)
        _state.update { it.copy(panelId = id, overview = null, error = null, screen = Screen.SERVERS, detail = null, detailId = null) }
        restartPolling()
    }

    fun openServer(server: Server) {
        _state.update {
            it.copy(screen = Screen.DETAIL, detailId = server.id, detailLocal = server.local, detail = server, detailError = null)
        }
        restartPolling()
    }

    fun navigate(screen: Screen) {
        _state.update { it.copy(screen = screen, detailId = if (screen == Screen.DETAIL) it.detailId else null) }
        restartPolling()
    }

    /** Back from any screen; returns false when the app should close. */
    fun back(): Boolean {
        val current = _state.value
        return when {
            current.screen == Screen.SERVERS -> false
            current.screen == Screen.BIND && current.panels.isEmpty() -> false
            else -> {
                navigate(Screen.SERVERS)
                true
            }
        }
    }

    suspend fun bind(name: String, rawUrl: String, key: String, certPin: String = "", editId: String? = null): BindResult {
        val url = PanelUrl.normalize(rawUrl) ?: return BindResult.Failure("面板地址格式不对，例如 http://1.2.3.4:2095/")
        if (key.isBlank()) return BindResult.Failure("请填写监控密钥")
        val panel = Panel(
            id = editId ?: UUID.randomUUID().toString(),
            name = name.trim().ifBlank { PanelUrl.displayHost(url) },
            url = url,
            key = key.trim(),
            certPin = certPin,
        )
        return try {
            MonitorClient(panel).overview()
            store.savePanel(panel)
            store.selectPanel(panel.id)
            _state.update {
                it.copy(panels = store.panels(), panelId = panel.id, screen = Screen.SERVERS, overview = null, error = null)
            }
            restartPolling()
            AlertService.sync(getApplication())
            BindResult.Success
        } catch (e: MonitorError.UntrustedCertificate) {
            BindResult.NeedsTrust(e.fingerprint)
        } catch (e: Exception) {
            BindResult.Failure(message(e))
        }
    }

    fun deletePanel(id: String) {
        store.deletePanel(id)
        val panels = store.panels()
        val selected = _state.value.panelId.takeIf { current -> panels.any { it.id == current } } ?: panels.firstOrNull()?.id
        selected?.let(store::selectPanel)
        _state.update {
            it.copy(
                panels = panels,
                panelId = selected,
                overview = if (selected == it.panelId) it.overview else null,
                screen = if (panels.isEmpty()) Screen.BIND else it.screen,
            )
        }
        restartPolling()
        AlertService.sync(getApplication())
    }

    fun updateSettings(transform: (AppSettings) -> AppSettings) {
        val next = transform(_state.value.settings)
        store.saveSettings(next)
        _state.update { it.copy(settings = next) }
        AlertService.sync(getApplication())
    }

    private fun message(e: Throwable): String = when (e) {
        is MonitorError -> e.message.orEmpty()
        is java.net.UnknownHostException -> "找不到这个地址，请检查面板地址"
        is java.net.SocketTimeoutException -> "连接超时，请检查网络或面板端口"
        is java.net.ConnectException -> "连接被拒绝，请检查面板地址和端口"
        is javax.net.ssl.SSLException -> "HTTPS 连接失败：${e.message.orEmpty()}"
        else -> e.message ?: e.javaClass.simpleName
    }
}
