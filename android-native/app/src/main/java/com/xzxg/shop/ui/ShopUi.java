package com.xzxg.shop.ui;

import android.content.Context;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.view.Gravity;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.TextView;

public final class ShopUi {
    private ShopUi() {
    }

    public static int dp(Context context, int value) {
        return (int) (value * context.getResources().getDisplayMetrics().density + 0.5f);
    }

    public static GradientDrawable rounded(int color, int radius) {
        GradientDrawable drawable = new GradientDrawable();
        drawable.setColor(color);
        drawable.setCornerRadius(radius);
        return drawable;
    }

    public static void flattenButton(Button button) {
        button.setStateListAnimator(null);
        button.setElevation(0);
    }

    public static Button iconButton(Context context, String text) {
        Button button = new Button(context);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(22);
        button.setTextColor(Color.BLACK);
        button.setBackground(rounded(Color.WHITE, dp(context, 28)));
        return button;
    }

    public static Button transparentIconButton(Context context, String text) {
        Button button = new Button(context);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(24);
        button.setTextColor(Color.BLACK);
        button.setPadding(0, 0, 0, 0);
        button.setMinWidth(0);
        button.setMinHeight(0);
        button.setBackground(new ColorDrawable(Color.TRANSPARENT));
        return button;
    }

    public static Button darkRoundButton(Context context, String text) {
        Button button = new Button(context);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(20);
        button.setTextColor(Color.WHITE);
        button.setPadding(0, 0, 0, 0);
        button.setMinWidth(0);
        button.setMinHeight(0);
        button.setBackground(rounded(Color.BLACK, dp(context, 24)));
        return button;
    }

    public static Button textOnlyButton(Context context, String text) {
        Button button = new Button(context);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.BLACK);
        button.setTextSize(15);
        button.setPadding(0, 0, 0, 0);
        button.setMinWidth(0);
        button.setMinHeight(0);
        button.setBackground(new ColorDrawable(Color.TRANSPARENT));
        return button;
    }

    public static Button primaryButton(Context context, String text) {
        Button button = new Button(context);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.WHITE);
        button.setTextSize(16);
        button.setBackground(rounded(Color.BLACK, dp(context, 24)));
        return button;
    }

    public static Button secondaryButton(Context context, String text) {
        Button button = new Button(context);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.BLACK);
        button.setTextSize(15);
        button.setBackground(rounded(Color.rgb(238, 239, 241), dp(context, 22)));
        return button;
    }

    public static LinearLayout panel(Context context) {
        LinearLayout view = new LinearLayout(context);
        view.setOrientation(LinearLayout.VERTICAL);
        view.setPadding(dp(context, 14), dp(context, 12), dp(context, 14), dp(context, 12));
        view.setBackground(rounded(Color.WHITE, dp(context, 12)));
        view.setElevation(dp(context, 1));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
        params.setMargins(0, 0, 0, dp(context, 12));
        view.setLayoutParams(params);
        return view;
    }

    public static TextView title(Context context, String text) {
        TextView view = new TextView(context);
        view.setText(text);
        view.setTextSize(22);
        view.setTypeface(Typeface.DEFAULT_BOLD);
        view.setTextColor(Color.rgb(20, 24, 30));
        return view;
    }

    public static TextView strong(Context context, String text) {
        TextView view = new TextView(context);
        view.setText(text);
        view.setTextSize(16);
        view.setTypeface(Typeface.DEFAULT_BOLD);
        view.setTextColor(Color.rgb(17, 24, 39));
        view.setPadding(0, dp(context, 4), 0, dp(context, 4));
        return view;
    }

    public static TextView priceStrong(Context context, String text) {
        TextView view = strong(context, text);
        view.setTextColor(Color.rgb(220, 38, 38));
        return view;
    }

    public static TextView muted(Context context, String text) {
        TextView view = new TextView(context);
        view.setText(text);
        view.setTextSize(13);
        view.setTextColor(Color.rgb(107, 114, 128));
        view.setPadding(0, dp(context, 8), 0, dp(context, 8));
        return view;
    }

    public static TextView card(Context context, String title, String detail) {
        TextView view = new TextView(context);
        view.setText(title + "\n" + detail);
        view.setTextSize(14);
        view.setTextColor(Color.rgb(24, 30, 37));
        view.setPadding(dp(context, 14), dp(context, 12), dp(context, 14), dp(context, 12));
        view.setBackground(rounded(Color.WHITE, dp(context, 12)));
        view.setElevation(dp(context, 1));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
        params.setMargins(0, 0, 0, dp(context, 12));
        view.setLayoutParams(params);
        return view;
    }

    public static EditText inputField(Context context, String hint, String value) {
        EditText editText = new EditText(context);
        editText.setHint(hint);
        editText.setText(value);
        editText.setSingleLine(true);
        editText.setPadding(dp(context, 14), dp(context, 10), dp(context, 14), dp(context, 10));
        editText.setBackground(rounded(Color.WHITE, dp(context, 12)));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(context, 52));
        params.setMargins(0, 0, 0, dp(context, 12));
        editText.setLayoutParams(params);
        return editText;
    }

    public static TextView topBarTextButton(Context context, String text, int color) {
        TextView view = new TextView(context);
        view.setText(text);
        view.setTextSize(17);
        view.setTextColor(color);
        view.setGravity(Gravity.CENTER_VERTICAL);
        view.setIncludeFontPadding(false);
        view.setPadding(0, 0, 0, 0);
        view.setClickable(true);
        return view;
    }
}
