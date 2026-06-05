package com.xzxg.shop;

import android.app.Activity;
import android.app.AlertDialog;
import android.Manifest;
import android.content.ClipData;
import android.content.ClipboardManager;
import android.content.ActivityNotFoundException;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.database.Cursor;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Path;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.provider.MediaStore;
import android.provider.OpenableColumns;
import android.speech.RecognitionListener;
import android.speech.RecognizerIntent;
import android.speech.SpeechRecognizer;
import android.util.Log;
import android.util.Patterns;
import android.view.ViewGroup;
import android.view.Gravity;
import android.view.MotionEvent;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowManager;
import android.view.animation.Animation;
import android.view.animation.TranslateAnimation;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.CheckBox;
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
import android.text.InputType;
import android.text.TextWatcher;

import org.json.JSONArray;
import org.json.JSONObject;

import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.net.URL;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Date;
import java.util.Deque;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Set;
import java.util.UUID;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class MainActivity extends Activity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private static final int REQUEST_PICK_IMAGE = 3101;
    private static final int REQUEST_PICK_FILE = 3102;
    private static final int REQUEST_RECORD_AUDIO = 3103;
    private static final int REQUEST_PICK_AVATAR = 3104;
    private static final int REQUEST_SPEECH_INPUT = 3105;
    private static final int PRODUCT_PAGE_SIZE = 20;
    private static final int HISTORY_PAGE_SIZE = 20;
    private static final boolean DEBUG_AGENT_BLOCKS = true;
    private static final String TAG = "XzxgShop";
    private SessionStore sessionStore;
    private LocalChatStore chatStore;
    private ApiClient api;
    private ImageLoader imageLoader;
    private FrameLayout root;
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
    private int lastImeBottom;
    private boolean wasChatAtBottomBeforeIme = true;
    private boolean voiceMode;
    private SpeechRecognizer speechRecognizer;
    private String lastPartialSpeech = "";
    private boolean voiceResultSent;
    private SpeechMode speechMode = SpeechMode.AUTO;
    private SpeechRealtimeClient speechRealtimeClient;
    private PcmRecorder pcmRecorder;
    private boolean realtimeVoiceActive;
    private boolean realtimeVoiceEnding;
    private boolean realtimeVoiceFinalSent;
    private boolean realtimeRecorderStarted;
    private String realtimePartialText = "";
    private long realtimeVoiceStartedAt;
    private Handler voiceHandler = new Handler(Looper.getMainLooper());
    private FrameLayout drawerLayer;
    private LinearLayout drawerPanel;
    private LinearLayout drawerHistoryList;
    private EditText drawerSearchInput;
    private ScrollView drawerHistoryScroll;
    private int drawerHistoryOffset;
    private boolean drawerHistoryHasMore = true;
    private boolean drawerHistoryLoading;
    private boolean drawerHistoryLocallyMutated;
    private String drawerHistoryQuery = "";
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
    private boolean hasReceivedThinkingDelta;
    private boolean loggedMissingThinkingDelta;
    private final List<String> pendingThinkingStatuses = new ArrayList<>();
    private JSONArray activeFollowups = new JSONArray();
    private View activeFollowupsView;
    private boolean streaming;
    private String activePage = "chat";
    private String lastProductKeyword = "";
    private String lastCategoryId = "";
    private String lastCategoryName = "全部";
    private JSONArray cachedCategoryTree = new JSONArray();
    private ScrollView productsScroll;
    private LinearLayout productsList;
    private int productsScrollY;
    private boolean restoreProductsScroll;
    private boolean loadingProducts;
    private ProductListState currentProductState;
    private String activeProductTab = "list";
    private FrameLayout productCartFab;
    private TextView productCartBadge;
    private int cartItemCountCache;
    private final Map<String, ProductListState> productListCache = new HashMap<>();
    private final Map<String, JSONObject> productDetailCache = new HashMap<>();
    private final Set<String> activeRenderedProductIds = new HashSet<>();
    private final Map<String, List<PendingAttachment>> pendingAttachmentsBySession = new HashMap<>();
    private final Object sessionSyncLock = new Object();
    private final Deque<SessionSyncJob> sessionSyncQueue = new ArrayDeque<>();
    private boolean sessionSyncRunning;
    private String activeRunId = "";
    private ApiClient.StreamCall activeStreamCall;
    private String activeStreamLocalSessionId = "";
    private String activeStreamServerSessionId = "";
    private String activeStreamTitle = "";
    private boolean stopRequested;
    private final Deque<Runnable> backStack = new ArrayDeque<>();
    private Runnable editProfileBackAction;
    private Toast activeToast;
    private byte[] pendingAvatarBytes;
    private String pendingAvatarMime = "image/jpeg";
    private Bitmap pendingAvatarPreview;
    private Runnable pendingAvatarChanged;

    private static class PendingAttachment {
        Uri uri;
        String name;
        String mimeType;
        String type;
        long size;

        PendingAttachment(Uri uri, String name, String mimeType, String type, long size) {
            this.uri = uri;
            this.name = name;
            this.mimeType = mimeType;
            this.type = type;
            this.size = size;
        }
    }

    private static class UploadingMessageViewState {
        LinearLayout container;
        TextView statusText;
        final List<View> attachmentViews = new ArrayList<>();
        boolean failed;
    }

    private String currentAttachmentSessionKey() {
        return localSessionId == null || localSessionId.isEmpty() ? "local_default" : localSessionId;
    }

    private List<PendingAttachment> currentPendingAttachments() {
        String key = currentAttachmentSessionKey();
        List<PendingAttachment> list = pendingAttachmentsBySession.get(key);
        if (list == null) {
            list = new ArrayList<>();
            pendingAttachmentsBySession.put(key, list);
        }
        return list;
    }

    private JSONArray localAttachmentPlaceholders(List<PendingAttachment> attachments) {
        JSONArray items = new JSONArray();
        if (attachments == null) {
            return items;
        }
        for (PendingAttachment attachment : attachments) {
            JSONObject item = new JSONObject();
            try {
                item.put("name", attachment.name == null ? "attachment" : attachment.name);
                item.put("mime_type", attachment.mimeType == null ? "application/octet-stream" : attachment.mimeType);
                item.put("type", attachment.type == null ? "file" : attachment.type);
                item.put("size", Math.max(0, attachment.size));
                item.put("local_uri", attachment.uri == null ? "" : attachment.uri.toString());
            } catch (Exception ignored) {
            }
            items.put(item);
        }
        return items;
    }

    private static class ProductListState {
        JSONArray items = new JSONArray();
        int nextPage = 1;
        boolean hasMore = true;
        boolean loaded;
        int scrollY;
    }

    private static class ThinkingViewState {
        LinearLayout container;
        TextView header;
        LinearLayout detail;
        boolean expanded = true;
        boolean completed;
        boolean segmentSaved;
        boolean userToggled;
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
        sessionStore = new SessionStore(this);
        chatStore = new LocalChatStore(this);
        api = new ApiClient(sessionStore);
        imageLoader = new ImageLoader();
        createFreshLocalSession();
        renderChatHome();
        loadHomeCopy();
        validateStoredToken();
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
        if ("edit_profile".equals(activePage)) {
            Runnable action = editProfileBackAction == null ? () -> renderSettings() : editProfileBackAction;
            action.run();
            return;
        }
        if ("help".equals(activePage) || "about".equals(activePage) || "account".equals(activePage) || "avatar".equals(activePage) || "advanced_settings".equals(activePage)) {
            renderSettings();
            return;
        }
        if (!backStack.isEmpty()) {
            Runnable action = backStack.pop();
            action.run();
            return;
        }
        if (isPrimaryPage(activePage)) {
            if (sessionStore.token().isEmpty()) {
                renderLoginPage();
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

    private void baseScreen() {
        if (root != null) {
            root.setOnApplyWindowInsetsListener(null);
        }
        chatViewport = null;
        lastImeBottom = 0;
        wasChatAtBottomBeforeIme = true;
        root = new FrameLayout(this);
        productCartFab = null;
        productCartBadge = null;
        root.setBackgroundColor(BG_COLOR);
        root.setClipChildren(false);
        root.setClipToPadding(false);
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setClipChildren(false);
        content.setClipToPadding(false);
        content.setPadding(0, 0, 0, 0);
        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        setContentView(root);
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
        bindImeInsets();
        root.requestApplyInsets();
    }

    private void bindImeInsets() {
        root.setOnApplyWindowInsetsListener((view, insets) -> {
            int imeBottom = 0;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                imeBottom = insets.getInsets(WindowInsets.Type.ime()).bottom;
            }
            applyChatImeInset(imeBottom);
            return insets;
        });
    }

    private void applyChatImeInset(int imeBottom) {
        if (chatViewport == null) {
            return;
        }
        boolean imeWillShow = imeBottom > 0;
        boolean imeWasHidden = lastImeBottom == 0;
        if (imeWillShow && imeWasHidden) {
            wasChatAtBottomBeforeIme = isChatScrolledToBottom();
        }
        ViewGroup.LayoutParams currentParams = chatViewport.getLayoutParams();
        if (!(currentParams instanceof FrameLayout.LayoutParams)) {
            return;
        }
        FrameLayout.LayoutParams params = (FrameLayout.LayoutParams) currentParams;
        if (params.height != FrameLayout.LayoutParams.MATCH_PARENT || params.bottomMargin != imeBottom) {
            params.height = FrameLayout.LayoutParams.MATCH_PARENT;
            params.bottomMargin = imeBottom;
            chatViewport.setLayoutParams(params);
        }
        lastImeBottom = imeBottom;
        if (imeWillShow && wasChatAtBottomBeforeIme) {
            chatViewport.post(this::scrollBottom);
        }
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
                renderLoginPage();
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
        LinearLayout toolbar = new LinearLayout(this);
        toolbar.setGravity(Gravity.CENTER_VERTICAL);
        toolbar.setPadding(dp(16), 0, dp(16), 0);
        toolbar.setBackgroundColor(BG_COLOR);
        toolbar.setClickable(true);
        toolbar.setElevation(dp(1));

        TextView back = new TextView(this);
        back.setText("‹");
        back.setTextSize(30);
        back.setTextColor(Color.BLACK);
        back.setGravity(Gravity.CENTER);
        back.setIncludeFontPadding(false);
        back.setClickable(true);
        back.setBackground(new ColorDrawable(Color.TRANSPARENT));
        back.setOnClickListener(v -> {
            hideKeyboard();
            if (backAction != null) {
                backAction.run();
            } else {
                onBackPressed();
            }
        });
        toolbar.addView(back, new LinearLayout.LayoutParams(dp(40), dp(40)));

        TextView title = new TextView(this);
        title.setText(titleText);
        title.setTextSize(18);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        title.setTextColor(Color.rgb(20, 24, 30));
        title.setGravity(Gravity.CENTER_VERTICAL);
        title.setPadding(dp(10), 0, dp(8), 0);
        toolbar.addView(title, new LinearLayout.LayoutParams(0, -1, 1));

        SpaceView right = new SpaceView(this);
        toolbar.addView(right, new LinearLayout.LayoutParams(dp(40), dp(40)));
        return toolbar;
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
            }
            return false;
        });

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
                wasChatAtBottomBeforeIme = isChatScrolledToBottom();
                input.postDelayed(() -> {
                    if (input != null && input.hasFocus() && wasChatAtBottomBeforeIme) {
                        scrollBottom();
                    }
                }, 220);
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
            if (streaming) {
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
        if (streaming) {
            setActionButtonText("■");
        } else if (!text.isEmpty() || !currentPendingAttachments().isEmpty()) {
            setActionButtonText("➤");
        } else if (voiceMode) {
            setActionButtonText(realtimeVoiceEnding ? "…" : "■");
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
        JSONArray attachments = new JSONArray();
        hideAttachmentPanel();
        if (attachmentSnapshot.isEmpty()) {
            sendMessage(text, attachments);
            return;
        }
        if (sessionStore.token().isEmpty()) {
            sendMessage(text, localAttachmentPlaceholders(attachmentSnapshot));
            List<PendingAttachment> current = pendingAttachmentsBySession.get(attachmentSessionKey);
            if (current != null) {
                current.removeAll(attachmentSnapshot);
            }
            renderAttachmentBuffer();
            updateInputActionButtonState();
            return;
        }
        UploadingMessageViewState uploadingState = addUploadingUserMessageBubble(text, attachmentSnapshot);
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
                    if (attachment.size > 10 * 1024 * 1024) {
                        throw new RuntimeException("单个附件不能超过 10MB");
                    }
                    byte[] data = readAttachmentBytes(attachment.uri);
                    if (data.length == 0) {
                        throw new RuntimeException("附件内容为空，无法发送");
                    }
                    JSONObject item = api.uploadAttachment(attachment.name, attachment.mimeType, attachment.type, data);
                    try {
                        item.put("mime_type", attachment.mimeType == null ? "application/octet-stream" : attachment.mimeType);
                        item.put("size", Math.max(0, attachment.size));
                        item.put("local_uri", attachment.uri == null ? "" : attachment.uri.toString());
                    } catch (Exception ignored) {
                    }
                    uploaded.put(item);
                    runOnUiThread(() -> updateUploadingAttachmentState(uploadingState, index, "上传完成"));
                }
                runOnUiThread(() -> {
                    List<PendingAttachment> current = pendingAttachmentsBySession.get(attachmentSessionKey);
                    if (current != null) {
                        current.removeAll(attachmentSnapshot);
                    }
                    renderAttachmentBuffer();
                    if (actionButton != null) {
                        actionButton.setEnabled(true);
                    }
                    finishUploadingUserMessageBubble(uploadingState, text, uploaded);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (input != null && !voiceMode && input.getText().toString().trim().isEmpty() && text != null && !text.trim().isEmpty()) {
                        input.setText(text);
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
                "application/pdf",
                "application/msword",
                "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
                "application/vnd.ms-excel",
                "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
                "application/vnd.ms-powerpoint",
                "application/vnd.openxmlformats-officedocument.presentationml.presentation",
                "text/*"
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
        if (requestCode == REQUEST_PICK_AVATAR) {
            try {
                pendingAvatarBytes = squareAvatarBytes(uri);
                pendingAvatarMime = "image/jpeg";
                if (pendingAvatarPreview != null) {
                    pendingAvatarPreview.recycle();
                }
                pendingAvatarPreview = BitmapFactory.decodeByteArray(pendingAvatarBytes, 0, pendingAvatarBytes.length);
                if (pendingAvatarChanged != null) {
                    pendingAvatarChanged.run();
                }
            } catch (Exception error) {
                toastLine("头像处理失败：" + error.getMessage());
            }
            return;
        }
        PendingAttachment attachment = attachmentFromUri(uri, requestCode == REQUEST_PICK_IMAGE);
        currentPendingAttachments().add(attachment);
        renderAttachmentBuffer();
        hideAttachmentPanel();
        updateInputActionButtonState();
    }

    private PendingAttachment attachmentFromUri(Uri uri, boolean forceImage) {
        String name = "attachment";
        long size = 0;
        String mime = getContentResolver().getType(uri);
        try (Cursor cursor = getContentResolver().query(uri, null, null, null, null)) {
            if (cursor != null && cursor.moveToFirst()) {
                int nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME);
                int sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE);
                if (nameIndex >= 0) {
                    name = cursor.getString(nameIndex);
                }
                if (sizeIndex >= 0) {
                    size = cursor.getLong(sizeIndex);
                }
            }
        } catch (Exception ignored) {
        }
        String type = forceImage || (mime != null && mime.startsWith("image/")) ? "image" : "file";
        return new PendingAttachment(uri, name, mime == null ? "application/octet-stream" : mime, type, size);
    }

    private byte[] readAttachmentBytes(Uri uri) throws Exception {
        try (java.io.InputStream in = getContentResolver().openInputStream(uri);
             java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream()) {
            if (in == null) {
                throw new RuntimeException("无法读取附件");
            }
            byte[] buffer = new byte[8192];
            int read;
            int total = 0;
            while ((read = in.read(buffer)) != -1) {
                total += read;
                if (total > 10 * 1024 * 1024) {
                    throw new RuntimeException("单个附件不能超过 10MB");
                }
                out.write(buffer, 0, read);
            }
            return out.toByteArray();
        }
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
        return speechMode == null ? SpeechMode.AUTO : speechMode;
    }

    private void startRealtimeSpeech() {
        voiceMode = true;
        voiceResultSent = false;
        realtimeVoiceActive = true;
        realtimeVoiceEnding = false;
        realtimeVoiceFinalSent = false;
        realtimeRecorderStarted = false;
        realtimePartialText = "";
        realtimeVoiceStartedAt = System.currentTimeMillis();
        if (input != null) {
            input.setText("");
            input.setHint("正在连接语音识别...");
            input.clearFocus();
        }
        hideKeyboard();
        updateInputActionButtonState();
        pcmRecorder = new PcmRecorder();
        speechRealtimeClient = new SpeechRealtimeClient(sessionStore, new SpeechRealtimeClient.Listener() {
            @Override
            public void onReady() {
                runOnUiThread(() -> {
                    if (!realtimeVoiceActive) {
                        return;
                    }
                    if (input != null) {
                        input.setHint("正在聆听...");
                    }
                    try {
                        realtimeRecorderStarted = true;
                        pcmRecorder.start(new PcmRecorder.FrameListener() {
                            @Override
                            public void onFrame(byte[] frame) {
                                SpeechRealtimeClient client = speechRealtimeClient;
                                if (realtimeVoiceActive && client != null) {
                                    client.sendAudio(frame);
                                }
                            }

                            @Override
                            public void onError(Exception error) {
                                runOnUiThread(() -> handleRealtimeSpeechError("speech_recognition_failed", "录音失败，请重试"));
                            }
                        });
                    } catch (Exception error) {
                        handleRealtimeSpeechError("speech_recognition_failed", "录音失败，请重试");
                    }
                });
            }

            @Override
            public void onPartial(String text) {
                runOnUiThread(() -> {
                    realtimePartialText = text == null ? "" : text.trim();
                    if (input != null && realtimeVoiceActive && !realtimePartialText.isEmpty()) {
                        input.setHint(realtimePartialText);
                    }
                });
            }

            @Override
            public void onFinal(String text) {
                runOnUiThread(() -> sendRealtimeSpeechFinal(text));
            }

            @Override
            public void onError(String code, String message) {
                runOnUiThread(() -> handleRealtimeSpeechError(code, message));
            }

            @Override
            public void onClosed() {
            }
        });
        speechRealtimeClient.connect();
        voiceHandler.postDelayed(() -> {
            if (realtimeVoiceActive && !realtimeRecorderStarted && !realtimeVoiceEnding) {
                handleRealtimeSpeechError("speech_network_error", "语音识别连接超时");
            }
        }, 8000);
        voiceHandler.postDelayed(() -> {
            if (realtimeVoiceActive && !realtimeVoiceEnding) {
                finishRealtimeVoice(false);
            }
        }, 60000);
    }

    private void startAndroidInlineSpeech() {
        if (!SpeechRecognizer.isRecognitionAvailable(this)) {
            startSpeechRecognizerActivity();
            return;
        }
        voiceMode = true;
        voiceResultSent = false;
        lastPartialSpeech = "";
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
        Intent intent = new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH);
        intent.putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM);
        intent.putExtra(RecognizerIntent.EXTRA_LANGUAGE, Locale.CHINA.toString());
        if (intent.resolveActivity(getPackageManager()) == null) {
            toastLine("当前设备不支持语音输入");
            finishVoiceMode();
            return;
        }
        voiceMode = true;
        voiceResultSent = false;
        lastPartialSpeech = "";
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
        if (realtimeVoiceActive) {
            finishRealtimeVoice(true);
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

    private void finishRealtimeVoice(boolean userStop) {
        if (!realtimeVoiceActive || realtimeVoiceEnding) {
            return;
        }
        long duration = System.currentTimeMillis() - realtimeVoiceStartedAt;
        if (userStop && duration < 800) {
            toastLine("说话时间太短");
            cancelRealtimeVoice();
            return;
        }
        realtimeVoiceEnding = true;
        if (pcmRecorder != null) {
            pcmRecorder.stop();
        }
        if (speechRealtimeClient != null) {
            speechRealtimeClient.sendEnd();
        }
        if (input != null) {
            input.setHint("正在识别语音...");
        }
        updateInputActionButtonState();
        voiceHandler.postDelayed(() -> {
            if (realtimeVoiceActive && realtimeVoiceEnding && !realtimeVoiceFinalSent) {
                sendRealtimeSpeechFinal(realtimePartialText);
            }
        }, 3000);
    }

    private void cancelRealtimeVoice() {
        if (pcmRecorder != null) {
            pcmRecorder.stop();
            pcmRecorder = null;
        }
        if (speechRealtimeClient != null) {
            speechRealtimeClient.cancel();
            speechRealtimeClient = null;
        }
        cleanupRealtimeVoice();
    }

    private void cleanupRealtimeVoice() {
        realtimeVoiceActive = false;
        realtimeVoiceEnding = false;
        realtimeRecorderStarted = false;
        realtimePartialText = "";
        voiceMode = false;
        voiceHandler.removeCallbacksAndMessages(null);
        if (pcmRecorder != null) {
            pcmRecorder.stop();
            pcmRecorder = null;
        }
        if (speechRealtimeClient != null) {
            speechRealtimeClient.close();
            speechRealtimeClient = null;
        }
        if (input != null) {
            input.setHint("输入问题或直接发送...");
            input.setGravity(Gravity.CENTER_VERTICAL);
        }
        updateInputActionButtonState();
    }

    private void sendRealtimeSpeechFinal(String text) {
        if (realtimeVoiceFinalSent) {
            return;
        }
        String value = text == null ? "" : text.trim();
        if (value.isEmpty()) {
            value = realtimePartialText == null ? "" : realtimePartialText.trim();
        }
        if (value.isEmpty()) {
            toastLine("没有识别到内容");
            cleanupRealtimeVoice();
            return;
        }
        realtimeVoiceFinalSent = true;
        cleanupRealtimeVoice();
        fillRecognizedSpeechToInput(value);
    }

    private void handleRealtimeSpeechError(String code, String message) {
        if (!realtimeVoiceActive || realtimeVoiceFinalSent) {
            return;
        }
        cleanupRealtimeVoice();
        if ("speech_not_enabled".equals(code) || "speech_network_error".equals(code)) {
            toastLine("当前语音识别不可用");
            return;
        }
        toastLine((message == null || message.trim().isEmpty()) ? "语音识别失败，请重试" : message);
    }

    private void startSpeechRecognition() {
        if (speechRecognizer != null) {
            speechRecognizer.destroy();
        }
        speechRecognizer = SpeechRecognizer.createSpeechRecognizer(this);
        speechRecognizer.setRecognitionListener(new RecognitionListener() {
            @Override public void onReadyForSpeech(Bundle params) {}
            @Override public void onBeginningOfSpeech() {}
            @Override public void onRmsChanged(float rmsdB) {}
            @Override public void onBufferReceived(byte[] buffer) {}
            @Override public void onEndOfSpeech() {}
            @Override
            public void onPartialResults(Bundle partialResults) {
                ArrayList<String> texts = partialResults.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);
                if (texts != null && !texts.isEmpty()) {
                    lastPartialSpeech = texts.get(0);
                }
            }
            @Override public void onEvent(int eventType, Bundle params) {}

            @Override
            public void onError(int error) {
                runOnUiThread(() -> {
                    if (voiceResultSent) {
                        return;
                    }
                    if ((error == SpeechRecognizer.ERROR_NO_MATCH || error == SpeechRecognizer.ERROR_SPEECH_TIMEOUT)
                            && lastPartialSpeech != null && !lastPartialSpeech.trim().isEmpty()) {
                        sendRecognizedSpeech(lastPartialSpeech);
                        return;
                    }
                    toastLine(speechErrorText(error));
                    finishVoiceMode();
                });
            }

            @Override
            public void onResults(Bundle results) {
                if (voiceResultSent) {
                    return;
                }
                ArrayList<String> texts = results.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);
                if (texts == null || texts.isEmpty() || texts.get(0).trim().isEmpty()) {
                    toastLine("没有识别到内容");
                    finishVoiceMode();
                    return;
                }
                sendRecognizedSpeech(texts.get(0));
            }
        });
        Intent intent = new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH);
        intent.putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM);
        intent.putExtra(RecognizerIntent.EXTRA_LANGUAGE, Locale.CHINA.toString());
        intent.putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true);
        lastPartialSpeech = "";
        speechRecognizer.startListening(intent);
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
        lastPartialSpeech = "";
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
        lastPartialSpeech = "";
        if (input != null) {
            input.setHint("输入问题或直接发送...");
            input.setGravity(Gravity.CENTER_VERTICAL);
        }
        updateInputActionButtonState();
    }

    private String speechErrorText(int error) {
        if (error == SpeechRecognizer.ERROR_NO_MATCH) {
            return "没有识别到内容";
        }
        if (error == SpeechRecognizer.ERROR_SPEECH_TIMEOUT) {
            return "未检测到语音";
        }
        if (error == SpeechRecognizer.ERROR_NETWORK || error == SpeechRecognizer.ERROR_NETWORK_TIMEOUT) {
            return "语音识别网络异常，请稍后重试";
        }
        return "语音识别失败，请重试";
    }

    private void stopSpeechRecognition() {
        if (speechRecognizer != null) {
            speechRecognizer.stopListening();
        }
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
        useKeyboardOverlay();
        hideKeyboard();
        if (sessionStore.token().isEmpty()) {
            renderLoginPage();
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
        drawer.setPadding(dp(14), dp(10), dp(14), dp(10));
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
        navGroup.addView(drawerNavButton("🏷", "商品", "products", v -> renderProducts()));
        navGroup.addView(drawerNavButton("🛒", "购物车", "cart", v -> renderCart()));
        navGroup.addView(drawerNavButton("📦", "订单", "orders", v -> renderOrders()));
        navGroup.addView(drawerNavButton("券", "优惠券", "coupons", v -> renderCoupons()));
        navGroup.addView(drawerNavButton("促", "活动", "promotions", v -> renderPromotions()));
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
        drawerHistoryLocallyMutated = false;
        int historyBottomPadding = sessionStore.showDrawerReturnChat() ? dp(72) : dp(16);
        historyList.setPadding(0, dp(12), 0, historyBottomPadding);
        resetHistoryPaging("");
        renderHistoryList(historyList, loadHistoryPage("", 0));
        loadRemoteHistoryPage("", 0, false);
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
                resetHistoryPaging(query);
                renderHistoryList(historyList, loadHistoryPage(query, 0));
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
        drawerLayer.addView(drawer, drawerParams);
        root.addView(drawerLayer, new FrameLayout.LayoutParams(-1, -1));
        drawerLayer.animate().alpha(1f).setDuration(160).start();
        TranslateAnimation slideIn = new TranslateAnimation(-panelWidth, 0, 0, 0);
        slideIn.setDuration(220);
        drawer.startAnimation(slideIn);
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
                JSONArray sessions = api.sessionsPage(1, HISTORY_PAGE_SIZE).items;
                for (int i = 0; i < sessions.length(); i++) {
                    JSONObject item = sessions.optJSONObject(i);
                    if (item == null) {
                        continue;
                    }
                    chatStore.upsertRemoteSession(item.optString("session_id", ""), item.optString("title", "导购会话"), item.optString("summary", ""), remoteSessionTime(item));
                }
            } catch (Exception error) {
                handleApiError(error);
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
        if (activeStreamCall != null) {
            activeStreamCall.cancel();
        }
        sessionStore.clearAuth();
        serverSessionId = "";
        cartItemCountCache = 0;
        toastLine("登录已过期，请重新登录");
        if ("cart".equals(activePage) || "orders".equals(activePage) || "settings".equals(activePage) || "account".equals(activePage) || "advanced_settings".equals(activePage)) {
            renderLoginPage();
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
        for (LocalChatStore.SessionSummary item : histories) {
            if (item != null) {
                historyList.addView(historyButton(item));
            }
        }
    }

    private void resetHistoryPaging(String query) {
        drawerHistoryQuery = query == null ? "" : query.trim();
        drawerHistoryOffset = 0;
        drawerHistoryHasMore = true;
        drawerHistoryLoading = false;
    }

    private void loadRemoteHistoryPage(String query, int offset, boolean append) {
        if (drawerHistoryList == null) {
            return;
        }
        String requestedQuery = query == null ? "" : query.trim();
        if (sessionStore.token().isEmpty()) {
            List<LocalChatStore.SessionSummary> page = loadHistoryPage(requestedQuery, offset);
            if (append) {
                appendHistoryList(drawerHistoryList, page);
            } else {
                renderHistoryList(drawerHistoryList, page);
            }
            return;
        }
        drawerHistoryLoading = true;
        int page = offset / HISTORY_PAGE_SIZE + 1;
        new Thread(() -> {
            try {
                ApiClient.SessionPage remotePage = requestedQuery.isEmpty()
                        ? api.sessionsPage(page, HISTORY_PAGE_SIZE)
                        : api.searchSessionsPage(requestedQuery, page, HISTORY_PAGE_SIZE);
                List<LocalChatStore.SessionSummary> summaries = upsertRemoteSearchResults(remotePage.items);
                runOnUiThread(() -> {
                    if (drawerHistoryList == null) {
                        return;
                    }
                    String currentQuery = drawerSearchInput == null ? "" : drawerSearchInput.getText().toString().trim();
                    if (!requestedQuery.equals(currentQuery)) {
                        drawerHistoryLoading = false;
                        return;
                    }
                    drawerHistoryQuery = requestedQuery;
                    drawerHistoryOffset = offset + remotePage.items.length();
                    drawerHistoryHasMore = remotePage.hasMore;
                    if (append) {
                        appendHistoryList(drawerHistoryList, summaries);
                    } else {
                        renderHistoryList(drawerHistoryList, summaries);
                    }
                    drawerHistoryLoading = false;
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (drawerHistoryList != null) {
                        List<LocalChatStore.SessionSummary> pageItems = loadHistoryPage(requestedQuery, offset);
                        if (append) {
                            appendHistoryList(drawerHistoryList, pageItems);
                        } else {
                            renderHistoryList(drawerHistoryList, pageItems);
                        }
                    }
                    drawerHistoryLoading = false;
                    handleApiError(error);
                });
            }
        }).start();
    }

    private List<LocalChatStore.SessionSummary> loadHistoryPage(String query, int offset) {
        List<LocalChatStore.SessionSummary> page;
        String keyword = query == null ? "" : query.trim();
        if (keyword.isEmpty()) {
            page = chatStore.recentSessionsWithMessages(HISTORY_PAGE_SIZE, offset);
        } else {
            page = chatStore.searchSessionsLocal(keyword, HISTORY_PAGE_SIZE, offset);
        }
        drawerHistoryOffset = offset + (page == null ? 0 : page.size());
        drawerHistoryHasMore = page != null && page.size() >= HISTORY_PAGE_SIZE;
        return page;
    }

    private void maybeLoadMoreHistory(LinearLayout historyList, ScrollView scrollView) {
        if (historyList == null || scrollView == null || drawerHistoryLoading || !drawerHistoryHasMore) {
            return;
        }
        int range = historyList.getHeight() - scrollView.getHeight();
        if (range <= 0 || scrollView.getScrollY() < range - dp(32)) {
            return;
        }
        drawerHistoryLoading = true;
        String query = drawerHistoryQuery;
        int offset = drawerHistoryOffset;
        if (sessionStore.token().isEmpty()) {
            List<LocalChatStore.SessionSummary> next = loadHistoryPage(query, offset);
            appendHistoryList(historyList, next);
            drawerHistoryLoading = false;
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
        resetHistoryPaging(query);
        renderHistoryList(drawerHistoryList, loadHistoryPage(query, 0));
        if (includeRemote) {
            loadRemoteHistoryPage(query, 0, false);
        }
    }

    private List<LocalChatStore.SessionSummary> upsertRemoteSearchResults(JSONArray sessions) {
        ArrayList<LocalChatStore.SessionSummary> results = new ArrayList<>();
        if (sessions == null) {
            return results;
        }
        for (int i = 0; i < sessions.length(); i++) {
            JSONObject item = sessions.optJSONObject(i);
            if (item == null) {
                continue;
            }
            String localId = chatStore.upsertRemoteSession(item.optString("session_id", ""), item.optString("title", "导购会话"), item.optString("summary", ""), remoteSessionTime(item));
            LocalChatStore.SessionSummary summary = chatStore.sessionSummary(localId);
            if (summary != null) {
                results.add(summary);
            }
        }
        return results;
    }

    private void pickAvatar() {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
        intent.setType("image/*");
        startActivityForResult(intent, REQUEST_PICK_AVATAR);
    }

    private byte[] squareAvatarBytes(Uri uri) throws Exception {
        try (InputStream inputStream = getContentResolver().openInputStream(uri)) {
            Bitmap source = BitmapFactory.decodeStream(inputStream);
            if (source == null) {
                throw new IllegalArgumentException("无法读取图片");
            }
            int side = Math.min(source.getWidth(), source.getHeight());
            int left = (source.getWidth() - side) / 2;
            int top = (source.getHeight() - side) / 2;
            Bitmap square = Bitmap.createBitmap(source, left, top, side, side);
            Bitmap scaled = Bitmap.createScaledBitmap(square, 512, 512, true);
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            scaled.compress(Bitmap.CompressFormat.JPEG, 88, out);
            if (scaled != square) {
                scaled.recycle();
            }
            if (square != source) {
                square.recycle();
            }
            source.recycle();
            return out.toByteArray();
        }
    }

    private List<LocalChatStore.SessionSummary> ensureActiveSessionVisible(List<LocalChatStore.SessionSummary> histories) {
        ArrayList<LocalChatStore.SessionSummary> result = new ArrayList<>();
        boolean found = false;
        if (histories != null) {
            for (LocalChatStore.SessionSummary item : histories) {
                if (item == null) {
                    continue;
                }
                if (localSessionId != null && localSessionId.equals(item.localSessionId)) {
                    found = true;
                }
                result.add(item);
            }
        }
        if (!found && localSessionId != null && !localSessionId.isEmpty()) {
            LocalChatStore.SessionSummary active = chatStore.sessionSummary(localSessionId);
            if (active != null) {
                result.add(0, active);
            }
        }
        return result;
    }

    private long remoteSessionTime(JSONObject item) {
        long updated = parseRemoteTime(item.optString("updated_at", ""));
        if (updated > 0) {
            return updated;
        }
        long lastMessage = parseRemoteTime(item.optString("last_message_at", ""));
        if (lastMessage > 0) {
            return lastMessage;
        }
        long created = parseRemoteTime(item.optString("created_at", ""));
        return created > 0 ? created : 1;
    }

    private long parseRemoteTime(String value) {
        if (value == null || value.trim().isEmpty()) {
            return 0;
        }
        String text = value.trim();
        String[] patterns = {
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                "yyyy-MM-dd'T'HH:mm:ssXXX",
                "yyyy-MM-dd'T'HH:mm:ss'Z'"
        };
        for (String pattern : patterns) {
            try {
                SimpleDateFormat format = new SimpleDateFormat(pattern, Locale.US);
                Date date = format.parse(text);
                if (date != null) {
                    return date.getTime();
                }
            } catch (ParseException ignored) {
            }
        }
        return 0;
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

    private String titleFromChatText(String text) {
        String value = text == null ? "" : text.trim().replace('\n', ' ');
        if (value.isEmpty()) {
            return "导购会话";
        }
        return value.length() > 18 ? value.substring(0, 18) + "..." : value;
    }

    private void sendMessage(String text) {
        sendMessage(text, new JSONArray());
    }

    private void sendMessage(String text, JSONArray attachments) {
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
        streaming = true;
        stopRequested = false;
        activeRunId = "";
        activeStreamLocalSessionId = localSessionId;
        updateInputActionButtonState();
        loadingAssistant = addLoadingBubble();
        ensureServerSessionThenStream(messageText, attachments == null ? new JSONArray() : attachments);
    }

    private String attachmentSummary(JSONArray attachments) {
        int count = attachments == null ? 0 : attachments.length();
        return count == 0 ? "" : "[已选择 " + count + " 个附件]";
    }

    private void ensureServerSessionThenStream(String text, JSONArray attachments) {
        if (!serverSessionId.isEmpty()) {
            enqueueSessionSync(localSessionId, serverSessionId, titleFromChatText(text), text);
            stream(text, attachments);
            return;
        }
        String ownerLocalSessionId = localSessionId;
        new Thread(() -> {
            try {
                JSONObject session = api.createSession("Android 新聊天");
                String createdServerSessionId = session.optString("session_id", "");
                chatStore.bindServerSession(ownerLocalSessionId, createdServerSessionId);
                enqueueSessionSync(ownerLocalSessionId, createdServerSessionId, titleFromChatText(text), text);
                runOnUiThread(() -> {
                    if (ownerLocalSessionId.equals(localSessionId)) {
                        serverSessionId = createdServerSessionId;
                    }
                    stream(ownerLocalSessionId, createdServerSessionId, text, attachments);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (isAuthExpired(error)) {
                        handleAuthExpired();
                    } else {
                        Log.e(TAG, "create session failed", error);
                        failStream("创建会话失败：" + readableErrorMessage(error));
                    }
                });
            }
        }).start();
    }

    private void stream(String text, JSONArray attachments) {
        stream(localSessionId, serverSessionId, text, attachments);
    }

    private void stream(String ownerLocalSessionId, String ownerServerSessionId, String text, JSONArray attachments) {
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
        hasReceivedThinkingDelta = false;
        loggedMissingThinkingDelta = false;
        pendingThinkingStatuses.clear();
        activeFollowups = new JSONArray();
        activeFollowupsView = null;
        activeRenderedProductIds.clear();
        activeStreamLocalSessionId = ownerLocalSessionId;
        activeStreamServerSessionId = ownerServerSessionId;
        activeStreamTitle = titleFromChatText(text);
        removeLoadingBubbleIfNeeded();
        String clientMessageId = "android_" + UUID.randomUUID().toString();
        activeStreamCall = api.streamMessage(ownerServerSessionId, clientMessageId, text, attachments, new ApiClient.SseCallback() {
            @Override
            public void onEvent(JSONObject event) {
                runOnUiThread(() -> handleSse(ownerLocalSessionId, event));
            }

            @Override
            public void onError(Throwable error) {
                runOnUiThread(() -> {
                    if (isAuthExpired(error)) {
                        handleAuthExpired();
                        streaming = false;
                        return;
                    }
                    if (!stopRequested) {
                        Log.e(TAG, "stream message failed", error);
                        failStream("发送失败：" + readableErrorMessage(error));
                    }
                });
            }
        });
    }

    private void handleSse(String ownerLocalSessionId, JSONObject event) {
        if (stopRequested || !ownerLocalSessionId.equals(localSessionId)) {
            return;
        }
        String type = event.optString("type");
        if ("message_start".equals(type)) {
            activeRunId = event.optString("run_id", "");
            return;
        }
        if ("duplicate_message".equals(type)) {
            finishDuplicateStream(ownerLocalSessionId, event);
            return;
        }
        if ("status".equals(type)) {
            String statusText = event.optString("text", "正在处理...");
            rememberThinkingStatus(statusText);
            if (activeThinking == null) {
                updateLoadingStatus(statusText);
            }
            return;
        }
        if ("thinking_status".equals(type)) {
            String statusText = event.optString("text", event.optString("status", "正在思考..."));
            rememberThinkingStatus(statusText);
            if (activeThinking == null) {
                updateLoadingStatus(statusText);
            }
            return;
        }
        if ("thinking_delta".equals(type)) {
            handleThinkingDelta(event);
            return;
        }
        if ("text_delta".equals(type)) {
            logMissingThinkingDeltaIfNeeded(event);
            appendAssistant(event.optString("delta"));
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
        MarkdownRenderer.setMarkdown(activeAssistant, rendered.visibleMarkdown);
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
        for (MarkdownPart part : splitMarkdownParts(markdown)) {
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
        flushPendingAssistantForm(true);
        removeLoadingBubbleIfNeeded();
        streaming = false;
        stopRequested = false;
        activeRunId = "";
        activeStreamCall = null;
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
        updateInputActionButtonState();
        toastLine("消息已提交过，正在刷新会话");
        if (activeStreamServerSessionId != null && !activeStreamServerSessionId.isEmpty()) {
            loadRemoteSessionDetail(activeStreamServerSessionId, ownerLocalSessionId);
        }
        activeStreamLocalSessionId = "";
        activeStreamServerSessionId = "";
        activeStreamTitle = "";
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
        if (!visibleMarkdown.trim().isEmpty() || activeAssistantBlocks.length() > 0 || activeFollowups.length() > 0 || activeAssistantSegments.length() > 0) {
            chatStore.saveAssistantTurn(ownerLocalSessionId, visibleMarkdown, activeAssistantBlocks.toString(), activeFollowups.toString(), activeAssistantSegments.toString(), "completed");
        }
        enqueueSessionSync(ownerLocalSessionId, activeStreamServerSessionId, activeStreamTitle, visibleMarkdown);
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
        activeRunId = "";
        activeStreamCall = null;
        activeStreamLocalSessionId = "";
        activeStreamServerSessionId = "";
        activeStreamTitle = "";
        stopRequested = false;
        removeLoadingBubbleIfNeeded();
        streaming = false;
        updateInputActionButtonState();
    }

    private void failStream(String message) {
        String ownerLocalSessionId = activeStreamLocalSessionId == null || activeStreamLocalSessionId.isEmpty() ? localSessionId : activeStreamLocalSessionId;
        String ownerServerSessionId = activeStreamServerSessionId;
        if (loadingAssistant != null) {
            loadingAssistant.setText(message);
            loadingAssistant = null;
        } else {
            addChatSystemLine(message);
        }
        enqueueSessionSync(ownerLocalSessionId, ownerServerSessionId, activeStreamTitle, message);
        streaming = false;
        activeRunId = "";
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
        activeStreamCall = null;
        activeStreamLocalSessionId = "";
        activeStreamServerSessionId = "";
        activeStreamTitle = "";
        stopRequested = false;
        updateInputActionButtonState();
    }

    private void stopActiveStream() {
        if (!streaming) {
            return;
        }
        stopRequested = true;
        if (actionButton != null) {
            actionButton.setEnabled(false);
        }
        updateLoadingStatus("正在停止...");
        String runId = activeRunId;
        if (runId != null && !runId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.cancelAgentRun(runId);
                } catch (Exception ignored) {
                }
            }).start();
        }
        ApiClient.StreamCall call = activeStreamCall;
        if (call != null) {
            call.cancel();
        }
        finishCanceledStream();
    }

    private void finishCanceledStream() {
        String ownerLocalSessionId = activeStreamLocalSessionId == null || activeStreamLocalSessionId.isEmpty() ? localSessionId : activeStreamLocalSessionId;
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
        if (!visibleMarkdown.trim().isEmpty() || activeAssistantBlocks.length() > 0 || activeFollowups.length() > 0 || activeAssistantSegments.length() > 0) {
            chatStore.saveAssistantTurn(ownerLocalSessionId, visibleMarkdown, activeAssistantBlocks.toString(), activeFollowups.toString(), activeAssistantSegments.toString(), "canceled");
        }
        if (ownerLocalSessionId.equals(localSessionId)) {
            addChatSystemLine("已停止生成");
        }
        enqueueSessionSync(ownerLocalSessionId, activeStreamServerSessionId, activeStreamTitle, visibleMarkdown);
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
        activeRunId = "";
        activeStreamCall = null;
        activeStreamLocalSessionId = "";
        activeStreamServerSessionId = "";
        activeStreamTitle = "";
        stopRequested = false;
        streaming = false;
        if (actionButton != null) {
            actionButton.setEnabled(true);
        }
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
        String rawStage = source.optString("stage", "");
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
        String status = source.optString("status", "");
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
        if (items != null) {
            stage.items = items;
        }
        renderThinkingView(thinking);
        if (isThinkingComplete(thinking) && !thinking.completed) {
            completeThinkingView();
        }
    }

    private JSONObject parseThinkingEvent(JSONObject event) {
        if (event == null) {
            return null;
        }
        JSONObject payload = event.optJSONObject("thinking");
        JSONObject source = payload == null ? event : payload;
        JSONObject parsed = new JSONObject();
        try {
            parsed.put("type", "thinking_delta");
            parsed.put("run_id", source.optString("run_id", event.optString("run_id", activeRunId)));
            parsed.put("stage", source.optString("stage", ""));
            parsed.put("status", source.optString("status", ""));
            parsed.put("title", source.optString("title", ""));
            parsed.put("delta", thinkingStageTextFromEvent(source));
            JSONArray items = source.optJSONArray("items");
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
        JSONObject source = payload == null ? event : payload;
        JSONArray items = source.optJSONArray("items");
        String delta = thinkingStageTextFromEvent(source);
        Log.d(TAG,
                "ThinkingEvent " + reason
                        + " type=" + event.optString("type", "")
                        + " run_id=" + source.optString("run_id", event.optString("run_id", activeRunId))
                        + " stage=" + source.optString("stage", "")
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
        Log.w(TAG, "ThinkingEvent missing before text_delta run_id=" + activeRunId + " raw=" + event);
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

    private void startFallbackThinkingTimeline() {
        if (activeThinking != null || chatList == null) {
            return;
        }
        removeLoadingBubbleIfNeeded();
        ThinkingViewState thinking = ensureThinkingView(chatList, true);
        ThinkingStageState userNeed = ensureThinkingStage(thinking, "user_need");
        userNeed.status = "completed";
        String title = activeStreamTitle == null ? "" : activeStreamTitle.trim();
        if (!title.isEmpty()) {
            userNeed.text.append("已理解用户需求：").append(title);
        } else {
            userNeed.text.append("已理解用户本轮导购需求。");
        }

        ThinkingStageState buyerExperience = ensureThinkingStage(thinking, "buyer_experience");
        buyerExperience.status = "completed";
        buyerExperience.text.append(fallbackBuyerExperienceText());

        ThinkingStageState answerSummary = ensureThinkingStage(thinking, "answer_summary");
        answerSummary.status = "running";
        answerSummary.text.append("正在整理回答。");

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
        thinking.container.setPadding(dp(24), dp(6), dp(20), dp(8));
        thinking.header = new TextView(this);
        thinking.header.setTextSize(15);
        thinking.header.setTextColor(Color.rgb(75, 85, 99));
        thinking.header.setGravity(Gravity.CENTER_VERTICAL);
        thinking.header.setPadding(0, dp(6), 0, dp(6));
        thinking.header.setOnClickListener(v -> {
            int beforeScrollY = chatScroll == null ? 0 : chatScroll.getScrollY();
            thinking.expanded = !thinking.expanded;
            thinking.userToggled = true;
            renderThinkingView(thinking, true);
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
        initializeThinkingStages(thinking);
        if (active) {
            activeThinking = thinking;
        }
        return thinking;
    }

    private void initializeThinkingStages(ThinkingViewState thinking) {
        if (thinking == null) {
            return;
        }
        for (String key : thinkingStageOrder()) {
            ensureThinkingStage(thinking, key);
        }
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
        String[] order = thinkingStageOrder();
        for (String key : order) {
            ThinkingStageState stage = ensureThinkingStage(thinking, key);
            if (key.equals(stageKey)) {
                if (!"completed".equals(stage.status) && !"failed".equals(stage.status)) {
                    stage.status = "running";
                }
                return;
            }
            if (!"failed".equals(stage.status)) {
                stage.status = "completed";
            }
        }
    }

    private void renderThinkingView(ThinkingViewState thinking) {
        renderThinkingView(thinking, false);
    }

    private void renderThinkingView(ThinkingViewState thinking, boolean preserveScroll) {
        if (thinking == null || thinking.header == null || thinking.detail == null) {
            return;
        }
        String headerText = thinking.completed ? "✦  已完成思考 " : "✦  正在思考 ";
        thinking.header.setText(headerText + (thinking.expanded ? "▴" : "▾"));
        thinking.detail.setVisibility(thinking.expanded ? View.VISIBLE : View.GONE);
        thinking.detail.removeAllViews();
        if (!thinking.expanded) {
            if (!preserveScroll) {
                scrollBottom();
            }
            return;
        }
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
        if (!preserveScroll) {
            scrollBottom();
        }
    }

    private void ensureThinkingStageViews(ThinkingStageState stage) {
        if (stage.row != null) {
            return;
        }
        stage.row = new LinearLayout(this);
        stage.row.setOrientation(LinearLayout.HORIZONTAL);
        stage.row.setPadding(0, dp(6), 0, dp(10));
        stage.markerView = new TextView(this);
        stage.markerView.setTextSize(14);
        stage.markerView.setGravity(Gravity.TOP | Gravity.CENTER_HORIZONTAL);
        stage.markerView.setLineSpacing(1, 1);
        stage.markerView.setTextColor(Color.rgb(156, 163, 175));
        stage.row.addView(stage.markerView, new LinearLayout.LayoutParams(dp(28), ViewGroup.LayoutParams.MATCH_PARENT));
        LinearLayout body = new LinearLayout(this);
        body.setOrientation(LinearLayout.VERTICAL);
        stage.titleView = new TextView(this);
        stage.titleView.setTextSize(16);
        stage.titleView.setTextColor(Color.rgb(107, 114, 128));
        stage.titleView.setPadding(0, 0, 0, dp(4));
        stage.bodyView = new TextView(this);
        stage.bodyView.setTextSize(14);
        stage.bodyView.setTextColor(Color.rgb(145, 151, 161));
        stage.bodyView.setLineSpacing(4, 1);
        stage.itemList = new LinearLayout(this);
        stage.itemList.setOrientation(LinearLayout.VERTICAL);
        body.addView(stage.titleView, new LinearLayout.LayoutParams(-1, -2));
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
        TextView title = strong(item.optString("title", "买手经验"));
        title.setTextSize(14);
        card.addView(title, new LinearLayout.LayoutParams(-1, -2));
        TextView summary = muted(item.optString("summary", item.optString("content", item.optString("text", ""))));
        summary.setTextSize(13);
        summary.setMaxLines(4);
        card.addView(summary, new LinearLayout.LayoutParams(-1, -2));
        return card;
    }

    private void completeThinkingView() {
        if (activeThinking == null) {
            return;
        }
        activeThinking.completed = true;
        initializeThinkingStages(activeThinking);
        markKnownThinkingStagesCompleted(activeThinking);
        activeThinking.expanded = true;
        renderThinkingView(activeThinking);
        if (!activeThinking.segmentSaved) {
            appendThinkingSegment(activeThinking);
            activeThinking.segmentSaved = true;
        }
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
            activeAssistantSegments.put(segment);
        } catch (Exception ignored) {
        }
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
        thinking.expanded = true;
        initializeThinkingStages(thinking);
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
        if (!fallback.trim().isEmpty()) {
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
                String productId = productField(product, "productId", "product_id", "id", "");
                if (!productId.isEmpty()) {
                    if (activeRenderedProductIds.contains(productId)) {
                        return;
                    }
                    activeRenderedProductIds.add(productId);
                }
                LinearLayout bubble = ensureActiveAssistantMessageBubble(parent);
                bubble.addView(chatProductCard(product));
                scrollBottom();
            }
            return;
        }
        if ("product_refs".equals(type)) {
            renderProductRefs(block.optJSONArray("product_ids"), ensureActiveAssistantMessageBubble(parent));
            return;
        }
        if ("comparison_table".equals(type)) {
            parent.addView(comparisonCard(block));
            scrollBottom();
            return;
        }
        if ("citation".equals(type)) {
            parent.addView(citationCard(block.optJSONObject("citation")));
            scrollBottom();
            return;
        }
        if ("citation_refs".equals(type)) {
            return;
        }
        if ("warning".equals(type)) {
            parent.addView(warningCard(block.optString("message", block.optString("content", ""))));
            scrollBottom();
            return;
        }
        if ("cart_state".equals(type)) {
            parent.addView(cartStateCard(block.optJSONObject("cart")));
            scrollBottom();
            return;
        }
        if ("order_summary".equals(type)) {
            parent.addView(orderSummaryCard(block.optJSONArray("orders")));
            scrollBottom();
            return;
        }
        if ("discount_preview".equals(type)) {
            parent.addView(discountPreviewCard(block));
            scrollBottom();
            return;
        }
        if ("coupon_list".equals(type)) {
            parent.addView(couponListCard(serviceItems(block, "coupons")));
            scrollBottom();
            return;
        }
        if ("navigation_action".equals(type)) {
            parent.addView(navigationActionCard(block));
            scrollBottom();
            return;
        }
        if ("review_summary".equals(type)) {
            parent.addView(reviewSummaryCard(block));
            scrollBottom();
            return;
        }
        if ("after_sales_policy".equals(type)) {
            parent.addView(afterSalesPolicyCard(block));
            scrollBottom();
            return;
        }
        String content = block.optString("content", "");
        if (!content.isEmpty()) {
            addBubbleTo(content, false, parent);
        } else if (!type.isEmpty()) {
            parent.addView(warningCard("暂不支持的内容类型：" + type));
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
            holder.addView(chatProductCard(cached));
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
                        holder.addView(chatProductCard(detail));
                        scrollBottom();
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    holder.removeAllViews();
                    holder.addView(warningCard("商品信息加载失败：" + productId));
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
        for (int i = 0; i < segments.length(); i++) {
            JSONObject segment = segments.optJSONObject(i);
            if (segment == null) {
                continue;
            }
            String type = segment.optString("type");
            if ("text".equals(type)) {
                String text = segment.optString("text", "");
                if (!text.trim().isEmpty()) {
                    renderHistoricalTextSegment(context, text);
                }
            } else if ("table".equals(type)) {
                renderHistoricalTableSegment(context, segment.optString("markdown", ""));
            } else if ("block".equals(type)) {
                renderHistoricalBlock(context, segment.optJSONObject("block"));
            } else if ("thinking".equals(type)) {
                renderStoredThinking(segment.optJSONObject("thinking"), context.parent);
            } else if ("product_refs".equals(type) || "product_card".equals(type)) {
                renderHistoricalBlock(context, segment);
            }
        }
    }

    private void renderHistoricalTextSegment(HistoricalMessageRenderContext context, String text) {
        if (text == null || text.trim().isEmpty()) {
            return;
        }
        LinearLayout bubble = ensureHistoricalMessageBubble(context);
        String cleaned = text;
        List<MarkdownPart> parts = splitMarkdownParts(cleaned);
        if (parts.isEmpty()) {
            parts = new ArrayList<>();
            parts.add(new MarkdownPart(false, cleaned));
        }
        for (MarkdownPart part : parts) {
            if (part.table) {
                addMarkdownTableToBubble(bubble, part.content);
            } else if (!part.content.trim().isEmpty()) {
                TextView textView = assistantBubbleText("");
                enableCopy(textView, part.content);
                MarkdownRenderer.setMarkdown(textView, sanitizeAgentMarkdown(part.content).visibleMarkdown);
                bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
            }
        }
    }

    private void renderHistoricalTableSegment(HistoricalMessageRenderContext context, String markdown) {
        String cleaned = markdown == null ? "" : markdown.trim();
        if (cleaned.isEmpty()) {
            return;
        }
        LinearLayout bubble = ensureHistoricalMessageBubble(context);
        if (containsMarkdownTable(cleaned)) {
            addMarkdownTableToBubble(bubble, cleaned);
        } else {
            TextView textView = assistantBubbleText("");
            enableCopy(textView, cleaned);
            MarkdownRenderer.setMarkdown(textView, sanitizeAgentMarkdown(cleaned).visibleMarkdown);
            bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
        }
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
            context.parent.addView(comparisonCard(block));
            return;
        }
        if ("warning".equals(type)) {
            context.parent.addView(warningCard(block.optString("message", block.optString("content", ""))));
            return;
        }
        if ("cart_state".equals(type)) {
            context.parent.addView(cartStateCard(block.optJSONObject("cart")));
            return;
        }
        if ("order_summary".equals(type)) {
            context.parent.addView(orderSummaryCard(block.optJSONArray("orders")));
            return;
        }
        if ("discount_preview".equals(type)) {
            context.parent.addView(discountPreviewCard(block));
            return;
        }
        if ("coupon_list".equals(type)) {
            context.parent.addView(couponListCard(serviceItems(block, "coupons")));
            return;
        }
        if ("review_summary".equals(type)) {
            context.parent.addView(reviewSummaryCard(block));
            return;
        }
        if ("after_sales_policy".equals(type)) {
            context.parent.addView(afterSalesPolicyCard(block));
            return;
        }
        if ("navigation_action".equals(type)) {
            context.parent.addView(navigationActionCard(block));
        }
    }

    private void renderHistoricalProductCard(HistoricalMessageRenderContext context, JSONObject product) {
        if (product == null) {
            return;
        }
        String productId = productField(product, "productId", "product_id", "id", "");
        if (!productId.isEmpty()) {
            if (context.renderedProductIds.contains(productId)) {
                return;
            }
            context.renderedProductIds.add(productId);
        }
        ensureHistoricalMessageBubble(context).addView(chatProductCard(product));
    }

    private void renderHistoricalProductRefs(HistoricalMessageRenderContext context, JSONArray productIds) {
        if (productIds == null || productIds.length() == 0) {
            return;
        }
        LinearLayout bubble = ensureHistoricalMessageBubble(context);
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
                holder.addView(chatProductCard(cached));
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
                            holder.addView(chatProductCard(detail));
                        }
                    });
                } catch (Exception error) {
                    runOnUiThread(() -> {
                        if (holder.getParent() != null) {
                            holder.removeAllViews();
                            holder.addView(warningCard("商品信息加载失败：" + productId));
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
        context.bubble = embeddedProductBubble();
        enableCopy(context.bubble, context.copyMarkdown);
        context.parent.addView(context.bubble);
        return context.bubble;
    }

    private View chatProductCard(JSONObject item) {
        LinearLayout card = new LinearLayout(this);
        card.setOrientation(LinearLayout.VERTICAL);
        card.setPadding(dp(12), dp(10), dp(12), dp(10));
        card.setBackground(rounded(Color.rgb(248, 249, 251), dp(14)));
        LinearLayout.LayoutParams cardParams = new LinearLayout.LayoutParams(-1, -2);
        cardParams.setMargins(0, dp(6), 0, dp(8));
        card.setLayoutParams(cardParams);
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(productImage(productField(item, "imageUrl", "image_url"), 72), new LinearLayout.LayoutParams(dp(72), dp(72)));
        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        TextView name = strong(item.optString("name", "商品"));
        name.setTextSize(15);
        name.setMaxLines(2);
        info.addView(name);
        info.addView(muted(item.optString("brand", "") + " · " + productField(item, "merchantName", "merchant_name", "商家")));
        TextView price = strong("¥" + item.optString("price", "0"));
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
        LinearLayout actions = new LinearLayout(this);
        Button detail = secondaryButton("详情");
        detail.setOnClickListener(v -> renderProductDetail(productField(item, "productId", "product_id", "id", ""), () -> renderChatHome()));
        LinearLayout.LayoutParams detailParams = new LinearLayout.LayoutParams(0, dp(42), 1);
        detailParams.rightMargin = dp(8);
        actions.addView(detail, detailParams);
        Button add = primaryButton("加入购物车");
        add.setOnClickListener(v -> addProductToCart(item, add));
        actions.addView(add, new LinearLayout.LayoutParams(0, dp(42), 1));
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(-1, dp(42));
        actionParams.topMargin = dp(8);
        card.addView(actions, actionParams);
        enableCopy(card, productCopyMarkdown(item));
        return card;
    }

    private View chatProductBubble(JSONObject product) {
        LinearLayout bubble = embeddedProductBubble();
        bubble.addView(chatProductCard(product));
        return bubble;
    }

    private LinearLayout ensureActiveAssistantMessageBubble(LinearLayout parent) {
        removeLoadingBubbleIfNeeded();
        if (activeAssistantMessageBubble != null) {
            return activeAssistantMessageBubble;
        }
        LinearLayout bubble = embeddedProductBubble();
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

    private LinearLayout embeddedProductBubble() {
        LinearLayout bubble = new LinearLayout(this);
        bubble.setOrientation(LinearLayout.VERTICAL);
        bubble.setPadding(dp(10), dp(8), dp(10), dp(4));
        bubble.setBackground(rounded(Color.WHITE, dp(16)));
        int bubbleWidth = (int) (getResources().getDisplayMetrics().widthPixels * 0.82f);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(bubbleWidth, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = Gravity.LEFT;
        params.setMargins(0, dp(4), 0, dp(8));
        bubble.setLayoutParams(params);
        return bubble;
    }

    private View comparisonCard(JSONObject block) {
        LinearLayout card = panel();
        card.setPadding(dp(14), dp(14), dp(14), dp(14));
        card.addView(strong("商品对比"));
        SpaceView gap = new SpaceView(this);
        card.addView(gap, new LinearLayout.LayoutParams(1, dp(12)));
        JSONArray columns = block.optJSONArray("columns");
        JSONArray rows = block.optJSONArray("rows");
        HorizontalScrollView scroll = new HorizontalScrollView(this);
        scroll.setHorizontalScrollBarEnabled(true);
        LinearLayout table = new LinearLayout(this);
        table.setOrientation(LinearLayout.VERTICAL);
        table.setMinimumWidth((int) (getResources().getDisplayMetrics().widthPixels * 1.15f));
        if (columns != null) {
            table.addView(comparisonRow(columns, true));
        }
        if (rows != null) {
            for (int i = 0; i < rows.length(); i++) {
                JSONObject row = rows.optJSONObject(i);
                JSONArray values = row == null ? null : row.optJSONArray("values");
                if (values == null) {
                    continue;
                }
                table.addView(comparisonRow(values, false));
            }
        }
        scroll.addView(table, new HorizontalScrollView.LayoutParams(-2, -2));
        card.addView(scroll, new LinearLayout.LayoutParams(-1, -2));
        enableCopy(card, block == null ? "" : block.toString());
        return card;
    }

    private View comparisonRow(JSONArray values, boolean header) {
        LinearLayout row = new LinearLayout(this);
        row.setOrientation(LinearLayout.HORIZONTAL);
        row.setBackgroundColor(header ? Color.rgb(243, 244, 246) : Color.WHITE);
        int count = Math.max(1, values.length());
        for (int i = 0; i < count; i++) {
            TextView cell = new TextView(this);
            cell.setText(values.optString(i, ""));
            cell.setTextSize(13);
            cell.setTextColor(header ? Color.rgb(17, 24, 39) : Color.rgb(75, 85, 99));
            cell.setTypeface(Typeface.DEFAULT, header ? Typeface.BOLD : Typeface.NORMAL);
            cell.setPadding(dp(10), dp(9), dp(10), dp(9));
            cell.setGravity(Gravity.CENTER_VERTICAL);
            row.addView(cell, new LinearLayout.LayoutParams(i == 0 ? dp(150) : dp(126), -2));
        }
        return row;
    }

    private View citationCard(JSONObject citation) {
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

    private View warningCard(String message) {
        TextView view = card("提示", message == null || message.isEmpty() ? "当前信息需要进一步确认。" : message);
        view.setBackground(rounded(Color.rgb(255, 250, 235), dp(12)));
        return view;
    }

    private View cartStateCard(JSONObject cart) {
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

    private View orderSummaryCard(JSONArray orders) {
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

    private View discountPreviewCard(JSONObject block) {
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

    private View couponListCard(JSONArray coupons) {
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

    private View reviewSummaryCard(JSONObject block) {
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
            card.addView(muted(block.optString("message", "暂无评价摘要")));
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

    private View afterSalesPolicyCard(JSONObject block) {
        LinearLayout card = panel();
        card.addView(strong(blockTitle(block, "售后规则")));
        String message = block.optString("message", "");
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

    private View navigationActionCard(JSONObject block) {
        LinearLayout card = panel();
        JSONObject action = block == null ? null : block.optJSONObject("action");
        String label = action == null ? blockTitle(block, "查看详情") : action.optString("label", blockTitle(block, "查看详情"));
        String route = action == null ? "" : action.optString("route", action.optString("target", ""));
        String message = block == null ? "" : block.optString("message", "");
        if (!message.isEmpty()) {
            card.addView(muted(message));
        }
        TextView view = strong(label + "  >");
        view.setOnClickListener(v -> navigateRoute(route));
        card.addView(view);
        return card;
    }

    private void navigateRoute(String route) {
        if (route == null) {
            route = "";
        }
        if ("orders".equals(route)) {
            renderOrders();
        } else if ("cart".equals(route)) {
            renderCart();
        } else if ("products".equals(route)) {
            renderProducts();
        } else if ("coupons".equals(route) || "coupon".equals(route)) {
            renderCoupons();
        } else if ("promotions".equals(route) || "promotion".equals(route) || "activity".equals(route)) {
            renderProductsTab("activity", false);
        } else {
            toastLine("暂不支持该跳转");
        }
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

    private void renderFollowups(JSONArray questions, LinearLayout parent) {
        if (questions == null || questions.length() == 0 || parent == null) {
            return;
        }
        if (streaming && activeFollowupsView != null && activeFollowupsView.getParent() == parent) {
            parent.removeView(activeFollowupsView);
        }
        LinearLayout wrap = new LinearLayout(this);
        wrap.setOrientation(LinearLayout.VERTICAL);
        wrap.setPadding(0, dp(8), 0, dp(6));
        TextView title = muted("您还可以继续追问：");
        title.setGravity(Gravity.CENTER);
        wrap.addView(title, new LinearLayout.LayoutParams(-1, -2));
        HorizontalScrollView scroll = new HorizontalScrollView(this);
        scroll.setHorizontalScrollBarEnabled(false);
        LinearLayout chips = new LinearLayout(this);
        chips.setOrientation(LinearLayout.HORIZONTAL);
        for (int i = 0; i < questions.length(); i++) {
            String question = questions.optString(i, "");
            if (question.isEmpty()) {
                continue;
            }
            TextView chip = new TextView(this);
            chip.setText(question);
            chip.setTextSize(13);
            chip.setTextColor(Color.rgb(55, 65, 81));
            chip.setGravity(Gravity.CENTER);
            chip.setPadding(dp(12), 0, dp(12), 0);
            chip.setBackground(rounded(Color.rgb(238, 239, 241), dp(19)));
            chip.setOnClickListener(v -> fillInputFromFollowup(((TextView) v).getText().toString()));
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-2, dp(38));
            params.setMargins(0, 0, dp(8), 0);
            chips.addView(chip, params);
        }
        scroll.addView(chips);
        wrap.addView(scroll, new LinearLayout.LayoutParams(-1, dp(42)));
        parent.addView(wrap, new LinearLayout.LayoutParams(-1, -2));
        if (streaming) {
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

            LinearLayout servicePanel = panel();
            servicePanel.addView(strong("我的服务"));
            Button coupons = secondaryButton("我的优惠券");
            coupons.setOnClickListener(v -> renderCoupons());
            addFormButton(servicePanel, coupons);
            page.addView(servicePanel);

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

    private void renderLoginPage() {
        closeDrawer();
        activePage = "login";
        backStack.clear();
        baseScreen();
        content.addView(createBackTopBar("登录", () -> {
            renderChatHome();
            loadHomeCopy();
        }), new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();
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
        Button register = secondaryButton("注册账号");
        register.setOnClickListener(v -> register(username.getText().toString(), password.getText().toString(), "user"));
        addFormButton(loginPanel, register);
        page.addView(loginPanel);
        if (BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            LinearLayout dev = panel();
            dev.addView(strong("测试后端"));
            EditText apiBase = inputField("后端地址", sessionStore.apiBase());
            dev.addView(apiBase);
            Button save = secondaryButton("保存测试地址");
            save.setOnClickListener(v -> {
                saveApiBaseFromInput(apiBase.getText().toString());
            });
            addFormButton(dev, save);
            page.addView(dev);
        }
    }

    private void register(String username, String password, String role) {
        new Thread(() -> {
            try {
                String displayName = role.equals("merchant") ? "新商家" : username;
                JSONObject result = api.register(username, password, displayName, role, displayName, "123456");
                JSONObject account = result.optJSONObject("account");
                saveAccountSession(result.optString("token"), account, role, displayName);
                runOnUiThread(() -> {
                    toastLine("注册成功");
                    syncRemoteSessions(null);
                    createFreshLocalSession();
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
                saveAccountSession(result.optString("token"), account, "user", username);
                runOnUiThread(() -> {
                    toastLine("登录成功");
                    syncRemoteSessions(null);
                    createFreshLocalSession();
                    renderChatHome();
                    loadHomeCopy();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("登录失败：" + error.getMessage()));
            }
        }).start();
    }

    private void saveAccountSession(String token, JSONObject account, String fallbackRole, String fallbackName) {
        if (account == null) {
            sessionStore.saveAuth(token, fallbackRole, fallbackName, "");
            return;
        }
        sessionStore.saveAuth(
                token,
                account.optString("role", fallbackRole),
                account.optString("display_name", fallbackName),
                account.optString("avatar_url", ""),
                account.optString("account_id", ""),
                account.optString("username", fallbackName),
                account.optString("phone", ""),
                account.optString("email", "")
        );
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
                sessionStore.saveAuth(sessionStore.token(), sessionStore.role(), profile.optString("display_name", value), profile.optString("avatar_url", image), profile.optString("account_id", sessionStore.accountId()), profile.optString("username", sessionStore.username()), profile.optString("phone", sessionStore.phone()), profile.optString("email", sessionStore.email()));
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
        renderProducts(false);
    }

    private void renderProducts(boolean restoreScroll) {
        renderProductsTab("list", restoreScroll);
    }

    private void renderProductsTab(String tab, boolean restoreScroll) {
        closeDrawer();
        activePage = "products";
        useKeyboardOverlay();
        activeProductTab = tab == null || tab.isEmpty() ? "list" : tab;
        backStack.clear();
        baseScreen();
        if (restoreScroll) {
            content.setAlpha(0f);
        }
        addPageHeader("商品", "搜索商品、查看详情、加入购物车");
        if ("activity".equals(activeProductTab)) {
            renderProductPromotionsTab();
        } else {
            renderProductListTab(restoreScroll);
        }
        content.addView(productBottomBar(), new LinearLayout.LayoutParams(-1, dp(58)));
        addProductCartFab();
    }

    private void renderProductListTab(boolean restoreScroll) {
        productsScroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(dp(16), dp(10), dp(16), dp(16));
        productsScroll.addView(page);
        content.addView(productsScroll, new LinearLayout.LayoutParams(-1, 0, 1));
        restoreProductsScroll = restoreScroll;

        LinearLayout searchRow = new LinearLayout(this);
        searchRow.setGravity(Gravity.CENTER_VERTICAL);
        TextView searchIcon = muted("⌕");
        searchIcon.setGravity(Gravity.CENTER);
        searchRow.addView(searchIcon, new LinearLayout.LayoutParams(dp(30), dp(48)));
        EditText keyword = underlineInputField("搜索商品", lastProductKeyword);
        searchRow.addView(keyword, new LinearLayout.LayoutParams(0, dp(52), 1));
        Button search = textOnlyButton("搜索");
        search.setTextColor(Color.rgb(37, 99, 235));
        LinearLayout.LayoutParams searchParams = new LinearLayout.LayoutParams(dp(64), dp(52));
        searchParams.leftMargin = dp(10);
        searchRow.addView(search, searchParams);
        page.addView(searchRow);

        LinearLayout categoryButton = textFilterRow("商品类别", lastCategoryName + "  >");
        categoryButton.setOnClickListener(v -> openCategoryDialog(() -> {
            lastProductKeyword = keyword.getText().toString();
            resetProductState(lastProductKeyword, lastCategoryId);
            renderProducts();
        }));
        LinearLayout.LayoutParams filterParams = new LinearLayout.LayoutParams(-1, dp(46));
        filterParams.setMargins(0, 0, 0, dp(10));
        page.addView(categoryButton, filterParams);

        productsList = new LinearLayout(this);
        productsList.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams listParams = new LinearLayout.LayoutParams(-1, -2);
        listParams.topMargin = dp(2);
        page.addView(productsList, listParams);
        search.setOnClickListener(v -> {
            lastProductKeyword = keyword.getText().toString();
            productsScrollY = 0;
            resetProductState(lastProductKeyword, lastCategoryId);
            loadProducts(productsList, lastProductKeyword, lastCategoryId);
        });
        productsScroll.setOnScrollChangeListener((v, scrollX, scrollY, oldScrollX, oldScrollY) -> {
            productsScrollY = scrollY;
            if (currentProductState != null) {
                currentProductState.scrollY = scrollY;
            }
            if (productsScroll == null || productsList == null || currentProductState == null) {
                return;
            }
            int bottomDistance = productsList.getBottom() - (productsScroll.getHeight() + scrollY);
            if (bottomDistance > dp(8) || !currentProductState.hasMore || loadingProducts) {
                return;
            }
            if (scrollY >= oldScrollY) {
                loadMoreProducts(productsList, lastProductKeyword, lastCategoryId);
            }
        });
        ensureCategoriesLoaded();
        loadProducts(productsList, lastProductKeyword, lastCategoryId);
    }

    private void renderProductPromotionsTab() {
        LinearLayout page = pageBody();
        page.addView(muted("正在加载活动..."));
        new Thread(() -> {
            try {
                JSONArray items = api.promotions();
                runOnUiThread(() -> renderPromotionItems(page, items));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "活动加载失败", error.getMessage(), () -> renderProductsTab("activity", false)));
            }
        }).start();
    }

    private View productBottomBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER);
        bar.setPadding(dp(12), dp(6), dp(12), dp(6));
        bar.setBackgroundColor(Color.WHITE);
        bar.addView(productTabButton("商品列表", "list"), new LinearLayout.LayoutParams(0, dp(46), 1));
        bar.addView(productTabButton("活动", "activity"), new LinearLayout.LayoutParams(0, dp(46), 1));
        return bar;
    }

    private void addProductCartFab() {
        if (sessionStore.token().isEmpty()) {
            return;
        }
        productCartFab = new FrameLayout(this);
        productCartFab.setClickable(true);
        productCartFab.setBackground(rounded(Color.BLACK, dp(28)));
        productCartFab.setElevation(dp(8));
        TextView icon = new TextView(this);
        icon.setText("购物车");
        icon.setTextSize(13);
        icon.setTypeface(Typeface.DEFAULT_BOLD);
        icon.setTextColor(Color.WHITE);
        icon.setGravity(Gravity.CENTER);
        productCartFab.addView(icon, new FrameLayout.LayoutParams(dp(56), dp(56), Gravity.CENTER));

        productCartBadge = new TextView(this);
        productCartBadge.setTextSize(10);
        productCartBadge.setTextColor(Color.WHITE);
        productCartBadge.setGravity(Gravity.CENTER);
        productCartBadge.setTypeface(Typeface.DEFAULT_BOLD);
        productCartBadge.setBackground(rounded(Color.rgb(220, 38, 38), dp(10)));
        FrameLayout.LayoutParams badgeParams = new FrameLayout.LayoutParams(dp(28), dp(20), Gravity.RIGHT | Gravity.TOP);
        badgeParams.topMargin = -dp(4);
        badgeParams.rightMargin = -dp(6);
        productCartFab.addView(productCartBadge, badgeParams);
        productCartFab.setOnClickListener(v -> renderCart());

        FrameLayout.LayoutParams fabParams = new FrameLayout.LayoutParams(dp(56), dp(56), Gravity.RIGHT | Gravity.BOTTOM);
        fabParams.rightMargin = dp(18);
        fabParams.bottomMargin = dp(76);
        root.addView(productCartFab, fabParams);
        updateProductCartBadge(cartItemCountCache);
        refreshProductCartBadge();
    }

    private void refreshProductCartBadge() {
        if (sessionStore.token().isEmpty() || productCartBadge == null) {
            return;
        }
        new Thread(() -> {
            try {
                JSONObject cart = api.cart();
                int count = cartItemCount(cart);
                cartItemCountCache = count;
                runOnUiThread(() -> updateProductCartBadge(count));
            } catch (Exception ignored) {
            }
        }).start();
    }

    private void updateProductCartBadge(int count) {
        if (productCartBadge == null) {
            return;
        }
        if (count <= 0) {
            productCartBadge.setVisibility(View.GONE);
            return;
        }
        productCartBadge.setVisibility(View.VISIBLE);
        productCartBadge.setText(cartBadgeText(count));
    }

    private int cartItemCount(JSONObject cart) {
        JSONArray items = cart == null ? null : cart.optJSONArray("items");
        int count = 0;
        if (items != null) {
            for (int i = 0; i < items.length(); i++) {
                JSONObject item = items.optJSONObject(i);
                if (item != null) {
                    count += Math.max(0, item.optInt("quantity", 0));
                }
            }
        }
        return count;
    }

    private String cartBadgeText(int count) {
        return count > 99 ? "99+" : String.valueOf(count);
    }

    private TextView productTabButton(String text, String tab) {
        boolean selected = tab.equals(activeProductTab);
        TextView view = new TextView(this);
        view.setText(text);
        view.setTextSize(15);
        view.setTypeface(Typeface.DEFAULT, selected ? Typeface.BOLD : Typeface.NORMAL);
        view.setTextColor(selected ? Color.BLACK : Color.rgb(107, 114, 128));
        view.setGravity(Gravity.CENTER);
        view.setBackground(rounded(selected ? Color.rgb(245, 246, 248) : Color.WHITE, dp(14)));
        view.setOnClickListener(v -> {
            if ("list".equals(tab)) {
                renderProductsTab("list", true);
            } else {
                renderProductsTab("activity", false);
            }
        });
        return view;
    }

    private void ensureCategoriesLoaded() {
        if (cachedCategoryTree.length() > 0) {
            return;
        }
        new Thread(() -> {
            try {
                JSONArray categories = api.categoriesTree();
                runOnUiThread(() -> cachedCategoryTree = categories);
            } catch (Exception ignored) {
            }
        }).start();
    }

    private void openCategoryDialog(Runnable onChanged) {
        if (cachedCategoryTree.length() == 0) {
            new Thread(() -> {
                try {
                    JSONArray categories = api.categoriesTree();
                    runOnUiThread(() -> {
                        cachedCategoryTree = categories;
                        openCategoryDialog(onChanged);
                    });
                } catch (Exception error) {
                    runOnUiThread(() -> toastLine("分类加载失败：" + error.getMessage()));
                }
            }).start();
            return;
        }
        final String[] pendingId = {lastCategoryId};
        final String[] pendingName = {lastCategoryName};
        LinearLayout rootLayout = new LinearLayout(this);
        rootLayout.setOrientation(LinearLayout.HORIZONTAL);
        rootLayout.setPadding(dp(8), dp(8), dp(8), dp(8));
        LinearLayout primary = new LinearLayout(this);
        primary.setOrientation(LinearLayout.VERTICAL);
        LinearLayout secondary = new LinearLayout(this);
        secondary.setOrientation(LinearLayout.VERTICAL);
        ScrollView primaryScroll = new ScrollView(this);
        primaryScroll.addView(primary);
        ScrollView secondaryScroll = new ScrollView(this);
        secondaryScroll.addView(secondary);
        rootLayout.addView(primaryScroll, new LinearLayout.LayoutParams(0, dp(420), 1));
        rootLayout.addView(secondaryScroll, new LinearLayout.LayoutParams(0, dp(420), 1));

        Runnable[] renderSecondary = new Runnable[1];
        renderSecondary[0] = () -> {
        };
        final JSONObject[] selectedPrimary = {null};
        addPrimaryCategoryButton(primary, secondary, "全部", null, pendingId, pendingName, pendingId[0] == null || pendingId[0].isEmpty());
        for (int i = 0; i < cachedCategoryTree.length(); i++) {
            JSONObject category = cachedCategoryTree.optJSONObject(i);
            if (category != null) {
                String categoryId = category.optString("categoryId", "");
                if (categoryId.equals(pendingId[0])) {
                    selectedPrimary[0] = category;
                }
                addPrimaryCategoryButton(primary, secondary, category.optString("name", "分类"), category, pendingId, pendingName, categoryId.equals(pendingId[0]));
            }
        }
        fillSecondaryCategories(secondary, selectedPrimary[0], pendingId, pendingName);

        new AlertDialog.Builder(this)
                .setTitle("选择商品类别")
                .setView(rootLayout)
                .setNegativeButton("取消", null)
                .setNeutralButton("重置", (dialog, which) -> {
                    lastCategoryId = "";
                    lastCategoryName = "全部";
                    productsScrollY = 0;
                    resetProductState(lastProductKeyword, lastCategoryId);
                    onChanged.run();
                })
                .setPositiveButton("确定", (dialog, which) -> {
                    lastCategoryId = pendingId[0];
                    lastCategoryName = pendingName[0];
                    productsScrollY = 0;
                    resetProductState(lastProductKeyword, lastCategoryId);
                    onChanged.run();
                })
                .show();
    }

    private void addPrimaryCategoryButton(LinearLayout primary, LinearLayout secondary, String name, JSONObject category, String[] pendingId, String[] pendingName, boolean selected) {
        TextView button = categoryRow(name, selected);
        button.setGravity(Gravity.LEFT | Gravity.CENTER_VERTICAL);
        button.setOnClickListener(v -> {
            for (int i = 0; i < primary.getChildCount(); i++) {
                primary.getChildAt(i).setBackgroundColor(Color.TRANSPARENT);
            }
            button.setBackgroundColor(Color.rgb(238, 242, 247));
            pendingId[0] = category == null ? "" : category.optString("categoryId", "");
            pendingName[0] = category == null ? "全部" : category.optString("name", "全部");
            fillSecondaryCategories(secondary, category, pendingId, pendingName);
        });
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(44));
        params.setMargins(0, 0, dp(8), 0);
        primary.addView(button, params);
    }

    private void fillSecondaryCategories(LinearLayout secondary, JSONObject category, String[] pendingId, String[] pendingName) {
        secondary.removeAllViews();
        String allName = category == null ? "全部" : category.optString("name", "全部");
        String allId = category == null ? "" : category.optString("categoryId", "");
        addSecondaryCategoryButton(secondary, "全部".equals(allName) ? "全部" : allName + "全部", allId, allName, pendingId, pendingName);
        JSONArray children = category == null ? null : category.optJSONArray("children");
        if (children == null) {
            return;
        }
        for (int i = 0; i < children.length(); i++) {
            JSONObject child = children.optJSONObject(i);
            if (child != null) {
                addSecondaryCategoryButton(secondary, child.optString("name", "分类"), child.optString("categoryId", ""), child.optString("name", "分类"), pendingId, pendingName);
            }
        }
    }

    private void addSecondaryCategoryButton(LinearLayout secondary, String label, String categoryId, String categoryName, String[] pendingId, String[] pendingName) {
        TextView button = categoryRow(label, categoryId.equals(pendingId[0]));
        button.setGravity(Gravity.LEFT | Gravity.CENTER_VERTICAL);
        button.setOnClickListener(v -> {
            pendingId[0] = categoryId;
            pendingName[0] = categoryName == null || categoryName.isEmpty() ? "全部" : categoryName;
            for (int i = 0; i < secondary.getChildCount(); i++) {
                secondary.getChildAt(i).setBackgroundColor(Color.TRANSPARENT);
            }
            button.setBackgroundColor(Color.rgb(238, 242, 247));
        });
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(44));
        params.setMargins(0, 0, 0, 0);
        secondary.addView(button, params);
    }

    private TextView categoryRow(String label, boolean selected) {
        TextView view = new TextView(this);
        view.setText(label);
        view.setTextSize(15);
        view.setTextColor(Color.rgb(17, 24, 39));
        view.setPadding(dp(14), 0, dp(10), 0);
        view.setBackgroundColor(selected ? Color.rgb(238, 242, 247) : Color.TRANSPARENT);
        return view;
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
        String key = productCacheKey(keyword, categoryId);
        currentProductState = productListCache.get(key);
        if (currentProductState != null && currentProductState.loaded) {
            renderProductItems(list, currentProductState);
            return;
        }
        if (restoreProductsScroll) {
            content.setAlpha(1f);
            restoreProductsScroll = false;
        }
        currentProductState = new ProductListState();
        productListCache.put(key, currentProductState);
        list.removeAllViews();
        list.addView(muted("正在加载商品..."));
        loadMoreProducts(list, keyword, categoryId);
    }

    private void loadMoreProducts(LinearLayout list, String keyword, String categoryId) {
        if (loadingProducts) {
            return;
        }
        if (currentProductState == null) {
            currentProductState = new ProductListState();
            productListCache.put(productCacheKey(keyword, categoryId), currentProductState);
        }
        loadingProducts = true;
        if (currentProductState.loaded) {
            renderProductItems(list, currentProductState);
        }
        new Thread(() -> {
            try {
                ApiClient.ProductPage page = api.productsPage(keyword, categoryId, currentProductState.nextPage, PRODUCT_PAGE_SIZE);
                runOnUiThread(() -> {
                    appendProducts(currentProductState.items, page.items);
                    currentProductState.nextPage = page.nextPage;
                    currentProductState.hasMore = page.hasMore;
                    currentProductState.loaded = true;
                    loadingProducts = false;
                    renderProductItems(list, currentProductState);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    loadingProducts = false;
                    if (currentProductState != null && currentProductState.loaded) {
                        toastLine("加载更多失败：" + error.getMessage());
                        renderProductItems(list, currentProductState);
                    } else {
                        renderError(list, "商品加载失败", error.getMessage(), () -> loadProducts(list, keyword, categoryId));
                    }
                });
            }
        }).start();
    }

    private void renderProductItems(LinearLayout list, ProductListState state) {
        list.removeAllViews();
        JSONArray items = state.items;
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
        if (state.hasMore) {
            TextView more = muted(loadingProducts ? "正在加载更多..." : "向下滑动加载更多");
            more.setGravity(Gravity.CENTER);
            list.addView(more, new LinearLayout.LayoutParams(-1, dp(44)));
        } else {
            TextView end = muted("已经到底了");
            end.setGravity(Gravity.CENTER);
            list.addView(end, new LinearLayout.LayoutParams(-1, dp(44)));
        }
        if (restoreProductsScroll && productsScroll != null) {
            int savedY = state.scrollY;
            productsScroll.post(() -> {
                productsScroll.scrollTo(0, savedY);
                content.setAlpha(1f);
                restoreProductsScroll = false;
            });
        }
    }

    private void appendProducts(JSONArray target, JSONArray source) {
        for (int i = 0; i < source.length(); i++) {
            target.put(source.opt(i));
        }
    }

    private String productCacheKey(String keyword, String categoryId) {
        return (keyword == null ? "" : keyword.trim()) + "::" + (categoryId == null ? "" : categoryId.trim());
    }

    private void resetProductState(String keyword, String categoryId) {
        productListCache.remove(productCacheKey(keyword, categoryId));
        currentProductState = null;
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
        detail.setOnClickListener(v -> {
            productsScrollY = productsScroll == null ? 0 : productsScroll.getScrollY();
            if (currentProductState != null) {
                currentProductState.scrollY = productsScrollY;
            }
            renderProductDetail(item.optString("productId"), () -> renderProducts(true));
        });
        LinearLayout.LayoutParams detailParams = new LinearLayout.LayoutParams(0, dp(44), 1);
        detailParams.rightMargin = dp(8);
        actions.addView(detail, detailParams);
        Button add = primaryButton("加入购物车");
        add.setOnClickListener(v -> addProductToCart(item, add));
        actions.addView(add, new LinearLayout.LayoutParams(0, dp(44), 1));
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(-1, dp(44));
        actionParams.topMargin = dp(8);
        card.addView(actions, actionParams);
        return card;
    }

    private void renderProductDetail(String productId, Runnable backAction) {
        closeDrawer();
        activePage = "product_detail";
        useKeyboardOverlay();
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
        page.addView(productDetailImage(imageUrl), new LinearLayout.LayoutParams(-1, dp(240)));

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

        addExpandableArrayPanel(page, "商品卖点", item.optJSONArray("sellingPoints"), 3);
        addExpandableTextPanel(page, "推荐理由", item.optString("recommendReason", ""), 3);
        addExpandableArrayPanel(page, "适合人群", item.optJSONArray("suitableFor"), 3);
        addExpandableArrayPanel(page, "不适合人群", item.optJSONArray("notSuitableFor"), 3);
        addExpandableArrayPanel(page, "风险提示", item.optJSONArray("riskNotes"), 3);
        addExpandableAttributesPanel(page, item.optJSONArray("attributes"), 6);
        addSkusPanel(page, skus);
        loadProductReviews(page, item.optString("productId"));

        Button add = primaryButton("加入购物车");
        add.setOnClickListener(v -> addProductToCart(item, add));
        page.addView(add, new LinearLayout.LayoutParams(-1, dp(52)));
    }

    private void loadProductReviews(LinearLayout page, String productId) {
        if (productId == null || productId.isEmpty()) {
            return;
        }
        LinearLayout panel = panel();
        panel.addView(strong("用户评价"));
        panel.addView(muted("正在加载评价..."));
        page.addView(panel);
        new Thread(() -> {
            try {
                JSONArray reviews = api.productReviews(productId);
                runOnUiThread(() -> renderProductReviews(panel, reviews));
            } catch (Exception error) {
                runOnUiThread(() -> renderProductReviews(panel, new JSONArray()));
            }
        }).start();
    }

    private void renderProductReviews(LinearLayout panel, JSONArray reviews) {
        panel.removeAllViews();
        panel.addView(strong("用户评价"));
        if (reviews == null || reviews.length() == 0) {
            panel.addView(muted("暂无评价"));
            return;
        }
        int total = Math.min(reviews.length(), 3);
        for (int i = 0; i < total; i++) {
            JSONObject review = reviews.optJSONObject(i);
            if (review != null) {
                panel.addView(muted("★ " + review.optInt("rating", 5) + "  " + review.optString("content", "")));
                String reply = review.optString("merchant_reply", "");
                if (!reply.isEmpty()) {
                    panel.addView(muted("商家回复：" + reply));
                }
            }
        }
    }

    private void addProductToCart(JSONObject item) {
        addProductToCart(item, null);
    }

    private void addProductToCart(JSONObject item, Button sourceButton) {
        if (sessionStore.token().isEmpty()) {
            toastLine("请先登录后再加入购物车");
            renderLoginPage();
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
                    refreshProductCartBadge();
                    if (sourceButton != null) {
                        sourceButton.setEnabled(true);
                        sourceButton.setAlpha(1f);
                        sourceButton.setText("已加入");
                        sourceButton.postDelayed(() -> sourceButton.setText("加入购物车"), 600);
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (sourceButton != null) {
                        sourceButton.setEnabled(true);
                        sourceButton.setAlpha(1f);
                        sourceButton.setText("加入购物车");
                    }
                    toastLine("加入失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private void renderCart() {
        closeDrawer();
        activePage = "cart";
        useKeyboardOverlay();
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
        cartItemCountCache = cartItemCount(cart);
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
        checkoutBar.addView(muted("商品总额：¥" + (summary == null ? "0" : summary.optString("totalAmount", "0"))));
        loadDiscountPreview(checkoutBar);
        Button checkout = primaryButton("结算");
        checkout.setOnClickListener(v -> confirmCheckout(page, summary));
        checkoutBar.addView(checkout, new LinearLayout.LayoutParams(-1, dp(52)));
        page.addView(checkoutBar);
    }

    private void loadDiscountPreview(LinearLayout checkoutBar) {
        new Thread(() -> {
            try {
                JSONObject preview = api.discountPreview();
                runOnUiThread(() -> renderDiscountPreviewRows(checkoutBar, preview));
            } catch (Exception ignored) {
            }
        }).start();
    }

    private void renderDiscountPreviewRows(LinearLayout parent, JSONObject discount) {
        View divider = new View(this);
        divider.setBackgroundColor(Color.rgb(238, 239, 242));
        LinearLayout.LayoutParams dividerParams = new LinearLayout.LayoutParams(-1, 1);
        dividerParams.setMargins(0, dp(14), 0, dp(10));
        parent.addView(divider, dividerParams);
        parent.addView(strong("优惠明细"));
        if (discount == null) {
            parent.addView(muted("暂无可用优惠"));
            return;
        }
        parent.addView(muted("优惠金额：¥" + discount.optString("discount_amount", discount.optString("discountAmount", "0"))));
        parent.addView(strong("应付：¥" + discount.optString("pay_amount", discount.optString("payAmount", "0"))));
        JSONArray lines = discount.optJSONArray("lines");
        if (lines != null) {
            for (int i = 0; i < lines.length(); i++) {
                JSONObject line = lines.optJSONObject(i);
                if (line != null) {
                    parent.addView(muted(line.optString("name", "优惠") + " -¥" + line.optString("amount", "0")));
                }
            }
        }
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
        LinearLayout.LayoutParams controlsParams = new LinearLayout.LayoutParams(-1, dp(46));
        controlsParams.topMargin = dp(14);
        card.addView(controls, controlsParams);
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
                JSONArray orders = api.checkout();
                runOnUiThread(() -> {
                    toastLine("订单已创建，请尽快支付");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("结算失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderCoupons() {
        closeDrawer();
        activePage = "coupons";
        backStack.clear();
        baseScreen();
        addPageHeader("优惠券", "领取和查看可用优惠");
        LinearLayout page = pageBody();
        if (sessionStore.token().isEmpty()) {
            page.addView(card("请先登录", "登录后可领取和查看优惠券。"));
            return;
        }
        page.addView(muted("正在加载优惠券..."));
        new Thread(() -> {
            try {
                JSONArray available = api.availableCoupons();
                JSONArray mine = api.myCoupons();
                runOnUiThread(() -> renderCouponContent(page, available, mine));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "优惠券加载失败", error.getMessage(), () -> renderCoupons()));
            }
        }).start();
    }

    private void renderCouponContent(LinearLayout page, JSONArray available, JSONArray mine) {
        page.removeAllViews();
        page.addView(strong("可领取"));
        for (int i = 0; i < available.length(); i++) {
            JSONObject coupon = available.optJSONObject(i);
            if (coupon != null) {
                page.addView(couponCard(coupon, true));
            }
        }
        page.addView(strong("我的优惠券"));
        if (mine.length() == 0) {
            page.addView(muted("暂无已领取优惠券"));
        }
        for (int i = 0; i < mine.length(); i++) {
            JSONObject item = mine.optJSONObject(i);
            JSONObject coupon = item == null ? null : item.optJSONObject("coupon");
            page.addView(couponCard(coupon == null ? item : coupon, false));
        }
    }

    private View couponCard(JSONObject coupon, boolean claimable) {
        LinearLayout card = panel();
        if (coupon == null) {
            card.addView(muted("优惠券信息缺失"));
            return card;
        }
        card.addView(strong(coupon.optString("name", "优惠券")));
        card.addView(muted("满 " + coupon.optString("threshold_amount", "0") + " 减 " + coupon.optString("discount_amount", "0")));
        card.addView(muted(coupon.optString("start_at", "") + " - " + coupon.optString("end_at", "")));
        if (claimable) {
            Button claim = textOnlyButton("领取");
            claim.setTextColor(Color.rgb(37, 99, 235));
            claim.setOnClickListener(v -> claimCoupon(coupon.optString("coupon_id")));
            card.addView(claim, new LinearLayout.LayoutParams(-1, dp(42)));
        }
        return card;
    }

    private void claimCoupon(String couponId) {
        new Thread(() -> {
            try {
                api.claimCoupon(couponId);
                runOnUiThread(() -> {
                    toastLine("领取成功");
                    renderCoupons();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("领取失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderPromotions() {
        closeDrawer();
        activePage = "promotions";
        backStack.clear();
        baseScreen();
        addPageHeader("活动", "平台和商家促销");
        LinearLayout page = pageBody();
        page.addView(muted("正在加载活动..."));
        new Thread(() -> {
            try {
                JSONArray items = api.promotions();
                runOnUiThread(() -> renderPromotionItems(page, items));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "活动加载失败", error.getMessage(), () -> renderPromotions()));
            }
        }).start();
    }

    private void renderPromotionItems(LinearLayout page, JSONArray items) {
        page.removeAllViews();
        if (items.length() == 0) {
            page.addView(card("暂无活动", "稍后再来看看。"));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item == null) {
                continue;
            }
            LinearLayout card = panel();
            card.addView(strong(item.optString("name", "促销活动")));
            card.addView(muted(item.optString("scope", "platform") + " · 满 " + item.optString("threshold_amount", "0") + " 减 " + item.optString("discount_amount", "0")));
            card.addView(muted("有效期：" + item.optString("start_at", "") + " - " + item.optString("end_at", "")));
            page.addView(card);
        }
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
        addOrderActions(card, item);
        return card;
    }

    private void addOrderActions(LinearLayout card, JSONObject order) {
        String status = order.optString("status", "");
        String orderId = order.optString("order_id", "");
        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        if ("pending_payment".equals(status)) {
            Button pay = primaryButton("去支付");
            pay.setOnClickListener(v -> payOrder(orderId));
            actions.addView(pay, new LinearLayout.LayoutParams(0, dp(44), 1));
            Button cancel = textOnlyButton("取消订单");
            cancel.setTextColor(Color.rgb(185, 28, 28));
            cancel.setOnClickListener(v -> cancelOrder(orderId));
            actions.addView(cancel, new LinearLayout.LayoutParams(0, dp(44), 1));
        } else if ("shipped".equals(status)) {
            Button confirm = primaryButton("确认收货");
            confirm.setOnClickListener(v -> confirmOrder(orderId));
            actions.addView(confirm, new LinearLayout.LayoutParams(-1, dp(44)));
        } else if ("completed".equals(status)) {
            Button review = secondaryButton("评价商品");
            review.setOnClickListener(v -> openReviewDialog(order));
            actions.addView(review, new LinearLayout.LayoutParams(-1, dp(44)));
        }
        if (actions.getChildCount() > 0) {
            card.addView(actions);
        }
    }

    private void payOrder(String orderId) {
        new Thread(() -> {
            try {
                api.payOrder(orderId);
                runOnUiThread(() -> {
                    toastLine("支付成功");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("支付失败：" + error.getMessage()));
            }
        }).start();
    }

    private void cancelOrder(String orderId) {
        new Thread(() -> {
            try {
                api.cancelOrder(orderId, "暂时不买了");
                runOnUiThread(() -> {
                    toastLine("订单已取消");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("取消失败：" + error.getMessage()));
            }
        }).start();
    }

    private void confirmOrder(String orderId) {
        new Thread(() -> {
            try {
                api.confirmReceipt(orderId);
                runOnUiThread(() -> {
                    toastLine("已确认收货");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("确认失败：" + error.getMessage()));
            }
        }).start();
    }

    private void openReviewDialog(JSONObject order) {
        JSONArray items = order.optJSONArray("items");
        if (items == null || items.length() == 0) {
            toastLine("暂无可评价商品");
            return;
        }
        if (items.length() == 1) {
            showReviewForm(order, items.optJSONObject(0));
            return;
        }
        LinearLayout chooser = new LinearLayout(this);
        chooser.setOrientation(LinearLayout.VERTICAL);
        chooser.setPadding(dp(8), dp(8), dp(8), dp(4));
        AlertDialog dialog = new AlertDialog.Builder(this)
                .setTitle("选择评价商品")
                .setView(chooser)
                .setNegativeButton("取消", null)
                .create();
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item == null) {
                continue;
            }
            Button option = secondaryButton(item.optString("name", "商品") + " x" + item.optInt("quantity", 1));
            option.setOnClickListener(v -> {
                dialog.dismiss();
                showReviewForm(order, item);
            });
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, dp(46));
            params.bottomMargin = dp(8);
            chooser.addView(option, params);
        }
        dialog.show();
    }

    private void showReviewForm(JSONObject order, JSONObject item) {
        if (item == null) {
            toastLine("订单项信息缺失");
            return;
        }
        LinearLayout form = new LinearLayout(this);
        form.setOrientation(LinearLayout.VERTICAL);
        form.setPadding(dp(4), dp(4), dp(4), 0);
        form.addView(muted(item.optString("name", "商品")));
        EditText rating = inputField("评分 1-5", "5");
        EditText content = inputField("评价内容", "");
        form.addView(rating);
        form.addView(content);
        new AlertDialog.Builder(this)
                .setTitle("评价商品")
                .setView(form)
                .setNegativeButton("取消", null)
                .setPositiveButton("提交", (dialog, which) -> submitReview(order.optString("order_id"), item.optString("order_item_id"), rating.getText().toString(), content.getText().toString()))
                .show();
    }

    private void submitReview(String orderId, String orderItemId, String ratingText, String content) {
        if (content == null || content.trim().isEmpty()) {
            toastLine("请填写评价内容");
            return;
        }
        int rating = 5;
        try {
            rating = Math.max(1, Math.min(5, Integer.parseInt(ratingText.trim())));
        } catch (Exception ignored) {
        }
        int finalRating = rating;
        new Thread(() -> {
            try {
                JSONArray tags = new JSONArray();
                api.reviewOrderItem(orderId, orderItemId, finalRating, content, tags);
                runOnUiThread(() -> {
                    toastLine("评价已提交");
                    renderOrders();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("评价失败：" + error.getMessage()));
            }
        }).start();
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

    private View addUserMessageBubble(String text, JSONArray attachments) {
        if (attachments == null || attachments.length() == 0) {
            return addBubble(text == null ? "" : text, true);
        }
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
            textView.setLineSpacing(4, 1);
            textView.setTextColor(Color.WHITE);
            textView.setPadding(0, 0, 0, dp(8));
            bubble.addView(textView, new LinearLayout.LayoutParams(-1, -2));
        }
        LinearLayout attachmentList = new LinearLayout(this);
        attachmentList.setOrientation(attachments.length() > 1 ? LinearLayout.HORIZONTAL : LinearLayout.VERTICAL);
        for (int i = 0; i < attachments.length(); i++) {
            JSONObject attachment = attachments.optJSONObject(i);
            if (attachment == null) {
                continue;
            }
            View view = isImageAttachment(attachment) ? userImageAttachmentView(attachment) : userFileAttachmentView(attachment);
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(attachments.length() > 1 ? dp(104) : dp(176), attachments.length() > 1 ? dp(104) : dp(132));
            params.setMargins(0, 0, i < attachments.length() - 1 ? dp(8) : 0, 0);
            attachmentList.addView(view, params);
        }
        if (attachments.length() > 1) {
            HorizontalScrollView scroll = new HorizontalScrollView(this);
            scroll.setHorizontalScrollBarEnabled(false);
            scroll.addView(attachmentList);
            bubble.addView(scroll, new LinearLayout.LayoutParams(-1, -2));
        } else {
            bubble.addView(attachmentList, new LinearLayout.LayoutParams(-1, -2));
        }
        enableCopy(bubble, userMessageCopyText(safeText, attachments));
        shell.addView(bubble);
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

    private View userImageAttachmentView(JSONObject attachment) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(rounded(Color.rgb(235, 238, 243), dp(12)));
        ImageView image = new ImageView(this);
        image.setScaleType(ImageView.ScaleType.CENTER_CROP);
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
        String local = attachment.optString("local_uri", "");
        if (!local.isEmpty()) {
            return local;
        }
        String url = attachment.optString("url", "");
        if (!url.isEmpty()) {
            return url;
        }
        return attachment.optString("object_key", "");
    }

    private void loadAttachmentImage(ImageView image, String uri) {
        if (image == null || uri == null || uri.trim().isEmpty()) {
            return;
        }
        String value = uri.trim();
        if (value.startsWith("http://") || value.startsWith("https://")) {
            new Thread(() -> {
                try (InputStream inputStream = new URL(value).openStream()) {
                    Bitmap bitmap = BitmapFactory.decodeStream(inputStream);
                    if (bitmap != null) {
                        runOnUiThread(() -> image.setImageBitmap(bitmap));
                    }
                } catch (Exception ignored) {
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
        LinearLayout bubble = new LinearLayout(this);
        bubble.setOrientation(LinearLayout.VERTICAL);
        bubble.setPadding(dp(12), dp(9), dp(12), dp(9));
        bubble.setBackground(rounded(Color.WHITE, dp(16)));
        int bubbleWidth = (int) (getResources().getDisplayMetrics().widthPixels * 0.74f);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(bubbleWidth, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = Gravity.LEFT;
        params.setMargins(0, dp(6), 0, dp(6));
        enableCopy(bubble, text);
        TextView firstText = null;
        for (MarkdownPart part : splitMarkdownParts(text)) {
            if (part.table) {
                addMarkdownTableToBubble(bubble, part.content);
            } else if (!part.content.trim().isEmpty()) {
                TextView normal = markdownTextView(part.content);
                if (firstText == null) {
                    firstText = normal;
                }
                bubble.addView(normal, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            }
        }
        if (firstText == null) {
            firstText = markdownTextView(text);
            bubble.addView(firstText, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        }
        parent.addView(bubble, params);
        scrollBottom();
        return firstText;
    }

    private void addMarkdownTableToBubble(LinearLayout bubble, String markdown) {
        if (bubble == null || markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        HorizontalScrollView scroll = new HorizontalScrollView(this);
        scroll.setHorizontalScrollBarEnabled(true);
        View table = markdownTableView(markdown);
        enableCopy(scroll, markdown);
        enableCopy(table, markdown);
        scroll.addView(table, new HorizontalScrollView.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        LinearLayout.LayoutParams tableParams = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        tableParams.setMargins(0, dp(8), 0, dp(10));
        bubble.addView(scroll, tableParams);
    }

    private void addMarkdownPartsToBubble(LinearLayout bubble, String markdown) {
        if (bubble == null || markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        for (MarkdownPart part : splitMarkdownParts(markdown)) {
            if (part.table) {
                addMarkdownTableToBubble(bubble, part.content);
            } else if (!part.content.trim().isEmpty()) {
                bubble.addView(markdownTextView(part.content), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            }
        }
    }

    private View markdownTableView(String markdown) {
        LinearLayout table = new LinearLayout(this);
        table.setOrientation(LinearLayout.VERTICAL);
        table.setMinimumWidth((int) (getResources().getDisplayMetrics().widthPixels * 1.05f));
        String[] lines = markdown == null ? new String[0] : markdown.split("\\n");
        boolean headerRendered = false;
        int bodyRow = 0;
        for (String line : lines) {
            if (isMarkdownSeparatorLine(line)) {
                continue;
            }
            List<String> cells = markdownTableCells(line);
            if (cells.isEmpty()) {
                continue;
            }
            LinearLayout row = new EqualHeightTableRow(this);
            row.setOrientation(LinearLayout.HORIZONTAL);
            boolean header = !headerRendered;
            row.setMinimumHeight(header ? dp(48) : dp(54));
            for (int i = 0; i < cells.size(); i++) {
                TextView cell = new TextView(this);
                MarkdownRenderer.setMarkdown(cell, sanitizeAgentMarkdown(cells.get(i)).visibleMarkdown, 13);
                cell.setTextColor(header ? Color.rgb(17, 24, 39) : Color.rgb(55, 65, 81));
                cell.setTypeface(Typeface.DEFAULT, header ? Typeface.BOLD : Typeface.NORMAL);
                cell.setPadding(dp(12), dp(10), dp(12), dp(12));
                cell.setGravity(Gravity.CENTER_VERTICAL);
                cell.setBackground(tableCellBackground(header, bodyRow % 2 == 1));
                row.addView(cell, new LinearLayout.LayoutParams(i == 0 ? dp(142) : dp(128), ViewGroup.LayoutParams.MATCH_PARENT));
            }
            table.addView(row, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            if (headerRendered) {
                bodyRow++;
            }
            headerRendered = true;
        }
        if (table.getChildCount() == 0) {
            return markdownTextView(markdown);
        }
        return table;
    }

    private List<String> markdownTableCells(String line) {
        ArrayList<String> cells = new ArrayList<>();
        if (line == null) {
            return cells;
        }
        String trimmed = line.trim();
        if (trimmed.startsWith("|")) {
            trimmed = trimmed.substring(1);
        }
        if (trimmed.endsWith("|")) {
            trimmed = trimmed.substring(0, trimmed.length() - 1);
        }
        String[] parts = trimmed.split("\\|", -1);
        for (String part : parts) {
            cells.add(part.trim());
        }
        return cells;
    }

    private GradientDrawable tableCellBackground(boolean header, boolean alternate) {
        GradientDrawable drawable = new GradientDrawable();
        drawable.setColor(header ? Color.rgb(238, 242, 247) : (alternate ? Color.rgb(250, 251, 252) : Color.WHITE));
        drawable.setStroke(1, Color.rgb(229, 231, 235));
        return drawable;
    }

    private TextView markdownTextView(String markdown) {
        TextView view = new TextView(this);
        MarkdownRenderer.setMarkdown(view, sanitizeAgentMarkdown(markdown).visibleMarkdown);
        view.setTextSize(15);
        view.setLineSpacing(4, 1);
        view.setTextColor(Color.rgb(24, 30, 37));
        view.setLinksClickable(true);
        enableCopy(view, markdown);
        return view;
    }

    private List<MarkdownPart> splitMarkdownParts(String markdown) {
        ArrayList<MarkdownPart> parts = new ArrayList<>();
        if (markdown == null || markdown.isEmpty()) {
            return parts;
        }
        String[] lines = markdown.split("\\n", -1);
        StringBuilder text = new StringBuilder();
        int i = 0;
        while (i < lines.length) {
            if (isMarkdownTableStart(lines, i)) {
                if (text.length() > 0) {
                    parts.add(new MarkdownPart(false, text.toString().trim()));
                    text.setLength(0);
                }
                StringBuilder table = new StringBuilder();
                while (i < lines.length && isMarkdownTableLine(lines[i])) {
                    if (table.length() > 0) {
                        table.append('\n');
                    }
                    table.append(lines[i]);
                    i++;
                }
                parts.add(new MarkdownPart(true, table.toString()));
                continue;
            }
            text.append(lines[i]);
            if (i < lines.length - 1) {
                text.append('\n');
            }
            i++;
        }
        if (text.length() > 0) {
            parts.add(new MarkdownPart(false, text.toString().trim()));
        }
        return parts;
    }

    private boolean isMarkdownTableStart(String[] lines, int index) {
        return index + 1 < lines.length && isMarkdownTableLine(lines[index]) && isMarkdownSeparatorLine(lines[index + 1]);
    }

    private boolean isMarkdownTableLine(String line) {
        return line != null && line.trim().startsWith("|") && line.trim().contains("|");
    }

    private boolean isMarkdownSeparatorLine(String line) {
        return line != null && Pattern.compile("^\\s*\\|\\s*:?-{3,}:?\\s*(\\|\\s*:?-{3,}:?\\s*)+\\|?\\s*$").matcher(line).find();
    }

    private static class MarkdownPart {
        final boolean table;
        final String content;

        MarkdownPart(boolean table, String content) {
            this.table = table;
            this.content = content == null ? "" : content;
        }
    }

    private interface CopyTextProvider {
        String copyText();
    }

    private static class EqualHeightTableRow extends LinearLayout {
        EqualHeightTableRow(android.content.Context context) {
            super(context);
        }

        @Override
        protected void onMeasure(int widthMeasureSpec, int heightMeasureSpec) {
            super.onMeasure(widthMeasureSpec, heightMeasureSpec);
            int maxHeight = getSuggestedMinimumHeight();
            for (int i = 0; i < getChildCount(); i++) {
                View child = getChildAt(i);
                if (child.getVisibility() != GONE) {
                    maxHeight = Math.max(maxHeight, child.getMeasuredHeight());
                }
            }
            if (maxHeight <= 0) {
                return;
            }
            for (int i = 0; i < getChildCount(); i++) {
                View child = getChildAt(i);
                if (child.getVisibility() != GONE) {
                    int childWidth = child.getMeasuredWidth();
                    child.measure(
                            View.MeasureSpec.makeMeasureSpec(childWidth, View.MeasureSpec.EXACTLY),
                            View.MeasureSpec.makeMeasureSpec(maxHeight, View.MeasureSpec.EXACTLY)
                    );
                }
            }
            setMeasuredDimension(getMeasuredWidth(), maxHeight);
        }
    }

    private boolean containsMarkdownTable(String text) {
        if (text == null) {
            return false;
        }
        return Pattern.compile("(?m)^\\s*\\|.+\\|\\s*$").matcher(text).find()
                && Pattern.compile("(?m)^\\s*\\|\\s*:?-{3,}:?\\s*(\\|\\s*:?-{3,}:?\\s*)+\\|?\\s*$").matcher(text).find();
    }

    private int findMarkdownTableStart(String markdown) {
        if (markdown == null || markdown.isEmpty()) {
            return -1;
        }
        String[] lines = markdown.split("\\n", -1);
        int offset = 0;
        for (int i = 0; i < lines.length; i++) {
            if (isMarkdownTableStart(lines, i)) {
                return offset;
            }
            offset += lines[i].length();
            if (i < lines.length - 1) {
                offset++;
            }
        }
        return -1;
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

    private String productField(JSONObject item, String primary, String fallback) {
        return productField(item, primary, fallback, "");
    }

    private String productField(JSONObject item, String primary, String fallback, String defaultValue) {
        if (item == null) {
            return defaultValue;
        }
        String value = item.optString(primary, "");
        if (!value.isEmpty()) {
            return value;
        }
        value = item.optString(fallback, "");
        return value.isEmpty() ? defaultValue : value;
    }

    private String productField(JSONObject item, String primary, String fallback, String secondFallback, String defaultValue) {
        String value = productField(item, primary, fallback, "");
        if (!value.isEmpty()) {
            return value;
        }
        value = item == null ? "" : item.optString(secondFallback, "");
        return value.isEmpty() ? defaultValue : value;
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
        int panelWidth = drawerPanel.getWidth() == 0 ? (int) (getResources().getDisplayMetrics().widthPixels * 0.82f) : drawerPanel.getWidth();
        FrameLayout closingLayer = drawerLayer;
        LinearLayout closingPanel = drawerPanel;
        drawerLayer = null;
        drawerPanel = null;
        drawerHistoryList = null;
        drawerSearchInput = null;
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
                restoreKeyboardModeForActivePage();
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
            } else if ("assistant".equals(message.role)) {
                renderAgentSegments(jsonArray(message.segmentsJson), message.content, message.blocksJson, chatList);
                renderFollowups(jsonArray(message.followupsJson), chatList);
            } else {
                addUserMessageBubble(message.content, jsonArray(message.attachmentsJson));
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
        return productImage(imageUrl, sizeDp, ImageView.ScaleType.CENTER_CROP, false);
    }

    private View productDetailImage(String imageUrl) {
        return productImage(imageUrl, 240, ImageView.ScaleType.FIT_CENTER, true);
    }

    private View productImage(String imageUrl, int sizeDp, ImageView.ScaleType scaleType, boolean previewable) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(rounded(Color.rgb(243, 244, 246), dp(12)));
        TextView placeholder = imagePlaceholder();
        frame.addView(placeholder, new FrameLayout.LayoutParams(-1, -1));
        ImageView image = new ImageView(this);
        image.setScaleType(scaleType);
        String url = api.absoluteUrl(imageUrl);
        if (!url.isEmpty()) {
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
            imageLoader.load(image, url);
            if (previewable) {
                frame.setOnClickListener(v -> showImagePreview(url));
            }
        }
        frame.setLayoutParams(new LinearLayout.LayoutParams(dp(sizeDp), dp(sizeDp)));
        return frame;
    }

    private void showImagePreview(String url) {
        FrameLayout preview = new FrameLayout(this);
        preview.setBackgroundColor(Color.BLACK);
        ImageView image = new ImageView(this);
        image.setScaleType(ImageView.ScaleType.FIT_CENTER);
        imageLoader.load(image, url);
        preview.addView(image, new FrameLayout.LayoutParams(-1, -1));
        TextView close = new TextView(this);
        close.setText("×");
        close.setTextSize(28);
        close.setGravity(Gravity.CENTER);
        close.setTextColor(Color.WHITE);
        close.setOnClickListener(v -> {
            if (preview.getParent() == root) {
                root.removeView(preview);
            }
        });
        FrameLayout.LayoutParams closeParams = new FrameLayout.LayoutParams(dp(56), dp(56), Gravity.RIGHT | Gravity.TOP);
        closeParams.topMargin = dp(24);
        closeParams.rightMargin = dp(12);
        preview.addView(close, closeParams);
        preview.setOnClickListener(v -> {
            if (preview.getParent() == root) {
                root.removeView(preview);
            }
        });
        root.addView(preview, new FrameLayout.LayoutParams(-1, -1));
    }

    private View accountAvatarView(int sizePx, int textSp) {
        String avatarUrl = api.absoluteUrl(sessionStore.avatarUrl());
        if (!avatarUrl.isEmpty()) {
            ImageView image = new CircleImageView(this);
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

    private void addExpandableTextPanel(LinearLayout page, String title, String text, int collapsedLines) {
        if (text == null || text.trim().isEmpty()) {
            return;
        }
        LinearLayout panel = panel();
        final boolean[] expanded = {false};
        Runnable render = new Runnable() {
            @Override
            public void run() {
                panel.removeAllViews();
                panel.addView(strong(title));
                TextView body = muted(text);
                body.setSingleLine(false);
                if (!expanded[0]) {
                    body.setMaxLines(collapsedLines);
                } else {
                    body.setMaxLines(Integer.MAX_VALUE);
                }
                panel.addView(body);
                if (text.length() > 80) {
                    Button more = textOnlyButton(expanded[0] ? "收起" : "显示更多");
                    more.setTextColor(Color.rgb(37, 99, 235));
                    more.setTextSize(14);
                    more.setOnClickListener(v -> {
                        expanded[0] = !expanded[0];
                        run();
                    });
                    panel.addView(more, new LinearLayout.LayoutParams(-2, dp(36)));
                }
            }
        };
        render.run();
        page.addView(panel);
    }

    private void addExpandableArrayPanel(LinearLayout page, String title, JSONArray items, int collapsedCount) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = panel();
        final boolean[] expanded = {false};
        Runnable render = new Runnable() {
            @Override
            public void run() {
                panel.removeAllViews();
                panel.addView(strong(title));
                int count = expanded[0] ? items.length() : Math.min(items.length(), collapsedCount);
                for (int i = 0; i < count; i++) {
                    panel.addView(muted("• " + items.optString(i)));
                }
                if (items.length() > collapsedCount) {
                    Button more = textOnlyButton(expanded[0] ? "收起" : "显示更多");
                    more.setTextColor(Color.rgb(37, 99, 235));
                    more.setTextSize(14);
                    more.setOnClickListener(v -> {
                        expanded[0] = !expanded[0];
                        run();
                    });
                    panel.addView(more, new LinearLayout.LayoutParams(-2, dp(36)));
                }
            }
        };
        render.run();
        page.addView(panel);
    }

    private void addExpandableAttributesPanel(LinearLayout page, JSONArray items, int collapsedCount) {
        if (items == null || items.length() == 0) {
            return;
        }
        LinearLayout panel = panel();
        final boolean[] expanded = {false};
        Runnable render = new Runnable() {
            @Override
            public void run() {
                panel.removeAllViews();
                panel.addView(strong("商品参数"));
                int count = expanded[0] ? items.length() : Math.min(items.length(), collapsedCount);
                for (int i = 0; i < count; i++) {
                    JSONObject item = items.optJSONObject(i);
                    if (item != null) {
                        panel.addView(muted(item.optString("key", "") + "：" + item.optString("value", "") + item.optString("unit", "")));
                    }
                }
                if (items.length() > collapsedCount) {
                    Button more = textOnlyButton(expanded[0] ? "收起" : "显示更多");
                    more.setTextColor(Color.rgb(37, 99, 235));
                    more.setTextSize(14);
                    more.setOnClickListener(v -> {
                        expanded[0] = !expanded[0];
                        run();
                    });
                    panel.addView(more, new LinearLayout.LayoutParams(-2, dp(36)));
                }
            }
        };
        render.run();
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
        if ("pending_pay".equals(status) || "pending_payment".equals(status)) return "待支付";
        if ("paid".equals(status)) return "已支付";
        if ("pending_ship".equals(status)) return "待发货";
        if ("shipped".equals(status)) return "已发货";
        if ("completed".equals(status)) return "已完成";
        if ("cancelled".equals(status) || "canceled".equals(status)) return "已取消";
        if ("closed_timeout".equals(status)) return "支付超时关闭";
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
        String[] actions = {pinText, "更改会话标题", "删除会话"};
        new AlertDialog.Builder(this)
                .setTitle(item.title == null || item.title.isEmpty() ? "导购会话" : item.title)
                .setItems(actions, (dialog, which) -> {
                    if (which == 0) {
                        toggleHistoryPinned(item);
                    } else if (which == 1) {
                        showRenameSessionDialog(item);
                    } else {
                        confirmDeleteSession(item);
                    }
                })
                .show();
    }

    private void toggleHistoryPinned(LocalChatStore.SessionSummary item) {
        boolean pinned = !item.pinned;
        chatStore.pinSession(item.localSessionId, pinned);
        refreshDrawerContent();
        if (item.serverSessionId != null && !item.serverSessionId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.pinSession(item.serverSessionId, pinned);
                } catch (Exception error) {
                    handleApiError(error);
                }
            }).start();
        }
    }

    private void showRenameSessionDialog(LocalChatStore.SessionSummary item) {
        EditText editor = new EditText(this);
        editor.setSingleLine(true);
        editor.setText(item.title == null ? "" : item.title);
        editor.setSelection(editor.getText().length());
        editor.setPadding(dp(12), dp(8), dp(12), dp(8));
        new AlertDialog.Builder(this)
                .setTitle("更改会话标题")
                .setView(editor)
                .setNegativeButton("取消", null)
                .setPositiveButton("保存", (dialog, which) -> {
                    String title = editor.getText().toString().trim();
                    if (title.isEmpty()) {
                        toastLine("标题不能为空");
                        return;
                    }
                    chatStore.renameSession(item.localSessionId, title);
                    refreshDrawerContent();
                    if (item.serverSessionId != null && !item.serverSessionId.isEmpty()) {
                        new Thread(() -> {
                            try {
                                api.updateSession(item.serverSessionId, title, "");
                            } catch (Exception error) {
                                handleApiError(error);
                            }
                        }).start();
                    }
                })
                .show();
    }

    private void confirmDeleteSession(LocalChatStore.SessionSummary item) {
        new AlertDialog.Builder(this)
                .setTitle("删除会话")
                .setMessage("删除后侧栏不再显示该会话。是否继续？")
                .setNegativeButton("取消", null)
                .setPositiveButton("删除", (dialog, which) -> deleteHistorySession(item))
                .show();
    }

    private void deleteHistorySession(LocalChatStore.SessionSummary item) {
        chatStore.deleteSession(item.localSessionId);
        boolean deletingCurrent = item.localSessionId != null && item.localSessionId.equals(localSessionId);
        if (item.serverSessionId != null && !item.serverSessionId.isEmpty()) {
            new Thread(() -> {
                try {
                    api.deleteSession(item.serverSessionId);
                    runOnUiThread(this::refreshDrawerContent);
                } catch (Exception error) {
                    handleApiError(error);
                }
            }).start();
        }
        if (deletingCurrent) {
            createFreshLocalSession();
        }
        drawerHistoryLocallyMutated = true;
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
                        long createdAt = parseRemoteTime(message.optString("created_at", ""));
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
        user.setOnClickListener(v -> renderSettings());
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
        if (sessionStore.token().isEmpty()) {
            renderLoginPage();
            return;
        }
        activePage = "settings";
        useKeyboardOverlay();
        backStack.clear();
        backStack.push(() -> renderChatHome());
        baseScreen();
        content.addView(createTopBar("设置", null), new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();
        LinearLayout header = new LinearLayout(this);
        header.setOrientation(LinearLayout.VERTICAL);
        header.setGravity(Gravity.CENTER_HORIZONTAL);
        header.setPadding(0, dp(36), 0, dp(24));
        View avatar = accountAvatarView(dp(118), 36);
        avatar.setOnClickListener(v -> renderAvatarPreviewPage());
        header.addView(avatar, new LinearLayout.LayoutParams(dp(118), dp(118)));
        TextView name = title(sessionStore.nickname().isEmpty() ? "用户名" : sessionStore.nickname());
        name.setGravity(Gravity.CENTER);
        name.setOnClickListener(v -> renderEditProfilePage());
        header.addView(name, new LinearLayout.LayoutParams(-1, -2));
        TextView id = muted("用户id：" + emptyFallback(sessionStore.accountId(), "-"));
        id.setGravity(Gravity.CENTER);
        header.addView(id, new LinearLayout.LayoutParams(-1, -2));
        Button account = secondaryButton("账号管理");
        account.setTextSize(18);
        account.setOnClickListener(v -> renderAccountPage());
        LinearLayout.LayoutParams accountParams = new LinearLayout.LayoutParams(dp(168), dp(52));
        accountParams.topMargin = dp(16);
        header.addView(account, accountParams);
        page.addView(header);

        LinearLayout group = panel();
        group.addView(settingsRow("?", "帮助", v -> renderHelpPage(), Color.rgb(124, 58, 237)));
        group.addView(settingsRow("i", "关于", v -> renderAboutPage(), Color.rgb(59, 130, 246)));
        group.addView(settingsRow("⚙", "高级设置", v -> renderAdvancedSettingsPage(), Color.rgb(37, 99, 235)));
        group.addView(settingsRow("→", "退出登录", v -> confirmLogout(), Color.rgb(239, 68, 68)));
        page.addView(group);

        TextView version = muted("Version: " + BuildConfig.VERSION_NAME + "\n由 AI 大模型提供支持");
        version.setGravity(Gravity.CENTER);
        page.addView(version, new LinearLayout.LayoutParams(-1, -2));
    }

    private void renderAdvancedSettingsPage() {
        closeDrawer();
        if (sessionStore.token().isEmpty()) {
            renderLoginPage();
            return;
        }
        activePage = "advanced_settings";
        useKeyboardOverlay();
        baseScreen();
        content.addView(createBackTopBar("高级设置", () -> renderSettings()), new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();

        LinearLayout feature = panel();
        feature.addView(strong("功能设置"));
        CheckBox showReturnChat = new CheckBox(this);
        showReturnChat.setText("侧栏显示“返回聊天”按钮");
        showReturnChat.setTextSize(15);
        showReturnChat.setTextColor(Color.rgb(24, 30, 37));
        showReturnChat.setChecked(sessionStore.showDrawerReturnChat());
        showReturnChat.setOnCheckedChangeListener((buttonView, isChecked) -> sessionStore.saveShowDrawerReturnChat(isChecked));
        feature.addView(showReturnChat, new LinearLayout.LayoutParams(-1, dp(52)));
        page.addView(feature);

        if (BuildConfig.SHOW_TEST_SERVER_SETTINGS) {
            LinearLayout dev = panel();
            EditText apiBase = inputField("后端地址", sessionStore.apiBase());
            dev.addView(strong("测试后端"));
            dev.addView(apiBase);
            Button saveApi = primaryButton("保存测试地址");
            saveApi.setOnClickListener(v -> saveApiBaseFromInput(apiBase.getText().toString()));
            addFormButton(dev, saveApi);
            page.addView(dev);
        }
    }

    private void saveApiBaseFromInput(String value) {
        String base = value == null ? "" : value.trim();
        while (base.endsWith("/")) {
            base = base.substring(0, base.length() - 1);
        }
        if (!base.contains("/api/v1")) {
            toastLine("后端地址应包含 /api/v1");
            return;
        }
        sessionStore.saveApiBase(base);
        api = new ApiClient(sessionStore);
        toastLine("已保存测试后端地址");
    }

    private View settingsRow(String iconText, String label, View.OnClickListener listener, int iconColor) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(4), dp(8), dp(4), dp(8));
        TextView icon = new TextView(this);
        icon.setText(iconText);
        icon.setTextSize(20);
        icon.setTypeface(Typeface.DEFAULT_BOLD);
        icon.setTextColor(Color.WHITE);
        icon.setGravity(Gravity.CENTER);
        icon.setBackground(rounded(iconColor, dp(12)));
        row.addView(icon, new LinearLayout.LayoutParams(dp(44), dp(44)));
        TextView text = strong(label);
        LinearLayout.LayoutParams textParams = new LinearLayout.LayoutParams(0, -2, 1);
        textParams.leftMargin = dp(18);
        row.addView(text, textParams);
        TextView arrow = new TextView(this);
        arrow.setText("›");
        arrow.setTextSize(24);
        arrow.setTextColor(Color.rgb(156, 163, 175));
        arrow.setGravity(Gravity.CENTER);
        row.addView(arrow, new LinearLayout.LayoutParams(dp(28), -1));
        row.setOnClickListener(listener);
        return row;
    }

    private void renderAvatarPreviewPage() {
        closeDrawer();
        activePage = "avatar";
        baseScreen();
        root.setBackgroundColor(Color.BLACK);
        content.addView(createBackTopBar("", () -> renderSettings()), new LinearLayout.LayoutParams(-1, dp(56)));
        FrameLayout avatarArea = new FrameLayout(this);
        ImageView image = new CircleImageView(this);
        image.setScaleType(ImageView.ScaleType.CENTER_CROP);
        image.setBackground(rounded(Color.rgb(236, 244, 255), dp(130)));
        String url = api.absoluteUrl(sessionStore.avatarUrl());
        if (!url.isEmpty()) {
            imageLoader.load(image, url);
        }
        avatarArea.addView(image, new FrameLayout.LayoutParams(dp(260), dp(260), Gravity.CENTER));
        content.addView(avatarArea, new LinearLayout.LayoutParams(-1, 0, 1));
        Button edit = secondaryButton("编辑个人资料");
        edit.setTextSize(18);
        edit.setOnClickListener(v -> renderEditProfilePage(() -> renderAvatarPreviewPage()));
        FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(-1, dp(56), Gravity.BOTTOM);
        params.leftMargin = dp(20);
        params.rightMargin = dp(20);
        params.bottomMargin = dp(36);
        root.addView(edit, params);
        image.setOnLongClickListener(v -> {
            saveAvatarToGallery(url);
            return true;
        });
    }

    private void saveAvatarToGallery(String url) {
        if (url == null || url.trim().isEmpty()) {
            toastLine("当前没有可保存的头像");
            return;
        }
        new Thread(() -> {
            try (InputStream inputStream = new URL(url).openStream()) {
                Bitmap bitmap = BitmapFactory.decodeStream(inputStream);
                if (bitmap == null) {
                    throw new IllegalArgumentException("头像图片不可用");
                }
                String saved = MediaStore.Images.Media.insertImage(getContentResolver(), bitmap, "xzxg_avatar_" + System.currentTimeMillis(), "小猪小狗导购头像");
                bitmap.recycle();
                runOnUiThread(() -> toastLine(saved == null ? "保存失败" : "已保存到相册"));
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("保存失败：" + error.getMessage()));
            }
        }).start();
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
            long createdAt = parseRemoteTime(run.optString("updated_at", ""));
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

    private void renderEditProfilePage() {
        renderEditProfilePage(() -> renderSettings());
    }

    private void renderEditProfilePage(Runnable backAction) {
        closeDrawer();
        if (sessionStore.token().isEmpty()) {
            renderLoginPage();
            return;
        }
        activePage = "edit_profile";
        baseScreen();
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(dp(18), 0, dp(18), 0);
        TextView cancel = topBarTextButton("取消", Color.BLACK);
        Runnable safeBackAction = backAction == null ? () -> renderSettings() : backAction;
        editProfileBackAction = safeBackAction;
        cancel.setOnClickListener(v -> safeBackAction.run());
        bar.addView(cancel, new LinearLayout.LayoutParams(0, -1, 1));
        TextView heading = title("个人资料");
        heading.setGravity(Gravity.CENTER);
        heading.setIncludeFontPadding(false);
        bar.addView(heading, new LinearLayout.LayoutParams(0, -1, 1));
        TextView done = topBarTextButton("保存", Color.rgb(180, 198, 230));
        done.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        done.setEnabled(false);
        bar.addView(done, new LinearLayout.LayoutParams(0, -1, 1));
        content.addView(bar, new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();
        page.setGravity(Gravity.CENTER_HORIZONTAL);
        FrameLayout avatarWrap = new FrameLayout(this);
        TextView avatarInitial = new TextView(this);
        avatarInitial.setText(initialOf(sessionStore.nickname()));
        avatarInitial.setTextSize(38);
        avatarInitial.setTypeface(Typeface.DEFAULT_BOLD);
        avatarInitial.setGravity(Gravity.CENTER);
        avatarInitial.setTextColor(Color.rgb(30, 64, 175));
        avatarInitial.setBackground(rounded(Color.rgb(219, 234, 254), dp(64)));
        avatarWrap.addView(avatarInitial, new FrameLayout.LayoutParams(dp(128), dp(128), Gravity.CENTER));
        ImageView avatarImage = new CircleImageView(this);
        avatarImage.setScaleType(ImageView.ScaleType.CENTER_CROP);
        avatarImage.setBackground(rounded(Color.rgb(236, 244, 255), dp(64)));
        avatarImage.setVisibility(View.GONE);
        String avatarUrl = api.absoluteUrl(sessionStore.avatarUrl());
        if (!avatarUrl.isEmpty()) {
            avatarImage.setVisibility(View.VISIBLE);
            avatarInitial.setVisibility(View.GONE);
            imageLoader.load(avatarImage, avatarUrl);
        }
        avatarWrap.addView(avatarImage, new FrameLayout.LayoutParams(dp(128), dp(128), Gravity.CENTER));
        TextView plus = new TextView(this);
        plus.setText("+");
        plus.setTextSize(30);
        plus.setTextColor(Color.WHITE);
        plus.setGravity(Gravity.CENTER);
        plus.setBackground(rounded(Color.rgb(37, 99, 235), dp(22)));
        FrameLayout.LayoutParams plusParams = new FrameLayout.LayoutParams(dp(44), dp(44), Gravity.RIGHT | Gravity.BOTTOM);
        avatarWrap.addView(plus, plusParams);
        avatarWrap.setOnClickListener(v -> pickAvatar());
        page.addView(avatarWrap, new LinearLayout.LayoutParams(dp(150), dp(150)));
        TextView label = strong("昵称");
        label.setGravity(Gravity.LEFT);
        page.addView(label, new LinearLayout.LayoutParams(-1, -2));
        EditText nickname = inputField("昵称", sessionStore.nickname());
        page.addView(nickname);
        final boolean[] changed = {false};
        pendingAvatarBytes = null;
        pendingAvatarPreview = null;
        pendingAvatarChanged = () -> {
            changed[0] = true;
            done.setEnabled(true);
            done.setTextColor(Color.rgb(37, 99, 235));
            if (pendingAvatarPreview != null) {
                avatarImage.setVisibility(View.VISIBLE);
                avatarInitial.setVisibility(View.GONE);
                avatarImage.setImageBitmap(pendingAvatarPreview);
            }
            toastLine("头像已裁剪为正方形");
        };
        nickname.addTextChangedListener(new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            @Override public void onTextChanged(CharSequence s, int start, int before, int count) {
                changed[0] = true;
                done.setEnabled(true);
                done.setTextColor(Color.rgb(37, 99, 235));
            }
            @Override public void afterTextChanged(Editable s) {}
        });
        done.setOnClickListener(v -> {
            if (!changed[0]) {
                return;
            }
            saveProfileChanges(nickname.getText().toString(), safeBackAction);
        });
    }

    private void saveProfileChanges(String nickname) {
        saveProfileChanges(nickname, () -> renderSettings());
    }

    private void saveProfileChanges(String nickname, Runnable backAction) {
        String name = nickname == null ? "" : nickname.trim();
        if (name.isEmpty()) {
            toastLine("昵称不能为空");
            return;
        }
        new Thread(() -> {
            try {
                api.profile();
                String avatarUrl = "";
                if (pendingAvatarBytes != null) {
                    JSONObject upload = api.uploadAvatar("avatar.jpg", pendingAvatarMime, pendingAvatarBytes);
                    avatarUrl = upload.optString("url", "");
                }
                JSONObject profile = api.updateProfile(name, avatarUrl);
                saveAccountSession(sessionStore.token(), profile, sessionStore.role(), name);
                pendingAvatarBytes = null;
                pendingAvatarPreview = null;
                pendingAvatarChanged = null;
                runOnUiThread(() -> {
                    toastLine("资料已保存");
                    if (backAction != null) {
                        backAction.run();
                    } else {
                        renderSettings();
                    }
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    String message = error.getMessage();
                    if (message != null && message.contains("404")) {
                        toastLine("保存失败：当前后端不是最新版，请切换测试后端");
                    } else {
                        toastLine("保存失败：" + message);
                    }
                });
            }
        }).start();
    }

    private void renderAccountPage() {
        closeDrawer();
        if (sessionStore.token().isEmpty()) {
            renderLoginPage();
            return;
        }
        activePage = "account";
        baseScreen();
        content.addView(createBackTopBar("账号管理", () -> renderSettings()), new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();
        LinearLayout user = panel();
        LinearLayout userRow = new LinearLayout(this);
        userRow.setGravity(Gravity.CENTER_VERTICAL);
        userRow.addView(accountAvatarView(dp(78), 28), new LinearLayout.LayoutParams(dp(78), dp(78)));
        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        info.addView(title(sessionStore.nickname().isEmpty() ? "用户名" : sessionStore.nickname()));
        info.addView(muted("用户id：" + emptyFallback(sessionStore.accountId(), "-")));
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = dp(18);
        userRow.addView(info, infoParams);
        user.addView(userRow);
        page.addView(user);
        LinearLayout group1 = panel();
        group1.addView(settingsRow("▦", "个人资料", v -> renderEditProfilePage(() -> renderAccountPage()), Color.rgb(124, 58, 237)));
        page.addView(group1);
        LinearLayout group2 = panel();
        group2.addView(accountInfoRow("☎", "手机号", maskPhone(sessionStore.phone()), v -> editContact(true)));
        group2.addView(accountInfoRow("@", "邮箱", maskEmail(sessionStore.email()), v -> editContact(false)));
        page.addView(group2);
        LinearLayout group3 = panel();
        group3.addView(settingsRow("×", "删除账号", v -> confirmDeleteAccount(), Color.rgb(239, 68, 68)));
        group3.addView(settingsRow("→", "退出登录", v -> confirmLogout(), Color.rgb(239, 68, 68)));
        page.addView(group3);
    }

    private View accountInfoRow(String iconText, String label, String value, View.OnClickListener listener) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.setPadding(dp(4), dp(8), dp(4), dp(8));
        TextView icon = new TextView(this);
        icon.setText(iconText);
        icon.setTextSize(18);
        icon.setTextColor(Color.WHITE);
        icon.setGravity(Gravity.CENTER);
        icon.setBackground(rounded(Color.rgb(107, 114, 128), dp(12)));
        row.addView(icon, new LinearLayout.LayoutParams(dp(44), dp(44)));
        TextView text = strong(label);
        LinearLayout.LayoutParams textParams = new LinearLayout.LayoutParams(0, -2, 1);
        textParams.leftMargin = dp(18);
        row.addView(text, textParams);
        TextView right = muted(value == null || value.isEmpty() ? "未设置  ›" : value + "  ›");
        right.setTextSize(17);
        row.addView(right, new LinearLayout.LayoutParams(-2, -2));
        row.setOnClickListener(listener);
        return row;
    }

    private void editContact(boolean phone) {
        AlertDialog dialog = new AlertDialog.Builder(this).create();
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        box.setPadding(dp(20), dp(20), dp(20), dp(16));
        box.setBackground(rounded(Color.WHITE, dp(18)));

        TextView titleView = title(phone ? "手机号设置" : "邮箱设置");
        titleView.setTextSize(20);
        box.addView(titleView, new LinearLayout.LayoutParams(-1, -2));

        TextView subtitle = muted(phone ? "用于账号安全验证和订单通知，可留空。" : "用于接收账号通知和订单信息，可留空。");
        subtitle.setPadding(0, dp(6), 0, dp(14));
        box.addView(subtitle, new LinearLayout.LayoutParams(-1, -2));

        EditText edit = inputField(phone ? "请输入手机号" : "请输入邮箱", phone ? sessionStore.phone() : sessionStore.email());
        edit.setInputType(phone ? InputType.TYPE_CLASS_PHONE : InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_EMAIL_ADDRESS);
        box.addView(edit);

        TextView errorView = new TextView(this);
        errorView.setTextSize(13);
        errorView.setTextColor(Color.rgb(220, 38, 38));
        errorView.setVisibility(View.GONE);
        LinearLayout.LayoutParams errorParams = new LinearLayout.LayoutParams(-1, -2);
        errorParams.setMargins(dp(2), 0, dp(2), dp(14));
        box.addView(errorView, errorParams);

        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        Button cancel = secondaryButton("取消");
        Button save = primaryButton("保存");
        cancel.setOnClickListener(v -> dialog.dismiss());
        save.setOnClickListener(v -> {
            String value = normalizedContactValue(phone, edit.getText().toString());
            String error = contactValidationError(phone, value);
            if (!error.isEmpty()) {
                errorView.setText(error);
                errorView.setVisibility(View.VISIBLE);
                return;
            }
            save.setEnabled(false);
            save.setAlpha(0.45f);
            String newPhone = phone ? value : sessionStore.phone();
            String newEmail = phone ? sessionStore.email() : value;
            updateContact(newPhone, newEmail, dialog, save);
        });
        actions.addView(cancel, new LinearLayout.LayoutParams(dp(92), dp(44)));
        LinearLayout.LayoutParams saveParams = new LinearLayout.LayoutParams(dp(92), dp(44));
        saveParams.leftMargin = dp(10);
        actions.addView(save, saveParams);
        box.addView(actions, new LinearLayout.LayoutParams(-1, -2));

        TextWatcher watcher = new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            @Override public void onTextChanged(CharSequence s, int start, int before, int count) {
                String error = contactValidationError(phone, normalizedContactValue(phone, s.toString()));
                errorView.setText(error);
                errorView.setVisibility(error.isEmpty() ? View.GONE : View.VISIBLE);
                save.setEnabled(error.isEmpty());
                save.setAlpha(error.isEmpty() ? 1f : 0.45f);
            }
            @Override public void afterTextChanged(Editable s) {}
        };
        edit.addTextChangedListener(watcher);
        watcher.onTextChanged(edit.getText(), 0, 0, 0);

        dialog.setView(box);
        dialog.setOnShowListener(d -> {
            Window window = dialog.getWindow();
            if (window != null) {
                window.setBackgroundDrawable(new ColorDrawable(Color.TRANSPARENT));
                WindowManager.LayoutParams params = new WindowManager.LayoutParams();
                params.copyFrom(window.getAttributes());
                params.width = (int) (getResources().getDisplayMetrics().widthPixels * 0.88f);
                params.height = WindowManager.LayoutParams.WRAP_CONTENT;
                window.setAttributes(params);
            }
        });
        dialog.show();
    }

    private void updateContact(String phone, String email) {
        updateContact(phone, email, null, null);
    }

    private void updateContact(String phone, String email, AlertDialog dialog, Button saveButton) {
        new Thread(() -> {
            try {
                JSONObject profile = api.updateContact(phone, email);
                saveAccountSession(sessionStore.token(), profile, sessionStore.role(), sessionStore.nickname());
                runOnUiThread(() -> {
                    if (dialog != null) {
                        dialog.dismiss();
                    }
                    toastLine("账号信息已更新");
                    renderAccountPage();
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    if (saveButton != null) {
                        saveButton.setEnabled(true);
                        saveButton.setAlpha(1f);
                    }
                    toastLine("保存失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private String normalizedContactValue(boolean phone, String value) {
        String raw = value == null ? "" : value.trim();
        return phone ? raw.replaceAll("[\\s-]", "") : raw;
    }

    private String contactValidationError(boolean phone, String value) {
        String normalized = value == null ? "" : value.trim();
        if (normalized.isEmpty()) {
            return "";
        }
        if (phone) {
            return Pattern.matches("^1[3-9]\\d{9}$", normalized) ? "" : "请输入 11 位有效手机号";
        }
        return Patterns.EMAIL_ADDRESS.matcher(normalized).matches() ? "" : "请输入有效邮箱地址";
    }

    private void confirmLogout() {
        new AlertDialog.Builder(this)
                .setTitle("退出登录")
                .setMessage("是否退出当前账号？")
                .setNegativeButton("取消", null)
                .setPositiveButton("退出", (dialog, which) -> logoutAccount())
                .show();
    }

    private void logoutAccount() {
        new Thread(() -> {
            try {
                api.logout();
            } catch (Exception ignored) {
            }
            sessionStore.clearAuth();
            runOnUiThread(() -> {
                toastLine("已退出登录");
                createFreshLocalSession();
                renderChatHome();
                loadHomeCopy();
            });
        }).start();
    }

    private void confirmDeleteAccount() {
        new AlertDialog.Builder(this)
                .setTitle("删除账号")
                .setMessage("删除后将无法继续使用当前账号登录。是否继续？")
                .setNegativeButton("取消", null)
                .setPositiveButton("继续", (dialog, which) -> new AlertDialog.Builder(this)
                        .setTitle("确认删除")
                        .setMessage("请再次确认删除账号。")
                        .setNegativeButton("取消", null)
                        .setPositiveButton("删除", (d, w) -> deleteAccount())
                        .show())
                .show();
    }

    private void deleteAccount() {
        new Thread(() -> {
            try {
                api.deleteAccount();
                sessionStore.clearAuth();
                runOnUiThread(() -> {
                    toastLine("账号已删除");
                    createFreshLocalSession();
                    renderChatHome();
                    loadHomeCopy();
                });
            } catch (Exception error) {
                runOnUiThread(() -> toastLine("删除失败：" + error.getMessage()));
            }
        }).start();
    }

    private void renderHelpPage() {
        activePage = "help";
        baseScreen();
        content.addView(createBackTopBar("帮助", () -> renderSettings()), new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();
        page.addView(card("AI导购", "在聊天页输入需求，AI 会结合商品、购物车和订单能力给出建议。"));
        page.addView(card("商品与购物车", "商品页可以搜索、筛选、查看详情并加入购物车。"));
        page.addView(card("账号与资料", "在设置页可以编辑头像、昵称、手机号和邮箱。"));
    }

    private void renderAboutPage() {
        activePage = "about";
        baseScreen();
        content.addView(createBackTopBar("关于", () -> renderSettings()), new LinearLayout.LayoutParams(-1, dp(56)));
        LinearLayout page = pageBody();
        page.addView(card("小猪小狗导购", "Version: " + BuildConfig.VERSION_NAME + "\n由 AI 大模型提供支持"));
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

    private TextView topBarTextButton(String text, int color) {
        TextView view = new TextView(this);
        view.setText(text);
        view.setTextSize(17);
        view.setTextColor(color);
        view.setGravity(Gravity.CENTER_VERTICAL);
        view.setIncludeFontPadding(false);
        view.setPadding(0, 0, 0, 0);
        view.setClickable(true);
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
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            window.setDecorFitsSystemWindows(true);
        }
        window.setStatusBarColor(BG_COLOR);
        window.setNavigationBarColor(BG_COLOR);
        window.getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR);
    }

    @Override
    protected void onDestroy() {
        cancelRealtimeVoice();
        if (speechRecognizer != null) {
            speechRecognizer.destroy();
            speechRecognizer = null;
        }
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

    private enum SpeechMode {
        AUTO,
        XUNFEI_REALTIME,
        ANDROID_INLINE,
        ANDROID_ACTIVITY
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

    private static class CircleImageView extends ImageView {
        private final Path clipPath = new Path();

        CircleImageView(Activity activity) {
            super(activity);
        }

        @Override
        protected void onSizeChanged(int w, int h, int oldw, int oldh) {
            super.onSizeChanged(w, h, oldw, oldh);
            clipPath.reset();
            clipPath.addCircle(w / 2f, h / 2f, Math.min(w, h) / 2f, Path.Direction.CW);
        }

        @Override
        protected void onDraw(Canvas canvas) {
            int save = canvas.save();
            canvas.clipPath(clipPath);
            super.onDraw(canvas);
            canvas.restoreToCount(save);
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
