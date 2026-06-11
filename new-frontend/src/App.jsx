import React, { useEffect } from "react"
import { HashRouter, Route, Routes } from "react-router-dom"
import "@/lib/theme"

import ConfirmDialog from "@/components/ConfirmDialog"
import LoadingBar from "@/components/LoadingBar"
import Sidebar from "@/components/Sidebar"
import Toasts from "@/components/Toasts"

import Account from "@/pages/Account"
import AccountSelect from "@/pages/AccountSelect"
import Bandwidth from "@/pages/Bandwidth"
import Connections from "@/pages/Connections"
import Devices from "@/pages/Devices"
import DNS from "@/pages/DNS"
import DNSStats from "@/pages/DNSStats"
import Login from "@/pages/Login"
import Logs from "@/pages/Logs"
import Settings from "@/pages/Settings"
import Support from "@/pages/Support"
import Tunnels from "@/pages/Tunnels"
import TwoFactor from "@/pages/TwoFactor"

import { connectLogSocket } from "@/api/logs"
import { fetchState } from "@/store/actions"
import { session } from "@/store/session"
import { useStore } from "@/store/store"

const App = () => {
	const user = useStore((s) => s.user)

	useEffect(() => {
		fetchState()
		connectLogSocket()
	}, [])

	return (
		<HashRouter>
			<div className="min-h-screen w-full bg-base-200">
				<LoadingBar />
				<Toasts />
				<Sidebar />

				<main className="min-h-screen">
					<Routes>
						{!user ? (
							<>
								<Route path="/" element={<Login />} />
								<Route path="*" element={<AccountSelect />} />
							</>
						) : (
							<>
								<Route path="/" element={<Tunnels />} />
								<Route path="*" element={<Tunnels />} />
								<Route path="tunnels" element={<Tunnels />} />
								<Route path="connections" element={<Connections />} />
								<Route path="bandwidth" element={<Bandwidth />} />
								<Route path="account" element={<Account />} />
								<Route path="devices" element={<Devices />} />
							</>
						)}

						<Route path="accounts" element={<AccountSelect />} />
						<Route path="twofactor/create" element={<TwoFactor />} />
						<Route path="logs" element={<Logs />} />
						<Route path="settings" element={<Settings />} />
						<Route path="dns" element={<DNS />} />
						<Route path="dnsstats" element={<DNSStats />} />
						<Route path="login" element={<Login />} />
						<Route path="login/:modeParam" element={<Login />} />
						<Route path="help" element={<Support />} />
					</Routes>
				</main>

				<ConfirmDialog />
			</div>
		</HashRouter>
	)
}

class ErrorBoundary extends React.Component {
	state = { hasError: false }

	static getDerivedStateFromError() {
		return { hasError: true }
	}

	reset() {
		session.clear()
		window.location.reload()
	}

	render() {
		if (this.state.hasError) {
			return (
				<div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-base-200 p-8 text-center">
					<h1 className="max-w-lg text-sm opacity-70">
						Something unexpected happened, please press Reload. If that doesn&apos;t work, please contact
						customer support.
					</h1>
					<button className="btn btn-primary btn-sm" onClick={() => this.reset()}>
						Reload
					</button>
				</div>
			)
		}
		return this.props.children
	}
}

const Root = () => (
	<ErrorBoundary>
		<App />
	</ErrorBoundary>
)

export default Root
