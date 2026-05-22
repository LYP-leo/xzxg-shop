package com.xzxg.shop;

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
        prefs.edit()
                .putString("token", token == null ? "" : token)
                .putString("role", role == null || role.isEmpty() ? "user" : role)
                .putString("nickname", nickname == null ? "" : nickname)
                .putString("avatar_url", avatarUrl == null ? "" : avatarUrl)
                .apply();
    }

    public void saveApiBase(String apiBase) {
        prefs.edit().putString("api_base", apiBase).apply();
    }

    public void clearAuth() {
        prefs.edit().remove("token").remove("role").remove("nickname").remove("avatar_url").apply();
    }
}
