package selvakn.ipv6diag.data.db

import androidx.room.ColumnInfo
import androidx.room.Entity
import androidx.room.ForeignKey
import androidx.room.Index
import androidx.room.PrimaryKey
import selvakn.ipv6diag.data.model.AddressFamily
import selvakn.ipv6diag.data.model.DiagnosticSession
import selvakn.ipv6diag.data.model.NetworkInfo
import selvakn.ipv6diag.data.model.ServerEndpoint
import selvakn.ipv6diag.data.model.SessionStatus
import selvakn.ipv6diag.data.model.TestResult
import selvakn.ipv6diag.data.model.TestStatus
import selvakn.ipv6diag.data.model.TestType
import selvakn.ipv6diag.data.model.XlatChainStatus
import selvakn.ipv6diag.data.model.XlatDiagnosticSummary
import kotlinx.serialization.json.Json

private val json = Json { ignoreUnknownKeys = true }

@Entity(tableName = "server_endpoints")
data class ServerEndpointEntity(
    @PrimaryKey val id: String,
    val hostname: String,
    @ColumnInfo(name = "ipv4_address") val ipv4Address: String? = null,
    @ColumnInfo(name = "ipv6_address") val ipv6Address: String? = null,
    @ColumnInfo(name = "http_port") val httpPort: Int = 80,
    @ColumnInfo(name = "https_port") val httpsPort: Int = 443,
    @ColumnInfo(name = "is_default") val isDefault: Boolean = false,
    @ColumnInfo(name = "last_verified") val lastVerified: Long? = null,
    @ColumnInfo(name = "use_https") val useHttps: Boolean = false,
) {
    fun toModel() = ServerEndpoint(id, hostname, ipv4Address, ipv6Address, httpPort, httpsPort, isDefault, lastVerified, useHttps)
}

fun ServerEndpoint.toEntity() = ServerEndpointEntity(id, hostname, ipv4Address, ipv6Address, httpPort, httpsPort, isDefault, lastVerified, useHttps)

@Entity(
    tableName = "diagnostic_sessions",
    foreignKeys = [ForeignKey(
        entity = ServerEndpointEntity::class,
        parentColumns = ["id"],
        childColumns = ["server_id"],
        onDelete = ForeignKey.CASCADE,
    )],
    indices = [Index("timestamp"), Index("server_id")],
)
data class DiagnosticSessionEntity(
    @PrimaryKey val id: String,
    val timestamp: Long,
    @ColumnInfo(name = "server_id") val serverId: String,
    @ColumnInfo(name = "test_endpoint_host") val testEndpointHost: String,
    @ColumnInfo(name = "network_info") val networkInfoJson: String,
    val status: String,
    @ColumnInfo(name = "abort_reason") val abortReason: String? = null,
) {
    fun toModel(endpoint: ServerEndpoint, results: List<TestResult>) = DiagnosticSession(
        id = id,
        timestamp = timestamp,
        serverEndpoint = endpoint,
        testEndpointHost = testEndpointHost,
        networkInfo = json.decodeFromString(networkInfoJson),
        testResults = results,
        status = SessionStatus.valueOf(status),
        abortReason = abortReason,
    )
}

fun DiagnosticSession.toEntity() = DiagnosticSessionEntity(
    id = id,
    timestamp = timestamp,
    serverId = serverEndpoint.id,
    testEndpointHost = testEndpointHost,
    networkInfoJson = json.encodeToString(NetworkInfo.serializer(), networkInfo),
    status = status.name,
    abortReason = abortReason,
)

@Entity(
    tableName = "test_results",
    foreignKeys = [ForeignKey(
        entity = DiagnosticSessionEntity::class,
        parentColumns = ["id"],
        childColumns = ["session_id"],
        onDelete = ForeignKey.CASCADE,
    )],
    indices = [Index("session_id")],
)
data class TestResultEntity(
    @PrimaryKey val id: String,
    @ColumnInfo(name = "session_id") val sessionId: String,
    @ColumnInfo(name = "test_type") val testType: String,
    @ColumnInfo(name = "address_family") val addressFamily: String,
    val status: String,
    @ColumnInfo(name = "latency_ms") val latencyMs: Long? = null,
    @ColumnInfo(name = "failure_reason") val failureReason: String? = null,
    @ColumnInfo(name = "resolved_address") val resolvedAddress: String? = null,
    @ColumnInfo(name = "server_confirmed_family") val serverConfirmedFamily: String? = null,
    @ColumnInfo(name = "packet_loss") val packetLoss: Float? = null,
    @ColumnInfo(name = "ice_candidates") val iceCandidates: List<String> = emptyList(),
    @ColumnInfo(name = "transfer_rate_kbps") val transferRateKbps: Double? = null,
    @ColumnInfo(name = "bytes_sent") val bytesSent: Long? = null,
    @ColumnInfo(name = "bytes_received") val bytesReceived: Long? = null,
    @ColumnInfo(name = "delivery_quality_ratio") val deliveryQualityRatio: Float? = null,
    @ColumnInfo(name = "quality_threshold_ratio") val qualityThresholdRatio: Float? = null,
    @ColumnInfo(name = "transfer_window_seconds") val transferWindowSeconds: Int? = null,
    @ColumnInfo(name = "payload_profile") val payloadProfile: String? = null,
    val timestamp: Long,
) {
    fun toModel() = TestResult(
        id = id,
        sessionId = sessionId,
        testType = TestType.valueOf(testType),
        addressFamily = AddressFamily.valueOf(addressFamily),
        status = TestStatus.valueOf(status),
        latencyMs = latencyMs,
        failureReason = failureReason,
        resolvedAddress = resolvedAddress,
        serverConfirmedFamily = serverConfirmedFamily,
        packetLoss = packetLoss,
        iceCandidates = iceCandidates,
        transferRateKbps = transferRateKbps,
        bytesSent = bytesSent,
        bytesReceived = bytesReceived,
        deliveryQualityRatio = deliveryQualityRatio,
        qualityThresholdRatio = qualityThresholdRatio,
        transferWindowSeconds = transferWindowSeconds,
        payloadProfile = payloadProfile,
        timestamp = timestamp,
    )
}

@Entity(
    tableName = "xlat_summaries",
    foreignKeys = [ForeignKey(
        entity = DiagnosticSessionEntity::class,
        parentColumns = ["id"],
        childColumns = ["session_id"],
        onDelete = ForeignKey.CASCADE,
    )],
)
data class XlatSummaryEntity(
    @PrimaryKey @ColumnInfo(name = "session_id") val sessionId: String,
    @ColumnInfo(name = "summary_json") val summaryJson: String,
    @ColumnInfo(name = "overall_status") val overallStatus: String,
) {
    fun toModel(): XlatDiagnosticSummary = json.decodeFromString(XlatDiagnosticSummary.serializer(), summaryJson)
}

fun XlatDiagnosticSummary.toEntity() = XlatSummaryEntity(
    sessionId = sessionId,
    summaryJson = json.encodeToString(XlatDiagnosticSummary.serializer(), this),
    overallStatus = overallStatus.name,
)

fun TestResult.toEntity() = TestResultEntity(
    id = id,
    sessionId = sessionId,
    testType = testType.name,
    addressFamily = addressFamily.name,
    status = status.name,
    latencyMs = latencyMs,
    failureReason = failureReason,
    resolvedAddress = resolvedAddress,
    serverConfirmedFamily = serverConfirmedFamily,
    packetLoss = packetLoss,
    iceCandidates = iceCandidates,
    transferRateKbps = transferRateKbps,
    bytesSent = bytesSent,
    bytesReceived = bytesReceived,
    deliveryQualityRatio = deliveryQualityRatio,
    qualityThresholdRatio = qualityThresholdRatio,
    transferWindowSeconds = transferWindowSeconds,
    payloadProfile = payloadProfile,
    timestamp = timestamp,
)
