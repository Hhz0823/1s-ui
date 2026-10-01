# 1S-UI 安卓监控 App

用手机实时查看面板里所有服务器的状态：在线情况、CPU、内存、磁盘、网络速度、流量、负载、延迟和历史曲线。App 可以在后台定时检查，服务器离线、恢复或资源过高时发通知。

## 绑定面板

1. 在面板打开「设置 → 前端与后端」，找到「手机监控 App」，点「生成密钥」。
2. 在 App 里点「扫码绑定」扫描面板显示的二维码；也可以在面板复制绑定码后点「粘贴绑定码」，或手动填写面板地址和监控密钥。
3. 可以绑定多个面板，在顶部标题处切换。

监控密钥是只读的：只能访问 `/apiv2/monitor/*`，不能登录面板、不能读取节点配置、不能执行任何操作。密钥只在生成时显示一次，面板只保存它的哈希。重新生成或停用后，旧密钥立即失效。

面板地址填写浏览器里打开面板的地址即可，例如 `http://1.2.3.4:2095/` 或 `https://panel.example.com/app/`。没有写 `http://` 或 `https://` 时按 `http://` 处理。HTTPS 面板使用自签证书时，App 会显示证书指纹，确认后固定信任这张证书；证书变化后 App 会拒绝连接，需要解绑后重新绑定。

## 数据接口

| 接口 | 说明 |
| --- | --- |
| `GET /apiv2/monitor/servers` | 主控本机和所有子服务器的实时状态 |
| `GET /apiv2/monitor/servers/:id` | 单台服务器详情和历史，`id=0` 为主控本机 |

请求头带 `X-Monitor-Key: <密钥>` 或 `Authorization: Bearer <密钥>`。

## 构建

需要 JDK 17 和 Android SDK（compileSdk 35）。

```bash
cd android
./gradlew testDebugUnitTest assembleRelease
```

GitHub Actions 的「Android 监控 App」工作流会在修改 `android/` 时构建 APK；发布版本时 APK 会作为 `1s-ui-monitor-android.apk` 附在 Release 上。

要让每次发布的 APK 能直接覆盖升级，需要在仓库 Secrets 里配置固定的签名密钥：

| Secret | 内容 |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | `base64 -w0 release.jks` 的输出 |
| `ANDROID_KEYSTORE_PASSWORD` | keystore 密码 |
| `ANDROID_KEY_ALIAS` | 密钥别名 |
| `ANDROID_KEY_PASSWORD` | 密钥密码 |

没有配置时 APK 使用调试签名，可以安装使用，但不同构建之间不能覆盖升级，需要先卸载旧版。
