package com.xzxg.shop.tts;

public final class TtsTextSanitizer {
    private TtsTextSanitizer() {
    }

    public static String sanitize(String markdown) {
        if (markdown == null) {
            return "";
        }
        String text = markdown;
        text = text.replaceAll("(?s)```.*?```", " ");
        text = text.replaceAll("(?m)^\\s*#{1,6}\\s*", "");
        text = text.replaceAll("(?m)^\\s*>\\s?", "");
        text = text.replaceAll("(?m)^\\s*[-*+]\\s+", "");
        text = text.replaceAll("(?m)^\\s*\\d+[.)]\\s+", "");
        text = text.replaceAll("\\[([^\\]]+)]\\(([^)]+)\\)", "$1");
        text = text.replaceAll("!\\[([^\\]]*)]\\(([^)]+)\\)", " ");
        text = text.replace("**", "");
        text = text.replace("__", "");
        text = text.replace("*", "");
        text = text.replace("_", "");
        text = text.replace("`", "");
        text = text.replace("|", " ");
        text = text.replaceAll("(?m)^\\s*:?-{3,}:?\\s*(\\s+:?-{3,}:?\\s*)*$", " ");
        text = text.replaceAll("<[^>]+>", " ");
        text = text.replaceAll("[\\t\\x0B\\f\\r]+", " ");
        text = text.replaceAll("\\n{3,}", "\n\n");
        text = text.replaceAll("[ ]{2,}", " ");
        return text.trim();
    }
}
