import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import path from "path"

// Content-Security-Policy injected into the built index.html only. The dev
// server needs a looser policy (HMR uses eval + a websocket), so we skip
// injection when serving. connect-src is same-origin plus the loopback daemon
// used by `vite dev` (port rewrite to 7777). Wails loads the UI from the API
// origin itself, so 'self' covers it. Control-server traffic is always
// proxied through the local daemon.
const CSP = [
	"default-src 'self'",
	"connect-src 'self' http://127.0.0.1:7777 ws://127.0.0.1:7777",
	"img-src 'self' data:",
	"font-src 'self' data:",
	"style-src 'self' 'unsafe-inline'",
	"script-src 'self'",
	"object-src 'none'",
	"base-uri 'none'",
	"frame-ancestors 'none'",
].join("; ")

const cspPlugin = () => ({
	name: "inject-csp",
	transformIndexHtml: {
		order: "post",
		handler(html, ctx) {
			if (ctx.server) return html // dev server: leave HMR alone
			return html.replace(
				"</head>",
				`  <meta http-equiv="Content-Security-Policy" content="${CSP}">\n</head>`,
			)
		},
	},
})

export default defineConfig({
	plugins: [react(), tailwindcss(), cspPlugin()],
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
})
