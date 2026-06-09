package com.xzxg.shop.ui;

import java.text.ParseException;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

public final class DateTimeFormatter {
    private DateTimeFormatter() {}

    public static String formatOrderTime(String raw) {
        if (raw == null) {
            return "";
        }
        String value = raw.trim();
        if (isMissingOrderTime(value)) {
            return "";
        }
        String[] patterns = new String[]{
                "yyyy-MM-dd'T'HH:mm:ssXXX",
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                "yyyy-MM-dd'T'HH:mm:ssX",
                "yyyy-MM-dd'T'HH:mm:ss.SSSX",
                "yyyy-MM-dd HH:mm:ss"
        };
        for (String pattern : patterns) {
            try {
                Date parsed = new SimpleDateFormat(pattern, Locale.US).parse(value);
                if (parsed != null) {
                    return new SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(parsed);
                }
            } catch (ParseException ignored) {
            }
        }
        String fallback = fallbackFormat(value);
        return isMissingOrderTime(fallback) ? "" : fallback;
    }

    private static boolean isMissingOrderTime(String value) {
        if (value == null) {
            return true;
        }
        String normalized = value.trim();
        if (normalized.isEmpty() || "null".equalsIgnoreCase(normalized)) {
            return true;
        }
        return normalized.startsWith("0001-01-01") || normalized.startsWith("0000-00-00");
    }

    private static String fallbackFormat(String value) {
        String cleaned = value.replace('T', ' ');
        int zoneIndex = cleaned.indexOf('+', 10);
        if (zoneIndex < 0) {
            zoneIndex = cleaned.indexOf('Z', 10);
        }
        if (zoneIndex < 0) {
            int minusIndex = cleaned.indexOf('-', 10);
            if (minusIndex > 0) {
                zoneIndex = minusIndex;
            }
        }
        if (zoneIndex > 0) {
            cleaned = cleaned.substring(0, zoneIndex);
        }
        return cleaned.length() >= 19 ? cleaned.substring(0, 19) : cleaned;
    }
}
