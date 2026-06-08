package com.xzxg.shop.ui;

import com.xzxg.shop.navigation.NavigationHelper;
import com.xzxg.shop.navigation.Routes;

import android.app.Activity;
import android.graphics.Color;
import android.graphics.Typeface;
import android.view.Gravity;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

public final class MainNavigationDrawer {
    private MainNavigationDrawer() {
    }

    public static void show(Activity activity, FrameLayout root, String activeRoute) {
        if (activity == null || root == null) {
            return;
        }
        FrameLayout layer = new FrameLayout(activity);
        layer.setBackgroundColor(Color.argb(92, 0, 0, 0));
        layer.setAlpha(0f);

        LinearLayout panel = new LinearLayout(activity);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setPadding(ShopUi.dp(activity, 14), ShopUi.dp(activity, 24), ShopUi.dp(activity, 14), ShopUi.dp(activity, 14));
        panel.setBackgroundColor(Color.WHITE);

        TextView title = new TextView(activity);
        title.setText("小猪小狗导购");
        title.setTextSize(18);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        title.setTextColor(Color.rgb(17, 24, 39));
        title.setGravity(Gravity.CENTER_VERTICAL);
        title.setPadding(ShopUi.dp(activity, 8), 0, 0, 0);
        panel.addView(title, new LinearLayout.LayoutParams(-1, ShopUi.dp(activity, 56)));

        LinearLayout navGroup = new LinearLayout(activity);
        navGroup.setOrientation(LinearLayout.VERTICAL);
        navGroup.setPadding(0, ShopUi.dp(activity, 10), 0, ShopUi.dp(activity, 10));
        navGroup.addView(navRow(activity, "💬", "AI导购", Routes.CHAT, activeRoute, layer, root));
        navGroup.addView(navRow(activity, "🏷", "商品", Routes.PRODUCTS, activeRoute, layer, root));
        navGroup.addView(navRow(activity, "🛒", "购物车", Routes.CART, activeRoute, layer, root));
        navGroup.addView(navRow(activity, "📦", "订单", Routes.ORDERS, activeRoute, layer, root));
        panel.addView(navGroup, new LinearLayout.LayoutParams(-1, -2));

        View spacer = new View(activity);
        panel.addView(spacer, new LinearLayout.LayoutParams(-1, 0, 1));

        TextView user = new TextView(activity);
        user.setText("小猪");
        user.setTextSize(16);
        user.setTypeface(Typeface.DEFAULT_BOLD);
        user.setTextColor(Color.rgb(31, 41, 55));
        user.setGravity(Gravity.CENTER_VERTICAL);
        user.setPadding(ShopUi.dp(activity, 8), 0, 0, 0);
        panel.addView(user, new LinearLayout.LayoutParams(-1, ShopUi.dp(activity, 56)));

        int panelWidth = (int) (activity.getResources().getDisplayMetrics().widthPixels * 0.82f);
        panel.setTranslationX(-panelWidth);
        layer.setOnClickListener(v -> close(root, layer, panel));
        panel.setOnClickListener(v -> {});
        layer.addView(panel, new FrameLayout.LayoutParams(panelWidth, -1, Gravity.LEFT | Gravity.TOP));
        root.addView(layer, new FrameLayout.LayoutParams(-1, -1));
        layer.animate().alpha(1f).setDuration(160).start();
        panel.animate().translationX(0f).setDuration(180).start();
    }

    private static View navRow(Activity activity, String icon, String label, String route, String activeRoute, FrameLayout layer, FrameLayout root) {
        LinearLayout row = new LinearLayout(activity);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(ShopUi.dp(activity, 14), 0, ShopUi.dp(activity, 14), 0);
        boolean active = route.equals(activeRoute);
        row.setBackground(ShopUi.rounded(active ? Color.rgb(243, 244, 246) : Color.WHITE, ShopUi.dp(activity, 4)));
        row.setClickable(true);

        TextView iconView = new TextView(activity);
        iconView.setText(icon);
        iconView.setTextSize(22);
        iconView.setGravity(Gravity.CENTER);
        row.addView(iconView, new LinearLayout.LayoutParams(ShopUi.dp(activity, 44), ShopUi.dp(activity, 56)));

        TextView text = new TextView(activity);
        text.setText(label);
        text.setTextSize(18);
        text.setTextColor(Color.rgb(17, 24, 39));
        text.setTypeface(active ? Typeface.DEFAULT_BOLD : Typeface.DEFAULT);
        text.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(text, new LinearLayout.LayoutParams(0, -1, 1));

        row.setOnClickListener(v -> {
            close(root, layer, (LinearLayout) row.getParent().getParent());
            if (!active) {
                NavigationHelper.navigateMainRoute(activity, route);
            }
        });
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, ShopUi.dp(activity, 56));
        params.bottomMargin = ShopUi.dp(activity, 4);
        row.setLayoutParams(params);
        return row;
    }

    private static void close(FrameLayout root, FrameLayout layer, LinearLayout panel) {
        if (root == null || layer == null || panel == null) {
            return;
        }
        int panelWidth = panel.getWidth();
        panel.animate().translationX(-panelWidth).setDuration(150).start();
        layer.animate().alpha(0f).setDuration(150).withEndAction(() -> root.removeView(layer)).start();
    }
}
