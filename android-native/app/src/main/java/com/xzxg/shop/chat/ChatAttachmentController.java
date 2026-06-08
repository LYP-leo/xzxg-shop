package com.xzxg.shop.chat;

import android.content.ContentResolver;
import android.database.Cursor;
import android.net.Uri;
import android.provider.OpenableColumns;
import android.view.View;
import android.widget.LinearLayout;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

public class ChatAttachmentController {
    private final Map<String, List<PendingAttachment>> pendingAttachmentsBySession = new HashMap<>();

    public String sessionKey(String localSessionId) {
        return localSessionId == null || localSessionId.isEmpty() ? "local_default" : localSessionId;
    }

    public List<PendingAttachment> currentPendingAttachments(String localSessionId) {
        String key = sessionKey(localSessionId);
        List<PendingAttachment> list = pendingAttachmentsBySession.get(key);
        if (list == null) {
            list = new ArrayList<>();
            pendingAttachmentsBySession.put(key, list);
        }
        return list;
    }

    public List<PendingAttachment> pendingAttachments(String sessionKey) {
        return pendingAttachmentsBySession.get(sessionKey);
    }

    public JSONArray localPlaceholders(List<PendingAttachment> attachments) {
        JSONArray items = new JSONArray();
        if (attachments == null) {
            return items;
        }
        for (PendingAttachment attachment : attachments) {
            JSONObject item = new JSONObject();
            try {
                item.put("name", attachment.name == null ? "attachment" : attachment.name);
                item.put("mime_type", attachment.mimeType == null ? "application/octet-stream" : attachment.mimeType);
                item.put("type", attachment.type == null ? "file" : attachment.type);
                item.put("size", Math.max(0, attachment.size));
                item.put("local_uri", attachment.uri == null ? "" : attachment.uri.toString());
            } catch (Exception ignored) {
            }
            items.put(item);
        }
        return items;
    }

    public PendingAttachment fromUri(ContentResolver resolver, Uri uri, boolean forceImage) {
        String name = "attachment";
        long size = 0;
        String mime = resolver.getType(uri);
        try (Cursor cursor = resolver.query(uri, null, null, null, null)) {
            if (cursor != null && cursor.moveToFirst()) {
                int nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                int sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE);
                if (nameIndex >= 0) {
                    name = cursor.getString(nameIndex);
                }
                if (sizeIndex >= 0) {
                    size = cursor.getLong(sizeIndex);
                }
            }
        } catch (Exception ignored) {
        }
        String type = forceImage || (mime != null && mime.startsWith("image/")) ? "image" : "file";
        return new PendingAttachment(uri, name, mime == null ? "application/octet-stream" : mime, type, size);
    }

    public byte[] readBytes(ContentResolver resolver, Uri uri) throws Exception {
        try (InputStream in = resolver.openInputStream(uri);
             ByteArrayOutputStream out = new ByteArrayOutputStream()) {
            if (in == null) {
                throw new RuntimeException("无法读取附件");
            }
            byte[] buffer = new byte[8192];
            int read;
            int total = 0;
            while ((read = in.read(buffer)) != -1) {
                total += read;
                if (total > 10 * 1024 * 1024) {
                    throw new RuntimeException("单个附件不能超过 10MB");
                }
                out.write(buffer, 0, read);
            }
            return out.toByteArray();
        }
    }

    public static class PendingAttachment {
        public Uri uri;
        public String name;
        public String mimeType;
        public String type;
        public long size;

        public PendingAttachment(Uri uri, String name, String mimeType, String type, long size) {
            this.uri = uri;
            this.name = name;
            this.mimeType = mimeType;
            this.type = type;
            this.size = size;
        }
    }

    public static class UploadingMessageViewState {
        public LinearLayout container;
        public TextView statusText;
        public final List<View> attachmentViews = new ArrayList<>();
        public boolean failed;
    }
}
