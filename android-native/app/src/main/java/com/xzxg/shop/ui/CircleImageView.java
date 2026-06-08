package com.xzxg.shop.ui;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Path;
import android.widget.ImageView;

public class CircleImageView extends ImageView {
    private final Path clipPath = new Path();

    public CircleImageView(Context context) {
        super(context);
    }

    @Override
    protected void onSizeChanged(int width, int height, int oldWidth, int oldHeight) {
        super.onSizeChanged(width, height, oldWidth, oldHeight);
        clipPath.reset();
        clipPath.addCircle(width / 2f, height / 2f, Math.min(width, height) / 2f, Path.Direction.CW);
    }

    @Override
    protected void onDraw(Canvas canvas) {
        int save = canvas.save();
        canvas.clipPath(clipPath);
        super.onDraw(canvas);
        canvas.restoreToCount(save);
    }
}
