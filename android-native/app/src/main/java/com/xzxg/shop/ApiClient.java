package com.xzxg.shop;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.nio.charset.StandardCharsets;

public class ApiClient {
    private final SessionStore sessionStore;

    public ApiClient(SessionStore sessionStore) {
        this.sessionStore = sessionStore;
    }

    public JSONObject getAgentHome() throws Exception {
        return get("/agent/home");
    }

    public JSONObject login(String username, String password) throws Exception {
        JSONObject body = new JSONObject();
        body.put("username", username);
        body.put("password", password);
        return post("/auth/login", body);
    }

    public JSONObject createVerificationCode(String scene, String targetType, String target) throws Exception {
        JSONObject body = new JSONObject();
        body.put("scene", scene);
        body.put("target_type", targetType);
        body.put("target", target);
        return post("/auth/verification-codes", body);
    }

    public JSONObject register(String username, String password, String displayName, String role, String merchantName, String code) throws Exception {
        JSONObject body = new JSONObject();
        body.put("username", username);
        body.put("password", password);
        body.put("display_name", displayName);
        body.put("role", role);
        body.put("merchant_name", merchantName);
        body.put("target_type", "email");
        body.put("target", username + "@example.com");
        body.put("verification_code", code);
        return post("/auth/register", body);
    }

    public JSONObject changePassword(String oldPassword, String newPassword) throws Exception {
        JSONObject body = new JSONObject();
        body.put("old_password", oldPassword);
        body.put("new_password", newPassword);
        return post("/auth/password:change", body);
    }

    public JSONObject profile() throws Exception {
        return get("/account/profile");
    }

    public JSONObject updateProfile(String nickname) throws Exception {
        JSONObject body = new JSONObject();
        body.put("nickname", nickname);
        body.put("avatar_url", "");
        return patch("/account/profile", body);
    }

    public JSONArray products() throws Exception {
        JSONObject response = get("/products");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONObject cart() throws Exception {
        return get("/cart");
    }

    public JSONArray orders() throws Exception {
        JSONObject response = get("/orders");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONArray sessions() throws Exception {
        JSONObject response = get("/agent/sessions");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONObject createSession(String title) throws Exception {
        JSONObject body = new JSONObject();
        body.put("title", title);
        return post("/agent/sessions", body);
    }

    public void streamMessage(String sessionId, String content, SseCallback callback) {
        new Thread(() -> {
            HttpURLConnection conn = null;
            try {
                JSONObject body = new JSONObject();
                body.put("client_message_id", "android_" + System.currentTimeMillis());
                body.put("content", content);
                body.put("attachments", new JSONArray());
                conn = open("/agent/sessions/" + sessionId + "/messages:stream", "POST");
                conn.setRequestProperty("Accept", "text/event-stream");
                writeBody(conn, body);

                int code = conn.getResponseCode();
                if (code < 200 || code >= 300) {
                    callback.onError(new RuntimeException(readText(conn.getErrorStream())));
                    return;
                }
                BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream(), StandardCharsets.UTF_8));
                String line;
                while ((line = reader.readLine()) != null) {
                    if (!line.startsWith("data:")) {
                        continue;
                    }
                    String data = line.substring(5).trim();
                    if (data.isEmpty()) {
                        continue;
                    }
                    callback.onEvent(new JSONObject(data));
                }
            } catch (Exception error) {
                callback.onError(error);
            } finally {
                if (conn != null) {
                    conn.disconnect();
                }
            }
        }).start();
    }

    private JSONObject get(String path) throws Exception {
        HttpURLConnection conn = open(path, "GET");
        return readJSON(conn);
    }

    private JSONObject post(String path, JSONObject body) throws Exception {
        HttpURLConnection conn = open(path, "POST");
        writeBody(conn, body);
        return readJSON(conn);
    }

    private JSONObject patch(String path, JSONObject body) throws Exception {
        HttpURLConnection conn = open(path, "PATCH");
        writeBody(conn, body);
        return readJSON(conn);
    }

    private HttpURLConnection open(String path, String method) throws Exception {
        URL url = new URL(trimSlash(sessionStore.apiBase()) + path);
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod(method);
        conn.setConnectTimeout(10000);
        conn.setReadTimeout(60000);
        conn.setRequestProperty("Content-Type", "application/json; charset=utf-8");
        String token = sessionStore.token();
        if (!token.isEmpty()) {
            conn.setRequestProperty("Authorization", "Bearer " + token);
        }
        if (!"GET".equals(method)) {
            conn.setDoOutput(true);
        }
        return conn;
    }

    private void writeBody(HttpURLConnection conn, JSONObject body) throws Exception {
        byte[] data = body.toString().getBytes(StandardCharsets.UTF_8);
        try (OutputStream out = conn.getOutputStream()) {
            out.write(data);
        }
    }

    private JSONObject readJSON(HttpURLConnection conn) throws Exception {
        int code = conn.getResponseCode();
        InputStream stream = code >= 200 && code < 300 ? conn.getInputStream() : conn.getErrorStream();
        String text = readText(stream);
        if (code < 200 || code >= 300) {
            throw new RuntimeException(text.isEmpty() ? ("HTTP " + code) : text);
        }
        return text.isEmpty() ? new JSONObject() : new JSONObject(text);
    }

    private String readText(InputStream stream) throws Exception {
        if (stream == null) {
            return "";
        }
        BufferedReader reader = new BufferedReader(new InputStreamReader(stream, StandardCharsets.UTF_8));
        StringBuilder builder = new StringBuilder();
        String line;
        while ((line = reader.readLine()) != null) {
            builder.append(line);
        }
        return builder.toString();
    }

    private String trimSlash(String value) {
        while (value.endsWith("/")) {
            value = value.substring(0, value.length() - 1);
        }
        return value;
    }

    public interface SseCallback {
        void onEvent(JSONObject event);
        void onError(Throwable error);
    }
}
