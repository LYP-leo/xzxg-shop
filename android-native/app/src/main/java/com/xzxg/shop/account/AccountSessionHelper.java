package com.xzxg.shop.account;

import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.LocalChatStore;
import com.xzxg.shop.storage.SessionStore;

import org.json.JSONArray;
import org.json.JSONObject;

import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

public final class AccountSessionHelper {
    private AccountSessionHelper() {
    }

    public static void saveAccountSession(SessionStore sessionStore, String token, JSONObject account, String fallbackRole, String fallbackName) {
        if (account == null) {
            sessionStore.saveAuth(token, fallbackRole, fallbackName, "");
            return;
        }
        sessionStore.saveAuth(
                token,
                account.optString("role", fallbackRole),
                account.optString("display_name", fallbackName),
                account.optString("avatar_url", ""),
                account.optString("account_id", ""),
                account.optString("username", fallbackName),
                account.optString("phone", ""),
                account.optString("email", "")
        );
    }

    public static void syncRemoteSessions(ApiClient api, LocalChatStore chatStore, SessionStore sessionStore, int pageSize, Runnable onAuthExpired, Runnable onDone) {
        if (sessionStore.token().isEmpty()) {
            if (onDone != null) {
                onDone.run();
            }
            return;
        }
        new Thread(() -> {
            try {
                JSONArray sessions = api.sessionsPage(1, pageSize).items;
                for (int i = 0; i < sessions.length(); i++) {
                    JSONObject item = sessions.optJSONObject(i);
                    if (item == null) {
                        continue;
                    }
                    chatStore.upsertRemoteSession(
                            item.optString("session_id", ""),
                            item.optString("title", "导购会话"),
                            item.optString("summary", ""),
                            remoteSessionTime(item)
                    );
                }
            } catch (Exception error) {
                if (error instanceof ApiClient.ApiException && ((ApiClient.ApiException) error).statusCode == 401 && onAuthExpired != null) {
                    onAuthExpired.run();
                }
            } finally {
                if (onDone != null) {
                    onDone.run();
                }
            }
        }).start();
    }

    private static long remoteSessionTime(JSONObject item) {
        long updated = parseRemoteTime(item.optString("updated_at", ""));
        if (updated > 0) {
            return updated;
        }
        long lastMessage = parseRemoteTime(item.optString("last_message_at", ""));
        if (lastMessage > 0) {
            return lastMessage;
        }
        long created = parseRemoteTime(item.optString("created_at", ""));
        return created > 0 ? created : 1;
    }

    private static long parseRemoteTime(String value) {
        if (value == null || value.trim().isEmpty()) {
            return 0;
        }
        String text = value.trim();
        String[] patterns = {
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                "yyyy-MM-dd'T'HH:mm:ssXXX",
                "yyyy-MM-dd'T'HH:mm:ss'Z'"
        };
        for (String pattern : patterns) {
            try {
                SimpleDateFormat format = new SimpleDateFormat(pattern, Locale.US);
                Date date = format.parse(text);
                if (date != null) {
                    return date.getTime();
                }
            } catch (ParseException ignored) {
            }
        }
        return 0;
    }
}
