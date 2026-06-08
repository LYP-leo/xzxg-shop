package com.xzxg.shop.coupon;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

public class CouponActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private LinearLayout page;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        render();
    }

    private void render() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("优惠券"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        setContentView(content);
        bindRootSystemBarPadding(content);
        loadCoupons();
    }

    private void loadCoupons() {
        page.removeAllViews();
        if (sessionStore().token().isEmpty()) {
            page.addView(ShopUi.card(this, "请先登录", "登录后可领取和查看优惠券。"));
            return;
        }
        page.addView(ShopUi.muted(this, "正在加载优惠券..."));
        new Thread(() -> {
            try {
                JSONArray available = api().availableCoupons();
                JSONArray mine = api().myCoupons();
                runOnUiThread(() -> renderCouponContent(available, mine));
            } catch (Exception error) {
                runOnUiThread(() -> renderError("优惠券加载失败", error.getMessage()));
            }
        }).start();
    }

    private void renderCouponContent(JSONArray available, JSONArray mine) {
        page.removeAllViews();
        JSONArray safeAvailable = available == null ? new JSONArray() : available;
        JSONArray safeMine = mine == null ? new JSONArray() : mine;
        page.addView(ShopUi.strong(this, "可领取"));
        for (int i = 0; i < safeAvailable.length(); i++) {
            JSONObject coupon = safeAvailable.optJSONObject(i);
            if (coupon != null) {
                page.addView(couponCard(coupon, true));
            }
        }
        page.addView(ShopUi.strong(this, "我的优惠券"));
        if (safeMine.length() == 0) {
            page.addView(ShopUi.muted(this, "暂无已领取优惠券"));
        }
        for (int i = 0; i < safeMine.length(); i++) {
            JSONObject item = safeMine.optJSONObject(i);
            JSONObject coupon = item == null ? null : item.optJSONObject("coupon");
            page.addView(couponCard(coupon == null ? item : coupon, false));
        }
    }

    private View couponCard(JSONObject coupon, boolean claimable) {
        LinearLayout card = ShopUi.panel(this);
        if (coupon == null) {
            card.addView(ShopUi.muted(this, "优惠券信息缺失"));
            return card;
        }
        card.addView(ShopUi.strong(this, coupon.optString("name", "优惠券")));
        card.addView(ShopUi.muted(this, "满 " + coupon.optString("threshold_amount", "0") + " 减 " + coupon.optString("discount_amount", "0")));
        card.addView(ShopUi.muted(this, coupon.optString("start_at", "") + " - " + coupon.optString("end_at", "")));
        if (claimable) {
            Button claim = ShopUi.textOnlyButton(this, "领取");
            claim.setTextColor(Color.rgb(37, 99, 235));
            claim.setOnClickListener(v -> claimCoupon(coupon.optString("coupon_id")));
            card.addView(claim, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 42)));
        }
        return card;
    }

    private void claimCoupon(String couponId) {
        new Thread(() -> {
            try {
                api().claimCoupon(couponId);
                runOnUiThread(() -> {
                    showToastLine("领取成功");
                    loadCoupons();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("领取失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderError(String title, String detail) {
        page.removeAllViews();
        page.addView(ShopUi.card(this, title, detail == null ? "" : detail));
        Button retry = ShopUi.secondaryButton(this, "重试");
        retry.setOnClickListener(v -> loadCoupons());
        page.addView(retry, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }
}
