package com.xzxg.shop;

import android.content.ContentValues;
import android.content.Context;
import android.database.Cursor;
import android.database.sqlite.SQLiteDatabase;
import android.database.sqlite.SQLiteOpenHelper;

import java.util.ArrayList;
import java.util.List;

public class LocalChatStore extends SQLiteOpenHelper {
    public LocalChatStore(Context context) {
        super(context, "xzxg_chat.db", null, 3);
    }

    @Override
    public void onCreate(SQLiteDatabase db) {
        db.execSQL("CREATE TABLE sessions (local_session_id TEXT PRIMARY KEY, server_session_id TEXT, title TEXT, summary TEXT, sync_state TEXT, created_at INTEGER, updated_at INTEGER, pinned_at INTEGER NOT NULL DEFAULT 0, deleted_at INTEGER NOT NULL DEFAULT 0)");
        db.execSQL("CREATE TABLE messages (local_message_id TEXT PRIMARY KEY, local_session_id TEXT, role TEXT, content TEXT, blocks_json TEXT, followups_json TEXT NOT NULL DEFAULT '[]', segments_json TEXT NOT NULL DEFAULT '[]', status TEXT, created_at INTEGER)");
    }

    @Override
    public void onUpgrade(SQLiteDatabase db, int oldVersion, int newVersion) {
        if (oldVersion < 2) {
            addColumnIfMissing(db, "messages", "followups_json", "TEXT NOT NULL DEFAULT '[]'");
        }
        if (oldVersion < 3) {
            addColumnIfMissing(db, "messages", "segments_json", "TEXT NOT NULL DEFAULT '[]'");
            addColumnIfMissing(db, "sessions", "pinned_at", "INTEGER NOT NULL DEFAULT 0");
            addColumnIfMissing(db, "sessions", "deleted_at", "INTEGER NOT NULL DEFAULT 0");
        }
    }

    public void ensureSession(String localSessionId, String title) {
        SQLiteDatabase db = getWritableDatabase();
        ContentValues values = new ContentValues();
        long now = System.currentTimeMillis();
        values.put("local_session_id", localSessionId);
        values.put("title", title);
        values.put("sync_state", "local_only");
        values.put("created_at", now);
        values.put("updated_at", now);
        db.insertWithOnConflict("sessions", null, values, SQLiteDatabase.CONFLICT_IGNORE);
    }

    public void bindServerSession(String localSessionId, String serverSessionId) {
        ContentValues values = new ContentValues();
        values.put("server_session_id", serverSessionId);
        values.put("sync_state", "synced");
        values.put("updated_at", System.currentTimeMillis());
        getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localSessionId});
    }

    public void markSessionSyncState(String localSessionId, String state) {
        ContentValues values = new ContentValues();
        values.put("sync_state", state);
        getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localSessionId});
    }

    public String upsertRemoteSession(String serverSessionId, String title, String summary, long updatedAt) {
        if (serverSessionId == null || serverSessionId.isEmpty()) {
            return "";
        }
        String existing = localSessionIdForServer(serverSessionId);
        String localId = existing.isEmpty() ? "remote_" + serverSessionId : existing;
        String syncState = existing.isEmpty() ? "" : sessionSyncState(localId);
        ContentValues values = new ContentValues();
        values.put("server_session_id", serverSessionId);
        values.put("title", title == null || title.isEmpty() ? "导购会话" : title);
        values.put("summary", summary == null ? "" : summary);
        values.put("sync_state", "synced");
        long remoteTime = updatedAt > 0 ? updatedAt : System.currentTimeMillis();
        if (existing.isEmpty()) {
            values.put("local_session_id", localId);
            values.put("updated_at", remoteTime);
            values.put("created_at", remoteTime);
            getWritableDatabase().insertWithOnConflict("sessions", null, values, SQLiteDatabase.CONFLICT_REPLACE);
        } else {
            long localTime = sessionUpdatedAt(localId);
            boolean hasLocalPendingChanges = "pending_sync".equals(syncState) || "failed".equals(syncState) || "local_only".equals(syncState);
            values.put("updated_at", hasLocalPendingChanges ? Math.max(localTime, remoteTime) : remoteTime);
            getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localId});
        }
        return localId;
    }

    public String localSessionIdForServer(String serverSessionId) {
        Cursor cursor = getReadableDatabase().query("sessions", new String[]{"local_session_id"}, "server_session_id = ?", new String[]{serverSessionId}, null, null, null, "1");
        try {
            return cursor.moveToFirst() ? cursor.getString(0) : "";
        } finally {
            cursor.close();
        }
    }

    public void saveMessage(String localSessionId, String role, String content, String status) {
        saveMessage(localSessionId, role, content, "[]", "[]", "[]", status);
    }

    public void saveAssistantTurn(String localSessionId, String content, String blocksJson, String followupsJson, String status) {
        saveAssistantTurn(localSessionId, content, blocksJson, followupsJson, "[]", status);
    }

    public void saveAssistantTurn(String localSessionId, String content, String blocksJson, String followupsJson, String segmentsJson, String status) {
        saveMessage(localSessionId, "assistant", content, blocksJson, followupsJson, segmentsJson, status);
    }

    public void saveMessage(String localSessionId, String role, String content, String blocksJson, String followupsJson, String status) {
        saveMessage(localSessionId, role, content, blocksJson, followupsJson, "[]", status);
    }

    public void saveMessage(String localSessionId, String role, String content, String blocksJson, String followupsJson, String segmentsJson, String status) {
        ContentValues values = new ContentValues();
        long now = System.currentTimeMillis();
        String safeContent = content == null ? "" : content;
        values.put("local_message_id", "local_msg_" + now + "_" + Math.abs(safeContent.hashCode()));
        values.put("local_session_id", localSessionId);
        values.put("role", role);
        values.put("content", safeContent);
        values.put("blocks_json", blocksJson == null || blocksJson.isEmpty() ? "[]" : blocksJson);
        values.put("followups_json", followupsJson == null || followupsJson.isEmpty() ? "[]" : followupsJson);
        values.put("segments_json", segmentsJson == null || segmentsJson.isEmpty() ? "[]" : segmentsJson);
        values.put("status", status);
        values.put("created_at", now);
        getWritableDatabase().insert("messages", null, values);

        ContentValues sessionValues = new ContentValues();
        sessionValues.put("updated_at", now);
        if ("user".equals(role)) {
            sessionValues.put("title", titleFromMessage(safeContent));
            sessionValues.put("summary", safeContent);
        }
        getWritableDatabase().update("sessions", sessionValues, "local_session_id = ?", new String[]{localSessionId});
    }

    public void saveRemoteMessageSnapshot(String localSessionId, String role, String content, String blocksJson, String followupsJson, String status, long createdAt) {
        saveRemoteMessageSnapshot(localSessionId, role, content, blocksJson, followupsJson, "[]", status, createdAt);
    }

    public void saveRemoteMessageSnapshot(String localSessionId, String role, String content, String blocksJson, String followupsJson, String segmentsJson, String status, long createdAt) {
        if (localSessionId == null || localSessionId.isEmpty()) {
            return;
        }
        String safeRole = role == null || role.trim().isEmpty() ? "user" : role.trim();
        String safeContent = content == null ? "" : content;
        if (safeContent.isEmpty() && isEmptyJsonArray(blocksJson) && isEmptyJsonArray(followupsJson) && isEmptyJsonArray(segmentsJson)) {
            return;
        }
        long messageTime = createdAt > 0 ? createdAt : 1;
        if (remoteMessageSnapshotExists(localSessionId, safeRole, safeContent, messageTime)) {
            return;
        }
        ContentValues values = new ContentValues();
        String fingerprint = localSessionId + "|" + safeRole + "|" + safeContent + "|" + messageTime;
        values.put("local_message_id", "remote_msg_" + Math.abs(fingerprint.hashCode()));
        values.put("local_session_id", localSessionId);
        values.put("role", safeRole);
        values.put("content", safeContent);
        values.put("blocks_json", blocksJson == null || blocksJson.isEmpty() ? "[]" : blocksJson);
        values.put("followups_json", followupsJson == null || followupsJson.isEmpty() ? "[]" : followupsJson);
        values.put("segments_json", segmentsJson == null || segmentsJson.isEmpty() ? "[]" : segmentsJson);
        values.put("status", status == null || status.isEmpty() ? "synced" : status);
        values.put("created_at", messageTime);
        getWritableDatabase().insertWithOnConflict("messages", null, values, SQLiteDatabase.CONFLICT_IGNORE);
    }

    private boolean isEmptyJsonArray(String value) {
        String text = value == null ? "" : value.trim();
        return text.isEmpty() || "[]".equals(text);
    }

    public List<SessionSummary> recentSessions() {
        ArrayList<SessionSummary> items = new ArrayList<>();
        String sql = "SELECT s.local_session_id, s.server_session_id, s.title, s.summary, s.sync_state, s.pinned_at " +
                "FROM sessions s " +
                "WHERE s.deleted_at = 0 AND ((s.server_session_id IS NOT NULL AND s.server_session_id != '') OR EXISTS (SELECT 1 FROM messages m WHERE m.local_session_id = s.local_session_id AND m.role = 'user')) " +
                "ORDER BY CASE WHEN s.pinned_at > 0 THEN 0 ELSE 1 END, s.pinned_at DESC, s.updated_at DESC LIMIT 100";
        Cursor cursor = getReadableDatabase().rawQuery(sql, null);
        try {
            while (cursor.moveToNext()) {
                items.add(new SessionSummary(cursor.getString(0), cursor.getString(1), cursor.getString(2), cursor.getString(3), cursor.getString(4)));
            }
        } finally {
            cursor.close();
        }
        return items;
    }

    public SessionSummary sessionSummary(String localSessionId) {
        Cursor cursor = getReadableDatabase().query("sessions", new String[]{"local_session_id", "server_session_id", "title", "summary", "sync_state", "pinned_at"}, "local_session_id = ? AND deleted_at = 0", new String[]{localSessionId}, null, null, null, "1");
        try {
            return cursor.moveToFirst() ? new SessionSummary(cursor.getString(0), cursor.getString(1), cursor.getString(2), cursor.getString(3), cursor.getString(4), cursor.getLong(5) > 0) : null;
        } finally {
            cursor.close();
        }
    }

    public List<SessionSummary> recentSessionsWithMessages() {
        return recentSessionsWithMessages(50, 0);
    }

    public List<SessionSummary> recentSessionsWithMessages(int limit, int offset) {
        ArrayList<SessionSummary> items = new ArrayList<>();
        int safeLimit = Math.max(1, Math.min(limit, 100));
        int safeOffset = Math.max(0, offset);
        String sql = "SELECT s.local_session_id, s.server_session_id, s.title, s.summary, s.sync_state, s.pinned_at " +
                "FROM sessions s " +
                "WHERE s.deleted_at = 0 AND ((s.server_session_id IS NOT NULL AND s.server_session_id != '') OR EXISTS (SELECT 1 FROM messages m WHERE m.local_session_id = s.local_session_id AND m.role = 'user')) " +
                "ORDER BY CASE WHEN s.pinned_at > 0 THEN 0 ELSE 1 END, s.pinned_at DESC, s.updated_at DESC LIMIT ? OFFSET ?";
        Cursor cursor = getReadableDatabase().rawQuery(sql, new String[]{String.valueOf(safeLimit), String.valueOf(safeOffset)});
        try {
            while (cursor.moveToNext()) {
                items.add(new SessionSummary(cursor.getString(0), cursor.getString(1), cursor.getString(2), cursor.getString(3), cursor.getString(4), cursor.getLong(5) > 0));
            }
        } finally {
            cursor.close();
        }
        return items;
    }

    public List<SessionSummary> searchSessionsLocal(String query) {
        return searchSessionsLocal(query, 50, 0);
    }

    public List<SessionSummary> searchSessionsLocal(String query, int limit, int offset) {
        String keyword = query == null ? "" : query.trim();
        if (keyword.isEmpty()) {
            return recentSessionsWithMessages(limit, offset);
        }
        ArrayList<SessionSummary> items = new ArrayList<>();
        int safeLimit = Math.max(1, Math.min(limit, 100));
        int safeOffset = Math.max(0, offset);
        String like = "%" + keyword + "%";
        String sql = "SELECT DISTINCT s.local_session_id, s.server_session_id, s.title, s.summary, s.sync_state, s.pinned_at, s.updated_at " +
                "FROM sessions s " +
                "LEFT JOIN messages m ON m.local_session_id = s.local_session_id " +
                "WHERE s.deleted_at = 0 AND ((s.server_session_id IS NOT NULL AND s.server_session_id != '') OR EXISTS (SELECT 1 FROM messages um WHERE um.local_session_id = s.local_session_id AND um.role = 'user')) " +
                "AND (s.title LIKE ? OR s.summary LIKE ? OR m.content LIKE ?) " +
                "ORDER BY CASE WHEN s.pinned_at > 0 THEN 0 ELSE 1 END, s.pinned_at DESC, s.updated_at DESC LIMIT ? OFFSET ?";
        Cursor cursor = getReadableDatabase().rawQuery(sql, new String[]{like, like, like, String.valueOf(safeLimit), String.valueOf(safeOffset)});
        try {
            while (cursor.moveToNext()) {
                items.add(new SessionSummary(cursor.getString(0), cursor.getString(1), cursor.getString(2), cursor.getString(3), cursor.getString(4), cursor.getLong(5) > 0));
            }
        } finally {
            cursor.close();
        }
        return items;
    }

    public boolean hasMessages(String localSessionId) {
        Cursor cursor = getReadableDatabase().rawQuery("SELECT 1 FROM messages WHERE local_session_id = ? LIMIT 1", new String[]{localSessionId});
        try {
            return cursor.moveToFirst();
        } finally {
            cursor.close();
        }
    }

    public List<MessageItem> messages(String localSessionId) {
        ArrayList<MessageItem> items = new ArrayList<>();
        Cursor cursor = getReadableDatabase().query("messages", new String[]{"role", "content", "status", "blocks_json", "followups_json", "segments_json"}, "local_session_id = ?", new String[]{localSessionId}, null, null, "created_at ASC");
        try {
            while (cursor.moveToNext()) {
                items.add(new MessageItem(cursor.getString(0), cursor.getString(1), cursor.getString(2), cursor.getString(3), cursor.getString(4), cursor.getString(5)));
            }
        } finally {
            cursor.close();
        }
        return items;
    }

    private void addColumnIfMissing(SQLiteDatabase db, String table, String column, String definition) {
        Cursor cursor = db.rawQuery("PRAGMA table_info(" + table + ")", null);
        try {
            while (cursor.moveToNext()) {
                if (column.equals(cursor.getString(1))) {
                    return;
                }
            }
        } finally {
            cursor.close();
        }
        db.execSQL("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition);
    }

    public void touchSession(String localSessionId) {
        ContentValues values = new ContentValues();
        values.put("updated_at", System.currentTimeMillis());
        getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localSessionId});
    }

    public void pinSession(String localSessionId, boolean pinned) {
        ContentValues values = new ContentValues();
        values.put("pinned_at", pinned ? System.currentTimeMillis() : 0);
        values.put("sync_state", "pending_sync");
        getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localSessionId});
    }

    public void renameSession(String localSessionId, String title) {
        ContentValues values = new ContentValues();
        values.put("title", title == null || title.trim().isEmpty() ? "导购会话" : title.trim());
        values.put("summary", title == null ? "" : title.trim());
        values.put("sync_state", "pending_sync");
        values.put("updated_at", System.currentTimeMillis());
        getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localSessionId});
    }

    public void deleteSession(String localSessionId) {
        ContentValues values = new ContentValues();
        values.put("deleted_at", System.currentTimeMillis());
        values.put("sync_state", "pending_sync");
        getWritableDatabase().update("sessions", values, "local_session_id = ?", new String[]{localSessionId});
    }

    private long sessionUpdatedAt(String localSessionId) {
        Cursor cursor = getReadableDatabase().query("sessions", new String[]{"updated_at"}, "local_session_id = ?", new String[]{localSessionId}, null, null, null, "1");
        try {
            return cursor.moveToFirst() ? cursor.getLong(0) : 0;
        } finally {
            cursor.close();
        }
    }

    private String sessionSyncState(String localSessionId) {
        Cursor cursor = getReadableDatabase().query("sessions", new String[]{"sync_state"}, "local_session_id = ?", new String[]{localSessionId}, null, null, null, "1");
        try {
            return cursor.moveToFirst() ? cursor.getString(0) : "";
        } finally {
            cursor.close();
        }
    }

    private boolean remoteMessageSnapshotExists(String localSessionId, String role, String content, long createdAt) {
        Cursor cursor = getReadableDatabase().query(
                "messages",
                new String[]{"local_message_id"},
                "local_session_id = ? AND role = ? AND content = ? AND created_at = ?",
                new String[]{localSessionId, role, content, String.valueOf(createdAt)},
                null,
                null,
                null,
                "1"
        );
        try {
            return cursor.moveToFirst();
        } finally {
            cursor.close();
        }
    }

    private String titleFromMessage(String content) {
        if (content == null || content.trim().isEmpty()) {
            return "导购会话";
        }
        String value = content.trim().replace('\n', ' ');
        return value.length() > 18 ? value.substring(0, 18) + "..." : value;
    }

    public static class SessionSummary {
        public final String localSessionId;
        public final String serverSessionId;
        public final String title;
        public final String summary;
        public final String syncState;
        public final boolean pinned;

        SessionSummary(String localSessionId, String serverSessionId, String title, String summary, String syncState) {
            this(localSessionId, serverSessionId, title, summary, syncState, false);
        }

        SessionSummary(String localSessionId, String serverSessionId, String title, String summary, String syncState, boolean pinned) {
            this.localSessionId = localSessionId;
            this.serverSessionId = serverSessionId;
            this.title = title;
            this.summary = summary;
            this.syncState = syncState;
            this.pinned = pinned;
        }
    }

    public static class MessageItem {
        public final String role;
        public final String content;
        public final String status;
        public final String blocksJson;
        public final String followupsJson;
        public final String segmentsJson;

        MessageItem(String role, String content, String status, String blocksJson, String followupsJson) {
            this(role, content, status, blocksJson, followupsJson, "[]");
        }

        MessageItem(String role, String content, String status, String blocksJson, String followupsJson, String segmentsJson) {
            this.role = role;
            this.content = content;
            this.status = status;
            this.blocksJson = blocksJson == null ? "[]" : blocksJson;
            this.followupsJson = followupsJson == null ? "[]" : followupsJson;
            this.segmentsJson = segmentsJson == null ? "[]" : segmentsJson;
        }
    }
}
