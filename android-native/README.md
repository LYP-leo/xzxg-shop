# Android Native Client

This is a native Android/Kotlin client for the xzxg-shop project. It is separate from the existing React/Capacitor frontend, which remains under `frontend/`.

Current scope:

- User login.
- Create Agent session.
- List historical Agent sessions.
- Open a historical session and show user messages with run status.
- Send a message to the Agent and render `text_delta` SSE output in real time.

Local backend URL:

```text
http://10.0.2.2:8080/api/v1
```

`10.0.2.2` is the Android emulator address for the host machine. Use a LAN IP if running on a physical device.

Start the backend on an IPv4 address before launching the emulator:

```bash
cd backend
API_ADDR=0.0.0.0:8080 go run ./cmd/api
```

Build from Android Studio by opening this `android-native/` directory, or run:

```bash
cd android-native
./gradlew assembleDebug
```

The existing `frontend/android/` directory is still the Capacitor shell and is intentionally left untouched.
