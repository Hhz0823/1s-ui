package com.onesui.monitor.data

/** Turns the panel's English messages and proxy check results into Chinese. */
object Messages {
    private val panelMessages = listOf(
        "managed server is offline" to "子服务器不在线",
        "not connected via WebSocket" to "子服务器不在线",
        "controller mode is disabled" to "面板未开启主控模式",
        "only allows read-only metrics" to "主控为仅监控模式，不能执行这个操作",
        "too old to check proxies" to "这台服务器的面板版本过旧，不能检测代理，请先更新",
        "too old for speed tests" to "这台服务器的面板版本过旧，不支持测速，请先更新",
        "speed tests from the app are turned off" to "面板已关闭 App 测速，可在面板「设置 → 手机监控 App」里开启",
        "managing proxy monitors from the app is turned off" to "面板已关闭 App 管理代理监测，可在面板「设置 → 手机监控 App」里开启",
        "too many proxy checks at once" to "同时检测的代理太多，请稍后再试",
        "a speed test is running on port" to "这台服务器正在用另一个端口测速，请稍后再试",
        "too many speed tests at once" to "同时进行的测速太多，请稍后再试",
        "speed test port" to "测速端口被占用，请在面板「设置 → 手机监控 App」里换一个端口",
        "proxy monitor not found" to "代理监测不存在，可能已被删除",
        "at most 100 proxy monitors" to "代理监测最多 100 个",
        "check interval must be" to "检测间隔需在 30～3600 秒之间",
        "the server chosen to run the check does not exist" to "选择的检测服务器不存在",
        "proxy type must be socks5 or http" to "代理类型只能是 SOCKS5 或 HTTP",
        "proxy host must be" to "代理地址格式不对",
        "proxy port must be" to "代理端口需在 1～65535 之间",
        "username and password must be at most" to "用户名和密码最多 255 个字节",
        "test URL must be" to "测试网址需以 http:// 或 https:// 开头",
        "HTTPS proxies" to "暂不支持 HTTPS 代理，请用 http:// 或 socks5://",
        "unsupported proxy" to "无法识别的格式，支持 socks5://、http://、IP:端口:用户名:密码",
        "the proxy link has no port" to "代理链接缺少端口",
        "invalid proxy" to "代理格式不对",
        "monitor name must be" to "名称最多 80 个字，不能含控制字符",
        "RPC timed out" to "子服务器响应超时",
        "command timed out" to "子服务器响应超时",
        "RPC queue is busy" to "子服务器正忙，请稍后再试",
    )

    fun panel(message: String): String {
        val text = message.trim()
        if (text.isEmpty()) return text
        return panelMessages.firstOrNull { text.contains(it.first, ignoreCase = true) }?.second ?: text
    }

    /** One line explaining a failed check; empty for a good one. */
    fun probe(result: ProbeResult): String {
        if (result.ok) return ""
        val error = result.error
        return when (result.stage) {
            "server" -> "无法检测：" + panel(error)
            "config" -> "设置有误：" + panel(error)
            "connect" -> "连不上代理：" + when {
                error.contains("refused") -> "连接被拒绝，端口没开或代理没运行"
                error.contains("timed out") -> "连接超时，地址不对或被防火墙拦截"
                error.contains("not allowed") -> "不能检测本机、链路本地或保留地址"
                error.contains("no such host") -> "找不到这个域名"
                error.contains("unreachable") -> "网络不可达"
                else -> error
            }
            "auth" -> "认证失败：" + when {
                error.contains("requires") -> "代理需要用户名和密码"
                error.contains("rejected") -> "用户名或密码错误"
                error.contains("none of the offered") -> "代理不接受这种登录方式"
                else -> error
            }
            "tunnel" -> "代理没有接通目标：" + when {
                error.contains("is it an HTTP proxy") -> "没有 SOCKS5 回应，可能是 HTTP 代理"
                error.contains("is it a SOCKS5 proxy") -> "没有 HTTP 代理回应，可能是 SOCKS5 代理"
                error.contains("refused the tunnel") -> "代理拒绝了请求（${error.substringAfter("tunnel: ")}）"
                error.contains("not allowed by ruleset") -> "代理规则不允许访问"
                error.contains("host unreachable") -> "目标主机不可达"
                error.contains("network unreachable") -> "目标网络不可达"
                error.contains("refused by the destination") -> "目标拒绝连接"
                error.contains("general SOCKS server failure") -> "代理服务器内部错误"
                error.contains("timed out") -> "代理响应超时"
                error.contains("closed") -> "代理断开了连接"
                else -> error
            }
            "tls" -> "经代理的 TLS 握手失败：$error"
            "http" -> "测试网址没有正常响应：" + if (error.startsWith("test URL answered")) "返回 ${error.substringAfter("answered ")}" else error
            else -> error.ifBlank { "检测失败" }
        }
    }
}
