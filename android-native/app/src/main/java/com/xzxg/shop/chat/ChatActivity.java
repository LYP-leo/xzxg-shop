package com.xzxg.shop.chat;

import com.xzxg.shop.BuildConfig;
import com.xzxg.shop.chat.ChatAttachmentController.PendingAttachment;
import com.xzxg.shop.chat.ChatAttachmentController.UploadingMessageViewState;
import com.xzxg.shop.account.AccountActivity;
import com.xzxg.shop.account.AccountSessionHelper;
import com.xzxg.shop.account.AccountUi;
import com.xzxg.shop.account.EditProfileActivity;
import com.xzxg.shop.account.LoginActivity;
import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.cart.CartActivity;
import com.xzxg.shop.coupon.CouponActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.order.OrderListActivity;
import com.xzxg.shop.product.ProductDetailActivity;
import com.xzxg.shop.product.ProductListActivity;
import com.xzxg.shop.settings.SettingsActivity;
import com.xzxg.shop.storage.LocalChatStore;
import com.xzxg.shop.storage.SessionStore;
import com.xzxg.shop.ui.BottomSheetHelper;
import com.xzxg.shop.ui.ImageLoader;
import com.xzxg.shop.ui.MarkdownRenderer;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;
import com.xzxg.shop.voice.VoiceInputController;
import com.xzxg.shop.voice.VoiceInputController.SpeechMode;

import android.animation.ValueAnimator;
import android.animation.AnimatorListenerAdapter;
import android.app.Activity;
import android.app.AlertDialog;
import android.Manifest;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.provider.MediaStore;
import android.speech.RecognizerIntent;
import android.util.Log;
import android.view.ViewGroup;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowManager;
import android.view.animation.DecelerateInterpolator;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.HorizontalScrollView;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ProgressBar;
import android.widget.ScrollView;
import android.widget.TextView;
import android.widget.Toast;
import android.text.Editable;
import android.text.TextWatcher;


import org.json.JSONArray;
import org.json.JSONObject;

import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Calendar;
import java.util.Collections;
import java.util.Deque;
import java.util.HashMap;
import java.util.HashSet;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Random;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class ChatActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private static final int REQUEST_PICK_IMAGE = 3101;
    private static final int REQUEST_PICK_FILE = 3102;
    private static final int REQUEST_RECORD_AUDIO = 3103;
    private static final int REQUEST_SPEECH_INPUT = 3105;
    private static final boolean DEBUG_AGENT_BLOCKS = BuildConfig.DEBUG;
    private static final String TAG = "XzxgShop";
    private static final String[] DEFAULT_HOME_PROMPTS = new String[]{
            "城市通勤自行车推荐", "为我推荐一些护肤品", "预算三千以内的手机怎么选", "适合送父母的按摩仪推荐", "干皮秋冬保湿水怎么选",
            "帮我找适合办公室的咖啡机", "儿童安全座椅怎么挑", "入门跑步鞋推荐", "油皮夏天底妆怎么选", "敏感肌可以用的面霜推荐",
            "学生党平价防晒推荐", "通勤用双肩包怎么选", "适合小户型的空气炸锅推荐", "租房党投影仪怎么选", "两千以内洗地机推荐",
            "扫地机器人避坑指南", "新手露营装备清单", "适合办公室久坐的腰靠推荐", "送女朋友生日礼物推荐", "送男朋友实用礼物推荐",
            "父亲节礼物怎么选", "母亲节护肤礼盒推荐", "宝宝湿巾和纸尿裤怎么选", "新生儿奶瓶推荐", "孕妇可用护肤品推荐",
            "儿童学习桌椅怎么挑", "家用净水器怎么选", "厨房小家电哪些值得买", "适合一人食的电饭煲推荐", "家用咖啡豆和咖啡机搭配",
            "降噪耳机通勤推荐", "运动耳机怎么选", "平板电脑学习记笔记推荐", "轻薄本办公怎么选", "游戏本预算六千推荐",
            "拍照好的手机推荐", "老人用手机怎么选", "儿童电话手表推荐", "智能手表健康监测怎么选", "家庭 NAS 入门推荐",
            "路由器大户型怎么选", "显示器办公护眼推荐", "机械键盘新手推荐", "人体工学椅怎么选", "护眼台灯学生党推荐",
            "卧室香薰和加湿器推荐", "床垫软硬怎么选", "四件套材质怎么挑", "猫砂和猫粮怎么选", "狗狗自动喂食器推荐",
            "车载吸尘器推荐", "行车记录仪怎么选", "电动车头盔推荐", "长途旅行行李箱怎么选", "防晒衣和遮阳伞怎么选",
            "户外徒步鞋推荐", "骑行头盔和手套推荐", "健身新手哑铃怎么选", "家用跑步机避坑", "瑜伽垫厚度怎么选",
            "游泳装备新手推荐", "男士控油洗面奶推荐", "男士剃须刀怎么选", "头发干枯毛躁护发推荐", "防脱洗发水怎么选",
            "口红色号日常通勤推荐", "粉底液色号怎么选", "新手化妆刷套装推荐", "香水入门怎么选", "美白精华怎么避坑",
            "抗老精华适合多少岁用", "眼霜黑眼圈推荐", "身体乳秋冬推荐", "牙刷电动还是手动好", "冲牙器新手推荐",
            "益生菌和维生素怎么选", "低糖零食推荐", "早餐麦片怎么选", "办公室咖啡和茶包推荐", "年货礼盒推荐",
            "端午礼盒怎么选", "中秋月饼礼盒推荐", "搬家清洁用品清单", "浴室收纳怎么做", "厨房锅具套装推荐",
            "不粘锅和铁锅怎么选", "保温杯材质怎么选", "雨天通勤鞋推荐", "冬季保暖内衣推荐", "羽绒服充绒量怎么选",
            "夏季凉感床品推荐", "通勤衬衫不易皱推荐", "牛仔裤版型怎么选", "女生面试穿搭推荐", "男生日常通勤鞋推荐",
            "儿童书包护脊推荐", "学习机和平板怎么选", "蓝牙音箱户外推荐", "家庭影院音响怎么选", "相机新手入门推荐"
    };
    private SessionStore sessionStore;
    private LocalChatStore chatStore;
    private ApiClient api;
    private ImageLoader imageLoader;
    private ChatAttachmentController attachmentController;
    private ChatMarkdownRenderer markdownRenderer;
    private VoiceInputController voiceController;
    private AgentStreamController streamController;
    private ChatHistoryDrawer historyDrawer;
    private AgentMessageRenderer messageRenderer;
    private FrameLayout root;
    private View statusBarScrim;
    private LinearLayout content;
    private LinearLayout chatViewport;
    private LinearLayout chatList;
    private ScrollView chatScroll;
    private EditText input;
    private TextView actionButton;
    private Button addButton;
    private LinearLayout composerOuter;
    private LinearLayout attachmentBufferView;
    private LinearLayout attachmentPanelView;
    private boolean voiceMode;
    private boolean voiceResultSent;
    private FrameLayout drawerLayer;
    private LinearLayout drawerPanel;
    private LinearLayout drawerHistoryList;
    private EditText drawerSearchInput;
    private ScrollView drawerHistoryScroll;
    private String localSessionId;
    private String serverSessionId = "";
    private TextView activeAssistant;
    private LinearLayout activeAssistantMessageBubble;
    private TextView loadingAssistant;
    private StringBuilder activeAssistantMarkdown;
    private StringBuilder activeAssistantFullMarkdown;
    private boolean activeAssistantFormMode;
    private boolean activeAssistantImplicitTableMode;
    private StringBuilder activeAssistantFormMarkdown = new StringBuilder();
    private StringBuilder activeAssistantTagPending = new StringBuilder();
    private LinearLayout activeAssistantFormCodeBox;
    private TextView activeAssistantFormCodeText;
    private JSONArray activeAssistantBlocks = new JSONArray();
    private JSONArray activeAssistantSegments = new JSONArray();
    private ThinkingViewState activeThinking;
    private boolean activeAssistantAnswerStarted;
    private boolean hasReceivedThinkingDelta;
    private boolean loggedMissingThinkingDelta;
    private final List<String> pendingThinkingStatuses = new ArrayList<>();
    private JSONArray activeFollowups = new JSONArray();
    private View activeFollowupsView;
    private String activePage = "chat";
    private final Map<String, JSONObject> productDetailCache = new HashMap<>();
    private final Set<String> activeRenderedProductIds = new HashSet<>();
    private final Object sessionSyncLock = new Object();
    private final Deque<SessionSyncJob> sessionSyncQueue = new ArrayDeque<>();
    private boolean sessionSyncRunning;
    private final Deque<Runnable> backStack = new ArrayDeque<>();
    private Toast activeToast;
    private boolean manualWindowInsets;
    private int appliedTopInset;
    private int appliedBottomInset;
    private boolean chatAutoScrollEnabled = true;
    private boolean userDetachedFromBottom;
    private String lastRiskNoticeText = "";
    private long lastRiskNoticeAt;

    private String currentAttachmentSessionKey() {
        return attachmentController.sessionKey(localSessionId);
    }

    private List<PendingAttachment> currentPendingAttachments() {
        return attachmentController.currentPendingAttachments(localSessionId);
    }

    private JSONArray localAttachmentPlaceholders(List<PendingAttachment> attachments) {
        return attachmentController.localPlaceholders(attachments);
    }

    private static class ThinkingViewState {
        LinearLayout container;
        TextView header;
        LinearLayout detail;
        boolean expanded = true;
        boolean completed;
        boolean segmentSaved;
        boolean userToggled;
        boolean hasRendered;
        boolean lastRenderedExpanded = true;
        ValueAnimator detailAnimator;
        final Map<String, ThinkingStageState> stages = new HashMap<>();
    }

    private static class ThinkingStageState {
        String stage;
        String title;
        String status = "pending";
        final StringBuilder text = new StringBuilder();
        JSONArray items = new JSONArray();
        LinearLayout row;
        TextView markerView;
        TextView titleView;
        TextView bodyView;
        LinearLayout itemList;
    }

    private static class SessionSyncJob {
        final String localSessionId;
        final String serverSessionId;
        final String title;
        final String summary;
        int retryCount;

        SessionSyncJob(String localSessionId, String serverSessionId, String title, String summary) {
            this.localSessionId = localSessionId;
            this.serverSessionId = serverSessionId;
            this.title = title;
            this.summary = summary;
        }
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureSystemBars();
        sessionStore = sessionStore();
        chatStore = localChatStore();
        api = api();
        imageLoader = imageLoader();
        attachmentController = new ChatAttachmentController();
        markdownRenderer = new ChatMarkdownRenderer(this, markdown -> sanitizeAgentMarkdown(markdown).visibleMarkdown, this::enableCopy);
        voiceController = new VoiceInputController();
        streamController = new AgentStreamController();
        historyDrawer = new ChatHistoryDrawer(chatStore);
        messageRenderer = new AgentMessageRenderer(this, api, imageLoader, markdownRenderer, this::enableCopy, this::navigateRoute, new AgentMessageRenderer.ProductActions() {
            @Override
            public void openDetail(String productId) {
                openProductDetailActivity(productId);
            }

            @Override
            public void addToCart(JSONObject item, Button sourceButton) {
                ChatActivity.this.addProductToCart(item, sourceButton);
            }
        });
        createFreshLocalSession();
        if (handleInitialRoute(getIntent())) {
            return;
        }
        renderChatHome();
        loadHomeCopy();
        validateStoredToken();
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        handleInitialRoute(intent);
    }

    private boolean handleInitialRoute(Intent intent) {
        String initialRoute = intent == null ? "" : intent.getStringExtra(Routes.EXTRA_ROUTE);
        if (Routes.CART.equals(initialRoute)) {
            persistAndCancelActiveStreamForNavigation();
            renderChatHome();
            loadHomeCopy();
            validateStoredToken();
            startActivity(new Intent(this, CartActivity.class));
            return true;
        }
        if (Routes.ORDERS.equals(initialRoute)) {
            persistAndCancelActiveStreamForNavigation();
            renderChatHome();
            loadHomeCopy();
            validateStoredToken();
            startActivity(new Intent(this, OrderListActivity.class));
            return true;
        }
        if (Routes.SETTINGS.equals(initialRoute)) {
            persistAndCancelActiveStreamForNavigation();
            renderChatHome();
            loadHomeCopy();
            validateStoredToken();
            startActivity(new Intent(this, SettingsActivity.class));
            return true;
        }
        if (Routes.ACCOUNT.equals(initialRoute)) {
            persistAndCancelActiveStreamForNavigation();
            renderChatHome();
            loadHomeCopy();
            validateStoredToken();
            startActivity(new Intent(this, AccountActivity.class));
            return true;
        }
        if (Routes.EDIT_PROFILE.equals(initialRoute)) {
            persistAndCancelActiveStreamForNavigation();
            renderChatHome();
            loadHomeCopy();
            validateStoredToken();
            startActivity(new Intent(this, EditProfileActivity.class));
            return true;
        }
        return false;
    }

    @Override
    public void onBackPressed() {
        if (drawerLayer != null) {
            closeDrawerAnimated();
            return;
        }
        if ("login".equals(activePage)) {
            renderChatHome();
            loadHomeCopy();
            return;
        }
        if (!backStack.isEmpty()) {
            Runnable action = backStack.pop();
            action.run();
            return;
        }
        if (isPrimaryPage(activePage)) {
            if (sessionStore.token().isEmpty()) {
                openLoginPage();
            } else {
                showDrawer();
            }
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

    private void openLoginPage() {
        closeDrawer();
        persistAndCancelActiveStreamForNavigation();
        startActivity(new Intent(this, LoginActivity.class));
        renderChatHome();
        loadHomeCopy();
    }

    private void openProductDetailActivity(String productId) {
        String id = productId == null ? "" : productId.trim();
        if (id.isEmpty()) {
            toastLine("商品信息缺失");
            return;
        }
        persistAndCancelActiveStreamForNavigation();
        Intent intent = new Intent(this, ProductDetailActivity.class);
        intent.putExtra(Routes.EXTRA_PRODUCT_ID, id);
        startActivity(intent);
    }

    private void baseScreen() {
        chatViewport = null;
        appliedTopInset = -1;
        appliedBottomInset = -1;
        root = new FrameLayout(this);
        statusBarScrim = null;
        root.setBackgroundColor(BG_COLOR);
        root.setClipChildren(false);
        root.setClipToPadding(false);
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setClipChildren(false);
        content.setClipToPadding(false);
        content.setPadding(0, 0, 0, 0);
        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        if (manualWindowInsets) {
            statusBarScrim = new View(this);
            statusBarScrim.setBackgroundColor(BG_COLOR);
            FrameLayout.LayoutParams scrimParams = new FrameLayout.LayoutParams(-1, 0);
            scrimParams.gravity = Gravity.TOP;
            root.addView(statusBarScrim, scrimParams);
        }
        setContentView(root);
        bindRootWindowInsets();
    }

    private void renderChatHome() {
        closeDrawer();
        activePage = "chat";
        useKeyboardResize();
        backStack.clear();
        baseScreen();

        root.removeView(content);
        chatViewport = new LinearLayout(this);
        chatViewport.setOrientation(LinearLayout.VERTICAL);
        chatViewport.setClipChildren(false);
        chatViewport.setClipToPadding(false);
        chatViewport.setPadding(0, 0, 0, 0);
        content = chatViewport;
        root.addView(chatViewport, new FrameLayout.LayoutParams(-1, -1));

        chatViewport.addView(createTopBar("AI导购", null), new LinearLayout.LayoutParams(-1, dp(56)));
        chatViewport.addView(createChatMessageLayer(), new LinearLayout.LayoutParams(-1, 0, 1));
        chatViewport.addView(createComposerBar(), new LinearLayout.LayoutParams(-1, -2));
        applyContentInsets(appliedTopInset < 0 ? 0 : appliedTopInset, appliedBottomInset < 0 ? 0 : appliedBottomInset);
    }

    private boolean isChatScrolledToBottom() {
        if (chatScroll == null || chatScroll.getChildCount() == 0) {
            return true;
        }
        View child = chatScroll.getChildAt(0);
        int distance = child.getBottom() - (chatScroll.getScrollY() + chatScroll.getHeight());
        return distance <= dp(24);
    }

    private void useKeyboardResize() {
        getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
    }

    private void useKeyboardOverlay() {
        getWindow().setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_NOTHING);
    }

    private void restoreKeyboardModeForActivePage() {
        if ("chat".equals(activePage)) {
            useKeyboardResize();
        } else {
            useKeyboardOverlay();
        }
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
        menu.setOnClickListener(v -> {
            hideAttachmentPanel();
            hideKeyboard();
            if (sessionStore.token().isEmpty()) {
                openLoginPage();
            } else {
                showDrawer();
            }
        });
        toolbar.addView(menu, new LinearLayout.LayoutParams(dp(40), dp(40)));

        TextView title = new TextView(this);
        title.setText(titleText);
        title.setTextSize(18);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        title.setTextColor(Color.rgb(20, 24, 30));
        title.setGravity(Gravity.CENTER_VERTICAL);
        title.setPadding(dp(10), 0, dp(8), 0);
        toolbar.addView(title, new LinearLayout.LayoutParams(0, -1, 1));

        toolbar.setOnClickListener(v -> {
            hideAttachmentPanel();
            hideKeyboard();
        });
        return toolbar;
    }

    private View createBackTopBar(String titleText, Runnable backAction) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, () -> {
            hideKeyboard();
            if (backAction != null) {
                backAction.run();
            } else {
                onBackPressed();
            }
        });
    }

    private View createChatMessageLayer() {
        chatScroll = new ScrollView(this);
        chatScroll.setFillViewport(true);
        chatList = new LinearLayout(this);
        chatList.setOrientation(LinearLayout.VERTICAL);
        chatList.setGravity(Gravity.CENTER_HORIZONTAL);
        chatList.setPadding(dp(16), dp(12), dp(16), dp(12));
        chatScroll.addView(chatList, new ScrollView.LayoutParams(-1, -2));
        chatScroll.setOnTouchListener((view, event) -> {
            if (event.getAction() == MotionEvent.ACTION_DOWN) {
                hideAttachmentPanel();
                hideKeyboard();
                if (streamController.isStreaming() && !isChatScrolledToBottom()) {
                    chatAutoScrollEnabled = false;
                    userDetachedFromBottom = true;
                }
            } else if (event.getAction() == MotionEvent.ACTION_UP || event.getAction() == MotionEvent.ACTION_CANCEL) {
                updateChatAutoScrollFromPosition();
            }
            return false;
        });
        chatScroll.setOnScrollChangeListener((v, scrollX, scrollY, oldScrollX, oldScrollY) -> {
            if (isChatScrolledToBottom()) {
                chatAutoScrollEnabled = true;
                userDetachedFromBottom = false;
            } else if (streamController.isStreaming() && scrollY < oldScrollY) {
                chatAutoScrollEnabled = false;
                userDetachedFromBottom = true;
            }
        });

        TextView welcome = new TextView(this);
        welcome.setTag("welcome");
        welcome.setText(greetingText());
        welcome.setTextSize(24);
        welcome.setTypeface(Typeface.DEFAULT_BOLD);
        welcome.setTextColor(Color.rgb(20, 24, 30));
        welcome.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams welcomeParams = new LinearLayout.LayoutParams(-1, -2);
        welcomeParams.topMargin = dp(96);
        chatList.addView(welcome, welcomeParams);

        TextView hint = muted("");
        hint.setGravity(Gravity.CENTER);
        chatList.addView(hint, new LinearLayout.LayoutParams(-1, -2));

        LinearLayout suggestions = new LinearLayout(this);
        suggestions.setTag("suggestions");
        suggestions.setOrientation(LinearLayout.VERTICAL);
        suggestions.setGravity(Gravity.CENTER);
        LinearLayout.LayoutParams suggestionParams = new LinearLayout.LayoutParams(-1, -2);
        suggestionParams.topMargin = dp(18);
        chatList.addView(suggestions, suggestionParams);
        renderHomeSuggestions(suggestions, null);

        activeRenderedProductIds.clear();
        renderStoredMessagesIfAny();

        return chatScroll;
    }

    private View createComposerBar() {
        composerOuter = new LinearLayout(this);
        composerOuter.setOrientation(LinearLayout.VERTICAL);
        composerOuter.setPadding(dp(14), dp(6), dp(14), dp(8));
        composerOuter.setBackgroundColor(BG_COLOR);
        composerOuter.setClickable(true);
        composerOuter.setClipChildren(false);
        composerOuter.setClipToPadding(false);

        attachmentBufferView = new LinearLayout(this);
        attachmentBufferView.setOrientation(LinearLayout.HORIZONTAL);
        attachmentBufferView.setVisibility(View.GONE);
        composerOuter.addView(attachmentBufferView, new LinearLayout.LayoutParams(-1, dp(92)));

        attachmentPanelView = new LinearLayout(this);
        attachmentPanelView.setOrientation(LinearLayout.HORIZONTAL);
        attachmentPanelView.setPadding(dp(2), dp(4), dp(2), dp(8));
        attachmentPanelView.setVisibility(View.GONE);
        composerOuter.addView(attachmentPanelView, new LinearLayout.LayoutParams(-1, dp(82)));

        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(12), 0, dp(7), 0);
        bar.setBackground(rounded(Color.WHITE, dp(26)));
        bar.setElevation(dp(2));
        bar.setClipChildren(false);
        bar.setClipToPadding(false);

        addButton = transparentIconButton("+");
        addButton.setTextSize(24);
        addButton.setOnClickListener(v -> toggleAttachmentPanel());
        bar.addView(addButton, new LinearLayout.LayoutParams(dp(40), dp(52)));

        input = new EditText(this);
        input.setHint("输入问题或直接发送...");
        input.setHintTextColor(Color.rgb(156, 163, 175));
        input.setSingleLine(false);
        input.setMaxLines(4);
        input.setImeOptions(EditorInfo.IME_ACTION_SEND);
        input.setTextSize(15);
        input.setBackground(new ColorDrawable(Color.TRANSPARENT));
        input.setPadding(dp(8), 0, dp(8), 0);
        input.setOnClickListener(v -> hideAttachmentPanel());
        input.setOnFocusChangeListener((v, hasFocus) -> {
            if (hasFocus) {
                hideAttachmentPanel();
            }
        });
        input.addTextChangedListener(new TextWatcher() {
            @Override
            public void beforeTextChanged(CharSequence s, int start, int count, int after) {
            }

            @Override
            public void onTextChanged(CharSequence s, int start, int before, int count) {
                updateInputActionButtonState();
            }

            @Override
            public void afterTextChanged(Editable s) {
            }
        });
        bar.addView(input, new LinearLayout.LayoutParams(0, dp(52), 1));

        actionButton = roundActionButton("🎙");
        actionButton.setOnClickListener(v -> {
            if (streamController.isStreaming()) {
                stopActiveStream();
                return;
            }
            String text = input == null ? "" : input.getText().toString().trim();
            if (!text.isEmpty() || !currentPendingAttachments().isEmpty()) {
                sendCurrentInput();
                return;
            }
            if (voiceMode) {
                exitVoiceMode();
                return;
            }
            enterVoiceMode();
        });
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(dp(42), dp(42));
        bar.addView(actionButton, actionParams);
        composerOuter.addView(bar, new LinearLayout.LayoutParams(-1, dp(52)));
        renderAttachmentPanel();
        renderAttachmentBuffer();
        applyComposerBottomInset(appliedBottomInset);
        return composerOuter;
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

    private void setActionButtonText(String text) {
        if (actionButton == null) {
            return;
        }
        actionButton.setText(text);
        actionButton.setTextSize(19);
    }

    private void updateInputActionButtonState() {
        if (actionButton == null) {
            return;
        }
        String text = input == null ? "" : input.getText().toString().trim();
        if (streamController.isStreaming()) {
            setActionButtonText("■");
        } else if (!text.isEmpty() || !currentPendingAttachments().isEmpty()) {
            setActionButtonText("➤");
        } else if (voiceMode) {
            setActionButtonText(voiceController.isRealtimeEnding() ? "…" : "■");
        } else {
            setActionButtonText("🎙");
        }
    }

    private void sendCurrentInput() {
        String text = input == null ? "" : input.getText().toString().trim();
        String attachmentSessionKey = currentAttachmentSessionKey();
        List<PendingAttachment> attachmentSnapshot = new ArrayList<>(currentPendingAttachments());
        if (text.isEmpty() && attachmentSnapshot.isEmpty()) {
            enterVoiceMode();
            return;
        }
        if (text.isEmpty() && !attachmentSnapshot.isEmpty()) {
            text = defaultAttachmentPrompt(attachmentSnapshot);
        }
        final String messageText = text;
        JSONArray attachments = new JSONArray();
        hideAttachmentPanel();
        if (attachmentSnapshot.isEmpty()) {
            sendMessage(messageText, attachments);
            return;
        }
        if (sessionStore.token().isEmpty()) {
            sendMessage(messageText, localAttachmentPlaceholders(attachmentSnapshot));
            List<PendingAttachment> current = attachmentController.pendingAttachments(attachmentSessionKey);
            if (current != null) {
                current.removeAll(attachmentSnapshot);
            }
            renderAttachmentBuffer();
            updateInputActionButtonState();
            return;
        }
        UploadingMessageViewState uploadingState = addUploadingUserMessageBubble(messageText, attachmentSnapshot);
        if (input != null && !voiceMode) {
            input.setText("");
        }
        setActionButtonText("…");
        actionButton.setEnabled(false);
        new Thread(() -> {
            try {
                JSONArray uploaded = new JSONArray();
                for (int i = 0; i < attachmentSnapshot.size(); i++) {
                    PendingAttachment attachment = attachmentSnapshot.get(i);
                    int index = i;
                    runOnUiThread(() -> updateUploadingAttachmentState(uploadingState, index, "正在上传 " + (index + 1) + "/" + attachmentSnapshot.size()));
                    if (!isSupportedAgentAttachment(attachment)) {
                        throw new RuntimeException("当前只支持发送图片和 PDF 文件");
                    }
                    byte[] data = attachmentController.readBytes(getContentResolver(), attachment.uri);
                    if (data.length == 0) {
                        throw new RuntimeException("附件内容为空，无法发送");
                    }
                    JSONObject item = api.uploadAttachment(attachment.name, attachment.mimeType, attachment.type, data);
                    try {
                        item.put("mime_type", attachment.mimeType == null ? "application/octet-stream" : attachment.mimeType);
                        item.put("size", data.length);
                        item.put("local_uri", attachment.uri == null ? "" : attachment.uri.toString());
                    } catch (Exception ignored) {
                    }
                    uploaded.put(item);
                    runOnUiThread(() -> updateUploadingAttachmentState(uploadingState, index, "上传完成"));
                }
                runOnUiThread(() -> {
                    List<PendingAttachment> current = attachmentController.pendingAttachments(attachmentSessionKey);
                    if (current != null) {
                        current.removeAll(attachmentSnapshot);
                    }
                    renderAttachmentBuffer();
                    if (actionButton != null) {
                        actionButton.setEnabled(true);
                    }
                    finishUploadingUserMessageBubble(uploadingState, messageText, uploaded);
                });
            } catch (Exception error) {
                Log.w(TAG, "upload attachment failed", error);
                runOnUiThread(() -> {
                    if (input != null && !voiceMode && input.getText().toString().trim().isEmpty() && messageText != null && !messageText.trim().isEmpty()) {
                        input.setText(messageText);
                        input.setSelection(input.getText().length());
                    }
                    if (actionButton != null) {
                        actionButton.setEnabled(true);
                        updateInputActionButtonState();
                    }
                    failUploadingUserMessageBubble(uploadingState, error);
                    toastLine("附件上传失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private String defaultAttachmentPrompt(List<PendingAttachment> attachments) {
        if (attachments == null || attachments.isEmpty()) {
            return "";
        }
        int imageCount = 0;
        int fileCount = 0;
        for (PendingAttachment attachment : attachments) {
            if (attachment != null && "image".equals(attachment.type)) {
                imageCount++;
            } else {
                fileCount++;
            }
        }
        if (imageCount > 0 && fileCount == 0) {
            return "帮我看看这张图片";
        }
        if (imageCount == 0 && fileCount > 0) {
            return "请根据这个附件帮我分析";
        }
        return "请根据这些附件帮我分析";
    }

    private boolean isSupportedAgentAttachment(PendingAttachment attachment) {
        if (attachment == null) {
            return false;
        }
        if ("image".equals(attachment.type)) {
            return true;
        }
        String mime = attachment.mimeType == null ? "" : attachment.mimeType.toLowerCase();
        String name = attachment.name == null ? "" : attachment.name.toLowerCase();
        return "application/pdf".equals(mime) || name.endsWith(".pdf");
    }

    private void toggleAttachmentPanel() {
        if (attachmentPanelView == null) {
            return;
        }
        boolean show = attachmentPanelView.getVisibility() != View.VISIBLE;
        attachmentPanelView.setVisibility(show ? View.VISIBLE : View.GONE);
        if (show) {
            hideKeyboard();
        }
    }

    private void hideAttachmentPanel() {
        if (attachmentPanelView != null) {
            attachmentPanelView.setVisibility(View.GONE);
        }
    }

    private void renderAttachmentPanel() {
        if (attachmentPanelView == null) {
            return;
        }
        attachmentPanelView.removeAllViews();
        attachmentPanelView.addView(attachmentEntry("相册", "图片", v -> pickImage()), new LinearLayout.LayoutParams(dp(86), dp(70)));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(dp(86), dp(70));
        params.leftMargin = dp(10);
        attachmentPanelView.addView(attachmentEntry("文件", "文档", v -> pickFile()), params);
    }

    private View attachmentEntry(String title, String subtitle, View.OnClickListener listener) {
        LinearLayout entry = new LinearLayout(this);
        entry.setOrientation(LinearLayout.VERTICAL);
        entry.setGravity(Gravity.CENTER);
        entry.setBackground(rounded(Color.WHITE, dp(14)));
        entry.setOnClickListener(listener);
        TextView name = strong(title);
        name.setGravity(Gravity.CENTER);
        entry.addView(name);
        TextView sub = muted(subtitle);
        sub.setGravity(Gravity.CENTER);
        entry.addView(sub);
        return entry;
    }

    private void renderAttachmentBuffer() {
        if (attachmentBufferView == null) {
            return;
        }
        attachmentBufferView.removeAllViews();
        List<PendingAttachment> attachments = currentPendingAttachments();
        if (attachments.isEmpty()) {
            attachmentBufferView.setVisibility(View.GONE);
            return;
        }
        attachmentBufferView.setVisibility(View.VISIBLE);
        for (PendingAttachment attachment : attachments) {
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(dp(76), dp(76));
            params.rightMargin = dp(10);
            attachmentBufferView.addView(attachmentPreview(attachment), params);
        }
        LinearLayout.LayoutParams addParams = new LinearLayout.LayoutParams(dp(76), dp(76));
        attachmentBufferView.addView(addMoreAttachmentView(), addParams);
    }

    private View attachmentPreview(PendingAttachment attachment) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(rounded(Color.WHITE, dp(12)));
        if ("image".equals(attachment.type)) {
            ImageView image = new ImageView(this);
            image.setScaleType(ImageView.ScaleType.CENTER_CROP);
            image.setImageURI(attachment.uri);
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
        } else {
            TextView file = muted(shortFileName(attachment.name));
            file.setGravity(Gravity.CENTER);
            file.setPadding(dp(6), 0, dp(6), 0);
            frame.addView(file, new FrameLayout.LayoutParams(-1, -1));
        }
        TextView close = new TextView(this);
        close.setText("×");
        close.setTextSize(18);
        close.setGravity(Gravity.CENTER);
        close.setTextColor(Color.WHITE);
        close.setBackground(rounded(Color.rgb(80, 80, 80), dp(14)));
        close.setOnClickListener(v -> {
            currentPendingAttachments().remove(attachment);
            renderAttachmentBuffer();
            updateInputActionButtonState();
        });
        FrameLayout.LayoutParams closeParams = new FrameLayout.LayoutParams(dp(28), dp(28), Gravity.RIGHT | Gravity.TOP);
        frame.addView(close, closeParams);
        return frame;
    }

    private View addMoreAttachmentView() {
        TextView add = new TextView(this);
        add.setText("+");
        add.setTextSize(28);
        add.setGravity(Gravity.CENTER);
        add.setTextColor(Color.rgb(75, 85, 99));
        add.setBackground(rounded(Color.WHITE, dp(12)));
        add.setOnClickListener(v -> {
            hideKeyboard();
            if (attachmentPanelView != null) {
                attachmentPanelView.setVisibility(View.VISIBLE);
            }
        });
        return add;
    }

    private void pickImage() {
        hideAttachmentPanel();
        Intent intent;
        if (Build.VERSION.SDK_INT >= 33) {
            intent = new Intent(MediaStore.ACTION_PICK_IMAGES);
        } else {
            intent = new Intent(Intent.ACTION_PICK, MediaStore.Images.Media.EXTERNAL_CONTENT_URI);
            intent.setType("image/*");
        }
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        try {
            startActivityForResult(intent, REQUEST_PICK_IMAGE);
        } catch (ActivityNotFoundException error) {
            Intent fallback = new Intent(Intent.ACTION_GET_CONTENT);
            fallback.setType("image/*");
            fallback.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
            startActivityForResult(fallback, REQUEST_PICK_IMAGE);
        }
    }

    private void pickFile() {
        hideAttachmentPanel();
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
        intent.setType("*/*");
        intent.putExtra(Intent.EXTRA_MIME_TYPES, new String[]{
                "application/pdf"
        });
        intent.putExtra("android.provider.extra.INITIAL_URI", Uri.parse("content://com.android.externalstorage.documents/root/primary"));
        intent.putExtra("android.provider.extra.SHOW_ADVANCED", true);
        startActivityForResult(intent, REQUEST_PICK_FILE);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == REQUEST_SPEECH_INPUT) {
            if (resultCode == RESULT_OK && data != null) {
                ArrayList<String> texts = data.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS);
                if (texts != null && !texts.isEmpty()) {
                    fillRecognizedSpeechToInput(texts.get(0));
                } else {
                    toastLine("没有识别到内容");
                    finishVoiceMode();
                }
            } else {
                finishVoiceMode();
            }
            return;
        }
        if (resultCode != RESULT_OK || data == null || data.getData() == null) {
            return;
        }
        Uri uri = data.getData();
        try {
            getContentResolver().takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION);
        } catch (Exception ignored) {
        }
        PendingAttachment attachment = attachmentController.fromUri(getContentResolver(), uri, requestCode == REQUEST_PICK_IMAGE);
        currentPendingAttachments().add(attachment);
        renderAttachmentBuffer();
        hideAttachmentPanel();
        updateInputActionButtonState();
    }

    private String shortFileName(String name) {
        if (name == null || name.isEmpty()) {
            return "文件";
        }
        return name.length() <= 8 ? name : name.substring(0, 6) + "…";
    }

    private void enterVoiceMode() {
        if (checkSelfPermission(Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.RECORD_AUDIO}, REQUEST_RECORD_AUDIO);
            return;
        }
        SpeechMode mode = currentSpeechMode();
        if (mode == SpeechMode.XUNFEI_REALTIME || mode == SpeechMode.AUTO) {
            startRealtimeSpeech();
            return;
        }
        if (mode == SpeechMode.ANDROID_INLINE) {
            startAndroidInlineSpeech();
            return;
        }
        startSpeechRecognizerActivity();
    }

    private SpeechMode currentSpeechMode() {
        return voiceController.currentMode();
    }

    private void startRealtimeSpeech() {
        voiceMode = true;
        voiceResultSent = false;
        if (input != null) {
            input.setText("");
            input.setHint("正在连接语音识别...");
            input.clearFocus();
        }
        hideKeyboard();
        updateInputActionButtonState();
        voiceController.startRealtimeSpeech(sessionStore, realtimeSpeechListener());
    }

    private void startAndroidInlineSpeech() {
        if (!voiceController.isInlineRecognitionAvailable(this)) {
            startSpeechRecognizerActivity();
            return;
        }
        voiceMode = true;
        voiceResultSent = false;
        if (input != null) {
            input.setText("");
            input.setHint("正在聆听...");
            input.clearFocus();
        }
        hideKeyboard();
        updateInputActionButtonState();
        startSpeechRecognition();
    }

    private void startSpeechRecognizerActivity() {
        Intent intent = voiceController.recognitionIntent(false);
        if (intent.resolveActivity(getPackageManager()) == null) {
            toastLine("当前设备不支持语音输入");
            finishVoiceMode();
            return;
        }
        voiceMode = true;
        voiceResultSent = false;
        if (input != null) {
            input.setText("");
            input.setHint("正在聆听...");
            input.clearFocus();
        }
        hideKeyboard();
        updateInputActionButtonState();
        startActivityForResult(intent, REQUEST_SPEECH_INPUT);
    }

    private void exitVoiceMode() {
        if (voiceController.isRealtimeActive()) {
            voiceController.finishRealtimeSpeech(true, realtimeSpeechListener());
            return;
        }
        voiceResultSent = true;
        stopSpeechRecognition();
        voiceMode = false;
        if (input != null) {
            input.setFocusableInTouchMode(true);
            input.setFocusable(true);
            input.setGravity(Gravity.CENTER_VERTICAL);
            input.setHint("输入问题或直接发送...");
        }
        updateInputActionButtonState();
    }

    private VoiceInputController.RealtimeSpeechListener realtimeSpeechListener() {
        return new VoiceInputController.RealtimeSpeechListener() {
            @Override
            public void onReady() {
                if (input != null) {
                    input.setHint("正在聆听...");
                }
            }

            @Override
            public void onPartial(String text) {
                String value = text == null ? "" : text.trim();
                if (input != null && voiceController.isRealtimeActive() && !value.isEmpty()) {
                    input.setHint(value);
                }
            }

            @Override
            public void onRecognizing() {
                if (input != null) {
                    input.setHint("正在识别语音...");
                }
                updateInputActionButtonState();
            }

            @Override
            public void onFinal(String text) {
                fillRecognizedSpeechToInput(text);
            }

            @Override
            public void onError(String code, String message) {
                if (currentSpeechMode() == SpeechMode.AUTO && shouldFallbackRealtimeSpeech(code)) {
                    Log.w(TAG, "Realtime speech failed, fallback to Android speech. code=" + code + " message=" + message);
                    startAndroidInlineSpeech();
                    return;
                }
                if ("speech_not_enabled".equals(code) || "speech_network_error".equals(code)) {
                    toastLine("当前语音识别不可用");
                    return;
                }
                toastLine((message == null || message.trim().isEmpty()) ? "语音识别失败，请重试" : message);
            }

            @Override
            public void onStopped() {
                voiceMode = false;
                if (input != null) {
                    input.setHint("输入问题或直接发送...");
                    input.setGravity(Gravity.CENTER_VERTICAL);
                }
                updateInputActionButtonState();
            }
        };
    }

    private boolean shouldFallbackRealtimeSpeech(String code) {
        String value = code == null ? "" : code.trim();
        return value.isEmpty()
                || "speech_not_enabled".equals(value)
                || "speech_network_error".equals(value)
                || "speech_recognition_failed".equals(value);
    }

    private void startSpeechRecognition() {
        voiceController.startInlineRecognition(this, new VoiceInputController.AndroidSpeechListener() {
            @Override
            public void onRecognized(String text) {
                sendRecognizedSpeech(text);
            }

            @Override
            public void onError(String message) {
                toastLine(message);
                finishVoiceMode();
            }
        });
    }

    private void sendRecognizedSpeech(String text) {
        String value = text == null ? "" : text.trim();
        if (value.isEmpty() || voiceResultSent) {
            return;
        }
        voiceResultSent = true;
        fillRecognizedSpeechToInput(value);
    }

    private void fillRecognizedSpeechToInput(String text) {
        String value = text == null ? "" : text.trim();
        if (value.isEmpty()) {
            toastLine("没有识别到内容");
            finishVoiceMode();
            return;
        }
        voiceResultSent = true;
        voiceMode = false;
        stopSpeechRecognition();
        if (input != null) {
            input.setText(value);
            input.setSelection(value.length());
            input.setHint("输入问题或直接发送...");
            input.setGravity(Gravity.CENTER_VERTICAL);
            input.setFocusableInTouchMode(true);
            input.setFocusable(true);
            input.requestFocus();
            showKeyboard();
        }
        updateInputActionButtonState();
    }

    private void finishVoiceMode() {
        voiceMode = false;
        if (input != null) {
            input.setHint("输入问题或直接发送...");
            input.setGravity(Gravity.CENTER_VERTICAL);
        }
        updateInputActionButtonState();
    }

    private void stopSpeechRecognition() {
        voiceController.stopRecognizer();
    }

    @Override
    public void onRequestPermissionsResult(int requestCode, String[] permissions, int[] grantResults) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults);
        if (requestCode == REQUEST_RECORD_AUDIO) {
            if (grantResults.length > 0 && grantResults[0] == PackageManager.PERMISSION_GRANTED) {
                enterVoiceMode();
            } else {
                toastLine("需要麦克风权限才能使用语音输入");
                finishVoiceMode();
            }
        }
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
            welcome.setText(greetingText());
        }
        LinearLayout suggestions = findTaggedLayout(chatList, "suggestions");
        if (suggestions == null) {
            return;
        }
        renderHomeSuggestions(suggestions, home == null ? null : home.optJSONArray("suggestions"));
    }

    private String greetingText() {
        String greeting = greetingPrefix();
        if (sessionStore == null || sessionStore.token().isEmpty()) {
            return greeting;
        }
        return greeting + "，" + displayNickname();
    }

    private String greetingPrefix() {
        Calendar calendar = Calendar.getInstance();
        int hour = calendar.get(Calendar.HOUR_OF_DAY);
        if (hour >= 5 && hour < 11) {
            return "早上好";
        }
        if (hour >= 11 && hour < 14) {
            return "中午好";
        }
        if (hour >= 14 && hour < 24) {
            return "晚上好";
        }
        return "夜深了";
    }

    private String displayNickname() {
        String nickname = sessionStore == null ? "" : sessionStore.nickname();
        return nickname == null || nickname.trim().isEmpty() ? "用户名" : nickname.trim();
    }

    private void renderHomeSuggestions(LinearLayout suggestions, JSONArray remoteItems) {
        if (suggestions == null) {
            return;
        }
        suggestions.removeAllViews();
        List<String> items = randomHomePrompts(remoteItems);
        for (String text : items) {
            Button chip = secondaryButton(text);
            chip.setTextSize(14);
            chip.setGravity(Gravity.CENTER);
            chip.setPadding(dp(14), 0, dp(14), 0);
            chip.setOnClickListener(v -> {
                chatAutoScrollEnabled = true;
                userDetachedFromBottom = false;
                sendMessage(((Button) v).getText().toString());
            });
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-2, dp(44));
            params.gravity = Gravity.CENTER_HORIZONTAL;
            params.setMargins(0, 0, 0, dp(10));
            suggestions.addView(chip, params);
        }
    }

    private List<String> randomHomePrompts(JSONArray remoteItems) {
        LinkedHashSet<String> pool = new LinkedHashSet<>();
        if (remoteItems != null) {
            for (int i = 0; i < remoteItems.length(); i++) {
                JSONObject item = remoteItems.optJSONObject(i);
                String text = item == null ? remoteItems.optString(i, "") : item.optString("text", "");
                if (text != null && !text.trim().isEmpty()) {
                    pool.add(text.trim());
                }
            }
        }
        Collections.addAll(pool, DEFAULT_HOME_PROMPTS);
        List<String> prompts = new ArrayList<>(pool);
        if (prompts.size() <= 3) {
            return prompts;
        }
        long seed = System.currentTimeMillis();
        if (localSessionId != null) {
            seed ^= localSessionId.hashCode();
        }
        Collections.shuffle(prompts, new Random(seed));
        return new ArrayList<>(prompts.subList(0, 3));
    }

    private void updateChatAutoScrollFromPosition() {
        if (isChatScrolledToBottom()) {
            chatAutoScrollEnabled = true;
            userDetachedFromBottom = false;
        }
    }

    private void forceScrollBottom() {
        if (chatScroll != null) {
            chatScroll.post(() -> chatScroll.fullScroll(View.FOCUS_DOWN));
        }
    }

    private void scrollBottomIfAllowed() {
        if (!streamController.isStreaming() || chatAutoScrollEnabled || !userDetachedFromBottom) {
            forceScrollBottom();
        }
    }

    private String trimDanglingProductPunctuation(String text) {
        if (text == null) {
            return "";
        }
        return text.replaceAll("[\\s，。、、,.;；:：]+$", "");
    }

    private boolean isOnlyDanglingProductPunctuation(String text) {
        return text != null && text.trim().matches("[，。、、,.;；:：]+");
    }

    private void removeDanglingPunctuationFromActiveAssistant() {
        if (activeAssistant == null || activeAssistantMarkdown == null) {
            return;
        }
        String trimmed = trimDanglingProductPunctuation(activeAssistantMarkdown.toString());
        if (trimmed.equals(activeAssistantMarkdown.toString())) {
            return;
        }
        activeAssistantMarkdown = new StringBuilder(trimmed);
        RenderedMarkdown rendered = sanitizeAgentMarkdown(trimmed);
        MarkdownRenderer.setMarkdown(activeAssistant, rendered.visibleMarkdown);
    }

    private void removeDanglingPunctuationFromHistoricalBubble(LinearLayout bubble) {
        if (bubble == null || bubble.getChildCount() == 0) {
            return;
        }
        View last = bubble.getChildAt(bubble.getChildCount() - 1);
        if (!(last instanceof TextView)) {
            return;
        }
        TextView textView = (TextView) last;
        String trimmed = trimDanglingProductPunctuation(textView.getText().toString());
        if (trimmed.trim().isEmpty()) {
            bubble.removeView(last);
            return;
        }
        if (!trimmed.equals(textView.getText().toString())) {
            MarkdownRenderer.setMarkdown(textView, sanitizeAgentMarkdown(trimmed).visibleMarkdown);
        }
    }

    private int currentTopSafeInset() {
        if (!manualWindowInsets) {
            return 0;
        }
        return stableTopInset(appliedTopInset < 0 ? 0 : appliedTopInset);
    }

    private int currentBottomSafeInset() {
        if (!manualWindowInsets) {
            return 0;
        }
        return Math.max(0, appliedBottomInset);
    }

    private LinearLayout makeBottomSheetBox() {
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setPadding(dp(20), dp(18), dp(20), dp(16));
        box.setBackground(rounded(Color.WHITE, dp(22)));
        return box;
    }

    private AlertDialog showBottomSheetDialog(View body) {
        return showBottomSheetDialog(body, true);
    }

    private AlertDialog showBottomSheetDialog(View body, boolean animated) {
        return BottomSheetHelper.show(this, body, true, animated, currentBottomSafeInset());
    }

    private void showConfirmBottomSheet(String titleText, String message, String confirmText, boolean danger, Runnable onConfirm) {
        showConfirmBottomSheet(titleText, message, confirmText, danger, true, onConfirm);
    }

    private void showConfirmBottomSheet(String titleText, String message, String confirmText, boolean danger, boolean animated, Runnable onConfirm) {
        LinearLayout box = makeBottomSheetBox();
        TextView titleView = title(titleText);
        titleView.setTextSize(20);
        box.addView(titleView, new LinearLayout.LayoutParams(-1, -2));
        TextView messageView = muted(message);
        messageView.setPadding(0, dp(8), 0, dp(18));
        box.addView(messageView, new LinearLayout.LayoutParams(-1, -2));
        final AlertDialog[] dialogRef = new AlertDialog[1];
        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        Button cancel = secondaryButton("取消");
        cancel.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
        });
        Button confirm = primaryButton(confirmText);
        if (danger) {
            confirm.setBackground(rounded(Color.rgb(220, 38, 38), dp(24)));
        }
        confirm.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
            if (onConfirm != null) {
                onConfirm.run();
            }
        });
        actions.addView(cancel, new LinearLayout.LayoutParams(0, dp(48), 1));
        LinearLayout.LayoutParams confirmParams = new LinearLayout.LayoutParams(0, dp(48), 1);
        confirmParams.leftMargin = dp(10);
        actions.addView(confirm, confirmParams);
        box.addView(actions, new LinearLayout.LayoutParams(-1, -2));
        dialogRef[0] = showBottomSheetDialog(box, animated);
    }

    private void showDrawer() {
        useKeyboardOverlay();
        hideKeyboard();
        if (sessionStore.token().isEmpty()) {
            openLoginPage();
            return;
        }
        if (drawerLayer != null) {
            return;
        }
        drawerLayer = new FrameLayout(this);
        drawerLayer.setBackgroundColor(Color.argb(92, 0, 0, 0));
        drawerLayer.setAlpha(0f);

        LinearLayout drawer = new LinearLayout(this);
        drawerPanel = drawer;
        drawer.setOrientation(LinearLayout.VERTICAL);
        drawer.setPadding(dp(14), dp(10) + currentTopSafeInset(), dp(14), dp(10));
        drawer.setBackgroundColor(Color.WHITE);

        LinearLayout top = new LinearLayout(this);
        top.setGravity(Gravity.CENTER_VERTICAL);
        EditText search = new EditText(this);
        search.setHint("历史会话搜索");
        search.setTextSize(15);
        search.setTextColor(Color.rgb(156, 163, 175));
        search.setHintTextColor(Color.rgb(156, 163, 175));
        search.setSingleLine(true);
        search.setImeOptions(EditorInfo.IME_ACTION_DONE);
        search.setGravity(Gravity.CENTER_VERTICAL);
        search.setPadding(dp(16), 0, dp(16), 0);
        search.setBackground(rounded(Color.rgb(246, 247, 249), dp(12)));
        top.addView(search, new LinearLayout.LayoutParams(0, dp(44), 1));
        Button create = transparentIconButton("✎");
        create.setTextSize(24);
        create.setOnClickListener(v -> {
            persistAndCancelActiveStreamForNavigation();
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
        navGroup.addView(drawerNavButton("💬", "AI导购", "chat", v -> renderChatHome()));
        navGroup.addView(drawerNavButton("🏷", "商品", "products", v -> {
            persistAndCancelActiveStreamForNavigation();
            startActivity(new Intent(this, ProductListActivity.class));
        }));
        navGroup.addView(drawerNavButton("🛒", "购物车", "cart", v -> {
            persistAndCancelActiveStreamForNavigation();
            startActivity(new Intent(this, CartActivity.class));
        }));
        navGroup.addView(drawerNavButton("📦", "订单", "orders", v -> {
            persistAndCancelActiveStreamForNavigation();
            startActivity(new Intent(this, OrderListActivity.class));
        }));
        drawer.addView(navGroup);

        View divider = new View(this);
        divider.setBackgroundColor(Color.rgb(238, 239, 242));
        drawer.addView(divider, new LinearLayout.LayoutParams(-1, 1));

        FrameLayout historyFrame = new FrameLayout(this);
        historyFrame.setClickable(true);
        ScrollView historyScroll = new ScrollView(this);
        historyScroll.setFillViewport(true);
        historyScroll.setClickable(true);
        LinearLayout historyList = new LinearLayout(this);
        historyList.setOrientation(LinearLayout.VERTICAL);
        historyList.setClickable(true);
        drawerHistoryList = historyList;
        drawerSearchInput = search;
        drawerHistoryScroll = historyScroll;
        historyDrawer.setLocallyMutated(false);
        int historyBottomPadding = sessionStore.showDrawerReturnChat() ? dp(72) : dp(16);
        historyList.setPadding(0, dp(12), 0, historyBottomPadding);
        String initialHistoryQuery = historyDrawer.initialQuery();
        if (!initialHistoryQuery.isEmpty()) {
            search.setText(initialHistoryQuery);
        }
        historyDrawer.resetPaging(initialHistoryQuery);
        renderInitialHistoryList(historyList, initialHistoryQuery);
        historyScroll.postDelayed(() -> loadRemoteHistoryPage(initialHistoryQuery, 0, false), 260);
        final int[] searchVersion = {0};
        drawerLayer.setOnClickListener(v -> closeDrawerAnimated());
        drawer.setOnClickListener(v -> {});
        search.setOnEditorActionListener((v, actionId, event) -> {
            if (actionId == EditorInfo.IME_ACTION_DONE) {
                hideKeyboardFrom(search);
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
                historyDrawer.resetPaging(query);
                renderInitialHistoryList(historyList, query);
                new Thread(() -> {
                    try {
                        Thread.sleep(300);
                        if (version != searchVersion[0]) {
                            return;
                        }
                        runOnUiThread(() -> {
                            String currentQuery = search.getText().toString().trim();
                            if (version == searchVersion[0] && query.equals(currentQuery)) {
                                loadRemoteHistoryPage(query, 0, false);
                            }
                        });
                    } catch (Exception error) {
                        runOnUiThread(() -> {
                            String currentQuery = search.getText().toString().trim();
                            if (version == searchVersion[0] && query.equals(currentQuery)) {
                                toastLine("搜索失败，请重试");
                            }
                        });
                    }
                }).start();
            }
        });
        historyScroll.getViewTreeObserver().addOnScrollChangedListener(() -> maybeLoadMoreHistory(historyList, historyScroll));
        historyScroll.addView(historyList);
        historyFrame.addView(historyScroll, new FrameLayout.LayoutParams(-1, -1));

        if (sessionStore.showDrawerReturnChat()) {
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
        }
        drawer.addView(historyFrame, new LinearLayout.LayoutParams(-1, 0, 1));

        View bottomDivider = new View(this);
        bottomDivider.setBackgroundColor(Color.rgb(238, 239, 242));
        drawer.addView(bottomDivider, new LinearLayout.LayoutParams(-1, 1));
        View bottomBar = bottomUserBar();
        drawer.addView(bottomBar, new LinearLayout.LayoutParams(-1, dp(68)));

        int panelWidth = (int) (getResources().getDisplayMetrics().widthPixels * 0.82f);
        FrameLayout.LayoutParams drawerParams = new FrameLayout.LayoutParams(panelWidth, -1, Gravity.LEFT | Gravity.TOP);
        drawer.setTranslationX(-panelWidth);
        drawer.setLayerType(View.LAYER_TYPE_HARDWARE, null);
        drawerLayer.addView(drawer, drawerParams);
        root.addView(drawerLayer, new FrameLayout.LayoutParams(-1, -1));
        drawerLayer.animate().alpha(1f).setDuration(160).start();
        drawer.animate()
                .translationX(0f)
                .setDuration(220)
                .withEndAction(() -> drawer.setLayerType(View.LAYER_TYPE_NONE, null))
                .start();
    }

    private void syncRemoteSessions(Runnable onDone) {
        if (sessionStore.token().isEmpty()) {
            if (onDone != null) {
                runOnUiThread(onDone);
            }
            return;
        }
        new Thread(() -> {
            try {
                JSONArray sessions = api.sessionsPage(1, ChatHistoryDrawer.PAGE_SIZE).items;
                for (int i = 0; i < sessions.length(); i++) {
                    JSONObject item = sessions.optJSONObject(i);
                    if (item == null) {
                        continue;
                    }
                    chatStore.upsertRemoteSession(item.optString("session_id", ""), item.optString("title", "导购会话"), item.optString("summary", ""), ChatHistoryDrawer.remoteSessionTime(item));
                }
            } catch (Exception error) {
                runOnUiThread(() -> handleApiError(error));
            } finally {
                if (onDone != null) {
                    runOnUiThread(onDone);
                }
            }
        }).start();
    }

    private void validateStoredToken() {
        if (sessionStore.token().isEmpty()) {
            return;
        }
        new Thread(() -> {
            try {
                JSONObject account = api.me();
                runOnUiThread(() -> saveAccountSession(sessionStore.token(), account, sessionStore.role(), sessionStore.nickname()));
            } catch (Exception error) {
                if (isAuthExpired(error)) {
                    runOnUiThread(() -> handleAuthExpired());
                }
            }
        }).start();
    }

    private boolean isAuthExpired(Throwable error) {
        return error instanceof ApiClient.ApiException && ((ApiClient.ApiException) error).statusCode == 401;
    }

    private boolean handleApiError(Throwable error) {
        if (!isAuthExpired(error)) {
            return false;
        }
        runOnUiThread(this::handleAuthExpired);
        return true;
    }

    private void handleAuthExpired() {
        if (sessionStore.token().isEmpty()) {
            return;
        }
        streamController.cancelActiveCall();
        sessionStore.clearAuth();
        serverSessionId = "";
        toastLine("登录已过期，请重新登录");
        if ("cart".equals(activePage) || "orders".equals(activePage) || "settings".equals(activePage) || "account".equals(activePage) || "advanced_settings".equals(activePage)) {
            openLoginPage();
        } else if ("chat".equals(activePage)) {
            addChatSystemLine("登录已过期，请重新登录。当前聊天仍保存在本地。");
        } else {
            renderChatHome();
            loadHomeCopy();
        }
    }

    private void renderHistoryList(LinearLayout historyList, List<LocalChatStore.SessionSummary> histories) {
        historyList.removeAllViews();
        if (histories == null || histories.isEmpty()) {
            TextView empty = muted("暂无历史聊天");
            empty.setGravity(Gravity.CENTER);
            historyList.addView(empty, new LinearLayout.LayoutParams(-1, dp(56)));
            return;
        }
        appendHistoryList(historyList, histories);
    }

    private void appendHistoryList(LinearLayout historyList, List<LocalChatStore.SessionSummary> histories) {
        if (historyList == null || histories == null || histories.isEmpty()) {
            return;
        }
        Set<String> existingIds = existingHistoryIds(historyList);
        for (LocalChatStore.SessionSummary item : histories) {
            if (item != null && existingIds.add(item.localSessionId)) {
                historyList.addView(historyButton(item));
            }
        }
    }

    private Set<String> existingHistoryIds(LinearLayout historyList) {
        HashSet<String> ids = new HashSet<>();
        if (historyList == null) {
            return ids;
        }
        for (int i = 0; i < historyList.getChildCount(); i++) {
            Object tag = historyList.getChildAt(i).getTag();
            if (tag instanceof String && ((String) tag).startsWith("history:")) {
                ids.add(((String) tag).substring("history:".length()));
            }
        }
        return ids;
    }

    private void renderInitialHistoryList(LinearLayout historyList, String query) {
        String keyword = query == null ? "" : query.trim();
        List<LocalChatStore.SessionSummary> histories = historyDrawer.initialPage(keyword);
        renderHistoryList(historyList, histories);
        if (historyDrawer.shouldRestoreSavedState(keyword)) {
            restoreSavedHistoryScroll();
        }
    }

    private void loadRemoteHistoryPage(String query, int offset, boolean append) {
        if (drawerHistoryList == null) {
            return;
        }
        String requestedQuery = query == null ? "" : query.trim();
        if (sessionStore.token().isEmpty()) {
            if (append) {
                List<LocalChatStore.SessionSummary> page = historyDrawer.loadLocalPage(requestedQuery, offset);
                appendHistoryList(drawerHistoryList, page);
            } else {
                renderInitialHistoryList(drawerHistoryList, requestedQuery);
            }
            return;
        }
        historyDrawer.setLoading(true);
        int page = offset / ChatHistoryDrawer.PAGE_SIZE + 1;
        new Thread(() -> {
            try {
                ApiClient.SessionPage remotePage = requestedQuery.isEmpty()
                        ? api.sessionsPage(page, ChatHistoryDrawer.PAGE_SIZE)
                        : api.searchSessionsPage(requestedQuery, page, ChatHistoryDrawer.PAGE_SIZE);
                List<LocalChatStore.SessionSummary> summaries = historyDrawer.upsertRemoteSearchResults(remotePage.items);
                runOnUiThread(() -> {
                    if (drawerHistoryList == null) {
                        return;
                    }
                    String currentQuery = drawerSearchInput == null ? "" : drawerSearchInput.getText().toString().trim();
                    if (!requestedQuery.equals(currentQuery)) {
                        historyDrawer.setLoading(false);
                        return;
                    }
                    if (append) {
                        historyDrawer.applyRemoteAppend(requestedQuery, offset, remotePage.items.length(), remotePage.hasMore);
                        appendHistoryList(drawerHistoryList, summaries);
                    } else if (historyDrawer.shouldRestoreSavedState(requestedQuery)) {
                        renderInitialHistoryList(drawerHistoryList, requestedQuery);
                        historyDrawer.mergeRemoteHasMore(remotePage.hasMore);
                    } else {
                        historyDrawer.applyRemoteReplace(requestedQuery, offset, remotePage.items.length(), remotePage.hasMore);
                        renderHistoryList(drawerHistoryList, summaries);
                    }
                    historyDrawer.setLoading(false);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (drawerHistoryList != null) {
                        if (append) {
                            List<LocalChatStore.SessionSummary> pageItems = historyDrawer.loadLocalPage(requestedQuery, offset);
                            appendHistoryList(drawerHistoryList, pageItems);
                        } else {
                            renderInitialHistoryList(drawerHistoryList, requestedQuery);
                        }
                    }
                    historyDrawer.setLoading(false);
                    handleApiError(error);
                });
            }
        }).start();
    }

    private void restoreSavedHistoryScroll() {
        if (drawerHistoryScroll == null) {
            return;
        }
        drawerHistoryScroll.post(() -> {
            if (drawerHistoryScroll == null) {
                return;
            }
            int contentHeight = drawerHistoryScroll.getChildCount() > 0
                    ? drawerHistoryScroll.getChildAt(0).getHeight()
                    : (drawerHistoryList == null ? 0 : drawerHistoryList.getHeight());
            int maxScrollY = Math.max(0, contentHeight - drawerHistoryScroll.getHeight());
            drawerHistoryScroll.scrollTo(0, Math.max(0, Math.min(historyDrawer.savedScrollY(), maxScrollY)));
        });
    }

    private void maybeLoadMoreHistory(LinearLayout historyList, ScrollView scrollView) {
        if (historyList == null || scrollView == null || historyDrawer.isLoading() || !historyDrawer.hasMore()) {
            return;
        }
        int range = historyList.getHeight() - scrollView.getHeight();
        if (range <= 0 || scrollView.getScrollY() < range - dp(32)) {
            return;
        }
        historyDrawer.setLoading(true);
        String query = historyDrawer.query();
        int offset = historyDrawer.offset();
        if (sessionStore.token().isEmpty()) {
            List<LocalChatStore.SessionSummary> next = historyDrawer.loadLocalPage(query, offset);
            appendHistoryList(historyList, next);
            historyDrawer.setLoading(false);
        } else {
            loadRemoteHistoryPage(query, offset, true);
        }
    }

    private void renderHistoryStatus(LinearLayout historyList, String message) {
        historyList.removeAllViews();
        TextView status = muted(message);
        status.setGravity(Gravity.CENTER);
        historyList.addView(status, new LinearLayout.LayoutParams(-1, dp(56)));
    }

    private void refreshDrawerContent() {
        refreshDrawerContent(true);
    }

    private void refreshDrawerContent(boolean includeRemote) {
        if (drawerLayer == null || drawerHistoryList == null) {
            return;
        }
        String query = drawerSearchInput == null ? "" : drawerSearchInput.getText().toString().trim();
        historyDrawer.resetPaging(query);
        renderInitialHistoryList(drawerHistoryList, query);
        if (includeRemote) {
            loadRemoteHistoryPage(query, 0, false);
        }
    }

    private void enqueueSessionSync(String localId, String remoteId, String title, String summary) {
        if (sessionStore.token().isEmpty() || localId == null || localId.isEmpty() || remoteId == null || remoteId.isEmpty()) {
            return;
        }
        SessionSyncJob job = new SessionSyncJob(localId, remoteId, title, summary);
        synchronized (sessionSyncLock) {
            SessionSyncJob existing = null;
            for (SessionSyncJob item : sessionSyncQueue) {
                if (localId.equals(item.localSessionId)) {
                    existing = item;
                    break;
                }
            }
            if (existing != null) {
                sessionSyncQueue.remove(existing);
            }
            sessionSyncQueue.addLast(job);
            chatStore.markSessionSyncState(localId, "pending_sync");
            if (!sessionSyncRunning) {
                sessionSyncRunning = true;
                new Thread(this::drainSessionSyncQueue).start();
            }
        }
    }

    private void drainSessionSyncQueue() {
        while (true) {
            SessionSyncJob job;
            synchronized (sessionSyncLock) {
                job = sessionSyncQueue.pollFirst();
                if (job == null) {
                    sessionSyncRunning = false;
                    return;
                }
            }
            try {
                chatStore.markSessionSyncState(job.localSessionId, "syncing");
                api.updateSession(job.serverSessionId, job.title, job.summary);
                chatStore.markSessionSyncState(job.localSessionId, "synced");
            } catch (Exception error) {
                if (job.retryCount < 2) {
                    job.retryCount++;
                    try {
                        Thread.sleep(job.retryCount == 1 ? 2000 : 5000);
                    } catch (InterruptedException ignored) {
                    }
                    synchronized (sessionSyncLock) {
                        sessionSyncQueue.addLast(job);
                    }
                } else {
                    chatStore.markSessionSyncState(job.localSessionId, "failed");
                }
            }
        }
    }

    private void sendMessage(String text) {
        sendMessage(text, new JSONArray());
    }

    private void sendMessage(String text, JSONArray attachments) {
        chatAutoScrollEnabled = true;
        userDetachedFromBottom = false;
        clearWelcomeIfNeeded();
        if (input != null && !voiceMode) {
            input.setText("");
        }
        String messageText = text == null ? "" : text.trim();
        addUserMessageBubble(messageText, attachments);
        chatStore.saveUserMessage(localSessionId, messageText, attachments == null ? "[]" : attachments.toString(), "pending");
        chatStore.touchSession(localSessionId);
        if (sessionStore.token().isEmpty()) {
            TextView assistant = addBubble("请先通过左上角入口完成登录。当前消息已经保存在本地，登录后可以继续使用 AI 导购。", false);
            chatStore.saveMessage(localSessionId, "assistant", assistant.getText().toString(), "local_only");
            toastLine("请先登录后再发送到后端");
            return;
        }
        streamController.start(api, chatStore, localSessionId, serverSessionId, messageText, attachments == null ? new JSONArray() : attachments, agentStreamListener());
        updateInputActionButtonState();
        loadingAssistant = addLoadingBubble();
    }

    private String attachmentSummary(JSONArray attachments) {
        int count = attachments == null ? 0 : attachments.length();
        return count == 0 ? "" : "[已选择 " + count + " 个附件]";
    }

    private AgentStreamController.Listener agentStreamListener() {
        return new AgentStreamController.Listener() {
            @Override
            public void onSessionSyncRequested(String localSessionId, String serverSessionId, String title, String summary) {
                enqueueSessionSync(localSessionId, serverSessionId, title, summary);
            }

            @Override
            public void onServerSessionCreated(String ownerLocalSessionId, String createdServerSessionId) {
                if (ownerLocalSessionId.equals(localSessionId)) {
                    serverSessionId = createdServerSessionId;
                }
            }

            @Override
            public void onStreamStarting(String ownerLocalSessionId, String ownerServerSessionId, String title) {
                prepareAgentStreamUi();
            }

            @Override
            public void onEvent(String ownerLocalSessionId, JSONObject event) {
                handleSse(ownerLocalSessionId, event);
            }

            @Override
            public void onAuthExpired() {
                handleAuthExpired();
            }

            @Override
            public void onCreateSessionFailed(String message, Throwable error) {
                Log.e(TAG, "create session failed", error);
                failStream(message);
            }

            @Override
            public void onStreamFailed(String message, Throwable error) {
                Log.e(TAG, "stream message failed", error);
                failStream(message);
            }
        };
    }

    private void prepareAgentStreamUi() {
        activeAssistantMarkdown = new StringBuilder();
        activeAssistantFullMarkdown = new StringBuilder();
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistantMessageBubble = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        activeAssistantAnswerStarted = false;
        hasReceivedThinkingDelta = false;
        loggedMissingThinkingDelta = false;
        pendingThinkingStatuses.clear();
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        activeRenderedProductIds.clear();
        removeLoadingBubbleIfNeeded();
    }

    private void handleSse(String ownerLocalSessionId, JSONObject event) {
        if (streamController.shouldIgnoreEvent(ownerLocalSessionId, localSessionId)) {
            return;
        }
        String type = event.optString("type");
        if ("message_start".equals(type)) {
            streamController.markRun(event.optString("run_id", ""));
            return;
        }
        if ("duplicate_message".equals(type)) {
            finishDuplicateStream(ownerLocalSessionId, event);
            return;
        }
        if ("status".equals(type)) {
            String statusText = event.optString("text", "正在处理...");
            rememberThinkingStatus(statusText);
            handleThinkingStatus(statusText);
            return;
        }
        if ("thinking_status".equals(type)) {
            String statusText = event.optString("text", event.optString("status", "正在思考..."));
            rememberThinkingStatus(statusText);
            handleThinkingStatus(statusText);
            return;
        }
        if ("thinking_delta".equals(type)) {
            handleThinkingDelta(event);
            return;
        }
        if ("text_delta".equals(type)) {
            logMissingThinkingDeltaIfNeeded(event);
            String delta = event.optString("delta");
            if (delta != null && !delta.isEmpty()) {
                beginAssistantAnswer();
            }
            appendAssistant(delta);
            return;
        }
        if ("content_delta".equals(type)) {
            JSONObject part = event.optJSONObject("part");
            if (part == null) {
                part = event.optJSONObject("block");
            }
            if (part == null) {
                return;
            }
            if (DEBUG_AGENT_BLOCKS) {
                Log.d("AgentBlock", part.toString());
            }
            if ("text".equals(part.optString("type"))) {
                return;
            }
            beginAssistantAnswer();
            flushPendingAssistantForm(false);
            if (isInlineProductBlock(part)) {
                ensureActiveAssistantMessageBubble(chatList);
                flushActiveTextSegment(false);
            } else {
                flushActiveTextSegment(false);
            }
            activeAssistantBlocks.put(part);
            appendBlockSegment(part);
            renderAgentBlock(part, chatList);
            return;
        }
        if ("block_delta".equals(type)) {
            JSONObject block = event.optJSONObject("block");
            JSONObject value = block == null ? event : block;
            if (DEBUG_AGENT_BLOCKS) {
                Log.d("AgentBlock", value.toString());
            }
            beginAssistantAnswer();
            flushPendingAssistantForm(false);
            if (isInlineProductBlock(value)) {
                ensureActiveAssistantMessageBubble(chatList);
                flushActiveTextSegment(false);
            } else {
                flushActiveTextSegment(false);
            }
            activeAssistantBlocks.put(value);
            appendBlockSegment(value);
            renderAgentBlock(value, chatList);
            return;
        }
        if ("followups".equals(type)) {
            JSONArray questions = event.optJSONArray("questions");
            activeFollowups = questions == null ? new JSONArray() : questions;
            renderFollowups(activeFollowups, chatList);
            return;
        }
        if ("message_end".equals(type)) {
            finishStream(ownerLocalSessionId);
            return;
        }
        if ("error".equals(type)) {
            if ("canceled".equals(event.optString("code"))) {
                finishCanceledStream();
            } else if (isRiskControlError(event)) {
                showRiskControlNotice(event.optString("message", "当前账号命中平台风控限制，请稍后再试"));
            } else {
                failStream(event.optString("message", "生成失败"));
            }
        }
    }

    private void updateLoadingStatus(String text) {
        if (loadingAssistant == null) {
            loadingAssistant = addLoadingBubble();
        }
        loadingAssistant.setText(text == null || text.isEmpty() ? "正在处理..." : text);
    }

    private void appendAssistant(String delta) {
        if (delta == null || delta.isEmpty()) {
            return;
        }
        String input = activeAssistantTagPending.toString() + delta;
        activeAssistantTagPending.setLength(0);
        while (!input.isEmpty()) {
            if (activeAssistantFormMode) {
                int end = input.indexOf("</form>");
                if (end >= 0) {
                    activeAssistantFormMarkdown.append(input, 0, end);
                    updateActiveFormCodeBox();
                    renderActiveFormTable(true);
                    activeAssistantFormMode = false;
                    input = input.substring(end + "</form>".length());
                } else {
                    int keep = partialTagSuffixLength(input, "</form>");
                    activeAssistantFormMarkdown.append(input, 0, input.length() - keep);
                    updateActiveFormCodeBox();
                    activeAssistantTagPending.append(input.substring(input.length() - keep));
                    return;
                }
            } else {
                int start = input.indexOf("<form>");
                if (start >= 0) {
                    appendAssistantText(input.substring(0, start));
                    flushActiveTextSegment(false);
                    activeAssistantFormMode = true;
                    activeAssistantFormMarkdown.setLength(0);
                    startActiveFormCodeBox();
                    input = input.substring(start + "<form>".length());
                } else {
                    int keep = partialTagSuffixLength(input, "<form>");
                    appendAssistantText(input.substring(0, input.length() - keep));
                    activeAssistantTagPending.append(input.substring(input.length() - keep));
                    return;
                }
            }
        }
    }

    private void appendAssistantText(String delta) {
        if (delta == null || delta.isEmpty()) {
            return;
        }
        if (activeAssistantImplicitTableMode) {
            if (activeAssistantFullMarkdown == null) {
                activeAssistantFullMarkdown = new StringBuilder();
            }
            activeAssistantFullMarkdown.append(delta);
            activeAssistantFormMarkdown.append(delta);
            updateActiveFormCodeBox();
            return;
        }
        if (activeAssistant == null) {
            LinearLayout bubble = ensureActiveAssistantMessageBubble(chatList);
            enableCopyProvider(bubble, this::activeAssistantCopyMarkdown);
            activeAssistant = assistantBubbleText("");
            enableCopyProvider(activeAssistant, this::activeAssistantCopyMarkdown);
            bubble.addView(activeAssistant, new LinearLayout.LayoutParams(-1, -2));
            activeAssistantMarkdown = new StringBuilder();
        }
        if (activeAssistantMarkdown == null) {
            activeAssistantMarkdown = new StringBuilder(activeAssistant.getText().toString());
        }
        if (activeAssistantFullMarkdown == null) {
            activeAssistantFullMarkdown = new StringBuilder();
        }
        activeAssistantMarkdown.append(delta);
        activeAssistantFullMarkdown.append(delta);
        int tableStart = findMarkdownTableStart(activeAssistantMarkdown.toString());
        if (tableStart >= 0) {
            String current = activeAssistantMarkdown.toString();
            String before = current.substring(0, tableStart);
            String tableAndRest = current.substring(tableStart);
            if (before.trim().isEmpty()) {
                if (activeAssistant != null && activeAssistant.getParent() instanceof ViewGroup) {
                    ((ViewGroup) activeAssistant.getParent()).removeView(activeAssistant);
                }
                activeAssistant = null;
                activeAssistantMarkdown = null;
            } else {
                activeAssistantMarkdown = new StringBuilder(before);
                RenderedMarkdown beforeRendered = sanitizeAgentMarkdown(before);
                MarkdownRenderer.setMarkdown(activeAssistant, beforeRendered.visibleMarkdown);
                renderItemRefs(beforeRendered.itemIds, chatList);
                flushActiveTextSegment(false);
            }
            activeAssistantImplicitTableMode = true;
            activeAssistantFormMarkdown.setLength(0);
            activeAssistantFormMarkdown.append(tableAndRest);
            startActiveFormCodeBox();
            updateActiveFormCodeBox();
            scrollBottom();
            return;
        }
        RenderedMarkdown rendered = sanitizeAgentMarkdown(activeAssistantMarkdown.toString());
        String visibleMarkdown = rendered.itemIds.length() > 0 ? trimDanglingProductPunctuation(rendered.visibleMarkdown) : rendered.visibleMarkdown;
        if (rendered.itemIds.length() > 0) {
            activeAssistantMarkdown = new StringBuilder(visibleMarkdown);
        }
        MarkdownRenderer.setMarkdown(activeAssistant, visibleMarkdown);
        renderItemRefs(rendered.itemIds, chatList);
        scrollBottom();
    }

    private int partialTagSuffixLength(String value, String tag) {
        int max = Math.min(value.length(), tag.length() - 1);
        for (int length = max; length > 0; length--) {
            if (tag.startsWith(value.substring(value.length() - length))) {
                return length;
            }
        }
        return 0;
    }

    private void flushPendingAssistantForm(boolean finish) {
        if (activeAssistantTagPending.length() > 0) {
            String pending = activeAssistantTagPending.toString();
            activeAssistantTagPending.setLength(0);
            if (activeAssistantFormMode) {
                activeAssistantFormMarkdown.append(pending);
                updateActiveFormCodeBox();
            } else {
                appendAssistantText(pending);
            }
        }
        if (finish && activeAssistantFormMode) {
            String raw = activeAssistantFormMarkdown.toString();
            updateActiveFormCodeBox(raw);
            appendTextSegment(raw);
            if (activeAssistantFullMarkdown == null) {
                activeAssistantFullMarkdown = new StringBuilder();
            }
            activeAssistantFullMarkdown.append('\n').append(raw).append('\n');
            activeAssistantFormMarkdown.setLength(0);
            activeAssistantFormMode = false;
        }
        if (activeAssistantImplicitTableMode && (finish || activeAssistantFormMarkdown.toString().trim().length() > 0)) {
            renderActiveFormTable(false);
            activeAssistantImplicitTableMode = false;
        }
    }

    private void renderActiveFormTable(boolean appendFullMarkdown) {
        String markdown = activeAssistantFormMarkdown.toString().trim();
        activeAssistantFormMarkdown.setLength(0);
        removeActiveFormCodeBox();
        if (markdown.isEmpty()) {
            return;
        }
        LinearLayout bubble = ensureActiveAssistantMessageBubble(chatList);
        if (!containsMarkdownTable(markdown)) {
            TextView textView = assistantBubbleText("");
            enableCopy(textView, markdown);
            MarkdownRenderer.setMarkdown(textView, sanitizeAgentMarkdown(markdown).visibleMarkdown);
            bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
            appendTextSegment(markdown);
            appendFormMarkdownToFull(markdown, appendFullMarkdown);
            scrollBottom();
            return;
        }
        for (ChatMarkdownRenderer.Part part : splitMarkdownParts(markdown)) {
            if (part.table) {
                addMarkdownTableToBubble(bubble, part.content);
                appendTableSegment(part.content);
                appendFormMarkdownToFull(part.content, appendFullMarkdown);
            } else if (!part.content.trim().isEmpty()) {
                TextView textView = assistantBubbleText("");
                enableCopy(textView, part.content);
                MarkdownRenderer.setMarkdown(textView, sanitizeAgentMarkdown(part.content).visibleMarkdown);
                bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
                appendTextSegment(part.content);
                appendFormMarkdownToFull(part.content, appendFullMarkdown);
            }
        }
        scrollBottom();
    }

    private void appendFormMarkdownToFull(String markdown, boolean appendFullMarkdown) {
        if (!appendFullMarkdown || markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        if (activeAssistantFullMarkdown == null) {
            activeAssistantFullMarkdown = new StringBuilder();
        }
        activeAssistantFullMarkdown.append('\n').append(markdown).append('\n');
    }

    private void startActiveFormCodeBox() {
        LinearLayout bubble = ensureActiveAssistantMessageBubble(chatList);
        activeAssistantFormCodeBox = new LinearLayout(this);
        activeAssistantFormCodeBox.setOrientation(LinearLayout.VERTICAL);
        activeAssistantFormCodeBox.setPadding(dp(10), dp(8), dp(10), dp(8));
        activeAssistantFormCodeBox.setBackground(rounded(Color.rgb(246, 248, 250), dp(10)));
        activeAssistantFormCodeText = new TextView(this);
        activeAssistantFormCodeText.setText("");
        activeAssistantFormCodeText.setTextSize(13);
        activeAssistantFormCodeText.setTextColor(Color.rgb(31, 41, 55));
        activeAssistantFormCodeText.setTypeface(Typeface.MONOSPACE);
        activeAssistantFormCodeText.setLineSpacing(2, 1);
        activeAssistantFormCodeText.setPadding(0, 0, 0, 0);
        activeAssistantFormCodeBox.addView(activeAssistantFormCodeText, new LinearLayout.LayoutParams(-1, -2));
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, -2);
        params.setMargins(0, dp(8), 0, dp(10));
        bubble.addView(activeAssistantFormCodeBox, params);
        scrollBottom();
    }

    private void updateActiveFormCodeBox() {
        updateActiveFormCodeBox(activeAssistantFormMarkdown == null ? "" : activeAssistantFormMarkdown.toString());
    }

    private void updateActiveFormCodeBox(String rawText) {
        if (activeAssistantFormCodeText == null) {
            startActiveFormCodeBox();
        }
        activeAssistantFormCodeText.setText(rawText == null ? "" : rawText);
        scrollBottom();
    }

    private void removeActiveFormCodeBox() {
        if (activeAssistantFormCodeBox != null && activeAssistantFormCodeBox.getParent() instanceof ViewGroup) {
            ((ViewGroup) activeAssistantFormCodeBox.getParent()).removeView(activeAssistantFormCodeBox);
        }
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
    }

    private void finishStream() {
        finishStream(localSessionId);
    }

    private void finishDuplicateStream(String ownerLocalSessionId, JSONObject event) {
        String activeServerSessionId = streamController.activeServerSessionId();
        flushPendingAssistantForm(true);
        removeLoadingBubbleIfNeeded();
        activeAssistant = null;
        activeAssistantMessageBubble = null;
        activeAssistantMarkdown = null;
        activeAssistantFullMarkdown = null;
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        hasReceivedThinkingDelta = false;
        loggedMissingThinkingDelta = false;
        pendingThinkingStatuses.clear();
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        streamController.reset();
        updateInputActionButtonState();
        toastLine("消息已提交过，正在刷新会话");
        if (!activeServerSessionId.isEmpty()) {
            loadRemoteSessionDetail(activeServerSessionId, ownerLocalSessionId);
        }
    }

    private void finishStream(String ownerLocalSessionId) {
        boolean hadUnclosedForm = activeAssistantFormMode;
        flushPendingAssistantForm(true);
        flushActiveTextSegment(!hadUnclosedForm);
        completeThinkingView();
        String markdown = activeAssistantFullMarkdown == null ? "" : activeAssistantFullMarkdown.toString();
        RenderedMarkdown rendered = sanitizeAgentMarkdown(markdown);
        String visibleMarkdown = rendered.visibleMarkdown;
        if (activeAssistant != null && activeAssistantMessageBubble == null) {
            MarkdownRenderer.setMarkdown(activeAssistant, visibleMarkdown);
        }
        bindFinishedAssistantCopy(markdown);
        renderItemRefs(rendered.itemIds, chatList);
        saveAssistantTurnForActiveStream(ownerLocalSessionId, visibleMarkdown, activeAssistantBlocks.toString(), activeFollowups.toString(), activeAssistantSegments.toString(), "completed");
        enqueueSessionSync(ownerLocalSessionId, streamController.activeServerSessionId(), streamController.activeTitle(), visibleMarkdown);
        activeAssistantMarkdown = null;
        activeAssistantFullMarkdown = null;
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistant = null;
        activeAssistantMessageBubble = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        removeLoadingBubbleIfNeeded();
        streamController.reset();
        updateInputActionButtonState();
    }

    private void failStream(String message) {
        String ownerLocalSessionId = streamController.ownerLocalOr(localSessionId);
        String ownerServerSessionId = streamController.activeServerSessionId();
        String ownerTitle = streamController.activeTitle();
        if (loadingAssistant != null) {
            loadingAssistant.setText(message);
            loadingAssistant = null;
        } else {
            addChatSystemLine(message);
        }
        enqueueSessionSync(ownerLocalSessionId, ownerServerSessionId, ownerTitle, message);
        activeAssistantMarkdown = null;
        activeAssistantFullMarkdown = null;
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistant = null;
        activeAssistantMessageBubble = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        streamController.reset();
        updateInputActionButtonState();
    }

    private void stopActiveStream() {
        if (!streamController.requestStop(api)) {
            return;
        }
        if (actionButton != null) {
            actionButton.setEnabled(false);
        }
        updateLoadingStatus("正在停止...");
        finishCanceledStream();
    }

    private void finishCanceledStream() {
        String ownerLocalSessionId = streamController.ownerLocalOr(localSessionId);
        String ownerServerSessionId = streamController.activeServerSessionId();
        String ownerTitle = streamController.activeTitle();
        boolean hadUnclosedForm = activeAssistantFormMode;
        flushPendingAssistantForm(true);
        flushActiveTextSegment(!hadUnclosedForm);
        String markdown = activeAssistantFullMarkdown == null ? "" : activeAssistantFullMarkdown.toString();
        RenderedMarkdown rendered = sanitizeAgentMarkdown(markdown);
        String visibleMarkdown = rendered.visibleMarkdown;
        removeLoadingBubbleIfNeeded();
        completeThinkingView();
        if (activeAssistant != null && activeAssistantMessageBubble == null) {
            MarkdownRenderer.setMarkdown(activeAssistant, visibleMarkdown);
        }
        bindFinishedAssistantCopy(markdown);
        saveAssistantTurnForActiveStream(ownerLocalSessionId, visibleMarkdown, activeAssistantBlocks.toString(), activeFollowups.toString(), activeAssistantSegments.toString(), "canceled");
        if (ownerLocalSessionId.equals(localSessionId)) {
            addChatSystemLine("已停止生成");
        }
        enqueueSessionSync(ownerLocalSessionId, ownerServerSessionId, ownerTitle, visibleMarkdown);
        activeAssistantMarkdown = null;
        activeAssistantFullMarkdown = null;
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistant = null;
        activeAssistantMessageBubble = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        streamController.reset();
        if (actionButton != null) {
            actionButton.setEnabled(true);
        }
        updateInputActionButtonState();
    }

    private void saveAssistantTurnForActiveStream(String ownerLocalSessionId, String visibleMarkdown, String blocksJson, String followupsJson, String segmentsJson, String status) {
        if (visibleMarkdown == null) {
            visibleMarkdown = "";
        }
        boolean hasBlocks = blocksJson != null && !"[]".equals(blocksJson.trim()) && !blocksJson.trim().isEmpty();
        boolean hasFollowups = followupsJson != null && !"[]".equals(followupsJson.trim()) && !followupsJson.trim().isEmpty();
        boolean hasSegments = segmentsJson != null && !"[]".equals(segmentsJson.trim()) && !segmentsJson.trim().isEmpty();
        if (visibleMarkdown.trim().isEmpty() && !hasBlocks && !hasFollowups && !hasSegments) {
            return;
        }
        String draftId = activeAssistantDraftMessageId();
        if (draftId.isEmpty()) {
            chatStore.saveAssistantTurn(ownerLocalSessionId, visibleMarkdown, blocksJson, followupsJson, segmentsJson, status);
            return;
        }
        chatStore.upsertAssistantDraft(draftId, ownerLocalSessionId, visibleMarkdown, blocksJson, followupsJson, segmentsJson, status);
    }

    private String activeAssistantDraftMessageId() {
        String requestId = streamController.activeRequestId();
        return requestId.isEmpty() ? "" : "assistant_draft_" + requestId;
    }

    private void persistActiveAssistantDraft(String status) {
        if (!streamController.isStreaming()) {
            return;
        }
        String ownerLocalSessionId = streamController.ownerLocalOr(localSessionId);
        String draftId = activeAssistantDraftMessageId();
        if (ownerLocalSessionId.isEmpty() || draftId.isEmpty()) {
            return;
        }
        boolean hadUnclosedForm = activeAssistantFormMode;
        flushPendingAssistantForm(true);
        flushActiveTextSegment(!hadUnclosedForm);
        if (activeThinking != null) {
            appendThinkingSegment(activeThinking);
            activeThinking.segmentSaved = true;
        }
        String markdown = activeAssistantFullMarkdown == null ? "" : activeAssistantFullMarkdown.toString();
        RenderedMarkdown rendered = sanitizeAgentMarkdown(markdown);
        String visibleMarkdown = rendered.visibleMarkdown;
        if (visibleMarkdown.trim().isEmpty()
                && activeAssistantBlocks.length() == 0
                && activeFollowups.length() == 0
                && activeAssistantSegments.length() == 0) {
            return;
        }
        chatStore.upsertAssistantDraft(
                draftId,
                ownerLocalSessionId,
                visibleMarkdown,
                activeAssistantBlocks.toString(),
                activeFollowups.toString(),
                activeAssistantSegments.toString(),
                status == null || status.isEmpty() ? "partial" : status
        );
        chatStore.touchSession(ownerLocalSessionId);
        String ownerServerSessionId = streamController.activeServerSessionId();
        if (!ownerServerSessionId.isEmpty()) {
            enqueueSessionSync(ownerLocalSessionId, ownerServerSessionId, streamController.activeTitle(), visibleMarkdown);
        }
    }

    private void persistAndCancelActiveStreamForNavigation() {
        if (!streamController.isStreaming()) {
            return;
        }
        persistActiveAssistantDraft("partial");
        streamController.requestStop(api);
        streamController.reset();
        activeAssistantMarkdown = null;
        activeAssistantFullMarkdown = null;
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistant = null;
        activeAssistantMessageBubble = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        removeLoadingBubbleIfNeeded();
        updateInputActionButtonState();
    }

    private boolean isRiskControlError(JSONObject event) {
        if (event == null) {
            return false;
        }
        String code = event.optString("code", "").toLowerCase();
        String message = event.optString("message", "");
        return code.contains("risk")
                || code.contains("policy")
                || code.contains("blocked")
                || code.contains("forbidden")
                || message.contains("风控")
                || message.contains("平台风控限制")
                || message.contains("账号命中");
    }

    private void showRiskControlNotice(String message) {
        String text = message == null || message.trim().isEmpty() ? "当前账号命中平台风控限制，请稍后再试" : message.trim();
        long now = System.currentTimeMillis();
        removeLoadingBubbleIfNeeded();
        if (!text.equals(lastRiskNoticeText) || now - lastRiskNoticeAt > 2000) {
            toastLine(text);
            lastRiskNoticeText = text;
            lastRiskNoticeAt = now;
        }
        streamController.reset();
        activeAssistantMarkdown = null;
        activeAssistantFullMarkdown = null;
        activeAssistantFormMode = false;
        activeAssistantImplicitTableMode = false;
        activeAssistantFormMarkdown = new StringBuilder();
        activeAssistantTagPending = new StringBuilder();
        activeAssistantFormCodeBox = null;
        activeAssistantFormCodeText = null;
        activeAssistant = null;
        activeAssistantMessageBubble = null;
        activeAssistantBlocks = new JSONArray();
        activeAssistantSegments = new JSONArray();
        activeThinking = null;
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        updateInputActionButtonState();
    }

    private String readableErrorMessage(Throwable error) {
        if (error == null) {
            return "未知错误";
        }
        String message = error.getMessage();
        if (message != null && !message.trim().isEmpty()) {
            return message.trim();
        }
        String type = error.getClass().getSimpleName();
        return type == null || type.trim().isEmpty() ? "未知错误" : type;
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

    private void handleThinkingDelta(JSONObject event) {
        if (event == null) {
            return;
        }
        logThinkingEvent(event, "received");
        JSONObject source = parseThinkingEvent(event);
        if (source == null) {
            logThinkingEvent(event, "ignored_missing_payload");
            return;
        }
        String rawStage = source.optString("stage", source.optString("id", ""));
        if (rawStage.trim().isEmpty()) {
            logThinkingEvent(event, "ignored_missing_stage");
            return;
        }
        hasReceivedThinkingDelta = true;
        removeLoadingBubbleIfNeeded();
        ThinkingViewState thinking = ensureThinkingView(chatList, true);
        String stageKey = thinkingStageKey(rawStage);
        ThinkingStageState stage = ensureThinkingStage(thinking, stageKey);
        activateThinkingStage(thinking, stageKey);
        String title = source.optString("title", "");
        if (!title.trim().isEmpty()) {
            stage.title = thinkingStageTitle(stageKey, title);
        }
        String status = normalizeThinkingStatus(source.optString("status", ""));
        if (!status.trim().isEmpty()) {
            stage.status = status;
        } else if ("pending".equals(stage.status)) {
            stage.status = "running";
        }
        String delta = thinkingStageTextFromEvent(source);
        if ("answer_summary".equals(stageKey)) {
            stage.text.setLength(0);
            stage.text.append("总结答案完成");
        } else if (!delta.trim().isEmpty()) {
            if (stage.text.length() > 0 && !stage.text.toString().endsWith(delta.trim())) {
                stage.text.append('\n');
            }
            if (!stage.text.toString().endsWith(delta.trim())) {
                stage.text.append(delta.trim());
            }
        }
        JSONArray items = source.optJSONArray("items");
        if (items == null) {
            items = source.optJSONArray("products");
        }
        if (items != null) {
            stage.items = items;
        }
        renderThinkingView(thinking);
        if (activeAssistantAnswerStarted && isThinkingComplete(thinking) && !thinking.completed) {
            completeThinkingView();
        }
    }

    private JSONObject parseThinkingEvent(JSONObject event) {
        if (event == null) {
            return null;
        }
        JSONObject payload = event.optJSONObject("thinking");
        JSONObject step = event.optJSONObject("step");
        JSONObject source = step != null ? step : (payload == null ? event : payload);
        JSONObject parsed = new JSONObject();
        try {
            parsed.put("type", "thinking_delta");
            parsed.put("run_id", source.optString("run_id", event.optString("run_id", streamController.activeRunId())));
            parsed.put("stage", source.optString("stage", source.optString("id", "")));
            parsed.put("status", normalizeThinkingStatus(source.optString("status", "")));
            parsed.put("title", source.optString("title", ""));
            parsed.put("delta", thinkingStageTextFromEvent(source));
            JSONArray items = source.optJSONArray("items");
            if (items == null) {
                items = source.optJSONArray("products");
            }
            if (items != null) {
                parsed.put("items", items);
            }
        } catch (Exception error) {
            Log.w(TAG, "ThinkingEvent parse failed raw=" + event, error);
            return null;
        }
        return parsed;
    }

    private void logThinkingEvent(JSONObject event, String reason) {
        if (event == null) {
            Log.d(TAG, "ThinkingEvent " + reason + " raw=null");
            return;
        }
        JSONObject payload = event.optJSONObject("thinking");
        JSONObject step = event.optJSONObject("step");
        JSONObject source = step != null ? step : (payload == null ? event : payload);
        JSONArray items = source.optJSONArray("items");
        if (items == null) {
            items = source.optJSONArray("products");
        }
        String delta = thinkingStageTextFromEvent(source);
        Log.d(TAG,
                "ThinkingEvent " + reason
                        + " type=" + event.optString("type", "")
                        + " run_id=" + source.optString("run_id", event.optString("run_id", streamController.activeRunId()))
                        + " stage=" + source.optString("stage", source.optString("id", ""))
                        + " status=" + source.optString("status", "")
                        + " title=" + source.optString("title", "")
                        + " delta_empty=" + delta.trim().isEmpty()
                        + " items=" + (items == null ? 0 : items.length())
                        + " raw=" + event);
    }

    private void logMissingThinkingDeltaIfNeeded(JSONObject event) {
        if (hasReceivedThinkingDelta || loggedMissingThinkingDelta) {
            return;
        }
        loggedMissingThinkingDelta = true;
        Log.w(TAG, "ThinkingEvent missing before text_delta run_id=" + streamController.activeRunId() + " raw=" + event);
        startFallbackThinkingTimeline();
    }

    private void rememberThinkingStatus(String text) {
        if (text == null) {
            return;
        }
        String value = text.trim();
        if (value.isEmpty()) {
            return;
        }
        if (pendingThinkingStatuses.isEmpty() || !pendingThinkingStatuses.get(pendingThinkingStatuses.size() - 1).equals(value)) {
            pendingThinkingStatuses.add(value);
        }
    }

    private void handleThinkingStatus(String statusText) {
        String value = statusText == null ? "" : statusText.trim();
        if (value.isEmpty()) {
            value = "正在思考...";
        }
        if (chatList == null) {
            updateLoadingStatus(value);
            return;
        }
        removeLoadingBubbleIfNeeded();
        ThinkingViewState thinking = ensureThinkingView(chatList, true);
        String stageKey = thinkingStageKeyFromStatus(value);
        ThinkingStageState stage = ensureThinkingStage(thinking, stageKey);
        activateThinkingStage(thinking, stageKey);
        if ("pending".equals(stage.status)) {
            stage.status = "running";
        }
        if (stage.text.length() == 0) {
            stage.text.append(value);
        } else if (!stage.text.toString().contains(value)) {
            stage.text.append('\n').append(value);
        }
        renderThinkingView(thinking);
    }

    private String thinkingStageKeyFromStatus(String text) {
        String value = text == null ? "" : text.trim().toLowerCase();
        if (value.contains("买手") || value.contains("经验") || value.contains("查询") || value.contains("检索") || value.contains("商品") || value.contains("知识库") || value.contains("工具")) {
            return "buyer_experience";
        }
        if (value.contains("总结") || value.contains("答案") || value.contains("回答") || value.contains("整理")) {
            return "answer_summary";
        }
        return "user_need";
    }

    private String normalizeThinkingStatus(String status) {
        String value = status == null ? "" : status.trim().toLowerCase();
        if (value.isEmpty()) {
            return "";
        }
        if ("done".equals(value) || "complete".equals(value) || "finished".equals(value) || "success".equals(value)) {
            return "completed";
        }
        if ("doing".equals(value) || "active".equals(value) || "in_progress".equals(value)) {
            return "running";
        }
        return value;
    }

    private void startFallbackThinkingTimeline() {
        if (activeThinking != null || chatList == null) {
            return;
        }
        removeLoadingBubbleIfNeeded();
        ThinkingViewState thinking = ensureThinkingView(chatList, true);
        ThinkingStageState userNeed = ensureThinkingStage(thinking, "user_need");
        userNeed.status = "completed";
        String title = streamController.activeTitle().trim();
        if (!title.isEmpty()) {
            userNeed.text.append("已理解用户需求：").append(title);
        } else {
            userNeed.text.append("已理解用户本轮导购需求。");
        }
        thinking.expanded = true;
        renderThinkingView(thinking);
    }

    private String fallbackBuyerExperienceText() {
        if (pendingThinkingStatuses.isEmpty()) {
            return "已结合商品库、知识库和可用经验信息进行筛选，优先按需求匹配度、价格带和库存状态整理推荐。";
        }
        StringBuilder builder = new StringBuilder("处理过程：");
        for (int i = 0; i < pendingThinkingStatuses.size(); i++) {
            if (i > 0) {
                builder.append('\n');
            }
            builder.append("- ").append(pendingThinkingStatuses.get(i));
        }
        return builder.toString();
    }

    private ThinkingViewState ensureThinkingView(LinearLayout parent, boolean active) {
        if (active && activeThinking != null && activeThinking.container != null) {
            return activeThinking;
        }
        ThinkingViewState thinking = new ThinkingViewState();
        thinking.container = new LinearLayout(this);
        thinking.container.setOrientation(LinearLayout.VERTICAL);
        thinking.container.setPadding(0, dp(6), 0, dp(8));
        thinking.header = new TextView(this);
        thinking.header.setTextSize(15);
        thinking.header.setTextColor(Color.rgb(75, 85, 99));
        thinking.header.setGravity(Gravity.CENTER_VERTICAL);
        thinking.header.setPadding(0, dp(6), 0, dp(6));
        thinking.header.setOnClickListener(v -> {
            int beforeScrollY = chatScroll == null ? 0 : chatScroll.getScrollY();
            thinking.expanded = !thinking.expanded;
            thinking.userToggled = true;
            renderThinkingView(thinking, true, true);
            if (chatScroll != null) {
                chatScroll.post(() -> chatScroll.setScrollY(beforeScrollY));
            }
        });
        thinking.detail = new LinearLayout(this);
        thinking.detail.setOrientation(LinearLayout.VERTICAL);
        thinking.container.addView(thinking.header, new LinearLayout.LayoutParams(-1, -2));
        thinking.container.addView(thinking.detail, new LinearLayout.LayoutParams(-1, -2));
        if (parent != null) {
            parent.addView(thinking.container, new LinearLayout.LayoutParams(-1, -2));
        }
        if (active) {
            activeThinking = thinking;
        }
        return thinking;
    }

    private ThinkingStageState ensureThinkingStage(ThinkingViewState thinking, String key) {
        String stageKey = thinkingStageKey(key);
        ThinkingStageState stage = thinking.stages.get(stageKey);
        if (stage != null) {
            return stage;
        }
        stage = new ThinkingStageState();
        stage.stage = stageKey;
        stage.title = thinkingStageTitle(stageKey, "");
        thinking.stages.put(stageKey, stage);
        return stage;
    }

    private void activateThinkingStage(ThinkingViewState thinking, String stageKey) {
        if (thinking == null) {
            return;
        }
        int activeIndex = thinkingStageIndex(stageKey);
        for (ThinkingStageState existing : thinking.stages.values()) {
            if (existing == null || existing.stage == null) {
                continue;
            }
            int existingIndex = thinkingStageIndex(existing.stage);
            if (activeIndex >= 0 && existingIndex >= 0 && existingIndex < activeIndex && !"failed".equals(existing.status)) {
                existing.status = "completed";
            }
        }
        ThinkingStageState activeStage = ensureThinkingStage(thinking, stageKey);
        if (!"completed".equals(activeStage.status) && !"failed".equals(activeStage.status)) {
            activeStage.status = "running";
        }
    }

    private void renderThinkingView(ThinkingViewState thinking) {
        renderThinkingView(thinking, false, false);
    }

    private void renderThinkingView(ThinkingViewState thinking, boolean preserveScroll) {
        renderThinkingView(thinking, preserveScroll, false);
    }

    private void renderThinkingView(ThinkingViewState thinking, boolean preserveScroll, boolean animateExpansion) {
        if (thinking == null || thinking.header == null || thinking.detail == null) {
            return;
        }
        boolean wasRendered = thinking.hasRendered;
        boolean wasExpanded = wasRendered && thinking.lastRenderedExpanded;
        String headerText = thinking.completed ? "✦  已完成思考 " : "✦  正在思考 ";
        thinking.header.setTextColor(thinking.completed ? Color.rgb(156, 163, 175) : Color.rgb(107, 114, 128));
        thinking.header.setText(headerText + (thinking.expanded ? "收起" : "展开"));
        if (thinking.detailAnimator != null) {
            thinking.detailAnimator.cancel();
            thinking.detailAnimator = null;
        }
        if (thinking.expanded) {
            populateThinkingDetail(thinking);
            boolean shouldAnimate = animateExpansion && wasRendered && !wasExpanded;
            if (shouldAnimate) {
                animateThinkingDetail(thinking, true, preserveScroll);
            } else {
                setThinkingDetailExpandedNow(thinking, true);
                if (!preserveScroll) {
                    scrollBottom();
                }
            }
        } else {
            boolean shouldAnimate = animateExpansion && wasRendered && wasExpanded && thinking.detail.getVisibility() == View.VISIBLE;
            if (shouldAnimate) {
                populateThinkingDetail(thinking);
                animateThinkingDetail(thinking, false, preserveScroll);
            } else {
                setThinkingDetailExpandedNow(thinking, false);
                if (!preserveScroll) {
                    scrollBottom();
                }
            }
        }
        thinking.hasRendered = true;
        thinking.lastRenderedExpanded = thinking.expanded;
    }

    private void populateThinkingDetail(ThinkingViewState thinking) {
        thinking.detail.setVisibility(View.VISIBLE);
        thinking.detail.removeAllViews();
        for (String key : thinkingStageOrder()) {
            ThinkingStageState stage = thinking.stages.get(key);
            if (stage == null) {
                continue;
            }
            ensureThinkingStageViews(stage);
            updateThinkingStageViews(stage);
            LinearLayout.LayoutParams rowParams = new LinearLayout.LayoutParams(-1, -2);
            rowParams.setMargins(0, 0, 0, dp(14));
            thinking.detail.addView(stage.row, rowParams);
        }
    }

    private void setThinkingDetailExpandedNow(ThinkingViewState thinking, boolean expanded) {
        ViewGroup.LayoutParams params = thinking.detail.getLayoutParams();
        if (params == null) {
            params = new LinearLayout.LayoutParams(-1, -2);
        }
        params.height = ViewGroup.LayoutParams.WRAP_CONTENT;
        thinking.detail.setLayoutParams(params);
        thinking.detail.setAlpha(1f);
        if (expanded) {
            thinking.detail.setVisibility(View.VISIBLE);
        } else {
            thinking.detail.setVisibility(View.GONE);
            thinking.detail.removeAllViews();
        }
    }

    private void animateThinkingDetail(ThinkingViewState thinking, boolean expanding, boolean preserveScroll) {
        int startHeight;
        int endHeight;
        if (expanding) {
            startHeight = 0;
            endHeight = measuredThinkingDetailHeight(thinking);
            thinking.detail.setVisibility(View.VISIBLE);
            thinking.detail.setAlpha(0f);
        } else {
            startHeight = thinking.detail.getHeight();
            if (startHeight <= 0) {
                startHeight = measuredThinkingDetailHeight(thinking);
            }
            endHeight = 0;
            thinking.detail.setVisibility(View.VISIBLE);
            thinking.detail.setAlpha(1f);
        }
        if (endHeight <= 0 && expanding) {
            setThinkingDetailExpandedNow(thinking, true);
            return;
        }
        ViewGroup.LayoutParams params = thinking.detail.getLayoutParams();
        if (params == null) {
            params = new LinearLayout.LayoutParams(-1, startHeight);
        }
        params.height = startHeight;
        thinking.detail.setLayoutParams(params);
        ValueAnimator animator = ValueAnimator.ofInt(startHeight, endHeight);
        animator.setDuration(expanding ? 220 : 180);
        animator.setInterpolator(new DecelerateInterpolator());
        animator.addUpdateListener(animation -> {
            int height = (int) animation.getAnimatedValue();
            ViewGroup.LayoutParams layoutParams = thinking.detail.getLayoutParams();
            layoutParams.height = height;
            thinking.detail.setLayoutParams(layoutParams);
            float fraction = animation.getAnimatedFraction();
            thinking.detail.setAlpha(expanding ? fraction : (1f - fraction));
            if (!preserveScroll) {
                scrollBottom();
            }
        });
        animator.addListener(new AnimatorListenerAdapter() {
            @Override
            public void onAnimationEnd(android.animation.Animator animation) {
                if (thinking.detailAnimator != animation) {
                    return;
                }
                thinking.detailAnimator = null;
                setThinkingDetailExpandedNow(thinking, expanding);
                if (!preserveScroll) {
                    scrollBottom();
                }
            }

            @Override
            public void onAnimationCancel(android.animation.Animator animation) {
                if (thinking.detailAnimator == animation) {
                    thinking.detailAnimator = null;
                }
            }
        });
        thinking.detailAnimator = animator;
        animator.start();
    }

    private int measuredThinkingDetailHeight(ThinkingViewState thinking) {
        int width = thinking.detail.getWidth();
        if (width <= 0 && thinking.container != null) {
            width = thinking.container.getWidth();
        }
        if (width <= 0) {
            width = Math.max(1, getResources().getDisplayMetrics().widthPixels - dp(32));
        }
        int widthSpec = View.MeasureSpec.makeMeasureSpec(width, View.MeasureSpec.EXACTLY);
        int heightSpec = View.MeasureSpec.makeMeasureSpec(0, View.MeasureSpec.UNSPECIFIED);
        thinking.detail.measure(widthSpec, heightSpec);
        return thinking.detail.getMeasuredHeight();
    }

    private void ensureThinkingStageViews(ThinkingStageState stage) {
        if (stage.row != null) {
            return;
        }
        stage.row = new LinearLayout(this);
        stage.row.setOrientation(LinearLayout.HORIZONTAL);
        stage.row.setGravity(Gravity.TOP);
        stage.row.setPadding(0, dp(6), 0, dp(10));
        stage.markerView = new TextView(this);
        stage.markerView.setTextSize(14);
        stage.markerView.setGravity(Gravity.CENTER);
        stage.markerView.setIncludeFontPadding(false);
        stage.markerView.setLineSpacing(1, 1);
        stage.markerView.setTextColor(Color.rgb(156, 163, 175));
        stage.row.addView(stage.markerView, new LinearLayout.LayoutParams(dp(28), dp(24)));
        LinearLayout body = new LinearLayout(this);
        body.setOrientation(LinearLayout.VERTICAL);
        stage.titleView = new TextView(this);
        stage.titleView.setTextSize(16);
        stage.titleView.setTextColor(Color.rgb(107, 114, 128));
        stage.titleView.setGravity(Gravity.CENTER_VERTICAL);
        stage.titleView.setIncludeFontPadding(false);
        stage.titleView.setPadding(0, 0, 0, 0);
        stage.titleView.setMinHeight(dp(24));
        stage.bodyView = new TextView(this);
        stage.bodyView.setTextSize(14);
        stage.bodyView.setTextColor(Color.rgb(145, 151, 161));
        stage.bodyView.setLineSpacing(4, 1);
        stage.bodyView.setPadding(0, dp(5), 0, 0);
        stage.itemList = new LinearLayout(this);
        stage.itemList.setOrientation(LinearLayout.VERTICAL);
        body.addView(stage.titleView, new LinearLayout.LayoutParams(-1, dp(24)));
        body.addView(stage.bodyView, new LinearLayout.LayoutParams(-1, -2));
        body.addView(stage.itemList, new LinearLayout.LayoutParams(-1, -2));
        stage.row.addView(body, new LinearLayout.LayoutParams(0, -2, 1));
    }

    private void updateThinkingStageViews(ThinkingStageState stage) {
        boolean done = "completed".equals(stage.status);
        boolean running = "running".equals(stage.status);
        stage.markerView.setText(done ? "✓" : (running ? "●" : "○"));
        stage.markerView.setTextColor(done ? Color.rgb(120, 130, 145) : (running ? Color.rgb(17, 24, 39) : Color.rgb(180, 187, 198)));
        String suffix = done ? "完成" : (running ? "中" : "");
        stage.titleView.setText(stage.title + suffix);
        stage.titleView.setTextColor(done || running ? Color.rgb(93, 101, 115) : Color.rgb(156, 163, 175));
        String body = stage.text.toString().trim();
        stage.bodyView.setText(body);
        stage.bodyView.setVisibility(body.isEmpty() ? View.GONE : View.VISIBLE);
        stage.itemList.removeAllViews();
        if ("buyer_experience".equals(stage.stage) && stage.items.length() > 0) {
            HorizontalScrollView scroll = new HorizontalScrollView(this);
            scroll.setHorizontalScrollBarEnabled(false);
            LinearLayout cards = new LinearLayout(this);
            cards.setOrientation(LinearLayout.HORIZONTAL);
            for (int i = 0; i < stage.items.length(); i++) {
                JSONObject item = stage.items.optJSONObject(i);
                if (item == null) {
                    continue;
                }
                LinearLayout card = thinkingExperienceCard(item);
                LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(dp(190), -2);
                params.setMargins(0, dp(6), dp(10), 0);
                cards.addView(card, params);
            }
            scroll.addView(cards);
            stage.itemList.addView(scroll, new LinearLayout.LayoutParams(-1, -2));
        }
    }

    private LinearLayout thinkingExperienceCard(JSONObject item) {
        LinearLayout card = new LinearLayout(this);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setPadding(dp(12), dp(10), dp(12), dp(10));
        card.setBackground(rounded(Color.rgb(248, 249, 251), dp(10)));
        TextView title = strong(item.optString("title", item.optString("name", "买手经验")));
        title.setTextSize(14);
        card.addView(title, new LinearLayout.LayoutParams(-1, -2));
        String summaryText = item.optString("summary",
                item.optString("recommendReason",
                        item.optString("recommend_reason",
                                item.optString("content", item.optString("text", "")))));
        String price = item.optString("price", "");
        if (!price.trim().isEmpty()) {
            summaryText = summaryText.trim().isEmpty() ? ("价格：" + price) : (summaryText + "\n价格：" + price);
        }
        TextView summary = muted(summaryText);
        summary.setTextSize(13);
        summary.setMaxLines(4);
        card.addView(summary, new LinearLayout.LayoutParams(-1, -2));
        return card;
    }

    private void completeThinkingView() {
        if (activeThinking == null) {
            return;
        }
        boolean animateCollapse = !activeThinking.completed && !activeThinking.userToggled;
        activeThinking.completed = true;
        markKnownThinkingStagesCompleted(activeThinking);
        if (!activeThinking.userToggled) {
            activeThinking.expanded = false;
        }
        renderThinkingView(activeThinking, false, animateCollapse);
        if (!activeThinking.segmentSaved) {
            appendThinkingSegment(activeThinking);
            activeThinking.segmentSaved = true;
        }
    }

    private void beginAssistantAnswer() {
        if (activeAssistantAnswerStarted) {
            return;
        }
        activeAssistantAnswerStarted = true;
        completeThinkingView();
    }

    private void markKnownThinkingStagesCompleted(ThinkingViewState thinking) {
        for (ThinkingStageState stage : thinking.stages.values()) {
            if (!"failed".equals(stage.status)) {
                stage.status = "completed";
            }
            if ("answer_summary".equals(stage.stage) && stage.text.length() == 0) {
                stage.text.append("总结答案完成");
            }
        }
    }

    private boolean isThinkingComplete(ThinkingViewState thinking) {
        if (thinking == null) {
            return false;
        }
        ThinkingStageState answer = thinking.stages.get("answer_summary");
        return answer != null && "completed".equals(answer.status);
    }

    private void appendThinkingSegment(ThinkingViewState thinking) {
        JSONObject value = thinkingToJson(thinking);
        if (value.length() == 0) {
            return;
        }
        try {
            JSONObject segment = new JSONObject();
            segment.put("type", "thinking");
            segment.put("thinking", value);
            activeAssistantSegments = replaceThinkingSegment(activeAssistantSegments, segment);
        } catch (Exception ignored) {
        }
    }

    private JSONArray replaceThinkingSegment(JSONArray source, JSONObject thinkingSegment) {
        JSONArray result = new JSONArray();
        boolean replaced = false;
        if (source != null) {
            for (int i = 0; i < source.length(); i++) {
                JSONObject item = source.optJSONObject(i);
                if (item == null) {
                    continue;
                }
                if ("thinking".equals(item.optString("type")) && !replaced) {
                    result.put(thinkingSegment);
                    replaced = true;
                } else if (!"thinking".equals(item.optString("type"))) {
                    result.put(item);
                }
            }
        }
        if (!replaced) {
            result.put(thinkingSegment);
        }
        return result;
    }

    private JSONObject thinkingToJson(ThinkingViewState thinking) {
        JSONObject value = new JSONObject();
        if (thinking == null || thinking.stages.isEmpty()) {
            return value;
        }
        try {
            value.put("completed", thinking.completed);
            JSONArray stages = new JSONArray();
            for (String key : thinkingStageOrder()) {
                ThinkingStageState stage = thinking.stages.get(key);
                if (stage == null) {
                    continue;
                }
                JSONObject item = new JSONObject();
                item.put("stage", stage.stage);
                item.put("title", stage.title);
                item.put("status", stage.status);
                item.put("text", stage.text.toString());
                item.put("items", new JSONArray(stage.items.toString()));
                stages.put(item);
            }
            value.put("stages", stages);
        } catch (Exception ignored) {
        }
        return value;
    }

    private void renderStoredThinking(JSONObject value, LinearLayout parent) {
        if (value == null || parent == null) {
            return;
        }
        ThinkingViewState thinking = ensureThinkingView(parent, false);
        thinking.completed = value.optBoolean("completed", true);
        thinking.expanded = !thinking.completed;
        JSONArray stages = value.optJSONArray("stages");
        if (stages != null) {
            for (int i = 0; i < stages.length(); i++) {
                JSONObject item = stages.optJSONObject(i);
                if (item == null) {
                    continue;
                }
                ThinkingStageState stage = new ThinkingStageState();
                stage.stage = thinkingStageKey(item.optString("stage", ""));
                stage.title = thinkingStageTitle(stage.stage, item.optString("title", ""));
                stage.status = item.optString("status", "completed");
                String text = item.optString("text", "");
                if (!text.isEmpty()) {
                    stage.text.append(text);
                }
                JSONArray items = item.optJSONArray("items");
                if (items != null) {
                    stage.items = items;
                }
                thinking.stages.put(stage.stage, stage);
            }
        }
        markKnownThinkingStagesCompleted(thinking);
        renderThinkingView(thinking);
    }

    private String[] thinkingStageOrder() {
        return new String[]{"user_need", "buyer_experience", "answer_summary"};
    }

    private int thinkingStageIndex(String stage) {
        String key = thinkingStageKey(stage);
        String[] order = thinkingStageOrder();
        for (int i = 0; i < order.length; i++) {
            if (order[i].equals(key)) {
                return i;
            }
        }
        return -1;
    }

    private String thinkingStageKey(String stage) {
        if ("buyer_experience".equals(stage) || "answer_summary".equals(stage) || "user_need".equals(stage)) {
            return stage;
        }
        if ("intent".equals(stage) || "query_rewrite".equals(stage)) {
            return "user_need";
        }
        if ("retrieve".equals(stage) || "retrieval".equals(stage) || "tool".equals(stage) || "skill".equals(stage)) {
            return "buyer_experience";
        }
        if ("answer".equals(stage) || "done".equals(stage)) {
            return "answer_summary";
        }
        return "user_need";
    }

    private String thinkingStageTitle(String stage, String fallback) {
        if ("buyer_experience".equals(stage)) {
            return "查询买手团经验";
        }
        if ("answer_summary".equals(stage)) {
            return "总结答案";
        }
        return "分析用户需求";
    }

    private String thinkingStageTextFromEvent(JSONObject event) {
        if (event == null) {
            return "";
        }
        String[] keys = new String[]{"delta", "text", "content", "summary", "message", "description"};
        for (String key : keys) {
            String value = event.optString(key, "");
            if (value != null && !value.trim().isEmpty()) {
                return value.trim();
            }
        }
        return "";
    }

    private void flushActiveTextSegment() {
        flushActiveTextSegment(true);
    }

    private void flushActiveTextSegment(boolean allowTableSplit) {
        if (activeAssistantMarkdown == null || activeAssistantMarkdown.toString().trim().isEmpty()) {
            activeAssistant = null;
            activeAssistantMarkdown = null;
            return;
        }
        String rawMarkdown = activeAssistantMarkdown.toString();
        RenderedMarkdown rendered = sanitizeAgentMarkdown(activeAssistantMarkdown.toString());
        if (!rendered.visibleMarkdown.trim().isEmpty()) {
            if (allowTableSplit && activeAssistant != null && containsMarkdownTable(rendered.visibleMarkdown)) {
                ViewGroup parent = (ViewGroup) activeAssistant.getParent();
                if (parent instanceof LinearLayout) {
                    ((LinearLayout) parent).removeView(activeAssistant);
                    addMarkdownPartsToBubble((LinearLayout) parent, rendered.visibleMarkdown);
                } else if (activeAssistantMessageBubble == null) {
                    chatList.removeView(activeAssistant);
                    addMarkdownPartsBubbleTo(rendered.visibleMarkdown, chatList);
                }
            }
            JSONObject segment = new JSONObject();
            try {
                segment.put("type", "text");
                segment.put("text", rawMarkdown);
                activeAssistantSegments.put(segment);
            } catch (Exception ignored) {
            }
        }
        activeAssistant = null;
        activeAssistantMarkdown = null;
    }

    private boolean isInlineProductBlock(JSONObject block) {
        if (block == null) {
            return false;
        }
        String type = block.optString("type");
        return "product_card".equals(type) || "product_refs".equals(type);
    }

    private void appendBlockSegment(JSONObject block) {
        if (block == null) {
            return;
        }
        JSONObject segment = new JSONObject();
        try {
            segment.put("type", "block");
            segment.put("block", new JSONObject(block.toString()));
            activeAssistantSegments.put(segment);
        } catch (Exception ignored) {
        }
    }

    private void appendTextSegment(String markdown) {
        if (markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        RenderedMarkdown rendered = sanitizeAgentMarkdown(markdown);
        if (rendered.visibleMarkdown.trim().isEmpty()) {
            return;
        }
        JSONObject segment = new JSONObject();
        try {
            segment.put("type", "text");
            segment.put("text", markdown);
            activeAssistantSegments.put(segment);
        } catch (Exception ignored) {
        }
    }

    private void appendTableSegment(String markdown) {
        if (markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        JSONObject segment = new JSONObject();
        try {
            segment.put("type", "table");
            segment.put("markdown", markdown.trim());
            activeAssistantSegments.put(segment);
        } catch (Exception ignored) {
        }
    }

    private void renderAgentBlock(JSONObject block, LinearLayout parent) {
        removeLoadingBubbleIfNeeded();
        if (block == null || parent == null) {
            return;
        }
        String type = block.optString("type");
        if ("markdown".equals(type)) {
            TextView bubble = addBubbleTo(block.optString("content", ""), false, parent);
            MarkdownRenderer.setMarkdown(bubble, sanitizeAgentMarkdown(block.optString("content", "")).visibleMarkdown);
            return;
        }
        if ("product_card".equals(type)) {
            JSONObject product = block.optJSONObject("product");
            if (product != null) {
                String productId = messageRenderer.productField(product, "productId", "product_id", "id", "");
                if (!productId.isEmpty()) {
                    if (activeRenderedProductIds.contains(productId)) {
                        return;
                    }
                    activeRenderedProductIds.add(productId);
                }
                LinearLayout bubble = ensureActiveAssistantMessageBubble(parent);
                bubble.addView(messageRenderer.productCard(product));
                scrollBottom();
            }
            return;
        }
        if ("product_refs".equals(type)) {
            renderProductRefs(block.optJSONArray("product_ids"), ensureActiveAssistantMessageBubble(parent));
            return;
        }
        if ("comparison_table".equals(type)) {
            parent.addView(messageRenderer.comparisonCard(block));
            scrollBottom();
            return;
        }
        if ("citation".equals(type)) {
            parent.addView(messageRenderer.citationCard(block.optJSONObject("citation")));
            scrollBottom();
            return;
        }
        if ("citation_refs".equals(type)) {
            return;
        }
        if ("warning".equals(type)) {
            parent.addView(messageRenderer.warningCard(block.optString("message", block.optString("content", ""))));
            scrollBottom();
            return;
        }
        if ("cart_state".equals(type)) {
            parent.addView(messageRenderer.cartStateCard(block.optJSONObject("cart")));
            scrollBottom();
            return;
        }
        if ("order_summary".equals(type)) {
            parent.addView(messageRenderer.orderSummaryCard(block.optJSONArray("orders")));
            scrollBottom();
            return;
        }
        if ("discount_preview".equals(type)) {
            parent.addView(messageRenderer.discountPreviewCard(block));
            scrollBottom();
            return;
        }
        if ("coupon_list".equals(type)) {
            parent.addView(messageRenderer.couponListCard(block));
            scrollBottom();
            return;
        }
        if ("navigation_action".equals(type)) {
            parent.addView(messageRenderer.navigationActionCard(block));
            scrollBottom();
            return;
        }
        if ("review_summary".equals(type)) {
            parent.addView(messageRenderer.reviewSummaryCard(block));
            scrollBottom();
            return;
        }
        if ("after_sales_policy".equals(type)) {
            parent.addView(messageRenderer.afterSalesPolicyCard(block));
            scrollBottom();
            return;
        }
        String content = block.optString("content", "");
        if (!content.isEmpty()) {
            addBubbleTo(content, false, parent);
        } else if (!type.isEmpty()) {
            parent.addView(messageRenderer.warningCard("暂不支持的内容类型：" + type));
            scrollBottom();
        }
    }

    private void renderProductRefs(JSONArray productIds, LinearLayout parent) {
        if (productIds == null || parent == null) {
            return;
        }
        for (int i = 0; i < productIds.length(); i++) {
            String productId = productIds.optString(i, "");
            renderProductRef(productId, parent);
        }
    }

    private void renderItemRefs(JSONArray productIds, LinearLayout parent) {
        if (productIds == null || productIds.length() == 0) {
            return;
        }
        removeDanglingPunctuationFromActiveAssistant();
        renderProductRefs(productIds, ensureActiveAssistantMessageBubble(parent));
    }

    private void renderProductRef(String productId, LinearLayout parent) {
        if (productId == null || productId.trim().isEmpty() || activeRenderedProductIds.contains(productId)) {
            return;
        }
        activeRenderedProductIds.add(productId);
        LinearLayout holder = new LinearLayout(this);
        holder.setOrientation(LinearLayout.VERTICAL);
        parent.addView(holder, new LinearLayout.LayoutParams(-1, -2));
        JSONObject cached = productDetailCache.get(productId);
        if (cached != null) {
            holder.addView(messageRenderer.productCard(cached));
            scrollBottom();
            return;
        }
        TextView loading = muted("正在加载商品信息...");
        holder.addView(loading, new LinearLayout.LayoutParams(-1, dp(32)));
        new Thread(() -> {
            try {
                JSONObject detail = api.productDetail(productId);
                productDetailCache.put(productId, detail);
                runOnUiThread(() -> {
                    if (holder != null) {
                        holder.removeAllViews();
                        holder.addView(messageRenderer.productCard(detail));
                        scrollBottom();
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    holder.removeAllViews();
                    holder.addView(messageRenderer.warningCard("商品信息加载失败：" + productId));
                    scrollBottom();
                });
            }
        }).start();
    }

    private void renderAgentBlocks(JSONArray blocks, LinearLayout parent) {
        if (blocks == null) {
            return;
        }
        for (int i = 0; i < blocks.length(); i++) {
            JSONObject block = blocks.optJSONObject(i);
            if (block != null) {
                renderAgentBlock(block, parent);
            }
        }
    }

    private void renderAgentSegments(JSONArray segments, String fallbackContent, String fallbackBlocks, LinearLayout parent) {
        HistoricalMessageRenderContext context = new HistoricalMessageRenderContext(parent, assistantOriginalMarkdown(segments, fallbackContent, fallbackBlocks));
        renderAgentSegmentsInternal(context, segments, fallbackContent, fallbackBlocks);
    }

    private String assistantOriginalMarkdown(JSONArray segments, String fallbackContent, String fallbackBlocks) {
        StringBuilder markdown = new StringBuilder();
        if (segments != null && segments.length() > 0) {
            for (int i = 0; i < segments.length(); i++) {
                JSONObject segment = segments.optJSONObject(i);
                if (segment == null) {
                    continue;
                }
                String type = segment.optString("type");
                if ("text".equals(type)) {
                    appendCopyMarkdownPart(markdown, segment.optString("text", ""));
                } else if ("table".equals(type)) {
                    appendCopyMarkdownPart(markdown, segment.optString("markdown", ""));
                } else if ("block".equals(type)) {
                    JSONObject block = segment.optJSONObject("block");
                    if (block != null) {
                        appendCopyMarkdownPart(markdown, block.optString("content", ""));
                    }
                }
            }
        }
        if (markdown.toString().trim().isEmpty()) {
            appendCopyMarkdownPart(markdown, fallbackContent);
        }
        if (markdown.toString().trim().isEmpty()) {
            JSONArray blocks = jsonArray(fallbackBlocks);
            for (int i = 0; i < blocks.length(); i++) {
                JSONObject block = blocks.optJSONObject(i);
                if (block != null) {
                    appendCopyMarkdownPart(markdown, block.optString("content", ""));
                }
            }
        }
        return markdown.toString().trim();
    }

    private void appendCopyMarkdownPart(StringBuilder markdown, String part) {
        if (markdown == null || part == null || part.trim().isEmpty()) {
            return;
        }
        if (markdown.length() > 0) {
            markdown.append("\n\n");
        }
        markdown.append(part.trim());
    }

    private void renderAgentSegmentsInternal(HistoricalMessageRenderContext context, JSONArray segments, String fallbackContent, String fallbackBlocks) {
        if (segments == null || segments.length() == 0) {
            if (fallbackContent != null && !fallbackContent.trim().isEmpty()) {
                renderHistoricalTextSegment(context, fallbackContent);
            }
            renderHistoricalBlocks(context, jsonArray(fallbackBlocks));
            return;
        }
        JSONArray pendingThoughts = new JSONArray();
        for (int i = 0; i < segments.length(); i++) {
            JSONObject segment = segments.optJSONObject(i);
            if (segment == null) {
                continue;
            }
            String type = segment.optString("type");
            if ("thinking".equals(type)) {
                messageRenderer.collectHistoricalThinkingSegment(pendingThoughts, segment);
                continue;
            }
            pendingThoughts = flushHistoricalThinkingPanel(context, pendingThoughts);
            if ("text".equals(type)) {
                String text = segment.optString("text", "");
                if (!text.trim().isEmpty()) {
                    renderHistoricalTextSegment(context, text);
                }
            } else if ("table".equals(type)) {
                renderHistoricalTableSegment(context, segment.optString("markdown", ""));
            } else if ("block".equals(type)) {
                renderHistoricalBlock(context, segment.optJSONObject("block"));
            } else if ("product_refs".equals(type) || "product_card".equals(type)) {
                renderHistoricalBlock(context, segment);
            }
        }
        flushHistoricalThinkingPanel(context, pendingThoughts);
    }

    private JSONArray flushHistoricalThinkingPanel(HistoricalMessageRenderContext context, JSONArray thoughts) {
        if (context == null || context.parent == null || thoughts == null || thoughts.length() == 0) {
            return new JSONArray();
        }
        return messageRenderer.flushHistoricalThinkingPanel(context.parent, thoughts, chatScroll);
    }

    private void renderHistoricalTextSegment(HistoricalMessageRenderContext context, String text) {
        if (text == null || text.trim().isEmpty()) {
            return;
        }
        LinearLayout bubble = ensureHistoricalMessageBubble(context);
        messageRenderer.renderHistoricalTextSegment(bubble, text);
    }

    private void renderHistoricalTableSegment(HistoricalMessageRenderContext context, String markdown) {
        String cleaned = markdown == null ? "" : markdown.trim();
        if (cleaned.isEmpty()) {
            return;
        }
        LinearLayout bubble = ensureHistoricalMessageBubble(context);
        messageRenderer.renderHistoricalTableSegment(bubble, cleaned);
    }

    private void renderHistoricalBlocks(HistoricalMessageRenderContext context, JSONArray blocks) {
        if (blocks == null) {
            return;
        }
        for (int i = 0; i < blocks.length(); i++) {
            JSONObject block = blocks.optJSONObject(i);
            if (block != null) {
                renderHistoricalBlock(context, block);
            }
        }
    }

    private void renderHistoricalBlock(HistoricalMessageRenderContext context, JSONObject block) {
        if (context == null || context.parent == null || block == null) {
            return;
        }
        String type = block.optString("type");
        if ("markdown".equals(type)) {
            renderHistoricalTextSegment(context, block.optString("content", ""));
            return;
        }
        if ("product_card".equals(type)) {
            JSONObject product = block.optJSONObject("product");
            if (product == null) {
                product = block.optJSONObject("data");
            }
            removeDanglingPunctuationFromHistoricalBubble(context.bubble);
            renderHistoricalProductCard(context, product);
            return;
        }
        if ("product_refs".equals(type)) {
            JSONArray ids = block.optJSONArray("product_ids");
            if (ids == null) {
                ids = block.optJSONArray("ids");
            }
            renderHistoricalProductRefs(context, ids);
            return;
        }
        if ("comparison_table".equals(type)) {
            context.parent.addView(messageRenderer.comparisonCard(block));
            return;
        }
        if ("warning".equals(type)) {
            context.parent.addView(messageRenderer.warningCard(block.optString("message", block.optString("content", ""))));
            return;
        }
        if ("cart_state".equals(type)) {
            context.parent.addView(messageRenderer.cartStateCard(block.optJSONObject("cart")));
            return;
        }
        if ("order_summary".equals(type)) {
            context.parent.addView(messageRenderer.orderSummaryCard(block.optJSONArray("orders")));
            return;
        }
        if ("discount_preview".equals(type)) {
            context.parent.addView(messageRenderer.discountPreviewCard(block));
            return;
        }
        if ("coupon_list".equals(type)) {
            context.parent.addView(messageRenderer.couponListCard(block));
            return;
        }
        if ("review_summary".equals(type)) {
            context.parent.addView(messageRenderer.reviewSummaryCard(block));
            return;
        }
        if ("after_sales_policy".equals(type)) {
            context.parent.addView(messageRenderer.afterSalesPolicyCard(block));
            return;
        }
        if ("navigation_action".equals(type)) {
            context.parent.addView(messageRenderer.navigationActionCard(block));
        }
    }

    private void renderHistoricalProductCard(HistoricalMessageRenderContext context, JSONObject product) {
        if (product == null) {
            return;
        }
        String productId = messageRenderer.productField(product, "productId", "product_id", "id", "");
        if (!productId.isEmpty()) {
            if (context.renderedProductIds.contains(productId)) {
                return;
            }
            context.renderedProductIds.add(productId);
        }
        ensureHistoricalMessageBubble(context).addView(messageRenderer.productCard(product));
    }

    private void renderHistoricalProductRefs(HistoricalMessageRenderContext context, JSONArray productIds) {
        if (productIds == null || productIds.length() == 0) {
            return;
        }
        LinearLayout bubble = ensureHistoricalMessageBubble(context);
        removeDanglingPunctuationFromHistoricalBubble(bubble);
        for (int i = 0; i < productIds.length(); i++) {
            String productId = productIds.optString(i, "").trim();
            if (productId.isEmpty() || context.renderedProductIds.contains(productId)) {
                continue;
            }
            context.renderedProductIds.add(productId);
            LinearLayout holder = new LinearLayout(this);
            holder.setOrientation(LinearLayout.VERTICAL);
            bubble.addView(holder, new LinearLayout.LayoutParams(-1, -2));
            JSONObject cached = productDetailCache.get(productId);
            if (cached != null) {
                holder.addView(messageRenderer.productCard(cached));
                continue;
            }
            holder.addView(muted("正在加载商品信息..."), new LinearLayout.LayoutParams(-1, dp(32)));
            new Thread(() -> {
                try {
                    JSONObject detail = api.productDetail(productId);
                    productDetailCache.put(productId, detail);
                    runOnUiThread(() -> {
                        if (holder.getParent() != null) {
                            holder.removeAllViews();
                            holder.addView(messageRenderer.productCard(detail));
                        }
                    });
                } catch (Exception error) {
                    runOnUiThread(() -> {
                        if (holder.getParent() != null) {
                            holder.removeAllViews();
                            holder.addView(messageRenderer.warningCard("商品信息加载失败：" + productId));
                        }
                    });
                }
            }).start();
        }
    }

    private LinearLayout ensureHistoricalMessageBubble(HistoricalMessageRenderContext context) {
        if (context.bubble != null) {
            return context.bubble;
        }
        context.bubble = messageRenderer.embeddedProductBubble();
        enableCopy(context.bubble, context.copyMarkdown);
        context.parent.addView(context.bubble);
        return context.bubble;
    }

    private LinearLayout ensureActiveAssistantMessageBubble(LinearLayout parent) {
        removeLoadingBubbleIfNeeded();
        if (activeAssistantMessageBubble != null) {
            return activeAssistantMessageBubble;
        }
        LinearLayout bubble = messageRenderer.embeddedProductBubble();
        enableCopyProvider(bubble, this::activeAssistantCopyMarkdown);
        if (activeAssistant != null && activeAssistant.getParent() instanceof LinearLayout) {
            LinearLayout oldParent = (LinearLayout) activeAssistant.getParent();
            int index = oldParent.indexOfChild(activeAssistant);
            oldParent.removeView(activeAssistant);
            activeAssistant.setBackground(new ColorDrawable(Color.TRANSPARENT));
            activeAssistant.setPadding(0, 0, 0, dp(6));
            activeAssistant.setMaxWidth(Integer.MAX_VALUE);
            bubble.addView(activeAssistant, new LinearLayout.LayoutParams(-1, -2));
            if (index >= 0) {
                oldParent.addView(bubble, index);
            } else {
                oldParent.addView(bubble);
            }
        } else if (parent != null) {
            parent.addView(bubble);
        }
        activeAssistantMessageBubble = bubble;
        return bubble;
    }

    private TextView assistantBubbleText(String text) {
        TextView view = new TextView(this);
        view.setText(text == null ? "" : text);
        view.setTextSize(15);
        view.setLineSpacing(4, 1);
        view.setPadding(0, 0, 0, dp(6));
        view.setTextColor(Color.rgb(24, 30, 37));
        view.setLinksClickable(true);
        return view;
    }

    private String activeAssistantCopyMarkdown() {
        return activeAssistantFullMarkdown == null ? "" : activeAssistantFullMarkdown.toString();
    }

    private void bindFinishedAssistantCopy(String markdown) {
        if (activeAssistant != null) {
            enableCopy(activeAssistant, markdown);
        }
        if (activeAssistantMessageBubble != null) {
            enableCopy(activeAssistantMessageBubble, markdown);
        }
    }

    private void navigateRoute(String route) {
        if (route == null) {
            route = "";
        }
        persistAndCancelActiveStreamForNavigation();
        if ("orders".equals(route)) {
            startActivity(new Intent(this, OrderListActivity.class));
        } else if ("cart".equals(route)) {
            startActivity(new Intent(this, CartActivity.class));
        } else if ("products".equals(route)) {
            startActivity(new Intent(this, ProductListActivity.class));
        } else if ("coupons".equals(route) || "coupon".equals(route)) {
            startActivity(new Intent(this, CouponActivity.class));
        } else if ("settings".equals(route)) {
            startActivity(new Intent(this, SettingsActivity.class));
        } else if ("account".equals(route)) {
            startActivity(new Intent(this, AccountActivity.class));
        } else if ("edit_profile".equals(route) || "profile".equals(route)) {
            startActivity(new Intent(this, EditProfileActivity.class));
        } else if ("promotions".equals(route) || "promotion".equals(route) || "activity".equals(route)) {
            Intent intent = new Intent(this, ProductListActivity.class);
            intent.putExtra(Routes.EXTRA_PRODUCT_TAB, "activity");
            startActivity(intent);
        } else {
            toastLine("暂不支持该跳转");
        }
    }

    private void renderFollowups(JSONArray questions, LinearLayout parent) {
        if (questions == null || questions.length() == 0 || parent == null) {
            return;
        }
        if (streamController.isStreaming() && activeFollowupsView != null && activeFollowupsView.getParent() == parent) {
            parent.removeView(activeFollowupsView);
        }
        View wrap = messageRenderer.followupsView(questions, this::fillInputFromFollowup);
        if (wrap == null) {
            return;
        }
        parent.addView(wrap, new LinearLayout.LayoutParams(-1, -2));
        if (streamController.isStreaming()) {
            activeFollowupsView = wrap;
        }
        scrollBottom();
    }

    private void fillInputFromFollowup(String question) {
        if (input == null || question == null || question.trim().isEmpty()) {
            return;
        }
        if (voiceMode) {
            exitVoiceMode();
        }
        input.setText(question.trim());
        input.setSelection(input.getText().length());
        updateInputActionButtonState();
        input.requestFocus();
    }

    private void hideKeyboard() {
        View view = getCurrentFocus();
        if (view == null) {
            view = input;
        }
        if (view == null) {
            return;
        }
        InputMethodManager manager = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
        if (manager != null) {
            manager.hideSoftInputFromWindow(view.getWindowToken(), 0);
        }
        view.clearFocus();
    }

    private void showKeyboard() {
        if (input == null) {
            return;
        }
        input.postDelayed(() -> {
            InputMethodManager manager = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
            if (manager != null) {
                manager.showSoftInput(input, InputMethodManager.SHOW_IMPLICIT);
            }
        }, 120);
    }

    private void showKeyboard(View target) {
        if (target == null) {
            return;
        }
        target.postDelayed(() -> {
            InputMethodManager manager = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
            if (manager != null) {
                manager.showSoftInput(target, InputMethodManager.SHOW_IMPLICIT);
            }
        }, 120);
    }

    private void hideKeyboardFrom(View view) {
        if (view == null) {
            hideKeyboard();
            return;
        }
        InputMethodManager manager = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
        if (manager != null) {
            manager.hideSoftInputFromWindow(view.getWindowToken(), 0);
        }
    }

    private RenderedMarkdown sanitizeAgentMarkdown(String raw) {
        String value = raw == null ? "" : raw;
        JSONArray toolCalls = new JSONArray();
        JSONArray itemIds = new JSONArray();
        value = stripTaggedBlock(value, "tool_call", toolCalls);
        value = stripTaggedBlock(value, "tool_response", toolCalls);
        value = stripTaggedBlock(value, "think", null);
        value = extractItemRefs(value, itemIds);
        return new RenderedMarkdown(value.trim(), toolCalls, itemIds);
    }

    private String extractItemRefs(String value, JSONArray itemIds) {
        Pattern pattern = Pattern.compile("\\s*[。\\.，,；;：:、]?\\s*<item>([^<]+)</item>\\s*[。\\.，,；;：:、]?\\s*");
        Matcher matcher = pattern.matcher(value == null ? "" : value);
        StringBuffer buffer = new StringBuffer();
        while (matcher.find()) {
            String productId = matcher.group(1).trim();
            if (!productId.isEmpty()) {
                itemIds.put(productId);
            }
            matcher.appendReplacement(buffer, " ");
        }
        matcher.appendTail(buffer);
        return buffer.toString()
                .replaceAll("[ \\t]+\\n", "\n")
                .replaceAll("\\n[ \\t]+", "\n")
                .replaceAll("[ \\t]{2,}", " ");
    }

    private String stripTaggedBlock(String value, String tag, JSONArray captures) {
        String open = "<" + tag + ">";
        String close = "</" + tag + ">";
        String result = value == null ? "" : value;
        while (true) {
            int start = result.indexOf(open);
            if (start < 0) {
                return result;
            }
            int end = result.indexOf(close, start + open.length());
            if (end < 0) {
                return result.substring(0, start);
            }
            String body = result.substring(start + open.length(), end).trim();
            if (captures != null && !body.isEmpty()) {
                captures.put(body);
            }
            result = result.substring(0, start) + result.substring(end + close.length());
        }
    }

    private JSONArray jsonArray(String value) {
        try {
            return value == null || value.isEmpty() ? new JSONArray() : new JSONArray(value);
        } catch (Exception ignored) {
            return new JSONArray();
        }
    }

    private void saveAccountSession(String token, JSONObject account, String fallbackRole, String fallbackName) {
        AccountSessionHelper.saveAccountSession(sessionStore, token, account, fallbackRole, fallbackName);
    }

    private void addProductToCart(JSONObject item) {
        addProductToCart(item, null);
    }

    private void addProductToCart(JSONObject item, Button sourceButton) {
        if (sessionStore.token().isEmpty()) {
            toastLine("请先登录后再加入购物车");
            openLoginPage();
            return;
        }
        if (sourceButton != null) {
            sourceButton.setEnabled(false);
            sourceButton.setAlpha(0.72f);
            sourceButton.setText("加入中...");
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
                runOnUiThread(() -> {
                    toastLine("已加入购物车");
                    if (sourceButton != null) {
                        sourceButton.setEnabled(true);
                        sourceButton.setAlpha(1f);
                        sourceButton.setText("进入购物车");
                        sourceButton.setOnClickListener(v -> startActivity(new Intent(this, CartActivity.class)));
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (sourceButton != null) {
                        sourceButton.setEnabled(true);
                        sourceButton.setAlpha(1f);
                        sourceButton.setText("加入购物车");
                        sourceButton.setOnClickListener(v -> addProductToCart(item, sourceButton));
                    }
                    toastLine("加入失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private View addUserMessageBubble(String text, JSONArray attachments) {
        if (attachments == null || attachments.length() == 0) {
            return addBubble(text == null ? "" : text, true);
        }
        LinearLayout shell = new LinearLayout(this);
        shell.setOrientation(LinearLayout.VERTICAL);
        shell.setGravity(Gravity.RIGHT);
        LinearLayout.LayoutParams shellParams = new LinearLayout.LayoutParams(-1, -2);
        shellParams.setMargins(0, dp(6), 0, dp(6));

        String safeText = text == null ? "" : text.trim();
        if (!safeText.isEmpty()) {
            LinearLayout bubble = userMessageBubbleContainer();
            TextView textView = new TextView(this);
            textView.setText(safeText);
            textView.setTextSize(15);
            textView.setLineSpacing(4, 1);
            textView.setTextColor(Color.WHITE);
            bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
            shell.addView(bubble);
        }
        LinearLayout attachmentPanel = userAttachmentPanel();
        LinearLayout attachmentList = new LinearLayout(this);
        attachmentList.setOrientation(attachments.length() > 1 ? LinearLayout.HORIZONTAL : LinearLayout.VERTICAL);
        for (int i = 0; i < attachments.length(); i++) {
            JSONObject attachment = attachments.optJSONObject(i);
            if (attachment == null) {
                continue;
            }
            View view = isImageAttachment(attachment) ? userImageAttachmentView(attachment) : userFileAttachmentView(attachment);
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(attachments.length() > 1 ? dp(132) : -1, attachments.length() > 1 ? dp(132) : dp(260));
            params.setMargins(0, 0, i < attachments.length() - 1 ? dp(8) : 0, 0);
            attachmentList.addView(view, params);
        }
        if (attachments.length() > 1) {
            HorizontalScrollView scroll = new HorizontalScrollView(this);
            scroll.setHorizontalScrollBarEnabled(false);
            scroll.addView(attachmentList);
            attachmentPanel.addView(scroll, new LinearLayout.LayoutParams(-1, -2));
        } else {
            attachmentPanel.addView(attachmentList, new LinearLayout.LayoutParams(-1, -2));
        }
        enableCopy(attachmentPanel, userMessageCopyText(safeText, attachments));
        LinearLayout.LayoutParams panelParams = new LinearLayout.LayoutParams((int) (getResources().getDisplayMetrics().widthPixels * 0.76f), ViewGroup.LayoutParams.WRAP_CONTENT);
        panelParams.topMargin = safeText.isEmpty() ? 0 : dp(6);
        shell.addView(attachmentPanel, panelParams);
        chatList.addView(shell, shellParams);
        scrollBottom();
        return shell;
    }

    private LinearLayout userMessageBubbleContainer() {
        LinearLayout bubble = new LinearLayout(this);
        bubble.setOrientation(LinearLayout.VERTICAL);
        bubble.setPadding(dp(10), dp(9), dp(10), dp(9));
        bubble.setBackground(rounded(Color.rgb(20, 20, 20), dp(16)));
        bubble.setLayoutParams(new LinearLayout.LayoutParams((int) (getResources().getDisplayMetrics().widthPixels * 0.76f), ViewGroup.LayoutParams.WRAP_CONTENT));
        return bubble;
    }

    private LinearLayout userAttachmentPanel() {
        LinearLayout panel = new LinearLayout(this);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setPadding(dp(8), dp(8), dp(8), dp(8));
        panel.setBackground(rounded(Color.WHITE, dp(14)));
        return panel;
    }

    private View userImageAttachmentView(JSONObject attachment) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(rounded(Color.rgb(235, 238, 243), dp(12)));
        ImageView image = new ImageView(this);
        image.setScaleType(ImageView.ScaleType.FIT_CENTER);
        image.setAdjustViewBounds(true);
        image.setBackgroundColor(Color.rgb(248, 250, 252));
        String imageUri = attachmentImageUri(attachment);
        if (!imageUri.isEmpty()) {
            loadAttachmentImage(image, imageUri);
        }
        frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
        if (imageUri.isEmpty()) {
            TextView placeholder = muted("图片");
            placeholder.setGravity(Gravity.CENTER);
            frame.addView(placeholder, new FrameLayout.LayoutParams(-1, -1));
        }
        return frame;
    }

    private View userFileAttachmentView(JSONObject attachment) {
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setGravity(Gravity.CENTER);
        box.setPadding(dp(10), dp(8), dp(10), dp(8));
        box.setBackground(rounded(Color.rgb(245, 247, 250), dp(12)));
        TextView icon = strong("文档");
        icon.setGravity(Gravity.CENTER);
        box.addView(icon, new LinearLayout.LayoutParams(-1, -2));
        TextView name = muted(shortFileName(attachmentDisplayName(attachment)));
        name.setGravity(Gravity.CENTER);
        name.setMaxLines(2);
        box.addView(name, new LinearLayout.LayoutParams(-1, -2));
        return box;
    }

    private UploadingMessageViewState addUploadingUserMessageBubble(String text, List<PendingAttachment> attachments) {
        clearWelcomeIfNeeded();
        UploadingMessageViewState state = new UploadingMessageViewState();
        LinearLayout shell = new LinearLayout(this);
        shell.setGravity(Gravity.RIGHT);
        LinearLayout.LayoutParams shellParams = new LinearLayout.LayoutParams(-1, -2);
        shellParams.setMargins(0, dp(6), 0, dp(6));
        LinearLayout bubble = userMessageBubbleContainer();
        String safeText = text == null ? "" : text.trim();
        if (!safeText.isEmpty()) {
            TextView textView = new TextView(this);
            textView.setText(safeText);
            textView.setTextSize(15);
            textView.setTextColor(Color.WHITE);
            textView.setPadding(0, 0, 0, dp(8));
            bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
        }
        LinearLayout previews = new LinearLayout(this);
        previews.setOrientation(attachments != null && attachments.size() > 1 ? LinearLayout.HORIZONTAL : LinearLayout.VERTICAL);
        if (attachments != null) {
            for (int i = 0; i < attachments.size(); i++) {
                PendingAttachment attachment = attachments.get(i);
                View preview = uploadingAttachmentView(attachment);
                state.attachmentViews.add(preview);
                LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(attachments.size() > 1 ? dp(104) : dp(176), attachments.size() > 1 ? dp(104) : dp(132));
                params.setMargins(0, 0, i < attachments.size() - 1 ? dp(8) : 0, 0);
                previews.addView(preview, params);
            }
        }
        if (attachments != null && attachments.size() > 1) {
            HorizontalScrollView scroll = new HorizontalScrollView(this);
            scroll.setHorizontalScrollBarEnabled(false);
            scroll.addView(previews);
            bubble.addView(scroll, new LinearLayout.LayoutParams(-1, -2));
        } else {
            bubble.addView(previews, new LinearLayout.LayoutParams(-1, -2));
        }
        state.statusText = new TextView(this);
        state.statusText.setText("准备上传");
        state.statusText.setTextSize(13);
        state.statusText.setTextColor(Color.rgb(220, 224, 230));
        state.statusText.setPadding(0, dp(8), 0, 0);
        bubble.addView(state.statusText, new LinearLayout.LayoutParams(-1, -2));
        shell.addView(bubble);
        state.container = shell;
        chatList.addView(shell, shellParams);
        scrollBottom();
        return state;
    }

    private View uploadingAttachmentView(PendingAttachment attachment) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(rounded(Color.rgb(235, 238, 243), dp(12)));
        if (attachment != null && "image".equals(attachment.type)) {
            ImageView image = new ImageView(this);
            image.setScaleType(ImageView.ScaleType.CENTER_CROP);
            image.setImageURI(attachment.uri);
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
        } else {
            TextView file = muted(attachment == null ? "附件" : shortFileName(attachment.name));
            file.setGravity(Gravity.CENTER);
            frame.addView(file, new FrameLayout.LayoutParams(-1, -1));
        }
        View overlay = new View(this);
        overlay.setBackgroundColor(Color.argb(96, 0, 0, 0));
        frame.addView(overlay, new FrameLayout.LayoutParams(-1, -1));
        ProgressBar progress = new ProgressBar(this);
        FrameLayout.LayoutParams progressParams = new FrameLayout.LayoutParams(dp(34), dp(34), Gravity.CENTER);
        frame.addView(progress, progressParams);
        return frame;
    }

    private void updateUploadingAttachmentState(UploadingMessageViewState state, int index, String status) {
        if (state == null || state.statusText == null) {
            return;
        }
        state.statusText.setText(status == null || status.isEmpty() ? "正在上传" : status);
    }

    private void finishUploadingUserMessageBubble(UploadingMessageViewState state, String text, JSONArray uploaded) {
        if (state != null && state.container != null && state.container.getParent() instanceof ViewGroup) {
            ((ViewGroup) state.container.getParent()).removeView(state.container);
        }
        if (actionButton != null) {
            actionButton.setEnabled(true);
        }
        sendMessage(text, uploaded == null ? new JSONArray() : uploaded);
    }

    private void failUploadingUserMessageBubble(UploadingMessageViewState state, Throwable error) {
        if (state == null || state.statusText == null) {
            return;
        }
        state.failed = true;
        state.statusText.setText("上传失败：" + readableErrorMessage(error));
        state.statusText.setTextColor(Color.rgb(255, 205, 210));
    }

    private boolean isImageAttachment(JSONObject attachment) {
        if (attachment == null) {
            return false;
        }
        String type = attachment.optString("type", "");
        String mime = attachment.optString("mime_type", attachment.optString("mimeType", ""));
        String value = (attachment.optString("url", "") + " " + attachment.optString("name", "")).toLowerCase();
        return "image".equals(type) || mime.startsWith("image/") || value.endsWith(".jpg") || value.endsWith(".jpeg") || value.endsWith(".png") || value.endsWith(".webp") || value.endsWith(".gif");
    }

    private String attachmentDisplayName(JSONObject attachment) {
        if (attachment == null) {
            return "附件";
        }
        String name = attachment.optString("name", "");
        if (!name.isEmpty()) {
            return name;
        }
        return attachment.optString("attachment_id", attachment.optString("object_key", "附件"));
    }

    private String attachmentImageUri(JSONObject attachment) {
        if (attachment == null) {
            return "";
        }
        String url = attachment.optString("url", "");
        if (!url.isEmpty()) {
            return url;
        }
        String local = attachment.optString("local_uri", "");
        if (!local.isEmpty()) {
            return local;
        }
        return attachment.optString("object_key", "");
    }

    private void loadAttachmentImage(ImageView image, String uri) {
        if (image == null || uri == null || uri.trim().isEmpty()) {
            return;
        }
        String value = uri.trim();
        if (value.startsWith("/")) {
            value = api.absoluteUrl(value);
        }
        if (value.startsWith("http://") || value.startsWith("https://")) {
            String imageUrl = value;
            new Thread(() -> {
                HttpURLConnection connection = null;
                try {
                    connection = (HttpURLConnection) new URL(imageUrl).openConnection();
                    connection.setConnectTimeout(10000);
                    connection.setReadTimeout(30000);
                    String token = sessionStore == null ? "" : sessionStore.token();
                    if (token != null && !token.isEmpty()) {
                        connection.setRequestProperty("Authorization", "Bearer " + token);
                    }
                    try (InputStream inputStream = connection.getInputStream()) {
                        Bitmap bitmap = BitmapFactory.decodeStream(inputStream);
                        if (bitmap != null) {
                            runOnUiThread(() -> image.setImageBitmap(bitmap));
                        }
                    }
                } catch (Exception ignored) {
                } finally {
                    if (connection != null) {
                        connection.disconnect();
                    }
                }
            }).start();
            return;
        }
        try {
            image.setImageURI(Uri.parse(value));
        } catch (Exception ignored) {
        }
    }

    private String userMessageCopyText(String text, JSONArray attachments) {
        StringBuilder builder = new StringBuilder(text == null ? "" : text.trim());
        if (attachments != null) {
            for (int i = 0; i < attachments.length(); i++) {
                JSONObject item = attachments.optJSONObject(i);
                if (item == null) {
                    continue;
                }
                if (builder.length() > 0) {
                    builder.append('\n');
                }
                builder.append(isImageAttachment(item) ? "!["
                        : "[");
                builder.append(attachmentDisplayName(item)).append("](").append(attachmentImageUri(item)).append(")");
            }
        }
        return builder.toString();
    }

    private TextView addBubble(String text, boolean right) {
        return addBubbleTo(text, right, chatList);
    }

    private TextView addBubbleTo(String text, boolean right, LinearLayout parent) {
        if (!right && containsMarkdownTable(text)) {
            return addMarkdownPartsBubbleTo(text, parent);
        }
        TextView bubble = new TextView(this);
        if (right) {
            bubble.setText(text);
        } else {
            MarkdownRenderer.setMarkdown(bubble, sanitizeAgentMarkdown(text).visibleMarkdown);
        }
        bubble.setTextSize(15);
        bubble.setLineSpacing(4, 1);
        bubble.setPadding(dp(12), dp(9), dp(12), dp(9));
        bubble.setTextColor(right ? Color.WHITE : Color.rgb(24, 30, 37));
        bubble.setBackground(rounded(right ? Color.rgb(20, 20, 20) : Color.WHITE, dp(16)));
        bubble.setMaxWidth((int) (getResources().getDisplayMetrics().widthPixels * 0.74f));
        bubble.setLinksClickable(true);
        enableCopy(bubble, text);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = right ? Gravity.RIGHT : Gravity.LEFT;
        params.setMargins(0, dp(6), 0, dp(6));
        parent.addView(bubble, params);
        scrollBottom();
        return bubble;
    }

    private TextView addMarkdownPartsBubbleTo(String text, LinearLayout parent) {
        TextView firstText = markdownRenderer.addPartsBubbleTo(text, parent);
        scrollBottom();
        return firstText;
    }

    private void addMarkdownTableToBubble(LinearLayout bubble, String markdown) {
        markdownRenderer.addTableToBubble(bubble, markdown);
    }

    private void addMarkdownPartsToBubble(LinearLayout bubble, String markdown) {
        markdownRenderer.addPartsToBubble(bubble, markdown);
    }

    private TextView markdownTextView(String markdown) {
        return markdownRenderer.markdownTextView(markdown);
    }

    private List<ChatMarkdownRenderer.Part> splitMarkdownParts(String markdown) {
        return markdownRenderer.splitParts(markdown);
    }

    private interface CopyTextProvider {
        String copyText();
    }

    private boolean containsMarkdownTable(String text) {
        return markdownRenderer.containsTable(text);
    }

    private int findMarkdownTableStart(String markdown) {
        return markdownRenderer.findTableStart(markdown);
    }

    private void enableCopy(View view, String text) {
        enableCopyProvider(view, () -> text == null ? "" : text);
    }

    private void enableCopyProvider(View view, CopyTextProvider provider) {
        if (view == null) {
            return;
        }
        view.setOnLongClickListener(v -> {
            copyToClipboard(provider == null ? "" : provider.copyText());
            return true;
        });
    }

    private void copyToClipboard(String text) {
        ClipboardManager manager = (ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
        if (manager != null) {
            manager.setPrimaryClip(ClipData.newPlainText("聊天内容", text == null ? "" : text));
            toastLine("已复制");
        }
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
            saveDrawerHistoryState();
            root.removeView(drawerLayer);
            drawerLayer = null;
            drawerPanel = null;
            drawerHistoryList = null;
            drawerSearchInput = null;
            drawerHistoryScroll = null;
        }
        restoreKeyboardModeForActivePage();
    }

    private void closeDrawerAnimated() {
        if (drawerLayer == null || drawerPanel == null) {
            restoreKeyboardModeForActivePage();
            return;
        }
        saveDrawerHistoryState();
        int panelWidth = drawerPanel.getWidth() == 0 ? (int) (getResources().getDisplayMetrics().widthPixels * 0.82f) : drawerPanel.getWidth();
        FrameLayout closingLayer = drawerLayer;
        LinearLayout closingPanel = drawerPanel;
        drawerLayer = null;
        drawerPanel = null;
        drawerHistoryList = null;
        drawerSearchInput = null;
        drawerHistoryScroll = null;
        closingLayer.animate().alpha(0f).setDuration(180).start();
        closingPanel.setLayerType(View.LAYER_TYPE_HARDWARE, null);
        closingPanel.animate()
                .translationX(-panelWidth)
                .setDuration(180)
                .withEndAction(() -> {
                    closingPanel.setLayerType(View.LAYER_TYPE_NONE, null);
                if (closingLayer.getParent() == root) {
                    root.removeView(closingLayer);
                }
                restoreKeyboardModeForActivePage();
                })
                .start();
    }

    private void saveDrawerHistoryState() {
        if (drawerHistoryScroll == null) {
            return;
        }
        String query = drawerSearchInput == null ? "" : drawerSearchInput.getText().toString().trim();
        historyDrawer.saveState(drawerHistoryScroll.getScrollY(), query);
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
            } else if ("assistant".equals(message.role)) {
                renderAgentSegments(jsonArray(message.segmentsJson), message.content, message.blocksJson, chatList);
                renderFollowups(jsonArray(message.followupsJson), chatList);
            } else {
                addUserMessageBubble(message.content, jsonArray(message.attachmentsJson));
            }
        }
    }

    private void scrollBottom() {
        scrollBottomIfAllowed();
    }

    private Button iconButton(String text) {
        return ShopUi.iconButton(this, text);
    }

    private Button transparentIconButton(String text) {
        return ShopUi.transparentIconButton(this, text);
    }

    private Button darkRoundButton(String text) {
        return ShopUi.darkRoundButton(this, text);
    }

    private Button textOnlyButton(String text) {
        return ShopUi.textOnlyButton(this, text);
    }

    private Button primaryButton(String text) {
        return ShopUi.primaryButton(this, text);
    }

    private Button secondaryButton(String text) {
        return ShopUi.secondaryButton(this, text);
    }

    private Button navButton(String text, View.OnClickListener listener) {
        Button button = secondaryButton(text);
        button.setGravity(Gravity.LEFT | Gravity.CENTER_VERTICAL);
        button.setOnClickListener(listener);
        button.setPadding(dp(18), 0, dp(18), 0);
        button.setLayoutParams(new LinearLayout.LayoutParams(-1, dp(50)));
        return button;
    }

    private View drawerNavButton(String icon, String text, String page, View.OnClickListener listener) {
        LinearLayout button = new LinearLayout(this);
        button.setGravity(Gravity.CENTER_VERTICAL);
        button.setPadding(dp(14), 0, dp(14), 0);
        button.setBackgroundColor(page.equals(activePage) ? Color.rgb(245, 246, 248) : Color.WHITE);
        button.setOnClickListener(listener);
        TextView iconView = new TextView(this);
        iconView.setText(icon);
        iconView.setTextSize(21);
        iconView.setTextColor(Color.rgb(31, 41, 55));
        iconView.setGravity(Gravity.CENTER);
        iconView.setIncludeFontPadding(false);
        LinearLayout.LayoutParams iconParams = new LinearLayout.LayoutParams(dp(34), dp(34));
        button.addView(iconView, iconParams);
        TextView label = new TextView(this);
        label.setText(text);
        label.setTextSize(16);
        label.setTextColor(Color.rgb(17, 24, 39));
        label.setGravity(Gravity.CENTER_VERTICAL);
        LinearLayout.LayoutParams labelParams = new LinearLayout.LayoutParams(0, -1, 1);
        labelParams.leftMargin = dp(12);
        button.addView(label, labelParams);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(48));
        params.setMargins(0, dp(2), 0, dp(2));
        button.setLayoutParams(params);
        return button;
    }

    private void flattenButton(Button button) {
        ShopUi.flattenButton(button);
    }

    private LinearLayout panel() {
        return ShopUi.panel(this);
    }

    private TextView strong(String text) {
        return ShopUi.strong(this, text);
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

    private String initialOf(String name) {
        String value = name == null || name.trim().isEmpty() ? "我" : name.trim();
        return value.substring(0, 1);
    }

    private String shortName(String name) {
        String value = name == null || name.trim().isEmpty() ? "用户名" : name.trim();
        return value.length() > 6 ? value.substring(0, 6) + "…" : value;
    }

    private String emptyFallback(String value, String fallback) {
        return value == null || value.trim().isEmpty() ? fallback : value.trim();
    }

    private String maskPhone(String phone) {
        String value = phone == null ? "" : phone.trim();
        if (value.length() < 7) {
            return value;
        }
        return value.substring(0, 3) + "******" + value.substring(value.length() - 2);
    }

    private String maskEmail(String email) {
        String value = email == null ? "" : email.trim();
        int at = value.indexOf("@");
        if (at <= 1) {
            return value;
        }
        return value.substring(0, 1) + "***" + value.substring(at);
    }

    private View historyButton(LocalChatStore.SessionSummary item) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setTag("history:" + item.localSessionId);
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
        title.setText(item.pinned ? "置顶 · " + label : label);
        title.setTextSize(15);
        title.setTextColor(Color.BLACK);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(0, -2, 1);
        titleParams.leftMargin = dp(14);
        row.addView(title, titleParams);
        row.setOnClickListener(v -> {
            hideKeyboard();
            persistAndCancelActiveStreamForNavigation();
            String targetLocalSessionId = item.localSessionId;
            localSessionId = targetLocalSessionId;
            serverSessionId = item.serverSessionId == null ? "" : item.serverSessionId;
            closeDrawerAnimated();
            renderChatHome();
            loadHomeCopy();
            if (!serverSessionId.isEmpty() && !chatStore.hasMessages(targetLocalSessionId)) {
                loadRemoteSessionDetail(serverSessionId, targetLocalSessionId);
            }
        });
        row.setOnLongClickListener(v -> {
            showHistoryActionSheet(item);
            return true;
        });
        return row;
    }

    private void showHistoryActionSheet(LocalChatStore.SessionSummary item) {
        String pinText = item.pinned ? "取消置顶" : "置顶会话";
        LinearLayout box = makeBottomSheetBox();
        TextView titleView = title(item.title == null || item.title.isEmpty() ? "导购会话" : item.title);
        titleView.setTextSize(20);
        box.addView(titleView, new LinearLayout.LayoutParams(-1, -2));
        TextView hint = muted("选择会话操作");
        hint.setPadding(0, dp(4), 0, dp(8));
        box.addView(hint, new LinearLayout.LayoutParams(-1, -2));

        final AlertDialog[] dialogRef = new AlertDialog[1];
        box.addView(actionSheetRow(pinText, false, () -> {
            dismissDialog(dialogRef[0]);
            toggleHistoryPinned(item);
        }), new LinearLayout.LayoutParams(-1, dp(48)));
        box.addView(actionSheetRow("更改会话标题", false, () -> {
            dismissDialog(dialogRef[0]);
            showRenameSessionDialog(item);
        }), new LinearLayout.LayoutParams(-1, dp(48)));
        box.addView(actionSheetRow("删除会话", true, () -> {
            dismissDialog(dialogRef[0]);
            confirmDeleteSession(item);
        }), new LinearLayout.LayoutParams(-1, dp(48)));
        dialogRef[0] = showBottomSheetDialog(box);
    }

    private TextView actionSheetRow(String text, boolean danger, Runnable onClick) {
        TextView row = new TextView(this);
        row.setText(text);
        row.setTextSize(16);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setIncludeFontPadding(false);
        row.setTextColor(danger ? Color.rgb(220, 38, 38) : Color.rgb(17, 24, 39));
        row.setPadding(0, 0, 0, 0);
        row.setOnClickListener(v -> {
            if (onClick != null) {
                onClick.run();
            }
        });
        return row;
    }

    private void dismissDialog(AlertDialog dialog) {
        if (dialog != null) {
            dialog.dismiss();
        }
    }

    private void toggleHistoryPinned(LocalChatStore.SessionSummary item) {
        boolean pinned = !item.pinned;
        chatStore.pinSession(item.localSessionId, pinned);
        refreshDrawerContent();
        if (item.serverSessionId != null && !item.serverSessionId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.pinSession(item.serverSessionId, pinned);
                    chatStore.markSessionSyncState(item.localSessionId, "synced");
                } catch (Exception error) {
                    chatStore.markSessionSyncState(item.localSessionId, "failed");
                    runOnUiThread(() -> handleApiError(error));
                }
            }).start();
        }
    }

    private void showRenameSessionDialog(LocalChatStore.SessionSummary item) {
        LinearLayout box = makeBottomSheetBox();
        TextView titleView = title("更改会话标题");
        titleView.setTextSize(20);
        box.addView(titleView, new LinearLayout.LayoutParams(-1, -2));
        EditText editor = new EditText(this);
        editor.setSingleLine(true);
        editor.setText(item.title == null ? "" : item.title);
        editor.setSelection(editor.getText().length());
        editor.setPadding(dp(12), dp(8), dp(12), dp(8));
        editor.setTextSize(16);
        editor.setBackground(rounded(Color.rgb(245, 246, 248), dp(12)));
        LinearLayout.LayoutParams editorParams = new LinearLayout.LayoutParams(-1, dp(48));
        editorParams.setMargins(0, dp(14), 0, dp(16));
        box.addView(editor, editorParams);

        final AlertDialog[] dialogRef = new AlertDialog[1];
        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        Button cancel = secondaryButton("取消");
        cancel.setOnClickListener(v -> dismissDialog(dialogRef[0]));
        Button save = primaryButton("保存");
        save.setOnClickListener(v -> saveRenamedSession(item, editor, dialogRef[0]));
        actions.addView(cancel, new LinearLayout.LayoutParams(0, dp(48), 1));
        LinearLayout.LayoutParams saveParams = new LinearLayout.LayoutParams(0, dp(48), 1);
        saveParams.leftMargin = dp(10);
        actions.addView(save, saveParams);
        box.addView(actions, new LinearLayout.LayoutParams(-1, -2));
        dialogRef[0] = showBottomSheetDialog(box);
        editor.postDelayed(() -> {
            editor.requestFocus();
            showKeyboard(editor);
        }, 220);
    }

    private void saveRenamedSession(LocalChatStore.SessionSummary item, EditText editor, AlertDialog dialog) {
        String title = editor.getText().toString().trim();
        if (title.isEmpty()) {
            toastLine("标题不能为空");
            return;
        }
        dismissDialog(dialog);
        chatStore.renameSession(item.localSessionId, title);
        refreshDrawerContent();
        if (item.serverSessionId != null && !item.serverSessionId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.updateSession(item.serverSessionId, title, "");
                    chatStore.markSessionSyncState(item.localSessionId, "synced");
                } catch (Exception error) {
                    chatStore.markSessionSyncState(item.localSessionId, "failed");
                    runOnUiThread(() -> handleApiError(error));
                }
            }).start();
        }
    }

    private void confirmDeleteSession(LocalChatStore.SessionSummary item) {
        showConfirmBottomSheet("删除会话", "删除后侧栏不再显示该会话。是否继续？", "删除", true, () -> deleteHistorySession(item));
    }

    private void deleteHistorySession(LocalChatStore.SessionSummary item) {
        chatStore.deleteSession(item.localSessionId);
        boolean deletingCurrent = item.localSessionId != null && item.localSessionId.equals(localSessionId);
        if (item.serverSessionId != null && !item.serverSessionId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.deleteSession(item.serverSessionId);
                    chatStore.markSessionSyncState(item.localSessionId, "synced");
                    runOnUiThread(this::refreshDrawerContent);
                } catch (Exception error) {
                    chatStore.markSessionSyncState(item.localSessionId, "failed");
                    runOnUiThread(() -> handleApiError(error));
                }
            }).start();
        }
        if (deletingCurrent) {
            createFreshLocalSession();
        }
        historyDrawer.setLocallyMutated(true);
        refreshDrawerContent(false);
        if (item.serverSessionId == null || item.serverSessionId.isEmpty()) {
            String query = drawerSearchInput == null ? "" : drawerSearchInput.getText().toString().trim();
            loadRemoteHistoryPage(query, 0, false);
        }
    }

    private void loadRemoteSessionDetail(String sessionId, String targetLocalSessionId) {
        new Thread(() -> {
            try {
                JSONObject detail = api.sessionDetail(sessionId);
                JSONArray messages = detail.optJSONArray("messages");
                if (messages == null) {
                    return;
                }
                for (int i = 0; i < messages.length(); i++) {
                    JSONObject message = messages.optJSONObject(i);
                    if (message != null) {
                        String content = message.optString("content", "");
                        JSONArray blocks = message.optJSONArray("blocks");
                        JSONArray followups = message.optJSONArray("followups");
                        JSONArray segments = message.optJSONArray("segments");
                        JSONArray runs = message.optJSONArray("runs");
                        long createdAt = ChatHistoryDrawer.parseRemoteTime(message.optString("created_at", ""));
                        String role = message.optString("role", "user");
                        if (hasJsonItems(blocks) || hasJsonItems(segments) || !content.trim().isEmpty()) {
                            chatStore.saveRemoteMessageSnapshot(targetLocalSessionId, role, content, jsonString(blocks), jsonString(followups), jsonString(segments), "synced", createdAt);
                        }
                        if ("user".equals(role) && runs != null) {
                            saveAssistantRunsFromRemote(targetLocalSessionId, runs, createdAt + 1);
                        }
                    }
                }
                runOnUiThread(() -> {
                    if (targetLocalSessionId.equals(localSessionId)) {
                        renderChatHome();
                        loadHomeCopy();
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("远程会话加载失败：" + error.getMessage()));
            }
        }).start();
    }

    private View bottomUserBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(6), dp(8), dp(2), dp(8));
        bar.setBackgroundColor(Color.WHITE);
        bar.setClickable(true);

        LinearLayout user = new LinearLayout(this);
        user.setGravity(Gravity.CENTER_VERTICAL);
        user.setOnClickListener(v -> {
            persistAndCancelActiveStreamForNavigation();
            startActivity(new Intent(this, SettingsActivity.class));
        });
        user.addView(AccountUi.avatarView(this, sessionStore, api, imageLoader, dp(42), 17), new LinearLayout.LayoutParams(dp(42), dp(42)));
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
        settings.setOnClickListener(v -> {
            persistAndCancelActiveStreamForNavigation();
            startActivity(new Intent(this, SettingsActivity.class));
        });
        bar.addView(settings, new LinearLayout.LayoutParams(dp(44), dp(44)));
        return bar;
    }

    private void saveAssistantRunsFromRemote(String targetLocalSessionId, JSONArray runs, long fallbackCreatedAt) {
        for (int i = 0; i < runs.length(); i++) {
            JSONObject run = runs.optJSONObject(i);
            if (run == null) {
                continue;
            }
            String content = run.optString("content", "");
            JSONArray blocks = run.optJSONArray("blocks");
            JSONArray followups = run.optJSONArray("followups");
            JSONArray segments = run.optJSONArray("segments");
            if (content.trim().isEmpty() && !hasJsonItems(blocks) && !hasJsonItems(segments)) {
                continue;
            }
            long createdAt = ChatHistoryDrawer.parseRemoteTime(run.optString("updated_at", ""));
            if (createdAt <= 0) {
                createdAt = fallbackCreatedAt + i;
            }
            chatStore.saveRemoteMessageSnapshot(targetLocalSessionId, "assistant", content, jsonString(blocks), jsonString(followups), jsonString(segments), run.optString("status", "synced"), createdAt);
        }
    }

    private String jsonString(JSONArray array) {
        return array == null ? "[]" : array.toString();
    }

    private boolean hasJsonItems(JSONArray array) {
        return array != null && array.length() > 0;
    }

    private TextView title(String text) {
        return ShopUi.title(this, text);
    }

    private TextView section(String text) {
        TextView view = muted(text);
        view.setTypeface(Typeface.DEFAULT_BOLD);
        view.setPadding(0, dp(24), 0, dp(8));
        return view;
    }

    private TextView muted(String text) {
        return ShopUi.muted(this, text);
    }

    private TextView topBarTextButton(String text, int color) {
        return ShopUi.topBarTextButton(this, text, color);
    }

    private TextView card(String title, String detail) {
        return ShopUi.card(this, title, detail);
    }

    private EditText inputField(String hint, String value) {
        return ShopUi.inputField(this, hint, value);
    }

    private EditText underlineInputField(String hint, String value) {
        EditText editText = new EditText(this);
        editText.setHint(hint);
        editText.setText(value);
        editText.setSingleLine(true);
        editText.setPadding(0, dp(8), 0, dp(6));
        GradientDrawable line = new GradientDrawable();
        line.setColor(Color.TRANSPARENT);
        line.setStroke(1, Color.rgb(209, 213, 219));
        editText.setBackground(line);
        return editText;
    }

    private LinearLayout textFilterRow(String label, String value) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(0, 0, 0, 0);
        TextView left = muted(label);
        row.addView(left, new LinearLayout.LayoutParams(0, -1, 1));
        TextView right = strong(value);
        right.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        row.addView(right, new LinearLayout.LayoutParams(0, -1, 1));
        return row;
    }

    private GradientDrawable rounded(int color, int radius) {
        return ShopUi.rounded(color, radius);
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
        return ShopUi.dp(this, value);
    }

    private void configureSystemBars() {
        Window window = getWindow();
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        manualWindowInsets = Build.VERSION.SDK_INT >= 35;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            window.setDecorFitsSystemWindows(!manualWindowInsets);
        }
        window.setStatusBarColor(BG_COLOR);
        window.setNavigationBarColor(BG_COLOR);
        int systemUi = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            systemUi |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
        }
        window.getDecorView().setSystemUiVisibility(systemUi);
    }

    private void bindRootWindowInsets() {
        if (root == null || !manualWindowInsets || Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
            return;
        }
        root.setOnApplyWindowInsetsListener((view, insets) -> {
            int safeTypes = WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout();
            android.graphics.Insets safeInsets = insets.getInsets(safeTypes);
            android.graphics.Insets imeInsets = insets.getInsets(WindowInsets.Type.ime());
            int bottomInset = Math.max(safeInsets.bottom, imeInsets.bottom);
            applyRootInsets(safeInsets.top, bottomInset);
            return insets;
        });
        root.post(root::requestApplyInsets);
    }

    private void applyRootInsets(int topInset, int bottomInset) {
        if (root == null) {
            return;
        }
        boolean changed = appliedTopInset != topInset || appliedBottomInset != bottomInset;
        appliedTopInset = topInset;
        appliedBottomInset = bottomInset;
        root.setPadding(0, 0, 0, 0);
        applyStatusBarScrim(topInset);
        applyContentInsets(topInset, bottomInset);
        if (changed && "chat".equals(activePage) && chatScroll != null) {
            chatScroll.post(this::scrollBottom);
        }
    }

    private void applyStatusBarScrim(int topInset) {
        if (statusBarScrim == null) {
            return;
        }
        int safeTop = stableTopInset(topInset);
        ViewGroup.LayoutParams params = statusBarScrim.getLayoutParams();
        if (params.height != safeTop) {
            params.height = safeTop;
            statusBarScrim.setLayoutParams(params);
        }
        statusBarScrim.bringToFront();
    }

    private void applyContentInsets(int topInset, int bottomInset) {
        View target = chatViewport != null ? chatViewport : content;
        if (target == null) {
            return;
        }
        int safeTop = stableTopInset(topInset);
        int safeBottom = Math.max(0, bottomInset);
        applyStatusBarScrim(topInset);
        if ("chat".equals(activePage)) {
            target.setPadding(0, safeTop, 0, 0);
            applyComposerBottomInset(safeBottom);
        } else {
            target.setPadding(0, safeTop, 0, safeBottom);
        }
    }

    private int stableTopInset(int topInset) {
        int safeTop = Math.max(0, topInset);
        if (!manualWindowInsets) {
            return safeTop;
        }
        int statusHeight = statusBarHeight();
        if (statusHeight <= 0) {
            statusHeight = dp(24);
        }
        return Math.max(safeTop, statusHeight);
    }

    private void requestRootInsets() {
        if (root == null || !manualWindowInsets || Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
            return;
        }
        root.requestApplyInsets();
    }

    
    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) {
            requestRootInsets();
        }
    }

    private void applyComposerBottomInset(int bottomInset) {
        if (composerOuter == null) {
            return;
        }
        composerOuter.setPadding(dp(14), dp(6), dp(14), dp(8) + Math.max(0, bottomInset));
    }

    @Override
    protected void onPause() {
        persistActiveAssistantDraft("partial");
        super.onPause();
    }

    @Override
    protected void onDestroy() {
        voiceController.destroy();
        super.onDestroy();
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

    private static class HistoricalMessageRenderContext {
        final LinearLayout parent;
        final String copyMarkdown;
        final Set<String> renderedProductIds = new HashSet<>();
        LinearLayout bubble;

        HistoricalMessageRenderContext(LinearLayout parent, String copyMarkdown) {
            this.parent = parent;
            this.copyMarkdown = copyMarkdown == null ? "" : copyMarkdown;
        }
    }

    private static class RenderedMarkdown {
        final String visibleMarkdown;
        final JSONArray toolCalls;
        final JSONArray itemIds;

        RenderedMarkdown(String visibleMarkdown, JSONArray toolCalls, JSONArray itemIds) {
            this.visibleMarkdown = visibleMarkdown == null ? "" : visibleMarkdown;
            this.toolCalls = toolCalls == null ? new JSONArray() : toolCalls;
            this.itemIds = itemIds == null ? new JSONArray() : itemIds;
        }
    }
}
