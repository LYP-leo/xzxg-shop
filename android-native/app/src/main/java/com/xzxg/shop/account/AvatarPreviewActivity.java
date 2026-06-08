package com.xzxg.shop.account;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.CircleImageView;
import com.xzxg.shop.ui.ShopUi;

import android.content.Intent;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Color;
import android.os.Bundle;
import android.provider.MediaStore;
import android.view.Gravity;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;

import java.io.InputStream;
import java.net.URL;

public class AvatarPreviewActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(Color.BLACK);
    }

    @Override
    protected void onResume() {
        super.onResume();
        if (sessionStore().token().isEmpty()) {
            startActivity(new Intent(this, LoginActivity.class));
            finish();
            return;
        }
        render();
    }

    private void render() {
        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.BLACK);
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        content.addView(AccountUi.createBackTopBar(this, "", BG_COLOR, null), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        FrameLayout avatarArea = new FrameLayout(this);
        ImageView image = new CircleImageView(this);
        image.setScaleType(ImageView.ScaleType.CENTER_CROP);
        image.setBackground(ShopUi.rounded(Color.rgb(236, 244, 255), ShopUi.dp(this, 130)));
        String url = api().absoluteUrl(sessionStore().avatarUrl());
        if (!url.isEmpty()) {
            imageLoader().load(image, url);
        }
        avatarArea.addView(image, new FrameLayout.LayoutParams(ShopUi.dp(this, 260), ShopUi.dp(this, 260), Gravity.CENTER));
        content.addView(avatarArea, new LinearLayout.LayoutParams(-1, 0, 1));

        Button edit = ShopUi.secondaryButton(this, "编辑个人资料");
        edit.setTextSize(18);
        edit.setOnClickListener(v -> startActivity(new Intent(this, EditProfileActivity.class)));
        FrameLayout.LayoutParams params = new FrameLayout.LayoutParams(-1, ShopUi.dp(this, 56), Gravity.BOTTOM);
        params.leftMargin = ShopUi.dp(this, 20);
        params.rightMargin = ShopUi.dp(this, 20);
        params.bottomMargin = ShopUi.dp(this, 36);
        root.addView(edit, params);
        image.setOnLongClickListener(v -> {
            saveAvatarToGallery(url);
            return true;
        });

        setContentView(root);
        bindRootSystemBarPadding(content);
    }

    private void saveAvatarToGallery(String url) {
        if (url == null || url.trim().isEmpty()) {
            showToastLine("当前没有可保存的头像");
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
                runOnUiThread(() -> showToastLine(saved == null ? "保存失败" : "已保存到相册"));
            } catch (Exception error) {
                runOnUiThread(() -> showToastLine("保存失败：" + error.getMessage()));
            }
        }).start();
    }
}
