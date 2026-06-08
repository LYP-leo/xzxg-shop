package com.xzxg.shop.account;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.CircleImageView;
import com.xzxg.shop.ui.ShopUi;

import android.content.Intent;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Color;
import android.graphics.Typeface;
import android.net.Uri;
import android.os.Bundle;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.Gravity;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;

public class EditProfileActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private static final int REQUEST_PICK_AVATAR = 5101;

    private byte[] pendingAvatarBytes;
    private String pendingAvatarMime = "image/jpeg";
    private Bitmap pendingAvatarPreview;
    private Runnable pendingAvatarChanged;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        if (sessionStore().token().isEmpty()) {
            startActivity(new Intent(this, LoginActivity.class));
            finish();
            return;
        }
        configureShopSystemBars(BG_COLOR);
        render();
    }

    private void render() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);

        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER_VERTICAL);
        bar.setPadding(ShopUi.dp(this, 18), 0, ShopUi.dp(this, 18), 0);
        TextView cancel = ShopUi.topBarTextButton(this, "取消", Color.BLACK);
        cancel.setOnClickListener(v -> finish());
        bar.addView(cancel, new LinearLayout.LayoutParams(0, -1, 1));
        TextView heading = ShopUi.title(this, "个人资料");
        heading.setGravity(Gravity.CENTER);
        heading.setIncludeFontPadding(false);
        bar.addView(heading, new LinearLayout.LayoutParams(0, -1, 1));
        TextView done = ShopUi.topBarTextButton(this, "保存", Color.rgb(180, 198, 230));
        done.setGravity(Gravity.RIGHT | Gravity.CENTER_VERTICAL);
        done.setEnabled(false);
        bar.addView(done, new LinearLayout.LayoutParams(0, -1, 1));
        content.addView(bar, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setGravity(Gravity.CENTER_HORIZONTAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));

        FrameLayout avatarWrap = new FrameLayout(this);
        TextView avatarInitial = new TextView(this);
        avatarInitial.setText(AccountUi.initialOf(sessionStore().nickname()));
        avatarInitial.setTextSize(38);
        avatarInitial.setTypeface(Typeface.DEFAULT_BOLD);
        avatarInitial.setGravity(Gravity.CENTER);
        avatarInitial.setTextColor(Color.rgb(30, 64, 175));
        avatarInitial.setBackground(ShopUi.rounded(Color.rgb(219, 234, 254), ShopUi.dp(this, 64)));
        avatarWrap.addView(avatarInitial, new FrameLayout.LayoutParams(ShopUi.dp(this, 128), ShopUi.dp(this, 128), Gravity.CENTER));
        ImageView avatarImage = new CircleImageView(this);
        avatarImage.setScaleType(ImageView.ScaleType.CENTER_CROP);
        avatarImage.setBackground(ShopUi.rounded(Color.rgb(236, 244, 255), ShopUi.dp(this, 64)));
        avatarImage.setVisibility(android.view.View.GONE);
        String avatarUrl = api().absoluteUrl(sessionStore().avatarUrl());
        if (!avatarUrl.isEmpty()) {
            avatarImage.setVisibility(android.view.View.VISIBLE);
            avatarInitial.setVisibility(android.view.View.GONE);
            imageLoader().load(avatarImage, avatarUrl);
        }
        avatarWrap.addView(avatarImage, new FrameLayout.LayoutParams(ShopUi.dp(this, 128), ShopUi.dp(this, 128), Gravity.CENTER));
        TextView plus = new TextView(this);
        plus.setText("+");
        plus.setTextSize(30);
        plus.setTextColor(Color.WHITE);
        plus.setGravity(Gravity.CENTER);
        plus.setBackground(ShopUi.rounded(Color.rgb(37, 99, 235), ShopUi.dp(this, 22)));
        avatarWrap.addView(plus, new FrameLayout.LayoutParams(ShopUi.dp(this, 44), ShopUi.dp(this, 44), Gravity.RIGHT | Gravity.BOTTOM));
        avatarWrap.setOnClickListener(v -> pickAvatar());
        page.addView(avatarWrap, new LinearLayout.LayoutParams(ShopUi.dp(this, 150), ShopUi.dp(this, 150)));

        TextView label = ShopUi.strong(this, "昵称");
        label.setGravity(Gravity.LEFT);
        page.addView(label, new LinearLayout.LayoutParams(-1, -2));
        EditText nickname = ShopUi.inputField(this, "昵称", sessionStore().nickname());
        page.addView(nickname);

        final boolean[] changed = {false};
        pendingAvatarBytes = null;
        pendingAvatarPreview = null;
        pendingAvatarChanged = () -> {
            changed[0] = true;
            done.setEnabled(true);
            done.setTextColor(Color.rgb(37, 99, 235));
            if (pendingAvatarPreview != null) {
                avatarImage.setVisibility(android.view.View.VISIBLE);
                avatarInitial.setVisibility(android.view.View.GONE);
                avatarImage.setImageBitmap(pendingAvatarPreview);
            }
            showToastLine("头像已裁剪为正方形");
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
            if (changed[0]) {
                saveProfileChanges(nickname.getText().toString());
            }
        });

        setContentView(content);
        bindRootSystemBarPadding(content);
    }

    private void pickAvatar() {
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE);
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION);
        intent.setType("image/*");
        startActivityForResult(intent, REQUEST_PICK_AVATAR);
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode != REQUEST_PICK_AVATAR || resultCode != RESULT_OK || data == null || data.getData() == null) {
            return;
        }
        Uri uri = data.getData();
        try {
            getContentResolver().takePersistableUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION);
        } catch (Exception ignored) {
        }
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
            showToastLine("头像处理失败：" + error.getMessage());
        }
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

    private void saveProfileChanges(String nickname) {
        String name = nickname == null ? "" : nickname.trim();
        if (name.isEmpty()) {
            showToastLine("昵称不能为空");
            return;
        }
        new Thread(() -> {
            try {
                api().profile();
                String avatarUrl = "";
                if (pendingAvatarBytes != null) {
                    JSONObject upload = api().uploadAvatar("avatar.jpg", pendingAvatarMime, pendingAvatarBytes);
                    avatarUrl = upload.optString("url", "");
                }
                JSONObject profile = api().updateProfile(name, avatarUrl);
                AccountSessionHelper.saveAccountSession(sessionStore(), sessionStore().token(), profile, sessionStore().role(), name);
                pendingAvatarBytes = null;
                pendingAvatarChanged = null;
                runOnUiThread(() -> {
                    showToastLine("资料已保存");
                    finish();
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    String message = error.getMessage();
                    if (message != null && message.contains("404")) {
                        showToastLine("保存失败：当前后端不是最新版，请切换测试后端");
                    } else {
                        showToastLine("保存失败：" + message);
                    }
                });
            }
        }).start();
    }

    @Override
    protected void onDestroy() {
        if (pendingAvatarPreview != null) {
            pendingAvatarPreview.recycle();
            pendingAvatarPreview = null;
        }
        super.onDestroy();
    }
}
