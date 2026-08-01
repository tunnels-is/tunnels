# Tunnels frontend

React + Vite + Tailwind v4 + daisyUI SPA for the local tunnels client API.

## Run

```sh
pnpm install
pnpm dev      # expects the client running with -dev (CORS) on 127.0.0.1:7777
pnpm build    # outputs dist/ (embedded by the Go client)
```

## Code pattern

```
src/
  api/client.js     callMethod() / callController() — the only fetch code.
                    Both resolve to { status, data, networkError }, never throw.
  api/logs.js       /logs websocket -> store
  store/store.js    single zustand store: backend data + ui state (toasts,
                    confirm dialog, loading). Synchronous helpers only.
  store/actions.js  all async backend logic. Pages call actions for global
                    state; page-local data (devices, 2FA) uses api()/controller()
                    from here directly.
  store/session.js  sessionStorage wrappers (the only storage; no localStorage).
  lib/              theme (daisyUI data-theme), formatting, country names
  components/       shared daisyUI building blocks (Page, Card, Field, Toggle,
                    Dialog, Sidebar, Toasts, ConfirmDialog, LoadingBar)
  pages/            one file per route, plain useState for local form state
```

Rules:

- All styling is daisyUI/Tailwind utility classes; themes (`suzko`,
  `suzko-dark`) are defined in `src/app.css` and switched via `data-theme`.
- No component talks to `fetch` directly — always `api()`/`controller()`
  (which inject auth, dedupe in-flight controller calls, and toast errors).
- Config mutations build a new config object and call `saveConfig(next)`.
- Confirmations go through `useStore().askConfirm(title, subtitle, fn)`.
