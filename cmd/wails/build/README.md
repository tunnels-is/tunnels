# Desktop app icon assets

- `appicon.png` — 512×512 source icon (from `frontend/src/assets/logo.svg`).
- `../winres.json` — Windows resource definition. **Icon must be `#3`** so it matches Wails `AppIconID` (title bar + window icon).
- `../rsrc_windows_*.syso` — PE resources auto-linked by `go build` when targeting Windows (taskbar **and** title-bar icon).

## Regenerate

From the repo root (requires `rsvg-convert` and `go-winres`):

```bash
# Square PNG from the project logo (transparent background)
cat > /tmp/tunnels-icon.svg << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512">
  <g transform="translate(56, 20) scale(3.26)">
    <g transform="translate(-48.191612,-62.745798)">
      <rect style="fill:#ffffff" width="29.845591" height="85.533081" x="97.908081" y="120.1103" rx="7.442276" ry="4.5684595"/>
      <rect style="fill:#4f8cff" width="122.29413" height="30.209558" x="29.433624" y="91.815094" rx="10.354041" ry="9.5959768" transform="rotate(-11.071732)"/>
    </g>
  </g>
</svg>
EOF
rsvg-convert -w 512 -h 512 --background-color none /tmp/tunnels-icon.svg -o cmd/wails/build/appicon.png

cd cmd/wails
go install github.com/tc-hib/go-winres@latest
# Use winres.json so the icon is resource ID 3 (Wails title-bar expects AppIconID=3)
go-winres make --in winres.json --arch amd64,arm64 --out rsrc
```
