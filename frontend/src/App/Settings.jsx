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
		state.resetEditor()
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
				<KeyValue label="Log Path" value={logPath} />
				<KeyValue label="Config File" value={configPath} />
				<KeyValue label="Trace Path" value={tracePath} />
				<KeyValue label="Trace File" value={traceFileName} />
				<KeyValue label="Log File" value={logFileName} />
				<KeyValue label="Base Path" value={state.State?.BasePath} />

				<div className="label-wrapper">
					<Label
						className={state.State?.IsAdmin ? "ok" : "warn"}
						value={state.State?.IsAdmin ? "Tunnels has the correct permissions" : "Tunnels is missing permissions"}
					/>
				</div>

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

				<CustomToggle
					label="Debug Mode"
					value={state?.debug}
					toggle={() => {
						state.toggleDebug()
						state.renderPage("settings")
					}}
				/>

				<CustomToggle
					label={"Tracing"}
					value={ConnectionTracer}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "ConnectionTracer")
						state.renderPage("settings")
					}}
				/>



				<div
					className="red card-button"
					onClick={() => state.resetApp()}
				>
					Reset Everything
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


		</div>
	)
}

export default Settings;
