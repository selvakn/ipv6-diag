package selvakn.ipv6diag.data.model

import kotlinx.serialization.Serializable

enum class SessionStatus { RUNNING, COMPLETED, ABORTED }

@Serializable
data class DiagnosticSession(
    val id: String,
    val timestamp: Long,
    val serverEndpoint: ServerEndpoint,
    val testEndpointHost: String,
    val networkInfo: NetworkInfo,
    val testResults: List<TestResult> = emptyList(),
    val status: SessionStatus = SessionStatus.RUNNING,
    val abortReason: String? = null,
)
