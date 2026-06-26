package com.lenovo.mesh.ipv6diag.data.repository

import com.lenovo.mesh.ipv6diag.data.db.AppDatabase
import com.lenovo.mesh.ipv6diag.data.db.toEntity
import com.lenovo.mesh.ipv6diag.data.model.DiagnosticSession
import com.lenovo.mesh.ipv6diag.data.model.ServerEndpoint
import com.lenovo.mesh.ipv6diag.data.model.XlatDiagnosticSummary
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import java.util.UUID

private const val MAX_SESSIONS = 50

class SessionRepository(private val db: AppDatabase) {

    /** Saves a completed session. Enforces the 50-session retention cap atomically. */
    suspend fun saveSession(session: DiagnosticSession) {
        db.serverEndpointDao().upsert(session.serverEndpoint.toEntity())
        db.sessionDao().insert(session.toEntity())
        db.testResultDao().insertAll(session.testResults.map { it.toEntity() })

        val count = db.sessionDao().count()
        if (count > MAX_SESSIONS) {
            db.sessionDao().deleteOldest(count - MAX_SESSIONS)
        }
    }

    /** Emits the full session list (with results) ordered newest-first. */
    fun getAllSessions(): Flow<List<DiagnosticSession>> =
        db.sessionDao().getAllFlow().map { entities ->
            entities.mapNotNull { entity ->
                val endpointEntity = db.serverEndpointDao().getDefault() ?: return@mapNotNull null
                val results = db.testResultDao().getBySession(entity.id).map { it.toModel() }
                entity.toModel(endpointEntity.toModel(), results)
            }
        }

    suspend fun getSessionById(id: String): DiagnosticSession? {
        val entity = db.sessionDao().getById(id) ?: return null
        val endpointEntity = db.serverEndpointDao().getDefault() ?: return null
        val results = db.testResultDao().getBySession(id).map { it.toModel() }
        return entity.toModel(endpointEntity.toModel(), results)
    }

    suspend fun getActiveEndpoint(): ServerEndpoint? =
        (db.serverEndpointDao().getCustom() ?: db.serverEndpointDao().getDefault())?.toModel()

    suspend fun saveCustomEndpoint(hostname: String, httpPort: Int = 80, httpsPort: Int = 443) {
        db.serverEndpointDao().deleteCustom()
        db.serverEndpointDao().upsert(
            ServerEndpoint(
                id = UUID.randomUUID().toString(),
                hostname = hostname,
                httpPort = httpPort,
                httpsPort = httpsPort,
                isDefault = false,
            ).toEntity()
        )
    }

    suspend fun clearCustomEndpoint() {
        db.serverEndpointDao().deleteCustom()
    }

    suspend fun saveXlatSummary(summary: XlatDiagnosticSummary) {
        db.xlatSummaryDao().insert(summary.toEntity())
    }

    suspend fun getXlatSummary(sessionId: String): XlatDiagnosticSummary? =
        db.xlatSummaryDao().getBySession(sessionId)?.toModel()

    suspend fun seedDefaultEndpointIfNeeded(hostname: String) {
        if (db.serverEndpointDao().getDefault() == null) {
            db.serverEndpointDao().upsert(
                ServerEndpoint(
                    id = "default",
                    hostname = hostname,
                    httpPort = 80,
                    httpsPort = 443,
                    isDefault = true,
                    useHttps = true,
                ).toEntity()
            )
        }
    }
}
