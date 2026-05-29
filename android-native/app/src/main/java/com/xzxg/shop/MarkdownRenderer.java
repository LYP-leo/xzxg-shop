package com.xzxg.shop;

import android.content.Context;
import android.widget.TextView;

import io.noties.markwon.Markwon;
import io.noties.markwon.ext.tables.TablePlugin;
import io.noties.markwon.ext.tasklist.TaskListPlugin;
import io.noties.markwon.html.HtmlPlugin;
import io.noties.markwon.image.ImagesPlugin;

public final class MarkdownRenderer {
    private static Markwon markwon;

    private MarkdownRenderer() {
    }

    public static void setMarkdown(TextView target, String markdown) {
        target.setTextSize(15);
        instance(target.getContext()).setMarkdown(target, normalizeHeadings(markdown == null ? "" : markdown));
    }

    private static String normalizeHeadings(String markdown) {
        String[] lines = markdown.split("\\n", -1);
        StringBuilder builder = new StringBuilder();
        for (int i = 0; i < lines.length; i++) {
            String line = lines[i];
            if (line.startsWith("### ")) {
                line = "**" + line.substring(4).trim() + "**";
            } else if (line.startsWith("## ")) {
                line = "**" + line.substring(3).trim() + "**";
            } else if (line.startsWith("# ")) {
                line = "**" + line.substring(2).trim() + "**";
            }
            if (i > 0) {
                builder.append('\n');
            }
            builder.append(line);
        }
        return builder.toString();
    }

    private static Markwon instance(Context context) {
        if (markwon == null) {
            markwon = Markwon.builder(context.getApplicationContext())
                    .usePlugin(TablePlugin.create(context.getApplicationContext()))
                    .usePlugin(TaskListPlugin.create(context.getApplicationContext()))
                    .usePlugin(HtmlPlugin.create())
                    .usePlugin(ImagesPlugin.create())
                    .build();
        }
        return markwon;
    }
}
