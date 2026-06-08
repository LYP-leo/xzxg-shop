package com.xzxg.shop.chat;

import com.xzxg.shop.ui.MarkdownRenderer;
import com.xzxg.shop.ui.ShopUi;

import android.content.Context;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.HorizontalScrollView;
import android.widget.LinearLayout;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.List;
import java.util.regex.Pattern;

public class ChatMarkdownRenderer {
    public interface MarkdownSanitizer {
        String visibleMarkdown(String markdown);
    }

    public interface CopyBinder {
        void bind(View view, String text);
    }

    public static class Part {
        public final boolean table;
        public final String content;

        Part(boolean table, String content) {
            this.table = table;
            this.content = content == null ? "" : content;
        }
    }

    private final Context context;
    private final MarkdownSanitizer sanitizer;
    private final CopyBinder copyBinder;

    public ChatMarkdownRenderer(Context context, MarkdownSanitizer sanitizer, CopyBinder copyBinder) {
        this.context = context;
        this.sanitizer = sanitizer;
        this.copyBinder = copyBinder;
    }

    public TextView addPartsBubbleTo(String text, LinearLayout parent) {
        LinearLayout bubble = new LinearLayout(context);
        bubble.setOrientation(LinearLayout.VERTICAL);
        bubble.setPadding(dp(12), dp(9), dp(12), dp(9));
        bubble.setBackground(rounded(Color.WHITE, dp(16)));
        int bubbleWidth = (int) (context.getResources().getDisplayMetrics().widthPixels * 0.74f);
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(bubbleWidth, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.gravity = Gravity.LEFT;
        params.setMargins(0, dp(6), 0, dp(6));
        bindCopy(bubble, text);
        TextView firstText = null;
        for (Part part : splitParts(text)) {
            if (part.table) {
                addTableToBubble(bubble, part.content);
            } else if (!part.content.trim().isEmpty()) {
                TextView normal = markdownTextView(part.content);
                if (firstText == null) {
                    firstText = normal;
                }
                bubble.addView(normal, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            }
        }
        if (firstText == null) {
            firstText = markdownTextView(text);
            bubble.addView(firstText, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        }
        parent.addView(bubble, params);
        return firstText;
    }

    public void addTableToBubble(LinearLayout bubble, String markdown) {
        if (bubble == null || markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        HorizontalScrollView scroll = new HorizontalScrollView(context);
        scroll.setHorizontalScrollBarEnabled(true);
        View table = tableView(markdown);
        bindCopy(scroll, markdown);
        bindCopy(table, markdown);
        scroll.addView(table, new HorizontalScrollView.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
        LinearLayout.LayoutParams tableParams = new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        tableParams.setMargins(0, dp(8), 0, dp(10));
        bubble.addView(scroll, tableParams);
    }

    public void addPartsToBubble(LinearLayout bubble, String markdown) {
        if (bubble == null || markdown == null || markdown.trim().isEmpty()) {
            return;
        }
        for (Part part : splitParts(markdown)) {
            if (part.table) {
                addTableToBubble(bubble, part.content);
            } else if (!part.content.trim().isEmpty()) {
                bubble.addView(markdownTextView(part.content), new LinearLayout.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            }
        }
    }

    public TextView markdownTextView(String markdown) {
        TextView view = new TextView(context);
        MarkdownRenderer.setMarkdown(view, sanitize(markdown));
        view.setTextSize(15);
        view.setLineSpacing(4, 1);
        view.setTextColor(Color.rgb(24, 30, 37));
        view.setLinksClickable(true);
        bindCopy(view, markdown);
        return view;
    }

    public boolean containsTable(String text) {
        if (text == null) {
            return false;
        }
        return Pattern.compile("(?m)^\\s*\\|.+\\|\\s*$").matcher(text).find()
                && Pattern.compile("(?m)^\\s*\\|\\s*:?-{3,}:?\\s*(\\|\\s*:?-{3,}:?\\s*)+\\|?\\s*$").matcher(text).find();
    }

    public int findTableStart(String markdown) {
        if (markdown == null || markdown.isEmpty()) {
            return -1;
        }
        String[] lines = markdown.split("\\n", -1);
        int offset = 0;
        for (int i = 0; i < lines.length; i++) {
            if (isTableStart(lines, i)) {
                return offset;
            }
            offset += lines[i].length();
            if (i < lines.length - 1) {
                offset++;
            }
        }
        return -1;
    }

    public List<Part> splitParts(String markdown) {
        ArrayList<Part> parts = new ArrayList<>();
        if (markdown == null || markdown.isEmpty()) {
            return parts;
        }
        String[] lines = markdown.split("\\n", -1);
        StringBuilder text = new StringBuilder();
        int i = 0;
        while (i < lines.length) {
            if (isTableStart(lines, i)) {
                if (text.length() > 0) {
                    parts.add(new Part(false, text.toString().trim()));
                    text.setLength(0);
                }
                StringBuilder table = new StringBuilder();
                while (i < lines.length && isTableLine(lines[i])) {
                    if (table.length() > 0) {
                        table.append('\n');
                    }
                    table.append(lines[i]);
                    i++;
                }
                parts.add(new Part(true, table.toString()));
                continue;
            }
            text.append(lines[i]);
            if (i < lines.length - 1) {
                text.append('\n');
            }
            i++;
        }
        if (text.length() > 0) {
            parts.add(new Part(false, text.toString().trim()));
        }
        return parts;
    }

    private View tableView(String markdown) {
        LinearLayout table = new LinearLayout(context);
        table.setOrientation(LinearLayout.VERTICAL);
        table.setMinimumWidth((int) (context.getResources().getDisplayMetrics().widthPixels * 1.05f));
        String[] lines = markdown == null ? new String[0] : markdown.split("\\n");
        boolean headerRendered = false;
        int bodyRow = 0;
        for (String line : lines) {
            if (isSeparatorLine(line)) {
                continue;
            }
            List<String> cells = tableCells(line);
            if (cells.isEmpty()) {
                continue;
            }
            LinearLayout row = new EqualHeightTableRow(context);
            row.setOrientation(LinearLayout.HORIZONTAL);
            boolean header = !headerRendered;
            row.setMinimumHeight(header ? dp(48) : dp(54));
            for (int i = 0; i < cells.size(); i++) {
                TextView cell = new TextView(context);
                MarkdownRenderer.setMarkdown(cell, sanitize(cells.get(i)), 13);
                cell.setTextColor(header ? Color.rgb(17, 24, 39) : Color.rgb(55, 65, 81));
                cell.setTypeface(Typeface.DEFAULT, header ? Typeface.BOLD : Typeface.NORMAL);
                cell.setPadding(dp(12), dp(10), dp(12), dp(12));
                cell.setGravity(Gravity.CENTER_VERTICAL);
                cell.setBackground(tableCellBackground(header, bodyRow % 2 == 1));
                row.addView(cell, new LinearLayout.LayoutParams(i == 0 ? dp(142) : dp(128), ViewGroup.LayoutParams.MATCH_PARENT));
            }
            table.addView(row, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));
            if (headerRendered) {
                bodyRow++;
            }
            headerRendered = true;
        }
        if (table.getChildCount() == 0) {
            return markdownTextView(markdown);
        }
        return table;
    }

    private List<String> tableCells(String line) {
        ArrayList<String> cells = new ArrayList<>();
        if (line == null) {
            return cells;
        }
        String trimmed = line.trim();
        if (trimmed.startsWith("|")) {
            trimmed = trimmed.substring(1);
        }
        if (trimmed.endsWith("|")) {
            trimmed = trimmed.substring(0, trimmed.length() - 1);
        }
        String[] parts = trimmed.split("\\|", -1);
        for (String part : parts) {
            cells.add(part.trim());
        }
        return cells;
    }

    private GradientDrawable tableCellBackground(boolean header, boolean alternate) {
        GradientDrawable drawable = new GradientDrawable();
        drawable.setColor(header ? Color.rgb(238, 242, 247) : (alternate ? Color.rgb(250, 251, 252) : Color.WHITE));
        drawable.setStroke(1, Color.rgb(229, 231, 235));
        return drawable;
    }

    private boolean isTableStart(String[] lines, int index) {
        return index + 1 < lines.length && isTableLine(lines[index]) && isSeparatorLine(lines[index + 1]);
    }

    private boolean isTableLine(String line) {
        return line != null && line.trim().startsWith("|") && line.trim().contains("|");
    }

    private boolean isSeparatorLine(String line) {
        return line != null && Pattern.compile("^\\s*\\|\\s*:?-{3,}:?\\s*(\\|\\s*:?-{3,}:?\\s*)+\\|?\\s*$").matcher(line).find();
    }

    private String sanitize(String markdown) {
        return sanitizer == null ? (markdown == null ? "" : markdown) : sanitizer.visibleMarkdown(markdown);
    }

    private void bindCopy(View view, String text) {
        if (copyBinder != null) {
            copyBinder.bind(view, text);
        }
    }

    private int dp(int value) {
        return ShopUi.dp(context, value);
    }

    private GradientDrawable rounded(int color, int radius) {
        return ShopUi.rounded(color, radius);
    }

    private static class EqualHeightTableRow extends LinearLayout {
        EqualHeightTableRow(Context context) {
            super(context);
        }

        @Override
        protected void onMeasure(int widthMeasureSpec, int heightMeasureSpec) {
            super.onMeasure(widthMeasureSpec, heightMeasureSpec);
            int maxHeight = getSuggestedMinimumHeight();
            for (int i = 0; i < getChildCount(); i++) {
                View child = getChildAt(i);
                if (child.getVisibility() != GONE) {
                    maxHeight = Math.max(maxHeight, child.getMeasuredHeight());
                }
            }
            if (maxHeight <= 0) {
                return;
            }
            for (int i = 0; i < getChildCount(); i++) {
                View child = getChildAt(i);
                if (child.getVisibility() != GONE) {
                    int childWidth = child.getMeasuredWidth();
                    child.measure(
                            View.MeasureSpec.makeMeasureSpec(childWidth, View.MeasureSpec.EXACTLY),
                            View.MeasureSpec.makeMeasureSpec(maxHeight, View.MeasureSpec.EXACTLY)
                    );
                }
            }
            setMeasuredDimension(getMeasuredWidth(), maxHeight);
        }
    }
}
