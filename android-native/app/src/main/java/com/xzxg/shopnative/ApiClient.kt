package com.xzxg.shopnative

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.TimeUnit

data class AuthSession(val token: String, val displayName: String)
data class AgentSessionItem(val id: String, val title: String, val createdAt: String)
data class AgentMessageItem(val content: String, val runId: String?, val status: String?)

class ApiClient(
    private val baseUrl: String = "http://10.0.2.2:8080/api/v1"
) {
    private val jsonType = "application/json; charset=utf-8".toMediaType()
    private val client = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.SECONDS)
        .build()

    suspend fun login(username: String, password: String): AuthSession = withContext(Dispatchers.IO) {
        val body = JSONObject()
            .put("username", username)
            .put("password", password)
            .toString()
            .toRequestBody(jsonType)
        val request = Request.Builder()
            .url("$baseUrl/auth/login")
            .post(body)
            .build()
        val json = executeJSON(request)
        val account = json.getJSONObject("account")
        AuthSession(json.getString("token"), account.optString("display_name", username))
    }

    suspend fun createSession(token: String, title: String = "AI 导购"): AgentSessionItem = withContext(Dispatchers.IO) {
        val body = JSONObject().put("title", title).toString().toRequestBody(jsonType)
        val request = authorized(token, "$baseUrl/agent/sessions")
            .post(body)
            .build()
        parseSession(executeJSON(request))
    }

    suspend fun listSessions(token: String): List<AgentSessionItem> = withContext(Dispatchers.IO) {
        val request = authorized(token, "$baseUrl/agent/sessions").get().build()
        val items = executeJSON(request).optJSONArray("items") ?: JSONArray()
        List(items.length()) { index -> parseSession(items.getJSONObject(index)) }
    }

    suspend fun getSessionMessages(token: String, sessionId: String): List<AgentMessageItem> = withContext(Dispatchers.IO) {
        val request = authorized(token, "$baseUrl/agent/sessions/$sessionId").get().build()
        val messages = executeJSON(request).optJSONArray("messages") ?: JSONArray()
        List(messages.length()) { index ->
            val item = messages.getJSONObject(index)
            val runs = item.optJSONArray("runs")
            val run = if (runs != null && runs.length() > 0) runs.getJSONObject(runs.length() - 1) else null
            AgentMessageItem(
                content = item.optString("content"),
                runId = run?.optString("run_id"),
                status = run?.optString("status")
            )
        }
    }

    suspend fun streamMessage(
        token: String,
        sessionId: String,
        content: String,
        onEvent: (AgentStreamEvent) -> Unit
    ) = withContext(Dispatchers.IO) {
        val body = JSONObject()
            .put("client_message_id", "android_${System.currentTimeMillis()}")
            .put("content", content)
            .put("attachments", JSONArray())
            .toString()
            .toRequestBody(jsonType)
        val request = authorized(token, "$baseUrl/agent/sessions/$sessionId/messages:stream")
            .addHeader("Accept", "text/event-stream")
            .post(body)
            .build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) error("Agent 请求失败：${response.code}")
            val source = response.body.source()
            while (!source.exhausted()) {
                val line = source.readUtf8Line() ?: break
                if (!line.startsWith("data:")) continue
                val payload = line.removePrefix("data:").trim()
                if (payload.isEmpty()) continue
                val event = JSONObject(payload)
                onEvent(
                    AgentStreamEvent(
                        type = event.optString("type"),
                        runId = event.optString("run_id"),
                        text = event.optString("text"),
                        delta = event.optString("delta"),
                        message = event.optString("message")
                    )
                )
            }
        }
    }

    private fun authorized(token: String, url: String): Request.Builder {
        return Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer $token")
            .addHeader("Content-Type", "application/json")
    }

    private fun executeJSON(request: Request): JSONObject {
        client.newCall(request).execute().use { response ->
            val body = response.body.string()
            if (!response.isSuccessful) error(body.ifBlank { "请求失败：${response.code}" })
            return JSONObject(body)
        }
    }

    private fun parseSession(json: JSONObject): AgentSessionItem {
        return AgentSessionItem(
            id = json.getString("session_id"),
            title = json.optString("title", "AI 导购"),
            createdAt = json.optString("created_at")
        )
    }
}

data class AgentStreamEvent(
    val type: String,
    val runId: String?,
    val text: String?,
    val delta: String?,
    val message: String?
)
