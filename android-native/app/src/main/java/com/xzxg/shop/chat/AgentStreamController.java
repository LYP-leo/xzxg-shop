package com.xzxg.shop.chat;

import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.LocalChatStore;

import android.os.Handler;
import android.os.Looper;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.UUID;

public class AgentStreamController {
    public interface Listener {
        void onSessionSyncRequested(String localSessionId, String serverSessionId, String title, String summary);

        void onServerSessionCreated(String localSessionId, String serverSessionId);

        void onStreamStarting(String localSessionId, String serverSessionId, String title);

        void onEvent(String localSessionId, JSONObject event);

        void onAuthExpired();

        void onCreateSessionFailed(String message, Throwable error);

        void onStreamFailed(String message, Throwable error);
    }

    private volatile boolean streaming;
    private volatile boolean stopRequested;
    private volatile String activeRunId = "";
    private volatile ApiClient.StreamCall activeStreamCall;
    private volatile String activeLocalSessionId = "";
    private volatile String activeServerSessionId = "";
    private volatile String activeTitle = "";
    private volatile String activeRequestId = "";
    private final Handler mainHandler = new Handler(Looper.getMainLooper());

    public boolean isStreaming() {
        return streaming;
    }

    public boolean isStopRequested() {
        return stopRequested;
    }

    public String activeRunId() {
        return activeRunId == null ? "" : activeRunId;
    }

    public String activeLocalSessionId() {
        return activeLocalSessionId == null ? "" : activeLocalSessionId;
    }

    public String activeServerSessionId() {
        return activeServerSessionId == null ? "" : activeServerSessionId;
    }

    public String activeTitle() {
        return activeTitle == null ? "" : activeTitle;
    }

    public String activeRequestId() {
        return activeRequestId == null ? "" : activeRequestId;
    }

    public String begin(String localSessionId) {
        streaming = true;
        stopRequested = false;
        activeRunId = "";
        activeLocalSessionId = localSessionId == null ? "" : localSessionId;
        activeRequestId = UUID.randomUUID().toString();
        return activeRequestId;
    }

    public void start(ApiClient api, LocalChatStore chatStore, String localSessionId, String serverSessionId, String text, JSONArray attachments, Listener listener) {
        String ownerLocalSessionId = safe(localSessionId);
        String ownerServerSessionId = safe(serverSessionId);
        String title = titleFromText(text);
        String requestId = begin(ownerLocalSessionId);
        if (!ownerServerSessionId.isEmpty()) {
            if (listener != null) {
                listener.onSessionSyncRequested(ownerLocalSessionId, ownerServerSessionId, title, text);
            }
            startStreamOnMain(api, requestId, ownerLocalSessionId, ownerServerSessionId, title, text, attachments, listener);
            return;
        }
        new Thread(() -> {
            try {
                JSONObject session = api.createSession("Android 新聊天");
                String createdServerSessionId = session.optString("session_id", "");
                if (createdServerSessionId.trim().isEmpty()) {
                    throw new IllegalStateException("创建会话失败：后端未返回 session_id");
                }
                if (!isCurrentRequest(requestId, ownerLocalSessionId)) {
                    return;
                }
                chatStore.bindServerSession(ownerLocalSessionId, createdServerSessionId);
                mainHandler.post(() -> {
                    if (!isCurrentRequest(requestId, ownerLocalSessionId)) {
                        return;
                    }
                    if (listener != null) {
                        listener.onSessionSyncRequested(ownerLocalSessionId, createdServerSessionId, title, text);
                        listener.onServerSessionCreated(ownerLocalSessionId, createdServerSessionId);
                    }
                    startStream(api, requestId, ownerLocalSessionId, createdServerSessionId, title, text, attachments, listener);
                });
            } catch (Exception error) {
                mainHandler.post(() -> {
                    if (!isCurrentRequest(requestId, ownerLocalSessionId)) {
                        return;
                    }
                    if (isAuthExpired(error)) {
                        stopStreamingOnly();
                        if (listener != null) {
                            listener.onAuthExpired();
                        }
                    } else if (listener != null) {
                        listener.onCreateSessionFailed("创建会话失败：" + readableErrorMessage(error), error);
                    }
                });
            }
        }).start();
    }

    public void prepareStream(String localSessionId, String serverSessionId, String title) {
        activeLocalSessionId = localSessionId == null ? "" : localSessionId;
        activeServerSessionId = serverSessionId == null ? "" : serverSessionId;
        activeTitle = title == null ? "" : title;
    }

    public void attachCall(ApiClient.StreamCall call) {
        activeStreamCall = call;
    }

    public void markRun(String runId) {
        activeRunId = runId == null ? "" : runId;
    }

    public boolean shouldIgnoreEvent(String ownerLocalSessionId, String currentLocalSessionId) {
        return stopRequested || !safe(ownerLocalSessionId).equals(safe(currentLocalSessionId));
    }

    public String ownerLocalOr(String fallbackLocalSessionId) {
        return activeLocalSessionId == null || activeLocalSessionId.isEmpty() ? safe(fallbackLocalSessionId) : activeLocalSessionId;
    }

    public boolean requestStop(ApiClient api) {
        if (!streaming) {
            return false;
        }
        stopRequested = true;
        String runId = activeRunId();
        if (!runId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.cancelAgentRun(runId);
                } catch (Exception ignored) {
                }
            }).start();
        }
        ApiClient.StreamCall call = activeStreamCall;
        if (call != null) {
            call.cancel();
        }
        return true;
    }

    public void cancelActiveCall() {
        ApiClient.StreamCall call = activeStreamCall;
        if (call != null) {
            call.cancel();
        }
    }

    public void clearCall() {
        activeRunId = "";
        activeStreamCall = null;
    }

    public void reset() {
        streaming = false;
        stopRequested = false;
        activeRunId = "";
        activeStreamCall = null;
        activeLocalSessionId = "";
        activeServerSessionId = "";
        activeTitle = "";
        activeRequestId = "";
    }

    public void stopStreamingOnly() {
        streaming = false;
    }

    private void startStreamOnMain(ApiClient api, String requestId, String localSessionId, String serverSessionId, String title, String text, JSONArray attachments, Listener listener) {
        mainHandler.post(() -> startStream(api, requestId, localSessionId, serverSessionId, title, text, attachments, listener));
    }

    private void startStream(ApiClient api, String requestId, String localSessionId, String serverSessionId, String title, String text, JSONArray attachments, Listener listener) {
        if (!isCurrentRequest(requestId, localSessionId)) {
            return;
        }
        prepareStream(localSessionId, serverSessionId, title);
        if (listener != null) {
            listener.onStreamStarting(localSessionId, serverSessionId, title);
        }
        String clientMessageId = "android_" + UUID.randomUUID().toString();
        attachCall(api.streamMessage(serverSessionId, clientMessageId, text, attachments, new ApiClient.SseCallback() {
            @Override
            public void onEvent(JSONObject event) {
                mainHandler.post(() -> {
                    if (!isCurrentRequest(requestId, localSessionId)) {
                        return;
                    }
                    if (listener != null) {
                        listener.onEvent(localSessionId, event);
                    }
                });
            }

            @Override
            public void onError(Throwable error) {
                mainHandler.post(() -> {
                    if (!isCurrentRequest(requestId, localSessionId)) {
                        return;
                    }
                    if (isAuthExpired(error)) {
                        stopStreamingOnly();
                        if (listener != null) {
                            listener.onAuthExpired();
                        }
                        return;
                    }
                    if (!isStopRequested() && listener != null) {
                        listener.onStreamFailed("发送失败：" + readableErrorMessage(error), error);
                    }
                });
            }
        }));
    }

    private boolean isCurrentRequest(String requestId, String localSessionId) {
        return streaming && safe(requestId).equals(activeRequestId) && safe(localSessionId).equals(activeLocalSessionId);
    }

    private String titleFromText(String text) {
        String value = text == null ? "" : text.trim().replace('\n', ' ');
        if (value.isEmpty()) {
            return "导购会话";
        }
        return value.length() > 18 ? value.substring(0, 18) + "..." : value;
    }

    private boolean isAuthExpired(Throwable error) {
        return error instanceof ApiClient.ApiException && ((ApiClient.ApiException) error).statusCode == 401;
    }

    private String readableErrorMessage(Throwable error) {
        if (error == null) {
            return "未知错误";
        }
        String message = error.getMessage();
        if (message != null && !message.trim().isEmpty()) {
            return message.trim();
        }
        String type = error.getClass().getSimpleName();
        return type == null || type.trim().isEmpty() ? "未知错误" : type;
    }

    private String safe(String value) {
        return value == null ? "" : value;
    }
}
