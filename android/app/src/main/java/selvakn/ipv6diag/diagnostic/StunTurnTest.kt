package selvakn.ipv6diag.diagnostic

import android.net.Network
import selvakn.ipv6diag.data.model.AddressFamily
import selvakn.ipv6diag.data.model.TestResult
import selvakn.ipv6diag.data.model.TestStatus
import selvakn.ipv6diag.data.model.TestType
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetSocketAddress
import java.net.PortUnreachableException
import java.net.SocketTimeoutException
import java.security.SecureRandom
import java.util.UUID

private const val STUN_MAGIC_COOKIE = 0x2112A442
private const val UDP_TIMEOUT_MS = 3000

private val random = SecureRandom()

suspend fun runStunTest(
    network: Network,
    sessionId: String,
    targetIp: String,
    targetPort: Int,
    addressFamily: AddressFamily,
): TestResult = withContext(Dispatchers.IO) {
    val start = System.currentTimeMillis()
    val transactionId = ByteArray(12).also { random.nextBytes(it) }
    val request = buildStunHeader(messageType = 0x0001, body = ByteArray(0), transactionId = transactionId)

    val response = runUdpProbe(network, targetIp, targetPort, request)
    val latency = System.currentTimeMillis() - start

    when (response) {
        is ProbeResponse.Unsupported -> unsupportedResult(sessionId, TestType.STUN, addressFamily, latency, targetIp)
        is ProbeResponse.Error -> failResult(sessionId, TestType.STUN, addressFamily, latency, targetIp, response.reason)
        is ProbeResponse.Data -> {
            val data = response.bytes
            if (!isValidStunEnvelope(data, transactionId)) {
                return@withContext failResult(sessionId, TestType.STUN, addressFamily, latency, targetIp, "invalid STUN response")
            }
            val messageType = readU16(data, 0)
            if (messageType == 0x0101 || messageType == 0x0111) {
                val iceCandidates = buildList {
                    response.localCandidate?.let { add("host $it") }
                    parseXorMappedAddress(data)?.let { add("srflx $it") }
                }
                return@withContext passResult(sessionId, TestType.STUN, addressFamily, latency, targetIp, iceCandidates)
            }
            failResult(sessionId, TestType.STUN, addressFamily, latency, targetIp, "unexpected STUN type 0x${messageType.toString(16)}")
        }
    }
}

// transport values: "udp" (default), "tcp", "tls", "dtls"
// Port convention: 3478 for udp/tcp, 5349 for tls/dtls.
//
// This performs a real TURN (RFC 5766) Allocate + CreatePermission + Send/Data-Indication
// relay test (see TurnProtocol.kt) — a real TURN server does not echo raw bytes sent
// straight to its listener port, so `credentials` (fetched from /turn/credentials) is
// required. If null, TURN is unsupported/disabled on the server and the test is skipped.
suspend fun runTurnTest(
    network: Network,
    sessionId: String,
    targetIp: String,
    targetPort: Int,
    addressFamily: AddressFamily,
    transferWindowSeconds: Int,
    payloadSizeBytes: Int,
    messagesPerSecond: Int,
    qualityThresholdRatio: Float,
    transport: String = "udp",
    credentials: TurnCredentials?,
): TestResult {
    if (credentials == null) {
        return unsupportedResult(sessionId, TestType.TURN, addressFamily, 0, targetIp)
    }
    return when (transport.lowercase()) {
        "tcp" -> runTurnTestReal(network, sessionId, targetIp, targetPort, addressFamily, transferWindowSeconds, payloadSizeBytes, messagesPerSecond, qualityThresholdRatio, credentials, TurnTransportKind.TCP)
        "tls" -> runTurnTestReal(network, sessionId, targetIp, targetPort, addressFamily, transferWindowSeconds, payloadSizeBytes, messagesPerSecond, qualityThresholdRatio, credentials, TurnTransportKind.TLS)
        "dtls" -> failResult(sessionId, TestType.TURN, addressFamily, 0, targetIp, "DTLS/UDP (TURNS) not supported by the Android client")
        else -> runTurnTestReal(network, sessionId, targetIp, targetPort, addressFamily, transferWindowSeconds, payloadSizeBytes, messagesPerSecond, qualityThresholdRatio, credentials, TurnTransportKind.UDP)
    }
}

private fun passResult(
    sessionId: String,
    testType: TestType,
    family: AddressFamily,
    latencyMs: Long,
    resolvedAddress: String,
    iceCandidates: List<String> = emptyList(),
): TestResult = TestResult(
    id = UUID.randomUUID().toString(),
    sessionId = sessionId,
    testType = testType,
    addressFamily = family,
    status = TestStatus.PASS,
    latencyMs = latencyMs,
    resolvedAddress = resolvedAddress,
    iceCandidates = iceCandidates,
)

private fun failResult(
    sessionId: String,
    testType: TestType,
    family: AddressFamily,
    latencyMs: Long,
    resolvedAddress: String,
    reason: String,
): TestResult = TestResult(
    id = UUID.randomUUID().toString(),
    sessionId = sessionId,
    testType = testType,
    addressFamily = family,
    status = TestStatus.FAIL,
    latencyMs = latencyMs,
    resolvedAddress = resolvedAddress,
    failureReason = reason,
)

private fun unsupportedResult(
    sessionId: String,
    testType: TestType,
    family: AddressFamily,
    latencyMs: Long,
    resolvedAddress: String?,
): TestResult = TestResult(
    id = UUID.randomUUID().toString(),
    sessionId = sessionId,
    testType = testType,
    addressFamily = family,
    status = TestStatus.SKIPPED,
    latencyMs = latencyMs,
    resolvedAddress = resolvedAddress,
    failureReason = "server unsupported",
)
 

private sealed class ProbeResponse {
    data class Data(val bytes: ByteArray, val localCandidate: String?) : ProbeResponse()
    data class Error(val reason: String) : ProbeResponse()
    object Unsupported : ProbeResponse()
}

private fun runUdpProbe(network: Network, targetIp: String, port: Int, payload: ByteArray): ProbeResponse {
    val socket = DatagramSocket()
    return try {
        network.bindSocket(socket)
        socket.soTimeout = UDP_TIMEOUT_MS
        socket.connect(InetSocketAddress(targetIp, port))
        val localCandidate = runCatching { "${socket.localAddress.hostAddress}:${socket.localPort}" }.getOrNull()

        socket.send(DatagramPacket(payload, payload.size))
        val responseBuf = ByteArray(1024)
        val responsePacket = DatagramPacket(responseBuf, responseBuf.size)
        socket.receive(responsePacket)
        ProbeResponse.Data(responseBuf.copyOf(responsePacket.length), localCandidate)
    } catch (_: SocketTimeoutException) {
        ProbeResponse.Unsupported
    } catch (_: PortUnreachableException) {
        ProbeResponse.Unsupported
    } catch (e: Exception) {
        val msg = e.message ?: "udp probe failed"
        if (msg.contains("Network is unreachable", ignoreCase = true) ||
            msg.contains("No route to host", ignoreCase = true) ||
            msg.contains("Connection refused", ignoreCase = true)) {
            ProbeResponse.Unsupported
        } else {
            ProbeResponse.Error(msg)
        }
    } finally {
        socket.close()
    }
}

private fun buildStunHeader(messageType: Int, body: ByteArray, transactionId: ByteArray): ByteArray {
    val out = ByteArray(20 + body.size)
    writeU16(out, 0, messageType)
    writeU16(out, 2, body.size)
    writeU32(out, 4, STUN_MAGIC_COOKIE)
    System.arraycopy(transactionId, 0, out, 8, 12)
    System.arraycopy(body, 0, out, 20, body.size)
    return out
}

private fun isValidStunEnvelope(data: ByteArray, txId: ByteArray): Boolean {
    if (data.size < 20) return false
    if (readU32(data, 4) != STUN_MAGIC_COOKIE) return false
    for (i in 0 until 12) {
        if (data[8 + i] != txId[i]) return false
    }
    return true
}

private fun parseXorMappedAddress(data: ByteArray): String? {
    if (data.size < 20) return null
    val bodyLen = readU16(data, 2)
    var offset = 20
    val end = (20 + bodyLen).coerceAtMost(data.size)
    while (offset + 4 <= end) {
        val attrType = readU16(data, offset)
        val attrLen = readU16(data, offset + 2)
        val valueStart = offset + 4
        val valueEnd = valueStart + attrLen
        if (valueEnd > data.size) break
        if (attrType == 0x0020 && attrLen >= 8) {
            val family = data[valueStart + 1].toInt() and 0xFF
            val xPort = readU16(data, valueStart + 2)
            val port = xPort xor (STUN_MAGIC_COOKIE ushr 16)
            if (family == 0x01 && attrLen >= 8) {
                val cookie = ByteArray(4)
                writeU32(cookie, 0, STUN_MAGIC_COOKIE)
                val ipBytes = ByteArray(4)
                for (i in 0 until 4) {
                    ipBytes[i] = (data[valueStart + 4 + i].toInt() xor (cookie[i].toInt() and 0xFF)).toByte()
                }
                return "${(ipBytes[0].toInt() and 0xFF)}.${(ipBytes[1].toInt() and 0xFF)}.${(ipBytes[2].toInt() and 0xFF)}.${(ipBytes[3].toInt() and 0xFF)}:$port"
            }
            if (family == 0x02 && attrLen >= 20) {
                val xorPad = ByteArray(16)
                writeU32(xorPad, 0, STUN_MAGIC_COOKIE)
                for (i in 0 until 12) {
                    xorPad[4 + i] = data[8 + i] // transaction ID from header
                }
                val ipWords = IntArray(8)
                for (i in 0 until 16) {
                    val b = (data[valueStart + 4 + i].toInt() xor (xorPad[i].toInt() and 0xFF)) and 0xFF
                    val wordIdx = i / 2
                    ipWords[wordIdx] = (ipWords[wordIdx] shl 8) or b
                }
                val ipv6 = ipWords.joinToString(":") { it.toString(16) }
                return "[$ipv6]:$port"
            }
        }
        val paddedLen = (attrLen + 3) and 0xFFFC
        offset = valueStart + paddedLen
    }
    return null
}

private fun readU16(bytes: ByteArray, offset: Int): Int =
    ((bytes[offset].toInt() and 0xFF) shl 8) or
        (bytes[offset + 1].toInt() and 0xFF)

private fun readU32(bytes: ByteArray, offset: Int): Int =
    ((bytes[offset].toInt() and 0xFF) shl 24) or
        ((bytes[offset + 1].toInt() and 0xFF) shl 16) or
        ((bytes[offset + 2].toInt() and 0xFF) shl 8) or
        (bytes[offset + 3].toInt() and 0xFF)

private fun writeU16(bytes: ByteArray, offset: Int, value: Int) {
    bytes[offset] = ((value ushr 8) and 0xFF).toByte()
    bytes[offset + 1] = (value and 0xFF).toByte()
}

private fun writeU32(bytes: ByteArray, offset: Int, value: Int) {
    bytes[offset] = ((value ushr 24) and 0xFF).toByte()
    bytes[offset + 1] = ((value ushr 16) and 0xFF).toByte()
    bytes[offset + 2] = ((value ushr 8) and 0xFF).toByte()
    bytes[offset + 3] = (value and 0xFF).toByte()
}
