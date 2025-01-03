import React, { useEffect } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import Loader from "react-spinners/ScaleLoader";
import STORE from "../store";
import GLOBAL_STATE from "../state";
import NewTable from "./component/newtable";
import CustomSelect from "./component/CustomSelect";

const ConnectionTable = () => {
	const state = GLOBAL_STATE("connections")
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

	const popEditor = (r) => {
		state.editorData = r
		state.editorReadOnly = true
		state.globalRerender()
	}

	const openConnectionEditor = (con) => {
		state.editorData = con
		state.editorReadOnly = false
		state.editorDelete = function() {
			state.DeleteConnection(con.WindowsGUID)
		}
		state.editorSave = function() {
			state.ConfigSave()
		}
		state.editorOnChange = function(data) {
			// console.dir(data)
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
				state.globalRerender()
				return
			}

			let modCons = state.GetModifiedConnections()
			let found = false
			modCons.forEach((n, i) => {
				if (n.WindowsGUID === x.WindowsGUID) {
					modCons[i] = x
					found = true
				}
			})

			if (!found) {
				modCons.push(x)
			}

			state.SaveConnectionsToModifiedConfig(modCons)
			state.globalRerender()
		}

		state.editorExtraButtons = []
		state.editorExtraButtons.push({
			func: function() {
				let ed = state.editorData
				if (ed === undefined) {
					console.log("no data!")
					return
				}
				if (ed.Networks.length < 1) {
					ed.Networks.push({
						Tag: "new-network",
						Network: "",
						Nat: "",
						Routes: []
					})
				}
				ed.Networks[0].Routes.push(
					{
						Address: "0.0.0.0/32",
						Metric: "0"
					}
				)

				state.globalRerender()
			},
			title: "+Route"
		})
		state.editorExtraButtons.push({
			func: function() {
				let ed = state.editorData
				if (ed === undefined) {
					console.log("no data!")
					return
				}
				ed.Networks.push({
					Tag: "new-network",
					Network: "",
					Nat: "",
					Routes: []
				})

				state.globalRerender()
			},
			title: "+Network"
		})
		state.editorExtraButtons.push({
			func: function() {
				let ed = state.editorData
				if (ed === undefined) {
					console.log("no data!")
					return
				}
				ed.DNS.push({
					Domain: "mydomain.local",
					Wildcard: true,
					IP: ["127.0.0.1"],
					TXT: ["this is a text record"],
					CNAME: "",
				})

				state.globalRerender()
			},
			title: "+DNS"
		})


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
			click: () => {
				openConnectionEditor(c)
			}
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
		})

		row.items.push({
			type: "text",
			align: "left",
			className: "type",
			value: c.Private ? "private" : "public",
		})

		row.items.push({
			type: "text",
			align: "left",
			className: "dns",
			value: c.DNS.length,
		})


		let routeCount = 0
		c.Networks?.map(n => {
			routeCount += n.Routes.length
		})

		row.items.push({
			type: "text",
			align: "left",
			className: "dns",
			value: routeCount,
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

		if (!server) {
			state.PrivateServers?.map((x) => {
				if (x._id === c.ServerID) {
					opts.push({ value: x.Server, key: x._id, selected: true })
					server = x
					return
				} else {
					opts.push({ value: x.Server, key: x._id, selected: false })
				}
			})
		}

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
			value: server ? server.IP : "",
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
		{ value: "DNS", align: "left" },
		{ value: "Routes", align: "left" },
		{ value: "Server", align: "left" },
		{ value: "IP", align: "left" },
		{ value: "", align: "right" },
	]


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
