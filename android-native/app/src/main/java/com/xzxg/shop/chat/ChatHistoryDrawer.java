package com.xzxg.shop.chat;

import com.xzxg.shop.storage.LocalChatStore;

import org.json.JSONArray;
import org.json.JSONObject;

import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.ArrayList;
import java.util.Date;
import java.util.HashSet;
import java.util.List;
import java.util.Locale;

public class ChatHistoryDrawer {
    public static final int PAGE_SIZE = 20;

    private final LocalChatStore chatStore;
    private int offset;
    private boolean hasMore = true;
    private boolean loading;
    private boolean locallyMutated;
    private boolean hasSavedState;
    private int savedScrollY;
    private int savedOffset;
    private String savedQuery = "";
    private String query = "";

    public ChatHistoryDrawer(LocalChatStore chatStore) {
        this.chatStore = chatStore;
    }

    public String initialQuery() {
        return hasSavedState ? savedQuery : "";
    }

    public String query() {
        return query == null ? "" : query;
    }

    public int offset() {
        return offset;
    }

    public boolean hasMore() {
        return hasMore;
    }

    public boolean isLoading() {
        return loading;
    }

    public void setLoading(boolean loading) {
        this.loading = loading;
    }

    public void setLocallyMutated(boolean locallyMutated) {
        this.locallyMutated = locallyMutated;
    }

    public boolean locallyMutated() {
        return locallyMutated;
    }

    public int savedScrollY() {
        return savedScrollY;
    }

    public void saveState(int scrollY, String searchQuery) {
        hasSavedState = true;
        savedScrollY = Math.max(0, scrollY);
        savedOffset = Math.max(PAGE_SIZE, offset);
        savedQuery = safe(searchQuery).trim();
    }

    public boolean shouldRestoreSavedState(String value) {
        return hasSavedState && safe(value).trim().equals(savedQuery);
    }

    public void resetPaging(String value) {
        query = safe(value).trim();
        offset = 0;
        hasMore = true;
        loading = false;
    }

    public List<LocalChatStore.SessionSummary> initialPage(String value) {
        String keyword = safe(value).trim();
        return shouldRestoreSavedState(keyword)
                ? loadLocalPageForSavedState(keyword)
                : loadLocalPage(keyword, 0);
    }

    public List<LocalChatStore.SessionSummary> loadLocalPage(String value, int pageOffset) {
        List<LocalChatStore.SessionSummary> page;
        String keyword = safe(value).trim();
        if (keyword.isEmpty()) {
            page = chatStore.recentSessionsWithMessages(PAGE_SIZE, pageOffset);
        } else {
            page = chatStore.searchSessionsLocal(keyword, PAGE_SIZE, pageOffset);
        }
        offset = pageOffset + (page == null ? 0 : page.size());
        hasMore = page != null && page.size() >= PAGE_SIZE;
        return page;
    }

    public List<LocalChatStore.SessionSummary> loadLocalPageForSavedState(String value) {
        ArrayList<LocalChatStore.SessionSummary> result = new ArrayList<>();
        HashSet<String> seen = new HashSet<>();
        int pageOffset = 0;
        boolean pageHasMore = false;
        int targetCount = Math.max(PAGE_SIZE, savedOffset);
        while (true) {
            List<LocalChatStore.SessionSummary> page;
            String keyword = safe(value).trim();
            if (keyword.isEmpty()) {
                page = chatStore.recentSessionsWithMessages(PAGE_SIZE, pageOffset);
            } else {
                page = chatStore.searchSessionsLocal(keyword, PAGE_SIZE, pageOffset);
            }
            if (page == null || page.isEmpty()) {
                pageHasMore = false;
                break;
            }
            pageOffset += page.size();
            pageHasMore = page.size() >= PAGE_SIZE;
            for (LocalChatStore.SessionSummary item : page) {
                if (item == null || !seen.add(item.localSessionId)) {
                    continue;
                }
                result.add(item);
            }
            if (result.size() >= targetCount || !pageHasMore) {
                break;
            }
        }
        query = safe(value).trim();
        offset = pageOffset;
        hasMore = pageHasMore;
        return result;
    }

    public void applyRemoteAppend(String requestedQuery, int requestedOffset, int itemCount, boolean remoteHasMore) {
        query = safe(requestedQuery).trim();
        offset = requestedOffset + Math.max(0, itemCount);
        hasMore = remoteHasMore;
        loading = false;
    }

    public void applyRemoteReplace(String requestedQuery, int requestedOffset, int itemCount, boolean remoteHasMore) {
        query = safe(requestedQuery).trim();
        offset = requestedOffset + Math.max(0, itemCount);
        hasMore = remoteHasMore;
        loading = false;
    }

    public void mergeRemoteHasMore(boolean remoteHasMore) {
        hasMore = hasMore || remoteHasMore;
        loading = false;
    }

    public List<LocalChatStore.SessionSummary> upsertRemoteSearchResults(JSONArray sessions) {
        ArrayList<LocalChatStore.SessionSummary> results = new ArrayList<>();
        if (sessions == null) {
            return results;
        }
        for (int i = 0; i < sessions.length(); i++) {
            JSONObject item = sessions.optJSONObject(i);
            if (item == null) {
                continue;
            }
            String localId = chatStore.upsertRemoteSession(
                    item.optString("session_id", ""),
                    item.optString("title", "导购会话"),
                    item.optString("summary", ""),
                    remoteSessionTime(item)
            );
            LocalChatStore.SessionSummary summary = chatStore.sessionSummary(localId);
            if (summary != null) {
                results.add(summary);
            }
        }
        return results;
    }

    public List<LocalChatStore.SessionSummary> ensureActiveSessionVisible(String activeLocalSessionId, List<LocalChatStore.SessionSummary> histories) {
        ArrayList<LocalChatStore.SessionSummary> result = new ArrayList<>();
        boolean found = false;
        if (histories != null) {
            for (LocalChatStore.SessionSummary item : histories) {
                if (item == null) {
                    continue;
                }
                if (activeLocalSessionId != null && activeLocalSessionId.equals(item.localSessionId)) {
                    found = true;
                }
                result.add(item);
            }
        }
        if (!found && activeLocalSessionId != null && !activeLocalSessionId.isEmpty()) {
            LocalChatStore.SessionSummary active = chatStore.sessionSummary(activeLocalSessionId);
            if (active != null) {
                result.add(0, active);
            }
        }
        return result;
    }

    public static long remoteSessionTime(JSONObject item) {
        if (item == null) {
            return 1;
        }
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

    public static long parseRemoteTime(String value) {
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

    private String safe(String value) {
        return value == null ? "" : value;
    }
}
