package com.lenovo.mesh.ipv6diag.upload

import com.lenovo.mesh.ipv6diag.data.model.DeviceInfo
import com.lenovo.mesh.ipv6diag.data.model.DiagnosticSession
import com.lenovo.mesh.ipv6diag.data.model.NetworkInfo
import com.lenovo.mesh.ipv6diag.data.model.TestResult
import com.lenovo.mesh.ipv6diag.data.model.TestStatus
import com.lenovo.mesh.ipv6diag.data.model.XlatDiagnosticSummary
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

@Serializable
data class CloudUploadRequest(
    @SerialName("session_id") val sessionId: String,
    val device: DeviceInfo,
    val network: NetworkInfo,
    @SerialName("test_results") val testResults: List<TestResult>,
    @SerialName("xlat_summary") val xlatSummary: XlatDiagnosticSummary? = null,
    @SerialName("pass_count") val passCount: Int,
    @SerialName("total_count") val totalCount: Int,
    @SerialName("run_timestamp") val runTimestamp: Long,
)

private val json = Json { encodeDefaults = true; ignoreUnknownKeys = true }
private val client = OkHttpClient.Builder()
    .connectTimeout(10L, TimeUnit.SECONDS)
    .readTimeout(15L, TimeUnit.SECONDS)
    .build()

suspend fun uploadReport(
    session: DiagnosticSession,
    device: DeviceInfo,
    xlatSummary: XlatDiagnosticSummary?,
    serverUrl: String,
): UploadStatus = withContext(Dispatchers.IO) {
    runCatching {
        val passCount = session.testResults.count { it.status == TestStatus.PASS }
        val payload = CloudUploadRequest(
            sessionId = session.id,
            device = device,
            network = session.networkInfo,
            testResults = session.testResults,
            xlatSummary = xlatSummary,
            passCount = passCount,
            totalCount = session.testResults.size,
            runTimestamp = session.timestamp,
        )
        val body = json.encodeToString(CloudUploadRequest.serializer(), payload)
            .toRequestBody("application/json".toMediaType())
        val request = Request.Builder()
            .url("$serverUrl/reports")
            .post(body)
            .build()
        val response = client.newCall(request).execute()
        if (response.isSuccessful) UploadStatus.Success
        else UploadStatus.Failed("HTTP ${response.code}")
    }.getOrElse { e ->
        UploadStatus.Failed(e.message ?: "unknown error")
    }
}
