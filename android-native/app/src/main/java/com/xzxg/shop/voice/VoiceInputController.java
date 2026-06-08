package com.xzxg.shop.voice;

import com.xzxg.shop.storage.SessionStore;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.speech.RecognitionListener;
import android.speech.RecognizerIntent;
import android.speech.SpeechRecognizer;

import java.util.ArrayList;
import java.util.Locale;

public class VoiceInputController {
    public interface AndroidSpeechListener {
        void onRecognized(String text);

        void onError(String message);
    }

    public interface RealtimeSpeechListener {
        void onReady();

        void onPartial(String text);

        void onRecognizing();

        void onFinal(String text);

        void onError(String code, String message);

        void onStopped();
    }

    public enum SpeechMode {
        AUTO,
        XUNFEI_REALTIME,
        ANDROID_INLINE,
        ANDROID_ACTIVITY
    }

    private SpeechMode speechMode = SpeechMode.AUTO;
    private SpeechRecognizer speechRecognizer;
    private String lastPartialSpeech = "";
    private boolean inlineResultSent;
    private SpeechRealtimeClient realtimeClient;
    private PcmRecorder pcmRecorder;
    private boolean realtimeActive;
    private boolean realtimeEnding;
    private boolean realtimeFinalSent;
    private boolean realtimeRecorderStarted;
    private String realtimePartialText = "";
    private long realtimeStartedAt;
    private final Handler realtimeHandler = new Handler(Looper.getMainLooper());

    public SpeechMode currentMode() {
        return speechMode == null ? SpeechMode.AUTO : speechMode;
    }

    public void setSpeechMode(SpeechMode speechMode) {
        this.speechMode = speechMode == null ? SpeechMode.AUTO : speechMode;
    }

    public Intent recognitionIntent(boolean partialResults) {
        Intent intent = new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH);
        intent.putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM);
        intent.putExtra(RecognizerIntent.EXTRA_LANGUAGE, Locale.CHINA.toString());
        if (partialResults) {
            intent.putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true);
        }
        return intent;
    }

    public boolean isInlineRecognitionAvailable(Context context) {
        return SpeechRecognizer.isRecognitionAvailable(context);
    }

    public boolean isRealtimeActive() {
        return realtimeActive;
    }

    public boolean isRealtimeEnding() {
        return realtimeEnding;
    }

    public void startRealtimeSpeech(SessionStore sessionStore, RealtimeSpeechListener listener) {
        cleanupRealtime(null);
        realtimeActive = true;
        realtimeEnding = false;
        realtimeFinalSent = false;
        realtimeRecorderStarted = false;
        realtimePartialText = "";
        realtimeStartedAt = System.currentTimeMillis();
        pcmRecorder = new PcmRecorder();
        realtimeClient = new SpeechRealtimeClient(sessionStore, new SpeechRealtimeClient.Listener() {
            @Override
            public void onReady() {
                realtimeHandler.post(() -> {
                    if (!realtimeActive) {
                        return;
                    }
                    if (listener != null) {
                        listener.onReady();
                    }
                    try {
                        pcmRecorder.start(new PcmRecorder.FrameListener() {
                            @Override
                            public void onFrame(byte[] frame) {
                                SpeechRealtimeClient client = realtimeClient;
                                if (realtimeActive && client != null) {
                                    client.sendAudio(frame);
                                }
                            }

                            @Override
                            public void onError(Exception error) {
                                realtimeHandler.post(() -> handleRealtimeError("speech_recognition_failed", "录音失败，请重试", listener));
                            }
                        });
                        realtimeRecorderStarted = true;
                    } catch (Exception error) {
                        handleRealtimeError("speech_recognition_failed", "录音失败，请重试", listener);
                    }
                });
            }

            @Override
            public void onPartial(String text) {
                realtimeHandler.post(() -> {
                    realtimePartialText = text == null ? "" : text.trim();
                    if (listener != null && realtimeActive && !realtimePartialText.isEmpty()) {
                        listener.onPartial(realtimePartialText);
                    }
                });
            }

            @Override
            public void onFinal(String text) {
                realtimeHandler.post(() -> sendRealtimeFinal(text, listener));
            }

            @Override
            public void onError(String code, String message) {
                realtimeHandler.post(() -> handleRealtimeError(code, message, listener));
            }

            @Override
            public void onClosed() {
            }
        });
        realtimeClient.connect();
        realtimeHandler.postDelayed(() -> {
            if (realtimeActive && !realtimeRecorderStarted && !realtimeEnding) {
                handleRealtimeError("speech_network_error", "语音识别连接超时", listener);
            }
        }, 8000);
        realtimeHandler.postDelayed(() -> {
            if (realtimeActive && !realtimeEnding) {
                finishRealtimeSpeech(false, listener);
            }
        }, 60000);
    }

    public void finishRealtimeSpeech(boolean userStop, RealtimeSpeechListener listener) {
        if (!realtimeActive || realtimeEnding) {
            return;
        }
        long duration = System.currentTimeMillis() - realtimeStartedAt;
        if (userStop && duration < 800) {
            if (listener != null) {
                listener.onError("speech_too_short", "说话时间太短");
            }
            cancelRealtimeSpeech(listener);
            return;
        }
        realtimeEnding = true;
        stopRecorder();
        if (realtimeClient != null) {
            realtimeClient.sendEnd();
        }
        if (listener != null) {
            listener.onRecognizing();
        }
        realtimeHandler.postDelayed(() -> {
            if (realtimeActive && realtimeEnding && !realtimeFinalSent) {
                sendRealtimeFinal(realtimePartialText, listener);
            }
        }, 3000);
    }

    public void cancelRealtimeSpeech(RealtimeSpeechListener listener) {
        stopRecorder();
        if (realtimeClient != null) {
            realtimeClient.cancel();
            realtimeClient = null;
        }
        cleanupRealtimeState(listener);
    }

    public void startInlineRecognition(Activity activity, AndroidSpeechListener listener) {
        destroyRecognizer();
        inlineResultSent = false;
        lastPartialSpeech = "";
        speechRecognizer = SpeechRecognizer.createSpeechRecognizer(activity);
        speechRecognizer.setRecognitionListener(new RecognitionListener() {
            @Override public void onReadyForSpeech(Bundle params) {}
            @Override public void onBeginningOfSpeech() {}
            @Override public void onRmsChanged(float rmsdB) {}
            @Override public void onBufferReceived(byte[] buffer) {}
            @Override public void onEndOfSpeech() {}
            @Override public void onEvent(int eventType, Bundle params) {}

            @Override
            public void onPartialResults(Bundle partialResults) {
                ArrayList<String> texts = partialResults.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);
                if (texts != null && !texts.isEmpty()) {
                    lastPartialSpeech = texts.get(0);
                }
            }

            @Override
            public void onError(int error) {
                activity.runOnUiThread(() -> {
                    if (inlineResultSent) {
                        return;
                    }
                    if ((error == SpeechRecognizer.ERROR_NO_MATCH || error == SpeechRecognizer.ERROR_SPEECH_TIMEOUT)
                            && lastPartialSpeech != null && !lastPartialSpeech.trim().isEmpty()) {
                        inlineResultSent = true;
                        listener.onRecognized(lastPartialSpeech);
                        return;
                    }
                    inlineResultSent = true;
                    listener.onError(errorText(error));
                });
            }

            @Override
            public void onResults(Bundle results) {
                activity.runOnUiThread(() -> {
                    if (inlineResultSent) {
                        return;
                    }
                    ArrayList<String> texts = results.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);
                    if (texts == null || texts.isEmpty() || texts.get(0).trim().isEmpty()) {
                        inlineResultSent = true;
                        listener.onError("没有识别到内容");
                        return;
                    }
                    inlineResultSent = true;
                    listener.onRecognized(texts.get(0));
                });
            }
        });
        speechRecognizer.startListening(recognitionIntent(true));
    }

    public void stopRecognizer() {
        if (speechRecognizer != null) {
            speechRecognizer.stopListening();
        }
    }

    public void destroyRecognizer() {
        if (speechRecognizer != null) {
            speechRecognizer.destroy();
            speechRecognizer = null;
        }
        lastPartialSpeech = "";
        inlineResultSent = false;
    }

    public void destroy() {
        cancelRealtimeSpeech(null);
        destroyRecognizer();
    }

    public String errorText(int error) {
        if (error == SpeechRecognizer.ERROR_NO_MATCH) {
            return "没有识别到内容";
        }
        if (error == SpeechRecognizer.ERROR_SPEECH_TIMEOUT) {
            return "未检测到语音";
        }
        if (error == SpeechRecognizer.ERROR_NETWORK || error == SpeechRecognizer.ERROR_NETWORK_TIMEOUT) {
            return "语音识别网络异常，请稍后重试";
        }
        return "语音识别失败，请重试";
    }

    private void sendRealtimeFinal(String text, RealtimeSpeechListener listener) {
        if (realtimeFinalSent) {
            return;
        }
        String value = text == null ? "" : text.trim();
        if (value.isEmpty()) {
            value = realtimePartialText == null ? "" : realtimePartialText.trim();
        }
        if (value.isEmpty()) {
            if (listener != null) {
                listener.onError("speech_no_match", "没有识别到内容");
            }
            cleanupRealtime(listener);
            return;
        }
        realtimeFinalSent = true;
        cleanupRealtime(listener);
        if (listener != null) {
            listener.onFinal(value);
        }
    }

    private void handleRealtimeError(String code, String message, RealtimeSpeechListener listener) {
        if (!realtimeActive || realtimeFinalSent) {
            return;
        }
        cleanupRealtime(listener);
        if (listener != null) {
            listener.onError(code, message);
        }
    }

    private void cleanupRealtime(RealtimeSpeechListener listener) {
        stopRecorder();
        if (realtimeClient != null) {
            realtimeClient.close();
            realtimeClient = null;
        }
        cleanupRealtimeState(listener);
    }

    private void cleanupRealtimeState(RealtimeSpeechListener listener) {
        realtimeActive = false;
        realtimeEnding = false;
        realtimeFinalSent = false;
        realtimeRecorderStarted = false;
        realtimePartialText = "";
        realtimeHandler.removeCallbacksAndMessages(null);
        if (listener != null) {
            listener.onStopped();
        }
    }

    private void stopRecorder() {
        if (pcmRecorder != null) {
            pcmRecorder.stop();
            pcmRecorder = null;
        }
    }
}
