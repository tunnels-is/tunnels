import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import ObjectEditorDialog from "./component/ObjectEditorDialog";
import STORE from "@/store";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const [pserver, setPServer] = useState(undefined)
	const [nserver, setNServer] = useState(undefined)
	const [editModalOpen, setEditModalOpen] = useState(false)
	const [createModalOpen, setCreateModalOpen] = useState(false)


	useEffect(() => {
		state.GetServers()
	}, [])

	const UpdateServer = async () => {
		let resp = await state.callController(null, null, "POST", "/v3/server/update", { Server: pserver }, false, false)
		if (resp?.status === 200) {
			state.PrivateServers.forEach((s, i) => {
				if (s._id === pserver._id) {
					state.PrivateServers[i] = pserver;
				}
			});
			state.updatePrivateServers();
			state.renderPage("pservers")
			setEditModalOpen(false)
		}
	}
	const serverUpdateOpts = {
		baseClass: "private-server-object-editor",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root__id: true,
		},
		saveButton: UpdateServer,
	}

	const CreateServer = async () => {
		let user = state.User
		if (!user) {
			state.errorNotification("Can not find user, please log in again")
			return
		}
		nserver.Admin = user._id

		let resp = await state.callController(null, null, "POST", "/v3/server/create", { Server: nserver }, false, false)
		if (resp?.status === 200) {
			if (!state.PrivateServers) {
				state.PrivateServers = [];
			}
			state.PrivateServers.push(resp.data);
			state.updatePrivateServers();
			state.renderPage("pservers")
			setCreateModalOpen(false)
		}
	}
	const serverCreateOpts = {
		baseClass: "",
		maxDepth: 1000,
		onlyKeys: false,
		disabled: {
			root_Admin: true,
			root__id: true,
			d0_Groups: true,

		},
		hidden: {
			Groups: true,

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

			if (assignedTunnels > 1) {
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
						setEditModalOpen(true)
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
				{ type: "text", value: server.IP, minWidth: "180px" },
				{ type: "text", value: server.Country },
				{ type: "text", value: server.Port },
				{ type: "text", value: server.DataPort },
				{
					type: "text",
					value: <div className={con ? "disconnect" : "connect"}>{con ? "Disconnect" : "Connect"}</div>,
					color: con ? "red" : "green",
					click: conButton,
					width: "50px",
					s_type: "connect-disconnect",
					s_state: con ? "disconnect" : "connect",
				},
			]
			rows.push(row)
		})

		return rows
	}


	let rows = generateServerTable()
	const headers = [
		// { value: "Tag", minWidth: "180px" },
		// { value: "Tunnel" },
		// { value: "IP", minWidth: "180px" },
		// { value: "Port" },
		// { value: "" }
	]

	return (
		<div className="ab private-server-wrapper w-full">
			<NewTable
				tableID={"private-servers"}
				className="w-full"
				placeholder={"Search .."}
				header={headers}
				background={true}
				rows={rows}
				design="private-vpn-servers"
				button={{
					text: "New Server",
					click: function() {
						let user = state.User
						if (!user) {
							return
						}

						setNServer({
							Admin: user._id,
							Tag: "",
							Country: "",
							IP: "",
							Port: "",
							DataPort: "",
							PubKey: "",
						})
						setCreateModalOpen(true)
					}
				}}
			/>

			<ObjectEditorDialog
				open={editModalOpen}
				onOpenChange={setEditModalOpen}
				object={pserver}
				editorOpts={serverUpdateOpts}
				title="Edit Server"
				description="Make changes to your server configuration"
				readOnly={false}
			/>

			<ObjectEditorDialog
				open={createModalOpen}
				onOpenChange={setCreateModalOpen}
				object={nserver}
				editorOpts={serverCreateOpts}
				title="New Server"
				description="Configure your new server settings"
				readOnly={false}
			/>
		</div>
	);
}

export default PrivateServers;
