import React, { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import GLOBAL_STATE from "../state"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import GenericTable from "./GenericTable";
import NewObjectEditorDialog from "./NewObjectEdiorDialog";

const InspectGroup = () => {
	const { id } = useParams()
	const [users, setUsers] = useState([])
	const [servers, setServers] = useState([])
	const [devices, setDevices] = useState([])
	const [dialog, setDialog] = useState(false)
	const [addForm, setAddForm] = useState({})
	const [group, setGroup] = useState()
	const [tag, setTag] = useState("users")
	const state = GLOBAL_STATE("groups")

	const addToGroup = async () => {
		console.log("ID:", addForm.id)
		let e = await state.callController(null, null, "POST", "/v3/group/add",
			{
				GroupID: id,
				TypeID: addForm.id,
				Type: addForm.type,
				TypeTag: addForm.idtype,
			},
			false, false)
		if (e.status === 200) {
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
				setDevices([...d])
			}
		}
	}

	const tagChange = async (tag) => {
		setDialog(false)
		setTag(tag)
		console.log("TAB:", tag)
		await getEntities(tag)
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
					setAddForm({ id: "", type: "server", idtype: "" })
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
					setAddForm({ id: "", type: "device", idtype: "" })
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
				setAddForm({ id: "", type: "user", idtype: "" })
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

	if (!group) {
		return (
			<div className="ab group-wrapper">			</div>
		)
	}

	return (
		<div className="ab group-wrapper">
			<NewObjectEditorDialog
				open={dialog}
				onOpenChange={setDialog}
				object={addForm}
				opts={{
					fields: {
						idtype: "hidden",
						type: "hidden"
					},
					nameMap: {
						id: tag + " ID"
					}
				}}
				title={tag}
				readOnly={false}
				saveButton={() => {
					addToGroup()
				}}
				onChange={(key, value, type) => {
					addForm[key] = value
					console.log(key, value, type)
				}}
			/>

			<Tabs defaultValue="server" className="w-full" onValueChange={(v) => tagChange(v)}>
				<TabsList
					className={state.Theme?.borderColor}
				>
					<TabsTrigger className={state.Theme?.tabs} value="server">Servers</TabsTrigger>
					<TabsTrigger className={state.Theme?.tabs} value="device">Devices</TabsTrigger>
					<TabsTrigger className={state.Theme?.tabs} value="user">Users</TabsTrigger>
				</TabsList>
				<TabsContent className="w-full" value="server">
					<GenericTable table={generateServerTable()} newButtonLabel={"Add"} />
				</TabsContent>
				<TabsContent value="device">
					<GenericTable table={generateDevicesTables()} newButtonLabel={"Add"} />
				</TabsContent>
				<TabsContent value="user">

					<GenericTable table={utable} newButtonLabel={"Add"} />
				</TabsContent>
			</Tabs>

		</div >
	)
}

export default InspectGroup;
