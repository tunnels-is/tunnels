import React, { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import GLOBAL_STATE from "../state"
import dayjs from "dayjs";
import NewTable from "./component/newtable";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
	Dialog,
	DialogContent,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Save,
} from "lucide-react";
import { Label } from "@/components/ui/label";
import GenericTable from "./GenericTable";

const InspectGroup = () => {
	const { id } = useParams()
	const [users, setUsers] = useState([])
	const [servers, setServers] = useState([])
	const [devices, setDevices] = useState([])
	const [dialog, setDialog] = useState(false)
	const [addForm, setAddForm] = useState({})
	const [group, setGroup] = useState()
	const state = GLOBAL_STATE("groups")
	const navigate = useNavigate()


	const FormField = ({ label, children }) => (
		<div className="grid gap-2 mb-4">
			<Label className="text-sm font-medium">{label}</Label>
			{children}
		</div>
	);

	const addToGroup = async () => {
		console.log("ID:", addForm.id)
		let e = await state.callController(null, null, "POST", "/v3/group/add",
			{
				GroupID: id,
				TypeID: addForm.id,
				Type: addForm.type,
			},
			false, false)
		if (e) {
			if (addForm.type === "user") {
				users.push(e.data)
				setUsers([...users])
			} else if (addForm.type === "server") {
				servers.push(e.data)
				setServers([...servers])
			} else if (addForm.type === "device") {
				devices.push(e.data)
				setDevices([...devices])
			}
			setDialog(false)
		}
	}

	const getEntities = async (type) => {
		let resp = await state.callController(null, null, "POST", "/v3/group/entities",
			{ GID: id, Type: type, Limit: 1000, Offset: 0 },
			false, false)
		if (type === "user") {
			setUsers(resp.data)
		} else if (type === "server") {
			setServers(resp.data)
		} else if (type === "device") {
			setDevices(resp.data)
		}
	}

	const removeEntity = async (gid, typeid, type) => {
		let e = await state.callController(null, null, "POST", "/v3/group/remove",
			{ GroupID: gid, TypeID: typeid, Type: type },
			false, true)
		if (e === true) {
			if (type === "user") {
				let u = users.filter((u) => u._id !== typeid)
				setUsers([...u])
			} else if (type === "server") {
				let s = servers.filter((s) => s._id !== typeid)
				setServers([...s])
			} else if (type === "device") {
				let d = devices.filter((s) => s._id !== typeid)
				setServers([...d])
			}
		}
	}

	const tagChange = async (tab) => {
		setDialog(false)
		console.log("TAB:", tab)
		await getEntities(tab)
	}

	const getGroup = async () => {
		let resp = await state.callController(null, null, "POST", "/v3/group", { GID: id, }, false, false)
		if (resp.status === 200) {
			setGroup(resp.data)
		}
	}
	useEffect(() => {
		getGroup()
		getEntities("user")
	}, [])

	if (!group) {
		return (
			<div className="ab group-wrapper">
				<div className="title">Group Not Found: {id}</div>
			</div>
		)
	}


	const generateServerTable = () => {

		return {
			data: servers,
			rowClick: (obj) => {
				console.log("row click!")
				console.dir(obj)
			},
			columns: {
				Tag: (obj) => {
					// TODO
					// navigate("/inspect/server/" + obj._id)
				},
				_id: true,
			},
			columFormat: {
				Tag: (obj) => {
					state?.PrivateServers?.forEach(sn => {
						if (sn._id === obj._id) {
							return sn.Tag
						}
					})
					return "??"
				},
			},
			Btn: {
				Delete: (obj) => {
					removeEntity(id, obj._id, "server")
				},
				New: () => {
					setDialog(true)
				},
			},
			columnClass: {},
			headers: ["Tag", "ID"],
			headerClass: {},
			opts: {
				RowPerPage: 50,
			},
		}

	}


	const generateDevicesTables = () => {
		return {
			data: devices,
			rowClick: (obj) => {
				console.log("row click!")
				console.dir(obj)
			},
			columns: {
				Tag: (obj) => {
					// TODO
					// navigate("/inspect/server/" + obj._id)
				},
				_id: true,
			},
			columFormat: {},
			Btn: {
				Delete: (obj) => {
					removeEntity(id, obj._id, "device")
				},
				New: () => {
					setDialog(true)
				},
			},
			columnClass: {},
			headers: ["Tag", "ID"],
			headerClass: {},
			opts: {
				RowPerPage: 50,
			},
		}

	}


	let utable = {
		data: users,
		rowClick: (obj) => {
			console.log("row click!")
			console.dir(obj)
		},
		columns: {
			Email: true,
			_id: true,
		},
		columFormat: {
		},
		Btn: {
			Delete: (obj) => {
				removeEntity(id, obj._id, "user")
			},
			New: () => {
				setDialog(true)
			},
		},
		columnClass: {},
		headers: ["Username", "ID"],
		headerClass: {},
		opts: {
			RowPerPage: 50,
		},
	}


	const addDialog = (type) => {
		return (
			<Dialog
				open={dialog}
				onOpenChange={() => setDialog(false)}
			>
				<DialogContent className="bg-black border border-gray-800 text-white max-w-2xl rounded-lg p-6">

					{type === "device" &&
						<FormField label="Device ID">
							<Input
								value={addForm.id}
								onChange={(e) =>
									setAddForm({ id: e.target.value, type: "device", idtype: "" })
								}
								placeholder="Device ID"
								className="w-full bg-gray-950 border-gray-700 text-white"
							/>
						</FormField>
					}

					{type === "server" &&
						<FormField label="Server ID">
							<Input
								value={addForm.id}
								onChange={(e) =>
									setAddForm({ id: e.target.value, type: "server", idtype: "" })
								}
								placeholder="Server ID"
								className="w-full bg-gray-950 border-gray-700 text-white"
							/>
						</FormField>
					}

					{type === "user" &&
						<>
							<FormField label="Add user by Email or ID">
								<Input
									value={addForm.id}
									onChange={(e) =>
										setAddForm({ id: e.target.value, type: "user", idtype: "" })
									}
									placeholder="User ID"
									className="w-full bg-gray-950 border-gray-700 text-white"
								/>
							</FormField>
							<FormField>
								<Input
									value={addForm.id}
									onChange={(e) =>
										setAddForm({ id: e.target.value, type: "user", idtype: "email" })
									}
									placeholder="User Email"
									className="w-full bg-gray-950 border-gray-700 text-white"
								/>
							</FormField>
						</>
					}

					<div className="flex justify-between mt-1">
						<Button
							variant="outline"
							className="flex items-center gap-2 bg-gray-950 border-gray-700 hover:bg-gray-700"
							onClick={() => addToGroup()}
						>
							<Save className="h-4 w-4" />
							Save
						</Button>
					</div>

				</DialogContent>
			</Dialog>
		)
	}


	return (
		<div className="ab group-wrapper">
			<Tabs defaultValue="user" className="w-full" onValueChange={(v) => tagChange(v)}>
				<TabsList>
					<TabsTrigger value="server">Server</TabsTrigger>
					<TabsTrigger value="device">Devices</TabsTrigger>
					<TabsTrigger value="user">Users</TabsTrigger>
				</TabsList>
				<TabsContent className="w-full" value="server">
					{addDialog("server")}
					<GenericTable table={generateServerTable()} />
				</TabsContent>
				<TabsContent value="device">
					{addDialog("device")}
					<GenericTable table={generateDevicesTables()} />
				</TabsContent>
				<TabsContent value="user">
					{addDialog("user")}
					<GenericTable table={utable} />
				</TabsContent>
			</Tabs>

		</div >
	)
}

export default InspectGroup;
