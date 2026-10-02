package com.onesui.monitor.data

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.awaitCancellation
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.Closeable
import java.io.DataInputStream
import java.io.IOException
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetSocketAddress
import java.net.PortUnreachableException
import java.net.Socket
import java.net.SocketTimeoutException
import java.nio.ByteBuffer
import java.security.SecureRandom
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicLongArray
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min

enum class SpeedPhase(val label: String, val test: String) {
    TCP_PING("TCP 延迟", "tcp_ping"),
    UDP_PING("UDP 延迟", "udp_ping"),
    TCP_DOWNLOAD("TCP 下载", "tcp_download"),
    TCP_UPLOAD("TCP 上传", "tcp_upload"),
    UDP_DOWNLOAD("UDP 下载", "udp_download"),
    UDP_UPLOAD("UDP 上传", "udp_upload"),
}

/** Live state of the running phase: [fraction] of its time, current rate or round trip. */
data class SpeedProgress(val phase: SpeedPhase, val fraction: Float, val bitsPerSecond: Double = 0.0, val latencyMs: Double? = null)

data class PingStats(val sent: Int, val received: Int, val minMs: Double, val avgMs: Double, val maxMs: Double, val jitterMs: Double) {
    val lossPct: Double get() = if (sent > 0) (sent - received) * 100.0 / sent else 0.0
}

data class ThroughputStats(val bytes: Long, val seconds: Double, val bitsPerSecond: Double, val streams: Int)

data class UdpStats(
    val targetMbps: Int,
    val sentPackets: Long,
    val receivedPackets: Long,
    val bytes: Long,
    val seconds: Double,
    val bitsPerSecond: Double,
    val jitterMs: Double,
    val outOfOrder: Long,
) {
    val lossPct: Double get() = if (sentPackets > 0) max(0.0, (sentPackets - receivedPackets) * 100.0 / sentPackets) else 0.0
}

class SpeedTestError(message: String) : IOException(message)

/**
 * Client of the panel's speed test port (backend/speedtest). One [SpeedTester]
 * belongs to one session token; every test opens its own connections, and
 * every UDP test its own local port, as the server tells tests apart by them.
 */
class SpeedTester(val host: String, val port: Int, private val token: ByteArray) {

    suspend fun tcpPing(count: Int = 10, onProgress: (SpeedProgress) -> Unit = {}): PingStats = withContext(Dispatchers.IO) {
        val socket = openTcp(MODE_ECHO, count * 300 + 2_000)
        socket.use {
            val output = socket.getOutputStream()
            val input = DataInputStream(socket.getInputStream())
            val rtts = mutableListOf<Double>()
            val buffer = ByteBuffer.allocate(8)
            for (seq in 0 until count) {
                val started = System.nanoTime()
                buffer.clear()
                buffer.putLong(seq.toLong())
                output.write(buffer.array())
                output.flush()
                val reply = ByteArray(8)
                input.readFully(reply)
                val rtt = (System.nanoTime() - started) / 1e6
                if (ByteBuffer.wrap(reply).long == seq.toLong()) rtts += rtt
                onProgress(SpeedProgress(SpeedPhase.TCP_PING, (seq + 1f) / count, latencyMs = rtt))
                delay(100)
            }
            pingStats(count, rtts)
        }
    }

    suspend fun udpPing(count: Int = 20, onProgress: (SpeedProgress) -> Unit = {}): PingStats = withContext(Dispatchers.IO) {
        val socket = openUdp()
        val sentAt = AtomicLongArray(count)
        val rtts = arrayOfNulls<Double>(count)
        var received = 0
        withClosing(listOf(socket)) {
            val sender = launch(Dispatchers.IO) {
                for (seq in 0 until count) {
                    sentAt.set(seq, System.nanoTime())
                    val packet = udpPacket(UDP_ECHO_REQUEST, seq, 64)
                    socket.send(DatagramPacket(packet, packet.size))
                    delay(50)
                }
            }
            val buffer = ByteArray(2048)
            val packet = DatagramPacket(buffer, buffer.size)
            socket.soTimeout = 100
            var sendingDoneAt = 0L
            while (received < count) {
                if (sender.isCompleted) {
                    if (sendingDoneAt == 0L) sendingDoneAt = System.nanoTime()
                    if (System.nanoTime() - sendingDoneAt > 1_000_000_000L) break
                }
                if (!receive(socket, packet)) continue
                val now = System.nanoTime()
                if (packet.length < HEADER_UDP || buffer[5] != UDP_ECHO_REPLY || !sameToken(buffer)) continue
                val seq = ByteBuffer.wrap(buffer, 22, 4).int
                if (seq in 0 until count && rtts[seq] == null && sentAt.get(seq) != 0L) {
                    val rtt = (now - sentAt.get(seq)) / 1e6
                    rtts[seq] = rtt
                    received++
                    onProgress(SpeedProgress(SpeedPhase.UDP_PING, received.toFloat() / count, latencyMs = rtt))
                }
            }
            sender.cancel()
        }
        pingStats(count, rtts.filterNotNull())
    }

    suspend fun tcpDownload(seconds: Int, streams: Int, onProgress: (SpeedProgress) -> Unit = {}): ThroughputStats =
        withContext(Dispatchers.IO) {
            val durationMs = seconds.coerceIn(1, MAX_SECONDS) * 1000
            val sockets = (1..streams.coerceIn(1, 8)).map { openTcp(MODE_DOWNLOAD, durationMs) }
            val total = AtomicLong()
            val lastByte = AtomicLong()
            val started = System.nanoTime()
            withClosing(sockets) {
                val readers = sockets.map { socket ->
                    launch(Dispatchers.IO) {
                        val input = socket.getInputStream()
                        val buffer = ByteArray(64 * 1024)
                        while (true) {
                            val n = input.read(buffer)
                            if (n < 0) break
                            total.addAndGet(n.toLong())
                            lastByte.set(System.nanoTime())
                        }
                    }
                }
                val sampler = Sampler(started, durationMs, total, SpeedPhase.TCP_DOWNLOAD, onProgress)
                val samples = launch(Dispatchers.IO) { sampler.run { readers.all { it.isCompleted } } }
                readers.joinAll()
                samples.cancel()
                throughput(started, lastByte.get(), total.get(), sockets.size, sampler.warmup)
            }
        }

    suspend fun tcpUpload(seconds: Int, streams: Int, onProgress: (SpeedProgress) -> Unit = {}): ThroughputStats =
        withContext(Dispatchers.IO) {
            val durationMs = seconds.coerceIn(1, MAX_SECONDS) * 1000
            val sockets = (1..streams.coerceIn(1, 8)).map { openTcp(MODE_UPLOAD, durationMs) }
            val written = AtomicLong()
            val received = AtomicLong()
            val spanMicros = AtomicLong()
            val started = System.nanoTime()
            val deadline = started + durationMs * 1_000_000L
            withClosing(sockets) {
                val writers = sockets.map { socket ->
                    launch(Dispatchers.IO) {
                        val output = socket.getOutputStream()
                        while (System.nanoTime() < deadline) {
                            output.write(PAYLOAD)
                            written.addAndGet(PAYLOAD.size.toLong())
                        }
                        output.flush()
                        socket.shutdownOutput()
                        socket.soTimeout = 15_000
                        val result = ByteArray(16)
                        DataInputStream(socket.getInputStream()).readFully(result)
                        val buffer = ByteBuffer.wrap(result)
                        received.addAndGet(buffer.long)
                        val span = buffer.long
                        spanMicros.updateAndGet { max(it, span) }
                    }
                }
                val sampler = Sampler(started, durationMs, written, SpeedPhase.TCP_UPLOAD, onProgress)
                val samples = launch(Dispatchers.IO) { sampler.run { writers.all { it.isCompleted } } }
                writers.joinAll()
                samples.cancel()
            }
            val secondsMeasured = if (spanMicros.get() > 0) spanMicros.get() / 1e6 else durationMs / 1000.0
            ThroughputStats(received.get(), secondsMeasured, received.get() * 8 / secondsMeasured, sockets.size)
        }

    suspend fun udpDownload(seconds: Int, rateMbps: Int, onProgress: (SpeedProgress) -> Unit = {}): UdpStats = withContext(Dispatchers.IO) {
        val durationMs = seconds.coerceIn(1, MAX_SECONDS) * 1000
        val socket = openUdp()
        socket.receiveBufferSize = 4 shl 20
        var received = 0L
        var bytes = 0L
        var maxSeq = -1L
        var outOfOrder = 0L
        var sent = -1L
        var jitter = 0.0
        var prevTransit = Double.NaN
        var first = 0L
        var last = 0L
        withClosing(listOf(socket)) {
            val start = udpPacket(UDP_DOWNLOAD_START, 0, HEADER_UDP + 8)
            ByteBuffer.wrap(start, HEADER_UDP, 8).putInt(rateMbps.coerceIn(1, 1000) * 1000).putShort(PACKET_SIZE.toShort()).putShort(durationMs.toShort())
            socket.send(DatagramPacket(start, start.size))
            val keepalive = launch(Dispatchers.IO) {
                val packet = udpPacket(UDP_KEEPALIVE, 0, HEADER_UDP)
                while (isActive) {
                    delay(200)
                    socket.send(DatagramPacket(packet, packet.size))
                }
            }
            val buffer = ByteArray(2048)
            val packet = DatagramPacket(buffer, buffer.size)
            socket.soTimeout = 3_000
            val started = System.nanoTime()
            var lastProgress = started
            var bytesAtProgress = 0L
            var doneAt = 0L
            while (true) {
                if (!receive(socket, packet)) {
                    if (received == 0L) throw SpeedTestError("收不到 UDP 数据：UDP ${port} 端口可能没有放行，或运营商限制了 UDP")
                    break
                }
                val now = System.nanoTime()
                if (packet.length < HEADER_UDP || !sameToken(buffer)) continue
                when (buffer[5]) {
                    UDP_DOWNLOAD_DATA -> {
                        val seq = ByteBuffer.wrap(buffer, 22, 4).int.toLong() and 0xffffffffL
                        val sentMicros = ByteBuffer.wrap(buffer, 26, 8).long
                        if (first == 0L) first = now
                        last = now
                        received++
                        bytes += packet.length
                        if (seq < maxSeq) outOfOrder++ else maxSeq = seq
                        val transit = (now - started) / 1000.0 - sentMicros
                        if (!prevTransit.isNaN()) jitter += (abs(transit - prevTransit) - jitter) / 16
                        prevTransit = transit
                    }
                    UDP_DOWNLOAD_DONE -> {
                        sent = ByteBuffer.wrap(buffer, HEADER_UDP, 4).int.toLong() and 0xffffffffL
                        if (doneAt == 0L) doneAt = now
                        socket.soTimeout = 200
                    }
                }
                if (now - lastProgress > 250_000_000L) {
                    val rate = (bytes - bytesAtProgress) * 8 / ((now - lastProgress) / 1e9)
                    onProgress(SpeedProgress(SpeedPhase.UDP_DOWNLOAD, min(1f, (now - started) / 1e6f / durationMs), rate))
                    lastProgress = now
                    bytesAtProgress = bytes
                }
                if (doneAt != 0L && now - doneAt > 200_000_000L) break
            }
            keepalive.cancel()
        }
        if (sent < 0) sent = maxSeq + 1
        val secondsMeasured = if (last > first) (last - first) / 1e9 else durationMs / 1000.0
        UdpStats(rateMbps, sent, received, bytes, secondsMeasured, bytes * 8 / secondsMeasured, jitter / 1000, outOfOrder)
    }

    suspend fun udpUpload(seconds: Int, rateMbps: Int, onProgress: (SpeedProgress) -> Unit = {}): UdpStats = withContext(Dispatchers.IO) {
        val durationMs = seconds.coerceIn(1, MAX_SECONDS) * 1000
        val socket = openUdp()
        socket.sendBufferSize = 4 shl 20
        val perSecond = rateMbps.coerceIn(1, 1000) * 1_000_000.0 / 8 / PACKET_SIZE
        withClosing(listOf(socket)) {
            var sent = 0L
            val packet = udpPacket(UDP_UPLOAD_DATA, 0, PACKET_SIZE)
            val datagram = DatagramPacket(packet, packet.size)
            val started = System.nanoTime()
            val end = started + durationMs * 1_000_000L
            var lastProgress = started
            var sentAtProgress = 0L
            while (true) {
                val now = System.nanoTime()
                if (now >= end) break
                val due = ((now - started) / 1e9 * perSecond).toLong() - sent
                var burst = min(due, 256L)
                while (burst > 0) {
                    ByteBuffer.wrap(packet, 22, 12).putInt(sent.toInt()).putLong((System.nanoTime() - started) / 1000)
                    socket.send(datagram)
                    sent++
                    burst--
                }
                if (now - lastProgress > 250_000_000L) {
                    val rate = (sent - sentAtProgress) * PACKET_SIZE * 8 / ((now - lastProgress) / 1e9)
                    onProgress(SpeedProgress(SpeedPhase.UDP_UPLOAD, (now - started) / 1e6f / durationMs, rate))
                    lastProgress = now
                    sentAtProgress = sent
                }
                if (due < 64) Thread.sleep(1)
            }
            // The server counts what arrived; ask a few times in case a reply is lost.
            val report = udpPacket(UDP_UPLOAD_REPORT, 0, UPLOAD_RESULT_LEN)
            val buffer = ByteArray(2048)
            val reply = DatagramPacket(buffer, buffer.size)
            socket.soTimeout = 700
            repeat(5) {
                socket.send(DatagramPacket(report, report.size))
                if (receive(socket, reply) && reply.length >= UPLOAD_RESULT_LEN && buffer[5] == UDP_UPLOAD_RESULT && sameToken(buffer)) {
                    val body = ByteBuffer.wrap(buffer, HEADER_UDP, 32)
                    val packets = body.int.toLong() and 0xffffffffL
                    val bytes = body.long
                    body.int // highest sequence number
                    val outOfOrder = body.int.toLong() and 0xffffffffL
                    val jitterMicros = body.int.toLong() and 0xffffffffL
                    val spanMicros = body.long
                    val secondsMeasured = if (spanMicros > 0) spanMicros / 1e6 else durationMs / 1000.0
                    return@withClosing UdpStats(rateMbps, sent, packets, bytes, secondsMeasured, bytes * 8 / secondsMeasured, jitterMicros / 1000.0, outOfOrder)
                }
            }
            throw SpeedTestError("服务器没有返回 UDP 上传结果：UDP ${port} 端口可能没有放行，或运营商限制了 UDP")
        }
    }

    /** Waits for one datagram; false on timeout. A closed UDP port is an error. */
    private fun receive(socket: DatagramSocket, packet: DatagramPacket): Boolean = try {
        socket.receive(packet)
        true
    } catch (e: SocketTimeoutException) {
        false
    } catch (e: PortUnreachableException) {
        throw SpeedTestError("UDP ${port} 端口没有开放，请在防火墙和安全组放行 UDP ${port}")
    }

    private fun openTcp(mode: Byte, durationMs: Int): Socket {
        val socket = Socket()
        try {
            socket.tcpNoDelay = true
            socket.connect(InetSocketAddress(host, port), CONNECT_TIMEOUT)
            socket.soTimeout = durationMs + 10_000
            val header = ByteBuffer.allocate(HEADER_TCP)
            header.put(TCP_MAGIC).put(VERSION).put(mode).putShort(durationMs.toShort()).put(token)
            socket.getOutputStream().apply { write(header.array()); flush() }
            when (val status = socket.getInputStream().read()) {
                STATUS_OK -> return socket
                STATUS_BAD_TOKEN -> throw SpeedTestError("测速会话已过期，请重新开始")
                STATUS_BUSY -> throw SpeedTestError("服务器测速连接已满，请稍后再试")
                -1 -> throw SpeedTestError("测速端口没有响应，可能不是测速服务")
                else -> throw SpeedTestError("测速服务拒绝了请求（$status）")
            }
        } catch (e: Exception) {
            socket.close()
            throw e
        }
    }

    private fun openUdp(): DatagramSocket {
        val socket = DatagramSocket()
        socket.connect(InetSocketAddress(host, port))
        return socket
    }

    private fun udpPacket(kind: Byte, seq: Int, size: Int): ByteArray {
        val packet = ByteArray(size)
        val buffer = ByteBuffer.wrap(packet)
        buffer.put(UDP_MAGIC).put(VERSION).put(kind).put(token).putInt(seq).putLong(System.nanoTime() / 1000)
        if (size > HEADER_UDP) System.arraycopy(PAYLOAD, 0, packet, HEADER_UDP, size - HEADER_UDP)
        return packet
    }

    private fun sameToken(packet: ByteArray): Boolean {
        for (i in token.indices) if (packet[6 + i] != token[i]) return false
        return true
    }

    /** Reports the live rate every 250 ms and remembers the byte count after the warm-up. */
    private class Sampler(
        private val started: Long,
        private val durationMs: Int,
        private val counter: AtomicLong,
        private val phase: SpeedPhase,
        private val onProgress: (SpeedProgress) -> Unit,
    ) {
        @Volatile
        var warmup: Pair<Long, Long>? = null

        suspend fun run(finished: () -> Boolean) {
            var lastAt = started
            var lastBytes = 0L
            val warmupAt = started + warmupNanos(durationMs)
            while (!finished()) {
                delay(250)
                val now = System.nanoTime()
                val bytes = counter.get()
                if (now >= warmupAt && warmup == null) warmup = now to bytes
                onProgress(SpeedProgress(phase, min(1f, (now - started) / 1e6f / durationMs), (bytes - lastBytes) * 8 / ((now - lastAt) / 1e9)))
                lastAt = now
                lastBytes = bytes
            }
        }
    }

    private fun throughput(started: Long, lastByte: Long, total: Long, streams: Int, warmup: Pair<Long, Long>?): ThroughputStats {
        val end = if (lastByte > started) lastByte else System.nanoTime()
        // Leave TCP slow start out of the result when the test is long enough.
        if (warmup != null && end - warmup.first > 1_000_000_000L && total > warmup.second) {
            val seconds = (end - warmup.first) / 1e9
            return ThroughputStats(total, (end - started) / 1e9, (total - warmup.second) * 8 / seconds, streams)
        }
        val seconds = max((end - started) / 1e9, 0.001)
        return ThroughputStats(total, seconds, total * 8 / seconds, streams)
    }

    companion object {
        const val MAX_SECONDS = 15
        const val PACKET_SIZE = 1200
        private const val CONNECT_TIMEOUT = 5_000
        private val TCP_MAGIC = "1SST".toByteArray()
        private val UDP_MAGIC = "1SSU".toByteArray()
        private const val VERSION: Byte = 1
        private const val HEADER_TCP = 24
        private const val HEADER_UDP = 34
        private const val UPLOAD_RESULT_LEN = HEADER_UDP + 32
        private const val MODE_DOWNLOAD: Byte = 1
        private const val MODE_UPLOAD: Byte = 2
        private const val MODE_ECHO: Byte = 3
        private const val STATUS_OK = 0
        private const val STATUS_BAD_TOKEN = 1
        private const val STATUS_BUSY = 2
        private const val UDP_ECHO_REQUEST: Byte = 1
        private const val UDP_ECHO_REPLY: Byte = 2
        private const val UDP_UPLOAD_DATA: Byte = 3
        private const val UDP_UPLOAD_REPORT: Byte = 4
        private const val UDP_UPLOAD_RESULT: Byte = 5
        private const val UDP_DOWNLOAD_START: Byte = 6
        private const val UDP_DOWNLOAD_DATA: Byte = 7
        private const val UDP_KEEPALIVE: Byte = 8
        private const val UDP_DOWNLOAD_DONE: Byte = 9

        /** Random bytes, so compression on the path cannot flatter the result. */
        private val PAYLOAD = ByteArray(64 * 1024).also { SecureRandom().nextBytes(it) }

        private fun warmupNanos(durationMs: Int): Long = min(1_000L, durationMs / 4L) * 1_000_000L

        fun decodeToken(hex: String): ByteArray {
            require(hex.length == 32) { "invalid speed test token" }
            return ByteArray(16) { i -> hex.substring(i * 2, i * 2 + 2).toInt(16).toByte() }
        }

        /**
         * Finds the first address that accepts a connection on the test port.
         * The server only answers holders of the token, so trying several is harmless.
         */
        suspend fun connect(target: SpeedtestTarget, preferred: List<String> = emptyList()): SpeedTester = withContext(Dispatchers.IO) {
            val token = decodeToken(target.token)
            val hosts = (preferred + target.hosts).map { it.trim().trim('[', ']') }.filter { it.isNotEmpty() }.distinct()
            if (hosts.isEmpty()) throw SpeedTestError("不知道这台服务器的公网地址，请在面板里给它填写公网地址")
            for (host in hosts) {
                val reachable = runCatching {
                    Socket().use { it.connect(InetSocketAddress(host, target.port), 3_000) }
                }.isSuccess
                if (reachable) return@withContext SpeedTester(host, target.port, token)
            }
            throw SpeedTestError("连不上测速端口 ${target.port}（试过 ${hosts.joinToString("、")}）。请在防火墙和云服务商安全组放行 TCP 和 UDP ${target.port}")
        }

        private fun pingStats(sent: Int, rtts: List<Double>): PingStats {
            if (rtts.isEmpty()) return PingStats(sent, 0, 0.0, 0.0, 0.0, 0.0)
            val jitter = if (rtts.size > 1) rtts.zipWithNext { a, b -> abs(b - a) }.average() else 0.0
            return PingStats(sent, rtts.size, rtts.min(), rtts.average(), rtts.max(), jitter)
        }

        /** Closes the sockets when the test is cancelled, which unblocks reads. */
        private suspend fun <T> withClosing(sockets: List<Closeable>, block: suspend CoroutineScope.() -> T): T = coroutineScope {
            val closer = launch {
                try {
                    awaitCancellation()
                } finally {
                    sockets.forEach { runCatching { it.close() } }
                }
            }
            try {
                block()
            } finally {
                closer.cancel()
            }
        }
    }
}
