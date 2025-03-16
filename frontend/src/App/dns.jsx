import React, { useEffect } from "react";
import CustomToggle from "./component/CustomToggle";
import FormKeyValue from "./component/formkeyvalue";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import dayjs from "dayjs";

import GLOBAL_STATE from "../state";
import STORE from "../store";

const DNSSort = (a, b) => {
	if (dayjs(a.LastSeen).unix() < dayjs(b.LastSeen).unix()) {
		return 1
	} else if (dayjs(a.LastSeen).unix() > dayjs(b.LastSeen).unix()) {
		return -1
	}
	return 0

}

const DNS = () => {
	const state = GLOBAL_STATE("dns")
	const navigate = useNavigate()
	let LogBlockedDomains = state.getKey("Config", "LogBlockedDomains")
	let LogAllDomains = state.getKey("Config", "LogAllDomains")
	let dnsStats = state.getKey("Config", "DNSstats")


	let DNS1 = state.getKey("Config", "DNS1Default")
	let DNS2 = state.getKey("Config", "DNS2Default")

	let DNSServerIP = state.getKey("Config", "DNSServerIP")
	let DNSOverHTTPS = state.getKey("Config", "DNSOverHTTPS")
	let DNSServerPort = state.getKey("Config", "DNSServerPort")

	let modified = STORE.Cache.GetBool("modified_Config")

	useEffect(() => {
		state.GetBackendState()
	}, [])

	let blockLists = state.Config?.DNSBlockLists
	state.modifiedLists?.forEach(l => {
		blockLists?.forEach((ll, i) => {
			if (ll.Tag === l.Tag) {
				blockLists[i] = l
			}
		})
	})
	if (!blockLists) {
		blockLists = []
	}

	const default_lists = ["Ads", "AdultContent", "CryptoCurrency", "Drugs", "FakeNews", "Fraud", "Gambling", "Malware", "SocialMedia", "Surveillance"]

	const isDefault = (tag) => {
		return default_lists.includes(tag)
	}

	const generateListTable = (blockLists) => {
		let rows = []
		blockLists.forEach(i => {
			let row = {}
			row.items = [
				{
					type: "text",
					value: <div
						className={`${i.Enabled ? "enabled" : "disabled"} clickable`}
						onClick={() => { state.toggleBlocklist(i) }}
					>	{i.Enabled ? "Blocked" : "Allowed"}</div>
				},
				{ type: "text", value: i.Tag },
				{ type: "text", value: i.Count },
				{
					type: "text",
					value: <div
						className={`${isDefault(i.Tag) ? "disabled" : "red"} clickable`}
						onClick={() => { state.deleteBlocklist(i) }}
					>Remove</div>
				}
			]
			rows.push(row)
		})
		return rows
	}

	const generateBlocksTable = () => {
		let dnsBlocks = state.State?.DNSBlocks
		let rows = []

		if (!dnsBlocks || dnsBlocks.length === 0) {
			return rows
		}

		let stats = []

		Object.entries(dnsBlocks).forEach(([key, value]) => {
			stats.push({ ...value, tag: key })
		});

		stats = stats.sort(DNSSort)

		stats.forEach(value => {
			let row = {}
			row.items = [
				{ type: "text", value: value.tag, tooltip: true, },
				{ type: "text", value: value.Tag, },
				{ type: "text", value: dayjs(value.FirstSeen).format(state.DNSListDateFormat) },
				{ type: "text", value: dayjs(value.LastSeen).format(state.DNSListDateFormat) },
				{ type: "text", value: value.Count },
			]
			rows.push(row)
		})
		return rows
	}

	const generateResolvesTable = () => {
		let dnsResolves = state.State?.DNSResolves
		let rows = []

		if (!dnsResolves || dnsResolves.length === 0) {
			return rows
		}

		let stats = []
		Object.entries(dnsResolves).forEach(([key, value]) => {
			stats.push({ ...value, tag: key })
		});

		stats = stats.sort(DNSSort)

		stats.forEach((value) => {
			let row = {}
			row.items = [
				{
					tooltip: true,
					type: "text",
					value: value.tag,
					color: "blue",
					width: 30,
					click: () => {
						navigate("/dns/answers/" + value.tag)
					}
				},
				{ type: "text", value: dayjs(value.FirstSeen).format(state.DNSListDateFormat) },
				{ type: "text", value: dayjs(value.LastSeen).format(state.DNSListDateFormat) },
				{ type: "text", value: value.Count, },
			]
			rows.push(row)
		})
		return rows
	}

	let rows = generateListTable(blockLists)
	const headers = [
		{ value: "Enabled" },
		{ value: "Tag" },
		{ value: "Domains" },
		{ value: "" },
	]

	let rowsDNSstats = generateBlocksTable()
	const headersDNSstats = [
		{ value: "Domain" },
		{ value: "List" },
		{ value: "First Seen" },
		{ value: "Last Seen" },
		{ value: "Blocked" },
	]

	let rowsDNSresolves = generateResolvesTable()
	const headerDNSresolves = [
		{ value: "Domain", width: 30 },
		{ value: "First Seen" },
		{ value: "Last Seen" },
		{ value: "Resolved" },
	]

	return (
		<div className="dns-page">
			{modified === true &&
				<div className="save-banner">
					<div className="button"
						onClick={() => state.v2_ConfigSave()}
					>
						Save
					</div>
					<div className="notice">Your config has un-saved changes</div>
				</div>
			}


			<div className="basic-info panel">
				<div className="title">Settings</div>
				<div className="warn-msg">Enabling blocklists will increase memory usage.</div>
				<div className="button-and-text-seperator"></div>

				<FormKeyValue label={"Server IP"} value={
					<input value={DNSServerIP} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"DNSServerIP",
							e.target.value)

						state.renderPage("dns")
					}} type="text" />}
				/>

				<FormKeyValue label={"Server Port"} value={
					<input value={DNSServerPort} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"DNSServerPort",
							e.target.value)

						state.renderPage("dns")
					}} type="text" />}
				/>

				<FormKeyValue label={"Primary DNS"} value={
					<input value={DNS1} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"DNS1Default",
							e.target.value)

						state.renderPage("dns")
					}} type="text" />}
				/>

				<FormKeyValue label={"Backup DNS"} value={
					<input value={DNS2} onChange={(e) => {

						state.setKeyAndReloadDom(
							"Config",
							"DNS2Default",
							e.target.value)

						state.renderPage("dns")
					}} type="text" />}
				/>

				<CustomToggle
					label="Secure DNS"
					value={DNSOverHTTPS}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "DNSOverHTTPS")
						state.fullRerender()
					}}
				/>

				<CustomToggle
					label="Log Blocked"
					value={LogBlockedDomains}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "LogBlockedDomains")
						state.fullRerender()
					}}
				/>

				<CustomToggle
					label="Log All"
					value={LogAllDomains}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "LogAllDomains")
						state.fullRerender()
					}}
				/>

				<CustomToggle
					label="DNS Stats"
					value={dnsStats}
					toggle={() => {
						state.toggleKeyAndReloadDom("Config", "DNSstats")
						state.fullRerender()
					}}
				/>
			</div>


			<NewTable
				tableID="dns-lists"
				title={"Block Lists"}
				className="domain-list-table"
				background={true}
				header={headers}
				rows={rows}
				button={{
					text: "New Blocklist",
					click: function() {
						navigate("/inspect/blocklist")
					}
				}}
			/>

			{dnsStats &&
				<>
					<NewTable
						tableID="dns-blocked"
						title={"Blocked Domains"}
						className="dns-stats"
						background={true}
						header={headersDNSstats}
						rows={rowsDNSstats}
					/>

					<NewTable
						tableID="dns-resolved"
						title={"Resolved Domains"}
						className="dns-stats"
						background={true}
						header={headerDNSresolves}
						rows={rowsDNSresolves}
					/>
				</>
			}
		</div >
	)
}

export default DNS;
