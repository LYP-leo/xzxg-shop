package com.xzxg.shop;

import android.app.Activity;
import android.app.AlertDialog;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.os.Bundle;
import android.view.ViewGroup;
import android.view.Gravity;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowManager;
import android.view.animation.Animation;
import android.view.animation.TranslateAnimation;
import android.view.inputmethod.EditorInfo;
import android.widget.Button;
import android.widget.CheckBox;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;
import android.text.Editable;
import android.text.InputType;
import android.text.TextWatcher;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.ArrayDeque;
import java.util.Deque;
import java.util.List;

public class MainActivity extends Activity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private SessionStore sessionStore;
    private LocalChatStore chatStore;
    private ApiClient api;
    private ImageLoader imageLoader;
    private FrameLayout root;
    private LinearLayout content;
    private LinearLayout chatList;
    private ScrollView chatScroll;
    private EditText input;
    private TextView actionButton;
    private FrameLayout drawerLayer;
    private LinearLayout drawerPanel;
    private String localSessionId;
    private String serverSessionId = "";
    private TextView activeAssistant;
    private TextView loadingAssistant;
    private StringBuilder activeAssistantMarkdown;
    private boolean streaming;
    private String activePage = "chat";
    private String lastProductKeyword = "";
    private String lastCategoryId = "";
    private final Deque<Runnable> backStack = new ArrayDeque<>();
    private Toast activeToast;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureSystemBars();
        sessionStore = new SessionStore(this);
        chatStore = new LocalChatStore(this);
        api = new ApiClient(sessionStore);
        imageLoader = new ImageLoader();
        createFreshLocalSession();
        renderChatHome();
        loadHomeCopy();
    }

    @Override
    public void onBackPressed() {
        if (drawerLayer != null) {
            closeDrawerAnimated();
            return;
        }
        if (!backStack.isEmpty()) {
            Runnable action = backStack.pop();
            action.run();
            return;
        }
        if (isPrimaryPage(activePage)) {
            showDrawer();
            return;
        }
        renderChatHome();
        loadHomeCopy();
    }

    private boolean isPrimaryPage(String page) {
        return "chat".equals(page) || "products".equals(page) || "cart".equals(page) || "orders".equals(page) || "profile".equals(page);
    }

    private void createFreshLocalSession() {
        localSessionId = "local_sess_" + System.currentTimeMillis();
        chatStore.ensureSession(localSessionId, "新的导购会话");
        serverSessionId = "";
    }

    private void baseScreen() {
        root = new FrameLayout(this);
        root.setBackgroundColor(BG_COLOR);
        root.setClipChildren(false);
        root.setClipToPadding(false);
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setClipChildren(false);
        content.setClipToPadding(false);
        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        setContentView(root);
    }

    private void renderChatHome() {
        closeDrawer();
        activePage = "chat";
        backStack.clear();
        baseScreen();

        content.addView(createTopBar("AI导购", null), new LinearLayout.LayoutParams(-1, dp(56)));
        content.addView(createChatMessageLayer(), new LinearLayout.LayoutParams(-1, 0, 1));
        content.addView(createComposerBar(), new LinearLayout.LayoutParams(-1, dp(72)));
        bindImeInsets();
    }

    private void bindImeInsets() {
        root.setOnApplyWindowInsetsListener((view, insets) -> {
            int imeBottom = insets.getInsets(WindowInsets.Type.ime()).bottom;
            content.setPadding(0, 0, 0, imeBottom);
            return insets;
        });
    }

    private View createTopBar(String titleText, String rightText) {
        LinearLayout toolbar = new LinearLayout(this);
        toolbar.setGravity(Gravity.CENTER_VERTICAL);
        toolbar.setPadding(dp(16), 0, dp(16), 0);
        toolbar.setBackgroundColor(BG_COLOR);
        toolbar.setClickable(true);
        toolbar.setElevation(dp(1));

        Button menu = transparentIconButton("☰");
        menu.setTextSize(22);
        menu.setOnClickListener(v -> showDrawer());
        toolbar.addView(menu, new LinearLayout.LayoutParams(dp(40), dp(40)));

        TextView title = new TextView(this);
        title.setText(titleText);
        title.setTextSize(18);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        title.setTextColor(Color.rgb(20, 24, 30));
        title.setGravity(Gravity.CENTER_VERTICAL);
        title.setPadding(dp(10), 0, dp(8), 0);
        toolbar.addView(title, new LinearLayout.LayoutParams(0, -1, 1));

        View account = topBarAccountView(rightText);
        if (account != null) {
            toolbar.addView(account, new LinearLayout.LayoutParams(-2, dp(40)));
        }

        return toolbar;
    }

    private View topBarAccountView(String rightText) {
        if (rightText != null && !rightText.isEmpty()) {
            TextView status = new TextView(this);
            status.setText(rightText);
            status.setTextSize(15);
            status.setTextColor(Color.rgb(75, 85, 99));
            status.setPadding(dp(12), 0, dp(12), 0);
            status.setOnClickListener(v -> renderProfile());
            status.setGravity(Gravity.CENTER);
            return status;
        }
        if (sessionStore.token().isEmpty()) {
            TextView login = new TextView(this);
            login.setText("登录");
            login.setTextSize(15);
            login.setTypeface(Typeface.DEFAULT_BOLD);
            login.setTextColor(Color.rgb(20, 24, 30));
            login.setGravity(Gravity.CENTER);
            login.setPadding(dp(12), 0, dp(12), 0);
            login.setOnClickListener(v -> renderProfile());
            return login;
        }
        LinearLayout user = new LinearLayout(this);
        user.setGravity(Gravity.CENTER_VERTICAL);
        user.setPadding(dp(6), 0, 0, 0);
        user.setOnClickListener(v -> renderProfile());
        ImageView avatarImage = new ImageView(this);
        avatarImage.setScaleType(ImageView.ScaleType.CENTER_CROP);
        avatarImage.setBackground(rounded(Color.rgb(236, 244, 255), dp(16)));
        String avatarUrl = api.absoluteUrl(sessionStore.avatarUrl());
        if (!avatarUrl.isEmpty()) {
            imageLoader.load(avatarImage, avatarUrl);
            user.addView(avatarImage, new LinearLayout.LayoutParams(dp(32), dp(32)));
        } else {
            TextView avatar = new TextView(this);
            avatar.setText(initialOf(sessionStore.nickname()));
            avatar.setTextSize(14);
            avatar.setTypeface(Typeface.DEFAULT_BOLD);
            avatar.setGravity(Gravity.CENTER);
            avatar.setTextColor(Color.rgb(30, 64, 175));
            avatar.setBackground(rounded(Color.rgb(219, 234, 254), dp(16)));
            user.addView(avatar, new LinearLayout.LayoutParams(dp(32), dp(32)));
        }
        TextView name = new TextView(this);
        name.setText(shortName(sessionStore.nickname()));
        name.setTextSize(14);
        name.setTextColor(Color.rgb(31, 41, 55));
        LinearLayout.LayoutParams nameParams = new LinearLayout.LayoutParams(-2, -2);
        nameParams.leftMargin = dp(6);
        user.addView(name, nameParams);
        return user;
    }

    private View createChatMessageLayer() {
        chatScroll = new ScrollView(this);
        chatScroll.setFillViewport(true);
        chatList = new LinearLayout(this);
        chatList.setOrientation(LinearLayout.VERTICAL);
        chatList.setGravity(Gravity.CENTER_HORIZONTAL);
        chatList.setPadding(dp(16), dp(12), dp(16), dp(12));
        chatScroll.addView(chatList, new ScrollView.LayoutParams(-1, -2));

        TextView welcome = new TextView(this);
        welcome.setTag("welcome");
        welcome.setText("");
        welcome.setTextSize(22);
        welcome.setTextColor(Color.rgb(20, 24, 30));
        welcome.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams welcomeParams = new LinearLayout.LayoutParams(-1, -2);
        welcomeParams.topMargin = dp(96);
        chatList.addView(welcome, welcomeParams);

        TextView hint = muted(sessionStore.token().isEmpty() ? "登录以提供个性化建议" : "");
        hint.setGravity(Gravity.CENTER);
        chatList.addView(hint, new LinearLayout.LayoutParams(-1, -2));

        LinearLayout suggestions = new LinearLayout(this);
        suggestions.setTag("suggestions");
        suggestions.setOrientation(LinearLayout.HORIZONTAL);
        HorizontalScrollView scroll = new HorizontalScrollView(this);
        scroll.setHorizontalScrollBarEnabled(false);
        scroll.addView(suggestions);
        chatList.addView(scroll, new LinearLayout.LayoutParams(-1, -2));

        renderStoredMessagesIfAny();

        return chatScroll;
    }

    private View createComposerBar() {
        LinearLayout outer = new LinearLayout(this);
        outer.setGravity(Gravity.CENTER_VERTICAL);
        outer.setPadding(dp(14), dp(8), dp(14), dp(8));
        outer.setBackgroundColor(BG_COLOR);
        outer.setClickable(true);
        outer.setClipChildren(false);
        outer.setClipToPadding(false);

        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(12), 0, dp(7), 0);
        bar.setBackground(rounded(Color.WHITE, dp(26)));
        bar.setElevation(dp(2));
        bar.setClipChildren(false);
        bar.setClipToPadding(false);

        Button add = transparentIconButton("+");
        add.setTextSize(24);
        add.setOnClickListener(v -> toastLine("图片输入入口已预留，后续接入相册/拍照。"));
        bar.addView(add, new LinearLayout.LayoutParams(dp(40), dp(52)));

        input = new EditText(this);
        input.setHint("");
        input.setSingleLine(false);
        input.setMaxLines(4);
        input.setImeOptions(EditorInfo.IME_ACTION_SEND);
        input.setTextSize(15);
        input.setBackground(new ColorDrawable(Color.TRANSPARENT));
        input.setPadding(dp(8), 0, dp(8), 0);
        input.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {
            }

            @Override
            public void onTextChanged(CharSequence s, int start, int before, int count) {
                if (!streaming) {
                    actionButton.setText(s.toString().trim().isEmpty() ? "≋" : "➤");
                }
            }

            @Override
            public void afterTextChanged(Editable s) {
            }
        });
        bar.addView(input, new LinearLayout.LayoutParams(0, dp(52), 1));

        actionButton = roundActionButton("≋");
        actionButton.setOnClickListener(v -> {
            if (streaming) {
                toastLine("停止生成能力已预留，后续接入 run cancel。");
                return;
            }
            String text = input.getText().toString().trim();
            if (text.isEmpty()) {
                toastLine("语音输入能力待接入");
                return;
            }
            sendMessage(text);
        });
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(dp(42), dp(42));
        bar.addView(actionButton, actionParams);
        outer.addView(bar, new LinearLayout.LayoutParams(-1, dp(52)));
        return outer;
    }

    private TextView roundActionButton(String text) {
        TextView button = new TextView(this);
        button.setText(text);
        button.setTextSize(19);
        button.setTextColor(Color.WHITE);
        button.setGravity(Gravity.CENTER);
        button.setIncludeFontPadding(false);
        button.setBackground(rounded(Color.BLACK, dp(24)));
        button.setClickable(true);
        return button;
    }

    private void loadHomeCopy() {
        new Thread(() -> {
            try {
                JSONObject home = api.getAgentHome();
                runOnUiThread(() -> applyHomeCopy(home));
            } catch (Exception ignored) {
                runOnUiThread(() -> applyHomeCopy(new JSONObject()));
            }
        }).start();
    }

    private void applyHomeCopy(JSONObject home) {
        if (chatList == null || chatList.getTag() != null) {
            return;
        }
        TextView welcome = findTaggedText(chatList, "welcome");
        if (welcome != null) {
            String text = home.optString("welcome_text", "");
            welcome.setText(text.isEmpty() ? "这里展示新会话欢迎语" : text);
        }
        LinearLayout suggestions = findTaggedLayout(chatList, "suggestions");
        if (suggestions == null || sessionStore.token().isEmpty()) {
            return;
        }
        suggestions.removeAllViews();
        JSONArray items = home.optJSONArray("suggestions");
        if (items == null) {
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            String text = items.optJSONObject(i).optString("text", "");
            if (text.isEmpty()) {
                continue;
            }
            Button chip = secondaryButton(text);
            chip.setOnClickListener(v -> sendMessage(((Button) v).getText().toString()));
            suggestions.addView(chip);
        }
    }

    private void showDrawer() {
        if (drawerLayer != null) {
            return;
        }
        drawerLayer = new FrameLayout(this);
        drawerLayer.setBackgroundColor(Color.argb(92, 0, 0, 0));
        drawerLayer.setAlpha(0f);
        drawerLayer.setOnClickListener(v -> closeDrawerAnimated());

        LinearLayout drawer = new LinearLayout(this);
        drawerPanel = drawer;
        drawer.setOrientation(LinearLayout.VERTICAL);
        drawer.setPadding(dp(14), dp(10), dp(14), dp(10));
        drawer.setBackgroundColor(Color.WHITE);
        drawer.setOnClickListener(v -> {
        });

        LinearLayout top = new LinearLayout(this);
        top.setGravity(Gravity.CENTER_VERTICAL);
        TextView search = new TextView(this);
        search.setText("⌕  搜索");
        search.setTextSize(15);
        search.setTextColor(Color.rgb(156, 163, 175));
        search.setGravity(Gravity.CENTER_VERTICAL);
        search.setPadding(dp(16), 0, dp(16), 0);
        search.setBackground(rounded(Color.rgb(246, 247, 249), dp(12)));
        top.addView(search, new LinearLayout.LayoutParams(0, dp(44), 1));
        Button create = transparentIconButton("✎");
        create.setTextSize(24);
        create.setOnClickListener(v -> {
            createFreshLocalSession();
            closeDrawerAnimated();
            renderChatHome();
            loadHomeCopy();
        });
        LinearLayout.LayoutParams createParams = new LinearLayout.LayoutParams(dp(44), dp(44));
        createParams.leftMargin = dp(8);
        top.addView(create, createParams);
        drawer.addView(top, new LinearLayout.LayoutParams(-1, dp(62)));

        LinearLayout navGroup = new LinearLayout(this);
        navGroup.setOrientation(LinearLayout.VERTICAL);
        navGroup.setPadding(0, dp(12), 0, dp(10));
        navGroup.addView(drawerNavButton("✦", "AI导购", "chat", v -> renderChatHome()));
        navGroup.addView(drawerNavButton("▣", "商品", "products", v -> renderProducts()));
        navGroup.addView(drawerNavButton("🛒", "购物车", "cart", v -> renderCart()));
        navGroup.addView(drawerNavButton("≡", "订单", "orders", v -> renderOrders()));
        drawer.addView(navGroup);

        View divider = new View(this);
        divider.setBackgroundColor(Color.rgb(238, 239, 242));
        drawer.addView(divider, new LinearLayout.LayoutParams(-1, 1));

        FrameLayout historyFrame = new FrameLayout(this);
        ScrollView historyScroll = new ScrollView(this);
        LinearLayout historyList = new LinearLayout(this);
        historyList.setOrientation(LinearLayout.VERTICAL);
        historyList.setPadding(0, dp(12), 0, dp(72));
        List<LocalChatStore.SessionSummary> histories = chatStore.recentSessionsWithMessages();
        if (histories.isEmpty()) {
            TextView empty = muted("暂无历史聊天");
            empty.setGravity(Gravity.CENTER);
            historyList.addView(empty, new LinearLayout.LayoutParams(-1, dp(56)));
        }
        for (LocalChatStore.SessionSummary item : histories) {
            historyList.addView(historyButton(item));
        }
        historyScroll.addView(historyList);
        historyFrame.addView(historyScroll, new FrameLayout.LayoutParams(-1, -1));

        Button returnChat = primaryButton("返回聊天");
        returnChat.setTextSize(15);
        returnChat.setPadding(dp(24), 0, dp(24), 0);
        returnChat.setMinWidth(0);
        returnChat.setMinHeight(0);
        returnChat.setOnClickListener(v -> {
            closeDrawerAnimated();
            if (!"chat".equals(activePage)) {
                renderChatHome();
                loadHomeCopy();
            }
        });
        FrameLayout.LayoutParams returnParams = new FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, dp(44), Gravity.BOTTOM | Gravity.CENTER_HORIZONTAL);
        returnParams.bottomMargin = dp(12);
        historyFrame.addView(returnChat, returnParams);
        drawer.addView(historyFrame, new LinearLayout.LayoutParams(-1, 0, 1));

        drawer.addView(bottomUserBar(), new LinearLayout.LayoutParams(-1, dp(68)));

        int panelWidth = (int) (getResources().getDisplayMetrics().widthPixels * 0.82f);
        FrameLayout.LayoutParams drawerParams = new FrameLayout.LayoutParams(panelWidth, -1, Gravity.LEFT | Gravity.TOP);
        drawerLayer.addView(drawer, drawerParams);
        root.addView(drawerLayer, new FrameLayout.LayoutParams(-1, -1));
        drawerLayer.animate().alpha(1f).setDuration(160).start();
        TranslateAnimation slideIn = new TranslateAnimation(-panelWidth, 0, 0, 0);
        slideIn.setDuration(220);
        drawer.startAnimation(slideIn);
    }

    private void sendMessage(String text) {
        clearWelcomeIfNeeded();
        input.setText("");
        addBubble(text, true);
        chatStore.saveMessage(localSessionId, "user", text, "pending");
        if (sessionStore.token().isEmpty()) {
            TextView assistant = addBubble("请先在右上角“未登录”入口完成登录。当前消息已经保存在本地，登录后可以继续使用 AI 导购。", false);
            chatStore.saveMessage(localSessionId, "assistant", assistant.getText().toString(), "local_only");
            toastLine("请先登录后再发送到后端");
            return;
        }
        streaming = true;
        actionButton.setText("■");
        loadingAssistant = addLoadingBubble();
        ensureServerSessionThenStream(text);
    }

    private void ensureServerSessionThenStream(String text) {
        if (!serverSessionId.isEmpty()) {
            stream(text);
            return;
        }
        new Thread(() -> {
            try {
                JSONObject session = api.createSession("Android 新聊天");
                serverSessionId = session.optString("session_id", "");
                chatStore.bindServerSession(localSessionId, serverSessionId);
                runOnUiThread(() -> stream(text));
            } catch (Exception error) {
                runOnUiThread(() -> failStream("创建会话失败：" + error.getMessage()));
            }
        }).start();
    }

    private void stream(String text) {
        activeAssistantMarkdown = new StringBuilder();
        api.streamMessage(serverSessionId, text, new ApiClient.SseCallback() {
            @Override
            public void onEvent(JSONObject event) {
                runOnUiThread(() -> handleSse(event));
            }

            @Override
            public void onError(Throwable error) {
                runOnUiThread(() -> failStream("发送失败：" + error.getMessage()));
            }
        });
    }

    private void handleSse(JSONObject event) {
        String type = event.optString("type");
        if ("text_delta".equals(type)) {
            appendAssistant(event.optString("delta"));
            return;
        }
        if ("block_delta".equals(type)) {
            JSONObject block = event.optJSONObject("block");
            renderAgentBlock(block == null ? event : block);
            return;
        }
        if ("followups".equals(type)) {
            JSONArray questions = event.optJSONArray("questions");
            addChatSystemLine("你还可以继续追问：" + (questions == null ? "" : questions.toString()));
            return;
        }
        if ("message_end".equals(type)) {
            finishStream();
            return;
        }
        if ("error".equals(type)) {
            failStream(event.optString("message", "生成失败"));
        }
    }

    private void appendAssistant(String delta) {
        if (activeAssistant == null) {
            removeLoadingBubbleIfNeeded();
            activeAssistant = addBubble("", false);
            activeAssistantMarkdown = new StringBuilder();
        }
        if (activeAssistantMarkdown == null) {
            activeAssistantMarkdown = new StringBuilder(activeAssistant.getText().toString());
        }
        activeAssistantMarkdown.append(delta);
        MarkdownRenderer.setMarkdown(activeAssistant, activeAssistantMarkdown.toString());
        scrollBottom();
    }

    private void finishStream() {
        if (activeAssistant != null) {
            String markdown = activeAssistantMarkdown == null ? activeAssistant.getText().toString() : activeAssistantMarkdown.toString();
            MarkdownRenderer.setMarkdown(activeAssistant, markdown);
            chatStore.saveMessage(localSessionId, "assistant", markdown, "completed");
        }
        activeAssistantMarkdown = null;
        activeAssistant = null;
        removeLoadingBubbleIfNeeded();
        streaming = false;
        if (actionButton != null && input != null) {
            actionButton.setText(input.getText().toString().trim().isEmpty() ? "≋" : "➤");
        }
    }

    private void failStream(String message) {
        if (loadingAssistant != null) {
            loadingAssistant.setText(message);
            loadingAssistant = null;
        } else {
            addChatSystemLine(message);
        }
        streaming = false;
        if (actionButton != null && input != null) {
            actionButton.setText(input.getText().toString().trim().isEmpty() ? "≋" : "➤");
        }
    }

    private TextView addLoadingBubble() {
        TextView bubble = addBubble("正在思考...", false);
        bubble.setTag("loading_assistant");
        return bubble;
    }

    private void removeLoadingBubbleIfNeeded() {
        if (loadingAssistant == null || chatList == null) {
            loadingAssistant = null;
            return;
        }
        chatList.removeView(loadingAssistant);
        loadingAssistant = null;
    }

    private void renderAgentBlock(JSONObject block) {
        removeLoadingBubbleIfNeeded();
        if (block == null) {
            return;
        }
        String type = block.optString("type");
        if ("product_card".equals(type)) {
            JSONObject product = block.optJSONObject("product");
            if (product != null) {
                chatList.addView(chatProductCard(product));
                scrollBottom();
            }
            return;
        }
        if ("comparison_table".equals(type)) {
            chatList.addView(comparisonCard(block));
            scrollBottom();
            return;
        }
        if ("warning".equals(type) || "citation".equals(type)) {
            String message = block.optString("message", block.optString("content", ""));
            if (!message.isEmpty()) {
                addChatSystemLine(message);
            }
            return;
        }
        if ("cart_state".equals(type)) {
            addChatSystemLine("购物车已更新，可以到购物车页面查看。");
            return;
        }
        if ("order_summary".equals(type)) {
            addChatSystemLine("订单信息已更新，可以到订单页面查看。");
            return;
        }
        String content = block.optString("content", "");
        if (!content.isEmpty()) {
            addBubble(content, false);
        }
    }

    private View chatProductCard(JSONObject item) {
        LinearLayout card = panel();
        card.setPadding(dp(12), dp(10), dp(12), dp(10));
        card.setBackground(rounded(Color.WHITE, dp(16)));
        LinearLayout.LayoutParams cardParams = new LinearLayout.LayoutParams((int) (getResources().getDisplayMetrics().widthPixels * 0.82f), -2);
        cardParams.gravity = Gravity.LEFT;
        cardParams.setMargins(0, dp(6), 0, dp(8));
        card.setLayoutParams(cardParams);
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(productImage(item.optString("imageUrl", ""), 72), new LinearLayout.LayoutParams(dp(72), dp(72)));
        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        TextView name = strong(item.optString("name", "商品"));
        name.setTextSize(15);
        name.setMaxLines(2);
        info.addView(name);
        info.addView(muted(item.optString("brand", "") + " · " + item.optString("merchantName", "商家")));
        TextView price = strong("¥" + item.optString("price", "0"));
        price.setTextSize(16);
        info.addView(price);
        String reason = item.optString("recommendReason", "");
        if (!reason.isEmpty()) {
            TextView reasonView = muted(reason);
            reasonView.setMaxLines(2);
            info.addView(reasonView);
        }
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = dp(10);
        row.addView(info, infoParams);
        card.addView(row);
        LinearLayout actions = new LinearLayout(this);
        Button detail = secondaryButton("详情");
        detail.setOnClickListener(v -> renderProductDetail(item.optString("productId"), () -> renderChatHome()));
        LinearLayout.LayoutParams detailParams = new LinearLayout.LayoutParams(0, dp(42), 1);
        detailParams.rightMargin = dp(8);
        actions.addView(detail, detailParams);
        Button add = primaryButton("加入购物车");
        add.setOnClickListener(v -> addProductToCart(item));
        actions.addView(add, new LinearLayout.LayoutParams(0, dp(42), 1));
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(-1, dp(42));
        actionParams.topMargin = dp(8);
        card.addView(actions, actionParams);
        return card;
    }

    private View comparisonCard(JSONObject block) {
        LinearLayout card = panel();
        card.addView(strong("商品对比"));
        JSONArray columns = block.optJSONArray("columns");
        JSONArray rows = block.optJSONArray("rows");
        if (rows != null) {
            for (int i = 0; i < rows.length(); i++) {
                JSONObject row = rows.optJSONObject(i);
                JSONArray values = row == null ? null : row.optJSONArray("values");
                if (values == null) {
                    continue;
                }
                card.addView(muted(joinArray(values, values.length())));
            }
        }
        if (columns != null && rows == null) {
            card.addView(muted(joinArray(columns, columns.length())));
        }
        return card;
    }

    private void renderProfile() {
        closeDrawer();
        activePage = "profile";
        backStack.clear();
        baseScreen();
        addPageHeader("我的", "账号与资料管理");
        LinearLayout page = pageBody();
        if (sessionStore.token().isEmpty()) {
            LinearLayout loginPanel = panel();
            loginPanel.addView(strong("账号登录"));
            EditText username = inputField("账号", "user");
            EditText password = inputField("密码", "user123456");
            password.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
            loginPanel.addView(username);
            loginPanel.addView(password);
            Button login = primaryButton("登录");
            login.setOnClickListener(v -> login(username.getText().toString(), password.getText().toString()));
            addFormButton(loginPanel, login);
            page.addView(loginPanel);

            LinearLayout registerPanel = panel();
            registerPanel.addView(strong("注册账号"));
            Button registerUser = secondaryButton("注册顾客账号");
            registerUser.setOnClickListener(v -> register(username.getText().toString(), password.getText().toString(), "user"));
            addFormButton(registerPanel, registerUser);
            Button registerMerchant = secondaryButton("注册商家账号");
            registerMerchant.setOnClickListener(v -> register(username.getText().toString(), password.getText().toString(), "merchant"));
            addFormButton(registerPanel, registerMerchant);
            page.addView(registerPanel);

            LinearLayout notePanel = panel();
            notePanel.addView(strong("说明"));
            notePanel.addView(muted("当前验证码为后端 mock，默认使用 123456；未来接入短信/邮件云服务。"));
            page.addView(notePanel);
        } else {
            LinearLayout profilePanel = panel();
            LinearLayout profileRow = new LinearLayout(this);
            profileRow.setGravity(Gravity.CENTER_VERTICAL);
            View avatar = accountAvatarView(dp(56), 18);
            profileRow.addView(avatar, new LinearLayout.LayoutParams(dp(56), dp(56)));
            LinearLayout info = new LinearLayout(this);
            info.setOrientation(LinearLayout.VERTICAL);
            info.addView(strong(sessionStore.nickname().isEmpty() ? "用户名" : sessionStore.nickname()));
            info.addView(muted("角色：" + sessionStore.role()));
            LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
            infoParams.leftMargin = dp(12);
            profileRow.addView(info, infoParams);
            profilePanel.addView(profileRow);
            page.addView(profilePanel);

            LinearLayout editPanel = panel();
            editPanel.addView(strong("编辑资料"));
            EditText nickname = inputField("昵称", sessionStore.nickname());
            EditText avatarUrl = inputField("头像 URL", sessionStore.avatarUrl());
            editPanel.addView(nickname);
            editPanel.addView(avatarUrl);
            Button save = primaryButton("保存资料");
            save.setOnClickListener(v -> updateProfile(nickname.getText().toString(), avatarUrl.getText().toString()));
            addFormButton(editPanel, save);
            page.addView(editPanel);

            LinearLayout securityPanel = panel();
            securityPanel.addView(strong("账号安全"));
            EditText oldPassword = inputField("原密码", "");
            EditText newPassword = inputField("新密码", "");
            EditText confirmPassword = inputField("确认新密码", "");
            oldPassword.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
            newPassword.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
            confirmPassword.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
            securityPanel.addView(oldPassword);
            securityPanel.addView(newPassword);
            securityPanel.addView(confirmPassword);
            Button changePassword = secondaryButton("修改密码");
            changePassword.setOnClickListener(v -> {
                String next = newPassword.getText().toString();
                if (oldPassword.getText().toString().trim().isEmpty()) {
                    toastLine("请输入原密码");
                    return;
                }
                if (next.length() < 6) {
                    toastLine("新密码至少 6 位");
                    return;
                }
                if (!next.equals(confirmPassword.getText().toString())) {
                    toastLine("两次输入的新密码不一致");
                    return;
                }
                changePassword(oldPassword.getText().toString(), next);
            });
            addFormButton(securityPanel, changePassword);
            page.addView(securityPanel);

            Button logout = secondaryButton("退出登录");
            logout.setOnClickListener(v -> {
                sessionStore.clearAuth();
                renderChatHome();
            });
            addFormButton(page, logout);
        }
    }

    private void register(String username, String password, String role) {
        new Thread(() -> {
            try {
                String displayName = role.equals("merchant") ? "新商家" : username;
                JSONObject result = api.register(username, password, displayName, role, displayName, "123456");
                JSONObject account = result.optJSONObject("account");
                sessionStore.saveAuth(result.optString("token"), account == null ? role : account.optString("role", role), account == null ? displayName : account.optString("display_name", displayName), account == null ? "" : account.optString("avatar_url", ""));
                runOnUiThread(() -> {
                    toastLine("注册成功");
                    renderChatHome();
                    loadHomeCopy();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("注册失败：" + error.getMessage()));
            }
        }).start();
    }

    private void login(String username, String password) {
        new Thread(() -> {
            try {
                JSONObject result = api.login(username, password);
                JSONObject account = result.optJSONObject("account");
                sessionStore.saveAuth(result.optString("token"), account == null ? "user" : account.optString("role", "user"), account == null ? username : account.optString("display_name", username), account == null ? "" : account.optString("avatar_url", ""));
                runOnUiThread(() -> {
                    toastLine("登录成功");
                    renderChatHome();
                    loadHomeCopy();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("登录失败：" + error.getMessage()));
            }
        }).start();
    }

    private void changePassword(String oldPassword, String newPassword) {
        new Thread(() -> {
            try {
                api.changePassword(oldPassword, newPassword);
                runOnUiThread(() -> toastLine("密码已修改"));
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("修改失败：" + error.getMessage()));
            }
        }).start();
    }

    private void updateProfile(String nickname, String avatarUrl) {
        String value = nickname == null ? "" : nickname.trim();
        String image = avatarUrl == null ? "" : avatarUrl.trim();
        if (value.isEmpty()) {
            toastLine("昵称不能为空");
            return;
        }
        if (value.length() > 24) {
            toastLine("昵称最多 24 个字符");
            return;
        }
        if (!image.isEmpty() && !image.startsWith("http://") && !image.startsWith("https://")) {
            toastLine("头像 URL 必须以 http:// 或 https:// 开头");
            return;
        }
        new Thread(() -> {
            try {
                JSONObject profile = api.updateProfile(value, image);
                sessionStore.saveAuth(sessionStore.token(), sessionStore.role(), profile.optString("nickname", value), profile.optString("avatar_url", image));
                runOnUiThread(() -> {
                    toastLine("资料已保存");
                    renderProfile();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("保存失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderProducts() {
        closeDrawer();
        activePage = "products";
        backStack.clear();
        baseScreen();
        addPageHeader("商品", "搜索商品、查看详情、加入购物车");
        LinearLayout page = pageBody();

        LinearLayout searchRow = new LinearLayout(this);
        searchRow.setGravity(Gravity.CENTER_VERTICAL);
        EditText keyword = inputField("搜索商品", "");
        searchRow.addView(keyword, new LinearLayout.LayoutParams(0, dp(52), 1));
        Button search = secondaryButton("搜索");
        LinearLayout.LayoutParams searchParams = new LinearLayout.LayoutParams(dp(88), dp(52));
        searchParams.leftMargin = dp(10);
        searchRow.addView(search, searchParams);
        page.addView(searchRow);

        HorizontalScrollView filterScroll = new HorizontalScrollView(this);
        filterScroll.setHorizontalScrollBarEnabled(false);
        LinearLayout filters = new LinearLayout(this);
        filters.setOrientation(LinearLayout.HORIZONTAL);
        filterScroll.addView(filters);
        LinearLayout.LayoutParams filterParams = new LinearLayout.LayoutParams(-1, dp(46));
        filterParams.setMargins(0, 0, 0, dp(10));
        page.addView(filterScroll, filterParams);

        LinearLayout list = new LinearLayout(this);
        list.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams listParams = new LinearLayout.LayoutParams(-1, -2);
        listParams.topMargin = dp(2);
        page.addView(list, listParams);
        search.setOnClickListener(v -> {
            lastProductKeyword = keyword.getText().toString();
            loadProducts(list, lastProductKeyword, lastCategoryId);
        });
        loadCategories(filters, list, keyword);
        loadProducts(list, lastProductKeyword, lastCategoryId);
    }

    private void loadCategories(LinearLayout filters, LinearLayout list, EditText keyword) {
        filters.removeAllViews();
        addCategoryChip(filters, "全部", "", list, keyword);
        new Thread(() -> {
            try {
                JSONArray categories = api.categoriesTree();
                runOnUiThread(() -> renderCategoryChips(filters, categories, list, keyword));
            } catch (Exception ignored) {
            }
        }).start();
    }

    private void renderCategoryChips(LinearLayout filters, JSONArray categories, LinearLayout list, EditText keyword) {
        while (filters.getChildCount() > 1) {
            filters.removeViewAt(1);
        }
        for (int i = 0; i < categories.length(); i++) {
            JSONObject item = categories.optJSONObject(i);
            if (item == null) {
                continue;
            }
            JSONArray children = item.optJSONArray("children");
            if (children != null && children.length() > 0) {
                for (int j = 0; j < children.length(); j++) {
                    JSONObject child = children.optJSONObject(j);
                    if (child != null) {
                        addCategoryChip(filters, child.optString("name", "分类"), child.optString("categoryId", ""), list, keyword);
                    }
                }
            } else {
                addCategoryChip(filters, item.optString("name", "分类"), item.optString("categoryId", ""), list, keyword);
            }
        }
    }

    private void addCategoryChip(LinearLayout filters, String name, String categoryId, LinearLayout list, EditText keyword) {
        Button chip = categoryId.equals(lastCategoryId) ? primaryButton(name) : secondaryButton(name);
        chip.setTextSize(14);
        chip.setOnClickListener(v -> {
            lastCategoryId = categoryId;
            lastProductKeyword = keyword.getText().toString();
            renderProducts();
        });
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-2, dp(38));
        params.setMargins(0, 0, dp(8), dp(8));
        filters.addView(chip, params);
    }

    private void loadProducts(LinearLayout list, String keyword, String categoryId) {
        list.removeAllViews();
        list.addView(muted("正在加载商品..."));
        new Thread(() -> {
            try {
                JSONArray items = api.products(keyword, categoryId);
                runOnUiThread(() -> renderProductItems(list, items));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(list, "商品加载失败", error.getMessage(), () -> loadProducts(list, keyword, categoryId)));
            }
        }).start();
    }

    private void renderProductItems(LinearLayout list, JSONArray items) {
        list.removeAllViews();
        if (items.length() == 0) {
            list.addView(card("暂无商品", "换个关键词试试。"));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                list.addView(productCard(item));
            }
        }
    }

    private View productCard(JSONObject item) {
        LinearLayout card = panel();
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(productImage(item.optString("imageUrl", ""), 88), new LinearLayout.LayoutParams(dp(88), dp(88)));

        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        TextView name = strong(item.optString("name", "未命名商品"));
        info.addView(name);
        info.addView(muted(item.optString("brand", "") + " · " + item.optString("merchantName", "商家")));
        TextView price = new TextView(this);
        price.setText("¥" + item.optString("price", "0"));
        price.setTextSize(16);
        price.setTypeface(Typeface.DEFAULT_BOLD);
        price.setTextColor(Color.rgb(17, 24, 39));
        info.addView(price);
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = dp(12);
        row.addView(info, infoParams);
        card.addView(row);

        JSONArray points = item.optJSONArray("sellingPoints");
        if (points != null && points.length() > 0) {
            card.addView(muted("卖点：" + joinArray(points, 3)));
        }
        String reason = item.optString("recommendReason", "");
        if (!reason.isEmpty()) {
            card.addView(muted("推荐理由：" + reason));
        }
        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        Button detail = secondaryButton("详情");
        detail.setOnClickListener(v -> renderProductDetail(item.optString("productId"), () -> renderProducts()));
        LinearLayout.LayoutParams detailParams = new LinearLayout.LayoutParams(0, dp(44), 1);
        detailParams.rightMargin = dp(8);
        actions.addView(detail, detailParams);
        Button add = primaryButton("加入购物车");
        add.setOnClickListener(v -> addProductToCart(item));
        actions.addView(add, new LinearLayout.LayoutParams(0, dp(44), 1));
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(-1, dp(44));
        actionParams.topMargin = dp(8);
        card.addView(actions, actionParams);
        return card;
    }

    private void renderProductDetail(String productId, Runnable backAction) {
        closeDrawer();
        activePage = "product_detail";
        backStack.clear();
        if (backAction != null) {
            backStack.push(backAction);
        }
        baseScreen();
        addPageHeader("商品详情", "");
        LinearLayout page = pageBody();
        page.addView(muted("正在加载商品详情..."));
        new Thread(() -> {
            try {
                JSONObject detail = api.productDetail(productId);
                JSONArray skus = api.productSkus(productId);
                runOnUiThread(() -> renderProductDetailContent(page, detail, skus));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "商品详情加载失败", error.getMessage(), () -> renderProductDetail(productId, backAction)));
            }
        }).start();
    }

    private void renderProductDetailContent(LinearLayout page, JSONObject item, JSONArray skus) {
        page.removeAllViews();
        JSONArray images = item.optJSONArray("imageUrls");
        String imageUrl = images != null && images.length() > 0 ? images.optString(0) : item.optString("imageUrl", "");
        page.addView(productImage(imageUrl, 220), new LinearLayout.LayoutParams(-1, dp(220)));

        LinearLayout main = panel();
        main.addView(strong(item.optString("name", "未命名商品")));
        main.addView(muted(item.optString("brand", "") + " · " + item.optString("merchantName", "商家")));
        TextView price = strong("¥" + item.optString("price", "0"));
        price.setTextSize(22);
        main.addView(price);
        String market = item.optString("marketPrice", "");
        if (!market.isEmpty()) {
            main.addView(muted("市场价：¥" + market));
        }
        main.addView(muted("库存：" + statusText(item.optString("stockStatus", ""))));
        page.addView(main);

        addArrayPanel(page, "商品卖点", item.optJSONArray("sellingPoints"));
        addTextPanel(page, "推荐理由", item.optString("recommendReason", ""));
        addArrayPanel(page, "适合人群", item.optJSONArray("suitableFor"));
        addArrayPanel(page, "不适合人群", item.optJSONArray("notSuitableFor"));
        addArrayPanel(page, "风险提示", item.optJSONArray("riskNotes"));
        addAttributesPanel(page, item.optJSONArray("attributes"));
        addSkusPanel(page, skus);

        Button add = primaryButton("加入购物车");
        add.setOnClickListener(v -> addProductToCart(item));
        page.addView(add, new LinearLayout.LayoutParams(-1, dp(52)));
    }

    private void addProductToCart(JSONObject item) {
        if (sessionStore.token().isEmpty()) {
            toastLine("请先登录后再加入购物车");
            renderProfile();
            return;
        }
        new Thread(() -> {
            try {
                String skuId = item.optString("skuId", "");
                if (skuId.isEmpty()) {
                    JSONArray skus = api.productSkus(item.optString("productId"));
                    for (int i = 0; i < skus.length(); i++) {
                        JSONObject sku = skus.optJSONObject(i);
                        if (sku != null && sku.optInt("stockQuantity", 0) > 0) {
                            skuId = sku.optString("skuId", "");
                            break;
                        }
                    }
                }
                api.addCartItem(item.optString("productId"), skuId, 1);
                runOnUiThread(() -> toastLine("已加入购物车"));
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("加入失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderCart() {
        closeDrawer();
        activePage = "cart";
        backStack.clear();
        baseScreen();
        addPageHeader("购物车", "调整数量、选择商品并结算");
        LinearLayout page = pageBody();
        if (sessionStore.token().isEmpty()) {
            page.addView(card("请先登录", "登录后可查看购物车并结算。"));
            return;
        }
        loadCart(page);
    }

    private void loadCart(LinearLayout page) {
        page.removeAllViews();
        page.addView(muted("正在加载购物车..."));
        new Thread(() -> {
            try {
                JSONObject cart = api.cart();
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "购物车加载失败", error.getMessage(), () -> loadCart(page)));
            }
        }).start();
    }

    private void renderCartContent(LinearLayout page, JSONObject cart) {
        page.removeAllViews();
        JSONArray items = cart.optJSONArray("items");
        if (items == null || items.length() == 0) {
            page.addView(card("购物车为空", "可以先去商品页添加商品。"));
            Button goProducts = primaryButton("去逛商品");
            goProducts.setOnClickListener(v -> renderProducts());
            page.addView(goProducts, new LinearLayout.LayoutParams(-1, dp(52)));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                page.addView(cartItemView(item, page));
            }
        }
        JSONObject summary = cart.optJSONObject("summary");
        LinearLayout checkoutBar = panel();
        checkoutBar.addView(strong("已选 " + (summary == null ? 0 : summary.optInt("selectedCount")) + " 件"));
        checkoutBar.addView(muted("应付：¥" + (summary == null ? "0" : summary.optString("payAmount", "0"))));
        Button checkout = primaryButton("结算");
        checkout.setOnClickListener(v -> confirmCheckout(page, summary));
        checkoutBar.addView(checkout, new LinearLayout.LayoutParams(-1, dp(52)));
        page.addView(checkoutBar);
    }

    private View cartItemView(JSONObject item, LinearLayout page) {
        LinearLayout card = panel();
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        CheckBox selected = new CheckBox(this);
        selected.setChecked(item.optBoolean("selected"));
        selected.setOnClickListener(v -> updateCartItem(page, item.optString("cartItemId"), null, ((CheckBox) v).isChecked()));
        row.addView(selected, new LinearLayout.LayoutParams(dp(48), dp(48)));
        row.addView(productImage(item.optString("imageUrl", ""), 64), new LinearLayout.LayoutParams(dp(64), dp(64)));

        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        info.addView(strong(item.optString("name", "商品")));
        info.addView(muted(item.optString("merchantName", "商家") + " · ¥" + item.optString("price", "0")));
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = dp(10);
        row.addView(info, infoParams);
        card.addView(row);

        LinearLayout controls = new LinearLayout(this);
        controls.setGravity(Gravity.CENTER_VERTICAL | Gravity.RIGHT);
        Button minus = secondaryButton("-");
        minus.setOnClickListener(v -> updateCartItem(page, item.optString("cartItemId"), Math.max(1, item.optInt("quantity", 1) - 1), null));
        controls.addView(minus, new LinearLayout.LayoutParams(dp(46), dp(42)));
        TextView quantity = muted(String.valueOf(item.optInt("quantity", 1)));
        quantity.setGravity(Gravity.CENTER);
        controls.addView(quantity, new LinearLayout.LayoutParams(dp(52), dp(42)));
        Button plus = secondaryButton("+");
        plus.setOnClickListener(v -> updateCartItem(page, item.optString("cartItemId"), item.optInt("quantity", 1) + 1, null));
        controls.addView(plus, new LinearLayout.LayoutParams(dp(46), dp(42)));
        Button delete = textOnlyButton("删除");
        delete.setTextColor(Color.rgb(185, 28, 28));
        delete.setOnClickListener(v -> deleteCartItem(page, item.optString("cartItemId")));
        controls.addView(delete, new LinearLayout.LayoutParams(dp(70), dp(42)));
        card.addView(controls);
        return card;
    }

    private void updateCartItem(LinearLayout page, String cartItemId, Integer quantity, Boolean selected) {
        new Thread(() -> {
            try {
                JSONObject cart = api.updateCartItem(cartItemId, quantity, selected);
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("更新失败：" + error.getMessage()));
            }
        }).start();
    }

    private void deleteCartItem(LinearLayout page, String cartItemId) {
        new Thread(() -> {
            try {
                JSONObject cart = api.deleteCartItem(cartItemId);
                runOnUiThread(() -> renderCartContent(page, cart));
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("删除失败：" + error.getMessage()));
            }
        }).start();
    }

    private void confirmCheckout(LinearLayout page, JSONObject summary) {
        int selectedCount = summary == null ? 0 : summary.optInt("selectedCount");
        if (selectedCount == 0) {
            toastLine("请先选择要结算的商品");
            return;
        }
        String amount = summary.optString("payAmount", "0");
        new AlertDialog.Builder(this)
                .setTitle("确认下单")
                .setMessage("已选择 " + selectedCount + " 件商品\n应付金额：¥" + amount + "\n订单提交后将在订单页查看处理状态。")
                .setNegativeButton("取消", null)
                .setPositiveButton("确认下单", (dialog, which) -> checkoutCart(page))
                .show();
    }

    private void checkoutCart(LinearLayout page) {
        new Thread(() -> {
            try {
                api.checkout();
                runOnUiThread(() -> {
                    toastLine("订单已提交");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("结算失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderOrders() {
        closeDrawer();
        activePage = "orders";
        backStack.clear();
        baseScreen();
        addPageHeader("订单", "查看顾客订单状态");
        LinearLayout page = pageBody();
        if (sessionStore.token().isEmpty()) {
            page.addView(card("请先登录", "登录后可查看订单。"));
            return;
        }
        page.addView(muted("正在加载订单..."));
        new Thread(() -> {
            try {
                JSONArray items = api.orders();
                runOnUiThread(() -> renderOrderItems(page, items));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "订单加载失败", error.getMessage(), () -> renderOrders()));
            }
        }).start();
    }

    private void renderOrderItems(LinearLayout page, JSONArray items) {
        page.removeAllViews();
        if (items.length() == 0) {
            page.addView(card("暂无订单", "购物车结算后会在这里展示订单。"));
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
        LinearLayout card = panel();
        card.addView(strong("订单 " + item.optString("order_id", "")));
        card.addView(muted(statusText(item.optString("status", "")) + " · " + item.optString("merchant_name", "商家")));
        card.addView(muted("金额：¥" + item.optString("total_amount", "0")));
        JSONArray items = item.optJSONArray("items");
        if (items != null && items.length() > 0) {
            JSONObject first = items.optJSONObject(0);
            String more = items.length() > 1 ? " 等 " + items.length() + " 件商品" : "";
            card.addView(muted("商品：" + (first == null ? "" : first.optString("name", "")) + more));
        }
        card.addView(muted("创建时间：" + item.optString("created_at", "")));
        return card;
    }

    private interface JsonLoader {
        JSONArray load();
    }

    private void renderListPage(String title, String subtitle, JsonLoader loader, String mainKey, String subKey) {
        baseScreen();
        addPageHeader(title, subtitle);
        LinearLayout page = pageBody();
        new Thread(() -> {
            JSONArray items = loader.load();
            runOnUiThread(() -> {
                if (items.length() == 0) {
                    page.addView(card("暂无数据", "请稍后重试或先登录。"));
                    return;
                }
                for (int i = 0; i < items.length(); i++) {
                    JSONObject item = items.optJSONObject(i);
                    page.addView(card(item.optString(mainKey, "未命名"), item.optString(subKey, item.toString())));
                }
            });
        }).start();
    }

    private void addPageHeader(String title, String subtitle) {
        content.addView(createTopBar(title, ""), new LinearLayout.LayoutParams(-1, dp(56)));
    }

    private LinearLayout pageBody() {
        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(dp(16), dp(10), dp(16), dp(16));
        scroll.addView(page);
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        return page;
    }

    private void addFormButton(LinearLayout page, Button button) {
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(48));
        params.setMargins(0, 0, 0, dp(10));
        page.addView(button, params);
    }

    private TextView addBubble(String text, boolean right) {
        TextView bubble = new TextView(this);
        if (right) {
            bubble.setText(text);
        } else {
            MarkdownRenderer.setMarkdown(bubble, text);
        }
        bubble.setTextSize(15);
        bubble.setLineSpacing(4, 1);
        bubble.setPadding(dp(12), dp(9), dp(12), dp(9));
        bubble.setTextColor(right ? Color.WHITE : Color.rgb(24, 30, 37));
        bubble.setBackground(rounded(right ? Color.rgb(20, 20, 20) : Color.WHITE, dp(16)));
        bubble.setMaxWidth((int) (getResources().getDisplayMetrics().widthPixels * 0.74f));
        bubble.setLinksClickable(true);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = right ? Gravity.RIGHT : Gravity.LEFT;
        params.setMargins(0, dp(6), 0, dp(6));
        chatList.addView(bubble, params);
        scrollBottom();
        return bubble;
    }

    private void addChatSystemLine(String text) {
        TextView view = muted(text);
        view.setGravity(Gravity.CENTER);
        chatList.addView(view, new LinearLayout.LayoutParams(-1, -2));
        scrollBottom();
    }

    private void toastLine(String text) {
        if (activeToast != null) {
            activeToast.cancel();
        }
        activeToast = Toast.makeText(this, text, Toast.LENGTH_SHORT);
        activeToast.show();
    }

    private void closeDrawer() {
        if (drawerLayer != null) {
            root.removeView(drawerLayer);
            drawerLayer = null;
            drawerPanel = null;
        }
    }

    private void closeDrawerAnimated() {
        if (drawerLayer == null || drawerPanel == null) {
            return;
        }
        int panelWidth = drawerPanel.getWidth() == 0 ? (int) (getResources().getDisplayMetrics().widthPixels * 0.82f) : drawerPanel.getWidth();
        FrameLayout closingLayer = drawerLayer;
        LinearLayout closingPanel = drawerPanel;
        drawerLayer = null;
        drawerPanel = null;
        closingLayer.animate().alpha(0f).setDuration(180).start();
        TranslateAnimation slideOut = new TranslateAnimation(0, -panelWidth, 0, 0);
        slideOut.setDuration(180);
        slideOut.setAnimationListener(new Animation.AnimationListener() {
            @Override
            public void onAnimationStart(Animation animation) {
            }

            @Override
            public void onAnimationEnd(Animation animation) {
                if (closingLayer.getParent() == root) {
                    root.removeView(closingLayer);
                }
            }

            @Override
            public void onAnimationRepeat(Animation animation) {
            }
        });
        closingPanel.startAnimation(slideOut);
    }

    private void clearWelcomeIfNeeded() {
        if (chatList.getChildCount() > 0 && "welcome".equals(chatList.getChildAt(0).getTag())) {
            chatList.removeAllViews();
            chatList.setTag("messages");
        }
    }

    private void renderStoredMessagesIfAny() {
        List<LocalChatStore.MessageItem> messages = chatStore.messages(localSessionId);
        if (messages.isEmpty()) {
            chatList.setTag(null);
            return;
        }
        chatList.removeAllViews();
        chatList.setGravity(Gravity.NO_GRAVITY);
        chatList.setTag("messages");
        for (LocalChatStore.MessageItem message : messages) {
            if ("system".equals(message.role)) {
                addChatSystemLine(message.content);
            } else {
                addBubble(message.content, "user".equals(message.role));
            }
        }
    }

    private void scrollBottom() {
        if (chatScroll != null) {
            chatScroll.post(() -> chatScroll.fullScroll(View.FOCUS_DOWN));
        }
    }

    private Button iconButton(String text) {
        Button button = new Button(this);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(22);
        button.setTextColor(Color.BLACK);
        button.setBackground(rounded(Color.WHITE, dp(28)));
        return button;
    }

    private Button transparentIconButton(String text) {
        Button button = new Button(this);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(24);
        button.setTextColor(Color.BLACK);
        button.setPadding(0, 0, 0, 0);
        button.setMinWidth(0);
        button.setMinHeight(0);
        button.setBackground(new ColorDrawable(Color.TRANSPARENT));
        return button;
    }

    private Button darkRoundButton(String text) {
        Button button = new Button(this);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(20);
        button.setTextColor(Color.WHITE);
        button.setPadding(0, 0, 0, 0);
        button.setMinWidth(0);
        button.setMinHeight(0);
        button.setBackground(rounded(Color.BLACK, dp(24)));
        return button;
    }

    private Button textOnlyButton(String text) {
        Button button = new Button(this);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.BLACK);
        button.setTextSize(15);
        button.setPadding(0, 0, 0, 0);
        button.setMinWidth(0);
        button.setMinHeight(0);
        button.setBackground(new ColorDrawable(Color.TRANSPARENT));
        return button;
    }

    private Button primaryButton(String text) {
        Button button = new Button(this);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.WHITE);
        button.setTextSize(16);
        button.setBackground(rounded(Color.BLACK, dp(24)));
        return button;
    }

    private Button secondaryButton(String text) {
        Button button = new Button(this);
        flattenButton(button);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.BLACK);
        button.setTextSize(15);
        button.setBackground(rounded(Color.rgb(238, 239, 241), dp(22)));
        return button;
    }

    private Button navButton(String text, View.OnClickListener listener) {
        Button button = secondaryButton(text);
        button.setGravity(Gravity.LEFT | Gravity.CENTER_VERTICAL);
        button.setOnClickListener(listener);
        button.setPadding(dp(18), 0, dp(18), 0);
        button.setLayoutParams(new LinearLayout.LayoutParams(-1, dp(50)));
        return button;
    }

    private Button drawerNavButton(String icon, String text, String page, View.OnClickListener listener) {
        Button button = textOnlyButton(icon + "   " + text);
        button.setGravity(Gravity.LEFT | Gravity.CENTER_VERTICAL);
        button.setTextSize(16);
        button.setPadding(dp(18), 0, dp(14), 0);
        button.setBackground(rounded(page.equals(activePage) ? Color.rgb(245, 246, 248) : Color.WHITE, dp(12)));
        button.setOnClickListener(listener);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(48));
        params.setMargins(0, dp(2), 0, dp(2));
        button.setLayoutParams(params);
        return button;
    }

    private void flattenButton(Button button) {
        button.setStateListAnimator(null);
        button.setElevation(0);
    }

    private LinearLayout panel() {
        LinearLayout view = new LinearLayout(this);
        view.setOrientation(LinearLayout.VERTICAL);
        view.setPadding(dp(14), dp(12), dp(14), dp(12));
        view.setBackground(rounded(Color.WHITE, dp(12)));
        view.setElevation(dp(1));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
        params.setMargins(0, 0, 0, dp(12));
        view.setLayoutParams(params);
        return view;
    }

    private TextView imagePlaceholder() {
        TextView image = new TextView(this);
        image.setText("图");
        image.setTextSize(14);
        image.setGravity(Gravity.CENTER);
        image.setTextColor(Color.rgb(107, 114, 128));
        image.setBackground(rounded(Color.rgb(243, 244, 246), dp(12)));
        return image;
    }

    private View productImage(String imageUrl, int sizeDp) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(rounded(Color.rgb(243, 244, 246), dp(12)));
        TextView placeholder = imagePlaceholder();
        frame.addView(placeholder, new FrameLayout.LayoutParams(-1, -1));
        ImageView image = new ImageView(this);
        image.setScaleType(ImageView.ScaleType.CENTER_CROP);
        String url = api.absoluteUrl(imageUrl);
        if (!url.isEmpty()) {
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
            imageLoader.load(image, url);
        }
        frame.setLayoutParams(new LinearLayout.LayoutParams(dp(sizeDp), dp(sizeDp)));
        return frame;
    }

    private View accountAvatarView(int sizePx, int textSp) {
        String avatarUrl = api.absoluteUrl(sessionStore.avatarUrl());
        if (!avatarUrl.isEmpty()) {
            ImageView image = new ImageView(this);
            image.setScaleType(ImageView.ScaleType.CENTER_CROP);
            image.setBackground(rounded(Color.rgb(236, 244, 255), sizePx / 2));
            imageLoader.load(image, avatarUrl);
            return image;
        }
        TextView avatar = new TextView(this);
        avatar.setText(initialOf(sessionStore.nickname()));
        avatar.setTextSize(textSp);
        avatar.setTypeface(Typeface.DEFAULT_BOLD);
        avatar.setGravity(Gravity.CENTER);
        avatar.setTextColor(Color.rgb(30, 64, 175));
        avatar.setBackground(rounded(Color.rgb(219, 234, 254), sizePx / 2));
        return avatar;
    }

    private void addTextPanel(LinearLayout page, String title, String text) {
        if (text == null || text.trim().isEmpty()) {
            return;
        }
        LinearLayout panel = panel();
        panel.addView(strong(title));
        panel.addView(muted(text));
        page.addView(panel);
    }

    private void addArrayPanel(LinearLayout page, String title, JSONArray items) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = panel();
        panel.addView(strong(title));
        for (int i = 0; i < items.length(); i++) {
            panel.addView(muted("• " + items.optString(i)));
        }
        page.addView(panel);
    }

    private void addAttributesPanel(LinearLayout page, JSONArray items) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = panel();
        panel.addView(strong("商品参数"));
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                panel.addView(muted(item.optString("key", "") + "：" + item.optString("value", "") + item.optString("unit", "")));
            }
        }
        page.addView(panel);
    }

    private void addSkusPanel(LinearLayout page, JSONArray items) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = panel();
        panel.addView(strong("规格"));
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                panel.addView(muted(item.optString("skuName", "默认款") + " · ¥" + item.optString("price", "0") + " · 库存 " + item.optInt("stockQuantity", 0)));
            }
        }
        page.addView(panel);
    }

    private TextView strong(String text) {
        TextView view = new TextView(this);
        view.setText(text);
        view.setTextSize(16);
        view.setTypeface(Typeface.DEFAULT_BOLD);
        view.setTextColor(Color.rgb(17, 24, 39));
        view.setPadding(0, dp(4), 0, dp(4));
        return view;
    }

    private void renderError(LinearLayout parent, String title, String detail, Runnable retry) {
        parent.removeAllViews();
        parent.addView(card(title, detail == null ? "" : detail));
        Button button = secondaryButton("重试");
        button.setOnClickListener(v -> retry.run());
        parent.addView(button, new LinearLayout.LayoutParams(-1, dp(52)));
    }

    private String joinArray(JSONArray array, int limit) {
        StringBuilder builder = new StringBuilder();
        int count = Math.min(array.length(), limit);
        for (int i = 0; i < count; i++) {
            if (i > 0) {
                builder.append(" / ");
            }
            builder.append(array.optString(i));
        }
        return builder.toString();
    }

    private String statusText(String status) {
        if ("in_stock".equals(status)) return "有货";
        if ("out_of_stock".equals(status)) return "缺货";
        if ("low_stock".equals(status)) return "库存紧张";
        if ("pending_pay".equals(status)) return "待支付";
        if ("paid".equals(status)) return "已支付";
        if ("pending_ship".equals(status)) return "待发货";
        if ("shipped".equals(status)) return "已发货";
        if ("completed".equals(status)) return "已完成";
        if ("cancelled".equals(status)) return "已取消";
        return status == null || status.isEmpty() ? "未知状态" : status;
    }

    private String initialOf(String name) {
        String value = name == null || name.trim().isEmpty() ? "我" : name.trim();
        return value.substring(0, 1);
    }

    private String shortName(String name) {
        String value = name == null || name.trim().isEmpty() ? "用户名" : name.trim();
        return value.length() > 6 ? value.substring(0, 6) + "…" : value;
    }

    private View historyButton(LocalChatStore.SessionSummary item) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(8), dp(7), dp(8), dp(7));
        row.setBackground(rounded(localSessionId.equals(item.localSessionId) ? Color.rgb(245, 246, 248) : Color.WHITE, dp(12)));
        TextView avatar = new TextView(this);
        avatar.setText("☁");
        avatar.setTextSize(18);
        avatar.setGravity(Gravity.CENTER);
        avatar.setTextColor(Color.rgb(88, 88, 88));
        int[] colors = {Color.rgb(237, 229, 255), Color.rgb(224, 239, 255), Color.rgb(255, 229, 238), Color.rgb(230, 251, 214)};
        avatar.setBackground(rounded(colors[Math.abs(item.localSessionId.hashCode()) % colors.length], dp(18)));
        row.addView(avatar, new LinearLayout.LayoutParams(dp(36), dp(36)));
        TextView title = new TextView(this);
        title.setSingleLine(false);
        title.setMaxLines(2);
        String label = item.title == null || item.title.isEmpty() ? "导购会话" : item.title;
        title.setText(label);
        title.setTextSize(15);
        title.setTextColor(Color.BLACK);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(0, -2, 1);
        titleParams.leftMargin = dp(14);
        row.addView(title, titleParams);
        row.setOnClickListener(v -> {
            localSessionId = item.localSessionId;
            serverSessionId = item.serverSessionId == null ? "" : item.serverSessionId;
            closeDrawerAnimated();
            renderChatHome();
            loadHomeCopy();
        });
        return row;
    }

    private View bottomUserBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(6), dp(6), dp(2), dp(6));
        bar.setBackgroundColor(Color.WHITE);

        LinearLayout user = new LinearLayout(this);
        user.setGravity(Gravity.CENTER_VERTICAL);
        user.setOnClickListener(v -> renderProfile());
        user.addView(accountAvatarView(dp(42), 17), new LinearLayout.LayoutParams(dp(42), dp(42)));
        TextView name = new TextView(this);
        name.setText(sessionStore.token().isEmpty() ? "未登录" : (sessionStore.nickname().isEmpty() ? "用户名" : sessionStore.nickname()));
        name.setTextSize(16);
        name.setTextColor(Color.BLACK);
        LinearLayout.LayoutParams nameParams = new LinearLayout.LayoutParams(-2, -2);
        nameParams.leftMargin = dp(10);
        user.addView(name, nameParams);
        bar.addView(user, new LinearLayout.LayoutParams(0, -2, 1));

        Button settings = transparentIconButton("⚙");
        settings.setTextSize(24);
        settings.setOnClickListener(v -> renderSettings());
        bar.addView(settings, new LinearLayout.LayoutParams(dp(44), dp(44)));
        return bar;
    }

    private void renderSettings() {
        closeDrawer();
        activePage = "settings";
        backStack.clear();
        backStack.push(() -> renderChatHome());
        baseScreen();
        addPageHeader("设置", "调试地址和应用偏好");
        LinearLayout page = pageBody();
        if (BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            EditText apiBase = inputField("后端地址", sessionStore.apiBase());
            page.addView(apiBase);
            Button saveApi = primaryButton("保存测试地址");
            saveApi.setOnClickListener(v -> {
                sessionStore.saveApiBase(apiBase.getText().toString().trim());
                api = new ApiClient(sessionStore);
                toastLine("已保存测试后端地址");
            });
            page.addView(saveApi, new LinearLayout.LayoutParams(-1, dp(52)));
        } else {
            page.addView(card("当前版本", "正式版不展示测试后端地址设置。"));
        }
    }

    private TextView title(String text) {
        TextView view = new TextView(this);
        view.setText(text);
        view.setTextSize(22);
        view.setTypeface(Typeface.DEFAULT_BOLD);
        view.setTextColor(Color.rgb(20, 24, 30));
        return view;
    }

    private TextView section(String text) {
        TextView view = muted(text);
        view.setTypeface(Typeface.DEFAULT_BOLD);
        view.setPadding(0, dp(24), 0, dp(8));
        return view;
    }

    private TextView muted(String text) {
        TextView view = new TextView(this);
        view.setText(text);
        view.setTextSize(13);
        view.setTextColor(Color.rgb(107, 114, 128));
        view.setPadding(0, dp(8), 0, dp(8));
        return view;
    }

    private TextView card(String title, String detail) {
        TextView view = new TextView(this);
        view.setText(title + "\n" + detail);
        view.setTextSize(14);
        view.setTextColor(Color.rgb(24, 30, 37));
        view.setPadding(dp(14), dp(12), dp(14), dp(12));
        view.setBackground(rounded(Color.WHITE, dp(12)));
        view.setElevation(dp(1));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
        params.setMargins(0, 0, 0, dp(12));
        view.setLayoutParams(params);
        return view;
    }

    private EditText inputField(String hint, String value) {
        EditText editText = new EditText(this);
        editText.setHint(hint);
        editText.setText(value);
        editText.setSingleLine(true);
        editText.setPadding(dp(14), dp(10), dp(14), dp(10));
        editText.setBackground(rounded(Color.WHITE, dp(12)));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(52));
        params.setMargins(0, 0, 0, dp(12));
        editText.setLayoutParams(params);
        return editText;
    }

    private GradientDrawable rounded(int color, int radius) {
        GradientDrawable drawable = new GradientDrawable();
        drawable.setColor(color);
        drawable.setCornerRadius(radius);
        return drawable;
    }

    private TextView findTaggedText(LinearLayout parent, String tag) {
        for (int i = 0; i < parent.getChildCount(); i++) {
            View child = parent.getChildAt(i);
            if (tag.equals(child.getTag()) && child instanceof TextView) {
                return (TextView) child;
            }
        }
        return null;
    }

    private LinearLayout findTaggedLayout(LinearLayout parent, String tag) {
        for (int i = 0; i < parent.getChildCount(); i++) {
            View child = parent.getChildAt(i);
            if (tag.equals(child.getTag()) && child instanceof LinearLayout) {
                return (LinearLayout) child;
            }
            if (child instanceof HorizontalScrollView) {
                View nested = ((HorizontalScrollView) child).getChildAt(0);
                if (tag.equals(nested.getTag()) && nested instanceof LinearLayout) {
                    return (LinearLayout) nested;
                }
            }
        }
        return null;
    }

    private int dp(int value) {
        return (int) (value * getResources().getDisplayMetrics().density + 0.5f);
    }

    private void configureSystemBars() {
        Window window = getWindow();
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        window.setStatusBarColor(BG_COLOR);
        window.setNavigationBarColor(BG_COLOR);
        window.getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
    }

    private int statusBarHeight() {
        int resourceId = getResources().getIdentifier("status_bar_height", "dimen", "android");
        return resourceId > 0 ? getResources().getDimensionPixelSize(resourceId) : 0;
    }

    private int navBarHeight() {
        int resourceId = getResources().getIdentifier("navigation_bar_height", "dimen", "android");
        return resourceId > 0 ? getResources().getDimensionPixelSize(resourceId) : 0;
    }

    private static class SpaceView extends View {
        SpaceView(Activity activity) {
            super(activity);
        }
    }
}
