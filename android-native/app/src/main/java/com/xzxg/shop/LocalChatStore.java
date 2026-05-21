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
        super(context, "xzxg_chat.db", null, 1);
    }

    @Override
    public void onCreate(SQLiteDatabase db) {
        db.execSQL("CREATE TABLE sessions (local_session_id TEXT PRIMARY KEY, server_session_id TEXT, title TEXT, summary TEXT, sync_state TEXT, created_at INTEGER, updated_at INTEGER)");
        db.execSQL("CREATE TABLE messages (local_message_id TEXT PRIMARY KEY, local_session_id TEXT, role TEXT, content TEXT, blocks_json TEXT, status TEXT, created_at INTEGER)");
    }

    @Override
    public void onUpgrade(SQLiteDatabase db, int oldVersion, int newVersion) {
        db.execSQL("DROP TABLE IF EXISTS messages");
        db.execSQL("DROP TABLE IF EXISTS sessions");
        onCreate(db);
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

    public void saveMessage(String localSessionId, String role, String content, String status) {
        ContentValues values = new ContentValues();
        long now = System.currentTimeMillis();
        values.put("local_message_id", "local_msg_" + now + "_" + Math.abs(content.hashCode()));
        values.put("local_session_id", localSessionId);
        values.put("role", role);
        values.put("content", content);
        values.put("blocks_json", "[]");
        values.put("status", status);
        values.put("created_at", now);
        getWritableDatabase().insert("messages", null, values);
    }

    public List<SessionSummary> recentSessions() {
        ArrayList<SessionSummary> items = new ArrayList<>();
        Cursor cursor = getReadableDatabase().query("sessions", new String[]{"local_session_id", "server_session_id", "title", "summary", "sync_state"}, null, null, null, null, "updated_at DESC", "30");
        try {
            while (cursor.moveToNext()) {
                items.add(new SessionSummary(cursor.getString(0), cursor.getString(1), cursor.getString(2), cursor.getString(3), cursor.getString(4)));
            }
        } finally {
            cursor.close();
        }
        return items;
    }

    public static class SessionSummary {
        public final String localSessionId;
        public final String serverSessionId;
        public final String title;
        public final String summary;
        public final String syncState;

        SessionSummary(String localSessionId, String serverSessionId, String title, String summary, String syncState) {
            this.localSessionId = localSessionId;
            this.serverSessionId = serverSessionId;
            this.title = title;
            this.summary = summary;
            this.syncState = syncState;
        }
    }
}
