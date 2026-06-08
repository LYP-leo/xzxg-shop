package com.xzxg.shop.ui;

import com.xzxg.shop.R;
import com.xzxg.shop.chat.ChatActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.network.ApiClient;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

public class PigGuideController {
    private final Activity activity;
    private final ApiClient api;
    private final FrameLayout root;
    private LinearLayout layer;
    private LinearLayout bubbleList;
    private float downRawX;
    private float downRawY;
    private float startX;
    private float startY;
    private boolean dragging;
    private int requestSeq;

    public PigGuideController(Activity activity, ApiClient api, FrameLayout root) {
        this.activity = activity;
        this.api = api;
        this.root = root;
    }

    public void show(String page, JSONObject context, int bottomMarginDp) {
        if (root == null || page == null || page.trim().isEmpty()) {
            return;
        }
        JSONArray fallback = fallbackSuggestions(page, context);
        ensureLayer(bottomMarginDp);
        renderBubbles(fallback);
        int requestId = ++requestSeq;
        new Thread(() -> {
            try {
                JSONArray remote = api.guideSuggestions(page, context, 3);
                activity.runOnUiThread(() -> {
                    if (requestId == requestSeq) {
                        renderBubbles(remote.length() == 0 ? fallback : remote);
                    }
                });
            } catch (Exception ignored) {
            }
        }).start();
    }

    private void ensureLayer(int bottomMarginDp) {
        if (layer != null && layer.getParent() == root) {
            layer.bringToFront();
            return;
        }
        layer = new LinearLayout(activity);
        layer.setOrientation(LinearLayout.HORIZONTAL);
        layer.setGravity(Gravity.BOTTOM | Gravity.RIGHT);
        layer.setClipChildren(false);
        layer.setClipToPadding(false);

        bubbleList = new LinearLayout(activity);
        bubbleList.setOrientation(LinearLayout.VERTICAL);
        bubbleList.setGravity(Gravity.RIGHT);
        LinearLayout.LayoutParams bubbleParams = new LinearLayout.LayoutParams(ShopUi.dp(activity, 214), -2);
        bubbleParams.rightMargin = ShopUi.dp(activity, 8);
        layer.addView(bubbleList, bubbleParams);

        ImageView pig = new ImageView(activity);
        pig.setImageResource(R.drawable.ic_pig_guide);
        pig.setScaleType(ImageView.ScaleType.FIT_CENTER);
        pig.setPadding(ShopUi.dp(activity, 5), ShopUi.dp(activity, 5), ShopUi.dp(activity, 5), ShopUi.dp(activity, 5));
        pig.setBackground(ShopUi.rounded(Color.rgb(255, 214, 224), ShopUi.dp(activity, 29)));
        pig.setElevation(ShopUi.dp(activity, 8));
        pig.setOnTouchListener((view, event) -> handleDrag(event));
        layer.addView(pig, new LinearLayout.LayoutParams(ShopUi.dp(activity, 58), ShopUi.dp(activity, 58)));

        FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(ShopUi.dp(activity, 280), -2, Gravity.RIGHT | Gravity.BOTTOM);
        params.rightMargin = ShopUi.dp(activity, 16);
        params.bottomMargin = ShopUi.dp(activity, bottomMarginDp);
        root.addView(layer, params);
        layer.post(() -> clamp(layer.getX(), layer.getY()));
    }

    private boolean handleDrag(MotionEvent event) {
        if (layer == null || root == null) {
            return false;
        }
        switch (event.getActionMasked()) {
            case MotionEvent.ACTION_DOWN:
                downRawX = event.getRawX();
                downRawY = event.getRawY();
                startX = layer.getX();
                startY = layer.getY();
                dragging = false;
                return true;
            case MotionEvent.ACTION_MOVE:
                float dx = event.getRawX() - downRawX;
                float dy = event.getRawY() - downRawY;
                if (!dragging && Math.hypot(dx, dy) > ShopUi.dp(activity, 6)) {
                    dragging = true;
                }
                clamp(startX + dx, startY + dy);
                return true;
            case MotionEvent.ACTION_UP:
            case MotionEvent.ACTION_CANCEL:
                if (!dragging && bubbleList != null) {
                    bubbleList.setVisibility(bubbleList.getVisibility() == View.VISIBLE ? View.GONE : View.VISIBLE);
                }
                dragging = false;
                return true;
            default:
                return true;
        }
    }

    private void clamp(float x, float y) {
        if (layer == null || root == null || root.getWidth() == 0 || root.getHeight() == 0) {
            return;
        }
        int maxX = Math.max(0, root.getWidth() - layer.getWidth() - ShopUi.dp(activity, 8));
        int maxY = Math.max(0, root.getHeight() - layer.getHeight() - ShopUi.dp(activity, 8));
        layer.setX(Math.max(ShopUi.dp(activity, 8), Math.min(x, maxX)));
        layer.setY(Math.max(ShopUi.dp(activity, 8), Math.min(y, maxY)));
    }

    private void renderBubbles(JSONArray suggestions) {
        if (bubbleList == null) {
            return;
        }
        bubbleList.removeAllViews();
        for (String question : questions(suggestions)) {
            TextView bubble = new TextView(activity);
            bubble.setText(question);
            bubble.setTextSize(13);
            bubble.setTextColor(Color.rgb(31, 41, 55));
            bubble.setGravity(Gravity.CENTER_VERTICAL);
            bubble.setMaxLines(2);
            bubble.setPadding(ShopUi.dp(activity, 12), ShopUi.dp(activity, 8), ShopUi.dp(activity, 12), ShopUi.dp(activity, 8));
            bubble.setBackground(ShopUi.rounded(Color.WHITE, ShopUi.dp(activity, 16)));
            bubble.setElevation(ShopUi.dp(activity, 4));
            bubble.setOnClickListener(v -> openChatWithQuestion(((TextView) v).getText().toString()));
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
            params.setMargins(0, 0, 0, ShopUi.dp(activity, 8));
            bubbleList.addView(bubble, params);
        }
        if (layer != null) {
            layer.bringToFront();
        }
    }

    private List<String> questions(JSONArray suggestions) {
        List<String> questions = new ArrayList<>();
        Set<String> seen = new LinkedHashSet<>();
        if (suggestions != null) {
            for (int i = 0; i < suggestions.length(); i++) {
                Object raw = suggestions.opt(i);
                String question = "";
                if (raw instanceof JSONObject) {
                    question = ((JSONObject) raw).optString("question", "");
                } else if (raw != null) {
                    question = String.valueOf(raw);
                }
                question = question == null ? "" : question.trim();
                if (!question.isEmpty() && seen.add(question)) {
                    questions.add(question);
                }
                if (questions.size() >= 3) {
                    break;
                }
            }
        }
        return questions;
    }

    private JSONArray fallbackSuggestions(String page, JSONObject context) {
        JSONArray items = new JSONArray();
        if (Routes.PRODUCTS.equals(page)) {
            String keyword = context == null ? "" : context.optString("keyword", "");
            String category = context == null ? "" : context.optString("category_name", "");
            putQuestion(items, keyword.isEmpty() ? "帮我推荐几款高性价比商品" : "帮我找和“" + keyword + "”相关的好物");
            putQuestion(items, category.isEmpty() ? "帮我比较当前这些商品" : "帮我按预算推荐几款" + category);
            putQuestion(items, "当前这些商品怎么选？");
        } else if (Routes.CART.equals(page)) {
            int count = context == null ? 0 : context.optInt("cart_item_count", 0);
            putQuestion(items, count > 0 ? "帮我分析这 " + count + " 件商品" : "帮我看看购物车怎么搭配");
            putQuestion(items, "购物车里哪些值得买？");
            putQuestion(items, "现在适合直接下单吗？");
        } else if (Routes.ORDERS.equals(page)) {
            putQuestion(items, "帮我总结订单状态");
            putQuestion(items, "哪些订单需要尽快处理？");
            putQuestion(items, "待评价订单怎么写评价？");
        }
        return items;
    }

    private void putQuestion(JSONArray items, String question) {
        if (question == null || question.trim().isEmpty()) {
            return;
        }
        JSONObject item = new JSONObject();
        try {
            item.put("id", "local_" + items.length());
            item.put("question", question.trim());
            item.put("reason", "local_fallback");
            items.put(item);
        } catch (Exception ignored) {
        }
    }

    private void openChatWithQuestion(String question) {
        String value = question == null ? "" : question.trim();
        if (value.isEmpty()) {
            return;
        }
        Intent intent = new Intent(activity, ChatActivity.class);
        intent.putExtra(Routes.EXTRA_INITIAL_QUESTION, value);
        activity.startActivity(intent);
    }
}
