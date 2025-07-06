import React, { useEffect, useState } from "react";
import GLOBAL_STATE from "../state";
import GenericTable from "./GenericTable";
import { TableCell } from "@/components/ui/table";
import { Select, SelectContent, SelectGroup, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";
import { DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { AccessibilityIcon } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { CircleArrowRight } from "lucide-react";
import { LogOut } from "lucide-react";

const PrivateServers = () => {
	const state = GLOBAL_STATE("pservers")
	const [server, setServer] = useState(undefined)
	const [editModalOpen, setEditModalOpen] = useState(false)
	const navigate = useNavigate()

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
		state?.Tunnels?.map(c => {
			if (c.ServerID === server._id) {
				servertun = c
				assignedTunnels++
			}
		})

		let conButton = function() {
			state.ConfirmAndExecute(
				"success",
				"connect",
				10000,
				"",
				"Connect to " + server.Tag,
				() => {
					if (assignedTunnels < 1) {
						state.connectToVPN(undefined, server)
					} else {
						state.connectToVPN(servertun, undefined)
					}
				})
		}

		if (assignedTunnels > 1) {
			conButton = function() {
				state.toggleError("too many tunnels assigned to server")
			}
		}

		let con = undefined

		state?.ActiveTunnels?.forEach((x, i) => {
			if (x.CR?.ServerID === server._id) {
				con = state?.ActiveTunnels[i]
				return
			}
		})

		let disconnectButton = undefined
		if (con) {
			disconnectButton = function() {
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



		return <div>
			<DropdownMenuItem
				key="connect"
				onClick={() => conButton()}
				className={"cursor-pointer text-[#3a994c]"}
			>
				<CircleArrowRight className="w-4 h-4 mr-2" /> Connect
			</DropdownMenuItem >
			{disconnectButton &&
				<DropdownMenuItem
					key="disconnect"
					onClick={() => disconnectButton()}
					className={"cursor-pointer text-[#ef4444]"}
				>
					<LogOut className="w-4 h-4 mr-2" /> Disconnect
				</DropdownMenuItem >
			}
		</div >
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
		let assigned = "Assign to tunnel"
		if (assignedTunnels > 1) {
			assigned = String(assignedTunnels) + " tunnels assigned"
		} else {
			value = servertun?.Tag
		}

		return <TableCell className={"w-[100px] text-white"}  >
			<Select value={value}
				onValueChange={(e) => {
					state.changeServerOnTunnelUsingTag(e, obj._id)
				}}
			>
				<SelectTrigger className="w-full">
					<SelectValue placeholder={assigned} />
				</SelectTrigger>
				<SelectContent
					className={"bg-transparent" + state.Theme.borderColor + state.Theme?.mainBG}
				>
					<SelectGroup>
						{opts?.map(t => {
							if (t.selected === true) {
								return (
									<SelectItem className={state.Theme?.activeSelect} value={t.value}>{t.key}</SelectItem>
								)
							} else {
								return (
									<SelectItem className={state.Theme?.neutralSelect} value={t.value}>{t.key}</SelectItem>
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
			Tag: (obj) => {
				navigate("/server/" + obj._id)
			},
			Country: true,
			IP: true,
			Port: true,
			_id: true,
		},
		columnFormat: {
			Country: (row) => {
				let x = state.GetCountryName(row.Country)
				return x
			}
		},
		columnClass: {
			Country: () => {
				return "min-w-[100px]"
			}
		},
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
				setServer({ Tag: "", Country: "", IP: "", Port: "", DataPort: "", PubKey: "" })
				setEditModalOpen(true)
			},
		},
		headerFormat: {
			_id: () => {
				return "ID"
			},
			Tag: () => {
				return "Name"
			}
		},
		headers: ["Tag", "Country", "IP", "Port", "_id", "Interface"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
		more: state.GetServers,
	}

	return (
		<div className="ab private-server-wrapper w-full" >
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
					setEditModalOpen(false)
				}}
				onChange={(key, value, type) => {
					server[key] = value
					console.log(key, value, type)
				}}
			/>

		</div >
	);
}

export default PrivateServers;
