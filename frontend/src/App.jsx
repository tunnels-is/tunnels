import { HashRouter, Route, Routes } from "react-router-dom";
import { createRoot } from "react-dom/client";
import React, { useEffect } from "react";

import { Toaster } from 'react-hot-toast';

import "./assets/style/app.scss";
import '@fontsource-variable/inter';

import InspectBlocklist from "./App/InspectBlocklist";
import InspectConnection from "./App/InspectConnection";
import InspectJSON from "./App/component/InspectJSON";
import ConnectionTable from "./App/ConnectionTable";
import DNSAnswers from "./App/component/DNSAnswers";
import PrivateServers from "./App/PrivateServers";
import ObjectEditor from "./App/ObjectEditor";
import ScreenLoader from "./App/ScreenLoader";
import InspectGroup from "./App/InspectGroup";
import Enable2FA from "./App/Enable2FA";
import Settings from "./App/Settings";
import Account from "./App/Account";
import Servers from "./App/Servers";
import Welcome from "./App/Welcome";
import SideBar from "./App/SideBar";
import Login from "./App/Login";
import Org from "./App/Org";
import DNS from "./App/dns";

import GLOBAL_STATE from "./state";
import { STATE } from "./state";
import STORE from "./store";
import WS from "./ws";

// Use this to automatically turn on debug 
STORE.Cache.Set("debug", true)

const appElement = document.getElementById('app')
const root = createRoot(appElement);

const LaunchApp = () => {
	const state = GLOBAL_STATE("root")

	let configModified = state.modifiedLists !== undefined
	if (!configModified) {
		configModified = state.modifiedConfig !== undefined
	}
	if (!configModified) {
		configModified = state.IsConfigModified()
	}


	if (state.getDarkMode()) {
		appElement.classList.remove("light")
		appElement.classList.add("dark")
		document.documentElement.classList.add("dark")
		document.documentElement.classList.remove("light")
	} else {
		appElement.classList.remove("dark")
		appElement.classList.add("light")
		document.documentElement.classList.add("light")
		document.documentElement.classList.remove("dark")
	}

	useEffect(() => {
		state.GetBackendState()
		WS.NewSocket(WS.GetURL("logs"), "logs", WS.ReceiveLogEvent)
	}, [])

	return (
		< HashRouter >

			<Toaster
				toastOptions={{
					className: 'toast',
					position: "top-right",
					success: {
						duration: 5000,
					},
					icon: null,
					error: {
						duration: 5000,
					},
				}}
			/>

			<SideBar />

			<ScreenLoader />
			<div className="ab app-wrapper">

				{configModified &&
					<div className="save-bar">
						<div className="text">Your config has unsaved changes</div>
						<div className="button"
							onClick={() => state.ConfigSave()}>
							SAVE
						</div>
						<div className="cancel-button button"
							onClick={() => state.RemoveModifiedConfig()}>
							CANCEL
						</div>
					</div>
				}

				{state.editorData && <InspectJSON />}

				<Routes>

					{(state.User?.Email === "" || !state.User) &&
						<>
							<Route path="/" element={<Login />} />
							<Route path="login" element={<Login />} />
							<Route path="settings" element={<Settings />} />
							<Route path="help" element={<Welcome />} />
							<Route path="dns" element={<DNS />} />
							<Route path="*" element={<Login />} />
						</>
					}

					{state.User &&
						<>
							<Route path="/" element={<Welcome />} />
							<Route path="account" element={<Account />} />
							<Route path="settings" element={<Settings />} />

							<Route path="twofactor" element={<Enable2FA />} />
							<Route path="org" element={<Org />} />

							<Route path="inspect/group/:id" element={<InspectGroup />} />
							<Route path="inspect/group" element={<InspectGroup />} />

							<Route path="tunnels" element={<ConnectionTable />} />
							<Route path="inspect/connection/:id" element={<InspectConnection />} />
							<Route path="routing" element={<ConnectionTable />} />

							<Route path="dns" element={<DNS />} />
							<Route path="dns/answers/:domain" element={<DNSAnswers />} />

							<Route path="servers" element={<Servers />} />
							<Route path="private" element={<PrivateServers />} />

							<Route path="inspect/blocklist/" element={<InspectBlocklist />} />

							<Route path="login" element={<Login />} />
							<Route path="help" element={<Welcome />} />
							<Route path="test" element={<ObjectEditor />} />

							<Route path="*" element={<Servers />} />
						</>
					}
				</Routes>
			</div>
		</HashRouter >
	)
}


class ErrorBoundary extends React.Component {
	constructor(props) {
		super(props);
		this.state = {
			hasError: false,
			title: "Something unexpected happened, please press Reload. If that doesn't work try pressing 'Close And Reset'. If nothing works, please contact customer support"
		};
	}

	static getDerivedStateFromError() {
		return { hasError: true };
	}

	componentDidCatch() {
		this.state.hasError = true
	}

	reloadAll() {
		STORE.Cache.Clear()
		window.location.reload()
	}

	async quit() {
		this.setState({ ...this.state, title: "closing app, please wait.." })
		window.location.reload()
		STORE.Cache.Clear()
	}

	async ProductionCheck() {

		if (!STATE.debug) {
			window.console.apply = function() { }
			window.console.dir = function() { }
			window.console.log = function() { }
			window.console.info = function() { }
			window.console.warn = function() { }
			window.console.error = function() { }
			window.console.debug = function() { }
		}

	}

	render() {
		this.ProductionCheck()

		if (this.state.hasError) {
			return (<>
				<h1 className="exception-title">
					{this.state.title}
				</h1>
				<button className="exception-button" onClick={() => this.reloadAll()}>Reload</button>
				<button className="exception2-button" onClick={() => this.quit()}>Close And Reset</button>
			</>)
		}

		return this.props.children;
	}
}


root.render(<React.StrictMode>
	<ErrorBoundary>
		<LaunchApp />
	</ErrorBoundary>
</React.StrictMode>)
