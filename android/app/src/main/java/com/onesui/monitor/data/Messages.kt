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
        // Node monitors
        "too old to check nodes" to "这台服务器的面板版本过旧，不能检测节点，请先更新",
        "too old to share node links" to "节点所在服务器的面板版本过旧，请先更新",
        "unsupported share link" to "不支持的分享链接，请用 vless://、vmess://、trojan://、ss://、hy2://、tuic://、anytls:// 或 naive 链接",
        "invalid share link" to "分享链接格式不对",
        "share link is too long" to "分享链接太长",
        "the share link has no server address" to "分享链接里没有服务器地址",
        "a node check needs the node's share link" to "检测节点需要分享链接",
        "no enabled user of this inbound has a share link" to "这个入站没有启用的用户，无法生成链接",
        "inbound not found" to "这个入站不存在，可能已被删除",
        "the server of this node does not exist" to "节点所在的服务器不存在",
        "the server returned an invalid node link" to "节点所在服务器返回的链接无效",
        "proxy type must be socks5, http or node" to "类型只能是 SOCKS5、HTTP 或节点",
        // Relay speed tests
        "pick another server to test from" to "请选择另一台服务器发起测速",
        "the server to test from does not exist" to "发起测速的服务器不存在",
        "the server to test from is offline" to "发起测速的服务器不在线",
        "a speed test from or to one of these servers is already running" to "这两台服务器之一正在测速，请稍后再试",
        "another speed test is running on this server" to "这台服务器正在进行另一个测速，请稍后再试",
        "speed test not found" to "测速任务已结束或过期，请重新测速",
        "the server returned an invalid speed test result" to "服务器返回的测速结果无效",
        // Proxy client
        "has no proxy client yet" to "这台设备的面板版本过旧，没有客户端代理，请先更新",
        "managing the proxy client from the app is turned off" to "面板已关闭 App 管理客户端代理，可在面板「设置 → 手机监控 App」里开启",
        "the app cannot do this" to "App 不能做这个操作，请在面板里操作",
        "a latency test is already running" to "正在测延迟，请等它结束",
        "no node matches the automatic selection" to "没有符合自动选择条件的节点",
        "no node is selected" to "还没有选择节点",
        "the transparent proxy needs Linux" to "透明代理只支持 Linux",
        "the transparent proxy needs the panel to run as root" to "透明代理需要面板以 root 运行",
        "/dev/net/tun is missing" to "缺少 /dev/net/tun，请安装 TUN 内核模块（OpenWrt：opkg install kmod-tun）",
        "sing-box is not running" to "sing-box 没有运行",
        "the proxy client is not running" to "客户端代理没有运行",
        "the subscription is empty" to "订阅是空的",
        "this is a Clash subscription" to "这是 Clash 订阅，请改用 v2rayN（分享链接）或 sing-box 订阅地址",
        "no supported nodes in the subscription" to "订阅里没有支持的节点",
        "the subscription answered HTTP" to "订阅地址返回错误",
        "the subscription is too large" to "订阅内容太大",
        "subscription not found" to "订阅不存在，可能已被删除",
        "node not found" to "节点不存在，可能已被删除",
        "server not found" to "服务器不存在，可能已被删除",
    )

    fun panel(message: String): String {
        val text = message.trim()
        if (text.isEmpty()) return text
        return panelMessages.firstOrNull { text.contains(it.first, ignoreCase = true) }?.second ?: text
    }

    /** One line explaining a failed check of a proxy or, with [node], a node; empty for a good one. */
    fun probe(result: ProbeResult, node: Boolean = false): String {
        if (result.ok) return ""
        val error = result.error
        val what = if (node) "节点" else "代理"
        if (error.contains("did not relay")) return "节点没有转发连接：UUID / 密码不对、被干扰，或节点访问不了测试网址"
        return when (result.stage) {
            "server" -> "无法检测：" + panel(error)
            "config" -> "设置有误：" + panel(error)
            "connect" -> "连不上$what：" + when {
                error.contains("refused") -> "连接被拒绝，端口没开或代理没运行"
                error.contains("timed out") -> "连接超时，地址不对或被防火墙拦截"
                error.contains("not allowed") -> "不能检测本机、链路本地或保留地址"
                error.contains("no such host") -> "找不到这个域名"
                error.contains("unreachable") -> "网络不可达"
                else -> error
            }
            "handshake" -> "节点握手失败：" + when {
                error.contains("timed out") || error.contains("timeout") -> "握手超时，可能被干扰或端口不对"
                error.contains("reality", ignoreCase = true) -> "REALITY 验证失败，检查公钥、short ID 和 SNI"
                error.contains("certificate") -> "证书校验失败，检查 SNI 或是否允许不安全证书"
                error.contains("reset") || error.contains("EOF") -> "连接被重置，可能被干扰或协议设置不对"
                else -> error
            } + "（TLS、REALITY、QUIC 或登录被拒绝 / 干扰）"
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
            "tls" -> "经${what}的 TLS 握手失败：$error"
            "http" -> "测试网址没有正常响应：" + if (error.startsWith("test URL answered")) "返回 ${error.substringAfter("answered ")}" else error
            else -> error.ifBlank { "检测失败" }
        }
    }
}
