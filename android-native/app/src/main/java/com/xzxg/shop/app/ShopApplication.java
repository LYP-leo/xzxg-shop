package com.xzxg.shop.app;

import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.LocalChatStore;
import com.xzxg.shop.storage.SessionStore;
import com.xzxg.shop.ui.ImageLoader;

import android.app.Application;

public class ShopApplication extends Application {
    private SessionStore sessionStore;
    private LocalChatStore localChatStore;
    private ApiClient apiClient;
    private ImageLoader imageLoader;

    public synchronized SessionStore sessionStore() {
        if (sessionStore == null) {
            sessionStore = new SessionStore(getApplicationContext());
        }
        return sessionStore;
    }

    public synchronized LocalChatStore localChatStore() {
        if (localChatStore == null) {
            localChatStore = new LocalChatStore(getApplicationContext());
        }
        return localChatStore;
    }

    public synchronized ApiClient apiClient() {
        if (apiClient == null) {
            apiClient = new ApiClient(sessionStore());
        }
        return apiClient;
    }

    public synchronized ApiClient refreshApiClient() {
        apiClient = new ApiClient(sessionStore());
        return apiClient;
    }

    public synchronized ImageLoader imageLoader() {
        if (imageLoader == null) {
            imageLoader = new ImageLoader();
        }
        return imageLoader;
    }
}
