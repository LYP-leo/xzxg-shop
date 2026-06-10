package com.xzxg.shop.tts;

import android.content.Context;
import android.media.MediaPlayer;
import android.net.Uri;

import java.io.File;
import java.io.FileOutputStream;

public class TtsPlaybackController {
    private final Context context;
    private MediaPlayer player;

    public TtsPlaybackController(Context context) {
        this.context = context.getApplicationContext();
    }

    public synchronized void play(byte[] audio) throws Exception {
        stop();
        if (audio == null || audio.length == 0) {
            return;
        }
        File file = new File(context.getCacheDir(), "chat-tts-current.mp3");
        try (FileOutputStream out = new FileOutputStream(file, false)) {
            out.write(audio);
        }
        MediaPlayer mediaPlayer = new MediaPlayer();
        mediaPlayer.setDataSource(context, Uri.fromFile(file));
        mediaPlayer.setOnCompletionListener(mp -> stop());
        mediaPlayer.setOnErrorListener((mp, what, extra) -> {
            stop();
            return true;
        });
        mediaPlayer.prepare();
        player = mediaPlayer;
        mediaPlayer.start();
    }

    public void playAndWait(byte[] audio) throws Exception {
        if (audio == null || audio.length == 0) {
            return;
        }
        MediaPlayer mediaPlayer;
        synchronized (this) {
            stop();
            File file = new File(context.getCacheDir(), "chat-tts-current.mp3");
            try (FileOutputStream out = new FileOutputStream(file, false)) {
                out.write(audio);
            }
            mediaPlayer = new MediaPlayer();
            mediaPlayer.setDataSource(context, Uri.fromFile(file));
            mediaPlayer.setOnCompletionListener(mp -> {
                synchronized (TtsPlaybackController.this) {
                    if (player == mp) {
                        player = null;
                    }
                }
                try {
                    mp.release();
                } catch (Exception ignored) {
                }
            });
            mediaPlayer.setOnErrorListener((mp, what, extra) -> {
                synchronized (TtsPlaybackController.this) {
                    if (player == mp) {
                        player = null;
                    }
                }
                try {
                    mp.release();
                } catch (Exception ignored) {
                }
                return true;
            });
            mediaPlayer.prepare();
            player = mediaPlayer;
            mediaPlayer.start();
        }
        while (true) {
            synchronized (this) {
                if (player != mediaPlayer) {
                    return;
                }
            }
            Thread.sleep(80);
        }
    }

    public synchronized void stop() {
        if (player == null) {
            return;
        }
        try {
            if (player.isPlaying()) {
                player.stop();
            }
        } catch (Exception ignored) {
        }
        try {
            player.release();
        } catch (Exception ignored) {
        }
        player = null;
    }
}
