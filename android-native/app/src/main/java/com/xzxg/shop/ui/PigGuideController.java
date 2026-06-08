package com.xzxg.shop.ui;

import com.xzxg.shop.R;
import com.xzxg.shop.chat.ChatActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.network.ApiClient;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Path;
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
    private static final int BUBBLE_COLOR = Color.WHITE;

    private final Activity activity;
    private final ApiClient api;
    private final FrameLayout root;
    private LinearLayout layer;
    private LinearLayout optionList;
    private float downRawX;
    private float downRawY;
    private float startX;
    private float startY;
    private boolean dragging;
    private boolean expanded;
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
        expanded = false;
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

        FrameLayout bubbleWrap = new FrameLayout(activity);
        bubbleWrap.setClipChildren(false);
        bubbleWrap.setClipToPadding(false);

        optionList = new LinearLayout(activity);
        optionList.setOrientation(LinearLayout.VERTICAL);
        optionList.setPadding(ShopUi.dp(activity, 10), ShopUi.dp(activity, 10), ShopUi.dp(activity, 10), ShopUi.dp(activity, 10));
        optionList.setBackground(ShopUi.rounded(BUBBLE_COLOR, ShopUi.dp(activity, 18)));
        optionList.setElevation(ShopUi.dp(activity, 8));
        FrameLayout.LayoutParams optionParams = new FrameLayout.LayoutParams(ShopUi.dp(activity, 214), -2, Gravity.BOTTOM | Gravity.RIGHT);
        optionParams.rightMargin = ShopUi.dp(activity, 10);
        bubbleWrap.addView(optionList, optionParams);

        BubbleTailView tail = new BubbleTailView(activity);
        FrameLayout.LayoutParams tailParams = new FrameLayout.LayoutParams(ShopUi.dp(activity, 34), ShopUi.dp(activity, 30), Gravity.RIGHT | Gravity.BOTTOM);
        tailParams.rightMargin = -ShopUi.dp(activity, 4);
        tailParams.bottomMargin = ShopUi.dp(activity, 22);
        bubbleWrap.addView(tail, tailParams);

        LinearLayout.LayoutParams bubbleParams = new LinearLayout.LayoutParams(ShopUi.dp(activity, 230), -2);
        bubbleParams.rightMargin = ShopUi.dp(activity, 2);
        layer.addView(bubbleWrap, bubbleParams);

        ImageView pig = new ImageView(activity);
        pig.setImageResource(R.drawable.ic_pig_guide);
        pig.setScaleType(ImageView.ScaleType.FIT_CENTER);
        pig.setPadding(ShopUi.dp(activity, 5), ShopUi.dp(activity, 5), ShopUi.dp(activity, 5), ShopUi.dp(activity, 5));
        pig.setBackground(ShopUi.rounded(Color.rgb(255, 214, 224), ShopUi.dp(activity, 29)));
        pig.setElevation(ShopUi.dp(activity, 8));
        pig.setOnTouchListener((view, event) -> handleDrag(event));
        layer.addView(pig, new LinearLayout.LayoutParams(ShopUi.dp(activity, 58), ShopUi.dp(activity, 58)));

        FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(ShopUi.dp(activity, 300), -2, Gravity.RIGHT | Gravity.BOTTOM);
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
                if (!dragging && optionList != null) {
                    expanded = !expanded;
                    applyBubbleVisibility();
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
        if (optionList == null) {
            return;
        }
        optionList.removeAllViews();
        for (String question : questions(suggestions)) {
            TextView option = new TextView(activity);
            option.setText(question);
            option.setTextSize(13);
            option.setTextColor(Color.rgb(31, 41, 55));
            option.setGravity(Gravity.CENTER_VERTICAL);
            option.setMaxLines(2);
            option.setPadding(ShopUi.dp(activity, 12), ShopUi.dp(activity, 8), ShopUi.dp(activity, 12), ShopUi.dp(activity, 8));
            option.setBackground(ShopUi.rounded(Color.rgb(248, 249, 251), ShopUi.dp(activity, 14)));
            option.setOnClickListener(v -> openChatWithQuestion(((TextView) v).getText().toString()));
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
            params.setMargins(0, 0, 0, optionList.getChildCount() == 2 ? 0 : ShopUi.dp(activity, 7));
            optionList.addView(option, params);
        }
        applyBubbleVisibility();
        if (layer != null) {
            layer.bringToFront();
        }
    }

    private void applyBubbleVisibility() {
        if (optionList == null) {
            return;
        }
        for (int i = 0; i < optionList.getChildCount(); i++) {
            optionList.getChildAt(i).setVisibility(expanded || i == 0 ? View.VISIBLE : View.GONE);
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
            putQuestion(items, keyword.isEmpty() ? "帮我推荐几款高性价比商品" : "帮我找和「" + keyword + "」相关的好物");
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

    private static class BubbleTailView extends View {
        private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);
        private final Path path = new Path();

        BubbleTailView(Activity activity) {
            super(activity);
            paint.setColor(Color.rgb(232, 76, 137));
            paint.setStyle(Paint.Style.STROKE);
            paint.setStrokeWidth(ShopUi.dp(activity, 2));
            paint.setStrokeCap(Paint.Cap.ROUND);
            paint.setStrokeJoin(Paint.Join.ROUND);
        }

        @Override
        protected void onDraw(Canvas canvas) {
            super.onDraw(canvas);
            float w = getWidth();
            float h = getHeight();
            path.reset();
            path.moveTo(w * 0.08f, h * 0.25f);
            path.cubicTo(w * 0.42f, h * 0.26f, w * 0.48f, h * 0.72f, w * 0.9f, h * 0.62f);
            canvas.drawPath(path, paint);
            canvas.drawLine(w * 0.9f, h * 0.62f, w * 0.72f, h * 0.5f, paint);
            canvas.drawLine(w * 0.9f, h * 0.62f, w * 0.76f, h * 0.78f, paint);
        }
    }
}
