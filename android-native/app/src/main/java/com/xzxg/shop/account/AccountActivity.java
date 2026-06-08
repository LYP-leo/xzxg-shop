package com.xzxg.shop.account;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.BottomSheetHelper;
import com.xzxg.shop.ui.ShopUi;

import android.app.AlertDialog;
import android.content.Intent;
import android.graphics.Color;
import android.os.Bundle;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;
import android.util.Patterns;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONObject;

import java.util.regex.Pattern;

public class AccountActivity extends BaseShopActivity {
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
        content.addView(AccountUi.createBackTopBar(this, "账号管理", BG_COLOR, null), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        LinearLayout user = ShopUi.panel(this);
        LinearLayout userRow = new LinearLayout(this);
        userRow.setGravity(Gravity.CENTER_VERTICAL);
        userRow.addView(AccountUi.avatarView(this, sessionStore(), api(), imageLoader(), ShopUi.dp(this, 78), 28), new LinearLayout.LayoutParams(ShopUi.dp(this, 78), ShopUi.dp(this, 78)));
        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        info.addView(ShopUi.title(this, sessionStore().nickname().isEmpty() ? "用户名" : sessionStore().nickname()));
        info.addView(ShopUi.muted(this, "用户id：" + AccountUi.emptyFallback(sessionStore().accountId(), "-")));
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = ShopUi.dp(this, 18);
        userRow.addView(info, infoParams);
        user.addView(userRow);
        page.addView(user);

        LinearLayout group1 = ShopUi.panel(this);
        group1.addView(AccountUi.settingsRow(this, "▦", "个人资料", v -> startActivity(new Intent(this, EditProfileActivity.class)), Color.rgb(124, 58, 237)));
        page.addView(group1);

        LinearLayout group2 = ShopUi.panel(this);
        group2.addView(AccountUi.accountInfoRow(this, "☎", "手机号", AccountUi.maskPhone(sessionStore().phone()), v -> editContact(true)));
        group2.addView(AccountUi.accountInfoRow(this, "@", "邮箱", AccountUi.maskEmail(sessionStore().email()), v -> editContact(false)));
        page.addView(group2);

        LinearLayout group3 = ShopUi.panel(this);
        group3.addView(AccountUi.settingsRow(this, "×", "删除账号", v -> confirmDeleteAccount(), Color.rgb(239, 68, 68)));
        group3.addView(AccountUi.settingsRow(this, "→", "退出登录", v -> confirmLogout(), Color.rgb(239, 68, 68)));
        page.addView(group3);

        setContentView(content);
        bindRootSystemBarPadding(content);
    }

    private void editContact(boolean phone) {
        LinearLayout box = BottomSheetHelper.box(this);
        final AlertDialog[] dialogRef = new AlertDialog[1];

        TextView titleView = ShopUi.title(this, phone ? "手机号设置" : "邮箱设置");
        titleView.setTextSize(20);
        box.addView(titleView, new LinearLayout.LayoutParams(-1, -2));

        TextView subtitle = ShopUi.muted(this, phone ? "用于账号安全验证和订单通知，可留空。" : "用于接收账号通知和订单信息，可留空。");
        subtitle.setPadding(0, ShopUi.dp(this, 6), 0, ShopUi.dp(this, 14));
        box.addView(subtitle, new LinearLayout.LayoutParams(-1, -2));

        EditText edit = ShopUi.inputField(this, phone ? "请输入手机号" : "请输入邮箱", phone ? sessionStore().phone() : sessionStore().email());
        edit.setInputType(phone ? InputType.TYPE_CLASS_PHONE : InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS);
        box.addView(edit);

        TextView errorView = new TextView(this);
        errorView.setTextSize(13);
        errorView.setTextColor(Color.rgb(220, 38, 38));
        errorView.setVisibility(View.GONE);
        LinearLayout.LayoutParams errorParams = new LinearLayout.LayoutParams(-1, -2);
        errorParams.setMargins(ShopUi.dp(this, 2), 0, ShopUi.dp(this, 2), ShopUi.dp(this, 14));
        box.addView(errorView, errorParams);

        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        Button cancel = ShopUi.secondaryButton(this, "取消");
        Button save = ShopUi.primaryButton(this, "保存");
        cancel.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
        });
        save.setOnClickListener(v -> {
            String value = normalizedContactValue(phone, edit.getText().toString());
            String error = contactValidationError(phone, value);
            if (!error.isEmpty()) {
                errorView.setText(error);
                errorView.setVisibility(View.VISIBLE);
                return;
            }
            save.setEnabled(false);
            save.setAlpha(0.45f);
            String newPhone = phone ? value : sessionStore().phone();
            String newEmail = phone ? sessionStore().email() : value;
            updateContact(newPhone, newEmail, dialogRef[0], save);
        });
        actions.addView(cancel, new LinearLayout.LayoutParams(ShopUi.dp(this, 92), ShopUi.dp(this, 44)));
        LinearLayout.LayoutParams saveParams = new LinearLayout.LayoutParams(ShopUi.dp(this, 92), ShopUi.dp(this, 44));
        saveParams.leftMargin = ShopUi.dp(this, 10);
        actions.addView(save, saveParams);
        box.addView(actions, new LinearLayout.LayoutParams(-1, -2));

        TextWatcher watcher = new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            @Override public void onTextChanged(CharSequence s, int start, int before, int count) {
                String error = contactValidationError(phone, normalizedContactValue(phone, s.toString()));
                errorView.setText(error);
                errorView.setVisibility(error.isEmpty() ? View.GONE : View.VISIBLE);
                save.setEnabled(error.isEmpty());
                save.setAlpha(error.isEmpty() ? 1f : 0.45f);
            }
            @Override public void afterTextChanged(Editable s) {}
        };
        edit.addTextChangedListener(watcher);
        watcher.onTextChanged(edit.getText(), 0, 0, 0);

        dialogRef[0] = BottomSheetHelper.show(this, box, true);
    }

    private void updateContact(String phone, String email, AlertDialog dialog, Button saveButton) {
        new Thread(() -> {
            try {
                JSONObject profile = api().updateContact(phone, email);
                AccountSessionHelper.saveAccountSession(sessionStore(), sessionStore().token(), profile, sessionStore().role(), sessionStore().nickname());
                runOnUiThread(() -> {
                    if (dialog != null) {
                        dialog.dismiss();
                    }
                    showToastLine("账号信息已更新");
                    render();
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (saveButton != null) {
                        saveButton.setEnabled(true);
                        saveButton.setAlpha(1f);
                    }
                    showToastLine("保存失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private String normalizedContactValue(boolean phone, String value) {
        String raw = value == null ? "" : value.trim();
        return phone ? raw.replaceAll("[\\s-]", "") : raw;
    }

    private String contactValidationError(boolean phone, String value) {
        String normalized = value == null ? "" : value.trim();
        if (normalized.isEmpty()) {
            return "";
        }
        if (phone) {
            return Pattern.matches("^1[3-9]\\d{9}$", normalized) ? "" : "请输入 11 位有效手机号";
        }
        return Patterns.EMAIL_ADDRESS.matcher(normalized).matches() ? "" : "请输入有效邮箱地址";
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

    private void confirmDeleteAccount() {
        AccountUi.showConfirmBottomSheet(this, "删除账号", "删除后将无法继续使用当前账号登录。是否继续？", "删除", true, this::deleteAccount);
    }

    private void deleteAccount() {
        new Thread(() -> {
            try {
                api().deleteAccount();
                sessionStore().clearAuth();
                runOnUiThread(() -> {
                    showToastLine("账号已删除");
                    AccountUi.openFreshMainActivity(this);
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("删除失败：" + error.getMessage()));
            }
        }).start();
    }
}
