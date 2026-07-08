package selvakn.ipv6diag.diagnostic

import android.net.Network
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import selvakn.ipv6diag.data.model.AddressFamily
import selvakn.ipv6diag.data.model.TestType
import java.io.Closeable
import java.io.InputStream
import java.io.OutputStream
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket
import java.net.SocketTimeoutException
import java.nio.ByteBuffer
import java.security.MessageDigest
import java.security.SecureRandom
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import javax.net.ssl.SSLSocketFactory

/**
 * A real TURN (RFC 5766) client, used because a real TURN relay (pion/turn on the
 * server side) does not echo arbitrary bytes sent straight to its listener port —
 * it requires an authenticated Allocate + CreatePermission handshake before any
 * data is relayed. This mirrors what the CLI (pion/webrtc ICE) and browser client
 * (RTCPeerConnection) already do, minus the ICE negotiation: since both TURN
 * "peers" in this test run on the same device, ICE candidate exchange is
 * unnecessary — we just allocate two relay addresses directly and exchange data
 * between them through the server.
 */

private const val MAGIC_COOKIE = 0x2112A442

private const val METHOD_ALLOCATE = 0x0003
private const val METHOD_CREATE_PERMISSION = 0x0008
private const val METHOD_SEND_INDICATION = 0x0016
private const val METHOD_DATA_INDICATION = 0x0017

private const val TYPE_ALLOCATE_ERROR = 0x0113
private const val TYPE_ALLOCATE_SUCCESS = 0x0103
private const val TYPE_CREATE_PERMISSION_SUCCESS = 0x0108

private const val ATTR_USERNAME = 0x0006
private const val ATTR_MESSAGE_INTEGRITY = 0x0008
private const val ATTR_REALM = 0x0014
private const val ATTR_NONCE = 0x0015
private const val ATTR_XOR_RELAYED_ADDRESS = 0x0016
private const val ATTR_REQUESTED_TRANSPORT = 0x0019
private const val ATTR_XOR_PEER_ADDRESS = 0x0012
private const val ATTR_DATA = 0x0013

private val secureRandom = SecureRandom()

private data class StunMessage(val type: Int, val txId: ByteArray, val body: ByteArray)

private fun randomTxId(): ByteArray = ByteArray(12).also { secureRandom.nextBytes(it) }

private fun buildHeader(msgType: Int, bodyLen: Int, txId: ByteArray): ByteArray =
    ByteBuffer.allocate(20)
        .putShort(msgType.toShort())
        .putShort(bodyLen.toShort())
        .putInt(MAGIC_COOKIE)
        .put(txId)
        .array()

private fun buildAttr(type: Int, value: ByteArray): ByteArray {
    val pad = (4 - value.size % 4) % 4
    return ByteBuffer.allocate(4 + value.size + pad)
        .putShort(type.toShort())
        .putShort(value.size.toShort())
        .put(value)
        .array()
}

private fun longTermKey(username: String, realm: String, password: String): ByteArray =
    MessageDigest.getInstance("MD5").digest("$username:$realm:$password".toByteArray(Charsets.UTF_8))

private fun hmacSha1(key: ByteArray, data: ByteArray): ByteArray {
    val mac = Mac.getInstance("HmacSHA1")
    mac.init(SecretKeySpec(key, "HmacSHA1"))
    return mac.doFinal(data)
}

private fun buildMessage(msgType: Int, txId: ByteArray, attrs: List<ByteArray>, integrityKey: ByteArray? = null): ByteArray {
    var body = attrs.fold(ByteArray(0)) { acc, a -> acc + a }
    if (integrityKey != null) {
        // Per RFC 5389 15.4, MESSAGE-INTEGRITY's own (unpadded, 24-byte) size must be
        // included in the length field used to compute the HMAC, before it's appended.
        val header = buildHeader(msgType, body.size + 24, txId)
        val mac = hmacSha1(integrityKey, header + body)
        body += buildAttr(ATTR_MESSAGE_INTEGRITY, mac)
    }
    return buildHeader(msgType, body.size, txId) + body
}

private fun parseMessage(raw: ByteArray): StunMessage? {
    if (raw.size < 20) return null
    val type = ((raw[0].toInt() and 0xFF) shl 8) or (raw[1].toInt() and 0xFF)
    val len = ((raw[2].toInt() and 0xFF) shl 8) or (raw[3].toInt() and 0xFF)
    if (raw.size < 20 + len) return null
    val txId = raw.copyOfRange(8, 20)
    val body = raw.copyOfRange(20, 20 + len)
    return StunMessage(type, txId, body)
}

private fun parseAttrs(body: ByteArray): Map<Int, ByteArray> {
    val map = mutableMapOf<Int, ByteArray>()
    var off = 0
    while (off + 4 <= body.size) {
        val type = ((body[off].toInt() and 0xFF) shl 8) or (body[off + 1].toInt() and 0xFF)
        val len = ((body[off + 2].toInt() and 0xFF) shl 8) or (body[off + 3].toInt() and 0xFF)
        if (off + 4 + len > body.size) break
        map[type] = body.copyOfRange(off + 4, off + 4 + len)
        val pad = (4 - len % 4) % 4
        off += 4 + len + pad
    }
    return map
}

private fun addressCookie(txId: ByteArray, ipv6: Boolean): ByteArray =
    if (ipv6) ByteBuffer.allocate(16).putInt(MAGIC_COOKIE).put(txId).array()
    else ByteBuffer.allocate(4).putInt(MAGIC_COOKIE).array()

private fun buildXorAddressAttr(type: Int, address: InetAddress, port: Int, txId: ByteArray): ByteArray {
    val ipv6 = address is java.net.Inet6Address
    val cookie = addressCookie(txId, ipv6)
    val raw = address.address
    val xored = ByteArray(raw.size) { i -> (raw[i].toInt() xor cookie[i].toInt()).toByte() }
    val xport = (port xor (MAGIC_COOKIE ushr 16)) and 0xFFFF
    val value = ByteArray(4 + xored.size)
    value[1] = (if (ipv6) 0x02 else 0x01).toByte()
    value[2] = (xport ushr 8).toByte()
    value[3] = xport.toByte()
    xored.copyInto(value, 4)
    return buildAttr(type, value)
}

private fun parseXorAddress(value: ByteArray, txId: ByteArray): InetSocketAddress {
    val family = value[1].toInt() and 0xFF
    val xport = ((value[2].toInt() and 0xFF) shl 8) or (value[3].toInt() and 0xFF)
    val port = xport xor (MAGIC_COOKIE ushr 16)
    val ipv6 = family == 0x02
    val addrLen = if (ipv6) 16 else 4
    val cookie = addressCookie(txId, ipv6)
    val xored = value.copyOfRange(4, 4 + addrLen)
    val raw = ByteArray(addrLen) { i -> (xored[i].toInt() xor cookie[i].toInt()).toByte() }
    return InetSocketAddress(InetAddress.getByAddress(raw), port)
}

/** Transport-agnostic channel for exchanging framed STUN/TURN messages with the server. */
private interface TurnChannel : Closeable {
    fun send(msg: ByteArray)
    fun receiveMessage(timeoutMs: Int): ByteArray?
}

private class UdpTurnChannel(private val socket: DatagramSocket, private val serverAddr: InetSocketAddress) : TurnChannel {
    override fun send(msg: ByteArray) {
        socket.send(DatagramPacket(msg, msg.size, serverAddr))
    }
    override fun receiveMessage(timeoutMs: Int): ByteArray? {
        return try {
            socket.soTimeout = timeoutMs
            val buf = ByteArray(2048)
            val pkt = DatagramPacket(buf, buf.size)
            socket.receive(pkt)
            buf.copyOf(pkt.length)
        } catch (_: SocketTimeoutException) {
            null
        } catch (_: Exception) {
            null
        }
    }
    override fun close() = socket.close()
}

// STUN messages are self-delimiting via the header's length field, so on a stream
// (TCP/TLS) transport we just read exactly header-then-body — no extra framing needed.
private class StreamTurnChannel(private val socket: Socket) : TurnChannel {
    private val out: OutputStream = socket.getOutputStream()
    private val inp: InputStream = socket.getInputStream()

    override fun send(msg: ByteArray) {
        out.write(msg)
        out.flush()
    }

    override fun receiveMessage(timeoutMs: Int): ByteArray? {
        return try {
            socket.soTimeout = timeoutMs
            val header = readExact(20) ?: return null
            val len = ((header[2].toInt() and 0xFF) shl 8) or (header[3].toInt() and 0xFF)
            val body = if (len > 0) readExact(len) ?: return null else ByteArray(0)
            header + body
        } catch (_: SocketTimeoutException) {
            null
        } catch (_: Exception) {
            null
        }
    }

    private fun readExact(n: Int): ByteArray? {
        val buf = ByteArray(n)
        var off = 0
        while (off < n) {
            val r = inp.read(buf, off, n - off)
            if (r < 0) return null
            off += r
        }
        return buf
    }

    override fun close() = socket.close()
}

private data class TurnAllocation(
    val channel: TurnChannel,
    val relayedAddr: InetSocketAddress,
    val realm: String,
    val nonce: ByteArray,
    val key: ByteArray,
)

/** Performs the Allocate handshake: unauthenticated probe -> 401 challenge -> authenticated retry. */
private fun turnAllocate(channel: TurnChannel, username: String, password: String): TurnAllocation? {
    val reqTransport = buildAttr(ATTR_REQUESTED_TRANSPORT, byteArrayOf(17, 0, 0, 0))

    channel.send(buildMessage(METHOD_ALLOCATE, randomTxId(), listOf(reqTransport)))
    val challenge = channel.receiveMessage(3000)?.let { parseMessage(it) } ?: return null
    if (challenge.type != TYPE_ALLOCATE_ERROR) return null
    val challengeAttrs = parseAttrs(challenge.body)
    val realm = challengeAttrs[ATTR_REALM]?.toString(Charsets.UTF_8) ?: return null
    val nonce = challengeAttrs[ATTR_NONCE] ?: return null
    val key = longTermKey(username, realm, password)

    val txId = randomTxId()
    val authedAttrs = listOf(
        reqTransport,
        buildAttr(ATTR_USERNAME, username.toByteArray(Charsets.UTF_8)),
        buildAttr(ATTR_REALM, realm.toByteArray(Charsets.UTF_8)),
        buildAttr(ATTR_NONCE, nonce),
    )
    channel.send(buildMessage(METHOD_ALLOCATE, txId, authedAttrs, integrityKey = key))
    val success = channel.receiveMessage(3000)?.let { parseMessage(it) } ?: return null
    if (success.type != TYPE_ALLOCATE_SUCCESS) return null
    val successAttrs = parseAttrs(success.body)
    val relayedValue = successAttrs[ATTR_XOR_RELAYED_ADDRESS] ?: return null
    val relayed = parseXorAddress(relayedValue, success.txId)
    return TurnAllocation(channel, relayed, realm, nonce, key)
}

private fun turnCreatePermission(allocation: TurnAllocation, username: String, peerAddress: InetAddress): Boolean {
    val txId = randomTxId()
    val attrs = listOf(
        buildXorAddressAttr(ATTR_XOR_PEER_ADDRESS, peerAddress, 0, txId),
        buildAttr(ATTR_USERNAME, username.toByteArray(Charsets.UTF_8)),
        buildAttr(ATTR_REALM, allocation.realm.toByteArray(Charsets.UTF_8)),
        buildAttr(ATTR_NONCE, allocation.nonce),
    )
    allocation.channel.send(buildMessage(METHOD_CREATE_PERMISSION, txId, attrs, integrityKey = allocation.key))
    val response = allocation.channel.receiveMessage(3000)?.let { parseMessage(it) } ?: return false
    return response.type == TYPE_CREATE_PERMISSION_SUCCESS
}

private fun turnSendIndication(channel: TurnChannel, peer: InetSocketAddress, payload: ByteArray) {
    val txId = randomTxId()
    val attrs = listOf(
        buildXorAddressAttr(ATTR_XOR_PEER_ADDRESS, peer.address, peer.port, txId),
        buildAttr(ATTR_DATA, payload),
    )
    channel.send(buildMessage(METHOD_SEND_INDICATION, txId, attrs))
}

private fun turnReceiveData(channel: TurnChannel, timeoutMs: Int): ByteArray? {
    val raw = channel.receiveMessage(timeoutMs) ?: return null
    val msg = parseMessage(raw) ?: return null
    if (msg.type != METHOD_DATA_INDICATION) return null
    return parseAttrs(msg.body)[ATTR_DATA]
}

private fun buildTurnPayload(size: Int, seq: Int): ByteArray {
    val buf = ByteArray(size.coerceAtLeast(8))
    buf[0] = (seq ushr 24).toByte()
    buf[1] = (seq ushr 16).toByte()
    buf[2] = (seq ushr 8).toByte()
    buf[3] = seq.toByte()
    for (i in 4 until buf.size) buf[i] = ((i % 251) + 1).toByte()
    return buf
}

private fun readTurnSeq(data: ByteArray): Int? {
    if (data.size < 4) return null
    return ((data[0].toInt() and 0xFF) shl 24) or ((data[1].toInt() and 0xFF) shl 16) or
        ((data[2].toInt() and 0xFF) shl 8) or (data[3].toInt() and 0xFF)
}

enum class TurnTransportKind { UDP, TCP, TLS }

/**
 * Runs a real TURN relay transfer test: two independent client allocations (A, B) on
 * this device each Allocate + CreatePermission the other's relayed address, then A
 * streams payloads to B (via Send Indication -> server relay -> Data Indication) while
 * B echoes everything straight back, mirroring the two-peer-connection design the CLI
 * and browser clients use for their TURN tests.
 */
suspend fun runTurnTestReal(
    network: Network,
    sessionId: String,
    targetIp: String,
    targetPort: Int,
    addressFamily: AddressFamily,
    transferWindowSeconds: Int,
    payloadSizeBytes: Int,
    messagesPerSecond: Int,
    qualityThresholdRatio: Float,
    credentials: TurnCredentials,
    kind: TurnTransportKind,
): selvakn.ipv6diag.data.model.TestResult = withContext(Dispatchers.IO) {
    val startedAt = System.currentTimeMillis()
    val serverAddr = InetSocketAddress(InetAddress.getByName(targetIp), targetPort)

    fun openChannel(): Pair<TurnChannel, Closeable> = when (kind) {
        TurnTransportKind.UDP -> {
            val socket = DatagramSocket()
            network.bindSocket(socket)
            UdpTurnChannel(socket, serverAddr) to socket
        }
        TurnTransportKind.TCP, TurnTransportKind.TLS -> {
            val raw: Socket = if (kind == TurnTransportKind.TLS) SSLSocketFactory.getDefault().createSocket() else Socket()
            network.bindSocket(raw)
            raw.connect(serverAddr, 5000)
            StreamTurnChannel(raw) to raw
        }
    }

    var closeableA: Closeable? = null
    var closeableB: Closeable? = null
    try {
        val (channelA, rawA) = try {
            openChannel()
        } catch (e: Exception) {
            return@withContext failResultTurn(sessionId, addressFamily, 0, targetIp, "failed to open TURN client socket: ${e.message}")
        }
        closeableA = rawA
        val (channelB, rawB) = try {
            openChannel()
        } catch (e: Exception) {
            return@withContext failResultTurn(sessionId, addressFamily, 0, targetIp, "failed to open TURN client socket: ${e.message}")
        }
        closeableB = rawB

        val allocA = turnAllocate(channelA, credentials.username, credentials.password)
        val allocB = turnAllocate(channelB, credentials.username, credentials.password)
        if (allocA == null || allocB == null) {
            return@withContext failResultTurn(sessionId, addressFamily, 0, targetIp, "TURN Allocate handshake failed")
        }

        val permOkA = turnCreatePermission(allocA, credentials.username, allocB.relayedAddr.address)
        val permOkB = turnCreatePermission(allocB, credentials.username, allocA.relayedAddr.address)
        if (!permOkA || !permOkB) {
            return@withContext failResultTurn(sessionId, addressFamily, 0, targetIp, "TURN CreatePermission failed")
        }

        val durationMs = transferWindowSeconds.coerceAtLeast(1) * 1000L
        val cadenceMs = (1000L / messagesPerSecond.coerceAtLeast(1)).coerceAtLeast(5L)
        val payloadSize = payloadSizeBytes.coerceAtLeast(64)

        val bytesSent = AtomicLong()
        val bytesReceived = AtomicLong()
        val sentPackets = AtomicLong()
        val recvPackets = AtomicLong()
        val rttSamples = AtomicLong()
        val totalRttMs = AtomicLong()
        val pendingSends = ConcurrentHashMap<Int, Long>()

        coroutineScope {
            val endTime = System.currentTimeMillis() + durationMs + 1000
            // B: echo every received payload straight back to A's relay address.
            launch(Dispatchers.IO) {
                while (System.currentTimeMillis() < endTime) {
                    val data = turnReceiveData(channelB, 500) ?: continue
                    runCatching { turnSendIndication(channelB, allocA.relayedAddr, data) }
                }
            }
            // A: collect echoes, track quality + RTT.
            launch(Dispatchers.IO) {
                while (System.currentTimeMillis() < endTime) {
                    val data = turnReceiveData(channelA, 500) ?: continue
                    readTurnSeq(data)?.let { seq ->
                        pendingSends.remove(seq)?.let { sentAt ->
                            totalRttMs.addAndGet(System.currentTimeMillis() - sentAt)
                            rttSamples.incrementAndGet()
                        }
                    }
                    bytesReceived.addAndGet(data.size.toLong())
                    recvPackets.incrementAndGet()
                }
            }
            // A: send loop, drives the transfer window.
            var seq = 0
            val sendEnd = System.currentTimeMillis() + durationMs
            while (System.currentTimeMillis() < sendEnd) {
                val iterStart = System.currentTimeMillis()
                val id = seq++
                pendingSends[id] = iterStart
                val payload = buildTurnPayload(payloadSize, id)
                runCatching { turnSendIndication(channelA, allocB.relayedAddr, payload) }
                bytesSent.addAndGet(payload.size.toLong())
                sentPackets.incrementAndGet()
                val remaining = cadenceMs - (System.currentTimeMillis() - iterStart)
                if (remaining > 0) kotlinx.coroutines.delay(remaining)
            }
        }

        val elapsed = (System.currentTimeMillis() - startedAt).coerceAtLeast(1)
        val sent = sentPackets.get()
        val received = recvPackets.get()
        val deliveryRatio = if (sent > 0) received.toFloat() / sent.toFloat() else 0f
        val rtt = if (rttSamples.get() > 0) totalRttMs.get() / rttSamples.get() else elapsed
        val transferRate = (bytesReceived.get() * 8.0) / (elapsed / 1000.0) / 1000.0
        val qualityOk = deliveryRatio >= qualityThresholdRatio
        val windowOk = elapsed >= (durationMs - 500)
        val base = if (qualityOk && windowOk) {
            passResultTurn(sessionId, addressFamily, rtt, targetIp)
        } else {
            failResultTurn(sessionId, addressFamily, rtt, targetIp,
                if (!windowOk) "transfer window incomplete" else "delivery quality below threshold")
        }
        base.copy(
            transferRateKbps = transferRate,
            bytesSent = bytesSent.get(),
            bytesReceived = bytesReceived.get(),
            deliveryQualityRatio = deliveryRatio,
            qualityThresholdRatio = qualityThresholdRatio,
            transferWindowSeconds = transferWindowSeconds,
            payloadProfile = "${payloadSize}B@${messagesPerSecond.coerceAtLeast(1)}Hz",
        )
    } finally {
        runCatching { closeableA?.close() }
        runCatching { closeableB?.close() }
    }
}

private fun passResultTurn(sessionId: String, family: AddressFamily, latencyMs: Long, resolvedAddress: String) =
    selvakn.ipv6diag.data.model.TestResult(
        id = java.util.UUID.randomUUID().toString(),
        sessionId = sessionId,
        testType = TestType.TURN,
        addressFamily = family,
        status = selvakn.ipv6diag.data.model.TestStatus.PASS,
        latencyMs = latencyMs,
        resolvedAddress = resolvedAddress,
    )

private fun failResultTurn(sessionId: String, family: AddressFamily, latencyMs: Long, resolvedAddress: String, reason: String) =
    selvakn.ipv6diag.data.model.TestResult(
        id = java.util.UUID.randomUUID().toString(),
        sessionId = sessionId,
        testType = TestType.TURN,
        addressFamily = family,
        status = selvakn.ipv6diag.data.model.TestStatus.FAIL,
        latencyMs = latencyMs,
        resolvedAddress = resolvedAddress,
        failureReason = reason,
    )
