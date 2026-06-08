package com.xzxg.shop.settings;

import com.xzxg.shop.BuildConfig;
import com.xzxg.shop.account.AccountActivity;
import com.xzxg.shop.account.AccountUi;
import com.xzxg.shop.account.AvatarPreviewActivity;
import com.xzxg.shop.account.EditProfileActivity;
import com.xzxg.shop.account.LoginActivity;
import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.ShopUi;

import android.content.Intent;
import android.graphics.Color;
import android.os.Bundle;
import android.view.Gravity;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

public class SettingsActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (sessionStore().token().isEmpty()) {
            startActivity(new Intent(this, LoginActivity.class));
            finish();
            return;
        }
        render();
    }

    private void render() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(AccountUi.createBackTopBar(this, "设置", BG_COLOR, null), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        LinearLayout header = new LinearLayout(this);
        header.setOrientation(LinearLayout.VERTICAL);
        header.setGravity(Gravity.CENTER_HORIZONTAL);
        header.setPadding(0, ShopUi.dp(this, 36), 0, ShopUi.dp(this, 24));
        android.view.View avatar = AccountUi.avatarView(this, sessionStore(), api(), imageLoader(), ShopUi.dp(this, 118), 36);
        avatar.setOnClickListener(v -> startActivity(new Intent(this, AvatarPreviewActivity.class)));
        header.addView(avatar, new LinearLayout.LayoutParams(ShopUi.dp(this, 118), ShopUi.dp(this, 118)));
        TextView name = ShopUi.title(this, sessionStore().nickname().isEmpty() ? "用户名" : sessionStore().nickname());
        name.setGravity(Gravity.CENTER);
        name.setOnClickListener(v -> startActivity(new Intent(this, EditProfileActivity.class)));
        header.addView(name, new LinearLayout.LayoutParams(-1, -2));
        TextView id = ShopUi.muted(this, "用户id：" + AccountUi.emptyFallback(sessionStore().accountId(), "-"));
        id.setGravity(Gravity.CENTER);
        header.addView(id, new LinearLayout.LayoutParams(-1, -2));
        Button account = ShopUi.secondaryButton(this, "账号管理");
        account.setTextSize(18);
        account.setOnClickListener(v -> startActivity(new Intent(this, AccountActivity.class)));
        LinearLayout.LayoutParams accountParams = new LinearLayout.LayoutParams(ShopUi.dp(this, 168), ShopUi.dp(this, 52));
        accountParams.topMargin = ShopUi.dp(this, 16);
        header.addView(account, accountParams);
        page.addView(header);

        LinearLayout group = ShopUi.panel(this);
        group.addView(AccountUi.settingsRow(this, "?", "帮助", v -> startActivity(new Intent(this, HelpActivity.class)), Color.rgb(124, 58, 237)));
        group.addView(AccountUi.settingsRow(this, "i", "关于", v -> startActivity(new Intent(this, AboutActivity.class)), Color.rgb(59, 130, 246)));
        group.addView(AccountUi.settingsRow(this, "⚙", "高级设置", v -> startActivity(new Intent(this, AdvancedSettingsActivity.class)), Color.rgb(37, 99, 235)));
        group.addView(AccountUi.settingsRow(this, "→", "退出登录", v -> confirmLogout(), Color.rgb(239, 68, 68)));
        page.addView(group);

        TextView version = ShopUi.muted(this, "Version: " + BuildConfig.VERSION_NAME + "\n由 AI 大模型提供支持");
        version.setGravity(Gravity.CENTER);
        page.addView(version, new LinearLayout.LayoutParams(-1, -2));

        setContentView(content);
        bindRootSystemBarPadding(content);
    }

    private void confirmLogout() {
        AccountUi.showConfirmBottomSheet(this, "退出登录", "是否退出当前账号？", "退出", false, this::logoutAccount);
    }

    private void logoutAccount() {
        new Thread(() -> {
            try {
                api().logout();
            } catch (Exception ignored) {
            }
            sessionStore().clearAuth();
            runOnUiThread(() -> {
                showToastLine("已退出登录");
                AccountUi.openFreshMainActivity(this);
            });
        }).start();
    }
}
