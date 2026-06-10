package com.xzxg.shop.tts;

import com.xzxg.shop.network.ApiClient;
import com.xzxg.shop.storage.SessionStore;

import org.json.JSONObject;

import java.util.ArrayDeque;
import java.util.Deque;

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
    private final Object queueLock = new Object();
    private final Deque<String> speechQueue = new ArrayDeque<>();
    private StringBuilder streamBuffer = new StringBuilder();
    private boolean workerRunning;
    private boolean unavailableNotified;

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

    public void startAssistantStream() {
        synchronized (queueLock) {
            generation++;
            speechQueue.clear();
            streamBuffer = new StringBuilder();
            workerRunning = false;
            unavailableNotified = false;
        }
        playback.stop();
    }

    public void appendAssistantMarkdownDelta(String markdownDelta) {
        if (!enabled || markdownDelta == null || markdownDelta.isEmpty()) {
            return;
        }
        String text = TtsTextSanitizer.sanitize(markdownDelta);
        if (text.isEmpty()) {
            return;
        }
        synchronized (queueLock) {
            streamBuffer.append(text);
            streamBuffer.append(' ');
            enqueueReadyChunksLocked(false);
            startWorkerLocked();
        }
    }

    public void finishAssistantStream() {
        if (!enabled) {
            return;
        }
        synchronized (queueLock) {
            enqueueReadyChunksLocked(true);
            startWorkerLocked();
        }
    }

    public void stop() {
        synchronized (queueLock) {
            generation++;
            speechQueue.clear();
            streamBuffer = new StringBuilder();
            workerRunning = false;
            unavailableNotified = false;
        }
        playback.stop();
    }

    public void destroy() {
        stop();
    }

    private void notifyUnavailable(String message) {
        synchronized (queueLock) {
            if (unavailableNotified) {
                return;
            }
            unavailableNotified = true;
        }
        Listener current = listener;
        if (current != null && enabled) {
            current.onUnavailable(message);
        }
    }

    private void enqueueReadyChunksLocked(boolean flushAll) {
        while (streamBuffer.length() > 0) {
            int end = readyChunkEnd(streamBuffer, flushAll);
            if (end <= 0) {
                return;
            }
            String chunk = streamBuffer.substring(0, end).trim();
            streamBuffer.delete(0, end);
            if (!chunk.isEmpty()) {
                speechQueue.addLast(chunk);
            }
        }
    }

    private int readyChunkEnd(StringBuilder text, boolean flushAll) {
        int length = text.length();
        if (length == 0) {
            return 0;
        }
        int punctuationEnd = lastSentenceBoundary(text);
        if (punctuationEnd > 0 && (punctuationEnd >= 24 || flushAll)) {
            return punctuationEnd;
        }
        if (length >= 90) {
            return punctuationEnd > 0 ? punctuationEnd : length;
        }
        return flushAll ? length : 0;
    }

    private int lastSentenceBoundary(StringBuilder text) {
        for (int i = text.length() - 1; i >= 0; i--) {
            char ch = text.charAt(i);
            if (ch == '。' || ch == '！' || ch == '？' || ch == '；' || ch == '!' || ch == '?' || ch == ';' || ch == '\n') {
                return i + 1;
            }
        }
        return 0;
    }

    private void startWorkerLocked() {
        if (workerRunning || speechQueue.isEmpty()) {
            return;
        }
        workerRunning = true;
        int workerGeneration = generation;
        new Thread(() -> drainSpeechQueue(workerGeneration)).start();
    }

    private void drainSpeechQueue(int workerGeneration) {
        while (true) {
            String chunk;
            synchronized (queueLock) {
                if (workerGeneration != generation || !enabled) {
                    workerRunning = false;
                    return;
                }
                chunk = speechQueue.pollFirst();
                if (chunk == null) {
                    workerRunning = false;
                    return;
                }
            }
            try {
                JSONObject config = api.speechTtsConfig();
                if (!config.optBoolean("enabled", false)) {
                    notifyUnavailable("语音朗读暂不可用");
                    continue;
                }
                int maxRunes = config.optInt("max_text_chars", 800);
                String payload = truncateByRunes(chunk, maxRunes <= 0 ? 800 : maxRunes);
                byte[] audio = api.synthesizeSpeechTTS(payload, config.optString("voice", ""));
                if (workerGeneration == generation && enabled) {
                    playback.playAndWait(audio);
                }
            } catch (Exception error) {
                notifyUnavailable("语音朗读暂不可用");
            }
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
