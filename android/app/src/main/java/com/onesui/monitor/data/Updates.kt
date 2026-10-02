package com.onesui.monitor.data

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject
import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/** The latest 1S-UI release: the panel and this app are published together. */
data class ReleaseInfo(
    val tag: String,
    val notes: String = "",
    val publishedAt: String = "",
    /** False when the release is known to carry no app (its API listing had no APK). */
    val hasApk: Boolean = true,
) {
    val version: String get() = tag.removePrefix("v")
    val pageUrl: String get() = "https://github.com/$REPO/releases/tag/$tag"
    val apkUrl: String get() = "https://github.com/$REPO/releases/download/$tag/$APK_NAME"

    /** The APK through a GitHub mirror, for phones in mainland China. */
    val mirrorApkUrl: String get() = MIRRORS.first() + apkUrl

    companion object {
        const val REPO = "Hhz0823/1s-ui"
        const val APK_NAME = "1s-ui-monitor-android.apk"
        val MIRRORS = listOf("https://ghfast.top/", "https://gh-proxy.com/", "https://ghproxy.net/")
    }
}

object Versions {
    /**
     * Compares release versions such as "v1.7.0" and "1.6.10" by their numbers.
     * Suffixes ("1.6.0-boost") do not count, so a build is never offered a
     * release with the same numbers.
     */
    fun compare(a: String, b: String): Int {
        val left = numbers(a)
        val right = numbers(b)
        for (i in 0 until maxOf(left.size, right.size)) {
            val diff = left.getOrElse(i) { 0 }.compareTo(right.getOrElse(i) { 0 })
            if (diff != 0) return diff
        }
        return 0
    }

    /** Whether [candidate] is newer than [current]; an unknown current version never is. */
    fun isNewer(candidate: String, current: String): Boolean =
        candidate.isNotBlank() && numbers(current).isNotEmpty() && compare(candidate, current) > 0

    private fun numbers(version: String): List<Int> = version.trim().removePrefix("v").removePrefix("V")
        .substringBefore('-').substringBefore('+')
        .split('.')
        .map { part -> part.takeWhile { it.isDigit() } }
        .takeWhile { it.isNotEmpty() }
        .map { it.toIntOrNull() ?: 0 }
}

/** One HTTP answer: redirects are not followed, so [location] can be read. */
data class HttpAnswer(val code: Int, val body: String, val location: String = "")

/**
 * Finds the latest release: GitHub's API first, then the redirect of the
 * release page, from GitHub and then through mirrors, because phones in
 * mainland China often cannot reach api.github.com.
 */
class ReleaseChecker(private val get: (String) -> HttpAnswer = ::httpGet) {
    suspend fun latest(): ReleaseInfo = withContext(Dispatchers.IO) {
        runCatching { fromApi() }.getOrNull()?.let { return@withContext it }
        for (source in listOf("") + ReleaseInfo.MIRRORS) {
            runCatching { fromPage(source) }.getOrNull()?.let { return@withContext it }
        }
        throw IOException("连不上 GitHub，暂时无法检查更新，请稍后再试")
    }

    private fun fromApi(): ReleaseInfo? {
        val answer = get("https://api.github.com/repos/${ReleaseInfo.REPO}/releases/latest")
        if (answer.code != 200) return null
        val json = JSONObject(answer.body)
        val tag = json.optString("tag_name").takeIf { validTag(it) } ?: return null
        val assets = json.optJSONArray("assets")
        val hasApk = assets == null || (0 until assets.length()).any { assets.optJSONObject(it)?.optString("name") == ReleaseInfo.APK_NAME }
        return ReleaseInfo(tag, json.optString("body"), json.optString("published_at"), hasApk)
    }

    private fun fromPage(source: String): ReleaseInfo? {
        val answer = get("${source}https://github.com/${ReleaseInfo.REPO}/releases/latest")
        val tag = tagIn(answer.location) ?: if (answer.code == 200) tagIn(answer.body) else null
        return tag?.let { ReleaseInfo(it) }
    }

    companion object {
        private val tagPattern = Regex("/${Regex.escape(ReleaseInfo.REPO)}/releases/tag/([^\"'?#<>\\s/]+)")

        fun tagIn(text: String): String? = tagPattern.find(text)?.groupValues?.get(1)?.takeIf(::validTag)

        private fun validTag(tag: String) = tag.matches(Regex("v?[0-9][0-9A-Za-z._-]{0,40}"))

        fun httpGet(url: String): HttpAnswer {
            val connection = URL(url).openConnection() as HttpURLConnection
            try {
                connection.instanceFollowRedirects = false
                connection.connectTimeout = 8_000
                connection.readTimeout = 10_000
                connection.setRequestProperty("Accept", "application/vnd.github+json, text/html")
                connection.setRequestProperty("User-Agent", "1s-ui-monitor")
                val code = connection.responseCode
                val body = if (code == 200) connection.inputStream.bufferedReader().use { it.readText().take(1 shl 20) } else ""
                return HttpAnswer(code, body, connection.getHeaderField("Location").orEmpty())
            } finally {
                connection.disconnect()
            }
        }
    }
}
