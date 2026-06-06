package com.xzxg.shop;

import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.util.LruCache;
import android.widget.ImageView;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class ImageLoader {
    private final LruCache<String, Bitmap> cache;
    private final ExecutorService executor = Executors.newFixedThreadPool(3);

    public ImageLoader() {
        int maxMemoryKb = (int) (Runtime.getRuntime().maxMemory() / 1024);
        int cacheSizeKb = Math.max(4 * 1024, maxMemoryKb / 8);
        cache = new LruCache<String, Bitmap>(cacheSizeKb) {
            @Override
            protected int sizeOf(String key, Bitmap value) {
                return value == null ? 0 : value.getByteCount() / 1024;
            }
        };
    }

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
        int targetWidth = target.getWidth();
        int targetHeight = target.getHeight();
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
                    byte[] data = readAllBytes(in);
                    Bitmap bitmap = decodeSampledBitmap(data, targetWidth, targetHeight);
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

    private byte[] readAllBytes(InputStream in) throws Exception {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        byte[] buffer = new byte[8192];
        int read;
        while ((read = in.read(buffer)) != -1) {
            out.write(buffer, 0, read);
        }
        return out.toByteArray();
    }

    private Bitmap decodeSampledBitmap(byte[] data, int targetWidth, int targetHeight) {
        if (data == null || data.length == 0) {
            return null;
        }
        int reqWidth = targetWidth > 0 ? targetWidth : 512;
        int reqHeight = targetHeight > 0 ? targetHeight : 512;
        BitmapFactory.Options bounds = new BitmapFactory.Options();
        bounds.inJustDecodeBounds = true;
        BitmapFactory.decodeByteArray(data, 0, data.length, bounds);

        BitmapFactory.Options options = new BitmapFactory.Options();
        options.inSampleSize = calculateInSampleSize(bounds, reqWidth, reqHeight);
        return BitmapFactory.decodeByteArray(data, 0, data.length, options);
    }

    private int calculateInSampleSize(BitmapFactory.Options options, int reqWidth, int reqHeight) {
        int height = options.outHeight;
        int width = options.outWidth;
        int inSampleSize = 1;
        if (height > reqHeight || width > reqWidth) {
            int halfHeight = height / 2;
            int halfWidth = width / 2;
            while ((halfHeight / inSampleSize) >= reqHeight && (halfWidth / inSampleSize) >= reqWidth) {
                inSampleSize *= 2;
            }
        }
        return Math.max(1, inSampleSize);
    }
}
