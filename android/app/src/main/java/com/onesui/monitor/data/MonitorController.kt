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
import org.json.JSONArray
import org.json.JSONObject
import java.net.InetSocketAddress
import java.net.Socket
import java.util.UUID
import kotlin.coroutines.cancellation.CancellationException

/** Selection meaning every bound panel at once. */
const val ALL_PANELS = "*"

enum class Screen { SERVERS, CLIENTS, PROXIES, DETAIL, CLIENT, PROXY_DETAIL, PROXY_EDIT, SETTINGS, BIND }

/** The home tabs; Back on them leaves the app. */
val HOME_SCREENS = setOf(Screen.SERVERS, Screen.CLIENTS, Screen.PROXIES)

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

/** A device whose proxy client the app can show: the panel host or a managed server. */
data class ClientDevice(val panel: Panel, val server: Server, val manage: Boolean) {
    val serverId: Long get() = if (server.local) 0 else server.id
    val key: String get() = "${panel.id}:$serverId"
}

/** The last state read from one device's proxy client. */
data class ClientView(val state: ClientState? = null, val error: String? = null, val loadedAt: Long = 0)

/** The latest release, compared with this app and with each bound panel. */
data class UpdateState(
    val appVersion: String = "",
    val checking: Boolean = false,
    val latest: ReleaseInfo? = null,
    val error: String? = null,
    val checkedAt: Long = 0,
    /** The banner was closed for this version. */
    val dismissed: Boolean = false,
) {
    val appUpdate: Boolean get() = latest?.hasApk == true && Versions.isNewer(latest.version, appVersion)

    /** Whether a panel or server running [version] is behind the latest release. */
    fun outdated(version: String): Boolean = latest != null && Versions.isNewer(latest.version, version)
}

/** What a monitor checks: a SOCKS5 / HTTP proxy, a node's share link, or a node picked from a server. */
enum class MonitorKind { PROXY, LINK, INBOUND }

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
    /** null tests from the phone; otherwise the server the test runs from (0 is the panel host). */
    val relayId: Long? = null,
    val relayName: String = "",
    /** The test a relay is running now, while the app polls it. */
    val relayCurrent: SpeedPhase? = null,
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
    val kind: MonitorKind = MonitorKind.PROXY,
    /** The inbounds of the server picked as the node's source. */
    val inbounds: List<NodeItem>? = null,
    val inboundsError: String? = null,
) {
    /** What is sent: the fields of the chosen kind only. */
    val prepared: ProxyInput
        get() = when (kind) {
            MonitorKind.PROXY -> input.copy(type = if (input.type == "node") "socks5" else input.type, nodeServerId = 0, nodeInboundId = 0)
            MonitorKind.LINK -> input.copy(type = "node", nodeServerId = 0, nodeInboundId = 0)
            MonitorKind.INBOUND -> input.copy(type = "node", link = "")
        }
}

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
    val clientKey: String? = null,
    val clientReturnTo: Screen = Screen.CLIENTS,
    val clientStates: Map<String, ClientView> = emptyMap(),
    /** The running client action, then its outcome until dismissed. */
    val clientAction: String? = null,
    val clientBusy: Boolean = false,
    val clientExit: ClientExit? = null,
    val update: UpdateState = UpdateState(),
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

    /** Devices with a proxy client in the panels in scope: the panel hosts and updated servers. */
    val clientDevices: List<ClientDevice>
        get() = scopedPanels.flatMap { panel -> devicesOf(panel) }

    private fun devicesOf(panel: Panel): List<ClientDevice> {
        val features = features(panel.id)
        if (!features.clients) return emptyList()
        return data[panel.id]?.overview?.servers.orEmpty()
            .filter { it.can(Capability.PROXY_CLIENT) }
            .map { ClientDevice(panel, it, features.manageClients) }
    }

    /** The device whose client is open, in any bound panel. */
    val openClient: ClientDevice?
        get() = panels.flatMap { devicesOf(it) }.firstOrNull { it.key == clientKey }

    /** The client device of the open server, when it has one. */
    val detailClient: ClientDevice?
        get() = detailPanel?.let { panel -> devicesOf(panel).firstOrNull { it.key == "${panel.id}:${if (detailLocal) 0 else detailId}" } }

    /** Servers of the open server's panel that can run a speed test to it. */
    val relaySources: List<Server>
        get() = data[detailPanelId]?.overview?.servers.orEmpty().filter { server ->
            server.online && server.can(Capability.SPEEDTEST_CLIENT) && (if (detailLocal) !server.local else server.local || server.id != detailId)
        }

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
    appVersion: String = "",
    private val latestRelease: suspend () -> ReleaseInfo = { ReleaseChecker().latest() },
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
            update = UpdateState(appVersion = appVersion),
        )
    }

    fun setForeground(value: Boolean) {
        foreground = value
        if (value) {
            restartPolling()
            maybeCheckUpdates()
        } else {
            poller?.cancel()
        }
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
                    Screen.CLIENTS, Screen.CLIENT -> maxOf(_state.value.settings.refreshSeconds, 10)
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
                Screen.CLIENTS -> {
                    pollOverviews(current.scopedPanels)
                    // A client's state carries all its nodes: read each device twice a minute at most.
                    val now = System.currentTimeMillis()
                    pollClients(_state.value.clientDevices.filter { it.server.online && now - (_state.value.clientStates[it.key]?.loadedAt ?: 0) >= 30_000 })
                }
                Screen.CLIENT -> {
                    val device = current.openClient
                    if (device == null) pollOverviews(current.panels.filter { current.data[it.id]?.overview == null })
                    pollClients(listOfNotNull(device ?: _state.value.openClient))
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

    private suspend fun pollClients(devices: List<ClientDevice>) = coroutineScope {
        devices.map { device ->
            async {
                val result = runCatching { clientFor(device.panel).clientState(device.serverId) }
                _state.update { state ->
                    val before = state.clientStates[device.key] ?: ClientView()
                    val after = result.fold(
                        { ClientView(it, null, System.currentTimeMillis()) },
                        { before.copy(error = message(it), loadedAt = System.currentTimeMillis()) },
                    )
                    state.copy(clientStates = state.clientStates + (device.key to after))
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
        maybeCheckUpdates()
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
            in HOME_SCREENS -> return false
            Screen.BIND -> if (current.panels.isEmpty()) return false else current.home
            Screen.PROXY_EDIT -> current.editing?.returnTo ?: Screen.PROXIES
            Screen.PROXY_DETAIL -> Screen.PROXIES
            Screen.CLIENT -> current.clientReturnTo
            else -> current.home
        }
        _state.update {
            it.copy(
                screen = target,
                editing = if (target == Screen.PROXY_EDIT) it.editing else null,
                clientAction = if (current.screen == Screen.CLIENT) null else it.clientAction,
                clientExit = if (current.screen == Screen.CLIENT) null else it.clientExit,
            )
        }
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

    /** Tests from the phone ([relay] null) or from another server of the same panel. */
    fun setSpeedSource(relay: Server?) {
        if (_state.value.speed.running) return
        _state.update { state ->
            state.copy(speed = SpeedState(serverKey = state.detailKey, relayId = relay?.let { if (it.local) 0L else it.id }, relayName = relay?.name.orEmpty()))
        }
    }

    fun startSpeedTest(suite: SpeedSuite) {
        val current = _state.value
        val panel = current.detailPanel ?: return
        val serverId = if (current.detailLocal) 0L else current.detailId ?: return
        val key = current.detailKey
        val settings = current.settings
        current.speed.relayId?.let { relayId ->
            startRelaySpeedTest(panel, serverId, relayId, suite)
            return
        }
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

    /**
     * Asks the panel to run the tests from [relayId] to [serverId] and polls
     * the job, filling the same results as a test from the phone.
     */
    private fun startRelaySpeedTest(panel: Panel, serverId: Long, relayId: Long, suite: SpeedSuite) {
        val key = _state.value.detailKey
        val settings = _state.value.settings
        val port = _state.value.features(panel.id).speedtestPort
        speedJob?.cancel()
        _state.update { state ->
            val cleared = suite.phases.fold(state.speed) { speed, phase -> clearPhase(speed, phase) }
            state.copy(speed = cleared.copy(serverKey = key, running = true, progress = null, error = null, host = "", port = port))
        }
        speedJob = scope.launch {
            try {
                val client = clientFor(panel)
                val options = RelayOptions(relayId, suite.phases.map { it.test }, settings.speedSeconds, settings.speedStreams, settings.udpMbps)
                var job = client.startRelaySpeedtest(serverId, options)
                while (true) {
                    applyRelayJob(key, job)
                    if (!job.running) break
                    delay(RELAY_POLL_MS)
                    job = client.relaySpeedtest(job.id)
                }
                if (job.error.isNotBlank()) updateSpeed(key) { it.copy(error = Messages.panel(job.error)) }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                updateSpeed(key) { it.copy(error = message(e)) }
            } finally {
                updateSpeed(key) { it.copy(running = false, progress = null, relayCurrent = null) }
            }
        }
    }

    private fun applyRelayJob(key: String, job: RelayJob) = updateSpeed(key) { speed ->
        var next = speed.copy(
            relayCurrent = if (job.running) SpeedPhase.entries.firstOrNull { it.test == job.current } else null,
            relayName = job.relayName.ifBlank { speed.relayName },
            host = job.host.ifBlank { speed.host },
        )
        for (result in job.results) {
            val phase = SpeedPhase.entries.firstOrNull { it.test == result.test } ?: continue
            if (result.error.isNotBlank()) {
                next = next.copy(errors = next.errors + (phase to Messages.panel(result.error)))
                continue
            }
            next = when (phase) {
                SpeedPhase.TCP_PING -> next.copy(tcpPing = result.ping)
                SpeedPhase.UDP_PING -> next.copy(udpPing = result.ping)
                SpeedPhase.TCP_DOWNLOAD -> next.copy(tcpDown = result.throughput)
                SpeedPhase.TCP_UPLOAD -> next.copy(tcpUp = result.throughput)
                SpeedPhase.UDP_DOWNLOAD -> next.copy(udpDown = result.udp)
                SpeedPhase.UDP_UPLOAD -> next.copy(udpUp = result.udp)
            }
            if (result.host.isNotBlank()) next = next.copy(host = result.host)
        }
        next
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
        val input = monitor?.let(::inputOf) ?: ProxyInput()
        val kind = when {
            monitor?.isNode != true -> MonitorKind.PROXY
            monitor.nodeInboundId != 0L -> MonitorKind.INBOUND
            else -> MonitorKind.LINK
        }
        _state.update {
            it.copy(
                screen = Screen.PROXY_EDIT,
                editing = ProxyEditState(panelId, input, hadPassword = monitor?.hasPassword == true, returnTo = it.screen, kind = kind),
            )
        }
        val panel = _state.value.panels.firstOrNull { it.id == panelId } ?: return
        if (_state.value.data[panelId]?.overview == null) scope.launch { pollOverviews(listOf(panel)) }
        if (kind == MonitorKind.INBOUND) loadInbounds(input.nodeServerId)
    }

    /** A monitor as the editor sends it back: the stored password and link are kept. */
    private fun inputOf(monitor: ProxyMonitor) = ProxyInput(
        id = monitor.id, name = monitor.name, type = monitor.type, host = monitor.host, port = monitor.port,
        username = monitor.username, password = null, target = monitor.target, serverId = monitor.serverId,
        interval = monitor.interval, enabled = monitor.enabled, nodeServerId = monitor.nodeServerId, nodeInboundId = monitor.nodeInboundId,
    )

    fun updateEditing(transform: (ProxyInput) -> ProxyInput) {
        _state.update { state -> state.copy(editing = state.editing?.let { it.copy(input = transform(it.input), error = null) }) }
    }

    fun setEditKind(kind: MonitorKind) {
        val editing = _state.value.editing ?: return
        if (editing.kind == kind) return
        _state.update { state -> state.copy(editing = state.editing?.copy(kind = kind, error = null, testResult = null)) }
        if (kind == MonitorKind.INBOUND && editing.inbounds == null) loadInbounds(editing.input.nodeServerId)
    }

    /** Picks the server whose inbounds the node comes from (0 is the panel host). */
    fun setNodeServer(serverId: Long) {
        updateEditing { it.copy(nodeServerId = serverId, nodeInboundId = 0) }
        loadInbounds(serverId)
    }

    private fun loadInbounds(serverId: Long) {
        val editing = _state.value.editing ?: return
        val panel = _state.value.panels.firstOrNull { it.id == editing.panelId } ?: return
        _state.update { state -> state.copy(editing = state.editing?.copy(inbounds = null, inboundsError = null)) }
        scope.launch {
            val result = runCatching { clientFor(panel).nodes(serverId).filter { it.id > 0 && it.port > 0 } }
            _state.update { state ->
                val current = state.editing
                if (current == null || current.input.nodeServerId != serverId) return@update state
                state.copy(
                    editing = result.fold(
                        { current.copy(inbounds = it, inboundsError = if (it.isEmpty()) "这台服务器还没有节点" else null) },
                        { current.copy(inbounds = emptyList(), inboundsError = message(it)) },
                    )
                )
            }
        }
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
        val missing = when {
            editing.kind == MonitorKind.INBOUND && editing.input.nodeInboundId <= 0 -> "请选择要监测的节点"
            editing.kind == MonitorKind.LINK && editing.input.id == 0L && editing.input.link.isBlank() -> "请粘贴节点的分享链接"
            else -> null
        }
        if (missing != null) {
            _state.update { it.copy(editing = editing.copy(error = missing)) }
            return
        }
        _state.update { it.copy(editing = editing.copy(busy = true, error = null, testResult = null)) }
        scope.launch {
            try {
                block(clientFor(panel), editing.prepared)
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { state -> state.copy(editing = state.editing?.copy(busy = false, error = message(e))) }
            }
        }
    }

    fun checkProxyNow() = proxyCall("检测完成") { client, id ->
        val result = client.checkProxy(id)
        val node = _state.value.proxyDetail?.monitor?.isNode == true
        if (!result.ok) Messages.probe(result, node) else "可用 · ${result.latencyMs} ms"
    }

    fun setProxyEnabled(enabled: Boolean) {
        val detail = _state.value.proxyDetail?.monitor ?: return
        proxyCall(if (enabled) "已恢复检测" else "已暂停检测") { client, _ ->
            client.saveProxy(inputOf(detail).copy(enabled = enabled))
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

    // Proxy client of a device

    fun openClient(device: ClientDevice) {
        _state.update {
            it.copy(
                screen = Screen.CLIENT,
                clientKey = device.key,
                clientReturnTo = if (it.screen == Screen.DETAIL) Screen.DETAIL else Screen.CLIENTS,
                clientAction = null,
                clientExit = null,
            )
        }
        restartPolling()
    }

    fun clientSetEnabled(enabled: Boolean) =
        clientCall(if (enabled) "正在开启代理…" else "正在关闭代理…", "mode", JSONObject().put("enabled", enabled)) {
            if (enabled) "代理已开启" else "代理已关闭"
        }

    fun clientSetMode(mode: String) =
        clientCall("正在切换模式…", "mode", JSONObject().put("mode", mode)) { "已切换为${ClientModes.label(mode)}" }

    /** Uses node [nodeId], or with 0 the fastest of the automatic selection. */
    fun clientSelect(nodeId: Long) = clientCall("正在切换节点…", "select", JSONObject().put("node_id", nodeId)) { state ->
        if (nodeId == 0L) "已改为自动选择最快节点" else "已切换到 ${state.current?.name ?: "所选节点"}"
    }

    /** Tests [ids], or every node; the device measures them through its own line. */
    fun clientTest(ids: List<Long> = emptyList()) =
        clientCall("正在测延迟，节点多时需要几分钟…", "test", JSONObject().put("ids", JSONArray(ids))) { state ->
            val tested = state.nodes.filter { ids.isEmpty() || it.id in ids }
            "延迟测试完成：${tested.count { it.delayMs > 0 }} / ${tested.size} 个可用"
        }

    /** Updates subscription [id], or every enabled one with 0. */
    fun clientUpdateSubscription(id: Long = 0) =
        clientCall("正在更新订阅…", "subscription.update", JSONObject().put("id", id)) { "订阅已更新" }

    /** Checks where traffic through the client leaves the internet. */
    fun clientCheckExit() {
        val device = _state.value.openClient ?: return
        if (_state.value.clientBusy) return
        _state.update { it.copy(clientBusy = true, clientAction = "正在检测出口…", clientExit = null) }
        scope.launch {
            try {
                val exit = clientFor(device.panel).clientExit(device.serverId)
                _state.update {
                    if (it.clientKey != device.key) it else it.copy(
                        clientExit = exit,
                        clientAction = if (exit.ok) null else "出口检测失败：" + Messages.panel(exit.error.ifBlank { "没有回应" }),
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { if (it.clientKey != device.key) it else it.copy(clientAction = message(e)) }
            } finally {
                _state.update { it.copy(clientBusy = false) }
            }
        }
    }

    fun clearClientAction() = _state.update { it.copy(clientAction = null) }

    private fun clientCall(running: String, action: String, data: JSONObject, done: (ClientState) -> String) {
        val device = _state.value.openClient ?: return
        if (_state.value.clientBusy) return
        _state.update { it.copy(clientBusy = true, clientAction = running) }
        scope.launch {
            try {
                val next = clientFor(device.panel).clientCall(device.serverId, action, data)
                _state.update {
                    it.copy(
                        clientStates = it.clientStates + (device.key to ClientView(next, null, System.currentTimeMillis())),
                        clientAction = if (it.clientKey == device.key) done(next) else it.clientAction,
                    )
                }
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                _state.update { if (it.clientKey != device.key) it else it.copy(clientAction = message(e)) }
            } finally {
                _state.update { it.copy(clientBusy = false) }
            }
        }
    }

    // Updates

    /** Checks for a new release at most every six hours, when the app comes to the front. */
    private fun maybeCheckUpdates() {
        val update = _state.value.update
        if (update.checking || System.currentTimeMillis() - update.checkedAt < UPDATE_INTERVAL_MS) return
        checkUpdates()
    }

    fun checkUpdates() {
        if (_state.value.update.checking) return
        _state.update { it.copy(update = it.update.copy(checking = true, error = null)) }
        scope.launch {
            val result = try {
                Result.success(latestRelease())
            } catch (e: CancellationException) {
                throw e
            } catch (e: Exception) {
                Result.failure(e)
            }
            _state.update { state ->
                val before = state.update
                state.copy(
                    update = result.fold(
                        { latest -> before.copy(checking = false, latest = latest, error = null, checkedAt = System.currentTimeMillis(), dismissed = before.dismissed && before.latest?.tag == latest.tag) },
                        { e -> before.copy(checking = false, error = message(e), checkedAt = System.currentTimeMillis()) },
                    )
                )
            }
        }
    }

    fun dismissUpdate() = _state.update { it.copy(update = it.update.copy(dismissed = true)) }

    companion object {
        private const val RELAY_POLL_MS = 1_000L
        private const val UPDATE_INTERVAL_MS = 6 * 3_600_000L

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
