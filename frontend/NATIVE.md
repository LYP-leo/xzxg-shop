# Native App Shell

This frontend uses Capacitor to package the existing React app into native iOS and Android projects.

## Commands

```bash
npm ci
npm run native:sync
```

Open native projects:

```bash
npm run native:ios
npm run native:android
```

Notes:

- `npm run native:sync` builds the Vite app and copies `dist/` into the native projects.
- iOS builds require Xcode on macOS.
- Android builds require Android Studio and a configured Android SDK.
- The current app falls back to mock data when the backend API is unavailable, so the shell can be previewed before the Go API is ready.
