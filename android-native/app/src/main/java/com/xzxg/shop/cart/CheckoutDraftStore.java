package com.xzxg.shop.cart;

import org.json.JSONObject;

import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

public final class CheckoutDraftStore {
    private static final Map<String, JSONObject> DRAFTS = new HashMap<>();

    private CheckoutDraftStore() {
    }

    public static synchronized String put(JSONObject cartSnapshot) {
        String id = UUID.randomUUID().toString();
        DRAFTS.put(id, cartSnapshot == null ? new JSONObject() : cartSnapshot);
        return id;
    }

    public static synchronized JSONObject get(String id) {
        if (id == null || id.isEmpty()) {
            return null;
        }
        return DRAFTS.get(id);
    }

    public static synchronized void remove(String id) {
        if (id != null && !id.isEmpty()) {
            DRAFTS.remove(id);
        }
    }
}
