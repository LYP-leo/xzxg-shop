package com.xzxg.shop.ui;

import com.xzxg.shop.account.AccountUi;
import com.xzxg.shop.app.ShopApplication;
import com.xzxg.shop.chat.ChatActivity;
import com.xzxg.shop.chat.ChatHistoryDrawer;
import com.xzxg.shop.navigation.NavigationHelper;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.LocalChatStore;
import com.xzxg.shop.storage.SessionStore;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.View;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

public final class MainNavigationDrawer {
    public interface Callbacks {
        void beforeNavigate(String route);
    }

    private static final int PANEL_WIDTH_RATIO_PERCENT = 82;
    private static final String TAG_LAYER = "main_navigation_drawer_layer";

    private MainNavigationDrawer() {
    }

    public static void show(Activity activity, FrameLayout root, String activeRoute) {
        show(activity, root, activeRoute, null);
    }

    public static void show(Activity activity, FrameLayout root, String activeRoute, Callbacks callbacks) {
        if (activity == null || root == null) {
            return;
        }
        if (!(activity.getApplication() instanceof ShopApplication)) {
            return;
        }
        if (findOpenLayer(root) != null) {
            return;
        }
        ShopApplication app = (ShopApplication) activity.getApplication();
        SessionStore sessionStore = app.sessionStore();
        LocalChatStore chatStore = app.localChatStore();
        ApiClient api = app.apiClient();
        ChatHistoryDrawer historyState = new ChatHistoryDrawer(chatStore);

        FrameLayout layer = new FrameLayout(activity);
        layer.setTag(TAG_LAYER);
        layer.setBackgroundColor(Color.argb(92, 0, 0, 0));
        layer.setAlpha(0f);

        LinearLayout panel = new LinearLayout(activity);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setPadding(dp(activity, 14), dp(activity, 24), dp(activity, 14), dp(activity, 10));
        panel.setBackgroundColor(Color.WHITE);
        panel.setClickable(true);

        LinearLayout header = new LinearLayout(activity);
        header.setGravity(Gravity.CENTER_VERTICAL);
        EditText search = new EditText(activity);
        search.setHint("历史会话搜索");
        search.setTextSize(15);
        search.setSingleLine(true);
        search.setImeOptions(EditorInfo.IME_ACTION_DONE);
        search.setGravity(Gravity.CENTER_VERTICAL);
        search.setTextColor(Color.rgb(31, 41, 55));
        search.setHintTextColor(Color.rgb(156, 163, 175));
        search.setPadding(dp(activity, 16), 0, dp(activity, 16), 0);
        search.setBackground(ShopUi.rounded(Color.rgb(246, 247, 249), dp(activity, 12)));
        header.addView(search, new LinearLayout.LayoutParams(0, dp(activity, 44), 1));

        Button create = ShopUi.textOnlyButton(activity, "+");
        create.setTextSize(24);
        create.setOnClickListener(v -> {
            if (callbacks != null) {
                callbacks.beforeNavigate(Routes.CHAT);
            }
            close(root, layer, panel);
            openChat(activity, "", "");
        });
        LinearLayout.LayoutParams createParams = new LinearLayout.LayoutParams(dp(activity, 44), dp(activity, 44));
        createParams.leftMargin = dp(activity, 8);
        header.addView(create, createParams);
        panel.addView(header, new LinearLayout.LayoutParams(-1, dp(activity, 62)));

        LinearLayout navGroup = new LinearLayout(activity);
        navGroup.setOrientation(LinearLayout.VERTICAL);
        navGroup.setPadding(0, dp(activity, 10), 0, dp(activity, 10));
        navGroup.addView(navRow(activity, "💬", "AI导购", Routes.CHAT, activeRoute, root, layer, panel, callbacks));
        navGroup.addView(navRow(activity, "🏷", "商品", Routes.PRODUCTS, activeRoute, root, layer, panel, callbacks));
        navGroup.addView(navRow(activity, "🛒", "购物车", Routes.CART, activeRoute, root, layer, panel, callbacks));
        navGroup.addView(navRow(activity, "📦", "订单", Routes.ORDERS, activeRoute, root, layer, panel, callbacks));
        panel.addView(navGroup, new LinearLayout.LayoutParams(-1, -2));

        View divider = new View(activity);
        divider.setBackgroundColor(Color.rgb(238, 239, 242));
        panel.addView(divider, new LinearLayout.LayoutParams(-1, 1));

        ScrollView historyScroll = new ScrollView(activity);
        historyScroll.setFillViewport(true);
        LinearLayout historyList = new LinearLayout(activity);
        historyList.setOrientation(LinearLayout.VERTICAL);
        historyList.setPadding(0, dp(activity, 12), 0, dp(activity, 16));
        historyScroll.addView(historyList, new ScrollView.LayoutParams(-1, -2));
        panel.addView(historyScroll, new LinearLayout.LayoutParams(-1, 0, 1));

        LinearLayout bottom = bottomUserBar(activity, sessionStore, api, app);
        panel.addView(bottom, new LinearLayout.LayoutParams(-1, dp(activity, 68)));

        final int[] searchVersion = {0};
        renderInitialHistory(activity, historyList, historyState, "", root, layer, panel, callbacks);
        loadRemoteHistory(activity, historyList, historyState, api, sessionStore, "", 0, false, root, layer, panel, callbacks);
        search.setOnEditorActionListener((v, actionId, event) -> {
            if (actionId == EditorInfo.IME_ACTION_DONE) {
                hideKeyboard(activity, search);
                return true;
            }
            return false;
        });
        search.addTextChangedListener(new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            @Override public void onTextChanged(CharSequence s, int start, int before, int count) {}
            @Override public void afterTextChanged(Editable editable) {
                String query = editable.toString().trim();
                int version = ++searchVersion[0];
                historyState.resetPaging(query);
                renderInitialHistory(activity, historyList, historyState, query, root, layer, panel, callbacks);
                new Thread(() -> {
                    try {
                        Thread.sleep(300);
                    } catch (Exception ignored) {
                    }
                    activity.runOnUiThread(() -> {
                        if (version == searchVersion[0] && query.equals(search.getText().toString().trim())) {
                            loadRemoteHistory(activity, historyList, historyState, api, sessionStore, query, 0, false, root, layer, panel, callbacks);
                        }
                    });
                }).start();
            }
        });
        historyScroll.getViewTreeObserver().addOnScrollChangedListener(() -> {
            if (historyState.isLoading() || !historyState.hasMore()) {
                return;
            }
            int range = historyList.getHeight() - historyScroll.getHeight();
            if (range <= 0 || historyScroll.getScrollY() < range - dp(activity, 160)) {
                return;
            }
            int offset = historyState.offset();
            String query = historyState.query();
            List<LocalChatStore.SessionSummary> next = historyState.loadLocalPage(query, offset);
            appendHistoryRows(activity, historyList, next, root, layer, panel, callbacks);
            if (!sessionStore.token().isEmpty()) {
                loadRemoteHistory(activity, historyList, historyState, api, sessionStore, query, offset, true, root, layer, panel, callbacks);
            }
        });

        int panelWidth = activity.getResources().getDisplayMetrics().widthPixels * PANEL_WIDTH_RATIO_PERCENT / 100;
        panel.setTranslationX(-panelWidth);
        layer.setOnClickListener(v -> close(root, layer, panel));
        layer.addView(panel, new FrameLayout.LayoutParams(panelWidth, -1, Gravity.LEFT | Gravity.TOP));
        root.addView(layer, new FrameLayout.LayoutParams(-1, -1));
        layer.animate().alpha(1f).setDuration(160).start();
        panel.animate().translationX(0f).setDuration(180).start();
    }

    public static boolean closeIfOpen(FrameLayout root) {
        FrameLayout layer = findOpenLayer(root);
        if (layer == null || layer.getChildCount() == 0 || !(layer.getChildAt(0) instanceof LinearLayout)) {
            return false;
        }
        close(root, layer, (LinearLayout) layer.getChildAt(0));
        return true;
    }

    private static View navRow(Activity activity, String icon, String label, String route, String activeRoute, FrameLayout root, FrameLayout layer, LinearLayout panel, Callbacks callbacks) {
        LinearLayout row = new LinearLayout(activity);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(activity, 14), 0, dp(activity, 14), 0);
        boolean active = route.equals(activeRoute);
        row.setBackground(ShopUi.rounded(active ? Color.rgb(243, 244, 246) : Color.WHITE, dp(activity, 4)));
        row.setClickable(true);

        TextView iconView = new TextView(activity);
        iconView.setText(icon);
        iconView.setTextSize(22);
        iconView.setGravity(Gravity.CENTER);
        row.addView(iconView, new LinearLayout.LayoutParams(dp(activity, 44), dp(activity, 56)));

        TextView text = new TextView(activity);
        text.setText(label);
        text.setTextSize(18);
        text.setTextColor(Color.rgb(17, 24, 39));
        text.setTypeface(active ? Typeface.DEFAULT_BOLD : Typeface.DEFAULT);
        text.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(text, new LinearLayout.LayoutParams(0, -1, 1));

        row.setOnClickListener(v -> {
            close(root, layer, panel);
            if (active) {
                return;
            }
            if (callbacks != null) {
                callbacks.beforeNavigate(route);
            }
            NavigationHelper.navigateMainRoute(activity, route);
        });
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(activity, 56));
        params.bottomMargin = dp(activity, 4);
        row.setLayoutParams(params);
        return row;
    }

    private static void renderInitialHistory(Activity activity, LinearLayout historyList, ChatHistoryDrawer historyState, String query, FrameLayout root, FrameLayout layer, LinearLayout panel, Callbacks callbacks) {
        historyState.resetPaging(query);
        List<LocalChatStore.SessionSummary> histories = historyState.initialPage(query);
        historyList.removeAllViews();
        if (histories == null || histories.isEmpty()) {
            TextView empty = ShopUi.muted(activity, query == null || query.isEmpty() ? "暂无历史会话" : "没有匹配的历史会话");
            empty.setTag("empty_history");
            empty.setGravity(Gravity.CENTER);
            historyList.addView(empty, new LinearLayout.LayoutParams(-1, dp(activity, 56)));
            return;
        }
        appendHistoryRows(activity, historyList, histories, root, layer, panel, callbacks);
    }

    private static void appendHistoryRows(Activity activity, LinearLayout historyList, List<LocalChatStore.SessionSummary> histories, FrameLayout root, FrameLayout layer, LinearLayout panel, Callbacks callbacks) {
        if (historyList == null || histories == null || histories.isEmpty()) {
            return;
        }
        removeEmptyHistoryState(historyList);
        Set<String> existingIds = existingHistoryIds(historyList);
        for (LocalChatStore.SessionSummary item : histories) {
            if (item == null || item.localSessionId == null || !existingIds.add(item.localSessionId)) {
                continue;
            }
            historyList.addView(historyRow(activity, item, root, layer, panel, callbacks));
        }
    }

    private static View historyRow(Activity activity, LocalChatStore.SessionSummary item, FrameLayout root, FrameLayout layer, LinearLayout panel, Callbacks callbacks) {
        LinearLayout row = new LinearLayout(activity);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setTag("history:" + item.localSessionId);
        row.setPadding(dp(activity, 8), dp(activity, 7), dp(activity, 8), dp(activity, 7));
        row.setBackground(ShopUi.rounded(Color.WHITE, dp(activity, 12)));
        row.setClickable(true);

        TextView avatar = new TextView(activity);
        avatar.setText("💬");
        avatar.setTextSize(17);
        avatar.setGravity(Gravity.CENTER);
        avatar.setBackground(ShopUi.rounded(Color.rgb(237, 242, 255), dp(activity, 18)));
        row.addView(avatar, new LinearLayout.LayoutParams(dp(activity, 36), dp(activity, 36)));

        TextView title = new TextView(activity);
        String label = item.title == null || item.title.isEmpty() ? "导购会话" : item.title;
        title.setText(item.pinned ? "置顶 · " + label : label);
        title.setTextSize(15);
        title.setTextColor(Color.rgb(17, 24, 39));
        title.setMaxLines(2);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(0, -2, 1);
        titleParams.leftMargin = dp(activity, 14);
        row.addView(title, titleParams);

        row.setOnClickListener(v -> {
            if (callbacks != null) {
                callbacks.beforeNavigate(Routes.CHAT);
            }
            if (root != null && layer != null && panel != null) {
                close(root, layer, panel);
            }
            openChat(activity, item.localSessionId, item.serverSessionId);
        });
        return row;
    }

    private static void loadRemoteHistory(Activity activity, LinearLayout historyList, ChatHistoryDrawer historyState, ApiClient api, SessionStore sessionStore, String query, int offset, boolean append, FrameLayout root, FrameLayout layer, LinearLayout panel, Callbacks callbacks) {
        if (sessionStore == null || sessionStore.token().isEmpty() || api == null) {
            return;
        }
        historyState.setLoading(true);
        int page = offset / ChatHistoryDrawer.PAGE_SIZE + 1;
        new Thread(() -> {
            try {
                ApiClient.SessionPage remotePage = query == null || query.isEmpty()
                        ? api.sessionsPage(page, ChatHistoryDrawer.PAGE_SIZE)
                        : api.searchSessionsPage(query, page, ChatHistoryDrawer.PAGE_SIZE);
                List<LocalChatStore.SessionSummary> summaries = historyState.upsertRemoteSearchResults(remotePage.items);
                activity.runOnUiThread(() -> {
                    historyState.setLoading(false);
                    if (append) {
                        historyState.applyRemoteAppend(query, offset, summaries.size(), remotePage.hasMore);
                    } else {
                        historyState.applyRemoteReplace(query, offset, summaries.size(), remotePage.hasMore);
                    }
                    appendHistoryRows(activity, historyList, summaries, root, layer, panel, callbacks);
                });
            } catch (Exception error) {
                activity.runOnUiThread(() -> historyState.setLoading(false));
            }
        }).start();
    }

    private static LinearLayout bottomUserBar(Activity activity, SessionStore sessionStore, ApiClient api, ShopApplication app) {
        LinearLayout bar = new LinearLayout(activity);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(activity, 8), dp(activity, 8), dp(activity, 2), dp(activity, 8));
        bar.setBackgroundColor(Color.WHITE);
        View avatar = AccountUi.avatarView(activity, sessionStore, api, app.imageLoader(), dp(activity, 42), 17);
        bar.addView(avatar, new LinearLayout.LayoutParams(dp(activity, 42), dp(activity, 42)));
        TextView name = new TextView(activity);
        name.setText(sessionStore.token().isEmpty() ? "未登录" : (sessionStore.nickname().isEmpty() ? "用户名" : sessionStore.nickname()));
        name.setTextSize(15);
        name.setTypeface(Typeface.DEFAULT_BOLD);
        name.setTextColor(Color.rgb(31, 41, 55));
        LinearLayout.LayoutParams nameParams = new LinearLayout.LayoutParams(0, -1, 1);
        nameParams.leftMargin = dp(activity, 12);
        bar.addView(name, nameParams);
        return bar;
    }

    private static void openChat(Activity activity, String localSessionId, String serverSessionId) {
        Intent intent = new Intent(activity, ChatActivity.class);
        intent.putExtra(Routes.EXTRA_ROUTE, Routes.CHAT);
        if (localSessionId != null && !localSessionId.isEmpty()) {
            intent.putExtra(Routes.EXTRA_LOCAL_SESSION_ID, localSessionId);
        } else {
            intent.putExtra(Routes.EXTRA_NEW_CHAT, true);
        }
        if (serverSessionId != null && !serverSessionId.isEmpty()) {
            intent.putExtra(Routes.EXTRA_SERVER_SESSION_ID, serverSessionId);
        }
        intent.addFlags(Intent.FLAG_ACTIVITY_REORDER_TO_FRONT | Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_NO_ANIMATION);
        activity.startActivity(intent);
        activity.overridePendingTransition(0, 0);
    }

    private static void close(FrameLayout root, FrameLayout layer, LinearLayout panel) {
        if (root == null || layer == null || panel == null) {
            return;
        }
        int panelWidth = panel.getWidth();
        panel.animate().translationX(-panelWidth).setDuration(150).start();
        layer.animate().alpha(0f).setDuration(150).withEndAction(() -> root.removeView(layer)).start();
    }

    private static FrameLayout findOpenLayer(FrameLayout root) {
        if (root == null) {
            return null;
        }
        for (int i = root.getChildCount() - 1; i >= 0; i--) {
            View child = root.getChildAt(i);
            if (child instanceof FrameLayout && TAG_LAYER.equals(child.getTag())) {
                return (FrameLayout) child;
            }
        }
        return null;
    }

    private static Set<String> existingHistoryIds(LinearLayout historyList) {
        Set<String> ids = new HashSet<>();
        for (int i = 0; i < historyList.getChildCount(); i++) {
            Object tag = historyList.getChildAt(i).getTag();
            if (tag instanceof String && ((String) tag).startsWith("history:")) {
                ids.add(((String) tag).substring("history:".length()));
            }
        }
        return ids;
    }

    private static void removeEmptyHistoryState(LinearLayout historyList) {
        for (int i = historyList.getChildCount() - 1; i >= 0; i--) {
            Object tag = historyList.getChildAt(i).getTag();
            if ("empty_history".equals(tag)) {
                historyList.removeViewAt(i);
            }
        }
    }

    private static void hideKeyboard(Activity activity, View view) {
        InputMethodManager imm = (InputMethodManager) activity.getSystemService(Activity.INPUT_METHOD_SERVICE);
        if (imm != null) {
            imm.hideSoftInputFromWindow(view.getWindowToken(), 0);
        }
    }

    private static int dp(Activity activity, int value) {
        return ShopUi.dp(activity, value);
    }
}
