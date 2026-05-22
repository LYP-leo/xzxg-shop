package com.xzxg.shop;

import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.widget.ImageView;

import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class ImageLoader {
    private final Map<String, Bitmap> cache = new ConcurrentHashMap<>();
    private final ExecutorService executor = Executors.newFixedThreadPool(3);

    public void load(ImageView target, String url) {
        if (url == null || url.trim().isEmpty()) {
            return;
        }
        String key = url.trim();
        target.setTag(key);
        Bitmap cached = cache.get(key);
        if (cached != null) {
            target.setImageBitmap(cached);
            return;
        }
        executor.execute(() -> {
            HttpURLConnection conn = null;
            try {
                conn = (HttpURLConnection) new URL(key).openConnection();
                conn.setConnectTimeout(8000);
                conn.setReadTimeout(12000);
                conn.setRequestProperty("User-Agent", "xzxg-android");
                int code = conn.getResponseCode();
                if (code < 200 || code >= 300) {
                    return;
                }
                try (InputStream in = conn.getInputStream()) {
                    Bitmap bitmap = BitmapFactory.decodeStream(in);
                    if (bitmap == null) {
                        return;
                    }
                    cache.put(key, bitmap);
                    target.post(() -> {
                        Object tag = target.getTag();
                        if (key.equals(tag)) {
                            target.setImageBitmap(bitmap);
                        }
                    });
                }
            } catch (Exception ignored) {
            } finally {
                if (conn != null) {
                    conn.disconnect();
                }
            }
        });
    }
}
