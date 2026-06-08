package com.xzxg.shop.cart;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.product.ProductListActivity;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

public class CartActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private FrameLayout root;
    private LinearLayout content;
    private LinearLayout cartCheckoutBar;
    private JSONObject activeCartSnapshot;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        renderCart();
    }

    private void renderCart() {
        root = new FrameLayout(this);
        root.setBackgroundColor(BG_COLOR);
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        content.addView(createBackTopBar("购物车"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));
        cartCheckoutBar = null;
        activeCartSnapshot = null;

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 96));
        scroll.addView(page);
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        setContentView(root);
        bindRootSystemBarPadding(content);

        if (sessionStore().token().isEmpty()) {
            page.addView(ShopUi.card(this, "请先登录", "登录后可查看购物车并结算。"));
            return;
        }
        loadCart(page);
    }

    private void loadCart(LinearLayout page) {
        removeCartCheckoutBar();
        page.removeAllViews();
        page.addView(ShopUi.muted(this, "正在加载购物车..."));
        new Thread(() -> {
            try {
                JSONObject cart = api().cart();
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "购物车加载失败", error.getMessage(), () -> loadCart(page)));
            }
        }).start();
    }

    private void renderCartContent(LinearLayout page, JSONObject cart) {
        page.removeAllViews();
        activeCartSnapshot = cart;
        JSONArray items = cart == null ? null : cart.optJSONArray("items");
        if (items == null || items.length() == 0) {
            removeCartCheckoutBar();
            page.addView(ShopUi.card(this, "购物车为空", "可以先去商品页添加商品。"));
            Button goProducts = ShopUi.primaryButton(this, "去逛商品");
            goProducts.setOnClickListener(v -> startActivity(new Intent(this, ProductListActivity.class)));
            page.addView(goProducts, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                page.addView(cartItemView(item, page));
            }
        }
        JSONObject summary = cart.optJSONObject("summary");
        renderCartCheckoutBar(page, items, summary);
    }

    private void removeCartCheckoutBar() {
        if (cartCheckoutBar != null && cartCheckoutBar.getParent() instanceof ViewGroup) {
            ((ViewGroup) cartCheckoutBar.getParent()).removeView(cartCheckoutBar);
        }
        cartCheckoutBar = null;
    }

    private void renderCartCheckoutBar(LinearLayout page, JSONArray items, JSONObject summary) {
        removeCartCheckoutBar();
        cartCheckoutBar = new LinearLayout(this);
        cartCheckoutBar.setGravity(Gravity.CENTER_VERTICAL);
        cartCheckoutBar.setPadding(ShopUi.dp(this, 12), ShopUi.dp(this, 8), ShopUi.dp(this, 12), ShopUi.dp(this, 8));
        cartCheckoutBar.setBackgroundColor(Color.WHITE);

        CheckBox selectAll = new CheckBox(this);
        selectAll.setChecked(cartAllSelected(items));
        selectAll.setOnClickListener(v -> toggleSelectAllCartItems(page, items, ((CheckBox) v).isChecked()));
        cartCheckoutBar.addView(selectAll, new LinearLayout.LayoutParams(ShopUi.dp(this, 42), ShopUi.dp(this, 48)));

        int selectedCount = summary == null ? 0 : summary.optInt("selectedCount", 0);
        TextView selected = ShopUi.muted(this, selectedCount > 0 ? "已选 " + selectedCount + " 件" : "全选");
        selected.setTextSize(15);
        cartCheckoutBar.addView(selected, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1));

        TextView amount = new TextView(this);
        amount.setText("合计：¥" + cartPayAmount(summary));
        amount.setTextSize(15);
        amount.setTextColor(Color.rgb(220, 38, 38));
        amount.setTypeface(Typeface.DEFAULT_BOLD);
        amount.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        LinearLayout.LayoutParams amountParams = new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1);
        amountParams.rightMargin = ShopUi.dp(this, 10);
        cartCheckoutBar.addView(amount, amountParams);

        Button checkout = ShopUi.primaryButton(this, "结算");
        checkout.setTextSize(17);
        checkout.setOnClickListener(v -> openCheckoutPage(summary));
        cartCheckoutBar.addView(checkout, new LinearLayout.LayoutParams(ShopUi.dp(this, 128), ShopUi.dp(this, 50)));
        content.addView(cartCheckoutBar, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 66)));
    }

    private View cartItemView(JSONObject item, LinearLayout page) {
        LinearLayout card = ShopUi.panel(this);
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        CheckBox selected = new CheckBox(this);
        selected.setChecked(item.optBoolean("selected"));
        selected.setOnClickListener(v -> updateCartItem(page, item.optString("cartItemId"), null, ((CheckBox) v).isChecked()));
        row.addView(selected, new LinearLayout.LayoutParams(ShopUi.dp(this, 48), ShopUi.dp(this, 48)));
        row.addView(productImage(item.optString("imageUrl", ""), 64), new LinearLayout.LayoutParams(ShopUi.dp(this, 64), ShopUi.dp(this, 64)));

        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        info.addView(ShopUi.strong(this, item.optString("name", "商品")));
        info.addView(ShopUi.muted(this, item.optString("merchantName", "商家") + " · ¥" + item.optString("price", "0")));
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = ShopUi.dp(this, 10);
        row.addView(info, infoParams);
        card.addView(row);

        LinearLayout controls = new LinearLayout(this);
        controls.setGravity(Gravity.CENTER_VERTICAL | Gravity.RIGHT);
        Button minus = ShopUi.secondaryButton(this, "-");
        minus.setOnClickListener(v -> updateCartItem(page, item.optString("cartItemId"), Math.max(1, item.optInt("quantity", 1) - 1), null));
        controls.addView(minus, new LinearLayout.LayoutParams(ShopUi.dp(this, 46), ShopUi.dp(this, 42)));
        TextView quantity = ShopUi.muted(this, String.valueOf(item.optInt("quantity", 1)));
        quantity.setGravity(Gravity.CENTER);
        controls.addView(quantity, new LinearLayout.LayoutParams(ShopUi.dp(this, 52), ShopUi.dp(this, 42)));
        Button plus = ShopUi.secondaryButton(this, "+");
        plus.setOnClickListener(v -> updateCartItem(page, item.optString("cartItemId"), item.optInt("quantity", 1) + 1, null));
        controls.addView(plus, new LinearLayout.LayoutParams(ShopUi.dp(this, 46), ShopUi.dp(this, 42)));
        Button delete = ShopUi.textOnlyButton(this, "删除");
        delete.setTextColor(Color.rgb(185, 28, 28));
        delete.setOnClickListener(v -> deleteCartItem(page, item.optString("cartItemId")));
        controls.addView(delete, new LinearLayout.LayoutParams(ShopUi.dp(this, 70), ShopUi.dp(this, 42)));
        LinearLayout.LayoutParams controlsParams = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 46));
        controlsParams.topMargin = ShopUi.dp(this, 14);
        card.addView(controls, controlsParams);
        return card;
    }

    private void updateCartItem(LinearLayout page, String cartItemId, Integer quantity, Boolean selected) {
        new Thread(() -> {
            try {
                JSONObject cart = api().updateCartItem(cartItemId, quantity, selected);
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("更新失败：" + error.getMessage()));
            }
        }).start();
    }

    private void deleteCartItem(LinearLayout page, String cartItemId) {
        new Thread(() -> {
            try {
                JSONObject cart = api().deleteCartItem(cartItemId);
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("删除失败：" + error.getMessage()));
            }
        }).start();
    }

    private void toggleSelectAllCartItems(LinearLayout page, JSONArray items, boolean selected) {
        if (items == null) {
            return;
        }
        new Thread(() -> {
            try {
                for (int i = 0; i < items.length(); i++) {
                    JSONObject item = items.optJSONObject(i);
                    if (item != null && item.optBoolean("selected") != selected) {
                        api().updateCartItem(item.optString("cartItemId"), null, selected);
                    }
                }
                JSONObject cart = api().cart();
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("全选失败：" + error.getMessage()));
            }
        }).start();
    }

    private void openCheckoutPage(JSONObject summary) {
        int selectedCount = summary == null ? 0 : summary.optInt("selectedCount");
        if (selectedCount == 0) {
            showToastLine("请先选择要结算的商品");
            return;
        }
        String draftId = CheckoutDraftStore.put(activeCartSnapshot);
        Intent intent = new Intent(this, CheckoutActivity.class);
        intent.putExtra(Routes.EXTRA_CHECKOUT_DRAFT_ID, draftId);
        startActivity(intent);
    }

    private boolean cartAllSelected(JSONArray items) {
        if (items == null || items.length() == 0) {
            return false;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null && !item.optBoolean("selected")) {
                return false;
            }
        }
        return true;
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

    private void renderError(LinearLayout parent, String title, String detail, Runnable retry) {
        parent.removeAllViews();
        parent.addView(ShopUi.card(this, title, detail == null ? "" : detail));
        Button button = ShopUi.secondaryButton(this, "重试");
        button.setOnClickListener(v -> retry.run());
        parent.addView(button, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
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
