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
 * runWireGuardTest runs a WireGuard tunnel diagnostic.
 *
 * This implementation uses the gomobile-generated wglib.aar (built from wgmodule/).
 * If the native library has not been built yet, the test returns SKIPPED.
 *
 * To build wglib.aar:
 *   bash android/wgmodule-build/build.sh
 *
 * When the aar is present and the Gradle dependency is resolved, uncomment the
 * native implementation block below and remove the SKIPPED stub.
 */
suspend fun runWireGuardTest(
    @Suppress("UNUSED_PARAMETER") network: Network,
    sessionId: String,
    serverURL: String,
    token: String,
    addressFamily: AddressFamily,
): TestResult = withContext(Dispatchers.IO) {
    val start = System.currentTimeMillis()

    // --- Native integration (requires wglib.aar) ---
    // Uncomment when wglib.aar is built via android/wgmodule-build/build.sh:
    //
    // val result = suspendCancellableCoroutine<wgmodule.WireGuardResult?> { cont ->
    //     val callback = object : wgmodule.WireGuardCallback {
    //         override fun onResult(result: wgmodule.WireGuardResult?, errMsg: String?) {
    //             cont.resume(result)
    //         }
    //     }
    //     wgmodule.Wgmodule.runWireGuardTestAsync(serverURL, token, addressFamily.name.lowercase(), callback)
    //     cont.invokeOnCancellation { /* goroutine cannot be cancelled; it will complete naturally */ }
    // }
    // val latency = System.currentTimeMillis() - start
    // if (result == null) {
    //     return@withContext skippedResult(sessionId, addressFamily, latency, "native library error")
    // }
    // return@withContext when (result.status) {
    //     "pass" -> TestResult(
    //         id = UUID.randomUUID().toString(),
    //         sessionId = sessionId,
    //         testType = TestType.WIREGUARD,
    //         addressFamily = addressFamily,
    //         status = TestStatus.PASS,
    //         latencyMs = result.avgRTTMs.toDoubleOrNull()?.toLong(),
    //         transferRateKbps = result.rateKbps.toDoubleOrNull(),
    //         bytesSent = result.bytesSent.toLongOrNull(),
    //         bytesReceived = result.bytesReceived.toLongOrNull(),
    //         timestamp = System.currentTimeMillis(),
    //     )
    //     "skipped" -> skippedResult(sessionId, addressFamily, latency, result.failureReason)
    //     else -> failResult(sessionId, addressFamily, latency, result.failureReason)
    // }
    // --- End native integration ---

    // Placeholder: return SKIPPED until wglib.aar is built.
    val latency = System.currentTimeMillis() - start
    skippedResult(sessionId, addressFamily, latency, "wglib.aar not built — run android/wgmodule-build/build.sh")
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
