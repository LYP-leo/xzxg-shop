package com.xzxg.shop.app;

import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.chat.ChatActivity;

import android.content.Intent;
import android.os.Bundle;

public class MainActivity extends BaseShopActivity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        Intent source = getIntent();
        Intent intent = new Intent(this, ChatActivity.class);
        if (source != null) {
            if (source.getExtras() != null) {
                intent.putExtras(source);
            }
            intent.setAction(source.getAction());
            intent.setData(source.getData());
        }
        startActivity(intent);
        finish();
        overridePendingTransition(0, 0);
    }
}
