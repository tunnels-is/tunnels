import React, { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import FormKeyValue from "./component/formkeyvalue";
import GLOBAL_STATE from "../state"
import KeyValue from "./component/keyvalue";
import InteractiveTable from "./component/interactive-table";
import dayjs from "dayjs";

const InspectGroup = () => {
	const { id } = useParams()
	const [tag, setTag] = useState("")
	const [groupID, setGroupID] = useState(id)
	const [users, setUsers] = useState([])
	const [servers, setServers] = useState([])
	const [devices, setDevices] = useState([])
	const [group, setGroup] = useState()
	const state = GLOBAL_STATE("groups")
	const navigate = useNavigate()

	let org = state?.Org

	const Create = async () => {
		let user = state.User
		if (!user) {
			console.log("LUL")
			return
		}
		let group = {}
		group.Nodes = {}
		group.Users = {}
		group.Devices = {}
		group.Tag = tag
		group.OrgID = user.OrgID

		await state.API_CreateGroup(group)
		state.renderPage("groups")
	}

	const Save = async () => {

		let newUsers = {}
		users.forEach(u => {
			newUsers[u._id] = { Email: u.Email, Added: u.Added }
		})

		let newServers = {}
		servers.forEach(n => {
			newServers[n._id] = { Added: n.Added }
		})

		let newDevices = {}
		devices.forEach(n => {
			newDevices[n._id] = { tag: n.Tag, Added: n.Added }
		})

		group.Devices = newDevices
		group.Users = newUsers
		group.Servers = newServers
		group.Tag = tag
		await state.API_UpdateGroup(group)
	}


	useEffect(() => {
		let org = state?.Org
		let group = undefined
		if (org) {
			org?.Groups?.forEach(g => {
				if (g._id === groupID) {
					group = g
					return
				}
			})
			if (group) {
				setGroup(group)
				setTag(group.Tag)
				if (users.length === 0 && group.Users) {
					Object.keys(group.Users).forEach(k => {
						users.push({ ...group.Users[k], _id: k })
					})
				}
				if (servers.length === 0 && group.Servers) {
					Object.keys(group.Servers).forEach(k => {
						servers.push({ ...group.Servers[k], _id: k })
					})
				}
				if (devices.length === 0 && group.Devices) {
					Object.keys(group.Devices).forEach(k => {
						devices.push({ ...group.Devices[k], _id: k })
					})
				}
			}
		}
	}, [])

	if (!groupID) {
		return (
			<div className="ab group-wrapper">
				<div className="title">Create Group</div>
				<FormKeyValue label={"Tag"} value={<input onChange={(e) => setTag(e.target.value)} type="text" />} />
				<div className="button" onClick={(e) => Create()}>Create</div>
			</div>
		)
	}

	if (!group) {
		return (
			<div className="ab group-wrapper">
				<div className="title">Group Not Found: {groupID}</div>
			</div>
		)
	}


	const usersInputChange = (e, id, key) => {
		users[id][key] = e.target.value
		setUsers(users)
		state.rerender()
	}

	const devicesInputChange = (e, id, key) => {
		devices[id][key] = e.target.value
		setDevices(devices)
		state.rerender()
	}

	const serversInputChange = (e, id, key) => {
		servers[id][key] = e.target.value
		setServers(servers)
		state.rerender()
	}

	const serversRemove = (id) => {
		let newServers = []
		servers.forEach((n) => {
			if (n._id !== id) {
				newServers.push(n)
			}
		})
		setServers(newServers)
		state.rerender()
	}

	const usersRemove = (id) => {
		let newUsers = []
		users.forEach((u) => {
			if (u._id !== id) {
				newUsers.push(u)
			}
		})
		setUsers(newUsers)
		state.rerender()
	}
	const devicesRemove = (id) => {
		let newDevices = []
		devices.forEach((u) => {
			if (u._id !== id) {
				newDevices.push(u)
			}
		})
		setDevices(newDevices)
		state.rerender()
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
					originalValue: tag,
					value: <div className="clickable" onClick={(e) => navigate("/inspect/server/" + s._id)}> {tag}</div >,
				},
				{
					originalValue: s._id,
					value: <input onChange={(e) => serversInputChange(e, i, "_id")} type="text" value={s._id} />
				},
				{ value: s.Added },
				{
					value: <div className="deleteable" onClick={() => serversRemove(s._id)} >Delete</div >,
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
					originalValue: u.Email,
					value: <input onChange={(e) => usersInputChange(e, i, "Email")} type="text" value={u.Email} />
				},
				{
					originalValue: u._id,
					value: <input onChange={(e) => usersInputChange(e, i, "_id")} type="text" value={u._id} />
				},
				{ value: u.Added },
				{
					value: <div className="deleteable" onClick={() => usersRemove(u._id)} >Delete</div >,
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
					originalValue: s.Tag,
					value: <input onChange={(e) => devicesInputChange(e, i, "Tag")} type="text" value={s.Tag} />
				},
				{
					originalValue: s._id,
					value: <input onChange={(e) => devicesInputChange(e, i, "_id")} type="text" value={s._id} />
				},
				{ value: s.Added },
				{
					value: <div className="deleteable" onClick={() => devicesRemove(s._id)} >Delete</div >,
				},
			]
			rows.push(row)
		});

		return rows
	}

	let usersRows = generateUsersTable(users)
	const usersHeaders = [
		{ value: "Email" },
		{ value: "ID" },
		{ value: "Added" },
		{ value: "" }
	]

	let nodesRows = generateServerTable(servers)
	const nodesHeaders = [
		{ value: "Tag" },
		{ value: "ID" },
		{ value: "" }
	]

	let deviceRows = generateDevicesTables(devices)
	const deviceHeader = [
		{ value: "Tag" },
		{ value: "ID" },
		{ value: "" }
	]

	return (
		<div className="ab group-wrapper">
			<div className="title">{tag}</div>

			<FormKeyValue label={"Tag"} value={
				<input type="text" value={tag} onChange={(e) => {
					// state.UpdateModifiedGroup(group, "Tag", e.target.value)
					setTag(e.target.value)
				}} />
			} />

			<KeyValue label={"ID"} value={group._id} />
			<KeyValue label={"Org"} value={org.Name} />
			<div className="button" onClick={() => Save()}>Update</div>

			<div className="nodes">
				<InteractiveTable
					title={"Servers"}
					className="group-table"
					header={nodesHeaders}
					rows={nodesRows}
					placeholder={"Search.."}
					newButton={{
						text: "Add",
						click: function(e) {
							setServers([...servers, { _id: servers.length + 1, Added: dayjs().format() }])
						}
					}}
				/>
			</div>

			<div className="nodes">
				<InteractiveTable
					title={"Users"}
					className="group-table"
					header={usersHeaders}
					rows={usersRows}
					placeholder={"Search.."}
					newButton={{
						text: "Add",
						click: function(e) {
							setUsers([...users, { Email: "", _id: users.length + 1, Added: dayjs().format() }])
						}
					}}
				/>
			</div>

			<div className="nodes">
				<InteractiveTable
					title={"Devices"}
					className="group-table"
					header={deviceHeader}
					rows={deviceRows}
					placeholder={"Search.."}
					newButton={{
						text: "Add",
						click: function(e) {
							setDevices([...devices, { Tag: "", _id: devices.length + 1, Added: dayjs().format() }])
						}
					}}
				/>
			</div>


		</div>
	)
}

export default InspectGroup;
