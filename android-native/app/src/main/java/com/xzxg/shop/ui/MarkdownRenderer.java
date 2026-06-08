package com.xzxg.shop.ui;

import android.content.Context;
import android.graphics.Color;
import android.graphics.Typeface;
import android.text.Spannable;
import android.text.SpannableString;
import android.text.style.ForegroundColorSpan;
import android.text.style.RelativeSizeSpan;
import android.text.style.StyleSpan;
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
        setMarkdown(target, markdown, 15);
    }

    public static void setMarkdown(TextView target, String markdown, float textSizeSp) {
        String source = markdown == null ? "" : markdown;
        target.setTextSize(textSizeSp);
        instance(target.getContext()).setMarkdown(target, normalizeHeadings(source));
        applyHeadingColors(target, source);
    }

    private static String normalizeHeadings(String markdown) {
        String[] lines = markdown.split("\\n", -1);
        StringBuilder builder = new StringBuilder();
        for (int i = 0; i < lines.length; i++) {
            String line = lines[i];
            if (line.startsWith("### ")) {
                line = line.substring(4).trim();
            } else if (line.startsWith("## ")) {
                line = line.substring(3).trim();
            } else if (line.startsWith("# ")) {
                line = line.substring(2).trim();
            }
            if (i > 0) {
                builder.append('\n');
            }
            builder.append(line);
        }
        return builder.toString();
    }

    private static void applyHeadingColors(TextView target, String markdown) {
        CharSequence rendered = target.getText();
        if (rendered == null || rendered.length() == 0 || markdown == null || markdown.isEmpty()) {
            return;
        }
        Spannable spannable = rendered instanceof Spannable
                ? (Spannable) rendered
                : new SpannableString(rendered);
        String renderedText = rendered.toString();
        String[] lines = markdown.split("\\n", -1);
        int searchFrom = 0;
        for (String line : lines) {
            Heading heading = parseHeading(line);
            if (heading == null || heading.text.isEmpty()) {
                continue;
            }
            int start = renderedText.indexOf(heading.text, searchFrom);
            if (start < 0) {
                start = renderedText.indexOf(heading.text);
            }
            if (start < 0) {
                continue;
            }
            int end = start + heading.text.length();
            spannable.setSpan(new ForegroundColorSpan(heading.color), start, end, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
            spannable.setSpan(new StyleSpan(Typeface.BOLD), start, end, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
            spannable.setSpan(new RelativeSizeSpan(heading.sizeMultiplier), start, end, Spannable.SPAN_EXCLUSIVE_EXCLUSIVE);
            searchFrom = end;
        }
        target.setText(spannable);
    }

    private static Heading parseHeading(String line) {
        if (line == null) {
            return null;
        }
        if (line.startsWith("### ")) {
            return new Heading(line.substring(4).trim(), Color.rgb(46, 125, 107), 1.0f);
        }
        if (line.startsWith("## ")) {
            return new Heading(line.substring(3).trim(), Color.rgb(31, 106, 165), 1.08f);
        }
        if (line.startsWith("# ")) {
            return new Heading(line.substring(2).trim(), Color.rgb(15, 76, 129), 1.2f);
        }
        return null;
    }

    private static class Heading {
        final String text;
        final int color;
        final float sizeMultiplier;

        Heading(String text, int color, float sizeMultiplier) {
            this.text = text == null ? "" : text;
            this.color = color;
            this.sizeMultiplier = sizeMultiplier;
        }
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
