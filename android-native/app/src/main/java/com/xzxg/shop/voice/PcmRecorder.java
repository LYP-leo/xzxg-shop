package com.xzxg.shop.voice;

import android.annotation.SuppressLint;
import android.media.AudioFormat;
import android.media.AudioRecord;
import android.media.MediaRecorder;

import java.util.Arrays;

class PcmRecorder {
    interface FrameListener {
        void onFrame(byte[] frame);
        void onError(Exception error);
    }

    static final int SAMPLE_RATE = 16000;
    static final int FRAME_MS = 40;
    static final int FRAME_BYTES = SAMPLE_RATE * 2 * FRAME_MS / 1000;

    private AudioRecord recorder;
    private Thread worker;
    private volatile boolean running;

    @SuppressLint("MissingPermission")
    void start(FrameListener listener) {
        stop();
        int minBuffer = AudioRecord.getMinBufferSize(
                SAMPLE_RATE,
                AudioFormat.CHANNEL_IN_MONO,
                AudioFormat.ENCODING_PCM_16BIT
        );
        if (minBuffer <= 0) {
            throw new IllegalStateException("当前设备不支持 16k 单声道录音");
        }
        int bufferSize = Math.max(minBuffer, FRAME_BYTES * 4);
        recorder = new AudioRecord(
                MediaRecorder.AudioSource.MIC,
                SAMPLE_RATE,
                AudioFormat.CHANNEL_IN_MONO,
                AudioFormat.ENCODING_PCM_16BIT,
                bufferSize
        );
        if (recorder.getState() != AudioRecord.STATE_INITIALIZED) {
            recorder.release();
            recorder = null;
            throw new IllegalStateException("录音设备初始化失败");
        }
        running = true;
        try {
            recorder.startRecording();
        } catch (Exception error) {
            running = false;
            recorder.release();
            recorder = null;
            throw error;
        }
        if (recorder.getRecordingState() != AudioRecord.RECORDSTATE_RECORDING) {
            running = false;
            recorder.release();
            recorder = null;
            throw new IllegalStateException("录音设备启动失败");
        }
        worker = new Thread(() -> readFrames(listener), "xzxg-pcm-recorder");
        worker.start();
    }

    void stop() {
        running = false;
        if (worker != null) {
            worker.interrupt();
            worker = null;
        }
        if (recorder != null) {
            try {
                recorder.stop();
            } catch (Exception ignored) {
            }
            recorder.release();
            recorder = null;
        }
    }

    private void readFrames(FrameListener listener) {
        byte[] buffer = new byte[FRAME_BYTES];
        while (running && recorder != null) {
            try {
                int offset = 0;
                while (running && offset < FRAME_BYTES) {
                    int read = recorder.read(buffer, offset, FRAME_BYTES - offset);
                    if (read < 0) {
                        throw new IllegalStateException("录音读取失败：" + read);
                    }
                    offset += read;
                }
                if (running && listener != null) {
                    listener.onFrame(Arrays.copyOf(buffer, FRAME_BYTES));
                }
            } catch (Exception error) {
                running = false;
                if (listener != null) {
                    listener.onError(error);
                }
            }
        }
    }
}
