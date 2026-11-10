# OwnWave for Android

Native client: Kotlin, Jetpack Compose, Media3. Plan and decisions: [docs/redesign/ANDROID.md](../../docs/redesign/ANDROID.md).

Status: sign in and station list. Playback, Android Auto and signed releases come in the next PRs.

## Build

Needs JDK 21 and the Android SDK with platform 37 (`ANDROID_HOME` set, or `sdk.dir` in `local.properties`).

```sh
cd apps/android
./gradlew :app:assembleDebug          # app/build/outputs/apk/debug/app-debug.apk
./gradlew :app:testDebugUnitTest :app:lintDebug
```

CI runs the same build and uploads the debug APK as the `ownwave-debug-apk` artifact.

## Modules

- `api`: Kotlin client generated on every build from `services/go-api-server/openapi.yaml` (openapi-generator, Retrofit, kotlinx.serialization). Nothing generated is checked in. A spec change that breaks the app breaks the build.
- `app`: the Android app (`io.github.felixclements.ownwave`).

## Signing in

The app asks for the server address, a username and a password. It signs in with `POST /api/auth/device` and keeps the device token in DataStore, encrypted with a key in the Android Keystore. The token is sent only to that server. A 401 signs the app out. Revoke devices in the web app.

Release builds accept only `https://` servers. Debug builds also accept `http://`, for example a local dev server at `http://10.0.2.2:8080` from the emulator.

If a login gate (for example Authentik forward auth) sits in front of the server, it has to let `/api/*` requests with a Bearer token, and `POST /api/auth/device`, through. Otherwise the app shows "The server did not answer like OwnWave".
