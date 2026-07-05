package selvakn.ipv6diag.diagnostic

import android.content.Context
import android.content.SharedPreferences
import android.net.ConnectivityManager
import android.net.Network
import android.net.NetworkCapabilities
import android.net.NetworkRequest
import selvakn.ipv6diag.R
import selvakn.ipv6diag.data.model.AddressFamily
import selvakn.ipv6diag.data.model.DiagnosticSession
import selvakn.ipv6diag.data.model.ServerEndpoint
import selvakn.ipv6diag.data.model.SessionStatus
import selvakn.ipv6diag.data.model.TestResult
import selvakn.ipv6diag.data.model.TestStatus
import selvakn.ipv6diag.data.model.TestType
import selvakn.ipv6diag.data.repository.SessionRepository
import selvakn.ipv6diag.data.model.XlatDiagnosticSummary
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.withContext
import java.net.InetAddress
import java.util.UUID
import java.util.concurrent.atomic.AtomicBoolean

enum class TestFilter { ALL, HTTP_HTTPS, ICMP, DNS, STUN_TURN, XLAT_464 }

class DiagnosticRunner(
    private val context: Context,
    private val repository: SessionRepository,
    private val networkInfoCollector: NetworkInfoCollector,
) {
    private val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager

    suspend fun runTests(
        endpoint: ServerEndpoint,
        filter: TestFilter = TestFilter.ALL,
    ): DiagnosticSession {
        val binder = CellularNetworkBinder(context)
        return binder.withCellularNetwork { network ->
            executeTests(network, endpoint, filter)
        }
    }

    private suspend fun executeTests(
        network: Network,
        endpoint: ServerEndpoint,
        filter: TestFilter,
    ): DiagnosticSession = coroutineScope {
        val (targetHost, customPort) = parseHostAndPort(endpoint.hostname)
        val turnTransport = context.getSharedPreferences("ipv6diag_prefs", Context.MODE_PRIVATE)
            .getString("turn_transport", "udp") ?: "udp"
        val encryptedPort = if (customPort != null && customPort != 3478) customPort else 5349
        val plainPort = customPort ?: 3478
        val stunTurnPort = when (turnTransport) {
            "tls", "dtls" -> encryptedPort
            else -> plainPort
        }
        val transferWindowSeconds = context.resources.getInteger(R.integer.turn_transfer_window_seconds)
        val transferPayloadBytes = context.resources.getInteger(R.integer.turn_transfer_payload_size_bytes)
        val transferMessagesPerSecond = context.resources.getInteger(R.integer.turn_transfer_messages_per_second)
        val transferQualityThreshold = context.resources
            .getInteger(R.integer.turn_transfer_quality_threshold_percent) / 100f
        val sessionId = UUID.randomUUID().toString()
        val networkInfo = networkInfoCollector.collect(network)
        val networkChanged = AtomicBoolean(false)
        val allResults = mutableListOf<TestResult>()

        // Register network change detector
        val changeCallback = object : ConnectivityManager.NetworkCallback() {
            override fun onLost(lost: Network) {
                if (lost == network) networkChanged.set(true)
            }
        }
        cm.registerNetworkCallback(
            NetworkRequest.Builder()
                .addTransportType(NetworkCapabilities.TRANSPORT_CELLULAR)
                .build(),
            changeCallback
        )

        try {
            // Resolve server addresses using system resolver
            val ipv4Addr = resolveAddress(targetHost, isIPv6 = false)
            val ipv6Addr = resolveAddress(targetHost, isIPv6 = true)

            // Run selected test types. Under ALL, run 464XLAT diagnostics whenever IPv6 is
            // present — NAT64/DNS64/CLAT only apply on IPv6 networks, and CLAT detection is
            // best-effort, so we must not gate the whole probe on clatPresent alone.
            val runXlat = filter == TestFilter.XLAT_464 ||
                (filter == TestFilter.ALL && (networkInfo.clatPresent || networkInfo.hasNativeIPv6))
            val baseTypes = when (filter) {
                TestFilter.ALL -> listOf(TestType.HTTP, TestType.HTTPS, TestType.ICMP, TestType.DNS, TestType.STUN, TestType.TURN)
                TestFilter.HTTP_HTTPS -> listOf(TestType.HTTP, TestType.HTTPS)
                TestFilter.ICMP -> listOf(TestType.ICMP)
                TestFilter.DNS -> listOf(TestType.DNS)
                TestFilter.STUN_TURN -> listOf(TestType.STUN, TestType.TURN)
                TestFilter.XLAT_464 -> emptyList()
            }
            val testTypes = baseTypes

            for (testType in testTypes) {
                if (networkChanged.get()) break

                val results = when (testType) {
                    TestType.HTTP -> buildList {
                        if (ipv4Addr != null) add(runHttpTest(network, sessionId, ipv4Addr, endpoint.httpPort, AddressFamily.IPv4))
                        if (ipv6Addr != null) add(runHttpTest(network, sessionId, ipv6Addr, endpoint.httpPort, AddressFamily.IPv6))
                    }
                    TestType.HTTPS -> buildList {
                        if (ipv4Addr != null) add(runHttpsTest(network, sessionId, targetHost, ipv4Addr, endpoint.httpsPort, AddressFamily.IPv4))
                        if (ipv6Addr != null) add(runHttpsTest(network, sessionId, targetHost, ipv6Addr, endpoint.httpsPort, AddressFamily.IPv6))
                    }
                    TestType.ICMP -> buildList {
                        if (ipv4Addr != null) add(runIcmpTest(sessionId, ipv4Addr, AddressFamily.IPv4))
                        if (ipv6Addr != null) add(runIcmpTest(sessionId, ipv6Addr, AddressFamily.IPv6))
                    }
                    TestType.DNS -> runDnsTests(context, network, sessionId, targetHost)
                    TestType.STUN -> buildList {
                        if (ipv4Addr != null) add(runStunTest(network, sessionId, ipv4Addr, stunTurnPort, AddressFamily.IPv4))
                        if (ipv6Addr != null) add(runStunTest(network, sessionId, ipv6Addr, stunTurnPort, AddressFamily.IPv6))
                        if (isEmpty()) {
                            add(
                                TestResult(
                                    id = UUID.randomUUID().toString(),
                                    sessionId = sessionId,
                                    testType = TestType.STUN,
                                    addressFamily = AddressFamily.IPv4,
                                    status = TestStatus.SKIPPED,
                                    failureReason = "server unsupported",
                                )
                            )
                        }
                    }
                    TestType.TURN -> buildList {
                        if (ipv4Addr != null) add(
                            runTurnTest(
                                network = network,
                                sessionId = sessionId,
                                targetIp = ipv4Addr,
                                targetPort = stunTurnPort,
                                addressFamily = AddressFamily.IPv4,
                                transferWindowSeconds = transferWindowSeconds,
                                payloadSizeBytes = transferPayloadBytes,
                                messagesPerSecond = transferMessagesPerSecond,
                                qualityThresholdRatio = transferQualityThreshold,
                                transport = turnTransport,
                            )
                        )
                        if (ipv6Addr != null) add(
                            runTurnTest(
                                network = network,
                                sessionId = sessionId,
                                targetIp = ipv6Addr,
                                targetPort = stunTurnPort,
                                addressFamily = AddressFamily.IPv6,
                                transferWindowSeconds = transferWindowSeconds,
                                payloadSizeBytes = transferPayloadBytes,
                                messagesPerSecond = transferMessagesPerSecond,
                                qualityThresholdRatio = transferQualityThreshold,
                                transport = turnTransport,
                            )
                        )
                        if (isEmpty()) {
                            add(
                                TestResult(
                                    id = UUID.randomUUID().toString(),
                                    sessionId = sessionId,
                                    testType = TestType.TURN,
                                    addressFamily = AddressFamily.IPv4,
                                    status = TestStatus.SKIPPED,
                                    failureReason = "server unsupported",
                                )
                            )
                        }
                    }
                    TestType.NAT64_DISCOVERY, TestType.DNS64_VALIDATION,
                    TestType.CLAT_QUALITY, TestType.PLAT_VERIFICATION -> emptyList()
                }

                if (networkChanged.get()) {
                    // Abort in-progress results and break
                    allResults.addAll(results.map { r ->
                        if (r.status == TestStatus.PASS) r
                        else r.copy(status = TestStatus.ABORTED, failureReason = "network changed during test")
                    })
                    break
                }
                allResults.addAll(results)
            }

            // Run 464XLAT sub-tests if requested
            var xlatSummary: XlatDiagnosticSummary? = null
            if (runXlat && !networkChanged.get()) {
                xlatSummary = runXlatDiagnostics(
                    context = context,
                    network = network,
                    networkInfo = networkInfo,
                    sessionId = sessionId,
                    serverIPv4 = ipv4Addr,
                    serverIPv6 = ipv6Addr,
                    serverPort = endpoint.httpPort,
                )
            }

            val status = if (networkChanged.get()) SessionStatus.ABORTED else SessionStatus.COMPLETED
            val session = DiagnosticSession(
                id = sessionId,
                timestamp = System.currentTimeMillis(),
                serverEndpoint = endpoint,
                testEndpointHost = endpoint.hostname,
                networkInfo = networkInfo,
                testResults = allResults,
                status = status,
                abortReason = if (networkChanged.get()) "network changed during test" else null,
            )
            repository.saveSession(session)
            xlatSummary?.let { repository.saveXlatSummary(it) }
            session
        } finally {
            runCatching { cm.unregisterNetworkCallback(changeCallback) }
        }
    }

    private suspend fun resolveAddress(hostname: String, isIPv6: Boolean): String? =
        withContext(Dispatchers.IO) {
            runCatching {
                InetAddress.getAllByName(hostname)
                    .firstOrNull { addr ->
                        if (isIPv6) addr is java.net.Inet6Address && !addr.isLinkLocalAddress
                        else addr is java.net.Inet4Address
                    }?.hostAddress
            }.getOrNull()
        }

    private fun parseHostAndPort(input: String): Pair<String, Int?> {
        val idx = input.lastIndexOf(':')
        if (idx <= 0 || idx == input.length - 1) return input to null
        val port = input.substring(idx + 1).toIntOrNull() ?: return input to null
        if (port !in 1..65535) return input to null
        val host = input.substring(0, idx)
        return host to port
    }
}
