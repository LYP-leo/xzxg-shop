package com.xzxg.shop.storage;

import com.xzxg.shop.BuildConfig;

import android.content.Context;
import android.content.SharedPreferences;

public class SessionStore {
    private static final String PREFS = "xzxg_session";
    private final SharedPreferences prefs;

    public SessionStore(Context context) {
        prefs = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE);
    }

    public String token() {
        return prefs.getString("token", "");
    }

    public String role() {
        return prefs.getString("role", "user");
    }

    public String nickname() {
        return prefs.getString("nickname", "");
    }

    public String avatarUrl() {
        return prefs.getString("avatar_url", "");
    }

    public String accountId() {
        return prefs.getString("account_id", "");
    }

    public String username() {
        return prefs.getString("username", "");
    }

    public String phone() {
        return prefs.getString("phone", "");
    }

    public String email() {
        return prefs.getString("email", "");
    }

    public String apiBase() {
        if (!BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            return BuildConfig.DEFAULT_API_BASE;
        }
        return prefs.getString("api_base", BuildConfig.DEFAULT_API_BASE);
    }

    public void saveAuth(String token, String role, String nickname) {
        saveAuth(token, role, nickname, avatarUrl());
    }

    public void saveAuth(String token, String role, String nickname, String avatarUrl) {
        saveAuth(token, role, nickname, avatarUrl, accountId(), username(), phone(), email());
    }

    public void saveAuth(String token, String role, String nickname, String avatarUrl, String accountId, String username, String phone, String email) {
        prefs.edit()
                .putString("token", token == null ? "" : token)
                .putString("role", role == null || role.isEmpty() ? "user" : role)
                .putString("nickname", nickname == null ? "" : nickname)
                .putString("avatar_url", avatarUrl == null ? "" : avatarUrl)
                .putString("account_id", accountId == null ? "" : accountId)
                .putString("username", username == null ? "" : username)
                .putString("phone", phone == null ? "" : phone)
                .putString("email", email == null ? "" : email)
                .apply();
    }

    public void saveProfile(String role, String nickname, String avatarUrl, String accountId, String username, String phone, String email) {
        saveAuth(token(), role, nickname, avatarUrl, accountId, username, phone, email);
    }

    public void saveApiBase(String apiBase) {
        prefs.edit().putString("api_base", apiBase).apply();
    }

    public boolean showDrawerReturnChat() {
        return prefs.getBoolean("show_drawer_return_chat", true);
    }

    public void saveShowDrawerReturnChat(boolean value) {
        prefs.edit().putBoolean("show_drawer_return_chat", value).apply();
    }

    public boolean ttsEnabled() {
        return prefs.getBoolean("chat_tts_enabled", false);
    }

    public void saveTtsEnabled(boolean value) {
        prefs.edit().putBoolean("chat_tts_enabled", value).apply();
    }

    public void clearAuth() {
        prefs.edit()
                .remove("token")
                .remove("role")
                .remove("nickname")
                .remove("avatar_url")
                .remove("account_id")
                .remove("username")
                .remove("phone")
                .remove("email")
                .apply();
    }
}
