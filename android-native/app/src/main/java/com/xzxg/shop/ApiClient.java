package com.xzxg.shop;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.HttpURLConnection;
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
        JSONObject response = get("/agent/sessions");
        return response.optJSONArray("items") == null ? new JSONArray() : response.optJSONArray("items");
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

    public JSONObject cancelAgentRun(String runId) throws Exception {
        return post("/agent/runs/" + urlEncode(runId) + ":cancel", new JSONObject());
    }

    public JSONObject createSession(String title) throws Exception {
        JSONObject body = new JSONObject();
        body.put("title", title);
        return post("/agent/sessions", body);
    }

    public JSONObject uploadAttachment(String name, String mimeType, String type, byte[] data) throws Exception {
        String boundary = "----xzxgAndroid" + System.currentTimeMillis();
        HttpURLConnection conn = openRaw("/attachments", "POST");
        conn.setRequestProperty("Content-Type", "multipart/form-data; boundary=" + boundary);
        try (OutputStream out = conn.getOutputStream()) {
            writeFormField(out, boundary, "type", type == null ? "file" : type);
            out.write(("--" + boundary + "\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(("Content-Disposition: form-data; name=\"file\"; filename=\"" + safeFileName(name) + "\"\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(("Content-Type: " + (mimeType == null || mimeType.isEmpty() ? "application/octet-stream" : mimeType) + "\r\n\r\n").getBytes(StandardCharsets.UTF_8));
            out.write(data);
            out.write(("\r\n--" + boundary + "--\r\n").getBytes(StandardCharsets.UTF_8));
        }
        return readJSON(conn);
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
        StreamCall call = new StreamCall();
        Thread thread = new Thread(() -> {
            HttpURLConnection conn = null;
            try {
                JSONObject body = new JSONObject();
                body.put("client_message_id", "android_" + System.currentTimeMillis());
                body.put("content", content);
                body.put("attachments", attachments == null ? new JSONArray() : attachments);
                conn = open("/agent/sessions/" + sessionId + "/messages:stream", "POST");
                call.connection = conn;
                conn.setRequestProperty("Accept", "text/event-stream");
                writeBody(conn, body);

                int code = conn.getResponseCode();
                if (code < 200 || code >= 300) {
                    if (!call.canceled) {
                        callback.onError(new RuntimeException(readText(conn.getErrorStream())));
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
                    callback.onEvent(new JSONObject(data));
                }
            } catch (Exception error) {
                if (!call.canceled) {
                    callback.onError(error);
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

    private String urlEncode(String value) throws Exception {
        return URLEncoder.encode(value == null ? "" : value, "UTF-8");
    }

    public interface SseCallback {
        void onEvent(JSONObject event);
        void onError(Throwable error);
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
}
