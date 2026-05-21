package com.xzxg.shop;

import android.app.Activity;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.view.inputmethod.EditorInfo;
import android.widget.Button;
import android.widget.EditText;
import android.widget.HorizontalScrollView;
import android.widget.LinearLayout;
import android.widget.PopupWindow;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

public class MainActivity extends Activity {
    private SessionStore sessionStore;
    private LocalChatStore chatStore;
    private ApiClient api;
    private LinearLayout root;
    private LinearLayout content;
    private LinearLayout chatList;
    private ScrollView chatScroll;
    private EditText input;
    private Button actionButton;
    private String localSessionId;
    private String serverSessionId = "";
    private TextView activeAssistant;
    private boolean streaming;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        sessionStore = new SessionStore(this);
        chatStore = new LocalChatStore(this);
        api = new ApiClient(sessionStore);
        createFreshLocalSession();
        renderChatHome();
        loadHomeCopy();
    }

    private void createFreshLocalSession() {
        localSessionId = "local_sess_" + System.currentTimeMillis();
        chatStore.ensureSession(localSessionId, "新的导购会话");
        serverSessionId = "";
    }

    private void baseScreen() {
        root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(Color.rgb(249, 250, 251));
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        root.addView(content, new LinearLayout.LayoutParams(-1, 0, 1));
        setContentView(root);
    }

    private void renderChatHome() {
        baseScreen();
        LinearLayout top = new LinearLayout(this);
        top.setGravity(Gravity.CENTER_VERTICAL);
        top.setPadding(dp(20), dp(18), dp(20), dp(6));
        Button menu = iconButton("☰");
        menu.setOnClickListener(v -> showDrawer());
        top.addView(menu, new LinearLayout.LayoutParams(dp(56), dp(56)));
        TextView status = muted(sessionStore.token().isEmpty() ? "未登录" : "已登录");
        status.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        top.addView(status, new LinearLayout.LayoutParams(0, -1, 1));
        content.addView(top);

        chatScroll = new ScrollView(this);
        chatScroll.setFillViewport(true);
        chatList = new LinearLayout(this);
        chatList.setOrientation(LinearLayout.VERTICAL);
        chatList.setGravity(Gravity.CENTER_HORIZONTAL);
        chatList.setPadding(dp(18), dp(24), dp(18), dp(18));
        chatScroll.addView(chatList, new ScrollView.LayoutParams(-1, -2));
        content.addView(chatScroll, new LinearLayout.LayoutParams(-1, 0, 1));

        TextView welcome = new TextView(this);
        welcome.setTag("welcome");
        welcome.setText("");
        welcome.setTextSize(26);
        welcome.setTypeface(Typeface.DEFAULT_BOLD);
        welcome.setTextColor(Color.rgb(20, 24, 30));
        welcome.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams welcomeParams = new LinearLayout.LayoutParams(-1, -2);
        welcomeParams.topMargin = dp(160);
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

        renderComposer();
    }

    private void renderComposer() {
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(16), dp(10), dp(16), dp(14));
        bar.setBackgroundColor(Color.WHITE);

        Button add = iconButton("+");
        add.setOnClickListener(v -> toastLine("图片输入入口已预留，后续接入相册/拍照。"));
        bar.addView(add, new LinearLayout.LayoutParams(dp(48), dp(48)));

        input = new EditText(this);
        input.setHint("问问想买什么...");
        input.setSingleLine(false);
        input.setMaxLines(4);
        input.setImeOptions(EditorInfo.IME_ACTION_SEND);
        input.setBackground(rounded(Color.rgb(243, 244, 246), dp(24)));
        input.setPadding(dp(16), dp(8), dp(16), dp(8));
        bar.addView(input, new LinearLayout.LayoutParams(0, -2, 1));

        actionButton = iconButton("🎙");
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
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(dp(52), dp(52));
        actionParams.leftMargin = dp(8);
        bar.addView(actionButton, actionParams);
        content.addView(bar, new LinearLayout.LayoutParams(-1, -2));
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
        LinearLayout drawer = new LinearLayout(this);
        drawer.setOrientation(LinearLayout.VERTICAL);
        drawer.setPadding(dp(22), dp(40), dp(18), dp(22));
        drawer.setBackgroundColor(Color.WHITE);

        if (sessionStore.token().isEmpty()) {
            Button login = primaryButton("登录 / 注册");
            login.setOnClickListener(v -> renderProfile());
            drawer.addView(login, new LinearLayout.LayoutParams(-1, dp(52)));
        } else {
            TextView user = title(sessionStore.nickname().isEmpty() ? "我的账号" : sessionStore.nickname());
            user.setOnClickListener(v -> renderProfile());
            drawer.addView(user);
        }

        drawer.addView(section("功能"));
        drawer.addView(navButton("商品", v -> renderProducts()));
        drawer.addView(navButton("购物车", v -> renderCart()));
        drawer.addView(navButton("订单", v -> renderOrders()));

        drawer.addView(section("历史聊天"));
        for (LocalChatStore.SessionSummary item : chatStore.recentSessions()) {
            drawer.addView(navButton(item.title == null ? "本地会话" : item.title, v -> toastLine("历史详情同步与加载已进入下一阶段。")));
        }

        SpaceView spacer = new SpaceView(this);
        drawer.addView(spacer, new LinearLayout.LayoutParams(-1, 0, 1));
        Button chat = primaryButton("✎  聊天");
        chat.setOnClickListener(v -> renderChatHome());
        drawer.addView(chat, new LinearLayout.LayoutParams(-1, dp(56)));

        PopupWindow popup = new PopupWindow(drawer, (int) (getResources().getDisplayMetrics().widthPixels * 0.82), -1, true);
        popup.setOutsideTouchable(true);
        popup.setBackgroundDrawable(rounded(Color.WHITE, 0));
        popup.showAtLocation(root, Gravity.LEFT | Gravity.TOP, 0, 0);
    }

    private void sendMessage(String text) {
        if (sessionStore.token().isEmpty()) {
            chatStore.saveMessage(localSessionId, "user", text, "local_only");
            toastLine("请先登录，当前消息已保存在本地。");
            renderProfile();
            return;
        }
        clearWelcomeIfNeeded();
        input.setText("");
        addBubble(text, true);
        chatStore.saveMessage(localSessionId, "user", text, "pending");
        streaming = true;
        actionButton.setText("■");
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
        activeAssistant = addBubble("", false);
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
            addSystemLine("已收到结构化内容，商品卡片渲染将在下一阶段完善。");
            return;
        }
        if ("followups".equals(type)) {
            JSONArray questions = event.optJSONArray("questions");
            addSystemLine("你还可以继续追问：" + (questions == null ? "" : questions.toString()));
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
            activeAssistant = addBubble("", false);
        }
        activeAssistant.append(delta);
        scrollBottom();
    }

    private void finishStream() {
        if (activeAssistant != null) {
            chatStore.saveMessage(localSessionId, "assistant", activeAssistant.getText().toString(), "completed");
        }
        streaming = false;
        actionButton.setText("🎙");
    }

    private void failStream(String message) {
        addSystemLine(message);
        streaming = false;
        actionButton.setText("🎙");
    }

    private void renderProfile() {
        baseScreen();
        addPageHeader("我的", "账号、资料和测试设置");
        LinearLayout page = pageBody();
        if (sessionStore.token().isEmpty()) {
            EditText username = inputField("账号", "user");
            EditText password = inputField("密码", "user123456");
            page.addView(username);
            page.addView(password);
            Button login = primaryButton("登录");
            login.setOnClickListener(v -> login(username.getText().toString(), password.getText().toString()));
            page.addView(login, new LinearLayout.LayoutParams(-1, dp(52)));
            Button registerUser = secondaryButton("注册顾客账号");
            registerUser.setOnClickListener(v -> register(username.getText().toString(), password.getText().toString(), "user"));
            page.addView(registerUser);
            Button registerMerchant = secondaryButton("注册商家账号");
            registerMerchant.setOnClickListener(v -> register(username.getText().toString(), password.getText().toString(), "merchant"));
            page.addView(registerMerchant);
            page.addView(muted("当前验证码为后端 mock，默认使用 123456；未来接入短信/邮件云服务。"));
        } else {
            page.addView(card("当前账号", "昵称：" + sessionStore.nickname() + "\n角色：" + sessionStore.role()));
            EditText nickname = inputField("昵称", sessionStore.nickname());
            page.addView(nickname);
            Button save = primaryButton("保存资料");
            save.setOnClickListener(v -> updateProfile(nickname.getText().toString()));
            page.addView(save, new LinearLayout.LayoutParams(-1, dp(52)));
            EditText oldPassword = inputField("原密码", "");
            EditText newPassword = inputField("新密码", "");
            page.addView(oldPassword);
            page.addView(newPassword);
            Button changePassword = secondaryButton("修改密码");
            changePassword.setOnClickListener(v -> changePassword(oldPassword.getText().toString(), newPassword.getText().toString()));
            page.addView(changePassword);
            Button logout = secondaryButton("退出登录");
            logout.setOnClickListener(v -> {
                sessionStore.clearAuth();
                renderChatHome();
            });
            page.addView(logout);
        }
        if (BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            EditText apiBase = inputField("后端地址", sessionStore.apiBase());
            page.addView(apiBase);
            Button saveApi = secondaryButton("保存测试地址");
            saveApi.setOnClickListener(v -> {
                sessionStore.saveApiBase(apiBase.getText().toString().trim());
                api = new ApiClient(sessionStore);
                toastLine("已保存测试后端地址");
            });
            page.addView(saveApi);
        }
    }

    private void register(String username, String password, String role) {
        new Thread(() -> {
            try {
                String displayName = role.equals("merchant") ? "新商家" : username;
                JSONObject result = api.register(username, password, displayName, role, displayName, "123456");
                JSONObject account = result.optJSONObject("account");
                sessionStore.saveAuth(result.optString("token"), account == null ? role : account.optString("role", role), account == null ? displayName : account.optString("display_name", displayName));
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
                sessionStore.saveAuth(result.optString("token"), account == null ? "user" : account.optString("role", "user"), account == null ? username : account.optString("display_name", username));
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

    private void updateProfile(String nickname) {
        new Thread(() -> {
            try {
                JSONObject profile = api.updateProfile(nickname);
                sessionStore.saveAuth(sessionStore.token(), sessionStore.role(), profile.optString("nickname", nickname));
                runOnUiThread(() -> toastLine("资料已保存"));
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("保存失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderProducts() {
        renderListPage("商品", "来自后端商品接口的移动端列表", () -> {
            try {
                return api.products();
            } catch (Exception error) {
                return new JSONArray();
            }
        }, "name", "price");
    }

    private void renderCart() {
        baseScreen();
        addPageHeader("购物车", "顾客结算入口");
        LinearLayout page = pageBody();
        if (sessionStore.token().isEmpty()) {
            page.addView(card("请先登录", "登录后可查看购物车并结算。"));
            return;
        }
        new Thread(() -> {
            try {
                JSONObject cart = api.cart();
                runOnUiThread(() -> page.addView(card("购物车", cart.toString())));
            } catch (Exception error) {
                runOnUiThread(() -> page.addView(card("加载失败", error.getMessage())));
            }
        }).start();
    }

    private void renderOrders() {
        renderListPage("订单", "顾客订单列表", () -> {
            try {
                return api.orders();
            } catch (Exception error) {
                return new JSONArray();
            }
        }, "order_id", "status");
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
        LinearLayout header = new LinearLayout(this);
        header.setGravity(Gravity.CENTER_VERTICAL);
        header.setPadding(dp(18), dp(18), dp(18), dp(12));
        Button back = iconButton("☰");
        back.setOnClickListener(v -> showDrawer());
        header.addView(back, new LinearLayout.LayoutParams(dp(52), dp(52)));
        LinearLayout texts = new LinearLayout(this);
        texts.setOrientation(LinearLayout.VERTICAL);
        texts.addView(title(title));
        texts.addView(muted(subtitle));
        header.addView(texts, new LinearLayout.LayoutParams(0, -2, 1));
        content.addView(header);
    }

    private LinearLayout pageBody() {
        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(dp(18), dp(10), dp(18), dp(18));
        scroll.addView(page);
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        return page;
    }

    private TextView addBubble(String text, boolean right) {
        TextView bubble = new TextView(this);
        bubble.setText(text);
        bubble.setTextSize(16);
        bubble.setLineSpacing(4, 1);
        bubble.setPadding(dp(14), dp(10), dp(14), dp(10));
        bubble.setTextColor(right ? Color.WHITE : Color.rgb(24, 30, 37));
        bubble.setBackground(rounded(right ? Color.rgb(20, 20, 20) : Color.WHITE, dp(18)));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams((int) (getResources().getDisplayMetrics().widthPixels * 0.78), -2);
        params.gravity = right ? Gravity.RIGHT : Gravity.LEFT;
        params.setMargins(0, dp(8), 0, dp(8));
        chatList.addView(bubble, params);
        scrollBottom();
        return bubble;
    }

    private void addSystemLine(String text) {
        TextView view = muted(text);
        view.setGravity(Gravity.CENTER);
        chatList.addView(view, new LinearLayout.LayoutParams(-1, -2));
        scrollBottom();
    }

    private void toastLine(String text) {
        if (chatList != null && content != null) {
            addSystemLine(text);
        }
    }

    private void clearWelcomeIfNeeded() {
        if (chatList.getChildCount() > 0 && "welcome".equals(chatList.getChildAt(0).getTag())) {
            chatList.removeAllViews();
        }
    }

    private void scrollBottom() {
        if (chatScroll != null) {
            chatScroll.post(() -> chatScroll.fullScroll(View.FOCUS_DOWN));
        }
    }

    private Button iconButton(String text) {
        Button button = new Button(this);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextSize(22);
        button.setTextColor(Color.BLACK);
        button.setBackground(rounded(Color.WHITE, dp(28)));
        return button;
    }

    private Button primaryButton(String text) {
        Button button = new Button(this);
        button.setText(text);
        button.setAllCaps(false);
        button.setTextColor(Color.WHITE);
        button.setTextSize(16);
        button.setBackground(rounded(Color.BLACK, dp(24)));
        return button;
    }

    private Button secondaryButton(String text) {
        Button button = new Button(this);
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
        view.setTextSize(15);
        view.setTextColor(Color.rgb(24, 30, 37));
        view.setPadding(dp(16), dp(14), dp(16), dp(14));
        view.setBackground(rounded(Color.WHITE, dp(14)));
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

    private static class SpaceView extends View {
        SpaceView(Activity activity) {
            super(activity);
        }
    }
}
