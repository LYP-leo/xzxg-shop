package com.xzxg.shop.settings;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.graphics.Color;
import android.graphics.Typeface;
import android.os.Bundle;
import android.view.Gravity;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

public class HelpActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        render();
    }

    private void render() {
        LinearLayout content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        content.addView(createBackTopBar("帮助"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));

        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 16), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page, new ScrollView.LayoutParams(-1, -2));

        page.addView(ShopUi.card(this, "AI导购", "在聊天页输入需求，AI 会结合商品、购物车和订单能力给出建议。"));
        page.addView(ShopUi.card(this, "商品与购物车", "商品页可以搜索、筛选、查看详情并加入购物车。"));
        page.addView(ShopUi.card(this, "账号与资料", "在设置页可以编辑头像、昵称、手机号和邮箱。"));

        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        setContentView(content);
        bindRootSystemBarPadding(content);
    }

    private View createBackTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }

}
