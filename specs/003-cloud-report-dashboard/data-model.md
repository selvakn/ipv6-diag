# Data Model: Cloud Diagnostic Report Dashboard

## Server-side (SQLite)

### `reports` table

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | TEXT | PRIMARY KEY | Session UUID from Android |
| `device_name` | TEXT | NOT NULL | Human-readable device name |
| `device_model` | TEXT | NOT NULL | Build.MODEL |
| `device_manufacturer` | TEXT | NOT NULL | Build.MANUFACTURER |
| `android_version` | TEXT | NOT NULL | Build.VERSION.RELEASE |
| `device_id` | TEXT | NOT NULL | Settings.Secure.ANDROID_ID |
| `network_json` | TEXT | NOT NULL | Full NetworkInfo as JSON |
| `test_results_json` | TEXT | NOT NULL | Array of TestResult as JSON |
| `xlat_summary_json` | TEXT | NULL | XlatDiagnosticSummary as JSON, null if absent |
| `pass_count` | INTEGER | NOT NULL | Count of PASS results |
| `total_count` | INTEGER | NOT NULL | Total test count |
| `run_timestamp` | INTEGER | NOT NULL | Unix millis when diagnostic ran |
| `uploaded_at` | INTEGER | NOT NULL | Unix millis when report was received by server |

**Indexes**: `idx_reports_uploaded_at` (for date filter), `idx_reports_device_name` (for device filter)

**Upsert semantics**: `INSERT OR REPLACE` — re-uploading the same session ID overwrites the record.

---

## Server Go structs

### `UploadRequest` (received from Android via POST /reports)

```go
type UploadRequest struct {
    SessionID    string          `json:"session_id"`
    Device       DeviceInfo      `json:"device"`
    Network      json.RawMessage `json:"network"`
    TestResults  json.RawMessage `json:"test_results"`
    XlatSummary  json.RawMessage `json:"xlat_summary"` // null if absent
    PassCount    int             `json:"pass_count"`
    TotalCount   int             `json:"total_count"`
    RunTimestamp int64           `json:"run_timestamp"`
}

type DeviceInfo struct {
    Name         string `json:"name"`
    Model        string `json:"model"`
    Manufacturer string `json:"manufacturer"`
    AndroidVersion string `json:"android_version"`
    DeviceID     string `json:"device_id"`
}
```

### `ReportSummary` (used in GET /reports list response)

```go
type ReportSummary struct {
    ID           string `json:"id"`
    DeviceName   string `json:"device_name"`
    DeviceModel  string `json:"device_model"`
    PassCount    int    `json:"pass_count"`
    TotalCount   int    `json:"total_count"`
    RunTimestamp int64  `json:"run_timestamp"`
    UploadedAt   int64  `json:"uploaded_at"`
}
```

### `ReportDetail` (used in GET /reports/{id} response)

```go
type ReportDetail struct {
    ReportSummary
    DeviceManufacturer string          `json:"device_manufacturer"`
    AndroidVersion     string          `json:"android_version"`
    DeviceID           string          `json:"device_id"`
    Network            json.RawMessage `json:"network"`
    TestResults        json.RawMessage `json:"test_results"`
    XlatSummary        json.RawMessage `json:"xlat_summary"`
}
```

---

## Android-side additions

### `DeviceInfo.kt` (new model)

```kotlin
@Serializable
data class DeviceInfo(
    val name: String,
    val model: String,
    val manufacturer: String,
    val androidVersion: String,
    val deviceId: String,
)
```

### `UploadStatus` sealed class

```kotlin
sealed class UploadStatus {
    object Idle : UploadStatus()
    object Uploading : UploadStatus()
    object Success : UploadStatus()
    data class Failed(val reason: String) : UploadStatus()
}
```

### `CloudUploadRequest` (serialized and POSTed)

```kotlin
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
```
