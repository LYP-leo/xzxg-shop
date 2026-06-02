# 安卓 APP 技术方案 v14

## 1. 背景

本文针对 [安卓APP_问题_v13.md](./安卓APP_问题_v13.md) 设计下一版 Android 原生 APP 修复方案。

v13 已完成顶部入口删除、侧栏搜索、设置二级页返回、头像保存和商品分类默认态等改造。v14 不新增业务能力，聚焦 v13 实现后的交互回归：

- 搜索框键盘收起后，焦点仍停留在“历史会话搜索”输入框。
- 本地搜索结果会先出现，随后被远程搜索空结果覆盖，表现为“出现一瞬间就消失”。
- 侧栏底部用户/设置栏背景需要恢复白色，并用横线和历史区域分隔。
- 个人资料页顶部“取消”和“保存”按钮垂直位置不一致。

## 2. 总体结论

v14 采用最小修复：

```text
侧栏搜索:
  搜索聚焦 -> 隐藏四个导航按钮
  键盘收起 -> 必须 clearFocus，恢复导航按钮
  本地搜索结果永远作为基础结果
  远程搜索结果只做补充合并，不允许用空远程结果覆盖本地结果

侧栏底部:
  背景恢复白色
  历史区和底部用户/设置栏之间增加 1px 分割线

个人资料页:
  顶部栏改为统一的 56dp 高度
  “取消”和“保存”使用同样 TextView 样式和 Gravity.CENTER_VERTICAL
```

## 3. 侧栏键盘与焦点

### 3.1 当前问题根因

当前 `showDrawer()` 中搜索状态主要由 `search.hasFocus()` 控制：

```java
boolean expanded = search.hasFocus();
navGroup.setVisibility(expanded ? View.GONE : View.VISIBLE);
```

但用户通过系统返回键或键盘完成键收起 IME 时，输入框仍可能保持 focus。结果是：

- 键盘已经收起；
- 搜索框仍显示光标/聚焦态；
- 四个导航按钮可能仍保持隐藏或状态不稳定。

### 3.2 修复方案

引入统一方法：

```java
private void collapseDrawerSearch(EditText search, Runnable refreshSearchChrome) {
    hideKeyboardFrom(search);
    search.clearFocus();
    refreshSearchChrome.run();
}
```

所有收键盘路径都调用该方法：

- 点击右侧半透明遮罩区域。
- 点击抽屉内部搜索框以外空白区域。
- 点击历史列表空白区域。
- 点击底部用户/设置栏背景。
- `WindowInsets.Type.ime()` 从可见变为不可见。
- 搜索框 `EditorInfo.IME_ACTION_DONE` 或键盘完成动作。

### 3.3 IME 可见状态监听

当前 `drawerLayer.setOnApplyWindowInsetsListener` 只在 `!imeVisible && searchExpanded[0] && search.hasFocus()` 时清焦点。v14 改为显式记录上一帧 IME 状态：

```java
final boolean[] imeWasVisible = {false};

drawerLayer.setOnApplyWindowInsetsListener((view, insets) -> {
    boolean imeVisible = insets.getInsets(WindowInsets.Type.ime()).bottom > 0;
    if (imeWasVisible[0] && !imeVisible && search.hasFocus()) {
        search.clearFocus();
        refreshSearchChrome.run();
    }
    imeWasVisible[0] = imeVisible;
    return insets;
});
```

这样只有“键盘从显示变为隐藏”时触发清焦点，避免首次布局时误清焦点。

### 3.4 搜索框键盘动作

搜索框应明确设置：

```java
search.setImeOptions(EditorInfo.IME_ACTION_DONE);
search.setSingleLine(true);
search.setOnEditorActionListener((v, actionId, event) -> {
    if (actionId == EditorInfo.IME_ACTION_DONE) {
        collapseDrawerSearch(search, refreshSearchChrome);
        return true;
    }
    return false;
});
```

## 4. 历史搜索结果一闪而过

### 4.1 当前问题根因

当前搜索流程：

```text
用户输入“我”
  立即 renderHistoryList(local search result)
  300ms 后请求 api.searchSessions("我")
  远程返回 items
  renderHistoryList(remote results)
```

如果远程搜索返回空、返回慢、后端只返回部分结果，或本地缓存的会话还没有完整对应服务端结果，就会出现：

```text
本地命中结果显示一瞬间 -> 远程空结果覆盖 -> 列表消失
```

这不是本地搜索失败，而是远程回调覆盖策略错误。

### 4.2 修复原则

搜索结果展示必须遵循：

```text
本地结果是基础结果。
远程结果只补充/更新本地结果。
远程空结果不能清空已有本地结果。
过期远程请求不能更新 UI。
搜索框清空时才恢复全量历史。
```

### 4.3 Android 合并策略

新增合并方法：

```java
private List<LocalChatStore.SessionSummary> mergeSessionResults(
    List<LocalChatStore.SessionSummary> local,
    List<LocalChatStore.SessionSummary> remote
)
```

规则：

- 用 `serverSessionId` 优先去重；没有服务端 id 时用 `localSessionId`。
- 保持本地结果排序在前。
- 远程新增结果追加到后面。
- 如果远程结果为空，返回本地结果。

伪代码：

```java
List<SessionSummary> base = chatStore.searchSessionsLocal(query);
renderHistoryList(historyList, base);

new Thread(() -> {
    JSONArray remote = api.searchSessions(query);
    List<SessionSummary> remoteItems = upsertRemoteResults(remote);
    runOnUiThread(() -> {
        if (version != searchVersion[0]) return;
        if (!query.equals(search.getText().toString().trim())) return;
        List<SessionSummary> latestLocal = chatStore.searchSessionsLocal(query);
        renderHistoryList(historyList, mergeSessionResults(latestLocal, remoteItems));
    });
}).start();
```

异常处理：

```text
远程搜索失败:
  保持当前本地结果
  不渲染空列表
  不 toast，避免用户每次输入都被打扰
```

### 4.4 同步刷新不能覆盖搜索结果

`syncRemoteSessions(onDone)` 完成后，如果搜索框不为空：

```java
String query = search.getText().toString().trim();
if (query.isEmpty()) {
    renderHistoryList(historyList, chatStore.recentSessionsWithMessages());
} else {
    renderHistoryList(historyList, chatStore.searchSessionsLocal(query));
}
```

这一逻辑可以保留，但要注意它也可能和远程搜索回调竞争。v14 建议：

- `syncRemoteSessions` 的刷新也走同一个 `renderSearchResults(query, version)` 方法。
- 所有异步回调都检查 `version == searchVersion[0]`。
- 搜索框清空时递增 `searchVersion`，让旧搜索回调失效。

## 5. 侧栏底部视觉回退

### 5.1 需求

侧栏底部用户和设置按钮背景改回白色；历史会话区和底部区域之间用横线分隔，类似四个导航按钮和历史区之间的分割线。

### 5.2 实现方案

当前 `bottomUserBar()` 背景为浅灰：

```java
bar.setBackgroundColor(Color.rgb(243, 244, 246));
```

v14 改回：

```java
bar.setBackgroundColor(Color.WHITE);
```

在加入底部栏前插入分割线：

```java
View bottomDivider = new View(this);
bottomDivider.setBackgroundColor(Color.rgb(238, 239, 242));
drawer.addView(bottomDivider, new LinearLayout.LayoutParams(-1, 1));
drawer.addView(bottomUserBar(), new LinearLayout.LayoutParams(-1, dp(68)));
```

注意：

- 分割线应在 `historyFrame` 和 `bottomUserBar` 之间。
- 底部栏仍可以响应空白点击收键盘。
- 不改变底部用户头像、用户名和齿轮按钮位置。

## 6. 个人资料页顶部按钮对齐

### 6.1 当前问题根因

`renderEditProfilePage()` 里“取消”和“保存”分别用 `muted()` 创建，`muted()` 默认带有上下 padding：

```java
view.setPadding(0, dp(8), 0, dp(8));
```

同时顶部栏高度是 64dp，左右文本和中间标题的字体大小/内边距不完全一致，容易造成“取消”看起来比“保存”更靠上。

### 6.2 修复方案

新增专用顶部栏文本方法：

```java
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
```

`renderEditProfilePage()` 顶部栏改为：

```java
LinearLayout bar = new LinearLayout(this);
bar.setGravity(Gravity.CENTER_VERTICAL);
bar.setPadding(dp(18), 0, dp(18), 0);

TextView cancel = topBarTextButton("取消", Color.BLACK);
TextView heading = title("个人资料");
heading.setGravity(Gravity.CENTER);
heading.setIncludeFontPadding(false);
TextView save = topBarTextButton("保存", disabledColor);

content.addView(bar, new LinearLayout.LayoutParams(-1, dp(56)));
```

关键要求：

- “取消”和“保存”使用同一个 helper。
- 顶部栏高度与其他二级页统一为 56dp。
- 两侧按钮布局权重相同：左/中/右均 `weight=1`。
- 禁用/启用只改颜色和 enabled，不改 padding、size、height。

## 7. 实施顺序

1. 侧栏搜索状态：
   - 新增 `collapseDrawerSearch`。
   - 记录 IME 上一帧可见状态。
   - 搜索框 DONE 动作清焦点。
2. 搜索结果稳定：
   - 增加本地/远程结果合并方法。
   - 远程空结果不覆盖本地结果。
   - 远程异常不清空列表。
3. 侧栏底部：
   - `bottomUserBar` 背景恢复白色。
   - `historyFrame` 和底部栏之间加入分割线。
4. 个人资料页顶部栏：
   - 增加 `topBarTextButton`。
   - “取消”和“保存”统一样式和高度。
5. 构建和模拟器回归。

## 8. 回归测试清单

### 8.1 Android 构建

```bash
ditto /Volumes/shared/xzxg-shop/android-native /private/tmp/xzxg-android-build
cd /private/tmp/xzxg-android-build
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```

### 8.2 模拟器手工验收

1. 打开侧栏，点击“历史会话搜索”，四个导航按钮收起。
2. 通过系统返回键收起键盘，搜索框失去焦点，四个导航按钮恢复。
3. 点击搜索框以外的抽屉空白区域，键盘收起，搜索框失焦。
4. 输入“我”，历史会话结果保持稳定，不再一闪而过。
5. 远程搜索失败或返回空时，本地命中结果仍然保留。
6. 清空搜索框，历史列表恢复为全部会话。
7. 侧栏底部用户/设置区域背景为白色，和历史区之间有横线。
8. 进入个人资料页，“取消”和“保存”垂直居中对齐。

## 9. 风险与边界

- v14 不改后端接口，不改数据库结构。
- 搜索稳定性主要取决于 Android 端合并策略；后端搜索仍作为补充结果源。
- 如果某些输入法不触发 `IME_ACTION_DONE`，仍依靠 `WindowInsets` 从可见到不可见的变化清焦点。
- 侧栏点击空白收键盘时，不应拦截历史会话行点击、底部设置按钮点击和导航按钮点击。
