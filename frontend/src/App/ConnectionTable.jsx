import React, { useEffect, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import Loader from "react-spinners/ScaleLoader";
import STORE from "../store";
import GLOBAL_STATE from "../state";
import NewTable from "./component/newtable";
import CustomSelect from "./component/CustomSelect";
import ObjectEditor from "./ObjectEditor";

const ConnectionTable = () => {
	const state = GLOBAL_STATE("connections")
	const [con, setCon] = useState(undefined)
	const navigate = useNavigate()

	let user = STORE.GetUser()

	if (!user) {
		return (<Navigate to={"/login"} />)
	}

	useEffect(() => {
		let x = async () => {
			await state.GetBackendState()
			await state.GetServers()
		}
		x()
	}, [])


	const addConnection = () => {
		let new_conn = {
			Tag: "newtag",
			IFName: "newconn",
			IFIP: "0.0.0.0",
		}
		state.createConnection(new_conn).then(function(conn) {
			if (conn !== undefined) {
				state.renderPage("connections")
			}
		})
	}

	const saveConnection = () => {
		let modCons = state.GetModifiedConnections()
		let found = false
		modCons.forEach((n, i) => {
			if (n.WindowsGUID === con.WindowsGUID) {
				modCons[i] = x
				found = true
			}
		})
		if (!found) {
			modCons.push(con)
		}

		state.SaveConnectionsToModifiedConfig(modCons)
		state.ConfigSave()
		state.renderPage("connections")
	}

	const editorOpts = {
		baseClass: "connection-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		defaults: {
			root_AllowedHosts: [],
		},
		titles: {
			root_DNSServers: "DNS Servers",
			root_DNS: "DNS Records",
			root_DNS_IP: "IP Addresses",
			root_DNS_TXT: "TXT Records"
		},
		newButtons: {
			root_DNS: (obj) => {
				obj.push({ Domain: "MEOW", Wildcard: true, CNAME: "", IP: [], TXT: [] })
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
		delButtons: [],
		saveButton: saveConnection,
	}

	const popEditor = (r) => {
		state.editorData = r
		state.editorReadOnly = true
		state.globalRerender()
	}

	let rows = []

	state?.Config?.Connections.forEach((c, i) => {
		let row = { items: [] }
		row.items.push({
			type: "text",
			align: "left",
			className: "tag",
			value: c.Tag,
		})

		let active = false
		state.State.ActiveConnections?.map((x) => {
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
				setCon(c)
				state.renderPage("connections")
				// openConnectionEditor(c)
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

		row.items.push({
			type: "select",
			opts: opts,
			value: <CustomSelect
				parentkey={c.Tag}
				className={"clickable"}
				placeholder={"Assign"}
				setValue={(opt) => {
					state.changeServerOnConnection(c.Tag, opt.key)
				}}
				options={opts}
			></CustomSelect>,
		})


		row.items.push({
			type: "text",
			align: "left",
			className: "serverip",
			value: server ? server.IP ? server.IP : c.PrivateIP : "",
			color: "blue",
			click: () => {
				popEditor(server)
			}
		})

		row.items.push({
			type: "text",
			align: "right",
			className: "serverip",
			value: active ? "disconnect" : server ? "connect" : "",
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

	const headers = [
		{ value: "Tag", align: "left" },
		{ value: "Interface", align: "left" },
		{ value: "Type", align: "left" },
		{ value: "Server", align: "left" },
		{ value: "IP", align: "left" },
		{ value: "", align: "right" },
	]

	if (con !== undefined) {
		return (
			<div className="connections">
				<div className="back" onClick={() => setCon(undefined)}>Back to tunnels</div>
				<ObjectEditor
					opts={editorOpts}
					object={con}
				/>
			</div>
		)
	}

	return (
		<div className="connections" >
			{(!state.Config?.Connections || state.Config?.Connections.length < 1) &&
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

		</div>
	);
}

export default ConnectionTable;
