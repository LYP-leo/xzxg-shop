package com.xzxg.shop.ui;

import android.app.Activity;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Paint;
import android.graphics.Typeface;
import android.graphics.drawable.ColorDrawable;
import android.view.Gravity;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

public final class TopBarHelper {
    private TopBarHelper() {
    }

    public static View backBar(Activity activity, int backgroundColor, String titleText, Runnable onBack) {
        LinearLayout toolbar = new LinearLayout(activity);
        toolbar.setGravity(Gravity.CENTER_VERTICAL);
        toolbar.setPadding(ShopUi.dp(activity, 14), 0, ShopUi.dp(activity, 14), 0);
        toolbar.setBackgroundColor(backgroundColor);
        toolbar.setElevation(ShopUi.dp(activity, 1));

        FrameLayout back = new FrameLayout(activity);
        back.setClickable(true);
        back.setBackground(new ColorDrawable(Color.TRANSPARENT));
        back.setContentDescription("返回");
        back.setOnClickListener(v -> {
            if (onBack != null) {
                onBack.run();
            } else {
                activity.finish();
            }
        });
        back.addView(new BackChevronView(activity), new FrameLayout.LayoutParams(ShopUi.dp(activity, 22), ShopUi.dp(activity, 22), Gravity.CENTER));
        toolbar.addView(back, new LinearLayout.LayoutParams(ShopUi.dp(activity, 44), ShopUi.dp(activity, 44)));

        TextView title = new TextView(activity);
        title.setText(titleText == null ? "" : titleText);
        title.setTextSize(18);
        title.setTypeface(Typeface.DEFAULT_BOLD);
        title.setTextColor(Color.rgb(20, 24, 30));
        title.setGravity(Gravity.CENTER_VERTICAL);
        title.setIncludeFontPadding(false);
        LinearLayout.LayoutParams titleParams = new LinearLayout.LayoutParams(0, -1, 1);
        titleParams.leftMargin = ShopUi.dp(activity, 8);
        toolbar.addView(title, titleParams);

        toolbar.addView(new FrameLayout(activity), new LinearLayout.LayoutParams(ShopUi.dp(activity, 44), ShopUi.dp(activity, 44)));
        return toolbar;
    }

    private static final class BackChevronView extends View {
        private final Paint paint = new Paint(Paint.ANTI_ALIAS_FLAG);

        BackChevronView(Activity activity) {
            super(activity);
            paint.setColor(Color.rgb(17, 24, 39));
            paint.setStrokeWidth(ShopUi.dp(activity, 2));
            paint.setStrokeCap(Paint.Cap.ROUND);
            paint.setStrokeJoin(Paint.Join.ROUND);
        }

        @Override
        protected void onDraw(Canvas canvas) {
            super.onDraw(canvas);
            float w = getWidth();
            float h = getHeight();
            float cx = w * 0.58f;
            float top = h * 0.24f;
            float mid = h * 0.50f;
            float bottom = h * 0.76f;
            float left = w * 0.34f;
            canvas.drawLine(cx, top, left, mid, paint);
            canvas.drawLine(left, mid, cx, bottom, paint);
        }
    }
}
