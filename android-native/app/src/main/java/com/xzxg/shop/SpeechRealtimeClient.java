package com.xzxg.shop;

import org.json.JSONObject;

import android.util.Log;

import java.util.concurrent.TimeUnit;

import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.WebSocket;
import okhttp3.WebSocketListener;
import okio.ByteString;

class SpeechRealtimeClient {
    private static final String TAG = "SpeechRealtimeClient";

    interface Listener {
        void onReady();
        void onPartial(String text);
        void onFinal(String text);
        void onError(String code, String message);
        void onClosed();
    }

    private final SessionStore sessionStore;
    private final Listener listener;
    private final OkHttpClient client;
    private WebSocket webSocket;
    private boolean finalDelivered;

    SpeechRealtimeClient(SessionStore sessionStore, Listener listener) {
        this.sessionStore = sessionStore;
        this.listener = listener;
        this.client = new OkHttpClient.Builder()
                .connectTimeout(8, TimeUnit.SECONDS)
                .readTimeout(0, TimeUnit.SECONDS)
                .build();
    }

    void connect() {
        Request.Builder builder = new Request.Builder()
                .url(realtimeUrl())
                .header("X-Client", "android-native");
        String token = sessionStore.token();
        if (token != null && !token.isEmpty()) {
            builder.header("Authorization", "Bearer " + token);
        }
        webSocket = client.newWebSocket(builder.build(), new WebSocketListener() {
            @Override
            public void onOpen(WebSocket webSocket, Response response) {
                Log.i(TAG, "speech websocket opened status=" + response.code());
                JSONObject start = new JSONObject();
                try {
                    start.put("type", "start");
                    start.put("language", "zh-CN");
                    start.put("format", "pcm");
                    start.put("sample_rate", PcmRecorder.SAMPLE_RATE);
                    webSocket.send(start.toString());
                } catch (Exception error) {
                    notifyError("speech_recognition_failed", "语音识别启动失败");
                }
            }

            @Override
            public void onMessage(WebSocket webSocket, String text) {
                Log.i(TAG, "speech websocket message length=" + (text == null ? 0 : text.length()));
                handleMessage(text);
            }

            @Override
            public void onClosing(WebSocket webSocket, int code, String reason) {
                Log.i(TAG, "speech websocket closing code=" + code + " reason=" + reason);
                webSocket.close(code, reason);
            }

            @Override
            public void onClosed(WebSocket webSocket, int code, String reason) {
                Log.i(TAG, "speech websocket closed code=" + code + " reason=" + reason);
                if (listener != null) {
                    listener.onClosed();
                }
            }

            @Override
            public void onFailure(WebSocket webSocket, Throwable t, Response response) {
                int status = response == null ? 0 : response.code();
                Log.w(TAG, "speech websocket failure status=" + status, t);
                String code;
                if (status == 401 || status == 403) {
                    code = "unauthorized";
                } else if (status == 404 || status == 501) {
                    code = "speech_not_enabled";
                } else {
                    code = "speech_network_error";
                }
                String message = t == null || t.getMessage() == null ? "语音识别连接失败" : t.getMessage();
                notifyError(code, message);
            }
        });
    }

    void sendAudio(byte[] frame) {
        WebSocket socket = webSocket;
        if (socket != null && frame != null && frame.length > 0) {
            socket.send(ByteString.of(frame));
        }
    }

    void sendEnd() {
        WebSocket socket = webSocket;
        if (socket != null) {
            socket.send("{\"type\":\"end\"}");
        }
    }

    void cancel() {
        WebSocket socket = webSocket;
        if (socket != null) {
            socket.send("{\"type\":\"cancel\"}");
            socket.close(1000, "cancel");
        }
        webSocket = null;
    }

    void close() {
        WebSocket socket = webSocket;
        if (socket != null) {
            socket.close(1000, "close");
        }
        webSocket = null;
    }

    private void handleMessage(String text) {
        try {
            JSONObject event = new JSONObject(text);
            String type = event.optString("type", "");
            if ("ready".equals(type)) {
                if (listener != null) {
                    listener.onReady();
                }
                return;
            }
            if ("partial".equals(type)) {
                if (listener != null) {
                    listener.onPartial(event.optString("text", ""));
                }
                return;
            }
            if ("final".equals(type)) {
                if (!finalDelivered && listener != null) {
                    finalDelivered = true;
                    listener.onFinal(event.optString("text", ""));
                }
                return;
            }
            if ("error".equals(type)) {
                notifyError(event.optString("code", "speech_recognition_failed"),
                        event.optString("message", "语音识别失败，请重试"));
            }
        } catch (Exception error) {
            notifyError("speech_recognition_failed", "语音识别结果解析失败");
        }
    }

    private String realtimeUrl() {
        String base = sessionStore.apiBase();
        String wsBase;
        if (base.startsWith("https://")) {
            wsBase = "wss://" + base.substring("https://".length());
        } else if (base.startsWith("http://")) {
            wsBase = "ws://" + base.substring("http://".length());
        } else {
            wsBase = base;
        }
        while (wsBase.endsWith("/")) {
            wsBase = wsBase.substring(0, wsBase.length() - 1);
        }
        return wsBase + "/speech/realtime?language=zh-CN&format=pcm&sample_rate=" + PcmRecorder.SAMPLE_RATE;
    }

    private void notifyError(String code, String message) {
        if (listener != null) {
            listener.onError(code, message);
        }
    }
}
