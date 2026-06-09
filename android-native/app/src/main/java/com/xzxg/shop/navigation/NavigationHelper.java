package com.xzxg.shop.navigation;

import com.xzxg.shop.R;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.widget.Toast;

public final class NavigationHelper {
    private NavigationHelper() {
    }

    public static Intent intentForRoute(Context context, String route) {
        String normalized = route == null ? "" : route.trim();
        String className = activityClassName(context, normalized);
        if (className.isEmpty()) {
            return null;
        }
        Intent intent = new Intent();
        intent.setClassName(context, className);
        intent.putExtra(Routes.EXTRA_ROUTE, normalized);
        if ("promotions".equals(normalized) || "promotion".equals(normalized) || "activity".equals(normalized)) {
            intent.putExtra(Routes.EXTRA_PRODUCT_TAB, "activity");
        }
        return intent;
    }

    public static boolean navigateRoute(Activity activity, String route) {
        Intent intent = intentForRoute(activity, route);
        if (intent == null) {
            Toast.makeText(activity, "暂不支持该跳转", Toast.LENGTH_SHORT).show();
            return false;
        }
        activity.startActivity(intent);
        return true;
    }

    public static boolean navigateMainRoute(Activity activity, String route) {
        Intent intent = intentForRoute(activity, route);
        if (intent == null) {
            Toast.makeText(activity, "暂不支持该跳转", Toast.LENGTH_SHORT).show();
            return false;
        }
        intent.addFlags(Intent.FLAG_ACTIVITY_REORDER_TO_FRONT | Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_NO_ANIMATION);
        activity.startActivity(intent);
        activity.overridePendingTransition(0, 0);
        return true;
    }

    public static boolean navigateMainRouteFromDrawer(Activity activity, String route) {
        Intent intent = intentForRoute(activity, route);
        if (intent == null) {
            Toast.makeText(activity, "暂不支持该跳转", Toast.LENGTH_SHORT).show();
            return false;
        }
        intent.addFlags(Intent.FLAG_ACTIVITY_REORDER_TO_FRONT | Intent.FLAG_ACTIVITY_SINGLE_TOP | Intent.FLAG_ACTIVITY_NO_ANIMATION);
        activity.startActivity(intent);
        activity.overridePendingTransition(0, R.anim.drawer_activity_exit_left);
        return true;
    }

    private static String activityClassName(Context context, String route) {
        String packageName = context.getPackageName();
        if (Routes.CHAT.equals(route)) {
            return packageName + ".chat.ChatActivity";
        }
        if (Routes.ORDERS.equals(route)) {
            return packageName + ".order.OrderListActivity";
        }
        if (Routes.CART.equals(route)) {
            return packageName + ".cart.CartActivity";
        }
        if (Routes.PRODUCTS.equals(route) || "promotions".equals(route) || "promotion".equals(route) || "activity".equals(route)) {
            return packageName + ".product.ProductListActivity";
        }
        if (Routes.COUPONS.equals(route) || "coupon".equals(route)) {
            return packageName + ".coupon.CouponActivity";
        }
        if (Routes.LOGIN.equals(route)) {
            return packageName + ".account.LoginActivity";
        }
        if (Routes.SETTINGS.equals(route)) {
            return packageName + ".settings.SettingsActivity";
        }
        if (Routes.ACCOUNT.equals(route)) {
            return packageName + ".account.AccountActivity";
        }
        if (Routes.EDIT_PROFILE.equals(route) || "profile".equals(route)) {
            return packageName + ".account.EditProfileActivity";
        }
        return "";
    }
}
