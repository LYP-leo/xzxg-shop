package com.xzxg.shop.base;

import com.xzxg.shop.app.ShopApplication;
import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.LocalChatStore;
import com.xzxg.shop.storage.SessionStore;
import com.xzxg.shop.ui.ImageLoader;

import android.app.Activity;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.view.Window;
import android.view.WindowInsets;
import android.view.WindowManager;
import android.widget.Toast;

public abstract class BaseShopActivity extends Activity {
    private Toast activeBaseToast;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
    }

    protected ShopApplication shopApplication() {
        return (ShopApplication) getApplication();
    }

    protected SessionStore sessionStore() {
        return shopApplication().sessionStore();
    }

    protected LocalChatStore localChatStore() {
        return shopApplication().localChatStore();
    }

    protected ApiClient api() {
        return shopApplication().apiClient();
    }

    protected ApiClient refreshApiClient() {
        return shopApplication().refreshApiClient();
    }

    protected ImageLoader imageLoader() {
        return shopApplication().imageLoader();
    }

    protected boolean isAuthExpiredError(Throwable error) {
        return error instanceof ApiClient.ApiException && ((ApiClient.ApiException) error).statusCode == 401;
    }

    protected void showToastLine(String text) {
        if (activeBaseToast != null) {
            activeBaseToast.cancel();
        }
        activeBaseToast = Toast.makeText(this, text, Toast.LENGTH_SHORT);
        activeBaseToast.show();
    }

    protected void configureShopSystemBars(int backgroundColor) {
        Window window = getWindow();
        window.setSoftInputMode(WindowManager.LayoutParams.SOFT_INPUT_ADJUST_RESIZE);
        boolean manualInsets = Build.VERSION.SDK_INT >= 35;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            window.setDecorFitsSystemWindows(!manualInsets);
        }
        window.setStatusBarColor(backgroundColor);
        window.setNavigationBarColor(backgroundColor);
        int systemUi = View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            systemUi |= View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR;
        }
        window.getDecorView().setSystemUiVisibility(systemUi);
    }

    protected void bindRootSystemBarPadding(View root) {
        if (root == null || Build.VERSION.SDK_INT < 35 || Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
            return;
        }
        root.setOnApplyWindowInsetsListener((view, insets) -> {
            int safeTypes = WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout();
            android.graphics.Insets safeInsets = insets.getInsets(safeTypes);
            view.setPadding(0, Math.max(0, safeInsets.top), 0, Math.max(0, safeInsets.bottom));
            return insets;
        });
        root.post(root::requestApplyInsets);
    }
}
