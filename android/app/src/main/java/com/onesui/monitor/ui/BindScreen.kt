package com.onesui.monitor.ui

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.journeyapps.barcodescanner.ScanContract
import com.journeyapps.barcodescanner.ScanOptions
import com.onesui.monitor.data.BindCode
import kotlinx.coroutines.launch

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BindScreen(state: UiState, vm: MonitorViewModel) {
    var name by rememberSaveable { mutableStateOf("") }
    var url by rememberSaveable { mutableStateOf("") }
    var key by rememberSaveable { mutableStateOf("") }
    var showKey by rememberSaveable { mutableStateOf(false) }
    var busy by rememberSaveable { mutableStateOf(false) }
    var error by rememberSaveable { mutableStateOf<String?>(null) }
    var trustPrompt by rememberSaveable { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    val clipboard = LocalClipboardManager.current

    fun submit(pin: String = "") {
        busy = true
        error = null
        scope.launch {
            when (val result = vm.bind(name, url, key, pin)) {
                BindResult.Success -> Unit
                is BindResult.NeedsTrust -> trustPrompt = result.fingerprint
                is BindResult.Failure -> error = result.message
            }
            busy = false
        }
    }

    fun applyCode(text: String): Boolean {
        val code = BindCode.parse(text) ?: return false
        url = code.url
        key = code.key
        submit()
        return true
    }

    val scanner = rememberLauncherForActivityResult(ScanContract()) { result ->
        val text = result.contents ?: return@rememberLauncherForActivityResult
        if (!applyCode(text)) error = "这不是面板的绑定二维码"
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("绑定面板") },
                navigationIcon = {
                    if (state.panels.isNotEmpty()) {
                        IconButton(onClick = { vm.back() }) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回") }
                    }
                },
            )
        },
    ) { padding ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            Text(
                "在面板「设置 → 前端与后端 → 手机监控 App」里生成监控密钥，然后扫描二维码，或手动填写面板地址和密钥。密钥只读，不能修改面板。",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Button(
                    onClick = {
                        scanner.launch(
                            ScanOptions()
                                .setDesiredBarcodeFormats(ScanOptions.QR_CODE)
                                .setPrompt("扫描面板里的绑定二维码")
                                .setBeepEnabled(false)
                                .setOrientationLocked(false)
                        )
                    },
                    enabled = !busy,
                    modifier = Modifier.weight(1f),
                ) { Text("扫码绑定") }
                OutlinedButton(
                    onClick = {
                        val text = clipboard.getText()?.text.orEmpty()
                        if (!applyCode(text)) error = "剪贴板里没有绑定码，请在面板里点复制绑定码"
                    },
                    enabled = !busy,
                    modifier = Modifier.weight(1f),
                ) { Text("粘贴绑定码") }
            }
            Text("或手动填写", style = MaterialTheme.typography.labelLarge)
            OutlinedTextField(
                value = name,
                onValueChange = { name = it },
                label = { Text("名称（可选）") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = url,
                onValueChange = { url = it },
                label = { Text("面板地址") },
                placeholder = { Text("http://1.2.3.4:2095/") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = key,
                onValueChange = { key = it },
                label = { Text("监控密钥") },
                singleLine = true,
                visualTransformation = if (showKey) VisualTransformation.None else PasswordVisualTransformation(),
                trailingIcon = { TextButton(onClick = { showKey = !showKey }) { Text(if (showKey) "隐藏" else "显示") } },
                modifier = Modifier.fillMaxWidth(),
            )
            error?.let { Text(it, color = MaterialTheme.colorScheme.error) }
            Button(onClick = { submit() }, enabled = !busy, modifier = Modifier.fillMaxWidth()) {
                if (busy) CircularProgressIndicator(Modifier.height(18.dp), strokeWidth = 2.dp) else Text("连接并绑定")
            }
            Spacer(Modifier.height(24.dp))
        }
    }

    trustPrompt?.let { fingerprint ->
        AlertDialog(
            onDismissRequest = { trustPrompt = null },
            title = { Text("信任这个证书吗？") },
            text = {
                Text(
                    "面板使用的 HTTPS 证书不被系统信任（常见于自签证书）。请确认下面的指纹与面板证书一致，再选择信任。之后证书一旦变化，App 会拒绝连接。\n\nSHA-256：\n$fingerprint",
                )
            },
            confirmButton = {
                TextButton(onClick = { trustPrompt = null; submit(fingerprint) }) { Text("信任并绑定") }
            },
            dismissButton = { TextButton(onClick = { trustPrompt = null }) { Text("取消") } },
        )
    }
}
