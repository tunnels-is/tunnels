import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import FormKeyValue from "./component/formkeyvalue";
import KeyValue from "./component/keyvalue";
import Label from "./component/label";
import CustomToggle from "./component/CustomToggle";
import FormKeyInput from "./component/formkeyrawvalue";
import STORE from "../store";

const Settings = () => {
	const state = GLOBAL_STATE("settings")

	let DebugLogging = state.getKey("Config", "DebugLogging")
	let ErrorLogging = state.getKey("Config", "ErrorLogging")
	let ConnectionTracer = state.getKey("Config", "ConnectionTracer")
	let InfoLogging = state.getKey("Config", "InfoLogging")

	let DarkMode = state.getKey("Config", "DarkMode")
	let AutoTLS = state.getKey("Config", "APIAutoTLS")

	let APICertDomains = state.getKey("Config", "APICertDomains")
	let APICertIPs = state.getKey("Config", "APICertIPs")
	let APICert = state.getKey("Config", "APICert")
	let APIKey = state.getKey("Config", "APIKey")
	let APIIP = state.getKey("Config", "APIIP")
	let APIPort = state.getKey("Config", "APIPort")

	let DialTimeout = state.getKey("Config", "RouterDialTimeoutSeconds")

	useEffect(() => {
		state.GetBackendState()
	}, [])

	const popEditor = (data) => {
		state.editorData = data
		state.editorReadOnly = true
		state.globalRerender()
	}

	const openConfigEditor = (config) => {
		state.editorData = config
		state.editorReadOnly = false
		state.editorDelete = undefined
		state.editorSave = function() {
			console.dir(state.modifiedConfig)
			state.ConfigSave()
			state.globalRerender()
		}
		state.editorOnChange = function(data) {
			state.editorError = ""
			let x = undefined
			try {
				x = JSON.parse(data)
			} catch (error) {
				console.log(error.message)
				console.dir(error)
				if (error.message) {
					state.editorError = error.message
				} else {
					state.editorError = "Invalid JSON"
				}
				return
			}

			console.dir(x)
			state.modifiedConfig = x
			state.ConfigSaveModifiedSate()
			state.globalRerender()
		}
		state.globalRerender()
	}

	let basePath = state.State?.BasePath
	let logPath = ""
	let tracePath = ""
	let logFileName = state.State?.LogFileName?.replace(state.State?.BasePath, "")
	let traceFileName = state.State?.TraceFileName?.replace(state.State?.BasePath, "")
	let configPath = state.State?.ConfigPath?.replace(state.State?.BasePath, "")
	if (state.State?.LogPath !== basePath) {
		logPath = state.State?.LogPath
	}
	if (state.State?.TracePath !== basePath) {
		tracePath = state.State?.TracePath
	}
	let version = state.getKey("State", "Version")

	return (
		<div className="settings-wrapper">
			<div className="state panel">
				<div className="title"
					onClick={() => popEditor(state?.State)}>
					Application State
				</div>

				<KeyValue label="Version" value={version} />
				<KeyValue label="Base Path" value={state.State?.BasePath} />
				<KeyValue label="Log Path" value={logPath} />
				<KeyValue label="Log File" value={logFileName} />
				<KeyValue label="Config File" value={configPath} />
				<KeyValue label="Trace Path" value={tracePath} />
				<KeyValue label="Trace File" value={traceFileName} />

				<div className="label-wrapper">
					<Label
						className={state.State?.IsAdmin ? "ok" : "warn"}
						value={state.State?.IsAdmin ? "Tunnels is running as admin" : "Tunnels Needs to run as administrator"}
					/>

					<Label
						className={state.State?.BaseFolderInitialized ? "ok" : "warn"}
						value={state.State?.BaseFolderInitialized ? "Base directory present" : "Base directory missing"}
					/>
				</div>

			</div>

			<div className="advanced panel">
				<div className="title" onClick={() => {
					openConfigEditor(state.Config)
				}}>Advanced Settings</div>

				<FormKeyValue label={"Timeouts"} value={
					<input value={DialTimeout} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"RouterDialTimeoutSeconds",
							Number(e.target.value))

						state.renderPage("settings")
					}} type="number" />}
				/>

				<FormKeyValue label={"API IP"} value={
					<input value={APIIP} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"APIIP",
							e.target.value)

						state.renderPage("settings")
					}} type="text" />}
				/>

				<FormKeyValue label={"API Port"} value={
					<input value={APIPort} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"APIPort",
							e.target.value)

						state.renderPage("settings")
					}} type="text" />}
				/>


				<FormKeyValue label={"API Cert"} value={
					<input value={APICert} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"APICert",
							e.target.value)

						state.renderPage("settings")
					}} type="text" />}
				/>

				<FormKeyValue label={"API Key"} value={
					<input value={APIKey} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"APIKey",
							e.target.value)
						state.renderPage("settings")
					}} type="text" />}
				/>

				<FormKeyInput
					label={"API Cert IPs"}
					type="text"
					value={APICertIPs}
					onChange={(e) => {

						state.setArrayAndReloadDom(
							"Config",
							"APICertIPs",
							e.target.value)
						state.renderPage("settings")
					}}
				/>

				<FormKeyInput
					label={"API Cert Domains"}
					type="text"
					value={APICertDomains}
					onChange={(e) => {

						state.setArrayAndReloadDom(
							"Config",
							"APICertDomains",
							e.target.value)
						state.renderPage("settings")
					}}
				/>


			</div>

			< div className="general panel">
				<div className="title">General Settings</div>

				<CustomToggle
					label={"Dark Mode"}
					value={DarkMode}
					toggle={() => {
						state.toggleDarkMode()
						state.globalRerender()
					}}
				/>

				<CustomToggle
					label={"Auto TLS"}
					value={AutoTLS}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "APIAutoTLS")
						state.renderPage("settings")
					}}
				/>

				<CustomToggle
					label="Basic Logging"
					value={InfoLogging}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "InfoLogging")
						state.renderPage("settings")
					}}
				/>

				<CustomToggle
					label="Error Logging"
					value={ErrorLogging}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "ErrorLogging")
						state.renderPage("settings")
					}}
				/>

				<CustomToggle
					label="Debug Logging"
					value={DebugLogging}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "DebugLogging")
						state.renderPage("settings")
					}}
				/>

			</div>

			{/* The classname of this div should probably change to advanced but it have functions and 
			styling that also need to be changed. And then then above div should be renamed to just general*/}


			<div className="debugging panel">
				<div className="title">Debugging</div>

				<CustomToggle
					label={"Tracing"}
					value={ConnectionTracer}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "ConnectionTracer")
						state.renderPage("settings")
					}}
				/>



				<CustomToggle
					label="Debug Mode"
					value={state?.debug}
					toggle={() => {
						state.toggleDebug()
						state.renderPage("settings")
					}}
				/>

				<div
					className="danger-button button"
					onClick={() => state.resetApp()}
				>
					Reset Everything
				</div>

			</div>
		</div>
	)
}

export default Settings;
