# Android Native v3

This is the Java native Android client for PRD v3.

Current scope:

- Chat-first customer home screen.
- Left drawer for profile, products, cart, orders, and local chat history.
- Debug-only backend address setting inside the profile page.
- Local chat/session persistence with SQLite.
- Login and authenticated SSE chat through the Go backend.

Build:

```bash
ANDROID_HOME=/opt/homebrew/share/android-commandlinetools gradle assembleDebug
```
