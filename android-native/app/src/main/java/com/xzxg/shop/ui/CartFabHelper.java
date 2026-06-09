package com.xzxg.shop.ui;

import com.xzxg.shop.cart.CartActivity;
import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.SessionStore;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.FrameLayout;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

public final class CartFabHelper {
    private final Activity activity;
    private final SessionStore sessionStore;
    private final ApiClient api;
    private final FrameLayout root;
    private final FrameLayout wrap;
    private final TextView badge;

    private CartFabHelper(Activity activity, SessionStore sessionStore, ApiClient api, FrameLayout root, FrameLayout wrap, TextView badge) {
        this.activity = activity;
        this.sessionStore = sessionStore;
        this.api = api;
        this.root = root;
        this.wrap = wrap;
        this.badge = badge;
    }

    public static CartFabHelper attach(Activity activity, SessionStore sessionStore, ApiClient api, FrameLayout root, int bottomMarginDp) {
        if (activity == null || sessionStore == null || api == null || root == null || sessionStore.token().isEmpty()) {
            return null;
        }
        FrameLayout fabWrap = new FrameLayout(activity);
        fabWrap.setClipChildren(false);
        fabWrap.setClipToPadding(false);

        FrameLayout fab = new FrameLayout(activity);
        fab.setClickable(true);
        GradientDrawable fabBackground = ShopUi.rounded(Color.WHITE, ShopUi.dp(activity, 28));
        fabBackground.setStroke(ShopUi.dp(activity, 1), Color.rgb(220, 224, 230));
        fab.setBackground(fabBackground);
        fab.setElevation(0);
        fab.setTranslationZ(0);
        fab.setOnClickListener(v -> activity.startActivity(new Intent(activity, CartActivity.class)));

        TextView icon = new TextView(activity);
        icon.setText("购物车");
        icon.setTextSize(13);
        icon.setTypeface(Typeface.DEFAULT_BOLD);
        icon.setTextColor(Color.rgb(17, 24, 39));
        icon.setGravity(Gravity.CENTER);
        fab.addView(icon, new FrameLayout.LayoutParams(ShopUi.dp(activity, 56), ShopUi.dp(activity, 56), Gravity.CENTER));
        fabWrap.addView(fab, new FrameLayout.LayoutParams(ShopUi.dp(activity, 56), ShopUi.dp(activity, 56), Gravity.CENTER));

        TextView badge = new TextView(activity);
        badge.setTextSize(10);
        badge.setTextColor(Color.WHITE);
        badge.setGravity(Gravity.CENTER);
        badge.setTypeface(Typeface.DEFAULT_BOLD);
        badge.setIncludeFontPadding(false);
        badge.setBackground(ShopUi.rounded(Color.rgb(220, 38, 38), ShopUi.dp(activity, 10)));
        badge.setElevation(0);
        badge.setTranslationZ(0);
        badge.setVisibility(View.GONE);
        fabWrap.addView(badge, new FrameLayout.LayoutParams(ShopUi.dp(activity, 30), ShopUi.dp(activity, 20), Gravity.RIGHT | Gravity.TOP));
        badge.bringToFront();

        FrameLayout.LayoutParams fabParams = new FrameLayout.LayoutParams(ShopUi.dp(activity, 72), ShopUi.dp(activity, 72), Gravity.RIGHT | Gravity.BOTTOM);
        fabParams.rightMargin = ShopUi.dp(activity, 18);
        fabParams.bottomMargin = ShopUi.dp(activity, bottomMarginDp);
        root.addView(fabWrap, fabParams);

        CartFabHelper helper = new CartFabHelper(activity, sessionStore, api, root, fabWrap, badge);
        helper.refresh();
        return helper;
    }

    public void refresh() {
        if (sessionStore.token().isEmpty() || badge == null) {
            return;
        }
        new Thread(() -> {
            try {
                JSONObject cart = api.cart();
                int count = cartItemCount(cart);
                activity.runOnUiThread(() -> updateBadge(count));
            } catch (Exception ignored) {
            }
        }).start();
    }

    public void detach() {
        if (wrap.getParent() instanceof ViewGroup) {
            ((ViewGroup) wrap.getParent()).removeView(wrap);
        }
    }

    private void updateBadge(int count) {
        if (activity.isFinishing() || wrap.getParent() == null) {
            return;
        }
        if (count <= 0) {
            badge.setVisibility(View.GONE);
            return;
        }
        badge.setVisibility(View.VISIBLE);
        badge.setText(count > 99 ? "99+" : String.valueOf(count));
    }

    private static int cartItemCount(JSONObject cart) {
        JSONArray items = cart == null ? null : cart.optJSONArray("items");
        int count = 0;
        if (items != null) {
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.optJSONObject(i);
                if (item != null) {
                    count += Math.max(0, item.optInt("quantity", 0));
                }
            }
        }
        return count;
    }
}
