# ChatWithRepo — Android client (Go + Gio)

A small, native Android frontend for the existing ChatWithRepo FastAPI
backend. This is **not** a wrapper around the HTML/CSS site — it's a
Gio (`gioui.org`) UI that talks to your API directly over HTTPS.

```
mobile/
├── main.go            # entry point + Gio event loop
├── go.mod
├── api/
│   ├── client.go       # HTTP client + JSON structs for every endpoint
│   └── storage.go       # local access-token persistence
├── ui/
│   ├── app.go           # screen router / shared app state
│   ├── components.go     # colors, buttons, text fields, chat bubbles
│   ├── login.go
│   ├── register.go
│   ├── dashboard.go
│   └── chat.go
└── README.md
```

## What it does

- **Login / Register** — calls `POST /auth/login` and `POST /auth/register`,
  stores the returned `access_token` on-device, and sends
  `Authorization: Bearer <token>` on every subsequent request.
- **Dashboard** — loads `GET /chat/list`, and creates new chats via
  `POST /chat/create` with `owner`, `repo`, and `branch` (defaults to
  `main`).
- **Chat** — loads `GET /chat/{id}/messages`, sends questions via
  `POST /chat/{id}/ask`, renders user/assistant bubbles with basic
  Markdown (fenced code blocks, bold), auto-scrolls to the newest
  message, and shows a loading state while waiting on an answer.
- Any `401` response anywhere in the app clears the stored session and
  drops the user back on the Login screen.

The backend URL lives in one place — the `baseURL` constant at the top
of `main.go` — so pointing this at a local dev server is a one-line change.

## Prerequisites

- Go 1.21+
- For Android builds: the `gogio` tool, a JDK (17 recommended), and the
  Android SDK + NDK. The easiest way to get the SDK/NDK is installing
  **Android Studio** once and letting it manage them; then point the
  env vars below at the SDK/NDK paths it created.

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
export ANDROID_NDK_HOME="$ANDROID_HOME/ndk/<installed-version>"
```

## 1. Install dependencies

```bash
cd mobile
go mod tidy
go install gioui.org/cmd/gogio@latest
```

> `go.mod` pins `gioui.org v0.7.1`. If `go mod tidy` reports that version
> doesn't exist by the time you run this, open
> https://pkg.go.dev/gioui.org?tab=versions, put the latest version in
> `go.mod`'s `require gioui.org vX.Y.Z` line, and re-run `go mod tidy` —
> the code in this repo only uses stable, long-standing Gio APIs
> (`app.Window`/`Option`/`Event`, `layout`, `widget`, `widget/material`),
> so it isn't tied to one exact patch release.

## 2. Run locally (desktop window, for fast iteration)

```bash
cd mobile
go run .
```

This opens a normal desktop window running the exact same UI code that
ships to Android — useful for iterating on layout without a
build/install cycle. To test against a local FastAPI instance instead
of the hosted one, temporarily change `baseURL` in `main.go` to
`http://127.0.0.1:8000` (or `http://10.0.2.2:8000` if you later run
against the Android *emulator* instead of a real device).

## 3. Build the Android APK

```bash
cd mobile
gogio -target android -appid com.chatwithrepo.mobile -o chatwithrepo.apk .
```

This produces `chatwithrepo.apk` in the current directory. `gogio`
cross-compiles the Go code for Android and packages it — no Java/Kotlin
code required.

## 4. Install the APK on a device

With a device connected over USB (developer mode + USB debugging
enabled) or a running emulator:

```bash
adb install -r chatwithrepo.apk
```

Then launch "ChatWithRepo" from the app drawer.

## Notes / limitations

- Markdown rendering is intentionally basic (fenced code blocks get a
  monospace block; `**bold**`/`__bold__` markers are stripped rather
  than rendered as mixed-weight inline text) — enough to read model
  answers clearly without pulling in a full Markdown engine.
- The access token is stored in a small file under the app's private
  storage directory (`os.UserConfigDir()`), not a database — matching
  the "no database" constraint.
- No payments, profile, themes, or analytics screens are included, by
  design — this client only implements the auth/chat flow described
  above.
