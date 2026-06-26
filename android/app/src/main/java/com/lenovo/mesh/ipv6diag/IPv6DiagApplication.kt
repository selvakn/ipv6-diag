package com.lenovo.mesh.ipv6diag

import android.app.Application
import com.lenovo.mesh.ipv6diag.data.db.AppDatabase
import com.lenovo.mesh.ipv6diag.data.repository.SessionRepository
import com.lenovo.mesh.ipv6diag.upload.UploadStatus
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

class IPv6DiagApplication : Application() {

    val appScope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    lateinit var sessionRepository: SessionRepository
        private set

    private val _uploadStatus = MutableStateFlow<Map<String, UploadStatus>>(emptyMap())
    val uploadStatus: StateFlow<Map<String, UploadStatus>> = _uploadStatus

    fun setUploadStatus(sessionId: String, status: UploadStatus) {
        _uploadStatus.value = _uploadStatus.value + (sessionId to status)
    }

    override fun onCreate() {
        super.onCreate()
        val db = AppDatabase.getInstance(this)
        sessionRepository = SessionRepository(db)

        // Seed default server endpoint from config.xml on first launch
        appScope.launch {
            val defaultHostname = getString(R.string.default_server_hostname)
            sessionRepository.seedDefaultEndpointIfNeeded(defaultHostname)
        }
    }
}
