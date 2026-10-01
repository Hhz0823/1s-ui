package com.onesui.monitor.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.onesui.monitor.MonitorApp
import com.onesui.monitor.data.MonitorController
import com.onesui.monitor.service.AlertService

/** Hosts the [MonitorController] for the activity's lifetime. */
class MonitorViewModel(app: Application) : AndroidViewModel(app) {
    val controller = MonitorController(
        store = (app as MonitorApp).store,
        scope = viewModelScope,
        onPanelsChanged = { AlertService.sync(app) },
    )
}
