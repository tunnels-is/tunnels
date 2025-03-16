import React, { useEffect, useRef } from "react";

import GLOBAL_STATE from "../../state";
import FormKeyValue from "./formkeyvalue";
import CustomToggle from "./CustomToggle";

const ConfigDNSRecordEditor = () => {
	const state = GLOBAL_STATE("DNSRecordForm")

	const addRecord = () => {
		if (!state.Config.DNSRecords) {
			state.Config.DNSRecords = []

		}
		state.Config?.DNSRecords.push({
			Domain: "domain.local",
			IP: [""],
			TXT: [""],
			CNAME: "",
			Wildcard: true,
		})
		state.renderPage("DNSRecordForm")
	}
	const saveAll = () => {
		state.Config.DNSRecords?.forEach((r,i) => {
			r.IP?.forEach((ip,ii) => {
				if (ip===""){
			state.Config.DNSRecords[i].IP.splice(ii, 1)
				}
			})
			r.TXT?.forEach((txt,ii) => {
				if (txt===""){
			state.Config.DNSRecords[i].TXT.splice(ii, 1)
				}
			})
		})
		state.ConfigSave()
		state.renderPage("DNSRecordForm")
	}
	const deleteRecord = (index) => {
		if (state.Config.DNSRecords.length === 1) {
			state.Config.DNSRecords = []
			state.v2_ConfigSave()
			state.globalRerender()
		} else {
			state.Config.DNSRecords.splice(index, 1)
			state.v2_ConfigSave()
			state.globalRerender()
		}

	}

	const updateRecord = (index, subindex, key, value) => {
		console.log("update:", index, subindex, key, value)
		if (key === "IP") {

			try {
				state.Config.DNSRecords[index].IP[subindex] = value
			} catch (error) {
				console.dir(error)
			}
		} else if (key === "TXT") {
			try {
				state.Config.DNSRecords[index].TXT[subindex] = value
			} catch (error) {
				console.dir(error)
			}

		} else if (key === "Wildcard") {
			state.Config.DNSRecords[index].Wildcard = value

		} else {
			state.Config.DNSRecords[index][key] = value
		}

		state.renderPage("DNSRecordForm")
	}

	const addIP = (index) => {
		state.Config?.DNSRecords[index].IP.push("0.0.0.0")
		state.renderPage("DNSRecordForm")
	}
	const addTXT = (index) => {
		state.Config?.DNSRecords[index].TXT.push("new text record")
		state.renderPage("DNSRecordForm")
	}

	const makeInput = (index, subindex, key, value, type) => {
		if (type === "textarea") {
			let rows = (value.length / 40) + 2
			return (
				<FormKeyValue label={key} key={key + subindex} value={
					<textarea
						cols={value.length }
						rows={rows}
						className="value"
						onChange={() => { console.log("on change!", value, index) }}
						type={type}
						value={value}
						onInput={(e) => {
							updateRecord(index, subindex, key, e.target.value)
						}}
					/>
				}
				/>
			)

		} else if (type === "toggle") {
			return (
				<CustomToggle
					label={key}
					value={value}
					toggle={() => {
						updateRecord(index, subindex, key, Boolean(!value))
					}}
				/>
			)

		}

		return (
			<FormKeyValue label={key} key={key + subindex} value={
				<input
					size={value.length}
					className="value"
					onChange={() => { console.log("on change!", value, index) }}
					type={type}
					value={value}
					onInput={(e) => {
						updateRecord(index, subindex, key, e.target.value)
					}}
				/>

			}
			/>
		)
	}

	return (
		<div className="ab config-dns-editor">
			<div className="plus-button" onClick={() => addRecord()}>Create a new DNS Record..</div>
			{state.Config?.DNSRecords?.map((r, i) => {
				if (!r) {
					return (<></>)
				}
				return (
					<>
						<div className="dns-record panel">
							{makeInput(i, 0, "Domain", r.Domain, "text")}
							{makeInput(i, 0, "CNAME", r.CNAME, "text")}
							{r.IP?.map((ip, ii) => makeInput(i, ii, "IP", ip, "text"))}
							<div className="add" onClick={() => addIP(i)}>Add IP</div>

							{r.TXT?.map((txt, ii) => makeInput(i, ii, "TXT", txt, "textarea"))}
								<div className="add" onClick={() => addTXT(i)}>Add TXT</div>
							{makeInput(i, 0, "Wildcard", r.Wildcard, "toggle")}

							<div className="buttons">

								<div className="item card-button green"
									onClick={() => {
										saveAll()
									}}>
									Save
								</div>

								<div className="item card-button red"
									onClick={() => {
										deleteRecord(i)
									}}>
									Remove
								</div>

							</div>
						</div>
					</>
				)
			})}



		</div >
	)
}

export default ConfigDNSRecordEditor
