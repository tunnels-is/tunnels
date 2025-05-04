import React, { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import FormKeyValue from "./component/formkeyvalue";
import GLOBAL_STATE from "../state"
import KeyValue from "./component/keyvalue";
import dayjs from "dayjs";
import NewTable from "./component/newtable";
import ObjectEditor from "./ObjectEditor";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
	Dialog,
	DialogContent,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Edit,
	Save,
} from "lucide-react";
import { Label } from "@/components/ui/label";

const InspectGroup = () => {
	const { id } = useParams()
	const [groupID, setGroupID] = useState(id)
	const [users, setUsers] = useState([])
	const [servers, setServers] = useState([])
	const [devices, setDevices] = useState([])
	const [dialog, setDialog] = useState(false)
	const [addForm, setAddForm] = useState({})
	const [tag, setTag] = useState([])
	const [group, setGroup] = useState()
	const state = GLOBAL_STATE("groups")
	const navigate = useNavigate()


	const FormField = ({ label, children }) => (
		<div className="grid gap-2 mb-4">
			<Label className="text-sm font-medium">{label}</Label>
			{children}
		</div>
	);

	const getEntities = async (type) => {
		let e = await state.API_GetGroupEntities(id, type, 1000, 0)
		if (type === "user") {
			setUsers(e)
		} else if (type === "server") {
			setServers(e)
		} else if (type === "device") {
			setDevices(e)
		}
	}

	const getGroup = async () => {
		setGroup(await state.API_GetGroup(id))
	}
	useEffect(() => {
		getGroup()
	}, [])

	if (!group) {
		return (
			<div className="ab group-wrapper">
				<div className="title">Group Not Found: {groupID}</div>
			</div>
		)
	}


	const generateServerTable = (servers) => {
		let rows = []
		servers.forEach((s, i) => {
			let tag = ""
			state?.PrivateServers?.forEach(sn => {
				if (sn._id === s._id) {
					tag = sn.Tag
					return
				}
			})
			let row = {}
			row.items = [
				{
					type: "text",
					color: "blue",
					click: () => {
						navigate("/inspect/server/" + s._id)
					},
					value: tag,
				},
				{
					minWidth: "250px",
					type: "text",
					value: s._id
				},
				{
					type: "text",
					value: dayjs(s.Added).format("DD-MM-YYYY HH:mm:ss"),
				},
				{
					type: "text",
					color: "red",
					click: () => {
						serversRemove(s._id)
					},
					value: "Delete"
				},
			]
			rows.push(row)
		});

		return rows
	}

	const generateUsersTable = (users) => {
		let rows = []
		users.forEach((u, i) => {
			let row = {}
			row.items = [
				{
					type: "text",
					color: "blue",
					value: u.Email,
				},
				{
					type: "text",
					minWidth: "250px",
					value: u._id,
				},
				{
					type: "text",
					value: dayjs(u.Added).format("DD-MM-YYYY HH:mm:ss"),
				},
				{
					type: "text",
					color: "red",
					click: () => {
						usersRemove(u._id)
					},
					value: "Delete"
				},
			]
			rows.push(row)
		});

		return rows
	}

	const generateDevicesTables = (devices) => {
		let rows = []
		devices.forEach((s, i) => {
			let row = {}
			row.items = [
				{
					type: "text",
					color: "blue",
					value: s.Tag,
				},
				{
					type: "text",
					minWidth: "310px",
					value: s._id,
				},
				{
					minWidth: "200px",
					type: "text",
					value: dayjs(s.Added).format("DD-MM-YYYY HH:mm:ss"),
				},
				{
					type: "text",
					color: "red",
					click: () => {
						devicesRemove(s._id)
					},
					value: "Delete"
				},
			]
			rows.push(row)
		});

		return rows
	}

	let usersRows = generateUsersTable(users)
	const usersHeaders = [
		{ value: "Email" },
		{ value: "ID", minWidth: "250px" },
		{ value: "Added" },
		{ value: "" }
	]

	let nodesRows = generateServerTable(servers)
	const nodesHeaders = [
		{ value: "Tag" },
		{ value: "ID", minWidth: "250px" },
		{ value: "Added" },
		{ value: "" }
	]

	let deviceRows = generateDevicesTables(devices)
	const deviceHeader = [
		{ value: "Tag" },
		{ value: "ID", minWidth: "310px" },
		{ value: "Added", minWidth: "200px" },
		{ value: "" }
	]

	const addUserDialog = () => {
		return (
			<Dialog
				open={dialog}
				onOpenChange={() => setDialog(false)}
			>
				<DialogContent className="bg-black border border-gray-800 text-white max-w-2xl rounded-lg p-6">

					<FormField label="Add user by Email or ID">
						<Input
							value={addForm.id}
							onChange={(e) =>
								setAddForm({ index: i, id: e.target.value, type: "user", idtype: "" })
							}
							placeholder="User ID"
							className="w-full bg-gray-950 border-gray-700 text-white"
						/>
					</FormField>
					<FormField>
						<Input
							value={addForm.id}
							onChange={(e) =>
								setAddForm({ index: i, id: e.target.value, type: "user", idtype: "email" })
							}
							placeholder="User Email"
							className="w-full bg-gray-950 border-gray-700 text-white"
						/>
					</FormField>
					<div className="flex justify-between mt-1">
						<Button
							variant="outline"
							className="flex items-center gap-2 bg-gray-950 border-gray-700 hover:bg-gray-700"
							onClick={() => addUser()}
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
			<Tabs defaultValue="general" className="w-[400px]" onValueChange={() => setDialog(false)}>
				<TabsList>
					<TabsTrigger value="servers">Server</TabsTrigger>
					<TabsTrigger value="devices">Devices</TabsTrigger>
					<TabsTrigger value="users">Users</TabsTrigger>
				</TabsList>
				<TabsContent value="servers">
					<NewTable
						background={true}
						title={"Servers"}
						className="group-table"
						header={nodesHeaders}
						rows={nodesRows}
						placeholder={"Search.."}
						button={{
							text: "Add",
							click: () => setDialog(true)
						}}
						button2={{
							color: "green",
							text: "Save",
							click: function() {
								Save()
							}
						}}
					/>
				</TabsContent>
				<TabsContent value="devices">
					<NewTable
						background={true}
						title={"Devices"}
						className="group-table"
						header={deviceHeader}
						rows={deviceRows}
						placeholder={"Search.."}
						button={{
							text: "Add",
							click: () => setDialog(true)
						}}
						button2={{
							color: "green",
							text: "Save",
							click: function() {
								Save()
							}
						}}
					/>
				</TabsContent>
				<TabsContent value="users">
					{addUserDialog()}
					<NewTable
						background={true}
						title={"Users"}
						className="group-table"
						header={usersHeaders}
						rows={usersRows}
						placeholder={"Search.."}
						button={{
							text: "Add",
							click: () => setDialog(true)
						}}
						button2={{
							color: "green",
							text: "Save",
							click: function() {
								Save()
							}
						}}
					/>
				</TabsContent>
			</Tabs>

		</div >
	)
}

export default InspectGroup;
