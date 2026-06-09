package com.xzxg.shop.tts;

import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.SessionStore;

import org.json.JSONObject;

public class TtsController {
    public interface Listener {
        void onStateChanged(boolean enabled);
        void onUnavailable(String message);
    }

    private final SessionStore sessionStore;
    private final ApiClient api;
    private final TtsPlaybackController playback;
    private Listener listener;
    private volatile boolean enabled;
    private volatile int generation;

    public TtsController(SessionStore sessionStore, ApiClient api, TtsPlaybackController playback) {
        this.sessionStore = sessionStore;
        this.api = api;
        this.playback = playback;
        this.enabled = sessionStore.ttsEnabled();
    }

    public void setListener(Listener listener) {
        this.listener = listener;
    }

    public boolean isEnabled() {
        return enabled;
    }

    public void toggle() {
        setEnabled(!enabled);
    }

    public void setEnabled(boolean value) {
        enabled = value;
        sessionStore.saveTtsEnabled(value);
        if (!value) {
            stop();
        }
        Listener current = listener;
        if (current != null) {
            current.onStateChanged(value);
        }
    }

    public void speakAssistantMarkdown(String markdown) {
        if (!enabled) {
            return;
        }
        String text = TtsTextSanitizer.sanitize(markdown);
        if (text.isEmpty()) {
            return;
        }
        int requestGeneration = ++generation;
        new Thread(() -> {
            try {
                JSONObject config = api.speechTtsConfig();
                if (!config.optBoolean("enabled", false)) {
                    notifyUnavailable("语音朗读暂不可用");
                    return;
                }
                int maxRunes = config.optInt("max_text_chars", 800);
                String payload = truncateByRunes(text, maxRunes <= 0 ? 800 : maxRunes);
                byte[] audio = api.synthesizeSpeechTTS(payload, config.optString("voice", ""));
                if (requestGeneration == generation && enabled) {
                    playback.play(audio);
                }
            } catch (Exception error) {
                notifyUnavailable("语音朗读暂不可用");
            }
        }).start();
    }

    public void stop() {
        generation++;
        playback.stop();
    }

    public void destroy() {
        stop();
    }

    private void notifyUnavailable(String message) {
        Listener current = listener;
        if (current != null && enabled) {
            current.onUnavailable(message);
        }
    }

    private static String truncateByRunes(String value, int maxRunes) {
        if (value == null || maxRunes <= 0) {
            return "";
        }
        int count = value.codePointCount(0, value.length());
        if (count <= maxRunes) {
            return value;
        }
        int end = value.offsetByCodePoints(0, maxRunes);
        return value.substring(0, end);
    }
}
