package com.onesui.monitor.service

import android.Manifest
import android.app.Notification
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat
import com.onesui.monitor.MonitorApp
import com.onesui.monitor.R
import com.onesui.monitor.data.AlertEvaluator
import com.onesui.monitor.data.MonitorClient
import com.onesui.monitor.data.ServerAlertState
import com.onesui.monitor.data.Store
import com.onesui.monitor.ui.MainActivity
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * Polls every bound panel in the background and raises notifications when a
 * server goes offline, comes back, or crosses a resource threshold.
 */
class AlertService : Service() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var loop: Job? = null
    private val states = mutableMapOf<String, ServerAlertState>()
    private val unreachable = mutableSetOf<String>()

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val store = (application as MonitorApp).store
        if (!store.settings().alertsEnabled || store.panels().isEmpty()) {
            stopSelf()
            return START_NOT_STICKY
        }
        ServiceCompat.startForeground(
            this,
            FOREGROUND_ID,
            foregroundNotification("正在监控 ${store.panels().size} 个面板"),
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE else 0,
        )
        if (loop?.isActive != true) loop = scope.launch { run(store) }
        return START_STICKY
    }

    private suspend fun run(store: Store) {
        while (scope.isActive) {
            val settings = store.settings()
            if (!settings.alertsEnabled) break
            var online = 0
            var total = 0
            for (panel in store.panels()) {
                val overview = runCatching { MonitorClient(panel).overview() }.getOrNull()
                if (overview == null) {
                    if (unreachable.add(panel.id)) notify("panel:${panel.id}", "${panel.name} 无法连接", "请检查网络或面板是否在运行", false)
                    continue
                }
                if (unreachable.remove(panel.id)) notify("panel:${panel.id}", "${panel.name} 已恢复连接", "面板可以正常访问了", true)
                val (alerts, next) = AlertEvaluator.evaluate(panel, overview.servers, states, settings)
                states.putAll(next)
                alerts.forEach { notify(it.key, it.title, it.text, it.recovered) }
                total += overview.servers.size
                online += overview.servers.count { it.online }
            }
            updateForeground("在线 $online / $total 台 · 每 ${settings.alertIntervalSeconds} 秒检查")
            delay(settings.alertIntervalSeconds.coerceAtLeast(15) * 1000L)
        }
        stopSelf()
    }

    private fun notify(key: String, title: String, text: String, recovered: Boolean) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) return
        val notification = NotificationCompat.Builder(this, MonitorApp.CHANNEL_ALERTS)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle(title)
            .setContentText(text)
            .setContentIntent(openApp())
            .setAutoCancel(true)
            .setCategory(NotificationCompat.CATEGORY_STATUS)
            .setPriority(if (recovered) NotificationCompat.PRIORITY_DEFAULT else NotificationCompat.PRIORITY_HIGH)
            .build()
        NotificationManagerCompat.from(this).notify(key.hashCode(), notification)
    }

    private fun updateForeground(text: String) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) return
        NotificationManagerCompat.from(this).notify(FOREGROUND_ID, foregroundNotification(text))
    }

    private fun foregroundNotification(text: String): Notification =
        NotificationCompat.Builder(this, MonitorApp.CHANNEL_SERVICE)
            .setSmallIcon(R.drawable.ic_launcher_foreground)
            .setContentTitle("后台监控运行中")
            .setContentText(text)
            .setContentIntent(openApp())
            .setOngoing(true)
            .setSilent(true)
            .build()

    private fun openApp(): PendingIntent = PendingIntent.getActivity(
        this, 0, Intent(this, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP),
        PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
    )

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    companion object {
        private const val FOREGROUND_ID = 1

        /** Starts or stops the service to match the saved settings. */
        fun sync(context: Context) {
            val store = (context.applicationContext as MonitorApp).store
            val intent = Intent(context, AlertService::class.java)
            if (store.settings().alertsEnabled && store.panels().isNotEmpty()) {
                runCatching { ContextCompat.startForegroundService(context, intent) }
            } else {
                context.stopService(intent)
            }
        }
    }
}
