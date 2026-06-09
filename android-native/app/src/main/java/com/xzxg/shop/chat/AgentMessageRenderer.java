package com.xzxg.shop.chat;

import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.ui.ImageLoader;
import com.xzxg.shop.ui.ShopUi;

import android.content.Context;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayList;
import java.util.List;

public class AgentMessageRenderer {
    public interface CopyBinder {
        void bind(View view, String text);
    }

    public interface RouteNavigator {
        void navigate(String route);
    }

    public interface ProductActions {
        void openDetail(String productId);

        void addToCart(JSONObject item, Button sourceButton);
    }

    public interface FollowupListener {
        void onFollowupSelected(String question);
    }

    private final Context context;
    private final ApiClient api;
    private final ImageLoader imageLoader;
    private final ChatMarkdownRenderer markdownRenderer;
    private final CopyBinder copyBinder;
    private final RouteNavigator routeNavigator;
    private final ProductActions productActions;

    public AgentMessageRenderer(Context context, ApiClient api, ImageLoader imageLoader, ChatMarkdownRenderer markdownRenderer, CopyBinder copyBinder, RouteNavigator routeNavigator, ProductActions productActions) {
        this.context = context;
        this.api = api;
        this.imageLoader = imageLoader;
        this.markdownRenderer = markdownRenderer;
        this.copyBinder = copyBinder;
        this.routeNavigator = routeNavigator;
        this.productActions = productActions;
    }

    public View comparisonCard(JSONObject block) {
        LinearLayout card = panel();
        card.setPadding(dp(14), dp(14), dp(14), dp(14));
        card.addView(strong("商品对比"));
        View gap = new View(context);
        card.addView(gap, new LinearLayout.LayoutParams(1, dp(12)));
        JSONArray columns = block == null ? null : block.optJSONArray("columns");
        JSONArray rows = block == null ? null : block.optJSONArray("rows");
        HorizontalScrollView scroll = new HorizontalScrollView(context);
        scroll.setHorizontalScrollBarEnabled(true);
        LinearLayout table = new LinearLayout(context);
        table.setOrientation(LinearLayout.VERTICAL);
        table.setMinimumWidth((int) (context.getResources().getDisplayMetrics().widthPixels * 1.15f));
        if (columns != null) {
            table.addView(comparisonRow(columns, true));
        }
        if (rows != null) {
            for (int i = 0; i < rows.length(); i++) {
                JSONObject row = rows.optJSONObject(i);
                JSONArray values = row == null ? null : row.optJSONArray("values");
                if (values != null) {
                    table.addView(comparisonRow(values, false));
                }
            }
        }
        scroll.addView(table, new HorizontalScrollView.LayoutParams(-2, -2));
        card.addView(scroll, new LinearLayout.LayoutParams(-1, -2));
        bindCopy(card, block == null ? "" : block.toString());
        return card;
    }

    public View citationCard(JSONObject citation) {
        LinearLayout card = panel();
        card.setBackground(rounded(Color.rgb(248, 250, 252), dp(10)));
        card.addView(strong(citation == null ? "引用来源" : citation.optString("title", "引用来源")));
        if (citation != null) {
            card.addView(muted(citation.optString("snippet", "")));
            String source = citation.optString("source", "");
            if (!source.isEmpty()) {
                card.addView(muted("来源：" + source));
            }
        }
        return card;
    }

    public View warningCard(String message) {
        TextView view = card("提示", message == null || message.isEmpty() ? "当前信息需要进一步确认。" : message);
        view.setBackground(rounded(Color.rgb(255, 250, 235), dp(12)));
        return view;
    }

    public View cartStateCard(JSONObject cart) {
        LinearLayout card = panel();
        card.addView(strong("购物车"));
        JSONArray items = cart == null ? null : cart.optJSONArray("items");
        if (items == null || items.length() == 0) {
            card.addView(muted("购物车为空。"));
        } else {
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.optJSONObject(i);
                if (item != null) {
                    card.addView(muted((i + 1) + ". " + item.optString("name", "商品") + " x" + item.optInt("quantity", 1) + " · ¥" + item.optString("price", "0")));
                }
            }
        }
        JSONObject summary = cart == null ? null : cart.optJSONObject("summary");
        if (summary != null) {
            card.addView(strong("已选 " + summary.optInt("selectedCount", 0) + " 件，应付 ¥" + summary.optString("payAmount", "0")));
        }
        return card;
    }

    public View orderSummaryCard(JSONArray orders) {
        LinearLayout card = panel();
        card.addView(strong("订单已创建"));
        if (orders == null || orders.length() == 0) {
            card.addView(muted("暂无订单信息。"));
            return card;
        }
        for (int i = 0; i < orders.length(); i++) {
            JSONObject order = orders.optJSONObject(i);
            if (order != null) {
                card.addView(muted(order.optString("merchant_name", "商家") + " · ¥" + order.optString("total_amount", "0") + " · " + statusText(order.optString("status", ""))));
            }
        }
        return card;
    }

    public View discountPreviewCard(JSONObject block) {
        LinearLayout card = panel();
        JSONObject discount = serviceSummary(block, "discount");
        card.addView(strong(blockTitle(block, "优惠明细")));
        if (discount == null) {
            card.addView(muted("暂无可用优惠"));
            return card;
        }
        card.addView(muted("商品总额：¥" + discount.optString("total_amount", discount.optString("totalAmount", "0"))));
        card.addView(muted("优惠金额：¥" + discount.optString("discount_amount", discount.optString("discountAmount", "0"))));
        card.addView(strong("应付：¥" + discount.optString("pay_amount", discount.optString("payAmount", "0"))));
        JSONArray lines = discount.optJSONArray("lines");
        if (lines == null) {
            lines = serviceItems(block, "lines");
        }
        if (lines != null) {
            for (int i = 0; i < lines.length(); i++) {
                JSONObject line = lines.optJSONObject(i);
                if (line != null) {
                    card.addView(muted(line.optString("name", "优惠") + " -¥" + line.optString("amount", line.optString("discount_amount", "0"))));
                }
            }
        }
        return card;
    }

    public View couponListCard(JSONObject block) {
        return couponListCard(serviceItems(block, "coupons"));
    }

    public View couponListCard(JSONArray coupons) {
        LinearLayout card = panel();
        card.addView(strong("优惠券"));
        if (coupons == null || coupons.length() == 0) {
            card.addView(muted("暂无优惠券"));
            return card;
        }
        for (int i = 0; i < coupons.length(); i++) {
            JSONObject coupon = coupons.optJSONObject(i);
            if (coupon != null) {
                card.addView(muted(coupon.optString("name", "优惠券") + " · 满 " + coupon.optString("threshold_amount", "0") + " 减 " + coupon.optString("discount_amount", "0")));
            }
        }
        return card;
    }

    public View reviewSummaryCard(JSONObject block) {
        LinearLayout card = panel();
        card.addView(strong(blockTitle(block, "评价摘要")));
        JSONObject summary = serviceSummary(block, "summary");
        if (summary != null) {
            String average = summary.optString("average_rating", summary.optString("rating_avg", ""));
            String count = summary.optString("review_count", summary.optString("rating_count", ""));
            if (!average.isEmpty() || !count.isEmpty()) {
                card.addView(muted("评分 " + (average.isEmpty() ? "-" : average) + " · " + (count.isEmpty() ? "0" : count) + " 条评价"));
            }
        }
        JSONArray items = serviceItems(block, "reviews");
        if (items == null || items.length() == 0) {
            card.addView(muted(block == null ? "暂无评价摘要" : block.optString("message", "暂无评价摘要")));
            return card;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject review = items.optJSONObject(i);
            if (review != null) {
                String rating = review.optString("rating", "");
                String content = review.optString("content", review.optString("summary", ""));
                card.addView(muted((rating.isEmpty() ? "" : "★ " + rating + "  ") + content));
            }
        }
        return card;
    }

    public View afterSalesPolicyCard(JSONObject block) {
        LinearLayout card = panel();
        card.addView(strong(blockTitle(block, "售后规则")));
        String message = block == null ? "" : block.optString("message", "");
        if (!message.isEmpty()) {
            card.addView(muted(message));
        }
        JSONArray items = serviceItems(block, "policies");
        if (items == null || items.length() == 0) {
            card.addView(muted("暂无可展示的售后规则。"));
            return card;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                String name = item.optString("name", item.optString("title", "规则"));
                String description = item.optString("description", item.optString("content", ""));
                card.addView(muted(name + (description.isEmpty() ? "" : "：" + description)));
            }
        }
        return card;
    }

    public View navigationActionCard(JSONObject block) {
        LinearLayout card = panel();
        JSONObject action = block == null ? null : block.optJSONObject("action");
        String label = action == null ? blockTitle(block, "查看详情") : action.optString("label", blockTitle(block, "查看详情"));
        String route = action == null ? "" : action.optString("route", action.optString("target", ""));
        String message = block == null ? "" : block.optString("message", "");
        if (!message.isEmpty()) {
            card.addView(muted(message));
        }
        TextView view = strong(label + "  >");
        view.setOnClickListener(v -> {
            if (routeNavigator != null) {
                routeNavigator.navigate(route);
            }
        });
        card.addView(view);
        return card;
    }

    public View productCard(JSONObject item) {
        LinearLayout card = new LinearLayout(context);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setPadding(dp(12), dp(10), dp(12), dp(10));
        card.setBackground(rounded(Color.rgb(248, 249, 251), dp(14)));
        LinearLayout.LayoutParams cardParams = new LinearLayout.LayoutParams(-1, -2);
        cardParams.setMargins(0, dp(6), 0, dp(8));
        card.setLayoutParams(cardParams);

        LinearLayout row = new LinearLayout(context);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(productImage(productField(item, "imageUrl", "image_url"), 72), new LinearLayout.LayoutParams(dp(72), dp(72)));

        LinearLayout info = new LinearLayout(context);
        info.setOrientation(LinearLayout.VERTICAL);
        TextView name = strong(item == null ? "商品" : item.optString("name", "商品"));
        name.setTextSize(15);
        name.setMaxLines(2);
        info.addView(name);
        info.addView(muted((item == null ? "" : item.optString("brand", "")) + " · " + productField(item, "merchantName", "merchant_name", "商家")));
        TextView price = strong("¥" + (item == null ? "0" : item.optString("price", "0")));
        price.setTextSize(16);
        info.addView(price);
        String reason = productField(item, "recommendReason", "recommend_reason");
        if (!reason.isEmpty()) {
            TextView reasonView = muted(reason);
            reasonView.setMaxLines(2);
            info.addView(reasonView);
        }
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = dp(10);
        row.addView(info, infoParams);
        card.addView(row);

        LinearLayout actions = new LinearLayout(context);
        Button detail = ShopUi.secondaryButton(context, "详情");
        detail.setOnClickListener(v -> {
            if (productActions != null) {
                productActions.openDetail(productField(item, "productId", "product_id", "id", ""));
            }
        });
        LinearLayout.LayoutParams detailParams = new LinearLayout.LayoutParams(0, dp(42), 1);
        detailParams.rightMargin = dp(8);
        actions.addView(detail, detailParams);
        Button add = ShopUi.primaryButton(context, "加入购物车");
        add.setOnClickListener(v -> {
            if (productActions != null) {
                productActions.addToCart(item, add);
            }
        });
        actions.addView(add, new LinearLayout.LayoutParams(0, dp(42), 1));
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(-1, dp(42));
        actionParams.topMargin = dp(8);
        card.addView(actions, actionParams);

        bindCopy(card, productCopyMarkdown(item));
        return card;
    }

    public View followupsView(JSONArray questions, FollowupListener listener) {
        if (questions == null || questions.length() == 0) {
            return null;
        }
        LinearLayout wrap = new LinearLayout(context);
        wrap.setOrientation(LinearLayout.VERTICAL);
        wrap.setPadding(0, dp(8), 0, dp(6));
        TextView title = muted("您还可以继续追问：");
        title.setGravity(Gravity.CENTER);
        wrap.addView(title, new LinearLayout.LayoutParams(-1, -2));

        HorizontalScrollView scroll = new HorizontalScrollView(context);
        scroll.setHorizontalScrollBarEnabled(false);
        LinearLayout chips = new LinearLayout(context);
        chips.setOrientation(LinearLayout.HORIZONTAL);
        for (int i = 0; i < questions.length(); i++) {
            String question = questions.optString(i, "");
            if (question.isEmpty()) {
                continue;
            }
            TextView chip = new TextView(context);
            chip.setText(question);
            chip.setTextSize(13);
            chip.setTextColor(Color.rgb(55, 65, 81));
            chip.setGravity(Gravity.CENTER);
            chip.setPadding(dp(12), 0, dp(12), 0);
            chip.setBackground(rounded(Color.rgb(238, 239, 241), dp(19)));
            chip.setOnClickListener(v -> {
                if (listener != null) {
                    listener.onFollowupSelected(((TextView) v).getText().toString());
                }
            });
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-2, dp(38));
            params.setMargins(0, 0, dp(8), 0);
            chips.addView(chip, params);
        }
        scroll.addView(chips);
        wrap.addView(scroll, new LinearLayout.LayoutParams(-1, dp(42)));
        return wrap;
    }

    public void collectHistoricalThinkingSegment(JSONArray thoughts, JSONObject segment) {
        if (thoughts == null || segment == null) {
            return;
        }
        JSONObject thought = segment.optJSONObject("thought");
        if (thought != null) {
            thoughts.put(thought);
            return;
        }
        JSONObject thinking = segment.optJSONObject("thinking");
        if (thinking == null) {
            return;
        }
        JSONArray stages = thinking.optJSONArray("stages");
        if (stages == null) {
            return;
        }
        for (int i = 0; i < stages.length(); i++) {
            JSONObject item = stages.optJSONObject(i);
            if (item == null) {
                continue;
            }
            try {
                JSONObject converted = new JSONObject();
                String stage = item.optString("stage", "");
                converted.put("id", stage);
                converted.put("title", thinkingStageTitle(thinkingStageKey(stage), item.optString("title", "")));
                converted.put("status", normalizedHistoricalThinkingStatus(item.optString("status", "done")));
                converted.put("summary", historicalThinkingSummary(item));
                converted.put("order", historicalThinkingOrder(stage, i + 1));
                thoughts.put(converted);
            } catch (Exception ignored) {
            }
        }
    }

    public JSONArray flushHistoricalThinkingPanel(LinearLayout parent, JSONArray thoughts, ScrollView scrollView) {
        if (parent == null || thoughts == null || thoughts.length() == 0) {
            return new JSONArray();
        }
        renderHistoricalThinkingPanel(parent, thoughts, scrollView);
        return new JSONArray();
    }

    public void renderHistoricalTextSegment(LinearLayout bubble, String text) {
        if (bubble == null || text == null || text.trim().isEmpty()) {
            return;
        }
        List<ChatMarkdownRenderer.Part> parts = markdownRenderer == null ? new ArrayList<>() : markdownRenderer.splitParts(text);
        if (parts.isEmpty()) {
            parts = new ArrayList<>();
            parts.add(new ChatMarkdownRenderer.Part(false, text));
        }
        for (ChatMarkdownRenderer.Part part : parts) {
            if (part.table) {
                if (markdownRenderer != null) {
                    markdownRenderer.addTableToBubble(bubble, part.content);
                }
            } else if (!part.content.trim().isEmpty()) {
                addHistoricalTextView(bubble, part.content);
            }
        }
    }

    public void renderHistoricalTableSegment(LinearLayout bubble, String markdown) {
        String cleaned = markdown == null ? "" : markdown.trim();
        if (bubble == null || cleaned.isEmpty()) {
            return;
        }
        if (markdownRenderer != null && markdownRenderer.containsTable(cleaned)) {
            markdownRenderer.addTableToBubble(bubble, cleaned);
        } else {
            addHistoricalTextView(bubble, cleaned);
        }
    }

    private void renderHistoricalThinkingPanel(LinearLayout parent, JSONArray thoughts, ScrollView scrollView) {
        LinearLayout panel = new LinearLayout(context);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setPadding(dp(4), dp(6), dp(4), dp(6));
        panel.setBackgroundColor(Color.TRANSPARENT);
        int bubbleWidth = (int) (context.getResources().getDisplayMetrics().widthPixels * 0.82f);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(bubbleWidth, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = Gravity.LEFT;
        params.setMargins(0, dp(4), 0, dp(8));

        TextView title = new TextView(context);
        title.setTextSize(15);
        title.setTypeface(Typeface.DEFAULT);
        title.setTextColor(Color.rgb(156, 163, 175));
        panel.addView(title, new LinearLayout.LayoutParams(-1, -2));

        LinearLayout list = new LinearLayout(context);
        list.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams listParams = new LinearLayout.LayoutParams(-1, -2);
        listParams.topMargin = dp(8);
        panel.addView(list, listParams);

        List<JSONObject> steps = sortedHistoricalThinkingSteps(thoughts);
        final boolean[] expanded = {false};
        final Runnable[] render = new Runnable[1];
        render[0] = () -> {
            String action = expanded[0] ? "收起" : "展开";
            title.setText("✦  已完成思考  " + action);
            list.removeAllViews();
            list.setVisibility(expanded[0] ? View.VISIBLE : View.GONE);
            if (!expanded[0]) {
                return;
            }
            for (JSONObject step : steps) {
                list.addView(historicalThinkingStepView(step));
            }
        };
        title.setOnClickListener(v -> {
            int beforeScrollY = scrollView == null ? 0 : scrollView.getScrollY();
            expanded[0] = !expanded[0];
            render[0].run();
            if (scrollView != null) {
                scrollView.post(() -> scrollView.setScrollY(beforeScrollY));
            }
        });
        render[0].run();
        parent.addView(panel, params);
    }

    private void addHistoricalTextView(LinearLayout bubble, String markdown) {
        TextView textView = markdownRenderer == null ? new TextView(context) : markdownRenderer.markdownTextView(markdown);
        textView.setPadding(0, 0, 0, dp(6));
        bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
    }

    private View historicalThinkingStepView(JSONObject step) {
        LinearLayout wrap = new LinearLayout(context);
        wrap.setOrientation(LinearLayout.HORIZONTAL);
        wrap.setPadding(0, dp(4), 0, dp(4));

        String status = normalizedHistoricalThinkingStatus(step == null ? "" : step.optString("status", "completed"));
        TextView icon = new TextView(context);
        icon.setText("failed".equals(status) ? "!" : "✓");
        icon.setTextSize(16);
        icon.setGravity(Gravity.TOP | Gravity.CENTER_HORIZONTAL);
        icon.setTextColor("failed".equals(status) ? Color.rgb(220, 38, 38) : Color.rgb(143, 150, 165));
        wrap.addView(icon, new LinearLayout.LayoutParams(dp(24), -2));

        LinearLayout body = new LinearLayout(context);
        body.setOrientation(LinearLayout.VERTICAL);
        TextView title = new TextView(context);
        title.setText((step == null ? "处理步骤" : step.optString("title", "处理步骤")) + ("failed".equals(status) ? "失败" : "完成"));
        title.setTextSize(15);
        title.setTextColor(Color.rgb(75, 85, 99));
        title.setTypeface(Typeface.DEFAULT_BOLD);
        body.addView(title, new LinearLayout.LayoutParams(-1, -2));

        String text = step == null ? "" : step.optString("summary", step.optString("detail", ""));
        if (!text.trim().isEmpty()) {
            TextView summary = new TextView(context);
            summary.setText(text.trim());
            summary.setTextSize(14);
            summary.setTextColor(Color.rgb(138, 145, 158));
            summary.setLineSpacing(4, 1);
            LinearLayout.LayoutParams summaryParams = new LinearLayout.LayoutParams(-1, -2);
            summaryParams.topMargin = dp(5);
            body.addView(summary, summaryParams);
        }
        wrap.addView(body, new LinearLayout.LayoutParams(0, -2, 1));
        return wrap;
    }

    public LinearLayout embeddedProductBubble() {
        LinearLayout bubble = new LinearLayout(context);
        bubble.setOrientation(LinearLayout.VERTICAL);
        bubble.setPadding(dp(10), dp(8), dp(10), dp(4));
        bubble.setBackground(rounded(Color.WHITE, dp(16)));
        int bubbleWidth = (int) (context.getResources().getDisplayMetrics().widthPixels * 0.82f);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(bubbleWidth, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = Gravity.LEFT;
        params.setMargins(0, dp(4), 0, dp(8));
        bubble.setLayoutParams(params);
        return bubble;
    }

    public String productField(JSONObject item, String primary, String fallback) {
        return productField(item, primary, fallback, "");
    }

    public String productField(JSONObject item, String primary, String fallback, String defaultValue) {
        if (item == null) {
            return defaultValue;
        }
        String value = item.optString(primary, "");
        if (value == null || value.trim().isEmpty()) {
            value = item.optString(fallback, "");
        }
        return value == null || value.trim().isEmpty() ? defaultValue : value;
    }

    public String productField(JSONObject item, String primary, String fallback, String secondFallback, String defaultValue) {
        String value = productField(item, primary, fallback, "");
        if (value.isEmpty() && item != null) {
            value = item.optString(secondFallback, "");
        }
        return value == null || value.trim().isEmpty() ? defaultValue : value;
    }

    private View comparisonRow(JSONArray values, boolean header) {
        LinearLayout row = new LinearLayout(context);
        row.setOrientation(LinearLayout.HORIZONTAL);
        row.setBackgroundColor(header ? Color.rgb(243, 244, 246) : Color.WHITE);
        int count = Math.max(1, values == null ? 0 : values.length());
        for (int i = 0; i < count; i++) {
            TextView cell = new TextView(context);
            cell.setText(values == null ? "" : values.optString(i, ""));
            cell.setTextSize(13);
            cell.setTextColor(header ? Color.rgb(17, 24, 39) : Color.rgb(75, 85, 99));
            cell.setTypeface(Typeface.DEFAULT, header ? Typeface.BOLD : Typeface.NORMAL);
            cell.setPadding(dp(10), dp(9), dp(10), dp(9));
            cell.setGravity(Gravity.CENTER_VERTICAL);
            row.addView(cell, new LinearLayout.LayoutParams(i == 0 ? dp(150) : dp(126), -2));
        }
        return row;
    }

    private View productImage(String imageUrl, int sizeDp) {
        FrameLayout frame = new FrameLayout(context);
        frame.setBackground(rounded(Color.rgb(243, 244, 246), dp(12)));
        ImageView image = new ImageView(context);
        image.setScaleType(ImageView.ScaleType.CENTER_CROP);
        String url = api == null ? "" : api.absoluteUrl(imageUrl);
        if (!url.isEmpty() && imageLoader != null) {
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
            imageLoader.load(image, url);
        }
        frame.setLayoutParams(new LinearLayout.LayoutParams(dp(sizeDp), dp(sizeDp)));
        return frame;
    }

    private String productCopyMarkdown(JSONObject item) {
        if (item == null) {
            return "";
        }
        StringBuilder builder = new StringBuilder();
        builder.append("**").append(item.optString("name", "商品")).append("**");
        String brand = item.optString("brand", "");
        if (!brand.isEmpty()) {
            builder.append("\n").append(brand);
        }
        builder.append("\n¥").append(item.optString("price", "0"));
        String reason = productField(item, "recommendReason", "recommend_reason");
        if (!reason.isEmpty()) {
            builder.append("\n").append(reason);
        }
        return builder.toString();
    }

    private String normalizedHistoricalThinkingStatus(String status) {
        if (status == null || status.trim().isEmpty()) {
            return "completed";
        }
        String value = status.trim();
        if ("error".equals(value)) {
            return "failed";
        }
        return value;
    }

    private List<JSONObject> sortedHistoricalThinkingSteps(JSONArray thoughts) {
        List<JSONObject> steps = new ArrayList<>();
        if (thoughts != null) {
            for (int i = 0; i < thoughts.length(); i++) {
                JSONObject step = thoughts.optJSONObject(i);
                if (step != null) {
                    steps.add(step);
                }
            }
        }
        steps.sort((a, b) -> {
            int orderA = a.optInt("order", 99);
            int orderB = b.optInt("order", 99);
            if (orderA != orderB) {
                return orderA - orderB;
            }
            return a.optString("id", "").compareTo(b.optString("id", ""));
        });
        return steps;
    }

    private String historicalThinkingSummary(JSONObject item) {
        if (item == null) {
            return "";
        }
        String text = item.optString("text", "");
        if (!text.trim().isEmpty()) {
            return text.trim();
        }
        JSONArray items = item.optJSONArray("items");
        if (items == null || items.length() == 0) {
            return item.optString("summary", item.optString("detail", "")).trim();
        }
        StringBuilder summary = new StringBuilder();
        for (int i = 0; i < items.length(); i++) {
            JSONObject child = items.optJSONObject(i);
            if (child == null) {
                continue;
            }
            String title = child.optString("title", child.optString("text", ""));
            if (title.trim().isEmpty()) {
                continue;
            }
            if (summary.length() > 0) {
                summary.append("\n");
            }
            summary.append(title.trim());
        }
        return summary.toString();
    }

    private int historicalThinkingOrder(String id, int fallback) {
        String key = thinkingStageKey(id);
        if ("intent".equals(id) || "user_need".equals(key)) {
            return 1;
        }
        if ("retrieve".equals(id) || "buyer_experience".equals(key)) {
            return 2;
        }
        if ("answer".equals(id) || "answer_summary".equals(key)) {
            return 3;
        }
        return fallback;
    }

    private String thinkingStageKey(String stage) {
        if ("buyer_experience".equals(stage) || "answer_summary".equals(stage) || "user_need".equals(stage)) {
            return stage;
        }
        if ("intent".equals(stage) || "query_rewrite".equals(stage)) {
            return "user_need";
        }
        if ("retrieval".equals(stage) || "tool".equals(stage)) {
            return "buyer_experience";
        }
        if ("answer".equals(stage) || "done".equals(stage)) {
            return "answer_summary";
        }
        return "user_need";
    }

    private String thinkingStageTitle(String stage, String fallback) {
        if (fallback != null && !fallback.trim().isEmpty()) {
            return fallback.trim();
        }
        if ("buyer_experience".equals(stage)) {
            return "查询买手团经验";
        }
        if ("answer_summary".equals(stage)) {
            return "总结答案";
        }
        return "分析用户需求";
    }

    private String blockTitle(JSONObject block, String fallback) {
        if (block == null) {
            return fallback;
        }
        String title = block.optString("title", "");
        return title.isEmpty() ? fallback : title;
    }

    private JSONObject serviceSummary(JSONObject block, String legacyKey) {
        if (block == null) {
            return null;
        }
        JSONObject summary = block.optJSONObject("summary");
        if (summary != null) {
            return summary;
        }
        return legacyKey == null ? null : block.optJSONObject(legacyKey);
    }

    private JSONArray serviceItems(JSONObject block, String legacyKey) {
        if (block == null) {
            return null;
        }
        JSONArray items = block.optJSONArray("items");
        if (items != null) {
            return items;
        }
        return legacyKey == null ? null : block.optJSONArray(legacyKey);
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

    private LinearLayout panel() {
        return ShopUi.panel(context);
    }

    private TextView strong(String text) {
        return ShopUi.strong(context, text);
    }

    private TextView muted(String text) {
        return ShopUi.muted(context, text);
    }

    private TextView card(String title, String detail) {
        return ShopUi.card(context, title, detail);
    }

    private GradientDrawable rounded(int color, int radius) {
        return ShopUi.rounded(color, radius);
    }

    private int dp(int value) {
        return ShopUi.dp(context, value);
    }

    private void bindCopy(View view, String text) {
        if (copyBinder != null) {
            copyBinder.bind(view, text);
        }
    }
}
