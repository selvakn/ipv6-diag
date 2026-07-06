package selvakn.ipv6diag.diagnostic

import android.net.Network
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.suspendCancellableCoroutine
import kotlinx.coroutines.withContext
import selvakn.ipv6diag.data.model.AddressFamily
import selvakn.ipv6diag.data.model.TestResult
import selvakn.ipv6diag.data.model.TestStatus
import selvakn.ipv6diag.data.model.TestType
import java.util.UUID
import kotlin.coroutines.resume

/**
 * Runs a WireGuard tunnel diagnostic via the gomobile-generated wglib.aar.
 *
 * The Go library manages its own virtual network interface (wireguard-go + netstack),
 * so [network] is not used for routing — the tunnel bypasses Android's network stack.
 * Build wglib.aar with `make wglib-build` before assembling the APK.
 */
suspend fun runWireGuardTest(
    @Suppress("UNUSED_PARAMETER") network: Network,
    sessionId: String,
    serverURL: String,
    token: String,
    addressFamily: AddressFamily,
): TestResult = withContext(Dispatchers.IO) {
    val start = System.currentTimeMillis()

    val result = suspendCancellableCoroutine<wgmodule.WireGuardResult?> { cont ->
        val callback = object : wgmodule.WireGuardCallback {
            override fun onResult(result: wgmodule.WireGuardResult?, errMsg: String?) {
                cont.resume(result)
            }
        }
        wgmodule.Wgmodule.runWireGuardTestAsync(serverURL, token, addressFamily.name.lowercase(), callback)
        cont.invokeOnCancellation { /* goroutine cannot be cancelled; it will complete naturally */ }
    }
    val latency = System.currentTimeMillis() - start
    if (result == null) {
        return@withContext skippedResult(sessionId, addressFamily, latency, "native library error")
    }
    return@withContext when (result.status) {
        "pass" -> TestResult(
            id = UUID.randomUUID().toString(),
            sessionId = sessionId,
            testType = TestType.WIREGUARD,
            addressFamily = addressFamily,
            status = TestStatus.PASS,
            latencyMs = result.avgRTTMs.toDoubleOrNull()?.toLong(),
            transferRateKbps = result.rateKbps.toDoubleOrNull(),
            bytesSent = result.bytesSent.toLongOrNull(),
            bytesReceived = result.bytesReceived.toLongOrNull(),
            timestamp = System.currentTimeMillis(),
        )
        "skipped" -> skippedResult(sessionId, addressFamily, latency, result.failureReason)
        else -> failResult(sessionId, addressFamily, latency, result.failureReason)
    }
}

private fun skippedResult(sessionId: String, family: AddressFamily, latencyMs: Long, reason: String) = TestResult(
    id = UUID.randomUUID().toString(),
    sessionId = sessionId,
    testType = TestType.WIREGUARD,
    addressFamily = family,
    status = TestStatus.SKIPPED,
    latencyMs = latencyMs,
    failureReason = reason,
)

private fun failResult(sessionId: String, family: AddressFamily, latencyMs: Long, reason: String) = TestResult(
    id = UUID.randomUUID().toString(),
    sessionId = sessionId,
    testType = TestType.WIREGUARD,
    addressFamily = family,
    status = TestStatus.FAIL,
    latencyMs = latencyMs,
    failureReason = reason,
)
