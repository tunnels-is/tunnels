import React, { useEffect, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import Loader from "react-spinners/ScaleLoader";
import STORE from "../store";
import GLOBAL_STATE from "../state";
import NewTable from "./component/newtable";
import CustomSelect from "./component/CustomSelect";
import ObjectEditor from "./ObjectEditor";
import dayjs from "dayjs";

const ConnectionTable = () => {
	const state = GLOBAL_STATE("connections")
	const [tunnel, setTunnel] = useState(undefined)
	const [tunnelTag, setTunnelTag] = useState(undefined)
	const [pserver, setPServer] = useState(undefined)

	let user = STORE.GetUser()

	if (!user) {
		return (<Navigate to={"/login"} />)
	}

	useEffect(() => {
		let x = async () => {
			await state.GetPrivateServers()
			await state.GetServers()
			state.GetBackendState()
		}
		x()
	}, [])


	const addConnection = () => {
		state.createTunnel()
	}


	const editorOpts = {
		baseClass: "connection-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		defaults: {
			root_AllowedHosts: [],
		},
		backButton: {
			title: "Back To Tunnels",
			func: () => {
				setTunnel(undefined)
				setTunnelTag(undefined)
			},
		},
		saveButton: (tun) => {
			state.v2_TunnelSave(tun, tunnelTag)
			state.renderPage("connections")
		},
		deleteButton: {
			title: "Delete",
			func: (tun) => {
				state.v2_TunnelDelete(tun)
				setTunnel(undefined)
				setTunnelTag(undefined)
			},
		},
		titles: {
			root_DNSServers: "DNS Servers",
			root_DNS: "DNS Records",
			root_DNS_IP: "IP Addresses",
			root_DNS_TXT: "TXT Records"
		},
		newButtons: {
			root_DNS: (obj) => {
				obj.push({ Domain: "mydomain.com", Wildcard: true, CNAME: "", IP: [], TXT: [] })
			},
			root_DNS_IP: (obj) => {
				obj.push("0.0.0.0")
			},
			root_DNS_TXT: (obj) => {
				obj.push("new text record")
			},
			root_Networks: (obj) => {
				obj.push({ Tag: "new-network", Network: "", Nat: "", Routes: [] })
			},
			root_Networks_Routes: (obj) => {
				obj.push({ Address: "0.0.0.0/0", Metric: "9999" })
			},
			root_AllowedHosts: (obj) => {
				obj.push("0.0.0.0")
			},
			root_DNSServers: (obj) => {
				obj.push("9.9.9.9")
			},
		},
	}

	const serverOpts = {
		baseClass: "private-server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		readOnly: true,
		backButton: {
			title: "Back",
			func: () => {
				setPServer(undefined)
			}
		}
	}

	let rows = []

	state?.Tunnels?.forEach((c, i) => {
		let row = { items: [] }
		row.items.push({
			type: "text",
			align: "left",
			className: "tag",
			value: c.Tag,
		})

		let active = false
		state.ActiveTunnels?.map((x) => {
			if (x.WindowsGUID === c.WindowsGUID) {
				active = true
				return
			}
		})

		row.items.push({
			type: "text",
			align: "left",
			className: "tag",
			value: c.IFName,
			color: "blue",
			click: () => {
				setTunnel(c)
				setTunnelTag(c.Tag)
				state.renderPage("connections")
			}
		})

		row.items.push({
			type: "text",
			align: "left",
			className: "type",
			value: c.Private ? "private" : "public",
		})


		let opts = []

		let server = undefined
		state.Servers?.map((x) => {
			if (x._id === c.ServerID) {
				opts.push({ value: x.Server, key: x._id, selected: true })
				server = x
				return
			} else {
				opts.push({ value: x.Server, key: x._id, selected: false })

			}
		})

		state.PrivateServers?.map((x) => {
			if (x._id === c.ServerID) {
				opts.push({ value: x.Tag, key: x._id, selected: true })
				server = x
				return
			} else {
				opts.push({ value: x.Tag, key: x._id, selected: false })
			}
		})
		if (opts.length === 0) {
			opts.push({ value: "No Servers..", key: "", selected: false })
		}

		row.items.push({
			type: "select",
			opts: opts,
			value: <CustomSelect
				parentkey={c.Tag}
				className={"clickable"}
				placeholder={"Click Here"}
				setValue={(opt) => {
					state.changeServerOnTunnel(c, opt.key)
				}}
				options={opts}
			></CustomSelect>,
		})


		row.items.push({
			type: "text",
			align: "left",
			className: "serverip",
			value: server ? server.IP ? server.IP : c.PrivateIP : "unknown",
			color: "blue",
			click: () => {
				if (server) {
					setPServer(server)
					state.renderPage("connections")
				}
			}
		})

		row.items.push({
			type: "text",
			align: "right",
			className: "connect-button",
			value: active ? "Disconnect" : server ? "Connect" : "",
			color: active ? "red" : "green",
			click: () => {
				if (active) {
					state.ConfirmAndExecute(
						"success",
						"disconnect",
						10000,
						"",
						"Disconnect from " + c.Tag,
						() => {
							state.disconnectFromVPN(c)
						})
				} else {
					state.ConfirmAndExecute(
						"success",
						"connect",
						10000,
						"",
						"Connect to " + c.Tag,
						() => {
							state.connectToVPN(c)
						})

				}
			}
		})


		rows.push(row)

	});

	const generateStatsTable = () => {
		let rows = []

		state?.ActiveTunnels?.map(cs => {

			let stocms = Math.floor(String(cs.ServerToClientMicro / 1000))
			let nonce = cs.Nonce1
			if (cs.Nonce2 > cs.Nonce1) {
				nonce = cs.Nonce2
			}

			let row = { items: [] }
			row.items.push({ type: "text", align: "left", className: "tag", value: cs.StatsTag, })
			row.items.push({ type: "text", align: "left", className: "egress", value: cs.EgressString, })
			row.items.push({ type: "text", align: "left", className: "ingress", value: cs.IngressString, })
			row.items.push({ type: "text", align: "left", className: "CPU", value: String(cs.CPU) + " %", })
			row.items.push({ type: "text", align: "left", className: "Mem", value: String(cs.MEM) + " %", })
			row.items.push({ type: "text", align: "left", className: "Disk", value: String(cs.DISK) + " %", })
			row.items.push({ type: "text", align: "left", className: "Ping", value: dayjs(cs.PingTime).format('HH:mm:ss') })
			row.items.push({ type: "text", align: "left", className: "Disk", value: stocms + "ms" })
			row.items.push({ type: "text", align: "left", className: "Disk", value: nonce })
			rows.push(row)
		})

		return rows
	}

	let statsRows = generateStatsTable()
	const statsHeaders = [
		{ value: "Tag", align: "left" },
		{ value: "Egress", align: "left" },
		{ value: "Ingress", align: "left" },
		{ value: "CPU", align: "left" },
		{ value: "Mem", align: "left" },
		{ value: "Disk", align: "left" },
		{ value: "Ping", align: "left" },
		{ value: "MS", align: "left" },
		{ value: "Nonce", align: "left" },
	]

	const headers = [
		{ value: "Tag", align: "left" },
		{ value: "Interface", align: "left" },
		{ value: "Type", align: "left" },
		{ value: "Server", align: "left" },
		{ value: "IP", align: "left" },
		{ value: "", align: "right", className: "connect-header" },
	]

	if (tunnel !== undefined) {
		return (
			<div className="connections">
				<ObjectEditor
					opts={editorOpts}
					object={tunnel}
				/>
			</div>
		)
	}

	if (pserver !== undefined) {
		return (
			<div className="connections">
				<ObjectEditor
					opts={serverOpts}
					object={pserver}
				/>
			</div>
		)
	}

	return (
		<div className="connections" >
			{(!state.Tunnels || state.Tunnels?.length < 1) &&
				<Loader
					key={"loader"}
					className="spinner"
					loading={true}
					color={"#20C997"}
					height={100}
					width={50}
				/>
			}


			{rows.length > 0 &&
				<NewTable
					title={"Tunnels"}
					tableID={"tunnels"}
					className="tunnels-table"
					placeholder={"Search .."}
					background={true}
					header={headers}
					rows={rows}
					button={{
						text: "New Tunnel",
						click: function() {
							addConnection()
						}
					}}
				/>
			}

			{statsRows.length > 0 &&
				<NewTable
					title={"Connection Stats"}
					tableID={"connection-stats"}
					className="connection-stats"
					placeholder={"Search .."}
					background={true}
					header={statsHeaders}
					rows={statsRows}
				/>
			}

		</div>
	);
}

export default ConnectionTable;
