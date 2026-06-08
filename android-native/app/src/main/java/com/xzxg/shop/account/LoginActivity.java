package com.xzxg.shop.account;

import com.xzxg.shop.BuildConfig;
import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.chat.ChatActivity;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.text.InputType;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONObject;

public class LoginActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        render();
    }

    @Override
    public void onBackPressed() {
        openFreshMainActivity();
    }

    private void render() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("登录"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        LinearLayout loginPanel = ShopUi.panel(this);
        loginPanel.addView(ShopUi.strong(this, "账号登录"));
        EditText username = ShopUi.inputField(this, "账号", "user");
        EditText password = ShopUi.inputField(this, "密码", "user123456");
        password.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
        loginPanel.addView(username);
        loginPanel.addView(password);

        Button login = ShopUi.primaryButton(this, "登录");
        login.setOnClickListener(v -> login(username.getText().toString(), password.getText().toString()));
        addFormButton(loginPanel, login);

        Button register = ShopUi.secondaryButton(this, "注册账号");
        register.setOnClickListener(v -> register(username.getText().toString(), password.getText().toString(), "user"));
        addFormButton(loginPanel, register);
        page.addView(loginPanel);

        if (BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            LinearLayout dev = ShopUi.panel(this);
            dev.addView(ShopUi.strong(this, "测试后端"));
            EditText apiBase = ShopUi.inputField(this, "后端地址", sessionStore().apiBase());
            dev.addView(apiBase);
            Button save = ShopUi.secondaryButton(this, "保存测试地址");
            save.setOnClickListener(v -> saveApiBaseFromInput(apiBase.getText().toString()));
            addFormButton(dev, save);
            page.addView(dev);
        }

        setContentView(content);
        bindRootSystemBarPadding(content);
    }

    private void login(String username, String password) {
        new Thread(() -> {
            try {
                JSONObject result = api().login(username, password);
                JSONObject account = result.optJSONObject("account");
                AccountSessionHelper.saveAccountSession(sessionStore(), result.optString("token"), account, "user", username);
                runOnUiThread(() -> {
                    showToastLine("登录成功");
                    AccountSessionHelper.syncRemoteSessions(api(), localChatStore(), sessionStore(), 20, () -> runOnUiThread(() -> sessionStore().clearAuth()), null);
                    openFreshMainActivity();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("登录失败：" + error.getMessage()));
            }
        }).start();
    }

    private void register(String username, String password, String role) {
        new Thread(() -> {
            try {
                String displayName = role.equals("merchant") ? "新商家" : username;
                JSONObject result = api().register(username, password, displayName, role, displayName, "123456");
                JSONObject account = result.optJSONObject("account");
                AccountSessionHelper.saveAccountSession(sessionStore(), result.optString("token"), account, role, displayName);
                runOnUiThread(() -> {
                    showToastLine("注册成功");
                    AccountSessionHelper.syncRemoteSessions(api(), localChatStore(), sessionStore(), 20, () -> runOnUiThread(() -> sessionStore().clearAuth()), null);
                    openFreshMainActivity();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("注册失败：" + error.getMessage()));
            }
        }).start();
    }

    private void saveApiBaseFromInput(String value) {
        String base = value == null ? "" : value.trim();
        while (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        if (!base.contains("/api/v1")) {
            showToastLine("后端地址应包含 /api/v1");
            return;
        }
        sessionStore().saveApiBase(base);
        refreshApiClient();
        showToastLine("已保存测试后端地址");
    }

    private void openFreshMainActivity() {
        Intent intent = new Intent(this, ChatActivity.class);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TASK);
        startActivity(intent);
        finish();
    }

    private void addFormButton(LinearLayout page, Button button) {
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 48));
        params.setMargins(0, 0, 0, ShopUi.dp(this, 10));
        page.addView(button, params);
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, this::openFreshMainActivity);
    }
}
