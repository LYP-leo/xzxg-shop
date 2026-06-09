package com.xzxg.shop.product;

import com.xzxg.shop.account.LoginActivity;
import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.ui.CartFabHelper;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
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

import java.util.HashMap;
import java.util.Locale;
import java.util.Map;

public class ProductDetailActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private FrameLayout root;
    private LinearLayout page;
    private CartFabHelper cartFabHelper;
    private String productId;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        productId = getIntent().getStringExtra(Routes.EXTRA_PRODUCT_ID);
        if (productId == null) {
            productId = "";
        }
        configureShopSystemBars(BG_COLOR);
        render();
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (cartFabHelper != null) {
            cartFabHelper.refresh();
        }
    }

    private void render() {
        cartFabHelper = null;
        root = new FrameLayout(this);
        root.setBackgroundColor(BG_COLOR);

        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("商品详情"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        setContentView(root);
        bindRootSystemBarPadding(content);
        cartFabHelper = attachCartFab(root, 24);
        loadProductDetail();
    }

    private void loadProductDetail() {
        page.removeAllViews();
        page.addView(ShopUi.muted(this, "正在加载商品详情..."));
        new Thread(() -> {
            try {
                JSONObject detail = api().productDetail(productId);
                JSONArray skus = api().productSkus(productId);
                runOnUiThread(() -> renderProductDetailContent(detail, skus));
            } catch (Exception error) {
                runOnUiThread(() -> renderError("商品详情加载失败", error.getMessage(), this::loadProductDetail));
            }
        }).start();
    }

    private void renderProductDetailContent(JSONObject item, JSONArray skus) {
        page.removeAllViews();
        JSONObject safeItem = item == null ? new JSONObject() : item;
        JSONArray images = safeItem.optJSONArray("imageUrls");
        String imageUrl = images != null && images.length() > 0 ? images.optString(0) : safeItem.optString("imageUrl", "");
        page.addView(productDetailImage(imageUrl), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 240)));

        LinearLayout main = ShopUi.panel(this);
        main.addView(ShopUi.strong(this, safeItem.optString("name", "未命名商品")));
        main.addView(ShopUi.muted(this, safeItem.optString("brand", "") + " · " + safeItem.optString("merchantName", "商家")));
        TextView price = ShopUi.strong(this, "¥" + safeItem.optString("price", "0"));
        price.setTextSize(22);
        main.addView(price);
        String market = safeItem.optString("marketPrice", "");
        if (!market.isEmpty()) {
            main.addView(ShopUi.muted(this, "市场价：¥" + market));
        }
        main.addView(ShopUi.muted(this, "库存：" + statusText(safeItem.optString("stockStatus", ""))));
        page.addView(main);

        addExpandableArrayPanel("商品卖点", safeItem.optJSONArray("sellingPoints"), 3);
        addExpandableTextPanel("推荐理由", safeItem.optString("recommendReason", ""), 3);
        addExpandableArrayPanel("适合人群", safeItem.optJSONArray("suitableFor"), 3);
        addExpandableArrayPanel("不适合人群", safeItem.optJSONArray("notSuitableFor"), 3);
        addExpandableArrayPanel("风险提示", safeItem.optJSONArray("riskNotes"), 3);
        addExpandableAttributesPanel(safeItem.optJSONArray("attributes"), 6);
        addSkusPanel(skus);
        loadProductReviews(safeItem.optString("productId", productId));

        Button add = ShopUi.primaryButton(this, "加入购物车");
        add.setOnClickListener(v -> addProductToCart(safeItem, add));
        page.addView(add, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
    }

    private View productDetailImage(String imageUrl) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(ShopUi.rounded(Color.rgb(243, 244, 246), ShopUi.dp(this, 12)));

        String url = api().absoluteUrl(imageUrl);
        if (!url.isEmpty()) {
            ImageView image = new ImageView(this);
            image.setScaleType(ImageView.ScaleType.FIT_CENTER);
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
            imageLoader().load(image, url);
            frame.setOnClickListener(v -> showImagePreview(url));
        }
        return frame;
    }

    private void showImagePreview(String url) {
        FrameLayout preview = new FrameLayout(this);
        preview.setBackgroundColor(Color.BLACK);
        ImageView image = new ImageView(this);
        image.setScaleType(ImageView.ScaleType.FIT_CENTER);
        imageLoader().load(image, url);
        preview.addView(image, new FrameLayout.LayoutParams(-1, -1));

        TextView close = new TextView(this);
        close.setText("×");
        close.setTextSize(28);
        close.setGravity(Gravity.CENTER);
        close.setTextColor(Color.WHITE);
        close.setOnClickListener(v -> root.removeView(preview));
        FrameLayout.LayoutParams closeParams = new FrameLayout.LayoutParams(ShopUi.dp(this, 56), ShopUi.dp(this, 56), Gravity.RIGHT | Gravity.TOP);
        closeParams.topMargin = ShopUi.dp(this, 24);
        closeParams.rightMargin = ShopUi.dp(this, 12);
        preview.addView(close, closeParams);

        preview.setOnClickListener(v -> root.removeView(preview));
        root.addView(preview, new FrameLayout.LayoutParams(-1, -1));
    }

    private void addExpandableTextPanel(String title, String text, int collapsedLines) {
        if (text == null || text.trim().isEmpty()) {
            return;
        }
        LinearLayout panel = ShopUi.panel(this);
        final boolean[] expanded = {false};
        Runnable render = new Runnable() {
            @Override
            public void run() {
                panel.removeAllViews();
                panel.addView(ShopUi.strong(ProductDetailActivity.this, title));
                TextView body = ShopUi.muted(ProductDetailActivity.this, text);
                body.setSingleLine(false);
                body.setMaxLines(expanded[0] ? Integer.MAX_VALUE : collapsedLines);
                panel.addView(body);
                if (text.length() > 80) {
                    Button more = ShopUi.textOnlyButton(ProductDetailActivity.this, expanded[0] ? "收起" : "显示更多");
                    more.setTextColor(Color.rgb(37, 99, 235));
                    more.setTextSize(14);
                    more.setOnClickListener(v -> {
                        expanded[0] = !expanded[0];
                        run();
                    });
                    panel.addView(more, new LinearLayout.LayoutParams(-2, ShopUi.dp(ProductDetailActivity.this, 36)));
                }
            }
        };
        render.run();
        page.addView(panel);
    }

    private void addExpandableArrayPanel(String title, JSONArray items, int collapsedCount) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = ShopUi.panel(this);
        final boolean[] expanded = {false};
        Runnable render = new Runnable() {
            @Override
            public void run() {
                panel.removeAllViews();
                panel.addView(ShopUi.strong(ProductDetailActivity.this, title));
                int count = expanded[0] ? items.length() : Math.min(items.length(), collapsedCount);
                for (int i = 0; i < count; i++) {
                    panel.addView(ShopUi.muted(ProductDetailActivity.this, "• " + items.optString(i)));
                }
                if (items.length() > collapsedCount) {
                    Button more = ShopUi.textOnlyButton(ProductDetailActivity.this, expanded[0] ? "收起" : "显示更多");
                    more.setTextColor(Color.rgb(37, 99, 235));
                    more.setTextSize(14);
                    more.setOnClickListener(v -> {
                        expanded[0] = !expanded[0];
                        run();
                    });
                    panel.addView(more, new LinearLayout.LayoutParams(-2, ShopUi.dp(ProductDetailActivity.this, 36)));
                }
            }
        };
        render.run();
        page.addView(panel);
    }

    private void addExpandableAttributesPanel(JSONArray items, int collapsedCount) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = ShopUi.panel(this);
        final boolean[] expanded = {false};
        Runnable render = new Runnable() {
            @Override
            public void run() {
                panel.removeAllViews();
                panel.addView(ShopUi.strong(ProductDetailActivity.this, "商品参数"));
                int count = expanded[0] ? items.length() : Math.min(items.length(), collapsedCount);
                for (int i = 0; i < count; i++) {
                    JSONObject item = items.optJSONObject(i);
                    if (item != null) {
                        panel.addView(ShopUi.muted(ProductDetailActivity.this, item.optString("key", "") + "：" + item.optString("value", "") + item.optString("unit", "")));
                    }
                }
                if (items.length() > collapsedCount) {
                    Button more = ShopUi.textOnlyButton(ProductDetailActivity.this, expanded[0] ? "收起" : "显示更多");
                    more.setTextColor(Color.rgb(37, 99, 235));
                    more.setTextSize(14);
                    more.setOnClickListener(v -> {
                        expanded[0] = !expanded[0];
                        run();
                    });
                    panel.addView(more, new LinearLayout.LayoutParams(-2, ShopUi.dp(ProductDetailActivity.this, 36)));
                }
            }
        };
        render.run();
        page.addView(panel);
    }

    private void addSkusPanel(JSONArray items) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = ShopUi.panel(this);
        panel.addView(ShopUi.strong(this, "规格"));
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                panel.addView(ShopUi.muted(this, item.optString("skuName", "默认款") + " · ¥" + item.optString("price", "0") + " · 库存 " + item.optInt("stockQuantity", 0)));
            }
        }
        page.addView(panel);
    }

    private void loadProductReviews(String targetProductId) {
        if (targetProductId == null || targetProductId.isEmpty()) {
            return;
        }
        LinearLayout panel = ShopUi.panel(this);
        panel.addView(ShopUi.strong(this, "用户评价"));
        panel.addView(ShopUi.muted(this, "正在加载评价..."));
        page.addView(panel);
        new Thread(() -> {
            try {
                JSONArray reviews = api().productReviews(targetProductId);
                runOnUiThread(() -> renderProductReviews(panel, reviews));
            } catch (Exception error) {
                runOnUiThread(() -> renderProductReviews(panel, new JSONArray()));
            }
        }).start();
    }

    private void renderProductReviews(LinearLayout panel, JSONArray reviews) {
        panel.removeAllViews();
        panel.addView(ShopUi.strong(this, "用户评价"));
        if (reviews == null || reviews.length() == 0) {
            panel.addView(ShopUi.muted(this, "暂无评价"));
            panel.addView(ShopUi.muted(this, "购买并完成订单后可以发表第一条评价。"));
            return;
        }
        int ratingSum = 0;
        Map<String, Integer> tagCounts = new HashMap<>();
        for (int i = 0; i < reviews.length(); i++) {
            JSONObject review = reviews.optJSONObject(i);
            if (review == null) {
                continue;
            }
            ratingSum += Math.max(1, Math.min(5, review.optInt("rating", 5)));
            JSONArray tags = review.optJSONArray("tags");
            if (tags != null) {
                for (int j = 0; j < tags.length(); j++) {
                    String tag = tags.optString(j, "").trim();
                    if (!tag.isEmpty()) {
                        tagCounts.put(tag, tagCounts.containsKey(tag) ? tagCounts.get(tag) + 1 : 1);
                    }
                }
            }
        }
        double average = reviews.length() == 0 ? 0 : (double) ratingSum / reviews.length();
        TextView summary = ShopUi.title(this, String.format(Locale.CHINA, "%.1f  %s  %d 条评价", average, ratingStars((int) Math.round(average)), reviews.length()));
        summary.setTextSize(18);
        panel.addView(summary);
        if (!tagCounts.isEmpty()) {
            LinearLayout tagsRow = new LinearLayout(this);
            tagsRow.setOrientation(LinearLayout.HORIZONTAL);
            int shown = 0;
            for (String tag : tagCounts.keySet()) {
                TextView chip = chipText(tag + " " + tagCounts.get(tag));
                LinearLayout.LayoutParams chipParams = new LinearLayout.LayoutParams(-2, ShopUi.dp(this, 32));
                chipParams.setMargins(0, ShopUi.dp(this, 8), ShopUi.dp(this, 8), ShopUi.dp(this, 4));
                tagsRow.addView(chip, chipParams);
                shown++;
                if (shown >= 5) {
                    break;
                }
            }
            panel.addView(tagsRow);
        }
        int total = Math.min(reviews.length(), 5);
        for (int i = 0; i < total; i++) {
            JSONObject review = reviews.optJSONObject(i);
            if (review != null) {
                panel.addView(reviewCard(review));
            }
        }
    }

    private View reviewCard(JSONObject review) {
        LinearLayout card = new LinearLayout(this);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setPadding(0, ShopUi.dp(this, 10), 0, ShopUi.dp(this, 10));
        TextView head = ShopUi.muted(this, emptyFallback(review.optString("username", ""), "匿名用户") + "  " + ratingStars(review.optInt("rating", 5)));
        card.addView(head);
        TextView content = new TextView(this);
        content.setText(review.optString("content", ""));
        content.setTextSize(15);
        content.setTextColor(Color.rgb(31, 41, 55));
        content.setLineSpacing(4, 1);
        LinearLayout.LayoutParams contentParams = new LinearLayout.LayoutParams(-1, -2);
        contentParams.topMargin = ShopUi.dp(this, 6);
        card.addView(content, contentParams);
        JSONArray tags = review.optJSONArray("tags");
        if (tags != null && tags.length() > 0) {
            LinearLayout tagRow = new LinearLayout(this);
            tagRow.setOrientation(LinearLayout.HORIZONTAL);
            for (int i = 0; i < tags.length(); i++) {
                String tag = tags.optString(i, "").trim();
                if (tag.isEmpty()) {
                    continue;
                }
                TextView chip = chipText(tag);
                LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-2, ShopUi.dp(this, 30));
                params.setMargins(0, ShopUi.dp(this, 8), ShopUi.dp(this, 8), 0);
                tagRow.addView(chip, params);
            }
            card.addView(tagRow);
        }
        String reply = review.optString("merchant_reply", "");
        if (!reply.isEmpty()) {
            TextView replyView = ShopUi.muted(this, "商家回复：" + reply);
            replyView.setPadding(ShopUi.dp(this, 10), ShopUi.dp(this, 8), ShopUi.dp(this, 10), ShopUi.dp(this, 8));
            replyView.setBackground(ShopUi.rounded(Color.rgb(246, 247, 249), ShopUi.dp(this, 10)));
            LinearLayout.LayoutParams replyParams = new LinearLayout.LayoutParams(-1, -2);
            replyParams.topMargin = ShopUi.dp(this, 8);
            card.addView(replyView, replyParams);
        }
        return card;
    }

    private void addProductToCart(JSONObject item, Button sourceButton) {
        if (sessionStore().token().isEmpty()) {
            showToastLine("请先登录后再加入购物车");
            startActivity(new Intent(this, LoginActivity.class));
            return;
        }
        sourceButton.setEnabled(false);
        sourceButton.setAlpha(0.72f);
        sourceButton.setText("加入中...");
        new Thread(() -> {
            try {
                String targetProductId = item.optString("productId", productId);
                String skuId = item.optString("skuId", "");
                if (skuId.isEmpty()) {
                    JSONArray skus = api().productSkus(targetProductId);
                    for (int i = 0; i < skus.length(); i++) {
                        JSONObject sku = skus.optJSONObject(i);
                        if (sku != null && sku.optInt("stockQuantity", 0) > 0) {
                            skuId = sku.optString("skuId", "");
                            break;
                        }
                    }
                }
                api().addCartItem(targetProductId, skuId, 1);
                runOnUiThread(() -> {
                    showToastLine("已加入购物车");
                    if (cartFabHelper != null) {
                        cartFabHelper.refresh();
                    }
                    sourceButton.setEnabled(true);
                    sourceButton.setAlpha(1f);
                    sourceButton.setText("已加入");
                    sourceButton.postDelayed(() -> sourceButton.setText("加入购物车"), 600);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    sourceButton.setEnabled(true);
                    sourceButton.setAlpha(1f);
                    sourceButton.setText("加入购物车");
                    showToastLine("加入失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private void renderError(String title, String detail, Runnable retry) {
        page.removeAllViews();
        page.addView(ShopUi.card(this, title, detail == null ? "" : detail));
        Button button = ShopUi.secondaryButton(this, "重试");
        button.setOnClickListener(v -> retry.run());
        page.addView(button, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }

    private TextView chipText(String text) {
        TextView chip = new TextView(this);
        chip.setText(text);
        chip.setTextSize(13);
        chip.setTextColor(Color.rgb(75, 85, 99));
        chip.setGravity(Gravity.CENTER);
        chip.setPadding(ShopUi.dp(this, 10), 0, ShopUi.dp(this, 10), 0);
        chip.setBackground(ShopUi.rounded(Color.rgb(243, 244, 246), ShopUi.dp(this, 15)));
        return chip;
    }

    private String ratingStars(int rating) {
        int safe = Math.max(1, Math.min(5, rating));
        StringBuilder stars = new StringBuilder();
        for (int i = 1; i <= 5; i++) {
            stars.append(i <= safe ? "★" : "☆");
        }
        return stars.toString();
    }

    private String statusText(String status) {
        if ("in_stock".equals(status)) return "有货";
        if ("out_of_stock".equals(status)) return "缺货";
        if ("low_stock".equals(status)) return "库存紧张";
        return status == null || status.isEmpty() ? "未知状态" : status;
    }

    private String emptyFallback(String value, String fallback) {
        return value == null || value.trim().isEmpty() ? fallback : value.trim();
    }
}
