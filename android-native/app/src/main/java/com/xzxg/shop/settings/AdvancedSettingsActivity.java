package com.xzxg.shop.settings;

import com.xzxg.shop.BuildConfig;
import com.xzxg.shop.account.LoginActivity;
import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

public class AdvancedSettingsActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        if (sessionStore().token().isEmpty()) {
            startActivity(new Intent(this, LoginActivity.class));
            finish();
            return;
        }
        configureShopSystemBars(BG_COLOR);
        render();
    }

    private void render() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("高级设置"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        LinearLayout feature = ShopUi.panel(this);
        feature.addView(ShopUi.strong(this, "功能设置"));
        CheckBox showReturnChat = new CheckBox(this);
        showReturnChat.setText("侧栏显示“返回聊天”按钮");
        showReturnChat.setTextSize(15);
        showReturnChat.setTextColor(Color.rgb(24, 30, 37));
        showReturnChat.setChecked(sessionStore().showDrawerReturnChat());
        showReturnChat.setOnCheckedChangeListener((buttonView, isChecked) -> sessionStore().saveShowDrawerReturnChat(isChecked));
        feature.addView(showReturnChat, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
        page.addView(feature);

        if (BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            LinearLayout dev = ShopUi.panel(this);
            EditText apiBase = ShopUi.inputField(this, "后端地址", sessionStore().apiBase());
            dev.addView(ShopUi.strong(this, "测试后端"));
            dev.addView(apiBase);
            Button saveApi = ShopUi.primaryButton(this, "保存测试地址");
            saveApi.setOnClickListener(v -> saveApiBaseFromInput(apiBase.getText().toString()));
            addFormButton(dev, saveApi);
            page.addView(dev);
        }

        setContentView(content);
        bindRootSystemBarPadding(content);
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

    private void addFormButton(LinearLayout page, Button button) {
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 48));
        params.setMargins(0, 0, 0, ShopUi.dp(this, 10));
        page.addView(button, params);
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }
}
