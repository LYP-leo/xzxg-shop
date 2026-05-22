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
        instance(target.getContext()).setMarkdown(target, markdown == null ? "" : markdown);
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
