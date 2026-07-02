package selvakn.ipv6diag.data.model

import kotlinx.serialization.Serializable

enum class TestType { HTTP, HTTPS, ICMP, DNS, STUN, TURN, NAT64_DISCOVERY, DNS64_VALIDATION, CLAT_QUALITY, PLAT_VERIFICATION }

enum class AddressFamily { IPv4, IPv6, XLAT }

enum class TestStatus { PASS, FAIL, SKIPPED, ABORTED }

@Serializable
data class TestResult(
    val id: String,
    val sessionId: String,
    val testType: TestType,
    val addressFamily: AddressFamily,
    val status: TestStatus,
    val latencyMs: Long? = null,
    val failureReason: String? = null,
    val resolvedAddress: String? = null,
    val serverConfirmedFamily: String? = null,
    val packetLoss: Float? = null,
    val iceCandidates: List<String> = emptyList(),
    val transferRateKbps: Double? = null,
    val bytesSent: Long? = null,
    val bytesReceived: Long? = null,
    val deliveryQualityRatio: Float? = null,
    val qualityThresholdRatio: Float? = null,
    val transferWindowSeconds: Int? = null,
    val payloadProfile: String? = null,
    val timestamp: Long = System.currentTimeMillis(),
)
