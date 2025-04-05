import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const [pserver, setPServer] = useState(undefined)
	const [nserver, setNServer] = useState(undefined)

	useEffect(() => {
		state.GetPrivateServers()
	}, [])

	const UpdateServer = async () => {
		let resp = await state.API_UpdateServer(pserver)
		if (resp?.status === 200) {
			state.renderPage("pservers")
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
			state.renderPage("pservers")
		}
	}
	const serverCreateOpts = {
		baseClass: "private-server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root_OrgID: true,
			root__id: true,
		},
		saveButton: CreateServer,
	}


	const generateServerTable = () => {
		let rows = []
		state?.PrivateServers?.forEach(server => {

			let servertun = undefined
			let assignedTunnels = 0
			let opts = []
			state?.Tunnels?.map(c => {
				if (c.ServerID === server._id) {
					servertun = c
					opts.push({ value: c.Tag, key: c.Tag, selected: true })
					assignedTunnels++
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
						state.connectToVPN(servertun, undefined)
					})
			}

			state?.ActiveTunnels?.forEach((x) => {
				if (x.CR?.ServerID === server._id) {
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

			if (assignedTunnels > 1){
				conButton = function() {
					state.toggleError("too many tunnels assigned to server")
				}
			}

			let row = {}
			row.items = [
				{
					type: "text",
					value: server.Tag,
					 minWidth: "180px",
					 color: "blue",
					click: function() {
						setPServer(server)
						state.renderPage("pservers")
					}
				},
				{
					type: "select",
					opts: opts,
					value: <CustomSelect
						parentkey={server._id}
						className={"clickable"}
						placeholder={"Assign"}
						setValue={(opt) => {
							state.changeServerOnTunnelUsingTag(opt.value, server._id)
						}}
						options={opts}
					></CustomSelect>,
				},
				{ type: "text", value: server.IP, minWidth:"180px" },
				{ type: "text", value: server.Port },
				{
					type: "text",
					value: <div className={con ? "disconnect" : "connect"}>{con ? "Disconnect" : "Connect"}</div>,
					color: con ? "red" : "green",
					click: conButton,
					width:"50px",
				},
			]
			rows.push(row)
		})

		return rows
	}


	let rows = generateServerTable()
	const headers = [
		{ value: "Tag", minWidth:"180px" },
		{ value: "Tunnel" },
		{ value: "IP", minWidth:"180px" },
		{ value: "Port"},
		{ value: "" }
	]

	if (nserver !== undefined) {
		return (
			<div className="connections">
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
		<div className="ab private-server-wrapper">

			<NewTable
				title={"Private VPN Servers"}
				tableID={"private-servers"}
				className="router-table"
				placeholder={"Search .."}
				header={headers}
				background={true}
				rows={rows}
				button={{
					text: "New Server",
					click: function() {
						let user = state.User
						if (!user) {
							return
						}

						setNServer({
							OrgID: user.OrgID,
							Admin: user._id,
							Tag: "",
							Serial: "",
						})
					}
				}}

			/>
		</div >
	);

}

export default PrivateServers;
