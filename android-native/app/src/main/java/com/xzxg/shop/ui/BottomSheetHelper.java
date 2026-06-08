package com.xzxg.shop.ui;

import com.xzxg.shop.R;

import android.app.Activity;
import android.app.AlertDialog;
import android.graphics.Color;
import android.graphics.drawable.ColorDrawable;
import android.view.Gravity;
import android.view.View;
import android.view.Window;
import android.view.WindowManager;
import android.widget.LinearLayout;

public final class BottomSheetHelper {
    private BottomSheetHelper() {
    }

    public static LinearLayout box(Activity activity) {
        LinearLayout box = new LinearLayout(activity);
        box.setOrientation(LinearLayout.VERTICAL);
        int horizontal = ShopUi.dp(activity, 18);
        int vertical = ShopUi.dp(activity, 16);
        box.setPadding(horizontal, vertical, horizontal, vertical);
        box.setBackground(ShopUi.rounded(Color.WHITE, ShopUi.dp(activity, 22)));
        return box;
    }

    public static AlertDialog show(Activity activity, View content, boolean cancelable) {
        return show(activity, content, cancelable, true, 0);
    }

    public static AlertDialog show(Activity activity, View content, boolean cancelable, boolean animated, int extraBottomPadding) {
        AlertDialog dialog = new AlertDialog.Builder(activity, R.style.XzxgBottomSheetDialog).create();
        dialog.setCancelable(cancelable);
        dialog.setCanceledOnTouchOutside(cancelable);
        dialog.setView(content, 0, 0, 0, 0);
        dialog.create();
        configureBottomWindow(activity, dialog.getWindow(), animated, extraBottomPadding);
        dialog.setOnShowListener(d -> configureBottomWindow(activity, dialog.getWindow(), animated, extraBottomPadding));
        dialog.show();
        configureBottomWindow(activity, dialog.getWindow(), animated, extraBottomPadding);
        return dialog;
    }

    private static void configureBottomWindow(Activity activity, Window window, boolean animated, int extraBottomPadding) {
        if (window == null) {
            return;
        }
        window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
        window.setDimAmount(0.35f);
        window.addFlags(WindowManager.LayoutParams.FLAG_DIM_BEHIND);
        window.setGravity(Gravity.BOTTOM);
        WindowManager.LayoutParams params = window.getAttributes();
        params.width = WindowManager.LayoutParams.MATCH_PARENT;
        params.height = WindowManager.LayoutParams.WRAP_CONTENT;
        params.gravity = Gravity.BOTTOM;
        window.setAttributes(params);
        window.setWindowAnimations(animated ? R.style.XzxgBottomSheetAnimation : 0);
        window.getDecorView().setPadding(
                ShopUi.dp(activity, 12),
                0,
                ShopUi.dp(activity, 12),
                ShopUi.dp(activity, 12) + Math.max(0, extraBottomPadding)
        );
    }
}
