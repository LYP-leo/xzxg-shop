package com.xzxg.shop.order;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.BottomSheetHelper;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.app.AlertDialog;
import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

public class OrderListActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private LinearLayout content;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        renderOrders();
    }

    private void renderOrders() {
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("订单"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page);
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        setContentView(content);
        bindRootSystemBarPadding(content);

        if (sessionStore().token().isEmpty()) {
            page.addView(ShopUi.card(this, "请先登录", "登录后可查看订单。"));
            return;
        }
        page.addView(ShopUi.muted(this, "正在加载订单..."));
        loadOrders(page);
    }

    private void loadOrders(LinearLayout page) {
        new Thread(() -> {
            try {
                JSONArray items = api().orders();
                runOnUiThread(() -> renderOrderItems(page, items));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "订单加载失败", error.getMessage(), () -> renderOrders()));
            }
        }).start();
    }

    private void renderOrderItems(LinearLayout page, JSONArray items) {
        page.removeAllViews();
        if (items == null || items.length() == 0) {
            page.addView(ShopUi.card(this, "暂无订单", "购物车结算后会在这里展示订单。"));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                page.addView(orderCard(item));
            }
        }
    }

    private View orderCard(JSONObject item) {
        LinearLayout card = ShopUi.panel(this);
        card.addView(ShopUi.strong(this, "订单 " + item.optString("order_id", "")));
        card.addView(ShopUi.muted(this, statusText(item.optString("status", "")) + " · " + item.optString("merchant_name", "商家")));
        card.addView(ShopUi.muted(this, "金额：¥" + item.optString("total_amount", "0")));
        JSONArray items = item.optJSONArray("items");
        if (items != null && items.length() > 0) {
            JSONObject first = items.optJSONObject(0);
            String more = items.length() > 1 ? " 等 " + items.length() + " 件商品" : "";
            card.addView(ShopUi.muted(this, "商品：" + (first == null ? "" : first.optString("name", "")) + more));
        }
        card.addView(ShopUi.muted(this, "创建时间：" + item.optString("created_at", "")));
        addOrderActions(card, item);
        return card;
    }

    private void addOrderActions(LinearLayout card, JSONObject order) {
        String status = order.optString("status", "");
        String orderId = order.optString("order_id", "");
        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        if ("pending_payment".equals(status)) {
            Button pay = ShopUi.primaryButton(this, "去支付");
            pay.setOnClickListener(v -> payOrder(orderId));
            actions.addView(pay, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 44), 1));
            Button cancel = ShopUi.textOnlyButton(this, "取消订单");
            cancel.setTextColor(Color.rgb(185, 28, 28));
            cancel.setOnClickListener(v -> cancelOrder(orderId));
            actions.addView(cancel, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 44), 1));
        } else if ("shipped".equals(status)) {
            Button confirm = ShopUi.primaryButton(this, "确认收货");
            confirm.setOnClickListener(v -> confirmOrder(orderId));
            actions.addView(confirm, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 44)));
        } else if ("completed".equals(status)) {
            Button review = ShopUi.secondaryButton(this, "评价商品");
            review.setOnClickListener(v -> openReviewDialog(order));
            actions.addView(review, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 44)));
        }
        if (actions.getChildCount() > 0) {
            card.addView(actions);
        }
    }

    private void payOrder(String orderId) {
        new Thread(() -> {
            try {
                api().payOrder(orderId);
                runOnUiThread(() -> {
                    showToastLine("支付成功");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("支付失败：" + error.getMessage()));
            }
        }).start();
    }

    private void cancelOrder(String orderId) {
        new Thread(() -> {
            try {
                api().cancelOrder(orderId, "暂时不买了");
                runOnUiThread(() -> {
                    showToastLine("订单已取消");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("取消失败：" + error.getMessage()));
            }
        }).start();
    }

    private void confirmOrder(String orderId) {
        new Thread(() -> {
            try {
                api().confirmReceipt(orderId);
                runOnUiThread(() -> {
                    showToastLine("已确认收货");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("确认失败：" + error.getMessage()));
            }
        }).start();
    }

    private void openReviewDialog(JSONObject order) {
        JSONArray items = order.optJSONArray("items");
        if (items == null || items.length() == 0) {
            showToastLine("暂无可评价商品");
            return;
        }
        if (items.length() == 1) {
            showReviewForm(order, items.optJSONObject(0));
            return;
        }
        LinearLayout chooser = new LinearLayout(this);
        chooser.setOrientation(LinearLayout.VERTICAL);
        chooser.setPadding(ShopUi.dp(this, 20), ShopUi.dp(this, 18), ShopUi.dp(this, 20), ShopUi.dp(this, 16));
        chooser.setBackground(ShopUi.rounded(Color.WHITE, ShopUi.dp(this, 22)));
        chooser.addView(ShopUi.title(this, "选择评价商品"), new LinearLayout.LayoutParams(-1, -2));
        final AlertDialog[] dialogRef = new AlertDialog[1];
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item == null) {
                continue;
            }
            Button option = ShopUi.secondaryButton(this, item.optString("name", "商品") + " x" + item.optInt("quantity", 1));
            option.setOnClickListener(v -> {
                if (dialogRef[0] != null) {
                    dialogRef[0].dismiss();
                }
                showReviewForm(order, item);
            });
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 46));
            params.bottomMargin = ShopUi.dp(this, 8);
            chooser.addView(option, params);
        }
        dialogRef[0] = BottomSheetHelper.show(this, chooser, true);
    }

    private void showReviewForm(JSONObject order, JSONObject item) {
        if (item == null) {
            showToastLine("订单项信息缺失");
            return;
        }
        LinearLayout form = BottomSheetHelper.box(this);
        form.addView(ShopUi.title(this, "评价商品"), new LinearLayout.LayoutParams(-1, -2));
        TextView product = ShopUi.muted(this, item.optString("name", "商品") + " x" + item.optInt("quantity", 1));
        product.setPadding(0, ShopUi.dp(this, 6), 0, ShopUi.dp(this, 10));
        form.addView(product);
        final int[] rating = {5};
        LinearLayout stars = new LinearLayout(this);
        stars.setGravity(Gravity.CENTER_VERTICAL);
        TextView ratingLabel = ShopUi.muted(this, "评分");
        stars.addView(ratingLabel, new LinearLayout.LayoutParams(ShopUi.dp(this, 52), ShopUi.dp(this, 44)));
        List<TextView> starViews = new ArrayList<>();
        for (int i = 1; i <= 5; i++) {
            final int value = i;
            TextView star = new TextView(this);
            star.setText("★");
            star.setTextSize(28);
            star.setTextColor(Color.rgb(245, 158, 11));
            star.setGravity(Gravity.CENTER);
            star.setOnClickListener(v -> {
                rating[0] = value;
                for (int j = 0; j < starViews.size(); j++) {
                    starViews.get(j).setTextColor(j < rating[0] ? Color.rgb(245, 158, 11) : Color.rgb(209, 213, 219));
                }
            });
            starViews.add(star);
            stars.addView(star, new LinearLayout.LayoutParams(ShopUi.dp(this, 40), ShopUi.dp(this, 44)));
        }
        form.addView(stars);

        String[] tagOptions = new String[]{"质量不错", "物流快", "包装好", "性价比高", "和描述一致", "会回购"};
        Set<String> selectedTags = new HashSet<>();
        LinearLayout tags = new LinearLayout(this);
        tags.setOrientation(LinearLayout.VERTICAL);
        for (String option : tagOptions) {
            CheckBox tag = new CheckBox(this);
            tag.setText(option);
            tag.setTextSize(14);
            tag.setTextColor(Color.rgb(55, 65, 81));
            tag.setOnCheckedChangeListener((buttonView, checked) -> {
                if (checked) {
                    selectedTags.add(option);
                } else {
                    selectedTags.remove(option);
                }
            });
            tags.addView(tag, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 38)));
        }
        form.addView(tags);

        EditText contentInput = ShopUi.inputField(this, "评价内容", "");
        form.addView(contentInput);
        final AlertDialog[] dialogRef = new AlertDialog[1];
        LinearLayout actions = new LinearLayout(this);
        Button cancel = ShopUi.secondaryButton(this, "取消");
        Button submit = ShopUi.primaryButton(this, "提交");
        cancel.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
        });
        submit.setOnClickListener(v -> submitReview(order.optString("order_id"), item.optString("order_item_id"), rating[0], contentInput.getText().toString(), selectedTags, dialogRef[0], submit));
        actions.addView(cancel, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1));
        LinearLayout.LayoutParams submitParams = new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1);
        submitParams.leftMargin = ShopUi.dp(this, 10);
        actions.addView(submit, submitParams);
        form.addView(actions, new LinearLayout.LayoutParams(-1, -2));
        dialogRef[0] = BottomSheetHelper.show(this, form, true);
    }

    private void submitReview(String orderId, String orderItemId, int rating, String content, Set<String> selectedTags, AlertDialog dialog, Button submitButton) {
        if (content == null || content.trim().isEmpty()) {
            showToastLine("请填写评价内容");
            return;
        }
        if (submitButton != null) {
            submitButton.setEnabled(false);
            submitButton.setAlpha(0.5f);
        }
        int finalRating = rating;
        new Thread(() -> {
            try {
                JSONArray tags = new JSONArray();
                if (selectedTags != null) {
                    for (String tag : selectedTags) {
                        tags.put(tag);
                    }
                }
                api().reviewOrderItem(orderId, orderItemId, finalRating, content, tags);
                runOnUiThread(() -> {
                    if (dialog != null) {
                        dialog.dismiss();
                    }
                    showToastLine("评价已提交");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (submitButton != null) {
                        submitButton.setEnabled(true);
                        submitButton.setAlpha(1f);
                    }
                    showToastLine("评价失败：该商品暂不可评价，可能已评价或订单未完成");
                });
            }
        }).start();
    }

    private void renderError(LinearLayout parent, String title, String detail, Runnable retry) {
        parent.removeAllViews();
        parent.addView(ShopUi.card(this, title, detail == null ? "" : detail));
        Button button = ShopUi.secondaryButton(this, "重试");
        button.setOnClickListener(v -> retry.run());
        parent.addView(button, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
    }

    private String statusText(String status) {
        if ("in_stock".equals(status)) return "有货";
        if ("out_of_stock".equals(status)) return "缺货";
        if ("low_stock".equals(status)) return "库存紧张";
        if ("pending_pay".equals(status) || "pending_payment".equals(status)) return "待支付";
        if ("paid".equals(status)) return "已支付";
        if ("pending_ship".equals(status)) return "待发货";
        if ("shipped".equals(status)) return "已发货";
        if ("completed".equals(status)) return "已完成";
        if ("cancelled".equals(status) || "canceled".equals(status)) return "已取消";
        if ("closed_timeout".equals(status)) return "支付超时关闭";
        return status == null || status.isEmpty() ? "未知状态" : status;
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }
}
