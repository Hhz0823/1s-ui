package com.onesui.monitor

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import com.onesui.monitor.data.Store

class MonitorApp : Application() {
    lateinit var store: Store
        private set

    override fun onCreate() {
        super.onCreate()
        store = Store(this)
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_ALERTS, "服务器告警", NotificationManager.IMPORTANCE_HIGH).apply {
                description = "服务器离线、恢复、资源过高和代理不可用提醒"
            }
        )
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_SERVICE, "后台监控", NotificationManager.IMPORTANCE_MIN).apply {
                description = "后台监控运行时的常驻通知"
                setShowBadge(false)
            }
        )
    }

    companion object {
        const val CHANNEL_ALERTS = "alerts"
        const val CHANNEL_SERVICE = "monitor_service"
    }
}
