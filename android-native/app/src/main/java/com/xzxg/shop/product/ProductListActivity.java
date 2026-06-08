package com.xzxg.shop.product;

import com.xzxg.shop.account.LoginActivity;
import com.xzxg.shop.base.BaseShopActivity;
import com.xzxg.shop.cart.CartActivity;
import com.xzxg.shop.navigation.Routes;
import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.ui.BottomSheetHelper;
import com.xzxg.shop.ui.ShopUi;
import com.xzxg.shop.ui.TopBarHelper;

import android.app.AlertDialog;
import android.content.Intent;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.os.Bundle;
import android.text.Editable;
import android.text.TextWatcher;
import android.view.Gravity;
import android.view.View;
import android.view.inputmethod.EditorInfo;
import android.view.inputmethod.InputMethodManager;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ImageView;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import org.json.JSONArray;
import org.json.JSONObject;

import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

public class ProductListActivity extends BaseShopActivity {
    private static final int BG_COLOR = 0xFFF8F9FB;
    private static final int PRODUCT_PAGE_SIZE = 20;

    private FrameLayout root;
    private LinearLayout content;
    private ScrollView productsScroll;
    private LinearLayout productsList;
    private FrameLayout productCartFab;
    private TextView productCartBadge;
    private String activeProductTab = "list";
    private String lastProductKeyword = "";
    private String lastCategoryId = "";
    private String lastCategoryName = "全部";
    private JSONArray cachedCategoryTree = new JSONArray();
    private ProductListState currentProductState;
    private boolean loadingProducts;
    private int cartItemCountCache;
    private final Set<String> loadingProductKeys = new HashSet<>();
    private final Map<String, ProductListState> productListCache = new HashMap<>();

    private static class ProductListState {
        JSONArray items = new JSONArray();
        int nextPage = 1;
        boolean hasMore = true;
        boolean loaded;
        int scrollY;
    }

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        configureShopSystemBars(BG_COLOR);
        String tab = getIntent() == null ? "" : getIntent().getStringExtra(Routes.EXTRA_PRODUCT_TAB);
        activeProductTab = tab == null || tab.isEmpty() ? "list" : tab;
        renderProductsTab(activeProductTab, false);
    }

    private void renderProductsTab(String tab, boolean restoreScroll) {
        activeProductTab = tab == null || tab.isEmpty() ? "list" : tab;
        root = new FrameLayout(this);
        root.setBackgroundColor(BG_COLOR);
        content = new LinearLayout(this);
        content.setOrientation(LinearLayout.VERTICAL);
        content.setBackgroundColor(BG_COLOR);
        root.addView(content, new FrameLayout.LayoutParams(-1, -1));
        content.addView(createTopBar("商品"), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 56)));
        if ("activity".equals(activeProductTab)) {
            renderProductPromotionsTab();
        } else {
            renderProductListTab(restoreScroll);
        }
        content.addView(productBottomBar(), new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 58)));
        setContentView(root);
        bindRootSystemBarPadding(content);
        addProductCartFab();
    }

    private void renderProductListTab(boolean restoreScroll) {
        productsScroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        productsScroll.addView(page);
        content.addView(productsScroll, new LinearLayout.LayoutParams(-1, 0, 1));

        LinearLayout searchRow = new LinearLayout(this);
        searchRow.setGravity(Gravity.CENTER_VERTICAL);
        searchRow.setPadding(ShopUi.dp(this, 12), 0, ShopUi.dp(this, 8), 0);
        searchRow.setBackground(ShopUi.rounded(Color.rgb(243, 244, 246), ShopUi.dp(this, 14)));
        TextView searchIcon = new TextView(this);
        searchIcon.setText("⌕");
        searchIcon.setTextSize(22);
        searchIcon.setTextColor(Color.rgb(107, 114, 128));
        searchIcon.setGravity(Gravity.CENTER);
        searchIcon.setIncludeFontPadding(false);
        searchRow.addView(searchIcon, new LinearLayout.LayoutParams(ShopUi.dp(this, 38), ShopUi.dp(this, 48)));
        EditText keyword = new EditText(this);
        keyword.setText(lastProductKeyword);
        keyword.setHint("搜索商品、品牌、功效");
        keyword.setTextSize(15);
        keyword.setSingleLine(true);
        keyword.setImeOptions(EditorInfo.IME_ACTION_SEARCH);
        keyword.setBackground(new ColorDrawable(Color.TRANSPARENT));
        keyword.setPadding(ShopUi.dp(this, 8), 0, ShopUi.dp(this, 8), 0);
        searchRow.addView(keyword, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1));
        Button clear = ShopUi.transparentIconButton(this, "×");
        clear.setTextSize(22);
        clear.setVisibility(lastProductKeyword.isEmpty() ? View.GONE : View.VISIBLE);
        searchRow.addView(clear, new LinearLayout.LayoutParams(ShopUi.dp(this, 36), ShopUi.dp(this, 48)));
        page.addView(searchRow, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 48)));

        LinearLayout categoryButton = textFilterRow("商品类别", lastCategoryName + "  >");
        categoryButton.setOnClickListener(v -> openCategoryDialog(() -> {
            lastProductKeyword = keyword.getText().toString();
            resetProductState(lastProductKeyword, lastCategoryId);
            renderProductsTab("list", false);
        }));
        LinearLayout.LayoutParams filterParams = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 46));
        filterParams.setMargins(0, ShopUi.dp(this, 10), 0, ShopUi.dp(this, 10));
        page.addView(categoryButton, filterParams);

        productsList = new LinearLayout(this);
        productsList.setOrientation(LinearLayout.VERTICAL);
        LinearLayout.LayoutParams listParams = new LinearLayout.LayoutParams(-1, -2);
        listParams.topMargin = ShopUi.dp(this, 2);
        page.addView(productsList, listParams);

        Runnable executeSearch = () -> {
            lastProductKeyword = keyword.getText().toString();
            resetProductState(lastProductKeyword, lastCategoryId);
            loadProducts(productsList, lastProductKeyword, lastCategoryId);
            hideKeyboardFrom(keyword);
            clear.setVisibility(lastProductKeyword.isEmpty() ? View.GONE : View.VISIBLE);
        };
        keyword.setOnEditorActionListener((v, actionId, event) -> {
            if (actionId == EditorInfo.IME_ACTION_SEARCH) {
                executeSearch.run();
                return true;
            }
            return false;
        });
        keyword.addTextChangedListener(new TextWatcher() {
            @Override public void beforeTextChanged(CharSequence s, int start, int count, int after) {}
            @Override public void onTextChanged(CharSequence s, int start, int before, int count) {
                clear.setVisibility(s == null || s.length() == 0 ? View.GONE : View.VISIBLE);
            }
            @Override public void afterTextChanged(Editable s) {}
        });
        clear.setOnClickListener(v -> {
            keyword.setText("");
            executeSearch.run();
        });
        productsScroll.setOnScrollChangeListener((v, scrollX, scrollY, oldScrollX, oldScrollY) -> {
            if (currentProductState != null) {
                currentProductState.scrollY = scrollY;
            }
            if (productsList == null || currentProductState == null) {
                return;
            }
            int bottomDistance = productsList.getBottom() - (productsScroll.getHeight() + scrollY);
            if (bottomDistance > ShopUi.dp(this, 8) || !currentProductState.hasMore || loadingProducts) {
                return;
            }
            if (scrollY >= oldScrollY) {
                loadMoreProducts(productsList, lastProductKeyword, lastCategoryId);
            }
        });
        ensureCategoriesLoaded();
        loadProducts(productsList, lastProductKeyword, lastCategoryId);
        if (restoreScroll && currentProductState != null) {
            productsScroll.post(() -> productsScroll.scrollTo(0, currentProductState.scrollY));
        }
    }

    private void renderProductPromotionsTab() {
        ScrollView scroll = new ScrollView(this);
        LinearLayout page = new LinearLayout(this);
        page.setOrientation(LinearLayout.VERTICAL);
        page.setPadding(ShopUi.dp(this, 16), ShopUi.dp(this, 10), ShopUi.dp(this, 16), ShopUi.dp(this, 16));
        scroll.addView(page);
        content.addView(scroll, new LinearLayout.LayoutParams(-1, 0, 1));
        page.addView(ShopUi.muted(this, "正在加载活动..."));
        new Thread(() -> {
            try {
                JSONArray items = api().promotions();
                runOnUiThread(() -> renderPromotionItems(page, items));
            } catch (Exception error) {
                runOnUiThread(() -> renderError(page, "活动加载失败", error.getMessage(), () -> renderProductsTab("activity", false)));
            }
        }).start();
    }

    private View productBottomBar() {
        LinearLayout bar = new LinearLayout(this);
        bar.setGravity(Gravity.CENTER);
        bar.setPadding(ShopUi.dp(this, 12), ShopUi.dp(this, 6), ShopUi.dp(this, 12), ShopUi.dp(this, 6));
        bar.setBackgroundColor(Color.WHITE);
        bar.addView(productTabButton("商品列表", "list"), new LinearLayout.LayoutParams(0, ShopUi.dp(this, 46), 1));
        bar.addView(productTabButton("活动", "activity"), new LinearLayout.LayoutParams(0, ShopUi.dp(this, 46), 1));
        return bar;
    }

    private TextView productTabButton(String text, String tab) {
        boolean selected = tab.equals(activeProductTab);
        TextView view = new TextView(this);
        view.setText(text);
        view.setTextSize(15);
        view.setTypeface(Typeface.DEFAULT, selected ? Typeface.BOLD : Typeface.NORMAL);
        view.setTextColor(selected ? Color.BLACK : Color.rgb(107, 114, 128));
        view.setGravity(Gravity.CENTER);
        view.setBackground(ShopUi.rounded(selected ? Color.rgb(245, 246, 248) : Color.WHITE, ShopUi.dp(this, 14)));
        view.setOnClickListener(v -> renderProductsTab(tab, true));
        return view;
    }

    private void addProductCartFab() {
        if (sessionStore().token().isEmpty() || root == null) {
            return;
        }
        FrameLayout fabWrap = new FrameLayout(this);
        fabWrap.setClipChildren(false);
        fabWrap.setClipToPadding(false);
        productCartFab = new FrameLayout(this);
        productCartFab.setClickable(true);
        android.graphics.drawable.GradientDrawable fabBackground = ShopUi.rounded(Color.WHITE, ShopUi.dp(this, 28));
        fabBackground.setStroke(ShopUi.dp(this, 1), Color.rgb(220, 224, 230));
        productCartFab.setBackground(fabBackground);
        productCartFab.setElevation(0);
        productCartFab.setTranslationZ(0);
        TextView icon = new TextView(this);
        icon.setText("购物车");
        icon.setTextSize(13);
        icon.setTypeface(Typeface.DEFAULT_BOLD);
        icon.setTextColor(Color.rgb(17, 24, 39));
        icon.setGravity(Gravity.CENTER);
        productCartFab.addView(icon, new FrameLayout.LayoutParams(ShopUi.dp(this, 56), ShopUi.dp(this, 56), Gravity.CENTER));
        productCartFab.setOnClickListener(v -> openLegacyCartPage());
        fabWrap.addView(productCartFab, new FrameLayout.LayoutParams(ShopUi.dp(this, 56), ShopUi.dp(this, 56), Gravity.CENTER));
        productCartBadge = new TextView(this);
        productCartBadge.setTextSize(10);
        productCartBadge.setTextColor(Color.WHITE);
        productCartBadge.setGravity(Gravity.CENTER);
        productCartBadge.setTypeface(Typeface.DEFAULT_BOLD);
        productCartBadge.setIncludeFontPadding(false);
        productCartBadge.setBackground(ShopUi.rounded(Color.rgb(220, 38, 38), ShopUi.dp(this, 10)));
        productCartBadge.setElevation(0);
        productCartBadge.setTranslationZ(0);
        FrameLayout.LayoutParams badgeParams = new FrameLayout.LayoutParams(ShopUi.dp(this, 30), ShopUi.dp(this, 20), Gravity.RIGHT | Gravity.TOP);
        badgeParams.topMargin = 0;
        badgeParams.rightMargin = 0;
        fabWrap.addView(productCartBadge, badgeParams);
        productCartBadge.bringToFront();
        FrameLayout.LayoutParams fabParams = new FrameLayout.LayoutParams(ShopUi.dp(this, 72), ShopUi.dp(this, 72), Gravity.RIGHT | Gravity.BOTTOM);
        fabParams.rightMargin = ShopUi.dp(this, 18);
        fabParams.bottomMargin = ShopUi.dp(this, 68);
        root.addView(fabWrap, fabParams);
        updateProductCartBadge(cartItemCountCache);
        refreshProductCartBadge();
    }

    private void refreshProductCartBadge() {
        if (sessionStore().token().isEmpty() || productCartBadge == null) {
            return;
        }
        new Thread(() -> {
            try {
                JSONObject cart = api().cart();
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
        productCartBadge.setText(count > 99 ? "99+" : String.valueOf(count));
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

    private void ensureCategoriesLoaded() {
        if (cachedCategoryTree.length() > 0) {
            return;
        }
        new Thread(() -> {
            try {
                JSONArray categories = api().categoriesTree();
                runOnUiThread(() -> cachedCategoryTree = categories);
            } catch (Exception ignored) {
            }
        }).start();
    }

    private void openCategoryDialog(Runnable onChanged) {
        if (cachedCategoryTree.length() == 0) {
            new Thread(() -> {
                try {
                    JSONArray categories = api().categoriesTree();
                    runOnUiThread(() -> {
                        cachedCategoryTree = categories;
                        openCategoryDialog(onChanged);
                    });
                } catch (Exception error) {
                    runOnUiThread(() -> showToastLine("分类加载失败：" + error.getMessage()));
                }
            }).start();
            return;
        }
        final String[] pendingId = {lastCategoryId};
        final String[] pendingName = {lastCategoryName};
        LinearLayout sheet = BottomSheetHelper.box(this);
        LinearLayout header = new LinearLayout(this);
        header.setGravity(Gravity.CENTER_VERTICAL);
        header.addView(ShopUi.title(this, "选择商品类别"), new LinearLayout.LayoutParams(0, -2, 1));
        Button reset = ShopUi.textOnlyButton(this, "重置");
        reset.setTextColor(Color.rgb(37, 99, 235));
        header.addView(reset, new LinearLayout.LayoutParams(ShopUi.dp(this, 72), ShopUi.dp(this, 42)));
        sheet.addView(header, new LinearLayout.LayoutParams(-1, -2));

        LinearLayout rootLayout = new LinearLayout(this);
        rootLayout.setOrientation(LinearLayout.HORIZONTAL);
        rootLayout.setPadding(0, ShopUi.dp(this, 8), 0, ShopUi.dp(this, 8));
        LinearLayout primary = new LinearLayout(this);
        primary.setOrientation(LinearLayout.VERTICAL);
        LinearLayout secondary = new LinearLayout(this);
        secondary.setOrientation(LinearLayout.VERTICAL);
        ScrollView primaryScroll = new ScrollView(this);
        primaryScroll.addView(primary);
        ScrollView secondaryScroll = new ScrollView(this);
        secondaryScroll.addView(secondary);
        rootLayout.addView(primaryScroll, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 420), 1));
        rootLayout.addView(secondaryScroll, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 420), 1));
        sheet.addView(rootLayout, new LinearLayout.LayoutParams(-1, -2));

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

        final AlertDialog[] dialogRef = new AlertDialog[1];
        reset.setOnClickListener(v -> {
            lastCategoryId = "";
            lastCategoryName = "全部";
            resetProductState(lastProductKeyword, lastCategoryId);
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
            onChanged.run();
        });
        LinearLayout actions = new LinearLayout(this);
        Button cancel = ShopUi.secondaryButton(this, "取消");
        Button confirm = ShopUi.primaryButton(this, "确定");
        cancel.setOnClickListener(v -> {
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
        });
        confirm.setOnClickListener(v -> {
            lastCategoryId = pendingId[0];
            lastCategoryName = pendingName[0];
            resetProductState(lastProductKeyword, lastCategoryId);
            if (dialogRef[0] != null) {
                dialogRef[0].dismiss();
            }
            onChanged.run();
        });
        actions.addView(cancel, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1));
        LinearLayout.LayoutParams confirmParams = new LinearLayout.LayoutParams(0, ShopUi.dp(this, 48), 1);
        confirmParams.leftMargin = ShopUi.dp(this, 10);
        actions.addView(confirm, confirmParams);
        sheet.addView(actions, new LinearLayout.LayoutParams(-1, -2));
        dialogRef[0] = BottomSheetHelper.show(this, sheet, true);
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
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 44));
        params.setMargins(0, 0, ShopUi.dp(this, 8), 0);
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
        secondary.addView(button, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 44)));
    }

    private TextView categoryRow(String label, boolean selected) {
        TextView view = new TextView(this);
        view.setText(label);
        view.setTextSize(15);
        view.setTextColor(Color.rgb(17, 24, 39));
        view.setPadding(ShopUi.dp(this, 14), 0, ShopUi.dp(this, 10), 0);
        view.setBackgroundColor(selected ? Color.rgb(238, 242, 247) : Color.TRANSPARENT);
        return view;
    }

    private void loadProducts(LinearLayout list, String keyword, String categoryId) {
        String key = productCacheKey(keyword, categoryId);
        currentProductState = productListCache.get(key);
        if (currentProductState != null && currentProductState.loaded) {
            renderProductItems(list, currentProductState);
            return;
        }
        currentProductState = new ProductListState();
        productListCache.put(key, currentProductState);
        list.removeAllViews();
        list.addView(ShopUi.muted(this, "正在加载商品..."));
        loadMoreProducts(list, keyword, categoryId);
    }

    private void loadMoreProducts(LinearLayout list, String keyword, String categoryId) {
        String requestKey = productCacheKey(keyword, categoryId);
        if (loadingProductKeys.contains(requestKey)) {
            return;
        }
        if (currentProductState == null) {
            currentProductState = new ProductListState();
            productListCache.put(requestKey, currentProductState);
        }
        ProductListState requestState = currentProductState;
        int requestPage = requestState.nextPage;
        loadingProductKeys.add(requestKey);
        loadingProducts = !loadingProductKeys.isEmpty();
        if (requestState.loaded) {
            renderProductItems(list, requestState);
        }
        new Thread(() -> {
            try {
                ApiClient.ProductPage page = api().productsPage(keyword, categoryId, requestPage, PRODUCT_PAGE_SIZE);
                runOnUiThread(() -> {
                    loadingProductKeys.remove(requestKey);
                    loadingProducts = !loadingProductKeys.isEmpty();
                    if (currentProductState != requestState) {
                        return;
                    }
                    appendProducts(requestState.items, page.items);
                    requestState.nextPage = page.nextPage;
                    requestState.hasMore = page.hasMore;
                    requestState.loaded = true;
                    renderProductItems(list, requestState);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    loadingProductKeys.remove(requestKey);
                    loadingProducts = !loadingProductKeys.isEmpty();
                    if (currentProductState != requestState) {
                        return;
                    }
                    if (requestState.loaded) {
                        showToastLine("加载更多失败：" + error.getMessage());
                        renderProductItems(list, requestState);
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
            list.addView(ShopUi.card(this, "暂无商品", "换个关键词试试。"));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item != null) {
                list.addView(productCard(item));
            }
        }
        TextView footer = ShopUi.muted(this, state.hasMore ? (loadingProducts ? "正在加载更多..." : "向下滑动加载更多") : "已经到底了");
        footer.setGravity(Gravity.CENTER);
        list.addView(footer, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 44)));
    }

    private View productCard(JSONObject item) {
        LinearLayout card = ShopUi.panel(this);
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(productImage(item.optString("imageUrl", ""), 88), new LinearLayout.LayoutParams(ShopUi.dp(this, 88), ShopUi.dp(this, 88)));
        LinearLayout info = new LinearLayout(this);
        info.setOrientation(LinearLayout.VERTICAL);
        info.addView(ShopUi.strong(this, item.optString("name", "未命名商品")));
        info.addView(ShopUi.muted(this, item.optString("brand", "") + " · " + item.optString("merchantName", "商家")));
        TextView price = new TextView(this);
        price.setText("¥" + item.optString("price", "0"));
        price.setTextSize(16);
        price.setTypeface(Typeface.DEFAULT_BOLD);
        price.setTextColor(Color.rgb(17, 24, 39));
        info.addView(price);
        LinearLayout.LayoutParams infoParams = new LinearLayout.LayoutParams(0, -2, 1);
        infoParams.leftMargin = ShopUi.dp(this, 12);
        row.addView(info, infoParams);
        card.addView(row);
        JSONArray points = item.optJSONArray("sellingPoints");
        if (points != null && points.length() > 0) {
            card.addView(ShopUi.muted(this, "卖点：" + joinArray(points, 3)));
        }
        String reason = item.optString("recommendReason", "");
        if (!reason.isEmpty()) {
            card.addView(ShopUi.muted(this, "推荐理由：" + reason));
        }
        LinearLayout actions = new LinearLayout(this);
        actions.setGravity(Gravity.CENTER_VERTICAL);
        Button detail = ShopUi.secondaryButton(this, "详情");
        detail.setOnClickListener(v -> openProductDetailActivity(item.optString("productId")));
        LinearLayout.LayoutParams detailParams = new LinearLayout.LayoutParams(0, ShopUi.dp(this, 44), 1);
        detailParams.rightMargin = ShopUi.dp(this, 8);
        actions.addView(detail, detailParams);
        Button add = ShopUi.primaryButton(this, "加入购物车");
        add.setOnClickListener(v -> addProductToCart(item, add));
        actions.addView(add, new LinearLayout.LayoutParams(0, ShopUi.dp(this, 44), 1));
        LinearLayout.LayoutParams actionParams = new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 44));
        actionParams.topMargin = ShopUi.dp(this, 8);
        card.addView(actions, actionParams);
        return card;
    }

    private View productImage(String imageUrl, int sizeDp) {
        FrameLayout frame = new FrameLayout(this);
        frame.setBackground(ShopUi.rounded(Color.rgb(243, 244, 246), ShopUi.dp(this, 12)));
        TextView placeholder = new TextView(this);
        placeholder.setText("图");
        placeholder.setTextSize(14);
        placeholder.setGravity(Gravity.CENTER);
        placeholder.setTextColor(Color.rgb(107, 114, 128));
        frame.addView(placeholder, new FrameLayout.LayoutParams(-1, -1));
        String url = api().absoluteUrl(imageUrl);
        if (!url.isEmpty()) {
            ImageView image = new ImageView(this);
            image.setScaleType(ImageView.ScaleType.CENTER_CROP);
            frame.addView(image, new FrameLayout.LayoutParams(-1, -1));
            imageLoader().load(image, url);
        }
        return frame;
    }

    private void addProductToCart(JSONObject item, Button sourceButton) {
        if (sessionStore().token().isEmpty()) {
            showToastLine("请先登录后再加入购物车");
            startActivity(new Intent(this, LoginActivity.class));
            return;
        }
        sourceButton.setEnabled(false);
        sourceButton.setAlpha(0.72f);
        sourceButton.setText("加入中...");
        new Thread(() -> {
            try {
                String skuId = item.optString("skuId", "");
                if (skuId.isEmpty()) {
                    JSONArray skus = api().productSkus(item.optString("productId"));
                    for (int i = 0; i < skus.length(); i++) {
                        JSONObject sku = skus.optJSONObject(i);
                        if (sku != null && sku.optInt("stockQuantity", 0) > 0) {
                            skuId = sku.optString("skuId", "");
                            break;
                        }
                    }
                }
                api().addCartItem(item.optString("productId"), skuId, 1);
                runOnUiThread(() -> {
                    showToastLine("已加入购物车");
                    refreshProductCartBadge();
                    sourceButton.setEnabled(true);
                    sourceButton.setAlpha(1f);
                    sourceButton.setText("已加入");
                    sourceButton.postDelayed(() -> sourceButton.setText("加入购物车"), 600);
                });
            } catch (Exception error) {
                runOnUiThread(() -> {
                    sourceButton.setEnabled(true);
                    sourceButton.setAlpha(1f);
                    sourceButton.setText("加入购物车");
                    showToastLine("加入失败：" + error.getMessage());
                });
            }
        }).start();
    }

    private void renderPromotionItems(LinearLayout page, JSONArray items) {
        page.removeAllViews();
        if (items == null || items.length() == 0) {
            page.addView(ShopUi.card(this, "暂无活动", "稍后再来看看。"));
            return;
        }
        for (int i = 0; i < items.length(); i++) {
            JSONObject item = items.optJSONObject(i);
            if (item == null) {
                continue;
            }
            LinearLayout card = ShopUi.panel(this);
            card.addView(ShopUi.strong(this, item.optString("name", "促销活动")));
            card.addView(ShopUi.muted(this, item.optString("scope", "platform") + " · 满 " + item.optString("threshold_amount", "0") + " 减 " + item.optString("discount_amount", "0")));
            card.addView(ShopUi.muted(this, "有效期：" + item.optString("start_at", "") + " - " + item.optString("end_at", "")));
            page.addView(card);
        }
    }

    private void renderError(LinearLayout parent, String title, String detail, Runnable retry) {
        parent.removeAllViews();
        parent.addView(ShopUi.card(this, title, detail == null ? "" : detail));
        Button button = ShopUi.secondaryButton(this, "重试");
        button.setOnClickListener(v -> retry.run());
        parent.addView(button, new LinearLayout.LayoutParams(-1, ShopUi.dp(this, 52)));
    }

    private LinearLayout textFilterRow(String label, String value) {
        LinearLayout row = new LinearLayout(this);
        row.setGravity(Gravity.CENTER_VERTICAL);
        TextView left = ShopUi.muted(this, label);
        left.setGravity(Gravity.CENTER_VERTICAL);
        row.addView(left, new LinearLayout.LayoutParams(-2, -1));
        TextView right = ShopUi.strong(this, value);
        right.setGravity(Gravity.CENTER_VERTICAL);
        LinearLayout.LayoutParams rightParams = new LinearLayout.LayoutParams(-2, -1);
        rightParams.leftMargin = ShopUi.dp(this, 8);
        row.addView(right, rightParams);
        return row;
    }

    private View createTopBar(String titleText) {
        return TopBarHelper.backBar(this, BG_COLOR, titleText, null);
    }

    private void openProductDetailActivity(String productId) {
        String id = productId == null ? "" : productId.trim();
        if (id.isEmpty()) {
            showToastLine("商品信息缺失");
            return;
        }
        Intent intent = new Intent(this, ProductDetailActivity.class);
        intent.putExtra(Routes.EXTRA_PRODUCT_ID, id);
        startActivity(intent);
    }

    private void openLegacyCartPage() {
        startActivity(new Intent(this, CartActivity.class));
    }

    private void hideKeyboardFrom(View view) {
        InputMethodManager manager = (InputMethodManager) getSystemService(INPUT_METHOD_SERVICE);
        if (manager != null) {
            manager.hideSoftInputFromWindow(view.getWindowToken(), 0);
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
}
