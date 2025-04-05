import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";

const ServersFull = () => {
	const state = GLOBAL_STATE("servers")
	const [server, setServer] = useState(undefined)
	const [pserver, setPServer] = useState(undefined)
	const [nserver, setNServer] = useState(undefined)

	const editorOpts = {
		baseClass: "server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		readOnly: true,
	}

	useEffect(() => {
		state.GetServers()
		state.GetPrivateServers()
	}, [])

	const CreateServerPane = () => {
						let user = state.User
						if (!user) {
							return
						}

						setNServer({
			OrgID: user.OrgID === "000000000000000000000000" ? "" : user.OrgID,
							Admin: user._id,
							Tag: "",
							Serial: "",
						})

	}

	const UpdateServer = async () => {
		let resp = await state.API_UpdateServer(pserver)
		if (resp?.status === 200) {
			state.renderPage("servers")
		}
	}
	const serverUpdateOpts = {
		baseClass: "private-server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root_OrgID: true,
			root__id: true,
		},
		saveButton: UpdateServer,
	}

	const CreateServer = async () => {
		let user = state.User
		if (!user) {
			return
		}
		nserver.OrgID = user.OrgID
		nserver.Admin = user._id

		let resp = await state.API_CreateServer(nserver)
		if (resp?.status === 200) {
			state.renderPage("servers")
		}
	}
	const serverCreateOpts = {
		baseClass: "private-server-object-creator",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root_OrgID: true,
			root__id: true,
		},
		saveButton: CreateServer,
	}

	const generatePrivateServerTable = () => {
		let rows = []
		state.PrivateServers?.forEach(server => {

			let servertun = undefined
			let opts = []
			state?.Tunnels?.map(c => {
				if (c.ServerID === server._id) {
					servertun = c
					opts.push({ value: c.Tag, key: c.Tag, selected: true })
				} else {
					opts.push({ value: c.Tag, key: c.Tag, selected: false })
				}
			})

			let con = undefined
			let conButton = function() {
				state.ConfirmAndExecute(
					"success",
					"connect",
					10000,
					"",
					"Connect to " + server.Tag,
					() => {
						if (server.IP !== "") {
							state.connectToVPN(undefined, server)
						} else if (servertun !== undefined) {
							state.connectToVPN(servertun, undefined)
						}
					})
			}

			state.ActiveTunnels?.forEach((x) => {
				if (x.ServerID === server._id) {
					con = x
					return
				}
			})

			if (con) {
				conButton = function() {
					state.ConfirmAndExecute(
						"success",
						"disconnect",
						10000,
						"",
						"Disconnect from " + server.Tag,
						() => {
							state.disconnectFromVPN(con)
						})
				}

			}



			let row = {}
			row.items = [
				{
					type: "text",
					value: server.Tag+"sfdfd",
					color: "blue",
					click: function() {
						setPServer(server)
						state.renderPage("servers")
					}
				},
				{ type: "text", value: server.IP },
				{ type: "text", minWidth: "380px", value: server.Serial },
				{
					type: "select",
					opts: opts,
					value: <CustomSelect
						parentkey={server._id}
						className={"clickable"}
						placeholder={"Assign"}
						setValue={(opt) => {
							state.changeServerOnConnection(opt.value, server._id)
						}}
						options={opts}
					></CustomSelect>,
				},
				{
					type: "text",
					value: <div className={con ? "disconnect" : "connect"}>{con ? "Disconnect" : "Connect"}</div>,
					color: con ? "red" : "green",
					click: conButton,
				},
			]
			rows.push(row)
		})

		return rows
	}


	let prows = generatePrivateServerTable()
	const pheaders = [
		{ value: "Tag" },
		{ value: "IP", },
		{ value: "Cert Serial", minWidth: "380px" },
		{ value: "Tunnel" },
		{ value: "" }
	]



	const generateServerTable = () => {
		let rows = []

		state.Servers?.forEach(server => {

			let con = undefined
			let conButton = function() {
				state.ConfirmAndExecute(
					"success",
					"connect",
					10000,
					"",
					"Connect to " + server.Tag,
					() => {
						state.connectToVPN(undefined, server)
					})
			}

			state.ActiveTunnels?.forEach((x) => {
				if (x.ServerID === server._id) {
					con = x
					return
				}
			})

			if (con) {
				conButton = function() {
					state.ConfirmAndExecute(
						"success",
						"disconnect",
						10000,
						"",
						"Disconnect from " + server.Tag,
						() => {
							state.disconnectFromVPN(con)
						})
				}

			}

			let country = "icon"
			if (server.Country !== "") {
				country = server.Country.toLowerCase()
			}

			let opts = []
			state?.Tunnels?.map(c => {
				if (c.ServerID === server._id) {
					opts.push({ value: c.Tag, key: c.Tag, selected: true })
				} else {
					opts.push({ value: c.Tag, key: c.Tag, selected: false })
				}
			})


			let row = {}
			row.items = [
				{
					type: "img",
					align: "right",
					className: "flag",
					value: "https://raw.githubusercontent.com/tunnels-is/media/master/nl-website/v2/flags/" + country + ".svg"
				},
				{
					type: "text",
					value: server.Tag,
					color: "blue",
					click: function() {
						setServer(server)
						state.renderPage("servers")
					}
				},
				{ type: "text", value: server.Server ? server.Server : "Unknown" },
				{ type: "text", value: server.IP },
				{
					type: "select",
					opts: opts,
					align: "center",
					value: <CustomSelect
						parentkey={server._id}
						className={"clickable"}
						placeholder={"Assign"}
						setValue={(opt) => {
							state.changeServerOnConnection(opt.value, server._id)
						}}
						options={opts}
					></CustomSelect>,
				},
				{
					type: "text",
					value: <div className={con ? "disconnect" : "connect"}>{con ? "Disconnect" : "Connect"}</div>,
					color: con ? "red" : "green",
					click: conButton,
				},
			]
			rows.push(row)
		})

		return rows
	}

	let rows = generateServerTable()
	const headers = [
		{ value: "" },
		{ value: "Tag" },
		{ value: "Server" },
		{ value: "IP" },
		{ value: "Tunnel", align:"center" },
		{ value: "" }
	]
	if (server !== undefined) {
		return (
			<div className="ab router-wrapper">
				<div className="back" onClick={() => setServer(undefined)}>Back to Servers</div>
				<ObjectEditor
					opts={editorOpts}
					object={server}
				/>
			</div>
		)
	}

	if (nserver !== undefined) {
		return (
			<div className="ab router-wrapper">
				<div className="back" onClick={() => setNServer(undefined)}>Back to server</div>
				<ObjectEditor
					opts={serverCreateOpts}
					object={nserver}
				/>
			</div>
		)
	}

	if (pserver !== undefined) {
		return (
			<div className="connections">
				<div className="back" onClick={() => setPServer(undefined)}>Back to server</div>
				<ObjectEditor
					opts={serverUpdateOpts}
					object={pserver}
				/>
			</div>
		)
	}

	return (
		<div className="ab router-wrapper">

			<div className="create"
				onClick={() => CreateServerPane()}
			>Create a private server

			</div>
			{prows?.items?.length > 0 &&
			<NewTable
				title={"Private VPN Servers"}
				tableID={"private-servers"}
				className="router-table"
				placeholder={"Search .."}
				header={pheaders}
				background={true}
				rows={prows}
				button={{
					text: "New Server",
					click: function() {
							CreateServerPane()
					}
				}}
			/>
			}

			<NewTable
				title={"Public VPN Servers"}
				tableID={"public-servers"}
				className="router-table"
				placeholder={"Search .."}
				background={true}
				header={headers}
				rows={rows}
			/>
		</div >
	);

}

export default ServersFull;
