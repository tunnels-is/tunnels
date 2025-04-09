import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import CustomSelect from "./component/CustomSelect";
import { useNavigate } from "react-router-dom";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";
import { Save } from "lucide-react";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogFooter,
	DialogClose
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const [pserver, setPServer] = useState(undefined)
	const [nserver, setNServer] = useState(undefined)
	const [editModalOpen, setEditModalOpen] = useState(false)
	const [createModalOpen, setCreateModalOpen] = useState(false)

	useEffect(() => {
		state.GetPrivateServers()
	}, [])

	const UpdateServer = async () => {
		let resp = await state.API_UpdateServer(pserver)
		if (resp?.status === 200) {
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
			setCreateModalOpen(false)
		}
	}
	const serverCreateOpts = {
		baseClass: "",
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
				{ type: "text", value: server.IP, minWidth:"180px" },
				{ type: "text", value: server.Port },
				{
					type: "text",
					value: <div className={con ? "disconnect" : "connect"}>{con ? "Disconnect" : "Connect"}</div>,
					color: con ? "red" : "green",
					click: conButton,
					width:"50px",
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
		{ value: "Tag", minWidth:"180px" },
		{ value: "Tunnel" },
		{ value: "IP", minWidth:"180px" },
		{ value: "Port"},
		{ value: "" }
	]

	return (
		<div className="ab private-server-wrapper w-full">
			<NewTable
				title={"Private VPN Servers"}
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
							OrgID: user.OrgID,
							Admin: user._id,
							Tag: "",
							Serial: "",
						})
						setCreateModalOpen(true)
					}
				}}
			/>

			{/* Edit Server Dialog */}
			<Dialog open={editModalOpen} onOpenChange={setEditModalOpen}>
				<DialogContent className="sm:max-w-[800px] bg-[#0a0a0a] border-[#222] text-white">
					<DialogHeader>
						<DialogTitle className="text-lg font-bold text-white">Edit Server</DialogTitle>
						<DialogDescription className="text-white/60">
							Make changes to your server configuration here.
						</DialogDescription>
					</DialogHeader>
					
					{pserver && (
						<div className="py-4 max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">
							<ObjectEditor
								hideSaveButton={true}
								opts={serverUpdateOpts}
								object={pserver}
							/>
						</div>
					)}
					
					<DialogFooter className="flex items-center justify-end gap-2 pt-4 border-t border-[#222]">
					{serverUpdateOpts.saveButton && pserver  && (
							<Button
								variant="outline"
								onClick={() => serverUpdateOpts.saveButton(pserver)}
								className="h-9 border-emerald-800/40 bg-[#0c1e0c] text-emerald-400 hover:bg-emerald-900/30 hover:text-emerald-300 shadow-sm font-medium"
							>
								<Save className="h-4 w-4 mr-1" />
								Save
							</Button>
						)}
						<Button 
							variant="outline" 
							onClick={() => setEditModalOpen(false)}
							className="h-9 px-4 text-sm font-medium text-white/80 border-[#222] bg-[#111] hover:bg-[#222] hover:text-white"
						>
							Cancel
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{/* Create Server Dialog */}
			<Dialog open={createModalOpen} onOpenChange={setCreateModalOpen}>
				<DialogContent className="sm:max-w-[800px] bg-[#0a0a0a] border-[#222] text-white">
					<DialogHeader>
						<DialogTitle className="text-lg font-bold text-white">New Server</DialogTitle>
						<DialogDescription className="text-white/60">
							Configure your new server settings below.
						</DialogDescription>
					</DialogHeader>
					
					{nserver && (
						<div className="py-4 max-h-[70vh] overflow-y-auto overflow-x-hidden pr-2">
							<ObjectEditor
								hideSaveButton={true}
								opts={serverCreateOpts}
								object={nserver}
							/>
						</div>
					)}
					
					<DialogFooter className="flex items-center justify-end gap-2 pt-4 border-t border-[#222]">
					{serverCreateOpts.saveButton && nserver  && (
							<Button
								variant="outline"
								onClick={() => serverCreateOpts.saveButton(nserver)}
								className="h-9 border-emerald-800/40 bg-[#0c1e0c] text-emerald-400 hover:bg-emerald-900/30 hover:text-emerald-300 shadow-sm font-medium"
							>
								<Save className="h-4 w-4 mr-1" />
								Save
							</Button>
						)}
						<Button 
							variant="outline" 
							onClick={() => setCreateModalOpen(false)}
							className="h-9 px-4 text-sm font-medium text-white/80 border-[#222] bg-[#111] hover:bg-[#222] hover:text-white"
						>
							Cancel
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
		</div>
	);
}

export default PrivateServers;
