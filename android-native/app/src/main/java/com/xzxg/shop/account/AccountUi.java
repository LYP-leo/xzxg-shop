package com.xzxg.shop.account;

import com.xzxg.shop.chat.ChatActivity;
import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.SessionStore;
import com.xzxg.shop.ui.BottomSheetHelper;
import com.xzxg.shop.ui.CircleImageView;
import com.xzxg.shop.ui.ImageLoader;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.Context;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

public final class AccountUi {
    private AccountUi() {
    }

    public static View avatarView(Context context, SessionStore sessionStore, ApiClient api, ImageLoader imageLoader, int sizePx, int textSp) {
        String avatarUrl = api.absoluteUrl(sessionStore.avatarUrl());
        if (!avatarUrl.isEmpty()) {
            ImageView image = new CircleImageView(context);
            image.setScaleType(ImageView.ScaleType.CENTER_CROP);
            image.setBackground(ShopUi.rounded(Color.rgb(236, 244, 255), sizePx / 2));
            imageLoader.load(image, avatarUrl);
            return image;
        }
        TextView avatar = new TextView(context);
        avatar.setText(initialOf(sessionStore.nickname()));
        avatar.setTextSize(textSp);
        avatar.setTypeface(Typeface.DEFAULT_BOLD);
        avatar.setGravity(Gravity.CENTER);
        avatar.setTextColor(Color.rgb(30, 64, 175));
        avatar.setBackground(ShopUi.rounded(Color.rgb(219, 234, 254), sizePx / 2));
        return avatar;
    }

    public static View settingsRow(Context context, String iconText, String label, View.OnClickListener listener, int iconColor) {
        LinearLayout row = new LinearLayout(context);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(ShopUi.dp(context, 4), ShopUi.dp(context, 8), ShopUi.dp(context, 4), ShopUi.dp(context, 8));
        TextView icon = new TextView(context);
        icon.setText(iconText);
        icon.setTextSize(20);
        icon.setTypeface(Typeface.DEFAULT_BOLD);
        icon.setTextColor(Color.WHITE);
        icon.setGravity(Gravity.CENTER);
        icon.setBackground(ShopUi.rounded(iconColor, ShopUi.dp(context, 12)));
        row.addView(icon, new LinearLayout.LayoutParams(ShopUi.dp(context, 44), ShopUi.dp(context, 44)));
        TextView text = ShopUi.strong(context, label);
        LinearLayout.LayoutParams textParams = new LinearLayout.LayoutParams(0, -2, 1);
        textParams.leftMargin = ShopUi.dp(context, 18);
        row.addView(text, textParams);
        TextView arrow = new TextView(context);
        arrow.setText("›");
        arrow.setTextSize(24);
        arrow.setTextColor(Color.rgb(156, 163, 175));
        arrow.setGravity(Gravity.CENTER);
        row.addView(arrow, new LinearLayout.LayoutParams(ShopUi.dp(context, 28), -1));
        row.setOnClickListener(listener);
        return row;
    }

    public static View accountInfoRow(Context context, String iconText, String label, String value, View.OnClickListener listener) {
        LinearLayout row = new LinearLayout(context);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(ShopUi.dp(context, 4), ShopUi.dp(context, 8), ShopUi.dp(context, 4), ShopUi.dp(context, 8));
        TextView icon = new TextView(context);
        icon.setText(iconText);
        icon.setTextSize(18);
        icon.setTextColor(Color.WHITE);
        icon.setGravity(Gravity.CENTER);
        icon.setBackground(ShopUi.rounded(Color.rgb(107, 114, 128), ShopUi.dp(context, 12)));
        row.addView(icon, new LinearLayout.LayoutParams(ShopUi.dp(context, 44), ShopUi.dp(context, 44)));
        TextView text = ShopUi.strong(context, label);
        LinearLayout.LayoutParams textParams = new LinearLayout.LayoutParams(0, -2, 1);
        textParams.leftMargin = ShopUi.dp(context, 18);
        row.addView(text, textParams);
        TextView right = ShopUi.muted(context, value == null || value.isEmpty() ? "未设置  ›" : value + "  ›");
        right.setTextSize(17);
        row.addView(right, new LinearLayout.LayoutParams(-2, -2));
        row.setOnClickListener(listener);
        return row;
    }

    public static View createBackTopBar(Activity activity, String titleText, int backgroundColor, Runnable onBack) {
        return TopBarHelper.backBar(activity, backgroundColor, titleText, onBack);
    }

    public static void showConfirmBottomSheet(Activity activity, String titleText, String message, String confirmText, boolean danger, Runnable onConfirm) {
        LinearLayout box = BottomSheetHelper.box(activity);
        box.addView(ShopUi.title(activity, titleText), new LinearLayout.LayoutParams(-1, -2));
        TextView body = ShopUi.muted(activity, message);
        body.setTextSize(15);
        body.setPadding(0, ShopUi.dp(activity, 8), 0, ShopUi.dp(activity, 18));
        box.addView(body, new LinearLayout.LayoutParams(-1, -2));
        LinearLayout actions = new LinearLayout(activity);
        Button cancel = ShopUi.secondaryButton(activity, "取消");
        Button confirm = ShopUi.primaryButton(activity, confirmText);
        if (danger) {
            confirm.setBackground(ShopUi.rounded(Color.rgb(220, 38, 38), ShopUi.dp(activity, 24)));
        }
        final AlertDialog[] dialogRef = new AlertDialog[1];
        cancel.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
        });
        confirm.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
            if (onConfirm != null) {
                onConfirm.run();
            }
        });
        actions.addView(cancel, new LinearLayout.LayoutParams(0, ShopUi.dp(activity, 48), 1));
        LinearLayout.LayoutParams confirmParams = new LinearLayout.LayoutParams(0, ShopUi.dp(activity, 48), 1);
        confirmParams.leftMargin = ShopUi.dp(activity, 10);
        actions.addView(confirm, confirmParams);
        box.addView(actions, new LinearLayout.LayoutParams(-1, -2));
        dialogRef[0] = BottomSheetHelper.show(activity, box, true);
    }

    public static void openFreshMainActivity(Activity activity) {
        Intent intent = new Intent(activity, ChatActivity.class);
        intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TASK);
        activity.startActivity(intent);
        activity.finish();
    }

    public static String initialOf(String name) {
        String value = name == null || name.trim().isEmpty() ? "我" : name.trim();
        return value.substring(0, 1);
    }

    public static String emptyFallback(String value, String fallback) {
        return value == null || value.trim().isEmpty() ? fallback : value.trim();
    }

    public static String maskPhone(String phone) {
        String value = phone == null ? "" : phone.trim();
        if (value.length() < 7) {
            return value;
        }
        return value.substring(0, 3) + "******" + value.substring(value.length() - 2);
    }

    public static String maskEmail(String email) {
        String value = email == null ? "" : email.trim();
        int at = value.indexOf("@");
        if (at <= 1) {
            return value;
        }
        return value.substring(0, 1) + "***" + value.substring(at);
    }
}
