package com.xzxg.shopnative

import android.os.Bundle
import android.view.Gravity
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.launch

class MainActivity : AppCompatActivity() {
    private val api = ApiClient()
    private var auth: AuthSession? = null
    private var activeSession: AgentSessionItem? = null

    private lateinit var root: LinearLayout
    private lateinit var statusText: TextView
    private lateinit var historyList: LinearLayout
    private lateinit var chatList: LinearLayout
    private lateinit var input: EditText

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        render()
    }

    private fun render() {
        root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(28, 28, 28, 28)
        }
        setContentView(root)

        statusText = TextView(this).apply {
            text = "未登录"
            textSize = 16f
        }
        root.addView(statusText)

        val loginRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
        }
        val userInput = EditText(this).apply {
            hint = "账号"
            setText("user")
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        val passInput = EditText(this).apply {
            hint = "密码"
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        val loginButton = Button(this).apply {
            text = "登录"
            setOnClickListener { login(userInput.text.toString(), passInput.text.toString()) }
        }
        loginRow.addView(userInput)
        loginRow.addView(passInput)
        loginRow.addView(loginButton)
        root.addView(loginRow)

        val actionRow = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        actionRow.addView(Button(this).apply {
            text = "新会话"
            setOnClickListener { createSession() }
        })
        actionRow.addView(Button(this).apply {
            text = "历史会话"
            setOnClickListener { loadHistory() }
        })
        root.addView(actionRow)

        historyList = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        root.addView(historyList)

        val scroll = ScrollView(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                0,
                1f
            )
        }
        chatList = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        scroll.addView(chatList)
        root.addView(scroll)

        val sendRow = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        input = EditText(this).apply {
            hint = "输入导购问题"
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        sendRow.addView(input)
        sendRow.addView(Button(this).apply {
            text = "发送"
            setOnClickListener { sendMessage(input.text.toString()) }
        })
        root.addView(sendRow)
    }

    private fun login(username: String, password: String) {
        lifecycleScope.launch {
            runCatching { api.login(username, password) }
                .onSuccess {
                    auth = it
                    statusText.text = "已登录：${it.displayName}"
                    createSession()
                }
                .onFailure { statusText.text = "登录失败：${it.message}" }
        }
    }

    private fun createSession() {
        val token = auth?.token ?: return setStatus("请先登录")
        lifecycleScope.launch {
            runCatching { api.createSession(token) }
                .onSuccess {
                    activeSession = it
                    historyList.removeAllViews()
                    chatList.removeAllViews()
                    setStatus("当前会话：${it.title} / ${it.id}")
                }
                .onFailure { setStatus("创建会话失败：${it.message}") }
        }
    }

    private fun loadHistory() {
        val token = auth?.token ?: return setStatus("请先登录")
        lifecycleScope.launch {
            runCatching { api.listSessions(token) }
                .onSuccess { sessions ->
                    historyList.removeAllViews()
                    sessions.take(10).forEach { session ->
                        historyList.addView(Button(this@MainActivity).apply {
                            text = "${session.title}  ${session.createdAt}"
                            setOnClickListener { openSession(session) }
                        })
                    }
                }
                .onFailure { setStatus("加载历史失败：${it.message}") }
        }
    }

    private fun openSession(session: AgentSessionItem) {
        val token = auth?.token ?: return setStatus("请先登录")
        activeSession = session
        lifecycleScope.launch {
            runCatching { api.getSessionMessages(token, session.id) }
                .onSuccess { messages ->
                    chatList.removeAllViews()
                    setStatus("历史会话：${session.id}")
                    messages.forEach { addMessage("用户", "${it.content}\nrun=${it.runId ?: "-"} status=${it.status ?: "-"}") }
                }
                .onFailure { setStatus("加载会话失败：${it.message}") }
        }
    }

    private fun sendMessage(content: String) {
        val token = auth?.token ?: return setStatus("请先登录")
        val session = activeSession ?: return setStatus("请先创建会话")
        if (content.isBlank()) return
        input.setText("")
        addMessage("用户", content)
        val answer = addMessage("Agent", "")
        lifecycleScope.launch {
            runCatching {
                api.streamMessage(token, session.id, content) { event ->
                    runOnUiThread {
                        when (event.type) {
                            "status" -> setStatus(event.text.orEmpty())
                            "text_delta" -> answer.append(event.delta.orEmpty())
                            "error" -> answer.append("\n${event.message}")
                            "message_end" -> setStatus("生成完成")
                        }
                    }
                }
            }.onFailure { answer.append("\n连接失败：${it.message}") }
        }
    }

    private fun addMessage(role: String, content: String): TextView {
        return TextView(this).apply {
            text = "$role：$content"
            textSize = 15f
            setPadding(0, 14, 0, 14)
            chatList.addView(this)
        }
    }

    private fun setStatus(text: String) {
        statusText.text = text
    }
}
