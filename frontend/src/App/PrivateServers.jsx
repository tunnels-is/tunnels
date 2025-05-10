import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import GenericTable from "./GenericTable";
import { TableCell } from "@/components/ui/table";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const [server, setServer] = useState(undefined)
	const [editModalOpen, setEditModalOpen] = useState(false)

	useEffect(() => {
		state.GetServers()
	}, [])

	const saveServer = () => {
		if (server._id !== undefined) {
			UpdateServer()
			return
		}

		CreateServer()
	}

	const UpdateServer = async () => {
		let resp = await state.callController(null, null, "POST", "/v3/server/update", { Server: server }, false, false)
		if (resp?.status === 200) {
			state.PrivateServers.forEach((s, i) => {
				if (s._id === server._id) {
					state.PrivateServers[i] = server;
				}
			});
			state.updatePrivateServers();
			state.renderPage("pservers")
			setEditModalOpen(false)
		}
	}

	const CreateServer = async () => {
		let resp = await state.callController(null, null, "POST", "/v3/server/create", { Server: server }, false, false)
		if (resp?.status === 200) {
			if (!state.PrivateServers) {
				state.PrivateServers = [];
			}
			state.PrivateServers.push(resp.data);
			state.updatePrivateServers();
			state.renderPage("pservers")
			setEditModalOpen(false)
		}
	}

	const ConnectColumn = (server) => {
		let servertun = undefined
		let assignedTunnels = 0
		let label = "Connect"
		state?.Tunnels?.map(c => {
			if (c.ServerID === server._id) {
				servertun = c
				assignedTunnels++
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
			label = "Disconnect"
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

		return <TableCell onClick={() => conButton()} className={"w-[10px] text-sky-100"}  >
			<Button>{label}</Button>
		</TableCell>
	}

	const TunnelsColumn = (obj) => {
		let servertun = undefined
		let assignedTunnels = 0
		let opts = []

		state?.Tunnels?.map(c => {
			if (c.ServerID === obj._id) {
				servertun = c
				opts.push({ value: c.Tag, key: c.Tag, selected: true })
				assignedTunnels++
			} else {
				opts.push({ value: c.Tag, key: c.Tag, selected: false })
			}
		})

		let value = undefined
		let assigned = "Assign To Tunnels"
		if (assignedTunnels > 1) {
			assigned = String(assignedTunnels) + " Assigned"
		} else {
			value = servertun?.Tag
		}

		return <TableCell className={"w-[10px] text-sky-100"}  >
			<Select value={value}
				onValueChange={(e) => {
					state.changeServerOnTunnelUsingTag(e, obj._id)
				}}
			>
				<SelectTrigger className="w-[150px]">
					<SelectValue placeholder={assigned} />
				</SelectTrigger>
				<SelectContent>
					<SelectGroup>
						{opts?.map(t => {
							if (t.selected === true) {
								return (
									<SelectItem className="bg-sky-300" value={t.value}>{t.key}</SelectItem>
								)
							} else {
								return (
									<SelectItem value={t.value}>{t.key}</SelectItem>
								)
							}
						})}
					</SelectGroup>
				</SelectContent>
			</Select>
		</TableCell >
	}

	let table = {
		data: state.PrivateServers,
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		columns: {
			Tag: true,
			Country: true,
			IP: true,
			Port: true,
			DataPort: true,
		},
		columFormat: {},
		customColumns: {
			Tunnels: TunnelsColumn,
		},
		customBtn: {
			Connect: ConnectColumn,
		},
		Btn: {
			Edit: (obj) => {
				setServer(obj)
				setEditModalOpen(true)
			},
			Delete: (obj) => {
				// TODO
			},
			New: () => {
				setServer({ Tag: "", Country: "", IP: "", Port: 0, DataPort: 0, PubKey: "" })
				setEditModalOpen(true)
			},
		},
		columnClass: {},
		headers: ["Tag", "Country", "IP", "Port", "DataPort", "Tunnels"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
		more: state.GetServers,
	}

	return (
		<div className="ab private-server-wrapper w-full">
			<GenericTable table={table} />

			<NewObjectEditorDialog
				open={editModalOpen}
				onOpenChange={setEditModalOpen}
				object={server}
				title="Server"
				description=""
				readOnly={false}
				saveButton={() => {
					saveServer()
				}}
				onChange={(key, value, type) => {
					server[key] = value
					console.log(key, value, type)
				}}
			/>

		</div>
	);
}

export default PrivateServers;
