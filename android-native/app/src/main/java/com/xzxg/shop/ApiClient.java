package com.xzxg.shop;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
import java.net.ProtocolException;
import java.net.SocketException;
import java.net.URL;
import java.net.URLEncoder;
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

    public JSONObject logout() throws Exception {
        return post("/auth/logout", new JSONObject());
    }

    public JSONObject me() throws Exception {
        return get("/auth/me");
    }

    public JSONObject profile() throws Exception {
        return get("/account/profile");
    }

    public JSONObject updateProfile(String nickname) throws Exception {
        return updateProfile(nickname, "");
    }

    public JSONObject updateProfile(String nickname, String avatarUrl) throws Exception {
        JSONObject body = new JSONObject();
        body.put("nickname", nickname);
        body.put("avatar_url", avatarUrl == null ? "" : avatarUrl);
        return patch("/account/profile", body);
    }

    public JSONObject updateContact(String phone, String email) throws Exception {
        JSONObject body = new JSONObject();
        body.put("phone", phone == null ? "" : phone);
        body.put("email", email == null ? "" : email);
        return patch("/account/contact", body);
    }

    public JSONObject deleteAccount() throws Exception {
        HttpURLConnection conn = open("/account", "DELETE");
        return readJSON(conn);
    }

    public JSONObject uploadAvatar(String name, String mimeType, byte[] data) throws Exception {
        String boundary = "----xzxgAvatar" + System.currentTimeMillis();
        HttpURLConnection conn = openRaw("/uploads/avatar", "POST");
        conn.setRequestProperty("Content-Type", "multipart/form-data; boundary=" + boundary);
        try (OutputStream out = conn.getOutputStream()) {
            out.write(("--" + boundary + "\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(("Content-Disposition: form-data; name=\"file\"; filename=\"" + safeFileName(name) + "\"\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(("Content-Type: " + (mimeType == null || mimeType.isEmpty() ? "image/jpeg" : mimeType) + "\r\n\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(data);
            out.write(("\r\n--" + boundary + "--\r\n").getBytes(StandardCharsets.UTF_8));
        }
        return readJSON(conn);
    }

    public JSONArray categoriesTree() throws Exception {
        JSONObject response = get("/categories/tree");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONArray products() throws Exception {
        return products("", "");
    }

    public JSONArray products(String keyword, String categoryId) throws Exception {
        return productsPage(keyword, categoryId, 1, 100).items;
    }

    public ProductPage productsPage(String keyword, String categoryId, int page, int pageSize) throws Exception {
        StringBuilder path = new StringBuilder("/products");
        String separator = "?";
        if (keyword != null && !keyword.trim().isEmpty()) {
            path.append(separator).append("keyword=").append(urlEncode(keyword.trim()));
            separator = "&";
        }
        if (categoryId != null && !categoryId.trim().isEmpty()) {
            path.append(separator).append("category_id=").append(urlEncode(categoryId.trim()));
            separator = "&";
        }
        if (pageSize > 0) {
            int safePage = Math.max(1, page);
            path.append(separator).append("page=").append(safePage);
            separator = "&";
            path.append(separator).append("page_size=").append(pageSize);
        }
        JSONObject response = get(path.toString());
        JSONArray items = response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
        int currentPage = response.optInt("page", Math.max(1, page));
        int currentPageSize = response.optInt("page_size", pageSize);
        int total = response.optInt("total", items.length());
        boolean hasMore = currentPageSize > 0 && currentPage * currentPageSize < total;
        return new ProductPage(items, currentPage + 1, hasMore, total);
    }

    public JSONArray productSkus(String productId) throws Exception {
        JSONObject response = get("/products/" + urlEncode(productId) + "/skus");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONObject productDetail(String productId) throws Exception {
        return get("/products/" + urlEncode(productId));
    }

    public JSONArray productReviews(String productId) throws Exception {
        JSONObject response = get("/products/" + urlEncode(productId) + "/reviews");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public String absoluteUrl(String url) {
        if (url == null || url.trim().isEmpty()) {
            return "";
        }
        String value = url.trim();
        if (value.startsWith("http://") || value.startsWith("https://")) {
            return value;
        }
        String base = trimSlash(sessionStore.apiBase());
        int marker = base.indexOf("/api/");
        String host = marker >= 0 ? base.substring(0, marker) : base;
        if (!value.startsWith("/")) {
            value = "/" + value;
        }
        return host + value;
    }

    public JSONObject cart() throws Exception {
        return get("/cart");
    }

    public JSONObject discountPreview() throws Exception {
        return get("/cart/discount-preview");
    }

    public JSONObject addCartItem(String productId, String skuId, int quantity) throws Exception {
        JSONObject body = new JSONObject();
        body.put("product_id", productId);
        body.put("sku_id", skuId == null ? "" : skuId);
        body.put("quantity", Math.max(1, quantity));
        return post("/cart/items", body);
    }

    public JSONObject updateCartItem(String cartItemId, Integer quantity, Boolean selected) throws Exception {
        JSONObject body = new JSONObject();
        if (quantity != null) {
            body.put("quantity", quantity);
        }
        if (selected != null) {
            body.put("selected", selected);
        }
        return patch("/cart/items/" + urlEncode(cartItemId), body);
    }

    public JSONObject deleteCartItem(String cartItemId) throws Exception {
        HttpURLConnection conn = open("/cart/items/" + urlEncode(cartItemId), "DELETE");
        return readJSON(conn);
    }

    public JSONArray orders() throws Exception {
        JSONObject response = get("/orders");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONObject orderDetail(String orderId) throws Exception {
        return get("/orders/" + urlEncode(orderId));
    }

    public JSONArray checkout() throws Exception {
        JSONObject response = post("/orders:checkout", new JSONObject());
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONObject payOrder(String orderId) throws Exception {
        JSONObject body = new JSONObject();
        body.put("method", "mock_balance");
        return post("/orders/" + urlEncode(orderId) + ":pay", body);
    }

    public JSONObject cancelOrder(String orderId, String reason) throws Exception {
        JSONObject body = new JSONObject();
        body.put("reason", reason == null || reason.isEmpty() ? "暂时不买了" : reason);
        return post("/orders/" + urlEncode(orderId) + ":cancel", body);
    }

    public JSONObject confirmReceipt(String orderId) throws Exception {
        return post("/orders/" + urlEncode(orderId) + ":confirm-receipt", new JSONObject());
    }

    public JSONObject reviewOrderItem(String orderId, String orderItemId, int rating, String content, JSONArray tags) throws Exception {
        JSONObject body = new JSONObject();
        body.put("rating", rating);
        body.put("content", content);
        body.put("tags", tags == null ? new JSONArray() : tags);
        return post("/orders/" + urlEncode(orderId) + "/items/" + urlEncode(orderItemId) + ":review", body);
    }

    public JSONArray promotions() throws Exception {
        JSONObject response = get("/promotions");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONArray availableCoupons() throws Exception {
        JSONObject response = get("/coupons/available");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONArray myCoupons() throws Exception {
        JSONObject response = get("/coupons/mine");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
    }

    public JSONObject claimCoupon(String couponId) throws Exception {
        return post("/coupons/" + urlEncode(couponId) + ":claim", new JSONObject());
    }

    public JSONArray sessions() throws Exception {
        return sessionsPage(1, 10).items;
    }

    public SessionPage sessionsPage(int page, int pageSize) throws Exception {
        int safePage = Math.max(1, page);
        int safePageSize = Math.max(1, pageSize);
        JSONObject response = get("/agent/sessions?page=" + safePage + "&page_size=" + safePageSize);
        return sessionPageFromResponse(response, safePage, safePageSize);
    }

    public JSONArray searchSessions(String keyword) throws Exception {
        return searchSessionsPage(keyword, 1, 10).items;
    }

    public SessionPage searchSessionsPage(String keyword, int page, int pageSize) throws Exception {
        int safePage = Math.max(1, page);
        int safePageSize = Math.max(1, pageSize);
        JSONObject response = get("/agent/sessions/search?q=" + urlEncode(keyword == null ? "" : keyword.trim()) + "&page=" + safePage + "&page_size=" + safePageSize);
        return sessionPageFromResponse(response, safePage, safePageSize);
    }

    private SessionPage sessionPageFromResponse(JSONObject response, int page, int pageSize) {
        JSONArray items = response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
        int currentPage = response.optInt("page", Math.max(1, page));
        int currentPageSize = response.optInt("page_size", pageSize);
        int total = response.optInt("total", items.length());
        boolean hasMore = currentPageSize > 0 && currentPage * currentPageSize < total;
        return new SessionPage(items, currentPage + 1, hasMore, total);
    }


    public JSONObject sessionDetail(String sessionId) throws Exception {
        return get("/agent/sessions/" + urlEncode(sessionId));
    }

    public JSONObject updateSession(String sessionId, String title, String summary) throws Exception {
        JSONObject body = new JSONObject();
        body.put("title", title == null ? "" : title);
        body.put("summary", summary == null ? "" : summary);
        return patch("/agent/sessions/" + urlEncode(sessionId), body);
    }

    public JSONObject summarizeSession(String sessionId) throws Exception {
        return post("/agent/sessions/" + urlEncode(sessionId) + ":summarize", new JSONObject());
    }

    public JSONObject pinSession(String sessionId, boolean pinned) throws Exception {
        JSONObject body = new JSONObject();
        body.put("pinned", pinned);
        return post("/agent/sessions/" + urlEncode(sessionId) + ":pin", body);
    }

    public JSONObject deleteSession(String sessionId) throws Exception {
        HttpURLConnection conn = open("/agent/sessions/" + urlEncode(sessionId), "DELETE");
        return readJSON(conn);
    }

    public JSONObject cancelAgentRun(String runId) throws Exception {
        return post("/agent/runs/" + urlEncode(runId) + ":cancel", new JSONObject());
    }

    public JSONObject createSession(String title) throws Exception {
        JSONObject body = new JSONObject();
        body.put("title", title);
        return post("/agent/sessions", body);
    }

    public JSONObject uploadAttachment(String name, String mimeType, String type, byte[] data) throws Exception {
        if (data == null || data.length == 0) {
            throw new IOException("附件内容为空，无法发送");
        }
        if (data.length > 10 * 1024 * 1024) {
            throw new IOException("单个附件不能超过 10MB");
        }
        String boundary = "----xzxgAndroid" + System.currentTimeMillis();
        byte[] body = attachmentMultipartBody(boundary, name, mimeType, type, data);
        try {
            return uploadAttachmentBody(boundary, body, name, type);
        } catch (Exception error) {
            if (!shouldRetryUpload(error)) {
                throw error;
            }
            return uploadAttachmentBody(boundary, body, name, type);
        }
    }

    private JSONObject uploadAttachmentBody(String boundary, byte[] body, String name, String type) throws Exception {
        HttpURLConnection conn = openRaw("/files", "POST");
        conn.setRequestProperty("Content-Type", "multipart/form-data; boundary=" + boundary);
        conn.setRequestProperty("Connection", "close");
        conn.setFixedLengthStreamingMode(body.length);
        try (OutputStream out = conn.getOutputStream()) {
            out.write(body);
            out.flush();
        } catch (Exception error) {
            conn.disconnect();
            throw error;
        }
        try {
            return fileUploadToAttachment(readJSON(conn), name, type);
        } finally {
            conn.disconnect();
        }
    }

    private byte[] attachmentMultipartBody(String boundary, String name, String mimeType, String type, byte[] data) throws Exception {
        ByteArrayOutputStream body = new ByteArrayOutputStream();
        writeFormField(body, boundary, "type", type == null || type.trim().isEmpty() ? "file" : type.trim());
        body.write(("--" + boundary + "\r\n").getBytes(StandardCharsets.UTF_8));
        body.write(("Content-Disposition: form-data; name=\"file\"; filename=\"" + safeFileName(name) + "\"\r\n").getBytes(StandardCharsets.UTF_8));
        body.write(("Content-Type: " + (mimeType == null || mimeType.isEmpty() ? "application/octet-stream" : mimeType) + "\r\n\r\n").getBytes(StandardCharsets.UTF_8));
        body.write(data);
        body.write(("\r\n--" + boundary + "--\r\n").getBytes(StandardCharsets.UTF_8));
        return body.toByteArray();
    }

    private boolean shouldRetryUpload(Exception error) {
        if (error instanceof SocketException || error instanceof ProtocolException) {
            return true;
        }
        if (error instanceof IOException && error.getMessage() != null) {
            String message = error.getMessage().toLowerCase();
            return message.contains("broken pipe") || message.contains("unexpected end of stream") || message.contains("connection reset");
        }
        return false;
    }

    private JSONObject fileUploadToAttachment(JSONObject response, String name, String type) throws Exception {
        JSONObject file = response == null ? null : response.optJSONObject("file");
        if (file == null) {
            return response == null ? new JSONObject() : response;
        }
        JSONObject attachment = new JSONObject();
        String fileId = file.optString("file_id", "");
        attachment.put("attachment_id", fileId);
        attachment.put("type", type == null || type.trim().isEmpty() ? attachmentTypeFromMime(file.optString("mime_type", "")) : type.trim());
        attachment.put("url", file.optString("url", ""));
        attachment.put("name", name == null || name.trim().isEmpty() ? fileId : name.trim());
        attachment.put("object_key", file.optString("object_key", ""));
        return attachment;
    }

    private String attachmentTypeFromMime(String mimeType) {
        return mimeType != null && mimeType.startsWith("image/") ? "image" : "file";
    }

    private void writeFormField(OutputStream out, String boundary, String name, String value) throws Exception {
        out.write(("--" + boundary + "\r\n").getBytes(StandardCharsets.UTF_8));
        out.write(("Content-Disposition: form-data; name=\"" + name + "\"\r\n\r\n").getBytes(StandardCharsets.UTF_8));
        out.write((value + "\r\n").getBytes(StandardCharsets.UTF_8));
    }

    private String safeFileName(String name) {
        String value = name == null || name.trim().isEmpty() ? "attachment" : name.trim();
        return value.replace("\"", "").replace("\r", "").replace("\n", "");
    }

    public StreamCall streamMessage(String sessionId, String content, SseCallback callback) {
        return streamMessage(sessionId, content, new JSONArray(), callback);
    }

    public StreamCall streamMessage(String sessionId, String content, JSONArray attachments, SseCallback callback) {
        return streamMessage(sessionId, "android_" + System.currentTimeMillis(), content, attachments, callback);
    }

    public StreamCall streamMessage(String sessionId, String clientMessageId, String content, JSONArray attachments, SseCallback callback) {
        StreamCall call = new StreamCall();
        Thread thread = new Thread(() -> {
            HttpURLConnection conn = null;
            try {
                JSONObject body = new JSONObject();
                body.put("client_message_id", clientMessageId == null || clientMessageId.trim().isEmpty() ? "android_" + System.currentTimeMillis() : clientMessageId);
                body.put("content", content);
                body.put("attachments", attachments == null ? new JSONArray() : attachments);
                conn = open("/agent/sessions/" + sessionId + "/messages:stream", "POST");
                call.connection = conn;
                conn.setRequestProperty("Accept", "text/event-stream");
                writeBody(conn, body);

                int code = conn.getResponseCode();
                if (code < 200 || code >= 300) {
                    if (!call.canceled) {
                        callback.onError(apiException(code, readText(conn.getErrorStream())));
                    }
                    return;
                }
                BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream(), StandardCharsets.UTF_8));
                String line;
                while (!call.canceled && (line = reader.readLine()) != null) {
                    if (!line.startsWith("data:")) {
                        continue;
                    }
                    String data = line.substring(5).trim();
                    if (data.isEmpty()) {
                        continue;
                    }
                    try {
                        callback.onEvent(new JSONObject(data));
                    } catch (Exception parseError) {
                        throw new IOException("流式响应解析失败", parseError);
                    }
                }
            } catch (Exception error) {
                if (!call.canceled) {
                    callback.onError(withReadableMessage(error, "流式连接中断"));
                }
            } finally {
                if (conn != null) {
                    conn.disconnect();
                }
            }
        });
        call.thread = thread;
        thread.start();
        return call;
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
        HttpURLConnection conn = openRaw(path, method);
        conn.setRequestProperty("Content-Type", "application/json; charset=utf-8");
        return conn;
    }

    private HttpURLConnection openRaw(String path, String method) throws Exception {
        URL url = new URL(trimSlash(sessionStore.apiBase()) + path);
        HttpURLConnection conn = (HttpURLConnection) url.openConnection();
        conn.setRequestMethod(method);
        conn.setConnectTimeout(10000);
        conn.setReadTimeout(60000);
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
            throw apiException(code, text);
        }
        return text.isEmpty() ? new JSONObject() : new JSONObject(text);
    }

    private ApiException apiException(int statusCode, String text) {
        String message = text == null || text.isEmpty() ? ("HTTP " + statusCode) : text;
        String code = "";
        try {
            JSONObject body = new JSONObject(message);
            code = body.optString("code", "");
            message = body.optString("message", message);
        } catch (Exception ignored) {
        }
        return new ApiException(statusCode, code, message);
    }

    private Exception withReadableMessage(Exception error, String fallback) {
        if (error == null) {
            return new IOException(fallback);
        }
        String message = error.getMessage();
        if (message != null && !message.trim().isEmpty()) {
            return error;
        }
        return new IOException(fallback + "：" + error.getClass().getSimpleName(), error);
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

    private String urlEncode(String value) throws Exception {
        return URLEncoder.encode(value == null ? "" : value, "UTF-8");
    }

    public interface SseCallback {
        void onEvent(JSONObject event);
        void onError(Throwable error);
    }

    public static class ApiException extends RuntimeException {
        public final int statusCode;
        public final String code;

        ApiException(int statusCode, String code, String message) {
            super(message);
            this.statusCode = statusCode;
            this.code = code == null ? "" : code;
        }
    }

    public static class StreamCall {
        private volatile boolean canceled;
        private volatile HttpURLConnection connection;
        private volatile Thread thread;

        public void cancel() {
            canceled = true;
            HttpURLConnection conn = connection;
            if (conn != null) {
                conn.disconnect();
            }
            Thread current = thread;
            if (current != null) {
                current.interrupt();
            }
        }

        public boolean isCanceled() {
            return canceled;
        }
    }

    public static class ProductPage {
        public final JSONArray items;
        public final int nextPage;
        public final boolean hasMore;
        public final int total;

        ProductPage(JSONArray items, int nextPage, boolean hasMore, int total) {
            this.items = items;
            this.nextPage = nextPage;
            this.hasMore = hasMore;
            this.total = total;
        }
    }

    public static class SessionPage {
        public final JSONArray items;
        public final int nextPage;
        public final boolean hasMore;
        public final int total;

        SessionPage(JSONArray items, int nextPage, boolean hasMore, int total) {
            this.items = items;
            this.nextPage = nextPage;
            this.hasMore = hasMore;
            this.total = total;
        }
    }
}
