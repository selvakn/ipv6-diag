package selvakn.ipv6diag.diagnostic

import android.net.Network
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request
import java.util.concurrent.TimeUnit

data class TurnCredentials(
    val username: String,
    val password: String,
    val realm: String,
    val uris: List<String>,
)

// Mirrors cli/diag/turn_creds.go's FetchTurnCredentials: GET /turn/credentials.
// Returns null if TURN is disabled on the server (HTTP 503), the response is
// malformed, or the request fails outright — callers treat null as "skip TURN".
suspend fun fetchTurnCredentials(network: Network, baseUrl: String): TurnCredentials? =
    withContext(Dispatchers.IO) {
        runCatching {
            val client = OkHttpClient.Builder()
                .socketFactory(network.socketFactory)
                .connectTimeout(5, TimeUnit.SECONDS)
                .readTimeout(5, TimeUnit.SECONDS)
                .build()
            val url = "${baseUrl.trimEnd('/')}/turn/credentials"
            val request = Request.Builder().url(url).get().build()
            client.newCall(request).execute().use { response ->
                if (!response.isSuccessful) return@use null
                val body = response.body?.string() ?: return@use null
                val json = Json.parseToJsonElement(body).jsonObject
                val username = json["username"]?.jsonPrimitive?.content
                val password = json["password"]?.jsonPrimitive?.content
                val realm = json["realm"]?.jsonPrimitive?.content
                if (username == null || password == null || realm == null) return@use null
                TurnCredentials(
                    username = username,
                    password = password,
                    realm = realm,
                    uris = json["uris"]?.jsonArray?.map { it.jsonPrimitive.content } ?: emptyList(),
                )
            }
        }.getOrNull()
    }
