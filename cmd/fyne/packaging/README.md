# Fyne app icons

- `../appicon.png` — 512×512 window / Linux / macOS source icon.
- `../winres.json` + `../rsrc_windows_*.syso` — PE icon linked into the Windows `.exe` (Explorer + taskbar).
- `tunnels.desktop` — Linux launcher. Copy with `tunnels.png` into the icon/applications dirs, or leave them next to the binary.

Regenerate Windows resources from the repo root:

```bash
go install github.com/tc-hib/go-winres@latest
(cd cmd/fyne && go-winres make --in winres.json --arch amd64,arm64 --out rsrc)
```
