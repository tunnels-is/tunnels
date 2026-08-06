import React from "react"
import { createRoot } from "react-dom/client"
import "@fontsource-variable/inter"
import "./app.css"
import App from "./App.jsx"
import { initZoom } from "./lib/zoom"

initZoom()

createRoot(document.getElementById("app")).render(
	<React.StrictMode>
		<App />
	</React.StrictMode>,
)
