# Fyne app icons / packaging notes

- `../appicon.png` — 512×512 window / Linux / macOS source icon.
- `../winres.json` + `../rsrc_windows_*.syso` — PE icon linked into the Windows `.exe` (Explorer + taskbar).
- `tunnels.desktop` — Linux launcher. Copy with `tunnels.png` into the icon/applications dirs, or leave them next to the binary.

## macOS admin launch

Fyne’s `fyne package` / build tooling does **not** support privilege escalation.
`./build-fyne.sh darwin` uses `scripts/package-darwin-app.sh`, which wraps the Fyne
binary in `Tunnels.app` with a **sudo askpass** launcher (not osascript
`with administrator privileges`, which freezes the Fyne UI / breaks pasteboard):

- `Contents/MacOS/tunnels-app` — CFBundleExecutable → `exec sudo -A …`
- `Contents/MacOS/tunnels-askpass` — GUI password dialog for `sudo -A`
- `Contents/MacOS/tunnels-app.bin` — real Fyne binary

Double-click → sudo password dialog → app runs as root (needed for DNS :53 / network),
while staying attached to the user GUI session so the window stays responsive.

Regenerate Windows resources from the repo root:

```bash
go install github.com/tc-hib/go-winres@latest
(cd cmd/fyne && go-winres make --in winres.json --arch amd64,arm64 --out rsrc)
```
