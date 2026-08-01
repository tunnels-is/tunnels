import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import path from "path"

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
			if (ctx.server) return html
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
