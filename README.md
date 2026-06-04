# AuraRPC

[ 🇺🇸 English ](README.md) | [ 🇪🇸 Español ](README-es.md)

A tiny, private Discord Rich Presence app that stays out of your way.

AuraRPC lets you set your own custom Discord status — the "Playing …" card on
your profile — without needing a game that supports it. It talks to your Discord
client directly on your machine, so there are no servers, no logins, and no
tokens involved.

Made by [Mokita Studio](https://github.com/Mokita-Studio). Free and open source.

> Looking for the internals? The technical docs live in [`docs/en/`](docs/en/INDEX.md).

---

## Why another one?

Most Rich Presence tools are heavy, cluttered, or both. AuraRPC takes the
opposite approach:

- **Light.** A single ~12 MB binary with nothing to install alongside it. It
  sits quietly in your tray and barely touches your RAM.
- **Private.** It never goes online — it only opens Discord's local pipe. No
  telemetry, no accounts, no tokens. Your Discord login is never read.
- **Simple.** Fill in a couple of fields, hit Connect, and you're done. Save as
  many presets as you like and switch between them straight from the tray.

---

## Features

- A clean editor for every Discord presence field: details, state, large and
  small images, buttons, timestamps, party size, and activity type.
- **Presets** — save different statuses and switch with a single click from the
  tray, without even opening the window.
- **Light and dark themes** that follow your Windows taskbar.
- **Runs in the background** — close the window and your status stays alive from
  the tray.
- **Optional update check** that simply tells you when a new version is out. It
  never installs anything behind your back.

---

## Getting started

1. Download `AuraRPC.exe` (or the installer) from the [Releases](../../releases) page.
2. Head to the [Discord Developer Portal](https://discord.com/developers/applications),
   create an application, and copy its **Application ID**. That app's name is what
   shows up as "Playing …" on your profile.
3. *(Optional)* Under *Rich Presence → Art Assets*, upload any images you want to
   use and note their **asset keys**.
4. Open AuraRPC, paste the Application ID, fill in your details, and hit **Connect**.
5. Hit **Save** to keep the preset in your sidebar for next time.

Closing the window doesn't quit the app — it keeps running in the tray.
**Left-click** the tray icon to reopen the window; **right-click** for quick
preset switching and disconnect.

---

## FAQ

**Do I need a bot or a token?**
No. AuraRPC only needs an Application ID, which is public. It never logs into
your account or asks for any token.

**Why do I need an Application ID at all?**
Discord ties every Rich Presence to an app. The app's name is what appears as
"Playing &lt;name&gt;" on your profile, and its uploaded images are what your status
can show. Creating one is free and takes about a minute in the Developer Portal.

**My status isn't showing up — what should I check?**
Make sure Discord is open, the Application ID is correct, and you clicked
Connect. Discord can take a few seconds to display a new presence. Also confirm
that *Activity Privacy → Display current activity as a status message* is enabled
in Discord's settings.

**My images don't appear.**
Images are referenced by their **asset key** — the name you gave them under
*Rich Presence → Art Assets* in the Developer Portal. Make sure the key matches
exactly; freshly uploaded assets can take a little while to become available.

**Windows warns me with SmartScreen when I open it.**
The binary isn't code-signed yet, so Windows shows a caution. Click
*More info → Run anyway*. You can always read or compile the source yourself —
it's all open.

**Does it use a lot of resources?**
No. It's one small binary that idles in the tray with minimal RAM and no
background network activity.

**Can I have more than one status?**
Yes — save as many presets as you want and switch between them instantly from
the tray menu.

**The tray icon doesn't respond to clicks.**
This usually happens when the icon is tucked into the hidden-icons overflow, or
when a taskbar-customizing tool interferes. Try pinning AuraRPC to the taskbar
(*Settings → Personalization → Taskbar → other system tray icons*). Either way,
you can always reopen the window by launching AuraRPC again from its shortcut —
it brings the running instance back to the front.

**Does it work on Linux or macOS?**
Windows for now. A Linux build is in the works.

---

## Tech specs

|                       |                                                  |
| --------------------- | ------------------------------------------------ |
| Language              | Go 1.23+                                          |
| UI                    | [Gio](https://gioui.org) (GPU-rendered)           |
| Binary size           | ~12 MB                                            |
| RAM                   | a few MB idle in the tray; more with the window open (varies by system) |
| Runtime dependencies  | none                                              |
| Platforms (v1)        | Windows 10 1809+ / Windows 11                     |
| Discord IPC           | local named pipe (`\\.\pipe\discord-ipc-{0..9}`)  |
| Telemetry / network   | none                                              |
| Languages             | English, Spanish                                  |
| Storage               | plain JSON in `%APPDATA%\AuraRPC\`                |

---

## Building from source

Requirements: **Go 1.23+** and **PowerShell 5.1+**. Optionally **Inno Setup 6**
(for the installer) and **MinGW-w64 / gcc** (to run tests with the `-race` flag).

```powershell
.\scripts\build.ps1      # builds AuraRPC.exe in the project root
.\scripts\package.ps1    # builds the installer in dist\
```

`build.ps1` regenerates the embedded `.syso` automatically whenever the icon or
resources change.

---

## License

MIT — see [LICENSE](LICENSE). Copyright © 2026 Mokita Studio.
