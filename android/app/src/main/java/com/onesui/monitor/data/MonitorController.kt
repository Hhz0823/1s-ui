package com.onesui.monitor.data

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.net.InetSocketAddress
import java.net.Socket
import java.util.UUID
import kotlin.coroutines.cancellation.CancellationException

/** Selection meaning every bound panel at once. */
const val ALL_PANELS = "*"

enum class Screen { SERVERS, PROXIES, DETAIL, PROXY_DETAIL, PROXY_EDIT, SETTINGS, BIND }

enum class DetailTab { OVERVIEW, NODES, SPEED }

/** What was last loaded from one panel. */
data class PanelData(
    val overview: Overview? = null,
    val error: String? = null,
    val updatedAt: Long = 0,
    val proxies: List<ProxyMonitor>? = null,
    val proxiesError: String? = null,
)

data class PanelServer(val panel: Panel, val server: Server) {
    val key: String get() = "${panel.id}:${if (server.local) "local" else server.id}"
}

data class PanelProxy(val panel: Panel, val monitor: ProxyMonitor)

/** A TCP ping from the phone to one node's port. */
data class NodePing(val ms: Double?, val error: String?)

data class SpeedState(
    val serverKey: String = "",
    val running: Boolean = false,
    val progress: SpeedProgress? = null,
    val host: String = "",
    val port: Int = 0,
    val tcpPing: PingStats? = null,
    val udpPing: PingStats? = null,
    val tcpDown: ThroughputStats? = null,
    val tcpUp: ThroughputStats? = null,
    val udpDown: UdpStats? = null,
    val udpUp: UdpStats? = null,
    val errors: Map<SpeedPhase, String> = emptyMap(),
    val error: String? = null,
)

enum class SpeedSuite(val label: String, val phases: List<SpeedPhase>) {
    LATENCY("延迟", listOf(SpeedPhase.TCP_PING, SpeedPhase.UDP_PING)),
    TCP("TCP 测速", listOf(SpeedPhase.TCP_PING, SpeedPhase.TCP_DOWNLOAD, SpeedPhase.TCP_UPLOAD)),
    UDP("UDP 测速", listOf(SpeedPhase.UDP_PING, SpeedPhase.UDP_DOWNLOAD, SpeedPhase.UDP_UPLOAD)),
    ALL("全部", SpeedPhase.entries.toList()),
}

data class ProxyEditState(
    val panelId: String,
    val input: ProxyInput,
    val hadPassword: Boolean = false,
    val busy: Boolean = false,
    val testResult: ProbeResult? = null,
    val error: String? = null,
    val returnTo: Screen = Screen.PROXIES,
)

data class UiState(
    val panels: List<Panel> = emptyList(),
    /** A panel id, or [ALL_PANELS]. */
    val panelId: String? = null,
    val settings: AppSettings = AppSettings(),
    val screen: Screen = Screen.SERVERS,
    /** The home tab the detail screens return to. */
    val home: Screen = Screen.SERVERS,
    val data: Map<String, PanelData> = emptyMap(),
    val loading: Boolean = false,
    val detailPanelId: String? = null,
    val detailId: Long? = null,
    val detailLocal: Boolean = false,
    val detail: Server? = null,
    val detailError: String? = null,
    val detailTab: DetailTab = DetailTab.OVERVIEW,
    val nodes: List<NodeItem>? = null,
    val nodesError: String? = null,
    val nodePings: Map<Long, NodePing> = emptyMap(),
    val pingingNodes: Boolean = false,
    val speed: SpeedState = SpeedState(),
    val proxyPanelId: String? = null,
    val proxyId: Long? = null,
    val proxyRange: Long = 86_400,
    val proxyDetail: ProxyDetail? = null,
    val proxyError: String? = null,
    val proxyAction: String? = null,
    val editing: ProxyEditState? = null,
) {
    val allPanels: Boolean get() = panelId == ALL_PANELS
    val panel: Panel? get() = panels.firstOrNull { it.id == panelId }
    val scopedPanels: List<Panel> get() = if (allPanels) panels else panels.filter { it.id == panelId }
    val detailPanel: Panel? get() = panels.firstOrNull { it.id == detailPanelId }
    val proxyPanel: Panel? get() = panels.firstOrNull { it.id == proxyPanelId }
    val detailKey: String get() = "$detailPanelId:${if (detailLocal) "local" else detailId}"

    val servers: List<PanelServer>
        get() = scopedPanels.flatMap { panel -> data[panel.id]?.overview?.servers.orEmpty().map { PanelServer(panel, it) } }

    val proxies: List<PanelProxy>
        get() = scopedPanels.flatMap { panel -> data[panel.id]?.proxies.orEmpty().map { PanelProxy(panel, it) } }

    fun features(panelId: String?): Features = data[panelId]?.overview?.features ?: Features()

    /** Panels in scope that could not be reached, with the reason. */
    val panelErrors: List<Pair<Panel, String>>
        get() = scopedPanels.mapNotNull { panel -> data[panel.id]?.error?.let { panel to it } }
}

sealed class BindResult {
    data object Success : BindResult()
    data class NeedsTrust(val fingerprint: String) : BindResult()
    data class Failure(val message: String) : BindResult()
}

/**
 * Everything the screens show and do, without Android: the view model only
 * hosts it. Panels are polled while the app is in the foreground, every
 * panel at once when [ALL_PANELS] is selected.
 */
class MonitorController(
    private val store: AppStore,
    private val scope: CoroutineScope,
    private val clientFor: (Panel) -> MonitorApi = ::MonitorClient,
    private val onPanelsChanged: () -> Unit = {},
) {
    private val _state = MutableStateFlow(UiState())
    val state: StateFlow<UiState> = _state.asStateFlow()
    private var poller: Job? = null
    private var speedJob: Job? = null
    private var foreground = false

    init {
        val panels = store.panels()
        val stored = store.selectedPanelId()
        val selected = when {
            stored == ALL_PANELS && panels.size > 1 -> ALL_PANELS
            stored != null && panels.any { it.id == stored } -> stored
            panels.size > 1 -> ALL_PANELS
            else -> panels.firstOrNull()?.id
        }
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
        poller = scope.launch {
            while (isActive) {
                poll()
                val seconds = when (_state.value.screen) {
                    Screen.PROXIES, Screen.PROXY_DETAIL -> maxOf(_state.value.settings.refreshSeconds, 10)
                    Screen.PROXY_EDIT -> 60
                    else -> _state.value.settings.refreshSeconds.coerceAtLeast(1)
                }
                delay(seconds * 1000L)
            }
        }
    }

    suspend fun poll() {
        val current = _state.value
        _state.update { it.copy(loading = true) }
        try {
            when (current.screen) {
                Screen.DETAIL -> pollDetail(current)
                Screen.PROXY_DETAIL -> pollProxyDetail(current)
                Screen.PROXIES, Screen.PROXY_EDIT -> {
                    pollOverviews(current.scopedPanels.filter { current.data[it.id]?.overview == null })
                    pollProxies(current.scopedPanels)
                }
                else -> pollOverviews(current.scopedPanels)
            }
        } finally {
            _state.update { it.copy(loading = false) }
        }
    }

    private suspend fun pollOverviews(panels: List<Panel>) = coroutineScope {
        panels.map { panel ->
            async {
                val result = runCatching { clientFor(panel).overview() }
                _state.update { state ->
                    val before = state.data[panel.id] ?: PanelData()
                    val after = result.fold(
                        { before.copy(overview = it, error = null, updatedAt = System.currentTimeMillis()) },
                        { before.copy(error = message(it)) },
                    )
                    state.copy(data = state.data + (panel.id to after))
                }
            }
        }.awaitAll()
    }

    private suspend fun pollProxies(panels: List<Panel>) = coroutineScope {
        panels.map { panel ->
            async {
                val result = runCatching { clientFor(panel).proxies() }
                _state.update { state ->
                    val before = state.data[panel.id] ?: PanelData()
                    val after = result.fold(
                        { before.copy(proxies = it, proxiesError = null) },
                        { before.copy(proxiesError = message(it)) },
                    )
                    state.copy(data = state.data + (panel.id to after))
                }
            }
        }.awaitAll()
    }

    private suspend fun pollDetail(current: UiState) {
        val panel = current.detailPanel ?: return
        val id = current.detailId ?: return
        val key = current.detailKey
        val client = clientFor(panel)
        val serverId = if (current.detailLocal) 0 else id
        runCatching { client.server(serverId) }
            .onSuccess { server -> _state.update { if (it.detailKey == key) it.copy(detail = server, detailError = null) else it } }
            .onFailure { e -> _state.update { if (it.detailKey == key) it.copy(detailError = message(e)) else it } }
        if (current.detailTab == DetailTab.NODES) loadNodes(client, serverId, key)
    }

    private suspend fun loadNodes(client: MonitorApi, serverId: Long, key: String) {
        runCatching { client.nodes(serverId) }
            .onSuccess { nodes -> _state.update { if (it.detailKey == key) it.copy(nodes = nodes, nodesError = null) else it } }
            .onFailure { e -> _state.update { if (it.detailKey == key) it.copy(nodesError = message(e)) else it } }
    }

    private suspend fun pollProxyDetail(current: UiState) {
        val panel = current.proxyPanel ?: return
        val id = current.proxyId ?: return
        runCatching { clientFor(panel).proxy(id, current.proxyRange) }
            .onSuccess { detail -> _state.update { if (it.proxyId == id) it.copy(proxyDetail = detail, proxyError = null) else it } }
            .onFailure { e -> _state.update { if (it.proxyId == id) it.copy(proxyError = message(e)) else it } }
    }

    fun selectPanel(id: String) {
        store.selectPanel(id)
        _state.update { it.copy(panelId = id, screen = it.home, detailId = null, detail = null) }
        restartPolling()
    }

    fun showHome(screen: Screen) {
        _state.update { it.copy(screen = screen, home = screen) }
        restartPolling()
    }

    fun openServer(item: PanelServer) {
        val key = "${item.panel.id}:${if (item.server.local) "local" else item.server.id}"
        _state.update {
            it.copy(
                screen = Screen.DETAIL,
                detailPanelId = item.panel.id,
                detailId = item.server.id,
                detailLocal = item.server.local,
                detail = item.server,
                detailError = null,
                detailTab = DetailTab.OVERVIEW,
                nodes = null,
                nodesError = null,
                nodePings = emptyMap(),
                speed = if (it.speed.serverKey == key) it.speed else SpeedState(serverKey = key),
            )
        }
        restartPolling()
    }

    fun selectDetailTab(tab: DetailTab) {
        _state.update { it.copy(detailTab = tab) }
        if (tab == DetailTab.NODES) restartPolling()
    }

    fun navigate(screen: Screen) {
        _state.update { it.copy(screen = screen) }
        restartPolling()
    }

    /** Back from any screen; returns false when the app should close. */
    fun back(): Boolean {
        val current = _state.value
        val target = when (current.screen) {
            Screen.SERVERS, Screen.PROXIES -> return false
            Screen.BIND -> if (current.panels.isEmpty()) return false else current.home
            Screen.PROXY_EDIT -> current.editing?.returnTo ?: Screen.PROXIES
            Screen.PROXY_DETAIL -> Screen.PROXIES
            else -> current.home
        }
        _state.update { it.copy(screen = target, editing = if (target == Screen.PROXY_EDIT) it.editing else null) }
        restartPolling()
        return true
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
            val overview = clientFor(panel).overview()
            store.savePanel(panel)
            val panels = store.panels()
            // A second panel switches the home page to every panel at once.
            val selected = if (panels.size > 1 && _state.value.panelId != null) ALL_PANELS else panel.id
            store.selectPanel(selected)
            _state.update {
                it.copy(
                    panels = panels,
                    panelId = selected,
                    screen = Screen.SERVERS,
                    home = Screen.SERVERS,
                    data = it.data + (panel.id to PanelData(overview = overview, updatedAt = System.currentTimeMillis())),
                )
            }
            restartPolling()
            onPanelsChanged()
            BindResult.Success
        } catch (e: MonitorError.UntrustedCertificate) {
            BindResult.NeedsTrust(e.fingerprint)
        } catch (e: CancellationException) {
            throw e
        } catch (e: Exception) {
            BindResult.Failure(message(e))
        }
    }

    fun deletePanel(id: String) {
        store.deletePanel(id)
        val panels = store.panels()
        val current = _state.value.panelId
        val selected = when {
            panels.isEmpty() -> null
            current == ALL_PANELS && panels.size > 1 -> ALL_PANELS
            panels.any { it.id == current } -> current
            else -> panels.first().id
        }
        selected?.let(store::selectPanel)
        _state.update {
            it.copy(
                panels = panels,
                panelId = selected,
                data = it.data - id,
                screen = if (panels.isEmpty()) Screen.BIND else it.screen,
            )
        }
        restartPolling()
        onPanelsChanged()
    }

    fun updateSettings(transform: (AppSettings) -> AppSettings) {
        val next = transform(_state.value.settings)
        store.saveSettings(next)
        _state.update { it.copy(settings = next) }
        onPanelsChanged()
    }

    // Nodes

    /** TCP-pings every TCP node of the open server from the phone, like a client's "tcping". */
    fun pingNodes() {
        val current = _state.value
        val nodes = current.nodes ?: return
        val host = serverAddress(current) ?: run {
            _state.update { it.copy(nodesError = "不知道这台服务器的公网地址，请在面板里给它填写公网地址") }
            return
        }
        val key = current.detailKey
        _state.update { it.copy(pingingNodes = true, nodePings = emptyMap()) }
        scope.launch {
            coroutineScope {
                nodes.filter { !it.udpOnly && it.port > 0 }.map { node ->
                    async {
                        val ping = tcpPing(host, node.port)
                        _state.update { if (it.detailKey == key) it.copy(nodePings = it.nodePings + (node.id to ping)) else it }
                    }
                }.awaitAll()
            }
            _state.update { if (it.detailKey == key) it.copy(pingingNodes = false) else it }
        }
    }

    /** The address the phone uses for the open server: the panel's host for the panel itself. */
    fun serverAddress(current: UiState = _state.value): String? {
        val server = current.detail ?: return null
        if (current.detailLocal) return current.detailPanel?.let { PanelUrl.displayHost(it.url) }
        return listOf(server.publicHost, server.remoteIp).firstOrNull { it.isNotBlank() }?.trim('[', ']')
    }

    // Speed tests

    fun startSpeedTest(suite: SpeedSuite) {
        val current = _state.value
        val panel = current.detailPanel ?: return
        val serverId = if (current.detailLocal) 0L else current.detailId ?: return
        val key = current.detailKey
        val settings = current.settings
        speedJob?.cancel()
        _state.update { state ->
            val cleared = suite.phases.fold(state.speed) { speed, phase -> clearPhase(speed, phase) }
            state.copy(speed = cleared.copy(serverKey = key, running = true, progress = null, error = null))
        }
        speedJob = scope.launch {
            try {
                val target = clientFor(panel).startSpeedtest(serverId)
                val preferred = if (current.detailLocal) listOfNotNull(serverAddress(current)) else emptyList()
                val tester = SpeedTester.connect(target, preferred)
                updateSpeed(key) { it.copy(host = tester.host, port = tester.port) }
                for (phase in suite.phases) {
                    val progress: (SpeedProgress) -> Unit = { p -> updateSpeed(key) { it.copy(progress = p) } }
                    updateSpeed(key) { it.copy(progress = SpeedProgress(phase, 0f)) }
                    try {
                        when (phase) {
                            SpeedPhase.TCP_PING -> tester.tcpPing(10, progress).let { r -> updateSpeed(key) { it.copy(tcpPing = r) } }
                            SpeedPhase.UDP_PING -> tester.udpPing(20, progress).let { r -> updateSpeed(key) { it.copy(udpPing = r) } }
                            SpeedPhase.TCP_DOWNLOAD -> tester.tcpDownload(settings.speedSeconds, settings.speedStreams, progress)
                                .let { r -> updateSpeed(key) { it.copy(tcpDown = r) } }
                            SpeedPhase.TCP_UPLOAD -> tester.tcpUpload(settings.speedSeconds, settings.speedStreams, progress)
                                .let { r -> updateSpeed(key) { it.copy(tcpUp = r) } }
                            SpeedPhase.UDP_DOWNLOAD -> tester.udpDownload(settings.speedSeconds, settings.udpMbps, progress)
                                .let { r -> updateSpeed(key) { it.copy(udpDown = r) } }
                            SpeedPhase.UDP_UPLOAD -> tester.udpUpload(settings.speedSeconds, settings.udpMbps, progress)
                                .let { r -> updateSpeed(key) { it.copy(udpUp = r) } }
                        }
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        // One blocked direction (often UDP) should not hide the others.
                        updateSpeed(key) { it.copy(errors = it.errors + (phase to message(e))) }
                    }
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                updateSpeed(key) { it.copy(error = message(e)) }
            } finally {
                updateSpeed(key) { it.copy(running = false, progress = null) }
            }
        }
    }

    fun stopSpeedTest() {
        speedJob?.cancel()
    }

    private fun clearPhase(speed: SpeedState, phase: SpeedPhase): SpeedState = when (phase) {
        SpeedPhase.TCP_PING -> speed.copy(tcpPing = null)
        SpeedPhase.UDP_PING -> speed.copy(udpPing = null)
        SpeedPhase.TCP_DOWNLOAD -> speed.copy(tcpDown = null)
        SpeedPhase.TCP_UPLOAD -> speed.copy(tcpUp = null)
        SpeedPhase.UDP_DOWNLOAD -> speed.copy(udpDown = null)
        SpeedPhase.UDP_UPLOAD -> speed.copy(udpUp = null)
    }.let { it.copy(errors = it.errors - phase) }

    private fun updateSpeed(key: String, transform: (SpeedState) -> SpeedState) {
        _state.update { if (it.speed.serverKey == key) it.copy(speed = transform(it.speed)) else it }
    }

    // Proxy monitors

    fun openProxy(item: PanelProxy) {
        _state.update {
            it.copy(
                screen = Screen.PROXY_DETAIL,
                proxyPanelId = item.panel.id,
                proxyId = item.monitor.id,
                proxyDetail = null,
                proxyError = null,
                proxyAction = null,
            )
        }
        restartPolling()
    }

    fun setProxyRange(seconds: Long) {
        _state.update { it.copy(proxyRange = seconds, proxyDetail = null) }
        restartPolling()
    }

    /** Opens the editor for a new monitor on [panelId], or for [monitor]. */
    fun editProxy(panelId: String, monitor: ProxyMonitor? = null) {
        val input = monitor?.let {
            ProxyInput(
                id = it.id, name = it.name, type = it.type, host = it.host, port = it.port, username = it.username,
                password = null, target = it.target, serverId = it.serverId, interval = it.interval, enabled = it.enabled,
            )
        } ?: ProxyInput()
        _state.update {
            it.copy(
                screen = Screen.PROXY_EDIT,
                editing = ProxyEditState(panelId, input, hadPassword = monitor?.hasPassword == true, returnTo = it.screen),
            )
        }
        val panel = _state.value.panels.firstOrNull { it.id == panelId } ?: return
        if (_state.value.data[panelId]?.overview == null) scope.launch { pollOverviews(listOf(panel)) }
    }

    fun updateEditing(transform: (ProxyInput) -> ProxyInput) {
        _state.update { state -> state.copy(editing = state.editing?.let { it.copy(input = transform(it.input), error = null) }) }
    }

    fun testEditing() = editingCall { client, input ->
        val result = client.testProxy(input)
        _state.update { state -> state.copy(editing = state.editing?.copy(testResult = result, busy = false)) }
    }

    fun saveEditing() = editingCall { client, input ->
        val saved = client.saveProxy(input)
        val editing = _state.value.editing
        _state.update { state ->
            val panelId = editing?.panelId
            val before = state.data[panelId] ?: PanelData()
            val list = before.proxies.orEmpty().filter { it.id != saved.id } + saved
            state.copy(
                editing = null,
                data = if (panelId != null) state.data + (panelId to before.copy(proxies = list.sortedBy { it.id })) else state.data,
                screen = if (editing?.returnTo == Screen.PROXY_DETAIL) Screen.PROXY_DETAIL else Screen.PROXIES,
            )
        }
        restartPolling()
    }

    private fun editingCall(block: suspend (MonitorApi, ProxyInput) -> Unit) {
        val editing = _state.value.editing ?: return
        val panel = _state.value.panels.firstOrNull { it.id == editing.panelId } ?: return
        _state.update { it.copy(editing = editing.copy(busy = true, error = null, testResult = null)) }
        scope.launch {
            try {
                block(clientFor(panel), editing.input)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { state -> state.copy(editing = state.editing?.copy(busy = false, error = message(e))) }
            }
        }
    }

    fun checkProxyNow() = proxyCall("检测完成") { client, id ->
        val result = client.checkProxy(id)
        if (!result.ok) Messages.probe(result) else "可用 · ${result.latencyMs} ms"
    }

    fun setProxyEnabled(enabled: Boolean) {
        val detail = _state.value.proxyDetail?.monitor ?: return
        proxyCall(if (enabled) "已恢复检测" else "已暂停检测") { client, _ ->
            client.saveProxy(
                ProxyInput(
                    id = detail.id, name = detail.name, type = detail.type, host = detail.host, port = detail.port,
                    username = detail.username, password = null, target = detail.target, serverId = detail.serverId,
                    interval = detail.interval, enabled = enabled,
                )
            )
            null
        }
    }

    fun deleteProxy() {
        val panel = _state.value.proxyPanel ?: return
        val id = _state.value.proxyId ?: return
        scope.launch {
            try {
                clientFor(panel).deleteProxy(id)
                _state.update { state ->
                    val before = state.data[panel.id] ?: PanelData()
                    state.copy(
                        screen = Screen.PROXIES,
                        proxyId = null,
                        proxyDetail = null,
                        data = state.data + (panel.id to before.copy(proxies = before.proxies?.filter { it.id != id })),
                    )
                }
                restartPolling()
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { it.copy(proxyAction = message(e)) }
            }
        }
    }

    private fun proxyCall(done: String, block: suspend (MonitorApi, Long) -> String?) {
        val panel = _state.value.proxyPanel ?: return
        val id = _state.value.proxyId ?: return
        _state.update { it.copy(proxyAction = "正在执行…") }
        scope.launch {
            val text = try {
                block(clientFor(panel), id) ?: done
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                message(e)
            }
            _state.update { if (it.proxyId == id) it.copy(proxyAction = text) else it }
            pollProxyDetail(_state.value)
        }
    }

    fun clearProxyAction() = _state.update { it.copy(proxyAction = null) }

    companion object {
        fun message(e: Throwable): String = when (e) {
            is MonitorError -> e.message.orEmpty()
            is SpeedTestError -> e.message.orEmpty()
            is java.net.UnknownHostException -> "找不到这个地址，请检查面板地址"
            is java.net.SocketTimeoutException -> "连接超时，请检查网络或端口"
            is java.net.ConnectException -> "连接被拒绝，请检查地址和端口"
            is javax.net.ssl.SSLException -> "HTTPS 连接失败：${e.message.orEmpty()}"
            else -> e.message ?: e.javaClass.simpleName
        }

        /** Best of three TCP connects to host:port. */
        suspend fun tcpPing(host: String, port: Int, attempts: Int = 3): NodePing = withContext(Dispatchers.IO) {
            var best: Double? = null
            var error: String? = null
            repeat(attempts) {
                val started = System.nanoTime()
                try {
                    Socket().use { it.connect(InetSocketAddress(host, port), 3_000) }
                    val ms = (System.nanoTime() - started) / 1e6
                    best = best?.let { minOf(it, ms) } ?: ms
                } catch (e: Exception) {
                    error = when (e) {
                        is java.net.SocketTimeoutException -> "超时"
                        is java.net.ConnectException -> "拒绝连接"
                        else -> e.javaClass.simpleName
                    }
                }
            }
            NodePing(best, if (best == null) error else null)
        }
    }
}
