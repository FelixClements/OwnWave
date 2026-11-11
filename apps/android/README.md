# OwnWave for Android

Native client: Kotlin, Jetpack Compose, Media3. Plan and decisions: [docs/redesign/ANDROID.md](../../docs/redesign/ANDROID.md).

Status: sign in, stations, background playback with like and ban, Android Auto, signed-in devices, and signed releases.

## Install

1. On the phone, open the repository's [Releases](https://github.com/FelixClements/OwnWave/releases) page and download the newest `ownwave-<version>.apk` from a release tagged `android-v…`.
2. Open the APK. Android asks to allow installs from your browser or file manager the first time: allow it, then go back and install.
3. Start OwnWave and sign in with your server address (`https://…`), username and password.
4. To update, install a newer APK over the old one. You stay signed in. Every release is signed with the same key, so Android refuses an APK signed with any other key, including a debug build: uninstall first to switch (this signs you out).

Needs Android 9 or newer.

### Android Auto

Auto hides apps that were not installed from the Play Store until you allow them:

1. In the Android Auto settings on the phone, tap the version number about ten times to turn on developer mode.
2. In the menu (three dots), open **Developer settings** and turn on **Unknown sources**.
3. In Auto's **Customize launcher**, turn on OwnWave if it is off.

Some cars or head units block sideloaded apps anyway. Play Store distribution is not planned yet (see [ANDROID.md](../../docs/redesign/ANDROID.md#risks-and-open-questions)).

## Build

Needs JDK 21 and the Android SDK with platform 37 (`ANDROID_HOME` set, or `sdk.dir` in `local.properties`).

```sh
cd apps/android
./gradlew :app:assembleDebug          # app/build/outputs/apk/debug/app-debug.apk
./gradlew :app:testDebugUnitTest :app:lintDebug
```

CI runs the same build and uploads the debug APK as the `ownwave-debug-apk` artifact.

## Release

Push a tag `android-v<major>.<minor>.<patch>` (for example `android-v0.2.0`) on `main`. The `Android release` workflow (`.github/workflows/android-release.yml`) runs the unit tests, builds `assembleRelease` with R8, signs it, and attaches `ownwave-<version>.apk` and `SHA256SUMS` to a GitHub Release for that tag. `versionName` comes from the tag; `versionCode` is the workflow's run number.

Signing reads these repository secrets. The keystore never goes into the repo:

| Secret | Value |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | The release keystore (`.jks`), base64 encoded |
| `ANDROID_KEYSTORE_PASSWORD` | Keystore password |
| `ANDROID_KEY_ALIAS` | Key alias |
| `ANDROID_KEY_PASSWORD` | Key password |

Keep a backup of the keystore and its passwords outside GitHub. Losing it means every phone has to uninstall and reinstall to get updates.

To build a signed APK locally, set `OWNWAVE_KEYSTORE_FILE`, `OWNWAVE_KEYSTORE_PASSWORD`, `OWNWAVE_KEY_ALIAS` and `OWNWAVE_KEY_PASSWORD`, then run `./gradlew :app:assembleRelease -Pownwave.versionName=0.2.0 -Pownwave.versionCode=2`. Without `OWNWAVE_KEYSTORE_FILE` the release APK is unsigned.

## Modules

- `api`: Kotlin client generated on every build from `services/go-api-server/openapi.yaml` (openapi-generator, Retrofit, kotlinx.serialization). Nothing generated is checked in. A spec change that breaks the app breaks the build.
- `app`: the Android app (`io.github.felixclements.ownwave`).

## Signing in

The app asks for the server address, a username and a password. It signs in with `POST /api/auth/device` and keeps the device token in DataStore, encrypted with a key in the Android Keystore. The token is sent only to that server. A 401 signs the app out. **Settings** lists the account's signed-in devices and can sign the others out; the web app can too.

Release builds accept only `https://` servers. Debug builds also accept `http://`, for example a local dev server at `http://10.0.2.2:8080` from the emulator.

If a login gate (for example Authentik forward auth) sits in front of the server, it has to let `/api/*` requests with a Bearer token, and `POST /api/auth/device`, through. Otherwise the app shows "The server did not answer like OwnWave".
