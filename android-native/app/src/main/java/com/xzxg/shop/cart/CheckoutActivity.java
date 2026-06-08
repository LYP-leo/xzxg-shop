package com.xzxg.shop.cart;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.chat.ChatActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.text.TextUtils;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

public class CheckoutActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private LinearLayout content;
    private String draftId;
    private JSONObject cartSnapshot;
    private JSONObject discountPreview;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        draftId = getIntent() == null ? "" : getIntent().getStringExtra(Routes.EXTRA_CHECKOUT_DRAFT_ID);
        cartSnapshot = CheckoutDraftStore.get(draftId);
        if (cartSnapshot == null) {
            renderExpiredDraft();
            return;
        }
        loadDiscountPreview();
    }

    private void loadDiscountPreview() {
        new Thread(() -> {
            try {
                JSONObject preview = api().discountPreview();
                runOnUiThread(() -> {
                    discountPreview = preview;
                    renderCheckoutPage();
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    discountPreview = null;
                    renderCheckoutPage();
                });
            }
        }).start();
    }

    private void renderExpiredDraft() {
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(BG_COLOR);
        root.addView(createBackTopBar("确认订单"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        page.addView(ShopUi.card(this, "订单信息已失效", "请返回购物车重新结算。"));
        Button back = ShopUi.primaryButton(this, "返回购物车");
        back.setOnClickListener(v -> {
            startActivity(new Intent(this, CartActivity.class));
            finish();
        });
        page.addView(back, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
        root.addView(page, new LinearLayout.LayoutParams(-1, 0, 1));
        setContentView(root);
        bindRootSystemBarPadding(root);
    }

    private void renderCheckoutPage() {
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("确认订单"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));
        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 96));
        scroll.addView(page);
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        renderItemPanel(page);
        renderAmountPanel(page);
        renderBottomBar();

        setContentView(content);
        bindRootSystemBarPadding(content);
    }

    private void renderItemPanel(LinearLayout page) {
        JSONArray items = cartSnapshot == null ? null : cartSnapshot.optJSONArray("items");
        if (items == null) {
            items = new JSONArray();
        }
        LinearLayout itemPanel = ShopUi.panel(this);
        itemPanel.addView(ShopUi.strong(this, "商品清单"));
        int selectedCount = 0;
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item == null || !item.optBoolean("selected")) {
                continue;
            }
            selectedCount++;
            LinearLayout row = new LinearLayout(this);
            row.setGravity(Gravity.CENTER_VERTICAL);
            row.addView(productImage(item.optString("imageUrl", ""), 48), new LinearLayout.LayoutParams(ShopUi.dp(this, 48), ShopUi.dp(this, 48)));
            LinearLayout info = new LinearLayout(this);
            info.setOrientation(LinearLayout.VERTICAL);
            TextView name = ShopUi.strong(this, checkoutProductName(item.optString("name", "商品")));
            name.setTextSize(15);
            name.setSingleLine(true);
            name.setMaxLines(1);
            name.setEllipsize(TextUtils.TruncateAt.END);
            TextView quantity = ShopUi.muted(this, "x" + item.optInt("quantity", 1));
            quantity.setSingleLine(true);
            quantity.setMaxLines(1);
            info.addView(name, new LinearLayout.LayoutParams(-1, -2));
            info.addView(quantity, new LinearLayout.LayoutParams(-1, -2));
            LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
            infoParams.leftMargin = ShopUi.dp(this, 10);
            row.addView(info, infoParams);
            TextView price = ShopUi.priceStrong(this, "¥" + item.optString("price", "0"));
            price.setTextSize(15);
            price.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
            row.addView(price, new LinearLayout.LayoutParams(-2, -1));
            itemPanel.addView(row, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 58)));
        }
        if (selectedCount == 0) {
            itemPanel.addView(ShopUi.muted(this, "暂无已选商品"));
        }
        page.addView(itemPanel);
    }

    private String checkoutProductName(String name) {
        String value = name == null || name.trim().isEmpty() ? "商品" : name.trim();
        int maxWidth = 20;
        int width = 0;
        for (int i = 0; i < value.length();) {
            int codePoint = value.codePointAt(i);
            int nextWidth = width + checkoutNameCharWidth(codePoint);
            if (nextWidth > maxWidth) {
                return value.substring(0, i) + "...";
            }
            width = nextWidth;
            i += Character.charCount(codePoint);
        }
        return value;
    }

    private int checkoutNameCharWidth(int codePoint) {
        Character.UnicodeBlock block = Character.UnicodeBlock.of(codePoint);
        if (block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_A
                || block == Character.UnicodeBlock.CJK_UNIFIED_IDEOGRAPHS_EXTENSION_B
                || block == Character.UnicodeBlock.CJK_COMPATIBILITY_IDEOGRAPHS
                || block == Character.UnicodeBlock.CJK_SYMBOLS_AND_PUNCTUATION
                || block == Character.UnicodeBlock.HALFWIDTH_AND_FULLWIDTH_FORMS) {
            return 2;
        }
        return 1;
    }

    private void renderAmountPanel(LinearLayout page) {
        JSONObject summary = cartSnapshot == null ? null : cartSnapshot.optJSONObject("summary");
        LinearLayout amountPanel = ShopUi.panel(this);
        amountPanel.addView(ShopUi.strong(this, "金额明细"));
        amountPanel.addView(ShopUi.muted(this, "商品总额：¥" + (summary == null ? "0" : summary.optString("totalAmount", "0"))));
        if (discountPreview == null) {
            amountPanel.addView(ShopUi.muted(this, "优惠金额：¥0"));
            amountPanel.addView(ShopUi.priceStrong(this, "应付：¥" + cartPayAmount(summary)));
        } else {
            amountPanel.addView(ShopUi.muted(this, "优惠金额：¥" + discountPreview.optString("discount_amount", discountPreview.optString("discountAmount", "0"))));
            amountPanel.addView(ShopUi.priceStrong(this, "应付：¥" + checkoutPayAmount(summary)));
            JSONArray lines = discountPreview.optJSONArray("lines");
            if (lines != null && lines.length() > 0) {
                View divider = new View(this);
                divider.setBackgroundColor(Color.rgb(238, 239, 242));
                LinearLayout.LayoutParams dividerParams = new LinearLayout.LayoutParams(-1, 1);
                dividerParams.setMargins(0, ShopUi.dp(this, 12), 0, ShopUi.dp(this, 8));
                amountPanel.addView(divider, dividerParams);
                amountPanel.addView(ShopUi.strong(this, "优惠明细"));
                for (int i = 0; i < lines.length(); i++) {
                    JSONObject line = lines.optJSONObject(i);
                    if (line != null) {
                        amountPanel.addView(ShopUi.muted(this, line.optString("name", "优惠") + " -¥" + line.optString("amount", line.optString("discount_amount", "0"))));
                    }
                }
            }
        }
        page.addView(amountPanel);
    }

    private void renderBottomBar() {
        JSONObject summary = cartSnapshot == null ? null : cartSnapshot.optJSONObject("summary");
        LinearLayout bottom = new LinearLayout(this);
        bottom.setGravity(Gravity.CENTER_VERTICAL);
        bottom.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 8), ShopUi.dp(this, 16), ShopUi.dp(this, 8));
        bottom.setBackgroundColor(Color.WHITE);
        TextView pay = ShopUi.priceStrong(this, "应付 ¥" + checkoutPayAmount(summary));
        pay.setTextSize(18);
        bottom.addView(pay, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 50), 1));
        Button submit = ShopUi.primaryButton(this, "提交订单");
        submit.setOnClickListener(v -> checkoutCart());
        bottom.addView(submit, new LinearLayout.LayoutParams(ShopUi.dp(this, 142), ShopUi.dp(this, 50)));
        content.addView(bottom, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 66)));
    }

    private void checkoutCart() {
        new Thread(() -> {
            try {
                api().checkout();
                CheckoutDraftStore.remove(draftId);
                runOnUiThread(() -> {
                    showToastLine("订单已创建，请尽快支付");
                    Intent intent = new Intent(this, ChatActivity.class);
                    intent.putExtra(Routes.EXTRA_ROUTE, Routes.ORDERS);
                    intent.addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP);
                    startActivity(intent);
                    finish();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("结算失败：" + error.getMessage()));
            }
        }).start();
    }

    private String checkoutPayAmount(JSONObject summary) {
        if (discountPreview == null) {
            return cartPayAmount(summary);
        }
        return discountPreview.optString("pay_amount", discountPreview.optString("payAmount", cartPayAmount(summary)));
    }

    private String cartPayAmount(JSONObject summary) {
        if (summary == null) {
            return "0";
        }
        String pay = summary.optString("payAmount", "");
        if (pay.isEmpty()) {
            pay = summary.optString("pay_amount", "");
        }
        if (pay.isEmpty()) {
            pay = summary.optString("totalAmount", summary.optString("total_amount", "0"));
        }
        return pay;
    }

    private View productImage(String imageUrl, int sizeDp) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(ShopUi.rounded(Color.rgb(243, 244, 246), ShopUi.dp(this, 12)));
        TextView placeholder = new TextView(this);
        placeholder.setText("图");
        placeholder.setTextSize(14);
        placeholder.setGravity(Gravity.CENTER);
        placeholder.setTextColor(Color.rgb(107, 114, 128));
        frame.addView(placeholder, new FrameLayout.LayoutParams(-1, -1));
        String url = api().absoluteUrl(imageUrl);
        if (!url.isEmpty()) {
            ImageView image = new ImageView(this);
            image.setScaleType(ImageView.ScaleType.CENTER_CROP);
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
            imageLoader().load(image, url);
        }
        return frame;
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }
}
